# Deploy capacity: dfe

Derived from `cdk/lib/api-stack.ts`, `cdk/lib/worker-stack.ts`, `cdk/lib/worker-definitions.ts`, `cdk/lib/event-bus-stack.ts`, `cdk/lib/egress-stack.ts`, `cdk/lib/dynamodb-stack.ts` and the shared `HaproxyEc2Service` in `@aoctech/cdk` (ctech-cdk). The CDK is the source of truth; update this page when it changes.

## API (EC2 ASG)

| Item | Value |
|---|---|
| Instance type | `t4g.nano` (arm64, 0.5 GiB), the `HaproxyEc2Service` default; no override |
| Purchase | On-demand (`onDemand: true`) |
| ASG | min 1, max 2 (+1 headroom to start the replacement before terminating the old one) |
| Rolling update | `minInstancesInService = 1` |
| Image / disk | Alpine arm64 AMI from SSM, root volume 3 GiB gp3 (shared default), 256 MB swap |
| Process layout | nginx in front of two app processes (`APP_PORT` / `APP_PORT_ALT`); `GOMEMLIMIT` is 75% of RAM split across them |
| Schedule | None (the daytime-window `schedule` is commented out): always on |

## Workers (Lambda `provided.al2023`, arm64)

- Ten SQS-triggered workers (`worker-definitions.ts`): NFe, CTe, MDFe and NFSe emission (300 s timeout, 128 MB), the matching event/inutilization workers (150 s, 128 MB), and `distribution-worker` (300 s, 256 MB). Each reads its own queue with `batchSize: 1`, `maxBatchingWindow: 5 min`, `reportBatchItemFailures`.
- Queue visibility timeout is `6 x timeout + 300 s`; messages move to the shared worker DLQ (`{env}-dfe-dlq`) after 3 receives, retention 14 days.
- Erasure queue: DLQ after 5 receives; results queue has its own DLQ (see `dlq-redrive.md`).
- Keep-warm: every worker is pinged by EventBridge Scheduler **every 1 minute** (`rate(1 minutes)`, 10 workers = about 14,400 invocations per day). The comment in `worker-stack.ts` still says 5 minutes; the code is the truth. Raise the interval if cost matters more than cold starts.
- Distribution dispatcher: Lambda 128 MB, 60 s, `rate(30 minutes)`.
- SEFAZ egress (`egress-stack.ts`): Lambda in `sa-east-1`, 256 MB, 120 s timeout, because SEFAZ endpoints must be called from Brazil.

## DynamoDB

- On-demand with a ceiling of **1000 RRU / 1000 WRU per table and per GSI**. PITR in prod only; tables `RETAIN` outside dev (`DESTROY` in dev). Existing tables are never recreated (Terraform import is a separate change).
- Temporary rows (idempotency keys, erasure state) carry a TTL.

## Known limits

- One `t4g.nano` API: during a deploy or an instance loss only the surviving instance (max 2) serves. No scaling policy: raise min/max in CDK.
- 0.5 GiB RAM with 256 MB swap: watch swap before raising concurrency.
- Worker concurrency is bounded only by SQS and Lambda account limits; a bulk emission is throttled downstream by SEFAZ, and the per-user API rate limit is 1200 reads / 300 writes per minute.
- The 1000 RRU/WRU ceiling throttles a burst of emissions before the Lambdas do; throttled writes surface as retries and, after 3 receives, DLQ entries that now raise a `dlq-exhausted` alert.
