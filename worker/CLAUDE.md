# CLAUDE.md — worker (ctech-dfe-worker)

Go Lambda — SQS (standard) consumer, async DFe issuance pipeline, `provided.al2023`.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §6`.

---

## Role

Consumes `DfeWorkerEvent` messages from SQS (standard), orchestrates the full DFe issuance:
fetches certificate from S3 → invokes the go-dfe-egress Lambda (sa-east-1; XML-DSig + SEFAZ SOAP /
NFS-e REST) → persists result in DynamoDB → uploads XML to S3 →
publishes results to the SNS results topic (DfeResultsBus).

**Flow:** `SQS (standard) → Handler → S3 (cert) → Invoke go-dfe-egress (sa-east-1; SEFAZ/municipal) → DynamoDB + S3 → SNS results topic`

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
5. State cross-project impact (worker ↔ api ↔ go-dfe-egress ↔ cdk).
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
- Every SEFAZ/municipal call goes through the **go-dfe-egress Lambda** (`DFE_LAMBDA_NAME`, region
  `DFE_EGRESS_REGION` = sa-east-1), via `invokeEgress` in `dfe.go`/`distribution.go`. There is no
  in-process `dfe.Call` and no fallback client: if the egress fails, the call fails and the normal
  retry applies. Some authorities (Teresina) drop traffic from outside Brazil, which is why it runs in
  sa-east-1. Auxiliary-document rendering belongs to the API. `go-dfe` is imported only for its types
  and helpers (`Request`, `nfse`, `BuildXMLFragment`).

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
- All XML signing and SEFAZ SOAP lives in `go-dfe` (executed by go-dfe-egress); do not duplicate SEFAZ
  logic in the worker.
- After SEFAZ response: always update DynamoDB status, upload XML to S3, publish results to the SNS
  results topic (DfeResultsBus) — in that order. Redis pub/sub and WebSocket fan-out are done by the
  API's ResultsConsumer, not the worker.
- Lambda timeout must be aligned with the worst-case SEFAZ latency + retry budget.
- NFS-e branches out of the shared pipeline in `internal/service/nfse.go` (issuance/events) and
  `distribution_nfse.go` (ADN NSU cursor). It goes through the same egress Lambda, the response carries no
  `cStat`/`xMotivo`, and a rejection is always terminal — never retry it.

---

## Critical Areas (require analysis before touching)

- DFe issuance handlers (NF-e, NFC-e, CT-e, MDF-e)
- go-dfe-egress Lambda invocation and response parsing
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
- [ ] Cross-project impact reviewed (worker ↔ api ↔ go-dfe-egress ↔ cdk)

## Mandatory Documentation Policy

**Every code change MUST be documented.**

There are NO exceptions.

Any modification affecting behavior, architecture, APIs, integrations, configuration, deployment, security, business rules, or developer workflow MUST include the corresponding documentation update in the same change.
