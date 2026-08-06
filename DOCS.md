# py-dfe — Technical Reference

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [py-dfe — Core Library](#3-py-dfe--core-library)
4. [ctech-dfe-api — REST Backend](#4-ctech-dfe-api--rest-backend)
5. [ui — Frontend](#5-ui--frontend)
6. [ctech-dfe-worker — Async Workers](#6-ctech-dfe-worker--async-workers)
8. [cdk — Infrastructure](#8-cdk--infrastructure)
9. [Database](#9-database)
10. [Security](#10-security)
11. [Observability](#11-observability)
12. [Deployment](#12-deployment)
13. [Constraints and Architectural Decisions](#13-constraints-and-architectural-decisions)

---

## 1. Overview

**py-dfe** is a multi-tenant SaaS platform for fiscal communication with SEFAZ (Secretaria da Fazenda). It enables
companies to issue and manage Electronic Tax Documents (DF-e) via REST API or web interface.

**Supported documents:**

- NF-e — Nota Fiscal Eletrônica (goods)
- NFC-e — Nota Fiscal de Consumidor Eletrônica (POS)
- CT-e — Conhecimento de Transporte Eletrônico
- MDF-e — Manifesto de Documento Fiscal Eletrônico

**SEFAZ environments:** Production and Homologation (per organization and document type).

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ui                            │
│              Next.js 16 · TypeScript · ShadCN · Tailwind        │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS · JWT Bearer
┌────────────────────────────▼────────────────────────────────────┐
│                     ctech-dfe-api                               │
│              Fiber v3 · Go · AWS SDK v2 · Redis JWKS cache      │
│  ┌──────────────┐ ┌───────────────┐ ┌──────────────────────┐   │
│  │ Auth / RBAC  │ │ Organizations │ │ NF-e / CT-e / MDF-e  │   │
│  └──────────────┘ └───────────────┘ └──────────────────────┘   │
└──────┬──────────────┬──────────┬─────────────┬──────────────────┘
       │              │          │             │
  ┌────▼─────┐  ┌─────▼──────┐  │   ┌─────────▼──────┐
  │ DynamoDB │  │     S3     │  │   │ Redis (Valkey)  │
  │ 26 tables│  │certs+xmls  │  │   │ JWKS · WS Pub/Sub│
  └──────────┘  └────────────┘  │   └────────────────┘
                            ┌───▼────────────────────┐
                            │ Outbox → SNS → SQS     │
                            └───┬────────────────────┘
                                │
                        ┌───────▼──────────────┐
                        │  ctech-dfe-worker    │
                        │  Go Lambda           │
                        └───────┬──────────────┘
                                │ InvokeFunction
                        ┌───────▼──────────────┐
                        │  ctech-dfe (py-dfe)  │
                        │  Python Lambda       │
                        │  Sign XML · SOAP     │
                        └───────┬──────────────┘
                                │ mTLS
                        ┌───────▼────────┐
                        │     SEFAZ      │
                        └────────────────┘
```

**Data flow:**

1. HTTP client authenticates via JWT and selects the active organization
2. ctech-dfe-api validates JWT (RS256, JWKS cached in Redis), loads data from DynamoDB and certificate from S3
3. Issuance transacts the fiscal number, document, and immutable `worker_outbox` command
4. The outbox DynamoDB Stream publisher forwards the command through SNS to standard SQS
5. ctech-dfe-worker conditionally claims the target, fetches the certificate, and invokes go-dfe or py-dfe
6. The fiscal engine signs XML, opens mTLS with the A1 certificate, and submits to SEFAZ
7. Worker persists the result, uploads XML to S3, and publishes terminal status to results SNS
8. ctech-dfe-api consumes results and delivers the update through Valkey/WebSocket

---

## 3. py-dfe — Core Library

**Location:** `/py-dfe/`
**Runtime:** AWS Lambda, Python 3.14

### Responsibility

Stateless library that receives a structured payload, handles all fiscal communication with SEFAZ, and returns the
result. Does not access DynamoDB, S3, or depend on FastAPI.

### Entrypoint

```python
# py_dfe/handler.py
from typing import Any


def handler(event: dict, context: Any) -> dict:
    ...
    # Validates LambdaRequest (Pydantic v2)
    # Instantiates CertificateManager with base64 PFX
    # Routes to NFeServiceClient / NFCeServiceClient / etc.
    # Returns LambdaResponse {statusCode, body, headers}
```

### LambdaRequest

```python
from pydantic import BaseModel
from typing import Literal


class LambdaRequest(BaseModel):
    cnpj: str  # 14 digits
    uf: str  # 2 letters, auto-uppercase
    environment: Literal["producao", "homologacao"]
    doc_type: Literal["nfe", "nfce", "cte", "mdfe"]
    service: str  # e.g. "NFeAutorizacao"
    certificate_b64: str  # PFX in base64
    certificate_password: str
    body: dict[str, Any]  # Payload that becomes XML
    validate_schema: bool = False  # XSD validation (CPU-intensive)
    max_retries: int = 3  # Range 0-10
```

### Module Structure

```
py_dfe/
├── handler.py              # Lambda entrypoint + routing
├── certificate/
│   └── manager.py          # Loads PFX, extracts private key, configures mTLS
├── constants/
│   ├── enums.py            # UF, Environment, DocType enums
│   └── endpoints.py        # SEFAZ endpoint registry (prod + hom, per UF)
├── services/
│   ├── base.py             # SefazClient — async httpx, retry with backoff
│   ├── _nf.py              # Shared NF-e/NFC-e logic
│   ├── nfe.py / nfce.py / cte.py / mdfe.py
├── danfe/                  # Auxiliary-document rendering (no cert, no SEFAZ)
│   ├── render.py           # Generic Jinja2 HTML → WeasyPrint PDF (fit_height flag)
│   ├── qr.py               # QR string → data-URI PNG (segno, level M, UTF-8)
│   ├── barcode.py          # CODE-128 → data-URI SVG + FS/FS-DA dados-nfe code
│   ├── formatters.py       # BR money/date/CNPJ/CPF/CEP/chave/aliquota formatting
│   ├── document.py         # GerarDanfe dispatcher by ide/mod (55/65)
│   ├── danfce.py           # DANFC-e (mod 65): XML → context, variant logic
│   ├── nfe55.py            # DANF-e (mod 55): XML → context, variant logic
│   ├── mdfe58.py           # DAMDFE (mod 58): MDF-e XML → context, modal logic
│   └── templates/          # danfce.html + _danfe_macros.html + danfe_{retrato,
│                           #   paisagem,simplificado,etiqueta}.html +
│                           #   _damdfe_macros.html + damdfe_{retrato,paisagem}.html
├── soap/
│   └── envelope.py         # Builds SOAP envelope for each service
├── xmlops/
│   ├── builder.py          # dict ↔ XML (lxml)
│   ├── signer.py           # XML-DSig (signxml)
│   └── validator.py        # XSD validation
└── schemas/xsds/           # Bundled XSDs (NF-e, NFC-e, CT-e, MDF-e)
```

### Render service — `GerarDanfe` (auxiliary documents)

A non-SEFAZ render service generates the auxiliary fiscal document from an
authorized XML locally. **No certificate, no SEFAZ call.**
`danfe/document.py::generate_danfe` dispatches by `ide/mod`:

- **mod 65 → DANFC-e** (`danfe/danfce.py`): thermal receipt, QR Code (segno);
  variants **completo**/**resumido** (`layout`); **2-via contingência** auto from
  `ide/tpEmis==9`; **homologação** banner+watermark from `tpAmb==2`; **cancelada**
  watermark from `canceled` flag. QR URL read from `infNFeSupl/qrCode`.
- **mod 55 → DANF-e** (`danfe/nfe55.py`): NF-e DANFE, **CODE-128 barcode** of the
  44-digit chave (`danfe/barcode.py`, python-barcode → inline SVG). Variants via
  `layout`: **retrato** (default), **paisagem** (fixed A4, multi-page with a
  running-element header + `counter(page)/pages`), **simplificado**, **etiqueta**
  (roll ≥55mm, auto-height). Contingency by `ide/tpEmis`: normal/SCAN/SVC
  (chave barcode + protocolo), **FS/FS-DA** (second "Dados da NF-e" 36-char
  barcode, protocolo suppressed), **EPEC** (protocolo do EPEC).

- Request: `doc_type="nfe"|"nfce"`, `service="GerarDanfe"`, `body={"xml": "…",
  "layout": <variant>, "canceled": false}`. Model auto-detected from the XML;
  `certificate_b64` optional (required only for SEFAZ services — handler raises
  `DFeError(400, "certificate required", …)` otherwise).
- Response: `{"pdf_b64": "<base64 PDF>", "html": ["<page>", …]}`.

### Render service — `GerarDamdfe` (DAMDFE, MDF-e mod 58)

Sibling render service for the **MDF-e** auxiliary document (manual_damdfe.md,
MOC 3.00b Anexo II). Same pure-local contract (no certificate, no SEFAZ).
`danfe/mdfe58.py::generate_damdfe`:

- Reads the authorized `<mdfeProc>` (or bare `<MDFe>`); requires `ide/mod==58`.
- **CODE-128 barcode** of the 44-digit chave + **QR Code** from
  `infMDFeSupl/qrCodMDFe` (segno). All four **modais** rendered from
  `ide/modal`: **1 rodoviário** (RNTRC, veículo tração/reboques, condutores,
  CIOT), **2 aéreo**, **3 aquaviário**, **4 ferroviário**.
- Variants via `layout`: **retrato** (default) / **paisagem**, both fixed A4
  multi-page (long document-key lists paginate naturally, `fit_height=False`).
- **Contingência** from `ide/tpEmis==2` (prints "EMISSÃO EM CONTINGÊNCIA",
  suppresses protocolo); **homologação** watermark from `tpAmb==2`; **cancelada**
  watermark from `canceled` flag.
- Document keys grouped per discharge municipality (`infMunDescarga`):
  NF-e/CT-e/MDF-e. Totals, seguros, lacres, observações rendered too.
- Request: `doc_type="mdfe"`, `service="GerarDamdfe"`, `body={"xml": "…",
  "layout": <variant>, "canceled": false}`. Routed in `MDFeServiceClient.call`;
  certificate optional (`RENDER_ONLY_SERVICES` allowlist in handler).
- Response: `{"pdf_b64": "<base64 PDF>", "html": ["<page>", …]}`.
- All data read from the XML only (manual mandate).
- Engine: WeasyPrint (HTML→PDF), Jinja2 (templates), segno (NFC-e QR),
  python-barcode (NF-e CODE-128). WeasyPrint needs native libs in the Lambda
  layer (see CONDUCT.md). Two sizing modes in `render.py::htmls_to_pdf`
  (`fit_height=True` roll/auto-height; `False` fixed A4 multi-page).

**API invocation (synchronous).** The Go API renders these PDFs on demand via
`ExternalService.GeneratePDF` (`api/internal/services/external.go`), which invokes
the **same py-dfe Lambda used for SEFAZ** (`SEFAZ_FUNCTION_NAME`) synchronously —
no SNS/worker hop, no certificate. It fetches the authorized XML from S3, sends
`{doc_type, service, body:{xml, canceled}}`, and decodes `pdf_b64` to PDF bytes.
The `cnpj`/`uf` request fields are required by the schema but unused by the render
path, so a CPF issuer or unknown UF falls back to a placeholder. Exposed as
`GET /v1.0/nfces/{key}/danfce` and `GET /v1.0/mdfes/{key}/damdfe`. (NF-e mod 55
`GET /v1.0/nfes/{key}/danfe` still uses the external `consultadanfe.com` provider.)

### Error Handling

```python
class DFeError(Exception):
    status_code: int  # Suggested HTTP status
    code: str  # Machine-readable error code
    message: str  # Descriptive message


# Examples:
DFeError(422, "SCHEMA_VALIDATION_FAILED", "XML failed XSD validation")
DFeError(502, "SEFAZ_UNREACHABLE", "Timeout communicating with SEFAZ")
DFeError(400, "CERT_EXPIRED", "A1 certificate has expired")
```

### Dependencies

```toml
httpx          # Async HTTP client (mTLS support)
lxml           # XML parsing and serialization
cryptography   # PFX loading, key extraction
signxml        # XML Digital Signature (XML-DSig)
pydantic> = 2    # Input validation
weasyprint     # HTML → PDF (DANFE rendering; needs native libs in Lambda layer)
jinja2         # DANFE HTML templating
segno          # QR Code generation
```

### go-dfe — In-Process Go Migration (New)

**Location:** `/go-dfe/` (module `gopkg.aoctech.app/dfe/go-dfe`, go 1.26; sibling of `worker/`/`api/`,
linked via root `go.work`). See `docs/plans/2026-07-17-go-dfe-migration.md` and `MIGRATION.md` (2026-07-18
entry) for the full rationale and phasing.

**Purpose:** incremental, in-process replacement for py-dfe's SEFAZ SOAP/mTLS communication —
called directly from `worker`/`api` (no Lambda Invoke round trip) instead of the py-dfe Lambda,
one operation at a time, with automatic fallback to py-dfe for anything not yet ported.

```
go-dfe/
  dfe.go                       # Call(ctx, Request) (Response, error); Implements(docType, service) bool
  request.go                   # Request/Response/Problem — same JSON contract as py-dfe's LambdaRequest/LambdaResponse
  internal/
    certificate/manager.go     # PKCS12 → tls.Certificate + http.Client (mTLS, InsecureSkipVerify — deliberate, mirrors py-dfe)
    xmlops/
      builder.go                # dict ↔ XML (@xmlns/@key/#text conventions)
      signer.go                 # XML-DSig: rsa-sha1 + sha1 + hand-written Canonical XML 1.0 (no exclusive C14N, no prefix)
      xsdorder/table.go          # 1:1 port of py-dfe's xsd_order.py (child-element ordering per XSD)
    soap/envelope.go            # SOAP 1.2 envelope build/parse
    services/
      config.go                 # ServiceConfig per doc_type (signature/validation sets, sign xpath)
      client.go                 # SefazClient.Call: payload prep → endpoint resolve → POST+retry → parse
    endpoints/table.go           # (doc_type, uf, env, service) → URL, incl. SVRS redirects + MT special-casing
    constants/constants.go       # enums, WSDL service/operation tables, retry defaults
```

**Dispatch/fallback mechanism** — no new feature-flag system, reuses the repo's existing
dual-path-with-fallback precedent.

- **`worker`** (`internal/service/dfe.go`'s `Process` seam and `distribution.go`'s `invokePyDfe`):
  hard cutover as of 2026-07-18. `godfeImplements`/`godfeCall` (package vars wrapping
  `godfe.Implements`/`godfe.Call`, `internal/service/godfe_shadow.go`) dispatch straight to go-dfe
  in-process for every implemented `(docType, service)` — the py-dfe Lambda is skipped entirely, not
  shadow-compared. This was done at explicit operator direction during a controlled zero-traffic
  window, ahead of the plan's normal shadow-mode/byte-identical gates for the signed operations this
  promoted (see below) — an accepted, explicit exception, documented in `go-dfe/dfe.go`'s
  `implemented` map doc comment and `go-dfe/CLAUDE.md`, not a silent skip. Each seam keeps one
  commented-out line to force the old unconditional py-dfe call again if this needs reverting.
- **`api`** (`internal/services/external.go`'s `invokeSefazLambda`): still **shadow mode** — the
  py-dfe Lambda call stays unconditional and authoritative; `godfe.ShadowCompare` runs alongside it
  for `Implements()` operations and only logs a divergence, never affecting the response. Unchanged
  by the worker cutover above.

**Current `Implements()` set:** every `(docType, service)` `worker` actually processes today (see
`cdk/lib/worker-definitions.ts`) — status/consulta/distribuição (unsigned, gated normally) plus
`NFeAutorizacao`/`RecepcaoEvento`/`NfeInutilizacao` (nfe+nfce) and CT-e/MDF-e's emission/event
services (signed, promoted 2026-07-18 per the exception above — the byte-identical gate against a
captured py-dfe corpus has NOT run for these; no dedicated test certificate exists in this repo yet.
See `internal/xmlops/signer.go`'s test file for what's verified today instead: W3C C14N spec vectors
and an internal sign/verify round trip, not a py-dfe diff).

**Explicitly out of scope:** XSD validation (no mature pure-Go validator, `CGO_ENABLED=0` rules out
libxml2-based options) and DANFE/DAMDFE rendering (no cert/signature/SOAP/mTLS involved — no fiscal
or security upside to porting it; py-dfe remains the only path for rendering indefinitely).

**CI:** a dedicated `godfe` job (`.github/workflows/godfe.yml`, build+test only — this package has no
deploy target of its own) runs in `deploy.yml`, gated on a `go-dfe/**`/`go.work`/`go.work.sum` path
filter. That same filter is also added to `worker`/`api`'s own filters (they consume `go-dfe`
in-process via the workspace, so a change there must re-run their test suites too, not just
`go-dfe`'s) and `worker`/`api`'s deploy jobs `needs: godfe`, so a `go-dfe` test failure blocks their
deploys.

**Camada NFS-e (`go-dfe/nfse/`, F2 — 2026-08-05):** NFS-e não é SOAP e não passa por
`internal/services` nem `internal/soap` — `dfe.Call` desvia para o pacote `nfse` logo após
`certificate.Load` (o certificado é o mesmo) e antes de montar qualquer cliente SOAP. Estrutura:

```
go-dfe/nfse/
  document.go, result.go, provider.go, errors.go, constants.go, dispatch.go
  tables/                      # tabelas de referência da F1 (trib nacional, NBS, indOp)
  nacional/                    # provider REST+JSON do Sistema Nacional NFS-e (F2)
    endpoints.go, dps.go, dps_ibscbs.go, evento.go, transport.go, provider.go, adn.go
```

- **Modelo neutro:** `nfse.Document` (moldado no DPS 1.01) é o formato que `api` monta em
  `dfe.Request.Body["document"]`; `nfse.DecodeDocument` rejeita campo desconhecido
  (`json.Decoder.DisallowUnknownFields`) — um typo na api estoura no decode, nunca vira DPS
  incompleta aceita pelo fisco. `nfse.DecodeEventRequest` é o equivalente para
  `Body["event"]`, exportado para a api rodar o mesmo decode nos próprios testes.
- **Regras de evento no pacote `nfse`, não em `nacional`:** `ContribuinteEvents`,
  `EventsRequiringMotivo` e `EventsRequiringXMotivo` vivem em `nfse/constants.go` porque quem monta
  o pedido (a api, que valida antes de enfileirar) e quem o serializa (`nacional.BuildPedRegEvento`)
  precisam da mesma regra — duas cópias divergiriam.
- **Serialização:** `nacional/dps.go`'s structs `encoding/xml` têm a ordem de campo normativa —
  ela É a ordem do XSD (`tiposComplexos_v1.01.xsd`); não existe tabela `xsdorder` para NFS-e como
  para os demais doc types. `TestBuildDPS_MatchesGolden` é o guarda contra reordenação acidental.
- **`Body` de `dfe.Request` para NFS-e** (chaves lidas por `nfse.Dispatch`,
  `go-dfe/nfse/dispatch.go`):

  | Chave | Serviço(s) | Uso |
  |---|---|---|
  | `provider` | todos | `"nacional"` ou `"abrasf204"` (F5) |
  | `document` | `NFSeRecepcao` | `nfse.Document` completo |
  | `event` | `NFSeEvento` | `nfse.EventRequest` completo |
  | `chave_acesso` | `NFSeConsulta`, `NFSeConsultaEvento`, `NFSeDistribuicao`*, `NFSeDANFSE` | chave de acesso da NFS-e |
  | `id_dps` | `NFSeConsultaDPS` | identificador da DPS |
  | `nsu` | `NFSeDistribuicao` | número sequencial único |
  | `cnpj_consulta` | `NFSeDistribuicao` | CNPJ de consulta (raiz do certificado) |
  | `param_kind` | `NFSeParametrosMunicipais` | `aliquota`\|`convenio`\|`beneficio`\|`regimes_especiais`\|`retencoes` |
  | `param_args` | `NFSeParametrosMunicipais` | argumentos posicionais do path (arity por `param_kind`) |

  \* `NFSeConsultaEvento` também aceita `tipo_evento`/`n_seq_evento`; sem eles a consulta lista
  todos os eventos pelo ADN.

- **Ambientes (Sistema Nacional):**

  | Sistema | Produção | Produção restrita (homologação) |
  |---|---|---|
  | Sefin | `https://sefin.nfse.gov.br/SefinNacional` | `https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional` |
  | ADN | `https://adn.nfse.gov.br/contribuintes` | `https://adn.producaorestrita.nfse.gov.br/contribuintes` |
  | DANFSE | `https://adn.nfse.gov.br/danfse` | `https://adn.producaorestrita.nfse.gov.br/danfse` |
  | Parametrização | `https://adn.nfse.gov.br/parametrizacao` | `https://adn.producaorestrita.nfse.gov.br/parametrizacao` |

  O segmento `/API` existe SÓ na produção restrita do Sefin — `go-dfe/nfse/nacional/endpoints.go`
  é a fonte de verdade em código (a tabela de ambientes de `docs/specs/2026-08-04-nfse-design.md`
  §1 foi corrigida para bater com isto).
- **`dfe.Implements(nfse, ...)`:** os 8 serviços acima estão promovidos sem shadow-mode — py-dfe
  nunca implementou NFS-e, então não há autoridade anterior para comparar. O portão aplicável é a
  homologação em produção restrita (F6), não a comparação de paridade normal.
- **Não implementado nesta fase:** ABRASF 2.04 (F5), persistência (F3), geração própria de DANFSE
  (fora de escopo — `DANFSE` baixa o PDF pronto do ADN).

---

## 4. ctech-dfe-api — REST Backend

**Location:** `/ctech-dfe-api/`
**Framework:** Fiber v3 · Go · AWS SDK v2

### Structure

```
ctech-dfe-api/
├── cmd/server/main.go          # Fiber app, dependency wiring, startup
├── internal/
│   ├── config/                 # 12-Factor env config (caarlos0/env)
│   ├── problem/                # RFC 7807 Problem type + helpers
│   ├── middleware/
│   │   ├── auth.go             # RS256 JWT validation, JWKS cached in Redis
│   │   ├── tenant.go           # Dfe-Organization-Pk header → org context
│   │   └── perm.go             # RBAC permission checker (OWNER/ADMIN bypass)
│   ├── cache/                  # Redis + in-memory backends
│   ├── ws/                     # WebSocket registry (Redis pub/sub)
│   ├── awsclient/              # AWS SDK v2 client wrappers
│   ├── repositories/           # Persistence only — DynamoDB access
│   ├── services/               # Business logic
│   │   ├── certificates.go     # CertificateService: upload (PFX parse + S3 + DynamoDB), list, delete
│   │   ├── fiscal_configs.go   # NfeConfig / NfceConfig / CteConfig / MdfeConfig
│   │   ├── nfes/               # NfeService: emit (transact_write), cancel, events
│   │   ├── organizations.go    # OrganizationService: CRUD + Redis cache
│   │   ├── persons.go          # PersonService
│   │   ├── users.go            # UserService: GetMe, AttachToOrg, Redis cache
│   │   └── vehicles.go         # VehicleService
│   └── api/v1/
│       ├── router.go           # Mounts all route groups
│       ├── auth.go             # /auth/me, /auth/roles
│       ├── organizations.go    # /organizations/*
│       ├── products.go         # /products
│       ├── persons.go          # /persons
│       ├── vehicles.go         # /vehicles
│       ├── nfes.go             # /nfes
│       ├── distributions.go    # /distributions
│       ├── external.go         # /external/lookup-organizations
│       └── helpers.go          # sendPage, sendItem, sendProblem, pagination helpers
```

### Configuration (environment variables)

```bash
# Auth (required)
# Service-to-service goes straight to the -api host (ALB), not the CloudFront app
# domain: an edge round trip buys a server in the same region nothing.
# The handler mounts /.well-known at the root, not under /v1.0.
CTECH_JWKS_URL=https://accounts-api.aoctech.app/.well-known/jwks.json

# Cache (Redis / Valkey)
VALKEY_URL=redis://...          # JWKS cached here; falls back to in-memory when unset

# AWS
AWS_REGION=us-east-1
TABLE_PREFIX=dev                # dev | staging | prod
S3_BUCKET_CERTIFICATES=dev-py-dfe-certificates
S3_BUCKET_DOCUMENTS=dev-py-dfe-documents
ENVIRONMENT=dev
PYDFE_FUNCTION_NAME=dev-py-dfe  # Python SEFAZ Lambda (Scenario A)
DFE_TOPIC_ARN=                  # SNS command topic; issuance reaches it through worker_outbox
TECHNICAL_CNPJ=62787449000107   # Technical contact CNPJ (DFe)
```

### Error handling

All errors are RFC 7807 Problem JSON. Routes call `sendProblem(c, err)` — never return raw errors.
Services return `*problem.Problem` (via `problem.BadRequest`, `problem.NotFound`, etc.) or wrap unexpected errors.

### Request validation (rigid schema enforcement)

Every mutating endpoint binds the JSON body into a **typed request DTO** and validates it
*before* any persistence. Binding is strict — unknown JSON fields are rejected
(`json.Decoder.DisallowUnknownFields`). Validation uses `go-playground/validator/v10`
through the shared instance in `internal/validation`, configured with custom rules that mirror
the frontend Zod schemas (`cpf`, `cnpj`, `cpfcnpj`, `uf`, `timezone`, `cfop`, `ncm`, `cest`,
`ibge`, `cep`, `phonebr`, `placa`, `rntrc`, `renavam`, `percent`, `money`, `ibscst`, `inscmun`,
`caepf`, `nif`, `cnae`, `tribnac`, `nbs`, `indop`, etc. — the last three consult the NFS-e
reference tables in `go-dfe/nfse/tables`, not a regex).

- DTOs live in `internal/api/v1/dto.go` (persons, organizations, products, vehicles, fiscal
  configs) and in the service packages for fiscal issuance (`NfeEmitBody`, `NfceEmitBody`,
  `MdfeEmitBody` and nested types).
- The route helper `bindJSON[T]` (and `bindAVValidated[T]`) decodes strictly + validates and
  returns a `*problem.Problem` on failure. No route calls `c.Bind().JSON` for typed bodies.
- Cross-field business rules (e.g. MDF-e modal/vehicle/owner combinations, NF-e
  receiver_id XOR self_issuance) remain in the service layer; the tags enforce
  presence/format/range only.

A validation failure returns **HTTP 422** with `type: "/problems/validation-error"` and a
field-level `errors` array:

```json
{
  "type": "/problems/validation-error",
  "title": "Validation Error",
  "status": 422,
  "detail": "the request body failed validation",
  "errors": [
    { "field": "person.addresses[0].postal_code", "message": "CEP deve ter 8 dígitos", "tag": "cep" },
    { "field": "cpf_or_cnpj", "message": "CPF/CNPJ inválido", "tag": "cpfcnpj" }
  ]
}
```

Malformed JSON (or an unknown field) returns **HTTP 400** (`/problems/bad-request`).
sqs_request(payload)

```

**AWS credential resolution order:**

1. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
2. IMDSv2 (EC2/ECS instance) with 2s timeout

### API Reference

#### Authentication

Authentication is delegated to ctech-account (accounts.aoctech.app) via OAuth 2.0 PKCE.
ctech-dfe-api has no login or registration endpoints — it only validates RS256 Bearer tokens.
Profile and password management are handled by ctech-account directly.

| Method | Endpoint           | Description                              |
|--------|--------------------|------------------------------------------|
| GET    | `/v1.0/auth/me`    | Authenticated user profile + org list    |
| GET    | `/v1.0/auth/roles` | Available RBAC roles (OWNER/ADMIN/USER/VIEWER, seeded on boot) |

#### Organizations

| Method | Endpoint                                      | Description           |
|--------|-----------------------------------------------|-----------------------|
| POST   | `/v1.0/organizations`                         | Create organization (**multipart** — KYC, see below) |
| GET    | `/v1.0/organizations`                         | List user's orgs      |
| GET    | `/v1.0/organizations/certificate-requirement?cpf_or_cnpj=` | `{required: bool}` — is an A1 upload needed to create this org |
| GET    | `/v1.0/organizations/{pk}`                    | Organization detail   |
| PUT    | `/v1.0/organizations/{pk}`                    | Update org            |
| PUT    | `/v1.0/organizations/{pk}/nfe-config`         | Configure NF-e        |
| PUT    | `/v1.0/organizations/{pk}/nfce-config`        | Configure NFC-e       |
| PUT    | `/v1.0/organizations/{pk}/cte-config`         | Configure CT-e        |
| PUT    | `/v1.0/organizations/{pk}/mdfe-config`        | Configure MDF-e       |
| POST   | `/v1.0/organizations/{pk}/certificates`       | Upload A1 certificate |
| GET    | `/v1.0/organizations/{pk}/certificates`       | List certificates     |
| DELETE | `/v1.0/organizations/{pk}/certificates/{md5}` | Remove certificate    |
| POST   | `/v1.0/organizations/{pk}/authorized-viewers` | Add SEFAZ autXML viewer (`{cpf_or_cnpj, name}`) — 400 if already at 10, 409 if CPF/CNPJ already authorized |
| DELETE | `/v1.0/organizations/{pk}/authorized-viewers/{cpf_cnpj}` | Remove autXML viewer (no-op if not present) |

**Organization creation (KYC).** `POST /organizations` is `multipart/form-data`:
`data` (JSON org body) + optional `file` (A1 PFX) + `password`. The organization, its certificate,
the founding OWNER membership, and the audit row are written in one `TransactWrite` (all-or-nothing).
An A1 certificate is **required** unless the caller already belongs to an org with the same CNPJ
root (raiz, first 8 digits) that has a valid certificate — a filial then inherits the matriz
certificate (`GET /organizations/certificate-requirement` reports which case applies). The
certificate's holder document (from its CN) must match the org's CNPJ/CPF. Creating an org whose
CNPJ already exists returns 409 unless the caller is already a member (idempotent).

`POST`/`PUT` also return 400 if `person.crt` is missing for a CNPJ, or if `person.state_registrations`
is empty for a CNPJ organization (organizations are always the fiscal emitter — see
`docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md`).

`POST /organizations` and `PUT /organizations/{pk}` (and the equivalent person endpoints below)
accept an optional `person.nfse` object with the NFS-e identity fields that don't exist on the
NF-e cadastro: `im` (inscrição municipal), `caepf`, `nif`, `c_nao_nif` (`0` não informado, `1`
dispensado, `2` não exigência), `reg_trib` (`{op_simp_nac: 1` não optante`|2` MEI`|3` ME/EPP,
`reg_ap_trib_sn` — only when `op_simp_nac=3`: `1` federais e municipal pelo SN`|2` federais pelo
SN e ISSQN por fora`|3` ambos por fora, `reg_esp_trib: 0` nenhum`|1` ato cooperado`|2` estimativa
`|3` microempresa municipal`|4` notário/registrador`|5` profissional autônomo`|6` sociedade de
profissionais`}`), and `foreign_address` for a prestador/tomador without a national address. All
optional at cadastro; required only when this person/org is used as prestador in a DPS emission
(F2+), enforced at emission time, not here.

#### Members & invitations

Org-scoped (tenant via `Dfe-Organization-Pk` header). Visibility is role-gated.

| Method | Endpoint                                             | Guard          | Description |
|--------|------------------------------------------------------|----------------|-------------|
| GET    | `/v1.0/organizations/{pk}/members`                   | OWNER or ADMIN | List members |
| DELETE | `/v1.0/organizations/{pk}/members/{user_id}`         | OWNER          | Remove member (never self, never the last OWNER) |
| PUT    | `/v1.0/organizations/{pk}/members/{user_id}/role`    | OWNER          | Change role (`{role}`, ADMIN/USER/VIEWER) |
| GET    | `/v1.0/organizations/{pk}/invitations`               | OWNER or ADMIN | List pending invitations |
| POST   | `/v1.0/organizations/{pk}/invitations`               | OWNER or ADMIN | Create invitation (`{role}`); response includes the one-time `token` |
| DELETE | `/v1.0/organizations/{pk}/invitations/{id}`          | OWNER or ADMIN | Revoke a pending invitation |

Token-addressed (auth only — the invitee is not yet a member):

| Method | Endpoint                              | Description |
|--------|---------------------------------------|-------------|
| GET    | `/v1.0/invitations/{token}`           | Non-consuming preview (`org_name`, `role`, `status`, `expired`, `already_member`) |
| POST   | `/v1.0/invitations/{token}/accept`    | Join the org — single-use, 409 if already used/expired/member |
| POST   | `/v1.0/invitations/{token}/decline`   | Decline (revokes) |

Invitations expire after 7 days, grant only ADMIN/USER/VIEWER (never OWNER), and are single-use
(enforced by a conditional TransactWrite on accept). The raw token appears only in the create
response / link; DynamoDB stores only its SHA-256.

#### Products

Use the `Dfe-Organization-Pk` header for org context.

| Method | Endpoint              | Description                    |
|--------|-----------------------|--------------------------------|
| GET    | `/v1.0/products`      | List (paginated, cursor-based) |
| POST   | `/v1.0/products`      | Create product                 |
| PUT    | `/v1.0/products/{id}` | Update product                 |
| DELETE | `/v1.0/products/{id}` | Remove product                 |

**Product schema:**

```json
{
  "code": "PROD001",
  "description": "Sample product",
  "ncm": "84714900",
  "origin": "0",
  "cest": "1234567",
  "cean": "SEM GTIN",
  "taxable_cean": "SEM GTIN",
  "unit": "UN",
  "taxable_unit": "UN",
  "value": "99.99",
  "value_resale": "89.99",
  "net_weight": "1.000",
  "gross_weight": "1.100",
  "cfop_nfce": "5405",
  "c_benef": "SEM CBENEF",
  "ext_ipi": null,
  "ind_escala": "S",
  "cnpj_fab": null,
  "ind_tot": "1",
  "icms_aliq_override": null,
  "fcp_aliq_override": null,
  "inf_ad_prod": null,
  "cfop_config": [
    {
      "cfop": "5102",
      "icms": "00",
      "icms_mod_bc": "3",
      "icms_aliq_override": null,
      "icms_fcp_override": null,
      "icms_sn_cred_aliq": null,
      "icms_ind_deduz_deson": null,
      "icms_p_red_bc": null,
      "icms_mot_des": null,
      "icms_p_dif": null,
      "icms_st_mod_bc": null,
      "icms_st_mva": null,
      "icms_st_red_bc": null,
      "icms_st_aliq": null,
      "icms_st_fcp_aliq": null,
      "pis": "07",
      "pis_aliq": null,
      "pis_aliq_unid": null,
      "cofins": "07",
      "cofins_aliq": null,
      "cofins_aliq_unid": null,
      "ipi_cst": null,
      "ipi_aliq": null,
      "is_cst": null,
      "is_aliq": null,
      "is_class_trib": null,
      "is_aliq_espec": null,
      "is_unid_trib": null,
      "ibs_cbs_cst": "400",
      "ibs_cbs_class_trib": "400001",
      "ibs_uf_aliq": "0.0000",
      "ibs_mun_aliq": "0.0000",
      "cbs_aliq": "0.0000",
      "ibs_uf_p_red": null,
      "ibs_mun_p_red": null,
      "cbs_p_red": null,
      "ibs_uf_p_dif": null,
      "ibs_mun_p_dif": null,
      "cbs_p_dif": null,
      "ibs_ind_doacao": null,
      "ibs_ad_rem": null,
      "cbs_ad_rem": null
    }
  ],
  "conversion_factors": []
}
```

**Pricing fields:**

- `value` — consumer-final price ("Preço (Consumidor Final)"). Required.
- `value_resale` — resale price ("Preço (Revenda)"). Optional.

The UI prefills the emission item unit price from these: NFC-e always uses `value`; NF-e uses
`value` when the recipient is a CPF and `value_resale` (falling back to `value` when empty) when
the recipient is a CNPJ. Products are stored as a dynamic map, so the API persists these fields
without server-side price validation.

**ICMS/FCP rate resolution:**

Rates are resolved dynamically at issuance with the following precedence:

1. `icms_aliq_override` / `fcp_aliq_override` on the product (product-specific taxation)
2. `icms_aliq_override` / `icms_fcp_override` on the `cfop_config` item (CFOP-specific rule)
3. Static table `app/constants/tax_tables.py`:
    - **Intrastate** operation: `ICMS_INTRA[dest_uf]`
    - **Interstate** with imported product (origin 1/2/6/7): `4.00%` (Resolution SF 13/2012)
    - Interstate South/Southeast → North/NE/CO: `7.00%`
    - All other interstate: `12.00%`

**Conditional rates by CST:**

| Scenario                                       | Fields used                                                                                                |
|------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| Taxed ICMS (CST 00/10/20/70/90)                | `icms_aliq_override` or UF table + `icms_mod_bc`                                                           |
| ICMS with FCP                                  | `icms_fcp_override` or UF table                                                                            |
| ICMS with ST (CST 10/30/70)                    | `icms_st_mod_bc`, `icms_st_mva`, `icms_st_aliq`, `icms_st_fcp_aliq`                                        |
| Simples with credit (CSOSN 101/201/900)        | `icms_sn_cred_aliq` (pCredSN)                                                                              |
| Simples with ST (CSOSN 201/202/203)            | `icms_st_*` same as Regular Regime                                                                         |
| Taxed PIS/COFINS (CST 01/02)                   | `pis_aliq`, `cofins_aliq`                                                                                  |
| PIS/COFINS per quantity (CST 03)               | `pis_aliq_unid`, `cofins_aliq_unid`                                                                        |
| Taxed IS (CST 000)                             | `is_aliq`, `is_class_trib` required                                                                        |
| IBS/CBS with reduction (CST 010/011)           | `ibs_uf_p_red`, `ibs_mun_p_red`, `cbs_p_red`                                                               |
| IBS/CBS with deferral (CST 200/220)            | `ibs_uf_p_dif`, `ibs_mun_p_dif`, `cbs_p_dif`                                                               |
| IBS/CBS monophase (CST 620)                    | `ibs_ad_rem`, `cbs_ad_rem`                                                                                 |
| Monophase ICMS fuel (CST 02)                   | `icms_ad_rem` (ad rem, R$/unit)                                                                            |
| Monophase ICMS with retention (CST 15)         | `icms_ad_rem`, `icms_ad_rem_reten`, `icms_p_red_ad_rem`, `icms_mot_red_ad_rem`                             |
| Monophase ICMS with deferral (CST 53)          | `icms_ad_rem`, `icms_p_dif_mono`                                                                           |
| Monophase ICMS previously collected (CST 61)   | `icms_ad_rem` (adRemICMSRet)                                                                               |
| ICMS ST previously withheld (CST 60, optional) | `icms_v_bc_st_ret`, `icms_v_icms_st_ret`, `icms_p_st`, `icms_fcp_v_bc_st_ret`, `icms_fcp_st_ret_aliq`      |
| ISSQN (services)                               | `issqn_ind_iss`, `issqn_c_list_serv`, `issqn_c_mun_fg`, `issqn_aliq`, `issqn_v_deducao`, `issqn_v_iss_ret` |

**Specific product types** (`prod_type`):

| `prod_type` | XML node | Required fields                                    | Optional fields                                                                     |
|-------------|----------|----------------------------------------------------|-------------------------------------------------------------------------------------|
| `comb`      | `comb`   | `comb_c_prod_anp`, `comb_desc_anp`, `comb_uf_cons` | `comb_codif`, `comb_p_glp`, `comb_p_gnn`, `comb_p_gni`, `comb_v_part`, `comb_p_bio` |
| `med`       | `med`    | `med_c_prod_anvisa`, `med_v_pmc`                   | `med_x_motivo_isencao` (required when `ISENTO`)                                     |

**Automatic DIFAL** (`app/services/nfes._build_icms_uf_dest`):

Calculated automatically when: CRT=3 (Regular Regime) + recipient without IE (non-taxpayer) + interstate operation +
CST ∈ {00, 20, 51, 70, 90}. Includes `ICMSUFDest` in `det.imposto` and totals in `ICMSTot`. 100% share to destination
state (EC 87/2015, effective since 2019).

**RJ + CST 40 rule** (`app/services/nfes._apply_uf_rules`):

When `emit_uf='RJ'` and `cst='40'` without a configured `icms_mot_des`, the system automatically inserts
`motDesICMS='9'` and calculates `vICMSDeson` to satisfy SEFAZ-RJ requirements.

**CFOPs with defined fiscal rules** (`app/constants/tax_tables.CFOP_FORCED_CST`):

| CFOP                | Rule                    | Forced CST      |
|---------------------|-------------------------|-----------------|
| 5910/6910/5911/6911 | Loan/commodate shipment | 50 (suspension) |
| 5920/6920           | Loan/commodate return   | 40 (exempt)     |
| 5915/6915           | Repair shipment         | 50 (suspension) |
| 5916/6916           | Repair return           | 40 (exempt)     |

#### Services (catálogo NFS-e) & Config NFS-e

Use the `Dfe-Organization-Pk` header for org context.

| Method | Endpoint                       | Description                                                                    |
|--------|---------------------------------|----------------------------------------------------------------------------------|
| GET    | `/v1.0/services`                | Listar catálogo (paginado; filtros `code`, `description`, `order_by`, `sort`, `limit`, `cursor`) |
| POST   | `/v1.0/services`                | Criar serviço                                                                    |
| GET    | `/v1.0/services/{service_id}`   | Detalhe                                                                          |
| PUT    | `/v1.0/services/{service_id}`   | Atualizar (objeto completo)                                                      |
| DELETE | `/v1.0/services/{service_id}`   | Remover                                                                          |
| GET    | `/v1.0/organizations/{pk}/nfse-config` | Config NFS-e da organização                                              |
| PUT    | `/v1.0/organizations/{pk}/nfse-config` | Upsert da config                                                          |

`ServiceBody` mirrors `ProductBody`'s pattern of one full-object payload for both create and update
(see `api/internal/api/v1/dto.go`, `ServiceBody`) — its fields cover the DPS `serv` group defaults
(`trib_nacional_code` from the Anexo B lookup, optional `iss`/`federal`/`ibs_cbs`/`tot_trib`
sub-objects). `NfseConfigBody` mirrors the other fiscal config bodies (`provider`, `environment`,
`c_loc_emi`, `serie`, numbering counters, optional `abrasf` block for the `abrasf204` provider) — it
deliberately does NOT carry the prestador's inscrição municipal or regime tributário, which live on
the organization's own `person.nfse` group instead (see Organizations above and
`docs/specs/2026-08-04-nfse-design.md` §3.2–§3.3).

**RBAC permissions:** the routes above are gated on `{list,get,create,update,delete}.organization_services`
and `{get,update}.organization_nfse_configs`. `seedRoles` grants the full
`{list,get,create,update,delete}` set for both resources (same as every resource in `roles.go`'s
generic list) — the `{get,update}` pair above is only what `/nfse-config`'s two routes actually
check, not a smaller permission set than what's seeded.

**OAuth scope families:** `dfe:organization_services:{read,write}` and `dfe:nfses:{read,write}` —
the latter also covers `nfses`, `nfse_events`, and `organization_nfse_configs` (read = list/get
across the family, write = create/update/delete), mirroring the `dfe:nfes:*`/`dfe:ctes:*`/etc.
document families.

#### Emissão de NFS-e (`NfseService.Emit`)

`api/internal/services/nfses` mirrors `NfeService.Emit` step by step (load cadastro context → build
the document → reserve the number and commit document + worker command in ONE `TransactWrite`), with
three NFS-e-specific differences:

| Difference | Why |
|---|---|
| The row's SK is the `id_dps`, not the access key | The 45-char `id_dps` is known before submission; the 50-digit access key only exists in the fisco response. `BuildIDDPS` delegates to `nacional.BuildIDDPS` so the SK and the signed `infDPS/@Id` can never diverge |
| `access_key` is NOT written on creation | Writing it empty would pollute the `access-key-index` GSI. `GetNfse` accepts either identifier and falls back to the GSI |
| `WorkerMessage.UF` is empty | NFS-e is municipal: there is no UF autorizadora. The município travels in the command's `Body.document.c_loc_emi` |

`WorkerMessage.SefazService` is `nfse.ServiceRecepcao` and `Body` is exactly what `nfse.Dispatch`
reads (`{provider, document}` — see the go-dfe section above). `nfse.Service*` are exported aliases
of `go-dfe/internal/constants`, so the api never retypes a service name.

**Field naming in NFS-e bodies:** JSON keys are English like the rest of the API
(`competence`, `provider_person_id`, `customer_id`, `intermediary_id`, `substitutes_access_key`,
`additional_info`, `service_id`, `tax_rate`). The exceptions are DPS layout codes kept under their
normative field name (`tp_emit`, `motivo_emis_ti`, `ch_nfse_rej`, `c_trib_mun`, `c_loc_emi`,
`trib_issqn`, …) — the same rule NF-e already applies to `tp_nf`/`fin_nfe`/`nat_op`. The neutral
`nfse.Document` inside `payload` is a separate contract: it mirrors the DPS 1.01 layout element
names on purpose and is not part of this convention.

**Corpo de `POST /v1.0/nfses` (`NfseEmitBody`).** Diferente da NF-e, a NFS-e tem **um** serviço por
documento — `service` é objeto, não lista.

| Campo                                 | Obrigatório | Observação                                                          |
|---------------------------------------|-------------|---------------------------------------------------------------------|
| `tp_emit`                             | sim         | 1 prestador, 2 tomador, 3 intermediário                              |
| `competence`                          | sim         | `dd/mm/aaaa`                                                         |
| `service.service_id`                  | sim         | SK em `organization_services`; o catálogo fornece os defaults        |
| `service.{description,value,tax_rate,quantity,c_trib_mun}` | não | sobrescrevem o catálogo nesta emissão              |
| `motivo_emis_ti`                      | condicional | exigido quando `tp_emit != 1`                                        |
| `provider_person_id`                  | condicional | exigido quando `tp_emit != 1` — o prestador vira pessoa do cadastro  |
| `customer_id` / `intermediary_id`     | não         | pessoas do cadastro; 404 se o id não existir                         |
| `ch_nfse_rej`                         | não         | chave da NFS-e rejeitada que esta emissão substitui                  |
| `substitutes_access_key` / `substitutes_reason` | não | preenchidos pelo próprio serviço em `POST /{id}/substitute`  |
| `additional_info`                     | não         | ≤2000 caracteres                                                     |

Três validações condicionais rejeitam antes de qualquer escrita ou chamada externa:

1. `tp_emit` 2 ou 3 exige `motivo_emis_ti` **e** `provider_person_id`.
2. Prestador com `nfse.reg_trib.op_simp_nac = 3` exige `reg_ap_trib_sn` no cadastro; prestador sem o
   grupo `nfse.reg_trib` é recusado com 400 citando o campo.
3. `provider = abrasf204` exige o grupo `abrasf` completo na config (`ErrNfseNoAbrasf`).

**Ciclo de vida da linha em `nfses`:**

| `status`     | Quem escreve | Quando                                                                 |
|--------------|--------------|------------------------------------------------------------------------|
| `pending`    | api          | na emissão, dentro da transação que reserva o número                    |
| `authorized` | worker       | resposta do fisco com a NFS-e emitida (grava `access_key` e `xml_s3_key`) |
| `rejected`   | worker       | rejeição do fisco — terminal, sem retry (não há `cStat` em NFS-e)       |
| `cancelled`  | worker       | evento 101101 registrado com sucesso                                    |

#### Eventos de NFS-e (`NfseService.SendEvent` / `Cancel` / `Substitute`)

`api/internal/services/nfses/events.go`. The event row goes to `nfse_events` with `pk = id_dps`
(not the access key — see DynamoDB-Tables.md §31) and the request is dispatched with
`SefazService: nfse.ServiceEvento` and `Body = {provider, event}`, the shape
`nfse.DecodeEventRequest` reads. `WorkerMessage.AccessKey` stays the `id_dps` (the `nfses` row SK),
while the *event itself* is addressed to the 50-digit access key, so an NFS-e without
`status == authorized` **and** a non-empty `access_key` is rejected with 400.

| Rule | Where it lives |
|---|---|
| Only the 10 contributor-emittable event types are accepted; `105104`, `105105`, `205204`, `305101`-`305103` are fisco-private and arrive only through ADN distribution | `nfse.ContribuinteEvents` — one map, read by both the api and `nacional.BuildPedRegEvento` |
| `cMotivo` / `xMotivo` requirements per event type | `nfse.EventsRequiringMotivo` / `nfse.EventsRequiringXMotivo` (moved out of `nacional` in F3 so the api can fail fast with a 400 instead of producing an asynchronously rejected event, without a second copy of the layout rule) |
| `TCInfPedReg` accepts either `CNPJAutor` or `CPFAutor`, never both | `buildEventRequest` picks by the length of the org's federal registration |

`Substitute` does **not** emit an event: substitution is a new DPS carrying the `subst` group with
the original note's access key, and the fisco generates event `105102` and cancels the original by
itself (manual do contribuinte, §1.3.2). `POST /nfses/{id}/substitute` therefore routes into `Emit`.
Requesting `105102` through the generic event endpoint returns 400 pointing at that route.

Events are published with `WorkerService.PublishWorkerEvent` (SNS directly), the same path NF-e,
NFC-e and MDF-e events already use — not the transactional outbox. The outbox exists to make fiscal
*number reservation* atomic with the worker command; an event reserves no number, and its
`operation_id` (`{table}#{access_key}`) would collide with the issuance row's. Making events
transactional is a cross-doc-type change, not an NFS-e-only one.

`ListEvents` and `GetEventXML` accept either identifier in `{id}`: `GetNfse` resolves it and the
`nfse_events` partition is always the `id_dps`.

#### Consultas de NFS-e: XML, DANFSE e parâmetros municipais

| Method | What it returns | Source |
|---|---|---|
| `ListNfses` | Page of `nfses` rows for the configured environment | `NfseRepository.ListNfses` |
| `GetNfseXML` | Authorized NFS-e XML | `xml_s3_key` → S3 (`nfse/{env}/{org_pk}/{id_dps}.xml`) |
| `GetDPSXML` | The DPS we signed and submitted | `dps_xml_s3_key` → S3 (`nfse/{env}/{org_pk}/{id_dps}_dps.xml`) |
| `GetDANFSE` | PDF proxied from the ADN | `dfe.Call` with `nfse.ServiceDANFSE`; never stored by us |
| `MunicipalParameters` | ADN municipal parametrization | `dfe.Call` with `nfse.ServiceParametrosMunicipais`, cached 6h |
| `ListDistributions` | Documents received through ADN distribution | `NfseDistributionRepository` |

Two XML attributes exist because NFS-e is the only doc type where the document we sign (the DPS) and
the document the fisco returns (the NFS-e) are different XMLs. The authorized one keeps the
`xml_s3_key` name every other doc type uses; the DPS is `dps_xml_s3_key`.

`GetDANFSE` returns **501** for `provider == abrasf204`: the ABRASF 2.04 layout defines no
standard DANFSE PDF, so this is a real capability gap in the municipality's standard, not a missing
implementation on our side (`problem.NotImplemented`, type `/problems/not-implemented`).

**Municipal-parameter cache key excludes the tenant.** The key is
`nfse:munparams:{kind}:{arg}:{arg}…` — these are public per municipality/competence, so keying by
`orgPK` would make every organization pay for the same ADN query. TTL is 6h
(`municipalParamsTTL`). Argument arity is validated against `nacional.ParamArity`, the same table the
provider uses to build the request path.

`MunicipalParameters` and `GetDANFSE` call `dfe.Call` **synchronously from the service**, not through
the worker: both are public reads with no write and no long-timeout risk. The certificate pair comes
from `ExternalService.CertificateB64` (org's first certificate, S3 read + base64), shared with the
NF-e cadastre lookup rather than re-implemented. A non-200 from go-dfe is translated by
`problemFromDfeBody`, preserving the fisco's status and detail instead of collapsing to a 500.

#### Vehicles

Organization is always resolved from the `Dfe-Organization-Pk` header, not a path parameter.
Only `plate`/`plate_uf`/`role` are required to create a vehicle — every other field is optional
and is gated per doc-type/role at emission time via the requirements endpoint below. Trailers
are ordinary vehicle rows with `role=trailer`, independently selectable — not nested under a
tractor.

| Method | Endpoint                            | Description                                                                |
|--------|--------------------------------------|-----------------------------------------------------------------------------|
| GET    | `/v1.0/vehicles`                     | List (`?plate=`, `?role=tractor\|trailer`, `?cursor=`, `?limit=`)           |
| POST   | `/v1.0/vehicles`                     | Register vehicle                                                            |
| GET    | `/v1.0/vehicles/{sk}`                | Get vehicle                                                                 |
| GET    | `/v1.0/vehicles/{sk}/requirements`   | `?doc_type=mdfe\|nfe\|cte_os&role=tractor\|trailer` → `{"missing": [...]}`  |
| PUT    | `/v1.0/vehicles/{sk}`                | Update (partial)                                                            |
| DELETE | `/v1.0/vehicles/{sk}`                | Remove                                                                      |

#### Persons (Customers/Suppliers)

| Method | Endpoint                   | Description                        |
|--------|----------------------------|------------------------------------|
| GET    | `/v1.0/persons`            | List (searchable by name/CPF/CNPJ) |
| POST   | `/v1.0/persons`            | Register — 400 if `person.crt` missing for CNPJ; 409 if CPF/CNPJ already registered in this org |
| PUT    | `/v1.0/persons/{cpf_cnpj}` | Update                             |
| DELETE | `/v1.0/persons/{cpf_cnpj}` | Remove                             |

CRT is required for a CNPJ person (same rule as organizations), but IE (`state_registrations`) is
**not** required even for a CNPJ — unlike organizations, a person is a destinatário/counterparty,
and whether they're an ICMS contributor is a per-emission choice (`indIEDest`), not a cadastro
requirement. See `docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md`.

`POST /persons` and `PUT /persons/{cpf_cnpj}` accept the same optional `person.nfse` object
described under Organizations above (`im`, `caepf`, `nif`, `c_nao_nif`, `reg_trib`,
`foreign_address`) — used when this person is the prestador/tomador of a DPS.

#### NF-e

| Method | Endpoint                                  | Description                                 |
|--------|-------------------------------------------|---------------------------------------------|
| GET    | `/v1.0/nfes`                              | List (filters: date, key, number, incoming) |
| POST   | `/v1.0/nfes`                              | Issue NF-e                                  |
| GET    | `/v1.0/nfes/{access_key}`                 | Detail                                      |
| POST   | `/v1.0/nfes/{access_key}/cancel`          | Cancel                                      |
| GET    | `/v1.0/nfes/{access_key}/xml`             | Download XML                                |
| GET    | `/v1.0/nfes/{access_key}/danfe`           | Download DANFE (future)                     |
| GET    | `/v1.0/nfes/{access_key}/events`          | List events                                 |
| GET    | `/v1.0/nfes/{access_key}/events/{sk}/xml` | Event XML                                   |

**NF-e status transitions:**

```
pending → authorized
pending → rejected
pending → failed
authorized → cancel_pending → cancelled
```

**Attribution:** every NF-e record carries `user_id`/`user_name` of the user who issued it, stamped
at `Emit` time; every event record (`nfe_events`) carries the same for whoever triggered it
(cancel/correction-letter/manifestation) — see "Audit Log" below.

**Optional issuance fields (`POST /v1.0/nfes`):**

| Field             | Type        | Description                                                |
|-------------------|-------------|------------------------------------------------------------|
| `nat_op`          | string      | Transaction nature — frontend sends a summarized CFOP description (≤60 chars; backend truncates with `…`). The UI also derives `tp_nf` from the first product's CFOP (1/2/3→0 inbound, 5/6/7→1 outbound) and blocks mixing inbound/outbound CFOPs. |
| `fin_nfe`         | "1"–"4"     | Purpose: 1=Normal, 2=Complementary, 3=Adjustment, 4=Return |
| `ind_final`       | "0"/"1"     | End consumer (default "1")                                 |
| `ind_pres`        | "1"–"4","9" | Presence: 1=In-person (default), 2=Internet, 9=Other       |
| `tp_nf`           | "0"/"1"     | Type: 0=Inbound, 1=Outbound (default)                      |
| `transport`       | object      | Carrier and vehicle (modFrete, CNPJ/CPF, plate)            |
| `cobr_fat`        | object      | Invoice: nFat, vOrig, vDesc, vLiq                          |
| `cobr_duplicatas` | list        | Installments: nDup, dVenc, vDup (max 120)                  |
| `v_troco`         | decimal     | Change amount in BRL                                       |
| `retirada`        | object      | Local de retirada (TLocal — no CEP, unlike an `AddressBody`): `cnpj`/`cpf`, `x_nome`, `x_lgr`, `nro`, `x_cpl`, `x_bairro`, `c_mun`, `x_mun`, `uf`, `fone`, `email`. Free-form per emission — org itself is the remetente for this purpose. |
| `entrega`         | object      | Local de entrega, same TLocal shape as `retirada`, scoped to the selected `receiver_id`. |
| `save_retirada_location` | bool | If `true` and `retirada` is set, best-effort appends it to `organizations.pickup_locations` (cap 5, dedup by street+number+complement) for reuse in future emissions. Never fails the emission. |
| `save_entrega_location`  | bool | Same as above, but onto `organization_persons.delivery_locations` for the selected `receiver_id`. |

Neither `retirada`/`entrega` nor `autXML` (see Organizations §) require any py-dfe change —
`xsd_order.py` already orders both. `autXML` is not a field on this body at all: it's always
pulled from the organization's `authorized_xml_viewers` and included whenever non-empty.

**Per product item fields (`products[].`):**

| Field     | Type    | Description               |
|-----------|---------|---------------------------|
| `v_frete` | decimal | Freight per item (vFrete) |
| `v_seg`   | decimal | Insurance per item (vSeg) |
| `v_outro` | decimal | Other expenses (vOutro)   |

**Per payment fields (`payments[].`):**

| Field             | Type      | Description                                           |
|-------------------|-----------|-------------------------------------------------------|
| `ind_pag`         | "0"/"1"   | 0=Upfront (default), 1=Installment                    |
| `d_pag`           | date      | Payment date (YYYY-MM-DD)                             |
| `card.tp_integra` | "1"/"2"   | 1=Integrated (TEF), 2=Not integrated (standalone POS) |
| `card.cnpj`       | string    | Payment institution CNPJ                              |
| `card.t_band`     | "01"–"99" | Card brand: 01=Visa, 02=Mastercard, 06=Elo, etc.      |
| `card.c_aut`      | string    | Authorization number                                  |

#### NFC-e (modelo 65)

NFC-e reuses the NF-e issuance builder (`BuildEnviNFe` with `model="65"`) and the
generic worker pipeline. Differences: no recipient required (consumer is optional,
CPF only); internal operations only (`idDest=1`, CFOP must start with `5`); a QR
Code (`infNFeSupl`: `qrCode` + `urlChave`) is generated in the API (QR v2.00,
online, SHA-1 with the CSC stored in `organization_nfce_configs` as
`{env}_csc` / `{env}_csc_id`); no transport or duplicatas.

| Method | Endpoint                                    | Description                                        |
|--------|---------------------------------------------|----------------------------------------------------|
| GET    | `/v1.0/nfces`                               | List (filters: date, number)                       |
| POST   | `/v1.0/nfces`                               | Issue NFC-e                                         |
| GET    | `/v1.0/nfces/{access_key}`                  | Detail                                              |
| POST   | `/v1.0/nfces/{access_key}/cancel`           | Cancel (event 110111)                              |
| POST   | `/v1.0/nfces/{access_key}/substitute`       | Cancel by substitution (event 110112, `chNFeRef`) |
| GET    | `/v1.0/nfces/{access_key}/xml`              | Download XML                                        |
| GET    | `/v1.0/nfces/{access_key}/danfce`           | Download DANFC-e PDF (py-dfe `GerarDanfe`)         |
| GET    | `/v1.0/nfces/{access_key}/events`           | List events                                         |
| GET    | `/v1.0/nfces/{access_key}/events/{sk}/xml`  | Event XML                                           |

**Issuance body (`POST /v1.0/nfces`):** `consumer_cpf?` (CPF only), `products[]`
(`product_id`, `cfop` 5xxx, `quantity`, `unit_value?`, `discount?`), `payments[]`
(`payment_type`, `value`), `additional_info?`, `nat_op?`. **Substitution body:**
`substitute_key` (44-digit access key of the already-authorized replacement NFC-e),
`justification`, `sequence_number`. Status transitions mirror NF-e
(`authorized → cancel_pending → cancelled`). Same `user_id`/`user_name` attribution as NF-e.

#### MDF-e (modelo 58 — Manifesto de Documentos Fiscais)

Authorization is **synchronous** (`MDFeRecepcaoSinc`): SEFAZ returns `protMDFe` inline, so the worker
persists the authorized status in a single pass. All MDF-e services route to **SVRS** for every UF.
Modal is **rodoviário only** in the MVP; other modais are reserved.

| Method | Endpoint                                            | Description                                       |
|--------|-----------------------------------------------------|---------------------------------------------------|
| GET    | `/v1.0/mdfes`                                        | List (filters: date, number)                      |
| POST   | `/v1.0/mdfes`                                        | Issue MDF-e                                        |
| POST   | `/v1.0/mdfes/cargo-preview`                          | Parse referenced docs → cargo preview (no persist) |
| GET    | `/v1.0/mdfes/{access_key}`                           | Detail                                             |
| GET    | `/v1.0/mdfes/{access_key}/xml`                       | Download XML                                       |
| GET    | `/v1.0/mdfes/{access_key}/damdfe`                    | Download DAMDFE PDF (py-dfe `GerarDamdfe`)         |
| POST   | `/v1.0/mdfes/{access_key}/cancel`                    | Cancel (event 110111, `justification` ≥ 15 chars) |
| POST   | `/v1.0/mdfes/{access_key}/close`                     | Encerramento (event 110112, `ibge_code`, `uf?`)   |
| POST   | `/v1.0/mdfes/{access_key}/include-condutor`          | Inclusão de condutor (event 110114)               |
| POST   | `/v1.0/mdfes/{access_key}/include-dfe`               | Inclusão de DF-e (event 110115)                   |
| GET    | `/v1.0/mdfes/{access_key}/events`                    | List events                                       |
| GET    | `/v1.0/mdfes/{access_key}/events/{sk}/xml`           | Event XML                                          |

**Issuance body (`POST /v1.0/mdfes`):**

> **All MDF-e JSON keys are English** (API always returns English field names). `drivers` (not
> `condutores`), `loadings`/`unloadings` (not `carregamento`/`descarregamento`), `route` (not
> `percurso`), `predominant` (not `prod_pred`), `bulk_cargo` (not `lotacao`), `trip_start` (dhIniViagem).

- `modal` — `"rodoviario"|"aereo"|"aquaviario"|"ferroviario"`. Only `rodoviario` is enabled for emission
  (others are modelled and dispatched but gated). Non-rodoviário payloads go in `air`/`water`/`rail`.
- `documents[]` — `{type: "nfe"|"cte", access_key, weight?}`. **Single type only** (NF-e and CT-e cannot be
  mixed). Each referenced document must already exist in the `nfes`/`ctes` table with an `xml_s3_key`.
  `weight` (kg, decimal string) is an optional gross-weight override used when the document XML carries
  no volume/peso — required in that case (emission rejects a zero-weight document).
- `uf_start?`, `uf_end?` — overrides; derived from the referenced docs when omitted.
- `route[]?` — intermediate UFs between origin and destination (exclusive).
- `loadings[]?` / `unloadings[]?` — loading/unloading municipalities `{ibge_code, city}` (ordering override;
  reorders the derived list to the supplied `ibge_code` order, unknown municipalities kept at the end).
- `vehicle` — `{sk}` (registered vehicle; the UI always uses this path) **or** manual
  `{placa, tara, uf, renavam?, cap_kg?, tp_rod?, tp_car?}`. When `sk` is given, the registered
  vehicle must already have `weight`/`wheelset`/`bodywork` set — an incomplete vehicle returns
  `400 Bad Request` naming the missing fields (see `services.Missing` /
  `GET /vehicles/{sk}/requirements`) instead of silently defaulting `tpRod`/`tpCar`. Optional
  `owner` `{cpf|cnpj, name, rntrc, ie?, uf?, tp_prop?, tp_transp?}` for a **third-party** traction
  vehicle — its presence emits `veicTracao/prop` and drives `ide/tpTransp` (CPF⇒TAC, CNPJ⇒ETC/CTC);
  omit it for carga própria (own vehicle: no `prop`, no `tpTransp` — SEFAZ rule F25/745). Owner
  must differ from the emitter (F21). This `owner` is a per-emission input, not read from the
  vehicle's own (optional, informational-only) `owner` cadastro field.
- `trailers[]?` — up to 3 `{sk}` (registered vehicles with `role=trailer`), emitted as
  `veicReboque`. Same completeness gating as `vehicle.sk` (`weight`/`cap_kg`/`bodywork` required).
- `drivers[]` — `{name, cpf}` (≥ 1 required).
- `predominant?` — override `{tp_carga, x_prod, ncm}`; otherwise auto-derived from the highest-value item.
- `bulk_cargo?` — required when exactly **one** document (carga lotação): `{cep_loading, cep_unloading, lat_*?, lon_*?}`.
- `trip_start?` — `dhIniViagem` (RFC3339).
- `rntrc?`, `ciot?`, `additional_info?`.

**`POST /v1.0/mdfes/cargo-preview`** (`{documents: [{type, access_key}]}`) returns the parsed cargo
without persisting: per-doc `{emit_name, dest_name, loading, unloading, uf_start, uf_end, weight,
has_weight, value, predominant}` plus aggregate `loadings`, `unloadings`, `uf_start`, `uf_end`,
`total_weight`, `total_value`, `predominant`. `has_weight=false` signals the frontend to collect the
gross weight from the user. The emission form drives its Carga/Trajeto steps from this endpoint.

The api downloads each referenced XML from S3 and parses it server-side: **loading** = emitter
municipality, **unloading** = recipient municipality (documents grouped per `infMunDescarga`),
**cargo weight** = Σ `transp/vol/pesoB` (NF-e) or `infCarga/infQ/qCarga` (CT-e), **predominant product**
= highest-`vProd` line item. **The `<MDFe>` document is the root node sent to py-dfe** (SEFAZ's
synchronous `MDFeRecepcaoSinc` no longer accepts the `<enviMDFe>` batch wrapper). py-dfe reorders child
elements via its `XSD_ORDER` table before signing.

**Status transitions:** `pending → authorized`; `authorized → cancel_pending → cancelled` (cancel);
`authorized → close_pending → closed` (encerramento). A rejected cancel/close reverts the document to
`authorized`. **Event-code note:** `110112` means *cancelamento por substituição* for NF-e/NFC-e but
*encerramento* for MDF-e — the worker disambiguates by `doc_type` (see `CONDUCT.md`). Same
`user_id`/`user_name` attribution as NF-e (stamped on the `mdfes` record and every `mdfe_events` row).

**Persisted `mdfes` record** (summary fields beyond the common DFe fields): `modal`, `doc_type`,
`documents[]`, `uf_start`, `uf_end`, `route[]`, `loadings[]`, `unloadings[]` (each with `access_keys[]`),
`cargo_weight`, `cargo_value`, `predominant`, `vehicle` (with `owner?` when third-party), `drivers[]`,
`trip_start?`, `bulk_cargo?`.

#### NFS-e (Sistema Nacional — DPS 1.01)

O identificador de rota (`{id}`) aceita **id_dps ou chave de acesso**: a chave de 50 dígitos só
existe depois da resposta do fisco, então a linha é chaveada por `id_dps` e `GetNfse` resolve os
dois. Toda chave JSON é em inglês; as exceções são os códigos do leiaute do DPS (`tp_emit`,
`motivo_emis_ti`, `ch_nfse_rej`, `c_trib_mun`, `cpf_ag_trib`, `id_ev_manif_rej`).

| Method | Endpoint                                          | Permissão            | Description                                     |
|--------|---------------------------------------------------|----------------------|-------------------------------------------------|
| GET    | `/v1.0/nfses`                                     | `list.nfses`         | List (filtros: `status`, `number`, `year`, `month`, `sort`) |
| POST   | `/v1.0/nfses`                                     | `create.nfses`       | Emitir NFS-e (assíncrono, 201)                  |
| GET    | `/v1.0/nfses/{id}`                                | `get.nfses`          | Detalhe                                          |
| GET    | `/v1.0/nfses/{id}/xml`                            | `get.nfses`          | XML da NFS-e autorizada (`xml_s3_key`)          |
| GET    | `/v1.0/nfses/{id}/dps-xml`                        | `get.nfses`          | XML da DPS assinada (`dps_xml_s3_key`)          |
| GET    | `/v1.0/nfses/{id}/danfse`                         | `get.nfses`          | PDF da DANFSE (proxy do ADN; 501 em abrasf204)  |
| POST   | `/v1.0/nfses/{id}/cancel`                         | `delete.nfses`       | Cancelamento (evento 101101)                     |
| POST   | `/v1.0/nfses/{id}/substitute`                     | `create.nfses`       | Substituição — nova emissão, não evento (201)   |
| POST   | `/v1.0/nfses/{id}/events`                         | `create.nfse_events` | Evento genérico do contribuinte                  |
| GET    | `/v1.0/nfses/{id}/events`                         | `get.nfse_events`    | Lista de eventos                                 |
| GET    | `/v1.0/nfses/{id}/events/{event_sk}/xml`          | `get.nfse_events`    | XML do evento                                    |
| GET    | `/v1.0/nfse/municipal-parameters/{city}/{kind}`   | `get.nfses`          | Parametrização municipal (cache 6h)              |
| GET    | `/v1.0/nfse/distributions`                        | `list.nfses`         | Documentos recebidos do ADN                      |

**Cancelamento (`POST /v1.0/nfses/{id}/cancel`):** `reason_code` (obrigatório, ≤2) —
`reason_description` (obrigatório, ≤255) — `sequence_number` (opcional, default 1).

**Evento genérico (`POST /v1.0/nfses/{id}/events`):** `event_type` (6 dígitos), `sequence_number?`,
`reason_code?`, `reason_description?`, `substitute_access_key?`, `cpf_ag_trib?`, `id_ev_manif_rej?`.
Só os tipos de `nfse.ContribuinteEvents` são aceitos; 105102 é recusado com 400 apontando para
`/substitute`, porque quem o gera é o fisco a partir de uma nova emissão. `reason_code` e
`reason_description` são exigidos conforme `nfse.EventsRequiringMotivo` / `EventsRequiringXMotivo`.

**Substituição (`POST /v1.0/nfses/{id}/substitute`):** corpo idêntico ao de emissão; o serviço
preenche `substitutes_access_key` a partir da NFS-e original e exige `substitutes_reason`.

**Parâmetros municipais:** os argumentos extras vêm por query e são posicionados conforme o `kind`
(a aridade é validada contra `nacional.ParamArity`):

| kind                | Query params usados            | args                                  |
|---------------------|--------------------------------|---------------------------------------|
| `aliquota`          | `service`, `competence`        | município, serviço, competência        |
| `convenio`          | —                              | município                              |
| `beneficio`         | `benefit_number`, `competence` | município, benefício, competência      |
| `regimes_especiais` | `service`, `competence`        | município, serviço, competência        |
| `retencoes`         | `competence`                   | município, competência                 |


#### DFe Distribution

| Method | Endpoint                                   | Description                                                         |
|--------|--------------------------------------------|---------------------------------------------------------------------|
| POST   | `/v1.0/distributions/{doc_type}/sync`      | Enqueue background distNSU (202). 429 if penalty/rate limit active. |
| GET    | `/v1.0/distributions/{doc_type}/nsu/{nsu}` | consNSU — lookup specific NSU (consumes 20/hr quota).               |
| GET    | `/v1.0/distributions/{doc_type}/key/{key}` | consChNFe — lookup by access key (consumes 20/hr quota).            |
| GET    | `/v1.0/distributions/nfe`                  | Legacy: synchronous distNSU poll (deprecated — use POST /sync).     |
| GET    | `/v1.0/distributions/nfe/history`          | List persisted distribution records (paginated, no SEFAZ call).     |

`doc_type` ∈ `{nfe, cte, mdfe}`.

**Rate limits (enforced per org/doc_type):**

- `distNSU` (POST /sync): 1 call/hour. Consumo indevido penalty extends block by 1h from last infraction.
- `consNSU` + `consChNFe`: shared quota of 20 calls/hour. Tracked in `cons_quota_calls` / `cons_quota_window_start`.

**State fields added to fiscal config tables** (`organization_nfe_configs` etc.):

| Field                     | Type | Description                                         |
|---------------------------|------|-----------------------------------------------------|
| `nsu`                     | N    | Last NSU fetched and persisted                      |
| `last_dist_nsu_at`        | S    | ISO-8601 UTC — last distNSU call timestamp          |
| `improper_usage_until`    | S    | ISO-8601 UTC — consumo indevido block expiry        |
| `cons_quota_calls`        | N    | Rolling consNSU/consChNFe call counter              |
| `cons_quota_window_start` | S    | ISO-8601 UTC — start of current 1-hour quota window |

**Counterparty persistence (worker):** When the distribution worker processes a received *processed*
NF-e/CT-e document (`procNFe`, `cteProc`), it persists the document's emitter and recipient as
`organization_persons` records (suppliers/customers) so future issuances can reuse the data. The
nested `person` object — `addresses`, `contacts` (`phones`/`emails`), `state_registrations`,
`fantasy_name`, `crt` — is extracted from the XML party (`enderEmit`/`enderDest`/`enderToma`, `IE`,
`fone`, `email`, `CRT`) and stored when present (`if they exist`), mirroring the api person model.
A party whose CPF/CNPJ equals the org's own is skipped. Records are written create-if-absent
(`attribute_not_exists(pk)`) — a manually curated person is never overwritten. MDF-e is excluded
(transport manifest, no fiscal supplier/customer). See `worker/internal/service/distribution.go`
(`persistCounterparties` / `persistPerson`) and `distribution_parser.go` (`buildPersonDetails`).
Each auto-created person also gets an `audit_logs` row (see below), attributed to `user_id=SYSTEM`
since there's no authenticated user in this background path — the person record and its audit row
are written atomically in one `TransactWriteItems` call.

#### Audit Log

Every mutating action is attributable to a user. Two mechanisms, depending on whether the
underlying record is ever overwritten in place:

- **Append-only records** (NF-e/NFC-e/CT-e/MDF-e issuance and their events — `nfes`/`nfces`/`ctes`/`mdfes`
  and `*_events` tables): the actor is stamped directly on the record as `user_id`/`user_name` at
  creation time. No separate audit row — the record itself never changes after write.
- **Mutable resources** (organizations, certificates, products, vehicles, persons, fiscal configs):
  each `Create`/`Update`/`Delete` writes a row to the `audit_logs` table (schema in
  `DynamoDB-Tables.md §23`) in the *same* DynamoDB transaction as the resource change, so the two
  can never diverge — a mutation can't commit without its audit trail, or vice versa. The row
  records only the fields that actually changed (`modifications: [{name, before, after}]`), the
  actor (`user_id`/`user_name`, resolved from the caller's JWT `sub` + a live/cached ctech-account
  profile lookup), and the action (`CREATE`/`UPDATE`/`DELETE`).

| Method | Endpoint          | Description                                                              |
|--------|-------------------|----------------------------------------------------------------------------|
| GET    | `/v1.0/audit-logs` | List audit_logs rows for the active org. **OWNER/ADMIN only** — bypasses the granular permission-string check entirely, since audit visibility is sensitive by nature. |

**Query params:** `resource_type` + `resource_id` (full history of one resource, e.g. one product's
changes over time), `user_id` (everything one user did, org-scoped), or neither (default org-wide
chronological feed). `cursor`/`limit` for pagination, same envelope as every other list endpoint.

#### Pagination

All list endpoints use cursor-based pagination:

```json
{
  "results": [
    ...
  ],
  "next_cursor": "base64-encoded-key",
  "previous_cursor": null,
  "has_next": true
}
```

#### Errors (RFC 7807)

```json
{
  "status": 400,
  "type": "about:blank",
  "title": "Bad Request",
  "detail": "Invalid CNPJ: incorrect check digit",
  "instance": "/v1.0/organizations"
}
```

### Repository Pattern

Layer rules (strictly enforced):

- **Router** — HTTP concerns only: parse request, call one service method, serialize response. No direct repo access, no
  business logic, no cache management.
- **Service** — Business logic, cache management, cross-repo orchestration. Raises `ProblemException` subclasses.
- **Repository** — DynamoDB access only. No business logic, no exceptions beyond `DynamoDBProblem`.

```python
# Repositories: persistence only
class ProductRepository(BaseRepository):
    async def get_product(self, org_pk: str, product_id: str) -> dict | None: ...

    async def get_products(self, org_pk: str, **kwargs) -> tuple[list, dict | None]: ...


# Services: business logic + caching
class ProductService:
    def __init__(self, repo: ProductRepository, cache: CacheBackend): ...

    async def create_product(self, org_pk: str, product: dict) -> dict: ...

    async def update_product(self, org_pk: str, product_id: str, updates: dict) -> dict: ...


# Routers: HTTP → service → schema
@router.post("/products")
async def create_product(body: ProductCreate, svc: ProductService = Depends(get_product_service)):
    result = await svc.create_product(org_pk, body.model_dump())
    return ProductOut(**result)
```

**Query parameter schemas** — All list endpoints use a dedicated class (in `app/schemas/query_params.py`) as a
`Depends()` parameter instead of individual `Query()` declarations:

```python
# app/schemas/query_params.py
class ProductListQuery:
    def __init__(
            self,
            limit: int = Query(default=50, ge=1, le=200),
            cursor: str | None = Query(default=None),
            description: str | None = Query(default=None),
            ...
    ): ...


# Usage in router
@router.get("")
async def list_products(q: ProductListQuery = Depends(), ...):
    items, last_key = await svc.list_products(org_pk, description=q.description, ...)
```

Key services:

| Service               | Responsibility                                                                     |
|-----------------------|------------------------------------------------------------------------------------|
| `UserService`         | user CRUD, `get_me_data()` (enriched profile), cache                               |
| `OrganizationService` | org CRUD with caching (TTL=300s)                                                   |
| `FiscalConfigService` | get/upsert for NF-e, NFC-e, CT-e, MDF-e configs with caching (TTL=300s)            |
| `CertificateService`  | upload/delete A1 certificates via S3                                               |
| `ProductService`      | product CRUD, cache (TTL=300s), cfop_config serialization                          |
| `VehicleService`      | vehicle CRUD + validation (plate, RENAVAM, RNTRC), cache (TTL=300s)                |
| `PersonService`       | person CRUD, SK generation (CPF_/CNPJ_), cache (TTL=300s)                          |
| `NfeService`          | NF-e issuance, cancellation, CCe, manifestation, XML/DANFE download, event listing |
| `ExternalService`     | SEFAZ NfeConsultaCadastro via Lambda, CPF/CNPJ + UF validation                     |

### DynamoDB storage policy — null omission

DynamoDB items omit null attributes to reduce item size. **The API contract stays nullable** — reads
reconstruct absent attributes as `null`, so responses are unchanged and clients keep sending/clearing
fields with `null`.

- **API encode:** items are encoded via `repositories.MarshalMapOmitNull`, which `Encode`/`EncodeItem`
  and `bindAV` delegate to. It recursively strips null attributes, including inside nested maps and list
  elements.
- **API updates:** `Base.UpdateItem` clears nil-valued fields with a DynamoDB `REMOVE` clause (combined
  `SET … REMOVE …`) instead of writing a `NULL` — clears the field and stores nothing.
- **Worker:** `mapToAttr` skips nil values when building items.
- **UI:** the ApiClient request interceptor strips null fields from **POST (create)** payloads only;
  `PUT`/`PATCH` keep explicit `null` (= clear the field → `REMOVE`). Non-plain bodies (FormData/Blob) are
  never stripped.

### Cache

```python
# InMemoryCache — non-distributed, TTL in seconds
# Cache is owned by each service, not by routers.
# Key patterns per service:
cache.set(f"me:{user_id}", user_data, ttl=900)
cache.set(f"user:{user_id}", user, ttl=900)
cache.set(f"org:{org_pk}", org, ttl=300)
cache.set(f"cfg:nfe:{org_pk}", config, ttl=300)
cache.set(f"res:{org_pk}:products:{product_id}", product, ttl=300)
cache.set(f"res:{org_pk}:products:list:...", paginated_result, ttl=300)
cache.set(f"res:{org_pk}:vehicles:list:...", paginated_result, ttl=300)
cache.set(f"res:{org_pk}:persons:{sk}", person, ttl=300)
cache.set(f"res:{org_pk}:persons:list:...", paginated_result, ttl=300)

# Eviction strategy: prefix-based for lists, key-based for single items.
# Mutations (create/update/delete) call cache.delete_prefix("res:{org_pk}:{resource}:")
```

---

## 5. ui — Frontend

**Location:** `/ui/`
**Framework:** Next.js 16.2.6, TypeScript, React 19

### Page Structure

```
app/
├── login/          # Authentication form
├── dashboard/      # Main panel
├── organizations/  # Manage active organization
├── nfe/            # NF-e list + issuance
│   └── detail/     # Specific NF-e details
├── nfce/           # NFC-e
├── cte/            # CT-e
├── mdfe/           # MDF-e
├── products/       # Product catalog
├── vehicles/       # Fleet
├── persons/        # Customers/suppliers
├── certificates/   # A1 certificates
└── fiscal-config/  # Issuance configuration
```

### ApiClient (`lib/api/client.ts`)

Type-safe Axios wrapper. Holds `access_token` in module-level memory (never localStorage). Injects
`Authorization: Bearer {token}` and `Dfe-Organization-Pk` headers on every request. On 401, calls the registered
`_refreshFn` to silently refresh via ctech-account before retrying once. The request interceptor also strips null
fields from POST (create) payloads (`stripNulls`/`isStrippableBody`) — see "DynamoDB storage policy — null omission".

**NF-e CFOP suffix grouping:** the emission form groups same-suffix saída CFOPs (e.g. `5920`/`6920`) into a single
dropdown option and sends the concrete intra (`5xxx`) / inter (`6xxx`) variant resolved automatically from whether the
destinatário is in the issuer's UF (re-resolved when the recipient changes). Emission is blocked when the
required-scope variant is not configured on the product. Helpers live in `lib/data/cfop.ts`
(`groupCfopConfigBySuffix`, `resolveCfopForUf`, `cfopGroupCodes`).

### Authentication Context

```typescript
// AuthContext provides:
const {user, loading, selectedOrg, login, logout, setSelectedOrg, handleCallback} = useAuth()

// OAuth PKCE flow:
// 1. login() → startOAuthFlow() → redirects to accounts.aoctech.app/v1.0/authorize
// 2. User authenticates at ctech-account → redirected to /callback?code=...&state=...
// 3. handleCallback(code, state) → exchangeCode() → POST /v1.0/token at ctech-account
// 4. access_token stored in memory; refresh_token stored in sessionStorage (pydfe_rt)
// 5. GET /auth/me → loads orgs + email; name comes from the id_token (see below)
// 6. setSelectedOrg(org) → stored in localStorage (pydfe_org); injects ORG_HEADER
// 7. 401 → tryRefresh() → doRefresh() → new tokens; retries original request
// 8. logout() → revokeToken() → clears state
```

**User name source (id_token, not `/auth/me`):** the user's name (`first_name`, `last_name`,
`username`) is decoded from the OIDC **id_token** returned by the `authorization_code` grant
(`scope=openid profile`), via `decodeIdToken()` in `lib/auth/oauth.ts` — mapping `given_name`,
`family_name`, `preferred_username`. `GET /auth/me` remains the source of truth for
**organizations and email**; its `first_name`/`last_name` fields are a **fallback** used only when
the id_token is absent or undecodable. Rationale: the DFe access token's `aud` is the DFe API, so
ctech-account's `/userinfo` rejects it — the id_token (whose `aud` is the OAuth client) is the only
profile source the UI can read. ctech-account issues a fresh id_token only on login, not on refresh;
since `refresh_token` lives in `sessionStorage` (cleared on tab close), each new session re-logs in
and picks up name changes. `AuthContext.refreshUser` preserves the id_token-derived name across
background revalidations rather than overwriting it with the backend fallback.

**Token storage:**

| Token         | Storage        | Key          |
|---------------|----------------|--------------|
| access_token  | Module memory  | —            |
| refresh_token | sessionStorage | `pydfe_rt`   |
| User data     | localStorage   | `pydfe_user` |
| Active org    | localStorage   | `pydfe_org`  |

### Form Validation

React Hook Form + Zod. Zod schemas in `lib/schemas/` mirror the backend Pydantic schemas.

### Main Dependencies

```json
{
  "@tanstack/react-query": "^5",
  "axios": "^1",
  "react-hook-form": "^7",
  "zod": "^4",
  "lucide-react": "^1"
}
```

---

## 6. ctech-dfe-worker — Async Workers

**Location:** `/ctech-dfe-worker/`
**Runtime:** AWS Lambda · Go · `provided.al2023` · binary named `bootstrap`

**Responsibility:**

- Publish immutable API command outbox rows from DynamoDB Streams to command SNS
- Consume standard SQS for at-least-once DFe issuance/event delivery (`DfeWorkerEvent`)
- Conditionally claim each document/event with an owner and six-minute processing lease before SEFAZ
- Fetch A1 certificate from S3, invoke `ctech-dfe` Python Lambda (SEFAZ XML-DSig + SOAP)
- Parse SEFAZ response, update DynamoDB (status, protocol, xml_s3_key)
- Upload signed XML to S3
- Publish terminal status to results SNS → API results consumer → Valkey/WebSocket

**Deployment:**

```
GitHub push → worker.yml
  1. test     go test ./... -race (ubuntu-24.04-arm)
  2. deploy
     a. build  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build → dist/bootstrap
     b. zip    bootstrap → worker-{version}.zip
     c. upload s3://{env}-py-dfe-deployments/worker/{version}.zip
     d. update aws lambda update-function-code (6 functions, parallel)
     e. wait   aws lambda wait function-updated
```

**Constraints (see CONDUCT.md §11):** Every SQS message may be delivered more than once. The handler must fail closed
if it cannot claim DynamoDB state, only the lease owner may finalize, and retryable infrastructure failures must not
write a terminal status. Command publication is also at-least-once because SNS publish and outbox acknowledgement are
separate operations; the worker claim is the downstream duplicate guard.

### NFS-e no worker (F3)

O roteamento não mudou: `Process` já escolhe go-dfe por `godfeImplements(msg.DocType, msg.SefazService)`, e a F2
colocou os serviços de NFS-e nesse mapa. O que muda é o tratamento da resposta — `handleSefazResponse` procura
`cStat`/`xMotivo`/`nProt`, que não existem no layout nacional. `Process` desvia para `handleNfseResponse`
(`worker/internal/service/nfse.go`) quando `isNfse(msg.DocType)`.

| Regra | Por quê |
|---|---|
| A SK da linha em `nfses` é o `id_dps` (`msg.AccessKey`); a `access_key` do fisco entra como **atributo** no mesmo update | A chave de acesso só existe na resposta; usá-la como SK criaria um item órfão ao lado do que a API criou |
| Rejeição (lista `erros` no corpo 200, ou `FiscalError` com HTTP != 200) é terminal — `failTerminal`, nunca retry | O fisco já avaliou a regra de negócio; repetir devolve a mesma recusa. Mesma regra do `cStat` da NF-e |
| O motivo gravado preserva `codigo - descricao` do fisco | É o que o usuário precisa para corrigir a DPS |
| Cancelamento aceito (evento `101101`) reverte a NFS-e para `cancelled` | `isCancellationEvent` ganhou o ramo de NFS-e: o código é `101101`, não o `110111` da NF-e |
| A notificação do evento sai de `publishEventResult`, não do status do documento | Igual ao caminho SOAP: o usuário vê o resultado do evento, não o status revertido |

Artefatos no S3 (mesma convenção `{s3_prefix}/{doc_pk com # → /}/{expected_file_name}.{ext}` dos outros tipos,
via `documentS3Key`/`putObject`, extraídos de `saveResponse` para não duplicar upload):

| Artefato | Chave | Atributo |
|---|---|---|
| NFS-e autorizada | `nfse/{env}/{org_pk}/{id_dps}.xml` | `xml_s3_key` em `nfses` |
| DPS assinada | `nfse/{env}/{org_pk}/{id_dps}_dps.xml` | `dps_xml_s3_key` em `nfses` |
| Evento | `nfse/{env}/{org_pk}/{id_dps}_{tipo_evento}_{nseq}.xml` | `xml_s3_key` em `nfse_events` |

`updateAttrs` ganhou `AccessKey` e `DPSXMLS3Key` — os dois só são preenchidos por NFS-e, e `buildUpdateExpression`
continua omitindo do SET tudo que for nil.

### Distribuição ADN (F3)

`DistributionService.Process` desvia `dist_nsu` para `runNfseDistNSU` (`worker/internal/service/distribution_nfse.go`)
quando `doc_type = nfse`. O `distribution-dispatcher` varre `{prefix}_organization_nfse_configs` e enfileira um job
por organização, sem mudança própria — só `nfse` na lista de `docTypes`.

Não reusa `runDistNSU` porque os dois só parecem o mesmo problema:

|             | NF-e / CT-e / MDF-e (`runDistNSU`)   | NFS-e (`runNfseDistNSU`)                    |
|-------------|--------------------------------------|---------------------------------------------|
| Protocolo   | SOAP `distDFeInt`                    | REST `GET /DFe/{NSU}` no ADN                |
| Paginação   | `ultNSU` + `maxNSU` na resposta      | NSU sequencial; pede-se sempre `cursor + 1` |
| Fim do lote | `cStat` 137 / 238                    | lote vazio                                  |
| Punição     | consumo indevido                     | não existe no ADN                           |
| Teto        | `ultNSU >= maxNSU`                   | `maxNfseDistBatches = 20` por invocação     |

O que é comum — `loadConfig`, `loadCert`, `getCertB64`, `claimDistNSUSlot`, `invokePyDfe`, `updateNSU` — é reusado
sem cópia.

| Regra | Por quê |
|---|---|
| Só `provider = nacional` distribui | O ADN é do Sistema Nacional; ABRASF 2.04 não tem distribuição |
| Cursor em `organization_nfse_configs` (`{env}_nsu`), igual à família NF-e | Homologação e produção têm sequências de NSU independentes |
| Erro do ADN (HTTP != 200) é terminal — loga e devolve nil | Repetir a chamada devolve a mesma recusa; só falha de rede merece retry (o `error` de `invokePyDfe`) |
| `PutItem` do NSU é condicional (`attribute_not_exists`) | SQS é at-least-once: a re-entrega não pode duplicar o registro |
| Não passa por `processDocZip` | Aquele descompacta gzip+base64 e faz parsing de `procNFe`/`resNFe`; o ADN já entrega o XML pronto em `xml` |
| `mapToDfeRequest` aceita UF vazia quando `doc_type = nfse` | NFS-e é competência municipal e viaja sem UF; o guard antigo jogaria a chamada no py-dfe, que não implementa NFS-e |

XML recebido: `nfse-distribution/{env}/{org_pk}/NSU_{015d}.xml` — mesma convenção dos outros tipos. O registro vai
para `nfse_distributions` (`pk = {env}#{org_pk}`, `sk = nsu`), que é o que `GET /nfse/distributions` lista.

---

## 8. cdk — Infrastructure

**Location:** `/cdk/`
**Language:** TypeScript, AWS CDK 2.257

### Stacks

| Stack           | File                 | Responsibility                                            |
|-----------------|----------------------|-----------------------------------------------------------|
| `OidcStack`     | `oidc-stack.ts`      | GitHub Actions OIDC provider + deploy roles (global)      |
| `DynamoDBStack` | `dynamodb-stack.ts`  | 26 tables + GSIs, including streamed worker outbox        |
| `S3Stack`       | `s3-stack.ts`        | 4 buckets: certificates, documents, deployments, logs     |
| `NetworkStack`  | `network-stack.ts`   | Dual-stack VPC, public subnets, security groups           |
| `EventBusStack` | `event-bus-stack.ts` | SNS command topic + SNS results topic + SQS results queue |
| `IAMStack`      | `iam-stack.ts`       | Lambda roles, EC2 (V1 EB + V2 custom), instance profiles  |
| `LayersStack`   | `layers-stack.ts`    | Lambda Layers scaffold                                    |
| `DfeStack`      | `dfe-stack.ts`       | py-dfe Lambda (SEFAZ) + inline layer                      |
| `WorkerStack`   | `worker-stack.ts`    | 8 fiscal workers + outbox publisher + SQS/DLQ             |
| `AlbStack`      | `alb-stack.ts`       | Dual-stack ALB without public IPv4, HTTPS listener        |
| `ApiStack`      | `api-stack.ts`       | **Legacy** — Elastic Beanstalk (migration in progress)    |
| `ApiStackV2`    | `api-v2-stack.ts`    | EC2 + ASG + Launch Template + Target Group + CW Logs      |
| `FrontendStack` | `frontend-stack.ts`  | S3 + CloudFront + Brazil geo-restriction                  |

### Network Architecture

The VPC is dual-stack IPv4 + IPv6 without a NAT Gateway. EC2 instances receive a **public IPv6 address** (free) and
**no public IPv4** (avoids $0.005/hour per IP). Outbound connectivity uses IPv6.

Free Gateway VPC Endpoints keep S3 and DynamoDB traffic inside AWS without going through the internet.

### ApiStackV2 — Custom EC2

Replaces Elastic Beanstalk. Each instance is provisioned via Launch Template with user data that:

1. Installs Python 3.14, nginx, SSM agent, CW agent
2. Creates `webapp` user (no login, no home)
3. Configures nginx on port 8080 (proxy → gunicorn :8000)
4. Creates 256 MB swap
5. Configures SSM agent with `UseDualStackEndpoint: true`
6. Creates `/opt/app/deploy.sh` and `/opt/app/upload-logs.sh`
7. Configures daily logrotate with S3 upload
8. Attempts automatic bootstrap via `s3://*/api/current.zip`

The ASG uses combined health checks: **EC2 + ELB** (`HealthChecks.withAdditionalChecks`), with `gracePeriod: 120s` to
allow bootstrap time before the ALB starts evaluating. This ensures that `AutoRollback` on Instance Refresh reacts to
both instance failures and application failures detected by the ALB.

### S3 Buckets

| Bucket                      | Lifecycle                     | Usage                           |
|-----------------------------|-------------------------------|---------------------------------|
| `{env}-py-dfe-certificates` | Expires `temp/` after 90 days | Organization A1 certificates    |
| `{env}-py-dfe-documents`    | —                             | Issued XML and fiscal documents |
| `{env}-py-dfe-deployments`  | Expires after 30 days         | API deployment artifacts        |
| `{env}-py-dfe-logs`         | — (RETAIN in prod)            | Rotated EC2 instance logs       |

### Deploy

```bash
cd cdk
npm install
cdk synth                    # Generate CloudFormation
cdk deploy --all             # Deploy all stacks
ENVIRONMENT=prod cdk deploy PyDfe-Prod-API-V2  # Specific stack
```

**Never run in production:**

```bash
cdk destroy --all  # Deletes EVERYTHING including data in dev/staging with DESTROY policy
```

---

## 9. Database

### Key Conventions

- `PK`: `USER_{uuid}`, `CNPJ_{cnpj}`, `CPF_{cpf}`, `{org_pk}`, `{access_key}`
- `SK` for documents: 44-digit access key
- `SK` for events: `{uuidv7}` (time-sortable)
- `SK` for products: `PRODUCT_{uuid}`
- `SK` for vehicles: `VEHICLE_{id}`

### GSIs per Table

```
users:                   email-index, username-index
organization_products:   code-index, description-index
organization_vehicles:   plate-index, role-index
organization_persons:    org-name-index
organization_certificates: md5-index
nfes/nfces/ctes/mdfes:   number-index, dfe-index
nfe_events/*:            org-timestamp-index
```

### Access Patterns

```python
# get_item (O(1)) — always preferred when the key is known
await repo.get_item(pk="USER_abc123")

# query with SK prefix — to list items in an org
await repo.query(pk="CNPJ_12345678901234", sk_prefix="PRODUCT_")

# GSI query — for searching by non-key attribute
await repo.query_gsi("email-index", pk="user@email.com")

# PROHIBITED in production without documented justification
await repo.scan()  # Full scan = unpredictable cost
```

### Pagination

```python
# DynamoDB LastEvaluatedKey → base64 → next_cursor in response
items, last_key = await repo.query(pk=pk, limit=50, cursor=cursor)
next_cursor = base64.b64encode(json.dumps(last_key)).decode() if last_key else None
```

---

## 10. Security

### JWT

- Issued by: ctech-account (accounts.aoctech.app)
- Algorithm: RS256
- Verification: JWKS fetched from `CTECH_JWKS_URL` (`{CTECH_URL}/.well-known/jwks.json`), cached in Valkey DB 0 for 1h
- Access token TTL: 15m
- Refresh token TTL: 30d (opaque; rotated on each use)
- Claims: `sub: ctech_user_id`, `sid: session_id`, `iat`, `exp`, `iss`, `aud`, `scope`
- No local secret key — api has no `SECRET_KEY` for JWT signing

### Passwords

- Argon2id via passlib + pwdlib
- Never stored in plain text
- Never logged or returned by the API

### A1 Certificates

- PFX stored in S3 with AWS Managed Keys
- The API never returns certificate content — only metadata (alias, md5, expires_at)
- Certificate password is never logged
- Loaded into memory only during Lambda invocation — not written to disk

### Multi-tenancy

Every authenticated endpoint follows the flow:

1. `get_current_user_id` → verifies JWT → returns `user_id`
2. `require_org_access` → verifies org membership → returns `org_pk`
3. Repository uses `org_pk` as PK — isolation by design

### RBAC

```python
Roles: OWNER | ADMIN | USER | VIEWER
Permissions: "action.resource"
# action: list | get | create | update | delete
# resource: DynamoDB table name (e.g. organization_products)

# OWNER has all permissions
# VIEWER has only list + get
```

---

## 11. Observability

### Logs

| Source           | Log Group                  | Retention         | Format |
|------------------|----------------------------|-------------------|--------|
| api (app) | `/py-dfe/{env}/app`        | 7d dev / 30d prod | text   |
| nginx access     | `/py-dfe/{env}/nginx`      | 7d dev / 30d prod | JSON   |
| nginx error      | `/py-dfe/{env}/nginx`      | 7d dev / 30d prod | text   |
| Lambda py-dfe    | `/aws/lambda/{env}-py-dfe` | Lambda default    | JSON   |

nginx uses `log_format json_log escape=json` with fields: `remote_addr`, `status`, `request`, `body_bytes_sent`,
`request_time`, `upstream_response_time`.

**CNPJ masking:** `_mask_cnpj()` in the py-dfe handler hides sensitive digits in logs.

### Log Rotation and Archiving (EC2)

Daily logrotate (`/etc/logrotate.d/py-dfe`) with `copytruncate` — files rotated on disk without restarting nginx or
gunicorn.

After rotation, `/opt/app/upload-logs.sh` packages the day's `.gz` files and uploads to:

```
s3://{env}-py-dfe-logs/api/{yyyymmdd}-{instance_id}.tar.gz
```

Local files are deleted after a successful upload. If upload fails, the file remains in `/var/log/app` or
`/var/log/nginx` until the next rotation (rotate 1).

### HTTP Metrics (CloudWatch Metric Filters)

Filters on the nginx log group — no application instrumentation required:

| Metric    | Namespace     | Filter                                      |
|-----------|---------------|---------------------------------------------|
| `HTTP2XX` | `PyDfe/{env}` | `{ ($.status >= 200) && ($.status < 300) }` |
| `HTTP3XX` | `PyDfe/{env}` | `{ ($.status >= 300) && ($.status < 400) }` |
| `HTTP4XX` | `PyDfe/{env}` | `{ ($.status >= 400) && ($.status < 500) }` |
| `HTTP5XX` | `PyDfe/{env}` | `{ $.status >= 500 }`                       |

### Other

- **Lambda Metrics:** CloudWatch Metrics (duration, errors, throttles)
- **Traces:** X-Ray (planned — not implemented)
- **Alerts:** SNS (configured in CDK for DLQ)

---

## 12. Deployment

### Environments

```
dev        → TABLE_PREFIX=dev,     S3=dev-py-dfe-*,     Lambda=dev-py-dfe
staging    → TABLE_PREFIX=staging, S3=staging-py-dfe-*, Lambda=staging-py-dfe
production → TABLE_PREFIX=prod,    S3=prod-py-dfe-*,    Lambda=prod-py-dfe
```

### py-dfe (Lambda)

```bash
cd py-dfe
pip install -e .
# Deploy via CDK — packaged as Lambda Layer
```

### ctech-dfe-api

```bash
# Local dev
cd ctech-dfe-api
go run ./cmd/server
# or
make build && ./dist/ctech-dfe-api
```

**Production (EC2 + ASG via ApiStackV2) — rolling deploy:**

```
GitHub push → api.yml
  1. test       go test ./... -race (ubuntu-24.04-arm)
  2. deploy     (depends on test)
     a. build     CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build → dist/ctech-dfe-api
     b. package   zip ctech-dfe-api + ctech-dfe-api.service → {version}.zip
     c. upload S3 api/{version}.zip  (expires 30 days via lifecycle)
                  api/current.zip   (always points to latest version)
     d. refresh   aws autoscaling start-instance-refresh
                    MinHealthyPercentage: 100   ← never reduces capacity
                    MaxHealthyPercentage: 200   ← brings up new instances before terminating old ones
                    InstanceWarmup: 300s
                    AutoRollback: true          ← reverts if health check fails
     e. wait      polling every 30s until Successful | Failed | Cancelled
```

New instances auto-bootstrap via `api/current.zip` on boot (user data) — installs binary to
`/opt/ctech-dfe-api/ctech-dfe-api` and registers the systemd service.

> The ASG uses combined EC2 + ELB health checks with `gracePeriod: 120s`. `AutoRollback` reverts the refresh if the
> instance fails either check — EC2 or ALB — within the 300s `InstanceWarmup`.

See `DEPLOYMENT.md §EC2` for diagnostic commands and log analysis.

### ui

```bash
cd ui
npm install && npm run build
# Production: S3 + CloudFront (CDK FrontendStack)
# Dev: npm run dev → http://localhost:3000
```

### cdk (Infrastructure)

**AWS Account:** `868899309401` · **Region:** `us-east-1`

```bash
cd cdk && npm install

# Bootstrap (once per account)
cdk bootstrap aws://868899309401/us-east-1

# Deploy
cdk synth                       # Verify template first
cdk deploy --all                # Full deploy
ENVIRONMENT=production TABLE_PREFIX=prod cdk deploy --all
```

See `DEPLOYMENT.md` for post-deploy verification and troubleshooting.

---

## 13. Constraints and Architectural Decisions

### Why not boto3?

`ctech-dfe-api` uses AWS SDK v2 for Go (`aws-sdk-go-v2`), not boto3.
The Python `api` (legacy) used a custom `AWSClient` with `aiohttp` + manual SigV4 because boto3 has no native
async support and adds Lambda cold start overhead. That constraint does not apply to Go.

### Why DynamoDB instead of PostgreSQL?

1. No server management (serverless)
2. Auto-scales to zero (cost savings in dev)
3. Flexible schema (NF-e fields vary by UF and type)
4. Trade-off: no JOINs; access patterns must be defined upfront

### Why async SQS worker instead of synchronous Lambda invocation?

NF-e/NFC-e/MDF-e issuance transacts the fiscal document, number reservation, and immutable `worker_outbox` command,
then returns immediately with a durable operation ID. A DynamoDB Stream publisher forwards the command through SNS
to standard SQS; `ctech-dfe-worker` claims it with a lease, invokes SEFAZ, and publishes terminal results for WebSocket.

**Benefits:** the API does not block on SEFAZ latency; a committed document cannot lose its command; SQS supplies
visibility timeout and DLQ redelivery; and the conditional worker claim, not FIFO ordering or deduplication, protects
the fiscal side effect from concurrent at-least-once delivery.

### Why Redis instead of in-memory cache?

JWKS keys and WebSocket connections are stored in Redis (Valkey). Shared across all API instances — no stale reads
or reconnect storms after rolling deploys. In-memory fallback is available for local dev (no Redis required).

### Why native WS ping/pong on the server but app-level JSON on the client?

The server (`api/internal/api/v1/ws.go`) sends a native ping control frame every 30s and enforces a
45s read-deadline via `SetPongHandler`/`SetReadDeadline` — a half-open connection (TCP reset lost
somewhere in the proxy chain) now breaks the read loop within a bounded time instead of blocking
`ReadMessage()` forever. The client side can't mirror this with native frames: the WHATWG WebSocket
API gives browser JS no way to send a ping frame itself (only Node's `ws` library exposes `.ping()`).
So the client sends its own app-level `{"type":"ping"}` every 20s and closes the socket if no
`{"type":"pong"}` arrives within 10s — the server's read loop replies to that explicitly. The hook
implementing this (`useWebSocket`) lives in the shared `@aoctech/ws-client` npm package (repo
`ctech-ws-client`), consumed by both `ctech-dfe/ui` and `ctech-wallet/ui`, so the heartbeat/backoff/
reconnect-on-token-refresh logic isn't duplicated across the two frontends.

### Why OAuth 2.0 via ctech-account?

Auth is fully delegated to ctech-account (accounts.aoctech.app): PKCE redirect flow, RS256 token issuance,
MFA, passkeys, profile management, and password changes are all handled there. ctech-dfe-api is a pure RS256 token
consumer — it has no login, registration, or profile-update endpoints.

The `access_token` is held in module-level memory in the client (never localStorage) to prevent XSS exfiltration.
The `refresh_token` lives in sessionStorage (`pydfe_rt`) — cleared when the browser tab closes — and is rotated
on every use with reuse detection on the server side.

### NF-e Access Key (44 digits)

```
cUF(2) + AAMM(4) + CNPJ(14) + mod(2) + serie(3) + nNF(9) + tpEmis(1) + cNF(8) + cDV(1)
```

Generated by the API, never by the Lambda. The check digit is calculated with modulo 11 over the preceding 43 digits.


## Production browser trust

In production, `CTECH_URL` and `CORS_ALLOWED_ORIGINS` are mandatory. The HTTP CORS middleware and WebSocket upgrade validation use the same explicit origin allowlist; missing browser Origin headers remain available to non-browser clients.


## Production browser trust

In production, CTECH_URL and CORS_ALLOWED_ORIGINS are mandatory. HTTP CORS and WebSocket upgrades use the same explicit allowlist.
