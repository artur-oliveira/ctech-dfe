package services

import "testing"

func TestCleanText(t *testing.T) {
	for in, want := range map[string]string{
		"  Plano   Fibra\t100 \n": "Plano Fibra 100",
		"a\x00b\x07c":             "abc",
		"":                        "",
		"   ":                     "",
		"já limpo":                "já limpo",
		"linha1\r\nlinha2":        "linha1 linha2",
	} {
		if got := CleanText(in); got != want {
			t.Errorf("CleanText(%q) = %q, want %q", in, got, want)
		}
	}
}
