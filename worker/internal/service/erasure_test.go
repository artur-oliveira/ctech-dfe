package service

import (
	"context"
	"errors"
	"testing"
)

func stoppedFor(company string) OrgStopped {
	return func(_ context.Context, c string) (bool, error) { return c == company, nil }
}

func TestCompanyOfDocPK(t *testing.T) {
	if got := companyOfDocPK("prod#0190-company"); got != "0190-company" {
		t.Fatalf("got %q", got)
	}
	if got := companyOfDocPK("no-env"); got != "" {
		t.Fatalf("got %q, want empty for a malformed key", got)
	}
}

func TestProcess_DropsJobOfStoppedCompany(t *testing.T) {
	s3m, dynm := certS3(), &mockDynamo{}
	svc := newSvc(s3m, &mockLambda{payload: invokeResp("100", "Autorizado", "1")}, dynm).
		WithOrgStopped(stoppedFor("CNPJ_12345678000195"))
	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("a dropped job must not fail (no retries, no DLQ): %v", err)
	}
	if dynm.claimCalls != 0 || len(s3m.putCalls) != 0 {
		t.Fatalf("an erased company's job touched DynamoDB (%d) or S3 (%d)", dynm.claimCalls, len(s3m.putCalls))
	}
}

func TestProcess_StoppedCheckFailureRetries(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{}, dynm).
		WithOrgStopped(func(context.Context, string) (bool, error) { return false, errors.New("dynamodb down") })
	if err := svc.Process(context.Background(), baseMsg); err == nil || dynm.claimCalls != 0 {
		t.Fatal("an unknown erasure state must fail closed before the claim")
	}
}

func TestDistribution_DropsJobOfStoppedCompany(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, testCfg).WithOrgStopped(stoppedFor("C1"))
	if err := svc.Process(context.Background(), DistributionMessage{OrgPK: "C1", DocType: "nfe"}); err != nil {
		t.Fatal(err)
	}
	if dynm.getIdx != 0 || dynm.queryIdx != 0 || len(dynm.updateCalls) != 0 || len(dynm.putCalls) != 0 {
		t.Fatal("an erased company's distribution job touched DynamoDB")
	}
}
