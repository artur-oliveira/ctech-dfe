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
| D1 | **Assinatura pertence à conta (usuário ctech-account), não à organização.** | `Customer.UserID` = subject do ctech-account; `Customer.ExternalRef` = `USER_{sub}`. `quota_companies` limita quantas organizações DF-e aquele usuário pode criar. Uma organização é governada pela assinatura do seu **OWNER**. |
| D2 | **`PAST_DUE`/`CANCELED` bloqueia emissão E todos os cadastros.**            | Gate em toda escrita org-scoped. Carve-outs abaixo.                                                                                                                                                                              |
| D3 | **Upgrade via nova rota em billing, com pró-rata.**                         | `POST /v1.0/subscriptions/:id/change` em ctech-billing, reusando `proration.go`. Duas linhas separadas na fatura (crédito do antigo, cobrança do novo), nunca uma linha líquida.                                                 |
| D4 | **NFS-e ganha cota e medidor agora.**                                       | `quota_nfse` nos preços fixos, `price_dfe_ondemand_nfse` no sob demanda.                                                                                                                                                         |

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
- [ ] **0.4** No ctech-account: cliente M2M `ctech-dfe` com os escopos
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

- [ ] **1.1 — Catálogo legível por M2M.**
  Montar em `internal/api/v1/router.go`, no grupo `m2m`:
  `GET /v1.0/products` e `GET /v1.0/products/:id` sob `billing:products:read`.
  Reusar `consoleHandlers.listProducts`/`getProduct` — a diferença é só o resolvedor de tenant,
  então extrair os dois corpos para `handlers` e deixar console e M2M chamando o mesmo código.
  Resposta inclui os `prices` com `metadata` (é de onde saem as cotas).
  *Teste:* contrato — um token M2M lista o catálogo; um token de sessão é recusado.

- [ ] **1.2 — `GET /v1.0/entitlements` enriquecido.**
  Acrescentar por assinatura: `plan` (de `Price.Metadata["plan"]` do primeiro item),
  `items: [{price_id, product_id, type, unit_amount, metadata}]`, `cancel_at_period_end`,
  e `open_invoice: {id, total_cents, due_date, checkout_url}` quando houver fatura `OPEN`.
  Campos aditivos apenas — nada removido, nada renomeado.
  *Teste:* integração — assinatura Pro com fatura aberta devolve `checkout_url` assinado e válido.

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

- [ ] **1.6 — Troca de plano com pró-rata (D3).**
  `POST /v1.0/subscriptions/:id/change` (M2M, `billing:subscriptions:write`, idempotente) e o
  espelho no console. Corpo: `{items: [{price_id, quantity}], effective: "now"}`.
  Serviço novo `services.Subscriber.ChangePlan`:
  1. Valida o conjunto novo pelas mesmas três regras de `resolveItemPrices` (uma recorrência,
  um timing, um `owner_key`) — reusar a função, não copiar.
  2. Calcula, com `proration.go`, o crédito do não consumido do preço antigo e a cobrança
  pró-rata do novo, para o restante do período corrente.
  3. Emite **uma** fatura com **duas linhas separadas** (crédito e cobrança), nunca uma linha
  líquida ambígua. Total zero ou negativo cai na regra de ADR 0019 (liquidada na emissão).
  4. Substitui os `SubscriptionItem`s, mantém `Anchor` e `PeriodIndex` — trocar de plano não
  muda o dia do vencimento.
  5. Transição `ACTIVE → ACTIVE` por `CauseManual`/`CauseCustomer` (aresta já existe),
  emitindo `EventSubscriptionUpdated`.
  Preços metered nunca são pró-rateados (uso é uso). Trocar de um plano fixo para sob demanda
  credita o fixo e não cobra nada adiantado; o inverso cobra o fixo pró-rata e a fatura de uso
  do período fechado chega normalmente pelo sweep.
  *Teste:* Free→Pro no dia 10 de um mês de 30 dias cobra 20/30 de R$ 350; Pro→Ilimitado credita
  o restante do Pro; a soma crédito+cobrança bate com a propriedade já testada em `proration_test.go`.

- [ ] **1.7 — NFS-e no catálogo (D4).**
  Em `api/tenants/ctech.json`: `quota_nfse` nos quatro preços fixos
  (`free: 3`, `pro: 1200`, `unlimited: -1`, `unlimited_internal: -1`) e um preço novo
  `price_dfe_ondemand_nfse` (metered, arrears, `unit_amount: 5` — mesma faixa da NF-e).
  Preço é imutável em billing: os preços fixos existentes **não podem ser editados**. Duas saídas,
  e a escolha depende de um fato a verificar em produção (0.3) — se já existe alguma assinatura
  viva sobre os preços `dfe`:
  - **Nenhuma assinatura viva** (esperado, já que o DF-e ainda não integrou): apagar os 4 preços
  fixos e reaplicar o seed com as cotas corretas. Mais limpo.
  - **Alguma assinatura viva**: criar `price_dfe_{free,pro,unlimited,unlimited_internal}_monthly_v2`
  e arquivar os antigos. Quem já assinou fica no antigo, que é o comportamento correto.

      **Fazer isto antes do primeiro cliente pagante**, que é o último momento barato.
      Atualizar `ui/src/app/page.tsx` (`PLANS`) para incluir NFS-e nas cotas anunciadas.

> **Verificação da fase:** `make test-integration` verde, e um roteiro manual em ambiente de teste:
> criar cliente → assinar Pro (`INCOMPLETE`) → pagar via checkout → assinatura `ACTIVE` → deixar
> vencer a fatura seguinte → `PAST_DUE` → pagar → `ACTIVE`.

---

### Fase 2 — ctech-dfe/api: cliente de billing, modelo de conta, webhook

- [ ] **2.1 — Cliente M2M.**
  `api/internal/billingclient/client.go`, sobre
  **`gopkg.aoctech.app/go-common/oauth2client`** (`ctech-go-common/oauth2client/client.go` —
  já faz cache do token). Não escrever um segundo gerenciador de token.
  Métodos: `ListProducts`, `GetEntitlements`, `CreateCustomer`, `CreateSubscription`,
  `ChangeSubscription`, `CancelSubscription`, `ListInvoices`, `ReportUsage`.
  Erros do billing chegam como RFC 7807 e são remapeados para `problem.*` do DF-e — nunca
  repassar o corpo do billing cru para o cliente final.
  Config: `BILLING_API_URL`, `BILLING_CLIENT_ID`, `BILLING_CLIENT_SECRET` — SSM SecureString sob
  `/ctech-dfe/{env}/billing/*`, mesmo mecanismo do `webhook-secret` da 0.5, lidos no `start.sh`.
  Ausência de configuração → o produto roda em **modo sem cobrança** (todo mundo ilimitado),
  que é o que os ambientes de dev precisam, e é logado no boot de forma barulhenta.

- [ ] **2.2 — Repositório e serviço de conta.**
  `repositories/account_billing.go` + `services/billing.go`.
  `GetOrCreateCustomer(userID)`: lê `account_billing`; se não existe, chama
  `POST /v1.0/customers` com `external_ref = USER_{sub}`, `user_id = sub`, `name`/`email`
  vindos do `GetUserInfo` que `UserService` já faz, e grava a linha local.
  `Sync(userID)`: chama `GET /v1.0/entitlements?customer_ref=USER_{sub}` e reescreve o snapshot.
  Cache curto em Valkey (mesmo padrão de `middleware/rbac.go`, TTL 60 s), invalidado por webhook.

- [ ] **2.3 — `owner_user_id` em `organizations`.**
  Escrito na mesma `TransactWrite` que já cria o OWNER (`services/organizations.go:205`).
  Atualizado na transferência de propriedade (`services/memberships.go:225`).
  Backfill: as organizações existentes são duas; um `cmd/` de uso único ou um script direto.

- [ ] **2.4 — Endpoint de webhook.**
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
  *Teste:* integração — assinatura forjada é 401; replay do mesmo `event_id` é 200 sem segunda escrita.

- [ ] **2.5 — Rotas de conta.**
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

- [ ] **3.1 — Contador atômico de cota.**
  `repositories/usage_counters.go`. `Reserve(userID, period, meter, limit)`:
  `UpdateItem` com `ADD #meter :one` e `ConditionExpression: attribute_not_exists(#meter) OR #meter < :limit`.
  Condição falha → `problem.PaymentRequired` (402) com o medidor, o limite e o plano sugerido no
  corpo, para a UI conseguir montar o convite ao upgrade sem uma segunda chamada.
  `limit == -1` (ilimitado) pula a condição e só incrementa — o número ainda importa para a tela de uso.
  `Refund(userID, period, meter)`: `ADD #meter :minus_one`, condicional a `#meter > 0`.

- [ ] **3.2 — Reserva na emissão.**
  Em `services/nfes`, `nfces`, `mdfes`, `nfses` (e CT-e quando existir), reservar a cota **antes**
  de `TransactReserveAndCreate`.
  Reserva no pedido, não na autorização, porque é a reserva que **é** o controle: contar só
  autorizados torna o limite assíncrono e furável por concorrência.
  O preço disso é um documento rejeitado pela SEFAZ que consumiu cota — resolvido em 4.2.

- [ ] **3.3 — Cota de empresas.**
  Em `services/organizations.go`, antes de criar: contar organizações onde
  `owner_user_id == userID` e recusar acima de `quotas.companies`. Ilimitado (`-1`) passa direto.

- [ ] **3.4 — Middleware de bloqueio (D2).**
  `middleware/subscription.go`, montado **depois** de `auth` e do resolvedor de org, **antes** do
  RBAC. Lê o snapshot da conta dona da org (cache Valkey 60 s).
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
  Modo sem cobrança (2.1) faz o middleware virar no-op.

- [ ] **3.5 — Endpoint de uso.**
  `GET /v1.0/billing/subscription` (2.5) passa a devolver
  `usage: {nfe: {used, limit}, nfce: {...}, ..., companies: {used, limit}}`.
  Fonte: contadores + `len(orgs do owner)` + cotas do snapshot.

> **Verificação da fase:** conta Free emite 3 NF-e com sucesso e a 4ª recebe 402 nomeando o medidor;
> uma conta com assinatura `PAST_DUE` recebe 402 em `POST /v1.0/products` mas 200 em
> `GET /v1.0/nfes/{key}/xml`.

---

### Fase 4 — worker: uso medido

- [ ] **4.1 — Reportar uso na autorização.**
  Em `worker/internal/service/dfe.go`, `handleSefazResponse`, no ramo terminal autorizado (e o
  equivalente em `nfse.go`): se o snapshot da conta tem `meters[docType]`, chamar
  `POST /v1.0/usage` com `subscription_id`, `price_id` do medidor, `quantity: 1` e
  **`idempotency_key` = a chave de acesso** (ou `id_dps` para NFS-e). Retentativa do SQS não
  duplica: billing devolve `{recorded: true, duplicate: true}` e o worker trata isso como sucesso.
  Falha no report **não** falha a emissão — o documento está autorizado na SEFAZ e desfazer isso
  é impossível. Loga, alarma, e a reconciliação do 4.3 recolhe.

- [ ] **4.2 — Devolver cota em rejeição terminal.**
  No ramo de rejeição terminal, `Refund` no contador (3.1). Uma NF-e rejeitada por erro de
  cadastro não pode consumir 1 das 3 do plano Free.

- [ ] **4.3 — Varredura de uso não reportado.**
  Job diário que varre documentos autorizados sem marca `usage_reported` e reenvia.
  Mesmo princípio da reconciliação de billing: webhook e chamada síncrona nunca são o único sinal.
  Atributo `usage_reported_at` no documento é a marca.

> **Verificação da fase:** conta sob demanda emite 5 NF-e; o sweep de billing no fim do período gera
> uma fatura com 5 unidades a R$ 0,05. Reenfileirar a mesma mensagem SQS não muda o total.

---

### Fase 5 — ui: fluxo completo (usar a skill `/impeccable`)

Toda tela desta fase é desenhada com **`/impeccable`**, sob as regras já vigentes em
`ui/CLAUDE.md`: mobile-first a 375 px, alvos de toque ≥ 44 px, skeletons em carregamento,
ESLint zero erros e zero warnings, e **seletores em vez de texto livre** onde houver conjunto
fechado (preferência registrada do produto).
Tema: verde `#50ba95` (`THEME.md`) — o DF-e continua verde; o sienna é do portal de billing.

Fluxo alvo, na ordem que o usuário vive:

```
login (ctech-account) → termos DF-e → [SEM ASSINATURA] → /onboarding/plano
   → Free / Sob demanda ──────────────────────────────────▶ /onboarding/empresa
   → Pro ──▶ redirect checkout billing ──▶ retorno ──▶ /onboarding/empresa
                                            (webhook já ativou)
```

- [ ] **5.1 — Gate de assinatura.**
  Componente irmão de `terms-addendum-gate.tsx`, montado depois dele em `ProtectedRoute`.
  Sem assinatura → redireciona para `/onboarding/plano`. Não é um modal: escolher plano é uma
  decisão com comparação, não uma confirmação.

- [ ] **5.2 — `/onboarding/plano`.**
  Cards de plano montados a partir de `GET /v1.0/billing/plans` — **não** da constante `PLANS`
  hardcoded. A landing pública (`app/page.tsx`) continua estática por ser pública e pré-auth,
  mas passa a ser gerada do mesmo catálogo em tempo de build, ou ganha um comentário apontando
  para o seed como fonte da verdade. Duas listas de preço que ninguém compara é como o site
  anuncia R$ 350 e a fatura cobra R$ 400.
  Free e Sob demanda: `POST /v1.0/billing/subscription` e segue direto.
  Pro: `POST`, depois `window.location = checkout_url` da fatura.

- [ ] **5.3 — Retorno do checkout.**
  `/onboarding/retorno`. A página de checkout de billing já tem SSE de liquidação; aqui basta
  um poll curto de `GET /v1.0/billing/subscription` até `status === "ACTIVE"`, com um teto de
  ~60 s e um caminho de saída honesto ("o PIX pode levar alguns minutos; avisamos por e-mail")
  — nunca uma tela que prende o usuário, que foi exatamente o bug encontrado no checkout de
  billing (`PLAN.md`, X1).

- [ ] **5.4 — `/onboarding/empresa`.**
  Reusa o formulário de `app/organizations/new` (certificado A1 + KYC) dentro do casco de
  onboarding, com barra de passos. Não duplicar o formulário.

- [ ] **5.5 — `/assinatura`.**
  Plano atual, status, período, `cancel_at_period_end`, e as barras de uso por medidor
  (`used/limit`, "ilimitado" quando `-1`). Fatura em aberto com botão de pagamento.
  Histórico de faturas.
  **Visibilidade por papel:** OWNER da conta vê tudo e age; ADMIN de uma organização vê o plano e
  o uso que governam aquela org, sem nenhum botão de ação (`GET /v1.0/organizations/{pk}/plan`);
  USER e VIEWER não veem o item no menu.

- [ ] **5.6 — Upgrade.**
  A partir de `/assinatura` e a partir do erro 402 de cota. O 402 já carrega medidor, limite e
  plano sugerido (3.1), então o diálogo mostra o custo pró-rata real antes de confirmar —
  pedir confirmação de uma cobrança sem dizer o valor é como se descobre o valor na fatura.

- [ ] **5.7 — Cancelamento.**
  Em `/assinatura`, no fim do período por padrão. Cancelamento imediato só com confirmação
  explícita que diz o que se perde e quando.

- [ ] **5.8 — Banners de bloqueio.**
  `PAST_DUE`: faixa persistente com o valor, o vencimento e o botão de pagar.
  `CANCELED`/`INCOMPLETE`: estado vazio nas telas de emissão explicando o que fazer, nunca um
  erro genérico.
  Os 402 da API precisam de mensagem específica por `problem.type` — um "erro interno" aqui é
  um cliente que liga para o suporte por uma fatura de R$ 350.

- [ ] **5.9 — Modo mock.**
  Cenários no `lib/mock` (padrão já existente): sem assinatura, Free no limite, Pro `PAST_DUE`,
  sob demanda com uso, checkout pendente. Metade destes é impossível de produzir contra um
  backend real em tempo hábil.

> **Verificação da fase:** conta nova, do zero até emitir a primeira NF-e, sem tocar em nenhum
> console — nos planos Free e Pro. Testado a 375 px.

---

### Fase 6 — Migração dos clientes atuais

Dois clientes: a conta do tio e a conta de teste. Ambos vão para
`price_dfe_unlimited_internal_monthly` (R$ 0, cotas `-1`), que já existe no seed exatamente para isso.

- [ ] **6.1** Backfill de `owner_user_id` nas organizações existentes (2.3).
- [ ] **6.2** Criar os dois `Customer` em billing e assinar o preço interno, via chamada M2M
  autenticada — não editando linha no DynamoDB. Toda assinatura tem que ter passado pela mesma
  máquina de estados, ou a auditoria mente.
- [ ] **6.3** Verificar: os dois recebem `entitled=true`, `plan=unlimited`, nenhuma fatura gerada
  (total zero liquida na emissão, ADR 0019), nenhum lembrete de dunning (fatura de total zero
  não entra na fila).
- [ ] **6.4** Só então ligar o middleware de bloqueio (3.4) em produção. Ligar antes é derrubar os
  dois únicos clientes que existem.

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
