package validation

import "testing"

func TestValidCPF(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"529.982.247-25", true}, // valid, punctuated
		{"52998224725", true},    // valid, raw
		{"111.111.111-11", false},
		{"12345678900", false},
		{"529.982.247-24", false}, // bad check digit
		{"123", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ValidCPF(c.in); got != c.want {
			t.Errorf("ValidCPF(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestValidCNPJ(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"11.222.333/0001-81", true}, // valid numeric, punctuated
		{"11222333000181", true},     // valid numeric, raw
		{"11.222.333/0001-80", false},
		{"00000000000000", false},
		{"123", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ValidCNPJ(c.in); got != c.want {
			t.Errorf("ValidCNPJ(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestStructTagValidation exercises the shared validator through small structs,
// confirming each custom tag is wired and that field paths use JSON names.
func TestStructTagValidation(t *testing.T) {
	type sample struct {
		CFOP string `json:"cfop" validate:"required,cfop"`
		UF   string `json:"uf" validate:"required,uf"`
		CEP  string `json:"cep" validate:"required,cep"`
	}

	if p := Struct(sample{CFOP: "5102", UF: "SP", CEP: "01001000"}); p != nil {
		t.Fatalf("expected valid sample, got %+v", p.Errors)
	}

	p := Struct(sample{CFOP: "51", UF: "XX", CEP: "abc"})
	if p == nil {
		t.Fatal("expected validation errors, got nil")
	}
	if p.Status != 422 {
		t.Errorf("status = %d, want 422", p.Status)
	}
	got := map[string]string{}
	for _, fe := range p.Errors {
		got[fe.Field] = fe.Tag
	}
	for _, field := range []string{"cfop", "uf", "cep"} {
		if _, ok := got[field]; !ok {
			t.Errorf("expected error for field %q, got %+v", field, got)
		}
	}
}

func TestTimezoneAndPercent(t *testing.T) {
	type cfg struct {
		TZ      string `json:"tz" validate:"required,timezone"`
		Aliquot string `json:"aliquot" validate:"required,percent"`
	}
	if p := Struct(cfg{TZ: "America/Sao_Paulo", Aliquot: "18.0000"}); p != nil {
		t.Fatalf("expected valid, got %+v", p.Errors)
	}
	if p := Struct(cfg{TZ: "Europe/London", Aliquot: "abc"}); p == nil {
		t.Fatal("expected errors for bad timezone/percent")
	}
}
