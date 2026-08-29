package documents

import "time"

const (
	DocTypeNFe  = "nfe"
	DocTypeNFCe = "nfce"
	DocTypeMDFe = "mdfe"

	modelNFe  = "55"
	modelNFCe = "65"
	modelMDFe = "58"

	templateDANFe  = "danfe_retrato.html"
	templateDANFCe = "danfce.html"
	templateDAMDFE = "damdfe_retrato.html"

	// mdfeConsultaURL is the national MDF-e lookup page the DAMDFE must print.
	mdfeConsultaURL = "https://dfe-portal.svrs.rs.gov.br/MDFE/Consulta"
	// defaultBoxBackground tints the DAMDFE field boxes until an emitente
	// configures its own colour.
	defaultBoxBackground = "#dbeaf5"

	cacheSchemaVersion = "v1"
	cacheStateActive   = "active"
	cacheStateCanceled = "canceled"
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
}
