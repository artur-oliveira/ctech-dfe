package services

// SEFAZ environment codes as they appear in a document and in a série claim.
// "1" is produção and "2" homologação — the same values tpAmb carries.
const (
	AmbienteProd = "1"
	AmbienteHom  = "2"
)

// SerieClaim is one série a company holds under its tax id.
//
// The tax id is not here: it comes from the company being saved, and carrying
// it would let a caller build a claim for a document that is not theirs.
type SerieClaim struct {
	Modelo   string
	Ambiente string
	Serie    int
}

// SerieClaimsFor lists the claims a fiscal configuration implies.
//
// Both environments, always. A configuration declares a série for each and both
// can be emitted under, so claiming only the active one leaves the other free
// for somebody else on the same CNPJ — and a homologação collision is how a
// company finds out about the problem in produção.
//
// Série zero is skipped: it is what a configuration carries before anybody set
// one, and claiming it would have the first company to save an empty form lock
// série 0 against every other company sharing that CNPJ.
func SerieClaimsFor(modelo string, prodSerie, homSerie int) []SerieClaim {
	out := make([]SerieClaim, 0, 2)
	if prodSerie > 0 {
		out = append(out, SerieClaim{Modelo: modelo, Ambiente: AmbienteProd, Serie: prodSerie})
	}
	if homSerie > 0 {
		out = append(out, SerieClaim{Modelo: modelo, Ambiente: AmbienteHom, Serie: homSerie})
	}
	return out
}

// AbandonedSerieClaims lists what the previous configuration held and the new
// one does not.
//
// Only what actually changed. A série kept across a save must not be released
// and re-claimed: that leaves a window in which another company on the same
// CNPJ can take it, for a save that changed nothing about it.
//
// A cleared série IS released. A company that stops using one would otherwise
// hold it against everybody else on that CNPJ forever, with no way to notice.
func AbandonedSerieClaims(before, after []SerieClaim) []SerieClaim {
	kept := make(map[SerieClaim]bool, len(after))
	for _, c := range after {
		kept[c] = true
	}
	out := make([]SerieClaim, 0)
	for _, c := range before {
		if !kept[c] {
			out = append(out, c)
		}
	}
	return out
}
