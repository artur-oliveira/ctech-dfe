# Cobertura total de tags — NF-e/NFC-e e MDF-e

**Data:** 2026-08-26 **Escopo:** levar a emissão de NF-e (mod 55), NFC-e (mod 65) e MDF-e de ~55% / ~47% das tags do XSD
para 100%, sem transferir complexidade fiscal para o usuário final. **XSDs de referência:** `PL_010e_v1.02` (NF-e 4.00 +
reforma), `PL_MDFe_300b_NT012025_1.05`
(idêntico ao 1.03 — nenhuma mudança estrutural).

Eventos, contingência e inutilização estão em
[`2026-08-26-contingencia-e-inutilizacao-dfe.md`](./2026-08-26-contingencia-e-inutilizacao-dfe.md).

---

## 1. Princípio de alocação

Nenhuma tag nova entra direto no corpo do request sem antes passar por esta régua. A pergunta não é "qual campo falta",
é **"quem sabe esse valor e com que frequência ele muda"**.

| Nível                 | Onde mora                                 | Critério                                         | Exemplos                                                                                          |
|-----------------------|-------------------------------------------|--------------------------------------------------|---------------------------------------------------------------------------------------------------|
| 0 — Derivado          | lugar nenhum, calculado                   | O dado já existe no sistema ou é função de outro | `categCombVeic`, `SegCodBarra`, `qMDFe`, `prodPred/cEAN`, `hashCSRT`                              |
| 1 — Empresa           | `organizations`, `organization_*_configs` | Invariante do emitente                           | `IEST`, `IM`, `CNAE`, `ISUFEmit`, `idCSRT`, `infAdFisco`                                          |
| 2 — Operação          | `organization_operations`                 | Invariante do *tipo* de venda/movimento          | `indIntermed`, `indFinal`, `finNFe`, `xNEmp`, retenções, mensagens fiscais                        |
| 3 — Tributário        | `organization_tax_profiles`               | Invariante do regime aplicado ao item            | CSTs faltantes, ICMS efetivo, FCP diferido, IPI por unidade, PISST/COFINSST                       |
| 4 — Produto           | `organization_products`                   | Invariante do item                               | `NVE`, `cBarra`, `nFCI`, `cSelo`, `nRECOPI`, `veicProd`, dados de `comb`/`med`                    |
| 5 — Contraparte       | `organization_persons`                    | Invariante da pessoa                             | `idEstrangeiro`, intermediador, contratante do frete, dados bancários do condutor                 |
| 6 — Cadastro dedicado | tabela nova                               | Recorre entre notas e tem identidade própria     | DI, lote/rastro, apólice, terminal de pagamento, vale-pedágio, unidade de carga, produto perigoso |
| 7 — Request           | corpo da emissão                          | Só existe naquela nota                           | `nVol`, `lacres`, `nProt`, chassi, `xPed`                                                         |

**Regra prática:** se o mesmo valor for digitado duas vezes em notas diferentes, ele está no nível errado. Nível 6
existe justamente para o que hoje só caberia no request e não deveria.

Reuso obrigatório antes de criar qualquer cadastro novo:
`services/interpolate.go` (mensagens fiscais com variáveis), `organization_persons`
(qualquer papel de pessoa), `organization_vehicles` (qualquer veículo), `pickup_locations`
(qualquer TLocal salvo), `organization_services` (catálogo de serviços, hoje só NFS-e).

---

## 2. NF-e / NFC-e

### Fase A — Desbloqueadores (nota comum hoje rejeita ou sai incompleta)

| Grupo XSD                                                            | Caso de uso                                                                      | Nível        | Trabalho                                                                                                                                                                                                               |
|----------------------------------------------------------------------|----------------------------------------------------------------------------------|--------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ide/NFref` (refNFe, refNF, refNFP, refCTe, refECF, refNFeSig)       | Devolução, complementar, ajuste, nota de entrada sobre NF modelo 1, cupom fiscal | 7 + derivado | Seletor de documento na própria base: cliente escolhe a nota; o tipo de `NFref` sai do modelo do documento. `refNF`/`refNFP`/`refECF` (documentos fora do sistema) por formulário. Sem isso, `finNFe` 2/3/4 é inválido |
| `emit/IM`, `emit/CNAE`                                               | NF-e mista mercadoria + serviço (ISSQN)                                          | 1            | `IM` **já existe** em `person.nfse.im` — só ler. `CNAE` é campo novo em `organizations`                                                                                                                                |
| `emit/IEST`                                                          | Emitente substituto tributário em operação interestadual                         | 1            | Nova coluna por UF em `person.state_registrations` (`ie_st`)                                                                                                                                                           |
| `dest/idEstrangeiro`                                                 | Venda a pessoa no exterior; NFC-e a turista                                      | 5            | Tipo de documento novo em `organization_persons`                                                                                                                                                                       |
| `transp/vol` completo (`esp`, `marca`, `nVol`, `lacres`)             | Qualquer expedição por transportadora — hoje só vai peso                         | 2 + 7        | `esp`/`marca` default na operação ou no produto; `nVol` e `lacres` no request                                                                                                                                          |
| `transp/veicTransp/RNTC`, `reboque`                                  | Frete rodoviário próprio                                                         | 0            | Cadastro de veículos **já tem** os campos; só wiring no builder                                                                                                                                                        |
| `infAdic/infAdFisco`, `obsCont`, `obsFisco`, `procRef`               | Mensagem obrigatória por benefício fiscal/UF; nº do processo de benefício        | 2            | Estender as mensagens fiscais da operação para os quatro destinos, reusando `interpolate.go`. Hoje só `infCpl` é preenchido                                                                                            |
| `pag/detPag/CNPJPag`, `UFPag`, `xPag`; `card/CNPJReceb`, `idTermPag` | Pagamento capturado por adquirente/POS (NFC-e principalmente)                    | 6            | Cadastro `organization_payment_terminals` (adquirente, CNPJ recebedor, id do terminal, UF). Cliente seleciona o terminal, não digita CNPJ                                                                              |
| `infRespTec/idCSRT` + `hashCSRT`                                     | Obrigatório em UFs que exigem CSRT                                               | 1 + 0        | CSRT nos `*_configs`; hash é `Base64(SHA1(CSRT + chave))` — calculado                                                                                                                                                  |
| `det/imposto/IS/adRemIS`                                             | **Correção**: `builders_tax.go:413` emite `pISEspec`, removido no 010e           | —            | Renomear a tag. Independe de validador                                                                                                                                                                                 |

### Fase B — Regimes tributários faltantes

Tudo aqui é nível 3 (`tax_profiles`) e não deve aparecer no request de emissão.

| Grupo                                                                       | Caso de uso                                                                                               |
|-----------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| `ICMSPart` (CST 10/90 partilha)                                             | Venda interestadual a não contribuinte com partilha entre UFs                                             |
| `ICMSST` (CST 41)                                                           | Repasse de ST já retido anteriormente, operação interestadual                                             |
| `vBCEfet`/`pICMSEfet`/`vICMSEfet`/`pRedBCEfet` em ICMS60, ICMSST, ICMSSN500 | Revenda de mercadoria com ST — exigido por MG, RS e outras                                                |
| `vICMSSTDeson`/`motDesICMSST` em ICMS10/70/90                               | ST desonerada                                                                                             |
| `pFCPDif`/`vFCPDif` em ICMS51/90                                            | FCP diferido                                                                                              |
| `IPITrib/qUnid`+`vUnid`                                                     | IPI por unidade — bebida, cigarro                                                                         |
| `IPI/CNPJProd`, `cSelo`, `qSelo`                                            | Selo de controle do IPI (nível 4, produto)                                                                |
| `PISST`, `COFINSST`                                                         | ST de PIS/COFINS — combustíveis, farmacêutico                                                             |
| `II` (`vDespAdu`, `vIOF`)                                                   | Importação — casado com DI, Fase C                                                                        |
| `ISSQN` completo + `ISSQNtot`                                               | NF-e mista com serviço. Reusar `organization_services`                                                    |
| `total/retTrib`                                                             | Retenções federais PIS/COFINS/CSLL/IRRF/INSS. Nível 2: perfil de retenção na operação, valores calculados |
| `impostoDevol/pDevol`+`vIPIDevol`                                           | Devolução por não contribuinte. Derivado de `finNFe=4` + `NFref`                                          |
| `transp/retTransp`                                                          | Frete com ICMS retido pelo remetente. Nível 5: perfil na transportadora                                   |
| `det/obsItem`                                                               | Observação fiscal por item (níveis 3/4)                                                                   |

### Fase C — Importação e exportação

| Grupo                                              | Caso de uso                                                            | Nível | Trabalho                                                                                                                                                                                                                               |
|----------------------------------------------------|------------------------------------------------------------------------|-------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `prod/DI` + `adi`                                  | Revenda de importado próprio. Uma DI cobre várias notas e vários itens | 6     | **Cadastro `organization_import_declarations`**: nDI, dDI, xLocDesemb, UFDesemb, dDesemb, tpViaTransp, vAFRMM, tpIntermedio, cExportador + adições. Na emissão o cliente vincula a DI ao item; `nAdicao`/`nSeqAdic` derivam do vínculo |
| `prod/nFCI`                                        | Produto importado com conteúdo de importação (origem 3/5/8)            | 4     | Campo no produto                                                                                                                                                                                                                       |
| `prod/detExport` + `exportInd`                     | Exportação indireta (venda a trading com remessa ao exterior)          | 7     | Formulário; `nDraw` pode ser nível 4                                                                                                                                                                                                   |
| `exporta` (UFSaidaPais, xLocExporta, xLocDespacho) | **Toda** exportação direta — hoje impossível                           | 1 + 6 | UF de saída na operação de exportação; local de despacho reusando `pickup_locations`                                                                                                                                                   |
| `prod/NVE`                                         | Nomenclatura de Valor Aduaneiro, até 8 por item                        | 4     | Lista no produto                                                                                                                                                                                                                       |

### Fase D — Segmentos verticais

| Grupo                                                    | Caso de uso                                                                    | Nível | Trabalho                                                                                                                                                                                       |
|----------------------------------------------------------|--------------------------------------------------------------------------------|-------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `prod/rastro` (nLote, qLote, dFab, dVal, cAgreg)         | Medicamento (obrigatório), alimento, bebida, agrotóxico                        | 6     | **Cadastro `organization_product_lots`** por produto. Cliente escolhe o lote; quantidade rateada automaticamente. Digitar lote a cada nota é inviável                                          |
| `prod/comb` completo (CIDE, encerrante, origComb, qTemp) | Posto de combustível (NFC-e) e distribuidora                                   | 6     | Bombas/tanques viram cadastro (`organization_fuel_pumps`: nBico, nBomba, nTanque); `vEncIni` é o `vEncFin` da venda anterior — **derivado**, nunca digitado. `origComb` no cadastro do produto |
| `prod/veicProd` completo                                 | Concessionária / montadora                                                     | 4 + 7 | Dados do modelo no produto (tpVeic, cMod, tpComb, CMT, cilindradas...), variáveis (chassi, motor, cor) no request — padrão já correto, só completar o cadastro                                 |
| `prod/med`                                               | Farmácia / distribuidor de medicamento                                         | 4     | `prod_type=med` já existe; completar cProdANVISA, vPMC, xMotivoIsencao                                                                                                                         |
| `agropecuario` (defensivo, guiaTransito)                 | Venda de defensivo agrícola; transporte de animal/vegetal com guia de trânsito | 1 + 7 | Responsável técnico (CPF) no cadastro da org; nº do receituário e da guia no request                                                                                                           |
| `cana` (forDia, deduc, totais)                           | Usina de açúcar/álcool — fornecimento de cana                                  | 2 + 6 | Safra/referência na operação; `forDia` gerado dos lançamentos diários de entrega, não digitado                                                                                                 |
| `compra` (xNEmp, xPed, xCont)                            | Venda a órgão público com nota de empenho                                      | 2 + 7 | Operação "venda a órgão público" habilita os campos                                                                                                                                            |
| `prod/nRECOPI`                                           | Papel imune                                                                    | 4     | Flag + número no produto                                                                                                                                                                       |
| `ide/indIntermed` + `infIntermed`                        | Venda por marketplace                                                          | 5     | Intermediador como `organization_persons` com papel próprio; operação aponta para ele                                                                                                          |
| `ide/dhSaiEnt`, `dPrevEntrega`                           | Data de saída distinta da emissão; previsão de entrega                         | 2 + 7 | Default na operação, override no request                                                                                                                                                       |
| `prod/xPed`, `nItemPed`                                  | Pedido do cliente                                                              | 7     | Request/integração                                                                                                                                                                             |
| `prod/cBarra`, `cBarraTrib`                              | Produto sem GTIN com código próprio                                            | 4     | Produto                                                                                                                                                                                        |

### Fase E — Reforma tributária (IBS/CBS/IS)

Bloco maior, dependente da NT vigente na data de implementação. Hoje só existe o esqueleto de
`IBSCBS`. Faltam, no item: `gTribRegular`, `gTribCompraGov`, `gIBSCBSMono` (`gMonoReten`,
`gMonoRet`, `gMonoDif`, totais), `gTransfCred`, `gAjusteCompet`, `gEstornoCred`,
`gCredPresOper`, `gCredPresIBSZFM`, `gALCZFMCBS` (novo no 010e), `pDevTrib` nos três
`gDevTrib`, `prod/gCred`, `prod/tpCredPresIBSZFM`, `prod/indBemMovelUsado`. Nos totais:
`IBSCBSTot/gMono`, `IBSCBSTot/gEstornoCred`, `ISTot`, `vNFTot`. Em `ide`: `cIndOp`,
`cMunFGIBS`, `tpNFDebito`, `tpNFCredito`, `gCompraGov` + `refDFeAnt`. Em `emit`: `ISUFEmit`.

Alocação: níveis 3 e 4 quase inteiramente — CST/cClassTrib e alíquotas efetivas são perfil tributário, não input de
nota. `gCompraGov`, `cIndOp` e `tpNFDebito/Credito` são nível 2.

Esta fase é pré-requisito dos eventos da reforma (série 1121xx/2111xx/2121xx) — não faz sentido emitir evento de
apropriação de crédito presumido sem `gCredPresOper` no item.

### Fora de escopo (declarado)

`avulsa` (NF-e emitida pelo fisco, não pelo contribuinte), `infSolicNFF` e `infPAA`
(exclusivos do fluxo Nota Fiscal Fácil). Reavaliar só sob demanda concreta.

---

## 3. MDF-e

### Fase A — Rodoviário completo (o modal que já está ligado)

| Grupo                                                                                                         | Caso de uso                                                                  | Nível | Trabalho                                                                                                                                                                                                  |
|---------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------|-------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `veicTracao/cInt`, `capM3`; `emit/enderEmit/xCpl`                                                             | —                                                                            | 0     | **Campos já existem** em `organization_vehicles` (`cint`, `cap_m3`) e no endereço da org. Puro wiring do builder                                                                                          |
| `categCombVeic`                                                                                               | Categoria da combinação veicular                                             | 0     | Derivado da composição: trator + nº de reboques                                                                                                                                                           |
| `infANTT/valePed` (disp: CNPJForn, CNPJPg, nCompra, vValePed, tpValePed)                                      | Vale-pedágio obrigatório no transporte rodoviário de carga (Lei 10.209)      | 6     | Cadastro `organization_toll_providers` (fornecedor + CNPJ pagador). Por viagem só entram nº da compra e valor                                                                                             |
| `infANTT/infContratante` + `infContrato`                                                                      | Frete contratado por terceiro                                                | 5     | Papel novo em `organization_persons`; contrato (nº + valor global) junto                                                                                                                                  |
| `infANTT/infPag` completo (Comp, vContrato, indPag, vAdiant, infPrazo, infBanc/PIX, tpAntecip, indAltoDesemp) | Pagamento a TAC / transportador autônomo — obrigatório quando há contratante | 5 + 6 | Dados bancários e PIX no cadastro do condutor/TAC (`organization_persons`, que já alimenta `vehicle_sets`); componentes (`tpComp`) como perfil de frete reutilizável. Parcelas derivam do prazo escolhido |
| `lacRodo`, `infMDFe/lacres`                                                                                   | Lacres da carga                                                              | 7     | Request                                                                                                                                                                                                   |
| `codAgPorto`                                                                                                  | Operação portuária                                                           | 7     | Request, nicho                                                                                                                                                                                            |

### Fase B — `infDoc` (a maior lacuna: 10 de 78 tags)

| Grupo                                                                        | Caso de uso                                                      | Nível | Trabalho                                                                                                                                                                                                              |
|------------------------------------------------------------------------------|------------------------------------------------------------------|-------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `SegCodBarra`, `indReentrega`                                                | Código de barras da NF-e; reentrega                              | 0     | **Derivados do XML da NF-e referenciada**, que o sistema já lê e parseia. Nunca perguntar                                                                                                                             |
| `peri` (nONU, xNomeAE, xClaRisco, grEmb, qTotProd, qVolTipo)                 | Transporte de produto perigoso — obrigatório e fiscalizado       | 4/6   | Classificação ONU no cadastro do produto. Ao referenciar a NF-e, o sistema identifica os itens perigosos e monta `peri` sozinho. **É o exemplo canônico do princípio**: o cliente cadastra uma vez, nunca mais digita |
| `infUnidTransp` + `infUnidCarga` + `lacUnidTransp`/`lacUnidCarga` + `qtdRat` | Carga em contêiner, carreta ou vagão com rateio entre documentos | 6     | Cadastro `organization_cargo_units` (tipo + identificação). Rateio calculado a partir dos pesos dos documentos                                                                                                        |
| `infEntregaParcial` (qtdTotal, qtdParcial)                                   | Entrega fracionada de uma NF-e em viagens distintas              | 7     | Request                                                                                                                                                                                                               |
| `indPrestacaoParcial`, `infNFePrestParcial`                                  | Prestação parcial de serviço sobre CT-e                          | 7     | Request                                                                                                                                                                                                               |
| `infMDFeTransp` + `tot/qMDFe`                                                | MDF-e transportando outro MDF-e (transbordo)                     | 7 + 0 | Mesma estrutura de `infNFe`/`infCTe`; `qMDFe` é contagem                                                                                                                                                              |

### Fase C — Seguro e complementos

| Grupo                                                     | Caso de uso                                                                      | Nível | Trabalho                                                                                                       |
|-----------------------------------------------------------|----------------------------------------------------------------------------------|-------|----------------------------------------------------------------------------------------------------------------|
| `seg/infResp` (respSeg, CNPJ/CPF) + `infSeg` (xSeg, CNPJ) | Seguro da carga — hoje só `nApol`/`nAver` soltos, sem responsável nem seguradora | 6     | Cadastro `organization_insurance_policies` (responsável, seguradora, nº da apólice). Por viagem só o averbação |
| `ide/indCanalVerde`                                       | Carga expressa / canal verde Brasil                                              | 2     | Config MDF-e                                                                                                   |
| `ide/indCarregaPosterior`                                 | Carregamento posterior à emissão                                                 | 2     | Config MDF-e                                                                                                   |
| `prodPred/cEAN`                                           | —                                                                                | 0     | Do produto predominante                                                                                        |
| `infAdic/infAdFisco`                                      | Mensagem ao fisco                                                                | 1     | Config MDF-e                                                                                                   |
| `infRespTec/idCSRT`+`hashCSRT`                            | UFs que exigem CSRT                                                              | 1 + 0 | Igual à NF-e                                                                                                   |

Fora de escopo: `infSolicNFF`, `infPAA` (fluxo NFF).

### Fase D — Demais modais

`enabledModals` em `mdfes.go:92` libera só `rodoviario`. Os builders de `aereo`, `aquav` e
`ferrov` existem e estão desligados.

- **Aéreo, ferroviário:** builders completos vs. XSD. Ligar + DAMDFE + testes.
- **Aquaviário:** builder ignora `infEmbComb`, `infUnidCargaVazia`, `infUnidTranspVazia`, `MMSI`. Completar antes de
  ligar. Embarcação e terminais são nível 6 (cadastro), não request.

---

## 4. Ordem de execução

1. **Fase A NF-e** + correção `pISEspec` → `adRemIS`. Destrava devolução, exportação por transportadora, marketplace,
   NFC-e com POS.
2. **Fase A MDF-e** (metade é wiring de campos já cadastrados) + **Fase B `infDoc`**. Maior salto de cobertura por
   esforço.
3. **Fase B NF-e** (tax profiles) — cada CST novo é isolado e testável.
4. **Fase C NF-e** (importação/exportação) — depende do cadastro de DI.
5. **Fase C/D MDF-e** (seguro, modais).
6. **Fase D NF-e** (verticais) — priorizar por demanda real de cliente.
7. **Fase E** (reforma) — cronograma amarrado à NT vigente; abre os eventos da reforma.

## 5. Critério de pronto por fase

- Toda tag nova tem teste unitário de builder **e** um payload de integração em
  `py-dfe/tests/integration/fiscal_payloads.py` (ou equivalente no `go-dfe`).
- Nenhuma tag nova chega ao request sem justificativa contra a régua da seção 1.
- `xsdorder/table.go` cobre a ordem — verificar antes de implementar; a tabela já conhece quase tudo.
- `DOCS.md` e `DynamoDB-Tables.md` atualizados no mesmo commit (cadastros novos = tabelas novas).
