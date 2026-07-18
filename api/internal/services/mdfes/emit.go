package mdfes

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// --- Request types ---

// MdfeEmitBody is the JSON body for POST /mdfes. All field names are in English
// and consistent with the cargo-preview and detail contracts.
type MdfeEmitBody struct {
	Modal     string       `json:"modal" validate:"required,oneof=rodoviario aereo aquaviario ferroviario"` // rodoviario|aereo|aquaviario|ferroviario
	Documents []MdfeDocRef `json:"documents" validate:"required,min=1,dive"`                                // NF-e or CT-e refs (single type, not mixed)

	UFStart string   `json:"uf_start" validate:"omitempty,uf"`   // optional override; derived from docs when empty
	UFEnd   string   `json:"uf_end" validate:"omitempty,uf"`     // optional override; derived from docs when empty
	Route   []string `json:"route" validate:"omitempty,dive,uf"` // intermediate UFs between UFStart/UFEnd (exclusive)

	// Optional ordering overrides for loading/unloading municipalities. When
	// empty, the order is derived from the referenced documents (first-seen).
	Loadings   []MdfeMun `json:"loadings" validate:"omitempty,dive"`
	Unloadings []MdfeMun `json:"unloadings" validate:"omitempty,dive"`

	Vehicle  MdfeVehicle   `json:"vehicle"`
	Trailers []MdfeTrailer `json:"trailers" validate:"omitempty,max=3,dive"`
	Drivers  []MdfeDriver  `json:"drivers" validate:"omitempty,dive"`

	Predominant *MdfeProdPred  `json:"predominant" validate:"omitempty"` // optional override; auto-derived otherwise
	BulkCargo   *MdfeBulkCargo `json:"bulk_cargo" validate:"omitempty"`  // required when exactly one document

	TripStart *string `json:"trip_start" validate:"omitempty"` // dhIniViagem (optional, RFC3339)

	RNTRC          *string `json:"rntrc" validate:"omitempty,rntrc"`
	CIOT           *string `json:"ciot" validate:"omitempty"`
	AdditionalInfo *string `json:"additional_info" validate:"omitempty,max=5000"`

	// Non-rodoviário modal payloads. Only the one matching Modal is consumed.
	Air   *MdfeAirModal   `json:"air" validate:"omitempty"`
	Water *MdfeWaterModal `json:"water" validate:"omitempty"`
	Rail  *MdfeRailModal  `json:"rail" validate:"omitempty"`
}

// MdfeDocRef references an NF-e or CT-e to be transported.
type MdfeDocRef struct {
	Type      string `json:"type" validate:"required,oneof=nfe cte"` // "nfe" | "cte"
	AccessKey string `json:"access_key" validate:"required,len=44,numeric"`
	Weight    string `json:"weight" validate:"omitempty,decimalv"` // optional gross-weight override (kg) when the XML carries none
}

// MdfeMun is an IBGE municipality (código + nome).
type MdfeMun struct {
	IBGECode string `json:"ibge_code" validate:"required,ibge"`
	City     string `json:"city" validate:"required,max=120"`
}

// MdfeVehicle is the traction vehicle. Either SK (registered vehicle) or the
// manual fields must be provided. Owner is supplied only when the vehicle does
// not belong to the MDF-e emitter (third-party prop).
type MdfeVehicle struct {
	SK      *string    `json:"sk" validate:"omitempty"`
	Placa   string     `json:"placa" validate:"omitempty,placa"`
	Tara    string     `json:"tara" validate:"omitempty,decimalv"`
	UF      string     `json:"uf" validate:"omitempty,uf"`
	RENAVAM *string    `json:"renavam" validate:"omitempty"`
	CapKG   *string    `json:"cap_kg" validate:"omitempty,decimalv"`
	TpRod   string     `json:"tp_rod" validate:"omitempty"` // tipo de rodado (01..06)
	TpCar   string     `json:"tp_car" validate:"omitempty"` // tipo de carroceria (00..05)
	Owner   *MdfeOwner `json:"owner" validate:"omitempty"`  // third-party owner (veicTracao/prop)
}

// MdfeTrailer is a reboque (trailer) — a registered vehicle with role=trailer.
type MdfeTrailer struct {
	SK string `json:"sk" validate:"required"`
}

// MdfeOwner is the third-party traction-vehicle owner (veicTracao/prop). Provide
// exactly one of CPF/CNPJ. Its presence drives ide/tpTransp (SEFAZ F18/F19/F25).
type MdfeOwner struct {
	CPF      string `json:"cpf" validate:"omitempty,cpf"`
	CNPJ     string `json:"cnpj" validate:"omitempty,cnpj"`
	Name     string `json:"name" validate:"omitempty,max=60"`
	IE       string `json:"ie" validate:"omitempty,max=20"`
	UF       string `json:"uf" validate:"omitempty,uf"`
	RNTRC    string `json:"rntrc" validate:"omitempty,rntrc"`
	TpProp   string `json:"tp_prop" validate:"omitempty,oneof=0 1 2"` // 0=TAC Agregado, 1=TAC Independente, 2=Outros
	TpTransp string `json:"tp_transp" validate:"omitempty"`           // optional CTC(3) override for a CNPJ owner
}

// MdfeDriver is a driver (condutor).
type MdfeDriver struct {
	Name string `json:"name" validate:"required,max=60"`
	CPF  string `json:"cpf" validate:"required,cpf"`
}

// MdfeProdPred overrides the auto-derived predominant product.
type MdfeProdPred struct {
	TpCarga string `json:"tp_carga" validate:"omitempty"`
	XProd   string `json:"x_prod" validate:"omitempty,max=120"`
	NCM     string `json:"ncm" validate:"omitempty,ncm"`
}

// MdfeBulkCargo carries the single-document (carga lotação) loading/unloading CEPs.
type MdfeBulkCargo struct {
	CEPLoading   string  `json:"cep_loading" validate:"omitempty,cep"`
	CEPUnloading string  `json:"cep_unloading" validate:"omitempty,cep"`
	LatLoading   *string `json:"lat_loading" validate:"omitempty"`
	LonLoading   *string `json:"lon_loading" validate:"omitempty"`
	LatUnloading *string `json:"lat_unloading" validate:"omitempty"`
	LonUnloading *string `json:"lon_unloading" validate:"omitempty"`
}

// resolvedOwner is the validated third-party owner ready for the prop builder.
type resolvedOwner struct {
	CPF      string
	CNPJ     string
	Name     string
	IE       string
	UF       string
	RNTRC    string
	TpProp   string
	TpTransp string
}

// resolvedCargo aggregates all parsed documents into the data needed to build
// the enviMDFe structure and the persisted summary.
type resolvedCargo struct {
	docs        []*docCargo
	carrega     []MdfeMun                  // ordered loading municipalities
	descarga    []descargaGroup            // unloading municipalities + their doc keys
	totalWeight decimal.Decimal            // kg
	totalValue  decimal.Decimal            // R$
	valueByNCM  map[string]decimal.Decimal // accumulated value per NCM
	prodPred    MdfeProdPred               // predominant product
	ufIni       string
	ufFim       string
}

// descargaGroup is one infMunDescarga node: a municipality and the access keys
// destined there, split by document type.
type descargaGroup struct {
	mun     MdfeMun
	nfeKeys []string
	cteKeys []string
}

// Emit resolves cargo from the referenced documents, builds the enviMDFe
// structure, atomically reserves a fiscal number, persists the record, and
// dispatches synchronous authorization (MDFeRecepcaoSinc) to the worker.
func (s *MdfeService) Emit(ctx context.Context, orgPK string, req MdfeEmitBody, userID, userName string) (map[string]types.AttributeValue, error) {
	modal := req.Modal
	if modal == "" {
		modal = ModalRodoviario
	}
	if _, ok := modalCodes[modal]; !ok {
		return nil, problem.BadRequest("modal inválido: " + modal)
	}
	if !enabledModals[modal] {
		return nil, problem.BadRequest("modal " + modal + " ainda não disponível para emissão")
	}
	if len(req.Documents) == 0 {
		return nil, problem.BadRequest("informe ao menos um documento (NF-e ou CT-e)")
	}
	docType, err := validateSingleDocType(req.Documents)
	if err != nil {
		return nil, err
	}
	if len(req.Documents) == 1 && req.BulkCargo == nil {
		return nil, problem.BadRequest("MDF-e com documento único (carga lotação) exige CEP de carregamento e descarregamento")
	}
	if req.Vehicle.SK == nil && (req.Vehicle.Placa == "" || req.Vehicle.Tara == "" || req.Vehicle.UF == "") {
		return nil, problem.BadRequest("informe um veículo cadastrado (sk) ou placa, tara e UF do veículo")
	}
	if len(req.Drivers) == 0 {
		return nil, problem.BadRequest("informe ao menos um condutor")
	}

	orgItem, err := s.orgRepo.GetOrganization(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if orgItem == nil {
		return nil, problem.NotFound("organização não encontrada")
	}

	certs, err := s.certRepo.List(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, problem.NoCertificate("certificado digital não encontrado")
	}
	cert := certs[0]

	configItem, err := s.configRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if configItem == nil {
		return nil, problem.BadRequest("configure o MDF-e em Configuração Fiscal antes de emitir")
	}

	environment := intAttr(configItem, "environment", 2)
	envPrefix := envToPrefix(environment)
	serie := intAttr(configItem, fmt.Sprintf("%s_current_serie", envPrefix), 1)
	currentNumber := intAttr(configItem, fmt.Sprintf("%s_current_number", envPrefix), 0)

	emitUF := extractEmitUF(orgItem)
	if emitUF == "" {
		return nil, problem.BadRequest("UF do emitente não configurada")
	}

	cargo, err := s.resolveCargo(ctx, orgPK, envPrefix, docType, req)
	if err != nil {
		return nil, err
	}

	resolvedVehicle, err := s.resolveVehicle(ctx, orgPK, req.Vehicle)
	if err != nil {
		return nil, err
	}

	trailers, err := s.resolveTrailers(ctx, orgPK, req.Trailers)
	if err != nil {
		return nil, err
	}

	owner, err := resolveOwner(req.Vehicle.Owner, orgPK)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cnpj := services.StripPKPrefix(orgPK)
	accessKey := services.GenerateAccessKey(emitUF, cnpj, services.ModelMDFe, serie, currentNumber, now)
	if accessKey == "" {
		return nil, problem.BadRequest("UF do emitente inválida: " + emitUF)
	}

	mdfeBody := BuildMDFe(buildParams{
		org:         orgItem,
		orgPK:       orgPK,
		accessKey:   accessKey,
		serie:       serie,
		number:      currentNumber,
		environment: environment,
		now:         now,
		modal:       modal,
		cargo:       cargo,
		vehicle:     resolvedVehicle,
		trailers:    trailers,
		owner:       owner,
		drivers:     req.Drivers,
		route:       req.Route,
		bulkCargo:   req.BulkCargo,
		tripStart:   req.TripStart,
		rntrc:       req.RNTRC,
		ciot:        req.CIOT,
		addInfo:     req.AdditionalInfo,
		air:         req.Air,
		water:       req.Water,
		rail:        req.Rail,
		tech:        s.tech,
	})

	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	record := s.buildRecord(pk, accessKey, orgPK, orgItem, currentNumber, serie, now, modal, docType, cargo, resolvedVehicle, owner, req, userID, userName)

	encoded, err := repositories.EncodeItem(record)
	if err != nil {
		return nil, problem.InternalServer("failed to encode MDF-e record")
	}

	if err := s.mdfeRepo.TransactReserveAndCreate(
		ctx, s.configRepo.TableName, orgPK, envPrefix, currentNumber, encoded,
	); err != nil {
		if strings.Contains(err.Error(), "TransactionCanceledException") {
			return nil, problem.Conflict("conflito ao reservar número do MDF-e. Tente novamente.")
		}
		return nil, err
	}

	if err := s.workerSvc.PublishWorkerEvent(ctx, services.WorkerMessage{
		DocPK:            pk,
		AccessKey:        accessKey,
		TableName:        tableMdfes,
		S3Prefix:         s3PrefixMdfe,
		ExpectedFileName: accessKey,
		CNPJ:             cnpj,
		UF:               emitUF,
		SefazEnvironment: sefazEnvFor(environment),
		CertS3Key:        strAttr(cert, "s3_key"),
		CertPassword:     strAttr(cert, "password"),
		DocType:          s3PrefixMdfe,
		SefazService:     sefazServiceAutorizacao,
		Body:             mdfeBody,
	}); err != nil {
		return nil, err
	}

	return encoded, nil
}

// validateSingleDocType ensures every reference is the same type and returns it.
func validateSingleDocType(docs []MdfeDocRef) (string, error) {
	first := docs[0].Type
	if first != docTypeNFe && first != docTypeCTe {
		return "", problem.BadRequest("tipo de documento inválido: " + first)
	}
	for _, d := range docs {
		if d.Type != first {
			return "", problem.BadRequest("não é possível misturar NF-e e CT-e no mesmo MDF-e")
		}
		if len(d.AccessKey) != 44 {
			return "", problem.BadRequest("chave de acesso inválida: " + d.AccessKey)
		}
	}
	return first, nil
}

// resolveCargo loads each referenced document, downloads + parses its XML, and
// aggregates carregamento/descarregamento, totals and the predominant product.
func (s *MdfeService) resolveCargo(ctx context.Context, orgPK, envPrefix, docType string, req MdfeEmitBody) (*resolvedCargo, error) {
	pk := fmt.Sprintf("%s#%s", envPrefix, orgPK)
	res := &resolvedCargo{
		totalWeight: decimal.Zero,
		totalValue:  decimal.Zero,
		valueByNCM:  map[string]decimal.Decimal{},
	}

	// Group unloading municipalities by IBGE code, preserving first-seen order.
	descIndex := map[string]int{}
	carregaSeen := map[string]bool{}
	predValue := decimal.Zero

	for _, ref := range req.Documents {
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
		// Caller may supply a gross-weight override when the document XML carries
		// no volume/peso (frontend collects it from the user in that case).
		if ref.Weight != "" {
			if w := parseDec(ref.Weight); w.IsPositive() {
				cargo.weightKG = w
			}
		}
		if cargo.weightKG.IsZero() {
			return nil, problem.BadRequest("documento " + ref.AccessKey + " não possui peso; informe o peso da carga")
		}
		res.docs = append(res.docs, cargo)

		// Carregamento = emitter municipality.
		if cargo.emit.cMun != "" && !carregaSeen[cargo.emit.cMun] {
			carregaSeen[cargo.emit.cMun] = true
			res.carrega = append(res.carrega, MdfeMun{IBGECode: cargo.emit.cMun, City: cargo.emit.xMun})
		}
		// Descarregamento = recipient municipality; group doc keys under it.
		dkey := cargo.dest.cMun
		idx, ok := descIndex[dkey]
		if !ok {
			idx = len(res.descarga)
			descIndex[dkey] = idx
			res.descarga = append(res.descarga, descargaGroup{mun: MdfeMun{IBGECode: cargo.dest.cMun, City: cargo.dest.xMun}})
		}
		if ref.Type == docTypeCTe {
			res.descarga[idx].cteKeys = append(res.descarga[idx].cteKeys, ref.accessKeyClean())
		} else {
			res.descarga[idx].nfeKeys = append(res.descarga[idx].nfeKeys, ref.accessKeyClean())
		}

		res.totalWeight = res.totalWeight.Add(cargo.weightKG)
		res.totalValue = res.totalValue.Add(cargo.totalValue)
		if cargo.predNCM != "" {
			res.valueByNCM[cargo.predNCM] = res.valueByNCM[cargo.predNCM].Add(cargo.totalValue)
		}
		if cargo.totalValue.GreaterThan(predValue) {
			predValue = cargo.totalValue
			res.prodPred = MdfeProdPred{TpCarga: defaultTpCarga, XProd: cargo.predProd, NCM: cargo.predNCM}
		}
	}

	// Apply caller overrides.
	if len(req.Loadings) > 0 {
		res.carrega = reorderMuns(res.carrega, req.Loadings)
	}
	if len(req.Unloadings) > 0 {
		res.descarga = reorderDescarga(res.descarga, req.Unloadings)
	}
	if req.Predominant != nil {
		res.prodPred = *req.Predominant
	}
	if res.prodPred.TpCarga == "" {
		res.prodPred.TpCarga = defaultTpCarga
	}
	if res.prodPred.XProd == "" {
		res.prodPred.XProd = "CARGA GERAL"
	}

	// UF início/fim: override, else first carregamento / last descarregamento doc UF.
	res.ufIni = req.UFStart
	if res.ufIni == "" && len(res.docs) > 0 {
		res.ufIni = res.docs[0].emit.uf
	}
	res.ufFim = req.UFEnd
	if res.ufFim == "" && len(res.docs) > 0 {
		res.ufFim = res.docs[len(res.docs)-1].dest.uf
	}
	if res.ufIni == "" || res.ufFim == "" {
		return nil, problem.BadRequest("não foi possível determinar UF de início/fim; informe manualmente")
	}
	if len(res.carrega) == 0 {
		return nil, problem.BadRequest("não foi possível determinar o município de carregamento")
	}
	if len(res.descarga) == 0 {
		return nil, problem.BadRequest("não foi possível determinar o município de descarregamento")
	}
	return res, nil
}

// accessKeyClean returns the 44-digit key without any prefix.
func (r MdfeDocRef) accessKeyClean() string { return r.AccessKey }

// reorderMuns reorders `current` to follow the c_mun order in `desired`,
// keeping any municipality not present in `desired` at the end (original order).
func reorderMuns(current, desired []MdfeMun) []MdfeMun {
	byCMun := make(map[string]MdfeMun, len(current))
	for _, m := range current {
		byCMun[m.IBGECode] = m
	}
	out := make([]MdfeMun, 0, len(current))
	seen := map[string]bool{}
	for _, d := range desired {
		if m, ok := byCMun[d.IBGECode]; ok && !seen[d.IBGECode] {
			out = append(out, m)
			seen[d.IBGECode] = true
		}
	}
	for _, m := range current {
		if !seen[m.IBGECode] {
			out = append(out, m)
			seen[m.IBGECode] = true
		}
	}
	return out
}

// reorderDescarga reorders unloading groups to follow the c_mun order in `desired`.
func reorderDescarga(current []descargaGroup, desired []MdfeMun) []descargaGroup {
	byCMun := make(map[string]descargaGroup, len(current))
	for _, g := range current {
		byCMun[g.mun.IBGECode] = g
	}
	out := make([]descargaGroup, 0, len(current))
	seen := map[string]bool{}
	for _, d := range desired {
		if g, ok := byCMun[d.IBGECode]; ok && !seen[d.IBGECode] {
			out = append(out, g)
			seen[d.IBGECode] = true
		}
	}
	for _, g := range current {
		if !seen[g.mun.IBGECode] {
			out = append(out, g)
			seen[g.mun.IBGECode] = true
		}
	}
	return out
}

// docRepo returns the document repository for the given reference type.
func (s *MdfeService) docRepo(docType string) *repositories.DocumentRepository {
	if docType == docTypeCTe {
		return &s.cteRepo.DocumentRepository
	}
	return &s.nfeRepo.DocumentRepository
}

// resolvedVehicle is the traction vehicle data ready for the enviMDFe builder.
type resolvedVehicle struct {
	Placa   string
	Tara    string
	UF      string
	RENAVAM string
	CapKG   string
	TpRod   string
	TpCar   string
	RNTRC   string // owner RNTRC, when the vehicle is registered with an owner
}

// resolveVehicle merges a registered vehicle (by SK) with the request
// overrides, then blocks with the specific missing fields when the
// registered vehicle isn't complete enough for MDF-e tractor use.
func (s *MdfeService) resolveVehicle(ctx context.Context, orgPK string, v MdfeVehicle) (resolvedVehicle, error) {
	out := resolvedVehicle{
		Placa: v.Placa,
		Tara:  v.Tara,
		UF:    v.UF,
		TpRod: v.TpRod,
		TpCar: v.TpCar,
	}
	if v.RENAVAM != nil {
		out.RENAVAM = *v.RENAVAM
	}
	if v.CapKG != nil {
		out.CapKG = *v.CapKG
	}

	if v.SK != nil && s.vehicleRepo != nil {
		vehicle, err := s.vehicleRepo.Get(ctx, orgPK, *v.SK)
		if err != nil {
			return resolvedVehicle{}, err
		}
		if vehicle == nil {
			return resolvedVehicle{}, problem.NotFound("veículo não encontrado: " + *v.SK)
		}
		if missing := services.Missing(vehicle, services.DocTypeMdfe, services.VehicleRoleTractor); len(missing) > 0 {
			return resolvedVehicle{}, problem.BadRequest("veículo incompleto para MDF-e (tração): campos faltando: " + strings.Join(missing, ", "))
		}
		if out.Placa == "" {
			out.Placa = strAttr(vehicle, "plate")
		}
		if out.UF == "" {
			out.UF = strAttr(vehicle, "plate_uf")
		}
		// organization_vehicles stores tara as numeric "weight"; tpRod/tpCar as
		// "wheelset"/"bodywork" (already MDF-e codes).
		if out.Tara == "" {
			if w := strconv.Itoa(intAttr(vehicle, "weight", 0)); w != "0" {
				out.Tara = w
			}
		}
		if out.TpRod == "" {
			out.TpRod = strAttr(vehicle, "wheelset")
		}
		if out.TpCar == "" {
			out.TpCar = strAttr(vehicle, "bodywork")
		}
		if out.RENAVAM == "" {
			out.RENAVAM = strAttr(vehicle, "renavam")
		}
		if owner, ok := vehicle["owner"].(*types.AttributeValueMemberM); ok {
			if r, ok := owner.Value["rntrc"].(*types.AttributeValueMemberS); ok {
				out.RNTRC = r.Value
			}
		}
	}

	return out, nil
}

// resolveTrailers resolves each trailer SK into a resolvedVehicle, blocking
// with the specific missing fields when a trailer isn't complete enough for
// MDF-e use (tara, cap_kg, bodywork — see services.Missing).
func (s *MdfeService) resolveTrailers(ctx context.Context, orgPK string, trailers []MdfeTrailer) ([]resolvedVehicle, error) {
	out := make([]resolvedVehicle, 0, len(trailers))
	for _, t := range trailers {
		vehicle, err := s.vehicleRepo.Get(ctx, orgPK, t.SK)
		if err != nil {
			return nil, err
		}
		if vehicle == nil {
			return nil, problem.NotFound("reboque não encontrado: " + t.SK)
		}
		if missing := services.Missing(vehicle, services.DocTypeMdfe, services.VehicleRoleTrailer); len(missing) > 0 {
			return nil, problem.BadRequest("reboque incompleto para MDF-e: campos faltando: " + strings.Join(missing, ", "))
		}
		rv := resolvedVehicle{
			Placa:   strAttr(vehicle, "plate"),
			UF:      strAttr(vehicle, "plate_uf"),
			TpCar:   strAttr(vehicle, "bodywork"),
			RENAVAM: strAttr(vehicle, "renavam"),
		}
		if w := strconv.Itoa(intAttr(vehicle, "weight", 0)); w != "0" {
			rv.Tara = w
		}
		if c := strconv.Itoa(intAttr(vehicle, "cap_kg", 0)); c != "0" {
			rv.CapKG = c
		}
		out = append(out, rv)
	}
	return out, nil
}

// resolveOwner validates and normalises a third-party traction-vehicle owner.
// Returns (nil, nil) when no owner is supplied (vehicle belongs to the emitter —
// carga própria). Enforces SEFAZ rules: exactly one of CPF/CNPJ, RNTRC and name
// required, and the owner must differ from the MDF-e emitter (F21/cStat 740).
func resolveOwner(o *MdfeOwner, orgPK string) (*resolvedOwner, error) {
	if o == nil {
		return nil, nil
	}
	cpf := onlyDigits(o.CPF)
	cnpj := onlyDigits(o.CNPJ)
	if (cpf == "") == (cnpj == "") {
		return nil, problem.BadRequest("proprietário do veículo: informe exatamente um entre CPF e CNPJ")
	}
	if o.RNTRC == "" {
		return nil, problem.BadRequest("proprietário do veículo: RNTRC é obrigatório")
	}
	if o.Name == "" {
		return nil, problem.BadRequest("proprietário do veículo: nome/razão social é obrigatório")
	}
	emitterDoc := services.StripPKPrefix(orgPK)
	if (cpf != "" && cpf == emitterDoc) || (cnpj != "" && cnpj == emitterDoc) {
		return nil, problem.BadRequest("proprietário do veículo deve ser diferente do emitente do MDF-e")
	}
	return &resolvedOwner{
		CPF:      cpf,
		CNPJ:     cnpj,
		Name:     o.Name,
		IE:       o.IE,
		UF:       o.UF,
		RNTRC:    o.RNTRC,
		TpProp:   o.TpProp,
		TpTransp: o.TpTransp,
	}, nil
}

// buildRecord assembles the DynamoDB summary record for the MDF-e.
func (s *MdfeService) buildRecord(
	pk, accessKey, orgPK string, orgItem map[string]types.AttributeValue,
	number, serie int, now time.Time, modal, docType string,
	cargo *resolvedCargo, vehicle resolvedVehicle, owner *resolvedOwner, req MdfeEmitBody,
	userID, userName string,
) map[string]any {
	docKeys := make([]map[string]any, 0, len(req.Documents))
	for _, d := range req.Documents {
		docKeys = append(docKeys, map[string]any{"type": d.Type, "access_key": d.AccessKey})
	}
	loadings := make([]map[string]any, 0, len(cargo.carrega))
	for _, m := range cargo.carrega {
		loadings = append(loadings, map[string]any{"ibge_code": m.IBGECode, "city": m.City})
	}
	unloadings := make([]map[string]any, 0, len(cargo.descarga))
	for _, g := range cargo.descarga {
		keys := make([]string, 0, len(g.nfeKeys)+len(g.cteKeys))
		keys = append(keys, g.nfeKeys...)
		keys = append(keys, g.cteKeys...)
		unloadings = append(unloadings, map[string]any{
			"ibge_code":   g.mun.IBGECode,
			"city":        g.mun.City,
			"access_keys": keys,
		})
	}
	drivers := make([]map[string]any, 0, len(req.Drivers))
	for _, d := range req.Drivers {
		drivers = append(drivers, map[string]any{"name": d.Name, "cpf": onlyDigits(d.CPF)})
	}

	vehicleRec := map[string]any{"placa": vehicle.Placa, "uf": vehicle.UF, "tara": vehicle.Tara, "rntrc": vehicle.RNTRC}
	if owner != nil {
		vehicleRec["owner"] = map[string]any{
			"cpf": owner.CPF, "cnpj": owner.CNPJ, "name": owner.Name, "rntrc": owner.RNTRC,
		}
	}

	record := map[string]any{
		"pk":            pk,
		"sk":            accessKey,
		"incoming":      0,
		"year":          now.Year(),
		"month":         int(now.Month()),
		"day":           now.Day(),
		"status":        StatusPending,
		"sefaz_status":  nil,
		"sefaz_motive":  nil,
		"emit_cpf_cnpj": services.StripPKPrefix(orgPK),
		"emit_name":     strAttr(orgItem, "name"),
		"number":        number,
		"serie":         serie,
		"modal":         modal,
		"doc_type":      docType,
		"documents":     docKeys,
		"uf_start":      cargo.ufIni,
		"uf_end":        cargo.ufFim,
		"route":         req.Route,
		"loadings":      loadings,
		"unloadings":    unloadings,
		"cargo_weight":  cargo.totalWeight.StringFixed(4),
		"cargo_value":   cargo.totalValue.StringFixed(2),
		"predominant":   map[string]any{"tp_carga": cargo.prodPred.TpCarga, "x_prod": cargo.prodPred.XProd, "ncm": cargo.prodPred.NCM},
		"vehicle":       vehicleRec,
		"drivers":       drivers,
		"dh_emi":        now.Format(layoutDateTimeTZ),
		"xml_s3_key":    nil,
		"created_at":    now.UTC().Format(time.RFC3339),
		"user_id":       userID,
		"user_name":     userName,
	}
	if req.TripStart != nil && *req.TripStart != "" {
		record["trip_start"] = *req.TripStart
	}
	if req.BulkCargo != nil {
		record["bulk_cargo"] = map[string]any{
			"cep_loading":   onlyDigits(req.BulkCargo.CEPLoading),
			"cep_unloading": onlyDigits(req.BulkCargo.CEPUnloading),
		}
	}
	return record
}
