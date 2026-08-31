# CLAUDE.md — ctech-dfe (monorepo root)

Brazilian tax SaaS (NF-e, NFC-e, CT-e, MDF-e) — direct SEFAZ communication via SOAP + mTLS.

**Before any task:** Read `OVERVIEW.md`. For cross-project changes also read `CONDUCT.md` and `DOCS.md`.

---

## Projects

| Project   | Role                                                   | Full guidelines    |
|-----------|--------------------------------------------------------|--------------------|
| `api/`    | Go REST API — Fiber v3, multi-tenant, DynamoDB         | `api/CLAUDE.md`    |
| `worker/` | Go Lambda — standard SQS consumer, DFe pipeline        | `worker/CLAUDE.md` |
| `ui/`     | Next.js 16 frontend — TypeScript, ShadCN               | `ui/CLAUDE.md`     |
| `cdk/`    | AWS CDK infrastructure — TypeScript                    | `cdk/CLAUDE.md`    |
| `py-dfe/` | Python Lambda — XML-DSig + SEFAZ SOAP + mTLS           | `py-dfe/CLAUDE.md` |
| `go-dfe/` | Go lib — in-process SEFAZ SOAP+mTLS (py-dfe migration) | `go-dfe/CLAUDE.md` |

**Always read the relevant subproject CLAUDE.md before making any change.**

---

## Universal Rules (apply to every project)

### DRY — think generic first

Before writing any function, search the codebase (`rg "..."`):

1. Reuse existing code.
2. Extend if reuse is insufficient.
3. Parameterize if behavior differs only by inputs.
4. Create new only when no suitable alternative exists.

Two implementations that solve the same problem must be unified.

### Constants — no magic variables

Every string key, numeric code, URL, header name, or enum value must be a named constant. Never scatter raw string
literals across files.

### Backend error handling

- **api / worker:** All errors MUST be returned as RFC 7807 Problem JSON via `problem.*` helpers. Never return raw
  errors, `fiber.Map`, or unstructured responses.
- **py-dfe:** All errors MUST be raised as `DFeError` with explicit `status_code`, `code`, `message`.

### Frontend quality gate

- **ui:** `npx eslint src --ext .ts,.tsx` must pass with **zero errors and zero warnings** before any commit.

### UI — guia, busca e navegação (regras inegociáveis)

Aplicam-se a toda alteração em `ui/`. Detalhe completo em `ui/CLAUDE.md`.

1. Toda feature nova voltada ao usuário — e toda mudança visual/de estilo — é documentada no guia
   (`/guide`) na mesma alteração.
2. Captura de tela adicionada/atualizada sempre que a mudança visual exigir
   (`npm run screens:capture`).
3. Toda página nova é registrada em `ui/src/lib/navigation/nav.tsx` — é isso que a coloca na
   navegação e na busca global (⌘K).
4. A documentação acompanha a UI: guia desatualizado conta como bug.

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

## Scope Control

Implement only what was requested. No unrelated fixes, opportunistic refactors, dir reorganization, or API changes.

---

## Never Assume

Never assume DynamoDB table/index names, API contracts, payload formats, tax XML structures, AWS resource names, or
business rules. If not explicit: search codebase → search docs → ask user.

---

## Secrets

Never commit: PFX certs, JWT secrets, AWS credentials, passwords, real customer data, real CNPJs.

---

## Documentation

| File                 | Contents                           |
|----------------------|------------------------------------|
| `OVERVIEW.md`        | System architecture + data flow    |
| `DOCS.md`            | Complete technical reference       |
| `CONDUCT.md`         | Engineering guidelines             |
| `DynamoDB-Tables.md` | Schema for all 22+ tables          |
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
