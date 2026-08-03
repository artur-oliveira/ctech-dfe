# ctech-dfe API

Go REST API — **Fiber v3**, multi-tenant, DynamoDB + S3 + SNS/SQS + WebSocket. This document is anchored to the actual
code (`internal/...`); where it disagrees with other docs, the code wins.

Up-to-date source maps: [`CLAUDE.md`](CLAUDE.md) · [`AGENTS.md`](AGENTS.md) · root [`DOCS.md`](../DOCS.md) · [
`INTEGRATION.md`](../INTEGRATION.md).

> **Hard rule (enforced in code):** never call `GET /v1.0/distributions/nfe`.
> Use `GET /v1.0/distributions/{doc_type}/history` instead
> (`internal/api/v1/distributions.go:18-29`). The bare `/distributions/nfe` route does
> not exist in this codebase.

---

## 1. Architecture

```
HTTP
  → middleware (auth.go, tenant.go, rbac.go, recover.go, scopes.go)
  → router   internal/api/v1/router.go   (mounts /v1.0 group)
  → handler  internal/api/v1/*.go        (parse request, call ONE service method, respond)
  → service  internal/services/*.go      (business logic, caching, audit, SNS/SQS dispatch)
  → repo     internal/repositories/*.go  (DynamoDB access only)
```

Every route group is mounted under `/v1.0` (`router.go:47`). Fx dependency injection wires everything
(`internal/app/app.go` `Module`).

Layer contract (see `CLAUDE.md` "Layer Separation"):

| Layer      | Allowed                         | Forbidden                      |
|------------|---------------------------------|--------------------------------|
| handler    | parse, call 1 service, respond  | business logic, repo/SDK calls |
| service    | business logic, cache, dispatch | DynamoDB SDK, HTTP parsing     |
| repository | DynamoDB read/write only        | business logic, cache, HTTP    |

---

## 2. Authentication & tenancy

- **JWT (RS256 only)**: verified by `middleware.NewVerifier` (`router.go:43`) against the
  `ctech-account` JWKS (`CtechJWKSURL`). `aud` must contain `ServiceAudience`
  (`cfg.ServiceAudience`); audience check is skipped when empty. `azp` = OAuth client_id. Scope decision is
  defense-in-depth: a scoped API-key token needs the matching `dfe:*`
  OAuth scope, while a first-party session (no scope) is unrestricted (`rbac.go:143-148`).
- **Tenant header `Dfe-Organization-Pk`**: defined **once** at
  `internal/middleware/rbac.go:22` (`OrgHeader = "Dfe-Organization-Pk"`). The UI sends the same constant from
  `ui/src/lib/api/client.ts:50`. Never rename either side independently.
- **Route-level guards**: `perm.Require("perm.string")` enforces a granular permission;
  `perm.RequireOwner()` / `RequireOwnerOrAdmin()` gate visibility-sensitive actions (`rbac.go:118-125`). OWNER/ADMIN
  bypass the permission-string check; other roles use
  `role.permissions ∪ membership extras` (`rbac.go:137-138`).
- **Errors**: every route returns RFC 7807 Problem JSON via `sendProblem(c, err)`
  (`helpers.go:46`); services return `*problem.Problem` (`problem.BadRequest`,
  `problem.NotFound`, `problem.InternalServer`, …). No raw `fiber.Map` / `fiber.NewError`.

Permission strings follow `<action>.<resource>` (e.g. `create.nfes`,
`list.organization_products`, `update.organization_nfe_configs`). The set lives in the seeded RBAC roles (`app.go:86`
`seedRoles` → `repositories.SystemRoles()`).

---

## 3. Endpoint reference

All paths are relative to `/v1.0`. Auth columns: **JWT** = bearer token required; **Org** = `Dfe-Organization-Pk` header
required; **Perm** = permission string enforced.
`sendPage` envelopes lists in `PaginatedResponse` (`helpers.go:25-32`, cursor pagination).
`resolveActor` attributes every mutation to the caller (`helpers.go:62`).

### 3.1 Auth & user

| Method | Path                          | JWT | Org | Perm | Notes                                                                                                 |
|--------|-------------------------------|-----|-----|------|-------------------------------------------------------------------------------------------------------|
| GET    | `/auth/me`                    | ✓  | —   | —    | Provisions local user row on first login; returns `{user_id, organizations[], ...}` (`auth.go:15-31`) |
| GET    | `/auth/roles`                 | ✓  | —   | —    | Lists built-in RBAC roles (`auth.go:41-58`)                                                           |
| POST   | `/auth/terms-addendum/accept` | ✓  | —   | —    | Marks terms accepted; clears `terms_addendum_accepted` gate in UI (`auth.go:33-39`)                   |

### 3.2 Organizations (`organizations.go`)

| Method          | Path                                                                            | Org | Perm                                           | Side-effects                                                                                                                                                                                       |
|-----------------|---------------------------------------------------------------------------------|-----|------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| GET             | `/organizations`                                                                | —   | —                                              | Lists orgs the caller is a member of (uses membership, not tenant header) (`organizations.go:32-54`)                                                                                               |
| GET             | `/organizations/certificate-requirement?cpf_or_cnpj=`                           | —   | —                                              | Whether an A1 cert must be uploaded or can be inherited from a matriz (`organizations.go:59-70`)                                                                                                   |
| POST            | `/organizations`                                                                | —   | —                                              | **multipart** `data` (JSON) + optional `file` (PFX) + `password`. KYC: cert required unless inheriting matriz cert. Caller becomes OWNER. Invalidates `/auth/me` cache (`organizations.go:76-110`) |
| GET             | `/organizations/:org_pk`                                                        | ✓  | `get.organizations`                            | —                                                                                                                                                                                                  |
| PUT             | `/organizations/:org_pk`                                                        | ✓  | `update.organizations`                         | Enforces PJ fields (`RequirePJFields`) (`organizations.go:125-158`)                                                                                                                                |
| POST            | `/organizations/:org_pk/authorized-viewers`                                     | ✓  | `update.organizations`                         | Adds SEFAZ autXML viewer (CNPJ) (`organizations.go:162-174`)                                                                                                                                       |
| DELETE          | `/organizations/:org_pk/authorized-viewers/:cpf_cnpj`                           | ✓  | `update.organizations`                         | Removes autXML viewer (`organizations.go:176-183`)                                                                                                                                                 |
| GET/PUT         | `/organizations/:org_pk/nfe-config` `/nfce-config` `/cte-config` `/mdfe-config` | ✓  | `get/update.organization_{...}_configs`        | Single fiscal-config sub-resource per doc type (`helpers.go:321-342`)                                                                                                                              |
| GET/POST/DELETE | `/organizations/:org_pk/certificates` `[/:md5]`                                 | ✓  | `list/create/delete.organization_certificates` | Upload PFX (multipart `file`+`password`), list, delete (`organizations.go:201-233`)                                                                                                                |

### 3.3 Members & invitations (`members.go`, `invitations.go`)

| Method | Path                                           | Org      | Perm        | Notes                                                                                       |
|--------|------------------------------------------------|----------|-------------|---------------------------------------------------------------------------------------------|
| GET    | `/organizations/:org_pk/members`               | ✓       | OWNER/ADMIN | List members (`members.go:22-28`)                                                           |
| DELETE | `/organizations/:org_pk/members/:user_id`      | ✓       | OWNER       | Cannot remove self (`members.go:31-42`)                                                     |
| PUT    | `/organizations/:org_pk/members/:user_id/role` | ✓       | OWNER       | Body `{role: ADMIN\|USER\|VIEWER}` (`members.go:45-54`)                                     |
| GET    | `/organizations/:org_pk/invitations`           | ✓       | OWNER/ADMIN | Pending invitations (`members.go:57-63`)                                                    |
| POST   | `/organizations/:org_pk/invitations`           | ✓       | OWNER/ADMIN | Creates invite; raw `token` returned here only (`members.go:66-78`)                         |
| DELETE | `/organizations/:org_pk/invitations/:id`       | ✓       | OWNER/ADMIN | Revoke (`members.go:81-86`)                                                                 |
| GET    | `/invitations/:token`                          | ✓ (JWT) | —           | Non-consuming preview (outside org RBAC; invitee not yet a member) (`invitations.go:17-24`) |
| POST   | `/invitations/:token/accept`                   | ✓ (JWT) | —           | Joins org, invalidates `/auth/me` (`invitations.go:27-36`)                                  |
| POST   | `/invitations/:token/decline`                  | ✓ (JWT) | —           | Refuses (`invitations.go:39-44`)                                                            |

### 3.4 CRUD resources (products, persons, vehicles)

Mounted via `mountCRUD` (`crud_handlers.go:59`) → `GET list`, `POST`, `GET /:param`,
`PUT /:param`, `DELETE /:param`. All require the org header and per-action permission.

| Resource | Path        | Detail param | Permissions                                                                 |
|----------|-------------|--------------|-----------------------------------------------------------------------------|
| Products | `/products` | `product_id` | `list/create/get/update/delete.organization_products` (`products.go:14-54`) |
| Persons  | `/persons`  | `cpf_cnpj`   | `…organization_persons`; enforces PJ fields (`persons.go:14-81`)            |
| Vehicles | `/vehicles` | `sk`         | `…organization_vehicles` (`vehicles.go:29-75`)                              |

Vehicle extras:

- `GET /vehicles/:sk/requirements?doc_type=&role=` — missing-required-fields check for MDF-e/NFe/CTeOS tractor/trailer
  (`vehicles.go:79-91`). `doc_type ∈ {mdfe,nfe,cte_os}`,
  `role ∈ {tractor,trailer}` (`vehicles.go:15-25`).

List query params (all): `cursor`, `limit` (default 50), `sort` (asc/desc). Resource filters: products `description`,
`code`,`order_by`; persons `name`; vehicles `plate`,`role`.

### 3.5 Fiscal documents (NF-e / NFC-e / MDF-e)

NF-e, NFC-e, and MDF-e emission builds the immutable `WorkerMessage` before persistence and writes it to
`${TABLE_PREFIX}_worker_outbox` in the same DynamoDB transaction as the document and fiscal-number reservation. The
response includes the durable `operation_id`; a DynamoDB Stream publisher forwards the command to the SNS event bus
(`WorkerTopicARN` = `${env}-ctech-dfe`). Cancellation/event routes still call
`WorkerService.PublishWorkerEvent` directly because their event state is created by the worker. API returns `201`
with `status: pending` — the SEFAZ result arrives asynchronously and is pushed over WebSocket (see §4). **CT-e has no
emission route in this API** (inbound/distribution only).

#### NF-e — `/nfes` (`nfes.go`)

| Method | Path                                     | Perm                | Notes                                                  |
|--------|------------------------------------------|---------------------|--------------------------------------------------------|
| POST   | `/nfes`                                  | `create.nfes`       | Emit; body `nfesvc.NfeEmitBody`                        |
| GET    | `/nfes`                                  | `list.nfes`         | Filters `incoming`,`number`,`year`,`month`,`day`       |
| GET    | `/nfes/:access_key`                      | `get.nfes`          | 404 → `ErrNFeNotFound`                                 |
| GET    | `/nfes/:access_key/xml`                  | `get.nfes`          | Signed XML download                                    |
| GET    | `/nfes/:access_key/danfe`                | `get.nfes`          | PDF via py-dfe `GerarDanfe` (cancelled flag honored)   |
| POST   | `/nfes/:access_key/cancel`               | `delete.nfes`       | Body `CancelEventBody{justification, sequence_number}` |
| POST   | `/nfes/:access_key/correction-letter`    | `create.nfe_events` | CC-e; `correction_text` 15–1000 chars                  |
| POST   | `/nfes/:access_key/manifestation`        | `create.nfe_events` | `event_type ∈ {210200,210210,210220,210240}`           |
| GET    | `/nfes/:access_key/events`               | `get.nfe_events`    | Event history                                          |
| GET    | `/nfes/:access_key/events/:event_sk/xml` | `get.nfe_events`    | Event XML                                              |

#### NFC-e — `/nfces` (`nfces.go`)

Same shape as NF-e (modelo 65) but reuses `NfeStatus`/`NfeListOut` (no separate status enum). | Method | Path | Perm |
Notes | |--------|------|------|-------| | POST | `/nfces` | `create.nfces` | Emit `nfesvc.NfceEmitBody` | | GET |
`/nfces` | `list.nfces` | list | | GET | `/nfces/:access_key` `[/xml]` | `get.nfces` | detail / signed XML | | GET |
`/nfces/:access_key/danfce` | `get.nfces` | DANFC-e PDF via py-dfe | | POST | `/nfces/:access_key/cancel` |
`delete.nfces` | cancellation | | POST | `/nfces/:access_key/substitute` | `delete.nfces` | cancellation-by-substitution
(event 110112); body `SubstituteCancelBody` | | GET | `/nfces/:access_key/events` `[/:event_sk/xml]` |
`get.nfce_events` | event history / XML |

#### MDF-e — `/mdfes` (`mdfes.go`)

| Method | Path                                           | Perm                 | Notes                                                |
|--------|------------------------------------------------|----------------------|------------------------------------------------------|
| POST   | `/mdfes`                                       | `create.mdfes`       | Emit `mdfesvc.MdfeEmitBody`                          |
| POST   | `/mdfes/cargo-preview`                         | `create.mdfes`       | Parses referenced docs → cargo data (no persistence) |
| GET    | `/mdfes`                                       | `list.mdfes`         | list                                                 |
| GET    | `/mdfes/:access_key` `[/xml]` `/damdfe`        | `get.mdfes`          | detail / XML / DAMDFE PDF                            |
| POST   | `/mdfes/:access_key/cancel`                    | `delete.mdfes`       | cancellation                                         |
| POST   | `/mdfes/:access_key/close`                     | `create.mdfe_events` | encerramento; `ibge_code`+`uf` required              |
| POST   | `/mdfes/:access_key/include-condutor`          | `create.mdfe_events` | add driver (CPF)                                     |
| POST   | `/mdfes/:access_key/include-dfe`               | `create.mdfe_events` | add referenced DFe                                   |
| GET    | `/mdfes/:access_key/events` `[/:event_sk/xml]` | `get.mdfe_events`    | event history / XML                                  |

### 3.6 Distributions (`distributions.go`)

Inbound DFe from SEFAZ (NFe/CTe/MDFe **Distribuição DF-e**). Triggered by
`POST .../sync` (enqueue) or the `distribution-dispatcher` Lambda schedule. Doc types supported here: `nfe`, `cte`,
`mdfe` (see worker divergence **H1**: NFC-e not distributed).

| Method | Path                                        | Perm                              | Notes                                                                                                  |
|--------|---------------------------------------------|-----------------------------------|--------------------------------------------------------------------------------------------------------|
| GET    | `/distributions/:doc_type/history`          | `list.{doc_type}_distributions`   | Paginated incoming DFe                                                                                 |
| POST   | `/distributions/:doc_type/sync`             | `create.{doc_type}_distributions` | Enqueues a sync job to `DistributionQueueURL` (SQS); returns `202 Accepted` (`distributions.go:32-38`) |
| GET    | `/distributions/:doc_type/history/:nsu/xml` | `get.{doc_type}_distributions`    | XML download, `Content-Disposition NSU_*.xml`                                                          |
| GET    | `/distributions/:doc_type/nsu/:nsu`         | `get.{doc_type}_distributions`    | Lookup by NSU                                                                                          |
| GET    | `/distributions/:doc_type/key/:access_key`  | `get.{doc_type}_distributions`    | Lookup by access key                                                                                   |

Permissions use `RequireDynamic("list.%s_distributions", "doc_type")`
(`distributions.go:19`, `rbac.go:59-63`).

### 3.7 External (`external.go`)

| Method | Path                                           | Perm                       | Notes                                                     |
|--------|------------------------------------------------|----------------------------|-----------------------------------------------------------|
| GET    | `/external/lookup-organizations?cpf_cnpj=&uf=` | `get.organization_persons` | SEFAZ `ConsultaCadastro` via py-dfe; both params required |

### 3.8 Audit logs (`audit_logs.go`)

| Method | Path          | Perm        | Notes                                                      |
|--------|---------------|-------------|------------------------------------------------------------|
| GET    | `/audit-logs` | OWNER/ADMIN | Filters `resource_type`,`resource_id`,`user_id`; paginated |

### 3.9 WebSocket (`ws.go`)

`GET /ws?org_pk=<pk>` (Upgrade). **JWT is NOT in the query string** (leaks to LB/CF logs); the client sends
`{"token":"…"}` as the first post-upgrade frame (`ws.go:79-95`). Org membership is re-checked every heartbeat tick so a
revoked member stops receiving events (`ws.go:128-135`). Messages broadcast: `connected`, `pong`, `error`, and the
result frames described in §4.

### 3.10 Health (`health.go`)

`GET /health-check` (no auth). Checks DynamoDB, S3, SNS (event bus), SQS (results + distribution), cache, CPU, memory.
Overall `pass`/`warn`/`fail` (207 on warn, 503 on fail). This is the ALB/health-check target
(`healthCheckPath: /v1.0/health-check` in CDK).

---

## 4. Async flow & side-effects

1. **Emit** → service builds `WorkerMessage`; document/config/outbox commit atomically → DynamoDB Stream invokes
   `outbox-publisher` → **SNS `${env}-ctech-dfe`**. A publish failure leaves the immutable outbox row pending and causes
   stream redelivery. **Cancel / event** messages currently publish directly with
   `WorkerService.PublishWorkerEvent`. Worker Lambda (s) conditionally claim the document/event with an owner and
   six-minute lease, call SEFAZ (go-dfe in-process or py-dfe Lambda fallback), and allow only the lease owner to
   finalize. Retryable infrastructure failures release the lease and return an SQS batch failure.
2. **Distribution sync** → `DistributionService.EnqueueSync` sends to **SQS
   `DistributionQueueURL`** (`services/distributions.go:155`). The `distribution-dispatcher`
   Lambda also enqueues per-org/per-docType jobs on a schedule.
3. **Worker → results** → worker publishes to **SNS `${env}-ctech-dfe-results`**
   (`ResultsTopicARN`). An SQS queue (`ResultsQueueURL`) subscribes; the API's
   `ResultsConsumer` polls it (`consumer/results.go:45`) and **broadcasts over WebSocket**
   to the org (`consumer/results.go:155`), invalidating the org's nfe cache.
4. **Storage**: documents/XML in S3 `S3BucketDocuments`; certs in S3 `S3BucketCerts`; all domain data in DynamoDB
   (tables in `cdk/lib/dynamodb-stack.ts`).
5. **Caching**: audit/user/role cache via Redis (`VALKEY_URL`) or in-memory fallback (`app.go:129-148`). JWKS cached 1h.

---

## 5. Known divergences (documented honestly — do NOT fix code here)

- **B4 — cert PFX password stored in plaintext.** `CertificateRepository.certFields`
  persists `password` into DynamoDB (`internal/repositories/certificates.go:30,34,41`). A `secretsmanager` client is
  constructed in `internal/awsclient/client.go:13,28,46` but **never used** for cert secrets. Flagged as a security
  debt; no code change here.
- **B14 — `distribution-dispatcher` Scan on config tables.** The dispatcher scans
  `*_organization_{nfe,cte,mdfe}_configs` to enumerate org PKs (`worker/internal/service/distribution.go` `scanOrgPKs`).
  A `Scan` in a hot path; documented as a known divergence (see `worker/README.md`).
- **Deprecated endpoint**: `GET /v1.0/distributions/nfe` does **not** exist; callers must use
  `GET /v1.0/distributions/{doc_type}/history`. Enforced by absence in code.
- **NFC-e emission status** reuses the NF-e status model (`NfeStatus`); NFC-e has no dedicated emit-status enum (UI
  shares `NfeStatusBadge`).

See root [`CONDUCT.md`](../CONDUCT.md) and [`DOCS.md`](../DOCS.md) for the full divergence register (B4, B5, B12, B14).
