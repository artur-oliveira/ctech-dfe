package mdfes

import (
	"bytes"
	"encoding/xml"
	"io"

	"github.com/shopspring/decimal"
)

// xnode is a minimal, namespace-agnostic XML tree node. Element lookups match on
// local name only, so the same walker handles NF-e and CT-e regardless of their
// (different) default namespaces.
type xnode struct {
	name     string
	text     string
	children []*xnode
}

// parseXML decodes raw XML bytes into an xnode tree.
func parseXML(data []byte) (*xnode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var root *xnode
	var stack []*xnode
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xnode{name: t.Name.Local}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(t)
			}
		}
	}
	return root, nil
}

// child returns the first direct child with the given local name, or nil.
func (n *xnode) child(name string) *xnode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// path walks a chain of direct children, returning the final node or nil.
func (n *xnode) path(names ...string) *xnode {
	cur := n
	for _, name := range names {
		cur = cur.child(name)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// firstDeep returns the first descendant (depth-first) with the given local name.
func (n *xnode) firstDeep(name string) *xnode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.name == name {
			return c
		}
		if found := c.firstDeep(name); found != nil {
			return found
		}
	}
	return nil
}

// allDeep returns all descendants (depth-first) with the given local name.
func (n *xnode) allDeep(name string) []*xnode {
	var out []*xnode
	if n == nil {
		return out
	}
	for _, c := range n.children {
		if c.name == name {
			out = append(out, c)
		}
		out = append(out, c.allDeep(name)...)
	}
	return out
}

// txt returns the trimmed text of the direct child, or "".
func (n *xnode) txt(name string) string {
	c := n.child(name)
	if c == nil {
		return ""
	}
	return trimText(c.text)
}

func trimText(s string) string {
	return string(bytes.TrimSpace([]byte(s)))
}

// party holds the resolved emitter/recipient location of a referenced document.
type party struct {
	cnpj string
	cpf  string
	name string
	cMun string
	xMun string
	uf   string
}

// parsedItem é um item (det/prod) da NF-e referenciada. Serve para reencontrar
// o produto no cadastro e derivar dele o que o MDF-e precisa declarar.
type parsedItem struct {
	CProd string
	XProd string
	QCom  string
	UCom  string
}

// docCargo is the cargo data extracted from one referenced NF-e or CT-e XML.
type docCargo struct {
	accessKey  string
	docType    string // "nfe" | "cte"
	emit       party  // loading point (carregamento)
	dest       party  // unloading point (descarregamento)
	weightKG   decimal.Decimal
	totalValue decimal.Decimal
	predNCM    string // NCM of the highest-value line item
	predProd   string // description of the highest-value line item
	predEAN    string // GTIN of the highest-value line item (may be "SEM GTIN")
	items      []parsedItem
}

// extractCargo parses a referenced document's XML and returns its cargo data.
// docType is "nfe" or "cte".
func extractCargo(accessKey, docType string, xmlData []byte) (*docCargo, error) {
	root, err := parseXML(xmlData)
	if err != nil {
		return nil, err
	}
	if docType == docTypeCTe {
		return extractCargoCTe(accessKey, root)
	}
	return extractCargoNFe(accessKey, root)
}

// extractCargoNFe pulls emit/dest location, gross weight (transp/vol/pesoB) and
// the predominant product (highest vProd line item) from an NF-e XML.
func extractCargoNFe(accessKey string, root *xnode) (*docCargo, error) {
	inf := root.firstDeep("infNFe")
	if inf == nil {
		return nil, errInvalidDocXML
	}
	c := &docCargo{
		accessKey:  accessKey,
		docType:    docTypeNFe,
		weightKG:   decimal.Zero,
		totalValue: decimal.Zero,
	}
	c.emit = parsePartyNFe(inf.child("emit"), "enderEmit")
	c.dest = parsePartyNFe(inf.child("dest"), "enderDest")

	// Gross weight: sum of transp/vol/pesoB; fall back to pesoL when pesoB absent.
	for _, vol := range inf.allDeep("vol") {
		w := parseDec(vol.txt("pesoB"))
		if w.IsZero() {
			w = parseDec(vol.txt("pesoL"))
		}
		c.weightKG = c.weightKG.Add(w)
	}

	// Predominant product: line item with the highest vProd.
	maxVal := decimal.Zero
	for _, det := range inf.allDeep("det") {
		prod := det.child("prod")
		if prod == nil {
			continue
		}
		v := parseDec(prod.txt("vProd"))
		c.totalValue = c.totalValue.Add(v)
		c.items = append(c.items, parsedItem{
			CProd: prod.txt("cProd"),
			XProd: prod.txt("xProd"),
			QCom:  prod.txt("qCom"),
			UCom:  prod.txt("uCom"),
		})
		if v.GreaterThan(maxVal) {
			maxVal = v
			c.predNCM = prod.txt("NCM")
			c.predProd = prod.txt("xProd")
			c.predEAN = prod.txt("cEAN")
		}
	}
	return c, nil
}

func parsePartyNFe(node *xnode, enderTag string) party {
	if node == nil {
		return party{}
	}
	ender := node.child(enderTag)
	return party{
		cnpj: node.txt("CNPJ"),
		cpf:  node.txt("CPF"),
		name: node.txt("xNome"),
		cMun: ender.txt("cMun"),
		xMun: ender.txt("xMun"),
		uf:   ender.txt("UF"),
	}
}

// extractCargoCTe pulls emit/recipient location, cargo value (vCarga) and weight
// (infCarga/infQ qCarga) from a CT-e XML.
func extractCargoCTe(accessKey string, root *xnode) (*docCargo, error) {
	inf := root.firstDeep("infCte")
	if inf == nil {
		return nil, errInvalidDocXML
	}
	c := &docCargo{
		accessKey:  accessKey,
		docType:    docTypeCTe,
		weightKG:   decimal.Zero,
		totalValue: decimal.Zero,
	}
	c.emit = parsePartyCTe(inf.child("emit"))
	// Recipient may be "dest" (CT-e) — fall back to "receb" (recebedor).
	if d := inf.child("dest"); d != nil {
		c.dest = parsePartyCTe(d)
	} else {
		c.dest = parsePartyCTe(inf.child("receb"))
	}

	if vPrest := inf.firstDeep("vCarga"); vPrest != nil {
		c.totalValue = parseDec(trimText(vPrest.text))
	}
	// Cargo weight: infCarga/infQ where the unit code indicates weight in KG.
	for _, infQ := range inf.allDeep("infQ") {
		c.weightKG = c.weightKG.Add(parseDec(infQ.txt("qCarga")))
	}
	if pred := inf.firstDeep("proPred"); pred != nil {
		c.predProd = trimText(pred.text)
	}
	return c, nil
}

func parsePartyCTe(node *xnode) party {
	if node == nil {
		return party{}
	}
	ender := node.child("enderEmit")
	if ender == nil {
		ender = node.child("enderDest")
	}
	if ender == nil {
		ender = node.child("enderReceb")
	}
	return party{
		cnpj: node.txt("CNPJ"),
		cpf:  node.txt("CPF"),
		name: node.txt("xNome"),
		cMun: ender.txt("cMun"),
		xMun: ender.txt("xMun"),
		uf:   ender.txt("UF"),
	}
}

func parseDec(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return v
}
