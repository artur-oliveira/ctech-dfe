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
