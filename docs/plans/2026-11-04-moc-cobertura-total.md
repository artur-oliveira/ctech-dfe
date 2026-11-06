# Plano consolidado: cobertura total MOC (NF-e, NFC-e, CT-e, MDF-e) e módulo CIOT

Base: `docs/specs/2026-11-04-moc-gap-nfe-nfce.md`, `...-cte.md`, `...-mdfe.md` e
`docs/specs/2026-11-04-ciot-antt-pef-integracao.md`.
Objetivo: 100% das tags, 100% da contingência, DANFE/DACTE/DAMDFE em todos os formatos e validação interna de cada tag e regra
antes de enviar à SEFAZ. Cada fase abaixo traz **infra, backend, frontend, testes e documentação**; fase sem uma dessas
colunas preenchida não está pronta para começar.

## 1. Fontes de verdade (decisão 2026-11-04)

- **Schemas (XSD):** portais da SVRS: `https://dfe-portal.svrs.rs.gov.br/{NFE,MDFE,CTE}/Documentos`. Download em
  `https://dfe-portal.svrs.rs.gov.br/{SIS}/DownloadArquivoEstatico/?sistema={SIS}&tipoArquivo=2&nomeArquivo={zip}`
  (`tipoArquivo=1` traz os PDFs do MOC).
- **Estado dos schemas no repo** (só em `cdk/cdk.out`, ignorado pelo git; nenhum `.xsd` é versionado):
  CT-e **desatualizado** (repo na NT 2026.001; portal em NT 2026.005 SN, 2026.004 e 2026.002); NF-e com o repo (PL_010e)
  **à frente** do portal (PL_010c), então a referência mais nova é o MOC (NT 2026.008); MDF-e igual ao portal (1.05).
- **MOC online:** o HTTPS recusa conexão (inclusive no WebFetch); `curl` em HTTP funciona.
- **Regra:** nenhuma tabela é gerada de XSD que não tenha sido conferido contra o MOC da mesma data.

## 2. Decisões já tomadas

- Pacote de validação mora em `go-dfe/validate` (específico de XML). Migra para `ctech-go-common` se outro serviço precisar.
- CT-e 4.00 segue o modelo do MDF-e (envio com gzip) e **não** envia o SOAP Header `cteCabecMsg` (informação do dono; confirmar em homologação).
- CIOT: gerador por endpoint é o canal oficial; chave no SSM; A1 do cliente basta; natureza da carga são os 4 primeiros dígitos
  do NCM; banco do pagamento é o código de compensação de 3 dígitos. Detalhes na especificação do CIOT.
- Webhooks: funcionalidade futura da plataforma inteira, fora deste plano.

## 3. Já corrigido (2026-11-04)

- `xsdorder`: `ISSQNcalc` virou `ISSQN`; entraram `ide/cIndOp`, `det/{impostoDevol,infAdProd,vItem,DFeReferenciado}`,
  `retirada|entrega/{CEP,IE}`; CT-e `infCTeNorm` e `infCTeSupl`. Conferido contra XSD e Leiaute do MOC.
- `vNF` e `ICMSTot/vProd` pela regra W16-10 e W07-10 (serviço ISSQN fora do vProd).
- MDF-e: `SegCodBarra` (36 dígitos, só FS-DA) deixou de ser emitido com a chave de 44.
- `GenerateAccessKey`: CPF com zeros à esquerda.
- Worker, eventos rejeitados: 631 (duplicidade de evento, MDF-e), 218 (já cancelado) e 609 (já encerrado, MDF-e) confirmam o estado pedido; cStat 134 é sucesso.
- Worker, autorização duplicada (cStat 204): consulta a situação e adota o protocolo vigente (`reconcile.go`); o 539 segue rejeição.

## 4. Regras de infra que valem para tudo (`ctech-dev-standards`, `cdk/CLAUDE.md`)

- **CDK está congelado:** nada de construtos compartilhados novos; stacks existentes seguem e podem ganhar tabela, fila, função ou
  permissão no mesmo estilo. Infra **independente** nova vai em Terraform. **Decisão pendente do dono:** o módulo CIOT e a fila de
  contingência são "extensão do dfe" (CDK existente) ou "infra nova" (Terraform)? O plano assume CDK existente, pelo menor risco.
- Sem NAT, ALB, alarme do CloudWatch, métrica customizada ou dashboard. Alertas vão pelo `alerts` do go-common para o tópico
  compartilhado (`/ctech/{env}/alerts/topic-arn`). Fila sem DLQ é proibida.
- DynamoDB sob demanda com teto de RRU/WRU (nunca abaixo de 100), PITR só em prod, RETAIN fora de dev, TTL em todo item temporário,
  GSI `KEYS_ONLY` ou `INCLUDE`; tabela já criada nunca é recriada. **Toda tabela ou recurso novo leva nota de custo** (modelo de
  cobrança, volume esperado, ciclo de vida) e atualiza `DynamoDB-Tables.md` e `DEPLOYMENT.md`.
- Segredos: SSM `SecureString` criado à mão, nunca pelo CDK (`DEPLOYMENT.md`, "Out-of-band parameters").
- Já existem workers de emissão e evento de NF-e, CT-e, MDF-e e NFS-e, e o de inutilização de NF-e (`cdk/lib/worker-definitions.ts`):
  CT-e **não precisa de função nova** para emitir; falta o resto da cadeia (API, builder, UI).

## 5. Trilha de prazos (separada das fases, feita em paralelo)

Os prazos não esperam F0 a F2. Cada trilha foi cortada **depois de ler as NTs** (baixadas do portal da SVRS em 2026-11-04):
NT 2023.002 v1.02, NT 2026.005 v1.00, NT 2026.006 v1.01, NT 2026.007 v1.11, NT 2026.008 v1.01 e o cronograma da NT 2025.002 v1.53.
Não li o corpo da NT 2025.002 (4.677 linhas de texto), só o histórico e o cronograma.

### O que as NTs dizem (e corrige as auditorias)

| NT | O que muda | Produção | Obrigatório para emitente atual? |
|---|---|---|---|
| 2023.002 v1.02 | **Inutilização some só em AC, BA e SP** (rejeição 998 "Serviço Desativado" ou HTTP 404); nesses estados a SEFAZ também deixa de checar numeração inutilizada (regra 3B08-100). Fim da autorização assíncrona (`indSinc=0` rejeitado, 129) e NFC-e com lote de no máximo 1 (126); fim da denegação na NFC-e (irregularidade vira rejeição 781) | inutilização **11/11/2026**; assíncrona 05/04/2027 | só para emitentes de AC, BA e SP (inutilização). `indSinc=1` já é fixo no builder |
| 2026.005 v1.00 (Simples Nacional) | `tpRegimeIBSCBS` (C21a, emitente), `cAtivSN` (I17d, item), grupo `gTribSN` do item (`gIBSSN`: `pIBSSN`, `vIBSSN`; `gCBSSN`: `pCBSSN`, `vCBSSN`), total `IBSCBSTot/gTribSN` (`vIBSSN`, `vCBSSN`), regras 1275, 1276, 1277 e 1284; evento 412140 é do Fisco, implementação futura | **16/11/2026** | campos opcionais (0-1); regras só valem quando o grupo é informado |
| 2026.006 v1.01 (split payment) | grupo `gPgtoVinc` e evento 110300 | 16/11/2026 | **não**: a própria NT diz "sem exigência de preenchimento em 2026"; ativação em 2027 |
| 2026.007 v1.11 (emitente exclusivo de IBS/CBS) | emitente **sem IE** pode emitir NF-e mod. 55; autorização **só na SVRS**; CFOP restrito pela tabela IT 2023.002; ICMS proibido e IBS/CBS obrigatório nesse caso; checagens LCC-RFB e CCC no servidor | 16/11/2026 | **não**: habilita um tipo novo de emitente |
| 2026.008 v1.01 (valor líquido) | `vUnComLiq`, `vProdLiq` (item), `vProdLiqTot` (total), grupo `ICMSPrevistoPagtoAntecip` (`vICMSPrevisto`, só para `tpNFDebito=06` e `tpOperGov=4`); **o campo `vItem` saiu da v1.01**; `vProdLiq` obrigatório é "implementação futura" | 16/11/2026 (RV NB01-30 só em 01/03/2027) | campos opcionais; regras 1278 a 1283 só quando informados |
| 2025.002 v1.53 (cronograma) | "Início da obrigatoriedade da informação dos novos tributos (RV UB12-10): **implementação futura**" | futura | IBS/CBS ainda não é obrigatório em produção |

Conclusões: (1) **T1 é regional**, não nacional. (2) **Nada de T2 é de preenchimento obrigatório em 16/11** para quem hoje emite;
é suporte a campos novos e a não regredir. O risco real é o builder atual **enviar o grupo `gIBSCBS` para emitente Simples** onde a
NT 2026.005 (seção 2.3) manda não informar e informar `gTribSN`: isso precisa ser auditado antes (item 1 de T2). (3) A auditoria
de NF-e e NFC-e tratava `vItem` e `gPgtoVinc` como pendências de 16/11; não são.

### Trilhas

| Prazo | Trilha | Infra | Backend | Frontend | Testes | Docs | Estimativa |
|---|---|---|---|---|---|---|---|
| **11/11/2026** | **T1 (feita em 2026-11-04, falta lote de 1 e 301/302 da NFC-e):** inutilização desativada em **AC, BA e SP** | nenhuma | tabela `UF -> data de fim` em constante nomeada; para emitente dessas UFs a partir da data, a rota recusa com RFC 7807 antes de chamar a SEFAZ; fora delas, nada muda; conferir que a NFC-e envia lote de 1 e que a resposta 301/302 virou rejeição 781 | esconder o botão e a aba de inutilização para essas UFs, com texto explicando que a SEFAZ não oferece mais o serviço | UF dentro e fora do conjunto, antes e depois da data; resposta 998 e 404 | `DOCS.md`, guia de inutilização (seção e captura) | 1 a 2 dias |
| **16/11/2026** | **T2a (feita em 2026-11-04: o builder enviava `IBSCBS` e `IBSCBSTot` para o MEI, corrigido; CRT 1 e 2 ainda por auditar contra a tabela da NT):** auditar o IBS/CBS emitido para emitente Simples (CRT 1, 2 e 4) contra a seção 2.3 da NT 2026.005 | nenhuma | relatório do que o builder envia hoje por CRT e `tpRegimeIBSCBS`; corrigir o que a tabela da NT proíbe | nenhum | caso por linha da tabela da NT | `DOCS.md` | 1 dia |
| **16/11/2026** | **T2b (feita em 2026-11-05):** NT 2026.005 completa | nenhuma | `tpRegimeIBSCBS`, `cAtivSN`, `gTribSN` do item e do total, regras 1275 a 1284 como validação local, `xsdorder` a partir do XSD | `tpRegimeIBSCBS` na organização; `cAtivSN` por serviço e produto (seletor com os 5 valores); guia | golden XML por linha da tabela da NT | `DOCS.md`, `CONDUCT.md`, guia, capturas | 4 a 5 dias |
| **16/11/2026** | **T2c:** NT 2026.008 | nenhuma | `vUnComLiq`, `vProdLiq`, `vProdLiqTot` calculados (regras 1279 e 1283, tolerância de 0,01) e `vICMSPrevisto` só no cenário da NT | sem campo novo: calculado | golden e tolerância | `DOCS.md` | 2 dias |
| fora de 2026 | NT 2026.006 (split payment) e NT 2026.007 (emitente sem IE, SVRS) | a NT 2026.007 pede roteamento para a SVRS (endpoint por tipo de emitente) | NT 2026.006 em F9 e F3; NT 2026.007 como funcionalidade de F3 e F7 | | | | reavaliar em 2027 |
| **23/11/2026** | **T3 (feita em 2026-11-04, falta `tpEmit` 1 e 3):** CIOT no MDF-e (regra F113, NT 2026.001) sem declarar o CIOT | nenhuma | `infCIOT` múltiplo com CPF ou CNPJ do responsável, validação de 12 dígitos, `tpEmit` 1 e 3 só no que a regra exige; mensagem de erro clara | campo de CIOT por documento, com responsável | casos de borda do F113; rejeições 684, 303 e 304 | `DOCS.md`, guia do MDF-e | 4 a 5 dias |

Total das trilhas: de 13 a 17 dias de uma pessoa. Cabe antes de 16/11 e de 23/11 com folga se T1 e T2a forem os primeiros.

## 6. Fases

Estimativas em semanas de uma pessoa, ordem de grandeza derivada das auditorias (NF-e e NFC-e 25, MDF-e 11 a 13, CT-e 6 a 9, CIOT 8);
não são compromisso. **Total aproximado: 50 semanas de uma pessoa**; com 2 pessoas e as fases paralelas indicadas, cerca de 6 meses.

| Fase | Entrega | Infra | Backend | Frontend | Testes | Docs | Sem. |
|---|---|---|---|---|---|---|---|
| F0 | Versionar os XSDs | nenhuma; tamanho dos `.xsd` no repo | `go-dfe/schemas/<doc>/<NT>/` (CT-e NT 2026.005 SN, MDF-e 1.05, NF-e PL_010e mais deltas do MOC) e script de download do portal | nenhum | teste que falha se um XSD sumir ou mudar de hash | `CONDUCT.md` (fonte dos schemas, já iniciado) | 0,5 |
| F1 | Gerador `FieldSpec` (path, ocorrência, tipo, tamanho, inteiros, decimais, padrão, enum, choice) e do `xsdorder` | etapa de CI que roda o gerador e falha com diferença | gerador em `go-dfe`; paridade com as chaves dos builders | nenhum | paridade tabela x builders x `xsdorder` | `go-dfe/CLAUDE.md`, `DOCS.md` | 1,5 |
| F2 | `go-dfe/validate`: motor genérico, saída RFC 7807 por caminho, decimais, tamanho e formato em todas as tags, modos shadow, warn e block | nenhuma; o shadow só registra log estruturado, sem métrica nem alarme | motor, integração em NF-e, NFC-e, CT-e e MDF-e antes do envio ao egress; `d()` passa a propagar erro | exibir o erro por tag no formulário (mapeado por `code`), estado de erro e carregando | testes de tabela por tipo; `-race`; corpus de documentos reais anonimizados | `DOCS.md`, guia ("por que a nota foi recusada antes do envio") | 2,5 |
| F3 | NF-e: restante das tags ausentes (~96 nomes), destinatário no exterior, `FinNFe` 5 e 6, `retirada`/`entrega` completos, NFC-e (consumidor CNPJ, `idEstrangeiro`, entrega a domicílio) | nenhuma | DTO, builders, limites do cadastro alinhados ao leiaute com migração dos dados existentes | campos e seletores nos formulários; cadastro de pessoa com país e exterior | golden XML por grupo | `DOCS.md`, guia, capturas | 3 |
| F4 | MDF-e completo: `tpEmit` 1 e 3, CIOT múltiplo, `autXML`, carregamento posterior, chaves com DV e CNPJ alfanumérico, consulta de situação e não encerrados | nenhuma | builder, regras F-série, fluxos de consulta no `api` e no worker; chamada pelo egress existente | telas de consulta e de não encerrados; formulário com tipo de emitente | por regra F; integração com egress simulado | `DOCS.md`, guia, capturas | 4 |
| F5 | Catálogo de regras por cStat com teste de completude (P0 offline primeiro; NF-e 912, CT-e 619, MDF-e) | nenhuma | `[]Rule{ID, cStat, When, Check, Needs}` por documento; mensagens do MOC | mensagem da regra no erro, com o texto do MOC | um teste por regra P0; completude contra a lista do MOC | `DOCS.md` (tabela de cobertura de regras) | 6 |
| F6 | CT-e: emissão rodoviária completa (serviço, builder, rotas, eventos) e depois CT-e OS, GTV-e, Simplificado e demais modais | **sem função nova**: os workers de CT-e já existem; só ajustes de timeout e permissão se necessário | serviço, DTO, builder e `xsdorder` gerado do pacote 2026.005; rotas `/ctes`; eventos; consulta; distribuição corrigida (`consChCTe`); `ValidAccessKey` para modelos 57, 64 e 67 | tela de emissão de CT-e (hoje "em breve"), cadastros, lista, detalhe, eventos; `nav.tsx`, fixtures no mock | golden XML por modal; contrato com egress simulado | `DOCS.md`, `DynamoDB-Tables.md` se houver tabela, guia, capturas | 10 |
| F7 | Contingência: NF-e e NFC-e (SVC-AN e SVC-RS, EPEC, offline), MDF-e `tpEmis=2`, CT-e `tpEmis` 2, 7 e 8 | **função agendada** de retransmissão (padrão do `distribution-dispatcher`), fila padrão com DLQ para pendentes; nota de custo; TTL nos itens temporários | máquina de estados por documento, prazos (168 h), endpoints SVC por UF (a confirmar no Portal Nacional), QR Code offline com `DigestValue` (muda a ordem: assinar antes de montar o QR), reconciliação de timeout | modo de contingência na emissão, painel de pendentes, aviso de prazo, impressão em contingência | cenários por `tpEmis`; reenvio idempotente; relógio simulado | `DOCS.md`, `OVERVIEW.md` (fluxo), guia, capturas | 8 |
| F8 | DANFE (7 `tpImp`), DAMDFE (8 modelos), DACTE (A4, A5, ofício, OS, contingência) | cache dos PDFs no bucket de documentos existente (prefixo novo, TTL); sem serviço novo | renderizadores no serviço de folio que já existe; seleção do formato por parâmetro | seletor de formato na visualização e no envio; prévia | comparação visual de referência por formato | `DOCS.md`, guia, capturas | 5 |
| F9 | Eventos restantes (NF-e reforma 112110 a 112150, 211110 a 211150, 212110 e 212120, 110140, 110130/110131, 110150, 110300; CT-e; MDF-e) | workers de evento já existem; só `sefazServices` novos nos filtros SNS | DTO, builder e validação por evento; consulta de eventos | telas e modais de evento por documento | por evento; idempotência do 573 e do 631 | `DOCS.md`, guia | 3 |

Dependências: F1 e F2 dependem de F0; F3 a F6 dependem de F2 (F3 e F6 podem andar juntas com duas pessoas); F7 depende de F5; F8
depende de F6; F9 depende de F6.

## 7. Módulo CIOT (especificação completa em `docs/specs/2026-11-04-ciot-antt-pef-integracao.md`)

| Fase | Entrega | Infra | Backend | Frontend | Testes | Docs | Sem. |
|---|---|---|---|---|---|---|---|
| C0 | Passada manual em homologação (gerador, consultas, uma declaração, um cancelamento) | certificado A1 de teste da organização, ambiente de homologação | nenhum | nenhum | roteiro manual registrado | resultado em `DOCS.md` | 0,5, depende de acesso |
| C1 | `go-dfe/ciot`: tipos, cliente, gerador (principal e contingência), normalização, validador, cálculo do piso | leitura dos parâmetros SSM no worker e no egress (IAM `ssm:GetParameter` e `kms:Decrypt` com escopo de caminho) | cliente REST/JSON, catálogo de regras, tabela do piso versionada | nenhum | servidor HTTP falso dos 8 serviços e do gerador; uma rejeição por grupo; timeout e 5xx; **nunca a ANTT real** | `go-dfe/CLAUDE.md` | 2 |
| C2 | `go-dfe-egress`: `doc_type` `ciot` e `Implements` | nenhuma | roteamento dos serviços, mTLS reaproveitado | nenhum | teste de contrato | `DOCS.md` | 0,5 |
| C3 | `api`: entidade, repositório, endpoints, outbox, auditoria imutável | **tabela `ciots`** (PK da organização, SK do CIOT; GSI por código e por MDF-e) e tabela de auditoria, sob demanda, PITR em prod, nota de custo; atualizar `DynamoDB-Tables.md` | `POST /v1.0/ciots` com `Idempotency-Key`, cancelar, retificar, encerrar, lista com cursor, consultas de transportador e frota, `GET /v1.0/freight-floor` | nenhum | unitários e de contrato; teste entre organizações (404) | `DOCS.md`, OpenAPI | 1,5 |
| C4 | `worker`: consumidor de CIOT, reconciliação do 219, alertas | **worker novo** `ciot-operation` com `sefazServices` próprios e DLQ (extensão de `worker-definitions.ts`); alerta de certificado vencendo e de prazo de cancelamento pelo `alerts`; cache de transportador com TTL do DynamoDB | consumidor, reconciliação com `ConsultarCIOTGerado`, rejeição de negócio nunca reenviada | nenhum | duplicata de mensagem; 219; 222 e 251 | `worker/CLAUDE.md`, `DOCS.md` | 1 |
| C5 | MDF-e: `infCIOT` múltiplo, responsável, cancelar e encerrar em cascata | nenhuma | vínculo CIOT e MDF-e; sobrepõe o trecho da T3 | seletor de CIOT (`declared`) por responsável; ações em cascata | integração MDF-e e CIOT | `DOCS.md`, guia | 1 |
| C6 | UI do CIOT | nenhuma | nenhum | lista, formulário (seletores de tipo, pagamento, carga por NCM de 4 dígitos, contratado, veículos, município, banco), valor mínimo do piso, `AvisoTransportador` em destaque, CPF, CNPJ e placa validados; `nav.tsx`, fixture no mock | testes de componente; zero lint | guia, capturas, `CONDUCT.md` | 1,5 |
| C7 | Coeficientes do piso e tabela de bancos versionados | nenhuma | geração da tabela do Banco Central e atualização do piso com data de vigência | seletor de banco | teste de vigência | `DOCS.md` | 0,5 |

Total de 8 semanas.

## 8. Testes e portões de qualidade (todas as fases)

- Go: `gofmt`, `go vet`, build arm64 e testes com `-race`; UI: `npm run lint` com zero erros e zero avisos, `tsc` e testes.
- `scripts/standards-check.sh` verde. Hoje o CI não roda staticcheck, govulncheck nem os testes do CDK (divergência já documentada); toda
  fase que mexe em infra roda `cdk synth` e os testes `jest` do CDK localmente.
- Cada correção de bug traz teste que reproduz o bug; cada regra do MOC traz teste pelo código de rejeição.
- XML: golden por documento e por NT; paridade entre tabela de campos, `xsdorder` e chaves dos builders.
- Homologação: passada manual por documento antes de produção; **a SEFAZ e a ANTT reais nunca entram no CI**.
- Validador novo entra em shadow (só registra), depois warn, depois block, com a data de cada virada registrada em `CONDUCT.md`.

## 9. Frontend: regras que valem em toda tela nova (`ui/CLAUDE.md`)

- Seletores em vez de texto livre; texto livre só onde é realmente livre, com validação estrita.
- Toda tela nova entra em `nav.tsx` (navegação e busca global), tem seção no guia com captura de slug igual ao da rota e fixture no mock;
  rota de app permanece `noindex`.
- Mobile-first a 375 px, alvos de toque de 44 px, estados de carregando, vazio e erro; sem travessão em texto visível.
- Sem i18n e sem `@aoctech/ui` nos componentes novos (divergência já documentada; não agravar).

## 10. Riscos e decisões abertas

- **T2 (16/11) deixou de ser o maior risco:** as NTs foram lidas e nada é obrigatório em produção; o ponto a verificar é o `gIBSCBS` enviado a emitente Simples (T2a). O corpo da NT 2025.002 v1.53 não foi lido.
- **T3 (23/11) passa a ser o prazo mais exigente** (regra F113, NT 2026.001): ler o texto da regra no MOC do MDF-e antes de implementar.
- **DACTE da GTV-e e do CT-e Simplificado:** o capítulo DACTE do MOC não cobre; decisão do dono.
- **Pacote XSD de NF-e a versionar:** o do repo (PL_010e) está à frente do portal; confirmar a origem.
- **CDK ou Terraform** para tabelas, worker e fila novos (seção 4).
- **Endpoints SVC por UF** (NF-e e NFC-e) e URL de contingência de CT-e: a confirmar no Portal Nacional.
- Nenhum prazo foi comprometido com o cliente; as estimativas são de uma pessoa e não incluem revisão nem homologação externa.

## 11. Bugs conhecidos ainda abertos (cada um com teste de regressão)

| Item | Onde | Observação |
|---|---|---|
| `d()` engole decimal inválido | `api/internal/services/nfes/v.go` | 129 chamadas; entra na F2 |
| `retirada`/`entrega` sem CEP/IE no builder; `cMun` com fallback `0000000` | `builders_party.go` | F3; tabela de ordem já pronta |
| Destinatário no exterior (`idDest=3`, `indIEDest=2`, `UF=EX`) | `builders_doc.go`, `builders_party.go` | F3 |
| `FinNFe` só 1 a 4 | `emit.go` | F3 |
| Limites de cadastro maiores que o leiaute (`xNome`, `xLgr`, `xProd`, `IE`) | `dto.go` | F3, com migração dos dados |
| `tpEmit` fixo em 2 no MDF-e | `mdfes/builder.go` | F4 (rejeição 639 com CT-e) |
| `EX` recusado em UF e encerramento; `nSeqEvento` sem sequenciador; soma de pagamento x `vContrato` | MDF-e | F4 |
| `ValidAccessKey` só mod 55; assinatura de `infCTeOS`/`infGTVe`; distribuição pede `consChCTe` | go-dfe | F6, confirmar com o XSD novo |

## 12. Padrões a respeitar

Sem scan, constantes nomeadas, RFC 7807, chamadas fiscais só via `go-dfe-egress`, sem travessão em texto visível, teste com cada
correção, docs (`DOCS.md`, `CONDUCT.md`) atualizados na mesma alteração.
