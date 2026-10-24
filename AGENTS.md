# AGENTS.md — ctech-dfe (monorepo root)

## CTech Standards (mandatory)

Before writing, changing or reviewing any code or infra here, load the `ctech-dev-standards` skill (plugin
`ctech-skills`, repo github.com/artur-oliveira/ctech-skills). It is the single source of truth for backend,
frontend and infra (patterns, resilience, scale, cost). If anything in this file contradicts it, the skill
wins and this file must be fixed. Install on a new machine:
`/plugin marketplace add artur-oliveira/ctech-skills` then `/plugin install ctech-skills@ctech-skills`.

Brazilian tax SaaS (NF-e, NFC-e, CT-e, MDF-e, NFS-e) — direct SEFAZ communication via SOAP + mTLS (NFS-e: national REST), always through the go-dfe-egress Lambda.

**Before any task:** Read `OVERVIEW.md`. For cross-project changes also read `CONDUCT.md` and `DOCS.md`.

---
- **Standards gate:** `scripts/standards-check.sh` (canonical copy in ctech-skills, identical bytes; CI job "Family standards"; reads `FAMILY_GOCOMMON_MINOR`, default v1.38, bump it on a go-common minor). Debt hidden: none via STANDARDS_SKIP; one-shot migration/report Scans are allow-listed. One-off lines carry a trailing `standards:allow <reason>`.

## Projects

| Project   | Role                                                   | Full guidelines    |
|-----------|--------------------------------------------------------|--------------------|
| `api/`    | Go REST API — Fiber v3, multi-tenant, DynamoDB         | `api/CLAUDE.md`    |
| `worker/` | Go Lambda — standard SQS consumer, DFe pipeline        | `worker/CLAUDE.md` |
| `ui/`     | Next.js 16 static-export frontend (Cloudflare) — TypeScript | `ui/CLAUDE.md` |
| `cdk/`    | AWS CDK infrastructure — TypeScript (frozen; see skill) | `cdk/CLAUDE.md`    |
| `go-dfe/` | Go lib — SEFAZ SOAP+mTLS and NFS-e REST client (runs inside go-dfe-egress) | `go-dfe/CLAUDE.md` |
| `go-dfe-egress/` | Go Lambda (sa-east-1) — runs `dfe.Call` so SEFAZ/prefeitura calls leave from Brazil | `go-dfe-egress/CLAUDE.md` |

**Always read the relevant subproject CLAUDE.md before making any change.**

---

## Universal Rules (apply to every project)

### Generic rules live in the skill

DRY / reuse, named constants, RFC 7807 errors, secrets, scope control and "never assume" are covered by
`ctech-dev-standards`; do not restate them here or in the subproject files. Product-specific additions:

- Search the codebase (`rg "..."`) before writing anything; two implementations that solve the same problem must be
  unified.
- Never assume DynamoDB table/index names, API contracts, payload formats, tax XML structures, AWS resource names or
  business rules (fiscal rules especially): search the code, then the docs, then ask.
- Never commit PFX certs, JWT secrets, AWS credentials, passwords, real customer data or real CNPJs.
- Implement only what was requested; no unrelated fixes, opportunistic refactors or API changes.
- **api / worker** return errors as RFC 7807 Problem JSON through `problem.*` helpers (api) and structured errors that
  let SQS route to the DLQ (worker). py-dfe was removed.
- **Fiscal calls** (SEFAZ and municipal NFS-e) always leave through the `go-dfe-egress` Lambda in `sa-east-1`
  (some authorities drop non-Brazilian traffic). Never call SEFAZ in process from `api` or `worker`.
- **Issuance is async** through a transactional outbox: API writes the outbox row, the publisher sends it to SNS/SQS,
  workers are idempotent consumers, results return through SNS to the API's WebSocket fan-out.
- **ui:** `npm run lint` (from `ui/`) must pass with zero errors and zero warnings before any commit.

### Sem travessão em texto visível

Proibido usar travessão (—) em qualquer texto visível ao usuário: UI (labels, guia, toasts, mensagens de erro),
copy de commit/PR, e nas respostas do Claude neste projeto. Prefira ponto e vírgula para pausa textual, ou lista
com marcadores para separar termos. Isso não se aplica a comentários de código nem a docs técnicos internos
(`OVERVIEW.md`, `DOCS.md`, `CONDUCT.md`).

### UI — guia, busca e navegação (regras inegociáveis)

Aplicam-se a toda alteração em `ui/`. Detalhe completo em `ui/CLAUDE.md`.

1. Toda feature nova voltada ao usuário — e toda mudança visual/de estilo — é documentada no guia
   (`/guide`) na mesma alteração.
2. Captura de tela adicionada/atualizada sempre que a mudança visual exigir
   (`npm run screens:capture`).
3. Toda página nova é registrada em `ui/src/lib/navigation/nav.tsx` — é isso que a coloca na
   navegação e na busca global (⌘K).
4. A documentação acompanha a UI: guia desatualizado conta como bug. Toda tela de cadastro ou
   configuração tem seção no guia com captura própria.
5. Só a landing (`/`) e o guia (`/guide/*`) são indexáveis; toda rota de app permanece `noindex`.

### Testing — core functions need integration tests

Every core function must be covered by an integration test in addition to unit tests.

| Change          | Required tests         |
|-----------------|------------------------|
| Schema          | Unit + contract        |
| Service logic   | Unit                   |
| AWS integration | Integration            |
| Fiscal issuance | Unit + integration     |
| Bug fix         | Reproduce + regression |

---

## Documentation

| File                 | Contents                           |
|----------------------|------------------------------------|
| `OVERVIEW.md`        | System architecture + data flow    |
| `DOCS.md`            | Complete technical reference       |
| `CONDUCT.md`         | Engineering guidelines             |
| `DynamoDB-Tables.md` | Schema for all tables (47 in CDK)  |
| `DEPLOYMENT.md`      | Infrastructure deployment guide    |
| `INTEGRATION.md`     | Frontend-backend integration guide |
| `THEME.md`           | Color palette and design system    |

---

## Mandatory Workflow

1. Read relevant docs (OVERVIEW.md, subproject CLAUDE.md, CONDUCT.md, DOCS.md as needed).
2. Search codebase for existing implementations — reuse → extend → parameterize → create.
3. Plan → Implement → Run affected tests.
4. Update docs: new endpoint/schema/module → DOCS.md; new constraint/workaround → CONDUCT.md.
5. Review cross-project impact (explicitly state which components were reviewed).
6. Suggest Conventional Commit (`feat:` / `fix:` / `refactor:` / `docs:` / `chore:`, no emojis).

# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and
clarifying questions come before implementation rather than after mistakes.

## Mandatory Documentation Policy

**Every code change MUST be documented.**

There are NO exceptions.

Any modification affecting behavior, architecture, APIs, integrations, configuration, deployment, security, business
rules, or developer workflow MUST include the corresponding documentation update in the same change.

## CTech Family

This repo is one service of the CTech family (ctech-cdk, ctech-go-common, ctech-ui, ctech-ws-client, ctech-account,
ctech-wallet, ctech-billing, ctech-poker, ctech-lbalancer, ...). "Does this apply to the whole family?" and
"reusable code goes to the shared package" are rules of `ctech-dev-standards`; a repo-local fix to a systemic problem
is incomplete.

## Known divergences from the standard (documented, not yet fixed)

- No CloudWatch alarm on any DLQ. Failures that need a human go through go-common `alerts` to the shared SNS topic
  (SSM `/ctech/{env}/alerts/topic-arn`); what alerts is listed in `worker/CLAUDE.md` and `api/CLAUDE.md`.
- Scans: only one-shot operator tools (migrations, `cmd/backfill-sweep`); services use the `sweep-index`, see `api/CLAUDE.md`.
- No inbound `Idempotency-Key`, no `@aoctech/ui`, no i18n in the UI.
- CI (`ci.yml`) runs gofmt, vet, arm64 build and `-race` tests plus UI lint/tsc/tests; there is no staticcheck,
  govulncheck, golangci-lint or `npm audit`, and the CDK jest tests are not run. `infra.yml` runs `cdk diff` with AWS
  credentials on pull requests.
- `go-dfe/internal/certificate/manager.go` sets `InsecureSkipVerify: true` for the SEFAZ server chain (deliberate and
  documented in `go-dfe/CLAUDE.md`); the standard names only the municipal exception, so the decision needs to be
  recorded in the skill.
