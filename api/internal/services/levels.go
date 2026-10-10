package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// LevelSweepInterval is how often every API instance delivers pending levels.
// Several instances sweeping at once is harmless: they send identical bodies
// under identical keys.
const LevelSweepInterval = 2 * time.Minute

type levelSink interface {
	ReportLevel(ctx context.Context, in billingclient.LevelReport) error
}

// LevelKey is the idempotency key of one version of an organization's
// companies level: a retry of that version is the same report.
func LevelKey(organizationID string, version int64) string {
	return fmt.Sprintf("%s:%s:v%d", MeterLevelCompanies, organizationID, version)
}

// LevelReporter delivers dirty companies levels to billing, durably: a marker
// leaves the dirty index only when billing accepted its version.
type LevelReporter struct {
	repo    *repositories.AccountBillingRepository
	sink    levelSink
	billing *BillingService
}

// NewLevelReporter builds the reporter. A nil sink (billing off) makes every
// method a no-op.
func NewLevelReporter(repo *repositories.AccountBillingRepository, sink levelSink, billing *BillingService) *LevelReporter {
	return &LevelReporter{repo: repo, sink: sink, billing: billing}
}

func (r *LevelReporter) off() bool { return r == nil || r.sink == nil }

// Flush reports the organization's companies level if its marker is dirty.
func (r *LevelReporter) Flush(ctx context.Context, organizationID string) error {
	return r.flush(ctx, organizationID)
}

// FlushEnabled is Flush right after companyPK was enabled: the count includes
// it even if the organization-index has not caught up with it yet.
func (r *LevelReporter) FlushEnabled(ctx context.Context, organizationID, companyPK string) error {
	return r.flush(ctx, organizationID, companyPK)
}

func (r *LevelReporter) flush(ctx context.Context, organizationID string, include ...string) error {
	if r.off() {
		return nil
	}
	m, err := r.repo.GetLevelMarker(ctx, organizationID, MeterLevelCompanies)
	if err != nil || m == nil || !m.Dirty {
		return err
	}
	count, err := r.billing.companiesUsed(ctx, organizationID, include...)
	if err != nil {
		return err
	}
	err = r.sink.ReportLevel(ctx, billingclient.LevelReport{
		CustomerRef:    repositories.OrgBillingPK(organizationID),
		Meter:          MeterLevelCompanies,
		Value:          count,
		OccurredAt:     m.ChangedAt,
		IdempotencyKey: LevelKey(organizationID, m.Version),
	})
	// 409 idempotency_key_reused: billing already holds this key with another
	// body, i.e. this version was recorded; the next change reports the current
	// count anyway. Any other error (409 concurrent_update included: nothing was
	// recorded) keeps the marker dirty for the sweeper.
	if err != nil && !errors.Is(err, billingclient.ErrLevelAlreadyRecorded) {
		return err
	}
	return r.repo.ClearLevelDirty(ctx, organizationID, MeterLevelCompanies, m.Version)
}

// MarkAndFlush marks the level dirty and delivers it (the migration command:
// an organization's initial level). A failed delivery stays for the sweeper.
func (r *LevelReporter) MarkAndFlush(ctx context.Context, organizationID string) error {
	if r.off() {
		return nil
	}
	if err := r.repo.MarkLevelDirty(ctx, organizationID, MeterLevelCompanies); err != nil {
		return err
	}
	return r.Flush(ctx, organizationID)
}

// Sweep delivers every pending marker. One failure does not stop the others.
func (r *LevelReporter) Sweep(ctx context.Context) error {
	if r.off() {
		return nil
	}
	pending, err := r.repo.ListDirtyLevels(ctx)
	if err != nil {
		return err
	}
	for _, m := range pending {
		if m.Meter != MeterLevelCompanies {
			continue
		}
		if err := r.Flush(ctx, m.OrganizationID); err != nil {
			slog.WarnContext(ctx, "billing: level still pending; retried next sweep",
				"organization_id", m.OrganizationID, "version", m.Version, "error", err)
		}
	}
	return nil
}

// Run sweeps until ctx ends.
func (r *LevelReporter) Run(ctx context.Context, every time.Duration) {
	if r.off() {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.Sweep(ctx); err != nil {
				slog.WarnContext(ctx, "billing: level sweep failed", "error", err)
			}
		}
	}
}
