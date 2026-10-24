# CLAUDE.md — api (ctech-dfe-api)

Go REST API — Fiber v3, multi-tenant, DynamoDB, S3, AWS SDK v2.

**CTech standards:** generic backend rules (layering, errors, resilience, DynamoDB access, scale, cost) live in the
`ctech-dev-standards` skill (`references/backend.md`); it wins over this file. This file records only what is
specific to the DFe API and where this code still diverges from the standard.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §4`; endpoint reference in `README.md`.

---

## Role

Handles authentication context, organization management, fiscal document issuance, and the business logic that
bridges the frontend and the async worker/SEFAZ pipeline.

**Request flow:** `HTTP → Fiber middleware (request id, recover, CORS, access log, deadline) → auth/tenant/RBAC → Route → Service → Repository → DynamoDB`

---

## Directory Structure

```
api/
├── cmd/
│   ├── server/                 # Fiber app, fx wiring (main only)
│   └── migrate-billing-org, migrate-grants, rekey-companies, openapi-dump   # one-shot tools
├── internal/
│   ├── app/app.go              # fx module, Fiber app, cache/ws wiring
│   ├── config/                 # env config (caarlos0/env); prod boot validation
│   ├── dfeproblem/             # DFe-only RFC 7807 problems (generic ones: api-commons/problem)
│   ├── middleware/             # auth, tenant, rbac, scopes, reach, subscription, internal
│   ├── oauthresource/          # OAuth scope manifest
│   ├── accountclient/ billingclient/   # outbound clients to ctech-account / ctech-billing
│   ├── consumer/               # SQS results consumer (worker results -> WebSocket)
│   ├── awsclient/              # aws-sdk-go-v2 clients
│   ├── repositories/           # DynamoDB access
│   ├── services/               # business logic (+ nfes/, mdfes/, nfses/, documents/ packages)
│   └── api/v1/                 # routes + helpers
└── tests/integration/
```

Cache, WebSocket registry, JWT verification, deadline, erasure and observability come from `gopkg.aoctech.app/api-commons`
(go-common), not from local packages.

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing implementations before creating new code.
3. Plan → Implement → `go test ./... -race` from `api/`.
4. Update `../DOCS.md` for new endpoints/schemas; `../CONDUCT.md` for new constraints.
5. State which components were cross-reviewed.
6. Suggest Conventional Commit.

---

## Product Rules

- **Layering:** Route parses and calls ONE service method; Service holds rules and cache management; Repository only
  reads/writes DynamoDB. Services and repositories are injected via `go.uber.org/fx`; never instantiated in handlers.
- **Errors:** all route errors go through `sendProblem(c, err)`; services return `*problem.Problem` from `api-commons/problem` (`problem.BadRequest`,
  `problem.NotFound`, ...). Never return raw errors or `fiber.Map`.
- **DRY helpers:** CRUD repositories extend `CRUDRepository[T]` (`repositories/base.go`); CRUD services use
  `CRUDMutationHelper` and `GetCachedItem`/`GetCachedList` (`services/crud.go`). UF→IBGE maps and SEFAZ environment
  strings are declared once; search `internal/` before adding constants.
- **Tenant:** the organization comes from the `Dfe-Organization-Pk` header (`OrgHeader`, `middleware/rbac.go`), never
  path parameters (except org creation), and must stay in sync with the UI `ORG_HEADER`.
- **Fiscal numbering** uses `TransactWrite` (never separate read + write). New GSIs need documented access patterns.
- **Auth:** RS256 only (JWKS of ctech-account). `iss`/`aud`/`azp` checked; in `prod` the boot fails when
  `SERVICE_AUDIENCE`, `CTECH_ISSUER_URL`, `WORKER_TOPIC_ARN` or `CORS_ALLOWED_ORIGINS` is empty (`config.go`).
- **Distributions:** use `GET /v1.0/distributions/{doc_type}/history` and `POST /v1.0/distributions/{doc_type}/sync`;
  there is no `GET /v1.0/distributions/nfe`.
- Profile/password management does not exist here; it belongs to ctech-account.
- Never commit JWT secrets, AWS credentials, PFX certs, real CPF/CNPJ or customer data.
- Binary deployed to EC2 must be named `app` (userdata runs `/opt/app/current/app`).

---

## Known Constraints

- **Issuance is asynchronous:** a transactional outbox is published through SNS to standard SQS, the worker
  processes it, and terminal results flow through SNS to this API's results consumer and WebSocket fan-out. The API
  returns 202.
- **Account deletion (LGPD):** the auth middleware refuses writes of a locked/erased user with 403
  `account-pending-deletion`; revocation entries come from the shared Valkey DB 0; `ERASURE_QUEUE_URL` is the
  participant on/off switch; acks and the subject lookup use `ERASURE_CLIENT_*`, never the reach `ACCOUNT_CLIENT_*`.
  New tables go in `services/erasure_catalog.go`.
- **Cache:** shared Valkey via `VALKEY_URL`; without it the cache falls back to an in-process memory backend
  (1000 entries, not shared across replicas).
- **CORS is wired:** `app.go` mounts `cors.New` with `CORS_ALLOWED_ORIGINS` (credentials allowed). The UI is a static
  export on Cloudflare and calls this API directly, so CORS is in the production path. The deployed value is the
  app URL (`SERVICE_AUDIENCE`, see `cdk/lib/api-stack.ts`).
- **NFS-e** lives in `internal/services/nfses/` (own package): the row SK is the `id_dps`, `WorkerMessage.UF` is empty
  (municipal competence), and reads that hit the ADN (`municipal.go`) call go-dfe through the go-dfe-egress Lambda
  (`ExternalService.CallDfe`, region `DFE_EGRESS_REGION`), never in process. See `../DOCS.md` (Emissao de NFS-e).
- Health: `/v1.0/health` (liveness, skipped by the access log) and `/v1.0/health-check`.

### Where the code differs from the CTech standard (do not copy these patterns)

- **No Scan in the service roles.** `dynamodb:Scan` is not granted to the API instance role or any worker role.
  Background enumeration uses the sparse `sweep-index` GSI (`sweep = "1"`, KEYS_ONLY) on the nfe/cte/mdfe/nfse config
  tables and the certificates table: every write stamps `sweep` (`repositories.AttrSweep`), responses and audit diffs
  strip it, and `cmd/backfill-sweep` stamps rows that predate the index. Readers: the worker's distribution dispatcher
  and `ErasureService.scanCertificates` (e-CPF search: query the index, then a consistent GetItem per row).
  `RoleRepository.ListAll` reads the four fixed system roles with GetItem.
  Still scanning, on purpose and only as one-shot operator tools that need operator credentials:
  `repositories/organizations.go` (`ListCompanyIndexGaps`, `ListOrganizationsWithCompanies`),
  `repositories/account_billing.go` (`ListUserSnapshots`), `cmd/migrate-grants`, `cmd/rekey-companies`,
  `cmd/backfill-sweep`. A new config/certificate write path MUST stamp `sweep` or the row is invisible to those readers.
- **Rate limit:** `middleware.RateLimit` runs after auth (`Verifier.WithAfterAuth`), per user, 1200 reads/min and 300
  writes/min, fail open, 429 + `Retry-After`. Budgets are the constants in `middleware/ratelimit.go`; there is no
  per-organization cap (the org is only resolved later, by RBAC).
- **Idempotency:** every authenticated POST requires `Idempotency-Key` (400 without it), enforced after auth by `middleware.Idempotency`
  (`Verifier.WithAfterAuth`, scope user + org header, table `{prefix}_idempotency`, fail closed: 503 if the store is down).
  Optional (honored if sent): `/billing/`, `/auth/`, `/invitations/`, `/cargo-preview`. Outbound calls to ctech-billing carry their own keys.
- `v1.Register` takes positional parameters (cache, cfg, ws registry, AWS clients, `Services`), not a single `Deps`.
- No custom metrics, no alarms: logs only.

---

## Testing

| Change | Required |
|--------|----------|
| Service logic | Unit test |
| Repository / AWS integration | Integration test (DynamoDB) |
| Fiscal issuance | Unit + integration |
| New endpoint | Unit + integration |
| Bug fix | Regression test |

Run: `go test ./... -race` from `api/`.

---

## Critical Areas (require analysis before touching)

- Fiscal document issuance (NF-e/NFC-e/CT-e/MDF-e/NFS-e) and fiscal numbering (`transact_write` reservation)
- Certificate upload and management
- JWT validation and RBAC middleware
- Cancellation and fiscal events

Before touching: identify risks + side effects, verify backward compatibility + regulatory impact.

---

## Completion Checklist

- [ ] `go test ./... -race` passes; no duplication introduced
- [ ] Constants named; errors via `sendProblem` / `problem.*`
- [ ] Docs updated (`../DOCS.md` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (api ↔ worker ↔ ui ↔ go-dfe-egress ↔ ctech-account)

Every change that alters behavior, a contract or an operational rule updates the matching docs in the same change.
