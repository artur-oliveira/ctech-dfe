# go-dfe-egress (sa-east-1) e remoção do py-dfe — Design

Data: 2026-10-06 · Status: aprovada; remoção do diretório py-dfe adiada (ver plano, Task 10)

## 1. Problema

Em 2026-10-06 uma emissão de NFS-e em Teresina falhou com `dial tcp 64.181.162.13:443: connect: connection timed out`
(invocação de 80,3 s). Teste direto, com um Lambda descartável em cada região:

| Região    | Resultado para `nfseapi.teresina.pi.gov.br:443` |
|-----------|-------------------------------------------------|
| us-east-1 | TCP não conecta (timeout de 10 s)               |
| sa-east-1 | TCP em 24 ms, TLS 1.2 em 257 ms                 |

Conclusão: o autorizador municipal descarta tráfego originado fora do Brasil. Não há IP fixo a liberar (Lambda fora de
VPC, IPs dinâmicos), então a correção é originar as chamadas ao fisco de sa-east-1.

Em paralelo, o py-dfe não tem mais função: todos os `(doc_type, service)` que ele atende estão no `implemented` do
go-dfe (conferido: NF-e 8, NFC-e 6, CT-e 8, MDF-e 6; NFS-e só existe no go-dfe). O worker já chama o go-dfe em
processo; só a API ainda invoca o Lambda py-dfe (`SEFAZ_FUNCTION_NAME`: `LookupOrganization`, `LookupByNSU`), com o
go-dfe em shadow-compare.

## 2. Objetivo e critérios de sucesso

1. Toda chamada a SEFAZ/prefeitura, de qualquer doc type, sai de um Lambda em sa-east-1.
2. Worker e API chamam esse Lambda; nenhum deles chama `dfe.Call` em processo nem o py-dfe.
3. `py-dfe/` e tudo que o referencia (CDK, CI, IAM, docs, comentários no go-dfe) é removido.
4. NFS-e de Teresina emite em produção restrita/produção sem timeout de conexão.
5. Falha do egress nunca é silenciosa: o erro chega ao caller como hoje chegava o do py-dfe.

Fora de escopo: mover filas, DynamoDB, S3 ou SNS para sa-east-1; qualquer mudança de regra fiscal.

## 3. Arquitetura

```
SQS (us-east-1) → ESM → Lambda worker (us-east-1) ─┐
                                                   ├─ Invoke síncrono → {env}-go-dfe-egress (sa-east-1) → SEFAZ / prefeituras
API (us-east-1) ───────────────────────────────────┘
```

O egress executa `dfe.Call` completo (monta XML, assina, POST mTLS, parse). Não acessa nenhum serviço AWS. A lógica
fiscal continua só no go-dfe; o egress é um invólucro.

### 3.1 Novo módulo `go-dfe-egress/`

- `go.mod` próprio, adicionado ao `go.work`; `main.go` com o handler `lambda.Start`: desserializa `dfe.Request`,
  chama `dfe.Call`, devolve `dfe.Response`. Mantém o go-dfe como biblioteca sem `cmd/` (regra do `go-dfe/CLAUDE.md`).
- Contrato idêntico ao do py-dfe (`request.go`: `Request`/`Response`/`Problem`), para worker e API não mudarem de
  formato.
- **Nunca loga o payload** (contém PFX e senha). Loga só `doc_type`, `service`, `uf`, status e duração.
- Runtime `provided.al2023`, arm64, `CGO_ENABLED=0` (regra do repo).

### 3.2 Worker e API

- Segundo client Lambda com `Region` vinda de variável de ambiente nova (`DFE_EGRESS_REGION`); o nome continua em
  `DFE_LAMBDA_NAME` (passa a apontar para `{env}-go-dfe-egress`).
- Remove-se o desvio `godfeImplements`/`godfeCall` e os `invokePyDfe*` viram uma única função de Invoke (renomeada).
- Remove-se o shadow-compare da API (`godfe_shadow.go`) e `go-dfe/shadow.go`.
- A API continua importando o go-dfe **como biblioteca de tipos** (modelo `nfse`, validadores, `Request`/`Response`);
  só o caminho de execução (`dfe.Call`) deixa de rodar nela.
- Erros: `FunctionError` do Invoke e `statusCode != 200` no envelope seguem o tratamento atual (`markRetryable` no
  worker; `problem.*` na API).

### 3.3 CDK

- Stack nova (`GoDfeEgressStack`) com `env.region = sa-east-1`, criada no mesmo app. Referências entre regiões só por
  string (nome/ARN calculados), sem cross-region references do CloudFormation.
- IAM: worker e API recebem `lambda:InvokeFunction` no ARN de `sa-east-1`; role do egress só com
  `AWSLambdaBasicExecutionRole`. A `iam-stack` troca a policy `...-py-dfe-lambda-invoke-policy` pela do egress.
- Pré-requisito operacional: `cdk bootstrap` de sa-east-1 na conta.
- Remove-se `DfeStack`, o layer Python, os roles `...-py-dfe-*` e `PyDfeLambdaDeployRole` (OIDC).
- CI: o job do py-dfe sai; entra o job do egress (build+test Go, deploy após o CDK e antes de worker/API,
  mantendo a ordem atual "Lambdas antes da API").

### 3.4 Timeouts

Restrição em cadeia: `timeout do worker/API > timeout do egress > pior caso das tentativas do go-dfe`.

- O client HTTP do NFS-e não tem timeout hoje; passa a ter `Dial` e timeout total como constantes nomeadas (connect ≈ 10
  s, alinhado ao `defaultTimeoutConnect` já existente), para uma indisponibilidade custar segundos, não
  80 s.
- Valores finais, incluindo memória do egress, ficam no plano; os workers com timeout de 60 s (`worker-definitions.ts`)
  precisam cobrir o do egress.

### 3.5 go-dfe

- Reescrever as ~190 referências ao py-dfe (comentários de fidelidade, `README.md`, `CLAUDE.md`): citar o commit/
  documento histórico em vez de um caminho que deixa de existir; não alterar tabelas nem lógica.
- `implemented` permanece como allowlist de operações aceitas por `Call`; some a linguagem de "promoção"/fallback.

## 4. Mudança de decisão registrada

Em 2026-07-18 o worker passou a chamar o go-dfe em processo (sem hop de rede). Esta mudança desfaz isso para os
documentos fiscais: volta a existir um Invoke entre regiões (~120 ms) por chamada ao fisco. Aceito porque o ganho de
latência com os servidores brasileiros (handshake mTLS de ~0,5 s para ~0,1 s por chamada) cobre o custo, e porque é
o único jeito de atingir os servidores que bloqueiam o exterior.

**Sem fallback em processo.** Se o egress falhar, a chamada falha e o retry normal do worker cobre. O rollback é
redeploy do commit anterior.

## 5. Riscos

| Risco                                         | Mitigação                                                                                            |
|-----------------------------------------------|------------------------------------------------------------------------------------------------------|
| PFX e senha trafegam entre regiões no payload | Mesma conta e IAM; o py-dfe já recebia assim; egress nunca loga o corpo; registrar em `CONDUCT.md`    |
| Egress fora do ar derruba toda emissão         | Retry existente do worker; alarme de erro/duração do egress; função única e sem dependências externas |
| Remover py-dfe antes de provar o egress        | Ordem de rollout fixa (seção 6); py-dfe só sai na última fase                                        |
| Timeouts em cascata mal dimensionados         | Restrição em cadeia da seção 3.4, verificada por teste/CDK test                                      |
| sa-east-1 sem bootstrap do CDK                | Passo explícito do plano; falha cedo no deploy                                                       |

## 6. Rollout

1. Limpeza de referências ao py-dfe no go-dfe (sem mudança de comportamento).
2. Módulo `go-dfe-egress/` + stack CDK em sa-east-1 + CI; deploy sem consumidores.
3. Worker e API apontam para o egress; remove-se o caminho em processo e o shadow.
4. Validação: NFS-e de Teresina e uma consulta de status de cada doc type, comparando o resultado com o esperado.
5. Remoção de `py-dfe/`, `DfeStack`, OIDC/IAM/CI do py-dfe e das demais referências.
6. Documentação (`DOCS.md`, `OVERVIEW.md`, `MIGRATION.md`, `CONDUCT.md`, `DEPLOYMENT.md`, `CLAUDE.md`s, `AGENTS.md`s).

Cada fase é um commit/PR independente e reversível, exceto a 5 (a remoção em si).

## 7. Testes

| Mudança                                 | Teste                                                                                                  |
|-----------------------------------------|--------------------------------------------------------------------------------------------------------|
| Handler do egress                        | Unit: round-trip `Request`→`Response`; erro de `dfe.Call` vira `Problem`; payload não aparece em log   |
| Client Lambda cross-region (worker/API) | Unit com fake `lambda` verificando região, nome e tratamento de `FunctionError`/status                 |
| Timeouts do NFS-e                       | Unit com servidor que não aceita a conexão (connect timeout dentro do limite)                          |
| CDK                                     | Teste de snapshot/asserts: stack em sa-east-1, política de invoke com ARN regional, ausência do py-dfe |
| Integração                              | Invoke real do egress em sa-east-1 (status de serviço em homologação) antes da fase 4                   |
| Regressão do bug                        | NFS-e Teresina (ou status do host) a partir de sa-east-1, sem timeout de conexão                       |
