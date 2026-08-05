package nacional

import (
	"encoding/xml"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// xmlIBSCBS espelha TCRTCInfoIBSCBS: finNFSe, indFinal?, cIndOp, tpOper?,
// gRefNFSe?, tpEnteGov?, indDest, dest?, imovel?, valores.
type xmlIBSCBS struct {
	XMLName   xml.Name         `xml:"IBSCBS"`
	FinNFSe   int              `xml:"finNFSe"`
	IndFinal  int              `xml:"indFinal,omitempty"`
	CIndOp    string           `xml:"cIndOp"`
	TpOper    int              `xml:"tpOper,omitempty"`
	GRefNFSe  *xmlRefNFSe      `xml:"gRefNFSe,omitempty"`
	TpEnteGov int              `xml:"tpEnteGov,omitempty"`
	IndDest   int              `xml:"indDest"`
	Dest      *xmlPessoa       `xml:"dest,omitempty"`
	Imovel    *xmlImovel       `xml:"imovel,omitempty"`
	Valores   xmlIBSCBSValores `xml:"valores"`
}

type xmlRefNFSe struct {
	ChNFSe string `xml:"chNFSe"`
}

type xmlImovel struct {
	CIB          string `xml:"cIB,omitempty"`
	InscImobFisc string `xml:"inscImobFisc,omitempty"`
	CMun         string `xml:"cMun,omitempty"`
}

type xmlIBSCBSValores struct {
	CST        string            `xml:"CST"`
	CClassTrib string            `xml:"cClassTrib"`
	VBC        string            `xml:"vBC,omitempty"`
	GIBSUF     *xmlIBSComponente `xml:"gIBSUF,omitempty"`
	GIBSMun    *xmlIBSComponente `xml:"gIBSMun,omitempty"`
	GCBS       *xmlIBSComponente `xml:"gCBS,omitempty"`
	GDif       *xmlDiferimento   `xml:"gDif,omitempty"`
	GCredPres  *xmlCredPresumido `xml:"gCredPres,omitempty"`
	VTotIBS    string            `xml:"vTotIBS,omitempty"`
	VTotCBS    string            `xml:"vTotCBS,omitempty"`
}

type xmlIBSComponente struct {
	PAliq    string `xml:"pAliq,omitempty"`
	PRedAliq string `xml:"pRedAliq,omitempty"`
	VTribOp  string `xml:"vTribOp,omitempty"`
	VTrib    string `xml:"vTrib,omitempty"`
}

type xmlDiferimento struct {
	PDif string `xml:"pDif,omitempty"`
	VDif string `xml:"vDif,omitempty"`
}

type xmlCredPresumido struct {
	CCredPres string `xml:"cCredPres,omitempty"`
	PCredPres string `xml:"pCredPres,omitempty"`
	VCredPres string `xml:"vCredPres,omitempty"`
}

func toXMLIBSCBS(g *nfse.IBSCBS) *xmlIBSCBS {
	if g == nil {
		return nil
	}
	out := &xmlIBSCBS{
		FinNFSe: g.FinNFSe, IndFinal: g.IndFinal, CIndOp: g.CIndOp,
		TpOper: g.TpOper, TpEnteGov: g.TpEnteGov, IndDest: g.IndDest,
		Dest: toXMLPessoa(g.Dest),
		Valores: xmlIBSCBSValores{
			CST: g.Valores.CST, CClassTrib: g.Valores.CClassTrib, VBC: g.Valores.VBC,
			GIBSUF: toXMLComponente(g.Valores.GIBSUF), GIBSMun: toXMLComponente(g.Valores.GIBSMun),
			GCBS:    toXMLComponente(g.Valores.GCBS),
			VTotIBS: g.Valores.VTotIBS, VTotCBS: g.Valores.VTotCBS,
		},
	}
	if g.GRefNFSe != nil {
		out.GRefNFSe = &xmlRefNFSe{ChNFSe: g.GRefNFSe.ChNFSe}
	}
	if g.Imovel != nil {
		out.Imovel = &xmlImovel{CIB: g.Imovel.CIB, InscImobFisc: g.Imovel.InscImobFisc, CMun: g.Imovel.CMun}
	}
	if d := g.Valores.GDif; d != nil {
		out.Valores.GDif = &xmlDiferimento{PDif: d.PDif, VDif: d.VDif}
	}
	if c := g.Valores.GCredPres; c != nil {
		out.Valores.GCredPres = &xmlCredPresumido{CCredPres: c.CCredPres,
			PCredPres: c.PCredPres, VCredPres: c.VCredPres}
	}
	return out
}

func toXMLComponente(c *nfse.IBSComponente) *xmlIBSComponente {
	if c == nil {
		return nil
	}
	return &xmlIBSComponente{PAliq: c.PAliq, PRedAliq: c.PRedAliq,
		VTribOp: c.VTribOp, VTrib: c.VTrib}
}
