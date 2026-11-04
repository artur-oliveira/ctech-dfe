# Auditoria MOC x implementação: MDF-e (modelo 58)

**Data:** 2026-11-04  **Tipo:** auditoria somente leitura (nenhum código, teste ou UI foi alterado)
**Objetivo do dono do produto:** estrutura que atenda 100% dos casos do MDF-e: (1) 100% das tags de todos os modais e
tipos de emitente, (2) 100% dos fluxos de contingência, (3) DAMDFE em todos os formatos, (4) validação interna criteriosa
antes da SEFAZ, com as regras e códigos de rejeição do MOC, via estrutura genérica.

---

## 0. Fontes, método e limites de leitura

**MOC online lido (versão 3.00b consolidada, com NT até 2026.001).** O host `moc.sped.fazenda.pr.gov.br` recusa HTTPS
(`ECONNREFUSED` na porta 443, também no WebFetch); o HTTP simples respondeu 200, então todas as páginas foram baixadas
com `curl` e convertidas para texto. Páginas lidas por inteiro: Visão Geral, Leiaute (geral, rodoviário, aéreo,
ferroviário, aquaviário), Regras de Validação (F01 a F123 e PAA01 a PAA06), Contingência, DAMDFE, QR Code e Consulta
Pública, Tabelas e Informações (ER1 a ER61 e domínios D1 a D21), MDFeRecepcaoSinc, MDFeConsulta,
MDFeConsultaNaoEncerrados, MDFeStatusServico, MDFeDistribuicaoDFe, MDFeRecepcaoEvento (Parte Geral, Cancelamento,
Eventos Específicos: 110112, 110114, 110115, 110116, 110117, 110118) e Histórico de Implantação.

**O que não consegui ler ou ficou incompleto (não inventado):**

- Os 8 modelos gráficos do DAMDFE (4 modais x normal/contingência) são imagens; li só o texto. A conferência pixel a
  pixel dos modelos fica pendente.
- A tabela do Anexo II de símbolos CODE-128C não foi usada (a lib de código de barras já a implementa).
- A página citada no menu como "Eventos Específicos" existe em `EventosEspecificos.html`; o endereço
  `RecepcaoEventosEspecificos.html` devolve 404 (não faz parte do MOC de MDF-e).
- Os domínios D14 e alguns itens da tabela de domínio têm texto tachado/atual misturado; usei o XSD para desambiguar.
- Nas regras, alguns textos de mensagem longos foram truncados na minha visualização (F58 a F62, F87, F88, F113), mas
  código, condição e cStat foram capturados.
- O MOC não publica tabela de municípios IBGE nem de fronteiras entre UF (necessárias para F09, F12, F90).

**Contra-verificação com o XSD.** Os schemas oficiais `PL_MDFe_300b_NT012025_1.05` existem no repo apenas dentro de
`cdk/cdk.out/asset.*/schemas/xsds/` (diretório ignorado pelo git; nenhum `.xsd` é versionado). Usei esses XSDs como
verdade para tipo/tamanho/decimais/ocorrência (tabela gerada por script de achatamento do XSD, 315 caminhos) e
para detectar onde o MOC online e o XSD divergem entre si. Onde divergem, aponto.

**Documentos do repo lidos:** `CLAUDE.md` raiz, `OVERVIEW.md`, `DOCS.md` (seções MDF-e), `CONDUCT.md`,
`api/CLAUDE.md`, `worker/CLAUDE.md`, `go-dfe/CLAUDE.md`, `docs/plans/2026-08-26-cobertura-total-tags-nfe-mdfe.md`,
`docs/plans/2026-08-27-cobertura-total-tags-implementacao.md`, `docs/plans/2026-08-26-contingencia-e-inutilizacao-dfe.md`.
Código: `api/internal/services/mdfes/*`, `api/internal/services/documents/{mdfe.go,templates}`,
`api/internal/validation/*`, `api/internal/api/v1/mdfes.go`, `worker/internal/service/{dfe.go,helpers.go}`,
`go-dfe/internal/{xmlops/xsdorder,endpoints,services,soap,constants}`, `ui/src/components/mdfe`.

---

## 1. Resumo executivo

Cobertura por nome de tag é alta: dos 315 caminhos do XSD (genérico + 4 modais), só ficam sem nenhuma emissão
`autXML`, `infPAA` (e filhos) e `infSolicNFF` (e filho). O problema não é o conjunto de tags, e sim (a) alguns
valores emitidos que violam MOC/XSD, (b) tipos de emitente e formas de emissão que não existem no modelo, e
(c) quase nenhuma pré-validação das regras F do MOC. Maiores gaps, em ordem de impacto:

| # | Gap | Evidência | Efeito |
|---|-----|-----------|--------|
| 1 | `SegCodBarra` é emitido com a **chave de 44 dígitos** em todo NF-e/CT-e/MDF-e transportado; XSD e MOC exigem `[0-9]{36}` e só para FS-DA (F35/F36/F41/F42) | `builder_infdoc.go:66`; teste `builder_infdoc_test.go:17-57` consagra o valor | Falha de schema (cStat 215/580) ou rejeição 603/607 em praticamente todo manifesto com documentos |
| 2 | `SegCodBarra` também é emitido em `infMDFeTransp`, grupo que não o tem no XSD | `builder_infdoc.go:40,52,60-66` | Falha de schema em todo MDF-e aquaviário com transbordo |
| 3 | `tpEmit` fixo em "2" (carga própria), mesmo quando os documentos são CT-e | `builder.go:30,183`; `emit.go:610` aceita `cte` | Todo MDF-e com CT-e é rejeitado 639 (F15); tpEmit 1 e 3 (transportadoras) não existem; F14/F16/F17 e toda a cadeia obrigatória do tpEmit 1/3 (seguro F91, RNTRC F108, CIOT F113, pagamento F52-F55) sem tratamento |
| 4 | Contingência (`tpEmis=2`) inexistente: `tpEmis` fixo normal; sem QR `sign`, sem prazo de 168 h, sem DAMDFE pré-autorização, sem fila | `emit.go:474-477`; plano C6 "não iniciado" | Fluxo (2) do objetivo está em 0% |
| 5 | Eventos: worker trata só cStat 573 (código de NF-e) como duplicidade e não conhece 134 (autorizado com alerta); MDF-e usa 218, 609, 631 | `worker/internal/service/helpers.go:67,99` | Encerramento/cancelamento já registrados na SEFAZ voltam o documento para `authorized` (estado divergente); evento com cStat 134 vira rejeição |
| 6 | Carregamento posterior configurável mas inutilizável: `indCarregaPosterior=1` exige zero documentos (F26/F27, cStat 706), mas `Emit` exige `documents min=1` e o DTO também | `emit.go:361`, `builder.go:229`, `emit.go:24` | Toda emissão com a flag ligada é rejeitada 706; evento 110115 só é útil com a flag |
| 7 | `autXML` ausente; `infCIOT` só um e com o CPF do primeiro condutor (errado); sem `veicReboque/prop` | `builder_antt.go:12-14`, `builder.go:288` | Tags do leiaute sem caminho; CIOT passa a ser exigido por regra em **23/11/2026** (NT 2026.001, produção 23/11/2026) |
| 8 | Chave de acesso de documentos referenciados validada só como `len=44,numeric` (sem DV, modelo, UF, ano, 6 meses); recusa CNPJ alfanumérico, que a NT conjunta 2025.001 colocou em produção em 06/07/2026 | `emit.go:229,611-621` | F30/F37/F45/F30a/F37a/F45a ausentes; chaves alfanuméricas válidas são recusadas na API |
| 9 | Nenhuma consulta de situação, de não encerrados nem de status do serviço no produto (o transporte existe no go-dfe) | `go-dfe/internal/endpoints/table.go:442-449`; nenhuma referência em `api/` e `worker/` | F85/F86/F87/F88 (as rejeições mais comuns do dia a dia) não têm pré-checagem; sem reconciliação de MDF-e pendente de retorno |
| 10 | Sem motor de validação por tabela: regras vivem em tags `validate:"..."` do DTO, com tamanhos que divergem do XSD (ver 3.4) | `modals.go`, `validators.go:33,41` | Erros de tamanho/formato só aparecem na SEFAZ |

Além disso: `GenerateAccessKey` completa CPF com zeros **à direita** (`shared.go:287`; o MOC pede à esquerda), `EX` (exterior)
é recusado em `UFStart/UFEnd` e no encerramento, DAMDFE tem um único leiaute (retrato) e `go-dfe` não valida XSD por decisão
de projeto, então a SEFAZ é o único validador de forma hoje.

---

## A. Matriz de cobertura de tags

Legenda de status: **Coberta** (emitida e coerente com XSD/MOC), **Parcial** (emitida com desvio, limitação ou sem
validação adequada), **Ausente** (sem caminho de emissão). Tipo/tamanho/decimais vêm do XSD 1.05, confrontados com o MOC
(ER = expressão regular do MOC). "Ocorr." é do XSD. Referências `arquivo:linha` são de `api/internal/services/mdfes/`
salvo indicação.

### A.1 ide

| Caminho | Ocorr. | Tipo/tamanho/decimais | Status | Evidência |
|---------|--------|-----------------------|--------|-----------|
| `infMDFe/@versao`, `@Id` | 1-1 | `3.00`; `MDFe`+44 | Coberta | `builder.go:128-129` |
| `ide/cUF`, `cMDF`, `cDV` | 1-1 | N2; N8; N1 | Coberta (derivadas da chave) | `builder.go:123-125` |
| `ide/tpAmb` | 1-1 | enum 1,2 | Coberta | `builder.go:182` |
| `ide/tpEmit` | 1-1 | enum 1,2,3 | **Parcial**: só "2" | `builder.go:30,183` |
| `ide/tpTransp` | 0-1 | enum 1,2,3 | Coberta para o caso do MOC (F18-F20: só com `prop`) | `builder.go:201,355` |
| `ide/mod`, `serie`, `nMDF` | 1-1 | 58; `0|[1-9][0-9]{0,2}`; `[1-9][0-9]{0,8}` | Coberta (sem validar faixa 920-969, F69-F71) | `builder.go:184-186` |
| `ide/modal` | 1-1 | enum 1-4 | Coberta | `builder.go:189` |
| `ide/dhEmi` | 1-1 | ER1 (UTC com fuso) | Coberta | `builder.go:190` |
| `ide/tpEmis` | 1-1 | enum 1,2,3 | **Parcial**: parametrizado mas sempre 1 | `emit.go:476`, `builder.go:24-25` |
| `ide/procEmi` | 1-1 | XSD: enum 0,4 (MOC: só 0) | Parcial: só 0 (4 = PAA, sem fluxo) | `builder.go:19` |
| `ide/verProc` | 1-1 | C1-20 | Coberta (sem truncar em 20) | `builder.go:485` |
| `ide/UFIni`, `UFFim` | 1-1 | D5 inclui `EX` | **Parcial**: `EX` recusado (`UFSet` sem EX) | `emit.go:26-27`, `validators.go:14` |
| `ide/infMunCarrega` (`cMunCarrega` N7, `xMunCarrega` C2-60) | 1-50 | | Parcial: nome com `max=120` no DTO (XSD 60); limite 50 não checado | `builder.go:205-212`, `emit.go:236-241` |
| `ide/infPercurso/UFPer` | 0-25 | D5 | Parcial: sem `max=25`, sem ordem (F90) | `builder.go:214-220` |
| `ide/dhIniViagem` | 0-1 | ER1 | Parcial: string livre, sem validação de formato | `emit.go:42`, `builder.go:221` |
| `ide/indCanalVerde`, `indCarregaPosterior` | 0-1 | enum 1 | Parcial: emitidas, mas F21 a F24 não validadas e carregamento posterior inviável (gap 6) | `builder.go:226-231` |

### A.2 emit

| Caminho | Ocorr. | Tipo/tamanho | Status | Evidência |
|---------|--------|--------------|--------|-----------|
| `emit/CNPJ` \| `CPF` (choice) | 1-1 | XSD `[A-Z0-9]{12}[0-9]{2}`; `[0-9]{11}` | Coberta | `builder.go:441-446` |
| `emit/IE` | 0-1 (MOC: obrigatória, F72) | `[0-9]{2,14}` | Parcial: omitida se a org não tem IE, sem recusar (F72/229) | `builder.go:447` |
| `emit/xNome`, `xFant` | 1-1; 0-1 | C2-60; C1-60 | Parcial: sem corte em 60 | `builder.go:433,437` |
| `enderEmit/xLgr`, `nro`, `xCpl`, `xBairro`, `xMun` | 1-1 (xCpl 0-1) | C2-60 / C1-60 | Parcial: `nro` cai em `S/N`; sem corte | `builder.go:456-465` |
| `enderEmit/cMun` | 1-1 | `[0-9]{7}` | **Parcial**: cai em `0000000` quando ausente (F78/408 certo) | `builder.go:459` |
| `enderEmit/CEP`, `UF`, `fone`, `email` | 0-1 / 1-1 | `[0-9]{8}`; D5; `[0-9]{7,12}`; C6-60 | Coberta | `builder.go:461-472` |
| Exterior (`cMun=9999999`, `xMun=EXTERIOR`, `UF=EX`) | | | Ausente | |

### A.3 infModal (todos os modais)

| Caminho | Ocorr. | Tipo/tamanho/decimais | Status | Evidência |
|---------|--------|-----------------------|--------|-----------|
| `infModal/@versaoModal` | 1-1 | `3\.(0[0-9]|[1-9][0-9])` | Coberta | `builder.go:239` |

**Rodoviário (`rodo`)**

| Caminho | Ocorr. | Tipo/tamanho | Status | Evidência |
|---------|--------|--------------|--------|-----------|
| `infANTT/RNTRC` | 0-1 | `[0-9]{8}` | Parcial: validador aceita 8 a 12 dígitos | `builder_antt.go:9`, `validators.go:33` |
| `infANTT/infCIOT` (`CIOT` N12; `CPF`\|`CNPJ`) | 0-n | `[0-9]{12}` | **Parcial**: um só CIOT, CPF do 1o condutor, sem CNPJ, sem validar 12 dígitos | `builder_antt.go:12-14`, `emit.go:43` |
| `infANTT/valePed/disp` (`CNPJForn`, `CNPJPg`\|`CPFPg`, `nCompra` N1-20, `vValePed` 13,2, `tpValePed` 01\|04) | 1-n | | Coberta (domínio de `tpValePed` do MOC D15 diverge do XSD: ver E) | `builder_antt.go:47-68` |
| `valePed/categCombVeic` | 0-1 | enum 02..14 | Parcial: só 02, 04, 06, 07 (derivada do nº de reboques; MOC lista até 14 por eixos) | `builder_antt.go:31-42` |
| `infANTT/infContratante` (`xNome`, `CPF`\|`CNPJ`\|`idEstrangeiro`, `infContrato`) | 0-n | `NroContrato` C2-20; `vContratoGlobal` 13,2 | Coberta (limite 10 do DTO é estrito demais frente ao XSD 0-n) | `builder_antt.go:73-98` |
| `infANTT/infPag` e filhos (`xNome`, `CPF`\|`CNPJ`\|`idEstrangeiro`, `Comp`, `vContrato`, `indAltoDesemp`, `indPag`, `vAdiant`, `indAntecipaAdiant`, `infPrazo`, `tpAntecip`, `infBanc`) | 0-n | valores 13,2 | Coberta na emissão; validações de coerência ausentes (F58-F63) | `payment_events.go:94-155`, `pagamento.go` |
| `infPag/infBanc/PIX` | 1-1 (choice) | C2-60 | Parcial: DTO aceita 250 | `payment_events.go:52` |
| `infPag/infBanc/codBanco`, `codAgencia` | | C3-5; C1-10 | Parcial: DTO `len=3,numeric` (XSD 3-5) | `payment_events.go:49-50` |
| `veicTracao` (`cInt`, `placa`, `RENAVAM`, `tara`, `capKG`, `capM3`, `tpRod`, `tpCar`, `UF`) | 1-1 | placa `[A-Z]{2,3}[0-9]{4}|[A-Z]{3,4}[0-9]{3}|[A-Z0-9]{7}`; `tara`, `capKG` `0|[1-9][0-9]{0,5}`; `capM3` `0|[1-9][0-9]{0,2}` | Parcial: `tara`/`capKG` validadas só com `decimalv` (aceita decimais); placa só formato nacional | `builder.go:255-276`, `emit.go:245-250`, `validators.go:32,41` |
| `veicTracao/prop` (`CPF`\|`CNPJ`, `RNTRC`, `xNome`, `IE`, `UF`, `tpProp`) | 0-1 | | Coberta | `builder.go:331-349` |
| `veicTracao/condutor` (`xNome`, `CPF`) | 1-10 | | Parcial: sem `max=10`, sem duplicidade (F99) | `builder.go:278-282` |
| `veicReboque` (`cInt`, `placa`, `RENAVAM`, `tara`, `capKG`, `capM3`, `tpCar`, `UF`) | 0-3 | | Coberta | `builder.go:285-307` |
| `veicReboque/prop` (`CPF`\|`CNPJ`, `RNTRC`, `xNome`, `IE`, `UF`, `tpProp`) | 0-1 | | **Ausente** (F104/719 prevê) | `builder.go:288` |
| `rodo/codAgPorto` | 0-1 | C0-16 | Coberta | `builder.go:311` |
| `rodo/lacRodo/nLacre` | 0-n | C1-20 | **Parcial**: DTO `max=60` (XSD 20) | `builder.go:314`, `emit.go:84` |

**Aéreo (`aereo`)** (`modals.go:17-38`)

| Caminho | Ocorr. | Tipo/tamanho | Status |
|---------|--------|--------------|--------|
| `nac` | 1-1 | C1-4 | Coberta |
| `matr` | 1-1 | C1-6 | Coberta |
| `nVoo` | 1-1 | C5-9 | Parcial: sem mínimo 5 |
| `cAerEmb`, `cAerDes` | 1-1 | C3-4 | Parcial: sem mínimo 3 |
| `dVoo` | 1-1 | AAAA-MM-DD | Coberta (`isodate`) |

**Aquaviário (`aquav`)** (`modals.go:54-118`)

| Caminho | Ocorr. | Tipo/tamanho | Status |
|---------|--------|--------------|--------|
| `irin` | 1-1 | C1-10 | Coberta |
| `tpEmb` | 1-1 | N2 | Coberta |
| `cEmbar`, `xEmbar` | 1-1 | C1-10; C1-60 | Coberta |
| `nViag` | 1-1 | `[1-9][0-9]{0,9}` | Parcial: aceita qualquer texto até 10 |
| `cPrtEmb`, `cPrtDest` | 1-1 | C1-5 | **Parcial**: DTO `len=5` (XSD e MOC aceitam 1-5) |
| `prtTrans`, `tpNav` | 0-1 | C1-60; 0\|1 | Coberta |
| `infTermCarreg`, `infTermDescarreg` (`cTerm*` C1-8, `xTerm*` C1-60) | 0-5 | | Coberta |
| `infEmbComb` (`cEmbComb`, `xBalsa`) | 0-30 | | Coberta (comboio) |
| `infUnidCargaVazia`, `infUnidTranspVazia` | 0-n | `[A-Z0-9]+`, 1-20 | Coberta (resolvidas do cadastro) |
| `MMSI` | 0-1 | `[0-9]{9}` | Parcial: `max=9`, sem exigir só dígitos |

**Ferroviário (`ferrov`)** (`modals.go:129-175`)

| Caminho | Ocorr. | Tipo/tamanho/decimais | Status |
|---------|--------|-----------------------|--------|
| `trem/xPref` | 1-1 | C1-10 | Coberta |
| `trem/dhTrem` | 0-1 | ER1 | Parcial: sem validação de formato |
| `trem/xOri`, `xDest` | 1-1 | **C1-3** (sigla da estação) | **Parcial**: DTO `max=100` |
| `trem/qVag` | 1-1 | `[1-9][0-9]{0,2}` | Coberta (derivado) |
| `vag/pesoBC`, `pesoR` | 1-1 | N3,3 (`TDec_0303`) | Parcial: `decimalv` sem limitar 3 inteiros/3 decimais |
| `vag/tpVag`, `serie` | 0-1; 1-1 | C3 (comprimento exato) | Parcial: `max=3` |
| `vag/nVag` | 1-1 | `[1-9][0-9]{0,7}` | **Parcial**: DTO `max=9` |
| `vag/nSeq` | 0-1 | `[1-9][0-9]{0,2}` | **Parcial**: DTO `max=2` |
| `vag/TU` | 1-1 | N3,2-3 | Parcial: `decimalv` |

### A.4 infDoc

| Caminho | Ocorr. | Tipo/tamanho | Status | Evidência |
|---------|--------|--------------|--------|-----------|
| `infMunDescarga` (`cMunDescarga`, `xMunDescarga`) | 1-1000 | N7; C2-60 | Parcial: limite 1000 não checado; `max=120` no DTO | `builder_infdoc.go:25-55` |
| `infCTe/chCTe`, `infNFe/chNFe`, `infMDFeTransp/chMDFe` | 0-20000 cada | XSD `[0-9]{6}[A-Z0-9]{12}[0-9]{26}` | Parcial: limite 20000 não checado; DTO `numeric` recusa alfanumérico | `emit.go:229,611-621` |
| `SegCodBarra` | 0-1 | `[0-9]{36}` | **Parcial com defeito**: emite a chave de 44 (gap 1) | `builder_infdoc.go:66` |
| `indReentrega` | 0-1 | enum 1 | Coberta | `builder_infdoc.go:67-69` |
| `infUnidTransp` (`tpUnidTransp`, `idUnidTransp` `[A-Z0-9]+` 1-20, `lacUnidTransp/nLacre` 1-20, `qtdRat` N3,2-3) | 0-n | | Coberta (rateio derivado dos pesos) | `unidades.go:57-99` |
| `infUnidCarga` (`tpUnidCarga`, `idUnidCarga`, `lacUnidCarga`, `qtdRat`) | 0-n | | Coberta | `unidades.go:97-99` |
| `peri` (`nONU` `[0-9]{4}|ND`, `xNomeAE` 1-150, `xClaRisco` 1-40, `grEmb` 1-6, `qTotProd` 1-20, `qVolTipo` 1-60) | 0-n | | Coberta (derivada do cadastro de produto) | `peri.go:24-63` |
| `infEntregaParcial` (`qtdTotal`, `qtdParcial` N11,4) | 0-1 só em `infCTe` | | Parcial: sem regra F34 (só modal aéreo) | `builder_infdoc.go:81-85` |
| `indPrestacaoParcial`, `infNFePrestParcial/chNFe` | 0-1 / 1-n | enum 1; chave | Coberta; F36a-F36g sem pré-validação | `builder_infdoc.go:86-93` |
| `infMDFeTransp` | 0-20000 | | **Parcial**: emite `SegCodBarra` inválido; F43/F44 sem checagem | `builder_infdoc.go:39-41` |

### A.5 seg, prodPred, tot, lacres, autXML, infAdic, infRespTec, infMDFeSupl, assinatura

| Caminho | Ocorr. | Tipo/tamanho/decimais | Status | Evidência |
|---------|--------|-----------------------|--------|-----------|
| `seg/infResp` (`respSeg` 1\|2, `CNPJ`\|`CPF`), `infSeg` (`xSeg` 1-30, `CNPJ`), `nApol` 1-20, `nAver` 1-40 | 0-n | | Coberta; F91 a F93 sem pré-validação | `seg.go:20-45` |
| `prodPred` (`tpCarga` 01-12, `xProd` 1-120, `cEAN`, `NCM` `[0-9]{2}|[0-9]{8}`) | 0-1 | | Parcial: NCM só 8 dígitos no validador (XSD aceita 2); `tpCarga=12` ausente do DAMDFE | `builder.go:368-378`, `validators.go:27` |
| `prodPred/infLotacao/infLocalCarrega|infLocalDescarrega` (`CEP`, `latitude` ER37, `longitude` ER38) | 0-1 | 6 decimais | Parcial: lat/long sem validação de formato | `builder.go:379-390`, `emit.go:290-297` |
| `tot/qCTe`, `qNFe`, `qMDFe` | 0-1 | `[0-9]{1,6}` | Parcial: ver defeito de `qNFe`/`qCTe` abaixo | `builder.go:394-410` |
| `tot/vCarga` | 1-1 | N13,2 | Coberta (`StringFixed(2)`) | `builder.go:396` |
| `tot/cUnid` | 1-1 | 01\|02 | Parcial: só 01 (KG) | `builder.go:397` |
| `tot/qCarga` | 1-1 | N11,4 | Parcial: sem checar 11 inteiros | `builder.go:398` |
| `lacres/nLacre` | 0-n | C1-60 | Coberta | `builder.go:140` |
| `autXML` (`CNPJ`\|`CPF`) | 0-10 | | **Ausente** | n/a |
| `infAdic/infAdFisco` (1-2000), `infCpl` (1-5000) | 0-1 | | Coberta | `builder.go:165-177` |
| `infRespTec` (`CNPJ`, `xContato`, `email`, `fone`, `idCSRT`, `hashCSRT`) | 0-1 | | Coberta | `builder.go:146,478` |
| `infSolicNFF/xSolic` | 0-1 | C2-8000 (MOC diz 2000) | Ausente (fora de escopo declarado) | n/a |
| `infPAA` (`CNPJPAA`, `PAASignature`, `SignatureValue`, `RSAKeyValue`) | 0-1 | | Ausente (fora de escopo declarado) | n/a |
| `infMDFeSupl/qrCodMDFe` | 0-1 | ER49, C50-1000 | Parcial: sem `sign` (contingência) | `builder.go:150-152` |
| `ds:Signature` | 1-1 | RSA-SHA1, enveloped, C14N | Coberta | `go-dfe/internal/xmlops/signer.go` |

Defeito de `tot`: `qNFe` é usado sempre que o primeiro documento não é CT-e (`builder.go:405-409`); e a contagem
`n` é o total de `cargo.docs`; correto hoje porque não se mistura NF-e e CT-e, mas o código e F51 só fecham por
construção.

### A.6 Eventos e serviços

| Item | Estado | Evidência |
|------|--------|-----------|
| 110111 Cancelamento (`descEvento`, `nProt`, `xJust` 15-255) | Coberto | `events.go:143`, `dto.go:1291` |
| 110112 Encerramento (`dtEnc`, `cUF` incl. 99, `cMun`, `indEncPorTerceiro`) | Parcial: `EX`/99 recusado (UF sem EX); sem K02/K03/K04 | `events.go:167-200` |
| 110114 Inclusão de condutor | Coberto | `events.go:203` |
| 110115 Inclusão de DF-e (`chNFe` só NF-e) | Coberto; só `len=44,numeric` na chave | `events.go:222`, `emit.go:262-264` |
| 110116, 110117, 110118 | Cobertos (builders de `infPag` compartilhado) | `payment_events.go:94+` |
| `infEvento/infSolicNFF`, `infPAA` | Ausentes (fora de escopo) | n/a |
| Controle de `nSeqEvento` | **Parcial**: cliente informa; padrão 1, sem sequenciador por tipo (K01: até 99; 631 em duplicidade) | `v1/mdfes.go:244-249` |
| Eventos do Fisco (310620, 310112, 510620, 240170, 9001xx) | Só leitura (distribuição) | `worker/.../distribution_parser.go` |
| MDFeConsulta, MDFeConsNaoEnc, MDFeStatusServico | Transporte pronto em go-dfe; **sem fluxo** em api/worker/UI | `go-dfe/internal/endpoints/table.go:442-449` |
| MDFeDistribuicaoDFe | Coberto (distNSU; sem consNSU pontual documentado) | `worker/cmd/distribution-*` |

---

## B. Regras de validação do MOC que não são pré-validadas localmente

Convenções: **P0** = rejeita hoje quase tudo ou tem prazo regulatório; **P1** = causa frequente, barata de prevenir
sem dados externos; **P2** = exige tabela embarcada ou consulta ao banco próprio; **P3** = depende de base externa
(ANTT, SENATRAN, RFB, SEFAZ) e deve ficar como "facultativa" ou ser tratada só na resposta.
"Por construção" = o código gera o valor certo e a regra não precisa de checagem extra.

### B.1 P0

| Regra | cStat | Condição resumida | Situação atual |
|-------|-------|-------------------|----------------|
| F35/F36, F41/F42 | 602, 603, 606, 607 | `SegCodBarra` só para FS-DA; 36 dígitos | Emitido sempre com 44 dígitos (`builder_infdoc.go:66`) |
| Schema (F07, base 215) | 215, 580 | `SegCodBarra` em `infMDFeTransp` inexistente no XSD | Emitido (`builder_infdoc.go:40`) |
| F14, F15, F16, F17 | 638, 639, 540, 541 | tpEmit x grupos de documentos; tpEmit 3 só intramunicipal/intraestadual | tpEmit fixo 2; CT-e leva 639 |
| F26, F27 | 616, 706 | Sem documentos só com `indCarregaPosterior=1`, que proíbe documentos | `documents min=1` impede o carregamento posterior |
| F113, F113a, F113b (NT 2026.001, produção 23/11/2026) | 684, 303, 304 | CIOT obrigatório (RNTRC com CIOT, TAC, tpEmit 1/3); dados bancários e `infPag` para TAC | Nenhuma exigência; CIOT único, CPF do 1o condutor, sem CNPJ |
| F58 | 746 | soma de `Comp/vComp` = `vContrato` (tolerância R$ 0,01) | Sem checagem em emissão nem nos eventos 110116/110118 |
| Eventos J08/K09/K06 (worker) | 631, 609, 218 | duplicidade de evento, MDF-e já encerrado, já cancelado | Worker só reconhece 573 (NF-e) e não conhece 134 |

### B.2 P1 (alta frequência, sem dados externos)

| Regra | cStat | Condição | Observação de implementação |
|-------|-------|----------|----------------------------|
| F28, F29 | 668, 669 | chave de CT-e/NF-e duplicada (interestadual: no MDF-e; interna: no município) | dedupe em `resolveCargo`; retornar a chave |
| F30, F37, F45 (e F36d) | 601, 604, 649 | chave válida: CNPJ/CPF, ano (CT-e 2008, NF-e 2006, MDF-e 2012), mês, modelo 57/55/58, número, tpEmis, UF, DV | hoje `len=44,numeric`; `ValidAccessKey` é só mod 55 e numérico |
| F30a, F37a, F45a, F36b | 518, 519, 520, 525 | AAMM da chave anterior a 6 meses da autorização | ler da própria chave |
| F33, F40, F48 | 673, 677, 657 | documento cancelado/denegado na base | `resolveCargo` lê o item mas não olha `status` |
| F34 | 702 | `infEntregaParcial` só em modal aéreo | DTO aceita `partial_deliveries` em qualquer modal |
| F43, F44 | 647, 648 | `infMDFeTransp` só no modal aquaviário e UF carga/descarga AM ou AP | sem checagem |
| F08, F11 | 456, 612 | 2 primeiros dígitos de `cMun` = código da UF (carga e descarga) | `cMun` vem do XML do documento ou do override |
| F21, F22, F23, F24 | 703, 704, 705, 707 | carregamento posterior: 1 município de carga e 1 de descarga iguais, UF igual e diferente de EX, modal rodoviário, tpEmit 2 | sem checagem |
| F69, F70, F71, F72 | 232, 233, 234, 229 | série x CNPJ/CPF (CPF só 920-969 e tpEmit 2), IE obrigatória | sem checagem |
| F77, F78 | 407, 408 | município do emitente x UF, existente | fallback silencioso `0000000` |
| F89c | 523 | `tpRod=03` (cavalo) exige ao menos 1 reboque | sem checagem |
| F99 | 577 | CPF de condutor duplicado | sem checagem (F100 coberta por `cpf`) |
| F63, F52, F53 | 739, 724, 729 | `vAdiant` proibido à vista; `infPrazo` obrigatório a prazo e proibido à vista | só o "prazo exige parcela" existe (`payment_events.go:134`) |
| F59, F60, F61, F62 | 735, 736, 737, 738 | parcelas sequenciais com 3 dígitos; datas >= emissão e crescentes; parcelas + adiantamento = contrato | cobertas por construção na emissão (`ExpandInstallments`); **não** nos eventos 110116/110118 (parcelas declaradas) |
| F65, F66 | 741, 742 | contratante = emitente quando há `prop`; contratante duplicado | sem checagem |
| F94 | 578 | sem CIOT e sem pagador de vale-pedágio exige ao menos 1 contratante | sem checagem |
| F55, F55a, F55b | 726, 301, 302 | um único DF-e: `infLotacao`, NCM em `prodPred`, `infPag` | só `infLotacao` (CEP) é exigido; NCM e `infPag` não |
| F91, F92, F93, F108 | 698, 699, 542, 688 | tpEmit 1/3: seguro completo, responsável, RNTRC | dependem de suportar tpEmit 1/3 (gap 3) |
| F85, F86, F87, F88 | 611, 686, 462, 662 | MDF-e não encerrado por placa/tipo/UF; mais de 30 dias; mais de 5 dias com até 2 UF; sentido oposto | pré-check local pela tabela `mdfes` e por `MDFeConsNaoEnc` |
| F114 a F119 | 480-496 | QR Code: URL, chave, `sign` presente só em contingência e igual ao calculado | só a URL normal |
| K01 a K14 (encerramento, condutor, DF-e, pagamento) | 636, 614, 689, 714, 715, 708, 709, 711, 722-724, 735-739, 747-749, 220, 219, 710 | ver 3 e 4 abaixo | sem pré-validação: cancelar após 24 h (220), `cMun` x UF e exterior (614, 689, 714), MDF-e com carregamento posterior sem inclusão (715), condutor só rodoviário (644), tpProp=0 no pagamento (723), sequência (636) |

### B.3 P2 (tabela embarcada ou consulta ao próprio banco)

| Regra | cStat | O que falta |
|-------|-------|-------------|
| F09, F12 | 405, 406 | tabela de municípios IBGE (nenhuma no repo) |
| F10, F13 | 685, 680 | cobertas por construção; cobrir também overrides `loadings/unloadings` |
| F36a | 524 | tipo CT-e Simplificado (saber o modelo do CT-e referenciado) |
| F36c, F36d, F36e a g | 526-530 | chave de NF-e de prestação parcial: duplicidade, formato, existência |
| F49, F50 | 658, 659 | MDF-e referenciado: modal rodoviário e mesmo tipo de emitente (tabela `mdfes` própria) |
| F89 | 646 | placas de exterior (`[A-Z]{2,3}[0-9]{4}|...|[A-Z0-9]{7}`) |
| F90 | 663 | ordem do percurso por fronteiras entre UF (tabela de adjacência) |
| F73 | 209 | dígito da IE por UF |
| F25 | 712 | MDF-e com carregamento posterior sem DF-e há mais de 168 h |
| F105, F106, F107 | 660, 661, 459 | `autXML` (inexistente) |
| PAA01 a PAA06 | 909-915 | fluxo PAA (fora de escopo) |

### B.4 P3 (bases externas; tratar como facultativas ou só na resposta)

F31, F32, F38, F39, F46, F47 (existência/diferença do documento na SEFAZ; contingência dispensa), F74, F75, F76, F76a, F76b,
F81 a F84 (banco do MDF-e), F89a, F89b, F97, F109 a F112, F120 a F122.

---

## C. Contingência

### C.1 O que o MOC prescreve para o MDF-e (única forma: Offline)

| Item | Regra do MOC |
|------|--------------|
| Modo | `tpEmis=2` (Contingência Off-line). **Não há SVC, EPEC, FS-DA** no MDF-e; todos os serviços vão para SVRS |
| Quem decide | Exclusivamente o contribuinte, sem autorização prévia; só para falha técnica real; o Fisco pode restringir por uso excessivo |
| Chave | Nova chave de acesso com `tpEmis=2`, **mesmo `cMDF`**; na transmissão posterior mantém a mesma chave e o mesmo `cMDF` |
| Numeração | Recomenda-se avançar um número ao entrar em contingência (evita 539/204 por duplicidade); sem série específica |
| Prazo | Transmitir em até **168 horas** da emissão em contingência (F80 só limita 24 h para `tpEmis=1`) |
| QR Code | Parâmetro `sign` obrigatório: RSA-SHA1, Base64, da chave de acesso, com o certificado que assina o MDF-e (F117/F119; F118 proíbe em normal) |
| DAMDFE | Impresso em papel comum, com "EMISSÃO EM CONTINGÊNCIA" em destaque no local do protocolo, e chaves dos documentos agregados (ou dados dos documentos em papel) |
| Riscos | Rejeição posterior com carga em trânsito; consulta pública/QR só após autorização (206: "volte a consultar após 24h"); risco de perda até constar na base |
| Dispensas | F31, F32, F33 (CT-e) e F46, F47, F48 (MDF-e transportado) não se aplicam a documento em contingência |
| Rejeição na retransmissão | Corrigir e retransmitir (a própria regra de numeração do CT-e; o MOC de MDF-e só pede para não reutilizar numeração de normal) |

### C.2 O que existe

- `ide/tpEmis` parametrizável e `tpEmis` entra na chave (`emit.go:474-500`, `shared.go:275`); constantes
  `TpEmisContingencia`/`TpEmisRegEspNFF` declaradas (`builder.go:24-25`).
- DAMDFE já exibe a faixa `EMISSÃO EM CONTINGÊNCIA` e oculta protocolo quando `tpEmis=2` (`documents/mdfe.go:71-75`,
  `_damdfe_macros.html:22`).
- Plano `2026-08-26-contingencia-e-inutilizacao-dfe.md` define fases C0 a C7 (C0 concluída apenas na NF-e; C6 do MDF-e
  "não iniciado").
- Todos os serviços de MDF-e já roteiam para SVRS (`endpoints/table.go:452-466`), então, ao contrário da NF-e,
  **não há tabela de SVC por UF a transcrever** para o MDF-e: o bloqueio "C1" do plano não se aplica ao MDF-e.

### C.3 O que falta (fluxo completo)

| # | Item | Observação |
|---|------|------------|
| C-1 | Modo de contingência por organização (estado, `dhEntrada`, justificativa interna), com saída manual ou automática | Reusar o desenho de `emission_mode` da NF-e (`nfes/emission_mode.go`); estado por org + documento |
| C-2 | `tpEmis=2` resolvido em `Emit` e chave gerada com `tpEmis=2` mantendo o `cMDF` quando a transmissão posterior reaproveita a chave | Hoje a chave é sempre regerada com `rand` (`shared.go:288`); precisa de parâmetro `cMDF` |
| C-3 | Persistir o MDF-e **assinado, não transmitido**, com `status=contingency_pending`, `contingency_deadline = dhEmi + 168 h` e `deadline_alerted_at` | Hoje a assinatura ocorre na egress durante a chamada e o XML só é guardado após resposta (`worker/dfe.go`); é preciso um modo "só assinar" na egress |
| C-4 | QR Code com `sign` (RSA-SHA1 da chave, mesmo certificado) | Helper em `go-dfe` (a assinatura RSA-SHA1 já existe em `signer.go`) |
| C-5 | DAMDFE a partir do XML assinado pré-autorização (sem protocolo) | `GetDAMDFEURL` exige `xml_s3_key` (`mdfes/documents.go:20`) |
| C-6 | Fila de retransmissão com prazo: reaproveitar `worker_outbox`; relógio de 168 h; alarme via go-common `alerts` antes do vencimento | Padrão já usado por `outbox-stuck` |
| C-7 | Reconciliação de "pendente de retorno" (timeout sem resposta): consultar `MDFeConsulta` antes de reenviar ou rejeitar | Hoje, falha de rede vira `retryable_failed` e o reenvio recria a chamada sem consultar; resposta 204 (duplicidade) com protocolo já autorizado não é tratada como sucesso |
| C-8 | Rejeição na retransmissão: manter a mesma numeração só se o documento anterior não foi autorizado; fluxo para corrigir e reemitir; opção de cancelar se autorizado indevidamente | Terminal `rejected` hoje queima a numeração |
| C-9 | Painel "Contingência": janelas abertas, pendentes e prazo restante (fase C7 do plano) | UI não tem nada de contingência para MDF-e |
| C-10 | Regras F25/F80 em contingência: `dhEmi` antigo permitido; sem F80 | Só ajusta validação local |
| C-11 | Detecção de indisponibilidade pela consulta de status (`MDFeStatusServico`: 107, 108, 109, `tMed`, `dhRetorno`) | Sem fluxo hoje; MOC pede intervalo mínimo de 3 min entre consultas |
| C-12 | Documentação: DOCS.md e CONDUCT.md com a máquina de estados (exigência do próprio plano C) | |

---

## D. DAMDFE

### D.1 Formatos exigidos pelo MOC (Anexo II 3.00b)

| Formato | Regra |
|---------|-------|
| Orientação | Papel comum, **retrato ou paisagem**; várias folhas conforme necessário; até 50% do verso, com "CONTINUA NO VERSO" no fim do quadro Observação |
| Modelos | 8 modelos: rodoviário, aéreo, aquaviário e ferroviário, cada um em emissão normal e em contingência |
| Código de barras | CODE-128C da chave de 44 caracteres (normal e contingência), 300 dpi, área 3 cm x 9 cm, altura 1,5 a 2,5 cm |
| QR Code | ISO/IEC 18004, nível M, mínimo 25 mm x 25 mm (22 mm de conteúdo + 3 mm de margem); em tamanho maior, margem de 10%; URL de `qrCodMDFe`, com `sign` em contingência |
| Contingência | "EMISSÃO EM CONTINGÊNCIA" no local do protocolo + chaves/dados dos documentos |
| Homologação | "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL", centralizado, caixa alta |
| Conteúdo | Só o que consta no XML, exceto o protocolo; legibilidade por 12 meses |

### D.2 O que existe

| Item | Estado | Evidência |
|------|--------|-----------|
| Geração de PDF A4 retrato a partir do XML autorizado, Folio + Gonja, cache em S3 por estado (ativo/cancelado) | Existe | `documents/renderer.go`, `service.go:79`, `mdfes/documents.go` |
| Quatro modais num só template com blocos `is_rodo`, `is_aereo`, `is_aqua`, `is_ferrov` | Existe (conferir visualmente com os 8 modelos oficiais) | `_damdfe_macros.html:68-112` |
| CODE-128C da chave e QR Code embutidos | Existe | `barcode.go`, `documents/mdfe.go` |
| Faixa de contingência, faixa de homologação, marca "CANCELADA" | Existe | `_damdfe_macros.html:22`, `documents/mdfe.go` |
| Protocolo omitido em contingência | Existe (porém omite também depois de autorizado em contingência; ver D.3) | `documents/mdfe.go:71-75` |

### D.3 O que falta

| # | Item | Observação |
|---|------|------------|
| D-1 | Leiaute **paisagem** (`damdfe_paisagem.html` ou parâmetro de orientação) | Só `layout: "retrato"` (`documents/mdfe.go:80`, `constants.go:45`) |
| D-2 | Conferência dos 8 modelos oficiais (cabeçalhos, quadros por modal, verso "CONTINUA NO VERSO", 50% do verso) | Não consegui ver as imagens do MOC |
| D-3 | DAMDFE antes da autorização (contingência), a partir do XML assinado, e DAMDFE com protocolo depois que a transmissão posterior autorizar | Hoje `GetDAMDFEURL` exige `xml_s3_key` e o contexto esconde o protocolo sempre que `tpEmis=2` |
| D-4 | QR de contingência com `sign` no XML e conferência do tamanho mínimo | |
| D-5 | Rótulo de carga `12 - Granel pressurizada` (XSD e MOC o têm) | `cargoLabels` vai só até 11 (`documents/mdfe.go`) |
| D-6 | Campos do XML ainda não exibidos: `infCIOT` múltiplos, `valePed`, `infContratante`, `infPag`, `autXML`, `infLotacao`, `peri`, `infUnidTransp`, `veicReboque/prop`, MMSI, terminais e comboio (`aquav`), `vag` completo (peso, tipo, TU), `infMDFeTransp`, entrega parcial | Confirmar contra o modelo do Anexo II; o MOC diz que o DAMDFE imprime "o que consta no XML" |
| D-7 | Mais de uma folha e paginação por documento (até 20000 documentos por manifesto) | `maxRenderedHTMLBytes`/`maxHTMLElements` limitam o HTML |
| D-8 | Rótulo do `tpEmis=3` (NFF) e de `procEmi`; faixa específica para homologação em contingência | |
| D-9 | Download do DAMDFE também no estado "pending" e "contingency_pending" | |
| D-10 | Opções de apresentação (logo, caixa) estão como "knobs" sem ligação a `fiscal_config` (`documents/mdfe.go`) | |

---

## E. Divergências de estrutura recomendada

| # | Tema | Situação no repo | MOC/XSD | Ação |
|---|------|------------------|---------|------|
| E-1 | **XSDs não versionados** | `xsdorder/table.go` cita o XSD como verdade, mas nenhum `.xsd` está no git (só em `cdk/cdk.out`, ignorado) | MOC pede validar contra o schema antes de enviar | Vendorizar `PL_MDFe_300b_NT012025_*` em `go-dfe/schemas/` (dados, não código) e testar a tabela de ordem contra eles |
| E-2 | **Ordem xsdorder do MDF-e** | `table.go:530-641`: confere com o XSD 1.05 em `ide`, `emit`, `infMunDescarga`, `infCTe` (XSD põe `indPrestacaoParcial` e `infNFePrestParcial` depois de `infEntregaParcial`), `seg`, `prodPred`, `tot`, `rodo`, `aquav`, `ferrov`; o repo está correto frente ao XSD | O MOC online mostra `indPrestacaoParcial`/`infNFePresParcial` logo após `chCTe` (e grafa `infNFePresParcial`), o que **diverge do XSD**; o repo segue o XSD | Nenhuma mudança; registrar em CONDUCT que o MOC online não é fonte de ordem |
| E-3 | Chave de `enderEmit` | `compartilhado["enderEmit"]` tem ordem `UF, CEP, cPais` e `infMDFe:emit:enderEmit` tem `CEP, UF`; a precedência depende da ordem do merge (`table.go:728-758`) | XSD MDF-e: `...cMun, xMun, CEP, UF, fone, email` | Adicionar teste que verifica o `enderEmit` do MDF-e via `Lookup("infMDFe:emit:enderEmit")` e o caminho de consulta usado pelo `Reorder` |
| E-4 | Namespace | Um `xmlns` no `MDFe` raiz, sem prefixo (`builder.go:156`, `mdfeXMLNS`) | Uma declaração, sem prefixo (D04, 404, 598) | Conforme |
| E-5 | Envelope | Raiz `MDFe` sem `enviMDFe` e `gzip+base64` (`client.go:35-40`); sem SOAP Header (`Build(..., false)`) | MDFeRecepcaoSinc: gzip+base64; header descontinuado | Conforme |
| E-6 | Assinatura | RSA-SHA1, SHA-1, enveloped, C14N, referência `infMDFe`/`infEvento` (`config.go:101-115`, `signer.go`) | Mesmo | Conforme; para contingência, reaproveitar para o `sign` do QR |
| E-7 | Eventos | `@Id` = `ID`+tpEvento+chave+nSeq com `%02d` (`events.go:272`); `descEvento` compatíveis com o XSD (`Inclusao Condutor`, `Inclusão DF-e`, `Pagamento Operação MDF-e`, etc.) | Id 54 ou 55 dígitos; `nSeqEvento` 1-3 dígitos | Conforme; falta sequenciador e `infSolicNFF`/`infPAA` |
| E-8 | Pagamento: `tpComp=04` e componente Frete | `oneof=01 02 03 04 99` (`payment_events.go:34`) | NT 2025.001 v1.03 (produção 10/2025) | Conforme |
| E-9 | CNPJ alfanumérico | Chave aceita letras só em `ValidAccessKey` da NF-e (`access_key.go:37-47`); MDF-e usa `numeric`; `ValidCNPJ` numérico; XSD 1.05 já permite `[A-Z0-9]{12}[0-9]{2}` | NT conjunta 2025.001: produção 06/07/2026 | Estender `GenerateAccessKey`, validadores e UI para alfanumérico; o MOC diz para ainda não usar letras até nova NT, mas o XSD aceita |
| E-10 | `TpValePed` | Passa o valor do cadastro (DTO); MOC D15 lista 01, 02, 03 e 04 | XSD: enum `01`,`04` | Limitar ao XSD (01 TAG, 04 leitura de placa) |
| E-11 | Limites de documentos | Sem limite local | XSD 20.000 por tipo e município de descarga (NT 2024.001 v1.02); MOC 10.000 na tabela do leiaute | Usar o XSD (20.000) e registrar a divergência |
| E-12 | `xSolic` | Não emitido | XSD 8000; MOC 2000 | Usar XSD ao implementar |
| E-13 | `procEmi` | `0` | XSD `0|4` (4 = PAA); MOC D13 só `0` | Só relevante com PAA |
| E-14 | Distribuição | `mdfe_distributions` por NSU; parser para `procMDFe`/`procEventoMDFe` | MOC: `consNSU` pontual, `docZip` por NSU, cStat 137/138, 678 (uso indevido), até 6 meses | Conforme no essencial; documentar consNSU e espera de 1 h com `ultNSU=maxNSU` |
| E-15 | Prazos de serviço | Consulta limitada a 180 dias (G02: 460 após 6 meses); status mínimo 3 min | | Constantes nomeadas quando as consultas existirem |
| E-16 | `GenerateAccessKey` com CPF | `cnpj += "0"` até 14 (**zeros à direita**) | CPF precedido de zeros (**à esquerda**) | Corrigir (afeta NF-e e MDF-e de emitente pessoa física); também CPF no emit exige série 920-969 e tpEmit 2 |
| E-17 | `go-dfe` e validação XSD | Fora de escopo por decisão (CGO desligado) | MOC: "mensagens devem ser validadas com o Schema antes do envio" | A tabela por dados (seção F) cumpre o papel sem CGO; precisa de decisão registrada |

---

## F. Validador genérico dirigido por tabela e plano em fases

### F.1 Princípios

- **Pura Go, sem CGO** (compatível com ARM64 `provided.al2023` e com a decisão de `go-dfe/CLAUDE.md`). Nenhum parser XSD em
  runtime: a tabela é gerada do XSD em build/CI e versionada como dados.
- **Um só motor** para MDF-e, reaproveitável por CT-e e NF-e (mesmos `TString`, `TCnpj`, `TDec_*`). Local sugerido:
  `go-dfe/validate` (pacote puro, importado por `api`, pela egress como portão final e por testes), por ser fiscal e
  não do `ctech-go-common`. **Pergunta aberta ao dono**: aceitar `go-dfe` ou propor a `ctech-go-common`?
- **Duas camadas**: (1) regras de forma (tipo, tamanho, decimais, formato, ocorrência, choice), 100% por tabela, geradas
  do XSD; (2) regras de negócio condicionais (códigos F/K do MOC), declarativas em Go, com `id`, `cStat`, mensagem do
  MOC e predicado de aplicação.
- **Saída**: RFC 7807 (`problem.*` com `errors[]` por caminho XML e JSON), cada item com `rule` (ex.: `F58`), `cStat`
  equivalente do MOC e mensagem em pt-BR, sem travessão. Resposta 422 antes de debitar cota e antes de gravar outbox.
- **Valida o resultado do builder** (`map[string]any`), não o DTO: protege contra bug de builder (gap 1 e 2 teriam sido
  pegos) e cobre todos os modais com a mesma regra. O DTO continua com validação de ergonomia.
- Constantes nomeadas para limites e prazos (168 h, 6 meses, 30 dias, 5 dias, tolerância R$ 0,01), nada de número solto.

### F.2 Estrutura da tabela

Gerada de `mdfeTiposBasico_v3.00.xsd`, dos 4 modais, de `eventoMDFe` e dos `ev*`; um arquivo Go por documento
(`spec_mdfe_gen.go`), com `go:generate` e um golden test que falha se o XSD vendorizado mudar sem regerar.

```go
// Spec descreve um nó do documento; o caminho é o do XSD (sem namespace).
type Spec struct {
    Path     string   // "infMDFe/rodo/infANTT/infPag/Comp/vComp"
    Min, Max int      // ocorrência; Max -1 = ilimitado
    Kind     Kind     // String, Digits, Decimal, Date, DateTime, Enum, Base64, Doc(CNPJ/CPF/Alfa)
    Len      Range    // C1-60, N2, etc.; Exact para comprimento fixo
    Int, Frac int     // TDec_1302: 13 e 2; Frac fixa ou faixa (2-3)
    Pattern  string   // ER do XSD, compilada uma vez
    Enum     []string // D5 etc.
    Choice   string   // nome do grupo choice (CNPJ|CPF)
    Seq      string   // grupo sequence opcional (IE+UF do prop)
    Digit    DVCheck  // none, CPF, CNPJ, chave, IE(UF)
}
```

Motor: percorre o mapa do builder com a tabela, checa ocorrência, choice, tipo, tamanho, decimais (conta dígitos
inteiros e fracionários sem converter para float), `Pattern`, `Enum`, DV de CPF/CNPJ/chave, ordem opcional (reaproveita
`xsdorder`), e devolve `[]Violation{Path, Rule, CStat, Msg}`. Reuso explícito: `xsdorder` já conhece a ordem; a nova
tabela é irmã dela e deve ser gerada do mesmo XSD vendorizado.

### F.3 Regras condicionais (negócio)

```go
type Rule struct {
    ID     string                 // "F58"
    CStat  int                    // 746
    Msg    string                 // texto do MOC
    When   func(c *Ctx) bool      // tpEmit, modal, tpEmis, indCarregaPosterior, presença de grupos
    Check  func(c *Ctx) []Violation
    Needs  Needs                  // nada | tabela embarcada | banco próprio | externo
}
```

`Ctx` expõe leitura por caminho (`c.Str("ide/tpEmit")`, `c.All("infDoc/infMunDescarga/infNFe/chNFe")`) e decimais
exatos (`shopspring/decimal`, já usado). Regras agrupadas por contexto como no MOC (2.1 a 2.14), registradas em tabela
`[]Rule`; **teste de completude** lê a lista de IDs do MOC (arquivo de dados) e falha se alguma regra F/K/J ficar sem
implementação ou sem justificativa de "por construção" ou "externa". Regras `Needs: banco` (F85 a F88, F25, F49, F50)
recebem uma interface `Lookup` implementada no `api` com consultas por índice (nunca Scan): `mdfes` por `number-index`,
placa pelo `dfe-index` (ou novo GSI), e `MDFeConsNaoEnc` como fonte autoritativa.

### F.4 Dados embarcados (tabelas pequenas, versionadas)

Municípios IBGE (código x UF x nome), fronteiras entre UFs, domínios D1 a D21 e ER1 a ER61 (geradas do MOC/XSD),
mapa regra para cStat. Tudo em `go:embed`, com teste de integridade.

### F.5 Fases, tamanho e dependências

Tamanhos em dias de uma pessoa (P até 3, M 4 a 8, G 9 a 15), incluindo testes e docs obrigatórios do repo.

| Fase | Entrega | Tamanho | Depende de |
|------|---------|---------|-----------|
| **0. Correções imediatas** | (a) `SegCodBarra` correto (36 dígitos, só FS-DA; nunca em `infMDFeTransp`) e ajuste do teste; (b) `tpEmit` derivado do tipo de documento e do cadastro (1 CT-e, 2 NF-e, 3 globalizado), com F14 a F17; (c) worker: 134 como sucesso de evento, 218/609/631 por documento (substituir `573` genérico por tabela por `doc_type`); (d) `GenerateAccessKey` com CPF à esquerda; (e) `EX` aceito em UF e `cMun=9999999`; (f) permitir `documents` vazio quando `indCarregaPosterior` | M (6) | nenhuma |
| **1. Motor genérico** | `go-dfe/validate`: gerador XSD para tabela, motor de forma, saída 7807, golden test, vendorizar XSDs | M (7) | decisão sobre local do pacote |
| **2. Regras de forma do MDF-e e eventos** | Aplicar o motor no `BuildMDFe` e nos 7 eventos (todos os tamanhos do XSD, decimais, enums, choice); alinhar DTOs (rodoviário `lacRodo` 20, ferroviário 1-3, `cPrtEmb` 1-5, `nVag`, `nSeq`, `tara` inteira, placa de exterior, RNTRC 8, `PIX` 60, NCM 2 ou 8); reduzir tags `validate` redundantes | M (6) | 1 |
| **3. Regras F/K sem dados externos (P1)** | F08, F11, F21 a F24, F26 a F29, F30/F37/F45 e idade de 6 meses, F33/F40/F48, F34, F43, F44, F52 a F66, F69 a F72, F77, F89c, F99, F94, F55*, K01 a K14, 24 h do cancelamento | G (10) | 1 |
| **4. Tipos de emitente completos** | tpEmit 1 e 3: seguro obrigatório, RNTRC, `infPag` e CIOT por regra, tpTransp, contratante; CPF (série 920-969, tpEmit 2); `veicReboque/prop`; CIOT 0-n com CPF/CNPJ do responsável; `autXML` (0-10) com F105 a F107; NT 2026.001 (CIOT) antes de **23/11/2026** | G (10) | 0, 2 |
| **5. Contingência (C-1 a C-12)** | modo offline, chave com mesmo `cMDF`, "só assinar" na egress, QR `sign`, DAMDFE pré-autorização, fila e prazo de 168 h, painel e alertas, reconciliação 204 | G (12) | 0, 2; egress |
| **6. Consultas e pré-checagens de não encerrados** | `MDFeConsulta` (situação e reconciliação), `MDFeConsNaoEnc`, `MDFeStatusServico` (com intervalo de 3 min), F85 a F88 locais e remotos, F25 | M (7) | 1 |
| **7. Tabelas embarcadas (P2)** | IBGE, fronteiras, IE por UF; F09, F12, F90, F73, F89 de exterior | M (5) | 1 |
| **8. DAMDFE completo** | paisagem, 8 modelos conferidos, contingência pré e pós autorização, campos faltantes (D-6), `tpCarga=12`, paginação | M (6) | 5 (D-3), 4 (campos) |
| **9. CNPJ alfanumérico** | chave de acesso, `ValidCNPJ`, DTOs, UI, DAMDFE | M (4) | 1 |
| **10. NFF e PAA (opcional, hoje fora de escopo)** | `infSolicNFF`, `infPAA`, `procEmi=4`, `tpEmis=3`, PAA01 a PAA06 e eventos com `infPAA` | G (12+) | 1 |

Total aproximado: 11 a 13 semanas-pessoa para fases 0 a 9; fase 10 é opcional. Ordem recomendada: 0, 1, 2 (alto
retorno, baixo risco), 4 (por causa da data do CIOT), 3, 6, 5, 8, 7, 9.

### F.6 Padrões do repo a respeitar na execução

- Sem Scan (índices existentes ou novo GSI documentado em `DynamoDB-Tables.md`); fiscal sempre via `go-dfe-egress`
  (inclusive a nova consulta de situação, não encerrados e status); erros RFC 7807 via `problem.*`; constantes
  nomeadas para cStat, prazos e limites; texto visível ao usuário sem travessão; UI só com seletores onde possível.
- Cada fase atualiza `DOCS.md`, `CONDUCT.md` e, na UI, o guia (`/guide`) e `nav.tsx` na mesma alteração.
- Testes: unitário do motor e de cada regra, contrato (tabela gerada x XSD vendorizado), integração para a
  contingência (fila, prazo, reconciliação) e regressão para os defeitos da fase 0.
- Impacto cruzado a declarar em cada PR: `api`, `worker`, `go-dfe`, `go-dfe-egress` (modo "só assinar" e consultas),
  `ui` (emissão, detalhe, contingência), `cdk` (permissões/alarme do prazo, se aplicável).

---

## Anexo 1. Pontos de baixa severidade encontrados

- `Close` grava `dtEnc` com comentário "placeholder; overridden below", mas não há sobrescrita (`events.go:189`).
- `CIOT`, `dhIniViagem` e `RENAVAM` do DTO sem validador de formato (`emit.go:42-44`, `emit.go:246`).
- `resolveCargo` soma `infQ/qCarga` do CT-e sem filtrar a unidade de medida (`xmlparse.go:250-251`); volumes viram quilos.
- Para CT-e, município de carga/descarga vêm de `emit`/`dest` do CT-e e não de `cMunIni`/`cMunFim`
  (`xmlparse.go:238-243`), o que pode divergir do trajeto real do CT-e.
- `IncludeDFe` valida só `len(ChNFe)==44` (`events.go:239`) e o DTO `numeric`.
- `categCombVeic` só gera 02, 04, 06, 07; MOC lista 02 a 14 por número de eixos (cadastro de eixos do veículo
  resolveria).
- `UFEnd` default usa `docs[len-1].dest.uf`, não necessariamente a UF de descarga final de maior distância.

## Anexo 2. Onde está a tabela de apoio

A tabela de 315 caminhos (XSD 1.05, ocorrência, tipo, tamanho, decimais, padrão e enum) usada nesta auditoria foi
produzida por um script de achatamento sobre `cdk/cdk.out/asset.*/schemas/xsds/PL_MDFe_300b_NT012025_1.05`. Ela é o
ponto de partida do gerador da Fase 1; o script em si é descartável (a Fase 1 deve entregar o gerador definitivo, com
teste dourado).
