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

// Paths, com placeholders no formato de fmt.Sprintf.
const (
	PathNFSe                  = "/nfse"
	PathNFSeByKey             = "/nfse/%s"
	PathDPS                   = "/dps/%s"
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
