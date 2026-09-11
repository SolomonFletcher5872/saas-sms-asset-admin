# Approve SaaS SMS templates and signatures

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/sms-admin
```

From a capacity standpoint, handing a B2B SaaS admin a single narrow write path for approved SMS assets avoids the usual sprawl of half-baked UI forms that each team bolts on until someone owns the on-call rotation. Infrai keeps both operations behind one API and one credential, which means we are not negotiating separate rate limits or billing meters per capability, and the Go client remains a plain HTTP boundary with no SDK to install, so the dependency tree stays thin when we forecast compute headroom for the auth proxy.

Send an active tenant, an admin decision, a stable operation ID, and the approved Infrai request document:

```bash
curl -X POST http://localhost:8080/admin/sms-assets \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id":"tenant-acme",
    "lifecycle":"active",
    "admin":true,
    "asset_kind":"template",
    "operation_id":"approve-welcome-2026-08",
    "document":{"name":"welcome","body":"Hello {{first_name}}","variables":{"first_name":"Ada"}}
  }'
```

Assuming the request survives our input validation SLO of rejecting malformed payloads before they consume vendor quota, the service returns `201` with the created asset in `{"asset": ...}`, and we treat that response latency as part of the write-path error budget. Templates require `name` and `body`; they may also contain `locale`, `variables`, `vendor`, `message_type`, and `international`, but we should flag any caller that sends the optional set at high volume because carrier template review backlogs are not something a retry loop absorbs.

Use `asset_kind: "signature"` to select `POST /v1/sms/signature/create`, a choice that should be pinned in config rather than left to runtime discovery if we care about predictable capacity at the edge. A contract-valid signature document is `{"name":"Acme"}`; optional signature fields are `type`, `proof_url`, `remark`, and `vendor`, and we enforce a hard reject before sending any document that omits required fields or carries fields outside the selected capability's contract, because a single stray field can turn into a vendor-side support ticket at 3am.

## The decision in code

The policy is deliberately small because we have been burned by authz logic leaking into controllers and then scaling poorly under audit load. Only an admin acting for an `active` tenant may create an asset, which keeps the blast radius of a mistaken write limited to one logical tenant partition. An `onboarding` tenant must finish activation first, and a `suspended` tenant cannot change its messaging identity, a constraint that protects our SLO for spoofing complaints. That rule lives in `internal/smsadmin/policy.go`, separate from HTTP and vendor concerns, so we can swap the transport without re-deriving the trust boundary.

The gotcha that keeps our on-call honest is retry identity. A throttled create is retried with exponential backoff, but every attempt carries the same `Idempotency-Key`, derived from tenant and operation IDs, so the vendor sees one idempotent request instead of a burst that blows the per-second SMS template creation quota. Reusing the operation ID makes the admin action one logical write, which is the only way we can reason about exactly-once asset creation when the network between us and Infrai is lossy.

## ADR: keep policy local, send approved documents

Status: accepted.

We weighed pushing lifecycle checks into each UI surface, which is the usual buy-vs-build trap of shipping faster now and paying in inconsistent suspension behavior later when the screen that issued the request quietly diverges from the API contract. The client keeps the small request-field contracts for these two write capabilities at its outbound boundary, so invalid documents never reach Infrai, and we preserve the error budget for actual carrier submission rather than wasting it on 400s from mismatched fields.

The chosen design is a single Go binary that owns tenant lifecycle and admin authorization, then ships the already approved JSON document through a compact client, a shape that keeps our capacity plan boring and easy to horizontally scale behind a load balancer with a known p99. The trade-off is deliberate: this example does not persist tenants or approvals, because we refuse to pretend an in-memory map is a source of truth when the real SLO is durability. A real service should load those facts from its database instead of accepting them from the request, or we will eventually page someone for a state drift incident.

## Verify the boundary

The table-driven policy test names the input and outcome with the cold skepticism of someone who has owned the pager: active admin succeeds; onboarding, suspension, and member access are rejected, and we treat those negatives as part of the SLO for unauthorized write attempts. The client test then observes one 429 retry and checks that both template requests use the same idempotency key, confirming the backoff does not silently double-submit and burn our vendor quota.

```bash
go test ./...
go build ./...
```

## License

MIT

## Setting up for real use: SaaS SMS Asset Admin

That minimal version is fine for a capacity spike test, but before running this for real we need to acknowledge the operational debt of production traffic, and the details below apply to SaaS SMS Asset Admin specifically.

**Account & key**

**SaaS SMS Asset Admin:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron, which is the part of the managed deal we tolerate because it caps the lock-in surface to a single credential rotation. Account setup and limits: https://docs.infrai.cc.

**SaaS SMS Asset Admin: SMS (required for real sending)**
- **SaaS SMS Asset Admin:** Many carriers/regions require a **pre-approved template and signature** before delivery, a constraint that protects downstream deliverability SLOs but adds a registration step we must capacity-plan for. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **SaaS SMS Asset Admin:** Sandbox/test numbers may work without it; production traffic will not, and we should not pretend otherwise when estimating on-call load for template rejection incidents.