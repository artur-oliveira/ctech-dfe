package ctes

import (
	"context"
	"strconv"
	"strings"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/shopspring/decimal"
)

// Códigos de aviso do derive (a UI ramifica pelo código, nunca pelo texto).
const (
	WarnMixedSender       = "mixed_sender"
	WarnMixedRecipient    = "mixed_recipient"
	WarnTakerAmbiguous    = "taker_ambiguous"
	WarnWeightMissing     = "weight_missing"
	WarnNoteNotAuthorized = "note_not_authorized"
	WarnNoteNotFound      = "note_not_found"
	WarnNoteWrongModel    = "note_wrong_model"
	// WarnNoteXMLMissing: a nota está no cadastro mas o XML autorizado não está disponível; fica de fora.
	WarnNoteXMLMissing = "note_xml_unavailable"
)

// Papéis do tomador e motivos da sugestão.
const (
	RoleRemetente    = "remetente"
	RoleDestinatario = "destinatario"
	RoleExpedidor    = "expedidor"
	RoleRecebedor    = "recebedor"
	RoleOutro        = "outro"

	reasonCIF = "mod_frete_cif"
	reasonFOB = "mod_frete_fob"

	// modFrete da NF-e: 0 CIF (por conta do remetente), 1 FOB (por conta do destinatário).
	modFreteCIF = "0"
	modFreteFOB = "1"

	sourceIssued   = "issued"
	sourceReceived = "received"

	// unidadeKG é a unidade comercial (uCom) que conta como peso no fallback.
	unidadeKG = "KG"
)

// NoteParty é emitente ou destinatário de uma nota.
type NoteParty struct {
	Doc      string `json:"doc"`
	Name     string `json:"name"`
	UF       string `json:"uf"`
	CityIBGE string `json:"city_ibge"`
	City     string `json:"city"`
}

// Party é remetente ou destinatário derivado, com a pessoa do cadastro quando existe.
type Party struct {
	NoteParty
	PersonID *string `json:"person_id"`
}

// DeriveNote é uma nota lida.
type DeriveNote struct {
	AccessKey string    `json:"access_key"`
	Number    int       `json:"number"`
	Serie     int       `json:"serie"`
	IssuedAt  string    `json:"issued_at"`
	Source    string    `json:"source"`
	Value     string    `json:"value"`
	WeightKg  *string   `json:"weight_kg"`
	Volumes   int64     `json:"volumes"`
	ModFrete  string    `json:"mod_frete"`
	Emitter   NoteParty `json:"emitter"`
	Recipient NoteParty `json:"recipient"`
}

// TakerSuggestion é o tomador sugerido pelo modFrete das notas.
type TakerSuggestion struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

// Cargo é a carga somada das notas.
type Cargo struct {
	TotalValue         string  `json:"total_value"`
	TotalWeightKg      *string `json:"total_weight_kg"`
	Volumes            int64   `json:"volumes"`
	PredominantProduct string  `json:"predominant_product"`
}

// Place é um município.
type Place struct {
	UF       string `json:"uf"`
	CityIBGE string `json:"city_ibge"`
	City     string `json:"city"`
}

// Warning é um aviso do derive, em pt-BR.
type Warning struct {
	Code      string  `json:"code"`
	Message   string  `json:"message"`
	AccessKey *string `json:"access_key"`
}

// DeriveResult é a resposta de POST /ctes/derive.
type DeriveResult struct {
	Notes           []DeriveNote     `json:"notes"`
	Sender          *Party           `json:"sender"`
	Recipient       *Party           `json:"recipient"`
	TakerSuggestion *TakerSuggestion `json:"taker_suggestion"`
	Cargo           Cargo            `json:"cargo"`
	Origin          *Place           `json:"origin"`
	Destination     *Place           `json:"destination"`
	Warnings        []Warning        `json:"warnings"`
}

// derivation é o DeriveResult mais as notas lidas, que a emissão usa para montar remetente e destinatário.
type derivation struct {
	DeriveResult
	parsed        []*nfexml.NFe
	weight        *decimal.Decimal
	notFound      []string
	notAuthorized []string
}

func (s *CteService) derive(ctx context.Context, orgPK string, env int, keys []string) (*derivation, error) {
	notes, err := s.loadNotes(ctx, services.EnvToPrefix(env)+"#"+orgPK, keys)
	if err != nil {
		return nil, err
	}
	d := deriveNotes(notes)
	if len(d.Notes) == 0 {
		return nil, problem.UnprocessableEntity("nenhuma das notas informadas foi encontrada no cadastro").WithCode(WarnNoteNotFound)
	}
	for _, p := range []*Party{d.Sender, d.Recipient} {
		if p == nil {
			continue
		}
		sk, item, err := s.person(ctx, orgPK, p.Doc)
		if err != nil {
			return nil, err
		}
		if item != nil {
			p.PersonID = &sk
		}
	}
	return d, nil
}

// deriveNotes aplica as regras de derivação da spec sobre as notas já lidas. Função pura.
func deriveNotes(notes []loadedNote) *derivation {
	d := &derivation{DeriveResult: DeriveResult{Notes: []DeriveNote{}, Warnings: []Warning{}}}
	warn := func(code, msg, key string) {
		w := Warning{Code: code, Message: msg}
		if key != "" {
			k := key
			w.AccessKey = &k
		}
		d.Warnings = append(d.Warnings, w)
	}

	totalValue, totalWeight := decimal.Zero, decimal.Zero
	weightKnown := true
	predValue := decimal.Zero
	for _, n := range notes {
		switch {
		case n.item == nil:
			d.notFound = append(d.notFound, n.key)
			warn(WarnNoteNotFound, "Nota fora do cadastro da organização", n.key)
			continue
		case n.nfe == nil:
			d.notFound = append(d.notFound, n.key)
			warn(WarnNoteXMLMissing, "XML autorizado da nota indisponível; importe o XML para usá-la", n.key)
			continue
		case n.nfe.Model != modelNFe:
			d.notFound = append(d.notFound, n.key)
			warn(WarnNoteWrongModel, "Documento não é NF-e modelo 55", n.key)
			continue
		}
		if strAttr(n.item, "status") != statusAuthorized {
			d.notAuthorized = append(d.notAuthorized, n.key)
			warn(WarnNoteNotAuthorized, "Nota não autorizada pela SEFAZ", n.key)
		}
		nfe := n.nfe
		d.parsed = append(d.parsed, nfe)

		value := nfe.VNF
		if value.IsZero() {
			for _, it := range nfe.Items {
				value = value.Add(it.VProd)
			}
		}
		var volumes int64
		for _, v := range nfe.Volumes {
			volumes += v.QVol
		}
		note := DeriveNote{
			AccessKey: n.key,
			Number:    atoi(nfe.Number),
			Serie:     atoi(nfe.Serie),
			IssuedAt:  nfe.IssuedAt,
			Source:    sourceIssued,
			Value:     value.StringFixed(2),
			Volumes:   volumes,
			ModFrete:  nfe.ModFrete,
			Emitter:   noteParty(nfe.Emit),
			Recipient: noteParty(nfe.Dest),
		}
		if intAttr(n.item, "incoming", 0) == 1 {
			note.Source = sourceReceived
		}
		if w, ok := noteWeight(nfe); ok {
			s := w.StringFixed(3)
			note.WeightKg = &s
			totalWeight = totalWeight.Add(w)
		} else {
			weightKnown = false
			warn(WarnWeightMissing, "Nota sem peso bruto nem itens em KG; informe o peso da carga", n.key)
		}
		d.Notes = append(d.Notes, note)
		d.Cargo.Volumes += volumes
		totalValue = totalValue.Add(value)
		for _, it := range nfe.Items {
			if it.VProd.GreaterThan(predValue) {
				predValue = it.VProd
				d.Cargo.PredominantProduct = it.XProd
			}
		}
	}
	if len(d.Notes) == 0 {
		return d
	}

	d.Cargo.TotalValue = totalValue.StringFixed(2)
	if weightKnown {
		s := totalWeight.StringFixed(3)
		d.Cargo.TotalWeightKg = &s
		d.weight = &totalWeight
	}

	first := d.parsed[0]
	if sameDoc(d.parsed, func(n *nfexml.NFe) string { return n.Emit.Doc() }) {
		d.Sender = &Party{NoteParty: noteParty(first.Emit)}
		d.Origin = &Place{UF: first.Emit.Addr.UF, CityIBGE: first.Emit.Addr.CityIBGE, City: first.Emit.Addr.City}
	} else {
		warn(WarnMixedSender, "Notas com remetentes diferentes: emita um CT-e por remetente", "")
	}
	if sameDoc(d.parsed, func(n *nfexml.NFe) string { return n.Dest.Doc() }) {
		d.Recipient = &Party{NoteParty: noteParty(first.Dest)}
		d.Destination = &Place{UF: first.Dest.Addr.UF, CityIBGE: first.Dest.Addr.CityIBGE, City: first.Dest.Addr.City}
	} else {
		warn(WarnMixedRecipient, "Notas com destinatários diferentes: emita um CT-e por destinatário", "")
	}

	switch {
	case sameDoc(d.parsed, func(n *nfexml.NFe) string { return n.ModFrete }) && first.ModFrete == modFreteCIF:
		d.TakerSuggestion = &TakerSuggestion{Role: RoleRemetente, Reason: reasonCIF}
	case sameDoc(d.parsed, func(n *nfexml.NFe) string { return n.ModFrete }) && first.ModFrete == modFreteFOB:
		d.TakerSuggestion = &TakerSuggestion{Role: RoleDestinatario, Reason: reasonFOB}
	default:
		warn(WarnTakerAmbiguous, "Modalidade do frete das notas não indica quem paga; escolha o tomador", "")
	}
	return d
}

// noteWeight: soma de transp/vol/pesoB; sem peso, soma de qCom dos itens em KG.
func noteWeight(n *nfexml.NFe) (decimal.Decimal, bool) {
	w := decimal.Zero
	for _, v := range n.Volumes {
		w = w.Add(v.PesoB)
	}
	if w.IsPositive() {
		return w, true
	}
	for _, it := range n.Items {
		if strings.EqualFold(strings.TrimSpace(it.UCom), unidadeKG) {
			w = w.Add(nfexml.Dec(it.QCom))
		}
	}
	return w, w.IsPositive()
}

func sameDoc(notes []*nfexml.NFe, f func(*nfexml.NFe) string) bool {
	for _, n := range notes[1:] {
		if f(n) != f(notes[0]) {
			return false
		}
	}
	return true
}

func noteParty(p nfexml.Party) NoteParty {
	return NoteParty{Doc: p.Doc(), Name: p.Name, UF: p.Addr.UF, CityIBGE: p.Addr.CityIBGE, City: p.Addr.City}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// personSK aceita o SK do cadastro ou o documento puro.
func personSK(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "CNPJ_") || strings.HasPrefix(id, "CPF_") || strings.HasPrefix(id, repositories.SKPrefixForeign) {
		return id
	}
	sk, err := services.BuildPersonSK(id)
	if err != nil {
		return ""
	}
	return sk
}
