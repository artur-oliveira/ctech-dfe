import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'
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
	outboxTableName: 'dev_dfe_worker_outbox',
	outboxTableArn: 'arn:aws:dynamodb:us-east-1:123456789012:table/dev_dfe_worker_outbox',
	outboxStreamArn: 'arn:aws:dynamodb:us-east-1:123456789012:table/dev_dfe_worker_outbox/stream/2026-07-31T00:00:00.000',
  })
  return Template.fromStack(stack)
}

test('every DLQ processor role can UpdateItem on its worker tables', () => {
  const template = buildTemplate()
  const workersWithTables = WORKERS.filter(w => w.dynamoTables?.length)

  const json = template.toJSON()
  const policies = Object.values(json.Resources).filter(
    (r: any) => r.Type === 'AWS::IAM::Policy'
  ) as any[]
  const updateItemPolicies = policies.filter(p =>
    JSON.stringify(p.Properties.PolicyDocument.Statement).includes('dynamodb:UpdateItem')
  )
  expect(updateItemPolicies.length).toBeGreaterThanOrEqual(workersWithTables.length)
})

test('every DLQ has a CloudWatch alarm wired to the ops-alerts topic', () => {
  const template = buildTemplate()

  template.resourceCountIs('AWS::CloudWatch::Alarm', WORKERS.length + 1)
  template.hasResourceProperties('AWS::CloudWatch::Alarm', {
    ComparisonOperator: 'GreaterThanOrEqualToThreshold',
    EvaluationPeriods: 1,
    Threshold: 1,
    Namespace: 'AWS/SQS',
    MetricName: 'ApproximateNumberOfMessagesVisible',
  })
})

// Regression: NFS-e emission silently vanished because no queue was subscribed to
// its sefaz_service — SNS drops what no filter policy matches, with no DLQ and no log.
// Every service the API publishes must be claimed by exactly one worker.
test('every sefaz_service published by the API has an SQS subscription', () => {
  const template = buildTemplate()
  const subscribed = WORKERS.flatMap(w => w.sefazServices)

  for (const service of [
    'NFeAutorizacao', 'RecepcaoEvento', 'NfeInutilizacao',
    'CTeRecepcaoSinc', 'CTeRecepcaoOS', 'CTeRecepcaoGTVe', 'CTeRecepcaoSimp', 'CTeRecepcaoEvento',
    'MDFeRecepcaoSinc', 'MDFeRecepcaoEvento',
    'NFSeRecepcao', 'NFSeEvento',
  ]) {
    expect(subscribed).toContain(service)
  }
  expect(new Set(subscribed).size).toBe(subscribed.length)

  for (const worker of WORKERS.filter(w => w.sefazServices.length > 0)) {
    template.hasResourceProperties('AWS::SNS::Subscription', {
      Protocol: 'sqs',
      RawMessageDelivery: true,
      FilterPolicyScope: 'MessageBody',
      FilterPolicy: {sefaz_service: worker.sefazServices},
    })
  }
})

test('distribution poller schedule is enabled', () => {
  const template = buildTemplate()
  template.hasResourceProperties('AWS::Scheduler::Schedule', {
    State: 'ENABLED',
  })
})

test('every worker has a keep-warm ping schedule invoking it directly with {"ping":true}', () => {
  const template = buildTemplate()

  // One ping schedule per worker + one distribution poller schedule.
  template.resourceCountIs('AWS::Scheduler::Schedule', WORKERS.length + 1)

  for (const worker of WORKERS) {
    template.hasResourceProperties('AWS::Scheduler::Schedule', {
      Name: `dev-${worker.name}-ping-schedule`,
      ScheduleExpression: 'rate(1 minute)',
      State: 'ENABLED',
      Target: Match.objectLike({
        Input: JSON.stringify({ping: true}),
      }),
    })
  }
})
