import * as cdk from 'aws-cdk-lib'
import {Duration} from 'aws-cdk-lib'
import * as sns from 'aws-cdk-lib/aws-sns'
import * as sqs from 'aws-cdk-lib/aws-sqs'
import * as destinations from 'aws-cdk-lib/aws-lambda-destinations'
import * as lambda from 'aws-cdk-lib/aws-lambda'
import * as subs from 'aws-cdk-lib/aws-sns-subscriptions'
import * as scheduler from 'aws-cdk-lib/aws-scheduler'
import * as schedulerTargets from 'aws-cdk-lib/aws-scheduler-targets'
import * as iam from 'aws-cdk-lib/aws-iam'
import * as ssm from 'aws-cdk-lib/aws-ssm'
import {Construct} from 'constructs'
import {CONTINGENCY_WORKER_ID, DISTRIBUTION_WORKER_ID, WorkerDefinition} from './worker-definitions'
import {Environment} from './types'
import path from 'node:path'
import {goLambdaCode} from './go-code'
import {egressFunctionArn} from './egress'

const CTECH_WORKER_DIR = path.join(__dirname, '../../worker')

// goCode builds a Go Lambda binary from the worker module (see go-code.ts).
function goCode(cmd: string): lambda.AssetCode {
  return goLambdaCode(CTECH_WORKER_DIR, `./cmd/${cmd}`)
}

// Lambda async invoke policy shared by every worker (SNS and direct invokes are both async).
// 2 retries = 3 tries, the old SQS maxReceiveCount. Lambda waits 1 min, then 2 min between
// tries. A job older than the max event age is dropped to the on-failure destination: one
// hour is far past three tries, and keeps a throttled backlog from failing documents late.
const ASYNC_RETRY_ATTEMPTS = 2
const ASYNC_MAX_EVENT_AGE = Duration.hours(1)

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
  dfeEgressRegion: string
  resultsTopicArn: string
}

export class WorkerStack extends cdk.Stack {
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
      dfeEgressRegion,
      resultsTopicArn,
      outboxTableName,
      outboxTableArn,
      outboxStreamArn,
    } = props

    const dfeLambdaArn = egressFunctionArn(environment, this.account)

    // Shared operator-alert topic (ctech-cdk AlertsStack). The dead-letter and outbox
    // Lambdas publish to it through go-common alerts; unset env = alerts.Nop.
    const alertsTopicArn = ssm.StringParameter.valueForStringParameter(this, `/ctech/${environment}/alerts/topic-arn`)

    // =========================
    // LOOP DE WORKERS
    // =========================
    // Kept without an alarm attached: the topic may carry email/chat
    // subscriptions added outside CloudFormation, and destroying it would drop
    // them. Publish to it from an alarm again if DLQ paging comes back.
    new sns.Topic(this, 'ops-alerts-topic', {
      topicName: `${environment}-dfe-ops-alerts`,
    })
    // Parking queue, NOT consumed by any event source mapping (an idle queue costs nothing;
    // a poller bills ~60k empty receives a week). It receives (a) SNS deliveries that failed
    // after SNS's own retries (subscription redrive) and (b) events the dlq-processor itself
    // could not handle. Drain runbook: docs/specs/2026-11-02-sns-lambda-worker-delivery.md.
    const workerDlq = new sqs.Queue(this, 'worker-dlq', {
      queueName: `${environment}-dfe-dlq`,
      retentionPeriod: Duration.days(14),
      receiveMessageWaitTime: Duration.seconds(20),
    })
    const workerDlqTableNames = [...new Set(workers.flatMap(worker => worker.dynamoTables ?? []))]

    // DLQ processor — the on-failure destination of every worker. After the last retry
    // Lambda invokes it with the failure envelope; it publishes status=failed to Results
    // SNS and alerts. Logical ids keep the first worker's id: the role has a fixed
    // physical name, so renaming them would make CloudFormation replace it and collide.
    const dlqOwnerId = workers[0].id
    const dlqRole = new iam.Role(this, `${dlqOwnerId}-dlq-role`, {
      roleName: `${environment}-dfe-dlq-processor-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    })

    dlqRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['sns:Publish'],
      resources: [resultsTopicArn, alertsTopicArn],
    }))

    if (workerDlqTableNames.length) {
      const dlqTableArns = workerDlqTableNames.map(t =>
        `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}`
      )
      dlqRole.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['dynamodb:UpdateItem'],
        resources: dlqTableArns,
      }))
    }

    const dlqProcessor = new lambda.Function(this, `${dlqOwnerId}-dlq-processor`, {
      functionName: `${environment}-dfe-dlq-processor`,
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goCode('dlq-processor'),
      role: dlqRole,
      architecture: lambda.Architecture.ARM_64,
      timeout: Duration.seconds(30),
      memorySize: 128,
      // It is itself an async target: a failed run is retried, then parked (not lost).
      retryAttempts: ASYNC_RETRY_ATTEMPTS,
      maxEventAge: ASYNC_MAX_EVENT_AGE,
      onFailure: new destinations.SqsDestination(workerDlq),
      environment: {
        APP_ENVIRONMENT: environment,
        TABLE_PREFIX: tablePrefix,
        CERTIFICATES_BUCKET: certificatesBucketName,
        DOCUMENTS_BUCKET: documentsBucketName,
        DFE_LAMBDA_NAME: dfeLambdaName,
        DFE_EGRESS_REGION: dfeEgressRegion,
        RESULTS_TOPIC_ARN: resultsTopicArn,
        ALERTS_TOPIC_ARN: alertsTopicArn,
      },
    })

    let distributionFn: lambda.Function | undefined
    let contingencyFn: lambda.Function | undefined
    for (const worker of workers) {
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

      // Drop jobs of companies in account deletion (erasure state, read only).
      role.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['dynamodb:GetItem'],
        resources: [`arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_erasure_state`],
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

      if (worker.dynamoTransactionTables?.length) {
        const transactionTableArns = worker.dynamoTransactionTables.map(t =>
          `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}`
        )
        role.addToPrincipalPolicy(new iam.PolicyStatement({
          actions: ['dynamodb:TransactWriteItems'],
          resources: transactionTableArns,
        }))
      }

      // Workers with their own message format use a dedicated binary.
      const workerCmd = worker.command ?? 'worker'

      const fn = new lambda.Function(this, `${worker.id}-lambda`, {
        functionName: `${environment}-${worker.name}`,
        runtime: lambda.Runtime.PROVIDED_AL2023,
        handler: 'bootstrap',
        code: goCode(workerCmd),
        role,
        architecture: lambda.Architecture.ARM_64,
        timeout: Duration.seconds(worker.timeoutSeconds ?? 300),
        memorySize: worker.memory ?? 128,
        retryAttempts: ASYNC_RETRY_ATTEMPTS,
        maxEventAge: ASYNC_MAX_EVENT_AGE,
        onFailure: new destinations.LambdaDestination(dlqProcessor),
        environment: {
          APP_ENVIRONMENT: environment,
          TABLE_PREFIX: tablePrefix,
          CERTIFICATES_BUCKET: certificatesBucketName,
          DOCUMENTS_BUCKET: documentsBucketName,
          DFE_LAMBDA_NAME: dfeLambdaName,
          DFE_EGRESS_REGION: dfeEgressRegion,
          RESULTS_TOPIC_ARN: resultsTopicArn,
          ...worker.environment,
        },
      })

      // Workers with no sefazServices are invoked directly (API, dispatcher), never from
      // the event bus. The others subscribe to it: SNS invokes the function asynchronously
      // (no polling, so no idle cost) with the same sefaz_service allowlist as before.
      // Raw message delivery does not exist for Lambda: the body arrives as Sns.Message.
      // deadLetterQueue is the subscription redrive for deliveries SNS itself gives up on.
      if (worker.sefazServices.length > 0) {
        eventBus.addSubscription(
          new subs.LambdaSubscription(fn, {
            deadLetterQueue: workerDlq,
            filterPolicyWithMessageBody: {
              sefaz_service: sns.FilterOrPolicy.filter(
                sns.SubscriptionFilter.stringFilter({allowlist: worker.sefazServices})
              ),
            },
          })
        )
      } else if (worker.id === DISTRIBUTION_WORKER_ID) {
        distributionFn = fn
      } else if (worker.id === CONTINGENCY_WORKER_ID) {
        contingencyFn = fn
      }

      // Keep-warm: direct invoke with {"ping":true} every minute, short-circuited
      // before event parsing (service.IsPingEvent) — no SEFAZ/AWS calls, avoids
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

    }

    // =========================
    // TRANSACTIONAL OUTBOX PUBLISHER
    // =========================
    const outboxDlq = new sqs.Queue(this, 'outbox-publisher-dlq', {
      queueName: `${environment}-dfe-outbox-publisher-dlq`,
      retentionPeriod: Duration.days(14),
      receiveMessageWaitTime: Duration.seconds(20),
    })
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
      resources: [eventBus.topicArn, alertsTopicArn],
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
        APP_ENVIRONMENT: environment,
        EVENT_BUS_TOPIC_ARN: eventBus.topicArn,
        OUTBOX_TABLE_NAME: outboxTableName,
        ALERTS_TOPIC_ARN: alertsTopicArn,
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
    // and invokes the distribution worker (async) once per (org, doc_type).
    const distributionWorker = distributionFn!

    const dispatcherRole = new iam.Role(this, 'distribution-dispatcher-role', {
      roleName: `${environment}-distribution-dispatcher-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    })

    // Async invoke of the distribution worker replaces the SQS send.
    distributionWorker.grantInvoke(dispatcherRole)

    // Dispatcher queries the sweep-index of the config tables to enumerate active orgs
    // (no Scan). The index ARNs are covered by the `/index/*` resource below.
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
        DISTRIBUTION_FUNCTION_NAME: distributionWorker.functionName,
      },
    })

    new scheduler.Schedule(this, 'DistributionV2Schedule', {
      scheduleName: `${environment}-distribution-schedule`,
      description: 'Triggers distribution dispatcher every 30 minutes to pull DFe documents from SEFAZ',
      schedule: scheduler.ScheduleExpression.rate(Duration.minutes(30)),
      target: new schedulerTargets.LambdaInvoke(dispatcher),
      enabled: true,
    })

    // =========================
    // CONTINGENCY RETRANSMISSION DISPATCHER
    // =========================
    // Every 15 min: Query the sparse contingency-index of the four document tables
    // (no Scan) and invoke the retransmission worker async once per pending document.
    // Same pattern as the distribution dispatcher; async invoke retries twice and
    // parks failures in the dlq-processor (the repo's replacement for an SQS queue
    // plus DLQ, which nothing long-polls). Cost: ~2,900 invocations/month, inside the
    // Lambda and Scheduler free tiers; the index is empty unless a contingency happened.
    const contingencyWorker = contingencyFn!
    const contingencyRole = new iam.Role(this, 'contingency-dispatcher-role', {
      roleName: `${environment}-contingency-dispatcher-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    })
    contingencyWorker.grantInvoke(contingencyRole)
    contingencyRole.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['dynamodb:Query'],
      resources: ['nfes', 'nfces', 'mdfes', 'ctes'].map(t =>
        `arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_${t}/index/contingency-index`),
    }))
    const contingencyDispatcher = new lambda.Function(this, 'contingency-dispatcher', {
      functionName: `${environment}-contingency-dispatcher`,
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goCode('contingency-dispatcher'),
      role: contingencyRole,
      architecture: lambda.Architecture.ARM_64,
      timeout: Duration.seconds(60),
      memorySize: 128,
      environment: {
        APP_ENVIRONMENT: environment,
        TABLE_PREFIX: tablePrefix,
        CONTINGENCY_FUNCTION_NAME: contingencyWorker.functionName,
      },
    })
    new scheduler.Schedule(this, 'ContingencySchedule', {
      scheduleName: `${environment}-contingency-schedule`,
      description: 'Triggers the contingency dispatcher every 15 minutes to retransmit EPEC/offline documents',
      schedule: scheduler.ScheduleExpression.rate(Duration.minutes(15)),
      target: new schedulerTargets.LambdaInvoke(contingencyDispatcher),
      enabled: true,
    })
  }
}
