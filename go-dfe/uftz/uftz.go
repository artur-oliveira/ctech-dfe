// Package uftz resolve o fuso horário civil de uma UF brasileira. dhEmi, dhSaiEnt e o AAMM da chave de acesso
// são horários locais da UF do emitente; um processo em UTC (Lambda) emitiria +00:00. Uso: uftz.Now("PI").
package uftz

import (
	"time"
	_ "time/tzdata" // o runtime provided.al2023 pode não ter /usr/share/zoneinfo
)

// zones: zona IANA da capital de cada UF. O Brasil não tem horário de verão desde 2019.
var zones = map[string]string{
	"SP": "America/Sao_Paulo", "RJ": "America/Sao_Paulo", "MG": "America/Sao_Paulo", "ES": "America/Sao_Paulo",
	"PR": "America/Sao_Paulo", "SC": "America/Sao_Paulo", "RS": "America/Sao_Paulo", "DF": "America/Sao_Paulo",
	"GO": "America/Sao_Paulo", "BA": "America/Bahia", "SE": "America/Maceio", "AL": "America/Maceio",
	"PE": "America/Recife", "PB": "America/Fortaleza", "RN": "America/Fortaleza", "CE": "America/Fortaleza",
	"PI": "America/Fortaleza", "MA": "America/Fortaleza", "TO": "America/Araguaina", "PA": "America/Belem",
	"AP": "America/Belem", "AM": "America/Manaus", "RR": "America/Boa_Vista", "RO": "America/Porto_Velho",
	"MT": "America/Cuiaba", "MS": "America/Campo_Grande", "AC": "America/Rio_Branco",
}

// fallbackZone vale para UF desconhecida ou vazia (horário de Brasília).
const fallbackZone = "America/Sao_Paulo"

// Location devolve o fuso da UF; UF desconhecida cai em Brasília.
func Location(uf string) *time.Location {
	name, ok := zones[uf]
	if !ok {
		name = fallbackZone
	}
	loc, err := time.LoadLocation(name)
	if err != nil { // tzdata embutido torna isto impossível; mantém o fuso fixo de Brasília
		return time.FixedZone("BRT", -3*3600)
	}
	return loc
}

// Now é o instante atual no fuso da UF.
func Now(uf string) time.Time { return time.Now().In(Location(uf)) }
