# Migração incremental py-dfe → go-dfe

## Context

py-dfe (Python Lambda: XML-DSig + SOAP SEFAZ + mTLS para NF-e/NFC-e/CT-e/MDF-e) está em produção e funcionando. Usuário
quer migrar para Go por performance/segurança, mas via biblioteca nova (`go-dfe`) trocada função por função — não
rewrite completo de uma vez. `MIGRATION.md` do repo já tinha avaliado e rejeitado rewrite total imediato (risco de
canonicalização XML), condicionando a um cenário futuro com ≥3 meses estáveis + 10k+ documentos validados em
homologação. Este plano adota caminho intermediário: migração incremental por operação, com fallback automático pro
py-dfe Lambda em qualquer operação ainda não portada, e gate de validação forte (byte-identical) antes de qualquer corte
de tráfego em operação que assina XML.

Decisões confirmadas com usuário:

- **DANFE/DAMDFE (renderização PDF) excluído permanentemente do escopo.** Sem certificado/assinatura/SOAP/mTLS, sem
  ganho fiscal ou de segurança — só custo (WeasyPrint sem equivalente Go). py-dfe continua vivo só pra isso,
  indefinidamente.
- **Validação XSD não portada por enquanto.** `CGO_ENABLED=0` é regra fixa em worker/api/cdk (build ARM64 Lambda
  `provided.al2023`) — única opção completa (libxml2) exige cgo. Sem validador XSD maduro em Go puro. Operações iniciais
  (status/consulta) têm payload trivial; SEFAZ valida server-side. Reavaliar só se operação assinada precisar
  `validate_schema=true`.

## Arquitetura

**Novo módulo Go** `go-dfe/` na raiz do monorepo (irmão de `worker/`, `api/`), module path
`gopkg.aoctech.app/dfe/go-dfe`, `go 1.27`.

```
go-dfe/
  go.mod
  dfe.go                      # Call(ctx, Request) (Response, error) — dispatch central
  request.go                  # Request/Response/Problem — mesmo contrato JSON de LambdaRequest/LambdaResponse
  internal/
    certificate/manager.go    # PKCS12 -> tls.Certificate + http.Client mTLS (InsecureSkipVerify:true — SEFAZ não valida server chain, propositalmente igual ao py-dfe)
    xmlops/
      builder.go              # dict<->XML (convenções @xmlns/@key/#text)
      signer.go               # XML-DSig: rsa-sha1 + sha1 + C14N puro (sem prefixo), fix newline X509Certificate
      xsdorder/table.go        # porte 1:1 de xsd_order.py (~640 linhas, dado puro)
    soap/envelope.go           # SOAP 1.2 envelope build/parse
    services/
      config.go                # ServiceConfig por doc_type (signature/validation sets, sign_id_xpath)
      client.go                # SefazClient.Call: prepara payload -> resolve endpoint -> POST+retry -> parse resposta
      nf.go cte.go mdfe.go      # facades finos por tipo de documento
    endpoints/table.go          # (doc_type, uf, env, service) -> URL (redirecionamentos SVRS/SVAN)
    constants/constants.go      # enums, timeouts, retry defaults
  testdata/                     # fixtures de XML assinado capturadas do py-dfe (corpus de teste)
```

**Dependência worker/api → go-dfe**: `go.work` na raiz listando `./worker ./api ./go-dfe`. Verificar em fase 0 se build
Docker/CDK (contexto de build do `api/Dockerfile` e `cdk/lib/worker-stack.ts`) enxerga a raiz do repo — se não der,
fallback é `replace` no go.mod de cada consumidor.

**Signer (parte mais arriscada)**: não usar `goxmldsig` (feito pra C14N exclusivo + algoritmos modernos, hostil aos
requisitos legados do SEFAZ). Porte à mão sobre stdlib: `crypto/rsa.SignPKCS1v15` + `crypto/sha1` (determinístico) +
implementação manual de Canonical XML 1.0 puro (`REC-xml-c14n-20010315` — o arquivo mais arriscado de todo o projeto,
sem lib pronta). Assinatura envelope + `reference_uri` + fix de newline no X509Certificate são triviais uma vez que o
C14N estiver correto.

**Mecanismo de corte incremental** (sem sistema de feature flag novo, seguindo precedente do repo de "dual-path +
fallback temporário"):

```go
// go-dfe expõe:
func Implements(docType, service string) bool // set compilado, ex: "nfe/NfeStatusServico"
func Call(ctx context.Context, req Request) (Response, error)
```

Em `worker/internal/service/dfe.go:355` (`invokePyDfe`) e `distribution.go:1070`:

```go
if dfe.Implements(payload.DocType, payload.Service) {
return s.callGoDfe(ctx, payload) // in-process, adapta pra lambdaResponse existente
}
// path Lambda invoke inalterado abaixo
```

Depois, mesmo tratamento em `api/internal/services/external.go:333`. Promover operação = adicionar chave no set.
Reverter = remover a chave (fallback automático pro Lambda). `DfeLambdaName`/cliente Lambda ficam até a última operação
ser cortada.

## Ordem de fases (risco crescente)

- **Fase 0 — Scaffolding**: módulo, `go.work`, verificar build Docker/CDK enxerga workspace, contratos
  Request/Response/Problem, cliente mTLS via cert (reusa `go-pkcs12` já presente em `api/`), SOAP envelope,
  `SefazClient.Call` com retry (só 500/502/503/504 + timeout/rede — nunca 4xx nem rejeição de negócio). Nenhuma operação
  ligada ainda. **Pronto quando**: chamada de status sem assinatura funciona ponta a ponta contra homologação SEFAZ,
  build verde com `CGO_ENABLED=0 GOARCH=arm64`.
- **Fase 1 — Status (menor risco, sem assinatura)**: `NfeStatusServico`, `CTeStatusServico`, `MDFeStatusServico`.
- **Fase 2 — Consulta (sem assinatura)**: `NfeConsultaProtocolo`, `NfeConsultaCadastro`, `CTeConsulta`, `MDFeConsulta`,
  `MDFeConsNaoEnc`.
- **Fase 3 — Distribuição (ambiente nacional AN, sem assinatura)**: `NFeDistribuicaoDFe`, `CTeDistribuicaoDFe`,
  `MDFeDistribuicaoDFe`. Também liga o seam do `api/` (`external.go`) pra `LookupByNSU`/`LookupByKey`.
- **Fase 4 — Signer + operações assinadas (maior risco fiscal, gate mais forte)**: construir `signer.go` + C14N, validar
  com teste byte-idêntico (ver abaixo) antes de qualquer corte. Depois, uma operação por vez: `RecepcaoEvento`/
  `CTeRecepcaoEvento`/`MDFeRecepcaoEvento`, `NfeInutilizacao`, depois grupo síncrono `NFeAutorizacao`,
  `CTeRecepcaoSinc`/`RecepcaoOS`/`RecepcaoGTVe`/`RecepcaoSimp`, `MDFeRecepcaoSinc`.

## Gate de validação por fase

- **Shadow mode (fases 1-3)**: seam chama Lambda (autoritativo, resultado usado) E, em goroutine que não afeta
  resposta/erro, chama `go-dfe`, compara, loga divergência. Promover só depois de janela de paridade limpa em produção.
- **Gate de assinatura (fase 4, o forte)**: RSA-SHA1/PKCS#1v1.5 é determinístico dado mesma chave+mensagem → py-dfe e
  go-dfe DEVEM produzir XML assinado byte-idêntico pro mesmo input canonicalizado + mesmo certificado. Teste
  automatizado table-driven em `go-dfe/testdata/`: captura output assinado do py-dfe pra corpus de documentos reais
  (certificado de teste dedicado, nunca certificado real de cliente), compara byte a byte com saída do go-dfe. Tem que
  passar antes de qualquer corte de tráfego em operação assinada. Isola bug de C14N/signer de falha intermitente de
  rede/SEFAZ. Manter bar do `MIGRATION.md` (10k+ docs em homologação) como critério pra fase 4.

## Testes (convenção do repo: integração por função core de service, `go test ./... -race`)

- Unit go-dfe: round-trip builder, tabela xsd_order, build/parse SOAP, resolução de endpoint (redirecionamentos
  SVRS/SVAN), classificação de retry, vetores de canonicalização C14N, teste de assinatura byte-idêntica.
- Integração go-dfe: chamadas gravadas/replay contra homologação por operação implementada, mais teste live opcional via
  env/build tag.
- Seam worker/api: teste comprovando (a) operação no set roteia pro go-dfe, (b) operação fora do set cai no Lambda, (c)
  divergência em shadow mode é logada sem alterar resultado autoritativo.

## Documentação a atualizar (Mandatory Workflow do CLAUDE.md raiz)

- **`worker/CLAUDE.md:88,122`** — guardrail atual ("não reescrever/bypassar py-dfe") precisa ser reescrito, não
  ignorado: nova redação "go-dfe é o caminho sancionado in-process para operações no seu set implementado; py-dfe Lambda
  é fallback pro resto e caminho permanente pra renderização DANFE".
- **`MIGRATION.md`** — adicionar entrada datada: organização optou por caminho intermediário (migração incremental por
  operação) entre Cenário A e B; gate byte-idêntico + shadow mode é como o risco de canonicalização (linha 46 do doc) é
  mitigado por operação; DANFE fora de escopo permanentemente. Não apagar racional antigo, superseder.
- **`DOCS.md`/`OVERVIEW.md`** — nova entrada do módulo go-dfe (propósito, layout, dependência via go.work, modelo de
  dispatch implemented-set/fallback, corte de escopo XSD/DANFE).
- **`go-dfe/CLAUDE.md`** novo, seguindo convenção por-módulo já existente em worker/api.

## Arquivos críticos

- `worker/internal/service/dfe.go` (seam :355, contrato `lambdaPayload`/`lambdaResponse` :94/:105)
- `worker/internal/service/distribution.go` (seam :1070, variante `map[string]any`)
- `api/internal/services/external.go` (seam :333) e `api/internal/services/certificates.go` (base `go-pkcs12` reusável)
- `py-dfe/py_dfe/xmlops/signer.py` (alvo de porte byte-a-byte) e `xsd_order.py`
- `MIGRATION.md`, `worker/CLAUDE.md` (docs que precisam mudar)

## Verificação

- Fase 0: build `CGO_ENABLED=0 GOARCH=arm64 go build ./...` verde em go-dfe; chamada de status contra homologação SEFAZ
  retorna cStat esperado.
- Cada fase 1-3: `go test ./... -race` verde; shadow mode sem divergência por janela definida antes de promover.
- Fase 4: teste byte-idêntico de assinatura verde offline antes de qualquer corte; shadow mode + volume de homologação
  (10k+, conforme MIGRATION.md) antes de flip de operação assinada.
