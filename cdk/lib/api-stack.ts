import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as logs from 'aws-cdk-lib/aws-logs';
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
      `    # Written by /opt/app/update-realip.sh: set_real_ip_from for the ALB and for`,
      `    # CloudFront's origin-facing ranges, so $remote_addr below is the real viewer`,
      `    # IP and not the proxy's. The glob keeps nginx bootable if the file is absent.`,
      `    include /etc/nginx/conf.d/realip*.conf;`,
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
      `    #`,
      `    # $binary_remote_addr is the viewer's IP, not the ALB's, only because the`,
      `    # realip module rewrote it (see the include above). Without that the whole`,
      `    # req_by_ip zone collapses onto the ALB's private IP and the rate becomes a`,
      `    # shared ceiling for every client at once.`,
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
      // Overwrite rather than append: $proxy_add_x_forwarded_for would carry through
      // whatever X-Forwarded-For the client sent, and the Go app trusts the leftmost
      // entry. $remote_addr is the realip-resolved viewer IP, which a client cannot forge.
      `            proxy_set_header X-Forwarded-For $remote_addr;`,
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
      // Overwrite rather than append: $proxy_add_x_forwarded_for would carry through
      // whatever X-Forwarded-For the client sent, and the Go app trusts the leftmost
      // entry. $remote_addr is the realip-resolved viewer IP, which a client cannot forge.
      `            proxy_set_header X-Forwarded-For $remote_addr;`,
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
      // Overwrite rather than append: $proxy_add_x_forwarded_for would carry through
      // whatever X-Forwarded-For the client sent, and the Go app trusts the leftmost
      // entry. $remote_addr is the realip-resolved viewer IP, which a client cannot forge.
      `            proxy_set_header X-Forwarded-For $remote_addr;`,
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
    );

    addRealipRefreshCommands(userData, vpc.vpcCidrBlock);

    userData.addCommands(
      'systemctl enable nginx',
      'systemctl start nginx',
    );

    addCloudWatchAgentDualStackOverride(userData);

    userData.addCommands(
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
      `VALKEY_URL=$(aws ssm get-parameter --name "${valkeyUrlSsmPath}" --with-decryption --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CTECH_JWKS_URL=$(aws ssm get-parameter --name "${accountInternalJwksUrlParameter}" --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CTECH_URL=$(aws ssm get-parameter --name "${accountInternalBaseUrlParameter}" --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CTECH_ISSUER_URL=$(aws ssm get-parameter --name "${accountIssuerUrlParameter}" --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `SERVICE_AUDIENCE=$(aws ssm get-parameter --name "${appUrlParameter}" --query Parameter.Value --output text --region us-east-1 2>/dev/null || echo "")`,
      `CORS_ALLOWED_ORIGINS="$SERVICE_AUDIENCE"`,
      `export CTECH_JWKS_URL CTECH_URL VALKEY_URL CTECH_ISSUER_URL SERVICE_AUDIENCE CORS_ALLOWED_ORIGINS`,
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
