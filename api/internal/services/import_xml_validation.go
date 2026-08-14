package services

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// maxImportXMLSize bounds the multipart file accepted by
// POST /distributions/{doc_type}/import-xml — generous for a single NF-e/
// NFC-e document (typically a few KB to a few hundred KB), rejects anything
// clearly not a single fiscal document upload.
const maxImportXMLSize = 1 << 20 // 1 MiB

var importXMLDocTypes = map[string]bool{DocTypeNFe: true, DocTypeNFCe: true}

// validImportDocType restricts XML import to nfe/nfce — CT-e/MDF-e/NFS-e are
// out of scope for this feature (docs/specs/2026-08-13-importacao-nfe-xml.md).
func validImportDocType(docType string) bool {
	return importXMLDocTypes[docType]
}

// peekXMLRoot tokenizes the document — no tree/semantic parse — and returns
// the root element's local name, so the caller can fail fast on an
// unsupported root or syntactically broken upload before spending
// S3/SQS/SEFAZ-quota on it. Full structural validation happens in the worker.
func peekXMLRoot(xmlBytes []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	root := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			if root == "" {
				return "", fmt.Errorf("xml sem elemento raiz")
			}
			return root, nil
		}
		if err != nil {
			return "", fmt.Errorf("xml malformado: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok && root == "" {
			root = start.Name.Local
		}
	}
}
