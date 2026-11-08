// Package ciotalert holds the proactive CIOT alerts (docs/specs/2026-11-04-ciot-antt-pef-integracao.md, C4):
// a certificate about to expire for an organization with open CIOTs, and the 24 h cancellation deadline (B35)
// about to pass. It runs inside the contingency dispatcher's 15-minute schedule (no schedule or Lambda of its own),
// reads the sparse alert-index of ciots (no Scan) and delivers only through go-common alerts. Alert text carries
// ids and dates only: no document numbers, no certificate content.
package ciotalert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/alerts"
)

const (
	// IndexName is the sparse GSI of ciots: only open CIOTs (declared, rectified) carry SweepAttr, sorted by
	// AtAttr (the cancellation deadline, RFC3339 UTC). The worker stamps both when a declaration succeeds and
	// removes both when the CIOT is cancelled or closed.
	IndexName  = "alert-index"
	SweepAttr  = "alert_sweep"
	SweepValue = "1"
	AtAttr     = "alert_at"
	// AlertedAttr records that the cancellation alert was sent (once per CIOT).
	AlertedAttr = "cancel_alerted"
	// TimeLayout is fixed width so the sort key orders chronologically.
	TimeLayout = "2006-01-02T15:04:05Z"

	// CancelWarn is how long before the deadline the cancellation alert fires.
	CancelWarn = 2 * time.Hour

	tableCiots   = "ciots"
	tableCerts   = "organization_certificates"
	markerPK     = "CERT_ALERT#"
	markerTTL    = 30 * 24 * time.Hour
	statusDecl   = "declared"
	statusRectif = "rectified"
	attrPending  = "pending_operation"
)

// CertThresholdsDays are the warning points; each fires once per certificate expiry. 0 means already expired.
var CertThresholdsDays = []int{15, 7, 3, 1, 0}

// DB is the DynamoDB subset used.
type DB interface {
	Query(ctx context.Context, in *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

type open struct {
	pk, sk string
	at     time.Time
}

// Sweep runs both alerts once. Errors are logged; the next 15-minute run redoes the work.
func Sweep(ctx context.Context, db DB, alerter alerts.Publisher, prefix string, now time.Time) {
	rows, err := openCiots(ctx, db, prefix)
	if err != nil {
		slog.Error("failed to list ciot alert index", "err", err)
		return
	}
	orgs := map[string]string{} // org_pk -> env
	for _, r := range rows {
		env, org, ok := strings.Cut(r.pk, "#")
		if !ok {
			continue
		}
		orgs[org] = env
		if !r.at.Before(now) && r.at.Sub(now) <= CancelWarn {
			cancelAlert(ctx, db, alerter, prefix, r, env, org)
		}
	}
	for org := range orgs {
		certAlert(ctx, db, alerter, prefix, org, now)
	}
}

func openCiots(ctx context.Context, db DB, prefix string) ([]open, error) {
	in := &dynamodb.QueryInput{
		TableName:                 aws.String(prefix + "_" + tableCiots),
		IndexName:                 aws.String(IndexName),
		KeyConditionExpression:    aws.String("#s = :v"),
		ExpressionAttributeNames:  map[string]string{"#s": SweepAttr},
		ExpressionAttributeValues: map[string]types.AttributeValue{":v": &types.AttributeValueMemberS{Value: SweepValue}},
	}
	var rows []open
	for {
		out, err := db.Query(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, it := range out.Items {
			r := open{pk: str(it, "pk"), sk: str(it, "sk")}
			r.at, _ = time.Parse(TimeLayout, str(it, AtAttr))
			if r.pk != "" && r.sk != "" {
				rows = append(rows, r)
			}
		}
		if out.LastEvaluatedKey == nil {
			return rows, nil
		}
		in.ExclusiveStartKey = out.LastEvaluatedKey
	}
}

func cancelAlert(ctx context.Context, db DB, a alerts.Publisher, prefix string, r open, env, org string) {
	_, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(prefix + "_" + tableCiots),
		Key:              map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: r.pk}, "sk": &types.AttributeValueMemberS{Value: r.sk}},
		UpdateExpression: aws.String("SET " + AlertedAttr + " = :t"),
		// an operation in flight (maybe the cancel itself) or a resolved row is not worth an alert
		ConditionExpression:      aws.String("attribute_exists(" + SweepAttr + ") AND attribute_not_exists(" + AlertedAttr + ") AND attribute_not_exists(" + attrPending + ") AND #st IN (:d, :r)"),
		ExpressionAttributeNames: map[string]string{"#st": "status"},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":t": &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":d": &types.AttributeValueMemberS{Value: statusDecl}, ":r": &types.AttributeValueMemberS{Value: statusRectif},
		},
	})
	if _, ok := errorsAsCondition(err); ok {
		return
	}
	if err != nil {
		slog.Error("failed to mark ciot cancel alert", "sk", r.sk, "err", err)
		return
	}
	a.Alert(ctx, alerts.Alert{Job: "ciot-cancel-deadline",
		Summary: "A declared CIOT is about to pass the 24 h cancellation deadline (B35).",
		Detail:  fmt.Sprintf("env: %s\norg: %s\nciot row: %s\ndeadline (UTC): %s", env, org, r.sk, r.at.Format(TimeLayout))})
}

func certAlert(ctx context.Context, db DB, a alerts.Publisher, prefix, org string, now time.Time) {
	out, err := db.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(prefix + "_" + tableCerts),
		KeyConditionExpression:    aws.String("pk = :pk"),
		ProjectionExpression:      aws.String("expires_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: org}},
	})
	if err != nil {
		slog.Error("failed to read certificates for ciot alert", "org", org, "err", err)
		return
	}
	var best time.Time // the latest expiry: a renewed certificate silences the old one
	var bestRaw string
	for _, it := range out.Items {
		if t, err := time.Parse(time.RFC3339, str(it, "expires_at")); err == nil && t.After(best) {
			best, bestRaw = t, str(it, "expires_at")
		}
	}
	if best.IsZero() {
		return
	}
	days := int(best.Sub(now).Hours() / 24)
	if best.Before(now) {
		days = -1
	}
	th := -1
	for _, t := range CertThresholdsDays { // tightest threshold already crossed
		if days < t {
			th = t
		}
	}
	if th < 0 {
		return
	}
	_, err = db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(prefix + "_" + tableCiots),
		Item: map[string]types.AttributeValue{
			"pk":         &types.AttributeValueMemberS{Value: markerPK + org},
			"sk":         &types.AttributeValueMemberS{Value: fmt.Sprintf("%s#%d", bestRaw, th)},
			"expires_at": &types.AttributeValueMemberN{Value: fmt.Sprint(best.Add(markerTTL).Unix())},
		},
		ConditionExpression: aws.String("attribute_not_exists(pk)"),
	})
	if _, ok := errorsAsCondition(err); ok {
		return
	}
	if err != nil {
		slog.Error("failed to record certificate alert", "org", org, "err", err)
		return
	}
	a.Alert(ctx, alerts.Alert{Job: "ciot-certificate-expiry",
		Summary: "The certificate of an organization with open CIOTs is expiring or expired; CIOT operations will be refused.",
		Detail:  fmt.Sprintf("org: %s\nexpires (UTC): %s\nthreshold (days): %d", org, best.UTC().Format(TimeLayout), th)})
}

func str(it map[string]types.AttributeValue, k string) string {
	if v, ok := it[k].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func errorsAsCondition(err error) (*types.ConditionalCheckFailedException, bool) {
	return errors.AsType[*types.ConditionalCheckFailedException](err)
}
