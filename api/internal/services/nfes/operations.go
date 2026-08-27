package nfes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Campos da natureza de operação lidos na emissão.
const (
	opFieldNatOp      = "nat_op"
	opFieldTpNF       = "tp_nf"
	opFieldFinNFe     = "fin_nfe"
	opFieldIndFinal   = "ind_final"
	opFieldIndPres    = "ind_pres"
	opFieldCfopSuffix = "cfop_suffix"
	opFieldModFrete   = "mod_frete"
	opFieldInfAdFisco = "inf_ad_fisco"
	opFieldInfCpl     = "inf_cpl"
	opFieldVolEsp     = "vol_esp"
	opFieldVolMarca   = "vol_marca"
	opFieldObsCont    = "obs_cont"
	opFieldObsFisco   = "obs_fisco"
)

// loadOperation carrega a natureza de operação referenciada na emissão.
// Devolve nil quando nenhuma foi informada — emitir sem operação continua
// sendo o caminho válido que sempre foi.
func loadOperation(
	ctx context.Context, repo *repositories.OperationRepository, orgPK string, operationID *string,
) (map[string]any, error) {
	if operationID == nil || *operationID == "" {
		return nil, nil
	}
	item, err := repo.Get(ctx, orgPK, *operationID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("natureza de operação não encontrada: " + *operationID)
	}
	var op map[string]any
	if err := attributevalue.UnmarshalMap(item, &op); err != nil {
		return nil, problem.InternalServer("failed to decode operation")
	}
	return op, nil
}

// operationDefault devolve o valor da operação para um campo, ou nil quando a
// operação não o define.
//
// A escada é sempre a mesma: **valor explícito no request vence a operação**.
// A operação é default, não prisão — trocar o indicador de presença de uma nota
// não pode exigir criar outra operação.
func operationDefault(op map[string]any, field string) *string {
	if op == nil {
		return nil
	}
	v, ok := op[field].(string)
	if !ok || v == "" {
		return nil
	}
	return &v
}

// firstNonNil devolve o primeiro ponteiro não nulo — o request na frente,
// a operação atrás.
func firstNonNil(values ...*string) *string {
	for _, v := range values {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}

// resolveItemCFOP devolve o CFOP de um item: o explícito quando veio no
// request, senão o resolvido a partir da natureza fiscal da operação e das UFs.
//
// Sem CFOP e sem operação, a mensagem de erro é a mesma de sempre — quem já
// emitia assim não vê diferença nenhuma.
func resolveItemCFOP(itemCFOP string, op map[string]any, emitUF, destUF string) (string, error) {
	if itemCFOP != "" {
		return itemCFOP, nil
	}
	suffix := operationDefault(op, opFieldCfopSuffix)
	if suffix == nil {
		return "", problem.BadRequest("cfop é obrigatório em cada item, ou informe uma natureza de operação com cfop_suffix")
	}
	return services.ResolveCFOPScope(*suffix, emitUF, destUF)
}

// interpolateOperationText aplica os placeholders do texto fiscal da operação.
// Texto vazio devolve nil para não gravar um campo em branco no documento.
func interpolateOperationText(op map[string]any, field string, vars map[string]string) (*string, error) {
	tpl := operationDefault(op, field)
	if tpl == nil {
		return nil, nil
	}
	out, err := services.Interpolate(*tpl, vars)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return &out, nil
}

// resolveNfceOperation devolve a operação da NFC-e: a informada no request ou,
// na ausência dela, a operação padrão da organização.
//
// É o que permite a venda de balcão continuar sendo uma tela só — o único
// acréscimo aceitável a uma tela de balcão é nada.
func (s *NfceService) resolveNfceOperation(ctx context.Context, orgPK string, operationID *string) (map[string]any, error) {
	if operationID != nil && *operationID != "" {
		return loadOperation(ctx, s.operationRepo, orgPK, operationID)
	}
	defaults, err := s.operationRepo.ListDefaults(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(defaults) == 0 {
		return nil, nil
	}
	var op map[string]any
	if err := attributevalue.UnmarshalMap(defaults[0], &op); err != nil {
		return nil, problem.InternalServer("failed to decode default operation")
	}
	return op, nil
}

// operationObs traduz obs_cont / obs_fisco da operação para os nós de infAdic,
// interpolando os mesmos placeholders de inf_cpl. Uma observação cujo texto não
// interpola é descartada — texto vazio é rejeição no XSD.
func operationObs(op map[string]any, field string, vars map[string]string) []map[string]any {
	raw, _ := op[field].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		campo := anyStr(m, "x_campo", "")
		texto, err := services.Interpolate(anyStr(m, "x_texto", ""), vars)
		if err != nil || campo == "" || texto == "" {
			continue
		}
		out = append(out, map[string]any{"@xCampo": campo, "xTexto": texto})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
