# Spec — Remediação dos achados do audit de engenharia (2026-07-17)

**Fonte:** `~/Documents/Projects/Ctech/_analysis/GENERAL-REPORT.md` e `ctech-dfe.md` (audit datado de 2026-07-17).
**Escopo:** apenas achados que afetam o repositório `ctech-dfe` (api, worker, cdk, docs). Achados de outros repositórios
(`ctech-account`, `ctech-wallet`, `ctech-cdk`) estão fora de escopo aqui. **Status verificado em:** 2026-07-18, contra o
código atual (não apenas o texto do audit — vários achados já foram corrigidos por commits recentes e estão marcados
como tal abaixo).

---

## 0. Achados já resolvidos (nenhuma ação necessária)

Confirmado lendo o código atual — não incluir nas tarefas de implementação:

| Achado do audit                                                            | Estado atual                                                                                                  |
|----------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| DynamoDB `maxReadRequestUnits/maxWriteRequestUnits: 5` em todas as tabelas | `cdk/lib/dynamodb-stack.ts` já usa `1000/1000` em todas as ocorrências.                                       |
| JWKS verifier duplicado (`api/internal/middleware/auth.go`)                | Já delega para `gopkg.aoctech.app/api-commons/jwtverify` (`auth.go:10,28,32`, `go.mod` `api-commons v1.0.2`). |
| `ws` (Redis/memory pub-sub registry) duplicado                             | Já extraído para `api-commons` — `api/internal/ws/` não existe mais localmente.                               |
| `cache` duplicado                                                          | Já extraído para `api-commons` — `api/internal/cache/` não existe mais localmente.                            |
| `repositories/marshal.go` duplicado                                        | Arquivo não existe mais (só resta `marshal_test.go`, órfão — ver Tarefa 8).                                   |

---

## 1. P0 — DLQ silenciosa (nenhum alarme, nenhuma escrita de status terminal)

**Problema:** um documento fiscal que falha 3 vezes (`maxReceiveCount: 3`,
`cdk/lib/worker-stack.ts:111`) cai na DLQ. O `dlq-processor`
(`worker/cmd/dlq-processor/main.go`) só publica uma notificação SNS best-effort — nunca escreve de volta no DynamoDB. Se
ninguém estiver conectado via WebSocket naquele instante exato, a notificação se perde e o documento fica travado em
`pending`/`authorized` (para eventos, via
`failDoc`) **para sempre**, sem reconciliação. Além disso, `grep -rni alarm cdk/lib/*.ts` não retorna nenhum resultado —
zero CloudWatch Alarms em qualquer DLQ do sistema, contradizendo
`worker/CLAUDE.md:121` ("DLQ receives messages after max retries — monitor via CloudWatch alarms (configured in CDK)").

### Requisito 1.1 — DLQ processor escreve status terminal no DynamoDB

- O `dlq-processor` (`worker/cmd/dlq-processor/main.go`) deve, para cada mensagem da DLQ, fazer um `UpdateItem` na
  tabela indicada por `table_name` (já presente no corpo da mensagem, linha 45)
  marcando o item como `StatusFailed` (`"failed"`, ver `worker/internal/service/helpers.go:14`)
  antes (ou independente) de publicar no SNS. A publicação SNS best-effort é mantida como está — isso é apenas
  notificação em tempo real, não o registro de fato.
- Reaproveitar o padrão de update já usado em `updateEvent`
  (`worker/internal/service/distribution.go` / `dfe.go:487-506`): `Key{pk, sk}`,
  `ConditionExpression: "attribute_exists(pk)"` (não falhar se o item já tiver sido processado por outra via — usar
  `UpdateItem` idempotente, não `PutItem`).
- Campos mínimos a atualizar: `status = "failed"`, `sefaz_motive = "Falha após todas as
  tentativas de reprocessamento"` (mesma mensagem hoje só mandada por SNS,
  `main.go:63`), `updated_at = now`.
- **Ambiguidade a resolver antes de codar:** a mensagem da DLQ hoje só carrega
  `access_key, doc_pk, table_name` (`main.go:43-45`) — não carrega `sk` (sort key) nem indica se é um documento
  (`pk=access_key`) ou um evento (`pk=org_pk`, `sk=uuidv7`, ver
  `DynamoDB-Tables.md` §15-18). O producer da mensagem original (`WorkerMessage`,
  `dfe.go:73-92`) tem esses campos (`DocPK`, `EventSK`, `EventsTableName`) mas eles não estão sendo propagados para o
  corpo que a DLQ recebe (esse corpo é a mensagem SQS original, não algo reconstruído — **confirmar no CDK/SQS que o
  corpo da mensagem original inclui os mesmos campos de `WorkerMessage` serializados**; se sim, o handler do DLQ
  processor deve decodificar o mesmo formato de `WorkerMessage` em vez do `map[string]any` ad-hoc atual, para ter acesso
  a
  `EventsTableName`/`EventSK` e diferenciar documento de evento).
- IAM: adicionar `dynamodb:UpdateItem` ao `dlqRole` em `cdk/lib/worker-stack.ts:220-231`
  (hoje só tem `sns:Publish`), escopado às mesmas tabelas do `worker.dynamoTables` do respectivo worker.

### Requisito 1.2 — CloudWatch Alarm em cada DLQ

- Em `cdk/lib/worker-stack.ts`, dentro do loop `for (const worker of workers)` (linha 99), após criar `dlq` (linha 101),
  adicionar um `cloudwatch.Alarm` sobre a métrica
  `ApproximateNumberOfMessagesVisible` da fila `dlq`, threshold `>= 1`, `evaluationPeriods: 1`, disparando sempre que
  qualquer mensagem chegar na DLQ (não é uma métrica de volume, é
  "aconteceu alguma coisa").
- **Decidido (2026-07-18):** criar um `opsAlertsTopicArn` novo (SNS topic dedicado a alertas operacionais, assinatura
  por e-mail/Slack fora do escopo desta spec/plano — só o topic e a inscrição do alarme nele).
- Rodar `cdk synth` e confirmar que o alarme aparece no template sintetizado para os 8 workers.

**Arquivos tocados:** `worker/cmd/dlq-processor/main.go`, `cdk/lib/worker-stack.ts`. **Testes:** unit test do
`dlq-processor` cobrindo o `UpdateItem` (mock DynamoDB); `cdk test`
(snapshot) cobrindo o novo `Alarm` construct.

---

## 2. P0 — Discrepância FIFO (docs afirmam garantia que não existe) + falta de idempotência

**Problema:** `OVERVIEW.md:31,100,139`, `MIGRATION.md:18,121,179,197,217,229`, `cdk/CLAUDE.md:12,27,145`
e `worker/CLAUDE.md:3,11,15,71,73,121` afirmam que as filas são **SQS FIFO** com
`MessageGroupId = org_pk` garantindo ordenação por organização. O CDK real (`cdk/lib/worker-stack.ts:101-113`) cria
`sqs.Queue` **standard** — sem `.fifo`, sem
`fifo: true`, sem `contentBasedDeduplication`, sem `MessageGroupId` em lugar nenhum. Standard SQS é *at-least-once* e
**sem garantia de ordem**. Hoje a única proteção contra reprocessamento duplicado é o próprio SEFAZ rejeitar o evento
duplicado (`DuplicatedEventError`,
`dfe.go:234,244` — dependência implícita de um sistema externo, não uma garantia própria) — e a função `Process`
(`dfe.go:111-163`) não tem nenhum `ConditionExpression` de dedup antes de invocar o SEFAZ, ao contrário de
`distribution.go:464` (`attribute_not_exists(pk) AND
attribute_not_exists(nsu)`) e `distribution.go:782` (`attribute_not_exists(pk)`).

### Decisão a tomar (bloqueia a implementação — ver Tarefa 2.0)

Duas opções, mutuamente exclusivas:

- **(a) Manter standard SQS + corrigir os docs + adicionar guarda de idempotência própria.**
  Justificativa técnica: a reserva de numeração fiscal já usa `transact_write` (atômico, não depende de ordem de
  chegada — ver `DynamoDB-Tables.md`), então ordenação por org pode não ser necessária de fato. Menor blast radius:
  nenhuma mudança de infraestrutura em SNS/SQS, só correção de 4 docs + 1 guarda de código.
- **(b) Converter as filas para FIFO real.** Exige: sufixo `.fifo` no nome da fila, `fifo: true`,
  `contentBasedDeduplication` ou `MessageDeduplicationId` explícito, `MessageGroupId = org_pk` no envio — e como as
  filas são alimentadas por uma subscription de um tópico SNS (`eventBus.addSubscription`, `worker-stack.ts:120-129`),
  **o tópico SNS também precisaria virar FIFO**, o que é uma mudança de infra maior e cascateia para quem publica no
  `eventBus` hoje.

**Decidido (2026-07-18): opção (a).** Manter standard SQS, corrigir a documentação, adicionar guarda de idempotência
própria em `Process`.

### Requisito 2.1 — Corrigir a documentação

Remover/corrigir toda menção a "SQS FIFO"/"MessageGroupId" garantindo ordenação, substituindo por uma descrição precisa
(standard SQS, at-least-once, idempotência garantida por
`ConditionExpression` na camada de aplicação):

- `OVERVIEW.md:31` (tabela de componentes), `:100`, `:139` (fluxo de emissão)
- `MIGRATION.md:18` (tabela de "Strengths"), `:121`, `:179`, `:197`, `:217`, `:229` (diagramas/texto)
- `cdk/CLAUDE.md:12`, `:27` (descrição do worker-stack), `:145` (área crítica)
- `worker/CLAUDE.md:3`, `:11`, `:15` (papel/fluxo), `:71` ("SQS FIFO provides at-least-once delivery" — frase hoje
  contraditória: FIFO não é "at-least-once", é a garantia oposta que está sendo documentada errada), `:73`, `:121` (
  "Known Constraints")

**Nota:** `ROADMAP.md` foi checado e **não** contém menções a FIFO/MessageGroupId — o achado do audit geral sobre esse
arquivo está desatualizado/impreciso; não precisa de alteração.

### Requisito 2.2 — Guarda de idempotência em `Process`

- Em `worker/internal/service/dfe.go`, função `Process` (linha 111), antes de chamar
  `s.invokePyDfe`, adicionar uma checagem/guarda condicional que impeça reprocessar um
  `WorkerMessage` cujo item já esteja em estado terminal (`StatusAuthorized`, `StatusRejected`,
  `StatusFailed` — `helpers.go:12-14` — ou `EventStatusSuccess`/`EventStatusError` para eventos,
  `helpers.go:18-19`).
- Padrão a seguir (mesmo já usado em `distribution.go:782`): um `GetItem` de leitura do status atual antes de prosseguir
  (mais simples e explícito que tentar um `ConditionExpression` em cima de uma chamada externa ao SEFAZ, já que o
  "write" real acontece só depois da resposta do SEFAZ, em `handleSefazResponse`/`updateStatus`). Se o item já estiver
  em status terminal, logar (`slog.Info`, com `access_key`) e retornar `nil` sem invocar o SEFAZ de novo.
- Isso não substitui a opção de manter o `DuplicatedEventError` do SEFAZ como segunda camada de defesa
  (`dfe.go:234,244`) — mantém-se como está, a guarda nova é a camada própria que falta.

**Arquivos tocados:** `OVERVIEW.md`, `MIGRATION.md`, `cdk/CLAUDE.md`, `worker/CLAUDE.md`,
`worker/internal/service/dfe.go`. **Testes:** teste de integração reproduzindo mensagem duplicada (SQS redelivery)
confirmando que o segundo processamento não invoca `invokePyDfe` — mesmo padrão pedido em `worker/CLAUDE.md`'s própria
tabela de testes ("Idempotency path | Integration test (duplicate msg)").

---

## 3. P1 — `RemovalPolicy.DESTROY` incondicional nos buckets S3

**Problema:** `cdk/lib/s3-stack.ts:29,44` — ambos os buckets (`certificatesBucket`,
`documentsBucket`) têm `removalPolicy: cdk.RemovalPolicy.DESTROY` incondicional, violando a própria regra documentada em
`cdk/CLAUDE.md` ("`RemovalPolicy.DESTROY` is dev-only. Never set it for staging or production."). Note que `versioned` e
`autoDeleteObjects` (linhas 28,30,43,45) já seguem o padrão condicional correto (`isProduction ? ... : ...`) — só
`removalPolicy` ficou de fora.

### Requisito 3.1

Em `cdk/lib/s3-stack.ts:29,44`, trocar:

```ts
removalPolicy: cdk.RemovalPolicy.DESTROY,
```

por:

```ts
removalPolicy: isProduction ? cdk.RemovalPolicy.RETAIN : cdk.RemovalPolicy.DESTROY,
```

em ambos os buckets. Seguir exatamente o padrão `isProduction ? ... : ...` já usado duas linhas acima para `versioned`/
`autoDeleteObjects` (não introduzir uma terceira variável de ambiente
`staging` separada — o resto do código trata só `dev` vs `prod`, `isProduction` já cobre o caso).

**Arquivos tocados:** `cdk/lib/s3-stack.ts`. **Testes:** `cdk test` (snapshot) para `prod` e `dev`, confirmando `RETAIN`
em prod e `DESTROY`
em dev.

---

## 4. P1 — Bucket de documentos sem lifecycle rule (Standard → IA)

**Problema:** `documentsBucket` (`cdk/lib/s3-stack.ts:39-46`) guarda os XMLs fiscais assinados (retenção legal de ~5
anos no Brasil) e não tem nenhuma `lifecycleRules`, ao contrário de
`certificatesBucket` que já tem uma regra (linhas 31-36, mas essa é `expiration` em 90 dias no prefixo `temp/`, não uma
transição de storage class). Tudo fica em S3 Standard para sempre — XMLs fiscais são lidos raramente após a primeira
semana, então Standard-IA (~45% mais barato)
é economia livre de risco.

### Requisito 4.1

Adicionar em `documentsBucket` (`cdk/lib/s3-stack.ts:39-46`):

```ts
lifecycleRules: [
    {
        transitions: [
            {
                storageClass: s3.StorageClass.INFREQUENT_ACCESS,
                transitionAfter: cdk.Duration.days(90),
            },
        ],
    },
],
```

(90 dias, não 60, para dar margem de segurança acima do mínimo de 30 dias exigido pelo S3 para transição a Standard-IA,
e alinhado com o precedente já usado no `certificatesBucket`.)

**Arquivos tocados:** `cdk/lib/s3-stack.ts`. **Testes:** `cdk test` (snapshot) confirmando a lifecycle rule no bucket de
documentos.

---

## 5. P1 — Documentação obsoleta / incorreta

Três problemas de doc-vs-realidade independentes, cada um resolvível sem tocar código:

### Requisito 5.1 — `DEPLOYMENT.md` descreve o stack errado

`DEPLOYMENT.md:12` tem um `TODO: Replace with actual stack topology` literal; linhas ~205-213 e
~362-374 instruem `systemctl status app` como se `app` fosse "FastAPI/Gunicorn". O binário real é Go/Fiber rodando via
systemd (`cdk/lib/api-v2-stack.ts` userdata, `/opt/app/current/app`). Reescrever essas seções para refletir o stack
Go/Fiber real: nome do processo, caminhos de log reais, comandos de diagnóstico corretos para o binário Go. Remover o
placeholder TODO.

### Requisito 5.2 — `cdk/CLAUDE.md` linha stale sobre migração Beanstalk

`cdk/CLAUDE.md:131` ("`ApiStack` (Elastic Beanstalk) is legacy — migration to `ApiStackV2` (EC2 ASG) is in progress.") —
a migração **já terminou**: `cdk/bin/ctech-dfe-cdk.ts` só instancia
`ApiStackV2`, nenhum construct Beanstalk existe em `cdk/lib`. Atualizar a linha para remover a menção a migração em
progresso (ex.: remover a frase ou trocar por "`ApiStackV2` (EC2 ASG) é o único stack de API; a migração do antigo
`ApiStack` (Beanstalk) foi concluída.").

### Requisito 5.3 — Nome do header de organização divergente entre doc e código

O código lê o header `Dfe-Organization-Pk` (`api/internal/middleware/rbac.go:22`), mas
`OVERVIEW.md:62` e `INTEGRATION.md:67,120,128,167` documentam `PyDfe-Organization-Pk`. Corrigir todas as ocorrências nos
dois docs para `Dfe-Organization-Pk`, batendo com o código (não mudar o código — o header já está em produção, mudar o
nome do header quebraria todo integrador existente; a correção é só documental).

**Arquivos tocados:** `DEPLOYMENT.md`, `cdk/CLAUDE.md`, `OVERVIEW.md`, `INTEGRATION.md`. **Testes:** nenhum — mudança
documental.

---

## 6. P2 — Decidir o destino do distribution-poller desabilitado

**Problema:** `cdk/lib/worker-stack.ts:322-328` define um `scheduler.Schedule` (a cada 30 min)
para o `distribution-dispatcher`, com `enabled: false`. É o item 0.4 do `ROADMAP.md` (`GET
/v1.0/distributions/nfe`) — a infra existe, só o toggle está desligado.

### Requisito 6.1

**Decidido (2026-07-18): ligar.** Trocar `enabled: false` → `enabled: true` em
`cdk/lib/worker-stack.ts:327`. Validar em `dev`/`staging` antes de subir para prod (o dispatcher escaneia tabelas de
config de todas as orgs ativas a cada 30 min — confirmar volume esperado antes de ligar em prod).

**Arquivos tocados:** `cdk/lib/worker-stack.ts`.

---

## 7. P2 — Extração dos 3 pacotes Go ainda duplicados com `ctech-wallet`

**Contexto:** o audit geral listava 6 pacotes duplicados entre `ctech-dfe` e `ctech-wallet`. Verificado hoje: **3 já
foram extraídos** para `gopkg.aoctech.app/api-commons` (`jwtverify`, `ws`,
`cache` — ver seção 0). Restam locais e não extraídos:

- `api/internal/problem/problem.go` (119 linhas) — tipo base RFC 7807 + helpers genéricos;
  `ctech-wallet` tem o mesmo tipo base e adiciona códigos de problema próprios por cima.
- `api/internal/awsclient/client.go` (57 linhas) — wrapper `Clients`; `ctech-wallet` usa o mesmo padrão com um
  subconjunto de clientes (DynamoDB+SSM vs. S3/SQS/SNS/Lambda/SecretsManager aqui).
- `api/internal/repositories/base.go` (133 linhas) — `Base` genérico de DynamoDB (table-name, `TransactWrite`, `Query`/
  `GetItem`); a versão do `ctech-wallet` já tem um método extra (`UpsertAttrs`) que este não tem — **já está
  divergindo**, exatamente o tipo de drift que
  `CLAUDE.md` (raiz, "Universal Rules") pede pra evitar.

**Fora de escopo desta spec — decisão cross-repo:** essa extração precisa acontecer no repo
`api-commons` (o mesmo já usado pelo `jwtverify`/`ws`/`cache`), com `ctech-dfe` e `ctech-wallet`
migrando as importações depois. Por instrução de projeto (CLAUDE.md raiz: "before hand-rolling... check whether a
sibling repo already solves it... ask whether it belongs in a shared package"), esse trabalho deve virar uma spec
própria no repo `api-commons` (ou onde ele vive), não ser feito dentro de `ctech-dfe` isoladamente — senão se repete o
mesmo erro que já criou a divergência do
`base.go`.

### Requisito 7.1 (ação restrita a este repo)

Depois que `api-commons` publicar `problem`, `awsclient` (padrão de init/config, não necessariamente o mesmo conjunto de
clientes) e `repositories/base` (reconciliando o `UpsertAttrs`
que falta aqui): trocar as importações locais de `ctech-dfe` para o pacote compartilhado e apagar os 3 arquivos locais.
**Não codar isso agora** — é um passo subsequente, bloqueado pela extração no outro repo.

### Nota — arquivo órfão

`api/internal/repositories/marshal_test.go` existe mas `marshal.go` não existe mais no diretório — confirmar se os
testes ainda compilam (provavelmente testam funções que migraram para
`base.go` ou outro arquivo do mesmo pacote) e, se o arquivo estiver testando algo que não existe mais, removê-lo ou
apontá-lo para o arquivo certo.

---

## Fora de escopo desta spec (mencionar, não implementar)

- **Hash-chaining / S3 Object Lock para `audit_logs`** (tamper-evidence legal) — achado P2 do audit (`ctech-dfe.md` item
  11), correto mas grande o suficiente (decisão de arquitetura sobre o que "legalmente inviolável" significa aqui) pra
  merecer sua própria spec depois que os itens P0/P1 acima estiverem resolvidos.
- Qualquer achado do `GENERAL-REPORT.md` referente a `ctech-account`, `ctech-wallet` ou
  `ctech-cdk` (secrets expostos, rate limiting, race de idempotência de saque, IAM
  `AdministratorAccess`, etc.) — pertence aos repositórios respectivos, não a este.

---

## Resumo executável (ordem sugerida)

| #  | Item                                                                    | Prioridade | Bloqueado por                          |
|----|-------------------------------------------------------------------------|------------|----------------------------------------|
| 1  | DLQ processor escreve status terminal + IAM                             | P0         | —                                      |
| 2  | CloudWatch Alarm por DLQ                                                | P0         | —                                      |
| 3  | Opção (a) decidida: corrigir docs + idempotência em `Process`           | P0         | —                                      |
| 4  | `removalPolicy` condicional nos 2 buckets S3                            | P1         | —                                      |
| 5  | Lifecycle rule Standard→IA no bucket de documentos                      | P1         | —                                      |
| 6  | Reescrever `DEPLOYMENT.md`                                              | P1         | —                                      |
| 7  | Corrigir `cdk/CLAUDE.md` (linha Beanstalk stale)                        | P1         | —                                      |
| 8  | Corrigir header `Dfe-Organization-Pk` em `OVERVIEW.md`/`INTEGRATION.md` | P1         | —                                      |
| 9  | Ligar distribution-poller (`enabled: true`)                             | P2         | —                                      |
| 10 | Migrar imports pros 3 pacotes restantes                                 | P2         | Extração em `api-commons` (outro repo) |
| 11 | Investigar `marshal_test.go` órfão                                      | P2         | —                                      |
