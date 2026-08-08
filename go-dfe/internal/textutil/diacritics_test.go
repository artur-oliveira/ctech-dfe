package textutil

import "testing"

func TestRemoveDiacriticsPreservesCase(t *testing.T) {
	input := "áàâãä éê í óôõö úü ç ÁÀÂÃÄ ÉÊ Í ÓÔÕÖ ÚÜ Ç"
	want := "aaaaa ee i oooo uu c AAAAA EE I OOOO UU C"
	if got := RemoveDiacritics(input); got != want {
		t.Errorf("RemoveDiacritics() = %q, esperado %q", got, want)
	}
}

func TestRemoveDiacriticsSupportsCombiningMarks(t *testing.T) {
	input := "Araga\u0303o"
	if got := RemoveDiacritics(input); got != "Aragao" {
		t.Errorf("RemoveDiacritics() = %q, esperado Aragao", got)
	}
}

func TestRemoveDiacriticsDoesNotChangePlainText(t *testing.T) {
	const input = "SVC001 - ABC xyz 123"
	if got := RemoveDiacritics(input); got != input {
		t.Errorf("RemoveDiacritics() alterou texto sem acento: %q", got)
	}
}
