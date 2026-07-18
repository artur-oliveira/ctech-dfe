package mdfes

import (
	"context"
	"fmt"

	"gopkg.aoctech.app/dfe/api/internal/problem"

	"github.com/shopspring/decimal"
)

// CargoPreview is returned by PreviewCargo: the cargo data the frontend shows
// (and lets the user correct) before emitting. All money/weight values are
// decimal strings so the frontend renders them without float drift.
type CargoPreview struct {
	Documents   []CargoPreviewDoc `json:"documents"`
	Loadings    []MdfeMun         `json:"loadings"`
	Unloadings  []MdfeMun         `json:"unloadings"`
	UFIni       string            `json:"uf_start"`
	UFFim       string            `json:"uf_end"`
	TotalWeight string            `json:"total_weight"`
	TotalValue  string            `json:"total_value"`
	Predominant MdfeProdPred      `json:"predominant"`
}

// CargoPreviewDoc is the per-document cargo extracted from the referenced XML.
type CargoPreviewDoc struct {
	Type        string       `json:"type"`
	AccessKey   string       `json:"access_key"`
	EmitName    string       `json:"emit_name"`
	DestName    string       `json:"dest_name"`
	Loading     MdfeMun      `json:"loading"`
	Unloading   MdfeMun      `json:"unloading"`
	UFIni       string       `json:"uf_start"`
	UFFim       string       `json:"uf_end"`
	Weight      string       `json:"weight"`
	HasWeight   bool         `json:"has_weight"`
	Value       string       `json:"value"`
	Predominant MdfeProdPred `json:"predominant"`
}

// PreviewCargo downloads + parses each referenced document's XML and returns the
// aggregated cargo data without persisting anything. HasWeight=false signals the
// frontend to collect the gross weight from the user (XML carried no volume).
func (s *MdfeService) PreviewCargo(ctx context.Context, orgPK string, docs []MdfeDocRef) (*CargoPreview, error) {
	if len(docs) == 0 {
		return nil, problem.BadRequest("informe ao menos um documento")
	}
	if _, err := validateSingleDocType(docs); err != nil {
		return nil, err
	}

	env, err := s.GetEnvironment(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk := fmt.Sprintf("%s#%s", envToPrefix(env), orgPK)

	out := &CargoPreview{}
	totalWeight := decimal.Zero
	totalValue := decimal.Zero
	predValue := decimal.Zero
	loadSeen := map[string]bool{}
	unloadSeen := map[string]bool{}

	for _, ref := range docs {
		repo := s.docRepo(ref.Type)
		item, err := repo.Get(ctx, pk, ref.AccessKey)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return nil, problem.NotFound("documento não encontrado: " + ref.AccessKey)
		}
		s3Key := strAttr(item, "xml_s3_key")
		if s3Key == "" {
			return nil, problem.BadRequest("XML do documento " + ref.AccessKey + " não disponível para manifestação")
		}
		xmlData, err := downloadS3(ctx, s.clients, s.bucketDocs, s3Key)
		if err != nil {
			return nil, err
		}
		cargo, err := extractCargo(ref.AccessKey, ref.Type, xmlData)
		if err != nil {
			return nil, errInvalidDocXML
		}

		hasWeight := cargo.weightKG.IsPositive()
		doc := CargoPreviewDoc{
			Type:        ref.Type,
			AccessKey:   ref.AccessKey,
			EmitName:    cargo.emit.name,
			DestName:    cargo.dest.name,
			Loading:     MdfeMun{IBGECode: cargo.emit.cMun, City: cargo.emit.xMun},
			Unloading:   MdfeMun{IBGECode: cargo.dest.cMun, City: cargo.dest.xMun},
			UFIni:       cargo.emit.uf,
			UFFim:       cargo.dest.uf,
			Weight:      cargo.weightKG.StringFixed(4),
			HasWeight:   hasWeight,
			Value:       cargo.totalValue.StringFixed(2),
			Predominant: MdfeProdPred{TpCarga: defaultTpCarga, XProd: cargo.predProd, NCM: cargo.predNCM},
		}
		out.Documents = append(out.Documents, doc)

		if cargo.emit.cMun != "" && !loadSeen[cargo.emit.cMun] {
			loadSeen[cargo.emit.cMun] = true
			out.Loadings = append(out.Loadings, MdfeMun{IBGECode: cargo.emit.cMun, City: cargo.emit.xMun})
		}
		if cargo.dest.cMun != "" && !unloadSeen[cargo.dest.cMun] {
			unloadSeen[cargo.dest.cMun] = true
			out.Unloadings = append(out.Unloadings, MdfeMun{IBGECode: cargo.dest.cMun, City: cargo.dest.xMun})
		}

		totalWeight = totalWeight.Add(cargo.weightKG)
		totalValue = totalValue.Add(cargo.totalValue)
		if cargo.totalValue.GreaterThan(predValue) {
			predValue = cargo.totalValue
			out.Predominant = MdfeProdPred{TpCarga: defaultTpCarga, XProd: cargo.predProd, NCM: cargo.predNCM}
		}
	}

	if out.Predominant.XProd == "" {
		out.Predominant.XProd = "CARGA GERAL"
	}
	if out.Predominant.TpCarga == "" {
		out.Predominant.TpCarga = defaultTpCarga
	}
	out.TotalWeight = totalWeight.StringFixed(4)
	out.TotalValue = totalValue.StringFixed(2)
	if len(out.Documents) > 0 {
		out.UFIni = out.Documents[0].UFIni
		out.UFFim = out.Documents[len(out.Documents)-1].UFFim
	}
	return out, nil
}
