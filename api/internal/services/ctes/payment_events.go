package ctes

// payment_events.go: CT-e payment link events (event XSDs evVincPgto and evCancVincPgto of the vendored
// CT-e 4.00 package, spec 2026-11-04-moc-gap-cte.md, 6.3), valid for the models 57 and 67:
//
//	110300 Vinculação do Pagamento (evVincPgto)                    nProt, pgto
//	110301 Cancelamento da vinculação do pagamento (evCancVincPgto) nProt, nProtVincPgto
//
// Not sourced in any repository document: who may author, which authorizer receives them, deadlines and the
// tpMeioPgto code list (IT DFe 2026.001). So the API does not restrict the author (the SEFAZ confers it), routes
// to the normal CT-e authorizer and reuses the NF-e payment method table in the UI. See the spec, "Perguntas em aberto".

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
)

const (
	TpEventoVinculoPagamento     = "110300"
	TpEventoCancVinculoPagamento = "110301"

	// descEvento: the first literal of the XSD enumeration (the second is the same text without accents).
	descVinculoPagamento     = "Vinculação do Pagamento"
	descCancVinculoPagamento = "Cancelamento da vinculação do pagamento"

	maxPaymentNumber = 990 // nPag pattern: 1 to 990
)

var (
	paymentTxRe     = regexp.MustCompile(`^[!-ÿ][ -ÿ]*[!-ÿ]$`)  // idTransacao (TStringRTC), 2 to 35 characters
	paymentMethodRe = regexp.MustCompile(`^\d{2}$`)             // tpMeioPgto
	paymentCNPJRe   = regexp.MustCompile(`^[A-Z0-9]{12}\d{2}$`) // CNPJReceb (TCnpjRTC)
	paymentBaseRe   = regexp.MustCompile(`^[A-Z0-9]{8}$`)       // CNPJBasePSP (TCnpjBaseRTC)
)

// PaymentLink is the body of 110300 (pgto: attributes nPag and idTransacao, children tpMeioPgto, CNPJReceb, CNPJBasePSP).
type PaymentLink struct {
	Number        int    `json:"number" validate:"required,gte=1,lte=990"`        // nPag
	TransactionID string `json:"transaction_id" validate:"required,min=2,max=35"` // idTransacao
	PaymentMethod string `json:"payment_method" validate:"required,len=2"`        // tpMeioPgto
	ReceiverCNPJ  string `json:"receiver_cnpj" validate:"required,len=14"`        // CNPJReceb
	PSPBaseCNPJ   string `json:"psp_base_cnpj" validate:"required,len=8"`         // CNPJBasePSP
}

func (p PaymentLink) check() error {
	n := len([]rune(p.TransactionID))
	switch {
	case p.Number < 1 || p.Number > maxPaymentNumber:
		return fmt.Errorf("number vai de 1 a %d", maxPaymentNumber)
	case n < 2 || n > 35 || !paymentTxRe.MatchString(p.TransactionID):
		return fmt.Errorf("transaction_id de 2 a 35 caracteres, sem espaço no início ou no fim")
	case !paymentMethodRe.MatchString(p.PaymentMethod):
		return fmt.Errorf("payment_method com 2 dígitos")
	case !paymentCNPJRe.MatchString(p.ReceiverCNPJ):
		return fmt.Errorf("receiver_cnpj com 14 caracteres")
	case !paymentBaseRe.MatchString(p.PSPBaseCNPJ):
		return fmt.Errorf("psp_base_cnpj com 8 caracteres")
	}
	return nil
}

func paymentLinkDet(p PaymentLink, nProt string) map[string]any {
	return map[string]any{
		"descEvento": descVinculoPagamento, "nProt": nProt,
		"pgto": map[string]any{
			"@nPag": fmt.Sprintf("%d", p.Number), "@idTransacao": p.TransactionID,
			"tpMeioPgto": p.PaymentMethod, "CNPJReceb": p.ReceiverCNPJ, "CNPJBasePSP": p.PSPBaseCNPJ,
		},
	}
}

func cancPaymentLinkDet(nProt, linkProt string) map[string]any {
	return map[string]any{"descEvento": descCancVinculoPagamento, "nProt": nProt, "nProtVincPgto": linkProt}
}

// RegisterPaymentLink registers the vinculação de pagamento (110300) of an authorized CT-e 57 or 67.
func (s *CteService) RegisterPaymentLink(ctx context.Context, orgPK, accessKey string, p PaymentLink, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if err := p.check(); err != nil {
		return nil, unprocessable(CodeEventInvalid, err.Error())
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt, err := ec.protocol()
	if err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoVinculoPagamento, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoVinculoPagamento, seq, "evVincPgto", paymentLinkDet(p, nProt))
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoVinculoPagamento, seq, body, "", userID, userName)
}

// CancelPaymentLink cancels a 110300 (110301). The event is chosen by sk; its protocol comes from the record, never typed.
func (s *CteService) CancelPaymentLink(ctx context.Context, orgPK, accessKey, eventSK string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt, err := ec.protocol()
	if err != nil {
		return nil, err
	}
	linkProt, err := s.authorizedEventProtocol(ctx, accessKey, eventSK, TpEventoVinculoPagamento)
	if err != nil {
		return nil, err
	}
	if seq, err = s.nextSequence(ctx, accessKey, TpEventoCancVinculoPagamento, seq); err != nil {
		return nil, err
	}
	body := buildEventEnvelope(ec, accessKey, TpEventoCancVinculoPagamento, seq, "evCancVincPgto", cancPaymentLinkDet(nProt, linkProt))
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCancVinculoPagamento, seq, body, "", userID, userName)
}

// authorizedEventProtocol returns the SEFAZ protocol of the event sk, which must be of type want and authorized.
func (s *CteService) authorizedEventProtocol(ctx context.Context, accessKey, eventSK, want string) (string, error) {
	target, err := s.eventRepo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return "", err
	}
	if target == nil {
		return "", problem.NotFound("evento não encontrado").WithCode(CodeEventNotFound)
	}
	prot := strAttr(target, "sefaz_protocol")
	if strAttr(target, "event_type") != want || strAttr(target, "status") != eventStatusSuccess || !protocolRe.MatchString(prot) {
		return "", unprocessable(CodeEventNotActive, "o evento escolhido não é um evento "+want+" autorizado")
	}
	return prot, nil
}
