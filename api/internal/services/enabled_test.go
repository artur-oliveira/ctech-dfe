package services

import (
	"context"
	"errors"
	"testing"
)

// fakeEnablement stands in for the fiscal config tables: one row per company
// per document type, written when somebody sets a série.
type fakeEnablement struct {
	// configured maps a company key to the document types it has a fiscal
	// config for.
	configured map[string][]string
	err        error
	calls      int
}

func (f *fakeEnablement) ConfiguredDocTypes(_ context.Context, orgPK string) ([]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.configured[orgPK], nil
}

// The customer this whole model exists for: forty CNPJs, one issuing. Counting
// what they OWN refuses them at forty while they use one — and ADR 0021 says
// the quota applies to what is ENABLED.
func TestTheQuotaCountsWhatIsEnabledNotWhatIsOwned(t *testing.T) {
	owned := []string{"cmp_1", "cmp_2", "cmp_3"}
	src := &fakeEnablement{configured: map[string][]string{"cmp_1": {"nfe"}}}

	got, err := countEnabled(context.Background(), src, owned)
	if err != nil {
		t.Fatalf("countEnabled: %v", err)
	}
	if got != 1 {
		t.Fatalf("counted %d of %d owned, want 1 enabled", got, len(owned))
	}
}

// A company registered and never configured emits nothing and costs nothing.
// That is what lets one organization hold forty CNPJs and pay for one.
func TestAnUnconfiguredCompanyCostsNothing(t *testing.T) {
	got, err := countEnabled(context.Background(),
		&fakeEnablement{configured: map[string][]string{}}, []string{"cmp_1", "cmp_2"})
	if err != nil {
		t.Fatalf("countEnabled: %v", err)
	}
	if got != 0 {
		t.Fatalf("counted %d, want 0", got)
	}
}

// Enabling counts once per company, not once per document type: a company with
// NF-e and NFC-e configured is one company, and counting configurations would
// bill somebody twice for issuing two kinds of document.
func TestEnablementCountsCompaniesNotConfigurations(t *testing.T) {
	src := &fakeEnablement{configured: map[string][]string{
		"cmp_1": {"nfe", "nfce", "cte", "mdfe"},
	}}
	got, err := countEnabled(context.Background(), src, []string{"cmp_1"})
	if err != nil {
		t.Fatalf("countEnabled: %v", err)
	}
	if got != 1 {
		t.Fatalf("counted %d for one company with four configs, want 1", got)
	}
}

// An unreadable enablement is an error, not a zero. Counting zero would let
// somebody past a quota during an outage, and this is the check that decides
// whether they may add another company.
func TestAnUnreadableEnablementIsAnError(t *testing.T) {
	_, err := countEnabled(context.Background(),
		&fakeEnablement{err: errors.New("timeout")}, []string{"cmp_1"})
	if err == nil {
		t.Fatal("an outage counted as zero enabled companies, which lets somebody past the quota")
	}
}

// No companies at all is zero and no error — a fresh account, which is the
// state every account starts in.
func TestNoCompaniesIsZero(t *testing.T) {
	got, err := countEnabled(context.Background(), &fakeEnablement{}, nil)
	if err != nil || got != 0 {
		t.Fatalf("got %d %v", got, err)
	}
}

// Without an enablement source the quota falls back to counting owned
// companies. That is the older and stricter answer, and stricter is the right
// direction for a fallback: it can annoy somebody, and it cannot let them past
// a limit.
func TestTheFallbackCountsOwnedAndIsStricter(t *testing.T) {
	owned := []string{"cmp_1", "cmp_2", "cmp_3"}
	src := &fakeEnablement{configured: map[string][]string{"cmp_1": {"nfe"}}}

	enabled, err := countEnabled(context.Background(), src, owned)
	if err != nil {
		t.Fatalf("countEnabled: %v", err)
	}
	if enabled >= len(owned) {
		t.Fatalf("enabled %d is not fewer than owned %d; the fallback would be looser, not stricter",
			enabled, len(owned))
	}
}

// A nil reader inside the enablement is skipped rather than counted. A document
// type this deployment does not wire is a document type nobody can configure,
// so it cannot be what makes a company enabled.
func TestANilReaderIsSkipped(t *testing.T) {
	e := NewFiscalConfigEnablement(nil, nil, nil, nil, nil)
	got, err := e.ConfiguredDocTypes(context.Background(), "cmp_1")
	if err != nil {
		t.Fatalf("ConfiguredDocTypes: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}
