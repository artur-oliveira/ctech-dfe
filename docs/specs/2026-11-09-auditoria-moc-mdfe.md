# Auditoria MOC x implementação: MDF-e (modelo 58), estado em 2026-11-09

**Tipo:** auditoria somente leitura. Nenhum código, teste ou UI foi alterado. Scripts descartáveis ficaram no scratchpad da sessão (`.../scratchpad/mdfe-audit/`), com testes Go injetados por `go test -overlay` (nada foi escrito no repositório além deste arquivo).

## 0. Fontes e método

Fontes usadas (somente estas):

- XSDs vendorizados `go-dfe/schemas/mdfe/PL_MDFe_300b_NT012025_1.05/*.xsd` (16 arquivos; `go test ./validate` confirma o `SHA256SUMS`).
- `docs/specs/2026-11-04-moc-gap-mdfe.md` (leitura do MOC online 3.00b feita em 2026-11-04; é a única transcrição do texto das regras F/K), `docs/plans/2026-11-04-moc-cobertura-total.md`, `docs/specs/2026-11-04-ciot-antt-pef-integracao.md`, `docs/specs/2026-11-08-contingencia.md`.
- `~/Downloads`: não há MOC, Anexo II do DAMDFE nem NT do MDF-e (só `MOC_CTe_Anexo II_DACTE_v4.00.pdf` e manuais de NFC-e, irrelevantes aqui). `docs/specs/2026-11-04-moc-cobertura-total.md` não existe; o documento é `docs/plans/2026-11-04-moc-cobertura-total.md`.

Método:

1. **XSD x builder por execução, não só por leitura.** Montei 14 documentos reais com `BuildMDFe` (rodoviário completo com prop CPF, 2 reboques, 1 CIOT, valePed, contratante, infPag, seg, lacres, autXML, infAdic, infRespTec, infLotação, unidade de transporte, peri; rodoviário com CT-e, entrega parcial e reentrega; aéreo; aquaviário com terminais, balsas, unidades vazias e infMDFeTransp; ferroviário; carregamento posterior; infPag a prazo com IPEF; tpEmit 3 com prop CNPJ; variantes de infLotação), serializei com `xmlops.BuildXML` (que aplica `xsdorder`) e validei com `xmllint --schema` contra o XSD do documento, de cada modal e de cada evento (7 eventos montados por `buildEventEnvelope`). `infModal` e `detEvento` são `xs:any` no XSD, então validei o modal e o grupo `evXxx` isoladamente contra o XSD próprio.
2. **Ordem:** o teste de paridade do repo cobre o documento e os eventos; os modais não eram cobertos. Rodei a mesma comparação para `mdfe_rodo`, `mdfe_aereo`, `mdfe_aquav`, `mdfe_ferrov`: sem divergência.
3. **Catálogo de regras:** comparei os IDs F01 a F123 com `go-dfe/validate/rules_mdfe.go` e li cada regra contra o texto transcrito no gap doc.
4. **Validador de forma:** exercitei `validate.Load("mdfe_*")` com entradas fora do XSD (tamanho, `length`, padrão, enum, ocorrência) para saber o que é detectado e, depois, o que o `Gate` realmente recusa.
5. DAMDFE, UI e eventos lidos no código; achados confirmados duas vezes (leitura do código e XSD, ou execução).

### Resultado geral

- Estrutura, ordem e ocorrência: **todos os documentos e modais gerados são válidos no XSD** (exceto os casos listados abaixo). O conjunto de tags emitível cobre o XSD, com as lacunas de §3.
- Os pontos fracos estão no que acontece **antes** do XML (DTOs, UI, enums de cadastro) e no **portão** (`Gate`), que só recusa tamanho de texto.

## 1. Contagem

| Severidade | Qtde |
|---|---|
| Blocker | 1 |
| Major | 12 |
| Minor | 20 |

## 2. Achados

Formato: ID, severidade, onde, evidência, correção proposta. Caminhos relativos a `/home/artur/Documents/Projects/Ctech/ctech-dfe`.

### Blocker

**B-01. Evento 110116 gera `infViagens` fora do padrão do XSD (toda chamada sem zeros à esquerda).**
- Onde: `api/internal/services/mdfes/payment_events.go:84-87` (DTO `max=5,numeric`), `payment_events.go:181-187` (`payTransportDet` repassa o valor cru).
- Evidência: XSD `evPagtoOperMDFe_v3.00.xsd`: `qtdViagens` e `nroViagem` com `pattern [0-9]{5}`. Gerei o evento com `Total:"1", Number:"1"`; `xmllint` recusou: `The value '1' is not accepted by the pattern '[0-9]{5}'` (os dois campos). O DTO aceita "1". O evento só passa por `ShadowEvent` (`events.go:100`, log), então a SEFAZ é quem recusa (215).
- Correção: o DTO deve ser `len=5,numeric` ou o serviço faz `%05d` ao montar; e promover o evento ao `Gate` (ver M-01).

### Major

**M-01. O portão local só recusa tamanho; todo o resto do XSD é só log, e eventos nunca passam por portão.**
- Onde: `api/internal/services/dfecheck/dfecheck.go:63` (`sizeCodes` só `min_length` e `max_length`), `dfecheck.go:68-93`; `api/internal/services/mdfes/events.go:100` (`ShadowEvent`).
- Evidência: o validador gerado detecta (rodei): `pattern`, `enum`, `length`, `required`, `choice_*`, `max_occurs`, dígitos. Exemplos detectados e **não recusados**: `ferrov/vag/tpVag` e `serie` (`length`, facet `length=3`, código `length` ausente em `sizeCodes`), `rodo/veicReboque/capKG` ausente (`required`), `indAntecipaAdiant="0"` (`enum`), `dhTrem` sem fuso (`pattern`), CEP junto de latitude (`choice_conflict`), `ide/serie`, `categCombVeic`. Pelo gap doc (linha 62 e F.1) o objetivo é "validação interna criteriosa antes da SEFAZ".
- Correção: promover ao `Gate` os códigos determinísticos do XSD (`pattern`, `enum`, `length`, `required`, `choice_*`, `max_occurs`, `total_digits`, `fraction_digits`), depois de rodar o shadow em homologação sem falso positivo; criar `GateMDFeEvent` para os 7 eventos.

**M-02. `emit/IE` é a primeira IE do cadastro, sem filtrar por UF, e "ISENTO" vai cru.**
- Onde: `api/internal/services/mdfes/builder.go:481` (`firstIE`), `api/internal/services/person.go:51-60` (`FirstIE` ignora `uf`); contraste: `api/internal/services/nfes/builders_party.go:14` (`getIEForUF`).
- Evidência: o cadastro aceita a literal `ISENTO` (`api/internal/api/v1/dto.go:47`) e várias UFs; o XSD `TIe` é `[0-9]{2,14}` (`tiposGeralMDFe_v3.00.xsd:405-413`). Organização com IE em mais de uma UF pode emitir com a IE de outra UF (F73/209), e "ISENTO" viola o padrão (F72 só olha vazio/zeros, `rules_mdfe.go:563-569`).
- Correção: reaproveitar `getIEForUF` com a UF de `enderEmit`; recusar IE não numérica no portão (F72 como block quando IE vazia ou `ISENTO`).

**M-03. `tpValePed` do cadastro de pedágio: 01, 02, 03 contra o XSD 01, 04.**
- Onde: `api/internal/api/v1/dto.go:804-805`, `ui/src/lib/schemas/toll-providers.ts:12-14,22`, repasse em `api/internal/services/mdfes/emit.go:963`.
- Evidência: XSD `valePed/disp/tpValePed`: `enumeration 01, 04` (rodoviário). 02 e 03 passam no DTO e na UI e geram XML inválido; 04 (valor válido) não pode ser cadastrado. Já listado como E-10 no gap doc (linha 402) e não corrigido.
- Correção: enum `01|04` no DTO, no schema Zod e no rótulo da UI (rótulos 01 e 04 do MOC D15; texto do 04 precisa do MOC).

**M-04. Veículo "Outros" do cadastro vira `tpRod=99`, fora do XSD (01 a 06).**
- Onde: `ui/src/lib/schemas/vehicles.ts:3-10` (`WHEELSET_OPTIONS` com `99`), `api/internal/api/v1/dto.go:1341,1356` (`Wheelset` sem validação), `api/internal/services/mdfes/emit.go:1124-1126` (copia `wheelset` para `tpRod`).
- Evidência: XSD `veicTracao/tpRod`: `enumeration 01 a 06`. Código 99 não existe.
- Correção: usar os códigos do XSD no cadastro (Outros = 06 pelo enum; rótulos dos 01 a 06 precisam do MOC) e validar `oneof` no DTO.

**M-05. Exterior (`EX`) recusado na entrada e no encerramento.**
- Onde: `api/internal/validation/validators.go:13-15,92` (`uf` sem EX), `emit.go:30-32` (`uf_start`, `uf_end`, `route`), `api/internal/api/v1/mdfes.go:135` e `api/internal/services/shared.go:99-107` (`UFCode` sem EX nem 99), `events.go:175`.
- Evidência: XSD `TUf` inclui `EX`; `evEncMDFe/cUF` é `TCodUfIBGE_EX` com `99` (`tiposGeralMDFe_v3.00.xsd:130`). As funções `requireCIOT` e `resolveTpEmit` tratam EX (`emitter_type.go:43,59`), mas nada chega a elas. Já era E-gap da fase 0e (gap doc linha 488).
- Correção: aceitar `EX` em `UFStart/UFEnd/UFPer` e em `Close` (`cUF=99`), com `cMun` do exterior (a tabela de domínio precisa do MOC).

**M-06. Documento único exige `bulk_cargo` em qualquer modal e tipo de emitente, mas a regra F55 do catálogo só vale na condição `mdfePagGate`.**
- Onde: `api/internal/services/mdfes/emit.go:404-406` e `ui/src/components/mdfe/MdfeEmitForm.tsx:590` (`needsBulk = isSingleDoc`) contra `go-dfe/validate/rules_mdfe.go:421-424` e `rules_mdfe_helpers.go:251-266` (`mdfePagGate`: rodoviário e tpEmit 1/3, ou 2 com `tpTransp`).
- Evidência: um MDF-e aéreo, aquaviário ou ferroviário com um só DF-e, ou carga própria sem `tpTransp`, é recusado com 400 por uma exigência que o catálogo (transcrito do MOC) não faz. `prodPred/infLotacao` é opcional no XSD.
- Correção: exigir CEP só quando o catálogo exigir (mesma condição); confirmar o texto F55 no MOC (documento necessário: MOC 3.00b, regras F55, F55a, F55b).

**M-07. Documento referenciado não tem a situação conferida (F33, F40, F48).**
- Onde: `api/internal/services/mdfes/emit.go:741-790` (`resolveCargo` lê `item` e nunca olha `status`).
- Evidência: gap doc linhas 259 (P1: rejeições 673, 677, 657, determinísticas com a base própria). NF-e/CT-e cancelado, denegado ou rejeitado entra no manifesto.
- Correção: recusar documento cuja linha local não está autorizada (com a exceção de contingência que o MOC prevê, F31/F32/F46/F47/F33/F40/F48 não se aplicam a documento em contingência).

**M-08. A UI só emite MDF-e de NF-e; vários grupos da API não têm tela.**
- Onde: `ui/src/components/mdfe/MdfeEmitForm.tsx:515` (`docType: 'nfe' as const`), `:662-665` (`type: 'nfe'`), `ui/src/components/mdfe/DocumentPicker.tsx` (só NF-e).
- Evidência: `grep` em `ui/src/components/mdfe` e `app/mdfe` não encontra `partial_deliveries`, `transported_mdfes`, `additional_info` (infCpl), nem seleção de CT-e. Com isso `tpEmit=1` (prestador, só CT-e, F14) fica inalcançável pela UI, e a regra F113 da UI (`EmitterTypeField.tsx`) nunca dispara para tpEmit 1. Objetivo do dono: 100% dos casos.
- Correção: seletor de tipo de documento (NF-e ou CT-e) no formulário, campos de observação (`infCpl`), entrega parcial e MDF-e transportado; guia e `nav.tsx` na mesma alteração.

**M-09. `veicReboque/prop` não tem caminho de emissão, embora F104 (719) exista no catálogo.**
- Onde: `api/internal/services/mdfes/builder.go:307-327` (reboque sem `prop`), `rules_mdfe.go:741-749`.
- Evidência: XSD `veicReboque/prop` (CPF|CNPJ, RNTRC, xNome, IE+UF, tpProp). Reboque de terceiro não pode ser declarado; a SEFAZ exige o proprietário quando ele não é o emitente.
- Correção: aceitar `owner` por reboque (cadastro do veículo já tem `owner`) e emitir `prop`.

**M-10. Regras de maior frequência sem pré-checagem: F85 a F88 (não encerrados), F44, F25.**
- Onde: catálogo `go-dfe/validate/rules_mdfe.go` (IDs ausentes: F01, F07, F09, F12, F25, F31, F32, F33, F38, F39, F40, F44, F46 a F50, F73 a F76, F78, F81 a F88, F97, F109 a F112, F119, F120, F122; PAA02 a PAA06); `api/internal/services/mdfes/consulta.go:127` (existe `ListUnclosed`, não ligado a `Emit`: `grep ListUnclosed emit.go` vazio).
- Evidência: gap doc linhas 274 (F85 a F88, "as rejeições mais comuns do dia a dia") e 261 (F44, 648, determinística: `infMDFeTransp` com UF AM ou AP). Não há teste de completude do catálogo contra a lista do MOC (planejado como F5 no plano, não existe).
- Correção: ligar F85 a F88 a `ListUnclosed` com cache curto; implementar F44; criar o teste de completude (arquivo de IDs do MOC com justificativa "por construção" ou "externa" por regra).

**M-11. Ferroviário: `dhTrem` sai sem segundos e sem fuso.**
- Onde: `ui/src/components/mdfe/ModalFields.tsx:106-108` (`datetime-local`), `MdfeEmitForm.tsx:659` (`rail` enviado cru), `api/internal/services/mdfes/modals.go:143,159`.
- Evidência: XSD `dhTrem` é `TDateTimeUTC` (`T hh:mm:ss` mais fuso). `datetime-local` entrega `AAAA-MM-DDTHH:MM`. O DTO é `omitempty` sem formato. Contraste: `trip_start` recebe `:00-03:00` (`MdfeEmitForm.tsx:661`).
- Correção: normalizar no serviço (RFC 3339) e validar o formato; no portão, ver M-01.

**M-12. Grupos do XSD sem nenhum caminho de emissão (escopo declarado fora): `infSolicNFF`, `infPAA`, `procEmi=4`, `tpEmis=3`, e os mesmos grupos em `infEvento`.**
- Onde: `builder.go:19` (`procEmiOwn = "0"`), `builder.go:24-25` (constantes sem uso de emissão), ausência de `infSolicNFF`/`infPAA` no builder e em `buildEventEnvelope` (`events.go:268-293`).
- Evidência: cobertura por script (§4): único conjunto de caminhos do XSD sem emissão. As regras F72, F123, PAA01 já existem no catálogo.
- Correção: decisão do dono (gap doc fase 10, 12+ dias); enquanto isso, registrar em CONDUCT.md que NFF e PAA estão fora do escopo.

### Minor

- **m-01. `infLotacao`: CEP e latitude/longitude juntos violam o `choice`.** `builder.go:409-420` coloca `CEP` e, se vierem, `latitude/longitude` (DTO `emit.go:326-333`, sem validação). Executado: `xmllint` recusa (`latitude: This element is not expected`). Só por API (a UI só envia CEP). Corrigir: um ou outro (e latitude com longitude), formato ER37/ER38 na entrada.
- **m-02. Contratante obriga `infPag`.** `emit.go:204-209` (`validateFreightDeclaration`) contra XSD `infPag minOccurs=0` e F113b (infPag exigido para TAC, gap doc linha 248). F65 (`rules_mdfe.go:509-520`) exige contratante quando há `prop`, então carga própria com proprietário PJ (ETC/CTC) fica obrigada a informar pagamento a um beneficiário com dado bancário. Confirmar F113b no MOC antes de afrouxar.
- **m-03. DAMDFE sem rótulo para `tpCarga=12`.** `api/internal/services/documents/mdfe.go:16-21` (só 01 a 11); XSD `tpCarga` vai a 12. Item D-5 do gap doc, aberto.
- **m-04. DAMDFE nunca mostra o protocolo de MDF-e autorizado em contingência.** `documents/mdfe.go:71-75` (`!contingency && prot != nil`) e `_damdfe_macros.html` (carga_e_fisco). Item D-3 do gap doc, aberto. Corrigir: faixa de contingência só enquanto não houver `infProt`.
- **m-05. Texto de homologação com travessão.** `documents/mdfe.go:113` usa travessão curto (U+2013); o MOC transcrito (gap doc linha 359) usa "-". Conferir no Anexo II.
- **m-06. DAMDFE prepara dados que o template não imprime, e não imprime vários grupos do XML.** Preparados e sem uso em `templates/_damdfe_macros.html`: `ciot`, `tpEmit_label`, `tpTransp_label`, `veic.renavam/tara/capkg`, `reboques.renavam/tara`, `tot.vCarga`. Grupos sem representação: `valePed`, `infContratante`, `infPag`, `autXML`, `infLotacao`, `peri`, `infUnidTransp`, terminais e MMSI (aquav), `vag` (pesos, tipo, TU), entrega parcial. O gap doc (D-6) pede conferir contra o Anexo II. Sem o Anexo II não dá para dizer quais são obrigatórios (ver §5).
- **m-07. `indAntecipaAdiant` aceita "0".** `emit.go:225` e `payment_events.go:75` (`oneof=0 1`); XSD só `enumeration 1`.
- **m-08. Limites de `infPag` diferentes do XSD.** `payment_events.go:52` `PIX max=250` (XSD 2 a 60; testado: `min_length` do validador gerado dispara com PIX curto, o DTO não impede 61 a 250); `payment_events.go:49` `codBanco len=3` (XSD 3 a 5); `ForeignID max=20` sem mínimo 5 do XSD de `infPag`.
- **m-09. Validadores mais frouxos que o XSD (passam no DTO, falham no schema).** `validators.go:34` `rntrc ^\d{8,12}$` (XSD `[0-9]{8}`, também `ui/src/lib/schemas/vehicles.ts` `RNTRC deve ter 8 a 12 dígitos`); `money` com até 4 decimais em `v_vale_ped`/`contract_value` (XSD 13,2; `10.5` viola `TDec_1302`); `decimalv` em `tara`/`cap_kg` (XSD inteiro) e em todos os valores de `infPag`, que são repassados sem normalizar (`buildInfPag`).
- **m-10. Ferroviário e aquaviário: DTO e UI com limites do XSD errados.** `modals.go:144-145` e `ModalFields.tsx:111,115` `xOri/xDest` até 100 (XSD 1 a 3; o validador gerado dispara `max_length` e o `Gate` recusa com 422, mas a UI deixa digitar); `nSeq max=2` (XSD até 3 dígitos, mais restritivo); `nVag max=9` (XSD 8); `serie` e `tpVag` com `max=3` (XSD `length=3`); `cPrtEmb/cPrtDest len=5` (XSD 1 a 5); `nViag` texto livre (XSD `[1-9][0-9]{0,9}`); `nVoo` sem mínimo 5.
- **m-11. Eventos com conferência local mínima.** `events.go:146` conta bytes (`len`) e não caracteres para os 15 de `xJust`, sem máximo 255; sem a janela de 24 h do cancelamento (K, 220); `Close` não confere `cMun` x `cUF` (614) nem formato do município; `IncludeDFe` só `len==44` (`events.go:217`); inclusão de condutor sem checar modal rodoviário (644). Já listados em B.2 do gap doc.
- **m-12. Cardinalidades do XSD sem limite no DTO.** `condutor` 1 a 10 (`emit.go:45` `Drivers` sem `max`), `infPercurso` até 25 (`Route` sem `max`), `infMunCarrega` até 50, `infMunDescarga` até 1000, `veicReboque` 3 (ok), `autXML` 10 (ok). O validador gerado acusa `max_occurs`, mas só em log (M-01).
- **m-13. IDs de regra trocados em comentários e docs.** `builder.go:220-221` e `CONDUCT.md:572`/`DOCS.md:2212,2216` chamam a regra do `tpTransp` de F25 (745); no catálogo é F20 (`rules_mdfe.go:275`) e F25 é 712 (gap doc linha 290). `DOCS.md:2212` diz "Owner must differ from the emitter (F21)"; F21 é carregamento posterior, a regra é F64 (740). Corrigir os textos.
- **m-14. Modo `warn` não tem canal até o usuário.** `go-dfe/validate/rules.go:14` define warn como "aviso ao usuário"; `Shadow` só faz `slog.Warn` e `Gate` só devolve block. `grep -i warning` em `api/internal/services/mdfes` e `dfecheck` não acha nada. Na prática, cerca de 55 das 95 regras são só log. Sem métrica nem alarme (CLAUDE.md raiz já registra).
- **m-15. `SegCodBarra` nunca é emitido.** `builder_infdoc.go:63-65`; F35/F41 (`rules_mdfe.go:347,375`) acusariam documento FS-DA sem o campo. Coerente com a correção da fase 0, mas então um documento FS-DA referenciado é inemitível.
- **m-16. `categCombVeic` só 02, 04, 06, 07.** `builder_antt.go:57-69`; XSD tem 02, 04, 06, 07, 08, 10, 11, 12, 13, 14 (por eixos). Já documentado (gap doc linha 125); o rótulo MOC por eixo precisa do D-table.
- **m-17. UFs derivadas por heurística.** `emit.go:830-837`: `UFFim` = UF de destino do **último** documento; `UFIni` = UF do primeiro. Com destinos em UFs distintas, F11 (`rules_mdfe.go:214-230`) e F90 acusam só em warn.
- **m-18. Fallbacks silenciosos no emitente.** `builder.go:491-493` (`nro=S/N`, `cMun=0000000`) e campos vazios podados por `PruneEmpty` (xLgr, xBairro, xMun, UF viram `required` no XSD; só log, M-01). F78 (408) não existe no catálogo.
- **m-19. Campos de domínio codificado como texto livre na UI.** `ModalFields.tsx` (`tpEmb`, `tpVag`, portos, terminais, balsas); regra de memória do projeto "pickers em vez de digitar". Os domínios (tipo de embarcação, UN/LOCODE) não estão nos XSDs; precisam do MOC (§5).
- **m-20. Fuso fixo `-03:00` em `trip_start` e eventos.** `MdfeEmitForm.tsx:661`, `mdfes.go:364-367`, `events.go:234`. O XSD aceita outros fusos; o emitente do AM, AC ou MT inicia em outro offset. `dhEmi` usa o relógio do servidor (`emit.go:540`) sem converter para o fuso do emitente (ver §5).

## 3. Cobertura de tags: o que o builder não consegue emitir

Comparação de caminhos do XSD (documento mais os 4 modais, 315 caminhos) contra o que a emissão cria (execução em §0 mais leitura do código):

| Grupo | Situação | Achado |
|---|---|---|
| `infSolicNFF`, `infPAA` (e em `infEvento`) | sem caminho | M-12 |
| `veicReboque/prop` e filhos | sem caminho | M-09 |
| `infMunDescarga/*/SegCodBarra` | sem caminho | m-15 |
| `procEmi=4`, `tpEmis=3` | sem caminho | M-12 |
| Demais caminhos (ide, emit, modais, infDoc, seg, prodPred, tot, lacres, autXML, infAdic, infRespTec, infMDFeSupl) | emitidos e válidos no XSD nas execuções | ok |

Verificado e correto (executado): ordem `xsdorder` de ide, emit, infDoc, seg, prodPred, tot, rodo, aereo, aquav, ferrov; escolha CNPJ/CPF do emitente; `infCIOT` com CPF ou CNPJ alfanumérico; `infPag` a prazo com `infPrazo`; `infContratante` com CNPJ, CPF e `idEstrangeiro`; `valePed` com `CNPJPg`; `autXML`; `infMDFeTransp`; QR Code com e sem `sign` (padrão `qrCodMDFe`); eventos 110111, 110112, 110114, 110115, 110117, 110118 (110116 só falha por B-01). `reboque` sem `capKG` viola o XSD, mas o cadastro exige `cap_kg` do reboque (`emit.go:1163`, `Missing`), então só ocorre se o cadastro for burlado.

## 4. Regras do catálogo: modos e condições

Revisão das 95 regras de `rules_mdfe.go` contra o gap doc. Sem erro de condição provado nas regras transcritas. Observações:

- Block (40 regras promovidas em `prevalidate.go:19-40`, mais as de modo block do próprio catálogo): DV da chave, CPF/CNPJ, contagens, duplicidades, F14 a F17, F21 a F24, F26, F27, F58, F62. Todas são determinísticas com o documento; nenhuma regra de relógio ou tabela externa foi promovida. Sem objeção. Risco a vigiar: F30/F37/F45 são block e dependem dos conjuntos de `tpEmis` válidos (`mdfeTpEmisCTe "14578"`, `mdfeTpEmisNFe "12345679"`, `mdfeTpEmisMDFe "123"`, `rules_mdfe.go:32-34`), cuja lista não está no gap doc. Se algum valor for omitido, o block recusa documento legítimo. Verificar contra o MOC (F30, F37, F45).
- `mdfeUFBorders` (F90, `rules_mdfe_helpers.go:44-54`): tabela de divisas sem fonte no repo (o MOC não a publica). Revisada por plausibilidade; só gera aviso.
- F113 (`rules_mdfe.go:792-797`) usa `mdfePagGate`, mais amplo que o bloqueio de `requireCIOT` (`emitter_type.go:55-63`: só tpEmit 1 e 3). Está documentado em `DOCS.md:2232`.
- F55b (`rules_mdfe.go:429`) e `Emit` divergem: ver M-06.
- Faltantes: lista em M-10.

## 5. Não pude verificar (documento necessário)

| Item | Documento |
|---|---|
| Texto das regras F/K, domínios D1 a D21, ER1 a ER61 (só há a transcrição do gap doc de 2026-11-04; reconferi o que o código usa, não o MOC) | MOC MDF-e 3.00b, Anexo I (leiaute e regras) e tabelas |
| Obrigatoriedade de cada quadro do DAMDFE, tamanhos, o verso, modelos por modal e em contingência | MOC MDF-e, Anexo II (DAMDFE) |
| Rótulos de `tpRod` 01 a 06, `tpValePed` 01/04, `categCombVeic` por eixos, `tpEmb`, `tpUnidTransp` | MOC MDF-e, tabelas D |
| Se `dhEmi` em UTC (servidor) é aceito e consistente com o AAMM da chave | MOC MDF-e, ER1 e regra do Id (F03) |
| `cStat 134` fecha o ciclo de cancelamento/encerramento corretamente quando o MDF-e está "em situação diferente de autorizada" (`worker/internal/service/helpers.go:134-137` trata como sucesso e leva o documento a cancelado/encerrado) | MOC MDF-e, RecepcaoEventoGeral |
| Comportamento real da SEFAZ (nenhuma chamada foi feita; só XSD local) | Homologação SEFAZ/SVRS |
| Divisas entre UFs (F90), municípios IBGE (F09, F12, F78) | Tabela IBGE e divisas, fonte oficial |
| F113a e F113b (TAC), categoria do RNTRC | NT 2026.001 e cadastro ANTT |
| NT do MDF-e posterior a 1.05 (o XSD vendorizado é de 25/04/2026) | Portal SVRS, MDF-e |

## 6. Próximos passos sugeridos (ordem)

1. B-01, M-03, M-04, M-11, m-07 (enums e formatos que já violam o XSD hoje) com teste de regressão por `xmllint` no CI (script em `scratchpad/mdfe-audit/val.py` serve de base).
2. M-01 (promover códigos de schema ao `Gate` e criar o gate de eventos), depois M-02, M-05, M-06, M-07.
3. M-08 e M-09 (cobertura de UI e reboque de terceiro), M-10 (F85 a F88 e teste de completude).
4. Decisão do dono sobre M-12 e atualização de `CONDUCT.md` (IDs trocados, m-13).

## 7. Status das correcoes (2026-11-09)

Corrigidos, cada um com teste de regressao (paridade contra os specs gerados do XSD via `dfecheck`, ou teste unitario):

| Achado | Correcao |
|---|---|
| B-01 | `payTransportDet` preenche `qtdViagens` e `nroViagem` com zeros a esquerda (5 digitos); teste `TestEventBodies_PagamentoViagensSemZeros` |
| M-02 | `emit/IE` so da UF do `enderEmit` (`services.IEForUF`), `ISENTO` e IE de outra UF omitem a tag |
| M-03 | `tp_vale_ped` `01|04` no DTO, Zod e rotulos; cadastro legado 02/03 e recusado na emissao (400) |
| M-04 | `wheelset` `01..06` no DTO e na UI (Outros = 06); legado `99` sai como `06` |
| M-05 | `ufex` em `uf_start`, `uf_end`, `route` e no encerramento (`cUF=99`) |
| M-07 | documento cancelado na base e recusado em `resolveCargo` (`denegado` nao existe como situacao local) |
| M-09 | `owner` do reboque cadastrado vira `veicReboque/prop` |
| M-11 | `dhTrem` normalizado para `TDateTimeUTC` (segundos, fuso `-03:00` por padrao) e validado |
| m-01 | `infLotacao` CEP ou lat+lon (validacao e builder) |
| m-03, m-04 | DAMDFE: rotulo `tpCarga=12` ("Granel pressurizada", texto do XSD); protocolo impresso no off-line ja autorizado |
| m-05 | travessao curto do texto de homologacao trocado por hifen (MOC transcrito) |
| m-07, m-08, m-09 | `indAntecipaAdiant` so `1`; PIX 2..60; `codBanco` 3..5; `idEstrangeiro` 2..20; valores de `infPag`/vale/contrato `money2`; RNTRC 8 digitos (DTO e Zod) |
| m-10 | ferroviario: `xOri`/`xDest` max 3, `serie`/`tpVag` `len=3`, `nVag` max 8 numerico, `nSeq` max 3 numerico (DTO e UI) |
| m-12 | limites `condutor` 10, `infPercurso` 25, `infMunCarrega` 50, `infMunDescarga` 1000 |
| m-13 | IDs de regra corrigidos (F20 em vez de F25; F64 em vez de F21) em codigo, DOCS e CONDUCT |

Nao corrigidos:

- M-01 corrigido (padrao shadow; promover tipo a tipo com `block`): Gate de schema em block (`DFE_GATE_SCHEMA_MDFE`) e gate de eventos (`GateEvent`, `DFE_GATE_SCHEMA_MDFE_EVENT`), testes `gate_schema_test.go`. Pendente: F44/F85 a F88 no catalogo (abaixo).
- F44/F85 a F88 no catalogo: pertencem ao agente do Gate/validador. F44: o texto transcrito ("modal aquaviario e UF AM ou AP") nao diz se a UF e de carga, descarga ou ambas; F85 a F88 tem texto truncado no gap doc e dependem de `ListUnclosed` (nao ligado ao `Emit`; a regra exige a tabela local e a consulta, sem texto completo no repo).
- M-06 (F55): a evidencia do relatorio e a propria transcricao do gap doc, que nao fixa o modal/tpEmit; pergunta aberta: o texto F55 do MOC 3.00b exige `infLotacao` fora da condicao `mdfePagGate`?
- M-08 (UI so emite NF-e) e M-12 (`infSolicNFF`, `infPAA`, `procEmi=4`, `tpEmis=3`): backlog separado, nao construidos.
- m-20 (corrigido): `dhEmi`, AAMM da chave, `dhEvento`, `dtEnc` do encerramento e "gerado em" do DAMDFE usam `go-dfe/uftz` (UF do emitente; no encerramento, a UF de encerramento). `dhTrem` sem fuso assume o fuso da UF do emitente (antes `-03:00` fixo); fuso informado pelo usuario nao e alterado. `trip_start` nao tem default no codigo (so repassa o valor do usuario), nada a mudar. Regressao: `mdfes/moc_uftz_test.go` e `modals_test.go`.
- m-02, m-06, m-11 (24 h, `cMun` x UF), m-14 a m-19: sem texto de regra ou Anexo II no repo, ou fora do escopo pedido.
- Teste do `cUF=99` no encerramento nao foi escrito (`Close` exige repositorios/egress); a constante e o ramo sao triviais.
