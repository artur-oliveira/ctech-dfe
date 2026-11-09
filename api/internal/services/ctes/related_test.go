package ctes

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"
)

func TestRefPutItemsOnePerOriginal(t *testing.T) {
	items, err := refPutItems("ctes", "producao#org", "CHILD", cteTypeComplement, []string{"A", "B"}, "now")
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %d, err %v", len(items), err)
	}
	sk := items[1].Put.Item["sk"].(*types.AttributeValueMemberS).Value
	if sk != "ref#B#CHILD" {
		t.Fatalf("sk = %s", sk)
	}
	if _, ok := items[0].Put.Item["number"]; ok {
		t.Fatal("a linha de referência não pode ter number (entraria no GSI)")
	}
}

func TestRelatedLinkAndPartyShape(t *testing.T) {
	link := relatedLink(map[string]types.AttributeValue{
		"sk": &types.AttributeValueMemberS{Value: "K"}, "status": &types.AttributeValueMemberS{Value: "authorized"},
		"contingency_msg": &types.AttributeValueMemberS{Value: "segredo"},
	}).(*types.AttributeValueMemberM).Value
	if link["access_key"].(*types.AttributeValueMemberS).Value != "K" || link["contingency_msg"] != nil || link["sk"] != nil {
		t.Fatalf("link = %v", link)
	}
	p := partyAV(&nfexml.CTeActor{CNPJ: "123", Name: "X"}).(*types.AttributeValueMemberM).Value
	if p["doc"].(*types.AttributeValueMemberS).Value != "123" {
		t.Fatalf("party = %v", p)
	}
}

func TestEventsOfServiceOnlyForModel57(t *testing.T) {
	os := "35260812345678000190670010000000011000000012"
	cte := "35260812345678000190570010000000011000000012"
	if requireModel57(os) == nil {
		t.Fatal("evento exclusivo do 57 deve ser recusado para o CT-e OS")
	}
	if requireModel57(cte) != nil {
		t.Fatal("modelo 57 segue permitido")
	}
}
