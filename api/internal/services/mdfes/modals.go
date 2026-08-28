package mdfes

import "strconv"

// modals.go holds the non-rodoviário transport modals. Unlike rodoviário (which
// is assembled from internal vehicle/owner records), the aéreo, aquaviário and
// ferroviário modals carry no internal data source: their payload is supplied
// directly through the API and translated here, field-for-field, into the
// XSD structure (mdfeModalAereo/Aquaviario/Ferroviario_v3.00). Element ordering
// is applied downstream by py-dfe's XSD_ORDER table.
//
// Quais modais a emissão aceita está em enabledModals; validateModalPayload
// garante que o payload do modal escolhido veio junto.

// MdfeAirModal mirrors the <aereo> group. Todos os seis campos são obrigatórios
// no XSD — não há grupo opcional no modal aéreo.
type MdfeAirModal struct {
	Nac     string `json:"nationality" validate:"required,max=4"`    // nac — matrícula da aeronave (nacionalidade)
	Matr    string `json:"registration" validate:"required,max=6"`   // matr — marca/matrícula
	NVoo    string `json:"flight_number" validate:"required,max=9"`  // nVoo
	CAerEmb string `json:"origin_airport" validate:"required,max=4"` // cAerEmb — aeródromo de embarque (IATA)
	CAerDes string `json:"dest_airport" validate:"required,max=4"`   // cAerDes — aeródromo de destino (IATA)
	DVoo    string `json:"flight_date" validate:"required,isodate"`  // dVoo — AAAA-MM-DD
}

func buildAereo(a *MdfeAirModal) map[string]any {
	if a == nil {
		return map[string]any{}
	}
	return map[string]any{
		"nac":     a.Nac,
		"matr":    a.Matr,
		"nVoo":    a.NVoo,
		"cAerEmb": a.CAerEmb,
		"cAerDes": a.CAerDes,
		"dVoo":    a.DVoo,
	}
}

// MdfeWaterTerminal is one loading/unloading terminal (infTermCarreg/Descarreg).
type MdfeWaterTerminal struct {
	Code string `json:"code" validate:"required,max=8"`  // cTermCarreg / cTermDescarreg
	Name string `json:"name" validate:"required,max=60"` // xTermCarreg / xTermDescarreg
}

// MdfeWaterBarge é uma balsa do comboio (infEmbComb). Apesar do nome, o grupo
// não tem nada de combustível: "EmbComb" é embarcação do comboio.
type MdfeWaterBarge struct {
	Code string `json:"code" validate:"required,max=10"` // cEmbComb
	Name string `json:"name" validate:"required,max=60"` // xBalsa
}

// MdfeWaterModal mirrors the <aquav> group.
type MdfeWaterModal struct {
	Irin            string              `json:"irin" validate:"required,max=10"`
	TpEmb           string              `json:"vessel_type" validate:"required,len=2,number"`   // tpEmb
	CEmbar          string              `json:"vessel_code" validate:"required,max=10"`         // cEmbar
	XEmbar          string              `json:"vessel_name" validate:"required,max=60"`         // xEmbar
	NViag           string              `json:"voyage_number" validate:"required,max=10"`       // nViag
	CPrtEmb         string              `json:"origin_port" validate:"required,len=5"`          // cPrtEmb
	CPrtDest        string              `json:"dest_port" validate:"required,len=5"`            // cPrtDest
	PrtTrans        string              `json:"transit_port" validate:"omitempty,max=60"`       // prtTrans (optional)
	TpNav           string              `json:"navigation_type" validate:"omitempty,oneof=0 1"` // tpNav (optional)
	LoadTerminals   []MdfeWaterTerminal `json:"loading_terminals" validate:"omitempty,max=5,dive"`
	UnloadTerminals []MdfeWaterTerminal `json:"unloading_terminals" validate:"omitempty,max=5,dive"`
	// Barges são as balsas do comboio (infEmbComb), até 30.
	Barges []MdfeWaterBarge `json:"barges" validate:"omitempty,max=30,dive"`
	// As unidades vazias apontam para organization_cargo_units: o contêiner ou a
	// carreta que viaja vazia é a mesma que já está cadastrada.
	EmptyCargoUnitIDs     []string `json:"empty_cargo_unit_ids" validate:"omitempty,dive,required"`
	EmptyTransportUnitIDs []string `json:"empty_transport_unit_ids" validate:"omitempty,dive,required"`
	// MMSI é a identificação da embarcação no sistema marítimo internacional.
	MMSI string `json:"mmsi" validate:"omitempty,max=9"`
}

// emptyUnitNodes são as unidades vazias já resolvidas contra o cadastro.
type emptyUnitNodes struct {
	Cargo     []map[string]any
	Transport []map[string]any
}

func buildAquav(w *MdfeWaterModal, empty emptyUnitNodes) map[string]any {
	if w == nil {
		return map[string]any{}
	}
	aquav := map[string]any{
		"irin":     w.Irin,
		"tpEmb":    w.TpEmb,
		"cEmbar":   w.CEmbar,
		"xEmbar":   w.XEmbar,
		"nViag":    w.NViag,
		"cPrtEmb":  w.CPrtEmb,
		"cPrtDest": w.CPrtDest,
	}
	setIfStr(aquav, "prtTrans", w.PrtTrans)
	setIfStr(aquav, "tpNav", w.TpNav)
	if terms := buildWaterTerminals(w.LoadTerminals, "cTermCarreg", "xTermCarreg"); len(terms) > 0 {
		aquav["infTermCarreg"] = terms
	}
	if terms := buildWaterTerminals(w.UnloadTerminals, "cTermDescarreg", "xTermDescarreg"); len(terms) > 0 {
		aquav["infTermDescarreg"] = terms
	}
	if len(w.Barges) > 0 {
		barges := make([]map[string]any, 0, len(w.Barges))
		for _, b := range w.Barges {
			barges = append(barges, map[string]any{"cEmbComb": b.Code, "xBalsa": b.Name})
		}
		aquav["infEmbComb"] = barges
	}
	if len(empty.Cargo) > 0 {
		aquav["infUnidCargaVazia"] = empty.Cargo
	}
	if len(empty.Transport) > 0 {
		aquav["infUnidTranspVazia"] = empty.Transport
	}
	setIfStr(aquav, "MMSI", w.MMSI)
	return aquav
}

func buildWaterTerminals(terminals []MdfeWaterTerminal, codeTag, nameTag string) []map[string]any {
	out := make([]map[string]any, 0, len(terminals))
	for _, t := range terminals {
		out = append(out, map[string]any{codeTag: t.Code, nameTag: t.Name})
	}
	return out
}

// MdfeRailWagon mirrors one <vag> entry.
type MdfeRailWagon struct {
	PesoBC string `json:"weight_bc" validate:"required,decimalv"`   // pesoBC
	PesoR  string `json:"weight_real" validate:"required,decimalv"` // pesoR
	TpVag  string `json:"wagon_type" validate:"omitempty,max=3"`    // tpVag (optional)
	Serie  string `json:"series" validate:"required,max=3"`         // serie
	NVag   string `json:"number" validate:"required,max=9"`         // nVag
	NSeq   string `json:"sequence" validate:"omitempty,max=2"`      // nSeq (optional)
	TU     string `json:"tu" validate:"required,decimalv"`          // TU — tonelada útil
}

// MdfeRailModal mirrors the <ferrov> group (trem + wagons). qVag é derivado da
// lista de vagões, nunca informado.
type MdfeRailModal struct {
	XPref  string          `json:"train_prefix" validate:"required,max=10"` // trem/xPref
	DhTrem string          `json:"train_datetime" validate:"omitempty"`
	XOri   string          `json:"origin_station" validate:"required,max=100"` // trem/xOri
	XDest  string          `json:"dest_station" validate:"required,max=100"`   // trem/xDest
	Wagons []MdfeRailWagon `json:"wagons" validate:"required,min=1,dive"`
}

func buildFerrov(r *MdfeRailModal) map[string]any {
	if r == nil {
		return map[string]any{}
	}
	trem := map[string]any{
		"xPref": r.XPref,
		"xOri":  r.XOri,
		"xDest": r.XDest,
		"qVag":  intString(len(r.Wagons)),
	}
	setIfStr(trem, "dhTrem", r.DhTrem)

	vags := make([]map[string]any, 0, len(r.Wagons))
	for _, w := range r.Wagons {
		vag := map[string]any{
			"pesoBC": w.PesoBC,
			"pesoR":  w.PesoR,
			"serie":  w.Serie,
			"nVag":   w.NVag,
			"TU":     w.TU,
		}
		setIfStr(vag, "tpVag", w.TpVag)
		setIfStr(vag, "nSeq", w.NSeq)
		vags = append(vags, vag)
	}
	return map[string]any{"trem": trem, "vag": vags}
}

func setIfStr(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

func intString(n int) string { return strconv.Itoa(n) }
