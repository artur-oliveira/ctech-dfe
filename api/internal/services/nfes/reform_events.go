package nfes

// reform_events.go: NF-e events of the tax reform (NT 2026.006, MOC "Eventos apuração do IBS e da CBS")
// and the payment link (110300). They are authorized at the SVRS, not at the issuer's UF.
//
//	112110 efetivo pagamento integral        emitente      indQuitacao
//	112140 fornecimento não realizado        emitente      gItemNaoFornecido
//	112150 data de previsão de entrega       emitente      dPrevEntrega
//	211110 apropriação de crédito presumido  emitente/dest gCredPresOper
//	211124 perecimento (transp. adquirente)  destinatário  gPerecimento
//	211128 aceite de débito (nota de crédito) destinatário indAceitacao
//	211130 imobilização de item              destinatário  gImobilizacao
//	211140 crédito de combustível            destinatário  gConsumoComb
//	211150 crédito dependente do adquirente  destinatário  gCredito
//	212110 / 212120 transferência de crédito sucessora (8) indAceitacao
//	110300 vinculação de pagamento           emitente      gPgto
//
// Not implemented: 112120 and 112130 (the MOC layout has unnamed or contradictory tags, see the spec),
// 211120 (removed by LC 227/2026), 412120 and 412130 (issued by the Fisco).

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	dfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	TpEventoPagamentoIntegral = "112110"
	TpEventoNaoFornecido      = "112140"
	TpEventoPrevEntrega       = "112150"
	TpEventoCredPresumido     = "211110"
	TpEventoPerecimentoAdq    = "211124"
	TpEventoAceiteDebito      = "211128"
	TpEventoImobilizacao      = "211130"
	TpEventoCredCombustivel   = "211140"
	TpEventoCredAdquirente    = "211150"
	TpEventoSucessaoIBS       = "212110"
	TpEventoSucessaoCBS       = "212120"
	TpEventoVinculoPagamento  = "110300"

	// tpAutor values of the reform events (MOC 8.x P21).
	TpAutorEmitente   = "1"
	TpAutorDestino    = "2"
	TpAutorSucessora  = "8"
	reformMaxItems    = 990
	reformIndicatorOn = 1
	dateLayout        = "2006-01-02"

	// CodeEventInvalid: the event request breaks a MOC layout rule that SEFAZ would reject.
	CodeEventInvalid = "event_invalid"
)

var (
	reCNPJ14   = regexp.MustCompile(`^\d{14}$`)
	reCNPJ8    = regexp.MustCompile(`^\d{8}$`)
	reDecimal2 = regexp.MustCompile(`^\d{1,13}(\.\d{2})$`)
	reQty4     = regexp.MustCompile(`^\d{1,11}(\.\d{1,4})?$`)
	rePct      = regexp.MustCompile(`^\d{1,3}\.\d{2,4}$`)
	reTwoDig   = regexp.MustCompile(`^\d{2}$`)
	reTransID  = regexp.MustCompile(`^.{2,35}$`)
)

// ReformItem is one det item of a reform event. Which fields apply depends on the event.
type ReformItem struct {
	ItemNumber int    `json:"item_number" validate:"required,gte=1,lte=990"` // nItem
	IBS        string `json:"ibs" validate:"omitempty,decimalv"`             // vIBS
	CBS        string `json:"cbs" validate:"omitempty,decimalv"`             // vCBS
	Quantity   string `json:"quantity" validate:"omitempty,decimalv"`        // q* of gControleEstoque
	Unit       string `json:"unit" validate:"omitempty,max=6"`               // u* of gControleEstoque
	CreditIBS  string `json:"credit_ibs" validate:"omitempty,decimalv"`      // 211150 vCredIBS
	CreditCBS  string `json:"credit_cbs" validate:"omitempty,decimalv"`      // 211150 vCredCBS
	// 211110
	BaseValue    string          `json:"base_value" validate:"omitempty,decimalv"`        // vBCCredPres
	PresumedCode string          `json:"presumed_credit_code" validate:"omitempty,len=2"` // cCredPres
	IBSPresumed  *PresumedCredit `json:"ibs_presumed_credit" validate:"omitempty"`
	CBSPresumed  *PresumedCredit `json:"cbs_presumed_credit" validate:"omitempty"`
}

// PresumedCredit is gIBSCredPres / gCBSCredPres of the event 211110.
type PresumedCredit struct {
	Code    string `json:"code" validate:"required,len=2"`     // cCredPres
	Percent string `json:"percent" validate:"required"`        // pCredPres, 3v2-4
	Value   string `json:"value" validate:"required,decimalv"` // vCredPres
}

// PaymentLink is gPgto of the event 110300.
type PaymentLink struct {
	TransactionID string `json:"transaction_id" validate:"required,min=2,max=35"` // idTransacao
	PaymentMethod string `json:"payment_method" validate:"required,len=2"`        // tpMeioPgto
	ReceiverCNPJ  string `json:"receiver_cnpj" validate:"required,len=14"`        // CNPJReceb
	PSPBaseCNPJ   string `json:"psp_base_cnpj" validate:"required,len=8"`         // CNPJBasePSP
}

// ReformEventRequest is the body of POST /nfes/:access_key/ibscbs-events.
type ReformEventRequest struct {
	EventType        string       `json:"event_type" validate:"required,oneof=112110 112140 112150 211110 211124 211128 211130 211140 211150 212110 212120 110300"`
	SequenceNumber   int          `json:"sequence_number" validate:"omitempty,gte=1,lte=20"`
	AuthorType       string       `json:"author_type" validate:"omitempty,oneof=1 2"` // only 211110 accepts both
	Indicator        *int         `json:"indicator" validate:"omitempty,oneof=0 1"`   // indQuitacao (must be 1) / indAceitacao
	DeliveryForecast string       `json:"delivery_forecast" validate:"omitempty,datetime=2006-01-02"`
	Items            []ReformItem `json:"items" validate:"omitempty,max=990,dive"`
	Payment          *PaymentLink `json:"payment" validate:"omitempty"`
}

// reformSpec describes one event: its literal descEvento (MOC 8.x, length-checked against the
// "Tam." column), who may author it and how its detEvento is built from the request.
type reformSpec struct {
	desc    string
	authors []string // allowed tpAutor; the first is the default
	build   func(r ReformEventRequest) (map[string]any, error)
}

func stockItems(group, qtyTag, unitTag string) func(ReformEventRequest) (map[string]any, error) {
	return func(r ReformEventRequest) (map[string]any, error) {
		items, err := eachItem(r.Items, func(it ReformItem) (map[string]any, error) {
			if !reDecimal2.MatchString(it.IBS) || !reDecimal2.MatchString(it.CBS) {
				return nil, fmt.Errorf("item %d: informe ibs e cbs com 2 casas decimais", it.ItemNumber)
			}
			if !reQty4.MatchString(it.Quantity) || it.Unit == "" {
				return nil, fmt.Errorf("item %d: informe quantity (até 4 casas) e unit", it.ItemNumber)
			}
			return map[string]any{
				"@nItem": strconv.Itoa(it.ItemNumber), "vIBS": it.IBS, "vCBS": it.CBS,
				"gControleEstoque": map[string]any{qtyTag: it.Quantity, unitTag: it.Unit},
			}, nil
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{group: items}, nil
	}
}

func eachItem(items []ReformItem, f func(ReformItem) (map[string]any, error)) ([]map[string]any, error) {
	if len(items) == 0 || len(items) > reformMaxItems {
		return nil, fmt.Errorf("informe de 1 a %d itens", reformMaxItems)
	}
	seen := map[int]bool{}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if seen[it.ItemNumber] { // the XSD rejects a repeated nItem (rule P24-05)
			return nil, fmt.Errorf("item %d repetido", it.ItemNumber)
		}
		seen[it.ItemNumber] = true
		m, err := f(it)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func presumed(p *PresumedCredit) (map[string]any, error) {
	if p == nil {
		return nil, nil
	}
	if !reTwoDig.MatchString(p.Code) || !rePct.MatchString(p.Percent) || !reDecimal2.MatchString(p.Value) {
		return nil, fmt.Errorf("crédito presumido: code com 2 dígitos, percent 3v2-4 e value com 2 casas")
	}
	// e211110 (pacote 010e): gIBSCredPres/gCBSCredPres têm só pCredPres e vCredPres; o cCredPres é do item
	// (gCredPresOper), então p.Code não vai ao XML.
	return map[string]any{"pCredPres": p.Percent, "vCredPres": p.Value}, nil
}

func indicator(tag string, onlyOn bool) func(ReformEventRequest) (map[string]any, error) {
	return func(r ReformEventRequest) (map[string]any, error) {
		if r.Indicator == nil {
			return nil, fmt.Errorf("informe indicator")
		}
		if onlyOn && *r.Indicator != reformIndicatorOn { // 112110: "Valor deve ser igual a 1"
			return nil, fmt.Errorf("indicator deve ser 1")
		}
		return map[string]any{tag: strconv.Itoa(*r.Indicator)}, nil
	}
}

var reformEvents = map[string]reformSpec{
	TpEventoPagamentoIntegral: {"Informação de efetivo pagamento integral para liberar crédito presumido do adquirente", []string{TpAutorEmitente}, indicator("indQuitacao", true)},
	TpEventoNaoFornecido:      {"Fornecimento não realizado com pagamento antecipado", []string{TpAutorEmitente}, stockItems("gItemNaoFornecido", "qNaoFornecida", "uNaoFornecida")},
	TpEventoPrevEntrega: {"Atualização da Data de Previsão de Entrega", []string{TpAutorEmitente}, func(r ReformEventRequest) (map[string]any, error) {
		if r.DeliveryForecast == "" {
			return nil, fmt.Errorf("informe delivery_forecast (AAAA-MM-DD)")
		}
		return map[string]any{"dPrevEntrega": r.DeliveryForecast}, nil
	}},
	TpEventoCredPresumido: {"Solicitação de Apropriação de crédito presumido", []string{TpAutorDestino, TpAutorEmitente}, func(r ReformEventRequest) (map[string]any, error) {
		items, err := eachItem(r.Items, func(it ReformItem) (map[string]any, error) {
			if !reDecimal2.MatchString(it.BaseValue) || !reTwoDig.MatchString(it.PresumedCode) {
				return nil, fmt.Errorf("item %d: informe base_value com 2 casas e presumed_credit_code com 2 dígitos", it.ItemNumber)
			}
			g := map[string]any{"@nItem": strconv.Itoa(it.ItemNumber), "vBCCredPres": it.BaseValue, "cCredPres": it.PresumedCode}
			for tag, p := range map[string]*PresumedCredit{"gIBSCredPres": it.IBSPresumed, "gCBSCredPres": it.CBSPresumed} {
				m, err := presumed(p)
				if err != nil {
					return nil, err
				}
				if m != nil {
					g[tag] = m
				}
			}
			return g, nil
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"gCredPresOper": items}, nil
	}},
	TpEventoPerecimentoAdq:  {"Perecimento, perda, roubo ou furto durante o transporte contratado pelo adquirente", []string{TpAutorDestino}, stockItems("gPerecimento", "qPerecimento", "uPerecimento")},
	TpEventoAceiteDebito:    {"Aceite de débito na apuração por emissão de nota de crédito", []string{TpAutorDestino}, indicator("indAceitacao", false)},
	TpEventoImobilizacao:    {"Imobilização de Item", []string{TpAutorDestino}, stockItems("gImobilizacao", "qImobilizado", "uImobilizado")},
	TpEventoCredCombustivel: {"Solicitação de Apropriação de Crédito de Combustível", []string{TpAutorDestino}, stockItems("gConsumoComb", "qComb", "uComb")},
	TpEventoCredAdquirente: {"Solicitação de Apropriação de Crédito para bens e serviços que dependem de atividade do adquirente", []string{TpAutorDestino}, func(r ReformEventRequest) (map[string]any, error) {
		items, err := eachItem(r.Items, func(it ReformItem) (map[string]any, error) {
			if !reDecimal2.MatchString(it.CreditIBS) || !reDecimal2.MatchString(it.CreditCBS) {
				return nil, fmt.Errorf("item %d: informe credit_ibs e credit_cbs com 2 casas decimais", it.ItemNumber)
			}
			return map[string]any{"@nItem": strconv.Itoa(it.ItemNumber), "vCredIBS": it.CreditIBS, "vCredCBS": it.CreditCBS}, nil
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"gCredito": items}, nil
	}},
	TpEventoSucessaoIBS: {"Manifestação sobre Pedido de Transferência de Crédito de IBS em Operação de Sucessão", []string{TpAutorSucessora}, indicator("indAceitacao", false)},
	TpEventoSucessaoCBS: {"Manifestação sobre Pedido de Transferência de Crédito de CBS em Operação de Sucessão", []string{TpAutorSucessora}, indicator("indAceitacao", false)},
}

// vinculoPagamento: 110300, author is always the emitente (rule P21-10 / 466).
const descVinculoPagamento = "Vinculação Pagamento"

// ReformEvent validates and dispatches a reform event or the payment link. The author type decides
// who must hold the NF-e: the emitente needs it authorized (nProt), the destinatário and the sucessora
// act on a NF-e they received.
func (s *NfeService) ReformEvent(
	ctx context.Context, orgPK, accessKey string, r ReformEventRequest, userID, userName string,
) (map[string]types.AttributeValue, error) {
	seq := r.SequenceNumber
	if seq <= 0 {
		seq = 1
	}
	var (
		desc, tpAutor string
		det           map[string]any
		err           error
	)
	if r.EventType == TpEventoVinculoPagamento {
		desc, tpAutor = descVinculoPagamento, TpAutorEmitente
	} else {
		spec, ok := reformEvents[r.EventType]
		if !ok {
			return nil, problem.BadRequest("evento da reforma tributária inválido: " + r.EventType).WithCode(CodeEventInvalid)
		}
		desc, tpAutor = spec.desc, spec.authors[0]
		if r.AuthorType != "" {
			if !containsString(spec.authors, r.AuthorType) {
				return nil, problem.UnprocessableEntity("tipo de autor não permitido para o evento " + r.EventType).WithCode(CodeEventInvalid)
			}
			tpAutor = r.AuthorType
		}
		if det, err = spec.build(r); err != nil {
			return nil, problem.UnprocessableEntity(err.Error()).WithCode(CodeEventInvalid)
		}
	}

	nfe, ectx, nProt, err := s.resolveEventTarget(ctx, orgPK, accessKey, tpAutor)
	if err != nil {
		return nil, err
	}
	if r.EventType == TpEventoVinculoPagamento {
		if det, err = paymentLinkDet(r.Payment, nProt); err != nil {
			return nil, problem.UnprocessableEntity(err.Error()).WithCode(CodeEventInvalid)
		}
	}
	completeReformDet(det, desc, authorUFCode(accessKey, ectx.emitUF, tpAutor), tpAutor, s.tech.Version)
	body := buildDetEventoBody(accessKey, ectx, r.EventType, seq, det)
	return s.dispatchExtraEvent(ctx, ectx, nfe, dfe.EventAuthorizerSVRS, accessKey, r.EventType, seq, body, userID, userName)
}

// completeReformDet adds the fields every reform event carries ahead of its own group.
func completeReformDet(det map[string]any, desc, cOrgaoAutor, tpAutor, verAplic string) {
	det["descEvento"] = desc
	det["cOrgaoAutor"] = cOrgaoAutor
	det["tpAutor"] = tpAutor
	det["verAplic"] = verAplic
}

// resolveEventTarget loads the NF-e for an event. The emitente author needs an authorized NF-e and its
// protocol; the other authors act on a received NF-e, which only has to exist.
func (s *NfeService) resolveEventTarget(ctx context.Context, orgPK, accessKey, tpAutor string) (
	map[string]types.AttributeValue, *nfeEventContext, string, error,
) {
	if tpAutor == TpAutorEmitente {
		return s.resolveAuthorizedEvent(ctx, orgPK, accessKey)
	}
	nfe, err := s.GetNFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, nil, "", err
	}
	if nfe == nil {
		return nil, nil, "", ErrNFeNotFound
	}
	ectx, err := resolveEventContext(ctx, s.orgRepo, s.certRepo, orgPK, nfe)
	if err != nil {
		return nil, nil, "", err
	}
	return nfe, ectx, strAttr(nfe, "sefaz_protocol"), nil
}

// authorUFCode is cOrgaoAutor: the issuer's UF code for the emitente (rule P20-10 / 455), the author's
// own UF for the others.
func authorUFCode(accessKey, authorUF, tpAutor string) string {
	if tpAutor == TpAutorEmitente || services.UFCode[authorUF] == "" {
		return accessKey[:2]
	}
	return services.UFCode[authorUF]
}

func paymentLinkDet(p *PaymentLink, nProt string) (map[string]any, error) {
	switch {
	case p == nil:
		return nil, fmt.Errorf("informe payment")
	case nProt == "":
		return nil, fmt.Errorf("protocolo de autorização da NF-e não encontrado")
	case !reTransID.MatchString(p.TransactionID), !reTwoDig.MatchString(p.PaymentMethod),
		!reCNPJ14.MatchString(p.ReceiverCNPJ), !reCNPJ8.MatchString(p.PSPBaseCNPJ):
		return nil, fmt.Errorf("payment inválido: transaction_id 2 a 35, payment_method 2 dígitos, receiver_cnpj 14 e psp_base_cnpj 8 dígitos")
	}
	return map[string]any{"nProt": nProt, "gPgto": map[string]any{
		"@idTransacao": p.TransactionID, "tpMeioPgto": p.PaymentMethod,
		"CNPJReceb": p.ReceiverCNPJ, "CNPJBasePSP": p.PSPBaseCNPJ,
	}}, nil
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
