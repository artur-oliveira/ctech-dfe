package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
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
	var out bytes.Buffer
	if err := (&DataExporter{repo: repo, now: time.Now}).Write(context.Background(), "alice", &out); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
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
	for name, body := range files {
		if strings.HasSuffix(name, ".json") && !json.Valid([]byte(body)) {
			t.Errorf("%s is not valid JSON: %s", name, body)
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
	var out bytes.Buffer
	if err := (&DataExporter{repo: &fakeExportRepo{}, now: time.Now}).Write(context.Background(), "nobody", &out); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
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

// pagedRepo answers the audit index two rows a page, five rows in all.
type pagedRepo struct{ fakeExportRepo }

func (*pagedRepo) QueryIndex(_ context.Context, table, _, _, _ string, start map[string]types.AttributeValue) (repositories.Page, error) {
	if table != "audit_logs" {
		return repositories.Page{}, nil
	}
	from := 0
	if start != nil {
		from, _ = strconv.Atoi(start["i"].(*types.AttributeValueMemberS).Value)
	}
	var page repositories.Page
	for i := from; i < min(from+2, 5); i++ {
		page.Items = append(page.Items, map[string]types.AttributeValue{"pk": sav("c"), "sk": sav(strconv.Itoa(i)), "action": sav("emit")})
	}
	if from+2 < 5 {
		page.Next = map[string]types.AttributeValue{"i": sav(strconv.Itoa(from + 2))}
	}
	return page, nil
}

// A list longer than the cap is cut mid-page, says so in its own file and in the
// README, and is still valid JSON; under the cap every page is followed.
func TestDataExporterPagesAnIndexAndCutsItAtTheCap(t *testing.T) {
	for _, tc := range []struct {
		max, items int
		cut        bool
	}{{0, 5, false}, {3, 3, true}} {
		var out bytes.Buffer
		if err := (&DataExporter{repo: &pagedRepo{}, now: time.Now, maxItems: tc.max}).Write(context.Background(), "alice", &out); err != nil {
			t.Fatal(err)
		}
		zr, _ := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
		files := map[string]string{}
		for _, f := range zr.File {
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			files[f.Name] = string(b)
		}
		var audit struct {
			Items     []map[string]any `json:"items"`
			Truncated bool             `json:"truncated"`
		}
		if err := json.Unmarshal([]byte(files["audit-logs.json"]), &audit); err != nil {
			t.Fatalf("max %d: %v\n%s", tc.max, err, files["audit-logs.json"])
		}
		if len(audit.Items) != tc.items || audit.Truncated != tc.cut {
			t.Fatalf("max %d: %d items, truncated %v", tc.max, len(audit.Items), audit.Truncated)
		}
		if got := strings.Contains(files["README.txt"], "cut at the size limit: audit-logs.json"); got != tc.cut {
			t.Errorf("max %d: README names the cut file = %v", tc.max, got)
		}
	}
}
