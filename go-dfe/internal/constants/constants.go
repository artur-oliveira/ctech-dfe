// Package constants is a 1:1 port of py-dfe's constants/enums.py: UF list,
// environment/doc-type enums, WSDL service/operation tables, and the
// per-authorizer overrides SEFAZ requires. Kept as a single file, mirroring
// the source layout, since these are pure lookup tables with no logic.
package constants

import "fmt"

// Environment values, already normalized (py-dfe also accepts
// "producao"/"homologacao" and normalizes on the way in — see
// py-dfe/py_dfe/models/request.py normalize_environment).
const (
	EnvironmentProd = "prod"
	EnvironmentHom  = "hom"
)

// DocType values.
const (
	DocTypeNFE  = "nfe"
	DocTypeNFCE = "nfce"
	DocTypeCTE  = "cte"
	DocTypeMDFE = "mdfe"
)

// Retry defaults (py-dfe/py_dfe/models/request.py: max_retries 0-10, default 3).
const (
	DefaultMaxRetries = 3
	MinMaxRetries     = 0
	MaxMaxRetries     = 10
)

// UFList is every valid UF code, including "EX" (exterior/estrangeiro).
var UFList = []string{
	"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS",
	"MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC",
	"SE", "SP", "TO", "EX",
}

// UFIBGE maps UF -> IBGE code (cUF), used in distDFeInt's cUFAutor field.
var UFIBGE = map[string]int{
	"RO": 11, "AC": 12, "AM": 13, "RR": 14, "PA": 15, "AP": 16, "TO": 17,
	"MA": 21, "PI": 22, "CE": 23, "RN": 24, "PB": 25, "PE": 26, "AL": 27,
	"SE": 28, "BA": 29, "MG": 31, "ES": 32, "RJ": 33, "SP": 35, "PR": 41,
	"SC": 42, "RS": 43, "MS": 50, "MT": 51, "GO": 52, "DF": 53, "EX": 91,
}

// GetUFIBGE returns the IBGE code for uf, or an error if uf is unknown.
func GetUFIBGE(uf string) (int, error) {
	if code, ok := UFIBGE[uf]; ok {
		return code, nil
	}
	return 0, fmt.Errorf("no IBGE code for UF %q", uf)
}

// DocTypeCode maps doc_type -> SEFAZ model code (mod).
var DocTypeCode = map[string]string{
	DocTypeNFE:  "55",
	DocTypeNFCE: "65",
	DocTypeCTE:  "57",
	DocTypeMDFE: "58",
}

// GetDocTypeCode returns the SEFAZ model code for docType, or an error if unknown.
func GetDocTypeCode(docType string) (string, error) {
	if code, ok := DocTypeCode[docType]; ok {
		return code, nil
	}
	return "", fmt.Errorf("no doc type code for %q", docType)
}

// SchemaVersion maps doc_type -> XSD schema version.
var SchemaVersion = map[string]string{
	DocTypeNFE:  "4.00",
	DocTypeNFCE: "4.00",
	DocTypeCTE:  "4.00",
	DocTypeMDFE: "3.00",
}

// SOAPHeaderVersionOverride: some services use a payload schema version
// different from the doc-type version — MT (and potentially other strict
// UFs) validate versaoDados against the actual payload schema.
var SOAPHeaderVersionOverride = map[string]string{
	"NfeConsultaCadastro": "2.00",
}

// DocNamespace maps doc_type -> XML namespace.
var DocNamespace = map[string]string{
	DocTypeNFE:  "http://www.portalfiscal.inf.br/nfe",
	DocTypeNFCE: "http://www.portalfiscal.inf.br/nfe",
	DocTypeCTE:  "http://www.portalfiscal.inf.br/cte",
	DocTypeMDFE: "http://www.portalfiscal.inf.br/mdfe",
}

// SOAPElementNames names the SOAP header/body/result element per doc_type.
type SOAPElementNames struct {
	Header string
	Body   string
	Result string
}

var SOAPElements = map[string]SOAPElementNames{
	DocTypeNFE:  {Header: "nfeCabecMsg", Body: "nfeDadosMsg", Result: "nfeResultMsg"},
	DocTypeNFCE: {Header: "nfeCabecMsg", Body: "nfeDadosMsg", Result: "nfeResultMsg"},
	DocTypeCTE:  {Header: "cteCabecMsg", Body: "cteDadosMsg", Result: "cteResultMsg"},
	DocTypeMDFE: {Header: "mdfeCabecMsg", Body: "mdfeDadosMsg", Result: "mdfeResultMsg"},
}

// NFeWSDLService covers both nfe and nfce doc types (nfce reuses NFe's WSDL).
var NFeWSDLService = map[string]string{
	"NFeAutorizacao":       "NFeAutorizacao4",
	"NFeRetAutorizacao":    "NFeRetAutorizacao4",
	"NfeInutilizacao":      "NFeInutilizacao4",
	"NfeConsultaProtocolo": "NFeConsultaProtocolo4",
	"NfeStatusServico":     "NFeStatusServico4",
	"NfeConsultaCadastro":  "CadConsultaCadastro4",
	"RecepcaoEvento":       "NFeRecepcaoEvento4",
	"NFeDistribuicaoDFe":   "NFeDistribuicaoDFe",
}

var CTeWSDLService = map[string]string{
	"CTeRecepcaoSinc":    "CTeRecepcaoSincV4",
	"CTeRecepcaoOS":      "CTeRecepcaoOSV4",
	"CTeRecepcaoGTVe":    "CTeRecepcaoGTVeV4",
	"CTeRecepcaoSimp":    "CTeRecepcaoSimpV4",
	"CTeConsulta":        "CTeConsultaV4",
	"CTeStatusServico":   "CTeStatusServicoV4",
	"CTeRecepcaoEvento":  "CTeRecepcaoEventoV4",
	"CTeDistribuicaoDFe": "CTeDistribuicaoDFe",
}

var MDFeWSDLService = map[string]string{
	"MDFeRecepcaoSinc":    "MDFeRecepcaoSinc",
	"MDFeConsulta":        "MDFeConsulta",
	"MDFeStatusServico":   "MDFeStatusServico",
	"MDFeConsNaoEnc":      "MDFeConsNaoEnc",
	"MDFeDistribuicaoDFe": "MDFeDistribuicaoDFe",
	"MDFeRecepcaoEvento":  "MDFeRecepcaoEvento",
}

// WSDLServiceByDocType resolves the WSDL service name given doc_type+service.
var WSDLServiceByDocType = map[string]map[string]string{
	DocTypeNFE:  NFeWSDLService,
	DocTypeNFCE: NFeWSDLService,
	DocTypeCTE:  CTeWSDLService,
	DocTypeMDFE: MDFeWSDLService,
}

var NFeWSDLOperation = map[string]string{
	"NFeAutorizacao":       "nfeAutorizacaoLote",
	"NFeRetAutorizacao":    "nfeRetAutorizacaoLote",
	"NfeInutilizacao":      "nfeInutilizacaoNF",
	"NfeConsultaProtocolo": "nfeConsultaNF",
	"NfeStatusServico":     "nfeStatusServicoNF",
	"NfeConsultaCadastro":  "consultaCadastro",
	"RecepcaoEvento":       "nfeRecepcaoEvento",
	"NFeDistribuicaoDFe":   "nfeDistDFeInteresse",
}

var CTeWSDLOperation = map[string]string{
	"CTeStatusServico":   "cteStatusServicoCT",
	"CTeRecepcaoSinc":    "cteRecepcao",
	"CTeRecepcaoOS":      "cteRecepcaoOS",
	"CTeRecepcaoGTVe":    "cteRecepcaoGTVe",
	"CTeRecepcaoSimp":    "cteRecepcaoSimp",
	"CTeConsulta":        "cteConsultaCT",
	"CTeRecepcaoEvento":  "cteRecepcaoEvento",
	"CTeDistribuicaoDFe": "cteDistDFeInteresse",
}

var MDFeWSDLOperation = map[string]string{
	"MDFeStatusServico":   "mdfeStatusServicoMDF",
	"MDFeRecepcaoSinc":    "mdfeRecepcao",
	"MDFeConsulta":        "mdfeConsultaMDF",
	"MDFeConsNaoEnc":      "mdfeConsNaoEnc",
	"MDFeRecepcaoEvento":  "mdfeRecepcaoEvento",
	"MDFeDistribuicaoDFe": "mdfeDistDFeInteresse",
}

// WSDLOperationByDocType resolves the SOAP operation element name given doc_type+service.
var WSDLOperationByDocType = map[string]map[string]string{
	DocTypeNFE:  NFeWSDLOperation,
	DocTypeNFCE: NFeWSDLOperation,
	DocTypeCTE:  CTeWSDLOperation,
	DocTypeMDFE: MDFeWSDLOperation,
}

// SOAPWrappedBodyServices are services that always require a wrapped SOAP
// body (an operation element wrapping the body element).
var SOAPWrappedBodyServices = map[string]bool{
	"NFeDistribuicaoDFe": true,
	"CTeDistribuicaoDFe": true,
}

// wrappedBodyOverrideKey builds the lookup key for SOAPWrappedBodyOverrides.
func wrappedBodyOverrideKey(uf, service string) string { return uf + "|" + service }

// SOAPWrappedBodyOverrides: per-(uf, service) overrides for wrapped SOAP
// body. MT's CadConsultaCadastro4 requires
// <consultaCadastro><nfeDadosMsg>...</nfeDadosMsg></consultaCadastro>.
var SOAPWrappedBodyOverrides = map[string]bool{
	wrappedBodyOverrideKey("MT", "NfeConsultaCadastro"): true,
}

// IsSOAPWrappedBodyOverride reports whether (uf, service) requires a wrapped body.
func IsSOAPWrappedBodyOverride(uf, service string) bool {
	return SOAPWrappedBodyOverrides[wrappedBodyOverrideKey(uf, service)]
}

// Códigos de tpEvento (tabela SEFAZ).
const (
	TpEventoCancelamento            = "110111"
	TpEventoCienciaOperacao         = "210210"
	TpEventoConfirmacaoOperacao     = "210200"
	TpEventoDesconhecimentoOperacao = "210220"
	TpEventoNaoRealizacao           = "210240"
)

// wsdlOperationOverrideKey builds the lookup key for WSDLOperationOverrides.
func wsdlOperationOverrideKey(uf, docType, service string) string {
	return uf + "|" + docType + "|" + service
}

// WSDLOperationOverrides: per-(uf, doc_type, service) overrides for the SOAP
// operation element name, where a UF's authorizer diverges from the
// national default.
var wsdlOperationOverrides = map[string]string{
	wsdlOperationOverrideKey("BA", "nfe", "RecepcaoEvento"):      "nfeRecepcaoEventoNF",
	wsdlOperationOverrideKey("PR", "nfe", "RecepcaoEvento"):      "nfeRecepcaoEventoNF",
	wsdlOperationOverrideKey("AN", "nfe", "RecepcaoEvento"):      "nfeRecepcaoEventoNF",
	wsdlOperationOverrideKey("PR", "nfce", "RecepcaoEvento"):     "nfeRecepcaoEventoNF",
	wsdlOperationOverrideKey("SP", "nfce", "RecepcaoEvento"):     "nfeRecepcaoEventoNF",
	wsdlOperationOverrideKey("AM", "nfe", "NfeConsultaCadastro"): "consultaCadastro4",
	wsdlOperationOverrideKey("PE", "nfe", "NFeRetAutorizacao"):   "NFeRetAutorizacaoLote",
	wsdlOperationOverrideKey("PR", "nfe", "NFeRetAutorizacao"):   "NFeRetAutorizacaoLote",
	wsdlOperationOverrideKey("PR", "nfce", "NFeRetAutorizacao"):  "NFeRetAutorizacaoLote",
}

// WSDLOperation resolves the SOAP operation element name for (uf, docType,
// service), applying the per-UF override if one exists, else the national
// default from WSDLOperationByDocType.
func WSDLOperation(uf, docType, service string) (string, error) {
	if op, ok := wsdlOperationOverrides[wsdlOperationOverrideKey(uf, docType, service)]; ok {
		return op, nil
	}
	ops, ok := WSDLOperationByDocType[docType]
	if !ok {
		return "", fmt.Errorf("no WSDL operation table for doc_type %q", docType)
	}
	op, ok := ops[service]
	if !ok {
		return "", fmt.Errorf("no WSDL operation for doc_type %q service %q", docType, service)
	}
	return op, nil
}

// Error codes, mirroring py-dfe/py_dfe/exceptions.py.
const (
	ErrCodeCertificate      = "certificate error"
	ErrCodeEndpointNotFound = "endpoint not found"
	ErrCodeSOAPRequest      = "soap request error"
	ErrCodeXMLBuild         = "xml build error"
	ErrCodeXMLSign          = "xml sign error"
	ErrCodeXMLValidation    = "xml validation error"
	ErrCodeInvalidResponse  = "invalid sefaz response error"
	ErrCodeRetryExhausted   = "retry exhausted error"
	ErrCodeValidation       = "validation error"
	ErrCodeUnexpected       = "unexpected error"
	ErrCodeCertRequired     = "certificate required"
)
