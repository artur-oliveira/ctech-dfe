# DLQ redrive: dfe

Queues (CDK; `{env}` = `prod`):

| DLQ | Fed by | Source in CDK |
|---|---|---|
| `{env}-dfe-dlq` | parking queue, no consumer: SNS deliveries to a worker Lambda that SNS gave up on (subscription redrive) and failure envelopes the `dlq-processor` itself could not handle | `cdk/lib/worker-stack.ts` |
| `{env}-dfe-outbox-publisher-dlq` | outbox publisher Lambda failures | `cdk/lib/worker-stack.ts` |
| `{env}-ctech-dfe-results-dlq` | `{env}-ctech-dfe-results` after 3 receives | `cdk/lib/event-bus-stack.ts` |
| `{env}-ctech-dfe-erasure-dlq` | `{env}-ctech-dfe-erasure` after 5 receives | `cdk/lib/event-bus-stack.ts` |

Worker failures no longer pass through a queue: after Lambda's 3 tries (or 1 h event age) the `{env}-dfe-dlq-processor` Lambda is
invoked as the on-failure destination, marks the document failed, publishes `status=failed` to the results topic and alerts
(`dlq-exhausted`). Messages that stay in `{env}-dfe-dlq` are ones that processor itself failed on, or SNS deliveries that never
reached a worker. Redriving them re-runs an emission: check the document's state in DynamoDB first and prefer the user retrying
with a new `Idempotency-Key`, because a SEFAZ call may already have succeeded. Details and exact commands:
`docs/specs/2026-11-02-sns-lambda-worker-delivery.md` (section "Parking queue runbook").

## Alert

None fires on DLQ depth: the family declined CloudWatch alarms (dfe). Consumer-side go-common `alerts` to `ctech-{env}-alerts` cover the rest: `dlq-exhausted` (the DLQ processor marked a document or event failed), `dlq-processor-failed` (it could not; it retries, then parks the event in `{env}-dfe-dlq`), `dlq-unparseable` (it dropped a payload it could not parse), `outbox-stuck`, and `erasure-purge-failed` (an account-deletion purge failed and the message stays for redelivery). A message that lands in `{env}-dfe-dlq` through SNS subscription redrive (delivery failure) is silent: look at the queue after a failed emission. (The `{env}-dfe-ops-alerts` and `{env}-ctech-dfe-results-ops-alerts` topics are kept only so externally added subscriptions survive; nothing publishes to them.)

## Inspect

```bash
export AWS_PROFILE=ctech AWS_REGION=us-east-1
Q=$(aws sqs get-queue-url --queue-name <dlq-name> --query QueueUrl --output text)
aws sqs get-queue-attributes --queue-url "$Q" --attribute-names ApproximateNumberOfMessages
aws sqs receive-message --queue-url "$Q" --max-number-of-messages 10 --visibility-timeout 0 \
  --message-attribute-names All
```

`--visibility-timeout 0` peeks without hiding the message. Messages are kept 14 days; read the cause in the
service log group (CloudWatch Logs) before redriving, or the message will just come back.

## Redrive

Fix the cause first, then move the messages back to their source queue:

```bash
aws sqs start-message-move-task --source-arn arn:aws:sqs:us-east-1:868899309401:<dlq-name>
aws sqs list-message-move-tasks --source-arn arn:aws:sqs:us-east-1:868899309401:<dlq-name>
```

Without `--destination-arn` the messages go back to the queue that dead-lettered them (not applicable to `{env}-dfe-dlq`, see the spec). Use `--max-number-of-messages-per-second`
to throttle. Cancel with `aws sqs cancel-message-move-task --task-handle <handle>`.
For a DLQ filled by something that is not a queue (Scheduler, Lambda stream failure) there is no source queue:
pass `--destination-arn` of a queue that can take the payload, or replay by hand and `aws sqs purge-queue` after.
