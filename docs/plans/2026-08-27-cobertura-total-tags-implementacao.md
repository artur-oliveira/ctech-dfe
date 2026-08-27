# Cobertura total de tags NF-e/NFC-e/MDF-e — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Levar a emissão de NF-e (mod 55), NFC-e (mod 65) e MDF-e de ~55% / ~47% das tags do XSD para 100%, sem
transferir complexidade fiscal para o usuário final.

**Architecture:** A API Go monta um `map[string]any` que espelha a árvore XML; o py-dfe (ou go-dfe, no shadow) ordena os
filhos pela tabela `xsdorder` e assina. Nenhuma tag nova entra direto no corpo do request: cada uma é alocada por uma
régua de 8 níveis (derivado → empresa → operação → perfil tributário → produto → contraparte → cadastro dedicado →
request). Cadastros novos reusam o par genérico `OrgEntityRepository`/`OrgEntityService`, que já dá CRUD + cache +
auditoria + `name-index` de graça.

**Tech Stack:** Go 1.2x (Fiber v3, fx, `shopspring/decimal`, `aws-sdk-go-v2`), DynamoDB on-demand, Python 3.14 (py-dfe:
XML-DSig + SOAP + mTLS), Next.js 16 + TypeScript + ShadCN + react-hook-form + zod, AWS CDK (TypeScript).

**Spec:** [`docs/plans/2026-08-26-cobertura-total-tags-nfe-mdfe.md`](./2026-08-26-cobertura-total-tags-nfe-mdfe.md)
(a régua de alocação da §1 é normativa; o plano argumenta contra ela em cada tarefa).

**Documento irmão já implementado:** [
`docs/plans/2026-08-26-contingencia-e-inutilizacao-dfe.md`](./2026-08-26-contingencia-e-inutilizacao-dfe.md)
— eventos, contingência e inutilização estão fora deste plano.

---

## Global Constraints

Estas valem para **toda** tarefa. Não são repetidas nos passos.

1. **DRY antes de escrever qualquer função:** `rg "<nome>"` no repositório. Reusar → estender → parametrizar → criar.
   Duas implementações do mesmo problema têm que virar uma.
2. **Sem variável mágica:** toda string de tag XML, código numérico, prefixo de `sk`, nome de tabela e nome de header é
   constante nomeada, no bloco `const` do arquivo que já agrupa suas irmãs.
3. **Erros:** api/worker devolvem RFC 7807 via os helpers `problem.*` (`problem.BadRequest`, `problem.NotFound`,
   `problem.Conflict`, `problem.InternalServer`). Nunca `fiber.Map`, nunca erro cru. py-dfe levanta `DFeError` com
   `status_code`, `code`, `message` explícitos.
4. **Frontend:** `npx eslint src --ext .ts,.tsx` tem que passar com **zero erros e zero warnings** antes de cada commit
   que toca `ui/`.
5. **Testes:** toda tag nova tem teste unitário de builder **e** um payload de integração em
   `py-dfe/tests/integration/fiscal_payloads.py`. Regra de ouro do plano-fonte §5.
6. **Ordem XSD:** antes de implementar qualquer grupo novo, conferir
   `go-dfe/internal/xmlops/xsdorder/table.go` e `py-dfe/py_dfe/xmlops/xsd_order.py`. A tabela **já conhece quase tudo**
   (`NFref`, `DI`, `rastro`, `detExport`, `exporta`, `ICMSPart`, `ICMSST`, `retTrib`, `peri`, `valePed`,
   `infContratante`, `seg`…). Só acrescentar chave quando `rg` provar que falta — e nas **duas** tabelas, que são porte
   1:1 uma da outra.
7. **Docs no mesmo commit:** endpoint/schema/módulo novo → `DOCS.md`; tabela nova → `DynamoDB-Tables.md`;
   restrição/workaround novo → `CONDUCT.md`. Sem exceção.
8. **Escopo:** implementar só o que a tarefa pede. Nada de refactor oportunista, nada de reorganizar diretório.
9. **Commits:** Conventional Commits, sem emoji, **sem `Co-Authored-By`** e sem qualquer linha de atribuição a IA. Nunca
   commitar sem o usuário pedir — a tarefa deixa o commit pronto e sugerido.
10. **Régua de alocação:** nenhuma tag nova chega ao request de emissão sem justificativa escrita contra a §1 do spec.
    Se o mesmo valor for digitado duas vezes em notas diferentes, ele está no nível errado.

---

## Estado atual verificado (2026-08-27)

Auditado por leitura direta antes de escrever o plano. **Não repetir a auditoria** — usar isto:

| Item do spec                                                | Estado real                                                                                                                                                                                                                     |
|-------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `det/imposto/IS/adRemIS` (§2 Fase A, "correção `pISEspec`") | **Já corrigido.** `builders_tax.go:413` emite `adRemIS`; `rg pISEspec` não retorna nada. Nenhuma tarefa.                                                                                                                        |
| `xsdorder`                                                  | Cobre `NFref`, `DI`, `detExport`, `rastro`, `exporta`, `ICMSPart`, `ICMSST`, `total`(+`retTrib`,`ISTot`,`vNFTot`), `peri`, `seg`, `valePed`, `infContratante`, `infPag`, `infMDFeTransp`. Falta pouco.                          |
| `TaxFieldsBody` (`api/internal/api/v1/dto.go:171`)          | Já tem `pis_st_aliq`/`cofins_st_aliq`/`pis_st_v_bc`/`cofins_st_v_bc`, todo o bloco monofásico (`icms_ad_rem*`, `icms_p_dif_mono`), ICMS60 retido, IS, ISSQN, IBS/CBS base. O que falta é **builder**, não DTO, em vários casos. |
| `buildICMSNormal` (`builders_tax.go:120`)                   | CSTs 00, 02, 10, 15, 20, 30, 40/41/50, 51, 53, 60, 61, 70, 90. **Faltam** `ICMSPart`, `ICMSST` (41 cai em ICMS40), `vBCEfet`/`pICMSEfet` (60), `vICMSSTDeson` (10/70/90), `pFCPDif` (51/90).                                    |
| `OperationBody` (`dto.go:321`)                              | Já tem `inf_ad_fisco` **validado com placeholders**, mas `emit.go` só interpola `inf_cpl` e só escreve `infAdic/infCpl`.                                                                                                        |
| `organization_vehicles`                                     | Já tem `cint`, `cap_m3`, `renavam`, `cap_kg`, `weight`. O builder MDF-e (`mdfes/builder.go:180`) **não** emite `cInt`/`capM3`. Puro wiring.                                                                                     |
| `enabledModals` (`mdfes/mdfes.go`)                          | Só `rodoviario: true`. `buildAereo`/`buildAquav`/`buildFerrov` existem em `modals.go`.                                                                                                                                          |
| `person.nfse.im`                                            | Existe (`organizations` e `organization_persons`). `emit/IM` é só leitura.                                                                                                                                                      |
| Cadastros reutilizáveis                                     | 4 tabelas (`tax_profiles`, `operations`, `payment_terms`, `vehicle_sets`) sobre um repositório e um serviço genéricos. Este plano acrescenta 7 no mesmo molde.                                                                  |

---

## File Structure

### Arquivos que existem e vão mudar

| Arquivo                                                                          | Responsabilidade                              | O que muda                                                              |
|----------------------------------------------------------------------------------|-----------------------------------------------|-------------------------------------------------------------------------|
| `api/internal/services/nfes/builders_doc.go` (1178 L)                            | Monta `enviNFe` inteiro                       | Cresce demais nesta obra. **Split obrigatório na Task 1** (ver abaixo). |
| `api/internal/services/nfes/builders_tax.go` (657 L)                             | ICMS/IPI/IS/PIS/COFINS/IBS-CBS/ISSQN por item | Ganha CSTs e grupos faltantes.                                          |
| `api/internal/services/nfes/emit.go` (851 L)                                     | Request types + orquestração da emissão NF-e  | Novos campos de request, novas resoluções de cadastro.                  |
| `api/internal/api/v1/dto.go`                                                     | Todos os DTOs HTTP                            | Novos bodies de cadastro e novos campos fiscais.                        |
| `api/internal/services/mdfes/builder.go` (485 L)                                 | Monta `MDFe`                                  | infANTT, infDoc, seg, prodPred.                                         |
| `api/internal/services/mdfes/modals.go` (146 L)                                  | aéreo/aquav/ferrov                            | Completar aquaviário.                                                   |
| `go-dfe/internal/xmlops/xsdorder/table.go` + `py-dfe/py_dfe/xmlops/xsd_order.py` | Ordem XSD                                     | Só o que faltar, sempre nos dois.                                       |
| `api/internal/repositories/org_entities.go`                                      | Registries genéricos                          | +7 registries concretos.                                                |
| `api/internal/services/org_entities.go`                                          | Serviços dos registries                       | +7 serviços concretos + cache scopes.                                   |
| `api/internal/api/v1/org_entities.go`                                            | Rotas dos registries                          | +7 `Register*`.                                                         |
| `api/internal/api/v1/router.go`                                                  | Montagem                                      | +7 campos em `Services` e +7 chamadas.                                  |
| `api/internal/app/app.go`                                                        | fx                                            | +7 providers de repo, +7 de serviço, +7 no `registerRoutes`.            |
| `api/internal/repositories/roles.go`                                             | RBAC                                          | +7 resources.                                                           |
| `api/internal/middleware/scopes.go`                                              | Escopos OAuth                                 | +7 famílias.                                                            |
| `api/internal/repositories/audit_logs.go`                                        | Auditoria                                     | +7 `AuditResource*`.                                                    |
| `cdk/lib/dynamodb-stack.ts`                                                      | Infra                                         | +7 `getOrgEntityTable`.                                                 |
| `DOCS.md`, `DynamoDB-Tables.md`, `CONDUCT.md`                                    | Documentação                                  | A cada tarefa.                                                          |

### Arquivos novos (Go, API)

Os builders NF-e são grandes demais para caber num contexto só. O split acontece **antes** de qualquer tag nova.

| Arquivo                                         | Responsabilidade                                                                                                    |
|-------------------------------------------------|---------------------------------------------------------------------------------------------------------------------|
| `api/internal/services/nfes/builders_ide.go`    | `ide` inteiro: cabeçalho, `NFref`, contingência, reforma (`cIndOp`, `gCompraGov`).                                  |
| `api/internal/services/nfes/builders_party.go`  | `emit`, `dest`, `retirada`, `entrega`, `autXML`, `infIntermed`.                                                     |
| `api/internal/services/nfes/builders_prod.go`   | Nó `prod` de um item: identificação, `DI`, `rastro`, `comb`, `med`, `veicProd`, `arma`, `NVE`, `detExport`.         |
| `api/internal/services/nfes/builders_total.go`  | `total`: `ICMSTot`, `ISSQNtot`, `retTrib`, `ISTot`, `IBSCBSTot`, `vNFTot`.                                          |
| `api/internal/services/nfes/builders_transp.go` | `transp` inteiro: `transporta`, `retTransp`, `veicTransp`, `reboque`, `vol`, `lacres`.                              |
| `api/internal/services/nfes/builders_extra.go`  | `infAdic`, `exporta`, `compra`, `cana`, `agropecuario`, `infRespTec` (+CSRT).                                       |
| `api/internal/services/nfes/builders_ibscbs.go` | Todo o bloco da reforma no item (Fase E).                                                                           |
| `api/internal/services/nfes/references.go`      | Resolução de `NFref` a partir de documento da própria base.                                                         |
| `api/internal/services/nfes/csrt.go`            | `hashCSRT = Base64(SHA1(CSRT + chave))`.                                                                            |
| `api/internal/services/mdfes/builder_antt.go`   | `infANTT` completo: `valePed`, `infContratante`, `infPag`.                                                          |
| `api/internal/services/mdfes/builder_infdoc.go` | `infDoc`: `SegCodBarra`, `indReentrega`, `peri`, `infUnidTransp`, `infUnidCarga`, entrega parcial, `infMDFeTransp`. |
| `api/internal/services/mdfes/peri.go`           | Derivação de produto perigoso a partir dos itens da NF-e referenciada.                                              |

`builders_doc.go` fica só com `BuildEnviNFe` (a orquestração) e o laço de `det`, sob 400 linhas.

### Arquivos novos (UI)

Um por cadastro novo, sempre no molde `page.tsx` + `new/page.tsx` + `edit/page.tsx` + `components/<x>/<X>Form.tsx` +
`lib/schemas/<x>.ts`, com entradas em `lib/api/client.ts`, `lib/api/query-keys.ts`, `lib/constants/entity-keys.ts`
e `lib/types/api.ts`.

---

## Receitas compartilhadas

Estas duas receitas são usadas por muitas tarefas. Elas contêm o código real; a tarefa que as invoca diz **quais**
valores usar e acrescenta só o que é próprio dela. Não são "veja a tarefa N" — são a definição.

### Receita R1 — cadastro reutilizável novo (org entity)

Dez pontos de fiação. Nenhum é opcional; um esquecido é 403 silencioso ou tabela que não existe em produção.

**R1.1 — `api/internal/repositories/org_entities.go`**, bloco `const` do topo (linhas 21-34):

```go
    TableImportDeclarations = "organization_import_declarations"
SKPrefixImportDeclaration = "IMPORTDI_"
```

E um registry concreto no fim do arquivo, junto dos outros:

```go
// ImportDeclarationRepository — organization_import_declarations. Uma DI cobre
// várias notas e vários itens; digitá-la por item é o erro que o cadastro evita.
type ImportDeclarationRepository struct{ OrgEntityRepository }

func NewImportDeclarationRepository(db *dynamodb.Client, cfg *config.Config) *ImportDeclarationRepository {
return &ImportDeclarationRepository{newOrgEntityRepository(db, cfg, TableImportDeclarations, SKPrefixImportDeclaration)}
}
```

**R1.2 — `api/internal/services/org_entities.go`**, serviço concreto + cache scope:

```go
// ImportDeclarationService owns organization_import_declarations.
type ImportDeclarationService struct{ OrgEntityService }

func NewImportDeclarationService(repo *repositories.ImportDeclarationRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ImportDeclarationService {
return &ImportDeclarationService{newOrgEntityService(
&repo.OrgEntityRepository, auditRepo, c,
CacheScopeImportDeclarations, repositories.AuditResourceImportDeclaration, "import declaration not found",
)}
}
```

e no bloco `const` final do arquivo: `CacheScopeImportDeclarations = "import_declarations"`.

**R1.3 — `api/internal/repositories/audit_logs.go`**, bloco `const` (linhas 23-39):
`AuditResourceImportDeclaration = "IMPORT_DECLARATION"`.

**R1.4 — `api/internal/api/v1/dto.go`**: o `<X>Body` com tags `json` + `validate` (a tarefa define os campos).

**R1.5 — `api/internal/api/v1/org_entities.go`**, no fim:

```go
// RegisterImportDeclarations mounts /import-declarations under a tenant-scoped group.
func RegisterImportDeclarations(router fiber.Router, svc *services.ImportDeclarationService, userSvc *services.UserService,
authMw fiber.Handler, perm *middleware.PermChecker) {
mountOrgEntity(router, authMw, perm, userSvc, svc, orgEntityRoutes{
path:       "/import-declarations",
param:      "import_declaration_id",
resource:   "organization_import_declarations",
bindCreate: bindEntityCreate[ImportDeclarationBody],
bindUpdate: bindEntityUpdate[ImportDeclarationBody],
})
}
```

**R1.6 — `api/internal/api/v1/router.go`**: campo no struct `Services`
(`ImportDeclaration *services.ImportDeclarationService`) e a chamada em `Register`, junto das outras
`Register*` de cadastro (perto da linha 85).

**R1.7 — `api/internal/app/app.go`**: `repositories.NewImportDeclarationRepository` na lista de providers (~linha 56),
um construtor no molde de `newTaxProfileService` (~linha 258), e o campo em `registerRoutes` (~linha 487).

**R1.8 — `api/internal/repositories/roles.go`**: `"organization_import_declarations"` no slice `resources`
(linhas 69-84). Isso gera as 5 permissões automaticamente.

**R1.9 — `api/internal/middleware/scopes.go`**: entrada em `scopeFamilies`:
`"organization_import_declarations": {"organization_import_declarations"}`.

**R1.10 — `cdk/lib/dynamodb-stack.ts`** (~linha 600):

```ts
    this.tables.set('import_declarations', getOrgEntityTable(
    this, removalPolicy, pointInTimeRecoverySpecification, tablePrefix, 'import_declarations'));
```

**R1.11 — UI**, cinco arquivos no molde de `payment-terms`:
`ui/src/lib/schemas/import-declarations.ts` (zod espelhando o Body),
`ui/src/components/import-declarations/ImportDeclarationForm.tsx`,
`ui/src/app/import-declarations/{page,new/page,edit/page}.tsx`, mais `queryKeys.importDeclarations = {list, detail}` em
`lib/api/query-keys.ts`, os métodos
`getImportDeclarations/createImportDeclaration/updateImportDeclaration/deleteImportDeclaration`
em `lib/api/client.ts`, `SK_PREFIX.IMPORT_DECLARATION = 'IMPORTDI_'` em `lib/constants/entity-keys.ts`, e o tipo
`ImportDeclarationItemOut` em `lib/types/api.ts`.

**R1.12 — Docs**: nova seção numerada em `DynamoDB-Tables.md` (a próxima livre é a **37**; `36` é `account_billing`),
linha na tabela **Table Index** do topo, e o schema em `DOCS.md § Cadastros reutilizáveis`.

**Teste de fiação (uma vez por cadastro novo):** `api/internal/api/v1/org_entities_wiring_test.go` já não existe; criar
na Task 6 e estender depois:

```go
func TestEveryRegistryResourceHasScopeFamily(t *testing.T) {
for _, r := range repositories.AllResources() {
if !strings.HasPrefix(r, "organization_") {
continue
}
if _, ok := middleware.ScopeFamilies()[r]; !ok {
t.Fatalf("resource %q sem família de escopo em middleware/scopes.go", r)
}
}
}
```

(`repositories.AllResources()` e `middleware.ScopeFamilies()` são getters de uma linha a criar na Task 6, expondo os
slices/maps privados que já existem.)

### Receita R2 — tag nova no builder NF-e

O ciclo de toda tag, sem exceção:

1. `rg "<tag>" go-dfe/internal/xmlops/xsdorder/table.go py-dfe/py_dfe/xmlops/xsd_order.py` — se faltar, acrescentar
   **nos dois**, na posição exata do XSD.
2. Escolher o nível pela régua (§1 do spec) e escrever a justificativa no comentário do campo.
3. Teste unitário em `builders_*_test.go` que chama o builder direto e afirma a subárvore.
4. Implementação mínima.
5. Payload de integração em `py-dfe/tests/integration/fiscal_payloads.py` (novo parâmetro opcional em `build_nfe`, nunca
   uma cópia da função).
6. `go test ./... && cd py-dfe && python -m pytest tests/integration -k <tag>`.

---

# Bloco 0 — Preparação

## Task 1: Split de `builders_doc.go`

`builders_doc.go` tem 1178 linhas e vai receber tag em quase toda tarefa deste plano. Um agente não segura o arquivo
inteiro em contexto, e edições cegas nele são o maior risco da obra. O split é **puramente mecânico**: mover funções,
nada de mudar comportamento.

**Files:**

- Create: `api/internal/services/nfes/builders_ide.go`
- Create: `api/internal/services/nfes/builders_party.go`
- Create: `api/internal/services/nfes/builders_prod.go`
- Create: `api/internal/services/nfes/builders_total.go`
- Create: `api/internal/services/nfes/builders_transp.go`
- Create: `api/internal/services/nfes/builders_extra.go`
- Modify: `api/internal/services/nfes/builders_doc.go`

**Interfaces:**

- Consumes: nada.
- Produces: as funções abaixo, no pacote `nfes`, com assinatura **idêntica** à de hoje:
  `buildTransp`, `transportaFromRequest`, `buildPartyTransporta`, `buildPag`, `buildCobr`, `buildEnder`,
  `buildAutXML`, `buildLocal`, `truncateNatOp`, `getPersonMap`, `getAnyInt`, `getCFOPConfig`, `findCFOPEntry`,
  `firstValue`, `dv`, `strOrDefault`, `firstNonEmpty`, `getIEForUF`, `anyStr`, `anyStrPtr`. Mais três funções novas,
  extraídas do corpo de `BuildEnviNFe`:
    - `buildIde(p ideParams) map[string]any`
    - `buildEmit(org, orgPerson, orgAddress map[string]any, orgPK string, orgCRT int) map[string]any`
    - `buildTotal(t totals, now time.Time) map[string]any`

  com

  ```go
  // ideParams agrupa o que o nó ide precisa. É struct e não 15 parâmetros
  // posicionais porque ide é o nó que mais cresce neste plano.
  type ideParams struct {
      CUF, CNF, NatOp, Model, AccessKey string
      Serie, Number, Environment        int
      DhEmi                             string
      TpNF, IdDest, CMunFG              string
      FinNFe, IndFinal, IndPres         string
      Mode                              EmissionMode
      VerProc                           string
  }

  // totals agrupa os acumuladores por item que viram o nó total.
  type totals struct {
      VBC, VICMS, VICMSDeson, VFCP, VBCST, VICMSST, VFCPST decimal.Decimal
      VPIS, VCOFINS, VIPI, VFrete, VSeg, VOutro            decimal.Decimal
      VICMSUFDest, VFCPUFDest                              decimal.Decimal
      VServ, VBCISSQN, VISSQN, VPISISSQN, VCOFINSISSQN     decimal.Decimal
      IBSBC, IBSUF, IBSMun, IBS, CBSBC, CBS                decimal.Decimal
      Products, Discount                                   decimal.Decimal
      HasISSQN                                             bool
  }
  ```

- [ ] **Step 1: Fixar o comportamento atual com um teste de ouro**

Criar `api/internal/services/nfes/builders_golden_test.go`:

```go
package nfes

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TestBuildEnviNFeGolden congela a árvore produzida hoje. O split da Task 1 não
// pode mudar um byte dela; qualquer tarefa posterior que mude a saída tem que
// atualizar o golden no mesmo commit, de propósito e à vista.
func TestBuildEnviNFeGolden(t *testing.T) {
	got := BuildEnviNFe(
		goldenOrg(), goldenReceiver(), "CNPJ_11647612000197",
		goldenItems(), goldenPayments(),
		1, 1, 2,
		"22260811647612000197550010000000011100000015",
		decimal.RequireFromString("100.00"), decimal.Zero,
		nil, time.Date(2026, 8, 27, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{CNPJ: "11647612000197", Name: "Ctech", Email: "t@t.com", Phone: "8630000000", Version: "1.0"},
		nfModel55, nil, nil, nil, NormalEmission(nfModel55),
	)
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertGolden(t, "testdata/envinfe_golden.json", b)
}
```

com os helpers `goldenOrg/goldenReceiver/goldenItems/goldenPayments` copiados dos fixtures que
`builders_doc_test.go` já monta, e `assertGolden` gravando o arquivo quando `-update` é passado:

```go
var updateGolden = flag.Bool("update", false, "rewrite golden files")

func assertGolden(t *testing.T, path string, got []byte) {
t.Helper()
if *updateGolden {
if err := os.WriteFile(path, got, 0o644); err != nil {
t.Fatal(err)
}
return
}
want, err := os.ReadFile(path)
if err != nil {
t.Fatalf("golden ausente (rode com -update): %v", err)
}
if !bytes.Equal(want, got) {
t.Fatalf("árvore mudou.\nwant:\n%s\ngot:\n%s", want, got)
}
}
```

- [ ] **Step 2: Gerar o golden e conferir que ele trava**

```bash
cd api && go test ./internal/services/nfes -run TestBuildEnviNFeGolden -update
go test ./internal/services/nfes -run TestBuildEnviNFeGolden
```

Esperado: primeiro comando cria `testdata/envinfe_golden.json`, segundo passa.

- [ ] **Step 3: Mover as funções, sem editar corpo nenhum**

`git mv` não serve — é recorte por função. Distribuição:

| Destino                  | Funções movidas de `builders_doc.go`                                                                                                                                            |
|--------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `builders_transp.go`     | `buildTransp`, `transportaFromRequest`, `buildPartyTransporta`                                                                                                                  |
| `builders_party.go`      | `buildEnder`, `buildLocal`, `buildAutXML`, `getIEForUF`, `getPersonMap`                                                                                                         |
| `builders_extra.go`      | `truncateNatOp` e o `const` `natOpMaxLen`                                                                                                                                       |
| `builders_doc.go` (fica) | `BuildEnviNFe`, `buildPag`, `buildCobr`, e os helpers `anyStr`, `anyStrPtr`, `getAnyInt`, `getCFOPConfig`, `findCFOPEntry`, `firstValue`, `dv`, `strOrDefault`, `firstNonEmpty` |

- [ ] **Step 4: Extrair `buildIde`, `buildEmit` e `buildTotal` do corpo de `BuildEnviNFe`**

`buildIde` recebe `ideParams` e devolve exatamente o `map` que hoje é montado inline em
`builders_doc.go:1009-1033`, incluindo o bloco de contingência:

```go
func buildIde(p ideParams) map[string]any {
ide := map[string]any{
"cUF": p.CUF, "cNF": p.CNF, "natOp": p.NatOp, "mod": p.Model,
"serie": fmt.Sprintf("%d", p.Serie), "nNF": fmt.Sprintf("%d", p.Number),
"dhEmi": p.DhEmi, "tpNF": p.TpNF, "idDest": p.IdDest, "cMunFG": p.CMunFG,
"tpImp": p.Mode.TpImp, "tpEmis": p.Mode.TpEmis,
"cDV":   string(p.AccessKey[len(p.AccessKey)-1]),
"tpAmb": fmt.Sprintf("%d", p.Environment),
"finNFe": p.FinNFe, "indFinal": p.IndFinal, "indPres": p.IndPres,
"procEmi": procEmiApp, "verProc": p.VerProc,
}
if p.Mode.IsContingency() {
ide["dhCont"] = fmtDhEmi(p.Mode.ContingencyAt)
ide["xJust"] = p.Mode.Justification
}
return ide
}
```

`buildTotal(t totals, now time.Time)` recebe o struct e devolve o nó `total` de hoje (`builders_doc.go:947-1002`),
incluindo `ISSQNtot` quando `t.HasISSQN`. `BuildEnviNFe` passa a preencher um
`totals` no laço em vez de 25 variáveis soltas.

- [ ] **Step 5: Rodar o golden — nenhuma diferença é permitida**

```bash
cd api && go test ./internal/services/nfes/... -v
```

Esperado: PASS, incluindo `TestBuildEnviNFeGolden` **sem** `-update`. Se o golden acusar diferença, o split mudou
comportamento: desfazer e refazer o recorte, nunca regravar o golden aqui.

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/nfes/
git commit -m "refactor(nfe): divide builders_doc em ide/party/prod/total/transp/extra"
```

---

## Task 2: Split de `mdfes/builder.go` e getters de fiação

Mesma razão da Task 1, mais os dois getters que a Receita R1 exige.

**Files:**

- Create: `api/internal/services/mdfes/builder_antt.go`
- Create: `api/internal/services/mdfes/builder_infdoc.go`
- Modify: `api/internal/services/mdfes/builder.go`
- Modify: `api/internal/repositories/roles.go`
- Modify: `api/internal/middleware/scopes.go`
- Create: `api/internal/api/v1/org_entities_wiring_test.go`

**Interfaces:**

- Consumes: nada.
- Produces:
    - `func (p buildParams) buildInfANTT() map[string]any` — movida, corpo intacto, em `builder_antt.go`.
    - `func (p buildParams) buildInfDoc() map[string]any` — movida, corpo intacto, em `builder_infdoc.go`.
    - `func repositories.AllResources() []string` — cópia defensiva de `resources`.
    - `func middleware.ScopeFamilies() map[string][]string` — cópia defensiva de `scopeFamilies`.

- [ ] **Step 1: Escrever o teste de fiação (falha por não compilar)**

`api/internal/api/v1/org_entities_wiring_test.go`, com o corpo dado na Receita R1 (teste de fiação).

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/api/v1 -run TestEveryRegistryResourceHasScopeFamily
```

Esperado: FAIL de compilação — `undefined: repositories.AllResources`.

- [ ] **Step 3: Criar os getters**

Em `api/internal/repositories/roles.go`:

```go
// AllResources devolve os resources RBAC conhecidos. Cópia: a lista é global e
// mutá-la de fora reescreveria as permissões do processo inteiro.
func AllResources() []string { return append([]string(nil), resources...) }
```

Em `api/internal/middleware/scopes.go`:

```go
// ScopeFamilies devolve o mapa de famílias de escopo. Cópia rasa pela mesma
// razão de AllResources.
func ScopeFamilies() map[string][]string {
out := make(map[string][]string, len(scopeFamilies))
for k, v := range scopeFamilies {
out[k] = append([]string(nil), v...)
}
return out
}
```

- [ ] **Step 4: Mover `buildInfANTT` e `buildInfDoc`, rodar tudo**

```bash
cd api && go test ./internal/services/mdfes/... ./internal/api/v1/... ./internal/middleware/...
```

Esperado: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/mdfes/ api/internal/repositories/roles.go api/internal/middleware/scopes.go api/internal/api/v1/org_entities_wiring_test.go
git commit -m "refactor(mdfe): separa infANTT e infDoc; expõe getters de fiação RBAC"
```

---

# Bloco 1 — NF-e Fase A: desbloqueadores

Ordem 1 do spec §4. Sem isto, devolução, exportação por transportadora, marketplace e NFC-e com POS são impossíveis.

## Task 3: `ide/NFref` — documento referenciado

Sem `NFref`, `finNFe` 2/3/4 (complementar, ajuste, devolução) é rejeitado pela SEFAZ. Nível 7 + derivado: o cliente
**escolhe uma nota da própria base** e o tipo de referência sai do modelo do documento; só documento de fora do sistema
vira formulário.

**Files:**

- Create: `api/internal/services/nfes/references.go`
- Create: `api/internal/services/nfes/references_test.go`
- Modify: `api/internal/services/nfes/emit.go` (struct `NfeEmitBody`; chamada de `BuildEnviNFe`)
- Modify: `api/internal/services/nfes/builders_ide.go`
- Modify: `api/internal/services/nfes/builders_doc.go` (passar `nfRefs` para `buildIde`)
- Modify: `ui/src/lib/schemas/…` e `ui/src/components/nfe/NfeEmitForm.tsx`
- Test: `api/internal/services/nfes/references_test.go`, `py-dfe/tests/integration/fiscal_payloads.py`

**Interfaces:**

- Consumes: `ideParams` (Task 1); `repositories.NfeRepository.Get`.
- Produces:
    - `type NfeRefBody struct` (abaixo) — usado pelo request e pela UI.
    -
    `func (s *NfeService) resolveNFRefs(ctx context.Context, orgPK, envPrefix string, refs []NfeRefBody) ([]map[string]any, error)`
    - `func buildNFref(refs []map[string]any) []map[string]any`
    - `ideParams` ganha o campo `NFref []map[string]any`.

- [ ] **Step 1: Conferir a ordem XSD**

```bash
rg '"NFref"|refNFeSig|refNFP' go-dfe/internal/xmlops/xsdorder/table.go py-dfe/py_dfe/xmlops/xsd_order.py
```

Esperado: `"NFref": {"refNFe", "refNFeSig", "refNF", "refNFP", "refCTe", "refECF"}` presente nos dois, e `NFref` já
listado em `infNFe:ide`. Nada a acrescentar. Se faltar `refNF`/`refNFP`/`refECF` como chaves próprias, acrescentar:

```go
    "refNF":  {"cUF", "AAMM", "CNPJ", "mod", "serie", "nNF"},
"refNFP": {"cUF", "AAMM", "CNPJ", "CPF", "IE", "mod", "serie", "nNF"},
"refECF": {"mod", "nECF", "nCOO"},
```

- [ ] **Step 2: Escrever o teste que falha**

`api/internal/services/nfes/references_test.go`:

```go
package nfes

import (
	"reflect"
	"testing"
)

func TestBuildNFrefChaveDeNotaDaBase(t *testing.T) {
	got := buildNFref([]map[string]any{
		{"kind": refKindNFe, "access_key": "22260811647612000197550010000000011100000015"},
	})
	want := []map[string]any{
		{"refNFe": "22260811647612000197550010000000011100000015"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestBuildNFrefNotaModelo1ForaDoSistema(t *testing.T) {
	got := buildNFref([]map[string]any{{
		"kind": refKindNF, "c_uf": "22", "aamm": "2608", "cnpj": "11647612000197",
		"mod": "01", "serie": "1", "n_nf": "42",
	}})
	inner, ok := got[0]["refNF"].(map[string]any)
	if !ok {
		t.Fatalf("refNF ausente: %v", got)
	}
	if inner["mod"] != "01" || inner["nNF"] != "42" || inner["AAMM"] != "2608" {
		t.Fatalf("refNF errado: %v", inner)
	}
}

func TestBuildNFrefCupomFiscal(t *testing.T) {
	got := buildNFref([]map[string]any{{
		"kind": refKindECF, "mod": "2D", "n_ecf": "001", "n_coo": "000123",
	}})
	inner := got[0]["refECF"].(map[string]any)
	if inner["mod"] != "2D" || inner["nECF"] != "001" || inner["nCOO"] != "000123" {
		t.Fatalf("refECF errado: %v", inner)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildNFref
```

Esperado: FAIL — `undefined: buildNFref`, `undefined: refKindNFe`.

- [ ] **Step 4: Implementar `references.go`**

```go
package nfes

// references.go resolve o grupo ide/NFref. A regra de produto: o cliente
// escolhe uma nota da própria base e o tipo de referência é derivado do modelo
// do documento; formulário só existe para documento que o sistema nunca viu
// (NF modelo 1/1A em papel, nota de produtor, cupom de ECF).

// Tipos de referência (ide/NFref, leiauteNFe_v4.00).
const (
	refKindNFe    = "nfe"    // refNFe    — chave de 44 dígitos de NF-e/NFC-e
	refKindNFeSig = "nfesig" // refNFeSig — chave com sigilo do destinatário
	refKindNF     = "nf"     // refNF     — NF modelo 1/1A
	refKindNFP    = "nfp"    // refNFP    — NF de produtor rural
	refKindCTe    = "cte"    // refCTe    — chave de CT-e
	refKindECF    = "ecf"    // refECF    — cupom fiscal
)

// buildNFref traduz as referências já resolvidas para os nós do XSD.
// Uma entrada com kind desconhecido é descartada em silêncio de propósito: o
// domínio é fechado pela validação do request (oneof), então chegar aqui com
// outro valor é bug, não input.
func buildNFref(refs []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, r := range refs {
		switch anyStr(r, "kind", "") {
		case refKindNFe:
			out = append(out, map[string]any{"refNFe": anyStr(r, "access_key", "")})
		case refKindNFeSig:
			out = append(out, map[string]any{"refNFeSig": anyStr(r, "access_key", "")})
		case refKindCTe:
			out = append(out, map[string]any{"refCTe": anyStr(r, "access_key", "")})
		case refKindNF:
			out = append(out, map[string]any{"refNF": map[string]any{
				"cUF": anyStr(r, "c_uf", ""), "AAMM": anyStr(r, "aamm", ""),
				"CNPJ": anyStr(r, "cnpj", ""), "mod": anyStr(r, "mod", ""),
				"serie": anyStr(r, "serie", ""), "nNF": anyStr(r, "n_nf", ""),
			}})
		case refKindNFP:
			inner := map[string]any{
				"cUF": anyStr(r, "c_uf", ""), "AAMM": anyStr(r, "aamm", ""),
			}
			if v := anyStr(r, "cnpj", ""); v != "" {
				inner["CNPJ"] = v
			} else {
				inner["CPF"] = anyStr(r, "cpf", "")
			}
			inner["IE"] = anyStr(r, "ie", "")
			inner["mod"] = anyStr(r, "mod", "")
			inner["serie"] = anyStr(r, "serie", "")
			inner["nNF"] = anyStr(r, "n_nf", "")
			out = append(out, map[string]any{"refNFP": inner})
		case refKindECF:
			out = append(out, map[string]any{"refECF": map[string]any{
				"mod": anyStr(r, "mod", ""), "nECF": anyStr(r, "n_ecf", ""),
				"nCOO": anyStr(r, "n_coo", ""),
			}})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
```

- [ ] **Step 5: Rodar e ver passar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildNFref -v
```

Esperado: PASS nos três.

- [ ] **Step 6: Ligar ao request e ao `ide`**

Em `emit.go`, dentro de `NfeEmitBody`:

```go
    // NFRefs são os documentos referenciados (ide/NFref). finNFe 2/3/4 é
// rejeitado sem pelo menos um. `nfe_id` referencia uma nota da própria base
// e dispensa o resto: chave e tipo saem do registro.
NFRefs []NfeRefBody `json:"nf_refs" validate:"omitempty,max=500,dive"`
```

e o body:

```go
// NfeRefBody é uma referência de documento em ide/NFref. Ou `nfe_id` (uma nota
// desta organização, de onde chave e tipo são derivados), ou os campos do
// documento de fora do sistema.
type NfeRefBody struct {
NfeID *string `json:"nfe_id" validate:"omitempty"`
Kind  *string `json:"kind" validate:"omitempty,oneof=nfe nfesig nf nfp cte ecf"`
// Chave de 44 dígitos, para kind nfe/nfesig/cte informados manualmente.
AccessKey *string `json:"access_key" validate:"omitempty,len=44,number"`
CUF       *string `json:"c_uf" validate:"omitempty,len=2,number"`
AAMM      *string `json:"aamm" validate:"omitempty,len=4,number"`
CNPJ      *string `json:"cnpj" validate:"omitempty,cnpj"`
CPF       *string `json:"cpf" validate:"omitempty,cpf"`
IE        *string `json:"ie" validate:"omitempty,max=14"`
Mod       *string `json:"mod" validate:"omitempty,max=2"`
Serie     *string `json:"serie" validate:"omitempty,max=3,number"`
NNF       *string `json:"n_nf" validate:"omitempty,max=9,number"`
NECF      *string `json:"n_ecf" validate:"omitempty,max=3,number"`
NCOO      *string `json:"n_coo" validate:"omitempty,max=6,number"`
}
```

`resolveNFRefs` em `references.go` — a parte "derivada" da régua:

```go
// resolveNFRefs converte o request em entradas prontas para buildNFref. Uma
// referência com nfe_id vira refNFe (ou refNFeSig, quando a nota foi emitida
// com destinatário em sigilo) lendo a nota da base; as demais passam direto.
func (s *NfeService) resolveNFRefs(ctx context.Context, orgPK, envPrefix string, refs []NfeRefBody) ([]map[string]any, error) {
out := make([]map[string]any, 0, len(refs))
for _, r := range refs {
if r.NfeID == nil || *r.NfeID == "" {
if r.Kind == nil {
return nil, problem.BadRequest("nf_refs: informe nfe_id ou kind")
}
out = append(out, map[string]any{
"kind": *r.Kind, "access_key": ptrStr(r.AccessKey),
"c_uf": ptrStr(r.CUF), "aamm": ptrStr(r.AAMM), "cnpj": ptrStr(r.CNPJ),
"cpf": ptrStr(r.CPF), "ie": ptrStr(r.IE), "mod": ptrStr(r.Mod),
"serie": ptrStr(r.Serie), "n_nf": ptrStr(r.NNF),
"n_ecf": ptrStr(r.NECF), "n_coo": ptrStr(r.NCOO),
})
continue
}
item, err := s.nfeRepo.Get(ctx, fmt.Sprintf("%s#%s", envPrefix, services.StripPKPrefix(orgPK)), *r.NfeID)
if err != nil {
return nil, err
}
if item == nil {
return nil, problem.NotFound("documento referenciado não encontrado: " + *r.NfeID)
}
out = append(out, map[string]any{"kind": refKindNFe, "access_key": strAttr(item, "sk")})
}
return out, nil
}
```

Em `Emit`, logo depois de `resolveTransport`:

```go
    nfRefs, err := s.resolveNFRefs(ctx, orgPK, envPrefix, req.NFRefs)
if err != nil {
return nil, err
}
```

e a regra de negócio que o XSD não pega sozinho, junto das validações do topo de `Emit`:

```go
    if finNFeExigeRef[finNFe] && len(nfRefs) == 0 {
return nil, problem.BadRequest("finNFe " + finNFe + " exige pelo menos um documento em nf_refs")
}
```

com

```go
// finNFeExigeRef: complementar, ajuste e devolução só existem contra um
// documento anterior (leiauteNFe_v4.00, regra B25a).
var finNFeExigeRef = map[string]bool{"2": true, "3": true, "4": true}
```

Em `buildIde`, no fim, antes do `return`:

```go
    if len(p.NFref) > 0 {
ide["NFref"] = p.NFref
}
```

- [ ] **Step 7: Teste de emissão ponta a ponta e payload de integração**

Em `emit_test.go`, um teste que chama `Emit` com `FinNFe: ptr("4")` e `NFRefs` vazio e afirma o 400; outro com
`NFRefs: []NfeRefBody{{Kind: ptr("nfe"), AccessKey: ptr("2226…")}}` e afirma `enviNFe.NFe.infNFe.ide.NFref`.

Em `py-dfe/tests/integration/fiscal_payloads.py`, `build_nfe` ganha o parâmetro opcional:

```python
def build_nfe(..., nfref: list[dict] | None = None) -> tuple[dict, str]:
    ...
    if nfref:
        ide["NFref"] = nfref
```

e o teste `test_nfe_service.py::test_nfe_devolucao_com_nfref` monta uma devolução (`finNFe="4"`) contra uma chave
autorizada na sessão anterior.

- [ ] **Step 8: UI — seletor de nota**

`ui/src/components/nfe/NfeEmitForm.tsx` ganha uma seção "Documentos referenciados", visível quando `fin_nfe != '1'`:
um combobox que consulta `apiClient.getNfes({search})` e adiciona `{nfe_id}` à lista, mais um "documento externo"
que abre os campos de `refNF`/`refNFP`/`refECF`. Schema zod novo em `ui/src/lib/schemas/nfe-refs.ts`, espelhando
`NfeRefBody`, com `superRefine` exigindo `nfe_id` **ou** `kind`.

- [ ] **Step 9: Rodar tudo e commitar**

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx
```

Esperado: PASS e zero warnings. Atualizar `DOCS.md` (§NF-e — corpo da emissão) no mesmo commit.

```bash
git add api/internal/services/nfes ui/src DOCS.md
git commit -m "feat(nfe): referencia documentos anteriores em ide/NFref"
```

---

## Task 4: `emit/IM`, `emit/CNAE`, `emit/IEST` e `emit/ISUFEmit`

Nível 1 (empresa). `IM` já está no cadastro e só precisa ser lido; os outros três são campos novos.
`ISUFEmit` (Suframa do emitente) é da reforma mas mora aqui, no mesmo nó, e sai junto de graça.

**Files:**

- Modify: `api/internal/api/v1/dto.go` (`PersonObjectBody`, `StateRegistrationBody`)
- Modify: `api/internal/services/nfes/builders_party.go`
- Modify: `api/internal/services/nfes/builders_doc.go` (`emitStruct` sai de `BuildEnviNFe` e vira `buildEmit`)
- Modify: `DynamoDB-Tables.md` (§2 e §6), `DOCS.md`
- Modify: `ui/src/components/EntityForm.tsx`, `ui/src/lib/schemas/entity.ts`
- Test: `api/internal/services/nfes/builders_party_test.go`

**Interfaces:**

- Consumes: `buildEmit` (Task 1).
- Produces: `buildEmit` passa a emitir `IEST`, `IM`, `CNAE`, `ISUFEmit` quando presentes.
  `getIESTForUF(person map[string]any, uf string) string` em `builders_party.go`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildEmitIncluiIESTIMCNAE(t *testing.T) {
org := map[string]any{"name": "ACME", "person": map[string]any{
"crt": 3,
"cnae": "4712100",
"isuf_emit": "123456789",
"nfse": map[string]any{"im": "998877"},
"state_registrations": []any{
map[string]any{"uf": "PI", "state_registration": "194000000", "ie_st": "070000000"},
},
"addresses": []any{map[string]any{"state_federation": "PI", "city": "Teresina"}},
}}
got := buildEmit(org, getPersonMap(org), map[string]any{"state_federation": "PI"}, "CNPJ_11647612000197", 3)
for k, want := range map[string]string{"IEST": "070000000", "IM": "998877", "CNAE": "4712100", "ISUFEmit": "123456789"} {
if got[k] != want {
t.Fatalf("%s: want %q, got %v", k, want, got[k])
}
}
}

// Sem os campos, as tags têm que estar ausentes — tag vazia é rejeição.
func TestBuildEmitOmiteCamposAusentes(t *testing.T) {
org := map[string]any{"name": "ACME", "person": map[string]any{
"crt": 1,
"state_registrations": []any{map[string]any{"uf": "PI", "state_registration": "194000000"}},
"addresses":           []any{map[string]any{"state_federation": "PI"}},
}}
got := buildEmit(org, getPersonMap(org), map[string]any{"state_federation": "PI"}, "CNPJ_11647612000197", 1)
for _, k := range []string{"IEST", "IM", "CNAE", "ISUFEmit"} {
if _, ok := got[k]; ok {
t.Fatalf("%s não deveria estar presente", k)
}
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildEmit
```

Esperado: FAIL — chaves ausentes.

- [ ] **Step 3: Implementar**

Em `builders_party.go`:

```go
// getIESTForUF devolve a inscrição de substituto tributário na UF de destino.
// Mora na mesma lista de state_registrations que a IE: é a mesma inscrição, no
// mesmo cadastro, com outro papel — criar uma segunda lista duplicaria a UF.
func getIESTForUF(person map[string]any, uf string) string {
regs, ok := person["state_registrations"].([]any)
if !ok {
return ""
}
for _, r := range regs {
rm, ok := r.(map[string]any)
if !ok || rm["uf"] != uf {
continue
}
if v, ok := rm["ie_st"].(string); ok {
return v
}
}
return ""
}
```

e em `buildEmit`, depois de `IE` e antes de `CRT` (a ordem XSD é
`CNPJ|CPF, xNome, xFant, enderEmit, IE, IEST, IM, CNAE, CRT, ISUFEmit`):

```go
    // IEST só é informado na operação interestadual em que o emitente é
// substituto tributário na UF de destino.
if iest := getIESTForUF(orgPerson, destUF); iest != "" {
emit["IEST"] = iest
}
// IM já existe no cadastro (person.nfse.im) por causa da NFS-e — a NF-e
// mista só precisa lê-lo. CNAE é obrigatório quando IM está presente.
if nfse, ok := orgPerson["nfse"].(map[string]any); ok {
if im := anyStr(nfse, "im", ""); im != "" {
emit["IM"] = im
if cnae := anyStr(orgPerson, "cnae", ""); cnae != "" {
emit["CNAE"] = cnae
}
}
}
if suframa := anyStr(orgPerson, "isuf_emit", ""); suframa != "" {
emit["ISUFEmit"] = suframa
}
```

`buildEmit` ganha o parâmetro `destUF string` (o `IEST` depende da UF de destino) — atualizar a chamada em
`BuildEnviNFe`.

- [ ] **Step 4: DTO + validação**

`StateRegistrationBody` ganha `IeSt *string \`json:"ie_st" validate:"omitempty,max=20"\``.
`PersonObjectBody` ganha:

```go
    // CNAE do emitente. Exigido pelo leiaute quando IM está presente (NF-e
// mista mercadoria + serviço).
Cnae *string `json:"cnae" validate:"omitempty,len=7,number"`
// Inscrição Suframa do emitente (emit/ISUFEmit, reforma tributária).
IsufEmit *string `json:"isuf_emit" validate:"omitempty,max=9,number"`
```

- [ ] **Step 5: Rodar, atualizar docs, commitar**

```bash
cd api && go test ./internal/services/nfes/... ./internal/api/v1/... && cd ../ui && npx eslint src --ext .ts,.tsx
```

`DynamoDB-Tables.md` §2 e §6 ganham as linhas `person.cnae`, `person.isuf_emit`, e `ie_st` dentro de
`person.state_registrations`.

```bash
git add api ui DynamoDB-Tables.md DOCS.md
git commit -m "feat(nfe): emite IEST, IM, CNAE e ISUFEmit no grupo emit"
```

---

## Task 5: `dest/idEstrangeiro`

Nível 5 (contraparte). Venda a pessoa no exterior e NFC-e a turista: hoje impossível, porque `dest` só aceita CPF/CNPJ.

**Files:**

- Modify: `api/internal/api/v1/dto.go` (`PersonCreateBody`, `PersonObjectBody`)
- Modify: `api/internal/services/nfes/builders_party.go` (extrair `buildDest`)
- Modify: `api/internal/services/nfes/builders_doc.go`
- Modify: `api/internal/repositories/persons.go` (SK para pessoa sem CPF/CNPJ)
- Test: `api/internal/services/nfes/builders_party_test.go`

**Interfaces:**

- Consumes: `getIEForUF`, `buildEnder`.
- Produces:
  `func buildDest(receiver, destPerson map[string]any, receiverSK, destUF string, isNFCe bool, environment int, destIE string) map[string]any`
  — move o `switch` que hoje monta `destStruct` em `builders_doc.go:922-946` e acrescenta o ramo estrangeiro.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildDestEstrangeiro(t *testing.T) {
receiver := map[string]any{"name": "John Doe", "sk": "IDEST_A1234567", "person": map[string]any{
"id_estrangeiro": "A1234567",
"addresses":      []any{map[string]any{"state_federation": "EX", "city": "Exterior"}},
}}
got := buildDest(receiver, getPersonMap(receiver), "IDEST_A1234567", "EX", false, 1, "")
if got["idEstrangeiro"] != "A1234567" {
t.Fatalf("idEstrangeiro ausente: %v", got)
}
if _, ok := got["CPF"]; ok {
t.Fatal("CPF não pode coexistir com idEstrangeiro (choice do XSD)")
}
if _, ok := got["CNPJ"]; ok {
t.Fatal("CNPJ não pode coexistir com idEstrangeiro")
}
if got["indIEDest"] != indIEDestNaoContrib {
t.Fatalf("estrangeiro é sempre não contribuinte: %v", got["indIEDest"])
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildDestEstrangeiro
```

Esperado: FAIL — `undefined: buildDest`.

- [ ] **Step 3: Implementar**

Prefixo de SK novo em `repositories/persons.go`, junto de `CNPJ_`/`CPF_`:

```go
// SKPrefixForeign identifica a pessoa sem CPF/CNPJ (dest/idEstrangeiro). O
// documento estrangeiro não tem formato fixo, então a unicidade por org é a
// própria string informada.
const SKPrefixForeign = "IDEST_"
```

Em `builders_party.go`:

```go
func buildDest(receiver, destPerson map[string]any, receiverSK, destUF string, isNFCe bool, environment int, destIE string) map[string]any {
if len(receiver) == 0 {
return nil
}
dest := map[string]any{}
switch {
case strings.HasPrefix(receiverSK, repositories.SKPrefixForeign):
// choice do XSD: idEstrangeiro exclui CPF e CNPJ.
dest["idEstrangeiro"] = services.StripPKPrefix(receiverSK)
case strings.HasPrefix(receiverSK, "CNPJ_"):
dest["CNPJ"] = services.StripPKPrefix(receiverSK)
default:
dest["CPF"] = services.StripPKPrefix(receiverSK)
}

if isNFCe {
dest["indIEDest"] = indIEDestNaoContrib
return dest
}

receiverName := anyStr(receiver, "name", "")
if environment != 1 {
receiverName = homNameReceiver
}
dest["xNome"] = receiverName
dest["enderDest"] = buildEnder(destPerson)
if destIE != "" && destUF != ufExterior {
dest["IE"] = destIE
dest["indIEDest"] = indIEDestContrib
} else {
dest["indIEDest"] = indIEDestNaoContrib
}
if email := services.FirstEmail(destPerson); email != "" {
dest["email"] = email
}
return dest
}
```

com `const ufExterior = "EX"` junto das outras constantes de `builders_doc.go`.

`PersonCreateBody` passa a aceitar documento estrangeiro:

```go
    // IDEstrangeiro é o documento de pessoa no exterior (dest/idEstrangeiro).
// Alternativa a cpf_or_cnpj, nunca acompanhante — o XSD é um choice.
IDEstrangeiro *string `json:"id_estrangeiro" validate:"omitempty,max=20"`
```

com validação de struct: exatamente um de `cpf_or_cnpj` / `id_estrangeiro`.

- [ ] **Step 4: Rodar, ver passar, e conferir que a emissão comum não mudou**

```bash
cd api && go test ./internal/services/nfes/... -v
```

Esperado: PASS, **inclusive `TestBuildEnviNFeGolden`** — o refactor de `buildDest` não pode mexer no caso comum.

- [ ] **Step 5: UI + docs + commit**

`EntityForm.tsx` ganha o toggle "pessoa no exterior" na variante `person`, que troca o campo de documento.
`DynamoDB-Tables.md` §6: SK passa a admitir `IDEST_{documento}`.

```bash
git add api ui DynamoDB-Tables.md DOCS.md
git commit -m "feat(nfe): aceita destinatário no exterior via dest/idEstrangeiro"
```

---

## Task 6: `transp` completo — `vol`, `lacres`, `RNTC`, `reboque`

Hoje `buildTransp` só monta um volume com peso. `esp`/`marca` são nível 2 (operação) ou 4 (produto);
`nVol` e `lacres` são nível 7. `RNTC` e `reboque` são nível 0 — o cadastro de veículos **já tem** os campos.

**Files:**

- Modify: `api/internal/services/nfes/builders_transp.go`
- Modify: `api/internal/services/nfes/emit.go` (`NfeTransportItem`, `resolveTransport`)
- Modify: `api/internal/api/v1/dto.go` (`OperationBody`)
- Test: `api/internal/services/nfes/builders_transp_test.go`

**Interfaces:**

- Consumes: `buildTransp` (Task 1).
- Produces: `buildTransp` ganha os parâmetros `vols []map[string]any` e `reboques []map[string]any`, e
  `veicTransp` passa a emitir `RNTC`. Assinatura nova:

  ```go
  func buildTransp(
      hasPesoL, hasPesoB bool, totalPesoL, totalPesoB decimal.Decimal,
      transport, emitTransporta, destTransporta map[string]any,
      vols []map[string]any, reboques []map[string]any,
  ) map[string]any
  ```

- [ ] **Step 1: Teste que falha**

```go
func TestBuildTranspVolumesELacres(t *testing.T) {
transport := map[string]any{"mod_frete": "1", "veiculo_placa": "ABC1D23", "veiculo_uf": "PI", "veiculo_rntrc": "12345678"}
vols := []map[string]any{{
"qVol": "2", "esp": "CAIXA", "marca": "ACME", "nVol": "001/002",
"pesoL": "10.000", "pesoB": "12.000",
"lacres": []map[string]any{{"nLacre": "L1"}, {"nLacre": "L2"}},
}}
reboques := []map[string]any{{"placa": "XYZ9Z99", "UF": "PI", "RNTC": "87654321"}}

got := buildTransp(false, false, decimal.Zero, decimal.Zero, transport, nil, nil, vols, reboques)

if got["veicTransp"].(map[string]any)["RNTC"] != "12345678" {
t.Fatalf("RNTC ausente: %v", got["veicTransp"])
}
if !reflect.DeepEqual(got["reboque"], reboques) {
t.Fatalf("reboque errado: %v", got["reboque"])
}
v := got["vol"].([]map[string]any)[0]
if v["esp"] != "CAIXA" || v["nVol"] != "001/002" || len(v["lacres"].([]map[string]any)) != 2 {
t.Fatalf("vol incompleto: %v", v)
}
}

// Sem volume explícito, o comportamento antigo continua: um volume só, com peso.
func TestBuildTranspVolumeDerivadoDosPesos(t *testing.T) {
got := buildTransp(true, true, decimal.RequireFromString("3.5"), decimal.RequireFromString("4.0"),
map[string]any{"mod_frete": "0"}, nil, nil, nil, nil)
vols := got["vol"].([]map[string]any)
if len(vols) != 1 || vols[0]["qVol"] != qVolPadrao || vols[0]["pesoL"] != "3.500" {
t.Fatalf("volume derivado errado: %v", vols)
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildTransp
```

Esperado: FAIL — assinatura antiga, `vol` é `map` e não lista.

- [ ] **Step 3: Implementar**

Substituir o bloco de `vol` em `buildTransp` por:

```go
    // vol é lista no XSD (0..N). Volume explícito vence; sem nenhum, o
// comportamento antigo é preservado — um volume sintético com os pesos
// somados dos itens, que é o que a maioria das notas precisa.
switch {
case len(vols) > 0:
transp["vol"] = vols
case hasPesoL || hasPesoB:
vol := map[string]any{"qVol": qVolPadrao}
if hasPesoL {
vol["pesoL"] = totalPesoL.StringFixed(3)
}
if hasPesoB {
vol["pesoB"] = totalPesoB.StringFixed(3)
}
transp["vol"] = []map[string]any{vol}
}
if len(reboques) > 0 {
transp["reboque"] = reboques
}
```

e no bloco de `veiculo`, acrescentar `RNTC` (hoje `resolveTransport` já lê `owner.rntrc` do veículo cadastrado, e
`buildTransp` grava em `"RNTRC"` — nome errado para NF-e, cuja tag é `RNTC`):

```go
        if v := anyStr(transport, "veiculo_rntrc", ""); v != "" {
veiculo["RNTC"] = v
}
```

**Atenção:** a tag hoje escrita como `RNTRC` em `veicTransp` é inválida no leiaute da NF-e (`RNTRC` é MDF-e/CT-e).
Corrigir e atualizar o golden da Task 1 no mesmo commit, com a mudança citada na mensagem.

Request:

```go
// NfeVolBody é um volume transportado (transp/vol).
type NfeVolBody struct {
QVol   *string  `json:"q_vol" validate:"omitempty,max=15,number"`
Esp    *string  `json:"esp" validate:"omitempty,max=60"`
Marca  *string  `json:"marca" validate:"omitempty,max=60"`
NVol   *string  `json:"n_vol" validate:"omitempty,max=60"`
PesoL  *string  `json:"peso_l" validate:"omitempty,weight3"`
PesoB  *string  `json:"peso_b" validate:"omitempty,weight3"`
Lacres []string `json:"lacres" validate:"omitempty,max=5000,dive,max=60"`
}

// NfeReboqueBody é um reboque do veículo transportador (transp/reboque, máx 5).
type NfeReboqueBody struct {
Placa string  `json:"placa" validate:"required,placa"`
UF    string  `json:"uf" validate:"required,uf"`
RNTC  *string `json:"rntc" validate:"omitempty,max=20"`
}
```

`NfeTransportItem` ganha `Vols []NfeVolBody` e `Reboques []NfeReboqueBody` (`validate:"omitempty,max=5,dive"` no
segundo). `esp`/`marca` sem valor no request caem para o default da operação:
`OperationBody` ganha `VolEsp *string` e `VolMarca *string` (`max=60`), aplicados em `resolveTransport`.

- [ ] **Step 4: Rodar e regravar o golden de propósito**

```bash
cd api && go test ./internal/services/nfes -run TestBuildTransp -v
go test ./internal/services/nfes -run TestBuildEnviNFeGolden -update
git diff api/internal/services/nfes/testdata/envinfe_golden.json
```

Esperado: o diff mostra **só** `vol` virando lista. Qualquer outra diferença é bug.

- [ ] **Step 5: Payload de integração, UI e commit**

`build_nfe(..., vols=None, reboques=None)`. Na UI, `NfeEmitForm.tsx` ganha a seção "Volumes" com
`useFieldArray`.

```bash
git add api ui DOCS.md
git commit -m "feat(nfe): volumes, lacres, reboques e RNTC no grupo transp"
```

---

## Task 7: `infAdic` completo — `infAdFisco`, `obsCont`, `obsFisco`, `procRef`

Nível 2. Hoje só `infCpl` é preenchido, embora `OperationBody.InfAdFisco` já exista e já valide placeholders. Reusar
`services.interpolate.go` — não escrever um segundo interpolador.

**Files:**

- Modify: `api/internal/services/nfes/builders_extra.go`
- Modify: `api/internal/services/nfes/emit.go`
- Modify: `api/internal/api/v1/dto.go` (`OperationBody`)
- Test: `api/internal/services/nfes/builders_extra_test.go`

**Interfaces:**

- Consumes: `services.ValidatePlaceholders`, `interpolateOperationText` (já em `operations.go`).
- Produces:
  `func buildInfAdic(infAdFisco, infCpl string, obsCont, obsFisco []map[string]any, procRef []map[string]any) map[string]any`

- [ ] **Step 1: Teste que falha**

```go
func TestBuildInfAdicTodosOsDestinos(t *testing.T) {
got := buildInfAdic(
"Beneficio fiscal 123", "Obrigado pela preferencia",
[]map[string]any{{"@xCampo": "Pedido", "xTexto": "42"}},
[]map[string]any{{"@xCampo": "Regime", "xTexto": "Especial"}},
[]map[string]any{{"nProc": "0001/2026", "indProc": "0"}},
)
if got["infAdFisco"] != "Beneficio fiscal 123" || got["infCpl"] != "Obrigado pela preferencia" {
t.Fatalf("textos errados: %v", got)
}
if len(got["obsCont"].([]map[string]any)) != 1 || len(got["procRef"].([]map[string]any)) != 1 {
t.Fatalf("listas ausentes: %v", got)
}
}

func TestBuildInfAdicVazioDevolveNil(t *testing.T) {
if buildInfAdic("", "", nil, nil, nil) != nil {
t.Fatal("infAdic vazio tem que ser omitido, não presente e vazio")
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildInfAdic
```

Esperado: FAIL — `undefined: buildInfAdic`.

- [ ] **Step 3: Implementar**

```go
// buildInfAdic monta infAdic. Ordem XSD: infAdFisco, infCpl, obsCont, obsFisco,
// procRef. Devolve nil quando não há nada — nó vazio é rejeição.
func buildInfAdic(infAdFisco, infCpl string, obsCont, obsFisco, procRef []map[string]any) map[string]any {
node := map[string]any{}
if infAdFisco != "" {
node["infAdFisco"] = infAdFisco
}
if infCpl != "" {
node["infCpl"] = infCpl
}
if len(obsCont) > 0 {
node["obsCont"] = obsCont
}
if len(obsFisco) > 0 {
node["obsFisco"] = obsFisco
}
if len(procRef) > 0 {
node["procRef"] = procRef
}
if len(node) == 0 {
return nil
}
return node
}
```

Em `Emit`, ao lado do `infCpl` que já existe:

```go
    infAdFisco, err := interpolateOperationText(operation, opFieldInfAdFisco, interpVars)
if err != nil {
return nil, err
}
```

com `opFieldInfAdFisco = "inf_ad_fisco"` junto dos outros `opField*` em `operations.go`.

`OperationBody` ganha as observações fixas por operação (nível 2 — texto que se repete por cenário):

```go
    // ObsCont/ObsFisco são observações de campo livre do leiaute (máx 10 cada).
// Aceitam os mesmos placeholders de inf_cpl.
ObsCont  []ObsBody `json:"obs_cont" validate:"omitempty,max=10,dive"`
ObsFisco []ObsBody `json:"obs_fisco" validate:"omitempty,max=10,dive"`
```

```go
// ObsBody é um par campo/texto de infAdic (obsCont ou obsFisco).
type ObsBody struct {
XCampo string `json:"x_campo" validate:"required,max=20"`
XTexto string `json:"x_texto" validate:"required,max=60"`
}
```

`procRef` (nº do processo de benefício fiscal) é nível 7 e entra em `NfeEmitBody`:

```go
    // ProcRef são processos referenciados (infAdic/procRef): nº do processo e
// sua origem. indProc: 0 SEFAZ, 1 Justiça Federal, 2 Justiça Estadual,
// 3 Secex/RFB, 9 outros.
ProcRef []NfeProcRefBody `json:"proc_ref" validate:"omitempty,max=100,dive"`
```

`validateOperationPlaceholders` passa a percorrer também os `XTexto` de `ObsCont`/`ObsFisco`.

- [ ] **Step 4: Rodar, atualizar golden, commitar**

```bash
cd api && go test ./internal/services/nfes/... ./internal/api/v1/...
```

Esperado: PASS. O golden não muda (fixture não tem operação).

```bash
git add api ui DOCS.md
git commit -m "feat(nfe): preenche infAdFisco, obsCont, obsFisco e procRef em infAdic"
```

---

## Task 8: Cadastro `organization_payment_terminals` + `pag`/`card` completos

Nível 6. Hoje o CNPJ do adquirente e o id do terminal seriam digitados a cada NFC-e. Um posto tem 4 terminais e emite
mil notas por dia — é o caso canônico de cadastro dedicado.

**Files:**

- Modify: `api/internal/repositories/org_entities.go`, `api/internal/services/org_entities.go`,
  `api/internal/api/v1/{dto,org_entities,router}.go`, `api/internal/app/app.go`,
  `api/internal/repositories/{roles,audit_logs}.go`, `api/internal/middleware/scopes.go`,
  `cdk/lib/dynamodb-stack.ts`
- Modify: `api/internal/services/nfes/builders_doc.go` (`buildPag`), `api/internal/services/nfes/emit.go`
- Create: `ui/src/lib/schemas/payment-terminals.ts`, `ui/src/components/payment-terminals/PaymentTerminalForm.tsx`,
  `ui/src/app/payment-terminals/{page,new/page,edit/page}.tsx`
- Test: `api/internal/services/nfes/builders_doc_test.go`, `api/internal/api/v1/org_entities_wiring_test.go`

**Interfaces:**

- Consumes: Receita R1 inteira.
- Produces:
    - `repositories.TablePaymentTerminals = "organization_payment_terminals"`, `SKPrefixPaymentTerminal = "TERMINAL_"`,
      `PaymentTerminalRepository`, `services.PaymentTerminalService`, `CacheScopePaymentTerminals`,
      `AuditResourcePaymentTerminal`.
    -
    `func (s *NfeService) resolvePaymentTerminals(ctx context.Context, orgPK string, payments []NfePaymentItem) (map[string]map[string]any, error)`
    — um `BatchGet` só, nunca `Get` dentro do laço.
    - `buildPag` ganha o parâmetro `terminals map[string]map[string]any`.

- [ ] **Step 1: Aplicar R1.1–R1.3 e R1.8–R1.10 com estes valores**

Tabela `organization_payment_terminals`, prefixo `TERMINAL_`, resource `organization_payment_terminals`, audit
`PAYMENT_TERMINAL`, cache scope `payment_terminals`, chave CDK `payment_terminals`.

- [ ] **Step 2: DTO (R1.4)**

```go
// PaymentTerminalBody é o body de POST/PUT /payment-terminals.
//
// Um terminal de captura (POS) tem CNPJ recebedor e identificador próprios,
// invariantes por maquininha. Ficam aqui para que a NFC-e só aponte o terminal.
type PaymentTerminalBody struct {
Name string `json:"name" validate:"required,min=2,max=120"`
// CNPJReceb — CNPJ do estabelecimento credenciado que recebe o pagamento.
CNPJReceb string `json:"cnpj_receb" validate:"required,cnpj"`
// IdTermPag — identificador do terminal, atribuído pela adquirente.
IdTermPag string `json:"id_term_pag" validate:"required,max=40"`
// CNPJPag/UFPag identificam o pagador institucional quando a operação de
// pagamento ocorre fora do estabelecimento emitente (detPag/CNPJPag).
CNPJPag *string `json:"cnpj_pag" validate:"omitempty,cnpj"`
UFPag   *string `json:"uf_pag" validate:"omitempty,uf"`
// TBand é a bandeira default (card/tBand). Sobrescrevível na emissão.
TBand *string `json:"t_band" validate:"omitempty,max=2"`
}
```

- [ ] **Step 3: Teste de builder que falha**

```go
func TestBuildPagComTerminal(t *testing.T) {
payments := []map[string]any{{
"payment_type": "03", "value": "50.00", "terminal_id": "TERMINAL_abc",
"card": map[string]any{"tp_integra": "1", "cnpj": "11111111111111", "c_aut": "999"},
}}
terminals := map[string]map[string]any{"TERMINAL_abc": {
"cnpj_receb": "22222222222222", "id_term_pag": "POS-01",
"cnpj_pag": "33333333333333", "uf_pag": "PI", "t_band": "01",
}}
pag := buildPag(payments, nil, terminals)
dp := pag["detPag"].([]map[string]any)[0]
if dp["CNPJPag"] != "33333333333333" || dp["UFPag"] != "PI" {
t.Fatalf("CNPJPag/UFPag ausentes: %v", dp)
}
card := dp["card"].(map[string]any)
if card["CNPJReceb"] != "22222222222222" || card["idTermPag"] != "POS-01" || card["tBand"] != "01" {
t.Fatalf("card incompleto: %v", card)
}
}
```

- [ ] **Step 4: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run TestBuildPagComTerminal
```

Esperado: FAIL — `buildPag` tem 2 parâmetros.

- [ ] **Step 5: Implementar**

`NfePaymentItem` ganha `TerminalID *string \`json:"terminal_id" validate:"omitempty"\`` e
`XPag *string \`json:"x_pag" validate:"omitempty,max=60"\``.

Em `buildPag`, dentro do laço, depois de `vPag`:

```go
        term := terminals[anyStr(p, "terminal_id", "")]
if v := anyStr(term, "cnpj_pag", ""); v != "" {
item["CNPJPag"] = v
// UFPag só é válido acompanhado de CNPJPag.
if uf := anyStr(term, "uf_pag", ""); uf != "" {
item["UFPag"] = uf
}
}
if v := anyStr(p, "x_pag", ""); v != "" {
item["xPag"] = v
}
```

e dentro do bloco `card`:

```go
            if v := anyStr(term, "cnpj_receb", ""); v != "" {
cardNode["CNPJReceb"] = v
}
if v := anyStr(term, "id_term_pag", ""); v != "" {
cardNode["idTermPag"] = v
}
if _, ok := cardNode["tBand"]; !ok {
if v := anyStr(term, "t_band", ""); v != "" {
cardNode["tBand"] = v
}
}
```

Ordem XSD de `card`: `tpIntegra, CNPJ, tBand, cAut, CNPJReceb, idTermPag`. Conferir nas duas tabelas de ordem.

- [ ] **Step 6: Resolver os terminais em `Emit` e `NfceService.Emit`**

```go
// resolvePaymentTerminals lê num BatchGet só todos os terminais citados pelos
// pagamentos. Um id inexistente é erro de request, não silêncio.
func (s *NfeService) resolvePaymentTerminals(ctx context.Context, orgPK string, payments []NfePaymentItem) (map[string]map[string]any, error) {
var ids []string
for _, p := range payments {
if p.TerminalID != nil && *p.TerminalID != "" {
ids = append(ids, *p.TerminalID)
}
}
if len(ids) == 0 {
return nil, nil
}
raw, err := s.paymentTerminalRepo.BatchGet(ctx, orgPK, ids)
if err != nil {
return nil, err
}
out := make(map[string]map[string]any, len(raw))
for id, item := range raw {
m, err := unmarshalToAny(item)
if err != nil {
return nil, problem.InternalServer("failed to decode payment terminal")
}
out[s.paymentTerminalRepo.SK(id)] = m
}
for _, id := range ids {
if _, ok := out[s.paymentTerminalRepo.SK(id)]; !ok {
return nil, problem.NotFound("terminal de pagamento não encontrado: " + id)
}
}
return out, nil
}
```

- [ ] **Step 7: UI (R1.11), docs (R1.12) e commit**

`DynamoDB-Tables.md` §37. `NfeEmitForm.tsx`/`NfceEmitForm`: quando `payment_type` é cartão (`03`/`04`), aparece o select
de terminal.

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx && cd ../cdk && npx tsc --noEmit
git add api ui cdk DOCS.md DynamoDB-Tables.md
git commit -m "feat(nfe): cadastro de terminais de pagamento e grupos CNPJPag/CNPJReceb"
```

---

## Task 9: CSRT — `infRespTec/idCSRT` + `hashCSRT`

Nível 1 + derivado. `hashCSRT = Base64(SHA1(CSRT + chave de acesso))`. O CSRT é segredo do responsável técnico:
**nunca** volta em resposta de API, e nunca vai para log.

**Files:**

- Create: `api/internal/services/nfes/csrt.go`, `api/internal/services/nfes/csrt_test.go`
- Modify: `api/internal/api/v1/dto.go` (`FiscalConfigBody`), `api/internal/services/fiscal_configs.go`
- Modify: `api/internal/services/nfes/builders_extra.go`, `api/internal/services/mdfes/builder.go`
- Modify: `DOCS.md`, `CONDUCT.md`

**Interfaces:**

- Consumes: nada.
- Produces:
    - `func HashCSRT(csrt, accessKey string) string`
    - `func BuildRespTec(tech TechData, idCSRT, csrt, accessKey string) map[string]any` — em `builders_extra.go`, usada
      por NF-e, NFC-e **e** MDF-e (o MDF-e importa do pacote `nfes` via um wrapper de uma linha, ou a função sobe para
      `services/` — decidir na implementação e justificar no comentário; a regra DRY exige uma só cópia).

- [ ] **Step 1: Teste com vetor conhecido**

```go
func TestHashCSRT(t *testing.T) {
// O valor esperado NÃO pode ser copiado da saída da implementação — isso
// tornaria o teste circular. Gere-o antes de escrever o código, com uma
// ferramenta independente:
//
//   printf '%s' 'G8063NG5H4YQ01M4L3AKG25OZ4A2GL43180906117473000150550010000000041000000047' \
//     | openssl dgst -sha1 -binary | openssl base64
//
// e cole o resultado em `want`, com este comentário preservado.
const csrt = "G8063NG5H4YQ01M4L3AKG25OZ4A2GL"
const chave = "43180906117473000150550010000000041000000047"
const want = "<saída do openssl acima>"
if got := HashCSRT(csrt, chave); got != want {
t.Fatalf("want %q, got %q", want, got)
}
}

func TestBuildRespTecSemCSRTOmiteOGrupo(t *testing.T) {
got := BuildRespTec(TechData{CNPJ: "1", Name: "n", Email: "e", Phone: "p"}, "", "", "chave")
if _, ok := got["idCSRT"]; ok {
t.Fatal("idCSRT não pode aparecer sem CSRT configurado")
}
if _, ok := got["hashCSRT"]; ok {
t.Fatal("hashCSRT não pode aparecer sem CSRT configurado")
}
}
```

Rodar o `openssl` do comentário **antes** de escrever `HashCSRT` e fixar `want` com o valor obtido. Se depois a
implementação divergir, o erro está nela — a concatenação é `CSRT + chave`, sem separador (NT 2018.005).

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/nfes -run 'TestHashCSRT|TestBuildRespTec'
```

Esperado: FAIL — `undefined: HashCSRT`.

- [ ] **Step 3: Implementar**

```go
package nfes

import (
	"crypto/sha1"
	"encoding/base64"
)

// csrt.go implementa o Código de Segurança do Responsável Técnico (NT 2018.005).
// Algumas UFs rejeitam a nota sem ele. O CSRT em si é segredo: entra por
// configuração, nunca sai por API, nunca vai para log — o que viaja no XML é só
// o hash, que não é reversível.

// HashCSRT devolve Base64(SHA1(CSRT + chave de acesso)), conforme a NT.
func HashCSRT(csrt, accessKey string) string {
	sum := sha1.Sum([]byte(csrt + accessKey))
	return base64.StdEncoding.EncodeToString(sum[:])
}
```

Em `builders_extra.go`:

```go
// BuildRespTec monta infRespTec. idCSRT/hashCSRT só aparecem quando a
// organização configurou um CSRT — o par é tudo-ou-nada no XSD.
func BuildRespTec(tech TechData, idCSRT, csrt, accessKey string) map[string]any {
node := map[string]any{
"CNPJ": tech.CNPJ, "xContato": tech.Name, "email": tech.Email, "fone": tech.Phone,
}
if idCSRT != "" && csrt != "" {
node["idCSRT"] = idCSRT
node["hashCSRT"] = HashCSRT(csrt, accessKey)
}
return node
}
```

- [ ] **Step 4: Configuração**

`fiscalConfigBase` ganha:

```go
    // CSRT do responsável técnico (NT 2018.005). Segredo: o serviço nunca o
// devolve numa resposta — ver FiscalConfigService.redactSecrets.
CsrtID *string `json:"csrt_id" validate:"omitempty,max=2,number"`
Csrt   *string `json:"csrt" validate:"omitempty,len=36"`
```

e o serviço de config passa a apagar `csrt` de toda resposta, no mesmo lugar onde `NfceConfigBody.ProdCsc` já é tratado.
Teste obrigatório:

```go
func TestGetFiscalConfigNuncaDevolveCSRT(t *testing.T) { /* GET após PUT com csrt; afirma ausência da chave */ }
```

- [ ] **Step 5: Rodar, documentar a regra em CONDUCT.md, commitar**

`CONDUCT.md` ganha uma linha em "Segredos": *CSRT e CSC nunca aparecem em resposta de API nem em log; só o hash derivado
entra no XML.*

```bash
cd api && go test ./...
git add api DOCS.md CONDUCT.md
git commit -m "feat(dfe): CSRT do responsável técnico em NF-e, NFC-e e MDF-e"
```

---

# Bloco 2 — MDF-e Fase A: rodoviário completo

Ordem 2 do spec §4. Metade é wiring de campo já cadastrado — maior salto de cobertura por esforço do plano inteiro.

## Task 10: Wiring de `cInt`, `capM3`, `xCpl` e `categCombVeic`

Nível 0 em tudo: os dados já existem, o builder simplesmente não os lê. `categCombVeic` é **derivado** da composição
(trator + número de reboques), nunca perguntado.

**Files:**

- Modify: `api/internal/services/mdfes/builder.go` (`buildRodo`, `buildEnderMDFe`)
- Modify: `api/internal/services/mdfes/emit.go` (`resolvedVehicle`)
- Modify: `api/internal/services/mdfes/builder_antt.go`
- Test: `api/internal/services/mdfes/builder_test.go`

**Interfaces:**

- Consumes: `resolvedVehicle` (já existe em `mdfes/emit.go`).
- Produces:
    - `resolvedVehicle` ganha os campos `CInt string` e `CapM3 string`.
    - `func categCombVeic(trailers int) string`

- [ ] **Step 1: Teste que falha**

```go
func TestCategCombVeic(t *testing.T) {
for trailers, want := range map[int]string{0: "02", 1: "04", 2: "06", 3: "07", 4: "07"} {
if got := categCombVeic(trailers); got != want {
t.Fatalf("%d reboques: want %q, got %q", trailers, want, got)
}
}
}

func TestBuildRodoIncluiCIntECapM3(t *testing.T) {
p := buildParams{vehicle: resolvedVehicle{Placa: "ABC1D23", Tara: "5000", TpRod: "06", TpCar: "02", UF: "PI", CInt: "T-01", CapM3: "90"}}
veic := p.buildRodo()["veicTracao"].(map[string]any)
if veic["cInt"] != "T-01" || veic["capM3"] != "90" {
t.Fatalf("cInt/capM3 ausentes: %v", veic)
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/mdfes -run 'TestCategCombVeic|TestBuildRodoInclui'
```

Esperado: FAIL — `undefined: categCombVeic`, campos ausentes.

- [ ] **Step 3: Implementar**

Em `builder_antt.go`:

```go
// categCombVeic (valePed/categCombVeic) classifica a combinação veicular pelo
// número de eixos, que aqui é derivado da composição: o trator sozinho é
// caminhão simples; cada reboque acrescenta uma categoria. Perguntar isso ao
// operador seria perguntar algo que o próprio manifesto já diz.
// Tabela: 02 caminhão, 04 caminhão + 1 reboque, 06 + 2, 07 + 3 ou mais.
func categCombVeic(trailers int) string {
switch trailers {
case 0:
return categCombCaminhao
case 1:
return categCombCaminhaoReboque
case 2:
return categCombCaminhaoDoisReboques
default:
return categCombCaminhaoTresReboques
}
}
```

com as quatro constantes no bloco `const` de `mdfes.go`.

Em `buildRodo`, junto de `RENAVAM`/`capKG` (ordem XSD de `veicTracao`:
`cInt, placa, RENAVAM, tara, capKG, capM3, prop, condutor, tpRod, tpCar, UF`):

```go
    if p.vehicle.CInt != "" {
veic["cInt"] = p.vehicle.CInt
}
if p.vehicle.CapM3 != "" {
veic["capM3"] = p.vehicle.CapM3
}
```

O mesmo para cada reboque, lendo `t.CInt`/`t.CapM3`.

Em `buildEnderMDFe`, depois de `xBairro`:

```go
    if cpl := anyStr(addr, "complement"); cpl != "" {
ender["xCpl"] = cpl
}
```

`resolveVehicle` (em `mdfes/emit.go`) passa a copiar `cint` e `cap_m3` do item do DynamoDB para o struct.

- [ ] **Step 4: Rodar, ver passar, commitar**

```bash
cd api && go test ./internal/services/mdfes/... -v
git add api DOCS.md
git commit -m "feat(mdfe): emite cInt, capM3, xCpl e deriva categCombVeic"
```

---

## Task 11: Cadastro `organization_toll_providers` + `infANTT/valePed`

Vale-pedágio é obrigatório no transporte rodoviário de carga (Lei 10.209). Nível 6: fornecedor e CNPJ pagador são
invariantes; por viagem só entram nº da compra e valor.

**Files:**

- R1 completa para `organization_toll_providers` / `TOLLPROVIDER_` / audit `TOLL_PROVIDER` / cache `toll_providers` /
  CDK `toll_providers`
- Modify: `api/internal/services/mdfes/builder_antt.go`, `api/internal/services/mdfes/emit.go`
- Test: `api/internal/services/mdfes/builder_antt_test.go`

**Interfaces:**

- Consumes: R1; `categCombVeic` (Task 10).
- Produces:
    - `TollProviderRepository`, `TollProviderService`, `TollProviderBody`.
    - `MdfeEmitBody` ganha `TollVouchers []MdfeTollBody` (`json:"toll_vouchers"`).
    - `func (p buildParams) buildValePed() map[string]any`

- [ ] **Step 1: Teste que falha**

```go
func TestBuildValePedComDispECategoria(t *testing.T) {
p := buildParams{
trailers: []resolvedVehicle{{Placa: "R1"}},
tolls: []resolvedToll{
{CNPJForn: "11111111111111", CNPJPg: "22222222222222", NCompra: "C-1", VValePed: "150.00", TpValePed: "01"},
},
}
got := p.buildValePed()
disp := got["disp"].([]map[string]any)
if len(disp) != 1 || disp[0]["CNPJForn"] != "11111111111111" || disp[0]["vValePed"] != "150.00" {
t.Fatalf("disp errado: %v", disp)
}
if got["categCombVeic"] != "04" {
t.Fatalf("categoria derivada errada: %v", got["categCombVeic"])
}
}

func TestBuildValePedSemValeDevolveNil(t *testing.T) {
if (buildParams{}).buildValePed() != nil {
t.Fatal("valePed sem vale tem que ser omitido")
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services/mdfes -run TestBuildValePed
```

Esperado: FAIL — `undefined: buildValePed`.

- [ ] **Step 3: Implementar**

```go
// buildValePed monta infANTT/valePed. Ordem XSD: disp (0..N), categCombVeic.
// O fornecedor e o CNPJ pagador vêm do cadastro de fornecedores de vale-pedágio;
// da viagem só saem número da compra e valor.
func (p buildParams) buildValePed() map[string]any {
if len(p.tolls) == 0 {
return nil
}
disp := make([]map[string]any, 0, len(p.tolls))
for _, t := range p.tolls {
item := map[string]any{"CNPJForn": t.CNPJForn}
if t.CNPJPg != "" {
item["CNPJPg"] = t.CNPJPg
}
if t.CPFPg != "" {
item["CPFPg"] = t.CPFPg
}
item["nCompra"] = t.NCompra
item["vValePed"] = t.VValePed
if t.TpValePed != "" {
item["tpValePed"] = t.TpValePed
}
disp = append(disp, item)
}
return map[string]any{"disp": disp, "categCombVeic": categCombVeic(len(p.trailers))}
}
```

`buildInfANTT` passa a incluir `valePed` entre `infCIOT` e `infContratante`.

DTO do cadastro:

```go
// TollProviderBody é o body de POST/PUT /toll-providers.
type TollProviderBody struct {
Name string `json:"name" validate:"required,min=2,max=120"`
// CNPJForn — CNPJ da fornecedora do vale-pedágio.
CNPJForn string `json:"cnpj_forn" validate:"required,cnpj"`
// Pagador do vale, quando não é o emitente. Um dos dois, nunca ambos.
CNPJPg *string `json:"cnpj_pg" validate:"omitempty,cnpj,excluded_with=CPFPg"`
CPFPg  *string `json:"cpf_pg" validate:"omitempty,cpf,excluded_with=CNPJPg"`
// TpValePed: 01 TAG, 02 cupom, 03 cartão.
TpValePed *string `json:"tp_vale_ped" validate:"omitempty,oneof=01 02 03"`
}
```

Request:

```go
// MdfeTollBody é um vale-pedágio da viagem. O fornecedor vem do cadastro;
// aqui só o que muda a cada viagem.
type MdfeTollBody struct {
TollProviderID string `json:"toll_provider_id" validate:"required"`
NCompra        string `json:"n_compra" validate:"required,max=20"`
VValePed       string `json:"v_vale_ped" validate:"required,money2"`
}
```

- [ ] **Step 4: Rodar, UI (R1.11), docs (§38), commit**

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx && cd ../cdk && npx tsc --noEmit
git add api ui cdk DOCS.md DynamoDB-Tables.md
git commit -m "feat(mdfe): cadastro de fornecedores de vale-pedágio e grupo valePed"
```

---

## Task 12: `infANTT/infContratante` + `infContrato`

Nível 5. Contratante do frete é papel de pessoa, não formulário: reusar `organization_persons`, que já tem
`roles`. Acrescentar o papel `freight_contractor` a `services.AllPersonRoles` e à tag
`personRolesValidation` no mesmo commit — há um teste (`TestPersonRolesTagMatchesAllPersonRoles`) que falha se
divergirem.

**Files:**

- Modify: `api/internal/services/person_roles.go`, `api/internal/api/v1/dto.go`
- Modify: `api/internal/services/mdfes/builder_antt.go`, `api/internal/services/mdfes/emit.go`
- Test: `api/internal/services/mdfes/builder_antt_test.go`, `api/internal/services/persons_test.go`

**Interfaces:**

- Consumes: `PersonRepository.BatchGet`… (não existe: usar `personRepo.Get`, um por contratante — o leiaute permite até
  10, e a emissão de MDF-e é rara comparada à de NF-e; documentar a escolha no comentário).
- Produces: `func (p buildParams) buildInfContratante() []map[string]any`; `resolvedContractor` struct.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildInfContratanteComContrato(t *testing.T) {
p := buildParams{contractors: []resolvedContractor{{
Name: "Transportadora X", CNPJ: "11111111111111",
ContractNumber: "CT-42", ContractValue: "9000.00",
}}}
got := p.buildInfContratante()
if got[0]["xNome"] != "Transportadora X" || got[0]["CNPJ"] != "11111111111111" {
t.Fatalf("contratante errado: %v", got[0])
}
ct := got[0]["infContrato"].(map[string]any)
if ct["NroContrato"] != "CT-42" || ct["vContratoGlobal"] != "9000.00" {
t.Fatalf("infContrato errado: %v", ct)
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `cd api && go test ./internal/services/mdfes -run TestBuildInfContratante`.
  Esperado: FAIL, `undefined: resolvedContractor`.

- [ ] **Step 3: Implementar**

```go
// buildInfContratante monta infANTT/infContratante (0..10). Ordem XSD:
// xNome, choice{CPF|CNPJ|idEstrangeiro}, infContrato.
func (p buildParams) buildInfContratante() []map[string]any {
if len(p.contractors) == 0 {
return nil
}
out := make([]map[string]any, 0, len(p.contractors))
for _, c := range p.contractors {
node := map[string]any{"xNome": c.Name}
switch {
case c.CNPJ != "":
node["CNPJ"] = c.CNPJ
case c.CPF != "":
node["CPF"] = c.CPF
default:
node["idEstrangeiro"] = c.Foreign
}
if c.ContractNumber != "" {
node["infContrato"] = map[string]any{
"NroContrato": c.ContractNumber, "vContratoGlobal": c.ContractValue,
}
}
out = append(out, node)
}
return out
}
```

`MdfeEmitBody` ganha:

```go
    // Contractors são os contratantes do frete (infANTT/infContratante, máx 10).
// person_id aponta para organization_persons; o contrato é da viagem.
Contractors []MdfeContractorBody `json:"contractors" validate:"omitempty,max=10,dive"`
```

- [ ] **Step 4: Rodar, docs, commit**

```bash
cd api && go test ./... && git add api DOCS.md && git commit -m "feat(mdfe): contratante do frete e contrato em infANTT"
```

---

## Task 13: `infANTT/infPag` — pagamento ao transportador autônomo

Obrigatório quando há contratante. Nível 5 + 6: dados bancários e PIX moram no cadastro do condutor/TAC
(`organization_persons`), os componentes do frete viram perfil reutilizável, e **as parcelas derivam do prazo
escolhido** — nunca digitadas uma a uma (é a mesma lógica de `ExpandPaymentTerm`, que já existe para NF-e: reusar).

**Files:**

- Modify: `api/internal/api/v1/dto.go` (`PersonObjectBody` ganha `bank`)
- Modify: `api/internal/services/mdfes/builder_antt.go`, `api/internal/services/mdfes/emit.go`
- Create: `api/internal/services/mdfes/pagamento.go`, `api/internal/services/mdfes/pagamento_test.go`
- Test: como acima

**Interfaces:**

- Consumes: `nfes.ExpandPaymentTerm` — **não importar `nfes` de `mdfes`**. A expansão de parcelas sobe para
  `services/installments.go` como `services.ExpandInstallments(total decimal.Decimal, count, intervalDays,
  firstDueDays int, from time.Time) []services.Installment`, e `nfes.ExpandPaymentTerm` passa a chamá-la. Um só
  algoritmo de parcelamento no repositório inteiro.
- Produces:
    - `func (p buildParams) buildInfPag() []map[string]any`
    - `services.ExpandInstallments` + `services.Installment{Number string; DueDate time.Time; Value decimal.Decimal}`

- [ ] **Step 1: Teste que falha (dois: o compartilhado e o do MDF-e)**

```go
// services/installments_test.go
func TestExpandInstallmentsUltimaAbsorveResiduo(t *testing.T) {
got := services.ExpandInstallments(decimal.RequireFromString("100.00"), 3, 30, 30,
time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
if len(got) != 3 {
t.Fatalf("want 3 parcelas, got %d", len(got))
}
sum := decimal.Zero
for _, i := range got {
sum = sum.Add(i.Value)
}
if !sum.Equal(decimal.RequireFromString("100.00")) {
t.Fatalf("soma %s != total", sum)
}
if got[2].Value.String() != "33.34" {
t.Fatalf("resíduo tem que cair na última: %s", got[2].Value)
}
}
```

```go
// mdfes/pagamento_test.go
func TestBuildInfPagComPrazoEPix(t *testing.T) {
p := buildParams{payments: []resolvedMdfePayment{{
Name: "João", CPF: "11144477735", VContrato: "3000.00", IndPag: "1", VAdiant: "0.00",
Components: []mdfePayComponent{{TpComp: "01", VComp: "3000.00"}},
Installments: 2, IntervalDays: 15, FirstDueDays: 15,
PixKey: "joao@pix.com",
Issued: time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
}}}
got := p.buildInfPag()[0]
if got["xNome"] != "João" || got["CPF"] != "11144477735" {
t.Fatalf("beneficiário errado: %v", got)
}
if len(got["infPrazo"].([]map[string]any)) != 2 {
t.Fatalf("parcelas não derivadas: %v", got["infPrazo"])
}
if got["infBanc"].(map[string]any)["PIX"] != "joao@pix.com" {
t.Fatalf("PIX ausente: %v", got["infBanc"])
}
}
```

- [ ] **Step 2: Rodar e ver falhar**

```bash
cd api && go test ./internal/services -run TestExpandInstallments ./internal/services/mdfes -run TestBuildInfPag
```

Esperado: FAIL nos dois.

- [ ] **Step 3: Implementar**

`services/installments.go` recebe o algoritmo que hoje está dentro de `nfes.ExpandPaymentTerm` (mover, não copiar), e
`ExpandPaymentTerm` vira uma casca que traduz `PaymentTermBody` → `ExpandInstallments`.

`buildInfPag` (ordem XSD: `xNome, choice{CPF|CNPJ|idEstrangeiro}, Comp, vContrato, indAltoDesemp, indPag, vAdiant,
tpAntecip, infPrazo, infBanc`):

```go
func (p buildParams) buildInfPag() []map[string]any {
if len(p.payments) == 0 {
return nil
}
out := make([]map[string]any, 0, len(p.payments))
for _, pay := range p.payments {
node := map[string]any{"xNome": pay.Name}
if pay.CNPJ != "" {
node["CNPJ"] = pay.CNPJ
} else {
node["CPF"] = pay.CPF
}
comps := make([]map[string]any, 0, len(pay.Components))
for _, c := range pay.Components {
comp := map[string]any{"tpComp": c.TpComp, "vComp": c.VComp}
if c.XComp != "" {
comp["xComp"] = c.XComp
}
comps = append(comps, comp)
}
node["Comp"] = comps
node["vContrato"] = pay.VContrato
if pay.IndAltoDesemp != "" {
node["indAltoDesemp"] = pay.IndAltoDesemp
}
node["indPag"] = pay.IndPag
if pay.IndPag == indPagPrazo {
node["vAdiant"] = pay.VAdiant
// As parcelas são derivadas do prazo, não digitadas: mesma regra da
// condição de pagamento da NF-e (services.ExpandInstallments).
rest := d(pay.VContrato).Sub(d(pay.VAdiant))
prazos := make([]map[string]any, 0, pay.Installments)
for _, inst := range services.ExpandInstallments(rest, pay.Installments, pay.IntervalDays, pay.FirstDueDays, pay.Issued) {
prazos = append(prazos, map[string]any{
"nParcela": inst.Number,
"dVenc":    inst.DueDate.Format("2006-01-02"),
"vParcela": inst.Value.StringFixed(2),
})
}
node["infPrazo"] = prazos
}
if pay.TpAntecip != "" {
node["tpAntecip"] = pay.TpAntecip
}
if banc := buildInfBanc(pay); banc != nil {
node["infBanc"] = banc
}
out = append(out, node)
}
return out
}

// buildInfBanc: o XSD é choice — ou PIX, ou (codBanco + codAgencia), ou CNPJIPEF.
func buildInfBanc(pay resolvedMdfePayment) map[string]any {
switch {
case pay.PixKey != "":
return map[string]any{"PIX": pay.PixKey}
case pay.BankCode != "":
return map[string]any{"codBanco": pay.BankCode, "codAgencia": pay.BranchCode}
case pay.CNPJIPEF != "":
return map[string]any{"CNPJIPEF": pay.CNPJIPEF}
}
return nil
}
```

`PersonObjectBody` ganha o bloco bancário:

```go
    // Bank são os dados de recebimento do condutor/TAC (MDF-e infANTT/infBanc).
// Ficam na pessoa porque são invariantes dela, não da viagem.
Bank *PersonBankBody `json:"bank" validate:"omitempty"`
```

```go
// PersonBankBody é o choice de infBanc: PIX, ou banco+agência, ou CNPJ do IPEF.
type PersonBankBody struct {
PixKey     *string `json:"pix_key" validate:"omitempty,max=77"`
BankCode   *string `json:"bank_code" validate:"omitempty,len=3,number"`
BranchCode *string `json:"branch_code" validate:"omitempty,max=10,number"`
CNPJIPEF   *string `json:"cnpj_ipef" validate:"omitempty,cnpj"`
}
```

- [ ] **Step 4: Regra de negócio: contratante exige infPag**

Em `MdfeService.Emit`, junto das outras validações:

```go
    if len(req.Contractors) > 0 && len(req.Payments) == 0 {
return nil, problem.BadRequest("MDF-e com contratante exige o grupo de pagamento (infPag)")
}
```

com teste em `mdfes_test.go`.

- [ ] **Step 5: Rodar tudo (o refactor de parcelas toca NF-e), UI, docs, commit**

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx
git add api ui DOCS.md DynamoDB-Tables.md
git commit -m "feat(mdfe): pagamento ao transportador autônomo com parcelas derivadas"
```

---

## Task 14: `lacRodo`, `infMDFe/lacres` e `codAgPorto`

Nível 7 puro. Três campos, um commit.

**Files:**

- Modify: `api/internal/services/mdfes/builder.go`, `api/internal/services/mdfes/builder_antt.go`,
  `api/internal/services/mdfes/emit.go`, `api/internal/api/v1/dto.go`
- Test: `api/internal/services/mdfes/builder_test.go`

**Interfaces:**

- Consumes: `buildRodo`, `BuildMDFe`.
- Produces: `MdfeEmitBody` ganha `Seals []string`, `RodoSeals []string`, `PortAgentCode *string`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildMDFeLacres(t *testing.T) {
p := baseBuildParams(t)
p.seals = []string{"L1", "L2"}
p.rodoSeals = []string{"R1"}
p.portAgentCode = "AG-9"
inf := BuildMDFe(p)["MDFe"].(map[string]any)["infMDFe"].(map[string]any)
if len(inf["lacres"].([]map[string]any)) != 2 {
t.Fatalf("lacres da carga ausentes: %v", inf["lacres"])
}
rodo := inf["infModal"].(map[string]any)["rodo"].(map[string]any)
if len(rodo["lacRodo"].([]map[string]any)) != 1 {
t.Fatalf("lacRodo ausente: %v", rodo)
}
if rodo["codAgPorto"] != "AG-9" {
t.Fatalf("codAgPorto ausente: %v", rodo)
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/mdfes -run TestBuildMDFeLacres`.

- [ ] **Step 3: Implementar**

Em `BuildMDFe`, depois de `tot` (ordem XSD de `infMDFe`: `ide, emit, infModal, infDoc, seg, prodPred, tot, lacres,
autXML, infAdic, infRespTec, infSolicNFF`):

```go
    if len(p.seals) > 0 {
infMDFe["lacres"] = sealNodes(p.seals)
}
```

Em `buildRodo`, depois de `veicReboque` (ordem: `infANTT, veicTracao, veicReboque, codAgPorto, lacRodo`):

```go
    if p.portAgentCode != "" {
rodo["codAgPorto"] = p.portAgentCode
}
if len(p.rodoSeals) > 0 {
rodo["lacRodo"] = sealNodes(p.rodoSeals)
}
```

```go
// sealNodes converte números de lacre no nó do XSD. Uma função só: lacres e
// lacRodo têm a mesma forma (nLacre), e duplicar isso seria duplicar por acaso.
func sealNodes(numbers []string) []map[string]any {
out := make([]map[string]any, 0, len(numbers))
for _, n := range numbers {
out = append(out, map[string]any{"nLacre": n})
}
return out
}
```

- [ ] **Step 4: Rodar, commitar**

```bash
cd api && go test ./internal/services/mdfes/...
git add api DOCS.md && git commit -m "feat(mdfe): lacres da carga, lacres rodoviários e código do agente portuário"
```

---

# Bloco 3 — MDF-e Fase B: `infDoc` (10 de 78 tags hoje)

A maior lacuna do MDF-e. Quase tudo aqui é **derivado do XML da NF-e referenciada**, que o sistema já lê e parseia em
`mdfes/xmlparse.go`.

## Task 15: `SegCodBarra` e `indReentrega` derivados da NF-e

Nível 0. `SegCodBarra` é o código de barras da NF-e (a própria chave); `indReentrega` marca reentrega.

**Files:**

- Modify: `api/internal/services/mdfes/xmlparse.go`, `api/internal/services/mdfes/builder_infdoc.go`,
  `api/internal/services/mdfes/emit.go`
- Test: `api/internal/services/mdfes/builder_infdoc_test.go`

**Interfaces:**

- Consumes: `resolvedCargo`, `parsedDoc` (em `xmlparse.go`).
- Produces: cada documento de `infMunDescarga` passa a carregar `SegCodBarra` e, quando marcado, `indReentrega`.
  `MdfeEmitBody` ganha `RedeliveryKeys []string` (chaves que são reentrega).

- [ ] **Step 1: Teste que falha**

```go
func TestBuildInfDocSegCodBarra(t *testing.T) {
p := buildParams{cargo: &resolvedCargo{descarga: []descargaGroup{{
mun:     municipality{IBGECode: "2211001", City: "Teresina"},
nfeKeys: []string{"22260811647612000197550010000000011100000015"},
}}}, redelivery: map[string]bool{"22260811647612000197550010000000011100000015": true}}
nfe := p.buildInfDoc()["infMunDescarga"].([]map[string]any)[0]["infNFe"].([]map[string]any)[0]
if nfe["SegCodBarra"] != "22260811647612000197550010000000011100000015" {
t.Fatalf("SegCodBarra ausente: %v", nfe)
}
if nfe["indReentrega"] != indReentregaSim {
t.Fatalf("indReentrega ausente: %v", nfe)
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/mdfes -run TestBuildInfDocSegCodBarra`.

- [ ] **Step 3: Implementar**

`const indReentregaSim = "1"` em `mdfes.go`. Em `buildInfDoc`, no laço de `nfeKeys`:

```go
            for _, k := range g.nfeKeys {
// SegCodBarra é o código de barras da NF-e — que é a própria
// chave. Perguntá-lo ao operador seria pedir de volta o dado
// que ele acabou de referenciar.
node := map[string]any{"chNFe": k, "SegCodBarra": k}
if p.redelivery[k] {
node["indReentrega"] = indReentregaSim
}
nfes = append(nfes, node)
}
```

- [ ] **Step 4: Rodar, commitar** —
  `git commit -m "feat(mdfe): deriva SegCodBarra e marca reentrega em infDoc"`

---

## Task 16: `peri` — produto perigoso derivado dos itens da NF-e

**O exemplo canônico da régua.** A classificação ONU é nível 4 (produto); ao referenciar a NF-e, o sistema encontra os
itens perigosos e monta `peri` sozinho. Digitar isso por viagem é o que o plano existe para impedir.

**Files:**

- Create: `api/internal/services/mdfes/peri.go`, `api/internal/services/mdfes/peri_test.go`
- Modify: `api/internal/api/v1/dto.go` (`ProductBody`), `api/internal/services/mdfes/{xmlparse,emit,builder_infdoc}.go`
- Test: como acima

**Interfaces:**

- Consumes: `repositories.ProductRepository` (novo campo no `MdfeService`); `parsedDoc.Items` — **não existe**:
  `xmlparse.go` passa a extrair `[]parsedItem{CProd, XProd, QCom, UCom}` de cada `det` da NF-e referenciada.
- Produces:
    - `func resolvePeri(items []parsedItem, byCode map[string]map[string]any) []map[string]any`
    - `ProductBody` ganha o bloco `peri_*`.

- [ ] **Step 1: Teste que falha**

```go
func TestResolvePeriSomaQuantidadesPorONU(t *testing.T) {
items := []parsedItem{
{CProd: "A", QCom: "10.0000", UCom: "L"},
{CProd: "B", QCom: "5.0000", UCom: "L"},
{CProd: "C", QCom: "1.0000", UCom: "UN"}, // não perigoso
}
byCode := map[string]map[string]any{
"A": {"peri_n_onu": "1203", "peri_x_nome_ae": "GASOLINA", "peri_x_cla_risco": "3", "peri_gr_emb": "II", "peri_q_vol_tipo": "TAMBOR"},
"B": {"peri_n_onu": "1203", "peri_x_nome_ae": "GASOLINA", "peri_x_cla_risco": "3", "peri_gr_emb": "II", "peri_q_vol_tipo": "TAMBOR"},
"C": {},
}
got := resolvePeri(items, byCode)
if len(got) != 1 {
t.Fatalf("itens do mesmo ONU têm que virar um grupo só: %v", got)
}
if got[0]["nONU"] != "1203" || got[0]["qTotProd"] != "15.0000" {
t.Fatalf("agrupamento errado: %v", got[0])
}
}

func TestResolvePeriSemProdutoPerigosoDevolveNil(t *testing.T) {
if resolvePeri([]parsedItem{{CProd: "C"}}, map[string]map[string]any{"C": {}}) != nil {
t.Fatal("nota sem produto perigoso não pode gerar peri")
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/mdfes -run TestResolvePeri`.

- [ ] **Step 3: Implementar**

```go
package mdfes

// peri.go deriva o grupo infDoc/.../peri (produto perigoso) a partir dos itens
// da NF-e referenciada, cruzando o código do produto com o cadastro. O operador
// classifica o produto uma vez, na ONU; nunca redigita nada por viagem.

// resolvePeri agrupa por número ONU e soma as quantidades. Ordem XSD:
// nONU, xNomeAE, xClaRisco, grEmb, qTotProd, qVolTipo.
func resolvePeri(items []parsedItem, byCode map[string]map[string]any) []map[string]any {
	type acc struct {
		node  map[string]any
		total decimal.Decimal
	}
	order := make([]string, 0, len(items))
	groups := make(map[string]*acc, len(items))

	for _, it := range items {
		prod := byCode[it.CProd]
		onu := anyStr(prod, "peri_n_onu")
		if onu == "" {
			continue
		}
		g, ok := groups[onu]
		if !ok {
			g = &acc{node: map[string]any{
				"nONU":      onu,
				"xNomeAE":   anyStr(prod, "peri_x_nome_ae"),
				"xClaRisco": anyStr(prod, "peri_x_cla_risco"),
				"qVolTipo":  anyStr(prod, "peri_q_vol_tipo"),
			}}
			if gr := anyStr(prod, "peri_gr_emb"); gr != "" {
				g.node["grEmb"] = gr
			}
			groups[onu] = g
			order = append(order, onu)
		}
		if q, err := decimal.NewFromString(it.QCom); err == nil {
			g.total = g.total.Add(q)
		}
	}
	if len(order) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(order))
	for _, onu := range order {
		g := groups[onu]
		g.node["qTotProd"] = g.total.StringFixed(4)
		out = append(out, g.node)
	}
	return out
}
```

`ProductBody` ganha:

```go
    // Classificação de produto perigoso (MDF-e peri). Cadastrada uma vez; o
// MDF-e a encontra sozinho ao referenciar a NF-e que contém o item.
PeriNOnu      *string `json:"peri_n_onu" validate:"omitempty,max=4,number"`
PeriXNomeAE   *string `json:"peri_x_nome_ae" validate:"omitempty,max=150"`
PeriXClaRisco *string `json:"peri_x_cla_risco" validate:"omitempty,max=40"`
PeriGrEmb     *string `json:"peri_gr_emb" validate:"omitempty,max=6"`
PeriQVolTipo  *string `json:"peri_q_vol_tipo" validate:"omitempty,max=60"`
```

`buildInfDoc` acrescenta `peri` ao nó `infNFe` do documento correspondente (ordem:
`chNFe, SegCodBarra, indReentrega, infUnidTransp, peri`).

- [ ] **Step 4: Rodar, docs, commit**

`DynamoDB-Tables.md` §4 ganha as cinco linhas `peri_*`.

```bash
cd api && go test ./internal/services/mdfes/... ./internal/api/v1/...
git add api ui DOCS.md DynamoDB-Tables.md
git commit -m "feat(mdfe): deriva produto perigoso (peri) do cadastro de produtos"
```

---

## Task 17: Cadastro `organization_cargo_units` + `infUnidTransp`/`infUnidCarga` + rateio

Nível 6. Contêiner, carreta e vagão recorrem entre viagens e têm identidade própria. O rateio (`qtdRat`) é **calculado**
a partir dos pesos dos documentos, não digitado.

**Files:**

- R1 completa para `organization_cargo_units` / `CARGOUNIT_` / audit `CARGO_UNIT` / cache `cargo_units` / CDK
  `cargo_units`
- Modify: `api/internal/services/mdfes/builder_infdoc.go`, `api/internal/services/mdfes/emit.go`
- Create: `api/internal/services/mdfes/rateio.go`, `api/internal/services/mdfes/rateio_test.go`

**Interfaces:**

- Consumes: R1; `resolvedCargo`.
- Produces:
    - `CargoUnitRepository`, `CargoUnitService`, `CargoUnitBody`.
    - `func rateCargo(docWeights map[string]decimal.Decimal, keys []string) map[string]string` — devolve `qtdRat`
      por chave, com a **última** chave absorvendo o resíduo (mesma regra das parcelas).
    - `func (p buildParams) buildUnidTransp(docKey string) []map[string]any`

- [ ] **Step 1: Teste que falha**

```go
func TestRateCargoSomaCem(t *testing.T) {
w := map[string]decimal.Decimal{
"A": decimal.RequireFromString("100"),
"B": decimal.RequireFromString("100"),
"C": decimal.RequireFromString("100"),
}
got := rateCargo(w, []string{"A", "B", "C"})
sum := decimal.Zero
for _, v := range got {
sum = sum.Add(decimal.RequireFromString(v))
}
if !sum.Equal(decimal.RequireFromString("100.00")) {
t.Fatalf("rateio tem que somar 100.00, deu %s (%v)", sum, got)
}
if got["C"] != "33.34" {
t.Fatalf("resíduo tem que cair na última chave: %v", got)
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/mdfes -run TestRateCargo`.

- [ ] **Step 3: Implementar**

```go
// rateCargo distribui 100% da unidade de carga entre os documentos que ela
// transporta, proporcionalmente ao peso. A última chave absorve o resíduo de
// arredondamento — sem isso o somatório fecha em 99.99 e a SEFAZ rejeita.
func rateCargo(weights map[string]decimal.Decimal, keys []string) map[string]string {
total := decimal.Zero
for _, k := range keys {
total = total.Add(weights[k])
}
out := make(map[string]string, len(keys))
if total.IsZero() {
return out
}
hundred := decimal.NewFromInt(100)
acc := decimal.Zero
for i, k := range keys {
if i == len(keys)-1 {
out[k] = hundred.Sub(acc).StringFixed(2)
continue
}
part := weights[k].Mul(hundred).Div(total).RoundBank(2)
acc = acc.Add(part)
out[k] = part.StringFixed(2)
}
return out
}
```

`CargoUnitBody`:

```go
// CargoUnitBody é o body de POST/PUT /cargo-units.
//
// Uma unidade de transporte (carreta, vagão) ou de carga (contêiner, pallet)
// recorre entre viagens e tem identificação própria.
type CargoUnitBody struct {
Name string `json:"name" validate:"required,min=2,max=120"`
// Kind separa infUnidTransp de infUnidCarga — a estrutura é a mesma, o nó não.
Kind string `json:"kind" validate:"required,oneof=transport cargo"`
// TpUnidTransp: 1 rodoviário tração, 2 rodoviário reboque, 3 navio, 4 balsa,
// 5 aeronave, 6 vagão, 7 outros. TpUnidCarga: 1 contêiner, 2 ULD, 3 pallet, 4 outros.
TpUnid string `json:"tp_unid" validate:"required,oneof=1 2 3 4 5 6 7"`
// IdUnid é a identificação (placa, número do contêiner, número do vagão).
IdUnid string `json:"id_unid" validate:"required,max=20"`
}
```

`buildUnidTransp` monta `infUnidTransp{tpUnidTransp, idUnidTransp, lacUnidTransp[], infUnidCarga[], qtdRat}` com
`qtdRat` vindo de `rateCargo`, e `infUnidCarga{tpUnidCarga, idUnidCarga, lacUnidCarga[], qtdRat}` aninhado.

- [ ] **Step 4: Rodar, UI, docs (§39), commit**

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx && cd ../cdk && npx tsc --noEmit
git add api ui cdk DOCS.md DynamoDB-Tables.md
git commit -m "feat(mdfe): unidades de transporte e carga com rateio calculado"
```

---

## Task 18: `infEntregaParcial`, `indPrestacaoParcial`, `infNFePrestParcial` e `infMDFeTransp`

Nível 7 (+0 para `qMDFe`, que é contagem).

**Files:**

- Modify: `api/internal/services/mdfes/{builder,builder_infdoc,emit}.go`, `api/internal/api/v1/dto.go`
- Test: `api/internal/services/mdfes/builder_infdoc_test.go`

**Interfaces:**

- Consumes: `buildInfDoc`, `buildTot`.
- Produces: `MdfeEmitBody` ganha `PartialDeliveries []MdfePartialDeliveryBody` e `TransportedMdfes []string`;
  `buildTot` passa a emitir `qMDFe`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildTotQMDFe(t *testing.T) {
p := baseBuildParams(t)
p.transportedMdfes = []string{"chave1", "chave2"}
if got := p.buildTot()["qMDFe"]; got != "2" {
t.Fatalf("qMDFe é contagem derivada: got %v", got)
}
}

func TestBuildInfDocEntregaParcial(t *testing.T) {
p := baseBuildParams(t)
p.partial = map[string]partialDelivery{"chNFe1": {QtdTotal: "10.0000", QtdParcial: "4.0000"}}
// … afirma infEntregaParcial{qtdTotal,qtdParcial} no nó da chave
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar** — `infMDFeTransp` reusa exatamente a mesma montagem de `infNFe`/`infCTe`
  (`chMDFe, indReentrega, infUnidTransp, peri`), então extrair uma função
  `docNode(key string, p buildParams, tag string) map[string]any` e chamar as três vezes, em vez de três laços.

- [ ] **Step 4: Rodar, commit** —
  `git commit -m "feat(mdfe): entrega parcial, prestação parcial e MDF-e transportado"`

---

# Bloco 4 — NF-e Fase B: regimes tributários faltantes

Tudo nível 3 (`tax_profiles`). Nada aqui pode aparecer no request de emissão. Cada CST é isolado e testável — são
tarefas pequenas de propósito, para poderem ser revisadas em separado.

## Task 19: `ICMSPart` (CST 10/90 com partilha)

**Files:** Modify `api/internal/services/nfes/builders_tax.go`, `api/internal/api/v1/dto.go`; Test
`api/internal/services/nfes/builders_tax_test.go`

**Interfaces:**

- Consumes: `buildICMSNormal`.
- Produces: `TaxFieldsBody` ganha `IcmsPartPBCOp *string` e `IcmsPartUFST *string`; `buildICMSNormal` passa a devolver
  `ICMSPart` quando ambos estão presentes e o CST é 10 ou 90.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildICMSPart(t *testing.T) {
cfg := map[string]any{
"icms_mod_bc": "3", "icms_st_aliq": "18.00", "icms_st_mva": "40.00",
"icms_part_p_bc_op": "60.00", "icms_part_uf_st": "SP",
}
got := buildICMSNormal("0", "10", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
node, ok := got["ICMSPart"].(map[string]any)
if !ok {
t.Fatalf("esperava ICMSPart, veio %v", got)
}
if node["pBCOp"] != "60.00" || node["UFST"] != "SP" || node["CST"] != "10" {
t.Fatalf("ICMSPart errado: %v", node)
}
if _, ok := got["ICMS10"]; ok {
t.Fatal("ICMSPart substitui ICMS10, não convive")
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/nfes -run TestBuildICMSPart`.

- [ ] **Step 3: Implementar**

No início do `switch cst`, antes dos cases:

```go
    // ICMSPart substitui ICMS10/ICMS90 quando há partilha do ICMS entre a UF de
// origem e a de destino (operação interestadual a não contribuinte). O par
// pBCOp+UFST é o que distingue os dois casos — não há CST próprio.
if pBCOp := cfgStrPtr(cfg, "icms_part_p_bc_op"); pBCOp != nil && icmsPartCSTs[cst] {
if ufST := cfgStrPtr(cfg, "icms_part_uf_st"); ufST != nil {
node := map[string]any{
"orig": origin, "CST": cst, "modBC": modBC, "vBC": vBC,
"pRedBC": pRedBC, "pICMS": pICMS, "vICMS": vICMS,
"modBCST": modBCST, "pMVAST": pMVAST, "pRedBCST": pRedBCST,
"vBCST": vBCST, "pICMSST": pICMSST, "vICMSST": vICMSST,
"pBCOp": *pBCOp, "UFST": *ufST,
}
return map[string]any{"ICMSPart": node}
}
}
```

com `var icmsPartCSTs = map[string]bool{"10": true, "90": true}` junto de `icmsCSTDifalEligible`.

- [ ] **Step 4: Rodar, payload de integração, commit**

```bash
cd api && go test ./internal/services/nfes -run TestBuildICMS -v
git commit -am "feat(nfe): grupo ICMSPart para partilha interestadual"
```

---

## Task 20: `ICMSST` (CST 41 — repasse de ST já retido)

Hoje CST 41 cai em `ICMS40`, que é isenção — errado para repasse interestadual.

**Files:** como a Task 19.

**Interfaces:**

- Produces: `TaxFieldsBody` ganha `IcmsVBcStDest *string` e `IcmsVIcmsStDest *string`; CST 41 com o par presente passa a
  produzir `ICMSST`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildICMSSTRepasse(t *testing.T) {
cfg := map[string]any{
"icms_v_bc_st_ret": "200.00", "icms_v_icms_st_ret": "36.00",
"icms_v_bc_st_dest": "150.00", "icms_v_icms_st_dest": "27.00",
}
got := buildICMSNormal("0", "41", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
node, ok := got["ICMSST"].(map[string]any)
if !ok {
t.Fatalf("CST 41 com ST retida tem que virar ICMSST, veio %v", got)
}
if node["vBCSTDest"] != "150.00" || node["vICMSSTDest"] != "27.00" {
t.Fatalf("ICMSST errado: %v", node)
}
}

// Sem os valores de ST, 41 continua sendo não tributada (ICMS40).
func TestBuildICMS41SemSTContinuaICMS40(t *testing.T) {
got := buildICMSNormal("0", "41", decimal.RequireFromString("100.00"), map[string]any{}, "12.00", "0.00", decimal.NewFromInt(1))
if _, ok := got["ICMS40"]; !ok {
t.Fatalf("esperava ICMS40, veio %v", got)
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar** — no `case "40", "41", "50"`, antes do `return` atual:

```go
    case "40", "41", "50":
// ICMSST (CST 41) é o repasse, na operação interestadual, do ICMS-ST já
// retido antes. Sem os valores retidos, 41 é apenas não tributada.
if cst == "41" {
if vBCSTRet := cfgStrPtr(cfg, "icms_v_bc_st_ret"); vBCSTRet != nil {
node := map[string]any{
"orig": origin, "CST": cst, "vBCSTRet": *vBCSTRet,
"vICMSSTRet": cfgStr(cfg, "icms_v_icms_st_ret", "0.00"),
}
if v := cfgStrPtr(cfg, "icms_v_bc_st_dest"); v != nil {
node["vBCSTDest"] = *v
node["vICMSSTDest"] = cfgStr(cfg, "icms_v_icms_st_dest", "0.00")
}
if v := cfgStrPtr(cfg, "icms_fcp_v_bc_st_ret"); v != nil {
node["vBCFCPSTRet"] = *v
node["pFCPSTRet"] = cfgStr(cfg, "icms_fcp_st_ret_aliq", "0.00")
node["vFCPSTRet"] = q2(d(*v).Mul(d(cfgStr(cfg, "icms_fcp_st_ret_aliq", "0"))).Div(decimal.NewFromInt(100)).RoundBank(2))
}
return map[string]any{"ICMSST": node}
}
}
return map[string]any{"ICMS40": addDeson(map[string]any{"orig": origin, "CST": cst})}
```

- [ ] **Step 4: Rodar, commit** — `git commit -am "feat(nfe): grupo ICMSST para repasse de ST retida (CST 41)"`

---

## Task 21: ICMS efetivo (`vBCEfet`/`pICMSEfet`/`vICMSEfet`/`pRedBCEfet`) em ICMS60, ICMSST e ICMSSN500

Exigido por MG, RS e outras na revenda de mercadoria com ST. Um grupo, três lugares — **uma** função.

**Files:** Modify `api/internal/services/nfes/builders_tax.go`, `api/internal/api/v1/dto.go`; Test idem.

**Interfaces:**

- Produces:
    - `TaxFieldsBody` ganha `IcmsPRedBcEfet *string` e `IcmsPIcmsEfet *string`.
    - `func addICMSEfetivo(node map[string]any, vProd decimal.Decimal, cfg map[string]any)` — chamada de
      `ICMS60`, `ICMSST` e `ICMSSN500`.

- [ ] **Step 1: Teste que falha**

```go
func TestAddICMSEfetivoCalculaBaseEValor(t *testing.T) {
node := map[string]any{}
addICMSEfetivo(node, decimal.RequireFromString("100.00"),
map[string]any{"icms_p_red_bc_efet": "20.00", "icms_p_icms_efet": "18.00"})
if node["pRedBCEfet"] != "20.00" || node["vBCEfet"] != "80.00" ||
node["pICMSEfet"] != "18.00" || node["vICMSEfet"] != "14.40" {
t.Fatalf("efetivo errado: %v", node)
}
}

func TestAddICMSEfetivoAusenteNaoPoluiONo(t *testing.T) {
node := map[string]any{"CST": "60"}
addICMSEfetivo(node, decimal.RequireFromString("100.00"), map[string]any{})
if len(node) != 1 {
t.Fatalf("sem configuração, nada pode ser acrescentado: %v", node)
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar**

```go
// addICMSEfetivo acrescenta o grupo do ICMS efetivo (vBCEfet, pRedBCEfet,
// pICMSEfet, vICMSEfet), exigido por algumas UFs na revenda de mercadoria com
// ST retida. Vale para ICMS60, ICMSST e ICMSSN500 — a mesma quádrupla, com o
// mesmo cálculo, nos três; por isso uma função e não três cópias.
func addICMSEfetivo(node map[string]any, vProd decimal.Decimal, cfg map[string]any) {
pICMSEfet := cfgStrPtr(cfg, "icms_p_icms_efet")
if pICMSEfet == nil || *pICMSEfet == "" {
return
}
pRed := cfgStr(cfg, "icms_p_red_bc_efet", "0.00")
vBCEfet := vProd.Mul(decimal.NewFromInt(1).Sub(d(pRed).Div(decimal.NewFromInt(100)))).RoundBank(2)
if pRed != "0.00" {
node["pRedBCEfet"] = pRed
}
node["vBCEfet"] = q2(vBCEfet)
node["pICMSEfet"] = *pICMSEfet
node["vICMSEfet"] = q2(vBCEfet.Mul(d(*pICMSEfet)).Div(decimal.NewFromInt(100)).RoundBank(2))
}
```

Chamar no fim do `case "60"`, no ramo `ICMSST` da Task 20, e no ramo CSOSN 500 de `buildICMSSN`. Ordem XSD em ICMS60:
`…vFCPSTRet, pRedBCEfet, vBCEfet, pICMSEfet, vICMSEfet`.

- [ ] **Step 4: Rodar, commit** — `git commit -am "feat(nfe): ICMS efetivo em ICMS60, ICMSST e ICMSSN500"`

---

## Task 22: `vICMSSTDeson`/`motDesICMSST` em ICMS10/70/90 e `pFCPDif`/`vFCPDif` em ICMS51/90

Dois grupos pequenos, um commit — mudam o mesmo `switch` e compartilham o padrão de "closure que acrescenta ao nó".

**Interfaces:**

- Produces: `TaxFieldsBody` ganha `IcmsMotDesSt *string` e `IcmsPFcpDif *string`;
  `addSTDeson(node)` e `addFCPDif(node)` como closures irmãs de `addDeson`/`addFCP`, dentro de `buildICMSNormal`.

- [ ] **Step 1: Testes que falham**

```go
func TestICMS70ComSTDesonerada(t *testing.T) {
cfg := map[string]any{"icms_st_aliq": "18.00", "icms_mot_des_st": "9"}
got := buildICMSNormal("0", "70", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
n := got["ICMS70"].(map[string]any)
if n["motDesICMSST"] != "9" || n["vICMSSTDeson"] == nil {
t.Fatalf("ST desonerada ausente: %v", n)
}
}

func TestICMS51ComFCPDiferido(t *testing.T) {
cfg := map[string]any{"icms_p_dif": "50.00", "icms_fcp_override": "2.00", "icms_p_fcp_dif": "100.00"}
got := buildICMSNormal("0", "51", decimal.RequireFromString("100.00"), cfg, "12.00", "0.00", decimal.NewFromInt(1))
n := got["ICMS51"].(map[string]any)
if n["pFCPDif"] != "100.00" || n["vFCPDif"] != "2.00" || n["vFCPEfet"] != "0.00" {
t.Fatalf("FCP diferido errado: %v", n)
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar**

```go
    // addSTDeson: ST desonerada. vICMSSTDeson é o ICMS-ST que deixou de ser
// cobrado, ou seja o próprio vICMSST calculado, e motDesICMSST diz por quê.
addSTDeson := func (nd map[string]any) map[string]any {
if mot := cfgStrPtr(cfg, "icms_mot_des_st"); mot != nil {
nd["vICMSSTDeson"] = vICMSST
nd["motDesICMSST"] = *mot
}
return nd
}
// addFCPDif: FCP diferido. vFCPDif é a parcela diferida do FCP e vFCPEfet o
// que sobra a recolher.
addFCPDif := func (nd map[string]any) map[string]any {
pFCPDif := cfgStrPtr(cfg, "icms_p_fcp_dif")
if pFCPDif == nil || !hasFCP {
return nd
}
vDif := vFCPd.Mul(d(*pFCPDif)).Div(decimal.NewFromInt(100)).RoundBank(2)
nd["pFCPDif"] = *pFCPDif
nd["vFCPDif"] = q2(vDif)
nd["vFCPEfet"] = q2(vFCPd.Sub(vDif).RoundBank(2))
return nd
}
```

`addSTDeson` entra em 10, 70 e 90; `addFCPDif` em 51 e 90. Conferir a ordem XSD dos dois grupos nas tabelas.

- [ ] **Step 4: Rodar, commit** — `git commit -am "feat(nfe): ST desonerada e FCP diferido nos grupos de ICMS"`

---

## Task 23: IPI por unidade (`qUnid`/`vUnid`) e selo (`CNPJProd`, `cSelo`, `qSelo`)

IPI por unidade é bebida e cigarro; o selo é nível 4 (produto).

**Files:** Modify `api/internal/services/nfes/builders_tax.go`, `builders_doc.go` (passar o item ao `buildIPI`),
`api/internal/api/v1/dto.go`; Test `builders_tax_test.go`.

**Interfaces:**

- Produces:
    - `buildIPI(ipiCST string, vProd, qty decimal.Decimal, cfg, item map[string]any) map[string]any` — assinatura nova.
    - `TaxFieldsBody` ganha `IpiVUnid *string`; `ProductBody` ganha `IpiCnpjProd`, `IpiCSelo`, `IpiQSelo`, `IpiCEnq`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildIPIPorUnidade(t *testing.T) {
got := buildIPI("50", decimal.RequireFromString("100.00"), decimal.RequireFromString("3"),
map[string]any{"ipi_v_unid": "1.5000"}, map[string]any{"ipi_c_enq": "999"})
trib := got["IPI"].(map[string]any)["IPITrib"].(map[string]any)
if trib["qUnid"] != "3.0000" || trib["vUnid"] != "1.5000" || trib["vIPI"] != "4.50" {
t.Fatalf("IPI por unidade errado: %v", trib)
}
if _, ok := trib["pIPI"]; ok {
t.Fatal("qUnid/vUnid e vBC/pIPI são choice — não coexistem")
}
}

func TestBuildIPISelo(t *testing.T) {
got := buildIPI("50", decimal.RequireFromString("100.00"), decimal.NewFromInt(1),
map[string]any{"ipi_aliq": "10.00"},
map[string]any{"ipi_cnpj_prod": "11111111111111", "ipi_c_selo": "S1", "ipi_q_selo": "10"})
ipi := got["IPI"].(map[string]any)
if ipi["CNPJProd"] != "11111111111111" || ipi["cSelo"] != "S1" || ipi["qSelo"] != "10" {
t.Fatalf("selo ausente: %v", ipi)
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar** — dentro de `buildIPI`, o ramo tributado passa a escolher entre os dois modos (`vUnid`
  presente ⇒ `qUnid`+`vUnid`; senão `vBC`+`pIPI`), e o nó `IPI` externo ganha
  `CNPJProd`/`cSelo`/`qSelo` antes de `cEnq` (ordem XSD: `CNPJProd, cSelo, qSelo, cEnq, choice{IPITrib|IPINT}`).
  `cEnq` deixa de ser fixo `"999"` e passa a ler `item["ipi_c_enq"]` com `"999"` de default.

- [ ] **Step 4: Rodar, atualizar golden, commit** —
  `git commit -am "feat(nfe): IPI por unidade e selo de controle"`

---

## Task 24: `PISST` e `COFINSST`

Combustíveis e farmacêutico. Os quatro campos já existem em `TaxFieldsBody` (`pis_st_aliq`, `cofins_st_aliq`,
`pis_st_v_bc`, `cofins_st_v_bc`) — falta só o builder.

**Interfaces:**

- Produces:
    - `func buildPISST(cfg map[string]any, vProd decimal.Decimal) map[string]any`
    - `func buildCOFINSST(cfg map[string]any, vProd decimal.Decimal) map[string]any`
    - Ambas devolvem `nil` quando não configuradas; `imposto["PISST"]`/`imposto["COFINSST"]` só entram se não-nil.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildPISST(t *testing.T) {
got := buildPISST(map[string]any{"pis_st_v_bc": "120.00", "pis_st_aliq": "1.65"}, decimal.RequireFromString("100.00"))
if got["vBC"] != "120.00" || got["pPIS"] != "1.65" || got["vPIS"] != "1.98" {
t.Fatalf("PISST errado: %v", got)
}
}

func TestBuildPISSTAusente(t *testing.T) {
if buildPISST(map[string]any{}, decimal.RequireFromString("100.00")) != nil {
t.Fatal("sem configuração de ST, o grupo não existe")
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar** — uma função genérica parametrizada pelos nomes das tags, e duas cascas:

```go
// buildPISCOFINSST monta PISST/COFINSST, que têm estrutura idêntica e só
// diferem nos nomes das tags. Base própria (v_bc) quando informada; senão o
// valor do produto. Modo por quantidade (qBCProd+vAliqProd) fica de fora até
// haver caso concreto — o XSD é choice e adivinhar qual ramo usar é pior que
// não emitir.
func buildPISCOFINSST(cfg map[string]any, vProd decimal.Decimal, aliqKey, vbcKey, pTag, vTag string) map[string]any {
aliq := cfgStrPtr(cfg, aliqKey)
if aliq == nil || *aliq == "" {
return nil
}
vBC := vProd.RoundBank(2)
if v := cfgStrPtr(cfg, vbcKey); v != nil && *v != "" {
vBC = d(*v)
}
return map[string]any{
"vBC": q2(vBC), pTag: *aliq,
vTag: q2(vBC.Mul(d(*aliq)).Div(decimal.NewFromInt(100)).RoundBank(2)),
}
}

func buildPISST(cfg map[string]any, vProd decimal.Decimal) map[string]any {
return buildPISCOFINSST(cfg, vProd, "pis_st_aliq", "pis_st_v_bc", "pPIS", "vPIS")
}

func buildCOFINSST(cfg map[string]any, vProd decimal.Decimal) map[string]any {
return buildPISCOFINSST(cfg, vProd, "cofins_st_aliq", "cofins_st_v_bc", "pCOFINS", "vCOFINS")
}
```

- [ ] **Step 4: Rodar, commit** — `git commit -am "feat(nfe): grupos PISST e COFINSST"`

---

## Task 25: `total/retTrib` — retenções federais

Nível 2: o **perfil** de retenção fica na operação; os valores são calculados sobre a base do documento.

**Files:** Modify `api/internal/api/v1/dto.go` (`OperationBody`), `api/internal/services/nfes/builders_total.go`,
`emit.go`; Test `builders_total_test.go`.

**Interfaces:**

- Produces:
    - `type RetTribBody struct{ PRetPis, PRetCofins, PRetCsll, PRetIrrf, PRetPrevInss *string }` em `dto.go`, embutido
      em `OperationBody` como `RetTrib *RetTribBody \`json:"ret_trib"\``.
    - `func buildRetTrib(profile map[string]any, base decimal.Decimal) map[string]any`
    - `buildTotal` ganha o parâmetro `retTrib map[string]any`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildRetTribCalculaSobreABase(t *testing.T) {
got := buildRetTrib(map[string]any{
"p_ret_pis": "0.65", "p_ret_cofins": "3.00", "p_ret_csll": "1.00", "p_ret_irrf": "1.50",
}, decimal.RequireFromString("1000.00"))
if got["vRetPIS"] != "6.50" || got["vRetCOFINS"] != "30.00" || got["vRetCSLL"] != "10.00" {
t.Fatalf("retenções erradas: %v", got)
}
if got["vBCIRRF"] != "1000.00" || got["vIRRF"] != "15.00" {
t.Fatalf("IRRF errado: %v", got)
}
if _, ok := got["vBCRetPrev"]; ok {
t.Fatal("INSS não configurado não pode aparecer")
}
}
```

- [ ] **Step 2: Rodar e ver falhar.**

- [ ] **Step 3: Implementar** — ordem XSD:
  `vRetPIS, vRetCOFINS, vRetCSLL, vBCIRRF, vIRRF, vBCRetPrev, vRetPrev`. `vBCIRRF`/`vBCRetPrev` só aparecem acompanhados
  do respectivo valor.

- [ ] **Step 4: Rodar, commit** — `git commit -am "feat(nfe): retenções federais em total/retTrib"`

---

## Task 26: `impostoDevol` (devolução por não contribuinte)

Derivado: só existe quando `finNFe=4` (devolução) e há `NFref` — as duas coisas que a Task 3 introduziu. O percentual
devolvido é nível 7 (é da nota); o `vIPIDevol` é calculado.

**Interfaces:**

- Produces: `NfeProductItem` ganha `PDevol *string \`json:"p_devol" validate:"omitempty,percent"\``;
  `func buildImpostoDevol (pDevol string, vIPI decimal.Decimal) map[string]any`;
  `ICMSTot.vIPIDevol` deixa de ser fixo `"0.00"`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildImpostoDevol(t *testing.T) {
got := buildImpostoDevol("100.00", decimal.RequireFromString("10.00"))
if got["pDevol"] != "100.00" || got["IPI"].(map[string]any)["vIPIDevol"] != "10.00" {
t.Fatalf("impostoDevol errado: %v", got)
}
}
```

- [ ] **Step 2/3/4:** implementar, somar em `totals.VIPIDevol`, emitir `vIPIDevol` no `ICMSTot`, e validar em `Emit`
  que `p_devol` só é aceito com `finNFe=4` (`problem.BadRequest` caso contrário, com teste).
  `git commit -am "feat(nfe): impostoDevol na devolução por não contribuinte"`

---

## Task 27: `transp/retTransp` e `det/obsItem`

`retTransp` é nível 5 (perfil na transportadora); `obsItem` é nível 3/4.

**Interfaces:**

- Produces:
    - `PersonObjectBody` ganha `FreightRetention *FreightRetentionBody` (`v_serv`, `v_bc_ret`, `p_icms_ret`, `cfop`,
      `c_mun_fg`).
    - `func buildRetTransp(profile map[string]any) map[string]any` — ordem
      `vServ, vBCRet, pICMSRet, vICMSRet, CFOP, cMunFG`, com `vICMSRet` calculado.
    - `TaxFieldsBody`/`ProductBody` ganham `obs_item_x_campo`/`obs_item_x_texto`;
      `func buildObsItem(cfg, item map[string]any) map[string]any`.

- [ ] **Steps:** teste → falha → implementação → PASS → commit
  (`feat(nfe): ICMS retido pelo remetente e observação fiscal por item`).

---

## Task 28: `ISSQN` completo e `ISSQNtot`

`buildISSQN` e `ISSQNtot` já existem parcialmente (`builders_tax.go:577`, `builders_doc.go:996`). Completar contra o
XSD: `vDeducao`, `vOutro`, `vDescIncond`, `vDescCond`, `vISSRet`, `indISS`, `cServico`, `cMun`, `cPais`, `nProcesso`,
`indIncentivo`. Reusar `organization_services` — o catálogo de serviços da NFS-e já tem código e alíquota, e criar um
segundo catálogo seria a duplicação que o plano proíbe.

**Interfaces:**

- Produces: `NfeProductItem` ganha `ServiceID *string`; `resolveProducts` passa a aceitar item de serviço (produto
  **ou** serviço, nunca ambos); `buildISSQN(vBC decimal.Decimal, cfg, service map[string]any) map[string]any`.

- [ ] **Steps:** teste com item de serviço completo → falha → implementação → PASS → payload de integração
  (`build_nfe(..., servicos=[…])`) → commit
  (`feat(nfe): NF-e mista com ISSQN completo reusando o catálogo de serviços`).

---

# Bloco 5 — NF-e Fase C: importação e exportação

## Task 29: Cadastro `organization_import_declarations` + `prod/DI` + `adi`

Este é o cadastro que a Receita R1 usa como exemplo — os valores de R1 já são os desta tarefa. Uma DI cobre várias notas
e vários itens; `nAdicao`/`nSeqAdic` são **derivados** do vínculo item↔adição, nunca digitados na emissão.

**Files:**

- R1 completa (`organization_import_declarations` / `IMPORTDI_` / `IMPORT_DECLARATION` / `import_declarations`)
- Modify: `api/internal/services/nfes/builders_prod.go`, `emit.go`
- Create: `api/internal/services/nfes/builders_prod_test.go`

**Interfaces:**

- Consumes: R1; `resolveProducts`.
- Produces:
    - `ImportDeclarationBody` (abaixo), `ImportDeclarationRepository`, `ImportDeclarationService`.
    - `NfeProductItem` ganha `ImportDeclarations []NfeItemDIBody` com
      `{ImportDeclarationID string; AdditionIndex int; NDraw *string}`.
    - `func buildDI(di map[string]any, additionIndex, seq int, nDraw string) map[string]any`

- [ ] **Step 1: Teste que falha**

```go
func TestBuildDIDerivaNumeroDaAdicao(t *testing.T) {
di := map[string]any{
"n_di": "2026/0000001", "d_di": "2026-01-15", "x_loc_desemb": "PORTO DE ITAQUI",
"uf_desemb": "MA", "d_desemb": "2026-01-20", "tp_via_transp": "01",
"v_afrmm": "150.00", "tp_intermedio": "1", "c_exportador": "EXP-1",
"additions": []any{
map[string]any{"n_adicao": "1", "c_fabricante": "F1", "v_desc_di": "0.00"},
map[string]any{"n_adicao": "2", "c_fabricante": "F2", "v_desc_di": "5.00"},
},
}
got := buildDI(di, 1, 1, "")
if got["nDI"] != "2026/0000001" || got["UFDesemb"] != "MA" || got["vAFRMM"] != "150.00" {
t.Fatalf("cabeçalho da DI errado: %v", got)
}
adi := got["adi"].([]map[string]any)
if len(adi) != 1 || adi[0]["nAdicao"] != "2" || adi[0]["nSeqAdic"] != "1" || adi[0]["cFabricante"] != "F2" {
t.Fatalf("adição derivada errada: %v", adi)
}
}
```

- [ ] **Step 2: Rodar e ver falhar** — `go test ./internal/services/nfes -run TestBuildDI`.

- [ ] **Step 3: Implementar**

```go
// ImportDeclarationBody é o body de POST/PUT /import-declarations.
//
// Uma DI cobre várias notas e vários itens. Ela é cadastrada uma vez, com suas
// adições; na emissão o item só aponta qual adição o representa, e nAdicao /
// nSeqAdic saem desse vínculo.
type ImportDeclarationBody struct {
Name         string `json:"name" validate:"required,min=2,max=120"`
NDI          string `json:"n_di" validate:"required,max=15"`
DDI          string `json:"d_di" validate:"required,datebr"`
XLocDesemb   string `json:"x_loc_desemb" validate:"required,max=60"`
UFDesemb     string `json:"uf_desemb" validate:"required,uf"`
DDesemb      string `json:"d_desemb" validate:"required,datebr"`
// tpViaTransp: 01 marítima … 12 por reboque. Ver TViaTransp do XSD.
TpViaTransp  string  `json:"tp_via_transp" validate:"required,len=2,number"`
// vAFRMM é obrigatório quando tpViaTransp = 01 (marítima).
VAFRMM       *string `json:"v_afrmm" validate:"omitempty,money2"`
// tpIntermedio: 1 conta própria, 2 conta e ordem, 3 encomenda.
TpIntermedio string  `json:"tp_intermedio" validate:"required,oneof=1 2 3"`
CNPJ         *string `json:"cnpj" validate:"omitempty,cnpj"`
UFTerceiro   *string `json:"uf_terceiro" validate:"omitempty,uf"`
CExportador  string  `json:"c_exportador" validate:"required,max=60"`
Additions    []ImportAdditionBody `json:"additions" validate:"required,min=1,max=100,dive"`
}

// ImportAdditionBody é uma adição da DI (prod/DI/adi).
type ImportAdditionBody struct {
NAdicao     string  `json:"n_adicao" validate:"required,max=3,number"`
CFabricante string  `json:"c_fabricante" validate:"required,max=60"`
VDescDI     *string `json:"v_desc_di" validate:"omitempty,money2"`
NDraw       *string `json:"n_draw" validate:"omitempty,max=20"`
}
```

`buildDI` monta o nó na ordem do XSD
(`nDI, dDI, xLocDesemb, UFDesemb, dDesemb, tpViaTransp, vAFRMM, tpIntermedio, CNPJ, UFTerceiro, cExportador, adi`), com
`adi` contendo só a adição escolhida (`nAdicao, nSeqAdic, cFabricante, vDescDI, nDraw`).
`vAFRMM` obrigatório quando `tpViaTransp == "01"`: validar no serviço, com `problem.BadRequest`, e testar.

- [ ] **Step 4: Ligar em `resolveProducts`** — as DIs citadas por todos os itens são lidas num `BatchGet` só, como os
  perfis fiscais já são hoje (`loadTaxProfiles`). Uma DI inexistente é `problem.NotFound`.

- [ ] **Step 5: Rodar, UI, docs (§40), commit**

```bash
cd api && go test ./... && cd ../ui && npx eslint src --ext .ts,.tsx && cd ../cdk && npx tsc --noEmit
git add api ui cdk DOCS.md DynamoDB-Tables.md
git commit -m "feat(nfe): cadastro de declarações de importação e grupo prod/DI"
```

---

## Task 30: `prod/nFCI`, `prod/NVE`, `prod/cBarra` e `cBarraTrib`

Nível 4 — quatro campos do produto, um commit.

**Interfaces:**

- Produces: `ProductBody` ganha `NFci *string` (`uuid`), `Nve []string` (`max=8,dive,len=6`),
  `CBarra *string` (`max=30`), `CBarraTrib *string` (`max=30`); `buildProd` os emite na ordem XSD (`cProd, cEAN, cBarra, xProd, NCM, NVE, CEST, indEscala, CNPJFab, cBenef, gCred, EXTIPI, CFOP, uCom, qCom,
  vUnCom, vProd, cEANTrib, cBarraTrib, uTrib, …`).

- [ ] **Step 1: Teste que falha**

```go
func TestBuildProdNVEComoLista(t *testing.T) {
item := map[string]any{"product_code": "P1", "nve": []any{"AA0001", "BB0002"}, "n_fci": "0A1B2C3D-…"}
prod := buildProd(item, /* … */)
nve := prod["NVE"].([]string)
if len(nve) != 2 || nve[0] != "AA0001" {
t.Fatalf("NVE errado: %v", prod["NVE"])
}
if prod["nFCI"] != "0A1B2C3D-…" {
t.Fatalf("nFCI ausente: %v", prod)
}
}
```

- [ ] **Step 2/3/4:** falhar → implementar → PASS → commit
  (`feat(nfe): NVE, nFCI e códigos de barra próprios no produto`).

---

## Task 31: `prod/detExport` + `exportInd`, e `infNFe/exporta`

`detExport` é nível 7 (`nDraw` pode ser nível 4, vindo da adição da DI da Task 29). `exporta` é nível 1 + 6:
a UF de saída vem da operação de exportação; o local de despacho reusa `pickup_locations`, que já existe na organização.

**Interfaces:**

- Produces:
    - `NfeProductItem` ganha `Exports []NfeDetExportBody{NDraw *string; NRE *string; ChNFe *string; QExport *string}`.
    - `OperationBody` ganha `ExportUFSaidaPais *string` e `ExportLocDespachoIndex *int` (índice em
      `organizations.pickup_locations`, para não copiar o endereço).
    - `func buildExporta(op map[string]any, pickups []any) map[string]any` em `builders_extra.go`.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildExportaUsaLocalDeRetiradaSalvo(t *testing.T) {
op := map[string]any{"export_uf_saida_pais": "PI", "export_loc_despacho_index": 0}
pickups := []any{map[string]any{"x_lgr": "Porto de Luís Correia", "x_mun": "Luis Correia"}}
got := buildExporta(op, pickups)
if got["UFSaidaPais"] != "PI" || got["xLocDespacho"] != "Porto de Luís Correia" {
t.Fatalf("exporta errado: %v", got)
}
}
```

- [ ] **Step 2/3/4:** falhar → implementar (`xLocExporta` = município do local; ordem
  `UFSaidaPais, xLocExporta, xLocDespacho`) → PASS → commit
  (`feat(nfe): exportação direta e indireta (exporta e detExport)`).

---

## Task 32: `II` (`vBC`, `vDespAdu`, `vII`, `vIOF`)

Casado com a DI da Task 29. Nível 3 para as alíquotas, nível 7 para as despesas aduaneiras do lote.

**Interfaces:**

- Produces: `NfeItemDIBody` ganha `VDespAdu`, `VII`, `VIOF`;
  `func buildII(item map[string]any, vProd decimal.Decimal) map[string]any`;
  `ICMSTot.vII` deixa de ser fixo `"0.00"` e passa a somar `totals.VII`.

- [ ] **Steps:** teste (`vII` somado no total, `II` ausente quando não há DI) → falha → implementação → PASS → commit
  (`feat(nfe): imposto de importação no item e no total`).

---

# Bloco 6 — MDF-e Fases C e D

## Task 33: Cadastro `organization_insurance_policies` + `seg` completo

Hoje só `nApol`/`nAver` soltos, sem responsável nem seguradora — o que o XSD não aceita. Nível 6: apólice recorre; por
viagem só a averbação.

**Files:** R1 completa (`organization_insurance_policies` / `INSURANCE_` / `INSURANCE_POLICY` /
`insurance_policies`); Modify `api/internal/services/mdfes/builder.go`, `emit.go`.

**Interfaces:**

- Produces:
    - `InsurancePolicyBody{Name, RespSeg, CNPJ, CPF, XSeg, CNPJSeg, NApol}` — `respSeg`: 1 emitente do MDF-e, 2
      contratante do serviço.
    - `func buildSeg(policies []resolvedPolicy) []map[string]any` — ordem XSD `infResp{respSeg, CNPJ|CPF},
    infSeg{xSeg, CNPJ}, nApol, nAver`.

- [x] **Step 1: Teste que falha**

```go
func TestBuildSegComResponsavelESeguradora(t *testing.T) {
got := buildSeg([]resolvedPolicy{{
RespSeg: "1", CNPJ: "11111111111111", XSeg: "Seguradora X", CNPJSeg: "22222222222222",
NApol: "AP-1", NAver: []string{"AV-1", "AV-2"},
}})
s := got[0]
if s["infResp"].(map[string]any)["respSeg"] != "1" {
t.Fatalf("infResp errado: %v", s)
}
if s["infSeg"].(map[string]any)["xSeg"] != "Seguradora X" {
t.Fatalf("infSeg errado: %v", s)
}
if len(s["nAver"].([]string)) != 2 {
t.Fatalf("averbações ausentes: %v", s)
}
}
```

- [x] **Step 2/3/4/5:** falhar → implementar → UI (R1.11) → docs (§41) → commit
  (`feat(mdfe): cadastro de apólices e grupo seg completo`).

---

## Task 34: `indCanalVerde`, `indCarregaPosterior`, `prodPred/cEAN`, `infAdFisco` e CSRT no MDF-e

Quatro campos de nível 1/2 e um derivado, num commit. `FiscalConfigBody` (usado por `mdfe-config`) ganha
`ind_canal_verde`, `ind_carrega_posterior` e `inf_ad_fisco`; o CSRT vem da Task 9.

**Interfaces:**

- Produces: `buildIde` do MDF-e emite `indCanalVerde`/`indCarregaPosterior`; `buildProdPred` emite `cEAN` lido do
  produto predominante; `BuildMDFe` usa `nfes.BuildRespTec` (ou o equivalente promovido a `services/`, ver Task 9).

- [ ] **Steps:** teste → falha → implementação → PASS → commit
  (`feat(mdfe): canal verde, carregamento posterior, cEAN do produto predominante e mensagem ao fisco`).

---

## Task 35: Ligar os modais aéreo e ferroviário

Os builders existem e estão desligados. Antes de ligar: conferir campo a campo contra
`mdfeModalAereo_v3.00.xsd` e `mdfeModalFerroviario_v3.00.xsd`, e cobrir o DAMDFE.

**Files:** Modify `api/internal/services/mdfes/{mdfes,modals,emit}.go`;
`py-dfe/py_dfe/**/damdfe*` (layout por modal); Test `mdfes_test.go`,
`py-dfe/tests/integration/test_damdfe_generation.py`.

**Interfaces:**

- Produces: `enabledModals` passa a `{rodoviario, aereo, ferroviario: true}`; `MdfeEmitBody` valida que o payload do
  modal escolhido está presente (`problem.BadRequest` quando falta).

- [ ] **Step 1: Teste que falha**

```go
func TestEmitAereoExigePayloadDoModal(t *testing.T) {
// modal "aereo" sem `air` no body → 400 com mensagem explícita
}

func TestEmitAereoAceito(t *testing.T) {
// modal "aereo" com payload completo → infModal.aereo presente
}
```

- [ ] **Step 2/3/4:** falhar → ligar + validar + DAMDFE → PASS → commit
  (`feat(mdfe): habilita emissão nos modais aéreo e ferroviário`).

---

## Task 36: Completar e ligar o modal aquaviário

`buildAquav` ignora `infEmbComb`, `infUnidCargaVazia`, `infUnidTranspVazia` e `MMSI`. Embarcação e terminais são nível
6 — reusar `organization_cargo_units` (Task 17) para as unidades, e acrescentar o tipo `vessel` a ele em vez de criar
uma oitava tabela.

**Interfaces:**

- Produces: `MdfeWaterModal` ganha `MMSI string`, `Fuels []MdfeVesselFuel{CComb, QTotComb}`,
  `EmptyCargoUnits []string`, `EmptyTransportUnits []string`; `CargoUnitBody.Kind` aceita `vessel`.

- [ ] **Steps:** teste comparando o nó `aquav` produzido com o XSD (todos os grupos presentes) → falha → implementação →
  `enabledModals[ModalAquaviario] = true` → PASS → commit
  (`feat(mdfe): completa o modal aquaviário e habilita sua emissão`).

---

# Bloco 7 — NF-e Fase D: segmentos verticais

Priorizar por demanda real de cliente (spec §4, item 6). A ordem abaixo é a do plano-fonte; qualquer reordenação dentro
do bloco é livre.

## Task 37: Cadastro `organization_product_lots` + `prod/rastro`

Obrigatório em medicamento; comum em alimento, bebida e agrotóxico. Nível 6: o cliente escolhe o lote e a **quantidade é
rateada automaticamente** pela quantidade vendida.

**Files:** R1 completa (`organization_product_lots` / `PRODUCTLOT_` / `PRODUCT_LOT` / `product_lots`); Modify
`api/internal/services/nfes/builders_prod.go`, `emit.go`.

**Interfaces:**

- Produces:
    - `ProductLotBody{Name, ProductID, NLote, QLote, DFab, DVal, CAgreg}`.
    - `NfeProductItem` ganha `Lots []NfeItemLotBody{LotID string; Quantity *string}`.
    - `func buildRastro(lots []resolvedLot, qty decimal.Decimal) []map[string]any` — quando o item cita lotes sem
      quantidade, a quantidade vendida é rateada entre eles, a última absorvendo o resíduo (mesma regra da Task 17).

- [ ] **Step 1: Teste que falha**

```go
func TestBuildRastroRateiaQuantidade(t *testing.T) {
got := buildRastro([]resolvedLot{
{NLote: "L1", DFab: "2026-01-01", DVal: "2027-01-01"},
{NLote: "L2", DFab: "2026-02-01", DVal: "2027-02-01"},
}, decimal.RequireFromString("10"))
if got[0]["qLote"] != "5.000" || got[1]["qLote"] != "5.000" {
t.Fatalf("rateio errado: %v", got)
}
if got[0]["nLote"] != "L1" || got[0]["dVal"] != "2027-01-01" {
t.Fatalf("lote errado: %v", got[0])
}
}
```

- [ ] **Step 2/3/4/5:** falhar → implementar (ordem `nLote, qLote, dFab, dVal, cAgreg`) → UI → docs (§42) → commit
  (`feat(nfe): cadastro de lotes e grupo prod/rastro com rateio`).

---

## Task 38: Cadastro `organization_fuel_pumps` + `prod/comb` completo

Nível 6 + derivado. Bombas e tanques viram cadastro; **`vEncIni` é o `vEncFin` da venda anterior da mesma bomba** —
derivado, nunca digitado. `origComb` fica no produto.

**Files:** R1 completa (`organization_fuel_pumps` / `FUELPUMP_` / `FUEL_PUMP` / `fuel_pumps`); Modify
`api/internal/services/nfes/{builders_prod,nfce_emit,emit}.go`, `api/internal/api/v1/dto.go`.

**Interfaces:**

- Produces:
    - `FuelPumpBody{Name, NBico, NBomba, NTanque}` + o atributo mutável `last_v_enc_fin` (não vai no Body: é escrito
      pela emissão, não pelo usuário).
    - `NfeProductItem` ganha `FuelPumpID *string` e `VEncFin *string`.
    - `func buildComb(item map[string]any, pump map[string]any, qty decimal.Decimal) map[string]any` — inclui
      `CIDE{qBCProd, vAliqProd, vCIDE}`, `encerrante{nBico, nBomba, nTanque, vEncIni, vEncFin}`, `origComb[]`,
      `qTemp`, `pBio`.
    - `func (s *NfeService) advanceEncerrante(ctx context.Context, orgPK, pumpID, vEncFin string) error` — atualiza
      `last_v_enc_fin` **no mesmo TransactWrite** da emissão, nunca depois.

- [ ] **Step 1: Teste que falha**

```go
func TestBuildCombEncerranteUsaLeituraAnterior(t *testing.T) {
pump := map[string]any{"n_bico": "1", "n_bomba": "2", "n_tanque": "3", "last_v_enc_fin": "1000.000"}
item := map[string]any{"comb_c_prod_anp": "320102001", "v_enc_fin": "1050.000"}
got := buildComb(item, pump, decimal.RequireFromString("50"))
enc := got["encerrante"].(map[string]any)
if enc["vEncIni"] != "1000.000" || enc["vEncFin"] != "1050.000" {
t.Fatalf("encerrante errado: %v", enc)
}
}

func TestBuildCombEncerranteRegressivoRecusado(t *testing.T) {
// vEncFin menor que a leitura anterior é impossível fisicamente: erro, não
// número negativo silencioso.
}
```

- [ ] **Step 2/3/4/5:** falhar → implementar (incluindo `problem.BadRequest` no caso regressivo) → transação → UI → docs
  (§43) → commit (`feat(nfe): cadastro de bombas e grupo comb completo com encerrante derivado`).

---

## Task 39: `prod/veicProd` completo e `prod/med` completo

`veicProd` já existe e está quase completo (`builders_doc.go:745-780`); falta conferir campo a campo contra o XSD e
tirar os defaults inventados (`"000001"` para `cMod`, `"06"` para `tpVeic`) — default de dado fiscal é rejeição adiada.
`med` só precisa de `cProdANVISA`, `vPMC` e `xMotivoIsencao`, que já existem: a tarefa é conferir a obrigatoriedade
condicional (`xMotivoIsencao` **exige** ausência de `vPMC`).

**Interfaces:**

- Produces: `buildVeicProd(item map[string]any) (map[string]any, error)` — devolve erro nomeando o campo faltante em vez
  de inventar valor; `buildMed(item map[string]any) (map[string]any, error)` idem.

- [ ] **Steps:** teste que afirma erro explícito por campo faltante → falha → implementação → PASS → commit
  (`fix(nfe): remove defaults inventados de veicProd e valida med`).

---

## Task 40: `agropecuario`, `cana` e `compra`

Três grupos de nicho, um commit por não compartilharem nada além do nível de raridade.

**Interfaces:**

- Produces:
    - `agropecuario`: `PersonObjectBody`/organização ganha `technical_manager_cpf` (nível 1);
      `NfeEmitBody` ganha `AgroReceipt *NfeAgroBody{NReceituario, NGuia, UFGuia}` (nível 7);
      `func buildAgropecuario(org map[string]any, req *NfeAgroBody) map[string]any`.
    - `cana`: `OperationBody` ganha `CanaSafra *string` e `CanaRef *string` (nível 2);
      `func buildCana(op map[string]any, deliveries []NfeCanaDeliveryBody) map[string]any` — `forDia` é gerado dos
      lançamentos diários, e `qTotMes`/`qTotAnt`/`qTotGer` são somas derivadas, nunca digitadas.
    - `compra`: `OperationBody` ganha `CompraXNEmp *string`; `NfeEmitBody` ganha `CompraXPed`/`CompraXCont`;
      `func buildCompra(op map[string]any, xPed, xCont string) map[string]any`.

- [ ] **Steps por grupo:** teste → falha → implementação → PASS → commit (`feat(nfe): grupo agropecuario`,
  `feat(nfe): grupo cana`, `feat(nfe): grupo compra`).

---

## Task 41: `prod/nRECOPI`, `ide/indIntermed` + `infIntermed`, `ide/dhSaiEnt` + `dPrevEntrega`, `prod/xPed` + `nItemPed`

Quatro itens pequenos, agrupados por serem todos "um campo, um nível óbvio".

**Interfaces:**

- Produces:
    - `ProductBody` ganha `NRecopi *string` (`len=20`) — nível 4.
    - Intermediador: papel novo `intermediary` em `services.AllPersonRoles` + `personRolesValidation`;
      `OperationBody` ganha `IntermediaryPersonID *string` e `IndIntermed *string`;
      `func buildInfIntermed(person map[string]any) map[string]any` (`CNPJ`, `idCadIntTran`).
    - `OperationBody` ganha `DhSaiEntOffsetDays *int` (default) e `NfeEmitBody` ganha `DhSaiEnt`/`DPrevEntrega`
      (override) — nível 2 + 7.
    - `NfeProductItem` ganha `XPed *string` e `NItemPed *string` — nível 7.

- [ ] **Steps:** um teste por campo → falha → implementação → PASS → commit
  (`feat(nfe): papel imune, marketplace, datas de saída/entrega e pedido do cliente`).

---

# Bloco 8 — NF-e Fase E: reforma tributária (IBS/CBS/IS)

Último por dependência: o conteúdo exato depende da NT vigente na data de implementação. **Antes de começar o bloco**,
reler a NT publicada e conferir contra `PL_010e_v1.02`; se a NT mudou, as tarefas abaixo continuam válidas em estrutura,
mas os nomes de tag têm que ser reconferidos um a um.

Alocação: quase tudo é nível 3 e 4 — CST, `cClassTrib` e alíquotas efetivas são perfil tributário, não input de nota.
`gCompraGov`, `cIndOp` e `tpNFDebito/tpNFCredito` são nível 2.

Esta fase é **pré-requisito dos eventos da reforma** (séries 1121xx/2111xx/2121xx): não faz sentido emitir evento de
apropriação de crédito presumido sem `gCredPresOper` no item.

## Task 42: `ide` da reforma — `cIndOp`, `cMunFGIBS`, `tpNFDebito`, `tpNFCredito`, `gCompraGov` + `refDFeAnt`

**Interfaces:**

- Produces: `ideParams` ganha `CIndOp, CMunFGIBS, TpNFDebito, TpNFCredito string` e `CompraGov map[string]any`;
  `OperationBody` ganha os quatro primeiros; `func buildCompraGov(op map[string]any, refs []string) map[string]any`.
- [ ] **Steps:** teste → falha → implementação → PASS → commit (`feat(nfe): campos de ide da reforma tributária`).

## Task 43: `prod/gCred`, `prod/tpCredPresIBSZFM`, `prod/indBemMovelUsado`

Nível 4/3. `gCred` é lista (`cCredPres`, `pCredPres`, `vCredPres`).

- [ ] **Steps:** teste → falha → implementação → PASS → commit
  (`feat(nfe): crédito presumido e bem móvel usado no produto`).

## Task 44: `gTribRegular` e `gTribCompraGov` no item

Ambos repetem a quádrupla CST/cClassTrib/alíquota/valor — extrair `buildTribBlock(prefix string, cfg map[string]any,
vBC decimal.Decimal) map[string]any` e usá-la nos dois, em vez de duas cópias.

- [ ] **Steps:** teste → falha → implementação → PASS → commit (`feat(nfe): gTribRegular e gTribCompraGov`).

## Task 45: `gIBSCBSMono` — `gMonoReten`, `gMonoRet`, `gMonoDif` e seus totais

O bloco monofásico da reforma. Espelha, em IBS/CBS, o que `ICMS02/15/53/61` já fazem em ICMS — reusar o mesmo formato de
alíquota específica (`adRem`) e o mesmo cálculo por quantidade.

- [ ] **Steps:** teste por sub-grupo → falha → implementação → PASS → commit (`feat(nfe): grupo monofásico de IBS/CBS`).

## Task 46: `gTransfCred`, `gAjusteCompet`, `gEstornoCred`

- [ ] **Steps:** teste → falha → implementação → PASS → commit
  (`feat(nfe): transferência, ajuste de competência e estorno de crédito`).

## Task 47: `gCredPresOper`, `gCredPresIBSZFM`, `gALCZFMCBS`

`gALCZFMCBS` é novo no `PL_010e_v1.02` — conferir que existe na versão do XSD em `py-dfe/schemas/` antes de emitir.

- [ ] **Steps:** teste → falha → implementação → PASS → commit (`feat(nfe): créditos presumidos e ALC/ZFM`).

## Task 48: `pDevTrib` nos três `gDevTrib`

Um campo, três lugares (`gIBSUF`, `gIBSMun`, `gCBS`) — uma função aplicada três vezes.

- [ ] **Steps:** teste → falha → implementação → PASS → commit
  (`feat(nfe): pDevTrib nos grupos de devolução de tributo`).

## Task 49: Totais da reforma — `IBSCBSTot/gMono`, `IBSCBSTot/gEstornoCred`, `ISTot`, `vNFTot`

Fecha a fase: todo grupo novo do item tem que aparecer somado aqui, e `vNFTot` é o total do documento com os tributos da
reforma. Teste obrigatório de **conservação**: a soma dos itens tem que bater com o total, item a item, para um
documento com pelo menos um item de cada grupo introduzido nas Tasks 42-48.

```go
func TestTotaisDaReformaConservamASomaDosItens(t *testing.T) {
// monta uma nota com um item de cada grupo (regular, monofásico, crédito
// presumido, estorno) e afirma que cada acumulador do IBSCBSTot é
// exatamente a soma dos itens correspondentes.
}
```

- [ ] **Steps:** teste de conservação → falha → implementação → PASS → payload de integração completo → commit
  (`feat(nfe): totais da reforma tributária`).

---

# Ordem de execução

Segue o spec §4, com o Bloco 0 na frente porque os splits são pré-requisito de todo o resto.

| # | Bloco                      | Tasks | Por quê agora                                                                   |
|---|----------------------------|-------|---------------------------------------------------------------------------------|
| 0 | Preparação                 | 1–2   | Sem o split, toda tarefa seguinte edita um arquivo de 1200 linhas às cegas.     |
| 1 | NF-e Fase A                | 3–9   | Destrava devolução, exportação por transportadora, marketplace e NFC-e com POS. |
| 2 | MDF-e Fase A               | 10–14 | Metade é wiring de campo já cadastrado.                                         |
| 3 | MDF-e Fase B (`infDoc`)    | 15–18 | Maior salto de cobertura por esforço do plano.                                  |
| 4 | NF-e Fase B (tax profiles) | 19–28 | Cada CST é isolado e testável em separado.                                      |
| 5 | NF-e Fase C (imp/exp)      | 29–32 | Depende do cadastro de DI (Task 29).                                            |
| 6 | MDF-e Fases C/D            | 33–36 | Seguro e modais restantes.                                                      |
| 7 | NF-e Fase D (verticais)    | 37–41 | Priorizar por demanda real de cliente.                                          |
| 8 | NF-e Fase E (reforma)      | 42–49 | Cronograma amarrado à NT vigente; abre os eventos da reforma.                   |

Cada bloco produz software que funciona e é testável sozinho. Parar entre blocos é seguro; parar no meio de um bloco
também, desde que a última tarefa concluída tenha commitado verde.

## Fora de escopo (declarado no spec, repetido aqui para não voltar por engano)

`avulsa` (NF-e emitida pelo fisco), `infSolicNFF` e `infPAA` (fluxo Nota Fiscal Fácil), em NF-e e em MDF-e. Reavaliar só
sob demanda concreta.

## Critério de pronto do plano inteiro

- [ ] Toda tag nova tem teste unitário de builder **e** payload de integração.
- [ ] Nenhuma tag nova chega ao request sem justificativa escrita contra a régua da §1 do spec.
- [ ] `xsdorder` (Go **e** Python) cobre a ordem de todo grupo novo.
- [ ] `DOCS.md` e `DynamoDB-Tables.md` refletem as 7 tabelas novas (§37-43) e todos os campos novos.
- [ ] `cd api && go test ./...`, `cd py-dfe && python -m pytest`, `cd ui && npx eslint src --ext .ts,.tsx`
  (zero warnings) e `cd cdk && npx tsc --noEmit` passam.
- [ ] `TestBuildEnviNFeGolden` está atualizado e o diff de cada regravação foi revisado, nunca aceito em bloco.
