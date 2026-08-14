package services

import (
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
