package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

type fakeEmissionRepo struct {
	row     map[string]types.AttributeValue
	gotTyp  string
	gotAt   time.Time
	gotAuto string
}

func (f *fakeEmissionRepo) Get(context.Context, string) (map[string]types.AttributeValue, error) {
	return f.row, nil
}
func (f *fakeEmissionRepo) BuildUpsertTxItem(string, map[string]types.AttributeValue, map[string]types.AttributeValue) (types.TransactWriteItem, map[string]types.AttributeValue, error) {
	return types.TransactWriteItem{}, nil, nil
}
func (f *fakeEmissionRepo) TransactWrite(context.Context, []types.TransactWriteItem) error {
	return nil
}
func (f *fakeEmissionRepo) SetEmission(_ context.Context, _, typ string, _ bool, autoType string, since time.Time, _ string) error {
	f.gotTyp, f.gotAt, f.gotAuto = typ, since, autoType
	return nil
}

func emissionSvc(model string, row map[string]types.AttributeValue) (*fiscalConfigService, *fakeEmissionRepo) {
	r := &fakeEmissionRepo{row: row}
	return &fiscalConfigService{repo: r, emissionModel: model, notFoundMsg: "x"}, r
}

func TestSetEmissionValidatesTypePerDocument(t *testing.T) {
	cases := map[string]struct {
		typ string
		ok  bool
	}{
		ModelNFe + "/svc": {repositories.EmissionSVC, true}, ModelNFe + "/offline": {repositories.EmissionOffline, false},
		ModelNFCe + "/offline": {repositories.EmissionOffline, true}, ModelNFCe + "/svc": {repositories.EmissionSVC, false},
		ModelMDFe + "/offline": {repositories.EmissionOffline, true}, ModelMDFe + "/epec": {repositories.EmissionEPEC, false},
		ModelCTe + "/svc": {repositories.EmissionSVC, true}, ModelCTe + "/epec": {repositories.EmissionEPEC, false},
	}
	for k, c := range cases {
		s, _ := emissionSvc(strings.SplitN(k, "/", 2)[0], map[string]types.AttributeValue{})
		_, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: c.typ, Reason: "autorizador indisponivel"})
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v want ok=%v", k, err, c.ok)
		}
	}
}

func TestSetEmissionReasonAndSince(t *testing.T) {
	s, r := emissionSvc(ModelNFe, map[string]types.AttributeValue{})
	if _, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: repositories.EmissionSVC, Reason: "curta"}); err == nil {
		t.Fatal("short reason must be rejected")
	}
	// back to normal needs no reason
	if _, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: repositories.EmissionNormal}); err != nil || r.gotTyp != repositories.EmissionNormal {
		t.Fatalf("normal: %v %q", err, r.gotTyp)
	}
	// switching inside contingency keeps the original timestamp and reason
	const since = "2026-11-08T10:00:00Z"
	s, r = emissionSvc(ModelCTe, map[string]types.AttributeValue{
		repositories.AttrEmissionType:      &types.AttributeValueMemberS{Value: repositories.EmissionSVC},
		repositories.AttrContingencySince:  &types.AttributeValueMemberS{Value: since},
		repositories.AttrContingencyReason: &types.AttributeValueMemberS{Value: "autorizador indisponivel"},
	})
	if _, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: repositories.EmissionOffline}); err != nil {
		t.Fatal(err)
	}
	if r.gotAt.UTC().Format(time.RFC3339) != since {
		t.Fatalf("since moved: %v", r.gotAt)
	}
}

func TestGetNormalizesLegacyRow(t *testing.T) {
	s, _ := emissionSvc(ModelNFe, map[string]types.AttributeValue{repositories.AttrContingencyMode: &types.AttributeValueMemberS{Value: "svc"}})
	item, _ := s.Get(context.Background(), "O")
	if attrStrAV(item, repositories.AttrEmissionType) != repositories.EmissionSVC {
		t.Fatalf("%v", item)
	}
	if _, ok := item[repositories.AttrContingencyMode]; ok {
		t.Fatal("legacy attribute leaked")
	}
	if _, ok := item[AttrEmissionOptions]; !ok {
		t.Fatal("options missing")
	}
}

// auto_contingency needs the owner's choice of contingency, validated against the document's options.
func TestSetEmissionAutoContingencyType(t *testing.T) {
	cases := []struct {
		model, auto string
		ok          bool
		code        string
	}{
		{ModelNFe, repositories.EmissionSVC, true, ""},
		{ModelNFe, repositories.EmissionEPEC, true, ""},
		{ModelNFe, repositories.EmissionOffline, false, CodeAutoContingencyTypeInvalid},
		{ModelNFe, repositories.EmissionNormal, false, CodeAutoContingencyTypeInvalid},
		{ModelNFe, "", false, CodeAutoContingencyTypeRequired},
		{ModelNFCe, repositories.EmissionOffline, true, ""},
		{ModelNFCe, repositories.EmissionEPEC, false, CodeAutoContingencyTypeInvalid},
		{ModelMDFe, repositories.EmissionSVC, false, CodeAutoContingencyTypeInvalid},
		{ModelCTe, repositories.EmissionSVC, true, ""},
	}
	for _, c := range cases {
		s, r := emissionSvc(c.model, map[string]types.AttributeValue{})
		_, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: repositories.EmissionNormal, Auto: true, AutoType: c.auto})
		if (err == nil) != c.ok {
			t.Errorf("%s/%s: err=%v", c.model, c.auto, err)
		}
		if !c.ok && err != nil {
			if b, _ := json.Marshal(err); !strings.Contains(string(b), c.code) {
				t.Errorf("%s/%s: want code %s in %s", c.model, c.auto, c.code, b)
			}
		}
		if c.ok && r.gotAuto != c.auto {
			t.Errorf("%s/%s: stored %q", c.model, c.auto, r.gotAuto)
		}
	}
	// without auto the type is neither required nor stored
	s, r := emissionSvc(ModelNFe, map[string]types.AttributeValue{})
	if _, err := s.SetEmission(context.Background(), "O", EmissionUpdate{Type: repositories.EmissionNormal, AutoType: "svc"}); err != nil || r.gotAuto != "svc" {
		t.Errorf("auto off passes the value through; the repository drops it: err=%v got=%q", err, r.gotAuto)
	}
}
