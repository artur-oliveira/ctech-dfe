package services

// contingency_offline.go: what every document type needs to issue offline (NFC-e tpEmis 9, MDF-e and
// CT-e tpEmis 2; docs/specs/2026-11-08-contingencia.md): the two local egress operations that need the
// certificate (SignText for the QR Code signature, SignOnly for the signed XML) and the pending
// record the retransmission worker sweeps. Per-document code only builds the document.

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

const (
	// Service names and body keys of the go-dfe local operations (go-dfe/internal/constants).
	EgressServiceSignText = "SignText"
	EgressServiceSignOnly = "SignOnly"
	EgressBodyText        = "text"
	EgressBodySignature   = "signature"
	EgressBodyXML         = "xml"
	// EgressBodySignedXML in an authorization Body means: send this already signed document as is.
	EgressBodySignedXML = "@signed_xml"

	// Mirror of worker/internal/contingency (the worker is another module). The sparse
	// contingency-index holds only items with AttrContingencySweep.
	AttrContingencySweep    = "contingency_sweep"
	AttrContingencyState    = "contingency_state"
	AttrContingencyDeadline = "contingency_deadline"
	AttrContingencyMsg      = "contingency_msg"
	ContingencySweepValue   = "1"
	ContingencyPending      = "pending"

	// OfflineWindowNFCe is the retransmission window of an offline NFC-e (owner decision 4; varies
	// by UF, hence a constant). MDF-e and CT-e use the MOC value: 168 hours counted from the offline
	// emission (MDF-e MOC Contingencia 3; CT-e MOC Contingencia 4.1.1 and rule G103).
	OfflineWindowNFCe = 3 * 24 * time.Hour
	OfflineWindowMDFe = 168 * time.Hour
	OfflineWindowCTe  = 168 * time.Hour
)

// EgressCaller is what the offline flows use of ExternalService: the certificate and the
// go-dfe-egress call. Tests fake the egress through it.
type EgressCaller interface {
	CertificateB64(ctx context.Context, orgPK string) (string, string, error)
	CallDfe(ctx context.Context, req godfe.Request) (godfe.Response, error)
}

// SignTextViaEgress asks the egress to sign text with the organization's certificate (RSA SHA-1, Base64).
func SignTextViaEgress(ctx context.Context, e EgressCaller, orgPK, issuerDoc, docType, text string) (string, error) {
	if e == nil {
		return "", problem.InternalServer("assinatura do QR Code offline não configurada")
	}
	certB64, certPassword, err := e.CertificateB64(ctx, orgPK)
	if err != nil {
		return "", err
	}
	resp, err := e.CallDfe(ctx, godfe.Request{
		CNPJ: issuerDoc, CertificateB64: certB64, CertificatePassword: certPassword,
		DocType: docType, Service: EgressServiceSignText, Body: map[string]any{EgressBodyText: text},
	})
	if err != nil {
		return "", problem.InternalServer("falha ao assinar o QR Code: " + err.Error())
	}
	if resp.StatusCode != 200 {
		return "", ProblemFromDfeBody(resp.StatusCode, resp.Body)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil || out[EgressBodySignature] == "" {
		return "", problem.InternalServer("resposta de assinatura em formato inesperado")
	}
	return out[EgressBodySignature], nil
}

// SignOnlyViaEgress has the egress build and sign body (the same payload the authorization sends)
// without transmitting it, and returns the signed XML.
func SignOnlyViaEgress(ctx context.Context, e EgressCaller, orgPK, issuerDoc, uf, sefazEnv, docType string, body map[string]any) (string, error) {
	if e == nil {
		return "", problem.InternalServer("assinatura da emissão offline não configurada")
	}
	certB64, certPassword, err := e.CertificateB64(ctx, orgPK)
	if err != nil {
		return "", err
	}
	resp, err := e.CallDfe(ctx, godfe.Request{
		CNPJ: issuerDoc, CertificateB64: certB64, CertificatePassword: certPassword,
		UF: uf, Environment: sefazEnv, DocType: docType, Service: EgressServiceSignOnly, Body: body,
	})
	if err != nil {
		return "", problem.InternalServer("falha ao assinar o documento: " + err.Error())
	}
	if resp.StatusCode != 200 {
		return "", ProblemFromDfeBody(resp.StatusCode, resp.Body)
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil || out[EgressBodyXML] == "" {
		return "", problem.InternalServer("resposta de assinatura do documento em formato inesperado")
	}
	return out[EgressBodyXML], nil
}

// MarkContingencyPending turns a document record into a pending contingency document: the sparse-index
// attributes with the deadline (now + window) and the worker message (signed XML as body) the
// retransmission sends later through the normal authorization service. msg.SefazService must be the
// document's authorization service. contingency_msg never leaves the API (api/v1/helpers.go).
func MarkContingencyPending(item map[string]types.AttributeValue, msg WorkerMessage, signedXML string, now time.Time, window time.Duration) error {
	msg.Body = map[string]any{EgressBodySignedXML: signedXML}
	raw, err := json.Marshal(msg)
	if err != nil {
		return problem.InternalServer("failed to encode contingency message")
	}
	item[AttrContingencySweep] = &types.AttributeValueMemberS{Value: ContingencySweepValue}
	item[AttrContingencyState] = &types.AttributeValueMemberS{Value: ContingencyPending}
	item[AttrContingencyDeadline] = &types.AttributeValueMemberS{Value: now.UTC().Add(window).Format(time.RFC3339)}
	item[AttrContingencyMsg] = &types.AttributeValueMemberS{Value: string(raw)}
	return nil
}

// ContingencySignedXML returns the signed document stored in contingency_msg, or "".
func ContingencySignedXML(item map[string]types.AttributeValue) string {
	s, _ := item[AttrContingencyMsg].(*types.AttributeValueMemberS)
	if s == nil {
		return ""
	}
	var msg struct {
		Body map[string]string `json:"body"`
	}
	if json.Unmarshal([]byte(s.Value), &msg) != nil {
		return ""
	}
	return msg.Body[EgressBodySignedXML]
}
