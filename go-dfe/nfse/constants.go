// Package nfse é a camada NFS-e do go-dfe: modelo neutro de documento,
// interface de provider e tipos de resultado, compartilhados entre o provider
// nacional (REST+JSON, este pacote na F2) e o ABRASF 2.04 (SOAP, F5).
//
// Diferente de NF-e/CT-e/MDF-e, NFS-e não passa por internal/services nem por
// internal/soap — dfe.Call desvia para cá antes de montar um cliente SOAP.
package nfse

import "gopkg.aoctech.app/dfe/go-dfe/internal/constants"

// Espelho exportado dos nomes de serviço NFS-e. Consumidores fora do módulo
// (a api monta dfe.Request.Service) não podem importar internal/constants;
// aliasar em vez de redigitar garante que os dois lados nunca divirjam.
const (
	ServiceRecepcao             = constants.ServiceNFSeRecepcao
	ServiceConsulta             = constants.ServiceNFSeConsulta
	ServiceConsultaDPS          = constants.ServiceNFSeConsultaDPS
	ServiceEvento               = constants.ServiceNFSeEvento
	ServiceConsultaEvento       = constants.ServiceNFSeConsultaEvento
	ServiceDistribuicao         = constants.ServiceNFSeDistribuicao
	ServiceDANFSE               = constants.ServiceNFSeDANFSE
	ServiceParametrosMunicipais = constants.ServiceNFSeParametrosMunicipais
)

// Namespace e versão do leiaute nacional
// (tmp/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01/DPS_v1.01.xsd).
const (
	Namespace     = "http://www.sped.fazenda.gov.br/nfse"
	LayoutVersion = "1.01"
)

// Providers suportados. O valor vem de dfe.Request.Body["provider"].
const (
	ProviderNacional  = "nacional"
	ProviderAbrasf204 = "abrasf204"
)

// Tipos de evento que o contribuinte PODE emitir (Anexo II).
const (
	EventCancelamento             = "101101" // TE101101
	EventCancelamentoPorSubst     = "105102" // TE105102
	EventSolicAnaliseFiscalCanc   = "101103" // TE101103
	EventConfirmacaoPrestador     = "202201" // TE202201
	EventConfirmacaoTomador       = "203202" // TE203202
	EventConfirmacaoIntermediario = "204203" // TE204203
	EventRejeicaoPrestador        = "202205" // TE202205
	EventRejeicaoTomador          = "203206" // TE203206
	EventRejeicaoIntermediario    = "204207" // TE204207
	EventAnulacaoRejeicao         = "205208" // TE205208
)

// ContribuinteEvents é o conjunto fechado do que este pacote serializa.
// Os demais tipos do XSD (105104, 105105, 205204, 305101-305103) são
// privativos do fisco/município e só chegam pela distribuição — nunca são
// emitidos por nós.
var ContribuinteEvents = map[string]bool{
	EventCancelamento: true, EventCancelamentoPorSubst: true,
	EventSolicAnaliseFiscalCanc: true, EventConfirmacaoPrestador: true,
	EventConfirmacaoTomador: true, EventConfirmacaoIntermediario: true,
	EventRejeicaoPrestador: true, EventRejeicaoTomador: true,
	EventRejeicaoIntermediario: true, EventAnulacaoRejeicao: true,
}

// EventsRequiringMotivo são os tipos cujo grupo específico tem cMotivo
// obrigatório. Vive aqui, e não em nacional, porque quem monta o pedido (a api)
// valida antes de enfileirar: duas cópias da regra divergiriam.
var EventsRequiringMotivo = map[string]bool{
	EventCancelamento: true, EventCancelamentoPorSubst: true,
	EventSolicAnaliseFiscalCanc: true, EventRejeicaoPrestador: true,
	EventRejeicaoTomador: true, EventRejeicaoIntermediario: true,
}

// EventsRequiringXMotivo são os tipos cujo xMotivo NÃO tem minOccurs="0" no
// XSD — TE105102 e TE202205/TE203206/TE204207 o têm opcional, mas
// TE101101/TE101103 exigem.
var EventsRequiringXMotivo = map[string]bool{
	EventCancelamento: true, EventSolicAnaliseFiscalCanc: true,
}
