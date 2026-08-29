package documents

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
)

// PROBE_HTML=/path/to.html DUMP_DIR=/path go test -run TestProbeLocal
func TestProbeLocal(t *testing.T) {
	src := os.Getenv("PROBE_HTML")
	dir := os.Getenv("DUMP_DIR")
	if src == "" || dir == "" {
		t.Skip("PROBE_HTML/DUMP_DIR unset")
	}
	html, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	doc := document.NewDocument(document.PageSizeA4)
	if err := doc.AddHTMLWithContext(context.Background(), string(html), &foliohtml.Options{MaxElements: maxHTMLElements, MaxDepth: maxHTMLDepth, MaxTotalAssetBytes: maxAssetBytes, StrictAssets: true}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, err := doc.WriteToWithContext(context.Background(), &out, document.WriteOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/probe.pdf", out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
