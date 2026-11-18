package nfcoms

// Corpo da emissão da NFCom (modelo 62). Os nomes seguem o JSON do restante da API (inglês, snake_case); cada campo
// aponta para a tag do leiaute. Campos de escolha fechada usam oneof com os valores do XSD (a UI mostra seletores,
// nunca texto livre). O que o servidor calcula não entra aqui: vProd do item, totais, tributos e a chave de acesso.
// Fora da v1 (docs/specs/2026-11-15-nfcom-design.md): substituição, ajuste e cofaturamento (finNFCom 3 e 4, tpFat 2),
// faturamento centralizado, IBS/CBS, FUST, FUNTTEL, retenções e a partilha do ICMS.

// NfcomEmitBody é o corpo de POST /nfcoms.
type NfcomEmitBody struct {
	Customer   NfcomCustomer   `json:"customer" validate:"required"`
	Subscriber NfcomSubscriber `json:"subscriber" validate:"required"`
	Items      []NfcomItem     `json:"items" validate:"required,min=1,max=990,dive"`
	// Invoice é o grupo gFat: obrigatório no faturamento normal, salvo serviço pré-pago.
	Invoice *NfcomInvoice `json:"invoice" validate:"omitempty"`
	// Prepaid marca o serviço pré-pago (ide/indPrePago).
	Prepaid        bool     `json:"prepaid"`
	AdditionalInfo *string  `json:"additional_info" validate:"omitempty,max=3000"`
	AuthorizedXML  []string `json:"authorized_xml" validate:"omitempty,max=10,dive,cpfcnpj"`
	// ContingencyReason é a justificativa (xJust) quando a empresa está em contingência offline: 15 a 256 caracteres.
	ContingencyReason *string `json:"contingency_reason" validate:"omitempty,min=15,max=256"`
}

// NfcomCustomer é o destinatário (tomador): grupo dest.
type NfcomCustomer struct {
	Name string  `json:"name" validate:"required,min=2,max=60"`
	CNPJ *string `json:"cnpj" validate:"omitempty,cnpj"`
	CPF  *string `json:"cpf" validate:"omitempty,cpf"`
	// OtherID é o idOutros: documento de quem não tem CNPJ nem CPF.
	OtherID *string `json:"other_id" validate:"omitempty,min=1,max=20"`
	// IEIndicator é indIEDest: 1 contribuinte, 2 isento, 9 não contribuinte.
	IEIndicator           string       `json:"ie_indicator" validate:"required,oneof=1 2 9"`
	IE                    *string      `json:"ie" validate:"omitempty,max=14"`
	MunicipalRegistration *string      `json:"municipal_registration" validate:"omitempty,max=15"`
	Address               NfcomAddress `json:"address" validate:"required"`
}

// NfcomAddress é o endereço do destinatário (enderDest).
type NfcomAddress struct {
	Street       string  `json:"street" validate:"required,min=2,max=60"`
	Number       string  `json:"number" validate:"required,max=60"`
	Complement   *string `json:"complement" validate:"omitempty,max=60"`
	Neighborhood string  `json:"neighborhood" validate:"required,min=2,max=60"`
	IBGECode     string  `json:"ibge_code" validate:"required,ibge"`
	City         string  `json:"city" validate:"required,min=2,max=60"`
	PostalCode   string  `json:"postal_code" validate:"required,len=8,numeric"`
	UF           string  `json:"uf" validate:"required,uf"`
	Phone        *string `json:"phone" validate:"omitempty,min=6,max=14,numeric"`
	Email        *string `json:"email" validate:"omitempty,email,max=60"`
}

// NfcomSubscriber é o assinante: grupo assinante.
type NfcomSubscriber struct {
	Code string `json:"code" validate:"required,min=1,max=30"`
	// Type é tpAssinante: 1 comercial, 2 industrial, 3 residencial, 4 produtor rural, 5 órgão público estadual,
	// 6 prestador de telecomunicação, 7 órgão público municipal, 8 órgão público federal, 99 outros.
	Type string `json:"type" validate:"required,oneof=1 2 3 4 5 6 7 8 99"`
	// ServiceUsed é tpServUtil: 1 telefonia, 2 dados, 3 TV por assinatura, 4 provimento de acesso à internet,
	// 5 multimídia, 6 outros, 7 vários.
	ServiceUsed    string  `json:"service_used" validate:"required,oneof=1 2 3 4 5 6 7"`
	ContractNumber *string `json:"contract_number" validate:"omitempty,min=1,max=20"`
	// ContractStart e ContractEnd são datas AAAA-MM-DD (dContratoIni e dContratoFim).
	ContractStart   *string `json:"contract_start" validate:"omitempty,datetime=2006-01-02"`
	ContractEnd     *string `json:"contract_end" validate:"omitempty,datetime=2006-01-02"`
	PrimaryTerminal *string `json:"primary_terminal" validate:"omitempty,min=1,max=12"`
	PrimaryUF       *string `json:"primary_uf" validate:"omitempty,uf"`
	PrimaryIBGE     *string `json:"primary_ibge_code" validate:"omitempty,ibge"`
}

// NfcomItem é um item (det): produto/serviço e tributos.
type NfcomItem struct {
	Code        string `json:"code" validate:"required,min=1,max=60"`
	Description string `json:"description" validate:"required,min=1,max=120"`
	// ClassCode é o cClass: classificação do item, 7 dígitos (tabela do MOC, item 7.8).
	ClassCode string `json:"class_code" validate:"required,len=7,numeric"`
	// CFOP é obrigatório, salvo quando o ICMS é "sem CST".
	CFOP *string `json:"cfop" validate:"omitempty,len=4,numeric"`
	// Unit é uMed: 1 minuto, 2 MB, 3 GB, 4 UN.
	Unit         string       `json:"unit" validate:"required,oneof=1 2 3 4"`
	Quantity     string       `json:"quantity" validate:"required"`
	UnitValue    string       `json:"unit_value" validate:"required"`
	Discount     *string      `json:"discount" validate:"omitempty"`
	OtherCharges *string      `json:"other_charges" validate:"omitempty"`
	ICMS         NfcomICMS    `json:"icms" validate:"required"`
	PIS          *NfcomPIS    `json:"pis" validate:"omitempty"`
	COFINS       *NfcomCOFINS `json:"cofins" validate:"omitempty"`
}

// NfcomICMS escolhe a situação do ICMS do item. CST: 00, 40, 41, 51, 90, "sn" (Simples Nacional: CST 90 e indSN 1)
// ou "sem_cst" (indSemCST: item sem situação tributária, sem CFOP). Rate (pICMS, %) só vale em 00 e 90; o servidor
// calcula a base (valor do item) e o valor do ICMS.
type NfcomICMS struct {
	CST  string  `json:"cst" validate:"required,oneof=00 40 41 51 90 sn sem_cst"`
	Rate *string `json:"rate" validate:"omitempty,percent"`
}

// NfcomPIS e NfcomCOFINS: CST do leiaute e alíquota em %; base = valor do item, valor calculado pelo servidor.
type NfcomPIS struct {
	CST  string `json:"cst" validate:"required,oneof=01 02 06 07 08 09 49"`
	Rate string `json:"rate" validate:"required,percent"`
}

type NfcomCOFINS struct {
	CST  string `json:"cst" validate:"required,oneof=01 02 06 07 08 09 49"`
	Rate string `json:"rate" validate:"required,percent"`
}

// NfcomInvoice é a fatura (gFat).
type NfcomInvoice struct {
	// Competence é CompetFat, AAAAMM.
	Competence string `json:"competence" validate:"required,len=6,numeric"`
	// DueDate é dVencFat, AAAA-MM-DD.
	DueDate string `json:"due_date" validate:"required,datetime=2006-01-02"`
	// BarCode é codBarras: 1 a 48 dígitos.
	BarCode string `json:"bar_code" validate:"required,min=1,max=48,numeric"`
}
