package mdfes

import "testing"

// infCIOT 0-n: cada CIOT leva o CPF ou CNPJ do seu responsável (MOC, infANTT/infCIOT).
func TestBuildInfCIOT_MultiplosComResponsavel(t *testing.T) {
	p := buildParams{ciots: []MdfeCIOT{
		{CIOT: "123456789012", ResponsibleDoc: "12345678000190"},
		{CIOT: "210987654321", ResponsibleDoc: "123.456.789-09"},
	}}
	got := p.buildInfCIOT()
	if len(got) != 2 {
		t.Fatalf("itens = %d, want 2", len(got))
	}
	if got[0]["CNPJ"] != "12345678000190" || got[0]["CPF"] != nil {
		t.Errorf("responsável CNPJ: %v", got[0])
	}
	if got[1]["CPF"] != "12345678909" || got[1]["CNPJ"] != nil {
		t.Errorf("responsável CPF sem máscara: %v", got[1])
	}
}

func TestBuildInfCIOT_SemCIOTNaoEmiteGrupo(t *testing.T) {
	if got := (buildParams{}).buildInfCIOT(); len(got) != 0 {
		t.Fatalf("sem CIOT não há infCIOT: %v", got)
	}
}

func TestValidateCIOTs(t *testing.T) {
	ok := []MdfeCIOT{{CIOT: "123456789012", ResponsibleDoc: "12345678909"}}
	if err := validateCIOTs(ok); err != nil {
		t.Fatalf("válido recusado: %v", err)
	}
	dup := append(ok, MdfeCIOT{CIOT: "123456789012", ResponsibleDoc: "12345678000190"})
	if err := validateCIOTs(dup); err == nil {
		t.Error("CIOT repetido devia ser recusado")
	}
	bad := []MdfeCIOT{{CIOT: "123456789012", ResponsibleDoc: "1234567890"}}
	if err := validateCIOTs(bad); err == nil {
		t.Error("responsável com 10 dígitos devia ser recusado")
	}
}
