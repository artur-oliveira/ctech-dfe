# autXML — Pessoas Autorizadas — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an organization register up to 10 CPF/CNPJ+name pairs authorized to view its NF-e
XMLs (SEFAZ `autXML`), with add/remove endpoints, no duplicates, always included in NF-e XML build.

**Architecture:** New `authorized_xml_viewers` list attribute on the existing `organizations`
item — no new table. `OrganizationService` gets two new methods that read-modify-write the list
through the already-existing generic `Update` (fetch→merge→diff→TransactWrite+audit). Builder gets
`buildAutXML`. No py-dfe change (`xsd_order.py` already orders `autXML`).

**Tech Stack:** Go/Fiber v3/DynamoDB (api), Next.js/TS/Zod (ui).

## Global Constraints

- No `Co-Authored-By:` trailer on any commit.
- `api`: errors via `problem.*`; layer separation strict; no new goroutines in handlers.
- `ui`: `npx eslint src --ext .ts,.tsx` zero errors/warnings; mobile-first; debounced inputs.
- Reuse the existing `update.organizations` RBAC permission for the two new routes — do not
  invent a new permission key (no seed-data/RBAC-migration mechanism was found for this repo in
  prior research; adding one is out of scope for this feature).

---

### Task 1: Backend — DTO + service methods

**Files:**
- Modify: `api/internal/api/v1/dto.go`
- Modify: `api/internal/services/organizations.go`
- Test: `api/internal/services/organizations_test.go`

**Interfaces:**
- Produces: `AuthorizedViewerBody{CpfOrCnpj, Name string}`.
- Produces: `OrganizationService.AddAuthorizedViewer(ctx, orgPK, v, userID, userName) (map[string]types.AttributeValue, error)`.
- Produces: `OrganizationService.RemoveAuthorizedViewer(ctx, orgPK, cpfCnpj, userID, userName) (map[string]types.AttributeValue, error)`.
- Consumes: `OrganizationService.Update` (existing, unchanged — `organizations.go:58-104`),
  `attributeMapToPlain` (`services/shared.go:71`).

- [ ] **Step 1: Write failing tests**

`api/internal/services/organizations_test.go` — append (check existing file first; this task
assumes Task 1 of the pessoas/organizações plan already added `RequireOrgIE` tests here — append
alongside, don't overwrite):
```go
func TestAddAuthorizedViewer_RejectsDuplicateCpfCnpj(t *testing.T) {
	current := map[string]any{
		"authorized_xml_viewers": []any{
			map[string]any{"cpf_cnpj": "11122233344", "name": "Existing"},
		},
	}
	viewers := extractAuthorizedViewers(current)
	if len(viewers) != 1 {
		t.Fatalf("expected 1 existing viewer, got %d", len(viewers))
	}
	_, err := appendAuthorizedViewer(viewers, AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "Dup"})
	if err == nil {
		t.Fatal("expected error for duplicate cpf_cnpj")
	}
}

func TestAddAuthorizedViewer_RejectsAt11thEntry(t *testing.T) {
	viewers := make([]AuthorizedViewerEntry, 10)
	for i := range viewers {
		viewers[i] = AuthorizedViewerEntry{CpfOrCnpj: fmt.Sprintf("%011d", i), Name: "V"}
	}
	_, err := appendAuthorizedViewer(viewers, AuthorizedViewerEntry{CpfOrCnpj: "99999999999", Name: "Overflow"})
	if err == nil {
		t.Fatal("expected error at 11th entry")
	}
}

func TestAddAuthorizedViewer_AppendsValid(t *testing.T) {
	out, err := appendAuthorizedViewer(nil, AuthorizedViewerEntry{CpfOrCnpj: "11122233344", Name: "New"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].CpfOrCnpj != "11122233344" {
		t.Fatalf("unexpected result: %+v", out)
	}
}

func TestRemoveAuthorizedViewer_FiltersMatching(t *testing.T) {
	viewers := []AuthorizedViewerEntry{
		{CpfOrCnpj: "11122233344", Name: "A"},
		{CpfOrCnpj: "22233344455", Name: "B"},
	}
	out := removeAuthorizedViewerEntry(viewers, "11122233344")
	if len(out) != 1 || out[0].CpfOrCnpj != "22233344455" {
		t.Fatalf("unexpected result: %+v", out)
	}
}
```
(Add `"fmt"` to the test file's imports if not already present.)

- [ ] **Step 2: Run, confirm fail**

Run: `cd api && go test ./internal/services/... -run 'TestAddAuthorizedViewer|TestRemoveAuthorizedViewer' -v`
Expected: FAIL — types/functions undefined.

- [ ] **Step 3: DTO**

`api/internal/api/v1/dto.go` — add near the other org-adjacent bodies:
```go
// AuthorizedViewerBody is one entry in an organization's SEFAZ autXML list
// (CPF/CNPJ + name authorized to view that organization's NF-e XMLs).
type AuthorizedViewerBody struct {
	CpfOrCnpj string `json:"cpf_or_cnpj" validate:"required,cpfcnpj"`
	Name      string `json:"name" validate:"required,min=2,max=60"`
}
```

- [ ] **Step 4: Service implementation**

`api/internal/services/organizations.go` — add:
```go
const maxAuthorizedViewers = 10

// AuthorizedViewerEntry is the plain-value shape of an authorized_xml_viewers
// entry (stored as a list of maps on the organization item).
type AuthorizedViewerEntry struct {
	CpfOrCnpj string `json:"cpf_cnpj"`
	Name      string `json:"name"`
}

func normalizeDoc(s string) string {
	return strings.NewReplacer(".", "", "-", "", "/", "").Replace(s)
}

func extractAuthorizedViewers(item map[string]any) []AuthorizedViewerEntry {
	raw, _ := item["authorized_xml_viewers"].([]any)
	out := make([]AuthorizedViewerEntry, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		cpfCnpj, _ := m["cpf_cnpj"].(string)
		name, _ := m["name"].(string)
		out = append(out, AuthorizedViewerEntry{CpfOrCnpj: cpfCnpj, Name: name})
	}
	return out
}

// appendAuthorizedViewer returns the new list with v appended, or an error if
// v.CpfOrCnpj is already present or the list is already at the SEFAZ-imposed
// cap of 10.
func appendAuthorizedViewer(current []AuthorizedViewerEntry, v AuthorizedViewerEntry) ([]AuthorizedViewerEntry, error) {
	if len(current) >= maxAuthorizedViewers {
		return nil, problem.BadRequest("limite de 10 pessoas autorizadas atingido")
	}
	normalized := normalizeDoc(v.CpfOrCnpj)
	for _, existing := range current {
		if normalizeDoc(existing.CpfOrCnpj) == normalized {
			return nil, problem.Conflict("CPF/CNPJ já autorizado")
		}
	}
	v.CpfOrCnpj = normalized
	return append(current, v), nil
}

func removeAuthorizedViewerEntry(current []AuthorizedViewerEntry, cpfCnpj string) []AuthorizedViewerEntry {
	normalized := normalizeDoc(cpfCnpj)
	out := make([]AuthorizedViewerEntry, 0, len(current))
	for _, existing := range current {
		if normalizeDoc(existing.CpfOrCnpj) != normalized {
			out = append(out, existing)
		}
	}
	return out
}

// AddAuthorizedViewer appends a person authorized to view this organization's
// NF-e XMLs (SEFAZ autXML, max 10, no duplicate CPF/CNPJ).
func (s *OrganizationService) AddAuthorizedViewer(ctx context.Context, orgPK string, v AuthorizedViewerEntry, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	currentMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}
	viewers, err := appendAuthorizedViewer(extractAuthorizedViewers(currentMap), v)
	if err != nil {
		return nil, err
	}
	return s.Update(ctx, orgPK, map[string]any{"authorized_xml_viewers": viewers}, userID, userName)
}

// RemoveAuthorizedViewer removes an authorized viewer by CPF/CNPJ. No-op
// (not an error) if the CPF/CNPJ wasn't in the list.
func (s *OrganizationService) RemoveAuthorizedViewer(ctx context.Context, orgPK, cpfCnpj, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("organization not found")
	}
	currentMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}
	viewers := removeAuthorizedViewerEntry(extractAuthorizedViewers(currentMap), cpfCnpj)
	return s.Update(ctx, orgPK, map[string]any{"authorized_xml_viewers": viewers}, userID, userName)
}
```
Add `"strings"` and `"github.com/artur-oliveira/ctech-dfe/api/internal/problem"` imports if not
already present in this file (Task 1 of the pessoas/organizações plan may have already added
`problem` — check before adding a duplicate import).

- [ ] **Step 5: Run, confirm pass**

Run: `cd api && go test ./internal/services/... -run 'TestAddAuthorizedViewer|TestRemoveAuthorizedViewer' -v`
Expected: PASS

- [ ] **Step 6: Run full suite, commit**

Run: `cd api && go build ./... && go test ./... -race`

```bash
git add api/internal/api/v1/dto.go api/internal/services/organizations.go \
        api/internal/services/organizations_test.go
git commit -m "feat(api): add/remove organization authorized XML viewers"
```

---

### Task 2: Backend — routes

**Files:**
- Modify: `api/internal/api/v1/organizations.go`
- Test: `api/tests/integration/organizations_test.go`

- [ ] **Step 1: Add routes**

In `RegisterOrganizations`, inside the `scoped` group (after the `PUT ""` handler, before
`registerFiscalConfig` calls — `organizations.go:119`), add:
```go
	// ── Authorized XML viewers (SEFAZ autXML) ──────────────────────────────────

	scoped.Post("/authorized-viewers", perm.Require("update.organizations"), func(c fiber.Ctx) error {
		var dto AuthorizedViewerBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, h.UserSvc)
		entry := services.AuthorizedViewerEntry{CpfOrCnpj: dto.CpfOrCnpj, Name: dto.Name}
		org, err := h.OrgSvc.AddAuthorizedViewer(c.Context(), middleware.GetOrgPK(c), entry, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})

	scoped.Delete("/authorized-viewers/:cpf_cnpj", perm.Require("update.organizations"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, h.UserSvc)
		org, err := h.OrgSvc.RemoveAuthorizedViewer(c.Context(), middleware.GetOrgPK(c), c.Params("cpf_cnpj"), userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, org)
	})
```

- [ ] **Step 2: Integration tests**

`api/tests/integration/organizations_test.go` — add, following the existing HTTP-harness pattern
in that file (look at an existing `TestOrganization_Update*` test for the request/org-setup
boilerplate):
```go
func TestOrganization_AddAuthorizedViewer(t *testing.T) {
	// POST /organizations/:pk/authorized-viewers with valid cpf_or_cnpj+name → 200,
	// response's person/org body includes it in authorized_xml_viewers.
}

func TestOrganization_AddAuthorizedViewer_DuplicateReturns409(t *testing.T) {
	// add once (200), add same cpf_or_cnpj again → 409
}

func TestOrganization_AddAuthorizedViewer_EleventhReturns400(t *testing.T) {
	// add 10 distinct entries (200 each), 11th → 400
}

func TestOrganization_RemoveAuthorizedViewer(t *testing.T) {
	// add one, DELETE /organizations/:pk/authorized-viewers/:cpf_cnpj → 200, list no longer contains it
}
```

- [ ] **Step 3: Run, commit**

Run: `cd api && go build ./... && go test ./... -race`
Run: `cd api && go test ./tests/integration/... -tags=integration -run TestOrganization_.*AuthorizedViewer -v`

```bash
git add api/internal/api/v1/organizations.go api/tests/integration/organizations_test.go
git commit -m "feat(api): expose authorized-viewers endpoints on organizations"
```

---

### Task 3: Backend — wire into NF-e builder

**Files:**
- Modify: `api/internal/services/nfes/builders_doc.go`
- Test: `api/internal/services/nfes/builders_doc_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildAutXML_CNPJRoutedCorrectly(t *testing.T) {
	org := map[string]any{
		"authorized_xml_viewers": []any{
			map[string]any{"cpf_cnpj": "11222333000181", "name": "Contador"},
			map[string]any{"cpf_cnpj": "11122233344", "name": "Auditor"},
		},
	}
	got := buildAutXML(org)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0]["CNPJ"] != "11222333000181" {
		t.Fatalf("expected CNPJ routing for 14-digit doc: %+v", got[0])
	}
	if got[1]["CPF"] != "11122233344" {
		t.Fatalf("expected CPF routing for 11-digit doc: %+v", got[1])
	}
}

func TestBuildAutXML_EmptyReturnsNil(t *testing.T) {
	if buildAutXML(map[string]any{}) != nil {
		t.Fatal("expected nil for organization with no authorized viewers")
	}
}
```

- [ ] **Step 2: Run, confirm fail**

Run: `cd api && go test ./internal/services/nfes/... -run TestBuildAutXML -v`
Expected: FAIL — `buildAutXML` undefined.

- [ ] **Step 3: Implement**

`api/internal/services/nfes/builders_doc.go` — add near `buildLocal` (or, if Task 3 of the
pessoas/organizações plan hasn't landed yet, near `buildEnder`):
```go
// buildAutXML builds the autXML list (CPF/CNPJ authorized to view this
// organization's NF-e XML) from the organization's authorized_xml_viewers
// attribute. Returns nil (key omitted) when the organization has none.
func buildAutXML(org map[string]any) []map[string]any {
	raw, _ := org["authorized_xml_viewers"].([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		doc := anyStr(vm, "cpf_cnpj", "")
		if doc == "" {
			continue
		}
		entry := map[string]any{}
		if len(doc) == 14 {
			entry["CNPJ"] = doc
		} else {
			entry["CPF"] = doc
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
```

- [ ] **Step 4: Run, confirm pass**

Run: `cd api && go test ./internal/services/nfes/... -run TestBuildAutXML -v`
Expected: PASS

- [ ] **Step 5: Wire into `BuildEnviNFe`**

In the same `infNFe` map assembly touched by Task 3/Step 6 of the pessoas/organizações plan (or,
if that task hasn't run yet, locate it directly — search `BuildEnviNFe` for where `"dest"` is set
on the `infNFe` map), add:
```go
if autXML := buildAutXML(org); autXML != nil {
	infNFe["autXML"] = autXML
}
```
`org` here is the same `map[string]any` already passed into `BuildEnviNFe` for `buildEnder`/emit
fields — no new parameter needed.

- [ ] **Step 6: Run full suite, commit**

Run: `cd api && go build ./... && go test ./... -race`

```bash
git add api/internal/services/nfes/builders_doc.go api/internal/services/nfes/builders_doc_test.go
git commit -m "feat(api): include organization authorized viewers as autXML in NF-e"
```

---

### Task 4: Frontend — types, client, UI section

**Files:**
- Modify: `ui/src/lib/types/api.ts`
- Modify: `ui/src/lib/api/client.ts`
- Modify: `ui/src/lib/api/query-keys.ts`
- Create: `ui/src/lib/schemas/authorized-viewers.ts`
- Create: `ui/src/components/organizations/AuthorizedViewersSection.tsx`
- Modify: `ui/src/app/organizations/edit/page.tsx`
- Test: `ui/src/__tests__/lib/schemas/authorized-viewers.test.ts`

- [ ] **Step 1: Types**

`ui/src/lib/types/api.ts` — add near `OrganizationOut`:
```ts
export interface AuthorizedViewerOut {
  cpf_cnpj: string
  name: string
}
```
Extend `OrganizationOut` (line 90-97): `authorized_xml_viewers?: AuthorizedViewerOut[]`.

- [ ] **Step 2: Client methods**

`ui/src/lib/api/client.ts` — add near `updateOrganization` (line 201-203):
```ts
async addAuthorizedViewer(orgPk: string, data: { cpf_or_cnpj: string; name: string }): Promise<OrganizationOut> {
  return this.post<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(orgPk)}/authorized-viewers`, data)
}

async removeAuthorizedViewer(orgPk: string, cpfCnpj: string): Promise<OrganizationOut> {
  return this.delete<OrganizationOut>(`/v1.0/organizations/${unformatCpfCnpj(orgPk)}/authorized-viewers/${unformatCpfCnpj(cpfCnpj)}`)
}
```
(Check the class already has a `delete<T>` helper — mirrors `get`/`post`/`put` used elsewhere in
this file; if absent, look at how `removeAuthorizedViewer`'s Vehicle/Certificate delete
counterparts call `this.client.delete(...)` directly and match that style instead.)

- [ ] **Step 3: Query key + schema**

`ui/src/lib/api/query-keys.ts` — add under the organizations section:
```ts
authorizedViewers: (orgPk: string) => [...queryKeys.organizations.detail(orgPk), 'authorized-viewers'] as const,
```
(Adapt to however `organizations.detail` is actually named in this file — read it first.)

`ui/src/lib/schemas/authorized-viewers.ts`:
```ts
import {z} from 'zod'
import {validateCNPJ, validateCPF} from '@/lib/utils/validators'

export const authorizedViewerSchema = z.object({
  cpf_or_cnpj: z.string().min(1, 'CPF/CNPJ obrigatório'),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(60),
}).superRefine((data, ctx) => {
  const raw = data.cpf_or_cnpj.replace(/\D/g, '')
  const valid = raw.length === 14 ? validateCNPJ(raw) : validateCPF(raw)
  if (!valid) {
    ctx.addIssue({code: 'custom', message: 'CPF/CNPJ inválido', path: ['cpf_or_cnpj']})
  }
})

export function hasDuplicateViewer(existing: {cpf_cnpj: string}[], cpfOrCnpj: string): boolean {
  const raw = cpfOrCnpj.replace(/\D/g, '')
  return existing.some(v => v.cpf_cnpj.replace(/\D/g, '') === raw)
}

export const MAX_AUTHORIZED_VIEWERS = 10
```

- [ ] **Step 4: Schema test**

`ui/src/__tests__/lib/schemas/authorized-viewers.test.ts`:
```ts
import {describe, expect, it} from 'vitest'
import {authorizedViewerSchema, hasDuplicateViewer} from '@/lib/schemas/authorized-viewers'

describe('authorizedViewerSchema', () => {
  it('rejects invalid CPF', () => {
    expect(authorizedViewerSchema.safeParse({cpf_or_cnpj: '11111111111', name: 'X'}).success).toBe(false)
  })
})

describe('hasDuplicateViewer', () => {
  it('detects duplicate ignoring formatting', () => {
    const existing = [{cpf_cnpj: '11122233344'}]
    expect(hasDuplicateViewer(existing, '111.222.333-44')).toBe(true)
    expect(hasDuplicateViewer(existing, '99988877766')).toBe(false)
  })
})
```

- [ ] **Step 5: Run, confirm pass**

Run: `cd ui && npm test -- authorized-viewers.test.ts`
Expected: PASS

- [ ] **Step 6: UI section component**

`ui/src/components/organizations/AuthorizedViewersSection.tsx` — new component: takes
`orgPk: string` and `viewers: AuthorizedViewerOut[]` props, renders a list with remove buttons, a
CPF/CNPJ+name add form gated by `authorizedViewerSchema` + `hasDuplicateViewer` (block submit
client-side, show inline error — the 409 from the backend is the source of truth, this is just to
avoid a wasted round-trip) and a "X/10" counter (disable add form at `MAX_AUTHORIZED_VIEWERS`).
Use `useMutation` + `apiClient.addAuthorizedViewer`/`removeAuthorizedViewer`, invalidate the org
query on success (loading state on the add button and each remove button per `ui/CLAUDE.md`).
Follow the mobile-first rules (stacked list on mobile, ≥44px touch targets on remove buttons).

- [ ] **Step 7: Mount in organization edit page**

`ui/src/app/organizations/edit/page.tsx` — read the file first to find where `OrganizationForm`
is rendered, then add `<AuthorizedViewersSection orgPk={...} viewers={org.authorized_xml_viewers ?? []}/>`
below it (or in an adjacent tab/section, matching whatever layout convention that page already
uses — do not restructure the page beyond adding this section).

- [ ] **Step 8: Lint + test, commit**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npx tsc --noEmit && npm test`
Expected: zero errors/warnings, all tests pass.

Manual verification: start dev server (reuse a running instance if one already exists — check
before starting a duplicate), open an organization's edit page, add an authorized viewer, confirm
it appears and the counter updates, try adding a duplicate CPF/CNPJ and confirm the inline error,
remove one and confirm it disappears.

```bash
git add ui/src/lib/types/api.ts ui/src/lib/api/client.ts ui/src/lib/api/query-keys.ts \
        ui/src/lib/schemas/authorized-viewers.ts ui/src/components/organizations/AuthorizedViewersSection.tsx \
        ui/src/app/organizations/edit/page.tsx ui/src/__tests__/lib/schemas/authorized-viewers.test.ts
git commit -m "feat(ui): manage organization authorized XML viewers"
```

---

### Task 5: Documentation

**Files:**
- Modify: `DOCS.md`
- Modify: `DynamoDB-Tables.md`

- [ ] **Step 1: `DynamoDB-Tables.md`**

Table `organizations`: add `authorized_xml_viewers` (L, optional, cap 10, `{cpf_cnpj, name}`).

- [ ] **Step 2: `DOCS.md`**

Organizations endpoint table: add `POST/DELETE /v1.0/organizations/{pk}/authorized-viewers[/{cpf_cnpj}]`
rows (400 if ≥10, 409 if duplicate). NF-e emission section: note `autXML` is automatic
(organization-level setting), not a per-emission payload field.

- [ ] **Step 3: Commit**

```bash
git add DOCS.md DynamoDB-Tables.md
git commit -m "docs: document autXML authorized viewers"
```

---

## Final Verification

- [ ] `cd api && go build ./... && go test ./... -race`
- [ ] `cd api && go test ./tests/integration/... -tags=integration` (DynamoDB Local running)
- [ ] `cd ui && npx eslint src --ext .ts,.tsx && npx tsc --noEmit && npm test`
- [ ] Manual browser walkthrough of add/remove/duplicate/limit on an organization's edit page
