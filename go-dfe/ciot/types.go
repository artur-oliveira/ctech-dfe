package ciot

import "strings"

// Operation types (TipoOperacao) [PDF].
const (
	TipoLotacao    = 1
	TipoFracionada = 2
	TipoAgregado   = 3
)

// Payment types (TipoPagamento) [PDF].
const (
	PagCartaoPrePago  = 1
	PagContaCorrente  = 2
	PagPoupanca       = 3
	PagContaPagamento = 4
	PagOutros         = 5
	PagPix            = 6
)

// Declaracao is the body of DeclaracaoOperacaoTransporte (spec section 5). Field names follow the spec table; the
// names of sub-fields the spec does not give (OrigemDestino codes, DadosCarga, bank fields, vehicle kind) are
// [pratica]/UNVERIFIED and must be confirmed in homologation. Money fields are text with 2 decimals.
type Declaracao struct {
	IdOperacaoTransporte       string          `json:"IdOperacaoTransporte"`
	TipoOperacao               int             `json:"TipoOperacao"`
	CpfCnpjContratado          string          `json:"CpfCnpjContratado"`
	RNTRCContratado            string          `json:"RNTRCContratado"`
	CpfCnpjContratante         string          `json:"CpfCnpjContratante"`
	RNTRCContratante           string          `json:"RNTRCContratante,omitempty"`
	CpfCnpjDestinatario        string          `json:"CpfCnpjDestinatario,omitempty"`
	ValorFrete                 string          `json:"ValorFrete"`
	DataDeclaracao             string          `json:"DataDeclaracao"`
	IndContingencia            bool            `json:"IndContingencia"`
	JustificativaContingencia  string          `json:"JustificativaContingencia,omitempty"`
	DataInicioViagem           string          `json:"DataInicioViagem,omitempty"`
	DataFimViagem              string          `json:"DataFimViagem"`
	Veiculos                   []Veiculo       `json:"Veiculos"`
	OrigemDestino              []OrigemDestino `json:"OrigemDestino,omitempty"`
	DadosCarga                 *DadosCarga     `json:"DadosCarga,omitempty"`
	InfPagamento               InfPagamento    `json:"InfPagamento"`
	InfIndicadoresOperacionais *Indicadores    `json:"InfIndicadoresOperacionais,omitempty"`
}

// Veiculo; Automotor/CavaloTrator are UNVERIFIED names (B83, B84, B101, B117 need the vehicle kind).
type Veiculo struct {
	Placa        string `json:"Placa"`
	RNTRCVeiculo string `json:"RNTRCVeiculo,omitempty"`
	NumeroEixos  int    `json:"NumeroEixos"`
	Automotor    bool   `json:"Automotor"`
	CavaloTrator bool   `json:"CavaloTrator,omitempty"`
}

// OrigemDestino: location by IBGE (7), CEP (8) or lat/long; sub-field names UNVERIFIED. Origem and Destino carry
// the same kind (B111); coordinates come in pairs (B110).
type OrigemDestino struct {
	OrigemIBGE          string `json:"OrigemIBGE,omitempty"`
	DestinoIBGE         string `json:"DestinoIBGE,omitempty"`
	OrigemCEP           string `json:"OrigemCEP,omitempty"`
	DestinoCEP          string `json:"DestinoCEP,omitempty"`
	OrigemLatitude      string `json:"OrigemLatitude,omitempty"`
	OrigemLongitude     string `json:"OrigemLongitude,omitempty"`
	DestinoLatitude     string `json:"DestinoLatitude,omitempty"`
	DestinoLongitude    string `json:"DestinoLongitude,omitempty"`
	DistanciaPercorrida int    `json:"DistanciaPercorrida"`
}

// DadosCarga; NaturezaCarga is the NCM position (4 digits); names other than ContratantesCargaFrac UNVERIFIED.
type DadosCarga struct {
	NaturezaCarga         string   `json:"NaturezaCarga"`
	PesoCarga             string   `json:"PesoCarga"`
	TipoCarga             int      `json:"TipoCarga"`
	ContratantesCargaFrac []string `json:"ContratantesCargaFrac,omitempty"`
}

// InfPagamento; bank and Pix sub-field names beyond the spec table are UNVERIFIED.
type InfPagamento struct {
	TipoPagamento               int       `json:"TipoPagamento"`
	IndPagamento                int       `json:"IndPagamento"`
	CodigoInstituicaoFinanceira string    `json:"CodigoInstituicaoFinanceira,omitempty"`
	Agencia                     string    `json:"Agencia,omitempty"`
	Conta                       string    `json:"Conta,omitempty"`
	ChavePix                    string    `json:"ChavePix,omitempty"`
	IdentificadorPix            string    `json:"IdentificadorPix,omitempty"`
	CpfCnpjCreditado            string    `json:"CpfCnpjCreditado"`
	Parcelas                    []Parcela `json:"Parcelas,omitempty"`
}

// Parcela is one installment.
type Parcela struct {
	NumeroParcela  int    `json:"NumeroParcela"`
	DataVencimento string `json:"DataVencimento"`
	ValorParcela   string `json:"ValorParcela"`
}

// Indicadores feed the freight floor.
type Indicadores struct {
	IndAltoDesempenho  bool `json:"IndAltoDesempenho"`
	IndRetornoVazio    bool `json:"IndRetornoVazio"`
	ComposicaoVeicular bool `json:"ComposicaoVeicular"`
}

// Normalize applies the spec section 2/5 normalizations in place: documents, RNTRC (8 to 9 digits), plates, and
// trims. Dates and money are produced by the Format helpers, so they are not touched here.
func (d *Declaracao) Normalize() {
	d.IdOperacaoTransporte = strings.ToUpper(strings.TrimSpace(d.IdOperacaoTransporte))
	d.CpfCnpjContratado = NormalizeDoc(d.CpfCnpjContratado)
	d.CpfCnpjContratante = NormalizeDoc(d.CpfCnpjContratante)
	d.CpfCnpjDestinatario = NormalizeDoc(d.CpfCnpjDestinatario)
	d.RNTRCContratado = NormalizeRNTRC(d.RNTRCContratado)
	d.RNTRCContratante = NormalizeRNTRC(d.RNTRCContratante)
	for i := range d.Veiculos {
		d.Veiculos[i].Placa = NormalizePlate(d.Veiculos[i].Placa)
		d.Veiculos[i].RNTRCVeiculo = NormalizeRNTRC(d.Veiculos[i].RNTRCVeiculo)
	}
	d.InfPagamento.CpfCnpjCreditado = NormalizeDoc(d.InfPagamento.CpfCnpjCreditado)
	if d.DadosCarga != nil {
		for i, c := range d.DadosCarga.ContratantesCargaFrac {
			d.DadosCarga.ContratantesCargaFrac[i] = NormalizeDoc(c)
		}
	}
}

// Requests of the other services. CodigoIdentificacaoOperacao is the 16-character CIOT. Field names other than
// the ones the spec gives (MotivoCancelamento, PesoCarga, QtdViagens, AnoDeclaracao, CpfCnpjInteressado,
// CPFCNPJTransportador, OrigemDestino, DistanciaPercorrida) are UNVERIFIED.
type (
	ConsultaTransportador struct {
		CpfCnpjInteressado   string `json:"CpfCnpjInteressado"`
		CpfCnpjTransportador string `json:"CpfCnpjTransportador"`
		RNTRC                string `json:"RNTRC,omitempty"`
	}
	ConsultaFrota struct {
		CpfCnpjInteressado   string   `json:"CpfCnpjInteressado"`
		CpfCnpjTransportador string   `json:"CpfCnpjTransportador"`
		Placas               []string `json:"Placas"` // up to 7 per call [pratica]
	}
	Cancelamento struct {
		CodigoIdentificacaoOperacao string `json:"CodigoIdentificacaoOperacao"`
		MotivoCancelamento          string `json:"MotivoCancelamento"` // up to 500 characters
	}
	Retificacao struct {
		CodigoIdentificacaoOperacao string `json:"CodigoIdentificacaoOperacao"`
		Declaracao
	}
	Encerramento struct {
		CodigoIdentificacaoOperacao string          `json:"CodigoIdentificacaoOperacao"`
		DataEncerramento            string          `json:"DataEncerramento,omitempty"`
		PesoCarga                   string          `json:"PesoCarga,omitempty"` // lotacao (B65)
		OrigemDestino               []OrigemDestino `json:"OrigemDestino,omitempty"`
		QtdViagens                  int             `json:"QtdViagens,omitempty"` // TAC-Agregado (B43)
	}
	ConsultaCIOTGerado struct {
		CodigoIdentificacaoOperacao string `json:"CodigoIdentificacaoOperacao"` // the 12 characters
		AnoDeclaracao               int    `json:"AnoDeclaracao,omitempty"`
	}
)
