package ctes

import (
	"strings"
	"testing"
	"time"
)

func proof() DeliveryProof {
	now := time.Now()
	return DeliveryProof{OccurredAt: now, HashAt: now, Hash: "AAAAAAAAAAAAAAAAAAAAAAAAAAA=",
		NFeKeys: []string{strings.Repeat("1", 44)}}
}

func TestDeliveryDescLengthsFitMOC(t *testing.T) {
	// "Tam." column of the MOC (IP02): 30, 46, 28, 44.
	for desc, max := range map[string]int{descComprovanteEntrega: 30, descCancComprovanteEntrega: 46, descInsucessoEntrega: 28, descCancInsucessoEntrega: 44} {
		if n := len([]rune(desc)); n > max {
			t.Errorf("%q has %d characters, MOC allows %d", desc, n, max)
		}
	}
}

func TestDeliveryProofCheck(t *testing.T) {
	if err := proof().check(); err != nil {
		t.Fatal(err)
	}
	p := proof()
	p.Hash = "short"
	if p.check() == nil {
		t.Fatal("hash must be 28 base64 characters")
	}
	p = proof()
	p.NFeKeys = append(p.NFeKeys, p.NFeKeys[0])
	if p.check() == nil {
		t.Fatal("duplicated NF-e (rejection 861) must be refused")
	}
	p = proof()
	p.Latitude = "-23.123456"
	if p.check() == nil {
		t.Fatal("latitude without longitude must be refused")
	}
}

func TestDeliveryProofDetailOrderKeys(t *testing.T) {
	p := proof()
	p.Latitude, p.Longitude = "-23.5", "-46.6"
	det := p.detail(map[string]any{}, "hashEntrega", "dhHashEntrega")
	for _, k := range []string{"latitude", "longitude", "hashEntrega", "dhHashEntrega", "infEntrega"} {
		if _, ok := det[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if det["infEntrega"].([]map[string]any)[0]["chNFe"] != p.NFeKeys[0] {
		t.Fatal("infEntrega/chNFe")
	}
}
