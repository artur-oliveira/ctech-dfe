// Package tables carrega as tabelas de referência da NFS-e nacional
// (Anexo B: código de tributação nacional e NBS 2.0; Anexo C: indOp IBS/CBS).
//
// Os arquivos *_table.go são gerados por gen/generate.py a partir das planilhas
// oficiais e versionados no repositório — não edite à mão. Regenerar:
//
//	python3 go-dfe/nfse/tables/gen/generate.py
package tables

import "sort"

// TribNacionalEntry é uma linha da lista de serviços nacional (Anexo B).
// Code tem 6 dígitos: item(2) + subitem(2) + desdobro(2) — formato TSCodTribNac do XSD
// oficial (tiposSimples_v1.01.xsd). O gerador reconstrói Code a partir das colunas
// ITEM/SUBITEM/DESDOBRO da planilha, não da coluna de código: essa coluna é numérica e
// perde o zero à esquerda do item quando item < 10 (ex.: "10101" em vez de "010101").
// Não "simplifique" isso de volta para ler a coluna A diretamente.
type TribNacionalEntry struct {
	Code        string
	Item        string
	Subitem     string
	Desdobro    string
	Description string
}

// NBSEntry é uma linha da Nomenclatura Brasileira de Serviços 2.0 (Anexo B).
// Code é normalizado sem pontos e tem sempre 9 dígitos (TSCodNBS do XSD oficial) — só
// os códigos-folha da planilha são mantidos; linhas de hierarquia (seção/posição, com
// menos de 9 dígitos) são descartadas pelo gerador.
type NBSEntry struct {
	Code        string
	Description string
}

// IndOpEntry é uma linha da tabela indOp do IBS/CBS (Anexo C).
type IndOpEntry struct {
	Code                       string
	TipoOperacao               string
	CaracteristicaFornecimento string
	LocalFornecimento          string
	CampoLeiaute               string
}

// TribNacional devolve a entrada do código de tributação nacional.
func TribNacional(code string) (TribNacionalEntry, bool) {
	e, ok := tribNacionalTable[code]
	return e, ok
}

// IsValidTribNacional informa se o código de tributação nacional existe.
func IsValidTribNacional(code string) bool {
	_, ok := tribNacionalTable[code]
	return ok
}

// NBS devolve a entrada da NBS 2.0. code deve vir sem pontos.
func NBS(code string) (NBSEntry, bool) {
	e, ok := nbsTable[code]
	return e, ok
}

// IsValidNBS informa se o código NBS existe. code deve vir sem pontos.
func IsValidNBS(code string) bool {
	_, ok := nbsTable[code]
	return ok
}

// IndOp devolve a entrada de indOp do IBS/CBS.
func IndOp(code string) (IndOpEntry, bool) {
	e, ok := indOpTable[code]
	return e, ok
}

// IsValidIndOp informa se o código indOp existe.
func IsValidIndOp(code string) bool {
	_, ok := indOpTable[code]
	return ok
}

// EnumEntry é um valor de domínio fechado do XSD com o rótulo oficial extraído
// da própria documentação do tipo (`xs:documentation`).
type EnumEntry struct {
	Value string
	Label string
}

// Nomes dos tipos XSD usados como chave do catálogo. Todo domínio fechado da
// NFS-e é referenciado por estas constantes — nunca por literal solto.
const (
	EnumTipoAmbiente         = "TSTipoAmbiente"
	EnumAmbGeradorNFSe       = "TSAmbGeradorNFSe"
	EnumMotivoEmisTI         = "TSMotivoEmisTI"
	EnumEmitenteDPS          = "TSEmitenteDPS"
	EnumCodNaoNIF            = "TSCodNaoNIF"
	EnumCodJustCanc          = "TSCodJustCanc"
	EnumCodJustSubst         = "TSCodJustSubst"
	EnumCodJustAnaliseFiscal = "TSCodJustAnaliseFiscalCanc"
	EnumCodMotivoRejeicao    = "TSCodMotivoRejeicao"
	EnumCodAutorManifestacao = "TSCodAutorManifestacao"
	EnumModoPrestacao        = "TSModoPrestacao"
	EnumVincPrest            = "TSVincPrest"
	EnumMecAFComExPrestador  = "TSMecAFComExPrest"
	EnumMecAFComExTomador    = "TSMecAFComExToma"
	EnumMovTempBens          = "TSMovTempBens"
	EnumEnvMDIC              = "TSEnvMDIC"
	EnumTribISSQN            = "TSTribISSQN"
	EnumTipoImunidadeISSQN   = "TSTipoImunidadeISSQN"
	EnumTipoRetISSQN         = "TSTipoRetISSQN"
	EnumOpExigSuspensa       = "TSOpExigSuspensa"
	EnumBeneficioMunicipal   = "TBMISSQN"
	EnumOpSimpNac            = "TSOpSimpNac"
	EnumRegApuracaoSimpNac   = "TSRegimeApuracaoSimpNac"
	EnumRegEspTrib           = "TSRegEspTrib"
	EnumIdeDedRed            = "TSIdeDedRed"
	EnumCSTPISCofins         = "TSTipoCST"
	EnumTipoRetPISCofins     = "TSTipoRetPISCofins"
	EnumRTCTpOper            = "TSRTCTpOper"
	EnumRTCTpEnteGov         = "TSRTCTpEnteGov"
	EnumRTCIndDest           = "TSRTCIndDest"
	EnumRTCIndFinal          = "TSRTCIndFinal"
	EnumRTCTpReeRepRes       = "TSRTCTpReeRepRes"
	EnumRTCTipoChaveDFe      = "TSRTCTipoChaveDFe"
)

// Enum devolve as opções de um domínio fechado pelo nome do tipo XSD.
func Enum(typeName string) ([]EnumEntry, bool) {
	entries, ok := enumTables[typeName]
	return entries, ok
}

// IsValidEnum informa se value pertence ao domínio fechado typeName. Um tipo
// desconhecido devolve false: validar contra um catálogo inexistente nunca pode
// passar silenciosamente.
func IsValidEnum(typeName, value string) bool {
	for _, entry := range enumTables[typeName] {
		if entry.Value == value {
			return true
		}
	}
	return false
}

// EnumLabel devolve o rótulo oficial de um valor do domínio fechado.
func EnumLabel(typeName, value string) (string, bool) {
	for _, entry := range enumTables[typeName] {
		if entry.Value == value {
			return entry.Label, true
		}
	}
	return "", false
}

// EnumTypes devolve todos os nomes de tipo do catálogo, em ordem.
func EnumTypes() []string {
	names := make([]string, 0, len(enumTables))
	for name := range enumTables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
