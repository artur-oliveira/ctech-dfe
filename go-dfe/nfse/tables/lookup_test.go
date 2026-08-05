package tables

import "testing"

func TestTribNacional_KnownCode(t *testing.T) {
	e, ok := TribNacional("010101")
	if !ok {
		t.Fatal("TribNacional(010101): esperado encontrado")
	}
	if e.Item != "1" || e.Subitem != "1" || e.Desdobro != "1" {
		t.Errorf("item/subitem/desdobro = %q/%q/%q, esperado 1/1/1", e.Item, e.Subitem, e.Desdobro)
	}
	if e.Description == "" {
		t.Error("descrição vazia")
	}
}

func TestTribNacional_RejectsGroupingRows(t *testing.T) {
	// "1" e "10" são cabeçalhos de item/subitem na planilha, nunca códigos válidos.
	// "10101" é a forma de 5 dígitos com o item sem zero à esquerda — a coluna A da
	// planilha perde esse zero por ser numérica; o código correto é "010101".
	for _, code := range []string{"", "1", "10", "99999", "10101"} {
		if IsValidTribNacional(code) {
			t.Errorf("IsValidTribNacional(%q) = true, esperado false", code)
		}
	}
}

func TestTribNacional_AllCodesAreSixDigits(t *testing.T) {
	if len(tribNacionalTable) < 300 {
		t.Fatalf("tabela com %d entradas, esperado >= 300", len(tribNacionalTable))
	}
	for code := range tribNacionalTable {
		if len(code) != 6 {
			t.Errorf("código %q tem %d dígitos, esperado 6", code, len(code))
		}
	}
}

func TestNBS_NormalizedCodes(t *testing.T) {
	if len(nbsTable) < 900 {
		t.Fatalf("tabela NBS com %d entradas, esperado >= 900", len(nbsTable))
	}
	for code := range nbsTable {
		if len(code) != 9 {
			t.Errorf("código NBS %q tem %d dígitos, esperado 9", code, len(code))
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("código NBS %q contém caractere não numérico", code)
			}
		}
	}
	if IsValidNBS("1.0101.11.00") {
		t.Error("código NBS com pontos deveria ser inválido — normalize antes de consultar")
	}
}

func TestIndOp_KnownCode(t *testing.T) {
	if !IsValidIndOp("020101") {
		t.Error("IsValidIndOp(020101) = false, esperado true")
	}
	if IsValidIndOp("999999") {
		t.Error("IsValidIndOp(999999) = true, esperado false")
	}
}

func TestIndOp_MergedCellsForwardFilled(t *testing.T) {
	// 020201 e 020301 herdam "Art. 11 / Inc. II" de células mescladas;
	// o gerador deve ter propagado os textos, não deixado vazio.
	for _, code := range []string{"020201", "020301"} {
		e, ok := indOpTable[code]
		if !ok {
			t.Fatalf("indOp %s ausente", code)
		}
		if e.TipoOperacao == "" || e.LocalFornecimento == "" {
			t.Errorf("indOp %s com campo vazio: %+v", code, e)
		}
	}
}
