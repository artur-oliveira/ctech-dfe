package mdfes

import "testing"

func strp(s string) *string { return &s }

// MOC F14 a F17: o tpEmit decide o tipo de documento e o alcance da operação.
func TestResolveTpEmit(t *testing.T) {
	cases := []struct {
		name, docType, ufIni, ufFim string
		requested                   *string
		want                        string
		wantErr                     bool
	}{
		{"NF-e sem pedido é carga própria", docTypeNFe, "PI", "MA", nil, "2", false},
		{"CT-e sem pedido é prestador", docTypeCTe, "PI", "MA", nil, "1", false},
		{"prestador com NF-e (F14)", docTypeNFe, "PI", "PI", strp("1"), "", true},
		{"carga própria com CT-e (F15)", docTypeCTe, "PI", "PI", strp("2"), "", true},
		{"globalizado com CT-e (F16)", docTypeCTe, "PI", "PI", strp("3"), "", true},
		{"globalizado intraestadual", docTypeNFe, "PI", "PI", strp("3"), "3", false},
		{"globalizado interestadual (F17)", docTypeNFe, "PI", "MA", strp("3"), "", true},
		{"globalizado com exterior (F17)", docTypeNFe, "EX", "EX", strp("3"), "", true},
		{"valor inválido", docTypeNFe, "PI", "PI", strp("9"), "", true},
		{"vazio vale como ausente", docTypeCTe, "PI", "MA", strp(""), "1", false},
	}
	for _, c := range cases {
		got, err := resolveTpEmit(c.docType, c.requested, c.ufIni, c.ufFim)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("%s: got (%q, %v), want (%q, err=%v)", c.name, got, err, c.want, c.wantErr)
		}
	}
}

// MOC F113 (684, NT 2026.001): rodoviário de prestador (1) ou globalizado (3), com RNTRC e fora do exterior, exige CIOT.
func TestRequireCIOT(t *testing.T) {
	cases := []struct {
		name, modal, tpEmit, rntrc, ufIni, ufFim string
		hasCIOT, wantErr                         bool
	}{
		{"prestador rodoviário sem CIOT", ModalRodoviario, "1", "012345678", "PI", "MA", false, true},
		{"globalizado sem CIOT", ModalRodoviario, "3", "012345678", "PI", "PI", false, true},
		{"prestador com CIOT", ModalRodoviario, "1", "012345678", "PI", "MA", true, false},
		{"carga própria não exige", ModalRodoviario, "2", "012345678", "PI", "MA", false, false},
		{"sem RNTRC não exige", ModalRodoviario, "1", "", "PI", "MA", false, false},
		{"exterior não exige", ModalRodoviario, "1", "012345678", "EX", "PI", false, false},
		{"outro modal não exige", ModalAereo, "1", "012345678", "PI", "MA", false, false},
	}
	for _, c := range cases {
		err := requireCIOT(c.modal, c.tpEmit, c.rntrc, c.ufIni, c.ufFim, c.hasCIOT)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v, wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestBuildIde_TpEmit(t *testing.T) {
	if got := (buildParams{}).tpEmitOrDefault(); got != "2" {
		t.Errorf("padrão = %q, want 2 (carga própria)", got)
	}
	if got := (buildParams{tpEmit: "1"}).tpEmitOrDefault(); got != "1" {
		t.Errorf("tpEmit pedido = %q, want 1", got)
	}
}
