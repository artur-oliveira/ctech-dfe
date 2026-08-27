package nfes

// builders_ide.go — nó ide da NF-e (identificação do documento). Extraído de
// builders_doc.go: ide é o nó que mais cresce ao longo da cobertura de tags.

import "fmt"

// ideParams agrupa o que o nó ide precisa. É struct e não 15 parâmetros
// posicionais porque ide é o nó que mais cresce neste plano.
type ideParams struct {
	CUF, CNF, NatOp, Model, AccessKey string
	Serie, Number, Environment        int
	DhEmi                             string
	TpNF, IdDest, CMunFG              string
	FinNFe, IndFinal, IndPres         string
	Mode                              EmissionMode
	VerProc                           string
	// NFref são os documentos referenciados já resolvidos (ide/NFref).
	NFref []map[string]any
}

// buildIde monta o nó ide, incluindo o grupo de contingência quando a emissão
// não é normal.
func buildIde(p ideParams) map[string]any {
	ide := map[string]any{
		"cUF":      p.CUF,
		"cNF":      p.CNF,
		"natOp":    p.NatOp,
		"mod":      p.Model,
		"serie":    fmt.Sprintf("%d", p.Serie),
		"nNF":      fmt.Sprintf("%d", p.Number),
		"dhEmi":    p.DhEmi,
		"tpNF":     p.TpNF,
		"idDest":   p.IdDest,
		"cMunFG":   p.CMunFG,
		"tpImp":    p.Mode.TpImp,
		"tpEmis":   p.Mode.TpEmis,
		"cDV":      string(p.AccessKey[len(p.AccessKey)-1]),
		"tpAmb":    fmt.Sprintf("%d", p.Environment),
		"finNFe":   p.FinNFe,
		"indFinal": p.IndFinal,
		"indPres":  p.IndPres,
		"procEmi":  procEmiApp,
		"verProc":  p.VerProc,
	}
	// dhCont + xJust são exigidos "apenas para tpEmis diferente de 1" (XSD).
	if p.Mode.IsContingency() {
		ide["dhCont"] = fmtDhEmi(p.Mode.ContingencyAt)
		ide["xJust"] = p.Mode.Justification
	}
	if len(p.NFref) > 0 {
		ide["NFref"] = p.NFref
	}
	return ide
}
