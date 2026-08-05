package nfses

import (
	"gopkg.aoctech.app/dfe/go-dfe/nfse/nacional"
)

// NfseEmitBody é o corpo de POST /v1.0/nfses. Diferente da NF-e, NFS-e tem
// UM serviço por documento — TCServ não é lista.
type NfseEmitBody struct {
	TpEmit       int    `json:"tp_emit" validate:"required,oneof=1 2 3"`
	MotivoEmisTI int    `json:"motivo_emis_ti" validate:"omitempty,oneof=1 2 3 4"`
	ChNFSeRej    string `json:"ch_nfse_rej" validate:"omitempty,len=50,numeric"`
	Competencia  string `json:"competencia" validate:"required,datebr"`

	// Quando tp_emit != 1 o prestador é uma pessoa do cadastro.
	PrestadorID     *string `json:"prestador_id" validate:"omitempty"`
	TomadorID       *string `json:"tomador_id" validate:"omitempty"`
	IntermediarioID *string `json:"intermediario_id" validate:"omitempty"`

	Service NfseServiceItem `json:"service" validate:"required"`

	// Substituição de NFS-e já emitida (gera o evento 105102 no fisco).
	SubstituiChave  *string `json:"substitui_chave" validate:"omitempty,len=50,numeric"`
	SubstituiMotivo *string `json:"substitui_motivo" validate:"omitempty,max=2"`

	InfoComplementar *string `json:"info_complementar" validate:"omitempty,max=2000"`
}

// NfseServiceItem referencia o catálogo e permite sobrescrever valor,
// alíquota e descrição por emissão — o mesmo padrão de resolveProducts.
type NfseServiceItem struct {
	ServiceSK   string  `json:"service_sk" validate:"required"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	Value       *string `json:"value" validate:"omitempty,money"`
	Aliquota    *string `json:"aliquota" validate:"omitempty,money"`
	Quantidade  *string `json:"quantidade" validate:"omitempty,money"`
	CTribMun    *string `json:"c_trib_mun" validate:"omitempty,max=20"`
}

// BuildIDDPS delega para a regra normativa que vive no go-dfe. NÃO
// reimplemente: a api e o go-dfe TÊM que produzir o mesmo identificador,
// porque um é a SK da linha e o outro é o Id assinado no infDPS.
func BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie string, nDPS int) string {
	return nacional.BuildIDDPS(cLocEmi, tpInsc, inscFederal, serie, nDPS)
}
