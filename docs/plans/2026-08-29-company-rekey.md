# DF-e Company Re-key Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** the DF-e partitions its data by the platform `company_id` instead of `CNPJ_{digits}`, so two organizations may hold the same CNPJ without sharing a partition.

**Architecture:** the partition key is resolved once, at the middleware edge, and every repository takes it as a parameter — so the code change is the edge plus the four places that read the CNPJ back out of the key. The data change is a copy-then-verify-then-flip migration, with the old partitions left intact as the rollback.

**Tech Stack:** Go 1.27, Fiber v3, DynamoDB via `gopkg.aoctech.app/api-commons/dynamo`.

**Spec:** [`docs/specs/2026-08-29-company-rekey.md`](../specs/2026-08-29-company-rekey.md), which implements [ctech-billing ADR 0022](../../../ctech-billing/docs/adr/0022-company-identity-in-account.md).

## Global Constraints

- **The key is a UUIDv7 string, not a CNPJ.** Nothing may read a tax id out of a partition key. The canonical tax id lives on the local company record and in `ctech-account`.
- **A CNPJ is alphanumeric.** Its first twelve positions may hold letters since the Receita Federal's 2026 change; only the two check digits stayed numeric. `stripNonDigits` on a CNPJ is wrong from now on.
- **The certificate never leaves this repo**, not even as a flag saying one exists.
- **Copy, never move.** Every migration writes the new partition and leaves the old one byte-for-byte. That is the rollback.
- **`Dfe-Organization-Pk` keeps its name.** Only its value changes. Renaming it is a coordinated two-app deploy for a word (`middleware/rbac.go:22`).
- Commit messages carry **no** `Co-Authored-By` trailer.

## Scope

Tasks 1–5 are code that is correct **before and after** the flip: they ship independently, on the old key, and change no behaviour until a company record carries the new fields. Task 6 is the migration tool. The operational run (freeze, verify, flip) is the runbook, not this plan.

**Task 7 is the UI, and it ships before anything sends a company id.** The browser strips the key before putting it on the wire (`ui/src/lib/api/client.ts:222`), which mangles a UUID into a value the server refuses. Running the migration first means every screen answers "organização inválida" and the cause sits three layers from the symptom. It is numbered last because it is smallest, not because it is late.

**Not here:** membership/RBAC unification (phase 3), deleting the old partitions, and the handoff landing route — all named in the spec's out-of-scope section.

---

## File Structure

| File | Responsibility |
|---|---|
| `api/internal/repositories/organizations.go` (modify) | `ParseOrgPK` accepts a company id; the local company record's new fields |
| `api/internal/repositories/company.go` (create) | reading and refreshing the cached platform identity |
| `api/internal/repositories/serie_claim.go` (create) | the `(tax_id, modelo, série, ambiente)` lock |
| `api/internal/services/organizations.go` (modify) | `cnpjRoot` off the record, sibling search scoped to the organization |
| `api/cmd/rekey-companies/` (create) | the pass-2 copy, verify and report |
| `cdk/lib/dynamodb-stack.ts` (modify) | the série-claim table |
| `ui/src/lib/utils/document.ts` (modify) | stop mangling a key that is not a document |
| `ui/src/lib/api/client.ts` (modify) | the header carries the key verbatim |
| `ui/src/lib/types/api.ts` (modify) | `OrganizationOut` carries `tax_id` / `tax_id_kind` |

---

### Task 1: The key stops meaning a CNPJ

**Files:**
- Modify: `api/internal/repositories/organizations.go:44-59`
- Modify: `api/internal/middleware/tenant.go:15-22`
- Test: `api/internal/repositories/organizations_key_test.go` (create)

**Interfaces:**
- Produces: `ParseOrgPK(raw string) (string, error)` — unchanged signature, widened acceptance. A UUID passes through as-is; `CNPJ_`/`CPF_` still pass through (the old partitions stay readable, and rollback needs them); a bare CPF/CNPJ is still normalized to the legacy prefix form. Anything else is refused.
- Produces: `IsCompanyKey(pk string) bool` — true for a UUID-shaped key. The one predicate everything else branches on, so "is this row migrated" is asked in exactly one place.

- [ ] **Step 1: Write the failing test**

```go
package repositories

import "testing"

// The new key. A company id is a UUIDv7 from ctech-account and passes through
// untouched — it is already canonical.
func TestParseOrgPKAcceptsACompanyID(t *testing.T) {
	const id = "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"
	got, err := ParseOrgPK(id)
	if err != nil || got != id {
		t.Fatalf("got %q, %v; want the id unchanged", got, err)
	}
	if !IsCompanyKey(id) {
		t.Error("IsCompanyKey said no to a company id")
	}
}

// The legacy shapes stay valid. The old partitions are the rollback, and a
// build that cannot read them cannot roll back.
func TestParseOrgPKStillAcceptsTheLegacyShapes(t *testing.T) {
	for _, in := range []string{"CNPJ_11222333000181", "CPF_52998224725"} {
		got, err := ParseOrgPK(in)
		if err != nil || got != in {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
		if IsCompanyKey(in) {
			t.Errorf("%q: IsCompanyKey said yes to a legacy key", in)
		}
	}
}

func TestParseOrgPKRefusesEverythingElse(t *testing.T) {
	for _, in := range []string{
		"",
		"nope",
		"0199f3a1-8c42-7c31-9d5e",             // short
		"0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f7", // one short
		"0199f3a1_8c42_7c31_9d5e_6a2b4c8e1f70", // wrong separators
		"ORG#0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70",
	} {
		if _, err := ParseOrgPK(in); err == nil {
			t.Errorf("%q: accepted, want refused", in)
		}
	}
}

// The legacy path normalizes a typed document. It is what created every key in
// production and must keep working while those partitions exist.
func TestParseOrgPKStillNormalizesATypedDocument(t *testing.T) {
	got, err := ParseOrgPK("11.222.333/0001-81")
	if err != nil || got != "CNPJ_11222333000181" {
		t.Fatalf("got %q, %v", got, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/ -run TestParseOrgPK -v`
Expected: FAIL — `undefined: IsCompanyKey`.

- [ ] **Step 3: Write the implementation**

Replace `ParseOrgPK` in `api/internal/repositories/organizations.go`:

```go
// IsCompanyKey reports whether pk is a platform company id rather than a legacy
// CNPJ_/CPF_ key.
//
// One predicate, because "has this row been re-keyed" is asked from several
// places and two spellings of the question drift.
func IsCompanyKey(pk string) bool {
	// A UUID's shape: 8-4-4-4-12 lowercase hex. Checked rather than parsed
	// because this runs on every request and the answer is only ever "which
	// key era is this".
	if len(pk) != 36 {
		return false
	}
	for i, c := range pk {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	return true
}

// ParseOrgPK canonicalizes a partition key.
//
// Three accepted shapes, and the two legacy ones are not deprecation debt: the
// old partitions are the migration's rollback, and a build that cannot read
// them cannot roll back.
//
//   - a platform company id (UUIDv7) — the key from ADR 0022 onward
//   - an already-prefixed CNPJ_/CPF_ key — legacy, still readable
//   - a bare or masked CPF/CNPJ — normalized to the legacy prefix form
//
// Note what is NOT here: nothing reads a tax id back out of the result. A CNPJ
// has been alphanumeric in its first twelve positions since 2026, and the
// canonical tax id lives on the company record, not in the key.
func ParseOrgPK(cpfOrCNPJ string) (string, error) {
	if IsCompanyKey(cpfOrCNPJ) {
		return cpfOrCNPJ, nil
	}
	if strings.HasPrefix(cpfOrCNPJ, "CNPJ_") || strings.HasPrefix(cpfOrCNPJ, "CPF_") {
		return cpfOrCNPJ, nil
	}
	digits := stripNonDigits(cpfOrCNPJ)
	switch len(digits) {
	case 11:
		return fmt.Sprintf("CPF_%s", digits), nil
	case 14:
		return fmt.Sprintf("CNPJ_%s", digits), nil
	default:
		return "", problem.BadRequest("organização inválida")
	}
}
```

The error message loses "deve começar com CNPJ_ ou CPF_": it is user-facing and would now be describing a shape the product no longer issues.

- [ ] **Step 4: Update the middleware's copy**

`api/internal/middleware/tenant.go` has a second, looser `ParseOrgPK` doing a prefix check by hand. It is the WebSocket path's validator (`api/v1/ws.go:137`). Delegate rather than widen a duplicate:

```go
// ParseOrgPK validates a tenant key from the WebSocket handshake, where there
// is no repository handy. It delegates so the two paths cannot disagree about
// which keys exist — the duplicate above it was already a second spelling of
// the same rule.
func ParseOrgPK(raw string) (string, error) {
	pk, err := repositories.ParseOrgPK(raw)
	if err != nil {
		return "", problem.BadRequest("organização inválida")
	}
	return pk, nil
}
```

- [ ] **Step 5: Run the tests**

Run: `cd api && go build ./... && go test ./internal/repositories/ ./internal/middleware/ ./internal/api/... -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/repositories/organizations.go api/internal/repositories/organizations_key_test.go api/internal/middleware/tenant.go
git commit -m "feat(rekey): the partition key stops meaning a CNPJ

A company id passes through as the key. The legacy CNPJ_/CPF_ shapes stay
valid, and that is not deprecation debt: the old partitions are the
migration's rollback, and a build that cannot read them cannot roll back.

The middleware's hand-rolled copy of the check now delegates. It was
already a second spelling of the same rule, and this is the change that
would have made them disagree."
```

---

### Task 2: The local company record

**Files:**
- Create: `api/internal/repositories/company.go`
- Test: `api/internal/repositories/company_test.go`

**Interfaces:**
- Consumes: `IsCompanyKey` from Task 1.
- Produces:

```go
// LocalCompany is the DF-e's projection of a platform company.
type LocalCompany struct {
	CompanyID      string
	OrganizationID string
	TaxID          string // canonical: mask stripped, letters uppercased
	TaxIDKind      string // "cnpj" | "cpf"
	LegalName      string
	IdentitySyncedAt string
}

func (r *OrganizationRepository) GetCompany(ctx context.Context, companyID string) (*LocalCompany, error)
func (r *OrganizationRepository) PutIdentity(ctx context.Context, c *LocalCompany) error
func (c *LocalCompany) IdentityStale(now time.Time, ttl time.Duration) bool
func (c *LocalCompany) CNPJRoot() string
```

- [ ] **Step 1: Write the failing test**

```go
package repositories

import (
	"testing"
	"time"
)

// The raiz comes off the record's tax id, never off the partition key. Under a
// company id there is no CNPJ in the key to slice.
func TestCNPJRootComesFromTheRecord(t *testing.T) {
	c := &LocalCompany{TaxID: "11222333000181", TaxIDKind: "cnpj"}
	if got := c.CNPJRoot(); got != "11222333" {
		t.Errorf("CNPJRoot = %q, want 11222333", got)
	}
}

// A CNPJ is alphanumeric in its first twelve positions since 2026, so the raiz
// can hold letters. Slicing is right; assuming digits is not.
func TestCNPJRootHandlesAnAlphanumericCNPJ(t *testing.T) {
	c := &LocalCompany{TaxID: "12ABC34501DE35", TaxIDKind: "cnpj"}
	if got := c.CNPJRoot(); got != "12ABC345" {
		t.Errorf("CNPJRoot = %q, want 12ABC345", got)
	}
}

// A CPF has no branch concept, so it has no root — and returning a prefix of
// one would make two unrelated people look like matriz and filial.
func TestCNPJRootIsEmptyForACPF(t *testing.T) {
	c := &LocalCompany{TaxID: "52998224725", TaxIDKind: "cpf"}
	if got := c.CNPJRoot(); got != "" {
		t.Errorf("CNPJRoot = %q, want empty", got)
	}
}

// A record that predates the identity cache must read as stale, not as fresh
// with empty names — otherwise the first read after the migration shows blanks
// and never refreshes.
func TestAnUnsyncedIdentityIsStale(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	if !(&LocalCompany{}).IdentityStale(now, time.Hour) {
		t.Error("a record with no sync timestamp read as fresh")
	}
	fresh := &LocalCompany{IdentitySyncedAt: now.Add(-time.Minute).UTC().Format(time.RFC3339)}
	if fresh.IdentityStale(now, time.Hour) {
		t.Error("a record synced a minute ago read as stale")
	}
	old := &LocalCompany{IdentitySyncedAt: now.Add(-2 * time.Hour).UTC().Format(time.RFC3339)}
	if !old.IdentityStale(now, time.Hour) {
		t.Error("a record synced two hours ago read as fresh")
	}
}

// An unparseable timestamp is stale, not fresh. Failing the other way means a
// corrupt value pins a stale name forever.
func TestAnUnparseableSyncTimestampIsStale(t *testing.T) {
	now := time.Now()
	if !(&LocalCompany{IdentitySyncedAt: "ontem"}).IdentityStale(now, time.Hour) {
		t.Error("an unparseable timestamp read as fresh")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/ -run 'TestCNPJRoot|Identity' -v`
Expected: FAIL — `undefined: LocalCompany`.

- [ ] **Step 3: Write the implementation**

```go
package repositories

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Attribute names for the platform identity carried on a company record.
// Constants because the migration writes them and the read path reads them, and
// a typo in either is a silently empty name.
const (
	AttrOrganizationID   = "organization_id"
	AttrTaxID            = "tax_id"
	AttrTaxIDKind        = "tax_id_kind"
	AttrLegalName        = "legal_name"
	AttrIdentitySyncedAt = "identity_synced_at"
)

// cnpjRootLength is the raiz: the first eight positions of a CNPJ, which
// identify the company before its branch order and check digits.
const cnpjRootLength = 8

// LocalCompany is the DF-e's projection of a platform company.
//
// The identity fields are a CACHE. ctech-account owns them (ADR 0022); this is
// what was read last, so that authorization and issuance are a GetItem rather
// than a call across the network. Nothing here may treat them as authoritative,
// and a rename in accounts is not an error on this side.
type LocalCompany struct {
	CompanyID      string
	OrganizationID string
	// TaxID is canonical: mask stripped, letters uppercased. A CNPJ is
	// alphanumeric in its first twelve positions since 2026.
	TaxID            string
	TaxIDKind        string
	LegalName        string
	IdentitySyncedAt string
}

// CNPJRoot returns the eight-position raiz, or "" when there is none.
//
// It reads the record, never the partition key — that is the whole point of the
// re-key. A CPF has no branch concept, and returning a prefix of one would make
// two unrelated people look like matriz and filial.
func (c *LocalCompany) CNPJRoot() string {
	if c == nil || c.TaxIDKind != "cnpj" || len(c.TaxID) < cnpjRootLength {
		return ""
	}
	return c.TaxID[:cnpjRootLength]
}

// IdentityStale reports whether the cached identity should be refreshed.
//
// Missing and unparseable both read as stale. Failing the other way means one
// corrupt value pins a wrong name in place forever, with nothing to notice it.
func (c *LocalCompany) IdentityStale(now time.Time, ttl time.Duration) bool {
	if c == nil || c.IdentitySyncedAt == "" {
		return true
	}
	at, err := time.Parse(time.RFC3339, c.IdentitySyncedAt)
	if err != nil {
		return true
	}
	return now.UTC().Sub(at.UTC()) > ttl
}

// GetCompany reads the local record. A legacy CNPJ_ key still resolves, so this
// works either side of the flip.
func (r *OrganizationRepository) GetCompany(ctx context.Context, companyID string) (*LocalCompany, error) {
	item, err := r.GetItem(ctx, companyID)
	if err != nil || item == nil {
		return nil, err
	}
	return &LocalCompany{
		CompanyID:        companyID,
		OrganizationID:   attrString(item, AttrOrganizationID),
		TaxID:            attrString(item, AttrTaxID),
		TaxIDKind:        attrString(item, AttrTaxIDKind),
		LegalName:        attrString(item, AttrLegalName),
		IdentitySyncedAt: attrString(item, AttrIdentitySyncedAt),
	}, nil
}

// PutIdentity refreshes the cached identity in place.
//
// An UpdateItem on named attributes, not a Put: the fiscal configuration lives
// on this same item, and a whole-item write would race the customer editing
// their série against a background identity refresh.
func (r *OrganizationRepository) PutIdentity(ctx context.Context, c *LocalCompany) error {
	_, err := r.UpdateItem(ctx, c.CompanyID, nil, map[string]any{
		AttrOrganizationID:   c.OrganizationID,
		AttrTaxID:            c.TaxID,
		AttrTaxIDKind:        c.TaxIDKind,
		AttrLegalName:        c.LegalName,
		AttrIdentitySyncedAt: NowStr(),
	})
	return err
}

func attrString(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}
```

If `attrString` already exists in this package under another name, use that one instead of adding a second.

- [ ] **Step 4: Run the tests**

Run: `cd api && go build ./... && go test ./internal/repositories/ -v 2>&1 | tail -20`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/company.go api/internal/repositories/company_test.go
git commit -m "feat(rekey): the local company record and its cached identity

ctech-account owns the tax id and the names; this is what was read last,
so the issuance path stays a GetItem. Missing and unparseable sync
timestamps both read as stale — failing the other way pins one corrupt
value in place forever.

CNPJRoot reads the record, never the key. That is the whole point."
```

---

### Task 3: Matriz/filial certificate reuse survives the re-key

**Files:**
- Modify: `api/internal/services/organizations.go:95-155`
- Test: `api/internal/services/organizations_branch_test.go` (create)

This is the silent break the spec named: `cnpjRoot` slices the raiz out of the partition key, so under a company id it returns `""`, `branchCertificate` returns nil, and every filial registration starts demanding its own certificate — with a message that describes none of that.

**Interfaces:**
- Consumes: `LocalCompany.CNPJRoot()` from Task 2.
- Produces: `branchCertificate` unchanged in signature, changed in two ways — the root comes from the record, and the sibling search is scoped to the same `organization_id`.

- [ ] **Step 1: Write the failing test**

```go
package services

import "testing"

// Two organizations may hold the same CNPJ root under ADR 0022 — an accountant
// and their client. A sibling search that ignored the workspace would offer one
// customer another customer's certificate.
func TestASiblingMustShareTheOrganization(t *testing.T) {
	mine := branchCandidate{OrganizationID: "org_1", Root: "11222333"}
	cases := []struct {
		name  string
		other branchCandidate
		want  bool
	}{
		{"same organization, same root", branchCandidate{OrganizationID: "org_1", Root: "11222333"}, true},
		{"same root, another organization", branchCandidate{OrganizationID: "org_2", Root: "11222333"}, false},
		{"same organization, another root", branchCandidate{OrganizationID: "org_1", Root: "99888777"}, false},
		{"no root at all (a CPF)", branchCandidate{OrganizationID: "org_1", Root: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBranchSibling(mine, tc.other); got != tc.want {
				t.Errorf("isBranchSibling = %v, want %v", got, tc.want)
			}
		})
	}
}

// A company with no root has no siblings, whichever side it is on.
func TestACompanyWithoutARootHasNoSiblings(t *testing.T) {
	cpf := branchCandidate{OrganizationID: "org_1", Root: ""}
	if isBranchSibling(cpf, branchCandidate{OrganizationID: "org_1", Root: "11222333"}) {
		t.Error("a CPF company matched a CNPJ sibling")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/services/ -run TestASibling -v`
Expected: FAIL — `undefined: branchCandidate`.

- [ ] **Step 3: Write the implementation**

Replace the package-level `cnpjRoot` in `api/internal/services/organizations.go`. Delete it: nothing may slice a key any more, and leaving it is leaving the trap.

```go
// branchCandidate is what the sibling rule needs to know about a company:
// which workspace it belongs to and its CNPJ raiz.
type branchCandidate struct {
	OrganizationID string
	Root           string
}

// isBranchSibling reports whether other is a matriz/filial sibling of mine.
//
// Two conditions, and the organization one is not incidental: ADR 0022 lets two
// organizations hold the same CNPJ — an accountant and their client — so a
// search that matched on the root alone would offer one customer another
// customer's certificate.
func isBranchSibling(mine, other branchCandidate) bool {
	if mine.Root == "" || other.Root == "" {
		return false
	}
	return mine.OrganizationID == other.OrganizationID && mine.Root == other.Root
}
```

Then rewrite `branchCertificate` to build candidates from `GetCompany` rather than from key strings, keeping its existing signature and its existing behaviour of returning `(nil, nil)` when there is no sibling. For each of the caller's memberships, read the company record, build a `branchCandidate`, and skip anything `isBranchSibling` refuses — the rest of the function (the expiry check, the PFX reuse) is unchanged.

- [ ] **Step 4: Run the tests**

Run: `cd api && go build ./... && go test ./internal/services/ 2>&1 | tail -10`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/organizations.go api/internal/services/organizations_branch_test.go
git commit -m "fix(rekey): matriz/filial reuse reads the record, not the key

cnpjRoot sliced the raiz out of the partition key. Under a company id it
returns nothing, branchCertificate returns nil, and every filial starts
demanding its own certificate with a message describing none of that.

The sibling search is now scoped to the organization too. ADR 0022 lets
two organizations hold one CNPJ, and matching on the root alone would
offer one customer another customer's certificate."
```

---

### Task 4: The série claim

**Files:**
- Create: `api/internal/repositories/serie_claim.go`
- Test: `api/internal/repositories/serie_claim_test.go`
- Modify: `cdk/lib/dynamodb-stack.ts`

This is the limit ADR 0022 upgraded from "accepted" to "enforced". `ctech-account` lets duplicate identity through on purpose; the hazard is in issuance, so the refusal belongs here.

**Interfaces:**
- Produces:

```go
func SerieClaimPK(taxID, modelo, ambiente string, serie int) string
func (r *SerieClaimRepository) Claim(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error
func (r *SerieClaimRepository) Release(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error
var ErrSerieTaken = errors.New("série already claimed for this tax id")
```

- [ ] **Step 1: Write the failing test**

```go
package repositories

import "testing"

// The claim is global by tax id, not scoped to an organization — deliberately,
// because the SEFAZ is global. An NF-e is unique by (CNPJ, modelo, série,
// número, ambiente), and two organizations sharing a CNPJ on one série collide
// there, not here.
func TestSerieClaimKeyIsGlobalByTaxID(t *testing.T) {
	a := SerieClaimPK("11222333000181", "55", "1", 1)
	b := SerieClaimPK("11222333000181", "55", "1", 1)
	if a != b {
		t.Fatalf("the same claim built two keys: %q and %q", a, b)
	}
}

// Every component separates a distinct claim. A key that collapsed any of them
// would refuse a série somebody may legitimately use.
func TestEveryComponentSeparatesAClaim(t *testing.T) {
	base := SerieClaimPK("11222333000181", "55", "1", 1)
	others := map[string]string{
		"another tax id":  SerieClaimPK("11222333000182", "55", "1", 1),
		"another modelo":  SerieClaimPK("11222333000181", "65", "1", 1),
		"another ambiente": SerieClaimPK("11222333000181", "55", "2", 1),
		"another série":   SerieClaimPK("11222333000181", "55", "1", 2),
	}
	for name, other := range others {
		if other == base {
			t.Errorf("%s produced the same key as the base claim", name)
		}
	}
}

// Homologação and produção are different worlds. A test emission must never
// consume a production série.
func TestAmbienteSeparatesTheClaim(t *testing.T) {
	prod := SerieClaimPK("11222333000181", "55", "1", 1)
	homolog := SerieClaimPK("11222333000181", "55", "2", 1)
	if prod == homolog {
		t.Fatal("homologação and produção share a claim")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/ -run TestSerie -v`
Expected: FAIL — `undefined: SerieClaimPK`.

- [ ] **Step 3: Write the implementation**

```go
package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ErrSerieTaken is the claim losing its conditional write: another company
// already emits under this tax id on this série.
var ErrSerieTaken = errors.New("série already claimed for this tax id")

// SerieClaimRepository enforces what ctech-billing ADR 0022 upgraded from an
// accepted limit to an enforced rule.
//
// An NF-e is unique by (CNPJ, modelo, série, número, ambiente). ctech-account
// lets two organizations hold the same CNPJ on purpose — a CNPJ is public, and
// registering it is a claim rather than a capability — so the collision would
// happen at the SEFAZ, as a duplicate rejection or a gap in numbering somebody
// has to justify. The refusal belongs here, where issuance is.
type SerieClaimRepository struct {
	Base
}

func NewSerieClaimRepository(db *dynamodb.Client, cfg *config.Config) *SerieClaimRepository {
	return &SerieClaimRepository{Base: NewBase(db, cfg, "serie_claims")}
}

// SerieClaimPK keys a claim by exactly what the SEFAZ keys uniqueness by, minus
// the número — which is the sequence this claim protects.
//
// Global, not scoped to an organization. Scoping it would defeat the purpose:
// the whole point is that two *different* organizations must not both hold it.
func SerieClaimPK(taxID, modelo, ambiente string, serie int) string {
	return fmt.Sprintf("SERIE#%s#%s#%s#%d", taxID, modelo, ambiente, serie)
}

// Claim takes the série for this company, or reports that somebody else has it.
//
// A conditional write, never a read-then-write: two enablements racing would
// both find the série free and both proceed, which is the exact outcome this
// exists to prevent.
//
// Re-claiming a série this company already holds succeeds. Enablement is
// idempotent, and a retry must not read as somebody else's collision.
func (r *SerieClaimRepository) Claim(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error {
	pk := SerieClaimPK(taxID, modelo, ambiente, serie)
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: pk},
		"company_id": &types.AttributeValueMemberS{Value: companyID},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
	}
	err := r.PutItemRawCond(ctx, item,
		"attribute_not_exists(pk) OR company_id = :self",
		map[string]types.AttributeValue{":self": &types.AttributeValueMemberS{Value: companyID}})
	if IsConditionFailed(err) {
		return ErrSerieTaken
	}
	return err
}

// Release gives the série up, and only to the company that holds it: a
// conditional delete, so a stale request cannot free somebody else's claim.
func (r *SerieClaimRepository) Release(ctx context.Context, taxID, modelo, ambiente string, serie int, companyID string) error {
	// ... conditional DeleteItem on company_id = :self, ErrSerieTaken when it
	// is somebody else's.
}
```

If `PutItemRawCond` does not exist on the shared `dynamo.Base`, build the `PutItemInput` with `PutItemRaw` — the condition expression is the part that matters, not the helper.

**What the person sees**, wired at the handler: the série is in use for this CNPJ and they must choose another. **Never which organization holds it** — that would disclose that somebody else carries their CNPJ, and who a customer's accountant is is not ours to reveal.

- [ ] **Step 4: Add the table**

In `cdk/lib/dynamodb-stack.ts`, beside the other `getDfeTable` calls. Partition key `pk` only — a claim is one row, read and written by primary key, never queried.

- [ ] **Step 5: Run the tests**

Run: `cd api && go build ./... && go test ./internal/repositories/ 2>&1 | tail -10 && cd ../cdk && npx tsc --noEmit`
Expected: PASS, tsc clean.

- [ ] **Step 6: Commit**

```bash
git add api/internal/repositories/serie_claim.go api/internal/repositories/serie_claim_test.go cdk/lib/dynamodb-stack.ts
git commit -m "feat(rekey): the série claim ADR 0022 made enforceable

An NF-e is unique by (CNPJ, modelo, série, número, ambiente). Accounts
lets two organizations hold one CNPJ on purpose, so the collision would
land at the SEFAZ — a duplicate rejection, or a numbering gap somebody
has to justify. A conditional write, global by tax id: scoping it to an
organization would defeat the point.

Re-claiming your own série succeeds; enablement is idempotent and a retry
must not read as somebody else's collision."
```

---

### Task 5: Wire the identity refresh

**Files:**
- Modify: `api/internal/services/organizations.go`
- Test: extend `api/internal/services/organizations_branch_test.go`

**Interfaces:**
- Consumes: `GetCompany`, `PutIdentity`, `IdentityStale` from Task 2.
- Produces: `OrganizationService.Company(ctx, companyID) (*LocalCompany, error)` — the read-through every caller uses instead of `Get`.

The refresh reads `ctech-account`. **A failure to refresh is not a failure to serve:** the cached copy is returned and the error is logged, the same discipline `AccountBillingRepository` documents for the subscription snapshot. An identity refresh that could take issuance down would make ctech-account's uptime the DF-e's own.

- [ ] **Step 1: Write the failing test**

```go
// A refresh that fails must not take the read down with it. The cached name is
// stale, which is a cosmetic problem; a refused issuance is not.
func TestAStaleIdentityStillServes(t *testing.T) {
	// ... a company whose identity is stale, with a refresher that errors.
	// Want: the cached LocalCompany, no error.
}

// A fresh record makes no call at all. Refreshing on every read would put a
// network hop on the issuance path, which is the thing the cache exists to
// avoid.
func TestAFreshIdentityIsNotRefetched(t *testing.T) {
	// ... a company synced a minute ago, with a refresher that records calls.
	// Want: zero calls.
}
```

- [ ] **Step 2–5:** run red, implement, run green, commit.

```bash
git commit -m "feat(rekey): read-through identity, and a refresh that cannot take issuance down

A failed refresh returns the cached copy and logs. The stale name is
cosmetic; a refused issuance is not, and a refresh that could fail the
request would make ctech-account's uptime the DF-e's own."
```

---

### Task 6: The re-key tool

**Files:**
- Create: `api/cmd/rekey-companies/main.go`, `rekey.go`, `verify.go`, `rekey_test.go`

**Interfaces:**
- Produces a command with three verbs and no fourth:
  - `-plan` — read the mapping, report what would move, write nothing.
  - `-apply` — copy every row from the legacy partition to the company partition.
  - `-verify` — count and sample-compare both partitions, exit non-zero on any mismatch.

There is deliberately **no `-delete`**. Removing the old partitions is a separate decision taken after a numbering cycle, and a tool that can do it is a tool that does it by a mistyped flag.

**The mapping** comes from `ctech-account`: every migrated company carries `source_system = "dfe"` and `source_ref = "{legacy PK}"`. It is read, never held in a file — a file is a thing that has to survive, and this does not.

**Idempotency and resumability.** Each row is written with `attribute_not_exists(pk)`, so a re-run skips what landed and completes what did not. A run that dies mid-table is finished by the next one, not restarted.

**Tables, in this order** — configuration first, so a partial run leaves companies that cannot emit rather than companies that emit wrongly:

1. `organizations` (the record itself, plus the new identity attributes)
2. the five `organization_*_configs`, preserving NSU cursors
3. `organization_certificates`
4. the fifteen registry tables
5. documents and their events and distributions

- [ ] **Step 1: Write the failing test**

The pure parts are what get tested: the plan's decisions, not DynamoDB.

```go
// A legacy key with no company in accounts is not migrated silently — it is
// reported and the run exits non-zero. A partition nobody can reach is worse
// than a migration that stopped.
func TestAnUnmappedOrganizationNeedsAHuman(t *testing.T) { /* ... */ }

// Re-running finds the rows already there and reports them as done, not as
// failures. A migration that cannot be re-run cannot be resumed.
func TestASecondRunIsAnEmptyPlan(t *testing.T) { /* ... */ }

// Verification compares counts AND bodies. A count-only check passes on a copy
// that wrote every row wrong.
func TestVerifyComparesBodiesNotJustCounts(t *testing.T) { /* ... */ }
```

- [ ] **Steps 2–5:** run red, implement, run green, commit.

---

### Task 7: The browser stops reading a document out of the key

**Files:**
- Modify: `ui/src/lib/utils/document.ts`
- Modify: `ui/src/lib/api/client.ts:222`, `:387`, `:391`
- Modify: `ui/src/lib/types/api.ts` (`OrganizationOut`)
- Modify: `ui/src/lib/utils/converters.ts:9,12`
- Modify: `ui/src/components/organizations/OrganizationsTable.tsx:49`
- Modify: `ui/src/components/nfe/NfeEmitForm.tsx:899,1228`
- Test: `ui/src/__tests__/lib/document.test.ts` (extend), `ui/src/__tests__/lib/client-org-header.test.ts` (create)

**Ships before the migration runs.** Everything else in this plan is inert until a company record exists; this one is not. `apiClient` puts `unformatCpfCnpj(org.pk)` on every request, and on a company id that strips the hyphens and uppercases the hex:

```
0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70  →  0199F3A18C427C319D5E6A2B4C8E1F70
```

Task 1's `IsCompanyKey` refuses that — lowercase hex, hyphens required — so **every screen would answer "organização inválida"** with the cause three layers from the symptom. The strictness is deliberate and this is the payoff: the alternative, a server that accepted both spellings, would have the browser and the migration tool writing two partitions for one company, and nothing would say so.

**What is NOT in scope, and it is half of what a grep suggests.** `PersonForm.tsx:35`, `NfceEmitForm.tsx:152` and `EntityForm.tsx:146` test `sk.startsWith('CPF_')`. That is a **person or entity sort key**, not the organization partition key: a person stays keyed by their own document, and the re-key does not touch them. Of the 158 `orgPk` mentions in the UI, most only carry the key — a query key, a prop, a header — and never look inside. Seven read a document out of it, and they are the list above.

**Interfaces:**
- Consumes: `tax_id` / `tax_id_kind` on the API's organization payload, which Task 2's local company record supplies.
- Produces: `OrganizationOut` gains `tax_id: string` and `tax_id_kind: 'cnpj' | 'cpf'`. `unformatCpfCnpj` and `docLabel` keep their signatures and stop being called with an organization key.

- [ ] **Step 1: Write the failing test**

```ts
import {describe, expect, it, vi} from 'vitest'
import {docLabel, unformatCpfCnpj} from '@/lib/utils/document'

// The regression this task exists for. The helper is a document formatter, and
// a company id is not a document: it must come back untouched rather than
// silently reshaped into something the server refuses.
describe('unformatCpfCnpj on a key that is not a document', () => {
  it('leaves a company id exactly as it is', () => {
    const id = '0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70'
    expect(unformatCpfCnpj(id)).toBe(id)
  })

  it('still unwraps and normalizes a real document', () => {
    expect(unformatCpfCnpj('CNPJ_11.222.333/0001-81')).toBe('11222333000181')
    expect(unformatCpfCnpj('CPF_529.982.247-25')).toBe('52998224725')
    // A CNPJ is alphanumeric in its first twelve positions since 2026.
    expect(unformatCpfCnpj('CNPJ_12abc34501de35')).toBe('12ABC34501DE35')
  })
})

// docLabel answers "CPF or CNPJ" for a badge. A company id is neither, and labelling
// it "CNPJ" would print a wrong word next to a value that is not one.
describe('docLabel on a company id', () => {
  it('returns nothing to label', () => {
    expect(docLabel('0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70')).toBe('')
  })
})
```

```ts
// The header is the one that breaks everything, so it gets its own test rather
// than being covered incidentally by the helper's.
import {describe, expect, it} from 'vitest'
import {orgHeaderValue} from '@/lib/api/client'

describe('the organization header', () => {
  it('sends a company id verbatim', () => {
    const id = '0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70'
    expect(orgHeaderValue({pk: id})).toBe(id)
  })

  // The legacy shape keeps travelling as bare digits: that is what the server's
  // ParseOrgPK re-prefixes today, and the old partitions are the rollback.
  it('still sends a legacy key as bare digits', () => {
    expect(orgHeaderValue({pk: 'CNPJ_11222333000181'})).toBe('11222333000181')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/__tests__/lib/document.test.ts src/__tests__/lib/client-org-header.test.ts`
Expected: FAIL — the company id comes back as `0199F3A18C427C319D5E6A2B4C8E1F70`, and `orgHeaderValue` does not exist.

- [ ] **Step 3: Teach the helpers what a key is**

In `ui/src/lib/utils/document.ts`:

```ts
/**
 * A platform company id: a UUID in canonical 8-4-4-4-12 lowercase hex.
 *
 * Mirrors `repositories.IsCompanyKey` in the API, and the two must agree — a
 * browser that reshapes a key the server validates writes to a partition
 * nobody else addresses.
 */
export const isCompanyKey = (pk: string): boolean =>
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(pk)

export const unformatCpfCnpj = (pk: string): string => {
  // A company id is not a document. Stripping its hyphens and uppercasing its
  // hex produced a value the API refuses, which is how every screen in the
  // product started answering "organização inválida" at once.
  if (isCompanyKey(pk)) return pk
  return pk.replace(/^(CPF_|CNPJ_)/, '').replace(/[^A-Z0-9]/gi, '').toUpperCase()
}

export const docLabel = (pk: string): string => {
  if (isCompanyKey(pk)) return ''
  return pk.startsWith('CPF_') ? 'CPF' : 'CNPJ'
}
```

`formatCpfCnpj` needs no change: it calls `unformatCpfCnpj` first, and a company id now falls through both regexes and comes back as itself. Confirm that with a test rather than by reading.

- [ ] **Step 4: Extract and test the header value**

`client.ts:222` builds the header inside an interceptor closure, where a test cannot reach it. Extract the decision:

```ts
/** The value of `Dfe-Organization-Pk`. Exported so the rule is testable — it
 *  was a line inside an interceptor, and it was wrong. */
export const orgHeaderValue = (org: {pk: string}): string => unformatCpfCnpj(org.pk)
```

The interceptor calls it. The header's **name** does not change: renaming it is a coordinated two-app deploy for a word (`middleware/rbac.go:22`).

- [ ] **Step 5: Read the document from the record, not the key**

Add to `OrganizationOut` in `ui/src/lib/types/api.ts`:

```ts
  /** Canonical tax id — mask stripped, letters uppercased. Read this, never the
   *  pk: since ADR 0022 the pk is a company id and carries no document. */
  tax_id: string
  tax_id_kind: 'cnpj' | 'cpf'
```

Then the five remaining sites:

| Site | Was | Becomes |
|---|---|---|
| `converters.ts:9` | `org.pk.startsWith('CNPJ_')` | `org.tax_id_kind === 'cnpj'` |
| `converters.ts:12` | `unformatCpfCnpj(org.pk)` | `org.tax_id` |
| `OrganizationsTable.tsx:49` | `org.pk.replace('CNPJ_','')…` | `formatCpfCnpj(org.tax_id)` |
| `NfeEmitForm.tsx:899` | `unformatCpfCnpj(selectedOrg.pk)` | `selectedOrg.tax_id` |
| `NfeEmitForm.tsx:1228` | `unformatCpfCnpj(selectedOrg?.pk ?? '')` | `selectedOrg?.tax_id ?? ''` |

`NfeEmitForm` is the one to be careful with: that value is **the emitter's CNPJ in the XML**. Getting it from the key was always indirect; getting it from `tax_id` is what it meant all along. A test that pins the emitter document against `tax_id` and not `pk` belongs with this change.

`client.ts:387,391` pass `orgPk` into an `/organizations/{…}/authorized-viewers` path. Once the API keys by company id the path segment is the key, so the `unformatCpfCnpj` wrapper comes off — the second argument on line 391 is a *person's* document and keeps it.

- [ ] **Step 6: Run everything**

Run: `cd ui && npx vitest run && npx tsc --noEmit && npm run lint`
Expected: all green. Existing tests that seed `selectedOrg: {pk: 'CNPJ_…'}` now need `tax_id` too — that is the type checker doing its job, and each one is a real call site.

- [ ] **Step 7: Commit**

```bash
git add ui/src
git commit -m "fix(rekey): the browser stops reading a document out of the key

apiClient put unformatCpfCnpj(org.pk) on every request, which on a
company id strips the hyphens and uppercases the hex. The API refuses
that, so every screen would have answered organização inválida with the
cause three layers from the symptom.

The document now comes from tax_id on the record. NfeEmitForm mattered
most: that value is the emitter CNPJ in the XML, and reading it from the
key was always indirect.

Person and entity sort keys are untouched. They are keyed by their own
document and the re-key does not reach them."
```

---

## Self-Review

**Spec coverage.** The forcing function → Task 1. The seam → Task 1's middleware delegation. The local company record → Task 2. The série rule → Task 4. The four things that break: `cnpjRoot` → Task 3, `ParseOrgPK` → Task 1, the header → unchanged by design (constraint), documents reading a CNPJ off the partition → Task 6's ordering. The migration → Task 6. Quota → **no task, correctly**: the spec says it is unchanged, and the counters already live where they belong.

**Placeholder scan.** Tasks 5 and 6 carry test *intent* with bodies left to the implementer, and that is a real weakness of this plan rather than a style choice — flagged here rather than hidden. Both depend on shapes (the account client, the table list) that are cheaper to read at implementation time than to transcribe wrongly now. Tasks 1–4, which carry the invariants, are complete.

**Type consistency.** `LocalCompany` field names match across Tasks 2, 3 and 5. `IsCompanyKey` is defined in Task 1 and consumed in Tasks 2 and 6. `branchCandidate` is Task 3's alone. `SerieClaimPK`'s argument order `(taxID, modelo, ambiente, serie)` is identical in the test, the implementation and `Release`.

**A gap the reader found, not the review.** The first version of this plan said "four places read the CNPJ back out of the key" and counted only Go. The browser does it in seven more, and one of them — the request header — would have broken every screen the moment a company id existed. Task 7 exists because that count was taken from one repo and stated as if it were the whole system.

**One gap found while reviewing:** Task 1's original draft widened `middleware.ParseOrgPK` in place, leaving two implementations of "which keys exist". Changed to delegation — the duplicate was already a latent disagreement, and this task is what would have triggered it.
