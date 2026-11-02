package services

import (
	"errors"

	"context"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
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

type fakeWorkerInvoker struct {
	in     *lambda.InvokeInput
	status int32
	err    error
}

func (f *fakeWorkerInvoker) Invoke(_ context.Context, in *lambda.InvokeInput, _ ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	f.in = in
	return &lambda.InvokeOutput{StatusCode: f.status}, f.err
}

// The hand-off to the worker must be an async (Event) invoke of the distribution
// function: that is what gives it Lambda's retries and the dlq-processor destination.
func TestEnqueue_AsyncInvokeOfDistributionWorker(t *testing.T) {
	inv := &fakeWorkerInvoker{status: 202}
	svc := &DistributionService{invoker: inv, functionName: "dev-distribution-worker"}
	if err := svc.enqueue(context.Background(), []byte(`{"job_type":"dist_nsu"}`)); err != nil {
		t.Fatal(err)
	}
	if *inv.in.FunctionName != "dev-distribution-worker" || inv.in.InvocationType != lambdatypes.InvocationTypeEvent || string(inv.in.Payload) != `{"job_type":"dist_nsu"}` {
		t.Fatalf("unexpected invoke: %+v", inv.in)
	}
}

func TestEnqueue_FailsOnErrorOrNon202(t *testing.T) {
	svc := &DistributionService{invoker: &fakeWorkerInvoker{err: errors.New("boom")}, functionName: "f"}
	if err := svc.enqueue(context.Background(), nil); err == nil {
		t.Fatal("invoke error must surface")
	}
	svc = &DistributionService{invoker: &fakeWorkerInvoker{status: 200}, functionName: "f"}
	if err := svc.enqueue(context.Background(), nil); err == nil {
		t.Fatal("a non-202 async status must be a failed hand-off")
	}
}

func TestEnqueueSync_NoFunctionConfigured_Rejected(t *testing.T) {
	svc := &DistributionService{}
	if _, err := svc.EnqueueSync(context.Background(), "CNPJ_1", "nfe"); err == nil {
		t.Fatal("expected error when the distribution worker is not configured")
	}
}

// KMS-encrypted certificate objects bill a KMS request per read; new writes must stay SSE-S3.
func TestCertObjectSSEIsAES256(t *testing.T) {
	if certObjectSSE != s3types.ServerSideEncryptionAes256 {
		t.Fatalf("certObjectSSE = %q, want AES256", certObjectSSE)
	}
}
