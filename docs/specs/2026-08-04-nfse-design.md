# Design — Módulo NFS-e (Nacional + ABRASF 2.04)

**Data:** 2026-08-04
**Status:** aprovado (design) — pendente plano de implementação
**Escopo:** novo tipo de documento fiscal (NFS-e) no ctech-dfe, atravessando `go-dfe`, `api`,
`worker`, `ui` e `cdk`.

---

## 1. Contexto

O ctech-dfe emite NF-e, NFC-e, CT-e e MDF-e via SEFAZ (SOAP + mTLS). NFS-e é um documento de
natureza diferente:

- **Competência municipal**, não estadual. Não existe "UF autorizadora"; existe **município emissor**
  (código IBGE, 7 dígitos).
- O **Sistema Nacional NFS-e** (Sefin Nacional) usa **REST + JSON com payload XML gzip+base64**, não
  SOAP.
- O padrão **ABRASF 2.04** usa SOAP, com WSDL e endpoint próprios por município.
- O documento enviado não é a nota: o contribuinte envia uma **DPS** (Declaração de Prestação de
  Serviço, padrão nacional) ou um **RPS** (Recibo Provisório de Serviços, ABRASF); o fisco valida e
  **gera** a NFS-e.

### Fontes normativas usadas

| Artefato | Local |
|---|---|
| XSD nacional 1.01 (DPS, NFSe, evento, pedRegEvento, tipos) | `tmp/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/` |
| Swagger Sefin Nacional | `tmp/nfse-sefin.json` |
| Swagger ADN Contribuintes / DANFSE / Parâmetros Municipais | `tmp/nfse-adn-contribuintes.json`, `tmp/nfse-danfse.json`, `tmp/nfse-parametros-municipais.json` |
| Anexo I — leiaute e RN de DPS/NFS-e | `tmp/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx` |
| Anexo II — leiaute e RN de eventos | `tmp/anexo_ii-sefin_adn-pedregevt_evt-snnfse-v1-01-20260122.xlsx` |
| Anexo B — código de tributação nacional / NBS 2.0 | `tmp/anexo_b-nbs2-lista_servico_nacional-snnfse-v1-01-20260122.xlsx` |
| Anexo C — `indOp` IBS/CBS | `tmp/anexo_c-indop_ibscbs-snnfse-v1-01-20260122.xlsx` |
| Anexo A — municípios IBGE / países ISO2 | `tmp/anexo_a-municipio_ibge-paises_iso2-v1-00-snnfse-20251210.xlsx` |
| ABRASF 2.04 — WSDL, XSD, MOC, tabela de erros | `tmp/abrasf/` |
| Manuais do contribuinte (emissor público + ADN) | `tmp/manual-contribuintes-*.md` |

### Ambientes

| | Produção restrita | Produção |
|---|---|---|
| Sefin Nacional | `https://sefin.producaorestrita.nfse.gov.br/SefinNacional` | `https://sefin.nfse.gov.br/SefinNacional` |
| ADN contribuintes | `https://adn.producaorestrita.nfse.gov.br/contribuintes` | `https://adn.nfse.gov.br/contribuintes` |
| DANFSE | `https://adn.producaorestrita.nfse.gov.br/danfse` | `https://adn.nfse.gov.br/danfse` |
| Parâmetros municipais | `https://adn.producaorestrita.nfse.gov.br/parametrizacao` | `https://adn.nfse.gov.br/parametrizacao` |

ABRASF: endpoint por município, configurado por organização — sem tabela embutida.

### Decisões de escopo (fechadas com o usuário)

| Decisão | Escolha |
|---|---|
| Provedores na v1 | Sefin Nacional **e** ABRASF 2.04 (motor genérico, sem cidade específica) |
| Teresina-PI | Nenhum adapter próprio — o município está migrando para o padrão nacional |
| IBS/CBS (reforma tributária) | **Completo** na v1 |
| `tpEmit` | Prestador (1), Tomador (2) e Intermediário (3) |
| Pipeline | Assíncrono, reusando outbox → SNS → SQS → worker |
| DANFSE | Nacional: proxy do PDF do ADN. ABRASF: não suportado na v1 (501) |
| Distribuição ADN (`GET /DFe/{NSU}`) | Incluída |
| Catálogo de serviços | Tabela nova `organization_services` |
| Pessoas | Estender o objeto `person` compartilhado — cobre `organizations` e `organization_persons` |

---

## 2. Arquitetura

```
ui  ──HTTP──▶  api  ──transact_write──▶  DynamoDB (nfses + worker_outbox)
                │                              │
                │ 202 + operation_id           │ Stream
                ▼                              ▼
            WebSocket ◀── results SNS ◀── worker ── go-dfe/nfse ──┬─▶ Sefin Nacional (REST+mTLS)
                                                                  └─▶ Município ABRASF (SOAP+mTLS)
```

Mesma espinha dorsal da NF-e. Duas diferenças:

1. O worker chama `dfe.Call` **in-process** (go-dfe). Nunca cai no py-dfe — py-dfe não tem NFS-e.
2. A comunicação com o Sefin Nacional é **síncrona** (POST devolve a NFS-e gerada). Isso ocorre
   dentro do worker; a API continua respondendo 202 imediatamente.

**Por que manter o pipeline assíncrono para uma API síncrona:** idempotência, lease de processamento,
retry com backoff, DLQ e a atualização por WebSocket já existem e são compartilhados. Chamar o Sefin
direto do handler HTTP duplicaria tudo isso e exporia o timeout de 29s do API Gateway.

---

## 3. Modelo de dados

### 3.1. `organization_services` (nova)

Catálogo de serviços — análogo direto a `organization_products`.

| Campo | Tipo | Descrição |
|---|---|---|
| `pk` | S | `{org_pk}` |
| `sk` | S | `SERVICE_{uuid}` |
| `code` | S | Código interno (`cIntContrib`). GSI `code-index` |
| `description` | S | Discriminação do serviço (`xDescServ`, até 2000 chars). GSI `description-index` |
| `trib_nacional_code` | S | Código de tributação nacional, 5 dígitos (Anexo B) — item+subitem+desdobro |
| `trib_municipal_code` | S | `cTribMun` — código municipal, opcional |
| `nbs_code` | S | `cNBS`, 9 dígitos — opcional |
| `cnae` | S | CNAE (usado por ABRASF) — opcional |
| `unit` | S | Unidade |
| `value` | S | Valor unitário, decimal string |
| `iss` | M | `{trib_issqn, aliquota, tp_ret_issqn, tp_imunidade, exig_susp{tp, n_processo}, bm{tp, n_ben, vl_red}, c_pais_resultado}` |
| `federal` | M | `{cst_pis_cofins, aliq_pis, aliq_cofins, tp_ret_pis_cofins, v_ret_cp, v_ret_irrf, v_ret_csll}` |
| `ibs_cbs` | M | `{c_ind_op (Anexo C), cst, c_class_trib, ind_dest, tp_oper, fin_nfse}` — defaults do serviço |
| `tot_trib` | M | `{ind_tot_trib, p_tot_trib_sn, v_tot_trib{...}, p_tot_trib{...}}` — Lei da Transparência |
| `created_at` / `updated_at` | S | ISO-8601 UTC |

**GSIs:** `code-index` (pk + code), `description-index` (pk + description) — idênticos aos de
`organization_products`.

### 3.2. `organization_persons` **e** `organizations` — extensão única

Nenhum campo existente muda. Adiciona-se um grupo `nfse` opcional ao objeto `person`:

```
nfse: {
  im,            # inscrição municipal (TSInscMun)
  caepf,         # Cadastro de Atividade Econômica da Pessoa Física
  nif,           # Número de Identificação Fiscal (exterior)
  c_nao_nif,     # 0 não informado | 1 dispensado | 2 não exigência
  reg_trib: { op_simp_nac, reg_ap_trib_sn, reg_esp_trib },   # TCRegTrib
  foreign_address: { c_pais, c_end_post, x_cidade, x_estado_prov, x_lgr, nro, x_cpl, x_bairro }
}
```

**Uma extensão cobre as duas tabelas.** `PersonObjectBody` (`api/internal/api/v1/dto.go:41`) já é o
objeto `person` compartilhado por `OrganizationCreateBody`/`OrganizationUpdateBody` e por
`PersonCreateBody`/`PersonUpdateBody`. Adicionar o grupo ali estende `organizations` e
`organization_persons` no mesmo campo, sem duplicar schema nem validação.

Isso corresponde ao leiaute: `TCInfoPrestador` e `TCInfoPessoa` têm exatamente os mesmos campos de
identidade (`CNPJ`/`CPF`/`NIF`/`cNaoNIF`/`CAEPF`/`IM`/`end`/`fone`/`email`). Divergem só em dois
pontos — `xNome` é obrigatório em pessoa e opcional em prestador, e `regTrib` existe apenas no
prestador.

**`reg_trib` mora aqui, não na config.** A organização não é sempre o prestador: com `tpEmit` 2 ou 3
ela emite como tomador ou intermediário e o **prestador é uma pessoa do cadastro** — que precisa do
próprio `regTrib`, obrigatório em `TCInfoPrestador`. Manter `reg_trib` junto da identidade dá uma
fonte única para os dois casos, em vez de um valor na config da org e outro na pessoa, com regra de
precedência e risco de divergência. Em pessoas o grupo é opcional; só é exigido quando aquela pessoa
é usada como prestador numa emissão `tpEmit` 2/3, e a validação ocorre na emissão.

Motivo geral: o tomador/intermediário de NFS-e é a mesma entidade jurídica que o destinatário de
NF-e. Tabelas separadas obrigariam o usuário a manter o mesmo cliente cadastrado em dois lugares.

### 3.3. `organization_nfse_configs` (nova)

| Campo | Tipo | Descrição |
|---|---|---|
| `pk` | S | `{org_pk}` |
| `provider` | S | `nacional` \| `abrasf204` |
| `environment` | N | `1` produção \| `2` homologação (produção restrita) |
| `c_loc_emi` | S | Código IBGE do município emissor (7 dígitos) |
| `serie` | S | Série da DPS/RPS (até 5 chars) |
| `prod_current_number` / `hom_current_number` | N | Último `nDPS`/número de RPS emitido por ambiente — reservado por `IncrementNumber` |
| `abrasf` | M | `{endpoint_url, wsdl_version, codigo_municipio, envio_sincrono}` — só quando `provider=abrasf204` |
| `certificate_sk` | S | Certificado usado na transmissão (default: o da org) |
| `created_at` / `updated_at` | S | ISO-8601 UTC |

Inscrição municipal e regime tributário do prestador **não** ficam aqui — vêm do grupo `nfse` do
objeto `person` da própria organização (§3.2).

O contador segue a convenção já existente de `FiscalConfigRepository.IncrementNumber`, que opera
sobre `{envPrefix}_current_number` (`api/internal/repositories/fiscal_config.go:91`) — não se
introduz um campo `next_number` divergente. Mesma forma e mesmo serviço-base dos configs existentes
(`fiscal_configs.go`).

### 3.4. `nfses` (nova)

| Campo | Tipo | Descrição |
|---|---|---|
| `pk` | S | `{env}#{CNPJ}` — igual a `nfes` |
| `sk` | S | **`id_dps`** — ver decisão abaixo |
| `access_key` | S | Chave de acesso da NFS-e (50 dígitos). GSI `access-key-index`. Ausente até o fisco gerar |
| `provider` | S | `nacional` \| `abrasf204` |
| `serie`, `number` | S/N | Série e número da DPS/RPS |
| `status` | S | `pending` \| `processing` \| `authorized` \| `rejected` \| `cancelled` \| `error` |
| `tp_emit` | N | 1 prestador \| 2 tomador \| 3 intermediário |
| `c_motivo_emis_ti` | N | Obrigatório quando `tp_emit` ∈ {2,3} |
| `payload` | M | Documento neutro completo (§4.2), como persistido na emissão |
| `s3_key_dps`, `s3_key_nfse` | S | XMLs no S3 |
| `verification_code` | S | ABRASF: `CodigoVerificacao` |
| `processing_owner`, `processing_lease_until` | S | Lease do worker (idêntico a NF-e) |
| `created_at` / `updated_at` | S | ISO-8601 UTC |

**Decisão de chave (SK): `id_dps`, não a chave de acesso.**

A regra de formação da chave está em `TSIdNFSe` (`tiposSimples_v1.01.xsd:34`):

```
"NFS" + cMun(7) + ambGer(1) + tpInsc(1) + inscFederal(14) + nNFSe(13) + AAAAMM(4) + cNum(9) + DV(1)
                                                            ^^^^^^^^^              ^^^^^^^^
```

Dois dos campos são gerados pelo fisco, não pelo emitente:

- `nNFSe(13)` — a documentação do XSD é explícita: *"A Sefin Nacional NFS-e irá gerar o número da
  NFS-e de forma sequencial por emitente. […] não irá reutilizar números inutilizados durante a
  geração da NFS-e."*
- `cNum(9)` — código numérico gerado pelo sistema nacional.

A chave de acesso, portanto, **só passa a existir na resposta HTTP 201 da Sefin**. Isso difere da
NF-e, onde a chave de 44 dígitos é inteiramente nossa (`cNF` aleatório gerado por nós, `nNF`
reservado por nós antes do envio) — por isso lá `sk = access_key` funciona e aqui não.

O `idDPS` (`TSIdDPS`, `tiposSimples_v1.01.xsd:47`) é composto só de campos que o emitente controla,
e é exatamente o que o `transact_write` reserva antes de publicar no outbox:

```
idDPS = "DPS" + cLocEmi(7) + tpInsc(1) + inscFederal(14) + serie(5) + nDPS(15)   // "DPS" + 42 dígitos
```

Isso não é preferência de modelagem — é imposto pelo pipeline. Entre `POST /v1.0/nfses` e a resposta
do fisco existe a janela outbox → SNS → SQS → worker, e nessa janela o registro precisa já existir
com chave imutável para sustentar idempotência, lease de processamento, retry, DLQ e o push por
WebSocket. As alternativas foram descartadas:

- gravar a linha só depois da resposta do fisco — perde a idempotência da requisição e o rastro das
  rejeitadas;
- gravar com SK placeholder e reescrever ao receber a chave — SK é imutável no DynamoDB, exigiria
  `delete` + `put`, o que emite dois eventos de stream (quebrando o outbox) e órfã as linhas filhas
  de `nfse_events`, cuja `pk` referencia a linha pai.

A própria API nacional adota o mesmo handle: `GET /dps/{id}` e `HEAD /dps/{id}` existem para
*"verificar se uma NFS-e foi emitida a partir do Id do DPS"*. O caminho de recuperação em retry é
chaveado por `idDPS`, não por chave de acesso — nossa SK e o handle de recuperação do fisco ficam
idênticos, sem tabela de mapeamento intermediária.

Para ABRASF, a mesma posição recebe `RPS{codigoMunicipio}{cnpj}{serie}{numero}` com o mesmo
comprimento fixo. `access_key` fica como atributo, indexado pela GSI `access-key-index`, e é
preenchido quando a nota é gerada.

**Nota sobre o documento emitido:** o contribuinte assina e envia **`TCDPS`** (`DPS_v1.01.xsd:9`);
o fisco devolve **`TCNFSe`** (`NFSe_v1.01.xsd:10`), que embute a DPS enviada
(`TCInfNFSe` termina com `<xs:element name="DPS" type="TCDPS"/>`). `TCNFSe` nunca é assinado por
nós — só recebido e persistido. A única rota que recepciona uma NFS-e pronta é
`POST /decisao-judicial/nfse`, destinada ao município, fora do escopo deste módulo.

**GSIs:** `access-key-index` (pk + access_key), `number-index` e `date-index` espelhando `nfes`.

### 3.5. `nfse_events` (nova)

`pk = {id_dps}` · `sk = {uuidv7}`. Reusa `DocumentEventRepository` (`NewDocumentEventRepository(db,
cfg, "nfse")`) sem alteração de código.

| Campo | Descrição |
|---|---|
| `event_type` | `101101`, `105102`, `101103`, `202201`, `203202`, `204203`, `202205`, `203206`, `204207`, `205208` |
| `sequence` | `nSeqEvento` |
| `status`, `payload`, `s3_key`, `protocol` | idem eventos de NF-e |

### 3.6. Distribuição ADN

Reusa a tabela `distributions` existente com `doc_type = "nfse"` e um cursor de **NSU** por
organização (o ADN é paginado por NSU sequencial, não por `ultNSU`+`maxNSU` como o DistDFe da NF-e).

### 3.7. Tabelas de referência (embutidas no código, não no DynamoDB)

- Municípios IBGE e países ISO2 (Anexo A) → `ui/src/lib/data/cities.ts` já existe; estender com
  código IBGE de 7 dígitos se faltar.
- Lista de serviços nacional / NBS 2.0 (Anexo B) → gerado como tabela Go + JSON para o front.
- `indOp` IBS/CBS (Anexo C) → tabela Go.

Geradas por script a partir dos `.xlsx`, versionadas no repositório. Não são dados por tenant.

---

## 4. `go-dfe` — camada NFS-e

### 4.1. Entrada

Mantém-se `dfe.Call(req)`. Adiciona-se `constants.DocTypeNFSE = "nfse"` e os serviços:

| Serviço | Nacional | ABRASF 2.04 |
|---|---|---|
| `NFSeRecepcao` | `POST /nfse` | `RecepcionarLoteRpsSincrono` / `GerarNfse` |
| `NFSeConsulta` | `GET /nfse/{chaveAcesso}` | `ConsultarNfsePorRps` / `ConsultarNfsePorFaixa` |
| `NFSeConsultaDPS` | `GET /dps/{id}` | `ConsultarLoteRps` |
| `NFSeEvento` | `POST /nfse/{chave}/eventos` | `CancelarNfse` / `SubstituirNfse` |
| `NFSeConsultaEvento` | `GET /nfse/{chave}/eventos/...` | — |
| `NFSeDistribuicao` | `GET /DFe/{NSU}` (ADN) | — |
| `NFSeDANFSE` | `GET /danfse/{chave}` | — |
| `NFSeParametrosMunicipais` | `GET /parametrizacao/...` | — |
| `NFSeServicoPrestado` | — | `ConsultarNfseServicoPrestado` |
| `NFSeServicoTomado` | — | `ConsultarNfseServicoTomado` |

`Implements()` retorna `true` para todos — não há fallback py-dfe.

`dfe.Request` é reusado como está. O campo `UF` não se aplica a NFS-e e fica vazio; o município
emissor e o provider vêm no `Body`. Roteamento de endpoint por `(provider, environment)`.

### 4.2. Modelo neutro

```go
package nfse

// Document é o modelo neutro de emissão. Moldado no DPS 1.01 (o leiaute mais
// rico); ABRASF mapeia o subconjunto que suporta.
type Document struct {
    Ambiente     int
    TpEmit       int      // 1 prestador | 2 tomador | 3 intermediário
    MotivoEmisTI int      // obrigatório quando TpEmit != 1
    ChNFSeRej    string   // quando TpEmit != 1 e motivo == 4
    Competencia  Date
    Serie        string
    Numero       int
    CLocEmi      string   // IBGE 7
    Substituicao *Substituicao

    Prestador    Prestador   // TCInfoPrestador + regTrib
    Tomador      *Pessoa     // TCInfoPessoa
    Intermediario *Pessoa

    Servico  Servico   // TCServ: locPrest, cServ, comExt, obra, atvEvento, infoCompl
    Valores  Valores   // TCInfoValores: vServPrest, descontos, dedRed, trib
    IBSCBS   *IBSCBS   // TCRTCInfoIBSCBS
}
```

Cobertura de grupos do DPS 1.01 (todos modelados):

- `prest` / `toma` / `interm` — CNPJ/CPF/NIF/cNaoNIF, CAEPF, IM, endereço nacional ou exterior,
  contato; `regTrib` (`opSimpNac`, `regApTribSN`, `regEspTrib`) no prestador.
- `serv.locPrest` — `cLocPrestacao`, `cPaisPrestacao`.
- `serv.cServ` — `cTribNac`, `cTribMun`, `xDescServ`, `cNBS`, `cIntContrib`.
- `serv.comExt` — modo de prestação, vínculo, moeda, mecanismos de apoio ao comércio exterior,
  movimentação temporária de bens, DI/RE, MDIC.
- `serv.obra` — `inscImobFisc`, `cObra`, `cCIB`, endereço da obra.
- `serv.atvEvento` — nome, período, identificador, endereço.
- `serv.infoCompl` — DRT, documento de referência, pedido, item do pedido, informação complementar.
- `valores.vServPrest` — `vReceb`, `vServ`.
- `valores.vDescCondIncond`, `valores.vDedRed` (com lista de documentos de dedução/redução).
- `valores.trib.tribMun` — `tribISSQN`, `cPaisResult`, `tpImunidade`, `exigSusp`, `BM`, `tpRetISSQN`,
  `pAliq`.
- `valores.trib.tribFed` — PIS/COFINS, `vRetCP`, `vRetIRRF`, `vRetCSLL`.
- `valores.trib.totTrib` — Lei da Transparência.
- `IBSCBS` — `finNFSe`, `indFinal`, `cIndOp`, `tpOper`, `gRefNFSe`, `tpEnteGov`, `indDest`, `dest`,
  `imovel`, `valores` (situação/classificação tributária, tributação regular, diferimento, crédito
  presumido, totais por UF/município/federal).

### 4.3. Provider

```go
type Provider interface {
    Emit(ctx context.Context, doc Document)          (Result, error)
    Event(ctx context.Context, ev EventRequest)       (Result, error)
    QueryByKey(ctx context.Context, key string)       (Result, error)
    QueryEvents(ctx context.Context, f EventFilter)   (Result, error)
}
```

**`nacional`** — serializa `Document` → DPS XSD 1.01, assina, gzip, base64, `POST` JSON com mTLS.
Resposta: `nfseXmlGZipB64` → descompacta → NFS-e XML + `chaveAcesso` + `idDps`.

**`abrasf204`** — serializa `Document` → `InfDeclaracaoPrestacaoServico` (XSD 2.04), assina, envelopa
em SOAP, envia com mTLS. Campos sem equivalente (IBS/CBS, `comExt` completo, `atvEvento`) fazem o
adapter **falhar explicitamente** com erro identificando o campo — nunca descartar em silêncio.

### 4.4. Assinatura

`xmlops.Sign` existente, sem alteração: XML-DSig enveloped, RSA-SHA1, digest SHA-1, C14N 1.0
(`REC-xml-c14n-20010315`) — confirmado nos manuais nacionais e no MOC ABRASF. É exatamente a
configuração já usada por NF-e/CT-e/MDF-e.

- Nacional: assina `infDPS` (atributo `Id`), e `infPedReg` no pedido de registro de evento.
- ABRASF: assina `InfDeclaracaoPrestacaoServico` e, quando o município exigir, o `LoteRps`.

### 4.5. Transporte

- Nacional: `net/http` com `tls.Config{Certificates: [...]}` (mTLS, mesmo certificado A1 já carregado
  por `internal/certificate`). gzip via `compress/gzip`. Sem SOAP.
- ABRASF: reusa `internal/soap/envelope.go` e `internal/services/client.go`.

Retry/backoff: reusa a política de `internal/services/client.go`.

---

## 5. `api`

### 5.1. Endpoints

```
GET    /v1.0/services                       Listar catálogo (paginado, filtros code/description)
POST   /v1.0/services                       Criar serviço
GET    /v1.0/services/{sk}                  Detalhe
PUT    /v1.0/services/{sk}                  Atualizar
DELETE /v1.0/services/{sk}                  Remover

GET    /v1.0/nfse-config                    Config NFS-e da org
PUT    /v1.0/nfse-config                    Upsert

POST   /v1.0/nfses                          Emitir → 202 + operation_id + canal WS
GET    /v1.0/nfses                          Listar (paginado)
GET    /v1.0/nfses/{id}                     Detalhe (id = id_dps ou chave de acesso)
GET    /v1.0/nfses/{id}/xml                 XML da NFS-e
GET    /v1.0/nfses/{id}/dps-xml             XML da DPS/RPS enviada
GET    /v1.0/nfses/{id}/danfse              PDF (proxy ADN; ABRASF → 501)
POST   /v1.0/nfses/{id}/cancel              Evento 101101
POST   /v1.0/nfses/{id}/substitute          Nova DPS com grupo subst → evento 105102
POST   /v1.0/nfses/{id}/events              Evento genérico {event_type, payload}
GET    /v1.0/nfses/{id}/events              Listar eventos
GET    /v1.0/nfses/{id}/events/{sk}/xml     XML do evento

GET    /v1.0/nfse/municipal-parameters/{mun}/aliquota
GET    /v1.0/nfse/municipal-parameters/{mun}/convenio
GET    /v1.0/nfse/municipal-parameters/{mun}/beneficio
GET    /v1.0/nfse/municipal-parameters/{mun}/regimes-especiais
GET    /v1.0/nfse/municipal-parameters/{mun}/retencoes

GET    /v1.0/nfse/distributions             Documentos recebidos via ADN (NSU)
```

`/v1.0/services` usa `crud_handlers.go` genérico, como produtos e veículos.

### 5.2. Emissão

`NfseService.Emit` espelha `NfeService.Emit`:

1. Carrega org, config NFS-e e certificado.
2. Resolve serviços do catálogo (`resolveServices`, análogo a `resolveProducts`) — o body pode
   referenciar `service_sk` e sobrescrever valor/alíquota por item.
3. Resolve tomador/intermediário do cadastro de pessoas, ou aceita inline.
4. Calcula `id_dps` determinístico.
5. Um `transact_write`: reserva `{env}_current_number` em `organization_nfse_configs` + cria item em `nfses` +
   cria comando imutável em `worker_outbox`.
6. Responde 202 com `operation_id` e canal WebSocket.

Erros: sempre RFC 7807 via `problem.*`.

### 5.3. Eventos

Handler genérico. `event_type` determina o schema do payload específico (`TE101101`, `TE202201`, …),
validado antes de enfileirar. Eventos privativos do fisco (`105104`, `105105`, `205204`, `305101–103`)
são rejeitados com 400 — só chegam pela distribuição.

### 5.4. Parâmetros municipais

Proxy fino sobre o ADN, com cache em Valkey (TTL 6h). São dados públicos por município/competência,
não por tenant — cachear evita rate-limit e latência no formulário de emissão.

---

## 6. `worker`

- `WorkerMessage.DocType = "nfse"`.
- Mesmo `claimProcessing` com lease de 6 minutos, mesma política de retry/terminal.
- `Process` roteia para `dfe.Call` in-process quando `docType == nfse` (nunca `invokePyDfe`).
- Persistência: `nfses` (status, `access_key`, `verification_code`, protocolo) e `nfse_events`.
- S3: `{org_pk}/nfse/{id_dps}.xml`, `{org_pk}/nfse/{id_dps}/dps.xml`,
  `{org_pk}/nfse/{id_dps}/events/{event_sk}.xml`.
- Publica resultado terminal no SNS de resultados → API → WebSocket.

Novo cursor de distribuição no `distribution-dispatcher` para NSU do ADN.

---

## 7. `ui`

| Rota | Conteúdo |
|---|---|
| `/services` | CRUD do catálogo, espelhando `/products` (lista, busca por código/descrição, form) |
| `/nfse` | Listagem com filtros (competência, status, tomador), detalhe, timeline de eventos |
| `/nfse/new` | Wizard: Prestador → Tomador/Intermediário → Serviço → Valores → IBS/CBS → Revisão |
| `/nfse/distributions` | Notas recebidas via ADN, com ações de manifestação (confirmar/rejeitar) |
| `/fiscal-config` | Aba NFS-e (provider, ambiente, município, série, regime tributário, ABRASF) |

Princípio de simplicidade: o wizard preenche a maior parte a partir do **serviço selecionado** e da
**config da org**. Grupos avançados (comércio exterior, obra, evento, IBS/CBS detalhado) ficam
colapsados e só abrem quando o serviço ou a config os exigir.

`npx eslint src --ext .ts,.tsx` com zero erros e zero warnings antes de qualquer commit.

---

## 8. `cdk`

Quatro tabelas novas — `organization_services`, `organization_nfse_configs`, `nfses`, `nfse_events` —
com os mesmos padrões: prefixo por ambiente, on-demand, PITR em produção. Nenhuma delas leva stream:
o stream do outbox já existe e fica em `worker_outbox` (`cdk/lib/dynamodb-stack.ts:542`), que é a
tabela de comandos compartilhada por todos os tipos de documento — NFS-e entra nela sem alteração de
infraestrutura. IAM do worker e da API estendidos para as tabelas novas e para o prefixo S3 `nfse/`.

---

## 9. Testes

| Camada | Cobertura |
|---|---|
| `go-dfe` unit | Builders XML por grupo do DPS; validação contra XSD 1.01; assinatura; gzip/base64; mapeamento neutro→ABRASF, incluindo os erros de campo não suportado |
| `go-dfe` golden | XMLs de referência versionados por leiaute (DPS, pedRegEvento, RPS 2.04) |
| `api` unit | `NfseService.Emit`, resolução de serviços, cálculo de `id_dps`, validação de eventos |
| `api` integração | `transact_write` de reserva + criação + outbox; RBAC multi-tenant |
| `worker` integração | Ciclo completo com go-dfe mockado: claim, chamada, persistência, S3, SNS |
| Fiscal ponta a ponta | Emissão e cancelamento reais contra produção restrita do Sefin Nacional |
| `ui` | Schemas Zod do wizard, componentes de item de serviço |

---

## 10. Fases

| Fase | Entrega | Depende de |
|---|---|---|
| **F1** | Tabelas + cadastros: `organization_services`, grupo `nfse` em `PersonObjectBody` (cobre `organizations` e `organization_persons`), `organization_nfse_configs`, CDK, tabelas de referência (Anexos B/C) | — |
| **F2** | `go-dfe/nfse` — provider `nacional` completo: DPS + IBS/CBS, eventos, consultas, DPS por id, DANFSE, parâmetros municipais, distribuição ADN | — |
| **F3** | `api` (emissão, eventos, consultas, proxies) + `worker` (pipeline NFS-e) | F1, F2 |
| **F4** | `ui` — catálogo, wizard de emissão, eventos, distribuição, config | F3 |
| **F5** | `go-dfe/nfse/abrasf204` — motor genérico das 10 operações do WSDL + wiring na API/worker | F2, F3 |
| **F6** | Hardening — homologação em produção restrita, testes ponta a ponta, documentação | todas |

---

## 11. Riscos e limites conscientes

- **IBS/CBS em transição.** O Anexo C é de janeiro/2026 e as regras ainda mudam. O leiaute é modelado
  por completo, mas as regras de negócio ficam com o Sefin — não são replicadas localmente. Erros do
  fisco são propagados com código e descrição.
- **Desvios municipais no ABRASF.** O motor implementa 2.04 puro. Divergências municipais são a norma,
  não a exceção; cada cidade entra depois, com homologação própria. A v1 não embute nenhuma tabela de
  endpoints.
- **DANFSE ABRASF.** Sem PDF padrão no leiaute; a v1 responde 501. Gerador próprio é projeto à parte.
- **Chave de acesso tardia.** A SK é o `idDPS`, não a chave. Qualquer consulta por chave passa pela GSI
  `access-key-index` — custo assumido em troca de imutabilidade da SK.
- **`opSimpNac` e alíquota.** O regime do Simples Nacional e a alíquota efetiva são declarados pelo
  contribuinte na config e no serviço. Não há integração com o cadastro do Simples — o Sefin valida.

---

## 12. Documentação a atualizar

`OVERVIEW.md` (tipos de documento, tabelas, fluxo), `DOCS.md` (endpoints, schemas, módulos),
`DynamoDB-Tables.md` (4 tabelas novas + grupo `nfse` em `organizations` e `organization_persons`), `CONDUCT.md` (decisão da
SK por `idDPS`; regra de falha explícita no adapter ABRASF), `INTEGRATION.md` (contratos do wizard).
