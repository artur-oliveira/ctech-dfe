// Package tables carrega as tabelas de referência da NFS-e nacional
// (Anexo B: código de tributação nacional e NBS 2.0; Anexo C: indOp IBS/CBS).
//
// Os arquivos *_table.go são gerados por gen/generate.py a partir das planilhas
// oficiais e versionados no repositório — não edite à mão. Regenerar:
//
//	python3 go-dfe/nfse/tables/gen/generate.py
package tables

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
