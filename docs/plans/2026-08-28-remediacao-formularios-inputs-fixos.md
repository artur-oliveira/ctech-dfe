# Remediação dos formulários de emissão — opções fixas, validação granular e visão simples/avançada

Origem: critique + audit Impeccable de 2026-08-28 sobre os sete formulários de criação/atualização tocados pelo plano
`2026-08-27-cobertura-total-tags-implementacao.md` (Tasks 1–49, todas concluídas). Snapshot em
`.impeccable/critique/2026-08-28T23-32-59Z__ui-src-components-tax-taxfieldseditor-tsx.md`.

Scores de partida: **Design 24/40**, **Audit 7/20**.

## Régua (vale para toda tarefa abaixo)

1. **Domínio fechado nunca é texto livre.** Tabela SEFAZ, enum, UF, município, país, unidade, CFOP, NCM, CEST, ANP,
   ONU, CST, cClassTrib, data — tudo é Select/Combobox/Radio/Checkbox/Datepicker. Regra de controle: **até ~12 opções,
   `OptionsSelect`; acima disso, `Combobox`** (buscável).
2. **A regra que hoje é texto de ajuda vira `superRefine`.** Se o formulário sabe explicar a dependência em português ao
   lado do campo, o schema tem que recusá-la.
3. **Valor derivado não é input.** Já é a prática em vCredPresumido, ICMS efetivo, totais de cana, parcelas — estender.
4. **Toda tela tem visão simples que resolve o caso comum e avançada que cobre o resto.** O simples é o default.
5. Nada de regressão de cobertura de tag: o XML emitido antes e depois tem que ser idêntico para os mesmos dados.

## Fase 0 — já executada nesta sessão

- [x] `useIcmsAliqPreview.ts` — objeto literal entrava num `useDebounce` que compara por identidade: loop de fetch a
  cada 300 ms enquanto o formulário estivesse montado. `useMemo` sobre as três primitivas + teste de regressão em
  `ui/src/__tests__/lib/useIcmsAliqPreview.test.tsx` (reprova sem o fix).
- [x] `combobox.tsx` — índice do Fuse memoizado por lista (era reconstruído a cada tecla sobre NCM/municípios/CFOP),
  listener de scroll com dep `[open]`, `aria-activedescendant` + `id` por opção, `aria-label`/`aria-controls` na busca.

## Fase 1 — P0 de emissão (bloquear o que hoje só avisa)

- [x] **Task 1: soma de pagamentos.** `canGoNext('pagamento')` exige `|vNF − Σ vPag| < 0,01`. NFC-e: excedente vira
  `vTroco` explícito; falta continua bloqueando. Botão "ajustar última parcela" absorve o centavo de arredondamento.
- [x] **Task 2: duplicatas.** Mesmo bloqueio entre `vFat` e `Σ vDup` antes de chegar em `revisao`; `d_venc` ganha `min`
  = data de emissão.
- [x] **Task 3: veículo e arma.** `canGoNext('produtos')` exige, por item: `prod_type === 'veiculo'` → chassi (17),
  nSerie e nMotor preenchidos; `prod_type === 'arma'` → ao menos uma entrada em `armas[]`. Item que falha é destacado
  como já faz `cfopMissingVariant`.
- [x] **Task 4: `dh_sai_ent`** limitado à janela do leiaute em relação à emissão.

## Fase 2 — validação granular nos schemas

- [x] **Task 5: `products.ts` `superRefine`** — comb (cProdANP/descANP/UFCons), veículo (13 campos), arma, med ISENTO →
  motivo, indEscala N → CNPJ fab, selo IPI (código+qtd juntos), peri (ONU → nome/classe/grupo), `Σ p_orig = 100`,
  `gross_weight ≥ net_weight`, dígito verificador do GTIN. **CEST × NCM fica para a Fase 5** — depende da
  tabela de vínculo CEST/NCM, que ainda não existe no repositório.
- [x] **Task 6: `entity.ts`** — `freight_retention` all-or-nothing com formato por campo; CNAE conferido contra
  `ALL_CNAES`; `bank_code` contra a tabela BACEN (Task 12).
- [x] **Task 7: `operations.ts`** — `cfop_suffix` conferido contra a tabela CFOP; `compra_gov_tp_oper ∈ {2,3}` exige as
  chaves de referência já no save da operação, não só na emissão.
- [x] **Task 8: TaxFieldsEditor** — IPI (pIPI xor vUnid quando CST tributado), ICMSPart (pBCOp + UFST juntos),
  modBC ∈ {1,2} → pauta, ALC/ZFM (tpCBS + nProcSUFRAMA juntos), obsItem (xCampo + xTexto juntos).

## Fase 3 — acessibilidade e DRY na raiz

- [x] **Task 9: `<TaxField>`** — um wrapper `label htmlFor` + controle `id`, com `useId()` como prefixo, substituindo os
  ~90 blocos copiados do TaxFieldsEditor (remove ~600 linhas e conserta os 13 ids literais que colidem quando o
  componente monta duas vezes na mesma tela).
- [x] **Task 10: `form.tsx`** — `aria-invalid` + `aria-describedby` ligados ao `FormMessage` que já existe e nunca é
  consumido; badge de erro nas abas/seções colapsadas (`ProductForm`, `EntityForm`, `OperationForm`).

## Fase 4 — trocas de controle com tabela já existente (custo zero)

- [x] **Task 11:** `ProductForm:1451` UF consumo, `NfeEmitForm:1892` UF veículo → `UF_OPTIONS`;
  `TaxFieldsEditor:752,799` e `EntityForm` `c_mun_fg` → `CITY_OPTIONS`; `NfeEmitForm:511` CFOP e `EntityForm` CFOP de
  retenção → `getAllCfopOptions()`; `OperationForm:358` sufixo → derivado de `groupCfopConfigBySuffix`;
  `EntityForm:426` CNAE → `ALL_CNAES`; `TaxFieldsEditor:637` unidade IS → `UNIT_OPTIONS`;
  `AccessKeyPicker:73` → `Combobox`; `EntityForm:554` valores → `CurrencyInput`/`NumericInput`.

## Fase 5 — tabelas de domínio novas

Cada uma: fonte oficial citada no cabeçalho do arquivo, teste de integridade (sem duplicata, formato do código,
contagem), e a troca do controle correspondente no mesmo commit.

- [x] **Task 12:** BACEN país — 249 códigos vigentes, do .ods oficial da NT 2018.003 v1.01. Endereço estrangeiro
  (`c_pais` em `address-fields`) fica pendente: o campo não existe no schema hoje.
- [~] **Task 13:** `cClassTribIS` — **bloqueada na fonte**. A NT 2025.002 v1.51, p. 95, diz literalmente
  "ANEXO II - CÓDIGO DE CLASSIFICAÇÃO TRIBUTÁRIA DO IMPOSTO SELETIVO (cClassTribIS) — Tabela a ser publicada".
  Não existe tabela para picker; o campo segue texto validado por `\d{6}`. Reavaliar quando a RFB publicar.
- [x] **Task 14:** cEnq IPI — 132 códigos da NT 2020.002 v1.01, com a faixa filtrada pelo CST (RV W16-10).
  O código do selo não tem tabela pública: segue texto.
- [x] **Task 15:** ANP — 1.031 códigos do arquivo oficial da SEFAZ, mais os 40 monofásicos com descrição, unidade
  tributável, pBio e ad rem do ICMS, que o formulário preenche sozinho ao escolher o combustível.
- [x] **Task 16:** classe de risco (25 entradas, Res. ANTT 5.998/2022) e grupo de embalagem I/II/III, com as
  classes que não recebem grupo desabilitando o campo. **A tabela ONU (~3.500 números) fica pendente** — não
  cabe numa pesquisa web confiável; precisa do anexo da própria resolução.
- [x] **Task 17:** cBenef — 1.891 códigos de RS, SP, RJ, PR e ES, filtrados pela UF do emitente e, no RS (que
  publica o par código × CST), também pelo CST da linha. UF sem tabela mantém o campo livre.
- [ ] **Task 18:** banco BACEN. **Pendente:** não está entre as tabelas do portal da SEFAZ (é do BACEN), e conferi
  `ctech-go-common`, `ctech-billing`, `ctech-wallet` e `ctech-ui` — nenhum tem. Por ser reutilizável, o lugar certo
  é um pacote compartilhado, não `ctech-dfe/ui`.

## Extra: pares de tipo × espécie de veículo

- [x] `vehicle_type_pairs.ts` — a SEFAZ publica quais espécies existem para cada tipo, e os dois campos eram selects
  independentes: "caminhão de passageiro" passava no cadastro e só era recusado na emissão.
- [x] **Task 19:** strings mágicas viram controle: `SEM GTIN` (checkbox), `ISENTO` ANVISA (radio + campo desabilitado).

## Fase 6 — visão simples e avançada

- [x] **Task 20: TaxFieldsEditor** — simples = CFOP + CST/CSOSN + PIS + COFINS e os condicionais que eles disparam
  (já é o que `:161-224` faz). Os 11 toggles de grupo vão para uma seção "Outros impostos" com badge de quantos estão
  ativos, que `deriveTaxGroups` já calcula.
- [x] **Task 21: ProductForm** — cadastro rápido (código, descrição, NCM, origem, unidade, preço, CFOP, perfil fiscal) e
  "mais campos" por aba; com perfil fiscal escolhido, o TaxFieldsEditor colapsa para um resumo com link "sobrescrever";
  blocos Importação/Reforma/Selo/Perigoso gateados por `prod_type` ou por um checklist explícito.
- [x] **Task 22: NfeEmitForm** — a gaveta única vira três: Transporte / Documentos e datas / Grupos setoriais; grupo que
  se torna obrigatório sobe para fora da gaveta, como `requiresNfRefs` já faz.
- [x] **Task 23: OperationForm** — renomear "Mensagens fiscais" e separar retenções, reforma e grupos setoriais.

## Fase 7 — dívida de design system

- [x] **Task 24:** 33 eyebrows `uppercase tracking-wider` → `SectionCard` ou `text-sm font-medium text-gray-600`.
- [x] **Task 25:** um vocabulário de label; `red-*`/`amber-*` crus → `text-danger`/`text-warning`; datas cruas →
  `Input type="date"`; alvos de 14 px → `min-h-11`; grids sem breakpoint.
- [x] **Task 26:** `GlossaryTerm` nos termos densos de ProductForm e TaxFieldsEditor; `caption` no front-matter do
  DESIGN.md para o detector parar de acusar `text-[0.8rem]`.

## Critério de pronto

- [ ] `cd api && go test ./...`, `cd ui && npx eslint src --ext .ts,.tsx` (zero warnings), `npx vitest run`,
  `cd cdk && npx tsc --noEmit` verdes.
- [ ] Nenhuma tag some do XML: golden `TestBuildEnviNFeGolden` inalterado.
- [ ] Toda regra nova de schema tem teste que reprova o payload inválido.
- [ ] Toda tabela nova tem fonte oficial citada e teste de integridade.
- [ ] `DOCS.md` atualizado nos campos e controles que mudaram.


## Conferência contra as tabelas do portal da SEFAZ (2026-08-28)

O usuário baixou as tabelas do portal; conferi cada uma contra o que o código já tinha.

**Divergências corrigidas:**

- `cClassTrib` do IBS/CBS: o código tinha 63 das 164 classificações publicadas.
- `tPag`: 12 e 13 (Vale Presente e Vale Combustível) eram tratados como PIX; faltavam 23, 24 e 91.
- `tBand`: 8 das 28 bandeiras publicadas.
- `cListServ`: derivava da tabela da NFS-e, que traz um 99.01 que o campo não aceita.

**Conferidas e corretas, sem mudança:** NCM (10.515, cobertura exata), unidades de medida (61),
`cCredPres` (13 — os rótulos são resumos deliberados), CST do IBS/CBS (18).

**Tabelas novas geradas:** ANP (1.031 + 40 monofásicos com uTrib/pBio/adRem), cBenef (1.891 em
RS/SP/RJ/PR/ES), pares tpVeic × espVeic (39), unidade tributável por NCM (10.515).

### Pendente conhecido

- [x] **59 CFOPs vigentes ausentes de `cfop.ts`** — resolvidos. As descrições vieram do texto do
  Convênio SINIEF s/nº, Anexo II, na redação do Ajuste SINIEF 03/24 (vigente desde 01.06.24), baixado
  do CONFAZ. São de ato cooperativo, Sistema de Integração e Parceria Rural, uso de bordo em tráfego
  internacional e formação de lote de exportação. **Atenção à fonte:** a página `cfop_cvsn_70_vigente`
  do CONFAZ, que os buscadores devolvem primeiro, é o anexo **revogado** — a vigente é
  `cfop_cvsn_1-6.24`, e as redações diferem de forma sutil.
- [ ] **Padrões de regime especial por UF** (`Tabela de Padrões de Regime Especial.xlsx`): traz a
  expressão regular de `nProc` por UF; hoje o campo é livre. Só PR e SP publicam.
- [ ] **Prefixo GS1**: útil para a mensagem de erro do GTIN dizer *qual* país/entidade emitiu o
  código, não só que o dígito está errado.
