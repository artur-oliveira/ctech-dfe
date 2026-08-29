//go:build integration

package documents

import (
	"bytes"
	"context"
	"testing"
)

func TestIntegrationXMLTemplatesAndFolio(t *testing.T) {
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for docType, xml := range map[string]string{
		DocTypeNFe: sampleNFeXML(), DocTypeNFCe: sampleNFCeXML(), DocTypeMDFe: sampleMDFeXML(),
	} {
		pdf, err := renderer.Render(context.Background(), docType, []byte(xml), true)
		if err != nil {
			t.Fatalf("render %s: %v", docType, err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatalf("render %s did not produce a PDF", docType)
		}
	}
}
