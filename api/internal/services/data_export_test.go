package services

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func sav(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

type fakeExportRepo struct {
	items map[string]map[string]types.AttributeValue // table|pk
	index map[string][]map[string]types.AttributeValue
}

func (f *fakeExportRepo) GetItem(_ context.Context, table string, key map[string]types.AttributeValue) (map[string]types.AttributeValue, error) {
	return f.items[table+"|"+key["pk"].(*types.AttributeValueMemberS).Value], nil
}

func (f *fakeExportRepo) QueryIndex(_ context.Context, table, index, _, value string, _ map[string]types.AttributeValue) (repositories.Page, error) {
	return repositories.Page{Items: f.index[table+"|"+index+"|"+value]}, nil
}

func TestDataExporterPart(t *testing.T) {
	repo := &fakeExportRepo{
		items: map[string]map[string]types.AttributeValue{
			"users|USER_alice":           {"pk": sav("USER_alice"), "email": sav("alice@example.com"), "cpf_hash": sav("SECRET")},
			"account_billing|USER_alice": {"pk": sav("USER_alice"), "plan": sav("pro")},
			"users|USER_bob":             {"pk": sav("USER_bob"), "email": sav("bob@example.com")},
		},
		index: map[string][]map[string]types.AttributeValue{
			"organization_users|user-index|USER_alice": {{"pk": sav("company-1"), "sk": sav("USER_alice"), "role": sav("admin")}},
			"audit_logs|user-id-index|alice": {{"pk": sav("company-1"), "sk": sav("a1"), "user_id": sav("alice"), "action": sav("emit"),
				"modifications": &types.AttributeValueMemberL{Value: []types.AttributeValue{sav("company secret payload")}}}},
		},
	}
	data, err := (&DataExporter{repo: repo, now: time.Now}).Part(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	files := map[string]string{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		files[f.Name] = string(b)
	}
	for _, want := range []string{"user.json", "billing.json", "memberships.json", "audit-logs.json", "README.txt"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	all := strings.Join([]string{files["user.json"], files["billing.json"], files["memberships.json"], files["audit-logs.json"]}, "")
	if strings.Contains(all, "bob@example.com") || strings.Contains(all, "SECRET") || strings.Contains(all, "company secret payload") {
		t.Errorf("another person's data, a secret or a company payload leaked: %s", all)
	}
	if !strings.Contains(files["user.json"], "alice@example.com") || !strings.Contains(files["memberships.json"], "admin") || !strings.Contains(files["audit-logs.json"], "emit") {
		t.Errorf("own data missing: %v", files)
	}
}

func TestDataExporterForAPersonWithNothing(t *testing.T) {
	data, err := (&DataExporter{repo: &fakeExportRepo{}, now: time.Now}).Part(context.Background(), "nobody")
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if len(zr.File) != 1 || zr.File[0].Name != "README.txt" {
		t.Fatalf("files=%d", len(zr.File))
	}
}

// Every table of the erasure catalog has a decided export treatment, and the
// exported ones are exactly the person-scoped ones.
func TestEveryTableIsClassifiedForTheExport(t *testing.T) {
	for _, tbl := range ErasureCatalog() {
		v := exportClassification(tbl.Name)
		if !strings.HasPrefix(v, "exported") && !strings.HasPrefix(v, "excluded: ") {
			t.Errorf("%s: classification %q", tbl.Name, v)
		}
		person := tbl.Scope == ScopePerson || tbl.Scope == ScopeAccountBilling || tbl.Name == "organization_users" || tbl.Name == "audit_logs"
		if person != strings.HasPrefix(v, "exported") {
			t.Errorf("%s (scope %d): classified %q; a person-scoped table must be exported, a company table excluded. Decide, and add a source to DataExporter.Part", tbl.Name, tbl.Scope, v)
		}
	}
}
