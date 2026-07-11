# AGENTS.md — ui

Next.js 16 frontend — TypeScript, Tailwind CSS 4, ShadCN, React 19.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §5`, `../INTEGRATION.md`, `../THEME.md`.

---

## Role

SaaS web application for issuing and managing Electronic Tax Documents. Authenticates via OAuth 2.0
PKCE (ctech-account), communicates with the Go API, and shows real-time document status via WebSocket.

---

## Directory Structure

```
ui/
├── src/
│   ├── app/                    # Next.js pages (App Router)
│   │   ├── login/              # Auth entry point
│   │   ├── callback/           # OAuth callback handler
│   │   ├── dashboard/
│   │   ├── nfe/ nfce/ cte/ mdfe/
│   │   ├── products/ vehicles/ persons/
│   │   ├── certificates/
│   │   └── fiscal-config/
│   ├── components/
│   │   └── ui/                 # Shared ShadCN + custom components
│   ├── lib/
│   │   ├── api/client.ts       # ApiClient — Axios wrapper (ORG_HEADER defined here)
│   │   ├── auth/               # OAuth PKCE flow
│   │   ├── hooks/              # React hooks (useWebSocket, etc.)
│   │   ├── providers/          # Context providers
│   │   └── schemas/            # Zod schemas (mirror backend)
│   └── __tests__/ test/
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing components/hooks/schemas before creating new ones.
3. Plan → Implement → **Run ESLint → Run tests**.
4. Update `../DOCS.md` for new API integrations; `../CONDUCT.md` for new constraints.
5. State cross-project impact (ui ↔ api ↔ INTEGRATION.md).
6. Suggest Conventional Commit.

---

## Engineering Rules

### ESLint (MUST pass before any commit)

```bash
npx eslint src --ext .ts,.tsx
```

**Zero errors, zero warnings.** Fix all reported issues before committing. No `// eslint-disable`
unless the rule genuinely does not apply and the reason is commented.

### DRY

- Never duplicate components, hooks, or API client methods.
- Before creating any component, search `src/components/` for an existing one.
- Do NOT add API client methods that differ only by a fixed argument — make the argument a
  parameter instead (e.g., `listDistributions(docType, ...)` not separate `listNfeDistributions`,
  `listCteDistributions`).
- If two pages share the same layout pattern, extract a shared layout component.

### Constants — no magic strings

- `ORG_HEADER` (`'PyDfe-Organization-Pk'`) is defined **once** in `lib/api/client.ts`.
  Never hardcode this string in any other file.
- localStorage/sessionStorage keys (`pydfe_rt`, `pydfe_user`, `pydfe_org`) must be defined as
  named constants — not inline string literals scattered across files.
- API base URL, OAuth client ID, and CTECH URL come from env vars — never hardcoded.

### Authentication (critical rules)

- `access_token` is stored in **module-level memory only** — NEVER write it to localStorage
  or sessionStorage.
- `refresh_token` lives in sessionStorage key `pydfe_rt` — cleared on logout/tab close.
- User data and active org are in localStorage (`pydfe_user`, `pydfe_org`).
- Silent refresh happens via `doRefresh()` on any 401 — implemented once in `ApiClient`.
- Do NOT add a second refresh mechanism anywhere else.

### Loading States (mandatory)

- Every API call must show a loading state. No blank/flickering UI during async operations.
- Use **skeletons** for initial page load and inline content loading.
- Use **spinners or progress indicators** for user-triggered actions (form submit, button click).
- Background refetches (filter changes on already-loaded lists) must show a subtle indicator
  (opacity dimming, spinner in pagination bar).

### Input Debouncing (mandatory)

- All inputs that trigger API calls must debounce the `onChange` callback.
- Use `DebouncedInput` (`@/components/ui/debounced-input`) for text inputs.
- Use `debounceMs` prop on `NumericInput` (`@/components/ui/numeric-input`) for numeric inputs.
- Default debounce: **300 ms**.

### No Deprecated Endpoints

- Do NOT call `GET /v1.0/distributions/nfe` — use `GET /v1.0/distributions/{doc_type}/history`
  or `POST /v1.0/distributions/{doc_type}/sync`.
- Always check `../DOCS.md` for current endpoint list before adding any new API call.

### Layer Rules

- All API calls go through `ApiClient` — never use `fetch` or raw axios directly.
- Zod schemas in `lib/schemas/` mirror backend validation — do not duplicate validation logic
  in individual components.
- Context providers in `lib/providers/` manage global state — do not duplicate state in pages.

### Secrets

Never commit: OAuth tokens, API keys, real CPF/CNPJ, real customer data.

---

## Testing

| Change             | Required                              |
|--------------------|---------------------------------------|
| New component      | Component test (Vitest + RTL)         |
| New hook           | Hook test                             |
| Auth flow          | Integration test                      |
| API client method  | Unit test (mock axios)                |
| Form validation    | Unit test (Zod schema)                |
| Core function      | Integration test                      |
| Bug fix            | Regression test                       |

**Every core function (auth flow, API client, data transforms) must have an integration test.**

Run: `npm test` from `ui/`.

---

## Theme

Primary color: `#50ba95` (soft green). Full palette in `../THEME.md`. Use ShadCN components and
Tailwind CSS 4 — do not add custom CSS frameworks. Responsive design (mobile/tablet/desktop) is mandatory.

---

## Known Constraints

- Auth is OAuth 2.0 PKCE — `login()` redirects to ctech-account; `/callback` exchanges the code.
- `ORG_HEADER` is injected by the Axios interceptor on every request — never pass it manually.
- Multiple org membership: stored org is re-validated against `GET /auth/me` on mount.
- WebSocket reconnects after silent refresh — handled in `useWebSocket` hook.
- UI validation intentionally duplicates backend validation for UX — this is by design.

---

## Critical Areas (require analysis before touching)

- OAuth PKCE flow (`lib/auth/oauth.ts`, `lib/providers/AuthContext`)
- Token storage rules (access_token memory, refresh_token sessionStorage)
- ApiClient silent refresh and org header injection
- NF-e issuance form (fiscal rules validation)

Before touching: identify risks + side effects, verify backward compatibility.

---

## Completion Checklist

- [ ] `npx eslint src --ext .ts,.tsx` passes with zero errors/warnings
- [ ] `npm test` passes
- [ ] No duplicate components, hooks, or API methods introduced
- [ ] All constants named (no magic strings)
- [ ] Loading states present for all async operations
- [ ] Inputs triggering API calls are debounced (300 ms)
- [ ] No deprecated endpoints called
- [ ] `access_token` never written to localStorage/sessionStorage
- [ ] Docs updated (`../DOCS.md`, `../INTEGRATION.md`, or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (ui ↔ api)
