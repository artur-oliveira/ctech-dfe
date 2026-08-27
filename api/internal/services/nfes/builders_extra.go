package nfes

// builders_extra.go — helpers de saneamento de campos livres da NF-e.
// Extraído de builders_doc.go.

// natOpMaxLen is the SEFAZ ide.natOp limit (xNatOp: 1-60 chars).
const natOpMaxLen = 60

// truncateNatOp enforces the SEFAZ ide.natOp 60-char limit. The frontend sends a
// summarized CFOP description; this is a rune-safe safety net that truncates with
// an ellipsis suffix when the value exceeds natOpMaxLen.
func truncateNatOp(s string) string {
	r := []rune(s)
	if len(r) <= natOpMaxLen {
		return s
	}
	return string(r[:natOpMaxLen-3]) + "..."
}

// docExtras carrega os grupos opcionais da NF-e que não cabem nos parâmetros
// posicionais de BuildEnviNFe. Cada tarefa de cobertura de tags acrescenta um
// campo aqui em vez de mais um parâmetro na assinatura.
type docExtras struct {
	// NFref são os documentos referenciados já resolvidos (ide/NFref).
	NFRefs []map[string]any
}
