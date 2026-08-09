package nfes

import "strings"

// aliqICMSTable[emit_uf][dest_uf] = alíquota ICMS interestadual.
// Mirrors app/constants/tax_tables.py ALIQ_ICMS_TABLE.
var aliqICMSTable = buildICMSTable()

func buildICMSTable() map[string]map[string]string {
	all := []string{
		"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA",
		"MG", "MS", "MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN",
		"RO", "RR", "RS", "SC", "SE", "SP", "TO",
	}
	row := func(intra string, overrides map[string]string) map[string]string {
		m := make(map[string]string, len(all))
		for _, uf := range all {
			m[uf] = "12.00"
		}
		for k, v := range overrides {
			m[k] = v
		}
		m[intra] = overrides[intra]
		return m
	}
	// For rows where inter = 7.00 for some states:
	row7 := func(intra, intraAliq string, inter12 []string, extra map[string]string) map[string]string {
		m := make(map[string]string, len(all))
		for _, uf := range all {
			m[uf] = "7.00"
		}
		for _, uf := range inter12 {
			m[uf] = "12.00"
		}
		for k, v := range extra {
			m[k] = v
		}
		_ = intra
		m[intra] = intraAliq
		return m
	}
	return map[string]map[string]string{
		"AC": row("AC", map[string]string{"AC": "19.00"}),
		"AL": row("AL", map[string]string{"AL": "21.50"}),
		"AM": row("AM", map[string]string{"AM": "20.00"}),
		"AP": row("AP", map[string]string{"AP": "18.00"}),
		"BA": row("BA", map[string]string{"BA": "20.50"}),
		"CE": row("CE", map[string]string{"CE": "20.00"}),
		"DF": row("DF", map[string]string{"DF": "20.00"}),
		"ES": row("ES", map[string]string{"ES": "17.00"}),
		"GO": row("GO", map[string]string{"GO": "19.00"}),
		"MA": row("MA", map[string]string{"MA": "23.00"}),
		"MG": row7("MG", "18.00", []string{"PR", "RS", "RJ", "SC", "SP"}, nil),
		"MS": row("MS", map[string]string{"MS": "17.00"}),
		"MT": row("MT", map[string]string{"MT": "17.00"}),
		"PA": row("PA", map[string]string{"PA": "19.00"}),
		"PB": row("PB", map[string]string{"PB": "20.00"}),
		"PE": row("PE", map[string]string{"PE": "20.50"}),
		"PI": row("PI", map[string]string{"PI": "22.50"}),
		"PR": row7("PR", "19.50", []string{"MG", "RS", "RJ", "SC", "SP"}, nil),
		"RJ": row7("RJ", "22.00", []string{"MG", "PR", "RS", "SC", "SP"}, nil),
		"RN": row("RN", map[string]string{"RN": "20.50"}),
		"RO": row("RO", map[string]string{"RO": "19.50"}),
		"RR": row("RR", map[string]string{"RR": "20.00"}),
		"RS": row7("RS", "17.00", []string{"MG", "PR", "RJ", "SC", "SP"}, nil),
		"SC": row7("SC", "17.00", []string{"MG", "PR", "RS", "RJ", "SP"}, nil),
		"SE": row("SE", map[string]string{"SE": "20.00"}),
		"SP": row7("SP", "18.00", []string{"MG", "PR", "RS", "RJ", "SC", "SE", "TO"}, nil),
		"TO": row("TO", map[string]string{"TO": "20.00"}),
	}
}

// fcpAliq[dest_uf] = alíquota FCP. Mirrors FCP_ALIQ in tax_tables.py.
var fcpAliq = map[string]string{
	"BA": "2.00", "DF": "2.00", "ES": "2.00", "MA": "2.00", "MS": "2.00",
	"MG": "2.00", "PB": "2.00", "PR": "2.00", "PE": "2.00", "PI": "2.00",
	"RN": "2.00", "RS": "2.00", "RO": "2.00", "SP": "2.00", "TO": "2.00",
	"GO": "2.00", "MT": "2.00", "RR": "2.00", "AL": "2.00", "AM": "2.00",
	"RJ": "2.00", "SE": "2.00",
	"AC": "0.00", "AP": "0.00", "CE": "0.00", "PA": "0.00", "SC": "0.00",
}

// icmsNcmEntry is an ICMS rate specific to one NCM prefix, within a UF.
// Mirrors ui/src/lib/data/icms_ncm_lookup.ts IcmsNcmEntry — migrated here so
// the backend is the single source of truth (design spec
// 2026-08-09-tax-config-redesign §Modelo de dados 5).
type icmsNcmEntry struct {
	ncm  string
	aliq string
	fcp  *string
}

// icmsNcmTable[dest_uf] = entries sorted from most to least specific NCM
// prefix. Populated by scripts/generate-icms-lookup (moved from the frontend
// lookup table, which is removed once this is the only copy).
var icmsNcmTable = map[string][]icmsNcmEntry{}

// resolveIcmsNcm returns the most specific icmsNcmTable entry for destUF+ncm,
// or nil if none matches.
func resolveIcmsNcm(destUF, ncm string) *icmsNcmEntry {
	for _, e := range icmsNcmTable[destUF] {
		if strings.HasPrefix(ncm, e.ncm) {
			return &e
		}
	}
	return nil
}

func resolveICMSAliq(emitUF, destUF, ncm string, override *string) string {
	if override != nil && *override != "" {
		return *override
	}
	if e := resolveIcmsNcm(destUF, ncm); e != nil {
		return e.aliq
	}
	if row, ok := aliqICMSTable[emitUF]; ok {
		if aliq, ok := row[destUF]; ok {
			return aliq
		}
	}
	return "17.00"
}

func resolveFCPAliq(destUF, ncm string, override *string) string {
	if override != nil {
		return *override
	}
	if e := resolveIcmsNcm(destUF, ncm); e != nil && e.fcp != nil {
		return *e.fcp
	}
	if v, ok := fcpAliq[destUF]; ok {
		return v
	}
	return "0.00"
}

func resolveICMSIntraAliq(uf string) string {
	if row, ok := aliqICMSTable[uf]; ok {
		if aliq, ok := row[uf]; ok {
			return aliq
		}
	}
	return "17.00"
}

func resolveICMSInterAliq(emitUF, destUF string, origin *string) string {
	if origin != nil {
		switch *origin {
		case "1", "2", "6", "7":
			return "4.00"
		}
	}
	aliq := "12.00"
	if row, ok := aliqICMSTable[emitUF]; ok {
		if v, ok := row[destUF]; ok {
			aliq = v
		}
	}
	if aliq == "4.00" || aliq == "7.00" {
		return aliq
	}
	return "12.00"
}
