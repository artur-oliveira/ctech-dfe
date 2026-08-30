package documents

import "time"

const (
	DocTypeNFe  = "nfe"
	DocTypeNFCe = "nfce"
	DocTypeMDFe = "mdfe"
	DocTypeNFSe = "nfse"

	modelNFe  = "55"
	modelNFCe = "65"
	modelMDFe = "58"

	templateDANFe  = "danfe_retrato.html"
	templateDANFCe = "danfce.html"
	templateDAMDFE = "damdfe_retrato.html"
	templateDANFSe = "danfse_v2.html"

	// nfseConsultaURL is the national NFS-e lookup page the DANFSe QR code and
	// footer must print (NT 008 v1.02); the access key is appended verbatim.
	nfseConsultaURL = "https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave="
	// mdfeConsultaURL is the national MDF-e lookup page the DAMDFE must print.
	mdfeConsultaURL = "https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta"
	// defaultBoxBackground tints the DAMDFE field boxes until an emitente
	// configures its own colour.
	defaultBoxBackground = "#dbeaf5"

	cacheSchemaVersion = "v1"
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
	DocTypeNFe:  templateDANFe,
	DocTypeNFCe: templateDANFCe,
	DocTypeMDFe: templateDAMDFE,
	DocTypeNFSe: templateDANFSe,
}

// accessKeyLengthByDocType is the digit count of each document's access key.
// NFS-e nacional uses 50 digits; the SEFAZ document types use 44. Validating
// with a single hard-coded 44 would silently reject every DANFSe request.
var accessKeyLengthByDocType = map[string]int{
	DocTypeNFe:  44,
	DocTypeNFCe: 44,
	DocTypeMDFe: 44,
	DocTypeNFSe: 50,
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
	StateActive: true, StateCancelled: true, StateSubstituted: true,
}
