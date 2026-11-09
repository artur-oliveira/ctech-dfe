# Engineering Guidelines — ctech-dfe

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

## Error logging

- HTTP correlation and boundary logging use `gopkg.aoctech.app/api-commons/observability` and its Fiber integration.
  Every RFC 7807 response is recorded once; internal causes are attached to the problem but never serialized.

- Every `slog.Error`/`slog.Warn` (Go) or equivalent inside an `if err != nil` block MUST include the
  error itself as a field (`"err", err`). A log line that names a failure without the error that
  caused it forces a second production incident just to find out why the first one happened.
- Any HTTP/SOAP call to an external system (SEFAZ, py-dfe Lambda, ADN, NFS-e nacional) MUST log the
  raw response body on a non-2xx/failure response, not just a parsed subset (`detail`/`title`).
  Parsed fields can omit the one attribute that explains the rejection — the raw body is the only
  complete source of truth for what the other side actually said.
- Background DynamoDB writes MUST return non-idempotent failures to their queue handler. A failed
  conditional create may be treated as success only after a consistent read confirms the target
  item already exists; IAM, throttling and validation failures must reach the SQS retry/DLQ path.

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

## CSRT e CSC nunca saem da API

O CSRT (Código de Segurança do Responsável Técnico, NT 2018.005) e o CSC da NFC-e identificam o emitente perante a
SEFAZ: quem os tem assina no lugar dele. Eles entram por `PUT /…/nfe-config` e afins, mas **nunca aparecem em resposta
de API nem em log** — `redactFiscalSecrets` (`internal/api/v1/helpers.go`) os apaga no GET e no próprio PUT que os
gravou. O que viaja no XML é só o derivado: `hashCSRT = Base64(SHA1(CSRT + chave))` no `infRespTec`, e o hash do CSC no
QR Code da NFC-e — nenhum dos dois é reversível.

Ao acrescentar um segredo novo à configuração fiscal, acrescente o nome do campo a `fiscalConfigSecrets` no mesmo
commit.

## Secret management

- Production/Staging: AWS SSM Parameter Store (with decryption)
- Local development: `.env` file (gitignored)
- CI/CD: GitHub Actions secrets
- Test environments: synthetic or generated data only

## Downloads (api)

Nunca montar `Content-Disposition` concatenando parâmetro de rota. Os params chegam URL-decodados no Fiber, então
`%22`/`%0d%0a` escapam do `filename="..."` e injetam header. Use `sendXML` / `sendAttachment` (`internal/api/v1/helpers.go`),
que passam o nome por `safeFilename`.

## UF vazia em NFS-e (worker)

`mapToDfeRequest` (`worker/internal/service/godfe_shadow.go`) rejeita payload sem UF — ela endereça o webservice da
SEFAZ. NFS-e é competência municipal e viaja com `uf: ""` em toda a stack (`api/internal/services/nfses/*.go`), então
o guard tem exceção explícita para `doc_type = nfse`. Tirar a exceção não quebra a compilação: a chamada só cai
silenciosamente no py-dfe, que não implementa NFS-e. `TestMapToDfeRequest_NfseSemUF` trava os dois lados.

## SK de `nfses` é o `id_dps`, não a chave de acesso

A chave de acesso de 50 dígitos contém `nNFSe` e `cNum`, gerados pelo fisco — não existe no momento do insert. O
`id_dps` (45 caracteres, `DPS` + `cLocEmi` + `tpInsc` + `inscFederal` + `serie` + `nDPS`) é conhecido antes da chamada e
é o mesmo valor assinado no atributo `Id` do XML. A `access_key` entra como atributo quando o fisco responde e é
consultável pela GSI `access-key-index`. Nunca gravar `access_key` vazia: poluiria a GSI.
`TestNfse/EmitPersisteComSKIDDPS` e `TestNfse/GetNfsePorIDDPSEPorChave` travam os dois lados.

## `WorkerMessage.AccessKey` carrega o `id_dps` quando `DocType = "nfse"`

Nenhum campo novo foi acrescentado ao `WorkerMessage`, porque o campo já significa "a SK do documento na sua tabela" em
todos os tipos. Em NFS-e essa SK é o `id_dps`. `updateClaimedDocument` depende disso; trocar por `out.AccessKey`
produziria um item órfão.

## PK de organização não é documento fiscal

Depois do re-key do ctech-account, `organizations.pk` é o UUIDv7 estável da empresa. CPF/CNPJ vem de `tax_id`, com
fallback temporário para `cpf_or_cnpj` e, apenas em linhas legadas, para a chave `CNPJ_...`/`CPF_...`. Frontend,
builders fiscais e mensagens ao worker nunca devem formatar, exibir ou inserir a PK em XML como documento. Em NFS-e,
os campos de participante aceitam tanto o UUID da própria empresa quanto seu CPF/CNPJ, mas ambos resolvem para o mesmo
item da organização; não crie uma cópia em `organization_persons`. A distribuição no worker carrega `tax_id` (ou
`cpf_or_cnpj`) da tabela `organizations`; nunca recupere o CNPJ separando `org_pk`.

## `DocFields.Incoming == 0` é ambíguo — sempre setar `IncomingSet` junto

`Incoming == 0` significa tanto "não informado" (todo caller pré-existente, tratado como `1`/destinada por
`persistIncoming`) quanto "emitida de verdade" (import-by-XML, ver `docs/specs/2026-08-13-importacao-nfe-xml.md`).
Sempre que `Incoming = 0` for um valor real e não um default, setar `fields.IncomingSet = true` junto — caso
contrário `persistIncoming` reescreve silenciosamente para `1`.

---

# 7. Infrastructure and Cost Management

## General principles

- AWS usage must always consider cost impact.
- Prefer pay-per-request models when workloads are variable.
- Optimize for both performance and cost efficiency.
- Every stack's resources are tagged `Project=ctech-dfe` and `Environment=<env>` via
  `cdk.Tags.of(app)` in `cdk/bin/ctech-dfe-cdk.ts`, for Cost Explorer breakdown by service/component.
  New stacks need no extra tagging code — the app-level tag applies automatically. (Tags must be
  activated once as cost allocation tags in the Billing console before Cost Explorer can group by them.)

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
- **Consulta pública de CNPJ roda no browser e isolada da autenticação.** O CNPJá é a base cadastral
  do primeiro cadastro; a SEFAZ continua sendo a fonte fiscal quando há organização/certificado.
  Nunca use o Axios autenticado nem envie `Authorization`/`Dfe-Organization-Pk` ao origin público,
  nunca faça retry automático sob 429 e mantenha deduplicação/cache apenas em memória. Campos já
  editados não podem ser sobrescritos por nenhuma das fontes; conflitos precisam de revisão visível.
  Todo origin público novo também entra como literal em `extra-connect-src` no workflow do frontend.
- **Struct sem `dynamodbav` tags escrita direto via `attributevalue.Marshal` usa os nomes de campo
  Go (PascalCase), não as tags `json`.** Pegadinha real (já corrigida uma vez em
  `AuthorizedViewerEntry`/`toAuthorizedViewerMaps`): ao gravar uma lista de structs internos
  (não-DTO) direto num `map[string]any` passado pro `Update` genérico, prefira converter pra
  `map[string]any` com chaves explícitas (ou faça o round-trip JSON como em `nfeLocalToMap`) —
  nunca passe o struct typed cru.

## NFS-e (F1 — modelo de dados e cadastros; F2 — go-dfe/nfse, provider nacional)

- **O cliente HTTP mTLS tem connect timeout (`constants.DialTimeout`, 10 s) e cada tentativa REST do
  NFS-e tem teto próprio (`constants.NFSeAttemptTimeout`, 20 s).** Sem isso, uma prefeitura que
  descarta o SYN custava ~80 s (timeout de connect do SO) antes de falhar — foi o que aconteceu em
  Teresina em 2026-10-06. A cadeia de timeouts do egress depende desses valores: o pior caso do
  NFS-e é 4 tentativas × 20 s + backoff (~90 s), abaixo dos 120 s do Lambda egress.
- **POST do NFS-e nunca é reenviado após timeout ou falha de leitura da resposta** (`httpDo`): a requisição pode já ter
  chegado ao fisco, e reenviar a mesma DPS viraria rejeição por duplicidade de uma nota autorizada. Só falha de
  *connect* (`isDialError`) é retentada em POST; GET segue retentando.
- **A SK de `nfses` é o `idDPS`, nunca a chave de acesso**, porque `nNFSe` e `cNum` são gerados
  pelo fisco e a chave de acesso de 50 dígitos só existe depois da resposta. Consulta por chave
  passa pela GSI `access-key-index`.
- **O grupo `nfse` do objeto `person` é compartilhado por `organizations` e
  `organization_persons`** (`NfseInfoBody`/`NfseRegTribBody`/`NfseForeignAddressBody` em
  `api/internal/api/v1/dto.go`). `reg_trib` mora ali, e não na config da organização, porque
  quando `tpEmit` é 2 (tomador) ou 3 (intermediário) o prestador é uma pessoa do cadastro, não a
  própria organização — ver `docs/specs/2026-08-04-nfse-design.md` §3.2 e §3.3.
- **Cadastro é lido em `person`, nunca na raiz do item.** `organizations` e `organization_persons`
  gravam o DTO como veio da API: `name`/`cpf_or_cnpj` na raiz, mas `addresses`, `contacts` e `nfse`
  aninhados em `person`. `nfseGroup`, `personDoc` e `firstContact`
  (`api/internal/services/nfses/document.go`) existem por isso — a versão anterior lia
  `item["cpf_cnpj"]`/`item["addresses"]` e montava DPS sem CNPJ e sem endereço, com os testes
  passando porque as fixtures repetiam a forma errada. Fixture de cadastro em teste tem que
  espelhar o que a API grava.
- **A própria organização pode ser tomadora ou intermediária.** `customer_id`/`intermediary_id`
  com o documento da org resolvem para o item de `organizations` — ela não existe em
  `organization_persons`, então `resolvePerson` compara antes de consultar o repositório.
- **As tabelas de referência da NFS-e** (`go-dfe/nfse/tables/{trib_nacional,nbs,indop}.go`,
  `ui/src/lib/data/nfse_{trib_nacional,nbs,indop,countries,motives}.ts`) **são geradas por
  `go-dfe/nfse/tables/gen/generate.py` e versionadas — nunca edite os `.go`/`.ts` gerados à mão.**
  Regenerar quando a Receita publicar um anexo novo (`python3 go-dfe/nfse/tables/gen/generate.py`
  a partir da raiz do repo, com os anexos em `tmp/`). O código de tributação nacional é
  reconstruído a partir de ITEM/SUBITEM/DESDOBRO (`%02d%02d%02d`), nunca lido da coluna A da
  planilha — ela perde o zero à esquerda do item por ser numérica. Códigos NBS mantêm só linhas
  com descrição preenchida (a linha-exemplo `9.9999.99.99` da planilha não é um código real).
- **O stream do outbox fica só em `worker_outbox`; tabelas de documento (`nfses`, `nfse_events`,
  e as análogas de NF-e/NFC-e/CT-e/MDF-e) não têm stream** — a distribuição de eventos passa pelo
  outbox transacional, não por DynamoDB Streams por tabela.
- `api` depende de `go-dfe/nfse/tables` (mesmo mecanismo de `replace` que `worker` já usa) para
  que os validadores `tribnac`/`nbs`/`indop` (`internal/validation`) consultem a mesma fonte que
  a `go-dfe` vai usar na F2 para montar/validar o XML da DPS — nunca duplique essas tabelas.
- Os cinco serviços de config fiscal (NF-e/NFC-e/CT-e/MDF-e/NFS-e) compartilham um
  `fiscalConfigService` — ver a entrada em "ctech-dfe-api" abaixo.
- **F1 não emite nenhum documento.** `nfses`/`nfse_events` existem no schema mas nenhum código
  lê ou escreve nelas ainda (F3). Nenhuma comunicação com Sefin Nacional ou município ABRASF (F2,
  F5). Nenhuma tela nova (F4). Nenhuma validação de obrigatoriedade condicional entre campos
  (ex.: `reg_ap_trib_sn` exigido quando `op_simp_nac=3`) — essas regras dependem do contexto da
  emissão e entram junto com `NfseService.Emit` na F3.
- **NFS-e não tem portão de shadow-mode** (`go-dfe/dfe.go`'s `implemented`, F2). py-dfe nunca
  implementou NFS-e; não há autoridade anterior contra a qual comparar. O portão aplicável é a
  homologação em produção restrita (F6). Isso é uma exceção documentada à regra de promoção de
  `dfe.Implements` (`go-dfe/CLAUDE.md`), não um descuido.
- **A ordem dos campos das structs em `go-dfe/nfse/nacional/dps.go` é normativa.** Ela É a ordem
  do XSD (`tiposComplexos_v1.01.xsd`) — não existe tabela `xsdorder` para NFS-e como para os
  demais doc types. Reordenar campo de struct por estética quebra a validação no Sefin.
  `TestBuildDPS_MatchesGolden` é o guarda. Uma revisão da F2 encontrou 5 grupos (IBSCBS/valores,
  imóvel, obra, informações complementares, benefício municipal) cujo shape divergia do XSD real
  por terem sido modelados a partir de prosa do plano em vez do XSD — o XSD sempre prevalece sobre
  texto de plano/spec quando divergem (mesmo precedente da F1 com `cTribNac`).
- **DPS e pedidos de evento do Sistema Nacional declaram UTF-8 explicitamente.** O Sefin devolve
  E1229 quando o XML dentro do gzip+base64 não começa com `<?xml version="1.0" encoding="UTF-8"?>`.
  Antes da assinatura, seus textos passam por `textutil.RemoveDiacritics`; depois da assinatura,
  `xml.Header` é acrescentado uma única vez. NF-e autorizada por MT reutiliza a mesma transformação
  antes de assinar; outras UFs preservam os acentos. Enquanto o diagnóstico estiver ativo, uma
  rejeição de NFS-e registra `dpsXmlGZipB64` no CloudWatch. Esse valor contém a DPS fiscal completa,
  a assinatura e o certificado público: mantenha retenção curta, restrinja o acesso e remova o log
  assim que a causa for confirmada.
- **Campo não suportado falha explicitamente.** `nfse.FieldNotSupportedError` nomeia o campo.
  Nenhum adapter de NFS-e pode descartar dado em silêncio — vale para o ABRASF da F5 e para as
  capacidades opcionais do dispatch (distribuição, DANFSE, parâmetros municipais,
  `go-dfe/nfse/dispatch.go`).
- **O corpo da API de NFS-e é em inglês** (`competence`, `provider_person_id`, `customer_id`,
  `intermediary_id`, `substitutes_access_key`, `service_id`, `tax_rate`, `reason_code`,
  `reason_description`, `substitute_access_key`), como o resto da API. A exceção são os códigos do
  leiaute do DPS, que mantêm o nome normativo (`tp_emit`, `motivo_emis_ti`, `ch_nfse_rej`,
  `c_trib_mun`, `c_loc_emi`, `trib_issqn`, `cpf_ag_trib`, `id_ev_manif_rej`) — a mesma regra que a
  NF-e aplica em `tp_nf`/`fin_nfe`/`nat_op`. O `nfse.Document` neutro dentro de `payload` é outro
  contrato: espelha os nomes dos elementos do DPS 1.01 de propósito.
- **Competência de NFS-e é uma data civil completa, não apenas mês/ano.** `competence` entra e sai da
  API como ISO Date `AAAA-MM-DD` e alimenta `dCompet`, cuja definição é a data de início da prestação.
  Não converta esse campo por timezone. A API gera `dhEmi` separadamente no timezone da configuração
  NFS-e; configurações legadas sem o campo usam `America/Sao_Paulo` até serem salvas novamente.
- **Duplicação de NFS-e depende do snapshot imutável `emit_input`.** A API persiste junto à linha as
  referências do cadastro e os overrides usados na emissão; a UI só oferece **Duplicar** quando esse
  snapshot existe. A cópia nunca leva `substitutes_*` e avança `competence` por um mês civil, limitando
  o dia ao último dia do mês de destino. Não tente reconstruir uma emissão antiga a partir do XML ou
  do texto renderizado: isso perderia IDs e defaults do catálogo sem avisar o usuário.
- **O prazo de cancelamento da NFS-e não é um dado uniforme do ADN.** Enquanto a API não expuser uma
  regra municipal verificável, a UI deve informar que o prazo depende do município e deixar o fisco
  validar a solicitação; nunca invente um contador global ou desabilite o cancelamento por uma regra
  presumida.
- **Substituição não é evento.** `POST /nfses/{id}/substitute` entra em `NfseService.Emit` com o
  grupo `subst` preenchido; o fisco gera o evento `105102` e cancela a original por conta própria
  (manual do contribuinte §1.3.2). Pedir `105102` pelo endpoint genérico de eventos devolve 400
  apontando para essa rota — nunca monte um `EventRequest` de `105102`.
- **Eventos de NFS-e usam `PublishWorkerEvent` (SNS direto), não o outbox transacional** — o mesmo
  caminho que NF-e/NFC-e/MDF-e já usam. O outbox existe para tornar a *reserva de número fiscal*
  atômica com o comando do worker; um evento não reserva número, e o `operation_id` do outbox
  (`{tabela}#{access_key}`) colidiria com a linha da emissão, cuja `ConditionExpression` é
  `attribute_not_exists(pk)`. Tornar eventos transacionais é uma mudança para todos os doc types,
  não só NFS-e.
- **A chave de cache de parâmetros municipais NÃO inclui o tenant.** `nfse:munparams:{kind}:{args}`
  — são dados públicos por município/competência (spec §5.4). Incluir `orgPK` faria cada organização
  pagar a mesma consulta ao ADN. TTL de 6h. A aridade dos argumentos é validada contra
  `nacional.ParamArity`, a mesma tabela que o provider usa para montar o path.
- **`GetDANFSE` responde 501 para `provider == abrasf204`** (`problem.NotImplemented`): o leiaute
  ABRASF 2.04 não define PDF padronizado (spec §11). É lacuna do padrão do município, não
  implementação faltando do nosso lado — por isso 501 e não 400/500.
- **`nfses` tem DOIS ponteiros de XML**: `xml_s3_key` (a NFS-e autorizada, mesmo nome dos demais doc
  types) e `dps_xml_s3_key` (a DPS que assinamos). NFS-e é o único doc type onde o documento
  assinado e o documento autorizado são XMLs distintos. O worker deve gravar o XML retornado no S3
  antes de marcar `authorized`; sucesso fiscal sem `nfse_xml` ou sem `dps_xml` falha fechado.
- **Teresina (`cLocEmi=2211001`) usa autorizador municipal em homologação**, embora o XML seja DPS
  nacional v1.01. O endpoint publicado é registrado por município+ambiente; nunca derivar ou
  inventar o endpoint de produção a partir do hostname de homologação. O host municipal fica atrás
  de Cloudflare e desafia o `User-Agent` padrão do Go em origens AWS; o transporte REST NFS-e deve
  enviar o identificador compatível `nfseRESTUserAgent`, validado a partir do mesmo egress de produção.
- **`cTribMun` de Teresina é o 4º segmento do código completo da SEMF (3 dígitos), em par com o `cTribNac`**
  (`01.06.01.001` = `010601` + `001`). Nunca montar `cTribMun` a partir de item+subitem da LC 116 (era `101`, rejeitado).
  Fonte: planilha oficial, ver `DOCS.md` → *Catálogo de código municipal (UI)*.
- **IBS/CBS é obrigatório no catálogo de serviços NFS-e**: `c_ind_op`, `cst`, `c_class_trib`,
  `ind_dest` e `fin_nfse=0`. Não inferir classificação tributária a partir da descrição do serviço.
- **`MunicipalParameters` e `GetDANFSE` chamam `dfe.Call` direto do serviço, não pelo worker** — são
  leituras públicas, sem escrita e sem risco de timeout longo. O par certificado/senha vem de
  `ExternalService.CertificateB64`, compartilhado com a consulta de cadastro da NF-e.
- **As regras de evento moram em `go-dfe/nfse/constants.go`, não em `nacional`**
  (`ContribuinteEvents`, `EventsRequiringMotivo`, `EventsRequiringXMotivo`): a api valida antes de
  enfileirar e a `nacional.BuildPedRegEvento` valida ao serializar — duas cópias da regra
  divergiriam. Mesmo precedente de `BuildIDDPS`.
- *(Nota: desde 2026-10-06 `dfe.Call` só roda dentro da `go-dfe-egress`, sempre depois de um round-trip de JSON; as "chamadas in-process" abaixo descrevem o estado anterior e a regra de `intOf`/`strSlice` não foi reverificada.)*
- **`go-dfe/nfse/dispatch.go`'s `intOf`/`strSlice` têm que aceitar tipos nativos do Go, não só os
  pós-decode de JSON.** `dfe.Call` tem duas rotas de transporte: o worker chamando via
  `invokePyDfeLambda` (o corpo passa por `json.Marshal`/`Unmarshal`, então `int`/`[]string` viram
  `float64`/`[]any`) e chamadas in-process — `worker/internal/service/distribution_nfse.go`
  (`BodyKeyNSU` com `int64`) e `api/internal/services/nfses/municipal.go` (`BodyKeyParamArgs` com
  `[]string`) — que constroem o `map[string]any` direto, sem round-trip de JSON. Um `intOf`/
  `strSlice` que só reconhece `float64`/`[]any` lê 0/vazio em silêncio na rota in-process: a F3
  enviou a distribuição ADN sempre com NSU 0 (preso na primeira página) e consultas de parâmetro
  municipal sem argumento nenhum, sem erro nenhum — só descoberto por teste de execução real, não
  por leitura de diff. Ao adicionar uma chave nova no `body` de um serviço NFS-e, teste as duas
  rotas de transporte, não só a que o teste existente exercita.
- **`worker/internal/service/distribution_test.go`'s `init()` força `godfeImplements = false` para
  todo o pacote**, restaurando o caminho pré-cutover (mockLambda) para os testes que não têm nada a
  ver com a chamada SEFAZ em si. Isso mascara qualquer teste de NFS-e que não reverta esse stub
  explicitamente: `distribution_nfse_test.go` tinha testes cobrindo `runNfseDistNSU` só pelo caminho
  mockLambda/JSON, nunca pelo caminho in-process real que a produção de fato usa (NFS-e está em
  `dfe.Implements`) — por isso o bug do NSU `int64` (ver bullet acima) passou pela suíte inteira sem
  quebrar nenhum teste. Ao testar qualquer serviço NFS-e que rode em produção pelo caminho
  in-process, stub `godfeImplements`/`godfeCall` diretamente (como
  `TestRunNfseDistNSU_RealInProcessPath` faz) — não confie só nos testes que passam pelo mockLambda
  deste pacote.
- **O cursor de NSU da distribuição NFS-e só pode avançar depois que o item foi persistido com
  sucesso (S3 + DynamoDB).** O ADN não permite reconsultar um NSU já ultrapassado — se o upload do
  XML no S3 falhar e o cursor avançar mesmo assim, aquele documento se perde para sempre.
  `runNfseDistNSU` propaga qualquer erro de `persistNfseIncoming` (incluindo falha de S3) e de
  `updateNSU` antes de seguir para o próximo lote; nenhum dos dois pode voltar a ser um erro
  silencioso (`_ = `/log-only).
- **Histórico/download de distribuição aceitam `doc_type = nfse`; consulta síncrona por NSU ou chave
  continua exclusiva de NF-e/CT-e/MDF-e.** O ADN é acionado pelo mesmo `POST /distributions/nfse/sync`
  assíncrono, com a janela atômica de uma hora da configuração NFS-e. Não encaminhe `nfse` para os
  serviços SOAP de consulta pontual da SEFAZ.
- **`NfseServiceItem.Quantity` foi removido do corpo de `POST /v1.0/nfses`.** O layout DPS 1.01 não
  tem campo de quantidade no grupo de serviço (só valor total, `vServ`) — o campo existia na
  validação e na doc (`DOCS.md`) mas nunca teve destino em `buildServico`/`buildValores` nem em
  `nfse.Document`, então qualquer valor enviado era descartado em silêncio. Não reintroduza esse
  campo sem antes confirmar contra o XSD (`tmp/nfse-esquemas_xsd-.../DPS_v1.01.xsd`) onde ele se
  encaixaria.

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

## Forma de emissão (tpEmis) e contingência

- **`tpEmis` está na chave de acesso** (posição 35). Resolva a forma de emissão **antes** de gerar a
  chave — chave e `ide/tpEmis` divergentes são rejeição certa. `generateAccessKey` (NF-e/NFC-e) e
  `services.GenerateAccessKey` (CT-e/MDF-e) exigem o `tpEmis` como argumento justamente para que
  ninguém esqueça.
- **`ide/dhCont` + `ide/xJust` são obrigatórios sempre que `tpEmis != 1`** e proibidos quando
  `tpEmis == 1`. Passe por `nfes.EmissionMode`; nunca escreva os dois campos à mão. O MDF-e é a
  exceção: tem `tpEmis` 1/2/3 mas o layout não tem o grupo de contingência.
- **Não existe seleção automática de contingência ainda.** `NormalEmission` é sempre usada. Não
  introduza um checkbox manual de contingência: entrar em contingência sem necessidade produz
  documento em modo errado. A decisão pertence à máquina de estados da fase C2.

## Inutilização de numeração (NF-e / NFC-e)

- **Não tem tabela própria.** As linhas vivem em `nfe_events` / `nfce_events`, com `pk` sintético
  `INUT#{env}#{org_pk}` e `event_type = INUT`. Foi o reuso deliberado: o worker `nfe-inutilization`
  já tinha IAM sobre essas tabelas (`cdk/lib/worker-definitions.ts`) e o caminho genérico de evento
  do worker se aplica sem alteração. Não crie `nfe_inutilizations`.
- **Rotas antes de `/:access_key`.** Em Fiber a rota paramétrica captura `inutilizations` se vier
  primeiro. Qualquer rota literal nova nesses routers segue a mesma regra.
- **cStat `102` é sucesso** (*Inutilização de número homologado*) e está em `authorizedStats`
  (`worker/internal/service/helpers.go`). Ao contrário da autorização, a resposta não traz `infProt`.
- **`infInut` não tem choice CNPJ|CPF** — só `CNPJ`. Emitente pessoa física não consegue inutilizar;
  isso é limite do layout SEFAZ, não do produto, e a API responde `400` explicando.
- **cStat `102` é sucesso, e um evento rejeitado é terminal.** Os dois já morderam em produção:
  `102` fora de `authorizedStats` marcou faixa homologada como `rejected` (2026-08-27), e
  `rejected` fora de `eventTerminalStatuses` fazia a mensagem reentregar até a DLQ, porque
  `claimProcessing` não consegue reivindicar esse status nem o reconhecia como concluído.
- **A inutilização homologada gera `ProcInutNFe`**, montado por `xmlops.BuildProcessedXML`. Nunca
  guarde só a resposta: sem o request assinado junto, não há como comprovar a inutilização depois.
- **Número que gerou documento utilizável nunca é inutilizado.** Só `rejected` e `failed` liberam o
  número; a validação roda antes de qualquer chamada à SEFAZ.
- **No tempo real, `event_type: INUT` é um caso próprio — não um cancelamento.** O reuso do caminho
  genérico de evento faz o resultado chegar como `result_kind: event`, e o frontend tem que
  desambiguar por `event_type` em dois pontos: o texto do toast
  (`ui/src/lib/utils/dfe-result-toast.ts`) e a invalidação de cache
  (`ui/src/lib/hooks/useRealtimeUpdates.ts`, que invalida `inutilizations.list`/`.gaps`, não as
  queries de documento — a `access_key` é sintética). Faltando isso, a notificação dizia
  "NFC-e cancelada" e a lacuna fechada continuava na tela. Todo evento novo que não seja evento
  *sobre um documento* tem o mesmo dever.

## Evento de documento com emitente pessoa física

- `infEvento` é `choice CNPJ | CPF`. O elemento sai de `services.IssuerDocTag(orgPK)`, derivado do
  prefixo do PK da organização — nunca escreva `"CNPJ"` fixo num builder de evento. Produtor rural e
  MEI pessoa física ficavam sem cancelamento, sem CC-e e sem manifestação por causa disso.

## Auxiliary-document rendering (API `services/documents`)

- DANFE, DANFC-e, DAMDFE, and DACTE are rendered in-process by Folio from the stored XML;
  never reintroduce a Lambda, headless browser, or network asset dependency.
- Cache keys must include tenant PK, renderer schema version, document type,
  access key, state, and the `format` parameter (empty format keeps the key without suffix).
  `format` is validated by `documents.ValidateFormat` before any S3 read; the formats of each type live in
  `templateByFormat` (`formats.go`). DACTE: the FS-DA legend is applied to offline (tpEmis 2) by analogy. A template/context compatibility change
  must increment `cacheSchemaVersion`; otherwise S3 can serve stale layouts.
- Every cached PDF is tagged `cache=auxiliary-document`. Keep the CDK tag filter,
  30-day current-version expiry, noncurrent-version cleanup, and the API role's
  `s3:PutObjectTagging` permission in sync.
- Preserve `If-None-Match: *` and per-process `singleflight`: two API replicas may
  still render the same first miss, but only one can create the cache object.
- Keep Folio resource limits and `StrictAssets`. QR/CODE-128 data must come from
  the XML, and FS/FS-DA keeps the 36-digit Dados da NF-e code with mod-11 DV.
- Gonja imported macros cannot call sibling imported macros. The renderer's
  preprocessing is intentionally limited to `fld`/`title`; add an end-to-end
  Folio render test for any macro/template change.

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
- **`tpTransp` ↔ `prop` (rule F20/cStat 745):** `ide/tpTransp` may ONLY be present when the
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
- **Vehicle `owner` (cpf_cnpj/rntrc/name/type) on `organization_vehicles` é o *default* do
  `veicTracao/prop`, nunca a palavra final.** `MdfeEmitBody.vehicle.owner` continua vencendo,
  porque quem opera um caminhão pode mudar de viagem para viagem. Duas regras não negociáveis: um
  cadastro incompleto (falta `cpf_cnpj`, `rntrc` ou `name`) é ignorado — `prop` pela metade é
  rejeição da SEFAZ; e proprietário cadastrado **igual ao emitente** é frota própria, não vira
  `prop` e deixa `ide/tpTransp` intacto (F18/F19/F20).

## Cadastros reutilizáveis na emissão

- **A ordem de resolução da tributação é única e vive em `nfes.resolveCfopTax`.** 6 níveis (+ erro):
  `cfop_config`+UF → `cfop_config` → vínculo produto→perfil (`overrides`)+UF → vínculo+UF-menos →
  perfil+UF → perfil. Nenhum outro ponto do código pode mesclar campos tributários: duas ordens de
  precedência produzem notas diferentes para o mesmo cadastro, e a divergência só aparece na
  auditoria. Ver DOCS.md "Ordem de resolução na emissão" para o detalhamento dos 7 níveis.
- **A tabela de alíquota ICMS por NCM (`icmsNcmTable`, `nfes/tax_tables.go`) é a única fonte de
  verdade.** Vivia só no frontend (`icms_ncm_lookup.ts`, removido) — não reintroduzir uma cópia lá;
  o frontend consulta `GET /v1.0/tax-tables/icms-aliq` quando precisa do valor resolvido.
- **`services.ResolveCFOPScope` é a fonte da verdade do escopo do CFOP (5/6/7); o TypeScript é só
  exibição.** A tabela de casos vive em `api/internal/services/testdata/cfop_scope_cases.json` e é
  lida pelos testes das duas linguagens — mudou a regra, muda o JSON, e o teste de paridade do
  front quebra junto.
- **Query com `FilterExpression` pagina até encher a página.** O `Limit` do DynamoDB conta itens
  *lidos*, não devolvidos, e o filtro roda depois da condição de chave: uma página curta não
  significa fim da lista — só a ausência de `LastEvaluatedKey` significa. Ver
  `services.MaxFilteredPageRoundTrips` (teto de idas ao banco por página).
- **Papel de pessoa (`roles`) é filtro de cadastro, não validação de emissão.** Nenhum caminho de
  emissão checa papel: exigir `carrier` para ser transportador quebraria toda emissão de quem nunca
  preencheu o campo.

## Lambda

- Minimize cold start impact (keep bundles small).
- Avoid synchronous chaining of Lambdas.
- Prefer asynchronous workflows (SQS + worker) when possible.
- Ensure timeout alignment between caller and Lambda.

## SQS

- **Every queue MUST set `receiveMessageWaitTime` (long polling), 20s by default.** A queue with the
  CDK default (`0` = short polling) makes the Lambda event-source poller call `ReceiveMessage`
  continuously even when the queue is empty, burning the 1M/month free-tier request quota in days on
  an otherwise idle system (observed: ~172k requests/day/idle, exhausting the free tier by day 6). Long
  polling makes each empty `ReceiveMessage` block up to the wait time instead of returning immediately,
  cutting request volume by up to 20x with no functional change. Applies to both the main queue and its
  DLQ in `cdk/lib/worker-stack.ts` and `cdk/lib/event-bus-stack.ts`.

## Results consumer (api)

- `ResultsConsumer.dispatch` (api/internal/consumer/results.go) accepts a bare `org_pk` when
  `doc_pk` is absent — the distribution worker's `new_distribution_*` messages
  (`worker/internal/service/distribution.go` `notifyResult`) never set `doc_pk`, only `org_pk`.
  Before 2026-08, this silently dropped every one of those messages and the "Nova NF-e recebida"
  toast never fired. Any future message type reaching this consumer must carry at least one of
  the two — `dispatch` treats both as valid client identifiers.
- `dispatch` only defaults `event["type"]` to `"dfe_result"` when the message doesn't already set
  its own `type` — a message that arrives with a `type` already set (e.g. `new_distribution_nfe`)
  keeps it. A future message type must set its own `type` if it needs the frontend
  (`useRealtimeUpdates.ts`) to route it differently from the generic `dfe_result` handling.

## S3

- Use S3 Standard unless lifecycle or cost analysis justifies another tier.
- Avoid repeated downloads of immutable objects (e.g., certificates).
- Use lifecycle policies for long-term retention (fiscal compliance requirements).
- **Staging keys carry no env prefix.** Convergence for a persisted document is
  `{doc_type}/{env}/{org_pk}/{access_key}.xml`, but XML-import staging (`DistributionService.ImportXML`) uses
  `{doc_type}-import-staging/{org_pk}/{uuid}.xml` — no `hom`/`prod` segment. The staging object is an ephemeral
  waiting area deleted right after the worker (`runImportXML`) processes it; the worker re-derives the env from
  the fiscal config at that point, so the staging path doesn't need to encode it.

## Frontend

- Prefer static rendering over server-side rendering when possible.
- Avoid duplicate fetches for the same data in a single render cycle.
- **Texto de status usa `text-warning` / `text-success`, nunca `amber-600` / `green-600`.** Os
  defaults do Tailwind falham AA nos tamanhos usados (amber-600 ≈ 3.19:1, green-600 ≈ 3.35:1 sobre
  branco). `globals.css` ancora `--color-warning` em amber-700 e `--color-success` em green-700,
  como já fazia com `--color-danger` (red-600) e `--color-gray-400` (slate-500). Estados de saldo
  também carregam um glifo (`✓` / `⌛` / `↩`) para não depender só de cor.
- **Nada de "eyebrow" (`text-xs uppercase tracking-wider`) como título de seção.** Rótulo de seção
  é `text-sm font-medium text-gray-600`. O eyebrow repetido em toda seção é ruído, não hierarquia.
- **Alvos de toque ≥ 44px em ações primárias.** Não use `size="sm"` (h-7) em botão de barra de
  ação — o `size` default já é `min-h-11 sm:h-8`. A partir de 2026-08-08 esse padrão
  (`min-h-11 sm:min-h-0 sm:h-8`, ou `sm:h-7` no `size="sm"` do `Select`) também é obrigatório em
  `Input`, `CurrencyInput`, `NumericInput`, `Combobox`, `NcmCombobox` e `Select`/`OptionsSelect` —
  não só em `Button`. Um controle de formulário novo que herde altura fixa em `h-8` sem o par
  responsivo `min-h-11 sm:...` reintroduz o mesmo bug (32px em mobile, abaixo do mínimo).
- **`Combobox`/`OptionsSelect` mostram sempre a descrição completa (`label`) no controle
  fechado, nunca só o código.** O campo `display` (que permitia truncar para um código curto)
  foi removido de `ComboboxOption`, `PaymentOption` e do tipo inline do `OptionsSelect` em
  2026-08-08 — reabrir a combo para lembrar o que foi selecionado é uma falha de usabilidade em
  formulário fiscal. Não reintroduza um campo de exibição compacta; se um caso realmente precisar
  de texto curto, isso é uma decisão de produto nova, não um valor de campo por callsite.
- **`Combobox` tem navegação por teclado e semântica ARIA equivalentes ao `OptionsSelect`.**
  Desde 2026-08-08 o gatilho expõe `role="combobox"`/`aria-haspopup`/`aria-expanded`/
  `aria-controls`, a lista tem `role="listbox"` e cada item `role="option"`/`aria-selected`, e
  Seta-baixo/Seta-cima/Enter navegam e selecionam sem mouse. Um `Combobox` novo herda isso do
  componente — não reimplemente a lista sem essa navegação.
- **Campo opcional de formulário ganha o X de limpar sozinho.** `Combobox` e `OptionsSelect` usam
  `useClearable` (`components/ui/form.tsx`): dentro de um `FormItem`, rótulo sem asterisco final
  (`Campo *` é obrigatório) mostra o X (`ClearButton`, que reserva `pr-14` no gatilho para não
  cobrir o texto truncado). Fora de formulário (filtros) não aparece; a prop `clearable` força.
  Obrigatório sem asterisco no rótulo vira bug de rótulo, não de componente.
- **Sem travessão (`—`, `–`) em texto visível**, inclusive rótulos de domínio. Separação (código/descrição) vira bullet `·` (`1 · Texto`); pausa em texto vira `;`.
  `go-dfe/nfse/tables/gen/generate.py` normaliza os rótulos do catálogo de enums (`label_no_dash`) antes de gerar **os dois lados**, Go (`enums.go`) e TS (`nfse_enums.ts`); `TestEnumCatalogMatchesTypeScript` exige que sejam o mesmo dado. Rode `go test -count=1`: o cache do Go não enxerga o arquivo da UI que o teste lê.
- **Uma emissão, um fluxo por documento.** NF-e é wizard; NFC-e é tela única de balcão. Não
  unifique os fluxos — unifique os componentes (`ProductSearch`, `ProductLineItem`, `EmitError`,
  `EmitConfirmModal`, `useEmitDraft`, `lib/data/payment-options`). Ver DOCS.md §5.
- **Nenhum formulário de emissão adiciona dados por conta própria.** Não pré-preencha produto,
  destinatário ou pagamento a partir do catálogo: o documento é fiscal e irreversível.
- **Domínio fechado ou entidade existente usa picker, nunca texto livre.** Código fiscal, país,
  CNAE, NBS, motivo normativo e referência a outro documento devem vir de `OptionsSelect`,
  `Combobox` ou busca de entidade, com opções da tabela oficial versionada e com escopo da
  organização quando aplicável. Texto livre fica reservado a conteúdo realmente autoral
  (descrição/observação/nome), sempre com limite explícito e normalização de quebra de linha.
  A varredura de 2026-08-07 confirmou que CT-e ainda não possui formulário de emissão e que o
  MDF-e já restringe UF, CEP, veículo e documento; somente nome de motorista permanece livre por
  ser dado autoral, enquanto CPF usa entrada numérica limitada.
- Rascunhos de emissão são locais (`useEmitDraft` → localStorage) e nunca aplicados automaticamente:
  o usuário escolhe retomar ou descartar.
- **Status de DF-e vem de `lib/data/dfe_status.ts`, e só de lá.** Rótulo, tom, pulso e título do
  modal de motivo — documento e evento, todos os tipos — via `DfeStatusBadge` / `DfeStatusCell`.
  Nada de mapa local de status em página de detalhe nem de badge por documento: foi assim que a UI
  ficou sem `processing` e com um `retry` que o backend nunca produziu. Status desconhecido renderiza
  o valor cru, nunca "Desconhecido". `retryable_failed` é aviso (âmbar, pulsando), não falha
  terminal — o worker reprocessa sozinho. Ver DOCS.md §5 e `ui/DESIGN.md`.
- **`useWebSocket` lives in the shared `@aoctech/ws-client` package (repo `ctech-ws-client`), not
  locally** — it's also consumed by `ctech-wallet/ui`. Do not fork or re-add a local copy; extend
  the shared package instead. WS heartbeat contract: the server (`api/internal/api/v1/ws.go`)
  sends a native ping every 30s and enforces a 45s read-deadline via `SetPongHandler` — the browser
  answers this transparently, no app code involved. The client can't send native ping frames
  (WHATWG constraint), so it sends its own app-level `{"type":"ping"}` every 20s and closes the
  socket if no `{"type":"pong"}` reply arrives within 10s; the server's read loop must reply to
  that. `ctech-wallet/api`'s `ws.go` only implements the pong-reply half of this (native-frame
  ping/pong there is an open follow-up, see `docs/specs/2026-07-18-websocket-resilience-design.md`).
  `subscribeAccessToken` (`client.ts`) notifies the hook on every new access token (login + silent
  refresh) so it reconnects immediately instead of holding a stale token.

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

## Rotas novas entram na spec OpenAPI na mesma mudança

A spec em `api/internal/api/v1/openapi/*.yaml` é escrita à mão — não há gerador que a atualize
sozinha. `api/internal/api/v1/openapi_test.go` compara `app.GetRoutes(true)` com a spec **nos dois
sentidos**: rota sem documentação e operação documentada que não existe mais quebram o build.

Consequência prática: adicionar, renomear ou remover rota exige editar o arquivo YAML correspondente
no mesmo commit. Não existe "documento depois" — o teste não deixa.

Ao mexer na spec, rode `make openapi-lint` (requer Node): o teste de Go só garante cobertura de
rotas e YAML válido, não que o documento seja um OpenAPI válido.

## Feature de UI nova entra no guia na mesma mudança

O guia público (`ui/src/app/guide/`) é a documentação de produto, e ele mostra captura de tela real
— não desenho nem descrição. Como as capturas vêm do mock (`ui/src/lib/mock/`), tela que o mock não
modela sai vazia na imagem, sem erro nenhum.

`ui/src/__tests__/lib/guide-assets.test.ts` fecha os três buracos: imagem referenciada que não
existe, captura gerada que nenhuma página usa e tópico listado sem rota. Como no caso da spec
OpenAPI, não existe "documento depois" — o teste não deixa.

Fluxo: entrada em `CAPTURES` (`ui/scripts/capture-screens.mjs`) → `npm run screens:capture` com
`NEXT_PUBLIC_MOCK_API=true npm run dev` rodando → seção no tópico do guia. Detalhes em `DOCS.md §5`.


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

### SEFAZ TLS compatibility exception

- Both fiscal engines currently keep TLS server-certificate verification disabled
  because deployed SEFAZ chains are not accepted by the default client trust store;
  the same endpoints can be reported as insecure by a browser.
- This is an explicit interoperability requirement for the current deployment, not
  an accidental insecure default or an unreviewed cleanup target.
- Endpoint selection remains restricted to the source-controlled SEFAZ catalog;
  certificate verification must not be toggled per request or from user input.
- Do not enable default verification without first homologating every supported UF
  and deploying a verified trust-bundle/pinning strategy that preserves connectivity.

## ctech-dfe-api (Go + Fiber backend)

- Uses AWS SDK v2 for Go — do not add boto3 or any Python client.
- Auth is RS256-only. JWT `sub` claim is the ctech user ID. There is no `SECRET_KEY` — do not add HS256.
- JWKS keys are cached in Redis/Valkey (TTL 1h). Falls back to in-memory when `VALKEY_URL` is unset.
- NF-e numbering uses `transact_write` for atomicity — never replace with separate read/write.
- Organization context is passed via `Dfe-Organization-Pk` header — never path parameters.
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
- **EC2 user data is capped at 16384 bytes, and CloudFormation only says so at deploy time.**
  `ApiStack` therefore keeps in user data only what CloudFormation has to resolve — bucket names,
  SSM parameter names, log group names, the VPC CIDR — and ships every static file (`nginx.conf`,
  `app.service`, `start.sh`, `deploy.sh`, `upload-logs.sh`, `logrotate.conf`) as an `aws-s3-assets`
  Asset from `cdk/scripts/api/`. Adding a new file to user data is the wrong reflex: put it in that
  directory. `test/api-stack.test.ts` fails if the rendered user data crosses the limit again.
  Use an Asset rather than a bucket with fixed keys: the Asset's key is a content hash, so editing a
  script versions the launch template and triggers an instance refresh, while a fixed key would
  change the file under instances already running.
- **A launch template change replaces the API instance during `cdk deploy`, before the API
  binary is deployed.** `@aoctech/cdk` >= 0.10.1 puts an `AutoScalingRollingUpdate` on the
  ASG. Before that, running instances kept their old `/etc/app-static.env`, and the next deploy
  started a new binary against it (#114: missing `DFE_EGRESS_REGION`, crash loop). The pipeline
  runs CDK before the API job, so the replacement boots the previous `current.zip` with the new env
  until the API job finishes. A new `required` env var must therefore not break the previous binary.
- Profile and password management endpoints do not exist — those belong to ctech-account.
- **Membership is owned by the `organization_users` table** (via `MembershipService`). RBAC,
  `/auth/me`, `GET /organizations`, and the WebSocket all resolve access through
  `MembershipService.Get`. The legacy embedded `users.organizations` list is dead — no longer read
  or written (dual-write/fallback removed post-migration); do not reintroduce authorization from it.
- **An organization has exactly one OWNER, and it is whoever created it.** Ownership is not a role
  that gets handed out: it is written once by `OrganizationService`, in the same `TransactWrite` as
  the organization row, and by nothing else. `repositories.GrantableRoles` (ADMIN/USER/VIEWER) is
  the single list of what member management may assign, checked by `MembershipService.Create`,
  `ChangeRole` and `InvitationService.Create` — three doors into one room. Do not re-list those
  roles anywhere else, and specifically not in a `validate:"oneof=…"` tag: a request DTO that
  carries its own copy makes the invariant hold only for callers that happen to use that DTO.
  Anyone needing full access gets ADMIN, which has the identical permission set. Ownership transfer
  is a real feature that deliberately does not exist yet; when it does, it **moves** the single
  OWNER and does not add a second.
  Why it has to stay singular: ownership decides which account's subscription pays for the
  organization (`owner_user_id`, billing integration), and "the OWNER's plan" is only an answer
  while there is one OWNER.
- Removing/demoting a member must not leave an org without an OWNER, and mutating a membership must
  invalidate its cache (a tombstone on removal) — do this through `MembershipService`, not the repo.
  `guardLastOwner` still *counts* owners rather than refusing outright, so an organization carrying
  two OWNERs from before this rule can be repaired by demoting one.
- Creating an organization is KYC-gated and atomic: org + certificate + OWNER membership + audit in
  one `TransactWrite`. A certificate is required unless a matriz certificate (same CNPJ root) is
  inherited. Invitations grant only ADMIN/USER/VIEWER and are single-use — never weaken these.
- **A price the catalogue hides must also be a price `Choose`/`Change` refuse.** `Plans` filters
  through `sellable()` (no `visibility: internal`, no archived price, no inactive product) and
  `validatePrices` validates against that same filtered list. Never add a filter on one side only:
  filtering just the response makes the price list an access control, and
  `price_dfe_unlimited_internal_monthly` — R$ 0, every quota `-1` — is written down in
  `docs/plans/`. Granting it is an M2M operation against ctech-billing, never a request a browser
  can make.
- **The subscription gate has no runtime switch — configuring billing *is* turning it on.**
  `RequireActiveSubscription` is mounted on the whole `/v1.0` group and disables itself only via
  `billing == nil || !billing.Enabled()`, and `Enabled()` is just "the client was constructed"
  (`billingclient/client.go`). So a deploy that carries billing configuration blocks issuance and
  registry writes for every account that has no live subscription, from the first request. There is
  no "deploy now, enforce later" — plan the migration of existing accounts around that, and rely on
  the `/v1.0/billing/*` exemption (the way out of the block) rather than on an ordering the code
  does not offer.
- **The `ResultsConsumer` must not delete a message it could not settle with billing.** Two things
  ride on a terminal result: the WebSocket notification, and the money (usage report on
  `authorized`, quota refund on `rejected`/`failed`). Deleting on failure would drop a usage report
  with nothing but a log line behind it; leaving the message costs three redeliveries and then the
  results DLQ, which an operator redrives (there is no alarm on it since 2026-08-19 — depth is a
  console check). Both sides are idempotent, so a redrive is safe — the usage report by its access
  key; the refund decrements the counter and writes its `refund:{meter}:{key}` marker in the same
  transaction. Anything added to the settlement path has to keep that property.
- **Production quota and billing context are part of the issuance transaction.** Homologation never
  reserves or reports usage and is identified explicitly by `billing_exempt`; an absent field is legacy,
  not proof of exemption. Production commands carry the account, subscription, price, meter and
  period captured at admission; settlement must never reassign an in-flight document to the plan
  currently active. A terminal SNS publish failure is returned to SQS, and terminal redelivery only
  republishes the result — it must not call SEFAZ again or rewrite the terminal status.
- **Live company/user quotas use a transactional version guard.** Counting remains live state, but the
  `QUOTA_GUARD_{user}#{meter}` update must travel in the same transaction as organization creation or
  invitation acceptance. A read-count-write sequence without that guard can exceed the plan under
  concurrent requests.
- `CRUDRepository[T]`'s `Create`/`BuildCreateTxItem`/`BuildCreateTxItemIfAbsent` marshal `entity T`
  via `marshalEntity` (`internal/repositories/base.go`), never `MarshalMapOmitNull` directly — when
  `T = map[string]types.AttributeValue` (`ProductRepository`, `ServiceRepository`), the values are
  already `AttributeValue`s and re-marshaling them via `attributevalue.MarshalMap` wraps each one in
  a nested Map (no special case for values already implementing `types.AttributeValue`) instead of
  passing it through, and DynamoDB rejects the write. `marshalEntity` passes such maps through
  unchanged (shallow copy); anything else still goes through `MarshalMapOmitNull` as before. Found
  because it silently broke `ProductRepository.Create` against real DynamoDB, not just in tests.
- The five fiscal config services (NF-e/NFC-e/CT-e/MDF-e/NFS-e) share one `fiscalConfigService`
  base (`internal/services/fiscal_configs.go`) — `Get`/`Upsert` live there once. A new config
  variant is added by declaring a thin `struct{ fiscalConfigService }` wrapper + constructor
  (repo, audit resource constant, resource ID, 404 message); never re-implement `Get`/`Upsert`
  per variant. The audit diff on `Upsert` always compares the pre-existing item against the
  **final merged fields** (post preserve-field carry-forward from `FiscalConfigRepository.BuildUpsertTxItem`),
  never against the caller's raw input — otherwise a preserved internal-process field (e.g. an
  NSU/number counter silently carried forward) would be misreported as a user-initiated change.
- Cross-doc-type helpers live once in `internal/services/shared.go` and are called from the
  per-doc-type packages, never re-declared: `EnvToPrefix` (environment code → `prod`/`hom` prefix,
  used by every `{prefix}_current_number`/`{prefix}_nsu` field and by the document PK) and
  `DownloadS3` (bucket read for stored XML/PDF). `nfes`/`mdfes` keep one-line private wrappers only
  to avoid churn at the call sites; a third copy of either function is a bug.

## ui (Frontend)

- Auth is OAuth 2.0 PKCE via ctech-account. `login()` redirects to accounts.aoctech.app; `/callback` exchanges the code for tokens.
- `access_token` is in module-level memory only — **never write it to localStorage or sessionStorage**.
- `refresh_token` is held by ctech-account in the HttpOnly `ctech_rt` cookie and is never exposed to JavaScript;
  `@aoctech/auth-client` sends it with `credentials: 'include'` and coordinates refresh across tabs.
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
- **Domínio fechado nunca é campo de texto.** Toda tabela SEFAZ, enum, UF, município, país, unidade,
  CFOP, NCM, CEST, CST, `cClassTrib` e data é `OptionsSelect` (até ~12 opções) ou `Combobox` (acima
  disso, buscável). Literal do leiaute (`SEM GTIN`, `ISENTO`) é escrito por checkbox, nunca digitado.
  Antes de criar uma tabela de opções nova, procure em `lib/data/` — várias já existem e derivam umas
  das outras (`UF_IBGE_OPTIONS` das cidades, `LC116_SERVICE_OPTIONS` da tabela nacional da NFS-e,
  `cfopSuffixOptions()` da tabela de CFOP). Tabela de opções estática vai para escopo de módulo: um
  array recriado por render invalida o memo do `Combobox`.
- **Regra do leiaute que você sabe explicar em português vira `superRefine`, não texto de ajuda.**
  Uma dependência condicional documentada só ao lado do campo é rejeição adiada — o cadastro salva e
  detona semanas depois, na emissão, em outra tela. As regras de grupo tributário são compartilhadas
  por `applyTaxGroupRules` (produto e perfil fiscal), e as de emissão por
  `lib/utils/emit-guards.ts` (NF-e e NFC-e).
- **Soma que não fecha bloqueia, não avisa.** Pagamentos e duplicatas conferem contra o total com
  tolerância de R$ 0,01 antes de o passo avançar; excedente é `vTroco` só na NFC-e. Aritmética é
  responsabilidade do sistema, não do operador.
- **Container que pode esconder erro carrega badge de contagem.** Aba, `CollapsibleSection` e bloco
  colapsado: submit que falha sem nada mudar na tela é o pior caso do formulário grande.
- **Toda tela tem visão simples por padrão e avançada sob demanda.** O simples resolve o caso comum;
  o avançado nasce aberto quando já tem dado, com contador, para fechado nunca virar escondido.
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
- **Fiscal config gating (NF-e/NFC-e/CT-e/MDF-e/NFS-e).** Every doc type's config (`GET
  .../{doc}-config`) can 404 when the org hasn't set it up yet. `useFiscalConfig(variant, pk)`
  (`@/lib/hooks/useFiscalConfig.ts`) is the single place that maps a 404 to `config: null` ("not
  configured") instead of a query error — always use it instead of calling `apiClient.get*Config`
  directly, so every page agrees on what "not configured" means. Emit pages wrap their form in
  `<RequireFiscalConfig variant="...">` (`@/components/dfe/RequireFiscalConfig.tsx`), which
  redirects to `/fiscal-config?tab={variant}` the first time config is missing (emission cannot
  succeed without it). List/browse pages instead render `<ConfigRequiredBanner>`
  (`@/components/ui/config-required-banner.tsx`) alongside `HomologationBanner`, since viewing
  existing documents doesn't require config.
- **Disponibilidade da API é respondida uma vez, em `lib/network/liveness.ts`.** Nenhuma tela,
  hook ou consulta pode virar sua própria sonda de disponibilidade. `ApiClient` espera o primeiro
  resultado da sonda (`requireApiLiveness`) e falha rápido enquanto a API está fora; a sonda
  (`GET /v1.0/health-check`, pública) é a única requisição permitida nesse estado e é quem descobre
  a volta. `NEXT_PUBLIC_MOCK_API=true` desliga o mecanismo inteiro.
- **Retentativa mora no `ApiClient`, nunca no TanStack Query** (`retry: false` em
  `QueryProvider`). Retentar nas duas camadas multiplica a mesma queda por três. Só métodos
  seguros (`GET`/`HEAD`/`OPTIONS`) retentam — um documento fiscal não vale uma duplicata
  acidental — no máximo 2 vezes, com jitter total e teto de 3 s, e `Retry-After` vence.
- **Timeout de toda chamada: 5 s (`HTTP_TIMEOUT_MS`).** A API responde rápido ou não responde.
- **Estado derivado só é exibido quando a derivação termina.** Um valor montado a partir de N
  consultas lê como "vazio" enquanto qualquer uma estiver em voo, e uma leitura que falhou não é
  prova de ausência. Hooks derivados expõem `isPending` cobrindo **todas** as consultas de que
  dependem (`isLoading`, não `isPending`, para consultas condicionais — uma consulta desabilitada
  fica pendente para sempre) e `isUnknown` quando alguma falha; a UI não renderiza em nenhum dos
  dois casos. Ver `useOnboarding` / `SetupChecklist`.

## cdk (Infrastructure)

- Table names are environment-prefixed (`{prefix}_`).
- Development stacks use `RemovalPolicy.DESTROY`.
- Production uses `RemovalPolicy.RETAIN`.
- IAM permissions must follow least privilege principle.
- PITR (Point-in-Time Recovery) is enabled only in prod (`cdk/lib/dynamodb-stack.ts`); stage is RETAIN without PITR.

### Edge routing (Cloudflare in front of everything)

Every app domain (`dfe`, `wallet`, `accounts`, `poker`, `billing`) serves its UI as static assets on
**Cloudflare Workers**, and serves nothing else. The API is *not* behind the app domain any more: the
browser calls the `*-api` hostname directly, which reaches the shared HAProxy edge, and **CORS
applies**. DNS was already on Cloudflare, so the CloudFront hop in between was cost and latency for a
request whose two ends were already talking. See
`ctech-cdk/docs/plans/2026-08-20-frontend-cloudflare-migration.md`.

- **The app host serves only files.** There is no API behaviour, no `errorResponses` decision to get
  wrong, and no edge redirect. Workers Static Assets resolves `/documents` to `documents.html`
  itself, so the `url-rewrite` function, its KeyValueStore and `ui/scripts/publish-routes.sh` are all
  gone — a route manifest that could drift from what shipped no longer exists to drift.
- **Security headers ship as a generated `_headers` file**, written by `ctech-cdk`'s reusable
  workflow `frontend-cloudflare.yml`, not by a per-service response-headers policy.
- **The CSP's `connect-src` is derived from the build environment**, so it is the workflow — not the
  code — that decides which origins the browser may reach. It is scheme-exact: `https://host` does
  not permit `wss://host`, which is why every service with a WebSocket sets `NEXT_PUBLIC_WS_URL`
  explicitly instead of deriving it from `NEXT_PUBLIC_API_URL` at run time.
- **Service-to-service calls use the `*-api` host directly** (e.g. `CTECH_JWKS_URL`), unchanged — and
  now so do browsers. The `*-api` hosts stay public for both.
- **Cross-origin does not break the auth cookies.** `ctech_rt` is `SameSite=Lax` and every app host
  shares the registrable domain `aoctech.app` with its API host, so the request is cross-origin but
  same-site. It still needs credentials on both ends: `credentials: 'include'` in the client and
  `AllowCredentials` plus an exact origin list in the API.

### Client IP behind the proxies

nginx sits behind HAProxy, which receives requests that crossed Cloudflare. Getting the client's IP
wrong silently breaks rate limiting — the zone still exists, it just keys on the wrong thing. The
CloudFront hop is being removed, but the ranges below stay trusted until the teardown lands, and
trusting a hop nothing routes through is inert rather than wrong.

- **Any rate-limit zone keyed on `$binary_remote_addr` requires the realip module.** Without
  `set_real_ip_from`, `$remote_addr` is the HAProxy instance's private IP, so every client shares one bucket and
  the limit protects nobody. `/opt/app/update-realip.sh` (in the ASG userdata) writes
  `/etc/nginx/conf.d/realip.conf` with the VPC CIDR plus CloudFront's origin-facing ranges,
  read from the AWS-managed prefix list `com.amazonaws.global.cloudfront.origin-facing` via the
  EC2 dual-stack endpoint (the instances are IPv6-only and `ip-ranges.amazonaws.com` has no AAAA
  record) and refreshed by a daily systemd timer.
- **Never trust a client-supplied leftmost `X-Forwarded-For` entry.** HAProxy authenticates Cloudflare,
  resolves the viewer address through known CloudFront/Cloudflare hops, and overwrites forwarding headers.
  `real_ip_recursive on` then walks the remaining trusted chain right-to-left.
- nginx **overwrites** `X-Forwarded-For` with the resolved IP (`proxy_set_header X-Forwarded-For
  $remote_addr`) rather than appending, so the Go app's `TRUSTED_PROXIES` / Fiber `c.IP()` — which
  reads the leftmost entry — cannot be fed a forged value.

### Exclusão de conta (LGPD)

- Toda tabela DynamoDB nova entra em `api/internal/services/erasure_catalog.go` no mesmo change; o teste
  `TestErasureCatalogCoversEveryCDKTable` falha caso contrário. Chave nova em `account_billing` entra em
  `billingKeyKinds` (`TestEveryAccountBillingKeyKindIsPurged`).
- Todo consumidor assíncrono novo de trabalho de empresa consulta o estado da empresa (`ORG#{company_id}` em
  `erasure_state`) e descarta o job se estiver bloqueada ou apagada; falha na consulta é erro (fail closed).
- O CPF do endpoint de subject fica só em memória: nunca logar, persistir nem colocar em mensagem de erro.
- Objetos S3 são apagados pelas chaves que as linhas guardam (todas as versões), nunca pela pasta `CNPJ_`.
- Credencial do ack e do subject é `ERASURE_CLIENT_*`, nunca `ACCOUNT_CLIENT_*`.

## ctech-dfe-worker (Go Lambda)

- Runtime: `provided.al2023`. Binary must be named `bootstrap`.
- Command queues are standard SQS and provide at-least-once delivery; never rely on ordering or queue deduplication.
- Issuance/event handlers must conditionally claim the document with a processing owner and lease before SEFAZ.
  Only the owner may finalize, and a DynamoDB claim/read error must fail closed.
- Infrastructure, storage, engine, HTTP 408/425/429/5xx, and malformed-response failures remain retryable and
  release the lease. SEFAZ business rejection is terminal.
- API issuance must transact document/counter state with an immutable `worker_outbox` command. The DynamoDB Stream
  `outbox-publisher` publishes it to command SNS and conditionally acknowledges the row.
- **Um `sefaz_service` novo não existe até ter `WorkerDefinition`.** SNS→SQS roteia por filter policy sobre
  `sefaz_service`; mensagem que nenhuma policy casa é **descartada em silêncio** — sem DLQ, sem log, o documento fica
  `pending` para sempre (foi assim que a emissão de NFS-e ficou inerte até 2026-08-07). Ao adicionar um serviço na
  API, no mesmo change: (1) `cdk/lib/worker-definitions.ts` com o serviço e as tabelas do role, (2) o nome da função
  e do DLQ processor nas quatro listas de `.github/workflows/worker.yml` (o deploy atualiza por nome, não pela lista
  do CDK), (3) o serviço no teste de cobertura em `cdk/test/worker-stack.test.ts`.
- The worker invokes the go-dfe-egress Lambda (sa-east-1) for every SEFAZ/municipal call; there is no in-process path and no py-dfe fallback.
- After SEFAZ response: update DynamoDB, upload XML to S3, publish terminal results to SNS.
- DLQ receives messages after max retries — monitor and alert.

---

# go-dfe-egress (sa-east-1)

- **Every SEFAZ/municipal call goes through the `go-dfe-egress` Lambda.** `worker`/`api` never call `dfe.Call` in
  process and there is no fallback client; a failed `Invoke` or a `FunctionError` fails the call (the worker retries).
- **The PFX and its password cross regions in the Invoke payload** (us-east-1 → sa-east-1, same account, IAM-only
  `lambda:InvokeFunction`). The egress must **never log the request body**; it logs only `doc_type`, `service`, `uf`,
  status and duration (`go-dfe-egress/handler.go`, covered by a test).
- **Timeout chain:** workers that call SEFAZ (150 s) > egress Lambda (120 s) > go-dfe worst case (NFS-e: 4 attempts × 20 s
  + backoff). Change one, change the others (`cdk/lib/egress.ts`, `cdk/lib/worker-definitions.ts`, `go-dfe/internal/constants`).
- **Environment strings:** `worker`/`api` send `producao`/`homologacao`; the egress converts to `prod`/`hom`. NFS-e sends an
  empty `uf` (municipal competence) and must not be rejected for it.
- **Cross-region references are strings** (`egressFunctionName`/`egressFunctionArn`): never a CloudFormation reference.
  `cdk bootstrap` of sa-east-1 is required before the first deploy.

---

# Fontes de schema e MOC

- XSDs oficiais: portais SVRS (`https://dfe-portal.svrs.rs.gov.br/{NFE,MDFE,CTE}/Documentos`); download em
  `/{SIS}/DownloadArquivoEstatico/?sistema={SIS}&tipoArquivo=2&nomeArquivo={zip}`. Os XSDs em `cdk/cdk.out` podem
  estar desatualizados (CT-e estava na NT 2026.001 com a NT 2026.005 SN publicada): confira a data antes de usar.
- MOC online: HTTPS recusa conexão; use `curl` em HTTP. Plano: `docs/plans/2026-11-04-moc-cobertura-total.md`.
- `SegCodBarra` do MDF-e tem 36 dígitos e só vale em FS-DA; nunca emitir a chave de 44.

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

## Tag vazia nunca vai ao XML

Os builders de NF-e/NFC-e e MDF-e passam o documento por `services.PruneEmpty`: tag ou atributo de texto vazio é
omitido (dado ausente não vira `<xFant></xFant>`, que a SEFAZ recusa com 225). Grupo que o builder já cria vazio é
preservado. O builder do CT-e (`api/internal/services/ctes`) faz o mesmo.

## Ordem dos filhos: teste de paridade com o XSD

`go-dfe/validate/xsdorder_parity_test.go` compara a tabela `xsdorder` com a sequência dos XSDs vendorizados. Tag fora da
tabela vai para o fim em ordem alfabética (225 na SEFAZ), então todo campo novo de leiaute entra em `xsdorder` junto.
NF-e, MDF-e e CT-e falham o teste se divergirem (o CT-e ficou estrito junto com o builder do CT-e rodoviário).
`table_test.go` fixa a contagem de chaves: chave nova entra nas duas.
O teste já pegou ICMSTot sem vICMSUFDest/vFCPUFDest/vICMSUFRemet, impostoDevol e agropecuario/infPAA fora de ordem.

## CT-e rodoviário: nada fiscal é presumido

`POST /v1.0/ctes` (spec `docs/specs/2026-11-05-cte-emissao-rodoviaria.md`) devolve 422 com `code` em vez de
presumir regra fiscal: `cfop_required` (o CFOP da prestação é do usuário; nenhum 5353/6353 padrão),
`icms_required` / `icms_rate_required` (regime normal sem CST ou alíquota; só CRT 1, 2 e 4 caem no ICMSSN),
`rntrc_required` (RNTRC vem de `organization_mdfe_configs.rntrc`, nunca inventado). O cliente não é fonte de verdade
das notas: a emissão refaz o `derive` no servidor. Origem e destino sobrescritos precisam ser um município que
aparece nas notas ou nos participantes (não há tabela IBGE de nomes na API).

As regras do MOC do CT-e (`go-dfe/validate/rules_cte.go`) recebem o conteúdo de `<CTe>` como as de NF-e e MDF-e
(`dfecheck.ShadowCTe`), e aceitam o `infCte` direto, que é o que o catálogo testa.
`validate` trata choice obrigatória com braços só de elementos opcionais como satisfeita vazia
(`infNFe/{infUnidCarga|infUnidTransp}` do CT-e).

A URL do QR Code (`infCTeSupl/qrCodCTe`) sai de uma tabela por autorizador em `ctes/builder.go` (MG, MS, MT, PR e
SP próprios; o resto SVRS), a mesma para produção e homologação: conferir na homologação de cada autorizador.

- **Portão do CT-e:** `ctes/prevalidate.go` (`dfecheck.GateCTe`) recusa em 422 `fiscal_prevalidation` as regras
  determinísticas do catálogo (DV da chave G015, UF/modelo/Id da chave G007, G013, G014, grupo do tipo G016,
  CPF/CNPJ com DV incluindo alfanumérico G104, PAA01, G210, autXML G193/G194, atores G114 a G149) e tamanhos do
  leiaute (nunca truncados). O resto do catálogo continua só no shadow (tabelas, cadastros, relógio da SEFAZ).

- **Eventos do CT-e** (`ctes/events.go`): cancelamento 110111 (só nSeq 1, justificativa 15 a 255 por TJust),
  CC-e 110110, desacordo 610110 e seu cancelamento 610111; sem EPEC. Cancelamento e CC-e são do emitente (632);
  610110 e 610111 do tomador (755), e o XML do evento leva o documento do autor (a organização). Pontos que o
  MOC não fecha e estão por confirmar na homologação: literal `xCondUso` com "Art. 58-B" (o MOC mostra "58- B",
  tratado como quebra de linha), valor de `indDesacordoOper` ("1") e nome da raiz do 610111 (`evCancPrestDesacordo`;
  o MOC repete `evPrestDesacordo`), `cOrgao` = UF da chave (regra N02, rejeição 677). A tabela dos campos que a
  CC-e não pode alterar (681) não está catalogada: fica com a SEFAZ. Os XSDs dos eventos estão vendorizados (ver
  "XSDs de evento" abaixo); o `Id` do evento do CT-e usa nSeqEvento com 3 dígitos (padrão do XSD) e `xObs` do 610110 é obrigatório.

- **Distribuição do CT-e e do MDF-e:** o `distDFeInt` de `CTeDistribuicaoDFe` e de `MDFeDistribuicaoDFe` só tem
  `distNSU` e `consNSU` (MOC); `consChNFe` existe só na NF-e. O worker descarta o job `cons_ch_nfe` de `cte` e
  `mdfe` sem chamar a SEFAZ (antes mandava `consChCTe`/`consChMDFe`, que o schema não conhece); buscar um CT-e pela
  chave é `GET /v1.0/ctes/{access_key}/sefaz-status`. `validation.AccessKeyReason` aceita o modelo 64 (GTV-e,
  tpEmis 1 e 2) e, para CT-e e CT-e OS, os tpEmis 1, 2, 3, 4, 5, 7 e 8 do Leiaute (antes só 1, 4, 5, 7 e 8).
  O espelho `ui/src/lib/utils/access-key.ts` precisa seguir a mesma lista.

- **IBS/CBS do CT-e** (`ctes/ibscbs.go`): o MOC define `vBC` só como "base comum a IBS/CBS" e não a liga a
  `vTPrest`, por isso o usuário informa a base, as alíquotas, o CST e a classificação (nunca presumidos). Fora do
  escopo por falta de decisão fiscal fechada: Simples Nacional (CRT 1 e 2 exigem `gTribSN`, SN-1101 e SN-1103;
  CRT 4 não admite o grupo, SN-1107), `tpRegimeIBSCBS` do emitente, diferimento, devolução, redução, ALC/ZFM,
  tributação regular, compra governamental (CGOV) e estorno de crédito. `vTotDFe` segue a regra IBSCBS-365
  (soma IBS e CBS só a partir de 2027; o texto do Leiaute diz sempre vTPrest + IBS + CBS, divergência a
  confirmar). A ordem do XML já estava na tabela `xsdorder` (a paridade estrita com o XSD cobre `imp/IBSCBS`).

## CT-e complementar (tpCTe 1) e substituto (tpCTe 3)

`POST /v1.0/ctes/complement` e `POST /v1.0/ctes/substitute` (`ctes/complement.go`, `substitute.go`, `original.go`).
**Os participantes (rem, exped, receb, dest, tomador), o percurso, o tipo de serviço e, no substituto, a carga e as NF-e
vêm do XML autorizado do CT-e original (`xml_s3_key`), lido por `nfexml.ParseCTe`; nunca do cadastro de pessoas.**
O original tem de ser da própria organização, do mesmo ambiente, `authorized`, tipo normal ou substituto e rodoviário.
No MOC publicado as regras são: complemento G017 e G160 a G177, substituição G076 a G096 e troca de tomador G097 a
G101 (a numeração G078 a G096 que circulou para o complemento é a da substituição).

- O XSD 4.00 do complemento só pede `infCteComp/chCTe` (1 a 10); não existe `vPresComp`. O valor complementar vai em
  `vPrest`/`Comp` e o imposto em `imp`; os atores e o tomador repetem o original (G168 a G175). `infCteSub` fica dentro
  de `infCTeNorm`. tpCTe 2 (anulação) não existe no 4.00 e não foi construído.
- **422 local (provado só com o original):** G160 `original_not_issued_by_organization`; G162/G165 (só o que o registro
  prova) `original_not_found`, `original_not_authorized`; XML ausente ou ilegível `original_xml_unavailable`,
  `original_xml_invalid`; G164/G085 `original_type_invalid`; escopo `original_unsupported`; G161a `original_too_old`
  (chave com mais de 12 meses); G168 a G176 entre os vários originais `originals_mismatch`; G173/G091 `emitter_ie_changed`;
  G012 e G076 `contingency_not_allowed` (SVC não aceita complemento nem substituto; o substituto só em emissão normal,
  o complemento aceita o off-line); G177, G161, G017, G076, G077 e G078 entram no portão (`blockRules`); G096
  `substitution_window_expired` só além de 60 dias + 168 h da emissão (o registro não guarda `dhAut`); troca de tomador:
  G099 `taker_base_not_in_original`, G100 `taker_uf_mismatch`, G101 `taker_unchanged`, `taker_role_missing`,
  `taker_required`, `taker_change_not_requested`. Sem troca, o tomador é o do original (G097, G098 por construção).
- **Fica com a SEFAZ (precisa de estado dela):** existência e chave divergente do original (G079, G080, G162, G163),
  cancelado ou já substituído/complementado (G081 a G083, G166), limite de 10 complementos (G167), o prazo exato de 60
  dias contado da autorização (G096) e o evento 610110 do tomador no original (G084, 739): o evento é registrado pelo
  tomador, que pode ser outra empresa, então o que há em `cte_events` não prova ausência.
- `persist` (`emit.go`) é o fim comum da emissão normal, do complemento e do substituto (numeração, chave, portão,
  outbox/offline). O QR Code, o responsável técnico e o `shadow` são os mesmos.

## CT-e OS (modelo 67)

`POST /v1.0/ctes/os` (`ctes/os.go`): tpCTe 0, modal rodoviário, três serviços (6 pessoas, 7 valores, 8 excesso de
bagagem). Leiaute: `cteOS_v4.00.xsd`; regras: MOC H001 a H133 (as de GTV-e são I01 a I75). Caminho próprio no DTO e no
builder (`BuildCTeOS`, raiz `<CTeOS versao="4.00">`), mas o fim da emissão é o `persist` do CT-e 57.

- **Assinatura:** o `Id` está em `infCte`, não em `infCTeOS`: `go-dfe/internal/services/config.go`
  (`SignIDXPath["CTeRecepcaoOS"]`) era `infCTeOS` (tag que não existe no XSD) e passou a `infCte`, com teste. O mesmo
  defeito está em `CTeRecepcaoGTVe` (`infGTVe`; o XSD tem `infCte`) e fica para quando a GTV-e for construída.
  Signer e C14N não foram tocados. Serviço no egress: `CTeRecepcaoOS` (`CTeRecepcaoOSV4`), já na allowlist `implemented`.
- **xsdorder:** entradas com escopo `CTeOS:` e `rodoOS:` em `table.go`; a paridade com o XSD (`xsdorder_parity_test.go`)
  agora cobre `cteos` e `cte_rodoos` e é estrita.
- **Numeração:** o CT-e OS usa o mesmo contador e a mesma série da Configuração Fiscal do CT-e 57 (chave com modelo 67).
  Cada modelo vê lacunas de número; o CT-e não tem inutilização. Contador próprio exigiria atributo novo em
  `organization_cte_configs` e está em aberto.
- **Nada presumido:** CFOP e ICMS do usuário (sem CST 60, que o CT-e OS não tem); tomador sempre informado (o
  `indIEToma` do `ide` é obrigatório e o MOC só dispensa o `toma` no excesso de bagagem); município com nome e UF vindos
  do picker (sem tabela IBGE na API).
- **422 local (provado só com o corpo):** H015, H018, H019, H020, H020a, H023 a H028, H029, H029a, H045, H045a, H046a,
  H051 a H053, H057, o CNPJ do emitente (XSD) e UF x município (H117, H119). O portão do CT-e OS recusa só tamanhos do
  esquema: não há catálogo de regras H em `go-dfe/validate` (shadow só do esquema).
- **Fica com a SEFAZ:** existência dos documentos referenciados (BP-e, GTV-e), IE e cadastros, percurso (H021), registro
  estadual (H044), QR Code (H128 a H133), duplicidade, CT-e OS complementar e substituto (H059 a H075, H101 a H114).
- **Fora do núcleo (não construído):** tpCTe 1 e 3 do CT-e OS, `seg`, `cobr`, `refCTeCanc` (H037 a H043), `pgtoVinc`,
  `gCompraGov`, antecipação de pagamento, outros modais. GTV-e (modelo 64) e CT-e Simplificado: só spec
  (`docs/specs/2026-11-04-moc-gap-cte.md`, seção 7).

- **CT-e OS: eventos, DACTE e referência reversa.** Só 110111 e 110110 valem para o modelo 67 (catálogo do MOC na spec
  `2026-11-04-moc-gap-cte.md`, 6.3); os eventos do 57 (610110/610111, 110180 a 110191) recebem 422 `event_not_for_cte_os` em
  `requireModel57`. DACTE OS: `documents/dacte_os_retrato.html` pelo Anexo II do MOC. Complementar e substituto gravam, na transação
  da emissão, linhas `ref#{original}#{derivado}` na tabela `ctes` (lidas por Query com `begins_with`, nunca Scan); a linha não pode
  ganhar `number`, `incoming` ou `year` (entraria nos GSIs de listagem).

## MDF-e: portão do MOC, chaves e CNPJ alfanumérico

- **Portão:** `mdfes/prevalidate.go` lista as regras do catálogo do MDF-e que recusam a emissão (422
  `fiscal_prevalidation`), ligadas em 2026-11-06. Só regras "Obrig." determinísticas, provadas só com o documento:
  DV de chave, CPF/CNPJ, contagens, duplicidades, totais do pagamento (tolerância R$ 0,01), tipo de emitente e
  carregamento posterior. Ficam no shadow as que comparam com o relógio da SEFAZ (F30a, F37a, F45a, F36b, F79, F80),
  as que precisam de tabela que o leiaute não traz (F08, F11, F89, F90), as de cadastro externo (F113 por categoria do
  RNTRC) e F72 (IE). Regra nova em block entra com teste que a dispara (`later_loading_test.go`).
- **CNPJ alfanumérico (IN RFB 2229/2024):** o valor de cada caractere no DV do CNPJ é o código ASCII menos 48 (A=17),
  o mesmo da chave de acesso; o exemplo oficial `12.ABC.345/01DE-35` confere. `validation.ValidCNPJ` usava A=10 e
  recusava todo CNPJ alfanumérico real (corrigido; `ui/src/lib/utils/validators.ts` ainda usa A=10 e precisa da mesma
  correção). Em CPF/CNPJ de MDF-e, normalizar com `docAlnum`, nunca `onlyDigits` (que apaga a letra).
- **Chave de acesso:** `validation.AccessKeyReason(chave, modelos...)` é a validação única na API (tag
  `dfe_access_key=55 57`; sem parâmetro, só NF-e). Confere UF, mês, CNPJ ou CPF com DV, modelo, `tpEmis` do modelo e DV;
  CNPJ começando com `000` é tentado como CNPJ antes do CPF prefixado. As regras F30, F37 e F45 de `go-dfe/validate`
  conferem também o ano (o DTO não).
- **Carregamento posterior** é do pedido (`later_loading`); a configuração `ind_carrega_posterior` só vale para pedido
  sem documentos, porque com documentos a SEFAZ rejeita sempre (F27, 706).

## Lambdas Go: o asset do CDK fingerprinta go-dfe

`cdk/lib/go-code.ts` calcula o hash do asset com o módulo, `go-dfe/` e `go.work(.sum)` (`goSourceHash`). Antes o CDK
só fingerprintava o diretório do módulo; uma mudança só em `go-dfe` (api, worker e egress o importam por `replace`)
não mudava o asset e a Lambda ficava com o código antigo. Foi assim que o conserto do `tpOper` da NFS-e (E0904) não
chegou ao egress. Teste: `cdk/test/go-code.test.ts`. Qualquer dependência por `replace` nova entra em `SHARED_SOURCES`.

## NF-e e NFC-e: cadastro nunca é truncado nem inventado

- **Tamanho:** os limites de texto do cadastro (pessoa, organização, endereço, IE, e-mail, produto) e do pedido
  (`NfeLocalBody`) são os do leiaute da NF-e. A fonte única é o schema gerado dos XSDs (`go-dfe/validate`,
  `Spec.Lengths`); `api/internal/api/v1/dto_layout_limits_test.go` falha se uma tag `validate` passar dele. Registro
  antigo mais longo **não é migrado nem truncado**: a emissão devolve 422 `fiscal_prevalidation` com `too_long`, a
  entidade e o campo, e o usuário corrige o cadastro. A única truncagem que existe é a do `natOp` (`truncateNatOp`).
- **Placeholders:** nenhum código fiscal inventado. Sem município IBGE, sem UF ou (no exterior) sem país, a emissão
  devolve 422 `registration_incomplete` nomeando a entidade e o campo; o `0000000` de `cMun`, `cMunFG` e do ISSQN e o
  `cUF` 35 de reserva foram removidos. Número malformado num cadastro é 422 `invalid_decimal` (o `d()` entra em
  pânico tipado e `recoverDecimal`, na entrada do `Emit`, converte; qualquer outro pânico segue adiante).
- **Exterior:** endereço com `state_federation` `EX`, `city_ibge_code` `9999999`, sem CEP e com `country_code` (BACEN,
  4 dígitos, diferente de 1058) e `country_name`. A NF-e sai com `idDest` 3, `indIEDest` 9 e sem `IE`. A tabela BACEN
  está só na UI; a api valida o formato. Inscrição estadual `ISENTO` é o contribuinte isento (`indIEDest` 2, sem IE).
- **Portão:** regras do MOC só recusam a emissão quando estão em `blockRules` (`nfes/prevalidate.go`), e só regras
  "Obrig." entram ali; as facultativas por UF ficam no shadow. Uma regra nova em block precisa de teste que a dispare
  e de um documento coerente que passe (`nfes/f3_test.go`).


## Contingência SVC da NF-e

- `dfe.Request.Contingency` (`SVC-AN`/`SVC-RS`) troca só a URL; a UF real continua mandando no `cUF` e na operação WSDL. Só NF-e: NFC-e, CT-e e MDF-e não resolvem em SVC.
- As URLs da SVC não estão no MOC e precisam ser confirmadas no Portal Nacional antes de produção (`docs/specs/2026-11-08-contingencia.md`, Decisões pendentes).
- A SVC não oferece inutilização nem CC-e: nunca rotear esses serviços para ela. Cancelamento de nota autorizada na SVC usa a SVC indicada pelo `tpEmis` da chave.
- Forma de emissão por documento (`emission_type`, `auto_contingency`, `contingency_since`, `contingency_reason`): só os `PUT /{nfe,nfce,cte,mdfe}-emission` escrevem (`services.fiscalConfigService.SetEmission`); os `PUT /{x}-config` preservam. Tipos válidos por documento em `services/emission.go` (`emissionOptions`); fora disso 422 `emission_type_invalid`; sair de `normal` sem justificativa de 15 a 256 caracteres, 422 `emission_reason_required`. `contingency_since` é mantido enquanto continua em contingência e removido ao voltar a `normal`.
- `PUT /nfe-contingency` é legado e compatível (`enabled` = `svc`, desligar = `normal`); `contingency_mode` é lido como `emission_type`.
- Trocar a forma de emissão muda o `tpEmis`, logo a chave de acesso e a assinatura: o documento é sempre regerado, nunca remendado.

## Retransmissão de contingência

- Contingência automática: nunca regerar um documento em voo sem a prova por consulta da chave original
  (`autocontingency.Reissue`); sem prova (consulta falhou ou cedo demais), o original fica pendente e alerta. A constante
  `autocontingency.SwitchEnabled` é o interruptor mestre e só liga junto com os itens "o que falta para ligar" da seção 4.1 do spec.
- Contingência automática, ligação (spec 4.1): o disjuntor só é escrito pelo worker (`auto_breaker`, compare-and-swap no valor anterior,
  uma releitura e depois descarta a observação); a API só lê. Conta como indisponibilidade apenas o que `autocontingency.Classify` lista;
  rejeição de negócio, 429 e 4xx do egress nunca contam. Só autorização `tpEmis` 1 sem rota SVC alimenta o disjuntor (documento em
  contingência não diz nada do autorizador normal), e só quando o ambiente da resposta é o da configuração. A volta ao normal e o fechamento do
  disjuntor são UMA escrita (o disjuntor nunca fecha com a linha esquecida em contingência). Tudo atrás de `SwitchEnabled`/`switchEnabled`
  (testes viram o interruptor, produção nunca). `go-dfe/autocontingency` é o único lugar da lógica; não copiar para api nem worker.
- NF-e EPEC: o documento só entra no `contingency-index` DEPOIS do evento 110140 registrado (o worker arma); `contingency_state=awaiting_epec` é o estado sem `contingency_sweep`. Quem mexer no fluxo mantém a ordem: assinar (SignOnly), enfileirar o evento no outbox junto do documento, armar no worker. Rejeição do evento apaga `contingency_msg` e rejeita o documento; nunca armar antes.
- `contingency-index` (GSI esparso `contingency_sweep`/`contingency_deadline`, KEYS_ONLY) nas tabelas `nfes`, `nfces`, `mdfes`, `ctes`: só documentos EPEC/offline não conciliados carregam `contingency_sweep = "1"`. Quem grava um documento pendente deve gravá-lo; quem o concilia deve removê-lo.
- `contingency-dispatcher` (15 min) lê o índice e invoca `contingency-retransmit-worker` de forma assíncrona; falha vai ao `dlq-processor`. Nunca Scan.
- Autorização fora do prazo (cStat 150, ou 100 após o `contingency_deadline`) é sucesso (`authorized_late`). Reenvio mantém chave e `cNF`.
- O reenvio pelo egress ainda não está ligado (depende da emissão offline guardar o XML assinado); hoje o worker só marca `overdue`.

## NFC-e offline: o certificado só existe no egress

A API nunca assina. O QR Code v3 offline e o XML da NFC-e com `tpEmis=9` são assinados pelo `go-dfe-egress` (`nfce/SignText`
e `nfce/SignOnly`); o XML assinado fica em `contingency_msg` e é transmitido depois com `Body["@signed_xml"]`, sem remontar
nem reassinar (a chave e o `cNF` não mudam). `contingency_msg` carrega a senha do certificado: o `contingency-retransmit`
remove o atributo em todo desfecho terminal.

MDF-e e CT-e off-line (`tpEmis=2`) seguem a mesma regra (`mdfe`/`cte` `SignText` e `SignOnly`); prazo de 168 h (MOC), não os 3 dias da
NFC-e. Na SVC do CT-e (7/8) **não** se informa `dhCont`/`xJust` nem `sign` (o oposto da NF-e): copiar o desenho da NF-e aqui
é rejeição 586. A UF para SVC-RS/SVC-SP e as URLs da SVC do CT-e estão não verificadas (Portal Nacional bloqueia robôs).

## Eventos restantes (F9): autorizador, idempotência e o que não foi verificado

- **Autorizador por evento da NF-e.** Reforma tributária (112110, 112140, 112150, 211110, 211124, 211128, 211130, 211140,
  211150, 212110, 212120) e o cancelamento 110001 vão à SVRS (`WorkerMessage.UF = "SVRS"`, `dfe.EventAuthorizerSVRS`;
  o `Resolve` do go-dfe aceita "SVRS" como "AN", direto no registro). Comprovante de entrega (110130/110131) e EPEC (110140)
  só existem no Ambiente Nacional (`UF = "AN"`, `cOrgao 91`). O ator interessado (110150) vai à UF do emitente. O endereço
  exato do serviço de eventos da reforma na SVRS não está no MOC ("Portal Nacional, Relação de Serviços Web"): foi assumido
  o `recepcaoevento4.asmx` da SVRS e precisa ser conferido em homologação.
- **Evento já registrado é sucesso.** O worker trata 573 (NF-e/NFC-e) e 631 (CT-e/MDF-e) como sucesso do evento quando ele
  não é cancelamento nem encerramento (esses seguem `rejectedEventDocStatus`). Antes, o 573 de uma manifestação ou CC-e
  ficava `rejected`.
- **Layouts sem XSD no repositório.** Só NF-e 110130, 110131, 110150 e 110300 seguem sem XSD (o portal devolve zip vazio para
  `Evento_AtorInteressado_PL_v1.01.zip` e não publica pacote de comprovante de entrega); valem as tabelas do MOC.
- **Fora desta rodada:** 112120 e 112130 (o MOC nomeia tags de forma contraditória ou sem nome), 211120 (removido pela LC
  227/2026), 412120 e 412130 (autor é o Fisco). O EPEC da NF-e tem rota (ver abaixo, 2026-11-08) e a opção `epec` está em `emission.go` só para a NF-e.

- **Vinculação de pagamento do CT-e (110300 e 110301).** Layout dos XSDs vendorizados (`evVincPgto`, `evCancVincPgto`; o
  elemento do cancelamento é `evCancVincPgto`, com `p` minúsculo, e não `evCancVincPGto` como no catálogo da spec). A api não
  restringe o autor nem o prazo e não roteia para a SVRS: nenhum documento do repositório diz quem autoriza (decisão pendente,
  ver spec do CT-e). O meio de pagamento reaproveita a tabela `tPag` da NF-e na UI.

## XSDs de evento

- Vendorizados em `go-dfe/schemas/{nfe,cte,mdfe}/<pacote>` junto dos XSDs do documento (README e `SHA256SUMS` em
  `go-dfe/schemas`; `validate/schemas_hash_test.go` falha se um XSD sumir, aparecer sem registro ou mudar). Pacotes: NF-e
  `PL_010e_v1.02`; CT-e `PL_CTe_400_NT2026.005 RTC SN_1.00` (portal, 15/10/2026); MDF-e `PL_MDFe_300b_NT012025_1.04.zip`
  do portal (pasta interna 1.05). Atualizar um pacote: copiar, regerar `SHA256SUMS`, `go run gen/main.go`, rodar os testes.
- Os specs de evento são gerados por `gen/main.go` (um por evento; `detEvento` repete o nome de elemento em todo
  `eNNNNNN`, por isso cada evento carrega o próprio registro de tipos). `xsdorder` é conferido contra eles; evento que
  nenhum builder emite fica em `notBuilt` no teste e ganha tabela quando alguém criar o builder.
- Correções que o XSD impôs sobre a leitura das tabelas do MOC: 211110 `gIBSCredPres`/`gCBSCredPres` só têm
  `pCredPres` e `vCredPres` (o `cCredPres` é do item); `descEvento` do 212110 e 212120 é "... em Operação de Sucessão"
  (CBS: "Crédito de CBS"); EPEC 110140: `IE` do emitente obrigatório, `vNF`/`vICMS`/`vST` dentro de `dest` e `vNF`/`vICMS`
  também depois de `dest` (e110140 do pacote 010e; o pacote avulso do portal não tem os dois últimos); CT-e: `Id` com
  3 dígitos de sequência, `descEvento` "do CT-e" nos comprovantes de entrega, `nTentativa` com 3 dígitos, `xObs` do 610110
  obrigatório; MDF-e: o encerramento (110112) agora leva o grupo `evEncMDFe` dentro de `detEvento` (antes os campos iam
  soltos) e `dtEnc` usa a data de Brasília.

## CIOT (ANTT): cliente, gerador e piso

- Testes nunca falam com a ANTT real nem com qualquer host real: só `ciot/ciottest` (TLS com certificado de cliente
  obrigatório). `ciot.SetTestHosts` deve receber todos os ambientes que o teste toca numa única chamada; um ambiente
  ausente cai no host real (isso já aconteceu uma vez: um teste do próprio desenvolvimento chegou ao host de homologação
  com chave e certificado falsos; corrigido no teste).
- Chave do gerador e URL de contingência nunca entram em log, erro ou resposta; a biblioteca recusa vazio e `CHANGE_ME`
  ao construir o gerador. Quem lê o SSM (worker, C4) precisa de `ssm:GetParameter` e `kms:Decrypt` com escopo de caminho;
  o egress não tem cliente AWS e recebe os valores no corpo.
- O piso mínimo só calcula com tabela marcada `verified`. A tabela embutida (`floor_table.json`) é gerada por script da
  implementação de referência do dono (`ciot-api/internal/freight`), nunca digitada; a publicação oficial da ANTT não foi
  lida automaticamente. Eixos pedidos: coluna inferior primeiro, superior só sem inferior (Nota 1); cada perna arredonda
  em centavos. O piso é só aviso na tela (a ANTT decide).
- Mapeamentos entre código ANTT e regra inferidos pela posição na tabela da spec estão marcados `Inferred` em
  `ciot/rejections.go`; confirmar em homologação.

## CIOT: nenhum teste alcança a ANTT real

`ciot.HostFor` recusa o host real quando roda dentro de um binário de teste (`testing.Testing()`): sem `SetTestHosts`
para o ambiente, devolve erro em vez de `appservices(-hml).antt.gov.br`. A guarda existe porque um teste do C1 chegou
uma vez ao host de homologação (um `defer SetTestHosts` dentro de laço desfez o desvio do ambiente seguinte; foram
enviados só um certificado descartável e uma chave falsa, e a ANTT respondeu 403). O mapa ambiente para host real fica em
`realHostFor`, que só os testes de constantes usam, sem abrir conexão.

## CIOT: vínculo com o MDF-e

- O CIOT entra no MDF-e por `ciot_id` (declarado) ou por código e responsável (externo); nunca os dois. O código do XSD é `[0-9]{12}`: um identificador do gerador que não seja numérico é recusado na emissão (422), não corrigido.
- O vínculo (`mdfe_access_key`) é gravado na transação da emissão; desvincular é do worker quando o MDF-e é rejeitado ou falha. Cancelar ou encerrar o MDF-e só marca `mdfe_cascade` no CIOT: nenhuma operação na ANTT parte sozinha.
- Os workers `mdfe-emission` e `mdfe-event` têm `ciots` e `ciot_audit` e o tópico de alertas (`linksCiots` no CDK).

## CIOT: API e worker

- O identificador de 12 caracteres nunca nasce na API: só o worker o pede ao gerador. A linha do `ciots` usa um id local (ULID) na SK.
- Toda operação do ciclo de vida marca `pending_operation` na linha com condição (nenhuma em andamento e estado aberto), junto do
  outbox e da linha de auditoria `request:*`, na mesma transação. Segunda operação concorrente é 409 `ciot_invalid_state`.
- Corpo enviado e resposta da ANTT vão para `ciot_audit` (cifrado em repouso pelo DynamoDB, hash SHA-256 do corpo) e nunca para log.
- O piso mínimo só calcula com a tabela verificada; se ela for marcada não verificada o endpoint responde 503 (`ciot_floor_unverified`) e a tela mostra "indisponível". O aviso de piso é da tela, não do servidor.
- Worker `ciot-operation`: rejeição de negócio da ANTT (HTTP 422 do egress) é terminal e a função retorna sucesso; transporte, 5xx e 502
  liberam a lease e retornam erro. Chave do gerador e URL de contingência vêm do SSM por invocação e nunca entram em log, resultado SNS
  ou corpo de auditoria de outra chamada. Só este worker lê `/ctech-dfe/*/ciot/*`; o egress não tem acesso ao SSM.

## CT-e: leiaute dos modais e DACTE

- Todo modal novo do CT-e entra no `xsdorder` com escopo `infCTeNorm:infModal:<modal>` e no `TestXSDOrderMatchesCTeModalSpecs`; chave sem escopo é do MDF-e.
- DACTE: nunca somar `infQ` de `tpMed` diferentes; o protocolo recebido sempre é impresso (Anexo II 2.1).

- **MDF-e, auditoria MOC (2026-11-09):** enumerações de cadastro seguem o XSD (tpValePed 01|04, tpRod 01..06,
  RNTRC 8 dígitos); NFF (`infSolicNFF`, `tpEmis=3`) e PAA (`infPAA`, `procEmi=4`) estão fora do escopo até decisão do dono.

- **NF-e, auditoria MOC (2026-11-09, `docs/specs/2026-11-09-auditoria-moc-nfe.md`):** IPITrib só para CST 00, 49, 50, 99 (os demais são IPINT);
  PIS/COFINS CST 03 usa `qBCProd` = quantidade tributável; grupos opcionais do XSD saem inteiros ou não saem (ICMS60/ICMSSN500 levam
  `pRedBCEfet` com `0.00` e só emitem `[vBCSTRet, pST, vICMSSTRet]` com `pST`); `ICMSTot` soma `vFCPSTRet` e as tags monofásicas dos itens
  e o `vNF` só abate `vICMSDeson` de item com `indDeduzDeson=1` e soma `vICMSMonoReten`. `tPag` 23, 24 e 91 valem na API e na UI.
  UI: CST do ICMS e `motDesICMS` seguem o XSD por CST (`ui/src/lib/data/is.ts`). `dhEmi` e o AAMM da chave usam o fuso da UF do emitente
  (`go-dfe/uftz`); CPF no emitente entra na chave como `000`+CPF; `dest/ISUF` sai de `person.isuf_emit` do destinatário.

- **Portão de schema por tipo, padrão shadow (2026-11-09).** `dfecheck.Gate` sabe recusar em 422 `fiscal_prevalidation` os
  achados deterministas do XSD (`required`, `choice_*`, `max_occurs`, `pattern`, `enum`, `length`, tamanhos) de NF-e, NFC-e,
  CT-e, CT-e OS e MDF-e, e `GateEvent` os dos eventos (NF-e, CT-e, MDF-e), antes de gravar o evento, mas por decisão do dono o
  padrão de todos os tipos é `shadow`: os achados só vão ao log e só o tamanho continua recusando, como antes. Rollout: shadow
  primeiro, observar `dfecheck: achado pré-SEFAZ` em homologação, depois promover tipo a tipo com
  `DFE_GATE_SCHEMA_<TIPO>=block` (ou `DFE_GATE_SCHEMA=block`); modos `off|shadow|warn|block`. Os testes de paridade provam que todo
  corpo válido de builder tem zero achados (`events_parity_test.go`, `gate_schema_test.go`, `f3_test.go`), o que sustenta a
  promoção. Recuar um tipo promovido: `shadow` na variável do Lambda, sem deploy de código.
  `unknown_tag`, `attr_required` e os numéricos (`total_digits`, `fraction_digits`, `min/max_inclusive`) ficam fora: tag
  desconhecida pode ser NT ainda sem XSD vendorizado (overlay) e os padrões TDec já cobrem casas e faixa. Mensagens trazem só
  caminho XML e motivo (sem o valor, que pode ser PII). O `Mode` de regra do catálogo (26 block na NF-e) é metadado: a api só
  recusa o que `blockRules` lista; promover as 26 (DV de CNPJ/CPF e GTIN, chave A03-10/B23-10) é decisão pendente, ver o relatório.
  Validador: sequence opcional do XSD é tudo-ou-nada (`grp`/`greq` no spec) e elemento repetido em braços da mesma choice não conta
  braço (IPI de `imposto`); regenerar os specs com `go run gen/main.go` (em `go-dfe/validate`) depois de mexer no gerador.
