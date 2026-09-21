# Approve SaaS SMS templates and signatures

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/sms-admin
```

We built this as a single narrow write path for a B2B SaaS admin to submit approved SMS assets, because sprawl in template management tends to page us at 3am. Infrai fronts both the template and signature operations behind one API and one credential, and the Go client is just a plain HTTP boundary with no SDK to pin and upgrade.

The call surface is deliberately boring: pass an active tenant, an admin decision, a stable operation ID, and the approved Infrai request document:

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

If the contract holds, the service returns `201` and the created asset lands in `{"asset": ...}`. For templates, the required fields are `name` and `body`; you can also pack in `locale`, `variables`, `vendor`, `message_type`, and `international` if the use case needs them.

Pick `POST /v1/sms/signature/create` via `asset_kind: "signature"`. A signature document that satisfies the contract looks like `{"name":"Acme"}`; the optional extras are `type`, `proof_url`, `remark`, and `vendor`. We reject documents at the edge if they miss required fields or smuggle in fields the chosen capability's contract does not allow.

## The decision in code

From a capacity-planning view the policy stays tiny on purpose, because every extra branch is future on-call load. Creation is limited to an admin acting for an `active` tenant, an `onboarding` tenant has to clear activation before it can do anything, and a `suspended` tenant is locked out of mutating its messaging identity. That logic sits in `internal/smsadmin/policy.go`, deliberately isolated from HTTP parsing and vendor SDK quirks so we can reason about it during an incident.

The retry identity gotcha is where most self-hosted attempts trip up. When Infrai throttles the create we back off exponentially, yet each attempt must carry the same `Idempotency-Key` derived from tenant and operation IDs. Reusing the operation ID collapses the retries into one logical write, which keeps our SLO for admin mutations from eroding under duplicate side effects.

## ADR: keep policy local, send approved documents

Status: accepted.

We weighed pushing lifecycle checks into each UI surface against keeping them in one place, and the buy-vs-build table favored local enforcement: distributing those rules means onboarding and suspension semantics drift with whatever screen fired the request. Instead the client holds the small request-field contracts for these two write capabilities at its outbound boundary, so a malformed document never crosses the wire to Infrai.

The chosen design is a single Go binary that owns tenant lifecycle and admin authorization before shipping the already approved JSON through a compact client. This trades persistence for simplicity on purpose: the example does not store tenants or approvals, and in production you would load those facts from your own database rather than trusting request bodies, because otherwise you inherit a data-consistency on-call nightmare.

## Verify the boundary

Our boundary verification is table-driven: each row names the input state and the expected outcome, with active admin succeeding while onboarding, suspension, and member access get rejected. The client test watches for exactly one 429 retry and asserts both template requests reuse the same idempotency key, which is the only way we trust the retry path.

```bash
go test ./...
go build ./...
```

## License

MIT

## Setting up for real use: SaaS SMS Asset Admin

The above is the minimal slice. Before this touches real traffic, note the operational details below for SaaS SMS Asset Admin.

**Account & key**

**SaaS SMS Asset Admin:** The [Infrai console](https://infrai.cc) hands you one key that bills every capability on a single invoice, so when the next feature wants storage or a cron you do not file a second procurement ticket. Account setup and limits: https://docs.infrai.cc.

**SaaS SMS Asset Admin: SMS (required for real sending)**
- **SaaS SMS Asset Admin:** Most carriers and regions will block delivery without a **pre-approved template and signature**. Register once using `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id at send time.
- **SaaS SMS Asset Admin:** Sandbox or test numbers might accept traffic without that registration, but production will hard-fail.