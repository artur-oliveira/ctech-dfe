# Auditoria MOC/XSD do CT-e (57, complementar, substituto, OS 67) x ctech-dfe

**Data:** 2026-11-09. **Tipo:** auditoria somente leitura (nenhum código, teste ou UI do repositório foi alterado; scripts e cópias
de trabalho ficaram no scratchpad da sessão).
**Fontes usadas (e só elas):** XSDs vendorizados em `go-dfe/schemas/cte/PL_CTe_400_NT2026.005_SN_1.00` (`SHA256SUMS` e
`README.md` de `go-dfe/schemas`); `docs/specs/2026-11-04-moc-gap-cte.md`; `docs/specs/2026-11-05-cte-emissao-rodoviaria.md`;
`docs/plans/2026-11-04-moc-cobertura-total.md` (o arquivo `docs/specs/2026-11-04-moc-cobertura-total.md` citado no pedido
**não existe**; só há o plano em `docs/plans`); `docs/specs/2026-11-08-contingencia.md`; PDF
`/home/artur/Downloads/MOC_CTe_Anexo II_DACTE_v4.00.pdf` (53 páginas, lido via `pdftotext -layout`, o mesmo conteúdo da leitura por páginas);
`/home/artur/Downloads/CTe_Nota_Tecnica_2012_007.pdf` (2 páginas, NT de 2012, não usada como fonte de regra atual).

## 1. Resumo

| Severidade | Quantidade |
|---|---|
| Blocker | 0 |
| Major | 8 |
| Minor | 9 |
| Informativo (verificado e conforme / registro) | ver seção 7 |

Nenhum blocker: o CT-e que o builder emite hoje (rodoviário, normal, complementar, substituto, OS, offline) **passa no XSD oficial
validado por libxml2** (seção 2). Os majors são (a) lacunas de cobertura que impedem a meta "todos os modais e tipos" e (b) três defeitos
reais: o portão não bloqueia violações de padrão, o DACTE soma pesos de medidas diferentes e o DACTE esconde o protocolo no offline.

## 2. Método e verificações que fecharam sem achado

1. **Specs gerados em dia:** copiei `go-dfe` para o scratchpad e rodei `go run gen/main.go`; `diff -rq specs` contra o versionado
   não apresentou diferença. `sha256sum -c SHA256SUMS` sem divergência. `go test ./validate ./internal/xmlops/...` verde.
2. **Teste diferencial independente (builder x libxml2):** em cópia do `api` + `go-dfe` gerei o XML real (`dfe.BuildXMLFragment`) de
   19 cenários e validei com `xmllint --schema cte_v4.00.xsd` (e `cteOS_v4.00.xsd`), com um stub só do `xmldsig` (o repositório não
   vendoriza o schema de assinatura, ver README). Cenários: 5 papéis de tomador (rem, dest, exped, receb, toma4), ICMS 00, 20, 40,
   60, 90, SN, IBS/CBS com `autXML` e `indDoacao`, offline `tpEmis=2` com `dhCont`, `xJust` e `sign` no QR, complementar, substituto
   com e sem troca de tomador, OS pessoas, bagagem e valores. **19 de 19 válidos.** Os tipos CT-e 57 e OS 67 não têm divergência de
   ordem, ocorrência, enum ou padrão nesses cenários.
3. **Fidelidade do validador próprio:** 24 mutações do mesmo documento (CFOP `5000`, `tpServ=9`, `tpEmis=5`, `modal=07`, `natOp`
   ausente, tag desconhecida, série 1000, `nCT=0`, `toma3`+`toma4`, `infQ` ausente, `qCarga` com 3 casas, `cUnid=06`, `infCteComp` junto de
   `infCTeNorm`, 11 `autXML`, chave NF-e curta, `vTPrest` com 3 casas, `imp` ausente, `email` inválido, `ISUF` curto, `retira=2`, `CRT`
   ausente, `infDoc` ausente, RNTRC de 7 dígitos). O `validate` (Shadow) e o libxml2 concordam em todas (a única "válida" nos dois lados
   é a que depende do `infModal`, que é `xs:any` no XSD principal; `infDoc` ausente é válido porque o XSD o marca opcional).
4. **Eventos:** `xCondUso` do 110110 confere byte a byte com a enumeração do `evCCeCTe`; limites de `xJust` (15 a 255), `xObs` (15 a 255),
   `infCorrecao` (1 a 20, 20, 500, item 1 a 99) e o padrão do `Id` do evento (`ID`+tpEvento+chave+nSeq de 3 dígitos) conferem com o XSD.

## 3. Achados

Formato: **ID, severidade, onde (arquivo:linha), evidência, correção proposta.** Cada um foi conferido duas vezes (leitura do código e
execução ou consulta ao XSD/PDF).

### Major

**M1. O portão (`Gate`) não recusa violação de padrão, enum, obrigatoriedade nem ocorrência; só tamanho.**
`api/internal/services/dfecheck/dfecheck.go:61-63` (`sizeCodes`) e `:81`. Evidência: teste no scratchpad
(`ws/api/internal/services/ctes/gate_probe_test.go`) com `rem.Phone="12345"`, `dest.Email="a@b"`, CEP `123` e CFOP `5000`: o Shadow
registrou 4 achados `pattern` (`rem/fone`, `dest/email`, `emit/enderEmit/CEP`, `ide/CFOP`) e `gateCTe(body)` devolveu `nil`. O item 3
acima mostra que esses achados coincidem com a recusa do libxml2 (SEFAZ 225). Dado de cadastro de pessoa chega ao CT-e sem recusa local.
Correção: promover a bloqueio os códigos `pattern`, `enum`, `required`, `max_occurs`, `choice_*` e `unknown_tag` do schema para
documentos montados pelo builder (determinísticos, 24/24 concordantes com o libxml2); manter o aviso para o que vier de terceiros.

**M2. `xsdorder`: modais do CT-e usam a tabela do MDF-e ou não têm tabela.**
`go-dfe/internal/xmlops/xsdorder/table.go:722-751` (entradas sem escopo `aereo`, `aquav`, `ferrov` são do MDF-e). Evidência: teste no
scratchpad `Resolve("infCTeNorm:infModal","aquav")` devolve `[irin tpEmb cEmbar xEmbar nViag ...]` (MDF-e), mas o XSD do CT-e
(`cteModalAquaviario_v4.00.xsd`) tem `vPrest, vAFRMM, xNavio, balsa, nViag, direc, irin, detCont, tpNav`; `aereo` devolve `[nac matr
nVoo ...]` contra `nMinu, nOCA, dPrevAereo, natCarga, tarifa, peri`; `ferrov` devolve `[trem vag]`; `duto` e `multimodal` não têm
entrada (resolução alfabética). `go-dfe/validate/xsdorder_parity_test.go:17` só confere `nfe, mdfe, cte, cteos, cte_rodoos` e `:49` lista
como `notBuilt` os eventos `evEPECCTe`, `evGTV`, `evRegMultimodal`, `evApropriaCredPres`, `evTributacaoSN`, `evCancTribSN`; `ctesimp` e
os modais `cte_aereo`, `cte_aquav`, `cte_duto`, `cte_multimodal` ficam de fora (rodei a mesma comparação no scratchpad: dezenas de
"falta no xsdorder"/"fora de ordem"). Hoje é latente (só o rodoviário é emitido), mas qualquer modal novo sairia fora de ordem e daria 215.
Correção: tabelas com escopo `infCTeNorm:infModal:<modal>` e dos subgrupos (`detCont`, `tarifa`, `natCarga`, `peri`, `fluxo`, ...), e
incluir os modais, `ctesimp` e GTV-e/EPEC no teste de paridade (estrito).

**M3. Não existe spec de validação do modal ferroviário do CT-e.**
`go-dfe/validate/gen/main.go:426-431` (geram-se `rodo, rodoOS, aereo, aquav, duto, multimodal`, nunca `ferrov` do CT-e) e
`go-dfe/validate/validate_test.go:9` (só `mdfe_ferrov`). O XSD `cteModalFerroviario_v4.00.xsd` (231 linhas) está vendorizado.
Correção: acrescentar o alvo `{"…cte/PL_CTe_400…", "ferrov", "specs/cte_ferrov.json"}` e a tabela de ordem (M2).

**M4. DACTE soma medidas de tipos diferentes numa única caixa "Peso bruto (kg)".**
`api/internal/services/documents/cte.go:72-98` (`cteLoad`) e `templates/dacte_retrato.html:98`. Evidência: a função soma todo `infQ` com
`cUnid` 01 ou 02 em `kg`; o Anexo II (seção 3, "Peso Bruto (KG)", "Peso Base Cálculo (Kg)", "Peso Aferido (Kg)") exige uma caixa por
`tpMed`, e o XSD documenta `tpMed` como "PESO BRUTO, PESO DECLARADO, PESO CUBADO, PESO AFORADO, PESO AFERIDO, PESO BASE DE CÁLCULO,
LITRAGEM, CAIXAS". Um CT-e recebido com PESO BRUTO 1000 e PESO BASE DE CÁLCULO 1200 imprime 2200 kg. O builder próprio emite só
`PESO BRUTO`, então o defeito aparece nos CT-e de terceiros (a distribuição também renderiza). Correção: separar por `tpMed` (normalizado)
nas três caixas do Anexo e não somar tipos diferentes.

**M5. DACTE esconde o protocolo de um CT-e offline já autorizado.**
`api/internal/services/documents/cte.go:161-165` e `:231-235` (`if !contingency && prot != nil`) e `templates/dacte_retrato.html:76`
(`{% if is_contingencia %}legenda{% elif protocolo %}`). O Anexo II 2.1 diz "O número do Protocolo de Autorização de Uso recebido da
SEFAZ deverá ser impresso no DACTE" e o campo 2 da seção 2.19.1 leva `nProt` e `dhRecbto`. O comentário do código admite que a legenda de
contingência é do FS-DA (revogado, o Anexo cita `tpEmis=5`, valor que nem existe no XSD) aplicada por analogia. Com `tpEmis=2` e `infProt`
cStat 100 presente, o DACTE não mostra o protocolo. Correção: imprimir o protocolo sempre que o `protCTe` autorizado existir e deixar a
legenda de contingência apenas quando não houver protocolo.

**M6. Cobertura de emissão: o builder cobre só uma fatia do leiaute.**
`api/internal/services/ctes/builder.go:23` e `:247` (`modal` fixo `01`), `:313-314` (`infDoc` só `infNFe`; `infModal` só `rodo/RNTRC`),
`:244+` (`tpServ` só 0; complementar e substituto herdam do original). Contagem por script: os 19 documentos emitidos (com os campos
opcionais que o código sabe preencher) tocam 192 dos 503 elementos do `CTe`; o restante opcional é: `compl` (Entrega, fluxo, ObsCont,
ObsFisco, xCaracAd/Ser, xEmi, origCalc/destCalc), `ide` (indGlobalizado, xDetRetira, gCompraGov, tpPagAnt, gPagAntecipado, gMultaJuros),
`emit` (IEST, ISUFEmit, tpRegimeIBSCBS), `dest/ISUF`, `imp` (ICMSOutraUF, ICMSUFFim, gICMSPrevistoPagtoAntecip, vTotTrib, infAdFisco,
e do IBSCBS os grupos gIBSCBS/gDif/gDevTrib/gRed/gALCZFMCBS/gTribRegular/gTribCompraGov/gTribSN/gEstornoCred), `vPrest/vTPrestLiq`,
`pgtoVinc`, `infCTeNorm` (`infDoc/infNF`, `infOutros`, `infDCe`, unidades de carga e transporte, PIN, dPrev, `docAnt`, `veicNovos`,
`cobr`, `infGlobalizado`, `infServVinc`), `infSolicNFF`, `infPAA`, e os modais 02, 03, 04, 05 e 06. `tpServ` 1 a 4 (subcontratação,
redespacho, redespacho intermediário, vinculado a multimodal) não são emitíveis, embora as regras G036, G037, G055 a G060 existam.
Isto é escopo documentado (`docs/specs/2026-11-05-cte-emissao-rodoviaria.md`), não regressão, mas contradiz a meta "100% das tags".
CT-e Simplificado (`cteSimp`) e GTV-e (64) são só spec, sem código (`2026-11-04-moc-gap-cte.md`, fim do arquivo).

**M7. Catálogo de regras: lacunas P0 e nenhuma regra para OS, Simplificado e GTV-e.**
`go-dfe/validate/rules_cte.go` (141 regras, `doc` `cte`); `validate.Rules("cteos"|"ctesimp"|"gtve")` devolve 0 (script no scratchpad).
`dfecheck.go:179` assume isso para o OS ("H001 a H133 não estão em go-dfe/validate"); as regras H vivem em `api/internal/services/ctes/os.go`
(fora do catálogo, sem Shadow nem `Mode`). Pela tabela P0 de `2026-11-04-moc-gap-cte.md:220-243`, faltam no catálogo (conferido por
diferença de conjuntos de cStat): 252 (G001, `tpAmb`); 515/516 (G011 e H007/H008, SVC só em tpEmis 7/8); 579, 580, 582, 583, 584
(G025 a G031; só o 581 existe); 540 (G044); 930 (ano/mês das chaves de documento); 676 (G049, CFOP na lista 3.1 do MOC); 850 a 855
(QR Code G212 a G217 e I70 a I75; `offline.go` trata o `sign` fora do catálogo); 310 a 314 e 388 (CST/cClassTrib existentes e
compatíveis); existência do município no IBGE (713, 532 a 537, 541, 542; hoje só o prefixo de UF). Datas 212 e 228 ficam de fora por
dependerem de relógio (decisão registrada no cabeçalho do arquivo). Correção: implementar os P0 faltantes; mover as regras H do OS para
o catálogo (`doc` `cteos`) para o mesmo modo shadow/warn/block.

**M8. CT-e OS só emite `tpCTe=0` e modal rodoviário.**
`api/internal/services/ctes/os.go:522` (`"tpCTe": tpCTeNormal`) e `:3` (modal `01`). O XSD `cteOS_v4.00.xsd` admite `tpCTe` 0, 1 e 3,
`modal` 01 a 04, `infCteComp` (1 a 10), `infCteSub` e `refCTeCanc`. Não há complementar nem substituto de OS, nem OS aéreo, aquaviário
ou ferroviário. Correção: escopo (pedir ao dono) e, se entrar, regras H correspondentes.

### Minor

**m1. G008 e G009 usam `tpEmis` "5", que não existe no XSD.** `go-dfe/validate/rules_cte.go:77,86` (`cteIn(tpEmis,"2","5")`). O enum do
`ide/tpEmis` é 1, 2, 3, 4, 7, 8 (`cteTiposBasico_v4.00.xsd`, doc: "1 Normal; 2 Contingência Offline; 3 Regime Especial NFF; 4 EPEC em
desativação; 7 SVC-RS; 8 SVC-SP"). O gap spec (`:228`) diz "`dhCont`/`xJust` só em `tpEmis=2`". Ramo morto; remover o "5".

**m2. DACTE do 57 omite campos que o Anexo II lista.** `api/internal/services/documents/cte.go:47-58,119-135,150-200` e o template. Itens do
Anexo II seção 3 e 2.21.4 sem impressão: `ISUF` do destinatário (Inscrição SUFRAMA), `indGlobalizado` e `infGlobalizado/xObs`, `ObsCont`
("Uso exclusivo do emissor", só o DACTE OS imprime) e `ObsFisco` ("Reservado ao Fisco"), `infDCe` (a função `cteOriginDocs` trata só
`infNFe`, `infNF`, `infOutros`), documentos de transporte anterior (`docAnt`, citados em 2.1), `veicNovos` e produto perigoso `peri`
(2.21.4), CNPJ/CPF do emitente do documento originário para `infNF` (Anexo: "CNPJ/CPF, em rem"; o código deixa vazio), e "CONTINUA NO
VERSO"/folhas adicionais com cabeçalho repetido (2.11 e 2.12; não verificável sem renderizar).

**m3. DACTE: geometria e posição fora do Anexo.** (a) Texto de homologação é marca d'água a 30% da página
(`templates/dacte_retrato.html:40,43`), o Anexo 2.20 manda imprimi-lo "nesta área", centralizado, na faixa de conteúdo variável abaixo da
chave. (b) QR com 22 mm sem margem própria (`:34`); o mínimo é 25 mm com 3 mm de quiet zone (Anexo 2.18). (c) Código de barras
`width:100%` de uma coluna de 97 mm sobre imagem 600x80 (`documents/barcode.go:26`) resulta em cerca de 12,9 mm de altura de barra, abaixo
do mínimo de 1,5 cm (Anexo 2.17); o `min-height:15mm` do `.barcode-band` não amplia a imagem. Conferir por render antes de corrigir.

**m4. Rótulo "Anulação" para `tpCTe=2` no DACTE.** `cte.go:32`. O enum `tpCTe` do XSD v4.00 é 0, 1, 3 (`TFinCTe`); não existe `infCteAnu` em
nenhum XSD do pacote (grep zero) e o CT-e de anulação deixou de existir no 4.00. Só cosmético; mantém `2` para XML antigo recebido.

**m5. UI: carta de correção oferece campos que não existem no grupo.** `ui/src/lib/data/cte_correction.ts:16-31` (`PARTY_FIELDS` usado em
`rem`, `dest`, `exped`, `receb`): `xFant` só existe em `rem` e `emit`; `dest`, `exped` e `receb` não o têm no XSD; `xLgr`, `nro`, `xCpl`,
`xBairro`, `CEP` pertencem a `enderReme`, `enderDest`, `enderExped`, `enderReceb`, não ao grupo `rem` etc. Resultado: `grupoAlterado=rem`
com `campoAlterado=xLgr` não corresponde à estrutura do leiaute. Correção: grupos `enderXxx` para os campos de endereço e retirar
`xFant` de dest, exped e receb.

**m6. CFOP: validação da UI e da API é mais frouxa que o XSD.** `ui/src/lib/schemas/cte.ts:70,92` (`/^[5-7]\d{3}$/`) e
`api/internal/validation/validators.go:27` (`^\d{4}$`) contra o XSD `[123567][0-9]([0-9][1-9]|[1-9][0-9])` (os dois últimos dígitos não
podem ser `00`). `5000` passa na API e, por M1, no portão (provado na sonda). Na UI o valor vem de lista (`cfop.ts`, 12 CFOP com `cte:
true`: 5351 a 5357, 5359, 5360, 5931, 5932, 7358 e suas variantes), então o risco é só por chamada direta. Corrigir o validador `cfop` do
CT-e para o padrão do XSD.

**m7. `p_red_bc` com CST 00 ou 60 reduz a base sem declarar a redução.** `api/internal/services/ctes/emit.go:539-590` (`resolveICMS`): `vBC`
é calculado com `pRed` antes do `switch`, mas só `ICMS20` e `ICMS90` emitem `pRedBC`; para `00` e `60` o XML sai com `vBC` menor que
`vTPrest` e sem `pRedBC`. A UI só mostra a redução para 20 e 90 (`CTE_CST_WITH_REDUCTION`), então é caminho de API. Correção: recusar
`p_red_bc` fora de 20 e 90 (422).

**m8. Documentação defasada.** `docs/specs/2026-11-08-contingencia.md:305` diz que não há renderizador de DACTE (existe
`api/internal/services/documents/cte.go`); `2026-11-04-moc-gap-cte.md` seção 0 item 1 ainda afirma "CT-e não emite"; o caminho
`docs/specs/2026-11-04-moc-cobertura-total.md` não existe (só o plano). A regra do repositório diz que doc desatualizada conta como bug.

**m9. Lista de UFs da G024 não verificável.** `rules_cte.go:139-141` (`AM BA CE GO MG MS MT PE RN SE SP PI PA RJ`). O gap spec só diz "lista de
UFs que vedam isento" sem a lista; nenhuma fonte disponível a comprova. Em `warn`, sem dano, mas a lista precisa de fonte (Anexo I).

## 4. Condicionais por tipo, tomador e modal (resultado)

- `tpCTe=1` (complementar): `infCteComp` 1 a 10 no lugar de `infCTeNorm`, regras G017, G161, G177 em `block`. Conforme o XSD (choice
  `ch11`) e validado por libxml2. Rodoviário apenas.
- `tpCTe=3` (substituto): `infCTeNorm/infCteSub` com `chCte` e `indAlteraToma` (única opção `1`), regras G076, G077, G078 em `block`.
  Conforme. A premissa de G076 ("tpEmis diferente de normal") não é verificável aqui (ver seção 6).
- Anulação: **não existe no leiaute vendorizado** (sem `infCteAnu`, `tpCTe` 0/1/3), nada a implementar; coerente com o código.
- `toma3` (0 a 3) e `toma4` (4, CNPJ|CPF, `enderToma`): conforme (choice `ch1` do XSD, 5 papéis validados). `indIEToma` derivado da IE do
  tomador (1/2/9) é coerente com G022/G023.
- Seguro, perigosos, veículos novos, `infDoc` além de NF-e, `docAnt`, `cobr`: não emitidos (M6); schema validado no catálogo
  (`cte.json`).
- ICMS: variantes `ICMS00/20/45/60/90/SN` emitidas e válidas; `ICMSOutraUF`, `ICMSUFFim`, `gICMSPrevistoPagtoAntecip` não emitidas.
- IBS/CBS: grupo regular `gIBSCBS` + `vTotDFe` válidos; CRT diferente de 3 recusado com 422 `ibscbs_regime_unsupported`
  (`ibscbs.go:65`), coerente com SN-1100/1107; `tpRegimeIBSCBS` e `gTribSN` não suportados (documentado).
- OS (67): `tpServ` 6, 7, 8, `rodoOS` com TAF|NroRegEstadual, `infGTVe`, `infDocRef`, `seg`, `infTribFed`: válidos pelo XSD.
- Eventos emitidos: 110111, 110110, 610110, 610111, 110180, 110181, 110190, 110191, 110300, 110301. Não emitidos (tabelas `xsdorder`
  também ausentes): EPEC (`evEPECCTe`, "em desativação" no XSD), GTV (`evGTV`), `evRegMultimodal`, `evApropriaCredPres`,
  `evTributacaoSN`, `evCancTribSN`.

## 5. UI x API (enums e regras)

Sem divergência em CST do ICMS (`00 20 40 41 51 60 90 SN` nos dois), papéis de tomador, limites (`Comp/xNome` 15, `xObs` 2000, `proPred`
60, `autXML` 10, notas 50) e IBS/CBS (CST 3 dígitos, classe 6, taxas de 2 a 4 casas). Divergências: m5 e m6 acima. A UI não oferece
`tpServ`, modal nem tipos de documento transportado, igual ao backend.

## 6. O que NÃO consegui verificar e o documento necessário

- **Texto das regras de validação (MOC Anexo I, Leiaute e Regras, G001 a G217, H001 a H133, I01 a I75):** não está no repositório nem em
  `~/Downloads` (só o Anexo II do DACTE). Por isso não provei: a semântica exata de G076 (substituto só em emissão normal), G012, G059,
  G053/G054 para `tpCTe=1`, a lista de UFs de G024, o CFOP permitido por UF (G049), e as regras H do OS (`os.go`). Necessário: MOC CT-e
  Anexo I (versão da NT 2026.005).
- **Regras da NT 2026.006 (split payment, 110300/110301):** autor, autorizador e prazo seguem sem fonte (`2026-11-08` do gap spec);
  necessário: IT DFe 2026.001 ou NT 2026.006 em PDF.
- **Leiaute visual do DACTE (figuras das seções 5 e 6 do Anexo II):** `pdftotext` não extrai as imagens; comparação de caixas e
  posições, "folhas adicionais" (2.12) e geometria final (m3) exigem render e inspeção visual com o PDF.
- **Comportamento da SEFAZ:** SOAP sem `cteCabecMsg`, URLs de QR Code por UF e do SVC, e `cOrgao` de evento em SVC (o código usa a UF da
  chave) dependem de homologação real.
- **Schema `xmldsig`:** não vendorizado (README); a assinatura foi tratada com stub no diferencial (item 2 da seção 2).
- **`NT 2012.007`:** só duas páginas e de 2012; não serve para o leiaute 4.00.

## 7. Registro (conforme, sem ação)

- 19 de 19 cenários do builder válidos no XSD oficial por libxml2; validador próprio concorda com libxml2 em 24 de 24 mutações.
- `specs/*.json` e `SHA256SUMS` em dia com os XSDs vendorizados; paridade `xsdorder` para `cte`, `cteos`, `cte_rodoos` verde.
- Eventos: literais, limites e `Id` conforme XSD (seção 2, item 4).
- DACTE: chave em 11 blocos, CODE-128C da chave de 44, QR da `qrCodCTe`, ton para kg com fator 1000, cubagem só para `cUnid=00`, volumes
  `cUnid=03`, homologação em caixa alta com o texto do Anexo 2.20, número da folha no topo (`@top-right`), tomador `toma3`/`toma4`
  conforme a correlação da seção 3 do Anexo.

## 8. Ordem sugerida de correção

1. M1 (portão), m6 e m7: pequenas e de alto retorno, sem mudar leiaute.
2. M4 e M5 (DACTE de CT-e recebido e offline).
3. M2 e M3 antes de qualquer modal novo; depois M6 por modal (cada um com tabela de ordem, spec, regras, builder, UI e guia).
4. M7 (regras P0 e OS no catálogo) junto de cada tipo novo.
5. Decisão do dono sobre M8 e sobre CT-e Simplificado e GTV-e.

## 9. Status das correções (2026-11-09)

| ID | Estado | Nota |
|---|---|---|
| M1 | fora do escopo | Gate/dfecheck: agente posterior |
| M2 | corrigido | tabelas `infCTeNorm:infModal:<modal>` para aereo, aquav, ferrov, duto, multimodal; `TestXSDOrderMatchesCTeModalSpecs` estrito (inclui `cte_rodo`). `ctesimp` e eventos não construídos seguem fora da paridade |
| M3 | corrigido | alvo `ferrov` em `gen/main.go`, `specs/cte_ferrov.json`, `TestLoadAllSpecs` |
| M4 | corrigido | `cteLoad` separa Peso Bruto, Base de Cálculo e Aferido; teste `TestDACTEWeightsSplitByTpMed`. `tpMed` sem caixa no Anexo não é impresso |
| M5 | corrigido | protocolo impresso com `tpEmis=2`; `TestDACTEOfflineAuthorizedShowsProtocol` |
| M6 | backlog | cobertura do builder (modais 02 a 06, tpServ 1 a 4, infDoc/infModal ampliados, Simplificado, GTV-e) |
| M7 | pendente | sem texto do MOC Anexo I no repositório, nenhuma regra P0 nova foi acrescentada (nada inventado) |
| M8 | backlog | OS tpCTe 1/3 e modais 02 a 04: decisão do dono |
| m1 | corrigido | G008 e G009 só com `tpEmis=2` |
| m2 | pendente | campos omitidos do DACTE; sem render visual não se prova a posição |
| m3 | parcial | QR 22 mm + 3 mm de margem e barras de 18 mm (Anexo 2.17 e 2.18); marca d'água de homologação (2.20) não alterada, exige render |
| m4 | mantido | rótulo "Anulação" só rotula XML legado (comentário no código) |
| m5 | corrigido | grupos `enderXxx`; `xFant` só em rem |
| m6 | corrigido | validador `cfop_cte` (API) e regex da UI |
| m7 | corrigido | 422 para `p_red_bc` fora de CST 20 e 90 |
| m8, m9 | não tratados | documentação antiga e lista de UFs da G024 (sem fonte) |

Não implementado por instrução: `dhEmi` com fuso da UF (helper compartilhado em go-dfe, outro agente).
