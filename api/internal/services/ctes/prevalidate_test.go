package ctes

import (
	"errors"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
)

func TestGateCTePassesBuilderOutput(t *testing.T) {
	for _, role := range []string{RoleRemetente, RoleDestinatario, RoleOutro} {
		if err := gateCTe(BuildCTe(baseParams(t, role))); err != nil {
			t.Fatalf("role %s: %v", role, err)
		}
	}
}

func TestGateCTeBlocksBadDocumentAndKey(t *testing.T) {
	p := baseParams(t, RoleRemetente)
	p.rem.CNPJ = "11222333000199"
	err := gateCTe(BuildCTe(p))
	var pr *problem.Problem
	if !errors.As(err, &pr) || pr.Code != dfecheck.CodeFiscalPrevalidation {
		t.Fatalf("want fiscal_prevalidation, got %v", err)
	}
	if !hasFieldRule(pr, "G114") {
		t.Fatalf("G114 missing: %+v", pr.Errors)
	}

	p = baseParams(t, RoleRemetente)
	dv := byte('0')
	if p.accessKey[43] == dv {
		dv = '1'
	}
	p.accessKey = p.accessKey[:43] + string(dv)
	pr = nil
	if !errors.As(gateCTe(BuildCTe(p)), &pr) || !hasFieldRule(pr, "G015") {
		t.Fatal("G015 (DV da chave) deveria recusar")
	}
}

func hasFieldRule(p *problem.Problem, rule string) bool {
	for _, e := range p.Errors {
		if e.Field == rule {
			return true
		}
	}
	return false
}
