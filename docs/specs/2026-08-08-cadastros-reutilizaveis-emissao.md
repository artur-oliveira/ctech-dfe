# Design — Cadastros reutilizáveis e resolução de dados na emissão

**Date:** 2026-08-08
**Scope:** `api/` (repositories, services, routes, dto), `cdk/` (tabelas + IAM), `ui/` (cadastros e formulários de
emissão), e uma extensão aditiva em `ctech-go-common` (`gopkg.aoctech.app/api-commons`, §3.6). `worker/` e
`py-dfe`/`go-dfe` **não mudam** — continuam recebendo o documento já resolvido.

---

## 1. Problema

Hoje a plataforma tem cinco cadastros (`organization_products`, `organization_services`, `organization_vehicles`,
`organization_persons`, `organization_*_configs`) e quatro fluxos de emissão (NF-e, NFC-e, NFS-e, MDF-e). O que está
cadastrado é reaproveitado, mas **as decisões que se repetem a cada emissão não estão modeladas em lugar nenhum** — elas
vivem na cabeça do operador e são redigitadas toda vez.

### 1.1 Tributação duplicada dentro de cada produto

`ProductBody.cfop_config[]` (`api/internal/api/v1/dto.go`) carrega ~60 campos fiscais (ICMS/CSOSN, ST, PIS, COFINS,
IBS/CBS, IPI, IS, ISSQN) **por CFOP, dentro de cada produto**. Uma empresa com 5.000 produtos e 4 CFOPs guarda 20.000
cópias da mesma alíquota.

Consequências diretas:

- Mudança de alíquota (a transição da Reforma Tributária altera IBS/CBS todo ano até 2033) exige reescrever todos os
  produtos.
- Cadastrar um produto novo exige preencher a aba "Tributação" inteira de novo — é a aba que trava o onboarding.
- Não há como responder "quais produtos usam esta regra?".

### 1.2 A "natureza da operação" não existe como entidade

Ao emitir uma NF-e o usuário informa, campo a campo, um conjunto de valores que **sempre andam juntos** por cenário de
negócio: `nat_op`, `fin_nfe`, `ind_final`, `ind_pres`, `tp_nf`, o CFOP de cada item, a mensagem em `infAdFisco`/`infCpl`
e a forma de pagamento. "Venda para revenda", "remessa para conserto", "devolução de compra", "bonificação" e
"transferência entre filiais" são combinações fixas — mas o produto trata cada campo como uma pergunta independente.

### 1.3 Pessoas não têm papel

`organization_persons` não distingue cliente, fornecedor, transportadora, condutor ou prestador. Consequências:

- MDF-e digita **nome + CPF do condutor a cada emissão** (`ui/src/components/mdfe/MdfeEmitForm.tsx:452`,
  `MdfeDriver` em `api/internal/services/mdfes/emit.go:100`). Não há cadastro.
- NF-e aceita `transporta_pk` apontando para uma pessoa, mas não há como listar "só as transportadoras".
- NFS-e com `tp_emit != 1` precisa de um prestador do cadastro, sem nenhuma marcação de quem serve como prestador.

### 1.4 Composição veicular e condição de pagamento redigitadas

- MDF-e: veículo + até 3 reboques + condutores são escolhidos um a um, todo dia, para os mesmos conjuntos.
- NF-e: `payments[]` + `cobr_duplicatas[]` + `cobr.fat` são digitados parcela a parcela para condições fixas
  ("30/60/90", "à vista", "boleto 28 dias").

### 1.5 Dado do cadastro deliberadamente ignorado

`VehicleOwnerBody` (`dto.go`) está documentado como *"Optional static metadata — not used for MDF-e prop building
(that's a per-emission input, see mdfes.MdfeOwner)"*. O mesmo proprietário/RNTRC é cadastrado e depois redigitado na
emissão.

### 1.6 A API obriga o cadastro prévio

`NfeService.Emit` exige `receiver_id` de uma pessoa já cadastrada (`api/internal/services/nfes/emit.go:176`) e
`product_id` de um produto já cadastrado (`emit.go:540`). Um integrador que já tem o catálogo no ERP dele precisa
espelhar tudo aqui antes de emitir a primeira nota. `NfeProductItem` só aceita 4 sobrescritas de preço/quantidade —
não há como mandar a tributação completa no request.

`MdfeVehicle` (`mdfes/emit.go:73`) já resolve isso corretamente — aceita `sk` **ou** os campos inline. É a
implementação de referência; ninguém mais segue.

---

## 2. Objetivos

1. **Configurar uma vez, reutilizar sempre.** Toda decisão que se repete vira uma entidade nomeada, escolhida por
   referência na emissão.
2. **Simples por padrão, completo por opção** na UI: o caminho curto usa os defaults do cadastro; o modo avançado
   expõe todo campo do leiaute.
3. **Liberdade total na API**: qualquer valor resolvido do cadastro pode ser sobrescrito inline no request, com
   fallback determinístico e documentado.
4. **Zero quebra.** Todo cadastro e todo contrato de API atuais continuam funcionando sem migração de dados.

### Não-objetivos

- CT-e: só o desenho das entidades leva CT-e em conta (`doc_types` já aceita `cte`). Nenhuma emissão de CT-e é
  construída aqui.
- Nenhuma mudança em `worker/`, `py-dfe/` ou `go-dfe/`. Eles recebem o documento já resolvido, como hoje.
- Nenhuma renomeação de campo existente.

---

## 3. Modelo de dados

Todas as tabelas novas seguem a convenção do repositório: `pk = {org_pk}`, `sk = {PREFIXO}_{uuid}`, criadas com
`NewCRUDRepository` e montadas com `mountCRUD` (`api/internal/api/v1/crud_handlers.go`), com GSI `description-index` ou
`name-index` para busca por prefixo, exatamente como `organization_products` e `organization_services`.

> **Alternativa considerada e recusada:** uma única tabela `organization_presets` com SK discriminado por tipo.
> Reduziria 4 blocos de CDK a 1, mas quebra a convenção "uma tabela por entidade" que todo o repo segue, e embaralha
> `AuditResource*` e as permissões RBAC (`create.operations` vs `create.presets`). O custo marginal de cada tabela nova
> é baixo justamente porque `NewCRUDRepository` + `mountCRUD` já existem.

### 3.1 `organization_tax_profiles` — perfis fiscais

| Campo | Tipo | Nota |
|---|---|---|
| `name` | string, obrigatório | "Venda de mercadoria — Simples Nacional" |
| `description` | string, opcional | |
| `cfops` | `[]string`, min 1 | CFOPs cobertos por este perfil (tipicamente `5102` + `6102`) |
| *(todos os campos de `CfopConfigBody` exceto `cfop`)* | | ICMS/CSOSN, ST, PIS, COFINS, IBS/CBS, IPI, IS, ISSQN |

**GSI:** `name-index` (`pk`, `name`).

Um perfil é **um tratamento tributário aplicado a um conjunto de CFOPs**. `5102` e `6102` compartilham perfil porque a
alíquota interestadual já é resolvida por `resolveICMSAliq(emitUF, destUF, override)`
(`api/internal/services/nfes/tax_tables.go:79`) — o que muda entre eles é dado derivado, não configuração. Quando o
tratamento realmente difere por CFOP, cria-se um segundo perfil. **Não há aninhamento por CFOP dentro do perfil.**

### 3.2 `organization_products` — referência a perfis (aditivo)

Campo novo, opcional:

```jsonc
"tax_profiles": [
  { "tax_profile_id": "TAXPROFILE_...", "overrides": { /* qualquer campo de CfopConfigBody */ } }
]
```

`cfop_config[]` **continua existindo e continua obrigatório-se-presente**. A resolução por CFOP é:

```
cfop_config[cfop]                      (explícito no produto — vence)
  → tax_profiles[].overrides           (override do produto sobre o perfil)
    → perfil (campos de tributação)
      → overrides de nível de produto  (icms_aliq_override, fcp_aliq_override)
        → tabela da UF                 (resolveICMSAliq / resolveFCPAliq)
```

O CFOP é válido para o produto se estiver em `cfop_config[]` **ou** nos `cfops` de algum perfil referenciado. A
validação de `resolveProducts` (`nfes/emit.go:554-568`) passa a checar a união dos dois.

Produto existente, sem `tax_profiles`: comportamento idêntico ao de hoje. **Nenhuma migração de dados.**

### 3.3 `organization_operations` — naturezas de operação

| Campo | Tipo | Nota |
|---|---|---|
| `name` | string, obrigatório | "Venda para revenda" |
| `doc_types` | `[]string` | `nfe` \| `nfce` \| `cte` \| `mdfe` |
| `nat_op` | string ≤60 | texto de `natOp` |
| `tp_nf`, `fin_nfe`, `ind_final`, `ind_pres` | string | mesmos domínios de `NfeEmitBody` |
| `cfop_suffix` | string(3), opcional | natureza fiscal; o dígito de escopo (5/6/7) é resolvido na emissão |
| `tax_profile_id` | string, opcional | perfil usado quando o produto não define um |
| `payment_term_id` | string, opcional | |
| `mod_frete` | string, opcional | default de `NfeTransportItem.ModFrete` |
| `inf_ad_fisco`, `inf_cpl` | string, opcional | aceitam placeholders (§3.7) |
| `requires_receiver` | bool | `false` habilita `self_issuance` |
| `is_default` | bool | operação pré-selecionada da organização |

**GSI:** `name-index`.

**Impacto entre projetos — resolução de escopo do CFOP.** Hoje a regra "5xxx intra-UF / 6xxx inter-UF" existe **só em
TypeScript** (`ui/src/lib/data/cfop.ts`, decisão explícita do design de 2026-06-23: *"Client-side only. Backend NF-e
contract is unchanged"*). Com `cfop_suffix` na operação, a API passa a precisar da mesma regra para clientes que não
usam a UI. A regra vira uma função Go — `services.ResolveCFOPScope(suffix, emitUF, destUF) (string, error)` — e o
TypeScript existente fica sendo apenas o agrupamento de exibição no dropdown. Duas implementações da mesma regra
violariam a regra de DRY do `CLAUDE.md`; a versão Go é a fonte da verdade, e o teste de paridade entre elas é
obrigatório.

Comportamento quando não existe variante para o escopo exigido: **bloqueia a emissão** com mensagem acionável, como já
decidido em 2026-06-23. Nada de sintetizar CFOP.

### 3.4 `organization_payment_terms` — condições de pagamento

| Campo | Tipo | Nota |
|---|---|---|
| `name` | string, obrigatório | "30/60/90" |
| `payment_type` | string | mesmo domínio de `NfePaymentItem.PaymentType` (`01`…`99`) |
| `ind_pag` | string, opcional | `0` à vista \| `1` a prazo |
| `installments` | int ≥1 | |
| `interval_days` | int ≥0 | intervalo entre parcelas |
| `first_due_days` | int ≥0 | dias até o primeiro vencimento |
| `card` | objeto, opcional | mesmos campos de `NfePaymentItem.Card` |

Na emissão, `payment_term_id` + total do documento expandem para `payments[]`, `cobr.fat` e `cobr_duplicatas[]`. A
expansão é uma função pura, testável isoladamente, e **a última parcela absorve a diferença de arredondamento** — a soma
das duplicatas tem que bater com `vNF` centavo a centavo, sob pena de rejeição da SEFAZ.

### 3.5 `organization_vehicle_sets` — composições veiculares

| Campo | Tipo | Nota |
|---|---|---|
| `name` | string, obrigatório | "Carreta 1 — ABC1D23" |
| `tractor_sk` | string, obrigatório | `organization_vehicles`, `role=tractor` |
| `trailer_sks` | `[]string`, máx 3 | `role=trailer` |
| `driver_docs` | `[]string` | CPFs de pessoas com papel `driver` |
| `rntrc`, `ciot` | string, opcional | |

Na emissão MDF-e, `vehicle_set_id` expande para `vehicle` + `trailers[]` + `drivers[]` + `rntrc`/`ciot`. **Cada campo
expandido continua sobrescrevível individualmente no mesmo request** — trocar o motorista de um dia não exige criar
outro conjunto.

O gating de `services.Missing` continua valendo: escolher um conjunto cujo veículo está incompleto bloqueia a emissão
com o mesmo modal de edição de hoje, agora apontando qual membro do conjunto está incompleto.

### 3.6 `organization_persons.roles` — papéis

O item da pessoa ganha um campo, e só isso:

```jsonc
"roles": ["customer", "supplier", "carrier", "driver", "provider"]
```

**Multi-papel é o caso normal, não a exceção.** Uma transportadora costuma ser cliente também; um prestador de serviço
costuma ser fornecedor. O modelo trata isso de frente: `roles` é lista, uma pessoa é um único item, e ela aparece
**uma vez** na listagem geral e em **todas** as listagens de papel que possui.

Exemplo — Transportes Acme, cliente e transportadora ao mesmo tempo:

```
pk = {org_pk}   sk = CNPJ_12345678000199
  name  = "Transportes Acme"
  roles = ["customer", "carrier"]
```

O picker de destinatário e o picker de transportadora acham a Acme; a listagem de pessoas mostra ela uma vez;
`GET /persons/12345678000199` devolve um item com os dois papéis.

#### Estratégia de consulta

`organization_persons` já tem o GSI `org-name-index` (`pk`, `name`). A busca por papel reusa esse índice com um filtro:

```
Query(IndexName = "org-name-index", pk = org_pk, begins_with(name, q))
  FilterExpression: contains(roles, :role)
```

**Nenhuma tabela nova, nenhum GSI novo, nenhuma linha derivada.** O item da pessoa continua sendo a única fonte da
verdade, o que elimina de saída a classe de bug "índice dessincronizado do cadastro".

O filtro do DynamoDB é aplicado **depois** da condição de chave, então paga-se a leitura das pessoas que casam o prefixo
de nome mas não têm o papel. Isso é aceitável porque **o picker é debounced e exige 2 caracteres antes de buscar**: numa
organização com 5.000 pessoas, `"jo"` traz algumas dezenas de itens e filtra sobre elas. É a leitura de uma página, não
uma varredura.

> **Alternativas consideradas e recusadas.**
> **(a) Lista embutida no item da organização** (o padrão de `pickup_locations`, `nfes/emit.go:450`). Recusada: aquele
> campo tem teto rígido de 5 (`maxSavedLocations`), é best-effort e é lista MRU, não cadastro. Um registro de motoristas
> não tem teto — uma transportadora de porte médio tem dezenas a centenas, por rotação e agregados — e o item da
> organização é lido em **toda** emissão (`GetOrganization` em NF-e, NFC-e, NFS-e e MDF-e), com limite rígido de 400 KB
> e sem controle de concorrência no read-modify-write.
> **(b) GSI multi-atributo** (`(pk)` / `(role, name)`, disponível desde nov/2025). Recusada: não resolve multi-papel. Um
> item da tabela base gera **no máximo uma entrada por GSI**, com 1 ou 8 atributos de chave, e uma lista não pode ser
> atributo de chave. A Acme precisaria de duas entradas e o índice só comporta uma.
> **(c) Linhas de adjacência** (uma linha por par pessoa-papel). Funciona e escala, mas é escrita extra, código extra e
> uma nova classe de bug, para resolver um problema que ainda não existe. Fica como caminho de escalada (abaixo).

#### Caminho de escalada

O filtro degrada num único cenário: **listar todos de um papel sem nenhum termo digitado**, numa organização com dezenas
de milhares de pessoas. Se isso aparecer, a saída é acrescentar linhas de adjacência (`pk = {org_pk}#ROLES`,
`sk = {role}#{doc}`) por backfill — `roles` no item continua sendo a fonte da verdade, as linhas viram índice derivado.
**Sem migração e sem mudança de contrato**, porque o endpoint já é o mesmo. Não construir isso agora é decisão
consciente, não omissão.

#### Dependência em `api-commons`

`dynamo.QueryOpts` (`ctech-go-common`, em uso na v1.4.1) só expõe filtro de igualdade via `FilterField`/`FilterValue`.
`contains(roles, :role)` exige estender o pacote compartilhado com um par irmão — `FilterContainsField` /
`FilterContainsValue`, seguindo o mesmo idioma de pares tipados do struct, sem abrir passagem de expressão crua.
Estender o pacote compartilhado é o caminho prescrito pelo `CLAUDE.md` global; contornar localmente com filtragem em
memória quebraria a paginação (uma página de 50 pessoas pode devolver 3 motoristas).

#### Endpoint

```
GET /v1.0/persons?role=driver&q=jo
```

`q` casa por prefixo de nome. Quando `q` é só dígitos, casa por prefixo de documento — o serviço escolhe entre
`begins_with(name, …)` no `org-name-index` e `begins_with(sk, …)` na tabela base, porque a SK **já é** o documento
(`CNPJ_…` / `CPF_…`). Busca por documento dentro de um papel, portanto, também não precisa de índice novo.

Pessoa existente sem `roles`: aparece na listagem geral e em nenhuma listagem por papel — comportamento correto, não
bug. A UI de pessoas ganha o seletor de papéis, com `customer` marcado por padrão.

**Papel é filtro de cadastro, não regra fiscal.** A emissão não valida papel: `receiver_id` e `transporta_pk` continuam
fazendo `GetItem` direto na pessoa. Exigir papel na emissão quebraria todo dado existente sem nenhum ganho fiscal.

### 3.7 Placeholders em mensagens fiscais

`inf_ad_fisco` / `inf_cpl` da operação aceitam `{{chave}}`, interpolados na emissão a partir de um mapa fechado e
documentado (`{{v_nf}}`, `{{v_icms_st}}`, `{{cliente}}`, `{{nat_op}}`, `{{competencia}}`). Chave desconhecida é **erro
de validação no cadastro da operação**, nunca uma interpolação silenciosa em branco no XML.

Uma tabela separada de mensagens fiscais reutilizáveis **não entra agora** — enquanto a mensagem for 1:1 com a
operação, um campo resolve. Vira tabela quando houver reuso comprovado entre operações.

---

## 4. Escada de resolução (contrato de API)

Uma única ordem, idêntica nos quatro documentos:

```
1. valor inline no request            (o integrador manda tudo)
2. entidade referenciada por id       (produto, serviço, pessoa, veículo, conjunto)
3. perfil / operação / condição       (as entidades desta spec)
4. config da organização              (organization_*_configs)
5. tabela normativa                   (tax_tables.go, ResolveCFOPScope)
```

Implementação: **uma** função compartilhada em `internal/services`, usada por `nfes`, `nfces`, `nfses` e `mdfes`:

```go
// MergeOverrides devolve base com todas as chaves não-nulas de inline
// aplicadas por cima, recursivamente em mapas aninhados. Chave presente
// com valor null no request explicitamente limpa o campo do cadastro.
func MergeOverrides(base, inline map[string]any) map[string]any
```

Quatro implementações separadas dessa mesma resolução seria exatamente a duplicação que o `CLAUDE.md` proíbe.

### 4.1 Sobrescrita inline (fase 3)

| Documento | Hoje | Passa a aceitar |
|---|---|---|
| NF-e / NFC-e | `product_id` + 4 overrides | `product_id` opcional + objeto `product` inline (shape de `ProductBody`, todos os campos opcionais) + `save_product` |
| NF-e | `receiver_id` obrigatório | `receiver_id` **ou** `receiver` inline (shape de `PersonCreateBody`) + `save_receiver` |
| NFS-e | `service.service_id` + 4 overrides | objeto `service` inline completo; `customer` / `intermediary` / `provider` inline |
| MDF-e | `vehicle` já aceita inline | mesmo padrão em `trailers[]` e `drivers[]` |

`save_*: true` promove o objeto inline a cadastro no mesmo `TransactWrite` da emissão.

### 4.2 `POST /v1.0/{doc}/preview` — dry-run

Recebe o mesmo body da emissão e devolve o documento **totalmente resolvido** — itens com tributação calculada, totais,
CFOP resolvido, parcelas expandidas — mais a lista de problemas de validação, **sem reservar número fiscal e sem gravar
nada**. É o que torna a API realmente completa: o integrador vê exatamente o que será enviado antes de se comprometer.

`api/internal/services/mdfes/preview.go` já faz isso para a carga do MDF-e; o padrão é generalizado, não inventado.

Na UI, o mesmo endpoint alimenta o passo de revisão e o inspetor de payload do modo avançado — a revisão deixa de ser
uma reconstrução em TypeScript do que o backend vai calcular.

### 4.3 Prontidão de cadastro

`services.Missing` (`api/internal/services/vehicle_requirements.go:35`) é a melhor abstração do repositório e está presa
a veículos. Generalizar a assinatura para `Missing(item, entity, docType, role)` e expor:

```
GET /v1.0/products/:product_id/requirements?doc_type=nfe
GET /v1.0/persons/:cpf_cnpj/requirements?doc_type=nfse&role=provider
GET /v1.0/services/:service_id/requirements?doc_type=nfse
GET /v1.0/vehicles/:sk/requirements?doc_type=mdfe   (já existe)
```

Alimenta o selo "pronto para NF-e ✓ / NFS-e ✗ (falta regime tributário)" nas listagens — e responde, no cadastro, a
pergunta que hoje só aparece como rejeição da SEFAZ.

---

## 5. Experiência por tipo de documento

Segue `DESIGN.md`: uma marca verde calma, um acento por tipo de documento via `data-dfe-theme`, vocabulário de controles
compartilhado. As entidades desta spec mudam **o que se pergunta**, não o vocabulário visual.

### 5.1 Modo avançado — regra transversal

Um único toggle **"Modo avançado"**, persistido **por usuário** (não por formulário), no topo de qualquer tela de
emissão. Desligado: só os campos que a operação não resolveu. Ligado: todos os grupos opcionais do leiaute + o inspetor
do payload resolvido (`/preview`).

Não é um `CollapsibleSection` a mais. Hoje a NF-e tem uma seção "Configurações avançadas"
(`NfeEmitForm.tsx:1569`) que só cobre transporte e informações adicionais; ela é substituída pelo toggle global.

### 5.2 NF-e — documento pensado

Wizard de 4 passos vira **3**, porque a operação responde o que hoje são perguntas soltas:

| Passo | Simples | Avançado revela |
|---|---|---|
| 1 · Cliente e operação | busca de pessoa (`role=customer`) + operação (a padrão já selecionada) | `tp_nf`, `fin_nfe`, `ind_final`, `ind_pres`, `nat_op` editáveis |
| 2 · Itens | busca de produto, quantidade, preço do cadastro; CFOP e tributação resolvidos e **exibidos como texto, não como campo** | CFOP e cada campo tributário do item viram editáveis; frete/seguro/outros; dados de veículo/arma |
| 3 · Revisão | documento renderizado com totais e impostos vindos de `/preview`, "Editar" por bloco; pagamento pré-expandido da condição da operação | transporte, `cobr`/duplicatas linha a linha, retirada/entrega, `infAdFisco`/`infCpl`, inspetor de payload |

O ganho real: **passo 2 deixa de ter dropdown de CFOP no caminho feliz.** Hoje o operador escolhe CFOP por item, o que
exige saber a natureza fiscal da operação item a item.

### 5.3 NFC-e — venda de balcão

Continua uma tela só (regra já registrada em `DESIGN.md`, "Shared Emission Vocabulary"). Ganha: operação implícita fixa
da organização, tributação do perfil, preço de consumidor final. **Nenhum passo novo** — o único acréscimo aceitável a
uma tela de balcão é nada.

### 5.4 NFS-e — um serviço, um tomador

Uma tela com cinco decisões visíveis e "Mais opções" para o resto — o que `DESIGN.md` já registra. Ganha: tomador
buscado por `role=customer`, prestador por `role=provider` quando `tp_emit != 1`. Estrutura inalterada.

### 5.5 MDF-e — confirmar o derivado

O passo "veículo" vira **um select de conjunto**, substituindo veículo + até 3 reboques + condutores. Os campos
expandidos continuam visíveis e editáveis (é a diferença entre um default e uma prisão). Rota, carregamento e descarga
continuam derivados dos documentos referenciados, como já são.

### 5.6 Telas de cadastro novas

- **Perfis fiscais** — a aba "Tributação" de `ProductForm.tsx` (hoje dentro de um formulário de 1.989 linhas) vira uma
  tela própria. `ProductForm` passa a mostrar um seletor de perfis + "sobrescrever para este produto", e encolhe
  bastante. É a mudança que mais reduz o custo de cadastrar produto.
- **Naturezas de operação** — lista + formulário curto; uma marcada como padrão.
- **Condições de pagamento** — lista + formulário curto, com pré-visualização das parcelas geradas.
- **Composições** — lista + formulário; o formulário reusa os seletores de veículo e de pessoa (`role=driver`).
- **Ação em massa:** "aplicar perfil a N produtos" na listagem de produtos. Sem ela, quem já tem catálogo não migra.

---

## 6. Compatibilidade

| Área | Garantia |
|---|---|
| `cfop_config[]` | continua aceito, continua vencendo sobre o perfil. Nenhuma migração de dados. |
| Produtos sem `tax_profiles` | resolução idêntica à de hoje |
| Pessoas sem `roles` | listagem geral inalterada; ausentes das listagens por papel |
| `Query` de pessoas sem `role` | caminho de código inalterado — o filtro só entra quando `role` é informado |
| `api-commons` | extensão puramente aditiva de `QueryOpts`; nenhum consumidor existente afetado |
| Bodies de emissão | todos os campos novos são opcionais; todo request válido hoje continua válido |
| `worker` / `py-dfe` / `go-dfe` | nenhuma mudança; recebem o documento resolvido |
| Numeração fiscal | intocada — a reserva por `transact_write` não muda |

---

## 7. Faseamento

| Fase | Conteúdo | Depende de |
|---|---|---|
| **1a** | `persons.roles` (multi-papel) + `?role=`/`?q=` + `FilterContains` em `api-commons` + condutor/transportadora por seleção | — |
| **1b** | `organization_tax_profiles` + `product.tax_profiles` + resolução + ação em massa | — |
| **1c** | `organization_operations` + `ResolveCFOPScope` em Go + NF-e/NFC-e consumindo | 1b |
| **1d** | `organization_payment_terms` + `organization_vehicle_sets` | 1a |
| **2** | Modo avançado global + telas de cadastro novas + NF-e em 3 passos | 1b, 1c |
| **3** | `MergeOverrides` + sobrescrita inline + `save_*` + `POST /{doc}/preview` | 2 |
| **4** | `Missing` generalizado + selos de prontidão + `emit_input` nos 4 documentos ("duplicar emissão") | 3 |

Fase 1 (a–d) é o escopo aprovado deste ciclo. Fases 2–4 estão especificadas aqui para que o modelo de dados da fase 1
não precise ser refeito depois.

---

## 8. Riscos

| Risco | Mitigação |
|---|---|
| `ResolveCFOPScope` divergir entre Go e TypeScript | Go é a fonte da verdade; teste de paridade sobre a mesma tabela de casos, obrigatório no CI |
| Busca por papel degradar numa organização com dezenas de milhares de pessoas | Mínimo de 2 caracteres no picker; caminho de escalada para linhas de adjacência já desenhado (§3.6), sem migração |
| Página de resultados encolher pelo filtro (`Limit` conta itens lidos, não retornados) | O serviço pagina até completar a página pedida ou esgotar o cursor, com teto de 5 idas por requisição; fim de lista é cursor ausente, nunca contagem baixa |
| Query de papel sem termo de busca varrer a partição inteira (~6 MB numa organização de 10 mil pessoas, 6 idas em série, 750 RRU contra o teto de 1.000 do índice) | Picker nunca consulta sem 2 caracteres; a listagem paginada lê os mesmos bytes que já lê hoje sem filtro |
| Expansão de parcelas não fechar com `vNF` | Última parcela absorve o resíduo; teste com totais que não dividem exatamente (ex.: R$ 100,00 em 3×) |
| Perfil editado alterar a tributação de notas futuras sem aviso | O cadastro do perfil informa quantos produtos o referenciam antes de salvar |
| `ProductForm` (1.989 linhas) regredir na refatoração | Perfis entram **aditivos** primeiro; a aba "Tributação" só é reduzida depois que a nova tela estiver em produção |

---

## 9. Documentação obrigatória

`DOCS.md` (endpoints, schemas, resolução, `?role=`/`?q=`), `DynamoDB-Tables.md` (4 tabelas novas + o campo `roles` em
`organization_persons`), `OVERVIEW.md` (mapa de tabelas e contagem), `CONDUCT.md` (a regra "a ordem de resolução é única
e vive em `MergeOverrides`"), e o README/AGENTS de `ctech-go-common` para o par `FilterContains*` novo.
