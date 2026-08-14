package services

import "testing"

func TestValidImportDocType(t *testing.T) {
	cases := map[string]bool{"nfe": true, "nfce": true, "cte": false, "mdfe": false, "nfse": false, "": false}
	for docType, want := range cases {
		if got := validImportDocType(docType); got != want {
			t.Errorf("validImportDocType(%q) = %v, want %v", docType, got, want)
		}
	}
}

func TestPeekXMLRoot_AcceptsAndRejects(t *testing.T) {
	cases := []struct {
		name    string
		xml     string
		want    string
		wantErr bool
	}{
		{"nfeProc", `<nfeProc xmlns="x"><NFe/></nfeProc>`, "nfeProc", false},
		{"bare NFe", `<NFe xmlns="x"><infNFe/></NFe>`, "NFe", false},
		{"other root", `<resNFe xmlns="x"/>`, "resNFe", false},
		{"malformed", `<nfeProc><NFe`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := peekXMLRoot([]byte(tc.xml))
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
