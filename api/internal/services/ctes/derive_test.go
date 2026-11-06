package ctes

import (
	"fmt"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CNPJ/CPF com dígito válido, só para teste.
const (
	cnpjA = "11222333000181"
	cnpjB = "11444777000161"
	cpfC  = "52998224725"
)

// nfeKeys guarda as chaves geradas: o cNF de GenerateAccessKey é aleatório.
var nfeKeys = map[int]string{}

func nfeKey(n int) string {
	if k, ok := nfeKeys[n]; ok {
		return k
	}
	nfeKeys[n] = services.GenerateAccessKey("SP", cnpjA, services.ModelNFe, 1, n, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), services.TpEmisNormal)
	return nfeKeys[n]
}

// nfeXML monta uma NF-e mínima; vol e itens são fragmentos já prontos.
func nfeXML(emit, dest, modFrete, vol, items string) string {
	party := func(tag, doc, ender, uf, cMun, xMun string) string {
		docTag := "CNPJ"
		if len(doc) == 11 {
			docTag = "CPF"
		}
		return fmt.Sprintf(`<%[1]s><%[2]s>%[3]s</%[2]s><xNome>N %[3]s</xNome><IE>123456789</IE><%[4]s><xLgr>Rua A</xLgr><nro>10</nro><xBairro>Centro</xBairro><cMun>%[6]s</cMun><xMun>%[7]s</xMun><UF>%[5]s</UF><CEP>01001000</CEP><cPais>1058</cPais><xPais>BRASIL</xPais></%[4]s></%[1]s>`,
			tag, docTag, doc, ender, uf, cMun, xMun)
	}
	return `<nfeProc><NFe xmlns="http://www.portalfiscal.inf.br/nfe"><infNFe><ide><mod>55</mod><serie>1</serie><nNF>123</nNF><dhEmi>2026-11-01T10:00:00-03:00</dhEmi></ide>` +
		party("emit", emit, "enderEmit", "SP", "3550308", "São Paulo") +
		party("dest", dest, "enderDest", "MG", "3106200", "Belo Horizonte") +
		items + `<total><ICMSTot><vNF>1500.00</vNF></ICMSTot></total><transp><modFrete>` + modFrete + `</modFrete>` + vol + `</transp></infNFe></NFe></nfeProc>`
}

const (
	volPeso   = `<vol><qVol>12</qVol><pesoB>320.000</pesoB><pesoL>300.000</pesoL></vol>`
	itemCaixa = `<det><prod><cProd>1</cProd><xProd>CAIXA</xProd><qCom>5</qCom><uCom>UN</uCom><vProd>1000.00</vProd></prod></det>`
	itemKG    = `<det><prod><cProd>2</cProd><xProd>ARROZ</xProd><qCom>50.5</qCom><uCom>kg</uCom><vProd>500.00</vProd></prod></det>`
)

func note(t *testing.T, key, status string, incoming int, xml string) loadedNote {
	t.Helper()
	n, err := nfexml.ParseNFe([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	return loadedNote{key: key, nfe: n, item: map[string]types.AttributeValue{
		"status":   &types.AttributeValueMemberS{Value: status},
		"incoming": &types.AttributeValueMemberN{Value: fmt.Sprint(incoming)},
	}}
}

func codes(d *derivation) map[string]int {
	out := map[string]int{}
	for _, w := range d.Warnings {
		out[w.Code]++
	}
	return out
}

func TestDerive_CIFSuggestsSender(t *testing.T) {
	d := deriveNotes([]loadedNote{
		note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, "0", volPeso, itemCaixa)),
		note(t, nfeKey(2), "authorized", 1, nfeXML(cnpjA, cnpjB, "0", volPeso, itemCaixa+itemKG)),
	})
	if d.TakerSuggestion == nil || d.TakerSuggestion.Role != RoleRemetente || d.TakerSuggestion.Reason != reasonCIF {
		t.Fatalf("sugestão = %+v", d.TakerSuggestion)
	}
	if d.Sender == nil || d.Sender.Doc != cnpjA || d.Recipient == nil || d.Recipient.Doc != cnpjB {
		t.Fatalf("partes = %+v %+v", d.Sender, d.Recipient)
	}
	if d.Origin.CityIBGE != "3550308" || d.Destination.UF != "MG" {
		t.Fatalf("percurso = %+v %+v", d.Origin, d.Destination)
	}
	if *d.Cargo.TotalWeightKg != "640.000" || d.Cargo.TotalValue != "3000.00" || d.Cargo.Volumes != 24 || d.Cargo.PredominantProduct != "CAIXA" {
		t.Fatalf("carga = %+v", d.Cargo)
	}
	if d.Notes[0].Source != sourceIssued || d.Notes[1].Source != sourceReceived {
		t.Fatalf("origem das notas = %s %s", d.Notes[0].Source, d.Notes[1].Source)
	}
	if len(d.Warnings) != 0 {
		t.Fatalf("avisos = %+v", d.Warnings)
	}
}

func TestDerive_FOBSuggestsRecipient(t *testing.T) {
	d := deriveNotes([]loadedNote{note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, "1", volPeso, itemCaixa))})
	if d.TakerSuggestion == nil || d.TakerSuggestion.Role != RoleDestinatario || d.TakerSuggestion.Reason != reasonFOB {
		t.Fatalf("sugestão = %+v", d.TakerSuggestion)
	}
}

func TestDerive_MixedOrOtherModFreteIsAmbiguous(t *testing.T) {
	for _, pair := range [][2]string{{"0", "1"}, {"2", "2"}, {"9", "9"}} {
		d := deriveNotes([]loadedNote{
			note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, pair[0], volPeso, itemCaixa)),
			note(t, nfeKey(2), "authorized", 0, nfeXML(cnpjA, cnpjB, pair[1], volPeso, itemCaixa)),
		})
		if d.TakerSuggestion != nil || codes(d)[WarnTakerAmbiguous] != 1 {
			t.Fatalf("%v: sugestão %+v avisos %+v", pair, d.TakerSuggestion, d.Warnings)
		}
	}
}

func TestDerive_WeightFallbacks(t *testing.T) {
	kg := deriveNotes([]loadedNote{note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, "0", "", itemCaixa+itemKG))})
	if kg.Notes[0].WeightKg == nil || *kg.Notes[0].WeightKg != "50.500" {
		t.Fatalf("peso pelos itens KG = %v", kg.Notes[0].WeightKg)
	}

	none := deriveNotes([]loadedNote{
		note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, "0", volPeso, itemCaixa)),
		note(t, nfeKey(2), "authorized", 0, nfeXML(cnpjA, cnpjB, "0", "", itemCaixa)),
	})
	if none.Notes[1].WeightKg != nil || none.Cargo.TotalWeightKg != nil || none.weight != nil {
		t.Fatal("nota sem peso deveria anular o peso total")
	}
	if w := none.Warnings; len(w) != 1 || w[0].Code != WarnWeightMissing || *w[0].AccessKey != nfeKey(2) {
		t.Fatalf("avisos = %+v", w)
	}
}

func TestDerive_MixedSendersAndRecipients(t *testing.T) {
	d := deriveNotes([]loadedNote{
		note(t, nfeKey(1), "authorized", 0, nfeXML(cnpjA, cnpjB, "0", volPeso, itemCaixa)),
		note(t, nfeKey(2), "authorized", 0, nfeXML(cnpjB, cpfC, "0", volPeso, itemCaixa)),
	})
	c := codes(d)
	if d.Sender != nil || d.Recipient != nil || d.Origin != nil || c[WarnMixedSender] != 1 || c[WarnMixedRecipient] != 1 {
		t.Fatalf("partes %+v %+v avisos %+v", d.Sender, d.Recipient, d.Warnings)
	}
}

func TestDerive_NotFoundAndNotAuthorized(t *testing.T) {
	d := deriveNotes([]loadedNote{
		{key: nfeKey(9)},
		note(t, nfeKey(1), "pending", 0, nfeXML(cnpjA, cnpjB, "0", volPeso, itemCaixa)),
	})
	c := codes(d)
	if c[WarnNoteNotFound] != 1 || c[WarnNoteNotAuthorized] != 1 || len(d.Notes) != 1 {
		t.Fatalf("avisos = %+v notas = %d", d.Warnings, len(d.Notes))
	}
	if len(d.notFound) != 1 || len(d.notAuthorized) != 1 {
		t.Fatal("emissão precisa das listas de notas fora e não autorizadas")
	}
	if empty := deriveNotes([]loadedNote{{key: nfeKey(9)}}); len(empty.Notes) != 0 {
		t.Fatal("sem nota achada não há derivação")
	}
}
