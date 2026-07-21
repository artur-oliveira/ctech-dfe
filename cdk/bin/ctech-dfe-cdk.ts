#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';

import {DynamoDBStack} from '../lib/dynamodb-stack';
import {S3Stack} from '../lib/s3-stack';
import {IAMStack} from '../lib/iam-stack';
import {FrontendStack} from '../lib/frontend-stack';
import {OidcStack} from '../lib/oidc-stack';
import {WorkerStack} from '../lib/worker-stack';
import {DfeStack} from '../lib/dfe-stack';
import {EventBusStack} from '../lib/event-bus-stack';
import {ApiStackV2} from '../lib/api-v2-stack';
import {WORKERS} from '../lib/worker-definitions';
import {Environment} from '../lib/types';

const app = new cdk.App();

// =====================
// Constants
// =====================
const AWS_ACCOUNT = '868899309401';
const AWS_REGION = 'us-east-1';
// Wildcard ACM cert — owned by ctech-cdk but referenced here for CloudFront.
const CERT_ARN =
  'arn:aws:acm:us-east-1:868899309401:certificate/29678869-bfc3-4688-b81b-55aa5b1d7443';

const ENVIRONMENT = (process.env.ENVIRONMENT || 'dev') as Environment;
const GITHUB_REPO = process.env.GITHUB_REPO || 'artur-oliveira/ctech-dfe';

// VPC is managed by ctech-cdk. The ID must be a concrete string (not a token)
// because ec2.Vpc.fromLookup resolves subnet/AZ metadata at synthesis time.
// The CI workflow reads /ctech/{env}/network/vpc-id from SSM and exports it.
const CTECH_VPC_ID = process.env.CTECH_VPC_ID || 'vpc-0adfd86727d17445b';
// Shared S3 buckets owned by ctech-cdk. CI reads these from SSM
// (/ctech/{env}/s3/deployments-bucket and /ctech/{env}/s3/logs-bucket)
// and sets them as env vars before running cdk deploy.
const CTECH_DEPLOYMENTS_BUCKET = process.env.CTECH_DEPLOYMENTS_BUCKET || `${ENVIRONMENT}-ctech-deployments`;
const CTECH_LOGS_BUCKET = process.env.CTECH_LOGS_BUCKET || `${ENVIRONMENT}-ctech-application-logs`;

const env = {account: AWS_ACCOUNT, region: AWS_REGION};

const BASE_DOMAIN = 'aoctech.app';

const domainForEnv = (environment: Environment, prefix: string) => {
  switch (environment) {
    case 'prod':
      return `${prefix}.${BASE_DOMAIN}`;
    case 'dev':
    case 'stage':
      return `${prefix}-${environment}.${BASE_DOMAIN}`;
  }
};

const id = (name: string) =>
  `CtechDfe-${ENVIRONMENT.charAt(0).toUpperCase() + ENVIRONMENT.slice(1)}-${name}`;

// =====================
// Global stack (OIDC GitHub Actions roles)
// =====================
new OidcStack(app, 'CtechDfe-Global-OIDC', {
  env,
  githubRepo: GITHUB_REPO,
  description: 'CTech DFe GitHub Actions OIDC provider and deployment roles (global)',
  deploymentsBucket: CTECH_DEPLOYMENTS_BUCKET
});

// =====================
// Base infrastructure (independent)
// =====================
const dynamodbStack = new DynamoDBStack(app, id('DynamoDB'), {
  env,
  environment: ENVIRONMENT,
  tablePrefix: `${ENVIRONMENT}_dfe`,
  description: `CTech DFe DynamoDB - ${ENVIRONMENT}`,
});

// ctech-dfe-specific buckets (certificates + documents).
// Deployments and logs use the shared ctech-cdk buckets (CTECH_DEPLOYMENTS_BUCKET / CTECH_LOGS_BUCKET).
const s3Stack = new S3Stack(app, id('S3'), {
  env,
  environment: ENVIRONMENT,
  bucketPrefix: `${ENVIRONMENT}-ctech-dfe`,
  description: `CTech DFe S3 Buckets - ${ENVIRONMENT}`,
});

const eventBusStack = new EventBusStack(app, id('EventBus'), {
  env,
  environment: ENVIRONMENT,
  description: `CTech DFe Event Bus - ${ENVIRONMENT}`,
});

// =====================
// DFE Lambda Stack
// =====================
// DfeStack and WorkerStack each create their own Lambda layer inline.
// No cross-stack layer references → layer version updates deploy freely.
new DfeStack(app, id('Dfe'), {
  env,
  environment: ENVIRONMENT,
  description: `CTech DFe Lambda (PyDFe) (SEFAZ) - ${ENVIRONMENT}`,
});

// WorkerStack is created before IAMStack so that distributionQueueArn is
// available to grant sqs:SendMessage to the API v2 role.
const workerStack = new WorkerStack(app, id('Worker'), {
  env,
  environment: ENVIRONMENT,
  tablePrefix: `${ENVIRONMENT}_dfe`,
  eventBus: eventBusStack.topic,
  workers: WORKERS,
  certificatesBucketName: s3Stack.certificatesBucketName,
  documentsBucketName: s3Stack.documentsBucketName,
  dfeLambdaName: `${ENVIRONMENT}-py-dfe`,
  resultsTopicArn: eventBusStack.resultsTopic.topicArn,
  certPasswordKeyArn: eventBusStack.certPasswordKey.keyArn,
  description: `CTech DFe Worker (SNS + SQS + Lambda) - ${ENVIRONMENT}`,
});

const iamStack = new IAMStack(app, id('IAM'), {
  env,
  environment: ENVIRONMENT,
  certificatesBucketArn: s3Stack.certificatesBucketArn,
  documentsBucketArn: s3Stack.documentsBucketArn,
  deploymentsBucketArn: `arn:aws:s3:::${CTECH_DEPLOYMENTS_BUCKET}`,
  logsBucketArn: `arn:aws:s3:::${CTECH_LOGS_BUCKET}`,
  dynamoDBTables: dynamodbStack.tables,
  topicArn: eventBusStack.topic.topicArn,
  resultsQueueArn: eventBusStack.resultsQueueArn,
  distributionQueueArn: workerStack.distributionQueueArn,
  certPasswordKeyArn: eventBusStack.certPasswordKey.keyArn,
  description: `CTech DFe IAM Roles - ${ENVIRONMENT}`,
});
iamStack.addDependency(dynamodbStack);
iamStack.addDependency(s3Stack);
iamStack.addDependency(eventBusStack);
iamStack.addDependency(workerStack);

// =====================
// API (EC2 + ASG, shared ALB from ctech-cdk)
// =====================
const apiV2Stack = new ApiStackV2(app, id('API-V2'), {
  env,
  environment: ENVIRONMENT,
  vpcId: CTECH_VPC_ID,
  domainName: domainForEnv(ENVIRONMENT, 'dfe-api'),
  appDomainName: domainForEnv(ENVIRONMENT, 'dfe'),
  instanceProfileName: iamStack.instanceProfileNameV2,
  deploymentsBucketName: CTECH_DEPLOYMENTS_BUCKET,
  logsBucketName: CTECH_LOGS_BUCKET,
  certificatesBucketName: s3Stack.certificatesBucketName,
  documentsBucketName: s3Stack.documentsBucketName,
  resultsQueueUrl: eventBusStack.resultsQueueUrl,
  nfeEmissionTopicArn: eventBusStack.topic.topicArn,
  distributionQueueUrl: workerStack.distributionQueueUrl,
  // Shared Valkey instance owned by ctech-cdk — same SSM path convention.
  valkeyUrlSsmPath: `/ctech/${ENVIRONMENT}/valkey/url`,
  certPasswordKmsKeyId: `alias/${ENVIRONMENT}-ctech-dfe-cert-password`,
  description: `CTech DFe API (EC2 + ASG + ALB) - ${ENVIRONMENT}`,
});
// instanceProfileNameV2 is a plain string, not a CFN token — CDK cannot
// infer the dependency automatically. Force it so the instance profile
// exists before the ASG validates the launch template.
apiV2Stack.addDependency(iamStack);
// distributionQueueUrl is a CFN cross-stack reference — CDK infers this automatically,
// but we make it explicit for clarity.
apiV2Stack.addDependency(workerStack);

// =====================
// Frontend (S3 + CloudFront)
// =====================
new FrontendStack(app, id('Frontend'), {
  env,
  environment: ENVIRONMENT,
  certificateArn: CERT_ARN,
  domainName: domainForEnv(ENVIRONMENT, 'dfe'),
  apiDomainName: domainForEnv(ENVIRONMENT, 'dfe-api'),
  authDomainName: domainForEnv(ENVIRONMENT, 'accounts'),
  description: `CTech DFe Frontend (S3 + CloudFront) - ${ENVIRONMENT}`,
});
