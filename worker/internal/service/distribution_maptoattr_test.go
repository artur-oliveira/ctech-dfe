package service

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestMapToAttr_OmitsNil(t *testing.T) {
	av := mapToAttr(map[string]any{
		"name": "X",
		"cest": nil,
	})
	if _, ok := av.Value["cest"]; ok {
		t.Errorf("expected nil 'cest' to be omitted, got %#v", av.Value["cest"])
	}
	if _, ok := av.Value["name"]; !ok {
		t.Errorf("expected 'name' to be present")
	}
	_ = types.AttributeValueMemberNULL{} // ensure types import used
}
