# CTech DFe — System Overview

**SaaS platform for issuing and managing Electronic Tax Documents (DF-e) with direct SEFAZ communication.**

Supports: NF-e · NFC-e · CT-e · MDF-e

---

## Monorepo Structure

```
ctech-dfe/
├── py-dfe/     # Python Lambda — SEFAZ communication (XML-DSig + SOAP mTLS)
├── api/        # REST backend — Go (Fiber v3), multi-tenant
├── ui/         # SaaS frontend — Next.js + TypeScript + ShadCN
├── worker/     # Async workers — Go Lambda, SQS consumers
└── cdk/        # AWS infrastructure — CDK TypeScript
```

---

## Technology Stack

| Layer     | Technology                                         |
|-----------|----------------------------------------------------|
| Backend   | Go (Fiber v3), aws-sdk-go-v2                       |
| Workers   | Go Lambda (aws-lambda-go), SQS consumers           |
| SEFAZ lib | Python 3.14 Lambda (py-dfe) — XML-DSig + SOAP mTLS |
| Frontend  | Next.js 16, TypeScript, Tailwind CSS 4, ShadCN     |
| Database  | DynamoDB (30 tables) + S3 (certificates and XMLs)  |
| Messaging | SQS (standard) + SNS                               |
| Infra     | AWS CDK TypeScript                                 |
| Auth      | OAuth 2.0 PKCE + RS256 via ctech-account           |

---

## Components

### py-dfe — Core Library

Lambda handler that receives a structured request, loads the A1 certificate (PFX), signs the XML with XML-DSig, and
submits via SOAP to SEFAZ with mTLS.

**Flow:**

```
LambdaRequest → CertificateManager → ServiceClient → SEFAZ SOAP → LambdaResponse
```

**Input (LambdaRequest):**

- `cnpj`, `uf`, `environment` (producao/homologacao)
- `doc_type` (nfe/nfce/cte/mdfe), `service` (e.g. NFeAutorizacao)
- `certificate_b64` (PFX in base64), `certificate_password`
- `body` (dict that becomes XML), `max_retries` (0-10, default 3)

### go-dfe — In-Process Go Migration (New)

In-process Go replacement for py-dfe's SEFAZ SOAP/mTLS calls, adopted operation-by-operation
(`docs/plans/2026-07-17-go-dfe-migration.md`, `MIGRATION.md`). `worker`/`api` call `dfe.Call`
directly (no Lambda Invoke) for any `(docType, service)` in `dfe.Implements()`; everything else still goes through the
py-dfe Lambda — same request/response JSON contract either way. Currently implements unsigned operations only
(status/consulta/distribuição); signed operations stay on py-dfe until the XML-DSig/C14N port passes a byte-identical
gate against captured py-dfe output.

### api — REST Backend

Multi-tenant Go API (Fiber v3). Manages organizations, users, certificates, products, vehicles, persons, and fiscal
document issuance.

**Authentication:** JWT RS256 Bearer validated against JWKS from ctech-account. Org access via `Dfe-Organization-Pk`
header or path param.

**Main endpoints:**

```
GET    /v1.0/auth/me                        # Current user + orgs (sub from JWT)

POST   /v1.0/organizations                  # Create organization (multipart + A1 cert, KYC)
POST   /v1.0/organizations/{pk}/certificates # Upload A1 certificate
POST   /v1.0/organizations/{pk}/invitations  # Invite a user (single-use link)
POST   /v1.0/invitations/{token}/accept      # Accept an invitation
PUT    /v1.0/organizations/{pk}/nfe-config  # Configure NF-e issuance

GET    /v1.0/products                       # List products (paginated)
POST   /v1.0/products                       # Create product
GET    /v1.0/services                       # List NFS-e service catalog (paginated)
PUT    /v1.0/organizations/{pk}/nfse-config # Configure NFS-e issuance
GET    /v1.0/persons                        # List customers/suppliers
POST   /v1.0/nfes                           # Issue NF-e
POST   /v1.0/nfes/{access_key}/cancel       # Cancel NF-e
GET    /v1.0/nfes/{access_key}/xml          # Download XML
```

**AWS Client:** `aws-sdk-go-v2`. Covers DynamoDB, S3, SQS, SNS, Lambda, SecretsManager.

### ui — Frontend

Next.js SPA with JWT authentication, active organization selector, and NF-e issuance forms.

**Pages:** Login · Dashboard · Organizations · NF-e · NFC-e · CT-e · MDF-e · Products · Vehicles · Persons ·
Certificates · Fiscal Configuration

**Theme:** Soft green (`#50ba95` as primary color). Defined in `THEME.md`.

### cdk — Infrastructure

14 CDK TypeScript stacks. Tables prefixed by environment (`dev_`, `staging_`, `prod_`).

**Main resources:** DynamoDB (30 tables) · S3 (2 buckets: certificates + documents) · Lambda (py-dfe, worker) · API
Gateway · IAM (least privilege) · SQS (standard) · SNS · CloudFront

---

## Data Model (DynamoDB Keys)

| Table                       | PK                       | SK               |
|-----------------------------|--------------------------|------------------|
| users                       | USER_{uuid}              | —                |
| organizations               | CNPJ_{cnpj} or CPF_{cpf} | —                |
| organization_users          | {org_pk}                 | USER_{sub}       |
| organization_invitations    | INVITE_{sha256(token)}   | —                |
| organization_products       | {org_pk}                 | PRODUCT_{uuid}   |
| organization_vehicles       | {org_pk}                 | VEHICLE_{id}     |
| organization_persons        | {cpf_cnpj}               | PERSON_{id}      |
| organization_certificates   | {org_pk}                 | CERT_{timestamp} |
| nfes / nfces / ctes / mdfes | {env}#{CNPJ}             | {access_key}     |
| nfe_events / ...            | {access_key}             | {uuidv7}         |
| organization_nfe_configs    | {org_pk}                 | —                |
| organization_services       | {org_pk}                 | SERVICE_{uuid}   |
| organization_nfse_configs   | {org_pk}                 | —                |
| nfses                       | {env}#{CNPJ}             | id_dps           |
| nfse_events                 | {id_dps}                 | {uuidv7}         |

**S3:**

```
certificates/  {org_pk}/{md5}.pfx
documents/     {org_pk}/nfe/{access_key}.xml
               {org_pk}/nfe/{access_key}/events/{event_sk}.xml
```

---

## NF-e Issuance Flow

```
HTTP Client
  → POST /v1.0/nfes  (Go Fiber)
  → Load organization from DynamoDB
  → Download certificate from S3
  → Generate access key (44 digits)
  → One transact_write reserves the fiscal number and creates the document plus an immutable worker_outbox command
  → Return 202 Accepted + operation_id + WebSocket channel to client

DynamoDB Stream → outbox-publisher Lambda → command SNS → SQS (standard, at-least-once)

SQS → worker Lambda (Go)
  → Conditionally claim the document/event with owner + six-minute processing lease
  → Fail closed when the claim store is unavailable; only the owner may finalize
  → Fetch certificate from S3
  → Invoke py-dfe Lambda
      → Sign XML (XML-DSig)
      → Submit via SOAP to SEFAZ (mTLS)
      → Return result
  → Persist NF-e + events in DynamoDB
  → Save XML to S3
  → Publish terminal result to results SNS
  → API results consumer updates state and publishes via Valkey
  → API pushes WebSocket update to client
```

---

## NFS-e Issuance Flow

Same outbox → SNS → SQS spine as NF-e; what changes is everything downstream of the worker.

```
HTTP Client
  → POST /v1.0/nfses  (Go Fiber)
  → Build the id_dps (45 chars) — the access key does NOT exist yet
  → One transact_write reserves the DPS number and creates the document plus the worker_outbox command
  → Return 202 Accepted + operation_id

DynamoDB Stream → outbox-publisher Lambda → command SNS → SQS

SQS → worker Lambda (Go)
  → go-dfe IN-PROCESS (no py-dfe at any point in NFS-e)
      → Sign the DPS (XML-DSig) → gzip+base64 → REST/mTLS to Sefin Nacional
  → Persist the NFS-e row (status authorized/rejected — a rejection is terminal, never retried)
  → Save DPS + NFS-e XML to S3
  → Publish terminal result to results SNS → WebSocket
```

Distribution runs on its own schedule and never touches SOAP `distDFe`:

```
EventBridge scheduler
  → distribution-dispatcher Lambda (sweeps organization_{nfe,cte,mdfe,nfse}_configs)
  → SQS
  → distribution worker
  → go-dfe → ADN REST GET /DFe/{NSU}?lote=true, sequential NSU paging
  → nfse_distributions rows + XML in S3; the cursor advances in organization_nfse_configs.{env}_nsu
```

---

## Security

- **Auth:** OAuth 2.0 Authorization Code + PKCE. ui redirects to accounts.aoctech.app; ctech-account issues RS256 access
  tokens (15m) and opaque refresh tokens (30d).
- **JWT verification:** api validates RS256 tokens by fetching JWKS from `CTECH_JWKS_URL`; keys are cached in Valkey
  (TTL 1h). No HS256, no local `SECRET_KEY`.
- **Token storage (client):** `access_token` in memory only. `refresh_token` in sessionStorage (`pydfe_rt`). Silent
  refresh on 401 via `doRefresh()`.
- **Certificates:** Stored in S3 with AWS Managed Keys; never returned by the API
- **Multi-tenancy:** Every route verifies org membership. Membership lives in `organization_users`
  (the source of truth); RBAC reads it per request (short-TTL cache, invalidated on member changes).
- **KYC:** Creating an organization requires an A1 certificate whose holder document matches the org's CNPJ/CPF — a
  filial (same CNPJ root) inherits the matriz certificate instead.
- **Invitations:** OWNER/ADMIN share a single-use, 7-day link (opaque token; only its SHA-256 is stored) granting
  ADMIN/USER/VIEWER — never OWNER.
- **RBAC:** Roles OWNER / ADMIN / USER / VIEWER (seeded on boot) with `action.resource` permissions; effective
  permission = role ∪ per-member extras.
- **IAM:** Least privilege per function (Lambda, API, Worker)

---

## Environments

| Environment | Table prefix | DynamoDB billing | Removal Policy | PITR |
|-------------|--------------|------------------|----------------|------|
| dev         | dev_         | On-demand        | DESTROY        | No   |
| staging     | staging_     | On-demand        | RETAIN         | No   |
| production  | prod_        | On-demand        | RETAIN         | Yes  |

---

## Local Setup

```bash
# SEFAZ library
cd py-dfe && pip install -e ".[dev]" && pytest tests/unit/

# Backend
cd api && go run ./cmd/server

# Frontend
cd ui && npm install && npm run dev  # http://localhost:3000

# Infrastructure
cd cdk && npm install && cdk synth
```

**Required environment variables (api):** (see `api/.env.example`)

```
PORT=8080
ENVIRONMENT=dev
AWS_REGION=us-east-1
TABLE_PREFIX=dev
S3_BUCKET_CERTIFICATES=dev-ctech-dfe-certificates
S3_BUCKET_DOCUMENTS=dev-ctech-dfe-documents
SEFAZ_FUNCTION_NAME=dev-ctech-dfe
SERVICE_AUDIENCE=https://dfe-api.aoctech.app
CTECH_URL=https://accounts.aoctech.app   # derives CTECH_JWKS_URL automatically
# VALKEY_URL=redis://...                        # JWKS cache (falls back to in-memory)
```

**Required environment variables (ui):**

```
NEXT_PUBLIC_API_URL=http://localhost:8000
NEXT_PUBLIC_CTECH_URL=https://accounts-api.aoctech.app
NEXT_PUBLIC_CTECH_CLIENT_ID=dfe
```

---

## Additional Documentation

| Document             | Contents                           |
|----------------------|------------------------------------|
| `DOCS.md`            | Complete technical reference       |
| `CONDUCT.md`         | Engineering guidelines             |
| `DynamoDB-Tables.md` | Detailed schema for all 30 tables  |
| `DEPLOYMENT.md`      | Infrastructure deployment guide    |
| `INTEGRATION.md`     | Frontend-backend integration guide |
| `THEME.md`           | Color palette and design system    |
