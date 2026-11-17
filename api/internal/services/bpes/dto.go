package bpes

// Corpo da emissão do BP-e comum (modelo 63; rodoviário, aquaviário e ferroviário). Os nomes seguem o JSON do restante da
// API (inglês, snake_case); cada campo aponta para a tag do leiaute. Escolha fechada é `oneof` com os valores do XSD (a UI
// mostra seletores). O servidor calcula o que não entra aqui: vBP (soma dos componentes), vPgto, vTroco, base e valor do ICMS e
// a chave de acesso. O TM e o TA têm leiaute próprio e entram em outras tarefas do plano (docs/plans/2026-11-16-bpe.md).
// Fora desta versão: agência, substituição (`tpBPe` 3), IBS/CBS, ICMS 20 e partilha, e os eventos 110300 e 110301.

// Variantes do BP-e. Nesta versão só a comum está disponível.
const (
	VariantNormal = "normal"
	VariantTM     = "tm"
	VariantTA     = "ta"
)

// BpeEmitBody é o corpo de POST /bpes.
type BpeEmitBody struct {
	// Variant: normal (padrão), tm ou ta. TM e TA ainda não estão disponíveis.
	Variant string `json:"variant" validate:"omitempty,oneof=normal tm ta"`
	// Modal: rodoviario, aquaviario ou ferroviario (ide/modal 1, 3, 4).
	Modal string `json:"modal" validate:"required,oneof=rodoviario aquaviario ferroviario"`
	// Presence é indPres: 1 presencial, 2 internet, 3 teleatendimento, 4 entrega em domicílio, 5 presencial fora do estabelecimento, 9 outros.
	Presence    string         `json:"presence" validate:"required,oneof=1 2 3 4 5 9"`
	Origin      BpePlace       `json:"origin" validate:"required"`
	Destination BpePlace       `json:"destination" validate:"required"`
	Buyer       *BpeBuyer      `json:"buyer" validate:"omitempty"`
	Passage     BpePassage     `json:"passage" validate:"required"`
	Trips       []BpeTrip      `json:"trips" validate:"required,min=1,max=20,dive"`
	Components  []BpeComponent `json:"components" validate:"required,min=1,max=20,dive"`
	Discount    *string        `json:"discount" validate:"omitempty"`
	// DiscountType é tpDesconto (01 a 13 e 99); com desconto maior que zero é obrigatório, com a descrição.
	DiscountType   *string      `json:"discount_type" validate:"omitempty,oneof=01 02 03 04 05 06 07 08 09 10 11 12 13 99"`
	DiscountDesc   *string      `json:"discount_desc" validate:"omitempty,min=2,max=100"`
	DiscountCode   *string      `json:"discount_code" validate:"omitempty,max=20"`
	Payments       []BpePayment `json:"payments" validate:"required,min=1,max=10,dive"`
	ICMS           BpeICMS      `json:"icms" validate:"required"`
	AdditionalInfo *string      `json:"additional_info" validate:"omitempty,max=5000"`
	AuthorizedXML  []string     `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	// ContingencyReason é a justificativa (xJust) na contingência offline: 15 a 256 caracteres.
	ContingencyReason *string `json:"contingency_reason" validate:"omitempty,min=15,max=256"`
}

// BpePlace é a UF e o município (código IBGE) do início ou do fim da viagem.
type BpePlace struct {
	UF       string `json:"uf" validate:"required,uf"`
	IBGECode string `json:"ibge_code" validate:"required,ibge"`
}

// BpeBuyer é o comprador (grupo comp). Informe um entre cnpj, cpf e foreign_id.
type BpeBuyer struct {
	Name      string     `json:"name" validate:"required,min=2,max=60"`
	CNPJ      *string    `json:"cnpj" validate:"omitempty,cnpj"`
	CPF       *string    `json:"cpf" validate:"omitempty,cpf"`
	ForeignID *string    `json:"foreign_id" validate:"omitempty,min=2,max=20"`
	IE        *string    `json:"ie" validate:"omitempty,max=14"`
	Address   BpeAddress `json:"address" validate:"required"`
}

// BpeAddress é o endereço do comprador (enderComp).
type BpeAddress struct {
	Street       string  `json:"street" validate:"required,min=2,max=60"`
	Number       string  `json:"number" validate:"required,max=60"`
	Complement   *string `json:"complement" validate:"omitempty,max=60"`
	Neighborhood string  `json:"neighborhood" validate:"required,min=2,max=60"`
	IBGECode     string  `json:"ibge_code" validate:"required,ibge"`
	City         string  `json:"city" validate:"required,min=2,max=60"`
	PostalCode   *string `json:"postal_code" validate:"omitempty,len=8,numeric"`
	UF           string  `json:"uf" validate:"required,ufex"`
	Phone        *string `json:"phone" validate:"omitempty,min=6,max=14,numeric"`
	Email        *string `json:"email" validate:"omitempty,email,max=60"`
}

// BpePassage é o grupo infPassagem: locais, embarque, validade e passageiro.
type BpePassage struct {
	FromCode  string        `json:"from_code" validate:"required,min=1,max=7"`
	FromName  string        `json:"from_name" validate:"required,min=2,max=60"`
	ToCode    string        `json:"to_code" validate:"required,min=1,max=7"`
	ToName    string        `json:"to_name" validate:"required,min=2,max=60"`
	Boarding  string        `json:"boarding" validate:"required"`
	Validity  string        `json:"validity" validate:"required"`
	Passenger *BpePassenger `json:"passenger" validate:"omitempty"`
}

// BpePassenger é o passageiro (infPassageiro). tpDoc: 1 RG, 2 título de eleitor, 3 passaporte, 4 CNH, 5 outros.
type BpePassenger struct {
	Name      string  `json:"name" validate:"required,min=2,max=60"`
	CPF       *string `json:"cpf" validate:"omitempty,cpf"`
	DocType   string  `json:"doc_type" validate:"required,oneof=1 2 3 4 5"`
	DocNumber string  `json:"doc_number" validate:"required,min=1,max=20"`
	DocDesc   *string `json:"doc_desc" validate:"omitempty,max=100"`
	Birth     *string `json:"birth" validate:"omitempty,datetime=2006-01-02"`
	Phone     *string `json:"phone" validate:"omitempty,min=6,max=14,numeric"`
	Email     *string `json:"email" validate:"omitempty,email,max=60"`
}

// BpeTrip é um trecho (infViagem).
type BpeTrip struct {
	RouteCode string `json:"route_code" validate:"required,min=1,max=20"`
	RouteName string `json:"route_name" validate:"required,min=2,max=100"`
	// TripType é tpViagem: 00 convencional, 01 fretamento.
	TripType string `json:"trip_type" validate:"required,oneof=00 01"`
	// ServiceType é tpServ: 1 convencional com sanitário, 2 sem sanitário, 3 semileito, 4 leito com ar, 5 leito sem ar, 6 executivo, 7 semiurbano, 8 longitudinal, 9 travessia, 10 semileito executivo, 11 leito executivo.
	ServiceType string `json:"service_type" validate:"required,oneof=1 2 3 4 5 6 7 8 9 10 11"`
	// Accommodation é tpAcomodacao: 1 assento, 2 rede, 3 cabine, 4 outros, 5 ferroviário.
	Accommodation string `json:"accommodation" validate:"required,oneof=1 2 3 4 5"`
	// LegType é tpTrecho: 1 normal, 2 inicial, 3 conexão.
	LegType    string       `json:"leg_type" validate:"required,oneof=1 2 3"`
	Departure  string       `json:"departure" validate:"required"`
	Connection *string      `json:"connection" validate:"omitempty"`
	Prefix     *string      `json:"prefix" validate:"omitempty,max=20"`
	Seat       *string      `json:"seat" validate:"omitempty,max=3"`
	Platform   *string      `json:"platform" validate:"omitempty,max=20"`
	Crossing   *BpeCrossing `json:"crossing" validate:"omitempty"`
}

// BpeCrossing é a travessia (infTravessia): tipo e situação do veículo.
type BpeCrossing struct {
	VehicleType   string `json:"vehicle_type" validate:"required,len=2"`
	VehicleStatus string `json:"vehicle_status" validate:"required,oneof=1 2 3"`
}

// BpeComponent é um componente do valor (Comp). tpComp: 01 tarifa, 02 pedágio, 03 taxa de embarque, 04 seguro, 05 TMR, 06 SCT, 99 outros.
type BpeComponent struct {
	Type  string `json:"type" validate:"required,oneof=01 02 03 04 05 06 99"`
	Value string `json:"value" validate:"required"`
}

// BpePayment é uma forma de pagamento (pag). tPag: 01 dinheiro, 02 cheque, 03 cartão de crédito, 04 cartão de débito, 05 vale, 06 PIX, 99 outros.
type BpePayment struct {
	Type  string   `json:"type" validate:"required,oneof=01 02 03 04 05 06 99"`
	Desc  *string  `json:"desc" validate:"omitempty,max=60"`
	Doc   *string  `json:"doc" validate:"omitempty,max=20"`
	Value string   `json:"value" validate:"required"`
	Card  *BpeCard `json:"card" validate:"omitempty"`
}

// BpeCard são os dados do cartão (card). tpIntegra: 1 integrado ao sistema de automação, 2 não integrado.
type BpeCard struct {
	Integration   string  `json:"integration" validate:"required,oneof=1 2"`
	CNPJ          *string `json:"cnpj" validate:"omitempty,cnpj"`
	Brand         *string `json:"brand" validate:"omitempty,oneof=01 02 03 04 05 06 99"`
	Authorization *string `json:"authorization" validate:"omitempty,max=20"`
}

// BpeICMS escolhe a situação do ICMS: CST 00, 40, 41, 51, 90, "sn" (Simples Nacional: CST 90 e indSN 1). Rate (pICMS, %)
// vale em 00 e 90; o servidor calcula a base (valor pago) e o valor do ICMS.
type BpeICMS struct {
	CST  string  `json:"cst" validate:"required,oneof=00 40 41 51 90 sn"`
	Rate *string `json:"rate" validate:"omitempty"`
}
