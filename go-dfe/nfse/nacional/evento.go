package nacional

import (
	"encoding/xml"
	"fmt"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

const (
	idPedRegPrefix  = "PRE"
	widthTipoEvento = 6
	widthNSeqEvento = 3
)

type xmlPedRegEvento struct {
	XMLName   xml.Name     `xml:"pedRegEvento"`
	Xmlns     string       `xml:"xmlns,attr"`
	Versao    string       `xml:"versao,attr"`
	InfPedReg xmlInfPedReg `xml:"infPedReg"`
}

// xmlInfPedReg espelha TCInfPedReg. Apenas UM dos ponteiros e* é preenchido.
type xmlInfPedReg struct {
	ID        string       `xml:"Id,attr"`
	TpAmb     int          `xml:"tpAmb"`
	VerAplic  string       `xml:"verAplic"`
	DhEvento  string       `xml:"dhEvento"`
	CNPJAutor string       `xml:"CNPJAutor,omitempty"`
	CPFAutor  string       `xml:"CPFAutor,omitempty"`
	ChNFSe    string       `xml:"chNFSe"`
	E101101   *xmlMotivo   `xml:"e101101,omitempty"`
	E105102   *xmlSubstEvt `xml:"e105102,omitempty"`
	E101103   *xmlMotivo   `xml:"e101103,omitempty"`
	E202201   *xmlVazio    `xml:"e202201,omitempty"`
	E203202   *xmlVazio    `xml:"e203202,omitempty"`
	E204203   *xmlVazio    `xml:"e204203,omitempty"`
	E202205   *xmlMotivo   `xml:"e202205,omitempty"`
	E203206   *xmlMotivo   `xml:"e203206,omitempty"`
	E204207   *xmlMotivo   `xml:"e204207,omitempty"`
	E205208   *xmlAnulacao `xml:"e205208,omitempty"`
}

type xmlVazio struct{}

type xmlMotivo struct {
	CMotivo string `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo,omitempty"`
}

// Descrições fixas de xDesc (TE105102/TE205208) — enumeração de valor único
// no XSD, nunca preenchida pelo chamador.
const (
	xDescCancelamentoPorSubst = "Cancelamento de NFS-e por Substituição"
	xDescAnulacaoRejeicao     = "Manifestação de NFS-e - Anulação da Rejeição"
)

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

// eventsRequiringMotivo são os tipos cujo grupo específico tem cMotivo obrigatório.
var eventsRequiringMotivo = map[string]bool{
	nfse.EventCancelamento: true, nfse.EventCancelamentoPorSubst: true,
	nfse.EventSolicAnaliseFiscalCanc: true, nfse.EventRejeicaoPrestador: true,
	nfse.EventRejeicaoTomador: true, nfse.EventRejeicaoIntermediario: true,
}

// BuildPedRegEvento serializa o pedido de registro de evento, ainda SEM
// assinatura. Devolve o XML e o Id do infPedReg.
func BuildPedRegEvento(ev nfse.EventRequest) ([]byte, string, error) {
	if !nfse.ContribuinteEvents[ev.TipoEvento] {
		return nil, "", fmt.Errorf("nacional: evento %q não pode ser emitido pelo contribuinte", ev.TipoEvento)
	}
	if eventsRequiringMotivo[ev.TipoEvento] && (ev.Motivo == nil || ev.Motivo.Codigo == "") {
		return nil, "", fmt.Errorf("nacional: evento %q exige cMotivo", ev.TipoEvento)
	}
	if ev.CNPJAutor == "" && ev.CPFAutor == "" {
		return nil, "", fmt.Errorf("nacional: evento sem CNPJAutor nem CPFAutor")
	}
	if ev.TipoEvento == nfse.EventCancelamentoPorSubst && ev.ChSubstituta == "" {
		return nil, "", fmt.Errorf("nacional: evento %q exige chSubstituta", ev.TipoEvento)
	}
	if ev.TipoEvento == nfse.EventAnulacaoRejeicao {
		if ev.CPFAgTrib == "" || ev.IDEvManifRej == "" || ev.Motivo == nil || ev.Motivo.Descricao == "" {
			return nil, "", fmt.Errorf("nacional: evento %q exige CPFAgTrib, idEvManifRej e xMotivo", ev.TipoEvento)
		}
	}

	seq := ev.NSeqEvento
	if seq == 0 {
		seq = 1
	}
	id := idPedRegPrefix + ev.ChaveAcesso +
		leftPad(ev.TipoEvento, widthTipoEvento) +
		leftPad(fmt.Sprintf("%d", seq), widthNSeqEvento)

	inf := xmlInfPedReg{
		ID: id, TpAmb: ev.TpAmb, VerAplic: ev.VerAplic,
		DhEvento:  ev.DhEvento.UTC().Format(time.RFC3339),
		CNPJAutor: ev.CNPJAutor, CPFAutor: ev.CPFAutor, ChNFSe: ev.ChaveAcesso,
	}

	motivo := &xmlMotivo{}
	if ev.Motivo != nil {
		motivo = &xmlMotivo{CMotivo: ev.Motivo.Codigo, XMotivo: ev.Motivo.Descricao}
	}

	switch ev.TipoEvento {
	case nfse.EventCancelamento:
		inf.E101101 = motivo
	case nfse.EventCancelamentoPorSubst:
		inf.E105102 = &xmlSubstEvt{XDesc: xDescCancelamentoPorSubst,
			CMotivo: motivo.CMotivo, XMotivo: motivo.XMotivo, ChSubstituta: ev.ChSubstituta}
	case nfse.EventSolicAnaliseFiscalCanc:
		inf.E101103 = motivo
	case nfse.EventConfirmacaoPrestador:
		inf.E202201 = &xmlVazio{}
	case nfse.EventConfirmacaoTomador:
		inf.E203202 = &xmlVazio{}
	case nfse.EventConfirmacaoIntermediario:
		inf.E204203 = &xmlVazio{}
	case nfse.EventRejeicaoPrestador:
		inf.E202205 = motivo
	case nfse.EventRejeicaoTomador:
		inf.E203206 = motivo
	case nfse.EventRejeicaoIntermediario:
		inf.E204207 = motivo
	case nfse.EventAnulacaoRejeicao:
		inf.E205208 = &xmlAnulacao{XDesc: xDescAnulacaoRejeicao, CPFAgTrib: ev.CPFAgTrib,
			IDEvManifRej: ev.IDEvManifRej, XMotivo: motivo.XMotivo}
	}

	out, err := xml.Marshal(xmlPedRegEvento{
		Xmlns: nfse.Namespace, Versao: nfse.LayoutVersion, InfPedReg: inf,
	})
	if err != nil {
		return nil, "", fmt.Errorf("nacional: serializar pedRegEvento: %w", err)
	}
	return out, id, nil
}
