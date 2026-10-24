package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"gopkg.aoctech.app/api-commons/erasure"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	erasedActorName        = "Usuário removido"
	systemActorID          = "SYSTEM"
	systemErasureActorName = "Sistema (exclusão de conta LGPD)"
	attrErasureRequest     = "erasure_request"
	calendarMonthLayout    = "2006-01"
)

// ErrErasureSettling: a company was locked less than the settle window ago.
// The consumer leaves the message, and the redelivery finishes the purge.
var ErrErasureSettling = errors.New("erasure: companies locked less than the settle window ago; waiting for redelivery")

// Purge is the erasure.PurgeFunc (saga protocol §4.3): erase every company of
// the organizations erased with the user (D4), then the person. Idempotent and
// resumable: every step re-reads its rows.
func (s *ErasureService) Purge(ctx context.Context, m erasure.Message) (erasure.Ack, error) {
	counts := map[string]int{}
	companies, err := s.companiesOf(ctx, m.Organizations)
	if err != nil {
		return erasure.Ack{}, err
	}
	settled := true
	for _, c := range companies {
		ok, err := s.lockCompany(ctx, c, m)
		if err != nil {
			return erasure.Ack{}, err
		}
		settled = settled && ok
	}
	for _, c := range companies {
		if err := s.purgeCompany(ctx, c, counts); err != nil {
			return erasure.Ack{}, fmt.Errorf("erasure: company %s: %w", c, err)
		}
	}
	if !settled {
		return erasure.Ack{}, ErrErasureSettling
	}
	for _, c := range companies {
		if _, err := s.store.Apply(ctx, erasure.OrgKey(c), m); err != nil {
			return erasure.Ack{}, err
		}
		existed, err := s.repo.DeleteKey(ctx, tblOrganizations, pkKey(c))
		if err != nil {
			return erasure.Ack{}, err
		}
		if existed {
			counts[tblOrganizations]++
		}
		cacheDelete(ctx, s.cache, "dfe:org:"+c)
	}
	for _, id := range m.Organizations {
		if err := s.eraseOrganizationBilling(ctx, id, counts); err != nil {
			return erasure.Ack{}, err
		}
		cacheDelete(ctx, s.cache, orgBillingCacheKey(id))
	}
	if err := s.purgePerson(ctx, m, counts); err != nil {
		return erasure.Ack{}, err
	}
	return erasure.Ack{Result: erasure.ResultDone, Counts: counts}, nil
}

// companiesOf maps ctech-account organization ids to dfe company ids
// (ruling R4). Empty after the organizations items are gone: done.
func (s *ErasureService) companiesOf(ctx context.Context, orgIDs []string) ([]string, error) {
	var out []string
	for _, id := range orgIDs {
		var start map[string]types.AttributeValue
		for {
			page, err := s.repo.QueryIndex(ctx, tblOrganizations, organizationIDIndex, repositories.AttrOrganizationID, id, start)
			if err != nil {
				return nil, err
			}
			for _, it := range page.Items {
				out = append(out, avAttr(it, "pk"))
			}
			if page.Next == nil {
				break
			}
			start = page.Next
		}
	}
	return out, nil
}

// lockCompany writes ORG#{company} locked (workers drop its jobs from then on)
// and reports whether the lock is older than the settle window.
func (s *ErasureService) lockCompany(ctx context.Context, company string, m erasure.Message) (bool, error) {
	key := erasure.OrgKey(company)
	rec, err := s.store.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if rec == nil || rec.State == erasure.StateActive {
		lock := m
		lock.Type = erasure.TypeLocked
		applied, err := s.store.Apply(ctx, key, lock)
		if err != nil {
			return false, err
		}
		rec = &applied
	}
	switch rec.State {
	case erasure.StateErased:
		return true, nil
	case erasure.StateLocked:
		at, err := time.Parse(time.RFC3339, rec.UpdatedAt)
		if err != nil {
			return false, fmt.Errorf("erasure: company %s lock time: %w", company, err)
		}
		return s.now().Sub(at) >= s.settle, nil
	}
	return false, fmt.Errorf("erasure: company %s could not be locked", company)
}

// purgePerson implements inventory §5's person table.
func (s *ErasureService) purgePerson(ctx context.Context, m erasure.Message, counts map[string]int) error {
	raw := repositories.RawUserID(m.Sub)
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblOrgUsers, userIndex, skKey, repositories.BuildMemberSK(raw), start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.leaveOrganization(ctx, avAttr(it, "pk"), raw, counts); err != nil {
				return err
			}
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	if err := s.anonymizeActor(ctx, raw, counts); err != nil {
		return err
	}
	if err := s.purgeECPF(ctx, m.RequestID, counts); err != nil {
		return err
	}
	if err := s.eraseBilling(ctx, raw, counts); err != nil {
		return err
	}
	existed, err := s.repo.DeleteKey(ctx, tblUsers, pkKey(repositories.BuildUserPK(raw)))
	if err != nil {
		return err
	}
	if existed {
		counts[tblUsers]++
	}
	for _, k := range []string{userItemCacheKey(raw), userMeCacheKey(raw), userOrgsCacheKey(raw)} {
		cacheDelete(ctx, s.cache, k)
	}
	return nil
}

func anonymousActor(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "erased:" + hex.EncodeToString(sum[:8])
}

func sameUser(stored, raw string) bool {
	return stored != "" && repositories.RawUserID(stored) == raw
}

// leaveOrganization removes the person from a surviving organization: the
// invitations they received go, the ones they sent lose the inviter, the
// member audit rows lose their payload, and the membership row goes.
func (s *ErasureService) leaveOrganization(ctx context.Context, org, raw string, counts map[string]int) error {
	anon := map[string]types.AttributeValue{
		"invited_by": repositories.SAV(anonymousActor(raw)), "invited_by_name": repositories.SAV(erasedActorName),
	}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblInvitations, orgInviteIndex, "org_pk", org, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			switch {
			case sameUser(avAttr(it, "accepted_by"), raw):
				existed, err := s.repo.DeleteKey(ctx, tblInvitations, rowKey(it, ""))
				if err != nil {
					return err
				}
				if existed {
					counts[tblInvitations]++
				}
			case sameUser(avAttr(it, "invited_by"), raw):
				if err := s.repo.SetAttrs(ctx, tblInvitations, rowKey(it, ""), anon); err != nil {
					return err
				}
				counts["organization_invitations_anonymized"]++
			}
		}
		if page.Next == nil {
			break
		}
		start = page.Next
	}
	memberSK := repositories.BuildMemberSK(raw)
	for _, id := range []string{raw, memberSK} { // both spellings are in use as resource_id
		if err := s.scrubAudit(ctx, org, repositories.AuditResourceMember+"#"+id+"#", counts); err != nil {
			return err
		}
	}
	existed, err := s.repo.DeleteKey(ctx, tblOrgUsers, map[string]types.AttributeValue{
		"pk": repositories.SAV(org), skKey: repositories.SAV(memberSK),
	})
	if err != nil {
		return err
	}
	if existed {
		counts[tblOrgUsers]++
	}
	cacheDelete(ctx, s.cache, memberCacheKey(org, raw))
	return nil
}

// scrubAudit empties modifications on audit rows of one resource (they may
// carry the person's name or an e-CPF alias with the CPF).
func (s *ErasureService) scrubAudit(ctx context.Context, org, skPrefix string, counts map[string]int) error {
	empty := map[string]types.AttributeValue{"modifications": &types.AttributeValueMemberL{Value: []types.AttributeValue{}}}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryPrefix(ctx, tblAuditLogs, org, skPrefix, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.repo.SetAttrs(ctx, tblAuditLogs, rowKey(it, skKey), empty); err != nil {
				return err
			}
			counts["audit_logs_scrubbed"]++
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// anonymizeActor replaces the person on every audit row they wrote, keeping
// the organization's trail (inventory §5).
func (s *ErasureService) anonymizeActor(ctx context.Context, raw string, counts map[string]int) error {
	attrs := map[string]types.AttributeValue{
		"user_id": repositories.SAV(anonymousActor(raw)), "user_name": repositories.SAV(erasedActorName),
	}
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblAuditLogs, userIDIndex, "user_id", raw, start)
		if err != nil {
			return err
		}
		for _, it := range page.Items {
			if err := s.repo.SetAttrs(ctx, tblAuditLogs, rowKey(it, skKey), attrs); err != nil {
				return err
			}
			counts["audit_logs_anonymized"]++
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

// purgeECPF erases the person's e-CPF certificates everywhere (D14). Pass 1
// marks matching rows while the PFX can still be opened (a branch row shares
// its matriz's object); pass 2 erases marked rows. The marker also lets a
// rerun finish a row whose PFX is already gone.
func (s *ErasureService) purgeECPF(ctx context.Context, requestID string, counts map[string]int) error {
	if s.subject == nil {
		return errors.New("erasure: ctech-account subject lookup is not configured")
	}
	cpf, err := s.subject(ctx, requestID)
	if err != nil {
		return fmt.Errorf("erasure: subject lookup: %w", err)
	}
	marker := map[string]types.AttributeValue{attrErasureRequest: repositories.SAV(requestID)}
	if err := s.scanCertificates(ctx, func(item map[string]types.AttributeValue) error {
		if avAttr(item, attrErasureRequest) == requestID {
			return nil
		}
		held, err := s.heldBy(ctx, item, cpf)
		if err != nil || !held {
			return err
		}
		return s.repo.SetAttrs(ctx, tblCertificates, rowKey(item, skKey), marker)
	}); err != nil {
		return err
	}
	return s.scanCertificates(ctx, func(item map[string]types.AttributeValue) error {
		if avAttr(item, attrErasureRequest) != requestID {
			return nil
		}
		return s.eraseCertificate(ctx, item, counts)
	})
}

// scanCertificates visits every certificate row through the table's sweep-index
// (plan ruling R8: certificates carry no attribute a company query could key
// on). The index is KEYS_ONLY, so each row is read whole, strongly consistent,
// before fn sees it; a row deleted between the page and the read is skipped.
func (s *ErasureService) scanCertificates(ctx context.Context, fn func(map[string]types.AttributeValue) error) error {
	var start map[string]types.AttributeValue
	for {
		page, err := s.repo.QueryIndex(ctx, tblCertificates, repositories.SweepIndex, repositories.AttrSweep, repositories.SweepValue, start)
		if err != nil {
			return err
		}
		for _, key := range page.Items {
			item, err := s.repo.GetItem(ctx, tblCertificates, map[string]types.AttributeValue{"pk": key["pk"], "sk": key["sk"]})
			if err != nil {
				return err
			}
			if item == nil {
				continue
			}
			if err := fn(item); err != nil {
				return err
			}
		}
		if page.Next == nil {
			return nil
		}
		start = page.Next
	}
}

func (s *ErasureService) heldBy(ctx context.Context, item map[string]types.AttributeValue, cpf string) (bool, error) {
	key := avAttr(item, "s3_key")
	if cpf == "" || key == "" {
		return false, nil
	}
	out, err := s.objects.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.certsBucket), Key: aws.String(key)})
	if err != nil {
		var missing *s3types.NoSuchKey
		if errors.As(err, &missing) {
			return false, nil
		}
		return false, fmt.Errorf("erasure: read certificate %s: %w", key, err)
	}
	defer closeReadCloser(ctx, out.Body, "erasure certificate read")
	pfx, err := io.ReadAll(out.Body)
	if err != nil {
		return false, fmt.Errorf("erasure: read certificate %s: %w", key, err)
	}
	return certificateHeldBy(pfx, avAttr(item, "password"), cpf), nil
}

// eraseCertificate deletes every version of the PFX, scrubs the certificate's
// audit history, then deletes the row (and its password) together with the
// audit notice the organization's admins see (ruling R10).
func (s *ErasureService) eraseCertificate(ctx context.Context, item map[string]types.AttributeValue, counts map[string]int) error {
	org, md5 := avAttr(item, "pk"), avAttr(item, "md5")
	n, err := purgeObjects(ctx, s.objects, s.certsBucket, avAttr(item, "s3_key"), true)
	if err != nil {
		return err
	}
	counts[countS3Versions] += n
	if err := s.scrubAudit(ctx, org, repositories.AuditResourceCertificate+"#"+md5+"#", counts); err != nil {
		return err
	}
	auditTx, err := s.audit.BuildLogTxItem(org, repositories.AuditResourceCertificate, md5,
		repositories.AuditActionDelete, systemActorID, systemErasureActorName, nil)
	if err != nil {
		return err
	}
	if err := s.certs.TransactWrite(ctx, []types.TransactWriteItem{s.certs.BuildDeleteTxItem(org, md5), auditTx}); err != nil {
		return err
	}
	counts["organization_certificates_ecpf"]++
	return nil
}

// eraseBilling removes the person's billing snapshot, quota guard and current
// usage counters (ruling R15).
func (s *ErasureService) eraseBilling(ctx context.Context, raw string, counts map[string]int) error {
	snap, err := s.repo.GetItem(ctx, tblAccountBilling, pkKey(repositories.AccountBillingPK(raw)))
	if err != nil {
		return err
	}
	keys := []string{
		repositories.AccountBillingPK(raw),
		repositories.QuotaGuardPK(raw, MeterCompanies),
		repositories.UsageCounterPK(raw, s.now().Format(calendarMonthLayout)),
	}
	if p := avAttr(snap, "period_start"); p != "" {
		keys = append(keys, repositories.UsageCounterPK(raw, p))
	}
	for _, k := range keys {
		existed, err := s.repo.DeleteKey(ctx, tblAccountBilling, pkKey(k))
		if err != nil {
			return err
		}
		if existed {
			counts[tblAccountBilling]++
		}
	}
	return nil
}

// usagePeriodsErased is how many calendar-month usage periods back the purge
// reaches (the rows carry a 13-month TTL, so older ones are already gone).
const usagePeriodsErased = 14

// billingKeyKinds lists every account_billing partition-key kind and how the
// account-deletion purge treats it; TestEveryAccountBillingKeyKindIsPurged fails
// when a new key builder has no entry here (plan ruling R15).
var billingKeyKinds = map[string]string{
	"USER_":                       "erased: legacy per-user snapshot (eraseBilling)",
	repositories.OrgBillingPrefix: "erased: organization snapshot (eraseOrganizationBilling)",
	"QUOTA_GUARD_":                "erased: concurrency guard per meter",
	"USAGE_":                      "erased: current period and the last calendar months; older rows expire by TTL",
	"LEVEL_DIRTY_":                "erased: pending level report is dropped, not sent",
	"EVENT_":                      "kept: processed webhook ids, no personal data, short TTL",
}

// eraseOrganizationBilling removes the local billing snapshot of an erased
// ctech-account organization (R15): the snapshot, the quota guard and level
// marker of every known meter, and its usage counters. The subscription itself
// is cancelled by ctech-billing.
func (s *ErasureService) eraseOrganizationBilling(ctx context.Context, organizationID string, counts map[string]int) error {
	snap, err := s.repo.GetItem(ctx, tblAccountBilling, pkKey(repositories.OrgBillingPK(organizationID)))
	if err != nil {
		return err
	}
	keys := []string{repositories.OrgBillingPK(organizationID)}
	meters := append(slices.Clone(DocumentMeters), MeterCompanies)
	for _, meter := range meters {
		keys = append(keys, repositories.QuotaGuardPK(organizationID, meter), repositories.LevelMarkerPK(organizationID, meter))
	}
	if p := avAttr(snap, "period_start"); p != "" {
		keys = append(keys, repositories.UsageCounterPK(organizationID, p))
	}
	month := s.now()
	for range usagePeriodsErased {
		keys = append(keys, repositories.UsageCounterPK(organizationID, month.Format(calendarMonthLayout)))
		month = month.AddDate(0, -1, 0)
	}
	for _, k := range keys {
		existed, err := s.repo.DeleteKey(ctx, tblAccountBilling, pkKey(k))
		if err != nil {
			return err
		}
		if existed {
			counts[tblAccountBilling]++
		}
	}
	return nil
}
