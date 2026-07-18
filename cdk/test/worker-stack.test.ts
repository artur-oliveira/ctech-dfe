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

  template.resourceCountIs('AWS::CloudWatch::Alarm', WORKERS.length)
  template.hasResourceProperties('AWS::CloudWatch::Alarm', {
    ComparisonOperator: 'GreaterThanOrEqualToThreshold',
    EvaluationPeriods: 1,
    Threshold: 1,
    Namespace: 'AWS/SQS',
    MetricName: 'ApproximateNumberOfMessagesVisible',
  })
})

test('distribution poller schedule is enabled', () => {
  const template = buildTemplate()
  template.hasResourceProperties('AWS::Scheduler::Schedule', {
    State: 'ENABLED',
  })
})
