package nfse

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Document é o modelo neutro de emissão, moldado no DPS 1.01.
type Document struct {
	Ambiente     int    `json:"ambiente"`                 // TSTipoAmbiente: 1 produção, 2 homologação
	VerAplic     string `json:"ver_aplic"`                // identificação do nosso aplicativo
	TpEmit       int    `json:"tp_emit"`                  // 1 prestador, 2 tomador, 3 intermediário
	MotivoEmisTI int    `json:"motivo_emis_ti,omitempty"` // obrigatório quando TpEmit != 1
	ChNFSeRej    string `json:"ch_nfse_rej,omitempty"`    // TpEmit != 1 e motivo == 4
	DhEmi        string `json:"dh_emi,omitempty"`         // RFC3339 UTC; vazio = agora
	Competencia  string `json:"competencia"`              // AAAA-MM-DD
	Serie        string `json:"serie"`
	Numero       int    `json:"numero"`
	CLocEmi      string `json:"c_loc_emi"` // IBGE 7 dígitos

	Substituicao  *Substituicao `json:"substituicao,omitempty"`
	Prestador     Prestador     `json:"prestador"`
	Tomador       *Pessoa       `json:"tomador,omitempty"`
	Intermediario *Pessoa       `json:"intermediario,omitempty"`
	Servico       Servico       `json:"servico"`
	Valores       Valores       `json:"valores"`
	IBSCBS        *IBSCBS       `json:"ibs_cbs,omitempty"`
}

type Substituicao struct {
	ChSubstda string `json:"ch_substda"`
	CMotivo   string `json:"c_motivo"`
	XMotivo   string `json:"x_motivo,omitempty"`
}

type Pessoa struct {
	CNPJ    string    `json:"cnpj,omitempty"`
	CPF     string    `json:"cpf,omitempty"`
	NIF     string    `json:"nif,omitempty"`
	CNaoNIF int       `json:"c_nao_nif,omitempty"`
	CAEPF   string    `json:"caepf,omitempty"`
	IM      string    `json:"im,omitempty"`
	XNome   string    `json:"x_nome,omitempty"`
	End     *Endereco `json:"endereco,omitempty"`
	Fone    string    `json:"fone,omitempty"`
	Email   string    `json:"email,omitempty"`
}

type Prestador struct {
	Pessoa  `json:",inline"`
	RegTrib RegTrib `json:"reg_trib"`
}

type RegTrib struct {
	OpSimpNac   int `json:"op_simp_nac"`
	RegApTribSN int `json:"reg_ap_trib_sn,omitempty"`
	RegEspTrib  int `json:"reg_esp_trib"`
}

// Endereco é a escolha endNac|endExt do TCEndereco, achatada: CMun preenchido
// significa endereço nacional; CPais preenchido significa exterior.
type Endereco struct {
	CMun        string `json:"c_mun,omitempty"`
	CEP         string `json:"cep,omitempty"`
	CPais       string `json:"c_pais,omitempty"`
	CEndPost    string `json:"c_end_post,omitempty"`
	XCidade     string `json:"x_cidade,omitempty"`
	XEstadoProv string `json:"x_estado_prov,omitempty"`
	XLgr        string `json:"x_lgr"`
	Nro         string `json:"nro"`
	XCpl        string `json:"x_cpl,omitempty"`
	XBairro     string `json:"x_bairro"`
}

type Servico struct {
	LocPrest  LocPrest   `json:"loc_prest"`
	CServ     CServ      `json:"c_serv"`
	ComExt    *ComExt    `json:"com_ext,omitempty"`
	Obra      *Obra      `json:"obra,omitempty"`
	AtvEvento *AtvEvento `json:"atv_evento,omitempty"`
	InfoCompl *InfoCompl `json:"info_compl,omitempty"`
}

type LocPrest struct {
	CLocPrestacao  string `json:"c_loc_prestacao,omitempty"`
	CPaisPrestacao string `json:"c_pais_prestacao,omitempty"`
	OpConsumServ   int    `json:"op_consum_serv,omitempty"`
}

type CServ struct {
	CTribNac    string `json:"c_trib_nac"`
	CTribMun    string `json:"c_trib_mun,omitempty"`
	XDescServ   string `json:"x_desc_serv"`
	CNBS        string `json:"c_nbs,omitempty"`
	CIntContrib string `json:"c_int_contrib,omitempty"`
}

type ComExt struct {
	MdPrestacao     int    `json:"md_prestacao"`
	VincPrest       int    `json:"vinc_prest"`
	TpMoeda         string `json:"tp_moeda"`
	VServMoeda      string `json:"v_serv_moeda"`
	MecAFComexP     int    `json:"mec_af_comex_p,omitempty"`
	MecAFComexT     int    `json:"mec_af_comex_t,omitempty"`
	MovTempBens     int    `json:"mov_temp_bens,omitempty"`
	NDI             string `json:"n_di,omitempty"`
	NRE             string `json:"n_re,omitempty"`
	MdicMovTempBens string `json:"mdic,omitempty"`
}

// Obra espelha TCInfoObra: inscImobFisc? seguido da escolha obrigatória
// cObra|cCIB|end (TCEnderObraEvento).
type Obra struct {
	InscImobFisc string           `json:"insc_imob_fisc,omitempty"`
	CObra        string           `json:"c_obra,omitempty"`
	CCIB         string           `json:"c_cib,omitempty"`
	End          *EnderecoSimples `json:"endereco,omitempty"`
}

// AtvEvento espelha TCAtvEvento: xNome, dtIni e dtFim são obrigatórios;
// idAtvEvt|end (TCEnderecoSimples) é uma escolha obrigatória.
type AtvEvento struct {
	XNome    string           `json:"x_nome"`
	DtIni    string           `json:"dt_ini"`
	DtFim    string           `json:"dt_fim"`
	IDAtvEvt string           `json:"id_atv_evt,omitempty"`
	End      *EnderecoSimples `json:"endereco,omitempty"`
}

// EnderecoSimples espelha TCEnderecoSimples/TCEnderObraEvento (mesmo shape,
// nomes de tipo diferentes no XSD): escolha CEP|endExt (sem cPais, diferente
// do TCEndereco completo), depois xLgr, nro, xCpl?, xBairro.
type EnderecoSimples struct {
	CEP         string `json:"cep,omitempty"`
	CEndPost    string `json:"c_end_post,omitempty"`
	XCidade     string `json:"x_cidade,omitempty"`
	XEstadoProv string `json:"x_estado_prov,omitempty"`
	XLgr        string `json:"x_lgr"`
	Nro         string `json:"nro"`
	XCpl        string `json:"x_cpl,omitempty"`
	XBairro     string `json:"x_bairro"`
}

// InfoCompl espelha TCInfoCompl: idDocTec?, docRef?, xPed?, gItemPed?, xInfComp?.
type InfoCompl struct {
	IDDocTec string   `json:"id_doc_tec,omitempty"`
	DocRef   string   `json:"doc_ref,omitempty"`
	XPed     string   `json:"x_ped,omitempty"`
	ItensPed []string `json:"itens_ped,omitempty"` // gItemPed>xItemPed, até 99
	XInfComp string   `json:"x_inf_comp,omitempty"`
}

type Valores struct {
	VServPrest      VServPrest      `json:"v_serv_prest"`
	VDescCondIncond *DescCondIncond `json:"v_desc_cond_incond,omitempty"`
	VDedRed         *DedRed         `json:"v_ded_red,omitempty"`
	Trib            Tributacao      `json:"trib"`
}

type VServPrest struct {
	VReceb string `json:"v_receb,omitempty"`
	VServ  string `json:"v_serv"`
}

type DescCondIncond struct {
	VDescIncond string `json:"v_desc_incond,omitempty"`
	VDescCond   string `json:"v_desc_cond,omitempty"`
}

type DedRed struct {
	PDR        string      `json:"p_dr,omitempty"`
	VDR        string      `json:"v_dr,omitempty"`
	Documentos []DedRedDoc `json:"documentos,omitempty"`
}

type DedRedDoc struct {
	ChNFSe   string `json:"ch_nfse,omitempty"`
	ChNFe    string `json:"ch_nfe,omitempty"`
	NDocFisc string `json:"n_doc_fisc,omitempty"`
	NDoc     string `json:"n_doc,omitempty"`
	TpDedRed int    `json:"tp_ded_red,omitempty"`
	VDedRed  string `json:"v_ded_red,omitempty"`
	DtEmiDoc string `json:"dt_emi_doc,omitempty"`
}

// Tributacao espelha TCInfoTributacao: tribMun e totTrib são obrigatórios
// (TCTribTotal é sempre uma escolha, nunca ausente — mesmo quando o único
// dado é indTotTrib=0, valor fixo do Decreto 8.264/2014).
type Tributacao struct {
	TribMun TribMunicipal `json:"trib_mun"`
	TribFed *TribFederal  `json:"trib_fed,omitempty"`
	TotTrib TotTrib       `json:"tot_trib"`
}

type TribMunicipal struct {
	TribISSQN   int       `json:"trib_issqn"`
	CPaisResult string    `json:"c_pais_result,omitempty"`
	TpImunidade int       `json:"tp_imunidade,omitempty"`
	ExigSusp    *ExigSusp `json:"exig_susp,omitempty"`
	BM          *BenefMun `json:"bm,omitempty"`
	TpRetISSQN  int       `json:"tp_ret_issqn"`
	PAliq       string    `json:"p_aliq,omitempty"`
}

type ExigSusp struct {
	TpSusp    int    `json:"tp_susp"`
	NProcesso string `json:"n_processo"`
}

// BenefMun espelha TCBeneficioMunicipal: nBM obrigatório, seguido da escolha
// vRedBCBM|pRedBCBM.
type BenefMun struct {
	NBM      string `json:"n_bm"`
	VRedBCBM string `json:"v_red_bc_bm,omitempty"`
	PRedBCBM string `json:"p_red_bc_bm,omitempty"`
}

type TribFederal struct {
	CST            string `json:"cst,omitempty"`
	VBCPisCofins   string `json:"v_bc_pis_cofins,omitempty"`
	PAliqPis       string `json:"p_aliq_pis,omitempty"`
	PAliqCofins    string `json:"p_aliq_cofins,omitempty"`
	VPis           string `json:"v_pis,omitempty"`
	VCofins        string `json:"v_cofins,omitempty"`
	TpRetPisCofins int    `json:"tp_ret_pis_cofins,omitempty"`
	VRetCP         string `json:"v_ret_cp,omitempty"`
	VRetIRRF       string `json:"v_ret_irrf,omitempty"`
	VRetCSLL       string `json:"v_ret_csll,omitempty"`
}

type TotTrib struct {
	IndTotTrib  int    `json:"ind_tot_trib"`
	PTotTribSN  string `json:"p_tot_trib_sn,omitempty"`
	VTotTribFed string `json:"v_tot_trib_fed,omitempty"`
	VTotTribEst string `json:"v_tot_trib_est,omitempty"`
	VTotTribMun string `json:"v_tot_trib_mun,omitempty"`
	PTotTribFed string `json:"p_tot_trib_fed,omitempty"`
	PTotTribEst string `json:"p_tot_trib_est,omitempty"`
	PTotTribMun string `json:"p_tot_trib_mun,omitempty"`
}

// IBSCBS espelha TCRTCInfoIBSCBS (reforma tributária).
type IBSCBS struct {
	FinNFSe   int           `json:"fin_nfse"`
	IndFinal  int           `json:"ind_final,omitempty"`
	CIndOp    string        `json:"c_ind_op"` // Anexo C, 6 dígitos
	TpOper    int           `json:"tp_oper,omitempty"`
	GRefNFSe  *RefNFSe      `json:"g_ref_nfse,omitempty"`
	TpEnteGov int           `json:"tp_ente_gov,omitempty"`
	IndDest   int           `json:"ind_dest"`
	Dest      *Pessoa       `json:"dest,omitempty"`
	Imovel    *Imovel       `json:"imovel,omitempty"`
	Valores   IBSCBSValores `json:"valores"`
}

// RefNFSe espelha TCInfoRefNFSe: refNFSe é repetível (até 99).
type RefNFSe struct {
	Chaves []string `json:"chaves"`
}

// Imovel espelha TCRTCInfoImovel: inscImobFisc? seguido da escolha
// obrigatória cCIB|end (TCEnderObraEvento).
type Imovel struct {
	InscImobFisc string           `json:"insc_imob_fisc,omitempty"`
	CIB          string           `json:"cib,omitempty"`
	End          *EnderecoSimples `json:"endereco,omitempty"`
}

// IBSCBSValores espelha TCRTCInfoValoresIBSCBS. gReeRepRes (reembolso/
// repasse/ressarcimento de terceiros) não é suportado nesta fase — é um
// grupo opcional e nenhum campo do modelo neutro o alimenta ainda.
type IBSCBSValores struct {
	Trib TribIBSCBS `json:"trib"`
}

// TribIBSCBS espelha TCRTCInfoTributosSitClas (dentro de trib>gIBSCBS).
type TribIBSCBS struct {
	CST         string       `json:"cst"`
	CClassTrib  string       `json:"c_class_trib"`
	CCredPres   string       `json:"c_cred_pres,omitempty"`
	TribRegular *TribRegular `json:"trib_regular,omitempty"`
	Dif         *DifIBSCBS   `json:"dif,omitempty"`
}

type TribRegular struct {
	CSTReg        string `json:"cst_reg"`
	CClassTribReg string `json:"c_class_trib_reg"`
}

// DifIBSCBS espelha TCRTCInfoTributosDif — três percentuais, todos obrigatórios.
type DifIBSCBS struct {
	PDifUF  string `json:"p_dif_uf"`
	PDifMun string `json:"p_dif_mun"`
	PDifCBS string `json:"p_dif_cbs"`
}

// DecodeDocument converte o Body["document"] recebido em dfe.Request para o
// modelo neutro. Campo desconhecido é erro: um typo na api tem que estourar
// aqui e não virar uma DPS silenciosamente incompleta.
func DecodeDocument(body map[string]any) (Document, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return Document{}, fmt.Errorf("nfse: encode document body: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("nfse: decode document: %w", err)
	}
	return doc, nil
}
