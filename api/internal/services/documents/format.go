package documents

import (
	"strings"
	"time"
	"unicode"

	"github.com/shopspring/decimal"
)

func digits(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, value)
}

func moneyBR(value string) string {
	amount, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		amount = decimal.Zero
	}
	raw := amount.StringFixed(2)
	parts := strings.SplitN(raw, ".", 2)
	integer := parts[0]
	start := 0
	if strings.HasPrefix(integer, "-") {
		start = 1
	}
	for i := len(integer) - 3; i > start; i -= 3 {
		integer = integer[:i] + "." + integer[i:]
	}
	return integer + "," + parts[1]
}

// percentBR formata um percentual fiscal no padrão pt-BR, preservando as casas
// decimais que vieram no XML — o XSD usa de 2 a 3 casas conforme o campo, e
// arredondar aqui mudaria o número impresso em relação ao documento assinado.
func percentBR(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if _, err := decimal.NewFromString(value); err != nil {
		return value
	}
	return strings.Replace(value, ".", ",", 1)
}

func cents(value string) string {
	amount, err := decimal.NewFromString(strings.TrimSpace(value))
	if err != nil {
		return "0"
	}
	return amount.Mul(decimal.NewFromInt(100)).Round(0).StringFixed(0)
}

func nonzero(value string) bool {
	amount, err := decimal.NewFromString(strings.TrimSpace(value))
	return err == nil && !amount.IsZero()
}

func parseISO(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		parsed, err = time.Parse("2006-01-02", value)
	}
	return parsed, err == nil
}

func dateTimeBR(value string) string {
	if parsed, ok := parseISO(value); ok {
		return parsed.Format("02/01/2006 15:04:05")
	}
	return ""
}

func dateBR(value string) string {
	if parsed, ok := parseISO(value); ok {
		return parsed.Format("02/01/2006")
	}
	return ""
}

func timeBR(value string) string {
	if parsed, ok := parseISO(value); ok {
		return parsed.Format("15:04:05")
	}
	return ""
}

func maskCNPJ(value string) string {
	d := digits(value)
	if len(d) != 14 {
		return value
	}
	return d[:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:]
}

func maskCPF(value string) string {
	d := digits(value)
	if len(d) != 11 {
		return value
	}
	return d[:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:]
}

func maskCPFCNPJ(value string) string {
	switch len(digits(value)) {
	case 11:
		return maskCPF(value)
	case 14:
		return maskCNPJ(value)
	default:
		return value
	}
}

func maskCEP(value string) string {
	d := digits(value)
	if len(d) != 8 {
		return value
	}
	return d[:5] + "-" + d[5:]
}

func keyBlocks(value string) string {
	d := digits(value)
	parts := make([]string, 0, (len(d)+3)/4)
	for len(d) > 0 {
		end := min(4, len(d))
		parts = append(parts, d[:end])
		d = d[end:]
	}
	return strings.Join(parts, " ")
}

func fiscalNumber(value string) string {
	d := digits(value)
	if d == "" {
		return ""
	}
	if len(d) < 9 {
		d = strings.Repeat("0", 9-len(d)) + d
	}
	return d[:3] + "." + d[3:6] + "." + d[6:9]
}

// keyParts pulls the emitente document, série and número out of a 44-digit DFe
// access key. The MDF-e XML carries only the keys of the documents it manifests,
// but the DAMDFE lists those three fields, and the key encodes them:
// cUF(2) AAMM(4) CNPJ(14) mod(2) série(3) número(9) tpEmis(1) cNF(8) cDV(1).
func keyParts(key string) (doc, series, number string) {
	key = digits(key)
	if len(key) != 44 {
		return "", "", ""
	}
	return maskCNPJ(key[6:20]), key[22:25], fiscalNumber(key[25:34])
}

// streetAddress is the DANFE ENDEREÇO field, which the MOC keeps separate from
// BAIRRO / DISTRITO — unlike the single-line address blocks.
func streetAddress(n *xmlNode) string {
	return strings.Join(nonempty(n.value("xLgr"), n.value("nro"), n.value("xCpl")), ", ")
}

func address(n *xmlNode) string {
	parts := []string{n.value("xLgr"), n.value("nro"), n.value("xCpl"), n.value("xBairro")}
	nonempty := parts[:0]
	for _, part := range parts {
		if part != "" {
			nonempty = append(nonempty, part)
		}
	}
	return strings.Join(nonempty, ", ")
}
