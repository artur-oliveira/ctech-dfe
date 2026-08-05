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

// xmlRefNFSe espelha TCInfoRefNFSe: refNFSe é repetível (até 99).
type xmlRefNFSe struct {
	RefNFSe []string `xml:"refNFSe"`
}

// xmlImovel espelha TCRTCInfoImovel: inscImobFisc? seguido da escolha
// obrigatória cCIB|end.
type xmlImovel struct {
	InscImobFisc string         `xml:"inscImobFisc,omitempty"`
	CCIB         string         `xml:"cCIB,omitempty"`
	End          *xmlEndSimples `xml:"end,omitempty"`
}

// xmlIBSCBSValores espelha TCRTCInfoValoresIBSCBS. gReeRepRes não é emitido
// nesta fase (ver nfse.IBSCBSValores).
type xmlIBSCBSValores struct {
	Trib xmlTribIBSCBS `xml:"trib"`
}

// xmlTribIBSCBS espelha TCRTCInfoTributosIBSCBS>gIBSCBS (TCRTCInfoTributosSitClas).
type xmlTribIBSCBS struct {
	GIBSCBS xmlInfoTributosSitClas `xml:"gIBSCBS"`
}

type xmlInfoTributosSitClas struct {
	CST          string                `xml:"CST"`
	CClassTrib   string                `xml:"cClassTrib"`
	CCredPres    string                `xml:"cCredPres,omitempty"`
	GTribRegular *xmlTribRegular       `xml:"gTribRegular,omitempty"`
	GDif         *xmlDiferimentoIBSCBS `xml:"gDif,omitempty"`
}

type xmlTribRegular struct {
	CSTReg        string `xml:"CSTReg"`
	CClassTribReg string `xml:"cClassTribReg"`
}

// xmlDiferimentoIBSCBS espelha TCRTCInfoTributosDif — três percentuais obrigatórios.
type xmlDiferimentoIBSCBS struct {
	PDifUF  string `xml:"pDifUF"`
	PDifMun string `xml:"pDifMun"`
	PDifCBS string `xml:"pDifCBS"`
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
			Trib: xmlTribIBSCBS{GIBSCBS: xmlInfoTributosSitClas{
				CST: g.Valores.Trib.CST, CClassTrib: g.Valores.Trib.CClassTrib,
				CCredPres: g.Valores.Trib.CCredPres,
			}},
		},
	}
	if g.GRefNFSe != nil {
		out.GRefNFSe = &xmlRefNFSe{RefNFSe: g.GRefNFSe.Chaves}
	}
	if g.Imovel != nil {
		out.Imovel = &xmlImovel{InscImobFisc: g.Imovel.InscImobFisc,
			CCIB: g.Imovel.CIB, End: toXMLEndSimples(g.Imovel.End)}
	}
	if r := g.Valores.Trib.TribRegular; r != nil {
		out.Valores.Trib.GIBSCBS.GTribRegular = &xmlTribRegular{
			CSTReg: r.CSTReg, CClassTribReg: r.CClassTribReg}
	}
	if d := g.Valores.Trib.Dif; d != nil {
		out.Valores.Trib.GIBSCBS.GDif = &xmlDiferimentoIBSCBS{
			PDifUF: d.PDifUF, PDifMun: d.PDifMun, PDifCBS: d.PDifCBS}
	}
	return out
}
