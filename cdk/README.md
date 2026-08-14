# ctech-dfe CDK Infrastructure

> HAProxy migration: the API ASG no longer creates an ALB target group or listener
> rule. `ctech-lbalancer` discovers it through its `dfe` route; the retained
> `/ctech/{env}/network/alb-sg-id` identifies the shared edge trusted by the API SG.
> `PrivateIpv4Ec2Service` cannot be used for this stack because its current contract
> always creates the retired ALB target group and listener rule. CI permits the
> private-IPv4 launch-template override only in `lib/api-stack.ts`.

AWS CDK (TypeScript) for the ctech-dfe platform. Anchored to `lib/*.ts` and `bin/*.ts`
(prefer `.ts` over the compiled `.js`/`.d.ts`; ignore `cdk.out/`).

Sibling docs: [`../api/README.md`](../api/README.md) · [`../worker/README.md`](../worker/README.md) ·
root [`DEPLOYMENT.md`](../DEPLOYMENT.md).

> This file supersedes the earlier README that described a single `PyDfeStack`,
> `staging`/`production` env names, and "provisioned auto-scaling" DynamoDB — **all stale**.
> Valid envs are `dev|stage|prod`; DynamoDB is on-demand.

---

## 1. Stacks (`bin/ctech-dfe-cdk.ts`)

`ENVIRONMENT = process.env.ENVIRONMENT || 'dev'`; type `Environment = 'prod'|'stage'|'dev'`
(`lib/types.ts:1`). Account `868899309401`, region `us-east-1` (`bin:21-22`). Stack ids
`CtechDfe-{Env}-{Name}` (`bin:54-55`); OIDC is global.

| Stack | id | Creates |
|-------|-----|---------|
| `OidcStack` | `CtechDfe-Global-OIDC` | GitHub Actions OIDC deploy roles (`bin:60-65`) |
| `DynamoDBStack` | `CtechDfe-{Env}-DynamoDB` | 26 tables, including streamed worker outbox (§4) (`bin:70-75`) |
| `S3Stack` | `CtechDfe-{Env}-S3` | Certificates + Documents buckets (`bin:79-84`) |
| `EventBusStack` | `CtechDfe-{Env}-EventBus` | SNS event/results/ops-alerts + results SQS (`bin:86-90`) |
| `DfeStack` | `CtechDfe-{Env}-Dfe` | py-dfe Lambda + layer (`bin:97-101`) |
| `WorkerStack` | `CtechDfe-{Env}-Worker` | 8 workers + DLQs + dispatcher + outbox publisher (§2/§3) (`bin:105-116`) |
| `IAMStack` | `CtechDfe-{Env}-IAM` | Lambda/API roles + policies (`bin:118-130`) |
| `ApiStack` | `CtechDfe-{Env}-API-V2` | EC2 ASG + HAProxy route (§5) (`bin:139-156`) |
| `FrontendStack` | `CtechDfe-{Env}-Frontend` | S3 + CloudFront (§6) (`bin:168-176`) |

## 2. Lambdas

**Workers** (`lib/worker-definitions.ts`, built via `goCode` → Go `PROVIDED_AL2023`/`arm64`,
handler `bootstrap`):

| id | function | binary | timeout | mem | SEFAZ services |
|----|----------|--------|---------|-----|----------------|
| nfe-emission | `${env}-nfe-emission-worker` | worker | 300 | 128 | NFeAutorizacao |
| nfe-event | `${env}-nfe-event-worker` | worker | 60 | 128 | RecepcaoEvento |
| nfe-inutilization | `${env}-nfe-inutilization-worker` | worker | 60 | 128 | NfeInutilizacao |
| cte-emission | `${env}-cte-emission-worker` | worker | 300 | 128 | CTeRecepcao{Sinc,OS,GTVe,Simp} |
| cte-event | `${env}-cte-event-worker` | worker | 60 | 128 | CTeRecepcaoEvento |
| mdfe-emission | `${env}-mdfe-emission-worker` | worker | 300 | 128 | MDFeRecepcaoSinc |
| mdfe-event | `${env}-mdfe-event-worker` | worker | 60 | 128 | MDFeRecepcaoEvento |
| distribution | `${env}-distribution-worker` | distribution-worker | 300 | 256 | — |

Extra Lambdas (in `lib/worker-stack.ts`, not in worker-definitions):
- **DLQ processor** per worker: `${env}-{name}-dlq-processor`, timeout 30 / mem 128
  (`worker-stack.ts:273-290`).
- **outbox-publisher**: `${env}-dfe-outbox-publisher`, consumes `${p}_worker_outbox` `NEW_IMAGE` stream records,
  publishes command SNS, and conditionally acknowledges the row; its own DLQ has a CloudWatch alarm.
- **distribution-dispatcher**: `${env}-distribution-dispatcher`, timeout 60 / mem 128,
  EventBridge schedule every **30 min** (`worker-stack.ts:346-368`).
- **py-dfe**: `${env}-py-dfe`, Python 3.14, arm64, timeout 30 / mem 512, handler
  `py_dfe.handler.handler` (`lib/dfe-stack.ts:93-112`). **There is no separate go-dfe
  Lambda** — go-dfe runs in-process inside the Go workers; a comment at
  `worker-stack.ts:226` about "go-dfe in-process" is forward-looking.

## 3. SQS / SNS (`lib/event-bus-stack.ts`, `lib/worker-stack.ts`)

All **standard** (no FIFO anywhere).
- SNS: `${env}-ctech-dfe` (event/command bus), `${env}-ctech-dfe-results` (results),
  `${env}-ctech-dfe-results-ops-alerts`, `${env}-dfe-ops-alerts` (per-worker DLQ alarms).
- SQS: per-worker main queue `${env}-{queue}` → DLQ, `maxReceiveCount: 3`; DLQ retention
  **14 days** (`worker-stack.ts:107-129`). Results queue retention **1h**, DLQ 14d
  (`event-bus-stack.ts:42-65`). Event source `batchSize: 1`, `reportBatchItemFailures: true`
  (`worker-stack.ts:217-223`).

## 4. DynamoDB (`lib/dynamodb-stack.ts`)

`TableV2`, **`Billing.onDemand({ maxReadRequestUnits: 1000, maxWriteRequestUnits: 1000 })`**
(on-demand, capped — NOT provisioned autoscaling). `RemovalPolicy` DESTROY in dev / RETAIN
otherwise; **PITR prod only** (`:221-222`). 26 tables; notable GSIs:

| table | keys | GSIs |
|-------|------|------|
| `${p}_users` | pk | email-index, username-index, ctech-user-id-index |
| `${p}_organization_users` | pk+sk | user-index (inverted) |
| `${p}_organization_invitations` | pk (+TTL `ttl`) | org-invite-index |
| `${p}_audit_logs` | pk+sk | org-time-index, user-id-index |
| `${p}_organization_products` | pk+sk | description-index, code-index |
| `${p}_organization_vehicles` | pk+sk | plate-index, role-index |
| `${p}_organization_persons` | pk+sk | org-name-index |
| `${p}_{nfes,nfces,ctes,mdfes}` | pk+sk | number-index-v2, dfe-index |
| `${p}_{nfe,nfce,cte,mdfe}_events` | pk+sk | org-event-key-index |
| `${p}_{nfe,cte,mdfe}_distributions` | pk + nsu(N) | — |
| `${p}_organization_{nfe,nfce,cte,mdfe}_configs` | pk only | — |
| `${p}_worker_outbox` | pk+sk (+TTL `ttl`, `NEW_IMAGE` stream) | — |

The `TableName` union (`:8-34`) is a **logical-name enum**, not the physical name — map via
the `getDfeTable`/`getEventsTable`/`getDistributionTable`/`getDfeConfigTable` builders.

## 5. IAM & EC2 (`lib/iam-stack.ts`, `lib/api-stack.ts`, `lib/oidc-stack.ts`)

- Roles: `${env}-py-dfe-lambda-role`, `${env}-ctech-dfe-api-v2-role`, per-worker roles,
  dispatcher role, DLQ-processor roles. `DynamoPolicy` grants CRUD + Query/**Scan** on all
  tables + `/index/*` (`iam-stack.ts:81-105`); S3 on cert/doc buckets; SNS publish on the
  event bus; SQS receive on results + send on distribution; SSM `GetParameter` on
  `/ctech-dfe|ctech-account|ctech/${env}/*`.
- **API on a private-IPv4 EC2 ASG routed by `ctech-lbalancer` HAProxy**
  (`api-stack.ts`): `minCapacity: 1`, `maxCapacity: prod ? 3 : 1`; HAProxy probes
  `/v1.0/health-check` and accepts `200,207`. The binary `app` runs from
  `/opt/app/current/app` via systemd; nginx `:8080 → :8000` keeps the per-IP/per-tenant
  rate limits. No ALB target group or listener rule is synthesized.
- CloudWatch Agent publishes four bounded 60-second host series under
  `CtechDfe/<env>/Host`: memory %, swap %, root-disk %, and application RSS.
  EC2's native `CPUUtilization`/`CPUCreditBalance` remain the CPU source.
- OIDC: GitHub Actions deploy roles gated on `repo:{githubRepo}:*` for frontend/api/infra/
  pydfe/worker (`oidc-stack.ts`).

## 6. CloudFront / Frontend (`lib/frontend-stack.ts`)

Private S3 `${env}-ctech-dfe-frontend` + OAC; CloudFront Function URL-rewrite (clean URLs →
`.html`, unknown → `/404.html`); API origin behavior for `/v1.0/*` (CACHING_DISABLED) and for
`/docs`, `/openapi.json`, `/openapi.yaml` (same behavior, own CSP allowing unpkg for Stoplight
Elements); security headers (HSTS 2y, frame DENY, CSP `default-src 'self'`); `priceClass PRICE_CLASS_100`;
HTTP2+3; TLS 1.2_2021. **No `geoRestriction`** in this stack (if any Brazil geo-restriction
exists it is applied elsewhere).

## 7. Deploy & cost

- `cdk.json:2` → `npx ts-node bin/ctech-dfe-cdk.ts`. `package.json`
  `deploy:prod = ENVIRONMENT=prod cdk deploy --all --require-approval never --profile ctech`.
  Env config via env vars (`ENVIRONMENT`, `CTECH_VPC_ID`, `CTECH_DEPLOYMENTS_BUCKET`,
  `CTECH_LOGS_BUCKET`, `GITHUB_REPO`); shared infra (VPC, edge SG, Valkey, buckets) from SSM.
- **Keep-warm (~B21 cost driver)**: for each of the 8 workers a `scheduler.Schedule`
  `${env}-{name}-ping-schedule` invokes the worker with `{ping:true}` at
  **`Duration.minutes(1)`** (`worker-stack.ts:228-236`) — i.e. 8 Lambdas pinged 1440×/day
  each. **Code/comment mismatch**: the comment at `worker-stack.ts:225` says "every 5 min";
  the code says 1 min — the code wins. No Lambda provisioned concurrency; EC2 `maxCapacity 3`
  (prod) / 1 (dev/stage); DynamoDB on-demand capped 1000 RU/WU; CloudFront PRICE_CLASS_100.

## 8. Known divergences (documented honestly)

- **B14** — DynamoDB IAM policy grants `Scan` (`iam-stack.ts:81-105`), consumed by the
  distribution-dispatcher's org enumeration (`worker/README.md` §6).
- Keep-warm rate is 1 min (code) vs 5 min (comment) — B21 cost note.
- No separate go-dfe Lambda; go-dfe is in-process in the Go workers.
- ASG `gracePeriod: 120s` is defined locally in `lib/api-stack.ts`; migration to
  the new `@aoctech/cdk` 0.2.0 HAProxy construct must preserve it.

See root [`DEPLOYMENT.md`](../DEPLOYMENT.md), [`CONDUCT.md`](../CONDUCT.md),
[`DOCS.md`](../DOCS.md).
