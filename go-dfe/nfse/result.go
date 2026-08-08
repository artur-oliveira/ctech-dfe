package nfse

import (
	"encoding/json"
	"time"
)

// Message é uma mensagem de processamento do fisco (MensagemProcessamento no
// Swagger do Sefin Nacional, tmp/nfse-sefin.json).
type Message struct {
	Codigo      string `json:"codigo"`
	Descricao   string `json:"descricao"`
	Complemento string `json:"complemento,omitempty"`
}

// UnmarshalJSON aceita tanto MensagemProcessamento ("descricao") quanto o
// envelope NFSePostResponseErro usado por autorizadores municipais
// ("mensagem"). O modelo neutro mantém Descricao como campo canônico para que
// o restante da stack não dependa da variante do provider.
func (m *Message) UnmarshalJSON(data []byte) error {
	type messageJSON struct {
		Codigo      string `json:"codigo"`
		Descricao   string `json:"descricao"`
		Mensagem    string `json:"mensagem"`
		Complemento string `json:"complemento"`
	}

	var raw messageJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Codigo = raw.Codigo
	m.Descricao = raw.Descricao
	if m.Descricao == "" {
		m.Descricao = raw.Mensagem
	}
	m.Complemento = raw.Complemento
	return nil
}

// Result é o retorno neutro de qualquer operação. Campos não pertinentes à
// operação ficam vazios.
type Result struct {
	ChaveAcesso           string             `json:"chave_acesso,omitempty"`
	IDDPS                 string             `json:"id_dps,omitempty"`
	NFSeXML               string             `json:"nfse_xml,omitempty"`
	DPSXML                string             `json:"dps_xml,omitempty"`
	EventoXML             string             `json:"evento_xml,omitempty"`
	PedRegEventoXML       string             `json:"ped_reg_evento_xml,omitempty"`
	Ambiente              int                `json:"ambiente,omitempty"`
	VersaoAplicativo      string             `json:"versao_aplicativo,omitempty"`
	DataHoraProcessamento string             `json:"data_hora_processamento,omitempty"`
	Alertas               []Message          `json:"alertas,omitempty"`
	Erros                 []Message          `json:"erros,omitempty"`
	Distribuicao          []DistributionItem `json:"distribuicao,omitempty"`
	StatusDistribuicao    string             `json:"status_distribuicao,omitempty"`
	PDF                   []byte             `json:"pdf,omitempty"`
	Parametros            map[string]any     `json:"parametros,omitempty"`
}

type DistributionItem struct {
	NSU             int64  `json:"nsu"`
	ChaveAcesso     string `json:"chave_acesso,omitempty"`
	TipoDocumento   string `json:"tipo_documento,omitempty"`
	TipoEvento      string `json:"tipo_evento,omitempty"`
	XML             string `json:"xml,omitempty"`
	DataHoraGeracao string `json:"data_hora_geracao,omitempty"`
}

type EventMotivo struct {
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao,omitempty"`
}

type EventRequest struct {
	ChaveAcesso  string       `json:"chave_acesso"`
	TipoEvento   string       `json:"tipo_evento"`
	NSeqEvento   int          `json:"n_seq_evento"`
	TpAmb        int          `json:"tp_amb"`
	VerAplic     string       `json:"ver_aplic"`
	DhEvento     time.Time    `json:"dh_evento"`
	CNPJAutor    string       `json:"cnpj_autor,omitempty"`
	CPFAutor     string       `json:"cpf_autor,omitempty"`
	Motivo       *EventMotivo `json:"motivo,omitempty"`
	ChSubstituta string       `json:"ch_substituta,omitempty"`
	CPFAgTrib    string       `json:"cpf_ag_trib,omitempty"`
	IDEvManifRej string       `json:"id_ev_manif_rej,omitempty"`
}

type EventFilter struct {
	ChaveAcesso string `json:"chave_acesso"`
	TipoEvento  string `json:"tipo_evento,omitempty"`
	NSeqEvento  int    `json:"n_seq_evento,omitempty"`
}
