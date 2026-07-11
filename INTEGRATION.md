# py-dfe — Frontend-Backend Integration Guide

This document covers how ui (Next.js) authenticates against ctech-account and communicates with
api.

---

## Environment Variables

### ui

| Variable                      | Example                                   | Description                                       |
|-------------------------------|-------------------------------------------|---------------------------------------------------|
| `NEXT_PUBLIC_API_URL`         | `https://dfe-api.aoctech.app`      | api base URL                               |
| `NEXT_PUBLIC_CTECH_URL`       | `https://accounts-api.aoctech.app` | ctech-account **Go API** URL (not the Next.js UI) |
| `NEXT_PUBLIC_CTECH_CLIENT_ID` | `dfe`                                     | OAuth client_id registered in ctech               |

> `NEXT_PUBLIC_CTECH_URL` must point to the Go API backend (which serves `/v1.0/authorize`, `/v1.0/token`,
`/v1.0/revoke`),
> **not** the ctech Next.js frontend. Locally this is `http://localhost:8080`; in production
`https://accountsapi.aoctech.app`.

### api

| Variable         | Example                                                     | Description                               |
|------------------|-------------------------------------------------------------|-------------------------------------------|
| `CTECH_URL`      | `https://accounts.aoctech.app`                       | Derives `CTECH_JWKS_URL` automatically    |
| `CTECH_JWKS_URL` | `https://accounts.aoctech.app/.well-known/jwks.json` | Override only when `CTECH_URL` is not set |
| `VALKEY_URL`     | `redis://10.0.1.5:6379`                                     | Caches JWKS keys (DB 0); pub/sub (DB 1)   |

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
     │     PyDfe-Organization-Pk: ..  │                               │
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

// All calls inject Authorization and PyDfe-Organization-Pk automatically.
await apiClient.me()                       // GET /v1.0/auth/me
await apiClient.listNfes(params)           // GET /v1.0/nfes
await apiClient.emitNfe(body)              // POST /v1.0/nfes
await apiClient.cancelNfe(accessKey, body) // POST /v1.0/nfes/{key}/cancel
await apiClient.getAuditLogs(params)       // GET /v1.0/audit-logs (OWNER/ADMIN only)
```

**`ORG_HEADER`** (`'PyDfe-Organization-Pk'`) is defined once in `client.ts`. Never hardcode this string elsewhere.

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
`preferred_username`→`username`) and uses those as the user's name. `GET /auth/me` supplies **organizations and
email** only; its name fields are a fallback. The DFe access token's `aud` is the DFe API, so ctech-account's
`/userinfo` rejects it — the id_token (aud = the OAuth client) is the only profile source the UI can read. A fresh
id_token arrives only on login (not on refresh); `refresh_token` lives in `sessionStorage`, so each new browser
session re-logs in and reflects a name changed in ctech-account.

---

## Organization Context

Every API call to api that requires an org scope sends the `PyDfe-Organization-Pk` header. The active org is:

1. Set in `AuthContext.setSelectedOrg(org)` — persisted to localStorage `pydfe_org`
2. Injected by the Axios request interceptor from localStorage on every request
3. Restored on page reload from `pydfe_user.organizations` cross-referenced with `pydfe_org`

If the user belongs to only one org, it is auto-selected on login. If multiple orgs exist, the stored org is
re-validated against the current `GET /auth/me` response.

---

## WebSocket (real-time updates)

**Location:** `src/lib/hooks/useWebSocket.ts`, `src/lib/providers/RealtimeProvider.tsx`

api exposes a WebSocket endpoint (`/v1.0/ws`) for real-time document status updates. The client connects
after login and subscribes per org. The access token is sent in the WebSocket handshake. On token expiry, the
connection is re-established after a silent refresh.

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

`ApiError` in `client.ts` wraps these with `status` and `detail` fields. Components should catch `ApiError` and
display `err.detail` to the user.

**Request-body validation (422).** Mutating endpoints validate the body strictly server-side
(unknown JSON fields are rejected with 400; invalid values return 422). A 422 from
`type: "/problems/validation-error"` carries a field-level `errors` array so the UI can map each
message back to its input:

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

`field` is a dotted JSON path (with `[i]` for array items). These server rules mirror the
frontend Zod schemas, so a payload that passes client validation should pass the server too —
the server is the authoritative gate. Send the documented contract exactly (`ui/src/lib/types/api.ts`):
do not include UI-only fields (e.g. `tipo`) or send `cpf_or_cnpj` in a partial PUT body.

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

Programmatic clients do not send raw API keys to the dfe API. Keys are exchanged at
ctech-account for a short-lived RS256 access token, and the dfe API keeps validating
only JWTs via JWKS — it never sees, stores, or looks up API keys:

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

- Re-exchange when the token expires (~15 min). Revoking the key at
  accounts stops new exchanges immediately; outstanding tokens die within the TTL.
- The token's `aud` includes `SERVICE_AUDIENCE` only when the key carries at least
  one `dfe:*` scope.

### Scope claim enforcement (REQUIRED — currently missing)

`middleware.Auth` today extracts only `sub` and ignores the `scope` claim. Every
authenticated caller therefore gets whatever its org RBAC allows, regardless of the
scopes granted to the token/key. To make scoped API keys (and future third-party OAuth
apps) meaningful, `PermChecker.Require(perm)` must ALSO check the token scopes:

- Mapping: `dfe:{resource}:read` covers `get.{resource}` + `list.{resource}`;
  `dfe:{resource}:write` covers `create/update/delete.{resource}` (nfes/nfces/mdfes
  write also covers their `*_events` permissions).
- Effective permission = token scopes ∩ org RBAC role. Missing scope → 403,
  even for org owners.
- Sessions from the first-party ui receive the full identity scopes and should be
  treated as unrestricted (backwards compatible); enforcement applies when the token
  carries `dfe:*` service scopes.

### Scope catalog

The list of grantable scopes lives in **ctech-account** (`internal/scopes/catalog.go`)
and is served by `GET /v1.0/scopes`. When the dfe API gains a new resource or
permission, add the corresponding `dfe:{resource}:{read|write}` entry to that catalog
in the ctech-account repo — otherwise users cannot select it for API keys or OAuth
clients. Keep the RBAC ↔ scope mapping above in sync.
