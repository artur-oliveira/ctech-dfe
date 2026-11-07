package v1

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/services/nfes"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

// Os tamanhos de texto do cadastro e do pedido de emissão nunca passam dos do leiaute da NF-e. A fonte única é o
// esquema gerado dos XSDs (go-dfe/validate); este teste falha quando uma tag `validate` deixa entrar um valor que
// a SEFAZ recusaria com 225, ou que a emissão teria de truncar.
func TestEntityLimitsMatchLayout(t *testing.T) {
	spec, err := validate.Load("nfe")
	if err != nil {
		t.Fatal(err)
	}
	const inf = "NFe/infNFe/"
	cases := []struct {
		typ   any
		field string
		path  string
	}{
		{AddressBody{}, "street", inf + "dest/enderDest/xLgr"},
		{AddressBody{}, "neighborhood", inf + "dest/enderDest/xBairro"},
		{AddressBody{}, "city", inf + "dest/enderDest/xMun"},
		{AddressBody{}, "number", inf + "dest/enderDest/nro"},
		{AddressBody{}, "complement", inf + "dest/enderDest/xCpl"},
		{AddressBody{}, "country_name", inf + "dest/enderDest/xPais"},
		{StateRegistrationBody{}, "state_registration", inf + "dest/IE"},
		{StateRegistrationBody{}, "ie_st", inf + "emit/IEST"},
		{ContactsBody{}, "emails", inf + "dest/email"},
		{PersonObjectBody{}, "fantasy_name", inf + "emit/xFant"},
		{PersonCreateBody{}, "name", inf + "dest/xNome"},
		{PersonUpdateBody{}, "name", inf + "dest/xNome"},
		{OrganizationCreateBody{}, "name", inf + "emit/xNome"},
		{OrganizationUpdateBody{}, "name", inf + "emit/xNome"},
		{ProductBody{}, "description", inf + "det/prod/xProd"},
		{ProductBody{}, "code", inf + "det/prod/cProd"},
		{nfes.NfeLocalBody{}, "x_nome", inf + "retirada/xNome"},
		{nfes.NfeLocalBody{}, "x_lgr", inf + "retirada/xLgr"},
		{nfes.NfeLocalBody{}, "nro", inf + "retirada/nro"},
		{nfes.NfeLocalBody{}, "x_cpl", inf + "retirada/xCpl"},
		{nfes.NfeLocalBody{}, "x_bairro", inf + "retirada/xBairro"},
		{nfes.NfeLocalBody{}, "x_mun", inf + "retirada/xMun"},
		{nfes.NfeLocalBody{}, "x_pais", inf + "retirada/xPais"},
		{nfes.NfeLocalBody{}, "email", inf + "retirada/email"},
		{nfes.NfeLocalBody{}, "ie", inf + "retirada/IE"},
		{nfes.NfeTransportItem{}, "transporta_nome", inf + "transp/transporta/xNome"},
		{nfes.NfeTransportItem{}, "transporta_ie", inf + "transp/transporta/IE"},
		{nfes.NfeTransportItem{}, "transporta_ender", inf + "transp/transporta/xEnder"},
		{nfes.NfeTransportItem{}, "transporta_mun", inf + "transp/transporta/xMun"},
	}
	for _, c := range cases {
		lo, hi, ok := spec.Lengths(c.path)
		if !ok {
			t.Errorf("%s: leiaute sem limites em %s", c.field, c.path)
			continue
		}
		dtoMin, dtoMax := tagLimits(t, c.typ, c.field)
		name := reflect.TypeOf(c.typ).Name() + "." + c.field
		if dtoMax == 0 || dtoMax > hi {
			t.Errorf("%s: max=%d, o leiaute aceita até %d (%s)", name, dtoMax, hi, c.path)
		}
		if lo > 1 && dtoMin < lo {
			t.Errorf("%s: min=%d, o leiaute exige ao menos %d (%s)", name, dtoMin, lo, c.path)
		}
	}
}

// tagLimits lê min= e max= da tag validate do campo json (depois de "dive", para listas).
func tagLimits(t *testing.T, typ any, jsonName string) (lo, hi int) {
	t.Helper()
	rt := reflect.TypeOf(typ)
	for i := range rt.NumField() {
		f := rt.Field(i)
		if strings.Split(f.Tag.Get("json"), ",")[0] != jsonName {
			continue
		}
		tag := f.Tag.Get("validate")
		if _, after, ok := strings.Cut(tag, "dive,"); ok {
			tag = after
		}
		for _, tok := range strings.Split(tag, ",") {
			if v, ok := strings.CutPrefix(tok, "min="); ok {
				lo, _ = strconv.Atoi(v)
			}
			if v, ok := strings.CutPrefix(tok, "max="); ok {
				hi, _ = strconv.Atoi(v)
			}
		}
		return lo, hi
	}
	t.Fatalf("%s sem o campo %s", rt.Name(), jsonName)
	return 0, 0
}
