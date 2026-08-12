# Manifestação manual e importação por chave de acesso (NF-e) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a user manually manifest a destined NF-e (any of the 4 SEFAZ event types) and manually force a fresh consulta-por-chave, from both the NF-e detail screen and the Distribuição screen — fixing the gap where a late `resNFe` (auto-Ciência rejected with SEFAZ code 596) never resolves into the complete NF-e.

**Architecture:** Reuse the existing manifestation endpoint and the existing async `cons_ch_nfe` worker job end-to-end; the only new backend work is (a) converting the one remaining synchronous distribution lookup to the same enqueue pattern the codebase already uses for `dist_nsu`, (b) an access-key structural validator applied at the API boundary (defense in depth — never trust the frontend), and (c) a pre-existing WebSocket-notification bug fix. Frontend work is two buttons + two modals + one access-key validator/mask module, wired to the existing WebSocket → toast → query-invalidation pipeline.

**Tech Stack:** Go (Fiber v3, DynamoDB) for `api`/`worker`; Next.js 16 + TypeScript + Zod + TanStack Query for `ui`. No new AWS infrastructure.

**Spec:** `docs/specs/2026-08-12-manifestacao-importacao-nfe.md`

## Global Constraints

- RFC 7807 Problem JSON for every api/worker error — never raw errors or `fiber.Map` (project `CLAUDE.md`).
- Every string key / numeric code / event type MUST be a named constant — no magic literals.
- `api` layer separation is strictly enforced: Route parses + calls one service method; Service holds business logic; Repository/raw DynamoDB calls never appear in routes (`api/CLAUDE.md`).
- `worker` handlers must stay idempotent (SQS is at-least-once) (`worker/CLAUDE.md`).
- `ui`: all API calls go through `ApiClient`; `npx eslint src --ext .ts,.tsx` must pass with zero errors/warnings before commit; every async action needs a loading state (`ui/CLAUDE.md`).
- No new CDK/infrastructure — the `distribution` SQS queue and `distribution-worker` Lambda already handle `cons_ch_nfe`.
- The access-key validator logic (UF codes, CNPJ/CPF-xor with check digit, mod=55, tpEmis, cDV) MUST be identical in Go (`api/internal/validation`) and TypeScript (`ui/src/lib/utils/access-key.ts`) — one is not "the frontend copy of the other," both are authoritative and must be kept in lock-step (spec §E, `Fora de escopo`).
- Every code change in this plan must be paired with the corresponding `DOCS.md`/`CONDUCT.md` update (project `CLAUDE.md` Mandatory Documentation Policy) — Task 12 covers this explicitly, but do not defer doc edits that are naturally part of an earlier task's diff.

---

## Task 1: Access-key structural validator (Go, `api/internal/validation`)

**Files:**
- Create: `api/internal/validation/access_key.go`
- Modify: `api/internal/validation/validation.go:56-68` (register the new tag)
- Modify: `api/internal/validation/validation.go:170-226` (add the `dfe_access_key` message case)
- Test: `api/internal/validation/access_key_test.go`

**Interfaces:**
- Produces: `func ValidAccessKey(s string) bool` (exported — used directly by the manifestation route for its path param, since that route has no struct body to tag) and the registered validator tag `dfe_access_key` (used on the new `POST /distributions/nfe/key` body field in Task 3).
- Consumes: `services.UFCode` (`api/internal/services/shared.go:84`, already exported) for the IBGE UF code set — do not redeclare it (project DRY rule). `ValidCPF`/`ValidCNPJ` (same package, `api/internal/validation/validators.go:119,152`).

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/validation/access_key_test.go
package validation

import "testing"

// A real authorized NF-e access key used across the worker test suite
// (worker/internal/service/distribution_test.go testAK) — numeric CNPJ, cUF
// 35 (SP), AAMM 2505, mod 55, tpEmis 1. Recomputed cDV below.
const validAccessKeyNumeric = "35250512345678000195550010000000011000000011"

func TestValidAccessKey_ValidNumericCNPJ(t *testing.T) {
	t.Parallel()
	if !ValidAccessKey(validAccessKeyNumeric) {
		t.Fatalf("ValidAccessKey(%q) = false, want true", validAccessKeyNumeric)
	}
}

func TestValidAccessKey_WrongLength(t *testing.T) {
	t.Parallel()
	if ValidAccessKey(validAccessKeyNumeric[:43]) {
		t.Fatal("expected false for a 43-char key")
	}
}

func TestValidAccessKey_InvalidUF(t *testing.T) {
	t.Parallel()
	key := "99" + validAccessKeyNumeric[2:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for cUF=99 (not an IBGE UF code)")
	}
}

func TestValidAccessKey_InvalidMonth(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:2] + "2513" + validAccessKeyNumeric[6:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for AAMM month=13")
	}
}

func TestValidAccessKey_WrongMod(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:20] + "65" + validAccessKeyNumeric[22:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for mod=65 (NFC-e, out of scope)")
	}
}

func TestValidAccessKey_InvalidTpEmisNFCeOnly(t *testing.T) {
	t.Parallel()
	key := validAccessKeyNumeric[:34] + "9" + validAccessKeyNumeric[35:]
	if ValidAccessKey(key) {
		t.Fatal("expected false for tpEmis=9 (NFC-e contingency only)")
	}
}

func TestValidAccessKey_BadCheckDigit(t *testing.T) {
	t.Parallel()
	last := validAccessKeyNumeric[43]
	bad := byte('0')
	if last == '0' {
		bad = '1'
	}
	key := validAccessKeyNumeric[:43] + string(bad)
	if ValidAccessKey(key) {
		t.Fatal("expected false for a corrupted cDV")
	}
}

func TestValidAccessKey_CPFPrefixedDoc(t *testing.T) {
	t.Parallel()
	// CPF-in-CNPJ-slot convention: "000" + 11-digit CPF (529.982.247-25, valid).
	// cUF/AAMM/mod/serie/nNF/tpEmis/cNF kept from validAccessKeyNumeric; cDV
	// must be recomputed by the test using calcAccessKeyDV since the doc
	// segment changed the first 43 characters.
	base := validAccessKeyNumeric[:6] + "00052998224725" + validAccessKeyNumeric[20:43]
	dv := calcAccessKeyDV(base)
	key := base + string(rune('0'+dv))
	if !ValidAccessKey(key) {
		t.Fatalf("ValidAccessKey(%q) = false, want true (CPF-prefixed doc)", key)
	}
}

func TestValidAccessKey_BothCPFAndCNPJInvalid(t *testing.T) {
	t.Parallel()
	// "000" prefix with a CPF that fails its own check digit.
	base := validAccessKeyNumeric[:6] + "00052998224724" + validAccessKeyNumeric[20:43]
	dv := calcAccessKeyDV(base)
	key := base + string(rune('0'+dv))
	if ValidAccessKey(key) {
		t.Fatal("expected false for an invalid CPF check digit")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/validation/... -run TestValidAccessKey -v`
Expected: FAIL — `ValidAccessKey` and `calcAccessKeyDV` undefined.

- [ ] **Step 3: Implement `access_key.go`**

```go
// api/internal/validation/access_key.go
package validation

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	accessKeyLen    = 44
	accessKeyModNFe = "55" // this feature is NF-e-only (mod 65 = NFC-e, out of scope)
)

// accessKeyValidTpEmis holds the tpEmis codes valid for chave-de-acesso
// manual entry — every code except 9, which SEFAZ reserves for NFC-e offline
// contingency and never appears on an NF-e.
var accessKeyValidTpEmis = map[byte]struct{}{
	'1': {}, '2': {}, '3': {}, '4': {}, '5': {}, '6': {}, '7': {},
}

// accessKeyUFCodes is the set of valid IBGE cUF codes, derived once from
// services.UFCode (api/internal/services/shared.go) — never redeclare a UF
// map (project DRY rule).
var accessKeyUFCodes = func() map[string]struct{} {
	m := make(map[string]struct{}, len(services.UFCode))
	for _, code := range services.UFCode {
		m[code] = struct{}{}
	}
	return m
}()

// accessKeyValidator wires ValidAccessKey into the shared go-playground
// validator instance under the "dfe_access_key" tag.
func accessKeyValidator(fl validator.FieldLevel) bool {
	return ValidAccessKey(fl.Field().String())
}

// ValidAccessKey validates an NF-e access key (chave de acesso) beyond its
// 44-character length: cUF, AAMM, CNPJ-xor-CPF (with check digit), mod=55,
// tpEmis, and the final cDV check digit. Exported for direct use on the
// manifestation route's `:access_key` path param, which has no request body
// struct to attach a `validate` tag to. Mirrors
// ui/src/lib/utils/access-key.ts — keep both in lock-step (see
// docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
func ValidAccessKey(s string) bool {
	if len(s) != accessKeyLen {
		return false
	}
	// Every position is a digit except the 14-char CNPJ/CPF segment [6:20),
	// which may contain uppercase letters (alphanumeric CNPJ, IN RFB 2229/2024).
	for i := 0; i < accessKeyLen; i++ {
		if i >= 6 && i < 20 {
			continue
		}
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	if _, ok := accessKeyUFCodes[s[0:2]]; !ok {
		return false
	}
	mm := int(s[4]-'0')*10 + int(s[5]-'0')
	if mm < 1 || mm > 12 {
		return false
	}
	if !validAccessKeyDoc(s[6:20]) {
		return false
	}
	if s[20:22] != accessKeyModNFe {
		return false
	}
	if _, ok := accessKeyValidTpEmis[s[34]]; !ok {
		return false
	}
	dv := int(s[43] - '0')
	return dv == calcAccessKeyDV(s[:43])
}

// validAccessKeyDoc validates the 14-char document segment: either a CPF
// (SEFAZ convention — "000" prefix + 11-digit CPF, both check-digit
// validated) or an alphanumeric CNPJ (IN RFB 2229/2024, check-digit
// validated) — never both, never neither.
func validAccessKeyDoc(doc string) bool {
	if strings.HasPrefix(doc, "000") {
		return ValidCPF(doc[3:])
	}
	return ValidCNPJ(doc)
}

// calcAccessKeyDV computes the NF-e access key's own check digit (cDV) over
// its first 43 characters: weights 2-9 cycling right-to-left, mod-11. NT
// 2023.002 defines an alphanumeric character's value here as (ASCII code −
// 48) — a DIFFERENT algorithm from the CNPJ field's own internal check
// digits, which use the IN RFB 2229/2024 A=10..Z=35 mapping (see ValidCNPJ,
// api/internal/validation/validators.go:152). Two distinct algorithms for two
// distinct check digits, both real, both required.
func calcAccessKeyDV(key43 string) int {
	weights := [8]int{2, 3, 4, 5, 6, 7, 8, 9}
	sum := 0
	for i, wi := len(key43)-1, 0; i >= 0; i, wi = i-1, wi+1 {
		sum += (int(key43[i]) - 48) * weights[wi%8]
	}
	rem := sum % 11
	if rem < 2 {
		return 0
	}
	return 11 - rem
}
```

- [ ] **Step 4: Register the tag and message (`validation.go`)**

In `api/internal/validation/validation.go`, add next to the other Brazilian-document validators (after line 62 `_ = v.RegisterValidation("cpfcnpj", cpfCnpjValidator)`):

```go
		_ = v.RegisterValidation("dfe_access_key", accessKeyValidator)
```

And in the `message` function's switch (after the `case "cpfcnpj":` block, ~line 181):

```go
	case "dfe_access_key":
		return "chave de acesso inválida"
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/validation/... -run TestValidAccessKey -v`
Expected: PASS (all 8 cases).

- [ ] **Step 6: Commit**

```bash
git add api/internal/validation/access_key.go api/internal/validation/access_key_test.go api/internal/validation/validation.go
git commit -m "feat(api): add structural access-key validator (dfe_access_key)"
```

---

## Task 2: Apply the validator to the manifestation route's path param

**Files:**
- Modify: `api/internal/api/v1/nfes.go:135-151`
- Test: `api/internal/api/v1/nfes_test.go` (create if it does not already exist — check first with `ls api/internal/api/v1/*_test.go`)

**Interfaces:**
- Consumes: `validation.ValidAccessKey` (Task 1).

**Context:** `POST /nfes/:access_key/manifestation` reads `c.Params("access_key")` directly and passes it to `svc.Manifestation` — nothing today rejects a malformed key before it reaches the service layer (which would call SEFAZ with garbage). This is the api-level defense-in-depth the user explicitly asked for.

- [ ] **Step 1: Write the failing test**

Check the actual test setup pattern first — read `api/internal/api/v1/dto_test.go` or any existing `*_test.go` in that package for how a fiber app + mock service gets built, then add:

```go
// api/internal/api/v1/nfes_manifestation_test.go
package v1

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestManifestationRoute_RejectsMalformedAccessKey(t *testing.T) {
	t.Parallel()
	app := newTestNFeApp(t) // helper already used by sibling tests in this package — reuse it, do not reinvent

	req := httptest.NewRequest("POST", "/nfes/not-a-valid-key/manifestation",
		strings.NewReader(`{"event_type":"210210","sequence_number":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}
```

If no `newTestNFeApp`-style helper exists in the package, inspect how `TestStructTagValidation` (validation package) and any router test under `api/internal/api/v1/` wire a minimal Fiber app + auth middleware stub, and adapt — do not invent a parallel test-harness convention.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/api/v1/... -run TestManifestationRoute_RejectsMalformedAccessKey -v`
Expected: FAIL (currently returns whatever the service layer does with a garbage key — likely a 404 or 500, not the intended 400).

- [ ] **Step 3: Implement the check**

In `api/internal/api/v1/nfes.go`, inside the `POST /:access_key/manifestation` handler (line 136), add the check right after `bindJSON`:

```go
	// POST /nfes/:access_key/manifestation
	g.Post("/:access_key/manifestation", perm.Require("create.nfe_events"), func(c fiber.Ctx) error {
		var body struct {
			EventType      string  `json:"event_type" validate:"required,oneof=210200 210210 210220 210240"`
			SequenceNumber int     `json:"sequence_number" validate:"omitempty,gte=1"`
			Justification  *string `json:"justification" validate:"omitempty,min=15,max=255"`
		}
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		accessKey := c.Params("access_key")
		if !validation.ValidAccessKey(accessKey) {
			return sendProblem(c, problem.BadRequest("chave de acesso inválida"))
		}
		userID, userName := resolveActor(c, userSvc)
		nfe, err := svc.Manifestation(c.Context(), middleware.GetOrgPK(c), accessKey, body.EventType, body.SequenceNumber, body.Justification, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, nfe)
	})
```

Add the two new imports to the top of `nfes.go`:

```go
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/validation"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd api && go test ./internal/api/v1/... -run TestManifestationRoute_RejectsMalformedAccessKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/api/v1/nfes.go api/internal/api/v1/nfes_manifestation_test.go
git commit -m "feat(api): validate access_key path param on manifestation route"
```

---

## Task 3: Convert `LookupByKey` to an async enqueue + new route (with body validation)

**Files:**
- Modify: `api/internal/services/distributions.go:254-273` (replace `LookupByKey` with `EnqueueLookupByKey`)
- Modify: `api/internal/api/v1/distributions.go:68-75` (replace the GET route with the new POST route)
- Test: `api/internal/services/distributions_test.go`

**Interfaces:**
- Produces: `func (s *DistributionService) EnqueueLookupByKey(ctx context.Context, orgPK, accessKey string) (map[string]any, error)` returning `{"status": "enqueued"}` on success.
- Consumes: `validation.ValidAccessKey` (Task 1, applied via the `dfe_access_key` struct tag), `s.checkConsQuota` (`distributions.go:350`, unchanged), `s.clients.SQS.SendMessage` (same client already used by `EnqueueSync`, `distributions.go:174`).
- Produces (SQS message contract, consumed by Task 5's `runConsAccessKey` — unchanged on the worker side): `{"job_type":"cons_ch_nfe","org_pk":...,"doc_type":"nfe","access_key":...,"trigger":"user","triggered_at":...}`.

**Context:** `LookupByKey` (distributions.go:255) is a direct synchronous SEFAZ call via py-dfe/go-dfe — the only caller is the GET route being replaced, confirmed via `rg -n "LookupByKey" api ui`. Deleting it (not leaving it as dead code) per the project's "remove orphans your change creates" rule.

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/services/distributions_test.go — append to the existing file
package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// mockSQSClient captures SendMessage calls; reuse if a similar mock already
// exists elsewhere in this package (check first) instead of duplicating.
type mockSQSClient struct {
	sentBody string
	err      error
}

func (m *mockSQSClient) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	if in.MessageBody != nil {
		m.sentBody = *in.MessageBody
	}
	return &sqs.SendMessageOutput{}, m.err
}

func TestEnqueueLookupByKey_PublishesConsChNfeJob(t *testing.T) {
	t.Parallel()
	// Wire a DistributionService with a mock SQS client and a fake org/config/
	// cert setup mirroring the fixtures already used by other tests in this
	// file (or the sibling *_test.go in this package) for EnqueueSync — do not
	// re-derive the fixture shape from scratch, copy the existing pattern.
	// Assert:
	//   1. result == map[string]any{"status": "enqueued"}
	//   2. the published SQS body unmarshals to job_type=="cons_ch_nfe",
	//      doc_type=="nfe", access_key==<the key passed in>, trigger=="user".
	//   3. checkConsQuota's counter was incremented (same assertion style
	//      EnqueueSync's own test — if any — uses for ClaimDistNSUSlot).
}

func TestEnqueueLookupByKey_QuotaExceeded(t *testing.T) {
	t.Parallel()
	// Drive the same quota counter used by checkConsQuota (distConsQuotaMax=20,
	// distributions.go:39) past the limit, then assert EnqueueLookupByKey
	// returns a 429 *problem.Problem and never calls SendMessage.
}
```

Fill in the fixture wiring by first reading how `EnqueueSync` is (or would be) tested in this same file/package — `distributions_test.go` currently only covers `validateDistDocType`/`validateSefazDistDocType`, so also check `api/tests/integration/` for an existing `DistributionService` integration harness (org/cert/config fixtures) before inventing a new one.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/services/... -run TestEnqueueLookupByKey -v`
Expected: FAIL — `EnqueueLookupByKey` undefined.

- [ ] **Step 3: Replace `LookupByKey` with `EnqueueLookupByKey`**

In `api/internal/services/distributions.go`, delete the entire `LookupByKey` method (lines 254-273) and replace it with:

```go
// EnqueueLookupByKey validates the rate limit then enqueues a background
// consChNFe call for the given access key — the async counterpart of the
// deleted synchronous LookupByKey. Mirrors EnqueueSync's shape (distributions.go:129)
// but publishes job_type "cons_ch_nfe" (worker/internal/service/distribution.go:180,
// runConsAccessKey) instead of "dist_nsu".
func (s *DistributionService) EnqueueLookupByKey(ctx context.Context, orgPK, accessKey string) (map[string]any, error) {
	if s.queueURL == "" {
		return nil, problem.BadRequest("fila de distribuição não configurada")
	}
	if err := s.checkConsQuota(ctx, orgPK, DocTypeNFe); err != nil {
		return nil, err
	}

	msg := map[string]any{
		"job_type":     "cons_ch_nfe",
		"org_pk":       orgPK,
		"doc_type":     DocTypeNFe,
		"access_key":   accessKey,
		"trigger":      "user",
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(msg)
	if _, err := s.clients.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return nil, problem.InternalServer("failed to enqueue lookup: " + err.Error())
	}
	return map[string]any{"status": "enqueued"}, nil
}
```

(`DocTypeNFe` is already defined in this package — confirm with `rg -n "DocTypeNFe\s*=" api/internal/services/*.go` before assuming the exact identifier name.)

- [ ] **Step 4: New DTO + route in `api/internal/api/v1/distributions.go`**

Add a named DTO near the top of the file (or in `dto.go`, matching wherever `CancelEventBody` lives):

```go
// LookupByKeyBody is the payload for POST /distributions/nfe/key.
type LookupByKeyBody struct {
	AccessKey string `json:"access_key" validate:"required,dfe_access_key"`
}
```

Replace the GET route (lines 68-75):

```go
	// GET /distributions/{doc_type}/key/{access_key}
	g.Get("/:doc_type/key/:access_key", perm.RequireDynamic("get.%s_distributions", "doc_type"), func(c fiber.Ctx) error {
		result, err := svc.LookupByKey(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), c.Params("access_key"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(result)
	})
```

with:

```go
	// POST /distributions/nfe/key — async consChNFe (NF-e only; see spec §E "Fora de escopo")
	g.Post("/nfe/key", perm.Require("create.nfe_distributions"), func(c fiber.Ctx) error {
		var body LookupByKeyBody
		if p := bindJSON(c, &body); p != nil {
			return sendProblem(c, p)
		}
		result, err := svc.EnqueueLookupByKey(c.Context(), middleware.GetOrgPK(c), body.AccessKey)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusAccepted).JSON(result)
	})
```

Verify `create.nfe_distributions` is an existing permission string (check `rg -n "nfe_distributions" api/internal/middleware/ api/internal/api/v1/distributions.go` — the GET route used `get.%s_distributions`, so confirm the `create.` variant is already granted to the relevant roles; if not, use whatever permission string the codebase's RBAC seed already assigns for this action instead of inventing a new one that nobody has).

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/services/... ./internal/api/v1/... -v`
Expected: PASS, and no remaining references to `LookupByKey` (`rg -n "\bLookupByKey\b" api` returns nothing).

- [ ] **Step 6: Commit**

```bash
git add api/internal/services/distributions.go api/internal/services/distributions_test.go api/internal/api/v1/distributions.go
git commit -m "feat(api): replace synchronous distribution key lookup with async enqueue"
```

---

## Task 4: Fix `ResultsConsumer.dispatch` dropping `new_distribution_nfe` messages

**Files:**
- Modify: `api/internal/consumer/results.go:119-157`
- Test: `api/internal/consumer/results_test.go` (create if absent — check first)

**Interfaces:**
- No new interfaces; this is a pure bug fix inside `dispatch`, called from `Start`'s message loop (`results.go:91`).

**Context:** `notifyResult` (`worker/internal/service/distribution.go:861-878`) publishes `{"type":"new_distribution_nfe","org_pk":...,"access_key":...,...}` — it never sets `doc_pk`. `dispatch` (`results.go:134-139`) requires `doc_pk` to contain `"#"` before it will broadcast anything, so every one of these messages is silently dropped today and the "Nova NF-e recebida" toast (`useRealtimeUpdates.ts:78-83`) has never fired. This plan's new import flows (Tasks 9-11) make this bug load-bearing — the user needs to see when their manually-triggered import lands.

- [ ] **Step 1: Write the failing test**

```go
// api/internal/consumer/results_test.go
package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
)

// fakeRegistry and a no-op cache.Backend — check whether ws.Registry and
// cache.Backend already have test doubles elsewhere in this module
// (`rg -n "Registry" api/internal/ws` / `rg -rn "cache.Backend" api --include=*_test.go`)
// before writing new ones.
type fakeRegistry struct {
	broadcasts []struct {
		orgPK   string
		payload []byte
	}
}

func (f *fakeRegistry) Broadcast(_ context.Context, orgPK string, payload []byte) {
	f.broadcasts = append(f.broadcasts, struct {
		orgPK   string
		payload []byte
	}{orgPK, payload})
}

func TestDispatch_BroadcastsWithOrgPKOnly(t *testing.T) {
	t.Parallel()
	reg := &fakeRegistry{}
	c := &ResultsConsumer{registry: reg, cache: cache.NewNoop() /* or whatever the package's no-op backend is called — check cache package */}

	event := map[string]any{
		"type":       "new_distribution_nfe",
		"org_pk":     "CNPJ_12345678000195",
		"access_key": "35250512345678000195550010000000011000000011",
	}
	body, _ := json.Marshal(event)
	c.dispatch(context.Background(), sqsMessageWithBody(body)) // helper — build a minimal sqstypes.Message

	if len(reg.broadcasts) != 1 {
		t.Fatalf("broadcasts = %d, want 1 (org_pk without doc_pk must still dispatch)", len(reg.broadcasts))
	}
	if reg.broadcasts[0].orgPK != "CNPJ_12345678000195" {
		t.Fatalf("orgPK = %q, want %q", reg.broadcasts[0].orgPK, "CNPJ_12345678000195")
	}
}
```

Write the small `sqsMessageWithBody` helper locally in the test file (`sqstypes.Message{Body: aws.String(string(body))}`), and replace `cache.NewNoop()` with whatever no-op/mock `cache.Backend` this codebase already has (`rg -rn "cache.Backend" api/internal --include=*.go` to find it) — do not add a new cache mock if one exists.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/consumer/... -run TestDispatch_BroadcastsWithOrgPKOnly -v`
Expected: FAIL — no broadcast recorded (dropped by the `doc_pk` guard).

- [ ] **Step 3: Fix `dispatch`**

In `api/internal/consumer/results.go`, replace lines 131-139:

```go
	accessKey, _ := event["access_key"].(string)
	docPK, _ := event["doc_pk"].(string)

	// doc_pk format: "{env}#{org_pk}" e.g. "prod#CNPJ_12345678000195"
	if docPK == "" || !strings.Contains(docPK, "#") {
		slog.Warn("results consumer: missing or invalid doc_pk", "doc_pk", docPK)
		return
	}
	orgPK := strings.SplitN(docPK, "#", 2)[1]
```

with:

```go
	accessKey, _ := event["access_key"].(string)
	docPK, _ := event["doc_pk"].(string)
	rawOrgPK, _ := event["org_pk"].(string)

	// Two message shapes reach this consumer: doc-result messages carry
	// doc_pk ("{env}#{org_pk}", e.g. "prod#CNPJ_12345678000195"); the
	// distribution worker's new_distribution_nfe/new_distribution_cte/
	// new_distribution_mdfe messages (worker/internal/service/distribution.go
	// notifyResult) carry org_pk directly and never set doc_pk. Accept either.
	var orgPK string
	switch {
	case docPK != "" && strings.Contains(docPK, "#"):
		orgPK = strings.SplitN(docPK, "#", 2)[1]
	case rawOrgPK != "":
		orgPK = rawOrgPK
	default:
		slog.Warn("results consumer: missing doc_pk and org_pk", "doc_pk", docPK)
		return
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd api && go test ./internal/consumer/... -v`
Expected: PASS, including any pre-existing `dispatch` tests (doc_pk path must still work unchanged).

- [ ] **Step 5: Commit**

```bash
git add api/internal/consumer/results.go api/internal/consumer/results_test.go
git commit -m "fix(api): broadcast distribution results that carry org_pk without doc_pk"
```

---

## Task 5: Worker-side duplicate quota check for `cons_ch_nfe`/`cons_nsu`

**Files:**
- Modify: `worker/internal/service/distribution.go:385-394` (wire the check into `runConsAccessKey`)
- Modify: `worker/internal/service/distribution.go` (add the new function near `claimDistNSUSlot`, ~line 884)
- Test: `worker/internal/service/distribution_test.go`

**Interfaces:**
- Produces: `func (s *DistributionService) checkConsQuota(ctx context.Context, orgPK, configTable string, cfg map[string]types.AttributeValue, envPrefix string, now time.Time) bool` — `true` = under quota, proceed; `false` = over quota, drop the job.
- Consumes: `cfg` already loaded by `runConsAccessKey`'s existing `s.loadConfig` call (no extra DynamoDB read) and `s.dynamo.UpdateItem` (same `DistributionDynamoClient` interface already used by `claimDistNSUSlot`, `distribution.go:918`).

**Context:** `api`'s `checkConsQuota` (`distributions.go:350-369`) enforces 20 calls/hour, but only at enqueue time. Two enqueued `cons_ch_nfe`/`cons_nsu` jobs (SQS at-least-once redelivery, or a user double-clicking before the first request's quota increment commits) can both reach the worker before either's quota state is visible to the other — this duplicates the existing `claimDistNSUSlot` precedent (worker re-checks a rate limit the api already checked, because the api's check can't see in-flight work). Field names (`{env}_cons_quota_window_start`, `{env}_cons_quota_calls`) must match `api/internal/repositories/fiscal_config.go:142-143` exactly — same DynamoDB item, read by both services.

- [ ] **Step 1: Write the failing tests**

```go
// worker/internal/service/distribution_test.go — append
func TestCheckConsQuota_AllowsUnderLimit(t *testing.T) {
	t.Parallel()
	dyn := &mockDistDynamo{}
	svc := NewDistribution(DistributionClients{Dynamo: dyn}, distCfg)
	cfg := configItem(2, envHom, "", "")
	now := mustParseTime(t, "2026-08-12T10:00:00Z")

	if !svc.checkConsQuota(context.Background(), testOrgPK, "dev_organization_nfe_configs", cfg, envHom, now) {
		t.Fatal("expected true (first call, well under 20/hr)")
	}
	if len(dyn.updateCalls) != 1 {
		t.Fatalf("UpdateItem calls = %d, want 1", len(dyn.updateCalls))
	}
}

func TestCheckConsQuota_BlocksOverLimit(t *testing.T) {
	t.Parallel()
	dyn := &mockDistDynamo{}
	svc := NewDistribution(DistributionClients{Dynamo: dyn}, distCfg)
	cfg := configItem(2, envHom, "", "")
	cfg[envHom+"_cons_quota_calls"] = &types.AttributeValueMemberN{Value: "20"}
	cfg[envHom+"_cons_quota_window_start"] = &types.AttributeValueMemberS{Value: "2026-08-12T09:59:00Z"}
	now := mustParseTime(t, "2026-08-12T10:00:00Z")

	// mockDistDynamo.UpdateItem must return the new (21st) count via
	// ReturnValueAllNew — extend mockDistDynamo's UpdateItem if it does not
	// already echo back ExpressionAttributeValues as Attributes; check its
	// current implementation (distribution_test.go:91-102) before assuming.
	if svc.checkConsQuota(context.Background(), testOrgPK, "dev_organization_nfe_configs", cfg, envHom, now) {
		t.Fatal("expected false (21st call within the hour, limit is 20)")
	}
}

func TestCheckConsQuota_ResetsExpiredWindow(t *testing.T) {
	t.Parallel()
	dyn := &mockDistDynamo{}
	svc := NewDistribution(DistributionClients{Dynamo: dyn}, distCfg)
	cfg := configItem(2, envHom, "", "")
	cfg[envHom+"_cons_quota_calls"] = &types.AttributeValueMemberN{Value: "20"}
	cfg[envHom+"_cons_quota_window_start"] = &types.AttributeValueMemberS{Value: "2026-08-12T08:00:00Z"} // >1h ago
	now := mustParseTime(t, "2026-08-12T10:00:00Z")

	if !svc.checkConsQuota(context.Background(), testOrgPK, "dev_organization_nfe_configs", cfg, envHom, now) {
		t.Fatal("expected true — window is stale, must reset before counting")
	}
	if len(dyn.updateCalls) != 2 { // one reset UpdateItem, one increment UpdateItem
		t.Fatalf("UpdateItem calls = %d, want 2 (reset + increment)", len(dyn.updateCalls))
	}
}
```

`mockDistDynamo.UpdateItem` (`distribution_test.go:91-102`) currently returns a bare `&dynamodb.UpdateItemOutput{}` — it needs to echo `Attributes` for `TestCheckConsQuota_BlocksOverLimit` to observe the post-increment count. Extend the mock (small, additive change) to optionally return canned `Attributes` per call, or compute them from the input's `ExpressionAttributeValues` — pick whichever is less invasive to the existing tests in this file; re-run the full `distribution_test.go` suite after touching the mock to confirm nothing else broke.

Add a tiny local helper if the file doesn't already have one:

```go
func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", s, err)
	}
	return tm
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd worker && go test ./internal/service/... -run TestCheckConsQuota -v`
Expected: FAIL — `checkConsQuota` undefined.

- [ ] **Step 3: Implement `checkConsQuota`**

Add to `worker/internal/service/distribution.go`, near `claimDistNSUSlot` (after line 935):

```go
// distWorkerConsQuotaMax mirrors api's distConsQuotaMax
// (api/internal/services/distributions.go:39) — the same 20-calls/hour limit
// re-checked here because the api's enqueue-time check can't see other
// in-flight jobs (SQS at-least-once redelivery, or two user clicks racing
// before the first request's quota increment is visible).
const distWorkerConsQuotaMax = 20

// checkConsQuota duplicates api's DistributionService.checkConsQuota
// (distributions.go:350) against the same DynamoDB fields
// ({env}_cons_quota_calls / {env}_cons_quota_window_start,
// api/internal/repositories/fiscal_config.go:142-143) so a cons_ch_nfe/
// cons_nsu job processed twice cannot bypass the hourly limit. Returns true
// when the call is allowed to proceed.
func (s *DistributionService) checkConsQuota(
	ctx context.Context,
	orgPK, configTable string,
	cfg map[string]types.AttributeValue,
	envPrefix string,
	now time.Time,
) bool {
	windowField := envPrefix + "_cons_quota_window_start"
	callsField := envPrefix + "_cons_quota_calls"

	if windowStr := dynAttrS(cfg, windowField); windowStr != "" {
		if ws, err := time.Parse(time.RFC3339, windowStr); err == nil && now.Sub(ws) >= time.Hour {
			_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
				TableName: aws.String(configTable),
				Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
				UpdateExpression: aws.String("SET " + callsField + " = :zero, " + windowField + " = :now"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":zero": &types.AttributeValueMemberN{Value: "0"},
					":now":  &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				},
			})
			if err != nil {
				slog.Warn("consQuota reset failed", "org_pk", orgPK, "err", err)
			}
		}
	}

	out, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(configTable),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
		UpdateExpression: aws.String("SET " + windowField + " = if_not_exists(" + windowField + ", :now), " +
			callsField + " = if_not_exists(" + callsField + ", :zero) + :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":  &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		slog.Warn("consQuota UpdateItem failed — allowing call", "org_pk", orgPK, "err", err)
		return true
	}
	if av, ok := out.Attributes[callsField].(*types.AttributeValueMemberN); ok {
		if n, convErr := strconv.Atoi(av.Value); convErr == nil {
			return n <= distWorkerConsQuotaMax
		}
	}
	return true
}
```

- [ ] **Step 4: Wire it into `runConsAccessKey`**

In `worker/internal/service/distribution.go`, inside `runConsAccessKey` (currently lines 385-394), add the check right after `cfg` is loaded:

```go
func (s *DistributionService) runConsAccessKey(ctx context.Context, orgPK, docType, accessKey string, dtcfg docTypeConfig) error {
	configTable := fmt.Sprintf("%s_organization_%s", s.cfg.TablePrefix, dtcfg.configTableSuffix)
	cfg, err := s.loadConfig(ctx, orgPK, configTable)
	if err != nil || cfg == nil {
		return nil
	}
	environment := attrN(cfg, "environment", 2)
	envPrefix := envHom
	if environment == 1 {
		envPrefix = envProd
	}
	if !s.checkConsQuota(ctx, orgPK, configTable, cfg, envPrefix, time.Now().UTC()) {
		slog.Warn("consChNFe quota exceeded — dropping duplicate job", "org_pk", orgPK, "access_key", accessKey)
		return nil
	}

	cert, err := s.loadCert(ctx, orgPK, dtcfg.configTableSuffix)
	...
```

Note `environment`/`envPrefix` are computed twice in the original function (once here for the quota check, again a few lines later for `sefazEnv`) — leave the existing later computation untouched rather than threading a shared variable through, since the function's existing structure already recomputes `environment` from `cfg` inline; do not refactor unrelated code while making this change (Surgical Changes rule).

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd worker && go test ./internal/service/... -v`
Expected: PASS — all three new tests plus the full existing suite (confirms the `mockDistDynamo` extension in Step 1 didn't break other tests).

- [ ] **Step 6: Commit**

```bash
git add worker/internal/service/distribution.go worker/internal/service/distribution_test.go
git commit -m "feat(worker): duplicate api's consNSU/consChNFe hourly quota check"
```

---

## Task 6: `maskAccessKey` (ui)

**Files:**
- Modify: `ui/src/lib/utils/masks.ts`
- Test: `ui/src/__tests__/lib/masks.test.ts`

**Interfaces:**
- Produces: `export function maskAccessKey(value: string): string` — groups of 4 uppercase alphanumeric characters, space-separated, capped at 44 raw characters.

- [ ] **Step 1: Write the failing tests**

```ts
// ui/src/__tests__/lib/masks.test.ts — append
import {maskAccessKey} from '@/lib/utils/masks'

describe('maskAccessKey', () => {
  it('agrupa em blocos de 4 caracteres', () => {
    expect(maskAccessKey('35250512345678000195550010000000011000000011'))
      .toBe('3525 0512 3456 7800 0195 5500 1000 0000 0110 0000 0011')
  })

  it('formata parcialmente durante digitação', () => {
    expect(maskAccessKey('352505')).toBe('3525 05')
  })

  it('aceita CNPJ alfanumérico em maiúsculas, ignora minúsculas convertendo', () => {
    expect(maskAccessKey('3525051234ab5678000195550010000000011000000011'))
      .toBe('3525 0512 34AB 5678 0001 9555 0010 0000 0001 1000 0000 11')
  })

  it('ignora caracteres não alfanuméricos e limita a 44', () => {
    expect(maskAccessKey('3525-0512.3456/7800 0195550010000000011000000011XXXX'))
      .toBe('3525 0512 3456 7800 0195 5500 1000 0000 0110 0000 0011')
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ui && npx vitest run src/__tests__/lib/masks.test.ts`
Expected: FAIL — `maskAccessKey` is not exported.

- [ ] **Step 3: Implement**

Append to `ui/src/lib/utils/masks.ts`:

```ts
/**
 * Access-key mask — 44 alphanumeric characters (digits + uppercase CNPJ
 * letters, IN RFB 2229/2024) grouped in blocks of 4, space-separated.
 */
export function maskAccessKey(value: string): string {
  const clean = value.replace(/[^A-Z0-9]/gi, '').toUpperCase().slice(0, 44)
  return clean.match(/.{1,4}/g)?.join(' ') ?? clean
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/__tests__/lib/masks.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/utils/masks.ts ui/src/__tests__/lib/masks.test.ts
git commit -m "feat(ui): add maskAccessKey"
```

---

## Task 7: Access-key structural validator (TypeScript, mirrors Task 1)

**Files:**
- Create: `ui/src/lib/utils/access-key.ts`
- Test: `ui/src/__tests__/lib/access-key.test.ts`

**Interfaces:**
- Produces: `export type AccessKeyField = 'length' | 'cUF' | 'AAMM' | 'doc' | 'mod' | 'tpEmis' | 'cDV'` and `export function validateAccessKey(key: string): {valid: boolean; error?: AccessKeyField}`.
- Consumes: `validateCPF`, `validateCNPJ` (`ui/src/lib/utils/validators.ts:1,22` — already exist, do not reimplement).

**Context:** Must reject exactly the same inputs as Task 1's Go `ValidAccessKey`, using the same two check-digit algorithms (CNPJ's own, from `validateCNPJ`; the access key's own `cDV`, computed here — do not conflate them, see the comment in Task 1's `calcAccessKeyDV`).

- [ ] **Step 1: Write the failing tests**

```ts
// ui/src/__tests__/lib/access-key.test.ts
import {validateAccessKey} from '@/lib/utils/access-key'

const VALID = '35250512345678000195550010000000011000000011'

describe('validateAccessKey', () => {
  it('aceita uma chave numérica válida', () => {
    expect(validateAccessKey(VALID)).toEqual({valid: true})
  })

  it('rejeita tamanho incorreto', () => {
    expect(validateAccessKey(VALID.slice(0, 43))).toEqual({valid: false, error: 'length'})
  })

  it('rejeita cUF inexistente', () => {
    expect(validateAccessKey('99' + VALID.slice(2))).toEqual({valid: false, error: 'cUF'})
  })

  it('rejeita mês 13 em AAMM', () => {
    expect(validateAccessKey(VALID.slice(0, 2) + '2513' + VALID.slice(6))).toEqual({valid: false, error: 'AAMM'})
  })

  it('rejeita mod diferente de 55 (NFC-e fora de escopo)', () => {
    expect(validateAccessKey(VALID.slice(0, 20) + '65' + VALID.slice(22))).toEqual({valid: false, error: 'mod'})
  })

  it('rejeita tpEmis=9 (exclusivo de NFC-e)', () => {
    expect(validateAccessKey(VALID.slice(0, 34) + '9' + VALID.slice(35))).toEqual({valid: false, error: 'tpEmis'})
  })

  it('rejeita cDV incorreto', () => {
    const lastDigit = VALID[43]
    const bad = lastDigit === '0' ? '1' : '0'
    expect(validateAccessKey(VALID.slice(0, 43) + bad)).toEqual({valid: false, error: 'cDV'})
  })

  it('aceita CPF com prefixo 000 e rejeita DV de CPF inválido', () => {
    const base = VALID.slice(0, 6) + '00052998224725' + VALID.slice(20, 43)
    // Recompute cDV the same way production code does — import is fine in a test.
    const ok = base + computeExpectedDV(base)
    expect(validateAccessKey(ok)).toEqual({valid: true})

    const badCpfBase = VALID.slice(0, 6) + '00052998224724' + VALID.slice(20, 43)
    const badCpf = badCpfBase + computeExpectedDV(badCpfBase)
    expect(validateAccessKey(badCpf)).toEqual({valid: false, error: 'doc'})
  })
})

// Test-local reimplementation of the cDV algorithm, kept intentionally
// separate from the production calcAccessKeyDV so this test doesn't silently
// pass if both were wrong in the same way — cross-checked against the
// Go test's calcAccessKeyDV output for the same inputs (Task 1).
function computeExpectedDV(key43: string): string {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9]
  let sum = 0
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8]
  }
  const rem = sum % 11
  return String(rem < 2 ? 0 : 11 - rem)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ui && npx vitest run src/__tests__/lib/access-key.test.ts`
Expected: FAIL — module does not exist.

- [ ] **Step 3: Implement**

```ts
// ui/src/lib/utils/access-key.ts
import {validateCPF, validateCNPJ} from '@/lib/utils/validators'

const ACCESS_KEY_LEN = 44
const ACCESS_KEY_MOD_NFE = '55' // this feature is NF-e-only (mod 65 = NFC-e, out of scope)
const ACCESS_KEY_VALID_TP_EMIS = new Set(['1', '2', '3', '4', '5', '6', '7'])

// IBGE cUF codes — mirrors api/internal/services/shared.go UFCode values.
// Keep both lists in lock-step; there is no shared source to import across
// the Go/TypeScript boundary.
const IBGE_UF_CODES = new Set([
  '12', '27', '13', '16', '29', '23', '53', '32', '52', '21', '31', '50',
  '51', '15', '25', '26', '22', '41', '33', '24', '11', '14', '43', '42',
  '28', '35', '17',
])

export type AccessKeyField = 'length' | 'cUF' | 'AAMM' | 'doc' | 'mod' | 'tpEmis' | 'cDV'

export interface AccessKeyValidation {
  valid: boolean
  error?: AccessKeyField
}

/**
 * NT 2023.002 access-key check digit (cDV): weights 2-9 cycling
 * right-to-left, mod-11. Character value = ASCII code − 48 — a DIFFERENT
 * algorithm from the CNPJ field's own internal check digits (validateCNPJ,
 * ui/src/lib/utils/validators.ts:22, which uses A=10..Z=35). Mirrors
 * api/internal/validation/access_key.go calcAccessKeyDV — keep in lock-step.
 */
function calcAccessKeyDV(key43: string): number {
  const weights = [2, 3, 4, 5, 6, 7, 8, 9]
  let sum = 0
  for (let i = key43.length - 1, wi = 0; i >= 0; i--, wi++) {
    sum += (key43.charCodeAt(i) - 48) * weights[wi % 8]
  }
  const rem = sum % 11
  return rem < 2 ? 0 : 11 - rem
}

/**
 * Validates an NF-e access key beyond its 44-character length: cUF, AAMM,
 * CNPJ-xor-CPF (with check digit), mod=55, tpEmis, and the final cDV check
 * digit. Mirrors api/internal/validation/access_key.go ValidAccessKey — keep
 * both in lock-step (docs/specs/2026-08-12-manifestacao-importacao-nfe.md §E).
 */
export function validateAccessKey(key: string): AccessKeyValidation {
  const digitsOutsideDoc = key.length === ACCESS_KEY_LEN && /^\d{6}$/.test(key.slice(0, 6)) && /^\d{24}$/.test(key.slice(20))
  if (!digitsOutsideDoc) {
    return {valid: false, error: 'length'}
  }

  const cUF = key.slice(0, 2)
  const mm = parseInt(key.slice(4, 6), 10)
  const doc = key.slice(6, 20)
  const mod = key.slice(20, 22)
  const tpEmis = key[34]
  const cDV = key[43]

  if (!IBGE_UF_CODES.has(cUF)) return {valid: false, error: 'cUF'}
  if (mm < 1 || mm > 12) return {valid: false, error: 'AAMM'}

  const validDoc = doc.startsWith('000') ? validateCPF(doc.slice(3)) : validateCNPJ(doc)
  if (!validDoc) return {valid: false, error: 'doc'}

  if (mod !== ACCESS_KEY_MOD_NFE) return {valid: false, error: 'mod'}
  if (!ACCESS_KEY_VALID_TP_EMIS.has(tpEmis)) return {valid: false, error: 'tpEmis'}
  if (parseInt(cDV, 10) !== calcAccessKeyDV(key.slice(0, 43))) return {valid: false, error: 'cDV'}

  return {valid: true}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/__tests__/lib/access-key.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/utils/access-key.ts ui/src/__tests__/lib/access-key.test.ts
git commit -m "feat(ui): add validateAccessKey"
```

---

## Task 8: `ApiClient` — replace `lookupDistributionByKey` with `importNfeByKey`, add `sendManifestation` reuse note

**Files:**
- Modify: `ui/src/lib/api/client.ts:820-822`
- Test: `ui/src/lib/api/__tests__/client.test.ts` (or wherever existing `ApiClient` method tests live — check `ls ui/src/lib/api/__tests__/`)

**Interfaces:**
- Produces: `async importNfeByKey(accessKey: string): Promise<SyncEnqueuedOut>` — `POST /v1.0/distributions/nfe/key`.
- Removes: `lookupDistributionByKey` (confirmed zero callers via `rg -rn "lookupDistributionByKey" ui/src` — only the definition itself).
- **No change needed** to `sendManifestation` (`client.ts:595-601`) — it already exists, already calls `POST /v1.0/nfes/:access_key/manifestation` with the exact shape Task 2's route expects. Task 9 calls it directly.

- [ ] **Step 1: Write the failing test**

```ts
// add to whichever client test file already covers distributions methods (syncDistributions, listDistributions)
it('importNfeByKey posts to /v1.0/distributions/nfe/key', async () => {
  const postSpy = vi.spyOn(apiClient['http'], 'post').mockResolvedValue({data: {status: 'enqueued'}})
  const result = await apiClient.importNfeByKey('35250512345678000195550010000000011000000011')
  expect(postSpy).toHaveBeenCalledWith('/v1.0/distributions/nfe/key', {access_key: '35250512345678000195550010000000011000000011'})
  expect(result).toEqual({status: 'enqueued'})
})
```

Match whatever spy/mock convention the existing `syncDistributions`/`listDistributions` tests already use in this file — do not introduce a new mocking style.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run <the client test file>`
Expected: FAIL — `importNfeByKey` does not exist.

- [ ] **Step 3: Implement**

In `ui/src/lib/api/client.ts`, replace lines 820-822:

```ts
  async lookupDistributionByKey(docType: string, accessKey: string): Promise<DistributionLookupOut> {
    return this.get(`/v1.0/distributions/${docType}/key/${accessKey}`)
  }
```

with:

```ts
  /** Enqueues an async consChNFe for the given NF-e access key (202). NF-e only — see DOCS.md. */
  async importNfeByKey(accessKey: string): Promise<SyncEnqueuedOut> {
    return this.post('/v1.0/distributions/nfe/key', {access_key: accessKey})
  }
```

Check whether `DistributionLookupOut` is still referenced elsewhere (`lookupDistributionByNsu` uses it, `client.ts:816-818`) — it is, so keep the type definition; only the `lookupDistributionByKey` method is removed.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run <the client test file>`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/api/client.ts <the client test file>
git commit -m "feat(ui): replace lookupDistributionByKey with async importNfeByKey"
```

---

## Task 9: Manifestation modal + import-via-distribution button (NF-e detail page)

**Files:**
- Modify: `ui/src/app/nfe/detail/page.tsx`
- Test: `ui/src/__tests__/app/nfe-detail.test.tsx` (create if no equivalent exists — check `ui/src/__tests__/app/` first for a sibling NF-e detail test to extend instead)

**Interfaces:**
- Consumes: `apiClient.sendManifestation` (existing, `client.ts:595`), `apiClient.importNfeByKey` (Task 8), `EVENT_TYPE_LABELS` (`ui/src/lib/data/dfe_event.ts:19-22`), `queryKeys.nfes.events`/`detail`/`lists` (existing), `OptionsSelect`, `Modal`, `JustificationField` (existing components — same ones `NfeDetail`'s CC-e modal already uses).
- Produces: no new exports — this is page-local UI wiring inside the existing `NfeDetail` function component (`ui/src/app/nfe/detail/page.tsx:19-115`), added via the same `headerActions`/`renderExtra` props `DfeDetail` already exposes (do not modify `ui/src/components/dfe/DfeDetail.tsx` itself — it is shared by NF-e/NFC-e/MDF-e and these two buttons are NF-e-destined-only).

**Context:** `doc.incoming === 1` means the note was received (destined to this org), which is the only case these buttons apply — mirrors the existing `isOwnEmission = doc.incoming === 0` check pattern in `DfeDetail.tsx:120`. "Complete" means `doc.products` is non-empty (`DfeDetail.tsx:225`'s own definition of a summary-only vs. full record) — the "Importar via distribuição" button is enabled only when that's false. The manifestation type select must hide any event type that already has a `status === 'success'` event on this note (`EVENT_STATUS_SUCCESS`, `ui/src/lib/utils/dfe-result-toast.ts:15`) — fetched via the same `queryKeys.nfes.events(accessKey)` query `DfeDetail` uses internally (React Query dedupes the network call by key, so this second `useQuery` here costs nothing extra).

- [ ] **Step 1: Write the failing test**

```tsx
// ui/src/__tests__/app/nfe-detail-manifestation.test.tsx
// Follow the render/mocking setup of whichever existing test already covers
// NfeDetail's CC-e flow (search `rg -rn "Carta de Correção" ui/src/__tests__`)
// — reuse its apiClient/useAuth mocks rather than rebuilding them.
import {describe, it, expect, vi} from 'vitest'
import {render, screen, fireEvent, waitFor} from '@testing-library/react'
// ... same imports/mocks as the CC-e test, plus:
import {apiClient} from '@/lib/api/client'

describe('NfeDetail — manifestation', () => {
  it('mostra o botão Manifestar apenas para NF-e destinada (incoming=1)', async () => {
    // render with a mocked getNfe() resolving {incoming: 1, products: [], ...}
    // assert screen.getByText('Manifestar') exists
    // re-render with incoming: 0 and assert it does not
  })

  it('oculta tipos de evento já autorizados no select', async () => {
    // mock getNfeEvents() to return an item with event_type '210210', status 'success'
    // open the modal, assert the '210210' option is absent from the select
  })

  it('exige justificativa apenas para Operação não realizada (210240)', async () => {
    // select '210240' in the modal, assert submit is disabled until justification >= 15 chars
    // select '210210', assert submit is enabled with no justification
  })

  it('envia a manifestação e invalida as queries relevantes', async () => {
    const sendSpy = vi.spyOn(apiClient, 'sendManifestation').mockResolvedValue({} as never)
    // open modal, pick 210210, submit
    await waitFor(() => expect(sendSpy).toHaveBeenCalledWith(expect.any(String), '210210', 1, undefined))
  })

  it('botão Importar via distribuição fica desabilitado quando a nota já está completa', async () => {
    // mock getNfe() with non-empty products[]; assert the button has the disabled attribute
  })

  it('botão Importar via distribuição chama importNfeByKey quando a nota é só resumo', async () => {
    const importSpy = vi.spyOn(apiClient, 'importNfeByKey').mockResolvedValue({status: 'enqueued'})
    // mock getNfe() with products: null
    // click the button
    await waitFor(() => expect(importSpy).toHaveBeenCalled())
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ui && npx vitest run src/__tests__/app/nfe-detail-manifestation.test.tsx`
Expected: FAIL — buttons/modal do not exist yet.

- [ ] **Step 3: Implement**

In `ui/src/app/nfe/detail/page.tsx`, add imports:

```ts
import {useQuery, useMutation, useQueryClient} from '@tanstack/react-query'
import {OptionsSelect} from '@/components/ui/options-select'
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event'
import {EVENT_STATUS_SUCCESS} from '@/lib/utils/dfe-result-toast'
import {toast} from 'sonner'
import {ApiError} from '@/lib/api/client'
```

(`useQuery`/`useMutation`/`useQueryClient` are already partly imported — merge with the existing `import {useMutation, useQueryClient} from '@tanstack/react-query'` line rather than duplicating it.)

Add constants near the top of the file, outside the component:

```ts
const MANIFEST_EVENT_TYPES = ['210210', '210200', '210220', '210240'] as const
const MANIFEST_JUSTIFICATION_REQUIRED_TYPE = '210240'
const MANIFEST_JUSTIFICATION_MIN_LENGTH = 15
```

Inside `NfeDetail` (after the existing CC-e state, ~line 24), add:

```ts
  const [showManifestModal, setShowManifestModal] = useState(false)
  const [manifestEventType, setManifestEventType] = useState<string>('210210')
  const [manifestJustification, setManifestJustification] = useState('')

  const {data: eventsData} = useQuery({
    queryKey: queryKeys.nfes.events(accessKey),
    queryFn: () => apiClient.getNfeEvents(accessKey),
    enabled: !!accessKey && !!selectedOrg,
  })
  const authorizedEventTypes = new Set(
    (eventsData?.items ?? []).filter((e) => e.status === EVENT_STATUS_SUCCESS).map((e) => e.event_type)
  )
  const availableManifestTypes = MANIFEST_EVENT_TYPES.filter((t) => !authorizedEventTypes.has(t))

  const manifestMutation = useMutation({
    mutationFn: () =>
      apiClient.sendManifestation(
        accessKey,
        manifestEventType,
        1,
        manifestEventType === MANIFEST_JUSTIFICATION_REQUIRED_TYPE ? manifestJustification.trim() : undefined,
      ),
    onSuccess: () => {
      setShowManifestModal(false)
      setManifestJustification('')
      toast.info('Manifestação enfileirada.')
      void qc.invalidateQueries({queryKey: queryKeys.nfes.detail(accessKey)})
      void qc.invalidateQueries({queryKey: queryKeys.nfes.events(accessKey)})
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao enviar manifestação.')
    },
  })

  const importMutation = useMutation({
    mutationFn: () => apiClient.importNfeByKey(accessKey),
    onSuccess: () => {
      toast.info('Importação enfileirada. A NF-e completa aparecerá automaticamente.')
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao importar NF-e.')
    },
  })

  const manifestJustificationOk =
    manifestEventType !== MANIFEST_JUSTIFICATION_REQUIRED_TYPE ||
    manifestJustification.trim().length >= MANIFEST_JUSTIFICATION_MIN_LENGTH
```

Extend `headerActions` (currently only the CC-e button, lines 52-64) to a fragment with three conditions:

```tsx
      headerActions={(doc) => (
        <>
          {doc.status === 'authorized' && doc.incoming === 0 && (
            <Button variant="outline" size="sm"
                    onClick={() => { setCceText(''); setCceSeq(1); setShowCceModal(true) }}
                    className="text-amber-600 border-amber-200 hover:bg-amber-50">
              Carta de Correção
            </Button>
          )}
          {doc.incoming === 1 && (
            <Button variant="outline" size="sm"
                    onClick={() => { setManifestEventType(availableManifestTypes[0] ?? '210210'); setManifestJustification(''); setShowManifestModal(true) }}
                    disabled={availableManifestTypes.length === 0}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              Manifestar
            </Button>
          )}
          {doc.incoming === 1 && (
            <Button variant="outline" size="sm"
                    onClick={() => importMutation.mutate()}
                    disabled={!!doc.products?.length || importMutation.isPending}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              {importMutation.isPending ? 'Importando…' : 'Importar via distribuição'}
            </Button>
          )}
        </>
      )}
```

Add the manifestation modal inside `renderExtra`, alongside the existing CC-e `Modal` (do not remove the CC-e one — wrap both in a fragment):

```tsx
      renderExtra={(doc) => (
        <>
          <Modal /* ... existing CC-e modal, unchanged ... */>
            {/* unchanged content */}
          </Modal>

          <Modal
            isOpen={showManifestModal}
            title={`Manifestação — NF-e nº ${doc.number}`}
            onClose={() => setShowManifestModal(false)}
            onSubmit={() => manifestMutation.mutate()}
            submitLabel="Manifestar"
            cancelLabel="Voltar"
            loading={manifestMutation.isPending}
            submitDisabled={!manifestJustificationOk}
          >
            <div className="space-y-4">
              <div>
                <label htmlFor="manifest-type" className="block text-sm font-medium text-gray-700 mb-1.5">
                  Tipo de manifestação
                </label>
                <OptionsSelect
                  id="manifest-type"
                  value={manifestEventType}
                  onValueChange={setManifestEventType}
                  options={availableManifestTypes.map((t) => ({value: t, label: EVENT_TYPE_LABELS[t] ?? t}))}
                />
              </div>
              {manifestEventType === MANIFEST_JUSTIFICATION_REQUIRED_TYPE && (
                <JustificationField
                  id="manifest-justification"
                  value={manifestJustification}
                  onChange={setManifestJustification}
                  minLength={MANIFEST_JUSTIFICATION_MIN_LENGTH}
                  placeholder="Descreva por que a operação não foi realizada (mínimo 15 caracteres)…"
                />
              )}
            </div>
          </Modal>
        </>
      )}
```

`JustificationField` needs importing too (`import {JustificationField} from '@/components/ui/justification-field'` — likely already imported for CC-e; confirm before duplicating).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/__tests__/app/nfe-detail-manifestation.test.tsx`
Expected: PASS.

- [ ] **Step 5: Run ESLint**

Run: `cd ui && npx eslint src/app/nfe/detail/page.tsx --ext .ts,.tsx`
Expected: zero errors/warnings.

- [ ] **Step 6: Commit**

```bash
git add ui/src/app/nfe/detail/page.tsx ui/src/__tests__/app/nfe-detail-manifestation.test.tsx
git commit -m "feat(ui): add manual manifestation and import-via-distribution to NF-e detail"
```

---

## Task 10: "Importar NF-e" button + modal (Distribuição tab)

**Files:**
- Modify: `ui/src/app/nfe/page.tsx` (`NfeDistributionTab`, lines 158-248)
- Test: `ui/src/__tests__/app/nfe-distribution-import.test.tsx`

**Interfaces:**
- Consumes: `apiClient.importNfeByKey` (Task 8), `maskAccessKey` (Task 6), `validateAccessKey` (Task 7), `Modal` (existing).

**Context:** Sits next to the existing "Consultar SEFAZ" button (`nfe/page.tsx:204-212`), same visual treatment. This is the manual-entry path — unlike Task 9's button (which reuses the note's own known-good access key), this one must run the full field-level validator on user-typed input before the request is sent, and show the specific failing field, not just "chave inválida" (spec §E).

- [ ] **Step 1: Write the failing test**

```tsx
// ui/src/__tests__/app/nfe-distribution-import.test.tsx
describe('NfeDistributionTab — Importar NF-e', () => {
  it('desabilita o envio enquanto a chave é inválida', () => {
    // render, open modal, type an incomplete key, assert submit disabled
  })

  it('mostra o campo específico que falhou', () => {
    // type a key with cUF=99, assert the cUF-specific error message renders
  })

  it('chama importNfeByKey com a chave desformatada ao submeter uma chave válida', async () => {
    const importSpy = vi.spyOn(apiClient, 'importNfeByKey').mockResolvedValue({status: 'enqueued'})
    // type the masked valid key (with spaces), submit
    await waitFor(() => expect(importSpy).toHaveBeenCalledWith('35250512345678000195550010000000011000000011'))
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/__tests__/app/nfe-distribution-import.test.tsx`
Expected: FAIL.

- [ ] **Step 3: Implement**

In `ui/src/app/nfe/page.tsx`, add imports:

```ts
import {maskAccessKey} from '@/lib/utils/masks'
import {validateAccessKey, type AccessKeyField} from '@/lib/utils/access-key'
import {Modal} from '@/components/ui/modal'
```

Add a small field-label map near the top of the file (module scope):

```ts
const ACCESS_KEY_FIELD_LABELS: Record<AccessKeyField, string> = {
  length: 'A chave deve ter 44 caracteres',
  cUF: 'Código da UF (cUF) inválido',
  AAMM: 'Ano/mês de emissão inválido',
  doc: 'CNPJ/CPF do emitente inválido (dígito verificador incorreto)',
  mod: 'Modelo do documento inválido (esperado 55 — NF-e)',
  tpEmis: 'Tipo de emissão inválido',
  cDV: 'Dígito verificador da chave inválido',
}
```

Inside `NfeDistributionTab` (after the existing `syncMutation`, ~line 182), add:

```ts
  const [showImportModal, setShowImportModal] = useState(false)
  const [importKeyInput, setImportKeyInput] = useState('')
  const qc = useQueryClient()

  const cleanImportKey = importKeyInput.replace(/[^A-Z0-9]/gi, '').toUpperCase()
  const importValidation = cleanImportKey.length === 44 ? validateAccessKey(cleanImportKey) : {valid: false as const}

  const importMutation = useMutation({
    mutationFn: () => apiClient.importNfeByKey(cleanImportKey),
    onSuccess: () => {
      setShowImportModal(false)
      setImportKeyInput('')
      toast.info('Importação enfileirada. A NF-e aparecerá automaticamente quando processada.')
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao importar NF-e.')
    },
  })
```

`useQueryClient`/`ApiError` may need adding to this file's imports — check the existing import block first (`useMutation` and `toast` are already imported per the file read earlier; `useQueryClient` and `ApiError` are not yet).

Add the button next to "Consultar SEFAZ" (inside the `<div className="flex items-center gap-2">`-style button group at `nfe/page.tsx:204-212` — check the exact wrapper element before inserting, since the file uses a flex row there):

```tsx
        <Button
          variant="outline"
          size="sm"
          onClick={() => { setImportKeyInput(''); setShowImportModal(true) }}
          className="text-brand-600 border-brand-200 hover:bg-brand-50"
        >
          Importar NF-e
        </Button>
```

Add the modal (as a sibling of the existing table, inside the component's returned JSX):

```tsx
      <Modal
        isOpen={showImportModal}
        title="Importar NF-e por chave de acesso"
        onClose={() => setShowImportModal(false)}
        onSubmit={() => importMutation.mutate()}
        submitLabel="Importar"
        cancelLabel="Cancelar"
        loading={importMutation.isPending}
        submitDisabled={!importValidation.valid}
      >
        <div className="space-y-2">
          <label htmlFor="import-access-key" className="block text-sm font-medium text-gray-700">
            Chave de acesso
          </label>
          <input
            id="import-access-key"
            value={maskAccessKey(importKeyInput)}
            onChange={(e) => setImportKeyInput(e.target.value)}
            placeholder="0000 0000 0000 0000 0000 0000 0000 0000 0000 0000 0000"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-400"
          />
          {cleanImportKey.length > 0 && !importValidation.valid && 'error' in importValidation && (
            <p className="text-xs text-red-600">{ACCESS_KEY_FIELD_LABELS[importValidation.error!]}</p>
          )}
        </div>
      </Modal>
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/__tests__/app/nfe-distribution-import.test.tsx`
Expected: PASS.

- [ ] **Step 5: Run ESLint**

Run: `cd ui && npx eslint src/app/nfe/page.tsx --ext .ts,.tsx`
Expected: zero errors/warnings.

- [ ] **Step 6: Commit**

```bash
git add ui/src/app/nfe/page.tsx ui/src/__tests__/app/nfe-distribution-import.test.tsx
git commit -m "feat(ui): add manual NF-e import by access key to the Distribuição tab"
```

---

## Task 11: Manual verification (mobile + desktop, both flows)

No new files — this is a manual pass, per `ui/CLAUDE.md`'s "start the dev server and use the feature in a browser before reporting complete" rule.

- [ ] Start `cd ui && npm run dev`, log in against an org with at least one destined NF-e that is summary-only (`products` empty) and one that is complete.
- [ ] On the NF-e detail page for the summary-only note: confirm "Manifestar" and "Importar via distribuição" both render, "Importar via distribuição" is enabled; click it, confirm a toast fires and the button shows "Importando…" while pending.
- [ ] On the complete note: confirm "Importar via distribuição" is disabled (or renders but does nothing destructive if clicked — per the `submitDisabled` wiring, it should be inert).
- [ ] Open the manifestation modal: confirm the select excludes any event type already `success` for that note; select "Operação não realizada" and confirm the submit button stays disabled until 15+ characters are typed in the justification field; submit a "Ciência" manifestation and confirm the toast/query-invalidation flow (existing `dfe_result` WebSocket path) reflects the outcome.
- [ ] On the Distribuição tab, click "Importar NF-e", type a deliberately wrong access key (e.g. flip the last digit) and confirm the specific field error renders (not a generic message); type a real valid key and confirm the modal submits, closes, and a toast confirms enqueueing.
- [ ] Trigger the SEFAZ-side result for one of the above (or wait for the worker to process it in a dev/staging environment) and confirm the "Nova NF-e recebida" toast now fires (Task 4's fix) and the Distribuição table refreshes without a manual reload.
- [ ] Repeat the NF-e detail screen checks at a 375px viewport (Chrome DevTools → iPhone SE): confirm the new buttons wrap onto their own row rather than overflowing, and both modals are `w-full sm:max-w-lg` (inherited from the shared `Modal` component — no page-specific CSS needed, but verify visually).

---

## Task 12: Documentation updates

**Files:**
- Modify: `DOCS.md:1126` (NF-e endpoint table — add the manifestation row, currently undocumented despite existing since before this feature)
- Modify: `DOCS.md:1350` (DFe Distribution endpoint table — replace the GET key-lookup row)
- Modify: `CONDUCT.md` (new constraint note — see below)

**Context:** Per the project's Mandatory Documentation Policy, every behavior/API change in this plan needs a paired doc update. Two of the three edits below cover this plan's own new endpoints; the manifestation row is a pre-existing gap this plan happens to be touching that endpoint anyway, so document it now rather than leave the omission for someone else to trip over.

- [ ] **Step 1:** In `DOCS.md`, add a row to the NF-e table (after line 1123, `POST /v1.0/nfes/{access_key}/cancel`):

```markdown
| POST   | `/v1.0/nfes/{access_key}/manifestation`   | Manifestação do destinatário (210200/210210/210220/210240) |
```

- [ ] **Step 2:** In `DOCS.md`, replace the `GET /v1.0/distributions/{doc_type}/key/{key}` row (line 1350) with:

```markdown
| POST   | `/v1.0/distributions/nfe/key`               | Enqueue consChNFe by access key (202, NF-e only, consumes 20/hr quota). |
```

Add a short note directly below the table (after the existing `doc_type ∈ {nfe, cte, mdfe}` line, ~1354):

```markdown
`POST /distributions/nfe/key` is NF-e-only (no `doc_type` path param) — CT-e/MDF-e have no
resNFe/Ciência-do-destinatário concept that motivates a manual re-consult. Body: `{"access_key": "..."}`,
validated structurally at the API boundary (`api/internal/validation.ValidAccessKey`) before enqueueing —
see `docs/specs/2026-08-12-manifestacao-importacao-nfe.md`.
```

- [ ] **Step 3:** In `CONDUCT.md`, add an entry documenting the `ResultsConsumer.dispatch` fix (search the file for its "known workarounds/gotchas" section heading and match its existing entry format — do not invent a new section):

```markdown
- `ResultsConsumer.dispatch` (api/internal/consumer/results.go) accepts a bare `org_pk` when
  `doc_pk` is absent — the distribution worker's `new_distribution_*` messages
  (`worker/internal/service/distribution.go` `notifyResult`) never set `doc_pk`, only `org_pk`.
  Before 2026-08, this silently dropped every one of those messages and the "Nova NF-e recebida"
  toast never fired. Any future message type reaching this consumer must carry at least one of
  the two — `dispatch` treats both as valid client identifiers.
```

- [ ] **Step 4: Commit**

```bash
git add DOCS.md CONDUCT.md
git commit -m "docs: document manifestation endpoint, async NF-e key import, and org_pk dispatch fix"
```

---

## Self-Review Notes

- **Spec coverage:** §Backend-api items 1-3 → Tasks 1-4. §Backend-worker → Task 5. §Frontend A → Task 9 (manifestation half). §Frontend B → Task 9 (import-via-distribution half). §Frontend C → Task 10. §Frontend D (`maskAccessKey`) → Task 6. §Frontend E (validation table) → Task 7. §Fora de escopo items are respected: no CDK changes anywhere in this plan, `POST /distributions/nfe/key` has no `doc_type` param (NF-e hardcoded), CT-e/MDF-e untouched. §Testes: every Go/TS test the spec calls for maps to a task's Step 1.
- **Type consistency:** `EnqueueLookupByKey` (Task 3) returns `map[string]any{"status": "enqueued"}`, matching `SyncEnqueuedOut`'s existing shape (`{status: string}` — reused as-is in Task 8's `importNfeByKey` return type, no new TS type needed). `validateAccessKey`'s `AccessKeyField` union (Task 7) is exactly the field set `ACCESS_KEY_FIELD_LABELS` (Task 10) switches on — verified no field name drift between the two.
- **Known follow-ups intentionally left as TODO markers in test code** (Task 3's fixture wiring, Task 4's mock lookup, Task 9's test-harness reuse): these point the implementer at *where* to find the existing pattern to copy, rather than duplicating a guess at that pattern here — the actual test harness code in this repo needs to be read at implementation time, not re-derived from this plan's summary of it.
