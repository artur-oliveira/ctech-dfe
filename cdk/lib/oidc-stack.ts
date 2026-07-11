import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import {Construct} from 'constructs';

interface OidcStackProps extends cdk.StackProps {
  // e.g. "myorg/ctech-dfe"
  githubRepo: string;
  deploymentsBucket: string;
}

/**
 * One-time stack (not per-environment).
 * Creates the GitHub Actions OIDC provider and the deployment roles
 * for each workflow (frontend, api, infra).
 */
export class OidcStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: OidcStackProps) {
    super(scope, id, props);
    
    const {githubRepo, deploymentsBucket} = props;
    
    // GitHub OIDC provider ownership transferred to ctech-cdk (Ctech-Global stack).
    // Import by well-known ARN — do not create here.
    const providerArn = `arn:aws:iam::${this.account}:oidc-provider/token.actions.githubusercontent.com`;
    const provider = iam.OpenIdConnectProvider.fromOpenIdConnectProviderArn(
      this, 'GitHubOidc', providerArn,
    );
    
    const subject = `repo:${githubRepo}:*`;
    
    const trust = new iam.FederatedPrincipal(
      provider.openIdConnectProviderArn,
      {StringLike: {'token.actions.githubusercontent.com:sub': subject}},
      'sts:AssumeRoleWithWebIdentity',
    );
    
    // ── Frontend deploy role ────────────────────────────────────────────────
    const frontendRole = new iam.Role(this, 'FrontendDeployRole', {
      roleName: 'ctech-dfe-gha-frontend',
      assumedBy: trust,
    });
    frontendRole.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:PutObject', 's3:DeleteObject', 's3:GetObject', 's3:ListBucket'],
      resources: [
        'arn:aws:s3:::*-ctech-dfe-frontend',
        'arn:aws:s3:::*-ctech-dfe-frontend/*',
      ],
    }));
    frontendRole.addToPolicy(new iam.PolicyStatement({
      actions: ['cloudfront:CreateInvalidation'],
      resources: ['*'],
    }));
    frontendRole.addToPolicy(new iam.PolicyStatement({
      actions: ['cloudformation:describeStacks'],
      resources: ['*'],
    }));
    
    // ── API deploy role ─────────────────────────────────────────────────────
    const apiRole = new iam.Role(this, 'ApiDeployRole', {
      roleName: 'ctech-dfe-gha-api',
      assumedBy: trust,
    });
    // AWSElasticBeanstalkFullAccess covers all the internal operations EB does
    // during UpdateEnvironment (CloudFormation, EC2, AutoScaling, S3, etc.).
    // Scoping individual actions is impractical - each EB release may require new ones.
    apiRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName('AdministratorAccess-AWSElasticBeanstalk'),
    );
    // Deployments bucket (our own, not EB-managed)
    apiRole.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:ListBucket'],
      resources: [
        `arn:aws:s3:::${deploymentsBucket}`,
        `arn:aws:s3:::${deploymentsBucket}/*`,
      ],
      conditions: {StringLike: {'s3:prefix': 'ctech-dfe/*'}},
    }));
    apiRole.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:PutObject', 's3:GetObject'],
      resources: [
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe`,
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe/*`,
      ],
    }));
    
    // Trigger deploy on running instances via SSM RunCommand
    apiRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'ssm:SendCommand',
        'ssm:GetCommandInvocation',
        'ssm:ListCommands',
        'ssm:ListCommandInvocations',
      ],
      resources: ['*'],
    }));
    // Discover instances by ASG tag
    apiRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'autoscaling:DescribeAutoScalingGroups',
        'ec2:DescribeInstances',
      ],
      resources: ['*'],
    }));
    apiRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'autoscaling:StartInstanceRefresh',
        'autoscaling:DescribeInstanceRefreshes',
        'autoscaling:DescribeAutoScalingGroups',
        'autoscaling:CancelInstanceRefresh',
      ],
      resources: ['*'],
    }));
    
    // ── Infra deploy role ───────────────────────────────────────────────────
    const infraRole = new iam.Role(this, 'InfraDeployRole', {
      roleName: 'ctech-dfe-gha-infra',
      assumedBy: trust,
    });
    // CDK requires broad permissions to manage CloudFormation stacks
    infraRole.addManagedPolicy(
      iam.ManagedPolicy.fromAwsManagedPolicyName('AdministratorAccess'),
    );
    
    // ── ctech-dfe Lambda deploy role ───────────────────────────────────────────
    const pyDfeRole = new iam.Role(this, 'PyDfeLambdaDeployRole', {
      roleName: 'ctech-dfe-gha-pydfe',
      assumedBy: trust,
    });
    pyDfeRole.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:PutObject', 's3:GetObject'],
      resources: [
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe`,
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe/*`,
      ],
    }));
    pyDfeRole.addToPolicy(new iam.PolicyStatement({
      actions: ['lambda:UpdateFunctionCode', 'lambda:GetFunction', 'lambda:GetFunctionConfiguration'],
      resources: ['arn:aws:lambda:*:*:function:*-py-dfe'],
    }));
    
    // ── Worker Lambda deploy role ───────────────────────────────────────────
    const workerRole = new iam.Role(this, 'WorkerLambdaDeployRole', {
      roleName: 'ctech-dfe-gha-worker',
      assumedBy: trust,
    });
    workerRole.addToPolicy(new iam.PolicyStatement({
      actions: ['s3:PutObject', 's3:GetObject'],
      resources: [
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe`,
        `arn:aws:s3:::${deploymentsBucket}/ctech-dfe/*`,
      ],
    }));
    workerRole.addToPolicy(new iam.PolicyStatement({
      actions: ['lambda:UpdateFunctionCode', 'lambda:GetFunction', 'lambda:GetFunctionConfiguration'],
      resources: [
        'arn:aws:lambda:*:*:function:*-*-worker',
        'arn:aws:lambda:*:*:function:*-*-dlq-processor',
        'arn:aws:lambda:*:*:function:*-*-dispatcher',
      ],
    }));
    
    new cdk.CfnOutput(this, 'FrontendRoleArn', {value: frontendRole.roleArn});
    new cdk.CfnOutput(this, 'ApiRoleArn', {value: apiRole.roleArn});
    new cdk.CfnOutput(this, 'InfraRoleArn', {value: infraRole.roleArn});
    new cdk.CfnOutput(this, 'PyDfeLambdaRoleArn', {value: pyDfeRole.roleArn});
    new cdk.CfnOutput(this, 'WorkerLambdaRoleArn', {value: workerRole.roleArn});
  }
}
