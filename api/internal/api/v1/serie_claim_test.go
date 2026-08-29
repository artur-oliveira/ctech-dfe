package v1

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func avNum(n string) types.AttributeValue { return &types.AttributeValueMemberN{Value: n} }

// The série arrives as a DynamoDB number. Anything unreadable is zero — the
// same value an unset série carries, which claims nothing — because the
// alternative is refusing a save over a field the claim does not need.
func TestAvIntReadsASerieAndFailsToZero(t *testing.T) {
	item := map[string]types.AttributeValue{
		"prod_current_serie": avNum("3"),
		"hom_current_serie":  &types.AttributeValueMemberS{Value: "3"},
	}
	if got := avInt(item, "prod_current_serie"); got != 3 {
		t.Errorf("prod = %d, want 3", got)
	}
	// A string where a number belongs: unreadable, so zero.
	if got := avInt(item, "hom_current_serie"); got != 0 {
		t.Errorf("hom = %d, want 0 for a non-numeric attribute", got)
	}
	if got := avInt(item, "absent"); got != 0 {
		t.Errorf("absent = %d, want 0", got)
	}
	if got := avInt(map[string]types.AttributeValue{"prod_current_serie": avNum("não")}, "prod_current_serie"); got != 0 {
		t.Errorf("unparseable = %d, want 0", got)
	}
}

// fakeGuard records what was claimed and released, and can refuse.
type fakeGuard struct {
	claimed  []string
	released []string
	refuse   bool
}

func (f *fakeGuard) Claim(_ context.Context, taxID, modelo, ambiente string, serie int, _ string) error {
	if f.refuse {
		return repositories.ErrSerieTaken
	}
	f.claimed = append(f.claimed, key(taxID, modelo, ambiente, serie))
	return nil
}

func (f *fakeGuard) Release(_ context.Context, taxID, modelo, ambiente string, serie int, _ string) error {
	f.released = append(f.released, key(taxID, modelo, ambiente, serie))
	return nil
}

func key(taxID, modelo, ambiente string, serie int) string {
	return taxID + "/" + modelo + "/" + ambiente + "/" + strconv.Itoa(serie)
}

// A variant that mounts with no modelo claims nothing. NFS-e is municipal and
// its numbering is not keyed (CNPJ, modelo, série, número, ambiente); claiming
// under an empty modelo would have every NFS-e company collide with every other
// one on the same CNPJ.
func TestAVariantWithNoModeloClaimsNothing(t *testing.T) {
	g := &fakeGuard{}
	deps := fiscalConfigDeps{claims: g, modelo: ""}
	if _, err := claimSeriesFor(context.Background(), "cmp_1", "11222333000181", deps, 1, 2, nil); err != nil {
		t.Fatalf("claimSeriesFor: %v", err)
	}
	if len(g.claimed) != 0 {
		t.Fatalf("claimed %v with no modelo", g.claimed)
	}
}

// An issuer with no document claims nothing either. Refusing would block every
// existing customer's configuration save before the migration runs, and the
// rule protects a CNPJ shared between organizations — which cannot happen while
// the key still IS the CNPJ.
func TestNoDocumentClaimsNothing(t *testing.T) {
	g := &fakeGuard{}
	deps := fiscalConfigDeps{claims: g, modelo: "55"}
	if _, err := claimSeriesFor(context.Background(), "cmp_1", "", deps, 1, 2, nil); err != nil {
		t.Fatalf("claimSeriesFor: %v", err)
	}
	if len(g.claimed) != 0 {
		t.Fatalf("claimed %v with no document", g.claimed)
	}
}

// Both environments, and nothing released on a first save.
func TestAFirstSaveClaimsBothEnvironments(t *testing.T) {
	g := &fakeGuard{}
	deps := fiscalConfigDeps{claims: g, modelo: "55"}
	release, err := claimSeriesFor(context.Background(), "cmp_1", "11222333000181", deps, 1, 2, nil)
	if err != nil {
		t.Fatalf("claimSeriesFor: %v", err)
	}
	release()
	if len(g.claimed) != 2 {
		t.Fatalf("claimed %v, want both environments", g.claimed)
	}
	if len(g.released) != 0 {
		t.Fatalf("released %v on a first save", g.released)
	}
}

// A refused claim returns a conflict and releases nothing. Releasing on the way
// out would give up a série this company still emits under.
func TestARefusedClaimChangesNothing(t *testing.T) {
	g := &fakeGuard{refuse: true}
	deps := fiscalConfigDeps{claims: g, modelo: "55"}
	previous := map[string]types.AttributeValue{"prod_current_serie": avNum("9")}
	_, err := claimSeriesFor(context.Background(), "cmp_1", "11222333000181", deps, 1, 2, previous)
	if err == nil {
		t.Fatal("a taken série was accepted")
	}
	if len(g.released) != 0 {
		t.Fatalf("released %v after a refusal", g.released)
	}
	// The message must never name the holder: that would disclose that somebody
	// else carries this CNPJ.
	msg := err.Error()
	for _, leak := range []string{"cmp_", "CNPJ_", "organização"} {
		if strings.Contains(msg, leak) {
			t.Errorf("the refusal leaks %q: %q", leak, msg)
		}
	}
}

// Only the série that changed is released — and only after the write. A série
// kept across a save must not be released and re-claimed, which would leave a
// window for another company on the same CNPJ to take it.
func TestOnlyTheChangedSerieIsReleased(t *testing.T) {
	g := &fakeGuard{}
	deps := fiscalConfigDeps{claims: g, modelo: "55"}
	previous := map[string]types.AttributeValue{
		"prod_current_serie": avNum("9"),
		"hom_current_serie":  avNum("2"),
	}
	release, err := claimSeriesFor(context.Background(), "cmp_1", "11222333000181", deps, 1, 2, previous)
	if err != nil {
		t.Fatalf("claimSeriesFor: %v", err)
	}
	if len(g.released) != 0 {
		t.Fatal("a série was released before the write succeeded")
	}
	release()
	if len(g.released) != 1 || !strings.HasSuffix(g.released[0], "/1/9") {
		t.Fatalf("released %v, want only produção série 9", g.released)
	}
}
