package bpes

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/validation"
)

// Corpos de emissão por cadastro: a venda aponta a linha (e, se quiser, pessoas) por id e informa só o que varia: data e
// hora, poltrona, valores, pagamento e o passageiro. O servidor resolve os cadastros e monta o corpo completo
// (BpeEmitBody, BpeTAEmitBody ou BpeTMEmitBody), que segue o fluxo de sempre. O passageiro avulso (nome e documento
// direto no corpo) é o caminho principal; a organização pode cadastrar a pessoa e usá-la.

// BpeTicketRequest é o corpo de POST /bpes do bilhete comum (`variant` normal, o padrão) e do TA (`variant` ta).
type BpeTicketRequest struct {
	Variant  string `json:"variant" validate:"omitempty,oneof=normal ta"`
	Presence string `json:"presence" validate:"required,oneof=1 2 3 4 5 9"`
	// Trips: um trecho (comum) ou voo (TA) por linha do cadastro, com a data e a hora de cada um.
	Trips    []BpeTripRequest `json:"trips" validate:"required,min=1,max=20,dive"`
	Boarding string           `json:"boarding" validate:"required"`
	Validity string           `json:"validity" validate:"required"`
	// Passenger é o passageiro avulso; PassengerPersonID usa uma pessoa física do cadastro. Um dos dois, salvo no BP-e múltiplo.
	Passenger         *BpePassenger `json:"passenger" validate:"omitempty"`
	PassengerPersonID *string       `json:"passenger_person_id" validate:"omitempty,refcode"`
	BuyerPersonID     *string       `json:"buyer_person_id" validate:"omitempty,refcode"`
	AgencyPersonID    *string       `json:"agency_person_id" validate:"omitempty,refcode"`
	// Só no TA: tipo de compra, chave do múltiplo (vinculado) e número do bilhete na IATA.
	PurchaseType string  `json:"purchase_type" validate:"omitempty,oneof=normal multiple linked"`
	MultipleKey  *string `json:"multiple_key" validate:"omitempty,dfe_access_key=63"`
	IATANumber   *string `json:"iata_number" validate:"omitempty,refcode,max=20"`
	// Substitution marca o bilhete como substituto (tpBPe 3).
	Substitution *BpeSubstitution `json:"substitution" validate:"omitempty"`
	Components   []BpeComponent   `json:"components" validate:"omitempty,min=1,max=20,dive"`
	Discount     *string          `json:"discount" validate:"omitempty,money"`
	DiscountType *string          `json:"discount_type" validate:"omitempty,oneof=01 02 03 04 05 06 07 08 09 10 11 12 13 99"`
	DiscountDesc *string          `json:"discount_desc" validate:"omitempty,min=2,max=100,fiscaltext"`
	DiscountCode *string          `json:"discount_code" validate:"omitempty,refcode,max=20"`
	Payments     []BpePayment     `json:"payments" validate:"omitempty,min=1,max=10,dive"`
	// BoardPass é o conteúdo do PDF417 do DABPE, só no comum.
	BoardPass         *string  `json:"board_pass" validate:"omitempty,min=50,max=1000"`
	AdditionalInfo    *string  `json:"additional_info" validate:"omitempty,max=5000,fiscaltext"`
	AuthorizedXML     []string `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	ContingencyReason *string  `json:"contingency_reason" validate:"omitempty,min=15,max=256,fiscaltext"`
}

// BpeTripRequest é um trecho da venda: a linha e o que varia nela.
type BpeTripRequest struct {
	LineID string `json:"line_id" validate:"required,refcode"`
	// LegType é tpTrecho: 1 normal, 2 inicial, 3 conexão.
	LegType    string       `json:"leg_type" validate:"required,oneof=1 2 3"`
	Departure  string       `json:"departure" validate:"required"`
	Connection *string      `json:"connection" validate:"omitempty"`
	Seat       *string      `json:"seat" validate:"omitempty,number,max=3"`
	Platform   *string      `json:"platform" validate:"omitempty,refcode,max=20"`
	Crossing   *BpeCrossing `json:"crossing" validate:"omitempty"`
}

// BpeTMRequest é o corpo de POST /bpes com `variant` tm: grupos de linha com os itens de cada trecho.
type BpeTMRequest struct {
	Variant         string              `json:"variant" validate:"required,eq=tm"`
	Competence      string              `json:"competence" validate:"required,datetime=2006-01-02"`
	SupplyIndicator bool                `json:"supply_indicator"`
	Groups          []BpeTMGroupRequest `json:"groups" validate:"required,min=1,max=99,dive"`
	AdditionalInfo  *string             `json:"additional_info" validate:"omitempty,max=5000,fiscaltext"`
	AuthorizedXML   []string            `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	// ContingencyReason é a justificativa (xJust) na contingência offline.
	ContingencyReason *string `json:"contingency_reason" validate:"omitempty,min=15,max=256,fiscaltext"`
}

type BpeTMGroupRequest struct {
	LineID    string             `json:"line_id" validate:"required,refcode"`
	CounterID *int               `json:"counter_id" validate:"omitempty,gte=1,lte=990"`
	Items     []BpeTMItemRequest `json:"items" validate:"required,min=1,max=990,dive"`
}

// BpeTMItemRequest é a apuração de um trecho da linha: Leg é a posição (1, 2, ...) do trecho no cadastro da linha.
type BpeTMItemRequest struct {
	Leg          int              `json:"leg" validate:"required,gte=1,lte=990"`
	TripNumber   *int             `json:"trip_number" validate:"omitempty,gte=1,lte=990"`
	StartCounter *string          `json:"start_counter" validate:"omitempty,number,max=20"`
	EndCounter   *string          `json:"end_counter" validate:"omitempty,number,max=20"`
	Passengers   string           `json:"passengers" validate:"required,number,max=6"`
	Value        string           `json:"value" validate:"required,money"`
	Components   []BpeTMComponent `json:"components" validate:"required,min=1,max=20,dive"`
}

const (
	maxPersonName = 60
	docCPFOutros  = "5" // tpDoc 5 (outros): o CPF do passageiro cadastrado vai como documento com a descrição "CPF"
	cpfLabel      = "CPF"
)

func decodeItem(item map[string]types.AttributeValue) map[string]any {
	m := map[string]any{}
	_ = attributevalue.UnmarshalMap(item, &m) // item já veio do DynamoDB: decode para mapa genérico não falha
	return m
}

// lineOf lê a linha gravada (o cadastro grava o JSON do BpeLine).
func lineOf(item map[string]types.AttributeValue) (BpeLine, error) {
	raw, err := json.Marshal(decodeItem(item))
	if err != nil {
		return BpeLine{}, problem.InternalServer("linha ilegível").WithCause(err)
	}
	var l BpeLine
	if err := json.Unmarshal(raw, &l); err != nil {
		return BpeLine{}, problem.InternalServer("linha ilegível").WithCause(err)
	}
	return l, nil
}

func (s *BpeService) loadLine(ctx context.Context, orgPK, id string, variant string, n int) (BpeLine, error) {
	item, err := s.lineRepo.Get(ctx, orgPK, id)
	if err != nil {
		return BpeLine{}, err
	}
	if item == nil {
		return BpeLine{}, problem.NotFound("linha não encontrada no trecho " + strconv.Itoa(n))
	}
	l, err := lineOf(item)
	if err != nil {
		return BpeLine{}, err
	}
	if l.Variant != variant {
		return BpeLine{}, problem.BadRequest(fmt.Sprintf("a linha \"%s\" é do tipo %s, não %s", l.Name, l.Variant, variant))
	}
	return l, nil
}

func (s *BpeService) loadPerson(ctx context.Context, orgPK, sk, label string) (string, map[string]any, error) {
	item, err := s.personRepo.Get(ctx, orgPK, sk)
	if err != nil {
		return "", nil, err
	}
	if item == nil {
		return "", nil, problem.NotFound(label + " não encontrado(a) no cadastro de pessoas")
	}
	return sk, decodeItem(item), nil
}

// addressOf monta o endereço (comprador e agência) da pessoa; sem município, o cadastro está incompleto.
func addressOf(person map[string]any, label string) (BpeAddress, error) {
	a := services.FirstAddress(person)
	ibge := anyStr(a, "city_ibge_code")
	if anyStr(a, "street") == "" || anyStr(a, "neighborhood") == "" || ibge == "" || anyStr(a, "city") == "" || anyStr(a, "state_federation") == "" {
		return BpeAddress{}, problem.BadRequest("complete o endereço de " + label + " no cadastro de pessoas (logradouro, bairro, município e UF)")
	}
	clean := func(k string, max int) string { return cut(services.CleanText(anyStr(a, k)), max) }
	out := BpeAddress{
		Street: clean("street", 60), Number: cut(services.CleanText(strOrDefault(anyStr(a, "number"), "S/N")), 60),
		Neighborhood: clean("neighborhood", 60), IBGECode: ibge, City: clean("city", 60), UF: anyStr(a, "state_federation"),
	}
	if comp := clean("complement", 60); comp != "" {
		out.Complement = &comp
	}
	if cep := onlyDigits(anyStr(a, "postal_code")); len(cep) == 8 {
		out.PostalCode = &cep
	}
	if phone := onlyDigits(services.FirstPhone(person)); len(phone) >= 6 && len(phone) <= 14 {
		out.Phone = &phone
	}
	return out, nil
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n]))
	}
	return s
}

func ptr(s string) *string { return &s }

func cleanPtr(p *string) *string {
	if p == nil {
		return nil
	}
	if v := services.CleanText(*p); v != "" {
		return &v
	}
	return nil
}

// buyerOf transforma a pessoa em comprador (comp).
func buyerOf(sk string, person map[string]any) (*BpeBuyer, error) {
	addr, err := addressOf(person, "o comprador")
	if err != nil {
		return nil, err
	}
	b := &BpeBuyer{Name: cut(services.CleanText(anyStr(person, "name")), maxPersonName), Address: addr}
	switch {
	case strings.HasPrefix(sk, "CNPJ_"):
		b.CNPJ = ptr(strings.TrimPrefix(sk, "CNPJ_"))
	case strings.HasPrefix(sk, "CPF_"):
		b.CPF = ptr(strings.TrimPrefix(sk, "CPF_"))
	case strings.HasPrefix(sk, repositories.SKPrefixForeign):
		b.ForeignID = ptr(strings.TrimPrefix(sk, repositories.SKPrefixForeign))
	default:
		return nil, problem.BadRequest("documento do comprador não reconhecido")
	}
	if ie := services.IEForUF(person, addr.UF); ie != "" {
		b.IE = &ie
	}
	return b, nil
}

// agencyOf transforma a pessoa jurídica em agência.
func agencyOf(sk string, person map[string]any) (*BpeAgency, error) {
	if !strings.HasPrefix(sk, "CNPJ_") {
		return nil, problem.BadRequest("a agência deve ser uma pessoa jurídica (CNPJ)")
	}
	addr, err := addressOf(person, "a agência")
	if err != nil {
		return nil, err
	}
	return &BpeAgency{Name: cut(services.CleanText(anyStr(person, "name")), maxPersonName), CNPJ: strings.TrimPrefix(sk, "CNPJ_"), Address: addr}, nil
}

// passengerOf transforma a pessoa física em passageiro: o CPF vai como documento "outros" (tpDoc 5) com a descrição CPF.
func passengerOf(sk string, person map[string]any) (*BpePassenger, error) {
	if !strings.HasPrefix(sk, "CPF_") {
		return nil, problem.BadRequest("o passageiro cadastrado deve ser uma pessoa física (CPF)")
	}
	cpf := strings.TrimPrefix(sk, "CPF_")
	p := &BpePassenger{Name: cut(services.CleanText(anyStr(person, "name")), maxPersonName), CPF: &cpf, DocType: docCPFOutros, DocNumber: cpf, DocDesc: ptr(cpfLabel)}
	if phone := onlyDigits(services.FirstPhone(person)); len(phone) >= 6 && len(phone) <= 14 {
		p.Phone = &phone
	}
	if email := services.FirstEmail(person); email != "" && len(email) <= 60 {
		p.Email = &email
	}
	return p, nil
}

// cleanPassenger normaliza o passageiro avulso (texto livre) antes da validação final.
func cleanPassenger(p *BpePassenger) *BpePassenger {
	if p == nil {
		return nil
	}
	out := *p
	out.Name = services.CleanText(p.Name)
	out.DocNumber = services.CleanText(p.DocNumber)
	out.DocDesc = cleanPtr(p.DocDesc)
	return &out
}

// ticketParties resolve comprador, agência e passageiro (cadastrado ou avulso).
func (s *BpeService) ticketParties(ctx context.Context, orgPK string, req BpeTicketRequest) (buyer *BpeBuyer, agency *BpeAgency, passenger *BpePassenger, err error) {
	passenger = cleanPassenger(req.Passenger)
	if req.PassengerPersonID != nil {
		if passenger != nil {
			return nil, nil, nil, problem.BadRequest("informe o passageiro avulso ou o cadastrado, não os dois")
		}
		sk, person, err := s.loadPerson(ctx, orgPK, *req.PassengerPersonID, "passageiro")
		if err != nil {
			return nil, nil, nil, err
		}
		if passenger, err = passengerOf(sk, person); err != nil {
			return nil, nil, nil, err
		}
	}
	if req.BuyerPersonID != nil {
		sk, person, err := s.loadPerson(ctx, orgPK, *req.BuyerPersonID, "comprador")
		if err != nil {
			return nil, nil, nil, err
		}
		if buyer, err = buyerOf(sk, person); err != nil {
			return nil, nil, nil, err
		}
	}
	if req.AgencyPersonID != nil {
		sk, person, err := s.loadPerson(ctx, orgPK, *req.AgencyPersonID, "agência")
		if err != nil {
			return nil, nil, nil, err
		}
		if agency, err = agencyOf(sk, person); err != nil {
			return nil, nil, nil, err
		}
	}
	return buyer, agency, passenger, nil
}

// Resolve monta o corpo completo do bilhete comum ou do TA a partir da venda e dos cadastros. Devolve a variante e o corpo
// correspondente (o outro vem zerado).
func (s *BpeService) ResolveTicket(ctx context.Context, orgPK string, req BpeTicketRequest) (string, BpeEmitBody, BpeTAEmitBody, error) {
	variant := req.Variant
	if variant == "" {
		variant = VariantNormal
	}
	lines := make([]BpeLine, 0, len(req.Trips))
	for i, t := range req.Trips {
		l, err := s.loadLine(ctx, orgPK, t.LineID, variant, i+1)
		if err != nil {
			return "", BpeEmitBody{}, BpeTAEmitBody{}, err
		}
		lines = append(lines, l)
	}
	buyer, agency, passenger, err := s.ticketParties(ctx, orgPK, req)
	if err != nil {
		return "", BpeEmitBody{}, BpeTAEmitBody{}, err
	}
	first, last := lines[0], lines[len(lines)-1]
	if first.Origin == nil || last.Destination == nil {
		return "", BpeEmitBody{}, BpeTAEmitBody{}, problem.BadRequest("a linha não tem origem ou destino")
	}
	origin := BpePlace{UF: first.Origin.UF, IBGECode: first.Origin.IBGECode}
	destination := BpePlace{UF: last.Destination.UF, IBGECode: last.Destination.IBGECode}
	info, discountDesc := cleanPtr(req.AdditionalInfo), cleanPtr(req.DiscountDesc)

	if variant == VariantTA {
		trips := make([]BpeTATrip, 0, len(lines))
		for i, l := range lines {
			t := req.Trips[i]
			trips = append(trips, BpeTATrip{
				FlightNumber: deref(l.FlightNumber), Carrier: deref(l.Carrier), TripType: deref(l.TripType),
				FromAirport: deref(l.FromAirport), ToAirport: deref(l.ToAirport), ServiceType: deref(l.ServiceType),
				Accommodation: deref(l.Accommodation), LegType: t.LegType, Departure: t.Departure, Connection: t.Connection,
			})
		}
		comps := make([]BpeTAComponent, 0, len(req.Components))
		for _, c := range req.Components {
			comps = append(comps, BpeTAComponent(c))
		}
		body := BpeTAEmitBody{
			Variant: VariantTA, Presence: req.Presence, Origin: origin, Destination: destination, Buyer: buyer,
			Passage: BpeTAPassage{Boarding: req.Boarding, Validity: req.Validity, Passenger: passenger}, Trips: trips,
			PurchaseType: req.PurchaseType, MultipleKey: req.MultipleKey, Substitution: req.Substitution, Agency: agency,
			Components: comps, Discount: req.Discount, DiscountType: req.DiscountType, DiscountDesc: discountDesc, DiscountCode: req.DiscountCode,
			Payments: req.Payments, IATANumber: req.IATANumber, AdditionalInfo: info, AuthorizedXML: req.AuthorizedXML,
			ContingencyReason: cleanPtr(req.ContingencyReason),
		}
		if first.Tax != nil {
			body.Tax = *first.Tax
		}
		if p := validation.Struct(body); p != nil {
			return "", BpeEmitBody{}, BpeTAEmitBody{}, p
		}
		return variant, BpeEmitBody{}, body, nil
	}

	modal := deref(first.Modal)
	trips := make([]BpeTrip, 0, len(lines))
	for i, l := range lines {
		if deref(l.Modal) != modal {
			return "", BpeEmitBody{}, BpeTAEmitBody{}, problem.BadRequest("as linhas do bilhete devem ter o mesmo modal")
		}
		t := req.Trips[i]
		trips = append(trips, BpeTrip{
			RouteCode: deref(l.RouteCode), RouteName: deref(l.RouteName), TripType: deref(l.TripType), ServiceType: deref(l.ServiceType),
			Accommodation: deref(l.Accommodation), LegType: t.LegType, Departure: t.Departure, Connection: t.Connection,
			Prefix: l.Prefix, Seat: t.Seat, Platform: t.Platform, Crossing: t.Crossing,
		})
	}
	body := BpeEmitBody{
		Variant: VariantNormal, Modal: modal, Presence: req.Presence, Origin: origin, Destination: destination, Buyer: buyer,
		Passage: BpePassage{
			FromCode: first.Origin.IBGECode, FromName: first.Origin.City, ToCode: last.Destination.IBGECode, ToName: last.Destination.City,
			Boarding: req.Boarding, Validity: req.Validity, Passenger: passenger,
		},
		Trips: trips, Components: req.Components, Discount: req.Discount, DiscountType: req.DiscountType, DiscountDesc: discountDesc,
		DiscountCode: req.DiscountCode, Payments: req.Payments, Substitution: req.Substitution, Agency: agency,
		BoardPass: req.BoardPass, AdditionalInfo: info, AuthorizedXML: req.AuthorizedXML, ContingencyReason: cleanPtr(req.ContingencyReason),
	}
	if first.ICMS != nil {
		body.ICMS = *first.ICMS
	}
	if p := validation.Struct(body); p != nil {
		return "", BpeEmitBody{}, BpeTAEmitBody{}, p
	}
	return variant, body, BpeTAEmitBody{}, nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ResolveTM monta o corpo completo do TM a partir dos grupos (linha + apuração por trecho).
func (s *BpeService) ResolveTM(ctx context.Context, orgPK string, req BpeTMRequest) (BpeTMEmitBody, error) {
	body := BpeTMEmitBody{
		Variant: VariantTM, Competence: req.Competence, SupplyIndicator: req.SupplyIndicator,
		AdditionalInfo: cleanPtr(req.AdditionalInfo), AuthorizedXML: req.AuthorizedXML, ContingencyReason: cleanPtr(req.ContingencyReason),
	}
	for gi, g := range req.Groups {
		line, err := s.loadLine(ctx, orgPK, g.LineID, VariantTM, gi+1)
		if err != nil {
			return BpeTMEmitBody{}, err
		}
		if gi == 0 {
			body.Modal, body.CFOP = deref(line.Modal), deref(line.CFOP)
		} else if deref(line.Modal) != body.Modal || deref(line.CFOP) != body.CFOP {
			return BpeTMEmitBody{}, problem.BadRequest("as linhas do TM devem ter o mesmo modal e o mesmo CFOP")
		}
		group := BpeTMGroup{CounterID: g.CounterID, OriginUF: deref(line.OriginUF), DestUF: line.DestUF, Plate: line.Plate, Prefix: line.Prefix}
		for ii, it := range g.Items {
			if it.Leg > len(line.Legs) {
				return BpeTMEmitBody{}, problem.BadRequest(fmt.Sprintf("grupo %d, item %d: a linha \"%s\" não tem o trecho %d", gi+1, ii+1, line.Name, it.Leg))
			}
			leg := line.Legs[it.Leg-1]
			for _, c := range it.Components {
				if !contains(line.PassengerTypes, c.Name) {
					return BpeTMEmitBody{}, problem.BadRequest(fmt.Sprintf("grupo %d, item %d: \"%s\" não é um tipo de passageiro da linha", gi+1, ii+1, c.Name))
				}
			}
			group.Items = append(group.Items, BpeTMItem{
				TripNumber: it.TripNumber, OriginIBGE: leg.OriginIBGE, DestIBGE: leg.DestIBGE, StartCounter: it.StartCounter,
				EndCounter: it.EndCounter, Passengers: it.Passengers, Value: it.Value, Components: it.Components, ICMS: leg.ICMS,
			})
		}
		body.Groups = append(body.Groups, group)
	}
	if p := validation.Struct(body); p != nil {
		return BpeTMEmitBody{}, p
	}
	return body, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
