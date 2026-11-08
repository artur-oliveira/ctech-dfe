package ciotalert

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/alerts"
)

type fakeDB struct {
	rows    []map[string]types.AttributeValue
	certs   map[string]string // org -> expires_at
	markers map[string]bool
	marked  []string // cancel_alerted updates
	alerted map[string]bool
	indexes []string
}

func s(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

func (f *fakeDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if in.IndexName != nil {
		f.indexes = append(f.indexes, *in.IndexName)
		return &dynamodb.QueryOutput{Items: f.rows}, nil
	}
	org := in.ExpressionAttributeValues[":pk"].(*types.AttributeValueMemberS).Value
	if exp, ok := f.certs[org]; ok {
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{{"expires_at": s(exp)}}}, nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (f *fakeDB) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	k := in.Item["pk"].(*types.AttributeValueMemberS).Value + "|" + in.Item["sk"].(*types.AttributeValueMemberS).Value
	if f.markers[k] {
		return nil, &types.ConditionalCheckFailedException{}
	}
	f.markers[k] = true
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	sk := in.Key["sk"].(*types.AttributeValueMemberS).Value
	if f.alerted[sk] {
		return nil, &types.ConditionalCheckFailedException{}
	}
	f.alerted[sk] = true
	f.marked = append(f.marked, aws.ToString(in.UpdateExpression))
	return &dynamodb.UpdateItemOutput{}, nil
}

type capture struct{ got []alerts.Alert }

func (c *capture) Alert(_ context.Context, a alerts.Alert) { c.got = append(c.got, a) }

var now = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func row(sk string, at time.Time) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": s("prod#ORG1"), "sk": s(sk), AtAttr: s(at.Format(TimeLayout))}
}

func newDB(rows ...map[string]types.AttributeValue) *fakeDB {
	return &fakeDB{rows: rows, certs: map[string]string{}, markers: map[string]bool{}, alerted: map[string]bool{}}
}

func TestCancelDeadlineAlertsOncePerCiot(t *testing.T) {
	db := newDB(row("CIOT_soon", now.Add(time.Hour)), row("CIOT_far", now.Add(10*time.Hour)), row("CIOT_past", now.Add(-time.Hour)))
	al := &capture{}
	Sweep(t.Context(), db, al, "t", now)
	Sweep(t.Context(), db, al, "t", now.Add(15*time.Minute)) // next run: deduped by the conditional update
	if len(al.got) != 1 || al.got[0].Job != "ciot-cancel-deadline" || !strings.Contains(al.got[0].Detail, "CIOT_soon") {
		t.Fatalf("%+v", al.got)
	}
	if db.indexes[0] != "alert-index" {
		t.Fatal("must read the sparse index")
	}
}

func TestCertificateExpiryAlertsOncePerThreshold(t *testing.T) {
	db := newDB(row("CIOT_1", now.Add(100*time.Hour)))
	db.certs["ORG1"] = now.Add(5*24*time.Hour + time.Hour).Format(time.RFC3339) // 5 days: threshold 7
	al := &capture{}
	Sweep(t.Context(), db, al, "t", now)
	Sweep(t.Context(), db, al, "t", now.Add(15*time.Minute))
	if len(al.got) != 1 || al.got[0].Job != "ciot-certificate-expiry" || !strings.Contains(al.got[0].Detail, "threshold (days): 7") {
		t.Fatalf("%+v", al.got)
	}
	Sweep(t.Context(), db, al, "t", now.Add(2*24*time.Hour+12*time.Hour)) // under 3 days left: next threshold
	if len(al.got) != 2 || !strings.Contains(al.got[1].Detail, "threshold (days): 3") {
		t.Fatalf("%+v", al.got)
	}
	for _, a := range al.got {
		if strings.Contains(a.Detail+a.Summary, "pfx") || strings.Contains(a.Detail, "password") {
			t.Fatal("no certificate content in alerts")
		}
	}
}

func TestCertificateFarFromExpiryAndNoCertificateStaySilent(t *testing.T) {
	db := newDB(row("CIOT_1", now.Add(100*time.Hour)))
	al := &capture{}
	Sweep(t.Context(), db, al, "t", now) // no certificate rows
	db.certs["ORG1"] = now.Add(40 * 24 * time.Hour).Format(time.RFC3339)
	Sweep(t.Context(), db, al, "t", now)
	if len(al.got) != 0 {
		t.Fatalf("%+v", al.got)
	}
	db.certs["ORG1"] = now.Add(-time.Hour).Format(time.RFC3339)
	Sweep(t.Context(), db, al, "t", now)
	if len(al.got) != 1 || !strings.Contains(al.got[0].Detail, "threshold (days): 0") {
		t.Fatalf("expired: %+v", al.got)
	}
}
