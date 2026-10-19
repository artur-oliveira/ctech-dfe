# CLAUDE.md — cdk

AWS CDK infrastructure — TypeScript, `aws-cdk-lib` v2 (resolved 2.268; see `package.json`), shared constructs from `@aoctech/cdk`.

**CTech standards:** infra rules (no NAT, no ALB, no CloudWatch alarms or custom metrics, alerts via the shared SNS
topic, CDK frozen / Terraform for new infra) live in the `ctech-dev-standards` skill (`references/infra.md`); it wins
over this file. This file only records what is specific to ctech-dfe.

**Before any task:** Read `../OVERVIEW.md`, `../CONDUCT.md`, `../DOCS.md §8`, `../DEPLOYMENT.md`.

---

## Role

Defines and deploys the DFE-owned AWS infrastructure: DynamoDB tables, S3 buckets, the Go worker Lambdas and the
`go-dfe-egress` Lambda, the API EC2 ASG, standard SQS, SNS, IAM and the GitHub OIDC roles. The VPC, the shared edge
security group, Valkey and the deployment/log buckets belong to `ctech-cdk` (read from SSM); ingress is the HAProxy of
`ctech-lbalancer`. There is **no ALB, no CloudFront and no network stack** here; the frontend is a static export on
Cloudflare Workers Static Assets, deployed by `.github/workflows/frontend.yml` (see `../ui/CLAUDE.md`).

---

## Directory Structure

```
cdk/
├── bin/ctech-dfe-cdk.ts        # app entry point: stack wiring, account/region, env vars
├── lib/
│   ├── dynamodb-stack.ts       # DynamoDB tables + GSIs (logical names in the TableName union)
│   ├── s3-stack.ts             # 2 buckets: certificates, documents
│   ├── event-bus-stack.ts      # command/results SNS, results + erasure SQS (+ DLQs)
│   ├── worker-stack.ts         # worker Lambdas, SQS + DLQ, DLQ processor, outbox publisher, distribution dispatcher
│   ├── worker-definitions.ts   # WORKERS table: one entry per worker Lambda
│   ├── egress-stack.ts / egress.ts  # go-dfe-egress Lambda (sa-east-1)
│   ├── api-stack.ts            # API EC2 ASG via HaproxyEc2Service (no ALB target group)
│   ├── iam-stack.ts            # Lambda + EC2 IAM roles
│   ├── oidc-stack.ts           # GitHub Actions OIDC deploy roles (global)
│   ├── dfe-stack.ts            # py-dfe Lambda + layer (retired, still deployed; see below)
│   └── go-code.ts, types.ts    # Go bundling helper, Environment type
└── test/                       # jest tests, one per stack (py-dfe-cdk.test.ts is the empty cdk init stub)
```

---

## Mandatory Workflow

1. Read relevant docs before starting.
2. `rg "..."` — search for existing stack definitions before creating new resources.
3. `npx cdk synth` before any deploy to verify template generation.
4. Plan → Implement → `cdk synth` → deploy to dev first.
5. Update `../DOCS.md §8` for new stacks/resources; `../DynamoDB-Tables.md` for table changes.
6. State cross-project impact (cdk ↔ api ↔ worker ↔ go-dfe-egress ↔ ui).
7. Suggest Conventional Commit.

---

## Engineering Rules

- **Do not duplicate stack constructs**; reuse `@aoctech/cdk` and the builders already in `dynamodb-stack.ts`
  (`getDfeTable`, `getEventsTable`, ...). Names derive from the environment prefix, never hard-coded.
- Environment is `dev | stage | prod` (`lib/types.ts`), from the `ENVIRONMENT` env var; stack ids `CtechDfe-{Env}-{Name}`.
- Never run `cdk destroy` without explicit environment confirmation from the user.
- **Existing DynamoDB tables are never recreated**; do not change their logical ids (standard: infra.md).
- IAM least privilege per role; EC2 instance profile lives in `iam-stack.ts`, never inline in the ASG stack.
- For every new resource write the cost note (billing model, volume, TTL/lifecycle).
- No secrets in the repo; the only account id that may appear is `868899309401`.

### Environment rules (as coded)

| Environment | DynamoDB removal | DynamoDB PITR | S3 / log-group removal | Table prefix |
|-------------|------------------|---------------|------------------------|--------------|
| `dev`       | `DESTROY`        | No            | `DESTROY`              | `dev_dfe_`   |
| `stage`     | `RETAIN`         | No            | `DESTROY`              | `stage_dfe_` |
| `prod`      | `RETAIN`         | Yes           | `RETAIN`               | `prod_dfe_`  |

(`dynamodb-stack.ts:289-290`, `s3-stack.ts`, `api-stack.ts`). Note that stage S3 buckets and log groups are `DESTROY`.

---

## Testing

Run `npm test` from `cdk/` (jest, `ts-jest`) and `npx cdk synth` before proposing a deploy. IAM changes: review the
synthesized policy by hand. Table schema changes: update `../DynamoDB-Tables.md`.

---

## Deployment

```bash
cd cdk && npm install
npx cdk synth                                   # always verify first
ENVIRONMENT=dev cdk deploy --all                # all stacks (dev)
ENVIRONMENT=prod cdk deploy CtechDfe-Prod-API-V2   # one stack (prod)
```

**AWS account:** `868899309401` · **Region:** `us-east-1` (except `GoDfeEgressStack`, `sa-east-1`; `cdk bootstrap` it once).
CI deploys through `.github/workflows/deploy.yml`; see `../DEPLOYMENT.md`.

---

## Known Constraints

- `ApiStack` is the only API stack. It uses `HaproxyEc2Service`: `minCapacity: 1`, `maxCapacity: 2`, On-Demand,
  `t4g.nano` arm64. `ctech-lbalancer` owns the route and private CNAME; the retained SSM param
  `/ctech/{env}/network/alb-sg-id` only identifies the shared edge security group (the name is historical, no ALB exists).
- Worker Lambda binary must be named `bootstrap` (`provided.al2023`, arm64). API binary must be named `app`
  (userdata runs `/opt/app/current/app`).
- **py-dfe is retired** (no worker or API calls it; they call the egress Lambda), but `DfeStack`, its IAM role and
  invoke policy (`iam-stack.ts`), the `ctech-dfe-gha-pydfe` OIDC role and the `py-dfe/layer` path filter in
  `deploy.yml` are still deployed. Removing them is a separate reviewed change; this doc set only records the state.
- Queues are standard SQS; each worker queue has a DLQ (`maxReceiveCount: 3`). **No CloudWatch alarm exists on any
  queue** (removed); the `*-ops-alerts` SNS topics are kept only because subscriptions were added outside
  CloudFormation. Family policy is alerts through `ctech-{env}-alerts`; no DFE component uses go-common `alerts` yet (nothing under `api/` or `worker/` imports it).
- At-least-once delivery, no ordering: idempotency is enforced in the application.
- Worker keep-warm: EventBridge Scheduler pings each worker every **1 minute** (`worker-stack.ts`; the comment in the
  file still says 5 minutes, the code wins).
- The DynamoDB IAM policy grants `Scan` (used by the distribution dispatcher, see `../worker/CLAUDE.md`).

---

## Critical Areas (require analysis before touching)

- DynamoDB table definitions (schema changes are destructive without migration)
- IAM roles, `RemovalPolicy` on any resource
- `ApiStack` ASG and HAProxy route registration (rolling deploy, health check)
- SQS + DLQ configuration
- Cross-region reference to the egress Lambda (built from name/ARN strings, never stack tokens)

Before touching: identify blast radius, verify environment, confirm with user for production.

---

## Completion Checklist

- [ ] `npx cdk synth` succeeds cleanly; `npm test` passes
- [ ] No magic strings; names derived from the env prefix
- [ ] `RemovalPolicy.DESTROY` only where the table above says so
- [ ] IAM least-privilege; cost note written
- [ ] `../DOCS.md §8` and/or `../DynamoDB-Tables.md` updated
- [ ] Cross-project impact reviewed (cdk ↔ api ↔ worker ↔ go-dfe-egress ↔ ui)

Every change that alters behavior, a contract or an operational rule updates the matching docs in the same change.
