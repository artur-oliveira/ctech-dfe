# py-dfe — Frontend-Backend Integration Guide

This document covers how ui (Next.js) authenticates against ctech-account and communicates with api.

---

## Environment Variables

### ui

| Variable                      | Example                        | Description                                               |
|-------------------------------|--------------------------------|-----------------------------------------------------------|
| `NEXT_PUBLIC_API_URL`         | `https://dfe.aoctech.app`      | api base URL — the **app** domain, empty locally          |
| `DEV_API_ORIGIN`              | `http://localhost:8000`        | dev only: where `next dev` proxies `/v1.0/*`              |
| `NEXT_PUBLIC_WS_URL`          | `http://localhost:8000`        | dev only: `next dev` does not proxy the WebSocket upgrade |
| `NEXT_PUBLIC_CTECH_URL`       | `https://accounts.aoctech.app` | ctech-account base URL (serves both its UI and `/v1.0/*`) |
| `NEXT_PUBLIC_CTECH_CLIENT_ID` | `dfe`                          | OAuth client_id registered in ctech                       |

> Browsers never call `dfe-api.aoctech.app` directly. CloudFront forwards
> `dfe.aoctech.app/v1.0/*` to the ALB, so the app is same-origin and CORS never applies;
> `next dev` reproduces that with a rewrite (`ui/next.config.ts`). `dfe-api.aoctech.app` stays
> public for the API's own clients.
>
> The same holds for `NEXT_PUBLIC_CTECH_URL`: `accounts.aoctech.app` serves the ctech-account UI
> *and* forwards `/v1.0/*` + `/.well-known/*` to its ALB. Locally, point it at
> `http://localhost:8080`.

### api

| Variable         | Example                                              | Description                               |
|------------------|------------------------------------------------------|-------------------------------------------|
| `CTECH_URL`      | `https://accounts.aoctech.app`                       | Derives `CTECH_JWKS_URL` automatically    |
| `CTECH_JWKS_URL` | `https://accounts.aoctech.app/.well-known/jwks.json` | Override only when `CTECH_URL` is not set |
| `VALKEY_URL`     | `redis://10.0.1.5:6379`                              | Caches JWKS keys (DB 0); pub/sub (DB 1)   |

---

## Authentication Flow

```
ui                   ctech-account                   api
     │                               │                               │
     │  1. login() → redirect        │                               │
     │──────────────────────────────>│                               │
     │                               │  User logs in + MFA           │
     │  2. /callback?code=...        │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │  3. POST /v1.0/token          │                               │
     │     grant_type=authorization_code                             │
     │     code_verifier=...         │                               │
     │──────────────────────────────>│                               │
     │  access_token (RS256, 15m)    │                               │
     │  refresh_token (opaque, 30d)  │                               │
     │<──────────────────────────────│                               │
     │                               │                               │
     │  4. GET /v1.0/auth/me         │                               │
     │     Authorization: Bearer ... │──────────────────────────────>│
     │                               │  verify RS256 via JWKS        │
     │                               │<──────────────────────────────│
     │  user + orgs                  │                               │
     │<──────────────────────────────────────────────────────────────│
     │                               │                               │
     │  5. API calls                 │                               │
     │     Authorization: Bearer ... │──────────────────────────────>│
     │     Dfe-Organization-Pk: ..  │                               │
```

---

## Token Storage

| Token           | Storage        | Key          | Cleared on           |
|-----------------|----------------|--------------|----------------------|
| `access_token`  | Module memory  | —            | Logout / page reload |
| `refresh_token` | sessionStorage | `pydfe_rt`   | Logout / tab close   |
| User data       | localStorage   | `pydfe_user` | Logout               |
| Selected org    | localStorage   | `pydfe_org`  | Logout               |

**Rules:**

- `access_token` is **never** written to localStorage or sessionStorage — in-memory only.
- `refresh_token` is rotated on every use (server-side reuse detection). A stale refresh token results in a 401 that
  logs the user out.

---

## Silent Refresh

`ApiClient` registers a `_refreshFn` via `registerRefreshFn()`. On any 401 response, the interceptor calls
`tryRefresh()` which does:

```typescript
// lib/auth/oauth.ts
doRefresh(refreshToken) → POST / v1
.0 / token(grant_type = refresh_token)
@ ctech
-account
  → new access_token + refresh_token
  → store
new refresh_token in sessionStorage
  → apiClient.setToken(new access_token)
  → retry
original
request
```

If `doRefresh` fails (revoked token, expired), the user is logged out and redirected to login.

---

## API Client

**Location:** `src/lib/api/client.ts`

```typescript
import {apiClient} from '@/lib/api/client'

// All calls inject Authorization and Dfe-Organization-Pk automatically.
await apiClient.me()                       // GET /v1.0/auth/me
await apiClient.listNfes(params)           // GET /v1.0/nfes
await apiClient.emitNfe(body)              // POST /v1.0/nfes
await apiClient.cancelNfe(accessKey, body) // POST /v1.0/nfes/{key}/cancel
await apiClient.getAuditLogs(params)       // GET /v1.0/audit-logs (OWNER/ADMIN only)
```

**`ORG_HEADER`** (`'Dfe-Organization-Pk'`) is defined once in `client.ts`. Never hardcode this string elsewhere.

The active org PK is read from localStorage (`pydfe_org`) on every request — no need to pass it explicitly.

**Null omission:** the request interceptor strips null fields from **POST (create)** payloads only; `PUT`/`PATCH`
keep explicit `null` (= clear the field). FormData/Blob bodies are never stripped. The backend likewise omits null
attributes from DynamoDB items, but **the API contract stays nullable** — responses still emit `null` for absent
attributes, and a field is cleared by sending `null` (persisted as a DynamoDB `REMOVE`).

---

## OAuth Callback Page

**Location:** `src/app/callback/page.tsx` (or equivalent)

```typescript
import {exchangeCode} from '@/lib/auth/oauth'

// Reads ?code= and ?state= from URL
// Validates state against sessionStorage.oauth_state
// Exchanges code via POST /v1.0/token at ctech-account
// Stores tokens; calls handleCallback(accessToken, refreshToken, idToken) from AuthContext
```

**PKCE:** `code_verifier` (64 random hex chars) stored in sessionStorage during the redirect and used in the exchange.
Never expose the `code_verifier` to the server.

**User name from id_token:** `exchangeCode` also returns the `id_token` (issued because `scope=openid profile`).
`AuthContext` decodes it with `decodeIdToken()` (`given_name`→`first_name`, `family_name`→`last_name`,
`preferred_username`→`username`) and uses those as the user's name. `GET /auth/me` supplies **organizations and email**
only; its name fields are a fallback. The DFe access token's `aud` is the DFe API, so ctech-account's
`/userinfo` rejects it — the id_token (aud = the OAuth client) is the only profile source the UI can read. A fresh
id_token arrives only on login (not on refresh); `refresh_token` lives in `sessionStorage`, so each new browser session
re-logs in and reflects a name changed in ctech-account.

---

## Organization Context

Every API call to api that requires an org scope sends the `Dfe-Organization-Pk` header. The active org is:

1. Set in `AuthContext.setSelectedOrg(org)` — persisted to localStorage `pydfe_org`
2. Injected by the Axios request interceptor from localStorage on every request
3. Restored on page reload from `pydfe_user.organizations` cross-referenced with `pydfe_org`

If the user belongs to only one org, it is auto-selected on login. If multiple orgs exist, the stored org is
re-validated against the current `GET /auth/me` response.

---

## WebSocket (real-time updates)

**Location:** `src/lib/hooks/useWebSocket.ts`, `src/lib/providers/RealtimeProvider.tsx`

api exposes a WebSocket endpoint (`/v1.0/ws`) for real-time document status updates. The client connects after login and
subscribes per org. The access token is sent in the WebSocket handshake. On token expiry, the connection is
re-established after a silent refresh.

---

## Error Handling

All API errors from api follow RFC 7807:

```json
{
  "status": 422,
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "detail": "...",
  "instance": "/v1.0/nfes"
}
```

`ApiError` in `client.ts` wraps these with `status` and `detail` fields. Components should catch `ApiError` and display
`err.detail` to the user.

**Request-body validation (422).** Mutating endpoints validate the body strictly server-side (unknown JSON fields are
rejected with 400; invalid values return 422). A 422 from
`type: "/problems/validation-error"` carries a field-level `errors` array so the UI can map each message back to its
input:

```json
{
  "type": "/problems/validation-error",
  "title": "Validation Error",
  "status": 422,
  "detail": "the request body failed validation",
  "errors": [
    { "field": "person.addresses[0].postal_code", "message": "CEP deve ter 8 dígitos", "tag": "cep" }
  ]
}
```

`field` is a dotted JSON path (with `[i]` for array items). These server rules mirror the frontend Zod schemas, so a
payload that passes client validation should pass the server too — the server is the authoritative gate. Send the
documented contract exactly (`ui/src/lib/types/api.ts`):
do not include UI-only fields (e.g. `tipo`) or send `cpf_or_cnpj` in a partial PUT body.

---

## NFS-e (frontend contract)

**Emission body** — `POST /v1.0/nfses`. One service per document (`service` is an object, not a list):

```json
{
  "tp_emit": 1,
  "competence": "05/08/2026",
  "service": { "service_id": "SERVICE_01HZ...", "value": "1000.00", "tax_rate": "5.00" },
  "customer_id": "CNPJ_12345678000195",
  "additional_info": "..."
}
```

`tp_emit` 2 (tomador) or 3 (intermediário) additionally require `motivo_emis_ti` and
`provider_person_id`. Full field table and the conditional rules: `DOCS.md` → *Emissão de NFS-e*.

**The `{id}` path parameter accepts either identifier.** `GET /v1.0/nfses/{id}` (and every
`/{id}/...` sub-route) resolves both the 45-char `id_dps` — which the emission response returns as
`sk` and which is the only identifier that exists before the fisco answers — and the 50-digit access
key, which only appears after authorization. Do not build a UI that requires the access key to open
a document.

**Outcome is asynchronous.** `POST` returns 202 with `operation_id`; the row starts as
`status: "pending"`. The final state (`authorized` / `rejected` / `cancelled`) arrives over the same
WebSocket channel as NF-e, or by polling `GET /v1.0/nfses/{id}`. NFS-e responses have no
`cStat`/`xMotivo`: a rejection is terminal and its reason arrives as the Problem `detail`.

**Event types the UI may offer** — only the 10 contributor-emittable ones (`nfse.ContribuinteEvents`):
`101101` cancelamento, `101103` solicitação de análise fiscal de cancelamento, `202201`/`203202`/`204203`
confirmação (prestador/tomador/intermediário), `202205`/`203206`/`204207` rejeição
(prestador/tomador/intermediário), `205208` anulação de rejeição. Never offer `105102` (substituição —
use `POST /v1.0/nfses/{id}/substitute`) nor the fisco-private codes `105104`, `105105`, `205204`,
`305101`–`305103`, which only arrive through ADN distribution.

**Cancellation body is different from every other document type** — `POST /v1.0/nfses/{id}/cancel`
requires **both** `reason_code` (≤2 chars, `cMotivo`) and `reason_description` (≤255 chars,
`xMotivo`); NF-e/NFC-e/MDF-e take a single free-text justification. The UI does not reuse
`CancelDfeModal` for NFS-e — see `components/nfse/NfseCancelModal.tsx`.

**Real-time updates.** The worker publishes `table_name: "nfses"` and `access_key: <id_dps>` (the
row's SK, not the fisco access key — it doesn't exist yet for `pending`/`processing` rows) on the
same `dfe_result` WebSocket channel as the other document types. `useRealtimeUpdates.ts`'s
`DOC_QUERY_KEYS` maps `nfses: queryKeys.nfses` so list/detail/event caches invalidate the same way.

**ABRASF 2.04 is configurable but not emittable from the front (F4).** `/fiscal-config`'s NFS-e tab
accepts `provider: "abrasf204"`, but `/nfse/emit` blocks submission with an explicit message when
the saved config has that provider, and DANFSE download is only offered when
`status === "authorized" && provider === "nacional"` — ABRASF 2.04 has no DANFSE proxy yet. Full
SOAP-municipal emission is F5.

---

## Local Development

```bash
# Terminal 1 — api
cd api
cp .env.example .env
# CTECH_URL=http://localhost:8080  (local ctech-account Go API)
# or CTECH_URL=https://accountsapi.aoctech.app  (prod ctech-account)
uvicorn app.main:app --reload --port 8000

# Terminal 2 — ui
cd ui
cp .env.local.example .env.local
# NEXT_PUBLIC_API_URL=http://localhost:8000
# NEXT_PUBLIC_CTECH_URL=http://localhost:8080   ← Go API, not the Next.js UI
# NEXT_PUBLIC_CTECH_CLIENT_ID=pydfe
npm run dev   # http://localhost:3000
```

The `/callback` redirect URI is built as `${window.location.origin}/callback`:

- Local dev → `http://localhost:3000/callback`
- Production → `https://dfe.aoctech.app/callback`

Both must be registered in ctech-account for the `pydfe` OAuth client (`redirect_uris` field in DynamoDB).

---

## API Key Authentication (machine-to-machine)

Programmatic clients do not send raw API keys to the dfe API. Keys are exchanged at ctech-account for a short-lived
RS256 access token, and the dfe API keeps validating only JWTs via JWKS — it never sees, stores, or looks up API keys:

```
client                        ctech-account                     api (dfe)
  │  POST /v1.0/token                │                               │
  │  grant_type=api_key              │                               │
  │  api_key=<raw key>               │                               │
  │─────────────────────────────────>│                               │
  │  access_token (15 min JWT:       │                               │
  │  sub=owner, scope=key scopes,    │                               │
  │  aud incl. SERVICE_AUDIENCE)     │                               │
  │<─────────────────────────────────│                               │
  │  Authorization: Bearer <JWT>     │                               │
  │─────────────────────────────────────────────────────────────────>│
```

- Re-exchange when the token expires (~15 min). Revoking the key at accounts stops new exchanges immediately;
  outstanding tokens die within the TTL.
- The token's `aud` includes `SERVICE_AUDIENCE` only when the key carries at least one `dfe:*` scope.

### Scope claim enforcement (IMPLEMENTED)

`Verifier.Verify` extracts the space-delimited `scope` claim alongside `sub`; the auth middleware stashes it in locals
(`middleware.GetScopes`). `PermChecker`
enforces it as **defense-in-depth on top of** the org RBAC decision (`middleware/scopes.go`, `middleware/rbac.go`):

- **Effective permission = org RBAC role ∩ token scopes.** The scope never widens what the underlying member could do —
  it only narrows it. Missing scope → 403, even for an org OWNER.
- **Identity-only sessions are unrestricted.** A token with no `dfe:*` scope (the first-party ui, which carries
  `openid profile`) skips scope enforcement — pure RBAC, backwards compatible. Enforcement kicks in only when the token
  carries at least one `dfe:*` service scope (i.e. an API key or third-party OAuth app).
- **Role-gated endpoints reject scoped tokens.** Member/invitation management and the audit trail are gated by role
  (`RequireOwner`/`RequireOwnerOrAdmin`), not by a permission string, and no scope grants them — so a scoped API-key
  token is refused there outright (403). Only a full first-party session can manage members.

Scope → RBAC mapping (`middleware/scopes.go`, `scopeFamilies`):

| Scope                             | Grants (RBAC action.resource)                                                         |
|-----------------------------------|---------------------------------------------------------------------------------------|
| `dfe:nfes:read`                   | `get`/`list` of `nfes`, `nfe_events`, `nfe_distributions`, `organization_nfe_configs` |
| `dfe:nfes:write`                  | `create`/`update`/`delete` of the same family                                         |
| `dfe:nfces:*`                     | `nfces`, `nfce_events`, `organization_nfce_configs` (NFC-e has no distributions)      |
| `dfe:ctes:*`                      | `ctes`, `cte_events`, `cte_distributions`, `organization_cte_configs`                 |
| `dfe:mdfes:*`                     | `mdfes`, `mdfe_events`, `mdfe_distributions`, `organization_mdfe_configs`             |
| `dfe:organization_products:*`     | `organization_products`                                                               |
| `dfe:organization_vehicles:*`     | `organization_vehicles`                                                               |
| `dfe:organization_persons:*`      | `organization_persons`                                                                |
| `dfe:organizations:*`             | `organizations`                                                                       |
| `dfe:organization_certificates:*` | `organization_certificates` (isolated — a doc-family `write` never grants it)         |

`read` → `get`+`list`; `write` → `create`+`update`+`delete`. A document family's scope covers its events, distributions,
and fiscal config. Certificate access is a dedicated scope (it grants the PFX + private key), never bundled into a doc
`write`.

### Scope catalog

The list of grantable scopes lives in **ctech-account** (`internal/scopes/catalog.go`)
and is served by `GET /v1.0/scopes`. When the dfe API gains a new resource or permission, add the corresponding
`dfe:{resource}:{read|write}` entry to that catalog in the ctech-account repo — otherwise users cannot select it for API
keys or OAuth clients. Keep the RBAC ↔ scope mapping above in sync.
