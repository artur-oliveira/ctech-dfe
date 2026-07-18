// Package endpoints is a 1:1 port of py-dfe's constants/endpoints.py: the
// SEFAZ endpoint URL registry per document type, authorizer (per-UF or
// shared SVRS/AN), and environment. Kept as a single file mirroring the
// source layout — pure lookup tables with one resolver function.
//
// Some UFs don't run their own SEFAZ webservice and redirect to a shared
// regional authorizer (SVRS for the doc types below — py-dfe's source has
// no SVAN entries for any doc type, so none are ported here; see table_test.go
// note). MT (Mato Grosso) is special-cased for NFe/NFCe/CTe: its CTe
// endpoints in particular split across three different path prefixes
// (ctews2, ctews, cte-ws) on the same domain — do not consolidate.
package endpoints

import (
	"fmt"
	"strings"
)

// serviceURLs maps service name -> full endpoint URL.
type serviceURLs map[string]string

// envURLs maps environment ("prod"/"hom") -> serviceURLs.
type envURLs map[string]serviceURLs

// registry maps authorizer (UF code, "SVRS", or "AN") -> envURLs.
type registry map[string]envURLs

// ep builds an envURLs from base URLs + relative paths (same paths for both
// prod/hom), mirroring py-dfe's _ep().
func ep(prod, hom string, paths map[string]string) envURLs {
	return envURLs{
		"prod": joinPaths(prod, paths),
		"hom":  joinPaths(hom, paths),
	}
}

func joinPaths(base string, paths map[string]string) serviceURLs {
	base = strings.TrimRight(base, "/")
	out := make(serviceURLs, len(paths))
	for k, v := range paths {
		out[k] = base + v
	}
	return out
}

// ep2 builds an envURLs from fully-formed URL maps per environment (used
// when prod/hom don't share a simple base+path shape), mirroring py-dfe's
// _ep2().
func ep2(prodURLs, homURLs map[string]string) envURLs {
	return envURLs{"prod": serviceURLs(prodURLs), "hom": serviceURLs(homURLs)}
}

// asmxFrag mirrors py-dfe's _asmx_frag: given a relative path like
// "/NFeInutilizacao4", returns "/NFeInutilizacao4/NFeInutilizacao4.asmx".
func asmxFrag(servicePath string) string {
	name := strings.TrimPrefix(servicePath, "/")
	return "/" + name + "/" + name + ".asmx"
}

// asmxFragMap applies asmxFrag to every value in paths, mirroring py-dfe's
// `{k: _asmx_frag(v) for k, v in paths.items()}` comprehensions.
func asmxFragMap(paths map[string]string) map[string]string {
	out := make(map[string]string, len(paths))
	for k, v := range paths {
		out[k] = asmxFrag(v)
	}
	return out
}

// uf codes catConsultaCadastro shares the same SVRS URL across RS and SVRS
// authorizers, mirroring py-dfe's _CAD_SVRS constant.
const catSVRS = "https://cad.svrs.rs.gov.br/ws/cadconsultacadastro/cadconsultacadastro4.asmx"

// nfFragPath mirrors py-dfe's _NF_FRAG_PATH: the common NF-e 4.00 service
// paths shared by GO, MG, MS (as-is) and BA (via asmxFrag).
var nfFragPath = map[string]string{
	"NfeInutilizacao":      "/NFeInutilizacao4",
	"NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
	"NfeStatusServico":     "/NFeStatusServico4",
	"NfeConsultaCadastro":  "/CadConsultaCadastro4",
	"RecepcaoEvento":       "/NFeRecepcaoEvento4",
	"NFeAutorizacao":       "/NFeAutorizacao4",
	"NFeRetAutorizacao":    "/NFeRetAutorizacao4",
}

// nfeRegistry mirrors py-dfe's _NFE.
var nfeRegistry = registry{
	"AM": ep(
		"https://nfe.sefaz.am.gov.br/services2/services",
		"https://homnfe.sefaz.am.gov.br/services2/services",
		map[string]string{
			"NfeInutilizacao":      "/NfeInutilizacao4",
			"NfeConsultaProtocolo": "/NfeConsulta4",
			"NfeStatusServico":     "/NfeStatusServico4",
			"NfeConsultaCadastro":  "/CadConsultaCadastro4",
			"RecepcaoEvento":       "/RecepcaoEvento4",
			"NFeAutorizacao":       "/NfeAutorizacao4",
			"NFeRetAutorizacao":    "/NfeRetAutorizacao4",
		},
	),
	"BA": ep(
		"https://nfe.sefaz.ba.gov.br/webservices",
		"https://hnfe.sefaz.ba.gov.br/webservices",
		asmxFragMap(nfFragPath),
	),
	"GO": ep(
		"https://nfe.sefaz.go.gov.br/nfe/services",
		"https://homolog.sefaz.go.gov.br/nfe/services",
		nfFragPath,
	),
	"MG": ep(
		"https://nfe.fazenda.mg.gov.br/nfe2/services",
		"https://hnfe.fazenda.mg.gov.br/nfe2/services",
		nfFragPath,
	),
	"MS": ep(
		"https://nfe.sefaz.ms.gov.br/ws",
		"https://hom.nfe.sefaz.ms.gov.br/ws",
		nfFragPath,
	),
	// MT: special-cased (own domain/path shape, not the shared nfFragPath
	// literal) — mirrors py-dfe's explicit MT dict in _NFE. Do not remove.
	"MT": ep(
		"https://nfe.sefaz.mt.gov.br/nfews/v2/services",
		"https://homologacao.sefaz.mt.gov.br/nfews/v2/services",
		map[string]string{
			"NfeInutilizacao":      "/NfeInutilizacao4",
			"NfeConsultaProtocolo": "/NfeConsulta4",
			"NfeStatusServico":     "/NfeStatusServico4",
			"NfeConsultaCadastro":  "/CadConsultaCadastro4",
			"RecepcaoEvento":       "/RecepcaoEvento4",
			"NFeAutorizacao":       "/NfeAutorizacao4",
			"NFeRetAutorizacao":    "/NfeRetAutorizacao4",
		},
	),
	"PE": ep(
		"https://nfe.sefaz.pe.gov.br/nfe-service/services",
		"https://nfehomolog.sefaz.pe.gov.br/nfe-service/services",
		nfFragPath,
	),
	"PR": ep(
		"https://nfe.sefa.pr.gov.br/nfe",
		"https://homologacao.nfe.sefa.pr.gov.br/nfe",
		nfFragPath,
	),
	"RS": ep2(
		map[string]string{
			"NfeInutilizacao":      "https://nfe.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfe.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfe.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"NfeConsultaCadastro":  catSVRS,
			"RecepcaoEvento":       "https://nfe.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
			"NFeAutorizacao":       "https://nfe.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfe.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
		},
		map[string]string{
			"NfeInutilizacao":      "https://nfe-homologacao.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"NfeConsultaCadastro":  catSVRS,
			"RecepcaoEvento":       "https://nfe-homologacao.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
			"NFeAutorizacao":       "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfe-homologacao.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
		},
	),
	"SP": ep(
		"https://nfe.fazenda.sp.gov.br/ws",
		"https://homologacao.nfe.fazenda.sp.gov.br/ws",
		map[string]string{
			"NfeInutilizacao":      "/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "/nfeconsultaprotocolo4.asmx",
			"NfeStatusServico":     "/nfestatusservico4.asmx",
			"NfeConsultaCadastro":  "/cadconsultacadastro4.asmx",
			"RecepcaoEvento":       "/nferecepcaoevento4.asmx",
			"NFeAutorizacao":       "/nfeautorizacao4.asmx",
			"NFeRetAutorizacao":    "/nferetautorizacao4.asmx",
		},
	),
	"SVRS": ep2(
		map[string]string{
			"NfeInutilizacao":      "https://nfe.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfe.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfe.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"NfeConsultaCadastro":  catSVRS,
			"RecepcaoEvento":       "https://nfe.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
			"NFeAutorizacao":       "https://nfe.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfe.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
		},
		map[string]string{
			"NfeInutilizacao":      "https://nfe-homologacao.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"NfeConsultaCadastro":  catSVRS,
			"RecepcaoEvento":       "https://nfe-homologacao.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
			"NFeAutorizacao":       "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfe-homologacao.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
		},
	),
	"AN": ep2(
		map[string]string{
			"NFeDistribuicaoDFe": "https://www1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
			"RecepcaoEvento":     "https://www.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
		},
		map[string]string{
			"NFeDistribuicaoDFe": "https://hom1.nfe.fazenda.gov.br/NFeDistribuicaoDFe/NFeDistribuicaoDFe.asmx",
			"RecepcaoEvento":     "https://hom1.nfe.fazenda.gov.br/NFeRecepcaoEvento4/NFeRecepcaoEvento4.asmx",
		},
	),
}

// nfeUFAuth mirrors py-dfe's _NFE_UF_AUTH.
var nfeUFAuth = mergeUFAuth(
	map[string]string{
		"AM": "AM", "BA": "BA", "GO": "GO", "MG": "MG", "MS": "MS",
		"MT": "MT", "PE": "PE", "PR": "PR", "RS": "RS", "SP": "SP",
	},
	"SVRS",
	[]string{"AC", "AL", "AP", "CE", "DF", "ES", "MA", "PA", "PB", "PI", "RJ", "RN", "RO", "RR", "SC", "SE", "TO", "EX"},
)

// nfceRegistry mirrors py-dfe's _NFCE.
var nfceRegistry = registry{
	"AM": ep(
		"https://nfce.sefaz.am.gov.br/nfce-services/services",
		"https://homnfce.sefaz.am.gov.br/nfce-services/services",
		map[string]string{
			"NFeAutorizacao":       "/NfeAutorizacao4",
			"NFeRetAutorizacao":    "/NfeRetAutorizacao4",
			"NfeInutilizacao":      "/NfeInutilizacao4",
			"NfeConsultaProtocolo": "/NfeConsulta4",
			"NfeStatusServico":     "/NfeStatusServico4",
			"RecepcaoEvento":       "/RecepcaoEvento4",
		},
	),
	"GO": ep(
		"https://nfe.sefaz.go.gov.br/nfe/services",
		"https://homolog.sefaz.go.gov.br/nfe/services",
		map[string]string{
			"NFeAutorizacao":       "/NFeAutorizacao4",
			"NFeRetAutorizacao":    "/NFeRetAutorizacao4",
			"NfeInutilizacao":      "/NFeInutilizacao4",
			"NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
			"NfeStatusServico":     "/NFeStatusServico4",
			"NfeConsultaCadastro":  "/CadConsultaCadastro4",
			"RecepcaoEvento":       "/NFeRecepcaoEvento4",
		},
	),
	"MS": ep(
		"https://nfce.sefaz.ms.gov.br/ws",
		"https://hom.nfce.sefaz.ms.gov.br/ws",
		map[string]string{
			"NFeAutorizacao":       "/NFeAutorizacao4",
			"NFeRetAutorizacao":    "/NFeRetAutorizacao4",
			"NfeInutilizacao":      "/NFeInutilizacao4",
			"NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
			"NfeStatusServico":     "/NFeStatusServico4",
			"NfeConsultaCadastro":  "/CadConsultaCadastro4",
			"RecepcaoEvento":       "/NFeRecepcaoEvento4",
		},
	),
	// MT: special-cased, own domain/service paths. Do not remove.
	"MT": ep(
		"https://nfce.sefaz.mt.gov.br/nfcews/services",
		"https://homologacao.sefaz.mt.gov.br/nfcews/services",
		map[string]string{
			"NFeAutorizacao":       "/NfeAutorizacao4",
			"NFeRetAutorizacao":    "/NfeRetAutorizacao4",
			"NfeInutilizacao":      "/NfeInutilizacao4",
			"NfeConsultaProtocolo": "/NfeConsulta4",
			"NfeStatusServico":     "/NfeStatusServico4",
			"RecepcaoEvento":       "/RecepcaoEvento4",
		},
	),
	"PR": ep(
		"https://nfce.sefa.pr.gov.br/nfce",
		"https://homologacao.nfce.sefa.pr.gov.br/nfce",
		map[string]string{
			"NFeAutorizacao":       "/NFeAutorizacao4",
			"NFeRetAutorizacao":    "/NFeRetAutorizacao4",
			"NfeInutilizacao":      "/NFeInutilizacao4",
			"NfeConsultaProtocolo": "/NFeConsultaProtocolo4",
			"NfeStatusServico":     "/NFeStatusServico4",
			"NfeConsultaCadastro":  "/CadConsultaCadastro4",
			"RecepcaoEvento":       "/NFeRecepcaoEvento4",
		},
	),
	"RS": ep2(
		map[string]string{
			"NFeAutorizacao":       "https://nfce.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfce.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
			"NfeInutilizacao":      "https://nfce.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfce.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfce.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"RecepcaoEvento":       "https://nfce.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
		},
		map[string]string{
			"NFeAutorizacao":       "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
			"NfeInutilizacao":      "https://nfce-homologacao.sefazrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfce-homologacao.sefazrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"RecepcaoEvento":       "https://nfce-homologacao.sefazrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
		},
	),
	"SP": ep(
		"https://nfce.fazenda.sp.gov.br/ws",
		"https://homologacao.nfce.fazenda.sp.gov.br/ws",
		map[string]string{
			"NFeAutorizacao":       "/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "/NFeRetAutorizacao4.asmx",
			"NfeInutilizacao":      "/NFeInutilizacao4.asmx",
			"NfeConsultaProtocolo": "/NFeConsultaProtocolo4.asmx",
			"NfeStatusServico":     "/NFeStatusServico4.asmx",
			"RecepcaoEvento":       "/NFeRecepcaoEvento4.asmx",
		},
	),
	"SVRS": ep2(
		map[string]string{
			"NFeAutorizacao":       "https://nfce.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfce.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
			"NfeInutilizacao":      "https://nfce.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfce.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfce.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"RecepcaoEvento":       "https://nfce.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
		},
		map[string]string{
			"NFeAutorizacao":       "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeAutorizacao/NFeAutorizacao4.asmx",
			"NFeRetAutorizacao":    "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeRetAutorizacao/NFeRetAutorizacao4.asmx",
			"NfeInutilizacao":      "https://nfce-homologacao.svrs.rs.gov.br/ws/nfeinutilizacao/nfeinutilizacao4.asmx",
			"NfeConsultaProtocolo": "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeConsulta/NfeConsulta4.asmx",
			"NfeStatusServico":     "https://nfce-homologacao.svrs.rs.gov.br/ws/NfeStatusServico/NfeStatusServico4.asmx",
			"RecepcaoEvento":       "https://nfce-homologacao.svrs.rs.gov.br/ws/recepcaoevento/recepcaoevento4.asmx",
		},
	),
}

// nfceUFAuth mirrors py-dfe's _NFCE_UF_AUTH.
var nfceUFAuth = mergeUFAuth(
	map[string]string{
		"AM": "AM", "GO": "GO", "MS": "MS", "MT": "MT",
		"PR": "PR", "RS": "RS", "SP": "SP",
	},
	"SVRS",
	[]string{"AC", "AL", "AP", "BA", "CE", "DF", "ES", "MA", "MG", "PA", "PB", "PE", "PI", "RJ", "RN", "RO", "RR", "SC", "SE", "TO", "EX"},
)

// cteFrag mirrors py-dfe's _CTE_FRAG.
var cteFrag = map[string]string{
	"CTeRecepcaoSinc":   "/CTeRecepcaoSincV4",
	"CTeRecepcaoOS":     "/CTeRecepcaoOSV4",
	"CTeRecepcaoGTVe":   "/CTeRecepcaoGTVeV4",
	"CTeRecepcaoSimp":   "/CTeRecepcaoSimpV4",
	"CTeConsulta":       "/CTeConsultaV4",
	"CTeStatusServico":  "/CTeStatusServicoV4",
	"CTeRecepcaoEvento": "/CTeRecepcaoEventoV4",
}

// cteAsmxDoubled mirrors py-dfe's SVRS CTe comprehension:
// f"https://{host}{v}{v}.asmx" for k, v in _CTE_FRAG.items() — the relative
// path is doubled (folder + file), e.g.
// "https://cte.svrs.rs.gov.br/ws/CTeRecepcaoSincV4/CTeRecepcaoSincV4.asmx".
func cteAsmxDoubled(host string) map[string]string {
	out := make(map[string]string, len(cteFrag))
	for k, v := range cteFrag {
		out[k] = host + v + v + ".asmx"
	}
	return out
}

// cteRegistry mirrors py-dfe's _CTE.
var cteRegistry = registry{
	"MG": ep(
		"https://cte.fazenda.mg.gov.br/cte/services",
		"https://hcte.fazenda.mg.gov.br/cte/services",
		cteFrag,
	),
	"MS": ep(
		"https://producao.cte.ms.gov.br/ws",
		"https://homologacao.cte.ms.gov.br/ws",
		cteFrag,
	),
	// MT: special-cased — three different path prefixes (ctews2, ctews,
	// cte-ws) on the same domain for different services. Do not consolidate.
	"MT": ep2(
		map[string]string{
			"CTeConsulta":       "https://cte.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4",
			"CTeRecepcaoEvento": "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoEventoV4",
			"CTeStatusServico":  "https://cte.sefaz.mt.gov.br/ctews2/services/CTeStatusServicoV4",
			"CTeRecepcaoSinc":   "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoSincV4",
			"CTeRecepcaoGTVe":   "https://cte.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoGTVeV4",
			"CTeRecepcaoOS":     "https://cte.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4",
			"CTeRecepcaoSimp":   "https://cte.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4",
		},
		map[string]string{
			"CTeConsulta":       "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeConsultaV4",
			"CTeRecepcaoEvento": "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoEventoV4",
			"CTeStatusServico":  "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeStatusServicoV4",
			"CTeRecepcaoSinc":   "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoSincV4",
			"CTeRecepcaoGTVe":   "https://homologacao.sefaz.mt.gov.br/ctews2/services/CTeRecepcaoGTVeV4",
			"CTeRecepcaoOS":     "https://homologacao.sefaz.mt.gov.br/ctews/services/CTeRecepcaoOSV4",
			"CTeRecepcaoSimp":   "https://homologacao.sefaz.mt.gov.br/cte-ws/services/CTeRecepcaoSimpV4",
		},
	),
	"PR": ep(
		"https://cte.fazenda.pr.gov.br/cte4",
		"https://homologacao.cte.fazenda.pr.gov.br/cte4",
		cteFrag,
	),
	// SP: cteFrag paths with ".asmx" appended (py-dfe applies a v.replace('V4',
	// 'V4') no-op before appending — kept here as a plain suffix, same result).
	"SP": ep(
		"https://nfe.fazenda.sp.gov.br/CTeWS/WS",
		"https://homologacao.nfe.fazenda.sp.gov.br/CTeWS/WS",
		suffixed(cteFrag, ".asmx"),
	),
	"SVRS": ep2(
		cteAsmxDoubled("https://cte.svrs.rs.gov.br/ws"),
		cteAsmxDoubled("https://cte-homologacao.svrs.rs.gov.br/ws"),
	),
	"AN": ep2(
		map[string]string{"CTeDistribuicaoDFe": "https://www1.cte.fazenda.gov.br/CTeDistribuicaoDFe/CTeDistribuicaoDFe.asmx"},
		map[string]string{"CTeDistribuicaoDFe": "https://hom1.cte.fazenda.gov.br/CTeDistribuicaoDFe/CTeDistribuicaoDFe.asmx"},
	),
}

// suffixed returns a copy of paths with suffix appended to every value.
func suffixed(paths map[string]string, suffix string) map[string]string {
	out := make(map[string]string, len(paths))
	for k, v := range paths {
		out[k] = v + suffix
	}
	return out
}

// cteUFAuth mirrors py-dfe's _CTE_UF_AUTH.
var cteUFAuth = mergeUFAuth(
	map[string]string{"MG": "MG", "MS": "MS", "MT": "MT", "PR": "PR", "SP": "SP"},
	"SVRS",
	[]string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "PA", "PB", "PE", "PI", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "TO", "EX"},
)

// mdfeSVRSFrag mirrors py-dfe's _MDFE_SVRS_FRAG.
var mdfeSVRSFrag = map[string]string{
	"MDFeRecepcaoEvento":  "/MDFeRecepcaoEvento/MDFeRecepcaoEvento.asmx",
	"MDFeConsulta":        "/MDFeConsulta/MDFeConsulta.asmx",
	"MDFeStatusServico":   "/MDFeStatusServico/MDFeStatusServico.asmx",
	"MDFeConsNaoEnc":      "/MDFeConsNaoEnc/MDFeConsNaoEnc.asmx",
	"MDFeDistribuicaoDFe": "/MDFeDistribuicaoDFe/MDFeDistribuicaoDFe.asmx",
	"MDFeRecepcaoSinc":    "/MDFeRecepcaoSinc/MDFeRecepcaoSinc.asmx",
}

// mdfeRegistry mirrors py-dfe's _MDFE. MDF-e has no per-UF authorizer at
// all — every UF redirects to SVRS.
var mdfeRegistry = registry{
	"SVRS": ep(
		"https://mdfe.svrs.rs.gov.br/ws",
		"https://mdfe-homologacao.svrs.rs.gov.br/ws",
		mdfeSVRSFrag,
	),
}

// mdfeUFAuth mirrors py-dfe's _MDFE_UF_AUTH: every UF -> SVRS.
var mdfeUFAuth = mergeUFAuth(
	nil,
	"SVRS",
	[]string{
		"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA",
		"MG", "MS", "MT", "PA", "PB", "PE", "PI", "PR", "RJ", "RN",
		"RO", "RR", "RS", "SC", "SE", "SP", "TO", "EX",
	},
)

// mergeUFAuth mirrors py-dfe's dict-merge idiom
// `{**direct, **{uf: shared for uf in ufs}}`: direct UF->authorizer entries
// plus every uf in ufs mapped to the shared authorizer.
func mergeUFAuth(direct map[string]string, shared string, ufs []string) map[string]string {
	out := make(map[string]string, len(direct)+len(ufs))
	for k, v := range direct {
		out[k] = v
	}
	for _, uf := range ufs {
		out[uf] = shared
	}
	return out
}

// docTypeTable pairs a doc type's registry with its UF->authorizer map,
// mirroring py-dfe's _REGISTRY.
type docTypeTable struct {
	reg    registry
	ufAuth map[string]string
}

// docTypeRegistry mirrors py-dfe's _REGISTRY. Doc type keys match
// constants.DocTypeNFE/DocTypeNFCE/DocTypeCTE/DocTypeMDFE ("nfe", "nfce",
// "cte", "mdfe").
var docTypeRegistry = map[string]docTypeTable{
	"nfe":  {nfeRegistry, nfeUFAuth},
	"nfce": {nfceRegistry, nfceUFAuth},
	"cte":  {cteRegistry, cteUFAuth},
	"mdfe": {mdfeRegistry, mdfeUFAuth},
}

// Authorizer returns the SEFAZ authorizer for (docType, uf), mirroring
// py-dfe's get_authorizer (py-dfe/py_dfe/constants/endpoints.py):
// `_UF_AUTH.get(doc_type, {}).get(uf, uf)` — unlike Resolve, an unknown
// doc_type or uf is NOT an error, it falls back to returning uf itself
// (this is what py-dfe actually does; used for response-node-path lookups,
// not endpoint URL resolution — a raw fallback there just means "no
// override applies", which is the correct default).
func Authorizer(docType, uf string) string {
	table, ok := docTypeRegistry[docType]
	if !ok {
		return uf
	}
	if a, ok := table.ufAuth[uf]; ok {
		return a
	}
	return uf
}

// Resolve returns the SEFAZ endpoint URL for the given doc type, UF,
// environment ("prod"/"hom") and service, mirroring py-dfe's get_endpoint().
//
// For services that route to the Ambiente Nacional (NFeDistribuicaoDFe,
// CTeDistribuicaoDFe), pass uf="AN" — this bypasses the per-UF authorizer
// lookup entirely and resolves directly against the "AN" registry entry.
// For every other uf, the authorizer is looked up in the doc type's
// UF->authorizer table (a direct per-UF authorizer, or the shared "SVRS"
// redirect); MDF-e has no direct authorizers at all — every UF resolves to
// SVRS.
func Resolve(docType, uf, environment, service string) (string, error) {
	table, ok := docTypeRegistry[docType]
	if !ok {
		return "", fmt.Errorf("endpoint not found: unknown doc_type %q", docType)
	}

	authorizer := uf
	if uf != "AN" {
		authorizer, ok = table.ufAuth[uf]
		if !ok {
			return "", fmt.Errorf("endpoint not found: doc_type=%q uf=%q (no authorizer mapping)", docType, uf)
		}
	}

	envs, ok := table.reg[authorizer]
	if !ok {
		return "", fmt.Errorf("endpoint not found: doc_type=%q uf=%q environment=%q service=%q (authorizer=%q)", docType, uf, environment, service, authorizer)
	}
	services, ok := envs[environment]
	if !ok {
		return "", fmt.Errorf("endpoint not found: doc_type=%q uf=%q environment=%q service=%q (authorizer=%q)", docType, uf, environment, service, authorizer)
	}
	url, ok := services[service]
	if !ok {
		return "", fmt.Errorf("endpoint not found: doc_type=%q uf=%q environment=%q service=%q (authorizer=%q)", docType, uf, environment, service, authorizer)
	}
	return url, nil
}
