# go-dfe-egress (sa-east-1) e remoção do py-dfe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Originar toda chamada a SEFAZ/prefeitura de um Lambda em sa-east-1 (`{env}-go-dfe-egress`) chamado por worker e API, e remover o py-dfe.

**Architecture:** Novo módulo `go-dfe-egress/` com um handler Lambda que só executa `dfe.Call` (sem AWS). Worker e API trocam o `dfe.Call` em processo/Lambda py-dfe por `Invoke` síncrono desse Lambda (client Lambda com região de sa-east-1). CDK ganha uma stack em sa-east-1; o py-dfe sai por último, depois de validado em produção.

**Tech Stack:** Go 1.27 (`go.work`), `aws-lambda-go`, `aws-sdk-go-v2/service/lambda`, AWS CDK (TypeScript), GitHub Actions.

**Spec:** `docs/specs/2026-10-06-go-dfe-egress-design.md`

## Global Constraints

- Plans/specs ficam em `docs/plans/` e `docs/specs/` (nunca `superpowers/`).
- Nenhum trailer de atribuição, nenhuma menção a Claude/Anthropic em commits, PRs ou arquivos. Commits Conventional, sem emojis.
- `CGO_ENABLED=0 GOARCH=arm64` deve compilar limpo em todos os módulos Go.
- Constantes nomeadas: nenhuma string/código/URL/nome de header solto (regra do repo).
- Erros em `api`/`worker`: `problem.*` (RFC 7807); nunca erro cru.
- Nunca logar o payload do egress (contém PFX e senha em base64).
- Nunca commitar PFX, senhas, credenciais AWS ou CNPJs reais.
- Região do egress: `sa-east-1`. Nome da função: `{env}-go-dfe-egress`. Variável de região: `DFE_EGRESS_REGION`. Nome da função continua em `DFE_LAMBDA_NAME` (worker) e `SEFAZ_FUNCTION_NAME` (API).
- Contrato do egress = `go-dfe/request.go` (`Request`/`Response`/`Problem`), idêntico ao contrato histórico do py-dfe.
- Cadeia de timeouts: `timeout do worker/API > timeout do egress > pior caso das tentativas do go-dfe`.
- Toda mudança de comportamento atualiza a documentação na mesma tarefa (`DOCS.md`, `OVERVIEW.md`, `MIGRATION.md`, `CONDUCT.md`, `DEPLOYMENT.md`, `CLAUDE.md`/`AGENTS.md` do subprojeto afetado).

## Review Focus

- Worker envia `environment` como `"producao"`/`"homologacao"` (py-dfe normalizava); o egress deve aceitar isso e converter para `"prod"`/`"hom"`, senão toda chamada falha com ambiente desconhecido.
- NFS-e chega com `uf` vazio (competência municipal): o egress não pode rejeitá-lo.
- `FunctionError` do Invoke (crash/timeout do egress) deve virar erro retryável no worker e `problem` 500 na API, nunca sucesso silencioso nem corpo vazio.
- O corpo do `Request` nunca pode aparecer em log do egress, nem em log de erro.
- Falha de conexão ao fisco deve estourar em segundos (connect timeout), não em ~80 s como em 2026-10-06.
- Referência cross-region no CDK só por string (nome/ARN calculados); um token entre regiões quebra o `synth`.

---

## File Structure

| Arquivo | Responsabilidade |
|---|---|
| `go-dfe-egress/main.go` | `lambda.Start` + wiring (único arquivo com `main`) |
| `go-dfe-egress/handler.go` | `newHandler(call)`: normaliza ambiente, chama `dfe.Call`, loga só metadados |
| `go-dfe-egress/handler_test.go` | testes do handler |
| `go-dfe/internal/constants/constants.go` | + `DialTimeout`, `NFSeAttemptTimeout` |
| `go-dfe/internal/certificate/manager.go` | `DialContext` com `DialTimeout` |
| `go-dfe/nfse/nacional/transport.go` | timeout por tentativa em `httpDo` |
| `cdk/lib/go-code.ts` | `goCode(moduleDir, cmdPath)` extraído de `worker-stack.ts` |
| `cdk/lib/egress.ts` | `EGRESS_REGION`, `egressFunctionName`, `egressFunctionArn` (DRY entre stacks) |
| `cdk/lib/egress-stack.ts` | `GoDfeEgressStack` (sa-east-1) |
| `worker/internal/service/*` | `invokeEgress` único; remove desvio in-process e shadow |
| `api/internal/services/external.go` | `CallDfe` + remove shadow; `nfses/municipal.go` usa `CallDfe` |

---

### Task 1: Remover tudo do py-dfe de dentro do go-dfe (comentários, docs e código)

**Files:**
- Modify: todos os `.go` e `.md` de `go-dfe/` que casam `rg -i "py-?dfe|py_dfe|pydfe|python"` (≈190 ocorrências; lista em `rg -c` — maiores: `internal/endpoints/table.go`, `internal/xmlops/signer.go`, `CLAUDE.md`, `internal/services/response.go`, `internal/services/client.go`, `internal/xmlops/builder.go`, `dfe.go`, `README.md`).
- Delete: `go-dfe/shadow.go`, `go-dfe/shadow_test.go` (comparação com o py-dfe; decisão do dono: o py-dfe não é usado por ninguém).
- Delete: `api/internal/services/godfe_shadow.go`; Modify: `api/internal/services/external.go` (remover a chamada `shadowCallGoDfeFromMap(...)` em `invokeSefazLambda`).
- Modify: `go-dfe/dfe.go` (mensagem de erro e doc de `Implements`), `go-dfe/internal/soap/envelope.go:68` (comentário), `go-dfe/CLAUDE.md`, `go-dfe/README.md`.

**Interfaces:**
- Produces: go-dfe sem nenhuma referência ao py-dfe (código, comentário, doc, teste) e sem `ShadowCompare`; `rg -i "py-?dfe|py_dfe|pydfe" go-dfe` vazio. Único código executável alterado: remoção do shadow e a mensagem de erro de `Call`.

Regras de reescrita (aplicar a cada ocorrência, sem alterar código executável):

1. Comentário que cita um arquivo/símbolo do py-dfe como **fonte de fidelidade** (`py-dfe/py_dfe/services/base.py: MAX_RETRIES`) → trocar por `histórico (removido em 2026-10; ver git: py-dfe/py_dfe/services/base.py)` **uma vez por arquivo**, e nas demais ocorrências do mesmo arquivo descrever o comportamento em si, sem citar o caminho.
2. Comentário "mirrors py-dfe's X" → descrever X diretamente ("retry apenas em 5xx/erro de rede, backoff exponencial"), sem citar origem.
3. `request.go` e `dfe.go`: o pacote não "substitui o py-dfe"; descrever como "cliente SEFAZ/NFS-e chamado pelo Lambda go-dfe-egress". `Request`/`Response`/`Problem` documentados como contrato do egress.
4. Mensagens de erro/strings: `dfe: %s/%s not implemented in go-dfe — caller must use the py-dfe Lambda fallback` → `dfe: %s/%s is not supported`. Atualizar o teste que a assertar (`rg "py-dfe Lambda fallback" go-dfe`).
5. Testes (`*_test.go`) com "cross-checked against py-dfe": trocar por "valores de referência fixados" mantendo os valores.
6. `CLAUDE.md`/`README.md` do go-dfe: remover seções de "fidelidade ao py-dfe" que mandam abrir o código do py-dfe; manter a regra "não consolidar tabelas portadas sem checar a origem" apontando para o histórico do git. Atualizar a descrição do papel: biblioteca executada em processo **pelo go-dfe-egress**.
7. Código: excluir `shadow.go`/`shadow_test.go` (go-dfe) e `api/internal/services/godfe_shadow.go`, e a linha `shadowCallGoDfeFromMap(ctx, payload, statusCode, bodyStr)` de `invokeSefazLambda`. `Implements` permanece como allowlist (usada por `Call`); a doc deixa de falar em "caller escolhe entre dfe.Call e Lambda".
8. `dfe.go` — comentário da `implemented`: remover a prosa de promoção/shadow/fallback; o mapa passa a ser a **allowlist de operações aceitas por `Call`**. Manter o aviso de que operações assinadas não passaram pelo portão byte-idêntico (fato histórico relevante).

- [ ] **Step 1: Medir a linha de base**

Run: `cd go-dfe && rg -i -c "py-?dfe|py_dfe|pydfe|python" --glob '*.go' --glob '*.md' . | sort -t: -k2 -nr > /tmp/pydfe-before.txt; CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./... 2>&1 | tail -15`
Expected: build limpo, testes PASS (anotar o resultado).

- [ ] **Step 2: Aplicar as regras 1–7 arquivo a arquivo**

Para cada arquivo de `/tmp/pydfe-before.txt`, aplicar as regras. Fora o código listado em "Files" (shadow e mensagem de erro), não alterar identificadores, tabelas ou lógica.

- [ ] **Step 3: Verificar que não resta nada**

Run: `cd go-dfe && rg -i -l "py-?dfe|py_dfe|pydfe" . ; rg -n "ShadowCompare|shadowCallGoDfe" /home/artur-revgas/Documents/Projects/Ctech/ctech-dfe --glob '*.go'`
Expected: ambos vazios.

- [ ] **Step 4: Confirmar que nada mudou em comportamento**

Run: `cd go-dfe && git diff --stat && git diff -U0 -- '*.go' | rg '^[+-]' | rg -v '^(\+\+\+|---)' | rg -v '^[+-]\s*(//|\*)' | head -40`
Expected: as únicas linhas não-comentário alteradas são a mensagem de erro de `dfe.go` (regra 4), o teste que a assertava e as remoções do shadow.

- [ ] **Step 5: Build e testes (go-dfe, worker e API)**

Run: `cd go-dfe && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./... -count=1 && cd ../worker && go test ./... -count=1 && cd ../api && go vet ./... && go test ./... -count=1`
Expected: build limpo, todos PASS.

- [ ] **Step 6: Commit**

```bash
git add go-dfe api
git commit -m "refactor(go-dfe): remove every py-dfe reference and the shadow comparison"
```

---

### Task 2: Connect timeout e timeout por tentativa no NFS-e

**Files:**
- Modify: `go-dfe/internal/constants/constants.go`
- Modify: `go-dfe/internal/certificate/manager.go:84-91`
- Modify: `go-dfe/nfse/nacional/transport.go` (função `httpDo`, laço de tentativas)
- Test: `go-dfe/internal/certificate/manager_test.go`, `go-dfe/nfse/nacional/transport_test.go`

**Interfaces:**
- Produces: `constants.DialTimeout time.Duration` (10 s) e `constants.NFSeAttemptTimeout time.Duration` (20 s) — usados pelo transport e por `httpDo`.

Motivo: em 2026-10-06 uma invocação levou 80,3 s esperando o `connect` do SO (o `http.Client` do NFS-e não tem timeout algum). Com `DefaultMaxRetries = 3` (4 tentativas) e 20 s por tentativa o pior caso é 80 s + ~10 s de backoff (jitter incluso); o egress (Task 4) usa 120 s e os workers que chamam o egress passam a ter timeout de 150 s (Task 5).

- [ ] **Step 1: Escrever o teste do transport**

`go-dfe/internal/certificate/manager_test.go` (acrescentar; usar o helper de PFX de teste já existente no arquivo — ver como `TestLoad...` constrói o `.pfx`):

```go
func TestLoadSetsDialTimeout(t *testing.T) {
	client, _, _, err := Load(testPFXB64(t), testPFXPassword)
	if err != nil {
		t.Fatal(err)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport = %T, want *http.Transport", client.Transport)
	}
	if tr.DialContext == nil {
		t.Fatal("DialContext is nil: connect has no timeout")
	}
}
```

(`testPFXB64`/`testPFXPassword`: usar os helpers/constantes que o arquivo de teste já define; se o nome for outro, usar o existente — `rg "func .*PFX|pfx" go-dfe/internal/certificate/*_test.go`.)

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd go-dfe && go test ./internal/certificate -run TestLoadSetsDialTimeout -count=1`
Expected: FAIL `DialContext is nil`.

- [ ] **Step 3: Constantes e transport**

`internal/constants/constants.go` (junto das constantes de retry):

```go
// Timeouts de rede do cliente HTTP mTLS. DialTimeout limita só o TCP connect:
// uma prefeitura que descarta SYN deve falhar em segundos, não no timeout do SO
// (~127 s no Linux). NFSeAttemptTimeout limita cada tentativa REST do NFS-e.
const (
	DialTimeout        = 10 * time.Second
	NFSeAttemptTimeout = 20 * time.Second
)
```

`internal/certificate/manager.go`:

```go
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: constants.DialTimeout}).DialContext,
			TLSClientConfig: &tls.Config{
```

Adicionar `"net"` e `constants` aos imports (se `constants` ainda não for importado ali).

- [ ] **Step 4: Teste da tentativa com timeout**

`go-dfe/nfse/nacional/transport_test.go`:

```go
func TestHTTPDoAttemptTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // nunca responde
	}))
	defer srv.Close()

	orig := attemptTimeout
	attemptTimeout = 50 * time.Millisecond
	defer func() { attemptTimeout = orig }()

	start := time.Now()
	_, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, nil, 0)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("took %v, attempt timeout not applied", elapsed)
	}
}
```

- [ ] **Step 5: Rodar e ver falhar**

Run: `cd go-dfe && go test ./nfse/nacional -run TestHTTPDoAttemptTimeout -count=1`
Expected: FAIL (compilação: `attemptTimeout` indefinido).

- [ ] **Step 6: Implementar em `httpDo`**

Em `transport.go`, variável de pacote (testável) e uso por tentativa:

```go
// attemptTimeout é var (não const) só para o teste reduzi-lo.
var attemptTimeout = constants.NFSeAttemptTimeout
```

No laço, trocar a criação da requisição:

```go
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		req, err := http.NewRequestWithContext(attemptCtx, method, url, reader)
		if err != nil {
			cancel()
			return 0, fmt.Errorf("nacional: build request: %w", err)
		}
```

e chamar `cancel()` depois de **ler o corpo** (o `io.ReadAll` ocorre depois do `client.Do`): inserir `cancel()` logo após `_ = resp.Body.Close()` no caminho normal, e `cancel()` antes de cada `continue`/`return` que sai do laço antes disso (incluindo o ramo `err != nil` de `client.Do`). Garantir que não vaza contexto: `go vet ./...` não deve acusar `lostcancel`.

- [ ] **Step 7: Rodar tudo**

Run: `cd go-dfe && go vet ./... && go test ./... -count=1`
Expected: PASS, sem `lostcancel`.

- [ ] **Step 8: Documentar e commitar**

`CONDUCT.md`: acrescentar a regra "NFS-e: connect 10 s, 20 s por tentativa; a cadeia de timeouts do egress depende desses valores". `DOCS.md` (seção go-dfe): citar as duas constantes.

```bash
git add go-dfe CONDUCT.md DOCS.md
git commit -m "fix(go-dfe): bound NFS-e connect and per-attempt timeouts"
```

---

### Task 3: Módulo `go-dfe-egress`

**Files:**
- Create: `go-dfe-egress/go.mod`, `go-dfe-egress/main.go`, `go-dfe-egress/handler.go`, `go-dfe-egress/handler_test.go`, `go-dfe-egress/CLAUDE.md`
- Modify: `go.work` (adicionar `./go-dfe-egress`)

**Interfaces:**
- Consumes: `dfe.Call(ctx, dfe.Request) (dfe.Response, error)` e os tipos de `go-dfe/request.go`.
- Produces: função Lambda que recebe `dfe.Request` (JSON) e devolve `dfe.Response`; `normalizeEnvironment(string) string`.

- [ ] **Step 1: Criar o módulo e o workspace**

`go-dfe-egress/go.mod`:

```
module gopkg.aoctech.app/dfe/go-dfe-egress

go 1.27

require (
	github.com/aws/aws-lambda-go v1.55.0
	gopkg.aoctech.app/dfe/go-dfe v0.0.0
)
```

`go.work`:

```
use (
	./api
	./go-dfe
	./go-dfe-egress
	./worker
)
```

- [ ] **Step 2: Escrever os testes**

`go-dfe-egress/handler_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

func TestNormalizeEnvironment(t *testing.T) {
	cases := map[string]string{
		"producao": "prod", "homologacao": "hom", "prod": "prod", "hom": "hom", "": "",
	}
	for in, want := range cases {
		if got := normalizeEnvironment(in); got != want {
			t.Errorf("normalizeEnvironment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHandlerNormalizesEnvironmentAndKeepsEmptyUF(t *testing.T) {
	var got dfe.Request
	h := newHandler(func(_ context.Context, req dfe.Request) (dfe.Response, error) {
		got = req
		return dfe.Response{StatusCode: 200, Body: "{}"}, nil
	})

	resp, err := h(context.Background(), dfe.Request{
		Environment: "homologacao", DocType: "nfse", Service: "NFSeRecepcao", UF: "",
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if got.Environment != "hom" {
		t.Errorf("environment = %q, want hom", got.Environment)
	}
	if got.UF != "" {
		t.Errorf("uf = %q, want empty (NFS-e is municipal)", got.UF)
	}
}

func TestHandlerPropagatesCallError(t *testing.T) {
	want := errors.New("boom")
	h := newHandler(func(context.Context, dfe.Request) (dfe.Response, error) { return dfe.Response{}, want })
	if _, err := h(context.Background(), dfe.Request{DocType: "nfe", Service: "x"}); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestHandlerNeverLogsPayload(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	h := newHandler(func(context.Context, dfe.Request) (dfe.Response, error) {
		return dfe.Response{}, errors.New("falhou")
	})
	_, _ = h(context.Background(), dfe.Request{
		CertificateB64: "SEGREDO-PFX", CertificatePassword: "SEGREDO-SENHA",
		DocType: "nfe", Service: "NfeStatusServico", UF: "SP", Environment: "prod",
	})
	if out := buf.String(); strings.Contains(out, "SEGREDO") {
		t.Fatalf("log contains secret material: %s", out)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd go-dfe-egress && go mod tidy && go test ./... -count=1`
Expected: FAIL (compilação: `newHandler`/`normalizeEnvironment` indefinidos).

- [ ] **Step 4: Implementar**

`go-dfe-egress/handler.go`:

```go
// Package main é o Lambda go-dfe-egress: executa dfe.Call em sa-east-1 para que
// toda chamada a SEFAZ/prefeitura saia de IP brasileiro. Não acessa nenhum
// serviço AWS e nunca loga o corpo da requisição (contém PFX e senha).
package main

import (
	"context"
	"log/slog"
	"time"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

const (
	envProdLong = "producao"
	envHomLong  = "homologacao"
	envProd     = "prod"
	envHom      = "hom"

	logMsgCall    = "egress call"
	logFieldType  = "doc_type"
	logFieldSvc   = "service"
	logFieldUF    = "uf"
	logFieldCode  = "status_code"
	logFieldMS    = "duration_ms"
	logFieldError = "error"
)

// callFunc é dfe.Call; injetável para teste.
type callFunc func(context.Context, dfe.Request) (dfe.Response, error)

// normalizeEnvironment aceita as formas longas que worker/API sempre enviaram
// ao py-dfe ("producao"/"homologacao") e devolve a forma que dfe.Call espera.
func normalizeEnvironment(env string) string {
	switch env {
	case envProdLong:
		return envProd
	case envHomLong:
		return envHom
	default:
		return env
	}
}

func newHandler(call callFunc) func(context.Context, dfe.Request) (dfe.Response, error) {
	return func(ctx context.Context, req dfe.Request) (dfe.Response, error) {
		req.Environment = normalizeEnvironment(req.Environment)
		start := time.Now()
		resp, err := call(ctx, req)
		attrs := []any{
			logFieldType, req.DocType, logFieldSvc, req.Service, logFieldUF, req.UF,
			logFieldCode, resp.StatusCode, logFieldMS, time.Since(start).Milliseconds(),
		}
		if err != nil {
			attrs = append(attrs, logFieldError, err.Error())
		}
		slog.Info(logMsgCall, attrs...)
		return resp, err
	}
}
```

`go-dfe-egress/main.go`:

```go
package main

import (
	"github.com/aws/aws-lambda-go/lambda"

	dfe "gopkg.aoctech.app/dfe/go-dfe"
)

func main() {
	lambda.Start(newHandler(dfe.Call))
}
```

- [ ] **Step 5: Rodar e ver passar; build de produção**

Run: `cd go-dfe-egress && go mod tidy && go test ./... -count=1 -race && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o /tmp/egress-bootstrap .`
Expected: PASS e binário gerado. (Se `-race` exigir C toolchain indisponível, rodar sem `-race` localmente; CI roda com.)

- [ ] **Step 6: `CLAUDE.md` do módulo, docs e commit**

`go-dfe-egress/CLAUDE.md`: papel (Lambda sa-east-1), contrato (`go-dfe/request.go`), regras: sem AWS, sem log de payload, sem lógica fiscal própria, build `CGO_ENABLED=0 GOARCH=arm64`. `CLAUDE.md` raiz: nova linha na tabela de projetos. `OVERVIEW.md`/`DOCS.md`: novo componente (preenchidos de verdade na Task 10; aqui só o `CLAUDE.md` raiz e do módulo).

```bash
git add go.work go.work.sum go-dfe-egress CLAUDE.md
git commit -m "feat(egress): add go-dfe-egress Lambda module"
```

---

### Task 4: CDK — stack do egress em sa-east-1 (aditivo)

**Files:**
- Create: `cdk/lib/go-code.ts`, `cdk/lib/egress.ts`, `cdk/lib/egress-stack.ts`, `cdk/test/egress-stack.test.ts`
- Modify: `cdk/lib/worker-stack.ts` (usar `go-code.ts`; ARN e env do egress), `cdk/lib/iam-stack.ts`, `cdk/lib/api-stack.ts`, `cdk/bin/ctech-dfe-cdk.ts`, `cdk/test/worker-stack.test.ts`
- Não remover o `DfeStack` aqui (Task 10).

**Interfaces:**
- Produces: `EGRESS_REGION = 'sa-east-1'`, `EGRESS_TIMEOUT_SECONDS = 120`, `egressFunctionName(env: Environment): string`, `egressFunctionArn(env: Environment, account: string): string`, `goCode(moduleDir: string, cmdPath: string): lambda.AssetCode`, `class GoDfeEgressStack`.

- [ ] **Step 1: Extrair `goCode` sem mudar comportamento**

`cdk/lib/go-code.ts`: mover `resolveGo` e `goCode` de `worker-stack.ts`, parametrizando o diretório do módulo e o caminho do `main`:

```ts
import * as lambda from 'aws-cdk-lib/aws-lambda'
import {spawnSync} from 'child_process'
import path from 'node:path'

function resolveGo(): string { /* corpo idêntico ao atual de worker-stack.ts */ }

// goCode builds a Go Lambda binary from moduleDir. cmdPath is the package to
// build, relative to moduleDir ('./cmd/worker' for the worker, '.' for the egress).
export function goCode(moduleDir: string, cmdPath: string): lambda.AssetCode {
  return lambda.Code.fromAsset(moduleDir, {
    bundling: {
      local: {
        tryBundle(outputDir: string): boolean {
          const r = spawnSync(
            resolveGo(),
            ['build', '-tags', 'lambda.norpc', '-ldflags', '-s -w', '-o', path.join(outputDir, 'bootstrap'), cmdPath],
            {cwd: moduleDir, env: {...process.env, GOOS: 'linux', GOARCH: 'arm64', CGO_ENABLED: '0'}, stdio: ['ignore', 'pipe', 'pipe']},
          )
          if (r.status !== 0) process.stderr.write(r.stderr ?? Buffer.alloc(0))
          return r.status === 0
        },
      },
      image: lambda.Runtime.PROVIDED_AL2023.bundlingImage,
      environment: {GOCACHE: '/tmp/go-build', GOPATH: '/tmp/go'},
      command: ['bash', '-c', `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags '-s -w' -o /asset-output/bootstrap ${cmdPath}`],
    },
  })
}
```

Em `worker-stack.ts`: remover `resolveGo`/`goCode` locais; importar `{goCode}` e trocar `goCode(workerCmd)` por `goCode(CTECH_WORKER_DIR, \`./cmd/${workerCmd}\`)`.

- [ ] **Step 2: Verificar que o worker não mudou**

Run: `cd cdk && npx tsc --noEmit && npx jest test/worker-stack.test.ts`
Expected: PASS.

- [ ] **Step 3: `egress.ts`**

```ts
import {Environment} from './types'

export const EGRESS_REGION = 'sa-east-1'

// Chain: callers (150 s) > egress (120 s) > NFS-e worst case (4 x 20 s + backoff).
export const EGRESS_TIMEOUT_SECONDS = 120

export const egressFunctionName = (environment: Environment): string => `${environment}-go-dfe-egress`

// Cross-region: built from strings, never from a stack token (CloudFormation
// cannot reference a resource in another region).
export const egressFunctionArn = (environment: Environment, account: string): string =>
  `arn:aws:lambda:${EGRESS_REGION}:${account}:function:${egressFunctionName(environment)}`
```

- [ ] **Step 4: Teste da stack (falhando)**

`cdk/test/egress-stack.test.ts`:

```ts
import * as cdk from 'aws-cdk-lib'
import {Template} from 'aws-cdk-lib/assertions'
import {GoDfeEgressStack} from '../lib/egress-stack'
import {EGRESS_REGION, EGRESS_TIMEOUT_SECONDS, egressFunctionName} from '../lib/egress'

function build(): Template {
  const app = new cdk.App()
  const stack = new GoDfeEgressStack(app, 'TestEgress', {
    env: {account: '123456789012', region: EGRESS_REGION},
    environment: 'dev',
  })
  return Template.fromStack(stack)
}

test('egress is an arm64 provided.al2023 Lambda named {env}-go-dfe-egress', () => {
  build().hasResourceProperties('AWS::Lambda::Function', {
    FunctionName: egressFunctionName('dev'),
    Runtime: 'provided.al2023',
    Architectures: ['arm64'],
    Handler: 'bootstrap',
  })
})

test('egress timeout is above the NFS-e worst case and below the callers', () => {
  const tpl = build().findResources('AWS::Lambda::Function')
  const fn: any = Object.values(tpl)[0]
  expect(fn.Properties.Timeout).toBe(EGRESS_TIMEOUT_SECONDS)
})

test('egress role has no permission beyond basic execution', () => {
  const tpl = build()
  tpl.resourceCountIs('AWS::IAM::Policy', 0)
})
```

- [ ] **Step 5: Rodar e ver falhar**

Run: `cd cdk && npx jest test/egress-stack.test.ts`
Expected: FAIL (módulo `../lib/egress-stack` inexistente).

- [ ] **Step 6: `egress-stack.ts`**

```ts
import * as cdk from 'aws-cdk-lib'
import * as iam from 'aws-cdk-lib/aws-iam'
import * as lambda from 'aws-cdk-lib/aws-lambda'
import {Construct} from 'constructs'
import path from 'node:path'
import {goCode} from './go-code'
import {EGRESS_TIMEOUT_SECONDS, egressFunctionName} from './egress'
import {Environment} from './types'

const EGRESS_DIR = path.join(__dirname, '../../go-dfe-egress')
const EGRESS_MEMORY_MB = 256

interface GoDfeEgressStackProps extends cdk.StackProps {
  environment: Environment
}

// Runs in sa-east-1 so every SEFAZ/municipal call originates from a Brazilian IP
// (some authorities drop foreign traffic). No AWS permissions beyond logging:
// it only talks to tax authorities.
export class GoDfeEgressStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: GoDfeEgressStackProps) {
    super(scope, id, props)
    const {environment} = props

    const role = new iam.Role(this, 'EgressRole', {
      roleName: `${environment}-go-dfe-egress-role`,
      assumedBy: new iam.ServicePrincipal('lambda.amazonaws.com'),
      managedPolicies: [iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole')],
    })

    new lambda.Function(this, 'EgressFunction', {
      functionName: egressFunctionName(environment),
      runtime: lambda.Runtime.PROVIDED_AL2023,
      handler: 'bootstrap',
      code: goCode(EGRESS_DIR, '.'),
      role,
      architecture: lambda.Architecture.ARM_64,
      timeout: cdk.Duration.seconds(EGRESS_TIMEOUT_SECONDS),
      memorySize: EGRESS_MEMORY_MB,
      environment: {APP_ENVIRONMENT: environment},
    })
  }
}
```

Nota: `roleName` é global em IAM; o nome `{env}-go-dfe-egress-role` não conflita com os existentes.

- [ ] **Step 7: Rodar e ver passar**

Run: `cd cdk && npx jest test/egress-stack.test.ts`
Expected: PASS (se o `synth` falhar por falta de `go`, instalar o Go local; é o mesmo requisito do worker).

- [ ] **Step 8: Ligar worker/API/IAM ao egress (ainda aditivo)**

- `cdk/bin/ctech-dfe-cdk.ts`: `new GoDfeEgressStack(app, id('GoDfeEgress'), {env: {account: AWS_ACCOUNT, region: EGRESS_REGION}, environment: ENVIRONMENT, description: \`CTech DFe egress Lambda (sa-east-1) - ${ENVIRONMENT}\`})`; no `WorkerStack`: `dfeLambdaName: egressFunctionName(ENVIRONMENT)` **só será trocado na Task 5**; aqui passar o novo prop `dfeEgressRegion: EGRESS_REGION` (worker passa a exportar `DFE_EGRESS_REGION` ao Lambda). O `dfeLambdaArn` do worker passa a `egressFunctionArn(environment, this.account)` na Task 5 junto com a troca do nome — nesta task apenas adicionar o prop e a env.
- `iam-stack.ts`: acrescentar `lambda:InvokeFunction` em `egressFunctionArn(environment, this.account)` (e `:*`) **além** do py-dfe atual (remoção do py-dfe na Task 10).
- `api-stack.ts`: acrescentar `DFE_EGRESS_REGION=${EGRESS_REGION}` ao `/etc/app-static.env`; `SEFAZ_FUNCTION_NAME` só muda na Task 6.
- `worker-stack.test.ts`: acrescentar `dfeEgressRegion: 'sa-east-1'` ao `buildTemplate` e um teste que assegura `DFE_EGRESS_REGION` nas variáveis dos Lambdas.

- [ ] **Step 9: Rodar todos os testes CDK e synth**

Run: `cd cdk && npx tsc --noEmit && npx jest && ENVIRONMENT=dev npx cdk synth --quiet`
Expected: PASS; `synth` gera a stack `CtechDfe-Dev-GoDfeEgress` em sa-east-1.

- [ ] **Step 10: Documentar e commitar**

`cdk/CLAUDE.md`/`cdk/AGENTS.md`/`DEPLOYMENT.md`: stack nova, pré-requisito `cdk bootstrap aws://868899309401/sa-east-1 --profile ctech`, nomes e região. 

```bash
git add cdk DEPLOYMENT.md
git commit -m "feat(cdk): add go-dfe-egress stack in sa-east-1"
```

---

### Task 5: Worker chama o egress

**Files:**
- Modify: `worker/internal/config/config.go`, `worker/cmd/worker/main.go`, `worker/cmd/distribution-worker/main.go`, `worker/internal/service/dfe.go`, `worker/internal/service/distribution.go`, `worker/internal/service/godfe_shadow.go` (reduzido), testes correspondentes, `cdk/bin/ctech-dfe-cdk.ts`, `cdk/lib/worker-stack.ts`, `cdk/test/worker-stack.test.ts`
- Test: `worker/internal/config/config_test.go`, `worker/internal/service/dfe_test.go`, `worker/internal/service/distribution_test.go`, `worker/internal/service/distribution_nfse_test.go`, `worker/internal/service/godfe_shadow_test.go`

**Interfaces:**
- Produces: `config.Config.DfeEgressRegion string` (env `DFE_EGRESS_REGION`, obrigatório); `(*DfeService).invokeEgress(ctx, lambdaPayload) (lambdaResponse, error)`; `(*DistributionService).invokeEgress(ctx, map[string]any) (map[string]any, error)`.

- [ ] **Step 1: Teste da configuração (falhando)**

`worker/internal/config/config_test.go`:

```go
func TestLoadRequiresEgressRegion(t *testing.T) {
	t.Setenv("DOCUMENTS_BUCKET", "docs")
	t.Setenv("CERTIFICATES_BUCKET", "certs")
	t.Setenv("DFE_LAMBDA_NAME", "dev-go-dfe-egress")
	t.Setenv("DFE_EGRESS_REGION", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DFE_EGRESS_REGION is empty")
	}
	t.Setenv("DFE_EGRESS_REGION", "sa-east-1")
	cfg, err := Load()
	if err != nil || cfg.DfeEgressRegion != "sa-east-1" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
```

Run: `cd worker && go test ./internal/config -run TestLoadRequiresEgressRegion -count=1` → Expected: FAIL (campo inexistente).

- [ ] **Step 2: Implementar a config**

`config.go`: campo `DfeEgressRegion string`; em `Load`: `DfeEgressRegion: os.Getenv("DFE_EGRESS_REGION")` e

```go
	if cfg.DfeEgressRegion == "" {
		return nil, fmt.Errorf("DFE_EGRESS_REGION is required")
	}
```

Run o teste → PASS.

- [ ] **Step 3: Client Lambda na região do egress**

`worker/cmd/worker/main.go` e `worker/cmd/distribution-worker/main.go`: trocar `Lambda: lambdaSDK.NewFromConfig(ac)` por

```go
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
```

(só o client Lambda muda de região; S3/Dynamo/SNS continuam em us-east-1.)

- [ ] **Step 4: Teste do envio ao egress (falhando)**

Em `dfe_test.go`, reescrever o teste atual do corte para go-dfe (linha ~242, "stubs godfeImplements/godfeCall") como: processa uma mensagem `nfe/NFeAutorizacao` e verifica que **o `mockLambda` foi invocado uma vez** com `FunctionName == cfg.DfeLambdaName` e payload contendo `"environment":"producao"` (o egress normaliza; o worker não precisa mais). Reaproveitar o `mockLambda` já usado nos demais testes do arquivo. Mesma troca em `distribution_nfse_test.go` (~287-310): o cursor NSU deve chegar ao `mockLambda` no payload (`body.distNSU...` — usar o campo que o teste atual já asserta em `req.Body`).

Run: `cd worker && go test ./internal/service -count=1` → Expected: FAIL.

- [ ] **Step 5: Implementar**

`dfe.go` (~linha 300): substituir todo o bloco if/else e o comentário "2026-07-18 cutover" por:

```go
	lambdaResp, err := s.invokeEgress(ctx, pyDfePayload)
```

renomear `pyDfePayload` → `egressPayload`, `invokePyDfe` → `invokeEgress` (definição ~linha 561) e o texto do log `"py-dfe returned error"` → `"egress returned error"` e `detail := "py-dfe error"` → `detail := "egress error"`. Remover o import `godfe` se ficar sem uso.

`distribution.go` (~1461-1500): `invokePyDfe` passa a ser só:

```go
// invokeEgress sends a SEFAZ/municipal call to the go-dfe-egress Lambda (sa-east-1).
func (s *DistributionService) invokeEgress(ctx context.Context, payload map[string]any) (map[string]any, error) {
	payloadBytes, err := json.Marshal(payload)
	...corpo atual de invokePyDfeLambda...
}
```

(excluir `invokePyDfeLambda` e o desvio `mapToDfeRequest`/`godfeImplements`); atualizar os chamadores (`distribution.go:288,501,616,690`, `distribution_nfse.go:167` e a mensagem `"invokePyDfe nfse: %w"` → `"invokeEgress nfse: %w"`).

`godfe_shadow.go` (worker): remover `godfeImplements`, `godfeCall`, `mapToDfeRequest`; manter `normalizeSefazEnvironment` **somente se** ainda tiver chamador (`rg normalizeSefazEnvironment worker`); se não tiver, removê-lo. Renomear o arquivo para `sefaz_env.go` se sobrar algo; senão excluir `godfe_shadow.go` e `godfe_shadow_test.go`. Remover o `init()` de `distribution_test.go` que força `godfeImplements=false` e os testes de `mapToDfeRequest`.

- [ ] **Step 6: Rodar o worker inteiro**

Run: `cd worker && go vet ./... && go test ./... -count=1 && CGO_ENABLED=0 GOARCH=arm64 go build ./...`
Expected: PASS, build limpo, nenhum import `go-dfe` sobrando exceto onde ainda há uso de tipos (`rg "go-dfe" worker --glob '*.go'` deve ficar vazio; se algo ainda usa tipo, justificar).

- [ ] **Step 7: CDK do worker**

`worker-stack.ts`: `dfeLambdaArn = egressFunctionArn(environment, this.account)`; passar `DFE_EGRESS_REGION: dfeEgressRegion` nas duas listas de variáveis (linhas ~223 e ~299). `ctech-dfe-cdk.ts`: `dfeLambdaName: egressFunctionName(ENVIRONMENT)`. `worker-stack.test.ts`: `dfeLambdaName: 'dev-go-dfe-egress'` e asserção do ARN regional (`arn:aws:lambda:sa-east-1:`) na policy de invoke.

Cadeia de timeouts: em `worker-definitions.ts`, os workers com `timeoutSeconds: 60` e `sefazServices` não vazio (`nfe-event-worker`, `nfe-inutilization-worker`, `cte-event-worker`, `mdfe-event-worker`, `nfse-event-worker`) passam a `timeoutSeconds: 150` (o `visibilityTimeout` da fila é derivado: `timeout*6+300`). Teste em `worker-stack.test.ts`:

```ts
import { EGRESS_TIMEOUT_SECONDS } from '../lib/egress'

test('every worker that calls SEFAZ outlives the egress Lambda', () => {
  for (const w of WORKERS.filter(w => w.sefazServices.length > 0)) {
    expect(w.timeoutSeconds).toBeGreaterThan(EGRESS_TIMEOUT_SECONDS)
  }
})
```
(escrever o teste antes de alterar `worker-definitions.ts`, ver falhar, depois subir os timeouts.)

Run: `cd cdk && npx tsc --noEmit && npx jest` → Expected: PASS.

- [ ] **Step 8: Documentar e commitar**

`worker/CLAUDE.md`, `worker/README.md`, `DOCS.md`, `OVERVIEW.md`, `MIGRATION.md` (entrada 2026-10-06: corte de 2026-07-18 desfeito para os docs fiscais; motivo), `CONDUCT.md` (egress é o único caminho; sem fallback).

```bash
git add worker cdk docs DOCS.md OVERVIEW.md MIGRATION.md CONDUCT.md
git commit -m "feat(worker): call go-dfe-egress instead of in-process go-dfe"
```

---

### Task 6: API chama o egress

**Files:**
- Modify: `api/internal/config/config.go`, `api/internal/awsclient/client.go`, `api/internal/services/external.go`, `api/internal/services/nfses/municipal.go`, `cdk/lib/api-stack.ts`, `cdk/test/api-stack.test.ts`
- Test: `api/internal/services/external_test.go` (criar se não existir), `api/internal/services/nfses/municipal_test.go`, `api/tests/integration/nfses_test.go`, `api/tests/integration/setup_test.go`

**Interfaces:**
- Produces: `config.Config.DfeEgressRegion string` (env `DFE_EGRESS_REGION`, required); `awsclient.Clients.Lambda` criado com essa região; `(*ExternalService).CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error)`.

- [ ] **Step 1: Config e client**

`config.go`: `DfeEgressRegion string \`env:"DFE_EGRESS_REGION,required"\``. `awsclient/client.go`:

```go
		Lambda:        lambda.NewFromConfig(awsCfg, func(o *lambda.Options) { o.Region = cfg.DfeEgressRegion }),
```

- [ ] **Step 2: Teste de `CallDfe` (falhando)**

`api/internal/services/external_test.go` — usando um fake do client Lambda. `ExternalService.clients.Lambda` é `*lambda.Client` concreto; para testar, extrair uma interface mínima **local** (DRY com `invokeSefazLambda`, que também a usará):

```go
// lambdaInvoker is the Invoke subset used to reach the go-dfe-egress Lambda.
type lambdaInvoker interface {
	Invoke(ctx context.Context, in *lambda.InvokeInput, opts ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}
```

Teste:

```go
type fakeInvoker struct {
	gotName string
	gotBody []byte
	out     *lambda.InvokeOutput
	err     error
}

func (f *fakeInvoker) Invoke(_ context.Context, in *lambda.InvokeInput, _ ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	f.gotName, f.gotBody = aws.ToString(in.FunctionName), in.Payload
	return f.out, f.err
}

func TestCallDfeRoundTrip(t *testing.T) {
	f := &fakeInvoker{out: &lambda.InvokeOutput{Payload: []byte(`{"statusCode":200,"body":"{\"ok\":true}"}`)}}
	resp, err := callDfe(context.Background(), f, "dev-go-dfe-egress", godfe.Request{DocType: "nfse", Service: "NFSeConsulta"})
	if err != nil || resp.StatusCode != 200 || resp.Body != `{"ok":true}` {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if f.gotName != "dev-go-dfe-egress" || !bytes.Contains(f.gotBody, []byte(`"doc_type":"nfse"`)) {
		t.Fatalf("name=%q body=%s", f.gotName, f.gotBody)
	}
}

func TestCallDfeFunctionError(t *testing.T) {
	fe := "Unhandled"
	f := &fakeInvoker{out: &lambda.InvokeOutput{FunctionError: &fe, Payload: []byte(`{"errorMessage":"x"}`)}}
	if _, err := callDfe(context.Background(), f, "fn", godfe.Request{}); err == nil {
		t.Fatal("FunctionError must be an error, never an empty success")
	}
}
```

Run: `cd api && go test ./internal/services -run TestCallDfe -count=1` → FAIL (indefinidos).

- [ ] **Step 3: Implementar `callDfe`/`CallDfe` e remover shadow**

Em `external.go`:

```go
// callDfe invokes the go-dfe-egress Lambda with a typed request and returns its
// typed response. A FunctionError (crash/timeout of the Lambda) is an error.
func callDfe(ctx context.Context, lam lambdaInvoker, funcName string, req godfe.Request) (godfe.Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return godfe.Response{}, fmt.Errorf("encode egress request: %w", err)
	}
	out, err := lam.Invoke(ctx, &lambda.InvokeInput{FunctionName: aws.String(funcName), Payload: payload})
	if err != nil {
		return godfe.Response{}, fmt.Errorf("invoke egress: %w", err)
	}
	if out.FunctionError != nil {
		return godfe.Response{}, fmt.Errorf("egress function error: %s", aws.ToString(out.FunctionError))
	}
	var resp godfe.Response
	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return godfe.Response{}, fmt.Errorf("decode egress response: %w", err)
	}
	return resp, nil
}

// CallDfe sends req to the go-dfe-egress Lambda (SEFAZ_FUNCTION_NAME).
func (s *ExternalService) CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error) {
	return callDfe(ctx, s.clients.Lambda, s.sefazFunctionName, req)
}
```

Atualizar `invokeSefazLambda` para aceitar `lambdaInvoker`; (o shadow já foi removido na Task 1); atualizar comentários que citam "py-dfe Lambda" em `external.go`/`distributions.go`. Importar `godfe "gopkg.aoctech.app/dfe/go-dfe"`.

`nfses/municipal.go:111`: trocar `godfe.Call(ctx, godfe.Request{...})` por `s.extSvc.CallDfe(ctx, godfe.Request{...})` (mesmos campos). Mensagem de erro: `"falha na consulta ao fisco: " + err.Error()` permanece.

- [ ] **Step 4: Ajustar testes de nfses e integração**

`rg -n "godfe|extSvc" api/internal/services/nfses/*_test.go api/tests/integration/*_test.go` — os testes que dependiam do `dfe.Call` em processo passam a injetar um `lambdaInvoker` fake no `ExternalService` (construtor de teste já usado por `municipal_test.go`; acrescentar um parâmetro/campo `invoker lambdaInvoker` em `NewExternalService` **só se** o `ExternalService` não tiver outro ponto de injeção — nesse caso o campo `lam lambdaInvoker` substitui `clients.Lambda` em `CallDfe`/`LookupOrganization`).

- [ ] **Step 5: Rodar a API**

Run: `cd api && go vet ./... && go test ./... -count=1 && CGO_ENABLED=0 GOARCH=arm64 go build ./...`
Expected: PASS.

- [ ] **Step 6: CDK da API**

`api-stack.ts`: `SEFAZ_FUNCTION_NAME=${egressFunctionName(environment)}`; `iam-stack.ts`: o `lambdaInvokePolicy` passa a ter só os ARNs do egress (`egressFunctionArn(environment, this.account)` e `:*`) — o py-dfe sai da política **na Task 10**, então aqui manter ambos. `api-stack.test.ts`: asserção do nome novo e de `DFE_EGRESS_REGION`.

Run: `cd cdk && npx tsc --noEmit && npx jest` → PASS.

- [ ] **Step 7: Documentar e commitar**

`api/CLAUDE.md`, `api/AGENTS.md`, `api/README.md`, `DOCS.md`, `INTEGRATION.md` (se citar o py-dfe).

```bash
git add api cdk docs DOCS.md INTEGRATION.md
git commit -m "feat(api): call go-dfe-egress for every SEFAZ and NFS-e request"
```

---

### Task 7: (absorvida pela Task 1)

O shadow e o vocabulário de fallback do go-dfe saem na Task 1, por decisão do dono. Nada a fazer aqui.

---

### Task 8: CI — testar e implantar o egress

**Files:**
- Modify: `.github/workflows/godfe.yml`, `.github/workflows/deploy.yml`, `.github/workflows/infra.yml`

O egress é implantado pelo CDK (`infra.yml`, role `ctech-dfe-gha-infra`, AdministratorAccess), então não há job de `update-function-code` separado.

- [ ] **Step 1: Testar o egress junto do go-dfe**

`godfe.yml`: acrescentar, depois de "Run tests":

```yaml
      - name: Build egress (arm64, CGO disabled)
        working-directory: go-dfe-egress
        run: CGO_ENABLED=0 GOARCH=arm64 go build ./...

      - name: Test egress
        working-directory: go-dfe-egress
        run: go test ./... -race -count=1
```

e ajustar `cache-dependency-path` para incluir `go-dfe-egress/go.sum`.

- [ ] **Step 2: Filtros e ordem em `deploy.yml`**

No `paths-filter`: `godfe` ganha `- 'go-dfe-egress/**'`; `infra` ganha `- 'go-dfe/**'`, `- 'go-dfe-egress/**'`, `- 'go.work'`, `- 'go.work.sum'`; remover o filtro `py-dfe/layer/requirements.txt` na Task 10. O job `infra` passa a `needs: [changes, godfe]` com `if: ${{ !cancelled() && !contains(needs.*.result, 'failure') && needs.changes.outputs.infra == 'true' }}` (egress só implanta depois de passar nos testes). Atualizar o comentário do cabeçalho: `CDK → OAuth scopes → worker → API → Frontend`.

- [ ] **Step 3: `infra.yml`**

Adicionar aos `paths` do `pull_request`: `'go-dfe/**'`, `'go-dfe-egress/**'`. O passo "setup-go" já existe (usado pelo bundling local).

- [ ] **Step 4: Validar**

Run: `rg -n "py-dfe" .github/workflows/deploy.yml .github/workflows/infra.yml` (ainda há referências a remover na Task 10) e `python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in sys.argv[1:]]" .github/workflows/*.yml`
Expected: YAML válido.

- [ ] **Step 5: Commit**

```bash
git add .github
git commit -m "ci: test go-dfe-egress and deploy it with the CDK stacks"
```

---

### Task 9: (removida)

Decisão do dono: sem gate manual de validação. Não há clientes usando o sistema e o deploy será feito de tudo de uma vez. Pré-requisito que continua valendo: `cdk bootstrap` de sa-east-1 antes do primeiro deploy (`cd cdk && npx cdk bootstrap aws://868899309401/sa-east-1 --profile ctech`).

---

### Task 10: Remover o py-dfe — ADIADA (não executar até o dono liberar)

**Files:**
- Delete: `py-dfe/` (diretório inteiro), `cdk/lib/dfe-stack.ts`, `cdk/test/py-dfe-cdk.test.ts`
- Modify: `cdk/bin/ctech-dfe-cdk.ts` (remover `DfeStack` e import), `cdk/lib/oidc-stack.ts` (remover `PyDfeLambdaDeployRole` e o output), `cdk/lib/iam-stack.ts` (remover os ARNs `...-py-dfe`; renomear a policy para `${environment}-go-dfe-egress-invoke-policy`), `cdk/package.json` (campo `name`), `.github/workflows/*.yml` (remover filtros/menções), `.github/dependabot.yml`, `AGENTS.md`, `CLAUDE.md`, `OVERVIEW.md`, `DOCS.md`, `MIGRATION.md`, `CONDUCT.md`, `DEPLOYMENT.md`, `INTEGRATION.md`, `README.md`, `ROADMAP.md`, `api/{CLAUDE,AGENTS,README}.md`, `cdk/{CLAUDE,AGENTS,README}.md`

**NÃO EXECUTAR ESTA TAREFA AGORA.** O dono adiou a remoção do diretório `py-dfe/`, da `DfeStack`, do OIDC e do CI do py-dfe. O código do go-dfe já foi limpo na Task 1; o py-dfe segue no repo e no deploy.

- [ ] **Step 1: Inventário**

Run: `rg -i -l "py-?dfe|py_dfe|pydfe" --glob '!py-dfe/**' --glob '!node_modules' --glob '!**/cdk.out/**' --glob '!docs/plans/**' --glob '!docs/specs/**' .`
Anotar a lista; planos/specs antigos em `docs/` são histórico e **não** são reescritos.

- [ ] **Step 2: Remover código e infra**

`git rm -r py-dfe cdk/lib/dfe-stack.ts cdk/test/py-dfe-cdk.test.ts`. Editar `ctech-dfe-cdk.ts` (remover `import {DfeStack}` e o bloco `new DfeStack(...)` e o comentário "DfeStack and WorkerStack..."); `oidc-stack.ts` (bloco `pyDfeRole` e `CfnOutput PyDfeLambdaRoleArn`); `iam-stack.ts` (policy só do egress). `cdk/package.json`: `"name": "ctech-dfe-cdk"`.

- [ ] **Step 3: CI**

`deploy.yml`/`infra.yml`/demais workflows: remover `py-dfe/layer/requirements.txt` dos filtros e as menções "py-dfe" nos cabeçalhos; `dependabot.yml`: remover o ecossistema pip de `py-dfe`.

- [ ] **Step 4: Verificar**

Run: `cd cdk && npx tsc --noEmit && npx jest && ENVIRONMENT=dev npx cdk synth --quiet; cd .. && rg -i -l "py-?dfe|py_dfe|pydfe" --glob '!docs/plans/**' --glob '!docs/specs/**' --glob '!**/node_modules/**' --glob '!**/cdk.out/**' .`
Expected: testes PASS; `synth` sem a stack `Dfe`; `rg` só devolve `MIGRATION.md` (entrada histórica, aceitável) e nada mais.

- [ ] **Step 5: Aviso de deploy**

O próximo `cdk deploy` **exclui** o Lambda `{env}-py-dfe`, o layer e os roles. Confirmar com o usuário a janela de deploy; layer em prod tem `RETAIN` (precisa de remoção manual do layer: `aws lambda delete-layer-version ...`).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: remove py-dfe Lambda, layer, roles and CI"
```

---

### Task 11: Documentação final e checklist (sem itens de remoção do py-dfe, que ficam na Task 10)

**Files:** `OVERVIEW.md`, `DOCS.md`, `MIGRATION.md`, `CONDUCT.md`, `DEPLOYMENT.md`, `CLAUDE.md` (tabela de projetos: remover `py-dfe/`, incluir `go-dfe-egress/`), `go-dfe/CLAUDE.md`

- [ ] **Step 1:** `OVERVIEW.md`: novo diagrama de fluxo (worker/API → Invoke → egress sa-east-1 → fisco), motivo (bloqueio geográfico de Teresina, evidência de 2026-10-06 com o teste us-east-1 × sa-east-1).
- [ ] **Step 2:** `DOCS.md`: seção do egress (contrato, env vars `DFE_EGRESS_REGION`/`DFE_LAMBDA_NAME`/`SEFAZ_FUNCTION_NAME`, timeouts: connect 10 s, 20 s por tentativa NFS-e, egress 120 s).
- [ ] **Step 3:** `CONDUCT.md`: PFX/senha cruzam regiões no payload; egress nunca loga o corpo; sem fallback em processo; cadeia de timeouts.
- [ ] **Step 4:** `DEPLOYMENT.md`: bootstrap de sa-east-1 e ordem de deploy `CDK(egress) → worker → API`.
- [ ] **Step 5:** `MIGRATION.md`: entrada 2026-10-06 — corte de 2026-07-18 desfeito para docs fiscais; py-dfe removido.
- [ ] **Step 6: Checklist final**

Run: `for m in go-dfe go-dfe-egress worker api; do (cd $m && CGO_ENABLED=0 GOARCH=arm64 go build ./... && go test ./... -count=1) || echo FAIL $m; done; cd cdk && npx jest && cd ../ui && npx eslint src --ext .ts,.tsx`
Expected: sem `FAIL`; eslint sem erros/avisos (a UI não é alterada, mas é o portão do repo).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "docs: document go-dfe-egress and the py-dfe removal"
```
