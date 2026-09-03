import {readFileSync} from 'node:fs';
import * as path from 'node:path';
import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as ssm from 'aws-cdk-lib/aws-ssm';
import {Construct} from 'constructs';
import {Ec2ScriptRunner, HaproxyEc2Service, SSM as CtechSSM} from '@aoctech/cdk';
import {Environment} from './types';

const API_SPOT_INSTANCE_TYPES = ['t4g.nano', 't4g.micro'] as const;

/** Emits `cat > /etc/nginx/conf.d/<name> << 'DELIM' … DELIM` for a checked-in file. */
function nginxFragment(name: string, delimiter: string): string[] {
  const body = readFileSync(path.join(__dirname, '..', 'scripts', 'api', name), 'utf8');
  return [
    `cat > /etc/nginx/conf.d/${name} << '${delimiter}'`,
    ...body.replace(/\n$/, '').split('\n'),
    delimiter,
  ];
}

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
  // Session Manager. CI deploys over SSM RunCommand (/opt/app/deploy.sh), which
  // needs the agent running. On also means a shell back onto the box.
  enableSsmAgent?: boolean;
  // 'alpine' pilots the same ctech-billing/ctech-account/ctech-wallet/
  // ctech-poker custom AMI + OpenRC pattern here. Default 'alpine';
  // 'al2023' is the one-line rollback.
  osFamily?: 'al2023' | 'alpine';
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
      enableSsmAgent = false,
      osFamily = 'alpine',
    } = props;
    const isAlpine = osFamily === 'alpine';

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
    // The reach check's own credential: whether a person may act for a company
    // (ctech-billing ADR 0023). Separate from the billing pair because it
    // carries a different scope, and one being wrong must not disable the other.
    //
    // Absent means the check is OFF and ctech-dfe's own membership row remains
    // the access record — the pre-flip behaviour, and a deliberate default.
    const accountClientIdParameter = `/ctech-dfe/${environment}/account-client-id`;
    const accountClientSecretParameter = `/ctech-dfe/${environment}/account-client-secret`;
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
    // Every shared bootstrap step lives in ctech-cdk's assets/ec2 and is fetched
    // from S3 at boot. What stays inline is only what CloudFormation has to
    // resolve: bucket names, SSM paths, log group names, the CloudWatch agent
    // config, and this service's nginx additions.
    //
    // The S3 key prefix is the content hash of assets/ec2, read from SSM at
    // deploy time, so editing a shared script changes this user data, versions
    // the launch template and triggers an instance refresh.
    const userData = ec2.UserData.forLinux();
    let scripts: Ec2ScriptRunner | undefined;

    if (isAlpine) {
      const scriptsBucket = ssm.StringParameter.valueForStringParameter(
        this, CtechSSM.ec2ScriptsAlpine(environment).bucket,
      );
      const scriptsVersion = ssm.StringParameter.valueForStringParameter(
        this, CtechSSM.ec2ScriptsAlpine(environment).version,
      );
      userData.addCommands(
        'export AWS_USE_DUALSTACK_ENDPOINT=true',
        `CTECH_SCRIPTS_BUCKET="${scriptsBucket}"`,
        `CTECH_SCRIPTS_VERSION="${scriptsVersion}"`,
        'ctech_run(){ s=$1; shift; ctech-ec2-agent s3-cp -bucket "$CTECH_SCRIPTS_BUCKET" -key "$CTECH_SCRIPTS_VERSION/$s" -dest "/tmp/$s"; bash "/tmp/$s" "$@"; }',
      );
      userData.addCommands(`ctech_run setup-base.sh ${svcName} nginx nginx-openrc`);
      userData.addCommands('ctech_run setup-swap.sh 256');
      userData.addCommands('ctech_run setup-dualstack.sh');
      userData.addCommands('ctech_run setup-cloudflare-ca.sh');
      if (!enableSsmAgent) {
        userData.addCommands('rc-service amazon-ssm-agent stop 2>/dev/null || true', 'rc-update del amazon-ssm-agent default 2>/dev/null || true');
      }
    } else {
      scripts = new Ec2ScriptRunner(this, 'Scripts', {environment});
      scripts.install(userData);

      scripts.run(userData, 'setup-base.sh', svcName, 'nginx');
      scripts.run(userData, 'setup-swap.sh', '256');
      scripts.run(userData, 'setup-dualstack.sh');
      scripts.run(userData, 'setup-cloudflare-ca.sh');

      // setup-base.sh installs the SSM agent and setup-dualstack.sh starts it, so
      // this is what stops it again.
      if (!enableSsmAgent) {
        userData.addCommands('systemctl disable --now amazon-ssm-agent 2>/dev/null || true');
      }
    }

    // /etc/app-static.env: non-secret values systemd loads via EnvironmentFile.
    // CDK tokens are substituted at synthesis; bash does not expand them.
    userData.addCommands(
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

    // Secrets are read by name at service start, never embedded: the launch
    // template is readable by anyone holding ec2:DescribeLaunchTemplateVersions.
    const ssmEnvArgs = [
      `VALKEY_URL=${valkeyUrlSsmPath}`,
      `CTECH_JWKS_URL=${accountInternalJwksUrlParameter}`,
      `CTECH_URL=${accountInternalBaseUrlParameter}`,
      `CTECH_ISSUER_URL=${accountIssuerUrlParameter}`,
      `SERVICE_AUDIENCE=${appUrlParameter}`,
      `BILLING_WEBHOOK_SECRET=${billingWebhookSecretParameter}`,
      `BILLING_API_URL=${billingBaseUrlParameter}`,
      `BILLING_CLIENT_ID=${billingClientIdParameter}`,
      `BILLING_CLIENT_SECRET=${billingClientSecretParameter}`,
      `ACCOUNT_CLIENT_ID=${accountClientIdParameter}`,
      `ACCOUNT_CLIENT_SECRET=${accountClientSecretParameter}`,
    ];
    if (isAlpine) {
      const quoted = ssmEnvArgs.map((a) => `'${a.replace(/'/g, `'\\''`)}'`).join(' ');
      userData.addCommands(`ctech_run setup-ssm-env.sh ${quoted}`);
    } else {
      scripts!.run(userData, 'setup-ssm-env.sh', ...ssmEnvArgs);
    }

    // CORS_ALLOWED_ORIGINS is derived, not fetched — the escape hatch start.sh
    // sources after load-ssm-env.sh.
    userData.addCommands(
      `cat > /opt/app/service-env.sh << 'SERVICEENV'`,
      `CORS_ALLOWED_ORIGINS="$SERVICE_AUDIENCE"`,
      `export CORS_ALLOWED_ORIGINS`,
      `SERVICEENV`,
      `chmod 0755 /opt/app/service-env.sh`,
    );

    // Per-service nginx additions, installed before setup-nginx.sh runs `nginx -t`.
    userData.addCommands(
      ...nginxFragment('http-dfe.conf', 'HTTPDFE'),
      ...nginxFragment('location-dfe.conf', 'LOCDFE'),
      ...nginxFragment('proxy-dfe.conf', 'PROXYDFE'),
    );

    if (isAlpine) {
      userData.addCommands(`ctech_run setup-realip.sh '${vpc.vpcCidrBlock}'`);
      // app-port-alt/alt-port (8001) turn on the zero-downtime rolling deploy: a
      // second app process nginx round-robins into, so deploy.sh can restart one
      // unit at a time instead of dropping the health check during a restart.
      userData.addCommands(`ctech_run setup-nginx.sh 8080 8000 /v1.0/health-check 100 20m 8001`);
      // Alpine's setup-app-service.sh has no After=-units argument — OpenRC
      // services here only ever declare `need net`.
      userData.addCommands(`ctech_run setup-app-service.sh 'CTech DFe API' app 8001`);
      userData.addCommands(
        `ctech_run setup-deploy.sh ${deploymentsBucketName} app 'http://127.0.0.1:8080/v1.0/health-check'`,
      );
      userData.addCommands(
        `ctech_run setup-logs.sh ${logsBucketName} ${svcName} ${svcName} /var/log/app /var/log/nginx`,
      );

      // ctech-ec2-agent logs-tail replaces the CloudWatch Agent (musl has no
      // working aws-cli/CW-agent build). One logGroup per config file, so two
      // separate services + configs, same as the other Alpine pilots.
      userData.addCommands(
        `cat > /tmp/ctech-logs-app.json << 'LOGSAPP'`,
        JSON.stringify({
          logGroup: logGroupApp,
          files: [
            {path: '/var/log/app/app.log', streamPrefix: 'app'},
            {path: '/var/log/app/app2.log', streamPrefix: 'app2'},
          ],
        }),
        `LOGSAPP`,
        `ctech_run setup-ctech-ec2-agent.sh /tmp/ctech-logs-app.json app`,
        `cat > /tmp/ctech-logs-nginx.json << 'LOGSNGINX'`,
        JSON.stringify({
          logGroup: logGroupNginx,
          files: [
            {path: '/var/log/nginx/access.log', streamPrefix: 'access'},
            {path: '/var/log/nginx/error.log', streamPrefix: 'error'},
          ],
        }),
        `LOGSNGINX`,
        `ctech_run setup-ctech-ec2-agent.sh /tmp/ctech-logs-nginx.json nginx`,
      );
      userData.addCommands(`ctech_run bootstrap-deploy.sh ${deploymentsBucketName} ctech-dfe/api/current.zip`);
    } else {
      scripts!.run(userData, 'setup-realip.sh', vpc.vpcCidrBlock);
      scripts!.run(userData, 'setup-nginx.sh', '8080', '8000', '/v1.0/health-check', '100', '20m', '8001');
      scripts!.run(userData, 'setup-app-service.sh', 'CTech DFe API', 'app', 'network.target nginx.service', '8001');
      scripts!.run(userData, 'setup-deploy.sh', deploymentsBucketName, 'app',
        'http://127.0.0.1:8080/v1.0/health-check');
      scripts!.run(userData, 'setup-logs.sh', logsBucketName, svcName, svcName,
        '/var/log/app', '/var/log/nginx');

      // Logs only. No `metrics` block: EC2 already publishes CPUUtilization and
      // CPUCreditBalance for free, and every custom series this service used to
      // publish was either that again or a number nobody alarmed on.
      // {instance_id} is resolved by the CW agent at runtime, not by bash.
      userData.addCommands(
        `cat > /tmp/cwagent.json << 'CWA'`,
        JSON.stringify({
          agent: {metrics_collection_interval: 60},
          logs: {
            logs_collected: {
              files: {
                collect_list: [
                  {file_path: '/var/log/app/app.log', log_group_name: logGroupApp, log_stream_name: '{instance_id}'},
                  {file_path: '/var/log/app/app2.log', log_group_name: logGroupApp, log_stream_name: '{instance_id}/app2'},
                  {file_path: '/var/log/nginx/access.log', log_group_name: logGroupNginx, log_stream_name: '{instance_id}/access'},
                  {file_path: '/var/log/nginx/error.log', log_group_name: logGroupNginx, log_stream_name: '{instance_id}/error'},
                ],
              },
            },
          },
        }),
        `CWA`,
      );
      scripts!.run(userData, 'setup-cloudwatch-agent.sh', '/tmp/cwagent.json');
      scripts!.run(userData, 'bootstrap-deploy.sh', deploymentsBucketName, 'ctech-dfe/api/current.zip');
    }

    const machineImage = isAlpine
      ? ec2.MachineImage.fromSsmParameter(
          CtechSSM.amiAlpine(environment).arm64,
          {os: ec2.OperatingSystemType.LINUX},
        )
      : undefined; // HaproxyEc2Service defaults to latest AL2023 arm64 minimal.

    // ctech-lbalancer still owns the bootstrap route and private CNAME.
    const service = new HaproxyEc2Service(this, 'ApiService', {
      vpc,
      edgeSecurityGroup: edgeSg,
      appPort: 8080,
      userData,
      machineImage,
      instanceProfileName,
      securityGroupName: `${environment}-${svcName}-api-sg`,
      securityGroupDescription: 'ctech-dfe API instances',
      appLogGroupName: logGroupApp,
      nginxLogGroupName: logGroupNginx,
      logRetention,
      logRemovalPolicy: isProd ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
      asgName: this.asgName,
      minCapacity: 1,
      // +1 over min: gives CapacityRebalance headroom to launch the
      // replacement before terminating the spot-interrupted instance instead
      // of waiting for it to go down first.
      maxCapacity: 2,
      // The ASG runs only inside a narrow daytime window: up at 11:55 and down
      // at 13:15 America/Sao_Paulo. Outside it the service is off — inbound
      // webhooks fail and nothing is reachable. Deliberate for a development
      // environment on a single t4g.nano.
      // schedule: {enableCron: '55 11 * * *', disableCron: '15 13 * * *'},
      spot: {
        instanceTypes: API_SPOT_INSTANCE_TYPES.map((type) => new ec2.InstanceType(type)),
      },
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
