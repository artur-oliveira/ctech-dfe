package v1

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type recordingConfig struct {
	upserts int
	extra   []types.TransactWriteItem
	err     error
}

func (r *recordingConfig) Get(context.Context, string) (map[string]types.AttributeValue, error) {
	return nil, nil
}

func (r *recordingConfig) Upsert(_ context.Context, _ string, _ map[string]types.AttributeValue, _, _ string, extra ...types.TransactWriteItem) (map[string]types.AttributeValue, error) {
	r.upserts++
	r.extra = extra
	return map[string]types.AttributeValue{}, r.err
}

type fakeReserver struct {
	reservation *services.CompanyReservation
	err         error
}

type fakeFlusher struct{ flushed int }

func (f *fakeFlusher) Flush(context.Context, string) error {
	f.flushed++
	return nil
}

func (f *fakeReserver) OrganizationOf(context.Context, string) (string, error) { return "org_1", nil }
func (f *fakeReserver) ReserveCompany(context.Context, string, string) (*services.CompanyReservation, error) {
	return f.reservation, f.err
}

// Spec § 5 test 7: above the limit → 402 and no configuration written.
func TestARefusedEnablementWritesNoConfiguration(t *testing.T) {
	res := &fakeReserver{err: problem.QuotaExceeded(services.MeterCompanies, "free", 1, 1, "limite")}
	_, err := reserveEnablement(context.Background(), res, "cmp_1")
	var p *problem.Problem
	if !errors.As(err, &p) || p.Status != 402 {
		t.Fatalf("err = %v, want 402", err)
	}
	// The handler returns here: neither the série claim nor the write runs.
}

func TestAnEnablementCommitsTheGuardWithTheConfigurationAndReports(t *testing.T) {
	guard := types.TransactWriteItem{}
	cfg := &recordingConfig{}
	marker := types.TransactWriteItem{}
	res := &fakeReserver{reservation: &services.CompanyReservation{Items: []types.TransactWriteItem{guard, marker}, OrganizationID: "org_1", CompanyPK: "cmp_1"}}
	levels := &fakeFlusher{}
	r, err := reserveEnablement(context.Background(), res, "cmp_1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, r, "cmp_1", nil, "usr_1", "Fulano"); err != nil {
		t.Fatal(err)
	}
	if cfg.upserts != 1 || len(cfg.extra) != 2 || levels.flushed != 1 {
		t.Fatalf("upserts=%d extra=%d flushed=%d", cfg.upserts, len(cfg.extra), levels.flushed)
	}
}

func TestAnEnabledCompanySavesWithoutGuardOrReport(t *testing.T) {
	cfg := &recordingConfig{}
	res := &fakeReserver{reservation: &services.CompanyReservation{}}
	levels := &fakeFlusher{}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, res.reservation, "cmp_1", nil, "usr_1", "Fulano"); err != nil {
		t.Fatal(err)
	}
	if len(cfg.extra) != 0 || levels.flushed != 0 {
		t.Fatalf("extra=%d flushed=%d", len(cfg.extra), levels.flushed)
	}
}

// A failed write delivers nothing (and wrote no marker).
func TestAFailedWriteReportsNothing(t *testing.T) {
	guard := types.TransactWriteItem{}
	cfg := &recordingConfig{err: errors.New("boom")}
	res := &fakeReserver{reservation: &services.CompanyReservation{Items: []types.TransactWriteItem{guard}, OrganizationID: "org_1"}}
	levels := &fakeFlusher{}
	if _, err := writeFiscalConfig(context.Background(), cfg, levels, res.reservation, "cmp_1", nil, "usr_1", "Fulano"); err == nil {
		t.Fatal("want the write error")
	}
	if levels.flushed != 0 {
		t.Fatal("flushed a level whose change was not written")
	}
}
