# CLAUDE.md — worker (ctech-dfe-worker)

Go Lambda — SQS (standard) consumer, async DFe issuance pipeline, `provided.al2023`.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §6`.

---

## Role

Consumes `DfeWorkerEvent` messages from SQS (standard), orchestrates the full DFe issuance:
fetches certificate from S3 → invokes go-dfe in-process (XML-DSig + SEFAZ SOAP; py-dfe Lambda
is the fallback for operations not yet ported) → persists result in DynamoDB → uploads XML to S3 →
publishes results to the SNS results topic (DfeResultsBus).

**Flow:** `SQS (standard) → Handler → S3 (cert) → go-dfe (in-process SEFAZ; py-dfe Lambda fallback) → DynamoDB + S3 → SNS results topic`

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

- SQS is a standard queue: at-least-once delivery, no ordering guarantee — every handler **MUST be idempotent**.
- Before writing to DynamoDB, check existing state to avoid double-processing (see `DfeService.alreadyTerminal`, `internal/service/dfe.go`).

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
- `go-dfe` (module `gopkg.aoctech.app/dfe/go-dfe`, in-process, see `docs/plans/2026-07-17-go-dfe-migration.md`)
  is the sanctioned path for any SEFAZ operation in its `dfe.Implements(docType, service)` set. The
  py-dfe Lambda invocation remains the fallback for every operation NOT yet in that set, and is the
  permanent path for DANFE/DAMDFE rendering (out of go-dfe's scope indefinitely — see `MIGRATION.md`).
  Do not call py-dfe for an operation `go-dfe` already implements, and do not add a new SEFAZ
  operation to `go-dfe`'s implemented set without the byte-identical signature gate (signed ops) or
  shadow-mode parity window (unsigned ops) described in the plan.

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

- DLQ receives messages after max retries — monitored via a CloudWatch alarm per queue (configured in `cdk/lib/worker-stack.ts`).
- SQS is standard (not FIFO) — ordering across messages for the same org is NOT guaranteed; correctness relies on the fiscal-numbering `transact_write` (atomic, order-independent) plus the idempotency guard in `DfeService.Process`.
- py-dfe Lambda is the fallback path for XML signing + SEFAZ SOAP not yet ported to `go-dfe`, and the
  permanent path for DANFE/DAMDFE rendering; do not duplicate SEFAZ logic outside `go-dfe`/py-dfe.
- After SEFAZ response: always update DynamoDB status, upload XML to S3, publish results to the SNS
  results topic (DfeResultsBus) — in that order. Redis pub/sub and WebSocket fan-out are done by the
  API's ResultsConsumer, not the worker.
- Lambda timeout must be aligned with the worst-case SEFAZ latency + retry budget.

---

## Critical Areas (require analysis before touching)

- DFe issuance handlers (NF-e, NFC-e, CT-e, MDF-e)
- py-dfe Lambda invocation and response parsing
- DynamoDB status persistence and idempotency checks
- SNS results topic publish (DfeResultsBus); Redis/WebSocket fan-out is done by the API's ResultsConsumer.
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
