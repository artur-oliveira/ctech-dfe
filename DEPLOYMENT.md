# CDK Deployment — py-dfe

For infrastructure architecture, stacks, and environment details, see:

* `DOCS.md §7` — Infrastructure Architecture
* `DOCS.md §11` — Deployment and Operations

---

# Infrastructure Overview

The API runs as a Go/Fiber binary (`app`) on an EC2 Auto Scaling Group reached through the shared CTech HAProxy edge —
see `cdk/lib/api-stack.ts`. The \bootstrap route parameter
`/ctech/{env}/lbalancer/routes/dfe` is currently owned by `ctech-lbalancer`, not
the DFE stack; ownership must be transferred explicitly before a future shared
service construct creates it here.
Fiscal issuance runs asynchronously through an outbox, SNS, standard SQS, Lambda workers
(`cdk/lib/worker-stack.ts`), in-process go-dfe operations, and the py-dfe fallback Lambda
(`cdk/lib/dfe-stack.ts`).

```text
AWS Account
│
├── DynamoDB
├── S3
├── Lambda
├── API Infrastructure
├── EC2 Auto Scaling Group
├── HAProxy route registration (shared ctech-lbalancer)
├── CloudWatch
└── Systems Manager (SSM)
```

---

# Prerequisites

Install dependencies:

```bash
cd cdk && npm install
```

Create or refresh the URL parameters before deploying the API. Run this from the
adjacent `ctech-cdk` repository; it is idempotent and uses the private hosted-zone
names for EC2-to-EC2 traffic:

```bash
CTECH_AWS_PROFILE=ctech ./scripts/configure-service-url-parameters.sh prod
```

The public account application URL remains the OIDC issuer/browser URL. Only
service transport and JWKS retrieval use `*.internal.aoctech.app`.

## Out-of-band parameters

Secrets are never created by CDK. CloudFormation cannot create an SSM
`SecureString` at all, and a secret Terraform or CDK owns is a secret that a
later `deploy` quietly reverts to the value it was rotated away from. The stacks
grant read access; the values are written once by hand.

| Parameter                                 | Type         | Env var                  | Value                                                                                             |
|-------------------------------------------|--------------|--------------------------|---------------------------------------------------------------------------------------------------|
| `/ctech-dfe/{env}/billing/webhook-secret` | SecureString | `BILLING_WEBHOOK_SECRET` | The same secret ctech-billing's seed was given as `WEBHOOK_SECRET_DFE` for the `whe_dfe` endpoint |
| `/ctech-dfe/{env}/billing/client-id`      | SecureString | `BILLING_CLIENT_ID`      | `dfe-billing` — the client-credentials client ctech-account issued for calling billing            |
| `/ctech-dfe/{env}/billing/client-secret`  | SecureString | `BILLING_CLIENT_SECRET`  | Its secret, shown once at issue                                                                   |

`BILLING_API_URL` is *not* on this list: it is a hostname, not a secret, and it
comes from `/ctech-billing/{env}/internal-base-url`, written by ctech-cdk's
`configure-service-url-parameters.sh` along with every other private service
endpoint. In prod it resolves to `https://billing.internal.aoctech.app`.

```bash
read -rs -p 'webhook secret: ' SECRET   # typed, not echoed, and not in history
aws ssm put-parameter --profile ctech --region us-east-1 \
  --name /ctech-dfe/prod/billing/webhook-secret \
  --type SecureString --overwrite --value "$SECRET"
unset SECRET
```

Both sides must hold the identical webhook secret: billing signs
`timestamp + "." + body` with it, the API recomputes the HMAC and compares. A
mismatch is indistinguishable from a forged request, so every delivery is
rejected and billing disables the endpoint after 12 consecutive failures.

Absent, the API runs in **no-charge mode**: every account is unlimited and the
boot log says so at WARN. That is a supported deployment — it is what a dev
environment needs — so a production instance missing these parameters starts
successfully and must be caught by reading the log, not by a crash.

The webhook secret is separate: without it the client still works, but
`POST /v1/internal/webhooks/billing` is **not mounted at all**. A signature check
that cannot run is not a signature check, so the route 404s rather than trusting
what arrives, and subscription changes then wait on the 60-second snapshot TTL
instead of arriving.

## AWS Credentials

Configure one of the following methods:

```bash
# Option A: AWS CLI
aws configure

# Option B: Environment variables
export AWS_ACCESS_KEY_ID="..."
export AWS_DEFAULT_REGION="us-east-1"

# Option C: Named profile
export AWS_PROFILE="your-profile"
```

---

# Bootstrap

Run once per AWS account and region:

```bash
cdk bootstrap aws://868899309401/us-east-1
```

---

# Deployment

## Preview Changes

Always synthesize before deployment:

```bash
cdk synth
```

## Deploy All Stacks

```bash
cdk deploy --all
```

## Deploy a Specific Stack

```bash
cdk deploy PyDfeStack/PyDfeStack-dynamodb
```

## CI/CD Deployment

```bash
cdk deploy --all --require-approval never
```

---

# Environment Deployments

## Production

```bash
ENVIRONMENT=prod npx cdk deploy --all
```

## Staging

```bash
ENVIRONMENT=stage npx cdk deploy --all
```

---

# Post-Deployment Verification

List CloudFormation stacks:

```bash
aws cloudformation list-stacks --region us-east-1 \
  --query 'StackSummaries[?StackStatus!=`DELETE_COMPLETE`].[StackName,StackStatus]' \
  --output table
```

List DynamoDB tables:

```bash
aws dynamodb list-tables --region us-east-1
```

Inspect a specific table:

```bash
aws dynamodb describe-table \
  --table-name dev_users \
  --region us-east-1
```

---

## Verify durable issuance dispatch

After deployment, verify the outbox stream and publisher mapping before accepting fiscal traffic:

```bash
aws dynamodb describe-table --table-name "${TABLE_PREFIX}_dfe_worker_outbox" --region us-east-1
aws lambda list-event-source-mappings --function-name "${ENVIRONMENT}-dfe-outbox-publisher" --region us-east-1
```

The table must expose a `NEW_IMAGE` stream, the event-source mapping must be enabled, and the publisher DLQ alarm must
be `OK`. Main worker queue visibility is derived from each Lambda timeout as six times the timeout plus the five-minute
maximum batching window; do not replace it with a fixed value below the function execution budget.

# Destroy

## WARNING

Development environments may use:

```text
RemovalPolicy.DESTROY
```

Destroying stacks may permanently delete data.

Remove all stacks:

```bash
cdk destroy --all
```

Without confirmation:

```bash
cdk destroy --all --force
```

Production and staging environments use:

```text
RemovalPolicy.RETAIN
```

DynamoDB tables remain after stack deletion.

---

# EC2 Instance Operations (ApiStackV2)

Instances do not have public IPv4 addresses.

All access must occur through:

```text
AWS Systems Manager Session Manager (SSM)
```

---

## Connect to an Instance

List Auto Scaling Group instances:

```bash
aws ec2 describe-instances \
  --filters "Name=tag:aws:autoscaling:groupName,Values=${ENV}-api-v2" \
  --query "Reservations[].Instances[].{Id:InstanceId,State:State.Name,IP:PrivateIpAddress}" \
  --output table
```

Start a shell session:

```bash
aws ssm start-session --target i-XXXXXXXXXXXXXXXXX
```

---

## Check Service Status

```bash
sudo systemctl status app
sudo systemctl status nginx
sudo systemctl status amazon-ssm-agent
sudo systemctl status amazon-cloudwatch-agent
```

Where:

* `app` = the Go/Fiber binary (`app`), managed by systemd
* `nginx` = Reverse proxy

---

## Real-Time Log Analysis

Application logs:

```bash
sudo journalctl -u app -f
```

Nginx logs:

```bash
sudo journalctl -u nginx -f
```

Direct log files:

```bash
sudo tail -f /var/log/app/app.log
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log
```

Cloud-init output:

```bash
sudo cat /var/log/cloud-init-output.log
```

Last 100 log lines:

```bash
sudo journalctl -u app --no-pager -n 100
```

---

## CloudWatch Log Analysis

Application logs:

```bash
aws logs tail /py-dfe/prod/app --follow
```

Nginx logs:

```bash
aws logs tail /py-dfe/prod/nginx \
  --follow \
  --log-stream-name-prefix i-XXXXX
```

Filter 5XX responses:

```bash
aws logs filter-log-events \
  --log-group-name /py-dfe/prod/nginx \
  --filter-pattern '{ $.status >= 500 }' \
  --start-time $(date -d '1 hour ago' +%s000)
```

---

## Archived Logs in S3

List archived files:

```bash
aws s3 ls s3://prod-py-dfe-logs/api/ --recursive | grep 20260603
```

Download and extract:

```bash
aws s3 cp \
  s3://prod-py-dfe-logs/api/20260603-i-XXXXX.tar.gz \
  /tmp/

tar xzf /tmp/20260603-i-XXXXX.tar.gz -C /tmp/logs/
```

---

## Manual Deployment Through SSM

```bash
ENV=prod
ASG="${ENV}-api-v2"
ARTIFACT="api/api-20260603-1200-main-abc1234.zip"

COMMAND_ID=$(aws ssm send-command \
  --targets "Key=tag:aws:autoscaling:groupName,Values=${ASG}" \
  --document-name "AWS-RunShellScript" \
  --parameters "commands=[\"/opt/app/deploy.sh ${ARTIFACT}\"]" \
  --timeout-seconds 300 \
  --query "Command.CommandId" \
  --output text)
```

Monitor execution:

```bash
aws ssm list-command-invocations \
  --command-id "$COMMAND_ID" \
  --details \
  --query "CommandInvocations[].{Instance:InstanceId,Status:Status,Output:CommandPlugins[0].Output}" \
  --output table
```

---

## Target Group Health Checks

Get Target Group ARN:

```bash
TG_ARN=$(aws elbv2 describe-target-groups \
  --names "${ENV}-api-v2-tg" \
  --query "TargetGroups[0].TargetGroupArn" \
  --output text)
```

Check target health:

```bash
aws elbv2 describe-target-health \
  --target-group-arn "$TG_ARN" \
  --query "TargetHealthDescriptions[].{Id:Target.Id,State:TargetHealth.State,Reason:TargetHealth.Reason}" \
  --output table
```

---

## Quick Diagnosis: Unhealthy New Instance

1. Check cloud-init output:

```bash
sudo cat /var/log/cloud-init-output.log | tail -50
```

2. Verify SSM Agent:

```bash
systemctl status amazon-ssm-agent
```

3. Verify application deployment:

```bash
ls -la /opt/app/current/
```

4. Verify the app service:

```bash
systemctl status app
```

5. Verify local health endpoint:

```bash
curl -s http://localhost:8080/v1.0/health-check
```

---

# Troubleshooting

| Error                                          | Cause                                                               | Resolution                                                      |
|------------------------------------------------|---------------------------------------------------------------------|-----------------------------------------------------------------|
| `No credentials have been configured`          | Missing AWS credentials                                             | Run `aws configure` or set environment variables                |
| `InvalidClientTokenId`                         | Expired credentials                                                 | Regenerate credentials in AWS Console                           |
| `Access Denied`                                | Missing IAM permissions                                             | Grant CloudFormation and DynamoDB permissions                   |
| `Account 868899309401 is not available`        | Wrong AWS account                                                   | Verify `bin/cdk.ts`                                             |
| `Bootstrap required`                           | CDK bootstrap not executed                                          | Run the bootstrap command                                       |
| `iamInstanceProfile.arn is invalid`            | Instance profile not created yet                                    | Verify IAM stack deployment completed successfully              |
| `Cannot exceed quota for PolicySize: 6144`     | Managed policy over the IAM 6 KB limit                              | Grant by ARN prefix wildcard, not one ARN per table              |
| `Export ... cannot be deleted as it is in use` | Stack still imports an export the producer just dropped             | Deploy the consumer stack alone first (`--exclusively`)          |
| `Volume of size XGB is smaller than snapshot`  | EBS volume smaller than AMI snapshot requirements                   | Use a larger volume or AL2023 Minimal                           |
| `SSM agent offline`                            | AL2023 Minimal does not include SSM Agent by default                | Install and enable `amazon-ssm-agent`                           |
| Instances continuously replaced                | ASG configured with ELB health checks before application deployment | Use EC2 health checks during bootstrap                          |
| `ln -sfn` creates a symlink inside a directory | `/opt/app/current` already exists as a directory                    | Use `ln -sfT` and avoid creating `current/` beforehand          |
| `AccessDenied` during `aws s3 ls`              | Missing `s3:ListBucket` permission                                  | Use `aws s3api head-object` if only `s3:GetObject` is available |

```
```
