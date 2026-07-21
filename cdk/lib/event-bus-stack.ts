import * as cdk from 'aws-cdk-lib'
import * as kms from 'aws-cdk-lib/aws-kms'
import * as sns from 'aws-cdk-lib/aws-sns'
import * as sqs from 'aws-cdk-lib/aws-sqs'
import * as subs from 'aws-cdk-lib/aws-sns-subscriptions'
import * as iam from 'aws-cdk-lib/aws-iam'
import * as cloudwatch from 'aws-cdk-lib/aws-cloudwatch'
import * as cwActions from 'aws-cdk-lib/aws-cloudwatch-actions'
import {Construct} from 'constructs'
import {Environment} from './types'

interface EventBusStackProps extends cdk.StackProps {
  environment: Environment
}

export class EventBusStack extends cdk.Stack {
  public readonly topic: sns.Topic
  public readonly resultsTopic: sns.Topic
  // CMK protecting certificate PFX passwords (B4): app-level encryption of the
  // password attribute (api encrypts, api/workers decrypt) and SSE for the two
  // SNS buses that carry it.
  public readonly certPasswordKey: kms.Key
  public readonly resultsQueueUrl: string
  public readonly resultsQueueArn: string

  constructor(scope: Construct, id: string, props: EventBusStackProps) {
    super(scope, id, props)

    const {environment} = props

    this.certPasswordKey = new kms.Key(this, 'CertPasswordKey', {
      alias: `${environment}-ctech-dfe-cert-password`,
      description: `CTech DFe certificate PFX password encryption (B4) - ${environment}`,
      enableKeyRotation: true,
      // Losing this key makes every stored certificate password unreadable
      // (certificates would need re-upload) — never delete with the stack.
      removalPolicy: environment === 'prod' ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
    })

    // Commands: API → Workers
    this.topic = new sns.Topic(this, 'DfeEventBus', {
      topicName: `${environment}-ctech-dfe`,
      displayName: `CTech DF-e Event Bus - ${environment}`,
      masterKey: this.certPasswordKey,
    })

    // Results: Workers → API
    this.resultsTopic = new sns.Topic(this, 'DfeResultsBus', {
      topicName: `${environment}-ctech-dfe-results`,
      displayName: `CTech DF-e Results Bus - ${environment}`,
      masterKey: this.certPasswordKey,
    })

    const opsAlertsTopic = new sns.Topic(this, 'results-ops-alerts-topic', {
      topicName: `${environment}-ctech-dfe-results-ops-alerts`,
    })

    const resultsDlq = new sqs.Queue(this, 'ResultsQueue-dlq', {
      queueName: `${environment}-ctech-dfe-results-dlq`,
      retentionPeriod: cdk.Duration.days(14),
      encryption: sqs.QueueEncryption.SQS_MANAGED,
    })

    new cloudwatch.Alarm(this, 'ResultsQueue-dlq-alarm', {
      alarmName: `${environment}-ctech-dfe-results-dlq-alarm`,
      alarmDescription: 'One or more messages landed in the results DLQ — a worker→api WebSocket notification failed after all retries.',
      metric: resultsDlq.metricApproximateNumberOfMessagesVisible({period: cdk.Duration.minutes(1)}),
      threshold: 1,
      evaluationPeriods: 1,
      comparisonOperator: cloudwatch.ComparisonOperator.GREATER_THAN_OR_EQUAL_TO_THRESHOLD,
      treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING,
    }).addAlarmAction(new cwActions.SnsAction(opsAlertsTopic))

    const resultsQueue = new sqs.Queue(this, 'ResultsQueue', {
      queueName: `${environment}-ctech-dfe-results`,
      visibilityTimeout: cdk.Duration.seconds(30),
      retentionPeriod: cdk.Duration.hours(1),
      encryption: sqs.QueueEncryption.SQS_MANAGED,
      deadLetterQueue: {
        queue: resultsDlq,
        maxReceiveCount: 3,
      },
    })

    this.resultsTopic.addSubscription(
      new subs.SqsSubscription(resultsQueue, {rawMessageDelivery: true})
    )

    resultsQueue.addToResourcePolicy(new iam.PolicyStatement({
      effect: iam.Effect.ALLOW,
      principals: [new iam.ServicePrincipal('sns.amazonaws.com')],
      actions: ['sqs:SendMessage'],
      resources: [resultsQueue.queueArn],
      conditions: {ArnEquals: {'aws:SourceArn': this.resultsTopic.topicArn}},
    }))

    this.resultsQueueUrl = resultsQueue.queueUrl
    this.resultsQueueArn = resultsQueue.queueArn

    new cdk.CfnOutput(this, 'EventBusArn', {value: this.topic.topicArn})
    new cdk.CfnOutput(this, 'ResultsTopicArn', {value: this.resultsTopic.topicArn})
    new cdk.CfnOutput(this, 'ResultsQueueUrl', {value: this.resultsQueueUrl})
  }
}