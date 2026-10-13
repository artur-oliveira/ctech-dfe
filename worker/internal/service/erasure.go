package service

import (
	"context"
	"strings"

	"gopkg.aoctech.app/api-commons/erasure"
)

// OrgStopped reports whether the account-deletion saga locked or erased a
// company (ORG#{company_id} in {prefix}_erasure_state). Its jobs are dropped:
// running them would write rows the purge is deleting (saga protocol §4.5).
type OrgStopped func(ctx context.Context, companyID string) (bool, error)

func NewOrgStopped(store *erasure.Store) OrgStopped {
	return func(ctx context.Context, companyID string) (bool, error) {
		rec, err := store.Get(ctx, erasure.OrgKey(companyID))
		if err != nil {
			return false, err
		}
		return rec != nil && rec.State != erasure.StateActive, nil
	}
}

// companyOfDocPK is the company half of a {env}#{company} document key.
func companyOfDocPK(docPK string) string {
	_, company, _ := strings.Cut(docPK, "#")
	return company
}

func (f OrgStopped) check(ctx context.Context, companyID string) (bool, error) {
	if f == nil || companyID == "" {
		return false, nil
	}
	return f(ctx, companyID)
}
