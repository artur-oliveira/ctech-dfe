package validate

import (
	"regexp"
	"strconv"
)

// Overlay das NTs que os XSDs vendorizados não trazem. O NF-e vendorizado (PL_010e) é anterior às NT 2026.005
// (Simples Nacional no IBS/CBS) e 2026.008 (valor líquido do item). Todo nó do overlay é opcional: a obrigatoriedade
// condicional dessas tags é regra de negócio (rules.go), não de schema. Remover cada entrada quando o XSD oficial
// que a contém for vendorizado (o teste TestOverlayNotInXSD avisa).
func pat(p string) *simple {
	s := &simple{Base: "string", Pattern: []string{p}}
	s.patterns = []*regexp.Regexp{regexp.MustCompile("^(?:" + p + ")$")}
	return s
}

func enumOf(vals ...string) *simple { return &simple{Base: "string", Enum: vals} }

// dec devolve o padrão TDec_<int>v<min>-<max> do leiaute (sem sinal).
func dec(intDigits, minFrac, maxFrac int) string {
	return "0|0\\.[0-9]{" + strconv.Itoa(minFrac) + "," + strconv.Itoa(maxFrac) + "}|[1-9][0-9]{0," + strconv.Itoa(intDigits-1) + "}(\\.[0-9]{" + strconv.Itoa(minFrac) + "," + strconv.Itoa(maxFrac) + "})?"
}

func leaf(name string, s *simple) *node {
	return &node{Name: name, Min: 0, Max: 1, Simple: s, overlay: true}
}

func group(name string, kids ...*node) *node {
	n := &node{Name: name, Min: 0, Max: 1, Children: kids, overlay: true}
	n.byName = map[string]*node{}
	for _, k := range kids {
		n.byName[k.Name] = k
	}
	return n
}

// attach acrescenta kids ao nó em path (nomes a partir da raiz) se ainda não existirem.
func attach(root *node, path []string, kids ...*node) {
	cur := root
	for _, p := range path {
		next := cur.byName[p]
		if next == nil {
			return
		}
		cur = next
	}
	for _, k := range kids {
		if _, ok := cur.byName[k.Name]; ok {
			continue
		}
		cur.Children = append(cur.Children, k)
		cur.byName[k.Name] = k
	}
}

func applyNFeOverlay(root *node) {
	attach(root, []string{"infNFe", "emit"}, leaf("tpRegimeIBSCBS", enumOf("1", "2", "3", "4")))

	attach(root, []string{"infNFe", "det", "prod"},
		leaf("vUnComLiq", pat(dec(11, 0, 10))),
		leaf("vProdLiq", pat(dec(13, 2, 2))),
		leaf("cAtivSN", pat("[0-9]{1,9}")),
	)

	gibs := group("gIBSSN", leaf("pIBSSN", pat(dec(3, 2, 4))), leaf("vIBSSN", pat(dec(13, 2, 2))))
	gcbs := group("gCBSSN", leaf("pCBSSN", pat(dec(3, 2, 4))), leaf("vCBSSN", pat(dec(13, 2, 2))))
	attach(root, []string{"infNFe", "det", "imposto", "IBSCBS"}, group("gTribSN", gibs, gcbs))

	attach(root, []string{"infNFe", "total"}, leaf("vProdLiqTot", pat(dec(13, 2, 2))))
	attach(root, []string{"infNFe", "total", "IBSCBSTot"}, group("gTribSN",
		leaf("vIBSSN", pat(dec(13, 2, 2))), leaf("vCBSSN", pat(dec(13, 2, 2)))))
}
