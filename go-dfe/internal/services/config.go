// Package services implements the SEFAZ HTTP client (Strategy pattern per
// doc_type) that py-dfe/py_dfe/services/base.py + config.py implement in
// Python: JSON payload -> XML -> (optional sign) -> SOAP envelope -> mTLS
// POST with retry -> parsed response.
package services

import "fmt"

// Config is an immutable per-doc-type configuration, mirroring py-dfe's
// ServiceConfig (py-dfe/py_dfe/services/config.py). The HTTP/retry/SOAP layer
// stays generic; only this config varies by doc_type.
type Config struct {
	DocType                     string
	SchemaVersion               string
	WSDLServices                map[string]string
	ServicesRequiringSignature  map[string]bool
	ServicesRequiringValidation map[string]bool
	SignIDXPath                 map[string]string
}

// RequiresSignature reports whether service must be XML-DSig signed before sending.
func (c Config) RequiresSignature(service string) bool {
	return c.ServicesRequiringSignature[service]
}

// RequiresValidation reports whether service must be XSD-validated before
// sending. go-dfe does not implement XSD validation yet (see
// docs/plans/2026-07-17-go-dfe-migration.md — CGO_ENABLED=0 rules out the
// only mature Go XSD validator, libxml2); this is exposed for parity with
// py-dfe's config shape and so callers can fail loudly if it's ever wired to
// a validator that isn't there.
func (c Config) RequiresValidation(service string) bool {
	return c.ServicesRequiringValidation[service]
}

// SignXPath returns the element-local-name whose subtree gets the
// <Signature>, or "" if service has no signing xpath (falls back to signing
// the document root — see py-dfe's SefazClient._sign).
func (c Config) SignXPath(service string) string {
	return c.SignIDXPath[service]
}

// NFeConfig, NFCeConfig, CTeConfig, MDFeConfig are 1:1 ports of py-dfe's
// NFE_CONFIG/NFCE_CONFIG/CTE_CONFIG/MDFE_CONFIG.
var (
	NFeConfig = Config{
		DocType:       "nfe",
		SchemaVersion: "4.00",
		WSDLServices:  nfeWSDLService,
		ServicesRequiringSignature: map[string]bool{
			"NFeAutorizacao": true, "NfeInutilizacao": true, "RecepcaoEvento": true,
		},
		ServicesRequiringValidation: map[string]bool{
			"NFeAutorizacao": true, "RecepcaoEvento": true,
		},
		SignIDXPath: map[string]string{
			"NFeAutorizacao":  "infNFe",
			"NfeInutilizacao": "infInut",
			"RecepcaoEvento":  "infEvento",
		},
	}

	NFCeConfig = Config{
		DocType:       "nfce",
		SchemaVersion: "4.00",
		WSDLServices:  nfeWSDLService,
		ServicesRequiringSignature: map[string]bool{
			"NFeAutorizacao": true, "NfeInutilizacao": true, "RecepcaoEvento": true,
		},
		ServicesRequiringValidation: map[string]bool{
			"NFeAutorizacao": true, "NfeInutilizacao": true, "RecepcaoEvento": true,
		},
		SignIDXPath: map[string]string{
			"NFeAutorizacao":  "infNFe",
			"NfeInutilizacao": "infInut",
			"RecepcaoEvento":  "infEvento",
		},
	}

	CTeConfig = Config{
		DocType:       "cte",
		SchemaVersion: "4.00",
		WSDLServices:  cteWSDLService,
		ServicesRequiringSignature: map[string]bool{
			"CTeRecepcaoSinc": true, "CTeRecepcaoOS": true, "CTeRecepcaoGTVe": true,
			"CTeRecepcaoSimp": true, "CTeRecepcaoEvento": true,
		},
		ServicesRequiringValidation: map[string]bool{
			"CTeRecepcaoSinc": true, "CTeRecepcaoOS": true, "CTeRecepcaoGTVe": true,
			"CTeRecepcaoSimp": true, "CTeRecepcaoEvento": true,
		},
		SignIDXPath: map[string]string{
			"CTeRecepcaoSinc":   "infCte",
			"CTeRecepcaoOS":     "infCTeOS",
			"CTeRecepcaoGTVe":   "infGTVe",
			"CTeRecepcaoSimp":   "infCte",
			"CTeRecepcaoEvento": "infEvento",
		},
	}

	MDFeConfig = Config{
		DocType:       "mdfe",
		SchemaVersion: "3.00",
		WSDLServices:  mdfeWSDLService,
		ServicesRequiringSignature: map[string]bool{
			"MDFeRecepcaoSinc": true, "MDFeRecepcaoEvento": true,
		},
		ServicesRequiringValidation: map[string]bool{
			"MDFeRecepcaoSinc": true, "MDFeRecepcaoEvento": true,
		},
		SignIDXPath: map[string]string{
			"MDFeRecepcaoSinc":   "infMDFe",
			"MDFeRecepcaoEvento": "infEvento",
		},
	}
)

// nfeWSDLService / cteWSDLService / mdfeWSDLService duplicate
// go-dfe/internal/constants' WSDL tables under the doc-type-specific names
// py-dfe's config.py uses; kept local to avoid the config/constants packages
// import-cycling on doc-type-keyed data neither owns exclusively.
var (
	nfeWSDLService = map[string]string{
		"NFeAutorizacao": "NFeAutorizacao4", "NFeRetAutorizacao": "NFeRetAutorizacao4",
		"NfeInutilizacao": "NFeInutilizacao4", "NfeConsultaProtocolo": "NFeConsultaProtocolo4",
		"NfeStatusServico": "NFeStatusServico4", "NfeConsultaCadastro": "CadConsultaCadastro4",
		"RecepcaoEvento": "NFeRecepcaoEvento4", "NFeDistribuicaoDFe": "NFeDistribuicaoDFe",
	}
	cteWSDLService = map[string]string{
		"CTeRecepcaoSinc": "CTeRecepcaoSincV4", "CTeRecepcaoOS": "CTeRecepcaoOSV4",
		"CTeRecepcaoGTVe": "CTeRecepcaoGTVeV4", "CTeRecepcaoSimp": "CTeRecepcaoSimpV4",
		"CTeConsulta": "CTeConsultaV4", "CTeStatusServico": "CTeStatusServicoV4",
		"CTeRecepcaoEvento": "CTeRecepcaoEventoV4", "CTeDistribuicaoDFe": "CTeDistribuicaoDFe",
	}
	mdfeWSDLService = map[string]string{
		"MDFeRecepcaoSinc": "MDFeRecepcaoSinc", "MDFeConsulta": "MDFeConsulta",
		"MDFeStatusServico": "MDFeStatusServico", "MDFeConsNaoEnc": "MDFeConsNaoEnc",
		"MDFeDistribuicaoDFe": "MDFeDistribuicaoDFe", "MDFeRecepcaoEvento": "MDFeRecepcaoEvento",
	}
)

// configs indexes NFeConfig/NFCeConfig/CTeConfig/MDFeConfig by doc_type
// string, mirroring py-dfe's SERVICE_CONFIGS/get_config.
var configs = map[string]Config{
	"nfe":  NFeConfig,
	"nfce": NFCeConfig,
	"cte":  CTeConfig,
	"mdfe": MDFeConfig,
}

// GetConfig returns the Config for docType, or an error if docType is unknown.
func GetConfig(docType string) (Config, error) {
	if cfg, ok := configs[docType]; ok {
		return cfg, nil
	}
	return Config{}, fmt.Errorf("unknown doc_type %q", docType)
}
