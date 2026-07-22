# AGENTS.md — api (ctech-dfe-api)

Go REST API — Fiber v3, multi-tenant, DynamoDB, S3, AWS SDK v2.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §4`.

---

## Role

Handles authentication, organization management, fiscal document issuance, and all business logic
that bridges the frontend and the async worker/SEFAZ pipeline.

**Request flow:** `HTTP → Middleware (auth, tenant, RBAC) → Route → Service → Repository → DynamoDB`

---

## Directory Structure

```
api/
├── cmd/server/main.go          # Fiber app, fx wiring, startup
├── internal/
│   ├── config/                 # 12-Factor env config (caarlos0/env)
│   ├── problem/                # RFC 7807 Problem type + helpers
│   ├── middleware/             # auth.go, tenant.go, perm.go
│   ├── cache/                  # Redis + in-memory backends
│   ├── ws/                     # WebSocket registry (Redis pub/sub)
│   ├── awsclient/              # aws-sdk-go-v2 wrappers
│   ├── repositories/           # Persistence only — DynamoDB access
│   ├── services/               # Business logic
│   └── api/v1/                 # Routes + helpers
└── tests/integration/
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing implementations before creating new code.
3. Plan → Implement → Run affected tests.
4. Update `../DOCS.md` for new endpoints/schemas; `../CONDUCT.md` for new constraints.
5. State which components were cross-reviewed.
6. Suggest Conventional Commit.

---

## Engineering Rules

### DRY

- Never duplicate functions. If two functions do the same thing, unify them.
- Before adding any function or type, search `internal/` for existing implementations.
- UF→IBGE code maps and SEFAZ environment strings MUST NOT be redeclared in individual files.
  Search `internal/` for existing constant definitions before adding any new constant.

### Constants — no magic strings/numbers

- All string keys, status codes, table name suffixes, header names, cache key prefixes, and env
  strings MUST be defined as named constants or config fields.
- The `Dfe-Organization-Pk` header name is defined once as `OrgHeader` in
  `middleware/rbac.go:22` — never hardcoded in route files. It MUST stay in sync with the UI
  constant `ORG_HEADER` in `ui/src/lib/api/client.ts:50`.
- **Never call the deprecated `GET /v1.0/distributions/nfe`** — it does not exist; use
  `GET /v1.0/distributions/{doc_type}/history` (`internal/api/v1/distributions.go:18-29`).
- Endpoint reference: [`README.md`](README.md).

### Error Handling (MUST follow)

- **All route errors go through `sendProblem(c, err)`** — never return raw errors, `fiber.Map`,
  or `fiber.NewError`.
- **Services return `*problem.Problem`** via `problem.BadRequest`, `problem.NotFound`,
  `problem.InternalServer` helpers (or wrap unexpected errors).
- Route handler shape:
  ```go
  result, err := svc.DoSomething(ctx, params)
  if err != nil {
      return sendProblem(c, err)
  }
  return sendItem(c, result)
  ```

### Layer Separation (strictly enforced)

| Layer      | Allowed                                         | Forbidden                            |
|------------|-------------------------------------------------|--------------------------------------|
| Repository | DynamoDB read/write only                        | Business logic, cache, HTTP concerns |
| Service    | Business logic, cache management                | DynamoDB SDK calls, HTTP parsing     |
| Route      | Parse request, call ONE service method, respond | Business logic, repo calls, cache    |

### Dependency Injection

- Services and repositories are injected via `go.uber.org/fx`.
- **Never instantiate** services, repositories, or AWS clients inside route handlers.

### DynamoDB

- Access priority: `GetItem` > `Query` > `Scan`.
- **No scans in production.**
- NF-e fiscal number reservation requires `TransactWrite` — never replace with separate read + write.
- New GSIs require documented access patterns before creation.

### Go Rules

- No goroutines inside request handlers — Fiber handles concurrency.
- Use `aws-sdk-go-v2` only — do not add any Python client or boto3 reference.
- Auth is RS256-only. No `SECRET_KEY`, no HS256.
- Binary deployed to EC2 must be named `app` (CDK userdata expects `/opt/app/current/app`).

### Secrets

Never commit: JWT secrets, AWS credentials, PFX certs, real CPF/CNPJ, real customer data.

---

## Testing

| Change          | Required                    |
|-----------------|-----------------------------|
| Service logic   | Unit test                   |
| Repository      | Integration test (DynamoDB) |
| AWS integration | Integration test            |
| Fiscal issuance | Unit + integration          |
| Bug fix         | Regression test             |
| New endpoint    | Unit + integration          |

**Every core service function must have an integration test.**

Run: `go test ./... -race` from `api/`.

---

## Known Constraints

- `InMemoryCache` TTL=300s — not shared across replicas. Redis/Valkey is authoritative for JWKS and WebSocket pub/sub.
- Lambda invocation for doc issuance is async: API enqueues to SQS FIFO, returns 202, worker processes and pushes
  WebSocket update.
- Organization context is always via `Dfe-Organization-Pk` header — never path parameters (except org creation
  endpoints).
- Profile/password management endpoints do not exist here — those belong to ctech-account.
- JWKS keys cached in Redis (TTL 1h), falls back to in-memory when `VALKEY_URL` is unset.

---

## Critical Areas (require analysis before touching)

- Fiscal document issuance (NF-e/NFC-e/CT-e/MDF-e)
- Fiscal numbering (`transact_write` reservation)
- Certificate upload and management
- JWT validation and RBAC middleware
- Cancellation and fiscal events

Before touching: identify risks + side effects, verify backward compatibility + regulatory impact.

---

## Completion Checklist

- [ ] Code compiles; `go test ./... -race` passes
- [ ] No duplication introduced (searched before creating)
- [ ] All constants named (no magic strings)
- [ ] Errors returned via `sendProblem` / `problem.*` helpers
- [ ] Docs updated (`../DOCS.md` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (api ↔ worker ↔ ui ↔ py-dfe)
