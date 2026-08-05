# py-dfe — Brazilian DFe Full-Stack Roadmap

## Context

The platform currently has **production-grade NF-e support** across all stacks. The SEFAZ communication layer (`py-dfe`
Lambda) already supports NF-e, NFC-e, CT-e and MDF-e fully. Infrastructure (DynamoDB tables, S3 buckets, CDK stacks, SQS
queues) is provisioned for these 4 types.

**Gap summary:**

- NFC-e, CT-e, MDF-e → API service/route layer and frontend are missing.
- BPe, NF3e, NFCom, NFF, DCe, NFag, NFGas → greenfield in all layers.

**Reuse rule:** Every new document type follows the NF-e pattern exactly:
`py-dfe enums+endpoints+service` → `api repo→service→routes+schemas` →
`worker handler+DLQ` → `ui list+emit+detail` → `cdk tables+queues+workers+IAM`

---

## Milestone Map

| Phase | Milestone              | Documents         | Status      |
|-------|------------------------|-------------------|-------------|
| 0     | Polish & Observability | NF-e gaps         | In progress |
| 1     | Core Four — Part A     | NFC-e             | Scaffolded  |
| 2     | Core Four — Part B     | CT-e              | Scaffolded  |
| 3     | Core Four — Part C     | MDF-e             | Scaffolded  |
| 4     | Advanced Wave 1        | BPe, NF3e         | Not started |
| 5     | Advanced Wave 2        | NFCom, NFF, NFGas | Not started |
| 6     | Advanced Wave 3        | DCe, NFag         | Not started |

---

## Phase 0 — Polish & Observability (NF-e gaps)

> Close known NF-e gaps before expanding to other document types.

### Back-end (`api`)

- [ ] ~~**0.1** `POST /v1.0/nfes/{access_key}/inutilization` — number range inutilization~~
    - ~~Service: `NfeService.inutilization(org, serie, num_ini, num_fin, justification)`~~
    - ~~Reuse existing `nfe-inutilization-worker` (already in CDK WorkerStack)~~
    - ~~Files: `app/api/nfes.py`, `app/services/nfe_service.py`~~
    - This task requires a new research because nfe inutilization does not have a DynamoDB dable and its schema does not
      match either nfes or nfe_events

- [ ] **0.2** `POST /v1.0/nfes/{access_key}/correction-letter` — Carta de Correção (CC-e)
    - Event type `110110`; reuse `RecepcaoEvento` pipeline
    - Schema: `CCeIn` (access_key, correction text 15–1000 chars)
    - Files: `app/schemas/nfes.py`, `app/services/nfe_service.py`

- [ ] ~~**0.3** `GET /v1.0/nfes/{access_key}/danfe` — DANFE PDF generation~~
    - ~~Evaluate `reportlab` vs `weasyprint` for Lambda cold-start impact~~
    - ~~ABNT NBR 15895 layout; return binary PDF with `Content-Disposition: attachment`~~
    - ~~Files: new `app/services/danfe_service.py`, `app/api/nfes.py`~~
    - There is already a route to perform a DANFE PDF (using external service), not priority.

- [ ] **0.4** `GET /v1.0/distributions/nfe` — distribution DFe consumer
    - Calls `NFeDistribuicaoDFe` SEFAZ service; saves each NSU/XML received to `nfe_distributions` table
    - Should persit the NF-e (procNFe) 1 → dest == organization, 2 → transporta == organization
    - Should persit related events (resEvento | procEventoNFe) at nfe_events
    - Should always trigger internal NF-e manifestation (210210) for "resNFe" received for distribution
    - Should notify the frontend using websocket when receives a new procNFe
    - Files: new `app/api/distributions.py`, `app/services/nfe_distribution_service.py`

- [ ] **0.5** `POST /v1.0/nfes/{access_key}/manifestation` — incoming document events
    - Event types: `210200` (confirmação), `210210` (ciência), `210220` (desconhecimento), `210240` (não realização)
    - Schema: `NFeManifestIn` with `event_type` enum
    - Files: `app/api/nfes.py`, `app/services/nfe_service.py`

### Front-end (`ui`)

- [ ] **0.6** Dashboard analytics page — document counts by type/status/month
    - Components: `DashboardCard` (reuse), add recharts for line/bar charts
    - File: `src/app/dashboard/page.tsx`

- [ ] **0.7** CC-e modal in NF-e detail page — inline correction letter form
    - Files: `src/app/nfe/detail/page.tsx`, new `src/components/NfeCceModal.tsx`

- [ ] ~~**0.8** DANFE download button in NF-e detail page~~
    - ~~Files: `src/app/nfe/detail/page.tsx`, `src/lib/api.ts`~~

- [ ] ~~**0.9** Inutilizacao form page — serie + number range + justification~~
    - ~~File: new `src/app/nfe/inutilization/page.tsx`~~

- [ ] **0.10** NF-e Distributions Page
    - Should list all NSUs distributions descending
    - Should create a sonner alert when received a new nfeProc (on any page)

---

## Phase 1 — NFC-e (Nota Fiscal do Consumidor Eletrônica)

> NFC-e shares SEFAZ services with NF-e (`NFeAutorizacao`, `RecepcaoEvento`) with QR Code differences.
> `py-dfe` already has `NFCeServiceClient`; CDK has config tables; API has config repos.

### `py-dfe` (Lambda Layer)

- [ ] **1.1** Verify `NFCeServiceClient` produces valid QR Code fields (`qrCode`, `urlChave`) in XML output
    - Validate against XSD in `schemas/xsds/PL_010c_NT2022_002v1.30/`
    - File: `py_dfe/services/nfce.py`

### `api`

- [ ] **1.2** Create `app/services/nfce_service.py` — mirrors `NfeService` pattern
    - Key differences: `mod=65`, QR Code generation (HMAC-SHA1 + CSC token), access key with mod 65
    - Reuse: access key algorithm, tax calculation pipeline, `transact_write` numbering
    - File: new `app/services/nfce_service.py`

- [ ] **1.3** Create `app/api/nfces.py` — CRUD routes mirroring `nfes.py`
    - `GET/POST /v1.0/nfces`, `GET /v1.0/nfces/{access_key}`, cancel, xml, events
    - Files: new `app/api/nfces.py`, update `app/api/__init__.py`

- [ ] **1.4** Create `app/schemas/nfces.py` — NFC-e specific schemas (QR Code, CSC token fields)
    - Reuse `NFeIn`/`NFeOut` base classes where possible

- [ ] **1.5** Verify `app/repositories/nfces.py` query methods (number-index, date-index GSIs)

### `worker`

- [ ] **1.6** Add NFC-e routing to `app/handlers/dfe.py`
    - NFC-e reuses `NFeAutorizacao` — route via `doc_type=nfce` SNS filter attribute

### `cdk`

- [ ] **1.7** Add `nfce-emission` + `nfce-event` SQS queues + Lambda workers in `WorkerStack`
    - SNS filter: `sefaz_service = NFeAutorizacao AND doc_type = nfce`
    - File: `lib/worker-stack.ts`

- [ ] **1.8** Update IAM policies for NFC-e tables in API and Worker roles
    - File: `lib/iam-stack.ts`

### `ui`

- [ ] **1.9** Replace `/nfce` ComingSoon with full NFC-e list page
    - Reuse: `usePagination`, `queryKeys`, NF-e list pattern
    - File: `src/app/nfce/page.tsx`

- [ ] **1.10** NFC-e emit form — simplified POS style (products, QR Code preview, payment)
    - Reuse: `NfeEmitForm` base, `ProductCombobox`, payment section
    - Files: `src/app/nfce/emit/page.tsx`, new `src/components/NfceEmitForm.tsx`

- [ ] **1.11** NFC-e detail page with QR Code display + cancellation modal
    - File: `src/app/nfce/detail/page.tsx`

- [ ] **1.12** Wire NFC-e in `src/lib/api.ts`: `getNfces()`, `emitNfce()`, `cancelNfce()`

---

## Phase 2 — CT-e (Conhecimento de Transporte Eletrônico)

> Most complex of the 4 core docs: 4 modal types (rodoviário, aéreo, aquaviário, ferroviário),
> multiple emission models (Sinc, OS, GTVe, Simplificado), specific ICMS transport rules.

### `py-dfe` (Lambda Layer)

- [ ] **2.1** Audit CT-e XSD schemas in `schemas/xsds/PL_CTe_400_NT2026.001/`
    - Confirm all modal schemas present: CTeOS, GTVe, CTeSimp, standard Cte

- [ ] **2.2** Create XML builder helpers for CT-e modals
    - `build_cte_rodoviario()`, `build_cte_aereo()`, `build_cte_os()`, `build_cte_simpl()`
    - File: new `py_dfe/xmlops/builders/cte.py`

### `api`

- [ ] **2.3** Create `app/services/cte_service.py`
    - CT-e access key (cDV algorithm identical to NF-e with `mod=57`)
    - ICMS transport: CST 00/20/45/60/90, CFOP transport ranges (6xxx/7xxx)
    - Modal dispatch based on `tipo_serv`: 0=normal, 6=multimodal, 7=ctrc, 8=os, 9=simplificado
    - File: new `app/services/cte_service.py`

- [ ] **2.4** Create `app/api/ctes.py` — routes mirroring NF-e
    - `GET/POST /v1.0/ctes`, `GET /v1.0/ctes/{access_key}`, cancel, xml, events, DACTE stub
    - File: new `app/api/ctes.py`

- [ ] **2.5** Create `app/schemas/ctes.py`
    - `CTeIn`: emitente, tomador, remetente, destinatario, expedidor, recebedor, cargas, modal details
    - File: new `app/schemas/ctes.py`

- [ ] **2.6** CT-e event handlers: cancellation `110111`, CC-e `110110`, prestação em desacordo `610710`
    - Reuse NF-e event pipeline; add CT-e specific event type constants
    - File: `app/services/cte_service.py`

### `worker`

- [ ] **2.7** Verify `cte-emission-worker` + `cte-event-worker` in `app/handlers/dfe.py`
    - Emission handles: `CTeRecepcaoSinc`, `CTeRecepcaoOS`, `CTeRecepcaoGTVe`, `CTeRecepcaoSimp`

### `cdk`

- [ ] **2.8** Verify `cte-emission` + `cte-event` stacks in `WorkerStack` are complete
    - File: `lib/worker-stack.ts`

### `ui`

- [ ] **2.9** CT-e list page — replace ComingSoon placeholder
    - File: `src/app/cte/page.tsx`

- [ ] **2.10** CT-e emit form — stepped (step 1: modal type, step 2: parties, step 3: cargo, step 4: values)
    - Reuse: `AddressFields`, `PersonCombobox`, `VehicleCombobox`
    - Files: new `src/components/CteEmitForm.tsx`, `src/app/cte/emit/page.tsx`

- [ ] **2.11** CT-e detail page + DACTE download stub
    - File: `src/app/cte/detail/page.tsx`

- [ ] **2.12** Wire CT-e in `src/lib/api.ts`: `getCtes()`, `emitCte()`, `cancelCte()`

---

## Phase 3 — MDF-e (Manifesto Eletrônico de Documentos Fiscais)

> Transport manifest linking CT-e/NF-e access keys to vehicles and drivers.
> Simpler business logic than CT-e; vehicles/drivers model already exists.

### `api`

- [ ] **3.1** Create `app/services/mdfe_service.py`
    - MDF-e access key uses `mod=58`
    - Fields: UF loading/unloading, list of referenced NF-e/CT-e keys, vehicles, drivers, CIOT
    - Modal: rodoviário (plate + RNTRC), others simpler
    - File: new `app/services/mdfe_service.py`

- [ ] **3.2** Create `app/api/mdfes.py` — routes
    - `GET/POST /v1.0/mdfes`, `GET /v1.0/mdfes/{access_key}`, encerrar, cancel, xml, events
    - Encerramento event: `110112`; cancellation: `110111`
    - File: new `app/api/mdfes.py`

- [ ] **3.3** Create `app/schemas/mdfes.py`
    - `MDFeIn`: UF percurso list, modal, NF-e/CT-e key references, vehicles, drivers, CIOT
    - File: new `app/schemas/mdfes.py`

- [ ] **3.4** `GET /v1.0/mdfes/nao-encerrados` — query open manifests
    - Calls `MDFeConsNaoEnc` SEFAZ service to retrieve non-closed manifests
    - Files: `app/api/mdfes.py`, `app/services/mdfe_service.py`

### `worker`

- [ ] **3.5** Verify `mdfe-emission-worker` + `mdfe-event-worker` in `app/handlers/dfe.py`

### `cdk`

- [ ] **3.6** Verify `mdfe-emission` + `mdfe-event` stacks in `WorkerStack` are complete

### `ui`

- [ ] **3.7** MDF-e list page — replace ComingSoon; distinguish open vs. closed manifests
    - File: `src/app/mdfe/page.tsx`

- [ ] **3.8** MDF-e emit form — UF percurso, document references (multi-select CT-e/NF-e), vehicle + driver
    - Reuse: `VehicleCombobox`, `AccessKeyListInput`
    - Files: new `src/components/MdfeEmitForm.tsx`, `src/app/mdfe/emit/page.tsx`

- [ ] **3.9** MDF-e detail page with "Encerrar" + "Cancelar" action buttons
    - File: `src/app/mdfe/detail/page.tsx`

- [ ] **3.10** Wire MDF-e in `src/lib/api.ts`: `getMdfes()`, `emitMdfe()`, `cancelMdfe()`, `encerrarMdfe()`

---

## Phase 4 — Advanced Wave 1: BPe + NF3e

### BPe (Bilhete de Passagem Eletrônico) — Bus/boat passenger ticket

> Regulated by ANTT. Per-UF SEFAZ endpoints. Schema `BPe_v1.00`.

#### `py-dfe`

- [ ] **4.1** Download BPe XSD schemas from SVRS/SEFAZ portals → `schemas/xsds/BPe_v100/`
- [ ] **4.2** Add BPe service enums to `py_dfe/constants/enums.py`
    - `BPeRecepcao`, `BPeConsulta`, `BPeStatusServico`, `BPeRecepcaoEvento`
- [ ] **4.3** Add BPe endpoints to `py_dfe/constants/endpoints.py` (SVRS + per-UF authorizers)
- [ ] **4.4** Create `py_dfe/services/bpe.py` — `BPeServiceClient`
    - Sign element: `infBPe`; namespace `http://www.portalfiscal.inf.br/bpe`
- [ ] **4.5** Register BPe signing config in `py_dfe/services/config.py`

#### `api`

- [ ] **4.6** Create `app/services/bpe_service.py`
    - Passenger data, route (origem→destino), vehicle, validity, modality (rodoviário/fluvial)
    - Tax: ICMS transport, ISS (municipal services)
- [ ] **4.7** Create `app/api/bpes.py` + `app/schemas/bpes.py`
- [ ] **4.8** Verify or create `app/repositories/bpes.py`

#### `cdk`

- [ ] **4.9** Add `bpes`, `bpe_configs`, `bpe_events` tables to `DynamoDBStack`
    - Pattern: same GSIs as `nfes` (number-index, dfe-index)
    - File: `lib/dynamodb-stack.ts`
- [ ] **4.10** Add `bpe-emission` + `bpe-event` SQS queues + Lambda workers to `WorkerStack`
    - File: `lib/worker-stack.ts`
- [ ] **4.11** Update IAM roles with BPe table permissions
    - File: `lib/iam-stack.ts`

#### `ui`

- [ ] **4.12** BPe list page + emit form (passenger, route, vehicle, price, seat) + detail page
    - Files: `src/app/bpe/`
- [ ] **4.13** Add BPe to sidebar navigation + `src/lib/api.ts`

---

### NF3e (Nota Fiscal de Energia Elétrica Eletrônica) — Electric utility invoice

> Issued by energy concessionaires (distribuidoras). Schema `NF3e_v1.00`. ANEEL regulated.

#### `py-dfe`

- [ ] **4.14** Download NF3e XSDs → `schemas/xsds/NF3e_v100/`
- [ ] **4.15** Add NF3e enums: `NF3eRecepcao`, `NF3eConsulta`, `NF3eStatusServico`, `NF3eRecepcaoEvento`
- [ ] **4.16** Add NF3e SEFAZ endpoints (check SVRS + per-state jurisdiction)
- [ ] **4.17** Create `py_dfe/services/nf3e.py` — `NF3eServiceClient`

#### `api`

- [ ] **4.18** Create `app/services/nf3e_service.py`
    - Consumption period, meter reading, tariff bands (TUSD/TE), taxes (ICMS/PIS/COFINS/CIP/COSIP)
- [ ] **4.19** Create `app/api/nf3es.py` + `app/schemas/nf3es.py` + `app/repositories/nf3es.py`

#### `cdk`

- [ ] **4.20** Add `nf3es`, `nf3e_configs`, `nf3e_events` tables
- [ ] **4.21** Add `nf3e-emission` + `nf3e-event` workers

#### `ui`

- [ ] **4.22** NF3e list page + emit form (consumption period, tariffs, meter data) + detail page
    - Files: `src/app/nf3e/`
- [ ] **4.23** Add NF3e to sidebar + `src/lib/api.ts`

---

## Phase 5 — Advanced Wave 2: NFCom + NFF + NFGas

### NFCom (Nota Fiscal de Comunicação) — Telecom invoice

> Issued by telecom operators. Schema `NFCom_v1.00`. ANATEL regulated.

#### `py-dfe`

- [ ] **5.1** Download NFCom XSDs → `schemas/xsds/NFCom_v100/`
- [ ] **5.2** Add NFCom enums + endpoints + `NFComServiceClient` in `py_dfe/services/nfcom.py`

#### `api`

- [ ] **5.3** `app/services/nfcom_service.py` — service line, SIM/IMEI, plan, taxes (ICMS/PIS/COFINS/FUST/FUNTTEL)
- [ ] **5.4** `app/api/nfcoms.py` + `app/schemas/nfcoms.py` + `app/repositories/nfcoms.py`

#### `cdk`

- [ ] **5.5** Add `nfcoms`, `nfcom_configs`, `nfcom_events` tables + `nfcom-emission` + `nfcom-event` workers

#### `ui`

- [ ] **5.6** NFCom list + emit form (service/plan details, SIM data) + detail page
    - Files: `src/app/nfcom/`
- [ ] **5.7** Add NFCom to sidebar + `src/lib/api.ts`

---

### NFF (Nota Fiscal Fácil / Produtor Rural) — Rural producer invoice

> Simplified emission for rural producers. SENAR/SEFAZ. Schema varies per UF (SP, MG, RS).

#### `py-dfe`

- [ ] **5.8** Download NFF XSDs per UF → `schemas/xsds/NFF/{uf}/`
- [ ] **5.9** Add NFF enums + per-UF endpoints + `NFFServiceClient` in `py_dfe/services/nff.py`

#### `api`

- [ ] **5.10** `app/services/nff_service.py` — rural products (NCM agro), FUNRURAL tax, IE rural
- [ ] **5.11** `app/api/nffs.py` + `app/schemas/nffs.py` + `app/repositories/nffs.py`

#### `cdk`

- [ ] **5.12** Add `nffs`, `nff_configs`, `nff_events` tables + `nff-emission` + `nff-event` workers

#### `ui`

- [ ] **5.13** NFF list + emit form (rural products, FUNRURAL, producer IE) + detail page
    - Files: `src/app/nff/`
- [ ] **5.14** Add NFF to sidebar + `src/lib/api.ts`

---

### NFGas (Nota Fiscal de Gás Natural) — Natural gas distributor invoice

> Schema `NFGas_v1.00` (SVRS jurisdiction).

#### `py-dfe`

- [ ] **5.15** Download NFGas XSDs → `schemas/xsds/NFGas_v100/`
- [ ] **5.16** Add NFGas enums + endpoints + `NFGasServiceClient` in `py_dfe/services/nfgas.py`

#### `api`

- [ ] **5.17** `app/services/nfgas_service.py` — consumption volume (m³), tariff, ICMS/PIS/COFINS
- [ ] **5.18** `app/api/nfgass.py` + `app/schemas/nfgass.py` + `app/repositories/nfgass.py`

#### `cdk`

- [ ] **5.19** Add `nfgass`, `nfgas_configs`, `nfgas_events` tables + workers

#### `ui`

- [ ] **5.20** NFGas list + emit form (consumption volume, tariffs) + detail page
    - Files: `src/app/nfgas/`
- [ ] **5.21** Add NFGas to sidebar + `src/lib/api.ts`

---

## Phase 6 — Advanced Wave 3: DCe + NFag

### DCe (Documento de Compensação de Energia Elétrica) — Energy settlement

> CCEE energy market settlements. Schema `DCe_v1.00`. Highly specialized.

#### `py-dfe`

- [ ] **6.1** Download DCe XSDs → `schemas/xsds/DCe_v100/`
- [ ] **6.2** Add DCe enums + CCEE endpoints + `DCeServiceClient` in `py_dfe/services/dce.py`

#### `api`

- [ ] **6.3** `app/services/dce_service.py` — energy contracts, settlement period, PLD prices, counterpart
- [ ] **6.4** `app/api/dces.py` + `app/schemas/dces.py` + `app/repositories/dces.py`

#### `cdk`

- [ ] **6.5** Add `dces`, `dce_configs`, `dce_events` tables + workers

#### `ui`

- [ ] **6.6** DCe list + emit form (settlement contract, energy volume, counterpart) + detail page
    - Files: `src/app/dce/`
- [ ] **6.7** Add DCe to sidebar + `src/lib/api.ts`

---

### NFag (Nota Fiscal Agrícola) — Agricultural invoice

> Agribusiness transactions. Tied to MAPA/SENAR. State-specific schemas.

#### `py-dfe`

- [ ] **6.8** Download NFag XSDs per applicable UFs → `schemas/xsds/NFag/{uf}/`
- [ ] **6.9** Add NFag enums + endpoints + `NFagServiceClient` in `py_dfe/services/nfag.py`

#### `api`

- [ ] **6.10** `app/services/nfag_service.py` — agricultural products (SIF/SIE codes), storage location, harvest data
- [ ] **6.11** `app/api/nfags.py` + `app/schemas/nfags.py` + `app/repositories/nfags.py`

#### `cdk`

- [ ] **6.12** Add `nfags`, `nfag_configs`, `nfag_events` tables + workers

#### `ui`

- [ ] **6.13** NFag list + emit form (agricultural products, harvest, location) + detail page
    - Files: `src/app/nfag/`
- [ ] **6.14** Add NFag to sidebar + `src/lib/api.ts`

---

## Cross-Cutting Tasks (all phases)

- [ ] **X.1** RBAC: add permissions for each new doc type to `USER` and `VIEWER` roles
    - Pattern: `list.organization_{doc}s`, `get.organization_{doc}s`, `create.organization_{doc}s`
- [ ] **X.2** Add each new doc type to sidebar navigation in `Sidebar.tsx` (grouped by category)
- [ ] **X.3** Add fiscal config tab per new doc type in `/fiscal-config` page
- [ ] **X.4** Add `PUT /v1.0/organizations/{pk}/{doc}-config` endpoint per new doc type
- [ ] **X.5** Update `DOCS.md` for each new endpoint/schema added
- [ ] **X.6** Update `CONDUCT.md` for new SEFAZ constraints or business rules discovered
- [ ] **X.7** Unit tests per new service (tax calculation, access key algorithm, XML building)
- [ ] **X.8** Integration test per new document type (issue in HOM, check S3 XML, verify DynamoDB record)

---

## Delegation Guide for AI Agents

Each task block above is scoped for parallel agent delegation. Suggested groupings:

| Agent | Tasks                        | Reference Template                                    |
|-------|------------------------------|-------------------------------------------------------|
| A1    | 0.1–0.5 (NF-e API gaps)      | `app/services/nfe_service.py`                         |
| A2    | 0.6–0.9 (NF-e frontend gaps) | `src/app/nfe/` pages                                  |
| A3    | 1.2–1.5 (NFC-e API)          | `NfeService` + `NFeRepository`                        |
| A4    | 1.9–1.12 (NFC-e frontend)    | `src/app/nfe/` as template                            |
| A5    | 1.7–1.8 (NFC-e CDK)          | `lib/worker-stack.ts` + `lib/dynamodb-stack.ts`       |
| A6    | 2.3–2.6 (CT-e API)           | `CTeServiceClient` in `py-dfe`                        |
| A7    | 2.9–2.12 (CT-e frontend)     | CT-e modal type guide                                 |
| A8    | 3.1–3.4 (MDF-e API)          | `MDFeServiceClient` + vehicle/driver models           |
| A9    | 3.7–3.10 (MDF-e frontend)    | MDF-e service + vehicle combobox                      |
| A10   | 4.1–4.5 (BPe py-dfe)         | Existing service client pattern in `py_dfe/services/` |

For Phase 4+ agents: always provide the agent with:

1. Path to NF-e service as the pattern template
2. The new doc type's schema spec (downloaded XSD) for field reference
3. The CDK worker-stack and dynamodb-stack patterns

---

## Verification Steps per Phase

| Phase | Verification                                                                               |
|-------|--------------------------------------------------------------------------------------------|
| 0     | Issue NF-e HOM → inutilize range → send CC-e → download DANFE PDF → manifest incoming NF-e |
| 1     | Issue NFC-e HOM → verify QR Code URL → cancel → check events endpoint                      |
| 2     | Issue CT-e rodoviário HOM → verify DACTE stub → send cancellation event                    |
| 3     | Issue MDF-e HOM linking CT-e keys → call encerrar → verify `nao-encerrados` returns empty  |
| 4     | Issue BPe HOM → cStat=100 → XML in S3 → DynamoDB record → cancel                           |
| 4     | Issue NF3e HOM → cStat=100 → XML in S3                                                     |
| 5–6   | Same pattern per document type using HOM environment                                       |

**HOM environment:** set `sefaz_environment=homologacao` in org config. Use SEFAZ test CNPJ `99999090910270` for
homologação testing where required.
