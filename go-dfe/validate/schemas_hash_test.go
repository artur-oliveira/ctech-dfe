package validate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Os XSDs vendorizados em go-dfe/schemas são a fonte do gerador de specs. Este teste falha se um arquivo
// sumir, aparecer sem registro ou mudar de conteúdo. Para atualizar um pacote:
// (cd go-dfe/schemas && find . -name '*.xsd' | sort | xargs sha256sum > SHA256SUMS), regenere os specs
// (go run gen/main.go) e registre pacote, versão e data no README de go-dfe/schemas.
func TestVendoredSchemasHash(t *testing.T) {
	const root = "../schemas"
	f, err := os.Open(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		sum, path, ok := strings.Cut(sc.Text(), "  ")
		if !ok {
			t.Fatalf("linha inválida em SHA256SUMS: %q", sc.Text())
		}
		want[filepath.ToSlash(strings.TrimPrefix(path, "./"))] = sum
	}
	got := map[string]bool{}
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".xsd" {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		got[rel] = true
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		switch w, ok := want[rel]; {
		case !ok:
			t.Errorf("%s: XSD sem registro em SHA256SUMS", rel)
		case w != hex.EncodeToString(h[:]):
			t.Errorf("%s: conteúdo mudou (hash diferente de SHA256SUMS)", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for rel := range want {
		if !got[rel] {
			t.Errorf("%s: XSD registrado em SHA256SUMS sumiu", rel)
		}
	}
}
