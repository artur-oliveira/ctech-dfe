package mdfes

// payment_events.go implements the MDF-e transport-payment events:
//
//	110116 evPagtoOperMDFe          — declaração do pagamento ao transportador
//	110117 evConfirmaServMDFe       — contratante confirma a prestação do serviço
//	110118 evAlteracaoPagtoServMDFe — alteração do pagamento declarado
//
// `infPag` é a MESMA estrutura de `infANTT/infPag` do modal rodoviário na
// emissão. `buildInfPag` é o único construtor dela — quando a emissão passar a
// emitir `infANTT/infPag` (Fase A do plano de cobertura de tags), ela reusa esta
// função em vez de ganhar uma cópia.

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
)

const (
	descEventoPagtoOper     = "Pagamento Operação MDF-e"
	descEventoConfirmaServ  = "Confirmação Serviço Transporte"
	descEventoAlteracaoPagt = "Alteração Pagamento Serviço MDFe"

	// tpCompOutros exige xComp preenchido (o XSD só aceita 01/02/03/04/99).
	tpCompOutros = "99"
)

// MdfePaymentComponent is one component of the freight payment (`Comp`).
type MdfePaymentComponent struct {
	Type        string  `json:"type" validate:"required,oneof=01 02 03 04 99"` // tpComp
	Value       string  `json:"value" validate:"required,decimalv"`            // vComp
	Description *string `json:"description" validate:"omitempty,max=60"`       // xComp — obrigatório quando type = 99
}

// MdfePaymentInstallment is one instalment of a term payment (`infPrazo`).
type MdfePaymentInstallment struct {
	Number  string `json:"number" validate:"required,max=3"`     // nParcela
	DueDate string `json:"due_date" validate:"required,isodate"` // dVenc (AAAA-MM-DD)
	Value   string `json:"value" validate:"required,decimalv"`   // vParcela
}

// MdfePaymentBank is the payment account (`infBanc`) — choice entre
// banco+agência, CNPJ da instituição de pagamento, ou chave PIX.
type MdfePaymentBank struct {
	BankCode   *string `json:"bank_code" validate:"omitempty,len=3,numeric"`    // codBanco
	AgencyCode *string `json:"agency_code" validate:"omitempty,max=10,numeric"` // codAgencia
	IPEFCNPJ   *string `json:"ipef_cnpj" validate:"omitempty,cnpj"`             // CNPJIPEF
	PIX        *string `json:"pix" validate:"omitempty,max=250"`                // PIX
}

// MdfePayment is one payee of the transport operation (`infPag`).
type MdfePayment struct {
	Name *string `json:"name" validate:"omitempty,max=60"` // xNome

	// Choice CPF | CNPJ | idEstrangeiro — exatamente um.
	CPF       *string `json:"cpf" validate:"omitempty,cpf"`
	CNPJ      *string `json:"cnpj" validate:"omitempty,cnpj"`
	ForeignID *string `json:"foreign_id" validate:"omitempty,max=20"`

	Components    []MdfePaymentComponent `json:"components" validate:"required,min=1,dive"`
	ContractValue string                 `json:"contract_value" validate:"required,decimalv"` // vContrato

	// indPag: 0 = à vista, 1 = a prazo. A prazo exige ao menos uma parcela.
	PaymentType string `json:"payment_type" validate:"required,oneof=0 1"`

	// HighPerformance (indAltoDesemp) só existe no modal rodoviário; nos
	// eventos de pagamento vem sempre nil.
	HighPerformance *string `json:"high_performance" validate:"omitempty,oneof=1"`

	AdvanceValue   *string `json:"advance_value" validate:"omitempty,decimalv"`    // vAdiant
	AdvanceRequest *string `json:"advance_request" validate:"omitempty,oneof=0 1"` // indAntecipaAdiant
	AdvanceKind    *string `json:"advance_kind" validate:"omitempty,oneof=0 1 2"`  // tpAntecip

	Installments []MdfePaymentInstallment `json:"installments" validate:"omitempty,dive"` // infPrazo
	Bank         MdfePaymentBank          `json:"bank"`
}

// MdfePaymentTrips is `infViagens` — quantas viagens e qual delas este
// pagamento cobre.
type MdfePaymentTrips struct {
	Total  string `json:"total" validate:"required,max=5,numeric"`  // qtdViagens
	Number string `json:"number" validate:"required,max=5,numeric"` // nroViagem
}

// paymentIndicatorTerm is indPag = "a prazo".
const paymentIndicatorTerm = "1"

// buildInfPag converts the request payees into the `infPag` list. Single source
// of truth for the group — shared by the three payment events and, once wired,
// by the rodoviário emission's infANTT.
func buildInfPag(payments []MdfePayment) ([]map[string]any, error) {
	if len(payments) == 0 {
		return nil, problem.BadRequest("informe ao menos um pagamento (infPag)")
	}
	out := make([]map[string]any, 0, len(payments))
	for i, p := range payments {
		node := map[string]any{}
		setIfPtr(node, "xNome", p.Name)

		switch {
		case p.CPF != nil && *p.CPF != "":
			node["CPF"] = onlyDigits(*p.CPF)
		case p.CNPJ != nil && *p.CNPJ != "":
			node["CNPJ"] = onlyDigits(*p.CNPJ)
		case p.ForeignID != nil && *p.ForeignID != "":
			node["idEstrangeiro"] = *p.ForeignID
		default:
			return nil, problem.BadRequest(fmt.Sprintf(
				"pagamento %d: informe CPF, CNPJ ou identificação de estrangeiro do recebedor", i+1))
		}

		comps := make([]map[string]any, 0, len(p.Components))
		for _, c := range p.Components {
			if c.Type == tpCompOutros && (c.Description == nil || *c.Description == "") {
				return nil, problem.BadRequest(fmt.Sprintf(
					"pagamento %d: componente tipo 99 exige descrição", i+1))
			}
			comp := map[string]any{"tpComp": c.Type, "vComp": c.Value}
			setIfPtr(comp, "xComp", c.Description)
			comps = append(comps, comp)
		}
		node["Comp"] = comps
		node["vContrato"] = p.ContractValue
		setIfPtr(node, "indAltoDesemp", p.HighPerformance)
		node["indPag"] = p.PaymentType

		setIfPtr(node, "vAdiant", p.AdvanceValue)
		setIfPtr(node, "indAntecipaAdiant", p.AdvanceRequest)

		if p.PaymentType == paymentIndicatorTerm && len(p.Installments) == 0 {
			return nil, problem.BadRequest(fmt.Sprintf(
				"pagamento %d: pagamento a prazo exige ao menos uma parcela", i+1))
		}
		if len(p.Installments) > 0 {
			parcels := make([]map[string]any, 0, len(p.Installments))
			for _, inst := range p.Installments {
				parcels = append(parcels, map[string]any{
					"nParcela": inst.Number,
					"dVenc":    inst.DueDate,
					"vParcela": inst.Value,
				})
			}
			node["infPrazo"] = parcels
		}
		setIfPtr(node, "tpAntecip", p.AdvanceKind)

		banc, err := buildInfBanc(p.Bank, i+1)
		if err != nil {
			return nil, err
		}
		node["infBanc"] = banc

		out = append(out, node)
	}
	return out, nil
}

// buildInfBanc resolves the infBanc choice: banco+agência, CNPJ da instituição
// de pagamento, ou chave PIX — exatamente um dos três.
func buildInfBanc(b MdfePaymentBank, idx int) (map[string]any, error) {
	switch {
	case b.PIX != nil && *b.PIX != "":
		return map[string]any{"PIX": *b.PIX}, nil
	case b.IPEFCNPJ != nil && *b.IPEFCNPJ != "":
		return map[string]any{"CNPJIPEF": onlyDigits(*b.IPEFCNPJ)}, nil
	case b.BankCode != nil && *b.BankCode != "":
		if b.AgencyCode == nil || *b.AgencyCode == "" {
			return nil, problem.BadRequest(fmt.Sprintf(
				"pagamento %d: informe a agência junto com o código do banco", idx))
		}
		return map[string]any{"codBanco": *b.BankCode, "codAgencia": *b.AgencyCode}, nil
	}
	return nil, problem.BadRequest(fmt.Sprintf(
		"pagamento %d: informe PIX, CNPJ da instituição de pagamento ou banco + agência", idx))
}

// PayTransportOperation dispatches evPagtoOperMDFe (110116): declares the
// payment made to the carrier for one trip of an authorized MDF-e.
func (s *MdfeService) PayTransportOperation(
	ctx context.Context, orgPK, accessKey string, trips MdfePaymentTrips,
	payments []MdfePayment, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	ec, nProt, err := s.resolvePaymentEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	infPag, err := buildInfPag(payments)
	if err != nil {
		return nil, err
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoPagamentoOper, seq, map[string]any{
		"evPagtoOperMDFe": map[string]any{
			"descEvento": descEventoPagtoOper,
			"nProt":      nProt,
			"infViagens": map[string]any{"qtdViagens": trips.Total, "nroViagem": trips.Number},
			"infPag":     infPag,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoPagamentoOper, seq, body, "", userID, userName)
}

// ConfirmService dispatches evConfirmaServMDFe (110117): the contratante
// confirms the transport service was rendered.
func (s *MdfeService) ConfirmService(
	ctx context.Context, orgPK, accessKey string, seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	ec, nProt, err := s.resolvePaymentEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoConfirmaServico, seq, map[string]any{
		"evConfirmaServMDFe": map[string]any{
			"descEvento": descEventoConfirmaServ,
			"nProt":      nProt,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoConfirmaServico, seq, body, "", userID, userName)
}

// ChangeServicePayment dispatches evAlteracaoPagtoServMDFe (110118): replaces
// the previously declared payment data.
func (s *MdfeService) ChangeServicePayment(
	ctx context.Context, orgPK, accessKey string, payments []MdfePayment,
	seq int, userID, userName string,
) (map[string]types.AttributeValue, error) {
	ec, nProt, err := s.resolvePaymentEvent(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	infPag, err := buildInfPag(payments)
	if err != nil {
		return nil, err
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoAlteracaoPagto, seq, map[string]any{
		"evAlteracaoPagtoServMDFe": map[string]any{
			"descEvento": descEventoAlteracaoPagt,
			"nProt":      nProt,
			"infPag":     infPag,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoAlteracaoPagto, seq, body, "", userID, userName)
}

// resolvePaymentEvent loads the event context and the authorization protocol
// the three payment events all require.
func (s *MdfeService) resolvePaymentEvent(ctx context.Context, orgPK, accessKey string) (*eventContext, string, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, "", err
	}
	nProt := strAttr(ec.mdfe, "sefaz_protocol")
	if nProt == "" {
		return nil, "", problem.BadRequest("protocolo de autorização não encontrado no MDF-e")
	}
	return ec, nProt, nil
}
