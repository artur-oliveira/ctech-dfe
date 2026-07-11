# PyDFe CDK Infrastructure

AWS Cloud Development Kit (CDK) for deploying PyDFe infrastructure.

## Stack Components

### DynamoDB Stack
- **users**: User accounts (PK: USER_<uuid>)
  - GSI: email-index, username-index
- **organizations**: Organization data (PK: CPF_<cpf> or CNPJ_<cnpj>)
- **organization_certificates**: A1 digital certificates (PK: org_pk, SK: CERTIFICATE_<md5>)
- **organization_products**: Product catalog (PK: org_pk, SK: PRODUCT_<uuid>)
- **organization_vehicles**: Vehicle fleet (PK: org_pk, SK: VEHICLE_<uuid>)
- **organization_nfe_configs**: NF-e configurations
- **organization_nfce_configs**: NFC-e configurations
- **organization_cte_configs**: CT-e configurations
- **organization_mdfe_configs**: MDF-e configurations

### S3 Stack
- **Certificates Bucket**: A1 digital certificates storage
- **Documents Bucket**: Generated documents (NF-e, NFC-e, CT-e, MDF-e)

### IAM Stack
- **Lambda Execution Role**: Permissions for Lambda functions
- **API Execution Role**: Permissions for API instances
- DynamoDB read/write access to all tables
- S3 access to certificates and documents buckets

## Prerequisites

```bash
npm install -g aws-cdk
npm install
```

## Configuration

Set environment variables:
```bash
export AWS_ACCOUNT=868899309401
export AWS_REGION=us-east-1
export ENVIRONMENT=dev                # dev, staging, production
export TABLE_PREFIX=dev               # Table prefix
```

## Commands

### Synthesize CloudFormation template
```bash
npm run build
cdk synth
```

### Deploy stack
```bash
cdk deploy PyDfeStack
```

### List stacks
```bash
cdk list
```

### Destroy stack
```bash
cdk destroy PyDfeStack
```

## Environment Setup

### Development
```bash
export ENVIRONMENT=dev
export TABLE_PREFIX=dev
cdk deploy --context environment=dev
```

### Staging
```bash
export ENVIRONMENT=staging
export TABLE_PREFIX=staging
cdk deploy --context environment=staging
```

### Production
```bash
export ENVIRONMENT=production
export TABLE_PREFIX=prod
cdk deploy --context environment=production
```

## Architecture Notes

- DynamoDB: Pay-per-request billing (dev), provisioned with auto-scaling (production)
- Point-in-time recovery enabled for production
- S3 buckets with encryption enabled
- All DynamoDB GSIs for optimized queries
- IAM roles follow least privilege principle

## CloudFormation Outputs

After deployment, outputs include:
- DynamoDB table names
- S3 bucket names
- IAM role ARNs

Access outputs:
```bash
aws cloudformation describe-stacks --stack-name PyDfeStack --query 'Stacks[0].Outputs'
```

## AWS Account Details

- **Account ID**: 868899309401
- **Region**: us-east-1
- **Service**: py-dfe (Plataforma de Emissão de Documentos Fiscais Eletrônicos)
