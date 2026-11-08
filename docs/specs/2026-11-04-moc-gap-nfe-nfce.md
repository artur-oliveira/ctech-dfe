# Auditoria MOC x implementação: NF-e (55) e NFC-e (65)

**Data:** 2026-11-04. **Tipo:** auditoria somente leitura (nenhum código, teste ou UI foi alterado).
**Escopo:** MOC online de NF-e/NFC-e (atualizado até a NT 2026.008 v1.01, publicada em 30/10/2026) contra o monorepo
`ctech-dfe` (`api/`, `worker/`, `go-dfe/`, `go-dfe-egress/`, `ui/`).
**Não repete:** o que já está nos planos `2026-08-26-cobertura-total-tags-nfe-mdfe.md`,
`2026-08-26-contingencia-e-inutilizacao-dfe.md`, `2026-08-27-cobertura-total-tags-implementacao.md` e
`2026-08-30-cobertura-total-tags-nfse-eventos-danfse.md`. Quando um item daqueles ainda está aberto, ele aparece aqui só
como referência.

---

## 0. Método, fontes e limites (leia antes dos números)

### 0.1 O que foi lido do MOC

O `WebFetch` falhou (`connect ECONNREFUSED 200.189.121.241:443`, tanto em http quanto em https). O mesmo host responde
em http via `curl`, então as páginas foram baixadas com `curl` para o scratchpad da sessão e convertidas para texto. O
site é uma cópia estática (HTTrack) e **cada página é um HTML único**; não existem subpáginas por seção. Páginas lidas
na íntegra (todas retornaram HTTP 200):

| Página | Conteúdo usado |
|---|---|
| `VisaoGeral.html` | conceitos, chave de acesso, CSRT, GTIN, cBenef, modalidades de emissão, EPEC, padrões XML, assinatura, lote, distribuição, consulta pública, tabelas (UF, município, país, SUFRAMA, RECOPI, NVE) |
| `Leiaute.html` | **1.203 linhas de leiaute** (grupos A a ZZ), extraídas por parser próprio |
| `RegrasDeValidação.html` | regras de validação, **1.019 identificadores, 912 com cStat** extraídos |
| `NFeAutorizacao`, `NFeRetAutorizacao`, `NFeInutilizacao`, `NFeConsultaProtocolo`, `NFeStatusServico`, `NFeConsultaCadastro`, `NFeDistribuicaoDFe`, `NFeConsultaGTIN` | serviços |
| `RecepcaoEventoGeral/Cancelamento/CartaCorrecao/Manifestacao/EPEC/Prorrogacao`, `RecepcaoAtorInteressado`, `RecepcaoComprovanteEntrega`, `RecepcaoCancelamentoComprovante`, `RecepcaoEventosIBSCBS`, `RecepcaoEventos_Simples_Nacional` | eventos |
| `DanfeCodigoBarras`, `DanfeQrCodeNFCe` | DANFE, DANFE NFC-e, QR Code |
| `Contingencia_NFe`, `ContingenciaNFCe` | contingência |
| `HistoricoDeImpla` | histórico das NTs e datas de vigência |

### 0.2 O que NÃO consegui ler ou não é confiável

- **Tabelas de apoio** (NCM, CFOP com indicadores, CEST, cBenef por UF, indicadores de CST do IBS/CBS, cClassTrib,
  municípios IBGE completos, países BACEN, ANP): são arquivos do Portal Nacional, não estão nesta cópia. As regras que
  dependem delas estão listadas, mas o conteúdo das tabelas não foi auditado.
- **Portal Nacional** (`nfe.fazenda.gov.br`, URLs dos web services SVC-AN/SVC-RS, URLs de QR Code por UF): redirecionamento
  em laço, não acessível. A tabela UF para SVC abaixo vem do texto do MOC (Ato COTEPE 39/2012) e **precisa ser confirmada**
  no Portal antes de virar código.
- **Layout do QR Code v3** (NT 2025.001): a página `DanfeQrCodeNFCe` só descreve a **versão 2**. A existência e as regras
  da v3 vêm das regras `ZX02-220` a `ZX02-338`; a ordem exata dos parâmetros v3 não está na página. Não inventei.
- **Texto das NTs**: o histórico traz só títulos. Em particular `NT 2023.002 v1.02` ("Eliminação da autorização
  assíncrona" e "Eliminação dos serviços de inutilização") precisa ser lida no Portal.
- A extração de regras tem margem de erro (tabelas HTML com células multilinha). Os totais são ordens de grandeza.

### 0.3 Como o repo foi comparado

- Matriz de tags: comparei **os nomes de tag** do leiaute com as chaves literais dos builders (`api/internal/services/nfes/*.go`,
  que montam `map[string]any`) e com `resp_tec.go`. Isso **superestima** a cobertura (um nome pode existir em outro pai) e
  **não valida** tipo, tamanho e decimais. Onde li o código para confirmar, citei `arquivo:linha`.
- Nenhum teste ou binário foi executado. Achados marcados "provável" vêm de leitura de código.
- Constatação estrutural que governa tudo: **XSD não é validado em lugar nenhum** (`go-dfe/CLAUDE.md`, "XSD validation
  and DANFE, explicitly out of scope"; `CGO_ENABLED=0`). A única validação antes da SEFAZ é a de DTO
  (`api/internal/validation`, go-playground/validator) sobre os **cadastros** e o corpo do request, e umas poucas regras
  de negócio em `emit.go`/`builders_*.go`. Nada valida o documento montado.

---

## 1. Resumo executivo

| Pergunta do dono do produto | Estado medido |
|---|---|
| (1) 100% das tags do leiaute | ~1.145 tags (sem `pk_*`/grupos de controle); **~96 nomes ausentes** nos builders, ~76 relevantes depois de descontar atributos e assinatura. Cobertura por nome ~91%. Descontando lacunas qualificadas por pai (ex.: `retirada/CEP`, `retirada/IE`) a cobertura real fica abaixo disso. O grosso do que falta é **reforma tributária recente (NT 2026.005/006/007/008)** e nichos (split payment, NFF, PAA) |
| (2) 100% dos fluxos de contingência | **Só `tpEmis=1` é emitido.** Existem `dhCont`/`xJust` e `tpEmis`/`tpImp` parametrizáveis (fase C0), mas `NormalEmission` é a única chamada (`emit.go:457`, `nfce_emit.go:137`). Sem endpoints SVC, sem EPEC, sem offline, sem máquina de estados, sem fila de pendentes |
| (3) DANFE em todos os formatos | 2 de 7 valores de `tpImp` (retrato `1` e NFC-e `4`). Faltam paisagem, simplificado, simplificado etiqueta, NFC-e mensagem eletrônica, simplificado tipo 2 |
| (4) Validação interna antes da SEFAZ | **Praticamente inexistente**: estimo menos de 5% das 912 regras com cStat (cerca de 40) têm checagem local. Nenhuma de casas decimais por tag, tamanho por tag, IE por UF, GTIN, NCM/CFOP/CEST, somatórios de total, CST x campos |

### 1.1 Achados que merecem ação imediata (prováveis defeitos hoje)

1. **`xsdorder` com nome errado:** `go-dfe/internal/xmlops/xsdorder/table.go:161` lista `"ISSQNcalc"` em `imposto`, mas a tag
   é `ISSQN` (o builder emite `"ISSQN"`, `builders_tax.go:638`). Tag fora da tabela vai para o **fim** (ordem alfabética
   depois das conhecidas, `builder.go` bloco `sort.Slice`), então `ISSQN` sairia depois de `PIS`/`COFINS`/`IBSCBS`:
   XML fora de ordem em item de serviço (NF-e mista). Provável; confirmar com teste golden de item ISSQN.
2. **Tags emitidas e ausentes da tabela de ordem** (caem no fim do nó, fora de ordem): `ide/cIndOp`
   (`builders_ide.go:79`), `det/impostoDevol` e `det/infAdProd` (`builders_doc.go:549,565`; a tabela de `det` é só
   `prod, imposto, obsItem`, `table.go:91`). `impostoDevol` com `obsItem` na mesma linha sai invertido.
3. **`total/ICMSTot/vNF` diverge da regra W16-10 (cStat 610)** (`builders_total.go:74`): não subtrai `vICMSDeson`, não soma
   `vFCPST`, `vICMSMonoReten`, `vIPIDevol`, `vServ` (ISSQN), nem `vPIS`/`vCOFINS` de PISST/COFINSST quando
   `indSomaPISST`/`indSomaCOFINSST`=1. Hoje só soma `vFrete+vSeg+vOutro+vIPI+vST+vII`.
4. **`d()` engole erro de decimal** (`nfes/v.go:11`): `decimal.NewFromString` falha, grava `slog.Warn` e devolve zero. Um
   valor malformado vira `0.00` calado no XML em vez de erro.
5. **`retirada`/`entrega` sem `CEP` e sem `IE`** (`builders_party.go:101-134`; a tabela `table.go:76-85` também não tem
   as duas). `cPais` fixo `1058`: local no exterior (`UF=EX`, `cMun=9999999`) impossível.
6. **`cMun` com fallback `"0000000"`** (`builders_party.go:46-67`): cadastro incompleto vira rejeição 272/274 da SEFAZ em vez
   de erro local claro.
7. **NF-e não valida pagamento contra total** (rejeições 865/866/869): `emit.go` não confere `Σ vPag` com `vNF` nem
   `vTroco`. NFC-e calcula troco (`nfce_emit.go:200-204`) mas não rejeita pago a menos.
8. **`FinNFe` aceita só 1 a 4** (`emit.go:~40`, `oneof=1 2 3 4`): nota de crédito (5) e de débito (6) da reforma só passam
   por operação, não pelo request; regras B25-xx não são verificadas.
9. **Destinatário no exterior impossível**: `idDest` só `1` ou `2` (`builders_doc.go:236-242`), `indIEDest` só `1` ou `9`
   (`builders_party.go:227-241`; `2`=isento ausente), `enderDest/cPais` sempre `1058`, `UF=EX` nunca. Rejeições 720, 721,
   727, 510, 511, 805.
10. **Limites de tamanho do cadastro maiores que o do leiaute**: nome de pessoa `max=255` (xNome 2-60), logradouro
    `max=255` (xLgr 2-60), bairro `max=120` (xBairro 2-60), cidade `max=120` (xMun 2-60), descrição de produto `max=255`
    (xProd 1-120), IE `max=20` (IE 2-14, `dto.go:27-39,166,993`). Só `natOp` é truncado (`builders_extra.go:25`). O
    resto estoura na SEFAZ (falha de schema, cStat 225).
11. **NT 2023.002 v1.02**: o histórico lista "Eliminação da autorização assíncrona" (produção 05/04/2027) e "Eliminação
    dos serviços de inutilização" (**produção 11/11/2026, 7 dias**). A inutilização acabou de ser entregue
    (`inutilization.go`) e o plano de contingência a usa para "pendentes de retorno". Confirmar o texto da NT hoje. A
    parte assíncrona não afeta: `indSinc=1` fixo (`builders_doc.go:58`).
12. **NFC-e offline v2 exige o `DigestValue` da assinatura no QR Code** (tabela 3 do MOC, parâmetro 6). Hoje o QR é
    montado na API antes da assinatura (`nfce_qrcode.go:100-135`, o próprio comentário diz que não depende da assinatura).
    Para `tpEmis=9` é uma mudança de arquitetura, não de campo.

---

## 2. Parte A: matriz de cobertura de tags

Legenda: **C** coberta; **P** parcial; **A** ausente. `Ocor.` é a ocorrência do leiaute; `Tipo/Tam.` é N numérico, C
caractere, D data, `13v2` = até 13 dígitos com 2 decimais, `3v2-4` = 2 a 4 decimais, `11v0-4` = 0 a 4 decimais.
Contagens são de **nomes de tag** (ver 0.3). Linhas `arquivo:linha` apontam a função que monta o grupo.

### 2.1 Visão por grupo (modelos 55 e 65)

| Grupo MOC | Tags | Nomes ausentes | Status | Onde está (`api/internal/services/nfes/`) |
|---|---|---|---|---|
| A infNFe (`versao`, `Id`) | 3 | atributos | C | `builders_doc.go:189` (`BuildEnviNFe`) |
| B ide | 34 | 0 | C (P na ordem e nos valores) | `builders_ide.go:45`; ver 1.1 itens 2, 8 |
| BA NFref | 24 | 0 | C | `references.go:33` |
| BB gCompraGov | 5 | 0 | C | `builders_ide.go:178` |
| BC gPagAntecipado | 2 | 0 | C | `builders_ide.go:~100` |
| C emit | 24 | 1 (`tpRegimeIBSCBS`, C21a) | P | `builders_party.go:151` |
| D avulsa (Fisco) | 12 | 8 | A, não aplicável (NFA-e do Fisco) | n/a |
| E dest | 22 | 1 (`ISUF`, E18) | P | `builders_party.go:211`; exterior/isento ausentes |
| F retirada | 17 | 0 por nome | **P** (sem `CEP`, `IE`, exterior) | `builders_party.go:101` |
| G entrega | 17 | 0 por nome | **P** (idem) | `builders_party.go:101` |
| GA autXML | 3 | 0 | C | `builders_party.go:69` |
| H det | 2 | `nItem` (atributo) | C | `builders_doc.go:~540-570` |
| I prod | 67 | 3 (`vUnComLiq`, `vProdLiq`, `cAtivSN`) | P | `builders_prod.go:25` |
| I01 DI, I03 detExport, I05 xPed, I07, I80 rastro, I86 infProdNFF, infProdEmb | 40 | 0 | C | `builders_prod.go:196,247`, `rastro.go:32` |
| J veicProd | 25 | 0 | C | `builders_prod.go:334` |
| K med | 8 | 0 | C | `builders_prod.go:357` |
| L arma | 5 | 0 | C | `builders_prod.go:131` |
| LA comb | 26 | 1 (`pMixGN`, já excluído do leiaute 4.00) | C | `comb.go:29` |
| LB RECOPI | 1 | 0 | C | `builders_prod.go` (nRECOPI) |
| N ICMS (N02 a N10h) | 296 | 10 | P | `builders_tax.go:41` (SN), `:125` (normal) |
| NA ICMSUFDest | 10 | 0 | C | `builders_tax.go:651` |
| NB ICMSPrevistoPagtoAntecip | 2 | 2 | **A** (NT 2026.008, produção 01/03/2027) | n/a |
| O IPI | 15 | 1 (`clEnq`, legado) | C | `builders_tax.go:422` |
| P II | 5 | 0 | C | `builders_prod.go:272` |
| Q PIS / R PISST | 27 | 1 (`indSomaPISST`) | P | `builders_tax.go:534,482` |
| S COFINS / T COFINSST | 27 | 1 (`indSomaCOFINSST`) | P | `builders_tax.go:561,486` |
| U ISSQN | 17 | 0 | C (**ordem XSD quebrada**, 1.1 item 1) | `builders_tax.go:588` |
| UA impostoDevol | 4 | 0 | C | `builders_total.go:196` |
| UB IBS/CBS/IS | ~207 | 34 | P (monofasia nova, Simples Nacional) | `builders_ibscbs.go:132`, `builders_tax.go:492` (IS) |
| V infAdProd, VA obsItem | 4 | `xCampo` (atributo) | C | `builders_doc.go:549`, `builders_transp.go:224` |
| VB vItem | 1 | 1 | **A** | n/a |
| VC DFeReferenciado (item) | 3 | 3 | **A** (devolução e crédito por item) | n/a |
| W total (ICMSTot, ISSQNtot, retTrib, IBSCBSTot, ISTot, vNFTot) | 89 | 4 | P (cálculo de `vNF` divergente, 1.1 item 3) | `builders_total.go:73,156,213` |
| X transp | 37 | 3 (`vagao`, `balsa`, `nLacre`) | P | `builders_transp.go:19,151,181,199` |
| Y cobr | 11 | 0 | C | `builders_doc.go:151` |
| YA pag | 18 | 0 | C (sem validação cruzada) | `builders_doc.go:91` |
| YB infIntermed | 3 | 0 | C | `builders_ide.go:136` |
| YC gPgtoVinc (split payment) | 7 | 6 | **A** (evento 110300 correlato também ausente) | n/a |
| Z infAdic | 13 | `xCampo` (atributo) | C | `builders_extra.go:84,108` |
| ZA exporta | 4 | 0 | C | `builders_extra.go:127` |
| ZB compra | 4 | 0 | C | `builders_extra.go:175` |
| ZC cana | 15 | 0 | C | `builders_extra.go:199` |
| ZD infRespTec | 8 | 0 | C | `api/internal/services/resp_tec.go:25` |
| ZE infSolicNFF | 2 | 2 | **A** (NFF, `tpEmis=3` desativado) | n/a |
| ZF agropecuario | 9 | 0 | C | `builders_extra.go:252` |
| ZG infPAA | 7 | 7 | **A** (Provedor de Assinatura e Autorização, `procEmi=4`) | n/a |
| ZX infNFeSupl | 3 | 0 | P (QR v2 online só) | `nfce_qrcode.go:100` |
| ZZ Signature | 1 | n/a | C (feita no go-dfe) | `go-dfe/internal/xmlops/signer.go` |

### 2.2 Tags ausentes ou parciais com caminho, ocorrência, tipo e tamanho

Fonte: leiaute do MOC. Os caminhos dos subgrupos de monofasia do IBS/CBS (UB85a a UB104b) foram inferidos pela coluna "Pai"
do leiaute e têm imprecisão; revalidar contra o XSD `PL_010e`/posterior no momento da implementação.

| ID MOC | Caminho XML | Ocor. | Tipo/Tam. |
|---|---|---|---|
| C21a | infNFe/emit/tpRegimeIBSCBS | 0-1 | N 1 |
| E18 | infNFe/dest/ISUF | 0-1 | N 8 - 9 |
| I11b | vUnComLiq | 1-1 | N 11v0-10 |
| I11c | vProdLiq | 1-1 | N 13v2 |
| I17d | infNFe/det/prod/cAtivSN | 0-1 | C 2 |
| LA03 | infNFe/det/prod/comb/pMixGN | 0-1 | N 2v4 |
| N14a | infNFe/det/imposto/ICMS/ICMS51/cBenefRBC | 0-1 | C 8,10 |
| N41a | infNFe/det/imposto/ICMS/ICMS53/qBCMonoDif | 0-1 | N 11v0-4 |
| N42 | infNFe/det/imposto/ICMS/ICMS53/adRemICMSDif | 0-1 | N 3v2-4 |
| N26b | infNFe/det/imposto/ICMS/ICMS60/vICMSSubstituto | 0-1 | N 13v2 |
| NB01 | infNFe/det/imposto/ICMS/ICMSPrevistoPagtoAntecip | 1-1 |   |
| NB02 | infNFe/det/imposto/ICMS/ICMSPrevistoPagtoAntecip/vICMSPrevisto | 1-1 | N 13v2 |
| O02 | infNFe/det/imposto/IPI/clEnq | 0-1 | C 1 - 5 |
| R07 | infNFe/det/imposto/PISST/indSomaPISST | 0-1 | N 1 |
| T07 | infNFe/det/imposto/COFINSST/indSomaCOFINSST | 0-1 | N 1 |
| UB85a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoPadrao/qBCMono/gIBSMonoAdRem | 0-1 | CG  |
| UB89 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoPadrao/qBCMono/gIBSMonoAdRem/gpBioDiferenca | 0-1 | G  |
| UB89a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoPadrao/vCBSMono/qBCBioComb | 1-1 | N 11v0-4 |
| UB89b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoPadrao/vCBSMono/vIBSDiferenca | 1-1 | N 13v2 |
| UB90 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/gIBSMonoAdValorem | 0-1 | G  |
| UB91a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/qBCMonoReten/vBCMono | 1-1 | N 13v2 |
| UB91b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/qBCMonoReten/pAliqMonoUF | 1-1 | N 3v2-4 |
| UB91c | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/qBCMonoReten/vIBSMonoUF | 1-1 | N 13v2 |
| UB91d | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/qBCMonoReten/pAliqMonoMun | 1-1 | N 3v2-4 |
| UB91e | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/qBCMonoReten/vIBSMonoMun | 1-1 | N 13v2 |
| UB92a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/adRemIBSReten/vBCMonoReten | 1-1 | N 13v2 |
| UB92b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/adRemIBSReten/pAliqMonoReten | 1-1 | N 3v2-4 |
| UB94 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoReten/gpBioDiferenca | 0-1 | G  |
| UB94a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoRet/qBCBioComb | 1-1 | N 11v0-4 |
| UB94b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoRet/vIBSDiferenca | 1-1 | N 13v2 |
| UB95a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoRet/qBCMonoRet/gCBSMonoAdRem | 0-1 | CG  |
| UB99 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoRet/qBCMonoRet/gCBSMonoAdRem/gpBioDiferenca | 0-1 | G  |
| UB99a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/qBCBioComb | 1-1 | N 11v0-4 |
| UB99b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/vCBSDiferenca | 1-1 | N 13v2 |
| UB100 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoRet/qBCMonoRet/gCBSMonoAdValorem | 0-1 | CG  |
| UB101a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/vIBSMonoDif/vBCMono | 1-1 | N 13v2 |
| UB101b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/vIBSMonoDif/pAliqMonoCBS | 1-1 | N 3v2-4 |
| UB102a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/pDifCBS/vBCMonoReten | 1-1 | N 13v2 |
| UB102b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/pDifCBS/pAliqMonoReten | 1-1 | N 3v2-4 |
| UB104 | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/pDifIBS/gpBioDiferenca | 0-1 | G  |
| UB104a | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/pDifIBS/gpBioDiferenca/qBCBioComb | 1-1 | N 11v0-4 |
| UB104b | infNFe/det/imposto/IBSCBS/gIBSCBSMono/gMonoDif/pDifIBS/gpBioDiferenca/vCBSDiferenca | 1-1 | N 13v2 |
| UB115a | infNFe/det/imposto/IBSCBS/gTribSN | 0-1 | -  |
| UB115b | infNFe/det/imposto/IBSCBS/gTribSN/gIBSSN | 0-1 | -  |
| UB115c | infNFe/det/imposto/IBSCBS/gTribSN/gIBSSN/pIBSSN | 1-1 | N 3v2-4 |
| UB115d | infNFe/det/imposto/IBSCBS/gTribSN/gIBSSN/vIBSSN | 1-1 | N 13v2 |
| UB115e | infNFe/det/imposto/IBSCBS/gTribSN/gCBSSN | 1-1 | -  |
| UB115f | infNFe/det/imposto/IBSCBS/gTribSN/gCBSSN/pCBSSN | 1-1 | N 3v2-4 |
| UB115g | infNFe/det/imposto/IBSCBS/gTribSN/gCBSSN/vCBSSN | 1-1 | N 13v2 |
| VB01 | infNFe/det/vItem | 0-1 | N 13v2 |
| VC01 | infNFe/det/DFeReferenciado | 0-1 |   |
| VC02 | infNFe/det/DFeReferenciado/chaveAcesso | 1-1 | C 44 |
| W01a | infNFe/total/vProdLiqTot | 0-1 | N 13v2 |
| W59h | infNFe/total/IBSCBSTot/gTribSN | 0-1 | -  |
| W59i | infNFe/total/IBSCBSTot/gTribSN/vIBSSN | 1-1 | N 13v2 |
| W59j | infNFe/total/IBSCBSTot/gTribSN/vCBSSN | 1-1 | N 13v2 |
| X25a | infNFe/transp/vagao | 0-1 | C 1 - 20 |
| X25b | infNFe/transp/balsa | 0-1 | C 1 - 20 |
| X34 | infNFe/transp/vol/lacres/nLacre | 1-1 | C 1 - 60 |
| YC01 | infNFe/gPgtoVinc | 0-1 | - - |
| YC02 | infNFe/gPgtoVinc/gPgto | 1-99 | - - |
| YC03 | infNFe/gPgtoVinc/gPgto/nPag | 1-1 | N 3 |
| YC04 | infNFe/gPgtoVinc/gPgto/idTransacao | 1-1 | C 2-35 |
| YC05 | infNFe/gPgtoVinc/gPgto/tpMeioPgto | 1-1 | N 2 |
| YC07 | infNFe/gPgtoVinc/gPgto/CNPJBasePSP | 1-1 | C 8 |
| ZE01 | infNFe/infSolicNFF | 0-1 |   |
| ZE02 | infNFe/infSolicNFF/xSolic | 1-1 | C 2-5000 |
| ZG01 | infNFe/infPAA | 0-1 |   |
| ZG02 | infNFe/infPAA/CNPJPAA | 1-1 | C 14 |
| ZG03 | infNFe/infPAA/PAASignature | 1-1 |   |
| ZG04 | infNFe/infPAA/PAASignature/SignatureValue | 1-1 | C  |
| ZG05 | infNFe/infPAA/PAASignature/RSAKeyValue | 1-1 |   |
| ZG06 | infNFe/infPAA/PAASignature/RSAKeyValue/Modulus | 1-1 | C  |
| ZG07 | infNFe/infPAA/PAASignature/RSAKeyValue/Exponent | 1-1 | C  |

Ausências qualificadas por pai (o nome existe no repo em outro lugar, mas **não** neste nó):

| ID | Caminho | Ocor. | Tipo/Tam. | Situação |
|---|---|---|---|---|
| F13/F14 | `retirada/CEP` | 0-1 | N 8 | A: `builders_party.go:101` não emite; `table.go:76` não lista |
| F15 | `retirada/IE` | 0-1 | C 2-14 | A (idem); regra F15-10 (971) |
| G13/G14 | `entrega/CEP`, `entrega/IE` | 0-1 | N 8, C 2-14 | A (idem); G15-10 (971) |
| B25d | `ide/cIndOp` | 0-1 | N 6 | P: emitido (`builders_ide.go:79`), mas **fora da tabela de ordem** |
| E16a | `dest/indIEDest = 2` | 1-1 | N 1 | P: só `1` e `9` (`builders_party.go:227-241`) |
| E14 | `dest/enderDest/cPais, xPais` != 1058, `UF=EX`, `cMun=9999999` | 1-1 | N 4 / C | A: `buildEnder` fixa Brasil (`builders_party.go:46-67`) |
| B11a | `ide/idDest = 3` | 1-1 | N 1 | A (`builders_doc.go:236-242`) |
| X25a/X25b | `transp/vagao`, `transp/balsa` | 0-1 | C 1-20 | A (choice com `veicTransp`) |
| N14a | `ICMS51/cBenefRBC` e `ICMS20/70/90/cBenefRBC` | 0-1 | C 8 ou 10 | A (NT 2023.001+) |
| N41a/N42 | `ICMS53/qBCMonoDif`, `adRemICMSDif` | 0-1 | N 11v0-4, 3v2-4 | A: `builders_tax.go:349-368` só calcula `vICMSMonoDif` |
| N26b | `ICMS60/vICMSSubstituto` (0-1), `ICMSSN500/vICMSSubstituto` (1-1) | | N 13v2 | A |

### 2.3 Diferenças NF-e (55) x NFC-e (65) no leiaute

| Grupo | NF-e 55 | NFC-e 65 | Estado no repo |
|---|---|---|---|
| `dest` | identificação completa, `enderDest`, `IE`, `indIEDest` 1/2/9 | só CPF/CNPJ/`idEstrangeiro`, `xNome` opcional (obrigatório se valor > R$ 10.000, W16-50 opcional por UF), endereço só em entrega a domicílio (`indPres=4`, B25b-50 1004), `indIEDest=9` fixo (E16a-10 789) | NFC-e: **só CPF** (`nfce_emit.go:44-60`); sem CNPJ, sem `idEstrangeiro`, sem nome/endereço (entrega a domicílio ausente) |
| `transp` | completo | só com entrega a domicílio (X03-20 786, X03-10 754) | NFC-e não emite `transp` |
| `retirada`/`entrega`/`cobr`/`exporta`/`compra`/`cana`/`veicProd`/`DI`/`detExport`/`ICMSUFDest` | permitidos | proibidos (ZA01-30 814, NA01-10 807 e família) | NFC-e não monta (correto), mas **não há trava** se o produto traz o dado |
| `finNFe`, `indFinal`, `indPres` | 1-6, 0/1, 0-5/9 | fixos 1, 1, 1 (`nfce_emit.go:34-38`) (ver `indPres` 4 e 5 abaixo) | NFC-e: `indPres=4` (entrega) e `5` (fora do estabelecimento, B25b-70 1005) ausentes |
| `tpImp` | 1, 2, 3, 6 (0 sem DANFE) | 4, 5, 6 | só 1 e 4 |
| `tpEmis` | 1, 4 (EPEC), 5 (FS-DA), 6, 7 (SVC); 9 só com `tpImp=6` | 1, 4 (a critério da UF), 9 | só 1 |
| `infNFeSupl` (qrCode, urlChave) | v3 (regra ZX02-220 foi removida na NT 2025.001 v1.01; NF-e 55 só entra em `tpImp=6`) | v2 (e v3) obrigatório | só v2 online, NFC-e |
| `pag/detPag` | `tPag` incl. 90 e 91 | idem, com troco | OK; sem validação |
| IPI, II, ISSQN | permitidos | NFC-e: ISSQN permitido em item de serviço; II/IPI não usuais | sem trava |

---

## 3. Parte B: regras de validação do MOC que **não** são pré-validadas localmente

### 3.1 O que existe de validação local (inventário completo)

Foi inventariado por leitura. Tudo o que não está aqui deve ser considerado **não validado**.

| Camada | O que valida | Onde |
|---|---|---|
| DTO do request (`validate:`) | formato de `cfop` (4 dígitos), `ncm` (8), `cest` (7), `ibge` (7 dígitos, sem tabela), `cep`, `cean` (8 a 14 dígitos ou "SEM GTIN"), `cbenef`, `serie`, `percent`, `money` (até 4 decimais), `money2` (até 2), `decimalv`, CPF/CNPJ com dígito, UF, chave de acesso com DV (`validation/access_key.go`) | `api/internal/validation/validators.go`, `api/internal/api/v1/dto.go` |
| Emissão NF-e/NFC-e | pelo menos um item e um pagamento; `finNFe` 2/3/4 exige `NFref`; `p_devol` só com `finNFe=4`; NFC-e só CFOP 5xxx e CPF; `dhCont`/`xJust` >= 15 caracteres em contingência; medicamento (`vPMC`, `xMotivoIsencao`); encerrante final >= inicial; cana; agropecuário; compra governamental (`tpOperGov` x `refDFeAnt`) | `emit.go:352-602`, `nfce_emit.go`, `emission_mode.go:51`, `builders_prod.go:348-371`, `comb.go:126`, `builders_extra.go`, `builders_ide.go:178` |
| Estrutura | `natOp` truncado a 60; DV da chave (mod 11) gerado | `builders_extra.go:25`, `emit.go:926` |
| XML | nada (sem XSD) | n/a |

### 3.2 Regras ausentes, por prioridade

Prioridade = frequência provável de rejeição x custo para o cliente (estimativa minha, sem dados de produção). Formato:
**ID MOC (cStat)** modelos. "Obrig." = a SEFAZ sempre aplica; "Facult." = a critério da UF.

**P0 (aparecem em quase toda implementação nova)**

1. **Totais e cálculo** (as mais frequentes). `W16-10 (610)` 55/65 vNF (ver 1.1 item 3); `W03-10 (531)`, `W04-10 (532)`,
   `W04a-10 (795)`, `W04b-10 (861)`, `W05-10 (533)`, `W06-10 (534)`, `W06a-10 (862)`, `W07-10 (564)`, `W08-10 (535)`,
   `W09-10 (536)`, `W10-10 (537)`, `W11-10 (601)`, `W12-10 (538)`, `W13-10 (602)`, `W14-10 (603)`, `W15-10 (604)`,
   `W18` a `W22f (605-609, 364-368)` (ISSQN); item: `I11-10 (629)` e `I11-20 (630)` (vProd = qCom x vUnCom, tolerância 0,01),
   `I17-10 (483)` (vDesc <= vProd), `N17-20 (528)` (vICMS = vBC x pICMS, tolerância 0,01), `N16a-10/N16c-10/N17-10 (351-353)`
   (CST 51), `N17c-10 (860)`, `N23d-10`, `N27d-10`. Os totais derivam dos itens por construção, então o risco real está
   nas **fórmulas** (vNF) e no arredondamento (ver 3.3).
2. **Inscrição Estadual por UF:** `C17-20 (209)`, `E17-50 (210)`, `F15-10/G15-10 (971)`, `X07-20 (544)`, `C18-30 (211)`; e
   coerência `indIEDest` x IE: `E17-20 (728)`, `E17-30 (791)`, `E17-40 (792)`, `E16a-10/20 (789/790)`, `E16a-30 (805,
   lista de UF)`, `E16a-40 (696)`, `E17-10 (729)`. Exige os 27 algoritmos de DV de IE.
3. **Tabelas oficiais:** NCM `I05-10 (777)`, `I05-20 (778)`, `I05-24 (471)`, `I05-30 (779, NFC-e)`; CFOP existente e permitido
   `I08-04 (770)`; CFOP x `tpNF`/`idDest`/UF `I08-10..70 (518-521, 731-733)`, `I08-90 (523)`, `I08-140/144 (327/328, devolução)`;
   CEST `I05c-10 (446)`, `N23-10 (806)`; **GTIN** (DV `I03-10/I12-10 (611/612)`, prefixo GS1 `I03-20/I12-20 (882/884)`,
   vazio `I03-30/I12-60 (883/888)`, par cEAN x cEANTrib `I12-30/40 (885/886)`); `uCom`/`uTrib` NFC-e `I09-10/I13-10
   (734/735)`; cBenef `N12-84..98 (928-946)`; FCP `N17b/N23b/N27b (874-881)`; ANP `LA02-10 (761)`;
   município `B12-10/20 (270/271)`, `C10-10/20 (272/273)`, `E10-10..30 (274, 275, 509)`, `F07/G07 (276-279, 513, 515)`,
   `U05-10 (287)`, `U14-10 (389)`, `X17-10 (288)`; país `E14-04/10/20 (377, 510, 511)`; SUFRAMA com DV `E18-20 (235)`,
   `C22-20 (1186)`.
4. **CST x campos obrigatórios:** `N11-10 (966)` (origem), `N12-20 (590)` e `N12a-10 (591)` (CST vs CRT), `N12-10 (527)`
   (exportação), `N12-70 (508)` e `N12a-70 (600)` (não contribuinte), `N12-80 (529)` (isento), `N12-81/N12a-50 (938)`,
   `N07-10/N12-97 (929)`, `N12-90 (934)` (desoneração), `N18-10/20 (932/933)` (`pMVAST`), `N28-10..30 (625-627)`
   (desoneração Suframa), monofasia `N38-10..N45-10 (959-966, 768, 769)`, `O06-10 (387)`/`O09-10 (388)` (`cEnq` x CST do IPI),
   `U01-10..30 (530, 592, 887)`.
5. **Datas e fuso:** `B09-10 (703)` (emissão posterior à recepção, tolerância 5 min), `B09-20 (228)` (mais de 30 dias,
   NF-e normal), `B09-30/50 (315)`, `B10-20/30 (504/505)`, `B10-40 (506)`, `B10a-10..50 (1153-1157)`, `B28-10..40
   (556-558, 569)`. O repo gera `dhEmi` com `time.Now()` e `Format(...-07:00)` (`emit.go:1576`, `nfce_emit.go`): o offset sai
   do fuso do processo (Lambda costuma ser UTC), não do fuso do **emitente**. Confirmar `TZ` e o fuso da UF do emitente.
6. **Identificação e chave:** `A03-10 (502)`, `B23-10 (253)` (DV, por construção ok), `B03-10 (897)`: o `cNF` pode ser
   igual a `nNF` ou a padrões proibidos (`11111111`, `12345678`...). `generateAccessKey` sorteia de 10.000.000 a 99.999.999
   com `math/rand` (`emit.go:~920`), sem checar; probabilidade pequena mas não zero. `B24-10 (252)`, série por processo
   `B26-10/20 (244/451)`, `C02-30/C02a-10 (503/495)`.
7. **Pagamento e cobrança:** `YA03-10 (865)`/`YA03-20 (866)`/`YA09-10 (869)`/`YA09-20 (965)`, `YA04-10 (391)`, `YA04-20 (963)`,
   `YA02-04 (871)`, `YA03-30 (904)`, `YA03a-10 (657)`, `YA03c-10 (961)`, cobrança `Y01-20 (905)`, `Y05-10 (901)`, `Y06-10
   (902)`, `Y08-10 (852)`, `Y09-30/40/50 (850, 853, 797)`, `Y10-10 (851, Σ dup = vLiq)`.
8. **Finalidade e referenciados (B25 e BA):** `B25-20 (715)`, `B25-30/40/50/60 (254/255/269/678)`, `B25-70 (321)`,
   `B25-80 (1001)`, `B25-90 (1002)`, `B25-100 (1003)`, `B25-110 (1102/1161)`, `B25-120 (1162)`, `B25.1/B25.2 (1139, 1009,
   1163, 1164, 1152)`, `B25b-10/20/40/50/60/70 (794, 717, 864, 1004, 1000, 1005)`, `B25c-10/20 (434/435)`, `B25d (1099, 1102, 1110)`,
   `BA02-xx`, `BA10`, `BA19`, `BA20` (34 regras BA). Hoje só "finNFe 2/3/4 exige NFref" (`emit.go:602`).
9. **Operação com exterior e destinatário:** `E03a-10/20/30/60 (720, 721, 925, 372)`, `E12-10..60 (727, 772, 773)`, `E01-10 (719)`,
   `E02-20 (220, NFC-e)`, `E03-10 (237)`.

**P1 (frequentes em nichos ou em UF específica)**

10. **Transporte:** `X03-30 (845)`, `X04-30..100 (846-849)` (CNPJ base da transportadora conforme `modFrete`), `X05-10 (543)`,
    `X07-10/20 (559/544)`, `X16-10 (722)`.
11. **Combustível:** `LA01-20 (660)`, `LA01-30 (1106)`, `LA03c-10/20 (461, 855)`, `LA03d-10 (856)`, `LA11-10 (378)`,
    `LA11-20 (379)`, `LA16-10 (380)`, `LA17-10/20 (907/908)`, `LA18-10..30 (909, 747)`, `LA21-10/20 (958)`, `N12-100/110 (959/960)`.
12. **Importação/exportação:** `I08-110/120/130 (525, 597, 599)`, `I19-10 (329)`, `I23-10 (376)`, `I23b..e (330-333)`, `I29a (334/335)`,
    `I50-10 (336)`, `I51 (338/339)`, `I52..I55 (340-346)`, `ZA01-10/20 (355/356)`.
13. **Veículos, medicamentos, armas, papel imune:** `J19-10 (841)`, `J20-10/20 (842/843)`, `K01-10/20`, `LB01-20/30 (349/350)`.
14. **Locais:** `F02/F02a (512/540)`, `G02/G02a (514/541)`, `F11/G11 (970)`, `GA02/GA03 (323, 325, 326)`.
15. **Identificação do emitente na SEFAZ** (não é pré-validável sem cadastro): `1C17-xx (230, 231, 622, 203, 781, 301, 307)`,
    `1C03-10 (935)`. Valor local: ler a situação cadastral via `NfeConsultaCadastro` (já implementado em go-dfe) **antes** de
    emitir e avisar.
16. **Cadastro técnico:** `ZD01-10/ZD02-10/ZD07-10 (972, 973, 975)` (CSRT; o `idCSRT`/`hashCSRT` já é emitido), `N12-30..44` etc.

**P2 (reforma tributária e DANFE simplificado tipo 2)**

17. **IBS/CBS/IS (grupos UB, VB, VC, W31 a W60, NB, B25-xx débito/crédito):** 180 regras UB com cStat, mais 24 regras de
    totais W31 a W59j. Vigências do histórico: NT 2026.005/006/007 em homologação até 26/10/2026 e **produção 16/11/2026**;
    NT 2026.008 em homologação até 05/10 e produção 03/11/2026 (campos líquidos, `ICMSPrevistoPagtoAntecip`, `vProdLiqTot`),
    com `NB01-30` e partes em 01/02 a 01/03/2027. Ou seja, **a janela de vigência abre em 12 dias e o repo não tem
    `gTribSN`, `vItem`, `vProdLiq`, `ICMSPrevistoPagtoAntecip`, `DFeReferenciado`, `gPgtoVinc`**.
18. **DANFE Simplificado Tipo 2 (`tpImp=6`, Ajuste SINIEF 13/26):** 66 regras com `tpImp=6` (`B09-40 (704)`, `B10-10 (705)`,
    `B11-10 (706)`, `B11a-10 (707)`, `B25-20 (715)`, `B25a-10 (716)`, `B25b-20 (717)`, `BA01-10 (708)`, `C18-10 (718)`, `E01-20 (787)`,
    `F01-10 (669)`, `I08-150 (725)`, `I17b-10 (774)`, `J01-10 (736)`, `L01-10 (738)`, `LB01-10 (348)`, `N12-30..60`).
19. **QR Code (ZX01 e ZX02, 56 regras):** v2 online/offline, v3 online/offline com `assinatura` (`ZX02-324..338`), `ZX03-20 (878)`.
20. **Envelope e mensagem (D01..D02, E, F, lote):** `D01d (587)`, `D01e (588)`, `D02 (404)`, `B02-20 (476)`, `B06-20 (765)`,
    `indSinc` (`NFeAutorizacao`: lote de NFC-e só com 1 documento; resposta assíncrona rejeitada). O builder fixa `indSinc=1` e
    o lote tem 1 nota, então hoje ok.

### 3.3 Casas decimais, arredondamento, tamanho e formato

Padrões de `Tam.` do leiaute (520 campos numéricos com decimais, extraídos): `13v2` (315 campos: valores monetários e
bases), `3v2-4` (141: alíquotas e percentuais, 2 a 4 decimais), `11v0-4` (27: quantidades, 0 a 4), `12v0-4` (8), `3v4` (5),
`12v3` (4: pesos e encerrante), `11v10` (4: cana), `11v0-10` (`vUnCom`, `vUnTrib`, `vUnComLiq`: 0 a 10 decimais), `9v4`, `11v4`,
`8v3`, `3v2`, `2v4`, `2v2`.

| Tema | Regra do MOC | Estado no repo |
|---|---|---|
| Casas por campo | cada tag tem `v` próprio (lista acima); fora do padrão = falha de schema (225) | **não validado**; builders usam `q2` e `q4` fixos (`nfes/v.go:20,22`); `vUnCom` com `money` (até 4 decimais, `validators.go:44`) é mais estreito que 0 a 10 (seguro, mas limita o produto) |
| Arredondamento | "arredondado para duas casas", **tolerância R$ 0,01** em `N17-20`, `I11-10`, `N16-*`; `pAliqEfet` com 4 casas "com arredondamento na última" | usa `RoundBank` (bancário) em todo o código (`builders_tax.go`, `builders_total.go`, `builders_ibscbs.go`). Dentro da tolerância de 0,01 o resultado passa; decidir uma política única e testar valores em `.xx5` |
| Tolerâncias | 0,01 (maioria), 0,04 (uma regra), 0,1% (`I17b/N`) | sem constante nomeada de tolerância |
| Valor parse | número válido | `d()` zera silenciosamente (1.1 item 4) |
| Tamanho | `xNome` 2-60, `xLgr` 2-60, `xProd` 1-120, `infCpl` 1-5000, `infAdFisco` 1-2000, `nNF` 1-9, `IE` 2-14, etc. | só `natOp`; cadastro permite mais (1.1 item 10) |
| Formato texto | caracteres de edição proibidos no início, fim e entre tags (`D01e 588`); sem espaços nas pontas e sem espaços duplos (TString do XSD); acentos e `&<>"'` tratados (`4.2.1.5`) | escape de `&<>` em `escapeText` (`builder.go:249`); trim/duplo espaço **não** garantidos |
| Obrigatoriedade condicional | ver 3.2 itens 4 a 9 | não validada |
| Data/hora | `AAAA-MM-DDThh:mm:ssTZD` | formato correto (`fmtDhEmi`), fuso do processo (3.2 item 5) |

---

## 4. Parte C: contingência

### 4.1 Mapa `tpEmis`

| `tpEmis` | Significado (MOC) | Modelos | Estado no repo | O que falta |
|---|---|---|---|---|
| 1 | Normal | 55, 65 | feito | n/a |
| 2 | FS-IA (formulário de segurança, impressor autônomo) | 55 | ausente | **Não recomendo implementar**: o MOC diz que o FS só serve para consumir estoque, aquisição encerrada. Aceitar `tpEmis=2` só na **importação** de XML |
| 3 | SCAN / Regime Especial NFF | 55, 65 | ausente | SCAN **desativado** (NT 2015/002; regra `B22-30 (570)`); NFF só se `infSolicNFF` entrar (`ZE01/ZE02`) |
| 4 | EPEC | 55; 65 a critério da UF (`B22-34 (714)`) | ausente (só a ordem em `xsdorder/table.go:389-412`) | evento `110140`, emissão da NF-e com `tpEmis=4`, DANFE com texto EPEC, **transmissão posterior**, conciliação |
| 5 | FS-DA | 55 | ausente | similar ao 2, mais exigente em papel. Implementar só se houver cliente com FS-DA; DANFE em FS-DA não é gerável em papel comum (`2.9.2` do MOC) |
| 6 | SVC-AN | 55 | ausente | endpoints, designação por UF, estado, DANFE comum |
| 7 | SVC-RS | 55 | ausente | idem |
| 9 | Offline NFC-e; NF-e só com `tpImp=6` | 65; 55 restrito | ausente | QR offline (v2/v3), via do estabelecimento, fila de transmissão, prazo |

### 4.2 Fluxos e regras do MOC (resumo operacional)

**Designação SVC por UF (Ato COTEPE 39/2012, texto do MOC):**
- SVC-AN (`tpEmis=6`): AC, AL, AP, DF, MG, PB, RJ, RS, RO, RR, SC, SE, SP, TO.
- SVC-RS (`tpEmis=7`): AM, BA, CE, ES, GO, MA, MT, MS, PA, PE, PI, PR, RN.
- **Transcrever e confirmar no Portal antes de usar** (a UF pode ter mudado de SVC).

**SVC:** a SVC só opera quando a SEFAZ de origem a ativa; detectar por `NFeStatusServico` na SVC com `cStat 107` (em
operação), `113` (em desativação) ou `114` (desabilitada). Na desativação há 15 minutos de coexistência. Serviços na SVC:
autorização, retorno, **cancelamento só de NF-e autorizada pela própria SVC**, consulta e status. **Sem inutilização e sem
CC-e na SVC** (a necessidade deve ser represada para a SEFAZ de origem). NF-e autorizada na SVC **não precisa** ser
retransmitida. NFC-e **não** é autorizada na SVC (`B22-70 (783)`). Chave: `tpEmis` está na chave (posição 35), então a
mesma numeração pode existir nos dois ambientes; tratar duplicidade de chave natural cancelando a que não circulou.

**Pendentes de retorno (transmitida e sem resposta):** usar **nova numeração** na contingência e, ao voltar, **cancelar** a
autorizada que foi substituída ou **inutilizar** a não autorizada. A NF-e em `tpEmis=4/5/9` mantém a mesma chave (inclusive
`cNF`) ao ser transmitida depois.

**Rejeição de nota emitida em contingência:** regerar com mesmo número e série, sanando a irregularidade, desde que **não**
mude valor do imposto, remetente/destinatário ou data de emissão/saída; reimprimir o DANFE no mesmo papel.

**EPEC (`110140`):** `tpEmis=4`, mesmo `dhCont`/`xJust`; XML do evento com UF, CNPJ/CPF e IE do emitente, chave, UF e
documento do destinatário, `vNF`, `vICMS`, `vST`; assina e envia ao **Ambiente Nacional**; DANFE impresso em papel comum
com a frase "DANFE impresso em contingência - EPEC regularmente recebida pela Receita Federal do Brasil"; protocolo do
evento `891xxxxxxxxxxxx`. **EPEC pendente de conciliação há mais de 168 horas bloqueia o ambiente de contingência EPEC do
emitente** (`moc.txt` seção 3.3.6-3.3.9); a NF-e precisa ser transmitida no prazo da legislação.

**NFC-e offline (`tpEmis=9`):** decisão do contribuinte e **da UF** (`B22-20 (712)`, facultativa); prazo de transmissão:
**final do primeiro dia útil subsequente**; `dhCont` e `xJust` obrigatórios (não impressos); `mod=65`, `idDest=1`,
`finNFe=1`, `indFinal=1`, `indPres=1`; **avançar a numeração** na entrada em contingência (é vedado reutilizar número já
transmitido como normal); QR Code com **DigestValue da assinatura** (v2) ou `assinatura` (v3); DANFE com "EMITIDA EM
CONTINGÊNCIA / Pendente de autorização" (duas vezes), **segunda via "Via do Estabelecimento"** (dispensável por UF ou
trocável por guarda eletrônica com termo no livro modelo 6); protocolo suprimido no DANFE. Em EPEC (4) a NFC-e também
exige a informação de contingência no DANFE.

**NF-e `tpEmis=9` (novo, Ajuste SINIEF 13/26):** só com `tpImp=6` e QR Code (`B22-10 (711)`, `ZX02-224..338`), em operação fora
do estabelecimento, interna, saída, `finNFe=1`, `indFinal=1`, `indPres` 1/4/5, sem `NFref`, sem `retirada`/`entrega`, só CST/CFOP
de varejo (66 regras com `tpImp=6`, 3.2 item 18). Emissão normal com `tpImp=6` precisa ser **on-line, com tolerância de 5
minutos** (`B09-40`).

### 4.3 O que existe

- `EmissionMode` com `TpEmis`, `TpImp`, `ContingencyAt`, `Justification` e `Validate` (`emission_mode.go`); `dhCont`/`xJust`
  emitidos quando `IsContingency` (`builders_ide.go:93-97`); `tpEmis` entra na chave (`emit.go:898-920`).
- `xsdorder` conhece o `detEvento` do EPEC (`table.go:389-412`); o transporte `RecepcaoEvento` no Ambiente Nacional existe
  em `go-dfe/internal/endpoints/table.go` (registry `AN`).
- `documents/nfe.go:104-106,163` já distingue FS/EPEC no DANFE; `documents/nfce.go:83-85` imprime via do estabelecimento só
  para `tpEmis==9`.
- `emission_mode_test.go` cobre a validação do modo.

### 4.4 O que falta (ordem de dependência)

| # | Entrega | Detalhe | Tamanho |
|---|---|---|---|
| C1 | Endpoints e tabela UF para SVC | registries `SVC-AN` e `SVC-RS` em `go-dfe/internal/endpoints/table.go` (hoje só `SVRS`, `AN`); tabela UF para SVC como dado com teste de tabela (como `nfeUFAuth`); transcrever do Portal | 0,5 sem |
| C2 | Detector e estado de contingência | consulta `NFeStatusServico` do autorizador e da SVC; N falhas consecutivas ou timeout; estado **por organização, UF e modelo** em DynamoDB (sem Scan; item único por org); grava `dhCont`/`xJust` uma vez por janela; saída automática; alerta via go-common `alerts` | 2 sem |
| C3 | NF-e SVC-AN/RS | `EmissionMode` resolvido pelo estado; roteamento do worker para o endpoint SVC; cancelamento só na SVC de origem da nota; represamento de inutilização/CC-e; DANFE comum | 1,5 sem |
| C4 | EPEC | builder do `110140`, `tpEmis=4`, envio ao AN, `tpEmis=4` na chave, DANFE com texto EPEC, **fila de transmissão posterior** (reusar `worker_outbox`), conciliação e alarme em 24 e 120 horas (limite 168 horas) | 2 sem |
| C5 | NFC-e offline | QR v2 offline com `DigestValue` (calcular após assinar: mover a montagem do `qrCode` para depois da assinatura no `go-dfe-egress` ou pré-calcular o digest da referência canônica); QR v3 offline; renumeração; segunda via; **fila com prazo (fim do 1º dia útil seguinte)**; regra por UF; alerta de prazo | 3 sem |
| C6 | NF-e `tpEmis=9` com `tpImp=6` | depende do DANFE tipo 2 (Parte D) e das 66 regras `tpImp=6` | 2 sem |
| C7 | Pendentes de retorno | ao entrar em contingência, listar notas enviadas sem resposta e, ao sair, **consultar por chave** (`NfeConsultaProtocolo`) e então cancelar ou inutilizar; depende de a inutilização continuar existindo (1.1 item 11) | 1 sem |
| C8 | Painel | janelas abertas, pendentes, prazo restante (UI) | 1 sem |

Observação sobre o worker: hoje timeout de rede é retentado por Lambda async retry e a rejeição vai ao DLQ
(`worker/CLAUDE.md`). Não há reconciliação por consulta de protocolo em timeout de autorização: reenviar a mesma nota é
idempotente na SEFAZ (duplicidade devolve o protocolo), mas o worker não trata o código de duplicidade da autorização
(`authorizedStats` só tem `100,102,135,136,150,155`, `helpers.go:93-96`). Provável rejeição "terminal" de nota que na
verdade foi autorizada.

---

## 5. Parte D: DANFE

### 5.1 Formatos do MOC e estado

| Formato | `tpImp` | MOC | Estado | Gap |
|---|---|---|---|---|
| DANFE retrato (A4) | 1 | 2.7, 2.8.1 | feito: `templates/danfe_retrato.html`, `documents/nfe.go` | campos novos: `vICMSDeson`, `vFCPST`, IBS/CBS/IS, ISSQN (total e itens), `infAdProd` ok; paginação e canhoto: ok; "Folhas soltas x formulário contínuo" (itens 4 e 5 do MOC) não aplicável a PDF |
| DANFE paisagem (A4) | 2 | 2.8.2, cap. 6 e 7 | **feito (F8)**: `danfe_paisagem.html`, `format=paisagem` | template novo; mesmo contexto de dados |
| DANFE simplificado | 3 | 2.11 | **feito (F8)**: `danfe_simplificado.html`, `format=simplificado`; EPEC recusado (422) | papel >= 55 mm; campos mínimos listados (emitente, tipo, série/número, data, destinatário, itens, total, chave, código de barras, protocolo); proibido com EPEC e FS |
| DANFE simplificado etiqueta | 3 | 2.12 (NT 2020.004) | **feito (F8)**: `format=etiqueta` (tpImp não definido pelo MOC; só pelo parâmetro) | para e-commerce/telemarketing |
| DANFE simplificado tipo 2 | 6 | NT (Ajuste SINIEF 13/26) | **ausente** | cupom NFC-e-like para NF-e fora do estabelecimento, QR v3, 66 regras |
| DANFE NFC-e (papel) | 4 | 1, 2 | feito: `templates/danfce.html`, `documents/nfce.go` | ver 5.2 |
| DANFE NFC-e em mensagem eletrônica | 5 | 3.2 | **feito como PDF (F8)**: `danfce_mensagem.html`; envio por e-mail/SMS (texto/HTML) não implementado | gerar a "mensagem" (texto/HTML com chave, QR e link) para e-mail ou mensageria, sem papel |
| DANFE NFC-e resumido ("ecológico") | 4 | 1, 2.1.2 | **feito (F8)**: `format=resumido`; a UF e o pedido do consumidor são decisão do chamador | `show_items` fixo `true` (`documents/nfce.go:~83`); suprimir divisão II quando UF permitir e o consumidor pedir |
| Sem DANFE | 0 | B21 | ausente | opção explícita |

### 5.2 DANFE NFC-e, requisitos do MOC (checklist)

| Requisito | Estado |
|---|---|
| Divisões I a IX; CNPJ com máscara; "Documento Auxiliar da Nota Fiscal de Consumidor Eletrônica" | feito |
| Itens: código, descrição, qtde, un, valor unit., valor total | feito |
| Totais: qtde de itens, valor total, acréscimos/desconto, valor a pagar, forma de pagamento, valor pago, troco | feito |
| Chave em 11 blocos de 4 e URL de consulta do XML (`urlChave`) | feito |
| QR Code >= 25 mm com quiet zone de 3 mm | verificar CSS (não medi) |
| Identificação do consumidor (CPF/CNPJ/Id. estrangeiro/"NÃO IDENTIFICADO"); nome e endereço em entrega a domicílio | CPF e "não identificado" feitos; CNPJ/Id.estrangeiro só se o XML trouxer (não emitimos); **endereço de entrega ausente** |
| Número, série, data local; protocolo e data de autorização em hora local | feito; protocolo suprimido em contingência: ok |
| Mensagem fiscal (`infAdFisco`) e do contribuinte (`infCpl`) | feito |
| Lei 12.741 (`vTotTrib`) | `vTotTrib` fixo `"0.00"` (`builders_total.go:99`): informação opcional, mas o DANFE imprime zero |
| Homologação: "EMITIDA EM AMBIENTE DE HOMOLOGAÇÃO - SEM VALOR FISCAL" | feito (`documents/nfce.go:108`) |
| Contingência: duas linhas "EMITIDA EM CONTINGÊNCIA / Pendente de autorização" em dois lugares; segunda via | só `tpEmis==9`; **falta `tpEmis==4` (EPEC)** na NFC-e |
| Valores com vírgula decimal e ponto de milhar | feito (`moneyBR`) |
| Papel >= 56 mm, margens >= 2 mm | verificar template (não medi) |

### 5.3 QR Code e consulta por chave

| Item | MOC | Estado |
|---|---|---|
| QR v2 online: `chave|2|tpAmb|idCSC|hash` com SHA-1 de `parâmetros+CSC` | 3.2.1, 3.3.1 | **feito** (`nfce_qrcode.go:128-136`) |
| QR v2 offline: `chave|2|tpAmb|dia|vNF|DigestValue|idCSC|hash` | 3.2.2 | **ausente** (e exige digest pós-assinatura) |
| QR v3 (NT 2025.001): online sem hash/CSC; offline com `assinatura`, tipo e id do destinatário; obrigatório em NF-e 55 com QR | regras `ZX02-220..338` | **ausente**. Ordem exata dos parâmetros v3: **não consta nas páginas lidas** |
| URL base do QR por UF (prod/hom) e `urlChave` | 3.2, `ZX02-20 (395)`, `ZX03-20 (878)` | feito: tabelas `nfceQRBaseURL` e `nfceConsultURL` (`nfce_qrcode.go:~30-98`). **Os comentários do próprio arquivo dizem "MUST be validated against SEFAZ homologação"**: não há validação automática dos 27 pares. Hosts com `http://` e IP em homologação RR (`200.174.88.103:8080`) merecem revisão |
| Consulta pública por chave e QR Code da consulta | seção 7 da Visão Geral | só a URL (redirecionamento para a SEFAZ) |
| QR em NF-e 55 (`infNFeSupl` com `tpImp=6`) | `ZX01-10 (393)` | ausente |
| Código de barras Code-128C da chave e "dados da NF-e" (EPEC/FS) | 2.10.4, cap. 3 | feito (`barcode.go`, `documents/nfe.go:~115-125`) |
| Marca d'água (homologação, cancelada) | 2.10.1 | feito |

---

## 6. Parte E: divergências de estrutura

| Tema | MOC | Repo | Divergência |
|---|---|---|---|
| Ordem de tags | cada `xs:sequence` do XSD | `go-dfe/internal/xmlops/xsdorder/table.go` (fallback alfabético para o que não está na tabela, `builder.go` bloco `sort.Slice`) | tabela congelada em `PL_010e`: faltam `ide/cIndOp`, `det/{impostoDevol,infAdProd,vItem,DFeReferenciado}`, `retirada|entrega/{CEP,IE}`, `emit/{tpRegimeIBSCBS,ISUFEmit}`, `prod/{cAtivSN,vUnComLiq,vProdLiq}`, `total/{vProdLiqTot}`, `IBSCBS/gTribSN`, `infNFe/{gPgtoVinc,infSolicNFF,infPAA}`; **`ISSQNcalc` em vez de `ISSQN`** (1.1). Tag desconhecida **não falha**: vai ao fim. Falta teste de paridade tabela x leiaute |
| Namespace | `http://www.portalfiscal.inf.br/nfe` único; sem prefixo; sem outro namespace (`D01d 587`, `D02 404`) | `@xmlns` único no root (`builders_doc.go:663`) | ok; não há teste que rejeite elementos com outro namespace |
| Caracteres de edição | proibidos (`D01e 588`) | sem normalização | risco com `\n`/`\t` em `infCpl`, `xProd` |
| Assinatura | enveloped, C14N 1.0, `rsa-sha1` + `sha1`, `Reference` em `infNFe`, **sem** `X509SubjectName`, `KeyValue`, etc.; certificado com CNPJ do estabelecimento | `signer.go` (`rsa-sha1`, `sha1`, C14N à mão, `X509Certificate` só) | ok no padrão; a ressalva do `go-dfe/CLAUDE.md` (sem portão byte-idêntico) continua. Falta checar que o certificado contém o CNPJ base do emitente antes de assinar (E/F do MOC); `documents`/`certificates.go` já valida documento da organização |
| Envelope SOAP | SOAP 1.2, sem cabeçalho | `internal/soap/envelope.go` | ok |
| Lote | até 50 NF-e; NFC-e lote de 1 e síncrono; `idLote` 15 dígitos | `idLote` = `%015d` do número (`builders_doc.go:664`), `indSinc=1` | ok. **Lote nunca > 1**; sem `NFeRetAutorizacao` em uso (o serviço está em `implemented`, sem consumidor) |
| Eventos | `detEvento` por tipo, `nSeqEvento`, `Id` = `ID`+tpEvento+chave+seq | `service.go:482-570`, `extra_events.go`, `nfce_service.go:286` | **ausentes**: 110140 EPEC, 110130/110131 (comprovante de entrega e cancelamento), 110150 (ator interessado, transportador), 110300 (vinculação de pagamento), todos os eventos da reforma (112110 a 112150, 211110 a 211150, 212110/212120). Existem apenas como lista de permitidos no `110001` (`extra_events.go:~40`). `412140` (Simples Nacional) é gerado pelo Fisco, só distribuição. Presentes: 110110, 110111, 110112, 111500 a 111503, 2102xx, 110001 |
| Inutilização | serviço será eliminado (NT 2023.002 v1.02, produção 11/11/2026) | `inutilization.go` completo | **risco de retrabalho**; confirmar a NT |
| Distribuição | `NFeDistribuicaoDFe`, `distNSU/consNSU/consChNFe`, evento `110140` e comprovantes distribuídos | worker `distribution*.go` | parser de resumo (`resNFe`) e de eventos precisam reconhecer os novos `tpEvento` (reforma, 110130, 110150, 412140): não verifiquei cada `descEvento` |
| Consulta GTIN | `ccgConsGTIN` | ausente em `dfe.go` (`implemented`) | serviço não implementado; só útil para validar GTIN a pedido |
| Consulta cadastro | `NfeConsultaCadastro` | implementado no go-dfe; **nenhum uso pré-emissão** | oportunidade para `1C17-xx` |
| QR no envelope | `infNFeSupl` depois de `infNFe`, `Signature` por último | `table.go` `NFe: infNFe, infNFeSupl`; assinatura anexada ao pai | ok |

---

## 7. Parte F: validador genérico dirigido por tabela

### 7.1 Princípios

1. **Um só lugar valida o documento montado**, antes de `BuildXML`/assinatura e **antes do outbox**. Não é substituto do
   DTO (que valida o cadastro) nem do XSD (que continua fora por causa de `CGO_ENABLED=0`): é a ponte.
2. **Dados, não código, para tudo que é estrutural** (path, tipo, tamanho, decimais, ocorrência, enum, modelo). Regras
   condicionais viram **linhas numa tabela de regras** quando cabem em um DSL mínimo e **funções registradas por ID do MOC**
   quando não cabem.
3. **Pura e sem I/O**: pacote Go sem cgo, linkável por `api`, `worker` e `go-dfe-egress` (o `go-dfe/nfse/tables` já é o
   precedente de dados gerados). Tabelas de referência embutidas com `go:embed`, atualizadas por gerador, **sem
   consulta de rede e sem Scan** na emissão.
4. **Falha com RFC 7807** (`problem.Validation([]FieldError)` de `api-commons`, o mesmo helper de campo que o DTO já usa): `errors[]` com
   `rule` (ID do MOC), `cstat`, `path` (XML e JSON), `nItem`, `message` em português **sem travessão**.
5. **Obrig. bloqueia; Facult./UF avisa**, e modo de implantação gradual: `shadow` (só registra), `warn`, `block` por regra.
6. **Fonte da verdade gerada do MOC**: um gerador lê `Leiaute.html` e `RegrasDeValidação.html` (o mesmo parser que usei,
   `parse.py`/`parse_r.py` no scratchpad) e emite a spec; nunca editar à mão (padrão `nfse/tables`).

### 7.2 Spec de campo (a tabela)

```go
// FieldSpec descreve uma tag do leiaute. Gerada de Leiaute.html.
type FieldSpec struct {
    ID       string   // "B25d" (id do MOC)
    Path     string   // "infNFe/ide/cIndOp" (mesma chave do xsdorder: "pai:tag")
    Kind     Kind     // N, C, D, DT (data-hora), G (grupo), E (enum), A (atributo)
    Occurs   [2]int   // {min, max}; max -1 = ilimitado
    Len      LenSpec  // exato, faixa, ou conjunto {8,10}
    IntDigits, DecMin, DecMax int // 13v2 => 13,2,2; 3v2-4 => 3,2,4; 11v0-10 => 11,0,10
    Pattern  string   // ER do XSD (TString, TCnpj, TChNFe...), opcional
    Enum     []string // "0|1|2"
    Models   ModelSet // 55, 65 ou ambos (coluna "Modelo" das regras e do leiaute)
    ChoiceOf string   // choice do XSD ("CNPJ|CPF")
    Since, Until time.Time // vigência por NT (histórico)
    Req      *Cond    // obrigatoriedade condicional (ver 7.3)
}
```

O motor percorre o `map[string]any` que `BuildXML` consome (mesma forma): 1) tag desconhecida é erro (pegaria
`ISSQNcalc`); 2) ocorrência; 3) choice; 4) tipo; 5) tamanho; 6) decimais **sem arredondar** (rejeita; o builder é quem
formata); 7) ER e enum; 8) ordem conferida contra `xsdorder` (paridade); 9) caracteres (`TString`: sem edição, sem espaço
nas pontas, sem duplo espaço, faixa Latin-1).

### 7.3 Regras de negócio (a tabela de regras)

```go
type Rule struct {
    ID      string  // "W16-10" (id MOC)
    CStat   int     // 610
    Models  ModelSet
    Mode    Mode    // Obrig | Facult
    Since   time.Time; Until time.Time // vigência (NT)
    UFs     []string // vazio = todas; "a critério da UF" = lista configurável
    Check   func(*Doc, *Ctx) []Finding // ou Expr declarativa para os casos simples
    Status  Status  // Implemented | Future | OutOfScope(reason)
}
```

DSL declarativo mínimo (cobre ~40% das regras): `When(path op valor)` `=>` `Require|Forbid|Equals|InSet|SumEquals(paths, tol)`.
Exemplos: `B25.1-10`: `When(tpNFDebito present) => Equals(finNFe, 6)`; `W03-10`: `SumEquals(total/ICMSTot/vBC, det[*]/imposto/ICMS/*/vBC, tol=0)`;
`I11-10`: `Equals(vProd, round2(qCom*vUnCom), tol=0.01)`. As demais (IE por UF, GTIN, CFOP x CST, devolução) são funções registradas por
ID. **Teste de contrato**: toda linha de `regras.tsv` gerada tem `Status` explícito (nada fica "esquecido").

### 7.4 Dados de referência (tabelas embutidas)

NCM (com `00`, `00000000` e especiais RFB), CFOP (com indicadores `indNFe`, `indComb`, exportação indireta), CEST, municípios IBGE
(com DV e UF), países BACEN (com DV), cProdANP (combustíveis monofásicos), cBenef x CST por UF, indicadores de CST IBS/CBS
(`ind_gDif`, etc.), cClassTrib, prefixos GS1, uCom NFC-e, 27 algoritmos de IE, SUFRAMA, RECOPI, NVE, parâmetros por UF
(limites de valor, regras "a critério da UF", datas de vigência `N12-85/86`). Cada tabela tem versão e data da fonte e é
gerada por `cmd/gen-moc-tables`; **checagem de validade** no CI (alerta se a tabela tem mais de N dias).

### 7.5 Integração (sem reescrever o que existe)

1. `api/internal/services/nfes`: chamar `Validate(doc, ctx)` em `Emit` depois de `BuildEnviNFe` e antes de gravar o outbox
   (`emit.go:~700`) e em `nfce_emit.go:~210`. O DTO continua rejeitando formato (cadastro). Substituir `d()` por parse com erro.
2. Alinhar os limites de DTO de cadastro com a spec (gerar o `max=` a partir de `FieldSpec.Len` para `xNome`, `xLgr`, `xBairro`,
   `xMun`, `xProd`, `IE`): é o item 10 de 1.1 resolvido na origem, no mesmo gerador.
3. `worker`: reexecutar só a validação **estrutural** (barata) antes do `invokeEgress` como defesa em profundidade; mensagem de
   falha vai ao mesmo caminho de rejeição de negócio (não retentar).
4. `ui`: mapear `path` do erro 7807 para o campo do formulário; **guia** documenta a regra e o código (regra do CLAUDE.md
   da raiz). Sem travessão em mensagens.
5. **Telemetria de calibração**: guardar `(rule, cstat)` das rejeições reais da SEFAZ ao lado dos achados locais. Quem
   rejeitou na SEFAZ sem achado local vira prioridade de regra nova; achado local sem rejeição vira candidato a `warn`.
6. **Teste de paridade** (obrigatório): (a) todo nome de tag da spec existe em `xsdorder`; (b) toda chave literal dos
   builders existe na spec (teria pego `ISSQNcalc` e `cIndOp`); (c) todo `ID` de regra do MOC tem `Status`.

### 7.6 Fases e estimativa (1 desenvolvedor, semanas úteis; LOC é ordem de grandeza)

| Fase | Entrega | Tamanho | LOC (Go) | Semanas |
|---|---|---|---|---|
| F0 | **Correções do item 1.1** (tabela `xsdorder`, `vNF`, `d()`, `retirada/entrega`, limites de DTO) com teste golden e teste de paridade | pequeno, **antes de tudo** | ~400 | 1 |
| F1 | Gerador do MOC (`Leiaute.html` e `RegrasDeValidação.html` para spec e catálogo de regras) + motor estrutural (tipo, tamanho, decimais, ocorrência, choice, enum, ER, caracteres, tag desconhecida) | médio | ~1.200 de motor + ~1.200 linhas de spec geradas | 3 |
| F2 | Integração em `Emit` NF-e e NFC-e, erro 7807 com `errors[]`, `shadow`/`warn`/`block`, UI mapeando campos, telemetria | médio | ~700 | 2 |
| F3 | Regras P0 de cálculo e consistência (3.2 itens 1, 5, 6, 7, 8, 9): totais, `vNF`, pagamento, datas e fuso, `cNF`, finalidade, exterior | grande | ~2.500 | 4 |
| F4 | Tabelas de referência e regras P0 de cadastro (NCM, CFOP, CEST, GTIN, município, país, ANP, cBenef; 27 IEs por UF) | grande (dados) | ~2.500 + dados | 4 |
| F5 | CST x campos (ICMS, IPI, PIS, COFINS, monofasia, desoneração, FCP) | grande | ~2.000 | 3 |
| F6 | Reforma (UB, VB, VC, W31 a W60, NB, B25 débito/crédito) em paralelo à implementação das tags novas (Parte A) | grande | ~3.000 | 4 |
| F7 | `tpImp=6`, QR (v2/v3), NFC-e e demais P1 (combustível, importação, exportação, veículos) | grande | ~2.500 | 3 |
| F8 | Consulta de situação do emitente antes de emitir (`1C17-xx`) usando `NfeConsultaCadastro` | pequeno | ~300 | 1 |

Total aproximado: **25 semanas de uma pessoa** para chegar a cerca de 80% das regras Obrig. de 55/65; as 49 regras marcadas
"Implementação futura" e as ~63 "a critério da UF/opcional" ficam como `Future` ou parametrizadas. A parte de maior risco
é F4 (dados de terceiros que mudam) e F6 (vigência em curso). Paralelizável: F4 com F3; F6 com F5.

---

## 8. Plano de ação priorizado (sugestão)

| Ordem | Item | Motivo | Esforço |
|---|---|---|---|
| 1 | Ler NT 2023.002 v1.02 e decidir o destino da inutilização | produção em 11/11/2026 | 0,5 dia |
| 2 | F0 (correções 1.1) | defeitos prováveis em produção hoje | 1 sem |
| 3 | Tags e eventos da reforma com vigência em 16/11/2026 (NT 2026.005/006/007) | janela abre em 12 dias | 3 sem |
| 4 | F1 + F2 (validador estrutural em `shadow`) | rede de proteção para todo o resto | 5 sem |
| 5 | C1, C2, C3 (SVC) e DANFE paisagem/simplificado | contingência mais barata para NF-e e fecha 2 formatos | 4 sem |
| 6 | F3, F4 | maior redução de rejeições | 8 sem |
| 7 | C4 (EPEC), C5 (NFC-e offline), QR v3 | exigem decisão de produto e digest pós-assinatura | 5 sem |
| 8 | Eventos faltantes (110130/110131, 110150, 110300, reforma) | cobertura de fluxo | 3 sem |
| 9 | Itens residuais (NFF, PAA, `D avulsa`) | fora do escopo de um emissor de contribuinte | avaliar |

---

## 9. Referências no repo e no MOC

- Builders: `api/internal/services/nfes/builders_ide.go`, `builders_party.go`, `builders_prod.go`, `builders_tax.go`,
  `builders_total.go`, `builders_ibscbs.go`, `builders_doc.go`, `builders_extra.go`, `builders_transp.go`, `emit.go`,
  `nfce_emit.go`, `nfce_qrcode.go`, `emission_mode.go`, `inutilization.go`, `extra_events.go`.
- Ordem XSD: `go-dfe/internal/xmlops/xsdorder/table.go`; montagem: `go-dfe/internal/xmlops/builder.go`; assinatura:
  `go-dfe/internal/xmlops/signer.go`; endpoints: `go-dfe/internal/endpoints/table.go`.
- DANFE: `api/internal/services/documents/{nfe,nfce,format,barcode}.go` e `templates/`.
- Worker: `worker/internal/service/dfe.go`, `helpers.go`.
- Validação atual: `api/internal/validation/*.go`, `api/internal/api/v1/dto.go`.
- MOC (cópia estática): `http://moc.sped.fazenda.pr.gov.br/NFe_NFCe/` (`VisaoGeral`, `Leiaute`, `RegrasDeValidação`,
  `Contingencia_NFe`, `ContingenciaNFCe`, `DanfeCodigoBarras`, `DanfeQrCodeNFCe`, `HistoricoDeImpla`, `Recepcao*`, `NFe*`).

## Status F9 (eventos), 2026-11-07

Feitos: 110130, 110131, 110150, 110300 e os eventos da reforma 112110, 112140, 112150, 211110, 211124, 211128, 211130,
211140, 211150, 212110, 212120 (rotas em `DOCS.md`; autorizador e riscos em `CONDUCT.md`, "Eventos restantes (F9)").
Corrigido o corpo do EPEC (110140): `vNF`, `vICMS` e `vST` são filhos de `detEvento`, não de `dest` (MOC P32 a P34), e `IE` é opcional.
Não feitos: rota de emissão do EPEC (a opção `epec` segue fora de `emission.go`), 112120, 112130, 211120 (removido), 412120 e 412130 (Fisco).
