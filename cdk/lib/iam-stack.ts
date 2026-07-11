import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import {Construct} from 'constructs';
import {Environment} from './types';
import {aws_dynamodb} from "aws-cdk-lib";

interface IAMStackProps extends cdk.StackProps {
  environment: Environment;
  
  // ARNs em vez de constructs (remove acoplamento)
  certificatesBucketArn: string;
  documentsBucketArn: string;
  deploymentsBucketArn: string;
  logsBucketArn: string;
  topicArn: string;
  
  dynamoDBTables: Map<string, aws_dynamodb.TableV2>;
  
  resultsQueueArn: string;
  distributionQueueArn: string;
}

export class IAMStack extends cdk.Stack {
  public readonly lambdaRole: iam.Role;
  public readonly apiV2Role: iam.Role;
  public readonly instanceProfileNameV2: string;
  
  constructor(scope: Construct, id: string, props: IAMStackProps) {
    super(scope, id, props);
    
    const {environment} = props;
    
    /**
     * =========================
     * Lambda Role
     * =========================
     */
    this.lambdaRole = new iam.Role(this, 'LambdaExecutionRole', {
      roleName: `${environment}-py-dfe-lambda-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName(
          'service-role/AWSLambdaBasicExecutionRole',
        ),
      ],
    });
    
    /**
     * =========================
     * API Role (EC2 / EB)
     * =========================
     */
    
    this.apiV2Role = new iam.Role(this, 'APIV2ExecutionRole', {
      roleName: `${environment}-ctech-dfe-api-v2-role`,
      assumedBy: new iam.ServicePrincipal('ec2.amazonaws.com'),
      managedPolicies: [
        iam.ManagedPolicy.fromAwsManagedPolicyName('AmazonSSMManagedInstanceCore'),
        iam.ManagedPolicy.fromAwsManagedPolicyName('CloudWatchAgentServerPolicy'),
      ],
    });
    
    /**
     * =========================
     * Instance Profile
     * =========================
     */
    this.instanceProfileNameV2 = `${environment}-ctech-dfe-api-v2-instance-profile`;
    
    new iam.CfnInstanceProfile(this, 'ApiV2InstanceProfile', {
      instanceProfileName: this.instanceProfileNameV2,
      roles: [this.apiV2Role.roleName],
    });
    
    /**
     * =========================
     * DynamoDB Permissions
     * =========================
     * (ARN-based to sem dependência de Table construct)
     */
    const dynamoPolicy = new iam.ManagedPolicy(this, 'DynamoPolicy', {
      managedPolicyName: `${environment}-ctech-dfe-dynamodb-policy`,
      statements: [
        new iam.PolicyStatement({
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
          resources: [...props.dynamoDBTables.values()].flatMap(it => [
            it.tableArn, `${it.tableArn}/index/*`
          ]),
        }),
        // ListTables doesn't support resource-level restrictions
        new iam.PolicyStatement({
          actions: ['dynamodb:ListTables'],
          resources: ['*'],
        }),
      ],
    });
    
    this.lambdaRole.addManagedPolicy(dynamoPolicy);
    this.apiV2Role.addManagedPolicy(dynamoPolicy);
    
    /**
     * =========================
     * S3 Permissions
     * =========================
     */
    const s3Policy = new iam.ManagedPolicy(this, 'S3Policy', {
      managedPolicyName: `${environment}-ctech-dfe-s3-policy`,
      statements: [
        new iam.PolicyStatement({
          actions: ['s3:GetObject', 's3:PutObject', 's3:DeleteObject'],
          resources: [
            `${props.certificatesBucketArn}/*`,
            `${props.documentsBucketArn}/*`,
          ],
        }),
      ],
    });
    
    this.lambdaRole.addManagedPolicy(s3Policy);
    
    const apiS3Policy = new iam.ManagedPolicy(this, 'ApiS3Policy', {
      managedPolicyName: `${environment}-ctech-dfe-api-s3-policy`,
      statements: [
        new iam.PolicyStatement({
          actions: ['s3:GetObject', 's3:PutObject'],
          resources: [
            `${props.certificatesBucketArn}/*`,
            `${props.documentsBucketArn}/*`,
          ],
        }),
        // s3:ListBucket must be on the bucket ARN (not /*) — used by health check
        new iam.PolicyStatement({
          actions: ['s3:ListBucket'],
          resources: [
            props.certificatesBucketArn,
            props.documentsBucketArn,
          ],
        }),
        new iam.PolicyStatement({
          actions: ['s3:GetObject'],
          resources: [`${props.deploymentsBucketArn}/ctech-dfe/*`],
        }),
      ],
    });
    
    const apiSNSPolicy = new iam.ManagedPolicy(this, 'ApiSnsPolicy', {
      managedPolicyName: `${environment}-ctech-dfe-api-sns-policy`,
      statements: [
        new iam.PolicyStatement({
          actions: ['SNS:Publish', 'SNS:GetTopicAttributes'],
          resources: [props.topicArn],
        }),
        // ListTopics doesn't support resource-level restrictions
        new iam.PolicyStatement({
          actions: ['SNS:ListTopics'],
          resources: ['*'],
        }),
      ],
    });
    
    this.apiV2Role.addManagedPolicy(apiS3Policy);
    this.apiV2Role.addManagedPolicy(apiSNSPolicy);
    this.apiV2Role.addManagedPolicy(
      new iam.ManagedPolicy(this, 'ApiV2LogsPolicy', {
        managedPolicyName: `${environment}-ctech-dfe-api-v2-logs-policy`,
        statements: [
          new iam.PolicyStatement({
            actions: ['s3:PutObject'],
            resources: [`${props.logsBucketArn}/ctech-dfe/*`],
          }),
        ],
      }),
    );
    
    /**
     * =========================
     * SSM Permissions
     * =========================
     */
    const ssmPolicy = new iam.ManagedPolicy(this, 'SsmPolicy', {
      managedPolicyName: `${environment}-ctech-dfe-ssm-policy`,
      statements: [
        new iam.PolicyStatement({
          actions: ['ssm:GetParameter'],
          resources: [
            `arn:aws:ssm:*:*:parameter/ctech-dfe/${environment}/*`,
            `arn:aws:ssm:*:*:parameter/ctech-account/${environment}/*`,
            `arn:aws:ssm:*:*:parameter/ctech/${environment}/*`,
          ],
        }),
      ],
    });
    
    this.apiV2Role.addManagedPolicy(ssmPolicy);
    
    const lambdaInvokePolicy = new iam.ManagedPolicy(this, 'LambdaInvokePolicy', {
      managedPolicyName: `${environment}-py-dfe-lambda-invoke-policy`,
      statements: [
        new iam.PolicyStatement({
          actions: ['lambda:InvokeFunction'],
          resources: [
            `arn:aws:lambda:${this.region}:${this.account}:function:${environment}-py-dfe`,
            `arn:aws:lambda:${this.region}:${this.account}:function:${environment}-py-dfe:*`,
          ],
        }),
      ],
    });
    
    this.apiV2Role.addManagedPolicy(lambdaInvokePolicy);
    
    const apiSqsPolicy = new iam.PolicyStatement({
      actions: ['sqs:ReceiveMessage', 'sqs:DeleteMessage', 'sqs:GetQueueAttributes'],
      resources: [props.resultsQueueArn],
    });
    
    this.apiV2Role.addToPrincipalPolicy(apiSqsPolicy);
    
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['sqs:SendMessage', 'sqs:GetQueueAttributes'],
      resources: [props.distributionQueueArn],
    }));
  }
}