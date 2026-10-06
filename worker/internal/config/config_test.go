package config

import "testing"

func TestLoadRequiresEgressRegion(t *testing.T) {
	t.Setenv("DOCUMENTS_BUCKET", "docs")
	t.Setenv("CERTIFICATES_BUCKET", "certs")
	t.Setenv("DFE_LAMBDA_NAME", "dev-go-dfe-egress")
	t.Setenv("DFE_EGRESS_REGION", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DFE_EGRESS_REGION is empty")
	}
	t.Setenv("DFE_EGRESS_REGION", "sa-east-1")
	cfg, err := Load()
	if err != nil || cfg.DfeEgressRegion != "sa-east-1" {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}
