package nacional

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// Tipos de consulta de parâmetros municipais.
const (
	ParamAliquota         = "aliquota"
	ParamConvenio         = "convenio"
	ParamBeneficio        = "beneficio"
	ParamRegimesEspeciais = "regimes_especiais"
	ParamRetencoes        = "retencoes"
)

// paramArity é quantos argumentos além do tipo cada consulta exige.
var paramArity = map[string]int{
	ParamAliquota: 3, ParamConvenio: 1, ParamBeneficio: 3,
	ParamRegimesEspeciais: 3, ParamRetencoes: 2,
}

// adnMessage é o MensagemProcessamento do ADN — PascalCase, diferente do
// Sefin (tmp/nfse-adn-contribuintes.json).
type adnMessage struct {
	Codigo      string `json:"Codigo"`
	Descricao   string `json:"Descricao"`
	Complemento string `json:"Complemento"`
}

type adnDistribuicaoItem struct {
	NSU             int64  `json:"NSU"`
	ChaveAcesso     string `json:"ChaveAcesso"`
	TipoDocumento   string `json:"TipoDocumento"`
	TipoEvento      string `json:"TipoEvento"`
	ArquivoXml      string `json:"ArquivoXml"`
	DataHoraGeracao string `json:"DataHoraGeracao"`
}

type adnLoteResponse struct {
	StatusProcessamento   string                `json:"StatusProcessamento"`
	LoteDFe               []adnDistribuicaoItem `json:"LoteDFe"`
	Alertas               []adnMessage          `json:"Alertas"`
	Erros                 []adnMessage          `json:"Erros"`
	TipoAmbiente          int                   `json:"TipoAmbiente"`
	VersaoAplicativo      string                `json:"VersaoAplicativo"`
	DataHoraProcessamento string                `json:"DataHoraProcessamento"`
}

func toNfseMessages(in []adnMessage) []nfse.Message {
	if len(in) == 0 {
		return nil
	}
	out := make([]nfse.Message, 0, len(in))
	for _, m := range in {
		out = append(out, nfse.Message{Codigo: m.Codigo, Descricao: m.Descricao, Complemento: m.Complemento})
	}
	return out
}

func (r adnLoteResponse) toResult() nfse.Result {
	res := nfse.Result{
		StatusDistribuicao: r.StatusProcessamento, Ambiente: r.TipoAmbiente,
		VersaoAplicativo: r.VersaoAplicativo, DataHoraProcessamento: r.DataHoraProcessamento,
		Alertas: toNfseMessages(r.Alertas), Erros: toNfseMessages(r.Erros),
	}
	for _, it := range r.LoteDFe {
		res.Distribuicao = append(res.Distribuicao, nfse.DistributionItem{
			NSU: it.NSU, ChaveAcesso: it.ChaveAcesso, TipoDocumento: it.TipoDocumento,
			TipoEvento: it.TipoEvento, XML: it.ArquivoXml, DataHoraGeracao: it.DataHoraGeracao,
		})
	}
	return res
}

// Distribute busca documentos fiscais de serviço a partir de um NSU.
// cnpjConsulta permite consultar outro CNPJ da mesma raiz do certificado.
func (n *Nacional) Distribute(ctx context.Context, nsu int64, cnpjConsulta string, lote bool) (nfse.Result, error) {
	base, err := n.base(SystemADN)
	if err != nil {
		return nfse.Result{}, err
	}
	url := base + fmt.Sprintf(PathDistribuicaoNSU, nsu) + fmt.Sprintf("?lote=%t", lote)
	if cnpjConsulta != "" {
		url += "&cnpjConsulta=" + cnpjConsulta
	}
	var resp adnLoteResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, url, nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return resp.toResult(), nil
}

// listEventsADN lista todos os eventos de uma chave. É o caminho usado por
// QueryEvents sem tipo/sequencial — o Sefin não expõe listagem completa.
func (n *Nacional) listEventsADN(ctx context.Context, chave string) (nfse.Result, error) {
	base, err := n.base(SystemADN)
	if err != nil {
		return nfse.Result{}, err
	}
	var resp adnLoteResponse
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet,
		base+fmt.Sprintf(PathEventosADN, chave), nil, &resp, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	res := resp.toResult()
	res.ChaveAcesso = chave
	return res, nil
}

// DANFSE baixa o PDF da NFS-e. Resposta binária, não JSON.
func (n *Nacional) DANFSE(ctx context.Context, chave string) ([]byte, error) {
	base, err := n.base(SystemDANFSE)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+fmt.Sprintf(PathDANFSE, chave), nil)
	if err != nil {
		return nil, fmt.Errorf("nacional: build request DANFSE: %w", err)
	}
	req.Header.Set("Accept", "application/pdf")
	resp, err := n.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nacional: DANFSE: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("nacional: ler DANFSE: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, toFiscalError(resp.StatusCode, body)
	}
	return body, nil
}

// MunicipalParameters consulta a parametrização do município. args segue a
// ordem do path de cada tipo:
//
//	aliquota          -> município, serviço, competência
//	convenio          -> município
//	beneficio         -> município, número do benefício, competência
//	regimes_especiais -> município, serviço, competência
//	retencoes         -> município, competência
func (n *Nacional) MunicipalParameters(ctx context.Context, kind string, args ...string) (nfse.Result, error) {
	want, ok := paramArity[kind]
	if !ok {
		return nfse.Result{}, fmt.Errorf("nacional: tipo de parâmetro municipal desconhecido %q", kind)
	}
	if len(args) != want {
		return nfse.Result{}, fmt.Errorf("nacional: %q exige %d argumentos, recebeu %d", kind, want, len(args))
	}

	base, err := n.base(SystemParametros)
	if err != nil {
		return nfse.Result{}, err
	}
	var path string
	switch kind {
	case ParamAliquota:
		path = fmt.Sprintf(PathParamAliquota, args[0], args[1], args[2])
	case ParamConvenio:
		path = fmt.Sprintf(PathParamConvenio, args[0])
	case ParamBeneficio:
		path = fmt.Sprintf(PathParamBeneficio, args[0], args[1], args[2])
	case ParamRegimesEspeciais:
		path = fmt.Sprintf(PathParamRegimesEspeciais, args[0], args[1], args[2])
	case ParamRetencoes:
		path = fmt.Sprintf(PathParamRetencoes, args[0], args[1])
	}

	var out map[string]any
	if _, err := httpDo(ctx, n.cfg.HTTPClient, http.MethodGet, base+path, nil, &out, n.cfg.MaxRetries); err != nil {
		return nfse.Result{}, err
	}
	return nfse.Result{Parametros: out}, nil
}
