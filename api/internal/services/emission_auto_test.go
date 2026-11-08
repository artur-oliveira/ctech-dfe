package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/go-dfe/autocontingency"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

type fakeSwitcher struct {
	calls    int
	target   string
	switched bool
	err      error
}

func (f *fakeSwitcher) SwitchAuto(_ context.Context, _, target string, _ time.Time, _ string) (bool, error) {
	f.calls++
	f.target = target
	return f.switched, f.err
}

func autoCfg(emission string, auto bool, typ string, br autocontingency.Breaker) map[string]types.AttributeValue {
	c := map[string]types.AttributeValue{
		repositories.AttrAutoContingency:     &types.AttributeValueMemberBOOL{Value: auto},
		repositories.AttrAutoContingencyType: &types.AttributeValueMemberS{Value: typ},
		repositories.AttrAutoBreaker:         &types.AttributeValueMemberS{Value: autocontingency.Encode(br)},
	}
	if emission != "" {
		c[repositories.AttrEmissionType] = &types.AttributeValueMemberS{Value: emission}
	}
	return c
}

var openBreaker = autocontingency.Breaker{State: autocontingency.Open, OpenedAt: time.Now().Add(-time.Hour)}

func TestPrecheckAutoContingency(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		enabled  bool
		model    string
		uf       string
		cfg      map[string]types.AttributeValue
		sw       *fakeSwitcher
		want     string // emission_type of the returned row
		wantCall int
	}{
		{"switch off (the shipped constant): never touches the row", false, ModelNFe, "SP", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{switched: true}, "normal", 0},
		{"open breaker, flag on: switches to the chosen type", true, ModelNFe, "SP", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{switched: true}, "svc", 1},
		{"offline for the NFC-e", true, ModelNFCe, "SP", autoCfg("normal", true, "offline", openBreaker), &fakeSwitcher{switched: true}, "offline", 1},
		{"closed breaker: no switch", true, ModelNFe, "SP", autoCfg("", true, "svc", autocontingency.Breaker{State: autocontingency.Closed}), &fakeSwitcher{switched: true}, "normal", 0},
		{"flag off: no switch", true, ModelNFe, "SP", autoCfg("", false, "svc", openBreaker), &fakeSwitcher{switched: true}, "normal", 0},
		{"already in contingency (manual or auto): left alone", true, ModelNFe, "SP", autoCfg("epec", true, "svc", openBreaker), &fakeSwitcher{switched: true}, "epec", 0},
		{"type not valid for the document", true, ModelMDFe, "", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{switched: true}, "normal", 0},
		{"SVC target without a designated SVC for the UF", true, ModelNFe, "XX", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{switched: true}, "normal", 0},
		{"lost the race (condition failed)", true, ModelNFe, "SP", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{switched: false}, "normal", 1},
		{"store error: emits normally", true, ModelNFe, "SP", autoCfg("", true, "svc", openBreaker), &fakeSwitcher{err: errors.New("boom")}, "normal", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func(old bool) { autoSwitchEnabled = old }(autoSwitchEnabled)
			autoSwitchEnabled = c.enabled
			got := PrecheckAutoContingency(context.Background(), c.sw, c.model, "ORG", c.uf, c.cfg, now)
			if e := repositories.EmissionTypeOf(got); e != c.want {
				t.Errorf("emission = %q, want %q", e, c.want)
			}
			if c.sw.calls != c.wantCall {
				t.Errorf("SwitchAuto calls = %d, want %d", c.sw.calls, c.wantCall)
			}
			if c.want != "normal" && c.wantCall == 1 && c.sw.switched {
				if attrStrAV(got, repositories.AttrContingencyReason) != autocontingency.AutoReason || attrStrAV(got, repositories.AttrContingencySince) == "" {
					t.Error("switched row lacks reason/since")
				}
				if n := len([]rune(autocontingency.AutoReason)); n < EmissionReasonMin || n > EmissionReasonMax {
					t.Errorf("AutoReason length %d outside xJust bounds", n)
				}
			}
		})
	}
}

type fakeTx struct {
	items []types.TransactWriteItem
	err   error
}

func (f *fakeTx) TransactWrite(_ context.Context, it []types.TransactWriteItem) error {
	f.items = it
	return f.err
}

func TestLinkReissueMarksOriginalAndReferencesIt(t *testing.T) {
	tx := &fakeTx{}
	if err := LinkReissue(context.Background(), tx, "t", "PK", "ORIG", "NEW", time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(tx.items) != 2 {
		t.Fatalf("items = %d", len(tx.items))
	}
	o, n := tx.items[0].Update, tx.items[1].Update
	if o.Key["sk"].(*types.AttributeValueMemberS).Value != "ORIG" || n.Key["sk"].(*types.AttributeValueMemberS).Value != "NEW" {
		t.Error("wrong keys")
	}
	// idempotent (same pair again) and never over an authorized or cancelled original
	for _, frag := range []string{"attribute_not_exists(#to) OR #to = :new", "#st <> :authorized", "#st <> :cancelled"} {
		if !strings.Contains(*o.ConditionExpression, frag) {
			t.Errorf("original condition lacks %q: %s", frag, *o.ConditionExpression)
		}
	}
	if !strings.Contains(*n.ConditionExpression, "#from = :orig") {
		t.Errorf("new condition not idempotent: %s", *n.ConditionExpression)
	}
	if o.ExpressionAttributeValues[":failed"].(*types.AttributeValueMemberS).Value != "failed" {
		t.Error("original must become failed (terminal for the worker)")
	}
}
