# CLAUDE.md — cdk

AWS CDK infrastructure — TypeScript, CDK v2.257.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §8`, `../DEPLOYMENT.md`.

---

## Role

Defines and deploys all AWS infrastructure: DynamoDB tables, S3 buckets, Lambda functions,
EC2 ASG (API), ALB, CloudFront, SQS FIFO, SNS, IAM roles, VPC, and CloudWatch.

---

## Directory Structure

```
cdk/
├── bin/                        # CDK app entry point
├── lib/
│   ├── dynamodb-stack.ts       # 23 DynamoDB tables + GSIs
│   ├── s3-stack.ts             # 4 S3 buckets
│   ├── network-stack.ts        # VPC, subnets, security groups
│   ├── iam-stack.ts            # Lambda + EC2 IAM roles
│   ├── dfe-stack.ts            # py-dfe Lambda
│   ├── worker-stack.ts         # Worker Lambdas + SQS FIFO + DLQ
│   ├── api-v2-stack.ts         # EC2 ASG + ALB target group
│   ├── frontend-stack.ts       # S3 + CloudFront
│   └── ...                     # Other stacks (see DOCS.md §8)
└── test/
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing stack definitions before creating new resources.
3. `cdk synth` before any deploy to verify template generation.
4. Plan → Implement → `cdk synth` → deploy to dev first.
5. Update `../DOCS.md §8` for new stacks/resources; `../DynamoDB-Tables.md` for table changes.
6. State cross-project impact (cdk ↔ api ↔ worker ↔ py-dfe ↔ ui).
7. Suggest Conventional Commit.

---

## Engineering Rules

### DRY

- Never duplicate stack constructs. If multiple stacks share a pattern (e.g., Lambda with SQS),
  extract a shared construct class.
- Before adding a new stack or resource, check existing stacks for similar patterns.
- Table name construction (`${env}_tableName`) must use a single helper — never repeated inline.

### Constants — no magic strings

- Environment prefix (`dev`, `staging`, `prod`) must flow from a single config/context variable.
- Table names MUST be constructed as `${prefix}_${tableName}` — never hardcoded full names.
- Lambda function names, S3 bucket names, and IAM role names follow the same pattern.
- Stack-to-stack references use CDK exports/imports — never hardcoded ARNs.

### Environment Rules (critical)

| Environment | Removal Policy    | PITR   | Table Prefix |
|-------------|-------------------|--------|--------------|
| dev         | `DESTROY`         | No     | `dev_`       |
| staging     | `RETAIN`          | No     | `staging_`   |
| production  | `RETAIN`          | Yes    | `prod_`      |

- `RemovalPolicy.DESTROY` is **dev-only**. Never set it for staging or production.
- **Never run `cdk destroy` without explicit environment confirmation from the user.**
- PITR (Point-in-Time Recovery) enabled only in staging/production.

### IAM — least privilege

- Every Lambda and EC2 role must have the minimum permissions required — no wildcards.
- Review IAM diffs carefully before deploying to production.
- Instance profiles for EC2 are defined in `iam-stack.ts` — never inline in ASG stacks.

### Network

- VPC is dual-stack IPv4 + IPv6 — no NAT Gateway (cost optimization).
- EC2 instances use public IPv6 (free) and no public IPv4.
- S3 and DynamoDB use Gateway VPC Endpoints (free, keeps traffic inside AWS).

### Cost Awareness

For every new resource, document:
- Billing model (on-demand, provisioned, reserved)
- Expected call/storage volume
- Whether a lifecycle policy or TTL is needed

### Secrets

Never commit: AWS credentials, real account IDs beyond `868899309401`, certificate content.

---

## Testing

| Change              | Required                            |
|---------------------|-------------------------------------|
| New stack/construct | CDK snapshot test (`jest`)          |
| Table schema change | Update `../DynamoDB-Tables.md`      |
| IAM change          | Manual review of synthesized policy |
| Deploy              | `cdk synth` must succeed cleanly    |

Run: `npm test` from `cdk/`. Always run `cdk synth` before proposing a deploy.

---

## Deployment

```bash
cd cdk && npm install
cdk synth                              # Always verify first
cdk deploy --all                       # All stacks (dev)
ENVIRONMENT=prod cdk deploy StackName  # Specific stack (prod)
```

**AWS Account:** `868899309401` · **Region:** `us-east-1`

See `../DEPLOYMENT.md` for step-by-step procedures and diagnostics.

---

## Known Constraints

- `ApiStack` (Elastic Beanstalk) is legacy — migration to `ApiStackV2` (EC2 ASG) is in progress.
- `ApiStackV2` ASG uses combined EC2 + ELB health checks with `gracePeriod: 120s`.
- Worker Lambda binary must be named `bootstrap` (runtime: `provided.al2023`).
- API binary must be named `app` (EC2 userdata expects `/opt/app/current/app`).
- CloudFront has Brazil geo-restriction on the `FrontendStack`.

---

## Critical Areas (require analysis before touching)

- DynamoDB table definitions (schema changes are destructive without migration)
- IAM roles (least privilege — over-permissioning is a security risk)
- `ApiStackV2` ASG and ALB configuration (rolling deploy, health check)
- `RemovalPolicy` on any resource
- SQS FIFO + DLQ configuration (at-least-once delivery, ordering)

Before touching: identify blast radius, verify environment, confirm with user for production.

---

## Completion Checklist

- [ ] `cdk synth` succeeds cleanly
- [ ] `npm test` passes (snapshot tests)
- [ ] No magic strings — all names derived from env prefix
- [ ] `RemovalPolicy.DESTROY` only in dev
- [ ] IAM permissions are least-privilege
- [ ] New resources documented in `../DOCS.md §8` and/or `../DynamoDB-Tables.md`
- [ ] Cost impact assessed
- [ ] Cross-project impact reviewed (cdk ↔ api ↔ worker ↔ py-dfe ↔ ui)
