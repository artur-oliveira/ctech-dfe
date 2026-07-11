# Cadastro de Pessoas/Organizações — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tighten backend validation (CRT/IE required for PJ organizations, person CPF/CNPJ dedup),
collapse optional fields into a progressive-disclosure "advanced" section shared by
persons/organizations, and add NF-e local de entrega/retirada with reuse-from-history.

**Architecture:** Backend validation lives in `PersonService`/`OrganizationService` (Create/Update).
`NfeLocalBody` is a new nested struct on `NfeEmitBody`, built into the XML via a new
`buildLocal` helper in `builders_doc.go`, persisted for reuse as plain attributes on the existing
`organization_persons`/`organizations` items (no new tables/GSIs). Frontend: `EntityForm.tsx`
gains a collapsible section (mirrors `VehicleForm.tsx`'s pattern) and a new `organizationSchema`;
`NfeEmitForm.tsx` gains an entrega/retirada picker fed by the person/org record.

**Tech Stack:** Go/Fiber v3/DynamoDB (api), Next.js/TS/Zod/react-hook-form (ui). No py-dfe or cdk
changes — `xsd_order.py` already orders `retirada`/`entrega`; new fields are plain optional
attributes on existing DynamoDB items.

## Global Constraints

- No `Co-Authored-By:` trailer on any commit.
- `api`: errors via `problem.*` helpers only; layer separation (route→service→repo) strict.
- `ui`: `npx eslint src --ext .ts,.tsx` zero errors/warnings; mobile-first; loading states on all
  async ops; inputs triggering API calls debounced 300ms.
- DynamoDB: no Scans; `Query`/`GetItem` only.
- CT-e is out of scope — no backend service exists yet (`api/internal/services/ctes` absent).

---

### Task 1: Backend — CRT/IE required validation for PJ

**Files:**
- Modify: `api/internal/services/persons.go`
- Modify: `api/internal/services/organizations.go`
- Test: `api/internal/services/persons_test.go` (new or extend)
- Test: `api/internal/services/organizations_test.go` (new or extend)

**Interfaces:**
- Produces: `services.RequirePJFields(cpfOrCNPJ string, crt *int) error` (shared helper, both
  services call it) and `services.RequireOrgIE(cpfOrCNPJ string, stateRegs []StateRegistrationBody) error`.

- [ ] **Step 1: Write failing tests**

`api/internal/services/persons_test.go`:
```go
package services

import (
	"testing"

	v1 "github.com/artur-oliveira/ctech-dfe/api/internal/api/v1"
)

func TestRequirePJFields_CNPJWithoutCRT_ReturnsError(t *testing.T) {
	err := RequirePJFields("11222333000181", nil)
	if err == nil {
		t.Fatal("expected error for CNPJ without CRT")
	}
}

func TestRequirePJFields_CNPJWithCRT_OK(t *testing.T) {
	crt := 1
	if err := RequirePJFields("11222333000181", &crt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequirePJFields_CPF_NoCRTRequired(t *testing.T) {
	if err := RequirePJFields("11122233344", nil); err != nil {
		t.Fatalf("CPF should not require CRT: %v", err)
	}
}

var _ = v1.PersonCreateBody{} // keep import if unused elsewhere in package tests
```

(If `v1` import ends up unused after adding real assertions, drop the blank var line — go vet
will catch it.)

`api/internal/services/organizations_test.go`:
```go
package services

import "testing"

func TestRequireOrgIE_CNPJWithoutStateRegistrations_ReturnsError(t *testing.T) {
	err := RequireOrgIE("11222333000181", nil)
	if err == nil {
		t.Fatal("expected error for CNPJ org without state_registrations")
	}
}

func TestRequireOrgIE_CNPJWithStateRegistration_OK(t *testing.T) {
	regs := []StateRegistrationEntry{{UF: "SP", StateRegistration: "123456"}}
	if err := RequireOrgIE("11222333000181", regs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireOrgIE_CPF_NoIERequired(t *testing.T) {
	if err := RequireOrgIE("11122233344", nil); err != nil {
		t.Fatalf("CPF org should not require IE: %v", err)
	}
}
```

- [ ] **Step 2: Run tests, confirm fail**

Run: `cd api && go test ./internal/services/... -run 'TestRequirePJFields|TestRequireOrgIE' -v`
Expected: FAIL — `RequirePJFields`/`RequireOrgIE`/`StateRegistrationEntry` undefined.

- [ ] **Step 3: Implement**

In `api/internal/services/persons.go`, add near the top (after `cnpjRe`/`cpfRe`):
```go
// StateRegistrationEntry is the plain-value shape of a state_registrations
// entry, shared by RequireOrgIE and any caller that already has decoded data.
type StateRegistrationEntry struct {
	UF                string
	StateRegistration string
}

// RequirePJFields returns a BadRequest problem if cpfOrCNPJ is a CNPJ (14
// digits) and crt is nil. CPF documents (pessoa física) never require CRT.
func RequirePJFields(cpfOrCNPJ string, crt *int) error {
	v := strings.NewReplacer(".", "", "-", "", "/", "").Replace(cpfOrCNPJ)
	if cnpjRe.MatchString(v) && crt == nil {
		return problem.BadRequest("crt é obrigatório para pessoa jurídica")
	}
	return nil
}
```

In `api/internal/services/organizations.go`, add:
```go
// RequireOrgIE returns a BadRequest problem if cpfOrCNPJ is a CNPJ and regs
// is empty. Organizations (always the fiscal emitter) must declare at least
// one state registration; persons (destinatário/counterparty) are exempt —
// IE-when-contribuinte is a per-emission choice (indIEDest), not a cadastro
// requirement. See docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md.
func RequireOrgIE(cpfOrCNPJ string, regs []StateRegistrationEntry) error {
	v := strings.NewReplacer(".", "", "-", "", "/", "").Replace(cpfOrCNPJ)
	if len(v) == 14 && len(regs) == 0 {
		return problem.BadRequest("ao menos uma inscrição estadual é obrigatória para organização com CNPJ")
	}
	return nil
}
```
Add `"strings"` and `"github.com/artur-oliveira/ctech-dfe/api/internal/problem"` imports to
`organizations.go` if not already present (check the file first — `persons.go` already imports
both).

- [ ] **Step 4: Run tests, confirm pass**

Run: `cd api && go test ./internal/services/... -run 'TestRequirePJFields|TestRequireOrgIE' -v`
Expected: PASS

- [ ] **Step 5: Wire into Create/Update call sites**

`api/internal/api/v1/persons.go` and `api/internal/api/v1/organizations.go` are the route
handlers that decode `PersonCreateBody`/`OrganizationCreateBody` into `fields map[string]any`
before calling the service. Read both files first to find the exact decode point (search for
`PersonCreateBody{}` / `bindJSON`). Add, right after successful body binding and before the
service call:

Persons route (Create handler):
```go
if err := services.RequirePJFields(body.CpfOrCnpj, body.Person.Crt); err != nil {
	return sendProblem(c, err)
}
```

Organizations route (Create handler):
```go
if err := services.RequirePJFields(body.CpfOrCnpj, body.Person.Crt); err != nil {
	return sendProblem(c, err)
}
regs := make([]services.StateRegistrationEntry, len(body.Person.StateRegistrations))
for i, r := range body.Person.StateRegistrations {
	regs[i] = services.StateRegistrationEntry{UF: r.UF, StateRegistration: r.StateRegistration}
}
if err := services.RequireOrgIE(body.CpfOrCnpj, regs); err != nil {
	return sendProblem(c, err)
}
```

For the **Update** handlers (`PersonUpdateBody`/`OrganizationUpdateBody`), both are partial —
`Person *PersonObjectBody` may be nil (caller not touching those fields) or `Crt`/
`StateRegistrations` may be omitted while other fields change. The correct check needs the
**merged** result, not just the patch. Simplest correct approach: after `service.Update(...)`
returns the updated item, decode `crt`/`state_registrations` back out of the returned
`map[string]types.AttributeValue` via `attributevalue.Unmarshal` and validate; if invalid, this
means the update produced a bad state — but the write already happened. To avoid a bad
write-then-reject, validate **before** calling Update by fetching current state first:

```go
// Organizations Update handler, before calling svc.Update:
if body.Person != nil {
	current, err := orgSvc.Get(c.Context(), orgPK)
	if err != nil {
		return sendProblem(c, err)
	}
	currentCrt, currentRegs := extractCrtAndRegs(current) // small local helper, attributevalue.Unmarshal
	crt := currentCrt
	if body.Person.Crt != nil {
		crt = body.Person.Crt
	}
	regs := currentRegs
	if body.Person.StateRegistrations != nil {
		regs = make([]services.StateRegistrationEntry, len(body.Person.StateRegistrations))
		for i, r := range body.Person.StateRegistrations {
			regs[i] = services.StateRegistrationEntry{UF: r.UF, StateRegistration: r.StateRegistration}
		}
	}
	if err := services.RequirePJFields(orgPK, crt); err != nil {
		return sendProblem(c, err)
	}
	if err := services.RequireOrgIE(orgPK, regs); err != nil {
		return sendProblem(c, err)
	}
}
```
Mirror the same pre-fetch pattern (without the IE check) for the Persons Update handler. Write
`extractCrtAndRegs` once as an unexported helper in `api/internal/api/v1/organizations.go` (used
by both org and person update handlers — if persons.go needs it too, move it to a shared
`api/internal/api/v1/dto.go`-adjacent helper file instead of duplicating).

- [ ] **Step 6: Add route-level regression tests**

Add to `api/tests/integration/persons_test.go` (or organizations_test.go): `POST /organizations`
with `cpf_or_cnpj` = valid CNPJ, `person.crt = null`, `person.state_registrations = []` → expect
400. `POST /persons` with CNPJ and `crt = null` → expect 400 (IE not required for persons, only
CRT). Follow the existing integration test setup pattern in that file (look at an existing
`TestOrganization_Create*`/`TestPerson_Create*` test for the HTTP harness boilerplate).

- [ ] **Step 7: Run full suite, commit**

Run: `cd api && go build ./... && go test ./... -race`
Expected: PASS

```bash
git add api/internal/services/persons.go api/internal/services/organizations.go \
        api/internal/api/v1/persons.go api/internal/api/v1/organizations.go \
        api/internal/services/persons_test.go api/internal/services/organizations_test.go \
        api/tests/integration/persons_test.go api/tests/integration/organizations_test.go
git commit -m "feat(api): require CRT for PJ and IE for CNPJ organizations"
```

---

### Task 2: Backend — person CPF/CNPJ dedup (409 on Create)

**Files:**
- Modify: `api/internal/repositories/base.go`
- Modify: `api/internal/repositories/persons.go`
- Modify: `api/internal/services/persons.go`
- Test: `api/internal/repositories/persons_test.go` (new)
- Test: `api/tests/integration/persons_test.go`

**Interfaces:**
- Produces: `repositories.IsConditionFailed(err error) bool` (exported), used by the service layer.
- Produces: `PersonRepository.BuildCreateTxItem` now conditions on `attribute_not_exists(pk)`.

- [ ] **Step 1: Write failing test**

`api/internal/repositories/persons_test.go` (extends the existing file from the vehicle work —
check it exists first via `ls api/internal/repositories/persons_test.go`; if absent, create it):
```go
package repositories

import "testing"

func TestBuildCreateTxItem_UsesConditionalPut(t *testing.T) {
	repo := &PersonRepository{Base: NewBase(nil, testConfig(), "organization_persons")}
	txItem, _ := repo.BuildCreateTxItem("ORG_1", "CPF_11122233344", map[string]any{"name": "Test"})
	if txItem.Put == nil {
		t.Fatal("expected Put transact item")
	}
	if txItem.Put.ConditionExpression == nil || *txItem.Put.ConditionExpression != "attribute_not_exists(pk)" {
		t.Fatalf("expected attribute_not_exists(pk) condition, got %v", txItem.Put.ConditionExpression)
	}
}
```
(If a `testConfig()` helper doesn't already exist in that package's tests, check
`vehicles_test.go` from the prior vehicle task — it defined one; reuse it, don't redefine.)

- [ ] **Step 2: Run, confirm fail**

Run: `cd api && go test ./internal/repositories/... -run TestBuildCreateTxItem_UsesConditionalPut -v`
Expected: FAIL (ConditionExpression is nil today).

- [ ] **Step 3: Implement**

`api/internal/repositories/base.go` — add next to `BuildPutTxItem` (line ~189):
```go
// BuildPutTxItemIfAbsent is like BuildPutTxItem but fails the transaction if
// an item with the same key already exists — used for create-only semantics
// (e.g. person dedup by CPF/CNPJ) instead of the default overwrite-on-put.
func (b *Base) BuildPutTxItemIfAbsent(item map[string]types.AttributeValue) types.TransactWriteItem {
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName:           aws.String(b.TableName),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(pk)"),
		},
	}
}

// IsConditionFailed reports whether err represents a DynamoDB conditional
// check failure, either from a single-item call or from within a
// TransactWrite (TransactionCanceledException wrapping a condition failure).
// Exported for the services layer to translate into problem.Conflict.
func IsConditionFailed(err error) bool {
	return isConditionFailed(err) || isTransactionCanceled(err)
}
```

`api/internal/repositories/persons.go` — change `BuildCreateTxItem` (line 101) from
`return r.BuildPutTxItem(item), item` to `return r.BuildPutTxItemIfAbsent(item), item`.

`api/internal/services/persons.go` — in `Create` (line 99), wrap the `TransactWrite` error:
```go
if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{personTx, auditTx}); err != nil {
	if repositories.IsConditionFailed(err) {
		return nil, problem.Conflict("pessoa com este CPF/CNPJ já cadastrada")
	}
	return nil, err
}
```

- [ ] **Step 4: Run, confirm pass**

Run: `cd api && go test ./internal/repositories/... ./internal/services/... -v -run 'TestBuildCreateTxItem_UsesConditionalPut|TestPersonSvc'`
Expected: PASS

- [ ] **Step 5: Integration test — real dedup against DynamoDB Local**

Add to `api/tests/integration/persons_test.go`:
```go
func TestPerson_CreateDuplicateCpfCnpj_Returns409(t *testing.T) {
	// ... use the existing integration harness (see an existing TestPerson_Create test in
	// this file for the HTTP client + org setup boilerplate) ...
	// 1. POST /persons with cpf_or_cnpj=11122233344 → expect 200/201
	// 2. POST /persons again with the same cpf_or_cnpj → expect 409
}
```

- [ ] **Step 6: Run full suite (including integration), commit**

Run: `cd api && go build ./... && go test ./... -race`
Run integration (DynamoDB Local on the port established in the prior session — see
`api/tests/integration/setup_test.go` for `DYNAMODB_ENDPOINT`):
`cd api && go test ./tests/integration/... -tags=integration -run TestPerson_CreateDuplicateCpfCnpj -v`
Expected: PASS

```bash
git add api/internal/repositories/base.go api/internal/repositories/persons.go \
        api/internal/services/persons.go api/internal/repositories/persons_test.go \
        api/tests/integration/persons_test.go
git commit -m "fix(api): reject duplicate CPF/CNPJ on person create"
```

---

### Task 3: Backend — NF-e local de entrega/retirada (DTO + builder)

**Files:**
- Modify: `api/internal/services/nfes/emit.go`
- Modify: `api/internal/services/nfes/builders_doc.go`
- Test: `api/internal/services/nfes/builders_doc_test.go`
- Test: `api/internal/services/nfes/emit_test.go`

**Interfaces:**
- Produces: `NfeLocalBody` struct, `NfeEmitBody.{Retirada,Entrega,SaveRetiradaLocation,SaveEntregaLocation}`.
- Produces: `buildLocal(l *NfeLocalBody) map[string]any`.
- Consumes: `cPaisBrasil`/`xPaisBrasil` constants (`builders_doc.go:34-35`), `anyStr`/`anyStrPtr` helpers (`builders_doc.go:87-100`).

- [ ] **Step 1: Write failing test**

`api/internal/services/nfes/builders_doc_test.go` — append:
```go
func TestBuildLocal_FullFields(t *testing.T) {
	cnpj := "11222333000181"
	xNome := "Depósito Sul"
	fone := "11988887777"
	email := "deposito@example.com"
	xCpl := "Galpão 3"
	l := &NfeLocalBody{
		CNPJ: &cnpj, XNome: &xNome, Fone: &fone, Email: &email, XCpl: &xCpl,
		XLgr: "Rua das Flores", Nro: "100", XBairro: "Centro",
		CMun: "3550308", XMun: "São Paulo", UF: "SP",
	}
	got := buildLocal(l)
	if got["CNPJ"] != cnpj || got["xNome"] != xNome || got["xLgr"] != "Rua das Flores" {
		t.Fatalf("unexpected build: %+v", got)
	}
	if _, hasCEP := got["CEP"]; hasCEP {
		t.Fatal("TLocal must not include CEP — that's a TEndereco-only field")
	}
	if got["cPais"] != cPaisBrasil || got["xPais"] != xPaisBrasil {
		t.Fatalf("expected default Brazil country, got %+v", got)
	}
}

func TestBuildLocal_NilReturnsNil(t *testing.T) {
	if buildLocal(nil) != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestBuildLocal_OmitsEmptyOptionalFields(t *testing.T) {
	l := &NfeLocalBody{XLgr: "Rua X", Nro: "1", XBairro: "B", CMun: "3550308", XMun: "SP", UF: "SP"}
	got := buildLocal(l)
	for _, k := range []string{"CNPJ", "CPF", "xNome", "xCpl", "fone", "email"} {
		if _, ok := got[k]; ok {
			t.Fatalf("expected %s omitted when empty, got %+v", k, got)
		}
	}
}
```

- [ ] **Step 2: Run, confirm fail**

Run: `cd api && go test ./internal/services/nfes/... -run TestBuildLocal -v`
Expected: FAIL — `NfeLocalBody`/`buildLocal` undefined.

- [ ] **Step 3: Implement DTO**

`api/internal/services/nfes/emit.go` — add after `NfeDuplicataItem` (line 99):
```go
// NfeLocalBody is a TLocal-shaped address (local de retirada/entrega) — a
// lighter shape than TEndereco (AddressBody): no CEP/postal code in the XSD.
type NfeLocalBody struct {
	CNPJ    *string `json:"cnpj" validate:"omitempty,cnpj"`
	CPF     *string `json:"cpf" validate:"omitempty,cpf"`
	XNome   *string `json:"x_nome" validate:"omitempty,max=60"`
	XLgr    string  `json:"x_lgr" validate:"required,max=255"`
	Nro     string  `json:"nro" validate:"required,max=60"`
	XCpl    *string `json:"x_cpl" validate:"omitempty,max=60"`
	XBairro string  `json:"x_bairro" validate:"required,max=60"`
	CMun    string  `json:"c_mun" validate:"required,ibge"`
	XMun    string  `json:"x_mun" validate:"required,max=60"`
	UF      string  `json:"uf" validate:"required,uf"`
	Fone    *string `json:"fone" validate:"omitempty,phonebr"`
	Email   *string `json:"email" validate:"omitempty,email"`
}
```
Add to `NfeEmitBody` (after `VTroco`, line 39):
```go
	Retirada             *NfeLocalBody `json:"retirada" validate:"omitempty"`
	Entrega              *NfeLocalBody `json:"entrega" validate:"omitempty"`
	SaveRetiradaLocation bool          `json:"save_retirada_location"`
	SaveEntregaLocation  bool          `json:"save_entrega_location"`
```

- [ ] **Step 4: Implement builder**

`api/internal/services/nfes/builders_doc.go` — add after `buildEnder` (line 313):
```go
// buildLocal builds a TLocal-shaped map (local de retirada/entrega) — same
// field set for both, per xsd_order.py's "retirada"/"entrega" ordering.
// Unlike buildEnder (TEndereco), TLocal has no CEP.
func buildLocal(l *NfeLocalBody) map[string]any {
	if l == nil {
		return nil
	}
	m := map[string]any{
		"xLgr":    l.XLgr,
		"nro":     l.Nro,
		"xBairro": l.XBairro,
		"cMun":    l.CMun,
		"xMun":    l.XMun,
		"UF":      l.UF,
		"cPais":   cPaisBrasil,
		"xPais":   xPaisBrasil,
	}
	if l.CNPJ != nil && *l.CNPJ != "" {
		m["CNPJ"] = *l.CNPJ
	}
	if l.CPF != nil && *l.CPF != "" {
		m["CPF"] = *l.CPF
	}
	if l.XNome != nil && *l.XNome != "" {
		m["xNome"] = *l.XNome
	}
	if l.XCpl != nil && *l.XCpl != "" {
		m["xCpl"] = *l.XCpl
	}
	if l.Fone != nil && *l.Fone != "" {
		m["fone"] = *l.Fone
	}
	if l.Email != nil && *l.Email != "" {
		m["email"] = *l.Email
	}
	return m
}
```

- [ ] **Step 5: Run, confirm pass**

Run: `cd api && go test ./internal/services/nfes/... -run TestBuildLocal -v`
Expected: PASS

- [ ] **Step 6: Wire into `BuildEnviNFe`**

Read `BuildEnviNFe` (`builders_doc.go:317` onward) to find where `infNFe` dict fields are
assembled (`dest`, `det`, etc.) and where `buildParams`/equivalent struct carries per-call data
into it. Add `retirada *NfeLocalBody` and `entrega *NfeLocalBody` fields to that params struct,
and in the `infNFe` map construction add:
```go
if retirada := buildLocal(p.retirada); retirada != nil {
	infNFe["retirada"] = retirada
}
if entrega := buildLocal(p.entrega); entrega != nil {
	infNFe["entrega"] = entrega
}
```
placed after `dest` is set (matches XSD order `dest, retirada, entrega, autXML, det` — Go map
insertion order doesn't matter since `xsd_order.py` re-orders on the Python side, but keep this
placement for readability). In `NfeService.Emit` (`emit.go`), pass `req.Retirada`/`req.Entrega`
through to the params literal used to call `BuildEnviNFe` (find the existing `buildParams{...}`
or equivalent call site — same pattern as the MDF-e `trailers` wiring from the vehicle task).

- [ ] **Step 7: Emit-level test**

`api/internal/services/nfes/emit_test.go` — add a test asserting that when `req.Entrega` is set,
the built `infNFe` (or whatever the test already inspects for `dest`) contains an `entrega` key
with the right `xLgr`; and that it's absent when `req.Entrega` is nil. Follow the existing test's
pattern for constructing a minimal valid `NfeEmitBody` in that file.

- [ ] **Step 8: Run full suite, commit**

Run: `cd api && go build ./... && go test ./... -race`
Expected: PASS

```bash
git add api/internal/services/nfes/emit.go api/internal/services/nfes/builders_doc.go \
        api/internal/services/nfes/builders_doc_test.go api/internal/services/nfes/emit_test.go
git commit -m "feat(api): support NF-e local de retirada/entrega"
```

---

### Task 4: Backend — persist entrega/retirada for reuse

**Files:**
- Modify: `api/internal/services/nfes/emit.go`
- Test: `api/internal/services/nfes/emit_test.go` or a new integration test

**Interfaces:**
- Consumes: `PersonService.Update`, `OrganizationService.Update` (existing, unchanged signatures).

- [ ] **Step 1: Implement**

In `NfeService.Emit`, after the emission succeeds (dispatch to worker/SQS — find that success
point, the same place where the prior vehicle-gating work confirmed the final return), add
best-effort persistence (never fail the emission on this):

```go
if req.SaveEntregaLocation && req.Entrega != nil && req.ReceiverID != nil {
	if err := s.appendDeliveryLocation(ctx, orgPK, *req.ReceiverID, req.Entrega, userID, userName); err != nil {
		// best-effort: log, don't fail the emission
	}
}
if req.SaveRetiradaLocation && req.Retirada != nil {
	if err := s.appendPickupLocation(ctx, orgPK, req.Retirada, userID, userName); err != nil {
		// best-effort
	}
}
```

Add the two helpers to `NfeService` (or a shared file if `NfeService` doesn't already have a
natural home — check `service.go` for where similar side-effect helpers live):
```go
const maxSavedLocations = 5

func (s *NfeService) appendDeliveryLocation(ctx context.Context, orgPK, receiverSK string, loc *NfeLocalBody, userID, userName string) error {
	current, err := s.personSvc.Get(ctx, orgPK, receiverSK) // adapt to however PersonService is reached from NfeService — check NewNfeService's deps
	if err != nil {
		return err
	}
	locs := extractLocations(current, "delivery_locations")
	locs = dedupeAppendLocation(locs, loc, maxSavedLocations)
	_, err = s.personSvc.Update(ctx, orgPK, receiverSK, map[string]any{"delivery_locations": locs}, userID, userName)
	return err
}

func (s *NfeService) appendPickupLocation(ctx context.Context, orgPK string, loc *NfeLocalBody, userID, userName string) error {
	current, err := s.orgSvc.Get(ctx, orgPK)
	if err != nil {
		return err
	}
	locs := extractLocations(current, "pickup_locations")
	locs = dedupeAppendLocation(locs, loc, maxSavedLocations)
	_, err = s.orgSvc.Update(ctx, orgPK, map[string]any{"pickup_locations": locs}, userID, userName)
	return err
}
```
`extractLocations`/`dedupeAppendLocation` are small new helpers (in the same file): decode the
existing attribute (if any) into `[]map[string]any`, compute a dedup key from
`xLgr+"|"+nro+"|"+xCpl` (normalized/uppercased), skip append if already present, cap at
`maxSavedLocations` by dropping the oldest. **Check first** whether `NfeService` already holds a
`*services.PersonService`/`*services.OrganizationService` field (`NewNfeService` constructor) —
if not, add them via the fx-wired constructor (mirrors how `orgRepo`/other deps are already
injected there — do not instantiate manually per `api/CLAUDE.md`'s DI rule).

- [ ] **Step 2: Integration test**

New test in `api/tests/integration/nfes_test.go` (or wherever NF-e emit integration tests
already live): emit an NF-e with `entrega` set and `save_entrega_location: true`, then
`GET /persons/:cpf_cnpj` and assert `person.delivery_locations` contains the entry. Follow the
existing NF-e emit integration test's harness (worker/SQS mocking — check how the prior MDF-e
gating integration test in this codebase handled the "no worker infra in tests" limitation from
the vehicle task; if full `Emit()` end-to-end still isn't testable without new mock
infrastructure, test `appendDeliveryLocation`/`appendPickupLocation` directly as white-box
functions against real DynamoDB Local, same pattern as `mdfes/vehicle_gating_test.go`).

- [ ] **Step 3: Run full suite, commit**

Run: `cd api && go build ./... && go test ./... -race`

```bash
git add api/internal/services/nfes/emit.go api/tests/integration/nfes_test.go
git commit -m "feat(api): persist NF-e entrega/retirada locations for reuse"
```

---

### Task 5: Frontend — progressive disclosure + organizationSchema

**Files:**
- Modify: `ui/src/lib/schemas/entity.ts`
- Modify: `ui/src/lib/schemas/organizations.ts`
- Modify: `ui/src/components/EntityForm.tsx`
- Test: `ui/src/__tests__/lib/schemas/entity.test.ts` (new)

- [ ] **Step 1: Write failing test**

`ui/src/__tests__/lib/schemas/entity.test.ts`:
```ts
import {describe, expect, it} from 'vitest'
import {organizationSchema} from '@/lib/schemas/organizations'
import {entitySchema} from '@/lib/schemas/entity'

const base = {
  tipo: 'pj' as const,
  cpf_or_cnpj: '11222333000181',
  name: 'Empresa Teste LTDA',
  description: '',
  person: {
    fantasy_name: '',
    crt: '1' as const,
    state_registrations: [],
    addresses: [{
      city_ibge_code: '3550308', street: 'Rua X', neighborhood: 'Centro', number: '1',
      city: 'São Paulo', state_federation: 'SP' as const, postal_code: '01000000', complement: '',
    }],
    contacts: {emails: [], phones: []},
  },
}

describe('organizationSchema', () => {
  it('rejects PJ organization without state_registrations', () => {
    const result = organizationSchema.safeParse(base)
    expect(result.success).toBe(false)
  })

  it('accepts PJ organization with at least one state_registration', () => {
    const result = organizationSchema.safeParse({
      ...base,
      person: {...base.person, state_registrations: [{uf: 'SP', state_registration: '123456'}]},
    })
    expect(result.success).toBe(true)
  })
})

describe('entitySchema (persons — unaffected)', () => {
  it('accepts PJ person without state_registrations', () => {
    const result = entitySchema.safeParse(base)
    expect(result.success).toBe(true)
  })
})
```

- [ ] **Step 2: Run, confirm fail**

Run: `cd ui && npm test -- entity.test.ts`
Expected: FAIL — `organizationSchema` not exported.

- [ ] **Step 3: Implement `organizationSchema`**

`ui/src/lib/schemas/entity.ts` — export `entitySchema` stays as-is (used by persons, unchanged
semantics). Add after it:
```ts
export const organizationSchema = entitySchema.superRefine((data, ctx) => {
  if (data.tipo === 'pj' && data.person.state_registrations.length === 0) {
    ctx.addIssue({
      code: 'custom',
      message: 'Ao menos uma Inscrição Estadual é obrigatória para organização com CNPJ',
      path: ['person', 'state_registrations'],
    })
  }
})
```

`ui/src/lib/schemas/organizations.ts` — add `organizationSchema` to the re-export list:
```ts
export {
  type EntityFormData as OrganizationFormData,
  addressSchema,
  stateRegistrationSchema,
  organizationSchema,
  UF_OPTIONS,
} from '@/lib/schemas/entity'
```

- [ ] **Step 4: Run, confirm pass**

Run: `cd ui && npm test -- entity.test.ts`
Expected: PASS

- [ ] **Step 5: Wire `organizationSchema` into `EntityForm`**

`ui/src/components/EntityForm.tsx` — import `organizationSchema` alongside `entitySchema`, and
in the `useForm` call (line 135-136) pick the resolver based on `variant`:
```ts
import {organizationSchema} from '@/lib/schemas/organizations'
// ...
const form = useForm<EntityFormData>({
  resolver: zodResolver(isOrg ? organizationSchema : entitySchema) as Resolver<EntityFormData>,
  // ...
```

- [ ] **Step 6: Progressive disclosure — collapse optional fields**

`ui/src/components/EntityForm.tsx` — add local state near the top of the component body (after
`const [submitError, ...]`, line 125):
```ts
const hasAdvancedData = !!(
  initialData?.person.fantasy_name ||
  (initialData?.person.addresses.length ?? 0) > 1 ||
  (initialData?.person.contacts.emails.length ?? 0) > 0 ||
  (initialData?.person.contacts.phones.length ?? 0) > 0 ||
  (!isOrg && (initialData?.person.state_registrations.length ?? 0) > 0)
)
const [advancedOpen, setAdvancedOpen] = useState(hasAdvancedData)
```
Then restructure the JSX: move the "Nome Fantasia" field (currently unconditionally shown at
line 305-316), the "Endereços adicionais" block (line 429-443), the "Contatos" `SectionCard`
(line 445-518), and — **only when `!isOrg`** — the Inscrições Estaduais block (line 364-416,
currently always shown for `isPJ`) behind a single collapsible section, same visual pattern as
`VehicleForm.tsx`'s advanced toggle:
```tsx
<Button type="button" variant="ghost" size="xs" onClick={() => setAdvancedOpen(!advancedOpen)}
        className="text-brand-600 hover:text-brand-700">
  {advancedOpen ? '− Ocultar informações adicionais' : '+ Informações adicionais'}
</Button>
{advancedOpen && (
  <div className="space-y-5">
    {/* fantasy_name field JSX moves here (org variant only — persons already keep it inline per current isPJ check; keep that condition) */}
    {/* endereços adicionais JSX moves here */}
    {/* Contatos SectionCard moves here */}
    {!isOrg && isPJ && (/* Inscrições Estaduais block moves here */)}
  </div>
)}
{isOrg && isPJ && (/* Inscrições Estaduais block stays OUTSIDE advanced, always visible — now required */)}
```
Keep the endereço principal (`SectionCard title="Endereço Principal"`, line 419-425) and the
"Adicionar endereço" button (line 440-443) — the button itself can stay outside advanced (it's
how the user gets to a 2nd address in the first place) but the resulting 2nd+ address cards
render inside the advanced block once `advancedOpen` is true; if `advancedOpen` is false and the
user clicks "+ Adicionar endereço", auto-open the section (`onClick={() => { appendAddress(...); setAdvancedOpen(true) }}`)
so the newly added card is actually visible.

- [ ] **Step 7: Component test**

`ui/src/__tests__/components/EntityForm.test.tsx` (check if this file already exists from prior
work — extend it, don't duplicate): add a test asserting the advanced section is collapsed by
default for a new organization, and expanded when `initialData` has `fantasy_name` set.

- [ ] **Step 8: Lint + test, commit**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero errors/warnings, all tests pass.

```bash
git add ui/src/lib/schemas/entity.ts ui/src/lib/schemas/organizations.ts \
        ui/src/components/EntityForm.tsx ui/src/__tests__/lib/schemas/entity.test.ts
git commit -m "feat(ui): progressive disclosure for pessoa/organização cadastro"
```

---

### Task 6: Frontend — types + client for entrega/retirada and saved locations

**Files:**
- Modify: `ui/src/lib/types/api.ts`
- Modify: `ui/src/lib/api/client.ts` (only if `emitNfe` needs no signature change — it already
  takes `NfeEmit`, so this task is type-only)

- [ ] **Step 1: Add types**

`ui/src/lib/types/api.ts` — add near `NfeTransportIn` (search for it):
```ts
export interface NfeLocalOut {
  cnpj?: string
  cpf?: string
  x_nome?: string
  x_lgr: string
  nro: string
  x_cpl?: string
  x_bairro: string
  c_mun: string
  x_mun: string
  uf: string
  fone?: string
  email?: string
}

export interface NfeLocalIn {
  cnpj?: string | null
  cpf?: string | null
  x_nome?: string | null
  x_lgr: string
  nro: string
  x_cpl?: string | null
  x_bairro: string
  c_mun: string
  x_mun: string
  uf: string
  fone?: string | null
  email?: string | null
}
```
Extend `NfeEmit` (line 597-613):
```ts
  retirada?: NfeLocalIn | null
  entrega?: NfeLocalIn | null
  save_retirada_location?: boolean
  save_entrega_location?: boolean
```
Extend `PersonDetailsOut` (line 461-467): `delivery_locations?: NfeLocalOut[]`.
Extend `OrganizationOut` (line 90-97): `pickup_locations?: NfeLocalOut[]`.

- [ ] **Step 2: Lint, commit**

Run: `cd ui && npx eslint src --ext .ts,.tsx`

```bash
git add ui/src/lib/types/api.ts
git commit -m "feat(ui): types for NF-e entrega/retirada and saved locations"
```

---

### Task 7: Frontend — NfeEmitForm entrega/retirada picker

**Files:**
- Modify: `ui/src/components/nfe/NfeEmitForm.tsx`

**Interfaces:**
- Consumes: `receiver: PersonItemOut | null` (state at line 809), `selectedOrg` from `useAuth()`
  (already destructured in this file — verify import), `NfeLocalIn` (Task 6).

- [ ] **Step 1: Local state**

Near `const [receiver, setReceiver] = useState<PersonItemOut | null>(null)` (line 809), add:
```ts
const [entrega, setEntrega] = useState<NfeLocalIn | null>(null)
const [saveEntregaLocation, setSaveEntregaLocation] = useState(false)
const [retirada, setRetirada] = useState<NfeLocalIn | null>(null)
const [saveRetiradaLocation, setSaveRetiradaLocation] = useState(false)
```
Reset `entrega`/`saveEntregaLocation` to `null`/`false` in the same place `receiver` is reset
(the `setSelfIssuance`/`setReceiver(null)` toggle around line 1176-1181, and wherever
`onChange`/`ReceiverSearch` clears the receiver) — entrega is tied to the selected destinatário,
stale entrega data must not survive a receiver change.

- [ ] **Step 2: Entrega picker UI**

After the `<ReceiverSearch value={receiver} onChange={setReceiver}/>` line (1238), inside the
`{receiver && (...)}`-guarded block (find or add one — entrega should only show once a receiver
is picked and `!selfIssuance`), add a new collapsible block:
```tsx
{receiver && !selfIssuance && (
  <LocationPicker
    label="Local de entrega"
    savedLocations={receiver.person.delivery_locations ?? []}
    value={entrega}
    onChange={setEntrega}
    save={saveEntregaLocation}
    onSaveChange={setSaveEntregaLocation}
  />
)}
```
Add the analogous block for retirada, fed by `selectedOrg?.pickup_locations ?? []` — note
`selectedOrg` here needs `pickup_locations` on its type; if `useAuth()`'s org type is a narrower
shape than full `OrganizationOut`, either extend it or fetch the full org record via the existing
`apiClient.getOrganization(selectedOrg.pk)` `useQuery` (check whether one already exists in this
file for the org, reuse it — don't add a duplicate query).

- [ ] **Step 3: New `LocationPicker` component**

Create `ui/src/components/nfe/LocationPicker.tsx` — a small collapsible: closed by default
(header "+ Local de entrega" / "− Ocultar"); when open, shows `savedLocations` as selectable
chips (click → `onChange(loc)`), a "endereço diferente" toggle that reveals a manual entry
mini-form (xLgr, nro, xCpl, xBairro, cMun via IBGE lookup — reuse whatever city/IBGE picker
`AddressFields` already uses, check `ui/src/components/ui/address-fields.tsx` for the exact
sub-component to reuse rather than reinventing city lookup — xMun, UF, fone, email, CNPJ/CPF+
xNome optional), and a "Salvar este local para reutilizar" checkbox wired to `onSaveChange`
(checked by default when the manual form has been touched and doesn't match an existing saved
location). Props:
```ts
interface LocationPickerProps {
  label: string
  savedLocations: NfeLocalOut[]
  value: NfeLocalIn | null
  onChange: (loc: NfeLocalIn | null) => void
  save: boolean
  onSaveChange: (save: boolean) => void
}
```
Follow `ui/CLAUDE.md` mobile-first rules (44px touch targets, `grid-cols-1 sm:grid-cols-2`, no
horizontal overflow) and debounce any text input that would trigger a lookup (300ms via
`DebouncedInput`, matching the rest of the codebase).

- [ ] **Step 4: Submit payload**

In `handleSubmit`'s `payload` construction (line 1098-1135), add:
```ts
  retirada: retirada,
  entrega: entrega,
  save_retirada_location: retirada ? saveRetiradaLocation : false,
  save_entrega_location: entrega ? saveEntregaLocation : false,
```

- [ ] **Step 5: Component test**

`ui/src/__tests__/components/nfe/LocationPicker.test.tsx` (new): renders collapsed by default;
clicking a saved-location chip calls `onChange` with that location; toggling "endereço diferente"
reveals the manual form.

- [ ] **Step 6: Lint + test + manual check, commit**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`

Manual verification (per `ui/CLAUDE.md` — UI changes must be exercised in a browser before
claiming done): start the dev server if none is already running (`cd ui && npm run dev` — check
for a port conflict from an existing instance first, as in the prior vehicle-task session), open
the NF-e emission flow, select a destinatário with no saved delivery locations, verify "Local de
entrega" shows the manual-entry form; select a destinatário that has saved locations (seed one via
the API or a prior test emission) and verify the chips render and populate the fields on click.

```bash
git add ui/src/components/nfe/NfeEmitForm.tsx ui/src/components/nfe/LocationPicker.tsx \
        ui/src/__tests__/components/nfe/LocationPicker.test.tsx
git commit -m "feat(ui): NF-e local de entrega/retirada picker with history reuse"
```

---

### Task 8: Documentation

**Files:**
- Modify: `DOCS.md`
- Modify: `CONDUCT.md`
- Modify: `DynamoDB-Tables.md`

- [ ] **Step 1: `DynamoDB-Tables.md`**

Table `organization_persons`: add `delivery_locations` (L, optional, cap 5, TLocal-shaped) to the
attribute list. Table `organizations`: add `pickup_locations` (L, optional, cap 5, same shape).
Also fix the pre-existing staleness noted during research (table #6's documented `pk`/`sk`/`ie`/
`address` shape doesn't match the real `org_pk`/`CNPJ_`-`CPF_`-prefixed-sk`/
`state_registrations[]`/`addresses[]` shape in code) — correct it while touching this section.

- [ ] **Step 2: `DOCS.md`**

Persons/Organizations endpoint tables: note the new 400 (CRT/IE required for PJ) and 409
(duplicate CPF/CNPJ) responses on `POST /persons`/`POST /organizations`. NF-e emission body: add
`retirada`/`entrega`/`save_retirada_location`/`save_entrega_location` fields with the TLocal
shape.

- [ ] **Step 3: `CONDUCT.md`**

Add a note under wherever party/pessoa conventions are documented (or a new subsection):
"Gating por doc-type (bloqueia emissão + modal, como em veículos) não se aplica a pessoas/
organizações — ver `docs/superpowers/specs/2026-07-11-pessoas-organizacoes-cadastro-design.md`
para o porquê. CRT e IE de organização PJ são validados como obrigatórios no cadastro, não na
emissão."

- [ ] **Step 4: Commit**

```bash
git add DOCS.md CONDUCT.md DynamoDB-Tables.md
git commit -m "docs: document pessoa/organização validation and NF-e entrega/retirada"
```

---

## Final Verification

- [ ] `cd api && go build ./... && go test ./... -race`
- [ ] `cd api && go test ./tests/integration/... -tags=integration` (DynamoDB Local running)
- [ ] `cd ui && npx eslint src --ext .ts,.tsx && npx tsc --noEmit && npm test`
- [ ] `grep -rn "TODO\|FIXME" api/internal/services/persons.go api/internal/services/organizations.go api/internal/services/nfes/emit.go` — confirm clean
- [ ] Manual browser walkthrough of NF-e emission with entrega/retirada (Task 7 Step 6)
