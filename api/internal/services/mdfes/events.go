package mdfes

import (
	"context"
	"fmt"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// eventContext bundles the data needed to dispatch a SEFAZ event for an MDF-e.
type eventContext struct {
	mdfe        map[string]types.AttributeValue
	pk          string
	environment int
	cnpj        string
	isPJ        bool
	emitUF      string
	sefazEnv    string
	certS3Key   string
	certPass    string
}

// emitUFFromAccessKey derives the 2-letter emitter UF from an MDF-e access key.
// The key embeds the numeric IBGE cUF code in positions 0-1 (e.g. "22"); py-dfe's
// endpoint resolver keys on the UF abbreviation (e.g. "PI"), so the code must be
// translated. Without this, SEFAZ event dispatch fails with a KeyError on the
// numeric code.
func emitUFFromAccessKey(accessKey string) string {
	if len(accessKey) < 2 {
		return ""
	}
	return services.UFFromCode[accessKey[0:2]]
}

// resolveEventContext loads the MDF-e, validates it is authorized, and resolves
// the org certificate + environment needed to dispatch a SEFAZ event.
func (s *MdfeService) resolveEventContext(ctx context.Context, orgPK, accessKey string) (*eventContext, error) {
	mdfe, err := s.GetMDFe(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	if mdfe == nil {
		return nil, ErrMDFeNotFound
	}
	if strAttr(mdfe, "status") != StatusAuthorized {
		return nil, problem.BadRequest("apenas MDF-e autorizados permitem esta operação")
	}

	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}
	cert := certs[0]

	pk := strAttr(mdfe, "pk")
	environment := 2
	if envPrefixFromPK(pk) == EnvProd {
		environment = 1
	}

	return &eventContext{
		mdfe:        mdfe,
		pk:          pk,
		environment: environment,
		cnpj:        services.StripPKPrefix(orgPK),
		isPJ:        len(orgPK) > 5 && orgPK[:5] == "CNPJ_",
		emitUF:      emitUFFromAccessKey(accessKey),
		sefazEnv:    sefazEnvFor(environment),
		certS3Key:   strAttr(cert, "s3_key"),
		certPass:    strAttr(cert, "password"),
	}, nil
}

// dispatchEvent persists the event record, optionally updates the document
// status, and publishes the SEFAZ event to the worker.
func (s *MdfeService) dispatchEvent(
	ctx context.Context, ec *eventContext, accessKey, eventType string, seq int,
	body map[string]any, newDocStatus string, userID, userName string,
) (map[string]types.AttributeValue, error) {
	event, err := s.eventRepo.CreateEvent(ctx, accessKey, eventType, seq, StatusPending, nil, nil, nil, userID, userName)
	if err != nil {
		return nil, err
	}

	if newDocStatus != "" {
		if _, err := s.mdfeRepo.Update(ctx, ec.pk, accessKey, map[string]any{
			"status":     newDocStatus,
			"updated_at": repositories.NowStr(),
		}); err != nil {
			return nil, err
		}
	}

	eventSK := strAttr(event, "sk")
	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK:            ec.pk,
		AccessKey:        accessKey,
		TableName:        tableMdfes,
		S3Prefix:         s3PrefixMdfe,
		ExpectedFileName: fmt.Sprintf("%s_%s_%03d", accessKey, eventType, seq),
		CNPJ:             ec.cnpj,
		UF:               ec.emitUF,
		SefazEnvironment: ec.sefazEnv,
		CertS3Key:        ec.certS3Key,
		CertPassword:     ec.certPass,
		DocType:          s3PrefixMdfe,
		SefazService:     sefazServiceEvento,
		Body:             body,
		EventsTableName:  aws.String(eventsTableKey + "_events"),
		EventType:        aws.String(eventType),
		SequenceNumber:   &seq,
		EventSK:          aws.String(eventSK),
	}); err != nil {
		return nil, err
	}

	if newDocStatus != "" {
		ec.mdfe["status"] = &types.AttributeValueMemberS{Value: newDocStatus}
	}
	return ec.mdfe, nil
}

// Cancel marks the MDF-e cancel_pending and dispatches a cancellation event (110111).
func (s *MdfeService) Cancel(ctx context.Context, orgPK, accessKey, justification string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if len(justification) < 15 {
		return nil, problem.BadRequest("justificativa de cancelamento deve ter ao menos 15 caracteres")
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt := strAttr(ec.mdfe, "sefaz_protocol")
	if nProt == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado no MDF-e")
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoCancelamento, seq, map[string]any{
		"evCancMDFe": map[string]any{
			"descEvento": "Cancelamento",
			"nProt":      nProt,
			"xJust":      justification,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoCancelamento, seq, body, StatusCancelPending, userID, userName)
}

// Close dispatches an encerramento event (110112). cMun/UF identify where the
// trip ended; when omitted, UF defaults to the MDF-e UFFim.
func (s *MdfeService) Close(ctx context.Context, orgPK, accessKey, cMun, uf string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt := strAttr(ec.mdfe, "sefaz_protocol")
	if nProt == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado no MDF-e")
	}
	if uf == "" {
		uf = strAttr(ec.mdfe, "uf_fim")
	}
	cUF, ok := services.UFCode[uf]
	if !ok {
		return nil, problem.BadRequest("UF de encerramento inválida: " + uf)
	}
	if cMun == "" {
		return nil, problem.BadRequest("informe o município de encerramento (cMun)")
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoEncerramento, seq, map[string]any{
		"evEncMDFe": map[string]any{
			"descEvento": "Encerramento",
			"nProt":      nProt,
			"dtEnc":      time.Now().Format("2006-01-02"), // placeholder; overridden below
			"cUF":        cUF,
			"cMun":       cMun,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoEncerramento, seq, body, StatusClosePending, userID, userName)
}

// IncludeCondutor dispatches an inclusão de condutor event (110114).
func (s *MdfeService) IncludeCondutor(ctx context.Context, orgPK, accessKey, name, cpf string, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if name == "" || onlyDigits(cpf) == "" {
		return nil, problem.BadRequest("informe nome e CPF do condutor")
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoInclusaoCond, seq, map[string]any{
		"evIncCondutorMDFe": map[string]any{
			"descEvento": "Inclusao Condutor",
			"condutor":   map[string]any{"xNome": name, "CPF": onlyDigits(cpf)},
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoInclusaoCond, seq, body, "", userID, userName)
}

// IncludeDFe dispatches an inclusão de DF-e event (110115), adding documents to
// an already-authorized MDF-e.
func (s *MdfeService) IncludeDFe(ctx context.Context, orgPK, accessKey, cMunCarrega, xMunCarrega string, docs []IncludeDFeDoc, seq int, userID, userName string) (map[string]types.AttributeValue, error) {
	if len(docs) == 0 {
		return nil, problem.BadRequest("informe ao menos um documento para inclusão")
	}
	if cMunCarrega == "" {
		return nil, problem.BadRequest("informe o município de carregamento da inclusão")
	}
	ec, err := s.resolveEventContext(ctx, orgPK, accessKey)
	if err != nil {
		return nil, err
	}
	nProt := strAttr(ec.mdfe, "sefaz_protocol")
	if nProt == "" {
		return nil, problem.BadRequest("protocolo de autorização não encontrado no MDF-e")
	}
	infDoc := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		if len(d.ChNFe) != 44 {
			return nil, problem.BadRequest("chave de NF-e inválida: " + d.ChNFe)
		}
		infDoc = append(infDoc, map[string]any{
			"cMunDescarga": d.CMunDescarga,
			"xMunDescarga": d.XMunDescarga,
			"chNFe":        d.ChNFe,
		})
	}
	body := s.buildEventEnvelope(ec, accessKey, TpEventoInclusaoDFe, seq, map[string]any{
		"evIncDFeMDFe": map[string]any{
			"descEvento":  "Inclusão DF-e",
			"nProt":       nProt,
			"cMunCarrega": cMunCarrega,
			"xMunCarrega": xMunCarrega,
			"infDoc":      infDoc,
		},
	})
	return s.dispatchEvent(ctx, ec, accessKey, TpEventoInclusaoDFe, seq, body, "", userID, userName)
}

// IncludeDFeDoc is one document added by an inclusão de DF-e event.
type IncludeDFeDoc struct {
	CMunDescarga string `json:"unloading_ibge_code" validate:"required,ibge"`
	XMunDescarga string `json:"unloading_city" validate:"required,max=120"`
	ChNFe        string `json:"nfe_key" validate:"required,len=44,numeric"`
}

// buildEventEnvelope assembles the envEventoMDFe wrapper around a detEvento body.
func (s *MdfeService) buildEventEnvelope(ec *eventContext, accessKey, tpEvento string, seq int, detEvento map[string]any) map[string]any {
	detEvento["@versaoEvento"] = mdfeVersao

	infEvento := map[string]any{
		"@Id":        fmt.Sprintf("ID%s%s%02d", tpEvento, accessKey, seq),
		"cOrgao":     accessKey[0:2],
		"tpAmb":      fmt.Sprintf("%d", ec.environment),
		"chMDFe":     accessKey,
		"dhEvento":   dhEvento(),
		"tpEvento":   tpEvento,
		"nSeqEvento": fmt.Sprintf("%d", seq),
		"detEvento":  detEvento,
	}
	if ec.isPJ {
		infEvento["CNPJ"] = ec.cnpj
	} else {
		infEvento["CPF"] = ec.cnpj
	}

	return map[string]any{
		"eventoMDFe": map[string]any{
			"@versao":   mdfeVersao,
			"@xmlns":    mdfeXMLNS,
			"infEvento": infEvento,
		},
	}
}
