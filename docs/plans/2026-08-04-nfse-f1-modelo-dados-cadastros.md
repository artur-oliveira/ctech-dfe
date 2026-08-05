# NFS-e — Fase F1: Modelo de Dados e Cadastros — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Entregar toda a base de dados e cadastro que a emissão de NFS-e vai consumir — catálogo de serviços, grupo `nfse` em organizações e pessoas, config NFS-e por organização, tabelas DynamoDB e tabelas de referência dos Anexos B e C — sem ainda emitir nenhum documento.

**Arquitetura:** Espelha o que já existe. `organization_services` é um clone estrutural de `organization_products` (CRUD genérico + `CRUDRepository` + `CRUDMutationHelper`). `organization_nfse_configs` é um quinto config singleton sobre `FiscalConfigRepository`. A extensão de identidade entra em `PersonObjectBody`, o objeto `person` já compartilhado por `organizations` e `organization_persons` — uma mudança cobre as duas tabelas. As tabelas de referência (código de tributação nacional, NBS, `indOp`) viram um pacote Go exportado em `go-dfe`, consumido pela `api` para validação e depois pela própria `go-dfe` na montagem do XML.

**Tech Stack:** Go 1.26 (Fiber v3, aws-sdk-go-v2, go-playground/validator, uber/fx), AWS CDK v2 TypeScript, DynamoDB, Next.js/TypeScript (só dados de referência nesta fase), Python 3 + openpyxl (apenas para gerar as tabelas, não roda em produção).

**Spec:** `docs/specs/2026-08-04-nfse-design.md` §3.1, §3.2, §3.3, §3.7, §8.

## Global Constraints

- Erros de api/worker SEMPRE em RFC 7807, via `problem.BadRequest` / `problem.NotFound` / `problem.InternalServer`. Rotas respondem por `sendProblem(c, err)`. Nunca `fiber.Map`, nunca `fiber.NewError`, nunca erro cru.
- Separação de camadas rígida: Repository só toca DynamoDB; Service só regra de negócio e cache; Route só parseia, chama UM método de service e responde.
- Zero string mágica. Todo nome de tabela, chave, prefixo de cache, tag de validação e código fiscal é constante nomeada.
- DRY: antes de criar qualquer função, `rg` no `internal/`. Duas implementações do mesmo problema devem ser unificadas.
- Serviços e repositórios são injetados por `go.uber.org/fx`. Nunca instanciar dentro de handler.
- DynamoDB: `GetItem` > `Query` > `Scan`. Sem scan em produção.
- `RemovalPolicy.DESTROY` é exclusivo de dev. PITR só em staging/prod.
- Prefixo de tabela no CDK: `TablePrefix = ${Environment}_dfe` — nomes sempre derivados, nunca literais completos.
- Testes: `go test ./... -race` na `api/`. Integração roda com build tag `integration`. `npm test` e `cdk synth` na `cdk/`.
- Nenhum commit pode conter certificado PFX, credencial AWS, segredo JWT, CPF/CNPJ real ou dado de cliente real. CNPJs de teste vêm de `randomCNPJ()`.
- Nenhum commit leva trailer `Co-Authored-By: Claude` — preferência global do usuário.
- Toda mudança de comportamento leva a atualização de documentação no MESMO commit.
- Commits em Conventional Commit, sem emoji.

---

## Estrutura de arquivos

| Arquivo | Responsabilidade | Ação |
|---|---|---|
| `cdk/lib/dynamodb-stack.ts` | Definição das 4 tabelas novas + `TableName` | Modificar |
| `cdk/test/dynamodb-stack.test.ts` | Snapshot/asserções das tabelas novas | Criar |
| `cdk/lib/worker-stack.ts` | ARNs de config para o distribution-dispatcher | Modificar |
| `go-dfe/nfse/tables/trib_nacional.go` | Tabela do código de tributação nacional (Anexo B) — gerada | Criar |
| `go-dfe/nfse/tables/nbs.go` | Tabela NBS 2.0 (Anexo B) — gerada | Criar |
| `go-dfe/nfse/tables/indop.go` | Tabela `indOp` IBS/CBS (Anexo C) — gerada | Criar |
| `go-dfe/nfse/tables/lookup.go` | API de consulta das três tabelas | Criar |
| `go-dfe/nfse/tables/lookup_test.go` | Testes de consulta | Criar |
| `go-dfe/nfse/tables/gen/generate.py` | Gerador xlsx → Go + JSON | Criar |
| `ui/src/lib/data/nfse_trib_nacional.ts` | Mesma tabela para o front — gerada | Criar |
| `ui/src/lib/data/nfse_indop.ts` | Mesma tabela para o front — gerada | Criar |
| `api/go.mod` | Dependência + replace para `go-dfe` | Modificar |
| `api/internal/repositories/services.go` | `ServiceRepository` (organization_services) | Criar |
| `api/internal/repositories/fiscal_configs.go` | `NfseConfigRepository` | Modificar |
| `api/internal/repositories/audit_logs.go` | Constantes `AuditResourceService`, `AuditResourceNfseConfig` | Modificar |
| `api/internal/repositories/roles.go` | Recursos RBAC novos | Modificar |
| `api/internal/middleware/scopes.go` | Famílias de escopo novas | Modificar |
| `api/internal/services/services.go` | `ServiceService` (catálogo) | Criar |
| `api/internal/services/fiscal_configs.go` | Base genérica + `NfseConfigService` | Modificar |
| `api/internal/api/v1/dto.go` | `ServiceBody`, `NfseInfoBody`, `NfseConfigBody`, extensão de `PersonObjectBody` | Modificar |
| `api/internal/api/v1/services.go` | Rotas `/services` | Criar |
| `api/internal/api/v1/organizations.go` | Rota `/nfse-config` | Modificar |
| `api/internal/api/v1/router.go` | Registro das rotas novas | Modificar |
| `api/internal/app/app.go` | Providers fx novos | Modificar |
| `api/internal/validation/validators.go` | Tags `tribnac`, `nbs`, `inscmun`, `caepf`, `indop` | Modificar |
| `api/internal/validation/validation.go` | Mensagens das tags novas | Modificar |
| `api/tests/integration/setup_test.go` | Bootstrap dos serviços novos | Modificar |
| `api/tests/integration/services_test.go` | Integração do catálogo | Criar |
| `api/tests/integration/nfse_configs_test.go` | Integração da config | Criar |
| `DynamoDB-Tables.md`, `DOCS.md`, `OVERVIEW.md`, `CONDUCT.md` | Documentação | Modificar |

---

### Task 1: Tabelas de referência dos Anexos B e C (`go-dfe/nfse/tables`)

Primeira porque a validação da `api` (Task 5) depende dela.

**Files:**
- Create: `go-dfe/nfse/tables/gen/generate.py`
- Create: `go-dfe/nfse/tables/trib_nacional.go` (gerado)
- Create: `go-dfe/nfse/tables/nbs.go` (gerado)
- Create: `go-dfe/nfse/tables/indop.go` (gerado)
- Create: `go-dfe/nfse/tables/lookup.go`
- Test: `go-dfe/nfse/tables/lookup_test.go`

**Interfaces:**
- Consumes: nada.
- Produces: pacote `gopkg.aoctech.app/dfe/go-dfe/nfse/tables` exportando
  `IsValidTribNacional(code string) bool`, `TribNacional(code string) (TribNacionalEntry, bool)`,
  `IsValidNBS(code string) bool`, `IsValidIndOp(code string) bool`,
  e os tipos `TribNacionalEntry{Code, Item, Subitem, Desdobro, Description string}`,
  `NBSEntry{Code, Description string}`,
  `IndOpEntry{Code, TipoOperacao, CaracteristicaFornecimento, LocalFornecimento, CampoLeiaute string}`.

**Contexto para quem implementa:** as planilhas fonte estão em `tmp/` na raiz do repositório. Estrutura já verificada:

- `tmp/anexo_b-nbs2-lista_servico_nacional-snnfse-v1-01-20260122.xlsx`
  - aba `LISTA.SERV.NAC.` — cabeçalho na linha 1: `CÓDIGO DE TRIBUTAÇÃO NACIONAL | ITEM | SUBITEM | DESDOBRO NACIONAL | DESCRIÇÃO`. Linhas com a primeira coluna vazia são cabeçalhos de agrupamento (item/subitem sem desdobro) — **descartar**; só entram linhas com código preenchido (5 dígitos).
  - aba `LISTA.NBS_v2.0` — cabeçalho `CÓDIGO NBS | DESCRIÇÃO`. Os códigos vêm com pontos (`1.0101.11.00`); normalizar removendo `.`.
- `tmp/anexo_c-indop_ibscbs-snnfse-v1-01-20260122.xlsx`, aba `IndOp`, cabeçalho na linha 1. A coluna 7 (índice 6, zero-based) é `Código indOp`, com 6 dígitos. Colunas 2, 4, 8 e 9 (índices 1, 3, 7, 8) são `Tipo de operação`, `Característica do fornecimento`, `Local do fornecimento` e `Campo no leiaute da NFSe`. Células mescladas deixam valores `None` nas linhas seguintes — propagar o último valor não-nulo para frente (forward-fill) nessas quatro colunas. Linhas sem código `indOp` são descartadas.

- [ ] **Step 1: Escrever o teste que falha**

Crie `go-dfe/nfse/tables/lookup_test.go`:

```go
package tables

import "testing"

func TestTribNacional_KnownCode(t *testing.T) {
	e, ok := TribNacional("10101")
	if !ok {
		t.Fatal("TribNacional(10101): esperado encontrado")
	}
	if e.Item != "1" || e.Subitem != "1" || e.Desdobro != "1" {
		t.Errorf("item/subitem/desdobro = %q/%q/%q, esperado 1/1/1", e.Item, e.Subitem, e.Desdobro)
	}
	if e.Description == "" {
		t.Error("descrição vazia")
	}
}

func TestTribNacional_RejectsGroupingRows(t *testing.T) {
	// "1" e "10" são cabeçalhos de item/subitem na planilha, nunca códigos válidos.
	for _, code := range []string{"", "1", "10", "99999"} {
		if IsValidTribNacional(code) {
			t.Errorf("IsValidTribNacional(%q) = true, esperado false", code)
		}
	}
}

func TestTribNacional_AllCodesAreFiveDigits(t *testing.T) {
	if len(tribNacionalTable) < 500 {
		t.Fatalf("tabela com %d entradas, esperado >= 500", len(tribNacionalTable))
	}
	for code := range tribNacionalTable {
		if len(code) != 5 {
			t.Errorf("código %q tem %d dígitos, esperado 5", code, len(code))
		}
	}
}

func TestNBS_NormalizedCodes(t *testing.T) {
	if len(nbsTable) < 1000 {
		t.Fatalf("tabela NBS com %d entradas, esperado >= 1000", len(nbsTable))
	}
	for code := range nbsTable {
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("código NBS %q contém caractere não numérico", code)
			}
		}
	}
	if IsValidNBS("1.0101.11.00") {
		t.Error("código NBS com pontos deveria ser inválido — normalize antes de consultar")
	}
}

func TestIndOp_KnownCode(t *testing.T) {
	if !IsValidIndOp("020101") {
		t.Error("IsValidIndOp(020101) = false, esperado true")
	}
	if IsValidIndOp("999999") {
		t.Error("IsValidIndOp(999999) = true, esperado false")
	}
}

func TestIndOp_MergedCellsForwardFilled(t *testing.T) {
	// 020201 e 020301 herdam "Art. 11 / Inc. II" de células mescladas;
	// o gerador deve ter propagado os textos, não deixado vazio.
	for _, code := range []string{"020201", "020301"} {
		e, ok := indOpTable[code]
		if !ok {
			t.Fatalf("indOp %s ausente", code)
		}
		if e.TipoOperacao == "" || e.LocalFornecimento == "" {
			t.Errorf("indOp %s com campo vazio: %+v", code, e)
		}
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd go-dfe && go test ./nfse/tables/...
```

Esperado: FAIL — o pacote não existe.

- [ ] **Step 3: Escrever o gerador**

Crie `go-dfe/nfse/tables/gen/generate.py`:

```python
#!/usr/bin/env python3
"""Gera as tabelas de referência NFS-e a partir dos Anexos B e C.

Uso (a partir da raiz do repositório):
    python3 go-dfe/nfse/tables/gen/generate.py

Entrada:  tmp/anexo_b-*.xlsx, tmp/anexo_c-*.xlsx
Saída:    go-dfe/nfse/tables/{trib_nacional,nbs,indop}.go
          ui/src/lib/data/{nfse_trib_nacional,nfse_indop}.ts

As saídas são versionadas no repositório; este script não roda em produção.
"""
import glob
import json
import os

import openpyxl

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
GO_DIR = os.path.join(ROOT, "go-dfe", "nfse", "tables")
UI_DIR = os.path.join(ROOT, "ui", "src", "lib", "data")

HEADER = "// Code generated by nfse/tables/gen/generate.py. DO NOT EDIT.\n\npackage tables\n\n"


def find(pattern):
    matches = glob.glob(os.path.join(ROOT, "tmp", pattern))
    if not matches:
        raise SystemExit(f"planilha não encontrada: tmp/{pattern}")
    return sorted(matches)[-1]


def rows(path, sheet):
    ws = openpyxl.load_workbook(path, read_only=True, data_only=True)[sheet]
    it = ws.iter_rows(values_only=True)
    next(it)  # descarta o cabeçalho
    return list(it)


def clean(v):
    return "" if v is None else str(v).replace("\n", " ").strip()


def go_quote(s):
    return json.dumps(s, ensure_ascii=False)


def parse_trib_nacional(path):
    out = []
    for row in rows(path, "LISTA.SERV.NAC."):
        code = clean(row[0])
        if not code:
            continue  # linha de agrupamento (item/subitem sem desdobro)
        out.append({
            "code": code,
            "item": clean(row[1]),
            "subitem": clean(row[2]),
            "desdobro": clean(row[3]),
            "description": clean(row[4]),
        })
    return out


def parse_nbs(path):
    out = []
    for row in rows(path, "LISTA.NBS_v2.0"):
        code = clean(row[0]).replace(".", "")
        if not code:
            continue
        out.append({"code": code, "description": clean(row[1])})
    return out


def parse_indop(path):
    # Colunas mescladas: propaga o último valor não-vazio.
    carry = {1: "", 3: "", 7: "", 8: ""}
    out = []
    for row in rows(path, "IndOp"):
        for i in carry:
            v = clean(row[i]) if i < len(row) else ""
            if v:
                carry[i] = v
        code = clean(row[6]) if len(row) > 6 else ""
        if not code:
            continue
        out.append({
            "code": code,
            "tipo_operacao": carry[1],
            "caracteristica_fornecimento": carry[3],
            "local_fornecimento": carry[7],
            "campo_leiaute": carry[8],
        })
    return out


def write_go(filename, decl, entries, fields):
    lines = [HEADER, decl, "\n"]
    for e in entries:
        vals = ", ".join(f"{go_field}: {go_quote(e[key])}" for go_field, key in fields)
        lines.append(f'\t{go_quote(e["code"])}: {{{vals}}},\n')
    lines.append("}\n")
    with open(os.path.join(GO_DIR, filename), "w", encoding="utf-8") as fh:
        fh.write("".join(lines))
    print(f"{filename}: {len(entries)} entradas")


def write_ts(filename, const_name, ts_type, entries):
    with open(os.path.join(UI_DIR, filename), "w", encoding="utf-8") as fh:
        fh.write("// Code generated by go-dfe/nfse/tables/gen/generate.py. DO NOT EDIT.\n\n")
        fh.write(ts_type + "\n\n")
        fh.write(f"export const {const_name} = ")
        fh.write(json.dumps(entries, ensure_ascii=False, indent=2))
        fh.write(" as const;\n")
    print(f"{filename}: {len(entries)} entradas")


def main():
    anexo_b = find("anexo_b-*.xlsx")
    anexo_c = find("anexo_c-*.xlsx")

    trib = parse_trib_nacional(anexo_b)
    nbs = parse_nbs(anexo_b)
    indop = parse_indop(anexo_c)

    write_go("trib_nacional.go",
             "var tribNacionalTable = map[string]TribNacionalEntry{\n", trib,
             [("Code", "code"), ("Item", "item"), ("Subitem", "subitem"),
              ("Desdobro", "desdobro"), ("Description", "description")])
    write_go("nbs.go",
             "var nbsTable = map[string]NBSEntry{\n", nbs,
             [("Code", "code"), ("Description", "description")])
    write_go("indop.go",
             "var indOpTable = map[string]IndOpEntry{\n", indop,
             [("Code", "code"), ("TipoOperacao", "tipo_operacao"),
              ("CaracteristicaFornecimento", "caracteristica_fornecimento"),
              ("LocalFornecimento", "local_fornecimento"),
              ("CampoLeiaute", "campo_leiaute")])

    write_ts("nfse_trib_nacional.ts", "NFSE_TRIB_NACIONAL",
             "interface TribNacionalEntry {\n  code: string;\n  item: string;\n"
             "  subitem: string;\n  desdobro: string;\n  description: string;\n}",
             trib)
    write_ts("nfse_indop.ts", "NFSE_INDOP",
             "interface IndOpEntry {\n  code: string;\n  tipo_operacao: string;\n"
             "  caracteristica_fornecimento: string;\n  local_fornecimento: string;\n"
             "  campo_leiaute: string;\n}",
             indop)


if __name__ == "__main__":
    main()
```

- [ ] **Step 4: Escrever a API de consulta**

Crie `go-dfe/nfse/tables/lookup.go`:

```go
// Package tables carrega as tabelas de referência da NFS-e nacional
// (Anexo B: código de tributação nacional e NBS 2.0; Anexo C: indOp IBS/CBS).
//
// Os arquivos *_table.go são gerados por gen/generate.py a partir das planilhas
// oficiais e versionados no repositório — não edite à mão. Regenerar:
//
//	python3 go-dfe/nfse/tables/gen/generate.py
package tables

// TribNacionalEntry é uma linha da lista de serviços nacional (Anexo B).
// Code tem 5 dígitos: item(2) + subitem(2) + desdobro(1).
type TribNacionalEntry struct {
	Code        string
	Item        string
	Subitem     string
	Desdobro    string
	Description string
}

// NBSEntry é uma linha da Nomenclatura Brasileira de Serviços 2.0 (Anexo B).
// Code é normalizado sem pontos.
type NBSEntry struct {
	Code        string
	Description string
}

// IndOpEntry é uma linha da tabela indOp do IBS/CBS (Anexo C).
type IndOpEntry struct {
	Code                       string
	TipoOperacao               string
	CaracteristicaFornecimento string
	LocalFornecimento          string
	CampoLeiaute               string
}

// TribNacional devolve a entrada do código de tributação nacional.
func TribNacional(code string) (TribNacionalEntry, bool) {
	e, ok := tribNacionalTable[code]
	return e, ok
}

// IsValidTribNacional informa se o código de tributação nacional existe.
func IsValidTribNacional(code string) bool {
	_, ok := tribNacionalTable[code]
	return ok
}

// NBS devolve a entrada da NBS 2.0. code deve vir sem pontos.
func NBS(code string) (NBSEntry, bool) {
	e, ok := nbsTable[code]
	return e, ok
}

// IsValidNBS informa se o código NBS existe. code deve vir sem pontos.
func IsValidNBS(code string) bool {
	_, ok := nbsTable[code]
	return ok
}

// IndOp devolve a entrada de indOp do IBS/CBS.
func IndOp(code string) (IndOpEntry, bool) {
	e, ok := indOpTable[code]
	return e, ok
}

// IsValidIndOp informa se o código indOp existe.
func IsValidIndOp(code string) bool {
	_, ok := indOpTable[code]
	return ok
}
```

- [ ] **Step 5: Gerar as tabelas**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
python3 -c "import openpyxl" || pip install --user openpyxl
python3 go-dfe/nfse/tables/gen/generate.py
```

Esperado: cinco linhas de saída, com `trib_nacional.go` acima de 500 entradas, `nbs.go` acima de 1000 e `indop.go` com aproximadamente 34.

- [ ] **Step 6: Rodar os testes e confirmar que passam**

```bash
cd go-dfe && go test ./nfse/tables/... -v
```

Esperado: PASS em todos os testes.

- [ ] **Step 7: Confirmar que o front compila com os dados novos**

```bash
cd ui && npx tsc --noEmit
```

Esperado: sem erros. Os dois arquivos gerados são só dados; ninguém os importa ainda.

- [ ] **Step 8: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add go-dfe/nfse/tables ui/src/lib/data/nfse_trib_nacional.ts ui/src/lib/data/nfse_indop.ts
git commit -m "feat(nfse): tabelas de referencia dos anexos B e C"
```

---

### Task 2: Tabelas DynamoDB no CDK

**Files:**
- Modify: `cdk/lib/dynamodb-stack.ts:8-35` (tipo `TableName`), `cdk/lib/dynamodb-stack.ts:479-494` (bloco de tabelas)
- Modify: `cdk/lib/worker-stack.ts:403`
- Test: `cdk/test/dynamodb-stack.test.ts`

**Interfaces:**
- Consumes: nada.
- Produces: as tabelas `${prefix}_organization_services`, `${prefix}_organization_nfse_configs`, `${prefix}_nfses`, `${prefix}_nfse_events`, registradas no mapa `this.tables` sob as chaves `'services'`, `'nfse_configs'`, `'nfses'`, `'nfse_events'`.

**Nota de escopo:** `nfses` e `nfse_events` só passam a ser lidas na F3, mas entram aqui para que a infraestrutura da NFS-e suba num único ciclo de synth/deploy em vez de dois. Tabela on-demand vazia não gera custo relevante.

- [ ] **Step 1: Escrever o teste que falha**

Crie `cdk/test/dynamodb-stack.test.ts`:

```typescript
import * as cdk from 'aws-cdk-lib';
import {Template} from 'aws-cdk-lib/assertions';
import {DynamoDBStack} from '../lib/dynamodb-stack';

const synth = () => {
    const app = new cdk.App();
    const stack = new DynamoDBStack(app, 'TestDynamoDBStack', {
        tablePrefix: 'dev_dfe',
        environment: 'dev',
    });
    return Template.fromStack(stack);
};

describe('DynamoDBStack — tabelas NFS-e', () => {
    test('cria as quatro tabelas novas', () => {
        const template = synth();
        for (const name of [
            'dev_dfe_organization_services',
            'dev_dfe_organization_nfse_configs',
            'dev_dfe_nfses',
            'dev_dfe_nfse_events',
        ]) {
            template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {TableName: name});
        }
    });

    test('organization_services tem os GSIs code-index e description-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_organization_services',
            GlobalSecondaryIndexes: expect.arrayContaining([
                expect.objectContaining({IndexName: 'code-index'}),
                expect.objectContaining({IndexName: 'description-index'}),
            ]),
        });
    });

    test('nfses tem o GSI access-key-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_nfses',
            GlobalSecondaryIndexes: expect.arrayContaining([
                expect.objectContaining({IndexName: 'access-key-index'}),
            ]),
        });
    });

    test('nenhuma tabela NFS-e declara stream — o outbox usa worker_outbox', () => {
        const template = synth();
        const tables = template.findResources('AWS::DynamoDB::GlobalTable');
        for (const res of Object.values(tables) as any[]) {
            const name: string = res.Properties.TableName;
            if (name.includes('nfse')) {
                expect(res.Properties.StreamSpecification).toBeUndefined();
            }
        }
    });
});
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd cdk && npx jest test/dynamodb-stack.test.ts
```

Esperado: FAIL — as tabelas não existem.

- [ ] **Step 3: Estender o tipo `TableName`**

Em `cdk/lib/dynamodb-stack.ts`, no tipo que começa na linha 8, acrescente as quatro entradas mantendo o agrupamento existente:

```typescript
    'mdfe_configs' |
    'nfse_configs' |
    'services' |
    'nfes' |
    'nfces' |
    'ctes' |
    'mdfes' |
    'nfses' |
    'nfe_events' |
    'nfce_events' |
    'cte_events' |
    'mdfe_events' |
    'nfse_events' |
```

- [ ] **Step 4: Criar as quatro tabelas**

Em `cdk/lib/dynamodb-stack.ts`, logo após o bloco de `personsTable` (que termina em `this.tables.set('persons', personsTable);`), insira a tabela do catálogo. Ela repete o formato de `productsTable` porque tem exatamente a mesma forma de acesso:

```typescript
        const servicesTable = new dynamodb.TableV2(this, `${tablePrefix}_organization_services`, {
            tableName: `${tablePrefix}_organization_services`,
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
            billing: Billing.onDemand({
                maxReadRequestUnits: 1000,
                maxWriteRequestUnits: 1000,
            }),
            removalPolicy,
            pointInTimeRecoverySpecification,
            encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
        });
        servicesTable.addGlobalSecondaryIndex({
            indexName: 'description-index',
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'description', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 1000,
            maxWriteRequestUnits: 1000,
        });
        servicesTable.addGlobalSecondaryIndex({
            indexName: 'code-index',
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'code', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 1000,
            maxWriteRequestUnits: 1000,
        });
        this.tables.set('services', servicesTable);
```

No bloco `CONFIGURATION TABLES`, após `mdfeConfigTable`:

```typescript
        const nfseConfigTable = getDfeConfigTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfse_configs');
        this.tables.set('nfse_configs', nfseConfigTable);
```

No bloco `DOCUMENT & EVENT TABLES`, após `mdfeEventsTable`:

```typescript
        const nfseEventsTable = getEventsTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfse_events');
        this.tables.set('nfse_events', nfseEventsTable);
```

E após `mdfesTable`, a tabela de documentos com o GSI extra da chave de acesso:

```typescript
        // nfses reutiliza getDfeTable (number-index-v2 + dfe-index) e acrescenta
        // access-key-index: a SK é o id_dps, porque a chave de acesso de 50
        // dígitos só existe depois da resposta do fisco — ver
        // docs/specs/2026-08-04-nfse-design.md §3.4.
        const nfsesTable = getDfeTable(this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'nfses');
        nfsesTable.addGlobalSecondaryIndex({
            indexName: 'access-key-index',
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'access_key', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 1000,
            maxWriteRequestUnits: 1000,
        });
        this.tables.set('nfses', nfsesTable);
```

- [ ] **Step 5: Incluir a config nova na varredura do dispatcher**

Em `cdk/lib/worker-stack.ts:403`, acrescente a tabela à lista:

```typescript
    const configTableArns = ['organization_nfe_configs', 'organization_cte_configs', 'organization_mdfe_configs', 'organization_nfse_configs'].flatMap(t => [
```

- [ ] **Step 6: Rodar os testes e o synth**

```bash
cd cdk && npx jest test/dynamodb-stack.test.ts && npm test && npx cdk synth --quiet
```

Esperado: todos os testes PASS e o synth conclui sem erro.

- [ ] **Step 7: Documentar as tabelas**

Em `DynamoDB-Tables.md`, acrescente as quatro tabelas seguindo o formato das existentes. Registre para cada uma: PK, SK, GSIs, atributos e padrão de acesso. Em `organization_services`, PK `{org_pk}` e SK `SERVICE_{uuid}`, GSIs `code-index` e `description-index`. Em `organization_nfse_configs`, PK `{org_pk}` sem SK. Em `nfses`, PK `{env}#{CNPJ}` e SK `id_dps` — inclua a nota de que a SK é o `idDPS` e não a chave de acesso, com o motivo (a chave só existe depois da resposta do fisco), e o GSI `access-key-index`. Em `nfse_events`, PK `{id_dps}` e SK `{uuidv7}`.

Em `OVERVIEW.md`, atualize a contagem de tabelas do DynamoDB (de 21 para 25) nas duas ocorrências (tabela de stack e seção `cdk`) e acrescente as quatro linhas à tabela `Data Model (DynamoDB Keys)`.

- [ ] **Step 8: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add cdk/lib/dynamodb-stack.ts cdk/lib/worker-stack.ts cdk/test/dynamodb-stack.test.ts DynamoDB-Tables.md OVERVIEW.md
git commit -m "feat(cdk): tabelas dynamodb da nfse"
```

---

### Task 3: Validadores de campos NFS-e

**Files:**
- Modify: `api/internal/validation/validators.go:22-45` (mapa `regexValidators`)
- Modify: `api/internal/validation/validation.go:145-160` (mensagens) e `:50-55` (registro)
- Modify: `api/go.mod`
- Test: `api/internal/validation/validation_test.go`

**Interfaces:**
- Consumes: `gopkg.aoctech.app/dfe/go-dfe/nfse/tables` da Task 1 — `IsValidTribNacional`, `IsValidNBS`, `IsValidIndOp`.
- Produces: as tags de validação `inscmun`, `caepf`, `nif`, `tribnac`, `nbs`, `indop`, `cnae`, usáveis em qualquer DTO. Valores monetários com quatro casas usam a tag `money` já existente (`^\d+(\.\d{1,4})?$`) — não crie uma tag nova para o mesmo formato.

**Por que a `api` passa a depender de `go-dfe`:** a tabela de códigos de tributação nacional é a mesma que a `go-dfe` vai usar na F2 para montar e validar o XML da DPS. Duas cópias violariam a regra de DRY do projeto ("Two implementations that solve the same problem must be unified"). O `worker` já depende de `go-dfe` pelo mesmo mecanismo (`worker/go.mod:41,44`).

- [ ] **Step 1: Escrever o teste que falha**

Acrescente ao fim de `api/internal/validation/validation_test.go`:

```go
func TestNfseValidators(t *testing.T) {
	type body struct {
		IM      string `json:"im" validate:"required,inscmun"`
		Caepf   string `json:"caepf" validate:"omitempty,caepf"`
		TribNac string `json:"trib_nac" validate:"required,tribnac"`
		NBS     string `json:"nbs" validate:"omitempty,nbs"`
		IndOp   string `json:"ind_op" validate:"omitempty,indop"`
	}

	valid := body{IM: "123456", Caepf: "12345678901234", TribNac: "10101", NBS: "101011100", IndOp: "020101"}
	if p := Struct(valid); p != nil {
		t.Fatalf("payload válido rejeitado: %+v", p)
	}

	cases := map[string]body{
		"im com letra":      {IM: "12A456", TribNac: "10101"},
		"caepf curto":       {IM: "123456", Caepf: "123", TribNac: "10101"},
		"tribnac inexistente": {IM: "123456", TribNac: "99999"},
		"tribnac curto":     {IM: "123456", TribNac: "101"},
		"nbs inexistente":   {IM: "123456", TribNac: "10101", NBS: "999999999"},
		"indop inexistente": {IM: "123456", TribNac: "10101", IndOp: "999999"},
	}
	for name, in := range cases {
		if p := Struct(in); p == nil {
			t.Errorf("%s: esperado erro de validação, veio nil", name)
		}
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd api && go test ./internal/validation/... -run TestNfseValidators
```

Esperado: FAIL — as tags não estão registradas, então `Struct` devolve `validation misconfigured`.

- [ ] **Step 3: Declarar a dependência de `go-dfe`**

```bash
cd api
go mod edit -require=gopkg.aoctech.app/dfe/go-dfe@v0.0.0
go mod edit -replace=gopkg.aoctech.app/dfe/go-dfe=../go-dfe
go mod tidy
```

- [ ] **Step 4: Acrescentar os validadores de formato**

Em `api/internal/validation/validators.go`, no mapa `regexValidators`, acrescente na seção `Identity / address`:

```go
	"inscmun": regexp.MustCompile(`^\d{1,15}$`),
	"caepf":   regexp.MustCompile(`^\d{14}$`),
	"nif":     regexp.MustCompile(`^[A-Za-z0-9]{1,40}$`),
	"cnae":    regexp.MustCompile(`^\d{7}$`),
```

Nada a acrescentar na seção `Generic numeric / units`: a tag `money` já existente aceita até quatro casas decimais, que é exatamente o formato dos valores da DPS.

Ainda em `validators.go`, ao fim do arquivo, os três validadores que consultam as tabelas oficiais:

```go
// Validadores de tabela NFS-e. Consultam as tabelas de referência geradas dos
// Anexos B e C, versionadas em go-dfe/nfse/tables — a mesma fonte que a go-dfe
// usa para montar o XML da DPS, para que api e go-dfe nunca divirjam.
func tribNacionalValidator(fl validator.FieldLevel) bool {
	return tables.IsValidTribNacional(fl.Field().String())
}

func nbsValidator(fl validator.FieldLevel) bool {
	return tables.IsValidNBS(fl.Field().String())
}

func indOpValidator(fl validator.FieldLevel) bool {
	return tables.IsValidIndOp(fl.Field().String())
}
```

Acrescente o import `"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"` no bloco de imports de `validators.go`.

- [ ] **Step 5: Registrar os validadores de tabela**

Em `api/internal/validation/validation.go`, logo após a linha `_ = v.RegisterValidation("cpfcnpj", cpfCnpjValidator)`:

```go
		// Validadores de tabela oficial NFS-e (Anexos B e C).
		_ = v.RegisterValidation("tribnac", tribNacionalValidator)
		_ = v.RegisterValidation("nbs", nbsValidator)
		_ = v.RegisterValidation("indop", indOpValidator)
```

E no `switch` de mensagens (o que contém `case "ibge":`), acrescente:

```go
	case "inscmun":
		return "inscrição municipal deve ter de 1 a 15 dígitos"
	case "caepf":
		return "CAEPF deve ter 14 dígitos"
	case "nif":
		return "NIF inválido (até 40 caracteres alfanuméricos)"
	case "cnae":
		return "CNAE deve ter 7 dígitos"
	case "tribnac":
		return "código de tributação nacional inexistente na lista de serviços"
	case "nbs":
		return "código NBS inexistente (informe sem pontos)"
	case "indop":
		return "código indOp inexistente na tabela do Anexo C"
```

- [ ] **Step 6: Rodar os testes e confirmar que passam**

```bash
cd api && go test ./internal/validation/... -race
```

Esperado: PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add api/go.mod api/go.sum api/internal/validation/
git commit -m "feat(api): validadores de campos nfse com tabelas oficiais"
```

---

### Task 4: Grupo `nfse` no objeto `person` compartilhado

Cobre `organizations` e `organization_persons` de uma vez.

**Files:**
- Modify: `api/internal/api/v1/dto.go:16-47`
- Test: `api/internal/api/v1/dto_test.go`

**Interfaces:**
- Consumes: as tags de validação da Task 3.
- Produces: os tipos `NfseRegTribBody`, `NfseForeignAddressBody` e `NfseInfoBody`, além do campo `Nfse *NfseInfoBody` em `PersonObjectBody`, que aparece automaticamente em `OrganizationCreateBody`, `OrganizationUpdateBody`, `PersonCreateBody` e `PersonUpdateBody`.

**Contexto:** os valores permitidos vêm do XSD `tiposSimples_v1.01.xsd`. `opSimpNac`: 1 não optante, 2 optante MEI, 3 optante ME/EPP. `regApTribSN` (só quando `opSimpNac` = 3): 1 federais e municipal pelo SN, 2 federais pelo SN e ISSQN por fora, 3 federais e municipal por fora. `regEspTrib`: 0 nenhum, 1 ato cooperado, 2 estimativa, 3 microempresa municipal, 4 notário/registrador, 5 profissional autônomo, 6 sociedade de profissionais. `cNaoNIF`: 0 não informado, 1 dispensado, 2 não exigência.

- [ ] **Step 1: Escrever o teste que falha**

Acrescente a `api/internal/api/v1/dto_test.go`:

```go
func TestPersonObjectBody_NfseGroup(t *testing.T) {
	base := func() PersonObjectBody {
		return PersonObjectBody{
			Addresses: []AddressBody{{
				CityIBGECode: "2211001", Street: "Rua A", Neighborhood: "Centro",
				Number: "10", City: "Teresina", StateFederation: "PI", PostalCode: "64000000",
			}},
		}
	}

	t.Run("grupo ausente é válido", func(t *testing.T) {
		if p := validation.Struct(base()); p != nil {
			t.Fatalf("person sem grupo nfse rejeitado: %+v", p)
		}
	})

	t.Run("grupo completo é válido", func(t *testing.T) {
		b := base()
		im := "987654"
		b.Nfse = &NfseInfoBody{
			IM:      &im,
			RegTrib: &NfseRegTribBody{OpSimpNac: 3, RegApTribSN: ptrInt(1), RegEspTrib: 0},
		}
		if p := validation.Struct(b); p != nil {
			t.Fatalf("grupo nfse válido rejeitado: %+v", p)
		}
	})

	t.Run("op_simp_nac fora do domínio é rejeitado", func(t *testing.T) {
		b := base()
		b.Nfse = &NfseInfoBody{RegTrib: &NfseRegTribBody{OpSimpNac: 9, RegEspTrib: 0}}
		if p := validation.Struct(b); p == nil {
			t.Fatal("op_simp_nac=9 aceito, esperado erro")
		}
	})

	t.Run("im não numérica é rejeitada", func(t *testing.T) {
		b := base()
		im := "ABC123"
		b.Nfse = &NfseInfoBody{IM: &im}
		if p := validation.Struct(b); p == nil {
			t.Fatal("im alfanumérica aceita, esperado erro")
		}
	})
}

func ptrInt(v int) *int { return &v }
```

Se `dto_test.go` ainda não importar `validation`, acrescente `"gopkg.aoctech.app/dfe/api/internal/validation"` aos imports.

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd api && go test ./internal/api/v1/... -run TestPersonObjectBody_NfseGroup
```

Esperado: FAIL de compilação — `NfseInfoBody` não existe.

- [ ] **Step 3: Declarar os tipos e estender `PersonObjectBody`**

Em `api/internal/api/v1/dto.go`, imediatamente antes de `PersonObjectBody`:

```go
// NfseRegTribBody é o regime tributário do prestador (TCRegTrib do DPS 1.01).
// Vive junto da identidade, e não na config da organização, porque quando a org
// emite como tomador ou intermediário (tpEmit 2 ou 3) o prestador é uma pessoa
// do cadastro e precisa do próprio regime — ver
// docs/specs/2026-08-04-nfse-design.md §3.2.
type NfseRegTribBody struct {
	// 1 não optante | 2 optante MEI | 3 optante ME/EPP
	OpSimpNac int `json:"op_simp_nac" validate:"required,oneof=1 2 3"`
	// Exigido apenas quando op_simp_nac = 3.
	// 1 federais e municipal pelo SN | 2 federais pelo SN, ISSQN por fora | 3 ambos por fora
	RegApTribSN *int `json:"reg_ap_trib_sn" validate:"omitempty,oneof=1 2 3"`
	// 0 nenhum | 1 ato cooperado | 2 estimativa | 3 microempresa municipal
	// 4 notário/registrador | 5 profissional autônomo | 6 sociedade de profissionais
	RegEspTrib int `json:"reg_esp_trib" validate:"oneof=0 1 2 3 4 5 6"`
}

// NfseForeignAddressBody é o endereço no exterior (TCEnderExt do DPS 1.01),
// usado quando a pessoa não tem endereço nacional.
type NfseForeignAddressBody struct {
	CPais        string  `json:"c_pais" validate:"required,len=2,alpha"`
	CEndPost     string  `json:"c_end_post" validate:"required,max=11"`
	XCidade      string  `json:"x_cidade" validate:"required,max=60"`
	XEstadoProv  string  `json:"x_estado_prov" validate:"required,max=60"`
	XLgr         string  `json:"x_lgr" validate:"required,max=255"`
	Nro          string  `json:"nro" validate:"required,max=60"`
	XCpl         *string `json:"x_cpl" validate:"omitempty,max=156"`
	XBairro      string  `json:"x_bairro" validate:"required,max=60"`
}

// NfseInfoBody é o grupo de campos exigidos pela NFS-e que não existem no
// cadastro de NF-e. Fica em PersonObjectBody porque TCInfoPrestador e
// TCInfoPessoa têm os mesmos campos de identidade — assim organizations e
// organization_persons são estendidas por uma única mudança.
type NfseInfoBody struct {
	IM      *string `json:"im" validate:"omitempty,inscmun"`
	Caepf   *string `json:"caepf" validate:"omitempty,caepf"`
	NIF     *string `json:"nif" validate:"omitempty,nif"`
	// 0 não informado | 1 dispensado | 2 não exigência
	CNaoNIF *int    `json:"c_nao_nif" validate:"omitempty,oneof=0 1 2"`
	// Obrigatório apenas quando a pessoa for usada como prestador numa emissão;
	// a validação dessa obrigatoriedade ocorre na emissão, não no cadastro.
	RegTrib        *NfseRegTribBody        `json:"reg_trib" validate:"omitempty"`
	ForeignAddress *NfseForeignAddressBody `json:"foreign_address" validate:"omitempty"`
}
```

E acrescente o campo em `PersonObjectBody`, ao fim da struct:

```go
	Nfse               *NfseInfoBody           `json:"nfse" validate:"omitempty"`
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd api && go test ./internal/api/v1/... -race
```

Esperado: PASS, inclusive os testes já existentes de organizações e pessoas — o campo é opcional e nenhum payload atual muda.

- [ ] **Step 5: Documentar**

Em `DynamoDB-Tables.md`, nas seções de `organizations` e `organization_persons`, acrescente o atributo `person.nfse` com os subcampos e a observação de que o grupo é opcional e compartilhado entre as duas tabelas.

Em `DOCS.md`, na descrição dos payloads de `POST /v1.0/organizations`, `PUT /v1.0/organizations/{pk}`, `POST /v1.0/persons` e `PUT /v1.0/persons/{cpf_cnpj}`, acrescente o objeto `nfse` com o domínio de cada código.

- [ ] **Step 6: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add api/internal/api/v1/dto.go api/internal/api/v1/dto_test.go DynamoDB-Tables.md DOCS.md
git commit -m "feat(api): grupo nfse no objeto person compartilhado"
```

---

### Task 5: Catálogo de serviços — repositório e serviço

**Files:**
- Create: `api/internal/repositories/services.go`
- Create: `api/internal/services/services.go`
- Modify: `api/internal/repositories/audit_logs.go:22-35`
- Test: `api/tests/integration/services_test.go`
- Modify: `api/tests/integration/setup_test.go`

**Interfaces:**
- Consumes: `CRUDRepository[map[string]types.AttributeValue]`, `NewCRUDRepository`, `GenerateID`, `QueryResult`, `QueryOpts` (de `repositories/base.go`); `CRUDMutationHelper`, `BuildItemCacheKey`, `GetCachedItem`, `GetCachedList` (de `services/crud.go`).
- Produces:
  - `repositories.NewServiceRepository(db *dynamodb.Client, cfg *config.Config) *ServiceRepository`
  - `repositories.ServiceListOpts{DescriptionPrefix, CodePrefix, OrderBy, Sort string; Limit int; StartKey map[string]types.AttributeValue}`
  - `services.NewServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ServiceService` com `Get`, `List`, `Create`, `Update`, `Delete` de assinaturas idênticas às de `ProductService`.
  - `repositories.AuditResourceService = "SERVICE"`

- [ ] **Step 1: Escrever o teste de integração que falha**

Crie `api/tests/integration/services_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func serviceFields(code, desc string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"code":               &types.AttributeValueMemberS{Value: code},
		"description":        &types.AttributeValueMemberS{Value: desc},
		"trib_nacional_code": &types.AttributeValueMemberS{Value: "10101"},
		"value":              &types.AttributeValueMemberS{Value: "1500.00"},
		"unit":               &types.AttributeValueMemberS{Value: "UN"},
	}
}

func TestService_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := serviceSvc.Create(ctx, orgPK, serviceFields("SRV001", "Desenvolvimento de sistemas"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := item["sk"].(*types.AttributeValueMemberS)
	if skAV == nil {
		t.Fatal("serviço criado sem sk")
	}
	if got := skAV.Value; len(got) < 9 || got[:8] != "SERVICE_" {
		t.Errorf("sk = %q, esperado prefixo SERVICE_", got)
	}

	got, err := serviceSvc.Get(ctx, orgPK, skAV.Value)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	descAV, _ := got["description"].(*types.AttributeValueMemberS)
	if descAV == nil || descAV.Value != "Desenvolvimento de sistemas" {
		t.Errorf("description = %v", got["description"])
	}
}

func TestService_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := serviceSvc.Get(ctx, orgPK, "SERVICE_inexistente"); problemStatus(err) != 404 {
		t.Errorf("status = %d, esperado 404", problemStatus(err))
	}
}

func TestService_ListByCodePrefix(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	for _, code := range []string{"CONS001", "CONS002", "DEV001"} {
		if _, err := serviceSvc.Create(ctx, orgPK, serviceFields(code, "Serviço "+code), "test-user", "Test User"); err != nil {
			t.Fatalf("Create(%s): %v", code, err)
		}
	}

	res, err := serviceSvc.List(ctx, orgPK, repositories.ServiceListOpts{CodePrefix: "CONS", Limit: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 2 {
		t.Errorf("len(Items) = %d, esperado 2", len(res.Items))
	}
}

func TestService_UpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	item, err := serviceSvc.Create(ctx, orgPK, serviceFields("SRV009", "Antes"), "test-user", "Test User")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value

	if _, err := serviceSvc.Update(ctx, orgPK, sk, map[string]any{"description": "Depois"}, "test-user", "Test User"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := serviceSvc.Get(ctx, orgPK, sk)
	if err != nil {
		t.Fatalf("Get após update: %v", err)
	}
	if got["description"].(*types.AttributeValueMemberS).Value != "Depois" {
		t.Error("update não persistiu")
	}

	if err := serviceSvc.Delete(ctx, orgPK, sk, "test-user", "Test User"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := serviceSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("após delete, status = %d, esperado 404", problemStatus(err))
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd api && go test -tags integration ./tests/integration/ -run TestService_
```

Esperado: FAIL de compilação — `serviceSvc` e `repositories.ServiceListOpts` não existem.

- [ ] **Step 3: Escrever o repositório**

Crie `api/internal/repositories/services.go`. É o mesmo formato de `products.go` porque o padrão de acesso é idêntico:

```go
package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ServiceRepository — organization_services, o catálogo de serviços que a
// emissão de NFS-e consome (análogo a organization_products para NF-e).
type ServiceRepository struct {
	CRUDRepository[map[string]types.AttributeValue]
}

func NewServiceRepository(db *dynamodb.Client, cfg *config.Config) *ServiceRepository {
	return &ServiceRepository{
		CRUDRepository: NewCRUDRepository[map[string]types.AttributeValue](db, cfg, "organization_services"),
	}
}

func buildServiceSK(sk string) string {
	if strings.HasPrefix(sk, "SERVICE_") {
		return sk
	}
	return fmt.Sprintf("SERVICE_%s", sk)
}

func (r *ServiceRepository) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	return r.CRUDRepository.Create(ctx, orgPK, buildServiceSK(id), fields)
}

func (r *ServiceRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.CRUDRepository.Get(ctx, orgPK, buildServiceSK(sk))
}

type ServiceListOpts struct {
	DescriptionPrefix string
	CodePrefix        string
	OrderBy           string
	Sort              string
	Limit             int
	StartKey          map[string]types.AttributeValue
}

func (r *ServiceRepository) List(ctx context.Context, orgPK string, opts ServiceListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.DescriptionPrefix != "" || opts.OrderBy == "description" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.DescriptionPrefix,
			IndexName: "description-index", SKField: "description",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	if opts.CodePrefix != "" || opts.OrderBy == "code" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.CodePrefix,
			IndexName: "code-index", SKField: "code",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: "SERVICE_",
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *ServiceRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	return r.CRUDRepository.Update(ctx, orgPK, buildServiceSK(sk), updates)
}

func (r *ServiceRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.CRUDRepository.Delete(ctx, orgPK, buildServiceSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new service, mirroring
// Create's key/timestamp construction, without writing.
func (r *ServiceRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	id := GenerateID()
	tx, item, _ := r.CRUDRepository.BuildCreateTxItem(orgPK, buildServiceSK(id), fields)
	return tx, item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// service, mirroring Update's timestamp bump, without writing.
func (r *ServiceRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	return r.CRUDRepository.BuildUpdateTxItem(orgPK, buildServiceSK(sk), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a service, without writing.
func (r *ServiceRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.CRUDRepository.BuildDeleteTxItem(orgPK, buildServiceSK(sk))
}
```

- [ ] **Step 4: Acrescentar a constante de auditoria**

Em `api/internal/repositories/audit_logs.go`, no bloco de constantes de recurso, após `AuditResourceProduct`:

```go
	AuditResourceService      = "SERVICE"
```

- [ ] **Step 5: Escrever o serviço**

Crie `api/internal/services/services.go`:

```go
package services

import (
	"context"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/repositories"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ServiceService is the catálogo de serviços consumido pela emissão de NFS-e.
type ServiceService struct {
	repo      *repositories.ServiceRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
	crud      *CRUDMutationHelper
}

func NewServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ServiceService {
	return &ServiceService{
		repo:      repo,
		auditRepo: auditRepo,
		cache:     c,
		crud:      NewCRUDMutationHelper(auditRepo, c),
	}
}

func (s *ServiceService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := BuildItemCacheKey(orgPK, "services", sk)
	return GetCachedItem(ctx, s.cache, key, func(ctx context.Context) (map[string]types.AttributeValue, error) {
		return s.repo.Get(ctx, orgPK, sk)
	}, "service not found")
}

func (s *ServiceService) List(ctx context.Context, orgPK string, opts repositories.ServiceListOpts) (*repositories.QueryResult, error) {
	return GetCachedList(ctx, s.cache, orgPK, "services", opts, func(ctx context.Context) (*repositories.QueryResult, error) {
		return s.repo.List(ctx, orgPK, opts)
	})
}

// Create writes the service and its CREATE audit row atomically.
func (s *ServiceService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Create(ctx, orgPK, repositories.AuditResourceService, userID, userName, func() (types.TransactWriteItem, map[string]types.AttributeValue, error) {
		tx, item := s.repo.BuildCreateTxItem(orgPK, fields)
		return tx, item, nil
	}, s.repo.TransactWrite)
}

// Update writes the service change and its UPDATE audit row atomically.
func (s *ServiceService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	return s.crud.Update(ctx, orgPK, sk, repositories.AuditResourceService, updates, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	}, s.repo.TransactWrite)
}

// Delete removes the service and writes its DELETE audit row atomically.
func (s *ServiceService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	return s.crud.Delete(ctx, orgPK, sk, repositories.AuditResourceService, userID, userName, s.repo.Get, func(ctx context.Context) (types.TransactWriteItem, error) {
		return s.repo.BuildDeleteTxItem(orgPK, sk), nil
	}, s.repo.TransactWrite)
}
```

- [ ] **Step 6: Ligar o serviço no bootstrap de integração**

Em `api/tests/integration/setup_test.go`, declare a variável junto das outras (bloco que começa em `orgSvc`):

```go
	serviceSvc     *services.ServiceService
```

E instancie junto das outras, ao lado de `productSvc`:

```go
	serviceRepo := repositories.NewServiceRepository(dbClient, cfg)
	serviceSvc = services.NewServiceService(serviceRepo, auditRepo, memCache)
```

Use o mesmo nome de variável do cliente DynamoDB e do `cfg` que o arquivo já usa nas linhas vizinhas — não introduza novos.

Se o arquivo criar as tabelas de teste explicitamente, acrescente `organization_services` à lista, seguindo o formato das existentes (PK `pk`, SK `sk`, GSIs `code-index` e `description-index`).

- [ ] **Step 7: Rodar os testes e confirmar que passam**

```bash
cd api && go test -tags integration ./tests/integration/ -run TestService_ -v
```

Esperado: PASS nos quatro testes.

- [ ] **Step 8: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add api/internal/repositories/services.go api/internal/repositories/audit_logs.go api/internal/services/services.go api/tests/integration/services_test.go api/tests/integration/setup_test.go
git commit -m "feat(api): repositorio e servico do catalogo de servicos"
```

---

### Task 6: Config NFS-e por organização

Inclui a unificação dos `Upsert` de config, que hoje são quatro cópias idênticas de 40 linhas. Acrescentar uma quinta cópia violaria a regra de DRY do projeto, então a base genérica entra junto.

**Files:**
- Modify: `api/internal/repositories/fiscal_configs.go`
- Modify: `api/internal/services/fiscal_configs.go`
- Test: `api/tests/integration/nfse_configs_test.go`
- Modify: `api/tests/integration/setup_test.go`
- Modify: `api/internal/repositories/audit_logs.go`

**Interfaces:**
- Consumes: `FiscalConfigRepository`, `newFiscalConfigBase`, `BuildUpsertTxItem`, `IncrementNumber` (de `repositories/fiscal_config.go`); `attributeMapToPlain`, `Diff`, `AuditLogRepository.BuildLogTxItem`.
- Produces:
  - `repositories.NewNfseConfigRepository(db, cfg) *NfseConfigRepository`
  - `services.NewNfseConfigService(repo *repositories.NfseConfigRepository, auditRepo *repositories.AuditLogRepository) *NfseConfigService` com `Get(ctx, orgPK)` e `Upsert(ctx, orgPK, fields, userID, userName)` — a mesma assinatura da interface `fiscalConfigSvc` já usada por `registerFiscalConfig`.
  - `repositories.AuditResourceNfseConfig = "NFSE_CONFIG"`

- [ ] **Step 1: Escrever o teste de integração que falha**

Crie `api/tests/integration/nfse_configs_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func nfseConfigFields() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"provider":            &types.AttributeValueMemberS{Value: "nacional"},
		"environment":         &types.AttributeValueMemberN{Value: "2"},
		"c_loc_emi":           &types.AttributeValueMemberS{Value: "2211001"},
		"serie":               &types.AttributeValueMemberS{Value: "00001"},
		"prod_current_number": &types.AttributeValueMemberN{Value: "0"},
		"hom_current_number":  &types.AttributeValueMemberN{Value: "0"},
	}
}

func TestNfseConfig_UpsertAndGet(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := nfseConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got["provider"].(*types.AttributeValueMemberS).Value != "nacional" {
		t.Errorf("provider = %v", got["provider"])
	}
	if got["c_loc_emi"].(*types.AttributeValueMemberS).Value != "2211001" {
		t.Errorf("c_loc_emi = %v", got["c_loc_emi"])
	}
}

func TestNfseConfig_GetNotFound(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Get(ctx, orgPK); problemStatus(err) != 404 {
		t.Errorf("status = %d, esperado 404", problemStatus(err))
	}
}

// O contador de numeração é o mesmo mecanismo dos demais documentos fiscais:
// {envPrefix}_current_number, incrementado atomicamente.
func TestNfseConfig_IncrementNumber(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	first, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom")
	if err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	second, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom")
	if err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}
	if second != first+1 {
		t.Errorf("segundo incremento = %d, esperado %d", second, first+1)
	}
}

// Upsert não pode zerar o contador — é campo de processo interno.
func TestNfseConfig_UpsertPreservesCounter(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, nfseConfigFields(), "test-user", "Test User"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := nfseConfigRepo.IncrementNumber(ctx, orgPK, "hom"); err != nil {
		t.Fatalf("IncrementNumber: %v", err)
	}

	fields := nfseConfigFields()
	delete(fields, "hom_current_number")
	if _, err := nfseConfigSvc.Upsert(ctx, orgPK, fields, "test-user", "Test User"); err != nil {
		t.Fatalf("segundo Upsert: %v", err)
	}

	got, err := nfseConfigSvc.Get(ctx, orgPK)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n := got["hom_current_number"].(*types.AttributeValueMemberN).Value; n != "1" {
		t.Errorf("hom_current_number = %s, esperado 1 — o upsert zerou o contador", n)
	}
}
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd api && go test -tags integration ./tests/integration/ -run TestNfseConfig_
```

Esperado: FAIL de compilação — `nfseConfigSvc` e `nfseConfigRepo` não existem.

- [ ] **Step 3: Escrever o repositório**

Em `api/internal/repositories/fiscal_configs.go`, ao fim do arquivo:

```go
// NfseConfigRepository — organization_nfse_configs.
// preserve: contadores de numeração da DPS/RPS, atualizados pela emissão.
type NfseConfigRepository struct {
	FiscalConfigRepository
}

func NewNfseConfigRepository(db *dynamodb.Client, cfg *config.Config) *NfseConfigRepository {
	return &NfseConfigRepository{
		FiscalConfigRepository: newFiscalConfigBase(db, cfg, "organization_nfse_configs", map[string]any{
			"prod_current_number": 0,
			"hom_current_number":  0,
		}),
	}
}
```

- [ ] **Step 4: Acrescentar a constante de auditoria**

Em `api/internal/repositories/audit_logs.go`, após `AuditResourceMdfeConfig`:

```go
	AuditResourceNfseConfig   = "NFSE_CONFIG"
```

- [ ] **Step 5: Extrair a base genérica do serviço de config**

Em `api/internal/services/fiscal_configs.go`, acrescente `nfseConfigResourceID = "nfse_config"` ao bloco `const` já existente no topo do arquivo (o que declara `nfeConfigResourceID`, `nfceConfigResourceID`, `cteConfigResourceID` e `mdfeConfigResourceID`) — não abra um `const` novo.

Depois, a base genérica. `Nfe`, `Nfce`, `Cte` e `Mdfe` já têm `Upsert` byte a byte idênticos — só variam no repositório, na constante de auditoria e no texto do 404:

```go
// fiscalConfigRepo é a parte de FiscalConfigRepository que o serviço usa.
// A assinatura de TransactWrite vem de dynamo.Base:
//   func (b *Base) TransactWrite(ctx context.Context, items []types.TransactWriteItem) error
// Os cinco repositórios de config a satisfazem por embedding.
type fiscalConfigRepo interface {
	Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error)
	BuildUpsertTxItem(orgPK string, fields map[string]types.AttributeValue, existing map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error)
	TransactWrite(ctx context.Context, items []types.TransactWriteItem) error
}

// fiscalConfigService implementa Get/Upsert para qualquer config fiscal
// singleton. Antes cada variante repetia o mesmo Upsert; a lógica de auditoria
// (diff contra os campos FINAIS, pós carry-forward dos campos preservados) é
// sutil o bastante para não valer cinco cópias — ver
// FiscalConfigRepository.BuildUpsertTxItem.
type fiscalConfigService struct {
	repo         fiscalConfigRepo
	auditRepo    *repositories.AuditLogRepository
	resourceType string
	resourceID   string
	notFoundMsg  string
}

func (s *fiscalConfigService) Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error) {
	item, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound(s.notFoundMsg)
	}
	return item, nil
}

// Upsert writes the config and its CREATE/UPDATE audit row atomically. The
// audit diff compares the pre-existing item against the FINAL merged fields
// (post preserve-field carry-forward), never against the caller's raw input.
// The single Get below feeds both the preserve-merge and the audit baseline:
// two independent reads could straddle a concurrent internal-process write
// (e.g. a counter increment) and misattribute it to the acting user.
func (s *fiscalConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	action := repositories.AuditActionUpdate
	var beforeMap map[string]any
	if current == nil {
		action = repositories.AuditActionCreate
	} else {
		beforeMap, err = attributeMapToPlain(current)
		if err != nil {
			return nil, err
		}
	}

	configTx, finalItem, err := s.repo.BuildUpsertTxItem(orgPK, fields, current)
	if err != nil {
		return nil, err
	}
	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}

	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, s.resourceType, s.resourceID, action,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{configTx, auditTx}); err != nil {
		return nil, err
	}
	return finalItem, nil
}
```

- [ ] **Step 6: Reduzir os cinco serviços à base**

Ainda em `api/internal/services/fiscal_configs.go`, substitua os quatro blocos `NfeConfigService`, `NfceConfigService`, `CteConfigService` e `MdfeConfigService` (structs, construtores, `Get` e `Upsert`) por:

```go
// NfeConfigService wraps NfeConfigRepository.
type NfeConfigService struct{ fiscalConfigService }

func NewNfeConfigService(repo *repositories.NfeConfigRepository, auditRepo *repositories.AuditLogRepository) *NfeConfigService {
	return &NfeConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfeConfig, resourceID: nfeConfigResourceID,
		notFoundMsg: "nfe config not found",
	}}
}

// NfceConfigService wraps NfceConfigRepository.
type NfceConfigService struct{ fiscalConfigService }

func NewNfceConfigService(repo *repositories.NfceConfigRepository, auditRepo *repositories.AuditLogRepository) *NfceConfigService {
	return &NfceConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfceConfig, resourceID: nfceConfigResourceID,
		notFoundMsg: "nfce config not found",
	}}
}

// CteConfigService wraps CteConfigRepository.
type CteConfigService struct{ fiscalConfigService }

func NewCteConfigService(repo *repositories.CteConfigRepository, auditRepo *repositories.AuditLogRepository) *CteConfigService {
	return &CteConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceCteConfig, resourceID: cteConfigResourceID,
		notFoundMsg: "cte config not found",
	}}
}

// MdfeConfigService wraps MdfeConfigRepository.
type MdfeConfigService struct{ fiscalConfigService }

func NewMdfeConfigService(repo *repositories.MdfeConfigRepository, auditRepo *repositories.AuditLogRepository) *MdfeConfigService {
	return &MdfeConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceMdfeConfig, resourceID: mdfeConfigResourceID,
		notFoundMsg: "mdfe config not found",
	}}
}

// NfseConfigService wraps NfseConfigRepository.
type NfseConfigService struct{ fiscalConfigService }

func NewNfseConfigService(repo *repositories.NfseConfigRepository, auditRepo *repositories.AuditLogRepository) *NfseConfigService {
	return &NfseConfigService{fiscalConfigService{
		repo: repo, auditRepo: auditRepo,
		resourceType: repositories.AuditResourceNfseConfig, resourceID: nfseConfigResourceID,
		notFoundMsg: "nfse config not found",
	}}
}
```

Ajuste o bloco de imports do arquivo se algum passar a não ser usado.

- [ ] **Step 7: Ligar no bootstrap de integração**

Em `api/tests/integration/setup_test.go`, declare junto das outras variáveis:

```go
	nfseConfigSvc  *services.NfseConfigService
	nfseConfigRepo *repositories.NfseConfigRepository
```

E instancie ao lado de `mdfeConfigSvc`:

```go
	nfseConfigRepo = repositories.NewNfseConfigRepository(dbClient, cfg)
	nfseConfigSvc = services.NewNfseConfigService(nfseConfigRepo, auditRepo)
```

Se o arquivo criar tabelas de teste explicitamente, acrescente `organization_nfse_configs` (PK `pk`, sem SK).

- [ ] **Step 8: Rodar toda a suíte, incluindo a de regressão das outras configs**

```bash
cd api && go test ./... -race && go test -tags integration ./tests/integration/ -v
```

Esperado: PASS. `fiscal_configs_audit_test.go` é a rede de segurança da extração do Step 6 — se o comportamento de auditoria de NF-e, NFC-e, CT-e ou MDF-e mudou, ele falha.

- [ ] **Step 9: Documentar a unificação**

Em `CONDUCT.md`, na seção de padrões do backend, registre que os serviços de config fiscal compartilham `fiscalConfigService` e que uma variante nova se declara pelo construtor, sem reescrever `Get`/`Upsert`. Registre também que o diff de auditoria compara sempre contra os campos finais pós carry-forward, e o motivo.

- [ ] **Step 10: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add api/internal/repositories/fiscal_configs.go api/internal/repositories/audit_logs.go api/internal/services/fiscal_configs.go api/tests/integration/nfse_configs_test.go api/tests/integration/setup_test.go CONDUCT.md
git commit -m "feat(api): config nfse por organizacao e unificacao dos services de config"
```

---

### Task 7: Rotas, RBAC e wiring

**Files:**
- Create: `api/internal/api/v1/services.go`
- Modify: `api/internal/api/v1/dto.go`
- Modify: `api/internal/api/v1/organizations.go:185-198`
- Modify: `api/internal/api/v1/router.go:17-38,62`
- Modify: `api/internal/app/app.go:52-86`
- Modify: `api/internal/repositories/roles.go:33-44`
- Modify: `api/internal/middleware/scopes.go:28-38`
- Test: `api/internal/api/v1/dto_test.go`

**Interfaces:**
- Consumes: `ServiceService` (Task 5), `NfseConfigService` (Task 6), `mountCRUD`, `crudHandlers`, `crudListOpts`, `bindAVValidated`, `bindJSON`, `structToMap`, `registerFiscalConfig`, `fiscalConfigSvc`.
- Produces: `RegisterServices(router fiber.Router, svc *services.ServiceService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker)`; os tipos `ServiceBody`, `ServiceIssBody`, `ServiceFederalBody`, `ServiceIbsCbsBody`, `NfseConfigBody`, `NfseAbrasfBody`; e o campo `Service *services.ServiceService` e `NfseConfig *services.NfseConfigService` em `v1.Services`.

- [ ] **Step 1: Escrever o teste que falha**

Acrescente a `api/internal/api/v1/dto_test.go`:

```go
func TestServiceBody_Validation(t *testing.T) {
	valid := ServiceBody{
		Code:             "SRV001",
		Description:      "Análise e desenvolvimento de sistemas",
		TribNacionalCode: "10101",
		Unit:             "UN",
		Value:            "1500.00",
		Iss:              ServiceIssBody{TribISSQN: 1, Aliquota: "5.00"},
	}
	if p := validation.Struct(valid); p != nil {
		t.Fatalf("ServiceBody válido rejeitado: %+v", p)
	}

	t.Run("trib_nacional_code inexistente é rejeitado", func(t *testing.T) {
		b := valid
		b.TribNacionalCode = "99999"
		if p := validation.Struct(b); p == nil {
			t.Fatal("código inexistente aceito")
		}
	})

	t.Run("descrição acima de 2000 chars é rejeitada", func(t *testing.T) {
		b := valid
		b.Description = strings.Repeat("x", 2001)
		if p := validation.Struct(b); p == nil {
			t.Fatal("descrição longa demais aceita")
		}
	})
}

func TestNfseConfigBody_Validation(t *testing.T) {
	valid := NfseConfigBody{
		Provider:    "nacional",
		Environment: 2,
		CLocEmi:     "2211001",
		Serie:       "00001",
	}
	if p := validation.Struct(valid); p != nil {
		t.Fatalf("NfseConfigBody válido rejeitado: %+v", p)
	}

	t.Run("provider desconhecido é rejeitado", func(t *testing.T) {
		b := valid
		b.Provider = "gissonline"
		if p := validation.Struct(b); p == nil {
			t.Fatal("provider desconhecido aceito")
		}
	})

	t.Run("c_loc_emi fora do formato IBGE é rejeitado", func(t *testing.T) {
		b := valid
		b.CLocEmi = "2211"
		if p := validation.Struct(b); p == nil {
			t.Fatal("código IBGE curto aceito")
		}
	})
}
```

Acrescente `"strings"` aos imports do arquivo de teste se ainda não estiver lá.

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd api && go test ./internal/api/v1/... -run 'TestServiceBody_Validation|TestNfseConfigBody_Validation'
```

Esperado: FAIL de compilação — os tipos não existem.

- [ ] **Step 3: Declarar os DTOs**

Em `api/internal/api/v1/dto.go`, após a seção de produtos, acrescente uma seção nova. Os domínios de código vêm de `tiposSimples_v1.01.xsd`: `tribISSQN` 1 operação tributável, 2 exportação, 3 não incidência, 4 imunidade; `tpRetISSQN` 1 não retido, 2 retido pelo tomador, 3 retido pelo intermediário; `tpImunidade` 0 a 9; `cstPisCofins` 00 a 99; `indTotTrib` 0 ou 1.

```go
// ── Serviços (catálogo NFS-e) ────────────────────────────────────────────────

// ServiceIssBody são os defaults de ISSQN do serviço (grupo tribMun do DPS).
type ServiceIssBody struct {
	// 1 operação tributável | 2 exportação | 3 não incidência | 4 imunidade
	TribISSQN   int     `json:"trib_issqn" validate:"required,oneof=1 2 3 4"`
	Aliquota    string  `json:"aliquota" validate:"required,percent"`
	// 1 não retido | 2 retido pelo tomador | 3 retido pelo intermediário
	TpRetISSQN  *int    `json:"tp_ret_issqn" validate:"omitempty,oneof=1 2 3"`
	TpImunidade *int    `json:"tp_imunidade" validate:"omitempty,gte=0,lte=9"`
	CPaisResultado *string `json:"c_pais_resultado" validate:"omitempty,len=2,alpha"`
}

// ServiceFederalBody são os defaults de tributos federais do serviço.
type ServiceFederalBody struct {
	CstPisCofins  *string `json:"cst_pis_cofins" validate:"omitempty,len=2,number"`
	AliqPis       *string `json:"aliq_pis" validate:"omitempty,percent"`
	AliqCofins    *string `json:"aliq_cofins" validate:"omitempty,percent"`
	TpRetPisCofins *int   `json:"tp_ret_pis_cofins" validate:"omitempty,oneof=1 2"`
	VRetCP        *string `json:"v_ret_cp" validate:"omitempty,money2"`
	VRetIRRF      *string `json:"v_ret_irrf" validate:"omitempty,money2"`
	VRetCSLL      *string `json:"v_ret_csll" validate:"omitempty,money2"`
}

// ServiceIbsCbsBody são os defaults de IBS/CBS do serviço (reforma tributária).
type ServiceIbsCbsBody struct {
	CIndOp      *string `json:"c_ind_op" validate:"omitempty,indop"`
	Cst         *string `json:"cst" validate:"omitempty,len=3,number"`
	CClassTrib  *string `json:"c_class_trib" validate:"omitempty,max=6,number"`
	IndDest     *int    `json:"ind_dest" validate:"omitempty,oneof=0 1 2"`
	TpOper      *int    `json:"tp_oper" validate:"omitempty,oneof=1 2 3 4"`
	FinNFSe     *int    `json:"fin_nfse" validate:"omitempty,oneof=1 2 3 4"`
}

// ServiceTotTribBody é a Lei da Transparência (grupo totTrib do DPS).
type ServiceTotTribBody struct {
	// 0 não informar | 1 informar valores | 2 informar percentuais
	IndTotTrib  int     `json:"ind_tot_trib" validate:"oneof=0 1 2"`
	PTotTribSN  *string `json:"p_tot_trib_sn" validate:"omitempty,percent"`
}

// ServiceBody is the body for POST /services and PUT /services/:service_id.
// O frontend envia o objeto completo em ambos.
type ServiceBody struct {
	Code             string  `json:"code" validate:"required,max=60"`
	Description      string  `json:"description" validate:"required,min=2,max=2000"`
	TribNacionalCode string  `json:"trib_nacional_code" validate:"required,tribnac"`
	TribMunicipalCode *string `json:"trib_municipal_code" validate:"omitempty,max=20"`
	NbsCode          *string `json:"nbs_code" validate:"omitempty,nbs"`
	Cnae             *string `json:"cnae" validate:"omitempty,cnae"`
	Unit             string  `json:"unit" validate:"required,unit"`
	Value            string  `json:"value" validate:"required,money"`
	Iss              ServiceIssBody      `json:"iss" validate:"required"`
	Federal          *ServiceFederalBody `json:"federal" validate:"omitempty"`
	IbsCbs           *ServiceIbsCbsBody  `json:"ibs_cbs" validate:"omitempty"`
	TotTrib          *ServiceTotTribBody `json:"tot_trib" validate:"omitempty"`
}

// ── Config NFS-e ─────────────────────────────────────────────────────────────

// NfseAbrasfBody é a configuração específica do provider abrasf204.
type NfseAbrasfBody struct {
	EndpointURL     string `json:"endpoint_url" validate:"required,url"`
	WsdlVersion     string `json:"wsdl_version" validate:"required,max=10"`
	CodigoMunicipio string `json:"codigo_municipio" validate:"required,ibge"`
	EnvioSincrono   bool   `json:"envio_sincrono"`
}

// NfseConfigBody is the body for PUT /organizations/:org_pk/nfse-config.
// Inscrição municipal e regime tributário do prestador NÃO ficam aqui — vêm do
// grupo `nfse` do objeto person da própria organização, porque quando ela emite
// como tomador ou intermediário o prestador é outra pessoa. Ver
// docs/specs/2026-08-04-nfse-design.md §3.2 e §3.3.
type NfseConfigBody struct {
	Provider          string  `json:"provider" validate:"required,oneof=nacional abrasf204"`
	Environment       int     `json:"environment" validate:"required,oneof=1 2"`
	CLocEmi           string  `json:"c_loc_emi" validate:"required,ibge"`
	Serie             string  `json:"serie" validate:"required,max=5,number"`
	ProdCurrentNumber int     `json:"prod_current_number" validate:"gte=0"`
	HomCurrentNumber  int     `json:"hom_current_number" validate:"gte=0"`
	CertificateSK     *string `json:"certificate_sk" validate:"omitempty,max=60"`
	Abrasf            *NfseAbrasfBody `json:"abrasf" validate:"omitempty"`
}
```

- [ ] **Step 4: Escrever as rotas do catálogo**

Crie `api/internal/api/v1/services.go`:

```go
package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// RegisterServices mounts /services routes under a tenant-scoped group.
func RegisterServices(router fiber.Router, svc *services.ServiceService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
	mountCRUD(router, "/services", authMw, perm, userSvc, crudHandlers{
		listPerm: "list.organization_services", createPerm: "create.organization_services",
		getPerm: "get.organization_services", updatePerm: "update.organization_services",
		deletePerm: "delete.organization_services",
		param:      "service_id",

		list: func(c fiber.Ctx, orgPK string, o crudListOpts) (*repositories.QueryResult, error) {
			return svc.List(c.Context(), orgPK, repositories.ServiceListOpts{
				DescriptionPrefix: c.Query("description"),
				CodePrefix:        c.Query("code"),
				OrderBy:           c.Query("order_by"),
				Sort:              o.Sort,
				Limit:             o.Limit,
				StartKey:          o.StartKey,
			})
		},
		create: func(c fiber.Ctx, orgPK, userID, userName string) (map[string]types.AttributeValue, error) {
			av, p := bindAVValidated[ServiceBody](c)
			if p != nil {
				return nil, p
			}
			return svc.Create(c.Context(), orgPK, av, userID, userName)
		},
		get: func(c fiber.Ctx, orgPK, id string) (map[string]types.AttributeValue, error) {
			return svc.Get(c.Context(), orgPK, id)
		},
		update: func(c fiber.Ctx, orgPK, id, userID, userName string) (map[string]types.AttributeValue, error) {
			var dto ServiceBody
			if p := bindJSON(c, &dto); p != nil {
				return nil, p
			}
			body, err := structToMap(dto)
			if err != nil {
				return nil, err
			}
			return svc.Update(c.Context(), orgPK, id, body, userID, userName)
		},
		del: func(c fiber.Ctx, orgPK, id, userID, userName string) error {
			return svc.Delete(c.Context(), orgPK, id, userID, userName)
		},
	})
}
```

- [ ] **Step 5: Montar a rota da config**

Em `api/internal/api/v1/organizations.go`, no bloco `── Fiscal configs ──`, após o `registerFiscalConfig` de mdfe:

```go
	registerFiscalConfig(scoped, "/nfse-config",
		"get.organization_nfse_configs", "update.organization_nfse_configs",
		h.NfseConfig, perm, bindAVValidated[NfseConfigBody], h.UserSvc)
```

E acrescente o campo à struct `OrgHandlers` (mesmo arquivo), ao lado de `MdfeConfig`:

```go
	NfseConfig *services.NfseConfigService
```

- [ ] **Step 6: Registrar RBAC e escopos**

Em `api/internal/repositories/roles.go`, na lista `resources`, acrescente as entradas na posição que mantém o agrupamento por família:

```go
	"nfses", "nfse_events",
	"organization_services",
	"organization_nfse_configs",
```

Em `api/internal/middleware/scopes.go`, no mapa `scopeFamilies`:

```go
	"nfses":                 {"nfses", "nfse_events", "organization_nfse_configs"},
	"organization_services": {"organization_services"},
```

- [ ] **Step 7: Ligar no fx e no router**

Em `api/internal/app/app.go`, na lista de providers, ao lado dos análogos:

```go
		repositories.NewServiceRepository,
		repositories.NewNfseConfigRepository,
```

e

```go
		newServiceService,
		services.NewNfseConfigService,
```

Acrescente o construtor junto de `newProductService`:

```go
func newServiceService(repo *repositories.ServiceRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *services.ServiceService {
	return services.NewServiceService(repo, auditRepo, c)
}
```

Em `api/internal/api/v1/router.go`, na struct `Services`:

```go
	Service      *services.ServiceService
	NfseConfig   *services.NfseConfigService
```

No `RegisterOrganizations`, acrescente `NfseConfig: svcs.NfseConfig,` ao literal de `OrgHandlers`. E após `RegisterProducts`:

```go
	RegisterServices(v1, svcs.Service, svcs.User, authMw, perm)
```

- [ ] **Step 8: Rodar tudo**

```bash
cd api && go build ./... && go test ./... -race && go test -tags integration ./tests/integration/
```

Esperado: build limpo e PASS em tudo. `seedRoles` roda no boot e passa a semear as permissões novas.

- [ ] **Step 9: Documentar os endpoints**

Em `DOCS.md`, acrescente os endpoints à referência da API, com payload de request e response:

```
GET    /v1.0/services                          Listar catálogo (paginado; filtros code, description, order_by, sort, limit, cursor)
POST   /v1.0/services                          Criar serviço
GET    /v1.0/services/{service_id}             Detalhe
PUT    /v1.0/services/{service_id}             Atualizar (objeto completo)
DELETE /v1.0/services/{service_id}             Remover

GET    /v1.0/organizations/{pk}/nfse-config    Config NFS-e da organização
PUT    /v1.0/organizations/{pk}/nfse-config    Upsert da config
```

Registre também as permissões RBAC novas (`{list,get,create,update,delete}.organization_services` e `.organization_nfse_configs`) e as famílias de escopo `dfe:nfses:*` e `dfe:organization_services:*`.

Em `OVERVIEW.md`, acrescente `/v1.0/services` à lista de endpoints principais.

- [ ] **Step 10: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add api/internal/api/v1/ api/internal/app/app.go api/internal/repositories/roles.go api/internal/middleware/scopes.go DOCS.md OVERVIEW.md
git commit -m "feat(api): rotas do catalogo de servicos e da config nfse"
```

---

### Task 8: Fechamento da fase

**Files:**
- Modify: `CONDUCT.md`
- Modify: `docs/specs/2026-08-04-nfse-design.md`

- [ ] **Step 1: Rodar a suíte inteira do repositório**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
(cd go-dfe && go test ./... -race)
(cd api && go test ./... -race && go test -tags integration ./tests/integration/)
(cd worker && go build ./...)
(cd cdk && npm test && npx cdk synth --quiet)
(cd ui && npx tsc --noEmit && npx eslint src --ext .ts,.tsx)
```

Esperado: tudo PASS. O eslint precisa terminar com zero erros e zero avisos.

- [ ] **Step 2: Registrar as decisões duráveis**

Em `CONDUCT.md`, acrescente:

- A SK de `nfses` é o `idDPS`, nunca a chave de acesso, porque `nNFSe` e `cNum` são gerados pelo fisco e a chave só existe depois da resposta. Consulta por chave passa pela GSI `access-key-index`.
- O grupo `nfse` do objeto `person` é compartilhado por `organizations` e `organization_persons`; `reg_trib` mora ali e não na config, porque com `tpEmit` 2 ou 3 o prestador é uma pessoa do cadastro.
- As tabelas de referência da NFS-e são geradas por `go-dfe/nfse/tables/gen/generate.py` e versionadas; nunca edite os `.go` gerados à mão. Regenerar quando a Receita publicar um anexo novo.
- O stream do outbox fica em `worker_outbox`; tabelas de documento não têm stream.

- [ ] **Step 3: Marcar a fase no spec**

Em `docs/specs/2026-08-04-nfse-design.md`, na tabela da §10, marque a F1 como concluída e registre a data.

- [ ] **Step 4: Commit**

```bash
cd /home/artur/Documents/Projects/Ctech/ctech-dfe
git add CONDUCT.md docs/specs/2026-08-04-nfse-design.md
git commit -m "docs(nfse): fecha a fase F1 do modulo nfse"
```

---

## Revisão cruzada de impacto

| Componente | Impacto da F1 |
|---|---|
| `api` | Tabelas, DTOs, validadores, rotas e RBAC novos. Nenhuma rota existente muda de contrato — o grupo `nfse` é opcional. |
| `go-dfe` | Pacote `nfse/tables` novo, sem tocar em nada existente. |
| `worker` | Nenhuma mudança de código. Recompilar para confirmar que o `go.sum` compartilhado segue consistente. |
| `ui` | Dois arquivos de dados gerados, ainda não importados. Nenhuma tela muda. |
| `cdk` | Quatro tabelas novas e o dispatcher passa a enxergar `organization_nfse_configs`. Nenhuma tabela existente é alterada. |
| `py-dfe` | Nenhum impacto — py-dfe não tem NFS-e e nunca terá. |
| `ctech-account` | Nenhum impacto. |

## O que a F1 deliberadamente não faz

- Não emite nenhum documento. `nfses` e `nfse_events` são criadas mas nenhum código as lê ou escreve — isso é a F3.
- Não implementa nenhuma comunicação com o Sefin Nacional ou com município ABRASF — isso é a F2 e a F5.
- Não cria nenhuma tela. As rotas `/services` e `/nfse` do front são a F4.
- Não valida obrigatoriedade condicional entre campos (por exemplo, `reg_ap_trib_sn` exigido quando `op_simp_nac` = 3, ou `abrasf` exigido quando `provider` = `abrasf204`). Essas regras dependem do contexto da emissão e entram junto com `NfseService.Emit`, na F3.
