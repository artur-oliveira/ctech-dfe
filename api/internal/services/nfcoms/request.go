package nfcoms

import (
	"context"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// NfcomEmitRequest é o corpo de POST /nfcoms: o id do assinante, os planos escolhidos e o que muda a cada fatura.
// Cliente, endereço, IE, classificação, unidade e tributos vêm dos cadastros (assinante, pessoa e plano); o servidor
// monta o NfcomEmitBody completo (resolve) e o resto do fluxo é o mesmo de sempre.
type NfcomEmitRequest struct {
	SubscriberID string `json:"subscriber_id" validate:"required,refcode"`
	// Items: um plano por linha; quantidade e, se o plano não fixa o valor, o valor unitário.
	Items []NfcomItemRequest `json:"items" validate:"required,min=1,max=990,dive"`
	// Invoice é o grupo gFat: obrigatório no faturamento normal, salvo serviço pré-pago.
	Invoice *NfcomInvoice `json:"invoice" validate:"omitempty"`
	Prepaid bool          `json:"prepaid"`
	// AdditionalInfo é texto livre: normalizado (CleanText) e limitado a 3000 caracteres do leiaute.
	AdditionalInfo    *string  `json:"additional_info" validate:"omitempty,max=3000,fiscaltext"`
	AuthorizedXML     []string `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	ContingencyReason *string  `json:"contingency_reason" validate:"omitempty,min=15,max=256,fiscaltext"`
}

// NfcomItemRequest é a linha da fatura: o plano e as quantidades. O valor unitário só vale quando o plano não tem
// valor padrão; desconto e outras despesas são sempre da fatura.
type NfcomItemRequest struct {
	PlanID       string  `json:"plan_id" validate:"required,refcode"`
	Quantity     string  `json:"quantity" validate:"required,decimalv"`
	UnitValue    *string `json:"unit_value" validate:"omitempty,money"`
	Discount     *string `json:"discount" validate:"omitempty,money"`
	OtherCharges *string `json:"other_charges" validate:"omitempty,money"`
}

const (
	customerNameMax   = 60 // xNome do dest
	skPrefixCNPJ      = "CNPJ_"
	skPrefixCPF       = "CPF_"
	ufExterior        = "EX"
	ieIsentoLiteral   = "ISENTO"
	defaultAddrNumber = "S/N"
	ieNaoContribuinte = "9"
)

// ValidatePlan confere um plano com as mesmas regras de item da emissão (CFOP x ICMS, alíquotas): o cadastro recusa
// o plano que a emissão recusaria.
func ValidatePlan(p NfcomItem) error {
	p.Quantity = "1"
	return validateItem(1, p)
}

// Resolve monta o NfcomEmitBody a partir dos cadastros.
func (s *NfcomService) Resolve(ctx context.Context, orgPK string, req NfcomEmitRequest) (NfcomEmitBody, error) {
	subItem, err := s.subscriberRepo.Get(ctx, orgPK, req.SubscriberID)
	if err != nil {
		return NfcomEmitBody{}, err
	}
	if subItem == nil {
		return NfcomEmitBody{}, problem.NotFound("assinante não encontrado")
	}
	subscriber := decode(subItem)
	personSK := anyStr(subscriber, "person_id")
	personItem, err := s.personRepo.Get(ctx, orgPK, personSK)
	if err != nil {
		return NfcomEmitBody{}, err
	}
	if personItem == nil {
		return NfcomEmitBody{}, problem.BadRequest("a pessoa do assinante não está mais cadastrada")
	}
	customer, err := customerFromPerson(personSK, decode(personItem))
	if err != nil {
		return NfcomEmitBody{}, err
	}
	items := make([]NfcomItem, 0, len(req.Items))
	for i, line := range req.Items {
		planItem, err := s.planRepo.Get(ctx, orgPK, line.PlanID)
		if err != nil {
			return NfcomEmitBody{}, err
		}
		if planItem == nil {
			return NfcomEmitBody{}, problem.NotFound("plano não encontrado no item " + strconv.Itoa(i+1))
		}
		item, err := itemFromPlan(decode(planItem), line)
		if err != nil {
			return NfcomEmitBody{}, problem.BadRequest("item " + strconv.Itoa(i+1) + ": " + err.Error())
		}
		items = append(items, item)
	}
	return NfcomEmitBody{
		Customer: customer, Subscriber: subscriberFromRecord(subscriber), Items: items,
		Invoice: req.Invoice, Prepaid: req.Prepaid, AuthorizedXML: req.AuthorizedXML,
		ContingencyReason: cleanPtr(req.ContingencyReason), AdditionalInfo: cleanPtr(req.AdditionalInfo),
	}, nil
}

func decode(item map[string]types.AttributeValue) map[string]any {
	m := map[string]any{}
	_ = attributevalue.UnmarshalMap(item, &m) // item já veio do DynamoDB: decode para mapa genérico não falha
	return m
}

func cleanPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := services.CleanText(*p)
	if v == "" {
		return nil
	}
	return &v
}

// customerFromPerson transforma a pessoa do cadastro no dest da NFCom. Endereço incompleto é erro claro: a emissão
// nunca inventa município ou CEP.
func customerFromPerson(sk string, person map[string]any) (NfcomCustomer, error) {
	addr := services.FirstAddress(person)
	uf := anyStr(addr, "state_federation")
	if uf == "" || uf == ufExterior || anyStr(addr, "street") == "" || anyStr(addr, "neighborhood") == "" ||
		anyStr(addr, "city_ibge_code") == "" || anyStr(addr, "city") == "" || len(onlyDigits(anyStr(addr, "postal_code"))) != 8 {
		return NfcomCustomer{}, problem.BadRequest("complete o endereço nacional da pessoa (logradouro, bairro, município, CEP e UF) antes de faturar")
	}
	clean := func(key string, max int) string { return truncate(services.CleanText(anyStr(addr, key)), max) }
	c := NfcomCustomer{
		Name: truncate(services.CleanText(anyStr(person, "name")), customerNameMax),
		Address: NfcomAddress{
			Street: clean("street", 60), Number: truncate(services.CleanText(strOrDefault(anyStr(addr, "number"), defaultAddrNumber)), 60),
			Neighborhood: clean("neighborhood", 60), IBGECode: anyStr(addr, "city_ibge_code"),
			City: clean("city", 60), PostalCode: onlyDigits(anyStr(addr, "postal_code")), UF: uf,
		},
	}
	if comp := clean("complement", 60); comp != "" {
		c.Address.Complement = &comp
	}
	if phone := onlyDigits(services.FirstPhone(person)); len(phone) >= 6 && len(phone) <= 14 {
		c.Address.Phone = &phone
	}
	switch {
	case strings.HasPrefix(sk, skPrefixCNPJ):
		c.CNPJ = ptr(strings.TrimPrefix(sk, skPrefixCNPJ))
	case strings.HasPrefix(sk, skPrefixCPF):
		c.CPF = ptr(strings.TrimPrefix(sk, skPrefixCPF))
	case strings.HasPrefix(sk, repositories.SKPrefixForeign):
		c.OtherID = ptr(strings.TrimPrefix(sk, repositories.SKPrefixForeign))
	default:
		return NfcomCustomer{}, problem.BadRequest("documento da pessoa do assinante não reconhecido")
	}
	// indIEDest: IE do estado do endereço (só dígitos) = contribuinte; marca de isento = 2; sem IE = não contribuinte.
	switch ie := services.IEForUF(person, uf); {
	case ie != "":
		c.IEIndicator, c.IE = nfcomIEContribuinte, &ie
	case registrationIsExempt(person, uf):
		c.IEIndicator, c.IE = nfcomIEIsento, ptr(ieIsentoLiteral)
	default:
		c.IEIndicator = ieNaoContribuinte
	}
	return c, nil
}

func registrationIsExempt(person map[string]any, uf string) bool {
	regs, _ := person["state_registrations"].([]any)
	for _, r := range regs {
		if rm, ok := r.(map[string]any); ok && rm["uf"] == uf && strings.EqualFold(strings.TrimSpace(anyStr(rm, "state_registration")), ieIsentoLiteral) {
			return true
		}
	}
	return false
}

func subscriberFromRecord(r map[string]any) NfcomSubscriber {
	return NfcomSubscriber{
		Code: anyStr(r, "code"), Type: anyStr(r, "type"), ServiceUsed: anyStr(r, "service_used"),
		ContractNumber: optStr(r, "contract_number"), ContractStart: optStr(r, "contract_start"), ContractEnd: optStr(r, "contract_end"),
		PrimaryTerminal: optStr(r, "primary_terminal"), PrimaryUF: optStr(r, "primary_uf"), PrimaryIBGE: optStr(r, "primary_ibge_code"),
	}
}

// itemFromPlan junta o plano (classificação, unidade, tributos, valor padrão) com a linha da fatura.
func itemFromPlan(plan map[string]any, line NfcomItemRequest) (NfcomItem, error) {
	unitValue := anyStr(plan, "unit_value")
	switch {
	case unitValue == "" && line.UnitValue == nil:
		return NfcomItem{}, problem.BadRequest("o plano não tem valor padrão: informe o valor unitário")
	case unitValue == "":
		unitValue = *line.UnitValue
	case line.UnitValue != nil:
		return NfcomItem{}, problem.BadRequest("o plano tem valor fixo: o valor unitário não pode ser trocado na fatura")
	}
	it := NfcomItem{
		Code: anyStr(plan, "code"), Description: anyStr(plan, "description"), ClassCode: anyStr(plan, "class_code"),
		CFOP: optStr(plan, "cfop"), Unit: anyStr(plan, "unit"),
		Quantity: line.Quantity, UnitValue: unitValue, Discount: line.Discount, OtherCharges: line.OtherCharges,
	}
	if icms, ok := plan["icms"].(map[string]any); ok {
		it.ICMS = NfcomICMS{CST: anyStr(icms, "cst"), Rate: optStr(icms, "rate")}
	}
	if pis, ok := plan["pis"].(map[string]any); ok {
		it.PIS = &NfcomPIS{CST: anyStr(pis, "cst"), Rate: anyStr(pis, "rate")}
	}
	if cofins, ok := plan["cofins"].(map[string]any); ok {
		it.COFINS = &NfcomCOFINS{CST: anyStr(cofins, "cst"), Rate: anyStr(cofins, "rate")}
	}
	return it, nil
}

func optStr(m map[string]any, key string) *string {
	if v := anyStr(m, key); v != "" {
		return &v
	}
	return nil
}

func ptr(s string) *string { return &s }

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n]))
	}
	return s
}
