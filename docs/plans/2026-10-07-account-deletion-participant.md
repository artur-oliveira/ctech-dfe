# Account Deletion (LGPD) — DF-e Participant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ctech-dfe a participant of the ctech-account erasure saga: answer eligibility, refuse writes of a locked user, purge the person (memberships, invitations, audit actor, e-CPF certificates, user rows) and every company of an organization erased with its only member, then ack.

**Architecture:**
- The API (EC2, long-running) runs `ctech-go-common/erasure.Consumer` in a background goroutine, like the existing results consumer. Its `PurgeFunc` is `services.ErasureService.Purge`.
- The purge is table-driven: one catalog (`services/erasure_catalog.go`) lists every DynamoDB table of `cdk/lib/dynamodb-stack.ts` with how its rows are reached from a company id. A test fails when a CDK table is missing from the catalog.
- Lock: the auth middleware (one choke point for every user route) refuses mutating requests of a locked/erased `sub`; JWT revocation (`jwtverify.WithRevocation`) is enabled in the same constructor.
- Workers (Lambda) drop jobs whose company is locked or erased (`ORG#{company_id}` in `{prefix}_erasure_state`).

**Tech Stack:** Go 1.27, Fiber v3, fx, aws-sdk-go-v2 (DynamoDB, S3, SQS), `gopkg.aoctech.app/api-commons` v1.13.1 (`erasure`, `jwtverify`, `oauth2client`), AWS CDK (TypeScript, jest).

**Spec:** ctech-account `docs/specs/2026-10-06-account-deletion-overview.md` (D1–D14), `…-data-inventory.md` §2 and §5, `…-saga-protocol.md` §3–§9; wire contract in ctech-account `docs/plans/2026-10-07-account-deletion-phase3-participants.md` (Global Constraints, Task 5).

## Global Constraints

- Branch per implementation: `feat/account-deletion-participant` from `main`. Conventional Commits. **No `Co-Authored-By` or any Claude/Anthropic attribution** in commits or PRs.
- Service id in messages, acks and the SNS filter: `dfe` (`services.ErasureServiceName`).
- Eligibility: `GET /v1.0/internal/erasure/eligibility/{sub}`, token minted by ctech-account with `aud` = this service's `SERVICE_AUDIENCE`, no `sid`, scope `internal:dfe:erasure-eligibility`. Response `erasure.Eligibility`. ctech-account's `ERASURE_PARTICIPANTS` entry for dfe uses `url` = `{dfe internal base URL}/v1.0`.
- Ack: `POST {CTECH_URL}/v1.0/internal/erasure/ack`, client-credentials token with scope `internal:account:erasure-ack`.
- Subject: `GET {CTECH_URL}/v1.0/internal/erasure/{request_id}/subject` with scope `internal:account:erasure-subject`, returns `{"cpf": "..."}` only while purging. **The CPF lives in memory only: never logged, never persisted, never put in an error message.**
- SNS topic ARN from SSM `/ctech/{env}/account/erasure-topic-arn`. Queue subscribed with raw delivery and `FilterPolicy {"services":["dfe"]}` at the default (message attributes) scope.
- `{prefix}_erasure_state`: partition key `pk` (S), TTL attribute `ttl`, schema owned by `erasure.Store`. Tombstones kept forever (`erasedTTL = 0`).
- Revocation entries live in Valkey DB 0; dfe's `VALKEY_URL` is `/ctech/{env}/valkey/url` (DB 0, verified in `cdk/bin/ctech-dfe-cdk.ts`), so the existing cache backend is passed as is.
- Queue: visibility timeout 30 min, `maxReceiveCount` 5, DLQ retention 14 days.
- Settle window before a company's last rows and its `organizations` item are deleted: 10 minutes after the company was locked (longer than the 300 s worker Lambda timeout).
- Everything erasure-related is OFF until `ERASURE_QUEUE_URL` is set (dark launch).
- After every Go task: `cd api && go vet ./... && go test ./... -race` green (worker task: `cd worker && go test ./...`). Integration tests: `cd api && docker compose -f docker-compose.test.yml up -d && make test-integration`.
- No visible text with travessão (—): use `;` or lists (root CLAUDE.md). Code comments and internal docs are exempt.

## Rulings (decisions this plan takes; cost if wrong)

- **R1 Consumer in the API, not the worker.** The worker is SQS-triggered Lambda; `erasure.Consumer` is a long-poll loop and the API already runs one (results consumer). Cost if wrong: purge competes with request CPU on a t4g.nano; move the loop to a Lambda later by wrapping `ErasureService.Purge`.
- **R2 No dfe blocker codes.** Ownership is checked by ctech-account. "In-flight emission by the user" cannot happen at LOCKED: the user is locked from grace start (7 days, D1) and every job runs ≤ 300 s. Rows have no index by actor, so a check would scan. The endpoint always answers `eligible: true`. Cost if wrong: add a code later; the endpoint shape does not change.
- **R3 Lock choke point = `middleware.Verifier.Middleware`** for `POST/PUT/PATCH/DELETE`, plus `UserService.GetOrCreate` (the only GET that writes). Cost if wrong: one GetItem per write request.
- **R4 `organizations[]` are ctech-account organization ids; dfe tenancy is the company id.** Companies are found through a new GSI `organization-id-index` on `organizations.organization_id`. Legacy `CNPJ_`/`CPF_` rows without `organization_id` are not found (they are erased only by the person purge's membership step). Cost if wrong: an orphan legacy company remains; operator deletes it.
- **R5 Company tombstones are written by dfe** (`ORG#{company_id}`), locked before the purge and erased after it, because workers only know company ids. The consumer still tombstones the account org ids itself.
- **R6 Settle window instead of in-flight detection.** The first delivery locks the companies, deletes everything but the `organizations` items, and returns an error; the redelivery (30 min later) deletes again and finishes. Cost if wrong: one extra SQS receive per deletion with organizations.
- **R7 S3 by row, never by tenant folder.** Emitted XML lives under `CNPJ_{doc}` folders that two companies with the same CNPJ share (see `documentS3Key`). Objects are deleted through the keys stored on rows, plus company-id prefixes (`pdfs/{t}/{c}/`, `{t}/{env}/{c}/`, `{t}-distribution/{env}/{c}/`, `certs/{c}/`). Cost if wrong: an orphan object nobody references stays until bucket lifecycle.
- **R8 e-CPF matching scans `organization_certificates`** and opens each PFX (SAN OID 2.16.76.1.3.1, fallback CN). The table holds one or two rows per company. Cost if wrong: purge time grows with certificate count; upgrade path: store a holder-kind attribute at upload and filter.
- **R9 e-CPF residue:** the PFX object (all versions) and row (with its password) are erased; copies of the password inside `worker_outbox.payload` (30-day TTL) and in-flight SQS bodies are left: the password is useless without the PFX. Cost if wrong: scan the outbox for `cert_s3_key`.
- **R10 "Notify the surviving organization's admins"** = an audit row `CERTIFICATE … DELETE` by `SYSTEM` / `Sistema (exclusão de conta LGPD)` in the org's audit feed. dfe sends no e-mail today. Cost if wrong: admins learn at the next emission; e-mail needs a ctech-account notification route.
- **R11 Ack/subject credential = the existing `ACCOUNT_CLIENT_ID/SECRET`** (reach check client), two token managers with one scope each. Cost if wrong: two more SSM parameters.
- **R12 XML export = one paged endpoint, no UI.** `GET /v1.0/xml-export/{doc_type}` returns a zip of up to 100 production documents and their events, `X-Next-Cursor` for the next page; OWNER/ADMIN only. The dfe UI button and the link from ctech-account's deletion page are a follow-up. Cost if wrong: the user exports one document at a time until the UI ships.
- **R13 `internal:dfe:erasure-eligibility` is not added to `scope-manifest.json`:** the manifest is public-only by test, and ctech-account mints this scope itself. Cost if wrong: add the entry and relax `TestScopeManifestMatchesEnforcementFamilies`.
- **R14 No DLQ alarm in dfe** (dfe has none; `cdk` DOCS §"CloudWatch alarms — none"). ctech-account logs "participant ack overdue" at 48 h. Cost if wrong: a poisoned message waits 48 h to be noticed.
- **R15 Billing rows of the user:** erase `USER_{sub}`, `QUOTA_GUARD_{sub}#companies` and the current period's `USAGE_{sub}#…`; older usage rows (counters only) expire by their 13-month TTL.
- **R16 Document/event rows of surviving orgs keep `user_id`/`user_name`** (inventory §5: fiscal documents of a surviving org are untouched). Only `audit_logs`, memberships and invitations are anonymized.

## Review Focus

1. **A scheduled distribution job writes rows for a company while its purge runs.** The company must end with zero rows. Test: Task 9 `TestPurge_WaitsForSettleWindowBeforeFinishingCompanies`.
2. **The purge crashes between deleting an e-CPF PFX and deleting its row.** The redelivery must still erase the row (and its password). Test: Task 9 `TestPurge_FinishesAHalfErasedCertificate`.
3. **The same `user.erase` is delivered twice (SQS duplicate, reconciler re-publish).** The second run succeeds and changes nothing. Test: Task 9 `TestPurge_EndToEndTwice`.
4. **Two companies share a CNPJ (and its S3 folder); only one is erased.** The other's XML survives. Test: Task 8 `TestPurgeCompany_LeavesASiblingCompanyOnTheSameCNPJ`.
5. **A locked user writes, or logs in for the first time after the purge.** Writes get 403 `account-pending-deletion`; `/auth/me` does not recreate the `users` row. Tests: Task 2 `TestMiddleware_RefusesWritesOfLockedUser`, `TestGetOrCreate_DoesNotResurrectALockedUser`.

## Cross-project impact

- **ctech-account:** add dfe to `ERASURE_PARTICIPANTS` (`service: dfe`, `url: {dfe internal base}/v1.0`, `audience: {dfe SERVICE_AUDIENCE}`, `client_id: {ACCOUNT_CLIENT_ID of dfe}`); grant that confidential client `internal:account:erasure-ack` and `internal:account:erasure-subject`; verify the minted token's `iss` equals dfe's `CTECH_ISSUER_URL`.
- **ctech-account ui:** blocker copy: none needed (R2). Link "Exportar XMLs" to dfe when the dfe UI button ships (R12).
- **ctech-go-common:** none (consumes v1.13.1). README "Account erasure" still says `MessageBody` scope; fixed by ctech-account phase 3 Task 8.
- **ctech-cdk:** candidate shared construct "erasure participant queue" (queue + DLQ + filtered subscription). Built locally here; extract when the second CDK participant (poker/wallet) needs it.
- **ctech-billing:** none (dfe's `account_billing` is a local snapshot).
- **dfe ui:** none in this plan (R12).
- **dfe worker:** new dependency on `api-commons` and a GetItem per job (Task 13).

---

### Task 1: Upgrade `api-commons` to v1.13.1

**Files:**
- Modify: `api/go.mod`, `api/go.sum`

**Interfaces:**
- Produces: packages `gopkg.aoctech.app/api-commons/erasure` and `jwtverify` revocation (`Revoke`, `ErrTokenRevoked`, `Verifier.WithRevocation`) available to later tasks.

- [ ] **Step 1: Baseline.** `cd api && go test ./... -race 2>&1 | tail -5` → all `ok`. If not green on `main`, stop and report.
- [ ] **Step 2: Bump.** `cd api && go get gopkg.aoctech.app/api-commons@v1.13.1 && go mod tidy`
- [ ] **Step 3: Verify.** `grep api-commons go.mod` → `gopkg.aoctech.app/api-commons v1.13.1`. `go build ./... && go vet ./... && go test ./... -race 2>&1 | tail -20` → all `ok`.
- [ ] **Step 4: Integration suite.** `docker compose -f docker-compose.test.yml up -d && make test-integration 2>&1 | tail -5` → `ok`.
- [ ] **Step 5: Commit.** `git add go.mod go.sum && git commit -m "chore(api): api-commons v1.13.1 (erasure, jwt revocation)"`

---

### Task 2: Token revocation and the account lock in the auth middleware

**Files:**
- Create: `api/internal/services/erasure_lock.go`
- Modify: `api/internal/middleware/auth.go`, `api/internal/problem/problem.go`, `api/internal/services/users.go`
- Test: `api/internal/middleware/auth_test.go`, `api/tests/integration/erasure_lock_test.go`

**Interfaces:**
- Produces: `type services.BlockedFunc func(ctx context.Context, sub string) (bool, error)` (matches `(*erasure.Store).Blocked`).
- Produces: `func (v *middleware.Verifier) WithLockCheck(blocked services.BlockedFunc) *middleware.Verifier`.
- Produces: `func (s *services.UserService) WithErasureLock(blocked services.BlockedFunc) *services.UserService`.
- Produces: `problem.TypeAccountPendingDeletion = "/problems/account-pending-deletion"`, `func problem.AccountPendingDeletion(detail string) *problem.Problem` (403).
- Changes: `middleware.NewVerifier` now enables `jwtverify` revocation on the cache backend it receives.

- [ ] **Step 1: Failing tests.** Append to `api/internal/middleware/auth_test.go` (add imports `errors`, `github.com/gofiber/fiber/v3`, `gopkg.aoctech.app/api-commons/jwtverify`):

```go
func TestMiddleware_RefusesWritesOfLockedUser(t *testing.T) {
	key, srv := newJWKSServer(t)
	locked := map[string]bool{"user-locked": true}
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16)).
		WithLockCheck(func(_ context.Context, sub string) (bool, error) { return locked[sub], nil })
	app := fiber.New()
	app.Use(v.Middleware())
	app.All("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	do := func(method, sub string) int {
		now := time.Now().Unix()
		tok := signToken(t, key, jwt.MapClaims{
			"sub": sub, "iss": testIssuer, "aud": []string{testAudience}, "iat": now, "exp": now + 900,
		})
		req := httptest.NewRequest(method, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if got := do(fiber.MethodPost, "user-locked"); got != fiber.StatusForbidden {
		t.Errorf("POST by a locked user = %d, want 403", got)
	}
	if got := do(fiber.MethodGet, "user-locked"); got != fiber.StatusNoContent {
		t.Errorf("GET by a locked user = %d, want 204 (reads are not state-changing)", got)
	}
	if got := do(fiber.MethodDelete, "user-free"); got != fiber.StatusNoContent {
		t.Errorf("DELETE by an active user = %d, want 204", got)
	}
}

func TestMiddleware_LockCheckFailureFailsClosed(t *testing.T) {
	key, srv := newJWKSServer(t)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16)).
		WithLockCheck(func(context.Context, string) (bool, error) { return false, errors.New("dynamodb down") })
	app := fiber.New()
	app.Use(v.Middleware())
	app.Post("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	now := time.Now().Unix()
	tok := signToken(t, key, jwt.MapClaims{"sub": "u", "iss": testIssuer, "aud": []string{testAudience}, "iat": now, "exp": now + 900})
	req := httptest.NewRequest(fiber.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (an unknown lock state must not allow a write)", resp.StatusCode)
	}
}

func TestVerify_RejectsRevokedToken(t *testing.T) {
	key, srv := newJWKSServer(t)
	mem := cache.NewMemoryBackend(16)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, mem)
	now := time.Now()
	tok := signToken(t, key, jwt.MapClaims{
		"sub": "user-1", "iss": testIssuer, "aud": []string{testAudience},
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(14 * time.Minute).Unix(),
	})
	if err := jwtverify.Revoke(context.Background(), mem, "user-1", now, jwtverify.RevocationTTL); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyClaims(context.Background(), tok); !errors.Is(err, jwtverify.ErrTokenRevoked) {
		t.Fatalf("err = %v, want ErrTokenRevoked", err)
	}
}
```

Create `api/tests/integration/erasure_lock_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// TestGetOrCreate_DoesNotResurrectALockedUser: GET /auth/me provisions the
// users row on first sight. For a locked or erased sub it must refuse, or a
// late token would bring back the row the purge deleted.
func TestGetOrCreate_DoesNotResurrectALockedUser(t *testing.T) {
	ctx := context.Background()
	svc := services.NewUserService(userRepo, memCache, "", nil, nil).
		WithErasureLock(func(context.Context, string) (bool, error) { return true, nil })

	_, err := svc.GetOrCreate(ctx, "locked-user-1")
	var p *problem.Problem
	if !errors.As(err, &p) || p.Type != problem.TypeAccountPendingDeletion {
		t.Fatalf("err = %v, want account-pending-deletion", err)
	}
	if item, _ := userRepo.GetByID(ctx, "locked-user-1"); item != nil {
		t.Fatal("a users row was created for a locked sub")
	}
}
```

- [ ] **Step 2: Run to fail.** `cd api && go test ./internal/middleware/ -run 'Locked|LockCheck|Revoked' -count=1` → build FAIL (`WithLockCheck` undefined).
- [ ] **Step 3: Implement.**

`api/internal/services/erasure_lock.go`:

```go
package services

import "context"

// BlockedFunc reports whether a sub is locked or erased by the account-deletion
// saga (ctech-go-common erasure.Store.Blocked). Nil means the lock is off.
type BlockedFunc func(ctx context.Context, sub string) (bool, error)
```

`api/internal/problem/problem.go` — add to the type const block:

```go
	// TypeAccountPendingDeletion refuses a write of an account in the deletion
	// saga (locked during grace, or erased). Same name ctech-account uses.
	TypeAccountPendingDeletion = "/problems/account-pending-deletion"
```

and after `Forbidden`:

```go
// AccountPendingDeletion refuses a state-changing request of a locked account.
func AccountPendingDeletion(detail string) *Problem {
	return New(http.StatusForbidden, TypeAccountPendingDeletion, "Account Pending Deletion", detail)
}
```

`api/internal/middleware/auth.go` — replace the `Verifier` type and `NewVerifier`, and extend `Middleware`:

```go
type Verifier struct {
	*jwtverify.Verifier
	blocked services.BlockedFunc
}

// NewVerifier also turns on the account-deletion revocation list: ctech-account
// writes ctech:jwt:revoked_sub:{sub} in Valkey DB 0, which is this service's
// cache backend (VALKEY_URL has no DB suffix). Fail open when Valkey is down;
// the local lock (WithLockCheck) is the independent second layer.
func NewVerifier(jwksURL, audience, issuer string, cacheBackend cache.Backend) *Verifier {
	v := jwtverify.NewVerifier(jwksURL, audience, issuer, cacheBackend)
	v.WithRevocation(cacheBackend)
	return &Verifier{Verifier: v}
}

// WithLockCheck refuses state-changing requests of a sub the deletion saga
// locked or erased. This middleware is the one choke point every user route
// goes through, so no write path can forget the check.
func (v *Verifier) WithLockCheck(blocked services.BlockedFunc) *Verifier {
	v.blocked = blocked
	return v
}
```

In `Middleware`, after the four `c.Locals(...)` lines and before `return c.Next()`:

```go
		if v.blocked != nil && isMutating(c.Method()) {
			locked, err := v.blocked(c.Context(), claims.Sub)
			if err != nil {
				return problem.InternalServer("estado da conta indisponível").WithCause(err).Send(c)
			}
			if locked {
				return problem.AccountPendingDeletion("Conta em processo de exclusão; operações de escrita estão bloqueadas.").Send(c)
			}
		}
```

Add the import `"gopkg.aoctech.app/dfe/api/internal/services"` (middleware already imports services in `reach.go`; `isMutating` is in `subscription.go`).

`api/internal/services/users.go` — add a field `blocked BlockedFunc` to `UserService`, the method, and the guard:

```go
// WithErasureLock makes GetOrCreate refuse to provision a locked or erased sub.
func (s *UserService) WithErasureLock(blocked BlockedFunc) *UserService {
	s.blocked = blocked
	return s
}
```

In `GetOrCreate`, replace `return s.repo.CreateMinimal(ctx, userID)` with:

```go
	if s.blocked != nil {
		locked, err := s.blocked(ctx, repositories.RawUserID(userID))
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, problem.AccountPendingDeletion("Conta em processo de exclusão.")
		}
	}
	return s.repo.CreateMinimal(ctx, userID)
```

- [ ] **Step 4: Pass.** `go test ./internal/middleware/ -count=1 -race` → `ok`. `make test-integration 2>&1 | grep -E 'Resurrect|^ok|FAIL'` → PASS / `ok`. `go vet ./... && go test ./... -race` → all `ok`.
- [ ] **Step 5: Commit.** `git add internal/middleware internal/problem internal/services/erasure_lock.go internal/services/users.go tests/integration/erasure_lock_test.go && git commit -m "feat(api): jwt revocation and account-deletion write lock"`

---

### Task 3: Eligibility endpoint

**Files:**
- Create: `api/internal/middleware/internal.go`, `api/internal/middleware/internal_test.go`, `api/internal/api/v1/erasure.go`, `api/internal/api/v1/erasure_test.go`
- Modify: `api/internal/api/v1/router.go`, `api/internal/api/v1/openapi/system.yaml`

**Interfaces:**
- Produces: `func middleware.RequireInternalScope(scope string) fiber.Handler`.
- Produces: `const v1.ScopeErasureEligibility = "internal:dfe:erasure-eligibility"`, `func v1.RegisterErasure(router fiber.Router, authMw fiber.Handler)`, route `GET /v1.0/internal/erasure/eligibility/:sub`.

- [ ] **Step 1: Failing tests.** `api/internal/middleware/internal_test.go`:

```go
package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequireInternalScope(t *testing.T) {
	const scope = "internal:dfe:erasure-eligibility"
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error {
		c.Locals(SessionIDKey, c.Get("X-Sid"))
		c.Locals(ScopesKey, strings.Fields(c.Get("X-Scope")))
		return c.Next()
	}, RequireInternalScope(scope), func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	cases := []struct {
		name, sid, scope string
		want             int
	}{
		{"service token with the scope", "", scope, fiber.StatusOK},
		{"service token without it", "", "internal:dfe:other", fiber.StatusForbidden},
		{"user session carrying the scope", "sid-1", scope, fiber.StatusForbidden},
	}
	for _, tc := range cases {
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("X-Sid", tc.sid)
		req.Header.Set("X-Scope", tc.scope)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, resp.StatusCode, tc.want)
		}
	}
}
```

`api/internal/api/v1/erasure_test.go`:

```go
package v1

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// dfe has no blocker of its own (plan ruling R2): every sub is eligible, and
// the body must be the exact erasure.Eligibility shape ctech-account decodes.
func TestErasureEligibility_AlwaysEligible(t *testing.T) {
	app := fiber.New()
	app.Get("/e/:sub", eligibility)
	resp, err := app.Test(httptest.NewRequest("GET", "/e/user-1", nil))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	blockers, ok := body["blockers"].([]any)
	if resp.StatusCode != 200 || body["eligible"] != true || !ok || len(blockers) != 0 {
		t.Fatalf("status %d body %s, want 200 {eligible:true, blockers:[]}", resp.StatusCode, raw)
	}
}
```

- [ ] **Step 2: Fail.** `go test ./internal/middleware/ ./internal/api/v1/ -run 'InternalScope|Eligibility' -count=1` → build FAIL.
- [ ] **Step 3: Implement.** `api/internal/middleware/internal.go`:

```go
package middleware

import (
	"slices"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

// RequireInternalScope guards service-to-service routes; it runs after the auth
// middleware. Only client-credentials tokens pass: they carry no session (user
// and API-key tokens always do) and must hold scope. Same rule as
// ctech-account's middleware of the same name.
func RequireInternalScope(scope string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if GetSessionID(c) != "" || !slices.Contains(GetScopes(c), scope) {
			return problem.Forbidden("rota interna: token de serviço com o escopo exigido").Send(c)
		}
		return c.Next()
	}
}
```

`api/internal/api/v1/erasure.go`:

```go
package v1

import (
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
)

// ScopeErasureEligibility is carried by the token ctech-account mints to ask
// this service for account-deletion blockers (saga protocol §4.1).
const ScopeErasureEligibility = "internal:dfe:erasure-eligibility"

// RegisterErasure mounts the saga's eligibility route.
func RegisterErasure(router fiber.Router, authMw fiber.Handler) {
	router.Get("/internal/erasure/eligibility/:sub", authMw,
		middleware.RequireInternalScope(ScopeErasureEligibility), eligibility)
}

// eligibility has no blocker to report (plan ruling R2): ownership of shared
// organizations is ctech-account's check, and a locked user cannot start
// an emission.
func eligibility(c fiber.Ctx) error {
	return c.JSON(erasure.NewEligibility())
}
```

`router.go`: after `RegisterAuditLogs(...)` add `RegisterErasure(v1, authMw)`.

`openapi/system.yaml` — append under `paths:`:

```yaml
  /v1.0/internal/erasure/eligibility/{sub}:
    get:
      tags: [ Auth ]
      summary: Bloqueios desta API para excluir a conta (saga LGPD)
      description: |
        Chamada pelo ctech-account com token de serviço (sem sessão) e escopo
        `internal:dfe:erasure-eligibility`. A DF-e não tem bloqueio próprio:
        a resposta é sempre `eligible: true`.
      operationId: erasureEligibility
      parameters:
        - name: sub
          in: path
          required: true
          schema: { type: string }
      responses:
        '200':
          description: Elegibilidade
          content:
            application/json:
              schema:
                type: object
                required: [ eligible, blockers ]
                properties:
                  eligible: { type: boolean }
                  blockers:
                    type: array
                    items:
                      type: object
                      properties:
                        code: { type: string }
                        detail: { type: object }
                        action_url: { type: string }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
```

- [ ] **Step 4: Pass.** `go test ./internal/middleware/ ./internal/api/v1/ -count=1` → `ok` (includes `TestOpenAPI*`). `go vet ./... && go test ./... -race` → `ok`.
- [ ] **Step 5: Commit.** `git add internal/middleware/internal.go internal/middleware/internal_test.go internal/api/v1 && git commit -m "feat(api): erasure eligibility endpoint for ctech-account"`

---

### Task 4: Versioned S3 purge helper and an in-memory S3 double

**Files:**
- Create: `api/internal/testsupport/objectstore.go`, `api/internal/services/erasure_s3.go`, `api/internal/services/erasure_s3_test.go`

**Interfaces:**
- Produces: `type services.ObjectStore interface { GetObject; ListObjectVersions; DeleteObjects }` (aws-sdk-go-v2 `*s3.Client` satisfies it).
- Produces: `func services.purgeObjects(ctx, store ObjectStore, bucket, prefix string, exact bool) (int, error)` (unexported).
- Produces: `testsupport.NewObjectStore() *testsupport.ObjectStore` with `Put(bucket, key, body string)`, `Versions(bucket, key string) int`, `Keys(bucket string) []string`.

- [ ] **Step 1: Double.** `api/internal/testsupport/objectstore.go`:

```go
// Package testsupport holds test doubles shared by unit and integration tests.
package testsupport

import (
	"context"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type version struct {
	id      string
	body    string
	deleted bool
}

// ObjectStore is an in-memory, versioned S3 double.
type ObjectStore struct {
	mu      sync.Mutex
	next    int
	objects map[string][]*version // "bucket/key" → versions, oldest first
}

func NewObjectStore() *ObjectStore { return &ObjectStore{objects: map[string][]*version{}} }

// Put adds a new version of key.
func (s *ObjectStore) Put(bucket, key, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	k := bucket + "/" + key
	s.objects[k] = append(s.objects[k], &version{id: strconv.Itoa(s.next), body: body})
}

// Versions counts the versions of key that were not deleted.
func (s *ObjectStore) Versions(bucket, key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, v := range s.objects[bucket+"/"+key] {
		if !v.deleted {
			n++
		}
	}
	return n
}

// Keys lists the keys of bucket that still have a version.
func (s *ObjectStore) Keys(bucket string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for k, vs := range s.objects {
		key, ok := strings.CutPrefix(k, bucket+"/")
		if !ok {
			continue
		}
		for _, v := range vs {
			if !v.deleted {
				out = append(out, key)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *ObjectStore) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vs := s.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(in.Key)]
	for i := len(vs) - 1; i >= 0; i-- {
		if !vs[i].deleted {
			return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(vs[i].body))}, nil
		}
	}
	return nil, &s3types.NoSuchKey{}
}

func (s *ObjectStore) ListObjectVersions(_ context.Context, in *s3.ListObjectVersionsInput, _ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bucket, prefix := aws.ToString(in.Bucket), aws.ToString(in.Prefix)
	out := &s3.ListObjectVersionsOutput{IsTruncated: aws.Bool(false)}
	keys := make([]string, 0, len(s.objects))
	for k := range s.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		key, ok := strings.CutPrefix(k, bucket+"/")
		if !ok || !strings.HasPrefix(key, prefix) {
			continue
		}
		for _, v := range s.objects[k] {
			if !v.deleted {
				out.Versions = append(out.Versions, s3types.ObjectVersion{Key: aws.String(key), VersionId: aws.String(v.id)})
			}
		}
	}
	return out, nil
}

func (s *ObjectStore) DeleteObjects(_ context.Context, in *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range in.Delete.Objects {
		for _, v := range s.objects[aws.ToString(in.Bucket)+"/"+aws.ToString(o.Key)] {
			if v.id == aws.ToString(o.VersionId) {
				v.deleted = true
			}
		}
	}
	return &s3.DeleteObjectsOutput{}, nil
}
```

- [ ] **Step 2: Failing test.** `api/internal/services/erasure_s3_test.go`:

```go
package services

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

func TestPurgeObjects_EveryVersionOfOnePrefix(t *testing.T) {
	ctx := context.Background()
	s := testsupport.NewObjectStore()
	s.Put("b", "certs/C1/a.pfx", "v1")
	s.Put("b", "certs/C1/a.pfx", "v2")
	s.Put("b", "certs/C1/b.pfx", "v1")
	s.Put("b", "certs/C10/a.pfx", "other company")

	n, err := purgeObjects(ctx, s, "b", "certs/C1/", false)
	if err != nil || n != 3 {
		t.Fatalf("purged %d (%v), want 3", n, err)
	}
	if got := s.Keys("b"); len(got) != 1 || got[0] != "certs/C10/a.pfx" {
		t.Fatalf("remaining %v, want only certs/C10/a.pfx", got)
	}
}

func TestPurgeObjects_ExactKeyLeavesSiblings(t *testing.T) {
	ctx := context.Background()
	s := testsupport.NewObjectStore()
	s.Put("b", "nfe/prod/CNPJ_1/K1.xml", "x")
	s.Put("b", "nfe/prod/CNPJ_1/K1.xml.bak", "sibling")

	if _, err := purgeObjects(ctx, s, "b", "nfe/prod/CNPJ_1/K1.xml", true); err != nil {
		t.Fatal(err)
	}
	if s.Versions("b", "nfe/prod/CNPJ_1/K1.xml") != 0 || s.Versions("b", "nfe/prod/CNPJ_1/K1.xml.bak") != 1 {
		t.Fatalf("exact purge touched the wrong keys: %v", s.Keys("b"))
	}
}

func TestPurgeObjects_EmptyPrefixIsANoop(t *testing.T) {
	s := testsupport.NewObjectStore()
	s.Put("b", "anything", "x")
	if n, err := purgeObjects(context.Background(), s, "b", "", false); err != nil || n != 0 || s.Versions("b", "anything") != 1 {
		t.Fatal("an empty prefix must never sweep the bucket")
	}
}
```

- [ ] **Step 3: Fail.** `go test ./internal/services/ -run PurgeObjects -count=1` → build FAIL.
- [ ] **Step 4: Implement.** `api/internal/services/erasure_s3.go`:

```go
package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ObjectStore is the S3 surface the account-deletion purge and the XML export
// use. *s3.Client satisfies it; tests use testsupport.ObjectStore.
type ObjectStore interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectVersions(ctx context.Context, in *s3.ListObjectVersionsInput, opts ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	DeleteObjects(ctx context.Context, in *s3.DeleteObjectsInput, opts ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

const s3DeleteBatch = 1000 // DeleteObjects limit

// purgeObjects deletes every version and delete marker under prefix (inventory
// §2: versioned buckets keep old versions). exact limits it to the key equal to
// prefix, because a key is also a prefix of its siblings. An empty prefix is a
// no-op: nothing here may ever sweep a whole bucket.
func purgeObjects(ctx context.Context, store ObjectStore, bucket, prefix string, exact bool) (int, error) {
	if prefix == "" {
		return 0, nil
	}
	n := 0
	var keyMarker, versionMarker *string
	for {
		out, err := store.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
			Bucket: aws.String(bucket), Prefix: aws.String(prefix),
			KeyMarker: keyMarker, VersionIdMarker: versionMarker,
		})
		if err != nil {
			return n, fmt.Errorf("s3: list versions of %s: %w", prefix, err)
		}
		var ids []s3types.ObjectIdentifier
		add := func(key, versionID *string) {
			if exact && aws.ToString(key) != prefix {
				return
			}
			ids = append(ids, s3types.ObjectIdentifier{Key: key, VersionId: versionID})
		}
		for _, v := range out.Versions {
			add(v.Key, v.VersionId)
		}
		for _, d := range out.DeleteMarkers {
			add(d.Key, d.VersionId)
		}
		for len(ids) > 0 {
			batch := ids[:min(s3DeleteBatch, len(ids))]
			ids = ids[len(batch):]
			res, err := store.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(bucket),
				Delete: &s3types.Delete{Objects: batch, Quiet: aws.Bool(true)},
			})
			if err != nil {
				return n, fmt.Errorf("s3: delete versions of %s: %w", prefix, err)
			}
			if len(res.Errors) > 0 {
				return n, fmt.Errorf("s3: delete versions of %s: %s", prefix, aws.ToString(res.Errors[0].Message))
			}
			n += len(batch)
		}
		if !aws.ToBool(out.IsTruncated) {
			return n, nil
		}
		keyMarker, versionMarker = out.NextKeyMarker, out.NextVersionIdMarker
	}
}
```

- [ ] **Step 5: Pass.** `go test ./internal/services/ -run PurgeObjects -count=1 -race` → `ok`; `go vet ./...` clean.
- [ ] **Step 6: Commit.** `git add internal/testsupport internal/services/erasure_s3.go internal/services/erasure_s3_test.go && git commit -m "feat(api): versioned S3 purge helper for account deletion"`

---
### Task 5: e-CPF holder CPF from an ICP-Brasil certificate

**Files:**
- Create: `api/internal/services/erasure_ecpf.go`, `api/internal/services/erasure_ecpf_test.go`

**Interfaces:**
- Produces: `func services.holderCPF(cert *x509.Certificate) string` — the 11-digit CPF in SAN otherName OID 2.16.76.1.3.1, or `""`.
- Produces: `func services.certificateHeldBy(pfx []byte, password, cpf string) bool` — true when the PFX's holder CPF (SAN, fallback CN via `ParsePFX`) equals `cpf`.

- [ ] **Step 1: Failing test.** `api/internal/services/erasure_ecpf_test.go`:

```go
package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// icpBrasilSAN builds a SubjectAltName with the ICP-Brasil "pessoa física"
// otherName: birth date (8) + CPF (11) + NIS (11) + RG (15) + issuer (6).
func icpBrasilSAN(t *testing.T, cpf string) pkix.Extension {
	t.Helper()
	inner, err := asn1.Marshal("01011990" + cpf + "00000000000" + "000000000000000" + "SSPPI ")
	if err != nil {
		t.Fatal(err)
	}
	on := struct {
		TypeID asn1.ObjectIdentifier
		Value  asn1.RawValue
	}{oidICPBrasilHolderPF, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: inner}}
	onBytes, err := asn1.MarshalWithParams(on, "tag:0")
	if err != nil {
		t.Fatal(err)
	}
	san, err := asn1.Marshal([]asn1.RawValue{{FullBytes: onBytes}})
	if err != nil {
		t.Fatal(err)
	}
	return pkix.Extension{Id: oidSubjectAltName, Value: san}
}

func ecpfPFX(t *testing.T, cn string, ext []pkix.Extension) ([]byte, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		ExtraExtensions: ext,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, "pw")
	if err != nil {
		t.Fatal(err)
	}
	return pfx, cert
}

func TestHolderCPF_ReadsICPBrasilOtherName(t *testing.T) {
	_, cert := ecpfPFX(t, "FULANO DE TAL", []pkix.Extension{icpBrasilSAN(t, "12345678909")})
	if got := holderCPF(cert); got != "12345678909" {
		t.Fatalf("holderCPF = %q, want 12345678909", got)
	}
}

func TestHolderCPF_EmptyWithoutTheOID(t *testing.T) {
	_, cert := ecpfPFX(t, "EMPRESA LTDA:12345678000195", nil)
	if got := holderCPF(cert); got != "" {
		t.Fatalf("holderCPF = %q, want empty for an e-CNPJ", got)
	}
}

func TestCertificateHeldBy(t *testing.T) {
	bySAN, _ := ecpfPFX(t, "FULANO DE TAL", []pkix.Extension{icpBrasilSAN(t, "12345678909")})
	byCN, _ := ecpfPFX(t, "FULANO DE TAL:12345678909", nil)
	other, _ := ecpfPFX(t, "OUTRA PESSOA:98765432100", nil)

	if !certificateHeldBy(bySAN, "pw", "12345678909") {
		t.Error("SAN holder not matched")
	}
	if !certificateHeldBy(byCN, "pw", "12345678909") {
		t.Error("CN fallback not matched")
	}
	if certificateHeldBy(other, "pw", "12345678909") {
		t.Error("another person's e-CPF matched")
	}
	if certificateHeldBy(bySAN, "wrong-password", "12345678909") {
		t.Error("an unreadable PFX must not match")
	}
	if certificateHeldBy(bySAN, "pw", "") {
		t.Error("an empty CPF must match nothing")
	}
}
```

- [ ] **Step 2: Fail.** `go test ./internal/services/ -run 'HolderCPF|CertificateHeldBy' -count=1` → build FAIL.
- [ ] **Step 3: Implement.** `api/internal/services/erasure_ecpf.go`:

```go
package services

import (
	"crypto/x509"
	"encoding/asn1"
)

var (
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}
	// oidICPBrasilHolderPF is the ICP-Brasil "dados do titular pessoa física"
	// otherName: birth date (8 digits) followed by the CPF (11 digits).
	oidICPBrasilHolderPF = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 1}
)

const (
	icpBirthDateLen = 8
	cpfLen          = 11
)

// holderCPF returns the CPF of an e-CPF's holder, read from the SubjectAltName
// otherName 2.16.76.1.3.1, or "" when the certificate carries none.
func holderCPF(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}
		var names []asn1.RawValue
		if _, err := asn1.Unmarshal(ext.Value, &names); err != nil {
			return ""
		}
		for _, n := range names {
			if n.Class != asn1.ClassContextSpecific || n.Tag != 0 {
				continue
			}
			var on struct {
				TypeID asn1.ObjectIdentifier
				Value  asn1.RawValue // [0] EXPLICIT wrapper around the string
			}
			if _, err := asn1.UnmarshalWithParams(n.FullBytes, &on, "tag:0"); err != nil || !on.TypeID.Equal(oidICPBrasilHolderPF) {
				continue
			}
			var inner asn1.RawValue
			if _, err := asn1.Unmarshal(on.Value.Bytes, &inner); err != nil {
				continue
			}
			digits := stripNonDigits(string(inner.Bytes))
			if len(digits) >= icpBirthDateLen+cpfLen {
				return digits[icpBirthDateLen : icpBirthDateLen+cpfLen]
			}
		}
	}
	return ""
}

// certificateHeldBy reports whether pfx is an e-CPF of cpf (D14). A PFX that
// cannot be opened matches nothing. The CPF is compared in memory only.
func certificateHeldBy(pfx []byte, password, cpf string) bool {
	if cpf == "" {
		return false
	}
	cert, _, info, err := ParsePFX(pfx, password)
	if err != nil {
		return false
	}
	return holderCPF(cert) == cpf || info.CPF == cpf
}
```

`services` has no digits-only helper (the ones in `nfes`, `mdfes` and `documents` are package-private there), so add to `erasure_ecpf.go`:

```go
func stripNonDigits(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			out = append(out, s[i])
		}
	}
	return string(out)
}
```

- [ ] **Step 4: Pass.** `go test ./internal/services/ -run 'HolderCPF|CertificateHeldBy' -count=1 -race` → `ok`.
- [ ] **Step 5: Commit.** `git add internal/services/erasure_ecpf.go internal/services/erasure_ecpf_test.go && git commit -m "feat(api): match e-CPF certificates by ICP-Brasil holder CPF"`

---

### Task 6: `ErasureRepository` — table-agnostic pages and deletes

**Files:**
- Create: `api/internal/repositories/erasure.go`, `api/tests/integration/erasure_repository_test.go`

**Interfaces:**
- Produces (`package repositories`):
  - `type Page struct { Items []map[string]types.AttributeValue; Next map[string]types.AttributeValue }`
  - `func NewErasureRepository(db *dynamodb.Client, cfg *config.Config) *ErasureRepository`
  - `GetItem(ctx, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error)`
  - `QueryPartition(ctx, table, pk string, start map[string]types.AttributeValue) (Page, error)`
  - `QueryPrefix(ctx, table, pk, skPrefix string, start map[string]types.AttributeValue) (Page, error)`
  - `QueryIndex(ctx, table, index, attr, value string, start map[string]types.AttributeValue) (Page, error)`
  - `ScanPage(ctx, table string, start map[string]types.AttributeValue) (Page, error)`
  - `DeleteKeys(ctx, table string, keys []map[string]types.AttributeValue) error`
  - `DeleteKey(ctx, table string, key map[string]types.AttributeValue) (bool, error)` (true when a row existed)
  - `SetAttrs(ctx, table string, key, attrs map[string]types.AttributeValue) error` (never creates a row)
  - `func SAV(v string) types.AttributeValue` (string attribute shorthand)

`table` is the physical name without the `{prefix}_` (e.g. `organization_products`).

- [ ] **Step 1: Failing test.** `api/tests/integration/erasure_repository_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestErasureRepository_PagesDeletesAndNeverCreates(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewErasureRepository(db, cfg)
	const table = "organization_products"
	org := "erasure-repo-org"

	for i := range 30 {
		putItem(t, table, map[string]any{"pk": org, "sk": fmt.Sprintf("PRODUCT_%02d", i)})
	}
	var keys []map[string]types.AttributeValue
	var start map[string]types.AttributeValue
	for {
		page, err := repo.QueryPartition(ctx, table, org, start)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range page.Items {
			keys = append(keys, map[string]types.AttributeValue{"pk": it["pk"], "sk": it["sk"]})
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	if len(keys) != 30 {
		t.Fatalf("paged %d rows, want 30", len(keys))
	}
	if err := repo.DeleteKeys(ctx, table, keys); err != nil { // > 25: two batches
		t.Fatal(err)
	}
	if page, _ := repo.QueryPartition(ctx, table, org, nil); len(page.Items) != 0 {
		t.Fatalf("%d rows left after DeleteKeys", len(page.Items))
	}

	missing := map[string]types.AttributeValue{"pk": repositories.SAV(org), "sk": repositories.SAV("PRODUCT_99")}
	if err := repo.SetAttrs(ctx, table, missing, map[string]types.AttributeValue{"name": repositories.SAV("x")}); err != nil {
		t.Fatal(err)
	}
	if item, _ := repo.GetItem(ctx, table, missing); item != nil {
		t.Fatal("SetAttrs created a row: an anonymization must never resurrect one")
	}
	if existed, err := repo.DeleteKey(ctx, table, missing); err != nil || existed {
		t.Fatalf("DeleteKey of an absent row = %v, %v; want false, nil", existed, err)
	}
}
```

Add the shared seeding helpers to `api/tests/integration/setup_test.go` (imports `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`, `gopkg.aoctech.app/api-commons/dynamo`):

```go
// putItem writes item to {tablePrefix}_{table}.
func putItem(t *testing.T, table string, item map[string]any) {
	t.Helper()
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.PutItem(context.Background(), &dynamodb.PutItemInput{
		TableName: aws.String(dynamo.TableName(tablePrefix, table)), Item: av,
	}); err != nil {
		t.Fatalf("put %s: %v", table, err)
	}
}

// getItem reads key from {tablePrefix}_{table}; nil when absent.
func getItem(t *testing.T, table string, key map[string]any) map[string]types.AttributeValue {
	t.Helper()
	av, err := attributevalue.MarshalMap(key)
	if err != nil {
		t.Fatal(err)
	}
	out, err := db.GetItem(context.Background(), &dynamodb.GetItemInput{
		TableName: aws.String(dynamo.TableName(tablePrefix, table)), Key: av, ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		t.Fatalf("get %s: %v", table, err)
	}
	if len(out.Item) == 0 {
		return nil
	}
	return out.Item
}
```

- [ ] **Step 2: Fail.** `make test-integration 2>&1 | grep -E 'ErasureRepository|FAIL|undefined'` → build FAIL (`NewErasureRepository` undefined).
- [ ] **Step 3: Implement.** `api/internal/repositories/erasure.go`:

```go
package repositories

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/dynamo"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// ErasureRepository is the account-deletion purge's access to every table by
// name. The purge is table-driven (services/erasure_catalog.go), so it works on
// keys and pages instead of the typed repositories, which each know one table.
type ErasureRepository struct {
	db     *dynamodb.Client
	prefix string
}

func NewErasureRepository(db *dynamodb.Client, cfg *config.Config) *ErasureRepository {
	return &ErasureRepository{db: db, prefix: cfg.TablePrefix}
}

// Page is one page of items and the key to continue from (nil at the end).
type Page struct {
	Items []map[string]types.AttributeValue
	Next  map[string]types.AttributeValue
}

const (
	erasurePageSize      = 100
	batchWriteMax        = 25
	batchWriteMaxRetries = 5
)

// SAV is a string attribute value.
func SAV(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

func (r *ErasureRepository) name(table string) *string {
	return aws.String(dynamo.TableName(r.prefix, table))
}

func (r *ErasureRepository) GetItem(ctx context.Context, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{TableName: r.name(table), Key: key, ConsistentRead: aws.Bool(true)})
	if err != nil {
		return nil, wrapDynamoErr(err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return out.Item, nil
}

func (r *ErasureRepository) query(ctx context.Context, in *dynamodb.QueryInput) (Page, error) {
	in.Limit = aws.Int32(erasurePageSize)
	out, err := r.db.Query(ctx, in)
	if err != nil {
		return Page{}, wrapDynamoErr(err)
	}
	return Page{Items: out.Items, Next: out.LastEvaluatedKey}, nil
}

func (r *ErasureRepository) QueryPartition(ctx context.Context, table, pk string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		KeyConditionExpression:    aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": SAV(pk)},
		ExclusiveStartKey:         start,
		ConsistentRead:            aws.Bool(true),
	})
}

func (r *ErasureRepository) QueryPrefix(ctx context.Context, table, pk, skPrefix string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		KeyConditionExpression:    aws.String("pk = :pk AND begins_with(sk, :p)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": SAV(pk), ":p": SAV(skPrefix)},
		ExclusiveStartKey:         start,
		ConsistentRead:            aws.Bool(true),
	})
}

func (r *ErasureRepository) QueryIndex(ctx context.Context, table, index, attr, value string, start map[string]types.AttributeValue) (Page, error) {
	return r.query(ctx, &dynamodb.QueryInput{
		TableName:                 r.name(table),
		IndexName:                 aws.String(index),
		KeyConditionExpression:    aws.String("#k = :v"),
		ExpressionAttributeNames:  map[string]string{"#k": attr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": SAV(value)},
		ExclusiveStartKey:         start,
	})
}

// ScanPage reads one page of a whole table. Used only for the e-CPF search
// (plan ruling R8): certificates carry no attribute a query could key on.
func (r *ErasureRepository) ScanPage(ctx context.Context, table string, start map[string]types.AttributeValue) (Page, error) {
	out, err := r.db.Scan(ctx, &dynamodb.ScanInput{
		TableName: r.name(table), ExclusiveStartKey: start,
		Limit: aws.Int32(erasurePageSize), ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return Page{}, wrapDynamoErr(err)
	}
	return Page{Items: out.Items, Next: out.LastEvaluatedKey}, nil
}

// DeleteKeys deletes keys in batches of 25, retrying unprocessed items.
// Deleting an absent key is a no-op, so a rerun is safe.
func (r *ErasureRepository) DeleteKeys(ctx context.Context, table string, keys []map[string]types.AttributeValue) error {
	for len(keys) > 0 {
		batch := keys[:min(batchWriteMax, len(keys))]
		keys = keys[len(batch):]
		reqs := make([]types.WriteRequest, 0, len(batch))
		for _, k := range batch {
			reqs = append(reqs, types.WriteRequest{DeleteRequest: &types.DeleteRequest{Key: k}})
		}
		pending := map[string][]types.WriteRequest{*r.name(table): reqs}
		for attempt := 0; len(pending) > 0; attempt++ {
			if attempt == batchWriteMaxRetries {
				return fmt.Errorf("dynamodb: %s: deletes still unprocessed after %d attempts", table, attempt)
			}
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
			}
			out, err := r.db.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{RequestItems: pending})
			if err != nil {
				return wrapDynamoErr(err)
			}
			pending = out.UnprocessedItems
		}
	}
	return nil
}

func (r *ErasureRepository) DeleteKey(ctx context.Context, table string, key map[string]types.AttributeValue) (bool, error) {
	out, err := r.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: r.name(table), Key: key, ReturnValues: types.ReturnValueAllOld,
	})
	if err != nil {
		return false, wrapDynamoErr(err)
	}
	return len(out.Attributes) > 0, nil
}

// SetAttrs overwrites attrs on an existing row. A missing row is left missing:
// an anonymization must never create the row the purge already deleted.
func (r *ErasureRepository) SetAttrs(ctx context.Context, table string, key, attrs map[string]types.AttributeValue) error {
	names := make([]string, 0, len(attrs))
	for n := range attrs {
		names = append(names, n)
	}
	sort.Strings(names)
	exprNames := map[string]string{}
	exprValues := map[string]types.AttributeValue{}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		nk, vk := fmt.Sprintf("#a%d", i), fmt.Sprintf(":v%d", i)
		exprNames[nk], exprValues[vk] = n, attrs[n]
		parts = append(parts, nk+" = "+vk)
	}
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: r.name(table), Key: key,
		UpdateExpression:          aws.String("SET " + strings.Join(parts, ", ")),
		ConditionExpression:       aws.String("attribute_exists(pk)"),
		ExpressionAttributeNames:  exprNames,
		ExpressionAttributeValues: exprValues,
	})
	if IsConditionFailed(err) {
		return nil
	}
	return wrapDynamoErr(err)
}
```

- [ ] **Step 4: Pass.** `make test-integration 2>&1 | grep -E 'ErasureRepository|^ok|FAIL'` → PASS, `ok`. `go vet ./...` clean.
- [ ] **Step 5: Commit.** `git add internal/repositories/erasure.go tests/integration && git commit -m "feat(api): table-agnostic repository for the account-deletion purge"`

---

### Task 7: Purge catalog with a CDK coverage test

**Files:**
- Create: `api/internal/services/erasure_catalog.go`, `api/internal/services/erasure_catalog_test.go`
- Modify: `cdk/lib/dynamodb-stack.ts` (add `'erasure_state'` to `TableName` only; the table itself is Task 14), `api/internal/services/shared.go` (add `InutEventPK`), `api/internal/services/nfes/inutilization.go` (use it), `api/internal/repositories/account_billing.go` (export `QuotaGuardPK`)

**Interfaces:**
- Produces: `type services.ErasureScope int` with `ScopeKept, ScopePerson, ScopeOrganization, ScopeOrgItem, ScopeOrgPartition, ScopeEnvDocs, ScopeEnvPartition, ScopeDocEvents, ScopeOutbox, ScopeInvitations, ScopeSerieClaims`.
- Produces: `type services.ErasureTable struct { CDK, Name string; Scope ErasureScope; SortKey, Events string; Inut bool }`, `func services.ErasureCatalog() []services.ErasureTable`.
- Produces: `func services.InutEventPK(env, orgPK string) string` (`INUT#{env}#{orgPK}`), `func repositories.QuotaGuardPK(userID, meter string) string` (renamed from `quotaGuardPK`).

- [ ] **Step 1: Failing test.** `api/internal/services/erasure_catalog_test.go`:

```go
package services

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// TestErasureCatalogCoversEveryCDKTable fails when a table is added to
// cdk/lib/dynamodb-stack.ts without deciding how the account-deletion purge
// reaches it (inventory §5: "erase everything keyed by the org").
func TestErasureCatalogCoversEveryCDKTable(t *testing.T) {
	src, err := os.ReadFile("../../../cdk/lib/dynamodb-stack.ts")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)export type TableName = \((.*?)\)`).FindSubmatch(src)
	if block == nil {
		t.Fatal("TableName union not found in dynamodb-stack.ts")
	}
	cdk := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([a-z_]+)'`).FindAllSubmatch(block[1], -1) {
		cdk[string(m[1])] = true
	}

	catalog := map[string]ErasureTable{}
	for _, tb := range ErasureCatalog() {
		if _, dup := catalog[tb.CDK]; dup {
			t.Fatalf("catalog lists %s twice", tb.CDK)
		}
		catalog[tb.CDK] = tb
	}
	var missing, stale []string
	for name := range cdk {
		if _, ok := catalog[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range catalog {
		if !cdk[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 || len(stale) > 0 {
		t.Fatalf("catalog drift: missing from catalog %v; not in CDK %v", missing, stale)
	}

	byName := map[string]ErasureTable{}
	for _, tb := range catalog {
		byName[tb.Name] = tb
	}
	for _, tb := range catalog {
		if tb.Scope == ScopeEnvDocs {
			if ev, ok := byName[tb.Events]; !ok || ev.Scope != ScopeDocEvents {
				t.Errorf("%s: events table %q is not a ScopeDocEvents entry", tb.Name, tb.Events)
			}
		}
	}
}

func TestInutEventPK(t *testing.T) {
	if got := InutEventPK(EnvProd, "C1"); got != "INUT#prod#C1" {
		t.Fatalf("InutEventPK = %q", got)
	}
}
```

- [ ] **Step 2: Fail.** `go test ./internal/services/ -run 'ErasureCatalog|InutEventPK' -count=1` → build FAIL.
- [ ] **Step 3: Implement.**

`cdk/lib/dynamodb-stack.ts`: in `export type TableName = (`, after `'worker_outbox'` add `|\n  'erasure_state'` (so the union ends `'worker_outbox' |\n  'erasure_state'\n  )`).

`api/internal/services/shared.go` (next to the other env helpers):

```go
// InutEventPK is the synthetic events-table partition of an organization's
// inutilizações in one environment: INUT#{env}#{org_pk}.
func InutEventPK(env, orgPK string) string {
	return "INUT#" + env + "#" + orgPK
}
```

`api/internal/services/nfes/inutilization.go`: replace the body of `inutEventPK` with `return services.InutEventPK(envPrefix, orgPK)` (the `fmt` import stays if still used elsewhere in the file; `go build` tells).

`api/internal/repositories/account_billing.go`: rename `quotaGuardPK` to `QuotaGuardPK` (definition and its one caller in `BuildQuotaGuardTx`) and give it the comment `// QuotaGuardPK keys the concurrency guard of a live resource quota.`

`api/internal/services/erasure_catalog.go`:

```go
package services

import (
	"slices"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// ErasureScope says how the account-deletion purge reaches a table's rows.
type ErasureScope int

const (
	ScopeKept         ErasureScope = iota // global (roles) or the tombstone table itself
	ScopePerson                           // keyed by the user's sub; person purge
	ScopeOrganization                     // the organizations item; deleted last, after the settle window
	ScopeOrgItem                          // pk = company id, no sort key (fiscal configs)
	ScopeOrgPartition                     // pk = company id, sk
	ScopeEnvDocs                          // pk = {env}#{company}; each row owns events and an outbox row
	ScopeEnvPartition                     // pk = {env}#{company} (distributions)
	ScopeDocEvents                        // pk = a document's sk; reached through ScopeEnvDocs
	ScopeOutbox                           // pk = {doc table}#{doc sk}; reached through ScopeEnvDocs
	ScopeInvitations                      // org_pk on org-invite-index
	ScopeSerieClaims                      // global; derived from the fiscal configs
)

// ErasureTable is one DynamoDB table and how the purge reaches it.
type ErasureTable struct {
	CDK     string // member of TableName in cdk/lib/dynamodb-stack.ts
	Name    string // physical name after "{prefix}_"
	Scope   ErasureScope
	SortKey string // "" (pk only), "sk" or "nsu"
	Events  string // ScopeEnvDocs: the events table keyed by the document's sk
	Inut    bool   // ScopeDocEvents: also holds INUT#{env}#{company} partitions
}

const skKey = "sk"

var erasureCatalog = []ErasureTable{
	{CDK: "roles", Name: "roles", Scope: ScopeKept},
	{CDK: "erasure_state", Name: "erasure_state", Scope: ScopeKept},
	{CDK: "users", Name: "users", Scope: ScopePerson},
	{CDK: "account_billing", Name: "account_billing", Scope: ScopePerson},
	{CDK: "organizations", Name: "organizations", Scope: ScopeOrganization},
	{CDK: "organization_users", Name: "organization_users", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "organization_invitations", Name: "organization_invitations", Scope: ScopeInvitations},
	{CDK: "serie_claims", Name: "serie_claims", Scope: ScopeSerieClaims},
	{CDK: "audit_logs", Name: "audit_logs", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "products", Name: "organization_products", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "vehicles", Name: "organization_vehicles", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "persons", Name: "organization_persons", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "certificates", Name: "organization_certificates", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "services", Name: "organization_services", Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "tax_profiles", Name: repositories.TableTaxProfiles, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "operations", Name: repositories.TableOperations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "payment_terms", Name: repositories.TablePaymentTerms, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "payment_terminals", Name: repositories.TablePaymentTerminals, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "toll_providers", Name: repositories.TableTollProviders, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "cargo_units", Name: repositories.TableCargoUnits, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "import_declarations", Name: repositories.TableImportDeclarations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "insurance_policies", Name: repositories.TableInsurancePolicies, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "product_lots", Name: repositories.TableProductLots, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "fuel_pumps", Name: repositories.TableFuelPumps, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "vehicle_sets", Name: repositories.TableVehicleSets, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "service_locations", Name: repositories.TableServiceLocations, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "reference_documents", Name: repositories.TableReferenceDocuments, Scope: ScopeOrgPartition, SortKey: skKey},
	{CDK: "nfe_configs", Name: "organization_nfe_configs", Scope: ScopeOrgItem},
	{CDK: "nfce_configs", Name: "organization_nfce_configs", Scope: ScopeOrgItem},
	{CDK: "cte_configs", Name: "organization_cte_configs", Scope: ScopeOrgItem},
	{CDK: "mdfe_configs", Name: "organization_mdfe_configs", Scope: ScopeOrgItem},
	{CDK: "nfse_configs", Name: "organization_nfse_configs", Scope: ScopeOrgItem},
	{CDK: "nfes", Name: "nfes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfe_events"},
	{CDK: "nfces", Name: "nfces", Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfce_events"},
	{CDK: "ctes", Name: "ctes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "cte_events"},
	{CDK: "mdfes", Name: "mdfes", Scope: ScopeEnvDocs, SortKey: skKey, Events: "mdfe_events"},
	{CDK: "nfses", Name: repositories.TableNfses, Scope: ScopeEnvDocs, SortKey: skKey, Events: "nfse_events"},
	{CDK: "nfe_events", Name: "nfe_events", Scope: ScopeDocEvents, SortKey: skKey, Inut: true},
	{CDK: "nfce_events", Name: "nfce_events", Scope: ScopeDocEvents, SortKey: skKey, Inut: true},
	{CDK: "cte_events", Name: "cte_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "mdfe_events", Name: "mdfe_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "nfse_events", Name: "nfse_events", Scope: ScopeDocEvents, SortKey: skKey},
	{CDK: "nfe_distributions", Name: "nfe_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "cte_distributions", Name: "cte_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "mdfe_distributions", Name: "mdfe_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "nfse_distributions", Name: "nfse_distributions", Scope: ScopeEnvPartition, SortKey: "nsu"},
	{CDK: "worker_outbox", Name: "worker_outbox", Scope: ScopeOutbox, SortKey: skKey},
}

// ErasureCatalog returns a copy of the purge catalog.
func ErasureCatalog() []ErasureTable { return slices.Clone(erasureCatalog) }
```

- [ ] **Step 4: Pass.** `go test ./internal/services/... -count=1 -race` → `ok`; `go vet ./...` clean; `cd ../cdk && npx tsc --noEmit` → no output.
- [ ] **Step 5: Commit.** `git add api/internal cdk/lib/dynamodb-stack.ts && git commit -m "feat(api): account-deletion purge catalog covering every CDK table"` (run from repo root).

---
### Task 8: Company purge (`ErasureService.PurgeCompany`)

**Files:**
- Create: `api/internal/services/erasure.go`, `api/tests/integration/erasure_test.go`
- Modify: `api/tests/integration/setup_test.go` (organizations GSI, catalog tables, drop)

**Interfaces:**
- Consumes: `repositories.ErasureRepository` (Task 6), `ErasureCatalog`/`InutEventPK` (Task 7), `purgeObjects`/`ObjectStore` (Task 4).
- Produces:
  - `const services.ErasureServiceName = "dfe"`
  - `type services.SubjectLookup func(ctx context.Context, requestID string) (string, error)`
  - `func services.NewErasureService(repo *repositories.ErasureRepository, certs *repositories.CertificateRepository, audit *repositories.AuditLogRepository, claims *repositories.SerieClaimRepository, store *erasure.Store, objects ObjectStore, docsBucket, certsBucket string, subject SubjectLookup, c cache.Backend) *ErasureService`
  - `func (s *ErasureService) WithSettle(d time.Duration) *ErasureService`
  - `func (s *ErasureService) PurgeCompany(ctx context.Context, company string) (map[string]int, error)` — every row and object of the company **except** its `organizations` item (deleted by `Purge` after the settle window, Task 9).
  - unexported, used by Task 9: `purgeCompany`, `purgeIndexed`, `rowKey`, `pkKey`, `avNum`, table-name consts `tbl*`, `erasureEnvs`.

- [ ] **Step 1: Harness.** In `api/tests/integration/setup_test.go`:

Replace the `_organizations` definition with one that carries the new GSI:

```go
		{
			TableName:   aws.String(tablePrefix + "_organizations"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("organization_id"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("organization-id-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("organization_id"), KeyType: types.KeyTypeHash},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly},
				},
			},
		},
```

Just before `for _, def := range definitions {` in `createTables`, add:

```go
	// The account-deletion purge reaches every catalog table: create the ones
	// the definitions above do not, with the plain key shape the purge uses.
	defined := map[string]bool{}
	for _, d := range definitions {
		defined[*d.TableName] = true
	}
	for _, tb := range services.ErasureCatalog() {
		if name := tablePrefix + "_" + tb.Name; !defined[name] {
			definitions = append(definitions, plainTable(name, tb.SortKey))
		}
	}
```

At the end of `dropTables`, add:

```go
	for _, tb := range services.ErasureCatalog() {
		_, _ = db.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: aws.String(tablePrefix + "_" + tb.Name)})
	}
```

and the helper:

```go
// plainTable is a pk (S) table with an optional sort key ("nsu" is numeric).
func plainTable(name, sortKey string) dynamodb.CreateTableInput {
	in := dynamodb.CreateTableInput{
		TableName:   aws.String(name),
		BillingMode: types.BillingModePayPerRequest,
		KeySchema:   []types.KeySchemaElement{{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash}},
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
		},
	}
	if sortKey != "" {
		typ := types.ScalarAttributeTypeS
		if sortKey == "nsu" {
			typ = types.ScalarAttributeTypeN
		}
		in.KeySchema = append(in.KeySchema, types.KeySchemaElement{AttributeName: aws.String(sortKey), KeyType: types.KeyTypeRange})
		in.AttributeDefinitions = append(in.AttributeDefinitions, types.AttributeDefinition{AttributeName: aws.String(sortKey), AttributeType: typ})
	}
	return in
}
```

- [ ] **Step 2: Failing test.** `api/tests/integration/erasure_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"testing"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

const (
	erasureDocsBucket  = "docs"
	erasureCertsBucket = "certs"
)

func newErasureSvc(objects *testsupport.ObjectStore, subject services.SubjectLookup) (*services.ErasureService, *erasure.Store) {
	store := erasure.NewStore(db, tablePrefix, 0)
	return services.NewErasureService(
		repositories.NewErasureRepository(db, cfg), certRepo, auditRepo,
		repositories.NewSerieClaimRepository(db, cfg), store, objects,
		erasureDocsBucket, erasureCertsBucket, subject, memCache,
	), store
}

type seedRow struct {
	table string
	item  map[string]any
}

// companySeed is one row in every kind of table the company purge reaches.
func companySeed(c, accountOrg, tax string) []seedRow {
	k := "NFEKEY-" + c
	dps := "DPS-" + c
	return []seedRow{
		{"organizations", map[string]any{"pk": c, "organization_id": accountOrg, "tax_id": tax, "tax_id_kind": "cnpj"}},
		{"organization_nfe_configs", map[string]any{"pk": c, "prod_current_serie": 1}},
		{"organization_nfse_configs", map[string]any{"pk": c}},
		{"serie_claims", map[string]any{"pk": repositories.SerieClaimPK(tax, services.ModelNFe, services.AmbienteProd, 1), "company_id": c}},
		{"nfes", map[string]any{"pk": "prod#" + c, "sk": k, "xml_s3_key": "nfe/prod/CNPJ_" + tax + "/" + k + ".xml"}},
		{"nfe_events", map[string]any{"pk": k, "sk": "EV1", "xml_s3_key": "nfe/prod/CNPJ_" + tax + "/" + k + "_110111_001.xml"}},
		{"nfe_events", map[string]any{"pk": services.InutEventPK(services.EnvHom, c), "sk": "EV2"}},
		{"worker_outbox", map[string]any{"pk": "nfes#" + k, "sk": "command"}},
		{"nfses", map[string]any{"pk": "hom#" + c, "sk": dps, "xml_s3_key": "nfse/hom/" + c + "/" + dps + ".xml", "dps_xml_s3_key": "nfse/hom/" + c + "/" + dps + "_dps.xml"}},
		{"nfse_events", map[string]any{"pk": dps, "sk": "EV3"}},
		{"nfe_distributions", map[string]any{"pk": "hom#" + c, "nsu": 7, "xml_s3_key": "nfe-distribution/hom/" + c + "/NSU_000000000000007.xml"}},
		{"organization_products", map[string]any{"pk": c, "sk": "PRODUCT_1"}},
		{repositories.TableFuelPumps, map[string]any{"pk": c, "sk": "FUELPUMP_1"}},
		{"organization_certificates", map[string]any{"pk": c, "sk": "CERTIFICATE_m", "md5": "m", "s3_key": "certs/" + c + "/m.pfx", "password": "pw"}},
		{"audit_logs", map[string]any{"pk": c, "sk": "PRODUCT#PRODUCT_1#01", "user_id": "u-x", "created_at": "2026-10-07T00:00:00Z"}},
		{"organization_users", map[string]any{"pk": c, "sk": "USER_u-x"}},
		{"organization_invitations", map[string]any{"pk": "INVITE_" + c, "org_pk": c, "created_at": "2026-10-07T00:00:00Z"}},
	}
}

// seed writes rows and, for every object key a row carries, two versions of
// the object (versioned buckets keep the old one).
func seed(t *testing.T, objects *testsupport.ObjectStore, rows []seedRow) {
	t.Helper()
	for _, r := range rows {
		putItem(t, r.table, r.item)
		for _, attr := range []string{"xml_s3_key", "dps_xml_s3_key"} {
			if key, ok := r.item[attr].(string); ok {
				objects.Put(erasureDocsBucket, key, "v1")
				objects.Put(erasureDocsBucket, key, "v2")
			}
		}
		if key, ok := r.item["s3_key"].(string); ok {
			objects.Put(erasureCertsBucket, key, "pfx")
		}
	}
}

func seedKey(r seedRow) map[string]any {
	key := map[string]any{"pk": r.item["pk"]}
	for _, tb := range services.ErasureCatalog() {
		if tb.Name == r.table && tb.SortKey != "" {
			key[tb.SortKey] = r.item[tb.SortKey]
		}
	}
	return key
}

func TestPurgeCompany_ErasesEveryCompanyRow(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, nil)
	c, tax := "erase-co-"+randomCNPJ(), randomCNPJ()
	rows := companySeed(c, "acct-org-"+c, tax)
	seed(t, objects, rows)
	objects.Put(erasureDocsBucket, "pdfs/nfe/"+c+"/v1/NFEKEY-"+c+"-authorized.pdf", "pdf")

	for range 2 { // a rerun must succeed and change nothing
		if _, err := svc.PurgeCompany(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range rows {
		got := getItem(t, r.table, seedKey(r))
		if r.table == "organizations" {
			if got == nil {
				t.Error("the organizations item goes only after the settle window (Purge, not PurgeCompany)")
			}
			continue
		}
		if got != nil {
			t.Errorf("%s %v survived the company purge", r.table, seedKey(r))
		}
	}
	if left := objects.Keys(erasureDocsBucket); len(left) != 0 {
		t.Errorf("document objects left: %v", left)
	}
	if left := objects.Keys(erasureCertsBucket); len(left) != 0 {
		t.Errorf("certificate objects left: %v", left)
	}
}

// Two companies may hold the same CNPJ (ctech-billing ADR 0022). Emitted XML
// sits in a folder named after the CNPJ, and a branch's certificate row points
// at the matriz's PFX. Erasing one company must leave the other's objects.
func TestPurgeCompany_LeavesASiblingCompanyOnTheSameCNPJ(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, nil)
	tax := randomCNPJ()
	c, d := "erase-c-"+randomCNPJ(), "keep-d-"+randomCNPJ()
	seed(t, objects, companySeed(c, "acct-c-"+c, tax))
	dRows := companySeed(d, "acct-d-"+d, tax) // same CNPJ; its série claim row wins
	seed(t, objects, dRows)
	putItem(t, "organization_certificates", map[string]any{
		"pk": c, "sk": "CERTIFICATE_d", "md5": "d", "s3_key": "certs/" + d + "/m.pfx", "password": "pw",
	})

	if _, err := svc.PurgeCompany(ctx, c); err != nil {
		t.Fatal(err)
	}
	for _, r := range dRows {
		if getItem(t, r.table, seedKey(r)) == nil {
			t.Errorf("sibling row %s %v was erased", r.table, seedKey(r))
		}
		if key, ok := r.item["xml_s3_key"].(string); ok && objects.Versions(erasureDocsBucket, key) == 0 {
			t.Errorf("sibling object %s was erased", key)
		}
	}
	if objects.Versions(erasureCertsBucket, "certs/"+d+"/m.pfx") == 0 {
		t.Error("the sibling's PFX, shared by a branch row of the erased company, was erased")
	}
}
```

- [ ] **Step 3: Fail.** `make test-integration 2>&1 | grep -E 'PurgeCompany|undefined|FAIL'` → build FAIL (`NewErasureService` undefined).
- [ ] **Step 4: Implement.** `api/internal/services/erasure.go`:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// ErasureServiceName is this service's id in erasure messages, acks and the
// SNS filter policy.
const ErasureServiceName = "dfe"

const (
	erasureSettleWindow = 10 * time.Minute // > the 300 s worker Lambda timeout

	organizationIDIndex = "organization-id-index"
	orgInviteIndex      = "org-invite-index"
	userIndex           = "user-index"
	userIDIndex         = "user-id-index"
	outboxCommandSK     = "command"

	tblOrganizations  = "organizations"
	tblOrgUsers       = "organization_users"
	tblInvitations    = "organization_invitations"
	tblAuditLogs      = "audit_logs"
	tblCertificates   = "organization_certificates"
	tblOutbox         = "worker_outbox"
	tblUsers          = "users"
	tblAccountBilling = "account_billing"

	countS3Versions  = "s3_object_versions"
	countSerieClaims = "serie_claims_released"
)

var (
	erasureEnvs     = []string{EnvHom, EnvProd}
	erasureDocTypes = []string{"nfe", "nfce", "cte", "mdfe", "nfse"}
	// serieConfigModels are the fiscal configs whose séries are claimed
	// (NFS-e is municipal and claims none).
	serieConfigModels = map[string]string{
		"organization_nfe_configs":  ModelNFe,
		"organization_nfce_configs": ModelNFCe,
		"organization_cte_configs":  ModelCTe,
		"organization_mdfe_configs": ModelMDFe,
	}
)

// SubjectLookup returns the CPF of the user of a purging request (ctech-account
// GET /internal/erasure/{request_id}/subject). The CPF stays in memory.
type SubjectLookup func(ctx context.Context, requestID string) (string, error)

// ErasureService is dfe's side of the account-deletion saga
// (docs/plans/2026-10-07-account-deletion-participant.md).
type ErasureService struct {
	repo        *repositories.ErasureRepository
	certs       *repositories.CertificateRepository
	audit       *repositories.AuditLogRepository
	claims      *repositories.SerieClaimRepository
	store       *erasure.Store
	objects     ObjectStore
	docsBucket  string
	certsBucket string
	subject     SubjectLookup
	cache       cache.Backend
	settle      time.Duration
	now         func() time.Time
}

func NewErasureService(
	repo *repositories.ErasureRepository,
	certs *repositories.CertificateRepository,
	audit *repositories.AuditLogRepository,
	claims *repositories.SerieClaimRepository,
	store *erasure.Store,
	objects ObjectStore,
	docsBucket, certsBucket string,
	subject SubjectLookup,
	c cache.Backend,
) *ErasureService {
	return &ErasureService{
		repo: repo, certs: certs, audit: audit, claims: claims, store: store, objects: objects,
		docsBucket: docsBucket, certsBucket: certsBucket, subject: subject, cache: c,
		settle: erasureSettleWindow, now: time.Now,
	}
}

// WithSettle overrides the settle window (tests use 0).
func (s *ErasureService) WithSettle(d time.Duration) *ErasureService {
	s.settle = d
	return s
}

func pkKey(pk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": repositories.SAV(pk)}
}

func rowKey(item map[string]types.AttributeValue, sortKey string) map[string]types.AttributeValue {
	key := map[string]types.AttributeValue{"pk": item["pk"]}
	if sortKey != "" {
		key[sortKey] = item[sortKey]
	}
	return key
}

func avNum(item map[string]types.AttributeValue, key string) int {
	if v, ok := item[key].(*types.AttributeValueMemberN); ok {
		n, _ := strconv.Atoi(v.Value)
		return n
	}
	return 0
}

// PurgeCompany erases every row and object of one company except its
// organizations item, which Purge deletes once the settle window has passed.
func (s *ErasureService) PurgeCompany(ctx context.Context, company string) (map[string]int, error) {
	counts := map[string]int{}
	return counts, s.purgeCompany(ctx, company, counts)
}

func (s *ErasureService) purgeCompany(ctx context.Context, company string, counts map[string]int) error {
	if err := s.releaseSerieClaims(ctx, company, counts); err != nil {
		return fmt.Errorf("serie claims: %w", err)
	}
	for _, t := range erasureCatalog {
		if err := s.purgeTable(ctx, t, company, counts); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	for _, p := range companyDocumentPrefixes(company) {
		n, err := purgeObjects(ctx, s.objects, s.docsBucket, p, false)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	n, err := purgeObjects(ctx, s.objects, s.certsBucket, companyCertPrefix(company), false)
	if err != nil {
		return err
	}
	counts[countS3Versions] += n
	for _, p := range []string{"dfe:res:" + company + ":", "dfe:member:" + company + ":", "dfe:reach:" + company + ":"} {
		if err := s.cache.DeletePrefix(ctx, p); err != nil {
			observability.Warn(ctx, "erasure: cache prefix delete failed", err, "prefix", p)
		}
	}
	return nil
}

func (s *ErasureService) purgeTable(ctx context.Context, t ErasureTable, company string, counts map[string]int) error {
	switch t.Scope {
	case ScopeOrgPartition:
		return s.purgePartition(ctx, company, t.Name, t.SortKey, company, counts)
	case ScopeOrgItem:
		existed, err := s.repo.DeleteKey(ctx, t.Name, pkKey(company))
		if existed {
			counts[t.Name]++
		}
		return err
	case ScopeEnvDocs:
		for _, env := range erasureEnvs {
			if err := s.purgeDocs(ctx, company, t, env+"#"+company, counts); err != nil {
				return err
			}
		}
	case ScopeEnvPartition:
		for _, env := range erasureEnvs {
			if err := s.purgePartition(ctx, company, t.Name, t.SortKey, env+"#"+company, counts); err != nil {
				return err
			}
		}
	case ScopeDocEvents:
		if t.Inut {
			for _, env := range erasureEnvs {
				if err := s.purgePartition(ctx, company, t.Name, t.SortKey, InutEventPK(env, company), counts); err != nil {
					return err
				}
			}
		}
	case ScopeInvitations:
		return s.purgeIndexed(ctx, t.Name, orgInviteIndex, "org_pk", company, counts)
	case ScopeKept, ScopePerson, ScopeOrganization, ScopeOutbox, ScopeSerieClaims:
		// Kept; person purge; after the settle window; through each document;
		// before the fiscal configs go (releaseSerieClaims).
	default:
		return fmt.Errorf("erasure: scope %d has no purge rule", t.Scope)
	}
	return nil
}

func (s *ErasureService) purgePartition(ctx context.Context, company, table, sortKey, pk string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPartition(ctx, table, pk, start)
		if err != nil {
			return err
		}
		if err := s.deleteRows(ctx, company, table, sortKey, page.Items, counts); err != nil {
			return err
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// purgeDocs erases a document partition: each document's events and outbox
// command first, then its objects and the document row itself.
func (s *ErasureService) purgeDocs(ctx context.Context, company string, t ErasureTable, pk string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPartition(ctx, t.Name, pk, start)
		if err != nil {
			return err
		}
		for _, doc := range page.Items {
			sk := avAttr(doc, skKey)
			if err := s.purgePartition(ctx, company, t.Events, skKey, sk, counts); err != nil {
				return err
			}
			existed, err := s.repo.DeleteKey(ctx, tblOutbox, map[string]types.AttributeValue{
				"pk": repositories.SAV(t.Name + "#" + sk), "sk": repositories.SAV(outboxCommandSK),
			})
			if err != nil {
				return err
			}
			if existed {
				counts[tblOutbox]++
			}
		}
		if err := s.deleteRows(ctx, company, t.Name, t.SortKey, page.Items, counts); err != nil {
			return err
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// deleteRows erases the objects the rows point to, then the rows. The row is
// the only index to its objects, so it goes last: a crash in between leaves
// both to the rerun.
func (s *ErasureService) deleteRows(ctx context.Context, company, table, sortKey string, items []map[string]types.AttributeValue, counts map[string]int) error {
	if len(items) == 0 {
		return nil
	}
	keys := make([]map[string]types.AttributeValue, 0, len(items))
	for _, it := range items {
		if err := s.purgeRowObjects(ctx, company, it, counts); err != nil {
			return err
		}
		keys = append(keys, rowKey(it, sortKey))
	}
	if err := s.repo.DeleteKeys(ctx, table, keys); err != nil {
		return err
	}
	counts[table] += len(keys)
	return nil
}

// purgeRowObjects deletes the objects a row names (ruling R7). A certificate
// object is deleted only under the company's own certs/ folder: a branch's row
// points at its matriz's PFX, which belongs to the matriz.
func (s *ErasureService) purgeRowObjects(ctx context.Context, company string, item map[string]types.AttributeValue, counts map[string]int) error {
	for _, attr := range []string{"xml_s3_key", "dps_xml_s3_key"} {
		n, err := purgeObjects(ctx, s.objects, s.docsBucket, avAttr(item, attr), true)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	if key := avAttr(item, "s3_key"); strings.HasPrefix(key, companyCertPrefix(company)) {
		n, err := purgeObjects(ctx, s.objects, s.certsBucket, key, true)
		if err != nil {
			return err
		}
		counts[countS3Versions] += n
	}
	return nil
}

func (s *ErasureService) purgeIndexed(ctx context.Context, table, index, attr, value string, counts map[string]int) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, table, index, attr, value, start)
		if err != nil {
			return err
		}
		keys := make([]map[string]types.AttributeValue, 0, len(page.Items))
		for _, it := range page.Items {
			keys = append(keys, rowKey(it, ""))
		}
		if err := s.repo.DeleteKeys(ctx, table, keys); err != nil {
			return err
		}
		counts[table] += len(keys)
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// releaseSerieClaims frees the séries the company's fiscal configs claim. It
// runs before the configs are deleted, because the claims are derived from
// them. A série held by another company on the same CNPJ is not ours to free.
func (s *ErasureService) releaseSerieClaims(ctx context.Context, company string, counts map[string]int) error {
	org, err := s.repo.GetItem(ctx, tblOrganizations, pkKey(company))
	if err != nil || org == nil {
		return err
	}
	taxID, _ := IssuerDocAV(org, company)
	if taxID == "" {
		return nil
	}
	for table, modelo := range serieConfigModels {
		cfg, err := s.repo.GetItem(ctx, table, pkKey(company))
		if err != nil {
			return err
		}
		for _, c := range SerieClaimsFor(modelo, avNum(cfg, "prod_current_serie"), avNum(cfg, "hom_current_serie")) {
			err := s.claims.Release(ctx, taxID, c.Modelo, c.Ambiente, c.Serie, company)
			if errors.Is(err, repositories.ErrSerieTaken) {
				continue
			}
			if err != nil {
				return err
			}
			counts[countSerieClaims]++
		}
	}
	return nil
}

// companyDocumentPrefixes are the documents-bucket folders named after the
// company id, which no other company shares (unlike the CNPJ_ folders).
func companyDocumentPrefixes(company string) []string {
	var out []string
	for _, t := range erasureDocTypes {
		out = append(out, "pdfs/"+t+"/"+company+"/")
		for _, env := range erasureEnvs {
			out = append(out, t+"/"+env+"/"+company+"/", t+"-distribution/"+env+"/"+company+"/")
		}
	}
	return out
}

func companyCertPrefix(company string) string { return "certs/" + company + "/" }
```

- [ ] **Step 5: Pass.** `make test-integration 2>&1 | grep -E 'PurgeCompany|^ok|FAIL'` → both PASS, `ok`. `go vet ./... && go test ./... -race` → `ok`.
- [ ] **Step 6: Commit.** `git add internal/services/erasure.go tests/integration && git commit -m "feat(api): table-driven company purge for account deletion"`

---

### Task 9: Person purge, e-CPF erasure and `Purge`

**Files:**
- Create: `api/internal/services/erasure_person.go`
- Modify: `api/tests/integration/erasure_test.go`

**Interfaces:**
- Consumes: Task 8 internals; `certificateHeldBy` (Task 5); `repositories.QuotaGuardPK` (Task 7).
- Produces:
  - `func (s *ErasureService) Purge(ctx context.Context, m erasure.Message) (erasure.Ack, error)` — an `erasure.PurgeFunc`.
  - `var services.ErrErasureSettling` — returned while a company was locked less than the settle window ago (the consumer leaves the message; it comes back after the visibility timeout).
- Ack `Counts` keys: physical table names, plus `s3_object_versions`, `serie_claims_released`, `audit_logs_anonymized`, `audit_logs_scrubbed`, `organization_invitations_anonymized`, `organization_certificates_ecpf`.

- [ ] **Step 1: Failing tests.** Append to `api/tests/integration/erasure_test.go` (add imports `crypto/rand`, `crypto/rsa`, `crypto/x509`, `crypto/x509/pkix`, `errors`, `math/big`, `time`, `github.com/aws/aws-sdk-go-v2/service/dynamodb/types`, `software.sslmate.com/src/go-pkcs12`):

```go
const erasureTestCPF = "12345678909"

func ecpfPFX(t *testing.T, cn string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, "pw")
	if err != nil {
		t.Fatal(err)
	}
	return string(pfx)
}

func sAttr(item map[string]types.AttributeValue, name string) string {
	if v, ok := item[name].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func eraseMessage(requestID, sub string, orgs ...string) erasure.Message {
	return erasure.Message{
		Version: erasure.Version, Type: erasure.TypeErase, RequestID: requestID, Sub: sub,
		Scope: erasure.ScopeAccount, Services: []string{services.ErasureServiceName},
		Organizations: orgs, Attempt: 1, IssuedAt: time.Now(),
	}
}

func cpfOf(cpf string) services.SubjectLookup {
	return func(context.Context, string) (string, error) { return cpf, nil }
}

func TestPurge_EndToEndTwice(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, store := newErasureSvc(objects, cpfOf(erasureTestCPF))
	svc.WithSettle(0)

	u, v := "user-"+randomCNPJ(), "user-"+randomCNPJ()
	o1 := "acct-o1-" + randomCNPJ()
	c1, c2, c3 := "c1-"+randomCNPJ(), "c2-"+randomCNPJ(), "c3-"+randomCNPJ()
	c1Rows := companySeed(c1, o1, randomCNPJ())
	seed(t, objects, c1Rows)
	putItem(t, "organization_users", map[string]any{"pk": c1, "sk": "USER_" + u})

	// c2 survives: u is an ADMIN beside v; u's e-CPF is installed there and
	// shared by a branch row in c3.
	putItem(t, "organizations", map[string]any{"pk": c2, "organization_id": "acct-o2-" + c2})
	putItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + u, "name": "Fulano"})
	putItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + v})
	objects.Put(erasureCertsBucket, "certs/"+c2+"/b.pfx", ecpfPFX(t, "FULANO DE TAL:"+erasureTestCPF))
	objects.Put(erasureCertsBucket, "certs/"+c2+"/b.pfx", ecpfPFX(t, "FULANO DE TAL:"+erasureTestCPF))
	objects.Put(erasureCertsBucket, "certs/"+c2+"/c.pfx", ecpfPFX(t, "OUTRA PESSOA:98765432100"))
	putItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_b", "md5": "b", "s3_key": "certs/" + c2 + "/b.pfx", "password": "pw"})
	putItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_c", "md5": "c", "s3_key": "certs/" + c2 + "/c.pfx", "password": "pw"})
	putItem(t, "organization_certificates", map[string]any{"pk": c3, "sk": "CERTIFICATE_b", "md5": "b", "s3_key": "certs/" + c2 + "/b.pfx", "password": "pw"})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "PRODUCT#P#1", "user_id": u, "user_name": "Fulano", "created_at": "2026-10-07T00:00:01Z"})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "CERTIFICATE#b#1", "user_id": v, "created_at": "2026-10-07T00:00:02Z",
		"modifications": []map[string]any{{"name": "alias", "after": "FULANO DE TAL:" + erasureTestCPF}}})
	putItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "MEMBER#" + u + "#1", "user_id": v, "created_at": "2026-10-07T00:00:03Z",
		"modifications": []map[string]any{{"name": "name", "after": "Fulano"}}})
	putItem(t, "organization_invitations", map[string]any{"pk": "INVITE_acc_" + u, "org_pk": c2, "accepted_by": u, "created_at": "2026-10-07T00:00:04Z"})
	putItem(t, "organization_invitations", map[string]any{"pk": "INVITE_sent_" + u, "org_pk": c2, "invited_by": u, "invited_by_name": "Fulano", "accepted_by": v, "created_at": "2026-10-07T00:00:05Z"})
	putItem(t, "users", map[string]any{"pk": "USER_" + u})
	putItem(t, "users", map[string]any{"pk": "USER_" + v})
	putItem(t, "account_billing", map[string]any{"pk": "USER_" + u, "period_start": "2026-10-01"})
	putItem(t, "account_billing", map[string]any{"pk": repositories.UsageCounterPK(u, "2026-10-01")})

	m := eraseMessage("req-e2e-"+u, u, o1, "acct-org-with-no-dfe-company")
	for i := range 2 {
		ack, err := svc.Purge(ctx, m)
		if err != nil || ack.Result != erasure.ResultDone {
			t.Fatalf("run %d: ack %+v, err %v", i+1, ack, err)
		}
	}

	for _, r := range c1Rows {
		if getItem(t, r.table, seedKey(r)) != nil {
			t.Errorf("c1 row %s %v survived", r.table, seedKey(r))
		}
	}
	if rec, err := store.Get(ctx, erasure.OrgKey(c1)); err != nil || rec == nil || rec.State != erasure.StateErased {
		t.Errorf("company tombstone = %+v (%v), want erased", rec, err)
	}
	if getItem(t, "organizations", map[string]any{"pk": c2}) == nil {
		t.Error("the surviving organization was erased")
	}
	if getItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + u}) != nil {
		t.Error("u's membership in the surviving org remains")
	}
	if getItem(t, "organization_users", map[string]any{"pk": c2, "sk": "USER_" + v}) == nil {
		t.Error("v's membership was erased")
	}
	for _, k := range []map[string]any{{"pk": c2, "sk": "CERTIFICATE_b"}, {"pk": c3, "sk": "CERTIFICATE_b"}} {
		if getItem(t, "organization_certificates", k) != nil {
			t.Errorf("e-CPF row %v remains", k)
		}
	}
	if objects.Versions(erasureCertsBucket, "certs/"+c2+"/b.pfx") != 0 {
		t.Error("a version of the e-CPF PFX remains")
	}
	if getItem(t, "organization_certificates", map[string]any{"pk": c2, "sk": "CERTIFICATE_c"}) == nil {
		t.Error("another person's e-CPF was erased")
	}
	row := getItem(t, "audit_logs", map[string]any{"pk": c2, "sk": "PRODUCT#P#1"})
	if sAttr(row, "user_id") == u || sAttr(row, "user_name") != "Usuário removido" {
		t.Errorf("audit actor not anonymized: %v", row)
	}
	for _, sk := range []string{"CERTIFICATE#b#1", "MEMBER#" + u + "#1"} {
		mods, _ := getItem(t, "audit_logs", map[string]any{"pk": c2, "sk": sk})["modifications"].(*types.AttributeValueMemberL)
		if mods == nil || len(mods.Value) != 0 {
			t.Errorf("audit %s still carries personal modifications", sk)
		}
	}
	notice, err := repositories.NewErasureRepository(db, cfg).QueryPrefix(ctx, "audit_logs", c2, "CERTIFICATE#b#", nil)
	if err != nil {
		t.Fatal(err)
	}
	system := 0
	for _, it := range notice.Items {
		if sAttr(it, "user_id") == "SYSTEM" {
			system++
		}
	}
	if system != 1 {
		t.Errorf("want exactly one SYSTEM audit notice for the erased certificate, got %d", system)
	}
	if getItem(t, "organization_invitations", map[string]any{"pk": "INVITE_acc_" + u}) != nil {
		t.Error("an invitation u received remains")
	}
	sent := getItem(t, "organization_invitations", map[string]any{"pk": "INVITE_sent_" + u})
	if sent == nil || sAttr(sent, "invited_by") == u || sAttr(sent, "invited_by_name") == "Fulano" {
		t.Errorf("an invitation u sent was not anonymized: %v", sent)
	}
	if getItem(t, "users", map[string]any{"pk": "USER_" + u}) != nil || getItem(t, "users", map[string]any{"pk": "USER_" + v}) == nil {
		t.Error("users table: u must be gone and v kept")
	}
	if getItem(t, "account_billing", map[string]any{"pk": "USER_" + u}) != nil ||
		getItem(t, "account_billing", map[string]any{"pk": repositories.UsageCounterPK(u, "2026-10-01")}) != nil {
		t.Error("u's billing snapshot or current usage remains")
	}
}

// Review Focus 1: a distribution job may write for a company while it is being
// purged. The first delivery locks and empties the company but keeps its
// organizations item; only a delivery after the settle window finishes.
func TestPurge_WaitsForSettleWindowBeforeFinishingCompanies(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, store := newErasureSvc(objects, cpfOf(""))
	u, o, c := "user-"+randomCNPJ(), "acct-"+randomCNPJ(), "co-"+randomCNPJ()
	seed(t, objects, companySeed(c, o, randomCNPJ()))
	m := eraseMessage("req-settle-"+u, u, o)

	if _, err := svc.Purge(ctx, m); !errors.Is(err, services.ErrErasureSettling) {
		t.Fatalf("first delivery err = %v, want ErrErasureSettling", err)
	}
	if rec, _ := store.Get(ctx, erasure.OrgKey(c)); rec == nil || rec.State != erasure.StateLocked {
		t.Fatalf("company record = %+v, want locked (workers drop its jobs)", rec)
	}
	if getItem(t, "organizations", map[string]any{"pk": c}) == nil {
		t.Fatal("organizations item deleted before the settle window: a rerun could no longer find the company")
	}
	putItem(t, "nfe_distributions", map[string]any{"pk": "prod#" + c, "nsu": 99}) // a late job's write

	ack, err := svc.WithSettle(0).Purge(ctx, m)
	if err != nil || ack.Result != erasure.ResultDone {
		t.Fatalf("redelivery: %+v, %v", ack, err)
	}
	if getItem(t, "nfe_distributions", map[string]any{"pk": "prod#" + c, "nsu": 99}) != nil {
		t.Error("the late write survived")
	}
	if getItem(t, "organizations", map[string]any{"pk": c}) != nil {
		t.Error("organizations item remains after the settle window")
	}
}

// Review Focus 2: the purge crashed after deleting an e-CPF's PFX and before
// deleting its row. The marker left on the row lets the rerun finish without
// being able to open the PFX.
func TestPurge_FinishesAHalfErasedCertificate(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	svc, _ := newErasureSvc(objects, cpfOf(erasureTestCPF))
	u, c := "user-"+randomCNPJ(), "half-"+randomCNPJ()
	requestID := "req-half-" + u
	putItem(t, "organization_certificates", map[string]any{
		"pk": c, "sk": "CERTIFICATE_h", "md5": "h", "s3_key": "certs/" + c + "/h.pfx", "password": "pw",
		"erasure_request": requestID,
	})

	if _, err := svc.WithSettle(0).Purge(ctx, eraseMessage(requestID, u)); err != nil {
		t.Fatal(err)
	}
	if getItem(t, "organization_certificates", map[string]any{"pk": c, "sk": "CERTIFICATE_h"}) != nil {
		t.Fatal("the half-erased certificate row (with its password) remains")
	}
}
```

- [ ] **Step 2: Fail.** `make test-integration 2>&1 | grep -E 'Purge_|undefined|FAIL'` → build FAIL (`Purge` undefined).
- [ ] **Step 3: Implement.** `api/internal/services/erasure_person.go`:

```go
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	erasedActorName        = "Usuário removido"
	systemActorID          = "SYSTEM"
	systemErasureActorName = "Sistema (exclusão de conta LGPD)"
	attrErasureRequest     = "erasure_request"
	calendarMonthLayout    = "2006-01"
)

// ErrErasureSettling: a company was locked less than the settle window ago.
// The consumer leaves the message, and the redelivery finishes the purge.
var ErrErasureSettling = errors.New("erasure: companies locked less than the settle window ago; waiting for redelivery")

// Purge is the erasure.PurgeFunc (saga protocol §4.3): erase every company of
// the organizations erased with the user (D4), then the person. Idempotent and
// resumable: every step re-reads its rows.
func (s *ErasureService) Purge(ctx context.Context, m erasure.Message) (erasure.Ack, error) {
	counts := map[string]int{}
	companies, err := s.companiesOf(ctx, m.Organizations)
	if err != nil {
		return erasure.Ack{}, err
	}
	settled := true
	for _, c := range companies {
		ok, err := s.lockCompany(ctx, c, m)
		if err != nil {
			return erasure.Ack{}, err
		}
		settled = settled && ok
	}
	for _, c := range companies {
		if err := s.purgeCompany(ctx, c, counts); err != nil {
			return erasure.Ack{}, fmt.Errorf("erasure: company %s: %w", c, err)
		}
	}
	if !settled {
		return erasure.Ack{}, ErrErasureSettling
	}
	for _, c := range companies {
		if _, err := s.store.Apply(ctx, erasure.OrgKey(c), m); err != nil {
			return erasure.Ack{}, err
		}
		existed, err := s.repo.DeleteKey(ctx, tblOrganizations, pkKey(c))
		if err != nil {
			return erasure.Ack{}, err
		}
		if existed {
			counts[tblOrganizations]++
		}
		cacheDelete(ctx, s.cache, "dfe:org:"+c)
	}
	if err := s.purgePerson(ctx, m, counts); err != nil {
		return erasure.Ack{}, err
	}
	return erasure.Ack{Result: erasure.ResultDone, Counts: counts}, nil
}

// companiesOf maps ctech-account organization ids to dfe company ids
// (ruling R4). Empty after the organizations items are gone: done.
func (s *ErasureService) companiesOf(ctx context.Context, orgIDs []string) ([]string, error) {
	var out []string
	for _, id := range orgIDs {
		var start map[string]types.AttributeValue
		for {
			page, err := s.repo.QueryIndex(ctx, tblOrganizations, organizationIDIndex, repositories.AttrOrganizationID, id, start)
			if err != nil {
				return nil, err
			}
			for _, it := range page.Items {
				out = append(out, avAttr(it, "pk"))
			}
			if page.Next == nil {
				break
			}
			start = page.Next
		}
	}
	return out, nil
}

// lockCompany writes ORG#{company} locked (workers drop its jobs from then on)
// and reports whether the lock is older than the settle window.
func (s *ErasureService) lockCompany(ctx context.Context, company string, m erasure.Message) (bool, error) {
	key := erasure.OrgKey(company)
	rec, err := s.store.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if rec == nil || rec.State == erasure.StateActive {
		lock := m
		lock.Type = erasure.TypeLocked
		applied, err := s.store.Apply(ctx, key, lock)
		if err != nil {
			return false, err
		}
		rec = &applied
	}
	switch rec.State {
	case erasure.StateErased:
		return true, nil
	case erasure.StateLocked:
		at, err := time.Parse(time.RFC3339, rec.UpdatedAt)
		if err != nil {
			return false, fmt.Errorf("erasure: company %s lock time: %w", company, err)
		}
		return s.now().Sub(at) >= s.settle, nil
	}
	return false, fmt.Errorf("erasure: company %s could not be locked", company)
}

// purgePerson implements inventory §5's person table.
func (s *ErasureService) purgePerson(ctx context.Context, m erasure.Message, counts map[string]int) error {
	raw := repositories.RawUserID(m.Sub)
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblOrgUsers, userIndex, skKey, repositories.BuildMemberSK(raw), start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.leaveOrganization(ctx, avAttr(it, "pk"), raw, counts); err != nil {
				return err
			}
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	if err := s.anonymizeActor(ctx, raw, counts); err != nil {
		return err
	}
	if err := s.purgeECPF(ctx, m.RequestID, counts); err != nil {
		return err
	}
	if err := s.eraseBilling(ctx, raw, counts); err != nil {
		return err
	}
	existed, err := s.repo.DeleteKey(ctx, tblUsers, pkKey(repositories.BuildUserPK(raw)))
	if err != nil {
		return err
	}
	if existed {
		counts[tblUsers]++
	}
	for _, k := range []string{userItemCacheKey(raw), userMeCacheKey(raw), userOrgsCacheKey(raw), accountBillingCacheKey(raw)} {
		cacheDelete(ctx, s.cache, k)
	}
	return nil
}

func anonymousActor(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "erased:" + hex.EncodeToString(sum[:8])
}

func sameUser(stored, raw string) bool {
	return stored != "" && repositories.RawUserID(stored) == raw
}

// leaveOrganization removes the person from a surviving organization: the
// invitations they received go, the ones they sent lose the inviter, the
// member audit rows lose their payload, and the membership row goes.
func (s *ErasureService) leaveOrganization(ctx context.Context, org, raw string, counts map[string]int) error {
	anon := map[string]types.AttributeValue{
		"invited_by": repositories.SAV(anonymousActor(raw)), "invited_by_name": repositories.SAV(erasedActorName),
	}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblInvitations, orgInviteIndex, "org_pk", org, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			switch {
			case sameUser(avAttr(it, "accepted_by"), raw):
				existed, err := s.repo.DeleteKey(ctx, tblInvitations, rowKey(it, ""))
				if err != nil {
					return err
				}
				if existed {
					counts[tblInvitations]++
				}
			case sameUser(avAttr(it, "invited_by"), raw):
				if err := s.repo.SetAttrs(ctx, tblInvitations, rowKey(it, ""), anon); err != nil {
					return err
				}
				counts["organization_invitations_anonymized"]++
			}
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	memberSK := repositories.BuildMemberSK(raw)
	for _, id := range []string{raw, memberSK} { // both spellings are in use as resource_id
		if err := s.scrubAudit(ctx, org, repositories.AuditResourceMember+"#"+id+"#", counts); err != nil {
			return err
		}
	}
	existed, err := s.repo.DeleteKey(ctx, tblOrgUsers, map[string]types.AttributeValue{
		"pk": repositories.SAV(org), skKey: repositories.SAV(memberSK),
	})
	if err != nil {
		return err
	}
	if existed {
		counts[tblOrgUsers]++
	}
	cacheDelete(ctx, s.cache, memberCacheKey(org, raw))
	return nil
}

// scrubAudit empties modifications on audit rows of one resource (they may
// carry the person's name or an e-CPF alias with the CPF).
func (s *ErasureService) scrubAudit(ctx context.Context, org, skPrefix string, counts map[string]int) error {
	empty := map[string]types.AttributeValue{"modifications": &types.AttributeValueMemberL{Value: []types.AttributeValue{}}}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPrefix(ctx, tblAuditLogs, org, skPrefix, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.repo.SetAttrs(ctx, tblAuditLogs, rowKey(it, skKey), empty); err != nil {
				return err
			}
			counts["audit_logs_scrubbed"]++
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// anonymizeActor replaces the person on every audit row they wrote, keeping
// the organization's trail (inventory §5).
func (s *ErasureService) anonymizeActor(ctx context.Context, raw string, counts map[string]int) error {
	attrs := map[string]types.AttributeValue{
		"user_id": repositories.SAV(anonymousActor(raw)), "user_name": repositories.SAV(erasedActorName),
	}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblAuditLogs, userIDIndex, "user_id", raw, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.repo.SetAttrs(ctx, tblAuditLogs, rowKey(it, skKey), attrs); err != nil {
				return err
			}
			counts["audit_logs_anonymized"]++
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// purgeECPF erases the person's e-CPF certificates everywhere (D14). Pass 1
// marks matching rows while the PFX can still be opened (a branch row shares
// its matriz's object); pass 2 erases marked rows. The marker also lets a
// rerun finish a row whose PFX is already gone.
func (s *ErasureService) purgeECPF(ctx context.Context, requestID string, counts map[string]int) error {
	if s.subject == nil {
		return errors.New("erasure: ctech-account subject lookup is not configured")
	}
	cpf, err := s.subject(ctx, requestID)
	if err != nil {
		return fmt.Errorf("erasure: subject lookup: %w", err)
	}
	marker := map[string]types.AttributeValue{attrErasureRequest: repositories.SAV(requestID)}
	if err := s.scanCertificates(ctx, func(item map[string]types.AttributeValue) error {
		if avAttr(item, attrErasureRequest) == requestID {
			return nil
		}
		held, err := s.heldBy(ctx, item, cpf)
		if err != nil || !held {
			return err
		}
		return s.repo.SetAttrs(ctx, tblCertificates, rowKey(item, skKey), marker)
	}); err != nil {
		return err
	}
	return s.scanCertificates(ctx, func(item map[string]types.AttributeValue) error {
		if avAttr(item, attrErasureRequest) != requestID {
			return nil
		}
		return s.eraseCertificate(ctx, item, counts)
	})
}

func (s *ErasureService) scanCertificates(ctx context.Context, fn func(map[string]types.AttributeValue) error) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.ScanPage(ctx, tblCertificates, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := fn(it); err != nil {
				return err
			}
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

func (s *ErasureService) heldBy(ctx context.Context, item map[string]types.AttributeValue, cpf string) (bool, error) {
	key := avAttr(item, "s3_key")
	if cpf == "" || key == "" {
		return false, nil
	}
	out, err := s.objects.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.certsBucket), Key: aws.String(key)})
	if err != nil {
		var missing *s3types.NoSuchKey
		if errors.As(err, &missing) {
			return false, nil
		}
		return false, fmt.Errorf("erasure: read certificate %s: %w", key, err)
	}
	defer closeReadCloser(ctx, out.Body, "erasure certificate read")
	pfx, err := io.ReadAll(out.Body)
	if err != nil {
		return false, fmt.Errorf("erasure: read certificate %s: %w", key, err)
	}
	return certificateHeldBy(pfx, avAttr(item, "password"), cpf), nil
}

// eraseCertificate deletes every version of the PFX, scrubs the certificate's
// audit history, then deletes the row (and its password) together with the
// audit notice the organization's admins see (ruling R10).
func (s *ErasureService) eraseCertificate(ctx context.Context, item map[string]types.AttributeValue, counts map[string]int) error {
	org, md5 := avAttr(item, "pk"), avAttr(item, "md5")
	n, err := purgeObjects(ctx, s.objects, s.certsBucket, avAttr(item, "s3_key"), true)
	if err != nil {
		return err
	}
	counts[countS3Versions] += n
	if err := s.scrubAudit(ctx, org, repositories.AuditResourceCertificate+"#"+md5+"#", counts); err != nil {
		return err
	}
	auditTx, err := s.audit.BuildLogTxItem(org, repositories.AuditResourceCertificate, md5,
		repositories.AuditActionDelete, systemActorID, systemErasureActorName, nil)
	if err != nil {
		return err
	}
	if err := s.certs.TransactWrite(ctx, []types.TransactWriteItem{s.certs.BuildDeleteTxItem(org, md5), auditTx}); err != nil {
		return err
	}
	counts["organization_certificates_ecpf"]++
	return nil
}

// eraseBilling removes the person's billing snapshot, quota guard and current
// usage counters (ruling R15).
func (s *ErasureService) eraseBilling(ctx context.Context, raw string, counts map[string]int) error {
	snap, err := s.repo.GetItem(ctx, tblAccountBilling, pkKey(repositories.AccountBillingPK(raw)))
	if err != nil {
		return err
	}
	keys := []string{
		repositories.AccountBillingPK(raw),
		repositories.QuotaGuardPK(raw, MeterCompanies),
		repositories.UsageCounterPK(raw, s.now().Format(calendarMonthLayout)),
	}
	if p := avAttr(snap, "period_start"); p != "" {
		keys = append(keys, repositories.UsageCounterPK(raw, p))
	}
	for _, k := range keys {
		existed, err := s.repo.DeleteKey(ctx, tblAccountBilling, pkKey(k))
		if err != nil {
			return err
		}
		if existed {
			counts[tblAccountBilling]++
		}
	}
	return nil
}
```

- [ ] **Step 4: Pass.** `make test-integration 2>&1 | grep -E 'Purge|^ok|FAIL'` → all PASS, `ok`. `go vet ./... && go test ./... -race` → `ok`.
- [ ] **Step 5: Commit.** `git add internal/services/erasure_person.go tests/integration/erasure_test.go && git commit -m "feat(api): person purge, e-CPF erasure and settle-window orchestration"`

---
### Task 10: ctech-account subject client

**Files:**
- Create: `api/internal/accountclient/erasure.go`, `api/internal/accountclient/erasure_test.go`

**Interfaces:**
- Produces: `const accountclient.ScopeErasureSubject = "internal:account:erasure-subject"`, `const accountclient.ScopeErasureAck = "internal:account:erasure-ack"`.
- Produces: `func accountclient.NewSubject(cfg Config) *SubjectClient` (nil when the credential is incomplete), `func (c *SubjectClient) CPF(ctx context.Context, requestID string) (string, error)` — matches `services.SubjectLookup`.

- [ ] **Step 1: Failing test.** `api/internal/accountclient/erasure_test.go`:

```go
package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubjectCPF(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"cpf":"12345678909"}`))
	}))
	t.Cleanup(srv.Close)
	c := &SubjectClient{http: srv.Client(), baseURL: srv.URL}

	cpf, err := c.cpfWithToken(context.Background(), "tok", "req-1")
	if err != nil || cpf != "12345678909" {
		t.Fatalf("cpf %q, err %v", cpf, err)
	}
	if gotPath != "/v1.0/internal/erasure/req-1/subject" || gotAuth != "Bearer tok" {
		t.Fatalf("path %q auth %q", gotPath, gotAuth)
	}
}

// Outside PURGING ctech-account answers 404. That is an error (the message is
// redelivered), and no error text may carry a response body.
func TestSubjectCPF_NotPurgingIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"12345678909"}`))
	}))
	t.Cleanup(srv.Close)
	c := &SubjectClient{http: srv.Client(), baseURL: srv.URL}
	_, err := c.cpfWithToken(context.Background(), "tok", "req-1")
	if err == nil || strings.Contains(err.Error(), "12345678909") {
		t.Fatalf("err = %v; want an error that carries no body", err)
	}
}

func TestSubjectNilClientRefuses(t *testing.T) {
	var c *SubjectClient
	if _, err := c.CPF(context.Background(), "req-1"); err == nil {
		t.Fatal("an unconfigured subject client must fail, not answer an empty CPF")
	}
	if NewSubject(Config{BaseURL: "https://a", TokenURL: "https://a/t", ClientID: "dfe"}) != nil {
		t.Fatal("built without a secret")
	}
}
```

- [ ] **Step 2: Fail.** `go test ./internal/accountclient/ -run Subject -count=1` → build FAIL.
- [ ] **Step 3: Implement.** `api/internal/accountclient/erasure.go`:

```go
package accountclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.aoctech.app/api-commons/oauth2client"
)

const (
	// ScopeErasureSubject reads the CPF of a user being purged, to find their
	// e-CPF certificates (saga protocol §4.4). Granted to dfe only.
	ScopeErasureSubject = "internal:account:erasure-subject"
	// ScopeErasureAck acks a finished user.erase (saga protocol §3).
	ScopeErasureAck = "internal:account:erasure-ack"
)

// SubjectClient asks ctech-account for the CPF of a purging request. Its own
// token manager, so the token carries this one scope (plan ruling R11).
type SubjectClient struct {
	http    *http.Client
	tokens  *oauth2client.TokenManager
	baseURL string
}

// NewSubject builds the client, or nil when the credential is incomplete.
func NewSubject(cfg Config) *SubjectClient {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	httpClient := &http.Client{Timeout: requestTimeout}
	return &SubjectClient{
		http:    httpClient,
		tokens:  oauth2client.New(httpClient, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, ScopeErasureSubject),
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

// CPF returns the CPF of the request's user ("" when the account has none).
// The value is personal data: callers keep it in memory and never log it.
func (c *SubjectClient) CPF(ctx context.Context, requestID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("ctech-account erasure subject client is not configured")
	}
	token, err := c.tokens.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("minting a service token: %w", err)
	}
	return c.cpfWithToken(ctx, token, requestID)
}

func (c *SubjectClient) cpfWithToken(ctx context.Context, token, requestID string) (string, error) {
	u := fmt.Sprintf("%s/v1.0/internal/erasure/%s/subject", c.baseURL, url.PathEscape(requestID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("building the subject request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Never the body: it may echo personal data into a log line.
		return "", fmt.Errorf("ctech-account answered %d for erasure subject %s", resp.StatusCode, requestID)
	}
	var out struct {
		CPF string `json:"cpf"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding the erasure subject")
	}
	return out.CPF, nil
}
```

- [ ] **Step 4: Pass.** `go test ./internal/accountclient/ -count=1 -race` → `ok`.
- [ ] **Step 5: Commit.** `git add internal/accountclient/erasure.go internal/accountclient/erasure_test.go && git commit -m "feat(api): ctech-account erasure subject client"`

---

### Task 11: Configuration, wiring and the consumer goroutine

**Files:**
- Modify: `api/internal/config/config.go`, `api/internal/app/app.go`, `api/internal/api/v1/router.go`, `api/.env.example` (if present; `ls api/.env.example`)
- Create: `api/internal/app/erasure_wiring_test.go`

**Interfaces:**
- Consumes: Tasks 2, 8, 9, 10.
- Produces: env `ERASURE_QUEUE_URL` (`Config.ErasureQueueURL`); `apiv1.Services.ErasureBlocked services.BlockedFunc`; app functions `newErasureStore`, `newErasureService`, `startErasureConsumer`, `blockedOf`.

- [ ] **Step 1: Failing test.** `api/internal/app/erasure_wiring_test.go`:

```go
package app

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.uber.org/fx"

	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Dark launch: without ERASURE_QUEUE_URL nothing erasure-related runs, not
// even the write lock (its table may not exist yet).
func TestErasureIsOffWithoutAQueue(t *testing.T) {
	if s := newErasureStore(&awsclient.Clients{}, &config.Config{}); s != nil {
		t.Fatal("store built without ERASURE_QUEUE_URL")
	}
	if blockedOf(nil) != nil {
		t.Fatal("a nil store must leave the lock off")
	}
	if svc := newErasureService(nil, nil, nil, nil, nil, &awsclient.Clients{}, &config.Config{}, nil); svc != nil {
		t.Fatal("purge service built without a store")
	}
}

func TestErasureIsOnWithAQueue(t *testing.T) {
	clients := &awsclient.Clients{DynamoDB: dynamodb.New(dynamodb.Options{Region: "us-east-1"})}
	s := newErasureStore(clients, &config.Config{ErasureQueueURL: "https://sqs.example/q", TablePrefix: "dev_dfe"})
	if s == nil || blockedOf(s) == nil {
		t.Fatal("store or lock missing with ERASURE_QUEUE_URL set")
	}
}

func TestModuleGraphResolves(t *testing.T) {
	if err := fx.ValidateApp(Module); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Fail.** `go test ./internal/app/ -count=1` → build FAIL.
- [ ] **Step 3: Implement.**

`config.go`, in the `// SQS` group:

```go
	// ErasureQueueURL is the account-deletion queue subscribed to ctech-account's
	// user-erasure topic. Empty turns the whole participant off (lock, purge,
	// consumer): the dark-launch switch.
	ErasureQueueURL string `env:"ERASURE_QUEUE_URL"`
```

`app.go`:
- imports: `net/http`, `strings`, `gopkg.aoctech.app/api-commons/erasure`, `gopkg.aoctech.app/api-commons/oauth2client`.
- `fx.Provide`: add `repositories.NewErasureRepository,` (Repositories group), `newErasureStore,` (Infrastructure group), `newErasureService,` (Services group).
- `fx.Invoke(startErasureConsumer),` after `fx.Invoke(startResultsConsumer),`.
- Replace `newUserService` with:

```go
func newUserService(repo *repositories.UserRepository, c cache.Backend, cfg *config.Config, orgSvc *services.OrganizationService, memberSvc *services.MembershipService, store *erasure.Store) *services.UserService {
	svc := services.NewUserService(repo, c, cfg.CtechURL, orgSvc, memberSvc)
	if blocked := blockedOf(store); blocked != nil {
		svc = svc.WithErasureLock(blocked)
	}
	return svc
}
```

- In the app `Services` struct add `ErasureStore *erasure.Store`; in `registerRoutes` add `ErasureBlocked: blockedOf(svcs.ErasureStore),` to the `apiv1.Services{...}` literal.
- Add:

```go
const erasureAckPath = "/v1.0/internal/erasure/ack"

// newErasureStore binds {prefix}_erasure_state, or nil (participant off) when
// ERASURE_QUEUE_URL is unset. Tombstones are kept forever (erasedTTL 0): dfe
// retains nothing, and the record is what rejects a late message.
func newErasureStore(clients *awsclient.Clients, cfg *config.Config) *erasure.Store {
	if cfg.ErasureQueueURL == "" {
		slog.Warn("ERASURE_QUEUE_URL is unset: the account-deletion lock and purge are OFF")
		return nil
	}
	return erasure.NewStore(clients.DynamoDB, cfg.TablePrefix, 0)
}

func blockedOf(store *erasure.Store) services.BlockedFunc {
	if store == nil {
		return nil
	}
	return store.Blocked
}

func newErasureService(
	repo *repositories.ErasureRepository,
	certs *repositories.CertificateRepository,
	audit *repositories.AuditLogRepository,
	claims *repositories.SerieClaimRepository,
	store *erasure.Store,
	clients *awsclient.Clients,
	cfg *config.Config,
	c cache.Backend,
) *services.ErasureService {
	if store == nil {
		return nil
	}
	subject := accountclient.NewSubject(accountclient.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.AccountClientID,
		ClientSecret: cfg.AccountClientSecret,
		Cache:        c,
	})
	return services.NewErasureService(repo, certs, audit, claims, store, clients.S3,
		cfg.S3BucketDocuments, cfg.S3BucketCerts, subject.CPF, c)
}

// startErasureConsumer runs ctech-go-common's erasure.Consumer in the
// background, like the results consumer (plan ruling R1). It needs the
// ctech-account credential to ack; without it the purge cannot finish, so it
// does not start and says so.
func startErasureConsumer(lc fx.Lifecycle, cfg *config.Config, clients *awsclient.Clients, store *erasure.Store, svc *services.ErasureService, c cache.Backend) {
	if svc == nil {
		return
	}
	if cfg.AccountClientID == "" || cfg.AccountClientSecret == "" {
		slog.Error("erasure consumer NOT started: ACCOUNT_CLIENT_ID/ACCOUNT_CLIENT_SECRET are unset, so acks cannot be sent")
		return
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	tokens := oauth2client.New(httpClient, c, billingclient.TokenURLFor(cfg.CtechURL),
		cfg.AccountClientID, cfg.AccountClientSecret, accountclient.ScopeErasureAck)
	acks := erasure.NewAckClient(httpClient, strings.TrimSuffix(cfg.CtechURL, "/")+erasureAckPath, tokens)
	consumer := erasure.NewConsumer(clients.SQS, cfg.ErasureQueueURL, services.ErasureServiceName, store, svc.Purge, acks)
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() { _ = consumer.Run(ctx) }()
			slog.Info("erasure consumer started", "queue", cfg.ErasureQueueURL)
			return nil
		},
		OnStop: func(context.Context) error {
			cancel()
			return nil
		},
	})
}
```

`api/internal/api/v1/router.go`: add to `Services`:

```go
	// ErasureBlocked is the account-deletion write lock; nil leaves it off.
	ErasureBlocked services.BlockedFunc
```

and in `Register`, right after `verifier := middleware.NewVerifier(...)`:

```go
	if svcs.ErasureBlocked != nil {
		verifier.WithLockCheck(svcs.ErasureBlocked)
	}
```

If `api/.env.example` exists, add `# ERASURE_QUEUE_URL=   # account-deletion queue; empty = participant off`.

- [ ] **Step 4: Pass.** `go build ./... && go vet ./... && go test ./... -race` → `ok` (includes `TestModuleGraphResolves`).
- [ ] **Step 5: Commit.** `git add internal/config internal/app internal/api/v1/router.go $(ls .env.example 2>/dev/null) && git commit -m "feat(api): wire the account-deletion consumer, lock and purge"`

---

### Task 12: XML export endpoint

**Files:**
- Create: `api/internal/services/xml_export.go`, `api/internal/api/v1/xml_export.go`, `api/tests/integration/xml_export_test.go`
- Modify: `api/internal/app/app.go`, `api/internal/api/v1/router.go`, `api/internal/api/v1/openapi/documents.yaml`

**Interfaces:**
- Produces: `func services.NewXMLExportService(docs map[string]*repositories.DocumentRepository, events map[string]*repositories.DocumentEventRepository, objects ObjectStore, bucket string) *XMLExportService`; `func (s *XMLExportService) Page(ctx context.Context, orgPK, docType string, start map[string]types.AttributeValue) ([]byte, map[string]types.AttributeValue, error)`.
- Produces: route `GET /v1.0/xml-export/{doc_type}` (`nfe|nfce|cte|mdfe|nfse`, query `cursor`), header `X-Next-Cursor`, OWNER/ADMIN only, `application/zip`.

- [ ] **Step 1: Failing test.** `api/tests/integration/xml_export_test.go`:

```go
//go:build integration

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"sort"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

func TestXMLExport_ZipsProductionDocumentsAndEvents(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	org := "export-" + randomCNPJ()
	putItem(t, "nfes", map[string]any{"pk": "prod#" + org, "sk": "K1", "xml_s3_key": "nfe/prod/CNPJ_x/K1.xml"})
	putItem(t, "nfes", map[string]any{"pk": "hom#" + org, "sk": "K2", "xml_s3_key": "nfe/hom/CNPJ_x/K2.xml"})
	putItem(t, "nfe_events", map[string]any{"pk": "K1", "sk": "E1", "xml_s3_key": "nfe/prod/CNPJ_x/K1_110111_001.xml"})
	objects.Put("docs", "nfe/prod/CNPJ_x/K1.xml", "<nfe/>")
	objects.Put("docs", "nfe/prod/CNPJ_x/K1_110111_001.xml", "<evento/>")
	objects.Put("docs", "nfe/hom/CNPJ_x/K2.xml", "<hom/>")

	nfe := repositories.NewNfeRepository(db, cfg)
	svc := services.NewXMLExportService(
		map[string]*repositories.DocumentRepository{"nfe": &nfe.DocumentRepository},
		map[string]*repositories.DocumentEventRepository{"nfe": repositories.NewDocumentEventRepository(db, cfg, "nfe")},
		objects, "docs")

	raw, next, err := svc.Page(ctx, org, "nfe", nil)
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Errorf("one page expected, got a cursor %v", next)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "K1.xml" || names[1] != "K1_110111_001.xml" {
		t.Fatalf("zip entries %v, want the production document and its event only", names)
	}
	if _, _, err := svc.Page(ctx, org, "boleto", nil); err == nil {
		t.Fatal("an unknown document type must be refused")
	}
}
```

- [ ] **Step 2: Fail.** `make test-integration 2>&1 | grep -E 'XMLExport|undefined|FAIL'` → build FAIL.
- [ ] **Step 3: Implement.** `api/internal/services/xml_export.go`:

```go
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"gopkg.aoctech.app/api-commons/observability"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// xmlExportPageSize bounds one zip: each document costs one or two S3 reads
// plus its events, and the whole page must fit the 10 s write timeout.
const xmlExportPageSize = 100

// XMLExportService zips an organization's production XMLs a page at a time,
// so an organization about to be erased with its only member can keep its
// fiscal documents (overview D4, plan ruling R12).
type XMLExportService struct {
	docs    map[string]*repositories.DocumentRepository
	events  map[string]*repositories.DocumentEventRepository
	objects ObjectStore
	bucket  string
}

func NewXMLExportService(
	docs map[string]*repositories.DocumentRepository,
	events map[string]*repositories.DocumentEventRepository,
	objects ObjectStore,
	bucket string,
) *XMLExportService {
	return &XMLExportService{docs: docs, events: events, objects: objects, bucket: bucket}
}

// Page returns a zip of up to xmlExportPageSize production documents of
// docType (document XML, DPS XML for NFS-e, and every event XML) and the key
// to continue from (nil on the last page).
func (s *XMLExportService) Page(ctx context.Context, orgPK, docType string, start map[string]types.AttributeValue) ([]byte, map[string]types.AttributeValue, error) {
	docs, events := s.docs[docType], s.events[docType]
	if docs == nil || events == nil {
		return nil, nil, problem.BadRequest("tipo de documento inválido; use nfe, nfce, cte, mdfe ou nfse")
	}
	res, err := docs.List(ctx, EnvProd+"#"+orgPK, repositories.DocumentListOpts{Limit: xmlExportPageSize, StartKey: start})
	if err != nil {
		return nil, nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, doc := range res.Items {
		keys := []string{avAttr(doc, "xml_s3_key"), avAttr(doc, "dps_xml_s3_key")}
		evs, err := events.GetDocumentEvents(ctx, avAttr(doc, "sk"), xmlExportPageSize, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, ev := range evs.Items {
			keys = append(keys, avAttr(ev, "xml_s3_key"))
		}
		for _, key := range keys {
			if key == "" {
				continue
			}
			if err := s.addObject(ctx, zw, key); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, nil, problem.InternalServer("falha ao gerar o zip").WithCause(err)
	}
	return buf.Bytes(), res.LastEvaluatedKey, nil
}

func (s *XMLExportService) addObject(ctx context.Context, zw *zip.Writer, key string) error {
	out, err := s.objects.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		var missing *s3types.NoSuchKey
		if errors.As(err, &missing) {
			observability.Warn(ctx, "xml export: object missing", err, "s3_key", key)
			return nil
		}
		return problem.InternalServer("falha ao ler XML do armazenamento").WithCause(err)
	}
	defer closeReadCloser(ctx, out.Body, "xml export")
	w, err := zw.Create(path.Base(key))
	if err != nil {
		return problem.InternalServer("falha ao gerar o zip").WithCause(err)
	}
	if _, err := io.Copy(w, out.Body); err != nil {
		return problem.InternalServer("falha ao ler XML do armazenamento").WithCause(err)
	}
	return nil
}
```

`api/internal/api/v1/xml_export.go`:

```go
package v1

import (
	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// HeaderNextCursor carries the cursor of the next export page.
const HeaderNextCursor = "X-Next-Cursor"

const contentTypeZip = "application/zip"

// RegisterXMLExport mounts the bulk XML export (OWNER/ADMIN; org via header).
func RegisterXMLExport(router fiber.Router, svc *services.XMLExportService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/xml-export", authMw, perm.RequireOwnerOrAdmin())
	g.Get("/:doc_type", func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		zipped, next, err := svc.Page(c.Context(), middleware.GetOrgPK(c), c.Params("doc_type"), decodeCursor(cursor))
		if err != nil {
			return sendProblem(c, err)
		}
		if n := buildNextCursor(next, cursor); n != nil {
			c.Set(HeaderNextCursor, *n)
		}
		return sendAttachment(c, zipped, contentTypeZip, c.Params("doc_type")+"-xml", ".zip")
	})
}
```

`router.go`: add `XMLExport *services.XMLExportService` to `Services`; after `RegisterErasure(v1, authMw)` add `RegisterXMLExport(v1, svcs.XMLExport, authMw, perm)`.

`app.go`:
- `fx.Provide`: add `newXMLExportService,` (Services group).
- app `Services` struct: `XMLExport *services.XMLExportService`; `registerRoutes`: `XMLExport: svcs.XMLExport,`.
- CORS: `ExposeHeaders: []string{fiberobs.RequestIDHeader, apiv1.HeaderNextCursor},`.
- Add:

```go
func newXMLExportService(
	nfe *repositories.NfeRepository, nfce *repositories.NfceRepository, cte *repositories.CteRepository,
	mdfe *repositories.MdfeRepository, nfse *repositories.NfseRepository,
	db *dynamodb.Client, clients *awsclient.Clients, cfg *config.Config,
) *services.XMLExportService {
	docs := map[string]*repositories.DocumentRepository{
		"nfe": &nfe.DocumentRepository, "nfce": &nfce.DocumentRepository, "cte": &cte.DocumentRepository,
		"mdfe": &mdfe.DocumentRepository, "nfse": &nfse.DocumentRepository,
	}
	events := make(map[string]*repositories.DocumentEventRepository, len(docs))
	for docType := range docs {
		events[docType] = repositories.NewDocumentEventRepository(db, cfg, docType)
	}
	return services.NewXMLExportService(docs, events, clients.S3, cfg.S3BucketDocuments)
}
```

`openapi/documents.yaml` — append under `paths:`:

```yaml
  /v1.0/xml-export/{doc_type}:
    get:
      tags: [ Organizations ]
      summary: Exporta os XMLs de produção da organização em zip, por página
      description: |
        Até 100 documentos de produção por chamada, com o XML do documento,
        o XML da DPS (NFS-e) e o XML de cada evento. Quando houver mais,
        o cabeçalho `X-Next-Cursor` traz o cursor da próxima página.
        Restrito a OWNER e ADMIN. Usado antes de excluir uma conta cuja
        organização será apagada junto.
      operationId: exportXML
      parameters:
        - $ref: '#/components/parameters/OrgHeader'
        - name: doc_type
          in: path
          required: true
          schema: { type: string, enum: [ nfe, nfce, cte, mdfe, nfse ] }
        - $ref: '#/components/parameters/Cursor'
      responses:
        '200':
          description: Zip com os XMLs da página
          headers:
            X-Next-Cursor:
              description: Cursor da próxima página; ausente na última.
              schema: { type: string }
          content:
            application/zip:
              schema: { type: string, format: binary }
        '400': { $ref: '#/components/responses/BadRequest' }
        '401': { $ref: '#/components/responses/Unauthorized' }
        '403': { $ref: '#/components/responses/Forbidden' }
```

- [ ] **Step 4: Pass.** `make test-integration 2>&1 | grep -E 'XMLExport|^ok|FAIL'` → PASS. `go vet ./... && go test ./... -race` → `ok` (OpenAPI route test included). `make openapi-lint` → no errors.
- [ ] **Step 5: Commit.** `git add internal tests/integration/xml_export_test.go && git commit -m "feat(api): paged XML export before account deletion"`

---

### Task 13: Workers drop jobs of a locked or erased company

**Files:**
- Create: `worker/internal/service/erasure.go`, `worker/internal/service/erasure_test.go`
- Modify: `worker/go.mod`, `worker/go.sum`, `worker/internal/service/dfe.go`, `worker/internal/service/distribution.go`, `worker/cmd/worker/main.go`, `worker/cmd/distribution-worker/main.go`

**Interfaces:**
- Produces: `type service.OrgStopped func(ctx context.Context, companyID string) (bool, error)`, `func service.NewOrgStopped(store *erasure.Store) OrgStopped`, `func (s *DfeService) WithOrgStopped(f OrgStopped) *DfeService`, `func (s *DistributionService) WithOrgStopped(f OrgStopped) *DistributionService`, `func companyOfDocPK(docPK string) string`.

- [ ] **Step 1: Dependency.** `cd worker && go get gopkg.aoctech.app/api-commons@v1.13.1 && go mod tidy && go build ./... && go test ./... 2>&1 | tail -5` → all `ok` (the AWS SDK minor bumps come from api-commons; the suite gates them).
- [ ] **Step 2: Failing tests.** `worker/internal/service/erasure_test.go`:

```go
package service

import (
	"context"
	"errors"
	"testing"
)

func stoppedFor(company string) OrgStopped {
	return func(_ context.Context, c string) (bool, error) { return c == company, nil }
}

func TestCompanyOfDocPK(t *testing.T) {
	if got := companyOfDocPK("prod#0190-company"); got != "0190-company" {
		t.Fatalf("got %q", got)
	}
	if got := companyOfDocPK("no-env"); got != "" {
		t.Fatalf("got %q, want empty for a malformed key", got)
	}
}

func TestProcess_DropsJobOfStoppedCompany(t *testing.T) {
	s3m, dynm := certS3(), &mockDynamo{}
	svc := newSvc(s3m, &mockLambda{payload: invokeResp("100", "Autorizado", "1")}, dynm).
		WithOrgStopped(stoppedFor("CNPJ_12345678000195"))
	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("a dropped job must not fail (no retries, no DLQ): %v", err)
	}
	if dynm.claimCalls != 0 || len(s3m.putCalls) != 0 {
		t.Fatalf("an erased company's job touched DynamoDB (%d) or S3 (%d)", dynm.claimCalls, len(s3m.putCalls))
	}
}

func TestProcess_StoppedCheckFailureRetries(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{}, dynm).
		WithOrgStopped(func(context.Context, string) (bool, error) { return false, errors.New("dynamodb down") })
	if err := svc.Process(context.Background(), baseMsg); err == nil || dynm.claimCalls != 0 {
		t.Fatal("an unknown erasure state must fail closed before the claim")
	}
}

func TestDistribution_DropsJobOfStoppedCompany(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, testCfg).WithOrgStopped(stoppedFor("C1"))
	if err := svc.Process(context.Background(), DistributionMessage{OrgPK: "C1", DocType: "nfe"}); err != nil {
		t.Fatal(err)
	}
	if dynm.getIdx != 0 || dynm.queryIdx != 0 || len(dynm.updateCalls) != 0 || len(dynm.putCalls) != 0 {
		t.Fatal("an erased company's distribution job touched DynamoDB")
	}
}
```

- [ ] **Step 3: Fail.** `go test ./internal/service/ -run 'Stopped|CompanyOfDocPK' -count=1` → build FAIL.
- [ ] **Step 4: Implement.** `worker/internal/service/erasure.go`:

```go
package service

import (
	"context"
	"strings"

	"gopkg.aoctech.app/api-commons/erasure"
)

// OrgStopped reports whether the account-deletion saga locked or erased a
// company (ORG#{company_id} in {prefix}_erasure_state). Its jobs are dropped:
// running them would write rows the purge is deleting (saga protocol §4.5).
type OrgStopped func(ctx context.Context, companyID string) (bool, error)

func NewOrgStopped(store *erasure.Store) OrgStopped {
	return func(ctx context.Context, companyID string) (bool, error) {
		rec, err := store.Get(ctx, erasure.OrgKey(companyID))
		if err != nil {
			return false, err
		}
		return rec != nil && rec.State != erasure.StateActive, nil
	}
}

// companyOfDocPK is the company half of a {env}#{company} document key.
func companyOfDocPK(docPK string) string {
	_, company, _ := strings.Cut(docPK, "#")
	return company
}

func (f OrgStopped) check(ctx context.Context, companyID string) (bool, error) {
	if f == nil || companyID == "" {
		return false, nil
	}
	return f(ctx, companyID)
}
```

`dfe.go`: add field `orgStopped OrgStopped` to `DfeService`, and:

```go
// WithOrgStopped makes Process drop jobs of companies the deletion saga stopped.
func (s *DfeService) WithOrgStopped(f OrgStopped) *DfeService {
	s.orgStopped = f
	return s
}
```

At the top of `Process`, after the `slog.Info("processing dfe", ...)` call:

```go
	if stopped, err := s.orgStopped.check(ctx, companyOfDocPK(msg.DocPK)); err != nil {
		return fmt.Errorf("erasure state: %w", err)
	} else if stopped {
		slog.Info("dropping job of a company in account deletion", "doc_pk", msg.DocPK, "access_key", msg.AccessKey)
		return nil
	}
```

`distribution.go`: add field `orgStopped OrgStopped` to `DistributionService`, the same `WithOrgStopped` method on `*DistributionService`, and at the top of `Process`:

```go
	if stopped, err := s.orgStopped.check(ctx, msg.OrgPK); err != nil {
		return fmt.Errorf("erasure state: %w", err)
	} else if stopped {
		slog.Info("dropping distribution job of a company in account deletion", "org_pk", msg.OrgPK, "job_type", msg.JobType)
		return nil
	}
```

`cmd/worker/main.go`: import `gopkg.aoctech.app/api-commons/erasure` and replace the `svc := service.New(...)` statement with:

```go
	dyn := dynamodb.NewFromConfig(ac)
	stopped := service.NewOrgStopped(erasure.NewStore(dyn, cfg.TablePrefix, 0))
	svc := service.New(service.Clients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
		Dynamo: dyn,
		SNS:    sns.NewFromConfig(ac),
	}, cfg).WithOrgStopped(stopped)
```

`cmd/distribution-worker/main.go`: same import, and replace its `svc := service.NewDistribution(...)` statement with:

```go
	dyn := dynamodb.NewFromConfig(ac)
	stopped := service.NewOrgStopped(erasure.NewStore(dyn, cfg.TablePrefix, 0))
	svc := service.NewDistribution(service.DistributionClients{
		S3:     s3.NewFromConfig(ac),
		Lambda: lambdaSDK.NewFromConfig(ac, func(o *lambdaSDK.Options) { o.Region = cfg.DfeEgressRegion }),
		Dynamo: dyn,
		SNS:    sns.NewFromConfig(ac),
	}, cfg).WithOrgStopped(stopped)
```

The table is created by the DynamoDB stack, which CDK deploys before the worker stack (the worker stack already depends on it through `worker_outbox`).

- [ ] **Step 5: Pass.** `go vet ./... && go test ./... -count=1` → `ok`. `GOOS=linux GOARCH=arm64 go build ./cmd/...` → no output.
- [ ] **Step 6: Commit.** `git add worker && git commit -m "feat(worker): drop jobs of companies in account deletion"` (from repo root).

---
### Task 14: Infrastructure (CDK)

**Files:**
- Modify: `cdk/lib/dynamodb-stack.ts`, `cdk/lib/event-bus-stack.ts`, `cdk/lib/iam-stack.ts`, `cdk/lib/worker-stack.ts`, `cdk/lib/api-stack.ts`, `cdk/bin/ctech-dfe-cdk.ts`
- Test: `cdk/test/dynamodb-stack.test.ts`, `cdk/test/event-bus-stack.test.ts` (new), `cdk/test/api-stack.test.ts`

**Interfaces:**
- Produces: table `{env}_dfe_erasure_state` (pk S, TTL `ttl`); GSI `organization-id-index` (pk `organization_id`, KEYS_ONLY) on `{env}_dfe_organizations`; queues `{env}-ctech-dfe-erasure` + `{env}-ctech-dfe-erasure-dlq`; `EventBusStack.erasureQueueUrl/erasureQueueArn`; `IAMStackProps.erasureQueueArn`; `ApiStackProps.erasureQueueUrl`; API env `ERASURE_QUEUE_URL`.

- [ ] **Step 1: Failing tests.**

Append to `cdk/test/dynamodb-stack.test.ts`:

```ts
describe('DynamoDBStack — exclusão de conta', () => {
    test('erasure_state: só pk, com TTL em ttl', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_erasure_state',
            KeySchema: [{AttributeName: 'pk', KeyType: 'HASH'}],
            TimeToLiveSpecification: {AttributeName: 'ttl', Enabled: true},
        });
    });

    test('organizations tem o GSI organization-id-index', () => {
        synth().hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_organizations',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'organization-id-index',
                    KeySchema: [{AttributeName: 'organization_id', KeyType: 'HASH'}],
                    Projection: {ProjectionType: 'KEYS_ONLY'},
                }),
            ]),
        });
    });
});
```

Create `cdk/test/event-bus-stack.test.ts`:

```ts
import * as cdk from 'aws-cdk-lib';
import {Match, Template} from 'aws-cdk-lib/assertions';
import {EventBusStack} from '../lib/event-bus-stack';

const synth = () => Template.fromStack(new EventBusStack(new cdk.App(), 'TestEventBus', {
    env: {account: '868899309401', region: 'us-east-1'},
    environment: 'dev',
}));

describe('EventBusStack — fila da exclusão de conta', () => {
    test('fila com DLQ, visibilidade de 30 min e 5 recebimentos', () => {
        synth().hasResourceProperties('AWS::SQS::Queue', {
            QueueName: 'dev-ctech-dfe-erasure',
            VisibilityTimeout: 1800,
            RedrivePolicy: Match.objectLike({maxReceiveCount: 5}),
        });
        synth().hasResourceProperties('AWS::SQS::Queue', {QueueName: 'dev-ctech-dfe-erasure-dlq'});
    });

    test('assina o tópico do ctech-account filtrando services=dfe no atributo, entrega raw', () => {
        const subs = synth().findResources('AWS::SNS::Subscription', {
            Properties: {Protocol: 'sqs', FilterPolicy: {services: ['dfe']}},
        });
        const props = Object.values(subs).map((r: any) => r.Properties);
        expect(props).toHaveLength(1);
        expect(props[0].RawMessageDelivery).toBe(true);
        expect(props[0].FilterPolicyScope ?? 'MessageAttributes').toBe('MessageAttributes');
    });
});
```

In `cdk/test/api-stack.test.ts`, add `erasureQueueUrl: 'https://sqs.us-east-1.amazonaws.com/868899309401/prod-ctech-dfe-erasure',` to the `ApiStack` props in `synth()`, and append:

```ts
test('API env carries the account-deletion queue', () => {
  expect(userDataText(synth())).toContain('ERASURE_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/868899309401/prod-ctech-dfe-erasure')
})
```

- [ ] **Step 2: Fail.** `cd cdk && npx jest 2>&1 | tail -20` → the new tests FAIL (and `api-stack.test.ts` fails to compile on the unknown prop).
- [ ] **Step 3: Implement.**

`dynamodb-stack.ts`, immediately before `this.tables.set('organizations', organizationsTable);` add:

```ts
    // Account deletion (ctech-account saga, D4): an erased ctech-account
    // organization id → its dfe companies. Keys only: the purge needs the pk.
    organizationsTable.addGlobalSecondaryIndex({
      indexName: 'organization-id-index',
      partitionKey: {name: 'organization_id', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.KEYS_ONLY,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
```

and before `// ============== OUTPUTS ==============`:

```ts
    // Account-deletion lock/tombstone state (SUB#{sub}, ORG#{id}), schema owned
    // by ctech-go-common erasure.Store. No PII; erased records have no TTL.
    const erasureStateTable = new dynamodb.TableV2(this, `${tablePrefix}_erasure_state`, {
      tableName: `${tablePrefix}_erasure_state`,
      partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
      billing: Billing.onDemand({
        maxReadRequestUnits: 1000,
        maxWriteRequestUnits: 1000,
      }),
      timeToLiveAttribute: 'ttl',
      removalPolicy,
      pointInTimeRecoverySpecification,
      encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
    });
    this.tables.set('erasure_state', erasureStateTable);
```

`event-bus-stack.ts`: add `import * as ssm from 'aws-cdk-lib/aws-ssm'`, public fields `erasureQueueUrl: string` and `erasureQueueArn: string`, and before the outputs:

```ts
    // Account deletion (ctech-account saga protocol §3): this service's queue
    // on ctech-account's user-erasure topic. Raw delivery; the filter is on the
    // `services` message attribute. Visibility >= 2x the slowest purge (§7).
    const erasureDlq = new sqs.Queue(this, 'ErasureQueue-dlq', {
      queueName: `${environment}-ctech-dfe-erasure-dlq`,
      retentionPeriod: cdk.Duration.days(14),
    })
    const erasureQueue = new sqs.Queue(this, 'ErasureQueue', {
      queueName: `${environment}-ctech-dfe-erasure`,
      visibilityTimeout: cdk.Duration.minutes(30),
      retentionPeriod: cdk.Duration.days(14),
      receiveMessageWaitTime: cdk.Duration.seconds(20),
      deadLetterQueue: {queue: erasureDlq, maxReceiveCount: 5},
    })
    const accountErasureTopic = sns.Topic.fromTopicArn(this, 'AccountUserErasureTopic',
      ssm.StringParameter.valueForStringParameter(this, `/ctech/${environment}/account/erasure-topic-arn`))
    accountErasureTopic.addSubscription(new subs.SqsSubscription(erasureQueue, {
      rawMessageDelivery: true,
      filterPolicy: {services: sns.SubscriptionFilter.stringFilter({allowlist: ['dfe']})},
    }))
    this.erasureQueueUrl = erasureQueue.queueUrl
    this.erasureQueueArn = erasureQueue.queueArn
    new cdk.CfnOutput(this, 'ErasureQueueUrl', {value: this.erasureQueueUrl})
```

`iam-stack.ts`: add `erasureQueueArn: string;` to `IAMStackProps` (next to `distributionQueueArn`), and after the `apiSqsPolicy` statement:

```ts
    // Account deletion: consume the erasure queue, and erase every version of
    // an erased company's objects or of an erased e-CPF (inventory §2).
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['sqs:ReceiveMessage', 'sqs:DeleteMessage', 'sqs:GetQueueAttributes'],
      resources: [props.erasureQueueArn],
    }));
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['s3:ListBucketVersions'],
      resources: [props.certificatesBucketArn, props.documentsBucketArn],
    }));
    this.apiV2Role.addToPrincipalPolicy(new iam.PolicyStatement({
      actions: ['s3:DeleteObject', 's3:DeleteObjectVersion'],
      resources: [`${props.certificatesBucketArn}/*`, `${props.documentsBucketArn}/*`],
    }));
```

(DynamoDB needs no change: `dynamoPolicy` already grants the API role every action on `${tablePrefix}_*`, including Scan and BatchWriteItem.)

`worker-stack.ts`: after the `lambda:InvokeFunction` statement in the per-worker loop:

```ts
      // Drop jobs of companies in account deletion (erasure state, read only).
      role.addToPrincipalPolicy(new iam.PolicyStatement({
        actions: ['dynamodb:GetItem'],
        resources: [`arn:aws:dynamodb:${this.region}:${this.account}:table/${tablePrefix}_erasure_state`],
      }))
```

`api-stack.ts`: add `erasureQueueUrl: string;` to `ApiStackProps`, destructure it, and add `` `ERASURE_QUEUE_URL=${erasureQueueUrl}`, `` after `` `DFE_DISTRIBUTION_QUEUE_URL=${distributionQueueUrl}`, `` in `/etc/app-static.env`.

`bin/ctech-dfe-cdk.ts`: IAMStack props `erasureQueueArn: eventBusStack.erasureQueueArn,`; ApiStack props `erasureQueueUrl: eventBusStack.erasureQueueUrl,`; and `apiV2Stack.addStackDependency(eventBusStack);`.

- [ ] **Step 4: Pass.** `cd cdk && npx tsc --noEmit && npx jest 2>&1 | tail -8` → all suites pass. `ENVIRONMENT=dev npx cdk synth --all --profile ctech > /dev/null` → exit 0.
- [ ] **Step 5: Commit.** `git add cdk && git commit -m "feat(cdk): erasure state table, organization-id GSI, erasure queue and IAM"`

---

### Task 15: Documentation

**Files:**
- Modify: `DOCS.md`, `DynamoDB-Tables.md`, `DEPLOYMENT.md`, `OVERVIEW.md`, `CONDUCT.md`, `api/CLAUDE.md`

- [ ] **Step 1: `DynamoDB-Tables.md`.**
  - Index rows: add `| 46 | erasure_state | SUB#{sub} / ORG#{id} | — | — |`, add the missing `account_billing` (36) and `serie_claims` rows, and add `organization-id-index` to `organizations`.
  - Fix `15–18` and `31`: the events partition key is the document's access key (`id_dps` for NFS-e), plus `INUT#{env}#{org_pk}` for inutilizações. The old `{org_pk}` text is wrong (see `DocumentEventRepository.CreateEvent`).
  - New section `## 46. erasure_state`: attributes `pk`, `erasure_state` (`active|locked|erased`), `request_id`, `seq_ns`, `updated_at`, `ttl`; owner `ctech-go-common/erasure.Store`; `ORG#{company_id}` written by dfe (plan R5), `ORG#{account_org_id}` written by the consumer; no PII.
  - `organizations`: document `organization-id-index` (PK `organization_id`, KEYS_ONLY; access pattern "companies of an erased ctech-account organization").
- [ ] **Step 2: `DOCS.md`.**
  - §4 *Configuration*: `ERASURE_QUEUE_URL` (empty = participant off).
  - §4 *API Reference*: `GET /v1.0/internal/erasure/eligibility/{sub}` (service token, scope, always eligible, R2) and `GET /v1.0/xml-export/{doc_type}` (OWNER/ADMIN, 100 docs/page, `X-Next-Cursor`, production only).
  - New §4 subsection *Exclusão de conta (LGPD)*: the lifecycle seen from dfe (lock → purge → ack), the 403 `account-pending-deletion`, the catalog and its coverage test, the settle window, S3 by row (R7), e-CPF matching and marker, what is anonymized vs kept (R16), ack counts keys.
  - §6 worker: jobs of a locked/erased company are dropped (Task 13).
  - §8 cdk: erasure queue/DLQ/subscription and the new IAM statements.
- [ ] **Step 3: `DEPLOYMENT.md` → *Out-of-band parameters*.** Operator steps: (1) deploy CDK (DynamoDB stack creates `erasure_state` and the GSI before any code reads them); (2) in ctech-account, grant the dfe confidential client (`/ctech-dfe/{env}/account-client-id`) `internal:account:erasure-ack` and `internal:account:erasure-subject`; (3) add dfe to ctech-account's `/ctech-account/{env}/erasure-participants` JSON with `url` = `{dfe internal base}/v1.0`, `audience` = dfe `SERVICE_AUDIENCE`, `client_id` = that client; (4) the queue is created and subscribed by CDK; `ERASURE_QUEUE_URL` turns the participant on. Backup-restore step (saga protocol §8): after restoring any dfe table from PITR, re-publish `user.erase` for every request purged after the restore point (ctech-account README "Account deletion").
- [ ] **Step 4: `OVERVIEW.md`.** Security: add the account-deletion lock and JWT revocation; api: the erasure consumer; data model: `erasure_state`.
- [ ] **Step 5: `CONDUCT.md`.** New section *Exclusão de conta*: a new DynamoDB table must be added to `services/erasure_catalog.go` (the test fails otherwise); a new async consumer of company work must check the company's erasure state (`ORG#{company_id}`) and drop the job; never log or persist the CPF from the subject endpoint; objects are erased by the keys rows store, never by `CNPJ_` folder.
- [ ] **Step 6: `api/CLAUDE.md` → *Known Constraints*.** One line each: write lock in the auth middleware (403 `account-pending-deletion`); revocation entries read from the shared Valkey DB 0; `ERASURE_QUEUE_URL` is the participant switch.
- [ ] **Step 7: Commit.** `git add DOCS.md DynamoDB-Tables.md DEPLOYMENT.md OVERVIEW.md CONDUCT.md api/CLAUDE.md && git commit -m "docs: account deletion participant (lock, purge, export, infra)"`

---

## Open questions

1. **Token issuer:** does the token ctech-account mints for eligibility carry `iss` = dfe's `CTECH_ISSUER_URL` (`/ctech-account/{env}/app-url`)? If not, eligibility is a 401 and every deletion request returns 503 at ctech-account.
2. **Notification of e-CPF removal (R10):** is an audit row enough, or should ctech-account e-mail the surviving organization's admins? That needs an account route dfe can call.
3. **XML export UI (R12):** who ships the dfe UI button and the link from ctech-account's deletion page, and must export also cover homologação documents?
4. **Matriz certificate shared by a surviving branch in another account organization:** erasing the matriz company deletes the PFX the branch row points at. Accept (D4: erase everything of the erased org), or keep the object while any surviving row references it?
5. **Service-scope unlink:** a returning user must get `Store.Clear(sub)` on re-consent (saga §4.5). Out of scope here, as in ctech-account phase 3; needs a plan when service unlink is built.
