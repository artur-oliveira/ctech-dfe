// Package nfexml lê o XML autorizado de documentos fiscais já emitidos ou recebidos (NF-e, CT-e) para quem
// deriva outro documento deles: o MDF-e (carga do manifesto) e o CT-e (notas transportadas). A árvore ignora
// namespace (casa só o nome local), então o mesmo leitor serve NF-e e CT-e.
package nfexml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/shopspring/decimal"
)

// ErrNotNFe: o XML não traz infNFe.
var ErrNotNFe = errors.New("nfexml: infNFe ausente")

// Node é um nó mínimo da árvore XML.
type Node struct {
	Name     string
	Text     string
	Children []*Node
}

// Parse decodifica o XML numa árvore de Node.
func Parse(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var root *Node
	var stack []*Node
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
			n := &Node{Name: t.Name.Local}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, n)
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
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	return root, nil
}

// Child devolve o primeiro filho direto com o nome, ou nil.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// FirstDeep devolve o primeiro descendente (profundidade primeiro) com o nome.
func (n *Node) FirstDeep(name string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
		if found := c.FirstDeep(name); found != nil {
			return found
		}
	}
	return nil
}

// AllDeep devolve todos os descendentes com o nome.
func (n *Node) AllDeep(name string) []*Node {
	var out []*Node
	if n == nil {
		return out
	}
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
		out = append(out, c.AllDeep(name)...)
	}
	return out
}

// Txt devolve o texto aparado do filho direto, ou "".
func (n *Node) Txt(name string) string {
	c := n.Child(name)
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Text)
}

// Value devolve o texto aparado do próprio nó ("" para nil).
func (n *Node) Value() string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Text)
}

// Dec lê um decimal do XML; vazio ou inválido é zero.
func Dec(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return v
}

// Address é o endereço de um ator (enderEmit/enderDest).
type Address struct {
	Street, Number, Complement, District string
	CityIBGE, City, UF, CEP              string
	CountryCode, Country, Phone          string
}

// Party é emitente ou destinatário de uma NF-e.
type Party struct {
	CNPJ, CPF, IE, Name, Email string
	Addr                       Address
}

// Doc devolve o CNPJ ou, na falta, o CPF.
func (p Party) Doc() string {
	if p.CNPJ != "" {
		return p.CNPJ
	}
	return p.CPF
}

// Item é um det/prod da NF-e.
type Item struct {
	CProd, XProd, NCM, CEAN, QCom, UCom string
	VProd                               decimal.Decimal
}

// Volume é um transp/vol.
type Volume struct {
	QVol         int64
	PesoB, PesoL decimal.Decimal
}

// NFe é o que os documentos derivados leem de uma NF-e autorizada.
type NFe struct {
	Model, Number, Serie, IssuedAt, ModFrete string
	Emit, Dest                               Party
	Items                                    []Item
	Volumes                                  []Volume
	// VNF é total/ICMSTot/vNF (zero se ausente).
	VNF decimal.Decimal
}

// ParseNFe lê uma NF-e (ou nfeProc) autorizada.
func ParseNFe(data []byte) (*NFe, error) {
	root, err := Parse(data)
	if err != nil {
		return nil, err
	}
	inf := root.FirstDeep("infNFe")
	if inf == nil {
		return nil, ErrNotNFe
	}
	ide := inf.Child("ide")
	out := &NFe{
		Model:    ide.Txt("mod"),
		Number:   ide.Txt("nNF"),
		Serie:    ide.Txt("serie"),
		IssuedAt: ide.Txt("dhEmi"),
		ModFrete: inf.Child("transp").Txt("modFrete"),
		Emit:     parseParty(inf.Child("emit"), "enderEmit"),
		Dest:     parseParty(inf.Child("dest"), "enderDest"),
		VNF:      Dec(inf.Child("total").Child("ICMSTot").Txt("vNF")),
	}
	for _, vol := range inf.AllDeep("vol") {
		q := Dec(vol.Txt("qVol"))
		out.Volumes = append(out.Volumes, Volume{QVol: q.IntPart(), PesoB: Dec(vol.Txt("pesoB")), PesoL: Dec(vol.Txt("pesoL"))})
	}
	for _, det := range inf.AllDeep("det") {
		prod := det.Child("prod")
		if prod == nil {
			continue
		}
		out.Items = append(out.Items, Item{
			CProd: prod.Txt("cProd"), XProd: prod.Txt("xProd"), NCM: prod.Txt("NCM"), CEAN: prod.Txt("cEAN"),
			QCom: prod.Txt("qCom"), UCom: prod.Txt("uCom"), VProd: Dec(prod.Txt("vProd")),
		})
	}
	return out, nil
}

func parseParty(node *Node, enderTag string) Party {
	if node == nil {
		return Party{}
	}
	e := node.Child(enderTag)
	return Party{
		CNPJ: node.Txt("CNPJ"), CPF: node.Txt("CPF"), IE: node.Txt("IE"), Name: node.Txt("xNome"), Email: node.Txt("email"),
		Addr: Address{
			Street: e.Txt("xLgr"), Number: e.Txt("nro"), Complement: e.Txt("xCpl"), District: e.Txt("xBairro"),
			CityIBGE: e.Txt("cMun"), City: e.Txt("xMun"), UF: e.Txt("UF"), CEP: e.Txt("CEP"),
			CountryCode: e.Txt("cPais"), Country: e.Txt("xPais"), Phone: e.Txt("fone"),
		},
	}
}
