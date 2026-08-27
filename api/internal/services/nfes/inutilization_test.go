package nfes

import (
	"reflect"
	"testing"
)

// currentNumber é o próximo número a emitir, não o último emitido — ele nunca
// entra numa lacuna, porque ainda vai virar documento.
func TestGapRuns(t *testing.T) {
	tests := []struct {
		name          string
		usable        []int
		currentNumber int
		want          []NumberGap
	}{
		{"no gaps", []int{1, 2, 3}, 4, []NumberGap{}},
		{"hole in the middle", []int{1, 4}, 5, []NumberGap{{Serie: 1, NumberStart: 2, NumberEnd: 3}}},
		{"open tail", []int{1}, 4, []NumberGap{{Serie: 1, NumberStart: 2, NumberEnd: 3}}},
		{"leading hole", []int{3}, 4, []NumberGap{{Serie: 1, NumberStart: 1, NumberEnd: 2}}},
		{"two runs", []int{3}, 6, []NumberGap{
			{Serie: 1, NumberStart: 1, NumberEnd: 2},
			{Serie: 1, NumberStart: 4, NumberEnd: 5},
		}},
		{"nothing issued yet", nil, 0, []NumberGap{}},
		{"next number pending is never a gap", []int{1, 2}, 3, []NumberGap{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usable := map[int]bool{}
			for _, n := range tt.usable {
				usable[n] = true
			}
			got := gapRuns(usable, 1, tt.currentNumber)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("gapRuns = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildInutBody(t *testing.T) {
	body := buildInutBody("35", "11222333000181", nfModel55, 2, 2026, 1, 10, 20,
		"numeros perdidos por falha de transmissao")
	inf := body[inutRootTag].(map[string]any)["infInut"].(map[string]any)

	// ID = "ID" + cUF(2) + ano(2) + CNPJ(14) + mod(2) + serie(3) + nNFIni(9) + nNFFin(9)
	wantID := "ID" + "35" + "26" + "11222333000181" + "55" + "001" + "000000010" + "000000020"
	if inf["@Id"] != wantID {
		t.Errorf("@Id = %v, want %v", inf["@Id"], wantID)
	}
	// "ID" + cUF(2)+ano(2)+CNPJ(14)+mod(2)+serie(3)+nNFIni(9)+nNFFin(9) = 43
	if len(wantID) != 43 {
		t.Errorf("Id length = %d, want 43", len(wantID))
	}
	for k, want := range map[string]string{
		"tpAmb": "2", "xServ": inutXServ, "cUF": "35", "ano": "26",
		"CNPJ": "11222333000181", "mod": nfModel55, "serie": "1",
		"nNFIni": "10", "nNFFin": "20",
	} {
		if inf[k] != want {
			t.Errorf("%s = %v, want %v", k, inf[k], want)
		}
	}
	// The layout has no CPF choice — never emit one.
	if _, ok := inf["CPF"]; ok {
		t.Error("infInut must not carry CPF")
	}
}

func TestInutEventKeyIsRangeUnique(t *testing.T) {
	a := inutEventKey(2026, 1, 10, 20)
	if a != "INUT#2026#001#000000010#000000020" {
		t.Fatalf("event_key = %q", a)
	}
	if a == inutEventKey(2026, 1, 10, 21) {
		t.Error("different ranges must not collide")
	}
	if pk := inutEventPK("hom", "CNPJ_11222333000181"); pk != "INUT#hom#CNPJ_11222333000181" {
		t.Errorf("pk = %q", pk)
	}
}
