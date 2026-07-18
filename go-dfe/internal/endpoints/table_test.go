package endpoints

import "testing"

// NOTE on SVAN: py-dfe's constants/endpoints.py has no SVAN entries for any
// doc type — every UF without its own authorizer redirects to SVRS only
// (MDF-e in particular has *no* per-UF authorizers at all: every UF,
// including RS, resolves to SVRS). So there is no "UF that redirects to
// SVAN" case to test; this file covers SVRS redirection instead, for both
// NFC-e and CT-e, plus the AN-only MDF-e-style all-SVRS routing.

func TestResolve_DirectAuthorizer_SP(t *testing.T) {
	got, err := Resolve("nfe", "SP", "prod", "NFeAutorizacao")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://nfe.fazenda.sp.gov.br/ws/nfeautorizacao4.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_DirectAuthorizer_SP_Hom(t *testing.T) {
	got, err := Resolve("nfe", "SP", "hom", "NfeStatusServico")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://homologacao.nfe.fazenda.sp.gov.br/ws/nfestatusservico4.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_SVRSRedirect_NFCe(t *testing.T) {
	// PE has no direct NFC-e authorizer -> redirects to SVRS.
	got, err := Resolve("nfce", "PE", "prod", "NFeAutorizacao")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://nfce.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_SVRSRedirect_CTe(t *testing.T) {
	// BA has no direct CT-e authorizer -> redirects to SVRS. The URL is the
	// "doubled path" shape: host + frag + frag + ".asmx".
	got, err := Resolve("cte", "BA", "prod", "CTeConsulta")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://cte.svrs.rs.gov.br/ws/CTeConsultaV4/CTeConsultaV4.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_CTeSVRS_Hom(t *testing.T) {
	got, err := Resolve("cte", "RJ", "hom", "CTeRecepcaoEvento")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://cte-homologacao.svrs.rs.gov.br/ws/CTeRecepcaoEventoV4/CTeRecepcaoEventoV4.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_MDFe_AllUFsRedirectToSVRS(t *testing.T) {
	// MDF-e has no per-UF authorizer at all, not even for RS itself.
	for _, uf := range []string{"RS", "SP", "MT", "AC"} {
		got, err := Resolve("mdfe", uf, "prod", "MDFeDistribuicaoDFe")
		if err != nil {
			t.Fatalf("uf=%s: unexpected error: %v", uf, err)
		}
		want := "https://mdfe.svrs.rs.gov.br/ws/MDFeDistribuicaoDFe/MDFeDistribuicaoDFe.asmx"
		if got != want {
			t.Errorf("uf=%s: got %q, want %q", uf, got, want)
		}
	}
}

func TestResolve_MT_NFe_SpecialCase(t *testing.T) {
	// MT is direct-authorized for itself with its own domain/path shape,
	// distinct from the shared nfFragPath used by GO/MG/MS/PE/PR.
	got, err := Resolve("nfe", "MT", "prod", "NfeConsultaCadastro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://nfe.sefaz.mt.gov.br/nfews/v2/services/CadConsultaCadastro4"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_MT_CTe_SpecialCase_ThreePathPrefixes(t *testing.T) {
	// MT CT-e splits services across three different path prefixes
	// (ctews2, ctews, cte-ws) on the same domain — this is the
	// "MT special case" py-dfe/CLAUDE.md calls out as critical.
	cases := []struct {
		env     string
		service string
		want    string
	}{
		{"prod", "CTeConsulta", "https://cte.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4"},
		{"prod", "CTeRecepcaoOS", "https://cte.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4"},
		{"prod", "CTeRecepcaoSimp", "https://cte.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4"},
		{"hom", "CTeConsulta", "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4"},
		{"hom", "CTeRecepcaoOS", "https://homologacao.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4"},
		{"hom", "CTeRecepcaoSimp", "https://homologacao.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4"},
	}
	for _, tc := range cases {
		got, err := Resolve("cte", "MT", tc.env, tc.service)
		if err != nil {
			t.Fatalf("env=%s service=%s: unexpected error: %v", tc.env, tc.service, err)
		}
		if got != tc.want {
			t.Errorf("env=%s service=%s: got %q, want %q", tc.env, tc.service, got, tc.want)
		}
	}
}

func TestResolve_AN_NFeDistribuicaoDFe(t *testing.T) {
	got, err := Resolve("nfe", "AN", "prod", "NFeDistribuicaoDFe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_AN_NFeDistribuicaoDFe_Hom(t *testing.T) {
	got, err := Resolve("nfe", "AN", "hom", "NFeDistribuicaoDFe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://hom1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolve_AN_CTeDistribuicaoDFe(t *testing.T) {
	got, err := Resolve("cte", "AN", "prod", "CTeDistribuicaoDFe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://www1.cte.fazenda.gov.br/CTeDistribuicaoDFe/CTeDistribuicaoDFe.asmx"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// AN bypasses the per-UF authorizer table entirely: passing uf="AN" must
// not go through docTypeRegistry[...].ufAuth (which has no "AN" key), it
// resolves directly against the registry's "AN" entry.
func TestResolve_AN_BypassesUFAuthTable(t *testing.T) {
	if _, ok := nfeUFAuth["AN"]; ok {
		t.Fatalf("nfeUFAuth unexpectedly contains an AN entry: resolution order assumption broken")
	}
	if _, err := Resolve("nfe", "AN", "prod", "NFeDistribuicaoDFe"); err != nil {
		t.Fatalf("unexpected error resolving AN: %v", err)
	}
}

func TestResolve_UnknownDocType(t *testing.T) {
	if _, err := Resolve("bogus", "SP", "prod", "NFeAutorizacao"); err == nil {
		t.Error("expected error for unknown doc_type, got nil")
	}
}

func TestResolve_UnknownUF(t *testing.T) {
	if _, err := Resolve("nfe", "ZZ", "prod", "NFeAutorizacao"); err == nil {
		t.Error("expected error for unknown uf, got nil")
	}
}

func TestResolve_UnknownService(t *testing.T) {
	if _, err := Resolve("nfe", "SP", "prod", "NoSuchService"); err == nil {
		t.Error("expected error for unknown service, got nil")
	}
}

func TestResolve_UnknownEnvironment(t *testing.T) {
	if _, err := Resolve("nfe", "SP", "staging", "NFeAutorizacao"); err == nil {
		t.Error("expected error for unknown environment, got nil")
	}
}
