# SNS -> Lambda worker delivery (zero idle SQS/KMS cost)

Branch `chore/aws-zero-idle-cost`. Status: implemented, not deployed.

## Why

Prod CloudWatch: each of the 11 dfe queues (cte/mdfe/nfe/nfse emission+event, nfe-inutilization, distribution, dfe-dlq) received
about 60k empty receives a week, because every Lambda `SqsEventSource` long-polls around the clock. That is about 2.9M SQS
requests a month from dfe alone (account total 4.08M; free tier 1M) for queues that are idle most of the day. DynamoDB tables
used `TableEncryptionV2.awsManagedKey()`, which bills a KMS request per table call, and `certificates.go` wrote with
`aws:kms`, billing a KMS call on every worker `GetObject`.

## New delivery architecture

```
API/outbox -> SNS event bus (${env}-ctech-dfe) --filter sefaz_service--> worker Lambda (async invoke)
API / distribution-dispatcher ------------------- InvocationType=Event ---> distribution-worker Lambda
                       worker Lambda --(3 tries, or 1 h event age)--> on-failure destination = dlq-processor Lambda
                       dlq-processor --> mark doc failed (DynamoDB), publish status=failed (Results SNS), alert (shared topic)
                       dlq-processor failure --(2 retries)--> parking queue ${env}-dfe-dlq (not consumed)
                       SNS cannot deliver --> subscription redrive --> parking queue ${env}-dfe-dlq
```

- 9 SNS-fed workers: `LambdaSubscription` with the same `filterPolicyWithMessageBody` (`sefaz_service` allowlist). Raw message
  delivery does not exist for the Lambda protocol: the published body arrives as `Records[].Sns.Message`. The worker parses it with
  `service.SNSMessages` and then unmarshals `WorkerMessage` exactly as before (`worker/cmd/worker/main.go`).
- `distribution-worker` has no sefaz services. Its payload is the `DistributionMessage` itself (async invoke by the API, 3 call
  sites, and by the dispatcher). `worker/cmd/distribution-worker/main.go` parses it directly.
- Keep-warm pings (`{"ping":true}`) short-circuit first in both handlers (`service.IsPingEvent`).
- The API no longer holds a queue URL: `DFE_DISTRIBUTION_FUNCTION_NAME`, `lambda:InvokeFunction` +
  `lambda:GetFunctionConfiguration` on that one function (`iam-stack.ts`), and `/health-check` reports `lambda_distribution`
  (Lambda control-plane reads are free) instead of `sqs_distribution`. The API gets a second Lambda client in the stack's own
  region (`awsclient.Clients.WorkerLambda`); the existing `Lambda` client stays pinned to the egress region.
- Dispatcher: `DISTRIBUTION_FUNCTION_NAME`, `grantInvoke` on the worker instead of `sqs:SendMessage`.
- Removed: 10 worker queues, their queue policies and SQS subscriptions, 11 SQS event source mappings (10 workers + dlq-processor).
  Kept: `${env}-dfe-dlq` (now a parking queue with no event source mapping, so zero idle requests), the outbox DLQ (a
  send-only DynamoDB-stream failure destination), results and erasure queues.

## Resilience decisions

| Call / hop | Timeout | Retry | DLQ / destination | Fails open or closed |
|---|---|---|---|---|
| SNS -> worker Lambda | SNS-managed | SNS retries Lambda service errors; throttles are retried by Lambda's async queue up to max event age | subscription redrive to `${env}-dfe-dlq` (only for deliveries SNS gives up on) | n/a (delivery) |
| Worker Lambda (SEFAZ emission 300 s; events/inutilization 150 s; distribution 300 s) | function timeout > egress 120 s (unchanged) | async `retryAttempts: 2` = 3 tries (was `maxReceiveCount: 3`); waits 1 min then 2 min | on-failure = `dlq-processor` Lambda; `maxEventAge` 1 h (`ASYNC_MAX_EVENT_AGE`) | closed: DynamoDB claim/lease failure never touches SEFAZ; handler returns the error so retry runs |
| dlq-processor | 30 s | async `retryAttempts: 2` | parking queue `${env}-dfe-dlq` | closed: a failed terminal write returns the error and alerts `dlq-processor-failed` |
| dlq-processor on unparseable payload | | none (nothing can fix it) | alert `dlq-unparseable` (no body in the alert) | open: drop and alert (behaviour from `fix/dlq-unparseable-alert`, preserved here; that branch is not on main, so it is carried in this change) |
| dlq-processor on distribution job | | none | alert `dlq-exhausted`, no DynamoDB write (no document row; next 30-min sweep or user retry redoes it) | open |
| API -> distribution worker invoke | SDK default | none in the API: one failure is returned to the user as 500 (`failed to enqueue ...`), the user retries. A non-202 async status counts as failure | n/a | closed (job not accepted) |
| Dispatcher -> distribution worker invoke | 60 s function | none per job: failure is logged and skipped like the SQS send was; the next 30-min sweep redoes it | n/a | open per org, loop continues |
| Results consumer (api, SQS long poll) | 20 s wait | SQS redelivery, results DLQ | `${env}-ctech-dfe-results-dlq` | unchanged |

### Idempotency under redelivery (verified in code)

- Emission/event workers: `claimProcessing` is a conditional DynamoDB write (status pending/retryable or an expired lease). A
  terminal document republishes its result and does no SEFAZ call. Concurrent delivery gets `errProcessingLeaseHeld` and retries.
- Lease sizing changed for async retries (`leaseFor`, `worker/internal/service/dfe.go`): the lease is the Lambda deadline plus 10 s
  instead of a fixed 6 minutes. Lambda retries 60 s and 120 s after a failed attempt; with the old 6-minute lease a 150 s worker that
  timed out would still hold its lease at the second retry and burn its last attempt on `lease held`, marking a document failed
  while SEFAZ may have accepted it. SQS hid this with a 20-minute visibility timeout.
- Distribution jobs: NSU persistence is conditional ("a re-entrega ... não pode duplicar o registro do NSU"), the hourly
  `cons_*` quota is re-checked in the worker, and `import_xml` is keyed by its staging object.
- During the one-time cut-over both the old queue and the new subscription can deliver the same message; the lease makes the
  second delivery a no-op or a retry.

## Results consumer: the one intentional always-on poller

`api/internal/consumer/results.go` keeps long-polling `${env}-ctech-dfe-results` (20 s wait). Cache invalidation latency matters
more than the request cost. Budget: 3 requests/min = about 131k requests/month per API instance (min 1, max 2 instances).
The erasure consumer (from go-common) adds the same again. Two queues x 1-2 instances stays at or under about 526k/month, inside
the 1M free tier. Unchanged by this work.

## Deploy order and CloudFormation notes

- `bin/ctech-dfe-cdk.ts`: the Worker stack now depends on IAM and API-V2 (it deploys last). IAM and API-V2 imported the
  distribution queue ARN/URL; CloudFormation refuses to delete an export that is still imported, so they now take the function
  name as a plain string (`distributionFunctionName(env)`) and deploy first.
- `cdk synth` diff, prod, against `origin/main`: no table, Lambda function, role or schedule is replaced. DynamoDB: only
  `SSESpecification` changes on 48 `AWS::DynamoDB::GlobalTable` resources (`{SSEEnabled:true,SSEType:KMS}` -> `{SSEEnabled:false}`,
  "Update requires: No interruption" per the CloudFormation reference). Worker stack: removes 10 `SQS::Queue`, 10
  `SQS::QueuePolicy`, 9 `SNS::Subscription` (SQS), 11 `Lambda::EventSourceMapping`; adds 9 Lambda subscriptions, 9 Lambda
  permissions, 11 `EventInvokeConfig`, 1 `SQS::QueuePolicy` (for subscription redrive). Output exports removed:
  `ExportsOutputRefdistributionqueue...`, `ExportsOutputFnGetAttdistributionqueue...Arn`. API-V2: launch template user data changes
  (env var) so the ASG rolls instances. **Flag: removing queues is a deletion; in-flight messages are dropped (drain first).**
- Old API binary + new env: the old binary reads `DFE_DISTRIBUTION_QUEUE_URL`, now absent, so distribution endpoints answer
  400 "fila de distribuição não configurada" until the new API binary is deployed (fails closed, nothing is lost silently).
- Concurrency: queue buffering is gone. A distribution sweep now fires one async invoke per org and doc type at once; Lambda queues
  throttled events itself (up to 1 h). If the account concurrency limit is low, set reserved concurrency on `distribution-worker`
  (not done; follow-up if throttling shows up).

## Drain runbook (before the deploy that deletes the queues)

Run in prod with `export AWS_PROFILE=ctech AWS_REGION=us-east-1`.

1. Find the queues and their depth. Nothing may be visible, in flight or delayed:

   ```bash
   for q in nfe-emission nfe-event nfe-inutilization cte-emission cte-event mdfe-emission mdfe-event nfse-emission nfse-event distribution; do
     url=$(aws sqs get-queue-url --queue-name "prod-$q" --query QueueUrl --output text)
     echo -n "$q "; aws sqs get-queue-attributes --queue-url "$url" \
       --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible ApproximateNumberOfMessagesDelayed \
       --query 'Attributes' --output text
   done
   url=$(aws sqs get-queue-url --queue-name prod-dfe-dlq --query QueueUrl --output text)
   aws sqs get-queue-attributes --queue-url "$url" --attribute-names ApproximateNumberOfMessages --query Attributes --output text
   ```

2. Deploy at a quiet moment. The old event source mappings keep consuming until the Worker stack update, so waiting and
   re-running step 1 drains the queues. Producers (API, outbox publisher, dispatcher every 30 min) keep adding: pick the minute
   after a dispatcher run and re-check right before deploying.
3. A message still in `prod-dfe-dlq` has the old shape (raw `WorkerMessage`) and the new processor does not poll SQS. If it is not
   0, let the old dlq-processor consume it (it does until the deploy) or handle it by hand (step 4).
4. By hand, for anything left in a queue that is about to be deleted: receive it
   (`aws sqs receive-message --queue-url "$url" --max-number-of-messages 10 --visibility-timeout 0`), check the document state in
   DynamoDB, and re-publish the body to the event bus
   (`aws sns publish --topic-arn <prod-ctech-dfe topic arn> --message "$body"`) or, for a distribution job, invoke it:
   `aws lambda invoke --function-name prod-distribution-worker --invocation-type Event --cli-binary-format raw-in-base64-out --payload "$body" /dev/null`.
5. Deploy: `cdk deploy --all` (IAM and API-V2 first, Worker last, then the API binary). Verify on the Deploy job itself, then:
   `aws sns list-subscriptions-by-topic --topic-arn <event bus>` shows 9 `lambda` subscriptions and no `sqs` worker
   subscription; `aws lambda list-event-source-mappings --function-name prod-nfe-emission-worker` is empty;
   `aws lambda get-function-event-invoke-config --function-name prod-nfe-emission-worker` shows 2 retries and the dlq-processor
   destination; `/v1.0/health-check` shows `lambda_distribution` pass.

## Parking queue runbook (`${env}-dfe-dlq`)

Two kinds of message arrive; the body tells them apart. There is no source queue, so SQS "start-message-move-task" does not apply.

- Failure envelope (has `requestPayload`): the dlq-processor failed on it. Fix the cause, then re-run the processor:
  `aws lambda invoke --function-name prod-dfe-dlq-processor --invocation-type Event --cli-binary-format raw-in-base64-out --payload "$body" /dev/null`
- Raw event (a `WorkerMessage` body, with the delivery error in the message attributes): SNS never delivered it. Check the
  document in DynamoDB first (a SEFAZ call may have happened), then re-publish to the event bus with `aws sns publish`.

Delete each message after handling (`aws sqs delete-message`). Retention 14 days. No alarm exists; an operator alert
(`dlq-processor-failed`) accompanies the first kind, the second is silent, so look at the queue after a failed emission.

## Cost note

- SQS: dfe worker queues drop from about 2.9M requests/month to 0 idle. Remaining dfe SQS: results + erasure long polls
  (about 263k/month/instance, see above), well inside the 1M free tier.
- SNS -> Lambda: no per-delivery charge; Lambda async invoke config and destinations are free. `${env}-dfe-dlq` is unconsumed: no
  idle requests.
- DynamoDB: AWS-owned key has no KMS charge (table-side encryption at rest stays on, only the key type changes). Existing items
  are re-encrypted by DynamoDB in the background after the update.
- S3: certificates now written with SSE-S3 (`AES256`, the bucket default), no KMS request per read. Older objects written with
  `aws:kms` stay readable: S3 decrypts them with the AWS-managed `aws/s3` key, whose key policy already lets this account's
  principals use it through S3, so no `kms:Decrypt` grant is added or kept for workers. The only `kms:Decrypt` in the stack is the
  API role's SSM SecureString decrypt, which stays.
- SSM SecureString: the API reads them once at service start through `setup-ssm-env.sh`/`load-ssm-env.sh`; the workers read none
  at runtime (CDK resolves the alerts topic at deploy time). No per-request or per-loop SSM/KMS read exists in dfe code.
- Keep-warm (unchanged, follow-up candidate): 10 workers x 1/min = 432,000 invocations/month (plus 1,440 dispatcher runs).
  Scheduler (14M free) and Lambda requests (1M free) are inside free tiers and compute is a few GB-seconds. The real cost is log
  ingestion: about 4 log lines per ping, roughly 170 MB/month, under US$0.10. Candidate: stop logging the ping line, or raise
  the interval (the cold-start benefit applies only to one warm environment per worker).

## Not verified here

No AWS credentials: `cdk diff` against the deployed stacks and a live SNS -> Lambda end-to-end run were not possible. Replacement
analysis is from comparing synthesized templates of `origin/main` and this branch. Confirm with `cdk diff` in the pipeline before
deploy; also confirm the live event bus name and account concurrency limit.
