import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import {Construct} from 'constructs';
import {Environment} from './types';

interface IAMStackProps extends cdk.StackProps {
  environment: Environment;

  // ARNs em vez de constructs (remove acoplamento)
  certificatesBucketArn: string;
  documentsBucketArn: string;
  deploymentsBucketArn: string;
  logsBucketArn: string;
  topicArn: string;

  tablePrefix: string;

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
     *
     * Prefix wildcard instead of one ARN per table: the explicit list grew past
     * the 6144-byte managed policy limit as tables were added. Every table in
     * this app is named `${tablePrefix}_*`, so the grant stays scoped to the
     * environment.
     */
    const tableArnPrefix = `arn:aws:dynamodb:${this.region}:${this.account}:table/${props.tablePrefix}_`;
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
            'dynamodb:TransactWriteItems',
          ],
          resources: [`${tableArnPrefix}*`, `${tableArnPrefix}*/index/*`],
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

    // Name of the bucket `cdk bootstrap` created for this account and region.
    // The constant beats a literal, but it is still the *default* qualifier: this
    // app does not set `@aws-cdk/core:bootstrapQualifier` in cdk.json, and if it
    // ever does, this line has to follow — the asset download would 403 otherwise.
    const cdkAssetsBucket = `cdk-${cdk.DefaultStackSynthesizer.DEFAULT_QUALIFIER}-assets-${this.account}-${this.region}`;

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
        // The CDK assets bucket, read once at boot: ApiStack ships nginx.conf, the
        // systemd unit and the operational scripts as an s3-assets Asset instead of
        // inlining them in user data, which EC2 caps at 16 KB.
        //
        // Granted here rather than with `asset.grantRead()` because the instance
        // profile is passed to ApiStack as a name, not as a Role — and a name is
        // not something `grantRead` can attach a policy to. The bucket name is
        // deterministic from the bootstrap qualifier, so this is not a guess.
        new iam.PolicyStatement({
          actions: ['s3:GetObject'],
          resources: [`arn:aws:s3:::${cdkAssetsBucket}/*`],
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
            `arn:aws:ssm:*:*:parameter/ctech-billing/${environment}/*`,
            `arn:aws:ssm:*:*:parameter/ctech/${environment}/*`,
          ],
        }),
        // SecureString values are KMS-encrypted, so reading one needs Decrypt as
        // well as GetParameter. Scoped to SSM's own key by a condition rather
        // than granted against every key in the account: this role decrypts its
        // configuration, not anybody else's data.
        new iam.PolicyStatement({
          actions: ['kms:Decrypt'],
          resources: ['arn:aws:kms:*:*:key/*'],
          conditions: {
            StringEquals: {'kms:ViaService': `ssm.${this.region}.amazonaws.com`},
          },
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

    // EC2 — update-realip.sh reads the AWS-managed CloudFront origin-facing
    // prefix list. Both actions are read-only and do not support resource-level
    // permissions, so Resource must be *.
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['ec2:DescribeManagedPrefixLists', 'ec2:GetManagedPrefixListEntries'],
      resources: ['*'],
    }));

    // The shared bootstrap scripts published by ctech-cdk's Ec2ScriptsStack.
    // Scoped to that bucket; the instance downloads them on every boot. Both
    // buckets granted unconditionally: which one userData actually pulls from
    // is an instance-launch-time decision (osFamily), not a deploy-time IAM
    // decision, and granting both costs nothing.
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      sid: 'ReadSharedEc2BootstrapScripts',
      actions: ['s3:GetObject'],
      resources: [
        `arn:aws:s3:::${environment}-ctech-ec2-scripts/*`,
        `arn:aws:s3:::${environment}-ctech-ec2-scripts-alpine/*`,
      ],
    }));
  }
}