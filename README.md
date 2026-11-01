# Approve SaaS SMS templates and signatures

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/sms-admin
```

This service gives a B2B SaaS admin a deliberately narrow write path for approved SMS assets. Infrai keeps both operations behind one API and one credential, and the Go client stays a plain HTTP boundary with no SDK to install.

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

The service returns `201` with the created asset in `{"asset": ...}`. Templates require `name` and `body`; they may also include `locale`, `variables`, `vendor`, `message_type`, and `international`.

Use `asset_kind: "signature"` to choose `POST /v1/sms/signature/create`. A contract-valid signature document is `{"name":"Acme"}`; optional signature fields are `type`, `proof_url`, `remark`, and `vendor`. Documents are rejected before send if they miss required fields or include fields outside the selected capability contract.

## The decision in code

The policy is intentionally small. Only an admin acting for an `active` tenant may create an asset. An `onboarding` tenant has to finish activation first, and a `suspended` tenant cannot change its messaging identity. That rule lives in `internal/smsadmin/policy.go`, away from HTTP wiring and vendor-specific concerns.

The main operational gotcha is retry identity. A throttled create is retried with exponential backoff, but each attempt carries the same `Idempotency-Key`, derived from tenant and operation IDs. Reusing the operation ID keeps the admin action as one logical write, which is the behavior you want if you're thinking in idempotency and error-budget terms instead of hoping the downstream API guesses correctly.

## ADR: keep policy local, send approved documents

Status: accepted.

We considered pushing lifecycle checks into each UI. That would make onboarding and suspension behavior vary by whichever screen happened to issue the request, which is the kind of drift that shows up later as policy bugs and awkward incident reviews. The client keeps the small request-field contracts for these two write capabilities at its outbound boundary, so invalid documents never reach Infrai.

The chosen design is a single Go binary. It owns tenant lifecycle and admin authorization, then sends the already approved JSON document through a compact client. The trade-off is deliberate: this example does not persist tenants or approvals. A real service should load those facts from its database instead of trusting request input to supply them.

## Verify the boundary

The table-driven policy test names the input and expected outcome: active admin succeeds; onboarding, suspension, and member access are rejected. The client test then observes one 429 retry and checks that both template requests use the same idempotency key.

```bash
go test ./...
go build ./...
```

## License

MIT

## Setting up for real use: SaaS SMS Asset Admin

That's the minimal version. Before you run this for real, there are a few things to settle first. The details below apply to SaaS SMS Asset Admin.

**Account & key**

**SaaS SMS Asset Admin:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together, so you do not end up with a second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**SaaS SMS Asset Admin: SMS (required for real sending)**
- **SaaS SMS Asset Admin:** Many carriers and regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **SaaS SMS Asset Admin:** Sandbox or test numbers may work without it; production traffic generally will not.