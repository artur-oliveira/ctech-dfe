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

## NFS-e (F1 — modelo de dados e cadastros; F2 — go-dfe/nfse, provider nacional)

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
  inventar o endpoint de produção a partir do hostname de homologação.
- **IBS/CBS é obrigatório no catálogo de serviços NFS-e**: `c_ind_op`, `cst`, `c_class_trib`,
  `ind_dest` e `fin_nfse=0`. Não inferir classificação tributária a partir da descrição do serviço.
- **`MunicipalParameters` e `GetDANFSE` chamam `dfe.Call` direto do serviço, não pelo worker** — são
  leituras públicas, sem escrita e sem risco de timeout longo. O par certificado/senha vem de
  `ExternalService.CertificateB64`, compartilhado com a consulta de cadastro da NF-e.
- **As regras de evento moram em `go-dfe/nfse/constants.go`, não em `nacional`**
  (`ContribuinteEvents`, `EventsRequiringMotivo`, `EventsRequiringXMotivo`): a api valida antes de
  enfileirar e a `nacional.BuildPedRegEvento` valida ao serializar — duas cópias da regra
  divergiriam. Mesmo precedente de `BuildIDDPS`.
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
- **Texto de status usa `text-warning` / `text-success`, nunca `amber-600` / `green-600`.** Os
  defaults do Tailwind falham AA nos tamanhos usados (amber-600 ≈ 3.19:1, green-600 ≈ 3.35:1 sobre
  branco). `globals.css` ancora `--color-warning` em amber-700 e `--color-success` em green-700,
  como já fazia com `--color-danger` (red-600) e `--color-gray-400` (slate-500). Estados de saldo
  também carregam um glifo (`✓` / `⌛` / `↩`) para não depender só de cor.
- **Nada de "eyebrow" (`text-xs uppercase tracking-wider`) como título de seção.** Rótulo de seção
  é `text-sm font-medium text-gray-600`. O eyebrow repetido em toda seção é ruído, não hierarquia.
- **Alvos de toque ≥ 44px em ações primárias.** Não use `size="sm"` (h-7) em botão de barra de
  ação — o `size` default já é `min-h-11 sm:h-8`.
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
- Profile and password management endpoints do not exist — those belong to ctech-account.
- **Membership is owned by the `organization_users` table** (via `MembershipService`). RBAC,
  `/auth/me`, `GET /organizations`, and the WebSocket all resolve access through
  `MembershipService.Get`. The legacy embedded `users.organizations` list is dead — no longer read
  or written (dual-write/fallback removed post-migration); do not reintroduce authorization from it.
- Removing/demoting a member must not leave an org without an OWNER, and mutating a membership must
  invalidate its cache (a tombstone on removal) — do this through `MembershipService`, not the repo.
- Creating an organization is KYC-gated and atomic: org + certificate + OWNER membership + audit in
  one `TransactWrite`. A certificate is required unless a matriz certificate (same CNPJ root) is
  inherited. Invitations grant only ADMIN/USER/VIEWER and are single-use — never weaken these.
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
  `/etc/nginx/conf.d/realip.conf` with the VPC CIDR plus CloudFront's origin-facing ranges,
  read from the AWS-managed prefix list `com.amazonaws.global.cloudfront.origin-facing` via the
  EC2 dual-stack endpoint (the instances are IPv6-only and `ip-ranges.amazonaws.com` has no AAAA
  record) and refreshed by a daily systemd timer.
- **Never key on the leftmost `X-Forwarded-For` entry.** CloudFront and the ALB *append* to the
  header, so a client can prepend anything it likes. `real_ip_recursive on` walks the chain
  right-to-left and discards only trusted hops, which is what makes the result unforgeable.
- nginx **overwrites** `X-Forwarded-For` with the resolved IP (`proxy_set_header X-Forwarded-For
  $remote_addr`) rather than appending, so the Go app's `TRUSTED_PROXIES` / Fiber `c.IP()` — which
  reads the leftmost entry — cannot be fed a forged value.

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
- The worker invokes go-dfe in-process when supported and the Python Lambda as fallback.
- After SEFAZ response: update DynamoDB, upload XML to S3, publish terminal results to SNS.
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
