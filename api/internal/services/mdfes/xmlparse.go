package mdfes

import (
	"errors"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
)

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
	if docType == docTypeCTe {
		root, err := nfexml.Parse(xmlData)
		if err != nil {
			return nil, err
		}
		return extractCargoCTe(accessKey, root)
	}
	nfe, err := nfexml.ParseNFe(xmlData)
	if errors.Is(err, nfexml.ErrNotNFe) {
		return nil, errInvalidDocXML
	}
	if err != nil {
		return nil, err
	}
	return extractCargoNFe(accessKey, nfe), nil
}

// extractCargoNFe pulls emit/dest location, gross weight (transp/vol/pesoB) and
// the predominant product (highest vProd line item) from an NF-e.
func extractCargoNFe(accessKey string, nfe *nfexml.NFe) *docCargo {
	c := &docCargo{
		accessKey:  accessKey,
		docType:    docTypeNFe,
		emit:       partyOf(nfe.Emit),
		dest:       partyOf(nfe.Dest),
		weightKG:   decimal.Zero,
		totalValue: decimal.Zero,
	}
	// Gross weight: sum of transp/vol/pesoB; fall back to pesoL when pesoB absent.
	for _, vol := range nfe.Volumes {
		w := vol.PesoB
		if w.IsZero() {
			w = vol.PesoL
		}
		c.weightKG = c.weightKG.Add(w)
	}
	// Predominant product: line item with the highest vProd.
	maxVal := decimal.Zero
	for _, it := range nfe.Items {
		c.totalValue = c.totalValue.Add(it.VProd)
		c.items = append(c.items, parsedItem{CProd: it.CProd, XProd: it.XProd, QCom: it.QCom, UCom: it.UCom})
		if it.VProd.GreaterThan(maxVal) {
			maxVal = it.VProd
			c.predNCM, c.predProd, c.predEAN = it.NCM, it.XProd, it.CEAN
		}
	}
	return c
}

func partyOf(p nfexml.Party) party {
	return party{cnpj: p.CNPJ, cpf: p.CPF, name: p.Name, cMun: p.Addr.CityIBGE, xMun: p.Addr.City, uf: p.Addr.UF}
}

// extractCargoCTe pulls emit/recipient location, cargo value (vCarga) and weight
// (infCarga/infQ qCarga) from a CT-e XML.
func extractCargoCTe(accessKey string, root *nfexml.Node) (*docCargo, error) {
	inf := root.FirstDeep("infCte")
	if inf == nil {
		return nil, errInvalidDocXML
	}
	c := &docCargo{
		accessKey:  accessKey,
		docType:    docTypeCTe,
		weightKG:   decimal.Zero,
		totalValue: decimal.Zero,
	}
	c.emit = parsePartyCTe(inf.Child("emit"))
	// Recipient may be "dest" (CT-e) — fall back to "receb" (recebedor).
	if d := inf.Child("dest"); d != nil {
		c.dest = parsePartyCTe(d)
	} else {
		c.dest = parsePartyCTe(inf.Child("receb"))
	}

	if vPrest := inf.FirstDeep("vCarga"); vPrest != nil {
		c.totalValue = parseDec(vPrest.Value())
	}
	// Cargo weight: infCarga/infQ where the unit code indicates weight in KG.
	for _, infQ := range inf.AllDeep("infQ") {
		c.weightKG = c.weightKG.Add(parseDec(infQ.Txt("qCarga")))
	}
	if pred := inf.FirstDeep("proPred"); pred != nil {
		c.predProd = pred.Value()
	}
	return c, nil
}

func parsePartyCTe(node *nfexml.Node) party {
	if node == nil {
		return party{}
	}
	ender := node.Child("enderEmit")
	if ender == nil {
		ender = node.Child("enderDest")
	}
	if ender == nil {
		ender = node.Child("enderReceb")
	}
	return party{
		cnpj: node.Txt("CNPJ"),
		cpf:  node.Txt("CPF"),
		name: node.Txt("xNome"),
		cMun: ender.Txt("cMun"),
		xMun: ender.Txt("xMun"),
		uf:   ender.Txt("UF"),
	}
}

func parseDec(s string) decimal.Decimal { return nfexml.Dec(s) }
