package services

// payment_link.go: vinculação de pagamento (110300, evVincPgto) e seu cancelamento (110301, evCancVincPgto) da NFCom e do BP-e
// (NT 2026.001; XSD vendorizados em go-dfe/schemas). O leiaute é o mesmo do CT-e (ctes/payment_events.go); o que muda é o
// tamanho do protocolo (NFCom 16 dígitos, BP-e 15). Quem pode vincular, prazos e a tabela de tpMeioPgto não constam de
// documento do repositório: a API não restringe o autor e deixa o autorizador conferir.

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	TpEventoVinculoPagamento     = "110300"
	TpEventoCancVinculoPagamento = "110301"

	// descEvento: o primeiro literal da enumeração do XSD (o segundo é o mesmo texto sem acento).
	descVinculoPagamento     = "Vinculação do Pagamento"
	descCancVinculoPagamento = "Cancelamento da vinculação do pagamento"

	maxPaymentNumber = 990 // nPag: 1 a 990

	// MaxEventSequence é o teto de nSeqEvento (1 a 20).
	MaxEventSequence = 20

	paymentEventStatusSuccess = "success"
)

var (
	paymentTxRe     = regexp.MustCompile(`^[!-ÿ][ -ÿ]*[!-ÿ]$`)  // idTransacao, 2 a 35 caracteres
	paymentMethodRe = regexp.MustCompile(`^\d{2}$`)             // tpMeioPgto
	paymentCNPJRe   = regexp.MustCompile(`^[A-Z0-9]{12}\d{2}$`) // CNPJReceb
	paymentBaseRe   = regexp.MustCompile(`^[A-Z0-9]{8}$`)       // CNPJBasePSP
)

// PaymentLink é o corpo do 110300 (pgto: atributos nPag e idTransacao; filhos tpMeioPgto, CNPJReceb, CNPJBasePSP).
type PaymentLink struct {
	Number        int    `json:"number" validate:"required,gte=1,lte=990"`        // nPag
	TransactionID string `json:"transaction_id" validate:"required,min=2,max=35"` // idTransacao
	PaymentMethod string `json:"payment_method" validate:"required,len=2"`        // tpMeioPgto
	ReceiverCNPJ  string `json:"receiver_cnpj" validate:"required,len=14"`        // CNPJReceb
	PSPBaseCNPJ   string `json:"psp_base_cnpj" validate:"required,len=8"`         // CNPJBasePSP
}

// Check confere o que as tags não alcançam; o erro é 422.
func (p PaymentLink) Check() error {
	n := len([]rune(p.TransactionID))
	var msg string
	switch {
	case p.Number < 1 || p.Number > maxPaymentNumber:
		msg = fmt.Sprintf("number vai de 1 a %d", maxPaymentNumber)
	case n < 2 || n > 35 || !paymentTxRe.MatchString(p.TransactionID):
		msg = "transaction_id de 2 a 35 caracteres, sem espaço no início ou no fim"
	case !paymentMethodRe.MatchString(p.PaymentMethod):
		msg = "payment_method com 2 dígitos"
	case !paymentCNPJRe.MatchString(p.ReceiverCNPJ):
		msg = "receiver_cnpj com 14 caracteres"
	case !paymentBaseRe.MatchString(p.PSPBaseCNPJ):
		msg = "psp_base_cnpj com 8 caracteres"
	}
	if msg != "" {
		return problem.UnprocessableEntity(msg)
	}
	return nil
}

// PaymentLinkDet é o detEvento do 110300 sobre o protocolo do documento.
func PaymentLinkDet(p PaymentLink, nProt string) map[string]any {
	return map[string]any{"evVincPgto": map[string]any{
		"descEvento": descVinculoPagamento, "nProt": nProt,
		"pgto": map[string]any{
			"@nPag": fmt.Sprintf("%d", p.Number), "@idTransacao": p.TransactionID,
			"tpMeioPgto": p.PaymentMethod, "CNPJReceb": p.ReceiverCNPJ, "CNPJBasePSP": p.PSPBaseCNPJ,
		},
	}}
}

// CancelPaymentLinkDet é o detEvento do 110301: protocolo do documento e protocolo do 110300 cancelado.
func CancelPaymentLinkDet(nProt, linkProt string) map[string]any {
	return map[string]any{"evCancVincPgto": map[string]any{
		"descEvento": descCancVinculoPagamento, "nProt": nProt, "nProtVincPgto": linkProt,
	}}
}

// NextEventSequence devolve o nSeqEvento: o pedido, ou o maior já gravado do tipo mais um (cada vinculação tem o seu).
func NextEventSequence(ctx context.Context, repo *repositories.DocumentEventRepository, accessKey, eventType string, requested int) (int, error) {
	if requested > 0 {
		if requested > MaxEventSequence {
			return 0, problem.UnprocessableEntity(fmt.Sprintf("o número sequencial do evento vai de 1 a %d", MaxEventSequence))
		}
		return requested, nil
	}
	res, err := repo.GetDocumentEvents(ctx, accessKey, 200, nil)
	if err != nil {
		return 0, err
	}
	maxSeq := 0
	for _, ev := range res.Items {
		if paymentAttrS(ev, "event_type") != eventType {
			continue
		}
		if v, ok := ev["sequence_number"].(*types.AttributeValueMemberN); ok {
			var n int
			if _, err := fmt.Sscanf(v.Value, "%d", &n); err == nil {
				maxSeq = max(maxSeq, n)
			}
		}
	}
	if maxSeq >= MaxEventSequence {
		return 0, problem.UnprocessableEntity(fmt.Sprintf("limite de %d eventos deste tipo atingido", MaxEventSequence))
	}
	return maxSeq + 1, nil
}

// AuthorizedEventProtocol devolve o protocolo do evento eventSK, que precisa ser do tipo want e estar autorizado. O protocolo
// vem do registro, nunca é digitado. protocolRe é o formato do documento (NFCom 16 dígitos, BP-e 15).
func AuthorizedEventProtocol(ctx context.Context, repo *repositories.DocumentEventRepository, accessKey, eventSK, want string, protocolRe *regexp.Regexp) (string, error) {
	target, err := repo.GetEvent(ctx, accessKey, eventSK)
	if err != nil {
		return "", err
	}
	if target == nil {
		return "", problem.NotFound("evento não encontrado")
	}
	prot := paymentAttrS(target, "sefaz_protocol")
	if paymentAttrS(target, "event_type") != want || paymentAttrS(target, "status") != paymentEventStatusSuccess || !protocolRe.MatchString(prot) {
		return "", problem.UnprocessableEntity("o evento escolhido não é um evento " + want + " autorizado")
	}
	return prot, nil
}

func paymentAttrS(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
