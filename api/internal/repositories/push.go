package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// Web Push (docs/specs/2026-11-17-web-push.md). One table, by prefix:
//
//	pk = USER#{sub}      sk = SUB#{sha256(endpoint)[:16]}   one browser subscription (rows expire in PushSubscriptionTTL)
//	                     sk = META#counts                   subscriptions count, the limit's guard
//	pk = COMPANY#{pk}    sk = USER#{sub}                    the events this person wants for the company
//	                     sk = EVENT#{dedupe}                marker: this event was already handed to the sender (24 h)
//	                     sk = THROTTLE#{event type}         the grouping window of one company and type
//
// The sparse `user-index` (pk user_sub, sk pk, KEYS_ONLY) holds only the preference rows: the purge of a person and
// the removal of a member find their rows without a Scan. The endpoint and the keys are not sealed (the spec's
// reasoning: a push service refuses a send without the VAPID private key, and the table is encrypted at rest).
const (
	TablePush     = "push"
	IndexPushUser = "user-index"
	// IndexPushEndpoint is sparse like IndexPushUser: only subscription rows carry `endpoint_id` (the hash in their sk), so
	// the rows of one browser endpoint are found across people without a Scan (see PushRepository.OtherOwners).
	IndexPushEndpoint = "endpoint-index"

	// MaxPushSubscriptionsPerUser is the most devices one person may register; the next replaces the least recently seen.
	MaxPushSubscriptionsPerUser = 10
	// MaxPushEndpointLen bounds a stored endpoint (webpush.MaxEndpointLen).
	MaxPushEndpointLen = 2048
	// PushSubscriptionTTL is how long a subscription lives without a sync; the client renews it on every app load.
	PushSubscriptionTTL = 60 * 24 * time.Hour
	// PushDedupeTTL is how long an event marker is kept: past any queue redelivery.
	PushDedupeTTL = 24 * time.Hour
	// PushThrottleGrace keeps a window row after it closes, so a clock skew cannot reopen it early.
	PushThrottleGrace = time.Hour

	pushUserPrefix    = "USER#"
	pushCompanyPrefix = "COMPANY#"
	pushSubPrefix     = "SUB#"
	pushCountsSK      = "META#counts"

	// AttrPushUserSub is the partition key of the user-index: who a preference row belongs to.
	AttrPushUserSub = "user_sub"
	// AttrPushEndpointID is the partition key of the endpoint-index: the endpoint hash of a subscription row.
	AttrPushEndpointID = "endpoint_id"
)

// ErrPushSubscriptionNotFound reports a subscription that does not exist for this person.
var ErrPushSubscriptionNotFound = errors.New("push subscription not found")

// PushSubscription is one browser of one person.
type PushSubscription struct {
	ID         string `dynamodbav:"id"           json:"id"`
	Endpoint   string `dynamodbav:"endpoint"     json:"-"`
	P256dh     string `dynamodbav:"p256dh"       json:"-"`
	Auth       string `dynamodbav:"auth"         json:"-"`
	VapidKID   string `dynamodbav:"vapid_kid"    json:"vapid_kid"`
	Device     string `dynamodbav:"device"       json:"device"`
	CreatedAt  string `dynamodbav:"created_at"   json:"created_at"`
	LastSeenAt string `dynamodbav:"last_seen_at" json:"last_seen_at"`
}

// String redacts: the endpoint is a per-subscription token and never reaches a log.
func (s PushSubscription) String() string { return "PushSubscription{id=" + s.ID + "}" }

// GoString mirrors String for %#v.
func (s PushSubscription) GoString() string { return s.String() }

// PushUserPK, PushSubSK, PushCompanyPK and PushPrefSK build the keys, exported for the tests that read a raw row.
func PushUserPK(sub string) string        { return pushUserPrefix + RawUserID(sub) }
func PushSubSK(id string) string          { return pushSubPrefix + id }
func PushCompanyPK(company string) string { return pushCompanyPrefix + company }
func PushPrefSK(sub string) string        { return pushUserPrefix + RawUserID(sub) }

// PushSubscriptionID names a subscription by its endpoint: 16 hex characters of the SHA-256. An upsert of the same
// browser lands on the same row.
func PushSubscriptionID(endpoint string) string {
	sum := sha256.Sum256([]byte(endpoint))
	return hex.EncodeToString(sum[:8])
}

func pushEventSK(key string) string    { return "EVENT#" + key }
func pushThrottleSK(typ string) string { return "THROTTLE#" + typ }

// PushRepository stores subscriptions, preferences and the grouping markers.
type PushRepository struct {
	base Base
	now  func() time.Time
}

func NewPushRepository(db *dynamodb.Client, cfg *config.Config) *PushRepository {
	return &PushRepository{base: NewBase(db, cfg, TablePush), now: time.Now}
}

// pushStamp is fixed width (nanoseconds always printed), so comparing two stamps as strings orders them in time;
// RFC3339Nano drops trailing zeros and does not.
func pushStamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000000000Z") }

func pushTTL(at time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(at.Unix(), 10)}
}

func nAV(n int) types.AttributeValue { return &types.AttributeValueMemberN{Value: strconv.Itoa(n)} }

// ── Subscriptions ────────────────────────────────────────────────────────────

// ListSubscriptions returns the person's devices, oldest first by key. At most MaxPushSubscriptionsPerUser rows exist.
func (r *PushRepository) ListSubscriptions(ctx context.Context, sub string) ([]PushSubscription, error) {
	res, err := r.base.Query(ctx, QueryOpts{
		PK: PushUserPK(sub), SKPrefix: pushSubPrefix, ScanIndexForward: true, Limit: MaxPushSubscriptionsPerUser + 1,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PushSubscription, 0, len(res.Items))
	for _, it := range res.Items {
		var s PushSubscription
		if err := attributevalue.UnmarshalMap(it, &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (r *PushRepository) subscriptionItem(sub string, s PushSubscription, now time.Time) (map[string]types.AttributeValue, error) {
	item, err := MarshalMapOmitNull(s)
	if err != nil {
		return nil, err
	}
	item["pk"] = SAV(PushUserPK(sub))
	item["sk"] = SAV(PushSubSK(s.ID))
	item[AttrPushEndpointID] = SAV(s.ID)
	item["ttl"] = pushTTL(now.Add(PushSubscriptionTTL))
	return item, nil
}

// OtherOwners returns the people other than `sub` who hold a subscription for this endpoint. One browser endpoint
// belongs to one person at a time: when someone signs in on a shared browser the previous person's row must go, or both
// would be notified on the same screen. The endpoint-index is eventually consistent; the logout cleanup (the browser
// subscription is cancelled first, so the push service answers 410 and the sender drops the row) is the primary
// guard and this is the one that corrects a row the logout never reached.
func (r *PushRepository) OtherOwners(ctx context.Context, endpoint, sub string) ([]string, error) {
	id := PushSubscriptionID(endpoint)
	mine := PushUserPK(sub)
	var owners []string
	var start map[string]types.AttributeValue
	for {
		res, err := r.base.Query(ctx, QueryOpts{
			PK: id, PKField: AttrPushEndpointID, IndexName: IndexPushEndpoint, Limit: 25, ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, err
		}
		for _, it := range res.Items {
			pk, ok := it["pk"].(*types.AttributeValueMemberS)
			if ok && pk.Value != mine && strings.HasPrefix(pk.Value, pushUserPrefix) {
				owners = append(owners, strings.TrimPrefix(pk.Value, pushUserPrefix))
			}
		}
		if res.LastEvaluatedKey == nil {
			return owners, nil
		}
		start = res.LastEvaluatedKey
	}
}

// UpsertSubscription registers or renews a device and returns its id. A known endpoint only renews last_seen_at and the
// keys (a PUT is idempotent). A new one takes a slot under the count row's guard; a full person loses the device seen
// least recently, in the same transaction, so the count never passes MaxPushSubscriptionsPerUser.
func (r *PushRepository) UpsertSubscription(ctx context.Context, sub string, s PushSubscription) (string, error) {
	now := r.now()
	s.ID = PushSubscriptionID(s.Endpoint)
	s.LastSeenAt = pushStamp(now)
	existing, err := r.ListSubscriptions(ctx, sub)
	if err != nil {
		return "", err
	}
	if i := slices.IndexFunc(existing, func(e PushSubscription) bool { return e.ID == s.ID }); i >= 0 {
		s.CreatedAt = existing[i].CreatedAt
		item, err := r.subscriptionItem(sub, s, now)
		if err != nil {
			return "", err
		}
		return s.ID, r.base.PutItem(ctx, item)
	}
	s.CreatedAt = s.LastSeenAt
	item, err := r.subscriptionItem(sub, s, now)
	if err != nil {
		return "", err
	}
	pk := PushUserPK(sub)
	if len(existing) < MaxPushSubscriptionsPerUser {
		count := r.base.BuildRawUpdateTxItem(pk, new(pushCountsSK),
			"SET subscriptions = if_not_exists(subscriptions, :zero) + :one",
			"attribute_not_exists(subscriptions) OR subscriptions < :max", nil,
			map[string]types.AttributeValue{":zero": nAV(0), ":one": nAV(1), ":max": nAV(MaxPushSubscriptionsPerUser)})
		err = r.base.TransactWrite(ctx, []types.TransactWriteItem{count, r.base.BuildPutTxItemIfAbsent(item)})
		if err == nil || !IsConditionFailed(err) {
			return s.ID, err
		}
		// Another request filled the last slot meanwhile: evict like a full person.
		if existing, err = r.ListSubscriptions(ctx, sub); err != nil {
			return "", err
		}
	}
	if len(existing) == 0 {
		return s.ID, errors.New("push: subscription slots are full but no device was found")
	}
	oldest := existing[0]
	for _, e := range existing[1:] {
		if e.LastSeenAt < oldest.LastSeenAt {
			oldest = e
		}
	}
	del := r.base.BuildDeleteTxItem(pk, PushSubSK(oldest.ID))
	return s.ID, r.base.TransactWrite(ctx, []types.TransactWriteItem{del, r.base.BuildPutTxItemIfAbsent(item)})
}

// DeleteSubscription removes one device of the person and frees its slot.
func (r *PushRepository) DeleteSubscription(ctx context.Context, sub, id string) error {
	pk := PushUserPK(sub)
	del := types.TransactWriteItem{Delete: &types.Delete{
		TableName:           aws.String(r.base.TableName),
		Key:                 map[string]types.AttributeValue{"pk": SAV(pk), "sk": SAV(PushSubSK(id))},
		ConditionExpression: aws.String("attribute_exists(pk)"),
	}}
	count := r.base.BuildRawUpdateTxItem(pk, new(pushCountsSK),
		"SET subscriptions = subscriptions - :one", "subscriptions > :zero", nil,
		map[string]types.AttributeValue{":zero": nAV(0), ":one": nAV(1)})
	err := r.base.TransactWrite(ctx, []types.TransactWriteItem{del, count})
	if IsConditionFailed(err) {
		return ErrPushSubscriptionNotFound
	}
	return err
}

// ── Preferences ──────────────────────────────────────────────────────────────

type pushPreference struct {
	Events    []string `dynamodbav:"events"`
	UserSub   string   `dynamodbav:"user_sub"`
	UpdatedAt string   `dynamodbav:"updated_at"`
}

// GetPreference returns the events the person wants for the company; nil when they never chose.
func (r *PushRepository) GetPreference(ctx context.Context, company, sub string) ([]string, error) {
	item, err := r.base.GetItem(ctx, PushCompanyPK(company), PushPrefSK(sub))
	if err != nil || item == nil {
		return nil, err
	}
	var p pushPreference
	if err := attributevalue.UnmarshalMap(item, &p); err != nil {
		return nil, err
	}
	return p.Events, nil
}

// PutPreference saves the events (already validated); an empty list removes the row, and the sparse index with it.
func (r *PushRepository) PutPreference(ctx context.Context, company, sub string, events []string) error {
	if len(events) == 0 {
		_, err := r.base.DeleteItem(ctx, PushCompanyPK(company), PushPrefSK(sub))
		return err
	}
	item, err := MarshalMapOmitNull(pushPreference{Events: events, UserSub: RawUserID(sub), UpdatedAt: pushStamp(r.now())})
	if err != nil {
		return err
	}
	item["pk"] = SAV(PushCompanyPK(company))
	item["sk"] = SAV(PushPrefSK(sub))
	return r.base.PutItem(ctx, item)
}

// SeedPreference stores the events only when the person has no row for the company yet: the defaults a new device
// starts with, never over a choice already made. True when it wrote.
func (r *PushRepository) SeedPreference(ctx context.Context, company, sub string, events []string) (bool, error) {
	item, err := MarshalMapOmitNull(pushPreference{Events: events, UserSub: RawUserID(sub), UpdatedAt: pushStamp(r.now())})
	if err != nil {
		return false, err
	}
	item["pk"] = SAV(PushCompanyPK(company))
	item["sk"] = SAV(PushPrefSK(sub))
	err = r.base.TransactWrite(ctx, []types.TransactWriteItem{r.base.BuildPutTxItemIfAbsent(item)})
	if IsConditionFailed(err) {
		return false, nil
	}
	return err == nil, err
}

// DeletePreference drops one person's choice for one company (a removed member).
func (r *PushRepository) DeletePreference(ctx context.Context, company, sub string) error {
	_, err := r.base.DeleteItem(ctx, PushCompanyPK(company), PushPrefSK(sub))
	return err
}

// PushRecipientPage is one page of the people who want an event.
type PushRecipientPage struct {
	Subs []string
	Next map[string]types.AttributeValue
}

// ListRecipients pages the company's preference rows and keeps the people who ticked the event. The page is bounded
// (limit) so a company with thousands of members is never read in one call; the caller follows Next.
func (r *PushRepository) ListRecipients(ctx context.Context, company, event string, limit int, start map[string]types.AttributeValue) (PushRecipientPage, error) {
	res, err := r.base.Query(ctx, QueryOpts{
		PK: PushCompanyPK(company), SKPrefix: pushUserPrefix, ScanIndexForward: true, Limit: limit, ExclusiveStartKey: start,
	})
	if err != nil {
		return PushRecipientPage{}, err
	}
	page := PushRecipientPage{Next: res.LastEvaluatedKey}
	for _, it := range res.Items {
		var p pushPreference
		if err := attributevalue.UnmarshalMap(it, &p); err != nil {
			return PushRecipientPage{}, err
		}
		if p.UserSub != "" && slices.Contains(p.Events, event) {
			page.Subs = append(page.Subs, p.UserSub)
		}
	}
	return page, nil
}

// ── Markers ──────────────────────────────────────────────────────────────────

// MarkEvent claims the event for the sender. False means it was already claimed: a queue redelivery.
func (r *PushRepository) MarkEvent(ctx context.Context, company, dedupeKey string) (bool, error) {
	item := map[string]types.AttributeValue{
		"pk": SAV(PushCompanyPK(company)), "sk": SAV(pushEventSK(dedupeKey)), "ttl": pushTTL(r.now().Add(PushDedupeTTL)),
	}
	err := r.base.TransactWrite(ctx, []types.TransactWriteItem{r.base.BuildPutTxItemIfAbsent(item)})
	if IsConditionFailed(err) {
		return false, nil
	}
	return err == nil, err
}

// UnmarkEvent releases an event claim after the hand-off failed, so the redelivery can try again.
func (r *PushRepository) UnmarkEvent(ctx context.Context, company, dedupeKey string) error {
	_, err := r.base.DeleteItem(ctx, PushCompanyPK(company), pushEventSK(dedupeKey))
	return err
}

// OpenWindow takes the grouping window of one company and event type: true for the first event of the window, which
// closes it for `window`; false while it is open.
func (r *PushRepository) OpenWindow(ctx context.Context, company, eventType string, window time.Duration) (bool, error) {
	return r.openWindow(ctx, PushCompanyPK(company), eventType, window)
}

// OpenUserWindow is OpenWindow for one person (the test notification's rate limit).
func (r *PushRepository) OpenUserWindow(ctx context.Context, sub, eventType string, window time.Duration) (bool, error) {
	return r.openWindow(ctx, PushUserPK(sub), eventType, window)
}

func (r *PushRepository) openWindow(ctx context.Context, pk, eventType string, window time.Duration) (bool, error) {
	now := r.now()
	until := now.Add(window)
	_, err := r.base.PutItemRaw(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.base.TableName),
		Item: map[string]types.AttributeValue{
			"pk": SAV(pk), "sk": SAV(pushThrottleSK(eventType)),
			"until": SAV(pushStamp(until)), "ttl": pushTTL(until.Add(PushThrottleGrace)),
		},
		ConditionExpression:       aws.String("attribute_not_exists(pk) OR #u < :now"),
		ExpressionAttributeNames:  map[string]string{"#u": "until"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":now": SAV(pushStamp(now))},
	})
	if IsConditionFailed(err) {
		return false, nil
	}
	return err == nil, err
}

// CloseWindow reopens a window after the hand-off failed.
func (r *PushRepository) CloseWindow(ctx context.Context, company, eventType string) error {
	_, err := r.base.DeleteItem(ctx, PushCompanyPK(company), pushThrottleSK(eventType))
	return err
}
