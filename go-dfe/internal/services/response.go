package services

import (
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
	"gopkg.aoctech.app/dfe/go-dfe/internal/endpoints"
)

// responseNodeKey is the (authorizer, service) lookup key for the
// response-node-path tables below.
type responseNodeKey struct{ authorizer, service string }

// nfeNfceResponseNodePath mirrors py-dfe's _RESPONSE_NODE_PATH in
// py-dfe/py_dfe/services/_nf.py — shared by both nfe.py and nfce.py (nfce.py
// defines no override table of its own; both subclass _NFServiceClient and
// use these same module-level tables).
var nfeNfceResponseNodePath = map[responseNodeKey][]string{
	{"MG", "NfeConsultaCadastro"}: {"consultaCadastro4Result"},
	{"AM", "NfeConsultaCadastro"}: {"consultaCadastro2Result"},
	{"MT", "NfeConsultaCadastro"}: {"nfeResultMsg", "consultaCadastroResult"},
	{"AN", "NFeDistribuicaoDFe"}:  {"nfeDistDFeInteresseResponse", "nfeDistDFeInteresseResult"},
	{"AN", "RecepcaoEvento"}:      {"nfeRecepcaoEventoNFResult"},
}

// cteResponseNodePath mirrors py-dfe/py_dfe/services/cte.py's _RESPONSE_NODE_PATH.
var cteResponseNodePath = map[responseNodeKey][]string{
	{"AN", "CTeDistribuicaoDFe"}: {"cteDistDFeInteresseResponse", "cteDistDFeInteresseResult"},
}

// mdfeResponseNodePath mirrors py-dfe/py_dfe/services/mdfe.py's _RESPONSE_NODE_PATH.
var mdfeResponseNodePath = map[responseNodeKey][]string{
	{"SVRS", "MDFeDistribuicaoDFe"}: {"mdfeDistDFeInteresseResult"},
	{"SVRS", "MDFeRecepcaoSinc"}:    {"mdfeRecepcaoResult"},
	{"SVRS", "MDFeRecepcaoEvento"}:  {"mdfeRecepcaoEventoResult"},
}

// nfeNfceEnsureListPaths mirrors py-dfe's module-level _ENSURE_LIST_PATHS in
// py-dfe/py_dfe/services/_nf.py, applied generically by _NFServiceClient.call()
// (the method the actual py-dfe Lambda handler dispatches through for every
// service — the doc-type-specific *convenience* methods like
// perform_distribution/distribuicao_dfe that bypass this, e.g. NF-e's own
// hand-rolled distribution ensure_list, are Python-API sugar the Lambda path
// never calls; NOT ported here since matching them would introduce behavior
// production doesn't actually have today).
var nfeNfceEnsureListPaths = map[string][]string{
	"NFeAutorizacao":      {"retEnviNFe/protNFe"},
	"NfeConsultaCadastro": {"retConsCad/infCons/infCad"},
	"RecepcaoEvento":      {"retEnvEvento/retEvento"},
}

// cteEnsureListPaths / mdfeEnsureListPaths mirror py-dfe's cte.py/mdfe.py
// _ENSURE_LIST_PATHS, both empty in the current py-dfe source. Kept as
// explicit (empty) maps rather than omitted, so a future py-dfe addition is
// easy to spot and port here too.
var cteEnsureListPaths = map[string][]string{}
var mdfeEnsureListPaths = map[string][]string{}

func responseNodePathFor(docType string) map[responseNodeKey][]string {
	switch docType {
	case constants.DocTypeNFE, constants.DocTypeNFCE:
		return nfeNfceResponseNodePath
	case constants.DocTypeCTE:
		return cteResponseNodePath
	case constants.DocTypeMDFE:
		return mdfeResponseNodePath
	default:
		return nil
	}
}

func ensureListPathsFor(docType, service string) []string {
	switch docType {
	case constants.DocTypeNFE, constants.DocTypeNFCE:
		return nfeNfceEnsureListPaths[service]
	case constants.DocTypeCTE:
		return cteEnsureListPaths[service]
	case constants.DocTypeMDFE:
		return mdfeEnsureListPaths[service]
	default:
		return nil
	}
}

// unwrapResponseNode navigates raw (the full parsed SOAP result, keyed by
// whatever tag SEFAZ's response actually used — the SOAP Body's first
// child, per py-dfe's extract_body) down the per-(authorizer,service) node
// path, mirroring py-dfe's _parse_result_message. Defaults to a
// single-element path matching the doc type's normal SOAP result element
// name (constants.SOAPElements[docType].Result) when no override applies —
// this is NOT always "unwrap exactly one level": some authorizers
// (MT NfeConsultaCadastro) need two, and some services entirely replace the
// default with an unrelated name (AN's distribution/event responses use
// their own WSDL's element names, not the per-UF authorizer's nfeResultMsg).
//
// Returns an error if an expected node is missing, mirroring py-dfe's
// InvalidSefazResponseError — a missing node is a real SEFAZ response
// shape mismatch, not something to silently paper over.
func unwrapResponseNode(docType, uf, service string, raw map[string]any) (map[string]any, error) {
	authorizer := endpoints.Authorizer(docType, uf)

	path := responseNodePathFor(docType)[responseNodeKey{authorizer, service}]
	if path == nil {
		elems, ok := constants.SOAPElements[docType]
		if !ok {
			return nil, fmt.Errorf("no default response node for doc_type %q", docType)
		}
		path = []string{elems.Result}
	}

	var inner any = raw
	for _, node := range path {
		m, ok := inner.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected node %q not found in sefaz response (parent is not an object)", node)
		}
		next, ok := m[node]
		if !ok {
			return nil, fmt.Errorf("expected node %q not found in sefaz response", node)
		}
		inner = next
	}

	result, ok := inner.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("sefaz response node path %v resolved to a non-object value", path)
	}
	return result, nil
}

// ensureList mirrors py-dfe's _ensure_list (py-dfe/py_dfe/services/base.py):
// given a "/"-separated path into d, if the value at that path is a single
// object (XML can't distinguish "one occurrence" from "a list of one"
// without schema info), replace it with a one-element list in place, so
// callers can always range over it uniformly. A missing path, or a path
// whose value is already a list, is left untouched.
func ensureList(d map[string]any, path string) {
	keys := splitPath(path)
	if len(keys) == 0 {
		return
	}
	target := d
	for _, k := range keys[:len(keys)-1] {
		next, ok := target[k].(map[string]any)
		if !ok {
			return
		}
		target = next
	}
	leaf := keys[len(keys)-1]
	if v, ok := target[leaf].(map[string]any); ok {
		target[leaf] = []any{v}
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			out = append(out, path[start:i])
			start = i + 1
		}
	}
	return append(out, path[start:])
}
