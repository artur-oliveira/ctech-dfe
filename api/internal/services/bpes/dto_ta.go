package bpes

// Corpo da emissão do BP-e TA (transporte aéreo, modal 2; NT 2025.002 v2.00). Mesma ideia do bilhete comum, com a viagem
// por voo (companhia, número, aeroportos) e sem poltrona, plataforma, prefixo nem percurso. O emitente precisa de
// credenciamento específico para o TA. Nesta versão só a compra normal (`tpCompra` 0): BP-e múltiplo e vinculado a múltiplo,
// substituição, agência, o evento de ajuste 112130 e a vinculação de pagamento ficam fora.

// BpeTAEmitBody é o corpo de POST /bpes com `variant` igual a `ta`.
type BpeTAEmitBody struct {
	Variant string `json:"variant" validate:"required,eq=ta"`
	// Presence é indPres: 1 presencial, 2 internet, 3 teleatendimento, 4 entrega em domicílio, 5 presencial fora do estabelecimento, 9 outros.
	Presence    string           `json:"presence" validate:"required,oneof=1 2 3 4 5 9"`
	Origin      BpePlace         `json:"origin" validate:"required"`
	Destination BpePlace         `json:"destination" validate:"required"`
	Buyer       *BpeBuyer        `json:"buyer" validate:"omitempty"`
	Passage     BpeTAPassage     `json:"passage" validate:"required"`
	Trips       []BpeTATrip      `json:"trips" validate:"required,min=1,max=20,dive"`
	Components  []BpeTAComponent `json:"components" validate:"required,min=1,max=20,dive"`
	Discount    *string          `json:"discount" validate:"omitempty"`
	// DiscountType é tpDesconto do TA: 03, 09, 10, 11, 12, 13 e 99.
	DiscountType *string      `json:"discount_type" validate:"omitempty,oneof=03 09 10 11 12 13 99"`
	DiscountDesc *string      `json:"discount_desc" validate:"omitempty,min=2,max=100"`
	DiscountCode *string      `json:"discount_code" validate:"omitempty,max=20"`
	Payments     []BpePayment `json:"payments" validate:"required,min=1,max=10,dive"`
	// Tax é o IBS/CBS (o TA não tem ICMS): CST e classificação tributária da tabela do MOC, e as alíquotas quando houver tributo.
	Tax BpeTATax `json:"tax" validate:"required"`
	// IATANumber é nroBPIATA, o número do bilhete na IATA.
	IATANumber        *string  `json:"iata_number" validate:"omitempty,min=1,max=20"`
	AdditionalInfo    *string  `json:"additional_info" validate:"omitempty,max=5000"`
	AuthorizedXML     []string `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	ContingencyReason *string  `json:"contingency_reason" validate:"omitempty,min=15,max=256"`
}

// BpeTAPassage é o infPassagem do TA: embarque, validade e passageiro (sem locais de origem e destino).
type BpeTAPassage struct {
	Boarding  string        `json:"boarding" validate:"required"`
	Validity  string        `json:"validity" validate:"required"`
	Passenger *BpePassenger `json:"passenger" validate:"omitempty"`
}

// BpeTATrip é um voo (infViagem do TA).
type BpeTATrip struct {
	FlightNumber string `json:"flight_number" validate:"required,min=1,max=20"`
	// Carrier é SiglaCiaOperVoo: sigla da companhia que opera o voo (até 3 caracteres).
	Carrier string `json:"carrier" validate:"required,min=2,max=3"`
	// TripType é tpViagem do TA: 00 convencional, 01 fretamento, 02 e 03 (ver leiaute).
	TripType    string `json:"trip_type" validate:"required,oneof=00 01 02 03"`
	FromAirport string `json:"from_airport" validate:"required,min=3,max=4"`
	ToAirport   string `json:"to_airport" validate:"required,min=3,max=4"`
	// ServiceType é tpServ do TA: 12 e 13.
	ServiceType string `json:"service_type" validate:"required,oneof=12 13"`
	// Accommodation é tpAcomodacao do TA: 6 e 7.
	Accommodation string `json:"accommodation" validate:"required,oneof=6 7"`
	// LegType é tpTrecho: 1 normal, 2 inicial, 3 conexão.
	LegType    string  `json:"leg_type" validate:"required,oneof=1 2 3"`
	Departure  string  `json:"departure" validate:"required"`
	Connection *string `json:"connection" validate:"omitempty"`
}

// common reorganiza o corpo do TA como o do bilhete comum, para reaproveitar a validação e os grupos de valor, imposto e
// pagamento, que são iguais nas duas variantes.
func (b BpeTAEmitBody) common() BpeEmitBody {
	trips := make([]BpeTrip, 0, len(b.Trips))
	for _, t := range b.Trips {
		trips = append(trips, BpeTrip{Departure: t.Departure, Connection: t.Connection, LegType: t.LegType})
	}
	comps := make([]BpeComponent, 0, len(b.Components))
	for _, c := range b.Components {
		comps = append(comps, BpeComponent(c))
	}
	return BpeEmitBody{
		Presence: b.Presence, Origin: b.Origin, Destination: b.Destination, Buyer: b.Buyer,
		Passage:    BpePassage{Boarding: b.Passage.Boarding, Validity: b.Passage.Validity, Passenger: b.Passage.Passenger},
		Trips:      trips,
		Components: comps, Discount: b.Discount, DiscountType: b.DiscountType, DiscountDesc: b.DiscountDesc,
		DiscountCode: b.DiscountCode, Payments: b.Payments, ICMS: BpeICMS{CST: "40"},
		AdditionalInfo: b.AdditionalInfo, AuthorizedXML: b.AuthorizedXML, ContingencyReason: b.ContingencyReason,
	}
}
