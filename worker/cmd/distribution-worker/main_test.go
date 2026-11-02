package main

import (
	"context"
	"errors"
	"testing"

	"gopkg.aoctech.app/dfe/worker/internal/service"
)

type fakeProc struct {
	got []service.DistributionMessage
	err error
}

func (f *fakeProc) Process(_ context.Context, m service.DistributionMessage) error {
	f.got = append(f.got, m)
	return f.err
}

func TestHandleParsesDirectJob(t *testing.T) {
	p := &fakeProc{}
	raw := `{"job_type":"dist_nsu","org_pk":"CNPJ_1","doc_type":"nfe","trigger":"scheduler"}`
	if err := handle(context.Background(), []byte(raw), p); err != nil {
		t.Fatal(err)
	}
	if len(p.got) != 1 || p.got[0].OrgPK != "CNPJ_1" || p.got[0].JobType != "dist_nsu" {
		t.Fatalf("got %+v", p.got)
	}
}

func TestHandlePingAndErrors(t *testing.T) {
	p := &fakeProc{}
	if err := handle(context.Background(), []byte(`{"ping":true}`), p); err != nil || len(p.got) != 0 {
		t.Fatalf("ping must be a no-op: err=%v got=%v", err, p.got)
	}
	if err := handle(context.Background(), []byte(`not json`), p); err == nil {
		t.Fatal("unparseable job must fail so async retry/DLQ runs")
	}
	boom := errors.New("boom")
	if err := handle(context.Background(), []byte(`{"org_pk":"x"}`), &fakeProc{err: boom}); !errors.Is(err, boom) {
		t.Fatalf("process error not returned: %v", err)
	}
}
