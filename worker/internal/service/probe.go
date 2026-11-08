package service

// probe.go: the health probe of the automatic contingency (docs/specs/2026-11-08-contingencia.md, 4.1). The
// contingency dispatcher (every 15 min) invokes this worker once per row of the sparse auto-contingency-index; the
// worker calls the status service of the normal authorizer through the egress (cheap: one small SOAP call, no
// document) and feeds the breaker. A schedule of one probe per minute would cost a Lambda per company and
// document per minute, so one run makes a short burst instead (up to RecoveryProbes calls, ProbeGap apart): enough
// to reach FailureThreshold inside FailureWindow, or RecoveryProbes healthy answers after CoolDown.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/go-dfe/autocontingency"
)

// statusSpec is the unsigned status call of each document type (egress service, root tag, version, namespace).
type statusSpec struct {
	service, rootTag, versao, ns string
	withUF                       bool
}

var statusSpecs = map[string]statusSpec{
	docTypeNFe:  {"NfeStatusServico", "consStatServ", "4.00", nsNFe, true},
	docTypeNFCe: {"NfeStatusServico", "consStatServ", "4.00", nsNFe, true},
	docTypeCTe:  {"CTeStatusServico", "consStatServCTe", "4.00", nsCTe, true},
	docTypeMDFe: {"MDFeStatusServico", "consStatServMDFe", "3.00", nsMDFe, false},
}

// docTypeOfConfigTable is the inverse of breakerConfigTable.
func docTypeOfConfigTable(suffix string) string {
	for dt, t := range breakerConfigTable {
		if t == suffix {
			return dt
		}
	}
	return ""
}

// ProbeJob is the dispatcher payload of one probe.
type ProbeJob struct {
	Table string // configuration table without the prefix, as listed by the auto-contingency-index
	PK    string
}

// Probe runs one burst for one configuration row. It never returns an error for an unreachable authorizer (that is
// the signal, not a failure); only a broken read of the row or certificate is an error.
func (s *DfeService) Probe(ctx context.Context, job ProbeJob) error {
	if !switchEnabled {
		return nil
	}
	docType := docTypeOfConfigTable(job.Table)
	spec, ok := statusSpecs[docType]
	if !ok {
		return fmt.Errorf("probe: unknown configuration table %q", job.Table)
	}
	table := s.cfg.TablePrefix + "_" + job.Table
	cfgOut, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(table), Key: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: job.PK}},
	})
	if err != nil {
		return fmt.Errorf("probe: read config: %w", err)
	}
	cfg := cfgOut.Item
	if cfg == nil || !boolAttr(cfg, attrAutoContingency) {
		return nil // stale index entry
	}
	b := autocontingency.Decode(attrS(cfg, attrAutoBreaker))
	if b.State == autocontingency.Open && s.now().Sub(b.OpenedAt) < autocontingency.CoolDown {
		return nil // inside the cool-down no probe can count; do not spend an egress call
	}
	payload, err := s.statusPayload(ctx, job.PK, spec, docType, cfg)
	if err != nil {
		return err
	}
	rowEnv := payload.Environment
	for i := range autocontingency.RecoveryProbes {
		if i > 0 {
			s.sleep(ctx, autocontingency.ProbeGap)
		}
		o := s.statusObservation(ctx, payload)
		verdict := autocontingency.Classify(o)
		after, tr, err := s.applyObservation(ctx, job.Table, job.PK, docType, true, rowEnv, o)
		if err != nil {
			return err
		}
		// A transition ends the burst. So does one healthy answer of a closed breaker (nothing to prove), an
		// unavailable one of an open breaker (the streak was reset) and an answer that says nothing.
		if tr != autocontingency.NoChange || verdict == autocontingency.Neutral ||
			(verdict == autocontingency.Healthy && after.State == autocontingency.Closed) ||
			(verdict == autocontingency.Unavailable && after.State == autocontingency.Open) {
			return nil
		}
	}
	return nil
}

func (s *DfeService) sleep(ctx context.Context, d time.Duration) {
	if s.pause != nil {
		s.pause(ctx, d)
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// statusPayload builds the egress request: certificate and identity of the company, the status body of the type.
func (s *DfeService) statusPayload(ctx context.Context, orgPK string, spec statusSpec, docType string, cfg map[string]types.AttributeValue) (lambdaPayload, error) {
	certOut, err := s.dynamo.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(s.cfg.TablePrefix + "_organization_certificates"),
		KeyConditionExpression: aws.String("pk = :pk"), Limit: aws.Int32(1), ScanIndexForward: aws.Bool(false),
		ExpressionAttributeValues: map[string]types.AttributeValue{":pk": &types.AttributeValueMemberS{Value: orgPK}},
	})
	if err != nil {
		return lambdaPayload{}, fmt.Errorf("probe: read certificate: %w", err)
	}
	if len(certOut.Items) == 0 {
		return lambdaPayload{}, errors.New("probe: company has no certificate")
	}
	orgOut, err := s.dynamo.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.cfg.TablePrefix + "_organizations"),
		Key:       map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: orgPK}},
	})
	if err != nil {
		return lambdaPayload{}, fmt.Errorf("probe: read organization: %w", err)
	}
	cnpj := organizationTaxID(orgOut.Item, orgPK)
	if cnpj == "" {
		return lambdaPayload{}, errors.New("probe: organization tax id not found")
	}
	certB64, err := s.getCertB64(ctx, attrS(certOut.Items[0], "s3_key"))
	if err != nil {
		return lambdaPayload{}, fmt.Errorf("probe: certificate: %w", err)
	}
	uf := extractUF(orgOut.Item)
	environment := attrN(cfg, attrConfigEnv, 2)
	tpAmb := "2"
	if environment == 1 {
		tpAmb = "1"
	}
	body := map[string]any{"@versao": spec.versao, "@xmlns": spec.ns, "tpAmb": tpAmb, "xServ": "STATUS"}
	if spec.withUF {
		body["cUF"] = ufCode(uf)
	}
	return lambdaPayload{
		CNPJ: cnpj, CertificateB64: certB64, CertificatePassword: attrS(certOut.Items[0], "password"),
		UF: uf, Environment: sefazEnvOf(environment), DocType: docType, Service: spec.service,
		Body: map[string]any{spec.rootTag: body},
	}, nil
}

// statusObservation calls the status service and reads what it says about availability. A 4xx of the egress (a
// certificate or request problem on our side) is neutral: it says nothing about the authorizer.
func (s *DfeService) statusObservation(ctx context.Context, p lambdaPayload) autocontingency.Observation {
	resp, err := s.invokeEgress(ctx, p)
	if err != nil {
		return autocontingency.Observation{TransportErr: true}
	}
	if resp.StatusCode != 200 {
		return autocontingency.Observation{HTTPStatus: resp.StatusCode}
	}
	var body map[string]any
	if json.Unmarshal([]byte(resp.Body), &body) != nil {
		slog.Warn("probe: unreadable status response")
		return autocontingency.Observation{}
	}
	return autocontingency.Observation{CStat: strVal(findValue(body, "cStat"))}
}
