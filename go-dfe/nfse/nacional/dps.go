package nacional

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
	"gopkg.aoctech.app/dfe/go-dfe/nfse/tables"
)

// Tipos de inscrição federal usados no idDPS (TSIdDPS).
const (
	tpInscCPF  = "1"
	tpInscCNPJ = "2"
)

// Larguras fixas do idDPS.
const (
	widthInscFederal = 14
	widthSerie       = 5
	widthNDPS        = 15
)

// dateTimeUTCLayout formata TSDateTimeUTC (tiposSimples_v1.01.xsd), cujo
// padrão exige TZD numérico (+hh:mm|-hh:mm) e NÃO aceita o sufixo "Z" que
// time.RFC3339 emite para UTC — por isso não usamos time.RFC3339 aqui.
const dateTimeUTCLayout = "2006-01-02T15:04:05-07:00"

// xmlDPS é a raiz do documento (DPS_v1.01.xsd:9 — elemento DPS, tipo TCDPS).
type xmlDPS struct {
	XMLName xml.Name  `xml:"DPS"`
	Xmlns   string    `xml:"xmlns,attr"`
	Versao  string    `xml:"versao,attr"`
	InfDPS  xmlInfDPS `xml:"infDPS"`
}

// xmlInfDPS espelha TCInfDPS — a ordem dos campos É a ordem do XSD.
type xmlInfDPS struct {
	ID        string       `xml:"Id,attr"`
	TpAmb     int          `xml:"tpAmb"`
	DhEmi     string       `xml:"dhEmi"`
	VerAplic  string       `xml:"verAplic"`
	Serie     string       `xml:"serie"`
	NDPS      int          `xml:"nDPS"`
	DCompet   string       `xml:"dCompet"`
	TpEmit    int          `xml:"tpEmit"`
	CMotivo   int          `xml:"cMotivoEmisTI,omitempty"`
	ChNFSeRej string       `xml:"chNFSeRej,omitempty"`
	CLocEmi   string       `xml:"cLocEmi"`
	Subst     *xmlSubst    `xml:"subst,omitempty"`
	Prest     xmlPrestador `xml:"prest"`
	Toma      *xmlPessoa   `xml:"toma,omitempty"`
	Interm    *xmlPessoa   `xml:"interm,omitempty"`
	Serv      xmlServ      `xml:"serv"`
	Valores   xmlValores   `xml:"valores"`
	IBSCBS    *xmlIBSCBS   `xml:"IBSCBS,omitempty"`
}

type xmlSubst struct {
	ChSubstda string `xml:"chSubstda"`
	CMotivo   string `xml:"cMotivo"`
	XMotivo   string `xml:"xMotivo,omitempty"`
}

type xmlPrestador struct {
	CNPJ    string     `xml:"CNPJ,omitempty"`
	CPF     string     `xml:"CPF,omitempty"`
	NIF     string     `xml:"NIF,omitempty"`
	CNaoNIF *int       `xml:"cNaoNIF,omitempty"`
	CAEPF   string     `xml:"CAEPF,omitempty"`
	IM      string     `xml:"IM,omitempty"`
	XNome   string     `xml:"xNome,omitempty"`
	End     *xmlEnd    `xml:"end,omitempty"`
	Fone    string     `xml:"fone,omitempty"`
	Email   string     `xml:"email,omitempty"`
	RegTrib xmlRegTrib `xml:"regTrib"`
}

type xmlPessoa struct {
	CNPJ    string  `xml:"CNPJ,omitempty"`
	CPF     string  `xml:"CPF,omitempty"`
	NIF     string  `xml:"NIF,omitempty"`
	CNaoNIF *int    `xml:"cNaoNIF,omitempty"`
	CAEPF   string  `xml:"CAEPF,omitempty"`
	IM      string  `xml:"IM,omitempty"`
	XNome   string  `xml:"xNome"`
	End     *xmlEnd `xml:"end,omitempty"`
	Fone    string  `xml:"fone,omitempty"`
	Email   string  `xml:"email,omitempty"`
}

type xmlRegTrib struct {
	OpSimpNac   int `xml:"opSimpNac"`
	RegApTribSN int `xml:"regApTribSN,omitempty"`
	RegEspTrib  int `xml:"regEspTrib"`
}

// xmlEnd espelha TCEndereco: escolha endNac|endExt e depois logradouro.
type xmlEnd struct {
	EndNac  *xmlEndNac `xml:"endNac,omitempty"`
	EndExt  *xmlEndExt `xml:"endExt,omitempty"`
	XLgr    string     `xml:"xLgr"`
	Nro     string     `xml:"nro"`
	XCpl    string     `xml:"xCpl,omitempty"`
	XBairro string     `xml:"xBairro"`
}

type xmlEndNac struct {
	CMun string `xml:"cMun"`
	CEP  string `xml:"CEP"`
}

type xmlEndExt struct {
	CPais       string `xml:"cPais"`
	CEndPost    string `xml:"cEndPost,omitempty"`
	XCidade     string `xml:"xCidade"`
	XEstProvReg string `xml:"xEstProvReg,omitempty"`
}

type xmlServ struct {
	LocPrest  xmlLocPrest   `xml:"locPrest"`
	CServ     xmlCServ      `xml:"cServ"`
	ComExt    *xmlComExt    `xml:"comExt,omitempty"`
	Obra      *xmlObra      `xml:"obra,omitempty"`
	AtvEvento *xmlAtvEvento `xml:"atvEvento,omitempty"`
	InfoCompl *xmlInfoCompl `xml:"infoCompl,omitempty"`
}

// xmlLocPrest espelha TCLocPrest: xs:choice, sem nenhum outro campo.
type xmlLocPrest struct {
	CLocPrestacao  string `xml:"cLocPrestacao,omitempty"`
	CPaisPrestacao string `xml:"cPaisPrestacao,omitempty"`
}

type xmlCServ struct {
	CTribNac    string `xml:"cTribNac"`
	CTribMun    string `xml:"cTribMun,omitempty"`
	XDescServ   string `xml:"xDescServ"`
	CNBS        string `xml:"cNBS,omitempty"`
	CIntContrib string `xml:"cIntContrib,omitempty"`
}

// xmlComExt espelha TCComExterior: todos os campos até movTempBens (mais
// mdic) são obrigatórios no XSD (nenhum tem minOccurs="0") — sem omitempty.
// mecAFComexP/T são enums string de 2 dígitos ("00"), nunca int.
type xmlComExt struct {
	MdPrestacao int    `xml:"mdPrestacao"`
	VincPrest   int    `xml:"vincPrest"`
	TpMoeda     string `xml:"tpMoeda"`
	VServMoeda  string `xml:"vServMoeda"`
	MecAFComexP string `xml:"mecAFComexP"`
	MecAFComexT string `xml:"mecAFComexT"`
	MovTempBens int    `xml:"movTempBens"`
	NDI         string `xml:"nDI,omitempty"`
	NRE         string `xml:"nRE,omitempty"`
	MDIC        int    `xml:"mdic"`
}

// xmlObra espelha TCInfoObra: inscImobFisc? seguido da escolha obrigatória
// cObra|cCIB|end.
type xmlObra struct {
	InscImobFisc string         `xml:"inscImobFisc,omitempty"`
	CObra        string         `xml:"cObra,omitempty"`
	CCIB         string         `xml:"cCIB,omitempty"`
	End          *xmlEndSimples `xml:"end,omitempty"`
}

// xmlAtvEvento espelha TCAtvEvento: xNome, dtIni, dtFim obrigatórios,
// seguidos da escolha obrigatória idAtvEvt|end.
type xmlAtvEvento struct {
	XNome    string         `xml:"xNome"`
	DtIni    string         `xml:"dtIni"`
	DtFim    string         `xml:"dtFim"`
	IDAtvEvt string         `xml:"idAtvEvt,omitempty"`
	End      *xmlEndSimples `xml:"end,omitempty"`
}

// xmlEndSimples espelha TCEnderecoSimples/TCEnderObraEvento (mesmo shape):
// escolha CEP|endExt (sem cPais), depois xLgr, nro, xCpl?, xBairro.
type xmlEndSimples struct {
	CEP     string            `xml:"CEP,omitempty"`
	EndExt  *xmlEndExtSimples `xml:"endExt,omitempty"`
	XLgr    string            `xml:"xLgr"`
	Nro     string            `xml:"nro"`
	XCpl    string            `xml:"xCpl,omitempty"`
	XBairro string            `xml:"xBairro"`
}

type xmlEndExtSimples struct {
	CEndPost    string `xml:"cEndPost"`
	XCidade     string `xml:"xCidade"`
	XEstProvReg string `xml:"xEstProvReg"`
}

// xmlInfoCompl espelha TCInfoCompl: idDocTec?, docRef?, xPed?, gItemPed?, xInfComp?.
type xmlInfoCompl struct {
	IDDocTec string          `xml:"idDocTec,omitempty"`
	DocRef   string          `xml:"docRef,omitempty"`
	XPed     string          `xml:"xPed,omitempty"`
	GItemPed *xmlInfoItemPed `xml:"gItemPed,omitempty"`
	XInfComp string          `xml:"xInfComp,omitempty"`
}

type xmlInfoItemPed struct {
	XItemPed []string `xml:"xItemPed"`
}

type xmlValores struct {
	VServPrest      xmlVServPrest      `xml:"vServPrest"`
	VDescCondIncond *xmlDescCondIncond `xml:"vDescCondIncond,omitempty"`
	VDedRed         *xmlDedRed         `xml:"vDedRed,omitempty"`
	Trib            xmlTrib            `xml:"trib"`
}

type xmlVServPrest struct {
	VReceb string `xml:"vReceb,omitempty"`
	VServ  string `xml:"vServ"`
}

type xmlDescCondIncond struct {
	VDescIncond string `xml:"vDescIncond,omitempty"`
	VDescCond   string `xml:"vDescCond,omitempty"`
}

type xmlDedRed struct {
	PDR       string         `xml:"pDR,omitempty"`
	VDR       string         `xml:"vDR,omitempty"`
	DocDedRed []xmlDedRedDoc `xml:"documentos>docDedRed,omitempty"`
}

// xmlDedRedDoc espelha TCDocDedRed: escolha obrigatória
// chNFSe|chNFe|NFSeMun|NFNFS|nDocFisc|nDoc, tpDedRed (obrigatório),
// xDescOutDed?, dtEmiDoc (obrigatório), vDedutivelRedutivel (obrigatório),
// vDeducaoReducao (obrigatório), fornec?.
type xmlDedRedDoc struct {
	ChNFSe              string            `xml:"chNFSe,omitempty"`
	ChNFe               string            `xml:"chNFe,omitempty"`
	NFSeMun             *xmlDocOutNFSeMun `xml:"NFSeMun,omitempty"`
	NFNFS               *xmlDocNFNFS      `xml:"NFNFS,omitempty"`
	NDocFisc            string            `xml:"nDocFisc,omitempty"`
	NDoc                string            `xml:"nDoc,omitempty"`
	TpDedRed            int               `xml:"tpDedRed"`
	XDescOutDed         string            `xml:"xDescOutDed,omitempty"`
	DtEmiDoc            string            `xml:"dtEmiDoc"`
	VDedutivelRedutivel string            `xml:"vDedutivelRedutivel"`
	VDeducaoReducao     string            `xml:"vDeducaoReducao"`
	Fornec              *xmlPessoa        `xml:"fornec,omitempty"`
}

type xmlDocOutNFSeMun struct {
	CMunNFSeMun   string `xml:"cMunNFSeMun"`
	NNFSeMun      string `xml:"nNFSeMun"`
	CVerifNFSeMun string `xml:"cVerifNFSeMun"`
}

type xmlDocNFNFS struct {
	NNFS     string `xml:"nNFS"`
	ModNFS   string `xml:"modNFS"`
	SerieNFS string `xml:"serieNFS"`
}

type xmlTrib struct {
	TribMun xmlTribMun  `xml:"tribMun"`
	TribFed *xmlTribFed `xml:"tribFed,omitempty"`
	TotTrib xmlTotTrib  `xml:"totTrib"`
}

type xmlTribMun struct {
	TribISSQN   int          `xml:"tribISSQN"`
	CPaisResult string       `xml:"cPaisResult,omitempty"`
	TpImunidade *int         `xml:"tpImunidade,omitempty"`
	ExigSusp    *xmlExigSusp `xml:"exigSusp,omitempty"`
	BM          *xmlBenefMun `xml:"BM,omitempty"`
	TpRetISSQN  int          `xml:"tpRetISSQN"`
	PAliq       string       `xml:"pAliq,omitempty"`
}

type xmlExigSusp struct {
	TpSusp    int    `xml:"tpSusp"`
	NProcesso string `xml:"nProcesso"`
}

// xmlBenefMun espelha TCBeneficioMunicipal: nBM obrigatório, seguido da
// escolha vRedBCBM|pRedBCBM.
type xmlBenefMun struct {
	NBM      string `xml:"nBM"`
	VRedBCBM string `xml:"vRedBCBM,omitempty"`
	PRedBCBM string `xml:"pRedBCBM,omitempty"`
}

type xmlTribFed struct {
	PisCofins *xmlPisCofins `xml:"piscofins,omitempty"`
	VRetCP    string        `xml:"vRetCP,omitempty"`
	VRetIRRF  string        `xml:"vRetIRRF,omitempty"`
	VRetCSLL  string        `xml:"vRetCSLL,omitempty"`
}

type xmlPisCofins struct {
	CST            string `xml:"CST"`
	VBCPisCofins   string `xml:"vBCPisCofins,omitempty"`
	PAliqPis       string `xml:"pAliqPis,omitempty"`
	PAliqCofins    string `xml:"pAliqCofins,omitempty"`
	VPis           string `xml:"vPis,omitempty"`
	VCofins        string `xml:"vCofins,omitempty"`
	TpRetPisCofins *int   `xml:"tpRetPisCofins,omitempty"`
}

// xmlTotTrib espelha TCTribTotal: xs:choice de exatamente um dos quatro
// campos — toXMLTotTrib decide qual ramo emitir.
type xmlTotTrib struct {
	VTotTrib   *xmlVTotTrib `xml:"vTotTrib,omitempty"`
	PTotTrib   *xmlPTotTrib `xml:"pTotTrib,omitempty"`
	IndTotTrib *int         `xml:"indTotTrib,omitempty"`
	PTotTribSN string       `xml:"pTotTribSN,omitempty"`
}

type xmlVTotTrib struct {
	VTotTribFed string `xml:"vTotTribFed"`
	VTotTribEst string `xml:"vTotTribEst"`
	VTotTribMun string `xml:"vTotTribMun"`
}

type xmlPTotTrib struct {
	PTotTribFed string `xml:"pTotTribFed"`
	PTotTribEst string `xml:"pTotTribEst"`
	PTotTribMun string `xml:"pTotTribMun"`
}

// BuildIDDPS monta o identificador da DPS (TSIdDPS, 45 caracteres).
func BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string {
	return "DPS" + cLocEmi + tpInsc +
		leftPad(inscFederal, widthInscFederal) +
		leftPad(serie, widthSerie) +
		leftPad(fmt.Sprintf("%d", nDPS), widthNDPS)
}

func leftPad(s string, width int) string {
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat("0", width-len(s)) + s
}

// BuildDPS serializa o modelo neutro na DPS 1.01, ainda SEM assinatura.
// Devolve o XML e o idDPS. now é injetado para tornar o teste determinístico;
// só é usado quando doc.DhEmi está vazio.
func BuildDPS(doc nfse.Document, now time.Time) ([]byte, string, error) {
	if err := validateDoc(doc); err != nil {
		return nil, "", err
	}

	tpInsc, inscFederal := tpInscCNPJ, doc.Prestador.CNPJ
	if doc.Prestador.CPF != "" {
		tpInsc, inscFederal = tpInscCPF, doc.Prestador.CPF
	}
	idDPS := BuildIDDPS(doc.CLocEmi, tpInsc, inscFederal, doc.Serie, doc.Numero)

	dhEmi := doc.DhEmi
	if dhEmi == "" {
		dhEmi = now.UTC().Format(dateTimeUTCLayout)
	}

	ibscbs, err := toXMLIBSCBS(doc.IBSCBS)
	if err != nil {
		return nil, "", err
	}

	inf := xmlInfDPS{
		ID: idDPS, TpAmb: doc.Ambiente, DhEmi: dhEmi, VerAplic: doc.VerAplic,
		Serie: doc.Serie, NDPS: doc.Numero, DCompet: doc.Competencia,
		TpEmit: doc.TpEmit, CMotivo: doc.MotivoEmisTI, ChNFSeRej: doc.ChNFSeRej,
		CLocEmi: doc.CLocEmi,
		Prest:   toXMLPrestador(doc.Prestador),
		Toma:    toXMLPessoa(doc.Tomador),
		Interm:  toXMLPessoa(doc.Intermediario),
		Serv:    toXMLServ(doc.Servico),
		Valores: toXMLValores(doc.Valores),
		IBSCBS:  ibscbs,
	}
	if doc.Substituicao != nil {
		inf.Subst = &xmlSubst{
			ChSubstda: doc.Substituicao.ChSubstda,
			CMotivo:   doc.Substituicao.CMotivo,
			XMotivo:   doc.Substituicao.XMotivo,
		}
	}

	out, err := xml.Marshal(xmlDPS{Xmlns: nfse.Namespace, Versao: nfse.LayoutVersion, InfDPS: inf})
	if err != nil {
		return nil, "", fmt.Errorf("nacional: serializar DPS: %w", err)
	}
	return out, idDPS, nil
}

// validateDoc cobre só as regras estruturais que impedem gerar um XML
// coerente. As regras de negócio fiscais ficam com o Sefin (spec §11).
func validateDoc(doc nfse.Document) error {
	if doc.Prestador.CNPJ == "" && doc.Prestador.CPF == "" && doc.Prestador.NIF == "" {
		return fmt.Errorf("nacional: prestador sem CNPJ, CPF ou NIF")
	}
	if doc.TpEmit != 1 && doc.MotivoEmisTI == 0 {
		return fmt.Errorf("nacional: cMotivoEmisTI é obrigatório quando tpEmit=%d", doc.TpEmit)
	}
	if !tables.IsValidTribNacional(doc.Servico.CServ.CTribNac) {
		return fmt.Errorf("nacional: cTribNac %q não consta no Anexo B", doc.Servico.CServ.CTribNac)
	}
	if c := doc.Servico.CServ.CNBS; c != "" && !tables.IsValidNBS(c) {
		return fmt.Errorf("nacional: cNBS %q não consta na NBS 2.0", c)
	}
	if doc.IBSCBS != nil && !tables.IsValidIndOp(doc.IBSCBS.CIndOp) {
		return fmt.Errorf("nacional: cIndOp %q não consta no Anexo C", doc.IBSCBS.CIndOp)
	}
	return nil
}

func toXMLEnd(e *nfse.Endereco) *xmlEnd {
	if e == nil {
		return nil
	}
	out := &xmlEnd{XLgr: e.XLgr, Nro: e.Nro, XCpl: e.XCpl, XBairro: e.XBairro}
	if e.CMun != "" {
		out.EndNac = &xmlEndNac{CMun: e.CMun, CEP: e.CEP}
	} else {
		out.EndExt = &xmlEndExt{CPais: e.CPais, CEndPost: e.CEndPost,
			XCidade: e.XCidade, XEstProvReg: e.XEstadoProv}
	}
	return out
}

// toXMLEndSimples converte nfse.EnderecoSimples (usado em Obra, AtvEvento e
// IBSCBS.Imovel — TCEnderecoSimples/TCEnderObraEvento, sem cPais).
func toXMLEndSimples(e *nfse.EnderecoSimples) *xmlEndSimples {
	if e == nil {
		return nil
	}
	out := &xmlEndSimples{XLgr: e.XLgr, Nro: e.Nro, XCpl: e.XCpl, XBairro: e.XBairro}
	if e.CEP != "" {
		out.CEP = e.CEP
	} else {
		out.EndExt = &xmlEndExtSimples{CEndPost: e.CEndPost,
			XCidade: e.XCidade, XEstProvReg: e.XEstadoProv}
	}
	return out
}

func toXMLPrestador(p nfse.Prestador) xmlPrestador {
	return xmlPrestador{
		CNPJ: p.CNPJ, CPF: p.CPF, NIF: p.NIF, CNaoNIF: p.CNaoNIF,
		CAEPF: p.CAEPF, IM: p.IM, XNome: p.XNome, End: toXMLEnd(p.End),
		Fone: p.Fone, Email: p.Email,
		RegTrib: xmlRegTrib{OpSimpNac: p.RegTrib.OpSimpNac,
			RegApTribSN: p.RegTrib.RegApTribSN, RegEspTrib: p.RegTrib.RegEspTrib},
	}
}

func toXMLPessoa(p *nfse.Pessoa) *xmlPessoa {
	if p == nil {
		return nil
	}
	return &xmlPessoa{
		CNPJ: p.CNPJ, CPF: p.CPF, NIF: p.NIF, CNaoNIF: p.CNaoNIF,
		CAEPF: p.CAEPF, IM: p.IM, XNome: p.XNome, End: toXMLEnd(p.End),
		Fone: p.Fone, Email: p.Email,
	}
}

func toXMLServ(s nfse.Servico) xmlServ {
	out := xmlServ{
		LocPrest: xmlLocPrest{CLocPrestacao: s.LocPrest.CLocPrestacao,
			CPaisPrestacao: s.LocPrest.CPaisPrestacao},
		CServ: xmlCServ{CTribNac: s.CServ.CTribNac, CTribMun: s.CServ.CTribMun,
			XDescServ: s.CServ.XDescServ, CNBS: s.CServ.CNBS, CIntContrib: s.CServ.CIntContrib},
	}
	if c := s.ComExt; c != nil {
		out.ComExt = &xmlComExt{MdPrestacao: c.MdPrestacao, VincPrest: c.VincPrest,
			TpMoeda: c.TpMoeda, VServMoeda: c.VServMoeda, MecAFComexP: c.MecAFComexP,
			MecAFComexT: c.MecAFComexT, MovTempBens: c.MovTempBens,
			NDI: c.NDI, NRE: c.NRE, MDIC: c.MDIC}
	}
	if o := s.Obra; o != nil {
		out.Obra = &xmlObra{InscImobFisc: o.InscImobFisc, CObra: o.CObra,
			CCIB: o.CCIB, End: toXMLEndSimples(o.End)}
	}
	if a := s.AtvEvento; a != nil {
		out.AtvEvento = &xmlAtvEvento{XNome: a.XNome, DtIni: a.DtIni, DtFim: a.DtFim,
			IDAtvEvt: a.IDAtvEvt, End: toXMLEndSimples(a.End)}
	}
	if i := s.InfoCompl; i != nil {
		ic := &xmlInfoCompl{IDDocTec: i.IDDocTec, DocRef: i.DocRef,
			XPed: i.XPed, XInfComp: i.XInfComp}
		if len(i.ItensPed) > 0 {
			ic.GItemPed = &xmlInfoItemPed{XItemPed: i.ItensPed}
		}
		out.InfoCompl = ic
	}
	return out
}

func toXMLValores(v nfse.Valores) xmlValores {
	out := xmlValores{
		VServPrest: xmlVServPrest{VReceb: v.VServPrest.VReceb, VServ: v.VServPrest.VServ},
		Trib: xmlTrib{TribMun: xmlTribMun{
			TribISSQN: v.Trib.TribMun.TribISSQN, CPaisResult: v.Trib.TribMun.CPaisResult,
			TpImunidade: v.Trib.TribMun.TpImunidade,
			TpRetISSQN:  v.Trib.TribMun.TpRetISSQN, PAliq: v.Trib.TribMun.PAliq,
		}, TotTrib: toXMLTotTrib(v.Trib.TotTrib)},
	}
	if d := v.VDescCondIncond; d != nil {
		out.VDescCondIncond = &xmlDescCondIncond{VDescIncond: d.VDescIncond, VDescCond: d.VDescCond}
	}
	if d := v.VDedRed; d != nil {
		dr := &xmlDedRed{PDR: d.PDR, VDR: d.VDR}
		for _, doc := range d.Documentos {
			ddoc := xmlDedRedDoc{
				ChNFSe: doc.ChNFSe, ChNFe: doc.ChNFe, NDocFisc: doc.NDocFisc,
				NDoc: doc.NDoc, TpDedRed: doc.TpDedRed, XDescOutDed: doc.XDescOutDed,
				DtEmiDoc: doc.DtEmiDoc, VDedutivelRedutivel: doc.VDedutivelRedutivel,
				VDeducaoReducao: doc.VDeducaoReducao,
			}
			if doc.NFSeMun != nil {
				ddoc.NFSeMun = &xmlDocOutNFSeMun{CMunNFSeMun: doc.NFSeMun.CMunNFSeMun,
					NNFSeMun: doc.NFSeMun.NNFSeMun, CVerifNFSeMun: doc.NFSeMun.CVerifNFSeMun}
			}
			if doc.NFNFS != nil {
				ddoc.NFNFS = &xmlDocNFNFS{NNFS: doc.NFNFS.NNFS, ModNFS: doc.NFNFS.ModNFS,
					SerieNFS: doc.NFNFS.SerieNFS}
			}
			if doc.Fornec != nil {
				ddoc.Fornec = toXMLPessoa(doc.Fornec)
			}
			dr.DocDedRed = append(dr.DocDedRed, ddoc)
		}
		out.VDedRed = dr
	}
	if e := v.Trib.TribMun.ExigSusp; e != nil {
		out.Trib.TribMun.ExigSusp = &xmlExigSusp{TpSusp: e.TpSusp, NProcesso: e.NProcesso}
	}
	if b := v.Trib.TribMun.BM; b != nil {
		out.Trib.TribMun.BM = &xmlBenefMun{NBM: b.NBM, VRedBCBM: b.VRedBCBM, PRedBCBM: b.PRedBCBM}
	}
	if f := v.Trib.TribFed; f != nil {
		tf := &xmlTribFed{VRetCP: f.VRetCP, VRetIRRF: f.VRetIRRF, VRetCSLL: f.VRetCSLL}
		if f.CST != "" {
			tf.PisCofins = &xmlPisCofins{CST: f.CST, VBCPisCofins: f.VBCPisCofins,
				PAliqPis: f.PAliqPis, PAliqCofins: f.PAliqCofins,
				VPis: f.VPis, VCofins: f.VCofins, TpRetPisCofins: f.TpRetPisCofins}
		}
		out.Trib.TribFed = tf
	}
	return out
}

// toXMLTotTrib resolve o xs:choice de TCTribTotal: o primeiro ramo com dado
// vence; na ausência de qualquer valor informado, o ramo padrão é
// indTotTrib=0 (Decreto 8.264/2014 — nenhum valor estimado de tributos).
func toXMLTotTrib(t nfse.TotTrib) xmlTotTrib {
	switch {
	case t.VTotTribFed != "":
		return xmlTotTrib{VTotTrib: &xmlVTotTrib{VTotTribFed: t.VTotTribFed,
			VTotTribEst: t.VTotTribEst, VTotTribMun: t.VTotTribMun}}
	case t.PTotTribFed != "":
		return xmlTotTrib{PTotTrib: &xmlPTotTrib{PTotTribFed: t.PTotTribFed,
			PTotTribEst: t.PTotTribEst, PTotTribMun: t.PTotTribMun}}
	case t.PTotTribSN != "":
		return xmlTotTrib{PTotTribSN: t.PTotTribSN}
	default:
		ind := t.IndTotTrib
		return xmlTotTrib{IndTotTrib: &ind}
	}
}
