# CLAUDE.md — worker (ctech-dfe-worker)

Go Lambda — SQS (standard) consumer, async DFe issuance pipeline, `provided.al2023`.

**CTech standards:** generic backend, resilience and infra rules live in the `ctech-dev-standards` skill
(`references/backend.md`, `references/infra.md`); it wins over this file. This file records only what is specific
to the DFe worker.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §6`, `README.md` (full contract).

---

## Role

Consumes `WorkerMessage` / `DistributionMessage` from SQS (standard, not FIFO) and orchestrates DFe issuance:
fetches the certificate from S3 → invokes the go-dfe-egress Lambda (sa-east-1; XML-DSig + SEFAZ SOAP / NFS-e REST)
→ persists the result in DynamoDB → uploads XML to S3 → publishes the result to the SNS results topic
(`${env}-ctech-dfe-results`), which the API's `ResultsConsumer` fans out over WebSocket.

**Flow:** `SQS (standard) → cmd/<binary> → service.Process → S3 (cert) → go-dfe-egress (sa-east-1) → DynamoDB + S3 → SNS results`

---

## Directory Structure

```
worker/
├── cmd/
│   ├── worker/                  # emission/event workers (one binary, many functions)
│   ├── distribution-worker/     # SEFAZ/ADN distribution pulls
│   ├── distribution-dispatcher/ # scheduled (30 min): enumerates orgs, enqueues distribution jobs
│   ├── dlq-processor/           # terminal sink for DLQ messages
│   └── outbox-publisher/        # DynamoDB stream -> command SNS (transactional outbox)
└── internal/
    ├── boot/                    # root ctx, JSON slog, AWS config, alerts publisher (go-common)
    ├── config/                  # env config
    └── service/                 # flat package: dfe.go, distribution*.go, nfse.go, erasure.go, status.go, ...
```

There are no `handlers/`, `services/` or `repositories/` packages: `cmd/*/main.go` parses the event and calls one
`service` method; DynamoDB/S3/SNS calls are made inside `internal/service` (no repository layer). This diverges
from the family handler → service → repository layout; do not "fix" it as a drive-by.

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing implementations before creating new code.
3. Plan → Implement → Run affected tests (`go test ./... -race` from `worker/`).
4. Update `../DOCS.md` for architectural changes; `../CONDUCT.md` for new constraints.
5. State cross-project impact (worker ↔ api ↔ go-dfe-egress ↔ cdk).
6. Suggest Conventional Commit.

---

## Product Rules

- **Idempotency is mandatory**: standard SQS is at-least-once with no ordering. Check existing state before writing
  (`DfeService.alreadyTerminal`, `internal/service/dfe.go`); status updates use conditional `UpdateItem`. Fiscal
  numbering relies on the atomic `transact_write` plus that guard, not on ordering.
- **SEFAZ business rejections (cStat other than 100/135/150) are not retried**: persist the rejection and publish the
  failed status. Only network/timeout errors are retried (SQS visibility timeout, then DLQ).
- NFS-e branches out of the shared pipeline (`internal/service/nfse.go`, `distribution_nfse.go`): the response has no
  `cStat`/`xMotivo` and a rejection is always terminal.
- Every SEFAZ/municipal call goes through the **go-dfe-egress Lambda** (`DFE_LAMBDA_NAME`, region
  `DFE_EGRESS_REGION` = sa-east-1) via `invokeEgress` in `dfe.go`/`distribution.go`. There is no in-process
  `dfe.Call` and no fallback client. `go-dfe` is imported only for types and helpers (`Request`, `nfse`,
  `BuildXMLFragment`). Do not duplicate SEFAZ logic here; auxiliary-document rendering belongs to the API.
- After the SEFAZ response, in order: update DynamoDB status, upload XML to S3, publish to the SNS results topic.
  Valkey/WebSocket fan-out is the API's job.
- Lambda timeout must exceed the egress Lambda (120 s): event workers use 150 s, emission workers 300 s.
- Binary must be named `bootstrap` (`provided.al2023`, arm64). Table names derive from `TABLE_PREFIX`.
- Never commit AWS credentials, PFX certs, real CPF/CNPJ or customer data.

---

## Known Constraints and Divergences from the Standard

- DLQs have **no CloudWatch alarm** (removed from the CDK). Alerts go through go-common `alerts` to the shared SNS
  topic (SSM `/ctech/{env}/alerts/topic-arn`, env `ALERTS_TOPIC_ARN`, `alerts.Nop` when unset): `dlq-processor`
  sends `dlq-exhausted` once a message is marked failed; `outbox-publisher` sends `outbox-stuck` when a record is
  still unpublished after 10 minutes. The distribution-worker DLQ shares the worker DLQ. The outbox DLQ has no
  consumer (onFailure destination only).
- **Dispatcher enumeration** (`cmd/distribution-dispatcher/main.go`, `queryOrgPKs`): queries the `sweep-index` GSI of each
  `organization_*_configs` table (no Scan; the role has no `dynamodb:Scan`). The API stamps `sweep = "1"` on every
  config write (`api/internal/repositories`, `AttrSweep`); a config row without it is never dispatched.
- Root context, JSON slog and AWS config come from `internal/boot` (go-common `awsconfig`); the Lambda runtime gets
  it via `lambda.WithContext`, so `context.Background()` appears only there. There is no go-common `supervisor`: the
  worker has no in-process loops (each Lambda invocation is the unit of work). There is no SEFAZ circuit breaker
  (go-common `resilient`) on purpose: SEFAZ is called from the go-dfe-egress Lambda with a per-certificate mTLS
  client built on every call, so a breaker would have no shared state, and an open circuit would burn SQS receives
  and push messages to the DLQ faster. Retry is SQS redelivery + DLQ and go-dfe's own backoff inside the egress.
- Keep-warm: EventBridge pings every worker each minute with `{ping:true}`, short-circuited in `service/ping.go`.

---

## Testing

| Change | Required |
|--------|----------|
| Service logic | Unit test (mock AWS calls) |
| AWS integration | Integration test |
| Fiscal issuance | Unit + integration |
| Idempotency path | Integration test (duplicate message) |
| Bug fix | Regression test |

Run: `go test ./... -race` from `worker/`.

---

## Critical Areas (require analysis before touching)

- DFe issuance (NF-e, NFC-e, CT-e, MDF-e, NFS-e) and go-dfe-egress invocation/response parsing
- DynamoDB status persistence and idempotency checks
- SNS results publish; DLQ handling and retry logic; outbox publisher

Before touching: identify risks + side effects, verify backward compatibility + regulatory impact.

---

## Completion Checklist

- [ ] `go test ./... -race` passes; handlers idempotent (duplicate-message test)
- [ ] No duplication; constants named
- [ ] SEFAZ rejections not retried (only network errors)
- [ ] Docs updated (`../DOCS.md` and/or `../CONDUCT.md`)
- [ ] Cross-project impact reviewed (worker ↔ api ↔ go-dfe-egress ↔ cdk)

Every change that alters behavior, a contract or an operational rule updates the matching docs in the same change.
