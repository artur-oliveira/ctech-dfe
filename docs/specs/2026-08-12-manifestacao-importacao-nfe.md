# Manifestação manual e importação por chave de acesso (NF-e)

Data: 2026-08-12
Status: aprovado (design), pendente plano de implementação

## Problema

Ao receber um `resNFe` da SEFAZ, o sistema dispara automaticamente o evento
"Ciência da Operação" (210210). Se a nota chegar após o prazo de 10 dias, a
SEFAZ rejeita o evento com o código 596
(`Rejeicao: Evento apresentado apos o prazo permitido para o evento: [10 dias]`).
Quando isso acontece, a manifestação falha silenciosamente e o usuário nunca
recebe a NF-e completa (produtos/pagamentos) de forma automática — só o
resumo (`resNFe`) fica disponível.

Hoje não existe nenhuma forma manual de:
- disparar um evento de manifestação diferente do automático;
- forçar uma nova consulta por chave de acesso para obter a nota completa;
- importar manualmente uma nota pela tela de Distribuição usando uma chave de
  acesso digitada pelo usuário.

## Escopo

Exclusivo para NF-e destinadas (`incoming === 1`). CT-e/MDF-e não têm o
conceito de `resNFe`/Ciência da Operação e ficam fora do escopo.

## Arquitetura

### Backend — api

1. **Endpoint de manifestação manual** já existe:
   `POST /nfes/:access_key/manifestation` (`api/internal/api/v1/nfes.go:135-151`,
   serviço `Manifestation` em `api/internal/services/nfes/service.go:253-291`).
   Nenhuma mudança de backend — só passa a ser chamado pelo frontend.

2. **Consulta por chave assíncrona**: `LookupByKey`
   (`api/internal/services/distributions.go:254-273`) hoje é síncrono
   (invoca py-dfe/go-dfe diretamente e devolve o resultado no corpo da
   resposta). Passa a enfileirar, no mesmo padrão de `EnqueueSync`
   (`api/internal/services/distributions.go:128-186`):
   - mantém `checkConsQuota` antes de enfileirar;
   - publica mensagem SQS `{"job_type":"cons_ch_nfe","org_pk":...,"doc_type":"nfe","access_key":...,"trigger":"user","triggered_at":...}`
     na fila `distribution` já existente (reaproveita
     `DistributionMessage.AccessKey`, `worker/internal/service/distribution.go:144-151`);
   - retorna 202 `{"status":"enqueued"}`.
   - Rota: `GET /distributions/{doc_type}/key/{access_key}` é **substituída**
     por `POST /distributions/nfe/key` com body `{"access_key": "..."}`
     (não há consumidor hoje da rota GET, sem necessidade de manter as duas).

3. **Fix do bug de notificação WS**: `ResultsConsumer.dispatch`
   (`api/internal/consumer/results.go:119-157`) hoje descarta qualquer
   mensagem sem `doc_pk` contendo `"#"` antes de setar `type` e fazer o
   broadcast. A mensagem `new_distribution_nfe` publicada por `notifyResult`
   (`worker/internal/service/distribution.go:861-878`) só tem `org_pk`, então
   é sempre descartada — o toast "Nova NF-e recebida" nunca funcionou. Fix:
   aceitar `org_pk` como identificador válido quando `doc_pk` estiver ausente,
   sem alterar o formato da mensagem publicada pelo worker.

### Backend — worker

Nenhuma infraestrutura nova. O job `cons_ch_nfe` já é tratado por
`runConsAccessKey` (`worker/internal/service/distribution.go:385-442`), que
já persiste o documento, sobe XML pro S3, grava evento e chama
`notifyResult`. Único acréscimo: checagem de quota duplicada no worker antes
de processar (mesmo padrão usado para `claimDistNSUSlot`,
`worker/internal/service/distribution.go:884-935`), para cobrir corrida de
SQS at-least-once / cliques duplicados do usuário.

Nenhuma mudança de CDK — fila e Lambda `distribution-worker` já cobrem o job.

### Frontend — ui

**A. Botão "Manifestação"** em `ui/src/components/dfe/DfeDetail.tsx`, visível
sempre que a nota for NF-e destinada (`incoming === 1`). Abre modal com:
- select do tipo de evento: Ciência (210210), Confirmação (210200),
  Desconhecimento (210220), Não Realizada (210240) — usa os labels já
  existentes em `ui/src/lib/data/dfe_event.ts:19-22`. Oculta da lista os
  tipos que já possuem um evento **autorizado** (não rejeitado) para essa
  nota, evitando manifestação duplicada óbvia (a SEFAZ segue sendo a fonte
  de verdade — rejeição de duplicidade continua tratada via toast de erro).
- campo justificativa (texto livre, 15-255 caracteres): obrigatório apenas
  quando o tipo selecionado é "Não Realizada" (210240), opcional nos demais,
  espelhando a regra de negócio da SEFAZ e a validação já existente no DTO
  do endpoint (`justification` 15-255 chars quando presente).
- submit chama `POST /nfes/:access_key/manifestation`. Resultado chega via
  fluxo assíncrono já existente (`dfe_result` por WebSocket → toast +
  invalidação de queries, `ui/src/lib/hooks/useRealtimeUpdates.ts:63-76`).

**B. Botão "Importar via distribuição"** no mesmo componente, ao lado do
botão de manifestação. Habilitado somente quando a nota **não** está
completa — mesma checagem já usada em `DfeDetail.tsx:225`
(`doc.products` vazio/ausente). Ao clicar, chama o novo endpoint assíncrono
de consulta por chave (`POST /distributions/nfe/key`) usando a própria
`access_key` da nota. Resultado chega via WS assim que o worker processar.

**C. Botão "Importar NF-e"** na aba de Distribuição NF-e
(`ui/src/app/nfe/page.tsx`, ao lado do "Consultar SEFAZ" existente,
linhas 170/204-212). Abre modal com input de chave de acesso.

**D. Input de chave de acesso**: nova função `maskAccessKey` em
`ui/src/lib/utils/masks.ts` (mesmo padrão de `maskCpf`/`maskCnpj`),
formatando em grupos de 4 caracteres, aceitando dígitos e letras maiúsculas
(chave de CNPJ alfanumérico, NT 2023.002). Ao submeter, valor é
desformatado (mesmo padrão de `unformatCpfCnpj`) antes de enviar ao backend.

**E. Validação criteriosa da chave de acesso** — novo módulo em
`ui/src/lib/utils/` reaproveitando a decomposição já feita por
`parseAccessKey` (`ui/src/lib/utils/dfe.ts:42-65`). Regras:

| Campo    | Regra |
|----------|-------|
| `cUF`    | 2 dígitos, deve estar entre os códigos IBGE de UF válidos |
| `AAMM`   | 4 dígitos; `MM` entre 01 e 12 |
| `CNPJ`/`CPF` | Exatamente um dos dois: CNPJ alfanumérico (14 caracteres, dígitos+letras maiúsculas) com DV validado pelo algoritmo mod-11 alfanumérico (NT 2023.002 — valor de letra = código ASCII − 48); ou CPF numérico de 11 dígitos com prefixo `"000"` e DV de CPF validado |
| `mod`    | Exatamente `"55"` |
| `serie`  | 3 dígitos, sem validação de faixa |
| `nNF`    | 9 dígitos, sem validação de faixa |
| `tpEmis` | Um de `1,2,3,4,5,6,7` (rejeita `9`, exclusivo de NFC-e, e qualquer outro valor) |
| `cNF`    | 8 dígitos, sem validação adicional |
| `cDV`    | 1 dígito; recalculado via mod-11 sobre os 43 caracteres anteriores (alphanumeric-aware) e comparado ao dígito informado |

Cada falha reporta o campo específico que não passou (não apenas "chave
inválida"), para o modal mostrar a mensagem certa. Integrado ao formulário
via schema Zod (`.refine`/`.superRefine`). A construção do modal/form em si
segue a skill `impeccable` (shape/craft), conforme pedido do usuário.

## Fora de escopo (decisões explícitas)

- **Validação de DV/composição só no frontend.** SEFAZ já rejeita chave
  inválida na consulta (`consChNFe`); o backend mantém apenas a checagem de
  tamanho que já existe hoje. Se essa rota vier a ter outro consumidor além
  desta UI, a validação completa deve ser portada para Go.
- **"Importar NF-e" só na aba de Distribuição de NF-e.** CT-e/MDF-e não têm
  `resNFe`/Ciência da Operação — fora do escopo pedido.
- **Nenhuma infraestrutura CDK nova** — fila `distribution` e worker
  `distribution-worker` já cobrem o fluxo (job `cons_ch_nfe`).

## Fluxo de dados (importar via distribuição / importar NF-e)

```
Usuário clica botão (B ou C)
  → POST /distributions/nfe/key {access_key}
  → api: checkConsQuota → publica SQS {job_type: cons_ch_nfe, ...} → 202
  → worker (distribution-worker, fila distribution): runConsAccessKey
      → checagem de quota duplicada (novo)
      → consChNFe via py-dfe/go-dfe
      → processDocZip: persiste distribution record, sobe XML no S3,
        persistIncoming, persistCounterparties, persistEvent
      → notifyResult: publica SNS new_distribution_nfe {org_pk, access_key, ...}
  → api ResultsConsumer.dispatch (fix): aceita org_pk sem doc_pk, broadcast WS
  → frontend useRealtimeUpdates: invalida queries de detail/lista, toast
```

## Testes

**Go**
- `EnqueueLookupByKey` (mock SQS): valida quota check, formato da mensagem
  publicada, resposta 202.
- Fix do `ResultsConsumer.dispatch`: mensagem com `org_pk` e sem `doc_pk`
  deve ser broadcastada (regressão do bug atual).
- Quota duplicada no worker: nova unidade de teste seguindo o padrão de
  `claimDistNSUSlot`.

**TypeScript**
- Validador de chave de acesso: casos válidos e inválidos por campo
  (cUF inválido, mês 13, tpEmis 9, mod diferente de 55, CNPJ alfanumérico
  com DV errado, CPF com DV errado, cDV incorreto, CNPJ e CPF ambos
  ausentes/ambos presentes).
- `maskAccessKey`: formatação e desformatação.

## Documentação a atualizar (pós-implementação)

- `DOCS.md`: novo contrato `POST /distributions/nfe/key`, substituindo o
  `GET /distributions/{doc_type}/key/{access_key}` antigo.
- `CONDUCT.md`: nota sobre o fix do `doc_pk`/`org_pk` no `ResultsConsumer`
  (comportamento antes silenciosamente quebrado).
