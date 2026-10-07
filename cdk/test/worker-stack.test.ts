import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'
import * as sns from 'aws-cdk-lib/aws-sns'
import { WorkerStack } from '../lib/worker-stack'
import { WORKERS } from '../lib/worker-definitions'
import { EGRESS_TIMEOUT_SECONDS } from '../lib/egress'

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
    dfeLambdaName: 'dev-go-dfe-egress',
    dfeEgressRegion: 'sa-east-1',
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

test('only the distribution worker can transact on persons and audit logs', () => {
  const template = buildTemplate()
  const json = template.toJSON()
  const policies = Object.values(json.Resources).filter(
    (r: any) => r.Type === 'AWS::IAM::Policy'
  ) as any[]
  const transactionStatements = policies.flatMap(policy =>
    policy.Properties.PolicyDocument.Statement.filter((statement: any) =>
      (Array.isArray(statement.Action) ? statement.Action : [statement.Action])
        .includes('dynamodb:TransactWriteItems')
    )
  )

  expect(transactionStatements).toHaveLength(1)
  const resources = JSON.stringify(transactionStatements[0].Resource)
  expect(resources).toContain('dev_dfe_organization_persons')
  expect(resources).toContain('dev_dfe_audit_logs')
  expect(resources).not.toContain('/index/*')
})

// No CloudWatch alarms at all (2026-08-19): the last one, on the outbox-publisher
// DLQ, went the way of the per-worker alarms — billed alarm-months with nobody
// subscribed to receive them. The ops-alerts topic is kept for out-of-band
// subscriptions; DLQ depth is checked from the console or a redrive runbook.
test('the worker stack creates no CloudWatch alarms', () => {
  const template = buildTemplate()

  template.resourceCountIs('AWS::CloudWatch::Alarm', 0)
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

test('every worker Lambda that invokes the DFE Lambda knows the egress region', () => {
  const fns: any[] = Object.values(buildTemplate().findResources('AWS::Lambda::Function'))
  const withDfe = fns.filter(f => f.Properties.Environment?.Variables?.DFE_LAMBDA_NAME)
  expect(withDfe.length).toBeGreaterThan(0)
  for (const f of withDfe) {
    expect(f.Properties.Environment.Variables.DFE_EGRESS_REGION).toBe('sa-east-1')
  }
})

test('worker roles may invoke the egress Lambda in sa-east-1', () => {
  const policies = Object.values(buildTemplate().findResources('AWS::IAM::Policy')) as any[]
  const egressArns = policies.flatMap(p => p.Properties.PolicyDocument.Statement
    .filter((st: any) => ([] as any[]).concat(st.Action).includes('lambda:InvokeFunction'))
    .flatMap((st: any) => ([] as any[]).concat(st.Resource)))
    .map((arn: any) => JSON.stringify(arn))
    .filter((arn: string) => arn.includes('go-dfe-egress'))
  expect(egressArns.length).toBeGreaterThan(0)
  for (const arn of egressArns) expect(arn).toContain('arn:aws:lambda:sa-east-1:')
})

test('every worker that calls SEFAZ outlives the egress Lambda', () => {
  for (const w of WORKERS.filter(w => w.sefazServices.length > 0)) {
    expect(w.timeoutSeconds).toBeGreaterThan(EGRESS_TIMEOUT_SECONDS)
  }
})
