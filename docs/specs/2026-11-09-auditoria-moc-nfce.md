# Auditoria MOC x implementação: NFC-e (modelo 65)

Data: 2026-11-09. Somente leitura (nenhum código alterado). Estado do repositório: `main` em `0d8fce3`.

## 1. Escopo e fontes

Fontes usadas (somente estas):

- XSDs vendorizados `go-dfe/schemas/nfe/PL_010e_v1.02` (`leiauteNFe_v4.00.xsd`, `e110111`, `e110112`; integridade em `go-dfe/schemas/SHA256SUMS` e `go-dfe/schemas/README.md`).
- `docs/specs/2026-11-04-moc-gap-nfe-nfce.md`, `docs/plans/2026-11-04-moc-cobertura-total.md`, `docs/specs/2026-11-08-contingencia.md`. O arquivo `docs/specs/2026-11-04-moc-cobertura-total.md` citado no pedido não existe; só o plano homônimo em `docs/plans/`.
- `/home/artur/Downloads/NT2025.001_v1.00 - NFCe_qrCode_3.pdf` (14 páginas; citada como "NT p.N").
- `/home/artur/Downloads/20250324 - Manual_de_Especificações_Técnicas_do_DANFE_NFC-e_QR_Code.pdf` (40 páginas, versão 6.0 de março de 2025; citado como "Manual item X, p.N" pela numeração do rodapé).

Resultado verificado (rodei): `go test ./validate -run 'Parity|SchemasHash'` em `go-dfe` passa; a ordem de `xsdorder` para `NFe`, `infNFeSupl`, `ide`, `pag` confere com o XSD.

## 2. Resumo

| Severidade | Quantidade |
|---|---|
| Blocker | 0 |
| Major | 9 |
| Minor | 11 |

Pontos que conferem (campo a campo, sem achado):

- QR v2 online: `chave|2|tpAmb|idCSC|SHA1(chave|2|tpAmb|idCSC+CSC)` em hex maiúsculo (`nfce_qrcode.go:153-160`) igual ao Manual item 4.3.4 e ao exemplo 4.3.6.1 (p.25-26).
- QR v2 offline: `chave|2|tpAmb|dia|vNF|hex(DigestValue)|idCSC|hash`, `digVal` com 56 caracteres (hex do texto base64 de 28 caracteres), hash sobre parâmetros 1 a 7 mais CSC (`nfce_offline.go:326-339`), igual ao Manual 4.3.2/4.3.5 e ao exemplo 4.3.6.2 (p.23-28); coberto por `TestQRV2OfflineManualExample`.
- QR v3 online `chave|3|tpAmb` (`nfce_qrcode.go:163-166`) e v3 offline `chave|3|tpAmb|dia|vNF|tp_idDest|idDest|assinatura` com RSA SHA-1 Base64 sobre os parâmetros 1 a 7 mantendo os `|` e separadores vazios para consumidor não identificado (`nfce_offline.go:227-250`, `go-dfe/dfe.go:149`), iguais ao Manual 4.4.1/4.4.2 (p.30-31) e ao padrão `QRCODE V3 OFFLINE` do XSD (`leiauteNFe_v4.00.xsd:6665`).
- Imposição de `indSinc=1` e lote único, `idLote` de 15 dígitos, `finNFe=1`, `indFinal=1`, `indPres=1`, `idDest=1`, `dest/indIEDest=9`, primeiro item com a descrição de homologação, `transp/modFrete=9`, `tpImp=4`, `dhCont`/`xJust` exigidos fora de `tpEmis=1`, nível de correção M do QR (`barcode.go:33`), eventos 110111 e 110112 na ordem do XSD (`e110111`, `e110112`).
- Ordem das tags e cardinalidade: validadas pelo teste de paridade com o XSD (não repeti a conferência manual).

## 3. Achados

### MAJOR

**M1. Chave de acesso de emitente CPF é montada com os zeros à direita**
- Onde: `api/internal/services/nfes/emit.go:971-976` (preenche `cnpj += "0"` até 14).
- Evidência: o catálogo de validação trata o CPF como `"000" + CPF` à esquerda: `go-dfe/validate/rules_nfe.go:104-107` (A03-10), e o mesmo em `rules_cte_helpers.go:173,207` e `rules_mdfe_helpers.go:178`. Um emitente CPF gera `CPF000` nas posições 7 a 20. A NT 2025.001 p.4 (item 02.2) e o Manual p.6 (histórico, item 3.1.1) tornam o emitente CPF (produtor rural) um caso real, e `nfce_qrcode.go:38-46` já o trata.
- Verificação dupla: (1) leitura de `generateAccessKey`; (2) o próprio validador A03-10 recalcularia a chave com `000` à esquerda e a recusaria. A rejeição não ocorre no portão local porque A03-10 não está em `blockRules` (`prevalidate.go:48-70`; o `Gate` só recusa o que o mapa promove); a SEFAZ recusaria (502).
- Correção proposta: preencher com zeros à esquerda (`strings.Repeat("0", 14-len(doc)) + doc`) para qualquer comprimento menor que 14, com teste de tabela para CPF; promover A03-10 e B23-10 a block (deterministas).
- Não verificável aqui: a regra textual do MOC para a posição do CPF na chave (MOC Anexo I, chave de acesso). Ver seção 5.

**M2. `dhEmi` sai no fuso do processo, não no fuso do emitente**
- Onde: `emit.go:1635-1637` (`fmtDhEmi`), `nfce_emit.go:152,240,279` (`now := time.Now()` formatado com o `Location` do processo). Nada define `TZ` nem `LoadLocation` para NF-e/NFC-e na API (`rg` em `api/` e `cdk/lib`). Contraste: `service.go:595-598` (`dhEvento`) fixa BRT.
- Evidência: o gap spec já registrava (`2026-11-04-moc-gap-nfe-nfce.md`, item 5 de "Datas e fuso") e o estado não mudou. Em Lambda (UTC) o XML sai `...+00:00` com a hora UTC; o XSD aceita `+00:00` (`TDateTimeUTC`, `tiposBasico_v4.00.xsd:552`), então passa no schema, mas: (a) o AAMM da chave e o dia do QR offline (`dhEmi[8:10]`, `nfce_offline.go:211-212,243`) viram do dia UTC, divergindo do dia local entre 21h e 24h de Brasília; (b) o DANFE-NFC-e imprime o relógio literal do XML (`format.go:83-99`), logo imprime hora UTC; o Manual item 3.1.7 (p.13) exige hora local.
- Verificação dupla: leitura de `fmtDhEmi`/`dateTimeBR`, mais ausência de qualquer configuração de fuso.
- Correção: um `time.Location` por UF do emitente (tabela de offsets; AM, MT, MS, RO, RR, AC têm fuso diferente) aplicado a `now` uma única vez em `Emit` e propagado para a chave, `dhEmi`, QR e `dhEvento`.

**M3. DANFE-NFC-e autorizada não imprime data e hora de emissão**
- Onde: `api/internal/services/documents/templates/danfce.html:196-209`. O ramo `{% if protocolo %}` imprime protocolo e data de autorização; `ident.dhEmi` só aparece no `else` (sem protocolo, isto é, contingência).
- Evidência: Manual item 3.1.7 (p.13) exige número, série, data e hora de emissão (B09) e protocolo com data e hora da autorização (PR08/PR09), todos juntos. `danfce_mensagem.html:25` imprime "Emissão", o template de papel não.
- Verificação dupla: leitura do template e do contexto (`nfce.go:102`, `ident.dhEmi` é preenchido mas só usado no ramo sem protocolo).
- Correção: imprimir "Emissão: {{ ident.dhEmi }}" fora do `if`, mantendo "Via do Estabelecimento" ao lado dela (Manual 3.1.8, p.14).

**M4. QR Code do DANFE-NFC-e abaixo do mínimo de 25 mm**
- Onde: `danfce.html:57-58` (`.qr { width: 20mm }`, `.qr-band { min-height: 20mm }`) e coluna de 40% do papel de 58 mm (`danfce.html:179-187`), cerca de 21,6 mm úteis.
- Evidência: Manual item 3.1.5 (p.11-12) e 3.4 (p.19): tamanho mínimo 25 mm x 25 mm, sendo 22 mm de conteúdo mais 3 mm de margem segura; para tamanho maior, margem de 10%. O `qrDataURI` (`barcode.go:29-38`) também não acrescenta quiet zone própria (`Scale` do QR cru), então o conteúdo efetivo é menor que 20 mm.
- Correção: layout centralizado (Figura 5 do Manual) com QR de no mínimo 25 mm no total, ou 28 mm para folga; garantir a margem segura na imagem.

**M5. DANFE-NFC-e de nota offline já autorizada continua "Pendente de autorização" e sem protocolo**
- Onde: `documents/nfce.go:64-69` (`isContingency := tpEmis == "9"`; protocolo só se `!isContingency`), `danfce.html:82-84,215-217`.
- Evidência: `docs/specs/2026-11-08-contingencia.md` seção 9.3 afirma que "o PDF é regerado, com protocolo, depois da autorização". O código nunca usa o `infProt` quando `tpEmis=9`; `renderer_test.go:83` fixa o comportamento mesmo com `protNFe` no XML. O Manual 3.1.7/3.1.8 (p.13-14) manda suprimir o protocolo "no caso de emissão em contingência" e define o texto "Pendente de autorização" para o cupom impresso na emissão; não define a reimpressão após a autorização.
- Verificação dupla: leitura do código mais o teste existente.
- Correção: decidir pelo estado do documento (presença de `infProt` com cStat 100/150), não só por `tpEmis`: com protocolo, imprimir protocolo e omitir a tarja; manter as duas vias só enquanto pendente.

**M6. UI e API discordam sobre `tPag`; a UI oferece códigos que a API recusa e que a NFC-e rejeita**
- Onde: `ui/src/lib/types/api.ts:1410-1434` e `ui/src/lib/data/payment-options.ts` (lista completa, incluindo 14, 23, 24, 90, 91) contra `api/internal/services/nfes/emit.go:277` e `api/internal/api/v1/dto.go:439` (`oneof=01 02 03 04 05 10 11 12 13 14 15 16 17 18 19 20 21 22 90 99`).
- Evidência: 23 (PIX automático), 24 (TEF book transfer) e 91 (pagamento posterior) existem na tabela vendorizada na própria UI (`ui/src/lib/data/payment-tables.ts:22-46`, "Portal Nacional, 04/03/2026") e o catálogo cita o 91 (`rules_nfe.go` YA03-30, YA03-10). A API devolve 400 para eles. Além disso 14 e 90 são oferecidos na tela da NFC-e (`NfceEmitForm.tsx:556-565`) e as regras YA02-10 (857) e YA02-40 (899) as recusam em NFC-e (`rules_nfe.go:1458-1459`), mas ficam só em Warn e fora de `blockRulesNFCe` (`prevalidate.go:76-84`): o número e a cota são gastos antes da rejeição.
- Correção: uma lista única de `tPag` por modelo (a mesma na API, na UI e no `paymentLabels` do DANFE); filtrar 14 e 90 para o modelo 65 e promover YA02-10 e YA02-40 a block (se o Aplic. for "Obrig." no MOC; ver seção 5).

**M7. A NFC-e emite grupos e CST/CFOP que o leiaute veda, com trava apenas em Warn**
- Onde: `api/internal/services/nfes/builders_doc.go:574` (IPI), `:593` (II), `:597,600` (PISST, COFINSST), `:584` (ICMSUFDest) sem condicionar a `isNFCe` (`:258`, usado só para idDest e descrição de homologação); regras O01-10, P01-10, R01-10, T01-10, J01-10, L01-10, I08-150, N12-30 a N12-44, N12a-* em `rules_nfe.go:378-560` todas em `ModeWarn`, e nenhuma em `blockRulesNFCe` (`prevalidate.go:76-84`). `nfce_emit.go:122` só checa o prefixo `5` do CFOP.
- Evidência: o gap spec reconhece ("não há trava se o produto traz o dado", linha 271). O catálogo cita as rejeições (742, 743, 746, 749, 736, 738, 725, 766, 382, 386); a mesma produto/perfil usado numa NF-e pode carregar `ipi_cst`, `ii` ou CST 10/30/70 e a NFC-e sai com o grupo.
- Correção: no builder, omitir IPI, II, PISST, COFINSST, ICMSUFDest, impostoDevol, `veicProd`, `arma` no modelo 65 (ou recusar com 422 nomeando o item); promover a block as regras de grupo proibido e as de CST/CSOSN x CFOP que o MOC marca como "Obrig.".

**M8. Linha "Acréscimos" do DANFE imprime só o frete**
- Onde: `danfce.html:144-147` (`R$ {{ totals.vFrete }}`); contexto em `documents/nfce.go:94-98` (`vSeg`, `vOutro` calculados e não usados).
- Evidência: Manual item 3.1.3 (p.10-11): "Acréscimos (frete, seguro e outras despesas) / Desconto: somatório dos valores"; "Valor a Pagar" só quando existir acréscimo ou desconto, e a linha só se existir um deles. O template imprime as duas linhas sempre que há qualquer um (`has_acrescimo_desconto`) e "Valor a pagar" sempre (`danfce.html:149-152`).
- Correção: `acrescimos = vFrete + vSeg + vOutro`; imprimir cada linha só se o valor for diferente de zero; "Valor a pagar" conforme o Manual.

**M9. Valor unitário do item arredondado para 2 casas**
- Onde: `documents/nfce.go:44` (`moneyBR(prod.value("vUnCom"))`) e `format.go:20-25` (`StringFixed(2)`).
- Evidência: `vUnCom` é `TDec_1110v` (0 a 10 casas, `leiauteNFe_v4.00.xsd:1053-1056`). Combustível (fluxo suportado em `comb.go` e `applyFuelPumps`) usa 3 e 4 casas; 4,899 imprime "4,90". Manual 3.1.2 (p.9) exige o "valor unitário (I10a)" e o item 2 (p.7) proíbe informação que não consta do XML. `qCom` sai com ponto e 4 casas ("1.0000"), fora do padrão de vírgula decimal citado no mesmo item.
- Correção: formatar com as casas do XML (milhar com ponto, decimal com vírgula) para `vUnCom` e `qCom`; manter 2 casas só para valores monetários totais.

### MINOR

**m1. DANFE não imprime emitente CPF**
`danfce.html:76` fixa "CNPJ:" e `documents/nfce.go:89` lê só `CNPJ`. O Manual item 3.1.1 (p.6 histórico e p.9) prevê CPF com máscara 999.999.999-99 (C02a). Com M1 corrigido, a nota sairia sem documento do emitente no cupom.

**m2. Validação de CSC incompleta; condição morta**
- `nfce_emit.go:99` converte o id do CSC com `%d` (vira "0" quando ausente) e `nfce_qrcode.go:134` testa `cscID == ""`, que nunca é verdadeiro; com CSC preenchido e id ausente o QR sai com `idCSC=0`.
- `dto.go:1431-1436`: `prod_csc` só tem `max=36`; o Manual item 4.6 (p.32-33) diz 16 a 36 caracteres alfanuméricos e id de até 6 dígitos (o XSD aceita `0|[1-9][0-9]{0,5}`, `leiauteNFe_v4.00.xsd:6659`). Falta `min=16`, `alphanum` e `lte=999999`.

**m3. Lacunas do catálogo para o grupo ZX da NT 2025.001**
`rules_nfe.go:1741-1764` tem ZX02-10, -220, -222, -228, -236, -244. Faltam (NT p.10-12): ZX02-268 (397, dia do QR diverge de `dhEmi`), ZX02-276 (397, `vNF` do QR diverge), ZX02-328 (397, tipo e identificação do destinatário diverge), ZX02-330 (445, assinatura em `tpEmis<>9` na v3), ZX02-334 (474, assinatura ausente em `tpEmis=9` na v3); todas são deterministas e protegeriam o QR offline que nós mesmos montamos. Falta também YA04-10 (391, cartão/PIX sem grupo `card`, NT p.9, facultativa por UF, então Warn). Colisão de identificador: o catálogo mapeia `ZX02-220` para "NF-e 55, rejeição 672" (`rules_nfe.go:1745`), enquanto a NT p.10 define ZX02-220 como modelo 65, rejeição 407 ("v3 e UF não aceita"). A 407 depende de parâmetro por UF e não é pré-validável (ver seção 5).

**m4. C02a-04 contradiz a NT**
`rules_nfe.go:502-504` avisa "NFC-e com CPF do emitente" (337), mas a NT 2025.001 (p.4, item 02.2; p.10, ZX02-222) e o Manual (histórico, março/2025) admitem emitente CPF na NFC-e. Hoje só gera aviso falso; deve ser removida ou restringida conforme o MOC atual.

**m5. Limite de R$ 10.000 do consumidor: `>` contra "igual ou superior"**
`rules_nfe.go:505-506` (W16-40) usa `vNF > 10000`. O Manual item 3.1.6 (p.12) diz "valor igual ou superior a R$ 10.000,00 (valor que poderá ser menor, a critério da UF)". A tela da NFC-e (`NfceEmitForm.tsx`) não aplica o limite de identificação. W16-60 (`rules_nfe.go:414`) exige endereço do destinatário acima de R$ 10.000, o que o Manual não pede (só identificação); ver seção 5.

**m6. Janela de retransmissão offline de 3 dias**
`services/contingency_offline.go:43` (`OfflineWindowNFCe = 3 * 24h`, decisão do dono). O gap spec (linha 434) registra o prazo como "final do primeiro dia útil subsequente"; as mensagens 234 e 235 do Manual (p.40) falam em 24 h. Um prazo de alerta mais longo que o legal atrasa o aviso. Tornar por UF ou usar o menor.

**m7. Rótulos de `tPag` incoerentes**
`documents/nfce.go:9-16` não tem 14, 23, 24, 91 (imprime o código cru); 05 e 18 divergem da tabela (`payment-tables.ts:22-46`: 05 "Cartão da Loja (Private Label)...", 18 "TED"); a própria UI diverge de si: `api.ts:1424` ("Transferência bancária, Carteira Digital") x `payment-tables.ts` ("TED"). Unificar na tabela do portal.

**m8. Fixture de teste do DANFE-NFC-e com `infNFeSupl` dentro de `infNFe`**
`documents/renderer_test.go:112`. No XSD `infNFeSupl` é irmão de `infNFe` (`leiauteNFe_v4.00.xsd:6641`). O renderizador usa `firstDeep` e não percebe. Corrigir o fixture para não mascarar regressão de posição.

**m9. E04-20 gera aviso falso na NFC-e**
`rules_nfe.go:643-646` exige o xNome de homologação do destinatário em 55/65 quando há `dest`; o builder da NFC-e nunca emite `xNome` (`builders_party.go:229-232`). Restringir a 55 ou aceitar `xNome` ausente (confirmar no MOC, seção 5).

**m10. Cabeçalho do DANFE sem complemento e CEP**
`documents/nfce.go:90` junta `xLgr, nro, xBairro, xMun, UF`; o Manual item 3.1.1 (p.9) pede "endereço completo do emitente sem a indicação do país". Acrescentar `xCpl` e `CEP`.

**m11. Documento de gap desatualizado sobre `transp`**
`docs/specs/2026-11-04-moc-gap-nfe-nfce.md` linha 270 diz que a NFC-e "não emite transp". O XSD exige `transp` (`leiauteNFe_v4.00.xsd:5635`, sem `minOccurs`) e o builder emite `transp/modFrete=9` sempre (`builders_doc.go`, montagem do `infNFe`). O comportamento do código está certo; corrigir o texto do spec.

## 4. Lacunas já documentadas e reconfirmadas (não contadas)

Estado igual ao gap spec; nada novo, só confirmação: consumidor só CPF (`nfce_emit.go:56-60`; sem CNPJ, `idEstrangeiro`, nome, endereço), `indPres` fixo 1 sem 4 e 5 (`nfce_emit.go:33-40`), sem `transp` de entrega a domicílio, sem EPEC na NFC-e (`nfce_offline.go:268-270`), DANFE sem `xMsg` do retorno da autorização (opcional pelo Manual 3.1.8).

## 5. O que NÃO foi possível verificar e documento necessário

| Item | Documento necessário |
|---|---|
| Posição e formato do CPF na chave de acesso (M1) | MOC Anexo I, composição da chave (Visão Geral) |
| Aplic. "Obrig." x "Facult." de YA02-10, YA02-40, O01-10, N12-30 a N12-44, I08-150 (decide o que promover a block, M6/M7) | MOC Anexo II, Regras de Validação (coluna Aplic.) |
| Texto atual de C02a-04, E04-20, W16-40, W16-50, W16-60 para o modelo 65 (m4, m5, m9) | MOC Anexo II atualizado após a NT 2025.001 v1.01 |
| Quais UFs aceitam QR v3 (rejeição 407) e comportamento de PR para emitente CPF (`nfce_qrcode.go:42` deixa PR passar com v2) | Portal Nacional NFC-e, "URL por UF utilizada no QR Code" e parâmetros por UF |
| URLs de consulta e de QR dos 27 estados (`nfce_qrcode.go:55-116`), http x https, IP em RR homologação | `nfce.encat.org/desenvolvedor/qrcode` e `/consumidor/consulteNota` |
| Atraso máximo de 5 minutos entre `dhEmi` e autorização da NFC-e (NT p.5, 02.4); a emissão é assíncrona via outbox, então um reenvio tardio mantém o `dhEmi` antigo | MOC Anexo II, regra B09 para o modelo 65 (cStat de atraso) |
| Prazo de transmissão offline por UF e dispensa da 2ª via | Legislação de cada UF / MOC Contingência NFC-e |
| Prazos e condições de cancelamento e de cancelamento por substituição da NFC-e (110111, 110112) | MOC, Eventos (prazo) e NT do evento |
| Meios de pagamento permitidos por modelo (tabela completa de `tPag`, 23, 24, 91 em NFC-e) | NT de meios de pagamento (tabela de 04/03/2026) |
| Reimpressão do DANFE-NFC-e de nota offline já autorizada (M5) | MOC DANFE NFC-e, `ContingenciaNFCe` |
| Mapa de ordem de tags para o ramo NFC-e do `det` além do que o teste de paridade cobre | Nenhum (já coberto), apenas confirmar o teste na CI |

## 6. Ordem sugerida de correção

1. M1 e M2 (identidade e data do documento; afetam chave, QR e DANFE).
2. M3, M4, M5, M8, M9 (DANFE conforme o Manual).
3. M6 e M7 (consistência de enums e travas locais, evitam gastar número e cota).
4. m2 a m11.

## 7. Status das correcoes (NFC-e, 2026-11-09)

Testes: `api/internal/services/nfes/nfce_moc_test.go`, `api/internal/services/documents/renderer_test.go`
(`TestNFCeContextManual`, `TestNFCeOfflineAuthorizedPrintsProtocol`), `api/internal/api/v1/dto_test.go`
(`TestNfceConfigCSCFormat`), `ui/src/__tests__/lib/data/payment-options-nfce.test.ts`.

| ID | Status | Nota |
|---|---|---|
| M1 | Ja corrigido (fase 1) | `generateAccessKey` usa `000`+CPF; promocao de A03-10 a block fica com o agente do Gate. |
| M2 | Corrigido | `nfce_emit.go` usa `uftz.Now(emitUF)`: `dhEmi`, AAMM e dia do QR offline no fuso da UF. DANFE converte `dhEmi` e `dhRecbto` para o fuso da UF do emitente. |
| M3 | Corrigido | data e hora de emissao impressas ao lado de numero e serie, com ou sem protocolo (e "Via do Estabelecimento" ao lado). |
| M4 | Corrigido | QR de 28 mm com margem de 10% por lado desenhada na imagem (conteudo 22,4 mm; total acima de 25 mm), centralizado (Manual figura 5). `cacheSchemaVersion` = v3. |
| M5 | Corrigido | protocolo impresso quando existe `infProt` com cStat 100 ou 150, mesmo com `tpEmis=9`; a tarja e a 2a via so enquanto pendente. |
| M6 | Parcial | UI da NFC-e nao oferece 14 e 90; API recusa 14 e 90 na NFC-e (400, antes de gastar numero); rotulos do DANFE da tabela do portal (com 14, 23, 24, 91). Lista unica da API e da UI ja vinha da fase 1. YA02-10 e YA02-40 continuam Warn no catalogo (agente do Gate). |
| M7 | Corrigido | builder omite IPI, II, PISST, COFINSST, `veicProd` e `arma` no modelo 65. `ICMSUFDest` ja nao sai (so com idDest 2 e NFC-e e sempre idDest 1); `impostoDevol` so existe com finNFe 4, que a NFC-e nao emite. CST 10/30/70 x CFOP nao tratado (sem fonte do Aplic.). |
| M8 | Corrigido | "Acrescimos" = vFrete+vSeg+vOutro; cada linha so com valor diferente de zero; "Valor a pagar" so com acrescimo ou desconto (Manual 3.1.3). |
| M9 | Corrigido | `vUnCom` e `qCom` com as casas do XML (virgula decimal, ponto de milhar); 4,8990 imprime 4,899. |
| m1 | Corrigido | DANFE imprime "CPF:" com mascara para emitente CPF. |
| m2 | Corrigido | id do CSC ausente fica vazio (a condicao `cscID == ""` passa a valer); DTO: CSC 16 a 36 alfanumericos, id ate 999999; mesmo na UI (zod). |
| m5 | Nao corrigido | o texto do Manual diz "igual ou superior" (3.1.6) mas W16-40 esta no catalogo do Gate (fora do escopo); a regra de W16-40 do MOC nao esta no repositorio. |
| m7 | Corrigido | `paymentLabels` da tabela do portal. |
| m10 | Corrigido | cabecalho com `xCpl` e CEP. |
| m3, m4, m6, m8, m9, m11 | Nao corrigido | catalogo/Gate (m3, m4, m9), decisao do dono (m6), fixture com `infNFeSupl` dentro de `infNFe` ainda mascara posicao (m8), texto do gap spec (m11). |
| Gate (NFC-e, transversal) | Corrigido | `dfecheck.GateEnviNFCe` com modo proprio (`DFE_GATE_SCHEMA_NFCE`, padrao `shadow`: rollout shadow primeiro, depois `block`): schema deterministico recusa; regras A03-10 e B23-10 continuam fora do mapa (decisao pendente, CONDUCT.md). |
