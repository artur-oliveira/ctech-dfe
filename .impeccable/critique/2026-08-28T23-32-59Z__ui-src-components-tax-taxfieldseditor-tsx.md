---
target: formulários de criação/atualização NF-e
total_score: 24
max_score: 40
na_heuristics: 
p0_count: 4
p1_count: 3
timestamp: 2026-08-28T23-32-59Z
slug: ui-src-components-tax-taxfieldseditor-tsx
---
Method: dual-agent (A: design review · B: detector + technical audit). No browser available — all findings verified in source.

Scope: the seven NF-e create/update forms — ProductForm, TaxFieldsEditor, NfeEmitForm, OperationForm, NicheGroupsFields, AccessKeyPicker, EntityForm.

## Design Health Score — 24/40

| # | Heuristic | Score | Key issue |
|---|---|---|---|
| 1 | System status | 3 | StepIndicator shows position, not per-step validity |
| 2 | Match real world | 3 | Labels leak XML tags (cIndOp); placeholders nSerie/nCano/nMotor |
| 3 | Control and freedom | 2 | Unchecking a tax group wipes its data, no undo; CST change wipes 8 fields |
| 4 | Consistency | 2 | Three label idioms; 33 DESIGN.md-banned uppercase eyebrows |
| 5 | Error prevention | 2 | 34 free-text inputs over closed domains; zero superRefine in products.ts |
| 6 | Recognition over recall | 2 | Operator must recall cEnq 999, ONU, cClassTribIS, IBGE 7-digit, BACEN 1058, CFOP suffix |
| 7 | Flexibility | 3 | Recent-recipient chips, profiles, drafts; no "emitir semelhante" |
| 8 | Minimalist design | 2 | Tipo Especial tab renders Importação/Reforma/Selo/Perigoso for a generic product |
| 9 | Error recovery | 3 | CFOP-mix error exemplary; errors inside collapsed tabs invisible |
| 10 | Help | 2 | GlossaryTerm used 5x repo-wide, 0x where jargon is densest |

## Audit Health Score — 7/20

| # | Dimension | Score | Basis |
|---|---|---|---|
| 1 | Accessibility | 1 | 90 of 103 labels without htmlFor; no aria-describedby/aria-invalid anywhere |
| 2 | Performance | 1 | useIcmsAliqPreview infinite 300ms fetch loop; CFOP arrays rebuilt per render |
| 3 | Theming | 2 | 33 banned eyebrows; ~21 raw red-*/amber-* where tokens exist |
| 4 | Responsive | 2 | 4 unbreakpointed grids; 22 group toggles at 14px |
| 5 | Implementation integrity | 1 | ~90 copy-pasted field blocks; 3 label vocabularies; inline CST arrays |

Detector: exit 2, 2 advisory findings (ProductForm.tsx:1270,1363, design-system-font-size) — both false positives; text-[0.8rem] is the documented Caption step missing from the DESIGN.md front-matter.

## Priority issues

- P0 Payment/duplicata sum is advisory, not blocking (NfeEmitForm.tsx:1526-1531, 1646-1656, canGoNext 1083-1088). Highest-frequency avoidable NF-e rejection, and it is arithmetic.
- P0 Vehicle/weapon per-unit data can reach "Emitir" empty (NfeEmitForm.tsx:532-544, 673-690; canGoNext:1082 checks neither).
- P0 Infinite fetch loop, useIcmsAliqPreview.ts:18 — object literal into identity-compared useDebounce. FIXED this session, with regression test.
- P0 90 controls without accessible name + 13 literal DOM ids colliding across two TaxFieldsEditor instances.
- P1 34 closed domains rendered as free text; 8 have the option table already in the repo (UF x2, CITY x3, CFOP x2, CNAE).
- P1 products.ts carries no cross-field rule at all — ~20 documented conditional dependencies live only as help text.
- P1 Combobox: Fuse index rebuilt per keystroke, scroll listener re-registered per render, no aria-activedescendant. FIXED this session.
- P2 33 banned eyebrows, three label vocabularies, hand-rolled date inputs bypassing the shared Input.
- P2 OptionsSelect (unsearchable) carrying 19-50 item lists; rule should be: over ~12 options use Combobox.

## Simple vs advanced

Reference implementations: EntityForm (role-gated disclosure), NicheGroupsFields (XSD choice as radio). Needs work: ProductForm (tabs split by subject, not frequency), TaxFieldsEditor (11 group toggles at once), NfeEmitForm (eleven unrelated concerns behind one disclosure), OperationForm (mislabelled container only).

## Root pattern

Specific and excellent where someone thought hard (AccessKeyPicker refuses 44 typed digits; resolveCfopScope removes the 5/6/7 decision; derived values never become inputs). Generic where the tag list was being closed out for coverage — the Bloco 8 reform fields are exactly the ones that became raw Inputs. Accessibility correlates with state management, not care: the three RHF forms got it for free, the two hand-rolled ones got none.
