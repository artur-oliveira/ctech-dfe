# Importação de NF-e por XML

Data: 2026-08-13
Status: aprovado (design), pendente plano de implementação

## Problema

Hoje a aba de Distribuição (NF-e) só permite importar um documento existente
via chave de acesso (`POST /distributions/nfe/key`, ver
`docs/specs/2026-08-12-manifestacao-importacao-nfe.md`). Não existe forma de
importar uma NF-e a partir do próprio arquivo XML — útil quando o usuário já
tem o XML em mãos (recebido por e-mail, baixado de outro sistema, etc.) e
quer que o sistema trate essa nota como se tivesse chegado pela distribuição
SEFAZ normal.

NFC-e também não tem nenhuma forma de importação por XML hoje.

## Escopo

- Doc types: `nfe` e `nfce` (CT-e/MDF-e fora de escopo — não pedidos).
- Aceita apenas XML cuja raiz seja `nfeProc` (NF-e com protocolo) ou `NFe`
  (NF-e sem protocolo, assinada mas ainda não autorizada/consultada).
  Validação XSD completa não é obrigatória nesta primeira versão — a
  validação estrutural é feita via os mesmos helpers de parsing genérico já
  usados pela distribuição (`worker/internal/service/distribution_parser.go`:
  `parseXMLBytes`, `findEl`, `findText`).
- Antes de persistir definitivamente, o worker sempre faz uma consulta
  protocolo (`NfeConsultaProtocolo`) contra a SEFAZ e compara o `digVal`
  retornado com o(s) digest(s) do XML enviado — nunca confia cegamente no
  arquivo do usuário.
- Se já existir uma NF-e completa (com produtos) para a mesma chave de
  acesso na organização, a importação falha (409-equivalente / rejeição de
  negócio).
- Se o XML não tiver nenhuma relação com a organização (nem `emit`, nem
  `dest`, nem `transp.transporta` batem com o CNPJ/CPF da org), a importação
  é rejeitada.

## Arquitetura

### Classificação / vínculo com a organização (emit > dest > transp)

Esta é a mesma checagem, usada com dois propósitos: decidir se a nota
pertence à organização E decidir em qual aba ela deve aparecer
(`emitidas`/`recebidas`/`transportadas`, campo `Incoming` 0/1/2 — ver
`api/internal/repositories/documents.go:101,137-145` e
`ui/src/app/nfe/page.tsx:40-61`).

Ordem de prioridade, checada nesta exata sequência contra o CNPJ/CPF da
organização (`extractCNPJ(orgPK)`):

1. `emit.CNPJ`/`emit.CPF` bate → `Incoming = 0` (emitida).
2. senão, `dest.CNPJ`/`dest.CPF` bate → `Incoming = 1` (destinada).
3. senão, `transp.transporta.CNPJ`/`CPF` bate → `Incoming = 2` (transportada).
4. nenhum bate → rejeita (XML não pertence à organização).

`extractProcNFe` (`distribution_parser.go:378-444`) não serve para isso sem
alteração: hoje ela nunca produz `Incoming = 0` (na distribuição normal a
SEFAZ nunca devolve para a org uma nota que ela mesma emitiu). A importação
por XML é o primeiro caso em que isso pode acontecer de verdade — usuário
pode importar uma nota que a própria org emitiu. Por isso a classificação
acima é uma função nova e específica desta feature (usa os mesmos
`findEl`/`findText`, não duplica o parsing), e o resultado sobrescreve o
`Incoming` que viria de `extractProcNFe` quando esta for reaproveitada para
extrair produtos/pagamentos/totais.

### Backend — api

**Novo endpoint**: `POST /distributions/:doc_type/import-xml`
(`doc_type` restrito a `nfe`/`nfce`, checagem dedicada — não reaproveita
`validateDistDocType`, que cobre CT-e/MDF-e/NFS-e fora de escopo aqui).
Registrado em `api/internal/api/v1/distributions.go`, mesmo grupo/padrão de
`RegisterDistributions`.

Body: `multipart/form-data`, campo `file` (XML), usando
`readOptionalUpload` (`api/internal/api/v1/helpers.go:69-85`) — tratando
ausência do campo como 400 (aqui o arquivo é obrigatório, diferente do uso
opcional em `organizations.go`).

Validações no service, antes de enfileirar (falha rápida, sem gastar
S3/SQS/quota SEFAZ):

1. Tamanho do arquivo — limite de 1 MiB (constante nova), acima disso 413.
2. Peek da tag raiz via `encoding/xml.Decoder` (só o primeiro
   `StartElement`, sem parse completo) — deve ser `nfeProc` ou `NFe`; senão
   400 Problem JSON. Parsing completo (emit/dest/transp, extração de
   produtos etc.) é responsabilidade do worker, não da API (separação de
   camadas do `worker/CLAUDE.md` — API valida e enfileira, lógica de negócio
   fica no worker).
3. `checkConsQuota` (mesmo reuso de `EnqueueLookupByKey`,
   `api/internal/services/distributions.go`) — a importação sempre dispara
   uma consulta protocolo real contra a SEFAZ, então conta contra a mesma
   cota de consumo.

Se tudo passar: upload do XML bruto para uma chave de staging em
`s.cfg.DocumentsBucket` (`{doc_type}-import-staging/{envPrefix}/{orgPK}/{uuid}.xml`),
publica SQS `{"job_type":"import_xml","org_pk":...,"doc_type":...,
"staging_key":...,"trigger":"user","triggered_at":...}` na fila
`distribution` já existente (novo campo `StagingKey` em
`DistributionMessage`, `worker/internal/service/distribution.go:144-151`),
retorna 202 `{"status":"enqueued"}`.

### Backend — worker

Novo case `"import_xml"` no switch de `Process`
(`worker/internal/service/distribution.go:168-184`) chamando
`runImportXML(ctx, msg.OrgPK, msg.DocType, msg.StagingKey, dtcfg)`.

Fluxo de `runImportXML`:

1. Baixa o XML do `staging_key` (S3 `GetObject`).
2. `parseXMLBytes` + valida raiz (`nfeProc` ou `NFe` bare) — revalida de
   verdade aqui, o peek da API é só uma otimização de fail-fast.
3. Extrai `emit`/`dest`/`transp.transporta` CNPJ/CPF e `chNFe` (mesmos
   `findEl`/`findText` de `distribution_parser.go`).
4. Classificação emit > dest > transp (seção acima). Sem match → rejeição
   de negócio (não retry): apaga staging, `notifyResult` de falha, retorna
   `nil`.
5. Idempotência: `GetItem` no `nfes` table por `docPK`+`accessKey` — se já
   existir com produtos (nota completa), rejeição de negócio (409-equivalente):
   apaga staging, `notifyResult` de falha, retorna `nil`.
6. Consulta protocolo via `go-dfe` (`godfe.Call`, `NfeConsultaProtocolo` já
   está em `dfe.Implements` — primeiro uso real desta operação em
   worker/api). Payload `consSitNFe` com `tpAmb`/`xServ=CONSULTAR`/`chNFe`
   (ordem de campos conforme `go-dfe/internal/xmlops/xsdorder/table.go:373`).
7. Comparação de digest:
   - Se raiz enviada é `nfeProc`: `protNFe/infProt/digVal` retornado pela
     SEFAZ deve bater com **ambos** o `protNFe/infProt/digVal` do XML
     enviado e o `Signature/SignedInfo/Reference/DigestValue` do XML
     enviado.
   - Se raiz enviada é `NFe` (sem protocolo): `digVal` retornado deve bater
     com o `Signature/SignedInfo/Reference/DigestValue` do XML enviado.
     Depois, monta o `nfeProc` final: pega o `protNFe` retornado pela
     consulta (dict), serializa para XML via
     `go-dfe/internal/xmlops/builder.go` (convenção `@attr`/`#text`),
     adiciona `xmlns="http://www.portalfiscal.inf.br/nfe"`, e envolve junto
     com o `NFe` original em um `nfeProc` — mesmo formato do exemplo de
     referência (`22260811647612000197550000000000501454670090.xml`).
   - Qualquer divergência → rejeição de negócio (não retry): apaga staging,
     `notifyResult` de falha ("divergência de assinatura"), retorna `nil`.
   - Erro de rede/timeout na consulta → retorna `error` (deixa SQS
     retry/DLQ tratar, regra padrão do `worker/CLAUDE.md`).
8. Com o `nfeProc` final (original ou montado), extrai campos completos
   (produtos, pagamentos, totais, partes) reaproveitando a extração de
   `extractProcNFe`, mas sobrescrevendo `Incoming` com o valor calculado no
   passo 4.
9. Persiste: `persistIncoming`, `persistCounterparties` (mesmas funções já
   existentes, `distribution.go:717,778`), eventos retornados pela consulta
   protocolo (`procEventoNFe[]`, se houver) via `persistEvent`
   (`distribution.go:866`). Upload do XML final para a chave canônica
   `{docType}/{envPrefix}/{orgPK}/{accessKey}.xml` (mesmo padrão de
   convergência já usado por emissão/distribuição,
   `distribution.go:552`). Apaga o objeto de staging. `notifyResult` de
   sucesso via SNS (mesmo fluxo WS → toast + invalidação de queries).

Nenhuma mudança de CDK — fila `distribution` e worker `distribution-worker`
já cobrem o job.

### Frontend — ui

- **NF-e**: novo botão "Importar XML" na aba de Distribuição
  (`ui/src/app/nfe/page.tsx`), ao lado do "Importar NF-e" existente (chave
  de acesso). Abre modal com seletor de arquivo (`.xml`), submit via
  multipart para `POST /distributions/nfe/import-xml`. Resultado chega pelo
  mesmo fluxo assíncrono via WebSocket (toast + invalidação de queries).
- **NFC-e**: opção equivalente, discreta (botão pequeno/ícone, não um botão
  primário de destaque), reaproveitando o mesmo componente de modal
  parametrizado por `doc_type=nfce`, postando para
  `POST /distributions/nfce/import-xml`.
- Construção de ambos via skill `impeccable`, conforme pedido do usuário.

## Fora de escopo (decisões explícitas)

- CT-e/MDF-e — não pedidos, sem conceito de importação por XML nesta
  feature.
- Validação XSD completa do XML — só validação estrutural via parser
  genérico existente; adicionar XSD é trabalho futuro se necessário.
- Reaproveitar `extractProcNFe` sem alteração para a classificação —
  função nova e específica para não arriscar quebrar o fluxo de
  distribuição normal (que nunca deveria produzir `Incoming = 0`).
- Nenhuma infraestrutura CDK nova.

## Fluxo de dados

```
Usuário seleciona XML no modal (NF-e ou NFC-e)
  → POST /distributions/{doc_type}/import-xml (multipart, campo "file")
  → api: valida tamanho, peek da tag raiz, checkConsQuota
      → upload staging S3 → publica SQS {job_type: import_xml, staging_key, ...} → 202
  → worker (distribution-worker, fila distribution): runImportXML
      → baixa staging, parse, classificação emit>dest>transp
      → checa nota completa já existente (idempotência)
      → NfeConsultaProtocolo via go-dfe
      → compara digest(s); se NFe bare, monta nfeProc final
      → persistIncoming, persistCounterparties, persistEvent (eventos da consulta)
      → upload XML final na chave canônica, apaga staging
      → notifyResult: publica SNS (sucesso ou falha)
  → api ResultsConsumer.dispatch → broadcast WS
  → frontend: toast + invalidação de queries
```

## Erros / rejeição

| Caso                                                    | Onde           | Tratamento                                             |
|----------------------------------------------------------|----------------|---------------------------------------------------------|
| Raiz XML inválida (não é `nfeProc`/`NFe`)                | api (peek) e worker (revalidação) | 400 Problem JSON (api) / rejeição de negócio sem retry (worker) |
| Arquivo acima do limite de tamanho                       | api            | 413                                                     |
| Nenhum CNPJ/CPF bate com a org (emit/dest/transp)        | worker         | Rejeição de negócio, sem retry, `notifyResult` de falha |
| NF-e completa já existe para a chave                     | worker         | Rejeição de negócio (409-equivalente), sem retry        |
| Divergência de digest (SEFAZ vs XML enviado)             | worker         | Rejeição de negócio, sem retry, `notifyResult` de falha |
| Erro de rede/timeout na consulta protocolo                | worker         | `error` retornado — SQS retry/DLQ normal                |
| SEFAZ retorna `cStat` de rejeição de negócio              | worker         | Sem retry, `notifyResult` de falha (regra padrão do `worker/CLAUDE.md`) |

## Testes

**api**

- Peek de tag raiz: casos válidos (`nfeProc`, `NFe`) e inválidos (outra tag,
  XML malformado) — 400 sem tocar S3/SQS.
- `doc_type` restrito a `nfe`/`nfce` — 400 para os demais.
- Limite de tamanho de arquivo — 413.
- Fluxo feliz: `checkConsQuota` chamado, upload de staging, formato da
  mensagem SQS, resposta 202 (mocks de S3/SQS).

**worker**

- Classificação emit > dest > transp: tabela de casos (emit bate, dest bate
  sem emit, transp bate sem emit/dest, nenhum bate → rejeição).
- Comparação de digest: caso `nfeProc` (dois digests a comparar) e caso
  `NFe` bare (um digest, join do `protNFe`), usando como fixture o XML de
  exemplo (`22260811647612000197550000000000501454670090.xml`) e variações
  com digest alterado.
- Join do `protNFe` retornado pela consulta em XML com `xmlns` — round-trip
  contra a fixture.
- Rejeição de nota já completa.
- Integração: fluxo completo de `runImportXML` com `go-dfe`/py-dfe mockado,
  verificando persistência, upload S3, `notifyResult`, e idempotência
  (mensagem duplicada não duplica o documento — mesma proteção de
  `ConditionExpression` já usada em `processDocZip`).
- Regressão: rejeição de negócio da SEFAZ (`cStat` != 100) não é
  reprocessada (retorna `nil`, sem retry).

## Documentação a atualizar (pós-implementação)

- `DOCS.md`: novos contratos `POST /distributions/nfe/import-xml` e
  `POST /distributions/nfce/import-xml`, e primeira menção de uso real de
  `NfeConsultaProtocolo` via `go-dfe`.
- `CONDUCT.md`: nota sobre a chave de staging em S3 (padrão novo,
  `{doc_type}-import-staging/...`) e sobre a classificação
  emit/dest/transp ser específica desta feature (não uma mudança de
  comportamento em `extractProcNFe`).
