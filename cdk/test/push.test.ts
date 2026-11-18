import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'
import * as sns from 'aws-cdk-lib/aws-sns'
import { DynamoDBStack } from '../lib/dynamodb-stack'
import { IAMStack } from '../lib/iam-stack'
import { WorkerStack } from '../lib/worker-stack'
import { pushParameterPath, pushSenderFunctionName, WORKERS } from '../lib/worker-definitions'

// Web Push (docs/specs/2026-11-17-web-push.md): the `push` table, the push-sender Lambda and the API's access to it.

const dynamoTemplate = () => {
  const app = new cdk.App()
  return Template.fromStack(new DynamoDBStack(app, 'TestDynamoDBStack', { tablePrefix: 'dev_dfe', environment: 'dev' }))
}

function workerTemplate(): Template {
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

function iamTemplate(): Template {
  const app = new cdk.App()
  const stack = new IAMStack(app, 'TestIamStack', {
    env: { account: '868899309401', region: 'us-east-1' },
    environment: 'prod',
    certificatesBucketArn: 'arn:aws:s3:::prod-ctech-dfe-certificates',
    documentsBucketArn: 'arn:aws:s3:::prod-ctech-dfe-documents',
    deploymentsBucketArn: 'arn:aws:s3:::prod-ctech-deployments',
    logsBucketArn: 'arn:aws:s3:::prod-ctech-application-logs',
    tablePrefix: 'prod_dfe',
    topicArn: 'arn:aws:sns:us-east-1:868899309401:prod-dfe-emission',
    resultsQueueArn: 'arn:aws:sqs:us-east-1:868899309401:prod-dfe-results',
    distributionFunctionName: 'prod-distribution-worker',
    erasureQueueArn: 'arn:aws:sqs:us-east-1:868899309401:prod-ctech-dfe-erasure',
  })
  return Template.fromStack(stack)
}

const actionsOf = (st: any) => (Array.isArray(st.Action) ? st.Action : [st.Action])

describe('push table', () => {
  const push = () => {
    const res = Object.values(dynamoTemplate().findResources('AWS::DynamoDB::GlobalTable')) as any[]
    return res.find((r) => r.Properties.TableName === 'dev_dfe_push')
  }

  test('has pk/sk and a TTL on ttl', () => {
    const t = push()
    expect(t).toBeDefined()
    expect(t.Properties.KeySchema).toEqual([
      { AttributeName: 'pk', KeyType: 'HASH' },
      { AttributeName: 'sk', KeyType: 'RANGE' },
    ])
    expect(t.Properties.TimeToLiveSpecification).toEqual({ AttributeName: 'ttl', Enabled: true })
  })

  test('user-index is sparse and KEYS_ONLY: only preference rows, so a person is erased without a Scan', () => {
    expect(push().Properties.GlobalSecondaryIndexes).toEqual(expect.arrayContaining([
      expect.objectContaining({
        IndexName: 'user-index',
        KeySchema: [
          { AttributeName: 'user_sub', KeyType: 'HASH' },
          { AttributeName: 'pk', KeyType: 'RANGE' },
        ],
        Projection: { ProjectionType: 'KEYS_ONLY' },
      }),
    ]))
  })

  test('endpoint-index is sparse and KEYS_ONLY: only subscription rows, so one browser endpoint is found across people without a Scan', () => {
    expect(push().Properties.GlobalSecondaryIndexes).toEqual(expect.arrayContaining([
      expect.objectContaining({
        IndexName: 'endpoint-index',
        KeySchema: [
          { AttributeName: 'endpoint_id', KeyType: 'HASH' },
          { AttributeName: 'pk', KeyType: 'RANGE' },
        ],
        Projection: { ProjectionType: 'KEYS_ONLY' },
      }),
    ]))
  })

  test('uses the AWS-owned key (no billed KMS)', () => {
    expect(push().Properties.Replicas[0].SSESpecification).toBeUndefined()
  })
})

describe('push-sender', () => {
  test('is invoked by the API only: no schedule, no SNS subscription, five at a time, two async retries', () => {
    const template = workerTemplate()
    template.hasResourceProperties('AWS::Lambda::Function', {
      FunctionName: 'dev-push-sender',
      Runtime: 'provided.al2023',
      Architectures: ['arm64'],
      ReservedConcurrentExecutions: 5,
      Timeout: 30,
      Environment: {
        Variables: Match.objectLike({
          TABLE_PREFIX: 'dev_dfe',
          PUSH_VAPID_PRIVATE_KEY_SSM_PATH: '/ctech-dfe/dev/push/vapid-private-key',
          PUSH_VAPID_PRIVATE_KEY_PREVIOUS_SSM_PATH: '/ctech-dfe/dev/push/vapid-private-key-previous',
          PUSH_VAPID_PUBLIC_KEY_SSM_PATH: '/ctech-dfe/dev/push/vapid-public-key',
          PUSH_SUBJECT_SSM_PATH: '/ctech-dfe/dev/app-url',
        }),
      },
    })
    template.hasResourceProperties('AWS::Lambda::EventInvokeConfig', {
      MaximumRetryAttempts: 2,
      MaximumEventAgeInSeconds: 3600,
    })
    expect(JSON.stringify(template.findResources('AWS::Scheduler::Schedule'))).not.toContain('push-sender')
    expect(JSON.stringify(template.findResources('AWS::SNS::Subscription'))).not.toContain('push-sender')
  })

  test('its role reads and deletes only the push table, reads only the VAPID and app-url parameters, and alerts', () => {
    const template = workerTemplate()
    const roles = Object.values(template.findResources('AWS::IAM::Role')) as any[]
    expect(roles.find((r) => r.Properties.RoleName === 'dev-push-sender-role')).toBeDefined()
    const policies = Object.values(template.findResources('AWS::IAM::Policy')) as any[]
    const mine = policies.filter((p) => JSON.stringify(p.Properties.Roles ?? []).includes('pushsenderrole'))
    expect(mine.length).toBeGreaterThan(0)
    const statements = mine.flatMap((p) => p.Properties.PolicyDocument.Statement)
    const text = JSON.stringify(statements)

    const dynamo = statements.find((s) => JSON.stringify(s.Action).includes('dynamodb:Query'))
    expect(JSON.stringify(dynamo.Resource)).toContain('dev_dfe_push')
    expect(JSON.stringify(dynamo.Resource)).not.toMatch(/dev_dfe_(nfes|organizations|users|webhooks)/)
    expect(JSON.stringify(dynamo.Action)).not.toContain('dynamodb:PutItem')
    expect(text).not.toContain('dynamodb:Scan')
    const ssmRead = statements.find((s) => JSON.stringify(s.Action).includes('ssm:GetParameter'))
    for (const name of ['vapid-private-key', 'vapid-private-key-previous', 'vapid-public-key']) {
      expect(JSON.stringify(ssmRead.Resource)).toContain(`/ctech-dfe/dev/push/${name}`)
    }
    expect(JSON.stringify(ssmRead.Resource)).toContain('/ctech-dfe/dev/app-url')
    expect(JSON.stringify(ssmRead.Resource)).not.toContain('*')
    expect(text).toContain('kms:ViaService')
    expect(text).toContain('PARAMETER_ARN')
    expect(text).toContain('sns:Publish')
  })
})

describe('API role and Web Push', () => {
  const statements = () => {
    const policies = Object.values(iamTemplate().findResources('AWS::IAM::Policy')) as any[]
    return policies
      .filter((p) => JSON.stringify(p.Properties.Roles ?? []).includes('APIV2ExecutionRole'))
      .flatMap((p) => p.Properties.PolicyDocument.Statement)
  }

  test('may invoke the push sender', () => {
    const invoke = statements().find((s) => actionsOf(s).includes('lambda:InvokeFunction') && JSON.stringify(s.Resource).includes(pushSenderFunctionName('prod')))
    expect(invoke).toBeDefined()
  })

  test('is explicitly denied the VAPID private keys, which only the sender may read', () => {
    const deny = statements().find((s) => s.Effect === 'Deny' && actionsOf(s).includes('ssm:GetParameter'))
    expect(deny).toBeDefined()
    expect(JSON.stringify(deny.Resource)).toContain(pushParameterPath('prod', 'vapid-private-key'))
    expect(JSON.stringify(deny.Resource)).toContain(pushParameterPath('prod', 'vapid-private-key-previous'))
    expect(JSON.stringify(deny.Resource)).not.toContain('vapid-public-key')
  })
})
