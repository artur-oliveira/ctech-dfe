# NFS-e — Fase F3: `api` + `worker` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ligar o pipeline de NFS-e ponta a ponta no backend — emissão assíncrona, eventos, consultas, XML, DANFSE,
parâmetros municipais e distribuição ADN — reusando outbox → SNS → SQS → worker sem nenhuma infraestrutura nova.

**Arquitetura:** `NfseService.Emit` espelha `NfeService.Emit` linha a linha: carrega organização, config e certificado,
resolve serviços e pessoas, calcula a chave, monta o comando do worker e grava tudo num único `TransactWriteItems` (
reserva do contador + item em `nfses` + comando em `worker_outbox`). A diferença é a chave: NF-e usa a chave de acesso
de 44 dígitos que geramos; NFS-e usa `id_dps`, porque a chave da NFS-e só existe depois da resposta do fisco (spec
§3.4). No worker, `docType == "nfse"` roteia para `dfe.Call` in-process — nunca `invokePyDfe`, que não tem NFS-e.

**Tech Stack:** go 1.27, Fiber v3, aws-sdk-go-v2, `go.uber.org/fx`, DynamoDB, S3, SNS/SQS, Valkey (cache).

**Spec:** `docs/specs/2026-08-04-nfse-design.md` §5, §6, §3.4, §3.5, §3.6.

**Depende de:** F1 (tabelas, catálogo de serviços, config, grupo `nfse` das pessoas) e F2 (`go-dfe/nfse`).

## Global Constraints

- Todo erro de `api` é RFC 7807 via `problem.*`. Rotas respondem por `sendProblem(c, err)`. Nunca `fiber.Map`, nunca
  `fiber.NewError`, nunca erro cru.
- Separação de camadas rígida: Repository só toca DynamoDB; Service só regra de negócio e cache; Route só parseia, chama
  UM método de service e responde. No worker: Handler parseia SQS, Service orquestra, Repository persiste.
- Zero string mágica. Nome de tabela, prefixo S3, status, tipo de evento, serviço do go-dfe e chave de cache são
  constantes nomeadas.
- **Chaves JSON da API em inglês.** Todo campo de body/response é em inglês, como no resto da API (`competence`,
  `customer_id`, `service_id`, `tax_rate`, `additional_info`, `reason_code`, …). A única exceção são os códigos do
  leiaute do DPS, que ficam com o nome normativo do campo (`tp_emit`, `motivo_emis_ti`, `ch_nfse_rej`, `c_trib_mun`,
  `c_loc_emi`, `trib_issqn`, …) — a mesma regra que a NF-e já aplica em `tp_nf`/`fin_nfe`/`nat_op`. O modelo neutro
  `nfse.Document` (atributo `payload`) é contrato separado: espelha o leiaute de propósito e não segue esta regra.
- DRY: antes de criar qualquer função, `rg` no `internal/`. Duas implementações do mesmo problema devem ser unificadas.
- Injeção por `go.uber.org/fx`. Nunca instanciar serviço dentro de handler.
- DynamoDB: `GetItem` > `Query` > `Scan`. Sem scan em produção. Update de status usa `UpdateItem` com condition
  expression.
- **Idempotência é obrigatória no worker.** SQS é standard: entrega ao menos uma vez, sem ordem. Antes de chamar o
  fisco, o handler reivindica o lease (`claimProcessing`) e pula mensagem já terminal.
- **Rejeição de negócio do fisco nunca é reprocessada.** Só erro de rede/timeout/5xx justifica retry.
- Testes: `cd api && go test ./... -race`; integração com build tag `integration`. `cd worker && go test ./...`.
- Nenhum commit leva certificado PFX, credencial AWS, segredo JWT, CPF/CNPJ real ou dado de cliente real.
- Nenhum commit leva trailer `Co-Authored-By: Claude`.
- Toda mudança de comportamento leva atualização de documentação no MESMO commit.
- Commits em Conventional Commit, sem emoji.

## Decisões que esta fase carrega da F1/F2 e não pode reabrir

- **SK de `nfses` é `id_dps`**, 45 caracteres, não a chave de acesso. `access_key` é atributo, indexado pela GSI
  `access-key-index`, e só é gravado quando o fisco responde.
- **`WorkerMessage.AccessKey` carrega o `id_dps`** para NFS-e. O campo é o identificador da linha do documento, não
  semanticamente uma chave de acesso — é o que `BuildOutboxTx` usa para compor o `operation_id` (
  `api/internal/services/worker.go:69`) e o que `claimProcessing` usa como SK. Nenhum campo novo é adicionado a
  `WorkerMessage`.
- **`WorkerMessage.UF` fica vazio** para NFS-e — competência é municipal. O município emissor vai dentro do `Body`.
- **`reg_trib` do prestador vem do cadastro de pessoas**, não da config (spec §3.2). Com `tp_emit` 2 ou 3 o prestador é
  uma pessoa do cadastro e precisa do próprio `reg_trib`; a validação é aqui, na emissão.

---

## Estrutura de arquivos

| Arquivo                                                  | Responsabilidade                                        | Ação      |
|----------------------------------------------------------|---------------------------------------------------------|-----------|
| `api/internal/repositories/documents.go`                 | `NfseRepository` + `TransactReserveAndCreate` reusado   | Modificar |
| `api/internal/repositories/nfses.go`                     | Consulta por `access_key` (GSI) e listagem NFS-e        | Criar     |
| `api/internal/repositories/distributions.go`             | `NfseDistributionRepository` + cursor de NSU            | Modificar |
| `api/internal/services/nfses/service.go`                 | `NfseService`, consultas, XML, contexto de evento       | Criar     |
| `api/internal/services/nfses/emit.go`                    | `NfseEmitBody`, `Emit`, `resolveServices`, `BuildIDDPS` | Criar     |
| `api/internal/services/nfses/document.go`                | Monta o `nfse.Document` a partir do body + cadastros    | Criar     |
| `api/internal/services/nfses/events.go`                  | Cancelamento, substituição, evento genérico             | Criar     |
| `api/internal/services/nfses/municipal.go`               | Proxy de parâmetros municipais com cache                | Criar     |
| `api/internal/api/v1/nfses.go`                           | Rotas `/nfses` e `/nfse/...`                            | Criar     |
| `api/internal/api/v1/router.go`                          | Registro das rotas                                      | Modificar |
| `api/internal/app/app.go`                                | Providers fx                                            | Modificar |
| `api/internal/repositories/roles.go`                     | Recursos RBAC `nfses`, `nfse_events`                    | Modificar |
| `api/internal/middleware/scopes.go`                      | Famílias de escopo                                      | Modificar |
| `worker/internal/service/dfe.go`                         | Roteamento e persistência de NFS-e                      | Modificar |
| `worker/internal/service/nfse.go`                        | Extração de chave/protocolo da resposta NFS-e           | Criar     |
| `worker/internal/service/distribution.go`                | Cursor de NSU do ADN                                    | Modificar |
| `api/tests/integration/nfses_test.go`                    | Integração da emissão                                   | Criar     |
| `worker/internal/service/nfse_test.go`                   | Unitários do parsing                                    | Criar     |
| `DOCS.md`, `OVERVIEW.md`, `CONDUCT.md`, `INTEGRATION.md` | Documentação                                            | Modificar |

---

### Task 1: Repositórios de NFS-e

**Files:**

- Create: `api/internal/repositories/nfses.go`
- Modify: `api/internal/repositories/documents.go`
- Modify: `api/internal/repositories/distributions.go`
- Test: `api/internal/repositories/nfses_test.go`

**Interfaces:**

- Consumes: `DocumentRepository` (`documents.go:17`), `DocumentEventRepository` (`dfe_events.go:20`),
  `DistributionRepository` (`distributions.go:17`), `QueryResult`.
- Produces:
    - `repositories.NewNfseRepository(db *dynamodb.Client, cfg *config.Config) *NfseRepository`
    -
    `(*NfseRepository).GetByAccessKey(ctx context.Context, pk, accessKey string) (map[string]types.AttributeValue, error)`
    -
    `repositories.NfseListOpts{Limit int; StartKey map[string]types.AttributeValue; Status *string; Year, Month *int; Sort string}`
    - `(*NfseRepository).ListNfses(ctx context.Context, pk string, opts NfseListOpts) (*QueryResult, error)`
    - `repositories.NewNfseDistributionRepository(db, cfg) *NfseDistributionRepository`
    - `(*DistributionRepository).GetLastNSU(ctx context.Context, pk string) (int64, error)` e
      `SetLastNSU(ctx context.Context, pk string, nsu int64) error`
    - constante `repositories.TableNfses = "nfses"` e `TableNfseEvents = "nfse_events"`

**Contexto:** `NfseRepository` embute `DocumentRepository`, exatamente como `NfeRepository` (`documents.go:52`). Com
isso `Create`, `Get`, `Update` e `TransactReserveAndCreate` vêm de graça. O que é novo:

1. `GetByAccessKey` consulta a GSI `access-key-index` — a spec §11 assume esse custo em troca de SK imutável.
2. `ListNfses` não usa `date-index` com `incoming`, porque NFS-e não tem o conceito de nota recebida por manifestação da
   mesma forma; usa `number-index` quando há filtro por número e a query direta por `pk` no restante.
3. O cursor de NSU do ADN é sequencial, não `ultNSU`+`maxNSU` (spec §3.6). Fica como um item de cursor na própria tabela
   `distributions`, com `sk = "CURSOR"`.

Os eventos reusam `NewDocumentEventRepository(db, cfg, "nfse")` **sem nenhuma alteração de código** — a spec §3.5 já
constatou isso. Nada a criar.

- [ ] **Step 1: Escrever o teste que falha**

Crie `api/internal/repositories/nfses_test.go`:

```go
package repositories

import "testing"

func TestNfseRepository_TableAndIndexNames(t *testing.T) {
	r := &NfseRepository{DocumentRepository: DocumentRepository{tableName: "dev_dfe_nfses"}}
	if r.tableName != "dev_dfe_nfses" {
		t.Errorf("tableName = %q", r.tableName)
	}
	if accessKeyIndexName != "access-key-index" {
		t.Errorf("accessKeyIndexName = %q, esperado access-key-index", accessKeyIndexName)
	}
}

func TestNfseListOpts_DefaultSort(t *testing.T) {
	opts := NfseListOpts{}
	if normalizeSort(opts.Sort) != "asc" {
		t.Errorf("sort default = %q, esperado asc", normalizeSort(opts.Sort))
	}
	if normalizeSort("desc") != "desc" {
		t.Error("sort desc não preservado")
	}
}

func TestDistributionCursorSK(t *testing.T) {
	if distributionCursorSK != "CURSOR" {
		t.Errorf("distributionCursorSK = %q", distributionCursorSK)
	}
}

```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/repositories/ -run 'TestNfse|TestDistributionCursor' -v`
Expected: FAIL — `undefined: NfseRepository`.

- [ ] **Step 3: Escrever `nfses.go`**

```go
package repositories

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Tabelas de NFS-e, criadas na fase F1 do módulo.
const (
	TableNfses      = "nfses"
	TableNfseEvents = "nfse_events"
)

// accessKeyIndexName é a GSI que resolve chave de acesso -> item. Existe
// porque a SK de nfses é o id_dps: a chave de acesso só passa a existir na
// resposta do fisco (spec §3.4).
const accessKeyIndexName = "access-key-index"

// NfseRepository reusa toda a mecânica de DocumentRepository — inclusive
// TransactReserveAndCreate, que a emissão usa para reservar número, criar o
// documento e enfileirar o comando do worker numa transação só.
type NfseRepository struct {
	DocumentRepository
}

func NewNfseRepository(db *dynamodb.Client, cfg *config.Config) *NfseRepository {
	return &NfseRepository{DocumentRepository: DocumentRepository{
		db: db, tableName: cfg.TablePrefix + "_" + TableNfses,
	}}
}

// GetByAccessKey resolve a chave de acesso pela GSI. Devolve nil quando não
// existe — o serviço traduz para 404.
func (r *NfseRepository) GetByAccessKey(ctx context.Context, pk, accessKey string) (map[string]types.AttributeValue, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		IndexName:              aws.String(accessKeyIndexName),
		KeyConditionExpression: aws.String("pk = :pk AND access_key = :ak"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":ak": &types.AttributeValueMemberS{Value: accessKey},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", accessKeyIndexName, err)
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	return out.Items[0], nil
}

// NfseListOpts filtra a listagem. Status e competência são os filtros que a
// tela de NFS-e oferece (spec §7).
type NfseListOpts struct {
	Limit    int
	StartKey map[string]types.AttributeValue
	Status   *string
	Number   *int
	Year     *int
	Month    *int
	Sort     string
}

func normalizeSort(s string) string {
	if s == "desc" {
		return "desc"
	}
	return "asc"
}

// ListNfses lista por pk, opcionalmente por número (number-index) ou
// competência (date-index). Sem scan.
func (r *NfseRepository) ListNfses(ctx context.Context, pk string, opts NfseListOpts) (*QueryResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	sort := normalizeSort(opts.Sort)

	if opts.Number != nil {
		return r.queryNumberIndex(ctx, pk, *opts.Number, opts.Limit, opts.StartKey, sort)
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.tableName),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		Limit:             aws.Int32(int32(opts.Limit)),
		ScanIndexForward:  aws.Bool(sort == "asc"),
		ExclusiveStartKey: opts.StartKey,
	}
	if opts.Status != nil {
		input.FilterExpression = aws.String("#st = :st")
		input.ExpressionAttributeNames = map[string]string{"#st": "status"}
		input.ExpressionAttributeValues[":st"] = &types.AttributeValueMemberS{Value: *opts.Status}
	}

	out, err := r.db.Query(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", r.tableName, err)
	}
	return &QueryResult{Items: out.Items, LastEvaluatedKey: out.LastEvaluatedKey}, nil
}
```

**Nota para quem implementa:** os campos `db` e `tableName` de `DocumentRepository` são minúsculos e o pacote é o mesmo,
então o acesso direto acima compila. Confirme com
`rg "type DocumentRepository" -A6 api/internal/repositories/documents.go` antes de escrever — se os nomes divergirem,
use os reais em vez de renomear o tipo existente.

- [ ] **Step 4: Estender `distributions.go`**

Acrescente ao final do arquivo:

```go
// distributionCursorSK é o item de cursor por organização. O ADN pagina por
// NSU sequencial, não por ultNSU+maxNSU como o DistDFe da NF-e (spec §3.6) —
// por isso o cursor é um único inteiro monotônico.
const distributionCursorSK = "CURSOR"

// NfseDistributionRepository guarda os documentos recebidos do ADN.
type NfseDistributionRepository struct{ DistributionRepository }

func NewNfseDistributionRepository(db *dynamodb.Client, cfg *config.Config) *NfseDistributionRepository {
return &NfseDistributionRepository{newDistributionRepo(db, cfg, TableNfses+"_distributions")}
}

// GetLastNSU devolve o último NSU consumido por pk. Zero quando nunca rodou.
func (r *DistributionRepository) GetLastNSU(ctx context.Context, pk string) (int64, error) {
out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
TableName: aws.String(r.tableName),
Key: map[string]types.AttributeValue{
"pk": &types.AttributeValueMemberS{Value: pk},
"sk": &types.AttributeValueMemberS{Value: distributionCursorSK},
},
})
if err != nil {
return 0, fmt.Errorf("get cursor: %w", err)
}
n, ok := out.Item["last_nsu"].(*types.AttributeValueMemberN)
if !ok {
return 0, nil
}
var v int64
if _, err := fmt.Sscanf(n.Value, "%d", &v); err != nil {
return 0, fmt.Errorf("parse last_nsu %q: %w", n.Value, err)
}
return v, nil
}

// SetLastNSU avança o cursor. A condição impede regressão: uma entrega
// duplicada de SQS não pode fazer o cursor voltar e reprocessar o lote.
func (r *DistributionRepository) SetLastNSU(ctx context.Context, pk string, nsu int64) error {
_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
TableName: aws.String(r.tableName),
Key: map[string]types.AttributeValue{
"pk": &types.AttributeValueMemberS{Value: pk},
"sk": &types.AttributeValueMemberS{Value: distributionCursorSK},
},
UpdateExpression:    aws.String("SET last_nsu = :n"),
ConditionExpression: aws.String("attribute_not_exists(last_nsu) OR last_nsu < :n"),
ExpressionAttributeValues: map[string]types.AttributeValue{
":n": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", nsu)},
},
})
if err != nil && !isConditionalCheckFailed(err) {
return fmt.Errorf("set cursor: %w", err)
}
return nil
}
```

Se `isConditionalCheckFailed` ainda não existir no pacote, adicione:

```go
func isConditionalCheckFailed(err error) bool {
var ccf *types.ConditionalCheckFailedException
return errors.As(err, &ccf)
}
```

com o import de `"errors"`. Antes de escrever, rode `rg "ConditionalCheckFailed" api/internal/repositories/` — se já
houver um helper equivalente, use o existente.

- [ ] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/repositories/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/repositories
git commit -m "feat(nfse): repositorios de nfses e cursor de distribuicao ADN"
```

---

### Task 2: Body de emissão e montagem do documento neutro

**Files:**

- Create: `api/internal/services/nfses/emit.go` (só os tipos e `BuildIDDPS` nesta task)
- Create: `api/internal/services/nfses/document.go`
- Test: `api/internal/services/nfses/document_test.go`

**Interfaces:**

- Consumes: `nfse.Document` e sub-structs (F2, Task 1); `tables.IsValidTribNacional` (F1).
- Produces:
    - `nfses.NfseEmitBody` e `nfses.NfseServiceItem` (abaixo)
    - `nfses.BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string`
    - `nfses.buildDocument(in documentInput) (nfse.Document, error)` e
      `nfses.documentInput{Org, Config, Prestador, Tomador, Intermediario map[string]types.AttributeValue; Service map[string]types.AttributeValue; Body NfseEmitBody; Serie string; Numero int; Environment int}`

**Contexto:**

`BuildIDDPS` é a mesma regra da F2 (`nacional.BuildIDDPS`). Ela existe **duas vezes de propósito**: a `api` precisa do
`id_dps` **antes** de chamar o go-dfe, porque é a SK que ela grava na transação; o go-dfe precisa dele para montar o
`Id` do `infDPS`. Duplicação de regra é justamente o que o CLAUDE.md manda unificar, então a `api` **importa**
`nacional.BuildIDDPS` em vez de reimplementar — `go-dfe` já é dependência da `api` desde a F1. A função
`nfses.BuildIDDPS` é um alias fino de uma linha, só para não espalhar o import do subpacote `nacional` pela camada de
serviço.

O corpo da emissão referencia o catálogo (`service_id`) e pode sobrescrever valor e alíquota por item, do mesmo jeito
que `resolveProducts` faz em NF-e (`emit.go:532`). Diferente da NF-e, NFS-e tem **um único serviço por documento** — o
leiaute `TCServ` não é lista. Por isso o body carrega `service` singular, não `services`.

Regras de validação que ficam aqui, e não no go-dfe:

| Regra                                                                       | Motivo                                                                                                        |
|-----------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| `tp_emit` 2/3 exige `c_motivo_emis_ti`                                      | Já validado no go-dfe, mas aqui a mensagem é RFC 7807 e chega ao usuário antes de gastar uma chamada ao fisco |
| `tp_emit` 2/3 exige que o prestador informado tenha o grupo `nfse.reg_trib` | `regTrib` é obrigatório em `TCInfoPrestador` e, nesses casos, o prestador é pessoa do cadastro (spec §3.2)    |
| `provider == abrasf204` exige o bloco `abrasf` na config                    | Sem endpoint não há para onde enviar                                                                          |
| `op_simp_nac == 3` exige `reg_ap_trib_sn`                                   | Regra condicional deixada explicitamente para a F3 pelo plano da F1                                           |

- [ ] **Step 1: Escrever o teste que falha**

Crie `api/internal/services/nfses/document_test.go`:

```go
package nfses

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func attrs(m map[string]string) map[string]types.AttributeValue {
	out := map[string]types.AttributeValue{}
	for k, v := range m {
		out[k] = &types.AttributeValueMemberS{Value: v}
	}
	return out
}

func TestBuildIDDPS_MatchesLayout(t *testing.T) {
	got := BuildIDDPS("2211001", "2", "11222333000181", "1", 42)
	if len(got) != 45 {
		t.Fatalf("len = %d, esperado 45 (TSIdDPS)", len(got))
	}
	if !strings.HasPrefix(got, "DPS2211001211222333000181") {
		t.Errorf("prefixo incorreto: %q", got)
	}
	if !strings.HasSuffix(got, "00001000000000000042") {
		t.Errorf("serie/nDPS mal preenchidos: %q", got)
	}
}

func TestBuildDocument_RequiresMotivoWhenTomadorEmits(t *testing.T) {
	in := minimalInput()
	in.Body.TpEmit = 2
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: c_motivo_emis_ti obrigatório com tp_emit=2")
	}
}

func TestBuildDocument_RequiresRegTribOnPrestador(t *testing.T) {
	in := minimalInput()
	in.Body.TpEmit = 2
	in.Body.MotivoEmisTI = 1
	in.Prestador = attrs(map[string]string{"name": "Prestador Terceiro"}) // sem grupo nfse
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: prestador sem reg_trib no cadastro")
	}
}

func TestBuildDocument_UsesServiceCatalogDefaults(t *testing.T) {
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Servico.CServ.CTribNac != "10101" {
		t.Errorf("cTribNac = %q, esperado vir do catálogo", doc.Servico.CServ.CTribNac)
	}
	if doc.Valores.VServPrest.VServ != "1000.00" {
		t.Errorf("vServ = %q, esperado vir do catálogo", doc.Valores.VServPrest.VServ)
	}
}

func TestBuildDocument_ItemOverridesCatalogValue(t *testing.T) {
	in := minimalInput()
	override := "2500.00"
	in.Body.Service.Value = &override
	doc, err := buildDocument(in)
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	if doc.Valores.VServPrest.VServ != override {
		t.Errorf("vServ = %q, esperado o override %q", doc.Valores.VServPrest.VServ, override)
	}
}

func TestBuildDocument_RequiresRegApTribSNWhenOpSimpNacIs3(t *testing.T) {
	in := minimalInput()
	in.Prestador = map[string]types.AttributeValue{
		"nfse": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"reg_trib": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"op_simp_nac":  &types.AttributeValueMemberN{Value: "3"},
				"reg_esp_trib": &types.AttributeValueMemberN{Value: "0"},
			}},
		}},
		"cpf_cnpj": &types.AttributeValueMemberS{Value: "11222333000181"},
	}
	if _, err := buildDocument(in); err == nil {
		t.Fatal("esperado erro: op_simp_nac=3 exige reg_ap_trib_sn")
	}
}
```

Escreva também o helper `minimalInput()` no mesmo arquivo, montando um `documentInput` válido com organização (CNPJ,
grupo `nfse` com `reg_trib` `op_simp_nac=1`), config (`provider=nacional`, `c_loc_emi=2211001`, `serie=1`), um item de
catálogo (`trib_nacional_code=10101`, `value=1000.00`, `description="Análise de sistemas"`, `iss.trib_issqn=1`,
`iss.tax_rate=2.00`, `iss.tp_ret_issqn=1`) e um
`NfseEmitBody{TpEmit: 1, Service: NfseServiceItem{ServiceID: "SERVICE_x"}}`.

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/services/nfses/ -v`
Expected: FAIL — `undefined: BuildIDDPS`.

- [ ] **Step 3: Escrever os tipos do body em `emit.go`**

```go
package nfses

import (
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// NfseEmitBody é o corpo de POST /v1.0/nfses. Diferente da NF-e, NFS-e tem
// UM serviço por documento — TCServ não é lista.
type NfseEmitBody struct {
	TpEmit       int    `json:"tp_emit" validate:"required,oneof=1 2 3"`
	MotivoEmisTI int    `json:"motivo_emis_ti" validate:"omitempty,oneof=1 2 3 4"`
	ChNFSeRej    string `json:"ch_nfse_rej" validate:"omitempty,len=50,numeric"`
	Competence   string `json:"competence" validate:"required,datebr"`

	// Quando tp_emit != 1 o prestador é uma pessoa do cadastro.
	ProviderPersonID *string `json:"provider_person_id" validate:"omitempty"`
	CustomerID       *string `json:"customer_id" validate:"omitempty"`
	IntermediaryID   *string `json:"intermediary_id" validate:"omitempty"`

	Service NfseServiceItem `json:"service" validate:"required"`

	// Substituição de NFS-e já emitida (gera o evento 105102 no fisco).
	SubstitutesAccessKey *string `json:"substitutes_access_key" validate:"omitempty,len=50,numeric"`
	SubstitutesReason    *string `json:"substitutes_reason" validate:"omitempty,max=2"`

	AdditionalInfo *string `json:"additional_info" validate:"omitempty,max=2000"`
}

// NfseServiceItem referencia o catálogo e permite sobrescrever valor,
// alíquota e descrição por emissão — o mesmo padrão de resolveProducts.
type NfseServiceItem struct {
	ServiceID   string  `json:"service_id" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Value       *string `json:"value" validate:"omitempty,money"`
	TaxRate     *string `json:"tax_rate" validate:"omitempty,money"`
	Quantity    *string `json:"quantity" validate:"omitempty,money"`
	CTribMun    *string `json:"c_trib_mun" validate:"omitempty,max=20"`
}

// BuildIDDPS delega para a regra normativa que vive no go-dfe. NÃO
// reimplemente: a api e o go-dfe TÊM que produzir o mesmo identificador,
// porque um é a SK da linha e o outro é o Id assinado no infDPS.
func BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string {
	return nacional.BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie, nDPS)
}
```

- [ ] **Step 4: Escrever `document.go`**

```go
package nfses

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Códigos do leiaute (tiposSimples_v1.01.xsd) usados nas validações abaixo.
const (
	tpEmitPrestador   = 1
	opSimpNacApuracao = 3 // exige regApTribSN
	appVersion        = "ctech-dfe-1.0"
)

// documentInput reúne tudo que a montagem precisa. Os map[string]AttributeValue
// vêm dos repositórios sem conversão intermediária, seguindo o padrão de
// NfeService.Emit.
type documentInput struct {
	Org           map[string]types.AttributeValue
	Config        map[string]types.AttributeValue
	Prestador     map[string]types.AttributeValue
	Tomador       map[string]types.AttributeValue
	Intermediario map[string]types.AttributeValue
	Service       map[string]types.AttributeValue
	Body          NfseEmitBody
	Serie         string
	Numero        int
	Environment   int
}

// buildDocument converte cadastro + catálogo + body no modelo neutro do
// go-dfe. Toda regra condicional que depende de contexto de emissão mora
// aqui — o go-dfe só valida estrutura.
func buildDocument(in documentInput) (nfse.Document, error) {
	if in.Body.TpEmit != tpEmitPrestador && in.Body.MotivoEmisTI == 0 {
		return nfse.Document{}, problem.BadRequest(
			"motivo_emis_ti é obrigatório quando a emissão não é do prestador")
	}

	prest, err := buildPrestador(in.Prestador)
	if err != nil {
		return nfse.Document{}, err
	}

	doc := nfse.Document{
		Ambiente: in.Environment, VerAplic: appVersion,
		TpEmit: in.Body.TpEmit, MotivoEmisTI: in.Body.MotivoEmisTI,
		ChNFSeRej:   in.Body.ChNFSeRej,
		Competencia: in.Body.Competence,
		Serie:       in.Serie, Numero: in.Numero,
		CLocEmi:       strAttr(in.Config, "c_loc_emi"),
		Prestador:     prest,
		Tomador:       buildPessoa(in.Tomador),
		Intermediario: buildPessoa(in.Intermediario),
		Servico:       buildServico(in.Service, in.Body),
		Valores:       buildValores(in.Service, in.Body),
		IBSCBS:        buildIBSCBS(in.Service),
	}

	if in.Body.SubstituiChave != nil {
		motivo := ""
		if in.Body.SubstituiMotivo != nil {
			motivo = *in.Body.SubstituiMotivo
		}
		doc.Substituicao = &nfse.Substituicao{
			ChSubstda: *in.Body.SubstituiChave, CMotivo: motivo,
		}
	}
	return doc, nil
}

// buildPrestador extrai identidade e regime tributário do item de cadastro.
// O grupo nfse é o mesmo em organizations e organization_persons (spec §3.2),
// então esta função serve aos dois casos sem ramificação.
func buildPrestador(item map[string]types.AttributeValue) (nfse.Prestador, error) {
	if item == nil {
		return nfse.Prestador{}, problem.BadRequest("prestador não encontrado")
	}
	grupo := mapAttr(item, "nfse")
	regTribItem := mapAttr(grupo, "reg_trib")
	if regTribItem == nil {
		return nfse.Prestador{}, problem.BadRequest(
			"o prestador não tem regime tributário de NFS-e cadastrado (grupo nfse.reg_trib)")
	}

	reg := nfse.RegTrib{
		OpSimpNac:   intAttr(regTribItem, "op_simp_nac", 0),
		RegApTribSN: intAttr(regTribItem, "reg_ap_trib_sn", 0),
		RegEspTrib:  intAttr(regTribItem, "reg_esp_trib", 0),
	}
	if reg.OpSimpNac == opSimpNacApuracao && reg.RegApTribSN == 0 {
		return nfse.Prestador{}, problem.BadRequest(
			"reg_ap_trib_sn é obrigatório quando op_simp_nac = 3")
	}

	p := basePessoa(item, grupo)
	return nfse.Prestador{Pessoa: p, RegTrib: reg}, nil
}

func buildPessoa(item map[string]types.AttributeValue) *nfse.Pessoa {
	if item == nil {
		return nil
	}
	p := basePessoa(item, mapAttr(item, "nfse"))
	return &p
}

// basePessoa mapeia identidade + endereço. Os campos de NFS-e (IM, CAEPF,
// NIF, cNaoNIF, endereço no exterior) vêm do grupo nfse adicionado na F1;
// nome, documento e endereço nacional vêm dos campos já existentes.
func basePessoa(item, grupo map[string]types.AttributeValue) nfse.Pessoa {
	doc := strAttr(item, "cpf_cnpj")
	p := nfse.Pessoa{
		XNome:   strAttr(item, "name"),
		IM:      strAttr(grupo, "im"),
		CAEPF:   strAttr(grupo, "caepf"),
		NIF:     strAttr(grupo, "nif"),
		CNaoNIF: intAttr(grupo, "c_nao_nif", 0),
		Fone:    strAttr(item, "phone"),
		Email:   strAttr(item, "email"),
	}
	if len(doc) == 14 {
		p.CNPJ = doc
	} else if len(doc) == 11 {
		p.CPF = doc
	}
	p.End = buildEndereco(item, grupo)
	return p
}
```

Complete `document.go` com `buildEndereco`, `buildServico`, `buildValores` e `buildIBSCBS`, todos leituras diretas dos
itens do DynamoDB para as structs do `nfse.Document`, e os helpers `strAttr`, `intAttr`, `mapAttr`. Regras:

- `buildEndereco` prefere `foreign_address` do grupo `nfse` quando presente (preenche `CPais`, `CEndPost`, `XCidade`,
  `XEstadoProv`); caso contrário usa o primeiro item de `addresses` do cadastro, mapeando `city_ibge_code` para `CMun` e
  `zip_code` para `CEP`.
- `buildServico` lê do catálogo: `trib_nacional_code` → `CTribNac`, `nbs_code` → `CNBS`, `code` → `CIntContrib`,
  `description` → `XDescServ`. `Body.Service.Description` e `Body.Service.CTribMun`, quando não-nil, sobrescrevem.
  `LocPrest.CLocPrestacao` recebe `c_loc_emi` da config.
- `buildValores` lê `value` do catálogo, sobrescrito por `Body.Service.Value`. `iss.tax_rate` vira `PAliq`, sobrescrita
  por `Body.Service.TaxRate`. `iss.trib_issqn` e `iss.tp_ret_issqn` vão para `TribISSQN`/`TpRetISSQN`. `exig_susp` e
  `bm` viram os grupos correspondentes quando presentes. `federal` alimenta `TribFed`, `tot_trib` alimenta `TotTrib`.
- `buildIBSCBS` devolve `nil` quando o mapa `ibs_cbs` do catálogo está ausente ou sem `c_ind_op`; caso contrário monta
  `nfse.IBSCBS` com `c_ind_op`, `cst`, `c_class_trib`, `ind_dest`, `tp_oper` e `fin_nfse`.

`strAttr` e `intAttr` já existem em `api/internal/services/nfes/service.go:411,420`. Rode
`rg "func strAttr" api/internal/services/` antes de escrever: se estiverem exportáveis ou houver um helper compartilhado
em `services/shared.go`, **reuse em vez de duplicar** — duplicar viola a regra de DRY do projeto.

- [ ] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/services/nfses/ -race -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/nfses
git commit -m "feat(nfse): body de emissao e montagem do documento neutro"
```

---

### Task 3: `NfseService.Emit`

**Files:**

- Modify: `api/internal/services/nfses/emit.go`
- Create: `api/internal/services/nfses/service.go`
- Test: `api/internal/services/nfses/emit_test.go`

**Interfaces:**

- Consumes: `buildDocument` (Task 2); `repositories.NfseRepository` (Task 1); `repositories.NfseConfigRepository`,
  `repositories.ServiceRepository` (F1); `OrganizationRepository`, `PersonRepository`, `CertificateRepository`;
  `services.WorkerService.BuildOutboxTx`.
- Produces:
    - `nfses.NewNfseService(...) *NfseService` (providers fx)
    -
    `(*NfseService).Emit(ctx context.Context, orgPK string, req NfseEmitBody, userID, userName string) (map[string]types.AttributeValue, error)`
    - constantes `nfses.StatusPending/Processing/Authorized/Rejected/Cancelled/Error`, `nfses.S3PrefixNfse = "nfse"`,
      `nfses.DocTypeNfse = "nfse"`

**Contexto:** o corpo de `Emit` espelha `NfeService.Emit` (`api/internal/services/nfes/emit.go:126`) passo a passo. Leia
aquele método inteiro antes de escrever este. As três diferenças:

1. **A chave.** NF-e chama `generateAccessKey`; NFS-e chama `BuildIDDPS`. O resultado vai para `sk` do item e para
   `WorkerMessage.AccessKey`.
2. **`access_key` nasce ausente.** Não grave `access_key` no item inicial — só o worker grava, quando o fisco responde.
   Gravar vazio poluiria a GSI.
3. **`UF` vazio.** `WorkerMessage.UF = ""`. O município vai no `Body` do comando, dentro do `document`.

O `Body` do `WorkerMessage` é exatamente o que `nfse.Dispatch` espera (F2, Task 8):

```go
map[string]any{
"provider": provider,
"document": documentAsMap,
}
```

- [ ] **Step 1: Escrever o teste que falha**

```go
package nfses

import (
	"strings"
	"testing"
)

func TestEmit_BuildsWorkerBodyForDispatch(t *testing.T) {
	// buildWorkerBody é a fronteira testável sem AWS: o que vai no comando
	// do outbox tem que casar com as chaves que nfse.Dispatch lê.
	doc, err := buildDocument(minimalInput())
	if err != nil {
		t.Fatalf("buildDocument: %v", err)
	}
	body, err := buildWorkerBody("nacional", doc)
	if err != nil {
		t.Fatalf("buildWorkerBody: %v", err)
	}
	if body["provider"] != "nacional" {
		t.Errorf("provider = %v", body["provider"])
	}
	sub, ok := body["document"].(map[string]any)
	if !ok {
		t.Fatalf("document não é um objeto: %T", body["document"])
	}
	if sub["c_loc_emi"] != "2211001" {
		t.Errorf("c_loc_emi perdido na serialização: %v", sub["c_loc_emi"])
	}
	// A chave "prestador" tem que existir com o reg_trib dentro — se o
	// achatamento do embedded Pessoa quebrar, isso pega.
	prest, ok := sub["prestador"].(map[string]any)
	if !ok {
		t.Fatalf("prestador ausente: %T", sub["prestador"])
	}
	if _, ok := prest["reg_trib"]; !ok {
		t.Error("reg_trib ausente no prestador serializado")
	}
}

func TestEmit_RejectsUnconfiguredOrg(t *testing.T) {
	svc := &NfseService{}
	_, err := svc.emitPreflight(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Configuração Fiscal") {
		t.Fatalf("esperado erro de config ausente, veio: %v", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/services/nfses/ -run TestEmit -v`
Expected: FAIL — `undefined: buildWorkerBody`.

- [ ] **Step 3: Escrever `service.go`**

```go
package nfses

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/cache"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Status do ciclo de vida da NFS-e (spec §3.4).
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusAuthorized = "authorized"
	StatusRejected   = "rejected"
	StatusCancelled  = "cancelled"
	StatusError      = "error"
)

// Constantes de roteamento do worker/go-dfe.
const (
	DocTypeNfse  = "nfse"
	S3PrefixNfse = "nfse"

	ProviderNacional  = "nacional"
	ProviderAbrasf204 = "abrasf204"
)

var (
	ErrNfseNotFound      = problem.NotFound("NFS-e não encontrada")
	ErrNfseNoConfig      = problem.BadRequest("configure a NFS-e em Configuração Fiscal antes de emitir")
	ErrNfseNoCertificate = problem.NoCertificate("certificado digital não encontrado")
)

// NfseService orquestra emissão, eventos e consultas de NFS-e.
type NfseService struct {
	nfseRepo     *repositories.NfseRepository
	eventRepo    *repositories.DocumentEventRepository
	configRepo   *repositories.NfseConfigRepository
	serviceRepo  *repositories.ServiceRepository
	orgRepo      *repositories.OrganizationRepository
	personRepo   *repositories.PersonRepository
	certRepo     *repositories.CertificateRepository
	distRepo     *repositories.NfseDistributionRepository
	workerSvc    *services.WorkerService
	extSvc       *services.ExternalService
	clients      *awsclient.Clients
	cacheBackend cache.Backend
	bucket       string
}

func NewNfseService(
	nfseRepo *repositories.NfseRepository,
	eventRepo *repositories.DocumentEventRepository,
	configRepo *repositories.NfseConfigRepository,
	serviceRepo *repositories.ServiceRepository,
	orgRepo *repositories.OrganizationRepository,
	personRepo *repositories.PersonRepository,
	certRepo *repositories.CertificateRepository,
	distRepo *repositories.NfseDistributionRepository,
	workerSvc *services.WorkerService,
	extSvc *services.ExternalService,
	clients *awsclient.Clients,
	cacheBackend cache.Backend,
	bucket string,
) *NfseService {
	return &NfseService{
		nfseRepo: nfseRepo, eventRepo: eventRepo, configRepo: configRepo,
		serviceRepo: serviceRepo, orgRepo: orgRepo, personRepo: personRepo,
		certRepo: certRepo, distRepo: distRepo, workerSvc: workerSvc,
		extSvc: extSvc, clients: clients, cacheBackend: cacheBackend, bucket: bucket,
	}
}

// emitPreflight valida a existência de organização e config antes de gastar
// qualquer chamada. Separado para poder ser testado sem AWS.
func (s *NfseService) emitPreflight(org, cfg map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	if org == nil {
		return nil, problem.NotFound("organização não encontrada")
	}
	if cfg == nil {
		return nil, ErrNfseNoConfig
	}
	if strAttr(cfg, "provider") == ProviderAbrasf204 && mapAttr(cfg, "abrasf") == nil {
		return nil, problem.BadRequest("configuração ABRASF incompleta: informe o endpoint do município")
	}
	return cfg, nil
}

// GetNfse aceita id_dps (SK direta) ou chave de acesso (via GSI).
func (s *NfseService) GetNfse(ctx context.Context, orgPK, id string) (map[string]types.AttributeValue, error) {
	pk := s.docPK(ctx, orgPK)
	item, err := s.nfseRepo.Get(ctx, pk, id)
	if err != nil {
		return nil, err
	}
	if item != nil {
		return item, nil
	}
	return s.nfseRepo.GetByAccessKey(ctx, pk, id)
}
```

`docPK` monta `{envPrefix}#{orgPK}` a partir do ambiente da config, exatamente como `NfeService` faz. Implemente-o lendo
a config uma vez e reusando o helper `envToPrefix` já existente em `api/internal/services/nfes/service.go:431` — se ele
não for exportado, mova-o para `api/internal/services/shared.go` e importe nos dois pacotes, em vez de duplicar.

- [ ] **Step 4: Escrever `Emit` em `emit.go`**

```go
// buildWorkerBody serializa o documento no formato que nfse.Dispatch lê
// (chaves "provider" e "document" — go-dfe/nfse/dispatch.go).
func buildWorkerBody(provider string, doc nfse.Document) (map[string]any, error) {
raw, err := json.Marshal(doc)
if err != nil {
return nil, problem.InternalServer("failed to encode nfse document")
}
var docMap map[string]any
if err := json.Unmarshal(raw, &docMap); err != nil {
return nil, problem.InternalServer("failed to decode nfse document")
}
return map[string]any{
nfse.BodyKeyProvider: provider,
nfse.BodyKeyDocument: docMap,
}, nil
}

// Emit espelha NfeService.Emit: carrega contexto, monta o documento, reserva
// o número e grava documento + comando do worker numa transação única.
func (s *NfseService) Emit(ctx context.Context, orgPK string, req NfseEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
if err != nil {
return nil, err
}
configItem, err := s.configRepo.Get(ctx, orgPK)
if err != nil {
return nil, err
}
if _, err := s.emitPreflight(orgItem, configItem); err != nil {
return nil, err
}

certs, err := s.certRepo.List(ctx, orgPK)
if err != nil {
return nil, err
}
if len(certs) == 0 {
return nil, ErrNfseNoCertificate
}
cert := certs[0]

environment := intAttr(configItem, "environment", 2)
envPrefix := envToPrefix(environment)
serie := strAttr(configItem, "serie")
currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)

// tp_emit 1: a própria organização é o prestador. 2 e 3: o prestador é
// pessoa do cadastro e precisa do próprio reg_trib (spec §3.2).
prestadorItem := orgItem
if req.TpEmit != tpEmitPrestador {
if req.ProviderPersonID == nil {
return nil, problem.BadRequest("provider_person_id é obrigatório quando tp_emit != 1")
}
prestadorItem, err = s.personRepo.Get(ctx, orgPK, *req.ProviderPersonID)
if err != nil {
return nil, err
}
if prestadorItem == nil {
return nil, problem.NotFound("prestador não encontrado: " + *req.ProviderPersonID)
}
}

tomadorItem, err := s.resolvePerson(ctx, orgPK, req.CustomerID)
if err != nil {
return nil, err
}
intermItem, err := s.resolvePerson(ctx, orgPK, req.IntermediaryID)
if err != nil {
return nil, err
}

serviceItem, err := s.serviceRepo.Get(ctx, orgPK, req.Service.ServiceID)
if err != nil {
return nil, err
}
if serviceItem == nil {
return nil, problem.NotFound("serviço não encontrado no catálogo: " + req.Service.ServiceID)
}

doc, err := buildDocument(documentInput{
Org: orgItem, Config: configItem, Prestador: prestadorItem,
Tomador: tomadorItem, Intermediario: intermItem, Service: serviceItem,
Body: req, Serie: serie, Numero: currentNumber, Environment: environment,
})
if err != nil {
return nil, err
}

tpInsc, inscFederal := tpInscCNPJ, doc.Prestador.CNPJ
if doc.Prestador.CPF != "" {
tpInsc, inscFederal = tpInscCPF, doc.Prestador.CPF
}
idDPS := BuildIDDPS(doc.CLocEmi, tpInsc, inscFederal, serie, currentNumber)

now := time.Now()
pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
record := map[string]any{
"pk": pk, "sk": idDPS,
"provider":      strAttr(configItem, "provider"),
"status":        StatusPending,
"tp_emit":       req.TpEmit,
"serie":         serie,
"number":        currentNumber,
"competence":    req.Competence,
"c_loc_emi":     doc.CLocEmi,
"year":          now.Year(),
"month":         int(now.Month()),
"emit_cpf_cnpj": services.StripPKPrefix(orgPK),
"emit_name":     strAttr(orgItem, "name"),
"dest_name":     strAttr(tomadorItem, "name"),
"dest_cpf_cnpj": strAttr(tomadorItem, "cpf_cnpj"),
"total":         doc.Valores.VServPrest.VServ,
"payload":       docAsMapForRecord(doc),
"created_at":    now.UTC().Format(time.RFC3339),
"user_id":       userID,
"user_name":     userName,
}
if req.MotivoEmisTI != 0 {
record["c_motivo_emis_ti"] = req.MotivoEmisTI
}
// access_key NÃO é gravada aqui: só existe na resposta do fisco
// (spec §3.4). Gravar vazio poluiria a GSI access-key-index.

encoded, err := repositories.EncodeItem(record)
if err != nil {
return nil, problem.InternalServer("failed to encode NFS-e record")
}

workerBody, err := buildWorkerBody(strAttr(configItem, "provider"), doc)
if err != nil {
return nil, err
}

sefazEnv := services.SefazEnvHom
if environment == 1 {
sefazEnv = services.SefazEnvProd
}
workerMsg := services.WorkerMessage{
DocPK:            pk,
AccessKey:        idDPS, // identificador da linha; NFS-e usa id_dps
TableName:        repositories.TableNfses,
S3Prefix:         S3PrefixNfse,
ExpectedFileName: idDPS,
CNPJ:             services.StripPKPrefix(orgPK),
UF:               "", // competência municipal: não há UF autorizadora
SefazEnvironment: sefazEnv,
CertS3Key:        strAttr(cert, "s3_key"),
CertPassword:     strAttr(cert, "password"),
DocType:          DocTypeNfse,
SefazService:     goconstants.ServiceNFSeRecepcao,
Body:             workerBody,
}
outboxTx, operationID, err := s.workerSvc.BuildOutboxTx(workerMsg)
if err != nil {
return nil, err
}
encoded["operation_id"] = &types.AttributeValueMemberS{Value: operationID}

if err := s.nfseRepo.TransactReserveAndCreate(
ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded, outboxTx,
); err != nil {
if strings.Contains(err.Error(), "TransactionCanceledException") {
return nil, problem.Conflict("conflito ao reservar número da NFS-e. Tente novamente.")
}
return nil, err
}
return encoded, nil
}

func (s *NfseService) resolvePerson(ctx context.Context, orgPK string, id *string) (map[string]types.AttributeValue, error) {
if id == nil {
return nil, nil
}
item, err := s.personRepo.Get(ctx, orgPK, *id)
if err != nil {
return nil, err
}
if item == nil {
return nil, problem.NotFound("pessoa não encontrada: " + *id)
}
return item, nil
}
```

`docAsMapForRecord` reusa `buildWorkerBody` e devolve só a submap `document` — é o `payload` que a spec §3.4 manda
persistir. Os imports novos: `goconstants "gopkg.aoctech.app/dfe/go-dfe/internal/constants"` **não compila** (pacote
interno). Use os nomes de serviço via um espelho exportado: acrescente a `go-dfe/nfse/constants.go` (F2) um bloco
`ServiceRecepcao = "NFSeRecepcao"` etc. e importe `nfse.ServiceRecepcao` aqui. Faça esse ajuste na F2 se ele ainda não
existir — é um bloco de constantes de uma linha cada, sem lógica.

- [ ] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/services/nfses/ -race -v && go build ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/nfses go-dfe/nfse/constants.go
git commit -m "feat(nfse): emissao assincrona com reserva de numero e outbox"
```

---

### Task 4: Eventos — cancelamento, substituição e genérico

**Files:**

- Create: `api/internal/services/nfses/events.go`
- Test: `api/internal/services/nfses/events_test.go`

**Interfaces:**

- Consumes: `nfse.ContribuinteEvents`, `nfse.EventRequest`, `nfse.BodyKeyEvent` (F2);
  `DocumentEventRepository.CreateEvent`.
- Produces:
    -
    `(*NfseService).Cancel(ctx, orgPK, id, motivoCodigo, motivoDescricao string, seq int, userID, userName string) (map[string]types.AttributeValue, error)`
    -
    `(*NfseService).Substitute(ctx, orgPK, id string, req NfseEmitBody, userID, userName string) (map[string]types.AttributeValue, error)`
    -
    `(*NfseService).SendEvent(ctx, orgPK, id string, body NfseEventBody, userID, userName string) (map[string]types.AttributeValue, error)`
    -
    `nfses.NfseEventBody{EventType string; SequenceNumber int; MotivoCodigo, MotivoDescricao, ChaveSubstituta, CPFAgTrib, IDEvManifRej string}`
    - `nfses.ListEvents`, `nfses.GetEventXML`

**Contexto:**

- **Substituição não é um evento que emitimos.** É uma nova DPS com o grupo `subst` preenchido; o fisco gera o evento
  105102 sozinho e cancela a original (manual do contribuinte, seção 1.3.2). Por isso `Substitute` delega para `Emit`
  com `SubstituiChave` preenchido — não monta `EventRequest` nenhum. Isso contraria a leitura ingênua da rota
  `POST /nfses/{id}/substitute`; a rota existe por ergonomia, o caminho interno é emissão.
- **Eventos privativos do fisco são rejeitados com 400.** `105104`, `105105`, `205204`, `305101`, `305102`, `305103` só
  chegam pela distribuição (spec §5.3). A checagem é `nfse.ContribuinteEvents[t]`, o mesmo conjunto que o go-dfe usa —
  uma fonte só.
- O evento é gravado em `nfse_events` com `pk = id_dps` (spec §3.5) e enfileirado com `EventsTableName`, `EventType`,
  `SequenceNumber` e `EventSK` preenchidos no `WorkerMessage` — os campos opcionais que já existem (
  `api/internal/services/worker.go:32`).
- A NFS-e precisa estar autorizada e ter `access_key`: o evento é endereçado à chave, não ao `id_dps`.

**Desvios do plano na implementação (2026-08-05):**

1. **Sem outbox transacional para eventos.** O passo 6 abaixo pedia `BuildOutboxTx`, mas o
   `operation_id` do outbox é `{tabela}#{access_key}` e a `ConditionExpression` é
   `attribute_not_exists(pk)`: a linha da emissão já ocupa `nfses#{id_dps}`, então o evento
   colidiria. Eventos usam `PublishWorkerEvent` (SNS direto), o mesmo caminho de NF-e/NFC-e/MDF-e.
   Tornar eventos transacionais é mudança para todos os doc types — fora do escopo da F3.
2. **Regras de motivo não foram duplicadas na api.** `eventsRequiringMotivo`/`eventsRequiringXMotivo`
   saíram de `go-dfe/nfse/nacional/evento.go` para `go-dfe/nfse/constants.go` como
   `EventsRequiringMotivo`/`EventsRequiringXMotivo`; a api lê os mesmos mapas. `decodeEvent` virou
   `nfse.DecodeEventRequest` (exportado) para o teste da api rodar o mesmo decode estrito do worker.
3. `NfseEventBody` não tem `MotivoCodigo`/`MotivoDescricao`/`ChaveSubstituta`: os nomes são
   `ReasonCode`/`ReasonDescription`/`SubstituteAccessKey` (Global Constraint de chaves em inglês).
4. `NfseService` ganhou `eventRepo`, `clients` e `bucketDocs` (necessários para `GetEventXML`).
   `services.DownloadS3` foi extraído para `internal/services/shared.go` — havia duas cópias
   privadas idênticas em `nfes` e `mdfes`.

- [x] **Step 1: Escrever o teste que falha**

```go
package nfses

import (
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestValidateEventType_RejectsFiscoOnly(t *testing.T) {
	for _, tipo := range []string{"105104", "105105", "205204", "305101"} {
		if err := validateEventType(tipo); err == nil {
			t.Errorf("evento %s privativo do fisco foi aceito", tipo)
		}
	}
}

func TestValidateEventType_AcceptsContribuinte(t *testing.T) {
	for tipo := range nfse.ContribuinteEvents {
		if err := validateEventType(tipo); err != nil {
			t.Errorf("evento %s do contribuinte foi rejeitado: %v", tipo, err)
		}
	}
}

func TestBuildEventRequest_CancelamentoExigeMotivo(t *testing.T) {
	_, err := buildEventRequest("11111111111111111111111111111111111111111111111111",
		NfseEventBody{EventType: nfse.EventCancelamento}, "11222333000181", 2)
	if err == nil || !strings.Contains(err.Error(), "motivo") {
		t.Fatalf("esperado erro de motivo obrigatório, veio: %v", err)
	}
}

func TestBuildEventRequest_SubstituicaoNaoEhEvento(t *testing.T) {
	// 105102 é gerado pelo fisco a partir de uma nova DPS com grupo subst.
	// A api nunca deve enfileirá-lo como pedido de registro de evento.
	_, err := buildEventRequest("1", NfseEventBody{EventType: nfse.EventCancelamentoPorSubst}, "x", 2)
	if err == nil || !strings.Contains(err.Error(), "substituição") {
		t.Fatalf("esperado erro direcionando para /substitute, veio: %v", err)
	}
}

func TestBuildEventRequest_DefaultsSequenceToOne(t *testing.T) {
	ev, err := buildEventRequest("chave", NfseEventBody{
		EventType: nfse.EventConfirmacaoTomador,
	}, "11222333000181", 2)
	if err != nil {
		t.Fatalf("buildEventRequest: %v", err)
	}
	if ev.NSeqEvento != 1 {
		t.Errorf("NSeqEvento = %d, esperado 1", ev.NSeqEvento)
	}
	if ev.CNPJAutor != "11222333000181" {
		t.Errorf("CNPJAutor = %q", ev.CNPJAutor)
	}
}
```

- [x] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/services/nfses/ -run 'TestValidateEventType|TestBuildEventRequest' -v`
Expected: FAIL — `undefined: validateEventType`.

- [x] **Step 3: Escrever `events.go`**

```go
package nfses

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// NfseEventBody é o corpo de POST /v1.0/nfses/{id}/events.
type NfseEventBody struct {
	EventType           string `json:"event_type" validate:"required,len=6,numeric"`
	SequenceNumber      int    `json:"sequence_number" validate:"omitempty,gte=1,lte=999"`
	ReasonCode          string `json:"reason_code" validate:"omitempty,max=2"`
	ReasonDescription   string `json:"reason_description" validate:"omitempty,max=255"`
	SubstituteAccessKey string `json:"substitute_access_key" validate:"omitempty,len=50,numeric"`
	CPFAgTrib           string `json:"cpf_ag_trib" validate:"omitempty,cpf"`
	IDEvManifRej        string `json:"id_ev_manif_rej" validate:"omitempty,max=60"`
}

// eventsRequiringMotivo espelha a mesma regra do go-dfe: validar aqui evita
// gastar uma chamada ao fisco para receber a rejeição.
var eventsRequiringMotivo = map[string]bool{
	nfse.EventCancelamento: true, nfse.EventSolicAnaliseFiscalCanc: true,
	nfse.EventRejeicaoPrestador: true, nfse.EventRejeicaoTomador: true,
	nfse.EventRejeicaoIntermediario: true,
}

// validateEventType rejeita o que o contribuinte não pode emitir. Os eventos
// privativos do fisco (105104, 105105, 205204, 305101-103) só chegam pela
// distribuição do ADN (spec §5.3).
func validateEventType(t string) error {
	if !nfse.ContribuinteEvents[t] {
		return problem.BadRequest("evento " + t + " não pode ser emitido pelo contribuinte")
	}
	return nil
}

// buildEventRequest monta o pedido neutro. Substituição é redirecionada:
// não é pedido de registro de evento, é uma nova DPS com o grupo subst.
func buildEventRequest(chave string, body NfseEventBody, cnpjAutor string, ambiente int) (nfse.EventRequest, error) {
	if body.EventType == nfse.EventCancelamentoPorSubst {
		return nfse.EventRequest{}, problem.BadRequest(
			"cancelamento por substituição é gerado pelo fisco a partir de uma nova emissão: use POST /nfses/{id}/substitute")
	}
	if err := validateEventType(body.EventType); err != nil {
		return nfse.EventRequest{}, err
	}
	if eventsRequiringMotivo[body.EventType] && body.MotivoCodigo == "" {
		return nfse.EventRequest{}, problem.BadRequest("motivo_codigo é obrigatório para o evento " + body.EventType)
	}

	seq := body.SequenceNumber
	if seq == 0 {
		seq = 1
	}
	ev := nfse.EventRequest{
		ChaveAcesso: chave, TipoEvento: body.EventType, NSeqEvento: seq,
		TpAmb: ambiente, VerAplic: appVersion, DhEvento: time.Now().UTC(),
		CNPJAutor: cnpjAutor, ChSubstituta: body.ChaveSubstituta,
		CPFAgTrib: body.CPFAgTrib, IDEvManifRej: body.IDEvManifRej,
	}
	if body.MotivoCodigo != "" {
		ev.Motivo = &nfse.EventMotivo{Codigo: body.MotivoCodigo, Descricao: body.MotivoDescricao}
	}
	return ev, nil
}
```

Complete com `SendEvent`, que:

1. Carrega a NFS-e por `GetNfse`; 404 se não existir.
2. Exige `status == StatusAuthorized` e `access_key` não vazia — sem chave não há evento a endereçar. Caso contrário,
   `problem.BadRequest("a NFS-e ainda não foi autorizada pelo fisco")`.
3. Chama `buildEventRequest`.
4. Cria a linha em `nfse_events` via `s.eventRepo.CreateEvent(ctx, idDPS, ...)` — assinatura exata em
   `api/internal/repositories/dfe_events.go:28`, leia antes de chamar.
5. Monta o `WorkerMessage` com `SefazService: nfse.ServiceEvento`,
   `Body: map[string]any{nfse.BodyKeyProvider: provider, nfse.BodyKeyEvent: eventAsMap}`, e os campos opcionais
   `EventsTableName`, `EventType`, `SequenceNumber`, `EventSK` preenchidos.
6. Publica pelo outbox com `s.workerSvc.BuildOutboxTx` na mesma transação que grava o evento — o mesmo padrão de
   `NfeService.Cancel` (`api/internal/services/nfes/service.go:146`). Leia aquele método e siga a mesma forma de
   transação.

`Cancel` é um invólucro de `SendEvent` com `EventType: nfse.EventCancelamento`. `Substitute` chama `s.Emit` com
`req.SubstituiChave` apontando para a `access_key` da nota original — nunca monta evento.

- [x] **Step 4: Rodar os testes**

Run: `cd api && go test ./internal/services/nfses/ -race -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add api/internal/services/nfses/events.go api/internal/services/nfses/events_test.go
git commit -m "feat(nfse): eventos de cancelamento, manifestacao e substituicao"
```

---

### Task 5: Consultas, XML, DANFSE e parâmetros municipais

**Desvios do plano na implementação (2026-08-05):**

1. **Atributos de XML.** O plano pedia `s3_key_nfse`/`s3_key_dps`; ficou `xml_s3_key` (o mesmo nome
   que `nfes`/`nfces`/`ctes`/`mdfes` usam para o documento autorizado) e `dps_xml_s3_key`. Inventar
   um nome novo para o XML principal quebraria a consistência com todas as outras tabelas. A Task 7
   (worker) tem que gravar exatamente esses dois nomes.
2. **`paramArity` não foi duplicada na api.** Virou `nacional.ParamArity` (exportada); a api valida
   contra a mesma tabela que monta o path. Mesmo precedente de `BuildIDDPS`.
3. **`cacheGetAny`/`cacheSetAny` não existem.** `cacheGet`/`cacheSet` de `internal/services/cache.go`
   foram exportadas (`CacheGet`/`CacheSet`) e usadas direto, como o próprio plano pedia no caso de as
   genéricas servirem.
4. **501 ganhou helper.** `problem.NotImplemented` + `problem.TypeNotImplemented`, em vez de
   `problem.New(fiber.StatusNotImplemented, ...)` na chamada — mantém o padrão de todos os outros
   status.
5. **`ExternalService.CertificateB64` foi extraída** (S3 + base64 do primeiro certificado da
   organização) e `LookupOrganization` passou a usá-la: a chamada síncrona ao go-dfe precisava do
   mesmo par e não podia alcançar o `downloadPFX` privado.
6. **`ListDistributions` ficou em `NfseService`, não em `DistributionService`.** Aquele serviço é
   construído em volta do DistDFe SOAP (`ultNSU`/`maxNSU`, body `distDFeInt`, `docTypeSefazService`);
   o ADN pagina por NSU sequencial em REST. Encaixar NFS-e lá exigiria furar a abstração.


**Files:**

- Modify: `api/internal/services/nfses/service.go`
- Create: `api/internal/services/nfses/municipal.go`
- Test: `api/internal/services/nfses/municipal_test.go`

**Interfaces:**

- Produces:
    - `(*NfseService).ListNfses(ctx, orgPK string, opts repositories.NfseListOpts) (*repositories.QueryResult, error)`
    - `(*NfseService).GetNfseXML(ctx, orgPK, id string) ([]byte, error)` e `GetDPSXML`
    - `(*NfseService).GetDANFSE(ctx, orgPK, id string) ([]byte, error)`
    - `(*NfseService).MunicipalParameters(ctx, orgPK, kind string, args []string) (map[string]any, error)`
    -
    `(*NfseService).ListDistributions(ctx, orgPK string, opts repositories.DistributionListOpts) (*repositories.QueryResult, error)`
    - constantes de chave de cache `cacheKeyMunicipalParams` e `municipalParamsTTL`

**Contexto:**

- Os XMLs ficam no S3 nos caminhos da spec §6: `{org_pk}/nfse/{id_dps}.xml` (a NFS-e) e
  `{org_pk}/nfse/{id_dps}/dps.xml` (a DPS enviada). O download reusa o helper `downloadS3` de
  `api/internal/services/nfes/service.go:389` — mova-o para `services/shared.go` se ainda for privado, em vez de
  duplicar.
- **DANFSE é proxy do ADN e depende do provider.** `provider == abrasf204` responde `501`: não existe PDF padrão no
  leiaute 2.04 (spec §11). Isso é `problem.New(501, ...)`; confirme se `problem` tem um construtor para 501 e, se não
  tiver, use `problem.New(fiber.StatusNotImplemented, ...)` com o `type`/`title` no padrão dos demais.
- **Parâmetros municipais são cacheados 6h em Valkey** (spec §5.4). São dados públicos por município/competência, não
  por tenant — a chave de cache **não** inclui `orgPK`. Incluir seria um erro de custo: cada organização pagaria a mesma
  consulta.
- A chamada ao ADN passa pelo mesmo `dfe.Call` síncrono, feito direto do handler: é consulta pública, sem escrita, sem
  risco de timeout longo. Não vai para o worker.

- [x] **Step 1: Escrever o teste que falha**

```go
package nfses

import "testing"

func TestCacheKeyMunicipalParams_ExcludesTenant(t *testing.T) {
	k1 := cacheKeyMunicipalParams("aliquota", []string{"2211001", "10101", "2026-08"})
	k2 := cacheKeyMunicipalParams("aliquota", []string{"2211001", "10101", "2026-08"})
	if k1 != k2 {
		t.Fatalf("chave não determinística: %q != %q", k1, k2)
	}
	if k1 == cacheKeyMunicipalParams("aliquota", []string{"3550308", "10101", "2026-08"}) {
		t.Error("municípios diferentes geraram a mesma chave")
	}
	if municipalParamsTTL != 6*60*60 {
		t.Errorf("TTL = %d, esperado 21600 (6h)", municipalParamsTTL)
	}
}

func TestMunicipalParamKind_Validation(t *testing.T) {
	if err := validateParamKind("aliquota", []string{"2211001", "10101", "2026-08"}); err != nil {
		t.Errorf("aliquota válida rejeitada: %v", err)
	}
	if err := validateParamKind("aliquota", []string{"2211001"}); err == nil {
		t.Error("aridade errada aceita")
	}
	if err := validateParamKind("inexistente", nil); err == nil {
		t.Error("tipo desconhecido aceito")
	}
}
```

- [x] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/services/nfses/ -run 'TestCacheKey|TestMunicipalParam' -v`
Expected: FAIL — `undefined: cacheKeyMunicipalParams`.

- [x] **Step 3: Escrever `municipal.go`**

```go
package nfses

import (
	"context"
	"strings"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// municipalParamsTTL é 6 horas, em segundos. Parâmetros municipais mudam por
// competência, não por hora — cachear evita rate-limit do ADN e latência no
// formulário de emissão (spec §5.4).
const municipalParamsTTL = 6 * 60 * 60

const cacheKeyMunicipalPrefix = "nfse:munparams:"

// paramArity espelha as rotas do serviço de parametrização do ADN.
var paramArity = map[string]int{
	"aliquota": 3, "convenio": 1, "beneficio": 3,
	"regimes_especiais": 3, "retencoes": 2,
}

// cacheKeyMunicipalParams NÃO inclui a organização: os parâmetros são
// públicos por município/competência. Incluir o tenant faria cada
// organização pagar a mesma consulta ao ADN.
func cacheKeyMunicipalParams(kind string, args []string) string {
	return cacheKeyMunicipalPrefix + kind + ":" + strings.Join(args, ":")
}

func validateParamKind(kind string, args []string) error {
	want, ok := paramArity[kind]
	if !ok {
		return problem.BadRequest("tipo de parâmetro municipal desconhecido: " + kind)
	}
	if len(args) != want {
		return problem.BadRequest("o parâmetro " + kind + " exige outra quantidade de argumentos")
	}
	return nil
}

// MunicipalParameters consulta o ADN, com cache. A chamada ao go-dfe é
// síncrona e feita direto: é leitura pública, sem escrita, sem risco de
// estourar o timeout do API Gateway.
func (s *NfseService) MunicipalParameters(ctx context.Context, orgPK, kind string, args []string) (map[string]any, error) {
	if err := validateParamKind(kind, args); err != nil {
		return nil, err
	}
	key := cacheKeyMunicipalParams(kind, args)
	if cached, ok := cacheGetAny(ctx, s.cacheBackend, key); ok {
		return cached, nil
	}

	out, err := s.callGoDfeMunicipal(ctx, orgPK, kind, args)
	if err != nil {
		return nil, err
	}
	cacheSetAny(ctx, s.cacheBackend, key, out, municipalParamsTTL)
	return out, nil
}
```

`callGoDfeMunicipal` carrega organização, config e certificado, monta
`dfe.Request{DocType: "nfse", Service: nfse.ServiceParametrosMunicipais, Environment: ..., CertificateB64: ..., Body: map[string]any{nfse.BodyKeyProvider: provider, nfse.BodyKeyParamKind: kind, nfse.BodyKeyParamArgs: args}}`
e chama `dfe.Call`, desembrulhando `Result.Parametros` do JSON de resposta. `cacheGetAny`/`cacheSetAny` são invólucros
sobre `cacheGet[map[string]any]`/`cacheSet` já existentes em `api/internal/services/cache.go:15,28` — se as funções
genéricas puderem ser usadas direto, use-as e não crie invólucro nenhum.

- [x] **Step 4: Acrescentar consultas e DANFSE a `service.go`**

`GetNfseXML`/`GetDPSXML` leem `s3_key_nfse`/`s3_key_dps` do item e baixam do bucket. `GetDANFSE` verifica o provider e
responde 501 para `abrasf204`; para `nacional`, chama `dfe.Call` com `nfse.ServiceDANFSE` e devolve `Result.PDF`.

- [x] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/services/nfses/ -race -v && go build ./...`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add api/internal/services/nfses
git commit -m "feat(nfse): consultas, XML, DANFSE e parametros municipais com cache"
```

---

### Task 6: Rotas, RBAC e wiring fx

**Files:**

- Create: `api/internal/api/v1/nfses.go`
- Modify: `api/internal/api/v1/router.go`, `api/internal/app/app.go`, `api/internal/repositories/roles.go`,
  `api/internal/middleware/scopes.go`
- Test: `api/internal/api/v1/nfses_test.go`

**Interfaces:**

- Consumes: `NfseService` (Tasks 3-5); helpers de rota `bindJSON`, `sendProblem`, `sendItem`, `sendPage`, `intQuery`,
  `decodeCursor`, `resolveActor`, `perm.Require`.
- Produces:
  `v1.RegisterNfses(router fiber.Router, svc *nfses.NfseService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker)`

**Contexto:** rotas da spec §5.1. Leia `api/internal/api/v1/nfes.go` inteiro antes de escrever — o arquivo de NFS-e é o
mesmo formato, com os mesmos helpers, e nenhum deles deve ser reimplementado.

| Método | Rota                                    | Permissão            |
|--------|-----------------------------------------|----------------------|
| POST   | `/nfses`                                | `create.nfses`       |
| GET    | `/nfses`                                | `list.nfses`         |
| GET    | `/nfses/:id`                            | `get.nfses`          |
| GET    | `/nfses/:id/xml`                        | `get.nfses`          |
| GET    | `/nfses/:id/dps-xml`                    | `get.nfses`          |
| GET    | `/nfses/:id/danfse`                     | `get.nfses`          |
| POST   | `/nfses/:id/cancel`                     | `delete.nfses`       |
| POST   | `/nfses/:id/substitute`                 | `create.nfses`       |
| POST   | `/nfses/:id/events`                     | `create.nfse_events` |
| GET    | `/nfses/:id/events`                     | `get.nfse_events`    |
| GET    | `/nfses/:id/events/:event_sk/xml`       | `get.nfse_events`    |
| GET    | `/nfse/municipal-parameters/:mun/:kind` | `get.nfses`          |
| GET    | `/nfse/distributions`                   | `list.nfses`         |

O `:id` aceita `id_dps` ou chave de acesso — `GetNfse` resolve os dois (Task 3). A rota de parâmetros municipais recebe
os argumentos extras por query (`servico`, `competencia`, `beneficio`), montados em `args` conforme a aridade do `kind`.

- [x] **Step 1: Escrever o teste que falha**

```go
package v1

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRegisterNfses_MountsAllRoutes(t *testing.T) {
	app := fiber.New()
	RegisterNfses(app, nil, nil, func(c fiber.Ctx) error { return c.Next() }, nil)

	want := []struct{ method, path string }{
		{"POST", "/nfses"}, {"GET", "/nfses"}, {"GET", "/nfses/x"},
		{"GET", "/nfses/x/xml"}, {"GET", "/nfses/x/dps-xml"}, {"GET", "/nfses/x/danfse"},
		{"POST", "/nfses/x/cancel"}, {"POST", "/nfses/x/substitute"},
		{"POST", "/nfses/x/events"}, {"GET", "/nfses/x/events"},
		{"GET", "/nfses/x/events/y/xml"},
	}
	for _, w := range want {
		req := httptest.NewRequest(w.method, w.path, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s %s: %v", w.method, w.path, err)
		}
		if resp.StatusCode == fiber.StatusNotFound {
			t.Errorf("%s %s não montada", w.method, w.path)
		}
	}
}
```

**Nota:** passar `nil` no serviço faz o handler entrar em pânico se for executado. O teste só verifica montagem, então
os handlers não chegam a rodar por causa do `perm.Require` nil — se isso causar pânico no seu ambiente, troque por um
`PermChecker` permissivo construído no teste. Ajuste o teste ao que o `middleware` realmente exige; o que precisa ser
verificado é que nenhuma rota devolve 404.

- [x] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./internal/api/v1/ -run TestRegisterNfses -v`
Expected: FAIL — `undefined: RegisterNfses`.

- [x] **Step 3: Escrever `nfses.go`** seguindo exatamente a forma de `nfes.go`.

- [x] **Step 4: Registrar RBAC e escopos**

Em `api/internal/repositories/roles.go`, acrescente os recursos `nfses` e `nfse_events` ao conjunto existente. Em
`api/internal/middleware/scopes.go`, acrescente as famílias correspondentes. Rode
`rg "nfe_events" api/internal/repositories/roles.go api/internal/middleware/scopes.go` primeiro e siga exatamente o
padrão encontrado — não invente forma nova.

- [x] **Step 5: Wiring fx**

Em `api/internal/app/app.go`, adicione os providers: `repositories.NewNfseRepository`,
`repositories.NewNfseDistributionRepository`, o `DocumentEventRepository` de NFS-e (via uma função nomeada, porque
`NewDocumentEventRepository` recebe o `docType` como argumento — siga o padrão já usado para os eventos de NF-e) e
`nfses.NewNfseService`. Registre `RegisterNfses` no `router.go`.

- [x] **Step 6: Rodar tudo**

Run: `cd api && go build ./... && go test ./... -race`
Expected: PASS.

**Desvios do plano na implementação (2026-08-05):**

1. **Step 4 já estava pronto.** `nfses` e `nfse_events` já constavam em `repositories/roles.go`
   (`resources`) e em `middleware/scopes.go` (`scopeFamilies["nfses"]`). Nenhuma linha alterada —
   RBAC e escopos de NFS-e já funcionavam desde a F1.
2. **O teste inspeciona `app.GetRoutes()`** em vez de disparar requisições: com `perm` nil o handler
   entra em pânico ao executar (`p.check` desreferencia o receptor), e o que se verifica é montagem.
   Cobre também `municipalParamArgs`, onde um deslocamento posicional consultaria o parâmetro errado.
3. **Rota de parâmetros municipais:** `:city` (não `:mun`) e query em inglês — `service`,
   `competence`, `benefit_number` — pela regra de chaves em inglês. A montagem posicional vive em
   `municipalParamArgs` (tradução de query string em posição é assunto de rota); a aridade continua
   validada no serviço contra `nacional.ParamArity`.
4. **Dois helpers novos em `helpers.go`:** `ptrQuery` (filtro `status`, que é `*string`) e `sendXML`
   (três rotas de XML neste arquivo). Os arquivos de NF-e/MDF-e continuam com os `c.Set` inline —
   trocá-los está fora do escopo desta task.
5. **`newNfseService` recebe 13 dependências**, não a forma do snippet do plano: o construtor cresceu
   nas Tasks 3-5 (`distRepo`, `extSvc`, `cacheBackend`). O `DocumentEventRepository` de NFS-e é criado
   dentro da factory, como em `newNFCeService`/`newMDFeService`.

- [x] **Step 7: Commit**

```bash
git add api/internal/api/v1 api/internal/app api/internal/repositories/roles.go api/internal/middleware/scopes.go
git commit -m "feat(nfse): rotas, RBAC e wiring do modulo NFS-e na api"
```

---

### Task 7: Worker — roteamento e persistência de NFS-e

**Files:**

- Create: `worker/internal/service/nfse.go`
- Modify: `worker/internal/service/dfe.go`
- Test: `worker/internal/service/nfse_test.go`

**Interfaces:**

- Consumes: `WorkerMessage` (`dfe.go:79`), `godfeCall`/`godfeImplements` (`godfe_shadow.go`), `updateClaimedDocument`,
  `saveResponse`, `publishResult`.
- Produces:
    - `service.parseNfseResponse(respBody map[string]any) (nfseOutcome, error)`
    - `service.nfseOutcome{AccessKey, IDDPS, NFSeXML, DPSXML, EventoXML, Status, Motivo string}`
    - `service.isNfse(docType string) bool`

**Contexto:**

`Process` (`dfe.go:227`) já roteia por `godfeImplements(msg.DocType, msg.SefazService)` — e a F2 colocou NFS-e no mapa
`implemented`, então **o roteamento já funciona sem alteração**. O que muda é o tratamento da resposta:
`handleSefazResponse` (`dfe.go:321`) procura `cStat`/`xMotivo`/`nProt`, que não existem em NFS-e. A resposta de NFS-e é
o `nfse.Result` serializado (F2, Task 8): `chave_acesso`, `id_dps`, `nfse_xml`, `dps_xml`, `evento_xml`, `alertas`,
`erros`.

Então `Process` ganha um desvio de uma linha antes de `handleSefazResponse`:

```go
    if isNfse(msg.DocType) {
if err := s.handleNfseResponse(ctx, msg, respBody); err != nil {
return s.markRetryable(ctx, msg, err.Error(), err)
}
return nil
}
```

Persistência (spec §6):

| O que                                               | Onde                                           |
|-----------------------------------------------------|------------------------------------------------|
| NFS-e XML                                           | `{org_pk}/nfse/{id_dps}.xml`                   |
| DPS enviada                                         | `{org_pk}/nfse/{id_dps}/dps.xml`               |
| Evento XML                                          | `{org_pk}/nfse/{id_dps}/events/{event_sk}.xml` |
| `access_key`, `status`, `s3_key_nfse`, `s3_key_dps` | tabela `nfses`                                 |
| `status`, `s3_key`, `protocol`                      | tabela `nfse_events`                           |

Rejeição do fisco (`erros` preenchido) é **terminal**: `failTerminal`, nunca retry. A regra já existe em
`worker/CLAUDE.md` — "SEFAZ business rejections must NOT be retried" — e vale igual aqui.

- [x] **Step 1: Escrever o teste que falha**

```go
package service

import "testing"

func TestParseNfseResponse_Authorized(t *testing.T) {
	out, err := parseNfseResponse(map[string]any{
		"chave_acesso": "99999999999999999999999999999999999999999999999999",
		"id_dps":       "DPS123",
		"nfse_xml":     "<NFSe/>",
		"dps_xml":      "<DPS/>",
	})
	if err != nil {
		t.Fatalf("parseNfseResponse: %v", err)
	}
	if out.Status != StatusAuthorized {
		t.Errorf("Status = %q, esperado authorized", out.Status)
	}
	if out.AccessKey == "" || out.NFSeXML == "" || out.DPSXML == "" {
		t.Errorf("campos perdidos: %+v", out)
	}
}

func TestParseNfseResponse_RejectedIsTerminal(t *testing.T) {
	out, err := parseNfseResponse(map[string]any{
		"erros": []any{map[string]any{"codigo": "E123", "descricao": "cTribNac inválido"}},
	})
	if err != nil {
		t.Fatalf("parseNfseResponse: %v", err)
	}
	if out.Status != StatusRejected {
		t.Errorf("Status = %q, esperado rejected", out.Status)
	}
	if out.Motivo != "E123 - cTribNac inválido" {
		t.Errorf("Motivo = %q, o código e a descrição do fisco têm que ser preservados", out.Motivo)
	}
}

func TestParseNfseResponse_Event(t *testing.T) {
	out, err := parseNfseResponse(map[string]any{"evento_xml": "<evento/>"})
	if err != nil {
		t.Fatalf("parseNfseResponse: %v", err)
	}
	if out.EventoXML != "<evento/>" {
		t.Errorf("EventoXML = %q", out.EventoXML)
	}
	if out.Status != StatusAuthorized {
		t.Errorf("evento aceito deveria ser terminal de sucesso, veio %q", out.Status)
	}
}

func TestIsNfse(t *testing.T) {
	if !isNfse("nfse") {
		t.Error("isNfse(nfse) = false")
	}
	if isNfse("nfe") {
		t.Error("isNfse(nfe) = true")
	}
}
```

- [x] **Step 2: Rodar e ver falhar**

Run: `cd worker && go test ./internal/service/ -run 'TestParseNfse|TestIsNfse' -v`
Expected: FAIL — `undefined: parseNfseResponse`.

- [x] **Step 3: Escrever `nfse.go`**

```go
package service

import "fmt"

// docTypeNfse é o valor de WorkerMessage.DocType para NFS-e.
const docTypeNfse = "nfse"

// Chaves do nfse.Result serializado pelo go-dfe (go-dfe/nfse/result.go).
const (
	fieldChaveAcesso = "chave_acesso"
	fieldIDDPS       = "id_dps"
	fieldNfseXML     = "nfse_xml"
	fieldDpsXML      = "dps_xml"
	fieldEventoXML   = "evento_xml"
	fieldErros       = "erros"
)

func isNfse(docType string) bool { return docType == docTypeNfse }

// nfseOutcome é o resultado normalizado de uma chamada NFS-e.
type nfseOutcome struct {
	AccessKey string
	IDDPS     string
	NFSeXML   string
	DPSXML    string
	EventoXML string
	Status    string
	Motivo    string
}

// parseNfseResponse traduz o nfse.Result do go-dfe. Não há cStat/xMotivo em
// NFS-e: a rejeição vem na lista "erros", e é sempre terminal — o fisco já
// avaliou as regras de negócio, repetir a chamada só produz a mesma recusa.
func parseNfseResponse(respBody map[string]any) (nfseOutcome, error) {
	out := nfseOutcome{
		AccessKey: strFromAny(respBody[fieldChaveAcesso]),
		IDDPS:     strFromAny(respBody[fieldIDDPS]),
		NFSeXML:   strFromAny(respBody[fieldNfseXML]),
		DPSXML:    strFromAny(respBody[fieldDpsXML]),
		EventoXML: strFromAny(respBody[fieldEventoXML]),
		Status:    StatusAuthorized,
	}

	if errs, ok := respBody[fieldErros].([]any); ok && len(errs) > 0 {
		out.Status = StatusRejected
		if m, ok := errs[0].(map[string]any); ok {
			out.Motivo = fmt.Sprintf("%s - %s", strFromAny(m["codigo"]), strFromAny(m["descricao"]))
		}
	}
	return out, nil
}

func strFromAny(v any) string {
	s, _ := v.(string)
	return s
}
```

- [x] **Step 4: Escrever `handleNfseResponse` em `nfse.go`**

Fluxo, na ordem:

1. `parseNfseResponse`.
2. Se `Status == StatusRejected`: `return s.failTerminal(ctx, msg, out.Motivo)`. Nada mais é gravado além do status e do
   motivo.
3. Grava os XMLs no S3 pelos caminhos da tabela acima, reusando `saveResponse` (`dfe.go:539`) quando a assinatura
   servir; se não servir, escreva um helper local e **não** duplique a lógica de upload — extraia a parte comum.
4. Se `msg.EventSK != nil`: atualiza `nfse_events` via `updateClaimedEvent` (`dfe.go:670`) com `status`, `s3_key`.
5. Senão: atualiza `nfses` via `updateClaimedDocument` (`dfe.go:580`) com `status = authorized`, `access_key`,
   `s3_key_nfse`, `s3_key_dps`.
6. `publishResult` para o SNS de resultados, que a API converte em push por WebSocket.

**Cuidado com a chave da atualização.** `updateClaimedDocument` usa `msg.AccessKey` como SK — que para NFS-e é o
`id_dps`, não a chave de acesso recebida. A `access_key` do fisco entra como **atributo** no mesmo update. Trocar os
dois é o erro mais provável desta task e produziria um item órfão.

- [x] **Step 5: Ligar o desvio em `Process`**

Insira o bloco de cinco linhas mostrado no Contexto, imediatamente antes da chamada existente a `s.handleSefazResponse`.

- [x] **Step 6: Rodar os testes**

Run: `cd worker && go build ./... && go test ./... -race`
Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add worker/internal/service/nfse.go worker/internal/service/nfse_test.go worker/internal/service/dfe.go
git commit -m "feat(nfse): processamento de NFS-e no worker"
```

**Desvios do plano na implementação (2026-08-05):**

1. `parseNfseResponse` não devolve `error` — não há caminho de falha na tradução do mapa. Testes ajustados.
2. Caminhos no S3 seguem a convenção já usada por todos os outros tipos (`{s3_prefix}/{doc_pk com # → /}/{arquivo}.xml`),
   não o `{org_pk}/nfse/{id_dps}/dps.xml` da tabela acima: a DPS é `{expected_file_name}_dps.xml` ao lado da NFS-e.
   `DOCS.md` e `DynamoDB-Tables.md` foram corrigidos para o caminho real.
3. Atributos gravados: `xml_s3_key` e `dps_xml_s3_key` (nomes escolhidos na Task 5), não `s3_key_nfse`/`s3_key_dps`.
   `updateAttrs` ganhou `AccessKey` e `DPSXMLS3Key`.
4. Em vez de chamar `saveResponse` (que decide XML-ou-JSON pela chave `@xml`), extraí dele `documentS3Key` e
   `putObject`; `saveResponse` passou a usar os dois. Zero duplicação de upload, e o caminho SOAP não mudou de
   comportamento.
5. `isCancellationEvent` ganhou o ramo de NFS-e (`101101`) em vez de uma segunda função no `nfse.go` — o
   cancelamento aceito precisa reverter a NFS-e para `cancelled`, e essa pergunta já tinha dono.
6. Além dos testes do plano, três testes de persistência com os mocks existentes (`TestHandleNfseResponse_*`):
   documento autorizado, rejeição sem upload, e cancelamento revertendo o documento. `capturedUpdate` ganhou
   `values` para permitir asserção sobre os atributos gravados.

---

### Task 8: Distribuição ADN no worker — cursor de NSU

**Files:**

- Create: `worker/internal/service/distribution_nfse.go`
- Modify: `worker/internal/service/distribution.go:64-95` (entrada `nfse` em `docTypeConfigs`), `:145-178` (roteamento
  em `Process`)
- Modify: `worker/cmd/distribution-dispatcher/main.go:17`
- Test: `worker/internal/service/distribution_nfse_test.go`

**Interfaces:**

- Consumes: `DistributionService` (`distribution.go:115`), `godfeImplements`/`godfeCall` (`godfe_shadow.go:14-15`),
  `nfse.BodyKeyNSU`/`BodyKeyCNPJConsulta`/`BodyKeyProvider` e `constants.ServiceNFSeDistribuicao` (F2 Task 8),
  `nfse.DistributionItem` (F2 Task 1).
- Produces:
    - `(*DistributionService).runNfseDistNSU(ctx context.Context, orgPK, trigger string, dtcfg docTypeConfig) error`
    - `buildNfseDistPayload(cnpj, certB64, certPassword, sefazEnv, provider string, nsu int64) map[string]any`
    - `parseNfseDistResponse(respBody map[string]any) (nfseDistBatch, error)`
    - `nfseDistBatch{Status string; Items []nfseDistItem}` e
      `nfseDistItem{NSU int64; ChaveAcesso, TipoDocumento, TipoEvento, XML string}`
    - constantes `nfseDistStatusFound = "DOCUMENTOS_LOCALIZADOS"`,
      `nfseDistStatusEmpty = "NENHUM_DOCUMENTO_LOCALIZADO"`, `nfseDistCursorSK = "CURSOR"`, `maxNfseDistBatches = 20`

**Contexto — por que não dá para reusar `runDistNSU`.** As duas rotinas parecem a mesma coisa e não são:

|             | NF-e / CT-e / MDF-e (`runDistNSU`)          | NFS-e (`runNfseDistNSU`)                            |
|-------------|---------------------------------------------|-----------------------------------------------------|
| Protocolo   | SOAP `distDFeInt`                           | REST `GET /DFe/{NSU}`                               |
| Paginação   | `ultNSU` + `maxNSU` na resposta             | NSU sequencial, o maior NSU do lote                 |
| Fim do lote | `cStat` 137 / 238                           | `StatusProcessamento` sem documentos                |
| Punição     | "consumo indevido" → `improper_usage_until` | não existe no ADN                                   |
| Rate limit  | 1 chamada/hora por organização              | não documentado; limitado por `maxNfseDistBatches`  |
| Cursor      | campo `{env}_nsu` no config da organização  | item `sk = "CURSOR"` na tabela `nfse_distributions` |

Tentar parametrizar `runDistNSU` para cobrir os dois exigiria condicionais em praticamente toda
linha do corpo. A regra de DRY do `CLAUDE.md` manda unificar duas implementações **do mesmo
problema** — aqui os problemas divergem no protocolo, na condição de parada e na localização do
cursor. O que é de fato comum (`loadConfig`, `loadCert`, `getCertB64`, `loadOrg`, `processDocZip`,
`persistIncoming`, `persistPerson`, `notifyResult`) **é reusado sem cópia**; a Task não reescreve
nenhum desses.

**Contexto — o cursor mora em dois módulos.** `api` grava e lê o cursor por
`repositories.distributionCursorSK` (Task 1); o worker faz o mesmo por `nfseDistCursorSK`. São
módulos Go distintos (`api/go.mod`, `worker/go.mod`) sem pacote compartilhado entre eles hoje,
então a constante é declarada duas vezes com o **mesmo valor literal `"CURSOR"`**. Divergir os
dois valores parte o cursor em dois itens e reprocessa o histórico inteiro a cada ciclo. O Step 6
registra isso no `CONDUCT.md` e o teste do Step 1 trava o valor nos dois lados.

- [x] **Step 1: Escrever o teste que falha**

Crie `worker/internal/service/distribution_nfse_test.go`:

```go
package service

import "testing"

func TestNfseDistCursorSK(t *testing.T) {
	// Deve bater com api/internal/repositories/distributionCursorSK.
	if nfseDistCursorSK != "CURSOR" {
		t.Errorf("nfseDistCursorSK = %q, esperado CURSOR", nfseDistCursorSK)
	}
}

func TestParseNfseDistResponse_LoteComDocumentos(t *testing.T) {
	body := map[string]any{
		"status_distribuicao": "DOCUMENTOS_LOCALIZADOS",
		"distribuicao": []any{
			map[string]any{
				"nsu": float64(11), "chave_acesso": "3526...",
				"tipo_documento": "NFSE", "xml": "<NFSe/>",
			},
			map[string]any{
				"nsu": float64(12), "chave_acesso": "3526...",
				"tipo_documento": "EVENTO", "tipo_evento": "101101", "xml": "<evento/>",
			},
		},
	}

	batch, err := parseNfseDistResponse(body)
	if err != nil {
		t.Fatalf("parseNfseDistResponse: %v", err)
	}
	if batch.Status != nfseDistStatusFound {
		t.Errorf("Status = %q", batch.Status)
	}
	if len(batch.Items) != 2 {
		t.Fatalf("Items = %d, esperado 2", len(batch.Items))
	}
	if batch.Items[0].NSU != 11 || batch.Items[1].NSU != 12 {
		t.Errorf("NSUs = %d, %d", batch.Items[0].NSU, batch.Items[1].NSU)
	}
	if batch.Items[1].TipoEvento != "101101" {
		t.Errorf("TipoEvento = %q", batch.Items[1].TipoEvento)
	}
}

func TestParseNfseDistResponse_LoteVazio(t *testing.T) {
	batch, err := parseNfseDistResponse(map[string]any{
		"status_distribuicao": "NENHUM_DOCUMENTO_LOCALIZADO",
	})
	if err != nil {
		t.Fatalf("parseNfseDistResponse: %v", err)
	}
	if len(batch.Items) != 0 {
		t.Errorf("lote vazio veio com %d itens", len(batch.Items))
	}
	if batch.Status != nfseDistStatusEmpty {
		t.Errorf("Status = %q", batch.Status)
	}
}

func TestMaxNSU(t *testing.T) {
	items := []nfseDistItem{{NSU: 11}, {NSU: 43}, {NSU: 27}}
	if got := maxNSUOf(items); got != 43 {
		t.Errorf("maxNSUOf = %d, esperado 43", got)
	}
	if got := maxNSUOf(nil); got != 0 {
		t.Errorf("maxNSUOf(nil) = %d, esperado 0", got)
	}
}

func TestBuildNfseDistPayload(t *testing.T) {
	p := buildNfseDistPayload("12345678000199", "Y2VydA==", "senha", sefazEnvHom, "nacional", 42)

	if p["doc_type"] != "nfse" {
		t.Errorf("doc_type = %v", p["doc_type"])
	}
	if p["service"] != "NFSeDistribuicao" {
		t.Errorf("service = %v", p["service"])
	}
	if p["uf"] != "" {
		t.Errorf("uf deveria ser vazia em NFS-e, veio %v", p["uf"])
	}
	body, ok := p["body"].(map[string]any)
	if !ok {
		t.Fatalf("body não é map: %T", p["body"])
	}
	if _, exists := body["distDFeInt"]; exists {
		t.Error("payload de NFS-e não pode conter distDFeInt (não é SOAP)")
	}
	if body["nsu"] != int64(42) {
		t.Errorf("nsu = %v", body["nsu"])
	}
	if body["cnpj_consulta"] != "12345678000199" {
		t.Errorf("cnpj_consulta = %v", body["cnpj_consulta"])
	}
	if body["provider"] != "nacional" {
		t.Errorf("provider = %v", body["provider"])
	}
}
```

- [x] **Step 2: Rodar e ver falhar**

Run: `cd worker && go test ./internal/service/ -run 'TestNfseDist|TestParseNfseDist|TestMaxNSU|TestBuildNfseDist' -v`
Expected: FAIL — `undefined: nfseDistCursorSK`.

- [x] **Step 3: Escrever `distribution_nfse.go`**

```go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// nfseDistCursorSK DEVE ter o mesmo valor de
	// api/internal/repositories/distributionCursorSK. Ver CONDUCT.md.
	nfseDistCursorSK = "CURSOR"

	nfseDistStatusFound = "DOCUMENTOS_LOCALIZADOS"
	nfseDistStatusEmpty = "NENHUM_DOCUMENTO_LOCALIZADO"

	// maxNfseDistBatches limita a paginação por invocação. O ADN não devolve
	// maxNSU, então a única condição natural de parada é o lote vazio; sem teto
	// uma organização com histórico grande estouraria o timeout do Lambda.
	// O que sobrar é buscado no próximo ciclo do scheduler.
	maxNfseDistBatches = 20

	serviceNFSeDistribuicao = "NFSeDistribuicao"
)

type nfseDistItem struct {
	NSU           int64
	ChaveAcesso   string
	TipoDocumento string
	TipoEvento    string
	XML           string
}

type nfseDistBatch struct {
	Status string
	Items  []nfseDistItem
}

// buildNfseDistPayload monta o payload de dfe.Request para o ADN. Não há
// envelope distDFeInt: a distribuição de NFS-e é REST (spec §4.1), e o corpo
// é lido por nfse.Dispatch a partir das chaves BodyKey* do go-dfe.
func buildNfseDistPayload(cnpj, certB64, certPassword, sefazEnv, provider string, nsu int64) map[string]any {
	return map[string]any{
		"cnpj":                 cnpj,
		"certificate_b64":      certB64,
		"certificate_password": certPassword,
		"uf":                   "", // NFS-e é competência municipal
		"environment":          sefazEnv,
		"doc_type":             docTypeNfse,
		"service":              serviceNFSeDistribuicao,
		"validate_schema":      false,
		"max_retries":          2,
		"body": map[string]any{
			"provider":      provider,
			"nsu":           nsu,
			"cnpj_consulta": cnpj,
		},
	}
}

// parseNfseDistResponse traduz o nfse.Result serializado pelo go-dfe.
func parseNfseDistResponse(respBody map[string]any) (nfseDistBatch, error) {
	batch := nfseDistBatch{Status: strFromAny(respBody["status_distribuicao"])}

	raw, ok := respBody["distribuicao"].([]any)
	if !ok {
		return batch, nil
	}
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			return batch, fmt.Errorf("item de distribuição não é objeto: %T", r)
		}
		batch.Items = append(batch.Items, nfseDistItem{
			NSU:           int64(getFloat(m, "nsu")),
			ChaveAcesso:   strFromAny(m["chave_acesso"]),
			TipoDocumento: strFromAny(m["tipo_documento"]),
			TipoEvento:    strFromAny(m["tipo_evento"]),
			XML:           strFromAny(m["xml"]),
		})
	}
	return batch, nil
}

func maxNSUOf(items []nfseDistItem) int64 {
	var m int64
	for _, it := range items {
		if it.NSU > m {
			m = it.NSU
		}
	}
	return m
}
```

- [x] **Step 4: Escrever `runNfseDistNSU` no mesmo arquivo**

Fluxo, na ordem exata:

1. `configTable := fmt.Sprintf("%s_organization_nfse_configs", s.cfg.TablePrefix)`; `cfg, err := s.loadConfig(...)`.
   Config ausente → `slog.Warn` + `return nil` (organização sem NFS-e habilitada não é erro).
2. `provider := attrS(cfg, "provider")`. Se `provider != "nacional"`, `return nil` — o ADN só existe no Sistema
   Nacional (spec §4.1); ABRASF 2.04 não distribui.
3. `cert, err := s.loadCert(ctx, orgPK, "nfse_configs")`; `certB64, err := s.getCertB64(...)`;
   `certPassword := attrS(cert, "password")`. Mesmas chamadas de `runDistNSU` — não reescreva.
4. `environment := attrN(cfg, "environment", 2)`; `sefazEnv`/`envPrefix` derivados como em `runDistNSU:198-205`.
5. `distPK := fmt.Sprintf("%s#%s", orgPK, envPrefix)` — o cursor é **por ambiente**. Homologação e produção têm
   sequências de NSU independentes; um cursor único faria a troca de ambiente pular documentos.
6. `nsu, err := s.getNfseLastNSU(ctx, distPK)`.
7. Laço de no máximo `maxNfseDistBatches` iterações:
    - `payload := buildNfseDistPayload(cnpj, certB64, certPassword, sefazEnv, provider, nsu+1)` — o ADN devolve
      documentos **a partir** do NSU informado, então pede-se sempre o próximo.
    - `resp, err := s.invokePyDfe(ctx, payload)` — reusa o desvio para go-dfe já existente (`distribution.go:1082`);
      como F2 põe `NFSeDistribuicao` no mapa `implemented`, a chamada nunca cai no Lambda py-dfe. Erro →
      `return fmt.Errorf("invokePyDfe nfse: %w", err)` (o SQS re-entrega).
    - `statusCode != 200` → `slog.Error` com `detail` + `return nil` (terminal; erro do ADN não melhora com retry).
    - `batch, err := parseNfseDistResponse(respBody)`.
    - `len(batch.Items) == 0` → `slog.Info("nenhum documento novo", ...)` + `break`.
    - Para cada item: `s.persistNfseIncoming(ctx, orgPK, distPK, it, dtcfg)`.
    - `nsu = maxNSUOf(batch.Items)`; `s.setNfseLastNSU(ctx, distPK, nsu)`.
8. `return nil`.

`getNfseLastNSU` / `setNfseLastNSU` são os espelhos exatos de `GetLastNSU`/`SetLastNSU` da Task 1
(mesma tabela `{prefix}_nfse_distributions`, mesma chave, mesma `ConditionExpression`
`attribute_not_exists(last_nsu) OR last_nsu < :n`, mesmo tratamento de
`ConditionalCheckFailedException` como sucesso silencioso). Copie a semântica, não invente outra.

`persistNfseIncoming` grava o XML em `{org_pk}/nfse/incoming/{nsu}.xml` no S3 e um item em
`{prefix}_nfse_distributions` com `pk = distPK`, `sk = fmt.Sprintf("NSU#%015d", it.NSU)`,
`access_key`, `doc_type`, `event_type`, `s3_key`, `received_at`. Use `s.s3` e `s.dynamo` já
injetados. Não chame `processDocZip`: aquele decodifica o gzip+base64 do DistDFe da NF-e e faz
parsing de `procNFe`/`resNFe`, que não existem em NFS-e — o go-dfe já entrega o XML descompactado
em `it.XML`.

- [x] **Step 5: Registrar `nfse` em `docTypeConfigs` e no `Process`**

Em `distribution.go`, dentro de `docTypeConfigs` (`:64`):

```go
    // NFS-e não usa SOAP: sefazService/xmlns/version ficam vazios de
// propósito — a distribuição é REST via ADN e o payload é montado por
// buildNfseDistPayload, não por buildPayload.
"nfse": {
configTableSuffix: "nfse_configs",
distTable:         "nfse_distributions",
docTable:          "nfses",
eventsTable:       "nfse_events",
},
```

E em `Process` (`:159`), no `case "dist_nsu", "":`

```go
    case "dist_nsu", "":
if msg.DocType == docTypeNfse {
return s.runNfseDistNSU(ctx, msg.OrgPK, msg.Trigger, dtcfg)
}
return s.runDistNSU(ctx, msg.OrgPK, msg.DocType, msg.Trigger, dtcfg)
```

Os jobs `cons_nsu` e `cons_ch_nfe` **não** ganham variante NFS-e nesta fase: a consulta pontual de
NFS-e já é atendida por `GET /nfses/{id}` (Task 5), que consulta o fisco direto pela chave. Deixe
o `default` atual devolvê-los como no-op para `doc_type = nfse`.

Em `worker/cmd/distribution-dispatcher/main.go:17`:

```go
var docTypes = []string{"nfe", "cte", "mdfe", "nfse"}
```

O dispatcher varre `{prefix}_organization_nfse_configs` — tabela criada na F1 — e enfileira um job
por organização, sem nenhuma outra mudança: o `scanOrgPKs` já é genérico sobre o nome da tabela.

- [x] **Step 6: Registrar a duplicação da constante no `CONDUCT.md`**

Acrescente à seção de decisões duráveis:

```markdown
### Cursor de NSU da distribuição de NFS-e

`api/internal/repositories/distributionCursorSK` e
`worker/internal/service/nfseDistCursorSK` são a mesma chave lógica (`"CURSOR"`) declarada em dois
módulos Go independentes, porque `api` e `worker` não compartilham pacote. Mudar um sem o outro
parte o cursor em dois itens e faz a distribuição reprocessar o histórico inteiro a cada ciclo.
Ambos têm teste que trava o valor literal. Se um pacote comum surgir entre os dois módulos, esta é
a primeira constante a migrar.
```

- [x] **Step 7: Rodar os testes**

Run: `cd worker && go build ./... && go test ./... -race`
Expected: PASS.


**Desvios do plano na implementação (2026-08-05):**

- **O cursor não está mais na tabela de distribuição.** O commit `7272cc4` moveu o NSU para
  `organization_nfse_configs` (`{env}_nsu`, `{env}_last_dist_nsu_at`), igual à família NF-e. Caem com ele o
  `nfseDistCursorSK`, o `getNfseLastNSU`/`setNfseLastNSU`, o teste do Step 1 que travava a constante e o Step 6
  inteiro (não há mais constante duplicada entre `api` e `worker`). `runNfseDistNSU` reusa `updateNSU` e
  `claimDistNSUSlot` sem cópia.
- **`parseNfseDistResponse` não devolve `error`** — item malformado é logado e pulado; abortar o ciclo por causa de
  um NSU perderia o lote inteiro.
- **`nfseDistStatusFound`/`nfseDistStatusEmpty` não existem.** A parada é `len(batch.Items) == 0`; nenhuma
  comparação usa os literais, então nomear os dois seria constante morta. O status vai para o log.
- **XML recebido segue a convenção dos outros docTypes**: `nfse-distribution/{env}/{org_pk}/NSU_{015d}.xml`, e não o
  `{org_pk}/nfse/incoming/{nsu}.xml` do plano. O registro em `nfse_distributions` usa `pk = {env}#{org_pk}`, que é o
  que `NfseService.ListDistributions` consulta.
- **`mapToDfeRequest` ganhou exceção de UF vazia para NFS-e** (fora do escopo listado). Sem isso o payload da
  distribuição era rejeitado pelo guard e caía no py-dfe, que não implementa NFS-e — a rotina inteira seria no-op.
  Registrado no `CONDUCT.md`.
- **`mockLambda` ganhou `payloads [][]byte`** para o teste de paginação (lote + lote vazio).
- **Rate limit reusa `claimDistNSUSlot` uma vez por invocação**, não por iteração como a NF-e: o ADN não documenta
  limite por chamada, e os campos `{env}_last_dist_nsu_at` já existem no config de NFS-e.

- [x] **Step 8: Commit**

```bash
git add worker/internal/service/distribution_nfse.go worker/internal/service/distribution_nfse_test.go \
        worker/internal/service/distribution.go worker/cmd/distribution-dispatcher/main.go CONDUCT.md
git commit -m "feat(nfse): distribuicao ADN por cursor de NSU no worker"
```

---

### Task 9: Testes de integração e documentação

**Files:**

- Create: `api/tests/integration/nfses_test.go`
- Modify: `DOCS.md`, `OVERVIEW.md`, `INTEGRATION.md`, `CONDUCT.md`
- Modify: `docs/specs/2026-08-04-nfse-design.md` (marcar F3 como concluída)
- Modify: `api/CLAUDE.md`, `worker/CLAUDE.md`

**Interfaces:**

- Consumes: tudo produzido pelas Tasks 1–8.
- Produces: nenhuma API nova. Esta task só fecha a cobertura e a documentação.

**Contexto:** a regra do `CLAUDE.md` raiz — "Every core function must be covered by an integration
test in addition to unit tests", e a linha "Fiscal issuance → Unit + integration". A emissão de
NFS-e é emissão fiscal. Sem esta task a fase está incompleta, não "quase pronta".

Confira antes de escrever como os testes de integração existentes sobem suas dependências:
`rg -l "func TestMain" api/tests/integration/` e leia o mais próximo (`nfes_test.go`). **Não**
introduza um segundo mecanismo de bootstrap.

- [x] **Step 1: Escrever o teste de integração da emissão**

Crie `api/tests/integration/nfses_test.go` cobrindo, cada um como subteste nomeado:

| Subteste                        | Verifica                                                                                                                                                 |
|---------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------|
| `EmitPersisteComSKIDDPS`        | `POST /v1.0/nfses` grava item em `nfses` com `sk` = `id_dps` de 45 caracteres começando em `DPS`, `status = processing`, e **sem** atributo `access_key` |
| `EmitEnfileiraOutbox`           | a transação escreveu o registro correspondente em `worker_outbox` com `operation_id = "{table}#{id_dps}"`                                                |
| `EmitDuplicadoRejeita`          | segunda emissão com o mesmo `serie`+`numero` devolve 409 Problem JSON, e o item original permanece intacto                                               |
| `EmitSemRegTribRejeita`         | prestador sem grupo `nfse.reg_trib` no cadastro devolve 422 Problem JSON citando o campo                                                                 |
| `GetNfsePorIDDPSEPorChave`      | `GET /v1.0/nfses/{id}` resolve tanto pelo `id_dps` quanto pela `access_key` (via GSI) e devolve o mesmo item                                             |
| `ListNfsesFiltraPorCompetencia` | `GET /v1.0/nfses?year=&month=` devolve só a competência pedida                                                                                           |
| `EventoCancelamentoExigeMotivo` | `POST /v1.0/nfses/{id}/events` com `event_type = 101101` sem `motivo` devolve 422                                                                        |
| `EventoFiscoRejeitado`          | `event_type = 105104` devolve 422 com mensagem de que é evento de emissão exclusiva do fisco                                                             |
| `SubstituicaoNaoEhEvento`       | `event_type = 105102` devolve 422 apontando `/substitute`                                                                                                |
| `ParametrosMunicipaisUsamCache` | duas chamadas seguidas a `GET /v1.0/nfse/municipal-parameters/...` fazem **uma** chamada ao provider (stub contando invocações)                          |

Toda asserção de erro deve checar o `Content-Type: application/problem+json` e o campo `type`, não
só o status — é a regra de erro do `CLAUDE.md` raiz e o teste é o que a mantém viva.

- [x] **Step 2: Rodar e ver falhar**

Run: `cd api && go test ./tests/integration/ -run TestNfse -v`
Expected: FAIL nos subtestes ainda não cobertos pelas Tasks 1–7 (se algum falhar por bug real,
corrija o código de produção, nunca o teste).

- [x] **Step 3: Fazer passar**

Run: `cd api && go test ./tests/integration/ -run TestNfse -v`
Expected: PASS em todos os subtestes.

- [x] **Step 4: Rodar a bateria completa dos dois projetos**

Run:

```bash
cd api && go build ./... && go vet ./... && go test ./... -race
cd ../worker && go build ./... && go vet ./... && go test ./... -race
```

Expected: PASS, zero falhas. Se a F2 já estiver mesclada, rode também `cd go-dfe && go test ./...`
para garantir que a assinatura de `nfse.Dispatch` consumida aqui não mudou.

- [x] **Step 5: `DOCS.md`**

Acrescente:

1. As 13 rotas da Task 6 na tabela de endpoints, com método, permissão exigida e forma do corpo.
2. O contrato de `NfseEmitBody` — os campos obrigatórios e as três validações condicionais
   (`tp_emit` 2/3 → `c_motivo_emis_ti`; `op_simp_nac = 3` → `reg_ap_trib_sn`; ABRASF exige config
   completa).
3. A tabela de status de `nfses` (`processing`, `authorized`, `rejected`, `cancelled`) e quem
   escreve cada um (API na criação, worker no desfecho).
4. Os caminhos de S3 da spec §6, verbatim.
5. Uma linha na tabela de tabelas: `nfse_distributions` com `pk = {org_pk}#{env}`,
   `sk = NSU#{015d}` mais o item de cursor `sk = CURSOR`.

- [x] **Step 6: `OVERVIEW.md`**

No diagrama/descrição de fluxo, acrescente o caminho de NFS-e ao lado do de NF-e, marcando o que
muda: `API → outbox → SQS → worker → go-dfe (in-process, REST/mTLS) → Sefin Nacional`, sem
py-dfe em nenhum ponto, e a distribuição `scheduler → distribution-dispatcher → SQS →
distribution-worker → go-dfe → ADN`.

- [x] **Step 7: `INTEGRATION.md`**

Documente para o front (consumido pela F4): a forma exata do corpo de emissão, o formato do
`id` aceito por `GET /nfses/{id}` (aceita `id_dps` **ou** chave de 50 dígitos), o polling/WebSocket
do desfecho, e a lista de `event_type` que o front pode oferecer ao usuário — que é estritamente o
`ContribuinteEvents` da F2, não a lista completa de eventos.

- [x] **Step 8: `CONDUCT.md`**

Registre duas decisões duráveis (além da do Step 6 da Task 8):

```markdown
### SK de `nfses` é o `id_dps`, não a chave de acesso

A chave de acesso de 50 dígitos contém `nNFSe` e `cNum`, gerados pelo fisco — não existe no momento
do insert. O `id_dps` (45 caracteres, `DPS` + `cLocEmi` + `tpInsc` + `inscFederal` + `serie` +
`nDPS`) é conhecido antes da chamada e é o mesmo valor assinado no atributo `Id` do XML. A
`access_key` entra como atributo quando o fisco responde e é consultável pela GSI
`access-key-index`. Nunca gravar `access_key` vazia: poluiria a GSI.

### `WorkerMessage.AccessKey` carrega o `id_dps` quando `DocType = "nfse"`

Nenhum campo novo foi acrescentado ao `WorkerMessage`, porque o campo já significa "a SK do
documento na sua tabela" em todos os tipos. Em NFS-e essa SK é o `id_dps`. `updateClaimedDocument`
depende disso; trocar por `out.AccessKey` produziria um item órfão.
```

- [x] **Step 9: `api/CLAUDE.md` e `worker/CLAUDE.md`**

Uma linha em cada: onde vive o serviço de NFS-e (`api/internal/services/nfses/`) e o desvio do
worker (`worker/internal/service/nfse.go`, `distribution_nfse.go`), com a advertência de que a
resposta de NFS-e não tem `cStat`/`xMotivo` e a rejeição é sempre terminal.

- [x] **Step 10: Marcar F3 na spec**

Em `docs/specs/2026-08-04-nfse-design.md`, na tabela de fases (§ final), marque F3 como concluída
com a data.

**Desvios do plano na implementação (2026-08-05):**

- **O harness de integração é de serviço, não de HTTP.** `api/tests/integration/` não sobe o app Fiber
  (não existe `nfes_test.go`); os testes chamam o serviço e verificam o `*problem.Problem` devolvido.
  As asserções são `problemStatus` + `problemType` — o `Content-Type: application/problem+json` é
  responsabilidade de `Problem.Send`, coberta na camada de rota. Não foi introduzido um segundo
  bootstrap, como o plano exige.
- **Status na criação é `pending`, não `processing`**, e `problem.BadRequest` é **400**, não 422 — os
  subtestes seguem o código, não a tabela do plano.
- **`EmitDuplicadoRejeita` reproduz o conflito real.** Duas emissões seguidas nunca colidem: o número
  é reservado na mesma transação do `Put`. O teste recua o contador para que a emissão seguinte
  recalcule um `id_dps` já existente — o mesmo estado de um retry concorrente — e confirma 409 +
  linha original intacta.
- **`ParametrosMunicipaisUsamCache` virou `ParametrosMunicipaisValidamAridade`.** `callGoDfe` chama
  `godfe.Call` direto (sem ponto de stub na api), então contar invocações do provider exigiria rede.
  A chave de cache e a aridade já são cobertas por `TestCacheKeyMunicipalParams_ExcludesTenant` e
  `TestMunicipalParamKind_Validation`; o subtest de integração prova que a validação acontece **antes**
  de qualquer ida ao ADN.
- **Subteste extra `EventoExigeNfseAutorizada`**: evento sobre NFS-e ainda pendente é recusado.
- **Bug real corrigido (Step 3).** `NfseListOpts.Year`/`Month` eram parseados pela rota e descartados
  por `ListNfses`. O filtro foi implementado como `FilterExpression` sobre a partição, não pela
  `dfe-index`: aquela GSI é chaveada por `incoming`, que NFS-e não tem. Verificado vermelho antes,
  verde depois.
- **Tabelas novas no harness**: `nfses` (com `access-key-index`), `nfse_events` e `worker_outbox`.
  `nfse_distributions` ficou de fora — `ListDistributions` não é exercida aqui.
- **`DynamoDB-Tables.md`**: o índice de tabelas ainda dizia `pk = {org_pk}` nas quatro tabelas de
  distribuição; corrigido para `{env}#{org_pk}`, alinhando com a §19–22.

- [x] **Step 11: Commit**

```bash
git add api/tests/integration/ api/internal/repositories/nfses.go DOCS.md OVERVIEW.md INTEGRATION.md \
        CONDUCT.md DynamoDB-Tables.md api/CLAUDE.md worker/CLAUDE.md \
        docs/specs/2026-08-04-nfse-design.md docs/plans/2026-08-04-nfse-f3-api-worker.md
git commit -m "docs(nfse): documenta API e worker de NFS-e; testes de integracao"
```

---

## Impacto entre projetos

| Projeto   | Impacto                                                                                                                                | Revisado          |
|-----------|----------------------------------------------------------------------------------------------------------------------------------------|-------------------|
| `api/`    | Repositórios, serviço `nfses`, 13 rotas, RBAC, escopos, fx                                                                             | Sim — Tasks 1–6   |
| `worker/` | Desvio de NFS-e no `Process`, persistência, distribuição ADN                                                                           | Sim — Tasks 7–8   |
| `go-dfe/` | **Nenhuma alteração.** Consumido pela superfície pública que a F2 fixou (`nfse.Document`, `nfse.Dispatch`, `nacional.BuildIDDPS`)      | Sim — só consumo  |
| `py-dfe/` | **Nenhuma alteração.** NFS-e nunca passa por lá                                                                                        | Sim               |
| `cdk/`    | **Nenhuma alteração nesta fase.** As tabelas `nfses`, `nfse_events`, `nfse_distributions` e a GSI `access-key-index` são criadas na F1 | Sim               |
| `ui/`     | Consome as rotas desta fase na F4                                                                                                      | Não alterado aqui |

## O que a F3 deliberadamente NÃO faz

- **Não implementa ABRASF 2.04.** As rotas aceitam `provider = "abrasf204"` na validação de
  configuração, mas a emissão devolve o erro do go-dfe e o DANFSE devolve 501. F5.
- **Não implementa manifestação sobre documentos recebidos.** A distribuição grava o recebido; as
  ações de confirmar/rejeitar da spec §7 dependem de eventos que o tomador emite e ficam para
  depois da F4, quando houver tela.
- **Não faz `cons_nsu` nem `cons_ch_nfe` para NFS-e.** A consulta pontual é atendida por
  `GET /nfses/{id}`.
- **Não gera DANFSE própria.** É proxy do PDF do ADN.
- **Não mexe no fluxo de NF-e/CT-e/MDF-e.** Toda mudança em arquivo compartilhado
  (`distribution.go`, `dfe.go`, `main.go` do dispatcher) é aditiva e guardada por `docType`.
- **Não altera `WorkerMessage`.** Ver a decisão registrada na Task 9, Step 8.
