# AGENTS.md — worker (ctech-dfe-worker)

Go Lambda — SQS consumer (standard queue, at-least-once), async DFe issuance pipeline,
`provided.al2023`.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §6`,
`../MIGRATION.md`, `../worker/README.md`.

---

## Role

Consumes `WorkerMessage` / `DistributionMessage` from SQS (standard — **not** FIFO),
orchestrates the full DFe issuance:
fetches certificate from S3 → calls SEFAZ via **go-dfe in-process (primary)** or
**py-dfe Lambda (fallback)** → persists result in DynamoDB → uploads XML to S3 →
publishes result to the **SNS results bus** (`${env}-ctech-dfe-results`), which the API's
`ResultsConsumer` polls and fans out over WebSocket.

**Flow:** `SQS (standard) → Handler → S3 (cert) → go-dfe (in-process SEFAZ; py-dfe Lambda fallback) → DynamoDB + S3 → SNS results topic`

See [`README.md`](README.md) for the full contract and the go-dfe/py-dfe dispatch decision.

---

## Directory Structure

```
worker/
├── cmd/                        # Lambda entry points (one per handler type)
├── internal/
│   ├── config/                 # Env config
│   ├── handlers/               # SQS message handlers (one per doc type)
│   ├── services/               # Business logic
│   ├── repositories/           # DynamoDB persistence
│   └── awsclient/              # aws-sdk-go-v2 wrappers
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing implementations before creating new code.
3. Plan → Implement → Run affected tests.
4. Update `../DOCS.md` for architectural changes; `../CONDUCT.md` for new constraints.
5. State cross-project impact (worker ↔ api ↔ py-dfe ↔ cdk).
6. Suggest Conventional Commit.

---

## Engineering Rules

### DRY

- Never duplicate functions. If two handlers share logic, extract to a shared package.
- Before adding any function, search `internal/` for existing implementations.
- Shared constants (env strings, DynamoDB status values, S3 key patterns) must not be redeclared
  per-handler — define once in `internal/` and import.

### Constants — no magic strings/numbers

- DynamoDB status values (`pending`, `authorized`, `rejected`, `failed`, `cancelled`), S3 key
  patterns, SQS attribute names, and Lambda function name env vars MUST be named constants.
- Never hardcode table names — always derive from `TABLE_PREFIX` config.

### Error Handling (MUST follow)

- Worker errors are non-HTTP, but structured errors must be used consistently.
- Return errors explicitly; never silently swallow failures.
- For DynamoDB/S3/Lambda call failures: log with context (org_pk, access_key) and return the
  error to let SQS handle visibility timeout + DLQ routing.
- **SEFAZ business rejections (cStat != 100/135/150) must NOT be retried** — persist the
  rejection and publish the failed status. Only network/timeout errors warrant retry.

### Idempotency (critical)

- Standard SQS provides at-least-once delivery and no ordering guarantee — every handler **MUST be idempotent**.
- Before writing to DynamoDB, check existing state to avoid double-processing.
- Do not rely on `MessageGroupId`, FIFO deduplication, or delivery order; conditional claims and state transitions are
  the concurrency boundary.

### Layer Separation

| Layer      | Allowed                              | Forbidden                        |
|------------|--------------------------------------|----------------------------------|
| Handler    | Parse SQS event, call one service    | Business logic, direct AWS calls |
| Service    | Orchestration, business logic        | SQS parsing, HTTP concerns       |
| Repository | DynamoDB read/write only             | Business logic, orchestration    |

### Go Rules

- Runtime: `provided.al2023`. Binary MUST be named `bootstrap`.
- Use `aws-sdk-go-v2` only.
- No goroutines that outlive the Lambda invocation.
- Invoke `go-dfe` in-process for promoted operations and use the py-dfe Lambda only as the compatibility fallback.

### DynamoDB

- `GetItem` > `Query` > `Scan`. No production scans.
- Status updates use `UpdateItem` with condition expressions to prevent race conditions.

### Secrets

Never commit: AWS credentials, PFX certs, real CPF/CNPJ, real customer data.

---

## Testing

| Change              | Required                         |
|---------------------|----------------------------------|
| Handler logic       | Unit test (mock AWS calls)       |
| Service logic       | Unit test                        |
| AWS integration     | Integration test                 |
| Fiscal issuance     | Unit + integration               |
| Idempotency path    | Integration test (duplicate msg) |
| Bug fix             | Regression test                  |

**Every core handler and service function must have an integration test.**

Run: `go test ./... -race` from `worker/`.

---

## Known Constraints

- DLQ receives messages after max retries — monitor via CloudWatch alarms (configured in CDK).
- Command queues are standard SQS; duplicate and out-of-order deliveries are expected.
- `go-dfe` is the primary in-process path for promoted services; py-dfe remains the fallback and PDF renderer.
- After a terminal SEFAZ response: persist state, upload XML to S3, and publish the result to SNS; the API results
  consumer performs Valkey/WebSocket fan-out.
- Lambda timeout must be aligned with the worst-case SEFAZ latency + retry budget.

---

## Critical Areas (require analysis before touching)

- DFe issuance handlers (NF-e, NFC-e, CT-e, MDF-e)
- py-dfe Lambda invocation and response parsing
- DynamoDB status persistence and idempotency checks
- Results SNS publication and API-side Valkey/WebSocket delivery
- DLQ handling and retry logic

Before touching: identify risks + side effects, verify backward compatibility + regulatory impact.

---

## Completion Checklist

- [ ] Code compiles; `go test ./... -race` passes
- [ ] Handlers are idempotent (verified with duplicate-message test)
- [ ] No duplication introduced (searched before creating)
- [ ] All constants named (no magic strings)
- [ ] SEFAZ rejections not retried (only network errors)
- [ ] Docs updated (`../DOCS.md` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (worker ↔ api ↔ py-dfe ↔ cdk)

## Mandatory Documentation Policy

**Every code change MUST be documented.**

There are NO exceptions.

Any modification affecting behavior, architecture, APIs, integrations, configuration, deployment, security, business rules, or developer workflow MUST include the corresponding documentation update in the same change.
