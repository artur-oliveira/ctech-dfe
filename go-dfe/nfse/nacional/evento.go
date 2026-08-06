package nacional

import (
	"encoding/xml"
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const (
	idPedRegPrefix  = "PRE"
	widthTipoEvento = 6
)

type xmlPedRegEvento struct {
	XMLName   xml.Name     `xml:"pedRegEvento"`
	Xmlns     string       `xml:"xmlns,attr"`
	Versao    string       `xml:"versao,attr"`
	InfPedReg xmlInfPedReg `xml:"infPedReg"`
}

// xmlInfPedReg espelha TCInfPedReg. Apenas UM dos ponteiros e* é preenchido.
type xmlInfPedReg struct {
	ID        string        `xml:"Id,attr"`
	TpAmb     int           `xml:"tpAmb"`
	VerAplic  string        `xml:"verAplic"`
	DhEvento  string        `xml:"dhEvento"`
	CNPJAutor string        `xml:"CNPJAutor,omitempty"`
	CPFAutor  string        `xml:"CPFAutor,omitempty"`
	ChNFSe    string        `xml:"chNFSe"`
	E101101   *xmlMotivoReq `xml:"e101101,omitempty"`
	E105102   *xmlSubstEvt  `xml:"e105102,omitempty"`
	E101103   *xmlMotivoReq `xml:"e101103,omitempty"`
	E202201   *xmlXDesc     `xml:"e202201,omitempty"`
	E203202   *xmlXDesc     `xml:"e203202,omitempty"`
	E204203   *xmlXDesc     `xml:"e204203,omitempty"`
	E202205   *xmlMotivoOpt `xml:"e202205,omitempty"`
	E203206   *xmlMotivoOpt `xml:"e203206,omitempty"`
	E204207   *xmlMotivoOpt `xml:"e204207,omitempty"`
	E205208   *xmlAnulacao  `xml:"e205208,omitempty"`
}

// Descrições fixas de xDesc — cada tipo de evento tem uma única enumeração
// possível no XSD (tiposEventos_v1.01.xsd), nunca preenchida pelo chamador.
const (
	xDescCancelamento             = "Cancelamento de NFS-e"
	xDescCancelamentoPorSubst     = "Cancelamento de NFS-e por Substituição"
	xDescSolicAnaliseFiscalCanc   = "Solicitação de Análise Fiscal para Cancelamento de NFS-e"
	xDescConfirmacaoPrestador     = "Manifestação de NFS-e - Confirmação do Prestador"
	xDescConfirmacaoTomador       = "Manifestação de NFS-e - Confirmação do Tomador"
	xDescConfirmacaoIntermediario = "Manifestação de NFS-e - Confirmação do Intermediário"
	xDescRejeicaoPrestador        = "Manifestação de NFS-e - Rejeição do Prestador"
	xDescRejeicaoTomador          = "Manifestação de NFS-e - Rejeição do Tomador"
	xDescRejeicaoIntermediario    = "Manifestação de NFS-e - Rejeição do Intermediário"
	xDescAnulacaoRejeicao         = "Manifestação de NFS-e - Anulação da Rejeição"
)

// xmlXDesc cobre os eventos "vazios" (TE202201/TE203202/TE204203), cujo
// único conteúdo é a descrição fixa do evento.
type xmlXDesc struct {
	XDesc string `xml:"xDesc"`
}

// xmlMotivoReq é usado por TE101101/TE101103, onde xMotivo é obrigatório
// (sem minOccurs="0" no XSD).
type xmlMotivoReq struct {
	XDesc   string `xml:"xDesc"`
	CMotivo string `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo"`
}

// xmlMotivoOpt é usado por TE202205/TE203206/TE204207, onde xMotivo é
// opcional (minOccurs="0" no XSD).
type xmlMotivoOpt struct {
	XDesc   string `xml:"xDesc"`
	CMotivo string `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo,omitempty"`
}

type xmlSubstEvt struct {
	XDesc        string `xml:"xDesc"`
	CMotivo      string `xml:"cMotivo"`
	XMotivo      string `xml:"xMotivo,omitempty"`
	ChSubstituta string `xml:"chSubstituta"`
}

type xmlAnulacao struct {
	XDesc        string `xml:"xDesc"`
	CPFAgTrib    string `xml:"CPFAgTrib"`
	IDEvManifRej string `xml:"idEvManifRej"`
	XMotivo      string `xml:"xMotivo"`
}

// BuildPedRegEvento serializa o pedido de registro de evento, ainda SEM
// assinatura. Devolve o XML e o Id do infPedReg.
func BuildPedRegEvento(ev nfse.EventRequest) ([]byte, string, error) {
	if !nfse.ContribuinteEvents[ev.TipoEvento] {
		return nil, "", fmt.Errorf("nacional: evento %q não pode ser emitido pelo contribuinte", ev.TipoEvento)
	}
	if nfse.EventsRequiringMotivo[ev.TipoEvento] && (ev.Motivo == nil || ev.Motivo.Codigo == "") {
		return nil, "", fmt.Errorf("nacional: evento %q exige cMotivo", ev.TipoEvento)
	}
	if nfse.EventsRequiringXMotivo[ev.TipoEvento] && (ev.Motivo == nil || ev.Motivo.Descricao == "") {
		return nil, "", fmt.Errorf("nacional: evento %q exige xMotivo", ev.TipoEvento)
	}
	if ev.CNPJAutor == "" && ev.CPFAutor == "" {
		return nil, "", fmt.Errorf("nacional: evento sem CNPJAutor nem CPFAutor")
	}
	if ev.CNPJAutor != "" && ev.CPFAutor != "" {
		return nil, "", fmt.Errorf("nacional: evento com CNPJAutor e CPFAutor simultâneos — TCInfPedReg exige escolha única")
	}
	if ev.TipoEvento == nfse.EventCancelamentoPorSubst && ev.ChSubstituta == "" {
		return nil, "", fmt.Errorf("nacional: evento %q exige chSubstituta", ev.TipoEvento)
	}
	if ev.TipoEvento == nfse.EventAnulacaoRejeicao {
		if ev.CPFAgTrib == "" || ev.IDEvManifRej == "" || ev.Motivo == nil || ev.Motivo.Descricao == "" {
			return nil, "", fmt.Errorf("nacional: evento %q exige CPFAgTrib, idEvManifRej e xMotivo", ev.TipoEvento)
		}
	}

	// TSIdPedRegEvt (tiposSimples_v1.01.xsd) é "PRE" + chave(50) + tipoEvento(6)
	// = 59 caracteres, padrão PRE[0-9]{56} — sem espaço para nSeqEvento, apesar
	// da anotação do XSD mencioná-lo; o padrão (regex) prevalece sobre a prosa.
	id := idPedRegPrefix + ev.ChaveAcesso + leftPad(ev.TipoEvento, widthTipoEvento)

	inf := xmlInfPedReg{
		ID: id, TpAmb: ev.TpAmb, VerAplic: ev.VerAplic,
		DhEvento:  ev.DhEvento.UTC().Format(dateTimeUTCLayout),
		CNPJAutor: ev.CNPJAutor, CPFAutor: ev.CPFAutor, ChNFSe: ev.ChaveAcesso,
	}

	var cMotivo, xMotivo string
	if ev.Motivo != nil {
		cMotivo, xMotivo = ev.Motivo.Codigo, ev.Motivo.Descricao
	}

	switch ev.TipoEvento {
	case nfse.EventCancelamento:
		inf.E101101 = &xmlMotivoReq{XDesc: xDescCancelamento, CMotivo: cMotivo, XMotivo: xMotivo}
	case nfse.EventCancelamentoPorSubst:
		inf.E105102 = &xmlSubstEvt{XDesc: xDescCancelamentoPorSubst,
			CMotivo: cMotivo, XMotivo: xMotivo, ChSubstituta: ev.ChSubstituta}
	case nfse.EventSolicAnaliseFiscalCanc:
		inf.E101103 = &xmlMotivoReq{XDesc: xDescSolicAnaliseFiscalCanc, CMotivo: cMotivo, XMotivo: xMotivo}
	case nfse.EventConfirmacaoPrestador:
		inf.E202201 = &xmlXDesc{XDesc: xDescConfirmacaoPrestador}
	case nfse.EventConfirmacaoTomador:
		inf.E203202 = &xmlXDesc{XDesc: xDescConfirmacaoTomador}
	case nfse.EventConfirmacaoIntermediario:
		inf.E204203 = &xmlXDesc{XDesc: xDescConfirmacaoIntermediario}
	case nfse.EventRejeicaoPrestador:
		inf.E202205 = &xmlMotivoOpt{XDesc: xDescRejeicaoPrestador, CMotivo: cMotivo, XMotivo: xMotivo}
	case nfse.EventRejeicaoTomador:
		inf.E203206 = &xmlMotivoOpt{XDesc: xDescRejeicaoTomador, CMotivo: cMotivo, XMotivo: xMotivo}
	case nfse.EventRejeicaoIntermediario:
		inf.E204207 = &xmlMotivoOpt{XDesc: xDescRejeicaoIntermediario, CMotivo: cMotivo, XMotivo: xMotivo}
	case nfse.EventAnulacaoRejeicao:
		inf.E205208 = &xmlAnulacao{XDesc: xDescAnulacaoRejeicao, CPFAgTrib: ev.CPFAgTrib,
			IDEvManifRej: ev.IDEvManifRej, XMotivo: xMotivo}
	}

	out, err := xml.Marshal(xmlPedRegEvento{
		Xmlns: nfse.Namespace, Versao: nfse.LayoutVersion, InfPedReg: inf,
	})
	if err != nil {
		return nil, "", fmt.Errorf("nacional: serializar pedRegEvento: %w", err)
	}
	return out, id, nil
}
