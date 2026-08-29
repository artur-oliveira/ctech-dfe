package services

import "testing"

// A fiscal config declares a série per environment, and both can be emitted
// under. Claiming only the active one leaves the other free for somebody else
// on the same CNPJ — and homologação collisions are how a company discovers the
// problem in production.
func TestBothEnvironmentsAreClaimed(t *testing.T) {
	got := SerieClaimsFor("55", 1, 2)
	if len(got) != 2 {
		t.Fatalf("got %+v, want one claim per environment", got)
	}
	var sawProd, sawHom bool
	for _, c := range got {
		if c.Ambiente == AmbienteProd && c.Serie == 1 {
			sawProd = true
		}
		if c.Ambiente == AmbienteHom && c.Serie == 2 {
			sawHom = true
		}
		if c.Modelo != "55" {
			t.Errorf("modelo = %q", c.Modelo)
		}
	}
	if !sawProd || !sawHom {
		t.Errorf("got %+v, want série 1 in produção and 2 in homologação", got)
	}
}

// Série zero is what a config carries before anybody set one. Claiming it would
// have the first company to save an empty form lock série 0 for every other
// company on the same CNPJ.
func TestSerieZeroIsNotClaimed(t *testing.T) {
	if got := SerieClaimsFor("55", 0, 0); len(got) != 0 {
		t.Fatalf("got %+v, want no claim for an unset série", got)
	}
	if got := SerieClaimsFor("55", 1, 0); len(got) != 1 {
		t.Fatalf("got %+v, want only the produção claim", got)
	}
}

// Releasing what the previous configuration held, and only what it no longer
// holds: a série kept across a save must not be released and re-claimed, which
// would leave a window where somebody else could take it.
func TestOnlyAbandonedSeriesAreReleased(t *testing.T) {
	before := SerieClaimsFor("55", 1, 5)
	after := SerieClaimsFor("55", 2, 5)

	got := AbandonedSerieClaims(before, after)
	if len(got) != 1 {
		t.Fatalf("got %+v, want just the produção série that changed", got)
	}
	if got[0].Serie != 1 || got[0].Ambiente != AmbienteProd {
		t.Errorf("released %+v, want produção série 1", got[0])
	}
}

func TestNothingIsReleasedWhenNothingChanged(t *testing.T) {
	claims := SerieClaimsFor("55", 1, 2)
	if got := AbandonedSerieClaims(claims, claims); len(got) != 0 {
		t.Fatalf("got %+v, want nothing released", got)
	}
}

// A first save has nothing to release.
func TestAFirstSaveReleasesNothing(t *testing.T) {
	if got := AbandonedSerieClaims(nil, SerieClaimsFor("55", 1, 2)); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

// Clearing a série releases it. Otherwise a company that stops using one holds
// it against every other company on that CNPJ, forever.
func TestClearingASerieReleasesIt(t *testing.T) {
	got := AbandonedSerieClaims(SerieClaimsFor("55", 1, 2), SerieClaimsFor("55", 0, 2))
	if len(got) != 1 || got[0].Serie != 1 {
		t.Fatalf("got %+v, want produção série 1 released", got)
	}
}

// The two environments never release each other: they are different worlds and
// the same série number in both is two independent claims.
func TestTheEnvironmentsDoNotReleaseEachOther(t *testing.T) {
	got := AbandonedSerieClaims(SerieClaimsFor("55", 1, 1), SerieClaimsFor("55", 1, 2))
	if len(got) != 1 || got[0].Ambiente != AmbienteHom {
		t.Fatalf("got %+v, want only the homologação claim released", got)
	}
}
