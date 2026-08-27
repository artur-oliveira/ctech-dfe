package mdfes

import "testing"

func TestResolvePeriSomaQuantidadesPorONU(t *testing.T) {
	items := []parsedItem{
		{CProd: "A", QCom: "10.0000", UCom: "L"},
		{CProd: "B", QCom: "5.0000", UCom: "L"},
		{CProd: "C", QCom: "1.0000", UCom: "UN"}, // não perigoso
	}
	gasolina := map[string]any{
		"peri_n_onu": "1203", "peri_x_nome_ae": "GASOLINA", "peri_x_cla_risco": "3",
		"peri_gr_emb": "II", "peri_q_vol_tipo": "TAMBOR",
	}
	byCode := map[string]map[string]any{"A": gasolina, "B": gasolina, "C": {}}

	got := resolvePeri(items, byCode)
	if len(got) != 1 {
		t.Fatalf("itens do mesmo ONU têm que virar um grupo só: %v", got)
	}
	if got[0]["nONU"] != "1203" || got[0]["qTotProd"] != "15.0000" {
		t.Fatalf("agrupamento errado: %v", got[0])
	}
	if got[0]["grEmb"] != "II" || got[0]["qVolTipo"] != "TAMBOR" {
		t.Fatalf("campos do cadastro perdidos: %v", got[0])
	}
}

func TestResolvePeriSemProdutoPerigosoDevolveNil(t *testing.T) {
	if resolvePeri([]parsedItem{{CProd: "C"}}, map[string]map[string]any{"C": {}}) != nil {
		t.Fatal("nota sem produto perigoso não pode gerar peri")
	}
}

// Produto sem grupo de embalagem é válido: grEmb é opcional no XSD.
func TestResolvePeriSemGrupoEmbalagem(t *testing.T) {
	got := resolvePeri([]parsedItem{{CProd: "A", QCom: "2"}}, map[string]map[string]any{
		"A": {"peri_n_onu": "1993", "peri_x_nome_ae": "LIQUIDO INFLAMAVEL", "peri_x_cla_risco": "3"},
	})
	if _, ok := got[0]["grEmb"]; ok {
		t.Fatalf("grEmb vazio não devia sair: %v", got[0])
	}
	if got[0]["qTotProd"] != "2.0000" {
		t.Fatalf("quantidade errada: %v", got[0])
	}
}
