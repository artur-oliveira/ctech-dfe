# Cadastros reutilizáveis — Plano de implementação (Fase 1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this
> plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Design spec:** `docs/specs/2026-08-08-cadastros-reutilizaveis-emissao.md`

**Goal:** Transformar em entidades nomeadas e reutilizáveis as quatro decisões que hoje são redigitadas a cada emissão —
tributação por produto, natureza da operação, papel da pessoa e composição veicular/condição de pagamento — sem migrar
dado nenhum e sem quebrar contrato de API.

**Escopo deste plano:** fases 1a–1d da spec. Fases 2–4 (modo avançado, `MergeOverrides`/inline, `/preview`,
prontidão de cadastro) estão especificadas na spec e ganham plano próprio.

**Tech stack:** Go (Fiber v3, aws-sdk-go-v2), Next.js 16 / TypeScript (zod, react-hook-form, TanStack Query), AWS CDK
TypeScript. `worker/`, `py-dfe/` e `go-dfe/` **não são tocados**. Uma dependência fora do monorepo: `ctech-go-common`
(Task 0), aditiva, tem que ser publicada antes da Task 2.

## Global Constraints

- **Sem migração de dados.** `cfop_config[]` continua válido e continua vencendo sobre o perfil. Pessoa sem `roles`
  continua funcionando.
- **Sem renomear campo existente.** Só adição de campos opcionais.
- **Sem mudança de contrato quebrando cliente.** Todo body válido hoje continua válido.
- `api`: erros só via `problem.*` (nunca erro cru ou `fiber.Map`). Acesso ao DynamoDB só por `Query`, nunca `Scan`.
  Toda chave/código/URL/nome de header é constante nomeada — nada de literal espalhado.
- `ui`: `npx eslint src --ext .ts,.tsx` com **zero erros e zero warnings** antes de dar qualquer task por concluída.
- Testes conforme a tabela do `CLAUDE.md`: schema → unit + contract; lógica de serviço → unit; integração AWS →
  integration; emissão fiscal → unit + integration.
- Reuso obrigatório antes de escrever qualquer coisa nova: `NewCRUDRepository` (`api/internal/repositories/base.go`),
  `CRUDMutationHelper` (`api/internal/services/crud.go`), `mountCRUD` (`api/internal/api/v1/crud_handlers.go`),
  `EntityForm` (`ui/src/components/EntityForm.tsx`).

---

## Fase 1a — Papéis de pessoa

> Uma pessoa tem **vários papéis ao mesmo tempo** (transportadora que também é cliente). `roles` é lista no item da
> pessoa; não há entidade, tabela nem índice derivado. Ver spec §3.6, incluindo as três alternativas recusadas.

### Task 0: `api-commons` — filtro `contains`

**Repo:** `~/Documents/Projects/Ctech/ctech-go-common` (`gopkg.aoctech.app/api-commons`, em uso na v1.4.1)
**Files:** `dynamo/base.go`

- [ ] `QueryOpts`: par novo `FilterContainsField` / `FilterContainsValue`, gerando
  `contains(#f, :v)` na `FilterExpression`. Seguir o idioma de pares tipados do struct — **não** expor passagem de
  expressão crua.
- [ ] Compor com o `FilterField`/`FilterValue` existente via `AND` quando ambos vierem preenchidos.
- [ ] Puramente aditivo: `QueryOpts` sem os campos novos gera exatamente o mesmo input de hoje.
- [ ] Publicar a versão e subir `api/go.mod` do ctech-dfe.

**Verify:** unit no ctech-go-common cobrindo só-contains, só-equality, ambos e nenhum; documentar o par em
`AGENTS.md`/README do pacote.

**Por que aqui e não localmente:** filtrar em memória depois da query quebra a paginação — uma página de 50 pessoas pode
devolver 3 motoristas e um cursor que mente. Regra do `CLAUDE.md` global: estender o pacote compartilhado.

### Task 1: Campo `roles`

**Files:** criar `api/internal/services/person_roles.go`; modificar `api/internal/api/v1/dto.go`,
`ui/src/lib/schemas/persons.ts`, `ui/src/lib/types/api.ts`

- [ ] Constantes em `person_roles.go`: `RoleCustomer`, `RoleSupplier`, `RoleCarrier`, `RoleDriver`, `RoleProvider`, e
  `AllPersonRoles` para alimentar validação e UI a partir de uma fonte só.
- [ ] `PersonObjectBody.Roles []string` com
  `validate:"omitempty,dive,oneof=customer supplier carrier driver provider"`.
- [ ] Nada mais. Sem repositório novo, sem tabela, sem GSI, sem linha derivada — `roles` é atributo do item da pessoa e
  o item é a única fonte da verdade.

**Verify:** unit do DTO (papel inválido rejeitado, lista vazia e ausente aceitas); unit zod espelhando os mesmos casos.

### Task 2: Listagem por papel

**Files:** `api/internal/repositories/persons.go`, `api/internal/services/persons.go`,
`api/internal/api/v1/persons.go`

- [ ] `PersonListOpts`: campos `Role string` e `Q string`.
- [ ] `PersonRepository.List` ganha três caminhos, e **o caminho sem `Role` fica byte a byte igual ao de hoje** — é o
  que garante que nada existente regride:
  | `Role` | `Q` | Consulta |
  |---|---|---|
  | vazio | qualquer | exatamente o código atual |
  | preenchido | alfabético ou vazio | `org-name-index`, `begins_with(name, Q)` + `FilterContains(roles, Role)` |
  | preenchido | só dígitos | tabela base, `begins_with(sk, Q)` + `FilterContains(roles, Role)` — a SK **já é** o
  documento (`CNPJ_…`/`CPF_…`), então busca por documento também não precisa de índice |
- [ ] **Paginação com filtro:** o `Limit` do DynamoDB conta itens **lidos**, não retornados — uma query filtrada devolve
  menos que `Limit` junto de um `LastEvaluatedKey`. O serviço itera até completar a página pedida ou esgotar o
  cursor, **com teto de 5 idas ao DynamoDB por requisição**. Sem esse teto, um papel raro numa organização grande
  faz o loop varrer a partição inteira para devolver uma página vazia.
- [ ] **Regra que torna o teto seguro:** fim da lista é `LastEvaluatedKey` ausente, **nunca** contagem de itens abaixo
  do `Limit`. Vale para a rota e para a UI. Com isso, estourar o teto e devolver página curta com cursor é
  degradação de latência, não bug de correção.
- [ ] Rota: `GET /v1.0/persons?role=&q=`, cursor e paginação pelo mesmo `sendPage` de hoje.

**Verify:** integration com **pessoa multi-papel** (`roles: ["customer","carrier"]`) afirmando que ela aparece em
`?role=customer` **e** em `?role=carrier`, e **uma única vez** na listagem sem filtro; integration de paginação numa
organização onde a maioria das pessoas não tem o papel buscado, afirmando que a página vem cheia.

### Task 3: UI — papéis e seletores por papel

**Files:** `ui/src/lib/schemas/persons.ts`, `ui/src/lib/types/api.ts`, `ui/src/lib/api/client.ts`,
`ui/src/components/persons/*`, `ui/src/components/mdfe/MdfeEmitForm.tsx`, `ui/src/components/nfe/NfeEmitForm.tsx`

- [ ] Seletor de papéis no formulário de pessoa — checkboxes **multi-seleção** (não radio, não select único),
  `customer` marcado por padrão, opções vindas de uma constante espelhando `AllPersonRoles`.
- [ ] `client.ts`: `role` e `q` em `listPersons`.
- [ ] Extrair um `PersonPicker` a partir do `NfsePersonSearch.tsx` existente (226 linhas, já é a busca de pessoa
  debounced) parametrizado por `role` — **não escrever um segundo componente de busca de pessoa.**
- [ ] `PersonPicker` exige **2 caracteres** antes de disparar a busca (é a premissa de custo da spec §3.6, não uma
  preferência de UX). Abaixo disso, estado vazio instrutivo em vez de lista.
- [ ] MDF-e: substituir os inputs manuais de condutor (`MdfeEmitForm.tsx:452-458`) por `PersonPicker role="driver"`,
  mantendo "adicionar condutor não cadastrado" como caminho secundário.
- [ ] NF-e: seletor de transportadora em `transporta_pk` usando `PersonPicker role="carrier"`.
- [ ] Filtro por papel na listagem de pessoas, e os papéis exibidos como badges na linha — uma pessoa multi-papel tem
  que ser visivelmente multi-papel, senão o usuário acha que o filtro está errado.

**Verify:** unit zod dos papéis; eslint zero.

---

## Fase 1b — Perfis fiscais

### Task 4: Tabela e CRUD de perfis

**Files:** `cdk/lib/dynamodb-stack.ts`, `cdk/lib/iam-stack.ts`; criar
`api/internal/repositories/tax_profiles.go`, `api/internal/services/tax_profiles.go`,
`api/internal/api/v1/tax_profiles.go`; modificar `api/internal/api/v1/dto.go`, `router.go`,
`api/internal/repositories/roles.go`

- [ ] CDK: tabela `organization_tax_profiles` (`pk`/`sk`) + GSI `name-index` (`pk`, `name`), copiando exatamente o bloco
  de `organization_services` (`dynamodb-stack.ts:486-512`). Adicionar ao `this.tables`.
- [ ] IAM: incluir a tabela nas policies de api e worker.
- [ ] `TaxProfileBody`: `name`, `description`, `cfops []string` (min 1, `dive,cfop`) e todos os campos de
  `CfopConfigBody` **exceto `cfop`**. Extrair esses campos para um struct embutido `taxFieldsBase`, embutido tanto
  em `CfopConfigBody` quanto em `TaxProfileBody` — duas cópias de 60 campos é exatamente a duplicação que a spec
  existe para eliminar.
- [ ] Repositório, serviço e rotas via `NewCRUDRepository` + `CRUDMutationHelper` + `mountCRUD`.
- [ ] `AuditResourceTaxProfile` e permissões RBAC `list/get/create/update/delete.tax_profiles` semeadas no boot.

**Verify:** unit do DTO; integration do CRUD; `cdk synth` limpo.

### Task 5: Resolução perfil → produto

**Files:** `api/internal/api/v1/dto.go`, `api/internal/services/nfes/builders_tax.go`,
`api/internal/services/nfes/emit.go`

- [ ] `ProductBody.TaxProfiles []ProductTaxProfileRef` — `{ tax_profile_id string, overrides *taxFieldsBase }`.
- [ ] Nova função `resolveCfopTax(product, profiles, cfop) (map[string]any, error)` implementando a ordem da spec §3.2:
  `cfop_config[cfop]` → `overrides` do produto → perfil → overrides de nível de produto → tabela da UF. **Uma
  função só**, consumida por NF-e e NFC-e.
- [ ] `resolveProducts` (`nfes/emit.go:532`): carregar os perfis referenciados (batch, um `BatchGetItem` por emissão —
  nunca um `Get` por item dentro do laço) e validar o CFOP contra a **união** de `cfop_config[]` e dos `cfops` dos
  perfis (hoje só `cfop_config`, `emit.go:554-568`).
- [ ] Mensagem de erro de CFOP inválido passa a dizer onde configurar (produto ou perfil).

**Verify:** unit de `resolveCfopTax` cobrindo (a) produto legado sem perfil → resultado idêntico ao de hoje,
(b) perfil puro, (c) override do produto vencendo o perfil, (d) `cfop_config` vencendo tudo. Integration de emissão
NF-e com produto por perfil.

### Task 6: UI — perfis fiscais

**Files:** criar `ui/src/app/tax-profiles/{page,new,edit}`, `ui/src/components/tax-profiles/TaxProfileForm.tsx`;
modificar `ui/src/components/products/ProductForm.tsx`, `ui/src/app/products/page.tsx`, `ui/src/lib/schemas/*`,
`ui/src/lib/api/client.ts`, `ui/src/lib/types/api.ts`

- [ ] `TaxProfileForm` reaproveitando os blocos de campos fiscais já existentes na aba "Tributação" de `ProductForm`
  (`ProductForm.tsx:1078+`) — extrair os subcomponentes, **não recriá-los**.
- [ ] `ProductForm`: seletor de perfis + "sobrescrever para este produto" (revela os campos do perfil, pré-preenchidos).
  A aba "Tributação" atual continua funcionando e **não é removida nesta fase**.
- [ ] Listagem de produtos: coluna de perfil + ação em massa "aplicar perfil aos selecionados".
- [ ] Item no menu lateral.

**Verify:** unit zod; eslint zero.

---

## Fase 1c — Naturezas de operação

### Task 7: `ResolveCFOPScope` em Go

**Files:** criar `api/internal/services/cfop.go` + teste; referência: `ui/src/lib/data/cfop.ts`

- [ ] `ResolveCFOPScope(suffix, emitUF, destUF string) (string, error)` — `5` intra-UF, `6` inter-UF, `7` exterior.
  Sem variante para o escopo exigido → `problem.BadRequest` acionável (decisão já firmada em
  `docs/specs/2026-06-23-cfop-suffix-grouping-and-null-omission-design.md`; **nada de sintetizar CFOP**).
- [ ] Tabela de casos compartilhada entre o teste Go e o teste TypeScript existente, garantindo paridade. Go é a fonte
  da verdade; o TypeScript passa a ser só agrupamento de exibição.

**Verify:** unit Go; unit TypeScript sobre a mesma tabela de casos.

### Task 8: Tabela e CRUD de operações

**Files:** `cdk/lib/dynamodb-stack.ts`, `cdk/lib/iam-stack.ts`; criar
`api/internal/repositories/operations.go`, `api/internal/services/operations.go`,
`api/internal/api/v1/operations.go`; modificar `dto.go`, `router.go`, `repositories/roles.go`

- [ ] CDK: `organization_operations` + GSI `name-index` + IAM.
- [ ] `OperationBody` conforme a spec §3.3, incluindo `is_default`.
- [ ] Validação de placeholders (spec §3.7): mapa fechado de chaves; chave desconhecida em `inf_ad_fisco`/`inf_cpl` é
  erro **no cadastro**, nunca interpolação vazia no XML.
- [ ] Regra de operação padrão única por organização, aplicada no `TransactWrite` (marcar uma desmarca a anterior).
- [ ] CRUD + auditoria + RBAC pelo mesmo caminho da Task 4.

**Verify:** unit do DTO e dos placeholders; integration cobrindo a exclusividade de `is_default`.

### Task 9: Emissão consumindo a operação

**Files:** `api/internal/services/nfes/emit.go`, `nfce_emit.go`, `api/internal/services/interpolate.go` (novo)

- [ ] `NfeEmitBody.OperationID *string` e `NfceEmitBody.OperationID *string`.
- [ ] Quando presente, a operação preenche `nat_op`, `tp_nf`, `fin_nfe`, `ind_final`, `ind_pres`, `mod_frete` e o CFOP
  de cada item sem CFOP explícito (via `ResolveCFOPScope` com a UF do destinatário). **Valor explícito no request
  sempre vence** — é a escada da spec §4.
- [ ] `Interpolate(template string, vars map[string]string) (string, error)` compartilhada, aplicada a
  `inf_ad_fisco`/`inf_cpl`.
- [ ] Item sem CFOP e sem operação: mesma mensagem de erro de hoje, inalterada.

**Verify:** unit da precedência request > operação; integration de emissão NF-e só com `operation_id` + itens.

### Task 10: UI — naturezas de operação

**Files:** criar `ui/src/app/operations/*`, `ui/src/components/operations/OperationForm.tsx`; modificar
`ui/src/components/nfe/NfeEmitForm.tsx`, `ui/src/components/nfce/NfceEmitForm.tsx`

- [ ] CRUD de operações + item no menu.
- [ ] NF-e passo 1: seletor de operação com a padrão pré-selecionada. Campos resolvidos por ela saem do caminho feliz e
  passam a aparecer só no modo avançado (o toggle global é da fase 2; até lá, dentro do `CollapsibleSection`
  existente em `NfeEmitForm.tsx:1569`).
- [ ] Passo 2: quando a operação define o CFOP, exibi-lo como **texto resolvido**, não como dropdown.
- [ ] NFC-e: operação padrão implícita, **sem nenhum passo ou campo novo na tela** (regra do `DESIGN.md` para venda de
  balcão).

**Verify:** unit zod; eslint zero.

---

## Fase 1d — Condições de pagamento e composições

### Task 11: Condições de pagamento

**Files:** `cdk/lib/dynamodb-stack.ts`, `cdk/lib/iam-stack.ts`; criar
`api/internal/repositories/payment_terms.go`, `api/internal/services/payment_terms.go`,
`api/internal/services/payment_expand.go`, `api/internal/api/v1/payment_terms.go`; modificar `dto.go`, `router.go`,
`nfes/emit.go`

- [ ] CDK + IAM + CRUD conforme a Task 4.
- [ ] 
  `ExpandPaymentTerm(term, total decimal.Decimal, issueDate time.Time) ([]NfePaymentItem, *NfeFatItem, []NfeDuplicataItem)`
  — função pura. **A última parcela absorve o resíduo de arredondamento**; a soma das duplicatas tem que fechar com
  `vNF` centavo a centavo.
- [ ] `NfeEmitBody.PaymentTermID *string`; `payments`/`cobr_*` explícitos no request vencem a expansão.
- [ ] UI: CRUD + pré-visualização das parcelas geradas; seletor no passo de revisão da NF-e.

**Verify:** unit de `ExpandPaymentTerm` com totais que não dividem exato (R$ 100,00 em 3×, R$ 0,01 em 2×);
integration de emissão com `payment_term_id`.

### Task 12: Composições veiculares

**Files:** `cdk/lib/dynamodb-stack.ts`, `cdk/lib/iam-stack.ts`; criar
`api/internal/repositories/vehicle_sets.go`, `api/internal/services/vehicle_sets.go`,
`api/internal/api/v1/vehicle_sets.go`; modificar `api/internal/services/mdfes/emit.go`,
`ui/src/components/mdfe/MdfeEmitForm.tsx`

- [ ] CDK + IAM + CRUD conforme a Task 4.
- [ ] `VehicleSetBody`: `name`, `tractor_sk`, `trailer_sks` (máx 3), `driver_docs`, `rntrc`, `ciot`. Validar no serviço
  que `tractor_sk` tem `role=tractor` e cada `trailer_sk` tem `role=trailer`.
- [ ] `MdfeEmitBody.VehicleSetID *string` expandindo para `vehicle`/`trailers`/`drivers`/`rntrc`/`ciot`, **cada um
  ainda sobrescrevível no mesmo request**.
- [ ] O gating de `services.Missing` continua valendo; a mensagem passa a nomear qual membro do conjunto está
  incompleto.
- [ ] UI: CRUD (reusando os seletores de veículo e o `PersonPicker role="driver"` da Task 3) + select de conjunto no
  passo de veículo do MDF-e, com os campos expandidos visíveis e editáveis.

**Verify:** unit da expansão e da precedência; integration de emissão MDF-e por `vehicle_set_id`, incluindo um conjunto
com veículo incompleto (tem que bloquear).

### Task 13: `vehicle.owner` deixa de ser dado morto

**Files:** `api/internal/services/mdfes/emit.go`, `api/internal/api/v1/dto.go`

- [ ] `resolveOwner` passa a usar `VehicleOwnerBody` do cadastro como default de `veicTracao/prop` quando
  `MdfeOwner` não vier no request. Remover o comentário "not used for MDF-e prop building" do `dto.go`.
- [ ] `MdfeOwner` explícito continua vencendo.
- [ ] A derivação de `ide/tpTransp` a partir da presença do proprietário (regras SEFAZ F18/F19/F25) tem que continuar
  idêntica — **este é o ponto de maior risco de regressão da fase 1d.**

**Verify:** unit cobrindo veículo próprio, veículo de terceiro só pelo cadastro, e override no request; integration de
emissão MDF-e afirmando `tpTransp` correto nos três casos.

---

## Task 14: Documentação

**Files:** `DOCS.md`, `DynamoDB-Tables.md`, `OVERVIEW.md`, `CONDUCT.md`

- [ ] `DOCS.md`: endpoints e schemas das 4 entidades novas; `?role=`/`?q=` em persons, com a nota de que papel é filtro
  de cadastro e **não** é validado na emissão; a ordem de resolução de tributação e de operação.
- [ ] `DynamoDB-Tables.md`: as 4 tabelas novas + o campo `roles` (lista) em `organization_persons`.
- [ ] `OVERVIEW.md`: mapa de tabelas e contagem (30 → 34).
- [ ] `CONDUCT.md`: registrar as três invariantes — "a ordem de resolução é única e vive em `MergeOverrides`",
  "`ResolveCFOPScope` é fonte da verdade em Go; o TypeScript é só exibição", e "query com `FilterExpression` pagina
  até encher a página — `Limit` conta itens lidos, não retornados".
- [ ] `ctech-go-common`: documentar `FilterContainsField`/`FilterContainsValue` no `AGENTS.md`/README do pacote.

---

## Revisão de impacto entre projetos

| Projeto              | Impacto                                                                                                                                    |
|----------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `ctech-go-common`    | `QueryOpts.FilterContains*` — aditivo, versão nova, Task 0 (única dependência fora do monorepo)                                            |
| `api/`               | 4 tabelas, 4 repositórios/serviços/rotas, `persons` (campo `roles` + listagem filtrada), resolução de tributação, emissão NF-e/NFC-e/MDF-e |
| `cdk/`               | 4 tabelas + GSIs em `dynamodb-stack.ts`, policies em `iam-stack.ts`                                                                        |
| `ui/`                | 4 cadastros novos, `PersonPicker` extraído, NF-e/NFC-e/MDF-e consumindo as entidades, `ProductForm` aditivo                                |
| `worker/`            | **nenhum** — consome o documento já resolvido                                                                                              |
| `py-dfe/`, `go-dfe/` | **nenhum** — o contrato de request/response não muda                                                                                       |

## Commit sugerido

`feat: cadastros reutilizáveis (perfis fiscais, naturezas de operação, papéis de pessoa, composições)`
