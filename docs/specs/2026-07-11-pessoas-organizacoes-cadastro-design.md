# Cadastro de Pessoas/Organizações — UX Fluida + Local de Entrega/Retirada — Design

## Contexto

`api/internal/repositories/persons.go` (`organization_persons`) e `api/internal/repositories/organizations.go`
(`organizations`) já compartilham o mesmo shape rico via `PersonObjectBody` (`api/internal/api/v1/dto.go:41-47`):
múltiplos endereços (`addresses[]`, min 1 — já obrigatório), múltiplas inscrições estaduais por UF
(`state_registrations[]`, já opcional), contatos (`emails[]`/`phones[]`, já opcional), `fantasy_name`, `crt`.
O front-end usa um único `EntityForm` (`ui/src/components/EntityForm.tsx`) + `entitySchema`
(`ui/src/lib/schemas/entity.ts`) para pessoas e organizações.

Ou seja: os campos mínimos (nome + CPF/CNPJ + 1 endereço) **já são obrigatórios** hoje, e a
experiência de múltiplas IEs **já existe**. O que falta:

1. UI ainda mostra tudo "aberto" — sem separação clara entre essencial e avançado.
2. Backend não valida CRT nem IE como obrigatórios para pessoa jurídica emitente (organização) —
   só o front bloqueia isso hoje, é possível burlar via API direta.
3. `PersonService.Create` não verifica CPF/CNPJ duplicado — sobrescreve silenciosamente
   (`api/internal/services/persons.go:79-104`, `BuildCreateTxItem` sem `ConditionExpression`).
4. Não existe local de entrega/retirada (TLocal do XSD NF-e) em nenhuma camada — nem DTO, nem
   builder, nem UI. Zero ocorrências de `entrega`/`retirada` em `api/internal/services/nfes/`.

## Por que NÃO replicar o gating de veículo (bloqueia emissão + modal)

No cadastro de veículo, tara/carroceria variam genuinamente por tipo de documento (MDF-e trator
vs. reboque). Em pessoa/organização, revisando os XSDs de NF-e emit/dest e CT-e emit/rem/exped/
receb/dest:

- Endereço já é sempre obrigatório no cadastro (baseline), então já satisfaz `TEndereco` (CT-e) e
  cobre a maioria dos casos de `TEnderEmi`/`enderDest` do NF-e mesmo onde o XSD permite omitir.
- IE em destinatário/remetente é condicional à **operação** (`indIEDest`), não ao **cadastro** — não
  dá pra "completar o cadastro" pra resolver isso, é uma decisão por emissão.
- CRT e "ao menos 1 IE" são, isso sim, requisitos reais e fixos do emitente (a organização) — mas
  não variam por tipo de documento, então são regra de cadastro (validação no Create/Update), não
  gating por emissão.

Conclusão: sem necessidade de endpoint `/requirements` nem modal de edição para pessoa/organização.

## Mudanças

### 1. Progressive disclosure — `EntityForm.tsx`

Seção "Informações adicionais" (colapsável, fechada por padrão — abre automaticamente se, ao
editar um registro existente, algum campo dela já tiver dado):
- Nome fantasia
- Endereços 2+ (endereço principal continua sempre visível)
- Contatos (e-mails/telefones)
- Inscrições Estaduais — **exceto** quando `variant === 'organization' && tipo === 'pj'`, onde
  fica sempre visível fora do avançado (agora obrigatória, ver item 3).

Mesmo padrão visual do `VehicleForm.tsx` (`+ Configurações avançadas` / `− Ocultar`).

### 2. CRT obrigatório no backend para PJ

Hoje `PersonObjectBody.Crt` é `*int` `omitempty` — sem checagem de presença no backend; só o
Zod (`entity.ts:57-59`) força isso no front. Adicionar checagem em
`PersonService.Create/Update` e `OrganizationService.Create/Update`: se `cpf_or_cnpj` tem 14
dígitos (CNPJ) e `Crt` é nil (Create) ou o valor final após merge é nil (Update), retornar
`problem.BadRequest("crt é obrigatório para pessoa jurídica")`.

### 3. IE obrigatória para Organização PJ

Nova checagem, **só em `OrganizationService`**: se CNPJ e `state_registrations` (após merge, no
caso de Update) está vazio, retornar `problem.BadRequest("ao menos uma inscrição estadual é
obrigatória para organização com CNPJ")`. Não se aplica a `PersonService` — destinatário PJ pode
legitimamente não ter IE registrada com a organização (isento/não contribuinte, resolvido via
`indIEDest` na emissão).

### 4. Dedup CPF/CNPJ em pessoas

`api/internal/repositories/persons.go` — `BuildCreateTxItem` ganha uma variante (ou parâmetro)
que adiciona `ConditionExpression: aws.String("attribute_not_exists(pk)")` ao `Put` transact item
(mesmo padrão de `worker/internal/service/distribution.go:774-780`, `persistPerson`).
`PersonService.Create` mapeia `*types.TransactionCanceledException` (condition check failed) para
`problem.Conflict("pessoa com este CPF/CNPJ já cadastrada")`.

`OrganizationService.Create` **não muda** — PK já é o próprio CNPJ/CPF, então duplicidade é
estruturalmente impossível; o comportamento atual de "get-or-return" é intencional (reentrada no
cadastro da mesma empresa) e já satisfaz "sem duplicados" de forma mais forte que um 409.

### 5. Local de entrega/retirada (NF-e)

Campo livre por emissão, com sugestão de reaproveitamento do que já foi usado para aquele
destinatário (mesma filosofia dos "favoritos" de destinatário que `NfeEmitForm.tsx:877-881,1217`
já implementa hoje a partir do histórico de NF-e emitidas).

**Shape TLocal** (idêntico para retirada e entrega, conforme `xsd_order.py:73-82` — **note:
sem `CEP`**, ao contrário de `enderEmit`/`enderDest` que usam `TEndereco`):
```go
// api/internal/services/nfes/emit.go
type NfeLocalBody struct {
    CNPJ    *string `json:"cnpj" validate:"omitempty,cnpj"`
    CPF     *string `json:"cpf" validate:"omitempty,cpf"`
    XNome   *string `json:"x_nome" validate:"omitempty,max=60"`
    XLgr    string  `json:"x_lgr" validate:"required,max=255"`
    Nro     string  `json:"nro" validate:"required,max=60"`
    XCpl    *string `json:"x_cpl" validate:"omitempty,max=60"`
    XBairro string  `json:"x_bairro" validate:"required,max=60"`
    CMun    string  `json:"c_mun" validate:"required,ibge"`
    XMun    string  `json:"x_mun" validate:"required,max=60"`
    UF      string  `json:"uf" validate:"required,uf"`
    Fone    *string `json:"fone" validate:"omitempty,phonebr"`
    Email   *string `json:"email" validate:"omitempty,email"`
}
```

`NfeEmitBody` ganha:
```go
Retirada             *NfeLocalBody `json:"retirada" validate:"omitempty"`
Entrega              *NfeLocalBody `json:"entrega" validate:"omitempty"`
SaveRetiradaLocation bool          `json:"save_retirada_location"`
SaveEntregaLocation  bool          `json:"save_entrega_location"`
```

`builders_doc.go` ganha `buildLocal(l *NfeLocalBody) map[string]any` (chaves: `CNPJ`/`CPF`,
`xNome`, `xLgr`, `nro`, `xCpl`, `xBairro`, `cMun`, `xMun`, `UF`, `cPais`/`xPais` sempre
`cPaisBrasil`/`xPaisBrasil`, `fone`, `email` — omitindo campos vazios), incluído em
`BuildEnviNFe`'s `infNFe` dict sob as chaves `retirada`/`entrega` só quando presentes. **Nenhuma
mudança no py-dfe** — `xsd_order.py` já ordena essas chaves.

**Persistência para reuso**: `organization_persons` ganha atributo opcional `delivery_locations`
(lista de `NfeLocalBody`-shaped maps, cap 5, dedup por `xLgr+nro+xCpl` normalizado); `organizations`
ganha `pickup_locations` (mesmo shape/cap, ligado à organização = remetente). Em `NfeService.Emit`,
após sucesso, se `SaveEntregaLocation`/`SaveRetiradaLocation`, chama
`PersonService.Update`/`OrganizationService.Update` (best-effort — falha aqui não derruba a
emissão, só loga).

**Leitura**: nenhum endpoint novo — os dois campos entram no shape já retornado por
`GET /persons/:cpf_cnpj` e `GET /organizations/:pk` (`PersonOut`/`OrganizationOut` no
`ui/src/lib/types/api.ts`).

**UI (`NfeEmitForm.tsx`)**: novo bloco colapsável "Local de entrega" (visível após destinatário
selecionado) — se `receiver.person.delivery_locations` não vazio, mostra como chips
selecionáveis pra preencher os campos; botão "endereço diferente" abre o formulário livre
(reaproveita `AddressFields`-like layout, mas sem CEP/IBGE code obrigatório — TLocal é mais
simples que `TEndereco`); checkbox "Salvar este local para reutilizar" (marcado por padrão ao
digitar endereço novo). Bloco "Local de retirada" análogo, alimentado por
`organization.pickup_locations` (fonte: contexto de auth/org atual, já carregado).

### Fora de escopo (documentado, não implementado)

CT-e: **não existe serviço CT-e no backend** (`api/internal/services/ctes` não existe) nem
formulário de emissão no front (`ui/src/app/cte/` só tem listagem/distribuições). O XSD de CT-e
foi usado só como referência de campos — nenhuma mudança é feita lá. Quando o backend/UI de CT-e
for construído, os mesmos princípios (endereço já obrigatório cobre `TEndereco`, sem gating extra
necessário) se aplicam diretamente.

## Testes

| Mudança | Teste |
|---|---|
| CRT/IE obrigatórios (backend) | Unit: `PersonService`/`OrganizationService` Create/Update rejeitam PJ sem CRT; Organization rejeita PJ sem IE |
| Dedup pessoa | Integration: segundo `Create` com mesmo CPF/CNPJ na mesma org retorna 409 |
| `buildLocal`/entrega/retirada no builder | Unit: `builders_doc_test.go` — presença/ausência condicional, ausência de `CEP` |
| Persistência de local para reuso | Integration: emissão com `save_entrega_location=true` atualiza `organization_persons.delivery_locations` |
| EntityForm progressive disclosure | Component test: avançado fechado por padrão, abre com dado existente |
| IE obrigatória só pra org (schema front) | Unit: `entity.test.ts` — `organizationSchema` reprova PJ sem IE, `personSchema`/`entitySchema` não |

## Impacto cross-project

- **api**: `dto.go`, `services/persons.go`, `services/organizations.go`, `services/nfes/emit.go`,
  `services/nfes/builders_doc.go`, `repositories/persons.go`.
- **ui**: `EntityForm.tsx`, `entity.ts` (novo `organizationSchema`), `NfeEmitForm.tsx`,
  `lib/types/api.ts`, `lib/api/client.ts`.
- **cdk**: nenhuma mudança (campos novos são atributos opcionais em itens existentes, sem GSI novo).
- **py-dfe**: nenhuma mudança (`xsd_order.py` já suporta `retirada`/`entrega`).
- **worker**: nenhuma mudança de escopo identificada (`persistPerson` continua funcionando —
  `delivery_locations`/`pickup_locations` são campos novos que ele simplesmente não popula).
- **DOCS.md / CONDUCT.md / DynamoDB-Tables.md**: atualizar com os campos novos e a nota "gating
  por doc-type não se aplica a pessoas — ver design doc" para não ser reinventado depois.
