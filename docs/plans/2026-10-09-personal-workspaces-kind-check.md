# Personal workspaces — the DF-e's kind check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The DF-e refuses — as "no access" — any company whose ctech-account workspace is not an organization, on company reach and on company link, so a personal space can never issue fiscal documents even if ctech-account's own refusal regresses.

**Architecture:** One choke point. Both questions the DF-e asks ctech-account about a company go through `internal/accountclient` (`Reach`, `Company`), and every reach decision — the RBAC middleware via `ReachService.MayAct`, and `LinkService.Link` — consumes them. The client reads `organization_kind` and turns a non-organization reach into a plain refusal (which `ReachService` caches as one, so a cached answer is refused on read too) and a non-organization identity into `ErrNotAnOrganization`, which `Link` answers with the same 403 as every other link refusal, before writing anything.

**Tech Stack:** Go 1.27, standard `testing` (`api/`).

**Spec:** [`docs/specs/2026-10-09-personal-workspaces-in-dfe.md`](../specs/2026-10-09-personal-workspaces-in-dfe.md). Upstream: ctech-account `docs/specs/2026-10-09-personal-workspaces.md` (artur-oliveira/ctech-account#46 sends `organization_kind`).

## Global Constraints

- An absent `organization_kind` is an organization: today's behaviour, byte for byte. ctech-account sends nothing before its #46 deploys.
- Fail closed: any kind other than absent or `organization` is not consent.
- A refusal stays indistinguishable: a company in a space answers the same 403 detail as a missing edge.
- An outage stays an error, never a refusal (existing reach rule).
- No new cache key or cache shape: `dfe:reach:{company}:{user}` and `reachAnswer` are unchanged.
- `gofmt` clean on touched files; `go vet ./...` clean. Commit messages carry no attribution trailer.

## Review Focus

1. **A reach answer from before the upstream deploy** (no `organization_kind`) must grant exactly as today — Task 1 `TestReachWithNoKindIsAnOrganization`.
2. **A company in a space** must be refused on reach without being reported as an outage (a plain refusal, so it is cached and does not page anyone) — Task 1 `TestReachRefusesACompanyInASpace`.
3. **A kind added upstream later** must not inherit a grant — Task 1 `TestReachRefusesAnUnknownKind`.
4. **A prober** must not learn from the link answer that an id belongs to a space — Task 2 `TestLinkRefusesACompanyOutsideAnOrganization` (same detail and status as a missing edge).
5. **An identity outage during link** must still surface as an error, not as "no access" — Task 2 (the last assertion of the same test).

**Deviation from the spec, recorded in Task 3:** the spec placed the check in `reachAnswer`/`checkReach`/the middleware. It lives in the client instead — the one place both answers pass through — so the cache, the middleware and `checkReach` need no change and cannot disagree.

---

All commands run from `api/`.

### Task 1: The account client refuses a company outside an organization

**Files:**
- Modify: `api/internal/accountclient/reach.go`
- Test: `api/internal/accountclient/kind_test.go`

**Interfaces:**
- Consumes: `serving(t, status, body) *Client` (existing test helper, `reach_test.go`); `(*Client).reachWithToken` (existing).
- Produces:
  - `ErrNotAnOrganization` (sentinel error).
  - `Identity.OrganizationKind string` (`json:"organization_kind"`).
  - `(*Client).companyWithToken(ctx, token, organizationID, companyID) (*Identity, error)` — `Company` minus the token, split like `reachWithToken`.
  - Behaviour: `Reach` answers `("", false, nil)` for `may_act: true` with a kind other than absent/`organization`; `Company` answers `ErrNotAnOrganization` for such a kind.

- [ ] **Step 1: Write the failing test** — `api/internal/accountclient/kind_test.go`

```go
package accountclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// A company whose workspace is a personal space is not one this product may
// issue for. ctech-account refuses to put a company in a space; this is the
// check that holds if that refusal ever regresses
// (docs/specs/2026-10-09-personal-workspaces-in-dfe.md).
func TestReachRefusesACompanyInASpace(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":true,"organization_id":"org_1","organization_kind":"personal"}`)
	orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
	if err != nil || ok || orgID != "" {
		t.Fatalf("got %q %v %v, want a plain refusal", orgID, ok, err)
	}
}

// An unknown kind is not consent either.
func TestReachRefusesAnUnknownKind(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":true,"organization_id":"org_1","organization_kind":"household"}`)
	if _, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1"); err != nil || ok {
		t.Fatalf("ok=%v err=%v, want a plain refusal", ok, err)
	}
}

// Before ctech-account sends the field, every workspace is an organization —
// today's behaviour, unchanged.
func TestReachWithNoKindIsAnOrganization(t *testing.T) {
	for _, body := range []string{
		`{"may_act":true,"organization_id":"org_1"}`,
		`{"may_act":true,"organization_id":"org_1","organization_kind":"organization"}`,
	} {
		c := serving(t, http.StatusOK, body)
		orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
		if err != nil || !ok || orgID != "org_1" {
			t.Fatalf("%s: got %q %v %v, want a grant", body, orgID, ok, err)
		}
	}
}

func TestIdentityRefusesACompanyInASpace(t *testing.T) {
	c := serving(t, http.StatusOK, `{"organization_id":"org_1","organization_kind":"personal","tax_id":"11222333000181","tax_id_kind":"cnpj","legal_name":"Acme"}`)
	if _, err := c.companyWithToken(context.Background(), "tok", "org_1", "cmp_1"); !errors.Is(err, ErrNotAnOrganization) {
		t.Fatalf("err = %v, want ErrNotAnOrganization", err)
	}
}

func TestIdentityWithNoKindIsAnOrganization(t *testing.T) {
	c := serving(t, http.StatusOK, `{"organization_id":"org_1","tax_id":"11222333000181","tax_id_kind":"cnpj","legal_name":"Acme"}`)
	ident, err := c.companyWithToken(context.Background(), "tok", "org_1", "cmp_1")
	if err != nil || ident.LegalName != "Acme" {
		t.Fatalf("got %+v %v", ident, err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/accountclient/`
Expected: FAIL — build fails: `c.companyWithToken undefined`, `undefined: ErrNotAnOrganization`.

- [ ] **Step 3: Apply the implementation** — save as `task1.patch`, run `git apply task1.patch` from the repository root

```diff
diff --git a/api/internal/accountclient/reach.go b/api/internal/accountclient/reach.go
index 0b05c1c..b97ad92 100644
--- a/api/internal/accountclient/reach.go
+++ b/api/internal/accountclient/reach.go
@@ -29,6 +29,20 @@ import (
 // company that is not there is a link to refuse, not an outage to retry.
 var ErrCompanyNotFound = errors.New("company not found in ctech-account")
 
+// ErrNotAnOrganization is a company whose workspace is not an organization —
+// a personal space, or a kind this product does not know. ctech-account refuses
+// to put a company in a space; this holds if that refusal ever regresses
+// (docs/specs/2026-10-09-personal-workspaces-in-dfe.md).
+var ErrNotAnOrganization = errors.New("company does not belong to an organization")
+
+// kindOrganization is the only workspace kind this product issues for. An
+// absent kind is one: ctech-account sent none before spaces existed.
+const kindOrganization = "organization"
+
+// isOrganization reads the kind ctech-account reports. Anything but absent or
+// "organization" — a space, or a kind added later — is not consent.
+func isOrganization(kind string) bool { return kind == "" || kind == kindOrganization }
+
 const Scope = "internal:account:company-actor"
 
 const (
@@ -78,8 +92,9 @@ func New(cfg Config) *Client {
 func (c *Client) Enabled() bool { return c != nil }
 
 type reachResponse struct {
-	MayAct         bool   `json:"may_act"`
-	OrganizationID string `json:"organization_id"`
+	MayAct           bool   `json:"may_act"`
+	OrganizationID   string `json:"organization_id"`
+	OrganizationKind string `json:"organization_kind"`
 }
 
 // Reach answers whether userID may act for companyID, and which organization
@@ -129,16 +144,23 @@ func (c *Client) reachWithToken(ctx context.Context, token, companyID, userID st
 	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
 		return "", false, fmt.Errorf("decoding the reach answer: %w", err)
 	}
+	if out.MayAct && !isOrganization(out.OrganizationKind) {
+		// A plain refusal, not an error: it is an answer about this company,
+		// and the reach cache stores it like any other refusal, so a cached
+		// answer is refused on read too.
+		return "", false, nil
+	}
 	return out.OrganizationID, out.MayAct, nil
 }
 
 // Identity is who a company is, as ctech-account records it.
 type Identity struct {
-	OrganizationID string `json:"organization_id"`
-	TaxID          string `json:"tax_id"`
-	TaxIDKind      string `json:"tax_id_kind"`
-	LegalName      string `json:"legal_name"`
-	TradeName      string `json:"trade_name"`
+	OrganizationID   string `json:"organization_id"`
+	OrganizationKind string `json:"organization_kind"`
+	TaxID            string `json:"tax_id"`
+	TaxIDKind        string `json:"tax_id_kind"`
+	LegalName        string `json:"legal_name"`
+	TradeName        string `json:"trade_name"`
 }
 
 // Company reads a company's identity.
@@ -155,7 +177,13 @@ func (c *Client) Company(ctx context.Context, organizationID, companyID string)
 	if err != nil {
 		return nil, fmt.Errorf("minting a service token: %w", err)
 	}
+	return c.companyWithToken(ctx, token, organizationID, companyID)
+}
 
+// companyWithToken is the identity request itself, split from the token for
+// the same reason reachWithToken is: so which answer means what is testable
+// without a token endpoint.
+func (c *Client) companyWithToken(ctx context.Context, token, organizationID, companyID string) (*Identity, error) {
 	path := fmt.Sprintf("%s/v1.0/internal/organizations/%s/companies/%s",
 		c.baseURL, url.PathEscape(organizationID), url.PathEscape(companyID))
 	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
@@ -186,5 +214,8 @@ func (c *Client) Company(ctx context.Context, organizationID, companyID string)
 		// worse message than this one.
 		return nil, fmt.Errorf("ctech-account returned a company with no tax id")
 	}
+	if !isOrganization(out.OrganizationKind) {
+		return nil, ErrNotAnOrganization
+	}
 	return &out, nil
 }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/accountclient/ -v && go vet ./... && go test ./...`
Expected: the five new tests and the six existing ones PASS; every package `ok`.

- [ ] **Step 5: Commit**

```bash
git add api/internal/accountclient/reach.go api/internal/accountclient/kind_test.go
git commit -m "feat(accountclient): refuse a company whose workspace is not an organization"
```

### Task 2: Link answers a company outside an organization with "no access"

**Files:**
- Modify: `api/internal/services/link.go`
- Test: `api/internal/services/link_test.go`

**Interfaces:**
- Consumes: Task 1 `accountclient.ErrNotAnOrganization`; existing `checkReach`, `accountclient.ErrCompanyNotFound`, test constant `linkOrg`.
- Produces: `identityProblem(err error) *problem.Problem` — the refusal an identity error becomes, or nil for an outage. `Link` uses it in place of its inline `ErrCompanyNotFound` branch.

Reach already refuses first in `Link` (Task 1), so this is the second line: it holds if the two upstream routes ever disagree.

- [ ] **Step 1: Write the failing test** — append to `api/internal/services/link_test.go` (apply with `git apply`)

```diff
diff --git a/api/internal/services/link_test.go b/api/internal/services/link_test.go
index cf1c4b9..72ef942 100644
--- a/api/internal/services/link_test.go
+++ b/api/internal/services/link_test.go
@@ -2,7 +2,10 @@ package services
 
 import (
 	"errors"
+	"fmt"
 	"testing"
+
+	"gopkg.aoctech.app/dfe/api/internal/accountclient"
 )
 
 const (
@@ -66,3 +69,24 @@ func TestEveryLinkRefusalLooksTheSame(t *testing.T) {
 		t.Fatalf("distinguishable:\n  no edge: %s\n  crossed: %s", noEdge.Detail, crossed.Detail)
 	}
 }
+
+// A company whose workspace is not an organization is refused with the same
+// "no access" as every other link refusal, before any local row is written.
+// ctech-account never puts a company in a personal space; this holds if that
+// ever regresses (docs/specs/2026-10-09-personal-workspaces-in-dfe.md).
+func TestLinkRefusesACompanyOutsideAnOrganization(t *testing.T) {
+	prob := identityProblem(fmt.Errorf("reading: %w", accountclient.ErrNotAnOrganization))
+	if prob == nil {
+		t.Fatal("a company in a space was linkable")
+	}
+	_, noEdge := checkReach(linkOrg, "", false, nil)
+	if prob.Detail != noEdge.Detail || prob.Status != noEdge.Status {
+		t.Fatalf("distinguishable from a missing edge: %+v vs %+v", prob, noEdge)
+	}
+	if identityProblem(accountclient.ErrCompanyNotFound) == nil {
+		t.Fatal("a missing company is no longer refused")
+	}
+	if identityProblem(errors.New("timeout")) != nil {
+		t.Fatal("an outage was turned into a refusal instead of an error")
+	}
+}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/services/ -run TestLinkRefusesACompanyOutsideAnOrganization`
Expected: FAIL — build fails: `undefined: identityProblem`.

- [ ] **Step 3: Apply the implementation** — save as `task2.patch`, run `git apply task2.patch` from the repository root

```diff
diff --git a/api/internal/services/link.go b/api/internal/services/link.go
index a86d696..c408577 100644
--- a/api/internal/services/link.go
+++ b/api/internal/services/link.go
@@ -92,8 +92,8 @@ func (s *LinkService) Link(ctx context.Context, organizationID, companyID, userI
 
 	ident, err := s.identity.Company(ctx, organizationID, companyID)
 	if err != nil {
-		if errors.Is(err, accountclient.ErrCompanyNotFound) {
-			return nil, problem.NotFound("empresa não encontrada na conta CTech")
+		if prob := identityProblem(err); prob != nil {
+			return nil, prob
 		}
 		return nil, fmt.Errorf("reading the company identity: %w", err)
 	}
@@ -134,6 +134,22 @@ func (s *LinkService) ensureOwner(ctx context.Context, companyID, userID, userNa
 	return s.orgUserRepo.Create(ctx, companyID, userID, repositories.RoleOwner, userID, userName, nil)
 }
 
+// identityProblem is the refusal an identity read turns into, or nil when the
+// error is an outage the caller must report as one. A company whose workspace is
+// not an organization answers the same "no access" as a missing edge: it is a
+// refusal about this company, and a distinct answer would tell a prober which
+// ids are spaces.
+func identityProblem(err error) *problem.Problem {
+	switch {
+	case errors.Is(err, accountclient.ErrCompanyNotFound):
+		return problem.NotFound("empresa não encontrada na conta CTech")
+	case errors.Is(err, accountclient.ErrNotAnOrganization):
+		_, prob := checkReach("", "", false, nil)
+		return prob
+	}
+	return nil
+}
+
 // checkReach decides whether a link may proceed, given what ctech-account said.
 //
 // Split from Link so the decision is testable without a database, because it is
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/services/ -run 'TestLink|TestEveryLinkRefusal' -v && go vet ./... && go test ./...`
Expected: PASS; every package `ok`.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/link.go api/internal/services/link_test.go
git commit -m "feat(link): a company outside an organization answers no access"
```

### Task 3: Record where the check lives

**Files:**
- Modify: `docs/specs/2026-10-09-personal-workspaces-in-dfe.md`

- [ ] **Step 1: Append to the end of the spec**

```markdown

## Amendment, implementation (2026-10-09)

The check lives in `internal/accountclient`, not in `reachAnswer`, `checkReach` and the middleware as
"What the DF-e adds" first said. Both questions this product asks about a company pass through that
client, so one place decides:

- `Reach` turns `may_act: true` with a kind other than absent or `organization` into a plain refusal.
  `ReachService` caches it as a refusal, so a cached answer is refused on read with no new cache field,
  and an entry from before this deploy (no kind) still reads as it always did.
- `Company` answers `ErrNotAnOrganization`, which `Link` turns into the same 403 as every other link
  refusal (`identityProblem`), before writing any row.
```

- [ ] **Step 2: Verify and commit**

Run: `cd api && go vet ./... && go test ./...`
Expected: every package `ok`.

```bash
git add docs/specs/2026-10-09-personal-workspaces-in-dfe.md
git commit -m "docs: personal workspaces in the DF-e — record where the kind check lives"
```

## Deploy order (outside this plan)

After ctech-account #46. Deploying this first is safe: with no `organization_kind` on the wire, nothing changes.
