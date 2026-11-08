package service

// breaker.go: the circuit breaker of the automatic contingency, worker side (docs/specs/2026-11-08-contingencia.md,
// 4.1). The state is one JSON attribute on the fiscal configuration row of the document type, written only here
// with a compare-and-swap on the previous value; the API reads it in the pre-check. Fed by (a) real
// authorizations answered through the egress (observeAuth) and (b) the scheduled probe (probe.go).
// Everything is gated by autocontingency.SwitchEnabled (false): with it off nothing here reads or writes.

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
	"gopkg.aoctech.app/dfe/go-dfe/autocontingency"
)

// Attribute names of the configuration row (mirror of api/internal/repositories/fiscal_config.go; another module).
const (
	attrAutoContingency  = "auto_contingency"
	attrAutoBreaker      = "auto_breaker"
	attrContingencyAuto  = "contingency_auto"
	attrEmissionType     = "emission_type"
	attrContingencyStart = "contingency_since"
	attrContingencyWhy   = "contingency_reason"
	attrConfigEnv        = "environment"
	emissionNormalValue  = "normal"

	accessKeyLen       = 44
	accessKeyTpEmisPos = 34 // tpEmis is digit 35 of the key
	tpEmisNormalDigit  = '1'

	alertJobBreaker = "auto-contingency"
)

// switchEnabled is autocontingency.SwitchEnabled; a variable only so tests can turn it on.
var switchEnabled = autocontingency.SwitchEnabled

// breakerConfigTable is the fiscal configuration table (without the prefix) of each document type.
var breakerConfigTable = map[string]string{
	docTypeNFe:  "organization_nfe_configs",
	docTypeNFCe: "organization_nfce_configs",
	docTypeCTe:  "organization_cte_configs",
	docTypeMDFe: "organization_mdfe_configs",
}

// WithAlerter sets the operator alert publisher (go-common alerts) for the breaker alerts.
func (s *DfeService) WithAlerter(a alerts.Publisher) *DfeService {
	if a != nil {
		s.alerter = a
	}
	return s
}

func (s *DfeService) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

// observeAuth feeds the breaker with the answer of a real authorization. Only a first emission in normal form
// counts (tpEmis 1 in the key, no SVC route): documents emitted in contingency say nothing about the normal
// authorizer. Events, other services and other document types are ignored. Never fails the document.
func (s *DfeService) observeAuth(ctx context.Context, msg WorkerMessage, o autocontingency.Observation) {
	table, ok := breakerConfigTable[msg.DocType]
	if !switchEnabled || !ok || msg.EventsTableName != nil || msg.Contingency != "" ||
		len(msg.AccessKey) != accessKeyLen || msg.AccessKey[accessKeyTpEmisPos] != tpEmisNormalDigit {
		return
	}
	if _, _, err := s.applyObservation(ctx, table, companyOfDocPK(msg.DocPK), msg.DocType, false, msg.SefazEnvironment, o); err != nil {
		slog.WarnContext(ctx, "breaker feed failed", "doc_type", msg.DocType, "err", err)
	}
}

// applyObservation loads the row, feeds the breaker one observation and stores the result. rowEnv is the
// sefaz environment the observation came from; it is ignored when empty and otherwise must match the row's
// current environment (a homologation answer must not move the production breaker). A transition to Closing
// returns the row to normal in the same write when the automatic switch put it in contingency.
// Two attempts: a lost compare-and-swap rereads once; after that the observation is dropped (a later one
// carries the same information).
func (s *DfeService) applyObservation(ctx context.Context, tableSuffix, orgPK, docType string, probe bool, rowEnv string, o autocontingency.Observation) (autocontingency.Breaker, autocontingency.Transition, error) {
	table := s.cfg.TablePrefix + "_" + tableSuffix
	key := map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}}
	var b autocontingency.Breaker
	tr := autocontingency.NoChange
	for range 2 {
		out, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(table), Key: key, ConsistentRead: aws.Bool(true)})
		if err != nil {
			return b, tr, fmt.Errorf("read config: %w", err)
		}
		item := out.Item
		if item == nil || !boolAttr(item, attrAutoContingency) {
			return autocontingency.Breaker{State: autocontingency.Closed}, tr, nil // not tracked: the flag is off
		}
		if rowEnv != "" && rowEnv != sefazEnvOf(attrN(item, attrConfigEnv, 2)) {
			return autocontingency.Decode(attrS(item, attrAutoBreaker)), tr, nil
		}
		old := attrS(item, attrAutoBreaker)
		b = autocontingency.Decode(old)
		tr = b.Observe(s.now(), autocontingency.Classify(o), probe)
		raw := autocontingency.Encode(b)
		if raw == old && tr == autocontingency.NoChange {
			return b, tr, nil
		}
		returning := tr == autocontingency.Closing &&
			autocontingency.ShouldReturn(autocontingency.Config{Enabled: switchEnabled, Auto: true, Current: attrEmissionOf(item)}, b, boolAttr(item, attrContingencyAuto))
		if err := s.casBreaker(ctx, table, key, old, raw, returning); err != nil {
			if isConditionFailed(err) {
				continue
			}
			return b, autocontingency.NoChange, err
		}
		s.alertTransition(ctx, tr, docType, orgPK, returning)
		return b, tr, nil
	}
	return b, autocontingency.NoChange, nil
}

func (s *DfeService) casBreaker(ctx context.Context, table string, key map[string]types.AttributeValue, old, raw string, returning bool) error {
	names := map[string]string{"#b": attrAutoBreaker}
	values := map[string]types.AttributeValue{}
	var set, remove []string
	cond := []string{"attribute_exists(pk)", "#a = :true"}
	names["#a"] = attrAutoContingency
	values[":true"] = &types.AttributeValueMemberBOOL{Value: true}
	if old == "" {
		cond = append(cond, "attribute_not_exists(#b)")
	} else {
		cond = append(cond, "#b = :old")
		values[":old"] = &types.AttributeValueMemberS{Value: old}
	}
	if raw == "" {
		remove = append(remove, "#b")
	} else {
		set = append(set, "#b = :raw")
		values[":raw"] = &types.AttributeValueMemberS{Value: raw}
	}
	if returning { // back to normal in the same write: the breaker can never close with the row left behind
		names["#t"], names["#s"], names["#r"], names["#ca"] = attrEmissionType, attrContingencyStart, attrContingencyWhy, attrContingencyAuto
		set = append(set, "#t = :normal")
		values[":normal"] = &types.AttributeValueMemberS{Value: emissionNormalValue}
		remove = append(remove, "#s", "#r", "#ca")
		cond = append(cond, "#ca = :true")
	}
	expr := ""
	if len(set) > 0 {
		expr += "SET " + strings.Join(set, ", ")
	}
	if len(remove) > 0 {
		expr += " REMOVE " + strings.Join(remove, ", ")
	}
	_, err := s.dynamo.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(table), Key: key, UpdateExpression: aws.String(strings.TrimSpace(expr)),
		ConditionExpression: aws.String(strings.Join(cond, " AND ")), ExpressionAttributeNames: names, ExpressionAttributeValues: values,
	})
	return err
}

// alertTransition tells the operator (go-common alerts only, no CloudWatch) when a breaker opens or closes.
// Ids only: no document number, no CNPJ.
func (s *DfeService) alertTransition(ctx context.Context, tr autocontingency.Transition, docType, orgPK string, returned bool) {
	switch tr {
	case autocontingency.Opened:
		s.alerter.Alert(ctx, alerts.Alert{Job: alertJobBreaker,
			Summary: "The normal authorizer looks unavailable: the circuit breaker of a document type opened; new emissions of companies with automatic contingency go to the chosen contingency.",
			Detail:  fmt.Sprintf("doc_type: %s\norg: %s", docType, orgPK)})
	case autocontingency.Closing:
		s.alerter.Alert(ctx, alerts.Alert{Job: alertJobBreaker,
			Summary: "The normal authorizer recovered: the circuit breaker closed.",
			Detail:  fmt.Sprintf("doc_type: %s\norg: %s\nreturned_to_normal: %t", docType, orgPK, returned)})
	}
}

func boolAttr(item map[string]types.AttributeValue, key string) bool {
	v, ok := item[key].(*types.AttributeValueMemberBOOL)
	return ok && v.Value
}

func attrEmissionOf(item map[string]types.AttributeValue) string {
	if v := attrS(item, attrEmissionType); v != "" {
		return v
	}
	return emissionNormalValue
}

func sefazEnvOf(environment int) string {
	if environment == 1 {
		return sefazEnvProd
	}
	return sefazEnvHom
}

func isConditionFailed(err error) bool {
	_, ok := errors.AsType[*types.ConditionalCheckFailedException](err)
	return ok
}
