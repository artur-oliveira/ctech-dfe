package services

import (
	"context"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// auditHousekeepingKeys are never surfaced as modifications — they're storage
// mechanics, not business data.
var auditHousekeepingKeys = map[string]bool{
	"pk": true, "sk": true, "created_at": true, "updated_at": true,
}

// Diff compares before/after field maps and returns only the fields that
// changed. A field present on only one side is reported with nil on the other
// (used for CREATE — pass before=nil — and DELETE — pass after=nil).
func Diff(before, after map[string]any) []repositories.Modification {
	seen := make(map[string]bool)
	var mods []repositories.Modification

	visit := func(key string) {
		if seen[key] || auditHousekeepingKeys[key] {
			return
		}
		seen[key] = true
		b, a := before[key], after[key]
		if reflect.DeepEqual(b, a) {
			return
		}
		mods = append(mods, repositories.Modification{Name: key, Before: b, After: a})
	}
	for k := range before {
		visit(k)
	}
	for k := range after {
		visit(k)
	}
	return mods
}

// AuditLogQueryOpts selects which audit_logs index to query.
type AuditLogQueryOpts struct {
	ResourceType string // with ResourceID: base-table query, full history of one resource
	ResourceID   string
	UserID       string // user-id-index: everything one user did
	Limit        int
	StartKey     map[string]types.AttributeValue
}

// AuditLogService lists audit_logs rows for an org.
type AuditLogService struct {
	repo *repositories.AuditLogRepository
}

func NewAuditLogService(repo *repositories.AuditLogRepository) *AuditLogService {
	return &AuditLogService{repo: repo}
}

// List picks the right index based on which filters are set: base table
// (resource history) > user-id-index (per-user) > org-time-index (default feed).
// The user-id-index query is additionally filtered back down to the caller's
// own org, since that GSI's partition key is user_id, not the org.
func (s *AuditLogService) List(ctx context.Context, orgPK string, opts AuditLogQueryOpts) (*repositories.QueryResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if opts.ResourceType != "" {
		skPfx := fmt.Sprintf("%s", opts.ResourceType)
		if opts.ResourceID != "" {
			skPfx = skPfx + "#" + opts.ResourceID
		}
		return s.repo.Query(ctx, repositories.QueryOpts{
			PK:               orgPK,
			SKPrefix:         skPfx,
			ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	if opts.UserID != "" {
		return s.repo.Query(ctx, repositories.QueryOpts{
			PK: opts.UserID, PKField: "user_id", IndexName: "user-id-index",
			FilterField: "pk", FilterValue: orgPK,
			ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return s.repo.Query(ctx, repositories.QueryOpts{
		PK: orgPK, IndexName: "org-time-index",
		ScanIndexForward: false, Limit: limit, ExclusiveStartKey: opts.StartKey,
	})
}
