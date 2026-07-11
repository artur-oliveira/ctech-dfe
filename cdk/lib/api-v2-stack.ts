import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as autoscaling from 'aws-cdk-lib/aws-autoscaling';
import {AdditionalHealthCheckType} from 'aws-cdk-lib/aws-autoscaling';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as ssm from 'aws-cdk-lib/aws-ssm';
import {Construct} from 'constructs';
import {Environment} from './types';
import {Duration} from 'aws-cdk-lib';

interface ApiStackV2Props extends cdk.StackProps {
  environment: Environment;
  // VPC ID must be provided as a concrete string (not a token) so CDK can
  // resolve subnet/AZ information via Vpc.fromLookup at synthesis time.
  // Read it from the CTECH_VPC_ID env var, which the CI workflow populates
  // from the /ctech/{env}/network/vpc-id SSM parameter before running cdk deploy.
  vpcId: string;
  domainName: string;
  appDomainName: string;
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

export class ApiStackV2 extends cdk.Stack {
  public readonly asgName: string;

  constructor(scope: Construct, id: string, props: ApiStackV2Props) {
    super(scope, id, props);

    const {
      environment,
      vpcId,
      domainName,
      appDomainName,
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
    const albSg = ec2.SecurityGroup.fromSecurityGroupId(this, 'AlbSg', albSgId);

    const apiSecurityGroup = new ec2.SecurityGroup(this, 'ApiSg', {
      vpc,
      securityGroupName: `${environment}-ctech-dfe-api-sg`,
      description: 'ctech-dfe API instances',
      allowAllOutbound: true,
      allowAllIpv6Outbound: true,
    });
    apiSecurityGroup.addIngressRule(albSg, ec2.Port.tcp(8080), 'ALB to API');

    const httpsListenerArn = ssm.StringParameter.valueForStringParameter(
      this, `/ctech/${environment}/alb/https-listener-arn`,
    );
    const httpsListener = elbv2.ApplicationListener.fromApplicationListenerAttributes(
      this, 'HttpsListener',
      {listenerArn: httpsListenerArn, securityGroup: albSg},
    );

    const isProd = environment === 'prod';
    this.asgName = `${environment}-ctech-dfe-api-v2`;
    const logRetention: logs.RetentionDays = isProd ? logs.RetentionDays.ONE_MONTH : logs.RetentionDays.ONE_WEEK;
    const logGroupApp = `/ctech-dfe/${environment}/app`;
    const logGroupNginx = `/ctech-dfe/${environment}/nginx`;

    // ── CloudWatch Log Groups ─────────────────────────────────────────────────
    const appLogGroup = new logs.LogGroup(this, 'AppLogGroup', {
      logGroupName: logGroupApp,
      retention: logRetention,
      removalPolicy: isProd ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
    });

    const nginxLogGroup = new logs.LogGroup(this, 'NginxLogGroup', {
      logGroupName: logGroupNginx,
      retention: logRetention,
      removalPolicy: isProd ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
    });

    // ── HTTP status code metric filters (nginx JSON access log) ───────────────
    for (const [name, pattern] of [
      ['HTTP2XX', '{ ($.status >= 200) && ($.status < 300) }'],
      ['HTTP3XX', '{ ($.status >= 300) && ($.status < 400) }'],
      ['HTTP4XX', '{ ($.status >= 400) && ($.status < 500) }'],
      ['HTTP5XX', '{ $.status >= 500 }'],
    ] as [string, string][]) {
      new logs.MetricFilter(this, `${name}Filter`, {
        logGroup: nginxLogGroup,
        metricNamespace: `CtechDfe/${environment}`,
        metricName: name,
        filterPattern: logs.FilterPattern.literal(pattern),
        metricValue: '1',
        defaultValue: 0,
      });
    }

    // ── User Data ─────────────────────────────────────────────────────────────
    const userData = ec2.UserData.forLinux();

    userData.addCommands(
      // ── Packages + directories ───────────────────────────────────────────────
      'dnf install -y nginx amazon-cloudwatch-agent amazon-ssm-agent unzip',
      'useradd --system --no-create-home --shell /sbin/nologin webapp',
      'mkdir -p /opt/app/releases /var/log/app',
      'chown -R webapp:webapp /opt/app /var/log/app',

      // ── Swap (256 MB) ──────────────────────────────────────────────────────────
      // Prevents OOM on t4g.micro (1 GB RAM) under memory pressure.
      'if [ ! -f /var/swapfile ]; then',
      '  dd if=/dev/zero of=/var/swapfile bs=1M count=256',
      '  chmod 600 /var/swapfile',
      '  mkswap /var/swapfile',
      '  swapon /var/swapfile',
      '  echo "/var/swapfile swap swap defaults 0 0" >> /etc/fstab',
      'fi',

      // ── System-wide dual-stack endpoint (SSM agent, CW agent, boto3 CLI) ────
      'echo "AWS_USE_DUALSTACK_ENDPOINT=true" >> /etc/environment',

      // ── SSM agent: force IPv6 dual-stack endpoint ────────────────────────────
      // Without this the SSM agent fails to connect when the instance has no public IPv4.
      `mkdir -p /etc/amazon/ssm`,
      `cat > /etc/amazon/ssm/amazon-ssm-agent.json << 'SSM'`,
      `{ "Agent": { "UseDualStackEndpoint": true } }`,
      `SSM`,
      'systemctl enable amazon-ssm-agent',
      'systemctl restart amazon-ssm-agent',

      // ── nginx: listens :8080, proxies to app :8000 ───────────────────────────
      // Incorporates all production tuning from .platform/nginx/nginx.conf.
      // Quoted delimiter prevents bash from expanding nginx $variables.
      // $http_x_forwarded_proto (not $scheme) is correct behind the ALB.
      `cat > /etc/nginx/nginx.conf << 'NGINX'`,
      `user nginx;`,
      `pid /run/nginx.pid;`,
      `worker_processes auto;`,
      `worker_rlimit_nofile 65535;`,
      `error_log /var/log/nginx/error.log warn;`,
      ``,
      `events {`,
      `    worker_connections 8192;`,
      `    use epoll;`,
      `    multi_accept on;`,
      `}`,
      ``,
      `http {`,
      `    include /etc/nginx/mime.types;`,
      `    default_type application/octet-stream;`,
      ``,
      `    log_format json_log escape=json '{"remote_addr":"$remote_addr","status":$status,"request":"$request","body_bytes_sent":$body_bytes_sent,"request_time":$request_time,"upstream_response_time":"$upstream_response_time"}';`,
      ``,
      `    include /usr/share/nginx/modules/*.conf;`,
      ``,
      `    sendfile on;`,
      `    tcp_nopush on;`,
      `    tcp_nodelay on;`,
      `    keepalive_timeout 30;`,
      `    keepalive_requests 10000;`,
      `    reset_timedout_connection on;`,
      `    open_file_cache max=1000 inactive=20s;`,
      `    open_file_cache_valid 30s;`,
      `    open_file_cache_min_uses 2;`,
      `    open_file_cache_errors on;`,
      ``,
      `    types_hash_max_size 2048;`,
      `    types_hash_bucket_size 128;`,
      ``,
      `    client_header_timeout 15s;`,
      `    client_body_timeout 30s;`,
      `    send_timeout 30s;`,
      ``,
      `    client_max_body_size 20m;`,
      `    client_body_buffer_size 128k;`,
      `    client_header_buffer_size 1k;`,
      `    large_client_header_buffers 4 8k;`,
      ``,
      `    gzip on;`,
      `    gzip_vary on;`,
      `    gzip_proxied any;`,
      `    gzip_comp_level 5;`,
      `    gzip_min_length 1024;`,
      `    gzip_buffers 16 8k;`,
      `    gzip_http_version 1.1;`,
      `    gzip_types application/json application/javascript text/plain text/css text/xml application/xml application/rss+xml;`,
      ``,
      `    server_tokens off;`,
      `    proxy_hide_header X-Powered-By;`,
      `    add_header X-Content-Type-Options nosniff always;`,
      `    add_header X-Frame-Options DENY always;`,
      `    add_header Referrer-Policy strict-origin-when-cross-origin always;`,
      ``,
      `    # ── Rate limiting zones ──────────────────────────────────────────────────`,
      `    # Keyed by IP and by tenant header. Empty header key is ignored by nginx`,
      `    # (no limit applied) so IP zone still protects unauthenticated traffic.`,
      `    limit_req_zone $binary_remote_addr        zone=req_by_ip:10m     rate=100r/s;`,
      `    limit_req_zone $http_dfe_organization_pk  zone=req_by_tenant:20m rate=500r/s;`,
      `    limit_conn_zone $binary_remote_addr       zone=conn_by_ip:10m;`,
      `    limit_conn_zone $http_dfe_organization_pk zone=conn_by_tenant:20m;`,
      `    limit_req_status  429;`,
      `    limit_conn_status 429;`,
      ``,
      `    upstream app {`,
      `        server 127.0.0.1:8000;`,
      `        keepalive 256;`,
      `        keepalive_requests 10000;`,
      `        keepalive_timeout 60s;`,
      `    }`,
      ``,
      `    server {`,
      `        listen 8080 default_server reuseport;`,
      `        server_name _;`,
      `        access_log /var/log/nginx/access.log json_log;`,
      `        error_log /var/log/nginx/error.log;`,
      ``,
      `        location = /v1.0/health-check {`,
      `            proxy_pass http://app;`,
      `            proxy_http_version 1.1;`,
      `            proxy_set_header Connection "";`,
      `            proxy_set_header Host $host;`,
      `            proxy_set_header X-Real-IP $remote_addr;`,
      `            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
      `            proxy_set_header X-Forwarded-Proto $http_x_forwarded_proto;`,
      `            proxy_connect_timeout 5s;`,
      `            proxy_read_timeout 5s;`,
      `            access_log off;`,
      `        }`,
      ``,
      `        location /v1.0/ws {`,
      `            proxy_pass http://app;`,
      `            proxy_http_version 1.1;`,
      `            proxy_set_header Upgrade $http_upgrade;`,
      `            proxy_set_header Connection "upgrade";`,
      `            proxy_set_header Host $host;`,
      `            proxy_set_header X-Real-IP $remote_addr;`,
      `            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
      `            proxy_set_header X-Forwarded-Proto $http_x_forwarded_proto;`,
      `            proxy_read_timeout 3600s;`,
      `            proxy_send_timeout 3600s;`,
      `            proxy_buffering off;`,
      `        }`,
      ``,
      `        location / {`,
      `            limit_req zone=req_by_ip     burst=200  nodelay;`,
      `            limit_req zone=req_by_tenant burst=1000 nodelay;`,
      `            limit_conn conn_by_ip     100;`,
      `            limit_conn conn_by_tenant 500;`,
      ``,
      `            proxy_pass http://app;`,
      `            proxy_http_version 1.1;`,
      `            proxy_set_header Connection "";`,
      `            proxy_set_header Host $host;`,
      `            proxy_set_header X-Real-IP $remote_addr;`,
      `            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`,
      `            proxy_set_header X-Forwarded-Proto $http_x_forwarded_proto;`,
      `            proxy_connect_timeout 10s;`,
      `            proxy_send_timeout 60s;`,
      `            proxy_read_timeout 60s;`,
      `            proxy_buffering on;`,
      `            proxy_buffer_size 8k;`,
      `            proxy_buffers 16 16k;`,
      `            proxy_busy_buffers_size 32k;`,
      `        }`,
      `    }`,
      `}`,
      `NGINX`,
      `systemctl enable nginx`,
      `systemctl start nginx`,

      // ── CloudWatch agent ─────────────────────────────────────────────────────
      // Force dual-stack endpoint so the agent can reach CloudWatch Logs over
      // IPv6 (instances have no public IPv4).
      `mkdir -p /etc/systemd/system/amazon-cloudwatch-agent.service.d`,
      `cat > /etc/systemd/system/amazon-cloudwatch-agent.service.d/override.conf << 'CWAENV'`,
      `[Service]`,
      `Environment=AWS_USE_DUALSTACK_ENDPOINT=true`,
      `CWAENV`,

      // {instance_id} is resolved by the CW agent at runtime, not by bash.
      `cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json << 'CWA'`,
      `{`,
      `  "logs": {`,
      `    "logs_collected": {`,
      `      "files": {`,
      `        "collect_list": [`,
      `          {"file_path":"/var/log/app/app.log","log_group_name":"${logGroupApp}","log_stream_name":"{instance_id}"},`,
      `          {"file_path":"/var/log/nginx/access.log","log_group_name":"${logGroupNginx}","log_stream_name":"{instance_id}/access"},`,
      `          {"file_path":"/var/log/nginx/error.log","log_group_name":"${logGroupNginx}","log_stream_name":"{instance_id}/error"}`,
      `        ]`,
      `      }`,
      `    }`,
      `  }`,
      `}`,
      `CWA`,
      `/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a fetch-config -m ec2 -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json -s`,

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
      `CORS_ALLOWED_ORIGINS=https://${appDomainName}`,
      `ENV`,

      // ── start.sh: fetches secrets from SSM then exec-replaces into the Go binary
      // $ENVIRONMENT comes from systemd EnvironmentFile at runtime.
      `cat > /opt/app/start.sh << 'START'`,
      `#!/bin/bash`,
      // APP_VERSION is shipped inside the release artifact (release.env) by CI/CD.
      // Format: YYMMDDHHMM:<7-char commit>. Used for the health check and nota verProc.
      `if [ -f /opt/app/current/release.env ]; then set -a; . /opt/app/current/release.env; set +a; fi`,
      // VALKEY_URL is written by the Valkey instance at boot. If the instance is
      // scaled to 0 or not deployed, the parameter may not exist — fall back to empty
      // so the app uses NoCacheBackend instead of crashing.
      `VALKEY_URL=$(aws ssm get-parameter --name "${valkeyUrlSsmPath}" --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CTECH_JWKS_URL=$(aws ssm get-parameter --name "/ctech-account/$ENVIRONMENT/jwks-url" --with-decryption --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CTECH_URL=$(aws ssm get-parameter --name "/ctech-account/$ENVIRONMENT/base-url" --with-decryption --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `export CTECH_JWKS_URL CTECH_URL VALKEY_URL`,
      `exec /opt/app/current/app`,
      `START`,
      `chmod +x /opt/app/start.sh`,

      // ── systemd app.service ──────────────────────────────────────────────────
      `cat > /etc/systemd/system/app.service << 'SVC'`,
      `[Unit]`,
      `Description=CTech DFe API`,
      `After=network.target nginx.service`,
      `StartLimitIntervalSec=300`,
      `StartLimitBurst=5`,
      ``,
      `[Service]`,
      `User=webapp`,
      `Group=webapp`,
      `WorkingDirectory=/opt/app/current`,
      `Environment=HOME=/opt/app`,
      `EnvironmentFile=/etc/app-static.env`,
      `ExecStartPre=/bin/test -x /opt/app/current/app`,
      `ExecStart=/opt/app/start.sh`,
      `StandardOutput=append:/var/log/app/app.log`,
      `StandardError=append:/var/log/app/app.log`,
      `Restart=on-failure`,
      `RestartSec=30`,
      ``,
      `[Install]`,
      `WantedBy=multi-user.target`,
      `SVC`,
      `systemctl daemon-reload`,
      `systemctl enable app`,

      // ── deploy.sh: called by SSM RunCommand from GitHub Actions ──────────────
      // Expects a zip containing a pre-built `app` binary (built for linux/arm64).
      // __BUCKET__ is replaced by sed so that bash $variables are not expanded
      // at write time (quoted 'DEPLOY' delimiter).
      `cat > /opt/app/deploy.sh << 'DEPLOY'`,
      `#!/bin/bash`,
      `set -euo pipefail`,
      `S3_KEY="$1"`,
      `RELEASE_DIR="/opt/app/releases/$(date +%Y%m%d_%H%M%S)"`,
      `mkdir -p "$RELEASE_DIR"`,
      `echo "Downloading release: $S3_KEY"`,
      `aws s3 cp "s3://__BUCKET__/$S3_KEY" /tmp/release.zip`,
      `unzip -o /tmp/release.zip -d "$RELEASE_DIR"`,
      `chmod +x "$RELEASE_DIR/app"`,
      `chown -R webapp:webapp "$RELEASE_DIR"`,
      `ln -sfT "$RELEASE_DIR" /opt/app/current`,
      `systemctl restart app 2>/dev/null || systemctl start app`,
      `for i in {1..60}; do`,
      `  if curl -sf http://127.0.0.1:8080/v1.0/health-check >/dev/null; then`,
      `    echo "Health check passed"`,
      `    break`,
      `  fi`,
      `  if systemctl is-failed --quiet app; then`,
      `    echo "Application failed to start"`,
      `    journalctl -u app --no-pager -n 100 || true`,
      `    exit 1`,
      `  fi`,
      `  sleep 2`,
      `done`,
      `curl -sf http://127.0.0.1:8080/v1.0/health-check >/dev/null || {`,
      `  echo "Timed out waiting for health check"`,
      `  exit 1`,
      `}`,
      `ls -dt /opt/app/releases/*/ 2>/dev/null | tail -n +2 | xargs rm -rf 2>/dev/null || true`,
      `echo "Deployment successful"`,
      `DEPLOY`,
      `sed -i 's|__BUCKET__|${deploymentsBucketName}|g' /opt/app/deploy.sh`,
      `chmod +x /opt/app/deploy.sh`,

      // ── upload-logs.sh: bundles rotated logs and ships to S3 ─────────────────
      // IMDSv2 token required (requireImdsv2 is enforced on this instance).
      // __LOG_BUCKET__ replaced by sed so bash $variables are not expanded.
      `cat > /opt/app/upload-logs.sh << 'UPLOAD'`,
      `#!/bin/bash`,
      `TOKEN=$(curl -sf -X PUT "http://169.254.169.254/latest/api/token" \\`,
      `    -H "X-aws-ec2-metadata-token-ttl-seconds: 60")`,
      `INSTANCE_ID=$(curl -sf -H "X-aws-ec2-metadata-token: $TOKEN" \\`,
      `    "http://169.254.169.254/latest/meta-data/instance-id" || echo "unknown")`,
      `DATE=$(date +%Y%m%d)`,
      `BUCKET="__LOG_BUCKET__"`,
      `ARCHIVE="/tmp/\${DATE}-\${INSTANCE_ID}.tar.gz"`,
      `ROTATED=$(find /var/log/app /var/log/nginx -name "*-\${DATE}.gz" 2>/dev/null)`,
      `[ -z "$ROTATED" ] && exit 0`,
      `tar czf "$ARCHIVE" $ROTATED 2>/dev/null || exit 0`,
      `aws s3 cp "$ARCHIVE" "s3://\${BUCKET}/ctech-dfe/\${DATE}-\${INSTANCE_ID}.tar.gz" --region us-east-1 || exit 0`,
      `find /var/log/app /var/log/nginx -name "*-\${DATE}.gz" -delete`,
      `rm -f "$ARCHIVE"`,
      `UPLOAD`,
      `sed -i 's|__LOG_BUCKET__|${logsBucketName}|g' /opt/app/upload-logs.sh`,
      `chmod +x /opt/app/upload-logs.sh`,

      // ── logrotate: daily, gzip, copytruncate, ship to S3 ─────────────────────
      // copytruncate truncates the live file in-place so the app/nginx keep
      // writing without needing a reload signal.
      `cat > /etc/logrotate.d/ctech-dfe << 'LOGROTATE'`,
      `/var/log/app/app.log`,
      `/var/log/app/access.log`,
      `/var/log/nginx/access.log`,
      `/var/log/nginx/error.log {`,
      `    daily`,
      `    compress`,
      `    copytruncate`,
      `    missingok`,
      `    notifempty`,
      `    dateext`,
      `    dateformat -%Y%m%d`,
      `    rotate 1`,
      `    sharedscripts`,
      `    postrotate`,
      `        /opt/app/upload-logs.sh`,
      `    endscript`,
      `}`,
      `LOGROTATE`,

      // ── Bootstrap: deploy current.zip if it already exists in S3 ────────────
      `aws s3api head-object --bucket "${deploymentsBucketName}" --key "ctech-dfe/api/current.zip" 2>/dev/null && /opt/app/deploy.sh ctech-dfe/api/current.zip || echo "No bootstrap artifact, waiting for first deploy"`,
    );

    // ── Launch Template ───────────────────────────────────────────────────────
    const instanceProfile = iam.InstanceProfile.fromInstanceProfileName(
      this, 'InstanceProfile', instanceProfileName,
    );

    const launchTemplate = new ec2.LaunchTemplate(this, 'LaunchTemplate', {
      launchTemplateName: `${this.asgName}-lt`,
      instanceType: ec2.InstanceType.of(ec2.InstanceClass.T4G, ec2.InstanceSize.MICRO),
      machineImage: ec2.MachineImage.latestAmazonLinux2023({
        cpuType: ec2.AmazonLinuxCpuType.ARM_64,
        edition: ec2.AmazonLinuxEdition.MINIMAL,
      }),
      blockDevices: [{
        deviceName: '/dev/xvda',
        volume: ec2.BlockDeviceVolume.ebs(3, {
          volumeType: ec2.EbsDeviceVolumeType.GP3,
          deleteOnTermination: true,
        }),
      }],
      userData,
      instanceProfile,
      requireImdsv2: true,
      // securityGroup is passed so CDK can resolve IConnectable for
      // attachToApplicationTargetGroup. The generated SecurityGroupIds property
      // is deleted below and moved into NetworkInterfaces, which is the only
      // place AssociatePublicIpAddress and Ipv6AddressCount can be set.
      securityGroup: apiSecurityGroup,
    });

    const cfnLT = launchTemplate.node.defaultChild as ec2.CfnLaunchTemplate;

    // Move security group from SecurityGroupIds into NetworkInterfaces so we
    // can disable public IPv4 and request one IPv6 address per instance.
    // AWS rejects a launch template that has both fields simultaneously.
    cfnLT.addPropertyDeletionOverride('LaunchTemplateData.SecurityGroupIds');
    cfnLT.addPropertyOverride('LaunchTemplateData.NetworkInterfaces', [{
      DeviceIndex: 0,
      Groups: [apiSecurityGroup.securityGroupId],
      AssociatePublicIpAddress: false,
      Ipv6AddressCount: 1,
    }]);

    // ── Target Group ──────────────────────────────────────────────────────────
    const targetGroup = new elbv2.ApplicationTargetGroup(this, 'TargetGroup', {
      targetGroupName: `${this.asgName}-tg-v2`,
      vpc,
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targetType: elbv2.TargetType.INSTANCE,
      healthCheck: {
        path: '/v1.0/health-check',
        interval: cdk.Duration.seconds(15),
        timeout: cdk.Duration.seconds(5),
        healthyThresholdCount: 2,
        unhealthyThresholdCount: 5,
        healthyHttpCodes: '200,207',
      },
      deregistrationDelay: cdk.Duration.seconds(30),
    });

    // ── Auto Scaling Group ────────────────────────────────────────────────────
    const asg = new autoscaling.AutoScalingGroup(this, 'ASG', {
      autoScalingGroupName: this.asgName,
      vpc,
      vpcSubnets: {subnetType: ec2.SubnetType.PUBLIC},
      launchTemplate,
      minCapacity: 1,
      maxCapacity: isProd ? 3 : 1,
      cooldown: cdk.Duration.seconds(120),
      healthChecks: autoscaling.HealthChecks.withAdditionalChecks({
        additionalTypes: [AdditionalHealthCheckType.ELB],
        gracePeriod: Duration.seconds(120),
      }),
      // healthChecks omitted → defaults to EC2-only.
      // ASG replaces only truly dead instances (VM crash/stop).
      // ALB target group health check handles traffic routing independently.
      // Adding ELB here causes infinite replacement loops before first deploy.
    });

    asg.attachToApplicationTargetGroup(targetGroup);

    // ── ALB Listener Rule ─────────────────────────────────────────────────────
    new elbv2.ApplicationListenerRule(this, 'ListenerRule', {
      listener: httpsListener,
      priority: 10,
      conditions: [
        elbv2.ListenerCondition.hostHeaders([domainName]),
        elbv2.ListenerCondition.pathPatterns(['/*']),
      ],
      action: elbv2.ListenerAction.forward([targetGroup]),
    });

    // ── Outputs ───────────────────────────────────────────────────────────────
    new cdk.CfnOutput(this, 'AsgName', {value: this.asgName, exportName: `${id}-asg-name`});
    new cdk.CfnOutput(this, 'AppLogGroupName', {value: appLogGroup.logGroupName, exportName: `${id}-app-log-group`});
    new cdk.CfnOutput(this, 'NginxLogGroupName', {
      value: nginxLogGroup.logGroupName,
      exportName: `${id}-nginx-log-group`
    });
  }
}
