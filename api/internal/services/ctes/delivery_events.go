package ctes

// delivery_events.go: CT-e delivery events of the MOC CTe/RecepcaoEventosEspecificos.html (items 6 and 7):
//
//	110180 Comprovante de Entrega (evCECTe)        110181 Cancelamento (evCancCECTe)
//	110190 Insucesso na Entrega (evIECTe)          110191 Cancelamento (evCancIECTe)
//
// Author: the issuer of the CT-e (model 57), CT-e authorized. A homologated event answers cStat 135.
// An already registered event (631) is a success in the worker.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/problem"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	TpEventoComprovanteEntrega     = "110180"
	TpEventoCancComprovanteEntrega = "110181"
	TpEventoInsucessoEntrega       = "110190"
	TpEventoCancInsucessoEntrega   = "110191"

	descComprovanteEntrega     = "Comprovante de Entrega do CT-e"
	descCancComprovanteEntrega = "Cancelamento do Comprovante de Entrega do CT-e"
	descInsucessoEntrega       = "Insucesso na Entrega do CT-e"
	descCancInsucessoEntrega   = "Cancelamento do Insucesso de Entrega do CT-e"

	maxDeliveryDocs  = 2000 // infEntrega 0-2000
	minJustInsucesso = 25
	maxJustInsucesso = 250
	tpMotivoOutros   = 4
)

var (
	deliveryHashRe = regexp.MustCompile(`^[A-Za-z0-9+/]{27}=$`) // SHA-1 in Base64, 28 characters
	latitudeRe     = regexp.MustCompile(`^-?\d{1,2}\.\d{1,6}$`)
	longitudeRe    = regexp.MustCompile(`^-?\d{1,3}\.\d{1,6}$`)
	nfeKeyRe       = regexp.MustCompile(`^\d{44}$`)
)

// DeliveryProof is the common body of 110180 and 110190: when, where, who, hash and the NF-e keys.
type DeliveryProof struct {
	OccurredAt time.Time `json:"occurred_at" validate:"required"` // dhEntrega / dhTentativaEntrega
	HashAt     time.Time `json:"hash_at" validate:"required"`     // dhHashEntrega / dhHashTentativaEntrega
	Hash       string    `json:"hash" validate:"required,len=28"` // hashEntrega / hashTentativaEntrega
	Latitude   string    `json:"latitude"`
	Longitude  string    `json:"longitude"`
	NFeKeys    []string  `json:"nfe_keys" validate:"omitempty,max=2000,dive,len=44,numeric"` // infEntrega/chNFe
}

// DeliveryReceipt is the body of 110180.
type DeliveryReceipt struct {
	DeliveryProof
	DocNumber string `json:"doc_number" validate:"required,min=2,max=20"` // nDoc
	Name      string `json:"name" validate:"required,min=2,max=60"`       // xNome
}

// DeliveryFailure is the body of 110190.
type DeliveryFailure struct {
	DeliveryProof
	Attempt       int    `json:"attempt" validate:"omitempty,gte=1,lte=999"` // nTentativa
	ReasonType    int    `json:"reason_type" validate:"required,oneof=1 2 3 4"`
	Justification string `json:"justification"` // xJustMotivo, only for reason_type 4
}

func (p DeliveryProof) check() error {
	switch {
	case !deliveryHashRe.MatchString(p.Hash):
		return fmt.Errorf("hash deve ser um SHA-1 em Base64 (28 caracteres)")
	case p.Latitude != "" && !latitudeRe.MatchString(p.Latitude), p.Longitude != "" && !longitudeRe.MatchString(p.Longitude):
		return fmt.Errorf("latitude e longitude com até 6 casas decimais")
	case (p.Latitude == "") != (p.Longitude == ""):
		return fmt.Errorf("informe latitude e longitude juntas")
	case len(p.NFeKeys) > maxDeliveryDocs:
		return fmt.Errorf("no máximo %d chaves de NF-e", maxDeliveryDocs)
	}
	seen := map[string]bool{}
	for _, k := range p.NFeKeys {
		if !nfeKeyRe.MatchString(k) {
			return fmt.Errorf("chave de NF-e inválida: %s", k)
		}
		if seen[k] { // rejeição 861 / 924
			return fmt.Errorf("NF-e em duplicidade: %s", k)
		}
		seen[k] = true
	}
	return nil
}

// detail builds the common tail; lat/long go before the hash, infEntrega last (xsdorder fixes the order).
func (p DeliveryProof) detail(det map[string]any, hashTag, hashAtTag string) map[string]any {
	if p.Latitude != "" {
		det["latitude"], det["longitude"] = p.Latitude, p.Longitude
	}
	det[hashTag] = p.Hash
	det[hashAtTag] = p.HashAt.Format(layoutDateTimeTZ)
	if len(p.NFeKeys) > 0 {
		docs := make([]map[string]any, 0, len(p.NFeKeys))
		for _, k := range p.NFeKeys {
			docs = append(docs, map[string]any{"chNFe": k})
		}
		det["infEntrega"] = docs
	}
	return det
}

func receiptDet(r DeliveryReceipt, nProt string) map[string]any {
	return r.detail(map[string]any{
		"descEvento": descComprovanteEntrega, "nProt": nProt, "dhEntrega": r.OccurredAt.Format(layoutDateTimeTZ),
		"nDoc": r.DocNumber, "xNome": r.Name,
	}, "hashEntrega", "dhHashEntrega")
}

func failureDet(f DeliveryFailure, nProt, just string) map[string]any {
	det := map[string]any{
		"descEvento": descInsucessoEntrega, "nProt": nProt, "dhTentativaEntrega": f.OccurredAt.Format(layoutDateTimeTZ),
		"tpMotivo": fmt.Sprintf("%d", f.ReasonType),
	}
	if f.Attempt > 0 {
		det["nTentativa"] = fmt.Sprintf("%03d", f.Attempt)
	}
	if just != "" {
		det["xJustMotivo"] = just
	}
	return f.detail(det, "hashTentativaEntrega", "dhHashTentativaEntrega")
}

// RegisterDeliveryReceipt registers the comprovante de entrega (110180). O05 (complementar) and the NF-e checks
// (O11 to O17) are left to the SEFAZ.
func (s *CteService) RegisterDeliveryReceipt(ctx context.Context, orgPK, accessKey string, r DeliveryReceipt, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := r.check(); err != nil {
		return nil, unprocessable(CodeEventInvalid, err.Error())
	}
	r.DocNumber, r.Name = strings.TrimSpace(r.DocNumber), strings.TrimSpace(r.Name)
	if n, m := len([]rune(r.DocNumber)), len([]rune(r.Name)); n < 2 || n > 20 || m < 2 || m > 60 {
		return nil, unprocessable(CodeEventInvalid, "doc_number de 2 a 20 e name de 2 a 60 caracteres")
	}
	ec, nProt, err := s.issuerEventTarget(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoComprovanteEntrega, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoComprovanteEntrega, seq, "evCECTe", receiptDet(r, nProt))
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoComprovanteEntrega, seq, body, "", userID, userName)
}

// RegisterDeliveryFailure registers the insucesso na entrega (110190). xJustMotivo only with tpMotivo 4 (Q18, 926).
func (s *CteService) RegisterDeliveryFailure(ctx context.Context, orgPK, accessKey string, f DeliveryFailure, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := f.check(); err != nil {
		return nil, unprocessable(CodeEventInvalid, err.Error())
	}
	just := strings.TrimSpace(f.Justification)
	switch n := len([]rune(just)); {
	case f.ReasonType == tpMotivoOutros && (n < minJustInsucesso || n > maxJustInsucesso):
		return nil, unprocessable(CodeEventInvalid, fmt.Sprintf("a justificativa do motivo 4 vai de %d a %d caracteres", minJustInsucesso, maxJustInsucesso))
	case f.ReasonType != tpMotivoOutros && n > 0:
		return nil, unprocessable(CodeEventInvalid, "a justificativa só vale para o motivo 4")
	}
	ec, nProt, err := s.issuerEventTarget(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoInsucessoEntrega, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoInsucessoEntrega, seq, "evIECTe", failureDet(f, nProt, just))
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoInsucessoEntrega, seq, body, "", userID, userName)
}

// CancelDeliveryEvent cancels a 110180 (110181) or a 110190 (110191). target is the event chosen by sk; its
// protocol comes from the record, never typed (rule O03 / 866).
func (s *CteService) CancelDeliveryEvent(ctx context.Context, orgPK, accessKey, eventSK string, insucesso bool, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, nProt, err := s.issuerEventTarget(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	target, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, problem.NotFound("evento não encontrado").WithCode(CodeEventNotFound)
	}
	want, tpCancel, root, desc, tag := TpEventoComprovanteEntrega, TpEventoCancComprovanteEntrega, "evCancCECTe", descCancComprovanteEntrega, "nProtCE"
	if insucesso {
		want, tpCancel, root, desc, tag = TpEventoInsucessoEntrega, TpEventoCancInsucessoEntrega, "evCancIECTe", descCancInsucessoEntrega, "nProtIE"
	}
	evProt := strAttr(target, "sefaz_protocol")
	if strAttr(target, "event_type") != want || strAttr(target, "status") != eventStatusSuccess || !protocolRe.MatchString(evProt) {
		return nil, unprocessable(CodeEventNotActive, "o evento escolhido não é um evento "+want+" autorizado")
	}
	if seq, err = s.nextSequence(ctx, accessKey, tpCancel, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, tpCancel, seq, root, map[string]any{"descEvento": desc, "nProt": nProt, tag: evProt})
	return s.dispatchEvent(ctx, ec, accessKey, tpCancel, seq, body, "", userID, userName)
}

// issuerEventTarget loads the CT-e for an issuer-only event and returns its authorization protocol.
func (s *CteService) issuerEventTarget(ctx context.Context, orgPK, accessKey string) (*eventContext, string, error) {
	if err := requireModel57(accessKey); err != nil {
		return nil, "", err
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, "", err
	}
	if err := ec.requireIssuer(); err != nil {
		return nil, "", err
	}
	nProt, err := ec.protocol()
	return ec, nProt, err
}
