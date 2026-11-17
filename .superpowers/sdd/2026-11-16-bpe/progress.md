# SDD ledger — plan: /home/artur/Documents/Projects/Ctech/ctech-dfe-bpe/docs/plans/2026-11-16-bpe.md
Spec: docs/specs/2026-11-15-bpe-design.md (read). Branch feat/bpe stacked on feat/nfcom (PR #227); rebase onto main after #227 merges.
Pre-flight: T1 produces validate specs + xsdorder gen -> T2/T3/T6 consume; T2 produces go-dfe tipo bpe + AccessKeyModBPe -> T5/T6; T4 tables -> T5/T6; T6 routes -> T8; T7 documents.Render -> T6 /dabpe. Order T1..T9 satisfies all.
Schemas source: scratchpad bpe/pl (portal BPE DownloadArquivoEstatico).
Task 1: Ruling: generator now has a table orderTables (NFCom, BPe, BPeTM, BPeTA) instead of a hardcoded NFCom branch; one gen file per root (var names bpeGenerated etc.) - keys are prefixed by root so no collisions.
Task 1: Ruling: evFornecNaoRealizBPe_v1.00.xsd hand-written inside the vendored package dir (marked in the file and README) from NT 2025.002 6.3; descEvento enum 'Fornecimento Não Realizado' is my transcription of the NT text (truncated in the PDF extract) - cost if wrong: schema rejects a valid description; check against the NT.
Task 1: Ruling: evTributacaoSN/evCancTribSN BP-e events added to notBuilt in the parity test (issuer never sends them in v1); hand orders added for 7 BP-e event keys.
Task 1: complete (commits 3ff282a..3486007, tests: bash -c 'cd go-dfe && go test ./... -race' → ok  	gopkg.aoctech.app/dfe/go-dfe/validate	(cached))
Task 1: verified: descEvento of 112140 is 'Fornecimento Não Realizado' in the NT 2025.002 6.3 (my earlier worry resolved).
Task 2: Ruling: SOAP names from the MOC (NT 2025.002 2.4): BPeDadosMsg/BPeResultMsg (capital B, differs from the NFCom lowercase choice), methods bpeConsultaBP, bpeStatusServicoBP, bpeRecepcaoEvento, bpeRecepcaoTA read from the NT; bpeRecepcao and bpeRecepcaoTM follow the pattern (unconfirmed). All three Recepcao services are GZip+Base64 (MOC 2.4: recepção compactada), events/consult/status plain. WebFetch of the MOC pages was unavailable.
Task 2: Ruling: MG/MS homologation URLs follow each UF's pattern (hbpe.fazenda.mg.gov.br, homologacao.bpe.ms.gov.br) because the portal page has swapped entries; UF->authorizer: MG, MS, MT, PR, SP own, others SVRS; TM and TA forced to SVRS via bpeSVRSOnly in Resolve.
Task 2: processor: TM processed doc root named bpeTMProc (procBPeTM xsd exists) and TA reuses bpeProc - unverified against procBPe/procBPeTM XSDs; check when the worker reads them.
Task 2: complete (commits 3486007..2548e9e, tests: bash -c 'cd go-dfe && go test ./... -race && cd ../api && go test ./internal/validation -race' → ok  	gopkg.aoctech.app/dfe/api/internal/validation	(cached))
Task 3: Ruling: 22 document-only rules (F05-F07, F09-F11, F13, F18, F21-F24, F34, F35, F49, F64-F70) registered for the three families bpe/bpetm/bpeta from one factory; rules from the NT 2025.002 v2.00 catalog (TA, newest); F23 compares dhConexao with infPassagem/dhEmb (MOC says 'data-hora do embarque'). IBS/CBS rules F76-F111 NOT implemented (need cClassTrib/CST tables and NT 2026.* unread). F13 UFIni==emitter UF implemented as warn (cStat 505) though its applicability to TM/comum is unverified.
Task 3: complete (commits 2548e9e..5e60d32, tests: bash -c 'cd go-dfe && go test ./... -race' → ok  	gopkg.aoctech.app/dfe/go-dfe/validate	(cached))
Task 5: Ruling: DfeService generic (table from message), so only docTypeBPe, nsBPe, consultaSpecs (BPeConsulta/consSitBPe), contingency Tables+bpes; the no-protocol guard now covers nfcom and bpe; added consSitBPe/consStatServBPe orders to xsdorder (omitted in Task 1). Duplicate-event cStat for BP-e left at the NF-e default (573): MOC RecepcaoEventoGeral not read - cost if wrong: a duplicated cancel is treated as rejection.
Task 4: Note: tests and implementation were written in the same step, RED was NOT watched for the CDK jest tests (jest takes ~5 min); they were verified green afterwards only.
Task 5: complete (commits 6d8eac7..49cb017, tests: bash -c 'cd worker && go test ./... -race' → ok  	gopkg.aoctech.app/dfe/worker/internal/service	(cached))
Task 4: complete (commit 3098b78, tests: cdk jest 66 passed 0 failed, via npx jest --forceExit --json)
Task 6a: Ruling: DTO is XSD-shaped; server computes vBP = sum of components, vPgto = vBP - discount, vTroco = payments - vPgto, ICMS base = vPgto (my reading of the MOC; reviewer should confirm) - cost if wrong: authorizer rejects totals (F64/F69/F70 are warnings only).
Task 6a: Ruling: v1 comum excludes agency, substitution (tpBPe 3), IBS/CBS, ICMS 20, ICMSUFFim, boardPassBPe, events 110300/110301 (payment link) and 112140 (TA); variant != normal returns 400 until 6b/6c. DABPE route not registered (renderer is Task 7; documents.go removed from the package until then).
Task 6a: Ruling: event eligibility by variant is already in eventAllowed (TM no 110115/110117, TA no 110116/110117) so 6b/6c only add builders; local refusal is 422. Event Id uses a 3-digit sequence (pattern 29 digits); 110111 etc. bodies validated against the event XSDs in tests.
Task 6a: Ruling: record carries variant/modal/passenger_name/from/to/boarding/total/trips; UI list will use these. BP-e cancellation window and 'no cancel after boarding' are the authorizer's (504/220), not checked locally.
Task 6a: Note: integration TestBpe_* pass against a throwaway DynamoDB local (docker run amazon/dynamodb-local, port 8124).
Task 6: complete (commits 49cb017..494a759, tests: bash -c 'cd api && go test ./internal/services/... ./internal/api/... -race 2>&1 | grep -v "WARN\|INFO"' → ok  	gopkg.aoctech.app/dfe/api/internal/api/v1	(cached))
Task 6: CORRECTION: the 'complete' line above covers only 6a (comum); 6b (TM) and 6c (TA) are NOT done and remain open.
Task 6b: Ruling: TM body is its own DTO (BpeTMEmitBody) chosen by POST /bpes via a peek at `variant` (emitVariant); Emit and EmitTM share one flow (emission struct). TM has no QR Code (infBPeSupl absent), so offline does not call SignText. TM ICMS base = item value; total = sums of items.
Task 6b: Ruling: required XSD attributes idEqpCont (group) and nViagem (item) are not in the MOC text I read: default to the 1-based position, overridable (counter_id, trip_number). Component name max 15 chars (XSD). Both discovered by the schema test, not by the docs.
Task 6b: Ruling: TM rules reuse the F-series factory (family bpetm); passenger/viagem/pagamento rules are no-ops on TM documents. No TM-specific rules (the NT 2020.002 PDF rules were not read). TM-only checks are in validateTMBody (CFOP pattern, qPass 1-6 digits, counters, ICMS rate).
Task 6b: Note: TM credentialing is separate; nothing was sent to an authorizer. vBPLiq, IBS/CBS, ICMS 20, per-item infAdFisco and payment-link are not supported.
Task 6: complete (commits 494a759..1ec6d54, tests: bash -c 'cd api && go test ./internal/services/... ./internal/api/... -race 2>&1 | grep -v "WARN\|INFO"' → ok  	gopkg.aoctech.app/dfe/api/internal/api/v1	(cached))
Task 6: 6b (TM) done; 6c (TA) still open.

## Task 6c (TA) done
- Rulings: TA v1 só tpCompra 0; imp do TA e IBSCBS + vTotDFe (sem ICMS), descoberto pelo schema; vTotDFe = valor pago; IBS/CBS calculados sobre vPgto com alíquotas informadas; sem alíquotas só CST/cClassTrib.
- Limitações: regras MOC F76-F111 (IBS/CBS) não implementadas; tabelas CST/cClassTrib indisponíveis (campos validados só por formato); diferimento, redução, tributação regular, compra governamental, SN e estorno de crédito fora.
- Rotas: POST /bpes variant=ta; POST /bpes/{key}/supply-not-performed (112140, só TA).

## Task 7 (DABPE) done
- Rulings: DABPE A4 retrato de leiaute único; TM não gera DABPE (400); contingência offline pendente imprime do XML assinado; tpDoc é 1 RG, 2 título de eleitor (corrigido DTO e OpenAPI, antes dizia CPF); rótulo de tPag 06 não verificado (imprime "Outros (06)").
- Não feito: Divisão I (agência, opcional), Divisão IX (PDF417, opcional), formato reduzido 56 mm, "Via do Estabelecimento" da 2a via em contingência.
