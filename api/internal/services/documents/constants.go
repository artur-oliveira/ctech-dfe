package documents

import "time"

const (
	DocTypeNFe  = "nfe"
	DocTypeNFCe = "nfce"
	DocTypeMDFe = "mdfe"
	DocTypeNFSe = "nfse"
	DocTypeCTe  = "cte"
	// DocTypeNFCom is the NFCom (modelo 62); its auxiliary document is the DANFE-COM.
	DocTypeNFCom = "nfcom"
	// DocTypeBPe is the BP-e (modelo 63, all variants); its auxiliary document is the DABPE.
	DocTypeBPe = "bpe"

	modelNFe   = "55"
	modelNFCe  = "65"
	modelMDFe  = "58"
	modelCTe   = "57"
	modelNFCom = "62"

	templateDANFe    = "danfe_retrato.html"
	templateDANFCe   = "danfce.html"
	templateDAMDFE   = "damdfe_retrato.html"
	templateDANFSe   = "danfse_v2.html"
	templateDACTE    = "dacte_retrato.html"
	templateDANFECOM = "danfecom.html"
	templateDACTEOS  = "dacte_os_retrato.html"
	templateDABPE    = "dabpe.html"

	// nfseConsultaURL is the national NFS-e lookup page the DANFSe QR code and
	// footer must print (NT 008 v1.02); the access key is appended verbatim.
	nfseConsultaURL = "https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="
	// mdfeConsultaURL is the national MDF-e lookup page the DAMDFE must print.
	mdfeConsultaURL = "https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta"
	// nfcomConsultaURL is the national NFCom lookup page the DANFE-COM prints (MOC DANFE-COM 2.1.3).
	nfcomConsultaURL = "https://dfe-portal.svrs.rs.gov.br/NFCOM/Consulta"
	// defaultBoxBackground tints the DAMDFE field boxes until an emitente
	// configures its own colour.
	defaultBoxBackground = "#dbeaf5"

	// v3: DANFE NFC-e layout (date and time of emission, 28 mm QR, additions line, decimals); v2: DACTE modal boxes and A5; bumping regenerates every cached PDF (DANFE, DACTE, DAMDFE, NFC-e, DANFSe).
	cacheSchemaVersion = "v3"
	cacheTagging       = "cache=auxiliary-document"
	contentTypePDF     = "application/pdf"
	putIfAbsent        = "*"
	fileExtensionPDF   = ".pdf"

	presignedURLTTL   = 15 * time.Minute
	generationTimeout = 8 * time.Second

	maxHTMLElements      = 100_000
	maxHTMLDepth         = 128
	maxAssetBytes        = 16 << 20
	maxRenderedHTMLBytes = 32 << 20
	maxSourceXMLBytes    = 20 << 20
)

var templateByDocType = map[string]string{
	DocTypeNFe:   templateDANFe,
	DocTypeNFCe:  templateDANFCe,
	DocTypeMDFe:  templateDAMDFE,
	DocTypeNFSe:  templateDANFSe,
	DocTypeCTe:   templateDACTE,
	DocTypeNFCom: templateDANFECOM,
	DocTypeBPe:   templateDABPE,
}

// accessKeyLengthByDocType is the digit count of each document's access key.
// NFS-e nacional uses 50 digits; the SEFAZ document types use 44. Validating
// with a single hard-coded 44 would silently reject every DANFSe request.
var accessKeyLengthByDocType = map[string]int{
	DocTypeNFe:   44,
	DocTypeNFCe:  44,
	DocTypeMDFe:  44,
	DocTypeNFSe:  50,
	DocTypeCTe:   44,
	DocTypeNFCom: 44,
	DocTypeBPe:   44,
}

// DocumentState is the closed set of fiscal states an auxiliary document can
// be rendered in. It replaces the previous `canceled bool`: NFS-e can also be
// substituted, which prints a different watermark and must not reuse the
// active document's cache entry.
type DocumentState string

const (
	StateActive      DocumentState = "active"
	StateCancelled   DocumentState = "cancelled"
	StateSubstituted DocumentState = "substituted"
	// StateContingencyPending is an offline NFC-e rendered from its stored signed XML, before the
	// authorization: its own cache key so the PDF is regenerated (with the protocol) afterwards.
	StateContingencyPending DocumentState = "contingency_pending"
)

// CancelledWhen mapeia o par ativo/cancelado dos doc types que não têm
// substituição — NF-e, NFC-e e MDF-e nunca chegam a StateSubstituted.
func CancelledWhen(cancelled bool) DocumentState {
	if cancelled {
		return StateCancelled
	}
	return StateActive
}

var documentStates = map[DocumentState]bool{
	StateActive: true, StateCancelled: true, StateSubstituted: true, StateContingencyPending: true,
}
