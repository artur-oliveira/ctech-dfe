package nfse

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/dfe/go-dfe/internal/constants"
)

// Chaves aceitas em dfe.Request.Body.
const (
	BodyKeyProvider     = "provider"
	BodyKeyDocument     = "document"
	BodyKeyEvent        = "event"
	BodyKeyAccessKey    = "chave_acesso"
	BodyKeyIDDPS        = "id_dps"
	BodyKeyNSU          = "nsu"
	BodyKeyCNPJConsulta = "cnpj_consulta"
	BodyKeyParamKind    = "param_kind"
	BodyKeyParamArgs    = "param_args"
)

// distributor, danfser e parametrizer são as capacidades que só o provider
// nacional tem. O ABRASF (F5) não as implementa e o dispatch falha
// explicitamente.
type distributor interface {
	Distribute(ctx context.Context, nsu int64, cnpjConsulta string, lote bool) (Result, error)
}

type danfser interface {
	DANFSE(ctx context.Context, chave string) ([]byte, error)
}

type parametrizer interface {
	MunicipalParameters(ctx context.Context, kind string, args ...string) (Result, error)
}

// Dispatch roteia o serviço para o método correspondente do provider.
func Dispatch(ctx context.Context, p Provider, service string, body map[string]any) (Result, error) {
	switch service {
	case constants.ServiceNFSeRecepcao:
		sub, err := subMap(body, BodyKeyDocument)
		if err != nil {
			return Result{}, err
		}
		doc, err := DecodeDocument(sub)
		if err != nil {
			return Result{}, err
		}
		return p.Emit(ctx, doc)

	case constants.ServiceNFSeEvento:
		sub, err := subMap(body, BodyKeyEvent)
		if err != nil {
			return Result{}, err
		}
		ev, err := DecodeEventRequest(sub)
		if err != nil {
			return Result{}, err
		}
		return p.Event(ctx, ev)

	case constants.ServiceNFSeConsulta:
		return p.QueryByKey(ctx, str(body, BodyKeyAccessKey))

	case constants.ServiceNFSeConsultaDPS:
		return p.QueryByDPSID(ctx, str(body, BodyKeyIDDPS))

	case constants.ServiceNFSeConsultaEvento:
		return p.QueryEvents(ctx, EventFilter{
			ChaveAcesso: str(body, BodyKeyAccessKey),
			TipoEvento:  str(body, "tipo_evento"),
			NSeqEvento:  intOf(body, "n_seq_evento"),
		})

	case constants.ServiceNFSeDistribuicao:
		d, ok := p.(distributor)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "distribuicao"}
		}
		return d.Distribute(ctx, int64(intOf(body, BodyKeyNSU)), str(body, BodyKeyCNPJConsulta), true)

	case constants.ServiceNFSeDANFSE:
		d, ok := p.(danfser)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "danfse"}
		}
		pdf, err := d.DANFSE(ctx, str(body, BodyKeyAccessKey))
		if err != nil {
			return Result{}, err
		}
		return Result{PDF: pdf}, nil

	case constants.ServiceNFSeParametrosMunicipais:
		pm, ok := p.(parametrizer)
		if !ok {
			return Result{}, &FieldNotSupportedError{Provider: fmt.Sprintf("%T", p), Field: "parametros_municipais"}
		}
		return pm.MunicipalParameters(ctx, str(body, BodyKeyParamKind), strSlice(body, BodyKeyParamArgs)...)

	default:
		return Result{}, fmt.Errorf("nfse: serviço desconhecido %q", service)
	}
}

func subMap(body map[string]any, key string) (map[string]any, error) {
	v, ok := body[key].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("nfse: body sem o objeto %q", key)
	}
	return v, nil
}

func str(body map[string]any, key string) string {
	s, _ := body[key].(string)
	return s
}

func intOf(body map[string]any, key string) int {
	switch v := body[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func strSlice(body map[string]any, key string) []string {
	raw, _ := body[key].([]any)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// DecodeEventRequest decodifica o submapa "event" do Body com a mesma política
// de campo desconhecido de DecodeDocument. Exportada para a api validar o
// pedido que ela mesma monta antes de enfileirar.
func DecodeEventRequest(m map[string]any) (EventRequest, error) {
	raw, err := jsonMarshal(m)
	if err != nil {
		return EventRequest{}, err
	}
	var ev EventRequest
	if err := jsonUnmarshalStrict(raw, &ev); err != nil {
		return EventRequest{}, fmt.Errorf("nfse: decode event: %w", err)
	}
	return ev, nil
}
