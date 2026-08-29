package service

import "testing"

// The path must not change across the company re-key. A company's history is
// browsed by folder, and splitting it at the moment of the migration is the
// thing this derivation exists to prevent.
func TestTheDocumentPathIsUnchangedByTheRekey(t *testing.T) {
	s := &DfeService{}
	const (
		file = "21260536978000000108550010000018651146202607"
		want = "nfe/prod/CNPJ_11647612000197/" + file + ".xml"
	)
	// Before: the partition key carried the document.
	if got := s.documentS3Key("prod#CNPJ_11647612000197", "nfe", "11647612000197", file, "xml"); got != want {
		t.Errorf("legacy key: %q", got)
	}
	// After: it is a company id, and the path is the same.
	if got := s.documentS3Key("prod#01a04fc3-baa2-7cae-ac62-0ca3260a5888", "nfe", "11647612000197", file, "xml"); got != want {
		t.Errorf("company key: %q, want the same folder as before the re-key", got)
	}
}

// A CPF issuer keeps its own prefix, the way the retired key spelled it.
func TestACPFIssuerKeepsItsPrefix(t *testing.T) {
	s := &DfeService{}
	got := s.documentS3Key("prod#01a04fc3-baa2-7cae-ac62-0ca3260a5888", "nfe", "52998224725", "AK", "xml")
	if got != "nfe/prod/CPF_52998224725/AK.xml" {
		t.Errorf("got %q", got)
	}
}

// A message with no issuer document falls back to the partition key. Messages
// queued before the document was read off the record are still in flight, and a
// path that changed under them would write their XML somewhere their row does
// not point to.
func TestAMessageWithNoDocumentFallsBackToTheKey(t *testing.T) {
	s := &DfeService{}
	got := s.documentS3Key("prod#CNPJ_11647612000197", "nfe", "", "AK", "xml")
	if got != "nfe/prod/CNPJ_11647612000197/AK.xml" {
		t.Errorf("got %q", got)
	}
}

// A document of an unrecognized length invents no folder: a wrong prefix is an
// XML nobody finds by browsing, which is worse than the key's own segment.
func TestAnUnrecognizedDocumentInventsNoFolder(t *testing.T) {
	for _, doc := range []string{"123", "0199f3a1-8c42-7c31-9d5e-6a2b4c8e1f70"} {
		if got := tenantSegment(doc); got != "" {
			t.Errorf("%q produced the folder %q", doc, got)
		}
	}
}

// A DocPK with no environment half is left alone rather than reshaped: the
// document tables all carry one, and guessing at a shape that is not there
// would move an XML for a row that never had that path.
func TestADocPKWithNoEnvironmentIsLeftAlone(t *testing.T) {
	s := &DfeService{}
	got := s.documentS3Key("noenvhere", "nfe", "11647612000197", "AK", "xml")
	if got != "nfe/noenvhere/AK.xml" {
		t.Errorf("got %q", got)
	}
}
