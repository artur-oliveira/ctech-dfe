package nacional

import (
	"encoding/xml"
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// xmlIBSCBS espelha TCRTCInfoIBSCBS: finNFSe, indFinal?, cIndOp, tpOper?,
// gRefNFSe?, tpEnteGov?, indDest, dest?, imovel?, valores.
type xmlIBSCBS struct {
	XMLName   xml.Name         `xml:"IBSCBS"`
	FinNFSe   int              `xml:"finNFSe"`
	IndFinal  *int             `xml:"indFinal,omitempty"`
	CIndOp    string           `xml:"cIndOp"`
	TpOper    int              `xml:"tpOper,omitempty"`
	GRefNFSe  *xmlRefNFSe      `xml:"gRefNFSe,omitempty"`
	TpEnteGov int              `xml:"tpEnteGov,omitempty"`
	IndDest   int              `xml:"indDest"`
	Dest      *xmlRTCDest      `xml:"dest,omitempty"`
	Imovel    *xmlImovel       `xml:"imovel,omitempty"`
	Valores   xmlIBSCBSValores `xml:"valores"`
}

// xmlRTCDest espelha TCRTCInfoDest: escolha CNPJ|CPF|NIF+cNaoNIF, xNome,
// end?(TCEndereco completo), fone?, email? — SEM CAEPF nem IM, diferente de
// TCInfoPessoa (usado em toma/interm/fornec).
type xmlRTCDest struct {
	CNPJ    string  `xml:"CNPJ,omitempty"`
	CPF     string  `xml:"CPF,omitempty"`
	NIF     string  `xml:"NIF,omitempty"`
	CNaoNIF *int    `xml:"cNaoNIF,omitempty"`
	XNome   string  `xml:"xNome"`
	End     *xmlEnd `xml:"end,omitempty"`
	Fone    string  `xml:"fone,omitempty"`
	Email   string  `xml:"email,omitempty"`
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

// xmlIBSCBSValores espelha TCRTCInfoValoresIBSCBS: gReeRepRes? seguido de trib.
type xmlIBSCBSValores struct {
	GReeRepRes *xmlReeRepRes `xml:"gReeRepRes,omitempty"`
	Trib       xmlTribIBSCBS `xml:"trib"`
}

// xmlReeRepRes espelha TCRTCInfoReeRepRes: documentos repetível (1..1000).
type xmlReeRepRes struct {
	Documentos []xmlReeRepResDoc `xml:"documentos"`
}

// xmlReeRepResDoc espelha TCRTCListaDoc — a ordem dos campos É a ordem do XSD.
type xmlReeRepResDoc struct {
	DFeNacional    *xmlReeRepResDFe        `xml:"dFeNacional,omitempty"`
	DocFiscalOutro *xmlReeRepResDocFiscal  `xml:"docFiscalOutro,omitempty"`
	DocOutro       *xmlReeRepResDocOutro   `xml:"docOutro,omitempty"`
	Fornec         *xmlReeRepResFornecedor `xml:"fornec,omitempty"`
	DtEmiDoc       string                  `xml:"dtEmiDoc"`
	DtCompDoc      string                  `xml:"dtCompDoc"`
	TpReeRepRes    string                  `xml:"tpReeRepRes"`
	XTpReeRepRes   string                  `xml:"xTpReeRepRes,omitempty"`
	VlrReeRepRes   string                  `xml:"vlrReeRepRes"`
}

type xmlReeRepResDFe struct {
	TipoChaveDFe  string `xml:"tipoChaveDFe"`
	XTipoChaveDFe string `xml:"xTipoChaveDFe,omitempty"`
	ChaveDFe      string `xml:"chaveDFe"`
}

type xmlReeRepResDocFiscal struct {
	CMunDocFiscal string `xml:"cMunDocFiscal"`
	NDocFiscal    string `xml:"nDocFiscal"`
	XDocFiscal    string `xml:"xDocFiscal"`
}

type xmlReeRepResDocOutro struct {
	NDoc string `xml:"nDoc"`
	XDoc string `xml:"xDoc"`
}

// xmlReeRepResFornecedor espelha TCRTCListaDocFornec — só a escolha de
// identificação e o nome; nunca endereço, CAEPF ou IM.
type xmlReeRepResFornecedor struct {
	CNPJ    string `xml:"CNPJ,omitempty"`
	CPF     string `xml:"CPF,omitempty"`
	NIF     string `xml:"NIF,omitempty"`
	CNaoNIF *int   `xml:"cNaoNIF,omitempty"`
	XNome   string `xml:"xNome"`
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

func toXMLIBSCBS(g *nfse.IBSCBS) (*xmlIBSCBS, error) {
	if g == nil {
		return nil, nil
	}
	out := &xmlIBSCBS{
		FinNFSe: g.FinNFSe, IndFinal: g.IndFinal, CIndOp: g.CIndOp,
		TpOper: g.TpOper, TpEnteGov: g.TpEnteGov, IndDest: g.IndDest,
		Valores: xmlIBSCBSValores{
			Trib: xmlTribIBSCBS{GIBSCBS: xmlInfoTributosSitClas{
				CST: g.Valores.Trib.CST, CClassTrib: g.Valores.Trib.CClassTrib,
				CCredPres: g.Valores.Trib.CCredPres,
			}},
		},
	}
	if g.Dest != nil {
		if g.Dest.CAEPF != "" || g.Dest.IM != "" {
			return nil, fmt.Errorf("nacional: TCRTCInfoDest não tem CAEPF nem IM (só TCInfoPessoa tem)")
		}
		out.Dest = &xmlRTCDest{
			CNPJ: g.Dest.CNPJ, CPF: g.Dest.CPF, NIF: g.Dest.NIF, CNaoNIF: g.Dest.CNaoNIF,
			XNome: g.Dest.XNome, End: toXMLEnd(g.Dest.End), Fone: g.Dest.Fone, Email: g.Dest.Email,
		}
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
	if docs := toXMLReeRepRes(g.Valores.ReeRepRes); docs != nil {
		out.Valores.GReeRepRes = docs
	}
	if d := g.Valores.Trib.Dif; d != nil {
		out.Valores.Trib.GIBSCBS.GDif = &xmlDiferimentoIBSCBS{
			PDifUF: d.PDifUF, PDifMun: d.PDifMun, PDifCBS: d.PDifCBS}
	}
	return out, nil
}

// toXMLReeRepRes converte a lista neutra de reembolso/repasse/ressarcimento.
// O grupo inteiro é omitido quando não há documento, porque documentos é
// obrigatório dentro de gReeRepRes (minOccurs=1).
func toXMLReeRepRes(docs []nfse.ReeRepResDoc) *xmlReeRepRes {
	if len(docs) == 0 {
		return nil
	}
	out := &xmlReeRepRes{Documentos: make([]xmlReeRepResDoc, 0, len(docs))}
	for _, doc := range docs {
		item := xmlReeRepResDoc{
			DtEmiDoc: doc.DtEmiDoc, DtCompDoc: doc.DtCompDoc,
			TpReeRepRes: doc.TpReeRepRes, XTpReeRepRes: doc.XTpReeRepRes,
			VlrReeRepRes: doc.VlrReeRepRes,
		}
		if d := doc.DFeNacional; d != nil {
			item.DFeNacional = &xmlReeRepResDFe{
				TipoChaveDFe: d.TipoChaveDFe, XTipoChaveDFe: d.XTipoChaveDFe, ChaveDFe: d.ChaveDFe}
		}
		if d := doc.DocFiscalOutro; d != nil {
			item.DocFiscalOutro = &xmlReeRepResDocFiscal{
				CMunDocFiscal: d.CMunDocFiscal, NDocFiscal: d.NDocFiscal, XDocFiscal: d.XDocFiscal}
		}
		if d := doc.DocOutro; d != nil {
			item.DocOutro = &xmlReeRepResDocOutro{NDoc: d.NDoc, XDoc: d.XDoc}
		}
		if f := doc.Fornec; f != nil {
			item.Fornec = &xmlReeRepResFornecedor{
				CNPJ: f.CNPJ, CPF: f.CPF, NIF: f.NIF, CNaoNIF: f.CNaoNIF, XNome: f.XNome}
		}
		out.Documentos = append(out.Documentos, item)
	}
	return out
}
