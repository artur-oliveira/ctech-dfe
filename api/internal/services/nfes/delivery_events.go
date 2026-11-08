package nfes

// delivery_events.go: NF-e events 110130 (comprovante de entrega), 110131 (cancelamento do
// comprovante) and 110150 (ator interessado, transportador). MOC NFeRecepcaoEvento pages
// "Comprovante de Entrega da NF-e", "Cancelamento Comprovante de Entrega da NF-e" and
// "Ator Interessado na NF-e - Transportador".
//
// 110130 and 110131 exist only in the Ambiente Nacional web service; 110150 goes to the
// issuer's UF authorizer (rule P12-40: "UF da Chave de Acesso diverge da UF autorizadora").

import (
	"context"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	TpEventoComprovanteEntrega     = "110130"
	TpEventoCancComprovanteEntrega = "110131"
	TpEventoAtorInteressado        = "110150"

	// ponytail: the descEvento literals come from the MOC event titles; the XSD enumerations are
	// not vendored, so they are unverified (see docs/specs/2026-11-04-moc-gap-nfe-nfce.md, F9).
	descComprovanteEntrega     = "Comprovante de Entrega da NF-e"
	descCancComprovanteEntrega = "Cancelamento Comprovante de Entrega da NF-e"
	descAtorInteressado        = "Ator interessado na NF-e"

	// xCondUso of 110150 when tpAutorizacao = 1 (MOC P27).
	condUsoAtorInteressado = "O emitente ou destinatário da NFe, declara que permite o transportador declarado no campo CNPJ/CPF deste evento a autorizar os transportadores subcontratados ou redespachados a terem acesso ao download da NF-e"

	// AnUF routes an event to the Ambiente Nacional web service.
	anUF = "AN"
)

var (
	reLatitude   = regexp.MustCompile(`^-?\d{1,2}\.\d{6}$`)
	reLongitude  = regexp.MustCompile(`^-?\d{1,3}\.\d{6}$`)
	reHashSHA1   = regexp.MustCompile(`^[A-Za-z0-9+/]{27}=$`) // SHA-1 in Base64: 28 characters
	reProtocol   = regexp.MustCompile(`^\d{15}$`)
	reDocNumber  = regexp.MustCompile(`^.{2,20}$`)
	rePersonName = regexp.MustCompile(`^.{2,60}$`)
)

// DeliveryReceipt is the body of the event 110130.
type DeliveryReceipt struct {
	DeliveredAt time.Time  `json:"delivered_at" validate:"required"`            // dhEntrega
	DocNumber   string     `json:"doc_number" validate:"required,min=2,max=20"` // nDoc
	Name        string     `json:"name" validate:"required,min=2,max=60"`       // xNome
	Latitude    string     `json:"latitude" validate:"omitempty"`               // latGPS [-]2v6
	Longitude   string     `json:"longitude" validate:"omitempty"`              // longGPS [-]3v6
	ReceiptHash string     `json:"receipt_hash" validate:"required,len=28"`     // hashComprovante
	HashAt      *time.Time `json:"hash_at" validate:"omitempty"`                // dhHashComprovante
}

// RegisterDeliveryReceipt dispatches the comprovante de entrega (110130) to the Ambiente Nacional.
func (s *NfeService) RegisterDeliveryReceipt(
	ctx context.Context, orgPK, accessKey string, r DeliveryReceipt, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	switch {
	case !reDocNumber.MatchString(r.DocNumber), !rePersonName.MatchString(r.Name):
		return nil, eventInvalid("doc_number e name devem ter de 2 a 20 e de 2 a 60 caracteres")
	case !reHashSHA1.MatchString(r.ReceiptHash):
		return nil, eventInvalid("receipt_hash deve ser um SHA-1 em Base64 (28 caracteres)")
	case r.Latitude != "" && !reLatitude.MatchString(r.Latitude), r.Longitude != "" && !reLongitude.MatchString(r.Longitude):
		return nil, eventInvalid("latitude e longitude com 6 casas decimais")
	case (r.Latitude == "") != (r.Longitude == ""):
		return nil, eventInvalid("informe latitude e longitude juntas")
	}
	nfe, ectx, _, err := s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	det := map[string]any{
		"descEvento": descComprovanteEntrega, "cOrgaoAutor": accessKey[:2], "tpAutor": TpAutorEmitente,
		"verAplic": s.tech.Version, "dhEntrega": fmtDhEmi(r.DeliveredAt),
		"nDoc": r.DocNumber, "xNome": r.Name,
	}
	if r.Latitude != "" {
		det["latGPS"], det["longGPS"] = r.Latitude, r.Longitude
	}
	det["hashComprovante"] = r.ReceiptHash
	if r.HashAt != nil {
		// rejections 1xx: the hash time cannot be after the event nor before the NF-e issue date;
		// SEFAZ checks both, the API only forwards it.
		det["dhHashComprovante"] = fmtDhEmi(*r.HashAt)
	}
	body := buildDetEventoBody(accessKey, ectx, TpEventoComprovanteEntrega, seq, det)
	body = withEventOrgao(body, cOrgaoAN)
	return s.dispatchExtraEvent(ctx, ectx, nfe, anUF, accessKey, TpEventoComprovanteEntrega, seq, body, userID, userName)
}

// CancelDeliveryReceipt dispatches 110131; deliveryProtocol is the nProt of the 110130 being cancelled.
func (s *NfeService) CancelDeliveryReceipt(
	ctx context.Context, orgPK, accessKey, deliveryProtocol string, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	if !reProtocol.MatchString(deliveryProtocol) {
		return nil, eventInvalid("delivery_protocol deve ter 15 dígitos")
	}
	nfe, ectx, _, err := s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	det := map[string]any{
		"descEvento": descCancComprovanteEntrega, "cOrgaoAutor": accessKey[:2], "tpAutor": TpAutorEmitente,
		"verAplic": s.tech.Version, "nProtEvento": deliveryProtocol,
	}
	body := withEventOrgao(buildDetEventoBody(accessKey, ectx, TpEventoCancComprovanteEntrega, seq, det), cOrgaoAN)
	return s.dispatchExtraEvent(ctx, ectx, nfe, anUF, accessKey, TpEventoCancComprovanteEntrega, seq, body, userID, userName)
}

// InterestedActor is the body of the event 110150: the transporter allowed to download the XML.
type InterestedActor struct {
	CNPJ              string `json:"cnpj" validate:"omitempty,len=14,numeric"`
	CPF               string `json:"cpf" validate:"omitempty,len=11,numeric"`
	AllowSubcontracts bool   `json:"allow_subcontracts"` // tpAutorizacao 1
	AuthorType        string `json:"author_type" validate:"omitempty,oneof=1 2"`
}

// RegisterInterestedActor dispatches 110150. The emitente (tpAutor 1) or the destinatário (2) author it,
// so tpAutorizacao is always sent (rule P26-10 / 827), with xCondUso exactly when it is 1 (P27-10 / 829).
func (s *NfeService) RegisterInterestedActor(
	ctx context.Context, orgPK, accessKey string, a InterestedActor, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	if (a.CNPJ == "") == (a.CPF == "") {
		return nil, eventInvalid("informe cnpj ou cpf do transportador, apenas um")
	}
	tpAutor := a.AuthorType
	if tpAutor == "" {
		tpAutor = TpAutorEmitente
	}
	nfe, ectx, _, err := s.resolveEventTarget(ctx, orgPK, accessKey, tpAutor)
	if err != nil {
		return nil, err
	}
	aut := map[string]any{"CNPJ": a.CNPJ}
	if a.CNPJ == "" {
		aut = map[string]any{"CPF": a.CPF}
	}
	tpAut := "0"
	det := map[string]any{
		"descEvento": descAtorInteressado, "cOrgaoAutor": authorUFCode(accessKey, ectx.emitUF, tpAutor), "tpAutor": tpAutor,
		"verAplic": s.tech.Version, "autXML": aut,
	}
	if a.AllowSubcontracts {
		tpAut = "1"
		det["xCondUso"] = condUsoAtorInteressado
	}
	det["tpAutorizacao"] = tpAut
	body := buildDetEventoBody(accessKey, ectx, TpEventoAtorInteressado, seq, det)
	return s.dispatchExtraEvent(ctx, ectx, nfe, authorizerUF(accessKey, ectx.emitUF, tpAutor), accessKey, TpEventoAtorInteressado, seq, body, userID, userName)
}

// authorizerUF is the UF whose authorizer receives an event authored by the issuer (its own UF) or by
// another party (the UF of the NF-e, which is the authorizer of the document).
func authorizerUF(accessKey, orgUF, tpAutor string) string {
	if tpAutor == TpAutorEmitente {
		return orgUF
	}
	return ufOfKey(accessKey)
}

func eventInvalid(msg string) error {
	return problem.UnprocessableEntity(msg).WithCode(CodeEventInvalid)
}

// withEventOrgao sets infEvento/cOrgao: the Ambiente Nacional events carry 91, not the key's UF.
func withEventOrgao(body map[string]any, cOrgao string) map[string]any {
	inf := body["envEvento"].(map[string]any)["evento"].(map[string]any)["infEvento"].(map[string]any)
	inf["cOrgao"] = cOrgao
	return body
}

// ufOfKey is the UF sigla of the IBGE code in the first two digits of an access key.
func ufOfKey(accessKey string) string {
	for uf, code := range services.UFCode {
		if code == accessKey[:2] {
			return uf
		}
	}
	return ""
}
