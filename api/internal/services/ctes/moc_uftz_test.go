package ctes

import (
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// Regressão: dhEmi e o AAMM da chave são horário local da UF do emitente. 31/12 23:30 em AM (UTC-4) e
// 31/12 21:30 em AC (UTC-5) já são 01/01 em UTC; o processo (Lambda) roda em UTC.
func TestDhEmiEAAMMNoFusoDaUF(t *testing.T) {
	ref := time.Date(2026, 1, 1, 3, 30, 0, 0, time.UTC)
	cases := []struct{ uf, dhEmi, aamm string }{
		{"AM", "2025-12-31T23:30:00-04:00", "2512"},
		{"AC", "2025-12-31T22:30:00-05:00", "2512"},
		{"SP", "2026-01-01T00:30:00-03:00", "2601"},
	}
	for _, c := range cases {
		p := baseParams(t, RoleRemetente)
		p.now = ref.In(uftz.Location(c.uf))
		p.accessKey = services.GenerateAccessKey(c.uf, cnpjA, services.ModelCTe, 1, 8, p.now, services.TpEmisNormal)
		ide := BuildCTe(p)["CTe"].(map[string]any)["infCte"].(map[string]any)["ide"].(map[string]any)
		if ide["dhEmi"] != c.dhEmi {
			t.Errorf("%s dhEmi = %v, want %s", c.uf, ide["dhEmi"], c.dhEmi)
		}
		if got := p.accessKey[2:6]; got != c.aamm {
			t.Errorf("%s AAMM = %s, want %s", c.uf, got, c.aamm)
		}
	}
}

// A clock de emissão (emit, complemento, OS) é uftz.Now(UF): o instante é o mesmo, o relógio de parede é o da UF.
func TestEmissaoUsaRelogioDaUF(t *testing.T) {
	if _, off := uftz.Now("AM").Zone(); off != -4*3600 {
		t.Errorf("offset AM = %d", off)
	}
}
