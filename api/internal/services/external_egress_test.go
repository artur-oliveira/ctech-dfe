package services

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	godfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

type fakeInvoker struct {
	gotName string
	gotBody []byte
	out     *lambda.InvokeOutput
	err     error
}

func (f *fakeInvoker) Invoke(_ context.Context, in *lambda.InvokeInput, _ ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	f.gotName, f.gotBody = aws.ToString(in.FunctionName), in.Payload
	return f.out, f.err
}

func TestCallDfeRoundTrip(t *testing.T) {
	f := &fakeInvoker{out: &lambda.InvokeOutput{Payload: []byte(`{"statusCode":200,"body":"{\"ok\":true}"}`)}}
	resp, err := callDfe(context.Background(), f, "dev-go-dfe-egress", godfe.Request{DocType: "nfse", Service: "NFSeConsulta"})
	if err != nil || resp.StatusCode != 200 || resp.Body != `{"ok":true}` {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if f.gotName != "dev-go-dfe-egress" || !bytes.Contains(f.gotBody, []byte(`"doc_type":"nfse"`)) {
		t.Fatalf("name=%q body=%s", f.gotName, f.gotBody)
	}
}

func TestCallDfeFunctionErrorIsAnError(t *testing.T) {
	fe := "Unhandled"
	f := &fakeInvoker{out: &lambda.InvokeOutput{FunctionError: &fe, Payload: []byte(`{"errorMessage":"x"}`)}}
	if _, err := callDfe(context.Background(), f, "fn", godfe.Request{}); err == nil {
		t.Fatal("FunctionError must be an error, never an empty success")
	}
}

func TestInvokeSefazLambdaUsesInvoker(t *testing.T) {
	f := &fakeInvoker{out: &lambda.InvokeOutput{Payload: []byte(`{"statusCode":200,"body":"{\"cStat\":\"111\"}"}`)}}
	got, err := invokeSefazLambda(context.Background(), f, "dev-go-dfe-egress", map[string]any{"doc_type": "nfe"})
	if err != nil || got["cStat"] != "111" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

// An egress crash/timeout is an infrastructure fault (5xx), not a user error:
// it must not be reported as the 400 a SEFAZ rejection gets.
func TestInvokeSefazLambdaFunctionErrorIsInternalServerError(t *testing.T) {
	fe := "Unhandled"
	f := &fakeInvoker{out: &lambda.InvokeOutput{FunctionError: &fe, Payload: []byte(`{"errorMessage":"Task timed out"}`)}}
	_, err := invokeSefazLambda(context.Background(), f, "dev-go-dfe-egress", map[string]any{"doc_type": "nfe"})
	var p *problem.Problem
	if !errors.As(err, &p) || p.Status != 500 {
		t.Fatalf("err = %#v, want a 500 problem", err)
	}
}
