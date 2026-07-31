# ctech-dfe Worker

Go Lambda workers — SQS consumers that sign and dispatch fiscal documents to SEFAZ
(via **go-dfe** in-process or **py-dfe** Lambda fallback) and run the inbound
**Distribuição DF-e** pipeline. This doc is anchored to `internal/...` and `cmd/...`.

Sibling docs: [`../api/README.md`](../api/README.md) · [`../go-dfe/README.md`](../go-dfe/README.md) ·
[`../py-dfe/README.md`](../py-dfe/README.md) · root [`DOCS.md`](../DOCS.md).

---

## 1. Entrypoints (`cmd/`)

| Binary | File | Role | In | Out |
|--------|------|------|----|-----|
| `worker` | `cmd/worker/main.go` | DFe SEFAZ issuance/events consumer | SQS DFe queue | SEFAZ (go-dfe/py-dfe), SNS results |
| `distribution-worker` | `cmd/distribution-worker/main.go` | Inbound Distribuição DF-e pipeline | SQS distribution queue | S3, SNS results, DynamoDB |
| `distribution-dispatcher` | `cmd/distribution-dispatcher/main.go` | Scheduler/enqueuer (no SEFAZ) | EventBridge (cron) | SQS distribution queue |
| `dlq-processor` | `cmd/dlq-processor/main.go` | Terminal sink for dead-letter messages | SQS DLQ | DynamoDB terminal status, SNS results (optional) |
| `outbox-publisher` | `cmd/outbox-publisher/main.go` | Durable command dispatcher | `worker_outbox` DynamoDB Stream | command SNS, conditional outbox acknowledgement |

Common handler pattern (`cmd/worker/main.go:54-80`, `cmd/distribution-worker/main.go:54-80`):
`IsPingEvent` keep-warm short-circuit → unmarshal SQS records → `svc.Process` → failures
appended to `batchItemFailures` → returned so SQS redelivers until `maxReceiveCount` then DLQ.

Env vars (`internal/config/config.go`):
- `worker` / `distribution-worker`: require `DOCUMENTS_BUCKET`, `CERTIFICATES_BUCKET`,
  `DFE_LAMBDA_NAME` (py-dfe function); optional `RESULTS_TOPIC_ARN`, `EVENT_BUS_TOPIC_ARN`,
  `DISTRIBUTION_QUEUE_URL` (`config.go:32-40`).
- `outbox-publisher`: requires `EVENT_BUS_TOPIC_ARN` and `OUTBOX_TABLE_NAME`; row identity and payload are carried by
  each DynamoDB Stream record.
- `distribution-dispatcher`: requires `DISTRIBUTION_QUEUE_URL`; `TABLE_PREFIX`, `AWS_REGION`
  have defaults (`config.go:51`).
- `dlq-processor`: reads `RESULTS_TOPIC_ARN`, `TABLE_PREFIX` directly via `os.Getenv`
  (`cmd/dlq-processor/main.go:34-35`) — **not** validated by `config.Load`; silently no-ops
  SNS if unset (divergence **H5**).

---

## 2. SQS / SNS contracts

**Messages**
- `WorkerMessage` (`internal/service/dfe.go:75`): `doc_pk, access_key, table_name,
  s3_prefix, expected_file_name, cnpj, uf, sefaz_environment, cert_s3_key, cert_password,
  doc_type, sefaz_service, body` + optional event fields. Published by the API
  (`api/internal/services/worker.go:14-33`).
- `DistributionMessage` (`internal/service/distribution.go:135`): `job_type, org_pk,
  doc_type, trigger, nsu?, access_key?`. The dispatcher emits
  `{job_type:"dist_nsu", trigger:"scheduler"}` (`cmd/distribution-dispatcher/main.go:54-59`).

**Topics**
- `${env}-ctech-dfe` — event/command bus (API → workers). `EVENT_BUS_TOPIC_ARN`.
- `${env}-ctech-dfe-results` — results bus (workers → API `ResultsConsumer`).
  `RESULTS_TOPIC_ARN`. Two payload shapes: `result_kind: document|event` (dfe) and
  `{type:"new_distribution_<doc_type>"}` (distribution) — see **H2**.
- `${env}-ctech-dfe-results-ops-alerts` — DLQ alarm notifications.

**Retry / DLQ** (`cdk/lib/worker-stack.ts`): per-worker main queue → DLQ,
`maxReceiveCount: 3`; DLQ retention 14 days; SQS `reportBatchItemFailures: true`.
Before any fiscal side effect, the issuance/event worker conditionally writes `status=processing`, a random
`processing_owner`, six-minute `processing_lease_until`, and an incremented attempt. Concurrent delivery while the
lease is active is rejected for redelivery; an expired lease can be stolen, and only its owner can finalize.
DynamoDB claim/read failures fail closed.

Certificate/S3/Lambda/engine failures, malformed responses, and HTTP 408/425/429/5xx set
`retryable_failed`, release the lease, and return an error so SQS retries. SEFAZ business rejection and other
non-retryable 4xx responses are terminal. The DLQ processor is the terminal sink after delivery exhaustion.

---

## 3. go-dfe (primary) vs py-dfe (fallback)

Decision point in both pipelines:
- Issuance: `if godfeImplements(msg.DocType, msg.SefazService) { godfeCall } else { invokePyDfe }`
  (`internal/service/dfe.go:205`).
- Distribution: same gate at `internal/service/distribution.go:1083`.

`godfeImplements` / `godfeCall` are package vars wrapping `godfe.Implements` / `godfe.Call`
(`internal/service/godfe_shadow.go:13-16`) — real go-dfe compiled in via `../go.work`.
py-dfe is reached via Lambda `InvokeFunction` on `DFE_LAMBDA_NAME`
(`internal/service/dfe.go:447`, `distribution.go:1098`). **go-dfe-primary / py-dfe-fallback**
is the live behavior (replaces the earlier Redis/py-dfe-primary design — see
`CLAUDE.md`). Note: there is **no separate go-dfe Lambda**; go-dfe runs in-process.

`godfe_shadow.go` is **not** a shadow/comparison harness — it is a test-stub indirection
(divergence **H3**). The real shadow comparison lives in `go-dfe/shadow.go` (logs only,
never affects the result).

---

## 4. Distribution pipeline (`internal/service/distribution.go`)

`DistributionService.Process` (`distribution.go:145`) routes by `job_type`:
- `dist_nsu` / `""` → `runDistNSU` (`distribution.go:179`) — full NFe/CTe/MDFe Distribuição.
- `cons_nsu` → `runConsNSU` (requires `nsu`).
- `cons_ch_nfe` → `runConsAccessKey` (CTe/MDFe via chave).

`runDistNSU` order: `loadConfig` → `loadCert` (queries `organization_certificates`) →
`loadOrg` → `getCertB64` (cert cache) → `buildPayload` → dispatch (go/py) → `processDocZip`.
`processDocZip` (`distribution.go:434`): decompress → parse XML → `persistIncoming` /
`persistEvent` / `autoScience` (ciência event re-fed via SNS) → S3 uploads. Audit rows are
written via `TransactWriteItems` (`audit.go:30`).

**Doc types distributed**: `nfe → NFeDistribuicaoDFe`, `cte → CTeDistribuicaoDFe`,
`mdfe → MDFeDistribuicaoDFe` (`distribution.go:64`). **NFC-e is NOT distributed**
(divergence **H1**).

---

## 5. Cert cache (`internal/service/cert_cache.go`)

In-memory, per-Lambda-execution (warm container reuse). `certCacheTTL = 15m`
(`cert_cache.go:12`). Caches the **base64, still password-encrypted PFX blob**; the password
is never cached (`cert_cache.go:17-21`) — only saves the S3 download, not decryption.
Consumed by `dfe.go:416` and `distribution.go:1010`.

---

## 6. Known divergences (documented honestly)

- **B14** — `distribution-dispatcher` **Scan**s `*_organization_{doc_type}_configs` to
  enumerate org PKs (`scanOrgPKs`, `cmd/distribution-dispatcher/main.go:75`). Scan in a
  periodic path; documented as a known divergence.
- **H1** — NFC-e is issuable but never scheduled/distributed.
- **H2** — distribution result notifications always emit `type:"new_distribution_nfe"`
  regardless of doc type (`distribution.go:853`).
- **H5** — `dlq-processor` uses unvalidated `os.Getenv` for its config.
- **H6** — dispatcher `dispatch()` always returns `nil` even if some `SendMessage` calls
  failed (logged + continue), so partial enqueues are not surfaced.
- **H7** — `config.Load` requires `DFE_LAMBDA_NAME` even when every op is go-dfe.

See root [`CONDUCT.md`](../CONDUCT.md) / [`DOCS.md`](../DOCS.md) for the full register
(B4, B5, B12, B14).
