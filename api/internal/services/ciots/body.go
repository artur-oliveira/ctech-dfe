package ciots

import (
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

// Request bodies. Field names are English snake_case like the rest of the API; the mapping to the ANTT names
// is toDeclaracao. Presence and format rules are NOT repeated as validate tags: ciot.Validate owns them and
// answers with the stable internal code (a missing mandatory field is 422, not 400).

type Vehicle struct {
	Plate     string `json:"plate"`
	RNTRC     string `json:"rntrc"`
	Axles     int    `json:"axles"`
	Automotor bool   `json:"automotor"`
	Tractor   bool   `json:"tractor"` // cavalo-trator (B117)
}

type Route struct {
	OriginIBGE      string `json:"origin_ibge"`
	DestinationIBGE string `json:"destination_ibge"`
	OriginCEP       string `json:"origin_cep"`
	DestinationCEP  string `json:"destination_cep"`
	OriginLat       string `json:"origin_latitude"`
	OriginLng       string `json:"origin_longitude"`
	DestinationLat  string `json:"destination_latitude"`
	DestinationLng  string `json:"destination_longitude"`
	DistanceKm      int    `json:"distance_km"`
}

type Cargo struct {
	Nature              string   `json:"nature"` // 4 first NCM digits
	Weight              string   `json:"weight"`
	CargoType           int      `json:"cargo_type"`
	FractionalHirerDocs []string `json:"fractional_hirer_docs"`
}

type Installment struct {
	Number  int    `json:"number"`
	DueDate string `json:"due_date"`
	Value   string `json:"value"`
}

type Payment struct {
	Type         int           `json:"type"`      // 1..6
	Indicator    int           `json:"indicator"` // 0 cash, 1 installments
	BankCode     string        `json:"bank_code"`
	Branch       string        `json:"branch"`
	Account      string        `json:"account"`
	PixKey       string        `json:"pix_key"`
	PixID        string        `json:"pix_id"`
	CreditedDoc  string        `json:"credited_doc"`
	Installments []Installment `json:"installments"`
}

type Indicators struct {
	HighPerformance  bool `json:"high_performance"`
	EmptyReturn      bool `json:"empty_return"`
	VehicleComposite bool `json:"vehicle_composition"`
}

// Operation is the declaration content, shared by declare and rectify.
type Operation struct {
	OperationType     int         `json:"operation_type"` // 1 full load, 2 fractional, 3 TAC-Agregado
	CarrierDoc        string      `json:"carrier_doc"`    // CpfCnpjContratado
	CarrierRNTRC      string      `json:"carrier_rntrc"`
	HirerDoc          string      `json:"hirer_doc"` // CpfCnpjContratante
	HirerRNTRC        string      `json:"hirer_rntrc"`
	RecipientDoc      string      `json:"recipient_doc"`
	FreightValue      string      `json:"freight_value"`
	DeclaredAt        *string     `json:"declared_at" validate:"omitempty,datetime=2006-01-02T15:04:05Z07:00"`
	Contingency       bool        `json:"contingency"`
	ContingencyReason string      `json:"contingency_reason" validate:"max=255"`
	TripStart         string      `json:"trip_start" validate:"omitempty,datetime=2006-01-02"`
	TripEnd           string      `json:"trip_end" validate:"omitempty,datetime=2006-01-02"`
	Vehicles          []Vehicle   `json:"vehicles" validate:"max=5"`
	Routes            []Route     `json:"routes" validate:"max=50"`
	Cargo             *Cargo      `json:"cargo"`
	Payment           Payment     `json:"payment"`
	Indicators        *Indicators `json:"indicators"`
}

// DeclareBody is POST /ciots.
type DeclareBody struct {
	Environment string `json:"environment" validate:"required,oneof=prod hom"`
	// PaymentInstitution: the certificate is a payment institution's (skips the carrier match, B112).
	PaymentInstitution bool    `json:"payment_institution"`
	MdfeAccessKey      *string `json:"mdfe_access_key" validate:"omitempty,len=44,numeric"`
	Operation
}

// RectifyBody is POST /ciots/{id}/rectification: the full corrected declaration.
type RectifyBody struct{ Operation }

// CancelBody is POST /ciots/{id}/cancellation.
type CancelBody struct {
	Reason string `json:"reason" validate:"required,max=500"`
}

// CloseBody is POST /ciots/{id}/closure. Full load sends cargo_weight (B65); TAC-Agregado sends trips (B43).
type CloseBody struct {
	ClosedAt    *string `json:"closed_at" validate:"omitempty,datetime=2006-01-02"`
	CargoWeight string  `json:"cargo_weight"`
	Routes      []Route `json:"routes" validate:"max=50"`
	TripCount   int     `json:"trip_count" validate:"gte=0"`
}

func (r Route) toANTT() ciot.OrigemDestino {
	return ciot.OrigemDestino{
		OrigemIBGE: r.OriginIBGE, DestinoIBGE: r.DestinationIBGE, OrigemCEP: r.OriginCEP, DestinoCEP: r.DestinationCEP,
		OrigemLatitude: r.OriginLat, OrigemLongitude: r.OriginLng, DestinoLatitude: r.DestinationLat,
		DestinoLongitude: r.DestinationLng, DistanciaPercorrida: r.DistanceKm,
	}
}

func routesToANTT(rs []Route) []ciot.OrigemDestino {
	var out []ciot.OrigemDestino
	for _, r := range rs {
		out = append(out, r.toANTT())
	}
	return out
}

// moneyText normalizes a valid amount to 2 decimals and leaves an invalid one for the validator to report.
func moneyText(s string) string {
	if c, err := ciot.ParseMoney(s); err == nil {
		return ciot.FormatMoney(c)
	}
	return s
}

// toDeclaracao maps the body to the ANTT declaration and normalizes it. id is the 12-character identifier
// (empty on a new declaration: the worker asks the generator, it is never made up here).
func (o Operation) toDeclaracao(id string, now time.Time) ciot.Declaracao {
	declared := now
	if o.DeclaredAt != nil {
		if t, err := time.Parse(time.RFC3339, *o.DeclaredAt); err == nil {
			declared = t
		}
	}
	d := ciot.Declaracao{
		IdOperacaoTransporte: id, TipoOperacao: o.OperationType,
		CpfCnpjContratado: o.CarrierDoc, RNTRCContratado: o.CarrierRNTRC,
		CpfCnpjContratante: o.HirerDoc, RNTRCContratante: o.HirerRNTRC, CpfCnpjDestinatario: o.RecipientDoc,
		ValorFrete: moneyText(o.FreightValue), DataDeclaracao: ciot.FormatDateTime(declared),
		IndContingencia: o.Contingency, JustificativaContingencia: o.ContingencyReason,
		DataInicioViagem: o.TripStart, DataFimViagem: o.TripEnd,
		OrigemDestino: routesToANTT(o.Routes),
		InfPagamento: ciot.InfPagamento{
			TipoPagamento: o.Payment.Type, IndPagamento: o.Payment.Indicator,
			CodigoInstituicaoFinanceira: o.Payment.BankCode, Agencia: o.Payment.Branch, Conta: o.Payment.Account,
			ChavePix: o.Payment.PixKey, IdentificadorPix: o.Payment.PixID, CpfCnpjCreditado: o.Payment.CreditedDoc,
		},
	}
	for _, v := range o.Vehicles {
		d.Veiculos = append(d.Veiculos, ciot.Veiculo{Placa: v.Plate, RNTRCVeiculo: v.RNTRC, NumeroEixos: v.Axles,
			Automotor: v.Automotor, CavaloTrator: v.Tractor})
	}
	if o.Cargo != nil {
		d.DadosCarga = &ciot.DadosCarga{NaturezaCarga: o.Cargo.Nature, PesoCarga: o.Cargo.Weight, TipoCarga: o.Cargo.CargoType,
			ContratantesCargaFrac: o.Cargo.FractionalHirerDocs}
	}
	for _, p := range o.Payment.Installments {
		d.InfPagamento.Parcelas = append(d.InfPagamento.Parcelas, ciot.Parcela{NumeroParcela: p.Number,
			DataVencimento: p.DueDate, ValorParcela: moneyText(p.Value)})
	}
	if i := o.Indicators; i != nil {
		d.InfIndicadoresOperacionais = &ciot.Indicadores{IndAltoDesempenho: i.HighPerformance, IndRetornoVazio: i.EmptyReturn,
			ComposicaoVeicular: i.VehicleComposite}
	}
	d.Normalize()
	return d
}
