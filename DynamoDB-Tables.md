# py-dfe — DynamoDB Tables Reference

All table names are prefixed by environment: `dev_`, `staging_`, `prod_`.  
Billing: on-demand (max 5 RCU/WCU per table, 10 RCU/WCU per GSI).  
Encryption: AWS Managed Keys.  
PITR: enabled in production only.

---

## Table Index

| #  | Table (without prefix)      | PK                           | SK                 | GSIs                              |
|----|-----------------------------|------------------------------|--------------------|-----------------------------------|
| 1  | `users`                     | `USER_{uuid}`                | —                  | `email-index`, `username-index`   |
| 2  | `organizations`             | `CNPJ_{cnpj}` or `CPF_{cpf}` | —                  | —                                 |
| 3  | `organization_certificates` | `{org_pk}`                   | `CERT_{timestamp}` | —                                 |
| 4  | `organization_products`     | `{org_pk}`                   | `PRODUCT_{uuid}`   | `code-index`, `description-index` |
| 5  | `organization_vehicles`     | `{org_pk}`                   | `VEHICLE_{id}`     | `plate-index`                     |
| 6  | `organization_persons`      | `{cpf_cnpj}`                 | `PERSON_{id}`      | `org-name-index`                  |
| 7  | `organization_nfe_configs`  | `{org_pk}`                   | —                  | —                                 |
| 8  | `organization_nfce_configs` | `{org_pk}`                   | —                  | —                                 |
| 9  | `organization_cte_configs`  | `{org_pk}`                   | —                  | —                                 |
| 10 | `organization_mdfe_configs` | `{org_pk}`                   | —                  | —                                 |
| 11 | `nfes`                      | `{env}#{CNPJ}`               | `{access_key}`     | `number-index-v2`, `dfe-index`    |
| 12 | `nfces`                     | `{env}#{CNPJ}`               | `{access_key}`     | `number-index-v2`, `dfe-index`    |
| 13 | `ctes`                      | `{env}#{CNPJ}`               | `{access_key}`     | `number-index-v2`, `dfe-index`    |
| 14 | `mdfes`                     | `{env}#{CNPJ}`               | `{access_key}`     | `number-index-v2`, `dfe-index`    |
| 15 | `nfe_events`                | `{org_pk}`                   | `{uuidv7}`         | `org-event-key-index`             |
| 16 | `nfce_events`               | `{org_pk}`                   | `{uuidv7}`         | `org-event-key-index`             |
| 17 | `cte_events`                | `{org_pk}`                   | `{uuidv7}`         | `org-event-key-index`             |
| 18 | `mdfe_events`               | `{org_pk}`                   | `{uuidv7}`         | `org-event-key-index`             |
| 19 | `nfe_distributions`         | `{org_pk}`                   | `nsu` (N)          | —                                 |
| 20 | `cte_distributions`         | `{org_pk}`                   | `nsu` (N)          | —                                 |
| 21 | `mdfe_distributions`        | `{org_pk}`                   | `nsu` (N)          | —                                 |
| 22 | `roles`                     | `{role_name}`                | —                  | —                                 |
| 23 | `audit_logs`                | `{org_pk}`                   | `{resource_type}#{resource_id}#{uuidv7}` | `org-time-index`, `user-id-index` |

---

## 1. `users`

Stores user identity. Membership data (`organizations`) embeds org PKs, role, and permissions.

| Attribute         | Type | Notes                                                               |
|-------------------|------|---------------------------------------------------------------------|
| `pk`              | S    | `USER_{uuid}` — partition key                                       |
| `email`           | S    | Lowercase. GSI: `email-index`                                       |
| `username`        | S    | Lowercase. GSI: `username-index`                                    |
| `hashed_password` | S    | Argon2id. Present for local-auth users; absent for OAuth-only users |
| `first_name`      | S    |                                                                     |
| `last_name`       | S    |                                                                     |
| `email_verified`  | BOOL |                                                                     |
| `is_enabled`      | BOOL |                                                                     |
| `last_login_at`   | S    | ISO-8601 UTC                                                        |
| `organizations`   | L    | `[{pk, role, permissions: []}]` — org membership list               |
| `created_at`      | S    | ISO-8601 UTC                                                        |
| `updated_at`      | S    | ISO-8601 UTC                                                        |

**GSIs:**

| Index            | PK         | Sort key | Use case                       |
|------------------|------------|----------|--------------------------------|
| `email-index`    | `email`    | —        | Login by email, password reset |
| `username-index` | `username` | —        | Lookup by username             |

---

## 2. `organizations`

One item per org. PK is the tax document number.

| Attribute      | Type | Notes                                                   |
|----------------|------|---------------------------------------------------------|
| `pk`           | S    | `CNPJ_{14 digits}` or `CPF_{11 digits}`                 |
| `name`         | S    | Razão social                                            |
| `fantasy_name` | S    | Nome fantasia                                           |
| `email`        | S    |                                                         |
| `phone`        | S    |                                                         |
| `address`      | M    | `{street, number, complement, district, city, uf, zip}` |
| `crt`          | S    | `1` Simples / `2` Presumido / `3` Real                  |
| `ie`           | S    | Inscrição Estadual                                      |
| `im`           | S    | Inscrição Municipal (optional)                          |
| `created_at`   | S    | ISO-8601 UTC                                            |
| `updated_at`   | S    | ISO-8601 UTC                                            |

---

## 3. `organization_certificates`

A1 certificates for SEFAZ communication. Private key never returned by API.

| Attribute    | Type | Notes                                  |
|--------------|------|----------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key             |
| `sk`         | S    | `CERT_{iso_timestamp}` — sort key      |
| `alias`      | S    | Human-readable label                   |
| `md5`        | S    | MD5 of the PFX file                    |
| `s3_key`     | S    | `certificates/{org_pk}/{md5}.pfx`      |
| `expires_at` | S    | ISO-8601 (from certificate's NotAfter) |
| `created_at` | S    | ISO-8601 UTC                           |

---

## 4. `organization_products`

Product catalog per org. Includes ICMS/IBS-CBS tax config per CFOP.

| Attribute            | Type | Notes                                                         |
|----------------------|------|---------------------------------------------------------------|
| `pk`                 | S    | `{org_pk}` — partition key                                    |
| `sk`                 | S    | `PRODUCT_{uuid}` — sort key                                   |
| `code`               | S    | Internal product code. GSI: `code-index`                      |
| `description`        | S    | GSI: `description-index`                                      |
| `ncm`                | S    | 8-digit NCM code                                              |
| `origin`             | S    | ICMS origin code (0–8)                                        |
| `unit`               | S    | Unit of measure (UN, KG, etc.)                                |
| `value`              | S    | Unit price as decimal string                                  |
| `cfop_config`        | L    | List of `{cfop, icms, pis, cofins, ibs_cbs_cst, ...}` objects |
| `icms_aliq_override` | S    | Optional: overrides UF rate table                             |
| `fcp_aliq_override`  | S    | Optional: overrides FCP rate                                  |
| `prod_type`          | S    | `comb` (fuel) or `med` (medicine) — optional                  |
| `created_at`         | S    | ISO-8601 UTC                                                  |
| `updated_at`         | S    | ISO-8601 UTC                                                  |

**GSIs:** `code-index` (PK: `pk`, SK: `code`), `description-index` (PK: `pk`, SK: `description`).

---

## 5. `organization_vehicles`

Fleet registry for CT-e and MDF-e operations.

| Attribute    | Type | Notes                             |
|--------------|------|-----------------------------------|
| `pk`         | S    | `{org_pk}` — partition key        |
| `sk`         | S    | `VEHICLE_{id}` — sort key         |
| `plate`      | S    | Vehicle plate. GSI: `plate-index` |
| `renavam`    | S    | RENAVAM (optional)                |
| `rntrc`      | S    | RNTRC for freight (optional)      |
| `tara`       | N    | Tare weight in kg (optional)      |
| `created_at` | S    | ISO-8601 UTC                      |
| `updated_at` | S    | ISO-8601 UTC                      |

**GSI:** `plate-index` (PK: `pk`, SK: `plate`).

---

## 6. `organization_persons`

Customers and suppliers.

| Attribute    | Type | Notes                                           |
|--------------|------|-------------------------------------------------|
| `pk`         | S    | `{cpf_cnpj}` (raw digits) — partition key       |
| `sk`         | S    | `PERSON_{id}` — sort key                        |
| `org_pk`     | S    | Owner org PK                                    |
| `name`       | S    | Full name / razão social. GSI: `org-name-index` |
| `email`      | S    |                                                 |
| `phone`      | S    |                                                 |
| `ie`         | S    | Inscrição Estadual (optional)                   |
| `address`    | M    | Same structure as organizations                 |
| `created_at` | S    | ISO-8601 UTC                                    |
| `updated_at` | S    | ISO-8601 UTC                                    |

**GSI:** `org-name-index` (PK: `pk`, SK: `name`).

---

## 7–10. `organization_{nfe,nfce,cte,mdfe}_configs`

Fiscal configuration per org per document type. PK only (no SK). Table names use prefix `organization_`.

| Attribute                 | Type | Notes                                                  |
|---------------------------|------|--------------------------------------------------------|
| `pk`                      | S    | `{org_pk}` — partition key                             |
| `serie`                   | N    | Document series (e.g. 1)                               |
| `next_number`             | N    | Next document number; incremented via `transact_write` |
| `environment`             | S    | `producao` or `homologacao`                            |
| `csc_token`               | S    | NFC-e only: CSC token for QR Code HMAC                 |
| `csc_id`                  | S    | NFC-e only: CSC identifier                             |
| `nsu`                     | N    | Last NSU fetched (distributions)                       |
| `last_dist_nsu_at`        | S    | ISO-8601 UTC — last distNSU call                       |
| `improper_usage_until`    | S    | ISO-8601 UTC — consumo indevido block expiry           |
| `cons_quota_calls`        | N    | Rolling consNSU/consChNFe counter (reset hourly)       |
| `cons_quota_window_start` | S    | ISO-8601 UTC — start of current 1-hour quota window    |
| `updated_at`              | S    | ISO-8601 UTC                                           |

---

## 11–14. `nfes` / `nfces` / `ctes` / `mdfes`

One item per issued fiscal document. PK encodes environment + issuer CNPJ for org-scoped access.

| Attribute      | Type | Notes                                                      |
|----------------|------|------------------------------------------------------------|
| `pk`           | S    | `{env}#{CNPJ}` — partition key (`producao#12345678901234`) |
| `sk`           | S    | 44-digit access key — sort key                             |
| `org_pk`       | S    | Org's DynamoDB PK (`CNPJ_...`)                             |
| `number`       | N    | Document number. GSI: `number-index-v2`                    |
| `serie`        | N    | Series                                                     |
| `status`       | S    | `authorized`, `rejected`, `pending`, `cancelled`, `failed` |
| `incoming`     | N    | `0` = outgoing, `1` = incoming. Used in `dfe-index`        |
| `year`         | N    | Issue year. Used in `dfe-index`                            |
| `month`        | N    | Issue month (1–12). Used in `dfe-index`                    |
| `day`          | N    | Issue day (1–31). Used in `dfe-index`                      |
| `sefaz_status` | S    | SEFAZ cStat code (e.g. `100`)                              |
| `sefaz_motive` | S    | SEFAZ xMotivo description                                  |
| `xml_s3_key`   | S    | S3 key of the authorized XML                               |
| `created_at`   | S    | ISO-8601 UTC                                               |
| `updated_at`   | S    | ISO-8601 UTC                                               |

**GSIs:**

| Index             | PK   | Sort keys                          | Use case                                 |
|-------------------|------|------------------------------------|------------------------------------------|
| `number-index-v2` | `pk` | `number`, `incoming`               | List by number, filter incoming/outgoing |
| `dfe-index`       | `pk` | `incoming`, `year`, `month`, `day` | List by date range, filter incoming      |

---

## 15–18. `nfe_events` / `nfce_events` / `cte_events` / `mdfe_events`

SEFAZ communication events for a document (authorization, cancellation, CC-e, manifestation, etc.).

| Attribute         | Type | Notes                                                        |
|-------------------|------|--------------------------------------------------------------|
| `pk`              | S    | `{org_pk}` — partition key                                   |
| `sk`              | S    | `{uuidv7}` — sort key (time-sortable, unique per event)      |
| `access_key`      | S    | 44-digit key of the parent document                          |
| `event_key`       | S    | `{access_key}#{event_type}#{seq:03d}` — GSI sort key         |
| `event_type`      | S    | SEFAZ event type code (`110111` cancel, `110110` CC-e, etc.) |
| `sequence_number` | N    | Event sequence (1-based, per type)                           |
| `status`          | S    | `authorized`, `rejected`, `pending`, `failed`                |
| `sefaz_status`    | S    | SEFAZ cStat                                                  |
| `sefaz_motive`    | S    | SEFAZ xMotivo                                                |
| `xml_s3_key`      | S    | S3 key of the event XML                                      |
| `created_at`      | S    | ISO-8601 UTC                                                 |
| `updated_at`      | S    | ISO-8601 UTC                                                 |

**GSI `org-event-key-index`** (PK: `pk`, SK: `event_key`):

```python
# All events for a document
begins_with(event_key, f"{access_key}#")

# All events of a specific type
begins_with(event_key, f"{access_key}#{event_type}#")

# Exact event (type + sequence)
event_key == f"{access_key}#{event_type}#001"
```

---

## 19–21. `nfe_distributions` / `cte_distributions` / `mdfe_distributions`

Records received via NFeDistribuicaoDFe (`distNSU`). SK is numeric NSU for range queries.

| Attribute    | Type | Notes                                                      |
|--------------|------|------------------------------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key                                 |
| `nsu`        | N    | NSU number — sort key                                      |
| `doc_type`   | S    | `nfe`, `cte`, `mdfe`                                       |
| `schema`     | S    | SEFAZ schema type (`resNFe`, `procNFe`, `resEvento`, etc.) |
| `access_key` | S    | 44-digit key (present for `procNFe` / `resNFe`)            |
| `xml_s3_key` | S    | S3 key of the received XML                                 |
| `created_at` | S    | ISO-8601 UTC                                               |

---

## 22. `roles`

Pre-seeded RBAC role definitions. Items are written once at bootstrap.

| Attribute     | Type | Notes                                                |
|---------------|------|------------------------------------------------------|
| `pk`          | S    | Role name: `OWNER`, `ADMIN`, `USER`, `VIEWER`        |
| `permissions` | L    | `["list.organization_products", "create.nfes", ...]` |
| `description` | S    | Human-readable description                           |

**Permission format:** `{action}.{resource}`  
Actions: `list`, `get`, `create`, `update`, `delete`  
Resources: DynamoDB table names without prefix (e.g. `organization_products`, `nfes`, `organizations`)

---

## 23. `audit_logs`

Per-field change record for org-owned mutating resources (products, vehicles, persons,
certificates, organizations, fiscal configs). DF-e issuance and events do NOT write here — those
tables are append-only, so `user_id`/`user_name` are stamped directly on the record/event instead
(see `nfes`/`nfces`/`ctes`/`mdfes` and their `_events` tables above).

Resource and audit-log row are written atomically in one `TransactWriteItems` call, so a mutation
can never commit without its audit trail (or vice versa).

| Attribute       | Type | Notes                                                                 |
|-----------------|------|------------------------------------------------------------------------|
| `pk`            | S    | `{org_pk}` — the owning organization                                  |
| `sk`            | S    | `{resource_type}#{resource_id}#{uuidv7}` — sort key                   |
| `resource_type` | S    | `ORGANIZATION`, `CERTIFICATE`, `PRODUCT`, `VEHICLE`, `PERSON`, `NFE_CONFIG`, `NFCE_CONFIG`, `CTE_CONFIG`, `MDFE_CONFIG` |
| `resource_id`   | S    | The resource's own id (e.g. a product's `sk`, a cert's `md5`, a fiscal-config doc-type string, or `org_pk` itself for organization/config rows) |
| `action`        | S    | `CREATE` \| `UPDATE` \| `DELETE`                                       |
| `modifications` | L    | `[{name, before, after}, ...]` — only fields that actually changed     |
| `user_id`       | S    | Actor's user id (JWT `sub`), or `SYSTEM` for background actions (e.g. worker auto-creating a supplier during NF-e distribution) |
| `user_name`     | S    | Actor's resolved display name, or `"Sistema (Distribuição DFe)"` for `SYSTEM` |
| `created_at`    | S    | ISO-8601 UTC                                                           |

**GSIs:**

| Index            | PK        | SK           | Use case                                                    |
|------------------|-----------|--------------|--------------------------------------------------------------|
| `org-time-index` | `pk`      | `created_at` | Org-wide chronological feed (default view)                  |
| `user-id-index`  | `user_id` | `created_at` | "Everything user X did" — post-filtered to the caller's org since this GSI's partition key is `user_id`, not `pk` |

The base table itself (`pk` + `sk` prefix `{resource_type}#{resource_id}#`) answers "full change
history of this one resource" without needing a GSI.

---

## Access Pattern Reference

| Operation                  | Method           | Table / GSI                            |
|----------------------------|------------------|----------------------------------------|
| Login by email             | `query_gsi`      | `users` / `email-index`                |
| Get org by CNPJ/CPF        | `get_item`       | `organizations`                        |
| List products (paginated)  | `query`          | `organization_products`                |
| Search products by code    | `query_gsi`      | `organization_products` / `code-index` |
| Get NF-e by access key     | `get_item`       | `nfes`                                 |
| List NF-e by date range    | `query_gsi`      | `nfes` / `dfe-index`                   |
| List events for a document | `query_gsi`      | `nfe_events` / `org-event-key-index`   |
| Get NF-e fiscal config     | `get_item`       | `organization_nfe_configs`             |
| Increment NF-e numbering   | `transact_write` | `nfes` + `organization_nfe_configs`    |
| List distribution records  | `query`          | `nfe_distributions`                    |
| Audit trail for one resource | `query`        | `audit_logs` (sk prefix)               |
| Org-wide audit feed        | `query_gsi`      | `audit_logs` / `org-time-index`        |
| Everything a user did      | `query_gsi`      | `audit_logs` / `user-id-index`         |
| Audit resource + its log atomically | `transact_write` | resource table + `audit_logs` |
