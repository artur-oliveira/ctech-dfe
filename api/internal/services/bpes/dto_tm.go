package bpes

// Corpo da emissão do BP-e TM (transporte metropolitano, `tpBPe` 4; NT 2020.002). O TM é outro documento: não tem
// passageiro, viagem nem pagamento; é uma apuração por grupo de viagem (linha, veículo, UF) com itens por trecho e
// componentes de tarifa, e o `total` sai da soma dos itens. O emitente precisa de credenciamento específico para o TM.
// Fora desta versão: IBS/CBS, ICMS 20, vBPLiq, vinculação de pagamento e informações adicionais por item.

// BpeTMEmitBody é o corpo de POST /bpes com `variant` igual a `tm`.
type BpeTMEmitBody struct {
	Variant string `json:"variant" validate:"required,eq=tm"`
	// Modal: rodoviario, aquaviario ou ferroviario (ide/modal 1, 3, 4).
	Modal string `json:"modal" validate:"required,oneof=rodoviario aquaviario ferroviario"`
	// CFOP da operação (ide/CFOP): primeiro dígito 1, 2, 3, 5, 6 ou 7, e os dois últimos nunca 00.
	CFOP string `json:"cfop" validate:"required,cfop_cte"`
	// Competence é dCompet, AAAA-MM-DD.
	Competence string `json:"competence" validate:"required,datetime=2006-01-02"`
	// SupplyIndicator é indFornecimento (único valor aceito: 1), opcional.
	SupplyIndicator bool         `json:"supply_indicator"`
	Groups          []BpeTMGroup `json:"groups" validate:"required,min=1,max=99,dive"`
	AdditionalInfo  *string      `json:"additional_info" validate:"omitempty,max=5000"`
	AuthorizedXML   []string     `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	// ContingencyReason é a justificativa (xJust) na contingência offline: 15 a 256 caracteres.
	ContingencyReason *string `json:"contingency_reason" validate:"omitempty,min=15,max=256"`
}

// BpeTMGroup é um grupo de viagem (detBPeTM): UF de início, UF de fim, placa e prefixo do veículo.
type BpeTMGroup struct {
	// CounterID é o atributo idEqpCont (identificador do equipamento contador, 1 a 990); vazio usa a posição do grupo.
	CounterID *int        `json:"counter_id" validate:"omitempty,gte=1,lte=990"`
	OriginUF  string      `json:"origin_uf" validate:"required,uf"`
	DestUF    *string     `json:"dest_uf" validate:"omitempty,ufex"`
	Plate     *string     `json:"plate" validate:"omitempty,placa"`
	Prefix    *string     `json:"prefix" validate:"omitempty,max=20"`
	Items     []BpeTMItem `json:"items" validate:"required,min=1,max=990,dive"`
}

// BpeTMItem é um item do grupo (det): trecho, contadores da catraca, passageiros, valor, componentes e ICMS.
type BpeTMItem struct {
	// TripNumber é o atributo nViagem (número da viagem, 1 a 990); vazio usa a posição do item no grupo.
	TripNumber   *int    `json:"trip_number" validate:"omitempty,gte=1,lte=990"`
	OriginIBGE   string  `json:"origin_ibge_code" validate:"required,ibge"`
	DestIBGE     *string `json:"dest_ibge_code" validate:"omitempty,ibge"`
	StartCounter *string `json:"start_counter" validate:"omitempty,numeric,max=20"`
	EndCounter   *string `json:"end_counter" validate:"omitempty,numeric,max=20"`
	// Passengers é qPass: inteiro de 1 a 6 dígitos.
	Passengers string `json:"passengers" validate:"required"`
	// Value é vBP: valor do item, decimal não negativo.
	Value      string           `json:"value" validate:"required"`
	Components []BpeTMComponent `json:"components" validate:"required,min=1,max=20,dive"`
	// ICMS: base = valor do item.
	ICMS BpeICMS `json:"icms" validate:"required"`
}

// BpeTMComponent é um componente de tarifa (Comp): nome e quantidade de passageiros (qComp, até 5 dígitos).
type BpeTMComponent struct {
	Name     string `json:"name" validate:"required,min=1,max=15"`
	Quantity int    `json:"quantity" validate:"gte=0,lte=99999"`
}
