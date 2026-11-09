package ctes

// original.go: leitura do CT-e original (autorizado, emitido pela organização) que um CT-e complementar (tpCTe 1)
// ou substituto (tpCTe 3) referencia. Os participantes e o percurso vêm do XML autorizado guardado no S3, nunca do
// cadastro de pessoas. Regras do MOC CT-e (RegrasDeValidacao): complemento G017 e G160 a G177; substituição G076 a G101.

import (
	"context"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// Códigos dos 422 do complemento e do substituto (problem.code). A UI ramifica por eles.
const (
	CodeOriginalNotFound        = "original_not_found"
	CodeOriginalNotIssued       = "original_not_issued_by_organization"
	CodeOriginalNotAuthorized   = "original_not_authorized"
	CodeOriginalXMLUnavailable  = "original_xml_unavailable"
	CodeOriginalXMLInvalid      = "original_xml_invalid"
	CodeOriginalTypeInvalid     = "original_type_invalid"
	CodeOriginalUnsupported     = "original_unsupported"
	CodeOriginalTooOld          = "original_too_old"
	CodeOriginalsMismatch       = "originals_mismatch"
	CodeEmitterIEChanged        = "emitter_ie_changed"
	CodeContingencyNotAllowed   = "contingency_not_allowed"
	CodeTakerNotRequested       = "taker_change_not_requested"
	CodeTakerRequired           = "taker_required"
	CodeTakerRoleMissing        = "taker_role_missing"
	CodeTakerUnchanged          = "taker_unchanged"
	CodeTakerBaseNotInOriginal  = "taker_base_not_in_original"
	CodeTakerUFMismatch         = "taker_uf_mismatch"
	CodeSubstitutionWindowEnded = "substitution_window_expired"
)

const (
	// complementMaxAgeMonths: a chave do original não pode ser anterior a 12 meses (G161a, 933; 24 em ferroviário e
	// multimodal, fora do escopo rodoviário).
	complementMaxAgeMonths = 12
	// substitutionWindow é o prazo de 60 dias da autorização do original (G096, 563). O registro não guarda dhAut;
	// dhEmi é o limite inferior e a autorização pode atrasar até 168 h (G103): só recusa localmente além de
	// 60 dias mais 168 h de dhEmi, o resto é da SEFAZ.
	substitutionWindow     = 60 * 24 * time.Hour
	authorizationSlack     = 168 * time.Hour
	accessKeyEmitterStart  = 6 // CNPJ do emitente: dígitos 7 a 20 da chave
	accessKeyEmitterEnd    = 20
	accessKeyYearMonthFrom = 2 // AAMM: dígitos 3 a 6
	accessKeyYearMonthTo   = 6
	modalRodoviarioCode    = modalRodoviario
	tomaDocBaseLen         = 8 // CNPJ-base
	ufExterior             = "EX"
)

// original é um CT-e referenciado, com o registro e o XML autorizado lidos.
type original struct {
	key  string
	item map[string]types.AttributeValue
	cte  *nfexml.CTe
}

// loadOriginal lê o original no cadastro da organização (mesmo ambiente) e o XML no S3, e aplica as regras do MOC
// que se provam sem a SEFAZ: emitido pela organização (G160), autorizado (G165/G081), tipo normal ou substituto
// (G164/G085), modal rodoviário (escopo) e IE do emitente igual (G173/G091).
func (s *CteService) loadOriginal(ctx context.Context, pk, emitDoc, emitIE, key string) (*original, error) {
	if len(key) != accessKeyLen || key[accessKeyEmitterStart:accessKeyEmitterEnd] != emitDoc {
		return nil, unprocessable(CodeOriginalNotIssued, "o CT-e "+key+" não foi emitido por esta organização")
	}
	item, err := s.cteRepo.Get(ctx, pk, key)
	if err != nil {
		return nil, err
	}
	if item == nil || intAttr(item, "incoming", 0) == 1 {
		return nil, unprocessable(CodeOriginalNotFound, "CT-e original "+key+" não encontrado entre os emitidos pela organização neste ambiente")
	}
	if strAttr(item, "status") != statusAuthorized {
		return nil, unprocessable(CodeOriginalNotAuthorized, "o CT-e original "+key+" precisa estar autorizado (não pode estar pendente, rejeitado ou cancelado)")
	}
	s3Key := strAttr(item, "xml_s3_key")
	if s3Key == "" {
		return nil, unprocessable(CodeOriginalXMLUnavailable, "o XML autorizado do CT-e original "+key+" não está disponível")
	}
	data, err := services.DownloadS3(ctx, s.clients, s.bucketDocs, s3Key)
	if err != nil {
		return nil, err
	}
	cte, err := nfexml.ParseCTe(data)
	if err != nil {
		return nil, unprocessable(CodeOriginalXMLInvalid, "o XML do CT-e original "+key+" é inválido")
	}
	return checkOriginal(&original{key: key, item: item, cte: cte}, emitIE)
}

// checkOriginal é a parte pura de loadOriginal, testável sem S3.
func checkOriginal(o *original, emitIE string) (*original, error) {
	c := o.cte
	if c.TpCTe != tpCTeNormal && c.TpCTe != tpCTeSubstitute {
		return nil, unprocessable(CodeOriginalTypeInvalid, "o CT-e original "+o.key+" precisa ser do tipo normal ou substituto")
	}
	if c.Modal != modalRodoviarioCode {
		return nil, unprocessable(CodeOriginalUnsupported, "só o CT-e rodoviário pode ser complementado ou substituído por esta API")
	}
	if onlyDigits(c.EmitIE) != onlyDigits(emitIE) {
		return nil, unprocessable(CodeEmitterIEChanged, "a IE do emitente mudou desde o CT-e original "+o.key+"; o MOC exige a mesma IE (G173, G091)")
	}
	return o, nil
}

// actorOf converte um participante lido do XML no ator do builder.
func actorOf(p *nfexml.CTeActor) *actor {
	if p == nil {
		return nil
	}
	return &actor{CNPJ: p.CNPJ, CPF: p.CPF, IE: p.IE, Name: p.Name, Phone: p.Addr.Phone, Email: p.Email, Addr: p.Addr}
}

func placeOf(uf, cMun, xMun string) Place { return Place{UF: uf, CityIBGE: cMun, City: xMun} }

// takerOf devolve o papel e o ator do tomador do original (toma3 por código, toma4 como "outro").
func takerOf(c *nfexml.CTe) (role string, a *actor) {
	if c.Toma4 != nil {
		return RoleOutro, actorOf(c.Toma4)
	}
	switch c.Toma3 {
	case toma3Remetente:
		return RoleRemetente, actorOf(c.Rem)
	case toma3Expedidor:
		return RoleExpedidor, actorOf(c.Exped)
	case toma3Recebedor:
		return RoleRecebedor, actorOf(c.Receb)
	case toma3Destinatario:
		return RoleDestinatario, actorOf(c.Dest)
	}
	return "", nil
}

// takerActors preenche o ator do tomador no papel dado em p (toma3 usa rem/exped/receb/dest; toma4 usa p.toma4).
func (p *buildParams) setActors(c *nfexml.CTe) {
	p.rem, p.dest = *actorOf(c.Rem), *actorOf(c.Dest)
	p.exped, p.receb = actorOf(c.Exped), actorOf(c.Receb)
	p.origin = placeOf(c.UFIni, c.CMunIni, c.XMunIni)
	p.destination = placeOf(c.UFFim, c.CMunFim, c.XMunFim)
	p.serviceType = c.TpServ
}

// actorDoc compara participantes por documento; nil e nil são iguais.
func actorDoc(p *nfexml.CTeActor) string {
	if p == nil {
		return ""
	}
	return p.Doc()
}

// sameShape diz o que difere entre dois originais que seriam complementados juntos: um CT-e complementar tem um
// só conjunto de participantes e um só percurso (G168 a G175).
func sameShape(a, b *nfexml.CTe) string {
	switch {
	case actorDoc(a.Rem) != actorDoc(b.Rem):
		return "remetente"
	case actorDoc(a.Dest) != actorDoc(b.Dest):
		return "destinatário"
	case actorDoc(a.Exped) != actorDoc(b.Exped):
		return "expedidor"
	case actorDoc(a.Receb) != actorDoc(b.Receb):
		return "recebedor"
	case a.Toma3 != b.Toma3 || actorDoc(a.Toma4) != actorDoc(b.Toma4):
		return "tomador"
	case a.UFIni != b.UFIni || a.UFFim != b.UFFim:
		return "UF de início ou fim da prestação"
	case a.TpServ != b.TpServ:
		return "tipo de serviço"
	}
	return ""
}

// checkKeyAge recusa original com chave de mais de 12 meses (G161a).
func checkKeyAge(key string, now time.Time) error {
	y, m := atoi(key[accessKeyYearMonthFrom:accessKeyYearMonthFrom+2]), atoi(key[accessKeyYearMonthFrom+2:accessKeyYearMonthTo])
	keyMonths := (2000+y)*12 + m
	nowMonths := now.Year()*12 + int(now.Month())
	if nowMonths-keyMonths > complementMaxAgeMonths {
		return unprocessable(CodeOriginalTooOld, "o CT-e "+key+" tem mais de 12 meses; o MOC não aceita complemento (G161a)")
	}
	return nil
}

// checkEmissionForType: complemento e substituto não vão pela SVC (G012, 517) e o substituto exige emissão normal
// (G076, 503). O complemento aceita a contingência off-line.
func checkEmissionForType(tpCTe string, em emission) error {
	switch {
	case tpCTe == "" || tpCTe == tpCTeNormal:
		return nil
	case em.svc != "" || (tpCTe == tpCTeSubstitute && em.tpEmis != services.TpEmisNormal):
		return unprocessable(CodeContingencyNotAllowed, "complemento e substituição só podem ser emitidos fora da SVC"+
			" (o substituto, só em emissão normal); volte a emissão ao normal na Configuração Fiscal")
	}
	return nil
}

// CteChargeBody é o que o usuário informa do valor e dos tributos do complemento e do substituto. Participantes,
// percurso e tomador vêm do XML do original.
type CteChargeBody struct {
	Freight       CteFreight         `json:"freight" validate:"required"`
	ICMS          *CteICMS           `json:"icms"`
	IBSCBS        *CteIBSCBS         `json:"ibs_cbs"`
	AuthorizedXML []CteAuthorizedXML `json:"authorized_xml" validate:"omitempty,max=10,dive"`
	// CFOP da prestação: regra fiscal, nunca presumido.
	CFOP           *string `json:"cfop" validate:"omitempty,cfop_cte"`
	AdditionalInfo *string `json:"additional_info" validate:"omitempty,max=2000"`
}

// charge é o CteChargeBody já resolvido.
type charge struct {
	vTPrest decimal.Decimal
	comps   []map[string]any
	icms    map[string]any
	ibscbs  *ibscbsResult
	autXML  []map[string]any
	cst     string
}

func resolveCharge(b CteChargeBody, crt int, actorDocs []string) (*charge, error) {
	if b.CFOP == nil || *b.CFOP == "" {
		return nil, unprocessable(CodeCFOPRequired, "informe o CFOP da prestação do serviço de transporte")
	}
	vTPrest, comps, err := resolveComponents(b.Freight)
	if err != nil {
		return nil, err
	}
	icms, err := resolveICMS(b.ICMS, crt, vTPrest)
	if err != nil {
		return nil, err
	}
	ibscbs, err := resolveIBSCBS(b.IBSCBS, crt)
	if err != nil {
		return nil, err
	}
	autXML, err := resolveAuthorizedXML(b.AuthorizedXML, actorDocs)
	if err != nil {
		return nil, err
	}
	cst := cstSN
	if b.ICMS != nil {
		cst = b.ICMS.CST
	}
	return &charge{vTPrest: vTPrest, comps: comps, icms: icms, ibscbs: ibscbs, autXML: autXML, cst: cst}, nil
}

func (c *charge) apply(p *buildParams, b CteChargeBody) {
	p.cfop = *b.CFOP
	p.vTPrest, p.comps, p.icms, p.ibscbs, p.autXML = c.vTPrest, c.comps, c.icms, c.ibscbs, c.autXML
	p.addInfo = valueOr(b.AdditionalInfo)
}

// docsOf lista os documentos dos participantes (para a validação do autXML).
func docsOf(emit actor, c *nfexml.CTe, toma *actor) []string {
	docs := []string{emit.doc()}
	for _, a := range []*nfexml.CTeActor{c.Rem, c.Dest, c.Exped, c.Receb} {
		if a != nil {
			docs = append(docs, a.Doc())
		}
	}
	if toma != nil {
		docs = append(docs, toma.doc())
	}
	return docs
}

func docBase(doc string) string {
	if len(doc) == cnpjLen {
		return doc[:tomaDocBaseLen]
	}
	return doc
}
