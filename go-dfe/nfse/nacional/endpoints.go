// Package nacional implementa o provider do Sistema Nacional NFS-e (Sefin
// Nacional): REST + JSON com payload XML gzip+base64, mTLS com o mesmo
// certificado A1 usado nos demais documentos fiscais.
package nacional

import (
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

// Sistemas do ambiente nacional. Cada um tem host próprio por ambiente.
const (
	SystemSefin      = "sefin"
	SystemADN        = "adn"
	SystemDANFSE     = "danfse"
	SystemParametros = "parametros"
)

const municipalityTeresina = "2211001"

// Paths, com placeholders no formato de fmt.Sprintf.
const (
	PathNFSe                  = "/nfse"
	PathNFSeByKey             = "/nfse/%s"
	PathDPS                   = "/dps/%s"
	PathNFSeDPS               = "/nfse/dps/%s"
	PathEventos               = "/nfse/%s/eventos"
	PathEventoEspecifico      = "/nfse/%s/eventos/%s/%d"
	PathDistribuicaoNSU       = "/DFe/%d"
	PathEventosADN            = "/NFSe/%s/Eventos"
	PathDANFSE                = "/%s"
	PathParamAliquota         = "/%s/%s/%s/aliquota"
	PathParamConvenio         = "/%s/convenio"
	PathParamBeneficio        = "/%s/%s/%s/beneficio"
	PathParamRegimesEspeciais = "/%s/%s/%s/regimes_especiais"
	PathParamRetencoes        = "/%s/%s/retencoes"
)

// bases mapeia (sistema, ambiente) -> URL base.
//
// Fonte: tmp/apis-prod-restrita-e-producao.txt. Atenção ao segmento "/API"
// que existe SÓ na produção restrita do Sefin Nacional — a tabela de
// ambientes da spec omitia esse segmento; a fonte primária prevalece.
var bases = map[string]map[string]string{
	SystemSefin: {
		constants.EnvironmentProd: "https://sefin.nfse.gov.br/SefinNacional",
		constants.EnvironmentHom:  "https://sefin.producaorestrita.nfse.gov.br/API/SefinNacional",
	},
	SystemADN: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/contribuintes",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/contribuintes",
	},
	SystemDANFSE: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/danfse",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/danfse",
	},
	SystemParametros: {
		constants.EnvironmentProd: "https://adn.nfse.gov.br/parametrizacao",
		constants.EnvironmentHom:  "https://adn.producaorestrita.nfse.gov.br/parametrizacao",
	},
}

// ResolveBase devolve a URL base de (system, environment).
func ResolveBase(system, environment string) (string, error) {
	envs, ok := bases[system]
	if !ok {
		return "", fmt.Errorf("nacional: sistema desconhecido %q", system)
	}
	base, ok := envs[environment]
	if !ok {
		return "", fmt.Errorf("nacional: ambiente desconhecido %q para o sistema %q", environment, system)
	}
	return base, nil
}

// Operation é uma operação NFS-e do contribuinte. O padrão nacional é um guia:
// cada prefeitura decide quais operações implementa e com que path, então o
// fallback para o Sefin Nacional é decidido por operação — não pela base do
// município.
type Operation string

const (
	OpEmit         Operation = "emit"
	OpEvent        Operation = "evento"
	OpQueryByKey   Operation = "consulta"
	OpQueryByDPSID Operation = "consulta_dps"
)

// nationalPaths é o destino de cada operação no Sefin Nacional — usado quando
// o município não publicou a operação.
var nationalPaths = map[Operation]string{
	OpEmit:         PathNFSe,
	OpEvent:        PathEventos,
	OpQueryByKey:   PathNFSeByKey,
	OpQueryByDPSID: PathDPS,
}

// municipalAuthorizer é um autorizador municipal que recebe o mesmo envelope
// REST e o mesmo XML DPS do padrão nacional. paths lista SÓ as operações que o
// município publicou oficialmente; ambiente ou operação ausente nunca é
// completado por inferência — cai no Sefin Nacional.
type municipalAuthorizer struct {
	bases map[string]string
	paths map[Operation]string
}

// Teresina publica quatro operações (tmp/nfse-teresina.txt §3) e os paths não
// coincidem com os nacionais: a consulta pelo identificador da DPS fica sob
// /nfse. As demais operações (distribuição, DANFSE, parâmetros, consulta de
// evento específico) continuam no ambiente nacional.
var municipalAuthorizers = map[string]municipalAuthorizer{
	municipalityTeresina: {
		bases: map[string]string{
			constants.EnvironmentHom:  "https://nfse2-the.dsfweb.com.br/notafiscal-ws",
			constants.EnvironmentProd: "https://nfseapi.teresina.pi.gov.br/notafiscal-ws",
		},
		paths: map[Operation]string{
			OpEmit:         PathNFSe,
			OpEvent:        PathEventos,
			OpQueryByKey:   PathNFSeByKey,
			OpQueryByDPSID: PathNFSeDPS,
		},
	},
}

// ResolveOperation devolve a URL da operação. O autorizador municipal prevalece
// quando publicou essa operação naquele ambiente; caso contrário a chamada vai
// para o Sefin Nacional. args preenche os placeholders do path.
func ResolveOperation(op Operation, environment, municipalityCode string, args ...any) (string, error) {
	nationalPath, ok := nationalPaths[op]
	if !ok {
		return "", fmt.Errorf("nacional: operação desconhecida %q", op)
	}
	if mun, found := municipalAuthorizers[municipalityCode]; found {
		if base, hasEnv := mun.bases[environment]; hasEnv {
			if path, hasOp := mun.paths[op]; hasOp {
				return base + formatPath(path, args...), nil
			}
		}
	}
	base, err := ResolveBase(SystemSefin, environment)
	if err != nil {
		return "", err
	}
	return base + formatPath(nationalPath, args...), nil
}

func formatPath(path string, args ...any) string {
	if len(args) == 0 {
		return path
	}
	return fmt.Sprintf(path, args...)
}
