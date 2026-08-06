# py-dfe — DynamoDB Tables Reference

All table names are prefixed by environment: `dev_`, `staging_`, `prod_`.  
Billing: on-demand (max 5 RCU/WCU per table, 10 RCU/WCU per GSI).  
Encryption: AWS Managed Keys.  
PITR: enabled in production only.

---

## Table Index

| #  | Table (without prefix)      | PK                           | SK                                       | GSIs                                               |
|----|-----------------------------|------------------------------|------------------------------------------|----------------------------------------------------|
| 1  | `users`                     | `USER_{uuid}`                | —                                        | `email-index`, `username-index`                    |
| 2  | `organizations`             | `CNPJ_{cnpj}` or `CPF_{cpf}` | —                                        | —                                                  |
| 3  | `organization_certificates` | `{org_pk}`                   | `CERT_{timestamp}`                       | —                                                  |
| 4  | `organization_products`     | `{org_pk}`                   | `PRODUCT_{uuid}`                         | `code-index`, `description-index`                  |
| 5  | `organization_vehicles`     | `{org_pk}`                   | `VEHICLE_{id}`                           | `plate-index`, `role-index`                        |
| 6  | `organization_persons`      | `{org_pk}`                   | `CNPJ_{cnpj}` or `CPF_{cpf}`             | `org-name-index`                                   |
| 7  | `organization_nfe_configs`  | `{org_pk}`                   | —                                        | —                                                  |
| 8  | `organization_nfce_configs` | `{org_pk}`                   | —                                        | —                                                  |
| 9  | `organization_cte_configs`  | `{org_pk}`                   | —                                        | —                                                  |
| 10 | `organization_mdfe_configs` | `{org_pk}`                   | —                                        | —                                                  |
| 11 | `nfes`                      | `{env}#{CNPJ}`               | `{access_key}`                           | `number-index-v2`, `dfe-index`                     |
| 12 | `nfces`                     | `{env}#{CNPJ}`               | `{access_key}`                           | `number-index-v2`, `dfe-index`                     |
| 13 | `ctes`                      | `{env}#{CNPJ}`               | `{access_key}`                           | `number-index-v2`, `dfe-index`                     |
| 14 | `mdfes`                     | `{env}#{CNPJ}`               | `{access_key}`                           | `number-index-v2`, `dfe-index`                     |
| 15 | `nfe_events`                | `{org_pk}`                   | `{uuidv7}`                               | `org-event-key-index`                              |
| 16 | `nfce_events`               | `{org_pk}`                   | `{uuidv7}`                               | `org-event-key-index`                              |
| 17 | `cte_events`                | `{org_pk}`                   | `{uuidv7}`                               | `org-event-key-index`                              |
| 18 | `mdfe_events`               | `{org_pk}`                   | `{uuidv7}`                               | `org-event-key-index`                              |
| 19 | `nfe_distributions`         | `{org_pk}`                   | `nsu` (N)                                | —                                                  |
| 20 | `cte_distributions`         | `{org_pk}`                   | `nsu` (N)                                | —                                                  |
| 21 | `mdfe_distributions`        | `{org_pk}`                   | `nsu` (N)                                | —                                                  |
| 22 | `nfse_distributions`        | `{org_pk}`                   | `nsu` (N)                                | —                                                  |
| 23 | `roles`                     | `ROLE_{NAME}`                | —                                        | —                                                  |
| 24 | `audit_logs`                | `{org_pk}`                   | `{resource_type}#{resource_id}#{uuidv7}` | `org-time-index`, `user-id-index`                  |
| 25 | `organization_users`        | `{org_pk}`                   | `USER_{sub}`                             | `user-index` (inverted)                            |
| 26 | `organization_invitations`  | `INVITE_{sha256(token)}`     | —                                        | `org-invite-index`                                 |
| 27 | `worker_outbox`             | `{table_name}#{access_key}`  | `command`                                | —                                                  |
| 28 | `organization_services`     | `{org_pk}`                   | `SERVICE_{uuid}`                         | `code-index`, `description-index`                  |
| 29 | `organization_nfse_configs` | `{org_pk}`                   | —                                        | —                                                  |
| 30 | `nfses`                     | `{env}#{CNPJ}`               | `id_dps`                                 | `number-index-v2`, `dfe-index`, `access-key-index` |
| 31 | `nfse_events`               | `{id_dps}`                   | `{uuidv7}`                               | `org-event-key-index`                              |

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

One item per org. PK is the tax document number. Only `name`/`cpf_or_cnpj`/1 endereço are required at cadastro (see
`docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md`); a PJ organization additionally requires
`crt` and ≥1 `state_registrations` entry (enforced in
`services.RequirePJFields`/`RequireOrgIE`, not just the frontend) — organizations are always the fiscal emitter, so
these aren't optional the way they can be for a `organization_persons` record.

| Attribute                    | Type | Notes                                                                                                                                                                                                               |
|------------------------------|------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`                         | S    | `CNPJ_{14 digits}` or `CPF_{11 digits}`                                                                                                                                                                             |
| `name`                       | S    | Razão social                                                                                                                                                                                                        |
| `description`                | S    | Apelido interno (optional)                                                                                                                                                                                          |
| `person.fantasy_name`        | S    | Nome fantasia (optional)                                                                                                                                                                                            |
| `person.crt`                 | N    | `1` Simples / `2` Simples c/ excesso / `3` Real / `4` MEI — required for CNPJ                                                                                                                                       |
| `person.state_registrations` | L    | List of `{uf, state_registration}` — ≥1 entry required for CNPJ                                                                                                                                                     |
| `person.addresses`           | L    | List of `{street, number, complement, neighborhood, city, state_federation, postal_code, city_ibge_code}` — min 1                                                                                                   |
| `person.contacts`            | M    | `{emails: [...], phones: [...]}` (optional, max 5 each)                                                                                                                                                             |
| `person.nfse`                | M    | `{im, caepf, nif, c_nao_nif, reg_trib: {op_simp_nac, reg_ap_trib_sn, reg_esp_trib}, foreign_address: {...}}` — NFS-e identity fields, optional, shared verbatim with `organization_persons.person.nfse` (see below) |
| `pickup_locations`           | L    | List of TLocal-shaped saved "local de retirada" (org = remetente), cap 5. See `api/internal/services/nfes/emit.go`, `appendPickupLocation`                                                                          |
| `authorized_xml_viewers`     | L    | List of `{cpf_cnpj, name}` — SEFAZ autXML, cap 10, no duplicate CPF/CNPJ. See `services.OrganizationService.AddAuthorizedViewer`                                                                                    |
| `created_at`                 | S    | ISO-8601 UTC                                                                                                                                                                                                        |
| `updated_at`                 | S    | ISO-8601 UTC                                                                                                                                                                                                        |

---

## 3. `organization_certificates`

A1 certificates for SEFAZ communication. Private key never returned by API.

| Attribute    | Type | Notes                                  |
|--------------|------|----------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key             |
| `sk`         | S    | `CERTIFICATE_{md5}` — sort key         |
| `alias`      | S    | Human-readable label (defaults to CN)  |
| `md5`        | S    | MD5 of the PFX file                    |
| `password`   | S    | PFX password — never returned by API   |
| `s3_key`     | S    | `certs/{org_pk}/{md5}.pfx`             |
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

Fleet registry for CT-e and MDF-e operations. Only `plate`/`plate_uf`/`role` are required at cadastro — everything else
is optional and gated per doc-type/role at emission time (see
`api/internal/services/vehicle_requirements.go`, function `Missing`). Trailers are ordinary rows with `role=trailer` —
not nested under a tractor — so one trailer can be reused across multiple tractors.

| Attribute    | Type | Notes                                                                                                                                                             |
|--------------|------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key                                                                                                                                        |
| `sk`         | S    | `VEHICLE_{id}` — sort key                                                                                                                                         |
| `role`       | S    | `tractor` \| `trailer`. GSI: `role-index`                                                                                                                         |
| `plate`      | S    | Vehicle plate. GSI: `plate-index`                                                                                                                                 |
| `plate_uf`   | S    | UF of plate registration                                                                                                                                          |
| `wheelset`   | S    | Tipo de rodado (MDF-e `tpRod`, tractor only). Optional                                                                                                            |
| `bodywork`   | S    | Tipo de carroceria (MDF-e `tpCar`). Optional                                                                                                                      |
| `renavam`    | S    | RENAVAM (optional)                                                                                                                                                |
| `weight`     | N    | Tare weight in kg (MDF-e `tara`). Optional                                                                                                                        |
| `cap_kg`     | N    | Capacity in kg (MDF-e `capKG`, required-for-emission on trailers). Optional                                                                                       |
| `cap_m3`     | N    | Capacity in m³ (optional)                                                                                                                                         |
| `cint`       | S    | Internal code (optional)                                                                                                                                          |
| `owner`      | M    | `{cpf_cnpj, rntrc, name, type}` — optional fleet metadata only; NOT used for MDF-e's `prop` group (that's a per-emission input, see `MdfeEmitBody.vehicle.owner`) |
| `created_at` | S    | ISO-8601 UTC                                                                                                                                                      |
| `updated_at` | S    | ISO-8601 UTC                                                                                                                                                      |

**GSIs:** `plate-index` (PK: `pk`, SK: `plate`), `role-index` (PK: `pk`, SK: `role`).

---

## 6. `organization_persons`

Customers and suppliers (destinatário/emitente counterparty). PK is the owning org, SK is derived from the person's own
CPF/CNPJ — so uniqueness is per-org, not global (two different orgs may each have a person record for the same
CPF/CNPJ). `Create` rejects a duplicate CPF/CNPJ within the same org with 409
(`ConditionExpression: attribute_not_exists(pk)` on the transact Put — see
`repositories.PersonRepository.BuildCreateTxItem`). Only `name`/`cpf_or_cnpj`/1 endereço are required — unlike
organizations, IE is **not** required here even for a CNPJ, since it's a per-emission choice (`indIEDest`), not a
cadastro requirement.

| Attribute                    | Type | Notes                                                                                                                                                                                                                                            |
|------------------------------|------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`                         | S    | `{org_pk}` — partition key                                                                                                                                                                                                                       |
| `sk`                         | S    | `CNPJ_{14 digits}` or `CPF_{11 digits}` — sort key                                                                                                                                                                                               |
| `name`                       | S    | Full name / razão social. GSI: `org-name-index`                                                                                                                                                                                                  |
| `person.fantasy_name`        | S    | Nome fantasia (optional)                                                                                                                                                                                                                         |
| `person.crt`                 | N    | Required for CNPJ (see `services.RequirePJFields`) — not required to have an IE                                                                                                                                                                  |
| `person.state_registrations` | L    | List of `{uf, state_registration}` (optional)                                                                                                                                                                                                    |
| `person.addresses`           | L    | List of `{street, number, complement, neighborhood, city, state_federation, postal_code, city_ibge_code}` — min 1                                                                                                                                |
| `person.contacts`            | M    | `{emails: [...], phones: [...]}` (optional, max 5 each)                                                                                                                                                                                          |
| `person.nfse`                | M    | `{im, caepf, nif, c_nao_nif, reg_trib: {op_simp_nac, reg_ap_trib_sn, reg_esp_trib}, foreign_address: {...}}` — same shape as `organizations.person.nfse` above; needed when this person is used as prestador/intermediário in a DPS (tpEmit 2/3) |
| `delivery_locations`         | L    | List of TLocal-shaped saved "local de entrega" for NF-e emissions to this destinatário, cap 5. See `appendDeliveryLocation`                                                                                                                      |
| `created_at`                 | S    | ISO-8601 UTC                                                                                                                                                                                                                                     |
| `updated_at`                 | S    | ISO-8601 UTC                                                                                                                                                                                                                                     |

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

## 19–22. `nfe_distributions` / `cte_distributions` / `mdfe_distributions` / `nfse_distributions`

Records received via DFe distribution services (NFeDistribuicaoDFe, DistDFe for CT-e/MDF-e, or ABRASF ADN for NFS-e). SK is numeric NSU for range queries.

| Attribute    | Type | Notes                                                      |
|--------------|------|------------------------------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key                                 |
| `nsu`        | N    | NSU number — sort key                                      |
| `doc_type`   | S    | `nfe`, `cte`, `mdfe`, or `nfse`                            |
| `schema`     | S    | SEFAZ schema type (`resNFe`, `procNFe`, `resEvento`, etc.); for `nfse` — RPS schema version |
| `access_key` | S    | 44-digit key for DFe (NF-e/CT-e/MDF-e); 50-digit for NFS-e |
| `xml_s3_key` | S    | S3 key of the received XML                                 |
| `created_at` | S    | ISO-8601 UTC                                               |

---

## 23. `roles`

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

## 24. `audit_logs`

Per-field change record for org-owned mutating resources (products, vehicles, persons, certificates, organizations,
fiscal configs). DF-e issuance and events do NOT write here — those tables are append-only, so `user_id`/`user_name` are
stamped directly on the record/event instead (see `nfes`/`nfces`/`ctes`/`mdfes` and their `_events` tables above).

Resource and audit-log row are written atomically in one `TransactWriteItems` call, so a mutation can never commit
without its audit trail (or vice versa).

| Attribute       | Type | Notes                                                                                                                                           |
|-----------------|------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`            | S    | `{org_pk}` — the owning organization                                                                                                            |
| `sk`            | S    | `{resource_type}#{resource_id}#{uuidv7}` — sort key                                                                                             |
| `resource_type` | S    | `ORGANIZATION`, `CERTIFICATE`, `PRODUCT`, `VEHICLE`, `PERSON`, `NFE_CONFIG`, `NFCE_CONFIG`, `CTE_CONFIG`, `MDFE_CONFIG`                         |
| `resource_id`   | S    | The resource's own id (e.g. a product's `sk`, a cert's `md5`, a fiscal-config doc-type string, or `org_pk` itself for organization/config rows) |
| `action`        | S    | `CREATE` \| `UPDATE` \| `DELETE`                                                                                                                |
| `modifications` | L    | `[{name, before, after}, ...]` — only fields that actually changed                                                                              |
| `user_id`       | S    | Actor's user id (JWT `sub`), or `SYSTEM` for background actions (e.g. worker auto-creating a supplier during NF-e distribution)                 |
| `user_name`     | S    | Actor's resolved display name, or `"Sistema (Distribuição DFe)"` for `SYSTEM`                                                                   |
| `created_at`    | S    | ISO-8601 UTC                                                                                                                                    |

**GSIs:**

| Index            | PK        | SK           | Use case                                                                                                          |
|------------------|-----------|--------------|-------------------------------------------------------------------------------------------------------------------|
| `org-time-index` | `pk`      | `created_at` | Org-wide chronological feed (default view)                                                                        |
| `user-id-index`  | `user_id` | `created_at` | "Everything user X did" — post-filtered to the caller's org since this GSI's partition key is `user_id`, not `pk` |

The base table itself (`pk` + `sk` prefix `{resource_type}#{resource_id}#`) answers "full change history of this one
resource" without needing a GSI.

---

## 25. `organization_users`

**Source of truth for user↔organization membership.** Read on every authorized request (RBAC), so its read-capacity cap
is set high (500 RCU) unlike the 5-RCU default of other tables. Replaces the legacy embedded `users.organizations` list,
which no longer carries authorization.

| Attribute     | Type | Notes                                                                                                                                              |
|---------------|------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`          | S    | `{org_pk}` — the organization                                                                                                                      |
| `sk`          | S    | `USER_{sub}` — the member (ctech-account `sub`)                                                                                                    |
| `user_id`     | S    | `{sub}` raw (avoids re-parsing the sk)                                                                                                             |
| `name`        | S    | Display-only name snapshot taken at grant time. **Never synced** with ctech-account — it only spares the members screen from rendering a bare UUID |
| `role`        | S    | `OWNER` \| `ADMIN` \| `USER` \| `VIEWER`                                                                                                           |
| `permissions` | L    | **Extra** grants beyond the role (usually `[]`). Effective perms = role.permissions ∪ this                                                         |
| `invited_by`  | S    | `sub` of the inviter; empty for the founding OWNER                                                                                                 |
| `created_at`  | S    | ISO-8601 UTC                                                                                                                                       |
| `updated_at`  | S    | ISO-8601 UTC                                                                                                                                       |

**GSIs:**

| Index        | PK   | SK   | Use case                                                                        |
|--------------|------|------|---------------------------------------------------------------------------------|
| `user-index` | `sk` | `pk` | Inverted index — every org a user belongs to (`/auth/me`, `GET /organizations`) |

Access patterns: RBAC → `get_item(pk=org, sk=USER_{sub})` (strong); member list → `query(pk=org, sk begins_with USER_)`;
user's orgs → `query_gsi(user-index, sk=USER_{sub})`.

---

## 26. `organization_invitations`

Single-use invitation links. Partition key is the SHA-256 of the opaque token, so acceptance is a strongly-consistent
`get_item` (never a Scan). The raw token exists only in the returned link.

| Attribute     | Type | Notes                                                                 |
|---------------|------|-----------------------------------------------------------------------|
| `pk`          | S    | `INVITE_{sha256hex(token)}`                                           |
| `org_pk`      | S    | Target organization. GSI: `org-invite-index`                          |
| `role`        | S    | `ADMIN` \| `USER` \| `VIEWER` — **never `OWNER`**                     |
| `permissions` | L    | Extra grants applied on accept (usually `[]`)                         |
| `status`      | S    | `PENDING` \| `ACCEPTED` \| `REVOKED`                                  |
| `invited_by`  | S    | `sub` of the inviter                                                  |
| `accepted_by` | S    | `sub` of the acceptor (set on accept)                                 |
| `expires_at`  | S    | ISO-8601 UTC (now + 7d) — checked in code                             |
| `ttl`         | N    | Epoch seconds (now + 7d + 48h slack) — DynamoDB TTL housekeeping only |
| `created_at`  | S    | ISO-8601 UTC                                                          |
| `updated_at`  | S    | ISO-8601 UTC                                                          |

**GSIs:**

| Index              | PK       | SK           | Use case                                                        |
|--------------------|----------|--------------|-----------------------------------------------------------------|
| `org-invite-index` | `org_pk` | `created_at` | List an org's invitations (newest first), filtered to `PENDING` |

Uniqueness/expiry are enforced by a `ConditionExpression` (`status = PENDING AND ttl > now`) inside the accept
`TransactWriteItems`, not by the TTL sweep.

---

## 27. `worker_outbox`

One immutable issuance command per fiscal operation. The API creates this row in the same `TransactWriteItems` as the
fiscal document and number reservation.

| Attribute        | Type | Notes                                               |
|------------------|------|-----------------------------------------------------|
| `pk`             | S    | Operation ID: `{table_name}#{access_key}`           |
| `sk`             | S    | Constant `command`                                  |
| `status`         | S    | `pending` → `published`                             |
| `payload`        | S    | Exact JSON `WorkerMessage` published to command SNS |
| `created_at`     | S    | ISO-8601 UTC                                        |
| `published_at`   | S    | ISO-8601 UTC, set after SNS accepts the message     |
| `sns_message_id` | S    | SNS acknowledgement identity                        |
| `ttl`            | N    | Epoch seconds, 30 days after creation               |

The API put is create-only. A `NEW_IMAGE` DynamoDB Stream invokes
`outbox-publisher`, which publishes pending rows and conditionally changes only
`pending` to `published`. Failed invocation is retried by the stream event source; downstream workers remain idempotent
because SNS publication and acknowledgement cannot be made atomic.

---

## 28. `organization_services`

Service catalog per org (NFS-e line items — analogous to `organization_products` for goods). Persists
`api/internal/api/v1.ServiceBody` (see `dto.go`) as a dynamic map, same pattern as
`organization_products`/`ProductBody`.

| Attribute             | Type | Notes                                                                                                          |
|-----------------------|------|----------------------------------------------------------------------------------------------------------------|
| `pk`                  | S    | `{org_pk}` — partition key                                                                                     |
| `sk`                  | S    | `SERVICE_{uuid}` — sort key                                                                                    |
| `code`                | S    | Internal service code. GSI: `code-index`                                                                       |
| `description`         | S    | GSI: `description-index`                                                                                       |
| `trib_nacional_code`  | S    | 6-digit código de tributação nacional (Anexo B) — validated against `go-dfe/nfse/tables`                       |
| `trib_municipal_code` | S    | Optional municipal-specific code (max 20 chars)                                                                |
| `nbs_code`            | S    | Optional 9-digit NBS code (Anexo B), validated against `go-dfe/nfse/tables`                                    |
| `cnae`                | S    | Optional 7-digit CNAE                                                                                          |
| `unit`                | S    | Unit of measure                                                                                                |
| `value`               | S    | Unit price as decimal string                                                                                   |
| `iss`                 | M    | `{trib_issqn, tax_rate, tp_ret_issqn?, tp_imunidade?, c_pais_resultado?}` — DPS `tribMun` defaults             |
| `federal`             | M    | Optional `{cst_pis_cofins?, aliq_pis?, aliq_cofins?, tp_ret_pis_cofins?, v_ret_cp?, v_ret_irrf?, v_ret_csll?}` |
| `ibs_cbs`             | M    | Optional `{c_ind_op?, cst?, c_class_trib?, ind_dest?, tp_oper?, fin_nfse?}` — reforma tributária defaults      |
| `tot_trib`            | M    | `{ind_tot_trib, p_tot_trib_sn?}` — Lei da Transparência                                                        |
| `created_at`          | S    | ISO-8601 UTC                                                                                                   |
| `updated_at`          | S    | ISO-8601 UTC                                                                                                   |

**GSIs:** `description-index` (PK: `pk`, SK: `description`), `code-index` (PK: `pk`, SK: `code`).

---

## 29. `organization_nfse_configs`

Fiscal configuration per org for NFS-e issuance. PK only (no SK), same base shape as the other
`organization_*_configs` tables (#7–10), with numbering counters (per `NfseConfigBody`) and ADN distribution
cursor fields (for tracking the last consumed NSU during ADN polling).

| Attribute             | Type | Notes                                                                                           |
|-----------------------|------|-------------------------------------------------------------------------------------------------|
| `pk`                  | S    | `{org_pk}` — partition key                                                                      |
| `provider`            | S    | `nacional` or `abrasf204`                                                                       |
| `environment`         | N    | `1` produção / `2` homologação                                                                  |
| `c_loc_emi`           | S    | 7-digit IBGE code of the local de emissão (município do prestador)                              |
| `serie`               | S    | Document series, up to 5 digits                                                                 |
| `prod_current_number` | N    | Next production DPS number; preserved across `Upsert` (never zeroed)                            |
| `hom_current_number`  | N    | Next homologação DPS number; preserved across `Upsert`                                          |
| `prod_nsu`            | N    | Last consumed NSU in production ADN distribution; preserved across `Upsert`                     |
| `hom_nsu`             | N    | Last consumed NSU in homologação ADN distribution; preserved across `Upsert`                    |
| `prod_last_dist_nsu_at` | S  | ISO-8601 UTC timestamp of last production NSU cursor update; used for rate-limiting              |
| `hom_last_dist_nsu_at`  | S  | ISO-8601 UTC timestamp of last homologação NSU cursor update; used for rate-limiting             |
| `certificate_sk`      | S    | Optional: `organization_certificates` SK to use for this provider                               |
| `abrasf`              | M    | Only for `provider=abrasf204`: `{endpoint_url, wsdl_version, municipality_code, synchronous}` |
| `updated_at`          | S    | ISO-8601 UTC                                                                                    |

Inscrição municipal and the prestador's regime tributário are NOT stored here — they live on the organization's own
`person.nfse` group (see `organizations`/`organization_persons` above), since when the org emits as
tomador/intermediário (`tpEmit` 2/3) the prestador is a different person from the cadastro, not the organization itself.

---

## 30. `nfses`

One item per issued NFS-e. Reuses the same `getDfeTable` shape as `nfes`/`nfces`/`ctes`/`mdfes`
(`number-index-v2` + `dfe-index`), plus an extra GSI for lookup by access key.

| Attribute    | Type | Notes                                                      |
|--------------|------|------------------------------------------------------------|
| `pk`         | S    | `{env}#{CNPJ}` — partition key (`producao#12345678901234`) |
| `sk`         | S    | `id_dps` — sort key                                        |
| `access_key` | S    | 50-digit access key. GSI: `access-key-index`               |
| `org_pk`     | S    | Org's DynamoDB PK (`CNPJ_...`)                             |
| `number`     | N    | Document number. GSI: `number-index-v2`                    |
| `status`     | S    | `authorized`, `rejected`, `pending`, `cancelled`, `failed` |
| `incoming`   | N    | `0` = outgoing, `1` = incoming. Used in `dfe-index`        |
| `year`       | N    | Issue year. Used in `dfe-index`                            |
| `month`      | N    | Issue month (1–12). Used in `dfe-index`                    |
| `day`        | N    | Issue day (1–31). Used in `dfe-index`                      |
| `created_at` | S    | ISO-8601 UTC                                               |
| `updated_at` | S    | ISO-8601 UTC                                               |

Written by `NfseService.Emit` in the same `TransactWrite` that reserves the DPS number:

| Attribute          | Type | Notes                                                                                      |
|--------------------|------|--------------------------------------------------------------------------------------------|
| `provider`         | S    | `nacional` or `abrasf204` — copied from the org's NFS-e config at issuance time             |
| `tp_emit`          | N    | `1` prestador / `2` tomador / `3` intermediário (DPS `tpEmit`)                              |
| `c_motivo_emis_ti` | N    | Only when `tp_emit != 1` (DPS `cMotivoEmisTI`)                                              |
| `serie`            | S    | DPS series, from the config                                                                 |
| `competence`       | S    | `AAAA-MM-DD` competence date (DPS `dCompet`)                                                 |
| `c_loc_emi`        | S    | 7-digit IBGE code of the local de emissão                                                    |
| `emit_cpf_cnpj`    | S    | Issuer document, PK prefix stripped                                                          |
| `emit_name`        | S    | Issuer name                                                                                  |
| `dest_cpf_cnpj`    | S    | Tomador document (absent for self-issuance) — same attribute names as `nfes`/`nfces`         |
| `dest_name`        | S    | Tomador name                                                                                 |
| `total`            | S    | `vServPrest.vServ` as decimal string                                                          |
| `payload`          | M    | The full neutral `nfse.Document` as issued — the same object sent in the worker command       |
| `operation_id`     | S    | `{table}#{id_dps}` — the `worker_outbox` PK of the command committed with this item           |
| `user_id`          | S    | Acting user                                                                                  |
| `user_name`        | S    | Acting user's display name                                                                   |

`access_key` is deliberately absent on creation: writing it empty would pollute `access-key-index`. The worker adds it
with the fisco response.

The sort key is `id_dps`, not the access key, because the 50-digit NFS-e access key only exists after the
SEFAZ/municipal fisco response — unlike the other DF-e, the DPS is submitted before the key is known. `access_key` is
populated once the response arrives and is looked up via
`access-key-index` (PK: `pk`, SK: `access_key`).

---

## 31. `nfse_events`

SEFAZ/municipal communication events for an NFS-e. Reuses `getEventsTable`, but is keyed by the document's `id_dps`
rather than `org_pk`, since events can arrive before the access key exists.

| Attribute    | Type | Notes                                                   |
|--------------|------|---------------------------------------------------------|
| `pk`         | S    | `{id_dps}` — partition key                              |
| `sk`         | S    | `{uuidv7}` — sort key (time-sortable, unique per event) |
| `event_type` | S    | Event type code                                         |
| `event_key`  | S    | GSI sort key on `org-event-key-index`                   |
| `status`     | S    | `authorized`, `rejected`, `pending`, `failed`           |
| `created_at` | S    | ISO-8601 UTC                                            |
| `updated_at` | S    | ISO-8601 UTC                                            |

---

## Access Pattern Reference

| Operation                           | Method           | Table / GSI                            |
|-------------------------------------|------------------|----------------------------------------|
| Login by email                      | `query_gsi`      | `users` / `email-index`                |
| Get org by CNPJ/CPF                 | `get_item`       | `organizations`                        |
| List products (paginated)           | `query`          | `organization_products`                |
| Search products by code             | `query_gsi`      | `organization_products` / `code-index` |
| Get NF-e by access key              | `get_item`       | `nfes`                                 |
| List NF-e by date range             | `query_gsi`      | `nfes` / `dfe-index`                   |
| List events for a document          | `query_gsi`      | `nfe_events` / `org-event-key-index`   |
| Get NF-e fiscal config              | `get_item`       | `organization_nfe_configs`             |
| Increment NF-e numbering            | `transact_write` | `nfes` + `organization_nfe_configs`    |
| List distribution records           | `query`          | `nfe_distributions`                    |
| Audit trail for one resource        | `query`          | `audit_logs` (sk prefix)               |
| Org-wide audit feed                 | `query_gsi`      | `audit_logs` / `org-time-index`        |
| Everything a user did               | `query_gsi`      | `audit_logs` / `user-id-index`         |
| Audit resource + its log atomically | `transact_write` | resource table + `audit_logs`          |
| Persist document command atomically | `transact_write` | document/config + `worker_outbox`      |
