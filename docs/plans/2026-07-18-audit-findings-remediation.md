# ctech-dfe Audit Findings Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resolve the 8 remaining in-scope items from `docs/specs/2026-07-18-audit-findings-remediation-design.md` — DLQ
observability + terminal-status write, the SQS FIFO doc/idempotency correction (decided: keep standard SQS), stale
documentation (FIFO claims, Beanstalk migration line, org header name), S3 bucket hardening (RemovalPolicy + lifecycle),
turning on the distribution-poller, and an orphaned test file.

**Architecture:** Surgical changes only, in three subprojects: `worker/` (Go Lambda), `cdk/` (TypeScript CDK), and
root/subproject docs. No new services, no schema changes, no new dependencies. Every code change reuses a pattern that
already exists elsewhere in the repo (the `ConditionExpression` idempotency guard already used in `distribution.go`, the
`isProduction ? ... : ...` conditional already used in `s3-stack.ts`, the mockable-interface-over-package-var pattern
already used for `snsClient`).

**Tech Stack:** Go 1.26 (`aws-sdk-go-v2`), AWS CDK v2.258 (TypeScript, `jest` + `ts-jest`).

## Global Constraints

- Go: `go test ./... -race` from `worker/` must pass after every Go task.
- CDK: `npm test` (jest, tests live in `cdk/test/*.test.ts`) must pass, and `cdk synth` must succeed cleanly, after
  every CDK task.
- No magic strings: reuse existing constants (`service.StatusFailed`, `service.EventStatusError`, etc.) — do not
  redeclare status strings.
- `cdk.RemovalPolicy.DESTROY` must never apply unconditionally to a resource that exists in production (`cdk/CLAUDE.md`
  rule) — Task 6 fixes the one existing violation; no new task may introduce another.
- Every diff must be minimal — no drive-by refactors, renames, or formatting changes to adjacent code (root `CLAUDE.md`
  Scope Control).
- Do not touch anything under `ctech-account`, `ctech-wallet`, or `ctech-cdk` — out of scope per the spec.

---

### Task 1: DLQ processor writes a terminal status to DynamoDB + IAM grant

**Files:**

- Modify: `worker/cmd/dlq-processor/main.go`
- Create: `worker/cmd/dlq-processor/main_test.go`
- Modify: `cdk/lib/worker-stack.ts` (dlq-role policy, lines 228-231 today)
- Create: `cdk/test/worker-stack.test.ts`

**Interfaces:**

- Consumes: `service.WorkerMessage` (existing, `worker/internal/service/dfe.go:73-92`), `service.StatusFailed` /
  `service.EventStatusError` (existing, `worker/internal/service/helpers.go:14,19`).
- Produces: nothing later tasks depend on (leaf change). Task 2 edits the same two files but different regions — no
  signature overlap.

- [ ] **Step 1: Write the failing Go test**

Create `worker/cmd/dlq-processor/main_test.go`:

```go
package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type fakeDynamo struct {
	calls []*dynamodb.UpdateItemInput
	err   error
}

func (f *fakeDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls = append(f.calls, in)
	return &dynamodb.UpdateItemOutput{}, nil
}

func attrS(t *testing.T, av types.AttributeValue) string {
	t.Helper()
	s, ok := av.(*types.AttributeValueMemberS)
	if !ok {
		t.Fatalf("attribute is not a string: %#v", av)
	}
	return s.Value
}

func ptr(s string) *string { return &s }

func TestHandler_DocumentMessage_WritesFailedStatus(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = ""
	fd := &fakeDynamo{}
	dynamoClient = fd

	msg := service.WorkerMessage{
		DocPK:     "prod#CNPJ_12345678000195",
		AccessKey: "35250512345678000195550010000000011000000011",
		TableName: "nfes",
	}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m1", Body: string(body)}}}

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(fd.calls) != 1 {
		t.Fatalf("expected 1 UpdateItem call, got %d", len(fd.calls))
	}
	call := fd.calls[0]
	if *call.TableName != "dev_nfes" {
		t.Errorf("table = %q, want dev_nfes", *call.TableName)
	}
	if attrS(t, call.Key["pk"]) != msg.DocPK {
		t.Errorf("pk = %q, want %q", attrS(t, call.Key["pk"]), msg.DocPK)
	}
	if attrS(t, call.Key["sk"]) != msg.AccessKey {
		t.Errorf("sk = %q, want %q", attrS(t, call.Key["sk"]), msg.AccessKey)
	}
	if attrS(t, call.ExpressionAttributeValues[":status"]) != service.StatusFailed {
		t.Errorf("status = %q, want %q", attrS(t, call.ExpressionAttributeValues[":status"]), service.StatusFailed)
	}
	if call.ConditionExpression == nil || *call.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition expression = %v, want attribute_exists(pk)", call.ConditionExpression)
	}
}

func TestHandler_EventMessage_WritesEventErrorStatus(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = ""
	fd := &fakeDynamo{}
	dynamoClient = fd

	eventsTable := "nfe_events"
	eventSK := "01930000-0000-7000-8000-000000000001"
	msg := service.WorkerMessage{
		DocPK:           "prod#CNPJ_12345678000195",
		AccessKey:       "35250512345678000195550010000000011000000011",
		TableName:       "nfes",
		EventsTableName: &eventsTable,
		EventType:       ptr("110111"),
		EventSK:         &eventSK,
	}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m2", Body: string(body)}}}

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(fd.calls) != 1 {
		t.Fatalf("expected 1 UpdateItem call, got %d", len(fd.calls))
	}
	call := fd.calls[0]
	if *call.TableName != "dev_nfe_events" {
		t.Errorf("table = %q, want dev_nfe_events", *call.TableName)
	}
	if attrS(t, call.Key["pk"]) != msg.AccessKey {
		t.Errorf("pk = %q, want %q", attrS(t, call.Key["pk"]), msg.AccessKey)
	}
	if attrS(t, call.Key["sk"]) != eventSK {
		t.Errorf("sk = %q, want %q", attrS(t, call.Key["sk"]), eventSK)
	}
	if attrS(t, call.ExpressionAttributeValues[":status"]) != service.EventStatusError {
		t.Errorf("status = %q, want %q", attrS(t, call.ExpressionAttributeValues[":status"]), service.EventStatusError)
	}
}

func TestHandler_DynamoUpdateFails_StillPublishesSNS(t *testing.T) {
	tablePrefix = "dev"
	resultsTopicARN = "" // keep SNS a no-op for this test; only asserting handler doesn't error
	fd := &fakeDynamo{err: context.DeadlineExceeded}
	dynamoClient = fd

	msg := service.WorkerMessage{DocPK: "pk", AccessKey: "ak", TableName: "nfes"}
	body, _ := json.Marshal(msg)
	event := sqsEvent{Records: []sqsRecord{{MessageID: "m3", Body: string(body)}}}

	if err := handler(context.Background(), event); err != nil {
		t.Fatalf("handler must not fail the whole batch on a DynamoDB error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails to compile**

Run: `cd worker && go test ./cmd/dlq-processor/... -race`
Expected: build failure — `dynamoClient`, `tablePrefix` undefined (they don't exist in `main.go` yet).

- [ ] **Step 3: Rewrite `worker/cmd/dlq-processor/main.go`**

Replace the full file content with:

```go
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"gopkg.aoctech.app/dfe/worker/internal/service"
)

// dynamoUpdater is the DynamoDB subset the DLQ processor needs — narrow
// enough to fake in tests without stubbing the full dynamodb.Client.
type dynamoUpdater interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

var (
	snsClient       *sns.Client
	dynamoClient    dynamoUpdater
	resultsTopicARN string
	tablePrefix     string
)

func init() {
	resultsTopicARN = os.Getenv("RESULTS_TOPIC_ARN")
	tablePrefix = os.Getenv("TABLE_PREFIX")
	ac, err := awscfg.LoadDefaultConfig(context.Background())
	if err != nil {
		panic("aws config: " + err.Error())
	}
	snsClient = sns.NewFromConfig(ac)
	dynamoClient = dynamodb.NewFromConfig(ac)
}

type sqsEvent struct {
	Records []sqsRecord `json:"Records"`
}

type sqsRecord struct {
	MessageID string `json:"messageId"`
	Body      string `json:"body"`
}

const dlqFailureMotive = "Falha após todas as tentativas de reprocessamento"

// terminalUpdateTarget resolves which table/key/status to write for a message
// that exhausted retries — mirrors the document-vs-event routing already used
// by DfeService.failDoc (worker/internal/service/dfe.go).
func terminalUpdateTarget(msg service.WorkerMessage) (table, pk, sk, status string) {
	if msg.EventsTableName != nil && msg.EventSK != nil {
		return tablePrefix + "_" + *msg.EventsTableName, msg.AccessKey, *msg.EventSK, service.EventStatusError
	}
	return tablePrefix + "_" + msg.TableName, msg.DocPK, msg.AccessKey, service.StatusFailed
}

// writeTerminalStatus marks the document or event as terminally failed. This
// is the record of fact; the SNS publish below is a best-effort, real-time
// notification only.
func writeTerminalStatus(ctx context.Context, msg service.WorkerMessage) error {
	table, pk, sk, status := terminalUpdateTarget(msg)
	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET #status = :status, sefaz_motive = :motive, updated_at = :updated"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status":  &types.AttributeValueMemberS{Value: status},
			":motive":  &types.AttributeValueMemberS{Value: dlqFailureMotive},
			":updated": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339Nano)},
		},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	})
	return err
}

func handler(ctx context.Context, event sqsEvent) error {
	for _, record := range event.Records {
		var msg service.WorkerMessage
		if err := json.Unmarshal([]byte(record.Body), &msg); err != nil {
			slog.Error("DLQ: failed to parse message body", "id", record.MessageID, "err", err)
			continue
		}

		slog.Warn("DLQ: message exhausted retries",
			"id", record.MessageID,
			"access_key", msg.AccessKey,
			"doc_pk", msg.DocPK,
		)

		if err := writeTerminalStatus(ctx, msg); err != nil {
			slog.Error("DLQ: failed to write terminal status", "id", record.MessageID, "access_key", msg.AccessKey, "err", err)
		}

		if resultsTopicARN == "" {
			continue
		}

		result := map[string]any{
			"access_key":     msg.AccessKey,
			"doc_pk":         msg.DocPK,
			"table_name":     msg.TableName,
			"status":         service.StatusFailed,
			"sefaz_status":   nil,
			"sefaz_motive":   dlqFailureMotive,
			"sefaz_protocol": nil,
			"xml_s3_key":     nil,
		}
		msgJSON, _ := json.Marshal(result)

		if _, err := snsClient.Publish(ctx, &sns.PublishInput{
			TopicArn: aws.String(resultsTopicARN),
			Message:  aws.String(string(msgJSON)),
		}); err != nil {
			slog.Error("failed to publish DLQ result", "id", record.MessageID, "err", err)
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
```

Note: the SNS notification payload's `status` field stays hardcoded to `service.StatusFailed` even for event messages —
this matches the pre-existing SNS shape exactly (out of scope to change here); only the new DynamoDB write branches on
document vs. event.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd worker && go test ./cmd/dlq-processor/... -race -v`
Expected: `PASS` for all three new tests.

- [ ] **Step 5: Run the full worker test suite to confirm no regressions**

Run: `cd worker && go test ./... -race`
Expected: `ok` for every package.

- [ ] **Step 6: Write the failing CDK test**

Create `cdk/test/worker-stack.test.ts`:

```ts
import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'
import * as sns from 'aws-cdk-lib/aws-sns'
import { WorkerStack } from '../lib/worker-stack'
import { WORKERS } from '../lib/worker-definitions'

function buildTemplate(): Template {
  const app = new cdk.App()
  const busStack = new cdk.Stack(app, 'BusStack')
  const eventBus = new sns.Topic(busStack, 'EventBus')
  const resultsTopic = new sns.Topic(busStack, 'Results')

  const stack = new WorkerStack(app, 'TestWorkerStack', {
    environment: 'dev',
    tablePrefix: 'dev_dfe',
    eventBus,
    workers: WORKERS,
    certificatesBucketName: 'dev-ctech-dfe-certificates',
    documentsBucketName: 'dev-ctech-dfe-documents',
    dfeLambdaName: 'dev-py-dfe',
    resultsTopicArn: resultsTopic.topicArn,
  })
  return Template.fromStack(stack)
}

test('every DLQ processor role can UpdateItem on its worker tables', () => {
  const template = buildTemplate()
  const workersWithTables = WORKERS.filter(w => w.dynamoTables?.length)

  template.resourceCountIs('AWS::IAM::Role', WORKERS.length * 2 + 1) // worker role + dlq role per worker, + dispatcher role
  template.hasResourceProperties('AWS::IAM::Policy', {
    PolicyDocument: {
      Statement: Template.arrayWith === undefined ? undefined : undefined,
    },
  })
  // Assert at least one dlq-role policy grants dynamodb:UpdateItem — precise
  // per-worker matching is done via a raw template scan since role names are
  // tokens at synth time.
  const json = template.toJSON()
  const policies = Object.values(json.Resources).filter(
    (r: any) => r.Type === 'AWS::IAM::Policy'
  ) as any[]
  const updateItemPolicies = policies.filter(p =>
    JSON.stringify(p.Properties.PolicyDocument.Statement).includes('dynamodb:UpdateItem')
  )
  expect(updateItemPolicies.length).toBeGreaterThanOrEqual(workersWithTables.length)
})
```

- [ ] **Step 7: Run test to verify it fails**

Run: `cd cdk && npm test -- worker-stack`
Expected: FAIL — `updateItemPolicies.length` is 0 (only the main worker role has `dynamodb:UpdateItem` today, from an
unrelated grant that also includes it — check the failure message; if it unexpectedly passes because the main worker
role's policy already contains the string, tighten the filter to `r.Properties.Roles` referencing a `*-dlq-role` logical
ID before proceeding to Step 8).

- [ ] **Step 8: Add the IAM grant in `cdk/lib/worker-stack.ts`**

Find this block (existing code, right after the DLQ processor role is created):

```ts
      dlqRole.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['sns:Publish'],
        resources: [resultsTopicArn],
      }))
```

Replace with:

```ts
      dlqRole.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['sns:Publish'],
        resources: [resultsTopicArn],
      }))

      if (worker.dynamoTables?.length) {
        const dlqTableArns = worker.dynamoTables.flatMap(t => [
          `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}`,
          `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}/index/*`,
        ])
        dlqRole.addToPrincipalPolicy(new iam.PolicyStatement({
          actions: ['dynamodb:UpdateItem'],
          resources: dlqTableArns,
        }))
      }
```

- [ ] **Step 9: Run test to verify it passes**

Run: `cd cdk && npm test -- worker-stack`
Expected: PASS.

- [ ] **Step 10: `cdk synth` sanity check**

Run: `cd cdk && npx cdk synth CtechDfe-Dev-Worker`
Expected: synthesizes cleanly (no errors). Requires `ENVIRONMENT=dev` and the usual local AWS/Go toolchain — if this
isn't runnable in the current sandbox, note it and rely on the jest test instead.

- [ ] **Step 11: Commit**

```bash
git add worker/cmd/dlq-processor/main.go worker/cmd/dlq-processor/main_test.go cdk/lib/worker-stack.ts cdk/test/worker-stack.test.ts
git commit -m "fix(worker,cdk): write terminal status to DynamoDB from DLQ processor"
```

---

### Task 2: CloudWatch Alarm on every DLQ, wired to a new ops-alerts SNS topic

**Files:**

- Modify: `cdk/lib/worker-stack.ts` (imports + one new topic + one alarm per worker in the existing loop)
- Modify: `cdk/test/worker-stack.test.ts` (extend, created in Task 1)

**Interfaces:**

- Consumes: `dlq` (existing local `sqs.Queue` per worker, `worker-stack.ts:101`).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Extend the CDK test file (failing first)**

Append to `cdk/test/worker-stack.test.ts`:

```ts
test('every DLQ has a CloudWatch alarm wired to the ops-alerts topic', () => {
  const template = buildTemplate()

  template.resourceCountIs('AWS::CloudWatch::Alarm', WORKERS.length)
  template.hasResourceProperties('AWS::CloudWatch::Alarm', {
    ComparisonOperator: 'GreaterThanOrEqualToThreshold',
    EvaluationPeriods: 1,
    Threshold: 1,
    Namespace: 'AWS/SQS',
    MetricName: 'ApproximateNumberOfMessagesVisible',
  })
  template.resourceCountIs('AWS::SNS::Topic', 3) // EventBus + Results (test-local) + the new ops-alerts topic
})
```

Note: the `resourceCountIs('AWS::SNS::Topic', 3)` counts topics created *inside* `WorkerStack` plus the two passed in
via props from `busStack` in the same synthesized app — if this assertion is flaky across CDK versions because
cross-stack topics aren't included in a single stack's `Template.fromStack`, drop that line and instead assert
`template.resourceCountIs('AWS::SNS::Topic', 1)` (only the ops-alerts topic is created inside `WorkerStack` itself).

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cdk && npm test -- worker-stack`
Expected: FAIL — 0 `AWS::CloudWatch::Alarm` resources found.

- [ ] **Step 3: Add imports**

At the top of `cdk/lib/worker-stack.ts`, after the existing `import * as iam from 'aws-cdk-lib/aws-iam'` line, add:

```ts
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch'
import * as cwActions from 'aws-cdk-lib/aws-cloudwatch-actions'
```

- [ ] **Step 4: Create the ops-alerts topic once, before the worker loop**

Find:

```ts
    const queueById = new Map<string, sqs.Queue>()

    for (const worker of workers) {
```

Replace with:

```ts
    const queueById = new Map<string, sqs.Queue>()

    const opsAlertsTopic = new sns.Topic(this, 'ops-alerts-topic', {
      topicName: `${environment}-dfe-ops-alerts`,
    })

    for (const worker of workers) {
```

- [ ] **Step 5: Add the alarm right after each DLQ is created**

Find:

```ts
      const dlq = new sqs.Queue(this, `${worker.id}-dlq`, {
        queueName: `${environment}-${worker.queueName}-dlq`,
        retentionPeriod: Duration.days(14),
      })
```

Replace with:

```ts
      const dlq = new sqs.Queue(this, `${worker.id}-dlq`, {
        queueName: `${environment}-${worker.queueName}-dlq`,
        retentionPeriod: Duration.days(14),
      })

      new cloudwatch.Alarm(this, `${worker.id}-dlq-alarm`, {
        alarmName: `${environment}-${worker.queueName}-dlq-alarm`,
        alarmDescription: `One or more messages landed in the ${worker.queueName} DLQ — a fiscal document/event failed after all retries.`,
        metric: dlq.metricApproximateNumberOfMessagesVisible({period: Duration.minutes(1)}),
        threshold: 1,
        evaluationPeriods: 1,
        comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
        treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
      }).addAlarmAction(new cwActions.SnsAction(opsAlertsTopic))
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd cdk && npm test -- worker-stack`
Expected: PASS.

- [ ] **Step 7: `cdk synth` sanity check**

Run: `cd cdk && npx cdk synth CtechDfe-Dev-Worker`
Expected: synthesizes cleanly; template contains `WORKERS.length` `AWS::CloudWatch::Alarm` resources.

- [ ] **Step 8: Commit**

```bash
git add cdk/lib/worker-stack.ts cdk/test/worker-stack.test.ts
git commit -m "feat(cdk): alarm every worker DLQ to a new ops-alerts SNS topic"
```

**Note for the team (not part of this task):** `opsAlertsTopic` has no subscription yet — nobody receives the alert
until an email/Slack/PagerDuty subscription is added to it out-of-band (e.g. via the AWS console or a follow-up change
once the on-call channel is decided). The topic and alarm wiring is the complete scope of this task.

---

### Task 3: Idempotency guard in `DfeService.Process`

**Files:**

- Modify: `worker/internal/service/dfe.go`
- Modify: `worker/internal/service/dfe_test.go`

**Interfaces:**

- Consumes: `service.StatusAuthorized/StatusRejected/StatusFailed/StatusCancelled/StatusClosed`,
  `service.EventStatusSuccess/EventStatusError` (existing, `helpers.go:12-19`).
- Produces: `DynamoClient` interface gains a `GetItem` method — any future test double for `DfeService.Dynamo` must
  implement it (Task's own `mockDynamo` update covers the only current implementer).

- [ ] **Step 1: Write the failing tests**

In `worker/internal/service/dfe_test.go`, add a `GetItem` method to the existing `mockDynamo` (it currently only
implements `UpdateItem`, so the interface change in Step 3 will break compilation until this is added) and add
call-tracking to `mockLambda`, then add the new test cases.

Find:

```go
type mockDynamo struct {
	updates []capturedUpdate
	err     error
}
```

Replace with:

```go
type mockDynamo struct {
	updates []capturedUpdate
	err     error
	// getItemOutput/getItemErr configure the response to GetItem (used by the
	// idempotency guard). Nil getItemOutput.Item (the zero value) means "not
	// found", matching the default behavior needed by every pre-existing test.
	getItemOutput *dynamodb.GetItemOutput
	getItemErr    error
}

func (m *mockDynamo) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	if m.getItemOutput != nil {
		return m.getItemOutput, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}
```

Find:

```go
type mockLambda struct {
	payload []byte
	err     error
}

func (m *mockLambda) Invoke(_ context.Context, _ *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &lambdaSDK.InvokeOutput{Payload: m.payload}, nil
}
```

Replace with:

```go
type mockLambda struct {
	payload []byte
	err     error
	calls   int
}

func (m *mockLambda) Invoke(_ context.Context, _ *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &lambdaSDK.InvokeOutput{Payload: m.payload}, nil
}
```

Then append these new tests at the end of the file:

```go
// ---------------------------------------------------------------------------
// Idempotency guard
// ---------------------------------------------------------------------------

func statusItem(status string) *dynamodb.GetItemOutput {
	return &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{
			"status": &types.AttributeValueMemberS{Value: status},
		},
	}
}

func TestProcess_SkipsWhenDocumentAlreadyTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{getItemOutput: statusItem(StatusAuthorized)}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected invokePyDfe NOT to be called, got %d calls", lamm.calls)
	}
	if len(dynm.updates) != 0 {
		t.Errorf("expected no UpdateItem calls, got %d", len(dynm.updates))
	}
}

func TestProcess_SkipsWhenEventAlreadyTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("135", "Sucesso", "")}
	dynm := &mockDynamo{getItemOutput: statusItem(EventStatusSuccess)}
	svc := newSvc(certS3(), lamm, dynm)

	eventsTable := "nfe_events"
	eventSK := "01930000-0000-7000-8000-000000000001"
	msg := baseMsg
	msg.EventsTableName = &eventsTable
	msg.EventType = strPtr(cancellationEvent)
	msg.EventSK = &eventSK

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected invokePyDfe NOT to be called, got %d calls", lamm.calls)
	}
}

func TestProcess_ProceedsWhenNotYetTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{} // GetItem returns "not found" by default
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 1 {
		t.Errorf("expected invokePyDfe to be called once, got %d", lamm.calls)
	}
}

func TestProcess_GetItemErrorFallsThroughToProcessing(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{getItemErr: errors.New("transient dynamodb error")}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 1 {
		t.Errorf("a GetItem error must not block processing (SEFAZ's own dedup is the backstop): expected 1 call, got %d", lamm.calls)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run test to verify it fails to compile**

Run: `cd worker && go test ./internal/service/... -race`
Expected: build failure — `mockDynamo` does not implement `DynamoClient` yet in the new tests' usage is fine (it already
does via `UpdateItem`), but `s.dynamo.GetItem` doesn't exist on the interface yet, so `newSvc`'s `Clients{Dynamo: dynm}`
line will fail once Step 3 changes the interface. Confirm the failure is specifically about the missing guard behavior
(all 4 new tests fail on assertions, e.g. `lamm.calls != 0` when it's actually 1) before moving to Step 3 if the code
still compiles at this point (it will, since `GetItem` is additive to `mockDynamo` and not yet required by
`DynamoClient`).

- [ ] **Step 3: Add `GetItem` to the `DynamoClient` interface**

In `worker/internal/service/dfe.go`, find:

```go
// DynamoClient is the DynamoDB operations subset used by the worker.
type DynamoClient interface {
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}
```

Replace with:

```go
// DynamoClient is the DynamoDB operations subset used by the worker.
type DynamoClient interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}
```

- [ ] **Step 4: Add the guard function and terminal-status sets**

In `worker/internal/service/dfe.go`, find:

```go
// Process handles a single DFe SEFAZ operation from an SQS message.
func (s *DfeService) Process(ctx context.Context, msg WorkerMessage) error {
	slog.Info("processing dfe",
		"doc_type", msg.DocType,
		"sefaz_service", msg.SefazService,
		"doc_pk", msg.DocPK,
		"access_key", msg.AccessKey,
	)

	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
```

Replace with:

```go
// docTerminalStatuses / eventTerminalStatuses are the statuses that mean "this
// message was already fully processed — do not invoke SEFAZ again."
var docTerminalStatuses = map[string]bool{
	StatusAuthorized: true, StatusRejected: true, StatusFailed: true,
	StatusCancelled: true, StatusClosed: true,
}

var eventTerminalStatuses = map[string]bool{
	EventStatusSuccess: true, EventStatusError: true,
}

// alreadyTerminal reports whether msg's target item is already in a terminal
// status, i.e. this is a redelivered message (standard SQS is at-least-once,
// see docs/specs/2026-07-18-audit-findings-remediation-design.md §2). On a
// DynamoDB read error it fails OPEN (returns false, logs a warning) — SEFAZ's
// own duplicate-rejection (DuplicatedEventError) remains the backstop, so a
// transient read error must not block real processing.
func (s *DfeService) alreadyTerminal(ctx context.Context, msg WorkerMessage) bool {
	var table, pk, sk string
	var terminal map[string]bool

	if msg.EventsTableName != nil && msg.EventSK != nil {
		table = s.cfg.TablePrefix + "_" + *msg.EventsTableName
		pk, sk = msg.AccessKey, *msg.EventSK
		terminal = eventTerminalStatuses
	} else {
		table = s.cfg.TablePrefix + "_" + msg.TableName
		pk, sk = msg.DocPK, msg.AccessKey
		terminal = docTerminalStatuses
	}

	out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: pk},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		ProjectionExpression:     aws.String("#status"),
		ExpressionAttributeNames: map[string]string{"#status": "status"},
	})
	if err != nil {
		slog.Warn("idempotency check failed, proceeding with processing", "access_key", msg.AccessKey, "err", err)
		return false
	}
	if out.Item == nil {
		return false
	}
	statusAttr, ok := out.Item["status"].(*types.AttributeValueMemberS)
	if !ok {
		return false
	}
	return terminal[statusAttr.Value]
}

// Process handles a single DFe SEFAZ operation from an SQS message.
func (s *DfeService) Process(ctx context.Context, msg WorkerMessage) error {
	slog.Info("processing dfe",
		"doc_type", msg.DocType,
		"sefaz_service", msg.SefazService,
		"doc_pk", msg.DocPK,
		"access_key", msg.AccessKey,
	)

	if s.alreadyTerminal(ctx, msg) {
		slog.Info("skipping already-terminal message (redelivery)", "access_key", msg.AccessKey, "doc_pk", msg.DocPK)
		return nil
	}

	certB64, err := s.getCertB64(ctx, msg.CertS3Key)
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd worker && go test ./internal/service/... -race -v`
Expected: `PASS` for the 4 new tests and every pre-existing test in the package.

- [ ] **Step 6: Run the full worker test suite**

Run: `cd worker && go test ./... -race`
Expected: `ok` for every package.

- [ ] **Step 7: Commit**

```bash
git add worker/internal/service/dfe.go worker/internal/service/dfe_test.go
git commit -m "fix(worker): add idempotency guard to DfeService.Process for redelivered messages"
```

---

### Task 4: Fix stale/incorrect documentation (FIFO claims, Beanstalk migration line, org header name)

**Files:**

- Modify: `OVERVIEW.md`
- Modify: `MIGRATION.md`
- Modify: `INTEGRATION.md`
- Modify: `cdk/CLAUDE.md`
- Modify: `worker/CLAUDE.md`
- Modify: `api/CLAUDE.md`

No tests — documentation only.

- [ ] **Step 1: Fix `OVERVIEW.md`**

```
old: | Messaging | SQS (FIFO) + SNS                                            |
new: | Messaging | SQS (standard) + SNS                                        |
```

```
old: **Authentication:** JWT RS256 Bearer validated against JWKS from ctech-account. Org access via `PyDfe-Organization-Pk`
new: **Authentication:** JWT RS256 Bearer validated against JWKS from ctech-account. Org access via `Dfe-Organization-Pk`
```

```
old: worker) · API Gateway · IAM (least privilege) · SQS FIFO · SNS · CloudFront
new: worker) · API Gateway · IAM (least privilege) · SQS (standard) · SNS · CloudFront
```

```
old:   → Send DfeWorkerEvent to SQS FIFO
new:   → Send DfeWorkerEvent to SQS (standard, at-least-once — idempotency enforced at the application layer, see worker/internal/service/dfe.go's alreadyTerminal guard)
```

- [ ] **Step 2: Fix `MIGRATION.md`**

```
old: | Worker                   | SQS FIFO with `org_pk` as `MessageGroupId` — correct ordering guarantee |
new: | Worker                   | SQS standard queues; idempotency enforced via an application-level status guard, not queue ordering |
```

```
old: 2. SQS FIFO event parsing (same schema)
new: 2. SQS standard event parsing (same schema)
```

In the "Target Architecture" mermaid diagram, find:

```
    API -->|AWS SDK v2 FIFO| SQS[SQS FIFO]
```

Replace with:

```
    API -->|AWS SDK v2| SQS[SQS standard]
```

In the "NF-e Issuance Flow" sequence diagram, find:

```
    participant Q as SQS FIFO
```

Replace with:

```
    participant Q as SQS (standard)
```

Find:

```
    A->>Q: SendMessage (DfeWorkerEvent, GroupId=org_pk)
```

Replace with:

```
    A->>Q: SendMessage (DfeWorkerEvent)
```

Find:

```
    Q->>W: SQS trigger (FIFO ordered)
```

Replace with:

```
    Q->>W: SQS trigger (at-least-once, order not guaranteed)
```

(Leave the "Current Architecture" diagram — describing the pre-rewrite Python system — untouched; it is a historical
record outside this task's scope.)

- [ ] **Step 3: Fix `INTEGRATION.md`**

Replace all 4 occurrences of `PyDfe-Organization-Pk` with `Dfe-Organization-Pk` (lines 67, 120, 128, 167 today):

```
old:      │     PyDfe-Organization-Pk: ..  │                               │
new:      │     Dfe-Organization-Pk: ..  │                               │
```

```
old: // All calls inject Authorization and PyDfe-Organization-Pk automatically.
new: // All calls inject Authorization and Dfe-Organization-Pk automatically.
```

```
old: **`ORG_HEADER`** (`'PyDfe-Organization-Pk'`) is defined once in `client.ts`. Never hardcode this string elsewhere.
new: **`ORG_HEADER`** (`'Dfe-Organization-Pk'`) is defined once in `client.ts`. Never hardcode this string elsewhere.
```

```
old: Every API call to api that requires an org scope sends the `PyDfe-Organization-Pk` header. The active org is:
new: Every API call to api that requires an org scope sends the `Dfe-Organization-Pk` header. The active org is:
```

- [ ] **Step 4: Fix `cdk/CLAUDE.md`**

```
old: EC2 ASG (API), ALB, CloudFront, SQS FIFO, SNS, IAM roles, VPC, and CloudWatch.
new: EC2 ASG (API), ALB, CloudFront, SQS (standard), SNS, IAM roles, VPC, and CloudWatch.
```

```
old: │   ├── worker-stack.ts         # Worker Lambdas + SQS FIFO + DLQ
new: │   ├── worker-stack.ts         # Worker Lambdas + SQS (standard) + DLQ + CloudWatch alarms
```

```
old: - `ApiStack` (Elastic Beanstalk) is legacy — migration to `ApiStackV2` (EC2 ASG) is in progress.
new: - `ApiStackV2` (EC2 ASG) is the only API stack; the migration from the legacy `ApiStack` (Elastic Beanstalk) is complete.
```

```
old: - SQS FIFO + DLQ configuration (at-least-once delivery, ordering)
new: - SQS + DLQ configuration (at-least-once delivery; ordering is not guaranteed — idempotency is enforced at the application layer)
```

- [ ] **Step 5: Fix `worker/CLAUDE.md`**

```
old: Go Lambda — SQS FIFO consumer, async DFe issuance pipeline, `provided.al2023`.
new: Go Lambda — SQS (standard) consumer, async DFe issuance pipeline, `provided.al2023`.
```

```
old: Consumes `DfeWorkerEvent` messages from SQS FIFO, orchestrates the full DFe issuance:
new: Consumes `DfeWorkerEvent` messages from SQS (standard), orchestrates the full DFe issuance:
```

```
old: **Flow:** `SQS FIFO → Handler → S3 (cert) → py-dfe Lambda → DynamoDB + S3 + Redis`
new: **Flow:** `SQS (standard) → Handler → S3 (cert) → py-dfe Lambda → DynamoDB + S3 + Redis`
```

```
old: - SQS FIFO provides at-least-once delivery — every handler **MUST be idempotent**.
- Before writing to DynamoDB, check existing state to avoid double-processing.
- `MessageGroupId = org_pk` ensures ordering per organization.
new: - SQS is a standard queue: at-least-once delivery, no ordering guarantee — every handler **MUST be idempotent**.
- Before writing to DynamoDB, check existing state to avoid double-processing (see `DfeService.alreadyTerminal`, `internal/service/dfe.go`).
```

```
old: - DLQ receives messages after max retries — monitor via CloudWatch alarms (configured in CDK).
- `MessageGroupId = org_pk` — messages for the same org are strictly ordered.
new: - DLQ receives messages after max retries — monitored via a CloudWatch alarm per queue (configured in `cdk/lib/worker-stack.ts`).
- SQS is standard (not FIFO) — ordering across messages for the same org is NOT guaranteed; correctness relies on the fiscal-numbering `transact_write` (atomic, order-independent) plus the idempotency guard in `DfeService.Process`.
```

- [ ] **Step 6: Fix `api/CLAUDE.md`**

```
old: - The `PyDfe-Organization-Pk` header name is defined once in `middleware/tenant.go` — never
  hardcoded in route files.
new: - The `Dfe-Organization-Pk` header name is defined once in `middleware/rbac.go` (`OrgHeader`) — never
  hardcoded in route files.
```

```
old: - Organization context is always via `PyDfe-Organization-Pk` header — never path parameters (except org creation
  endpoints).
new: - Organization context is always via `Dfe-Organization-Pk` header — never path parameters (except org creation
  endpoints).
```

```
old: - Lambda invocation for doc issuance is async: API enqueues to SQS FIFO, returns 202, worker processes and pushes
  WebSocket update.
new: - Lambda invocation for doc issuance is async: API enqueues to SQS (standard), returns 202, worker processes and pushes
  WebSocket update.
```

- [ ] **Step 7: Verify no stray mentions remain**

Run:

```bash
grep -rn "FIFO\|MessageGroupId" OVERVIEW.md MIGRATION.md cdk/CLAUDE.md worker/CLAUDE.md api/CLAUDE.md
grep -rn "PyDfe-Organization-Pk" OVERVIEW.md INTEGRATION.md api/CLAUDE.md
```

Expected: no output from either command (the "Current Architecture" diagram in `MIGRATION.md` is the one intentional
exception — if the first grep matches only lines inside that diagram block, that's correct; anything outside it is a
miss).

- [ ] **Step 8: Commit**

```bash
git add OVERVIEW.md MIGRATION.md INTEGRATION.md cdk/CLAUDE.md worker/CLAUDE.md api/CLAUDE.md
git commit -m "docs: correct SQS FIFO claims and org header name across docs"
```

---

### Task 5: Rewrite `DEPLOYMENT.md` for the actual Go/Fiber + systemd stack

**Files:**

- Modify: `DEPLOYMENT.md`

No tests — documentation only. Read the full current file before editing (`Read DEPLOYMENT.md`) since only the first 40
lines and the flagged sections are quoted in the spec — confirm line numbers before editing, they may have shifted
slightly since the audit.

- [ ] **Step 1: Fix the "Infrastructure Overview" placeholder**

Find:

```
# Infrastructure Overview

TODO: Replace with actual stack topology.

```text
AWS Account
│
├── DynamoDB
├── S3
├── Lambda
├── API Infrastructure
├── EC2 Auto Scaling Group
├── Application Load Balancer
├── CloudWatch
└── Systems Manager (SSM)
```

```

Replace the `TODO:` line and restate the topology accurately (the list below it is already correct — only the placeholder sentence is wrong):

```

# Infrastructure Overview

The API runs as a Go/Fiber binary (`app`) on an EC2 Auto Scaling Group behind an Application Load Balancer — see
`cdk/lib/api-v2-stack.ts`. Fiscal issuance runs asynchronously via SQS + Lambda workers (`cdk/lib/worker-stack.ts`) and
the py-dfe Lambda (`cdk/lib/dfe-stack.ts`).

```text
AWS Account
│
├── DynamoDB
├── S3
├── Lambda
├── API Infrastructure
├── EC2 Auto Scaling Group
├── Application Load Balancer
├── CloudWatch
└── Systems Manager (SSM)
```

```

- [ ] **Step 2: Find and fix every `systemctl status app` / FastAPI/Gunicorn troubleshooting section**

Run `grep -n "systemctl\|Gunicorn\|FastAPI\|gunicorn" DEPLOYMENT.md` to get the current exact line numbers and surrounding text (they may have drifted from the audit's `~205-213` / `~362-374`). For each match:
- Replace any reference to "FastAPI"/"Gunicorn" as the running process with "the Go/Fiber binary (`app`), managed by systemd".
- Keep `systemctl status app` / `systemctl restart app` commands as-is if they already use unit name `app` — those are correct (`api-v2-stack.ts` userdata runs `/opt/app/current/app` as a systemd unit); only the prose describing *what* `app` is needs correcting, not the commands themselves.
- If log paths are described as Python/Gunicorn log paths (e.g. `gunicorn.log`), correct them to match whatever log path the actual systemd unit/userdata in `cdk/lib/api-v2-stack.ts` configures (read the userdata script's `journalctl`/log redirection to get the exact path — do not guess).

- [ ] **Step 3: Verify no stale references remain**

Run: `grep -in "fastapi\|gunicorn\|TODO: Replace" DEPLOYMENT.md`
Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add DEPLOYMENT.md
git commit -m "docs: rewrite DEPLOYMENT.md to describe the actual Go/Fiber + systemd stack"
```

---

### Task 6: S3 stack — conditional `RemovalPolicy` + lifecycle rule on the documents bucket

**Files:**

- Modify: `cdk/lib/s3-stack.ts`
- Create: `cdk/test/s3-stack.test.ts`

**Interfaces:**

- Consumes: `isProduction` (existing local const, `s3-stack.ts:22`).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the failing test**

Create `cdk/test/s3-stack.test.ts`:

```ts
import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'
import { S3Stack } from '../lib/s3-stack'

function synth(environment: 'dev' | 'stage' | 'prod') {
  const app = new cdk.App()
  const stack = new S3Stack(app, `TestS3Stack-${environment}`, {
    environment,
    bucketPrefix: `${environment}-ctech-dfe`,
  })
  return Template.fromStack(stack)
}

test('prod buckets are RETAIN, dev buckets are DESTROY', () => {
  const prod = synth('prod')
  prod.allResourcesProperties('AWS::S3::Bucket', {})
  prod.hasResource('AWS::S3::Bucket', { DeletionPolicy: 'Retain' })

  const dev = synth('dev')
  dev.hasResource('AWS::S3::Bucket', { DeletionPolicy: 'Delete' })
})

test('every prod bucket resource is RETAIN (none left as DESTROY)', () => {
  const prod = synth('prod')
  const json = prod.toJSON()
  const buckets = Object.values(json.Resources).filter((r: any) => r.Type === 'AWS::S3::Bucket') as any[]
  expect(buckets.length).toBe(2)
  for (const b of buckets) {
    expect(b.DeletionPolicy).toBe('Retain')
  }
})

test('documents bucket has a Standard-IA lifecycle transition', () => {
  const prod = synth('prod')
  prod.hasResourceProperties('AWS::S3::Bucket', {
    BucketName: 'prod-ctech-dfe-documents',
    LifecycleConfiguration: {
      Rules: Template.arrayWith ? undefined : undefined,
    },
  })
  const json = prod.toJSON()
  const docsBucket = Object.values(json.Resources).find(
    (r: any) => r.Type === 'AWS::S3::Bucket' && r.Properties?.BucketName === 'prod-ctech-dfe-documents'
  ) as any
  expect(docsBucket).toBeDefined()
  const rules = docsBucket.Properties.LifecycleConfiguration.Rules
  const transitionRule = rules.find((r: any) => r.Transitions?.some((t: any) => t.StorageClass === 'STANDARD_IA'))
  expect(transitionRule).toBeDefined()
  expect(transitionRule.Transitions[0].TransitionInDays).toBe(90)
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cdk && npm test -- s3-stack`
Expected: FAIL — both `prod` buckets currently have `DeletionPolicy: Delete` (unconditional `RemovalPolicy.DESTROY`),
and the documents bucket has no `LifecycleConfiguration` at all.

- [ ] **Step 3: Fix `cdk/lib/s3-stack.ts`**

Find:

```ts
    const certificatesBucket = new s3.Bucket(this, 'CertificatesBucket', {
      bucketName: `${bucketPrefix}-certificates`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
      lifecycleRules: [
        {
          expiration: cdk.Duration.days(90),
          prefix: 'temp/',
        },
      ],
    });

    const documentsBucket = new s3.Bucket(this, 'DocumentsBucket', {
      bucketName: `${bucketPrefix}-documents`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
    });
```

Replace with:

```ts
    const certificatesBucket = new s3.Bucket(this, 'CertificatesBucket', {
      bucketName: `${bucketPrefix}-certificates`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
      lifecycleRules: [
        {
          expiration: cdk.Duration.days(90),
          prefix: 'temp/',
        },
      ],
    });

    const documentsBucket = new s3.Bucket(this, 'DocumentsBucket', {
      bucketName: `${bucketPrefix}-documents`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      encryption: s3.BucketEncryption.S3_MANAGED,
      versioned: isProduction,
      removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: !isProduction,
      lifecycleRules: [
        {
          transitions: [
            {
              storageClass: s3.StorageClass.INFREQUENT_ACCESS,
              transitionAfter: cdk.Duration.days(90),
            },
          ],
        },
      ],
    });
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd cdk && npm test -- s3-stack`
Expected: PASS.

- [ ] **Step 5: `cdk synth` sanity check for both environments**

Run:

```bash
cd cdk && ENVIRONMENT=dev npx cdk synth CtechDfe-Dev-S3 && ENVIRONMENT=prod npx cdk synth CtechDfe-Prod-S3
```

Expected: both synthesize cleanly.

- [ ] **Step 6: Commit**

```bash
git add cdk/lib/s3-stack.ts cdk/test/s3-stack.test.ts
git commit -m "fix(cdk): retain S3 buckets in prod, add Standard-IA lifecycle to documents bucket"
```

---

### Task 7: Enable the distribution-poller in production

**Files:**

- Modify: `cdk/lib/worker-stack.ts`
- Modify: `cdk/test/worker-stack.test.ts` (extend, created in Task 1)

**Interfaces:** none — single boolean flag flip.

- [ ] **Step 1: Extend the CDK test**

Append to `cdk/test/worker-stack.test.ts`:

```ts
test('distribution poller schedule is enabled', () => {
  const template = buildTemplate()
  template.hasResourceProperties('AWS::Scheduler::Schedule', {
    State: 'ENABLED',
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cdk && npm test -- worker-stack`
Expected: FAIL — current `State` synthesizes as `DISABLED`.

- [ ] **Step 3: Flip the flag**

Find:

```ts
    new scheduler.Schedule(this, 'DistributionV2Schedule', {
      scheduleName: `${environment}-distribution-schedule`,
      description: 'Triggers distribution dispatcher every 30 minutes to pull DFe documents from SEFAZ',
      schedule: scheduler.ScheduleExpression.rate(Duration.minutes(30)),
      target: new schedulerTargets.LambdaInvoke(dispatcher),
      enabled: false,
    })
```

Replace with:

```ts
    new scheduler.Schedule(this, 'DistributionV2Schedule', {
      scheduleName: `${environment}-distribution-schedule`,
      description: 'Triggers distribution dispatcher every 30 minutes to pull DFe documents from SEFAZ',
      schedule: scheduler.ScheduleExpression.rate(Duration.minutes(30)),
      target: new schedulerTargets.LambdaInvoke(dispatcher),
      enabled: true,
    })
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd cdk && npm test -- worker-stack`
Expected: PASS.

- [ ] **Step 5: Deploy to dev/staging first — do NOT deploy straight to prod**

Per the spec decision (§6.1): validate in `dev`/`staging` before `prod`. This is a deploy step, not a code step — flag
it explicitly to whoever runs the actual `cdk deploy`:

1. Deploy to `dev`, confirm the dispatcher Lambda runs on schedule and enqueues expected volume (CloudWatch Logs for
   `{env}-distribution-dispatcher`).
2. Repeat in `staging`.
3. Only then deploy to `prod`.

- [ ] **Step 6: Commit**

```bash
git add cdk/lib/worker-stack.ts cdk/test/worker-stack.test.ts
git commit -m "feat(cdk): enable distribution-poller schedule"
```

---

### Task 8: Fix or remove the orphaned `marshal_test.go`

**Files:**

- Investigate: `api/internal/repositories/marshal_test.go`
- Investigate: `api/internal/repositories/base.go` (or wherever `MarshalMapOmitNull` now lives)

- [ ] **Step 1: Confirm the test currently compiles and passes**

Run: `cd api && go test ./internal/repositories/... -run TestMarshalMapOmitNull -v`

- [ ] **Step 2: Locate where `MarshalMapOmitNull` is actually defined today**

Run: `cd api && grep -rn "func MarshalMapOmitNull" internal/repositories/`

- [ ] **Step 3a: If the function still exists (in `base.go` or elsewhere) and the test passes** — rename the test file
  to match its target file's name (e.g. `base_test.go` if `MarshalMapOmitNull` lives in `base.go`), so the file name
  isn't misleading about a `marshal.go` that no longer exists:

```bash
cd api/internal/repositories && git mv marshal_test.go base_test.go
```

(Adjust the destination name to whatever file actually defines `MarshalMapOmitNull` — confirm with Step 2's grep output
before renaming.)

- [ ] **Step 3b: If the function no longer exists and the test fails to compile** — this test is dead; delete it:

```bash
cd api/internal/repositories && git rm marshal_test.go
```

- [ ] **Step 4: Run the full api test suite**

Run: `cd api && go test ./... -race`
Expected: `ok` for every package.

- [ ] **Step 5: Commit**

```bash
git add -A api/internal/repositories/
git commit -m "chore(api): rename or remove orphaned marshal_test.go"
```

---

## Explicitly out of scope (do not implement as part of this plan)

- Extracting `api/internal/problem/`, `api/internal/awsclient/`, `api/internal/repositories/base.go` into the shared
  `api-commons` module — spec §7 marks this as a cross-repo decision requiring its own spec in the `api-commons` repo.
- Hash-chaining / S3 Object Lock for `audit_logs` tamper-evidence — spec's "Fora de escopo" section, needs its own
  architecture decision.
- Any finding from `GENERAL-REPORT.md` scoped to `ctech-account`, `ctech-wallet`, or `ctech-cdk`.
