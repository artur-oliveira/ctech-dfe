# Audit Logging Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every mutating action (POST/PUT/DELETE) in `api` attributable to a user, via a new `audit_logs` table for
state-changing resources and in-record `user_id`/`user_name` fields for append-only DF-e issuance/events.

**Architecture:** A new `AuditLogRepository` + `AuditService` (diff + actor resolution) are wired into every mutating
resource service. Each resource mutation and its audit row are written in a single `dynamodb.TransactWriteItems` call
(new non-executing `Build*TxItem` companions to `Base.PutItem`/`UpdateItem`/`DeleteItem`), so the two can never diverge.
DF-e issuance/events get two new fields on write — no separate table row, since those records are never mutated in
place.

**Tech Stack:** Go 1.x, Fiber v3, aws-sdk-go-v2 (DynamoDB), go.uber.org/fx, CDK v2 TypeScript, Next.js/TypeScript (UI).

## Global Constraints

- RFC 7807 Problem JSON for all api errors — never raw errors/`fiber.Map` (`api/CLAUDE.md`).
- No DynamoDB scans in production; `GetItem` > `Query` > `Scan` (`api/CLAUDE.md`, `worker/CLAUDE.md`).
- Table names are environment-prefixed via `cfg.TablePrefix`/`Base.NewBase` — never hardcoded (`api/CLAUDE.md`).
- Every core service/repository function needs a unit test; DynamoDB-touching code needs an integration test (
  `api/CLAUDE.md`, `worker/CLAUDE.md`).
- `npx eslint src --ext .ts,.tsx` must pass with zero errors/warnings before any UI commit (`ui/CLAUDE.md`).
- UI: loading states on every async call, 300ms debounce on filter inputs, mobile-first 375px layout (`ui/CLAUDE.md`).
- `RemovalPolicy.DESTROY` only in `dev`; PITR only in staging/prod (`cdk/CLAUDE.md`).
- No unrelated refactors — implement only what this plan specifies (root `CLAUDE.md`).
- Design reference: `docs/superpowers/specs/2026-07-11-audit-logging-design.md`.

---

## Part A — Foundation

### Task 1: CDK — `audit_logs` table + GSIs

**Files:**

- Modify: `cdk/lib/dynamodb-stack.ts:8-31` (add to `TableName` union), `cdk/lib/dynamodb-stack.ts:430-441` (add table +
  GSIs, mirroring the `usersTable`/`personsTable` construction style already in the file)
- Test: `cdk/test/dynamodb-stack.test.ts` (create if it doesn't exist — check first with `ls cdk/test/`)

**Interfaces:**

- Produces: DynamoDB table `${tablePrefix}_audit_logs` with `pk`(S)/`sk`(S) keys, GSI `org-time-index` (pk=`pk`, sk=
  `created_at`), GSI `user-id-index` (pk=`user_id`, sk=`created_at`).

- [ ] **Step 1: Check for an existing CDK snapshot test file**

Run: `ls cdk/test/`
Expected: note whether a `dynamodb-stack.test.ts` (or similar snapshot test) already exists. If it does, you'll add to
it in Step 5; if not, Step 5 creates it.

- [ ] **Step 2: Add `'audit_logs'` to the `TableName` union**

In `cdk/lib/dynamodb-stack.ts`, change:

```typescript
export type TableName = (
    'roles' |
    'users' |
    'organizations' |
```

to:

```typescript
export type TableName = (
    'roles' |
    'users' |
    'organizations' |
    'audit_logs' |
```

(Leave the rest of the union untouched — this only adds one new member.)

- [ ] **Step 3: Add the table construction, right after the `organizationsTable` block (after line 283
  `this.tables.set('organizations', organizationsTable);`)**

```typescript
        // ============== AUDIT ==============

        const auditLogsTable = new dynamodb.TableV2(this, `${tablePrefix}_audit_logs`, {
            tableName: `${tablePrefix}_audit_logs`,
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'sk', type: dynamodb.AttributeType.STRING},
            billing: Billing.onDemand({
                maxReadRequestUnits: 5,
                maxWriteRequestUnits: 5,
            }),
            removalPolicy,
            pointInTimeRecoverySpecification,
            encryption: dynamodb.TableEncryptionV2.awsManagedKey(),
        });
        auditLogsTable.addGlobalSecondaryIndex({
            indexName: 'org-time-index',
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 10,
            maxWriteRequestUnits: 10,
        });
        auditLogsTable.addGlobalSecondaryIndex({
            indexName: 'user-id-index',
            partitionKey: {name: 'user_id', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 10,
            maxWriteRequestUnits: 10,
        });
        this.tables.set('audit_logs', auditLogsTable);
```

- [ ] **Step 4: Verify synth succeeds**

Run: `cd cdk && npm install && cdk synth CtechDfe-Dev-DynamoDB 2>&1 | tail -30` (adjust the stack id if `cdk synth`
without an id lists a different name for the DynamoDB stack — run `cdk synth --list` first if unsure)
Expected: synth completes with no errors, and the printed template contains a resource for `audit_logs`.

- [ ] **Step 5: Add/extend the snapshot test**

If `cdk/test/dynamodb-stack.test.ts` exists, add a case asserting the new table + both GSIs exist (follow the exact
assertion style already used for `personsTable`/`plate-index` in that file). If it doesn't exist, skip creating one
now — CDK snapshot testing conventions in this repo should be confirmed with the user rather than invented; note this in
the task's completion comment.

Run: `cd cdk && npm test`
Expected: PASS (or "no test files" if none exist yet — not a failure).

- [ ] **Step 6: Commit**

```bash
git add cdk/lib/dynamodb-stack.ts cdk/test/
git commit -m "feat(cdk): add audit_logs table with org-time and user-id GSIs"
```

---

### Task 2: `api` — non-executing transaction-item builders on `Base`

**Files:**

- Modify: `api/internal/repositories/base.go`
- Test: `api/internal/repositories/base_test.go`

**Interfaces:**

- Produces:
  ```go
  func (b *Base) BuildPutTxItem(item map[string]types.AttributeValue) types.TransactWriteItem
  func (b *Base) BuildUpdateTxItem(pk string, sk *string, updates map[string]any) (types.TransactWriteItem, error)
  func (b *Base) BuildDeleteTxItem(pk string, sk ...string) types.TransactWriteItem
  ```
  These mirror `PutItem`/`UpdateItem`/`DeleteItem` exactly (same key shape, same `buildUpdateExpr` helper, same
  `attribute_exists(pk)` condition on update/delete) but return a `types.TransactWriteItem` instead of executing — for
  composing multi-table transactions via the existing `Base.TransactWrite`.

- [ ] **Step 1: Write the failing tests**

Add to `api/internal/repositories/base_test.go` (check the existing file first for the test harness/table setup pattern
already used there — e.g. how `TestBase_PutItem`/`TestBase_UpdateItem` set up a local DynamoDB table — and follow it
exactly):

```go
func TestBase_BuildPutTxItem(t *testing.T) {
	b := Base{TableName: "test_table"} // no client needed — these builders only read TableName, exactly like existing base_test.go tests construct pure inputs (see TestBuildUpdateExpr_* in the same file) without a DynamoDB client
	item := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "PK1"},
		"sk": &types.AttributeValueMemberS{Value: "SK1"},
	}
	txItem := b.BuildPutTxItem(item)
	if txItem.Put == nil {
		t.Fatal("expected Put transact item, got nil")
	}
	if *txItem.Put.TableName != b.TableName {
		t.Errorf("table name = %q, want %q", *txItem.Put.TableName, b.TableName)
	}
	if txItem.Put.Item["pk"].(*types.AttributeValueMemberS).Value != "PK1" {
		t.Error("item not carried through unchanged")
	}
}

func TestBase_BuildUpdateTxItem(t *testing.T) {
	b := Base{TableName: "test_table"}
	sk := "SK1"
	txItem, err := b.BuildUpdateTxItem("PK1", &sk, map[string]any{"name": "new-name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txItem.Update == nil {
		t.Fatal("expected Update transact item, got nil")
	}
	if *txItem.Update.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition = %q, want attribute_exists(pk)", *txItem.Update.ConditionExpression)
	}
	if txItem.Update.Key["sk"].(*types.AttributeValueMemberS).Value != "SK1" {
		t.Error("sk not set on key")
	}
}

func TestBase_BuildDeleteTxItem(t *testing.T) {
	b := Base{TableName: "test_table"}
	txItem := b.BuildDeleteTxItem("PK1", "SK1")
	if txItem.Delete == nil {
		t.Fatal("expected Delete transact item, got nil")
	}
	if *txItem.Delete.ConditionExpression != "attribute_exists(pk)" {
		t.Errorf("condition = %q, want attribute_exists(pk)", *txItem.Delete.ConditionExpression)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
`cd api && go test ./internal/repositories/... -run 'TestBase_BuildPutTxItem|TestBase_BuildUpdateTxItem|TestBase_BuildDeleteTxItem' -v`
Expected: FAIL — `b.BuildPutTxItem undefined` (compile error).

- [ ] **Step 3: Implement the three builders in `base.go`, right after `DeleteItem` (after line 185)**

```go
// BuildPutTxItem returns a TransactWriteItem equivalent to PutItem, for composing
// a multi-item transaction via TransactWrite instead of writing immediately.
func (b *Base) BuildPutTxItem(item map[string]types.AttributeValue) types.TransactWriteItem {
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(b.TableName),
			Item:      item,
		},
	}
}

// BuildUpdateTxItem returns a TransactWriteItem equivalent to UpdateItem, for
// composing a multi-item transaction via TransactWrite instead of writing
// immediately. Same SET/REMOVE semantics and attribute_exists(pk) condition as
// UpdateItem.
func (b *Base) BuildUpdateTxItem(pk string, sk *string, updates map[string]any) (types.TransactWriteItem, error) {
	key := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
	}
	if sk != nil {
		key["sk"] = &types.AttributeValueMemberS{Value: *sk}
	}

	expr, exprNames, exprValues, err := buildUpdateExpr(updates)
	if err != nil {
		return types.TransactWriteItem{}, err
	}

	update := &types.Update{
		TableName:                aws.String(b.TableName),
		Key:                      key,
		UpdateExpression:         aws.String(expr),
		ExpressionAttributeNames: exprNames,
		ConditionExpression:      aws.String("attribute_exists(pk)"),
	}
	if len(exprValues) > 0 {
		update.ExpressionAttributeValues = exprValues
	}
	return types.TransactWriteItem{Update: update}, nil
}

// BuildDeleteTxItem returns a TransactWriteItem equivalent to DeleteItem, for
// composing a multi-item transaction via TransactWrite instead of writing
// immediately.
func (b *Base) BuildDeleteTxItem(pk string, sk ...string) types.TransactWriteItem {
	key := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
	}
	if len(sk) > 0 && sk[0] != "" {
		key["sk"] = &types.AttributeValueMemberS{Value: sk[0]}
	}
	return types.TransactWriteItem{
		Delete: &types.Delete{
			TableName:           aws.String(b.TableName),
			Key:                 key,
			ConditionExpression: aws.String("attribute_exists(pk)"),
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:
`cd api && go test ./internal/repositories/... -run 'TestBase_BuildPutTxItem|TestBase_BuildUpdateTxItem|TestBase_BuildDeleteTxItem' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/base.go api/internal/repositories/base_test.go
git commit -m "feat(api): add non-executing transaction-item builders to Base"
```

---

### Task 3: `api` — `AuditLogRepository`

**Files:**

- Create: `api/internal/repositories/audit_logs.go`
- Test: `api/internal/repositories/audit_logs_test.go`

**Interfaces:**

- Consumes: `Base.NewBase`, `Base.BuildPutTxItem`, `GenerateID()` (`api/internal/repositories/users.go:104`),
  `NowStr()` (`api/internal/repositories/base.go:43`).
- Produces:
  ```go
  type Modification struct {
      Name   string `dynamodbav:"name"`
      Before any    `dynamodbav:"before"`
      After  any    `dynamodbav:"after"`
  }
  func NewAuditLogRepository(db *dynamodb.Client, cfg *config.Config) *AuditLogRepository
  func (r *AuditLogRepository) BuildLogTxItem(orgPK, resourceType, resourceID, action, userID, userName string, modifications []Modification) (types.TransactWriteItem, error)
  ```

- [ ] **Step 1: Write the failing test**

```go
package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestAuditLogRepository_BuildLogTxItem(t *testing.T) {
	r := &AuditLogRepository{Base: Base{TableName: "dev_dfe_audit_logs"}}

	txItem, err := r.BuildLogTxItem(
		"CNPJ_12345678000195", "PRODUCT", "PRODUCT_abc123", "UPDATE",
		"user-1", "Jane Doe",
		[]Modification{{Name: "description", Before: "old", After: "new"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txItem.Put == nil {
		t.Fatal("expected Put transact item, got nil")
	}
	item := txItem.Put.Item
	if item["pk"].(*types.AttributeValueMemberS).Value != "CNPJ_12345678000195" {
		t.Errorf("pk = %v, want org pk", item["pk"])
	}
	sk := item["sk"].(*types.AttributeValueMemberS).Value
	wantPrefix := "PRODUCT#PRODUCT_abc123#"
	if len(sk) <= len(wantPrefix) || sk[:len(wantPrefix)] != wantPrefix {
		t.Errorf("sk = %q, want prefix %q", sk, wantPrefix)
	}
	if item["action"].(*types.AttributeValueMemberS).Value != "UPDATE" {
		t.Errorf("action = %v, want UPDATE", item["action"])
	}
	if item["user_id"].(*types.AttributeValueMemberS).Value != "user-1" {
		t.Errorf("user_id = %v, want user-1", item["user_id"])
	}
	mods := item["modifications"].(*types.AttributeValueMemberL).Value
	if len(mods) != 1 {
		t.Fatalf("modifications len = %d, want 1", len(mods))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/... -run TestAuditLogRepository_BuildLogTxItem -v`
Expected: FAIL — `AuditLogRepository` undefined (compile error).

- [ ] **Step 3: Implement `audit_logs.go`**

```go
package repositories

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Audit action constants — the `action` attribute on an audit_logs row.
const (
	AuditActionCreate = "CREATE"
	AuditActionUpdate = "UPDATE"
	AuditActionDelete = "DELETE"
)

// Audit resource-type constants — the `resource_type` attribute, and the first
// segment of the sort key (`{resource_type}#{resource_id}#{uuidv7}`).
const (
	AuditResourceOrganization = "ORGANIZATION"
	AuditResourceCertificate  = "CERTIFICATE"
	AuditResourceProduct      = "PRODUCT"
	AuditResourceVehicle      = "VEHICLE"
	AuditResourcePerson       = "PERSON"
	AuditResourceNfeConfig    = "NFE_CONFIG"
	AuditResourceNfceConfig   = "NFCE_CONFIG"
	AuditResourceCteConfig    = "CTE_CONFIG"
	AuditResourceMdfeConfig   = "MDFE_CONFIG"
)

// SystemActorID/SystemActorName attribute the row to a background process
// instead of an authenticated user (e.g. worker auto-creating a supplier).
const (
	SystemActorID   = "SYSTEM"
	SystemActorName = "Sistema"
)

// Modification is one changed field within an audit_logs row.
type Modification struct {
	Name   string `dynamodbav:"name"`
	Before any    `dynamodbav:"before"`
	After  any    `dynamodbav:"after"`
}

// AuditLogRepository stores per-field change records for org-owned mutating
// resources. Table structure (audit_logs):
//
//	pk = {org_pk}
//	sk = {resource_type}#{resource_id}#{uuidv7}
//
// GSIs: org-time-index (pk, created_at), user-id-index (user_id, created_at).
type AuditLogRepository struct {
	Base
}

func NewAuditLogRepository(db *dynamodb.Client, cfg *config.Config) *AuditLogRepository {
	return &AuditLogRepository{Base: NewBase(db, cfg, "audit_logs")}
}

// BuildLogTxItem returns a TransactWriteItem that writes one audit_logs row.
// Callers combine this with the primary resource's own Build*TxItem and execute
// both via Base.TransactWrite, so the mutation and its audit row commit atomically.
func (r *AuditLogRepository) BuildLogTxItem(
	orgPK, resourceType, resourceID, action, userID, userName string,
	modifications []Modification,
) (types.TransactWriteItem, error) {
	modsAV, err := attributevalue.MarshalList(modifications)
	if err != nil {
		return types.TransactWriteItem{}, fmt.Errorf("marshal modifications: %w", err)
	}

	item := map[string]types.AttributeValue{
		"pk":            &types.AttributeValueMemberS{Value: orgPK},
		"sk":            &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%s", resourceType, resourceID, GenerateID())},
		"resource_type": &types.AttributeValueMemberS{Value: resourceType},
		"resource_id":   &types.AttributeValueMemberS{Value: resourceID},
		"action":        &types.AttributeValueMemberS{Value: action},
		"modifications": &types.AttributeValueMemberL{Value: modsAV},
		"user_id":       &types.AttributeValueMemberS{Value: userID},
		"user_name":     &types.AttributeValueMemberS{Value: userName},
		"created_at":    &types.AttributeValueMemberS{Value: NowStr()},
	}
	return r.BuildPutTxItem(item), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd api && go test ./internal/repositories/... -run TestAuditLogRepository_BuildLogTxItem -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/audit_logs.go api/internal/repositories/audit_logs_test.go
git commit -m "feat(api): add AuditLogRepository"
```

---

### Task 4: `api` — `AuditService.Diff`

**Files:**

- Create: `api/internal/services/audit.go`
- Test: `api/internal/services/audit_test.go`

**Interfaces:**

- Consumes: `repositories.Modification`.
- Produces:
  ```go
  func Diff(before, after map[string]any) []repositories.Modification
  ```
  Excludes housekeeping keys (`pk`, `sk`, `created_at`, `updated_at`) from both sides. Compares the union of keys
  present in either map; a key present in only one side counts as changed (nil on the missing side). Used with
  `before=nil`-ish empty map for CREATE (every `after` field is a modification with `Before: nil`) and `after=nil`-ish
  empty map for DELETE (every `before` field is a modification with `After: nil`).

- [ ] **Step 1: Write the failing test**

```go
package services

import (
	"reflect"
	"sort"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestDiff_UpdateOnlyChangedFields(t *testing.T) {
	before := map[string]any{"pk": "P", "sk": "S", "description": "old", "code": "SKU1", "updated_at": "t1"}
	after := map[string]any{"description": "new", "code": "SKU1"}

	got := Diff(before, after)
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })

	want := []repositories.Modification{{Name: "description", Before: "old", After: "new"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}
}

func TestDiff_Create(t *testing.T) {
	after := map[string]any{"description": "new", "code": "SKU1"}
	got := Diff(nil, after)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for _, m := range got {
		if m.Before != nil {
			t.Errorf("Before = %v, want nil for CREATE", m.Before)
		}
	}
}

func TestDiff_Delete(t *testing.T) {
	before := map[string]any{"pk": "P", "description": "old"}
	got := Diff(before, nil)
	if len(got) != 1 || got[0].Name != "description" || got[0].After != nil {
		t.Errorf("Diff() = %+v, want one modification {description, old, nil}", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/services/... -run TestDiff -v`
Expected: FAIL — `Diff` undefined (compile error).

- [ ] **Step 3: Implement `audit.go`**

```go
package services

import (
	"reflect"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// auditHousekeepingKeys are never surfaced as modifications — they're storage
// mechanics, not business data.
var auditHousekeepingKeys = map[string]bool{
	"pk": true, "sk": true, "created_at": true, "updated_at": true,
}

// Diff compares before/after field maps and returns only the fields that
// changed. A field present on only one side is reported with nil on the other
// (used for CREATE — pass before=nil — and DELETE — pass after=nil).
func Diff(before, after map[string]any) []repositories.Modification {
	seen := make(map[string]bool)
	var mods []repositories.Modification

	visit := func(key string) {
		if seen[key] || auditHousekeepingKeys[key] {
			return
		}
		seen[key] = true
		b, a := before[key], after[key]
		if reflect.DeepEqual(b, a) {
			return
		}
		mods = append(mods, repositories.Modification{Name: key, Before: b, After: a})
	}
	for k := range before {
		visit(k)
	}
	for k := range after {
		visit(k)
	}
	return mods
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/services/... -run TestDiff -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/audit.go api/internal/services/audit_test.go
git commit -m "feat(api): add Diff helper for audit modification lists"
```

---

### Task 5: `api` — `UserService.ResolveActor`

**Files:**

- Modify: `api/internal/services/users.go`
- Test: `api/internal/services/users_test.go` (check if it exists first: `ls api/internal/services/*_test.go`; if
  `users_test.go` doesn't exist, create it)

**Interfaces:**

- Consumes: `s.cache` (`cache.Backend`), `s.GetUserInfo` (`api/internal/services/users.go:207`), `cacheGet`/`cacheSet`
  helpers already used in `GetMeData`.
- Produces:
  ```go
  func (s *UserService) ResolveActor(ctx context.Context, userID, accessToken string) (resolvedUserID, userName string)
  ```
  Always returns a non-empty `userName` — cache hit → live fetch → `userID` itself, in that order, never erroring or
  blocking the caller.

- [ ] **Step 1: Write the failing tests**

Check `api/internal/services/users_test.go` for the existing `cache.Backend` test double used elsewhere in this package
(search `cache.NewMemoryBackend` or a mock in `_test.go` files under `internal/services/`) and reuse it. Then add:

```go
func TestUserService_ResolveActor_CacheHit(t *testing.T) {
	c := cache.NewMemoryBackend(10)
	repo := &repositories.UserRepository{} // not touched on a cache hit
	svc := NewUserService(repo, c, "http://ctech.invalid", nil)

	cacheSet(context.Background(), c, "me:user-1", map[string]any{
		"username": "jane", "email": "jane@example.com", "first_name": "Jane", "last_name": "Doe",
	}, userCacheTTL)

	_, name := svc.ResolveActor(context.Background(), "user-1", "token-doesnt-matter")
	if name != "Jane Doe" {
		t.Errorf("name = %q, want %q", name, "Jane Doe")
	}
}

func TestUserService_ResolveActor_NoCacheNoNetwork_FallsBackToUserID(t *testing.T) {
	c := cache.NewMemoryBackend(10)
	repo := &repositories.UserRepository{}
	svc := NewUserService(repo, c, "http://127.0.0.1:1", nil) // unroutable — GetUserInfo fails fast

	_, name := svc.ResolveActor(context.Background(), "user-2", "token")
	if name != "user-2" {
		t.Errorf("name = %q, want fallback to userID %q", name, "user-2")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/services/... -run TestUserService_ResolveActor -v`
Expected: FAIL — `ResolveActor` undefined (compile error).

- [ ] **Step 3: Implement `ResolveActor` in `users.go`, right after `GetMeData` (after line 198)**

```go
// ResolveActor returns (userID, userName) for audit attribution: the cache
// populated by GetMeData is checked first (cheap, already warm for any user who
// has hit GET /auth/me — which the frontend calls on every mount). On a miss it
// falls back to a live userinfo fetch, and on total failure to userID itself.
// Never blocks or errors the caller — audit attribution degrades, it never fails
// the underlying mutation.
func (s *UserService) ResolveActor(ctx context.Context, userID, accessToken string) (string, string) {
	if v, ok := cacheGet[map[string]any](ctx, s.cache, fmt.Sprintf("me:%s", userID)); ok {
		if name := actorNameFromMeCache(*v); name != "" {
			return userID, name
		}
	}

	profile, err := s.GetUserInfo(ctx, accessToken)
	if err == nil {
		if name := actorNameFromProfile(profile); name != "" {
			return userID, name
		}
	}

	return userID, userID
}

// actorNameFromMeCache extracts a display name from the map cached by
// GetMeData ("me:{userID}"): "first last", or the username, in that order.
func actorNameFromMeCache(m map[string]any) string {
	first, _ := m["first_name"].(string)
	last, _ := m["last_name"].(string)
	if full := strings.TrimSpace(first + " " + last); full != "" {
		return full
	}
	username, _ := m["username"].(string)
	return username
}

// actorNameFromProfile extracts a display name from a live ctech-account
// userinfo response: Name, then "given family", then the email local-part.
func actorNameFromProfile(p *CtechUserInfo) string {
	if p.Name != "" {
		return p.Name
	}
	if full := strings.TrimSpace(p.GivenName + " " + p.FamilyName); full != "" {
		return full
	}
	if idx := strings.Index(p.Email, "@"); idx != -1 {
		return p.Email[:idx]
	}
	return ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/services/... -run TestUserService_ResolveActor -v`
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/users.go api/internal/services/users_test.go
git commit -m "feat(api): add UserService.ResolveActor for audit attribution"
```

---

### Task 6: `api` — wire `AuditLogRepository` into fx

**Files:**

- Modify: `api/internal/app/app.go:41-61` (repositories list), `api/internal/app/app.go:306-329` (`Services` struct +
  `registerRoutes`, if the repo needs to reach a route — see Task 18)

**Interfaces:**

- Consumes: `repositories.NewAuditLogRepository` (Task 3).
- Produces: `*repositories.AuditLogRepository` available for fx injection into any service constructor from Task 7
  onward.

- [ ] **Step 1: Add `repositories.NewAuditLogRepository` to the `fx.Provide` repositories list**

In `api/internal/app/app.go`, change:

```go
		repositories.NewOrganizationRepository,
		repositories.NewCertificateRepository,
```

to:

```go
		repositories.NewOrganizationRepository,
		repositories.NewCertificateRepository,
		repositories.NewAuditLogRepository,
```

- [ ] **Step 2: Verify the app still compiles and boots**

Run: `cd api && go build ./...`
Expected: builds cleanly (fx wiring is resolved at runtime, not compile time, but this confirms no syntax errors).

Run: `cd api && go vet ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add api/internal/app/app.go
git commit -m "feat(api): wire AuditLogRepository into fx"
```

---

## Part B — Table-audited CRUD resources

Each task in this part follows the same shape for one resource: (1) repository gains `Build*TxItem`-based methods
alongside the existing blind-write ones, (2) service `Create`/`Update`/`Delete` resolve the actor, fetch current state
where needed, diff, and execute one `TransactWrite` covering both the resource and the audit row, (3) the route handler
resolves the actor from the request and passes it down.

**Diffing gotcha (found and fixed during Task 7's review — apply the same fix in every task below):** `updates` in an
`Update` method is a *partial* map (only the fields the caller sent). Never call `Diff(beforeMap, updates)` directly —
every field present in `beforeMap` but absent from `updates` would be logged as a false "changed to null" modification.
Always merge a copy of `beforeMap` with `updates` first (`afterMap := copy of beforeMap, then overlay updates' keys`),
then call `Diff(beforeMap, afterMap)`. `ProductService.Update` (Task 7, already fixed) is the reference implementation
for this — copy its exact merge pattern, not the literal `Diff(beforeMap, updates)` text that may still appear in this
plan's older step descriptions below.

### Task 7: Products — transactional audit

**Files:**

- Modify: `api/internal/repositories/products.go`, `api/internal/services/products.go`,
  `api/internal/api/v1/products.go`
- Test: `api/internal/services/products_test.go` (create if it doesn't exist)

**Interfaces:**

- Consumes: `AuditLogRepository.BuildLogTxItem` (Task 3), `Diff` (Task 4), `UserService.ResolveActor` (Task 5),
  `Base.BuildPutTxItem`/`BuildUpdateTxItem`/`BuildDeleteTxItem` (Task 2).
- Produces: `ProductService.Create`/`Update`/`Delete` gain `userID, userName string` parameters.

**Confirmed test harness (read `api/tests/integration/setup_test.go` and `api/tests/integration/products_test.go` in
full before starting — this replaces any assumption made during planning):** integration tests live in package
`integration_test`, gated by build tag `//go:build integration`, and skip entirely unless env var `DYNAMODB_ENDPOINT` is
set (`docker run -p 8000:8000 amazon/dynamodb-local`). `TestMain` in `setup_test.go` creates tables once via
`createTables(ctx, db)` into package-level vars (`db`, `cfg`, `productRepo`, `productSvc`, etc.) that every test file in
the package uses directly — there is no per-test `setupTestDB(t)` call. `dropTables` tears down at the end. A
`randomCNPJ()` helper generates a valid org PK per test (avoids cross-test collisions since tables persist for the whole
run).

- [ ] **Step 1: Extend `api/tests/integration/setup_test.go` for `audit_logs`**

This task is the first to need the `audit_logs` table in integration tests — extend the shared harness once here; later
tasks in Part B reuse it as-is.

Add `auditRepo *repositories.AuditLogRepository` to the `var (...)` block (after `productRepo`), initialize it in
`TestMain` (`auditRepo = repositories.NewAuditLogRepository(db, cfg)`, right after `productRepo = ...`), and change the
`productSvc` construction to `productSvc = services.NewProductService(productRepo, auditRepo, memCache)` (this line will
not compile until Step 6 changes `NewProductService`'s signature — that's expected; Step 1-4 here establish the failing
test first, per TDD).

Add a table definition to `createTables`'s `definitions` slice (after the `organization_products` entry):

```go
		{
			TableName:   aws.String(tablePrefix + "_audit_logs"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("sk"), KeyType: types.KeyTypeRange},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("sk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("org-time-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("user-id-index"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
```

Add `tablePrefix + "_audit_logs"` to the `tables` slice in `dropTables`.

- [ ] **Step 2: Write the failing integration test**

Create `api/tests/integration/products_audit_test.go`:

```go
//go:build integration

package integration_test

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestProductService_CreateUpdateDelete_WriteAuditLogs(t *testing.T) {
	ctx := context.Background()
	orgPK := "CNPJ_" + randomCNPJ()

	created, err := productSvc.Create(ctx, orgPK, productFields("AUD001", "Audited Widget"), "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	skAV, _ := created["sk"].(interface{ GetValue() string })
	_ = skAV
	sk := created["sk"].(*attributeValueMemberSCompat).Value // placeholder — see note below

	logs, err := auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if err != nil {
		t.Fatalf("query audit_logs: %v", err)
	}
	if len(logs.Items) != 1 {
		t.Fatalf("audit_logs rows after Create = %d, want 1", len(logs.Items))
	}

	if _, err := productSvc.Update(ctx, orgPK, sk, map[string]any{"description": "Audited Widget v2"}, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	logs, _ = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if len(logs.Items) != 2 {
		t.Fatalf("audit_logs rows after Update = %d, want 2", len(logs.Items))
	}

	if err := productSvc.Delete(ctx, orgPK, sk, "user-1", "Jane Doe"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	logs, _ = auditRepo.Query(ctx, repositories.QueryOpts{PK: orgPK})
	if len(logs.Items) != 3 {
		t.Fatalf("audit_logs rows after Delete = %d, want 3", len(logs.Items))
	}
	if got, err := productSvc.Get(ctx, orgPK, sk); problemStatus(err) != 404 {
		t.Errorf("product should be gone after Delete, got %v (err %v)", got, err)
	}
}
```

Fix the sk-extraction line before running this — it was deliberately left inconsistent above (
`attributeValueMemberSCompat` is not a real type). Use the exact same pattern `products_test.go`'s own tests already use
one line above it: `skAV, _ := created["sk"].(*types.AttributeValueMemberS); sk := skAV.Value` (add
`"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"` to the imports, drop the two placeholder lines). `productFields`
and `problemStatus` are existing helpers already defined in this package (`products_test.go` /
`service_logic_test.go`) — reuse them, do not redefine.

- [ ] **Step 3: Update the existing tests in `products_test.go` for the new signatures**

`ProductService.Create`/`Update`/`Delete` are about to gain trailing `userID, userName string` params (Step 6) — every
existing call in `api/tests/integration/products_test.go` (`TestProduct_CreateAndGet`, `TestProduct_List`,
`TestProduct_Update`, `TestProduct_Delete`, `TestProduct_CacheInvalidatedOnCreate`) breaks otherwise. Update every
`productSvc.Create(...)`, `productSvc.Update(...)`, `productSvc.Delete(...)` call in that file to append
`"test-user", "Test User"` as the final two arguments (these tests aren't testing audit behavior, so a fixed constant
actor is fine — do not over-engineer this into a shared variable unless three or more call sites in the same test
function need it).

- [ ] **Step 4: Run tests to verify the new one fails and the existing ones still compile-fail together (expected — same
  task, same commit)**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'TestProductService_CreateUpdateDelete_WriteAuditLogs|TestProduct_' -v`
Expected: FAIL — compile error (`NewProductService`/`Create`/`Update`/`Delete` arity mismatch). If `DYNAMODB_ENDPOINT`
has no local DynamoDB listening, start one first: `docker run -d -p 8000:8000 amazon/dynamodb-local`.

- [ ] **Step 5: Add `Build*Tx`-based methods to `ProductRepository`**

In `api/internal/repositories/products.go`, add after `Delete` (after line 81):

```go
// BuildCreateTxItem returns a TransactWriteItem for a new product, mirroring
// Create's key/timestamp construction, without writing.
func (r *ProductRepository) BuildCreateTxItem(orgPK string, fields map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue) {
	now := NowStr()
	id := GenerateID()
	fields["pk"] = &types.AttributeValueMemberS{Value: orgPK}
	fields["sk"] = &types.AttributeValueMemberS{Value: buildProductSK(id)}
	fields["created_at"] = &types.AttributeValueMemberS{Value: now}
	fields["updated_at"] = &types.AttributeValueMemberS{Value: now}
	return r.BuildPutTxItem(fields), fields
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// product, mirroring Update's timestamp bump, without writing.
func (r *ProductRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, new(buildProductSK(sk)), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a product, without writing.
func (r *ProductRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, buildProductSK(sk))
}
```

- [ ] **Step 6: Rewrite `ProductService` to require `AuditLogRepository` and go through the transactional path**

Replace the whole of `api/internal/services/products.go` from `type ProductService struct` through the end of `Delete`
with:

```go
// ProductService mirrors api/app/services/products.py.
type ProductService struct {
	repo      *repositories.ProductRepository
	auditRepo *repositories.AuditLogRepository
	cache     cache.Backend
}

func NewProductService(repo *repositories.ProductRepository, auditRepo *repositories.AuditLogRepository, c cache.Backend) *ProductService {
	return &ProductService{repo: repo, auditRepo: auditRepo, cache: c}
}

func productCacheKey(orgPK, sk string) string {
	return fmt.Sprintf("res:%s:products:%s", orgPK, sk)
}

func productListCachePrefix(orgPK string) string {
	return fmt.Sprintf("res:%s:products:", orgPK)
}

func (s *ProductService) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	key := productCacheKey(orgPK, sk)
	if v, ok := cacheGetItem(ctx, s.cache, key); ok {
		return v, nil
	}
	item, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("product not found")
	}
	cacheSetItem(ctx, s.cache, key, item, productCacheTTL)
	return item, nil
}

func (s *ProductService) List(ctx context.Context, orgPK string, opts repositories.ProductListOpts) (*repositories.QueryResult, error) {
	return s.repo.List(ctx, orgPK, opts)
}

// Create writes the product and its CREATE audit row atomically.
func (s *ProductService) Create(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	productTx, finalItem := s.repo.BuildCreateTxItem(orgPK, fields)

	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, attrStrAV(finalItem, "sk"), repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return finalItem, nil
}

// Update writes the product change and its UPDATE audit row atomically. Fetches
// the current item first so only actually-changed fields are logged.
func (s *ProductService) Update(ctx context.Context, orgPK, sk string, updates map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, problem.NotFound("product not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return nil, err
	}

	productTx, err := s.repo.BuildUpdateTxItem(orgPK, sk, updates)
	if err != nil {
		return nil, err
	}
	// updates is partial (only the fields the caller sent) — diffing it directly
	// against beforeMap would log every untouched field as "changed to null".
	// Merge onto a copy of beforeMap first so only real changes survive Diff.
	afterMap := make(map[string]any, len(beforeMap))
	for k, v := range beforeMap {
		afterMap[k] = v
	}
	for k, v := range updates {
		afterMap[k] = v
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, buildProductSK(sk), repositories.AuditActionUpdate,
		userID, userName, Diff(beforeMap, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.Delete(ctx, productCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return s.repo.Get(ctx, orgPK, sk)
}

// Delete removes the product and writes its DELETE audit row atomically.
func (s *ProductService) Delete(ctx context.Context, orgPK, sk, userID, userName string) error {
	current, err := s.repo.Get(ctx, orgPK, sk)
	if err != nil {
		return err
	}
	if current == nil {
		return problem.NotFound("product not found")
	}
	beforeMap, err := attributeMapToPlain(current)
	if err != nil {
		return err
	}

	productTx := s.repo.BuildDeleteTxItem(orgPK, sk)
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceProduct, buildProductSK(sk), repositories.AuditActionDelete,
		userID, userName, Diff(beforeMap, nil),
	)
	if err != nil {
		return err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{productTx, auditTx}); err != nil {
		return err
	}
	_ = s.cache.Delete(ctx, productCacheKey(orgPK, sk))
	_ = s.cache.DeletePrefix(ctx, productListCachePrefix(orgPK))
	return nil
}
```

Add the shared helpers `attributeMapToPlain` and `attrStrAV` to `api/internal/services/shared.go` (open it first — if
either already exists under a different name, e.g. `unmarshalItem`, reuse it instead of adding a duplicate, and adjust
the code above to call the existing name):

```go
// attributeMapToPlain converts a DynamoDB attribute map to a plain Go map, for
// diffing against a JSON-shaped `updates` map from a request body.
func attributeMapToPlain(item map[string]types.AttributeValue) (map[string]any, error) {
	var m map[string]any
	if err := attributevalue.UnmarshalMap(item, &m); err != nil {
		return nil, problem.InternalServer("failed to unmarshal item for audit diff")
	}
	return m, nil
}

// attrStrAV extracts a string attribute from a DynamoDB item, or "" if absent.
// (If internal/api/v1/helpers.go's attrStr is visible from this package, reuse
// it instead — this exists only if services can't import the v1 package.)
func attrStrAV(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}
```

- [ ] **Step 7: Update the route handlers to resolve the actor and pass it through**

In `api/internal/api/v1/products.go`, the `RegisterProducts` signature needs `userSvc *services.UserService` — update
the call site in `router.go`/`app.go` accordingly (grep `RegisterProducts(` to find it:
`grep -rn "RegisterProducts(" api/internal/`). Change the `POST`, `PUT`, and `DELETE` handlers:

```go
	// POST /products
	g.Post("", perm.Require("create.organization_products"), func(c fiber.Ctx) error {
		av, p := bindAVValidated[ProductBody](c)
		if p != nil {
			return sendProblem(c, p)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Create(c.Context(), middleware.GetOrgPK(c), av, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		m, err := unmarshal(item)
		if err != nil {
			return sendProblem(c, err)
		}
		return c.Status(fiber.StatusCreated).JSON(m)
	})
```

```go
	// PUT /products/:product_id
	g.Put("/:product_id", perm.Require("update.organization_products"), func(c fiber.Ctx) error {
		var dto ProductBody
		if p := bindJSON(c, &dto); p != nil {
			return sendProblem(c, p)
		}
		body, err := structToMap(dto)
		if err != nil {
			return sendProblem(c, err)
		}
		userID, userName := resolveActor(c, userSvc)
		item, err := svc.Update(c.Context(), middleware.GetOrgPK(c), c.Params("product_id"), body, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		return sendItem(c, item)
	})
```

```go
	// DELETE /products/:product_id
	g.Delete("/:product_id", perm.Require("delete.organization_products"), func(c fiber.Ctx) error {
		userID, userName := resolveActor(c, userSvc)
		if err := svc.Delete(c.Context(), middleware.GetOrgPK(c), c.Params("product_id"), userID, userName); err != nil {
			return sendProblem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
```

Add the shared `resolveActor` helper to `api/internal/api/v1/helpers.go` (this is reused by every task in Part B and
Part C — write it once here):

```go
// resolveActor extracts the caller's user_id (from auth middleware locals) and
// resolves their display name for audit attribution. Every mutating route that
// needs to attribute a change calls this once, right after request validation.
func resolveActor(c fiber.Ctx, userSvc *services.UserService) (userID, userName string) {
	userID = middleware.GetUserID(c)
	accessToken := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	_, userName = userSvc.ResolveActor(c.Context(), userID, accessToken)
	return userID, userName
}
```

(Add `"strings"` and `"gopkg.aoctech.app/dfe/api/internal/services"` to `helpers.go`'s imports if not already present —
check first.)

- [ ] **Step 8: Update `RegisterProducts`'s signature and its call site**

In `api/internal/api/v1/products.go`:

```go
func RegisterProducts(router fiber.Router, svc *services.ProductService, userSvc *services.UserService, authMw fiber.Handler, perm *middleware.PermChecker) {
```

Find and update the call site (`grep -rn "RegisterProducts(" api/internal/api/v1/router.go`) to pass `svcs.User` (or
whatever the `Services` struct's `UserService` field is named in that file — check `router.go`'s existing `Services`
struct, which is separate from `app.go`'s).

- [ ] **Step 9: Update `NewProductService`'s fx wiring**

`ProductService` is constructed directly by `fx.Provide(services.NewProductService)` in `app.go` — since its constructor
signature changed (new `auditRepo` param), fx resolves this automatically as long as `*repositories.AuditLogRepository`
is providable (done in Task 6). No `app.go` change needed here — verify with the build in Step 10.

- [ ] **Step 10: Run the tests and full build**

Run: `cd api && go build ./... && go vet ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests, no DynamoDB needed).

Run: `docker run -d -p 8000:8000 amazon/dynamodb-local` (skip if one is already running), then
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Product' -v`
Expected: PASS — all `TestProduct_*` (existing) and `TestProductService_CreateUpdateDelete_WriteAuditLogs` (new).

- [ ] **Step 11: Commit**

```bash
git add api/internal/repositories/products.go api/internal/services/products.go \
        api/internal/services/shared.go api/internal/api/v1/products.go \
        api/internal/api/v1/helpers.go api/internal/api/v1/router.go \
        api/tests/integration/setup_test.go api/tests/integration/products_test.go \
        api/tests/integration/products_audit_test.go
git commit -m "feat(api): audit product create/update/delete atomically"
```

---

### Task 8: Vehicles — transactional audit

Same pattern as Task 7, applied to `api/internal/repositories/vehicles.go`, `api/internal/services/vehicles.go` (
`VehicleService`), `api/internal/api/v1/vehicles.go`.

**Files:**

- Modify: `api/internal/repositories/vehicles.go`, `api/internal/services/vehicles.go`,
  `api/internal/api/v1/vehicles.go`
- Test: `api/tests/integration/vehicles_audit_test.go`

**Interfaces:**

- Produces: `VehicleService.Create`/`Update`/`Delete` gain `userID, userName string` parameters; audit rows use
  `repositories.AuditResourceVehicle`.

`api/tests/integration/setup_test.go` already declares `vehicleRepo`/`vehicleSvc` and creates the
`organization_vehicles` table (Task 7 Step 1 already added `audit_logs` — no table changes needed here, just wire
`auditRepo` into `vehicleSvc`'s construction).

- [ ] **Step 1: Update `setup_test.go`'s `vehicleSvc` construction and write the failing integration test**

Change `vehicleSvc = services.NewVehicleService(vehicleRepo, memCache)` to
`vehicleSvc = services.NewVehicleService(vehicleRepo, auditRepo, memCache)` (won't compile until Step 4 — expected).

Copy `api/tests/integration/products_audit_test.go` (Task 7) to `api/tests/integration/vehicles_audit_test.go`, same
`//go:build integration` / `package integration_test` header, replacing `productSvc`→`vehicleSvc`, `productFields(...)`→
`map[string]any{"plate": "ABC1D23", "plate_uf": "SP"}` (a valid Mercosul plate per `plateRe` in
`api/internal/services/vehicles.go:19`) for `Create`, and `map[string]any{"plate": "ABC1D24"}` for `Update`.

- [ ] **Step 2: Update the existing tests in `vehicles_test.go` for the new signatures**

Same as Task 7 Step 3 — append `"test-user", "Test User"` to every `vehicleSvc.Create`/`Update`/`Delete` call in
`api/tests/integration/vehicles_test.go`.

- [ ] **Step 3: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'TestVehicleService_CreateUpdateDelete_WriteAuditLogs|TestVehicle_' -v`
Expected: FAIL — compile error.

- [ ] **Step 4: Add `Build*TxItem` methods to `VehicleRepository`**

Read `api/internal/repositories/vehicles.go` first (its `Create`/`Update`/`Delete` signatures differ slightly from
products' — `Create` takes individual typed params, not a `fields map[string]any`). Add companions mirroring Task 7 Step
5's shape but matching this file's actual
`Create(ctx, orgPK, plate, plateUF, wheelset, bodywork, renavam string, weight int, owner map[string]any, trailers []map[string]any)`
signature — build the same field-assembly logic that `Create` uses internally, but return a `TransactWriteItem` (via
`r.BuildPutTxItem`) plus the assembled item, instead of calling `PutItem`. Name them `BuildCreateTxItem`,
`BuildUpdateTxItem` (thin wrapper delegating to `r.Base.BuildUpdateTxItem` after bumping `updated_at`, same as
products), `BuildDeleteTxItem`.

- [ ] **Step 5: Rewrite `VehicleService.Create`/`Update`/`Delete`**

Same structure as Task 7 Step 6 — `NewVehicleService` gains an `auditRepo *repositories.AuditLogRepository` parameter
(matching Step 1's updated construction), keep the existing plate/renavam/owner-type validation at the top of `Create`/
`Update` unchanged, then build both `TransactWriteItem`s and call `s.repo.TransactWrite`. Use
`repositories.AuditResourceVehicle` and the vehicle's own `sk` as `resourceID`.

- [ ] **Step 6: Update `api/internal/api/v1/vehicles.go`**

Same as Task 7 Step 7 — add `userSvc *services.UserService` to `RegisterVehicles`, call `resolveActor(c, userSvc)` in
the `POST`/`PUT`/`DELETE` handlers, pass `userID, userName` through to the service calls. Update the call site in
`router.go`.

- [ ] **Step 7: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests).

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Vehicle' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add api/internal/repositories/vehicles.go api/internal/services/vehicles.go \
        api/internal/api/v1/vehicles.go api/internal/api/v1/router.go \
        api/tests/integration/setup_test.go api/tests/integration/vehicles_test.go \
        api/tests/integration/vehicles_audit_test.go
git commit -m "feat(api): audit vehicle create/update/delete atomically"
```

---

### Task 9: Persons — transactional audit (org_pk attribute, not the table's own pk)

Same pattern as Task 7/8, with one structural difference: `organization_persons`' own table `pk` is the counterparty's
`cpf_cnpj`, not the org PK (`DynamoDB-Tables.md`), but the `org_pk` attribute on every person item holds the owning
org's PK — `audit_logs.pk` must use that `org_pk` value, which the route/service already has as its own `orgPK`
parameter (it's passed in explicitly on every `PersonService` call), so no extra lookup is needed.

**Files:**

- Modify: `api/internal/repositories/persons.go`, `api/internal/services/persons.go`, `api/internal/api/v1/persons.go`
- Test: `api/tests/integration/persons_audit_test.go`

**Interfaces:**

- Produces: `PersonService.Create`/`Update`/`Delete` gain `userID, userName string` parameters; audit rows use
  `repositories.AuditResourcePerson`, `resourceID` = the person's own `pk` (`{CNPJ|CPF}_{digits}`, from
  `BuildPersonSK`... actually re-check: `DynamoDB-Tables.md` says persons table `pk = {cpf_cnpj}` and
  `sk = PERSON_{id}` — confirm the exact key shape by reading `api/internal/repositories/persons.go` before writing this
  task's code, since `services/persons.go`'s `BuildPersonSK` (which returns `"CNPJ_"+digits` or `"CPF_"+digits`) is used
  as the **sk** passed to `r.repo.Get(ctx, orgPK, sk)`, meaning the table's `pk` is `orgPK` after all — re-verify
  against the actual repository file, not just `DynamoDB-Tables.md`, before assuming the org_pk-attribute workaround
  above is even needed.

`api/tests/integration/setup_test.go` already declares `personRepo`/`personSvc` and creates the `organization_persons`
table — only `personSvc`'s construction needs `auditRepo` added.

- [ ] **Step 1: Verify the real key shape before writing any code**

Run: `sed -n '1,40p' api/internal/repositories/persons.go`
Expected: confirms whether `pk` is `orgPK` (matching how `PersonService` already calls `r.repo.Get(ctx, orgPK, sk)`) or
the CPF/CNPJ. Given `PersonService.Get`/`Create`/`Update`/`Delete` in `api/internal/services/persons.go:53-115` all call
the repo with `orgPK` as the first key argument, the table's `pk` is almost certainly `orgPK` and `DynamoDB-Tables.md`'
s "pk = {cpf_cnpj}" is describing the *legacy/DynamoDB-Tables.md's* documented intent rather than this Go
implementation, OR the doc is right and `orgPK` in these calls is misleadingly named. **Resolve this discrepancy by
reading the file — do not guess** — then write Steps 2+ using whatever the real key shape is. If `pk` genuinely is
`orgPK`, `audit_logs.pk` is just `orgPK` directly, same as products/vehicles, and the design doc's "org_pk attribute"
caveat does not apply — note that in the commit message.

- [ ] **Step 2: Update `setup_test.go`'s `personSvc` construction and write the failing integration test**

Change `personSvc = services.NewPersonService(personRepo, memCache)` to
`personSvc = services.NewPersonService(personRepo, auditRepo, memCache)` (won't compile until Step 5 — expected).

Copy `api/tests/integration/products_audit_test.go`, adapted for persons:
`personSvc.Create(ctx, orgPK, "12345678000195", map[string]any{"name": "Acme Ltda"}, "user-1", "Jane Doe")`.

- [ ] **Step 3: Update the existing tests in `persons_test.go` for the new signatures**

Same as Task 7 Step 3 — append `"test-user", "Test User"` to every `personSvc.Create`/`Update`/`Delete` call in
`api/tests/integration/persons_test.go`.

- [ ] **Step 4: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'TestPersonService_CreateUpdateDelete_WriteAuditLogs|TestPerson_' -v`
Expected: FAIL — compile error.

- [ ] **Step 5: Add `Build*TxItem` methods to `PersonRepository`, matching whatever `Create`/`Update`/`Delete` actually
  do (read the file first, mirror Task 7 Step 5's shape)**

- [ ] **Step 6: Rewrite `PersonService.Create`/`Update`/`Delete`** — same structure as Task 7 Step 6, `NewPersonService`
  gains the `auditRepo` param (matching Step 2), using `repositories.AuditResourcePerson` and the resolved `sk` (from
  `BuildPersonSK`) as `resourceID`, and the key established in Step 1 as `audit_logs.pk`.

- [ ] **Step 7: Update `api/internal/api/v1/persons.go`** — same as Task 7 Step 7.

- [ ] **Step 8: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests).

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Person' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add api/internal/repositories/persons.go api/internal/services/persons.go \
        api/internal/api/v1/persons.go api/internal/api/v1/router.go \
        api/tests/integration/setup_test.go api/tests/integration/persons_test.go \
        api/tests/integration/persons_audit_test.go
git commit -m "feat(api): audit person create/update/delete atomically"
```

---

### Task 10: Certificates — transactional audit (Create + Delete only, no Update)

**Files:**

- Modify: `api/internal/repositories/certificates.go`, `api/internal/services/certificates.go`,
  `api/internal/api/v1/organizations.go` (certificate routes live here, per Task investigation —
  `scoped.Post("/certificates", ...)` at line 144, `scoped.Delete("/certificates/:md5", ...)` at line 173)
- Test: `api/tests/integration/certificates_audit_test.go`

**Interfaces:**

- Produces: `CertificateService.Upload`/`Delete` gain `userID, userName string` parameters;
  `repositories.AuditResourceCertificate`; `resourceID` = the certificate's `md5` (its `sk`, per `DynamoDB-Tables.md`:
  `sk = CERT_{iso_timestamp}` — verify against `api/internal/repositories/certificates.go` directly, same caution as
  Task 9).

Unlike vehicles/persons, `setup_test.go` has **no** `certRepo`/`certSvc` var or `organization_certificates` table yet —
this task adds both, following the exact pattern used for `productRepo`/`productSvc` (declare vars, initialize in
`TestMain`, add a table definition to `createTables`, add the table name to `dropTables`). Copy the
`organization_products` table definition's shape (`pk`+`sk` both `S`, no GSI needed since certificates have no `code`/
`description`-style secondary lookup — check `DynamoDB-Tables.md`'s `organization_certificates` entry to confirm no GSI
is documented for it before assuming none is needed).

- [ ] **Step 1: Read `api/internal/repositories/certificates.go` in full to confirm `Create`/`Delete` signatures and the
  exact sk shape**

- [ ] **Step 2: Extend `setup_test.go`**: add `certRepo *repositories.CertificateRepository` and
  `certSvc *services.CertificateService` to the var block, initialize both in `TestMain` (`certSvc` needs an
  `*awsclient.Clients` and bucket name — check `NewCertificateService`'s signature from Task 10's own reading of
  `certificates.go`; if wiring a real S3 client into the integration harness is impractical, construct `certSvc` with a
  `nil`-safe/local test double the same way this project already fakes AWS clients elsewhere for tests — check
  `api/internal/services/certificates_test.go` and `api/tests/integration/` for an existing pattern before inventing
  one), add the `organization_certificates` table definition to `createTables`, add it to `dropTables`.

- [ ] **Step 3: Write the failing integration test**

Adapted from Task 7's pattern: `certSvc.Upload(ctx, orgPK, pfxBytes, password, alias, "user-1", "Jane Doe")`, then
assert one `audit_logs` row with `action=CREATE`; `certSvc.Delete(ctx, orgPK, md5, "user-1", "Jane Doe")`, assert a
second row with `action=DELETE`. Certificates require a real PFX fixture — check `api/tests/integration/` or
`api/internal/services/certificates_test.go` for an existing test PFX fixture (
`grep -rln "pkcs12\|\.pfx" api/tests/ api/internal/services/*_test.go`) and reuse it; do not generate a new one.

- [ ] **Step 4: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run TestCertificateService_UploadDelete_WriteAuditLogs -v`
Expected: FAIL — compile error.

- [ ] **Step 5: Add `Build*TxItem` methods to `CertificateRepository`** (mirror Task 7 Step 5, using this repo's actual
  `Create`/`Delete` field-assembly logic).

- [ ] **Step 6: Rewrite `CertificateService.Upload`/`Delete`** to build both transact items and call `TransactWrite` —
  note `Upload` has no prior "current state" to diff (it's a create), and note it uploads to S3 *before* the DynamoDB
  transaction (keep that order — an audit row for a cert that failed to upload to S3 would be wrong); `Delete` has no
  repository `Get`-before pattern today (`api/internal/services/certificates.go:167-170` deletes blind) — add a
  `s.repo.Get` first, matching Task 7's `Delete`, so the DELETE audit row can record what was removed (`password` field
  must be excluded from the diff — check it explicitly and drop it from the `beforeMap` before calling `Diff`, since
  `Delete`'s neighboring methods already redact `password` from responses — grep `delete(out, "password")` in this file
  for the existing pattern to follow).

- [ ] **Step 7: Update the certificate routes in `api/internal/api/v1/organizations.go`** — add
  `resolveActor(c, userSvc)` calls at the `POST /certificates` and `DELETE /certificates/:md5` handlers (lines ~144
  and ~173), matching Task 7 Step 7's pattern. `RegisterOrganizations`'s handler struct (`OrgHandlers`, referenced at
  `organizations.go:25`) needs a `UserSvc *services.UserService` field — check its definition (likely in the same file
  or `dto.go`) and add it, then update the call site.

- [ ] **Step 8: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests).

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Certificate' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add api/internal/repositories/certificates.go api/internal/services/certificates.go \
        api/internal/api/v1/organizations.go \
        api/tests/integration/setup_test.go \
        api/tests/integration/certificates_audit_test.go
git commit -m "feat(api): audit certificate upload/delete atomically"
```

---

### Task 11: Organizations — transactional audit (Update only, company data)

**Files:**

- Modify: `api/internal/repositories/organizations.go`, `api/internal/services/organizations.go`,
  `api/internal/api/v1/organizations.go`
- Test: `api/tests/integration/organizations_audit_test.go`

**Interfaces:**

- Produces: `OrganizationService.Update` gains `userID, userName string` parameters;
  `repositories.AuditResourceOrganization`; `resourceID` = `orgPK` itself (the org's own pk).

`api/tests/integration/setup_test.go` already declares `orgRepo`/`orgSvc` and creates the `organizations` table — only
`orgSvc`'s construction needs `auditRepo` added.

- [ ] **Step 1: Read `api/internal/repositories/organizations.go`'s `UpdateOrganization` to confirm its signature before
  changing it**

- [ ] **Step 2: Update `setup_test.go`'s `orgSvc` construction and write the failing integration test**

Change `orgSvc = services.NewOrganizationService(orgRepo, memCache)` to
`orgSvc = services.NewOrganizationService(orgRepo, auditRepo, memCache)` (won't compile until Step 5 — expected).

`orgSvc.Update(ctx, orgPK, map[string]any{"name": "New Name"}, "user-1", "Jane Doe")`, assert one `audit_logs` row with
`action=UPDATE`, `resource_id=orgPK`.

- [ ] **Step 3: Update the existing tests in `organizations_test.go` for the new signature**

Same as Task 7 Step 3 — append `"test-user", "Test User"` to every `orgSvc.Update` call in
`api/tests/integration/organizations_test.go`.

- [ ] **Step 4: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'TestOrganizationService_Update_WritesAuditLog|TestOrganization' -v`
Expected: FAIL — compile error.

- [ ] **Step 5: Add a `BuildUpdateTxItem` method to `OrganizationRepository`**, mirroring `UpdateOrganization`'s
  existing key/condition logic but delegating to `r.Base.BuildUpdateTxItem`.

- [ ] **Step 6: Rewrite `OrganizationService.Update`** (in `api/internal/services/organizations.go:53-61`) —
  `NewOrganizationService` gains the `auditRepo` param (matching Step 2); fetch the current item first (it already has
  `s.Get`, reuse it), diff, build both transact items, and call `TransactWrite`. Note this service has no
  `TransactWrite` call site to delegate to today — add `TransactWrite(ctx, items)` on `s.repo.Base` directly (same as
  every other task in Part B).

- [ ] **Step 7: Update `PUT /organizations/{pk}` in `api/internal/api/v1/organizations.go:104`** — add
  `resolveActor(c, userSvc)`, pass through.

- [ ] **Step 8: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests).

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Organization' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add api/internal/repositories/organizations.go api/internal/services/organizations.go \
        api/internal/api/v1/organizations.go \
        api/tests/integration/setup_test.go api/tests/integration/organizations_test.go \
        api/tests/integration/organizations_audit_test.go
git commit -m "feat(api): audit organization company-data updates atomically"
```

---

### Task 12: Fiscal configs (NF-e/NFC-e/CT-e/MDF-e) — transactional audit

All four config services (`NfeConfigService`, `NfceConfigService`, `CteConfigService`, `MdfeConfigService`) share an
identical shape (`api/internal/services/fiscal_configs.go`) — handle all four in this one task. `resourceID` for each is
a fixed constant (`"nfe_config"`, `"nfce_config"`, `"cte_config"`, `"mdfe_config"`), since these tables hold one item
per org (no `sk`).

**Files:**

- Modify: `api/internal/repositories/fiscal_config.go` (or `fiscal_configs.go` — check which file actually holds
  `NfeConfigRepository.Upsert` etc. with `grep -rn "func.*ConfigRepository.*Upsert" api/internal/repositories/`),
  `api/internal/services/fiscal_configs.go`, `api/internal/api/v1/organizations.go` (config routes, per
  `registerFiscalConfig` at lines 121-132)
- Test: `api/tests/integration/fiscal_configs_audit_test.go`

**Interfaces:**

- Produces: `NfeConfigService.Upsert`/`NfceConfigService.Upsert`/`CteConfigService.Upsert`/`MdfeConfigService.Upsert`
  gain `userID, userName string` parameters.

`api/tests/integration/setup_test.go` has **no** config repo/service vars or config tables yet — this task adds all four
(`nfeConfigRepo`/`nfeConfigSvc`, etc.), following the `productRepo`/`productSvc` pattern from `TestMain`. Per
`DynamoDB-Tables.md`, each config table is `pk`-only (no `sk`, no GSI) — simplest table shape in the file, one
`CreateTableInput` per doc type, four total.

- [ ] **Step 1: Read the repository file (s) to confirm `Upsert`'s exact behavior (does it use `PutItem` or
  `UpdateItem`? Is it truly upsert-or-create, i.e. does CREATE vs UPDATE distinction even apply?)**

- [ ] **Step 2: Extend `setup_test.go`**: add all four config repo/service vars, initialize in `TestMain`, add all four
  `pk`-only table definitions to `createTables` (`organization_nfe_configs`, `organization_nfce_configs`,
  `organization_cte_configs`, `organization_mdfe_configs` — confirm exact table-name suffixes against
  `api/internal/repositories/fiscal_config*.go`'s `NewBase(db, cfg, "...")` calls, don't assume), add all four to
  `dropTables`.

- [ ] **Step 3: Write the failing integration test** for one config type (`NfeConfigService`) —
  `nfeConfigSvc.Upsert(ctx, orgPK, fields, "user-1", "Jane Doe")`, assert one `audit_logs` row with
  `resource_type=NFE_CONFIG`, `resource_id="nfe_config"`.

- [ ] **Step 4: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run TestNfeConfigService_Upsert_WritesAuditLog -v`
Expected: FAIL — compile error.

- [ ] **Step 5: Add a `BuildUpsertTxItem` method to each of the four config repositories**, mirroring their existing
  `Upsert`.

- [ ] **Step 6: Rewrite each of the four `*ConfigService.Upsert` methods** in
  `api/internal/services/fiscal_configs.go` — each `New*ConfigService` gains the `auditRepo` param (matching Step 2's
  updated constructions) — to fetch current (via existing `Get`, tolerating "not found" as an empty before-map rather
  than erroring — first-time config setup is a CREATE, not an UPDATE, so use `repositories.AuditActionCreate` when `Get`
  returns not-found and `AuditActionUpdate` otherwise), diff, build both transact items, call `TransactWrite`. Use the
  matching `AuditResourceNfeConfig`/`AuditResourceNfceConfig`/`AuditResourceCteConfig`/`AuditResourceMdfeConfig`
  constant and fixed `resourceID` string per service.

- [ ] **Step 7: Update the four config routes in `api/internal/api/v1/organizations.go`** (`registerFiscalConfig`
  helper, lines 121-132) — add `resolveActor` and thread it through the shared helper function so all four inherit it
  from one change (check `registerFiscalConfig`'s signature first; it likely takes the `Upsert` func as a parameter —
  update that function type to accept `userID, userName` too).

- [ ] **Step 8: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/repositories/... ./internal/services/... -v`
Expected: PASS (unit tests).

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Config' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add api/internal/repositories/fiscal_config*.go api/internal/services/fiscal_configs.go \
        api/internal/api/v1/organizations.go \
        api/tests/integration/setup_test.go \
        api/tests/integration/fiscal_configs_audit_test.go
git commit -m "feat(api): audit fiscal config upserts atomically"
```

---

## Part C — DF-e in-record actor fields

### Task 13: `DocumentEventRepository.CreateEvent` gains actor fields

**Files:**

- Modify: `api/internal/repositories/dfe_events.go`
- Test: `api/internal/repositories/dfe_events_test.go` (create if it doesn't exist)

**Interfaces:**

- Produces: `CreateEvent` signature becomes:
  ```go
  func (r *DocumentEventRepository) CreateEvent(ctx context.Context, accessKey, eventType string, sequenceNumber int, status string, sefazStatus, sefazMotive, xmlS3Key *string, userID, userName string) (map[string]types.AttributeValue, error)
  ```

- [ ] **Step 1: Write the failing test**

```go
func TestDocumentEventRepository_CreateEvent_StampsActor(t *testing.T) {
	r := newTestEventRepo(t) // reuse whatever local-DynamoDB harness this file's other tests use
	item, err := r.CreateEvent(context.Background(), "43210000000000000000000000000000000000000000", "210200", 1, "pending", nil, nil, nil, "user-1", "Jane Doe")
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if item["user_id"].(*types.AttributeValueMemberS).Value != "user-1" {
		t.Errorf("user_id = %v, want user-1", item["user_id"])
	}
	if item["user_name"].(*types.AttributeValueMemberS).Value != "Jane Doe" {
		t.Errorf("user_name = %v, want Jane Doe", item["user_name"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/... -run TestDocumentEventRepository_CreateEvent_StampsActor -v`
Expected: FAIL — argument count mismatch (compile error).

- [ ] **Step 3: Update `CreateEvent` in `api/internal/repositories/dfe_events.go:28-54`**

```go
func (r *DocumentEventRepository) CreateEvent(
	ctx context.Context,
	accessKey, eventType string,
	sequenceNumber int,
	status string,
	sefazStatus, sefazMotive, xmlS3Key *string,
	userID, userName string,
) (map[string]types.AttributeValue, error) {
	id := GenerateID()
	now := NowStr()

	item := map[string]types.AttributeValue{
		"pk":              &types.AttributeValueMemberS{Value: accessKey},
		"sk":              &types.AttributeValueMemberS{Value: id},
		"access_key":      &types.AttributeValueMemberS{Value: accessKey},
		"event_key":       &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%03d", accessKey, eventType, sequenceNumber)},
		"event_type":      &types.AttributeValueMemberS{Value: eventType},
		"sequence_number": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", sequenceNumber)},
		"status":          &types.AttributeValueMemberS{Value: status},
		"user_id":         &types.AttributeValueMemberS{Value: userID},
		"user_name":       &types.AttributeValueMemberS{Value: userName},
		"created_at":      &types.AttributeValueMemberS{Value: now},
		"updated_at":      &types.AttributeValueMemberS{Value: now},
	}
	setNullableStr(item, "sefaz_status", sefazStatus)
	setNullableStr(item, "sefaz_motive", sefazMotive)
	setNullableStr(item, "xml_s3_key", xmlS3Key)

	return item, r.PutItem(ctx, item)
}
```

- [ ] **Step 4: Fix every existing call site (this will not compile until all are updated — do this in the same commit,
  not a follow-up)**

Run: `grep -rln "\.CreateEvent(" api/internal/services/` — expect `internal/services/nfes/service.go` (3 call sites:
Cancel, CorrectionLetter, Manifestation), `internal/services/nfes/nfce_service.go` (2: Cancel, Substitute — check
`prepareEvent`/`Substitute` for a possible 3rd), `internal/services/mdfes/events.go` (1: `dispatchEvent`). Each of these
call sites currently ends `..., nil, nil, nil)` — this task only makes them compile by appending two placeholder
strings; **do not thread real actor values through yet** — that's Tasks 14-16. Change each to
`..., nil, nil, nil, "", "")` for now, run `go build ./...` to confirm the whole module compiles, then leave a
`// TODO(task-14/15/16): thread real userID/userName` comment... **actually, do not leave TODO comments per the
no-placeholders plan rule — instead, do Tasks 14, 15, and 16 as part of finishing this task's compile step**, since
they're small and this task cannot be verified green (`go build ./...`) without them. Proceed directly into Task
14/15/16 below before running Step 5's full-build check.

- [ ] **Step 5: Run test and full build**

Run:
`cd api && go test ./internal/repositories/... -run TestDocumentEventRepository_CreateEvent_StampsActor -v && go build ./...`
Expected: PASS, builds cleanly (only true once Tasks 14-16's call-site updates are also in place — see Step 4's note).

- [ ] **Step 6: Commit**

```bash
git add api/internal/repositories/dfe_events.go api/internal/repositories/dfe_events_test.go
git commit -m "feat(api): DocumentEventRepository.CreateEvent stamps actor fields"
```

---

### Task 14: NF-e — actor fields on emit + events

**Files:**

- Modify: `api/internal/services/nfes/emit.go:103` (`Emit` signature + record literal at line ~269-291),
  `api/internal/services/nfes/service.go:145,202,246` (`Cancel`/`CorrectionLetter`/`Manifestation` signatures + their
  `CreateEvent` calls at lines 166, 219, 260), `api/internal/api/v1/nfes.go`
- Test: `api/tests/integration/nfes_audit_test.go` (create if no equivalent exists — check `api/tests/integration/` for
  an existing NF-e emission integration test first and extend it instead of duplicating setup)

**Interfaces:**

- Produces: `NfeService.Emit`/`Cancel`/`CorrectionLetter`/`Manifestation` gain `userID, userName string` parameters
  (appended last, after existing params).

- [ ] **Step 1: Write the failing test**

Extend whichever existing NF-e emission integration test covers `svc.Emit` (find it:
`grep -rln "NfeService\|\.Emit(" api/tests/integration/`) with an assertion that the created record's `user_id`/
`user_name` attributes match what was passed in. If none exists, create `api/tests/integration/nfes_audit_test.go`
following the harness pattern established in Task 7 Step 2, calling
`svc.Emit(ctx, orgPK, emitBody, "user-1", "Jane Doe")` and asserting `nfe["user_id"]`/`nfe["user_name"]`.

- [ ] **Step 2: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Nfe.*Audit|Emit.*Actor' -v`
Expected: FAIL — compile error (argument count mismatch).

- [ ] **Step 3: Update `Emit`'s signature and record literal**

In `api/internal/services/nfes/emit.go:103`:

```go
func (s *NfeService) Emit(ctx context.Context, orgPK string, req NfeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
```

In the `nfeRecord` map literal (`api/internal/services/nfes/emit.go:269-291`), add two entries right after
`"created_at"`:

```go
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}
```

- [ ] **Step 4: Update `Cancel`/`CorrectionLetter`/`Manifestation` signatures and their `CreateEvent` calls**

In `api/internal/services/nfes/service.go`:

```go
func (s *NfeService) Cancel(ctx context.Context, orgPK, accessKey, justification string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
```

```go
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCancelamento, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
```

```go
func (s *NfeService) CorrectionLetter(ctx context.Context, orgPK, accessKey, correctionText string, sequenceNumber int, userID, userName string) (map[string]types.AttributeValue, error) {
```

```go
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, TpEventoCCe, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
```

```go
func (s *NfeService) Manifestation(ctx context.Context, orgPK, accessKey, eventType string, sequenceNumber int, justification *string, userID, userName string) (map[string]types.AttributeValue, error) {
```

```go
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, sequenceNumber, StatusPending, nil, nil, nil, userID, userName)
```

- [ ] **Step 5: Update the four route handlers in `api/internal/api/v1/nfes.go`**

Add `userSvc *services.UserService` to `RegisterNFes`'s signature, call `resolveActor(c, userSvc)` in the `POST ""`,
`POST "/:access_key/cancel"`, `POST "/:access_key/correction-letter"`, `POST "/:access_key/manifestation"` handlers
(lines 16, 86, 102, 118), pass `userID, userName` as the trailing args to each `svc.*` call. Update the `RegisterNFes(`
call site (`router.go`) to pass the user service.

- [ ] **Step 6: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/services/nfes/... ./tests/integration/... -run 'Nfe' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/nfes/emit.go api/internal/services/nfes/service.go \
        api/internal/api/v1/nfes.go api/internal/api/v1/router.go \
        api/tests/integration/nfes_audit_test.go
git commit -m "feat(api): stamp actor fields on NF-e issuance and events"
```

---

### Task 15: NFC-e — actor fields on emit + events

Same pattern as Task 14, applied to `api/internal/services/nfes/nfce_emit.go:40` (`Emit`, record literal at lines ~
180-202), `api/internal/services/nfes/nfce_service.go:100,132` (`Cancel`, `Substitute`), `api/internal/api/v1/nfces.go`.

**Files:**

- Modify: `api/internal/services/nfes/nfce_emit.go`, `api/internal/services/nfes/nfce_service.go`,
  `api/internal/api/v1/nfces.go`
- Test: `api/tests/integration/nfces_audit_test.go`

- [ ] **Step 1: Write the failing test** — mirror Task 14 Step 1 for `NfceService.Emit`.

- [ ] **Step 2: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Nfce.*Audit' -v`
Expected: FAIL — compile error.

- [ ] **Step 3: Update `Emit`'s signature (`nfce_emit.go:40`) and record literal (lines ~180-202)** — same shape as Task
  14 Step 3.

- [ ] **Step 4: Update `Cancel`/`Substitute` signatures and `CreateEvent` calls in `nfce_service.go:100,132`** — same
  shape as Task 14 Step 4. Note `Substitute` — read its full body first (only `Cancel` was shown during planning) to
  find its `CreateEvent` call site before editing.

- [ ] **Step 5: Update the route handlers in `api/internal/api/v1/nfces.go`** (`POST ""` line 17,
  `POST "/:access_key/cancel"` line 102, `POST "/:access_key/substitute"` line 118) — same shape as Task 14 Step 5.

- [ ] **Step 6: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/services/nfes/... ./tests/integration/... -run 'Nfce' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/nfes/nfce_emit.go api/internal/services/nfes/nfce_service.go \
        api/internal/api/v1/nfces.go api/internal/api/v1/router.go \
        api/tests/integration/nfces_audit_test.go
git commit -m "feat(api): stamp actor fields on NFC-e issuance and events"
```

---

### Task 16: MDF-e — actor fields on emit + events

Same pattern, applied to `api/internal/services/mdfes/emit.go:154` (`Emit`) + `:603` (`buildRecord`, record literal at
lines ~639-669), `api/internal/services/mdfes/events.go:85` (`dispatchEvent` — the single shared `CreateEvent` call site
for `Cancel`/`Close`/`IncludeCondutor`/`IncludeDFe`, at line 89), `api/internal/api/v1/mdfes.go`.

**Files:**

- Modify: `api/internal/services/mdfes/emit.go`, `api/internal/services/mdfes/events.go`, `api/internal/api/v1/mdfes.go`
- Test: `api/tests/integration/mdfes_audit_test.go`

**Interfaces:**

- Produces: `MdfeService.Emit`/`Cancel`/`Close`/`IncludeCondutor`/`IncludeDFe` gain `userID, userName string`
  parameters. Since all four event methods funnel through the single `dispatchEvent` helper, only `dispatchEvent` and
  the four public methods need changing — `buildEventEnvelope` is untouched.

- [ ] **Step 1: Write the failing test** — mirror Task 14 Step 1 for `MdfeService.Emit`.

- [ ] **Step 2: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run 'Mdfe.*Audit' -v`
Expected: FAIL — compile error.

- [ ] **Step 3: Update `Emit` (`emit.go:154`) to accept and thread `userID, userName` into `buildRecord`**

`Emit`'s signature:

```go
func (s *MdfeService) Emit(ctx context.Context, orgPK string, req MdfeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
```

`buildRecord`'s signature (`emit.go:603-607`) gains two trailing params:

```go
func (s *MdfeService) buildRecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, modal, docType string,
	cargo *resolvedCargo, vehicle resolvedVehicle, owner *resolvedOwner, req MdfeEmitBody,
	userID, userName string,
) map[string]any {
```

And its `record` literal (lines 639-669), add two entries right after `"created_at"`:

```go
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}
```

Update the call site inside `Emit` where `buildRecord` is invoked (search for `s.buildRecord(` within `emit.go`) to pass
the new trailing args.

- [ ] **Step 4: Update `dispatchEvent` (`events.go:85-89`) and its four callers**

```go
func (s *MdfeService) dispatchEvent(
	ctx context.Context, ec *eventContext, accessKey, eventType string, seq int,
	body map[string]any, newDocStatus string, userID, userName string,
) (map[string]types.AttributeValue, error) {
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, StatusPending, nil, nil, nil, userID, userName)
```

Update `Cancel` (`events.go:133`), `Close` (`:157`), `IncludeCondutor` (`:189`), `IncludeDFe` (`:208`) to each gain
trailing `userID, userName string` params and pass them through to their `s.dispatchEvent(...)` call.

- [ ] **Step 5: Update the route handlers in `api/internal/api/v1/mdfes.go`** (`POST ""` line 17,
  `POST "/:access_key/cancel"` line 116, `POST "/:access_key/close"` line 132, `POST "/:access_key/include-condutor"`
  line 149, `POST "/:access_key/include-dfe"` line 166) — same `resolveActor` pattern as Task 14 Step 5.

- [ ] **Step 6: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/services/mdfes/... ./tests/integration/... -run 'Mdfe' -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/mdfes/emit.go api/internal/services/mdfes/events.go \
        api/internal/api/v1/mdfes.go api/internal/api/v1/router.go \
        api/tests/integration/mdfes_audit_test.go
git commit -m "feat(api): stamp actor fields on MDF-e issuance and events"
```

---

## Part D — Worker SYSTEM actor

### Task 17: Worker — SYSTEM-attributed audit row for auto-created suppliers

**Files:**

- Create: `worker/internal/service/audit.go`
- Modify: `worker/internal/service/distribution.go:751-785` (`persistPerson`)
- Test: `worker/internal/service/audit_test.go`, extend whatever existing test covers `persistPerson` (find it:
  `grep -rln "persistPerson\|TestDistributionService" worker/internal/service/*_test.go`)

**Interfaces:**

- Consumes: `s.dynamo` (`*dynamodb.Client`, already a `DistributionService` field per `distribution.go:775`),
  `s.cfg.TablePrefix`.
- Produces:
  ```go
  func buildAuditLogTxItem(tablePrefix, orgPK, resourceType, resourceID, action string, modifications []auditModification) types.TransactWriteItem
  ```
  (worker has no shared `Base`/`repositories` abstraction like `api` — this is a small standalone helper in the
  `service` package, matching worker's existing direct-`dynamo`-client style seen in `persistPerson` itself.)

- [ ] **Step 1: Write the failing test**

```go
package service

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestBuildAuditLogTxItem(t *testing.T) {
	txItem := buildAuditLogTxItem("dev_dfe", "CNPJ_12345678000195", "PERSON", "CNPJ_99999999000191", "CREATE", nil)
	if txItem.Put == nil {
		t.Fatal("expected Put transact item")
	}
	if *txItem.Put.TableName != "dev_dfe_audit_logs" {
		t.Errorf("table = %q, want dev_dfe_audit_logs", *txItem.Put.TableName)
	}
	item := txItem.Put.Item
	if item["user_id"].(*types.AttributeValueMemberS).Value != "SYSTEM" {
		t.Errorf("user_id = %v, want SYSTEM", item["user_id"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd worker && go test ./internal/service/... -run TestBuildAuditLogTxItem -v`
Expected: FAIL — `buildAuditLogTxItem` undefined (compile error).

- [ ] **Step 3: Implement `worker/internal/service/audit.go`**

```go
package service

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Mirrors api/internal/repositories/audit_logs.go's schema and constants —
// worker has no shared repository layer with api, so this is a standalone
// equivalent for the one write path worker needs (auto-created suppliers).
const (
	auditActionCreate   = "CREATE"
	auditResourcePerson = "PERSON"
	systemActorID       = "SYSTEM"
	systemActorName     = "Sistema (Distribuição DFe)"
)

type auditModification struct {
	Name   string `dynamodbav:"name"`
	Before any    `dynamodbav:"before"`
	After  any    `dynamodbav:"after"`
}

// buildAuditLogTxItem returns a TransactWriteItem writing one audit_logs row,
// for composing into the same transaction as the resource it documents.
func buildAuditLogTxItem(tablePrefix, orgPK, resourceType, resourceID, action string, modifications []auditModification) types.TransactWriteItem {
	modsAV, _ := attributevalue.MarshalList(modifications) // modifications is always a small, well-typed local slice — marshal cannot fail here
	item := map[string]types.AttributeValue{
		"pk":            &types.AttributeValueMemberS{Value: orgPK},
		"sk":            &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%s#%s", resourceType, resourceID, genULID())},
		"resource_type": &types.AttributeValueMemberS{Value: resourceType},
		"resource_id":   &types.AttributeValueMemberS{Value: resourceID},
		"action":        &types.AttributeValueMemberS{Value: action},
		"modifications": &types.AttributeValueMemberL{Value: modsAV},
		"user_id":       &types.AttributeValueMemberS{Value: systemActorID},
		"user_name":     &types.AttributeValueMemberS{Value: systemActorName},
		"created_at":    &types.AttributeValueMemberS{Value: nowStr()},
	}
	return types.TransactWriteItem{
		Put: &types.Put{
			TableName: aws.String(tablePrefix + "_audit_logs"),
			Item:      item,
		},
	}
}
```

(`genULID` and `nowStr` already exist in this package — `persistEvent` uses `genULID()` at `distribution.go:795`,
and `now.UTC().Format(time.RFC3339Nano)` is used inline at `distribution.go:761` for `persistPerson`'s own timestamp;
check whether a `nowStr()` helper already exists in `worker/internal/service/` before adding a new one — if not, either
add a one-line `nowStr()` returning that same format, or inline `time.Now().UTC().Format(time.RFC3339Nano)` directly in
the item map above instead of introducing a new helper name.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd worker && go test ./internal/service/... -run TestBuildAuditLogTxItem -v`
Expected: PASS.

- [ ] **Step 5: Rewrite `persistPerson` to write both items transactionally**

Replace `worker/internal/service/distribution.go:775-784` (the `s.dynamo.PutItem` call and its error handling) with:

```go
	personTx := types.TransactWriteItem{
		Put: &types.Put{
			TableName:           aws.String(table),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(pk)"),
		},
	}
	auditTx := buildAuditLogTxItem(s.cfg.TablePrefix, orgPK, auditResourcePerson, sk, auditActionCreate, personToModifications(item))

	_, err := s.dynamo.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{personTx, auditTx},
	})
	if err != nil {
		// Most commonly the conditional check failed because the person already
		// exists — expected and harmless. A transaction-cancelled error from the
		// audit side (rather than the condition) would be a real bug, but is
		// logged identically here since it's not actionable in this fire-and-forget path.
		slog.Debug("person not persisted (already exists or put failed)", "sk", sk, "org_pk", orgPK, "err", err)
	}
```

Add a small `personToModifications` helper right below `persistPerson` in the same file:

```go
// personToModifications turns a freshly-built person item into a CREATE
// modification list (every field, before=nil), for the audit row.
func personToModifications(item map[string]types.AttributeValue) []auditModification {
	mods := make([]auditModification, 0, len(item))
	for k, v := range item {
		if k == "pk" || k == "sk" || k == "created_at" || k == "updated_at" {
			continue
		}
		var plain any
		_ = attributevalue.Unmarshal(v, &plain)
		mods = append(mods, auditModification{Name: k, Before: nil, After: plain})
	}
	return mods
}
```

- [ ] **Step 6: Run the existing `persistPerson` test (s) plus the new build**

Run: `cd worker && go build ./... && go test ./internal/service/... -run 'Person|Audit' -v`
Expected: PASS. If an existing `persistPerson` test asserts on `PutItem` call count/args via a mock, update it to expect
`TransactWriteItems` instead — check for that mock (`grep -rn "PutItem" worker/internal/service/*_test.go`) before
assuming Step 5 doesn't break anything.

- [ ] **Step 7: Commit**

```bash
git add worker/internal/service/audit.go worker/internal/service/audit_test.go \
        worker/internal/service/distribution.go
git commit -m "feat(worker): attribute auto-created suppliers to SYSTEM in audit_logs"
```

---

## Part E — Read endpoint + UI

### Task 18: `GET /v1.0/audit-logs` — OWNER/ADMIN-gated read endpoint

**Files:**

- Create: `api/internal/api/v1/audit_logs.go`
- Modify: `api/internal/api/v1/router.go` (register the new route group), `api/internal/app/app.go` (add
  `*repositories.AuditLogRepository` to the route-layer `Services` struct if the route needs the repo directly, or add a
  thin `AuditLogService` — prefer the latter to keep the route layer repo-free, matching every other resource's
  layering)
- Test: `api/tests/integration/audit_logs_read_test.go`

**Interfaces:**

- Consumes: `AuditLogRepository` (Task 3), `middleware.UserOrgRole`, `roleOwner`/`roleAdmin` constants (
  `middleware/rbac.go:19-20` — these are unexported; either export them or add the OWNER/ADMIN check as a small new
  exported helper in `middleware/rbac.go` itself, e.g.
  `func RequireOwnerOrAdmin(userSvc *services.UserService) fiber.Handler`, rather than reaching into unexported
  constants from the `v1` package).
- Produces: `GET /v1.0/audit-logs?resource_type=&resource_id=&user_id=&cursor=&limit=` → paginated `audit_logs` rows for
  the active org.

- [ ] **Step 1: Add `AuditLogService` (thin query wrapper, mirrors `ProductService.List`'s shape)**

Create the service in `api/internal/services/audit.go` (same file as `Diff`, from Task 4) — add below `Diff`:

```go
// AuditLogQueryOpts selects which audit_logs index to query.
type AuditLogQueryOpts struct {
	ResourceType string // with ResourceID: base-table query, full history of one resource
	ResourceID   string
	UserID       string // user-id-index: everything one user did
	Limit        int
	StartKey     map[string]types.AttributeValue
}

// AuditLogService lists audit_logs rows for an org.
type AuditLogService struct {
	repo *repositories.AuditLogRepository
}

func NewAuditLogService(repo *repositories.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

// List picks the right index based on which filters are set: base table
// (resource history) > user-id-index (per-user) > org-time-index (default feed).
func (s *AuditLogService) List(ctx context.Context, orgPK string, opts AuditLogQueryOpts) (*repositories.QueryResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if opts.ResourceType != "" && opts.ResourceID != "" {
		return s.repo.Query(ctx, repositories.QueryOpts{
			PK: orgPK, SKPrefix: fmt.Sprintf("%s#%s#", opts.ResourceType, opts.ResourceID),
			ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	if opts.UserID != "" {
		return s.repo.Query(ctx, repositories.QueryOpts{
			PK: opts.UserID, PKField: "user_id", IndexName: "user-id-index",
			ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return s.repo.Query(ctx, repositories.QueryOpts{
		PK: orgPK, IndexName: "org-time-index",
		ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
	})
}
```

Note: `Query`'s `QueryOpts.PK`/`PKField` combination for the `user-id-index` case queries by `user_id` as the partition
key directly (not `orgPK`) — this means a `user_id` filter is **not** org-scoped by the query itself. Add an explicit
post-filter in the route handler (Step 3) discarding any row whose `pk` (org) doesn't match the caller's active org,
since a GSI query can't apply a second equality condition on a non-key attribute without a `FilterExpression` — check
whether `Base.Query`/`QueryOpts` already supports a `FilterExpression` passthrough before adding a manual post-filter;
if it does, prefer `FilterExpression: "pk = :org_pk"` over an in-memory filter.

- [ ] **Step 2: Write the failing integration test**

```go
func TestAuditLogService_List_ByOrgTimeIndex(t *testing.T) {
	db, cfg := setupTestDB(t) // ensure it creates audit_logs + both GSIs
	auditRepo := repositories.NewAuditLogRepository(db, cfg)
	svc := services.NewAuditLogService(auditRepo)
	ctx := context.Background()
	orgPK := "CNPJ_12345678000195"

	txItem, _ := auditRepo.BuildLogTxItem(orgPK, "PRODUCT", "PRODUCT_1", "CREATE", "user-1", "Jane", nil)
	if err := auditRepo.TransactWrite(ctx, []types.TransactWriteItem{txItem}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := svc.List(ctx, orgPK, services.AuditLogQueryOpts{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(res.Items))
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
`cd api && DYNAMODB_ENDPOINT=http://localhost:8000 go test ./tests/integration/... -tags integration -run TestAuditLogService_List_ByOrgTimeIndex -v`
Expected: FAIL — compile error.

- [ ] **Step 4: Add `RequireOwnerOrAdmin` to `api/internal/middleware/rbac.go`**

Add after `Require` (after line 47):

```go
// RequireOwnerOrAdmin returns a Fiber handler that allows only OWNER/ADMIN org
// members, bypassing the granular permission-string check entirely — for
// endpoints like the audit trail where visibility itself is the sensitive
// thing, not a specific action.
func (p *PermChecker) RequireOwnerOrAdmin() fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := GetUserID(c)
		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("missing user identity"))
		}
		foundOrgPK := c.Get(OrgHeader)
		if foundOrgPK == "" {
			foundOrgPK = c.Params(OrgPKKey)
		}
		if foundOrgPK == "" {
			return c.Status(fiber.StatusBadRequest).JSON(problem.BadRequest("missing organization: " + OrgHeader))
		}
		orgPK, err := repositories.ParseOrgPK(foundOrgPK)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(problem.BadRequest("invalid organization: " + foundOrgPK))
		}
		user, err := p.userSvc.GetMe(c.Context(), userID)
		if err != nil {
			return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Acesso negado"))
		}
		roleName, ok := UserOrgRole(user, orgPK)
		if !ok || (roleName != roleOwner && roleName != roleAdmin) {
			return c.Status(fiber.StatusForbidden).JSON(problem.Forbidden("Apenas proprietários e administradores podem ver o log de auditoria"))
		}
		c.Locals(OrgPKKey, orgPK)
		return c.Next()
	}
}
```

- [ ] **Step 5: Create `api/internal/api/v1/audit_logs.go`**

```go
package v1

import (
	"gopkg.aoctech.app/dfe/api/internal/middleware"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gofiber/fiber/v3"
)

// RegisterAuditLogs mounts /audit-logs — OWNER/ADMIN-only, org-scoped via the
// active-org header like every other top-level resource.
func RegisterAuditLogs(router fiber.Router, svc *services.AuditLogService, authMw fiber.Handler, perm *middleware.PermChecker) {
	g := router.Group("/audit-logs", authMw, perm.RequireOwnerOrAdmin())

	// GET /audit-logs
	g.Get("", func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		res, err := svc.List(c.Context(), middleware.GetOrgPK(c), services.AuditLogQueryOpts{
			ResourceType: c.Query("resource_type"),
			ResourceID:   c.Query("resource_id"),
			UserID:       c.Query("user_id"),
			Limit:        intQuery(c, "limit", 50),
			StartKey:     decodeCursor(cursor),
		})
		if err != nil {
			return sendProblem(c, err)
		}
		items := make([]map[string]any, 0, len(res.Items))
		for _, it := range res.Items {
			m, err := unmarshal(it)
			if err != nil {
				return sendProblem(c, err)
			}
			items = append(items, m)
		}
		_ = types.AttributeValueMemberS{} // placeholder import anchor — remove if unmarshal/sendPage already cover typing without this
		return sendPage(c, res, cursor)
	})
}
```

(Drop the placeholder `_ = types.AttributeValueMemberS{}` line — it was left in during drafting to flag that `types` may
end up unused once you confirm `unmarshal`'s signature; check whether `items` is actually needed or whether
`sendPage(c, res, cursor)` already serializes `res.Items` correctly the way every other list route in this codebase does
it — e.g. `products.go`'s `GET ""` handler just calls `sendPage(c, res, cursor)` directly on the raw
`*repositories.QueryResult` without a manual `unmarshal` loop. Follow that exact existing pattern instead of the manual
loop above — check `sendPage`'s implementation in `helpers.go` to confirm it already unmarshals `res.Items` internally
before writing this handler's final form.)

- [ ] **Step 6: Wire `AuditLogService` into fx and register the route**

In `api/internal/app/app.go`: add `services.NewAuditLogService` to the `fx.Provide` services list (near
`newProductService`); add `AuditLogSvc *services.AuditLogService` to the `Services` struct (`app.go:306-329`) and pass
it through `registerRoutes` into `apiv1.Services{..., AuditLog: svcs.AuditLogSvc}`.

In `api/internal/api/v1/router.go`: find the `Services` struct there (separate from `app.go`'s) and add an
`AuditLog *services.AuditLogService` field; find `Register(...)` and add a call to
`RegisterAuditLogs(router, svcs.AuditLog, authMw, perm)` alongside the other `Register*` calls.

- [ ] **Step 7: Run tests and full build**

Run: `cd api && go build ./... && go test ./internal/... ./tests/integration/... -run 'AuditLog' -v`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add api/internal/services/audit.go api/internal/middleware/rbac.go \
        api/internal/api/v1/audit_logs.go api/internal/api/v1/router.go \
        api/internal/app/app.go api/tests/integration/audit_logs_read_test.go
git commit -m "feat(api): add GET /v1.0/audit-logs, OWNER/ADMIN only"
```

---

### Task 19: UI — `AuditLogOut` type + `ApiClient.listAuditLogs`

**Files:**

- Modify: `ui/src/lib/types/api.ts`, `ui/src/lib/api/client.ts`
- Test: `ui/src/lib/api/__tests__/client.test.ts` (check the exact test file location/naming by looking at how an
  existing method like `listProducts` is tested — `grep -rln "listProducts" ui/src/**/__tests__/* ui/src/**/*.test.ts`)

**Interfaces:**

- Produces:
  ```typescript
  export interface AuditLogModification { name: string; before: unknown; after: unknown }
  export interface AuditLogOut {
    pk: string; sk: string; resource_type: string; resource_id: string;
    action: 'CREATE' | 'UPDATE' | 'DELETE';
    modifications: AuditLogModification[];
    user_id: string; user_name: string; created_at: string;
  }
  listAuditLogs(params: { resourceType?: string; resourceId?: string; userId?: string; cursor?: string; limit?: number }): Promise<PaginatedResponse<AuditLogOut>>
  ```

- [ ] **Step 1: Find and read the existing test for a comparable list method (e.g. `listProducts` or `listVehicles`) to
  match its exact test style**

Run: `grep -rln "listProducts\|listVehicles" ui/src --include='*.test.ts'`

- [ ] **Step 2: Write the failing test**, following that file's structure — mock `axios`, call
  `apiClient.listAuditLogs({})`, assert it hit `GET /v1.0/audit-logs` with the right query params and returned the
  parsed `PaginatedResponse<AuditLogOut>`.

- [ ] **Step 3: Run test to verify it fails**

Run: `cd ui && npm test -- --run <the test file from Step 2>`
Expected: FAIL — `listAuditLogs` undefined.

- [ ] **Step 4: Add `AuditLogModification`/`AuditLogOut` to `ui/src/lib/types/api.ts`**, placed
  alphabetically/thematically near other `*Out` types (check the file's existing ordering convention first).

```typescript
export interface AuditLogModification {
  name: string
  before: unknown
  after: unknown
}

export interface AuditLogOut {
  pk: string
  sk: string
  resource_type: string
  resource_id: string
  action: 'CREATE' | 'UPDATE' | 'DELETE'
  modifications: AuditLogModification[]
  user_id: string
  user_name: string
  created_at: string
}
```

- [ ] **Step 5: Add `listAuditLogs` to `ApiClient` in `ui/src/lib/api/client.ts`**, following the exact style of the
  nearest existing list method (e.g. `listProducts`) for how it builds query params and calls `this.axios.get`:

```typescript
async listAuditLogs(params: {
  resourceType?: string
  resourceId?: string
  userId?: string
  cursor?: string
  limit?: number
}): Promise<PaginatedResponse<AuditLogOut>> {
  const { data } = await this.axios.get<PaginatedResponse<AuditLogOut>>('/v1.0/audit-logs', {
    params: {
      resource_type: params.resourceType,
      resource_id: params.resourceId,
      user_id: params.userId,
      cursor: params.cursor,
      limit: params.limit,
    },
  })
  return data
}
```

(Add `AuditLogOut` to the `import type {...} from '@/lib/types/api'` block at the top of `client.ts` — check the
existing import list, keep it alphabetically consistent with the rest.)

- [ ] **Step 6: Run test, ESLint**

Run: `cd ui && npm test -- --run` then `npx eslint src --ext .ts,.tsx`
Expected: test PASS; ESLint zero errors/warnings.

- [ ] **Step 7: Commit**

```bash
git add ui/src/lib/types/api.ts ui/src/lib/api/client.ts ui/src/lib/api/__tests__/
git commit -m "feat(ui): add AuditLogOut type and ApiClient.listAuditLogs"
```

---

### Task 20: UI — Audit Log page

**Files:**

- Create: `ui/src/app/audit-logs/page.tsx`
- Test: `ui/src/app/audit-logs/__tests__/page.test.tsx` (or wherever this project's page-level tests conventionally
  live — check `ui/src/app/products/__tests__/` or equivalent first and mirror it exactly)

**Interfaces:**

- Consumes: `ApiClient.listAuditLogs` (Task 19), `AuditLogOut` (Task 19), whatever shared pagination/table/skeleton
  components `ui/src/app/products/page.tsx` already uses (read that file in full before writing this one, and reuse its
  components rather than inventing new ones — per `ui/CLAUDE.md`'s DRY rule).

- [ ] **Step 1: Read `ui/src/app/products/page.tsx` in full**, noting: which layout/table/pagination/skeleton components
  it imports, how it handles the debounced filter inputs, how it shows loading state, and how it gates on role (check
  whether any existing page already does an OWNER/ADMIN-only gate client-side, e.g. a settings page — reuse that
  pattern; if none exists, the gate is enforced server-side by Task 18's `RequireOwnerOrAdmin` returning 403, and the
  page just needs to render that 403 sensibly, e.g. via the same error-state component `products/page.tsx` uses for a
  failed fetch).

- [ ] **Step 2: Write the failing component test**, mirroring whichever test file was found in Step 1's search,
  asserting: the page renders a loading skeleton first, then calls `listAuditLogs`, then renders rows once resolved.

- [ ] **Step 3: Run test to verify it fails**

Run: `cd ui && npm test -- --run <test file from Step 2>`
Expected: FAIL — module not found.

- [ ] **Step 4: Implement `ui/src/app/audit-logs/page.tsx`**

Build it as a client component (`'use client'`) following `products/page.tsx`'s exact structure:

- A filter row (resource type `<Select>`, date — reuse whatever date-range control an existing page already has, e.g.
  `nfe/page.tsx`'s year/month/day filters, rather than adding a new one) using `DebouncedInput`/300ms debounce per
  `ui/CLAUDE.md`.
- A table (desktop) / card list (mobile, per the mobile-first rule) showing columns: timestamp, resource type + id,
  action, user_name, and an expandable row or a "view changes" action showing the `modifications` list (
  `name: before → after`).
- Cursor pagination reusing whatever pagination component/hook `products/page.tsx` uses.
- A skeleton for initial load, a subtle dimming/spinner for filter refetches (mandatory per `ui/CLAUDE.md`'s Loading
  States rule).
- `min-h-11` (44px) touch targets on any interactive row/button, `grid-cols-1 md:grid-cols-N` responsive filter layout,
  per the Mobile-First rules.

Do not write the full JSX here — write it directly in the file by copying `products/page.tsx`'s structure and adapting
field names, since this plan cannot predict `products/page.tsx`'s exact current shape without you reading it in Step 1.
The one non-negotiable structural requirement: no new pagination/table/skeleton component gets invented if
`products/page.tsx` (or a shared component it imports from `ui/src/components/ui/`) already has one that fits.

- [ ] **Step 5: Add the nav entry**

Find wherever the sidebar/nav lists `products`/`vehicles`/`persons`/`certificates` (
`grep -rln "\"Produtos\"\|'/products'" ui/src/components/layout/`) and add an "Audit Log"/"Log de Auditoria" entry
pointing at `/audit-logs`, gated the same way any existing OWNER/ADMIN-only nav entry is gated (if none exists yet, show
it unconditionally and rely on the page's own 403 handling from Step 1 — do not invent a new client-side role-gating
mechanism for just this entry).

- [ ] **Step 6: Run test, ESLint, and manually verify in the browser**

Run: `cd ui && npm test -- --run && npx eslint src --ext .ts,.tsx`
Expected: PASS, zero ESLint errors/warnings.

Then start the dev server and manually exercise the page (per the root `CLAUDE.md`'s UI verification rule):
`cd ui && npm run dev`, log in, navigate to `/audit-logs`, confirm the list loads, filters debounce, pagination works,
and mobile viewport (375px) has no horizontal overflow.

- [ ] **Step 7: Commit**

```bash
git add ui/src/app/audit-logs/ ui/src/components/layout/
git commit -m "feat(ui): add org-wide Audit Log page"
```

---

## Part F — Docs

### Task 21: Update cross-project documentation

**Files:**

- Modify: `DynamoDB-Tables.md`, `DOCS.md`, `INTEGRATION.md`, `CONDUCT.md`

- [ ] **Step 1: `DynamoDB-Tables.md`** — add `audit_logs` to the table index table (near `organizations`), and a full
  section (pk/sk/attributes/GSIs) matching the style of the existing `organization_products`/`nfe_events` sections,
  using the exact schema from Task 3.

- [ ] **Step 2: `DOCS.md`** — document `GET /v1.0/audit-logs` in the endpoints reference (§4, alongside the other `api`
  endpoints), and note the new `user_id`/`user_name` fields on `nfes`/`nfces`/`mdfes` and their `*_events` tables.

- [ ] **Step 3: `INTEGRATION.md`** — document `ApiClient.listAuditLogs` alongside the other list methods.

- [ ] **Step 4: `CONDUCT.md`** — add a short entry describing the `Build*TxItem` + `Base.TransactWrite` pattern as the
  required approach for any future mutation that needs an atomic audit row, so the next engineer doesn't reinvent it or
  fall back to a non-atomic best-effort write.

- [ ] **Step 5: Commit**

```bash
git add DynamoDB-Tables.md DOCS.md INTEGRATION.md CONDUCT.md
git commit -m "docs: document audit_logs table, endpoint, and transactional-audit pattern"
```

---

## Self-review notes

- **Spec coverage:** every design-doc section has a task — data model (Task 1, 3), in-record fields (Tasks 13-16),
  transactional write pattern (Task 2, applied throughout Part B), actor resolution (Task 5), SYSTEM actor (Task 17),
  read endpoint + RBAC (Task 18), UI (Tasks 19-20), docs (Task 21).
- **Known unresolved specifics deferred to task-execution time, by design** (not placeholders — each is a concrete "read
  this file, confirm X, then proceed" instruction, because the exact current shape of some files wasn't fully read
  during planning): persons' true key shape (Task 9 Step 1), certificates' exact `Create`/`Delete` signatures (Task 10
  Step 1), fiscal config repositories' `Upsert` behavior (Task 12 Step 1), NFC-e's `Substitute` event call site (Task 15
  Step 4), and the UI page's exact component reuse (Task 20 Step 1). Each of these tasks starts with a read/verify step
  specifically because guessing the code would violate the "no placeholders" rule worse than asking the implementer to
  look first.
- **Type consistency:** `userID, userName string` is appended as the last two parameters on every touched method,
  consistently, across Parts B and C. `repositories.Modification` (Task 3) is the one shared type used by `Diff` (Task
    4) and every `BuildLogTxItem` call. Worker (Task 17) intentionally does **not** import `api`'s types — it's a
       separate Go module — and instead has its own tiny mirror (`auditModification`), documented as such.
