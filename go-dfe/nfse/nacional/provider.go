package nacional

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Nomes de campo dos envelopes JSON do Sefin Nacional (tmp/nfse-sefin.json).
const (
	fieldDpsXMLGZipB64       = "dpsXmlGZipB64"
	fieldPedRegEvtXMLGZipB64 = "pedidoRegistroEventoXmlGZipB64"
)

// Config configura o provider nacional. Cert/Key podem ser nil apenas em
// teste — sem eles a DPS segue sem assinatura e o fisco rejeita.
type Config struct {
	Environment string
	HTTPClient  *http.Client
	Cert        *x509.Certificate
	Key         *rsa.PrivateKey
	MaxRetries  int
	CNPJ        string
	Now         func() time.Time
}

// Nacional implementa nfse.Provider contra o Sistema Nacional NFS-e.
type Nacional struct {
	cfg Config
	// baseOverride substitui as bases resolvidas; usado só pelos testes,
	// que apontam todos os sistemas para um httptest.Server.
	baseOverride map[string]string
}

// New valida a configuração e devolve o provider.
func New(cfg Config) (*Nacional, error) {
	if cfg.HTTPClient == nil {
		return nil, fmt.Errorf("nacional: HTTPClient é obrigatório (mTLS)")
	}
	if _, err := ResolveBase(SystemSefin, cfg.Environment); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Nacional{cfg: cfg}, nil
}

func (n *Nacional) base(system string) (string, error) {
	if n.baseOverride != nil {
		if b, ok := n.baseOverride[system]; ok {
			return b, nil
		}
	}
	return ResolveBase(system, n.cfg.Environment)
}

// emitResponse cobre NFSePostResponseSucesso.
type emitResponse struct {
	TipoAmbiente          int            `json:"tipoAmbiente"`
	VersaoAplicativo      string         `json:"versaoAplicativo"`
	DataHoraProcessamento string         `json:"dataHoraProcessamento"`
	IDDps                 string         `json:"idDps"`
	ChaveAcesso           string         `json:"chaveAcesso"`
	NFSeXMLGZipB64        string         `json:"nfseXmlGZipB64"`
	Alertas               []nfse.Message `json:"alertas"`
}

// Emit monta a DPS, assina, comprime e envia. O POST é síncrono: a resposta
// 201 já traz a NFS-e gerada.
func (n *Nacional) Emit(ctx context.Context, doc nfse.Document) (nfse.Result, error) {
	raw, idDPS, err := BuildDPS(doc, n.cfg.Now())
	if err != nil {
		return nfse.Result{}, err
	}
	if n.cfg.Key != nil {
		raw, err = SignDPS(raw, n.cfg.Cert, n.cfg.Key)
		if err != nil {
			return nfse.Result{}, fmt.Errorf("nacional: assinar DPS: %w", err)
		}
	}
	packed, err := GzipB64(raw)
	if err != nil {
		return nfse.Result{}, err
	}

	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp emitResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodPost, base+PathNFSe,
		map[string]string{fieldDpsXMLGZipB64: packed}, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}

	res := nfse.Result{
		ChaveAcesso: resp.ChaveAcesso, IDDPS: resp.IDDps,
		DPSXML: string(raw), Ambiente: resp.TipoAmbiente,
		VersaoAplicativo:      resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
		Alertas:               resp.Alertas,
	}
	if res.IDDPS == "" {
		res.IDDPS = idDPS
	}
	if resp.NFSeXMLGZipB64 != "" {
		nfseXML, err := UngzipB64(resp.NFSeXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.NFSeXML = string(nfseXML)
	}
	return res, nil
}

type eventResponse struct {
	TipoAmbiente          int    `json:"tipoAmbiente"`
	VersaoAplicativo      string `json:"versaoAplicativo"`
	DataHoraProcessamento string `json:"dataHoraProcessamento"`
	EventoXMLGZipB64      string `json:"eventoXmlGZipB64"`
}

// Event envia o pedido de registro de evento e devolve o evento gerado.
func (n *Nacional) Event(ctx context.Context, ev nfse.EventRequest) (nfse.Result, error) {
	raw, _, err := BuildPedRegEvento(ev)
	if err != nil {
		return nfse.Result{}, err
	}
	if n.cfg.Key != nil {
		raw, err = SignPedRegEvento(raw, n.cfg.Cert, n.cfg.Key)
		if err != nil {
			return nfse.Result{}, fmt.Errorf("nacional: assinar pedRegEvento: %w", err)
		}
	}
	packed, err := GzipB64(raw)
	if err != nil {
		return nfse.Result{}, err
	}
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}

	var resp eventResponse
	url := base + fmt.Sprintf(PathEventos, ev.ChaveAcesso)
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodPost, url,
		map[string]string{fieldPedRegEvtXMLGZipB64: packed}, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}

	res := nfse.Result{
		ChaveAcesso: ev.ChaveAcesso, PedRegEventoXML: string(raw),
		Ambiente: resp.TipoAmbiente, VersaoAplicativo: resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
	}
	if resp.EventoXMLGZipB64 != "" {
		evXML, err := UngzipB64(resp.EventoXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.EventoXML = string(evXML)
	}
	return res, nil
}

type queryResponse struct {
	TipoAmbiente          int    `json:"tipoAmbiente"`
	VersaoAplicativo      string `json:"versaoAplicativo"`
	DataHoraProcessamento string `json:"dataHoraProcessamento"`
	ChaveAcesso           string `json:"chaveAcesso"`
	IDDps                 string `json:"idDps"`
	NFSeXMLGZipB64        string `json:"nfseXmlGZipB64"`
	EventoXMLGZipB64      string `json:"eventoXmlGZipB64"`
}

// QueryByKey consulta a NFS-e pela chave de acesso.
func (n *Nacional) QueryByKey(ctx context.Context, key string) (nfse.Result, error) {
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp queryResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathNFSeByKey, key), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return n.toResult(resp)
}

// QueryByDPSID recupera a chave de acesso a partir do identificador da DPS —
// é o caminho de recuperação em retry (spec §3.4).
func (n *Nacional) QueryByDPSID(ctx context.Context, idDPS string) (nfse.Result, error) {
	base, err := n.base(SystemSefin)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp queryResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathDPS, idDPS), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return n.toResult(resp)
}

// QueryEvents busca um evento específico no Sefin quando tipo e sequencial
// vêm preenchidos; caso contrário lista todos pelo ADN, porque o Sefin não
// expõe listagem completa (tmp/nfse-sefin.json vs tmp/nfse-adn-contribuintes.json).
// A listagem sem filtro é implementada na Task 7 (adn.go).
func (n *Nacional) QueryEvents(ctx context.Context, f nfse.EventFilter) (nfse.Result, error) {
	if f.TipoEvento != "" && f.NSeqEvento > 0 {
		base, err := n.base(SystemSefin)
		if err != nil {
			return nfse.Result{}, err
		}
		var resp queryResponse
		url := base + fmt.Sprintf(PathEventoEspecifico, f.ChaveAcesso, f.TipoEvento, f.NSeqEvento)
		if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, url, nil, &resp, n.cfg.MaxRetries); err != nil {
			return nfse.Result{}, err
		}
		return n.toResult(resp)
	}
	// listEventsADN é implementado na Task 7 (adn.go); até lá, filtro vazio
	// não é suportado — mantém a suíte verde entre commits.
	return nfse.Result{}, fmt.Errorf("nacional: listagem de eventos sem filtro exige ADN (ainda não implementado)")
}

func (n *Nacional) toResult(resp queryResponse) (nfse.Result, error) {
	res := nfse.Result{
		ChaveAcesso: resp.ChaveAcesso, IDDPS: resp.IDDps,
		Ambiente: resp.TipoAmbiente, VersaoAplicativo: resp.VersaoAplicativo,
		DataHoraProcessamento: resp.DataHoraProcessamento,
	}
	if resp.NFSeXMLGZipB64 != "" {
		x, err := UngzipB64(resp.NFSeXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.NFSeXML = string(x)
	}
	if resp.EventoXMLGZipB64 != "" {
		x, err := UngzipB64(resp.EventoXMLGZipB64)
		if err != nil {
			return res, err
		}
		res.EventoXML = string(x)
	}
	return res, nil
}
