# ctech-dfe UI

Next.js 16 SPA — TypeScript, ShadCN, multi-tenant. Talks to the API over `/v1.0/*` with the
`Dfe-Organization-Pk` header and OAuth PKCE. This doc is anchored to `src/...`.

A production build is `output: 'export'`, deployed to **Cloudflare Workers Static Assets**. Nothing
proxies `/v1.0/*` at the edge, so the browser calls `NEXT_PUBLIC_API_URL`
(`https://dfe-api.aoctech.app`) **cross-origin** and CORS applies; `next dev`'s `rewrites()` is the
only same-origin path left. The CSP's `connect-src` is generated from the `https://`/`wss://`
literals in `.github/workflows/frontend.yml` and is scheme-exact — an origin the app talks to but the
workflow does not name is an origin the browser refuses.

Sibling docs: [`../api/README.md`](../api/README.md) · root [`INTEGRATION.md`](../INTEGRATION.md).

Quality gates for dependency and application changes:

```bash
npm ci
npm test
npm run lint
npm run build
npm audit --omit=dev
```

The current baseline is Next.js 16.3.1 / React 19.2.8. The production
dependency audit must remain at zero known vulnerabilities.

---

## 1. Routes (`src/app/`)

Next.js App Router. **No route groups, no `[param]` segments** — doc detail pages take the
access key via query string (`?key=`). Auth is per-page via `<ProtectedRoute>`.

| Route | Component | Notes |
|-------|-----------|-------|
| `/` | `Home` (`app/page.tsx:119`) | Marketing/landing (not the app shell) |
| `/login` | `LoginInner` (`login/page.tsx:36`) | `startOAuthFlow(returnTo)` |
| `/callback` | `CallbackInner` (`callback/page.tsx:8`) | exchanges code → `router.replace(returnTo)` |
| `/dashboard` | `DashboardContent` (`dashboard/page.tsx:84`) | Authenticated home |
| `/nfe` `[/emit]` `[/detail]` | `NfesContent` (`nfe/page.tsx:494`); `NfeEmitForm` | emitidas/recebidas/transportadas + distribution tabs |
| `/nfce` `[/emit]` `[/detail]` | `NfceEmitForm` | reuses `NfeStatusBadge` (no separate status enum) |
| `/cte` `[/distributions]` | CTe list / inbound distribution | **No emit route** — CTe emission unimplemented in UI |
| `/mdfe` `[/emit]` `[/detail]` `[/distributions]` | `MdfeEmitForm` | MDF-e emit + distribution |
| `/products` `[/new]` `[/edit]` | catalog CRUD | |
| `/vehicles` `[/new]` `[/edit]` | vehicle CRUD | |
| `/persons` `[/new]` `[/edit]` | person (cliente/fornecedor) CRUD | |
| `/organizations` `[/new]` `[/edit]` | org CRUD | |
| `/members` | members | OWNER/ADMIN only (`Sidebar.tsx:91`) |
| `/certificates` `/fiscal-config` `/audit-logs` `/profile` | — | role-gated where noted |
| `/invite` `/terms-addendum[/v1]` | invitation accept / legal addendum | |

Shared chrome: `components/layout/RootLayout.tsx` (sidebar + topbar + `<main>`, applies
`data-dfe-theme`), `Sidebar.tsx`, `Topbar.tsx`.

## 2. Multi-tenancy (`Dfe-Organization-Pk`)

- Header name defined **once**: `const ORG_HEADER = 'Dfe-Organization-Pk'`
  (`src/lib/api/client.ts:50`). The API defines the same constant at
  `api/internal/middleware/rbac.go:22` — keep them in sync.
- Injected by the axios request interceptor from `localStorage[STORAGE_KEY_ORG]`
  (`client.ts:112-125`), alongside the `Bearer` token. `STORAGE_KEY_ORG = 'pydfe_org'`
  (`constants/storage.ts:3`).
- **Switching org**: `Topbar` calls `setSelectedOrg(org)`
  (`Topbar.tsx:108` → `AuthContext.tsx:56-63`, writes localStorage). Org-scoped React Query
  keys embed `orgPk` (`query-keys.ts`), so switching refetches. Org-scoped endpoints
  (organizations, fiscal-configs, certificates, members, invitations) instead pass `pk` in
  the URL (`client.ts:201-242,323-374,376-399`) and do **not** need the header.

## 3. Auth & WebSocket

### OAuth PKCE (`src/lib/auth/oauth.ts`)
- `OAuthClient` from `@aoctech/auth-client`; `redirectUri = ${origin}/callback`,
  `scope: 'openid profile'` (`oauth.ts:7-12`).
- `startOAuthFlow` / `exchangeCode` / `decodeIdToken` (reads name claims client-side to
  avoid /userinfo audience block) (`oauth.ts:18-36`).
- **Refresh token is an HttpOnly `ctech_rt` cookie**; SPA never holds it (`oauth.ts:38-47`).
- `endSessionRedirect` + `revokeToken` (`oauth.ts:56-65`).

### Session (`src/lib/context/AuthContext.tsx`)
- Bootstrap: `doRefresh()` (or `mockDoRefresh()` in mock mode) on mount
  (`AuthContext.tsx:127-154`). `handleCallback` sets token + merges id_token claims +
  `refreshUser()` (`AuthContext.tsx:118-123`).
- On 401, the axios interceptor calls the registered `_refreshFn` and re-issues
  (`client.ts:136-164`); on failure redirects to OAuth (never `/callback`).
- **Terms gate**: if `!user.terms_addendum_accepted`, `ProtectedRoute` renders
  `<TermsAddendumGate>` (`ProtectedRoute.tsx:61-63`) → `acceptTermsAddendum()` +
  `refreshUser()` (`terms-addendum-gate.tsx:17-28`).

### WebSocket (`src/lib/hooks/useRealtimeUpdates.ts`)
- URL `${base}/v1.0/ws?org_pk=${orgPk}` (`useRealtimeUpdates.ts:15-21`); `base` =
  `NEXT_PUBLIC_WS_URL || NEXT_PUBLIC_API_URL`. `NEXT_PUBLIC_WS_URL` is set explicitly per
  environment and is not optional in a deployed build: `connect-src` is scheme-exact, so without a
  `wss://` literal in `build-env-*` the socket is blocked on every page. The `API_URL` fallback is
  for local development only.
- Enabled only when token + `selectedOrg.pk` present (`:54-56`). Uses `@aoctech/ws-client`
  with `authToken` (Bearer) + `subscribeToken` (`:96-102`). Reconnects on every new access
  token (`_refreshFn` listener, `client.ts:71-78`).
- Messages: `ping`/`connected` ignored; `dfe_result` → invalidates detail/list/events queries
  for the doc type + toast (`dfe-result-toast.ts`); `new_distribution_{nfe,cte,mdfe}` →
  invalidates distribution history + toast.

## 4. Key components & hooks

- `ProtectedRoute` (`components/ProtectedRoute.tsx`): 15s debounce on OAuth redirect to avoid
  loops (`:8-25`).
- `EntityForm` (`components/EntityForm.tsx`): shared PF/PJ form for persons + orgs; Zod
  resolver switches schema (`:165-176`); CNPJ/CPF autofill via `useCnpjLookup`.
- `StatusBadge` / `StatusCell` (`components/ui/status-badge.tsx`, `components/nfe/NfeStatusBadge.tsx`,
  `components/mdfe/MdfeStatusBadge.tsx`): `StatusBadge` uses a fixed semantic palette (not
  themed); per-type wrappers add SEFAZ `sefaz_motive` on click for rejected/failed.
- Hooks (`src/lib/hooks/`): `useAuth`, `useRealtimeUpdates`, `useCnpjLookup`, `useDebounce`,
  `usePagination`, `useEntityDelete` (optimistic + undo), `useRowSelection`,
  `useSavedFilterViews`, `useKeyboardShortcuts`.

## 5. Emit forms & status badges

- Emit forms `NfeEmitForm` / `NfceEmitForm` / `MdfeEmitForm` call `emitNfe/emitNfce/emitMdfe`
  via `apiClient` + `EmitConfirmModal`. MDF-e form includes route suggestion; NFC-e includes
  `PaymentCardFields`.
- Cancellation: `CancelDfeModal` (`CANCEL_JUSTIFICATION_MIN_LENGTH`); NFC-e also
  `SubstituteModal` (event 110112). Optimistic status patch via `dfe-status.ts:17-30`.
- Status enums (`src/lib/types/api.ts`): `NfeStatus` (pending/authorized/rejected/failed/
  cancel_pending/cancelled) shared by NF-e **and** NFC-e; `MdfeStatus` adds
  close_pending/closed.

## 6. Mock / dev layer (`src/lib/mock/`)

- Gate: `MOCK_ENABLED = NEXT_PUBLIC_MOCK_API === 'true'` (`mock/env.ts:8`) — build-time
  inlined, never set in deployed builds. When enabled, `apiClient.setAdapter(mockAdapter)`
  (`mock/index.ts:13-18`); `AuthProvider` calls `mockDoRefresh()` instead of `doRefresh()`
  (`AuthContext.tsx:130`). `mock/handler.ts` routes method+path to fixtures (250ms latency);
  `?mock=error[:status]` forces errors.

## 7. Known divergences (documented honestly)

- **NFC-e has no dedicated status badge/enum** — shares NF-e `NfeStatus` (modelo 65 reuses
  the status model). UI import: `app/nfce/page.tsx:28`, `components/nfe/NfeStatusBadge.tsx`.
- **CT-e has no emit route** in the UI (inbound/distribution only). Do not claim CTe emission.
- **Org switch is implicit** — relies on `orgPk` being embedded in every query key; no
  explicit `invalidateQueries` on switch was found in `AuthContext.setSelectedOrg`.
- **`/` is marketing**, not the app shell; the authenticated entry is `/dashboard`.

See root [`CONDUCT.md`](../CONDUCT.md) / [`DOCS.md`](../DOCS.md) for the full register
(B4, B5, B12, B14).
