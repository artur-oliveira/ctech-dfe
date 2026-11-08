package documents

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
)

// Printed formats selectable with the `format` request parameter. The empty format means "the format the XML
// asks for" (NF-e: tpImp) and keeps the pre-F8 cache key.
const (
	FormatRetrato      = "retrato"
	FormatPaisagem     = "paisagem"
	FormatSimplificado = "simplificado"
	FormatEtiqueta     = "etiqueta"
	// NFC-e (Manual DANFE NFC-e 2.1 and 4.1): completo, resumido (no items division) and mensagem eletrônica.
	FormatCompleto = "completo"
	FormatResumido = "resumido"
	FormatMensagem = "mensagem"

	templateDANFePaisagem     = "danfe_paisagem.html"
	templateDANFeSimplificado = "danfe_simplificado.html"
	templateDANFCeMensagem    = "danfce_mensagem.html"
	templateDAMDFEPaisagem    = "damdfe_paisagem.html"

	tpImpPaisagem     = "2"
	tpImpSimplificado = "3"
	tpImpNFCeMensagem = "5"
	tpEmisEPEC        = "4"

	// ProblemCodeFormatNotAllowed is the RFC 7807 code when the format cannot be printed for this document.
	ProblemCodeFormatNotAllowed = "document-format-not-allowed"
	// ProblemCodeInvalidFormat is the RFC 7807 code for a format value the document type does not list.
	ProblemCodeInvalidFormat = "invalid-document-format"
)

// templateByFormat lists, per document type, the selectable formats. A document type absent here has a single
// format and rejects the parameter.
var templateByFormat = map[string]map[string]string{
	DocTypeNFe: {
		FormatRetrato: templateDANFe, FormatPaisagem: templateDANFePaisagem,
		FormatSimplificado: templateDANFeSimplificado, FormatEtiqueta: templateDANFeSimplificado,
	},
	DocTypeMDFe: {FormatRetrato: templateDAMDFE, FormatPaisagem: templateDAMDFEPaisagem},
	DocTypeNFCe: {FormatCompleto: templateDANFCe, FormatResumido: templateDANFCe, FormatMensagem: templateDANFCeMensagem},
}

// formatFromTpImp maps the XML tpImp to the default format of each document type.
var formatFromTpImp = map[string]func(tpImp string) string{
	DocTypeNFe: func(tpImp string) string {
		switch tpImp {
		case tpImpPaisagem:
			return FormatPaisagem
		case tpImpSimplificado:
			return FormatSimplificado
		}
		return FormatRetrato
	},
	DocTypeMDFe: func(string) string { return FormatRetrato },
	DocTypeNFCe: func(tpImp string) string {
		if tpImp == tpImpNFCeMensagem {
			return FormatMensagem
		}
		return FormatCompleto
	},
}

// FormatNotAllowedError: the requested format is valid but forbidden for this particular document.
type FormatNotAllowedError struct{ Format, Reason string }

func (e *FormatNotAllowedError) Error() string {
	return fmt.Sprintf("format %q not allowed: %s", e.Format, e.Reason)
}

// AllowedFormats returns the sorted formats of a document type.
func AllowedFormats(docType string) []string {
	out := make([]string, 0, len(templateByFormat[docType]))
	for f := range templateByFormat[docType] {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// ValidateFormat rejects, before any S3 or render work, a format the document type does not list.
func ValidateFormat(docType, format string) *problem.Problem {
	if format == "" {
		return nil
	}
	if _, ok := templateByFormat[docType][format]; ok {
		return nil
	}
	allowed := "nenhum (formato único)"
	if list := AllowedFormats(docType); len(list) > 0 {
		allowed = strings.Join(list, ", ")
	}
	return problem.BadRequest("formato do documento inválido: " + format + "; permitidos: " + allowed).WithCode(ProblemCodeInvalidFormat)
}

// resolveFormat returns the effective format and its template. format "" derives the NF-e format from tpImp.
func resolveFormat(docType, format string, root *xmlNode) (effective, template string, err error) {
	formats, ok := templateByFormat[docType]
	if !ok {
		if format != "" {
			return "", "", fmt.Errorf("document type %q has a single format", docType)
		}
		return "", templateByDocType[docType], nil
	}
	ide := root.firstDeep("infNFe").child("ide") // NF-e and NFC-e; absent (nil-safe) for MDF-e
	if format == "" {
		format = formatFromTpImp[docType](ide.value("tpImp"))
	}
	tpl, ok := formats[format]
	if !ok {
		return "", "", fmt.Errorf("unknown format %q for %q", format, docType)
	}
	// MOC DANFE 2.11: the simplified DANFE (and its etiqueta) cannot be issued with EPEC.
	if (format == FormatSimplificado || format == FormatEtiqueta) && ide.value("tpEmis") == tpEmisEPEC {
		return "", "", &FormatNotAllowedError{Format: format, Reason: "DANFE simplificado não é admitido em contingência EPEC"}
	}
	return format, tpl, nil
}
