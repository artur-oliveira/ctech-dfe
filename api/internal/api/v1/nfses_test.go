package v1

import (
	"testing"

	"github.com/gofiber/fiber/v3"
)

// TestRegisterNfses_MountsAllRoutes garante que toda rota da spec §5.1 existe.
// Inspeciona a tabela de rotas em vez de disparar requisições: os handlers
// dependem de PermChecker e NfseService reais, e o que precisa ser verificado
// aqui é só a montagem.
func TestRegisterNfses_MountsAllRoutes(t *testing.T) {
	app := fiber.New()
	RegisterNfses(app, nil, nil, func(c fiber.Ctx) error { return c.Next() }, nil)

	mounted := map[string]bool{}
	for _, r := range app.GetRoutes() {
		mounted[r.Method+" "+r.Path] = true
	}

	want := []string{
		"POST /nfses",
		"GET /nfses",
		"GET /nfses/:id",
		"GET /nfses/:id/xml",
		"GET /nfses/:id/dps-xml",
		"GET /nfses/:id/danfse",
		"POST /nfses/:id/cancel",
		"POST /nfses/:id/substitute",
		"POST /nfses/:id/events",
		"GET /nfses/:id/events",
		"GET /nfses/:id/events/:event_sk/xml",
		"GET /nfse/municipal-parameters/:city/:kind",
		"GET /nfse/distributions",
	}
	for _, w := range want {
		if !mounted[w] {
			t.Errorf("rota não montada: %s", w)
		}
	}
}

// TestMunicipalParamArgs cobre a montagem posicional dos argumentos: a ordem é
// a do path do ADN e um deslocamento silencioso consultaria o parâmetro errado.
func TestMunicipalParamArgs(t *testing.T) {
	tests := []struct {
		kind                            string
		service, competence, benefitNum string
		want                            []string
	}{
		{"aliquota", "010101", "2026-01", "", []string{"3550308", "010101", "2026-01"}},
		{"convenio", "", "", "", []string{"3550308"}},
		{"beneficio", "", "2026-01", "42", []string{"3550308", "42", "2026-01"}},
		{"regimes_especiais", "010101", "2026-01", "", []string{"3550308", "010101", "2026-01"}},
		{"retencoes", "", "2026-01", "", []string{"3550308", "2026-01"}},
		{"desconhecido", "", "", "", []string{"3550308"}},
	}
	for _, tt := range tests {
		got := municipalParamArgs(tt.kind, "3550308", tt.service, tt.competence, tt.benefitNum)
		if len(got) != len(tt.want) {
			t.Fatalf("%s: got %v, want %v", tt.kind, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: arg %d = %q, want %q", tt.kind, i, got[i], tt.want[i])
			}
		}
	}
}
