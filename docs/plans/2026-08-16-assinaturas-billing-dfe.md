# Assinaturas CTech DF-e — integração com ctech-billing

**Data:** 2026-08-16
**Repos tocados:** `ctech-dfe` (api, worker, ui, cdk), `ctech-billing` (api, tenants), `ctech-account` (configuração)

---

## 1. Veredito sobre o ctech-billing atual

**Suficiente para MVP.** É o repo mais maduro da família nesse eixo: domínio, persistência, cobrança
via wallet/PIX, dunning, webhooks assinados, auditoria transacional e portal de pagamento estão
construídos e cobertos por testes de integração. O catálogo DF-e **já está semeado** em
`api/tenants/ctech.json` com os cinco produtos e os preços com cotas em metadata.

O que existe e será reusado sem alteração:

| Necessidade DF-e                | Já existe em billing                                                                                                    |
|---------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| Catálogo de planos com cotas    | `tenants/ctech.json` — `prod_dfe_{free,pro,unlimited,ondemand,unlimited_internal}`, cotas em `Price.Metadata`           |
| Criar cliente / assinatura      | `POST /v1.0/customers`, `POST /v1.0/subscriptions` (M2M, escopo `billing:*`)                                            |
| Checkout do plano pago          | resposta de `POST /v1.0/subscriptions` traz `invoice.checkout_url` (paylink HMAC) + página hospedada `/checkout?token=` |
| Cobrança PIX                    | `services.Collector` → wallet `POST /v1.0/internal/wallet/charge` + webhook + reconciliação                             |
| Bloqueio por falta de pagamento | `internal/domain/billing/dunning.go` — D−3/D+1/D+3/D+7, `PAST_DUE` em D+10, `UNCOLLECTIBLE`+cancelamento em D+30        |
| Cancelamento                    | imediato e no fim do período, nas duas superfícies, com causa auditada                                                  |
| Uso medido (sob demanda)        | `POST /v1.0/usage` com `idempotency_key` obrigatória                                                                    |
| Notificação de eventos          | `cmd/deliver`, HMAC `X-Billing-Signature: v1=`, roteado por `owner_key` — endpoint `whe_dfe` **já semeado**             |
| Pró-rata                        | `internal/domain/billing/proration.go`, testado por propriedade                                                         |
| Credencial M2M do DF-e          | `credentials: [{client_id: "ctech-dfe"}]` já no plano do tenant                                                         |

### Lacunas reais (todas pequenas, nenhuma estrutural)

1. **`Subscribe` cria `ACTIVE`, nunca `INCOMPLETE`** (`services/subscribing.go:67`, comentário
   admite ser interino). Assinatura Pro concede serviço antes de pagar. A máquina de estados já
   suporta `INCOMPLETE → ACTIVE` por `CauseInvoicePaid`; ninguém a aciona.
2. **`invoice.paid` não propaga para a assinatura.** `grep CauseInvoicePaid internal/services` não
   retorna nada. Consequência: quem cai em `PAST_DUE` no D+10 e paga no D+12 **fica `PAST_DUE`
   para sempre**. A aresta `PastDue → Active` (`EventSubscriptionRecovered`) existe e está morta.
3. **Não há rota de troca de plano.** Só create e cancel. O upgrade Free→Pro não tem caminho.
4. **Catálogo não é legível por M2M.** `billing:products:read` existe no manifesto mas só está
   montado em `/v1.0/console/*` (sessão de usuário dono da org de billing). O DF-e não consegue
   listar planos/preços/cotas.
5. **`GET /v1.0/entitlements` é pobre demais.** Devolve `{entitled, status, period}` por assinatura
   — sem plano, sem preços, sem cotas. O DF-e precisaria de N chamadas extras.
6. **NFS-e não tem cota nem medidor** no catálogo, e NFS-e já emite em produção.

Nenhuma delas bloqueia: **billing está deployado e rodando em produção** com o que tem hoje. O
`PLAN.md § "What is actually left"` do repo de billing está desatualizado nesse ponto — o item 1,
"the first deploy, never run", já aconteceu.

### Lacunas no lado DF-e

Zero integração hoje: nenhuma referência a billing/assinatura em `api/`, `worker/` ou `ui/`.
Nenhuma cota é aplicada em lugar nenhum — qualquer conta emite ilimitadamente hoje.
A landing (`ui/src/app/page.tsx`) já anuncia Free / Pro R$ 350 / Sob demanda com as mesmas cotas do
catálogo semeado, em constante `PLANS` hardcoded.

---

## 2. Decisões (confirmadas com o dono do produto, 2026-08-16)

| #  | Decisão                                                                     | Consequência                                                                                                                                                                                                                     |
|----|-----------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| D1 | **Assinatura pertence à conta (usuário ctech-account), não à organização.** | `Customer.UserID` = subject do ctech-account; `Customer.ExternalRef` = `USER_{sub}`. `quota_companies` limita quantas organizações DF-e aquele usuário pode criar. Uma organização é governada pela assinatura da conta apontada pelo seu `owner_user_id` (Fase 2.3) — ver D1a. |
| D1a | **A organização aponta para a assinatura; a assinatura não pertence à organização** (esclarecido 2026-08-16). | Os membros não assinam nada: a organização carrega `owner_user_id`, e é a assinatura **daquela conta** que governa emissão, cotas e bloqueio para **todos** os membros, qualquer que seja o papel. Não inverter e pôr a assinatura na organização, porque `quota_companies` só significa alguma coisa se uma assinatura abrange várias organizações — assinatura por organização faria o Pro (10 empresas) ser cobrado dez vezes. `owner_user_id` é um **campo gravado**, nunca derivado do papel: é a mesma informação que o OWNER carrega, mas lida numa consulta e não numa varredura de membros, e sobrevive a qualquer mudança futura de papéis. Ver D1b — a unicidade do OWNER, de que isto depende, era uma suposição e virou uma invariante. |
| D1b | **Uma organização tem exatamente um OWNER, e é quem a criou** (implementado 2026-08-16, antes do resto). | A revisão da D1a apontou que o DF-e deixava criar um segundo OWNER: `MembershipService.ChangeRole` aceitava `OWNER`, e o que barrava era um `validate:"oneof=ADMIN USER VIEWER"` no DTO de uma rota — invariante que valia só para quem passasse por aquele DTO, e a futura transferência de propriedade seria um segundo chamador. Com dois OWNERs, "o plano do dono" teria duas respostas e o billing pegaria a linha que voltasse primeiro. Fechado com `repositories.GrantableRoles` — uma lista, checada por `MembershipService.Create`, `ChangeRole` e `InvitationService.Create` — e o `oneof` do DTO removido, porque era a terceira cópia da mesma lista. Ownership passa a ser fato de criação, não papel concedido; quem precisa de acesso total recebe ADMIN, que tem o conjunto idêntico de permissões. `guardLastOwner` continua **contando** em vez de recusar de saída, para que uma organização com dois OWNERs legados possa ser consertada rebaixando um. Transferência de propriedade continua fora de escopo; quando existir, **move** o OWNER único. |
| D2 | **`PAST_DUE`/`CANCELED` bloqueia emissão E todos os cadastros.**            | Gate em toda escrita org-scoped. Carve-outs abaixo.                                                                                                                                                                              |
| D3 | **Upgrade via nova rota em billing, com pró-rata.**                         | `POST /v1.0/subscriptions/:id/change` em ctech-billing, reusando `proration.go`. Duas linhas separadas na fatura (crédito do antigo, cobrança do novo), nunca uma linha líquida.                                                 |
| D4 | **NFS-e ganha cota e medidor agora.**                                       | `quota_nfse` nos preços fixos, `price_dfe_ondemand_nfse` no sob demanda.                                                                                                                                                         |
| D5 | **Cota de usuários** (2026-08-16). Free = 1, Pro = 25, Ilimitado = sem limite. | `quota_users` nos quatro preços fixos. Contada **por conta**, não por organização — é a mesma unidade que `quota_companies`, e 25 usuários por organização vezes 10 organizações seriam 250, que não é o que o plano vende. Usuários distintos: quem participa de duas organizações da mesma conta conta uma vez. Aplicada no convite (Fase 3.3). |

### Carve-outs do D2 (bloqueio) — deliberados, não esquecimento

Continuam permitidos com assinatura `PAST_DUE` ou `CANCELED`:

- **Leitura de tudo** — listar/consultar documentos, baixar XML e DANFE/DANFCE. O cliente pagou por
  documentos já emitidos; guarda fiscal é obrigação dele por 5 anos e não pode ser retida.
- **Cancelamento e eventos de documentos já emitidos** (`POST /nfes/{key}/cancel`, carta de
  correção, encerramento de MDF-e). Cancelamento de NF-e tem prazo legal de 24 h; bloquear isso
  causa dano fiscal real ao cliente por uma dívida de R$ 350.
- **Manifestação e distribuição** — recebimento de documentos de terceiros é passivo e tem prazo legal.
- **Tudo do próprio billing** — ver plano, ver faturas, pagar, trocar de plano, cancelar.

Bloqueado: emissão (NF-e/NFC-e/CT-e/MDF-e/NFS-e), criação de organização, e toda escrita de cadastro
(produtos, serviços, pessoas, veículos, conjuntos de veículos, perfis tributários, operações,
condições de pagamento, certificados, configurações fiscais, convites e membros).

### Nota sobre `IsEntitled()`

`billing.Subscription.IsEntitled()` devolve **true para `PAST_DUE`** por decisão de billing
(o cliente tinha o serviço e o dunning ainda não desistiu). O DF-e **não pode usar esse campo** para
o gate do D2 — precisa ler o `status`. Não alterar a semântica em billing: poker e wallet dependem dela.

---

## 3. Arquitetura da integração

```
                      ┌──────────────────┐
   OAuth PKCE ───────▶│  ctech-account   │◀────── client_credentials (ctech-dfe)
                      └──────────────────┘
                               ▲
   ┌───────────┐               │                    ┌────────────────┐
   │  ui (dfe) │───────────────┼───────────────────▶│ ctech-billing  │
   └─────┬─────┘  redirect p/ checkout hospedado    └───────┬────────┘
         │                                                   │
         ▼  /v1.0/billing/*                                  │ webhook HMAC
   ┌───────────┐   M2M billing:*                             │ (owner_key=dfe)
   │ api (dfe) │◀────────────────────────────────────────────┘
   └─────┬─────┘   POST /v1/internal/webhooks/billing
         │
         │ contador atômico de cota (DynamoDB)
         ▼
   ┌───────────┐   POST /v1.0/usage (só planos metered)
   │  worker   │──────────────────────────────────▶ ctech-billing
   └───────────┘
```

**Regras de fronteira:**

- O DF-e é a **única** origem de escrita em billing para o produto `dfe`. A UI do DF-e nunca fala
  direto com a API de billing — exceto pelo redirect para a página de checkout hospedada, que é
  autenticada pelo token assinado na URL e não pela sessão.
- Cota é **decidida e aplicada no DF-e**, com os limites lidos do catálogo de billing. Billing é a
  fonte da verdade do *limite*; o DF-e é a fonte da verdade do *consumo*.
- Uso medido é reportado **na autorização** (worker), nunca no pedido — documento rejeitado não é
  receita.

### Modelo de dados novo no DF-e

**Tabela nova: `{prefix}_account_billing`** — snapshot local da assinatura, mantido por webhook.

| Campo                              | Tipo | Nota                                                                       |
|------------------------------------|------|----------------------------------------------------------------------------|
| `pk`                               | S    | `USER_{sub}` — partition key, sem sort key                                 |
| `billing_customer_id`              | S    | `cus_...` de billing                                                       |
| `subscription_id`                  | S    | `sub_...`, vazio se nunca assinou                                          |
| `plan`                             | S    | `free` \| `pro` \| `unlimited` \| `ondemand` — de `Price.Metadata["plan"]` |
| `status`                           | S    | `INCOMPLETE`/`TRIALING`/`ACTIVE`/`PAST_DUE`/`PAUSED`/`CANCELED`            |
| `quotas`                           | M    | `{companies, nfe, nfce, cte, mdfe, nfse}` — inteiros, `-1` = ilimitado     |
| `meters`                           | M    | `{nfe: "price_...", ...}` — só em plano metered, para reportar uso         |
| `period_start` / `period_end`      | S    | `YYYY-MM-DD`                                                               |
| `cancel_at_period_end`             | BOOL |                                                                            |
| `open_invoice_id` / `checkout_url` | S    | fatura em aberto, para o banner de pagamento                               |
| `synced_at`                        | S    | ISO-8601 UTC                                                               |

**Tabela nova: `{prefix}_account_usage_counters`** — consumo do período, atômico.

| Campo                                    | Tipo | Nota                                                         |
|------------------------------------------|------|--------------------------------------------------------------|
| `pk`                                     | S    | `USER_{sub}`                                                 |
| `sk`                                     | S    | `{YYYY-MM}` do período de cobrança                           |
| `nfe` / `nfce` / `cte` / `mdfe` / `nfse` | N    | contadores                                                   |
| `companies`                              | N    | não vive aqui — contado por `organization_users`, ver abaixo |
| `ttl`                                    | N    | 400 dias                                                     |

**Campo novo em `organizations`: `owner_user_id`** (S). Desnormalizado na mesma transação que já
cria o membership OWNER (`services/organizations.go:205`), e mantido em sincronia pela troca de dono
(`services/memberships.go:225`). Evita uma query em `organization_users` a cada request para
descobrir de quem é a assinatura que governa a org.

Contagem de empresas (`quota_companies`) não usa contador: é `len(orgs onde owner_user_id = user)`,
lido no momento de criar organização. Barato e sem risco de divergir.

---

## 4. Fases

Cada fase termina com algo demonstrável. Ordem é dependência, não preferência.

---

### Fase 0 — Verificação e configuração (sem código)

Billing já roda em produção, então esta fase é **conferir o que existe**, não construir. Os três
primeiros itens provavelmente já estão feitos; o custo de confirmar é um comando cada, e o custo de
assumir é descobrir na Fase 2 com um 401 sem explicação.

- [x] ~~Aplicar `terraform/github` e `terraform/billing`~~ — feito, billing está no ar.
- [x] **0.2** Confirmar o SSM `email-from` = `billing@aoctech.app`, idêntico a `var.email_from`, com
  o domínio verificado no SES. A policy fixa `ses:FromAddress` na variável do Terraform; as duas
  são cópias de um mesmo fato e nada checa que concordam. Divergência só aparece quando o
  primeiro lembrete de dunning é recusado no envio — ou seja, no primeiro cliente que atrasa. - Já verificado
- [x] **0.3** Confirmar que `cmd/seed` já aplicou `api/tenants/ctech.json` em produção: a
  organização `ctech`, a credencial `ctech-dfe`, os 5 produtos, os 9 preços e o endpoint
  `whe_dfe`. `cmd/seed` é create-or-skip, então reaplicar é seguro e é a forma mais barata de - Aplicado
  verificar. Confirmar também `PORTAL_ORGANIZATION_ID` apontando para `ctech`.
- [x] **0.4** No ctech-account: cliente M2M `dfe-billing` (feito em 2026-08-16). Credenciais em
  `/ctech-dfe/{env}/billing/client-id` e `/client-secret`, sob o prefixo do **chamador** e não do
  chamado — o billing não sabe nada sobre elas. `BILLING_API_URL` vem de
  `/ctech-billing/{env}/internal-base-url`, publicado pelo
  `configure-service-url-parameters.sh` do ctech-cdk junto com os outros endpoints privados; o
  `IamStack` ganhou leitura em `/ctech-billing/{env}/*`. Escopos:
  `billing:customers:read`, `billing:customers:write`, `billing:subscriptions:read`,
  `billing:subscriptions:write`, `billing:invoices:read`, `billing:usage:write`,
  `billing:entitlements:read`, `billing:products:read`.
- [x] **0.5** Segredo do webhook `whe_dfe`. Lado billing: já configurado — o seed recebeu
  `WEBHOOK_SECRET_DFE` e guardou a versão selada na linha do endpoint. Lado DF-e: feito em
  2026-08-16 — SSM SecureString `/ctech-dfe/{env}/billing/webhook-secret`, lido pelo `start.sh`
  como `BILLING_WEBHOOK_SECRET`, mais `kms:Decrypt` (condicionado a `kms:ViaService = ssm`) no
  `SsmPolicy`, que faltava e sem o qual **nenhuma** SecureString seria legível. SSM e não Secrets
  Manager: é o mecanismo que o DF-e já usa para toda configuração sensível, e o `IamStack` já
  concede leitura em `/ctech-dfe/{env}/*`. O valor é escrito fora de banda (`DEPLOYMENT.md` §
  Out-of-band parameters) — o CloudFormation não sabe criar SecureString, e um segredo que o
  deploy possui é um segredo que o próximo deploy devolve ao valor de antes da rotação.
- [x] **0.6** URL do endpoint `whe_dfe` em `tenants/ctech.json` — corrigida em 2026-08-16 para
  `https://dfe.internal.aoctech.app/v1/internal/webhooks/billing` (o host estava invertido:
  `dfe.aoctech.internal.app`). Reaplicar o seed para que a linha em produção acompanhe — um
  endpoint apontando para um host que não resolve falha em silêncio até o contador de falhas
  chegar a 12 e desabilitar o endpoint sozinho.

> **Verificação da fase:** `cmd/seed` idempotente rodando duas vezes sem criar linha duplicada, e um
> `curl` autenticado com o client `ctech-dfe` recebendo 200 em `GET /v1.0/entitlements?customer_ref=x`
> (404 do cliente é sucesso — significa que auth, tenant e escopo passaram).

---

### Fase 1 — ctech-billing: fechar as seis lacunas

Todo o trabalho desta fase é em `ctech-billing`, e nenhum item aqui é específico do DF-e — poker e
qualquer produto futuro herdam.

- [x] **1.1 — Catálogo legível por M2M.** *(feito 2026-08-16)*
  `GET /v1.0/products` e `GET /v1.0/products/:id` no grupo `m2m`, sob `billing:products:read`.
  Os dois corpos saíram de `consoleHandlers` para `handlers`, e console e M2M chamam o mesmo
  código: uma cota que a integração lê no `metadata` do preço e a cota que o operador vê na tela
  são o mesmo número, ou são um chamado de suporte. `consoleLimit` virou `pageLimit` pela mesma
  razão — deixou de ser do console.
  *Teste:* `TestCatalogIsReadableByAnIntegration` — token M2M lista; token de sessão com o
  **mesmo** escopo recebe 403; token M2M sem `billing:products:read` recebe 403.

- [x] **1.2 — `GET /v1.0/entitlements` enriquecido.** *(feito 2026-08-16)*
  Acrescentados por assinatura: `plan`, `items[]` com `metadata` (as cotas), `cancel_at_period_end`
  e `open_invoice: {id, total_cents, due_date, checkout_url}`. Aditivo — nada removido nem
  renomeado; `price_id`, que existia e nunca era preenchido, passou a trazer o primeiro item.
  O `checkout_url` sai de `newInvoiceResponse`, não de uma segunda cópia da regra
  `Payable`+`links` — é assim que uma superfície acaba publicando um link que a outra não tem.
  Custo: uma leitura por item mais uma consulta de faturas por assinatura. Aceitável **por causa
  de quem chama** — um produto consulta um cliente por vez e cacheia; num endpoint de lista não
  seria, e deliberadamente não existe um.
  *Teste:* `TestEntitlementsCarryThePlanAndTheOpenInvoice`.

- [x] **1.3 — `INCOMPLETE` de verdade.** *(feito 2026-08-16)*
  `Subscribe` cria `SubscriptionIncomplete` quando `cycle.Timing == BillAdvance` **e**
  `firstPeriodCost(prices, items) > 0`. Free (`unit_amount: 0`) e sob demanda (arrears)
  continuam nascendo `ACTIVE` — não há nada a pagar, ou nada foi servido ainda.

  O status é decidido **antes** da escrita da linha, a partir dos preços, e não corrigido depois
  que a fatura existe: uma assinatura que fica `ACTIVE` por um instante é uma assinatura que uma
  consulta de entitlement enxerga. O custo é a mesma aritmética de `billing.FixedLine` num
  segundo lugar, e o que mantém os dois honestos é um teste
  (`TestSubscribingToAPaidPlanStartsIncomplete` confere o status escolhido contra a fatura
  produzida), não um comentário.

  **Duas consequências que a mudança forçou** — questões reais, não escopo extra:
    - Dunning deixou de escalar assinatura que nunca ativou. Os lembretes continuam (a fatura é
      devida e são eles que fazem pagar); o D+10 não faz nada, porque não há serviço a restringir;
      o fim da política cancela com `CauseActivationExpired`. Antes disto o passo final falhava
      com `ErrCauseNotAllowed` e deixava a fatura `OPEN` para sempre.
    - Cancelar no fim do período uma assinatura `INCOMPLETE` encerra na hora. Não há período pago
      a proteger, e o domínio não tem self-edge para agendar.

- [x] **1.4 — `invoice.paid` propaga para a assinatura.** *(feito 2026-08-16)*
  Em `services/collecting.go`, no mesmo ponto em que a fatura chega a `PAID` (`Confirm`),
  transicionar a assinatura com `CauseInvoicePaid`:
  `INCOMPLETE → ACTIVE` (ativação) e `PAST_DUE → ACTIVE` (`EventSubscriptionRecovered`).
  `ACTIVE → ACTIVE` não é aresta válida por essa causa, então a chamada é condicional ao status
  atual — e um `ErrInvalidTransition` aqui não pode derrubar a confirmação do pagamento: a
  fatura está paga, e reverter isso porque a assinatura não quis mudar de estado é perder
  dinheiro recebido. Logar e alarmar, nunca falhar.
  **Esta é a correção mais importante da fase**: sem ela, quem paga depois do D+10 fica
  bloqueado para sempre.
  *Teste:* integração — fatura vence, dunning marca `PAST_DUE` no D+10, webhook de pagamento
  chega no D+12, assinatura volta a `ACTIVE` com `EventSubscriptionRecovered` na auditoria.

- [x] ~~**1.5 — Expiração de `INCOMPLETE`** como job separado.~~ **Não construído, por decisão
  tomada durante a 1.3.** O dunning já percorre a vida inteira de uma primeira fatura não paga, e
  a percorre com lembretes que um job de expiração não enviaria. Dois jobs disputando o
  cancelamento da mesma linha seriam duas políticas para uma pergunta — e a que perdesse a corrida
  gravaria a causa errada na auditoria. A janela deixa de ser 7 dias e passa a ser o D+30 da
  política de dunning, o que não custa nada: `INCOMPLETE` não concede serviço nenhum enquanto
  isso, e a fatura segue pagável o tempo todo.

- [x] **1.6 — Troca de plano com pró-rata (D3).** *(feito 2026-08-16)*
  `POST /v1.0/subscriptions/:id/change` (M2M) e `POST /v1.0/console/subscriptions/:id/change`.
  `services.Subscriber.ChangePlan` reusa `resolveItemPrices` (uma recorrência, um timing, um
  `owner_key`), calcula com `proration.go`, e `repositories.ChangeItems` troca os itens na mesma
  transação da auditoria e do `subscription.updated`. `Anchor` e `PeriodIndex` não se movem.
  A guarda de status é do **domínio**, não uma checagem no serviço: a troca é uma self-edge, e
  só `ACTIVE` tem uma — `INCOMPLETE`, `PAST_DUE`, `PAUSED` e `CANCELED` são recusados sem código
  novo. `owner_key` é recomputado aqui, que é exatamente o lugar que o comentário de
  `Subscription.OwnerKey` nomeava.

  **Duas divergências do plano original, ambas deliberadas:**
    - **Downgrade não emite fatura.** O plano dizia "total zero ou negativo cai na regra de
      ADR 0019". Mas negativo não é zero: é dinheiro a devolver, e isso é um `CreditNote` — outro
      documento, que este serviço ainda não emite. Limitar o crédito para o total fechar em zero
      poria na fatura uma linha cuja fração declarada não é o valor cobrado, que é a única coisa
      que a regra das duas linhas existe para impedir. Então o plano troca, nada é cobrado, e o
      restante do período já pago é perdido. **É este o ramo que concede um crédito quando as
      notas de crédito existirem.**
    - **A linha de crédito de valor zero é omitida.** Sair do Free não imprime
      "Crédito proporcional — DF-e Free: R$ 0,00", que não explica nada e convida à pergunta que
      parece estar respondendo.

  Sem chave de geração na fatura da troca — aquela chave reivindica um *período*, e duas trocas
  no mesmo mês são dois documentos reais. Contra repetição vale o `Idempotency-Key` do HTTP, e a
  rota está atrás dele.
  *Testes:* `plan_change_test.go` — Pro→Ilimitado (duas linhas, valores conferidos contra
  `ProrateSwap`, período intacto), Free→Pro (uma linha), Pro→Free (nenhuma fatura, plano trocado),
  fixo→metered (nada cobrado, `Timing` acompanha), `INCOMPLETE` recusado.

- [x] **1.7 — NFS-e e usuários no catálogo (D4, D5).** *(feito 2026-08-16)*
  Em `api/tenants/ctech.json`: `quota_nfse` (`free: 3`, `pro: 1200`, `unlimited: -1`,
  `unlimited_internal: -1`), `quota_users` (`1`, `25`, `-1`, `-1`) e o preço novo
  `price_dfe_ondemand_nfse` (metered, arrears, `unit_amount: 5`). `PLANS` na landing atualizado,
  com NFS-e marcada **"(em breve)"** — o DF-e ainda não emite NFS-e (está no roadmap da própria
  página), e anunciar uma cota de um documento que o produto não emite é vender o que não existe.
  A cota entra agora mesmo assim porque preço é imutável: este é o último momento barato.

  **Corrigido junto, e é um bloqueio da Fase 2:** o `client_id` semeado era `ctech-dfe`, e o
  cliente M2M que o ctech-account de fato emitiu (item 0.4) é **`dfe-billing`**. O
  `ResolveTenant` procura a credencial por esse id exato, então toda chamada M2M do DF-e
  receberia 403 — "credencial não habilitada para o billing" — até a semente ser corrigida.

  O restante deste item **não é código, é operação**. Preço é imutável em billing: os preços
  fixos existentes **não podem ser editados**, e reaplicar a semente sozinha não muda nenhuma
  linha viva (`Apply` é criar-ou-pular, nunca atualizar). Duas saídas,
  e a escolha depende de um fato a verificar em produção (0.3) — se já existe alguma assinatura
  viva sobre os preços `dfe`:
  - **Nenhuma assinatura viva** (esperado, já que o DF-e ainda não integrou): apagar os 4 preços
  fixos e reaplicar o seed com as cotas corretas. Mais limpo.
  - **Alguma assinatura viva**: criar `price_dfe_{free,pro,unlimited,unlimited_internal}_monthly_v2`
  e arquivar os antigos. Quem já assinou fica no antigo, que é o comportamento correto.

      **Fazer isto antes do primeiro cliente pagante**, que é o último momento barato.
      A credencial tem o mesmo problema pelo mesmo motivo: `Apply` pula a que já existe, então
      `dfe-billing` precisa ser criada — e `ctech-dfe`, desativada — fora da semente.

> **Verificação da fase:** `make test-integration` verde, e um roteiro manual em ambiente de teste:
> criar cliente → assinar Pro (`INCOMPLETE`) → pagar via checkout → assinatura `ACTIVE` → deixar
> vencer a fatura seguinte → `PAST_DUE` → pagar → `ACTIVE`.

---

### Fase 2 — ctech-dfe/api: cliente de billing, modelo de conta, webhook

- [x] **2.1 — Cliente M2M.** *(feito 2026-08-16)*
  `api/internal/billingclient/client.go`, sobre **`gopkg.aoctech.app/api-commons/oauth2client`**
  — o plano dizia `go-common`, mas o módulo publicado (e o que o ctech-billing importa) é
  `api-commons`; o pacote é o mesmo. Nenhum segundo gerenciador de token.
  Métodos: `ListProducts`, `GetEntitlements`, `CreateCustomer`, `CreateSubscription`,
  `ChangeSubscription`, `CancelSubscription`, `ListInvoices`, `ReportUsage`.
  Erros do billing chegam como RFC 7807 e são remapeados para `problem.*` do DF-e — nunca
  repassar o corpo do billing cru para o cliente final.
  Config: `BILLING_API_URL`, `BILLING_CLIENT_ID`, `BILLING_CLIENT_SECRET` — SSM SecureString sob
  `/ctech-dfe/{env}/billing/*`, mesmo mecanismo do `webhook-secret` da 0.5, lidos no `start.sh`.
  Ausência de configuração → o produto roda em **modo sem cobrança** (todo mundo ilimitado),
  que é o que os ambientes de dev precisam, e é logado no boot de forma barulhenta.

- [x] **2.2 — Repositório e serviço de conta.** *(feito 2026-08-16)*
  `repositories/account_billing.go` + `services/billing.go`.
  `GetOrCreateCustomer(userID)`: lê `account_billing`; se não existe, chama
  `POST /v1.0/customers` com `external_ref = USER_{sub}`, `user_id = sub`, `name`/`email`
  vindos do `GetUserInfo` que `UserService` já faz, e grava a linha local.
  `Sync(userID)`: chama `GET /v1.0/entitlements?customer_ref=USER_{sub}` e reescreve o snapshot.
  Cache curto em Valkey (mesmo padrão de `middleware/rbac.go`, TTL 60 s), invalidado por webhook.

- [x] **2.3 — `owner_user_id` em `organizations`.** *(feito 2026-08-16)*
  Escrito na mesma `TransactWrite` que já cria o OWNER (`services/organizations.go:205`).
  Atualizado na transferência de propriedade (`services/memberships.go:225`).
  Backfill: **não foi escrito script.** A leitura repara sozinha — `BillingService.OwnerOf` cai
  para o membro OWNER quando o campo falta e grava o valor. É o mesmo *read-fallback self-heal*
  que a tabela de membros usou na própria migração, e evita um `cmd/` que alguém precisa lembrar
  de rodar em cada ambiente.
  **É um campo, não uma consulta** (D1a). A unicidade do OWNER já é invariante (D1b), então o valor
  não é ambíguo — mas continua sendo campo, porque derivá-lo custaria varrer os membros a cada
  decisão de cobrança, e um `GetItem` responde a mesma pergunta. Quem paga só muda na transferência
  explícita de propriedade, que é onde este campo é reescrito.

- [x] **2.4 — Endpoint de webhook.** *(feito 2026-08-16)*
  `POST /v1/internal/webhooks/billing`, sem auth de token, fora do resolvedor de org.
  - Verifica `X-Billing-Signature: v1=<hmac>` como **HMAC-SHA256 sobre `timestamp + "." + body`
  nos bytes crus**, com `X-Billing-Timestamp` dentro do material assinado (é o formato de
  `services.Sign` em billing — conferir contra o código, não contra a documentação).
  - Rejeita timestamp fora de ±5 min.
  - Idempotente em `X-Billing-Event-Id`.
  - O payload é **só um id e um tipo**. O handler relê a entidade em billing com a própria
  credencial e refaz o snapshot — nunca confia no corpo.
  - Eventos tratados: `subscription.*` → `Sync`; `invoice.paid`/`invoice.finalized`/
  `invoice.payment_failed` → `Sync` (a fatura aberta e a `checkout_url` fazem parte do snapshot).
  - Invalida o cache Valkey da conta.

      Duas decisões tomadas na implementação:
      - **Sem `BILLING_WEBHOOK_SECRET` a rota não é montada.** 404, não endpoint que confia no que
        chega — uma verificação de assinatura que não pode rodar não é uma verificação.
      - **A resolução assinatura → conta passa pelo billing**, não por um índice local
        (`GET /v1.0/subscriptions/:id` → `customer_id` → `GET /v1.0/customers/:id` →
        `external_ref`). Um índice seria mais uma coisa a manter em dia, e estaria faltando
        exatamente na corrida que o webhook mais perde: o `subscription.created` que ultrapassa a
        escrita deste serviço. Duas leituras num evento raro.

  *Teste:* `billing_webhook_test.go` (rota não montada sem segredo; corpo adulterado, segredo
  errado, sem assinatura, sem timestamp e replay de uma hora atrás, todos 401 — com o serviço
  `nil`, o que prova que a recusa acontece antes de qualquer trabalho); `webhook_test.go` confere
  a implementação contra o `Sign` do billing escrito à mão; `TestEventIsProcessedExactlyOnce`
  cobre a deduplicação sob concorrência real.

- [x] **2.5 — Rotas de conta.** *(feito 2026-08-16)*
  Todas sob `/v1.0/billing/*`, **sem** header de organização, agindo sempre sobre a conta do
  chamador — que é o que torna "só o proprietário cria/altera" uma propriedade estrutural e não
  uma checagem que alguém pode esquecer.

      | Rota | Nota |
      |---|---|
      | `GET /v1.0/billing/plans` | catálogo (proxy de `GET /v1.0/products`), cacheado 5 min |
      | `GET /v1.0/billing/subscription` | snapshot + uso do período + fatura aberta |
      | `POST /v1.0/billing/subscription` | escolher plano. Recusa se já existe assinatura viva |
      | `POST /v1.0/billing/subscription/change` | upgrade/downgrade (Fase 1.6) |
      | `POST /v1.0/billing/subscription/cancel` | `{at_period_end: bool}` |
      | `GET /v1.0/billing/invoices` | faturas da conta, com `checkout_url` quando pagáveis |

      E uma rota org-scoped, **somente leitura**, para o ADMIN:

      | `GET /v1.0/organizations/{pk}/plan` | plano e uso que governam esta organização (a assinatura do `owner_user_id`). Atrás de `RequireOwnerOrAdmin` — USER e VIEWER recebem 403, que é o "o resto não pode fazer nada" |

> **Verificação da fase:** um usuário novo faz login, `GET /v1.0/billing/subscription` devolve
> `{plan: null}`, `POST` com `price_dfe_free_monthly` cria cliente+assinatura em billing, e o
> webhook `subscription.created` chega e atualiza o snapshot sozinho.

---

### Fase 3 — ctech-dfe/api: cotas e bloqueio

- [x] **3.1 — Contador atômico de cota.** *(feito 2026-08-16)*
  **Sem tabela nova:** os contadores foram para a `account_billing`, como
  `pk = USAGE_{sub}#{period}`. O padrão de acesso é idêntico ao do snapshot — uma linha, por chave
  primária, nunca consultada nem varrida —, e uma segunda tabela seria mais uma coisa a criar,
  permissionar, prefixar e lembrar, sem propriedade ganha. Chaves diferentes ficam em partições
  diferentes, então não disputam.

  `ReserveUsage(userID, period, meter, limit)`:
  `UpdateItem` com `ADD #meter :one` e `ConditionExpression: attribute_not_exists(#meter) OR #meter < :limit`.
  Condição falha → `problem.QuotaExceeded` (402) com `meter`, `plan`, `quota_limit` e `quota_used`.
  `limit == -1` (ilimitado) pula a condição e só incrementa. `RefundUsage`: `ADD #meter :minusOne`,
  condicional a `#meter > 0`.

  **Bug encontrado pelo teste, não pela leitura:** `attribute_not_exists(#m) OR #m < :limit` libera
  a primeira emissão mesmo com `limit == 0` — o `quota_cte: 0` do Free teria virado um CT-e grátis.
  Limite zero usa `#m < :limit` sozinho, que é falso para atributo ausente. O ramo está em Go e não
  na expressão porque o DynamoDB não compara dois literais (`:zero < :limit` exige que um operando
  seja um caminho do documento).

  *Testes:* `TestQuotaIsClaimedAtomically` (10 requisições disputando 3 vagas — uma implementação
  read-then-write passa na versão sequencial e falha aqui), `TestQuotaOfZeroGrantsNothing`,
  `TestUnlimitedStillCounts`, `TestRefundGivesTheSlotBackButNeverGoesNegative`,
  `TestPeriodsAreCountedSeparately`.

- [x] **3.2 — Reserva na emissão.** *(feito 2026-08-16)*
  Em `services/nfes`, `nfces`, `mdfes`, `nfses` (e CT-e quando existir), reservar a cota **antes**
  de `TransactReserveAndCreate`.
  Reserva no pedido, não na autorização, porque é a reserva que **é** o controle: contar só
  autorizados torna o limite assíncrono e furável por concorrência.
  O preço disso é um documento rejeitado pela SEFAZ que consumiu cota — resolvido em 4.2.

- [x] **3.3 — Cotas de empresas e de usuários.** *(feito 2026-08-16)*
  Empresas: em `services/organizations.go`, antes de criar, contar organizações onde
  `owner_user_id == userID` e recusar acima de `quotas.companies`. Ilimitado (`-1`) passa direto.
  Usuários (D5): checado **nas duas pontas** — ao criar o convite (avisa o proprietário, que está
  ali e pode resolver) e ao aceitar (o plano pode ter encolhido entre uma coisa e outra). Contar os
  `user_id`
  **distintos** de todas as organizações daquela conta e recusar acima de `quotas.users`.
  Distintos, e por conta e não por organização: alguém que ajuda em duas empresas do mesmo
  cliente é uma pessoa, e cobrar duas vezes por ela é o tipo de conta que o cliente confere.
  O OWNER conta. Rebaixar de plano com mais membros do que o novo limite **não** expulsa
  ninguém — recusa o downgrade, nomeando quantos precisam sair antes.

- [x] **3.4 — Middleware de bloqueio (D2).** *(feito 2026-08-16)*
  `middleware/subscription.go`, montado **uma vez no grupo `/v1.0`** — não por rota, e não depois de
  um "resolvedor de org" que não existe: quem resolve a organização neste código é o próprio RBAC
  (`perm.check`). O gate resolve sozinho a partir do header/param, o que é barato, e roda antes do
  RBAC como o plano pedia.

  O ganho real é a **forma default-deny**: toda mutação é bloqueada a menos que o caminho esteja na
  lista de isenções, então uma rota criada amanhã já nasce protegida. A alternativa — adicionar o
  gate a cada rota de escrita — é uma lista que alguém eventualmente esquece de estender, e o
  esquecimento é silencioso.

  Custo de ter o gate antes do RBAC: um VIEWER numa conta inadimplente recebe 402 em vez de 403,
  ou seja, descobre que a assinatura da organização venceu. Aceitável — é o que a tela vai lhe
  dizer em seguida.
  Recusa com 402 e um `problem` que nomeia o motivo (`subscription_past_due`,
  `subscription_canceled`, `subscription_missing`) quando o status não é
  `ACTIVE`/`TRIALING`/`INCOMPLETE`... — não: **`INCOMPLETE` também bloqueia emissão**, porque é
  exatamente "assinou o Pro e nunca pagou".
  Estados que liberam: `ACTIVE`, `TRIALING`. Estados que bloqueiam: `INCOMPLETE`, `PAST_DUE`,
  `PAUSED`, `CANCELED`, e ausência de assinatura.
  Aplicado em: todas as rotas de emissão e toda escrita de cadastro.
  **Não** aplicado em: leituras, download de XML/DANFE, cancelamento e eventos de documentos já
  emitidos, manifestação, distribuição, e tudo sob `/v1.0/billing/*` e `/v1.0/auth/*`.
  A lista de exceções vive em **um** slice nomeado no pacote, não espalhada por handler.
  Modo sem cobrança (2.1) faz o middleware virar no-op — nem sequer lê o snapshot, então um
  ambiente de desenvolvimento não paga uma leitura de DynamoDB por requisição.

  Um erro ao ler o snapshot **libera** em vez de bloquear: o snapshot é durável e vem do DynamoDB,
  então um erro aqui é o DynamoDB indisponível — a requisição vai falhar em seguida de qualquer
  jeito, e falhá-la aqui rotularia a causa errada.

  *Testes:* `subscription_test.go` percorre as duas metades — o que passa (as isenções, cada uma com
  o motivo) e o que não passa, incluindo `substitute`, que **emite documento novo** e seria o caminho
  para contornar o gate inteiro.

- [x] **3.5 — Endpoint de uso.** *(feito 2026-08-16)*
  `GET /v1.0/billing/subscription` (2.5) passa a devolver
  `usage: {nfe: {used, limit}, nfce: {...}, ..., companies: {used, limit}}`.
  Fonte: contadores + `len(orgs do owner)` + cotas do snapshot.

> **Verificação da fase:** conta Free emite 3 NF-e com sucesso e a 4ª recebe 402 nomeando o medidor;
> uma conta com assinatura `PAST_DUE` recebe 402 em `POST /v1.0/products` mas 200 em
> `GET /v1.0/nfes/{key}/xml`.

---

### Fase 4 — uso medido *(feita 2026-08-16, exceto 4.3)*

> **Desvio de lugar, não de comportamento.** Nada disto foi para o worker. O acerto acontece no
> `ResultsConsumer` da API (`api/internal/consumer/results.go`), que já consome todo resultado
> terminal para invalidar cache e avisar o WebSocket. O worker é outro módulo Go: levaria uma
> segunda cópia do cliente de billing, do gerenciador de token OAuth2, da resolução
> organização→dono e dos contadores para alcançar exatamente as mesmas linhas — a duplicação que a
> regra DRY do repositório existe para impedir. `services.MeterForTable` traduz a tabela do
> documento no medidor; `billingActionFor` decide, só a partir da mensagem, o que ela deve.

- [x] **4.1 — Reportar uso na autorização.**
  `BillingService.ReportUsage` chama `POST /v1.0/usage` com `subscription_id`, o `price_id` do
  medidor, `quantity: 1` e **`idempotency_key` = a chave de acesso** (o `id_dps` na NFS-e). Plano
  fixo não reporta nada — o preço não carrega `meter` e a mensalidade já pagou a emissão.

- [x] **4.2 — Devolver cota em rejeição terminal.**
  `rejected` e `failed` devolvem a vaga; `retryable_failed` não, porque ainda está em voo.
  `RefundOnce` reivindica o marcador `refund:{meter}:{chave}` **antes** de decrementar: falhar entre
  os dois perde a devolução em vez de repeti-la.

- [x] **Confiabilidade sem varredura.** A mensagem não é apagada quando o acerto falha: a fila
  redelivera 3 vezes e dispara o alarme da DLQ de resultados, que já existia. Redrive é seguro
  porque os dois lados são idempotentes.

- [ ] **4.3 — Varredura de uso não reportado.** *Adiada, deliberadamente.*
  O que a DLQ **não** cobre é o `publishResult` do worker falhar — aí não existe mensagem nenhuma.
  Fechar isso pede marca `usage_reported_at` no documento (uma escrita a mais por emissão), um job
  agendado que a API ainda não tem, e uma varredura que não pode ser `Scan`. Hoje não há **nenhuma
  conta medida**: os dois clientes existentes vão para o unlimited. **Gatilho para construir:** o
  primeiro cliente sob demanda.

> **Verificação da fase:** `TestAuthorisedIssuanceIsReportedToBilling` (chave de acesso como
> `idempotency_key`, estável na redelivery), `TestAFixedPlanReportsNothing`,
> `TestARejectedDocumentGivesItsSlotBackExactlyOnce`, e `billingActionFor` cobrindo evento,
> distribuição, `retryable_failed` e tabela não declarada.

> **Ponta solta conhecida:** o marcador de devolução tem TTL de 7 dias e a DLQ de resultados retém
> 14. Um redrive depois do sétimo dia devolveria a vaga duas vezes — e num período que já virou,
> decrementando o contador do mês errado. Irrelevante hoje (o alarme dispara em um minuto), mas é
> onde olhar se um dia alguém redirecionar uma DLQ antiga.

---

### Fase 5 — ui: fluxo completo (usar a skill `/impeccable`) *(5.1–5.4 feitas 2026-08-16)*

Toda tela desta fase é desenhada com **`/impeccable`**, sob as regras já vigentes em
`ui/CLAUDE.md`: mobile-first a 375 px, alvos de toque ≥ 44 px, skeletons em carregamento,
ESLint zero erros e zero warnings, e **seletores em vez de texto livre** onde houver conjunto
fechado (preferência registrada do produto).
Tema: verde `#50ba95` (`THEME.md`) — o DF-e continua verde; o sienna é do portal de billing.

> **O onboarding virou camadas (decisão do dono do produto, 2026-08-16).**
> A 5.4 não é o fim: escolher o plano, cadastrar a empresa e configurar a
> numeração são só as três camadas obrigatórias. Depois delas o fluxo pergunta
> **quais documentos a empresa emite** e, conforme a resposta, oferece o catálogo
> de produtos (NF-e/NFC-e) ou o de serviços (NFS-e). CT-e/MDF-e ativam uma NF-e
> em modo recebimento, com numeração zerada, porque é por ela que chegam as notas
> da carga. O progresso é **derivado** do que a API já sabe — não existe tabela de
> onboarding. Detalhe completo em `DOCS.md § 5 · Onboarding em camadas`.
>
> A camada 7 (guia de todas as features) fica fora: é um projeto próprio e o
> escopo desta fase já é o caminho até a primeira emissão.

Fluxo alvo, na ordem que o usuário vive:

```
login (ctech-account) → termos DF-e → [SEM ASSINATURA] → /onboarding/plano
   → Free / Sob demanda ──────────────────────────────────▶ /onboarding/empresa
   → Pro ──▶ redirect checkout billing ──▶ retorno ──▶ /onboarding/empresa
                                            (webhook já ativou)
```

- [x] **5.1 — Gate de assinatura.**
  Componente irmão de `terms-addendum-gate.tsx`, montado depois dele em `ProtectedRoute`.
  Sem assinatura → redireciona para `/onboarding/plano`. Não é um modal: escolher plano é uma
  decisão com comparação, não uma confirmação.

- [x] **5.2 — `/onboarding/plano`.**
  Cards de plano montados a partir de `GET /v1.0/billing/plans` — **não** da constante `PLANS`
  hardcoded. A landing pública (`app/page.tsx`) continua estática por ser pública e pré-auth,
  mas passa a ser gerada do mesmo catálogo em tempo de build, ou ganha um comentário apontando
  para o seed como fonte da verdade. Duas listas de preço que ninguém compara é como o site
  anuncia R$ 350 e a fatura cobra R$ 400.
  Free e Sob demanda: `POST /v1.0/billing/subscription` e segue direto.
  Pro: `POST`, depois `window.location = checkout_url` da fatura.

- [x] **5.3 — Retorno do checkout.**
  `/onboarding/retorno`. A página de checkout de billing já tem SSE de liquidação; aqui basta
  um poll curto de `GET /v1.0/billing/subscription` até `status === "ACTIVE"`, com um teto de
  ~60 s e um caminho de saída honesto ("o PIX pode levar alguns minutos; avisamos por e-mail")
  — nunca uma tela que prende o usuário, que foi exatamente o bug encontrado no checkout de
  billing (`PLAN.md`, X1).

- [x] **5.4 — `/onboarding/empresa`.**
  Reusa o formulário de `app/organizations/new` (certificado A1 + KYC) dentro do casco de
  onboarding, com barra de passos. Não duplicar o formulário.

- [x] **5.4b — `/onboarding/documentos`, `/produtos`, `/servicos`, `/pronto`.**
  As camadas 3 a 6. A tela de documentos reusa `FiscalConfigForm` numa fila, um tipo por vez,
  em vez de um formulário novo — o formulário existente já sabe CSC, provedor NFS-e e código
  IBGE. `SetupChecklist` substituiu os três passos mortos do dashboard e some quando acaba.

> **Duas falhas do seed encontradas ao ligar o catálogo** (corrigidas em
> `ctech-billing/api/tenants/ctech.json`, ainda não aplicadas em produção):
> 1. **O plano sob demanda não concedia cota nenhuma.** Nenhum preço tinha
>    `quota_*`, e `Quota()` responde "não concedido" para chave ausente — toda
>    emissão seria recusada com "seu plano não inclui a emissão de NFE". Cada
>    preço medido agora declara `quota_<meter>: "-1"`.
> 2. **`meter: "company"` no singular**, enquanto a cota e `services.MeterCompanies`
>    dizem `companies`. Dormente hoje (ninguém reporta uso de empresas), mas é um
>    preço que nunca seria encontrado. Corrigido para `companies`.
>
> Também marcado `visibility: "internal"` no preço ilimitado de valor zero: sem
> isso o seletor ofereceria um plano ilimitado grátis a qualquer visitante.
>
> **Falta no billing:** `SuccessURL` existe em `domain/billing/payment.go` e não é
> usado em lugar nenhum — não há retorno do checkout. O contorno é o portão tratar
> `INCOMPLETE`; o conserto de verdade é o checkout redirecionar ao liquidar.

- [x] **5.5 — `/assinatura`.**
  Plano atual, status, período, `cancel_at_period_end`, e as barras de uso por medidor
  (`used/limit`, "ilimitado" quando `-1`). Fatura em aberto com botão de pagamento.
  Histórico de faturas.
  **Visibilidade por papel:** OWNER da conta vê tudo e age; ADMIN de uma organização vê o plano e
  o uso que governam aquela org, sem nenhum botão de ação (`GET /v1.0/organizations/{pk}/plan`);
  USER e VIEWER não veem o item no menu.

- [x] **5.6 — Upgrade.**
  A partir de `/assinatura` e a partir do erro 402 de cota. O 402 já carrega medidor, limite e
  plano sugerido (3.1), então o diálogo mostra o custo pró-rata real antes de confirmar —
  pedir confirmação de uma cobrança sem dizer o valor é como se descobre o valor na fatura.

- [x] **5.7 — Cancelamento.**
  Em `/assinatura`, no fim do período por padrão. Cancelamento imediato só com confirmação
  explícita que diz o que se perde e quando.

- [x] **5.8 — Banners de bloqueio.**
  `PAST_DUE`: faixa persistente com o valor, o vencimento e o botão de pagar.
  `CANCELED`/`INCOMPLETE`: estado vazio nas telas de emissão explicando o que fazer, nunca um
  erro genérico.
  Os 402 da API precisam de mensagem específica por `problem.type` — um "erro interno" aqui é
  um cliente que liga para o suporte por uma fatura de R$ 350.

- [x] **5.9 — Modo mock.**
  Cenários no `lib/mock` (padrão já existente): sem assinatura, Free no limite, Pro `PAST_DUE`,
  sob demanda com uso, checkout pendente. Metade destes é impossível de produzir contra um
  backend real em tempo hábil.

> **Verificação da fase:** conta nova, do zero até emitir a primeira NF-e, sem tocar em nenhum
> console — nos planos Free e Pro. Testado a 375 px.

> **5.5–5.9 feitas 2026-08-16.** `/assinatura` com uso por medidor, fatura em aberto e histórico
> por mês; `ChangePlanDialog` e `CancelSubscriptionDialog`; `SubscriptionBanner` no `RootLayout` e
> `SubscriptionBlocked` dentro do `RequireFiscalConfig`; `lib/billing/notice.ts` como único tradutor
> de 402 em frase e botão; seis cenários de assinatura no `lib/mock` com seletor no `MockDevPanel`.
>
> **Desvio em 5.6:** o custo pró-rata exato não é exibido antes de confirmar. O ctech-billing
> calcula o rateio na própria mudança e não publica endpoint de prévia — qualquer valor renderizado
> aqui seria a aritmética da UI, não a da fatura. O diálogo diz a regra e a nova mensalidade, e o
> valor real aparece na tela de pagamento antes de qualquer cobrança. Fechar isso de verdade exige
> um `POST /subscriptions/{id}/change/preview` no billing.
>
> **Defeito de mock corrigido de passagem:** `meFixture.organizations[0].role` era `'owner'`
> minúsculo, contra o `RoleName` `'OWNER'` — todo item de menu com `roles` ficava invisível no
> modo mock, inclusive o novo.

---

### Fase 6 — Migração dos clientes atuais

Dois clientes: a conta do tio e a conta de teste. Ambos vão para
`price_dfe_unlimited_internal_monthly` (R$ 0, cotas `-1`), que já existe no seed exatamente para isso.

> **Pré-requisitos concluídos (2026-08-16, confirmados pelo Artur):** reseed do `ctech.json`
> corrigido aplicado, `/ctech-dfe/prod/billing/webhook-secret` gravado, credencial `dfe-billing`
> criada.

- [x] **6.1** Backfill de `owner_user_id` — **não se aplica**: `BillingService.OwnerOf` cai para o
  membro OWNER quando o campo falta e grava o valor (2.3). A primeira leitura de cada organização
  faz a migração; não há script para lembrar de rodar.

- [ ] **6.2** Assinar o preço interno nas duas contas.
  **Pelo próprio endpoint do DF-e, com o token do dono** — não por escrita M2M direta no billing e
  muito menos por linha no DynamoDB. `BillingService.Choose` não valida `visibility`, então o preço
  interno é aceito normalmente, e o caminho é o mesmo de qualquer cliente pagante:
  `GetOrCreateCustomer` (cria o `Customer` com `external_ref = USER_{sub}`, nome e e-mail lidos do
  ctech-account) → `CreateSubscription` → `Sync`. Criar o `Customer` por fora replicaria nome e
  e-mail à mão, e **um customer não é editável depois** por nenhuma rota que este serviço tenha.

  Para cada uma das duas contas, logado como ela, com o `Authorization` copiado de qualquer
  requisição da UI (o token vive só em memória, por isso vem do devtools):

  ```bash
  curl -sS -X POST https://dfe-api.aoctech.app/v1.0/billing/subscription \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/json' \
    -d '{"price_ids":["price_dfe_unlimited_internal_monthly"]}' | jq
  ```

  Antes disso, confirmar que o reseed pegou — o sob demanda precisa ter `quota_*` em cada preço
  medido, senão toda emissão é recusada:

  ```bash
  curl -sS https://dfe-api.aoctech.app/v1.0/billing/plans -H "Authorization: Bearer $TOKEN" \
    | jq '.data[] | {name, prices: [.prices[] | {id, metadata}]}'
  ```

- [ ] **6.3** Verificar: `GET /v1.0/billing/subscription` responde `grants_service: true`,
  `plan: "unlimited"`, `quotas` com `-1`. Nenhuma fatura gerada (total zero liquida na emissão,
  ADR 0019) e nenhum lembrete de dunning (fatura de total zero não entra na fila).
  Na UI: `/assinatura` mostra "Ilimitado" e nenhuma faixa de bloqueio.
  *Nota:* o preço interno é `visibility: internal`, então o plano atual **não** aparece no
  `ChangePlanDialog` — a conta interna vê o plano no topo da tela, mas não marcado como "Plano
  atual" na lista. É consequência de esconder o preço, não defeito.

- [ ] **6.4** **Não existe chave para "ligar depois"** (corrigido 2026-08-16, ao reler o código).
  `RequireActiveSubscription` já está montado no grupo `/v1.0` inteiro e se desliga sozinho por
  `billing == nil || !billing.Enabled()`, e `Enabled()` é só "o cliente foi construído"
  (`billingclient/client.go:110`). Ou seja: o portão liga no instante em que o deploy de produção
  levar a configuração de billing — que é exatamente o que o 6.2 precisa para existir. A ordem
  pedida aqui ("subir o bloqueio só depois de migrar") não é executável como escrita.

  O que fica, então, é **encurtar a janela**, não evitá-la:

  1. Deploy da API com billing configurado.
  2. Rodar o 6.2 nas duas contas **em seguida** — as rotas `/v1.0/billing/*` são isentas do portão
     (`exemptPrefixes`), então dá para assinar mesmo estando bloqueado. É essa isenção que impede o
     impasse "não posso pagar porque não paguei".
  3. Rodar o 6.2 e o 6.3 antes de qualquer emissão.

  Durante a janela as duas contas recebem 402 em emissão e em cadastro. Leitura, download de
  XML/DANFE, cancelamento e eventos de documento já emitido, manifestação e distribuição continuam
  liberados pelos carve-outs — ou seja, nada com prazo legal quebra na janela.

  A alternativa que fecharia a janela — criar `Customer` e assinatura por M2M **antes** do deploy —
  foi descartada: replicaria nome e e-mail à mão num `Customer` que não é editável depois, para
  poupar alguns minutos de bloqueio em duas contas internas de baixo volume.

---

### Fase 7 — Documentação e verificação final

Obrigatório pela política do repo, não é formalidade.

- [ ] **7.1** `DOCS.md` — rotas novas `/v1.0/billing/*`, `/v1/internal/webhooks/billing`,
  `GET /v1.0/organizations/{pk}/plan`.
- [ ] **7.2** `DynamoDB-Tables.md` — `account_billing`, `account_usage_counters`, campo
  `owner_user_id` em `organizations`. A contagem de tabelas em `OVERVIEW.md` sai de 35 para 37.
- [ ] **7.3** `INTEGRATION.md` — contrato UI↔API do fluxo de assinatura, incluindo os `problem.type`
  dos 402.
- [ ] **7.4** `CONDUCT.md` — a regra do carve-out (o que nunca é bloqueado e por quê) e a nota sobre
  `IsEntitled()` não servir como gate.
- [ ] **7.5** `OVERVIEW.md` — ctech-billing no diagrama de componentes.
- [ ] **7.6** `ctech-billing/PLAN.md` — Fase 5 (primeira integração real) marcada, com o que foi
  encontrado.
- [ ] **7.7** `cdk/` — as duas tabelas novas, as variáveis de ambiente e o segredo do webhook.

---

## 5. Riscos e o que fazer com eles

| Risco                                                      | Por que importa                                                                                                                                                                       | Mitigação                                                                                                                                                                                                                                 |
|------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Billing em produção, DF-e é o primeiro consumidor real** | Nenhuma das lacunas da § 1 apareceu ainda porque ninguém exerceu o caminho completo. A 1.4 (`invoice.paid` não propaga) só dói depois do primeiro D+10.                               | Fase 1 antes de qualquer cliente pagante entrar. As mudanças da Fase 1 são aditivas ou correções de arestas mortas — nenhuma altera comportamento de que a produção atual dependa.                                                        |
| ~~**Cobrança é live-only**~~                               | ADR 0004: a máquina de compra da wallet não tem modo de teste, então billing recusa cobrança em test mode. Checkout Pro não testa ponta a ponta em sandbox.                           | **Aceito pelo dono do produto (2026-08-16):** o dinheiro cai na conta dele e o Inter não cobra taxa por cobrança PIX, então testar com cobranças reais de valor baixo não tem custo. Free e sob demanda continuam testáveis em test mode. |
| **Preço é imutável em billing**                            | Corrigir cota ou valor depois de existir assinatura viva exige preço novo, e quem já assinou fica no antigo — que é o comportamento correto, e é caro se o erro for descoberto tarde. | Fechar cotas e preços na Fase 1.7, com o seed reaplicado limpo, **antes** do primeiro cliente.                                                                                                                                            |
| **Um único inquilino esconde bugs de tenant**              | Tudo do DF-e vive na organização `ctech` de billing. Erros de escopo por `owner_key` só aparecem quando poker também estiver lá.                                                      | Os testes de roteamento de webhook em billing já cobrem dois serviços em um tenant. Não relaxar isso.                                                                                                                                     |
| **Contador de cota e faturamento podem divergir**          | O DF-e conta o consumo, billing cobra por ele. Duas verdades.                                                                                                                         | Só nos planos metered os dois números existem ao mesmo tempo — e nesses a cota é ilimitada, então o contador é informativo e o cobrado é o que billing recebeu. Divergência é visível, não silenciosa.                                    |
| **`quota_companies` vs. filial**                           | No DF-e uma filial é uma organização separada (chave `CNPJ_{cnpj}`). Uma empresa com matriz e três filiais consome 4 do limite de 1 do Free.                                          | Decisão de produto pendente, **não bloqueia o MVP**: no Free (1 empresa) o caso não aparece, e no Pro (10) é folgado. Registrar e revisitar antes de anunciar o plano para grupos com muitas filiais.                                     |

## 6. Fora de escopo (explicitamente)

- Console de billing (C1–C9) — o DF-e não precisa dele; operação sai pela API.
- NFS-e automática sobre as próprias faturas do CTech (sugestão § 9.1 do `OVERVIEW.md` de billing).
  É a Fase 5 do plano de billing, e depende de regras de ISS que são um projeto próprio.
- Plano híbrido base+excedente.
- Dunning configurável por plano.
- PDF de fatura.
- Trial. Nenhum plano tem `trial_days` hoje, e o Free já cumpre o papel.
