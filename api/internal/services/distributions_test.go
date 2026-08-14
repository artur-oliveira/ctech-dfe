package services

import (
	"context"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestFiscalCfg_NFCe_ResolvesNonNil(t *testing.T) {
	nfceRepo := &repositories.NfceConfigRepository{}
	svc := &DistributionService{NfceConfig: nfceRepo}
	if got := svc.fiscalCfg(DocTypeNFCe); got == nil {
		t.Fatal("expected fiscalCfg(nfce) to resolve to a non-nil repository")
	}
}

func TestImportXML_InvalidDocType_Rejected(t *testing.T) {
	svc := &DistributionService{}
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "cte", []byte(`<nfeProc/>`))
	if err == nil {
		t.Fatal("expected error for unsupported doc_type")
	}
}

func TestImportXML_FileTooLarge_Rejected(t *testing.T) {
	svc := &DistributionService{}
	big := make([]byte, maxImportXMLSize+1)
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "nfe", big)
	if err == nil {
		t.Fatal("expected error for oversized upload")
	}
}

func TestImportXML_InvalidRoot_Rejected(t *testing.T) {
	svc := &DistributionService{}
	_, err := svc.ImportXML(context.Background(), "CNPJ_11647612000197", "nfe", []byte(`<resNFe xmlns="x"/>`))
	if err == nil {
		t.Fatal("expected error for unsupported root element")
	}
}

func TestValidateDistDocTypeAllowsNfseHistory(t *testing.T) {
	t.Parallel()

	if err := validateDistDocType(DocTypeNfse); err != nil {
		t.Fatalf("validateDistDocType(%q) returned %v", DocTypeNfse, err)
	}
}

func TestValidateSefazDistDocTypeRejectsNfseLookup(t *testing.T) {
	t.Parallel()

	if err := validateSefazDistDocType(DocTypeNfse); err == nil {
		t.Fatalf("validateSefazDistDocType(%q) returned nil", DocTypeNfse)
	}
}
