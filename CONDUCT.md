# Engineering Guidelines — py-dfe

> This document defines the engineering standards for all development within the py-dfe monorepo.
>
> These guidelines apply to all contributors, maintainers, contractors, and AI-assisted development workflows.
>
> Project-specific constraints are documented in Section 10 and must be followed in addition to the general engineering
> standards defined in this document.

---

# 1. Core Engineering Principles

- Simplicity is preferred over complexity.
- Correctness is preferred over cleverness.
- Explicit behavior is preferred over implicit behavior.
- Failures must be observable, traceable, and debuggable.
- Systems should be designed for long-term maintainability, not short-term convenience.

---

# 2. Scalability and Reliability

- All design decisions must consider behavior under 10x expected load.
- All external operations must be assumed to be unreliable (network, AWS, SEFAZ, S3, DynamoDB).
- Implement retry logic with exponential backoff for all external calls.
- Retries must be safe (idempotent operations only).
- Avoid shared mutable in-memory state in distributed environments.
- Any cache with TTL > 60s must be evaluated for distributed alternatives (e.g., ElastiCache, DAX).
- Prefer predictable failure modes over partial or silent failures.

---

# 3. Code Quality and Reuse (DRY)

Code duplication is considered a defect.

Before introducing new code:

1. Search for existing implementations.
2. Reuse existing code whenever possible.
3. Extend existing code if reuse is insufficient.
4. Parameterize existing code if behavior differs only by inputs.
5. Create new code only when no suitable alternative exists.

### Duplication examples to avoid

- Utility functions duplicated across backend, frontend, and shared modules.
- Repeated formatting logic (e.g., SEFAZ date formatting).
- Repeated business logic in different services.

### Rule of thumb

If two implementations solve the same problem, they must be unified.

---

# 4. Architecture and Design

## Layer separation (api)

- Repository layer: data persistence only
- Service layer: business logic only
- Schema layer: API contract validation only
- Route layer: request/response handling only

## Dependency management

- Use dependency injection (`Depends()`) for external dependencies.
- Do not instantiate AWS clients, repositories, or services inside routes.

## Naming conventions

- Python: `snake_case`
- TypeScript: `camelCase`
- Classes: `PascalCase`
- Constants: `UPPER_CASE`

Names must describe behavior, not implementation.

---

# 5. Verification Over Assumption

Do not assume:

- API contracts
- Database schemas
- DynamoDB table or index names
- AWS resources
- Tax XML structure (SEFAZ)
- Business rules or regulatory logic
- Environment variables

If information is not explicitly known:

1. Search the codebase.
2. Search project documentation.
3. Request clarification.

Assumptions are treated as potential defects.

---

# 6. Security and Secrets

Never commit or expose:

- AWS credentials
- JWT secrets
- Private keys or certificates (`.pfx`, `.p12`, `.pem`, `.key`)
- Real customer data (CPF/CNPJ, names, tax identifiers)
- External API tokens

## Secret management

- Production/Staging: AWS SSM Parameter Store (with decryption)
- Local development: `.env` file (gitignored)
- CI/CD: GitHub Actions secrets
- Test environments: synthetic or generated data only

---

# 7. Infrastructure and Cost Management

## General principles

- AWS usage must always consider cost impact.
- Prefer pay-per-request models when workloads are variable.
- Optimize for both performance and cost efficiency.

## DynamoDB

- `scan` is prohibited in production.
- Prefer `get_item` > `query` > `scan`.
- **Never write `NULL` attributes.** Encode items via `repositories.MarshalMapOmitNull` (or `Encode`/`EncodeItem`,
  which delegate to it — it recursively strips nulls, including nested maps/lists). Clear fields on update via
  `REMOVE` (handled by `Base.UpdateItem` for nil values). The worker's `mapToAttr` skips nil values. The UI strips
  null fields from **POST** payloads only (`PUT`/`PATCH` keep explicit `null` = clear), and must not strip non-plain
  bodies (FormData/Blob). The API contract stays nullable — reads still emit `null`.
- Use `transact_write` only when atomicity is required.
- **Auditing a mutation:** any future mutating resource that needs an `audit_logs` row MUST follow
  the pattern established for products/vehicles/persons/certificates/organizations/fiscal-configs
  (`api/internal/services/{products,vehicles,persons,certificates,organizations,fiscal_configs}.go`):
  fetch current state (for `Update`/`Delete`, to compute the diff), merge `beforeMap` with the
  caller's partial `updates` into a fresh `afterMap` *before* diffing (never `Diff(beforeMap, updates)`
  directly — a partial update map would falsely log every untouched field as "changed to null"; see
  `services.Diff`), build both the resource's own `TransactWriteItem` (via the repository's
  `Build{Create,Update,Delete,Upsert}TxItem`, a non-executing sibling of `PutItem`/`UpdateItem`/
  `DeleteItem`) and the audit row's `TransactWriteItem` (via `AuditLogRepository.BuildLogTxItem`),
  then execute both in one `Base.TransactWrite` call. Never write the resource and its audit row as
  two separate calls — a partial failure would leave a mutation with no trail, or an orphan audit
  row for a mutation that never happened.
- GSI indexes must be justified by real query patterns.
- Avoid `SELECT *`-style projections in GSIs.
- **GSI reads are eventually consistent.** Document lists (NF-e/NFC-e) query the
  `dfe-index` GSI, so an immediate refetch after a base-table write (e.g. setting
  `cancel_pending` on cancel) can still return the stale prior status. For
  transitional states the UI must patch the React Query cache optimistically
  (`setDocStatusOptimistic`) instead of invalidating the list; the final status
  arrives via the WebSocket `dfe_result` message. List-cache invalidation must use
  the 2-element prefix `['nfes'|'nfces', orgPk]` — `queryKeys.*.list(orgPk)` with an
  omitted `params` arg does NOT partial-match the paginated cache keys.
- **`dfe_result` carries `result_kind` (`document` | `event`).** A SEFAZ event
  (cancellation/encerramento) failure reverts the *document* to `authorized` in
  DynamoDB, so the worker must NOT publish that reverted document status as the
  notification — it would mask the event error as "Documento autorizado". The
  worker publishes a separate `result_kind="event"` payload from
  `publishEventResult` (carrying `event_type`, `event_sk`, the event `status`, and
  `sefaz_motive`); the internal document revert passes `notify=false` to
  `updateStatus`. The UI (`resolveDfeResultToast`) branches on `result_kind` and
  reports the event outcome, never the document status, for events.

## Pessoas / Organizações

- **Gating por doc-type (bloqueia emissão + modal, como em `services.Missing` para veículos)
  não se aplica a pessoas/organizações.** Investigado e descartado deliberadamente: endereço já é
  sempre obrigatório no cadastro, cobrindo a maioria dos requisitos do XSD; IE em destinatário é
  condicional à **operação** (`indIEDest`), não ao **cadastro**, então não há "campo faltante" pra
  bloquear. Os únicos requisitos reais e fixos (CRT, ≥1 IE) são regra de cadastro — ver
  `docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md`.
- **CRT obrigatório para CNPJ** em `organizations` e `organization_persons`
  (`services.RequirePJFields`) — validado no backend, não só no Zod do front. **IE (≥1
  `state_registrations`) obrigatória só para `organizations` CNPJ** (`services.RequireOrgIE`) —
  organização é sempre o emitente fiscal; pessoa (destinatário/counterparty) não.
- **Sem CPF/CNPJ duplicado em `organization_persons`** — `Create` usa
  `ConditionExpression: attribute_not_exists(pk)` no transact Put, mapeado para 409
  (`repositories.IsConditionFailed`). `organizations.Create` mantém get-or-return (PK já é o
  próprio CNPJ/CPF — duplicidade estruturalmente impossível, comportamento intencional).
- **Local de entrega/retirada (NF-e)** é campo livre por emissão (`NfeLocalBody`, TLocal — sem
  CEP, diferente de `AddressBody`/TEndereco), com reaproveitamento do histórico:
  `organization_persons.delivery_locations` (por destinatário) e `organizations.pickup_locations`
  (org = remetente sempre), cap 5, dedup por logradouro+número+complemento. Persistência é
  best-effort após emissão bem-sucedida — nunca derruba a emissão.
- **autXML é configuração de organização, não de emissão** — `organizations.authorized_xml_viewers`
  (cap 10, sem CPF/CNPJ duplicado) é sempre incluído no XML de NF-e quando não-vazio
  (`buildAutXML`), não é um campo do payload de `POST /nfes`.
- **Struct sem `dynamodbav` tags escrita direto via `attributevalue.Marshal` usa os nomes de campo
  Go (PascalCase), não as tags `json`.** Pegadinha real (já corrigida uma vez em
  `AuthorizedViewerEntry`/`toAuthorizedViewerMaps`): ao gravar uma lista de structs internos
  (não-DTO) direto num `map[string]any` passado pro `Update` genérico, prefira converter pra
  `map[string]any` com chaves explícitas (ou faça o round-trip JSON como em `nfeLocalToMap`) —
  nunca passe o struct typed cru.

## NFC-e (modelo 65)

- NFC-e reuses `BuildEnviNFe(..., model="65", supl)` — do not fork the builder.
- The QR Code (`infNFeSupl`) is built in the API (`nfce_qrcode.go`), not in py-dfe.
  The UF URL maps and QR v2.00 hash **must be validated against SEFAZ homologação**
  before production use.
- CSC is read from `organization_nfce_configs` as `{env}_csc` / `{env}_csc_id`.
- NFC-e is internal-only: `idDest=1`, CFOP must start with `5`, consumer is optional
  and CPF-only. Cancellation by substitution uses event `110112` (`chNFeRef` = the
  replacement NFC-e). For NF-e/NFC-e the worker treats `110111` and `110112` alike
  (doc → cancelled) — **but `110112` is NOT cancellation for MDF-e** (see below).

## DANFE rendering (py-dfe `danfe/`)

- DANFE generation (`service="GerarDanfe"`) is **pure-local**: no certificate, no
  SEFAZ. Routed in `_NFServiceClient.call` before any SEFAZ work. All content is read
  from the XML only (manual mandate) — the QR URL comes from `infNFeSupl/qrCode`,
  never recomputed.
- `GerarDanfe` is **model-dispatched** (`danfe/document.py`): mod 65 → DANFC-e
  (`danfce.py`), mod 55 → DANF-e (`nfe55.py`). Never branch on model inside a
  renderer; add the branch in the dispatcher.
- DANF-e (mod 55) uses **CODE-128** barcodes via **python-barcode** (pure-Python,
  SVG, no native binary) — distinct from NFC-e's QR (segno). FS/FS-DA contingency
  adds the 36-char "Dados da NF-e" code with a chave-style mod-11 DV (`barcode.py`).
- Two sizing modes in `render.py::htmls_to_pdf(fit_height=...)`: roll/auto-height
  (NFC-e, DANFE simplificado/etiqueta) vs fixed A4 multi-page (retrato/paisagem).
  Multi-page DANFE repeats its header via WeasyPrint running elements
  (`position: running()` + `@top-center { content: element(...) }`) and numbers
  folhas with CSS `counter(page)/counter(pages)`.
- Jinja gotcha: never name a context list `items` — `ctx.items` resolves to the
  dict's `.items()` method, not your data. The DANF-e context uses `produtos`.
- Rendering uses **WeasyPrint**, which requires native libraries (cairo, pango,
  gdk-pixbuf, glib, gobject, fonts) bundled in the Lambda layer/image. The CDK layer
  build **MUST** include them or the Lambda fails at import. Render pipeline
  (`danfe/render.py`, `danfe/qr.py`, `danfe/barcode.py`, `danfe/formatters.py`) is
  generic — reuse it for DACT-e/DAMDFE; do not fork per document.
- **DAMDFE** (`service="GerarDamdfe"`, MDF-e mod 58, `danfe/mdfe58.py`) follows the
  same pure-local pattern but routes in **`MDFeServiceClient.call`** (doc_type
  `mdfe`), not `_NFServiceClient`. The handler's no-certificate allowlist is
  `RENDER_ONLY_SERVICES` (= GerarDanfe + GerarDamdfe) — add future render services
  there, never special-case a single service name. DAMDFE renders all four modais
  (rodoviário/aéreo/aquaviário/ferroviário) from `ide/modal` via one macro set
  (`_damdfe_macros.html`); barcode = CODE-128 of the chave, QR = `qrCodMDFe`.

## MDF-e (modelo 58)

- **Authorization is synchronous** (`MDFeRecepcaoSinc`): SEFAZ returns `protMDFe`
  inline. The worker persists `authorized` in one pass — there is no separate
  "consulta recibo" poll. All MDF-e services route to **SVRS** for every UF.
- **Event-code collision — `110112`:** for MDF-e this is *Encerramento* (close),
  not cancellation. The worker disambiguates by `doc_type`:
  `isCancellationEvent(docType, eventType)` excludes `110112` when `docType == "mdfe"`,
  and `isCloseEvent(docType, eventType)` routes it to `StatusClosed`. Never key event
  semantics on the code alone. Codes: `110111` cancel, `110112` encerramento,
  `110114` inclusão condutor, `110115` inclusão DF-e, `110116` pagamento operação.
- **Status lifecycle:** `pending → authorized`; cancel → `cancel_pending → cancelled`;
  encerramento → `close_pending → closed`. A rejected cancel/close reverts to
  `authorized` (mirrors NF-e cancellation revert).
- **Event UF must be the 2-letter abbreviation, not the cUF code.** py-dfe's
  endpoint resolver keys on the UF abbreviation (`uf_auth["PI"]`), so the API must
  convert the numeric cUF embedded in the access key (`accessKey[0:2]`, e.g. `"22"`)
  via `services.UFFromCode` before sending the worker message — see
  `emitUFFromAccessKey` (`services/mdfes/events.go`). Passing the raw cUF caused a
  `KeyError` in py-dfe on cancel/encerrar. (The XML's `cOrgao` field correctly keeps
  the numeric code — only the worker `UF` field needs the abbreviation.)
- **Cargo is parsed from the referenced documents' XML server-side** (S3), never
  trusted from the client: weight = Σ `transp/vol/pesoB` (NF-e); predominant product
  = highest-`vProd` line item. Only documents present in `nfes`/`ctes` with an
  `xml_s3_key` can be manifested (distribution *resumo* records lack the detail).
- **Root node is `<MDFe>`, not `<enviMDFe>`.** SEFAZ's synchronous receiver
  (`MDFeRecepcaoSinc`) rejects the `<enviMDFe>` batch wrapper — `BuildMDFe`
  (`services/mdfes/builder.go`) emits `{MDFe: {@xmlns, infMDFe}}` and the SOAP layer
  gzips it into `mdfeDadosMsg`. (Events still use the `envEventoMDFe` batch wrapper.)
- The `<MDFe>` document is built in Go (`services/mdfes/builder.go`) as an unordered
  map; **py-dfe's `XSD_ORDER` table is authoritative for element ordering.** Any new
  MDF-e element MUST be added to `py_dfe/xmlops/xsd_order.py` — Go marshals map keys
  alphabetically, so a missing entry yields invalid XML and SEFAZ rejection.
- **All MDF-e API JSON keys are English** (the API always returns English):
  `drivers`, `loadings`/`unloadings`, `route`, `predominant`, `bulk_cargo`, `trip_start`,
  `uf_start`/`uf_end`, `ibge_code`/`city` (municipalities), `cep_loading`/`cep_unloading`.
  Do not reintroduce Portuguese keys (`condutores`, `carregamento`, `percurso`, `uf_ini`,
  `c_mun`, `cep_carrega`, etc.).
- **`tpTransp` ↔ `prop` (rule F25/cStat 745):** `ide/tpTransp` may ONLY be present when the
  traction vehicle has a third-party owner (`veicTracao/prop`). For carga própria (own
  registered vehicle, no owner) BOTH `prop` and `tpTransp` MUST be omitted — emitting
  `tpTransp` without `prop` is rejected. Derivation (`builder.go:tpTranspFor`): CPF owner ⇒
  TAC(2) [F18/743]; CNPJ owner ⇒ ETC(1) or CTC(3) [F19/744]. The owner CPF/CNPJ must differ
  from the emitter [F21/740] — enforced in `resolveOwner`.
- **Modal dispatch:** `buildInfModal` switches on the modal; `ide/modal` codes are single-digit
  (`1`-rodoviário, `2`-aéreo, `3`-aquaviário, `4`-ferroviário). Only rodoviário is enabled for
  emission (`enabledModals`); the other modals are modelled (`modals.go`) and ordered in
  `xsd_order.py` but gated at `Emit`.
- **Vehicle completeness is gated, never silently defaulted.** `organization_vehicles` only
  requires `plate`/`plate_uf`/`role` at cadastro; every other field (`weight`, `wheelset`,
  `bodywork`, `cap_kg`, ...) is optional there. `api/internal/services/vehicle_requirements.go`
  (`Missing(item, docType, role) []string`) is the **single source of truth** for which fields a
  doc-type + role actually needs — never duplicate this matrix in `ui` (call
  `GET /vehicles/{sk}/requirements` instead). `resolveVehicle`/`resolveTrailers`
  (`services/mdfes/emit.go`) call `Missing` before building XML and return `400 Bad Request`
  naming the missing fields; this replaced an earlier behavior that silently defaulted
  `tpRod`→`01`/`tpCar`→`00` when a registered vehicle omitted them — do not reintroduce that
  fallback, it masked incomplete registrations instead of prompting the user to fix them.
- **Trailers are first-class vehicles, not nested data.** A trailer is an ordinary
  `organization_vehicles` row with `role=trailer` (GSI `role-index`), independently selectable
  by any tractor — not an array nested under a parent vehicle. `MdfeEmitBody.trailers[]`
  (`{sk}`, up to 3) resolves each into `veicReboque`.
- **Vehicle `owner` (cpf_cnpj/rntrc/name/type) on `organization_vehicles` is optional fleet
  metadata only** — it is NOT the source of MDF-e's third-party `veicTracao/prop` group. `prop`
  stays a per-emission input (`MdfeEmitBody.vehicle.owner` / `MdfeOwner`) because who
  leases/operates a given truck can vary trip-to-trip even for the same plate — the same
  "varies per emission" reasoning that already excludes `condutor` from the vehicle record.

## Lambda

- Minimize cold start impact (keep bundles small).
- Avoid synchronous chaining of Lambdas.
- Prefer asynchronous workflows (SQS + worker) when possible.
- Ensure timeout alignment between caller and Lambda.

## S3

- Use S3 Standard unless lifecycle or cost analysis justifies another tier.
- Avoid repeated downloads of immutable objects (e.g., certificates).
- Use lifecycle policies for long-term retention (fiscal compliance requirements).

## Frontend

- Prefer static rendering over server-side rendering when possible.
- Avoid duplicate fetches for the same data in a single render cycle.

---

# 8. Testing Standards

All new business logic must include automated tests.

| Change Type     | Required Tests     |
|-----------------|--------------------|
| Schema change   | Unit + contract    |
| Service logic   | Unit               |
| AWS integration | Integration        |
| Fiscal issuance | Unit + integration |
| Bug fix         | Regression test    |

## Requirements

- Tests must cover success and failure cases.
- External dependencies must be mocked in unit tests.
- Integration tests must not use production resources.

## Test organization

- `unit/` → isolated logic tests
- `integration/` → AWS / system-level tests
- Frontend → component + hook tests

---

# 9. Documentation Requirements

Documentation is part of the implementation.

Work is not complete until required documentation is updated.

## Must be documented

- New API endpoints
- New schemas
- New AWS resources
- New DynamoDB tables or indexes
- New business rules
- Architectural decisions
- Workarounds or non-obvious behavior changes

## Where to document

| Change Type             | Location                       |
|-------------------------|--------------------------------|
| API changes             | DOCS.md (API Reference)        |
| Core library changes    | DOCS.md (Core Library)         |
| Database changes        | DOCS.md (Data Model)           |
| Architectural decisions | DOCS.md (Architecture)         |
| Workarounds             | CONDUCT.md (Known Constraints) |

---

# 10. Git Workflow

## Branching strategy

- `main` → production (protected)
- `develop` → integration branch
- `feature/*` → feature development
- `hotfix/*` → urgent production fixes

## Commit convention

Must follow Conventional Commits:

- `feat:` new feature
- `fix:` bug fix
- `refactor:` code restructuring
- `docs:` documentation changes
- `chore:` maintenance tasks

## Forbidden in commits

- Secrets, credentials, certificates
- Real customer data
- Debug prints (`print`, `console.log`)
- Temporary experimental code

---

# 11. Project-Specific Constraints

## py-dfe (Lambda core)

- mTLS is mandatory for all SEFAZ communication.
- Certificate handling must not be simplified.
- Retry logic applies only to network errors, not SEFAZ rejection responses.
- XML structure must follow SEFAZ specification strictly.
- Schema validation is disabled by default in production for performance reasons.

## ctech-dfe-api (Go + Fiber backend)

- Uses AWS SDK v2 for Go — do not add boto3 or any Python client.
- Auth is RS256-only. JWT `sub` claim is the ctech user ID. There is no `SECRET_KEY` — do not add HS256.
- JWKS keys are cached in Redis/Valkey (TTL 1h). Falls back to in-memory when `VALKEY_URL` is unset.
- NF-e numbering uses `transact_write` for atomicity — never replace with separate read/write.
- Organization context is passed via `PyDfe-Organization-Pk` header — never path parameters.
- All route errors go through `sendProblem(c, err)` — never return raw errors or `fiber.NewError`.
- Services return `*problem.Problem` via `problem.BadRequest/NotFound/InternalServer` helpers.
- **Every mutating endpoint binds a typed request DTO and validates it before persistence.**
  Use `bindJSON[T]` / `bindAVValidated[T]` (strict decode — unknown fields rejected — plus
  `go-playground/validator`). Never persist a raw `map[string]any` straight from the body.
  Validation rules mirror the frontend Zod schemas; add new custom rules to
  `internal/validation`, never as scattered regexes. Validation failures return HTTP 422 with a
  field-level `errors` array (`problem.Validation`); keep cross-field business rules in services.
- No goroutines inside request handlers — Fiber handles concurrency.
- Binary name in deployment zip must be `app` (CDK userdata expects `/opt/app/current/app`).
- Profile and password management endpoints do not exist — those belong to ctech-account.

## ui (Frontend)

- Auth is OAuth 2.0 PKCE via ctech-account. `login()` redirects to accounts.aoctech.app; `/callback` exchanges the code for tokens.
- `access_token` is in module-level memory only — **never write it to localStorage or sessionStorage**.
- `refresh_token` is in sessionStorage (`pydfe_rt`); it is rotated on every use and cleared on logout/tab close.
- User data (`pydfe_user`) and selected org (`pydfe_org`) are in localStorage for persistence across reloads.
- Organization selection is in-memory state (AuthContext) restored from localStorage on mount.
- **User name comes from the OIDC id_token, not `/auth/me`.** The DFe access token's `aud` is the DFe
  API, so ctech-account's `/userinfo` **rejects it** — do NOT try to fetch profile from `/userinfo`
  (from the UI or the backend; the API also has no M2M credentials for it). Name (`first_name`,
  `last_name`, `username`) is decoded from the id_token (`scope=openid profile`) via `decodeIdToken()`;
  `/auth/me` name fields are a fallback only. A fresh id_token is issued only on the `authorization_code`
  grant (login), not on refresh — acceptable because `refresh_token` is session-scoped, so each new
  session re-logs in.
- UI validation duplicates backend validation intentionally (UX vs security).
- All UI must use shared component library unless explicitly justified otherwise.
- Responsiveness across mobile/tablet/desktop is mandatory.
- **All API calls must always show a loading state.** Use skeletons for initial/inline content
  loading and spinners/progress indicators for actions (e.g., selecting recent items, clicking
  search buttons, submitting forms). Never allow empty, blank, or flickering UI during async
  operations. Background refetches (e.g., filter changes on an already-loaded list) must show a
  subtle indicator (opacity dimming, spinner in the pagination bar, etc.).
- **ESLint must pass with zero errors and zero warnings** before any commit. Run
  `npx eslint src --ext .ts,.tsx` from `ui/` and fix all reported issues.
- **All inputs that trigger API calls must debounce the `onChange` callback.** Use
  `DebouncedInput` (`@/components/ui/debounced-input`) for text inputs or the `debounceMs` prop
  on `NumericInput` (`@/components/ui/numeric-input`). Default debounce: **300 ms**. This
  prevents a request on every keystroke (e.g., number-search filters, free-text search fields).

## cdk (Infrastructure)

- Table names are environment-prefixed (`{prefix}_`).
- Development stacks use `RemovalPolicy.DESTROY`.
- Production uses `RemovalPolicy.RETAIN`.
- IAM permissions must follow least privilege principle.
- PITR (Point-in-Time Recovery) is enabled only in staging/production.

### Edge routing (CloudFront in front of the ALB)

Every app domain (`dfe`, `wallet`, `accounts`) serves the UI from S3 *and* forwards the API paths
to the shared ALB, so the browser is always same-origin and CORS never applies. The `*-api` hosts
stay public for API clients and service-to-service calls.

- **Never set `errorResponses` on a distribution that also has an API behavior.** They are
  distribution-wide, not per-behavior, and would replace the API's RFC 7807 Problem JSON on every
  403/404. Unknown UI routes are resolved instead by the `url-rewrite` CloudFront Function, which
  looks the path up in a KeyValueStore and rewrites a miss to `/404.html`.
- The route manifest is published by the **frontend workflow, right after the S3 sync**
  (`ui/scripts/publish-routes.sh`) — never at synth time, or the key set would drift from the
  objects actually in the bucket.
- **The API origin is the `*-api` domain, not the ALB DNS name.** Listener rules match on the Host
  header, and `ALL_VIEWER_EXCEPT_HOST_HEADER` makes CloudFront send the origin's domain as Host. An
  ALB-DNS origin falls through to the listener's `fixedResponse(503)`.
- API behaviors use `CACHING_DISABLED` + `ALL_VIEWER_EXCEPT_HOST_HEADER` + `ALLOW_ALL` methods.
  Origin read timeout is 60s to match nginx's `proxy_read_timeout`.
- **Service-to-service calls use the `*-api` host directly** (e.g. `CTECH_JWKS_URL`). CloudFront is
  for browsers; an edge round trip buys a server in the same region nothing.

### Client IP behind the proxies

nginx sits behind the ALB, which may sit behind CloudFront. Getting the client's IP wrong silently
breaks rate limiting — the zone still exists, it just keys on the wrong thing.

- **Any rate-limit zone keyed on `$binary_remote_addr` requires the realip module.** Without
  `set_real_ip_from`, `$remote_addr` is the ALB's private IP, so every client shares one bucket and
  the limit protects nobody. `/opt/app/update-realip.sh` (in the ASG userdata) writes
  `/etc/nginx/conf.d/realip.conf` with the VPC CIDR plus CloudFront's `CLOUDFRONT_ORIGIN_FACING`
  ranges, fetched from `ip-ranges.amazonaws.com` and refreshed by a daily systemd timer.
- **Never key on the leftmost `X-Forwarded-For` entry.** CloudFront and the ALB *append* to the
  header, so a client can prepend anything it likes. `real_ip_recursive on` walks the chain
  right-to-left and discards only trusted hops, which is what makes the result unforgeable.
- nginx **overwrites** `X-Forwarded-For` with the resolved IP (`proxy_set_header X-Forwarded-For
  $remote_addr`) rather than appending, so the Go app's `TRUSTED_PROXIES` / Fiber `c.IP()` — which
  reads the leftmost entry — cannot be fed a forged value.

## ctech-dfe-worker (Go Lambda)

- Runtime: `provided.al2023`. Binary must be named `bootstrap`.
- SQS FIFO ensures ordering per organization (`org_pk` as MessageGroupId).
- Handlers must be idempotent (at-least-once delivery guarantee).
- Invokes `ctech-dfe` Python Lambda for SEFAZ XML-DSig + SOAP — do not rewrite this path.
- After SEFAZ response: update DynamoDB, upload XML to S3, publish to Redis pub/sub.
- DLQ receives messages after max retries — monitor and alert.

---

# 12. Definition of Done

A change is not complete until:

- Code is implemented
- Relevant tests are written and passing
- Documentation is updated
- No duplication is introduced
- Security implications are reviewed
- Cost implications are reviewed
- Cross-project impact is reviewed

If any step is missing, it must be explicitly stated.