package nfes

// nfce_offline.go: QR Code v3 offline of the NFC-e (tpEmis 9) and the egress calls that need the
// certificate (docs/specs/2026-11-08-contingencia.md section 8.2).
//
// p = chave|3|tpAmb|dia|vNF|tp_idDest|idDest|assinatura (Manual DANFE NFC-e 4.4.2). The signature is
// RSA SHA-1 (Base64) over parameters 1 to 7 with their "|", by the certificate that signs the NFC-e,
// which lives only in the go-dfe-egress Lambda: the API sends the text to the SignText operation.

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	// tp_idDest of the offline QR Code (Manual 4.4.2, parameter 6).
	qrDestCNPJ    = "1"
	qrDestCPF     = "2"
	qrDestForeign = "3"

	// dhEmi is "YYYY-MM-DDThh:mm:ss-03:00": the day (parameter 4) is characters 8 and 9.
	dhEmiDayStart, dhEmiDayEnd = 8, 10

	cnpjDigits = 14
	cpfDigits  = 11
)

// WithEgress sets the egress used by the offline flow (QR signature, sign-only).
func (s *NfceService) WithEgress(e services.EgressCaller) *NfceService {
	s.egress = e
	return s
}

// qrDestParams returns parameters 6 and 7 for the recipient document: CNPJ (1), CPF (2) or, for an
// unidentified recipient, both empty (only the separator is written). The NFC-e flow never carries an
// idEstrangeiro (kind 3 with an empty id is what the Manual prescribes for it).
func qrDestParams(destDoc string) (kind, id string) {
	switch len(destDoc) {
	case cnpjDigits:
		return qrDestCNPJ, destDoc
	case cpfDigits:
		return qrDestCPF, destDoc
	}
	return "", ""
}

// qrV3OfflineParams is the text that gets signed: parameters 1 to 7 with their separators.
// dhEmi is ide/dhEmi as written in the XML; vNF is W16, two decimals with a dot and no thousands separator.
func qrV3OfflineParams(accessKey string, environment int, dhEmi string, vNF, destDoc string) string {
	kind, id := qrDestParams(destDoc)
	return strings.Join([]string{
		accessKey, strconv.Itoa(qrCodeVersionV3), strconv.Itoa(environment),
		dhEmi[dhEmiDayStart:dhEmiDayEnd], vNF, kind, id,
	}, "|")
}

// buildQRCodeV3Offline appends the signature (parameter 8) to the signed text and builds the URL.
func buildQRCodeV3Offline(baseURL, signedParams, signature string) string {
	return appendQueryParam(baseURL, "p="+signedParams+"|"+signature)
}

// signText asks the egress to sign text with the organization's certificate (RSA SHA-1, Base64).
func (s *NfceService) signText(ctx context.Context, orgPK, issuerDoc, text string) (string, error) {
	return services.SignTextViaEgress(ctx, s.egress, orgPK, issuerDoc, "nfce", text)
}

// ── Emissão offline (tpEmis 9) ───────────────────────────────────────────────

const (
	// qrPlaceholder stands in for the qrCode in the XML sent to be signed in v2 offline, where
	// the QR needs the DigestValue the signature produces. The signature covers only infNFe, so
	// replacing the text in infNFeSupl afterwards keeps it valid.
	qrPlaceholder = "QRCODE_PLACEHOLDER"

	sefazServiceAuth = "NFeAutorizacao"
)

// resolveNFCeEmission picks the form of emission of an NFC-e from the fiscal configuration: normal,
// or offline (tpEmis 9) when the owner switched the document to it. EPEC for the NFC-e is not wired.
func resolveNFCeEmission(config map[string]types.AttributeValue) (mode EmissionMode, offline bool, err error) {
	mode = NormalEmission(nfModel65)
	if repositories.EmissionTypeOf(config) != repositories.EmissionOffline {
		return mode, false, nil
	}
	since, perr := time.Parse(time.RFC3339, strAttr(config, repositories.AttrContingencySince))
	if perr != nil {
		return mode, false, problem.InternalServer("emissão offline sem data de início válida (contingency_since)")
	}
	mode.TpEmis = TpEmisOffline
	mode.ContingencyAt = since
	mode.Justification = strAttr(config, repositories.AttrContingencyReason)
	return mode, true, nil
}

// offlineQR carries what the offline QR Code needs from the emission.
type offlineQR struct {
	orgPK, issuerDoc, uf string
	environment, version int
	dhEmi                string
	vNF                  string
	destDoc              string
	accessKey, csc       string
	cscID                string
}

// nfceSuplOf returns the infNFeSupl node of a built enviNFe.
func nfceSuplOf(enviNFe map[string]any) map[string]any {
	env, _ := enviNFe["enviNFe"].(map[string]any)
	nfe, _ := env["NFe"].(map[string]any)
	supl, _ := nfe["infNFeSupl"].(map[string]any)
	return supl
}

// applyOfflineQRv3 writes the final QR Code v3 (signed by the egress) into infNFeSupl. For v2 it only
// puts the placeholder, since the DigestValue is known after signing the XML.
func (s *NfceService) applyOfflineQRv3(ctx context.Context, q *offlineQR, enviNFe map[string]any) error {
	supl := nfceSuplOf(enviNFe)
	if supl == nil {
		return problem.InternalServer("infNFeSupl ausente na NFC-e offline")
	}
	if q.version != qrCodeVersionV3 {
		supl["qrCode"] = qrPlaceholder
		return nil
	}
	params := qrV3OfflineParams(q.accessKey, q.environment, q.dhEmi, q.vNF, q.destDoc)
	sig, err := s.signText(ctx, q.orgPK, q.issuerDoc, params)
	if err != nil {
		return err
	}
	supl["qrCode"] = buildQRCodeV3Offline(qrBaseForEnv(nfceQRBaseURL[q.uf], q.environment), params, sig)
	return nil
}

var digestValueRe = regexp.MustCompile(`<DigestValue>([^<]+)</DigestValue>`)

// qrV2OfflineParams is parameters 1 to 7 of the v2 offline QR: chave|2|tpAmb|dia|vNF|digVal|idCSC,
// with digVal the hexadecimal of the DigestValue text (Manual 4.3.5, step 1).
func qrV2OfflineParams(accessKey string, environment int, dhEmi string, vNF, digestValue, cscID string) string {
	return strings.Join([]string{
		accessKey, qrCodeVersion, strconv.Itoa(environment), dhEmi[dhEmiDayStart:dhEmiDayEnd],
		vNF, hex.EncodeToString([]byte(digestValue)), cscID,
	}, "|")
}

// buildQRCodeV2Offline appends the hash SHA1(params + CSC), upper-case hex (Manual 4.3.5, steps 3 and 4).
func buildQRCodeV2Offline(baseURL, params, csc string) string {
	sum := sha1.Sum([]byte(params + csc))
	return appendQueryParam(baseURL, "p="+params+"|"+strings.ToUpper(hex.EncodeToString(sum[:])))
}

// signOfflineXML has the egress sign the enviNFe (no transmission) and returns the signed XML. In v2
// the QR is built from the resulting DigestValue and replaces the placeholder.
func (s *NfceService) signOfflineXML(ctx context.Context, q *offlineQR, enviNFe map[string]any, sefazEnv string) (string, error) {
	xmlStr, err := services.SignOnlyViaEgress(ctx, s.egress, q.orgPK, q.issuerDoc, q.uf, sefazEnv, "nfce", enviNFe)
	if err != nil {
		return "", err
	}
	if q.version == qrCodeVersionV3 {
		return xmlStr, nil
	}
	m := digestValueRe.FindStringSubmatch(xmlStr)
	if m == nil {
		return "", problem.InternalServer("DigestValue ausente na NFC-e assinada")
	}
	params := qrV2OfflineParams(q.accessKey, q.environment, q.dhEmi, q.vNF, m[1], q.cscID)
	qr := buildQRCodeV2Offline(qrBaseForEnv(nfceQRBaseURL[q.uf], q.environment), params, q.csc)
	var esc strings.Builder
	if err := xml.EscapeText(&esc, []byte(qr)); err != nil {
		return "", problem.InternalServer("falha ao montar o QR Code")
	}
	return strings.Replace(xmlStr, qrPlaceholder, esc.String(), 1), nil
}

// markOfflinePending turns the NFC-e record into a pending contingency document: the sparse-index
// attributes with the 3-day deadline and the worker message (signed XML as body) the retransmission
// sends later through the normal authorization service.
func markOfflinePending(item map[string]types.AttributeValue, msg services.WorkerMessage, signedXML string, now time.Time) error {
	msg.SefazService = sefazServiceAuth
	return services.MarkContingencyPending(item, msg, signedXML, now, services.OfflineWindowNFCe)
}

// offlineSignedXML returns the signed enviNFe stored in contingency_msg, or "".
func offlineSignedXML(item map[string]types.AttributeValue) string {
	return services.ContingencySignedXML(item)
}
