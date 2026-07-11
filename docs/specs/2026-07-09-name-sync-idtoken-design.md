# Design — Sync user name from ctech-account via id_token profile claims

**Date:** 2026-07-09
**Scope:** `ui/` only. No backend (`api`/`worker`) changes.

## Problem

A user who changes their name in ctech-account (the OIDC provider) does not see it
reflected in ctech-dfe. `GET /v1.0/auth/me` returns `first_name`/`last_name` read from the
internal DynamoDB `users` table (`api/internal/services/users.go:178-179`), which is never
refreshed against ctech-account (its only writer, `GetOrCreate`, has no callers).

## Constraints discovered

- The DFe access token carries `aud = https://dfe-api.aoctech.app` (`SERVICE_AUDIENCE`).
  ctech-account's `/v1.0/userinfo` rejects a token whose audience is the DFe API.
  → A backend or UI call that forwards the user's access token to `/userinfo` is **not viable**.
- The DFe API has **no M2M / client-credential** path to authenticate to ctech-account
  (no `client_secret`/`client_credentials` anywhere in `api/`). → Backend cannot look the
  user up server-side.
- The UI's PKCE flow requests `scope=openid profile` (`ui/src/lib/auth/oauth.ts:31`), so
  ctech-account returns an **id_token containing profile claims** (`preferred_username`,
  `given_name`, `family_name`). The UI currently **discards** it (`oauth.ts:74-79`).
  The id_token's `aud` is the `dfe` OAuth client itself → no audience problem; it is the
  correct token for reading the user's profile client-side.
- ctech-account issues a fresh id_token **only on the `authorization_code` grant (login)**,
  not on `refresh_token` grant. `refresh_token` lives in `sessionStorage`
  (`SESSION_KEY_REFRESH`), cleared on tab close, so every new browser session performs a full
  login → fresh id_token. Name is therefore stale only *within* one long-lived session and
  refreshes each new session.

## Chosen approach — Option D: UI reads name from the id_token

Split the source of truth:

- `GET /v1.0/auth/me` → **organizations, email, user_id** (unchanged; still the source for orgs).
- **name** (`first_name`, `last_name`, `username`) → decoded from the id_token profile claims.

The backend response is left untouched; its `first_name`/`last_name` fields become a **fallback**
used only when the id_token is absent or cannot be decoded.

### Rejected alternatives

- **B — backend `/auth/me` calls `/userinfo`:** blocked by audience; backend has no M2M creds.
- **C — UI calls `/userinfo` directly:** same audience block; also splits profile across two
  backends and needs CORS.

## Changes (all in `ui/`)

### 1. `src/lib/auth/oauth.ts`

- Export `IdTokenClaims` = `{ username?; first_name?; last_name? }`.
- Add `decodeIdToken(idToken: string): IdTokenClaims | null`:
  - Split JWT, base64url-decode the payload segment (pad to length % 4, `-`→`+`, `_`→`/`),
    UTF-8-decode (accented Brazilian names), `JSON.parse`.
  - Map `given_name`→`first_name`, `family_name`→`last_name`, `preferred_username`→`username`.
  - Return `null` on any parse failure or when no name field is present.
- `exchangeCode` returns `idToken: string | null` (`data.id_token ?? null`) alongside the
  existing fields.
- `doRefresh`: unchanged (refresh response carries no id_token).

### 2. `src/app/callback/page.tsx`

Thread `idToken` from `exchangeCode` into `handleCallback`.

### 3. `src/lib/context/AuthContext.tsx`

- `handleCallback(accessToken, refreshToken, idToken: string | null)`: decode the id_token to
  `IdTokenClaims` and pass them into `refreshUser`.
- `refreshUser(nameClaims?: IdTokenClaims | null)`: after `apiClient.me()`, apply name via
  priority:
  1. `nameClaims` (fresh id_token from this login), else
  2. name from the currently cached `pydfe_user` (id_token-derived from an earlier login this
     session), else
  3. backend `me()` name (fallback).
  Store the merged object in state and `pydfe_user`. Organization-selection logic unchanged
  (still keyed off `data.organizations`).
- `AuthContextType.handleCallback` signature updated.

`MeResponse` type is unchanged — `first_name`/`last_name`/`username` remain; consumers
(Topbar, dashboard, profile) are untouched.

### Name is not a credential

The id_token itself is **not** persisted (kept only long enough to decode). The decoded name
is stored in `pydfe_user` (localStorage) exactly as today — name is profile data, not a token,
so this does not violate the access-token storage rule.

## Testing

- Unit `decodeIdToken`: valid token → mapped claims; malformed/empty payload → `null`;
  token with no name claims → `null`; accented UTF-8 name decoded correctly.
- Integration (AuthContext): login sets name from id_token; background `refreshUser` (no
  id_token) preserves the cached name and does not overwrite it with the backend's stale name;
  decode-failure falls back to backend `me()` name.

## Docs to update

- `INTEGRATION.md` and `DOCS.md §5`: name is sourced from id_token `profile` claims;
  `/auth/me` name fields are fallback only.
- `CONDUCT.md`: record the audience constraint (DFe-aud token cannot call ctech-account
  `/userinfo`) so future work does not attempt options B/C.

## Cross-project impact

`ui` only. No `api`/`worker`/`cdk`/`py-dfe` change. No API contract change.
