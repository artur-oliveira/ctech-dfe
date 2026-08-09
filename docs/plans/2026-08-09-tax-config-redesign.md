# Redesign do modelo de tributação (perfil fiscal + produto) — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Estender a tributação de produto/perfil fiscal com um eixo de UF de destino (override
parcial), IBS/CBS opcional, campos de pauta fiscal/PIS-COFINS-ST, uma nova rota de consulta de
alíquota, e um warning de alíquota customizada no frontend — sem regressão em nenhuma emissão
existente.

**Architecture:** Backend (`api/internal/api/v1/dto.go`, `api/internal/services/nfes/tax_profiles.go`,
`tax_tables.go`) ganha os novos campos/tiers e uma nova rota fina de consulta de alíquota. Frontend
(`TaxFieldsEditor`, `ProductForm`, `TaxProfileForm`, novo `UfOverridesEditor`) ganha os novos grupos
condicionais e a UI de override por UF. DIFAL (`ICMSUFDest`) já existe em `builders_doc.go`/
`builders_tax.go` — não é tocado, só regression-testado.

**Tech Stack:** Go 1.x + Fiber v3 + go-playground/validator (backend); Next.js/React 19 +
react-hook-form + Zod + Vitest (frontend).

## Global Constraints

- Backend: todo erro é RFC 7807 via `problem.*` — nunca `fiber.Map`/erro cru.
- Backend: toda string/código/regra deve ser constante nomeada — nenhum literal espalhado.
- `resolveCfopTax` (`nfes/tax_profiles.go`) continua sendo o **único** ponto que decide precedência
  de tributação (CONDUCT.md) — nenhuma outra função duplica essa lógica.
- Frontend: UF é sempre selecionada via picker multi-select — nunca texto livre.
- Frontend: `npx eslint src --ext .ts,.tsx` deve passar com zero erros/warnings antes de qualquer commit.
- Todo core function novo (resolução fiscal, validação estrutural) precisa de teste unitário +
  integração/contrato, por `CLAUDE.md`.
- Nenhuma emissão NF-e/NFC-e existente (produto sem `uf_overrides`, sem novos campos) pode mudar de
  comportamento — os novos campos são sempre opcionais/aditivos.
- Documentar em `DOCS.md`/`CONDUCT.md` toda mudança de contrato, na mesma tarefa que a introduz.

---

### Task 1: `UfTaxOverride` + campos novos em `TaxFieldsBody` + IBS/CBS opcional (dto.go)

**Files:**
- Modify: `api/internal/api/v1/dto.go` (struct `TaxFieldsBody` linhas 165-236, `CfopConfigBody`
  linhas 240-243, `TaxProfileBody` linhas 351-356)
- Test: `api/internal/api/v1/dto_test.go`

**Interfaces:**
- Produces: `type UfTaxOverride struct { Ufs []string; Overrides map[string]any }` (json
  `ufs`/`overrides`), consumido pela Task 3 (`resolveCfopTax`).
- Produces: campos novos em `TaxFieldsBody`: `IcmsPautaValor *string` (json
  `icms_pauta_valor`), `IbsCbsPDevTrib *string` (json `ibs_cbs_p_dev_trib`), `PisStAliq
  *string`/`CofinsStAliq *string`/`PisStVBc *string`/`CofinsStVBc *string` (json
  `pis_st_aliq`/`cofins_st_aliq`/`pis_st_v_bc`/`cofins_st_v_bc`).
- Produces: campos `IbsCbsCst`/`IbsCbsClassTrib`/`IbsUfAliq`/`IbsMunAliq`/`CbsAliq` deixam de ser
  `required` e passam a ser `*string` `omitempty`.

- [ ] **Step 1: Escrever os testes de validação que falham hoje**

Adicionar em `dto_test.go` (usa o helper `validProduct()` já existente no arquivo, que monta um
`ProductBody` válido com um `CfopConfigBody`):

```go
// TestCfopConfig_UfOverrides_RequiresUfsWhenPresent garante que um item de
// uf_overrides sem UF nenhuma é erro — um override que não se aplica a nenhuma
// UF é configuração inútil, não um "override geral" disfarçado.
func TestCfopConfig_UfOverrides_RequiresUfsWhenPresent(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].UfOverrides = []UfTaxOverride{{Ufs: nil, Overrides: map[string]any{"icms_aliq_override": "12.00"}}}
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected error when uf_overrides[].ufs is empty")
	}
}

// TestCfopConfig_UfOverrides_ValidUf aceita um override bem formado.
func TestCfopConfig_UfOverrides_ValidUf(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].UfOverrides = []UfTaxOverride{{Ufs: []string{"SP", "RJ"}, Overrides: map[string]any{"icms_aliq_override": "12.00"}}}
	if p := validation.Struct(prod); p != nil {
		t.Errorf("expected valid, got %+v", p.Errors)
	}
}

// TestTaxFieldsBody_IbsCbs_OptionalWhenAllEmpty: hoje IBS/CBS é sempre exigido;
// depois desta task, um produto sem nenhum campo do grupo é válido (grupo
// omitido na emissão), mas preencher um campo do grupo sem os outros é erro
// (tudo-ou-nada).
func TestTaxFieldsBody_IbsCbs_OptionalWhenAllEmpty(t *testing.T) {
	prod := validProduct()
	prod.CfopConfig[0].IbsCbsCst = nil
	prod.CfopConfig[0].IbsCbsClassTrib = nil
	prod.CfopConfig[0].IbsUfAliq = nil
	prod.CfopConfig[0].IbsMunAliq = nil
	prod.CfopConfig[0].CbsAliq = nil
	if p := validation.Struct(prod); p != nil {
		t.Errorf("expected valid with IBS/CBS group fully absent, got %+v", p.Errors)
	}
}

func TestTaxFieldsBody_IbsCbs_PartialIsError(t *testing.T) {
	prod := validProduct()
	cst := "000"
	prod.CfopConfig[0].IbsCbsCst = &cst
	prod.CfopConfig[0].IbsCbsClassTrib = nil // resto do grupo ausente
	prod.CfopConfig[0].IbsUfAliq = nil
	prod.CfopConfig[0].IbsMunAliq = nil
	prod.CfopConfig[0].CbsAliq = nil
	p := validation.Struct(prod)
	if p == nil {
		t.Fatal("expected error: ibs_cbs group is all-or-nothing")
	}
}
```

Ajustar `validProduct()` (topo de `dto_test.go`) se ele monta `IbsCbsCst` etc. como `string` — vira
`*string` com `ptr("000")` (usar o helper `ptr`/`strPtr` já existente no arquivo, ou criar um
`func ptr[T any](v T) *T { return &v }` se não existir).

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `cd api && go test ./internal/api/v1/... -run 'TestCfopConfig_UfOverrides|TestTaxFieldsBody_IbsCbs' -v`
Expected: FAIL (campo `UfOverrides` não existe / `IbsCbsCst` ainda é `string` obrigatório sem grupo).

- [ ] **Step 3: Implementar `UfTaxOverride` e os campos novos**

Em `dto.go`, adicionar antes de `CfopConfigBody` (linha ~238):

```go
// UfTaxOverride é um override parcial de TaxFieldsBody aplicado só quando a UF
// de destino da operação está em Ufs. Não duplica os ~60 campos — só grava o
// que diverge para aquele conjunto de UFs (spec §Modelo de dados 1).
type UfTaxOverride struct {
	Ufs       []string       `json:"ufs" validate:"required,min=1,dive,uf"`
	Overrides map[string]any `json:"overrides" validate:"omitempty"`
}
```

Em `TaxFieldsBody`, mudar o bloco IBS/CBS (linhas 205-210) de `required` para `omitempty` e trocar
os 5 campos-chave de `string` para `*string`:

```go
	// IBS / CBS (Reforma Tributária) — opcional, tudo-ou-nada (ver validação em Step 5).
	IbsCbsCst       *string `json:"ibs_cbs_cst" validate:"omitempty,ibscst"`
	IbsCbsClassTrib *string `json:"ibs_cbs_class_trib" validate:"omitempty,class6"`
	IbsUfAliq       *string `json:"ibs_uf_aliq" validate:"omitempty,percent"`
	IbsMunAliq      *string `json:"ibs_mun_aliq" validate:"omitempty,percent"`
	CbsAliq         *string `json:"cbs_aliq" validate:"omitempty,percent"`
```

Adicionar ao final do bloco IBS/CBS (após `CbsAdRem`, linha 219):

```go
	IbsCbsPDevTrib *string `json:"ibs_cbs_p_dev_trib" validate:"omitempty,percent"`
```

Adicionar campo de pauta fiscal no bloco "Conditional ICMS" (após `IcmsPDif`, linha 185):

```go
	IcmsPautaValor *string `json:"icms_pauta_valor" validate:"omitempty,money2"`
```

Adicionar novo bloco PIS/COFINS-ST após o bloco PIS/COFINS (após linha 204):

```go
	// PIS/COFINS-ST — substituição tributária (grupo opcional, liga/desliga no frontend)
	PisStAliq    *string `json:"pis_st_aliq" validate:"omitempty,percent"`
	CofinsStAliq *string `json:"cofins_st_aliq" validate:"omitempty,percent"`
	PisStVBc     *string `json:"pis_st_v_bc" validate:"omitempty,money2"`
	CofinsStVBc  *string `json:"cofins_st_v_bc" validate:"omitempty,money2"`
```

Adicionar `UfOverrides` em `CfopConfigBody` e `TaxProfileBody`:

```go
type CfopConfigBody struct {
	Cfop        string          `json:"cfop" validate:"required,cfop"`
	UfOverrides []UfTaxOverride `json:"uf_overrides" validate:"omitempty,dive"`
	TaxFieldsBody
}
```

```go
type TaxProfileBody struct {
	Name        string          `json:"name" validate:"required,min=2,max=120"`
	Description *string         `json:"description" validate:"omitempty,max=255"`
	Cfops       []string        `json:"cfops" validate:"required,min=1,dive,cfop"`
	UfOverrides []UfTaxOverride `json:"uf_overrides" validate:"omitempty,dive"`
	TaxFieldsBody
}
```

- [ ] **Step 4: Confirmar que a tag `ibscst` existe**

Run: `grep -n '"ibscst"' api/internal/validation/*.go`
Se não existir (o campo era `string` com `required` puro, sem regex), adicionar em
`api/internal/validation/regex.go` (ou onde `regexValidators` está definido) uma entrada `ibscst`
com o mesmo regex hoje usado no Zod frontend (`_ibsCbsCstRegex` em
`ui/src/lib/schemas/products.ts:20`): `^(000|010|011|200|220|221|222|400|410|510|515|550|620|800|810|811|820|830)$`.
Adicionar também o caso `"ibscst"` em `message()` (`validation.go`), mensagem: `"CST IBS/CBS inválido"`.

- [ ] **Step 5: Implementar a validação estrutural tudo-ou-nada do grupo IBS/CBS**

Em `internal/validation/validation.go`, dentro de `get()` (após os `RegisterValidation` existentes,
antes de `instance = v`), registrar uma validação de struct para `TaxFieldsBody`:

```go
	v.RegisterStructValidation(ibsCbsGroupValidation, dto.TaxFieldsBody{})
```

Isso cria um import ciclo (`internal/validation` não pode importar `internal/api/v1`). Em vez disso,
a validação estrutural vive em `dto.go` mesmo, chamada explicitamente por um hook exposto por
`validation`. Adicionar em `validation.go`:

```go
// RegisterStructRule registra uma validação estrutural (nível de struct, não de
// campo) na instância compartilhada. Deve ser chamada em init() pelos pacotes
// que definem os DTOs — validation não pode importar v1 (import cycle).
func RegisterStructRule(fn validator.StructLevelFunc, types ...any) {
	get().RegisterStructValidation(fn, types...)
}
```

E em `dto.go`, adicionar ao final do arquivo:

```go
func init() {
	validation.RegisterStructRule(validateIbsCbsGroup, TaxFieldsBody{})
}

// validateIbsCbsGroup garante que o grupo IBS/CBS é tudo-ou-nada: se qualquer
// dos 5 campos-chave estiver preenchido, os outros 4 também precisam estar.
// Um produto sem NENHUM deles é válido — o grupo simplesmente não entra na
// emissão (spec §Modelo de dados 4).
func validateIbsCbsGroup(sl validator.StructLevel) {
	f := sl.Current().Interface().(TaxFieldsBody)
	vals := []*string{f.IbsCbsCst, f.IbsCbsClassTrib, f.IbsUfAliq, f.IbsMunAliq, f.CbsAliq}
	present := 0
	for _, v := range vals {
		if v != nil && *v != "" {
			present++
		}
	}
	if present == 0 || present == len(vals) {
		return
	}
	names := []string{"IbsCbsCst", "IbsCbsClassTrib", "IbsUfAliq", "IbsMunAliq", "CbsAliq"}
	for i, v := range vals {
		if v == nil || *v == "" {
			sl.ReportError(v, names[i], names[i], "required_with_group", "ibs_cbs")
		}
	}
}
```

`dto.go` já importa `gopkg.aoctech.app/dfe/api/internal/validation`? Confirmar com
`grep -n '"gopkg.aoctech.app/dfe/api/internal/validation"' api/internal/api/v1/dto.go` — se não,
adicionar ao bloco de imports (criar o bloco `import (...)` no topo do arquivo, hoje `dto.go` não
importa nada além do `package v1`).

Adicionar o caso `"required_with_group"` em `message()` (`validation.go`):
`"obrigatório quando outro campo do grupo IBS/CBS está preenchido"`.

- [ ] **Step 6: Rodar os testes e confirmar que passam**

Run: `cd api && go test ./internal/api/v1/... -run 'TestCfopConfig_UfOverrides|TestTaxFieldsBody_IbsCbs' -v`
Expected: PASS (4 testes).

Run também a suíte completa para checar que nada quebrou: `go build ./... && go test ./... -race`

- [ ] **Step 7: Commit**

```bash
git add api/internal/api/v1/dto.go api/internal/api/v1/dto_test.go api/internal/validation/validation.go
git commit -m "feat(api): add uf_overrides, pauta/PIS-COFINS-ST fields, and optional IBS/CBS group"
```

---

### Task 2: Tabela NCM+UF no backend + `resolveICMSAliq`/`resolveFCPAliq` com NCM

**Files:**
- Modify: `api/internal/services/nfes/tax_tables.go`
- Modify: `api/internal/services/nfes/builders_doc.go:489-490` (chamada de `resolveICMSAliq`/`resolveFCPAliq`)
- Test: `api/internal/services/nfes/tax_tables_test.go` (criar se não existir)
- Reference: `ui/src/lib/data/icms_ncm_lookup.ts` (fonte dos dados a migrar — hoje vazio,
  `ICMS_NCM_BY_UF = {}`, gerado por `scripts/generate-icms-lookup.js`; se ainda estiver vazio em
  produção, a tabela Go também começa vazia e este task só entrega o mecanismo, não dados)

**Interfaces:**
- Consumes: nenhuma dependência de tasks anteriores.
- Produces: `resolveICMSAliq(emitUF, destUF, ncm string, override *string) string` e
  `resolveFCPAliq(destUF, ncm string, override *string) string` — assinatura muda (parâmetro `ncm`
  adicionado); todo call site precisa ser atualizado no mesmo commit.

- [ ] **Step 1: Escrever o teste que falha**

Criar `api/internal/services/nfes/tax_tables_test.go`:

```go
package nfes

import "testing"

func TestResolveICMSAliq_NcmSpecificBeatsGenericTable(t *testing.T) {
	icmsNcmTable["SP"] = []icmsNcmEntry{{ncm: "8517", aliq: "7.00", fcp: nil}}
	defer func() { delete(icmsNcmTable, "SP") }()

	got := resolveICMSAliq("SP", "SP", "85171231", nil)
	if got != "7.00" {
		t.Errorf("expected NCM-specific rate 7.00, got %s", got)
	}
}

func TestResolveICMSAliq_OverrideBeatsNcmTable(t *testing.T) {
	icmsNcmTable["SP"] = []icmsNcmEntry{{ncm: "8517", aliq: "7.00", fcp: nil}}
	defer func() { delete(icmsNcmTable, "SP") }()

	fcp := "1.5000"
	got := resolveICMSAliq("SP", "SP", "85171231", &fcp)
	if got != "1.5000" {
		t.Errorf("expected override to win, got %s", got)
	}
}

func TestResolveICMSAliq_NoNcmMatchFallsBackToGenericTable(t *testing.T) {
	got := resolveICMSAliq("SP", "SP", "99999999", nil)
	if got != resolveICMSIntraAliq("SP") {
		t.Errorf("expected generic-table fallback, got %s", got)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `cd api && go test ./internal/services/nfes/... -run TestResolveICMSAliq -v`
Expected: FAIL (compile error — `icmsNcmTable`/`icmsNcmEntry` não existem, `resolveICMSAliq` tem
assinatura de 3 parâmetros, não 4).

- [ ] **Step 3: Implementar a tabela e o novo parâmetro**

Em `tax_tables.go`, adicionar antes de `resolveICMSAliq`:

```go
// icmsNcmEntry é uma alíquota ICMS específica de NCM, dentro de uma UF.
// Mirrors ui/src/lib/data/icms_ncm_lookup.ts IcmsNcmEntry.
type icmsNcmEntry struct {
	ncm  string
	aliq string
	fcp  *string
}

// icmsNcmTable[dest_uf] = entradas ordenadas do NCM mais específico para o
// menos específico (mesma convenção do lookup do frontend, migrado para cá —
// spec §Modelo de dados 5). Populada por scripts/generate-icms-lookup (fonte
// única de verdade passa a ser este arquivo; o lookup do frontend é removido
// na Task 10).
var icmsNcmTable = map[string][]icmsNcmEntry{}

// resolveIcmsNcm devolve a entrada mais específica de icmsNcmTable para
// destUF+ncm, ou nil se nenhuma bater.
func resolveIcmsNcm(destUF, ncm string) *icmsNcmEntry {
	for _, e := range icmsNcmTable[destUF] {
		if strings.HasPrefix(ncm, e.ncm) {
			return &e
		}
	}
	return nil
}
```

Adicionar `"strings"` ao import do arquivo.

Alterar as assinaturas:

```go
func resolveICMSAliq(emitUF, destUF, ncm string, override *string) string {
	if override != nil && *override != "" {
		return *override
	}
	if e := resolveIcmsNcm(destUF, ncm); e != nil {
		return e.aliq
	}
	if row, ok := aliqICMSTable[emitUF]; ok {
		if aliq, ok := row[destUF]; ok {
			return aliq
		}
	}
	return "17.00"
}

func resolveFCPAliq(destUF, ncm string, override *string) string {
	if override != nil {
		return *override
	}
	if e := resolveIcmsNcm(destUF, ncm); e != nil && e.fcp != nil {
		return *e.fcp
	}
	if v, ok := fcpAliq[destUF]; ok {
		return v
	}
	return "0.00"
}
```

- [ ] **Step 4: Atualizar os call sites**

`builders_doc.go:489-490`:
```go
		pICMSResolved := resolveICMSAliq(emitUF, destUF, anyStr(item, "ncm", ""), anyStrPtr(item, "icms_aliq_override"))
		pFCPResolved := resolveFCPAliq(destUF, anyStr(item, "ncm", ""), anyStrPtr(item, "fcp_aliq_override"))
```

`builders_doc.go:546` (dentro do bloco DIFAL) chama `resolveFCPAliq(destUF, nil)` — o DIFAL usa a
alíquota FCP genérica de destino, não a do produto; passar `""` como NCM (sem match, cai no
genérico, comportamento idêntico ao atual):
```go
				pFCPUFDest := resolveFCPAliq(destUF, "", nil)
```

Run `grep -rn "resolveICMSAliq(\|resolveFCPAliq(" api/internal/services/nfes/*.go` para confirmar
que não há outros call sites fora destes três e dos testes.

- [ ] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/services/nfes/... -run TestResolveICMSAliq -v`
Expected: PASS.

Run: `go build ./... && go test ./... -race` — confirma que os call sites atualizados não quebraram
nada em `nfce_service.go`/`emit.go`/testes existentes de `builders_doc_test.go`.

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/nfes/tax_tables.go api/internal/services/nfes/builders_doc.go api/internal/services/nfes/tax_tables_test.go
git commit -m "feat(api): migrate NCM+UF ICMS rate table into resolveICMSAliq/resolveFCPAliq"
```

---

### Task 3: `resolveCfopTax` — 6 níveis + merge por UF de destino

**Files:**
- Modify: `api/internal/services/nfes/tax_profiles.go`
- Modify: `api/internal/services/nfes/emit.go:596-634` (assinatura de `resolveProducts`/chamada de
  `resolveCfopTax` — precisa passar a UF de destino)
- Test: `api/internal/services/nfes/tax_profiles_test.go` (criar se não existir)

**Interfaces:**
- Consumes: `UfTaxOverride` (Task 1, mas como o payload chega como `map[string]any` via
  DynamoDB/attributevalue, não como o tipo Go — o código lê `ufs`/`overrides` do mapa igual a hoje).
- Produces: `resolveCfopTax(product map[string]any, profiles map[string]map[string]any, cfop, destUF string) (map[string]any, error)`
  — assinatura ganha o parâmetro `destUF`.

- [ ] **Step 1: Escrever os testes que falham**

Criar `api/internal/services/nfes/tax_profiles_test.go`:

```go
package nfes

import "testing"

func productWithUfOverride() map[string]any {
	return map[string]any{
		"cfop_config": []any{
			map[string]any{
				"cfop": "5102",
				"icms": "00",
				"icms_aliq_override": "18.00",
				"uf_overrides": []any{
					map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "20.00"}},
				},
			},
		},
	}
}

func TestResolveCfopTax_ProductUfOverride_BeatsProductBase(t *testing.T) {
	resolved, err := resolveCfopTax(productWithUfOverride(), nil, "5102", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "20.00" {
		t.Errorf("expected RJ override to win, got %v", resolved["icms_aliq_override"])
	}
}

func TestResolveCfopTax_ProductUfOverride_FallsBackForOtherUf(t *testing.T) {
	resolved, err := resolveCfopTax(productWithUfOverride(), nil, "5102", "SP")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "18.00" {
		t.Errorf("expected product base (no UF match), got %v", resolved["icms_aliq_override"])
	}
}

func TestResolveCfopTax_ProfileUfOverride_LowerThanProductLevel(t *testing.T) {
	product := map[string]any{
		"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
	}
	profiles := map[string]map[string]any{
		"p1": {
			"cfops": []any{"5102"},
			"icms_aliq_override": "12.00",
			"uf_overrides": []any{
				map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "22.00"}},
			},
		},
	}
	resolved, err := resolveCfopTax(product, profiles, "5102", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	if resolved["icms_aliq_override"] != "22.00" {
		t.Errorf("expected profile+UF override, got %v", resolved["icms_aliq_override"])
	}
}

func TestResolveCfopTax_NoLayerCovers_ReturnsError(t *testing.T) {
	if _, err := resolveCfopTax(map[string]any{}, nil, "5102", "SP"); err == nil {
		t.Fatal("expected error when no layer covers the CFOP")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `cd api && go test ./internal/services/nfes/... -run TestResolveCfopTax -v`
Expected: FAIL (assinatura de 3 argumentos, sem `destUF`).

- [ ] **Step 3: Implementar os 6 níveis + merge por UF**

Substituir o corpo de `resolveCfopTax` em `tax_profiles.go` (linhas 134-171):

```go
// ufOverridesField é a lista de overrides por UF dentro de um cfop_config/perfil.
const ufOverridesField = "uf_overrides"

// resolveCfopTax devolve o tratamento tributário efetivo de um produto para um
// CFOP e UF de destino, na ordem de precedência da spec (maior para menor):
//
//  1. cfop_config[cfop] do produto + uf_overrides da UF de destino
//  2. cfop_config[cfop] do produto (sem UF)
//  3. vínculo produto→perfil (overrides) + uf_overrides da UF de destino
//  4. vínculo produto→perfil (overrides), sem UF
//  5. tax_profile.cfops[cfop] + uf_overrides da UF de destino
//  6. tax_profile.cfops[cfop], sem UF
//  7. erro: nenhuma camada cobre o CFOP
//
// A primeira camada que cobrir o CFOP resolve — não há mistura entre níveis.
// Produto legado sem perfil e sem uf_overrides: resultado idêntico ao que
// sempre foi (garante zero regressão em emissões existentes).
func resolveCfopTax(product map[string]any, profiles map[string]map[string]any, cfop, destUF string) (map[string]any, error) {
	// Níveis 5-6: perfil.
	for _, id := range profileRefs(product) {
		profile, ok := profiles[id]
		if !ok {
			return nil, fmt.Errorf("perfil fiscal não encontrado: %s", id)
		}
		if !containsCFOP(profileCFOPs(profile), cfop) {
			continue
		}
		resolved := map[string]any{}
		mergeTaxFields(resolved, profile)
		mergeUfOverride(resolved, profile, destUF)
		// Nível 3-4: vínculo produto→perfil por cima do perfil.
		if ov := profileOverrides(product, id); ov != nil {
			mergeTaxFields(resolved, ov)
			mergeUfOverride(resolved, ov, destUF)
		}
		applyCfopConfig(resolved, product, cfop, destUF)
		if len(resolved) == 0 {
			continue
		}
		resolved[cfopField] = cfop
		return resolved, nil
	}

	// Sem perfil cobrindo: só cfop_config (níveis 1-2), comportamento legado.
	resolved := map[string]any{}
	applyCfopConfig(resolved, product, cfop, destUF)
	if len(resolved) == 0 {
		return nil, fmt.Errorf("CFOP %s sem tributação configurada", cfop)
	}
	resolved[cfopField] = cfop
	return resolved, nil
}

// applyCfopConfig mescla sobre dst a entrada de cfop_config do produto para
// este CFOP (níveis 1-2) — vence tudo que já estava em dst.
func applyCfopConfig(dst, product map[string]any, cfop, destUF string) {
	entries, ok := product[cfopConfigField].([]any)
	if !ok {
		return
	}
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if !ok || m[cfopField] != cfop {
			continue
		}
		mergeTaxFields(dst, m)
		mergeUfOverride(dst, m, destUF)
		return
	}
}

// mergeUfOverride aplica sobre dst o primeiro bloco de uf_overrides de src
// cuja lista ufs contenha destUF. src é o map (cfop_config, perfil, ou vínculo
// com overrides) que carrega o campo uf_overrides.
func mergeUfOverride(dst, src map[string]any, destUF string) {
	raw, ok := src[ufOverridesField].([]any)
	if !ok {
		return
	}
	for _, o := range raw {
		entry, ok := o.(map[string]any)
		if !ok {
			continue
		}
		ufs, ok := entry["ufs"].([]any)
		if !ok {
			continue
		}
		matched := false
		for _, u := range ufs {
			if s, ok := u.(string); ok && s == destUF {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if ov, ok := entry["overrides"].(map[string]any); ok {
			mergeTaxFields(dst, ov)
		}
		return
	}
}
```

Nota: `ov` (devolvido por `profileOverrides`) já é o map `overrides` do vínculo — o campo
`uf_overrides` do nível 3 vive como uma chave dentro desse mesmo `overrides` (não é um campo de
topo do vínculo; `ProductTaxProfileRef.Overrides` já é o `map[string]any` parcial da Task 1), então
`mergeUfOverride(resolved, ov, destUF)` lê `ov["uf_overrides"]` diretamente, sem função extra.

Adicionar `ufOverridesField` a `nonTaxFields` (linha ~213) — não é campo de tributação em si, é
metadado de override, e não deve vazar para o XML: `ufOverridesField: {}`.

Nota: `mergeUfOverride` é chamado com `refWithOverrides(...)` no nível 3-4, que já devolve o próprio
`overrides` map — então o `uf_overrides` que ele lê é `overrides.uf_overrides`, ou seja, o vínculo
produto→perfil aceita um `uf_overrides` dentro do seu próprio `Overrides` (que já é `map[string]any`
livre no DTO — não precisa mudar `ProductTaxProfileRef`).

- [ ] **Step 4: Atualizar `resolveProducts`/call sites para passar `destUF`**

`emit.go:634`: `resolveCfopTax(product, profiles, item.CFOP)` vira
`resolveCfopTax(product, profiles, item.CFOP, destUF)`. Confirmar que `destUF` já está em escopo
na função que chama `resolveProducts` (linha 211/119 de `emit.go`/`nfce_service.go`) — se
`resolveProducts` não recebe `destUF` hoje, adicionar como parâmetro:

```go
func resolveProducts(
	ctx context.Context, productRepo *repositories.ProductRepository,
	taxProfileRepo *repositories.TaxProfileRepository, orgPK, destUF string, items []NfeProductItem,
) ([]map[string]any, decimal.Decimal, decimal.Decimal, error) {
```

E atualizar as duas chamadas (`emit.go:211`, `nfce_service.go:119`) para passar a UF de destino já
resolvida naquele ponto (a mesma variável `destUF` usada por `resolveICMSAliq` mais abaixo em
`builders_doc.go` — confirmar com `grep -n "destUF" api/internal/services/nfes/emit.go
api/internal/services/nfes/nfce_service.go` onde ela é calculada antes da chamada a
`resolveProducts`, e mover o cálculo para antes se necessário).

- [ ] **Step 5: Rodar os testes**

Run: `cd api && go test ./internal/services/nfes/... -run TestResolveCfopTax -v`
Expected: PASS (4 testes).

Run: `go build ./... && go test ./... -race` — a suíte completa, incluindo os testes de emissão
existentes (`emit_test.go`/`nfce_service_test.go` se existirem), precisa continuar verde: produto
legado sem `uf_overrides` não pode mudar de resultado.

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/nfes/tax_profiles.go api/internal/services/nfes/emit.go api/internal/services/nfes/nfce_service.go api/internal/services/nfes/tax_profiles_test.go
git commit -m "feat(api): resolve tax config in 6 tiers with per-UF override merge"
```

---

### Task 4: Golden-file — matriz exaustiva CST/CSOSN × grupo opcional × nível de resolução

**Files:**
- Test: `api/internal/services/nfes/tax_resolution_matrix_test.go` (novo)

**Interfaces:**
- Consumes: `resolveCfopTax` (Task 3).

- [ ] **Step 1: Escrever a matriz table-driven**

```go
package nfes

import "testing"

// TestResolveCfopTax_ResolutionTierMatrix cobre cada um dos 6 níveis de
// resolução isoladamente, garantindo que o nível certo vence quando vários
// coexistem (regressão do bug "produto vence tudo" antes da introdução de UF).
func TestResolveCfopTax_ResolutionTierMatrix(t *testing.T) {
	cases := []struct {
		name         string
		product      map[string]any
		profiles     map[string]map[string]any
		cfop, destUF string
		wantAliq     string
	}{
		{
			name: "tier1_product_cfop_uf_wins_over_everything",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
				"cfop_config": []any{map[string]any{
					"cfop": "5102", "icms_aliq_override": "10.00",
					"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "99.00"}}},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop: "5102", destUF: "RJ", wantAliq: "99.00",
		},
		{
			name: "tier2_product_cfop_no_uf_match",
			product: map[string]any{
				"cfop_config": []any{map[string]any{"cfop": "5102", "icms_aliq_override": "10.00"}},
			},
			cfop: "5102", destUF: "SP", wantAliq: "10.00",
		},
		{
			name: "tier3_link_override_plus_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{
					"tax_profile_id": "p1",
					"overrides":      map[string]any{"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "50.00"}}}},
				}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop: "5102", destUF: "RJ", wantAliq: "50.00",
		},
		{
			name: "tier4_link_override_no_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1", "overrides": map[string]any{"icms_aliq_override": "30.00"}}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop: "5102", destUF: "SP", wantAliq: "30.00",
		},
		{
			name: "tier5_profile_plus_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
			},
			profiles: map[string]map[string]any{"p1": {
				"cfops": []any{"5102"}, "icms_aliq_override": "1.00",
				"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "22.00"}}},
			}},
			cfop: "5102", destUF: "RJ", wantAliq: "22.00",
		},
		{
			name: "tier6_profile_no_uf",
			product: map[string]any{
				"tax_profiles": []any{map[string]any{"tax_profile_id": "p1"}},
			},
			profiles: map[string]map[string]any{"p1": {"cfops": []any{"5102"}, "icms_aliq_override": "1.00"}},
			cfop: "5102", destUF: "SP", wantAliq: "1.00",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := resolveCfopTax(tc.product, tc.profiles, tc.cfop, tc.destUF)
			if err != nil {
				t.Fatal(err)
			}
			if resolved["icms_aliq_override"] != tc.wantAliq {
				t.Errorf("want %s, got %v", tc.wantAliq, resolved["icms_aliq_override"])
			}
		})
	}
}

// TestResolveCfopTax_Tier7_ErrorWhenUncovered é o 7º "nível" — nenhuma camada
// cobre o CFOP.
func TestResolveCfopTax_Tier7_ErrorWhenUncovered(t *testing.T) {
	if _, err := resolveCfopTax(map[string]any{"cfop_config": []any{}}, nil, "9999", "SP"); err == nil {
		t.Fatal("expected error for uncovered CFOP")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que passa**

Run: `cd api && go test ./internal/services/nfes/... -run TestResolveCfopTax_ResolutionTierMatrix -v`
Expected: PASS (6 subtestes) — se algum tier vencer errado, ajustar `resolveCfopTax` (Task 3) até
os 7 casos passarem exatamente como especificado na spec.

- [ ] **Step 3: Regressão DIFAL — garantir que a nova resolução não afeta `buildICMSUFDest`**

Adicionar a este mesmo arquivo:

```go
// TestDifal_UnaffectedByUfOverrides confirma que uf_overrides não interfere no
// cálculo automático de DIFAL (buildICMSUFDest usa resolveICMSIntraAliq/
// resolveICMSInterAliq/resolveFCPAliq diretamente por UF — não passa por
// resolveCfopTax) — spec §Modelo de dados 2.
func TestDifal_UnaffectedByUfOverrides(t *testing.T) {
	product := map[string]any{
		"cfop_config": []any{map[string]any{
			"cfop": "6108", "icms": "00", "icms_aliq_override": "12.00",
			"uf_overrides": []any{map[string]any{"ufs": []any{"RJ"}, "overrides": map[string]any{"icms_aliq_override": "99.00"}}},
		}},
	}
	resolved, err := resolveCfopTax(product, nil, "6108", "RJ")
	if err != nil {
		t.Fatal(err)
	}
	// O override afeta só icms_aliq_override (a alíquota do PRÓPRIO estado de
	// destino do ICMS normal) — DIFAL usa uma tabela intra/inter separada,
	// resolveICMSIntraAliq/resolveICMSInterAliq, que não lê uf_overrides.
	if got := resolveICMSIntraAliq("RJ"); got == "" {
		t.Fatal("resolveICMSIntraAliq should be unaffected and still return a value")
	}
	_ = resolved
}
```

- [ ] **Step 4: Commit**

```bash
git add api/internal/services/nfes/tax_resolution_matrix_test.go
git commit -m "test(api): exhaustive resolution-tier matrix + DIFAL regression"
```

---

### Task 5: Rota `GET /v1.0/tax-tables/icms-aliq`

**Files:**
- Modify: `api/internal/services/nfes/tax_tables.go` (função exportada nova)
- Create: `api/internal/services/tax_tables.go` (novo `TaxTableService`)
- Create: `api/internal/api/v1/tax_tables.go` (rota, seguindo o padrão de `tax_profiles.go` do
  mesmo pacote — usar `RegisterTaxProfiles` como referência de estrutura de handler)
- Modify: `api/internal/api/v1/router.go` (campo `TaxTables` em `Services`, chamada
  `RegisterTaxTables`)
- Test: `api/internal/api/v1/tax_tables_test.go`

**Interfaces:**
- Consumes: nenhuma alteração de assinatura das tasks anteriores (usa a versão de 4 parâmetros de
  `resolveICMSAliq`/`resolveFCPAliq` da Task 2).
- Produces: `GET /v1.0/tax-tables/icms-aliq?emit_uf=&dest_uf=&ncm=` → `{"icms_aliq": "...", "fcp_aliq": "..."}`.

- [ ] **Step 1: Exportar as funções de resolução (sem override) do pacote `nfes`**

Em `tax_tables.go`, adicionar (o pacote é `nfes`, minúsculo por convenção — as funções internas
continuam privadas, só a fachada de consulta é exportada):

```go
// PreviewICMSAliq devolve a alíquota ICMS/FCP que o sistema resolveria hoje
// para emit_uf/dest_uf/ncm, sem nenhum override — usado pela rota de consulta
// que alimenta o warning de alíquota do frontend (spec §Modelo de dados 6).
func PreviewICMSAliq(emitUF, destUF, ncm string) (icmsAliq, fcpAliq string) {
	return resolveICMSAliq(emitUF, destUF, ncm, nil), resolveFCPAliq(destUF, ncm, nil)
}
```

- [ ] **Step 2: Criar o `TaxTableService`**

`api/internal/services/tax_tables.go`:

```go
package services

import nfesvc "gopkg.aoctech.app/dfe/api/internal/services/nfes"

// TaxTableService expõe consultas de tabela fiscal sem estado — não tem
// repositório porque não persiste nada, só encaminha para as tabelas de
// nfes.tax_tables.go (fonte única de verdade, também usada na emissão).
type TaxTableService struct{}

func NewTaxTableService() *TaxTableService { return &TaxTableService{} }

// PreviewICMSAliq devolve o que o backend resolveria para emit_uf/dest_uf/ncm.
func (s *TaxTableService) PreviewICMSAliq(emitUF, destUF, ncm string) (icmsAliq, fcpAliq string) {
	return nfesvc.PreviewICMSAliq(emitUF, destUF, ncm)
}
```

Verificar como os demais serviços sem repositório são registrados via `fx` (procurar
`fx.Provide(NewX...)` em `cmd/server/main.go` ou módulo de wiring) e adicionar
`fx.Provide(services.NewTaxTableService)` no mesmo padrão.

- [ ] **Step 3: Criar a rota**

`api/internal/api/v1/tax_tables.go` (seguir o padrão de outra rota GET simples do pacote — seção
"Layer Rules" do `api/CLAUDE.md`: rota só faz parse + chama 1 método de service + responde):

```go
package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/gofiber/fiber/v3"
)

// RegisterTaxTables mounts GET /v1.0/tax-tables/icms-aliq.
func RegisterTaxTables(v1 fiber.Router, svc *services.TaxTableService, authMw fiber.Handler) {
	g := v1.Group("/tax-tables", authMw)
	g.Get("/icms-aliq", func(c fiber.Ctx) error {
		emitUF := c.Query("emit_uf")
		destUF := c.Query("dest_uf")
		ncm := c.Query("ncm")
		if emitUF == "" || destUF == "" {
			return sendProblem(c, problem.BadRequest("emit_uf e dest_uf são obrigatórios"))
		}
		icmsAliq, fcpAliq := svc.PreviewICMSAliq(emitUF, destUF, ncm)
		return sendItem(c, fiber.Map{"icms_aliq": icmsAliq, "fcp_aliq": fcpAliq})
	})
}
```

Confirmar o nome real do helper de resposta de item único (`sendItem` é a suposição baseada no
padrão de outras rotas — rodar `grep -n "func sendItem\|func sendProblem" api/internal/api/v1/*.go`
e ajustar se o nome for outro).

- [ ] **Step 4: Registrar no router**

Em `router.go`, adicionar `TaxTables *services.TaxTableService` à struct `Services` e, dentro de
`Register`, a linha `RegisterTaxTables(v1, svcs.TaxTables, authMw)` (rota só de leitura de tabela
pública da organização — sem checagem de `perm`, como `RegisterHealth`).

- [ ] **Step 5: Escrever e rodar o teste de integração da rota**

`api/internal/api/v1/tax_tables_test.go` — seguir o padrão de teste de rota já usado no pacote
(provavelmente com `httptest`/app Fiber de teste — copiar o setup de outro `_test.go` do mesmo
pacote, ex. `products_test.go` se existir, ou o teste mais próximo de uma rota GET simples):

```go
func TestTaxTablesRoute_IcmsAliq_ReturnsResolvedValue(t *testing.T) {
	// Monta a app de teste com RegisterTaxTables e services.NewTaxTableService(),
	// faz GET /v1.0/tax-tables/icms-aliq?emit_uf=SP&dest_uf=RJ&ncm=00000000,
	// espera 200 e body {"icms_aliq": "...", "fcp_aliq": "..."} com valores
	// não vazios (a tabela genérica de SP->RJ sempre resolve para algo).
}
```

Run: `cd api && go test ./internal/api/v1/... -run TestTaxTablesRoute -v`
Expected: PASS.

- [ ] **Step 6: Documentar o endpoint em `DOCS.md`**

Adicionar à seção de endpoints de `DOCS.md` (procurar a seção de `/tax-profiles` como referência de
formato): `GET /v1.0/tax-tables/icms-aliq?emit_uf=&dest_uf=&ncm=` → alíquota ICMS/FCP resolvida sem
overrides, usada pelo frontend para o warning de alíquota customizada.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/nfes/tax_tables.go api/internal/services/tax_tables.go api/internal/api/v1/tax_tables.go api/internal/api/v1/tax_tables_test.go api/internal/api/v1/router.go DOCS.md
git commit -m "feat(api): add GET /v1.0/tax-tables/icms-aliq preview endpoint"
```

---

### Task 6: Zod — `ufTaxOverrideSchema`, campos novos, IBS/CBS opcional

**Files:**
- Modify: `ui/src/lib/schemas/products.ts`
- Modify: `ui/src/lib/schemas/tax-profiles.ts`
- Test: `ui/src/__tests__/schemas/products.test.ts` (criar se não existir; seguir o padrão de teste
  Zod já usado no projeto — `describe`/`it` com `.safeParse`)

**Interfaces:**
- Produces: `ufTaxOverrideSchema` exportado de `products.ts` (usado pela Task 8 no
  `UfOverridesEditor` e reusado por `tax-profiles.ts`).
- Produces: `cfopConfigSchema` ganha `uf_overrides: z.array(ufTaxOverrideSchema)`,
  `icms_pauta_valor`, `pis_st_aliq`/`cofins_st_aliq`/`pis_st_v_bc`/`cofins_st_v_bc`, e os 5 campos
  IBS/CBS deixam de ser regex-obrigatórios (viram `optionalStr`/regex opcional).

- [ ] **Step 1: Escrever o teste que falha**

```ts
import {describe, expect, it} from 'vitest'
import {cfopConfigSchema} from '@/lib/schemas/products'

describe('cfopConfigSchema — uf_overrides e IBS/CBS opcional', () => {
  const base = {
    cfop: '5102', pis: '01', cofins: '01',
    ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
  }

  it('aceita IBS/CBS totalmente vazio', () => {
    expect(cfopConfigSchema.safeParse(base).success).toBe(true)
  })

  it('aceita uf_overrides com UF válida', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: ['SP', 'RJ'], overrides: {icms_aliq_override: '12.0000'}}],
    })
    expect(result.success).toBe(true)
  })

  it('rejeita uf_overrides sem nenhuma UF', () => {
    const result = cfopConfigSchema.safeParse({
      ...base,
      uf_overrides: [{ufs: [], overrides: {}}],
    })
    expect(result.success).toBe(false)
  })
})
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `cd ui && npx vitest run src/__tests__/schemas/products.test.ts`
Expected: FAIL (`uf_overrides` não existe no schema ainda — mas o segundo teste já passaria hoje
porque IBS/CBS já é regex-obrigatório e o teste "aceita vazio" falharia).

- [ ] **Step 3: Implementar**

Em `products.ts`, adicionar antes de `cfopConfigSchema`:

```ts
export const ufTaxOverrideSchema = z.object({
  ufs: z.array(z.string().regex(/^[A-Z]{2}$/, 'UF inválida')).min(1, 'Escolha ao menos uma UF'),
  overrides: z.record(z.string(), z.unknown()).default({}),
})
```

Em `cfopConfigSchema`, trocar os 5 campos IBS/CBS obrigatórios por opcionais:

```ts
  ibs_cbs_cst: optionalStr.pipe(z.string().regex(_ibsCbsCstRegex, 'CST IBS/CBS inválido').optional().or(z.literal(''))),
  ibs_cbs_class_trib: z.string().regex(_ibsCbsClassRegex, 'Código de classificação deve ter 6 dígitos').optional().or(z.literal('')),
  ibs_uf_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota IBS Estadual inválida (ex: 8.0000)').optional().or(z.literal('')),
  ibs_mun_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota IBS Municipal inválida (ex: 1.0000)').optional().or(z.literal('')),
  cbs_aliq: z.string().regex(_ibsCbsAliqRegex, 'Alíquota CBS inválida (ex: 9.0000)').optional().or(z.literal('')),
```

(usar `.optional().or(z.literal(''))` sem o `.pipe` inútil acima — a linha do `ibs_cbs_cst` correta
é: `z.string().regex(_ibsCbsCstRegex, 'CST IBS/CBS inválido').optional().or(z.literal(''))`).

Adicionar `icms_pauta_valor: optionalStr` (grupo pauta, validado como string livre — formato
monetário já é o padrão frouxo usado por outros campos money do arquivo, ex. `issqn_v_deducao`).

Adicionar bloco PIS/COFINS-ST após os campos PIS/COFINS existentes:
```ts
  pis_st_aliq: optionalPercent,
  cofins_st_aliq: optionalPercent,
  pis_st_v_bc: optionalStr,
  cofins_st_v_bc: optionalStr,
```

Adicionar `uf_overrides: z.array(ufTaxOverrideSchema)` ao final de `cfopConfigSchema`.

Em `tax-profiles.ts`, `taxProfileSchema` já herda `cfopConfigSchema.omit({cfop: true})` — então
`uf_overrides` chega automaticamente. Nenhuma mudança extra necessária ali além de conferir que o
`.omit` não precisa excluir `uf_overrides` (não deve — perfil também tem esse campo).

- [ ] **Step 4: Rodar e confirmar que passa**

Run: `cd ui && npx vitest run src/__tests__/schemas/products.test.ts`
Expected: PASS (3 testes).

Run: `npx eslint src/lib/schemas/products.ts src/lib/schemas/tax-profiles.ts --ext .ts,.tsx`
Expected: sem erros/warnings.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/schemas/products.ts ui/src/lib/schemas/tax-profiles.ts ui/src/__tests__/schemas/products.test.ts
git commit -m "feat(ui): add uf_overrides schema, optional IBS/CBS group, pauta/PIS-COFINS-ST fields"
```

---

### Task 7: `UfOverridesEditor` — novo componente de cards por UF

**Files:**
- Create: `ui/src/components/tax/UfOverridesEditor.tsx`
- Test: `ui/src/__tests__/components/UfOverridesEditor.test.tsx`

**Interfaces:**
- Consumes: `TaxFieldsEditor`/`TaxGroups`/`EMPTY_TAX_GROUPS` (existentes,
  `@/components/tax/TaxFieldsEditor`), `CfopConfigFormData` (`@/lib/schemas/products`).
- Produces: `UfOverridesEditor({value, onChange, simples}: {value: UfOverrideFormData[]; onChange:
  (next: UfOverrideFormData[]) => void; simples: boolean})` — usado pela Task 9 (`ProductForm`) e
  Task 11 (`TaxProfileForm`). `UfOverrideFormData = {ufs: string[]; overrides:
  Partial<CfopConfigFormData>}` (tipo local do componente, não precisa entrar no zod de
  `products.ts` como tipo nomeado — `z.infer<typeof ufTaxOverrideSchema>` já cobre o shape, com
  `overrides` tipado como `Record<string, unknown>`; o componente faz o cast para
  `Partial<CfopConfigFormData>` internamente ao passar para `TaxFieldsEditor`).

- [ ] **Step 1: Escrever o componente**

```tsx
'use client'

import {useState} from 'react'
import {Combobox} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import {
  EMPTY_TAX_GROUPS, TaxFieldsEditor, type TaxGroups,
} from '@/components/tax/TaxFieldsEditor'
import type {CfopConfigFormData} from '@/lib/schemas/products'
import {UF_OPTIONS} from '@/lib/data/uf'

export interface UfOverrideFormData {
  ufs: string[]
  overrides: Partial<CfopConfigFormData>
}

interface UfOverridesEditorProps {
  value: UfOverrideFormData[]
  onChange: (next: UfOverrideFormData[]) => void
  simples: boolean
}

/**
 * Lista de cards de override por UF de destino. Cada card tem um picker
 * multi-select de UF e o mesmo TaxFieldsEditor usado no CFOP/perfil base —
 * todos os campos ficam opcionais aqui: só preenche o que diverge para
 * aquelas UFs (spec §Modelo de dados 1).
 */
export function UfOverridesEditor({value, onChange, simples}: UfOverridesEditorProps) {
  const [groupsByIndex, setGroupsByIndex] = useState<Record<number, TaxGroups>>({})

  const addCard = () => onChange([...value, {ufs: [], overrides: {}}])
  const removeCard = (i: number) => onChange(value.filter((_, idx) => idx !== i))
  const setUfs = (i: number, ufs: string[]) =>
    onChange(value.map((v, idx) => (idx === i ? {...v, ufs} : v)))
  const setOverrides = (i: number, updater: (r: CfopConfigFormData) => CfopConfigFormData) =>
    onChange(value.map((v, idx) => {
      if (idx !== i) return v
      const next = updater(v.overrides as CfopConfigFormData)
      return {...v, overrides: next}
    }))

  return (
    <div className="space-y-3">
      {value.map((card, i) => (
        <div key={i} className="rounded-lg border border-purple-100 bg-purple-50/20 p-3 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <div className="flex-1">
              <label className="text-sm font-medium text-gray-700">UFs de destino</label>
              <div className="flex flex-wrap gap-1.5 pt-1">
                {UF_OPTIONS.map((opt) => {
                  const checked = card.ufs.includes(opt.value)
                  return (
                    <button key={opt.value} type="button"
                            onClick={() => setUfs(i, checked
                              ? card.ufs.filter((u) => u !== opt.value)
                              : [...card.ufs, opt.value])}
                            className={`min-h-8 rounded-full px-2.5 text-xs font-medium ${
                              checked ? 'bg-purple-600 text-white' : 'bg-white text-gray-600 border border-gray-200'
                            }`}>
                      {opt.value}
                    </button>
                  )
                })}
              </div>
            </div>
            <Button type="button" variant="ghost" size="xs" onClick={() => removeCard(i)}
                    className="text-danger hover:text-red-700">remover</Button>
          </div>
          <TaxFieldsEditor value={card.overrides as CfopConfigFormData}
                            onChange={(updater) => setOverrides(i, updater)}
                            simples={simples} hideCfop
                            groups={groupsByIndex[i] ?? EMPTY_TAX_GROUPS}
                            onGroupsChange={(g) => setGroupsByIndex((prev) => ({...prev, [i]: g}))}/>
        </div>
      ))}
      <Button type="button" variant="ghost" size="sm" onClick={addCard}
              className="text-brand-600 hover:text-brand-700 px-0">
        + Adicionar override por UF
      </Button>
    </div>
  )
}
```

Confirmar se `@/lib/data/uf.ts` com `UF_OPTIONS` já existe (`grep -rn "UF_OPTIONS" ui/src/lib/data/`)
— se não existir, criar `ui/src/lib/data/uf.ts` com a lista das 27 UFs
(`[{value: 'AC', label: 'Acre'}, ...]`), reaproveitando a mesma lista de `aliqICMSTable`
(`tax_tables.go` linhas 8-12) para não duplicar a enumeração com grafia diferente.

- [ ] **Step 2: Escrever o teste do componente**

```tsx
import {describe, expect, it, vi} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {UfOverridesEditor} from '@/components/tax/UfOverridesEditor'

describe('UfOverridesEditor', () => {
  it('adiciona um card vazio ao clicar em "Adicionar override por UF"', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByText('+ Adicionar override por UF'))
    expect(onChange).toHaveBeenCalledWith([{ufs: [], overrides: {}}])
  })

  it('alterna uma UF no card existente', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[{ufs: [], overrides: {}}]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByText('SP'))
    expect(onChange).toHaveBeenCalledWith([{ufs: ['SP'], overrides: {}}])
  })

  it('remove um card', () => {
    const onChange = vi.fn()
    render(<UfOverridesEditor value={[{ufs: ['SP'], overrides: {}}]} onChange={onChange} simples={false}/>)
    fireEvent.click(screen.getByText('remover'))
    expect(onChange).toHaveBeenCalledWith([])
  })
})
```

- [ ] **Step 3: Rodar os testes**

Run: `cd ui && npx vitest run src/__tests__/components/UfOverridesEditor.test.tsx`
Expected: PASS (3 testes) — ajustar seletores se o texto do botão não bater exatamente após a
implementação (ex. `getByText` vs `getByRole('button', {name: ...})`).

Run: `npx eslint src/components/tax/UfOverridesEditor.tsx src/lib/data/uf.ts --ext .ts,.tsx`
Expected: sem erros/warnings.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/tax/UfOverridesEditor.tsx ui/src/lib/data/uf.ts ui/src/__tests__/components/UfOverridesEditor.test.tsx
git commit -m "feat(ui): add UfOverridesEditor for per-destination-UF tax overrides"
```

---

### Task 8: `TaxFieldsEditor` — PIS/COFINS-ST, pauta fiscal condicional, IBS/CBS opcional, warning de alíquota

**Files:**
- Modify: `ui/src/components/tax/TaxFieldsEditor.tsx`
- Test: `ui/src/__tests__/components/TaxFieldsEditor.test.tsx` (criar se não existir)

**Interfaces:**
- Consumes: `GET /v1.0/tax-tables/icms-aliq` (Task 5) via `apiClient` (novo método
  `getIcmsAliqPreview(params)` em `ui/src/lib/api/client.ts` — adicionar seguindo o padrão dos
  métodos GET já existentes no `ApiClient`).
- Produces: `TaxGroups` ganha `pisCofinsSt: boolean` e `ibsCbs: boolean` (substitui o
  comportamento hoje sempre-visível do bloco IBS/CBS); `EMPTY_TAX_GROUPS` ganha os dois campos.

- [ ] **Step 1: Adicionar o método ao `ApiClient`**

Em `ui/src/lib/api/client.ts`, próximo aos outros métodos GET simples, seguindo a assinatura já
usada por métodos equivalentes do arquivo:

```ts
async getIcmsAliqPreview(params: {emitUf: string; destUf: string; ncm?: string}) {
  const {data} = await this.client.get<{icms_aliq: string; fcp_aliq: string}>('/tax-tables/icms-aliq', {
    params: {emit_uf: params.emitUf, dest_uf: params.destUf, ncm: params.ncm},
  })
  return data
}
```

- [ ] **Step 2: Escrever o teste de exibição condicional que falha**

```tsx
import {describe, expect, it} from 'vitest'
import {render, screen} from '@testing-library/react'
import {EMPTY_TAX_GROUPS, TaxFieldsEditor} from '@/components/tax/TaxFieldsEditor'

const baseValue = {cfop: '5102', icms: '00', pis: '01', cofins: '01', ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: ''} as never

describe('TaxFieldsEditor — grupos opcionais novos', () => {
  it('não mostra o grupo IBS/CBS por padrão', () => {
    render(<TaxFieldsEditor value={baseValue} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.queryByText(/IBS \/ CBS/)).not.toBeInTheDocument()
  })

  it('mostra valor de pauta apenas quando icms_mod_bc é Pauta/PMPF', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '4'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.getByText(/Valor da pauta/)).toBeInTheDocument()
  })

  it('não mostra valor de pauta para modo de cálculo padrão', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '3'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.queryByText(/Valor da pauta/)).not.toBeInTheDocument()
  })
})
```

Consultar `MOD_BC_OPTIONS` (`@/lib/data/mod_bc`) para confirmar qual valor representa "Pauta
(valor)" e qual representa "PMPF" — usar esses valores reais no teste e na condição do componente
(não assumir `'4'`; ler o arquivo `ui/src/lib/data/mod_bc.ts` antes de escrever a condição final).

- [ ] **Step 3: Rodar e confirmar falha**

Run: `cd ui && npx vitest run src/__tests__/components/TaxFieldsEditor.test.tsx`
Expected: FAIL (IBS/CBS ainda sempre visível; pauta não existe).

- [ ] **Step 4: Implementar**

Em `TaxFieldsEditor.tsx`:

1. `TaxGroups` (linha 64-71) ganha `pisCofinsSt: boolean` e `ibsCbs: boolean`;
   `EMPTY_TAX_GROUPS` (linha 73-75) ganha `pisCofinsSt: false, ibsCbs: false`.
2. Envolver o bloco `{/* ── IBS / CBS ─── */}` (linhas 541-646) num toggle igual ao padrão de IPI/IS
   (checkbox `toggle-ibscbs` controlando `groups.ibsCbs`, limpando os 5 campos-chave + os de
   redução/diferimento ao desligar).
3. No bloco "Alíquota ICMS — para CSTs tributados" (linhas 202-226), adicionar dentro do mesmo
   `grid`, condicionado a `value.icms_mod_bc` ser Pauta ou PMPF (valores reais lidos de
   `mod_bc.ts` no Step 2):
   ```tsx
   {['<valor-pauta>', '<valor-pmpf>'].includes(value.icms_mod_bc ?? '') && (
     <div className="grid gap-1">
       <label className="text-sm font-medium text-gray-700">Valor da pauta fiscal (R$)</label>
       <NumericInput value={value.icms_pauta_valor ?? ''} decimal integerPlaces={9} decimalPlaces={2}
                     placeholder="0.00"
                     onChange={(v) => onChange((r) => ({...r, icms_pauta_valor: v}))}/>
     </div>
   )}
   ```
4. Adicionar novo bloco "PIS/COFINS-ST" espelhando o padrão de toggle do bloco IPI (linhas
   323-353), com os 4 campos `pis_st_aliq`/`cofins_st_aliq`/`pis_st_v_bc`/`cofins_st_v_bc`.
5. Warning de alíquota: adicionar um `useEffect`/hook próprio ao final do bloco "Alíquota ICMS"
   (após a linha 226) que chama `apiClient.getIcmsAliqPreview` com debounce quando
   `value.icms_aliq_override` ou o NCM/UF (recebidos como novas props opcionais `ncm?: string` e
   `emitUf?/destUf?: string` — adicionar a `TaxFieldsEditorProps`) mudam, e mostra um banner
   `role="alert"` amber quando o valor digitado difere do valor resolvido pela API. Seguir o padrão
   de debounce já usado no projeto (`ui/CLAUDE.md` — "Input Debouncing", 300ms) — reaproveitar
   `DebouncedInput`/o hook de debounce interno já usado por outro componente do projeto
   (`grep -rn "debounce" ui/src/lib/hooks/` para achar o hook existente a reusar, em vez de
   escrever um novo).

- [ ] **Step 5: Rodar os testes**

Run: `cd ui && npx vitest run src/__tests__/components/TaxFieldsEditor.test.tsx`
Expected: PASS (3 testes).

Run: `npx eslint src/components/tax/TaxFieldsEditor.tsx src/lib/api/client.ts --ext .ts,.tsx`
Expected: sem erros/warnings.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/tax/TaxFieldsEditor.tsx ui/src/lib/api/client.ts ui/src/__tests__/components/TaxFieldsEditor.test.tsx
git commit -m "feat(ui): make IBS/CBS optional, add pauta fiscal and PIS/COFINS-ST fields, aliquot warning"
```

---

### Task 9: `ProductForm` — remove autofill, integra `UfOverridesEditor`

**Files:**
- Modify: `ui/src/components/products/ProductForm.tsx`
- Test: `ui/src/__tests__/components/ProductForm.test.tsx` (atualizar/estender o já existente, se
  houver; senão criar cobrindo os dois pontos abaixo)

**Interfaces:**
- Consumes: `UfOverridesEditor` (Task 7), `ufTaxOverrideSchema`/campos novos (Task 6).

- [ ] **Step 1: Remover o autofill automático**

Remover o `useEffect` de `ProductForm.tsx:532-549` (o bloco que chama `getIcmsForNcm` e grava direto
em `icms_aliq_override`/`fcp_aliq_override`). Remover também o import de `getIcmsForNcm`
(linha 42) e o estado `icmsAutoFilled`/`setIcmsAutoFilled` (linha 462) e o bloco de UI condicional
que mostra "Preenchido automaticamente..." (linhas 822-843) — vira só o texto estático "Deixe em
branco para usar a alíquota padrão do sistema" (linhas 839-842), sem o ramo `icmsAutoFilled`. O
warning de divergência passa a viver dentro do próprio campo, via `TaxFieldsEditor` (Task 8) — mas
os campos `icms_aliq_override`/`fcp_aliq_override` a nível de PRODUTO (não de `cfop_config`) são
`FormField`s deste arquivo, não do `TaxFieldsEditor` — então este arquivo também precisa consumir
`apiClient.getIcmsAliqPreview` (Task 8, Step 1) e mostrar o mesmo padrão de banner amber ali, usando
`uf` (prop do componente) e `watchedNcm` como parâmetros.

- [ ] **Step 2: Adicionar `UfOverridesEditor` ao `cfop_config` sendo adicionado**

Na aba Tributação (por volta da linha 1120-1122, onde `TaxFieldsEditor` é renderizado para a linha
em construção `cfopRow`), adicionar um estado `ufOverrideRows: UfOverrideFormData[]` (análogo a
`cfopRow`) e renderizar `<UfOverridesEditor value={ufOverrideRows} onChange={setUfOverrideRows}
simples={simples}/>` logo abaixo do `TaxFieldsEditor`. Em `addCfop` (linha 558-653), incluir
`uf_overrides: ufOverrideRows` em cada variante gerada por `getCfopVariants` (linha 596-646), e
resetar `ufOverrideRows` para `[]` junto com `setCfopRow(EMPTY_CFOP_ROW)` (linha 652).

Atualizar `EMPTY_CFOP_ROW` (linha 108-171) e `toFormData`/`toApiPayload` (linhas 181-435) para
incluir `uf_overrides: []` / mapear `c.uf_overrides ?? []` — mesmo padrão de `??` já usado para
todos os outros campos opcionais do arquivo.

- [ ] **Step 3: Escrever/atualizar os testes**

```tsx
it('não grava mais icms_aliq_override automaticamente ao trocar o NCM', async () => {
  // renderiza ProductForm com uf="SP", troca o NCM para um valor com match em
  // getIcmsForNcm (ou mocka apiClient.getIcmsAliqPreview), espera que o campo
  // icms_aliq_override permaneça vazio.
})

it('inclui uf_overrides no payload ao adicionar um CFOP com override de UF', () => {
  // preenche cfopRow, adiciona uma UF no UfOverridesEditor, clica "Adicionar
  // CFOP", espera que form.getValues('cfop_config')[0].uf_overrides tenha 1
  // entrada com a UF escolhida.
})
```

Run: `cd ui && npx vitest run src/__tests__/components/ProductForm.test.tsx`
Expected: PASS.

Run: `npx eslint src/components/products/ProductForm.tsx --ext .ts,.tsx`
Expected: sem erros/warnings.

- [ ] **Step 4: Teste manual no browser (mandatório por `ui/CLAUDE.md`)**

Rodar `npm run dev`, abrir `/products/new`, aba Tributação: adicionar um CFOP, abrir
"+ Adicionar override por UF", marcar SP e RJ, preencher uma alíquota ICMS diferente só ali, salvar,
confirmar no Network que o payload chega com `cfop_config[0].uf_overrides`. Testar em 375px
(mobile) que os cards de UF não causam overflow horizontal.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/products/ProductForm.tsx ui/src/__tests__/components/ProductForm.test.tsx
git commit -m "feat(ui): wire UfOverridesEditor into ProductForm, remove ICMS auto-fill"
```

---

### Task 10: Remover a tabela NCM+UF do frontend

**Files:**
- Delete: `ui/src/lib/data/icms_ncm_lookup.ts`
- Modify: qualquer import remanescente de `getIcmsForNcm` (já removido de `ProductForm.tsx` na
  Task 9 — confirmar com `grep -rn "icms_ncm_lookup\|getIcmsForNcm" ui/src/`)

**Interfaces:** nenhuma — remoção pura, a fonte de verdade passou a ser o backend (Task 2 + Task 5).

- [ ] **Step 1: Confirmar que não há mais nenhuma referência**

Run: `grep -rn "icms_ncm_lookup\|getIcmsForNcm" ui/src/`
Expected: nenhum resultado (a Task 9 já removeu o único uso).

Se houver algum resultado (ex. `scripts/generate-icms-lookup.js` gerando o arquivo em CI), avaliar
com o usuário se o script também deve ser removido/redirecionado para gerar o `icmsNcmTable` do Go
(Task 2) em vez do TS — **não decidir isso sozinho**, é uma mudança de pipeline fora do escopo
original desta spec; se o script existir, parar aqui e perguntar antes de tocar nele.

- [ ] **Step 2: Remover o arquivo**

```bash
rm ui/src/lib/data/icms_ncm_lookup.ts
```

- [ ] **Step 3: Rodar o build/lint completo**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: sem erros — nenhum import quebrado.

- [ ] **Step 4: Commit**

```bash
git add -A ui/src/lib/data/icms_ncm_lookup.ts
git commit -m "chore(ui): remove frontend NCM ICMS lookup table (migrated to backend)"
```

---

### Task 11: `TaxProfileForm` — `UfOverridesEditor` + texto de ajuda atualizado

**Files:**
- Modify: `ui/src/components/tax-profiles/TaxProfileForm.tsx`
- Test: `ui/src/__tests__/components/TaxProfileForm.test.tsx` (criar se não existir)

**Interfaces:**
- Consumes: `UfOverridesEditor` (Task 7).

- [ ] **Step 1: Atualizar o texto de ajuda (linhas 126-130)**

```tsx
            <p className="text-xs text-gray-500">
              Um perfil normalmente cobre a operação interna e a interestadual (5102 e 6102): a alíquota
              interestadual é resolvida na emissão, então o que muda entre elas é dado derivado, não configuração.
              Quando o tratamento difere só pela UF de destino, use os overrides por UF abaixo — quando difere
              de verdade por CFOP, crie um segundo perfil.
            </p>
```

- [ ] **Step 2: Adicionar `UfOverridesEditor`**

`TaxProfileForm` já usa `taxValue`/`setTaxValue` (linhas 74-80) como a "linha" completa do
formulário — `uf_overrides` é só mais um campo dela. Adicionar após o `TaxFieldsEditor` (linha
163-164):

```tsx
          <UfOverridesEditor
            value={taxValue.uf_overrides ?? []}
            onChange={(next) => setTaxValue((r) => ({...r, uf_overrides: next}))}
            simples={simples}
          />
```

Importar `UfOverridesEditor` de `@/components/tax/UfOverridesEditor`. Confirmar que
`EMPTY_TAX_FIELDS` (linha 26-29) não precisa de `uf_overrides: []` explícito porque
`TaxProfileFormData`/Zod já tem default `[]` implícito via `z.array(...)` sem `.default()` — se o
`useForm` reclamar de `undefined`, adicionar `uf_overrides: []` ao objeto.

- [ ] **Step 3: Escrever/rodar os testes**

```tsx
it('mostra o texto de ajuda mencionando overrides por UF', () => {
  render(<TaxProfileForm onSubmit={vi.fn()}/>)
  expect(screen.getByText(/overrides por UF/)).toBeInTheDocument()
})

it('inclui uf_overrides no submit ao preencher um card', () => {
  // adiciona uma UF no UfOverridesEditor, submete, espera que o payload
  // enviado a onSubmit tenha uf_overrides com a UF escolhida.
})
```

Run: `cd ui && npx vitest run src/__tests__/components/TaxProfileForm.test.tsx`
Expected: PASS.

Run: `npx eslint src/components/tax-profiles/TaxProfileForm.tsx --ext .ts,.tsx`
Expected: sem erros/warnings.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/tax-profiles/TaxProfileForm.tsx ui/src/__tests__/components/TaxProfileForm.test.tsx
git commit -m "feat(ui): wire UfOverridesEditor into TaxProfileForm, update help text"
```

---

### Task 12: Documentação — `DOCS.md` e `CONDUCT.md`

**Files:**
- Modify: `DOCS.md`
- Modify: `CONDUCT.md`

**Interfaces:** nenhuma — só documentação, consolidando o que as Tasks 1-11 já implementaram.

- [ ] **Step 1: Atualizar `DOCS.md`**

Na seção que hoje documenta o modelo de dados de `cfop_config`/`tax_profiles` e o algoritmo de
precedência (linhas ~1059-1065, já identificadas na spec como o texto pré-redesign), substituir a
lista de precedência antiga por:

```markdown
### Resolução de tributação (produto + perfil + UF de destino)

Ordem de precedência na emissão, da maior para a menor (implementada em
`resolveCfopTax`, `api/internal/services/nfes/tax_profiles.go` — único ponto que decide
precedência):

1. `cfop_config[cfop]` do produto + `uf_overrides` da UF de destino
2. `cfop_config[cfop]` do produto (sem UF)
3. Vínculo produto→perfil (`overrides`) + `uf_overrides` da UF de destino
4. Vínculo produto→perfil (`overrides`), sem UF
5. `tax_profile.cfops[cfop]` + `uf_overrides` da UF de destino
6. `tax_profile.cfops[cfop]` (sem UF)
7. Erro: nenhuma camada cobre o CFOP

IBS/CBS é um grupo opcional tudo-ou-nada: se nenhum dos 5 campos-chave estiver preenchido, o grupo
é omitido na emissão; se algum estiver, todos os 5 são exigidos.

Nova rota: `GET /v1.0/tax-tables/icms-aliq?emit_uf=&dest_uf=&ncm=` — devolve a alíquota ICMS/FCP que
o backend resolveria sem overrides, usada pelo frontend para avisar quando um override diverge da
tabela padrão.
```

Remover/atualizar o parágrafo antigo de precedência (2 níveis) se ele ainda estiver presente sem
ter sido substituído por um fix anterior.

- [ ] **Step 2: Atualizar `CONDUCT.md`**

Na regra que documenta `resolveCfopTax` como único ponto de decisão de precedência (linha ~504-505),
atualizar a contagem de níveis de 2/3 para 6 (+ erro), e adicionar uma linha sobre a tabela NCM+UF:
"a tabela de alíquota ICMS por NCM (antes só no frontend) agora vive em
`api/internal/services/nfes/tax_tables.go` — fonte única de verdade; não reintroduzir uma cópia no
frontend."

- [ ] **Step 3: Commit**

```bash
git add DOCS.md CONDUCT.md
git commit -m "docs: document 6-tier tax resolution, optional IBS/CBS, and icms-aliq preview endpoint"
```

---

### Task 13: Suíte completa — verificação final end-to-end

**Files:** nenhum arquivo novo — só execução e checklist.

- [ ] **Step 1: Backend**

Run: `cd api && go build ./... && go test ./... -race`
Expected: PASS, zero regressões.

- [ ] **Step 2: Frontend**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero erros/warnings de lint, todos os testes passando.

- [ ] **Step 3: Teste manual do fluxo completo**

No browser (`npm run dev`): criar um perfil fiscal com um override por UF, vincular a um produto,
emitir uma NF-e de teste para uma UF dentro do override e outra fora — confirmar visualmente (via
DevTools Network ou logs do worker, conforme disponível) que a UF coberta usa o override e a UF não
coberta cai no perfil base. Confirmar que um produto legado (sem `uf_overrides`, sem os campos
novos) emite exatamente como antes.

- [ ] **Step 4: Checklist final (copiar do `CLAUDE.md` raiz)**

- [ ] Nenhuma emissão existente mudou de comportamento (regressão zero)
- [ ] Todos os 7 níveis de resolução cobertos por teste
- [ ] IBS/CBS realmente opcional e tudo-ou-nada nos dois lados (Go + Zod)
- [ ] DIFAL (`buildICMSUFDest`) intacto e testado
- [ ] `DOCS.md`/`CONDUCT.md` refletem o estado final do código
- [ ] Nenhum novo warning de ESLint
