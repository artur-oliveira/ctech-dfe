package main

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func mappingFor(pk, companyID string) *mapping {
	return &mapping{
		LegacyPK: pk, CompanyID: companyID, OrganizationID: "org_1",
		TaxID: "11222333000181", TaxIDKind: "cnpj", LegalName: "ACME LTDA",
	}
}

// A legacy key with no company in ctech-account is not migrated silently. A
// partition nobody can reach is worse than a migration that stopped, and "it
// was not in the mapping" is a fact somebody explains before the flip.
func TestAnUnmappedOrganizationNeedsAHuman(t *testing.T) {
	got := plan([]string{"CNPJ_11222333000181"}, map[string]*mapping{})
	if len(got) != 1 || !got[0].NeedsHuman() {
		t.Fatalf("got %+v, want one decision needing a human", got)
	}
	if !strings.Contains(got[0].Review, "CNPJ_11222333000181") {
		t.Errorf("the review line does not name the organization: %q", got[0].Review)
	}
}

// Each missing field fails on its own, because each breaks something
// different after the flip and a shared message would hide which.
func TestEachMissingFieldIsItsOwnRefusal(t *testing.T) {
	cases := map[string]func(*mapping){
		"no id":              func(m *mapping) { m.CompanyID = "" },
		"no organization_id": func(m *mapping) { m.OrganizationID = "" },
		"no tax_id":          func(m *mapping) { m.TaxID = "" },
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			m := mappingFor("CNPJ_1", "cmp_1")
			break_(m)
			got := plan([]string{"CNPJ_1"}, map[string]*mapping{"CNPJ_1": m})
			if !got[0].NeedsHuman() {
				t.Fatalf("%s was migrated anyway", name)
			}
		})
	}
}

// The happy path, so the refusals above are refusals and not a broken planner.
func TestACompleteMappingIsReady(t *testing.T) {
	got := plan([]string{"CNPJ_1"}, map[string]*mapping{"CNPJ_1": mappingFor("CNPJ_1", "cmp_1")})
	if got[0].NeedsHuman() {
		t.Fatalf("a complete mapping needs a human: %q", got[0].Review)
	}
	if got[0].Mapping.CompanyID != "cmp_1" {
		t.Errorf("company id = %q", got[0].Mapping.CompanyID)
	}
}

// The report is what somebody reads before allowing the flip, so a refusal must
// be impossible to skim past.
func TestTheReportCountsBothOutcomes(t *testing.T) {
	got := report(plan(
		[]string{"CNPJ_1", "CNPJ_2"},
		map[string]*mapping{"CNPJ_1": mappingFor("CNPJ_1", "cmp_1")},
	))
	if !strings.Contains(got, "1 ready, 1 need a human") {
		t.Fatalf("report = %q", got)
	}
	if !strings.Contains(got, "REVIEW") {
		t.Error("the refusal is not marked in the report")
	}
}

// A document table holds one partition per SEFAZ environment. Copying only
// produção would leave homologação empty after the flip, which reads as data
// loss to whoever tests there.
func TestADocumentTableHasAPartitionPerEnvironment(t *testing.T) {
	got := partitionsFor(table{"nfes", kindEnvOrgPK}, "cmp_1")
	if len(got) != 2 {
		t.Fatalf("got %v, want one partition per environment", got)
	}
	for _, want := range []string{"prod#cmp_1", "hom#cmp_1"} {
		var found bool
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q missing from %v", want, got)
		}
	}
}

func TestAConfigTableHasOnePartition(t *testing.T) {
	got := partitionsFor(table{"organization_nfe_configs", kindOrgPK}, "cmp_1")
	if len(got) != 1 || got[0] != "cmp_1" {
		t.Fatalf("got %v, want [cmp_1]", got)
	}
}

// Configuration before documents. A run that dies halfway leaves companies that
// cannot emit — bad and visible — rather than companies that emit with the
// wrong configuration, which is worse and silent.
func TestConfigurationIsCopiedBeforeDocuments(t *testing.T) {
	firstDocument := -1
	lastConfig := -1
	for i, tb := range tables {
		if tb.Kind == kindEnvOrgPK && firstDocument == -1 {
			firstDocument = i
		}
		if tb.Kind == kindOrgPK {
			lastConfig = i
		}
	}
	if firstDocument == -1 || lastConfig == -1 {
		t.Fatal("the table list lost one of the two kinds")
	}
	if lastConfig > firstDocument {
		t.Errorf("a configuration table (%d) is copied after a document table (%d)", lastConfig, firstDocument)
	}
}

// The organization record comes first of all: a partial run should leave
// something a person can look at and recognize.
func TestTheOrganizationRecordIsCopiedFirst(t *testing.T) {
	if tables[0].Name != "organizations" {
		t.Errorf("first table = %q, want organizations", tables[0].Name)
	}
}

// Verification compares bodies, not just counts. A count-only check passes on a
// copy that wrote every row wrong, and the flip is irreversible in practice
// once customers have emitted under the new key.
func TestVerifyComparesBodiesNotJustCounts(t *testing.T) {
	source := map[string]string{"AK1": "a", "AK2": "b"}
	sameCountWrongBody := map[string]string{"AK1": "a", "AK2": "DIFFERENT"}
	got := verify("nfes", "prod#cmp_1", source, sameCountWrongBody)
	if len(got) == 0 {
		t.Fatal("a copy with the right count and the wrong content verified clean")
	}
	if !strings.Contains(got[0].Reason, "AK2") {
		t.Errorf("the mismatch does not name the row: %+v", got)
	}
}

func TestVerifyReportsAMissingRow(t *testing.T) {
	got := verify("nfes", "prod#cmp_1", map[string]string{"AK1": "a", "AK2": "b"}, map[string]string{"AK1": "a"})
	var sawMissing bool
	for _, m := range got {
		if strings.Contains(m.Reason, "missing row AK2") {
			sawMissing = true
		}
	}
	if !sawMissing {
		t.Fatalf("a missing row was not reported: %+v", got)
	}
}

// A row that exists only in the copy means somebody wrote under the new key
// while the copy was running — which is what the write freeze exists to
// prevent. It has to be visible, or a broken freeze passes verification.
func TestVerifyNoticesAWriteDuringTheCopy(t *testing.T) {
	got := verify("nfes", "prod#cmp_1", map[string]string{"AK1": "a"}, map[string]string{"AK1": "a", "AK2": "new"})
	var sawExtra bool
	for _, m := range got {
		if strings.Contains(m.Reason, "only in the copy") {
			sawExtra = true
		}
	}
	if !sawExtra {
		t.Fatalf("a row written during the copy was not reported: %+v", got)
	}
}

func TestAnIdenticalCopyVerifiesClean(t *testing.T) {
	rows := map[string]string{"AK1": "a", "AK2": "b"}
	if got := verify("nfes", "prod#cmp_1", rows, map[string]string{"AK1": "a", "AK2": "b"}); len(got) != 0 {
		t.Fatalf("an identical copy reported %+v", got)
	}
}

// An empty partition on both sides is clean, not an error. Most organizations
// have no CT-e at all.
func TestAnEmptyPartitionVerifiesClean(t *testing.T) {
	if got := verify("ctes", "prod#cmp_1", map[string]string{}, map[string]string{}); len(got) != 0 {
		t.Fatalf("two empty partitions reported %+v", got)
	}
}

// The identity is written onto the company record and nowhere else. Spreading
// it across tables would give the flip more than one source of truth for the
// issuer document.
func TestTheIdentityGoesOnTheCompanyRecord(t *testing.T) {
	attrs := identityAttrs(mappingFor("CNPJ_1", "cmp_1"))
	for _, want := range []string{"organization_id", "tax_id", "tax_id_kind", "legal_name"} {
		if _, ok := attrs[want]; !ok {
			t.Errorf("%q missing from the identity written at migration time", want)
		}
	}
	if len(attrs) != 4 {
		t.Errorf("the identity carries %d attributes; every extra one is a second copy to keep current", len(attrs))
	}
}

// Exactly one verb. Two would be ambiguous about ordering, and defaulting to
// apply is how a rehearsal becomes a migration.
func TestExactlyOneVerb(t *testing.T) {
	if countTrue(false, false, false) == 1 {
		t.Error("no verb passed as one")
	}
	if countTrue(true, false, false) != 1 {
		t.Error("one verb did not count as one")
	}
	if countTrue(true, true, false) == 1 {
		t.Error("two verbs counted as one")
	}
}

// pk is the one attribute the copy is meant to change, so it must not make two
// otherwise-identical rows compare as different — otherwise every row fails
// verification, which is the same as verification not existing.
func TestTheComparisonIgnoresThePartitionKey(t *testing.T) {
	source := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "CNPJ_11222333000181"},
		"sk": &types.AttributeValueMemberS{Value: "AK1"},
	}
	copied := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "cmp_1"},
		"sk": &types.AttributeValueMemberS{Value: "AK1"},
	}
	a, err := canonicalBody(source, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalBody(copied, false)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("the same row under two keys compared as different:\n  %s\n  %s", a, b)
	}
}

// Any other attribute differing must be caught. The comparison exists to find
// exactly that.
func TestTheComparisonNoticesEveryOtherAttribute(t *testing.T) {
	a, _ := canonicalBody(map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "x"}, "status": &types.AttributeValueMemberS{Value: "authorized"},
	}, false)
	b, _ := canonicalBody(map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "y"}, "status": &types.AttributeValueMemberS{Value: "cancelled"},
	}, false)
	if a == b {
		t.Fatal("two rows with different content compared as identical")
	}
}

// The company record is the one table whose copy is deliberately not identical:
// the migration adds the platform identity. Comparing it would report every
// company as a mismatch and make the verification useless exactly where it
// matters most.
func TestTheIdentityIsExcludedFromTheCompanyRecordComparison(t *testing.T) {
	source := map[string]types.AttributeValue{
		"pk":   &types.AttributeValueMemberS{Value: "CNPJ_11222333000181"},
		"name": &types.AttributeValueMemberS{Value: "ACME"},
	}
	enriched := map[string]types.AttributeValue{
		"pk":              &types.AttributeValueMemberS{Value: "cmp_1"},
		"name":            &types.AttributeValueMemberS{Value: "ACME"},
		"organization_id": &types.AttributeValueMemberS{Value: "org_1"},
		"tax_id":          &types.AttributeValueMemberS{Value: "11222333000181"},
		"tax_id_kind":     &types.AttributeValueMemberS{Value: "cnpj"},
		"legal_name":      &types.AttributeValueMemberS{Value: "ACME"},
	}
	a, err := canonicalBody(source, true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := canonicalBody(enriched, true)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("the enriched company record compared as different:\n  %s\n  %s", a, b)
	}
}

// And the exclusion must not hide a real change: a row whose actual content
// differs still fails, identity or not.
func TestTheExclusionStillCatchesARealChange(t *testing.T) {
	a, _ := canonicalBody(map[string]types.AttributeValue{
		"name": &types.AttributeValueMemberS{Value: "ACME"},
	}, true)
	b, _ := canonicalBody(map[string]types.AttributeValue{
		"name":   &types.AttributeValueMemberS{Value: "OUTRA"},
		"tax_id": &types.AttributeValueMemberS{Value: "11222333000181"},
	}, true)
	if a == b {
		t.Fatal("a changed name passed because the identity was excluded")
	}
}

// Every other table keeps the strict comparison. Excluding the identity there
// would hide a real difference in a document.
func TestOtherTablesKeepTheStrictComparison(t *testing.T) {
	a, _ := canonicalBody(map[string]types.AttributeValue{
		"tax_id": &types.AttributeValueMemberS{Value: "11222333000181"},
	}, false)
	b, _ := canonicalBody(map[string]types.AttributeValue{}, false)
	if a == b {
		t.Fatal("tax_id was excluded on a table that is not the company record")
	}
}

// PermChecker resolves a membership on every request, so a flip that left this
// table behind would answer 403 to everybody. It was excluded once — the spec
// separates identity from membership — and the two share a partition key.
func TestMembershipIsRekeyedToo(t *testing.T) {
	for _, want := range []string{"organization_users", "organization_invitations"} {
		var found bool
		for _, tb := range tables {
			if tb.Name == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is missing; the flip would refuse every request", want)
		}
	}
}

// And it comes before the documents, like every other thing a company needs in
// order to be usable at all.
func TestMembershipIsCopiedBeforeDocuments(t *testing.T) {
	var membership, firstDocument = -1, -1
	for i, tb := range tables {
		if tb.Name == "organization_users" {
			membership = i
		}
		if tb.Kind == kindEnvOrgPK && firstDocument == -1 {
			firstDocument = i
		}
	}
	if membership == -1 || membership > firstDocument {
		t.Errorf("membership at %d, first document at %d", membership, firstDocument)
	}
}

// Every table this repo partitions by the organization key must be in the list.
// Two were missed: organization_users, which would have made the flip answer
// 403 to everybody, and audit_logs, whose name does not start with
// organization_ and so read as somebody else's table twice.
//
// The list is explicit rather than derived, which is right — a derived list
// quietly includes whatever it finds — but explicit means a human maintains it,
// and this is what fails when one forgets.
func TestTheKnownOrganizationPartitionedTablesAreAllListed(t *testing.T) {
	// Names verified against production, not remembered.
	mustBeListed := []string{
		"organizations", "organization_users", "organization_invitations",
		"organization_certificates", "audit_logs",
		"organization_nfe_configs", "organization_nfce_configs",
		"organization_cte_configs", "organization_mdfe_configs", "organization_nfse_configs",
		"nfes", "nfces", "ctes", "mdfes", "nfses",
	}
	listed := map[string]bool{}
	for _, tb := range tables {
		listed[tb.Name] = true
	}
	for _, want := range mustBeListed {
		if !listed[want] {
			t.Errorf("%q is partitioned by the organization key and is not in the re-key list", want)
		}
	}
}

// And the ones that are NOT tenant-scoped must stay out. Copying them under a
// company key would duplicate global data per company.
func TestNonTenantTablesAreNotListed(t *testing.T) {
	// Verified in production: users and account_billing are keyed by the
	// account, roles is a global catalogue, worker_outbox by document, and
	// serie_claims deliberately by tax id.
	for _, name := range []string{"users", "roles", "account_billing", "worker_outbox", "serie_claims"} {
		for _, tb := range tables {
			if tb.Name == name {
				t.Errorf("%q is not partitioned by the organization key and must not be copied", name)
			}
		}
	}
}
