# The DF-e subscription belongs to the organization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move the DF-e subscription, its quotas and its usage counters from the owner's user (`USER_{sub}`) to the ctech-account organization that holds the companies (`ORG_{organization_id}`), managed by that organization's `owner`/`admin`, with the company quota checked when a company is first enabled, a one-shot migration of the existing subscriptions, and a dual-read window that a final, separate deploy removes.

**Architecture:** Every billing decision now starts from a **company** (the `Dfe-Organization-Pk` header or the document's `org_pk`) and resolves its ctech-account organization from the local company record's `organization_id` (written by the re-key and by `LinkService.Link`). The snapshot, counters and guard are keyed by that id in `account_billing`; a new sparse GSI `organization-index` on `{env}_organizations` lists an organization's companies (count of enabled companies, and the billing company sent to ctech-billing). Who may manage the plan is read from ctech-account's membership route through a new `accountclient.WorkspaceClient` and a cached, fail-closed `WorkspaceRoleService`. The company quota moves from the legacy create route to the first fiscal configuration of a company (`ReserveCompany`, whose guard item joins the configuration's own transaction, together with a dirty marker for the organization's `dfe_companies` level); a `LevelReporter` delivers that level (the whole count of enabled companies) to billing right after the commit and from a sweeper, so the on-demand price bills the monthly peak and no report is lost. `cmd/migrate-billing-org` moves the two existing R$ 0 subscriptions; Phase 2 deletes the `USER_` fallback.

**Tech Stack:** Go 1.27, Fiber v3, DynamoDB (aws-sdk-go-v2), `gopkg.aoctech.app/api-commons` (cache, oauth2client, dynamo); AWS CDK (TypeScript, jest); Next.js 16 + React Query + vitest (`ui/`).

**Spec:** [`docs/specs/2026-10-10-organization-subscription.md`](../specs/2026-10-10-organization-subscription.md). Upstream (ships first): `ctech-billing/docs/specs/2026-10-10-plans-design.md` § 8 (organization customers, `CUSTOMER_ORG#` pointer) § 6 (level metering, `POST /v1.0/usage/levels`) and § 10 step 1 (catalogue drops `quota_users`, archives `price_dfe_ondemand_user`; adds `price_dfe_ondemand_companies_monthly`, archives `price_dfe_ondemand_company`). The spec's section "Amendment, planning (2026-10-10)" records the decisions this plan makes where the spec was silent or contradicted the code. Supersedes D1/D1a of [`2026-08-16-assinaturas-billing-dfe.md`](2026-08-16-assinaturas-billing-dfe.md).

## Global Constraints

- Customer `ExternalRef` is `ORG_{organization_id}`; snapshot `pk = ORG_{organization_id}`; counters `pk = USAGE_{organization_id}#{period}`; company guard `pk = QUOTA_GUARD_{organization_id}#companies`. The 13-month counter TTL and the conditional `ADD` against the limit are unchanged.
- Managing (choose, change, cancel, pay, list invoices) requires the ctech-account role `owner` or `admin` in the company's organization; everyone else with access to the company gets **403** on those and can read the plan and usage. The role read **fails closed** (an outage is a 403, never a grant), like reach.
- The subscription gate keeps its semantics: fail-open when the snapshot cannot be read; only `ACTIVE`/`TRIALING` grant.
- The company quota is checked when a company becomes enabled (first fiscal configuration of a company with none). Over the limit, nothing that exists is removed or blocked; only enabling another company is refused, with 402 `quota_exceeded`.
- The on-demand company charge is **monthly by peak** (planning amendment of 2026-10-10): the DF-e reports the level `dfe_companies` = enabled companies of the organization to `POST /v1.0/usage/levels` with `customer_ref: ORG_{organization_id}`, the whole count, never a delta, on every enablement change, whatever the plan; delivery is durable (dirty marker in the same transaction + sweeper). No one-shot `company:{companyPK}` usage is reported any more.
- `LinkService.Link` stays unchecked by billing.
- Migration is dry-run by default (`-apply` to write), idempotent, and stops (lists, skips) any user whose entitled subscription has an item whose `unit_amount` is not 0.
- Dual read (Phase 1 only): snapshot by `ORG_`, falling back to the company owner's `USER_` snapshot while the window is open; usage and guard **writes go to `ORG_` only**. Phase 2 (Task 17) is its own deploy.
- Tasks 1–16 ship as **one** deploy (spec deploy step 2). Intermediate commits compile and pass their tests but are not deployable on their own: until Task 6 lands, the write routes still pass a user id where an organization id is now expected.
- `quota_users` leaves the UI; the catalogue change itself is ctech-billing's.
- Every key, header, role, scope and route string is a named constant (`CLAUDE.md`, "Constants").
- API errors are RFC 7807 via `problem.*` only.
- No travessão (`—`) in UI-visible text, guide copy, commit messages or PR text. Code comments and technical docs may use it.
- UI tasks (13, 14, 15) are executed with the `/impeccable` skill. Gates: `npx eslint src --ext .ts,.tsx` with zero errors and zero warnings; `npx vitest run --maxWorkers=2 <files>` judged by its **exit code**; guide updated and `npm run screens:capture` where a screen changed.
- Docs move with code: `DOCS.md` (endpoints, behaviour), `DynamoDB-Tables.md` (keys, GSI), `DEPLOYMENT.md` (credentials, order). The OpenAPI files under `api/internal/api/v1/openapi/` must keep `openapi_test.go` green.
- Commits: Conventional Commits, no attribution trailer of any kind.
- All Go commands run from `api/` unless a step says otherwise; integration tests need DynamoDB Local (`docker compose -f docker-compose.test.yml up -d`, then `make test-integration` or the `go test -tags integration` line given).

### Cross-repo prerequisites (not tasks in this repo)

1. **ctech-billing** deploys its § 10 step 1: `POST /v1.0/customers` accepts `external_ref: ORG_…` (+ `tax_id`) and writes `CUSTOMER_ORG#`, `GET /v1.0/entitlements?customer_ref=ORG_…` answers, the catalogue loses `quota_users`.
2. **ctech-account**: the DF-e needs a client holding `internal:account:org-member` and `internal:account:user-organizations`. ctech-account has no command that adds scopes to an existing M2M client (`cmd/createclient` only creates), so this plan provisions a **second** DF-e client:
   `go run ./cmd/createclient -client-id ctech-dfe-workspaces -name "ctech-dfe workspaces" -scopes internal:account:org-member,internal:account:user-organizations -ssm-path-client /ctech-dfe/{env}/account-workspace-client-id -ssm-path-secret /ctech-dfe/{env}/account-workspace-client-secret`.
3. The membership client below duplicates `ctech-billing/api/internal/accountclient/membership.go`. Extracting both into `ctech-go-common` is the family-wide follow-up; it is out of scope here and listed in `DEPLOYMENT.md` by Task 16.

## Review Focus

1. **A company with no `organization_id`** (a legacy `CNPJ_`/`CPF_` partition, or a re-keyed row the backfill missed) must never be billed to another organization or crash the gate: during the window it falls back to its owner's `USER_` snapshot; after Phase 2 it reads as "no subscription" (402 on writes); `ReserveCompany` refuses it with 409. Tests: Task 5 `TestACompanyWithNoOrganizationFallsBackToItsOwner`, Task 7 `TestReserveCompanyRefusesACompanyWithNoOrganization`, Task 17 `TestACompanyWithNoOrganizationHasNoPlan`.
2. **ctech-account down while an admin changes the plan** must answer 403 (not 500, not success), and the plan must still be readable with `manageable: false`. Tests: Task 4 `TestRoleOutageIsAnErrorAndNotCached`, Task 6 `TestManagingFailsClosedWhenTheRoleCannotBeRead`.
3. **Two companies of one organization enabled at the same time at `limit - 1`**: exactly one configuration is written, the other is refused (guard version). Test: Task 11 `TestTwoEnablementsRaceForTheLastSlot`.
4. **The migration re-run after a partial failure** (customer and subscription created, cancel failed) creates no second subscription and does not add the counters twice. Test: Task 12 `TestApplyTwiceCreatesNothingTheSecondTime`.
5. **A result message in flight across the deploy** (carrying only `billing_user_id`) must refund the counter it reserved (`USAGE_{sub}#…`), and new messages the organization's. Test: Task 10 `TestRefundUsesTheAccountTheReservationNamed`.

---

## Phase 1: organization billing with the dual read (spec deploy step 2)

### Task 1: The `organization-index` GSI and the company listing

**Files:**
- Modify: `cdk/lib/dynamodb-stack.ts:337-348` (organizations table)
- Test: `cdk/test/dynamodb-stack.test.ts`
- Modify: `api/internal/repositories/organizations.go` (constant, `CompanyRef`, `ListCompaniesOfOrganization`, `CompanyIndexGap`, `ListCompanyIndexGaps`, `BackfillIndexKeys`)
- Modify: `api/tests/integration/setup_test.go:183-192` (GSI on the local table)
- Create: `api/tests/integration/organization_index_test.go`
- Modify: `DynamoDB-Tables.md` (organizations: the GSI)

**Interfaces:**
- Consumes: `repositories.IsCompanyKey`, `repositories.AttrOrganizationID`, `repositories.AttrOwnerUserID`, `Base.QueryRaw`, `Base.ScanRaw`, `Base.UpdateItemRaw`.
- Produces:
  - `const repositories.OrganizationIndex = "organization-index"`
  - `type repositories.CompanyRef struct { PK string; CreatedAt string }`
  - `func (r *OrganizationRepository) ListCompaniesOfOrganization(ctx context.Context, organizationID string) ([]CompanyRef, error)` — company-keyed rows only, oldest `created_at` first.
  - `type repositories.CompanyIndexGap struct { PK, OwnerUserID, CreatedAt string; MissingOrganization, MissingCreatedAt bool }`
  - `func (r *OrganizationRepository) ListCompanyIndexGaps(ctx context.Context) ([]CompanyIndexGap, error)` — company-keyed rows that the GSI cannot see.
  - `func (r *OrganizationRepository) BackfillIndexKeys(ctx context.Context, companyPK, organizationID, createdAt string) error` — sets only what is missing.

- [ ] **Step 1: Write the failing CDK test** — append to `cdk/test/dynamodb-stack.test.ts`

```ts
describe('DynamoDBStack — organizations por workspace', () => {
    test('organizations tem o GSI organization-index (organization_id, created_at), só chaves', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_organizations',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'organization-index',
                    KeySchema: [
                        {AttributeName: 'organization_id', KeyType: 'HASH'},
                        {AttributeName: 'created_at', KeyType: 'RANGE'},
                    ],
                    Projection: {ProjectionType: 'KEYS_ONLY'},
                }),
            ]),
        });
    });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd cdk && npx jest test/dynamodb-stack.test.ts`
Expected: FAIL — "organizations tem o GSI organization-index" (no `GlobalSecondaryIndexes` on the table).

- [ ] **Step 3: Add the GSI** — in `cdk/lib/dynamodb-stack.ts`, right before `this.tables.set('organizations', organizationsTable);`

```ts
    // The company records of one ctech-account organization (ORG_ billing,
    // docs/specs/2026-10-10-organization-subscription.md): the enabled-company
    // count behind `quota_companies`, and the organization's billing company
    // (the first one linked, hence created_at as the sort key).
    //
    // Sparse by construction: the legacy CNPJ_/CPF_ rollback partitions carry no
    // organization_id and never appear. KEYS_ONLY because both readers only need
    // the company pk and its created_at; anything else is one GetItem away.
    organizationsTable.addGlobalSecondaryIndex({
      indexName: 'organization-index',
      partitionKey: {name: 'organization_id', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'created_at', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.KEYS_ONLY,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
```

- [ ] **Step 4: Run the CDK test to verify it passes**

Run: `cd cdk && npx jest test/dynamodb-stack.test.ts`
Expected: PASS.

- [ ] **Step 5: Write the failing integration test** — `api/tests/integration/organization_index_test.go`

```go
//go:build integration

package integration_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// newCompanyPK returns a lowercase canonical UUID, the shape IsCompanyKey
// accepts. Random so tests never share a partition.
func newCompanyPK(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// seedCompany writes a re-keyed company record under organizationID, the way
// Link and the re-key leave it.
func seedCompany(t *testing.T, organizationID, ownerSub, taxID, legalName string) string {
	t.Helper()
	pk := newCompanyPK(t)
	item := map[string]types.AttributeValue{
		repositories.AttrOrganizationID: &types.AttributeValueMemberS{Value: organizationID},
		repositories.AttrOwnerUserID:    &types.AttributeValueMemberS{Value: ownerSub},
		repositories.AttrTaxID:          &types.AttributeValueMemberS{Value: taxID},
		repositories.AttrTaxIDKind:      &types.AttributeValueMemberS{Value: repositories.TaxKindCNPJ},
		repositories.AttrLegalName:      &types.AttributeValueMemberS{Value: legalName},
	}
	if err := orgRepo.CreateOrganization(context.Background(), pk, item); err != nil {
		t.Fatal(err)
	}
	return pk
}

func TestListCompaniesOfOrganizationIsOldestFirstAndCompanyKeyedOnly(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := "org-idx-a-"+newCompanyPK(t), "org-idx-b-"+newCompanyPK(t)

	first := seedCompany(t, orgA, "owner-idx", "11222333000181", "Primeira Ltda")
	second := seedCompany(t, orgA, "owner-idx", "11222333000262", "Segunda Ltda")
	_ = seedCompany(t, orgB, "owner-idx", "44555666000105", "Outra Ltda")

	// A legacy partition that somehow carries an organization_id must not be
	// listed: it is a rollback copy, not a company.
	legacy := "CNPJ_" + randomCNPJ()
	if err := orgRepo.CreateOrganization(ctx, legacy, map[string]types.AttributeValue{
		repositories.AttrOrganizationID: &types.AttributeValueMemberS{Value: orgA},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := orgRepo.ListCompaniesOfOrganization(ctx, orgA)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PK != first || got[1].PK != second {
		t.Fatalf("got %+v, want [%s %s] in creation order", got, first, second)
	}
	if empty, err := orgRepo.ListCompaniesOfOrganization(ctx, ""); err != nil || len(empty) != 0 {
		t.Fatalf("an empty organization id must list nothing, got %+v (%v)", empty, err)
	}
}

func TestIndexGapsAreListedAndBackfilledWithoutOverwriting(t *testing.T) {
	ctx := context.Background()
	pk := newCompanyPK(t)
	// A re-keyed row the GSI cannot see: no organization_id.
	if err := orgRepo.CreateOrganization(ctx, pk, map[string]types.AttributeValue{
		repositories.AttrOwnerUserID: &types.AttributeValueMemberS{Value: "gap-owner"},
	}); err != nil {
		t.Fatal(err)
	}

	gaps, err := orgRepo.ListCompanyIndexGaps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found *repositories.CompanyIndexGap
	for i := range gaps {
		if gaps[i].PK == pk {
			found = &gaps[i]
		}
	}
	if found == nil || !found.MissingOrganization || found.MissingCreatedAt || found.OwnerUserID != "gap-owner" {
		t.Fatalf("gap = %+v", found)
	}

	if err := orgRepo.BackfillIndexKeys(ctx, pk, "org-gap", "2000-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	got, err := orgRepo.ListCompaniesOfOrganization(ctx, "org-gap")
	if err != nil || len(got) != 1 || got[0].PK != pk {
		t.Fatalf("after backfill: %+v (%v)", got, err)
	}
	// created_at existed and must be kept, not replaced by the fallback.
	if got[0].CreatedAt == "2000-01-01T00:00:00Z" {
		t.Fatal("backfill overwrote an existing created_at")
	}
}
```

- [ ] **Step 6: Add the GSI to the local table** — in `api/tests/integration/setup_test.go`, replace the `_organizations` definition (lines 183-192) with:

```go
		{
			TableName:   aws.String(tablePrefix + "_organizations"),
			BillingMode: types.BillingModePayPerRequest,
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("pk"), KeyType: types.KeyTypeHash},
			},
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String(repositories.AttrOrganizationID), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{{
				IndexName: aws.String(repositories.OrganizationIndex),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String(repositories.AttrOrganizationID), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeKeysOnly},
			}},
		},
```

- [ ] **Step 7: Run it to verify it fails**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestListCompaniesOfOrganization|TestIndexGaps' -v`
Expected: FAIL — build error `undefined: repositories.OrganizationIndex` / `orgRepo.ListCompaniesOfOrganization undefined`.

- [ ] **Step 8: Implement** — append to `api/internal/repositories/organizations.go` (add imports `github.com/aws/aws-sdk-go-v2/aws`, `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`)

```go
// OrganizationIndex lists the company records of one ctech-account
// organization, oldest first. Sparse: only rows carrying organization_id (and
// created_at) are in it, so the legacy CNPJ_/CPF_ partitions never are.
const OrganizationIndex = "organization-index"

// attrCreatedAt is the GSI's sort key; CreateOrganization writes it.
const attrCreatedAt = "created_at"

// CompanyRef is what the KEYS_ONLY index answers about one company.
type CompanyRef struct {
	PK        string `dynamodbav:"pk"`
	CreatedAt string `dynamodbav:"created_at"`
}

// ListCompaniesOfOrganization lists the companies of one organization, oldest
// first. It pages internally: an organization's companies are a count and a
// "first linked", and both need the whole list.
func (r *OrganizationRepository) ListCompaniesOfOrganization(ctx context.Context, organizationID string) ([]CompanyRef, error) {
	if organizationID == "" {
		return nil, nil
	}
	var out []CompanyRef
	var start map[string]types.AttributeValue
	for {
		res, err := r.QueryRaw(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(r.TableName),
			IndexName:                 aws.String(OrganizationIndex),
			KeyConditionExpression:    aws.String("#o = :o"),
			ExpressionAttributeNames:  map[string]string{"#o": AttrOrganizationID},
			ExpressionAttributeValues: map[string]types.AttributeValue{":o": &types.AttributeValueMemberS{Value: organizationID}},
			ScanIndexForward:          aws.Bool(true),
			ExclusiveStartKey:         start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range res.Items {
			var ref CompanyRef
			if err := attributevalue.UnmarshalMap(item, &ref); err != nil {
				return nil, fmt.Errorf("decoding a company of %s: %w", organizationID, err)
			}
			// A legacy partition carrying the attribute is a rollback copy,
			// never a company to count or to bill.
			if IsCompanyKey(ref.PK) {
				out = append(out, ref)
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// CompanyIndexGap is a company-keyed record the organization index cannot see.
type CompanyIndexGap struct {
	PK                  string
	OwnerUserID         string
	CreatedAt           string
	MissingOrganization bool
	MissingCreatedAt    bool
}

// ListCompanyIndexGaps scans for company records without organization_id or
// created_at. Used once, by cmd/migrate-billing-org, before the migration:
// a company the index misses is a company the quota does not count.
func (r *OrganizationRepository) ListCompanyIndexGaps(ctx context.Context) ([]CompanyIndexGap, error) {
	var out []CompanyIndexGap
	var start map[string]types.AttributeValue
	for {
		res, err := r.ScanRaw(ctx, &dynamodb.ScanInput{
			TableName:                aws.String(r.TableName),
			FilterExpression:         aws.String("attribute_not_exists(#o) OR attribute_not_exists(#c)"),
			ExpressionAttributeNames: map[string]string{"#o": AttrOrganizationID, "#c": attrCreatedAt},
			ExclusiveStartKey:        start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range res.Items {
			pk := itemString(item, "pk")
			if !IsCompanyKey(pk) {
				continue
			}
			out = append(out, CompanyIndexGap{
				PK:                  pk,
				OwnerUserID:         itemString(item, AttrOwnerUserID),
				CreatedAt:           itemString(item, attrCreatedAt),
				MissingOrganization: itemString(item, AttrOrganizationID) == "",
				MissingCreatedAt:    itemString(item, attrCreatedAt) == "",
			})
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}

// BackfillIndexKeys writes organization_id and created_at only where absent, so
// a value already on the record always wins over the backfill's guess.
func (r *OrganizationRepository) BackfillIndexKeys(ctx context.Context, companyPK, organizationID, createdAt string) error {
	if !IsCompanyKey(companyPK) {
		return fmt.Errorf("not a company key: %s", companyPK)
	}
	sets := []string{}
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	if organizationID != "" {
		sets = append(sets, "#o = if_not_exists(#o, :o)")
		names["#o"] = AttrOrganizationID
		values[":o"] = &types.AttributeValueMemberS{Value: organizationID}
	}
	if createdAt != "" {
		sets = append(sets, "#c = if_not_exists(#c, :c)")
		names["#c"] = attrCreatedAt
		values[":c"] = &types.AttributeValueMemberS{Value: createdAt}
	}
	if len(sets) == 0 {
		return nil
	}
	_, err := r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.TableName),
		Key:                       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: companyPK}},
		UpdateExpression:          aws.String("SET " + strings.Join(sets, ", ")),
		ConditionExpression:       aws.String("attribute_exists(pk)"),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}
```

(`itemString` already exists in `company.go`; `strings` is already imported in `organizations.go`.)

- [ ] **Step 9: Run the integration tests to verify they pass**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestListCompaniesOfOrganization|TestIndexGaps' -v`
Expected: PASS. Then `go vet ./... && go build ./...` clean.

- [ ] **Step 10: Document** — in `DynamoDB-Tables.md`, under `{env}_organizations`, add the GSI row: `organization-index` — PK `organization_id`, SK `created_at`, projection KEYS_ONLY, sparse (company-keyed rows only); readers: `BillingService.companiesUsed`, the billing-company lookup, `cmd/migrate-billing-org`.

- [ ] **Step 11: Commit**

```bash
git add cdk/lib/dynamodb-stack.ts cdk/test/dynamodb-stack.test.ts api/internal/repositories/organizations.go api/tests/integration/setup_test.go api/tests/integration/organization_index_test.go DynamoDB-Tables.md
git commit -m "feat(dfe): organization-index GSI listing the companies of a ctech-account organization"
```

---

### Task 2: `account_billing` keyed by organization

**Files:**
- Modify: `api/internal/repositories/account_billing.go`
- Modify: `api/tests/integration/account_billing_test.go`
- Modify: `DynamoDB-Tables.md` (`account_billing` row shapes)

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `const repositories.OrgBillingPrefix = "ORG_"`; `func repositories.OrgBillingPK(organizationID string) string`
  - `AccountSnapshot.OrganizationID string` (`dynamodbav:"organization_id,omitempty" json:"organization_id,omitempty"`)
  - `AccountSnapshot.InheritedFromUser bool` (`dynamodbav:"-" json:"-"`) — set only by the dual read (Task 6), never stored.
  - `func (r *AccountBillingRepository) GetOrg(ctx context.Context, organizationID string) (*AccountSnapshot, error)`
  - `Put` writes `pk = ORG_{OrganizationID}` when `OrganizationID != ""`, else `USER_{UserID}` (legacy rows, tests).
  - Parameter `userID` renamed `accountID` (same type, same key shape `USAGE_{id}#…`, `QUOTA_GUARD_{id}#…`) in `UsageCounterPK`, `quotaGuardPK`, `BuildQuotaGuardTx`, `ReserveUsage`, `BuildReserveUsageTx`, `RefundUsage`, `RefundUsageOnce`, `GetUsage`.
  - `type repositories.UsageSource struct { AccountID, Period string }`
  - `func (r *AccountBillingRepository) CopyUsageOnce(ctx context.Context, sources []UsageSource, toAccount, toPeriod, marker string) (bool, error)` — adds the sum of the sources' meters to the target in one transaction with a once-only marker; false when the marker already existed.
  - `func (r *AccountBillingRepository) ListUserSnapshots(ctx context.Context) ([]AccountSnapshot, error)` — every `USER_` snapshot row.

- [ ] **Step 1: Write the failing integration tests** — append to `api/tests/integration/account_billing_test.go`

```go
// TestOrgSnapshotLivesUnderTheOrganization: the ORG_ row and the legacy USER_
// row are two partitions; reading one never returns the other.
func TestOrgSnapshotLivesUnderTheOrganization(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)

	if err := repo.Put(ctx, &repositories.AccountSnapshot{
		OrganizationID: "org-snap-1", UserID: "", SubscriptionID: "sub_org", Status: "ACTIVE",
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Put(ctx, &repositories.AccountSnapshot{UserID: "org-snap-1", SubscriptionID: "sub_user"}); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetOrg(ctx, "org-snap-1")
	if err != nil || got == nil || got.SubscriptionID != "sub_org" || got.OrganizationID != "org-snap-1" {
		t.Fatalf("GetOrg = %+v (%v)", got, err)
	}
	user, err := repo.Get(ctx, "org-snap-1")
	if err != nil || user == nil || user.SubscriptionID != "sub_user" {
		t.Fatalf("Get(USER_) = %+v (%v)", user, err)
	}
	if none, err := repo.GetOrg(ctx, "org-snap-absent"); err != nil || none != nil {
		t.Fatalf("an organization with no row reads as absent, got %+v (%v)", none, err)
	}
}

// TestCopyUsageOnceAddsTheSourcesExactlyOnce: the migration's counter copy.
// Twice is the same as once, and the target keeps what it already had.
func TestCopyUsageOnceAddsTheSourcesExactlyOnce(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)

	for range 3 {
		if _, err := repo.ReserveUsage(ctx, "copy-user", "2026-10-01", "nfe", -1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.ReserveUsage(ctx, "copy-org", "2026-10-01", "nfe", -1); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReserveUsage(ctx, "copy-org", "2026-10-10", "mdfe", -1); err != nil {
		t.Fatal(err)
	}
	sources := []repositories.UsageSource{
		{AccountID: "copy-user", Period: "2026-10-01"},
		{AccountID: "copy-org", Period: "2026-10-01"},
		// The target itself is ignored as a source.
		{AccountID: "copy-org", Period: "2026-10-10"},
	}

	for i := range 2 {
		fresh, err := repo.CopyUsageOnce(ctx, sources, "copy-org", "2026-10-10", "migrate-usage:copy-user:copy-org")
		if err != nil {
			t.Fatal(err)
		}
		if fresh != (i == 0) {
			t.Fatalf("run %d: fresh = %v", i, fresh)
		}
	}
	got, err := repo.GetUsage(ctx, "copy-org", "2026-10-10")
	if err != nil {
		t.Fatal(err)
	}
	if got["nfe"] != 4 || got["mdfe"] != 1 {
		t.Fatalf("usage = %+v, want nfe 4 (3 + 1) and mdfe 1 (kept)", got)
	}
}

func TestListUserSnapshotsReturnsOnlyUserRows(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	if err := repo.Put(ctx, &repositories.AccountSnapshot{UserID: "list-user-1", SubscriptionID: "sub_l1"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Put(ctx, &repositories.AccountSnapshot{OrganizationID: "list-org-1", SubscriptionID: "sub_o1"}); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListUserSnapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sawUser bool
	for _, s := range all {
		if s.OrganizationID != "" {
			t.Fatalf("an ORG_ row was listed: %+v", s)
		}
		if s.UserID == "list-user-1" {
			sawUser = true
		}
	}
	if !sawUser {
		t.Fatal("the USER_ row was not listed")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestOrgSnapshot|TestCopyUsageOnce|TestListUserSnapshots' -v`
Expected: FAIL — build errors (`OrganizationID` unknown field, `repo.GetOrg undefined`, …).

- [ ] **Step 3: Implement** — in `api/internal/repositories/account_billing.go`:

Update the type comment's row list to:

```go
//	pk = ORG_{organization_id}                the subscription snapshot (docs/specs/2026-10-10-organization-subscription.md)
//	pk = USER_{sub}                           the pre-migration snapshot, read only by the dual read
//	pk = EVENT_{event_id}                     a processed webhook (or a once-only marker), with a TTL
//	pk = USAGE_{organization_id}#{period}     this period's meters
//	pk = QUOTA_GUARD_{organization_id}#{meter} concurrency guard for live resource quotas
```

Add after `AccountBillingPK`:

```go
// OrgBillingPrefix marks an organization's snapshot, and is the same string the
// DF-e sends billing as the customer's external_ref.
const OrgBillingPrefix = "ORG_"

// OrgBillingPK is the snapshot key of a ctech-account organization.
func OrgBillingPK(organizationID string) string { return OrgBillingPrefix + organizationID }

// snapshotPK is where a snapshot is filed: by organization, or by user for a
// legacy row.
func snapshotPK(s *AccountSnapshot) string {
	if s.OrganizationID != "" {
		return OrgBillingPK(s.OrganizationID)
	}
	return AccountBillingPK(s.UserID)
}
```

In `AccountSnapshot`, add above `UserID`:

```go
	// OrganizationID is the ctech-account organization this snapshot governs.
	// Empty only on a legacy USER_ row.
	OrganizationID string `dynamodbav:"organization_id,omitempty" json:"organization_id,omitempty"`
```

and above `SyncedAt`:

```go
	// InheritedFromUser marks a snapshot served by the dual read: the
	// organization has no subscription of its own yet and is running on its
	// owner's pre-migration one. Never stored; mutations refuse it.
	InheritedFromUser bool `dynamodbav:"-" json:"-"`
```

In `Put`, replace the pk line with `item["pk"] = &types.AttributeValueMemberS{Value: snapshotPK(s)}`.

Add `GetOrg`:

```go
// GetOrg reads an organization's snapshot. Nil with a nil error: never synced.
func (r *AccountBillingRepository) GetOrg(ctx context.Context, organizationID string) (*AccountSnapshot, error) {
	item, err := r.GetItem(ctx, OrgBillingPK(organizationID))
	if err != nil || item == nil {
		return nil, err
	}
	var out AccountSnapshot
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		return nil, fmt.Errorf("decoding organization billing snapshot: %w", err)
	}
	return &out, nil
}
```

Rename `userID` to `accountID` in the signatures and bodies of `UsageCounterPK`, `quotaGuardPK`, `BuildQuotaGuardTx`, `ReserveUsage`, `BuildReserveUsageTx`, `RefundUsage`, `RefundUsageOnce`, `GetUsage`, and change `UsageCounterPK`'s comment first line to "keys one organization's counters for one billing period". Bodies are otherwise unchanged (`RawUserID` on an organization id is a no-op).

Add the copy and the listing:

```go
// UsageSource names one counter row to copy from.
type UsageSource struct {
	AccountID string
	Period    string
}

// CopyUsageOnce adds the sources' counters to toAccount's row for toPeriod,
// once. The marker (an EVENT_ row with the webhook TTL) and every ADD are one
// transaction, so a re-run is a no-op and a failure leaves nothing half done.
// A source equal to the target is skipped: the target keeps what it has.
func (r *AccountBillingRepository) CopyUsageOnce(ctx context.Context, sources []UsageSource, toAccount, toPeriod, marker string) (bool, error) {
	target := UsageCounterPK(toAccount, toPeriod)
	sum := map[string]int64{}
	for _, src := range sources {
		if UsageCounterPK(src.AccountID, src.Period) == target {
			continue
		}
		got, err := r.GetUsage(ctx, src.AccountID, src.Period)
		if err != nil {
			return false, err
		}
		for meter, n := range got {
			sum[meter] += n
		}
	}

	markerItem := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: BillingEventPK(marker)},
		"event_id":   &types.AttributeValueMemberS{Value: marker},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
		"ttl": &types.AttributeValueMemberN{
			Value: strconv.FormatInt(time.Now().Add(billingEventTTL).Unix(), 10),
		},
	}
	items := []types.TransactWriteItem{r.BuildPutTxItemIfAbsent(markerItem)}

	if len(sum) > 0 {
		meters := make([]string, 0, len(sum))
		for m := range sum {
			meters = append(meters, m)
		}
		sort.Strings(meters)
		names := map[string]string{"#ttl": "ttl"}
		values := map[string]types.AttributeValue{
			":ttl": &types.AttributeValueMemberN{Value: strconv.FormatInt(time.Now().Add(usageCounterTTL).Unix(), 10)},
		}
		adds := make([]string, 0, len(meters))
		for i, m := range meters {
			n, v := fmt.Sprintf("#m%d", i), fmt.Sprintf(":v%d", i)
			names[n] = m
			values[v] = &types.AttributeValueMemberN{Value: strconv.FormatInt(sum[m], 10)}
			adds = append(adds, n+" "+v)
		}
		items = append(items, types.TransactWriteItem{Update: &types.Update{
			TableName:                 aws.String(r.TableName),
			Key:                       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: target}},
			UpdateExpression:          aws.String("ADD " + strings.Join(adds, ", ") + " SET #ttl = if_not_exists(#ttl, :ttl)"),
			ExpressionAttributeNames:  names,
			ExpressionAttributeValues: values,
		}})
	}

	err := r.TransactWrite(ctx, items)
	if IsConditionFailed(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListUserSnapshots scans every pre-migration USER_ snapshot. Used once, by
// cmd/migrate-billing-org; the table is small and this is never on a request path.
func (r *AccountBillingRepository) ListUserSnapshots(ctx context.Context) ([]AccountSnapshot, error) {
	var out []AccountSnapshot
	var start map[string]types.AttributeValue
	for {
		res, err := r.ScanRaw(ctx, &dynamodb.ScanInput{
			TableName:                 aws.String(r.TableName),
			FilterExpression:          aws.String("begins_with(pk, :p)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{":p": &types.AttributeValueMemberS{Value: AccountBillingPK("")}},
			ExclusiveStartKey:         start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range res.Items {
			var s AccountSnapshot
			if err := attributevalue.UnmarshalMap(item, &s); err != nil {
				return nil, fmt.Errorf("decoding a user snapshot: %w", err)
			}
			if s.OrganizationID == "" {
				out = append(out, s)
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}
```

Add imports `sort`, `strings`. (`AccountBillingPK("")` is `"USER_"` via `BuildMemberSK`; check that `BuildMemberSK("")` returns `"USER_"`, it does: `"USER_" + ""`.)

- [ ] **Step 4: Fix the callers of the renamed parameters** — `go build ./...`. The only callers are in `internal/services/billing.go` and pass `snap.UserID`; leave them compiling as is (Task 6 changes what they pass).

- [ ] **Step 5: Run the integration tests to verify they pass**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestOrgSnapshot|TestCopyUsageOnce|TestListUserSnapshots|TestAccountSnapshotRoundTrip' -v`
Expected: PASS.

- [ ] **Step 6: Document** — `DynamoDB-Tables.md`, `account_billing`: replace the row list with the five shapes from Step 3 and note "`USER_` rows are read only by the dual read until Phase 2".

- [ ] **Step 7: Commit**

```bash
git add api/internal/repositories/account_billing.go api/tests/integration/account_billing_test.go DynamoDB-Tables.md
git commit -m "feat(dfe): account_billing rows keyed by ctech-account organization"
```

---

### Task 3: ctech-account workspace client

**Files:**
- Create: `api/internal/accountclient/workspace.go`
- Create: `api/internal/accountclient/workspace_test.go`

**Interfaces:**
- Consumes: `accountclient.Config`, `requestTimeout`, `maxBody`, `isOrganization` (existing in `reach.go`).
- Produces:
  - `const accountclient.MemberScope = "internal:account:org-member"`, `const accountclient.ListScope = "internal:account:user-organizations"`
  - `type accountclient.WorkspaceClient struct` (unexported fields `http`, `memberTokens`, `listTokens`, `baseURL`)
  - `func accountclient.NewWorkspace(cfg Config) *WorkspaceClient` — nil when any field is empty.
  - `func (c *WorkspaceClient) Membership(ctx context.Context, organizationID, userID string) (role, kind string, member bool, err error)`
  - `type accountclient.Workspace struct { ID, DisplayName, Role, Kind string }` (json `id`, `display_name`, `role`, `kind`)
  - `func (c *WorkspaceClient) Organizations(ctx context.Context, userID string) ([]Workspace, error)`
  - unexported `membershipWithToken`, `organizationsWithToken` for tests.

- [ ] **Step 1: Write the failing test** — `api/internal/accountclient/workspace_test.go`

```go
package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func workspaceServing(t *testing.T, status int, body string, gotPath *string) *WorkspaceClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &WorkspaceClient{http: srv.Client(), baseURL: srv.URL}
}

func TestMembershipReadsTheRole(t *testing.T) {
	var path string
	c := workspaceServing(t, http.StatusOK, `{"member":true,"role":"admin","kind":"organization"}`, &path)
	role, kind, member, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1")
	if err != nil || !member || role != "admin" || kind != "organization" {
		t.Fatalf("got %q %q %v %v", role, kind, member, err)
	}
	if path != "/v1.0/internal/organizations/org_1/members/usr_1" {
		t.Fatalf("path = %q", path)
	}
}

// "Not a member" is an answer, not an error.
func TestMembershipNonMemberIsARefusal(t *testing.T) {
	c := workspaceServing(t, http.StatusOK, `{"member":false}`, nil)
	role, _, member, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1")
	if err != nil || member || role != "" {
		t.Fatalf("got %q %v %v", role, member, err)
	}
}

// Any non-200 (a 403 means our own credential is wrong) is an outage.
func TestMembershipNon200IsAnError(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		c := workspaceServing(t, status, `{}`, nil)
		if _, _, _, err := c.membershipWithToken(context.Background(), "tok", "org_1", "usr_1"); err == nil {
			t.Fatalf("status %d: want an error", status)
		}
	}
}

func TestANilWorkspaceClientRefuses(t *testing.T) {
	var c *WorkspaceClient
	if _, _, member, err := c.Membership(context.Background(), "org_1", "usr_1"); err == nil || member {
		t.Fatalf("member=%v err=%v", member, err)
	}
	if _, err := c.Organizations(context.Background(), "usr_1"); err == nil {
		t.Fatal("want an error")
	}
	if NewWorkspace(Config{}) != nil {
		t.Fatal("an incomplete config must build no client")
	}
}

func TestOrganizationsListsWorkspaces(t *testing.T) {
	var path string
	c := workspaceServing(t, http.StatusOK,
		`{"organizations":[{"id":"org_1","display_name":"Escritório","role":"owner","kind":"organization"},{"id":"sp_1","display_name":"Pessoal","role":"owner","kind":"personal"}]}`, &path)
	got, err := c.organizationsWithToken(context.Background(), "tok", "usr_1")
	if err != nil || len(got) != 2 || got[0].ID != "org_1" || got[0].DisplayName != "Escritório" || got[1].Kind != "personal" {
		t.Fatalf("got %+v %v", got, err)
	}
	if path != "/v1.0/internal/users/usr_1/organizations" {
		t.Fatalf("path = %q", path)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/accountclient/ -run 'Membership|Workspace|Organizations'`
Expected: FAIL — `undefined: WorkspaceClient`.

- [ ] **Step 3: Implement** — `api/internal/accountclient/workspace.go`

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

// MemberScope lets this client ask whether a person belongs to an organization
// and with which ladder role. The DF-e uses the answer to decide who may manage
// the organization's subscription (docs/specs/2026-10-10-organization-subscription.md § 2).
const MemberScope = "internal:account:org-member"

// ListScope lets this client list a person's organizations. Used for the
// organization's display name on the plan screen and by cmd/migrate-billing-org;
// never for an authorization decision.
const ListScope = "internal:account:user-organizations"

// WorkspaceClient asks ctech-account about organization membership.
//
// A separate type and a separate credential from Client (reach): the two hold
// different scopes, and one being wrong must not disable the other. One token
// manager per scope so a grant missing for the list never breaks the role check.
type WorkspaceClient struct {
	http         *http.Client
	memberTokens *oauth2client.TokenManager
	listTokens   *oauth2client.TokenManager
	baseURL      string
}

// NewWorkspace builds the client, or nil when the credential is not configured.
// Callers treat nil as a refusal, like Client.
func NewWorkspace(cfg Config) *WorkspaceClient {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	hc := &http.Client{Timeout: requestTimeout}
	return &WorkspaceClient{
		http:         hc,
		memberTokens: oauth2client.New(hc, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, MemberScope),
		listTokens:   oauth2client.New(hc, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, ListScope),
		baseURL:      strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

type membershipResponse struct {
	Member bool   `json:"member"`
	Role   string `json:"role"`
	Kind   string `json:"kind"`
}

// Membership answers whether userID belongs to organizationID and with which
// role. ("", "", false, nil) is a refusal; any error is an outage the caller
// must refuse on.
func (c *WorkspaceClient) Membership(ctx context.Context, organizationID, userID string) (string, string, bool, error) {
	if c == nil {
		return "", "", false, fmt.Errorf("ctech-account workspace client is not configured")
	}
	token, err := c.memberTokens.Get(ctx)
	if err != nil {
		return "", "", false, fmt.Errorf("minting a service token: %w", err)
	}
	return c.membershipWithToken(ctx, token, organizationID, userID)
}

func (c *WorkspaceClient) membershipWithToken(ctx context.Context, token, organizationID, userID string) (string, string, bool, error) {
	path := fmt.Sprintf("%s/v1.0/internal/organizations/%s/members/%s",
		c.baseURL, url.PathEscape(organizationID), url.PathEscape(userID))
	var out membershipResponse
	if err := c.getJSON(ctx, token, path, maxBody, &out); err != nil {
		return "", "", false, err
	}
	if !out.Member {
		return "", "", false, nil
	}
	return out.Role, out.Kind, true, nil
}

// Workspace is one organization a person belongs to.
type Workspace struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Kind        string `json:"kind"`
}

// IsOrganization reports a workspace this product bills (absent kind included).
func (w Workspace) IsOrganization() bool { return isOrganization(w.Kind) }

// Organizations lists the person's workspaces. Information only.
func (c *WorkspaceClient) Organizations(ctx context.Context, userID string) ([]Workspace, error) {
	if c == nil {
		return nil, fmt.Errorf("ctech-account workspace client is not configured")
	}
	token, err := c.listTokens.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("minting a service token: %w", err)
	}
	return c.organizationsWithToken(ctx, token, userID)
}

func (c *WorkspaceClient) organizationsWithToken(ctx context.Context, token, userID string) ([]Workspace, error) {
	path := fmt.Sprintf("%s/v1.0/internal/users/%s/organizations", c.baseURL, url.PathEscape(userID))
	var out struct {
		Organizations []Workspace `json:"organizations"`
	}
	if err := c.getJSON(ctx, token, path, maxListBody, &out); err != nil {
		return nil, err
	}
	return out.Organizations, nil
}

// maxListBody caps the organization list: a person in many organizations
// outgrows the 8 KiB that suits a single membership answer.
const maxListBody = 256 << 10

// getJSON is the request both calls make: bearer token, 200 or error, capped body.
func (c *WorkspaceClient) getJSON(ctx context.Context, token, path string, limit int64, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ctech-account answered %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, limit)).Decode(out); err != nil {
		return fmt.Errorf("decoding the answer: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/accountclient/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/accountclient/workspace.go api/internal/accountclient/workspace_test.go
git commit -m "feat(dfe): ctech-account workspace client for organization membership"
```

---

### Task 4: `WorkspaceRoleService`, its credential and its wiring

**Files:**
- Create: `api/internal/services/workspace_roles.go`
- Create: `api/internal/services/workspace_roles_test.go`
- Modify: `api/internal/config/config.go:60-70` (two fields)
- Modify: `api/internal/app/app.go` (provider `newWorkspaceRoleService`, registered next to `newReachService`)
- Modify: `cdk/lib/api-stack.ts:99-107,197-201` (SSM params + env args)
- Test: `cdk/test/api-stack.test.ts`

**Interfaces:**
- Consumes: `accountclient.WorkspaceClient`, `accountclient.Workspace`, `CacheGet`/`CacheSet`/`cacheDelete` (services/cache.go).
- Produces:
  - `const services.AccountRoleOwner = "owner"`, `const services.AccountRoleAdmin = "admin"`
  - `func services.MayManageBilling(role string) bool`
  - `type services.WorkspaceRoleService struct`; `func services.NewWorkspaceRoleService(src workspaceSource, c cache.Backend) *WorkspaceRoleService`
  - `func (s *WorkspaceRoleService) Role(ctx context.Context, organizationID, userID string) (string, error)` — `""` for a non-member; error on outage or nil service.
  - `func (s *WorkspaceRoleService) OrganizationName(ctx context.Context, organizationID, userID string) string` — best effort, `""` on any failure.
  - Config: `AccountWorkspaceClientID` (`ACCOUNT_WORKSPACE_CLIENT_ID`), `AccountWorkspaceClientSecret` (`ACCOUNT_WORKSPACE_CLIENT_SECRET`).

- [ ] **Step 1: Write the failing test** — `api/internal/services/workspace_roles_test.go`

```go
package services

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
)

type fakeWorkspaceSource struct {
	role, kind string
	member     bool
	err        error
	orgs       []accountclient.Workspace
	listErr    error
	calls      int
}

func (f *fakeWorkspaceSource) Membership(_ context.Context, _, _ string) (string, string, bool, error) {
	f.calls++
	return f.role, f.kind, f.member, f.err
}

func (f *fakeWorkspaceSource) Organizations(_ context.Context, _ string) ([]accountclient.Workspace, error) {
	return f.orgs, f.listErr
}

func TestRoleIsReadAndCached(t *testing.T) {
	src := &fakeWorkspaceSource{role: AccountRoleAdmin, kind: "organization", member: true}
	s := NewWorkspaceRoleService(src, cache.NewMemoryBackend(16))
	for range 2 {
		role, err := s.Role(context.Background(), "org_1", "usr_1")
		if err != nil || role != AccountRoleAdmin {
			t.Fatalf("role=%q err=%v", role, err)
		}
	}
	if src.calls != 1 {
		t.Fatalf("calls = %d, want 1 (cached)", src.calls)
	}
}

func TestANonMemberHasNoRole(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{}, cache.NewMemoryBackend(16))
	if role, err := s.Role(context.Background(), "org_1", "usr_1"); err != nil || role != "" {
		t.Fatalf("role=%q err=%v", role, err)
	}
}

// A personal space is never an organization whose plan the DF-e manages.
func TestARoleInASpaceIsNoRole(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{role: AccountRoleOwner, kind: "personal", member: true}, cache.NewMemoryBackend(16))
	if role, err := s.Role(context.Background(), "sp_1", "usr_1"); err != nil || role != "" {
		t.Fatalf("role=%q err=%v", role, err)
	}
}

func TestRoleOutageIsAnErrorAndNotCached(t *testing.T) {
	src := &fakeWorkspaceSource{err: errors.New("down")}
	s := NewWorkspaceRoleService(src, cache.NewMemoryBackend(16))
	if _, err := s.Role(context.Background(), "org_1", "usr_1"); err == nil {
		t.Fatal("an outage must be an error")
	}
	src.err, src.role, src.kind, src.member = nil, AccountRoleOwner, "", true
	if role, err := s.Role(context.Background(), "org_1", "usr_1"); err != nil || role != AccountRoleOwner {
		t.Fatalf("after recovery role=%q err=%v (an outage must not be cached)", role, err)
	}
}

func TestANilRoleServiceFailsClosed(t *testing.T) {
	var s *WorkspaceRoleService
	if _, err := s.Role(context.Background(), "org_1", "usr_1"); err == nil {
		t.Fatal("a nil service must refuse")
	}
	if s.OrganizationName(context.Background(), "org_1", "usr_1") != "" {
		t.Fatal("a nil service names nothing")
	}
}

func TestMayManageBilling(t *testing.T) {
	for role, want := range map[string]bool{AccountRoleOwner: true, AccountRoleAdmin: true, "member": false, "viewer": false, "": false} {
		if MayManageBilling(role) != want {
			t.Errorf("MayManageBilling(%q) = %v", role, !want)
		}
	}
}

func TestOrganizationNameIsBestEffort(t *testing.T) {
	s := NewWorkspaceRoleService(&fakeWorkspaceSource{orgs: []accountclient.Workspace{{ID: "org_1", DisplayName: "Escritório Silva"}}}, cache.NewMemoryBackend(16))
	if got := s.OrganizationName(context.Background(), "org_1", "usr_1"); got != "Escritório Silva" {
		t.Fatalf("name = %q", got)
	}
	failing := NewWorkspaceRoleService(&fakeWorkspaceSource{listErr: errors.New("down")}, cache.NewMemoryBackend(16))
	if got := failing.OrganizationName(context.Background(), "org_1", "usr_1"); got != "" {
		t.Fatalf("name on failure = %q", got)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/services/ -run 'Role|MayManageBilling|OrganizationName'`
Expected: FAIL — `undefined: NewWorkspaceRoleService`.

- [ ] **Step 3: Implement** — `api/internal/services/workspace_roles.go`

```go
package services

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
)

// ctech-account's ladder roles that manage an organization's DF-e subscription
// (ctech-billing ADR 0025 amendment of 2026-10-07).
const (
	AccountRoleOwner = "owner"
	AccountRoleAdmin = "admin"
)

// workspaceRoleCacheTTL matches reachCacheTTL: a demotion lands within a minute.
const workspaceRoleCacheTTL = 60

// workspaceNameCacheTTL is longer: the name is display-only.
const workspaceNameCacheTTL = 300

// MayManageBilling reports a role that may choose, change, cancel or pay.
func MayManageBilling(role string) bool {
	return role == AccountRoleOwner || role == AccountRoleAdmin
}

type workspaceSource interface {
	Membership(ctx context.Context, organizationID, userID string) (role, kind string, member bool, err error)
	Organizations(ctx context.Context, userID string) ([]accountclient.Workspace, error)
}

// WorkspaceRoleService answers "which role does this person hold in this
// ctech-account organization", cached, failing closed like ReachService: an
// outage is an error and is never cached, and nil is a refusal.
type WorkspaceRoleService struct {
	src   workspaceSource
	cache cache.Backend
}

func NewWorkspaceRoleService(src workspaceSource, c cache.Backend) *WorkspaceRoleService {
	return &WorkspaceRoleService{src: src, cache: c}
}

func workspaceRoleCacheKey(organizationID, userID string) string {
	return fmt.Sprintf("dfe:workspace-role:%s:%s", organizationID, userID)
}

func workspaceNameCacheKey(organizationID string) string {
	return "dfe:workspace-name:" + organizationID
}

type workspaceRoleAnswer struct {
	Role string `json:"role"`
}

// Role returns the person's role, "" for a non-member (or a member of a
// workspace that is not an organization).
func (s *WorkspaceRoleService) Role(ctx context.Context, organizationID, userID string) (string, error) {
	if s == nil || s.src == nil {
		return "", fmt.Errorf("the ctech-account workspace credential is not configured")
	}
	key := workspaceRoleCacheKey(organizationID, userID)
	if v, ok := CacheGet[workspaceRoleAnswer](ctx, s.cache, key); ok {
		return v.Role, nil
	}
	role, kind, member, err := s.src.Membership(ctx, organizationID, userID)
	if err != nil {
		return "", fmt.Errorf("reading the role in %s: %w", organizationID, err)
	}
	if !member || !(accountclient.Workspace{Kind: kind}).IsOrganization() {
		role = ""
	}
	CacheSet(ctx, s.cache, key, workspaceRoleAnswer{Role: role}, workspaceRoleCacheTTL)
	return role, nil
}

// OrganizationName is the organization's display name as the person sees it,
// or "" when it cannot be read. Never an authorization input.
func (s *WorkspaceRoleService) OrganizationName(ctx context.Context, organizationID, userID string) string {
	if s == nil || s.src == nil {
		return ""
	}
	key := workspaceNameCacheKey(organizationID)
	if v, ok := CacheGet[string](ctx, s.cache, key); ok {
		return *v
	}
	orgs, err := s.src.Organizations(ctx, userID)
	if err != nil {
		return ""
	}
	for _, w := range orgs {
		if w.ID == organizationID {
			CacheSet(ctx, s.cache, key, w.DisplayName, workspaceNameCacheTTL)
			return w.DisplayName
		}
	}
	return ""
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/services/ -run 'Role|MayManageBilling|OrganizationName'`
Expected: PASS.

- [ ] **Step 5: Configuration and wiring**

`api/internal/config/config.go`, after `AccountClientSecret`:

```go
	// AccountWorkspaceClientID/Secret are the client-credentials client holding
	// internal:account:org-member and internal:account:user-organizations
	// (docs/specs/2026-10-10-organization-subscription.md). Separate from the
	// reach client so one wrong grant cannot disable the other. Absent means
	// nobody can manage a DF-e subscription (fail closed).
	AccountWorkspaceClientID     string `env:"ACCOUNT_WORKSPACE_CLIENT_ID"`
	AccountWorkspaceClientSecret string `env:"ACCOUNT_WORKSPACE_CLIENT_SECRET"`
```

`api/internal/app/app.go`: add `newWorkspaceRoleService` to the provider list next to `newReachService`, and:

```go
// newWorkspaceRoleService builds the role check for managing an organization's
// subscription. Nil when ctech-account has not issued the credential: the
// service then answers every management attempt with 403, and reading the plan
// keeps working.
func newWorkspaceRoleService(cfg *config.Config, c cache.Backend) *services.WorkspaceRoleService {
	client := accountclient.NewWorkspace(accountclient.Config{
		BaseURL:      cfg.CtechURL,
		TokenURL:     billingclient.TokenURLFor(cfg.CtechURL),
		ClientID:     cfg.AccountWorkspaceClientID,
		ClientSecret: cfg.AccountWorkspaceClientSecret,
		Cache:        c,
	})
	if client == nil {
		slog.Warn("the ctech-account workspace credential is not configured — nobody can manage a DF-e subscription")
		return nil
	}
	return services.NewWorkspaceRoleService(client, c)
}
```

(Task 6 consumes it in `newBillingService`.)

`cdk/lib/api-stack.ts`: after `accountClientSecretParameter`:

```ts
    // The organization-membership credential (internal:account:org-member and
    // internal:account:user-organizations): who may manage an organization's
    // DF-e subscription. Created by ctech-account's cmd/createclient with these
    // SSM paths. Absent means nobody can manage a plan; reading still works.
    const accountWorkspaceClientIdParameter = `/ctech-dfe/${environment}/account-workspace-client-id`;
    const accountWorkspaceClientSecretParameter = `/ctech-dfe/${environment}/account-workspace-client-secret`;
```

and in `ssmEnvArgs`, after the `ACCOUNT_CLIENT_SECRET` line:

```ts
      `ACCOUNT_WORKSPACE_CLIENT_ID=${accountWorkspaceClientIdParameter}`,
      `ACCOUNT_WORKSPACE_CLIENT_SECRET=${accountWorkspaceClientSecretParameter}`,
```

Append to `cdk/test/api-stack.test.ts` (reuse the file's existing synth helper; if it has none, synth like its first test does) a test that the rendered user data contains both `ACCOUNT_WORKSPACE_CLIENT_ID=/ctech-dfe/` and `ACCOUNT_WORKSPACE_CLIENT_SECRET=/ctech-dfe/`, mirroring the assertion the file already makes for `ACCOUNT_CLIENT_ID` (`grep -n "ACCOUNT_CLIENT_ID" cdk/test/api-stack.test.ts`; if no such assertion exists, assert with `JSON.stringify(template.toJSON())` containing both strings).

- [ ] **Step 6: Verify**

Run: `go build ./... && go vet ./...` and `cd cdk && npx jest test/api-stack.test.ts`
Expected: clean build; jest PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/workspace_roles.go api/internal/services/workspace_roles_test.go api/internal/config/config.go api/internal/app/app.go cdk/lib/api-stack.ts cdk/test/api-stack.test.ts
git commit -m "feat(dfe): ctech-account role check for managing an organization subscription"
```

---
### Task 5: The billing read side by organization, with the dual read

**Files:**
- Modify: `api/internal/services/billing.go` (snapshot, sync, `SnapshotFrom`, `SnapshotForOrg`, reservations, refunds, `Usage`, `companiesUsed`; `ownedOrganizations` deleted)
- Modify: `api/internal/services/organizations.go` (`CompaniesOf`)
- Modify: `api/internal/services/billing_test.go` (the `SnapshotFrom` tests)
- Modify: `api/internal/services/nfes/emit.go:749`, `nfes/nfce_emit.go:288`, `mdfes/emit.go:577`, `nfses/emit.go:301` (reservation field rename only)
- Modify: `api/internal/app/app.go` (`newBillingService` turns the dual read on)
- Modify: `api/tests/integration/billing_usage_test.go` (`seedPayingOrg` seeds an organization)
- Create: `api/tests/integration/org_billing_test.go`

**Interfaces:**
- Consumes: Task 1 `ListCompaniesOfOrganization`, `CompanyRef`; Task 2 `GetOrg`, `OrgBillingPK`, `AccountSnapshot.OrganizationID`, `InheritedFromUser`.
- Produces:
  - `func (s *OrganizationService) CompaniesOf(ctx context.Context, organizationID string) ([]repositories.CompanyRef, error)`
  - `func (s *BillingService) WithUserFallback() *BillingService` (Phase 1 only; Task 17 deletes it)
  - `func (s *BillingService) Snapshot(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)` — the organization's own row, cache-first, never billing.
  - `func (s *BillingService) Sync(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)`
  - `func (s *BillingService) Invalidate(ctx context.Context, organizationID string)`
  - `func services.SnapshotFrom(organizationID string, ent *billingclient.Entitlements) *repositories.AccountSnapshot`
  - `func (s *BillingService) OrganizationOf(ctx context.Context, companyPK string) (string, error)` — `""` for a company with no `organization_id`; 404 problem for an unknown company.
  - `func (s *BillingService) SnapshotForOrg(ctx context.Context, companyPK string) (*repositories.AccountSnapshot, error)` — unchanged name and parameter (a company pk), now via the company's organization.
  - `func (s *BillingService) snapshotFor(ctx context.Context, organizationID, companyPK string) (*repositories.AccountSnapshot, error)` (unexported; dual read lives here)
  - `UsageReservation.OrganizationID string` replaces `UsageReservation.UserID`.
  - `func (s *BillingService) RefundReservedUsage(ctx context.Context, accountID, period, meter, docKey string) error` (parameter renamed)
  - `func (s *BillingService) Usage(ctx context.Context, organizationID, companyPK string) (map[string]UsageMeter, error)`
  - `func (s *BillingService) companiesUsed(ctx context.Context, organizationID string) (int64, error)`
  - `var services.ErrNoOrganization *problem.Problem` — 409 "esta empresa não está vinculada a uma organização da conta CTech".

- [ ] **Step 1: Write the failing integration tests** — `api/tests/integration/org_billing_test.go`

```go
//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// seedOrgSnapshot files an organization's own snapshot.
func seedOrgSnapshot(t *testing.T, organizationID string, snap *repositories.AccountSnapshot) {
	t.Helper()
	snap.OrganizationID = organizationID
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(context.Background(), snap); err != nil {
		t.Fatal(err)
	}
}

func proSnapshot(subID string) *repositories.AccountSnapshot {
	return &repositories.AccountSnapshot{
		SubscriptionID: subID, Status: services.StatusActive, Plan: "pro", Entitled: true,
		PeriodStart: "2026-10-01", PeriodEnd: "2026-11-01",
		Quotas: map[string]int64{services.MeterNFe: 3, services.MeterCompanies: 10},
	}
}

// Spec § 5 test 1: two companies of one organization share one snapshot and
// one counter.
func TestTwoCompaniesOfOneOrganizationShareSnapshotAndCounter(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	org := "org-share-" + newCompanyPK(t)
	a := seedCompany(t, org, "owner-share", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-share", "11222333000262", "B Ltda")
	seedOrgSnapshot(t, org, proSnapshot("sub_share"))

	snapA, err := svc.SnapshotForOrg(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	snapB, err := svc.SnapshotForOrg(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if snapA.SubscriptionID != "sub_share" || snapB.SubscriptionID != "sub_share" {
		t.Fatalf("snapshots = %+v / %+v", snapA, snapB)
	}

	for _, company := range []string{a, b, a} {
		if err := svc.Reserve(ctx, company, services.MeterNFe); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Reserve(ctx, b, services.MeterNFe); err == nil {
		t.Fatal("the fourth NF-e of the organization must be refused, whichever company asks")
	}
	usage, err := svc.Usage(ctx, org, a)
	if err != nil || usage[services.MeterNFe].Used != 3 {
		t.Fatalf("usage = %+v (%v)", usage, err)
	}
}

// Spec § 5 test 2: two organizations of the same owner have separate quotas and
// subscriptions.
func TestTwoOrganizationsOfOneOwnerAreSeparate(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	org1, org2 := "org-sep1-"+newCompanyPK(t), "org-sep2-"+newCompanyPK(t)
	c1 := seedCompany(t, org1, "owner-sep", "11222333000181", "Um Ltda")
	c2 := seedCompany(t, org2, "owner-sep", "44555666000105", "Dois Ltda")
	seedOrgSnapshot(t, org1, proSnapshot("sub_one"))
	seedOrgSnapshot(t, org2, proSnapshot("sub_two"))

	for range 3 {
		if err := svc.Reserve(ctx, c1, services.MeterNFe); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.Reserve(ctx, c2, services.MeterNFe); err != nil {
		t.Fatalf("the second organization has its own quota: %v", err)
	}
	s2, _ := svc.SnapshotForOrg(ctx, c2)
	if s2.SubscriptionID != "sub_two" {
		t.Fatalf("snapshot = %+v", s2)
	}
}

// Spec § 5 test 6: dual read. Only the owner's USER_ snapshot exists → it is
// served; once the organization has its own, the ORG_ one wins.
func TestDualReadFallsBackToTheOwnerUntilTheOrganizationHasItsOwn(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithUserFallback()
	org := "org-dual-" + newCompanyPK(t)
	company := seedCompany(t, org, "owner-dual", "11222333000181", "Dual Ltda")
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, &repositories.AccountSnapshot{
		UserID: "owner-dual", SubscriptionID: "sub_user", Status: services.StatusActive, Plan: "unlimited",
		Entitled: true, PeriodStart: "2026-10-01", Quotas: map[string]int64{services.MeterNFe: -1},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_user" || !got.InheritedFromUser || got.OrganizationID != org {
		t.Fatalf("fallback = %+v (%v)", got, err)
	}
	// Writes go to ORG_ only.
	if err := svc.Reserve(ctx, company, services.MeterNFe); err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewAccountBillingRepository(db, cfg)
	if u, _ := repo.GetUsage(ctx, org, "2026-10-01"); u[services.MeterNFe] != 1 {
		t.Fatalf("org counter = %+v", u)
	}
	if u, _ := repo.GetUsage(ctx, "owner-dual", "2026-10-01"); u[services.MeterNFe] != 0 {
		t.Fatalf("the USER_ counter moved: %+v", u)
	}

	seedOrgSnapshot(t, org, proSnapshot("sub_org"))
	svc.Invalidate(ctx, org)
	got, err = svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_org" || got.InheritedFromUser {
		t.Fatalf("ORG_ must win: %+v (%v)", got, err)
	}
}

// Review Focus 1: a company with no organization_id falls back to its owner
// for reading, and refuses every counter write.
func TestACompanyWithNoOrganizationFallsBackToItsOwner(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithUserFallback()
	company := newCompanyPK(t)
	if err := orgRepo.CreateOrganization(ctx, company, map[string]types.AttributeValue{
		repositories.AttrOwnerUserID: &types.AttributeValueMemberS{Value: "owner-noorg"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, &repositories.AccountSnapshot{
		UserID: "owner-noorg", SubscriptionID: "sub_noorg", Status: services.StatusActive, Entitled: true,
		Quotas: map[string]int64{services.MeterNFe: -1},
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "sub_noorg" || got.OrganizationID != "" {
		t.Fatalf("snapshot = %+v (%v)", got, err)
	}
	if _, err := svc.PrepareUsageReservation(ctx, company, services.MeterNFe, true); err == nil {
		t.Fatal("a company with no organization must not reserve anywhere")
	}
}

// companiesUsed counts enabled companies of the organization only.
func TestCompaniesUsedCountsEnabledCompaniesOfTheOrganization(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls)).WithEnablement(services.NewFiscalConfigEnablement(
		nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
	org := "org-count-" + newCompanyPK(t)
	enabled := seedCompany(t, org, "owner-count", "11222333000181", "Habilitada Ltda")
	_ = seedCompany(t, org, "owner-count", "11222333000262", "Vinculada Ltda") // linked, never configured
	_ = seedCompany(t, "org-other-"+newCompanyPK(t), "owner-count", "44555666000105", "Outra Ltda")
	seedNfeConfig(t, enabled)
	seedOrgSnapshot(t, org, proSnapshot("sub_count"))

	usage, err := svc.Usage(ctx, org, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if usage[services.MeterCompanies].Used != 1 {
		t.Fatalf("companies used = %d, want 1 (linked-only companies cost nothing)", usage[services.MeterCompanies].Used)
	}
}
```

Add the helper used above to the same file:

```go
// seedNfeConfig gives a company an NF-e configuration, which is what makes it
// enabled (services.FiscalConfigEnablement).
func seedNfeConfig(t *testing.T, companyPK string) {
	t.Helper()
	tx, _, err := nfeConfigRepo.BuildUpsertTxItem(companyPK, map[string]types.AttributeValue{
		"prod_current_serie": &types.AttributeValueMemberN{Value: "1"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := nfeConfigRepo.TransactWrite(context.Background(), []types.TransactWriteItem{tx}); err != nil {
		t.Fatal(err)
	}
}
```

In `api/tests/integration/billing_usage_test.go`, replace `seedPayingOrg` so the existing money-path tests run against an organization:

```go
// seedPayingOrg creates a company in a fresh organization and files that
// organization's billing snapshot, which is what the issuance path reads.
func seedPayingOrg(t *testing.T, ownerID string, snap *repositories.AccountSnapshot) string {
	t.Helper()
	org := "org-pay-" + newCompanyPK(t)
	company := seedCompany(t, org, ownerID, "11222333000181", "Pagante Ltda")
	seedOrgSnapshot(t, org, snap)
	return company
}
```

and in `TestARejectedDocumentGivesItsSlotBackExactlyOnce` replace `svc.Usage(ctx, "refund-owner")` with:

```go
	orgID, err := svc.OrganizationOf(ctx, orgPK)
	if err != nil {
		t.Fatal(err)
	}
	usage, err := svc.Usage(ctx, orgID, orgPK)
```

(Grep the file for any other `svc.Usage(` and apply the same change.)

- [ ] **Step 2: Run them to verify they fail**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestTwoCompanies|TestTwoOrganizations|TestDualRead|TestACompanyWithNoOrganization|TestCompaniesUsed|TestAuthorised|TestAFixedPlan|TestARejected' -v`
Expected: FAIL — build errors (`svc.WithUserFallback undefined`, `svc.OrganizationOf undefined`, `Usage` arity).

- [ ] **Step 3: Implement** — `api/internal/services/organizations.go`, after `Company`:

```go
// CompaniesOf lists the company records of one ctech-account organization,
// oldest first (repositories.OrganizationIndex).
func (s *OrganizationService) CompaniesOf(ctx context.Context, organizationID string) ([]repositories.CompanyRef, error) {
	return s.repo.ListCompaniesOfOrganization(ctx, organizationID)
}
```

`api/internal/services/billing.go`:

1. Replace `accountBillingCacheKey` with both keys:

```go
// orgBillingCacheKey caches an organization's snapshot. Keyed by organization,
// never by company: two companies of one organization must read one answer.
func orgBillingCacheKey(organizationID string) string {
	return "dfe:billing:org:" + organizationID
}

// userBillingCacheKey caches a pre-migration USER_ snapshot for the dual read.
// Deleted with it (Phase 2).
func userBillingCacheKey(userID string) string {
	return fmt.Sprintf("dfe:billing:%s", repositories.RawUserID(userID))
}
```

2. Add to `BillingService` the field `userFallback bool` and:

```go
// WithUserFallback opens the dual-read window
// (docs/specs/2026-10-10-organization-subscription.md § 4): an organization
// with no subscription of its own is served its company owner's pre-migration
// USER_ snapshot. Writes never follow it. Removed in Phase 2.
func (s *BillingService) WithUserFallback() *BillingService {
	s.userFallback = true
	return s
}

// ErrNoOrganization refuses a counter write for a company whose record names
// no ctech-account organization: there is no ORG_ row to write to, and
// writing to the owner's would split one organization's usage across keys.
var ErrNoOrganization = problem.Conflict(
	"esta empresa não está vinculada a uma organização da conta CTech; vincule-a pela conta CTech para continuar")
```

3. `noChargeSnapshot(organizationID string)` sets `OrganizationID: organizationID` instead of `UserID`.

4. Replace `Snapshot`, `Sync`, `Invalidate`:

```go
// Snapshot returns an organization's own billing standing, cache-first. It
// never calls billing: an organization with no row is "sem assinatura", which
// DynamoDB answers in a millisecond.
func (s *BillingService) Snapshot(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(organizationID), nil
	}
	if organizationID == "" {
		return &repositories.AccountSnapshot{}, nil
	}
	key := orgBillingCacheKey(organizationID)
	if v, ok := CacheGet[repositories.AccountSnapshot](ctx, s.cache, key); ok {
		return v, nil
	}
	snap, err := s.repo.GetOrg(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		snap = &repositories.AccountSnapshot{OrganizationID: organizationID}
	}
	CacheSet(ctx, s.cache, key, *snap, snapshotCacheTTL)
	return snap, nil
}

// Sync re-reads billing for ORG_{organizationID} and rewrites the snapshot.
// The only writer of ORG_ rows.
func (s *BillingService) Sync(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(organizationID), nil
	}
	ent, err := s.client.GetEntitlements(ctx, repositories.OrgBillingPK(organizationID))
	switch {
	case err == nil:
	case errors.Is(err, billingclient.ErrCustomerNotFound):
		ent = &billingclient.Entitlements{}
	default:
		return nil, err
	}
	snap := SnapshotFrom(organizationID, ent)
	if err := s.repo.Put(ctx, snap); err != nil {
		return nil, err
	}
	s.Invalidate(ctx, organizationID)
	return snap, nil
}

// Invalidate drops an organization's cached snapshot.
func (s *BillingService) Invalidate(ctx context.Context, organizationID string) {
	cacheDelete(ctx, s.cache, orgBillingCacheKey(organizationID))
}
```

5. `SnapshotFrom(organizationID string, ent …)`: first line becomes `snap := &repositories.AccountSnapshot{OrganizationID: organizationID}`; the rest is unchanged.

6. Replace `SnapshotForOrg` (keep `OwnerOf` as is; it is the dual read's owner lookup until Phase 2) and add:

```go
// OrganizationOf returns the ctech-account organization of a company, from the
// local record (written by the re-key and by LinkService.Link, the same fact
// reach answers). "" for a company whose record has none.
func (s *BillingService) OrganizationOf(ctx context.Context, companyPK string) (string, error) {
	company, err := s.orgs.Company(ctx, companyPK)
	if err != nil {
		return "", err
	}
	if company == nil {
		return "", problem.NotFound("organização não encontrada")
	}
	return company.OrganizationID, nil
}

// SnapshotForOrg returns the standing that governs a company: its
// organization's subscription (spec O1). The parameter is the company pk the
// request carries, as before.
func (s *BillingService) SnapshotForOrg(ctx context.Context, companyPK string) (*repositories.AccountSnapshot, error) {
	if !s.Enabled() {
		return noChargeSnapshot(""), nil
	}
	organizationID, err := s.OrganizationOf(ctx, companyPK)
	if err != nil {
		return nil, err
	}
	return s.snapshotFor(ctx, organizationID, companyPK)
}

// snapshotFor is Snapshot plus the dual read: while the window is open, an
// organization with no subscription of its own runs on its company owner's
// USER_ snapshot, marked InheritedFromUser and stamped with the organization
// so every write still lands on ORG_.
func (s *BillingService) snapshotFor(ctx context.Context, organizationID, companyPK string) (*repositories.AccountSnapshot, error) {
	snap, err := s.Snapshot(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if !s.userFallback || snap.NoCharge || snap.SubscriptionID != "" {
		return snap, nil
	}
	owner, err := s.OwnerOf(ctx, companyPK)
	if err != nil {
		return nil, err
	}
	if owner == "" {
		return snap, nil
	}
	user, err := s.userSnapshot(ctx, owner)
	if err != nil {
		return nil, err
	}
	if user == nil || user.SubscriptionID == "" {
		return snap, nil
	}
	inherited := *user
	inherited.OrganizationID = organizationID
	inherited.InheritedFromUser = true
	return &inherited, nil
}

// userSnapshot reads a pre-migration USER_ row, cache-first. Dual read only.
func (s *BillingService) userSnapshot(ctx context.Context, userID string) (*repositories.AccountSnapshot, error) {
	key := userBillingCacheKey(userID)
	if v, ok := CacheGet[repositories.AccountSnapshot](ctx, s.cache, key); ok {
		return v, nil
	}
	snap, err := s.repo.Get(ctx, userID)
	if err != nil || snap == nil {
		return snap, err
	}
	CacheSet(ctx, s.cache, key, *snap, snapshotCacheTTL)
	return snap, nil
}

// counterAccount is the account a snapshot's counters live under: always its
// organization.
func counterAccount(s *repositories.AccountSnapshot) (string, error) {
	if s.OrganizationID == "" {
		return "", ErrNoOrganization
	}
	return s.OrganizationID, nil
}
```

7. In `UsageReservation`, replace `UserID string` with `OrganizationID string`.

8. In `Reserve`, after the quota check: `account, err := counterAccount(snap); if err != nil { return err }` and pass `account` to `ReserveUsage`. In `PrepareUsageReservation`, the same before `GetUsage`, and pass `account` to `GetUsage`, `BuildReserveUsageTx`, and `OrganizationID: account` into the returned reservation. In `Refund` and `RefundOnce`, replace `snap.UserID == ""` with `snap.OrganizationID == ""` and pass `snap.OrganizationID`. `RefundReservedUsage`'s parameter becomes `accountID` (body unchanged).

9. Replace `Usage`, `companiesUsed`, delete `ownedOrganizations`:

```go
// Usage reports what an organization has consumed this period against what it
// may. Document meters from the counters; companies counted live.
func (s *BillingService) Usage(ctx context.Context, organizationID, companyPK string) (map[string]UsageMeter, error) {
	snap, err := s.snapshotFor(ctx, organizationID, companyPK)
	if err != nil {
		return nil, err
	}
	out := map[string]UsageMeter{}
	if organizationID == "" {
		return out, nil
	}
	counters, err := s.repo.GetUsage(ctx, organizationID, usagePeriod(snap))
	if err != nil {
		return nil, err
	}
	for _, meter := range DocumentMeters {
		if limit, ok := Quota(snap, meter); ok {
			out[meter] = UsageMeter{Used: counters[meter], Limit: limit}
		}
	}
	if limit, ok := Quota(snap, MeterCompanies); ok {
		used, err := s.companiesUsed(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		out[MeterCompanies] = UsageMeter{Used: used, Limit: limit}
	}
	return out, nil
}

// companiesUsed is how many of the organization's companies count against the
// plan: the enabled ones (ADR 0021). Without an enablement source it counts
// every linked company, the stricter answer.
func (s *BillingService) companiesUsed(ctx context.Context, organizationID string) (int64, error) {
	refs, err := s.orgs.CompaniesOf(ctx, organizationID)
	if err != nil {
		return 0, err
	}
	pks := make([]string, 0, len(refs))
	for _, r := range refs {
		pks = append(pks, r.PK)
	}
	if s.enablement == nil {
		return int64(len(pks)), nil
	}
	enabled, err := countEnabled(ctx, s.enablement, pks)
	if err != nil {
		return 0, err
	}
	return int64(enabled), nil
}
```

`CheckCompanyQuota` still calls `s.companiesUsed(ctx, raw)` with a user id after this step; leave it compiling (Task 7 deletes it).

10. The four emitters: `BillingUserID: reservation.UserID` → `BillingUserID: reservation.OrganizationID` for now (Task 10 moves it to its own field).

11. `api/internal/app/app.go` `newBillingService`: append `.WithUserFallback()` to the chain.

12. `api/internal/services/billing_test.go`: every `SnapshotFrom("user-N", …)` keeps its argument; the one assertion on `snap.UserID` (line ~246) becomes `snap.OrganizationID != "user-9"`.

- [ ] **Step 4: Run unit and integration tests to verify they pass**

Run: `go build ./... && go test ./internal/services/ ./internal/middleware/ ./internal/consumer/`
Then: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS (the whole integration package, so the reseeded money-path tests are covered).

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/billing.go api/internal/services/organizations.go api/internal/services/billing_test.go api/internal/services/nfes api/internal/services/mdfes api/internal/services/nfses api/internal/app/app.go api/tests/integration/billing_usage_test.go api/tests/integration/org_billing_test.go
git commit -m "feat(dfe): billing snapshot, counters and company count by ctech-account organization"
```

---

### Task 6: Managing the organization's plan (routes, customer, owner/admin)

**Files:**
- Modify: `api/internal/services/billing.go` (`BillingScope`, roles, `GetOrCreateCustomer`, `Choose`, `Change`, `Cancel`, `Invoices`)
- Modify: `api/internal/billingclient/client.go:302-312` (`CreateCustomerInput.TaxID`)
- Modify: `api/internal/middleware/rbac.go` (`RequireMember`)
- Modify: `api/internal/api/v1/billing.go` (routes, views)
- Modify: `api/internal/api/v1/router.go:106` (pass `perm`)
- Modify: `api/internal/app/app.go` (`newBillingService` takes the role service)
- Modify: `api/internal/api/v1/openapi/billing.yaml`
- Create: `api/internal/api/v1/billing_view_test.go`
- Modify: `api/tests/integration/org_billing_test.go` (management tests and a fuller billing stub)
- Modify: `DOCS.md` (billing section)

**Interfaces:**
- Consumes: Task 4 `WorkspaceRoleService.Role`, `.OrganizationName`, `MayManageBilling`; Task 5 `OrganizationOf`, `snapshotFor`, `Sync`, `Usage`.
- Produces:
  - `type services.BillingScope struct { CompanyPK, OrganizationID, UserID string }`
  - `func (s *BillingService) ScopeFor(ctx context.Context, companyPK, userID string) (*BillingScope, error)` — `ErrNoOrganization` when the company has none.
  - `func (s *BillingService) WithWorkspaceRoles(r workspaceRoles) *BillingService`; `type workspaceRoles interface { Role(ctx, organizationID, userID string) (string, error); OrganizationName(ctx, organizationID, userID string) string }`
  - `func (s *BillingService) CanManage(ctx context.Context, scope *BillingScope) (bool, error)`
  - `func (s *BillingService) OrganizationName(ctx context.Context, scope *BillingScope) string`
  - `func (s *BillingService) SnapshotOf(ctx context.Context, scope *BillingScope) (*repositories.AccountSnapshot, error)`
  - `type services.CustomerPayer struct { UserID, Name, Email string }`
  - `func (s *BillingService) GetOrCreateCustomer(ctx context.Context, organizationID string, payer CustomerPayer) (string, error)`
  - `func (s *BillingService) Choose(ctx context.Context, scope *BillingScope, accessToken string, priceIDs []string) (*repositories.AccountSnapshot, *billingclient.Invoice, error)`
  - `func (s *BillingService) Change(ctx context.Context, scope *BillingScope, priceIDs []string) (…same…)`
  - `func (s *BillingService) Cancel(ctx context.Context, scope *BillingScope, atPeriodEnd bool) (*repositories.AccountSnapshot, error)`
  - `func (s *BillingService) Invoices(ctx context.Context, scope *BillingScope, year, month int) ([]billingclient.Invoice, error)`
  - `func (p *PermChecker) RequireMember() fiber.Handler` — any local role, reach enforced, scoped API tokens refused.
  - `func v1.RegisterBilling(router fiber.Router, app *fiber.App, svc *services.BillingService, webhookSecret string, authMw fiber.Handler, perm *middleware.PermChecker)`
  - Wire: `GET /v1.0/billing/subscription` adds `organization: {id, name}` and `manageable: bool`.

- [ ] **Step 1: Write the failing view test** — `api/internal/api/v1/billing_view_test.go`

```go
package v1

import (
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestOrganizationViewNamesTheOrganizationAndWhetherTheCallerManagesIt(t *testing.T) {
	view := organizationSubscriptionView(&repositories.AccountSnapshot{
		OrganizationID: "org_1", SubscriptionID: "sub_1", Status: "ACTIVE", Plan: "pro",
	}, "Escritório Silva", true)

	org, ok := view["organization"].(map[string]string)
	if !ok || org["id"] != "org_1" || org["name"] != "Escritório Silva" {
		t.Fatalf("organization = %#v", view["organization"])
	}
	if view["manageable"] != true || view["has_subscription"] != true {
		t.Fatalf("view = %#v", view)
	}
	if view := organizationSubscriptionView(&repositories.AccountSnapshot{OrganizationID: "org_1"}, "", false); view["manageable"] != false {
		t.Fatalf("a member's view must not be manageable: %#v", view)
	}
}
```

- [ ] **Step 2: Write the failing management tests** — append to `api/tests/integration/org_billing_test.go`

```go
// fakeRoles stands in for ctech-account's membership route.
type fakeRoles struct {
	roles map[string]string // userID → role
	err   error
}

func (f fakeRoles) Role(_ context.Context, _, userID string) (string, error) {
	return f.roles[userID], f.err
}
func (f fakeRoles) OrganizationName(context.Context, string, string) string { return "Escritório" }

// managementStub answers what Choose/Change/Cancel need: the catalogue, the
// entitlements, customer and subscription creation. It records creations.
type managementStub struct {
	customers     []map[string]any
	subscriptions int
	entitled      bool
}

func (m *managementStub) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/products", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"prod_dfe_free","name":"Free","active":true,"prices":[{"id":"price_dfe_free","product_id":"prod_dfe_free","unit_amount":0,"metadata":{"plan":"free","quota_nfe":"3","quota_companies":"1"}}]}]}`))
	})
	mux.HandleFunc("/v1.0/entitlements", func(w http.ResponseWriter, _ *http.Request) {
		if !m.entitled {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"customer_id":"cus_org","entitled":true,"subscriptions":[{"id":"sub_org","status":"ACTIVE","entitled":true,"plan":"free","items":[{"price_id":"price_dfe_free","unit_amount":0,"metadata":{"plan":"free","quota_nfe":"3","quota_companies":"1"}}],"current_period":{"start":"2026-10-10","end":"2026-11-10"}}]}`))
	})
	mux.HandleFunc("/v1.0/customers", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		m.customers = append(m.customers, body)
		_, _ = w.Write([]byte(`{"id":"cus_org","external_ref":"` + body["external_ref"].(string) + `"}`))
	})
	mux.HandleFunc("/v1.0/subscriptions", func(w http.ResponseWriter, _ *http.Request) {
		m.subscriptions++
		m.entitled = true
		_, _ = w.Write([]byte(`{"subscription":{"id":"sub_org","customer_id":"cus_org","status":"ACTIVE"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// Spec § 5 test 3: member → 403 on choose/change/cancel; admin → allowed; both
// read the plan.
func TestOnlyOwnerAndAdminManageThePlan(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).WithWorkspaceRoles(fakeRoles{roles: map[string]string{
		"usr-admin": services.AccountRoleAdmin, "usr-member": "member",
	}})
	org := "org-manage-" + newCompanyPK(t)
	company := seedCompany(t, org, "usr-owner", "11222333000181", "Gerida Ltda")

	member, err := svc.ScopeFor(ctx, company, "usr-member")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, member, "", []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member choose: %v, want 403", err)
	}
	if _, _, err := svc.Change(ctx, member, []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member change: %v, want 403", err)
	}
	if _, err := svc.Cancel(ctx, member, true); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member cancel: %v, want 403", err)
	}
	if _, err := svc.Invoices(ctx, member, 2026, 10); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("member invoices: %v, want 403", err)
	}
	if _, err := svc.SnapshotOf(ctx, member); err != nil {
		t.Fatalf("a member reads the plan: %v", err)
	}

	admin, err := svc.ScopeFor(ctx, company, "usr-admin")
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := svc.Choose(ctx, admin, "", []string{"price_dfe_free"})
	if err != nil || snap.SubscriptionID != "sub_org" || snap.OrganizationID != org {
		t.Fatalf("admin choose = %+v (%v)", snap, err)
	}
	if len(stub.customers) != 1 || stub.customers[0]["external_ref"] != repositories.OrgBillingPK(org) {
		t.Fatalf("customers = %+v", stub.customers)
	}
	// The organization's billing company names the invoice.
	if stub.customers[0]["tax_id"] != "11222333000181" || stub.customers[0]["name"] != "Gerida Ltda" {
		t.Fatalf("customer identity = %+v", stub.customers[0])
	}
	if ok, _ := svc.CanManage(ctx, member); ok {
		t.Fatal("a member must not be reported manageable")
	}
}

// Review Focus 2: ctech-account down → 403, never a grant; reading still works.
func TestManagingFailsClosedWhenTheRoleCannotBeRead(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).WithWorkspaceRoles(fakeRoles{err: errors.New("down")})
	company := seedCompany(t, "org-down-"+newCompanyPK(t), "usr-owner", "11222333000181", "Fora Ltda")
	scope, err := svc.ScopeFor(ctx, company, "usr-owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, scope, "", []string{"price_dfe_free"}); !isStatus(err, http.StatusForbidden) {
		t.Fatalf("choose during an outage: %v, want 403", err)
	}
	if stub.subscriptions != 0 {
		t.Fatal("nothing may be created when the role is unknown")
	}
	if ok, err := svc.CanManage(ctx, scope); ok || err == nil {
		t.Fatalf("CanManage = %v, %v", ok, err)
	}
	if _, err := svc.SnapshotOf(ctx, scope); err != nil {
		t.Fatalf("reading must survive the outage: %v", err)
	}
}

// isStatus reports a problem with the given HTTP status.
func isStatus(err error, status int) bool {
	var p *problem.Problem
	return errors.As(err, &p) && p.Status == status
}
```

Add imports `encoding/json`, `errors`, `net/http`, `net/http/httptest`, `gopkg.aoctech.app/dfe/api/internal/problem` to the file.

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/api/v1/ -run TestOrganizationView` and `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestOnlyOwnerAndAdmin|TestManagingFailsClosed' -v`
Expected: FAIL — `undefined: organizationSubscriptionView`, `svc.WithWorkspaceRoles undefined`, `svc.ScopeFor undefined`.

- [ ] **Step 4: Implement the service** — `api/internal/services/billing.go`

```go
// workspaceRoles is the role check ScopeFor's callers are authorized by.
// *WorkspaceRoleService satisfies it; a nil one refuses (fail closed).
type workspaceRoles interface {
	Role(ctx context.Context, organizationID, userID string) (string, error)
	OrganizationName(ctx context.Context, organizationID, userID string) string
}

// WithWorkspaceRoles wires who may manage an organization's plan.
func (s *BillingService) WithWorkspaceRoles(r workspaceRoles) *BillingService {
	s.roles = r
	return s
}

// BillingScope is what a /billing route acts on: the organization of the
// selected company, on behalf of one person.
type BillingScope struct {
	CompanyPK      string
	OrganizationID string
	UserID         string
}

// ScopeFor resolves the selected company to its organization.
func (s *BillingService) ScopeFor(ctx context.Context, companyPK, userID string) (*BillingScope, error) {
	organizationID, err := s.OrganizationOf(ctx, companyPK)
	if err != nil {
		return nil, err
	}
	if organizationID == "" {
		return nil, ErrNoOrganization
	}
	return &BillingScope{CompanyPK: companyPK, OrganizationID: organizationID, UserID: repositories.RawUserID(userID)}, nil
}

// manageDenied is the single answer to every refusal to manage, outage
// included: like reach, the cause goes to the log, not to the caller.
const manageDenied = "apenas proprietários e administradores da organização podem gerenciar o plano"

// CanManage reports whether the person is owner or admin of the organization
// in ctech-account. An error is an outage; the caller must read it as "no".
func (s *BillingService) CanManage(ctx context.Context, scope *BillingScope) (bool, error) {
	if s.roles == nil {
		return false, fmt.Errorf("no workspace role check is wired")
	}
	role, err := s.roles.Role(ctx, scope.OrganizationID, scope.UserID)
	if err != nil {
		return false, err
	}
	return MayManageBilling(role), nil
}

func (s *BillingService) requireManager(ctx context.Context, scope *BillingScope) error {
	ok, err := s.CanManage(ctx, scope)
	if err != nil {
		slog.WarnContext(ctx, "billing: could not read the role; refusing", "organization_id", scope.OrganizationID, "error", err)
		return problem.Forbidden(manageDenied)
	}
	if !ok {
		return problem.Forbidden(manageDenied)
	}
	return nil
}

// OrganizationName is the display name for the plan screen, "" when unknown.
func (s *BillingService) OrganizationName(ctx context.Context, scope *BillingScope) string {
	if s.roles == nil {
		return ""
	}
	return s.roles.OrganizationName(ctx, scope.OrganizationID, scope.UserID)
}

// SnapshotOf is the organization's standing as the routes read it (dual read
// included).
func (s *BillingService) SnapshotOf(ctx context.Context, scope *BillingScope) (*repositories.AccountSnapshot, error) {
	return s.snapshotFor(ctx, scope.OrganizationID, scope.CompanyPK)
}

// migratingConflict refuses a mutation on a plan inherited through the dual
// read: changing it would change the owner's pre-migration subscription.
var migratingConflict = problem.Conflict("o plano desta organização está sendo migrado; tente novamente em instantes")

// CustomerPayer is who acts when the organization's customer is created: the
// fallback name and the e-mail billing writes to.
type CustomerPayer struct {
	UserID string
	Name   string
	Email  string
}

// GetOrCreateCustomer returns ORG_{organizationID}'s billing customer,
// registering it on first use with the organization's billing company (its
// first linked company: legal name and tax id) and, failing that, the payer's
// name. Billing writes the CUSTOMER_ORG# pointer (its spec § 8).
func (s *BillingService) GetOrCreateCustomer(ctx context.Context, organizationID string, payer CustomerPayer) (string, error) {
	if !s.Enabled() {
		return "", billingclient.ErrNotConfigured
	}
	externalRef := repositories.OrgBillingPK(organizationID)
	if snap, err := s.Snapshot(ctx, organizationID); err == nil && snap.CustomerID != "" {
		return snap.CustomerID, nil
	}
	switch existing, err := s.client.GetEntitlements(ctx, externalRef); {
	case err == nil && existing.CustomerID != "":
		return existing.CustomerID, nil
	case err != nil && !errors.Is(err, billingclient.ErrCustomerNotFound):
		return "", err
	}

	name, taxID, err := s.billingCompany(ctx, organizationID)
	if err != nil {
		return "", err
	}
	if name == "" {
		name = payer.Name
	}
	customer, err := s.client.CreateCustomer(ctx, billingclient.CreateCustomerInput{
		ExternalRef: externalRef,
		UserID:      repositories.RawUserID(payer.UserID),
		Name:        name,
		Email:       payer.Email,
		TaxID:       taxID,
	})
	if err != nil {
		return "", err
	}
	return customer.ID, nil
}

// billingCompany is the organization's first linked company, as this product
// knows it (organization-index, oldest first).
func (s *BillingService) billingCompany(ctx context.Context, organizationID string) (name, taxID string, err error) {
	refs, err := s.orgs.CompaniesOf(ctx, organizationID)
	if err != nil || len(refs) == 0 {
		return "", "", err
	}
	company, err := s.orgs.Company(ctx, refs[0].PK)
	if err != nil || company == nil {
		return "", "", err
	}
	return company.LegalName, company.TaxID, nil
}
```

`Choose`, `Change`, `Cancel`, `Invoices` take `scope *BillingScope` in place of `userID`. Each starts (after the `Enabled` and empty-price guards) with:

```go
	if err := s.requireManager(ctx, scope); err != nil {
		return nil, nil, err
	}
	snap, err := s.SnapshotOf(ctx, scope)
	if err != nil {
		return nil, nil, err
	}
```

(with the return arity of each function). Then:
- `Choose`: keep the "já tem uma assinatura" conflict (an inherited snapshot has a subscription, so it is refused too); message becomes "esta organização já tem uma assinatura; use a troca de plano". Then:

```go
	if err := s.validatePrices(ctx, priceIDs); err != nil {
		return nil, nil, err
	}
	payer := CustomerPayer{UserID: scope.UserID}
	// users is nil only where no person is acting (tests, the migration
	// command, which calls GetOrCreateCustomer with its own payer).
	if s.users != nil {
		profile, err := s.users.GetUserInfo(ctx, accessToken)
		if err != nil {
			return nil, nil, problem.InternalServer("não foi possível ler o perfil da conta para iniciar a assinatura")
		}
		payer.Name, payer.Email = actorNameFromProfile(profile), profile.Email
	}
	customerID, err := s.GetOrCreateCustomer(ctx, scope.OrganizationID, payer)
	if err != nil {
		return nil, nil, err
	}
	res, err := s.client.CreateSubscription(ctx, customerID, itemsOf(priceIDs),
		subscribeIdempotencyKey(repositories.OrgBillingPK(scope.OrganizationID), priceIDs))
	if err != nil {
		return nil, nil, err
	}
	fresh, err := s.Sync(ctx, scope.OrganizationID)
	if err != nil {
		return nil, nil, err
	}
	return fresh, res.Invoice, nil
```
- `Change` and `Cancel`: after the no-subscription check add `if snap.InheritedFromUser { return …, migratingConflict }`; end with `s.Sync(ctx, scope.OrganizationID)`. Messages say "esta organização" instead of "esta conta".
- `Invoices`: manager only; filter by `snap.SubscriptionID` as today.

`api/internal/billingclient/client.go`, `CreateCustomerInput`: update the `ExternalRef` comment to "`ORG_{organization_id}` (the DF-e's customers since 2026-10-10)" and add:

```go
	// TaxID is the CNPJ (or CPF) printed on the invoice: the organization's
	// billing company.
	TaxID string `json:"tax_id,omitempty"`
```

`api/internal/middleware/rbac.go`, after `RequireOwnerOrAdmin`:

```go
// RequireMember allows anybody with access to the company, whatever their
// role. For reads whose sensitivity is the company itself (the billing plan);
// the action-level decision is made downstream.
func (p *PermChecker) RequireMember() fiber.Handler {
	return p.requireRoles(accessDenied, roleOwner, roleAdmin, repositories.RoleUser, repositories.RoleViewer)
}
```

- [ ] **Step 5: Implement the routes** — `api/internal/api/v1/billing.go`

Replace the file's top comment (the "takes no organization header" paragraphs) with:

```go
// The organization's billing surface (docs/specs/2026-10-10-organization-subscription.md § 2).
//
// Every /billing/subscription* and /billing/invoices route requires the
// Dfe-Organization-Pk header: the subscription addressed is that of the
// selected company's ctech-account organization. Reading the plan and usage is
// open to anybody with access to the company; choosing, changing, cancelling
// and listing invoices require the ctech-account role owner or admin, checked
// in BillingService (403 otherwise, outage included).
```

Change the signature to `RegisterBilling(router fiber.Router, app *fiber.App, svc *services.BillingService, webhookSecret string, authMw fiber.Handler, perm *middleware.PermChecker)` and `router.go:106` to pass `perm`. `/billing/plans` stays without the header. Mount the rest behind `perm.RequireMember()` and resolve the scope in each handler with:

```go
// scopeOf resolves the selected company to the organization a billing route
// acts on.
func scopeOf(c fiber.Ctx, svc *services.BillingService) (*services.BillingScope, error) {
	return svc.ScopeFor(c.Context(), middleware.GetOrgPK(c), middleware.GetUserID(c))
}
```

GET `/subscription`:

```go
	billing.Get("/subscription", perm.RequireMember(), func(c fiber.Ctx) error {
		scope, err := scopeOf(c, svc)
		if err != nil {
			return sendProblem(c, err)
		}
		snap, err := svc.SnapshotOf(c.Context(), scope)
		if err != nil {
			return sendProblem(c, err)
		}
		usage, err := svc.Usage(c.Context(), scope.OrganizationID, scope.CompanyPK)
		if err != nil {
			return sendProblem(c, err)
		}
		// An outage reading the role shows the plan read-only; it never fails
		// the read.
		manageable, _ := svc.CanManage(c.Context(), scope)
		view := organizationSubscriptionView(snap, svc.OrganizationName(c.Context(), scope), manageable)
		view["usage"] = usage
		return c.JSON(view)
	})
```

POST `/subscription`, `/subscription/change`, `/subscription/cancel`, GET `/invoices`: same `perm.RequireMember()` + `scopeOf`, then `svc.Choose(c.Context(), scope, currentAccessToken(c), body.PriceIDs)`, `svc.Change(c.Context(), scope, body.PriceIDs)`, `svc.Cancel(c.Context(), scope, body.AtPeriodEnd)`, `svc.Invoices(c.Context(), scope, year, month)`.

Add the view:

```go
// organizationSubscriptionView is subscriptionView plus who the plan belongs
// to and whether the caller may act on it. `manageable` is the UI's only input
// for showing the buttons that spend money.
func organizationSubscriptionView(s *repositories.AccountSnapshot, organizationName string, manageable bool) fiber.Map {
	out := subscriptionView(s)
	out["organization"] = map[string]string{"id": s.OrganizationID, "name": organizationName}
	out["manageable"] = manageable
	return out
}
```

`RegisterOrganizationPlan` keeps its route and permission; update its comment to "answers for the organization of that company" and leave `manageable: false`.

`api/internal/app/app.go` `newBillingService`: add parameter `roles *services.WorkspaceRoleService` and chain `.WithWorkspaceRoles(roles)` (a nil `*WorkspaceRoleService` refuses every management attempt, which is the intended fail-closed behaviour).

`api/internal/api/v1/openapi/billing.yaml`: on every `/billing/subscription*` and `/billing/invoices` operation add the `Dfe-Organization-Pk` header parameter (reuse the component the other org-scoped files reference; `grep -n "Dfe-Organization-Pk" api/internal/api/v1/openapi/*.yaml`), document 403 for POSTs and `/invoices`, 409 `ErrNoOrganization`, and add `organization {id, name}` and `manageable` to the subscription response schema.

`DOCS.md`, billing section: the routes now act on the selected company's organization; owner/admin manage, others read; the new response fields; 403/409.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/api/v1/ ./internal/services/ ./internal/middleware/` (includes `openapi_test.go`)
Then: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/billing.go api/internal/billingclient/client.go api/internal/middleware/rbac.go api/internal/api/v1/billing.go api/internal/api/v1/billing_view_test.go api/internal/api/v1/router.go api/internal/app/app.go api/internal/api/v1/openapi/billing.yaml api/tests/integration/org_billing_test.go DOCS.md
git commit -m "feat(dfe): organization plan managed by its ctech-account owners and admins"
```

---

### Task 7: `ReserveCompany`, the companies-level marker, and the legacy create route stops metering

The on-demand company charge is **monthly by peak** (amendment of 2026-10-10 to the spec; billing plans spec § 6, price `price_dfe_ondemand_companies_monthly`, `aggregation: max`, meter `dfe_companies`). So an enablement no longer reports a one-shot usage unit: it changes the organization's **level** (its count of enabled companies), and every change marks that level dirty in the same transaction. Task 8 delivers dirty levels to billing durably.

**Files:**
- Modify: `cdk/lib/dynamodb-stack.ts:404-435` (account_billing: sparse GSI `level-dirty-index`)
- Modify: `cdk/test/dynamodb-stack.test.ts`
- Modify: `api/internal/repositories/account_billing.go` (level marker)
- Modify: `api/tests/integration/setup_test.go:500-510` (GSI on the local `account_billing`)
- Modify: `api/internal/services/billing.go` (add `CompanyReservation`, `ReserveCompany`, `MeterLevelCompanies`; delete `CheckCompanyQuota`, `CompanyQuotaGuard`, `ReportCompanyUsage`, `companyUsageKeyPrefix`)
- Modify: `api/internal/api/v1/organizations.go:107-127` (legacy create route)
- Modify: `api/tests/integration/org_billing_test.go`
- Modify: `DynamoDB-Tables.md` (`LEVEL_DIRTY_` row, GSI)

**Interfaces:**
- Consumes: Task 5 `snapshotFor`, `companiesUsed`; Task 2 `BuildQuotaGuardTx(ctx, accountID, meter)`; `enablementSource.ConfiguredDocTypes`.
- Produces:
  - `const services.MeterLevelCompanies = "dfe_companies"` (billing's level meter; distinct from the quota meter `companies`)
  - `const repositories.LevelDirtyIndex = "level-dirty-index"`
  - `func repositories.LevelMarkerPK(organizationID, meter string) string` → `LEVEL_DIRTY_{organization_id}#{meter}`
  - `type repositories.LevelMarker struct { OrganizationID, Meter, ChangedAt string; Version int64; Dirty bool }`
  - `func (r *AccountBillingRepository) BuildMarkLevelDirtyTx(organizationID, meter string) types.TransactWriteItem`
  - `func (r *AccountBillingRepository) MarkLevelDirty(ctx context.Context, organizationID, meter string) error`
  - `func (r *AccountBillingRepository) GetLevelMarker(ctx context.Context, organizationID, meter string) (*LevelMarker, error)`
  - `func (r *AccountBillingRepository) ClearLevelDirty(ctx context.Context, organizationID, meter string, version int64) error` — no-op when a newer change bumped the version.
  - `func (r *AccountBillingRepository) ListDirtyLevels(ctx context.Context) ([]LevelMarker, error)`
  - `type services.CompanyReservation struct { Items []types.TransactWriteItem; OrganizationID, CompanyPK string }` — `Items` empty means "not an enablement" (already enabled, or billing off).
  - `func (s *BillingService) ReserveCompany(ctx context.Context, organizationID, companyPK string) (*CompanyReservation, error)` — on an enablement, `Items` = quota guard (when the plan caps companies) + level marker.
  - `func (s *BillingService) CompanyLevelChangeTx(organizationID string) (types.TransactWriteItem, bool)` — the marker item for any **disable** path (none exists today: no route deletes a fiscal configuration or a company); `false` when billing is off.

- [ ] **Step 1: Write the failing CDK test** — append to `cdk/test/dynamodb-stack.test.ts`

```ts
describe('DynamoDBStack — níveis de cobrança pendentes', () => {
    test('account_billing tem o GSI esparso level-dirty-index', () => {
        const template = synth();
        template.hasResourceProperties('AWS::DynamoDB::GlobalTable', {
            TableName: 'dev_dfe_account_billing',
            GlobalSecondaryIndexes: Match.arrayWith([
                Match.objectLike({
                    IndexName: 'level-dirty-index',
                    KeySchema: [
                        {AttributeName: 'dirty_shard', KeyType: 'HASH'},
                        {AttributeName: 'changed_at', KeyType: 'RANGE'},
                    ],
                }),
            ]),
        });
    });
});
```

- [ ] **Step 2: Write the failing integration tests** — append to `api/tests/integration/org_billing_test.go`

```go
func companyQuotaBilling(t *testing.T, calls *[]usageCall) *services.BillingService {
	t.Helper()
	return chargingBilling(t, billingStub(t, calls)).WithEnablement(services.NewFiscalConfigEnablement(
		nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
}

// Spec § 3 O5: the company quota applies when a company becomes enabled.
func TestReserveCompanyRefusesAboveTheLimitAndIgnoresLinkedCompanies(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-reserve-" + newCompanyPK(t)
	enabled := seedCompany(t, org, "owner-r", "11222333000181", "Um Ltda")
	seedNfeConfig(t, enabled)
	// Spec § 5 test 8: linked companies above the limit are fine; they cost nothing.
	for _, tax := range []string{"11222333000262", "11222333000343"} {
		_ = seedCompany(t, org, "owner-r", tax, "Vinculada Ltda")
	}
	snap := proSnapshot("sub_reserve")
	snap.Quotas[services.MeterCompanies] = 1
	seedOrgSnapshot(t, org, snap)
	candidate := seedCompany(t, org, "owner-r", "11222333000424", "Nova Ltda")

	if _, err := svc.ReserveCompany(ctx, org, candidate); !isStatus(err, http.StatusPaymentRequired) {
		t.Fatalf("second enabled company on a 1-company plan: %v, want 402", err)
	}
	// An already-enabled company saving another configuration is not a change.
	r, err := svc.ReserveCompany(ctx, org, enabled)
	if err != nil || len(r.Items) != 0 {
		t.Fatalf("already enabled: %+v %v", r, err)
	}
}

// An enablement carries the guard and the level marker; committing them marks
// the organization's dfe_companies level dirty with a new version.
func TestAnEnablementMarksTheCompaniesLevelDirty(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-level-" + newCompanyPK(t)
	seedOrgSnapshot(t, org, proSnapshot("sub_level"))
	company := seedCompany(t, org, "owner-l", "11222333000181", "Nível Ltda")

	r, err := svc.ReserveCompany(ctx, org, company)
	if err != nil || len(r.Items) != 2 {
		t.Fatalf("enablement items = %+v (%v), want guard + marker", r, err)
	}
	nfe := services.NewNfeConfigService(nfeConfigRepo, auditRepo)
	fields := map[string]types.AttributeValue{"prod_current_serie": &types.AttributeValueMemberN{Value: "1"}}
	if _, err := nfe.Upsert(ctx, company, fields, "owner-l", "Dono", r.Items...); err != nil {
		t.Fatal(err)
	}
	m, err := repositories.NewAccountBillingRepository(db, cfg).GetLevelMarker(ctx, org, services.MeterLevelCompanies)
	if err != nil || m == nil || !m.Dirty || m.Version != 1 || m.ChangedAt == "" {
		t.Fatalf("marker = %+v (%v)", m, err)
	}
}

// Levels are reported whatever the plan: a plan with no company quota still
// marks the level, so it is known the day the organization moves to on-demand.
func TestAnUncappedPlanStillMarksTheLevel(t *testing.T) {
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-uncapped-" + newCompanyPK(t)
	snap := proSnapshot("sub_uncapped")
	snap.Quotas[services.MeterCompanies] = services.QuotaUnlimited
	seedOrgSnapshot(t, org, snap)
	company := seedCompany(t, org, "owner-u", "11222333000181", "Livre Ltda")
	r, err := svc.ReserveCompany(context.Background(), org, company)
	if err != nil || len(r.Items) != 1 {
		t.Fatalf("items = %+v (%v), want only the marker", r, err)
	}
}

func TestClearLevelDirtyKeepsANewerChange(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewAccountBillingRepository(db, cfg)
	org := "org-clear-" + newCompanyPK(t)
	for range 2 {
		if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
			t.Fatal(err)
		}
	}
	// Clearing the version a reporter read before the second change must not
	// hide the second change.
	if err := repo.ClearLevelDirty(ctx, org, services.MeterLevelCompanies, 1); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); !m.Dirty {
		t.Fatal("a stale clear removed a newer change")
	}
	if err := repo.ClearLevelDirty(ctx, org, services.MeterLevelCompanies, 2); err != nil {
		t.Fatal(err)
	}
	m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies)
	if m.Dirty {
		t.Fatal("the current version was not cleared")
	}
	dirty, err := repo.ListDirtyLevels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirty {
		if d.OrganizationID == org {
			t.Fatal("a clean marker is still listed")
		}
	}
}

// Review Focus 1.
func TestReserveCompanyRefusesACompanyWithNoOrganization(t *testing.T) {
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	if _, err := svc.ReserveCompany(context.Background(), "", newCompanyPK(t)); !isStatus(err, http.StatusConflict) {
		t.Fatalf("err = %v, want 409", err)
	}
}
```

In `api/tests/integration/setup_test.go`, give the local `_account_billing` table the same GSI:

```go
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String("pk"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("dirty_shard"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("changed_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{{
				IndexName: aws.String(repositories.LevelDirtyIndex),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("dirty_shard"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("changed_at"), KeyType: types.KeyTypeRange},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			}},
```

- [ ] **Step 3: Run them to verify they fail**

Run: `cd cdk && npx jest test/dynamodb-stack.test.ts` and `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestReserveCompany|TestAnEnablementMarks|TestAnUncappedPlan|TestClearLevelDirty' -v`
Expected: jest FAIL (no GSI); go FAIL — `svc.ReserveCompany undefined`, `repositories.LevelDirtyIndex undefined`.

- [ ] **Step 4: Add the GSI** — `cdk/lib/dynamodb-stack.ts`, before `this.tables.set('account_billing', accountBillingTable);` (and change the table comment's "No GSI." to "One sparse GSI, for the dirty levels."):

```ts
    // Dirty billing levels (LEVEL_DIRTY_{organization}#{meter}): an
    // organization whose count of enabled companies changed and has not been
    // reported to billing yet. Sparse: a marker leaves the index when its
    // report succeeds (dirty_shard removed), so the sweeper's Query reads only
    // what is pending. One shard value; the set is tiny by construction.
    accountBillingTable.addGlobalSecondaryIndex({
      indexName: 'level-dirty-index',
      partitionKey: {name: 'dirty_shard', type: dynamodb.AttributeType.STRING},
      sortKey: {name: 'changed_at', type: dynamodb.AttributeType.STRING},
      projectionType: dynamodb.ProjectionType.ALL,
      warmThroughput: undefined,
      maxReadRequestUnits: 1000,
      maxWriteRequestUnits: 1000,
    });
```

- [ ] **Step 5: Implement the marker** — `api/internal/repositories/account_billing.go` (add the row to the type comment: `pk = LEVEL_DIRTY_{organization_id}#{meter}  a billing level changed and not yet reported`):

```go
// LevelDirtyIndex lists the levels waiting to be reported (sparse).
const LevelDirtyIndex = "level-dirty-index"

const (
	attrDirtyShard   = "dirty_shard"
	dirtyShardLevels = "level"
	attrChangedAt    = "changed_at"
	attrLevelVersion = "version"
	attrLevelMeter   = "meter"
)

// LevelMarkerPK keys one organization's marker for one level meter.
func LevelMarkerPK(organizationID, meter string) string {
	return "LEVEL_DIRTY_" + organizationID + "#" + meter
}

// LevelMarker says a level changed. Version grows by one per change; the
// reporter clears only the version it reported, so a change made while a
// report was in flight stays dirty.
type LevelMarker struct {
	OrganizationID string `dynamodbav:"organization_id"`
	Meter          string `dynamodbav:"meter"`
	ChangedAt      string `dynamodbav:"changed_at"`
	Version        int64  `dynamodbav:"version"`
	Dirty          bool   `dynamodbav:"-"`
}

// BuildMarkLevelDirtyTx bumps the marker, for the transaction that changes
// the level (a fiscal configuration that enables a company).
func (r *AccountBillingRepository) BuildMarkLevelDirtyTx(organizationID, meter string) types.TransactWriteItem {
	return types.TransactWriteItem{Update: &types.Update{
		TableName: aws.String(r.TableName),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: LevelMarkerPK(organizationID, meter)}},
		UpdateExpression: aws.String("ADD #v :one SET #o = :o, #m = :m, #s = :s, #c = :now"),
		ExpressionAttributeNames: map[string]string{
			"#v": attrLevelVersion, "#o": "organization_id", "#m": attrLevelMeter, "#s": attrDirtyShard, "#c": attrChangedAt,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one": &types.AttributeValueMemberN{Value: "1"},
			":o":   &types.AttributeValueMemberS{Value: organizationID},
			":m":   &types.AttributeValueMemberS{Value: meter},
			":s":   &types.AttributeValueMemberS{Value: dirtyShardLevels},
			":now": &types.AttributeValueMemberS{Value: NowStr()},
		},
	}}
}

// MarkLevelDirty is BuildMarkLevelDirtyTx on its own (the migration command).
func (r *AccountBillingRepository) MarkLevelDirty(ctx context.Context, organizationID, meter string) error {
	return r.TransactWrite(ctx, []types.TransactWriteItem{r.BuildMarkLevelDirtyTx(organizationID, meter)})
}

func decodeLevelMarker(item map[string]types.AttributeValue) (*LevelMarker, error) {
	var m LevelMarker
	if err := attributevalue.UnmarshalMap(item, &m); err != nil {
		return nil, fmt.Errorf("decoding a level marker: %w", err)
	}
	_, m.Dirty = item[attrDirtyShard]
	return &m, nil
}

// GetLevelMarker reads a marker; nil when the level never changed.
func (r *AccountBillingRepository) GetLevelMarker(ctx context.Context, organizationID, meter string) (*LevelMarker, error) {
	item, err := r.GetItem(ctx, LevelMarkerPK(organizationID, meter))
	if err != nil || item == nil {
		return nil, err
	}
	return decodeLevelMarker(item)
}

// ClearLevelDirty takes the marker out of the index if it is still at version.
// A newer change (higher version) keeps it dirty: that change is not reported.
func (r *AccountBillingRepository) ClearLevelDirty(ctx context.Context, organizationID, meter string, version int64) error {
	_, err := r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		TableName:                aws.String(r.TableName),
		Key:                      map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: LevelMarkerPK(organizationID, meter)}},
		UpdateExpression:         aws.String("REMOVE #s"),
		ConditionExpression:      aws.String("#v = :v"),
		ExpressionAttributeNames: map[string]string{"#s": attrDirtyShard, "#v": attrLevelVersion},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":v": &types.AttributeValueMemberN{Value: strconv.FormatInt(version, 10)},
		},
	})
	if IsConditionFailed(err) {
		return nil
	}
	return wrapDynamoErr(err)
}

// ListDirtyLevels reads every pending marker, oldest change first.
func (r *AccountBillingRepository) ListDirtyLevels(ctx context.Context) ([]LevelMarker, error) {
	var out []LevelMarker
	var start map[string]types.AttributeValue
	for {
		res, err := r.QueryRaw(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(r.TableName),
			IndexName:                 aws.String(LevelDirtyIndex),
			KeyConditionExpression:    aws.String("#s = :s"),
			ExpressionAttributeNames:  map[string]string{"#s": attrDirtyShard},
			ExpressionAttributeValues: map[string]types.AttributeValue{":s": &types.AttributeValueMemberS{Value: dirtyShardLevels}},
			ExclusiveStartKey:         start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range res.Items {
			m, err := decodeLevelMarker(item)
			if err != nil {
				return nil, err
			}
			out = append(out, *m)
		}
		if len(res.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = res.LastEvaluatedKey
	}
}
```

(`ListUserSnapshots` from Task 2 filters on the `USER_` prefix, so marker rows never decode as snapshots.)

- [ ] **Step 6: Implement `ReserveCompany`** — `api/internal/services/billing.go`. Delete `CheckCompanyQuota`, `CompanyQuotaGuard`, `ReportCompanyUsage` and `companyUsageKeyPrefix`; add:

```go
// MeterLevelCompanies is billing's level meter for an organization's enabled
// companies (billing plans spec § 6; price price_dfe_ondemand_companies_monthly,
// aggregation max). Not the quota meter MeterCompanies: the quota reads the
// plan's `quota_companies`, the level is what the on-demand price bills by peak.
const MeterLevelCompanies = "dfe_companies"

// CompanyReservation is what ReserveCompany hands the fiscal configuration
// write: items for the same transaction. Empty Items: not an enablement.
type CompanyReservation struct {
	Items          []types.TransactWriteItem
	OrganizationID string
	CompanyPK      string
}

// ReserveCompany checks the company quota where a company starts to count, its
// first fiscal configuration (spec O5), and marks the organization's companies
// level dirty in the same transaction. It refuses with 402 when the enabled
// companies already fill the plan; companies already enabled keep emitting
// whatever the count (O6).
//
// The guard serializes this decision with every other enablement of the
// organization: two companies racing for the last slot both read N-1, and only
// the first transaction to bump the guard's version commits. The marker is
// written whatever the plan, so the level is known the day the organization
// moves to on-demand.
func (s *BillingService) ReserveCompany(ctx context.Context, organizationID, companyPK string) (*CompanyReservation, error) {
	if !s.Enabled() {
		return &CompanyReservation{}, nil
	}
	if organizationID == "" {
		return nil, ErrNoOrganization
	}
	if s.enablement != nil {
		docTypes, err := s.enablement.ConfiguredDocTypes(ctx, companyPK)
		if err != nil {
			return nil, err
		}
		if len(docTypes) > 0 {
			return &CompanyReservation{}, nil
		}
	}
	snap, err := s.snapshotFor(ctx, organizationID, companyPK)
	if err != nil {
		return nil, err
	}
	if !GrantsService(snap) {
		return nil, BlockedProblem(snap)
	}
	limit, ok := Quota(snap, MeterCompanies)
	if !ok {
		return nil, problem.QuotaExceeded(MeterCompanies, snap.Plan, 0, 0,
			"o plano da organização não permite habilitar empresas")
	}
	out := &CompanyReservation{OrganizationID: organizationID, CompanyPK: companyPK}
	if limit >= 0 {
		used, err := s.companiesUsed(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		if used >= limit {
			return nil, problem.QuotaExceeded(MeterCompanies, snap.Plan, limit, used,
				fmt.Sprintf("o plano da organização permite %d empresa(s) habilitada(s) e já há %d", limit, used))
		}
		guard, err := s.repo.BuildQuotaGuardTx(ctx, organizationID, MeterCompanies)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, guard)
	}
	out.Items = append(out.Items, s.repo.BuildMarkLevelDirtyTx(organizationID, MeterLevelCompanies))
	return out, nil
}

// CompanyLevelChangeTx is the marker any path that DISABLES a company must add
// to its own transaction (removing the last fiscal configuration, deleting or
// unlinking a company). No such path exists today; whoever adds one includes
// this, and Task 8's reporter does the rest.
func (s *BillingService) CompanyLevelChangeTx(organizationID string) (types.TransactWriteItem, bool) {
	if !s.Enabled() || organizationID == "" {
		return types.TransactWriteItem{}, false
	}
	return s.repo.BuildMarkLevelDirtyTx(organizationID, MeterLevelCompanies), true
}
```

Update the `DocumentMeters` comment ("deleting an organization must give the slot back" → "disabling a company gives the slot back"), and add one line to the "People are not metered" note: "The users quota left the catalogue on 2026-10-10 (spec O4)."

- [ ] **Step 7: The legacy create route** — `api/internal/api/v1/organizations.go`, `POST /organizations`: delete the `CompanyQuotaGuard` block and the `ReportCompanyUsage` call; call `h.OrgSvc.CreateWithOwner(c.Context(), dto.CpfOrCnpj, userID, userName, av, pfx, password)` with no quota items; replace the deleted comment with:

```go
		// No company quota here any more: a company counts when it is enabled
		// (its first fiscal configuration, BillingService.ReserveCompany), not
		// when it is registered. A company created by this legacy route has no
		// ctech-account organization, so ReserveCompany refuses to enable it
		// (409): it must be created through the CTech account and linked.
```

Remove `types` from the file's imports if it becomes unused.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `cd cdk && npx jest test/dynamodb-stack.test.ts`; `go build ./... && go vet ./...`; `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS. `grep -rn "CompanyQuotaGuard\|CheckCompanyQuota\|ReportCompanyUsage\|ownedOrganizations(" api/` returns nothing.

- [ ] **Step 9: Document** — `DynamoDB-Tables.md`, `account_billing`: the `LEVEL_DIRTY_{organization_id}#{meter}` row (version, changed_at, dirty_shard while pending) and the sparse `level-dirty-index` GSI.

- [ ] **Step 10: Commit**

```bash
git add cdk/lib/dynamodb-stack.ts cdk/test/dynamodb-stack.test.ts api/internal/repositories/account_billing.go api/internal/services/billing.go api/internal/api/v1/organizations.go api/tests/integration/setup_test.go api/tests/integration/org_billing_test.go DynamoDB-Tables.md
git commit -m "feat(dfe): company quota reserved on first enablement, companies level marked for billing"
```

---

### Task 8: Durable delivery of the companies level

A dirty marker is delivered by `LevelReporter`: right after the transaction that wrote it (best effort), and by a sweeper that every API instance runs on a ticker. The report is the **whole** count (`companiesUsed`), keyed by the marker's version, with `occurred_at` = the marker's `changed_at`, so a retry sends the identical body and billing dedupes it; a newer report repairs any lost one. Nothing is logged and forgotten: a marker leaves the index only when billing accepted its version.

**Files:**
- Modify: `api/internal/billingclient/client.go` (`LevelReport`, `ReportLevel`)
- Create: `api/internal/billingclient/levels_test.go`
- Create: `api/internal/services/levels.go`
- Modify: `api/internal/services/billing.go` (`levelFlush` hook; `Choose` and `Change` mark and flush the level)
- Modify: `api/internal/app/app.go` (`newLevelReporter`, `startLevelSweeper`)
- Modify: `api/tests/integration/org_billing_test.go`
- Modify: `DOCS.md` (billing: levels)

**Interfaces:**
- Consumes: Task 7 marker functions and `MeterLevelCompanies`; Task 5 `companiesUsed`.
- Produces:
  - `type billingclient.LevelReport struct { CustomerRef, Meter string; Value int64; OccurredAt, IdempotencyKey string }` (json `customer_ref`, `meter`, `value`, `occurred_at`, `idempotency_key`)
  - `func (c *Client) ReportLevel(ctx context.Context, in LevelReport) error` — `POST /v1.0/usage/levels`, `Idempotency-Key` = `in.IdempotencyKey`.
  - `type services.LevelReporter struct`; `func services.NewLevelReporter(repo *repositories.AccountBillingRepository, sink levelSink, billing *BillingService) *LevelReporter` (nil sink → every method is a no-op)
  - `func (r *LevelReporter) Flush(ctx context.Context, organizationID string) error`
  - `func (r *LevelReporter) MarkAndFlush(ctx context.Context, organizationID string) error`
  - `func (r *LevelReporter) Sweep(ctx context.Context) error`
  - `func (r *LevelReporter) Run(ctx context.Context, every time.Duration)`
  - `func services.LevelKey(organizationID string, version int64) string` → `dfe_companies:{org}:v{version}`
  - `func (s *BillingService) WithLevelFlush(flush func(ctx context.Context, organizationID string) error) *BillingService`
  - Behaviour: every plan selection for an organization (`Choose`, `Change`) marks its `dfe_companies` level dirty and flushes it, so the level is known from the moment it subscribes, migrated or not.

- [ ] **Step 1: Write the failing client test** — `api/internal/billingclient/levels_test.go`

```go
package billingclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
)

func TestReportLevelPostsTheWholeCount(t *testing.T) {
	var got map[string]any
	var key string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/usage/levels", func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/v1.0/token", ClientID: "dfe", ClientSecret: "s", Cache: cache.NewMemoryBackend(4)})

	err := c.ReportLevel(context.Background(), LevelReport{
		CustomerRef: "ORG_org_1", Meter: "dfe_companies", Value: 3,
		OccurredAt: "2026-10-10T14:03:00Z", IdempotencyKey: "dfe_companies:org_1:v7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["customer_ref"] != "ORG_org_1" || got["meter"] != "dfe_companies" || got["value"] != float64(3) ||
		got["occurred_at"] != "2026-10-10T14:03:00Z" || got["idempotency_key"] != "dfe_companies:org_1:v7" || key != "dfe_companies:org_1:v7" {
		t.Fatalf("body = %v, header key = %q", got, key)
	}
}
```

- [ ] **Step 2: Write the failing reporter tests** — append to `api/tests/integration/org_billing_test.go`

```go
// levelSinkStub records level reports and can fail.
type levelSinkStub struct {
	reports []billingclient.LevelReport
	fail    error
}

func (l *levelSinkStub) ReportLevel(_ context.Context, in billingclient.LevelReport) error {
	if l.fail != nil {
		return l.fail
	}
	l.reports = append(l.reports, in)
	return nil
}

func TestFlushReportsTheWholeCountOnceAndClears(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{}
	rep := services.NewLevelReporter(repo, sink, svc)
	org := "org-flush-" + newCompanyPK(t)
	a := seedCompany(t, org, "owner-f", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-f", "11222333000262", "B Ltda")
	seedNfeConfig(t, a)
	seedNfeConfig(t, b)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		if err := rep.Flush(ctx, org); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.reports) != 1 {
		t.Fatalf("reports = %+v, want one (the second flush finds nothing dirty)", sink.reports)
	}
	got := sink.reports[0]
	if got.CustomerRef != repositories.OrgBillingPK(org) || got.Meter != services.MeterLevelCompanies || got.Value != 2 ||
		got.IdempotencyKey != services.LevelKey(org, 1) || got.OccurredAt == "" {
		t.Fatalf("report = %+v", got)
	}
}

// Gap 12 of the planning amendment: a failed delivery is not forgotten. The
// marker stays dirty and the sweeper delivers it later, with the same key.
func TestAFailedReportStaysDirtyAndTheSweeperDeliversIt(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{fail: errors.New("billing down")}
	rep := services.NewLevelReporter(repo, sink, svc)
	org := "org-sweep-" + newCompanyPK(t)
	c := seedCompany(t, org, "owner-s", "11222333000181", "S Ltda")
	seedNfeConfig(t, c)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}

	if err := rep.Flush(ctx, org); err == nil {
		t.Fatal("a failed report must surface")
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); !m.Dirty {
		t.Fatal("a failed report cleared the marker")
	}
	sink.fail = nil
	if err := rep.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	var delivered bool
	for _, r := range sink.reports {
		if r.CustomerRef == repositories.OrgBillingPK(org) && r.Value == 1 && r.IdempotencyKey == services.LevelKey(org, 1) {
			delivered = true
		}
	}
	if !delivered {
		t.Fatalf("the sweeper did not deliver: %+v", sink.reports)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m.Dirty {
		t.Fatal("still dirty after delivery")
	}
}

// Billing answers 409 for a key it holds with another body: the version was
// already recorded (by another instance), so the marker is cleared, not retried forever.
func TestAConflictingReportCountsAsDelivered(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	repo := repositories.NewAccountBillingRepository(db, cfg)
	rep := services.NewLevelReporter(repo, &levelSinkStub{fail: problem.Conflict("x")}, svc)
	org := "org-409-" + newCompanyPK(t)
	if err := repo.MarkLevelDirty(ctx, org, services.MeterLevelCompanies); err != nil {
		t.Fatal(err)
	}
	if err := rep.Flush(ctx, org); err != nil {
		t.Fatal(err)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m.Dirty {
		t.Fatal("a 409 must clear the version")
	}
}
```

(Add `gopkg.aoctech.app/dfe/api/internal/billingclient` to the file's imports.)

Also append (it reuses `managementStub` and `fakeRoles` from Task 6):

```go
// Any plan selection makes the organization's level known: subscribing marks
// it dirty and flushes it, migrated or not.
func TestChoosingAPlanReportsTheCompaniesLevel(t *testing.T) {
	ctx := context.Background()
	stub := &managementStub{}
	svc := chargingBilling(t, stub.server(t)).
		WithWorkspaceRoles(fakeRoles{roles: map[string]string{"usr-admin": services.AccountRoleAdmin}}).
		WithEnablement(services.NewFiscalConfigEnablement(nfeConfigRepo, nfceConfigRepo, nil, nil, nil))
	repo := repositories.NewAccountBillingRepository(db, cfg)
	sink := &levelSinkStub{}
	svc.WithLevelFlush(services.NewLevelReporter(repo, sink, svc).Flush)
	org := "org-choose-level-" + newCompanyPK(t)
	company := seedCompany(t, org, "usr-owner", "11222333000181", "Escolhe Ltda")
	seedNfeConfig(t, company)

	scope, err := svc.ScopeFor(ctx, company, "usr-admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Choose(ctx, scope, "", []string{"price_dfe_free"}); err != nil {
		t.Fatal(err)
	}
	if len(sink.reports) != 1 || sink.reports[0].CustomerRef != repositories.OrgBillingPK(org) || sink.reports[0].Value != 1 {
		t.Fatalf("reports = %+v, want the level 1 for the organization", sink.reports)
	}
	if m, _ := repo.GetLevelMarker(ctx, org, services.MeterLevelCompanies); m == nil || m.Dirty {
		t.Fatalf("marker = %+v, want written and delivered", m)
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/billingclient/ -run TestReportLevel` and `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestFlush|TestAFailedReport|TestAConflicting|TestChoosingAPlan' -v`
Expected: FAIL — `undefined: LevelReport`, `undefined: services.NewLevelReporter`.

- [ ] **Step 4: Implement the client** — `api/internal/billingclient/client.go`, after `ReportUsage`:

```go
// LevelReport is a level: the whole current count of something, never a delta
// (billing plans spec § 6). A lost report is repaired by the next one.
type LevelReport struct {
	CustomerRef    string `json:"customer_ref"`
	Meter          string `json:"meter"`
	Value          int64  `json:"value"`
	OccurredAt     string `json:"occurred_at"`
	IdempotencyKey string `json:"idempotency_key"`
}

// ReportLevel records a level for a customer reference. No customer is created
// by it, and it is accepted whatever the plan.
func (c *Client) ReportLevel(ctx context.Context, in LevelReport) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPost, "/v1.0/usage/levels", in.IdempotencyKey, body, nil)
}
```

- [ ] **Step 5: Implement the reporter** — `api/internal/services/levels.go`

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// LevelSweepInterval is how often every API instance delivers pending levels.
// Several instances sweeping at once is harmless: they send identical bodies
// under identical keys.
const LevelSweepInterval = 2 * time.Minute

type levelSink interface {
	ReportLevel(ctx context.Context, in billingclient.LevelReport) error
}

// LevelKey is the idempotency key of one version of an organization's
// companies level: a retry of that version is the same report.
func LevelKey(organizationID string, version int64) string {
	return fmt.Sprintf("%s:%s:v%d", MeterLevelCompanies, organizationID, version)
}

// LevelReporter delivers dirty companies levels to billing, durably: a marker
// leaves the dirty index only when billing accepted its version.
type LevelReporter struct {
	repo    *repositories.AccountBillingRepository
	sink    levelSink
	billing *BillingService
}

// NewLevelReporter builds the reporter. A nil sink (billing off) makes every
// method a no-op.
func NewLevelReporter(repo *repositories.AccountBillingRepository, sink levelSink, billing *BillingService) *LevelReporter {
	return &LevelReporter{repo: repo, sink: sink, billing: billing}
}

func (r *LevelReporter) off() bool { return r == nil || r.sink == nil }

// Flush reports the organization's companies level if its marker is dirty.
func (r *LevelReporter) Flush(ctx context.Context, organizationID string) error {
	if r.off() {
		return nil
	}
	m, err := r.repo.GetLevelMarker(ctx, organizationID, MeterLevelCompanies)
	if err != nil || m == nil || !m.Dirty {
		return err
	}
	count, err := r.billing.companiesUsed(ctx, organizationID)
	if err != nil {
		return err
	}
	err = r.sink.ReportLevel(ctx, billingclient.LevelReport{
		CustomerRef:    repositories.OrgBillingPK(organizationID),
		Meter:          MeterLevelCompanies,
		Value:          count,
		OccurredAt:     m.ChangedAt,
		IdempotencyKey: LevelKey(organizationID, m.Version),
	})
	var p *problem.Problem
	if err != nil && !(errors.As(err, &p) && p.Status == http.StatusConflict) {
		return err
	}
	// 409: billing already holds this key with another body, i.e. this version
	// was recorded; the next change reports the current count anyway.
	return r.repo.ClearLevelDirty(ctx, organizationID, MeterLevelCompanies, m.Version)
}

// MarkAndFlush marks the level dirty and delivers it (the migration command:
// an organization's initial level). A failed delivery stays for the sweeper.
func (r *LevelReporter) MarkAndFlush(ctx context.Context, organizationID string) error {
	if r.off() {
		return nil
	}
	if err := r.repo.MarkLevelDirty(ctx, organizationID, MeterLevelCompanies); err != nil {
		return err
	}
	return r.Flush(ctx, organizationID)
}

// Sweep delivers every pending marker. One failure does not stop the others.
func (r *LevelReporter) Sweep(ctx context.Context) error {
	if r.off() {
		return nil
	}
	pending, err := r.repo.ListDirtyLevels(ctx)
	if err != nil {
		return err
	}
	for _, m := range pending {
		if m.Meter != MeterLevelCompanies {
			continue
		}
		if err := r.Flush(ctx, m.OrganizationID); err != nil {
			slog.WarnContext(ctx, "billing: level still pending; retried next sweep",
				"organization_id", m.OrganizationID, "version", m.Version, "error", err)
		}
	}
	return nil
}

// Run sweeps until ctx ends.
func (r *LevelReporter) Run(ctx context.Context, every time.Duration) {
	if r.off() {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.Sweep(ctx); err != nil {
				slog.WarnContext(ctx, "billing: level sweep failed", "error", err)
			}
		}
	}
}
```

`api/internal/services/billing.go`: add the field `levelFlush func(ctx context.Context, organizationID string) error` and:

```go
// WithLevelFlush wires the level delivery (LevelReporter.Flush). Set after
// both exist, since the reporter counts through this service.
func (s *BillingService) WithLevelFlush(flush func(ctx context.Context, organizationID string) error) *BillingService {
	s.levelFlush = flush
	return s
}

// markCompaniesLevel makes the organization's dfe_companies level known to
// billing at a plan selection, so an organization that subscribes (migrated or
// not) has its level from day one. The marker is durable; a failed flush is
// left for the sweeper and never fails the plan selection.
func (s *BillingService) markCompaniesLevel(ctx context.Context, organizationID string) {
	if err := s.repo.MarkLevelDirty(ctx, organizationID, MeterLevelCompanies); err != nil {
		slog.ErrorContext(ctx, "billing: could not mark the companies level", "organization_id", organizationID, "error", err)
		return
	}
	if s.levelFlush == nil {
		return
	}
	if err := s.levelFlush(ctx, organizationID); err != nil {
		slog.WarnContext(ctx, "billing: companies level left for the sweeper", "organization_id", organizationID, "error", err)
	}
}
```

In `Choose` and `Change`, right after the successful `s.Sync(ctx, scope.OrganizationID)`, call `s.markCompaniesLevel(ctx, scope.OrganizationID)`.

Wiring in `api/internal/app/app.go` (follow `startResultsConsumer`'s shape):

```go
// newLevelReporter delivers companies levels to billing. Billing off → a nil
// sink and a no-op reporter.
func newLevelReporter(repo *repositories.AccountBillingRepository, client *billingclient.Client, billing *services.BillingService) *services.LevelReporter {
	if client == nil {
		return services.NewLevelReporter(repo, nil, billing)
	}
	r := services.NewLevelReporter(repo, client, billing)
	billing.WithLevelFlush(r.Flush)
	return r
}

// startLevelSweeper runs the sweeper for the life of the process.
func startLevelSweeper(lc fx.Lifecycle, r *services.LevelReporter) {
	ctx, cancel := context.WithCancel(context.Background())
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error { go r.Run(ctx, services.LevelSweepInterval); return nil },
		OnStop:  func(context.Context) error { cancel(); return nil },
	})
}
```

Register `newLevelReporter` with the providers and `startLevelSweeper` with the invokes, next to `startResultsConsumer`. (The `client == nil` branch matters: a nil `*billingclient.Client` stored in the interface would not be nil.)

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/billingclient/ ./internal/services/` and `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS.

- [ ] **Step 7: Document** — `DOCS.md`, billing: the companies level (`dfe_companies` = enabled companies of the organization) is reported to `POST /v1.0/usage/levels` for `ORG_{id}` on every enablement change, whatever the plan; delivery is durable (marker + sweeper every 2 minutes); the on-demand price bills the monthly peak.

- [ ] **Step 8: Commit**

```bash
git add api/internal/billingclient api/internal/services/levels.go api/internal/services/billing.go api/internal/app/app.go api/tests/integration/org_billing_test.go DOCS.md
git commit -m "feat(dfe): companies level delivered to billing durably"
```

---

### Task 9: The webhook syncs `ORG_` customers and ignores `USER_`

**Files:**
- Modify: `api/internal/services/billing.go` (`SyncBySubscription`, `organizationFromRef`)
- Modify: `api/internal/services/billing_test.go`
- Modify: `api/tests/integration/org_billing_test.go`

**Interfaces:**
- Consumes: Task 5 `Sync(ctx, organizationID)`.
- Produces: `func services.OrganizationFromRef(externalRef string) (string, bool)`; `SyncBySubscription` unchanged signature.

- [ ] **Step 1: Write the failing tests**

Unit, append to `api/internal/services/billing_test.go`:

```go
func TestOrganizationFromRef(t *testing.T) {
	cases := map[string]struct {
		id string
		ok bool
	}{
		"ORG_0199f3a1": {"0199f3a1", true},
		"USER_abc":     {"", false},
		"ORG_":         {"", false},
		"FIN_x":        {"", false},
		"":             {"", false},
	}
	for ref, want := range cases {
		id, ok := OrganizationFromRef(ref)
		if id != want.id || ok != want.ok {
			t.Errorf("%q → %q %v, want %q %v", ref, id, ok, want.id, want.ok)
		}
	}
}
```

Integration (spec § 5 test 4), append to `org_billing_test.go`:

```go
func TestWebhookSyncsAnOrganizationAndIgnoresAUser(t *testing.T) {
	ctx := context.Background()
	org := "org-hook-" + newCompanyPK(t)
	ref := repositories.OrgBillingPK(org)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/subscriptions/sub_org", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sub_org","customer_id":"cus_org"}`))
	})
	mux.HandleFunc("/v1.0/subscriptions/sub_user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sub_user","customer_id":"cus_user"}`))
	})
	mux.HandleFunc("/v1.0/customers/cus_org", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cus_org","external_ref":"` + ref + `"}`))
	})
	mux.HandleFunc("/v1.0/customers/cus_user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cus_user","external_ref":"USER_hook-owner"}`))
	})
	mux.HandleFunc("/v1.0/entitlements", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("customer_ref") != ref {
			t.Errorf("entitlements asked for %q", r.URL.Query().Get("customer_ref"))
		}
		_, _ = w.Write([]byte(`{"customer_id":"cus_org","subscriptions":[{"id":"sub_org","status":"PAST_DUE","entitled":true,"plan":"pro","current_period":{"start":"2026-10-01","end":"2026-11-01"}}]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	svc := chargingBilling(t, srv)

	if err := svc.SyncBySubscription(ctx, "sub_org"); err != nil {
		t.Fatal(err)
	}
	got, err := repositories.NewAccountBillingRepository(db, cfg).GetOrg(ctx, org)
	if err != nil || got == nil || got.Status != "PAST_DUE" {
		t.Fatalf("ORG_ snapshot = %+v (%v)", got, err)
	}
	if err := svc.SyncBySubscription(ctx, "sub_user"); err != nil {
		t.Fatalf("a USER_ customer is ignored, not an error: %v", err)
	}
	if u, _ := repositories.NewAccountBillingRepository(db, cfg).Get(ctx, "hook-owner"); u != nil {
		t.Fatalf("a USER_ webhook wrote a snapshot: %+v", u)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/services/ -run TestOrganizationFromRef`
Expected: FAIL — `undefined: OrganizationFromRef`.

- [ ] **Step 3: Implement** — `api/internal/services/billing.go`

```go
// OrganizationFromRef reads a DF-e organization out of a billing customer's
// external_ref. Anything else (USER_, another product's prefix) is not ours to
// sync.
func OrganizationFromRef(externalRef string) (string, bool) {
	id, ok := strings.CutPrefix(externalRef, repositories.OrgBillingPrefix)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}
```

In `SyncBySubscription`, replace everything from `userID := repositories.RawUserID(customer.ExternalRef)` to the end with:

```go
	organizationID, ok := OrganizationFromRef(customer.ExternalRef)
	if !ok {
		// A USER_ customer is a pre-migration subscription: after
		// cmd/migrate-billing-org the only USER_ events expected are the
		// cancellations of the old subscriptions, and nothing reads USER_ rows
		// after the dual-read window. Anything else belongs to another product.
		slog.InfoContext(ctx, "billing: webhook for a customer that is not a DF-e organization; ignored",
			"customer_id", customer.ID, "external_ref", customer.ExternalRef)
		return nil
	}
	_, err = s.Sync(ctx, organizationID)
	return err
```

Update the function comment's first line to "resolves a subscription id back to the DF-e organization and re-syncs it".

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/services/ ./internal/api/v1/` and `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run TestWebhook -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/billing.go api/internal/services/billing_test.go api/tests/integration/org_billing_test.go
git commit -m "feat(dfe): billing webhook syncs organization customers"
```

---

### Task 10: Worker messages carry the organization of a reservation

**Files:**
- Modify: `api/internal/services/worker.go:31` (`BillingOrganizationID`)
- Modify: `api/internal/services/nfes/emit.go:749`, `nfes/nfce_emit.go:288`, `mdfes/emit.go:577`, `nfses/emit.go:301`
- Modify: `api/internal/consumer/results.go:28-45,268-305`
- Modify: `worker/internal/service/dfe.go:97,677`, `worker/internal/service/helpers.go:46`, `worker/cmd/dlq-processor/main.go:137`
- Modify: `api/internal/consumer/results_billing_test.go`
- Modify: `api/internal/services/billing.go` (comment at `MeterCTe`)

**Interfaces:**
- Consumes: Task 5 `UsageReservation.OrganizationID`, `RefundReservedUsage(ctx, accountID, period, meter, docKey)`.
- Produces: wire key `billing_organization_id` (api `WorkerMessage.BillingOrganizationID`, worker `DfeMessage.BillingOrganizationID`, notify key `notifyKeyBillingOrganizationID`, consumer `resultKeyBillingOrganizationID`); unexported `func reservationAccount(event map[string]any) string` in `consumer`.

- [ ] **Step 1: Write the failing test** — append to `api/internal/consumer/results_billing_test.go`

```go
// Review Focus 5: a message reserved before the deploy names the user whose
// counter it took; one reserved after names the organization. The refund goes
// back to whichever it names.
func TestRefundUsesTheAccountTheReservationNamed(t *testing.T) {
	cases := []struct {
		event map[string]any
		want  string
	}{
		{map[string]any{resultKeyBillingOrganizationID: "org_1", resultKeyBillingUserID: "org_1"}, "org_1"},
		{map[string]any{resultKeyBillingUserID: "user_legacy"}, "user_legacy"},
		{map[string]any{}, ""},
	}
	for _, tc := range cases {
		if got := reservationAccount(tc.event); got != tc.want {
			t.Errorf("reservationAccount(%v) = %q, want %q", tc.event, got, tc.want)
		}
	}
}

// CT-e has a meter and a table but no emission path yet (spec § 3). Whoever
// builds it must reserve with MeterCTe like the other four.
func TestCTeTableIsMeteredAsCTe(t *testing.T) {
	if services.MeterForTable["ctes"] != services.MeterCTe {
		t.Fatalf("ctes → %q", services.MeterForTable["ctes"])
	}
}
```

(Add the `services` import if the file lacks it.)

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/consumer/ -run 'TestRefundUsesTheAccount|TestCTeTable'`
Expected: FAIL — `undefined: resultKeyBillingOrganizationID`, `undefined: reservationAccount`.

- [ ] **Step 3: Implement**

`api/internal/services/worker.go`, after `BillingUserID`:

```go
	// BillingOrganizationID is the organization whose counter the reservation
	// took (docs/specs/2026-10-10-organization-subscription.md). BillingUserID
	// is kept only so a message reserved before this deploy still refunds the
	// user counter it took; nothing sets it any more.
	BillingOrganizationID string `json:"billing_organization_id,omitempty"`
```

The four emitters: `BillingUserID: reservation.OrganizationID` → `BillingOrganizationID: reservation.OrganizationID`.

`api/internal/consumer/results.go`: add `resultKeyBillingOrganizationID = "billing_organization_id"` to the constants block, and:

```go
// reservationAccount is the counter a reservation took: the organization for
// messages reserved since the organization re-key, the user for those reserved
// before it and still in flight.
func reservationAccount(event map[string]any) string {
	if org, _ := event[resultKeyBillingOrganizationID].(string); org != "" {
		return org
	}
	user, _ := event[resultKeyBillingUserID].(string)
	return user
}
```

In `settleBilling`, replace `userID, _ := event[resultKeyBillingUserID].(string)` with `account := reservationAccount(event)` and the refund branch's `userID` uses with `account`.

Worker (separate module): `worker/internal/service/dfe.go` add `BillingOrganizationID string \`json:"billing_organization_id,omitempty"\`` after `BillingUserID`, and `notifyKeyBillingOrganizationID: msg.BillingOrganizationID,` in the notify map; `worker/internal/service/helpers.go` add `notifyKeyBillingOrganizationID = "billing_organization_id"`; `worker/cmd/dlq-processor/main.go` add `"billing_organization_id": msg.BillingOrganizationID,`.

`api/internal/services/billing.go`, above `MeterCTe` in the meter constants, add:

```go
	// MeterCTe has a table and a quota but no emission path yet. When CT-e
	// emission is built it must call PrepareUsageReservation(…, MeterCTe, …)
	// like the other four; a Free plan (quota_cte: 0) then refuses it with 402.
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/consumer/ ./internal/services/...` and `cd ../worker && go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/worker.go api/internal/services/nfes api/internal/services/mdfes api/internal/services/nfses api/internal/consumer api/internal/services/billing.go worker/internal/service/dfe.go worker/internal/service/helpers.go worker/cmd/dlq-processor/main.go
git commit -m "feat(dfe): worker messages carry the organization a quota reservation took"
```

---

### Task 11: First fiscal configuration reserves the company

**Files:**
- Modify: `api/internal/services/fiscal_configs.go:28-32,64-103` (`Upsert` takes extra transaction items)
- Modify: `api/internal/api/v1/helpers.go:369-430` (`fiscalConfigSvc`, `fiscalConfigDeps`, `reserveEnablement`, `writeFiscalConfig`)
- Modify: `api/internal/api/v1/organizations.go:243-265` (deps carry billing, the level reporter and the organization service for every variant)
- Modify: `api/internal/api/v1/router.go` (`Services.Levels`, passed into `OrgHandlers`), `api/internal/app/app.go` (fill `Levels`)
- Create: `api/internal/api/v1/fiscal_config_enablement_test.go`
- Modify: `api/tests/integration/org_billing_test.go` (race test)

**Interfaces:**
- Consumes: Task 7 `ReserveCompany`, `CompanyReservation`; Task 8 `LevelReporter.Flush`; Task 5 `OrganizationOf`.
- Produces:
  - `func (s *fiscalConfigService) Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string, extra ...types.TransactWriteItem) (map[string]types.AttributeValue, error)`
  - `type v1.companyReserver interface { OrganizationOf(ctx, companyPK string) (string, error); ReserveCompany(ctx, organizationID, companyPK string) (*services.CompanyReservation, error) }`
  - `type v1.levelFlusher interface { Flush(ctx context.Context, organizationID string) error }`
  - `fiscalConfigDeps.billing companyReserver`, `fiscalConfigDeps.levels levelFlusher`; `OrgHandlers.Levels *services.LevelReporter`; `v1.Services.Levels *services.LevelReporter`
  - `func reserveEnablement(ctx context.Context, billing companyReserver, orgPK string) (*services.CompanyReservation, error)` — runs **before** the série claim, so a refused company claims nothing.
  - `func writeFiscalConfig(ctx context.Context, svc fiscalConfigSvc, levels levelFlusher, reservation *services.CompanyReservation, orgPK string, av map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error)`
  - Handler order: `reserveEnablement` → `claimSeries` → `writeFiscalConfig` → `released()`.

- [ ] **Step 1: Write the failing handler-logic test** — `api/internal/api/v1/fiscal_config_enablement_test.go`

```go
package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type recordingConfig struct {
	upserts int
	extra   []types.TransactWriteItem
	err     error
}

func (r *recordingConfig) Get(context.Context, string) (map[string]types.AttributeValue, error) {
	return nil, nil
}

func (r *recordingConfig) Upsert(_ context.Context, _ string, _ map[string]types.AttributeValue, _, _ string, extra ...types.TransactWriteItem) (map[string]types.AttributeValue, error) {
	r.upserts++
	r.extra = extra
	return map[string]types.AttributeValue{}, r.err
}

type fakeReserver struct {
	reservation *services.CompanyReservation
	err         error
}

type fakeFlusher struct{ flushed int }

func (f *fakeFlusher) Flush(context.Context, string) error {
	f.flushed++
	return nil
}

func (f *fakeReserver) OrganizationOf(context.Context, string) (string, error) { return "org_1", nil }
func (f *fakeReserver) ReserveCompany(context.Context, string, string) (*services.CompanyReservation, error) {
	return f.reservation, f.err
}

// Spec § 5 test 7: above the limit → 402 and no configuration written.
func TestARefusedEnablementWritesNoConfiguration(t *testing.T) {
	res := &fakeReserver{err: problem.QuotaExceeded(services.MeterCompanies, "free", 1, 1, "limite")}
	_, err := reserveEnablement(context.Background(), res, "cmp_1")
	var p *problem.Problem
	if !errors.As(err, &p) || p.Status != 402 {
		t.Fatalf("err = %v, want 402", err)
	}
	// The handler returns here: neither the série claim nor the write runs.
}

func TestAnEnablementCommitsTheGuardWithTheConfigurationAndReports(t *testing.T) {
	guard := types.TransactWriteItem{}
	cfg := &recordingConfig{}
	marker := types.TransactWriteItem{}
	res := &fakeReserver{reservation: &services.CompanyReservation{Items: []types.TransactWriteItem{guard, marker}, OrganizationID: "org_1", CompanyPK: "cmp_1"}}
	levels := &fakeFlusher{}
	r, err := reserveEnablement(context.Background(), res, "cmp_1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, r, "cmp_1", nil, "usr_1", "Fulano"); err != nil {
		t.Fatal(err)
	}
	if cfg.upserts != 1 || len(cfg.extra) != 2 || levels.flushed != 1 {
		t.Fatalf("upserts=%d extra=%d flushed=%d", cfg.upserts, len(cfg.extra), levels.flushed)
	}
}

func TestAnEnabledCompanySavesWithoutGuardOrReport(t *testing.T) {
	cfg := &recordingConfig{}
	res := &fakeReserver{reservation: &services.CompanyReservation{}}
	levels := &fakeFlusher{}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, res.reservation, "cmp_1", nil, "usr_1", "Fulano"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.extra) != 0 || levels.flushed != 0 {
		t.Fatalf("extra=%d flushed=%d", len(cfg.extra), levels.flushed)
	}
}

// A failed write delivers nothing (and wrote no marker).
func TestAFailedWriteReportsNothing(t *testing.T) {
	guard := types.TransactWriteItem{}
	cfg := &recordingConfig{err: errors.New("boom")}
	res := &fakeReserver{reservation: &services.CompanyReservation{Items: []types.TransactWriteItem{guard}, OrganizationID: "org_1"}}
	levels := &fakeFlusher{}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, res.reservation, "cmp_1", nil, "usr_1", "Fulano"); err == nil {
		t.Fatal("want the write error")
	}
	if levels.flushed != 0 {
		t.Fatal("flushed a level whose change was not written")
	}
}
```

- [ ] **Step 2: Write the failing race test** — append to `api/tests/integration/org_billing_test.go` (Review Focus 3)

```go
// Two companies of one organization enabled together at limit-1: exactly one
// configuration commits.
func TestTwoEnablementsRaceForTheLastSlot(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := companyQuotaBilling(t, &calls)
	org := "org-race-" + newCompanyPK(t)
	snap := proSnapshot("sub_race")
	snap.Quotas[services.MeterCompanies] = 1
	seedOrgSnapshot(t, org, snap)
	a := seedCompany(t, org, "owner-race", "11222333000181", "A Ltda")
	b := seedCompany(t, org, "owner-race", "11222333000262", "B Ltda")

	ra, err := svc.ReserveCompany(ctx, org, a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := svc.ReserveCompany(ctx, org, b)
	if err != nil {
		t.Fatal(err) // both read 0 of 1: the guard decides
	}
	nfe := services.NewNfeConfigService(nfeConfigRepo, auditRepo)
	fields := map[string]types.AttributeValue{"prod_current_serie": &types.AttributeValueMemberN{Value: "1"}}
	_, errA := nfe.Upsert(ctx, a, fields, "owner-race", "Dono", ra.Items...)
	_, errB := nfe.Upsert(ctx, b, fields, "owner-race", "Dono", rb.Items...)
	if (errA == nil) == (errB == nil) {
		t.Fatalf("exactly one must commit: errA=%v errB=%v", errA, errB)
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test ./internal/api/v1/ -run 'Enablement|Configuration|FailedWrite'`
Expected: FAIL — `undefined: reserveEnablement`, `undefined: writeFiscalConfig`; `recordingConfig` does not implement `fiscalConfigSvc` (Upsert arity).

- [ ] **Step 4: Implement**

`api/internal/services/fiscal_configs.go`: add `extra ...types.TransactWriteItem` to `Upsert`, and write `items := append([]types.TransactWriteItem{configTx, auditTx}, extra...)` then `s.repo.TransactWrite(ctx, items)`. Update its comment: "extra items (the company quota guard on a first configuration) commit in the same transaction, so a refused guard leaves no configuration behind."

`api/internal/api/v1/helpers.go`:

```go
type fiscalConfigSvc interface {
	Get(ctx context.Context, orgPK string) (map[string]types.AttributeValue, error)
	Upsert(ctx context.Context, orgPK string, fields map[string]types.AttributeValue, userID, userName string, extra ...types.TransactWriteItem) (map[string]types.AttributeValue, error)
}

// companyReserver is the company quota as the configuration save needs it
// (BillingService). An interface so the ordering below is testable.
type companyReserver interface {
	OrganizationOf(ctx context.Context, companyPK string) (string, error)
	ReserveCompany(ctx context.Context, organizationID, companyPK string) (*services.CompanyReservation, error)
}

// levelFlusher delivers the organization's companies level after the commit
// (services.LevelReporter). The marker is already durable in the same
// transaction; this only saves the sweeper's wait.
type levelFlusher interface {
	Flush(ctx context.Context, organizationID string) error
}

// reserveEnablement checks the company quota when this save may be the
// company's first configuration (spec O5). It runs before the série claim, so
// a refused company claims nothing and writes nothing.
func reserveEnablement(ctx context.Context, billing companyReserver, orgPK string) (*services.CompanyReservation, error) {
	if billing == nil {
		return nil, nil
	}
	organizationID, err := billing.OrganizationOf(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	return billing.ReserveCompany(ctx, organizationID, orgPK)
}

// writeFiscalConfig writes the configuration with the reservation's items (the
// quota guard and the companies-level marker) in the same transaction, then
// asks for the level to be delivered now rather than at the next sweep.
func writeFiscalConfig(ctx context.Context, svc fiscalConfigSvc, levels levelFlusher, reservation *services.CompanyReservation, orgPK string, av map[string]types.AttributeValue, userID, userName string) (map[string]types.AttributeValue, error) {
	var extra []types.TransactWriteItem
	if reservation != nil {
		extra = reservation.Items
	}
	item, err := svc.Upsert(ctx, orgPK, av, userID, userName, extra...)
	if err != nil {
		if len(extra) > 0 && repositories.IsConditionFailed(err) {
			return nil, problem.Conflict("outra empresa da organização foi habilitada ao mesmo tempo; tente de novo")
		}
		return nil, err
	}
	if len(extra) > 0 && levels != nil {
		// Best effort: the dirty marker committed with the configuration, so a
		// failure here is delivered by the sweeper, never lost.
		if err := levels.Flush(ctx, reservation.OrganizationID); err != nil {
			slog.WarnContext(ctx, "billing: companies level left for the sweeper",
				"organization_id", reservation.OrganizationID, "error", err)
		}
	}
	return item, nil
}
```

(If `repositories.IsConditionFailed` is not re-exported, use `dynamo.IsConditionFailed` the way `account_billing.go` does; `grep -n "func IsConditionFailed\|IsConditionFailed =" api/internal/repositories/*.go`.) Add `billing companyReserver` to `fiscalConfigDeps`. In the PUT handler of `registerFiscalConfig`:

```go
		orgPK := middleware.GetOrgPK(c)

		// The company quota first (spec O5): a company refused here has
		// claimed no série and written nothing.
		reservation, err := reserveEnablement(c.Context(), deps.billing, orgPK)
		if err != nil {
			return sendProblem(c, err)
		}

		released, err := claimSeries(c, orgPK, av, svc, deps)
		if err != nil {
			return sendProblem(c, err)
		}

		userID, userName := resolveActor(c, userSvc)
		item, err := writeFiscalConfig(c.Context(), svc, deps.levels, reservation, orgPK, av, userID, userName)
		if err != nil {
			return sendProblem(c, err)
		}
		released()
		return sendItem(c, redactFiscalSecrets(item))
```

`api/internal/api/v1/organizations.go`: add `Levels *services.LevelReporter` to `OrgHandlers` (set from a new `Levels` field on `v1.Services`, filled in `app.go` from `newLevelReporter`); `serieDeps` sets `billing: h.BillingSvc, levels: h.Levels`; the NFS-e registration passes `fiscalConfigDeps{orgSvc: h.OrgSvc, billing: h.BillingSvc, levels: h.Levels}` (no claims, no modelo, so no série claim runs).

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go build ./... && go test ./internal/api/v1/ ./internal/services/` then `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS (including the existing `fiscal_configs_audit_test.go` and `nfse_configs_test.go`).

- [ ] **Step 6: Document** — `DOCS.md`, fiscal configuration section: the first configuration of a company checks the organization's company quota (402 `quota_exceeded`, nothing written) and marks the organization's `dfe_companies` level for delivery to billing (Task 8); 409 when the company has no ctech-account organization or when another company took the last slot concurrently.

- [ ] **Step 7: Commit**

```bash
git add api/internal/services/fiscal_configs.go api/internal/api/v1/helpers.go api/internal/api/v1/organizations.go api/internal/api/v1/router.go api/internal/app/app.go api/internal/api/v1/fiscal_config_enablement_test.go api/tests/integration/org_billing_test.go DOCS.md
git commit -m "feat(dfe): first fiscal configuration of a company checks the organization company quota"
```

---
### Task 12: `cmd/migrate-billing-org`

**Files:**
- Create: `api/cmd/migrate-billing-org/main.go` (flags, wiring)
- Create: `api/cmd/migrate-billing-org/migrate.go` (the logic, behind interfaces)
- Create: `api/cmd/migrate-billing-org/migrate_test.go`
- Modify: `api/internal/repositories/organizations.go` (`ListOrganizationsWithCompanies`)

**Interfaces:**
- Consumes: Task 8 `LevelReporter.MarkAndFlush`; Task 1 `ListCompanyIndexGaps`, `BackfillIndexKeys`, `ListCompaniesOfOrganization`; Task 2 `ListUserSnapshots`, `GetOrg`, `CopyUsageOnce`, `UsageSource`; Task 3 `WorkspaceClient.Organizations`, `Workspace.IsOrganization`; Task 6 `GetOrCreateCustomer`, `CustomerPayer`; Task 5 `Sync`; `accountclient.Client.Reach`; `billingclient.Client` methods `GetEntitlements`, `GetCustomer`, `CreateSubscription`, `CancelSubscription`.
- Produces: `func (r *OrganizationRepository) ListOrganizationsWithCompanies(ctx context.Context) ([]string, error)` (distinct `organization_id` of company-keyed records, sorted); flag `-report-levels-all`; the command. `run(ctx, deps, apply bool) (*Report, error)`; `Report{Backfilled, Unresolved, Migrated, Review []string}`, `(*Report).NeedsReview() bool`, `(*Report).Print(io.Writer)`. Exit codes: 0 done, 1 error, 2 bad flags, 3 done with items needing review.

Behaviour (spec § 4, in order):
0. Index check: every company-keyed record the `organization-index` cannot see is listed; with `-apply`, `organization_id` is backfilled from ctech-account reach (company + its `owner_user_id`) and a missing `created_at` is set to now. Records reach cannot place are listed as unresolved.
1. List `USER_` snapshots with a subscription; re-read billing's entitlements for each (`customer_ref=USER_{sub}`); skip those with no entitled subscription.
2. Ask ctech-account for the user's organizations; keep `kind` organization, `role == owner`, with at least one DF-e company.
3. Any item with `unit_amount != 0` → listed for review, nothing done for that user.
4. Per organization, unless its `ORG_` snapshot already grants service: `GetOrCreateCustomer(ORG_…)` (payer: the old customer's name and e-mail), `CreateSubscription` with the **same price ids** and key `migrate:{sub}:{org}`, `Sync`.
5. `CopyUsageOnce` from `USAGE_{sub}#{p}` and `USAGE_{org}#{p}` (each `p` among the old subscription's period start and the stale `USER_` row's period start) into `USAGE_{org}#{new period}`, marker `migrate-usage:{sub}:{org}`.
5b. `MarkAndFlush` the organization's `dfe_companies` level (its initial level; a failed delivery stays dirty for the API's sweeper).
6. Only when every organization of the user now grants service: cancel the `USER_` subscription immediately, key `migrate-cancel:{subscription}`.

7. With `-report-levels-all`: mark and flush the `dfe_companies` level of **every** organization holding DF-e companies, migrated or not; a dry run only lists them (`Report.Levels`).

- [ ] **Step 1: Write the failing tests** — `api/cmd/migrate-billing-org/migrate_test.go`

```go
package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type fakeSnaps struct {
	users  []repositories.AccountSnapshot
	orgs   map[string]*repositories.AccountSnapshot
	copies []string // markers that committed
	tried  int
}

func (f *fakeSnaps) ListUserSnapshots(context.Context) ([]repositories.AccountSnapshot, error) {
	return f.users, nil
}
func (f *fakeSnaps) GetOrg(_ context.Context, id string) (*repositories.AccountSnapshot, error) {
	return f.orgs[id], nil
}
func (f *fakeSnaps) CopyUsageOnce(_ context.Context, _ []repositories.UsageSource, _, _, marker string) (bool, error) {
	f.tried++
	for _, m := range f.copies {
		if m == marker {
			return false, nil
		}
	}
	f.copies = append(f.copies, marker)
	return true, nil
}

type fakeBilling struct {
	ent          map[string]*billingclient.Entitlements // by external ref
	created      []string                              // "customer:prices"
	cancelled    []string
	cancelErr    error
}

func (f *fakeBilling) GetEntitlements(_ context.Context, ref string) (*billingclient.Entitlements, error) {
	if e, ok := f.ent[ref]; ok {
		return e, nil
	}
	return nil, billingclient.ErrCustomerNotFound
}
func (f *fakeBilling) GetCustomer(context.Context, string) (*billingclient.Customer, error) {
	return &billingclient.Customer{ID: "cus_user", Name: "Dono", Email: "dono@example.com"}, nil
}
func (f *fakeBilling) CreateSubscription(_ context.Context, customerID string, items []billingclient.Item, _ string) (*billingclient.SubscriptionResult, error) {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.PriceID)
	}
	f.created = append(f.created, customerID+":"+strings.Join(ids, ","))
	return &billingclient.SubscriptionResult{}, nil
}
func (f *fakeBilling) CancelSubscription(_ context.Context, id string, _ bool, _ string) (*billingclient.Subscription, error) {
	if f.cancelErr != nil {
		return nil, f.cancelErr
	}
	f.cancelled = append(f.cancelled, id)
	// Billing now reports the user customer with nothing entitled.
	for ref, e := range f.ent {
		if strings.HasPrefix(ref, "USER_") {
			for i := range e.Subscriptions {
				if e.Subscriptions[i].ID == id {
					e.Subscriptions[i].Entitled = false
				}
			}
		}
	}
	return &billingclient.Subscription{ID: id}, nil
}

type fakeOrgBilling struct{ snaps *fakeSnaps }

func (f fakeOrgBilling) GetOrCreateCustomer(_ context.Context, org string, _ services.CustomerPayer) (string, error) {
	return "cus_" + org, nil
}
func (f fakeOrgBilling) Sync(_ context.Context, org string) (*repositories.AccountSnapshot, error) {
	s := &repositories.AccountSnapshot{OrganizationID: org, SubscriptionID: "sub_" + org, Status: services.StatusActive, Entitled: true, PeriodStart: "2026-10-15"}
	f.snaps.orgs[org] = s
	return s, nil
}

type fakeWorkspaces struct{ orgs []accountclient.Workspace }

func (f fakeWorkspaces) Organizations(context.Context, string) ([]accountclient.Workspace, error) {
	return f.orgs, nil
}

type fakeCompanies struct {
	all        []string
	byOrg      map[string][]repositories.CompanyRef
	gaps       []repositories.CompanyIndexGap
	backfilled []string
}

func (f *fakeCompanies) ListCompaniesOfOrganization(_ context.Context, org string) ([]repositories.CompanyRef, error) {
	return f.byOrg[org], nil
}
func (f *fakeCompanies) ListOrganizationsWithCompanies(context.Context) ([]string, error) {
	return f.all, nil
}
func (f *fakeCompanies) ListCompanyIndexGaps(context.Context) ([]repositories.CompanyIndexGap, error) {
	return f.gaps, nil
}
func (f *fakeCompanies) BackfillIndexKeys(_ context.Context, pk, org, _ string) error {
	f.backfilled = append(f.backfilled, pk+"="+org)
	return nil
}

type fakeLevels struct{ orgs []string }

func (f *fakeLevels) MarkAndFlush(_ context.Context, org string) error {
	f.orgs = append(f.orgs, org)
	return nil
}

type fakeReach map[string]string // company → organization

func (f fakeReach) Reach(_ context.Context, company, _ string) (string, bool, error) {
	org, ok := f[company]
	return org, ok, nil
}

func entitled(subID string, unitAmount int64) *billingclient.Entitlements {
	return &billingclient.Entitlements{CustomerID: "cus_user", Subscriptions: []billingclient.EntitlementSubscription{{
		ID: subID, Status: services.StatusActive, Entitled: true,
		Period: billingclient.Period{Start: "2026-10-01"},
		Items:  []billingclient.EntitlementItem{{PriceID: "price_dfe_unlimited_internal_monthly", UnitAmount: unitAmount}},
	}}}
}

func fixture(unitAmount int64) (deps, *fakeSnaps, *fakeBilling, *fakeCompanies) {
	d, snaps, bill, comps, _ := fixtureWithLevels(unitAmount)
	return d, snaps, bill, comps
}

func fixtureWithLevels(unitAmount int64) (deps, *fakeSnaps, *fakeBilling, *fakeCompanies, *fakeLevels) {
	snaps := &fakeSnaps{
		users: []repositories.AccountSnapshot{{UserID: "u1", SubscriptionID: "sub_u1", Entitled: true, PeriodStart: "2026-09-01"}},
		orgs:  map[string]*repositories.AccountSnapshot{},
	}
	bill := &fakeBilling{ent: map[string]*billingclient.Entitlements{"USER_u1": entitled("sub_u1", unitAmount)}}
	levels := &fakeLevels{}
	comps := &fakeCompanies{byOrg: map[string][]repositories.CompanyRef{"org_a": {{PK: "cmp_a"}}, "org_b": {{PK: "cmp_b"}}}}
	d := deps{
		snaps:   snaps,
		billing: bill,
		orgs:    fakeOrgBilling{snaps: snaps},
		workspaces: fakeWorkspaces{orgs: []accountclient.Workspace{
			{ID: "org_a", Role: "owner", Kind: "organization"},
			{ID: "org_b", Role: "owner"},
			{ID: "org_admin", Role: "admin", Kind: "organization"},  // not owned
			{ID: "sp_1", Role: "owner", Kind: "personal"},             // a space
			{ID: "org_empty", Role: "owner", Kind: "organization"},    // no DF-e companies
		}},
		companies: comps,
		levels:    levels,
		reach:     fakeReach{},
		now:       func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
	}
	return d, snaps, bill, comps, levels
}

// Spec § 5 test 5: --dry-run writes nothing.
func TestDryRunWritesNothing(t *testing.T) {
	d, snaps, bill, _ := fixture(0)
	rep, err := run(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 0 || len(bill.cancelled) != 0 || snaps.tried != 0 || len(snaps.orgs) != 0 {
		t.Fatalf("dry run wrote: created=%v cancelled=%v copies=%d", bill.created, bill.cancelled, snaps.tried)
	}
	if len(rep.Migrated) != 2 {
		t.Fatalf("plan = %v, want org_a and org_b", rep.Migrated)
	}
}

func TestApplyMigratesEachOwnedOrganizationAndCancelsTheUserSubscription(t *testing.T) {
	d, snaps, bill, _, levels := fixtureWithLevels(0)
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	want := []string{"cus_org_a:price_dfe_unlimited_internal_monthly", "cus_org_b:price_dfe_unlimited_internal_monthly"}
	if strings.Join(bill.created, "|") != strings.Join(want, "|") {
		t.Fatalf("created = %v, want %v (same price id)", bill.created, want)
	}
	if len(snaps.copies) != 2 {
		t.Fatalf("counter copies = %v", snaps.copies)
	}
	if len(bill.cancelled) != 1 || bill.cancelled[0] != "sub_u1" {
		t.Fatalf("cancelled = %v", bill.cancelled)
	}
	// The initial dfe_companies level of each migrated organization.
	if strings.Join(levels.orgs, ",") != "org_a,org_b" {
		t.Fatalf("levels = %v", levels.orgs)
	}
}

func TestReportLevelsAllMarksEveryOrganizationWithCompanies(t *testing.T) {
	d, _, _, comps, levels := fixtureWithLevels(0)
	comps.all = []string{"org_a", "org_b", "org_unmigrated"}
	d.reportLevelsAll = true

	rep, err := run(context.Background(), d, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels.orgs) != 0 || strings.Join(rep.Levels, ",") != "org_a,org_b,org_unmigrated" {
		t.Fatalf("dry run: levels=%v listed=%v", levels.orgs, rep.Levels)
	}

	levels.orgs = nil
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	// Migrated organizations are marked by the migration and again by the
	// flag: harmless, a level is the whole count.
	if !strings.Contains(strings.Join(levels.orgs, ","), "org_unmigrated") {
		t.Fatalf("levels = %v, want every organization including the unmigrated one", levels.orgs)
	}
}

func TestDryRunReportsNoLevel(t *testing.T) {
	d, _, _, _, levels := fixtureWithLevels(0)
	if _, err := run(context.Background(), d, false); err != nil {
		t.Fatal(err)
	}
	if len(levels.orgs) != 0 {
		t.Fatalf("dry run reported levels: %v", levels.orgs)
	}
}

// Review Focus 4 / spec § 5 test 5: --apply twice creates nothing the second
// time, including after a run that failed to cancel.
func TestApplyTwiceCreatesNothingTheSecondTime(t *testing.T) {
	d, snaps, bill, _ := fixture(0)
	bill.cancelErr = errors.New("billing down")
	if _, err := run(context.Background(), d, true); err == nil {
		t.Fatal("the failed cancel must surface")
	}
	bill.cancelErr = nil
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	if _, err := run(context.Background(), d, true); err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 2 {
		t.Fatalf("created = %v, want one subscription per organization in total", bill.created)
	}
	if len(snaps.copies) != 2 {
		t.Fatalf("counters copied %d times, want 2 (once per organization)", len(snaps.copies))
	}
	if len(bill.cancelled) != 1 {
		t.Fatalf("cancelled = %v", bill.cancelled)
	}
}

// Spec § 5 test 5: a paid subscription is listed and skipped.
func TestAPaidSubscriptionIsListedAndSkipped(t *testing.T) {
	d, _, bill, _ := fixture(35000)
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.created) != 0 || len(bill.cancelled) != 0 {
		t.Fatalf("a paid subscription was touched: %v %v", bill.created, bill.cancelled)
	}
	if !rep.NeedsReview() || !strings.Contains(strings.Join(rep.Review, "\n"), "USER_u1") {
		t.Fatalf("review = %v", rep.Review)
	}
}

func TestIndexGapsAreBackfilledThroughReach(t *testing.T) {
	d, _, _, comps := fixture(0)
	comps.gaps = []repositories.CompanyIndexGap{
		{PK: "cmp_gap", OwnerUserID: "u1", MissingOrganization: true},
		{PK: "cmp_orphan", OwnerUserID: "u9", MissingOrganization: true},
		{PK: "cmp_nodate", MissingCreatedAt: true},
	}
	d.reach = fakeReach{"cmp_gap": "org_a"}
	rep, err := run(context.Background(), d, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(comps.backfilled, ",") != "cmp_gap=org_a,cmp_nodate=" {
		t.Fatalf("backfilled = %v", comps.backfilled)
	}
	if len(rep.Unresolved) != 1 || !strings.HasPrefix(rep.Unresolved[0], "cmp_orphan") {
		t.Fatalf("unresolved = %v", rep.Unresolved)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./cmd/migrate-billing-org/`
Expected: FAIL — `undefined: deps`, `undefined: run`.

- [ ] **Step 3: Implement the logic** — `api/cmd/migrate-billing-org/migrate.go`

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Idempotency keys and markers. Derived from the user and organization so a
// re-run repeats the same intent, which billing and the marker row recognise.
func migrateSubscriptionKey(sub, org string) string { return "migrate:" + sub + ":" + org }
func migrateUsageMarker(sub, org string) string     { return "migrate-usage:" + sub + ":" + org }
func migrateCancelKey(subscriptionID string) string { return "migrate-cancel:" + subscriptionID }

// calendarPeriodLayout matches services.usagePeriod's fallback.
const calendarPeriodLayout = "2006-01"

type snapshotStore interface {
	ListUserSnapshots(ctx context.Context) ([]repositories.AccountSnapshot, error)
	GetOrg(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)
	CopyUsageOnce(ctx context.Context, sources []repositories.UsageSource, toAccount, toPeriod, marker string) (bool, error)
}

type billingAPI interface {
	GetEntitlements(ctx context.Context, externalRef string) (*billingclient.Entitlements, error)
	GetCustomer(ctx context.Context, customerID string) (*billingclient.Customer, error)
	CreateSubscription(ctx context.Context, customerID string, items []billingclient.Item, key string) (*billingclient.SubscriptionResult, error)
	CancelSubscription(ctx context.Context, subscriptionID string, atPeriodEnd bool, key string) (*billingclient.Subscription, error)
}

type orgBilling interface {
	GetOrCreateCustomer(ctx context.Context, organizationID string, payer services.CustomerPayer) (string, error)
	Sync(ctx context.Context, organizationID string) (*repositories.AccountSnapshot, error)
}

type workspaceLister interface {
	Organizations(ctx context.Context, userID string) ([]accountclient.Workspace, error)
}

type companyStore interface {
	ListOrganizationsWithCompanies(ctx context.Context) ([]string, error)
	ListCompaniesOfOrganization(ctx context.Context, organizationID string) ([]repositories.CompanyRef, error)
	ListCompanyIndexGaps(ctx context.Context) ([]repositories.CompanyIndexGap, error)
	BackfillIndexKeys(ctx context.Context, companyPK, organizationID, createdAt string) error
}

type levelReporter interface {
	MarkAndFlush(ctx context.Context, organizationID string) error
}

type reacher interface {
	Reach(ctx context.Context, companyID, userID string) (string, bool, error)
}

type deps struct {
	snaps      snapshotStore
	billing    billingAPI
	orgs       orgBilling
	workspaces workspaceLister
	companies  companyStore
	levels     levelReporter
	reach      reacher // nil: nothing can be backfilled, gaps are listed
	// reportLevelsAll is -report-levels-all.
	reportLevelsAll bool
	now        func() time.Time
}

// Report is what the run did (or, dry, would do).
type Report struct {
	Apply      bool
	Backfilled []string
	Unresolved []string
	Migrated   []string
	Review     []string
	Levels     []string // organizations whose level was (or, dry, would be) reported by -report-levels-all
}

func (r *Report) NeedsReview() bool { return len(r.Review) > 0 || len(r.Unresolved) > 0 }

func (r *Report) Print(w io.Writer) {
	mode := "DRY RUN (nothing written; pass -apply)"
	if r.Apply {
		mode = "APPLIED"
	}
	_, _ = fmt.Fprintf(w, "migrate-billing-org: %s\n", mode)
	sections := []struct {
		title string
		lines []string
	}{
		{"index backfill", r.Backfilled}, {"index unresolved", r.Unresolved},
		{"migrated", r.Migrated}, {"levels reported", r.Levels}, {"needs review", r.Review},
	}
	for _, sec := range sections {
		_, _ = fmt.Fprintf(w, "\n%s (%d)\n", sec.title, len(sec.lines))
		for _, l := range sec.lines {
			_, _ = fmt.Fprintf(w, "  %s\n", l)
		}
	}
}

func run(ctx context.Context, d deps, apply bool) (*Report, error) {
	rep := &Report{Apply: apply}
	if err := checkCompanyIndex(ctx, d, apply, rep); err != nil {
		return rep, fmt.Errorf("company index: %w", err)
	}
	users, err := d.snaps.ListUserSnapshots(ctx)
	if err != nil {
		return rep, err
	}
	for _, u := range users {
		if u.SubscriptionID == "" {
			continue
		}
		if err := migrateUser(ctx, d, apply, u, rep); err != nil {
			return rep, fmt.Errorf("%s: %w", repositories.AccountBillingPK(u.UserID), err)
		}
	}
	if d.reportLevelsAll {
		if err := reportAllLevels(ctx, d, apply, rep); err != nil {
			return rep, fmt.Errorf("levels: %w", err)
		}
	}
	return rep, nil
}

// reportAllLevels marks (and flushes) the companies level of every
// organization holding DF-e companies, so an organization that never
// subscribed still has its level known. Dry: list only.
func reportAllLevels(ctx context.Context, d deps, apply bool, rep *Report) error {
	orgs, err := d.companies.ListOrganizationsWithCompanies(ctx)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		if apply {
			if err := d.levels.MarkAndFlush(ctx, org); err != nil {
				return fmt.Errorf("%s: %w", org, err)
			}
		}
		rep.Levels = append(rep.Levels, org)
	}
	return nil
}

func checkCompanyIndex(ctx context.Context, d deps, apply bool, rep *Report) error {
	gaps, err := d.companies.ListCompanyIndexGaps(ctx)
	if err != nil {
		return err
	}
	for _, g := range gaps {
		org := ""
		if g.MissingOrganization {
			if d.reach == nil || g.OwnerUserID == "" {
				rep.Unresolved = append(rep.Unresolved, g.PK+": no organization_id and nothing to resolve it with")
				continue
			}
			got, ok, err := d.reach.Reach(ctx, g.PK, g.OwnerUserID)
			if err != nil {
				return err
			}
			if !ok || got == "" {
				rep.Unresolved = append(rep.Unresolved, g.PK+": ctech-account does not place its owner on it")
				continue
			}
			org = got
		}
		createdAt := ""
		if g.MissingCreatedAt {
			createdAt = d.now().UTC().Format(time.RFC3339)
		}
		if apply {
			if err := d.companies.BackfillIndexKeys(ctx, g.PK, org, createdAt); err != nil {
				return err
			}
		}
		rep.Backfilled = append(rep.Backfilled, g.PK)
	}
	return nil
}

func migrateUser(ctx context.Context, d deps, apply bool, u repositories.AccountSnapshot, rep *Report) error {
	userRef := repositories.AccountBillingPK(u.UserID)
	ent, err := d.billing.GetEntitlements(ctx, userRef)
	if errors.Is(err, billingclient.ErrCustomerNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	sub := firstEntitled(ent.Subscriptions)
	if sub == nil {
		return nil // already cancelled: migrated by an earlier run, or never live
	}
	for _, it := range sub.Items {
		if it.UnitAmount != 0 {
			rep.Review = append(rep.Review, fmt.Sprintf("%s: subscription %s has a paid price (%s); not migrated, nothing duplicated", userRef, sub.ID, it.PriceID))
			return nil
		}
	}
	orgs, err := ownedOrganizationsWithCompanies(ctx, d, u.UserID)
	if err != nil {
		return err
	}
	if len(orgs) == 0 {
		rep.Review = append(rep.Review, fmt.Sprintf("%s: owns no organization with DF-e companies; subscription %s left as is", userRef, sub.ID))
		return nil
	}
	priceIDs := make([]string, 0, len(sub.Items))
	items := make([]billingclient.Item, 0, len(sub.Items))
	for _, it := range sub.Items {
		priceIDs = append(priceIDs, it.PriceID)
		items = append(items, billingclient.Item{PriceID: it.PriceID})
	}
	if !apply {
		for _, org := range orgs {
			rep.Migrated = append(rep.Migrated, fmt.Sprintf("%s → %s [%s]", userRef, repositories.OrgBillingPK(org), strings.Join(priceIDs, ",")))
		}
		return nil
	}

	customer, err := d.billing.GetCustomer(ctx, ent.CustomerID)
	if err != nil {
		return err
	}
	payer := services.CustomerPayer{UserID: u.UserID, Name: customer.Name, Email: customer.Email}
	allLive := true
	for _, org := range orgs {
		target, err := d.snaps.GetOrg(ctx, org)
		if err != nil {
			return err
		}
		if target == nil || !services.GrantsService(target) {
			customerID, err := d.orgs.GetOrCreateCustomer(ctx, org, payer)
			if err != nil {
				return err
			}
			if _, err := d.billing.CreateSubscription(ctx, customerID, items, migrateSubscriptionKey(u.UserID, org)); err != nil {
				return err
			}
			if target, err = d.orgs.Sync(ctx, org); err != nil {
				return err
			}
		}
		if !services.GrantsService(target) {
			allLive = false
			rep.Review = append(rep.Review, fmt.Sprintf("%s: %s is %q after creation; USER_ subscription kept", userRef, org, target.Status))
			continue
		}
		toPeriod := target.PeriodStart
		if toPeriod == "" {
			toPeriod = d.now().Format(calendarPeriodLayout)
		}
		if _, err := d.snaps.CopyUsageOnce(ctx, usageSources(u, sub, org), org, toPeriod, migrateUsageMarker(u.UserID, org)); err != nil {
			return err
		}
		// The organization's initial companies level (whole count). The
		// marker is durable: if billing does not take it now, the API's
		// sweeper delivers it.
		if err := d.levels.MarkAndFlush(ctx, org); err != nil {
			return err
		}
		rep.Migrated = append(rep.Migrated, fmt.Sprintf("%s → %s [%s]", userRef, repositories.OrgBillingPK(org), strings.Join(priceIDs, ",")))
	}
	if !allLive {
		return nil
	}
	_, err = d.billing.CancelSubscription(ctx, sub.ID, false, migrateCancelKey(sub.ID))
	return err
}

// usageSources are the counters the organization inherits: the user's for the
// current period, and whatever the dual read already wrote to the organization
// under the user's period. Both the subscription's period and the possibly
// stale USER_ row's period are read; CopyUsageOnce skips the target itself.
func usageSources(u repositories.AccountSnapshot, sub *billingclient.EntitlementSubscription, org string) []repositories.UsageSource {
	periods := []string{sub.Period.Start}
	if u.PeriodStart != "" && u.PeriodStart != sub.Period.Start {
		periods = append(periods, u.PeriodStart)
	}
	out := []repositories.UsageSource{}
	for _, p := range periods {
		if p == "" {
			continue
		}
		out = append(out,
			repositories.UsageSource{AccountID: u.UserID, Period: p},
			repositories.UsageSource{AccountID: org, Period: p},
		)
	}
	return out
}

func firstEntitled(subs []billingclient.EntitlementSubscription) *billingclient.EntitlementSubscription {
	for i := range subs {
		if subs[i].Entitled {
			return &subs[i]
		}
	}
	return nil
}

// ownedOrganizationsWithCompanies is spec § 4 step 2: organizations (not
// spaces) the user owns that hold at least one DF-e company.
func ownedOrganizationsWithCompanies(ctx context.Context, d deps, userID string) ([]string, error) {
	all, err := d.workspaces.Organizations(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, w := range all {
		if w.Role != services.AccountRoleOwner || !w.IsOrganization() {
			continue
		}
		refs, err := d.companies.ListCompaniesOfOrganization(ctx, w.ID)
		if err != nil {
			return nil, err
		}
		if len(refs) > 0 {
			out = append(out, w.ID)
		}
	}
	return out, nil
}
```

Add to `api/internal/repositories/organizations.go`:

```go
// ListOrganizationsWithCompanies scans the distinct organizations of the
// company-keyed records. Used once, by cmd/migrate-billing-org
// -report-levels-all; never on a request path.
func (r *OrganizationRepository) ListOrganizationsWithCompanies(ctx context.Context) ([]string, error) {
	seen := map[string]bool{}
	var start map[string]types.AttributeValue
	for {
		res, err := r.ScanRaw(ctx, &dynamodb.ScanInput{
			TableName:                aws.String(r.TableName),
			FilterExpression:         aws.String("attribute_exists(#o)"),
			ProjectionExpression:     aws.String("pk, #o"),
			ExpressionAttributeNames: map[string]string{"#o": AttrOrganizationID},
			ExclusiveStartKey:        start,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range res.Items {
			if org := itemString(item, AttrOrganizationID); org != "" && IsCompanyKey(itemString(item, "pk")) {
				seen[org] = true
			}
		}
		if len(res.LastEvaluatedKey) == 0 {
			break
		}
		start = res.LastEvaluatedKey
	}
	out := make([]string, 0, len(seen))
	for org := range seen {
		out = append(out, org)
	}
	sort.Strings(out)
	return out, nil
}
```

(add `sort` to the imports).

- [ ] **Step 4: Implement the command** — `api/cmd/migrate-billing-org/main.go`

```go
// Command migrate-billing-org moves the DF-e subscriptions from the owner's user
// (USER_{sub}) to the ctech-account organizations that hold the companies
// (ORG_{organization_id}). docs/specs/2026-10-10-organization-subscription.md § 4.
//
// Dry run by default; -apply writes. Idempotent: a second -apply creates
// nothing. A user whose subscription has any non-zero price is listed and left
// alone, never duplicated.
//
//	AWS_REGION=… BILLING_API_URL=… BILLING_CLIENT_ID=… BILLING_CLIENT_SECRET=… CTECH_URL=… \
//	ACCOUNT_CLIENT_ID=… ACCOUNT_CLIENT_SECRET=… \
//	ACCOUNT_WORKSPACE_CLIENT_ID=… ACCOUNT_WORKSPACE_CLIENT_SECRET=… \
//	go run ./cmd/migrate-billing-org -table-prefix prod_dfe [-report-levels-all] [-apply]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"gopkg.aoctech.app/api-commons/awsconfig"
	"gopkg.aoctech.app/api-commons/cache"

	"gopkg.aoctech.app/dfe/api/internal/accountclient"
	"gopkg.aoctech.app/dfe/api/internal/awsclient"
	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/config"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	exitError  = 1
	exitUsage  = 2
	exitReview = 3
)

func main() {
	prefix := flag.String("table-prefix", "", "ctech-dfe table prefix (e.g. prod_dfe)")
	region := flag.String("region", "us-east-1", "AWS region")
	apply := flag.Bool("apply", false, "write; without it nothing is written")
	reportLevelsAll := flag.Bool("report-levels-all", false, "also report the dfe_companies level of every organization holding DF-e companies")
	flag.Parse()
	if *prefix == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: migrate-billing-org -table-prefix PREFIX [-region R] [-apply]")
		os.Exit(exitUsage)
	}

	ctx := context.Background()
	d, err := wire(ctx, *prefix, *region)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate-billing-org: %v\n", err)
		os.Exit(exitError)
	}
	d.reportLevelsAll = *reportLevelsAll
	rep, err := run(ctx, d, *apply)
	rep.Print(os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nmigrate-billing-org failed: %v\n", err)
		os.Exit(exitError)
	}
	if rep.NeedsReview() {
		os.Exit(exitReview)
	}
}

func wire(ctx context.Context, prefix, region string) (deps, error) {
	awsCfg, err := awsconfig.Load(ctx, region)
	if err != nil {
		return deps{}, fmt.Errorf("loading aws config: %w", err)
	}
	db := dynamodb.NewFromConfig(awsCfg)
	cfg := &config.Config{TablePrefix: prefix, AWSRegion: region}
	mem := cache.NewMemoryBackend(1000)
	ctechURL := os.Getenv("CTECH_URL")
	tokenURL := billingclient.TokenURLFor(ctechURL)

	bill := billingclient.New(billingclient.Config{
		BaseURL: os.Getenv("BILLING_API_URL"), TokenURL: tokenURL,
		ClientID: os.Getenv("BILLING_CLIENT_ID"), ClientSecret: os.Getenv("BILLING_CLIENT_SECRET"), Cache: mem,
	})
	if bill == nil {
		return deps{}, fmt.Errorf("BILLING_API_URL, BILLING_CLIENT_ID, BILLING_CLIENT_SECRET and CTECH_URL are required")
	}
	workspaces := accountclient.NewWorkspace(accountclient.Config{
		BaseURL: ctechURL, TokenURL: tokenURL,
		ClientID: os.Getenv("ACCOUNT_WORKSPACE_CLIENT_ID"), ClientSecret: os.Getenv("ACCOUNT_WORKSPACE_CLIENT_SECRET"), Cache: mem,
	})
	if workspaces == nil {
		return deps{}, fmt.Errorf("ACCOUNT_WORKSPACE_CLIENT_ID and ACCOUNT_WORKSPACE_CLIENT_SECRET are required")
	}
	var reach reacher
	if c := accountclient.New(accountclient.Config{
		BaseURL: ctechURL, TokenURL: tokenURL,
		ClientID: os.Getenv("ACCOUNT_CLIENT_ID"), ClientSecret: os.Getenv("ACCOUNT_CLIENT_SECRET"), Cache: mem,
	}); c != nil {
		reach = c
	}

	billingRepo := repositories.NewAccountBillingRepository(db, cfg)
	orgRepo := repositories.NewOrganizationRepository(db, cfg)
	auditRepo := repositories.NewAuditLogRepository(db, cfg)
	certRepo := repositories.NewCertificateRepository(db, cfg)
	orgUserRepo := repositories.NewOrgUserRepository(db, cfg)
	roleRepo := repositories.NewRoleRepository(db, cfg)
	memberSvc := services.NewMembershipService(orgUserRepo, auditRepo, roleRepo, mem)
	certSvc := services.NewCertificateService(certRepo, auditRepo, &awsclient.Clients{}, "")
	orgSvc := services.NewOrganizationService(orgRepo, auditRepo, certRepo, orgUserRepo, certSvc, memberSvc, mem)
	// The enablement source is required: the level reported is the count of
	// ENABLED companies, and without it companiesUsed counts every linked one.
	billingSvc := services.NewBillingService(billingRepo, bill, nil, memberSvc, orgSvc, mem).
		WithEnablement(services.NewFiscalConfigEnablement(
			repositories.NewNfeConfigRepository(db, cfg), repositories.NewNfceConfigRepository(db, cfg),
			repositories.NewCteConfigRepository(db, cfg), repositories.NewMdfeConfigRepository(db, cfg),
			repositories.NewNfseConfigRepository(db, cfg)))
	levels := services.NewLevelReporter(billingRepo, bill, billingSvc)

	return deps{
		snaps: billingRepo, billing: bill, orgs: billingSvc, workspaces: workspaces,
		companies: orgRepo, levels: levels, reach: reach, now: time.Now,
	}, nil
}
```

(`awsclient` import path: `grep -rn '"gopkg.aoctech.app/dfe/api/internal/awsclient"' api/tests/integration/setup_test.go`; use the same.)

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./cmd/migrate-billing-org/ && go vet ./cmd/migrate-billing-org/ && go build ./cmd/migrate-billing-org/`
Expected: PASS, clean.

- [ ] **Step 6: Commit**

```bash
git add api/cmd/migrate-billing-org api/internal/repositories/organizations.go
git commit -m "feat(dfe): migrate-billing-org moves user subscriptions to their organizations"
```

---

### Task 13: The users meter leaves the UI

> Execute with the `/impeccable` skill.

**Files:**
- Modify: `ui/src/lib/constants/billing.ts` (drop `METER_USERS`, its label; `ACCOUNT_METERS = [METER_COMPANIES]`)
- Modify: `ui/src/components/billing/PlanChooser.tsx:3,100`
- Modify: `ui/src/lib/types/billing.ts:41` (metadata comment)
- Modify: `ui/src/components/billing/__tests__/UsageList.test.tsx`
- Create: `ui/src/lib/billing/__tests__/catalog.test.ts`
- Modify: `ui/src/app/guide/account/page.tsx:118` (copy)

**Interfaces:**
- Consumes: nothing new.
- Produces: `ACCOUNT_METERS` is `[METER_COMPANIES]`; `METER_USERS` no longer exported.

- [ ] **Step 1: Write the failing tests**

Append to `ui/src/components/billing/__tests__/UsageList.test.tsx`:

```tsx
  it('never lists users, even when an older price still carries the quota', () => {
    render(<UsageList quotas={{nfe: 3, companies: 1, users: 1}} usage={{users: {used: 1, limit: 1}}}/>);
    expect(screen.queryByText('Usuários')).not.toBeInTheDocument();
    expect(screen.queryByText('users')).not.toBeInTheDocument();
    expect(screen.getByText('Empresas')).toBeInTheDocument();
  });
```

`ui/src/lib/billing/__tests__/catalog.test.ts`:

```ts
import {describe, expect, it} from 'vitest';
import {grantedMeters} from '@/lib/billing/catalog';
import {ACCOUNT_METERS} from '@/lib/constants/billing';

describe('grantedMeters', () => {
  it('has no users meter', () => {
    expect(ACCOUNT_METERS).toEqual(['companies']);
    expect(grantedMeters({nfe: 3, users: 25, companies: 10})).toEqual(['nfe', 'companies']);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd ui && npx vitest run --maxWorkers=2 src/components/billing/__tests__/UsageList.test.tsx src/lib/billing/__tests__/catalog.test.ts; echo "exit=$?"`
Expected: `exit=1` (the users meter is still listed).

- [ ] **Step 3: Implement** — in `ui/src/lib/constants/billing.ts` delete `export const METER_USERS = 'users';` and the `[METER_USERS]: 'Usuários',` label; set `export const ACCOUNT_METERS = [METER_COMPANIES] as const;` and change its comment to "Meters that count current state (enabled companies) rather than accumulated issuance. People are not metered: they belong to ctech-account and are shared between products." In `PlanChooser.tsx`, drop `METER_USERS` from the import and write the suffix as `{m.meter === METER_COMPANIES ? ' /empresa' : ' /doc'}`. In `types/billing.ts`, the metadata comment becomes "Where the quotas (`quota_nfe`, `quota_companies`, …) and the `meter` live." In the guide (`account/page.tsx:118`), the sentence becomes "O plano define quantas empresas podem emitir e quantos documentos de cada tipo por mês."

- [ ] **Step 4: Verify**

Run: `cd ui && npx vitest run --maxWorkers=2 src/components/billing/__tests__/UsageList.test.tsx src/lib/billing/__tests__/catalog.test.ts; echo "exit=$?"` then `npx eslint src --ext .ts,.tsx`
Expected: `exit=0`; eslint zero errors, zero warnings. `grep -rn "METER_USERS\|Usuários'" ui/src` returns nothing.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/constants/billing.ts ui/src/components/billing/PlanChooser.tsx ui/src/lib/types/billing.ts ui/src/components/billing/__tests__/UsageList.test.tsx ui/src/lib/billing/__tests__/catalog.test.ts ui/src/app/guide/account/page.tsx
git commit -m "feat(ui): o plano não limita mais usuários"
```

---

### Task 14: The plan screens act on the organization

> Execute with the `/impeccable` skill.

**Files:**
- Modify: `ui/src/lib/types/billing.ts` (`organization`, `manageable`)
- Modify: `ui/src/lib/api/query-keys.ts:132-139` (`subscription(companyPk)`, `subscriptionAll()`; drop `orgPlan`)
- Modify: `ui/src/lib/api/client.ts:1211-1245` (comment; drop `getOrganizationPlan`)
- Modify: `ui/src/lib/hooks/useSubscription.ts`
- Modify: `ui/src/lib/hooks/useSubscriptionNotice.ts`
- Create: `ui/src/lib/billing/organization.ts`
- Create: `ui/src/lib/billing/__tests__/organization.test.ts`
- Create: `ui/src/lib/hooks/__tests__/useSubscriptionNotice.test.tsx`
- Modify: `ui/src/app/assinatura/page.tsx`
- Modify: `ui/src/app/onboarding/plano/page.tsx` (title, organization name, invalidation)
- Modify: `ui/src/components/billing/ChangePlanDialog.tsx`, `ui/src/components/billing/CancelSubscriptionDialog.tsx` (invalidation key)
- Modify: `ui/src/app/guide/account/page.tsx` (section "Plano e cobrança")

**Interfaces:**
- Consumes: Task 6 response fields `organization {id, name}`, `manageable`.
- Produces:
  - `AccountSubscription.organization?: {id: string; name: string}`, `AccountSubscription.manageable?: boolean`
  - `queryKeys.billing.subscription(companyPk: string)` → `['billing', 'subscription', companyPk]`; `queryKeys.billing.subscriptionAll()` → `['billing', 'subscription']`
  - `organizationPlanTitle(sub?: AccountSubscription): string`; `canManagePlan(sub?: AccountSubscription): boolean`; `MEMBER_NO_PLAN_MESSAGE` (all in `lib/billing/organization.ts`)

- [ ] **Step 1: Write the failing tests**

`ui/src/lib/billing/__tests__/organization.test.ts`:

```ts
import {describe, expect, it} from 'vitest';
import {canManagePlan, MEMBER_NO_PLAN_MESSAGE, organizationPlanTitle} from '@/lib/billing/organization';
import type {AccountSubscription} from '@/lib/types/billing';

const base: AccountSubscription = {
  has_subscription: true, status: 'ACTIVE', plan: 'pro', grants_service: true,
  cancel_at_period_end: false, period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('organization plan', () => {
  it('names the organization', () => {
    expect(organizationPlanTitle({...base, organization: {id: 'org_1', name: 'Escritório Silva'}}))
      .toBe('Plano da organização Escritório Silva');
  });

  it('falls back when the name is unknown', () => {
    expect(organizationPlanTitle({...base, organization: {id: 'org_1', name: ''}})).toBe('Plano da organização');
    expect(organizationPlanTitle(undefined)).toBe('Plano da organização');
  });

  it('only manageable answers allow managing', () => {
    expect(canManagePlan({...base, manageable: true})).toBe(true);
    expect(canManagePlan({...base, manageable: false})).toBe(false);
    expect(canManagePlan({...base})).toBe(false);
    expect(canManagePlan(undefined)).toBe(false);
  });

  it('tells a member who chooses the plan, without a dash', () => {
    expect(MEMBER_NO_PLAN_MESSAGE).toMatch(/ainda não escolheu um plano/);
    expect(MEMBER_NO_PLAN_MESSAGE).not.toContain('—');
  });
});
```

`ui/src/lib/hooks/__tests__/useSubscriptionNotice.test.tsx`:

```tsx
import {describe, expect, it, vi} from 'vitest';
import {renderHook} from '@testing-library/react';
import type {AccountSubscription} from '@/lib/types/billing';

const state: { subscription?: AccountSubscription } = {};
vi.mock('@/lib/hooks/useSubscription', () => ({
  useSubscription: () => ({subscription: state.subscription, isPending: false}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'cmp_1', role: 'USER'}})}));

import {useSubscriptionNotice} from '@/lib/hooks/useSubscriptionNotice';

const missing: AccountSubscription = {
  has_subscription: false, status: '', plan: '', grants_service: false, cancel_at_period_end: false,
  period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('useSubscriptionNotice', () => {
  it('warns an owner or admin of the organization, whatever their DF-e role', () => {
    state.subscription = {...missing, manageable: true};
    expect(renderHook(() => useSubscriptionNotice()).result.current.notice).not.toBeNull();
  });

  it('stays silent for anyone who cannot act on it', () => {
    state.subscription = {...missing, manageable: false};
    expect(renderHook(() => useSubscriptionNotice()).result.current.notice).toBeNull();
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd ui && npx vitest run --maxWorkers=2 src/lib/billing/__tests__/organization.test.ts src/lib/hooks/__tests__/useSubscriptionNotice.test.tsx; echo "exit=$?"`
Expected: `exit=1` (module `@/lib/billing/organization` not found; the notice is null for a non-OWNER DF-e role).

- [ ] **Step 3: Implement**

`ui/src/lib/billing/organization.ts`:

```ts
import type {AccountSubscription} from '@/lib/types/billing';

/**
 * The DF-e plan belongs to the ctech-account organization of the selected
 * company. Its owners and admins manage it; everybody else reads it. The API
 * says which with `manageable`; the screen never derives it from the DF-e role.
 */

export const MEMBER_NO_PLAN_MESSAGE =
  'A organização ainda não escolheu um plano. Peça a um proprietário ou administrador da organização para escolher.';

export function organizationPlanTitle(sub?: AccountSubscription): string {
  const name = sub?.organization?.name?.trim();
  return name ? `Plano da organização ${name}` : 'Plano da organização';
}

export function canManagePlan(sub?: AccountSubscription): boolean {
  return sub?.manageable === true;
}
```

`ui/src/lib/types/billing.ts`, in `AccountSubscription`:

```ts
  /** The ctech-account organization the plan belongs to. */
  organization?: { id: string; name: string }
  /** True for the organization's owners and admins: they may choose, change, cancel and pay. */
  manageable?: boolean
```

`query-keys.ts`:

```ts
    subscription: (companyPk: string) => ['billing', 'subscription', companyPk] as const,
    /** Every company's cached subscription; what mutations invalidate. */
    subscriptionAll: () => ['billing', 'subscription'] as const,
```

and delete `orgPlan`. In `client.ts`, delete `getOrganizationPlan` and replace the billing comment with: "Billing: the plan of the selected company's organization. The `Dfe-Organization-Pk` header the interceptor adds selects it; owners and admins of the organization manage it, everyone else reads it (`manageable`)."

`useSubscription.ts`: read `selectedOrg` from `useAuth()`; `queryKey: queryKeys.billing.subscription(selectedOrg?.pk ?? '')`, `enabled: !!user && !!selectedOrg`; comment: "Switching between two companies of one organization refetches and shows the same plan."

`useSubscriptionNotice.ts`:

```ts
export function useSubscriptionNotice(): { notice: BillingNotice | null; isPending: boolean } {
  const {subscription, isPending} = useSubscription();
  if (!canManagePlan(subscription)) return {notice: null, isPending: false};
  return {notice: noticeForSubscription(subscription), isPending};
}
```

with a comment: "Only the organization's owners and admins see it: they are the ones who can act on it."

`ChangePlanDialog.tsx`, `CancelSubscriptionDialog.tsx`, `onboarding/plano/page.tsx`: every `invalidateQueries({queryKey: queryKeys.billing.subscription()})` becomes `queryKeys.billing.subscriptionAll()` (grep the UI for `billing.subscription()` to catch all).

`assinatura/page.tsx`: replace `OrganizationPlanView`, `OwnerSubscriptionView` and the role switch with one `OrganizationSubscriptionView`:
- `PageHeader title={organizationPlanTitle(subscription)}`, description "O plano vale para todas as empresas da organização." when manageable, otherwise "O plano que vale para todas as empresas da organização. Só proprietários e administradores da organização podem alterá-lo."
- no subscription: manageable → current "Escolher plano" card; otherwise a card with `MEMBER_NO_PLAN_MESSAGE` and no button.
- with a subscription: the plan card, open invoice "Pagar agora", "Mudar de plano", "Cancelar assinatura", the invoices section and the dialogs render only when `canManagePlan(subscription)`; "Uso do período" renders for everyone.
- delete `ROLE_OWNER` import and `useQuery` for the org plan.

`onboarding/plano/page.tsx`: `title="Escolha o plano da organização"`; description "O plano vale para todas as empresas da organização {name} e define quantas empresas podem emitir e quantos documentos de cada tipo por mês. Dá para trocar depois." using `useSubscription().subscription?.organization?.name` (omit the name when empty).

Guide `account/page.tsx`, section `assinatura`: the plan belongs to the organization in the CTech account and covers all its companies; owners and administrators of the organization choose, change, cancel and pay; everyone else sees the plan and the usage; a company counts toward the limit when its first fiscal configuration is saved; companies only linked do not count. No travessão. Update the screenshot alt text if it mentions "conta".

- [ ] **Step 4: Verify**

Run: `cd ui && npx vitest run --maxWorkers=2 src/lib/billing src/lib/hooks/__tests__/useSubscriptionNotice.test.tsx src/components/billing; echo "exit=$?"` then `npx eslint src --ext .ts,.tsx` then `npm run screens:capture` (the assinatura screen; commit the updated captures the script writes).
Expected: `exit=0`; eslint clean; captures updated.

- [ ] **Step 5: Commit**

```bash
git add ui/src
git commit -m "feat(ui): a assinatura é da organização, gerida por proprietários e administradores"
```

---

### Task 15: Onboarding asks for the company before the plan

> Execute with the `/impeccable` skill.

**Files:**
- Modify: `ui/src/lib/constants/onboarding.ts:1-22,55-96` (order, header comment)
- Create: `ui/src/lib/onboarding/target.ts`
- Create: `ui/src/lib/onboarding/__tests__/target.test.ts`
- Modify: `ui/src/components/onboarding/OnboardingGate.tsx`
- Modify: `ui/src/lib/hooks/useOnboarding.ts:195-205` (plan step applicability)
- Modify: `ui/src/app/onboarding/plano/page.tsx` (next step), `ui/src/app/onboarding/retorno/page.tsx:113-118` (next step and copy), `ui/src/app/onboarding/empresa/page.tsx:44-47` (copy)
- Modify: `ui/src/app/guide/getting-started/page.tsx` (order of the first steps)

**Interfaces:**
- Consumes: Task 14 `canManagePlan`.
- Produces: `onboardingTarget(input: {hasCompany: boolean; subscription?: AccountSubscription}): string | null` in `lib/onboarding/target.ts`.

- [ ] **Step 1: Write the failing test** — `ui/src/lib/onboarding/__tests__/target.test.ts`

```ts
import {describe, expect, it} from 'vitest';
import {onboardingTarget} from '@/lib/onboarding/target';
import type {AccountSubscription} from '@/lib/types/billing';

const none: AccountSubscription = {
  has_subscription: false, status: '', plan: '', grants_service: false, cancel_at_period_end: false,
  period_start: '', period_end: '', quotas: {}, no_charge: false,
};

describe('onboardingTarget', () => {
  it('asks for a company first: there is no organization to subscribe before it', () => {
    expect(onboardingTarget({hasCompany: false, subscription: undefined})).toBe('/onboarding/empresa');
  });

  it('sends an owner or admin of an organization with no plan to the plan step', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, manageable: true}})).toBe('/onboarding/plano');
  });

  it('never gates someone who cannot choose the plan', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, manageable: false}})).toBeNull();
  });

  it('waits for the payment of an incomplete subscription', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, has_subscription: true, status: 'INCOMPLETE', manageable: true}}))
      .toBe('/onboarding/retorno');
  });

  it('lets a second company of an organization that already pays straight through', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, has_subscription: true, status: 'ACTIVE', manageable: true}})).toBeNull();
  });

  it('no-charge installations skip the plan', () => {
    expect(onboardingTarget({hasCompany: true, subscription: {...none, no_charge: true, manageable: true}})).toBeNull();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd ui && npx vitest run --maxWorkers=2 src/lib/onboarding/__tests__/target.test.ts; echo "exit=$?"`
Expected: `exit=1` (module not found).

- [ ] **Step 3: Implement**

`ui/src/lib/onboarding/target.ts`:

```ts
import {ONBOARDING_ROOT, STEP_CHECKOUT_RETURN, STEP_COMPANY, STEP_PLAN} from '@/lib/constants/onboarding';
import {STATUS_INCOMPLETE} from '@/lib/constants/billing';
import {canManagePlan} from '@/lib/billing/organization';
import type {AccountSubscription} from '@/lib/types/billing';

/**
 * Where the first-run gate sends someone, or null to let them through.
 *
 * The company comes first: the plan belongs to the organization that holds it,
 * so there is nothing to subscribe before a company is linked. Only those who
 * can choose the plan (the organization's owners and admins) are sent to it.
 */
export function onboardingTarget({hasCompany, subscription}: {
  hasCompany: boolean
  subscription?: AccountSubscription
}): string | null {
  if (!hasCompany) return `${ONBOARDING_ROOT}/${STEP_COMPANY}`;
  if (!subscription || subscription.no_charge || !canManagePlan(subscription)) return null;
  if (subscription.status === STATUS_INCOMPLETE) return `${ONBOARDING_ROOT}/${STEP_CHECKOUT_RETURN}`;
  if (!subscription.has_subscription) return `${ONBOARDING_ROOT}/${STEP_PLAN}`;
  return null;
}
```

`OnboardingGate.tsx`: replace the `memberOnly`/`needsPlan`/`awaitingPayment`/`needsCompany`/`target` block with `const target = onboardingTarget({hasCompany: organizations.length > 0, subscription});` and `shouldRedirect = !exempt && !error && !!target`; remove `memberOnly` from the early return and the now-unused imports (`ROLE_OWNER`, step constants, `STATUS_INCOMPLETE`). Rewrite the component comment's first rule as "Only those who can choose the plan are sent to it: the organization's owners and admins, as ctech-account says (`manageable`)." The spinner condition becomes `isPending && organizations.length > 0 || shouldRedirect` (the subscription query is disabled without a selected company, and a disabled query stays pending).

`constants/onboarding.ts`: move the `STEP_COMPANY` entry above `STEP_PLAN` in `ONBOARDING_STEPS`; change the header comment's first sentence to "there is no plan before the organization that holds the company, no certificate before the company it belongs to, …".

`useOnboarding.ts`: `[STEP_PLAN]: hasCompany && canManagePlan(subscription)` in `applicableById` (import `canManagePlan`).

`onboarding/plano/page.tsx`: on success without checkout go to `${ONBOARDING_ROOT}/${STEP_CERTIFICATE}`; the no-charge redirect goes to `STEP_CERTIFICATE` too.

`onboarding/retorno/page.tsx`: the continue button goes to `STEP_CERTIFICATE`; its helper text becomes "Você já pode enviar o certificado. A emissão libera quando o pagamento for confirmado."

`onboarding/empresa/page.tsx`, item 3: "Na sequência, o plano da organização, o certificado A1 e os documentos que você emite."

`guide/getting-started/page.tsx`: the steps read company (in the CTech account), then the organization's plan, then certificate; the "Plano" term says the plan is the organization's and covers all its companies. No travessão.

- [ ] **Step 4: Verify**

Run: `cd ui && npx vitest run --maxWorkers=2 src/lib/onboarding src/lib/billing src/components; echo "exit=$?"` then `npx eslint src --ext .ts,.tsx` then `npm run screens:capture` (onboarding plan and company steps, getting-started guide).
Expected: `exit=0`; eslint clean; captures updated.

- [ ] **Step 5: Commit**

```bash
git add ui/src
git commit -m "feat(ui): o primeiro acesso vincula a empresa antes de escolher o plano da organização"
```

---

### Task 16: Deploy runbook

**Files:**
- Modify: `DEPLOYMENT.md` (new section "Assinatura por organização (2026-10-10)")
- Modify: `OVERVIEW.md` (billing paragraph: subscription per organization)

**Interfaces:** none (documentation).

- [ ] **Step 1: Write the section** — in `DEPLOYMENT.md`:

```markdown
## Assinatura por organização (2026-10-10)

Spec: docs/specs/2026-10-10-organization-subscription.md. Plan: docs/plans/2026-10-10-organization-subscription.md.

Prerequisites
1. ctech-billing § 10 step 1 deployed: ORG_ customers + CUSTOMER_ORG# pointer, catalogue without quota_users, POST /v1.0/usage/levels, price_dfe_ondemand_companies_monthly (metered, aggregation max, meter dfe_companies, included 0) with price_dfe_ondemand_company archived.
2. ctech-account: create the DF-e workspace client (internal:account:org-member, internal:account:user-organizations):
   go run ./cmd/createclient -client-id ctech-dfe-workspaces -name "ctech-dfe workspaces" \
     -scopes internal:account:org-member,internal:account:user-organizations \
     -ssm-path-client /ctech-dfe/{env}/account-workspace-client-id \
     -ssm-path-secret /ctech-dfe/{env}/account-workspace-client-secret
3. Production check: list entitled USER_ subscriptions and their prices (the script's dry run prints them). At design time: two, both price_dfe_unlimited_internal_monthly (R$ 0).

Order
1. cdk deploy of the DynamoDB stack (organization-index GSI; wait until ACTIVE), then the API stack (new SSM env).
2. Worker deploy (passes billing_organization_id through), then API deploy (dual read on).
3. go run ./cmd/migrate-billing-org -table-prefix {prefix}           # dry run: review index gaps, migrations, review list
   go run ./cmd/migrate-billing-org -table-prefix {prefix} -report-levels-all -apply    # exit 3 = something listed for review
   Re-run the dry run: it must list nothing to migrate.
   Check no level is stuck: account_billing level-dirty-index should be empty a few minutes after the run (the sweeper runs every 2 minutes).
4. After every USER_ subscription is cancelled and a full billing period has passed: deploy Phase 2 (Task 17), which removes the dual read.

Rollback: Phase 1 is additive; redeploying the previous API reads USER_ rows again (they are not deleted). After the script ran, the USER_ subscriptions are cancelled: a rollback past step 3 needs them recreated in billing.

Follow-up (family-wide): api/internal/accountclient/workspace.go duplicates ctech-billing's accountclient/membership.go; extract both to ctech-go-common.
```

In `OVERVIEW.md`, wherever billing is described as "per account / owner", state that the subscription belongs to the ctech-account organization of the selected company, managed by its owners and admins (`grep -n -i "assinatura\|subscription\|owner_user_id" OVERVIEW.md`).

- [ ] **Step 2: Verify** — `grep -n "—" DEPLOYMENT.md` shows no new dash in user-facing copy (technical docs may use it; this check is only to keep the commit message clean). Read the section once against spec § 4 "Deploy order".

- [ ] **Step 3: Commit**

```bash
git add DEPLOYMENT.md OVERVIEW.md
git commit -m "docs: runbook da assinatura por organização"
```

---

## Phase 2: close the dual-read window (spec deploy step 4, a separate deploy)

Start only after `cmd/migrate-billing-org -apply` reported nothing to migrate and nothing to review on a dry re-run, and every `USER_` subscription is cancelled in billing.

### Task 17: Remove the dual read, `OwnerOf` and the user snapshot path

**Files:**
- Modify: `api/internal/services/billing.go` (delete `userFallback`, `WithUserFallback`, `userSnapshot`, `userBillingCacheKey`, `OwnerOf`, the fallback branch of `snapshotFor`, `migratingConflict` and its uses; drop the `members` dependency)
- Modify: `api/internal/repositories/account_billing.go` (delete `InheritedFromUser`)
- Modify: `api/internal/services/organizations.go` (delete `SetOwnerUserID`)
- Modify: `api/internal/app/app.go`, `api/tests/integration/setup_test.go:135`, `api/tests/integration/billing_usage_test.go` (`chargingBilling`), `api/cmd/migrate-billing-org/main.go` (constructor arity)
- Modify: `api/tests/integration/org_billing_test.go` (replace the dual-read tests)
- Modify: `DEPLOYMENT.md` (mark step 4 done), `DynamoDB-Tables.md` (`USER_` rows: no longer read)

**Interfaces:**
- Consumes: everything above.
- Produces: `func services.NewBillingService(repo *repositories.AccountBillingRepository, client *billingclient.Client, users *UserService, orgs *OrganizationService, c cache.Backend) *BillingService` (the `members` parameter removed).

- [ ] **Step 1: Replace the dual-read tests with their Phase 2 behaviour** — in `org_billing_test.go`, delete `TestDualReadFallsBackToTheOwnerUntilTheOrganizationHasItsOwn` and `TestACompanyWithNoOrganizationFallsBackToItsOwner`; drop `.WithUserFallback()` from every test; add:

```go
// Phase 2: an organization with only its owner's USER_ snapshot has no plan.
func TestAnOrganizationWithOnlyItsOwnersUserSnapshotHasNoPlan(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	org := "org-p2-" + newCompanyPK(t)
	company := seedCompany(t, org, "owner-p2", "11222333000181", "P2 Ltda")
	if err := repositories.NewAccountBillingRepository(db, cfg).Put(ctx, &repositories.AccountSnapshot{
		UserID: "owner-p2", SubscriptionID: "sub_old", Status: services.StatusActive, Entitled: true,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || got.SubscriptionID != "" || services.GrantsService(got) {
		t.Fatalf("snapshot = %+v (%v), want no plan", got, err)
	}
}

// Review Focus 1, Phase 2: a company with no organization reads as "no plan".
func TestACompanyWithNoOrganizationHasNoPlan(t *testing.T) {
	ctx := context.Background()
	var calls []usageCall
	svc := chargingBilling(t, billingStub(t, &calls))
	company := newCompanyPK(t)
	if err := orgRepo.CreateOrganization(ctx, company, map[string]types.AttributeValue{}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SnapshotForOrg(ctx, company)
	if err != nil || services.GrantsService(got) {
		t.Fatalf("snapshot = %+v (%v)", got, err)
	}
	if p := services.BlockedProblem(got); p.Status != http.StatusPaymentRequired {
		t.Fatalf("blocked = %+v", p)
	}
}
```

Also switch the integration helpers to the Phase 2 constructor first, so the build is the failing test: in `api/tests/integration/setup_test.go:135` and in `chargingBilling` (`billing_usage_test.go`), call `services.NewBillingService(repo, client, nil, orgSvc, cache)` (no `memberSvc`).

- [ ] **Step 2: Run them to verify they fail**

Run: `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -run 'TestAnOrganizationWithOnlyItsOwners|TestACompanyWithNoOrganizationHasNoPlan' -v`
Expected: FAIL — build error `not enough arguments in call to services.NewBillingService` (the constructor still takes `members`).

- [ ] **Step 3: Delete** — in `billing.go`, `snapshotFor` becomes:

```go
// snapshotFor is the organization's snapshot. (The pre-migration fallback to
// the owner's USER_ row was removed when the window closed.)
func (s *BillingService) snapshotFor(ctx context.Context, organizationID, _ string) (*repositories.AccountSnapshot, error) {
	return s.Snapshot(ctx, organizationID)
}
```

Delete `userFallback`, `WithUserFallback`, `userSnapshot`, `userBillingCacheKey`, `OwnerOf`, `migratingConflict` and the two `InheritedFromUser` checks in `Change`/`Cancel`; delete `AccountSnapshot.InheritedFromUser`; delete `OrganizationService.SetOwnerUserID`; remove the `members` field and constructor parameter and fix the four callers listed under Files. Remove `.WithUserFallback()` from `newBillingService`. Update the comment on `SyncBySubscription`'s `USER_` branch: "Pre-migration customers; nothing reads USER_ rows any more."

- [ ] **Step 4: Verify**

Run: `go build ./... && go vet ./... && go test ./...` then `DYNAMODB_ENDPOINT=http://localhost:8123 go test -tags integration -count=1 ./tests/integration/ -v`
Expected: PASS. `grep -rn "OwnerOf\|WithUserFallback\|InheritedFromUser\|SetOwnerUserID\|ownedOrganizations(" api/` returns nothing.

- [ ] **Step 5: Document** — `DEPLOYMENT.md`: step 4 done with the date; `DynamoDB-Tables.md`: `USER_` rows are no longer read and may be deleted by hand once billing's cancellations are confirmed.

- [ ] **Step 6: Commit**

```bash
git add api DEPLOYMENT.md DynamoDB-Tables.md
git commit -m "refactor(dfe): close the dual-read window of the organization subscription"
```

---

## Spec coverage

| Spec | Task |
|---|---|
| O1, § 1 keys (snapshot, usage, guard) | 2, 5 |
| § 1 reservations in the results consumer | 10 |
| § 1 `GetOrCreateCustomer(ORG_)` with billing company | 6 |
| § 1 `Snapshot/Sync/Choose/Change/Cancel/Usage` by organization | 5, 6 |
| § 1 `SnapshotForOrg` via the company's organization | 5 |
| § 1 `companiesUsed` via GSI, `FiscalConfigEnablement` fallback kept | 1, 5 |
| § 1 webhook `ORG_` accepted, `USER_` logged and ignored | 9 |
| O2, § 2 routes, header, owner/admin, 403, read for all | 3, 4, 6 |
| § 2 `GET /organizations/:org_pk/plan` stays | 6 |
| § 2 gate keyed by organization, same semantics | 5 (inside `SnapshotForOrg`; `middleware/subscription.go` unchanged) |
| § 2 UI (title, switching companies, notice, members' message) | 14, 15 |
| O4 users quota out of the UI | 13 |
| O5 `ReserveCompany` at enablement | 7, 11 |
| Amendment: `dfe_companies` level, durable delivery, level on plan selection, initial levels on migration (`-report-levels-all`) | 7, 8, 12 |
| O6 nothing removed over the limit | 7 (`ReserveCompany` refuses only new enablements) |
| `Link` unchecked | 7 (test with linked companies above the limit); `LinkService` untouched |
| CT-e rule | 10 (comment + `TestCTeTableIsMeteredAsCTe`) |
| O3, § 4 migration | 12 |
| § 4 deploy order, dual read and its removal | 5, 16, 17 |
| § 5 tests 1–9 | 5 (1, 2, 6), 6 (3), 9 (4), 12 (5), 11 and 7 (7: refusal; re-enabling is one level change per version, not a second charge), 7 (8), 13 (9) |
