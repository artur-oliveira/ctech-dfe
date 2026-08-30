package documents

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	"github.com/nikolalohinski/gonja/v2/loaders"
)

//go:embed templates/*.html
var templateFS embed.FS

var (
	fieldMacroCall = regexp.MustCompile(`\{\{\s*fld\((.+)\)\s*}}`)
	titleMacroCall = regexp.MustCompile(`\{\{\s*title\((.+)\)\s*}}`)
)

type pdfRenderer interface {
	Render(context.Context, string, []byte, DocumentState) ([]byte, error)
}

type folioRenderer struct {
	loader loaders.Loader
}

func newFolioRenderer() (*folioRenderer, error) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("list auxiliary document templates: %w", err)
	}
	templates := make(map[string]string, len(entries))
	for _, entry := range entries {
		contents, err := templateFS.ReadFile("templates/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read auxiliary document template %s: %w", entry.Name(), err)
		}
		templates["/templates/"+entry.Name()] = expandNestedMacros(string(contents))
	}
	loader, err := loaders.NewMemoryLoader(templates)
	if err != nil {
		return nil, fmt.Errorf("initialize auxiliary document templates: %w", err)
	}
	gonja.SetLoggerOutput(io.Discard)
	return &folioRenderer{loader: loader}, nil
}

// Gonja intentionally does not expose one imported macro to another macro in
// the same file. The fiscal templates use two tiny presentation helpers that
// way, so expand only those calls before parsing and preserve the larger Jinja
// macros unchanged.
func expandNestedMacros(source string) string {
	source = fieldMacroCall.ReplaceAllStringFunc(source, func(call string) string {
		match := fieldMacroCall.FindStringSubmatch(call)
		args := splitTemplateArguments(match[1])
		if len(args) < 2 || len(args) > 3 {
			return call
		}
		class := `""`
		if len(args) == 3 {
			class = args[2]
		}
		return `<td class="fld {{ ` + class + ` }}"><span class="lbl">{{ ` + args[0] + ` }}</span><span class="val">{{ ` + args[1] + ` }}</span></td>`
	})
	return titleMacroCall.ReplaceAllString(source, `<div class="quadro-title">{{ $1 }}</div>`)
}

func splitTemplateArguments(value string) []string {
	var args []string
	start, depth := 0, 0
	var quote rune
	for index, char := range value {
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		case char == '(':
			depth++
		case char == ')':
			depth--
		case char == ',' && depth == 0:
			args = append(args, strings.TrimSpace(value[start:index]))
			start = index + 1
		}
	}
	return append(args, strings.TrimSpace(value[start:]))
}

func (r *folioRenderer) Render(ctx context.Context, docType string, xmlBytes []byte, state DocumentState) ([]byte, error) {
	templateName, ok := templateByDocType[docType]
	if !ok {
		return nil, fmt.Errorf("unsupported auxiliary document type %q", docType)
	}
	root, err := parseXML(xmlBytes)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	switch docType {
	case DocTypeNFe:
		data, err = buildNFeContext(root, state == StateCancelled)
	case DocTypeNFCe:
		data, err = buildNFCeContext(root, state == StateCancelled)
	case DocTypeMDFe:
		data, err = buildMDFeContext(root, state == StateCancelled)
	case DocTypeNFSe:
		data, err = buildNFSeContext(root, state)
	}
	if err != nil {
		return nil, err
	}
	if docType == DocTypeNFCe {
		vias, _ := data["vias"].([]map[string]any)
		parts := make([][]byte, 0, len(vias))
		for _, via := range vias {
			data["via"] = via
			part, err := r.renderOne(ctx, templateName, data)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		}
		if len(parts) > 1 {
			return mergePDFs(parts)
		}
		if len(parts) == 1 {
			return parts[0], nil
		}
	}
	return r.renderOne(ctx, templateName, map[string]any{"ctx": data})
}

func (r *folioRenderer) renderOne(ctx context.Context, templateName string, data map[string]any) ([]byte, error) {
	tmpl, err := exec.NewTemplate("/templates/"+templateName, gonja.DefaultConfig, r.loader, gonja.DefaultEnvironment)
	if err != nil {
		return nil, fmt.Errorf("parse auxiliary document template %s: %w", templateName, err)
	}
	html, err := tmpl.ExecuteToString(exec.NewContext(data))
	if err != nil {
		return nil, fmt.Errorf("execute auxiliary document template %s: %w", templateName, err)
	}
	if len(html) > maxRenderedHTMLBytes {
		return nil, fmt.Errorf("rendered auxiliary document HTML exceeds %d bytes", maxRenderedHTMLBytes)
	}
	doc := document.NewDocument(document.PageSizeA4)
	if err := doc.AddHTMLWithContext(ctx, html, &foliohtml.Options{
		MaxElements:        maxHTMLElements,
		MaxDepth:           maxHTMLDepth,
		MaxTotalAssetBytes: maxAssetBytes,
		StrictAssets:       true,
	}); err != nil {
		return nil, fmt.Errorf("convert auxiliary document HTML: %w", err)
	}
	var out bytes.Buffer
	if _, err := doc.WriteToWithContext(ctx, &out, document.WriteOptions{
		UseXRefStream:       true,
		UseObjectStreams:    true,
		OrphanSweep:         true,
		CleanContentStreams: true,
		DeduplicateObjects:  true,
		RecompressStreams:   true,
	}); err != nil {
		return nil, fmt.Errorf("serialize auxiliary document PDF: %w", err)
	}
	return out.Bytes(), nil
}

func mergePDFs(parts [][]byte) ([]byte, error) {
	readers := make([]*reader.PdfReader, 0, len(parts))
	for _, part := range parts {
		parsed, err := reader.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("parse generated PDF for merge: %w", err)
		}
		readers = append(readers, parsed)
	}
	merged, err := reader.Merge(readers...)
	if err != nil {
		return nil, fmt.Errorf("merge generated PDFs: %w", err)
	}
	var out bytes.Buffer
	if _, err := merged.WriteTo(&out); err != nil {
		return nil, fmt.Errorf("serialize merged PDF: %w", err)
	}
	return out.Bytes(), nil
}
