package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

// AccountBillingRepository stores what ctech-billing says about each account.
//
// Table structure (account_billing), three kinds of row:
//
//	pk = USER_{sub}                the subscription snapshot
//	pk = EVENT_{event_id}          a processed webhook, with a TTL
//	pk = USAGE_{sub}#{period}      this period's meters
//
// They share a table because they share a subject and their access pattern is
// identical — one row, by primary key, never queried or scanned. A table each
// would be three things to create, grant, prefix and remember, for no property
// gained; different pk values sit in different partitions, so they do not
// contend.
//
// **The snapshot is a cache with a durable floor, not a source of truth.**
// Billing owns the subscription; this row is what the DF-e read last, so that a
// quota check on the issuance path is a GetItem rather than a call across the
// network — and so that an issuance is still decidable while billing is
// unreachable. Every write to it comes from re-reading billing, never from a
// webhook body.
type AccountBillingRepository struct {
	Base
}

func NewAccountBillingRepository(db *dynamodb.Client, cfg *config.Config) *AccountBillingRepository {
	return &AccountBillingRepository{Base: NewBase(db, cfg, "account_billing")}
}

// AccountBillingPK is the snapshot's key. It is the same `USER_{sub}` shape a
// membership sort key uses, and the same string sent to billing as
// `external_ref` — one identifier for one account across three systems.
func AccountBillingPK(userID string) string {
	return BuildMemberSK(userID)
}

// BillingEventPK keys a processed webhook.
func BillingEventPK(eventID string) string {
	return "EVENT_" + eventID
}

// billingEventTTL is how long a processed event id is remembered.
//
// Seven days, against billing's own delivery policy: it retries an event for
// roughly two days and then gives up (MaxDeliveryAttempts). Remembering for
// longer than the sender will ever retry is what makes the deduplication
// complete rather than probabilistic, and the margin costs a few bytes per
// event.
const billingEventTTL = 7 * 24 * time.Hour

// OpenInvoice is the bill an account has waiting, copied from billing.
type OpenInvoice struct {
	ID          string `dynamodbav:"id" json:"id"`
	TotalCents  int64  `dynamodbav:"total_cents" json:"total_cents"`
	DueDate     string `dynamodbav:"due_date" json:"due_date"`
	CheckoutURL string `dynamodbav:"checkout_url,omitempty" json:"checkout_url,omitempty"`
}

// AccountSnapshot is everything the DF-e needs to know about an account's
// subscription without asking billing.
//
// It is deliberately flat and derived: `Quotas` and `Meters` are computed from
// the price metadata billing carries but does not read (ADR 0008), so the limit
// this service enforces and the limit the invoice was priced from come from the
// same place. Recomputing them on every read would mean re-deriving business
// rules from a catalogue on the issuance hot path.
type AccountSnapshot struct {
	UserID     string `dynamodbav:"user_id" json:"user_id"`
	CustomerID string `dynamodbav:"customer_id,omitempty" json:"customer_id,omitempty"`
	// SubscriptionID is empty for an account that has never chosen a plan, which
	// is an ordinary state and not an error.
	SubscriptionID string `dynamodbav:"subscription_id,omitempty" json:"subscription_id,omitempty"`
	// Status is billing's SubscriptionStatus verbatim — ACTIVE, INCOMPLETE,
	// PAST_DUE, PAUSED, CANCELED, TRIALING — or empty for no subscription. It is
	// not narrowed to a boolean here: the blocking middleware answers different
	// things for INCOMPLETE ("assine e pague") and PAST_DUE ("regularize"), and a
	// bool would have thrown that away at the moment it was cheapest to keep.
	Status string `dynamodbav:"status,omitempty" json:"status,omitempty"`
	// Plan is the plan key from the price metadata: free, pro, unlimited,
	// ondemand.
	Plan string `dynamodbav:"plan,omitempty" json:"plan,omitempty"`
	// Entitled is billing's own answer, kept as it was given. **It is not the
	// gate.** Billing counts PAST_DUE as entitled — the customer had service and
	// dunning has not given up — while the DF-e blocks it by decision (D2). The
	// two live side by side rather than one overwriting the other, so a
	// disagreement is visible instead of silently resolved.
	Entitled          bool `dynamodbav:"entitled" json:"entitled"`
	CancelAtPeriodEnd bool `dynamodbav:"cancel_at_period_end" json:"cancel_at_period_end"`

	PeriodStart string `dynamodbav:"period_start,omitempty" json:"period_start,omitempty"`
	PeriodEnd   string `dynamodbav:"period_end,omitempty" json:"period_end,omitempty"`

	// Quotas maps a meter name to its monthly limit; -1 is unlimited. Absent
	// means the plan grants none of that document type.
	Quotas map[string]int64 `dynamodbav:"quotas,omitempty" json:"quotas,omitempty"`
	// Meters maps a meter name to the billing price id that charges for it, and
	// is populated only on usage-based plans. Its presence is what tells the
	// worker to report an emission at all.
	Meters map[string]string `dynamodbav:"meters,omitempty" json:"meters,omitempty"`

	OpenInvoice *OpenInvoice `dynamodbav:"open_invoice,omitempty" json:"open_invoice,omitempty"`

	// NoCharge marks a snapshot produced with billing switched off. Everything
	// is granted, and the flag says why — so a screen can state that billing is
	// disabled instead of claiming the account is on an unlimited plan it never
	// bought.
	NoCharge bool `dynamodbav:"no_charge,omitempty" json:"no_charge,omitempty"`

	SyncedAt string `dynamodbav:"synced_at,omitempty" json:"synced_at,omitempty"`
}

// Get reads an account's snapshot. A nil snapshot with a nil error means the
// account has never been synced.
func (r *AccountBillingRepository) Get(ctx context.Context, userID string) (*AccountSnapshot, error) {
	item, err := r.GetItem(ctx, AccountBillingPK(userID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, nil
	}
	var out AccountSnapshot
	if err := attributevalue.UnmarshalMap(item, &out); err != nil {
		return nil, fmt.Errorf("decoding account billing snapshot: %w", err)
	}
	return &out, nil
}

// Put writes the snapshot, replacing whatever was there.
//
// A whole-item Put rather than a field-wise update, and that is the right shape:
// the snapshot is one consistent picture of a moment in billing, so merging a
// new subscription's fields over an old one's would produce a row describing
// neither — an old plan's quotas beside a new plan's id.
func (r *AccountBillingRepository) Put(ctx context.Context, s *AccountSnapshot) error {
	s.SyncedAt = NowStr()
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return fmt.Errorf("encoding account billing snapshot: %w", err)
	}
	item["pk"] = &types.AttributeValueMemberS{Value: AccountBillingPK(s.UserID)}
	return r.PutItem(ctx, item)
}

// MarkEventProcessed records a webhook event id, returning false if it was
// already there.
//
// The condition is what makes the webhook idempotent, and it is a conditional
// write rather than a read-then-write on purpose: billing delivers at least
// once, so two deliveries of one event can be in flight together, and a check
// followed by a write would let both through.
//
// It is called **before** the work rather than after. A delivery that fails
// midway is retried by billing and finds the marker, so the work is at-most-once
// — which is correct here because the work is "re-read billing and overwrite the
// snapshot", and skipping a redundant re-read costs nothing. The opposite order
// would make a crash between work and marker into a duplicate, which for a write
// that is not idempotent would matter.
func (r *AccountBillingRepository) MarkEventProcessed(ctx context.Context, eventID string) (bool, error) {
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: BillingEventPK(eventID)},
		"event_id":   &types.AttributeValueMemberS{Value: eventID},
		"created_at": &types.AttributeValueMemberS{Value: NowStr()},
		"ttl": &types.AttributeValueMemberN{
			Value: strconv.FormatInt(time.Now().Add(billingEventTTL).Unix(), 10),
		},
	}
	err := r.TransactWrite(ctx, []types.TransactWriteItem{r.BuildPutTxItemIfAbsent(item)})
	if IsConditionFailed(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Usage counters
// ---------------------------------------------------------------------------

// UsageCounterPK keys one account's counters for one billing period.
//
// The counters live in this table rather than one of their own. They share the
// account's key space and their access pattern is identical — one row, by
// primary key, never queried or scanned — so a second table would be a second
// thing to create, grant, prefix and remember, for no property gained. Different
// pk values sit in different partitions, so the counters and the snapshot do not
// contend.
//
// `period` is the subscription's own period start, not the calendar month: a
// plan anchored on the 10th resets on the 10th, and counting by calendar month
// would give that customer a short first month and a free stretch every time
// they changed plan.
func UsageCounterPK(userID, period string) string {
	return "USAGE_" + RawUserID(userID) + "#" + period
}

// usageCounterTTL keeps a closed period around long enough to answer "how much
// did we use last month" and then lets it go. Thirteen months, so a
// year-over-year comparison is still possible on the last day of the year.
const usageCounterTTL = 13 * 30 * 24 * time.Hour

// ErrQuotaExceeded reports a reservation refused because the meter is at its
// limit. It is a sentinel rather than a problem so the repository stays free of
// HTTP concerns; the service turns it into a 402 with the numbers attached.
var ErrQuotaExceeded = errors.New("quota exceeded")

// ReserveUsage claims one unit of a meter, atomically.
//
// **The reservation is the control.** It happens when the document is requested,
// not when SEFAZ authorises it, because a limit enforced on the authorised count
// is a limit two concurrent requests can both pass: each reads three of three
// used and each issues a fourth. `ADD` with a condition on the current value is
// what makes the check and the increment one operation.
//
// `limit` below zero means unlimited: the counter still moves — the usage screen
// needs the number, and a usage-based plan bills from it — but nothing is
// refused. A limit of exactly zero refuses everything, which is how the Free
// plan grants no CT-e.
//
// It returns the count **after** the reservation, so a caller can report "3 of 3
// used" without a second read.
func (r *AccountBillingRepository) ReserveUsage(ctx context.Context, userID, period, meter string, limit int64) (int64, error) {
	names := map[string]string{"#m": meter, "#ttl": "ttl"}
	values := map[string]types.AttributeValue{
		":one": &types.AttributeValueMemberN{Value: "1"},
		":ttl": &types.AttributeValueMemberN{
			Value: strconv.FormatInt(time.Now().Add(usageCounterTTL).Unix(), 10),
		},
	}
	// SET on the TTL rather than ADD, so a row that already exists keeps one
	// expiry instead of being pushed further out on every emission.
	update := "ADD #m :one SET #ttl = if_not_exists(#ttl, :ttl)"
	condition := ""
	switch {
	case limit > 0:
		// `attribute_not_exists` covers the first emission of the period, when
		// the counter has not been written yet — without it the very first
		// document on every plan would be refused.
		condition = "attribute_not_exists(#m) OR #m < :limit"
		values[":limit"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(limit, 10)}
	case limit == 0:
		// A limit of zero grants nothing, and the absent-attribute branch above
		// would grant exactly one — which is how the Free plan's `quota_cte: 0`
		// would have become "one free CT-e". `#m < 0` is false for a missing
		// attribute, so this refuses from the first attempt.
		//
		// The branch is in Go rather than in the expression because DynamoDB will
		// not compare two literals: `:zero < :limit` needs at least one operand to
		// be a document path.
		condition = "#m < :limit"
		values[":limit"] = &types.AttributeValueMemberN{Value: "0"}
	}

	out, err := r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: UsageCounterPK(userID, period)},
		},
		UpdateExpression:          aws.String(update),
		ConditionExpression:       conditionOrNil(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if IsConditionFailed(err) {
		return limit, ErrQuotaExceeded
	}
	if err != nil {
		return 0, wrapDynamoErr(err)
	}
	return counterValue(out.Attributes, meter), nil
}

// RefundUsage gives one unit back, for a document that never reached SEFAZ.
//
// Conditional on the counter being above zero: a refund that could drive it
// negative would hand out free headroom every time a refund was replayed, and a
// retried worker message is exactly that.
//
// A failed condition is not an error. It means there was nothing to give back —
// the counter was already at zero — and the caller has nothing to do about it.
func (r *AccountBillingRepository) RefundUsage(ctx context.Context, userID, period, meter string) error {
	_, err := r.UpdateItemRaw(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.TableName),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: UsageCounterPK(userID, period)},
		},
		UpdateExpression:         aws.String("ADD #m :minusOne"),
		ConditionExpression:      aws.String("#m > :zero"),
		ExpressionAttributeNames: map[string]string{"#m": meter},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":minusOne": &types.AttributeValueMemberN{Value: "-1"},
			":zero":     &types.AttributeValueMemberN{Value: "0"},
		},
	})
	if IsConditionFailed(err) {
		return nil
	}
	return wrapDynamoErr(err)
}

// GetUsage reads every counter for one account and period. An absent row is no
// usage, not an error.
func (r *AccountBillingRepository) GetUsage(ctx context.Context, userID, period string) (map[string]int64, error) {
	item, err := r.GetItem(ctx, UsageCounterPK(userID, period))
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for k, v := range item {
		// `pk` and `ttl` are bookkeeping, not meters. Everything else in the row
		// is a counter by construction — nothing but ReserveUsage writes here.
		if k == "pk" || k == "ttl" {
			continue
		}
		if n, ok := v.(*types.AttributeValueMemberN); ok {
			if parsed, err := strconv.ParseInt(n.Value, 10, 64); err == nil {
				out[k] = parsed
			}
		}
	}
	return out, nil
}

func conditionOrNil(expr string) *string {
	if expr == "" {
		return nil
	}
	return aws.String(expr)
}

func counterValue(attrs map[string]types.AttributeValue, meter string) int64 {
	n, ok := attrs[meter].(*types.AttributeValueMemberN)
	if !ok {
		return 0
	}
	v, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil {
		slog.Warn("account billing numeric attribute parse failed", "err", err)
	}
	return v
}
