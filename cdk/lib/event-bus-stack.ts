import * as cdk from 'aws-cdk-lib'
import * as sns from 'aws-cdk-lib/aws-sns'
import * as sqs from 'aws-cdk-lib/aws-sqs'
import * as subs from 'aws-cdk-lib/aws-sns-subscriptions'
import * as iam from 'aws-cdk-lib/aws-iam'
import {Construct} from 'constructs'
import {Environment} from './types'

interface EventBusStackProps extends cdk.StackProps {
  environment: Environment
}

export class EventBusStack extends cdk.Stack {
  public readonly topic: sns.Topic
  public readonly resultsTopic: sns.Topic
  public readonly resultsQueueUrl: string
  public readonly resultsQueueArn: string

  constructor(scope: Construct, id: string, props: EventBusStackProps) {
    super(scope, id, props)

    const {environment} = props

    // Commands: API → Workers
    this.topic = new sns.Topic(this, 'DfeEventBus', {
      topicName: `${environment}-ctech-dfe`,
      displayName: `CTech DF-e Event Bus - ${environment}`,
    })

    // Results: Workers → API
    this.resultsTopic = new sns.Topic(this, 'DfeResultsBus', {
      topicName: `${environment}-ctech-dfe-results`,
      displayName: `CTech DF-e Results Bus - ${environment}`,
    })

    const resultsQueue = new sqs.Queue(this, 'ResultsQueue', {
      queueName: `${environment}-ctech-dfe-results`,
      visibilityTimeout: cdk.Duration.seconds(30),
      retentionPeriod: cdk.Duration.hours(1),
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