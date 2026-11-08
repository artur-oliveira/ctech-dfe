package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/worker/internal/contingency"
)

var epecMsg = func() WorkerMessage {
	m := baseMsg
	m.UF, m.SefazService = "AN", "RecepcaoEvento"
	m.EventsTableName, m.EventType, m.SequenceNumber, m.EventSK = new("nfe_events"), new(epecEvent), new(1), new("evt-epec-1")
	m.BillingOrganizationID, m.BillingMeter = "org-1", "nfe"
	return m
}()

func armUpdates(u []capturedUpdate) (arm, other []capturedUpdate) {
	for _, c := range u {
		if _, ok := c.values[":awaiting"]; ok {
			arm = append(arm, c)
		} else {
			other = append(other, c)
		}
	}
	return
}

// The event registered (136 = registrado e não vinculado, the EPEC success; 573 = already registered)
// arms the sweep; the arm is conditional on the stored message and on the awaiting state.
func TestProcess_EPECEventRegistered_ArmsSweep(t *testing.T) {
	for _, cStat := range []string{"136", "573"} {
		dynm := &mockDynamo{}
		svc := newSvc(certS3(), &mockLambda{payload: invokeResp(cStat, "ok", "")}, dynm)
		if err := svc.Process(context.Background(), epecMsg); err != nil {
			t.Fatalf("%s: %v", cStat, err)
		}
		arm, other := armUpdates(dynm.updates)
		if len(arm) != 1 || len(other) != 1 || other[0].status != EventStatusSuccess {
			t.Fatalf("%s: want one arm and one event success, got %+v", cStat, dynm.updates)
		}
		v := arm[0].values
		if v[":pending"].(*types.AttributeValueMemberS).Value != contingency.StatePending ||
			v[":one"].(*types.AttributeValueMemberS).Value != contingency.SweepValue {
			t.Errorf("%s: arm values %+v", cStat, v)
		}
		dl, err := time.Parse(time.RFC3339, v[":dl"].(*types.AttributeValueMemberS).Value)
		if err != nil || time.Until(dl) < 167*time.Hour || time.Until(dl) > 169*time.Hour {
			t.Errorf("%s: deadline %v (%v)", cStat, dl, err)
		}
		if !strings.Contains(arm[0].condition, "attribute_exists(#msg)") || !strings.Contains(arm[0].condition, ":awaiting") {
			t.Errorf("%s: arm must be conditional: %q", cStat, arm[0].condition)
		}
	}
}

// A rejected EPEC (142 environment blocked, 121 UF refused, ...) never arms: the stored message is removed
// and the document ends rejected, with the SNS result carrying the billing fields for the quota release.
func TestProcess_EPECEventRejected_NeverArms(t *testing.T) {
	dynm := &mockDynamo{}
	snsm := &mockSNS{}
	cfg := *testCfg
	cfg.ResultsTopicARN = "arn:aws:sns:sa-east-1:1:results"
	svc := New(Clients{S3: certS3(), Lambda: &mockLambda{payload: invokeResp("142", "EPEC bloqueado", "")}, Dynamo: dynm, SNS: snsm}, &cfg)
	if err := svc.Process(context.Background(), epecMsg); err != nil {
		t.Fatal(err)
	}
	if arm, _ := armUpdates(dynm.updates); len(arm) != 0 {
		t.Fatalf("a rejected EPEC must not arm: %+v", arm)
	}
	var removed, rejected bool
	for _, u := range dynm.updates {
		removed = removed || u.status == "" && u.table == "dev_nfes" // REMOVE contingency attrs
		rejected = rejected || u.table == "dev_nfes" && u.status == StatusRejected
	}
	if !removed || !rejected {
		t.Fatalf("want contingency attrs removed and document rejected: %+v", dynm.updates)
	}
	if len(snsm.calls) < 1 || !strings.Contains(snsm.calls[0], `"billing_organization_id":"org-1"`) {
		t.Fatalf("document result must carry billing fields: %v", snsm.calls)
	}
}

// A terminal engine failure on the EPEC event also leaves the document unarmed and rejected.
func TestProcess_EPECEventEngineFailure_NeverArms(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeRespStatus(422, "payload invalido")}, dynm)
	if err := svc.Process(context.Background(), epecMsg); err != nil {
		t.Fatal(err)
	}
	if arm, _ := armUpdates(dynm.updates); len(arm) != 0 {
		t.Fatalf("must not arm: %+v", arm)
	}
}

// Other NF-e events never touch the sweep.
func TestProcess_OtherEventDoesNotArm(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "ok", "")}, dynm)
	m := epecMsg
	m.EventType = new("110110")
	if err := svc.Process(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if arm, _ := armUpdates(dynm.updates); len(arm) != 0 || len(dynm.updates) != 1 {
		t.Fatalf("got %+v", dynm.updates)
	}
}
