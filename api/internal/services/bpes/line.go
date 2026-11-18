package bpes

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/api-commons/problem"
)

// Linha do BP-e: o cadastro do que não muda de uma venda para outra. Uma linha serve a uma variante: no bilhete comum é o
// percurso (origem, destino, código e nome da linha, serviço, acomodação, prefixo e o ICMS); no TA é o voo (companhia,
// número, aeroportos, serviço, acomodação e o IBS/CBS); no TM é o grupo de viagem (placa, prefixo, trechos com o ICMS de
// cada um e os tipos de passageiro). A venda aponta a linha por id e informa só data e hora, poltrona e valores.

// BpeLine é o corpo do cadastro (`/bpe-lines`) e o formato em que a linha fica gravada.
type BpeLine struct {
	Name    string `json:"name" validate:"required,min=2,max=60,fiscaltext"`
	Variant string `json:"variant" validate:"required,oneof=normal ta tm"`
	// Modal vale no comum e no TM; no TA o modal é sempre o aéreo.
	Modal       *string       `json:"modal" validate:"omitempty,oneof=rodoviario aquaviario ferroviario"`
	Origin      *BpeLinePlace `json:"origin" validate:"omitempty"`
	Destination *BpeLinePlace `json:"destination" validate:"omitempty"`
	// Comum.
	RouteCode     *string  `json:"route_code" validate:"omitempty,refcode,max=20"`
	RouteName     *string  `json:"route_name" validate:"omitempty,min=2,max=100,fiscaltext"`
	Prefix        *string  `json:"prefix" validate:"omitempty,refcode,max=20"`
	TripType      *string  `json:"trip_type" validate:"omitempty,oneof=00 01 02 03"`
	ServiceType   *string  `json:"service_type" validate:"omitempty,oneof=1 2 3 4 5 6 7 8 9 10 11 12 13"`
	Accommodation *string  `json:"accommodation" validate:"omitempty,oneof=1 2 3 4 5 6 7"`
	ICMS          *BpeICMS `json:"icms" validate:"omitempty"`
	// TA.
	Carrier      *string   `json:"carrier" validate:"omitempty,alphanum,uppercase,min=2,max=3"`
	FlightNumber *string   `json:"flight_number" validate:"omitempty,refcode,max=20"`
	FromAirport  *string   `json:"from_airport" validate:"omitempty,alphanum,uppercase,min=3,max=4"`
	ToAirport    *string   `json:"to_airport" validate:"omitempty,alphanum,uppercase,min=3,max=4"`
	Tax          *BpeTATax `json:"tax" validate:"omitempty"`
	// TM.
	CFOP           *string      `json:"cfop" validate:"omitempty,cfop_cte"`
	Plate          *string      `json:"plate" validate:"omitempty,placa"`
	OriginUF       *string      `json:"origin_uf" validate:"omitempty,uf"`
	DestUF         *string      `json:"dest_uf" validate:"omitempty,ufex"`
	Legs           []BpeLineLeg `json:"legs" validate:"omitempty,max=990,dive"`
	PassengerTypes []string     `json:"passenger_types" validate:"omitempty,max=20,dive,min=1,max=15,fiscaltext"`
}

// BpeLinePlace é o município de início ou de fim, escolhido na tabela do IBGE (a UI grava o código, a UF e o nome).
type BpeLinePlace struct {
	UF       string `json:"uf" validate:"required,uf"`
	IBGECode string `json:"ibge_code" validate:"required,ibge"`
	City     string `json:"city" validate:"required,min=2,max=60,fiscaltext"`
}

// BpeLineLeg é um trecho da linha do TM: município de início, de fim (opcional) e o ICMS do trecho.
type BpeLineLeg struct {
	OriginIBGE string  `json:"origin_ibge_code" validate:"required,ibge"`
	DestIBGE   *string `json:"dest_ibge_code" validate:"omitempty,ibge"`
	ICMS       BpeICMS `json:"icms" validate:"required"`
}

// Domínios de tpViagem, tpServ e tpAcomodacao por variante (os mesmos dos corpos de emissão).
var (
	lineTripTypes      = map[string][]string{VariantNormal: {"00", "01"}, VariantTA: {"00", "01", "02", "03"}}
	lineServiceTypes   = map[string][]string{VariantNormal: {"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}, VariantTA: {"12", "13"}}
	lineAccommodations = map[string][]string{VariantNormal: {"1", "2", "3", "4", "5"}, VariantTA: {"6", "7"}}
)

func has(list []string, v *string) bool {
	if v == nil {
		return false
	}
	for _, x := range list {
		if x == *v {
			return true
		}
	}
	return false
}

func blank(p *string) bool { return p == nil || strings.TrimSpace(*p) == "" }

// Validate cobre o que as tags não expressam: os campos que cada variante exige e os domínios por variante. A linha é
// recusada na hora em que a emissão a recusaria.
func (l BpeLine) Validate() error {
	need := func(label string, ok bool) error {
		if ok {
			return nil
		}
		return problem.BadRequest(label + " é obrigatório na linha " + l.Variant)
	}
	switch l.Variant {
	case VariantNormal, VariantTA:
		for _, c := range []struct {
			label string
			ok    bool
		}{
			{"origem", l.Origin != nil}, {"destino", l.Destination != nil},
			{"tipo de viagem", has(lineTripTypes[l.Variant], l.TripType)},
			{"serviço", has(lineServiceTypes[l.Variant], l.ServiceType)},
			{"acomodação", has(lineAccommodations[l.Variant], l.Accommodation)},
		} {
			if err := need(c.label, c.ok); err != nil {
				return err
			}
		}
		if l.Variant == VariantNormal {
			return l.validateNormal(need)
		}
		return l.validateTA(need)
	default:
		return l.validateTM(need)
	}
}

func (l BpeLine) validateNormal(need func(string, bool) error) error {
	if err := need("modal", l.Modal != nil); err != nil {
		return err
	}
	if err := need("código da linha", !blank(l.RouteCode)); err != nil {
		return err
	}
	if err := need("nome do percurso", !blank(l.RouteName)); err != nil {
		return err
	}
	if err := need("ICMS", l.ICMS != nil); err != nil {
		return err
	}
	if msg := icmsProblem(*l.ICMS); msg != "" {
		return problem.BadRequest(msg)
	}
	return nil
}

func (l BpeLine) validateTA(need func(string, bool) error) error {
	for _, c := range []struct {
		label string
		ok    bool
	}{
		{"companhia", !blank(l.Carrier)}, {"número do voo", !blank(l.FlightNumber)},
		{"aeroporto de origem", !blank(l.FromAirport)}, {"aeroporto de destino", !blank(l.ToAirport)},
		{"classificação tributária", l.Tax != nil && l.Tax.ClassTrib != ""},
	} {
		if err := need(c.label, c.ok); err != nil {
			return err
		}
	}
	return nil
}

func (l BpeLine) validateTM(need func(string, bool) error) error {
	for _, c := range []struct {
		label string
		ok    bool
	}{
		{"modal", l.Modal != nil}, {"CFOP", !blank(l.CFOP)}, {"UF de início", !blank(l.OriginUF)},
		{"trecho", len(l.Legs) > 0}, {"tipo de passageiro", len(l.PassengerTypes) > 0},
	} {
		if err := need(c.label, c.ok); err != nil {
			return err
		}
	}
	for i, leg := range l.Legs {
		if msg := icmsProblem(leg.ICMS); msg != "" {
			return problem.BadRequest("trecho " + strconv.Itoa(i+1) + ": " + msg)
		}
	}
	return nil
}

// icmsProblem diz o que está errado no ICMS ("" se nada): o CST 00 e o 90 levam alíquota (decimal não negativo).
func icmsProblem(i BpeICMS) string {
	if i.CST != "00" && i.CST != "90" {
		return ""
	}
	if i.Rate == nil {
		return "informe a alíquota do ICMS"
	}
	if d, err := decimal.NewFromString(strings.TrimSpace(*i.Rate)); err != nil || d.Sign() < 0 {
		return "alíquota do ICMS inválida"
	}
	return ""
}
