package documents

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

// multiItemNFeXML clones the sample NF-e up to n items so pagination is exercised.
func multiItemNFeXML(n int) string {
	base := sampleNFeXML()
	one := base[strings.Index(base, `<det nItem="1">`) : strings.Index(base, "</det>")+len("</det>")]
	var b strings.Builder
	for i := 1; i <= n; i++ {
		item := strings.Replace(one, `nItem="1"`, fmt.Sprintf("nItem=%q", fmt.Sprint(i)), 1)
		item = strings.Replace(item, "<cProd>1</cProd>", fmt.Sprintf("<cProd>%d</cProd>", i), 1)
		item = strings.Replace(item, "PRODUTO TESTE", fmt.Sprintf("PRODUTO TESTE %d", i), 1)
		b.WriteString(item)
	}
	return strings.Replace(base, one, b.String(), 1)
}

// Local visual loop: DUMP_DIR=/path go test -run TestDumpLocal ./...
func TestDumpLocal(t *testing.T) {
	dir := os.Getenv("DUMP_DIR")
	if dir == "" {
		t.Skip("DUMP_DIR unset")
	}
	renderer, err := newFolioRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for name, xml := range map[string]string{
		"nfe": sampleNFeXML(), "nfce": sampleNFCeXML(), "mdfe": sampleMDFeXML(),
	} {
		pdf, err := renderer.Render(context.Background(), name, []byte(xml), false)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := os.WriteFile(dir+"/dump_"+name+".pdf", pdf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Also dump the rendered HTML for bisecting layout problems.
	root, err := parseXML([]byte(sampleNFeXML()))
	if err != nil {
		t.Fatal(err)
	}
	nfeCtx, err := buildNFeContext(root, false)
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := exec.NewTemplate("/templates/"+templateDANFe, gonja.DefaultConfig, renderer.loader, gonja.DefaultEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	html, err := tmpl.ExecuteToString(exec.NewContext(map[string]any{"ctx": nfeCtx}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/dump_nfe.html", []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	bare := strings.Replace(sampleNFeXML(), "<infAdic><infCpl>Documento sintético</infCpl></infAdic>", "", 1)
	empty, err := renderer.Render(context.Background(), DocTypeNFe, []byte(bare), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/dump_nfe_empty.pdf", empty, 0o644); err != nil {
		t.Fatal(err)
	}
	big, err := renderer.Render(context.Background(), DocTypeNFe, []byte(multiItemNFeXML(60)), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/dump_nfe_multi.pdf", big, 0o644); err != nil {
		t.Fatal(err)
	}
}
