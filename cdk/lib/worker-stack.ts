import * as cdk from 'aws-cdk-lib'
import {Duration} from 'aws-cdk-lib'
import * as sns from 'aws-cdk-lib/aws-sns'
import * as sqs from 'aws-cdk-lib/aws-sqs'
import * as lambda from 'aws-cdk-lib/aws-lambda'
import * as subs from 'aws-cdk-lib/aws-sns-subscriptions'
import * as lambdaEvents from 'aws-cdk-lib/aws-lambda-event-sources'
import * as scheduler from 'aws-cdk-lib/aws-scheduler'
import * as schedulerTargets from 'aws-cdk-lib/aws-scheduler-targets'
import * as iam from 'aws-cdk-lib/aws-iam'
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch'
import * as cwActions from 'aws-cdk-lib/aws-cloudwatch-actions'
import {Construct} from 'constructs'
import {WorkerDefinition} from './worker-definitions'
import {Environment} from './types'
import path from 'node:path'
import {spawnSync} from 'child_process'

const CTECH_WORKER_DIR = path.join(__dirname, '../../worker')

// resolveGo returns the absolute path to the go binary.
// Checks PATH first, then falls back to ~/sdk/go*/bin/go (Google's default SDK dir).
function resolveGo(): string {
  const lookup = spawnSync('bash', ['-c',
    'which go 2>/dev/null || ls "${HOME}/sdk/go"*/bin/go 2>/dev/null | sort -rV | head -1',
  ], {stdio: 'pipe', env: process.env})
  if (lookup.status === 0 && lookup.stdout) {
    const found = lookup.stdout.toString().trim()
    if (found) return found
  }
  return 'go'
}

// goCode builds a Go Lambda binary from the worker module.
// Local bundling (no Docker) is attempted first; Docker is the fallback.
function goCode(cmd: string): lambda.AssetCode {
  return lambda.Code.fromAsset(CTECH_WORKER_DIR, {
    bundling: {
      local: {
        tryBundle(outputDir: string): boolean {
          const r = spawnSync(
            resolveGo(),
            ['build', '-tags', 'lambda.norpc', '-ldflags', '-s -w', '-o', path.join(outputDir, 'bootstrap'), `./cmd/${cmd}`],
            {
              cwd: CTECH_WORKER_DIR,
              env: {...process.env, GOOS: 'linux', GOARCH: 'arm64', CGO_ENABLED: '0'},
              stdio: ['ignore', 'pipe', 'pipe'],
            },
          )
          if (r.status !== 0) process.stderr.write(r.stderr ?? Buffer.alloc(0))
          return r.status === 0
        },
      },
      image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
      // GOCACHE/GOPATH must be writable; Docker runs as uid 1000:1000 which has no HOME.
      environment: {GOCACHE: '/tmp/go-build', GOPATH: '/tmp/go'},
      command: [
        'bash', '-c',
        `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o /asset-output/bootstrap ./cmd/${cmd}`,
      ],
    },
  })
}

interface WorkerStackProps extends cdk.StackProps {
  environment: Environment
  tablePrefix: string
	outboxTableName: string
	outboxTableArn: string
	outboxStreamArn: string
  eventBus: sns.Topic
  workers: WorkerDefinition[]
  certificatesBucketName: string
  documentsBucketName: string
  dfeLambdaName: string
  resultsTopicArn: string
}

export class WorkerStack extends cdk.Stack {
  public readonly distributionQueueUrl: string
  public readonly distributionQueueArn: string

  constructor(scope: Construct, id: string, props: WorkerStackProps) {
    super(scope, id, props)

    const {
      environment,
      tablePrefix,
      eventBus,
      workers,
      certificatesBucketName,
      documentsBucketName,
      dfeLambdaName,
      resultsTopicArn,
	  outboxTableName,
	  outboxTableArn,
	  outboxStreamArn,
    } = props

    const dfeLambdaArn = `arn:aws:lambda:${this.region}:${this.account}:function:${dfeLambdaName}`

    // =========================
    // LOOP DE WORKERS
    // =========================
    const queueById = new Map<string, sqs.Queue>()

    const opsAlertsTopic = new sns.Topic(this, 'ops-alerts-topic', {
      topicName: `${environment}-dfe-ops-alerts`,
    })

    for (const worker of workers) {

      const dlq = new sqs.Queue(this, `${worker.id}-dlq`, {
        queueName: `${environment}-${worker.queueName}-dlq`,
        retentionPeriod: Duration.days(14),
        // Long polling: without this the Lambda poller short-polls continuously,
        // burning SQS free-tier requests even on an idle queue.
        receiveMessageWaitTime: Duration.seconds(20),
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

      const queue = new sqs.Queue(this, `${worker.id}-queue`, {
        queueName: `${environment}-${worker.queueName}`,
        // AWS recommends six times the Lambda timeout, plus the batch window.
        visibilityTimeout: Duration.seconds((worker.timeoutSeconds ?? 300) * 6 + 300),
        receiveMessageWaitTime: Duration.seconds(20),
        deadLetterQueue: {
          queue: dlq,
          maxReceiveCount: 3,
        },
      })

      queueById.set(worker.id, queue)

      // Workers with no sefazServices send messages directly (API or scheduler).
      // Do not subscribe them to the SNS event bus.
      if (worker.sefazServices.length > 0) {
        eventBus.addSubscription(
          new subs.SqsSubscription(queue, {
            rawMessageDelivery: true,
            filterPolicyWithMessageBody: {
              sefaz_service: sns.FilterOrPolicy.filter(
                sns.SubscriptionFilter.stringFilter({allowlist: worker.sefazServices})
              ),
            },
          })
        )

        queue.addToResourcePolicy(new iam.PolicyStatement({
          effect: iam.Effect.ALLOW,
          principals: [new iam.ServicePrincipal('sns.amazonaws.com')],
          actions: ['sqs:SendMessage'],
          resources: [queue.queueArn],
          conditions: {ArnEquals: {'aws:SourceArn': eventBus.topicArn}},
        }))
      }

      const role = new iam.Role(this, `${worker.id}-role`, {
        roleName: `${environment}-${worker.id}-worker-role`,
        assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
        managedPolicies: [
          iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
        ],
      })

      role.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['s3:GetObject', 's3:PutObject'],
        resources: [
          `arn:aws:s3:::${certificatesBucketName}/*`,
          `arn:aws:s3:::${documentsBucketName}/*`,
        ],
      }))

      role.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['lambda:InvokeFunction'],
        resources: [dfeLambdaArn],
      }))

      role.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['sns:Publish'],
        resources: [resultsTopicArn],
      }))

      if (worker.dynamoTables?.length) {
        const tableArns = worker.dynamoTables.flatMap(t => [
          `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}`,
          `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}/index/*`,
        ])
        const dynamoActions = ['dynamodb:GetItem', 'dynamodb:PutItem', 'dynamodb:UpdateItem', 'dynamodb:Query']
        role.addToPrincipalPolicy(new iam.PolicyStatement({
          actions: dynamoActions,
          resources: tableArns,
        }))
      }

      // Distribution worker uses a dedicated binary with its own SQS message format.
      const workerCmd = worker.id === 'distribution' ? 'distribution-worker' : 'worker'

      const fn = new lambda.Function(this, `${worker.id}-lambda`, {
        functionName: `${environment}-${worker.name}`,
        runtime: lambda.Runtime.PROVIDED_AL2023,
        handler: 'bootstrap',
        code: goCode(workerCmd),
        role,
        architecture: lambda.Architecture.ARM_64,
        timeout: Duration.seconds(worker.timeoutSeconds ?? 300),
        memorySize: worker.memory ?? 128,
        environment: {
          APP_ENVIRONMENT: environment,
          TABLE_PREFIX: tablePrefix,
          CERTIFICATES_BUCKET: certificatesBucketName,
          DOCUMENTS_BUCKET: documentsBucketName,
          DFE_LAMBDA_NAME: dfeLambdaName,
          RESULTS_TOPIC_ARN: resultsTopicArn,
          ...worker.environment,
        },
      })

      fn.addEventSource(
        new lambdaEvents.SqsEventSource(queue, {
          batchSize: 1,
          maxBatchingWindow: Duration.minutes(5),
          reportBatchItemFailures: true
        })
      )

      // Keep-warm: direct invoke with {"ping":true} every 5 min, short-circuited
      // before SQS parsing (service.IsPingEvent) — no SEFAZ/AWS calls, avoids
      // cold start on real traffic once this worker starts calling go-dfe in-process.
      new scheduler.Schedule(this, `${worker.id}-ping-schedule`, {
        scheduleName: `${environment}-${worker.name}-ping-schedule`,
        description: `Keeps ${worker.name} warm — direct invoke, no SEFAZ call`,
        schedule: scheduler.ScheduleExpression.rate(Duration.minutes(1)),
        target: new schedulerTargets.LambdaInvoke(fn, {
          input: scheduler.ScheduleTargetInput.fromObject({ping: true}),
        }),
        enabled: true,
      })

      // Distribution worker needs to publish Ciência da Operação to the event bus
      // so the nfe-event worker processes it asynchronously.
      if (worker.sefazServices.length === 0) {
        role.addToPrincipalPolicy(new iam.PolicyStatement({
          actions: ['sns:Publish'],
          resources: [eventBus.topicArn],
        }))
        fn.addEnvironment('EVENT_BUS_TOPIC_ARN', eventBus.topicArn)
      }

      // DLQ processor — publishes status=failed to Results SNS after 3 failed attempts.
      const dlqRole = new iam.Role(this, `${worker.id}-dlq-role`, {
        roleName: `${environment}-${worker.id}-dlq-processor-role`,
        assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
        managedPolicies: [
          iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
        ],
      })

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

      const dlqProcessor = new lambda.Function(this, `${worker.id}-dlq-processor`, {
        functionName: `${environment}-${worker.name}-dlq-processor`,
        runtime: lambda.Runtime.PROVIDED_AL2023,
        handler: 'bootstrap',
        code: goCode('dlq-processor'),
        role: dlqRole,
        architecture: lambda.Architecture.ARM_64,
        timeout: Duration.seconds(30),
        memorySize: 128,
        environment: {
          APP_ENVIRONMENT: environment,
          TABLE_PREFIX: tablePrefix,
          CERTIFICATES_BUCKET: certificatesBucketName,
          DOCUMENTS_BUCKET: documentsBucketName,
          DFE_LAMBDA_NAME: dfeLambdaName,
          RESULTS_TOPIC_ARN: resultsTopicArn,
        },
      })

      // reportBatchItemFailures: false — the handler must not re-raise; all messages consumed.
      dlqProcessor.addEventSource(
        new lambdaEvents.SqsEventSource(dlq, {
          batchSize: 10,
          maxBatchingWindow: Duration.seconds(30),
          reportBatchItemFailures: false
        })
      )
    }

    // =========================
    // TRANSACTIONAL OUTBOX PUBLISHER
    // =========================
    const outboxDlq = new sqs.Queue(this, 'outbox-publisher-dlq', {
      queueName: `${environment}-dfe-outbox-publisher-dlq`,
      retentionPeriod: Duration.days(14),
      receiveMessageWaitTime: Duration.seconds(20),
    })
    new cloudwatch.Alarm(this, 'outbox-publisher-dlq-alarm', {
      alarmName: `${environment}-dfe-outbox-publisher-dlq-alarm`,
      alarmDescription: 'A durable DFe command could not be published from the transactional outbox.',
      metric: outboxDlq.metricApproximateNumberOfMessagesVisible({period: Duration.minutes(1)}),
      threshold: 1,
      evaluationPeriods: 1,
      comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
      treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
    }).addAlarmAction(new cwActions.SnsAction(opsAlertsTopic))

    const outboxRole = new iam.Role(this, 'outbox-publisher-role', {
      roleName: `${environment}-dfe-outbox-publisher-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    })
    outboxRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['dynamodb:GetItem', 'dynamodb:UpdateItem'],
      resources: [outboxTableArn],
    }))
    outboxRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['dynamodb:DescribeStream', 'dynamodb:GetRecords', 'dynamodb:GetShardIterator', 'dynamodb:ListStreams'],
      resources: [outboxStreamArn],
    }))
    outboxRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['sns:Publish'],
      resources: [eventBus.topicArn],
    }))
    outboxDlq.grantSendMessages(outboxRole)

    const outboxPublisher = new lambda.Function(this, 'outbox-publisher-lambda', {
      functionName: `${environment}-dfe-outbox-publisher`,
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goCode('outbox-publisher'),
      role: outboxRole,
      architecture: lambda.Architecture.ARM_64,
      timeout: Duration.seconds(30),
      memorySize: 128,
      environment: {
        EVENT_BUS_TOPIC_ARN: eventBus.topicArn,
        OUTBOX_TABLE_NAME: outboxTableName,
      },
    })
    outboxDlq.addToResourcePolicy(new iam.PolicyStatement({
      principals: [new iam.ServicePrincipal('lambda.amazonaws.com')],
      actions: ['sqs:SendMessage'],
      resources: [outboxDlq.queueArn],
      conditions: {ArnEquals: {'aws:SourceArn': outboxPublisher.functionArn}},
    }))
    new lambda.CfnEventSourceMapping(this, 'outbox-stream-mapping', {
      functionName: outboxPublisher.functionName,
      eventSourceArn: outboxStreamArn,
      startingPosition: 'TRIM_HORIZON',
      batchSize: 10,
      bisectBatchOnFunctionError: true,
      maximumRecordAgeInSeconds: 82800,
      maximumRetryAttempts: -1,
      destinationConfig: {onFailure: {destination: outboxDlq.queueArn}},
    })

    // =========================
    // DISTRIBUTION DISPATCHER
    // =========================
    // A lightweight Lambda triggered every 30 min by EventBridge.
    // It scans the NF-e/CT-e/MDF-e config tables to find all active orgs
    // and enqueues one distribution job per (org, doc_type) into the distribution queue.
    const distributionQueue = queueById.get('distribution')!
    this.distributionQueueUrl = distributionQueue.queueUrl
    this.distributionQueueArn = distributionQueue.queueArn

    const dispatcherRole = new iam.Role(this, 'distribution-dispatcher-role', {
      roleName: `${environment}-distribution-dispatcher-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    })

    dispatcherRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['sqs:SendMessage'],
      resources: [distributionQueue.queueArn],
    }))

    // Dispatcher scans the config tables to enumerate active orgs.
    // These tables are small control-plane tables (one item per org), not transaction tables.
    const configTableArns = ['organization_nfe_configs', 'organization_cte_configs', 'organization_mdfe_configs', 'organization_nfse_configs'].flatMap(t => [
      `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}`,
      `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}/index/*`,
    ])

    dispatcherRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: [
        'dynamodb:GetItem',
        'dynamodb:PutItem',
        'dynamodb:UpdateItem',
        'dynamodb:DeleteItem',
        'dynamodb:Query',
        'dynamodb:Scan',
        'dynamodb:BatchGetItem',
        'dynamodb:BatchWriteItem',
      ],
      resources: configTableArns,
    }))

    const dispatcher = new lambda.Function(this, 'distribution-dispatcher', {
      functionName: `${environment}-distribution-dispatcher`,
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goCode('distribution-dispatcher'),
      role: dispatcherRole,
      architecture: lambda.Architecture.ARM_64,
      timeout: Duration.seconds(60),
      memorySize: 128,
      environment: {
        APP_ENVIRONMENT: environment,
        TABLE_PREFIX: tablePrefix,
        DISTRIBUTION_QUEUE_URL: distributionQueue.queueUrl,
      },
    })

    new scheduler.Schedule(this, 'DistributionV2Schedule', {
      scheduleName: `${environment}-distribution-schedule`,
      description: 'Triggers distribution dispatcher every 30 minutes to pull DFe documents from SEFAZ',
      schedule: scheduler.ScheduleExpression.rate(Duration.minutes(30)),
      target: new schedulerTargets.LambdaInvoke(dispatcher),
      enabled: true,
    })
  }
}
