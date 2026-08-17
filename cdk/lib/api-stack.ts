import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as s3assets from 'aws-cdk-lib/aws-s3-assets';
import * as ssm from 'aws-cdk-lib/aws-ssm';
import {Construct} from 'constructs';
import {
  addCloudflareOriginCaCommands,
  addCloudWatchAgentDualStackOverride,
  addDualStackSsmAgentCommands,
  addRealipRefreshCommands,
  addSwapCommands,
  buildCloudWatchAgentConfig,
  HaproxyEc2Service,
} from '@aoctech/cdk';
import {Environment} from './types';

interface ApiStackProps extends cdk.StackProps {
  environment: Environment;
  // VPC ID must be provided as a concrete string (not a token) so CDK can
  // resolve subnet/AZ information via Vpc.fromLookup at synthesis time.
  // Read it from the CTECH_VPC_ID env var, which the CI workflow populates
  // from the /ctech/{env}/network/vpc-id SSM parameter before running cdk deploy.
  vpcId: string;
  instanceProfileName: string;
  deploymentsBucketName: string;
  logsBucketName: string;
  certificatesBucketName: string;
  documentsBucketName: string;
  resultsQueueUrl: string;
  nfeEmissionTopicArn: string;
  distributionQueueUrl: string;
  // SSM path written by ValkeyStack at boot. If omitted, VALKEY_URL is not set
  // and the app falls back to NoCacheBackend.
  valkeyUrlSsmPath: string;
}

export class ApiStack extends cdk.Stack {
  public readonly asgName: string;

  constructor(scope: Construct, id: string, props: ApiStackProps) {
    super(scope, id, props);

    const {
      environment,
      vpcId,
      instanceProfileName,
      deploymentsBucketName,
      logsBucketName,
      certificatesBucketName,
      documentsBucketName,
      resultsQueueUrl,
      nfeEmissionTopicArn,
      distributionQueueUrl,
      valkeyUrlSsmPath,
    } = props;

    // ── Shared infrastructure from ctech-cdk (resolved at deploy time via SSM) ─
    const vpc = ec2.Vpc.fromLookup(this, 'Vpc', {vpcId});

    const albSgId = ssm.StringParameter.valueForStringParameter(
      this, `/ctech/${environment}/network/alb-sg-id`,
    );
    const edgeSg = ec2.SecurityGroup.fromSecurityGroupId(this, 'EdgeSg', albSgId);

    const isProd = environment === 'prod';
    const accountInternalBaseUrlParameter = `/ctech-account/${environment}/internal-base-url`;
    const accountInternalJwksUrlParameter = `/ctech-account/${environment}/internal-jwks-url`;
    const accountIssuerUrlParameter = `/ctech-account/${environment}/app-url`;
    const appUrlParameter = `/ctech-dfe/${environment}/app-url`;
    // HMAC key that authenticates ctech-billing's outbound webhooks. A
    // SecureString, and one CloudFormation cannot create — so this stack only
    // reads it. It is written once, out of band, with the same value the billing
    // seed was given for the `whe_dfe` endpoint (`WEBHOOK_SECRET_DFE`); see
    // DEPLOYMENT.md § Out-of-band parameters.
    const billingWebhookSecretParameter = `/ctech-dfe/${environment}/billing/webhook-secret`;
    // The other direction: DF-e calling billing as an OAuth client-credentials
    // client (`dfe-billing`, issued by ctech-account). The credentials live under
    // this service's own prefix because they belong to the caller, not the
    // callee — billing knows nothing about them. The base URL is published by
    // ctech-cdk alongside every other private service endpoint, so a hostname
    // change is one edit rather than one per caller.
    const billingBaseUrlParameter = `/ctech-billing/${environment}/internal-base-url`;
    const billingClientIdParameter = `/ctech-dfe/${environment}/billing/client-id`;
    const billingClientSecretParameter = `/ctech-dfe/${environment}/billing/client-secret`;
    // Bumped (v2 → v3 / new log-and-SG names): moving the ASG/SG/log groups into
    // HaproxyEc2Service changes their CloudFormation logical IDs, which
    // CloudFormation treats as delete-old/create-new. Explicit physical names
    // must differ from the old ones or the create side of that swap collides
    // with the still-live old resource.
    const svcName = 'ctech-dfe';
    this.asgName = `${environment}-${svcName}`;
    const logRetention: logs.RetentionDays = isProd ? logs.RetentionDays.ONE_MONTH : logs.RetentionDays.ONE_WEEK;
    const logGroupApp = `/${svcName}/${environment}/app`;
    const logGroupNginx = `/${svcName}/${environment}/nginx`;

    // ── User Data ─────────────────────────────────────────────────────────────
    // Everything static — nginx.conf, the systemd unit, start/deploy/upload-logs —
    // lives in `scripts/api` and rides to the instance as an S3 asset. EC2 caps
    // user data at 16 KB and those files alone were most of it; what stays inline
    // is only what CloudFormation has to resolve: bucket names, SSM paths, log
    // group names, the VPC CIDR.
    //
    // The asset's S3 key is the hash of the directory, so editing a script changes
    // the user data, which versions the launch template and triggers an instance
    // refresh. A fixed key in a bucket of our own would not: the file would change
    // under running instances while the template stayed byte-identical, and new
    // instances would quietly boot a different machine than the old ones.
    const bootstrap = new s3assets.Asset(this, 'ApiBootstrap', {
      path: path.join(__dirname, '..', 'scripts', 'api'),
    });

    const userData = ec2.UserData.forLinux();

    userData.addCommands(
      // ── Packages + directories ───────────────────────────────────────────────
      'dnf install -y nginx amazon-cloudwatch-agent amazon-ssm-agent cronie unzip jq',
      'useradd --system --no-create-home --shell /sbin/nologin webapp',
      'mkdir -p /opt/app/releases /var/log/app /etc/nginx/conf.d',
      'chown -R webapp:webapp /opt/app /var/log/app',
      // AL2023 does not enable crond by default (unlike AL2) — without it
      // /etc/cron.daily/logrotate never fires and rotated logs never reach S3.
      'systemctl enable crond',
      'systemctl start crond',
    );

    addSwapCommands(userData);
    addDualStackSsmAgentCommands(userData);
    addCloudflareOriginCaCommands(userData);

    userData.addCommands(
      // ── /etc/bootstrap.env: the per-environment half ─────────────────────────
      // Read by setup.sh, start.sh, deploy.sh and upload-logs.sh. It carries SSM
      // parameter *names*, never their values — the secrets are read at service
      // start by the instance role, so they never touch the launch template, which
      // is world-readable to anyone with ec2:DescribeLaunchTemplateVersions.
      `cat > /etc/bootstrap.env << 'BOOTSTRAP'`,
      `AWS_REGION=${this.region}`,
      `DEPLOYMENTS_BUCKET=${deploymentsBucketName}`,
      `LOGS_BUCKET=${logsBucketName}`,
      `VALKEY_URL_PARAM=${valkeyUrlSsmPath}`,
      `CTECH_JWKS_URL_PARAM=${accountInternalJwksUrlParameter}`,
      `CTECH_URL_PARAM=${accountInternalBaseUrlParameter}`,
      `CTECH_ISSUER_URL_PARAM=${accountIssuerUrlParameter}`,
      `APP_URL_PARAM=${appUrlParameter}`,
      `BILLING_WEBHOOK_SECRET_PARAM=${billingWebhookSecretParameter}`,
      `BILLING_API_URL_PARAM=${billingBaseUrlParameter}`,
      `BILLING_CLIENT_ID_PARAM=${billingClientIdParameter}`,
      `BILLING_CLIENT_SECRET_PARAM=${billingClientSecretParameter}`,
      `BOOTSTRAP`,

      // ── Static env file (loaded by systemd EnvironmentFile=) ─────────────────
      // CDK tokens are substituted at synthesis time; bash does not expand them.
      `cat > /etc/app-static.env << 'ENV'`,
      `ENVIRONMENT=${environment}`,
      `TABLE_PREFIX=${environment}_dfe`,
      `AWS_REGION=${this.region}`,
      `AWS_USE_DUALSTACK_ENDPOINT=true`,
      `S3_BUCKET_CERTIFICATES=${certificatesBucketName}`,
      `S3_BUCKET_DOCUMENTS=${documentsBucketName}`,
      `DFE_TOPIC_ARN=${nfeEmissionTopicArn}`,
      `DFE_RESULTS_QUEUE_URL=${resultsQueueUrl}`,
      `DFE_DISTRIBUTION_QUEUE_URL=${distributionQueueUrl}`,
      `SEFAZ_FUNCTION_NAME=${environment}-py-dfe`,
      `TRUSTED_PROXIES=127.0.0.1`,
      `ENV`,
    );

    // ── The static half, from S3 ──────────────────────────────────────────────
    userData.addS3DownloadCommand({
      bucket: bootstrap.bucket,
      bucketKey: bootstrap.s3ObjectKey,
      localFile: '/tmp/api-bootstrap.zip',
    });
    userData.addCommands(
      'rm -rf /opt/bootstrap',
      'mkdir -p /opt/bootstrap',
      'unzip -o /tmp/api-bootstrap.zip -d /opt/bootstrap',
      // Zip does not carry the exec bit through CDK's asset staging.
      'chmod +x /opt/bootstrap/*.sh',
    );

    // Before setup.sh, which is what first starts nginx: the fragment generates
    // realip.conf, and nginx must never serve a request with the ALB as the
    // rate-limit key.
    addRealipRefreshCommands(userData, vpc.vpcCidrBlock);

    userData.addCommands('/opt/bootstrap/setup.sh');

    addCloudWatchAgentDualStackOverride(userData);

    userData.addCommands(
      // Generated rather than shipped in the asset: the log group names and the
      // metric namespace are CloudFormation values, and `buildCloudWatchAgentConfig`
      // is shared with every other CTech service. ~1.5 KB, which the budget affords.
      // {instance_id} is resolved by the CW agent at runtime, not by bash.
      `cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json << 'CWA'`,
      buildCloudWatchAgentConfig({
        metricNamespace: `CtechDfe/${environment}/Host`,
        appProcessPattern: '/opt/app/current/(app|bootstrap)',
        logFiles: [
          {filePath: '/var/log/app/app.log', logGroupName: logGroupApp, logStreamName: '{instance_id}'},
          {filePath: '/var/log/nginx/access.log', logGroupName: logGroupNginx, logStreamName: '{instance_id}/access'},
          {filePath: '/var/log/nginx/error.log', logGroupName: logGroupNginx, logStreamName: '{instance_id}/error'},
        ],
      }),
      `CWA`,
      `/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -s`,

      // ── Bootstrap: deploy current.zip if it already exists in S3 ────────────
      `aws s3api head-object --bucket "${deploymentsBucketName}" --key "ctech-dfe/api/current.zip" 2>/dev/null && /opt/app/deploy.sh ctech-dfe/api/current.zip || echo "No bootstrap artifact, waiting for first deploy"`,
    );

    // ctech-lbalancer still owns the bootstrap route and private CNAME.
    const service = new HaproxyEc2Service(this, 'ApiService', {
      vpc,
      edgeSecurityGroup: edgeSg,
      appPort: 8080,
      userData,
      instanceProfileName,
      securityGroupName: `${environment}-${svcName}-api-sg`,
      securityGroupDescription: 'ctech-dfe API instances',
      appLogGroupName: logGroupApp,
      nginxLogGroupName: logGroupNginx,
      logRetention,
      logRemovalPolicy: isProd ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      asgName: this.asgName,
      minCapacity: 1,
      maxCapacity: isProd ? 3 : 1,
    });

    // ── Outputs ───────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'AsgName', {value: service.autoScalingGroup.autoScalingGroupName, exportName: `${id}-asg-name`});
    new cdk.CfnOutput(this, 'AppLogGroupName', {
      value: service.appLogGroup.logGroupName,
      exportName: `${id}-app-log-group`,
    });
    new cdk.CfnOutput(this, 'NginxLogGroupName', {
      value: service.nginxLogGroup!.logGroupName,
      exportName: `${id}-nginx-log-group`,
    });
  }
}
