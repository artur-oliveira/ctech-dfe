// Package ciot is the ANTT CIOT (Pagamento Eletronico de Frete) client: REST+JSON over the mTLS transport, the
// identifier generator, a local validator, the rejection table and the freight floor calculator.
// Contract: docs/specs/2026-11-04-ciot-antt-pef-integracao.md. Origins tagged [PDF], [pratica], [proposta] there.
package ciot

import "time"

// Environments, as dfe.Request.Environment carries them.
const (
	EnvProd = "prod"
	EnvHom  = "hom"
)

// Hosts [PDF]. Never mixed: HostFor fails for any other environment.
const (
	HostHomologacao = "https://appservices-hml.antt.gov.br/pefServices"
	HostProducao    = "https://appservices.antt.gov.br/pefServices"
)

// Service names [PDF]; the path is {host}/api/{Service} [pratica].
const (
	ServiceSituacaoTransportador = "ConsultarSituacaoTransportador"
	ServiceFrotaTransportador    = "ConsultarFrotaTransportador"
	ServiceDeclaracao            = "DeclaracaoOperacaoTransporte"
	ServiceCancelamento          = "CancelamentoOperacaoTransporte"
	ServiceRetificacao           = "RetificacaoOperacaoTransporte"
	ServiceEncerramento          = "EncerramentoOperacaoTransporte"
	ServiceExcecao               = "ConsultarExcecao"
	ServiceCIOTGerado            = "ConsultarCIOTGerado"
	// ServiceGerarIdentificador is the generator operation (token + gerar), not an ANTT PEF service.
	ServiceGerarIdentificador = "GerarIdentificador"
)

// Services lists the 8 PEF services plus the generator operation (the dfe.Implements allowlist).
var Services = []string{
	ServiceSituacaoTransportador, ServiceFrotaTransportador, ServiceDeclaracao, ServiceCancelamento,
	ServiceRetificacao, ServiceEncerramento, ServiceExcecao, ServiceCIOTGerado, ServiceGerarIdentificador,
}

const (
	apiPathPrefix = "/api/"
	pathToken     = "/token"
	pathGerar     = "/gerar"
	headerChave   = "Chave"
	queryCarrier  = "CPFCNPJTransportador" // [PDF] example of ConsultarExcecao
)

// Success codes of the ANTT response [PDF].
const (
	CodeInserted = 110
	CodeQueried  = 111
	// CodeDuplicateID is B17: the identifier was already declared; reconcile with ConsultarCIOTGerado.
	CodeDuplicateID = 219
)

// Timeouts and retries (spec section 10). The 5 s connect timeout is certificate.Load's dialer.
const (
	DeclarationTimeout = 30 * time.Second
	QueryTimeout       = 15 * time.Second
	GeneratorTimeout   = 30 * time.Second
	QueryRetries       = 2
)

// Generator token cache (spec section 4).
const (
	TokenLifetime     = time.Hour
	TokenRenewMargin  = 10 * time.Minute
	PlaceholderAPIKey = "CHANGE_ME"
)

// SSM parameter names (spec section 4); the secret provider resolves them, the library never reads AWS.
const (
	ParamAPIKey      = "generator-api-key"
	ParamFallbackURL = "generator-fallback-url"
)

// ParamName returns /ctech-dfe/{env}/ciot/{name}.
func ParamName(env, name string) string { return "/ctech-dfe/" + env + "/ciot/" + name }

// Formats [PDF].
const (
	DateLayout     = "2006-01-02"
	DateTimeLayout = "2006-01-02T15:04:05"
	// BrasiliaOffsetSeconds is UTC-3, fixed since 2019 (no DST).
	BrasiliaOffsetSeconds = -3 * 3600
	CIOTLen               = 16 // 12 identifier characters + 4 verifier digits
	IDLen                 = 12
)

// Brasilia is the fixed UTC-3 zone ANTT dates use.
var Brasilia = time.FixedZone("BRT", BrasiliaOffsetSeconds)
