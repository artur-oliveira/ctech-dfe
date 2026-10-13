//go:build integration

package integration_test

import (
	"archive/zip"
	"bytes"
	"context"
	"sort"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/testsupport"
)

func TestXMLExport_ZipsProductionDocumentsAndEvents(t *testing.T) {
	ctx := context.Background()
	objects := testsupport.NewObjectStore()
	org := "export-" + randomCNPJ()
	putItem(t, "nfes", map[string]any{"pk": "prod#" + org, "sk": "K1", "xml_s3_key": "nfe/prod/CNPJ_x/K1.xml"})
	putItem(t, "nfes", map[string]any{"pk": "hom#" + org, "sk": "K2", "xml_s3_key": "nfe/hom/CNPJ_x/K2.xml"})
	putItem(t, "nfe_events", map[string]any{"pk": "K1", "sk": "E1", "xml_s3_key": "nfe/prod/CNPJ_x/K1_110111_001.xml"})
	objects.Put("docs", "nfe/prod/CNPJ_x/K1.xml", "<nfe/>")
	objects.Put("docs", "nfe/prod/CNPJ_x/K1_110111_001.xml", "<evento/>")
	objects.Put("docs", "nfe/hom/CNPJ_x/K2.xml", "<hom/>")

	nfe := repositories.NewNfeRepository(db, cfg)
	svc := services.NewXMLExportService(
		map[string]*repositories.DocumentRepository{"nfe": &nfe.DocumentRepository},
		map[string]*repositories.DocumentEventRepository{"nfe": repositories.NewDocumentEventRepository(db, cfg, "nfe")},
		objects, "docs")

	raw, next, err := svc.Page(ctx, org, "nfe", nil)
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Errorf("one page expected, got a cursor %v", next)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "K1.xml" || names[1] != "K1_110111_001.xml" {
		t.Fatalf("zip entries %v, want the production document and its event only", names)
	}
	if _, _, err := svc.Page(ctx, org, "boleto", nil); err == nil {
		t.Fatal("an unknown document type must be refused")
	}
}
