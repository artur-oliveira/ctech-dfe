# Plano consolidado: cobertura total MOC (NF-e, NFC-e, CT-e, MDF-e) e módulo CIOT

Base: `docs/specs/2026-11-04-moc-gap-nfe-nfce.md`, `...-cte.md`, `...-mdfe.md`.
Objetivo: 100% das tags, 100% da contingência, DANFE/DACTE/DAMDFE em todos os formatos e validação
interna de cada tag e regra antes de enviar à SEFAZ.

## 1. Fontes de verdade (decisão 2026-11-04)

- **Schemas (XSD):** portais da SVRS. A lista está em
  `https://dfe-portal.svrs.rs.gov.br/{NFE,MDFE,CTE}/Documentos`. O download é
  `https://dfe-portal.svrs.rs.gov.br/{SIS}/DownloadArquivoEstatico/?sistema={SIS}&tipoArquivo=2&nomeArquivo={zip}`
  (`tipoArquivo=1` traz os PDFs do MOC).
- **Estado dos schemas no repo** (só em `cdk/cdk.out`, ignorado pelo git; nenhum `.xsd` é versionado):
  - CT-e: **desatualizado.** Repo na NT 2026.001; portal em NT 2026.005 SN (02/11/2026), 2026.004 e 2026.002.
    O pacote novo traz `gPagAntecipado`, `IBSCBSTot`, `tpRegimeIBSCBS`, `ISUFEmit` etc.
  - NF-e: repo (PL_010e) **à frente** do portal (PL_010c). Para NF-e a referência mais nova é o MOC (NT 2026.008).
  - MDF-e: repo igual ao portal (1.05).
- **MOC online:** o HTTPS recusa conexão (inclusive no WebFetch); `curl` em HTTP funciona.
- **Regra:** nenhuma tabela é gerada de XSD que não tenha sido conferido contra o MOC da mesma data.

## 2. Decisões já tomadas

- Pacote de validação mora em `go-dfe/validate` (específico de XML). Migra para `ctech-go-common` se outro serviço precisar.
- CT-e 4.00 segue o modelo do MDF-e (envio com gzip) e **não** envia o SOAP Header `cteCabecMsg` (informação do dono; confirmar em homologação).
- Abertas: DACTE da GTV-e e do CT-e Simplificado (o capítulo DACTE do MOC não cobre); localização do pacote XSD oficial a versionar.

## 3. Já corrigido (2026-11-04)

- `xsdorder`: `ISSQNcalc` virou `ISSQN`; entraram `ide/cIndOp`, `det/{impostoDevol,infAdProd,vItem,DFeReferenciado}`,
  `retirada|entrega/{CEP,IE}`; CT-e `infCTeNorm` e `infCTeSupl`. Conferido contra XSD e Leiaute do MOC.
- `vNF` e `ICMSTot/vProd` pela regra W16-10 e W07-10 (serviço ISSQN fora do vProd).
- MDF-e: `SegCodBarra` (36 dígitos, só FS-DA) deixou de ser emitido com a chave de 44.
- `GenerateAccessKey`: CPF com zeros à esquerda.
- Worker, eventos rejeitados: 631 (duplicidade de evento, MDF-e), 218 (já cancelado) e 609 (já encerrado, MDF-e) confirmam o estado pedido em vez de reverter para autorizado; cStat 134 (evento aceito com alerta) é sucesso. Códigos conferidos no MOC do MDF-e.
- Worker, autorização duplicada (cStat 204, mesma chave): consulta a situação (`NfeConsultaProtocolo`, `MDFeConsulta`, `CTeConsulta`); se consta autorizado com o mesmo digest do documento assinado, adota o protocolo vigente e remonta o processado (`reconcile.go`). O 539 (chave diferente) segue rejeição.

## 4. Bugs conhecidos ainda abertos (cada um com teste de regressão)

| Item | Onde | Observação |
|---|---|---|
| `d()` engole decimal inválido | `api/internal/services/nfes/v.go` | 129 chamadas; migrar para erro propagado junto com o validador |
| `retirada`/`entrega` sem CEP/IE no builder; `cMun` com fallback `0000000` | `builders_party.go` | tabela de ordem já pronta |
| Destinatário no exterior (`idDest=3`, `indIEDest=2`, `UF=EX`) | `builders_doc.go`, `builders_party.go` | |
| `FinNFe` só 1 a 4 | `emit.go` | notas de crédito e débito (5, 6) |
| Limites de cadastro maiores que o leiaute (`xNome`, `xLgr`, `xProd`, `IE`) | `dto.go` | só com migração dos dados existentes |
| `tpEmit` fixo em 2 no MDF-e | `mdfes/builder.go` | rejeição 639 com CT-e |

| `EX` recusado em UF e encerramento; `nSeqEvento` sem sequenciador; soma de pagamento x `vContrato` | MDF-e | |
| `ValidAccessKey` só mod 55; assinatura de `infCTeOS`/`infGTVe`; distribuição pede `consChCTe` | go-dfe | confirmar com XSD novo |

## 5. Fases

Ordem pelos prazos: NF-e 11/11/2026 (fim da inutilização, confirmar a NT) e 16/11/2026 (NTs 2026.005, 006, 007);
MDF-e 23/11/2026 (CIOT obrigatório, NT 2026.001).

| Fase | Entrega | Depende de |
|---|---|---|
| F0 | Versionar XSDs (`go-dfe/schemas/<doc>/<NT>/`): CT-e NT 2026.005 SN, MDF-e 1.05, NF-e PL_010e mais deltas do MOC; script de download do portal | decisão sobre o NF-e acima |
| F1 | Gerador `FieldSpec` (path, ocorrência, tipo, tamanho, inteiros, decimais, padrão, enum, choice) a partir do XSD; gera também o `xsdorder`; teste de paridade com as chaves dos builders | F0 |
| F2 | `go-dfe/validate`: motor genérico + saída RFC 7807 por caminho; validação de decimais, tamanho e formato em todas as tags; modos shadow, warn e block | F1 |
| F3 | NF-e: tags das NTs 2026.005/006/007 (`gTribSN`, `vItem`, `vProdLiq`, `ICMSPrevistoPagtoAntecip`, `DFeReferenciado`, `gPgtoVinc`, monofasia); total (`vICMSMono*`); revisão da inutilização | F1 |
| F4 | MDF-e: `tpEmit` 1 e 3, CIOT, `autXML`, carregamento posterior, chaves com DV e CNPJ alfanumérico, consulta e não encerrados, worker | F2 |
| F5 | Regras do MOC por cStat, catálogo com teste de completude: P0 offline primeiro | F2 |
| F6 | CT-e: emissão rodoviária completa (serviço, builder, rotas, eventos), depois OS, GTV-e, Simplificado e demais modais | F0, F2 |
| F7 | Contingência: NF-e/NFC-e (SVC, EPEC, offline, fila), MDF-e `tpEmis=2`, CT-e `tpEmis` 2, 7, 8 | F5 |
| F8 | Documentos auxiliares: DANFE (7 `tpImp`), DAMDFE (8 modelos), DACTE | F6 |
| F9 | Eventos restantes (NF-e reforma, CT-e, MDF-e) | F6 |

## 6. Módulo CIOT (ANTT, "CIOT para Todos")

Referência: `https://www.gov.br/antt/pt-br/assuntos/cargas/ciot-para-todos-1/dcs_pef_v1-1.pdf` (não lido ainda).

- Escopo proposto: serviço `ciot` no `ctech-dfe` com cadastro do contratante e do transportador, abertura,
  retificação, encerramento e cancelamento do CIOT, e o vínculo com o MDF-e (`infCIOT`) e o pagamento do frete.
- Pré-requisitos antes de estimar: ler o PDF (autenticação, ambiente de homologação, certificado, formato JSON ou XML,
  eventos e códigos de erro); decidir se a chamada passa pelo `go-dfe-egress` (regra do projeto para chamadas fiscais).
- Cada integração externa precisa de timeout, retry idempotente, breaker e decisão de fail open ou closed (`ctech-dev-standards`).
- Prazo: a NT 2026.001 do MDF-e exige CIOT a partir de 23/11/2026; o módulo entra na F4.

## 7. Padrões a respeitar

Sem scan, constantes nomeadas, RFC 7807, chamadas fiscais só via `go-dfe-egress`, sem travessão em texto visível,
teste com cada correção, docs (`DOCS.md`, `CONDUCT.md`) atualizados na mesma alteração.
