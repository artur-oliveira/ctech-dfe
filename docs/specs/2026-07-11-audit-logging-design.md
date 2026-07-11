# Audit Logging — Design

**Date:** 2026-07-11
**Status:** Approved (pending final review)

## Problem

Mutating actions (POST/PUT/DELETE) are not auditable even when a user has permission to perform
them. There's no record of who issued/canceled a fiscal document, or who changed a product,
supplier, vehicle, or company/fiscal-config data.

## Goals

- Every user-initiated mutation is attributable to a `user_id` + `user_name`.
- Deterministic, append-only actions (DF-e issuance, DF-e events) record the actor directly on
  the record — no separate audit row needed, since the record itself never changes after write.
- Actions that mutate existing state (company data, certificates, products, vehicles, persons,
  fiscal configs) get a durable, queryable change record in a new `audit_logs` table, with a
  per-field before/after diff.
- Audit writes cannot silently fail while the underlying mutation succeeds.

## Non-goals

- No retroactive backfill of history for existing records.
- No UI change beyond one new org-wide Audit Log page (no per-resource "history" tabs in this task).
- No cross-org / platform-wide audit view (org-scoped only, per existing multi-tenancy model).
- `worker`'s automatic supplier creation and the automatic fiscal-number increment are addressed
  as documented below, not left ambiguous.

## Data model

### New table: `{TABLE_PREFIX}_audit_logs`

Follows the same environment-prefixed naming as every other table (`cdk/lib/dynamodb-stack.ts`
builds `${tablePrefix}_${tbName}` where `tablePrefix` is `${env}_dfe`). Added to the `TableName`
union as `'audit_logs'`.

| Attribute        | Type | Notes                                                                 |
|-------------------|------|------------------------------------------------------------------------|
| `pk`              | S    | `{org_pk}` — the owning organization's PK (`CNPJ_...`/`CPF_...`)      |
| `sk`              | S    | `{resource_type}#{resource_id}#{uuidv7}`                              |
| `resource_type`   | S    | `ORGANIZATION`, `CERTIFICATE`, `PRODUCT`, `VEHICLE`, `PERSON`, `NFE_CONFIG`, `NFCE_CONFIG`, `CTE_CONFIG`, `MDFE_CONFIG` |
| `resource_id`     | S    | The resource's own key within its table (e.g. `PRODUCT_<uuid>`, a `CERT_<timestamp>` sk, a person's `cpf_cnpj` pk, org's own pk, or the doc-type string for the singleton config tables) |
| `action`          | S    | `CREATE` \| `UPDATE` \| `DELETE`                                      |
| `modifications`   | L    | List of `{name, before, after}` — only fields that actually changed. `before`/`after` are `NULL` (DynamoDB null) for CREATE/DELETE as appropriate. |
| `user_id`         | S    | Actor's user id (JWT `sub`), or `SYSTEM` for background actions (see below) |
| `user_name`       | S    | Actor's resolved display name (see Actor resolution)                  |
| `created_at`       | S    | ISO 8601, same `NowStr()` helper used everywhere else                 |

**GSIs:**

| Index               | PK   | SK           | Purpose                                                             |
|---------------------|------|--------------|-----------------------------------------------------------------------|
| `org-time-index`    | `pk` | `created_at` | Org-wide chronological feed — powers the Audit Log UI page          |
| `user-id-index`     | `user_id` | `created_at` | "Everything user X did" across resource types                  |

The base table itself (`pk` + `sk` prefix `{resource_type}#{resource_id}#`) answers "full history
of this one resource" without a GSI.

Billing/limits match every other table in this stack: on-demand, 5 RCU/WCU base, 10 RCU/WCU per GSI.

### In-record fields (no audit_logs row)

Added directly to `nfes` / `nfces` / `ctes` / `mdfes` and `nfe_events` / `nfce_events` /
`cte_events` / `mdfe_events`:

- `user_id`
- `user_name`

These tables are already append-only/immutable per document (DOCS.md, OVERVIEW.md) — issuance
creates a new item, events append new items, nothing is ever overwritten in place. Attribution
lives on the record itself.

## Edge cases (resolved)

- **Automatic fiscal-number increment at emission** — no separate `audit_logs` row. The NF-e (or
  NFC-e/CT-e/MDF-e) record created by that same request already carries `user_id`/`user_name`,
  which fully attributes the numbering side effect. `TransactReserveAndCreate` needs no changes
  beyond the emit item already carrying the actor fields.
- **Worker auto-creates supplier (`organization_persons`) during NF-e distribution processing** —
  this has no authenticated user. It still writes an `audit_logs` row, with `user_id = "SYSTEM"`
  and `user_name = "Sistema"` (or similar constant), `action = CREATE`. This keeps the table the
  single source of "who/what created this person row" without inventing a second mechanism.

## Backend architecture (`api`)

### Actor resolution

JWT claims are `sub`, `sid`, `iat`, `exp`, `iss`, `aud`, `scope` only — no name/email. Profile data
is intentionally not duplicated locally (see `UserRepository.CreateMinimal` comment) and is
normally fetched live from ctech-account's `/v1.0/userinfo`, cached under `me:{userID}` (300s TTL)
by `UserService.GetMeData`.

New helper on `UserService`:

```go
// ResolveActor returns (userID, userName) for audit attribution: cache-first, live fallback.
func (s *UserService) ResolveActor(ctx context.Context, userID, accessToken string) (string, string)
```

- Checks the `me:{userID}` cache first (already populated whenever the user has hit `/auth/me`,
  which the frontend calls on every mount per `INTEGRATION.md`).
- On miss, calls `GetUserInfo` directly (short timeout, already used by `GetMeData`) and derives a
  display name (`profile.Name`, falling back to the `username` derivation already used in
  `GetMeData` — email local-part).
- On total failure, degrades to `userID` as the name (never blocks the mutation), consistent with
  the existing "profile fetch failure degrades to blank profile" pattern in `GetMeData`.
- Route handlers already have the raw access token (`Authorization` header) available; it's passed
  through the same way `GetMeData` already receives it in `auth.go`.

### Diffing and transactional write

New `internal/repositories/audit_logs.go` (`AuditLogRepository`, `NewBase(db, cfg, "audit_logs")`)
with one method:

```go
func (r *AuditLogRepository) BuildLogTxItem(orgPK, resourceType, resourceID, action, userID, userName string, modifications []Modification) types.TransactWriteItem
```

New `internal/services/audit.go` (`AuditService`) with a diff helper:

```go
// Diff compares old and new field maps and returns only the fields that changed.
// Housekeeping fields (pk, sk, created_at, updated_at) are always excluded.
func Diff(before, after map[string]any) []Modification
```

`Base` (repositories) gains non-executing companions to the existing `PutItem`/`UpdateItem`/
`DeleteItem`, mirroring the shape `documents.go`'s `TransactReserveAndCreate` already hand-rolls:

```go
func (b *Base) BuildPutTxItem(item map[string]types.AttributeValue) types.TransactWriteItem
func (b *Base) BuildUpdateTxItem(pk string, sk *string, updates map[string]any) (types.TransactWriteItem, error)
func (b *Base) BuildDeleteTxItem(pk string, sk ...string) types.TransactWriteItem
```

Each mutating resource service (`organizations`, `certificates`, `products`, `vehicles`,
`persons`, `fiscal_configs`) changes its `Create`/`Update`/`Delete` to:

1. For `Update`/`Delete`: `GetItem` the current state first (needed for the diff — this is a new
   read where one didn't previously exist for `Update`, since `UpdateItem` today is a blind write).
2. Resolve the actor once (`ResolveActor`, called from the route handler, passed down).
3. Build the primary-table `TransactWriteItem` (`BuildPutTxItem`/`BuildUpdateTxItem`/`BuildDeleteTxItem`).
4. Build the `audit_logs` `TransactWriteItem` (`AuditLogRepository.BuildLogTxItem`) from the diff.
5. `Base.TransactWrite(ctx, []types.TransactWriteItem{primary, audit})` — the existing generic
   executor, already used by `TransactReserveAndCreate`, needs no changes.

This makes the resource mutation and its audit row succeed or fail together — no code path can
commit one without the other.

**`organization_persons` note:** that table's own `pk` is the counterparty's `cpf_cnpj`, not the
org PK — the `org_pk` attribute already stored on each person item must be used as the
`audit_logs.pk` instead of the person record's own key.

**Fiscal configs note:** these tables have no `sk` (one item per org per doc type) — `resource_id`
is the doc-type string (`nfe_config`, `nfce_config`, `cte_config`, `mdfe_config`).

DF-e emit/cancel/correction-letter/manifestation services (`services/nfes`, `services/mdfes`, plus
NFC-e/CT-e equivalents) gain a `userID, userName string` parameter, threaded through from the
route handlers (`svc.Emit(ctx, orgPK, body, userID, userName)`), and stamp those two fields onto
the document/event item being built. No transaction changes needed there — it's a field addition,
not a new write.

### Read endpoint

`GET /v1.0/audit-logs` — new `internal/api/v1/audit_logs.go` (`RegisterAuditLogs`), header-scoped
via `middleware.GetOrgPK(c)` like `products`/`vehicles`/`persons`/`nfes`, not path-param-scoped
like the `organizations.go` sub-resources:

- Query params: `resource_type`, `resource_id`, `user_id`, `cursor`, `limit` — selects which index
  to query (base table when `resource_type`+`resource_id` given, `user-id-index` when `user_id`
  given, `org-time-index` otherwise for the default chronological feed). Same
  `sendPage`/`decodeCursor` pagination envelope as every other list endpoint.
- **Access control:** OWNER/ADMIN only. Implemented as a small check in the handler (reusing
  `middleware.UserOrgRole` + the existing `roleOwner`/`roleAdmin` constants already in
  `middleware/rbac.go`) rather than a new granular permission string — audit history is sensitive
  by nature, not something to delegate via the roles table.

## Worker changes

`persistPerson` (in `worker/internal/service/distribution.go`) gains the `AuditLogRepository` +
`Base.TransactWrite` call described above, with `user_id = "SYSTEM"`, on the create-if-absent path
only (an update to an existing counterparty is not expected here, per the existing "create-if-absent:
a manually curated person is never overwritten" comment).

## UI

One new page: `ui/src/app/audit-logs/page.tsx`, a top-level route matching the existing
`products`/`vehicles`/`persons`/`certificates` sibling pattern (org-scoped implicitly via the
active-org header, not a path param). OWNER/ADMIN-gated same as the backend, showing a
paginated, filterable (resource type, date) table of `audit_logs` entries for the active
organization. New `ApiClient.listAuditLogs(...)` method, new `AuditLogOut` type in
`lib/types/api.ts`, following the same list-page pattern as `products`/`vehicles` pages (skeleton
loading, debounced filters, cursor pagination).

## Testing

Per `CLAUDE.md` and `api/CLAUDE.md` core-function rule:

- Unit tests: `Diff` (all three actions, field exclusions), `ResolveActor` (cache hit/miss/failure
  fallback), `BuildUpdateTxItem`/`BuildPutTxItem`/`BuildDeleteTxItem`.
- Integration tests (real DynamoDB Local per existing `tests/integration` pattern): one full
  create→update→delete cycle per audited resource type verifying the transactional write lands
  both the resource change and the matching `audit_logs` row, and that a resource-side condition
  failure (e.g. concurrent delete) rolls back the whole transaction (no orphan audit row).
- Regression test: worker's `persistPerson` SYSTEM-actor audit row.

## Docs to update

- `DynamoDB-Tables.md` — new `audit_logs` table + both GSIs.
- `DOCS.md` — new endpoint, new in-record fields on DF-e/event tables.
- `INTEGRATION.md` — new `listAuditLogs` API client method.
- `CONDUCT.md` — the transactional write pattern (`BuildXxxTxItem` + `Base.TransactWrite`) as the
  required approach for any future audited mutation.
