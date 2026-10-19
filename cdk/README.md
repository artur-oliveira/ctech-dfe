# ctech-dfe CDK Infrastructure

> The API ASG creates no ALB, target group or listener rule (there is no ALB in this repo). `ctech-lbalancer` discovers it through its `dfe` route; the retained
> `/ctech/{env}/network/alb-sg-id` identifies the shared edge trusted by the API SG.
> The stack now uses `HaproxyEc2Service` from `@aoctech/cdk`; route creation stays
> disabled because the existing `dfe` route parameter is owned by
> `ctech-lbalancer`.

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
(`lib/types.ts:1`). Account `868899309401`, region `us-east-1`. Stack ids
`CtechDfe-{Env}-{Name}`; OIDC is global.

| Stack | id | Creates |
|-------|-----|---------|
| `OidcStack` | `CtechDfe-Global-OIDC` | GitHub Actions OIDC deploy roles |
| `DynamoDBStack` | `CtechDfe-{Env}-DynamoDB` | 47 tables, including streamed worker outbox (§4) |
| `S3Stack` | `CtechDfe-{Env}-S3` | Certificates + Documents buckets |
| `EventBusStack` | `CtechDfe-{Env}-EventBus` | SNS event/results/ops-alerts + results and erasure SQS, no alarms |
| `DfeStack` | `CtechDfe-{Env}-Dfe` | py-dfe Lambda + layer (retired, still deployed) |
| `GoDfeEgressStack` | `CtechDfe-{Env}-GoDfeEgress` | go-dfe-egress Lambda, **sa-east-1** (§2) |
| `WorkerStack` | `CtechDfe-{Env}-Worker` | 10 workers + DLQs + dispatcher + outbox publisher (§2/§3) |
| `IAMStack` | `CtechDfe-{Env}-IAM` | Lambda/API roles + policies |
| `ApiStack` | `CtechDfe-{Env}-API-V2` | EC2 ASG + HAProxy route (§5) |

## 2. Lambdas

**Workers** (`lib/worker-definitions.ts`, built via `goCode` → Go `PROVIDED_AL2023`/`arm64`,
handler `bootstrap`; every one is memory 128 except `distribution`):

| id | function | binary | timeout (s) | SEFAZ services |
|----|----------|--------|-------------|----------------|
| nfe-emission | `${env}-nfe-emission-worker` | worker | 300 | NFeAutorizacao |
| nfe-event | `${env}-nfe-event-worker` | worker | 150 | RecepcaoEvento |
| nfe-inutilization | `${env}-nfe-inutilization-worker` | worker | 150 | NfeInutilizacao |
| cte-emission | `${env}-cte-emission-worker` | worker | 300 | CTeRecepcao{Sinc,OS,GTVe,Simp} |
| cte-event | `${env}-cte-event-worker` | worker | 150 | CTeRecepcaoEvento |
| mdfe-emission | `${env}-mdfe-emission-worker` | worker | 300 | MDFeRecepcaoSinc |
| mdfe-event | `${env}-mdfe-event-worker` | worker | 150 | MDFeRecepcaoEvento |
| nfse-emission | `${env}-nfse-emission-worker` | worker | 300 | NFSeRecepcao |
| nfse-event | `${env}-nfse-event-worker` | worker | 150 | NFSeEvento |
| distribution | `${env}-distribution-worker` | distribution-worker (mem 256) | 300 | — (SQS direct) |

Event-type workers use 150 s (`SEFAZ_CALL_TIMEOUT_SECONDS`) so they outlive the egress Lambda (120 s).

Extra Lambdas (in `lib/worker-stack.ts`, not in worker-definitions):
- **DLQ processor**: one shared `${env}-dfe-dlq-processor` (timeout 30 / mem 128), created once and fed by the
  worker DLQ; it publishes `status=failed` to the results SNS topic.
- **outbox-publisher**: `${env}-dfe-outbox-publisher`, consumes `${p}_worker_outbox` `NEW_IMAGE` stream records,
  publishes command SNS, and conditionally acknowledges the row; its own DLQ has no alarm.
- **distribution-dispatcher**: `${env}-distribution-dispatcher`, timeout 60 / mem 128,
  EventBridge schedule every **30 min**.
- **go-dfe-egress** (`lib/egress-stack.ts`, `GoDfeEgressStack`): `${env}-go-dfe-egress` in **sa-east-1**,
  `provided.al2023` arm64, timeout 120 s / mem 256. Every SEFAZ and municipal call goes through it
  (referenced from other stacks by name/ARN strings only, since it is cross-region).
- **py-dfe**: `${env}-py-dfe`, Python 3.14, arm64, timeout 30 / mem 512, handler
  `py_dfe.handler.handler`. **Retired**: no worker or API code invokes it (they call the egress Lambda via
  `DFE_LAMBDA_NAME`), but `DfeStack`, its role and invoke policy are still deployed. Removal is a separate change.

## 3. SQS / SNS (`lib/event-bus-stack.ts`, `lib/worker-stack.ts`)

All **standard** (no FIFO anywhere).
- SNS: `${env}-ctech-dfe` (event/command bus), `${env}-ctech-dfe-results` (results),
  `${env}-ctech-dfe-results-ops-alerts`, `${env}-dfe-ops-alerts` — both kept for out-of-band
  subscriptions; no alarm publishes to either (the DLQ alarms were removed from the CDK). The family alert topic
  is `ctech-{env}-alerts` (owned by ctech-cdk); this service does not publish to it yet.
- SQS: per-worker main queue `${env}-{queue}` → DLQ, `maxReceiveCount: 3`; DLQ retention
  **14 days**. Results queue retention **1h**, DLQ 14d
 . Event source `batchSize: 1`, `reportBatchItemFailures: true`
 .

## 4. DynamoDB (`lib/dynamodb-stack.ts`)

`TableV2`, **`Billing.onDemand({ maxReadRequestUnits: 1000, maxWriteRequestUnits: 1000 })`**
(on-demand, capped — NOT provisioned autoscaling). `RemovalPolicy` DESTROY in dev / RETAIN
otherwise (so `stage` is RETAIN); **PITR prod only**. 47 tables, physical name `{env}_dfe_{name}`
(org-owned entities as `{env}_dfe_organization_{name}`); notable GSIs:

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

The `TableName` union  is a **logical-name enum**, not the physical name — map via
the `getDfeTable`/`getEventsTable`/`getDistributionTable`/`getDfeConfigTable` builders.

## 5. IAM & EC2 (`lib/iam-stack.ts`, `lib/api-stack.ts`, `lib/oidc-stack.ts`)

- Roles: `${env}-py-dfe-lambda-role`, `${env}-ctech-dfe-api-v2-role`, per-worker roles,
  dispatcher role, DLQ-processor roles. `DynamoPolicy` grants CRUD + Query/**Scan** on all
  tables + `/index/*`; S3 on cert/doc buckets; SNS publish on the
  event bus; SQS receive on results + send on distribution; SSM `GetParameter` on
  `/ctech-dfe|ctech-account|ctech/${env}/*`.
- The API role can tag objects in the documents bucket. PDFs tagged
  `cache=auxiliary-document` expire after 30 days; in versioned production buckets,
  noncurrent tagged versions expire after one additional day.
- **API on an EC2 ASG routed by `ctech-lbalancer` HAProxy**
  (`api-stack.ts`): the shared `HaproxyEc2Service` creates the encrypted launch
  template, ASG, SG, log groups and CPU target tracking. Capacity is
  `minCapacity: 1`, `maxCapacity: 2` (all environments). The binary `app` runs
  via systemd behind nginx (`:8080`, per-IP/per-tenant rate limits). No ALB, target group or
  listener rule is synthesized; the retained `/ctech/{env}/network/alb-sg-id` SSM parameter only
  names the shared edge security group.
- User data installs the Cloudflare Origin CA (`setup-cloudflare-ca.sh` from ctech-cdk assets) so private
  `*.internal.aoctech.app` TLS endpoints validate without disabling verification.
- CloudWatch agent config is **logs only** (app and nginx log groups); the host-metric series that earlier
  versions published were removed. EC2's native `CPUUtilization`/`CPUCreditBalance` remain the CPU source.
- OIDC: GitHub Actions deploy roles gated on `repo:{githubRepo}:*` for api/infra/
  pydfe (retired Lambda, role still present)/worker (`oidc-stack.ts`), plus `ctech-dfe-gha-scopes`. The scope role can
  read only Account's app URL and DFe publisher client ID/secret parameters.

## 6. Frontend

There is no frontend stack in this repo: the UI is a static export deployed to Cloudflare Workers Static
Assets by `.github/workflows/frontend.yml` (reusable workflow `frontend-cloudflare.yml` in ctech-cdk). The
browser calls the API host directly (CORS applies). `FrontendStack`/CloudFront no longer exist in `cdk/lib`.

## 7. Deploy & cost

- `cdk.json:2` → `npx ts-node bin/ctech-dfe-cdk.ts`. `package.json`
  `deploy:prod = ENVIRONMENT=prod cdk deploy --all --require-approval never --profile ctech`.
  Env config via env vars (`ENVIRONMENT`, `CTECH_VPC_ID`, `CTECH_DEPLOYMENTS_BUCKET`,
  `CTECH_LOGS_BUCKET`, `GITHUB_REPO`); shared infra (VPC, edge SG, Valkey, buckets) from SSM.
- Before deploying the API, run from `ctech-cdk`:
  `CTECH_AWS_PROFILE=ctech ./scripts/configure-service-url-parameters.sh {env}`.
  The API resolves account transport/JWKS and its own audience/CORS URLs from SSM
  on every service start; changing them no longer changes the CloudFormation template.
- **Keep-warm (cost driver)**: for each of the 10 workers a `scheduler.Schedule`
  `${env}-{name}-ping-schedule` invokes the worker with `{ping:true}` at
  **`Duration.minutes(1)`** (`worker-stack.ts`) — 1440 pings/day per worker. **Code/comment mismatch**: the
  comment in `worker-stack.ts` says "every 5 min"; the code says 1 min, the code wins. No Lambda provisioned
  concurrency; EC2 `maxCapacity` 2; DynamoDB on-demand capped 1000 RU/WU.

## 8. Known divergences (documented honestly)

- **B14** — DynamoDB IAM policy grants `Scan`, consumed by the
  distribution-dispatcher's org enumeration (`worker/README.md` §6).
- Keep-warm rate is 1 min (code) vs 5 min (comment) — B21 cost note.
- py-dfe is retired but its stack, IAM and OIDC role are still deployed.
- DLQs have no alarm; alerts to the family SNS topic `ctech-{env}-alerts` are not wired yet.
- ASG `gracePeriod: 120s` is supplied to the shared `HaproxyEc2Service` construct.
- The API runs `t4g.nano` On-Demand (`HaproxyEc2Service` `onDemand: true`, `@aoctech/cdk` >= 0.10.0),
  covered by the `t4g` EC2 Instance Savings Plan. Spot and the micro fallback were dropped after
  repeated Spot capacity shortages.

See root [`DEPLOYMENT.md`](../DEPLOYMENT.md), [`CONDUCT.md`](../CONDUCT.md),
[`DOCS.md`](../DOCS.md).
