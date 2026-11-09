# Auditoria MOC CT-e x ctech-dfe: lacunas de cobertura, validação, contingência e DACTE

**Data:** 2026-11-04. **Tipo:** auditoria somente leitura (nenhum código, teste ou UI alterado).
**Escopo:** MOC on-line do CT-e (CT-e 57, CT-e OS 67, GTV-e 64, CT-e Simplificado 57) contra o monorepo
`ctech-dfe` (`api/`, `worker/`, `go-dfe/`, `go-dfe-egress/`, `ui/`).
**Meta do dono do produto:** estrutura que atenda 100% dos casos (100% das tags de todos os modais e tipos,
100% dos fluxos de contingência, DACTE em todos os formatos, validação interna criteriosa antes de ir à SEFAZ).

---

## 0. Resumo executivo

1. **CT-e não emite.** O repositório tem a infraestrutura de transporte e a distribuição (recebimento de CT-e de
   terceiros), mas **não existe** serviço de emissão, builder de XML, rotas `/ctes`, eventos, DACTE, contingência
   nem pré-validação para CT-e. A UI mostra "Emissão de CT-e em breve" (`ui/src/app/cte/page.tsx:213`) e o
   `MobileNav.tsx:24` registra que CT-e ainda não emite. O estado real confirma o texto do `ui/CLAUDE.md`.
2. Portanto os itens 1, 2, 3 do objetivo partem de **zero na camada de domínio** (api/worker/ui). O que existe
   e é reaproveitável: SOAP + mTLS + gzip + assinatura + endpoints por UF (go-dfe), `cteProc`/`procEventoCTe`
   (processor), tabelas e filas no CDK, distribuição por NSU, cadastros de pessoas e veículos, infraestrutura
   de PDF (folio + gonja) e o padrão de builder/emit de NF-e e MDF-e.
3. **Defeitos latentes em código existente que bloqueiam a emissão** (corrigir antes de construir em cima):
   - `go-dfe/internal/xmlops/xsdorder/table.go:456-460` usa `infCteNorm` e `infCteSupl`; o MOC usa
     `infCTeNorm` e `infCTeSupl`. Pela regra de resolução (`Resolve`), tag sem entrada ordena por ordem
     alfabética, o que gera XML fora da sequência do XSD.
   - `table.go:453-456` descreve `enviCTe` (lote, leiaute 3.00). O CT-e 4.00 é **síncrono sem lote**: a mensagem é
     o próprio `CTe`/`CTeOS`/`GTVe`/`CTeSimp` assinado e compactado (MOC CTeAutorizacao 2.1).
   - `go-dfe/internal/services/config.go:94-95` assina `infCTeOS` e `infGTVe`; o leiaute do MOC (seções 4 e 5)
     mostra o grupo assinado como `infCte` com `Id` nos três modelos. **Provável erro**; confirmar com o XSD
     do pacote de liberação antes de usar CT-e OS/GTV-e.
   - `ide` e demais ordens em `xsdorder` estão desatualizadas frente ao 4.00 (ver seção 2.2): faltam
     `retira`, `xDetRetira`, `gCompraGov`, `tpPagAnt`, `gPagAntecipado` e sobram `dhIniViagem`, `indCarga`,
     `infCte:...:UFIni` fora de lugar. Estimativa de ordem coberta: no máximo 56% (261 de 463 pares
     pai/filho do modelo 57), e esse número é um teto, porque chaves simples compartilhadas com NF-e/MDF-e
     (`ICMS`, `infNFe`, `IBSCBS`...) escondem lacunas.
   - `go-dfe` não valida schema (`validateSchema`, `client.go:54`) e `stringifyScalar` formata float com
     `FormatFloat('f', -1)` (`builder.go:221-235`): `10.5` sai sem a segunda casa. Casas decimais precisam
     ser garantidas **antes** de entrar no mapa do builder (a API de NF-e usa `shopspring/decimal`).
   - A distribuição pede `consChCTe` (`worker/internal/service/distribution.go:695`), mas o MOC do
     `CTeDistribuicaoDFe` só define `distNSU` e `consNSU`. `consChCTe` não existe no leiaute (e `xsdorder`
     tem só `consChNFe`, `table.go:445-448`).
4. O MOC mudou muito frente ao que o pedido assume: **inutilização, denegação, CT-e de anulação, serviço
   assíncrono, FS-DA (tpEmis 5) e EPEC (evento 110113) estão eliminados ou revogados** (Histórico de
   Implantação 4.00a e NT 2023.001 v1.03; Contingência; NT 2026.004 v1.00). Não construir esses fluxos.
   Contingência viva: `tpEmis=2` (offline), `7` (SVC-RS), `8` (SVC-SP) e `3` (Regime Especial NFF, só SVRS).
5. Maior volume de trabalho: ~1000 tags e ~620 regras de validação (219 do 57, ~135 do 67, ~75 do GTV-e, 121 do
   Simplificado, ~80 de IBS/CBS). Recomendação: **validador genérico dirigido por tabela gerada do MOC**
   (seção 7) e builders por modal, entregues em 8 fases (~30 a 45 dias úteis de uma pessoa).

### 0.1 Método e limitações (leia antes de confiar nos números)

- O `WebFetch` falhou (`ECONNREFUSED` em `moc.sped.fazenda.pr.gov.br:443`; o site não serve HTTPS). O MOC foi
  obtido por `curl` em HTTP, **todas as 16 páginas do menu** (Visão Geral, CTeAutorizacao, GTVeAutorizacao,
  CTeSimplificadoV4, CTeConsultaProtocolo, CTeStatusServico, NFeConsultaCadastro, RecepcaoEvento Geral e
  Específicos, CTeDistribuicaoDFe, Leiaute, Regras de Validação, DACTE, QR Code, Contingência, Tabelas e
  Informações, Histórico de Implantação). Convertidas em texto e tabelas em um diretório temporário
  (fora do repositório).
- **Não li:** as imagens (modelos oficiais do DACTE, DACTE OS, quadros); os XSDs (não estão no repositório e
  `py-dfe/schemas/xsds/` foi removido); o corpo integral das ~135 regras do CT-e OS e das ~75 do GTV-e (li
  as descrições truncadas e os códigos; as do GTV-e `I01..I75` só por amostra e pelo grupo QR Code); as 121 regras
  do CT-e Simplificado (3.3) e as ~80 de IBS/CBS (3.6 e 3.11) foram vistas por estrutura e por amostra, não
  uma a uma. Onde o MOC cita "item 12 do MOC" (lista de CFOP) usei a Tabela 3.1 da página de Tabelas.
- Tipo, tamanho e casas decimais vêm da tabela do Leiaute do MOC (coluna Tamanho: `13v2`, `13, 2`, `3v2-4`).
  Pode haver divergência entre o HTML e o XSD publicado; a tabela gerada deve ser conferida contra o XSD na
  Fase 0.
- "Coberta" na matriz significa XML gerado pelo repositório. Como não há builder de CT-e, nenhuma tag
  está coberta nesse sentido; a coluna de status distingue **ordem em xsdorder** (parcial) de **builder**
  (ausente).
- Nenhum arquivo do repositório foi tocado além deste documento.

---

## 1. Estado real do CT-e no repositório

| Camada | Existe | Evidência |
|---|---|---|
| go-dfe: serviços SOAP | `CTeRecepcaoSinc`, `CTeRecepcaoOS`, `CTeRecepcaoGTVe`, `CTeRecepcaoSimp`, `CTeRecepcaoEvento`, `CTeConsulta`, `CTeStatusServico`, `CTeDistribuicaoDFe` | `go-dfe/dfe.go:59-68`, `constants.go:144-207` |
| go-dfe: gzip+base64 nos 4 serviços de emissão | sim (certo conforme MOC 9.1) | `go-dfe/internal/services/client.go:37-40` |
| go-dfe: assinatura (tag do `Id`) | sim, **com 2 tags prováveis erradas** (`infCTeOS`, `infGTVe`) | `config.go:92-98` |
| go-dfe: endpoints por UF (MG, MS, MT, PR, SP, SVRS) | sim; **sem SVC-RS/SVC-SP de CT-e** | `endpoints/table.go:371-445` (nenhum `SVC`) |
| go-dfe: `cteProc`, `cteOSProc`, `GTVeProc`, `cteSimpProc`, `procEventoCTe` | sim | `xmlops/processor.go:14-22,43-47` |
| go-dfe: ordem XSD (`xsdorder`) | parcial e com nomes errados | `xsdorder/table.go:450-519` |
| api: config fiscal `PUT /organizations/{pk}/cte-config` | sim (apenas configuração) | `api/internal/api/v1/organizations.go:261`, `services/fiscal_configs.go:17,130` |
| api: tabelas `ctes`, `cte_events`, `cte_distributions` | sim (repositório genérico) | `repositories/documents.go:70-77`, `roles.go:68` |
| api: rotas de emissão/consulta/evento (`ctes.go`) | **não** (existe `nfes.go`, `nfces.go`, `mdfes.go`, `nfses.go`) | `api/internal/api/v1/` |
| api: builder, emit, cancelamento, CC-e, eventos | **não** (existe `services/nfes`, `services/mdfes`, `services/nfses`) | `api/internal/services/` |
| api: DACTE | **não** (`documents/` tem `nfe`, `nfce`, `mdfe`, `nfse`) | `documents/templates/` |
| api: chave de acesso mod 57/67/64 | **não**: `ValidAccessKey` aceita só `mod=55` | `validation/access_key.go:12,40` |
| api: requisito de veículo para CT-e OS | só a constante `cte_os` na matriz de campos obrigatórios | `services/vehicle_requirements.go:14` |
| worker: filas `cte-emission` e `cte-event` | definidas no CDK; o serviço do worker é genérico por `doc_type` | `cdk/lib/worker-definitions.ts:56-69`, `worker/internal/service/dfe.go` |
| worker: distribuição CT-e | sim (`cteProc`, `cteOSProc`, `GTVeProc`, `cteSimpProc`, `resCTe`, `procEventoCTe`) | `worker/internal/service/distribution.go:845-876` |
| ui: página CT-e | só aba de distribuição; emissão é `ComingSoon` | `ui/src/app/cte/page.tsx:213` |
| docs | ROADMAP histórico lista CT-e como futuro (`ROADMAP.md:163-212`); planos de cobertura cobrem só NF-e/MDF-e/NFS-e | `docs/plans/2026-08-26-cobertura-total-tags-nfe-mdfe.md` |

Consequência para a meta: a base de **comunicação** está pronta; faltam **modelo de dados, builders, validação,
eventos, contingência e DACTE**.

---

## 2. Matriz A: cobertura de tags

### 2.1 Convenções

- Ocorrência `min-max`; tipo `C` caractere, `N` numérico, `D` data, `T` hora, `E` enum; tamanho/decimais como no
  MOC (`13v2` = até 13 dígitos, 2 casas fixas; `3v2-4` = 2 a 4 casas).
- **Status** por grupo: `builder` (XML gerado pelo repo: **ausente em todos**) e `ordem` (tag presente na tabela
  `xsdorder` do go-dfe, em `arquivo:linha`). Contagens são de elementos (atributos e marcadores de escolha
  excluídos), extraídas do Leiaute do MOC.
- A lista completa tag a tag (1363 linhas, com ocorrência, tipo, tamanho, expressão regular e domínio) é a
  entrada da Fase 0 (seção 7.4). Aqui está o resumo por grupo e as exceções relevantes.

### 2.2 CT-e de carga, modelo 57 (463 elementos; ordem em xsdorder: até 261)

| Grupo | Elem. | Ocorr. | Pontos de tipo/tamanho/decimais que exigem regra | Builder | Ordem (`table.go`) |
|---|---|---|---|---|---|
| `infCte` (`versao` 2v2 ER56, `Id` 47 = "CTe"+44) | 3 | 1-1 | `Id` deve bater com a chave (rej 227) | ausente | linha 458 |
| `ide` | 63 | 1-1 | `cUF` 2; `cCT` 8 dígitos; `CFOP` 4; `natOp` 1-60; `mod` 57; `serie` 1-3; `nCT` 1-9; `dhEmi` 21 (UTC com TZ); `tpImp` 1-2; `tpEmis` 1,2,3,4,5,7,8; `cDV` 1; `procEmi` 0; `verProc` 1-20; `indGlobalizado` 0-1; `cMunEnv` 7; `xMunEnv` 2-60; `modal` 01-06; `tpServ` 0-4 (D21); `retira` 0/1; `xDetRetira` 1-160; `indIEToma` 1,2,9; `toma3`/`toma4` (escolha); `dhCont` 21 e `xJust` 15-256 (só `tpEmis` 2); `gCompraGov` (`tpEnteGov`, `pRedutor` 3v2-4, `tpOperGov`, `refDFeAnt` 0-n x 44); `tpPagAnt`; `gPagAntecipado/chDFePagAnt` 1-99 x 44 | ausente | 465-475: **parcial e desatualizada** (faltam `retira`, `xDetRetira`, `gCompraGov`, `tpPagAnt`, `gPagAntecipado`; ordem de `UFIni..` e `indIEToma` divergente; `dhIniViagem`/`indCarga` não existem no 57) |
| `ide/toma4` | 17 | 0-1 | `CNPJ` 14 \| `CPF` 11; `IE` 14; `xNome` 2-60; `fone` 6-14; `enderToma` (`xLgr` 2-255, `nro` 1-60, `CEP` 8, `cPais` 1-4); `email` 1-60 ER52 | ausente | **ausente** (`toma3`, `toma4`, `enderToma` sem entrada) |
| `compl` | 36 | 0-1 | `xCaracAd` 1-15; `xCaracSer` 1-30; `xEmi` 1-20; `fluxo` (`xOrig` 1-60, `pass` 0-n `xPass` 1-15, `xDest`, `xRota` 1-10); `Entrega` (6 variantes de escolha com `tpPer`, `dProg`, `dIni`, `dFim`, `hProg`, `hIni`, `hFim`); `origCalc`/`destCalc` 2-40; `xObs` 1-2000; `ObsCont`/`ObsFisco` 0-10 (`xCampo` 1-20, `xTexto` 1-160/1-60) | ausente | 9 de 36 (linha 487-489); `fluxo`, `Entrega`, `origCalc`, `destCalc` ausentes |
| `emit` | 19 | 1-1 | `CNPJ` 14 (alfanumérico pelo MOC 2.3.1) \| `CPF`; `IE` 14; `IEST` 14; `xNome` 2-60; `enderEmit` (`CEP` 8, `fone` 6-14); `CRT` 1; `ISUFEmit` 8-9 | ausente | 18 de 19 (linha 477; falta `ISUFEmit`) |
| `rem` | 19 | 0-1 | idem tomador; `enderReme` com `cPais`/`xPais`; `email` | ausente | 8 de 19 (`enderReme` e `email` sem entrada) |
| `exped` | 18 | 0-1 | idem, sem `xFant`; `enderExped` | ausente | 7 de 18 |
| `receb` | 18 | 0-1 | idem; `enderReceb` | ausente | 7 de 18 |
| `dest` | 19 | 0-1 | `ISUF` 8-9; `enderDest` | ausente | 17 de 19 (linha 483; faltam `ISUF`, `email`) |
| `vPrest` | 7 | 1-1 | `vTPrest` 13v2; `vTPrestLiq` 13v2 (novo); `vRec` 13v2; `Comp` 0-n (`xNome` 1-15, `vComp` 13v2) | ausente | 6 de 7 (falta `vTPrestLiq`) |
| `imp/ICMS` (7 CSTs: `ICMS00`, `20`, `45`, `60`, `90`, `OutraUF`, `SN`) | 38 | 1-1 | `CST` 2; `pRedBC` 3v2; `vBC` 13v2; `pICMS` 3v2; `vICMS` 13v2; `vBCSTRet`, `vICMSSTRet`, `pICMSSTRet`, `vCred`; `vICMSDeson` e `cBenef` 10 (Simplificado); `indSN`; `gICMSPrevistoPagtoAntecip/vICMSPrevisto` | ausente | 18 de 38 (`ICMS45`, `ICMSOutraUF`, `ICMSSN`, `gICMSPrevisto...` ausentes) |
| `imp/ICMSUFFim` | 8 | 0-1 | `vBCUFFim`, `pFCPUFFim` 3v2, `pICMSUFFim`, `pICMSInter`, `vFCPUFFim`, `vICMSUFFim`, `vICMSUFIni` | ausente | 1 de 8 |
| `imp/vTotTrib`, `infAdFisco`, `vTotDFe` | 3 | 0-1 | `vTotTrib` 13v2; `infAdFisco` 1-2000; `vTotDFe` 13v2 (novo) | ausente | 2 de 3 |
| `imp/IBSCBS` (reforma) | 65 | 0-1 | `CST` 3; `cClassTrib` 6; `indDoacao`; `gIBSCBS` (`vBC` 13v2; `gIBSUF`/`gIBSMun`/`gCBS` com `pIBSUF` 3v2/3v4, `gDif`, `gDevTrib`, `gRed`, `vIBSUF`; `gALCZFMCBS`; `gTribRegular`; `gTribCompraGov`); `gEstornoCred` | ausente | marcado 64/65 mas é **colisão com a chave `IBSCBS` da NF-e**; ordem do CT-e não verificada |
| `infCTeNorm/infCarga` | 9 | 1-1 | `vCarga` 13v2 (obrig. salvo dutoviário, rej 581); `proPred` 1-60; `xOutCat` 1-30; `infQ` 1-n (`cUnid` 2, `tpMed` 1-20, `qCarga` 11v4); `vCargaAverb` 13v2 | ausente | **ausente** |
| `infCTeNorm/infDoc` (`infNF`, `infNFe`, `infOutros`, `infDCe`) | 85 | 0-1 | `infNF`: `mod` 2, `serie` 1-3, `nDoc` 1-20, `dEmi` D, valores 13v2, `nCFOP` 4, `nPeso` 12v3, `PIN` 2-9; `infNFe/chave` 44; `infOutros` (`tpDoc` 2, `descOutros` 1-100); `infUnidTransp`/`infUnidCarga` com `lacre` 1-20, `qtdRat` 3v2/3v3; **máx. 2000 documentos** (rej 601) | ausente | 48 de 85 (`infNFe`, `infNF`, `infOutros`, `infDCe` e unidades sem entrada própria) |
| `infCTeNorm/docAnt` (`emiDocAnt`, `idDocAntPap`, `idDocAntEle`) | 16 | 0-1 | `CNPJ`/`CPF`, `IE`, `UF`, `xNome` 1-60; `idDocAntPap` (`tpDoc` 2, `serie`, `subser`, `nDoc` 1-30, `dEmi`); `idDocAntEle/chCTe` 44 | ausente | **ausente** |
| `infCTeNorm/infModal` | 2 | 1-1 | `versaoModal` 4 ER48; conteúdo `xs:any` = grupo do modal (seção 2.7) | ausente | **ausente** |
| `infCTeNorm/veicNovos` | 7 | 0-n | `chassi` 17; `cCor` 1-4; `xCor` 1-40; `cMod` 1-6; `vUnit`/`vFrete` 13v2 | ausente | **ausente** |
| `infCTeNorm/cobr` (`fat`, `dup`) | 10 | 0-1 | `nFat` 1-60; `vOrig`/`vDesc`/`vLiq` 13v2; `dup` 0-n | ausente | 9 de 10 (a ordem de `cobr` vem da NF-e) |
| `infCTeNorm/infCteSub` | 3 | 0-1 | `chCte` 44; `indAlteraToma` 0-1 | ausente | **ausente** |
| `infCTeNorm/infGlobalizado` | 2 | 0-1 | `xObs` 15-256 | ausente | **ausente** |
| `infCTeNorm/infServVinc/infCTeMultimodal` | 3 | 0-1 / 1-n | `chCTeMultimodal` 44 | ausente | **ausente** |
| `infCteComp` | 2 | 1-10 (tipo 1) | `chCTe` 44 | ausente | 1 de 2 (nome `infCteComp` ok; `chCTe` sem lista) |
| `autXML` | 3 | 0-10 | `CNPJ` 14 \| `CPF` 11 | ausente | completo |
| `infRespTec` | 5 | 0-1 | `CNPJ`, `xContato` 2-60, `email`, `fone` 7-12, `idCSRT` 3, `hashCSRT` 20 | ausente | completo (`table.go:696`) |
| `infSolicNFF`, `infPAA`, `pgtoVinc` (NT 2026.001) | ~14 | 0-1 | `xSolic` 2-2000 (8000 no Simplificado); PAA (`CNPJPAA`, `PAASignature`); `pgtoVinc/pgto` 1-99 (`nPag` 3, `idTransacao` 2-35, `tpMeioPgto` 2, `CNPJReceb` 14, `CNPJBasePSP` 8) | ausente | `infSolicNFF` presente; `infPAA`, `pgtoVinc` ausentes |
| `infCTeSupl/qrCodCTe` | 1 | 0-1 (obrig. por regra 850) | 50-1000 caracteres, ER57; URL do portal + `chCTe` + `tpAmb` (+ `sign` em contingência offline) | ausente | **ausente** (`infCteSupl` com T minúsculo) |
| `Signature` (xmldsig) | n/a | 1-1 | enveloped + C14N, SHA-1/RSA conforme MOC 6.6 | go-dfe assina (`signer.go`) | n/a |

### 2.3 CT-e Simplificado, modelo 57 `CTeSimp` (310 elementos; ordem até 202)

Diferenças estruturais frente ao 57 normal: sem `rem`/`exped`/`receb`/`dest`; `toma` obrigatório com `indIEToma`,
`ISUF` e endereço; `infCarga` no nível de `infCte`; **`det` 1-999** (cada item com `nItem` 3, `cMunIni..xMunFim`,
`vPrest`, `vPrestLiq`, `vRec`, `Comp`, `infNFe` 1-n com unidades, `infDocAnt` 1-n com `infNFeTranspParcial`, e
`IBSCBS` por item); `total` (`vTPrest`, `vTPrestLiq`, `vTRec`, `IBSCBSTot` com 18 campos, `vTotDFe`); ICMS com
`vICMSDeson` e `cBenef` obrigatórios por CST; `infSolicNFF/xSolic` até 8000.
**Status:** builder ausente; `toma`, `infCarga`, `infModal`, `total/IBSCBSTot` sem entrada em `xsdorder`;
`det` tem 82 de 103 por colisão com `det` da NF-e (ordem **provavelmente errada**, pois `det` da NF-e tem
`prod`/`imposto`).

### 2.4 CT-e OS, modelo 67 (256 elementos; ordem até 170)

Diferenças: `ide/infPercurso` 0-25 (`UFPer`); `toma` (grupo próprio, 0-1) em vez de toma3/toma4; `imp/infTribFed`
(`vPIS`, `vCOFINS`, `vIR`, `vINSS`, `vCSLL`, 13v2); `infCTeNorm/infServico` (`xDescServ` 1-30, `infQ/qCarga` 11v4);
`infDocRef` 0-n (`nDoc`, `serie`, `subserie`, `dEmi`, `vDoc`, `chBPe` 44); `seg` 0-n; `refCTeCanc` 44; `infGTVe` 0-n
(`chCTe`, `Comp` com `tpComp`, `vComp`, `xComp`); modal único `rodoOS` (`TAF` 12 \| `NroRegEstadual` 25, `veic`
com `placa` 7, `RENAVAM` 9-11, `prop`, `UF`; `infFretamento` com `tpFretamento`, `dhViagem`). **Status:** builder
ausente; `toma`, `infPercurso`, `infTribFed`, `infServico`, `infDocRef`, `infGTVe`, `rodoOS` sem entrada em
`xsdorder`; `ResolveConfig` assina `infCTeOS` (provável erro).

### 2.5 GTV-e, modelo 64 (152 elementos; ordem até 83)

Estrutura própria: `ide/dhSaidaOrig`, `ide/dhChegadaDest`, `ide/toma` ou `ide/tomaTerceiro` (escolha), sem
`vPrest`/`imp`/`infCTeNorm`; `rem`, `dest`, `origem`/`destino` (`xLgr` 2-60, `nro`, `xBairro`, `cMun`, `UF`, `fone`),
`detGTV` (`infEspecie` 1-n com `tpEspecie`, `vEspecie` 13v2, `tpNumerario`, `xMoedaEstr` 2-60; `qCarga` 11v4;
`infVeiculo` 1-n com `placa`, `UF`, `RNTRC` 8), `autXML`, `infRespTec`, `infCTeSupl`. **Status:** builder, `origem`,
`destino`, `detGTV`, `tomaTerceiro` ausentes. Atenção: no CT-e OS a GTV-e é referenciada (`infGTVe`) quando
`tpServ` é transporte de valores.

### 2.6 Eventos e grupos auxiliares do leiaute

| Item | Estado |
|---|---|
| `evTributacaoSN` (412140, gerado pelo Fisco; `gTribItemSN` 1-990, `gTotalSN`) e `evCancTribSN` (412141) | só chegam por distribuição; precisa de parser/consulta (ver seção 6) |
| Eventos do emitente, ver tabela C do item 6.3 | todos ausentes (só `evCancCTe` tem ordem em `table.go:517-518`) |

### 2.7 Grupos por modal (`infModal/versaoModal`, versão 4.00)

| Modal (`modal`) | Grupo | Elem. | Conteúdo | Builder | Ordem |
|---|---|---|---|---|---|
| 01 Rodoviário | `rodo` | 12 | `RNTRC` 8 (ER44); `occ` 0-10 (`serie`, `nOcc` 1-6, `dEmi`, `emiOcc`: `CNPJ`, `cInt` 1-10, `IE`, `UF`, `fone`) | ausente | ausente (MDF-e tem `rodo` diferente) |
| 02 Aéreo | `aereo` | 17 | `nMinu` 9; `nOCA` 11; `dPrevAereo`; `natCarga` (`xDime` 5-14, `cInfManu` 0-n); `tarifa` (`CL` 1, `cTar` 1-4, `vTar` 13v2); `peri` 0-n (`nONU` 4, `qTotEmb`, `infTotAP`: `qTotProd` 11v4, `uniAP` 1) | ausente | 8 de 16 por colisão com MDF-e |
| 03 Aquaviário | `aquav` | 22 | `vPrest`, `vAFRMM` 13v2; `xNavio` 1-60; `balsa` 0-3; `nViag` 1-10; `direc`; `irin` 1-10; `detCont` 0-n (`nCont`, `lacre` 0-3, `infDoc`: `infNF`, `infNFe` com `unidRat`); `tpNav` | ausente | 10 de 21 por colisão |
| 04 Ferroviário | `ferrov` | 5 | `tpTraf`; `trafMut` (`respFat`, `ferrEmi`); **sem `fluxo`/`vagao` no 4.00** | ausente | 2 de 4 |
| 05 Dutoviário | `duto` | 9 | `vTar` 9v6 (ER20); `dIni`, `dFim`; `classDuto`; `tpContratacao`; `codPontoEntrada`/`codPontoSaida` 20; `nContrato` 20 | ausente | ausente |
| 06 Multimodal | `multimodal` | 9 | `COTM` 1-20; `indNegociavel`; `seg/infSeg` (`xSeg` 1-30, `CNPJ`); `nApol`; `nAver` | ausente | 5 de 8 |
| CT-e OS rodoviário | `rodoOS` | 19 | ver 2.4 | ausente | 8 de 18 |

Para os 6 modais o repositório já tem dados parciais reutilizáveis: veículos (`organization_vehicles`), conjuntos
de veículos, seguros, `reference-documents`, `cargo-units`, `toll-providers` (planos 2026-07-11 e 2026-08-26 e
`api/internal/services/mdfes/modals.go`); falta mapeá-los para o leiaute do CT-e.

### 2.8 Entidades novas recomendadas (nível da régua do plano 2026-08-26)

| Dado | Nível | Onde |
|---|---|---|
| `tpServ`, `modal`, `tpCTe`, `indIEToma`, `retira`, componentes de `vPrest` (`Comp`), CFOP de transporte, CST de ICMS/IBSCBS | 2 e 3 | operação + `tax_profiles` de transporte (reusar `organization_operations`) |
| Unidade de carga/transporte (`infUnidCarga`, `lacre`) | 6 | `cargo-units` (já existe) |
| Percurso/rota (`fluxo`, `infPercurso`, `origCalc/destCalc`) | 6 | `service_locations` (já existe) + novo `routes` |
| Seguro (`seg`, `infSeg`, `nApol`, `nAver`) | 6 | `insurance-policies` (já existe) |
| Documento anterior (`docAnt`, redespacho) | 7 | request (referência a CT-e de `ctes`) |
| Veículos novos (`veicNovos`), `infGlobalizado`, `infServVinc` | 7 | request |
| Espécies/numerário da GTV-e (`detGTV`) | 7 + cadastro | novo cadastro de numerário/moeda |
| Terminal/porto e dados ANTAQ/ANAC | 6 | reaproveitar `reference-documents` e estender `service_locations` |

---

## 3. Matriz B: regras do MOC sem pré-validação local (por prioridade)

Hoje **nenhuma** regra do CT-e é pré-validada (não há builder). Fora o validador de CPF/CNPJ
(`api/internal/validation/validators.go:126,159`), UF (`:90`) e dígito da chave de acesso (`access_key.go:98`,
restrito a mod 55), o que existe é reaproveitável e está em outras camadas. Lista por **código de rejeição (cStat)**
e regra do MOC (`Gxxx` do modelo 57, `Hxxx` do 67, `Ixxx` do 64). Totais lidos: 619 regras no MOC.

### 3.1 P0: verificáveis offline, sem base da SEFAZ (implementar no validador genérico)

| Tema | cStat (regra) | O que checar localmente |
|---|---|---|
| Formato e tipo de cada tag | 215 (schema) | tipo, tamanho, casas decimais, regex, enum, ocorrência, escolha (a base do validador, seção 7) |
| Chave e `Id` | 227 (G014), 253 (G015), 732 (G013, mod 57), 721 (H010, mod 67) | `Id="CTe"+chave`; DV mod 11 (inclui CNPJ alfanumérico, MOC 2.3.1); mod da chave |
| Ambiente e série | 252 (G001), 670 (G006) | `tpAmb`; série fora de 890-899 (exceto NFF) |
| UF e município | 247 (G007), 712/713 (G112/G113), 418/532, 424/533, 431/534, 439/535, 447/536, 493/537, 456/541, 414/542 | UF do emitente x chave; 2 primeiros dígitos do município x UF; município existe na tabela IBGE |
| Emissão e contingência | 586/587/937/588 (G008, G008b, G009, G010), 516/517/515 (G011, G012, H007, H008) | `dhCont`/`xJust` só em `tpEmis=2`; `dhCont <= dhEmi`; `tpEmis` 7/8 só no SVC; SVC só tipo normal |
| Datas | 212 (G102, tolerância 5 min), 228 (G103, 168 h) | `dhEmi` vs relógio; atraso (exceto `tpEmis=2`) |
| Documentos das partes | 207/208/237/415/416/428/429/436/437/444/445/699/700 | CNPJ/CPF com DV, sem zeros ou repetidos; IE com DV por UF (209/210/419/432/440/448/614: **precisa tabela de algoritmos de IE por UF**) |
| Estrutura x `tpCTe`/`tpServ` | 458/459 (G016/G017), 503/505 (G076/G077), 460-463 (G018-G021), 469/470/474/475 (G053-G056), 693/694 (G036/G037), 521 (G059), 747 (G060), 651/814 (G057/G058), 579-584 (G025-G031) | grupos obrigatórios/vedados conforme tipo de CT-e, tipo de serviço, modal e tomador |
| Tomador | 481/482/617 (G022-G024) | `indIEToma` x IE; lista de UFs que vedam isento |
| Documentos transportados | 601 (G038/G045a, máx. 2000), 527/375 (duplicidade), 843/376 (chave válida), 930 (ano/mês), 540 (G044) | chaves NF-e/DC-e válidas, sem duplicata, lote <= 2000 |
| Valores | 650 (G046, R$ 9.999.999,99), 675 (G047, ICMS = BC x alíquota com tolerância), 531 (G048, `vRec <= vTPrest`), 899 (H053, soma GTV-e), 760 (H057, INSS) | cálculo e limites |
| CFOP | 676 (G049), 519 (G050), 524/908 (G051/G052) | CFOP na lista 3.1 do MOC; 5/6/7 conforme UFs; 5932/6932 |
| Globalizado | 743, 722, 723, 724, 744, 725-730, 737 (G197-G208) | UF início = fim; tomador rem/dest; `xNome` "DIVERSOS"; CNPJ do emitente nas pontas |
| Tipo de serviço no CT-e OS | 751/752/380 (H019-H020a), 753, 757, 798, 829, 837-841 (fretamento), 754, 881/927 (infGTVe), 898, 899 | regras de pessoas, valores, excesso de bagagem, transporte de valores |
| Homologação | 646-649 (G002-G005), 725/726 | literal "CTE EMITIDO EM AMBIENTE DE HOMOLOGACAO - SEM VALOR FISCAL" em `xNome` |
| autXML, RT | 699, 700, 715, 828, 836 | sem duplicidade, sem repetir ator, CNPJ do RT |
| QR Code | 850-855 (G212-G217, I70-I75) | `infCTeSupl` presente; URL e parâmetros; `sign` só em `tpEmis=2` |
| IBS/CBS (3.6, 3.11) | 310-314, 388 e demais | existência de CST e `cClassTrib` nas tabelas 4.1-4.2, compatibilidade CST x classe, grupos exigidos/vedados por CST, soma de totais (`IBSCBSTot`) |
| Simples Nacional (1.1, 3.12) | regras 2026 | tabela de `tpRegimeIBSCBS` x CRT x grupos (já tabelada no MOC 1.1) |

### 3.2 P1: dependem de dados que o próprio produto já tem ou pode consultar (via egress)

| Tema | cStat | Como |
|---|---|---|
| Duplicidade de número e chave | 204 (G158), 539 (G157), 218 (G159) | reserva de numeração atômica (`transact_write`) já existente para NF-e; tabela `ctes` |
| Substituição | 568-571, 659, 739, 510-512, 550/551, 553, 559/560, 734, 834, 563 (G078-G096, H059-H075) | ler o CT-e substituído em `ctes` ou consultar `CTeConsulta` via egress; exige evento de prestação em desacordo (610110) |
| Complemento | 267, 269, 491, 520, 655, 657, 671, 800-805, 811/812, 835, 907 (G160-G177) | idem; máximo de complementos; mesma partes e UFs |
| Alteração de tomador | 552, 738-742 (G097-G101) | comparar com o CT-e substituído |
| Redespacho/multimodal/subcontratação | 543, 667, 690-692, 709-711, 733, 745, 746, 748-750, 832, 844 | consulta de chaves anteriores e do OTM |
| Referências a CT-e/GTV-e/BP-e/NF-e | 661, 662, 652 (NF-e), 824-827, 883-887, 892-897 | existência, não cancelada, mesmo emitente |
| Cadastro de contribuinte (IE ativa, CCC) | 230, 231, 203, 421/422, 426/427, 232, 716-719, 799 | só a SEFAZ responde; usar `NFeConsultaCadastro` como aviso (não bloqueante) |

### 3.3 P2: infraestrutura da SEFAZ ou já cobertas pelo transporte

Grupos A a E (certificado TLS e de assinatura, 280-286, descompactação, `B-0`, assinatura XML 213-220 etc.): o
go-dfe já faz mTLS e assinatura; falta apenas **checagem preventiva local** de validade do PFX, CNPJ no certificado e
`KeyUsage` (reusar o gerenciador de certificados). Regras revogadas (EPEC G184-G192, FS-DA G008a/H004a, PAA
G111a-c) **não** devem ser implementadas.

---

## 4. Matriz C: contingência

| `tpEmis` | Significado no MOC 2026 | Estado no repo | Falta |
|---|---|---|---|
| 1 Normal | emissão normal | nenhum (não emite) | tudo (fase 2) |
| 2 Contingência offline | **novo e imediato**; transmitir em até 168 h; `dhCont` e `xJust` obrigatórios; QR Code com `sign` (assinatura da chave com o certificado do emitente) | NF-e já tem `EmissionMode`, `dhCont`, `xJust` (C0, `nfes/emission_mode.go`); CT-e: nada | `EmissionMode` do CT-e; chave com `tpEmis=2`; QR Code assinado; fila de reenvio; relógio das 168 h; cStat 150 (autorizado fora de prazo) como sucesso; reaproveitar a máquina de estados da fase C2 |
| 3 Regime Especial NFF | só SVRS; sem CNPJ/IE (`infSolicNFF`); certificado e-CNPJ da SVRS (A08-A10, 900-906) | nenhum | **fora de escopo para emissor próprio** (só SVRS emite); apenas rejeitar com mensagem clara |
| 4 EPEC pela SVC | **revogado**; CT-e com `tpEmis=4` ainda é recebido para conciliar eventos legados | nenhum | não implementar emissão; aceitar leitura na distribuição e rejeição 629 do evento 110113 |
| 5 FS-DA | **revogado**; schema deixou de aceitar | nenhum | não implementar |
| 7 SVC-RS / 8 SVC-SP | autorização no ambiente SVC conforme a UF; só `tpCTe=0`; ativação anunciada por `CTeStatusServico` (cStat 113/114) | nenhum; **sem endpoints SVC de CT-e** em `table.go` (nenhum `SVC` no arquivo); NF-e também pendente (plano 2026-08-26, C1 bloqueado) | transcrever do MOC/portal SVRS a tabela UF -> SVC-RS/SVC-SP (a pendência C1 do plano é a mesma); endpoints SVC por ambiente; detecção por `CTeStatusServico`; cancelar CT-e pendentes substituídos (MOC 4.1.3) |

Fluxos transversais que faltam (todos novos):
1. **Recuperação:** reenvio com mesma numeração e série após rejeição (MOC 4.1.2), sem alterar valor, partes ou data.
2. **Pendentes de retorno:** consulta (`CTeConsulta`) de CT-e sem retorno, cancelamento dos autorizados que foram
   substituídos por CT-e em contingência (4.1.3). Reusar `outbox`, `dlq-processor` e a consulta de recibo.
3. **Regra de numeração:** vedada a reutilização, em contingência, de número já transmitido como normal.
4. **Detecção automática e override manual** por organização e UF (decisão C2 do plano 2026-08-26): reutilizar,
   não duplicar.
5. **DACTE de contingência** (seção 5): texto "DACTE em Contingência" e QR Code com `sign`.
6. **Prazos:** 168 h de emissão e 60 dias para substituição (G096/H075).
7. Contingência de **evento**: eventos em SVC só 110111 (N09, cStat 530); EPEC não existe mais.

---

## 5. Matriz D: DACTE

| Formato (MOC seções 2.15-2.17, 4, 5) | Estado | Observação |
|---|---|---|
| DACTE A4 retrato (modal rodoviário, aéreo, aquaviário, ferroviário, dutoviário, multimodal, normal) | **feito (F8), rota `GET /v1.0/ctes/{k}/dacte`**: blocos comuns, RNTRC do rodoviário, quadros específicos de aéreo, aquaviário (contêiner), ferroviário (ferrovias envolvidas) e multimodal, previsão do fluxo; dutoviário só o título (Anexo: "a ser utilizado no futuro"); veículos novos, produto perigoso, canhoto, verso e folhas adicionais ainda não | existe `danfe_retrato.html` e `damdfe_retrato.html` (folio + gonja, `renderer.go`) como base de layout e macros |
| DACTE A4 paisagem | ausente | o MOC exige mesma disposição proporcional; códigos de barras não reduzem altura |
| DACTE A5 paisagem e Ofício II (230x330) | **A5 feito (F8)**: `format=a5` (A5 paisagem, mesmo arranjo do retrato, folhas adicionais pela paginação); Ofício II ausente; sem seletor na UI | tamanhos mínimo A5 (210x148) e máximo Ofício II; excedente vai na largura e no quadro de componentes/observações |
| DACTE de contingência (texto, QR Code com `sign`) | ausente | o modelo FS-DA do MOC está revogado; só offline (`tpEmis=2`) e SVC (impresso como normal) |
| DACTE OS (modal rodoviário OS; normal; contingência FS-DA revogada) | ausente | 3.2 do MOC correlaciona XML 67 com DACTE OS |
| **GTV-e (modelo 64)**: o MOC não traz leiaute de documento auxiliar próprio no capítulo DACTE | **lacuna do MOC**: GTV-e aparece só em QR Code e Leiaute. Recomenda-se um "DAGTVE" simples reutilizando o layout do DACTE OS, **depois de confirmar com a UF/NT** | pergunta aberta ao dono do produto |
| CT-e Simplificado (`CTeSimp`) | ausente | o DACTE do CT-e Simplificado deve vir de outro anexo (NT 2024.002); não está no capítulo DACTE lido |
| Elementos obrigatórios do DACTE | ausente | chave (CODE-128C, 44 caracteres, resolução mínima 300 dpi; **variante híbrida C/A para CNPJ alfanumérico**, 6.2), QR Code (mín. 25x25 mm, quiet zone 3 mm), protocolo, tarja de homologação "EMITIDO EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL", numeração de folhas (inclusive na 1ª), "CONTINUA NO VERSO", canhoto opcional, folhas adicionais para documentos originários e `docAnt`, quadros de veículo novo e produto perigoso, contêiner no aquaviário |
| Geração a partir do XML autorizado (cteProc) e armazenamento | parcial | `documents/service.go` e `signed_file.go` já fazem para NF-e/MDF-e; precisa de `documents/cte.go`, templates e testes golden |
| Barcode CODE-128 | reaproveitável | `documents/barcode.go` (verificar suporte a variante híbrida para CNPJ alfanumérico) |
| UI | ausente | botão de download, guia em `/guide` e captura (regras do `CLAUDE.md` do repo) |

---

## 6. Matriz E: divergências de estrutura

### 6.1 Ordem XSD (`xsdorder`)

Ver seção 0 e 2. Em resumo: nomes `infCTeNorm`, `infCTeSupl` e `infCte:ide` desatualizados; `enviCTe` obsoleta; faltam
tabelas para `infCTeNorm` e filhos (`infCarga`, `infDoc`, `docAnt`, `infModal`, `veicNovos`, `cobr`, `infCteSub`,
`infGlobalizado`, `infServVinc`), `toma3`/`toma4`/`toma`, `enderToma`/`enderReme`/`enderExped`/`enderReceb`, `imp/ICMS*`,
`IBSCBS` do CT-e, `total`, `det` do Simplificado, os 6 modais, o CT-e OS e a GTV-e. A tabela tem de ser
**regerada do MOC** e acompanhada de um teste que compara cada chave com o leiaute (Fase 0), em vez de
manter transcrição manual.

### 6.2 Namespaces, envelope e assinatura

| Tópico | MOC | Repo | Divergência |
|---|---|---|---|
| Namespace único no raiz, sem prefixo; `Signature` com `xmlns` próprio | 6.1 b, c | `builder.go` aceita `@xmlns` e `constants.go:114` usa `http://www.portalfiscal.inf.br/cte` | ok |
| Mensagem de emissão | `CTe`/`CTeOS`/`GTVe`/`CTeSimp` direto, gzip+base64, `cteDadosMsg` | gzip ok (`client.go:37`); `xsdorder` e docs ainda falam em `enviCTe` | remover `enviCTe` e documentar |
| Tag assinada | `infCte` nos 3 modelos | `config.go:93-96` usa `infCTeOS`, `infGTVe` | corrigir após conferir XSD |
| Versão do leiaute | `4.00` | `constants.go:99` | ok |
| SOAP | 1.2, sem SOAP Header | `soap/envelope.go` com header `cteCabecMsg` (`constants.go:128`) | **verificar**: o MOC eliminou o SOAP Header na 4.00a (Histórico); conferir se o envelope ainda envia `cteCabecMsg` |
| Assinatura | enveloped + C14N, SHA-1/RSA 1024+, sem `X509SubjectName` e similares | `signer.go` (port do cliente Python) | ok em princípio; sem teste byte a byte para CT-e (já documentado em `go-dfe/CLAUDE.md`) |
| Resposta síncrona | `retCTe` (`protCTe`), `retCTeOS`, `retGTVe`, `retCTeSimp` | `response.go` genérico | conferir nomes de nó por serviço em `cteResponseNodePath` (só tem distribuição) |
| Formatação | sem espaços entre tags, sem zeros à esquerda, tags opcionais vazias suprimidas | builder suprime nada por conta própria | tratar no validador/normalizador |

### 6.3 Eventos (SRE 4.00)

Mensagem única `eventoCTe` (síncrona, sem lote): `infEvento` com `Id = "ID"+tpEvento+chave+nSeqEvento(3)`,
`cOrgao`, `tpAmb`, `CNPJ|CPF`, `chCTe`, `dhEvento`, `tpEvento`, `nSeqEvento`, `detEvento` (+ `infSolicNFF`), `Signature`.
`table.go:510-518` cobre só o envelope e o cancelamento. A lista abaixo é o catálogo vivo do MOC (item 12.1):

| tpEvento | Evento | Modelos | Schema | Repo |
|---|---|---|---|---|
| 110111 | Cancelamento | 57, 67, 64 | `evCancCTe` | ordem em `table.go:517`; builder e regras ausentes |
| 110110 | Carta de Correção (CC-e) | 57, 67 | `evCCeCTe` (`infCorrecao` 1-n com `grupoAlterado`, `campoAlterado`, `valorAlterado`, `nroItemAlterado`) | ausente; **tabela de campos vedados na CC-e** (MOC 3.2.1 e 3.2.2) precisa virar tabela |
| 110160 | Registros do Multimodal | 57 | `evRegMultimodal` | ausente |
| 110180 / 110181 | Comprovante de Entrega / Cancelamento | 57 | `evCECTe` / `evCancCECTe` | ausente |
| 110190 / 110191 | Insucesso na Entrega / Cancelamento | 57 | `evIECTe` / `evCancIECTe` | ausente |
| 610110 / 610111 | Prestação de Serviço em Desacordo / Cancelamento | 57 | `evPrestDesacordo` / `evCancPrestDesacordo` | ausente (**pré-requisito da substituição**) |
| 110300 / 110301 | Vinculação / Cancelamento de pagamento | 57, 67 | `evVincPgto` / `evCancVincPGto` | ausente |
| 211110 | Apropriação do Crédito Presumido (tomador, TAC/MEI, SVRS) | 57 | `evApropriaCredPres` | ausente |
| 310610 / 310611 | MDF-e Autorizado / Cancelado (Fisco) | 57 | gerados pelo Ambiente Nacional | só leitura na distribuição; **regra 528 (Q15)**: não cancelar CT-e com MDF-e autorizado |
| 310620 / 510620 | Registro de Passagem (Fisco) | 57 | Fisco | leitura |
| 240130, 240131, 240140, 240170, 240180, 240181 | Fisco do emitente: complementar autorizado/cancelado, substituição, liberação de prazo, CT-e OS autorizado/cancelado | 57, 67, 64 | Fisco | leitura; 240170 libera cancelamento após prazo (GTV-e: 45 dias, regra 876) |
| 412140 / 412141 | Tributação SN / Cancelamento (Fisco) | todos | `evTributacaoSN`, `evCancTribSN` | leitura |
| 110113, 240160, 110170 | EPEC, Liberação de EPEC, Informações da GTV | | **revogados** (rej 629) | não implementar |

Lacunas adicionais em `xsdorder`: `infEvento` sem `infSolicNFF`; nenhum `det*` específico; `detEvento` com atributo
`versaoEvento`.

### 6.4 Inutilização, anulação e denegação

**Eliminadas no CT-e 4.00a** (Histórico de Implantação, linha 4.00a e NT 2023.001 v1.03). O repo **não deve** replicar
o fluxo de inutilização de NF-e para CT-e; não existe `CTeInutilizacao`. `tpCTe` aceita só 0 (normal), 1 (complemento)
e 3 (substituição). O valor 2 (anulação) não existe mais. **Ação:** registrar isso no plano e na UI; não criar
tela nem rota.

### 6.5 Distribuição e consulta

- `CTeDistribuicaoDFe` (síncrono, 3 meses, sem `consChCTe`): o repo pede `consChCTe` (`distribution.go:695`).
  **Corrigir** para `consNSU` ou remover; checar o impacto em quotas (`checkConsQuota`).
- Pacotes novos da distribuição: `procEventoCTe`, `resCTe`, `cteProc`, `cteOSProc`, `GTVeProc`, `cteSimpProc`: o parser
  já reconhece. Falta tratar o literal de 44 noves nas chaves de terceiros (`autXML`) e os eventos de Fisco
  (310610 etc.) na UI.
- `CTeConsultaProtocolo` e `CTeStatusServico` já têm serviços (`dfe.go:60-61`); falta a leitura estruturada do
  `cStat` 113/114 (SVC ativa) para disparar contingência.
- `NFeConsultaCadastro` é compartilhado (`dfe.go:37`).

---

## 7. Proposta F: validador genérico dirigido por tabela

### 7.1 Princípios

1. **Uma tabela, um motor.** A tabela descreve cada caminho XML (path, ocorrência, tipo, tamanho, decimais, regex,
   domínio, escolha, condição). O motor é um só e valida o mapa que entra em `xmlops.BuildXML` **e** o XML
   final (defesa em profundidade).
2. **Tabela gerada, não digitada.** Gerada do MOC (HTML do Leiaute) por uma ferramenta one-shot (`cmd/gen-moc-table`,
   que seria o único "Scan"-like permitido pelo `standards-check`; não faz parte de serviço) e conferida contra os
   XSDs quando houver pacote de liberação. Sem digitação manual = sem drift.
3. **Regras condicionais em código pequeno, nomeadas pelo cStat.** As ~220 regras de negócio condicionais (3.1) viram
   funções `func(doc) []Violation` registradas em tabela por `cStat`, cada uma com o código da regra do MOC
   (`G023`, `H055`...) e mensagem **idêntica à do MOC** (reusa a taxonomia de rejeições para a UI).
4. **Onde roda.** `go-dfe` (pacote `moc` ou `validate`, reutilizável por NF-e/MDF-e depois, e consumido pela
   `api` na emissão para devolver **RFC 7807** com `errors[]` por path, e pelo `worker` antes de chamar o egress).
   A chamada fiscal continua **sempre pelo `go-dfe-egress`**; o validador é puro, sem rede e sem scan.
5. **Resultado.** `[]Violation{Path, Rule, CStat, Kind, Got, Want, Message}`; a API traduz para `problem.Problem`
   com `errors` (não 500). Prioridade: erro de formato (`schema`) antes de regra de negócio.

### 7.2 Desenho da tabela (Go, `go-dfe/moc/tags_cte.go` gerado)

```go
// Tag descreve um elemento ou atributo do leiaute. Gerado do MOC; nao editar a mao.
type Tag struct {
    Path      string   // "infCte/ide/cMunIni" (relativo ao modelo)
    Kind      Kind     // Element, Attr, Group, Choice (escolha entre irmaos)
    Min, Max  int      // Max -1 = n; ocorrencia base (ex.: 0-1, 1-n)
    Type      DType    // C, N, D, T, E (dominio), Dec
    MinLen    int      // tamanho minimo (1 se fixo)
    MaxLen    int
    IntDigits int      // parte inteira (13v2 -> 13)
    MinDec    int      // casas decimais minimas (2 em 13v2; 2 e 4 em 3v2-4)
    MaxDec    int
    Regex     string   // ER do MOC (ER36, ER27...), nome e padrao (tabela ER)
    Domain    []string // D1.., enumerados
    Since     string   // NT/versao de entrada em vigor (para dhEmi >= data)
    Models    uint8    // bitmask 57, 67, 64, SIMP
    When      CondID   // referencia a uma condicao nomeada (ver 7.3)
}
```

Regras genéricas derivadas só da tabela (sem código por tag):

| Regra | Comportamento |
|---|---|
| Ocorrência | presente quando `Min=0` e vazio -> **suprimir** (MOC 6.1 d); `Min=1` ausente -> violação; `Max` excedido -> violação |
| Escolha (`CE`/`CG`) | exatamente um dos irmãos do grupo `Choice` |
| Tipo `C` | UTF-8, sem espaços nas pontas, sem quebra de linha, tamanho `MinLen..MaxLen` em runas, regex `ER36` (sem caracteres de controle e sem sequências proibidas) |
| Tipo `N` inteiro | só dígitos, sem zeros à esquerda quando não for código fixo, tamanho exato ou faixa |
| Tipo `Dec` | `shopspring/decimal`, ponto decimal, parte inteira `IntDigits`, **casas entre `MinDec` e `MaxDec`**, sem notação científica; normaliza (`"10.5"` vira `"10.50"` quando `13v2`) e valida |
| Data `D` / `T` / UTC | regex `ER1` (`AAAA-MM-DDThh:mm:ssTZD`), faixa -11..+12, data real |
| Domínio | pertence à lista; tabelas externas (IBGE, CFOP 3.1, cClassTrib, CST, meios de pagamento) via `Domain` carregado de tabela, não de literal |
| Condicional | `When` aponta para função booleana sobre o documento (ex.: `tpEmis==2`, `tpServ in {3,4}`) |
| Normalização | remove tags vazias, não escreve zeros à esquerda, corta espaços, impede float (`BuildXML` só recebe string já normalizada) |
| Ordem | **a mesma tabela gera o `xsdorder`** do CT-e (fim da transcrição manual), com teste que falha se a ordem de qualquer irmão diverge do MOC |

### 7.3 Regras de negócio

`RuleSet` por modelo (57, 67, 64, SIMP), cada regra com `ID` (`G023`), `CStat`, `Effect` (rejeição ou aviso),
`Applies(doc)`, `Check(doc)`. Duas famílias:

- **Local-pura (P0):** depende só do documento e de tabelas estáticas (IBGE, CFOP, IE por UF, cClassTrib/CST).
- **Contextual (P1):** recebe um `Lookup` (interface) que a API implementa lendo `ctes` e, se preciso, chamando a
  consulta via `go-dfe-egress`; sem `Lookup`, a regra emite **aviso**, não bloqueio.

### 7.4 Fases, entregas e estimativa

Estimativas em dias úteis de uma pessoa sênior, com testes e docs (a política do repo exige guia, DOCS.md e testes de
integração). T = pequeno (1-3 d), M = médio (4-8 d), G = grande (9-15 d).

| Fase | Entrega | Tamanho | Depende |
|---|---|---|---|
| **0. Fundação** | corrigir `xsdorder` (nomes, `enviCTe`), chave mod 57/67/64 em `ValidAccessKey`, `config.go` das tags assinadas (conferir XSD), decimais no builder, gerador `gen-moc-table` do Leiaute para `Tag[]` e para `xsdorder`, teste de paridade com o MOC | M | pacote de liberação XSD (confirmar) |
| **1. Motor de validação** | pacote `moc` com `Tag`, tipos genéricos, `Violation`, integração RFC 7807, ~120 testes de tipo/tamanho/decimais; cobre 100% das tags só em forma | M | 0 |
| **2. CT-e 57 modal rodoviário** | modelo de dados (operação de transporte, componentes, tax profile de transporte), `ctes/` builder (ide, partes, vPrest, imp ICMS, infCarga, infDoc, rodo), emit com numeração e outbox, worker, DocumentRepository, rotas `/ctes`, UI de emissão, `cteProc` salvo no S3 | G | 1 |
| **3. Regras P0 do 57** | ~140 regras locais (seção 3.1), com testes por código G; tabelas IBGE, CFOP, IE por UF | G | 2 |
| **4. Demais modais e grupos do 57** | aéreo, aquaviário, ferroviário, dutoviário, multimodal; `veicNovos`, `docAnt`, `infGlobalizado`, `infServVinc`, `cobr`, `compl`; complementar e substituição (precisa evento 610110); IBS/CBS e Simples Nacional (grupo `IBSCBS`, `gTribRegular`, `gEstornoCred`); `total` | G | 3 |
| **5. CT-e OS, GTV-e, Simplificado** | builders e regras H e I e 3.3; CT-e OS rodoviário; vínculo `infGTVe` | G | 4 |
| **6. Eventos** | cancelamento, CC-e (tabela de campos vedados), desacordo, comprovante de entrega e cancelamentos, insucesso, vinculação de pagamento, multimodal, apropriação de crédito presumido; leitura de eventos do Fisco; regras P1 (substituição, complemento) | G | 4 |
| **7. Contingência** | `EmissionMode` do CT-e, `tpEmis` 2/7/8 na chave, QR Code com `sign`, tabela UF -> SVC, endpoints SVC, detecção por `CTeStatusServico`, reenvio e pendentes (reaproveita C2 do plano 2026-08-26) | M | 2 |
| **8. DACTE** | `documents/cte.go`, templates retrato e paisagem A4/A5/Ofício, DACTE OS, definição do documento auxiliar da GTV-e e do Simplificado, CODE-128C híbrido, QR Code, contingência, tarja de homologação, guia e capturas, testes golden | G | 2 (iterar nas fases 4 e 5) |

Total: ~30 a 45 dias úteis (uma pessoa), paralelizável em 2 pessoas após a Fase 2 (validação/regras x eventos/DACTE).
Menor caminho para valor: Fases 0, 1, 2 e 3 (CT-e rodoviário completo e validado, ~15 a 22 dias).

### 7.5 Aderência ao CTech standards

- **Sem scan:** regras P1 usam chave de acesso e `sweep-index`; a ferramenta geradora é one-shot, listada com `standards:allow`.
- **Constantes nomeadas:** `cStat` e IDs de regra como constantes geradas; nada de literal solto.
- **RFC 7807:** violações viram `problem.BadRequest` com `errors[]` (path, regra, cStat, mensagem MOC).
- **Fiscal via egress:** o validador roda antes do envio; nenhuma chamada SEFAZ é feita dentro dele.
- **Documentação:** DOCS.md (endpoints), CONDUCT.md (decisão de tabela gerada), guia da UI, e `go-dfe/CLAUDE.md`
  (a decisão "go-dfe não valida schema" precisa ser revisitada: aqui é validação **estrutural local**, não XSD).
- **Texto visível sem travessão:** mensagens do MOC com travessão devem ser normalizadas pelo gerador.
- **Família:** o motor `moc` serve também NF-e e MDF-e (cobertura de tags do plano 2026-08-26 e regras da NF-e já
  têm cStat); avaliar subir para o `ctech-go-common` se houver uso em outro serviço.

---

## 8. Riscos e perguntas em aberto

1. **XSD oficial:** sem o pacote de liberação não há como conferir `infCTeOS` x `infCte`, ordem exata e a tabela
   gerada; obter o `PL_CTe_400_NT2026.xxx` antes da Fase 0 (o `ROADMAP.md:163` cita o caminho esperado).
2. **SOAP Header:** confirmar se o envelope atual ainda envia `cteCabecMsg` em v4.00a.
3. **DACTE da GTV-e e do Simplificado:** o capítulo DACTE do MOC não os cobre; definir com o dono do produto e confirmar
   com a UF de teste.
4. **Algoritmos de IE por UF e tabela IBGE de municípios:** precisam de fonte versionada (não há hoje no repo); avaliar
   um pacote compartilhado no `ctech-go-common`.
5. **Escopo contingência:** `tpEmis=3` (NFF) não é emissão própria; `tpEmis=4`/`5` são revogados. Confirmar a decisão.
6. **Datas de vigência:** IBS/CBS e Simples Nacional têm datas de homologação/produção em 2026 (ex.: SN 26/10/2026 e
   16/11/2026; regras 14/12/2026); o campo `Since` da tabela deve guardar essas datas para gating por `dhEmi`.

## 9. Arquivos citados

- `go-dfe/dfe.go`, `go-dfe/internal/services/{client,config,response}.go`, `go-dfe/internal/xmlops/{builder,processor,signer}.go`
- `go-dfe/internal/xmlops/xsdorder/table.go`, `go-dfe/internal/endpoints/table.go`, `go-dfe/internal/constants/constants.go`
- `api/internal/api/v1/organizations.go`, `api/internal/services/{fiscal_configs,vehicle_requirements}.go`
- `api/internal/validation/{access_key,validators}.go`, `api/internal/services/documents/`
- `worker/internal/service/{dfe,distribution,distribution_parser}.go`, `cdk/lib/worker-definitions.ts`
- `ui/src/app/cte/page.tsx`, `ui/src/components/layout/MobileNav.tsx`
- `docs/plans/2026-08-26-cobertura-total-tags-nfe-mdfe.md`, `docs/plans/2026-08-26-contingencia-e-inutilizacao-dfe.md`

## Status F9 (eventos), 2026-11-07

Feitos: 110180, 110181, 110190, 110191 (`api/internal/services/ctes/delivery_events.go`). 110300/110301 (`api/internal/services/ctes/payment_events.go`, layout dos XSDs vendorizados; autor, autorizador, prazo e a lista de tpMeioPgto não estão em documento disponível: perguntas em aberto abaixo). Pendentes: 110160, 211110.

Perguntas em aberto do 110300/110301: (1) quem pode registrar (emitente, tomador, outro)? (2) vai ao autorizador do CT-e ou à SVRS, como o 110300 da NF-e? (3) há prazo ou regra de unicidade de nPag/idTransacao entre eventos? (4) a lista de tpMeioPgto é a tPag da NF-e? (5) o 110301 só cancela o 110300 indicado em nProtVincPgto (o XSD diz assim)? Documentos que respondem: MOC CT-e (RecepcaoEventosEspecificos e RegrasDeValidacao) e a IT DFe 2026.001.
MDF-e: o MOC lista como eventos do emitente apenas 110111, 110112, 110114, 110115, 110116, 110117 e 110118, todos já implementados.

## Status F6 parte 3 (complemento, substituto, CT-e OS), 2026-11-08

Feitos: CT-e complementar (tpCTe 1), substituto (tpCTe 3) e o núcleo do CT-e OS; contratos em
`docs/specs/2026-11-05-cte-emissao-rodoviaria.md` e regras provadas localmente em `CONDUCT.md`. Correção de numeração: no
MOC publicado as regras do complemento são G017 e G160 a G177 e as da substituição G076 a G101 (G078 a G096 são da
substituição). tpCTe 2 (anulação) não existe no 4.00.

### GTV-e, modelo 64 (`TGTVe` em cteTiposBasico_v4.00.xsd; só spec, sem código)

Raiz `<GTVe versao>` com `infCte (Id, versao)` e `infCTeSupl/qrCodCTe`. Ordem: `ide`, `compl`, `emit`, `rem`, `dest`,
`origem`, `destino`, `detGTV`, `autXML`, `infRespTec`. `ide`: cUF, cCT, CFOP, natOp, `mod`=64, serie, nCT, dhEmi, tpImp,
`tpEmis` 1|2 (só normal e off-line), cDV, tpAmb, `tpCTe`=4 (TFinGTVe), verProc, cMunEnv/xMunEnv/UFEnv, `modal` 01|06
(rodoviário ou aéreo), `tpServ`=9, indIEToma, `dhSaidaOrig`, `dhChegadaDest`, um choice de tomador (`toma`/`tomaTerceiro`) e
`dhCont`/`xJust` opcionais. `emit` só CNPJ. `rem` e `dest` com CNPJ|CPF, IE, xNome, fone, ender, email (dest tem ISUF).
`origem` e `destino` são endereços TEndeEmi (UF sem EX). `detGTV`: `infEspecie` (1..n), `qCarga` (11,4), `infVeiculo`
(1..n). Não há `vPrest`, `imp` nem `infDoc`. Regras: I01 a I75 (ambiente, série 890-899, chave modelo 64, tomador I13 a
I15, emissão atrasada mais de 30 dias I17, emitente I18 a I26, remetente I27 a I34, destinatário I35 a I43, tomador I45 a
I53, duplicidade e cancelamento I54 a I56, município I57 a I62, autXML I63 a I66, responsável técnico I67 a I69, QR Code I70
a I75). Serviço `CTeRecepcaoGTVe` (`CTeRecepcaoGTVeV4`); assinatura em `infCte` (a config traz `infGTVe`, ver CONDUCT.md).
O CT-e OS de valores referencia a GTV-e por `infGTVe` (H045a, H046 a H053).

### CT-e Simplificado (`TCTeSimp` em cteSimp_v4.00.xsd; só spec, sem código)

Raiz `<CTeSimp versao>`, modelo 57, serviço `CTeRecepcaoSimp` (`CTeSimplificadoV4`). `ide`: como o 57 com `tpCTe` 5
(simplificado) ou 6 (substituição do simplificado), `tpEmis` 1|2|3|4|7|8, `procEmi` 0|3|4, `modal` 01|02|03, `tpServ`
0|1|2, UFIni e UFFim (sem municípios), `retira`. O tomador é o grupo `toma` (código 0 a 4, `indIEToma`, CNPJ|CPF, IE,
xNome, ender), sem rem/dest/exped/receb. Estrutura: `ide`, `compl`, `emit`, `toma`, `infCarga`, `det` (1..999, `@nItem`, com
documentos, `vPrest`, `vPrestLiq`, `vRec`, `Comp` e `IBSCBS` por item), `infModal`, `cobr`, `infCteSub` (`chCte`,
`indAlteraToma`), `imp`, `pgtoVinc`, `total` (`vTPrest`, `vTPrestLiq`, `vTRec`, `IBSCBSTot`, `vTotDFe`), `autXML`,
`infRespTec`, `infSolicNFF`, `infPAA`. Regras: seção 3.3 do MOC, ids G001 a G111c com numeração própria (125 linhas, não
confundir com as do CT-e 57), mais PAA01 a PAA05 e as seções de IBS/CBS. O gate de pré-validação precisa de catálogo
separado (`cte_simp`) porque os ids colidem com os do CT-e 57. Assinatura em `infCte`.
