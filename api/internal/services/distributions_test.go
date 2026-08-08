package services

import "testing"

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
