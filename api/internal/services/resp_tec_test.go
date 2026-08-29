package services

import "testing"

func TestHashCSRT(t *testing.T) {
	// O valor esperado NÃO foi copiado da saída da implementação — isso tornaria
	// o teste circular. Foi gerado antes, com uma ferramenta independente:
	//
	//   printf '%s' 'G8063NG5H4YQ01M4L3AKG25OZ4A2GL43180906117473000150550010000000041000000047' \
	//     | openssl dgst -sha1 -binary | openssl base64
	const csrt = "G8063NG5H4YQ01M4L3AKG25OZ4A2GL"
	const chave = "43180906117473000150550010000000041000000047"
	const want = "GDDI7mbBk+EPDy9J8QzqWFn6AOg="
	if got := HashCSRT(csrt, chave); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestBuildRespTecSemCSRTOmiteOGrupo(t *testing.T) {
	got := BuildRespTec("1", "n", "e", "p", "", "", "chave")
	if _, ok := got["idCSRT"]; ok {
		t.Fatal("idCSRT não pode aparecer sem CSRT configurado")
	}
	if _, ok := got["hashCSRT"]; ok {
		t.Fatal("hashCSRT não pode aparecer sem CSRT configurado")
	}
}

// O par é tudo-ou-nada: só o id, sem o segredo, não emite nada.
func TestBuildRespTecIdSemCsrtOmiteOGrupo(t *testing.T) {
	got := BuildRespTec("1", "n", "e", "p", "01", "", "chave")
	if _, ok := got["idCSRT"]; ok {
		t.Fatalf("idCSRT sem CSRT não deveria sair: %v", got)
	}
}

func TestBuildRespTecComCSRT(t *testing.T) {
	got := BuildRespTec("1", "n", "e", "p", "01", "G8063NG5H4YQ01M4L3AKG25OZ4A2GL",
		"43180906117473000150550010000000041000000047")
	if got["idCSRT"] != "01" || got["hashCSRT"] != "GDDI7mbBk+EPDy9J8QzqWFn6AOg=" {
		t.Fatalf("grupo CSRT errado: %v", got)
	}
}
