# CLAUDE.md — worker (ctech-dfe-worker)

Go Lambda — SNS-subscribed / async-invoked consumer (no SQS polling), async DFe issuance pipeline, `provided.al2023`.

**CTech standards:** generic backend, resilience and infra rules live in the `ctech-dev-standards` skill
(`references/backend.md`, `references/infra.md`); it wins over this file. This file records only what is specific
to the DFe worker.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §6`, `README.md` (full contract).

---

## Role

Consumes `WorkerMessage` (SNS event, `Records[].Sns.Message`) / `DistributionMessage` (direct async invoke payload) and orchestrates DFe issuance:
fetches the certificate from S3 → invokes the go-dfe-egress Lambda (sa-east-1; XML-DSig + SEFAZ SOAP / NFS-e REST)
→ persists the result in DynamoDB → uploads XML to S3 → publishes the result to the SNS results topic
(`${env}-ctech-dfe-results`), which the API's `ResultsConsumer` fans out over WebSocket.

**Flow:** `SNS / async invoke → cmd/<binary> → service.Process → S3 (cert) → go-dfe-egress (sa-east-1) → DynamoDB + S3 → SNS results`

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
  failed status. Only network/timeout errors are retried (Lambda async retry x2, then the dlq-processor destination).
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
  and push messages to the DLQ faster. Retry is Lambda async retry + dlq-processor destination and go-dfe's own backoff inside the egress.
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


Delivery design, resilience table and queue drain runbook: `../docs/specs/2026-11-02-sns-lambda-worker-delivery.md`.

## Contingency retransmission (`cmd/contingency-retransmit`)

Sends the document's stored `contingency_msg` (a `WorkerMessage` whose body is `{"@signed_xml": ...}`) through
`DfeService.Process`, then classifies the item with `contingency.Classify`. Needs the same env and IAM as
`cmd/worker`. Terminal outcomes remove `contingency_sweep` and `contingency_msg` (the message carries the cert password).

## CIOT (`ciot-operation`, `internal/service/ciot.go`)

Same `cmd/worker` binary and SNS filter mechanism as the other workers; `DfeService.Process` routes `doc_type == "ciot"` to
`processCiot` before the document claim. Spec: `docs/specs/2026-11-04-ciot-antt-pef-integracao.md` sections 4, 8 and 10.

- **Message:** `doc_pk` = `{env}#{org}`, `event_sk` = the `ciots` row sk, `event_type` = `declare|cancel|rectify|close`, `access_key` =
  unique operation key (outbox pk), `sefaz_environment` = `prod|hom` (the CIOT environment, also the SSM `{env}` segment), `body`
  = `{declaration | request, payment_institution}`.
- **Claim:** `UPDATE ciots` sets `processing_owner` + lease under `pending_operation = :op`. Redelivery of a finished operation
  (no `pending_operation`) republishes the stored outcome and calls nothing; a held lease is a retryable error.
- **Declaration:** identifier first (stored on the row as `code` and reused on retries; never fabricated), then
  `DeclaracaoOperacaoTransporte` with a fresh `DataDeclaracao` outside contingency (B11 window). Generator key and fallback URL
  are read from SSM (`/ctech-dfe/{env}/ciot/generator-api-key` SecureString, `generator-fallback-url`) through an SSM-backed
  `ciot.SecretProvider` cached per invocation and are passed to the egress only in the generator request; they are never logged.
- **Outcomes (`decideCiot`, pure and tested):** 110/111 update the state and store the 16-character CIOT, code and
  `carrier_warning`; business rejection (HTTP 422 from the egress) is terminal and returns nil so SQS/Lambda never retry it
  (declaration becomes `rejected`; a lifecycle operation keeps its status and stores `last_error`); **219** reconciles with
  `ConsultarCIOTGerado` and adopts the existing CIOT; **222** confirms a cancellation and **251** a closure (the B37/B38 to
  code mapping is inferred; any other combination is a rejection); transport/5xx/502 release the lease and return an error
  (platform retries, then the dlq-processor); credential group (`IsCredentialProblem`) and egress 400 raise an operator alert
  (`ciot-credential`) instead of a business error and are retried.
- **Results:** SNS `result_kind=document`, `table_name=ciots`, `access_key` = row sk, `status`, `operation`, `ciot`, `code`,
  `carrier_warning`, `antt_code`, `internal_code`. No billing fields: billing ignores the table.
- **Audit:** every egress answer is appended to `ciot_audit` (body sent with its SHA-256, response), insert-only, best effort.
- **DLQ:** `service.CiotDLQ` (used by `cmd/dlq-processor`) turns an exhausted declaration into `failed`; a lifecycle operation keeps
  its status and only clears `pending_operation`.
- **Proactive alerts (`internal/ciotalert`):** no schedule or Lambda of their own; `cmd/contingency-dispatcher` calls
  `ciotalert.Sweep` at the start of its 15-minute run (a failure is logged and never blocks the retransmissions). Delivery is only
  go-common `alerts` (env `ALERTS_TOPIC_ARN`), no CloudWatch alarm or metric. Sparse `alert-index` on `ciots`
  (`alert_sweep = "1"`, sort `alert_at`, KEYS_ONLY): `finishCiot` stamps it when a declaration succeeds (`alert_at` = trip start,
  or the declaration date for TAC-Agregado, in Brasilia time, plus 24 h, `ciot.CancelDeadline`) and removes it on cancel or close;
  a rectification leaves it. The sweep queries the whole index (bounded by open CIOTs, no Scan).
  - `ciot-cancel-deadline`: a row whose `alert_at` is within 2 h ahead and that has no operation in flight; a conditional update of
    `cancel_alerted` is the once-only guard.
  - `ciot-certificate-expiry`: per organization with open CIOTs, the latest `expires_at` in `organization_certificates` (a renewed
    certificate silences the old one) against thresholds 15, 7, 3, 1 days and 0 (expired); the tightest crossed threshold alerts once,
    guarded by a conditional-put marker row in `ciots` (`CERT_ALERT#{org}`, TTL). Marker first, then alert: a crash in between loses
    that threshold's alert (the next threshold still fires) rather than repeating it.
  - Alert text carries ids, env and dates only (org pk, row sk, expiry, threshold); never a document number or certificate content.
  - Role: `Query` on `ciots/alert-index` and `organization_certificates`, `PutItem`/`UpdateItem` on `ciots`, `sns:Publish` on the alerts topic.
  - Why not a new schedule: same cadence, same Lambda, no extra cost; the dispatcher is named after contingency only for history.
  - CIOTs declared before this change have no `alert_*` keys and are not covered (a rectification does not stamp them; no backfill built).
