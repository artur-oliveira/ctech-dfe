# Auditoria MOC x XSD: NF-e (modelo 55)

**Data:** 2026-11-09. **Tipo:** auditoria somente leitura (nenhum código do repositório foi alterado; os experimentos rodaram numa
cópia em scratchpad). **Fontes usadas:** XSDs vendorizados `go-dfe/schemas/nfe/PL_010e_v1.02` (SHA256SUMS e README),
`docs/specs/2026-11-04-moc-gap-nfe-nfce.md`, `docs/plans/2026-11-04-moc-cobertura-total.md`, tabela de meios de pagamento
em `tmp/tabelas/` e o código. Nada foi afirmado de memória: cada achado cita o trecho do XSD ou do documento.

## 1. Resumo

| Severidade | Quantidade |
|---|---|
| Blocker (a SEFAZ rejeita ou o dado sai errado) | 5 |
| Major | 11 |
| Minor | 9 |
| **Total** | **25** |

O que está **correto** (verificado por script independente, ver seção 2): estrutura, ocorrência, tipos, tamanhos, padrões e
enumerações do spec gerado batem com o XSD; a tabela `xsdorder` bate com o XSD; o IBS/CBS do item (bloco da reforma) passou na
validação de todos os CSTs testados. Os defeitos estão em lugares que o teste de paridade não alcança: construtores de ICMS/IPI/PIS/IS
com configuração completa, totais, e a UI.

## 2. Método e o que foi provado

1. **Gerador fiel ao XSD.** Reescrevi em Python (lxml) um percurso independente do `leiauteNFe_v4.00.xsd` e comparei com
   `go-dfe/validate/specs/nfe.json`: 1046 caminhos idênticos nos dois lados, nenhum só de um lado; ocorrência, `minLength`, `maxLength`,
   `length`, `totalDigits`, `fractionDigits`, padrões e enumerações iguais. As únicas diferenças são semânticas de choice (o spec marca
   o braço com `choice`, não com `min=0`) e de sequence opcional (achado F-13).
2. **`xsdorder`.** `TestXSDOrderMatchesSpecs` passa; além disso nenhum nome da tabela de NF-e (linhas 36 a 382 de `table.go`) é
   desconhecido do XSD (exceto `xCampo`, que é atributo e inofensivo). As posições dos nós de overlay (NT 2026.005 e 2026.008) não são
   verificáveis (seção 6).
3. **Construtores contra o XSD.** Copiei `api` e `go-dfe` para o scratchpad, acrescentei um `ValidateAt` ao validador e rodei os
   construtores com configuração máxima e mínima: ICMS normal (CST 00, 02, 10, 15, 20, 30, 40, 41, 50, 51, 53, 60, 61, 70, 90 e ICMSPart),
   ICMSSN (101 a 900), IPI (14 CSTs, dois modos), PIS e COFINS (33 CSTs), PISST, COFINSST, IS, ISSQN, ICMSUFDest, IBS/CBS (14 CSTs
   x 6 variantes e 3 regimes do Simples). Scripts em
   `/tmp/claude-1000/-home-artur-Documents-Projects-Ctech-ctech-dfe/bdec0605-6b99-4ef6-91f0-b953c8fc46c9/scratchpad/copy/api/internal/services/nfes/zz_*_test.go`.
4. **Catálogo de regras.** 348 regras de NF-e (26 com modo `block`, 322 `warn`); as 32 regras que a API promove a block existem todas no
   catálogo (nenhum id digitado errado).

## 3. Achados

Formato: **ID, severidade, local, evidência, correção proposta.** Cada achado foi conferido duas vezes (leitura do código e execução do
construtor ou consulta ao XSD).

### Blockers

**F-01. IPI: CST 00, 49, 99 saem como `IPINT` e CST 51 sai como `IPITrib` (schema inválido, 225).**
- Local: `api/internal/services/nfes/builders_tax.go:420` (`ipiCSTTributado = {"50","51"}`) e `:440-441`.
- Evidência: XSD `leiauteNFe_v4.00.xsd`, `IPI/IPITrib/CST` enum `{00,49,50,99}` e `IPI/IPINT/CST` enum `{01,02,03,04,05,51,52,53,54,55}`.
  Execução: `IPI 00`, `49`, `99` falham em `IPINT/CST` e `IPI 51` falha em `IPITrib/CST`. A UI oferece os 14 valores
  (`ui/src/lib/data/ipi.ts:2-17`), então é alcançável.
- Correção: `ipiCSTTributado = {00, 49, 50, 99}`. O cálculo de vBC/pIPI/vIPI e o total (`t.VIPI`) passam a valer também para 00, 49 e 99.

**F-02. PIS/COFINS CST 03 (`PISQtde`/`COFINSQtde`) sempre sai com `qBCProd 0.0000` e `vPIS 0.00`: imposto zerado, dado errado.**
- Local: `builders_tax.go:538-546` (PIS) e `:565-573` (COFINS).
- Evidência: XSD `PISQtde`: "03 - Base de Calculo = Quantidade Vendida x Alíquota por Unidade de Produto" (`qBCProd`, `vAliqProd`,
  `vPIS`). O construtor fixa `qBCProd="0.0000"` e `vPIS="0.00"` e só repassa `vAliqProd`. Os totais `vPIS`/`vCOFINS` herdam o zero.
- Correção: `qBCProd = qTrib` do item, `vPIS = qBCProd x vAliqProd` (2 casas); mesma coisa em COFINS.

**F-03. ICMS60, ICMSST (CST 41) e ICMSSN500: grupos opcionais do XSD saem incompletos (225).**
- Local: `builders_tax.go:693-712` (`addICMSEfetivo`, `pRedBCEfet` só sai se a redução for diferente de zero) e `:715-722`
  (`addSTRetida`, `pST` só sai se configurado).
- Evidência: XSD `ICMS60` e `ICMSSN500` têm `sequence minOccurs=0` com `[pRedBCEfet, vBCEfet, pICMSEfet, vICMSEfet]` (todos obrigatórios)
  e `[vBCSTRet, pST, vICMSSubstituto?, vICMSSTRet]` (pST obrigatório). Execução com `icms_p_icms_efet` sozinho:
  `{"pICMSEfet","vBCEfet","vICMSEfet"}` sem `pRedBCEfet`; com `icms_v_bc_st_ret` sozinho: sem `pST`. O validador não acusa (F-13).
- Correção: emitir `pRedBCEfet` sempre que o grupo sair (valor `0.00` quando não há redução) e exigir `pST` junto de `vBCSTRet`.

**F-04. `ICMSTot`: `vFCPSTRet` fixo em `0.00` e totais monofásicos ausentes (rejeições 859, 967, 968, 969).**
- Local: `builders_total.go:179` (`"vFCPSTRet": "0.00"`), `:171-196` (nenhum `qBCMono`, `vICMSMono`, `qBCMonoReten`, `vICMSMonoReten`,
  `qBCMonoRet`, `vICMSMonoRet`) e a struct `totals` sem esses acumuladores.
- Evidência: o catálogo do próprio repositório tem `W06b-10 (859) vFCPSTRet`, `W06c-10 (967) vICMSMono`, `W06d-10 (968) vICMSMonoReten`,
  `W06e-10 (969) vICMSMonoRet` (`go-dfe/validate/rules_nfe.go:1308-1311`) comparando o total com a soma dos itens. Os itens emitem
  `vFCPSTRet` (`addSTRetida`) e os CSTs 02, 15, 53, 61 emitem `vICMSMono`, `vICMSMonoReten`, `vICMSMonoRet`. XSD: os seis campos existem
  em `ICMSTot` (`vFCPSTRet` 1..1; os demais 0..1). `vNF` também não soma `vICMSMonoReten` (`nfeVNF` soma, `rules_nfe.go:1385`).
- Correção: acumular dos itens, emitir quando algum item trouxe, somar `vICMSMonoReten` no `vNF`.

**F-05. IS com `uTrib`: `qTrib` fixo em `"0.0000"` não passa no padrão do XSD e é dado inventado.**
- Local: `builders_tax.go:515-519`.
- Evidência: XSD `IS/qTrib` (`TDec_1104OpRTC`) padrão `0\.[1-9]...|[1-9]{1}[0-9]{0,10}(\.[0-9]{4})?`: não aceita zero. Execução `IS 000`:
  `qTrib pattern`.
- Correção: `qTrib` = quantidade tributável do item; `vIS` por `adRemIS x qTrib` quando o modo for específico.

### Major

**F-06. `vNF` subtrai `vICMSDeson` sempre, mesmo com `indDeduzDeson=0` (que é o default do construtor).**
- Local: `builders_total.go:162` e default em `builders_tax.go:138` (`icms_ind_deduz_deson`, `"0"`).
- Evidência: XSD `indDeduzDeson`: "0=Valor do ICMS desonerado (vICMSDeson) não deduz do valor do item (vProd) / total da NF-e; 1=...deduz".
  O catálogo `nfeVNF` (`rules_nfe.go:1386-1412`) só abate nos itens com `indDeduzDeson=1`, mas aceita as duas leituras, então não detecta.
- Correção: abater no `vNF` apenas o `vICMSDeson` dos itens com `indDeduzDeson=1`.

**F-07. API recusa `tPag` 23, 24 e 91, que a UI oferece e que estão vigentes.**
- Local: `api/internal/services/nfes/emit.go:277` e `api/internal/api/v1/dto.go:439` (`oneof=... 22 90 99`).
- Evidência: `tmp/tabelas/Tabela_de_meios_de_pagamento - 04032026.xlsx` (23 e 24 desde 2026-05-04, 91 desde 2025-11-03);
  `ui/src/lib/types/api.ts:1429-1432` lista 23, 24 e 91; XSD `tPag` aceita qualquer `[0-9]{2}`.
- Correção: acrescentar 23, 24, 91 nos dois `oneof` (fonte única: uma constante).

**F-08. UI: lista de CST do ICMS não inclui 02, 10, 15, 30, 53, 61.**
- Local: `ui/src/lib/data/icms.ts:1-11` (9 valores) usada em `ui/src/components/tax/TaxFieldsEditor.tsx:245`.
- Evidência: o mesmo editor define `ICMS_ST_CSTS = {10,30,70}` e `ICMS_MONO_CSTS = {02,15,53,61}` (`TaxFieldsEditor.tsx:35-38`) e o construtor
  trata todos; XSD tem `ICMS10`, `ICMS30`, `ICMS02`, `ICMS15`, `ICMS53`, `ICMS61`. Resultado: ICMS-ST de CST 10/30 e monofasia não
  podem ser cadastrados.
- Correção: completar a lista (e ligar `ICMSPart` por 10/90 já existente).

**F-09. UI: motivo de desoneração e redução de BC não seguem o XSD por CST.**
- Local: `ui/src/lib/data/is.ts:12-25`, `TaxFieldsEditor.tsx:57-59, 286-294, 391-397`.
- Evidência: XSD `motDesICMS`: ICMS20 `{3,9,10,11,12}`, ICMS30 `{6,7,9}`, ICMS40 `{1,3,4,5,6,7,8,9,10,11,16,90}`, ICMS70/90 `{3,9,12}`,
  ICMSPart `{9,10,11}`; `motDesICMSST` `{3,9,12}`. A UI usa uma lista só, que contém `2` (inexistente em qualquer enumeração) e não tem
  `12`; `showMotDeSon` é `['40','41','50','51']` (ICMS51 não tem o campo; 20, 30, 70 e 90 têm e a UI não mostra); `showPRedBC` é
  `['20','40','70']` (ICMS40 não tem `pRedBC`; 51 e 90 têm). O construtor lê `icms_mot_des` para 20, 30, 70, 90, então o valor só entra por API.
- Correção: enumeração por CST (fonte única gerada do XSD) e visibilidade por CST.

**F-10. `dest/ISUF` (E18) não pode ser emitido; a UI oferece motivo "7 SUFRAMA".**
- Local: `builders_party.go:214-257` (`buildDest` sem ISUF), `dto.go` (só `isuf_emit`).
- Evidência: XSD `dest/ISUF` `[0-9]{8,9}`; regra do catálogo `N28-10 (625)` exige ISUF do destinatário com `motDesICMS=7`
  (`rules_nfe.go:1178-1184`); gap spec 3.2 item 3 (`E18-20`, DV do SUFRAMA).
- Correção: campo `isuf` no cadastro de destinatário e emissão em `dest`.

**F-11. `dhEmi` e a chave usam `time.Now()` no fuso do processo.**
- Local: `emit.go:487` e `:1636` (`fmtDhEmi`), `nfce_emit.go:152`.
- Evidência: não há `TZ`, `LoadLocation` ou `time.Local` no caminho da NF-e (a NFS-e usa o fuso da configuração, `nfses/emit.go:135`); em
  Lambda o padrão é UTC, logo `+00:00`. O gap spec 3.2 item 5 lista `B10-20/30 (504/505)`, `B10a`. **Não provado**: o texto dessas regras
  não está nos documentos disponíveis (seção 6).
- Correção: fuso do emitente pela UF ou da configuração fiscal, como na NFS-e.

**F-12. Validador: falso `choice_conflict` em todo item com IPI.**
- Local: `go-dfe/validate/validate.go:262-271, 289-294`.
- Evidência: o XSD de `imposto` tem dois braços (`ICMS, IPI?, II?` e `IPI?, ISSQN`), logo `IPI` aparece nos dois braços do spec
  (`arm 1` e `arm 2`); o laço marca os dois braços presentes. Execução: `imposto` com `ICMSSN102` + `IPITrib` ou `IPINT` devolve
  `choice_conflict`; sem IPI devolve zero. 9 ocorrências nos testes existentes. Hoje só vai ao log (shadow); promover o schema a block
  recusaria toda nota com IPI e esconderia conflitos reais.
- Correção: contar braços só pelos elementos exclusivos do braço, ou deduplicar por nome antes de marcar.

**F-13. Validador: sequência opcional do XSD não é verificada (tudo-ou-nada do grupo).**
- Local: `go-dfe/validate/gen/main.go:335-400` (grupo `sequence minOccurs=0` vira filhos com `min=0`) e `validate.go:262-285`.
- Evidência: o XSD tem mais de 40 grupos assim em NF-e (`vBCFCP,pFCP,vFCP`; `vICMSDeson,motDesICMS`; `pRedBCEfet,vBCEfet,pICMSEfet,vICMSEfet`;
  `vBCSTRet,pST,vICMSSTRet`; `dhCont,xJust`; `CEST` etc.). Execução: os dois casos de F-03 devolvem zero achados.
- Correção: o gerador marca o grupo (`group` id, `min` do grupo); o validador exige todos os obrigatórios do grupo quando algum aparece.

**F-14. Gate recusa só tamanho; enum, padrão, obrigatório, choice, max_occurs e tag desconhecida ficam em shadow.**
- Local: `api/internal/services/dfecheck/dfecheck.go:68-95` (`sizeCodes = {min_length, max_length}`).
- Evidência: F-01, F-03, F-05 e a ausência de limite de itens (F-19) são rejeição 225 que o Gate deixaria passar. Além disso, as 26 regras
  `ModeBlock` do catálogo (`A03-10, B23-10, I03-10, I12-10 ...`) não são aplicadas: o Gate só considera os 32 ids de
  `nfes/prevalidate.go:46-64` (a API ignora o retorno `blocking` de `Apply`). O modo do catálogo é metadado morto.
- Correção: depois de corrigir F-12 e F-13, promover `enum`, `pattern`, `required`, `choice_*` e `max_occurs` a block; ou aplicar o modo do
  catálogo.

**F-15. UI/PIS e COFINS: rótulos e valores não seguem o XSD.**
- Local: `ui/src/lib/data/pis_cofins.ts:2-12`.
- Evidência: XSD: CST 02 é `PISAliq` "Base de Cálculo = Valor da Operação (Alíquota Diferenciada)"; CST 03 é `PISQtde`; 05 é `PISNT`
  "Operação Tributável (ST)"; `PISOutr` aceita 49 a 56, 60 a 67, 70 a 75, 98, 99. A UI rotula o 02 como "BC = qtd x alíq", não oferece 03,
  05 nem os demais de `PISOutr`.
- Correção: tabela única por grupo do XSD, rótulos do XSD.

**F-16. Tags do XSD que o construtor nunca emite e que têm regra no catálogo.**
- `ICMS51/cBenefRBC` e `ICMS90/cBenefRBC` (N14a), `ICMS60/ICMSST/ICMSSN500 vICMSSubstituto`, `ICMS53/qBCMonoDif` e `adRemICMSDif`
  (nenhuma ocorrência em código de `api`; só em `xsdorder/table.go`). XSD: todas `minOccurs=0`. Nota: o gap spec afirma `cBenefRBC` também
  em ICMS20 e ICMS70, mas o XSD vendorizado só o tem em ICMS51 e ICMS90 (divergência MOC x XSD, ver seção 6).
- `PISST/indSomaPISST` e `COFINSST/indSomaCOFINSST`: o próprio código assume que "o builder ainda não emite" (`builders_total.go:160-161`);
  o `vNF` do catálogo (`rules_nfe.go:1392-1400`) soma o ST quando o indicador é 1.

### Minor

**F-17.** Destinatário e transporte: `transp/vagao` e `transp/balsa` (X25a/X25b) não existem no construtor
(`builders_transp.go:19-87`, XSD choice com `veicTransp`). Regras `X25a-10/X25b-10` já existem no catálogo.

**F-18.** Não expressíveis, todos opcionais no XSD: `det/DFeReferenciado` e `det/vItem`, `prod/infProdEmb`, `prod/infProdNFF`, `infPAA`,
`infSolicNFF`, `avulsa` (esta última é do Fisco). O catálogo já tem `I86-10`, `I87-10`.

**F-19.** Sem limites de ocorrência no DTO onde o XSD limita: `products` sem `max=990` (`emit.go:40`, XSD `det` 1..990), `payments` sem
`max=100` (`emit.go:41`, `detPag` 1..100), `cobr_duplicatas` sem `max=120` (`emit.go:52`, `dup` 0..120), `armas` sem `max=500`
(`emit.go:197`), e `DI` (100), `adi` (999), `cana/forDia` (31), `agropecuario/defensivo` (20) sem teto. `NFref` está em 500 (XSD 999,
restritivo, ok). O Gate não bloqueia `max_occurs` (F-14).

**F-20.** Evento 112130 (perecimento no transporte do fornecedor, `tpAutor=1`) e 112120 (importação ALC/ZFM não convertida) têm XSD
(`e112130_v1.00.xsd`, `e112120_v1.00.xsd`) e nenhum construtor; `TestXSDOrderMatchesEventSpecs` os lista como `notBuilt`
(`xsdorder_parity_test.go`). 412120 e 412130 são do Fisco (fora de escopo). Eventos 110130, 110131, 110150, 110300 existem no código mas
não têm XSD vendorizado (README): paridade não verificável.

**F-21.** `retirada` e `entrega` sem CNPJ nem CPF passam pelo DTO (`emit.go:348-349`, ambos `omitempty`) e saem sem o choice obrigatório
(`choices min 1`, ids 5 e 6 do spec): schema inválido.

**F-22.** Inscrição estadual só valida tamanho: `StateRegistrationBody.state_registration` `min=2,max=14` (`dto.go:48`) e UI
`layout-limits.ts:24`; XSD `emit/IE`, `dest/IE` etc. `[0-9]{2,14}|ISENTO`. "12.345.678-9" passa e falha no schema.

**F-23.** Overlay: `cAtivSN` com padrão `[0-9]{1,9}` (`validate/overlay.go:49`) enquanto o gap spec dá "C 2" (I17d) e a UI/DTO usam
`01 02 03 04 90` (`dto.go:311`); o validador aceita o que o MOC recusaria.

**F-24.** Rótulos de `tPag` divergentes da tabela oficial: 18 ("Transferência bancária, Carteira Digital" vs "TED (Transferência
Eletrônica Disponível)"), 17 ("PIX" vs "Pagamento Instantâneo (PIX) - Dinâmico") em `ui/src/lib/types/api.ts:1423-1424`.

**F-25.** `DI` só emite `CNPJ` do terceiro (`builders_prod.go:~212-232`); o XSD aceita `CNPJ` ou `CPF` (choice). Item `imposto/vTotTrib`
nunca é emitido e `ICMSTot/vTotTrib` é fixo `0.00` (`builders_total.go:191`), opcionais no XSD.

## 4. Cobertura das regras de validação (catálogo x MOC)

- Catálogo de NF-e: 348 regras, `warn` 322 e `block` 26. A API bloqueia 32 (`prevalidate.go:46-64`), todas existentes.
- O gap spec cita 202 ids de regra explicitamente; 88 não existem no catálogo. Dependem de tabela externa (NCM, CFOP, CEST, GTIN, município,
  país, IE por UF): `I05-10/20/30`, `I05c-10`, `I08-04`, `I08-90`, `I08-140`, `I03-20/30`, `I12-20/60`, `I09-10`, `I13-10`, `B12-10`,
  `C10-10`, `E10-10`, `E14-04`, `X07-10/20`, `X17-10`, `U05-10`, `U14-10`, `LA01-20/30`, `LA02-10`, `ZA01-10`. Determinísticas ainda ausentes
  (texto da regra não disponível, ver seção 6): `B25-70`, `B25b-10/50`, `N07-10`, `N11-10`, `N12-10/70/80/81/84/85/90/97/100`, `N12a-50/70`,
  `N23d-10`, `N38-10`, `O06-10`, `W16-50`, `Y01-20`, `YA03a-10`, `YA04-10`, `ZD01-10`, `ZD07-10`, `NB01-30`, `ZX01-10`, `ZX02-20/224/324/338`.
- Modos: nenhuma regra de totais é `block` no catálogo, e nenhuma está no mapa da API; o `vNF` (W16-10) e os somatórios W03 a W15 só vão ao
  shadow, mas por F-04 e F-06 o construtor os viola. Recomendação: depois dos reparos, promover `W03-10 .. W16-10` a block, que são
  determinísticas (autoconsistência do documento).
- Regras erradas ou largas: `nfeVNF` aceita duas leituras do `vICMSDeson` (F-06), então não protege.

## 5. UI e API: campos que não se expressam ou estão mal rotulados

F-07, F-08, F-09, F-10, F-15, F-22, F-24. Fora esses, os `oneof` do DTO para `finNFe (1 a 6)`, `indPres`, `tpNF`, `modFrete`, `tpNFDebito`,
`tpNFCredito`, `indIntermed`, `indEscala`, `indTot`, `tpCredPresIBSZFM`, `tpRest` coincidem com as enumerações do XSD.

## 6. O que NÃO foi possível verificar e o documento necessário

| Item | Por que | Documento necessário |
|---|---|---|
| Posição e tipos de `tpRegimeIBSCBS`, `cAtivSN`, `gTribSN` (item e total), `vUnComLiq`, `vProdLiq`, `vProdLiqTot`, `ICMSPrevistoPagtoAntecip` | o XSD vendorizado (PL_010e) é anterior às NT 2026.005 e 2026.008; o overlay e a posição em `xsdorder` não têm fonte | XSD oficial `PL_010f` ou posterior, ou NT 2026.005 v1.00 e NT 2026.008 v1.01 |
| Texto de todas as regras de validação (condição, cStat, obrigatoriedade) | o MOC online não está no repositório nem em `~/Downloads`; só há os ids citados no gap spec | MOC NF-e "Regras de Validação" (HTML ou PDF) |
| `dhEmi`: fuso exigido por UF (B09, B10) | idem | MOC NF-e, regras B09 e B10 |
| `cBenefRBC` em ICMS20 e ICMS70 | o gap spec diz que sim; o XSD vendorizado só tem em 51 e 90 | MOC leiaute N14a ou XSD mais novo |
| Eventos 110130, 110131, 110150, 110300 (NF-e) | sem XSD no pacote (README) | pacotes `Evento_ComprovanteEntrega`, `Evento_AtorInteressado`, `Eventos_RTC` do portal |
| Rótulos de CSOSN 103 e 203 na UI ("Isenção para MEI") | nenhum documento de descrição dos CSOSN | Manual de Orientação do Contribuinte, tabela CSOSN |
| Tabelas de apoio (NCM, CFOP, CEST, cBenef por UF, cClassTrib) | o conteúdo foi só amostrado | `tmp/tabelas/*` completas e a IT 2023.002 |
| NFC-e (QR Code, DANFE) | fora do escopo desta auditoria de NF-e 55 | `NT2025.001_v1.00 - NFCe_qrCode_3.pdf` e manual do DANFE NFC-e (em `~/Downloads`) |

## 7. Plano de correção sugerido

1. F-01, F-02, F-03, F-04, F-05 e F-06 (construtores), cada um com teste que reproduz o caso e passa o XSD; ampliar
   `TestBuildEnviNFe_ParidadeComXSD` com a varredura por CST usada nesta auditoria (preferível a teste de documento mínimo).
2. F-12 e F-13 no validador; só então promover enum, padrão, obrigatório e choice a block (F-14).
3. F-07 a F-10 e F-15 (API e UI com tabelas geradas do XSD); guia da UI atualizado na mesma alteração.
4. F-11 depois de ler as regras B09 e B10 no MOC.
5. Minors em lote com a F3 do plano consolidado.
