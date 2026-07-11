# AGENTS.md — ctech-dfe (monorepo root)

Brazilian tax SaaS (NF-e, NFC-e, CT-e, MDF-e) — direct SEFAZ communication via SOAP + mTLS.

**Before any task:** Read `OVERVIEW.md`. For cross-project changes also read `CONDUCT.md` and `DOCS.md`.

---

## Projects

| Project    | Role                                              | Full guidelines          |
|------------|---------------------------------------------------|--------------------------|
| `api/`     | Go REST API — Fiber v3, multi-tenant, DynamoDB    | `api/AGENTS.md`          |
| `worker/`  | Go Lambda — SQS FIFO consumer, DFe pipeline       | `worker/AGENTS.md`       |
| `ui/`      | Next.js 16 frontend — TypeScript, ShadCN          | `ui/AGENTS.md`           |
| `cdk/`     | AWS CDK infrastructure — TypeScript               | `cdk/AGENTS.md`          |
| `py-dfe/`  | Python Lambda — XML-DSig + SEFAZ SOAP + mTLS      | `py-dfe/AGENTS.md`       |

**Always read the relevant subproject AGENTS.md before making any change.**

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

Every string key, numeric code, URL, header name, or enum value must be a named constant.
Never scatter raw string literals across files.

### Backend error handling

- **api / worker:** All errors MUST be returned as RFC 7807 Problem JSON via `problem.*` helpers.
  Never return raw errors, `fiber.Map`, or unstructured responses.
- **py-dfe:** All errors MUST be raised as `DFeError` with explicit `status_code`, `code`, `message`.

### Frontend quality gate

- **ui:** `npx eslint src --ext .ts,.tsx` must pass with **zero errors and zero warnings** before
  any commit.

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

Implement only what was requested. No unrelated fixes, opportunistic refactors, dir reorganization,
or API changes.

---

## Never Assume

Never assume DynamoDB table/index names, API contracts, payload formats, tax XML structures,
AWS resource names, or business rules.
If not explicit: search codebase → search docs → ask user.

---

## Secrets

Never commit: PFX certs, JWT secrets, AWS credentials, passwords, real customer data, real CNPJs.

---

## Documentation

| File                 | Contents                              |
|----------------------|---------------------------------------|
| `OVERVIEW.md`        | System architecture + data flow       |
| `DOCS.md`            | Complete technical reference          |
| `CONDUCT.md`         | Engineering guidelines                |
| `DynamoDB-Tables.md` | Schema for all 22+ tables             |
| `DEPLOYMENT.md`      | Infrastructure deployment guide       |
| `INTEGRATION.md`     | Frontend-backend integration guide    |
| `THEME.md`           | Color palette and design system       |

---

## Mandatory Workflow

1. Read relevant docs (OVERVIEW.md, subproject AGENTS.md, CONDUCT.md, DOCS.md as needed).
2. Search codebase for existing implementations — reuse → extend → parameterize → create.
3. Plan → Implement → Run affected tests.
4. Update docs: new endpoint/schema/module → DOCS.md; new constraint/workaround → CONDUCT.md.
5. Review cross-project impact (explicitly state which components were reviewed).
6. Suggest Conventional Commit (`feat:` / `fix:` / `refactor:` / `docs:` / `chore:`, no emojis).
