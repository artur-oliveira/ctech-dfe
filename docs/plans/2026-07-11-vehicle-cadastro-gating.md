# Vehicle Cadastro Minimal-Fields + Emission Gating Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this
> plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make vehicle cadastro require only `plate`+`plate_uf`, turn trailers into first-class
independently-selectable vehicle records, and block MDF-e emission (with a reused edit modal)
when the selected vehicle is missing a field that specific doc-type/role actually needs —
replacing the current silent `tpRod`/`tpCar` defaulting.

**Architecture:** A single Go-owned required-fields matrix (`services.Missing`) is the one
source of truth for "what does this vehicle need for this doc-type+role"; a new
`GET /vehicles/:sk/requirements` endpoint exposes it to the UI so no matrix logic is duplicated
in TypeScript. MDF-e emission gains real `veicReboque` (trailer) support, which didn't exist
before this plan.

**Tech Stack:** Go (Fiber v3, DynamoDB SDK v2), Next.js/TypeScript (zod, react-hook-form,
TanStack Query), AWS CDK (TypeScript), Python (py-dfe — no changes needed, already supports
`veicReboque`).

## Global Constraints

- No data migration — `organization_vehicles` is pre-launch/empty (per design spec).
- Do not rename existing fields (`wheelset`, `bodywork`, `weight`, `renavam`, `owner`, `plate`,
  `plate_uf`) — only relax required→optional and add new fields.
- `owner`/third-party-ownership data stays exactly as-is (per-emission `MdfeOwner` input) — not
  part of the vehicle record or the gating matrix.
- No CT-e OS emission wiring — that service doesn't exist yet; only add its (empty) matrix row.
- No NF-e reboque wiring — NF-e's `veicTransp`/`reboque` have no required fields regardless.
- `api`: all errors via `problem.*` helpers (never raw errors/`fiber.Map`). DynamoDB access via
  `Query`, never `Scan`. No goroutines in route handlers.
- `ui`: `npx eslint src --ext .ts,.tsx` must pass with zero errors/warnings before considering
  any UI task done.
- Every core function needs a test per each subproject's `CLAUDE.md` testing table (service
  logic → unit; repository/AWS integration → integration; new endpoint → unit+integration; form
  validation → zod unit test).

---

### Task 1: CDK — add `role-index` GSI to `organization_vehicles`

**Files:**
- Modify: `cdk/lib/dynamodb-stack.ts:368-389`

**Interfaces:**
- Produces: `role-index` GSI (PK: `pk`, SK: `role`) on the `organization_vehicles` table, used
  by Task 2's `ListByRole`.

- [ ] **Step 1: Add the GSI definition**

In `cdk/lib/dynamodb-stack.ts`, right after the existing `plate-index` GSI block (ends at line
388 with `});`), add:

```typescript
        vehiclesTable.addGlobalSecondaryIndex({
            indexName: 'role-index',
            partitionKey: {name: 'pk', type: dynamodb.AttributeType.STRING},
            sortKey: {name: 'role', type: dynamodb.AttributeType.STRING},
            projectionType: dynamodb.ProjectionType.ALL,
            warmThroughput: undefined,
            maxReadRequestUnits: 10,
            maxWriteRequestUnits: 10,
        });
```

- [ ] **Step 2: Synth to verify it compiles**

Run: `cd cdk && npx cdk synth CtechDfe-Prod-DynamoDB > /dev/null`
Expected: no errors; exit code 0.

- [ ] **Step 3: Commit**

```bash
git add cdk/lib/dynamodb-stack.ts
git commit -m "feat(cdk): add role-index GSI to organization_vehicles table"
```

---

### Task 2: Repository — flat schema, `role`, drop `trailers[]`, add `ListByRole`

**Files:**
- Modify: `api/internal/repositories/vehicles.go`
- Test: `api/internal/repositories/vehicles_test.go` (new)

**Interfaces:**
- Produces: `type VehicleFields struct{Plate, PlateUF, Role, Wheelset, Bodywork, Renavam string;
  Weight, CapKG, CapM3 int; Cint string; Owner map[string]any}`; `Create(ctx, orgPK string, f
  VehicleFields) (map[string]types.AttributeValue, error)`;
  `BuildCreateTxItem(orgPK string, f VehicleFields) (types.TransactWriteItem,
  map[string]types.AttributeValue)`; `ListByRole(ctx context.Context, orgPK, role string, opts
  VehicleListOpts) (*QueryResult, error)`.
- Consumes: `Base.Query`, `Base.PutItem`, `Base.BuildPutTxItem`, `deleteNulls` (all existing,
  `api/internal/repositories/base.go` and `marshal.go`).

- [ ] **Step 1: Write the failing repository test**

Create `api/internal/repositories/vehicles_test.go`:

```go
package repositories

import "testing"

func TestBuildVehicleSK_AddsPrefix(t *testing.T) {
	if got := buildVehicleSK("abc123"); got != "VEHICLE_abc123" {
		t.Errorf("buildVehicleSK(%q) = %q, want VEHICLE_abc123", "abc123", got)
	}
}

func TestBuildVehicleSK_IdempotentWithPrefix(t *testing.T) {
	if got := buildVehicleSK("VEHICLE_abc123"); got != "VEHICLE_abc123" {
		t.Errorf("buildVehicleSK(%q) = %q, want unchanged", "VEHICLE_abc123", got)
	}
}
```

(This mirrors the existing behavior — it will already pass. It's a placeholder anchor; the real
new-behavior test is the integration test in Task 9, since `ListByRole`/`Create` need a live
DynamoDB Query/PutItem call that only the integration suite exercises, per
`api/CLAUDE.md`: "Repository | Integration test (DynamoDB)".)

- [ ] **Step 2: Run it to confirm the file compiles and passes**

Run: `cd api && go test ./internal/repositories/... -run TestBuildVehicleSK -v`
Expected: both tests PASS.

- [ ] **Step 3: Replace `vehicles.go` with the flat-schema version**

Replace the entire content of `api/internal/repositories/vehicles.go` with:

```go
package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/config"
)

type VehicleRepository struct {
	Base
}

func NewVehicleRepository(db *dynamodb.Client, cfg *config.Config) *VehicleRepository {
	return &VehicleRepository{Base: NewBase(db, cfg, "organization_vehicles")}
}

func buildVehicleSK(sk string) string {
	if strings.HasPrefix(sk, "VEHICLE_") {
		return sk
	}
	return fmt.Sprintf("VEHICLE_%s", sk)
}

// VehicleFields is the flat set of attributes stored on organization_vehicles.
// Only Plate, PlateUF and Role are ever required; everything else may be the
// zero value (unset) at cadastro time and completed later — see
// services.Missing for what each doc-type/role actually requires at emission.
type VehicleFields struct {
	Plate    string
	PlateUF  string
	Role     string // "tractor" | "trailer"
	Wheelset string
	Bodywork string
	Renavam  string
	Weight   int
	CapKG    int
	CapM3    int
	Cint     string
	Owner    map[string]any
}

func (r *VehicleRepository) buildItem(orgPK, sk string, f VehicleFields, now string) map[string]types.AttributeValue {
	ownerAV, _ := attributevalue.MarshalMap(f.Owner)
	item := map[string]types.AttributeValue{
		"pk":         &types.AttributeValueMemberS{Value: orgPK},
		"sk":         &types.AttributeValueMemberS{Value: sk},
		"plate":      &types.AttributeValueMemberS{Value: f.Plate},
		"plate_uf":   &types.AttributeValueMemberS{Value: f.PlateUF},
		"role":       &types.AttributeValueMemberS{Value: f.Role},
		"wheelset":   &types.AttributeValueMemberS{Value: f.Wheelset},
		"bodywork":   &types.AttributeValueMemberS{Value: f.Bodywork},
		"renavam":    &types.AttributeValueMemberS{Value: f.Renavam},
		"weight":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.Weight)},
		"cap_kg":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.CapKG)},
		"cap_m3":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", f.CapM3)},
		"cint":       &types.AttributeValueMemberS{Value: f.Cint},
		"owner":      &types.AttributeValueMemberM{Value: ownerAV},
		"created_at": &types.AttributeValueMemberS{Value: now},
		"updated_at": &types.AttributeValueMemberS{Value: now},
	}
	deleteNulls(item)
	return item
}

func (r *VehicleRepository) Create(ctx context.Context, orgPK string, f VehicleFields) (map[string]types.AttributeValue, error) {
	now := NowStr()
	item := r.buildItem(orgPK, buildVehicleSK(GenerateID()), f, now)
	return item, r.PutItem(ctx, item)
}

func (r *VehicleRepository) Get(ctx context.Context, orgPK, sk string) (map[string]types.AttributeValue, error) {
	return r.GetItem(ctx, orgPK, buildVehicleSK(sk))
}

type VehicleListOpts struct {
	PlatePrefix string
	Sort        string
	Limit       int
	StartKey    map[string]types.AttributeValue
}

// List returns vehicles for an org, optionally filtered by plate prefix.
func (r *VehicleRepository) List(ctx context.Context, orgPK string, opts VehicleListOpts) (*QueryResult, error) {
	forward := opts.Sort != "desc"
	if opts.PlatePrefix != "" {
		return r.Query(ctx, QueryOpts{
			PK: orgPK, SKPrefix: opts.PlatePrefix,
			IndexName: "plate-index", SKField: "plate",
			ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
		})
	}
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: "VEHICLE_",
		ScanIndexForward: forward, Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

// ListByRole returns vehicles for an org filtered to one role ("tractor" or
// "trailer") via role-index, without a Scan.
func (r *VehicleRepository) ListByRole(ctx context.Context, orgPK, role string, opts VehicleListOpts) (*QueryResult, error) {
	return r.Query(ctx, QueryOpts{
		PK: orgPK, SKPrefix: role,
		IndexName: "role-index", SKField: "role",
		ScanIndexForward: opts.Sort != "desc", Limit: opts.Limit, ExclusiveStartKey: opts.StartKey,
	})
}

func (r *VehicleRepository) Update(ctx context.Context, orgPK, sk string, updates map[string]any) (bool, error) {
	updates["updated_at"] = NowStr()
	return r.UpdateItem(ctx, orgPK, new(buildVehicleSK(sk)), updates)
}

func (r *VehicleRepository) Delete(ctx context.Context, orgPK, sk string) (bool, error) {
	return r.DeleteItem(ctx, orgPK, buildVehicleSK(sk))
}

// BuildCreateTxItem returns a TransactWriteItem for a new vehicle, mirroring
// Create's key/timestamp/field construction, without writing.
func (r *VehicleRepository) BuildCreateTxItem(orgPK string, f VehicleFields) (types.TransactWriteItem, map[string]types.AttributeValue) {
	now := NowStr()
	item := r.buildItem(orgPK, buildVehicleSK(GenerateID()), f, now)
	return r.BuildPutTxItem(item), item
}

// BuildUpdateTxItem returns a TransactWriteItem for updating an existing
// vehicle, mirroring Update's timestamp bump, without writing.
func (r *VehicleRepository) BuildUpdateTxItem(orgPK, sk string, updates map[string]any) (types.TransactWriteItem, error) {
	updates["updated_at"] = NowStr()
	return r.Base.BuildUpdateTxItem(orgPK, new(buildVehicleSK(sk)), updates)
}

// BuildDeleteTxItem returns a TransactWriteItem for deleting a vehicle, without writing.
func (r *VehicleRepository) BuildDeleteTxItem(orgPK, sk string) types.TransactWriteItem {
	return r.Base.BuildDeleteTxItem(orgPK, buildVehicleSK(sk))
}
```

- [ ] **Step 4: Run tests, confirm the package still compiles**

Run: `cd api && go build ./... && go test ./internal/repositories/... -v`
Expected: build succeeds; `TestBuildVehicleSK_*` PASS. (Callers in `services/vehicles.go` will
be broken until Task 4 — that's expected and fixed in that task; if you're running
`go build ./...` at the whole-module level before Task 4 lands, it will fail on those callers.
Scope this build/test run to `./internal/repositories/...` only for now.)

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/vehicles.go api/internal/repositories/vehicles_test.go
git commit -m "feat(api): flatten organization_vehicles schema, add role + ListByRole"
```

---

### Task 3: DTOs — relax required fields, add `role`/`cap_kg`/`cap_m3`/`cint`, drop trailers

**Files:**
- Modify: `api/internal/api/v1/dto.go:237-278`
- Modify: `api/internal/api/v1/dto_test.go:59-74`

**Interfaces:**
- Produces: `VehicleCreateBody{Plate, PlateUf, Role string; Wheelset, Bodywork, Renavam string
  \`omitempty\`; Weight, CapKG, CapM3 int \`omitempty\`; Cint string \`omitempty\`; Owner
  *VehicleOwnerBody \`omitempty\`}`, `VehicleUpdateBody` mirrored with all-pointer fields except
  no change needed to its already-optional shape besides adding `Role`, `CapKG`, `CapM3`,
  `Cint` and making `Owner` a plain pointer already (unchanged). Removes
  `VehicleTrailerBody` entirely.

- [ ] **Step 1: Update `dto_test.go`'s `validVehicle()` fixture to add `Role`**

In `api/internal/api/v1/dto_test.go:59-74`, change:

```go
func validVehicle() VehicleCreateBody {
	return VehicleCreateBody{
		Plate:    "ABC1D23",
		PlateUf:  "SP",
		Wheelset: "02",
		Bodywork: "01",
		Renavam:  "123456789",
		Weight:   8000,
		Owner: VehicleOwnerBody{
			CpfCnpj: "11222333000181",
			Rntrc:   "12345678",
			Name:    "Transportadora X",
			Type:    "ETC",
		},
	}
}
```

to:

```go
func validVehicle() VehicleCreateBody {
	return VehicleCreateBody{
		Plate:   "ABC1D23",
		PlateUf: "SP",
		Role:    "tractor",
	}
}

func validVehicleWithAllFields() VehicleCreateBody {
	return VehicleCreateBody{
		Plate:    "ABC1D23",
		PlateUf:  "SP",
		Role:     "tractor",
		Wheelset: "02",
		Bodywork: "01",
		Renavam:  "123456789",
		Weight:   8000,
		CapKG:    12000,
		CapM3:    40,
		Cint:     "INT-01",
		Owner: &VehicleOwnerBody{
			CpfCnpj: "11222333000181",
			Rntrc:   "12345678",
			Name:    "Transportadora X",
			Type:    "ETC",
		},
	}
}
```

Then in `TestValidDTOsPass` (`dto_test.go:77-90`), add the new fixture to the `cases` map:

```go
		"vehicle":            validVehicle(),
		"vehicle-all-fields": validVehicleWithAllFields(),
```

- [ ] **Step 2: Run the test, confirm it fails (fields don't exist yet)**

Run: `cd api && go test ./internal/api/v1/... -run TestValidDTOsPass -v`
Expected: FAIL — compile error, `VehicleCreateBody` has no field `Role`/`CapKG`/`CapM3`/`Cint`,
or `Owner` type mismatch (`VehicleOwnerBody` vs `*VehicleOwnerBody`).

- [ ] **Step 3: Update `dto.go`**

In `api/internal/api/v1/dto.go`, replace lines 237-278 (from `// VehicleOwnerBody...` through
the end of `VehicleUpdateBody`) with:

```go
// VehicleOwnerBody is the owner (proprietário) of a vehicle. Optional static
// metadata — not used for MDF-e prop building (that's a per-emission input,
// see mdfes.MdfeOwner); kept only as informational fleet-management data.
type VehicleOwnerBody struct {
	CpfCnpj string `json:"cpf_cnpj" validate:"required,cpfcnpj"`
	Rntrc   string `json:"rntrc" validate:"required,rntrc"`
	Name    string `json:"name" validate:"required,min=2,max=255"`
	Type    string `json:"type" validate:"required,oneof=TAC ETC CTC"`
}

// VehicleCreateBody is the body for POST /vehicles. Only Plate, PlateUf and
// Role are required — everything else is completed later, gated at
// emission time per doc-type/role (see services.Missing).
type VehicleCreateBody struct {
	Plate    string            `json:"plate" validate:"required,placa"`
	PlateUf  string            `json:"plate_uf" validate:"required,uf"`
	Role     string            `json:"role" validate:"required,oneof=tractor trailer"`
	Wheelset string            `json:"wheelset" validate:"omitempty"`
	Bodywork string            `json:"bodywork" validate:"omitempty"`
	Renavam  string            `json:"renavam" validate:"omitempty,renavam"`
	Weight   int                `json:"weight" validate:"omitempty,gte=0"`
	CapKG    int                `json:"cap_kg" validate:"omitempty,gte=0"`
	CapM3    int                `json:"cap_m3" validate:"omitempty,gte=0"`
	Cint     string            `json:"cint" validate:"omitempty,max=10"`
	Owner    *VehicleOwnerBody `json:"owner" validate:"omitempty"`
}

// VehicleUpdateBody is the body for PUT /vehicles/:sk (partial).
type VehicleUpdateBody struct {
	Plate    *string           `json:"plate" validate:"omitempty,placa"`
	PlateUf  *string           `json:"plate_uf" validate:"omitempty,uf"`
	Role     *string           `json:"role" validate:"omitempty,oneof=tractor trailer"`
	Wheelset *string           `json:"wheelset" validate:"omitempty"`
	Bodywork *string           `json:"bodywork" validate:"omitempty"`
	Renavam  *string           `json:"renavam" validate:"omitempty,renavam"`
	Weight   *int              `json:"weight" validate:"omitempty,gte=0"`
	CapKG    *int              `json:"cap_kg" validate:"omitempty,gte=0"`
	CapM3    *int              `json:"cap_m3" validate:"omitempty,gte=0"`
	Cint     *string           `json:"cint" validate:"omitempty,max=10"`
	Owner    *VehicleOwnerBody `json:"owner" validate:"omitempty"`
}
```

This removes `VehicleTrailerBody` entirely and the `Trailers` field from both bodies.

- [ ] **Step 4: Run the test, confirm it passes**

Run: `cd api && go test ./internal/api/v1/... -run TestValidDTOsPass -v`
Expected: PASS for both `vehicle` and `vehicle-all-fields` cases.

- [ ] **Step 5: Commit**

```bash
git add api/internal/api/v1/dto.go api/internal/api/v1/dto_test.go
git commit -m "feat(api): relax vehicle DTOs to plate+plate_uf+role only, drop trailers"
```

---

### Task 4: Required-fields matrix — `services.Missing`

**Files:**
- Create: `api/internal/services/vehicle_requirements.go`
- Test: `api/internal/services/vehicle_requirements_test.go`

**Interfaces:**
- Produces: constants `DocTypeMdfe = "mdfe"`, `DocTypeNfe = "nfe"`, `DocTypeCteOS = "cte_os"`,
  `VehicleRoleTractor = "tractor"`, `VehicleRoleTrailer = "trailer"`; function
  `Missing(item map[string]types.AttributeValue, docType, role string) []string` returning the
  JSON field names (e.g. `"weight"`, `"bodywork"`) still missing for that doc-type+role, empty
  slice when ready.
- Consumes: `github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue`,
  `github.com/aws/aws-sdk-go-v2/service/dynamodb/types` (both already imported elsewhere in
  this package).

- [ ] **Step 1: Write the failing unit tests**

Create `api/internal/services/vehicle_requirements_test.go`:

```go
package services

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func vehicleItem(t *testing.T, fields map[string]any) map[string]types.AttributeValue {
	t.Helper()
	av, err := attributevalue.MarshalMap(fields)
	if err != nil {
		t.Fatalf("MarshalMap: %v", err)
	}
	return av
}

func TestMissing_MdfeTractor_AllPresent(t *testing.T) {
	item := vehicleItem(t, map[string]any{"weight": 8000, "wheelset": "01", "bodywork": "00"})
	if got := Missing(item, DocTypeMdfe, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty", got)
	}
}

func TestMissing_MdfeTractor_AllAbsent(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	got := Missing(item, DocTypeMdfe, VehicleRoleTractor)
	want := []string{"weight", "wheelset", "bodywork"}
	if len(got) != len(want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Missing()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMissing_MdfeTrailer_RequiresCapKGNotWheelset(t *testing.T) {
	item := vehicleItem(t, map[string]any{"weight": 8000, "bodywork": "00"})
	got := Missing(item, DocTypeMdfe, VehicleRoleTrailer)
	if len(got) != 1 || got[0] != "cap_kg" {
		t.Errorf("Missing() = %v, want [cap_kg]", got)
	}
}

func TestMissing_Nfe_NeverRequiresAnything(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	if got := Missing(item, DocTypeNfe, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty for nfe", got)
	}
}

func TestMissing_CteOS_NeverRequiresAnything(t *testing.T) {
	item := vehicleItem(t, map[string]any{})
	if got := Missing(item, DocTypeCteOS, VehicleRoleTractor); len(got) != 0 {
		t.Errorf("Missing() = %v, want empty for cte_os", got)
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

Run: `cd api && go test ./internal/services/... -run TestMissing -v`
Expected: FAIL — compile error, `Missing`/`DocTypeMdfe`/etc. undefined.

- [ ] **Step 3: Implement `vehicle_requirements.go`**

Create `api/internal/services/vehicle_requirements.go`:

```go
package services

import (
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Doc-type and vehicle-role constants for the required-fields matrix.
const (
	DocTypeMdfe  = "mdfe"
	DocTypeNfe   = "nfe"
	DocTypeCteOS = "cte_os"

	VehicleRoleTractor = "tractor"
	VehicleRoleTrailer = "trailer"
)

// vehicleRequirementFields mirrors the subset of organization_vehicles
// attributes the required-fields matrix inspects.
type vehicleRequirementFields struct {
	Weight   int    `dynamodbav:"weight"`
	Wheelset string `dynamodbav:"wheelset"`
	Bodywork string `dynamodbav:"bodywork"`
	CapKG    int    `dynamodbav:"cap_kg"`
}

// Missing is the single source of truth for which vehicle fields a given
// doc-type + role combination requires beyond plate/plate_uf (always
// required at cadastro). Returns the JSON field names still missing, in a
// fixed order; empty when the vehicle is ready for that doc-type+role.
//
// NF-e and CT-e OS never require anything beyond plate per their XSDs
// (veicTransp/veic have no other required fields) — their rows exist so the
// matrix is ready when CT-e OS emission is built.
func Missing(item map[string]types.AttributeValue, docType, role string) []string {
	var v vehicleRequirementFields
	_ = attributevalue.UnmarshalMap(item, &v)

	var missing []string
	if docType != DocTypeMdfe {
		return missing
	}
	if v.Weight == 0 {
		missing = append(missing, "weight")
	}
	switch role {
	case VehicleRoleTractor:
		if v.Wheelset == "" {
			missing = append(missing, "wheelset")
		}
		if v.Bodywork == "" {
			missing = append(missing, "bodywork")
		}
	case VehicleRoleTrailer:
		if v.CapKG == 0 {
			missing = append(missing, "cap_kg")
		}
		if v.Bodywork == "" {
			missing = append(missing, "bodywork")
		}
	}
	return missing
}
```

- [ ] **Step 4: Run to confirm it passes**

Run: `cd api && go test ./internal/services/... -run TestMissing -v`
Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/vehicle_requirements.go api/internal/services/vehicle_requirements_test.go
git commit -m "feat(api): add vehicle required-fields matrix (services.Missing)"
```

---

### Task 5: Service layer — `role` on create, drop trailers handling

**Files:**
- Modify: `api/internal/services/vehicles.go:86-134` (Create), `:138-193` (Update)

**Interfaces:**
- Consumes: `repositories.VehicleFields` (Task 2), `Missing`/constants (Task 4, not called here
  yet — cadastro itself is never gated, only emission is).
- Produces: `VehicleService.Create`/`Update` now build `repositories.VehicleFields` instead of
  positional args, and no longer touch `trailers`.

- [ ] **Step 1: Write the failing test**

Add to `api/internal/services/vehicles_test.go`:

```go
func TestValidatePlate_StillWorksAfterRoleField(t *testing.T) {
	// Guard: relaxing Create's field extraction must not affect plate validation.
	if err := ValidatePlate("XYZ9A87"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

(This is a thin regression guard — the meaningful new-behavior coverage for `Create` itself,
since it needs a real/fake DynamoDB, lives in the integration test in Task 9 per
`api/CLAUDE.md`'s "Repository | Integration test" / "New endpoint | Unit + integration" rows.)

- [ ] **Step 2: Run to confirm it currently passes (compiles)**

Run: `cd api && go build ./internal/services/... 2>&1 | head -30`
Expected: build FAILS right now — `vehicles.go` still calls the old `repo.Create(...)` /
`repo.BuildCreateTxItem(...)` positional signature that Task 2 already changed. This is the
"red" state driving this task.

- [ ] **Step 3: Update `Create` and `Update` in `vehicles.go`**

Replace `api/internal/services/vehicles.go:86-134` (the `Create` method) with:

```go
// Create builds strongly-typed arguments then writes the vehicle and its
// CREATE audit row atomically.
func (s *VehicleService) Create(ctx context.Context, orgPK string, fields map[string]any, userID, userName string) (map[string]types.AttributeValue, error) {
	plate, _ := fields["plate"].(string)
	if err := ValidatePlate(plate); err != nil {
		return nil, err
	}
	renavam, _ := fields["renavam"].(string)
	if renavam != "" {
		if err := validateRenavam(renavam); err != nil {
			return nil, err
		}
	}
	owner, _ := fields["owner"].(map[string]any)
	if ownerType, _ := owner["type"].(string); ownerType != "" && !validOwnerTypes[ownerType] {
		return nil, problem.BadRequest("owner_type must be TAC, ETC, or CTC")
	}

	f := repositories.VehicleFields{
		Plate:    plate,
		PlateUF:  strField(fields, "plate_uf"),
		Role:     strField(fields, "role"),
		Wheelset: strField(fields, "wheelset"),
		Bodywork: strField(fields, "bodywork"),
		Renavam:  renavam,
		Weight:   intField(fields, "weight"),
		CapKG:    intField(fields, "cap_kg"),
		CapM3:    intField(fields, "cap_m3"),
		Cint:     strField(fields, "cint"),
		Owner:    owner,
	}

	vehicleTx, finalItem := s.repo.BuildCreateTxItem(orgPK, f)

	afterMap, err := attributeMapToPlain(finalItem)
	if err != nil {
		return nil, err
	}
	auditTx, err := s.auditRepo.BuildLogTxItem(
		orgPK, repositories.AuditResourceVehicle, attrStrAV(finalItem, "sk"), repositories.AuditActionCreate,
		userID, userName, Diff(nil, afterMap),
	)
	if err != nil {
		return nil, err
	}

	if err := s.repo.TransactWrite(ctx, []types.TransactWriteItem{vehicleTx, auditTx}); err != nil {
		return nil, err
	}
	_ = s.cache.DeletePrefix(ctx, vehicleListCachePrefix(orgPK))
	return finalItem, nil
}

// strField/intField pull a plain string/int out of the untyped fields map
// produced by structToMap, defaulting to the zero value when absent —
// matching the existing "zero value means unset" convention already used by
// resolveVehicle for weight/wheelset/bodywork.
func strField(fields map[string]any, key string) string {
	v, _ := fields[key].(string)
	return v
}

func intField(fields map[string]any, key string) int {
	switch v := fields[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
```

`Update` (`vehicles.go:138-193`) needs no structural change — it already forwards the raw
`updates map[string]any` straight to `s.repo.BuildUpdateTxItem`/`UpdateItem`, which is
field-name-agnostic. Only remove the now-dead `weight`/`wheelset`/`bodywork` extraction that
doesn't exist there (there isn't any — confirm by re-reading the method; no edit needed there
beyond what Task 2/3 already changed upstream).

Also remove the now-unused `ownerType`/`weight` locals still hanging around from before this
edit if `go vet` flags them (check with Step 4).

- [ ] **Step 4: Run to confirm it passes**

Run: `cd api && go build ./... && go test ./internal/services/... -v`
Expected: whole module builds; all `services` package tests PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/vehicles.go api/internal/services/vehicles_test.go
git commit -m "refactor(api): VehicleService.Create builds repositories.VehicleFields"
```

---

### Task 6: Routes — role filter + `GET /vehicles/:sk/requirements`

**Files:**
- Modify: `api/internal/api/v1/vehicles.go`

**Interfaces:**
- Consumes: `services.Missing`, `services.DocTypeMdfe/Nfe/CteOS`,
  `services.VehicleRoleTractor/Trailer` (Task 4); `repositories.VehicleRepository.ListByRole`
  (Task 2).
- Produces: `GET /vehicles?role=trailer` (list filter); `GET
  /vehicles/:sk/requirements?doc_type=mdfe&role=tractor` → `200 {"missing": [...]}`.

- [ ] **Step 1: Write the failing integration test**

Add to `api/tests/integration/vehicles_test.go` (append a new test function near the existing
vehicle CRUD tests — match that file's existing setup/helper pattern, e.g. its org/auth fixture
helper, by reading the top of the file for the exact helper names before writing this):

```go
func TestGetVehicleRequirements_IncompleteTractorListsMissingFields(t *testing.T) {
	// Uses this file's existing test-org + auth-header setup (see other tests
	// in this file for the exact helper calls).
	created := createTestVehicle(t, map[string]any{
		"plate": "ABC1D23", "plate_uf": "SP", "role": "tractor",
	})
	resp := doGet(t, fmt.Sprintf("/v1.0/vehicles/%s/requirements?doc_type=mdfe&role=tractor", created["sk"]))
	var body struct{ Missing []string `json:"missing"` }
	decodeJSON(t, resp, &body)
	want := []string{"weight", "wheelset", "bodywork"}
	if len(body.Missing) != len(want) {
		t.Fatalf("missing = %v, want %v", body.Missing, want)
	}
}
```

(`createTestVehicle`, `doGet`, `decodeJSON` are placeholders for whatever this file's existing
helpers are actually named — inspect `api/tests/integration/vehicles_test.go`'s current tests
before writing this and reuse the exact existing helpers/fixtures rather than inventing new
ones, per this repo's DRY rule.)

- [ ] **Step 2: Run to confirm it fails**

Run: `cd api && go test ./tests/integration/... -run TestGetVehicleRequirements -v`
Expected: FAIL — 404, route doesn't exist yet.

- [ ] **Step 3: Add the route**

In `api/internal/api/v1/vehicles.go`, modify the `GET /vehicles` handler
(lines 16-28) to accept `role`:

```go
	// GET /vehicles
	g.Get("", perm.Require("list.organization_vehicles"), func(c fiber.Ctx) error {
		cursor := c.Query("cursor")
		opts := repositories.VehicleListOpts{
			PlatePrefix: c.Query("plate"),
			Sort:        c.Query("sort", "asc"),
			Limit:       intQuery(c, "limit", 50),
			StartKey:    decodeCursor(cursor),
		}
		var res *repositories.QueryResult
		var err error
		if role := c.Query("role"); role != "" {
			res, err = svc.ListByRole(c.Context(), middleware.GetOrgPK(c), role, opts)
		} else {
			res, err = svc.List(c.Context(), middleware.GetOrgPK(c), opts)
		}
		if err != nil {
			return sendProblem(c, err)
		}
		return sendPage(c, res, cursor)
	})
```

Then add the new route, right after the existing `GET /vehicles/:sk` block (after line 59):

```go
	// GET /vehicles/:sk/requirements
	g.Get("/:sk/requirements", perm.Require("get.organization_vehicles"), func(c fiber.Ctx) error {
		docType := c.Query("doc_type")
		role := c.Query("role")
		validDocTypes := map[string]bool{services.DocTypeMdfe: true, services.DocTypeNfe: true, services.DocTypeCteOS: true}
		validRoles := map[string]bool{services.VehicleRoleTractor: true, services.VehicleRoleTrailer: true}
		if !validDocTypes[docType] {
			return sendProblem(c, problem.BadRequest("doc_type inválido: "+docType))
		}
		if !validRoles[role] {
			return sendProblem(c, problem.BadRequest("role inválido: "+role))
		}
		item, err := svc.Get(c.Context(), middleware.GetOrgPK(c), c.Params("sk"))
		if err != nil {
			return sendProblem(c, err)
		}
		return c.JSON(fiber.Map{"missing": services.Missing(item, docType, role)})
	})
```

Add `"gopkg.aoctech.app/dfe/api/internal/problem"` to this file's imports if not
already present (check the top of the file first).

Add `ListByRole` to `VehicleService` (`api/internal/services/vehicles.go`, right after the
existing `List` method):

```go
func (s *VehicleService) ListByRole(ctx context.Context, orgPK, role string, opts repositories.VehicleListOpts) (*repositories.QueryResult, error) {
	return s.repo.ListByRole(ctx, orgPK, role, opts)
}
```

- [ ] **Step 4: Run to confirm it passes**

Run: `cd api && go test ./tests/integration/... -run TestGetVehicleRequirements -v`
Expected: PASS.

Also run the full suite to catch regressions: `go test ./... -race`

- [ ] **Step 5: Commit**

```bash
git add api/internal/api/v1/vehicles.go api/internal/services/vehicles.go api/tests/integration/vehicles_test.go
git commit -m "feat(api): add GET /vehicles/:sk/requirements and role list filter"
```

---

### Task 7: MDF-e emission — gate on `Missing`, remove silent defaults, resolve trailers

**Files:**
- Modify: `api/internal/services/mdfes/emit.go`
- Modify: `api/internal/services/mdfes/builder.go`
- Test: `api/internal/services/mdfes/builder_test.go`, `api/internal/services/mdfes/emit_test.go`
  (check whether `emit_test.go` already exists before creating it)

**Interfaces:**
- Consumes: `services.Missing`, `services.DocTypeMdfe`, `services.VehicleRoleTractor/Trailer`.
- Produces: `MdfeEmitBody.Trailers []MdfeTrailer` (`{SK string}`, `validate:"omitempty,max=3,dive"`);
  `resolveTrailers(ctx, orgPK string, trailers []MdfeTrailer) ([]resolvedVehicle, error)`;
  `resolveVehicle` now returns `problem.BadRequest` instead of defaulting when the registered
  vehicle is missing a required field.

- [ ] **Step 1: Write the failing unit test**

Add to `api/internal/services/mdfes/builder_test.go` (near the existing `veic`-building tests —
match its existing test helper pattern, e.g. `baseParams`, `ide`, by reading the file first):

```go
func TestBuildRodo_IncludesVeicReboque(t *testing.T) {
	p := baseParams(nil)
	p.trailers = []resolvedVehicle{
		{Placa: "XYZ1A23", Tara: "5000", TpCar: "01", CapKG: "9000"},
	}
	rodo := p.buildRodo()
	reboques, ok := rodo["veicReboque"].([]map[string]any)
	if !ok || len(reboques) != 1 {
		t.Fatalf("veicReboque = %v, want 1-item list", rodo["veicReboque"])
	}
	if reboques[0]["placa"] != "XYZ1A23" || reboques[0]["capKG"] != "9000" {
		t.Errorf("veicReboque[0] = %v, want placa=XYZ1A23 capKG=9000", reboques[0])
	}
}
```

- [ ] **Step 2: Run to confirm it fails**

Run: `cd api && go test ./internal/services/mdfes/... -run TestBuildRodo_IncludesVeicReboque -v`
Expected: FAIL — compile error, `buildParams` has no `trailers` field.

- [ ] **Step 3a: Update `builder.go`**

In `api/internal/services/mdfes/builder.go`:

Remove the now-unused constants (lines 36-38):
```go
	// Default traction-vehicle codes when the registered vehicle omits them.
	defaultTpRod = "01" // Truck
	defaultTpCar = "00" // não aplicável
```

Add `trailers []resolvedVehicle` to the `buildParams` struct (after the existing `vehicle
resolvedVehicle` field, around line 52):

```go
	vehicle     resolvedVehicle
	trailers    []resolvedVehicle // veicReboque — up to 3
```

Update `buildRodo` (lines 174-203) to append `veicReboque`:

```go
// buildRodo builds the rodoviário modal node (infANTT + veicTracao + veicReboque).
func (p buildParams) buildRodo() map[string]any {
	veic := map[string]any{
		"placa": p.vehicle.Placa,
		"tara":  p.vehicle.Tara,
		"tpRod": p.vehicle.TpRod,
		"tpCar": p.vehicle.TpCar,
		"UF":    p.vehicle.UF,
	}
	if p.vehicle.RENAVAM != "" {
		veic["RENAVAM"] = p.vehicle.RENAVAM
	}
	if p.vehicle.CapKG != "" {
		veic["capKG"] = p.vehicle.CapKG
	}
	if prop := buildProp(p.owner); prop != nil {
		veic["prop"] = prop
	}

	condutores := make([]map[string]any, 0, len(p.drivers))
	for _, c := range p.drivers {
		condutores = append(condutores, map[string]any{"xNome": c.Name, "CPF": onlyDigits(c.CPF)})
	}
	veic["condutor"] = condutores

	rodo := map[string]any{"veicTracao": veic}
	if len(p.trailers) > 0 {
		reboques := make([]map[string]any, 0, len(p.trailers))
		for _, t := range p.trailers {
			reboque := map[string]any{"placa": t.Placa, "tara": t.Tara, "tpCar": t.TpCar}
			if t.RENAVAM != "" {
				reboque["RENAVAM"] = t.RENAVAM
			}
			if t.CapKG != "" {
				reboque["capKG"] = t.CapKG
			}
			if t.UF != "" {
				reboque["UF"] = t.UF
			}
			reboques = append(reboques, reboque)
		}
		rodo["veicReboque"] = reboques
	}
	if infANTT := p.buildInfANTT(); len(infANTT) > 0 {
		rodo["infANTT"] = infANTT
	}
	return rodo
}
```

- [ ] **Step 3b: Update `emit.go`**

In `api/internal/services/mdfes/emit.go`:

Add `MdfeTrailer` type near `MdfeVehicle` (after line 79):

```go
// MdfeTrailer is a reboque (trailer) — a registered vehicle with role=trailer.
type MdfeTrailer struct {
	SK string `json:"sk" validate:"required"`
}
```

Add `Trailers` to `MdfeEmitBody` (after the `Vehicle MdfeVehicle` field, line 35):

```go
	Vehicle  MdfeVehicle   `json:"vehicle"`
	Trailers []MdfeTrailer `json:"trailers" validate:"omitempty,max=3,dive"`
```

Replace `resolveVehicle` (lines 505-565) to gate on `services.Missing` instead of defaulting:

```go
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
```

Add the `"gopkg.aoctech.app/dfe/api/internal/services"` import if not already
present (check the top of `emit.go` — the file is `package mdfes`, so it needs the parent
`services` package for `Missing`/`DocTypeMdfe`/etc.; this is already how `builder.go` imports
`services` for its `procEmiOwn` etc., so this import almost certainly already exists — verify
before adding a duplicate).

In `Emit` (around line 154-180), after resolving `vehicle`, resolve and pass `trailers` — find
where `resolveVehicle` is currently called and the `buildParams{...}` is constructed further
down (search for `vehicle, err := s.resolveVehicle` and `vehicle: vehicle,` in this file), and:

1. Right after the existing `vehicle, err := s.resolveVehicle(ctx, orgPK, req.Vehicle)` call,
   add:
   ```go
	trailers, err := s.resolveTrailers(ctx, orgPK, req.Trailers)
	if err != nil {
		return nil, err
	}
   ```
2. In the `buildParams{...}` literal passed to `BuildMDFe`, add `trailers: trailers,` alongside
   the existing `vehicle: vehicle,` line.

- [ ] **Step 4: Run to confirm it passes**

Run: `cd api && go build ./... && go test ./internal/services/mdfes/... -v`
Expected: whole module builds (no more references to `defaultTpRod`/`defaultTpCar`); all
`mdfes` package tests PASS including the new `TestBuildRodo_IncludesVeicReboque`.

Also check for now-broken existing tests that relied on defaulting behavior (search
`grep -rn "defaultTpRod\|defaultTpCar" api/internal/services/mdfes/` — should return nothing).
If any existing test asserted the old silent-default behavior (e.g. an incomplete vehicle
producing `tpRod=01`), update it to instead assert `problem.BadRequest` is now returned.

- [ ] **Step 5: Commit**

```bash
git add api/internal/services/mdfes/emit.go api/internal/services/mdfes/builder.go api/internal/services/mdfes/builder_test.go
git commit -m "feat(api): MDF-e gates incomplete tractor/trailers, adds veicReboque output"
```

---

### Task 8: API integration tests — gating + successful trailer emission

**Files:**
- Modify: `api/tests/integration/vehicles_test.go` (from Task 6, or a new adjacent file if that
  one is getting long — follow the existing file's size/convention)
- Modify or create: `api/tests/integration/mdfes_test.go` (check if it already exists first)

**Interfaces:**
- Consumes: everything from Tasks 2-7.

- [ ] **Step 1: Write the failing integration tests**

Before writing, read the existing `api/tests/integration/mdfes_test.go` (if present) or the
closest existing MDF-e integration test file in full, to reuse its exact test-org/cert/document
fixture setup helpers — do not invent new ones. Then add:

```go
func TestEmitMdfe_IncompleteTractorReturnsBadRequest(t *testing.T) {
	// Reuse this suite's existing helper(s) to: create an org + cert, create
	// two NF-e documents to reference, and create a vehicle with ONLY
	// plate/plate_uf/role=tractor (no weight/wheelset/bodywork).
	vehicle := createTestVehicle(t, map[string]any{"plate": "ABC1D23", "plate_uf": "SP", "role": "tractor"})
	resp := doPost(t, "/v1.0/mdfes/emit", mdfeEmitPayload(t, vehicle["sk"], nil))
	assertStatus(t, resp, 400)
}

func TestEmitMdfe_CompleteTractorPlusTrailerSucceeds(t *testing.T) {
	tractor := createTestVehicle(t, map[string]any{
		"plate": "ABC1D23", "plate_uf": "SP", "role": "tractor",
		"weight": 8000, "wheelset": "01", "bodywork": "00",
	})
	trailer := createTestVehicle(t, map[string]any{
		"plate": "XYZ1A23", "plate_uf": "SP", "role": "trailer",
		"weight": 5000, "cap_kg": 9000, "bodywork": "01",
	})
	resp := doPost(t, "/v1.0/mdfes/emit", mdfeEmitPayload(t, tractor["sk"], []string{trailer["sk"].(string)}))
	assertStatus(t, resp, 200)
}
```

(`createTestVehicle`, `doPost`, `assertStatus`, `mdfeEmitPayload` are placeholders standing in
for this suite's real existing helpers — inspect and reuse them exactly, matching whatever
document/cert fixtures the existing MDF-e emit tests already set up.)

- [ ] **Step 2: Run to confirm both fail**

Run: `cd api && go test ./tests/integration/... -run TestEmitMdfe_IncompleteTractor -v`
Run: `cd api && go test ./tests/integration/... -run TestEmitMdfe_CompleteTractorPlusTrailer -v`
Expected: first FAILs if it currently returns 200 (pre-Task-7 behavior); second FAILs if
`trailers` isn't wired yet. (If Tasks 6-7 are already merged when this task runs, the first
should already pass — keep it as a permanent regression test either way.)

- [ ] **Step 3: (No production code change — this task only adds coverage.)**

If either test fails against the already-implemented Tasks 6-7 code, that's a real bug in this
plan's earlier tasks — stop and fix the root cause in `emit.go`/`builder.go`, don't adjust the
test to match broken behavior.

- [ ] **Step 4: Run to confirm both pass**

Run: `cd api && go test ./tests/integration/... -v`
Expected: full integration suite PASSES.

- [ ] **Step 5: Commit**

```bash
git add api/tests/integration/
git commit -m "test(api): cover MDF-e vehicle-completeness gating and trailer emission"
```

---

### Task 9: UI — types + api client + query keys

**Files:**
- Modify: `ui/src/lib/types/api.ts:392-442` (`VehicleOut`/`VehicleCreate`/`VehicleUpdate`),
  `:716-726` (`MdfeVehicleIn`), `:748-764` (`MdfeEmit`)
- Modify: `ui/src/lib/api/client.ts:229-248`
- Modify: `ui/src/lib/api/query-keys.ts:17-20`

**Interfaces:**
- Produces: `VehicleOut{role: 'tractor'|'trailer'; wheelset?, bodywork?, renavam?: string;
  weight?, cap_kg?, cap_m3?: number; cint?: string; owner?: OwnerOut}` (drops `trailers`);
  `VehicleRequirements = {missing: string[]}`; `apiClient.getVehicles(params?: {role?:
  'tractor'|'trailer'; limit?; cursor?})`; `apiClient.getVehicleRequirements(sk: string, docType:
  string, role: string): Promise<VehicleRequirements>`; `MdfeEmit.trailers?: {sk: string}[]`.

- [ ] **Step 1: Update `api.ts` types**

Replace `ui/src/lib/types/api.ts:392-442` (from `TrailerOut`'s field block, i.e. wherever the
existing `wheelset/bodywork/renavam/weight/owner` fields start through `VehicleUpdate`'s closing
brace — re-read the current file at this point before editing, since Task 3 doesn't touch this
file and line numbers should still match) with:

```typescript
export interface VehicleOut {
  pk: string
  sk: string
  plate: string
  plate_uf: string
  role: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: OwnerOut
  created_at: string
  updated_at: string
}

export interface VehicleCreate {
  plate: string
  plate_uf: string
  role: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: {
    cpf_cnpj: string
    rntrc: string
    name: string
    type: string
  }
}

export interface VehicleUpdate {
  plate?: string
  plate_uf?: string
  role?: 'tractor' | 'trailer'
  wheelset?: string
  bodywork?: string
  renavam?: string
  weight?: number
  cap_kg?: number
  cap_m3?: number
  cint?: string
  owner?: VehicleCreate['owner']
}

export interface VehicleRequirements {
  missing: string[]
}
```

Remove the `TrailerOut` interface entirely (it's superseded — trailers are now plain
`VehicleOut` rows with `role: 'trailer'`); search the file for any other reference to
`TrailerOut` before deleting and update those call sites too (expected: only `VehicleOut` and
`VehicleForm.tsx` referenced it — the latter is handled in Task 11).

In `MdfeVehicleIn` (around line 716-726), no field changes are needed (it already models the
per-emission override shape correctly) — leave as-is.

In `MdfeEmit` (around line 748-764), add trailers:

```typescript
export interface MdfeEmit {
  modal: 'rodoviario' | 'aereo' | 'aquaviario' | 'ferroviario'
  documents: MdfeDocRef[]
  uf_start?: string
  uf_end?: string
  route?: string[]
  loadings?: MdfeMunIn[]
  unloadings?: MdfeMunIn[]
  vehicle: MdfeVehicleIn
  trailers?: { sk: string }[]
  drivers: MdfeDriverIn[]
  predominant?: MdfeProdPredIn | null
  bulk_cargo?: MdfeBulkCargoIn | null
  trip_start?: string | null
  rntrc?: string | null
  ciot?: string | null
  additional_info?: string | null
}
```

- [ ] **Step 2: Update `client.ts`**

Replace `ui/src/lib/api/client.ts:229-248` (the `// Vehicles` block) with:

```typescript
  // Vehicles
  async getVehicles(params?: { role?: 'tractor' | 'trailer'; limit?: number; cursor?: string }): Promise<PaginatedResponse<VehicleOut>> {
    return this.get('/v1.0/vehicles', {params})
  }

  async getVehicle(id: string): Promise<VehicleOut> {
    return this.get(`/v1.0/vehicles/${id}`)
  }

  async getVehicleRequirements(id: string, docType: string, role: string): Promise<VehicleRequirements> {
    return this.get(`/v1.0/vehicles/${id}/requirements`, {params: {doc_type: docType, role}})
  }

  async createVehicle(data: VehicleCreate): Promise<VehicleOut> {
    return this.post('/v1.0/vehicles', data)
  }

  async updateVehicle(id: string, data: VehicleUpdate): Promise<VehicleOut> {
    return this.put(`/v1.0/vehicles/${id}`, data)
  }

  async deleteVehicle(id: string): Promise<void> {
    return this.del(`/v1.0/vehicles/${id}`)
  }
```

Add `VehicleRequirements` to this file's existing `import type {...} from '@/lib/types/api'`
block (find it near the top, alongside `VehicleOut`/`VehicleCreate`).

- [ ] **Step 3: Update `query-keys.ts`**

In `ui/src/lib/api/query-keys.ts:17-20`, replace:

```typescript
  vehicles: {
    list: (orgPk: string | undefined) => ['vehicles', orgPk] as const,
    detail: (id: string) => ['vehicle', id] as const,
  },
```

with:

```typescript
  vehicles: {
    list: (orgPk: string | undefined, role?: string) => ['vehicles', orgPk, role] as const,
    detail: (id: string) => ['vehicle', id] as const,
    requirements: (id: string, docType: string, role: string) => ['vehicle-requirements', id, docType, role] as const,
  },
```

Any existing call site using `queryKeys.vehicles.list(orgPk)` still works unchanged (the new
`role` param is optional) — but grep for `queryKeys.vehicles.list(` afterward to confirm no call
site breaks from the key shape changing when a role IS passed (`MdfeEmitForm.tsx` gets updated
in Task 12 to pass roles explicitly for its two pickers).

- [ ] **Step 4: Verify types compile**

Run: `cd ui && npx tsc --noEmit`
Expected: no new type errors. (Existing errors unrelated to this change, if any, are out of
scope — but there should be none introduced by this task; `VehicleForm.tsx` will show errors
here until Task 11 lands, so scope this check to confirm no *new categories* of error beyond
`VehicleForm.tsx`/`MdfeEmitForm.tsx`, which are fixed in later tasks.)

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/types/api.ts ui/src/lib/api/client.ts ui/src/lib/api/query-keys.ts
git commit -m "feat(ui): vehicle types/client for role, cap_kg/cap_m3/cint, requirements endpoint"
```

---

### Task 10: UI — `vehicles.ts` zod schema (role, optional fields, drop trailers)

**Files:**
- Modify: `ui/src/lib/schemas/vehicles.ts`
- Test: `ui/src/__tests__/lib/schemas/vehicles.test.ts` (new)

**Interfaces:**
- Produces: `vehicleSchema` requiring only `plate`, `plate_uf`, `role`; `ROLE_OPTIONS` constant;
  `VehicleFormData` type without `trailers`.

- [ ] **Step 1: Write the failing schema test**

Create `ui/src/__tests__/lib/schemas/vehicles.test.ts`:

```typescript
import {describe, it, expect} from 'vitest'
import {vehicleSchema} from '@/lib/schemas/vehicles'

describe('vehicleSchema', () => {
  it('aceita apenas placa, UF e role (mínimo)', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'tractor'})
    expect(result.success).toBe(true)
  })

  it('rejeita sem role', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP'})
    expect(result.success).toBe(false)
  })

  it('rejeita role inválido', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'carro'})
    expect(result.success).toBe(false)
  })

  it('aceita campos avançados quando presentes', () => {
    const result = vehicleSchema.safeParse({
      plate: 'ABC1D23', plate_uf: 'SP', role: 'tractor',
      wheelset: '01', bodywork: '00', renavam: '123456789', weight: '8000',
    })
    expect(result.success).toBe(true)
  })

  it('não exige mais o bloco owner', () => {
    const result = vehicleSchema.safeParse({plate: 'ABC1D23', plate_uf: 'SP', role: 'trailer'})
    expect(result.success).toBe(true)
  })
})
```

- [ ] **Step 2: Run to confirm it fails**

Run: `cd ui && npx vitest run src/__tests__/lib/schemas/vehicles.test.ts`
Expected: FAIL — current schema requires `wheelset`/`bodywork`/`renavam`/`weight`/`owner`/
`trailers`, and has no `role`.

- [ ] **Step 3: Rewrite `vehicles.ts`**

Replace the entire content of `ui/src/lib/schemas/vehicles.ts` with:

```typescript
import {z} from 'zod'
import {UF_LIST} from '@/lib/schemas/entity'

export const WHEELSET_OPTIONS = [
  { value: '01', label: 'Truck' },
  { value: '02', label: 'Toco' },
  { value: '03', label: 'Cavalo Mecânico' },
  { value: '04', label: 'VAN' },
  { value: '05', label: 'Utilitário' },
  { value: '99', label: 'Outros' },
]

export const BODYWORK_OPTIONS = [
  { value: '00', label: 'Não aplicável' },
  { value: '01', label: 'Aberto' },
  { value: '02', label: 'Fechado/Baú' },
  { value: '03', label: 'Graneleiro' },
  { value: '04', label: 'Porta Container' },
  { value: '05', label: 'Sider' },
]

export const OWNER_TYPE_OPTIONS = [
  { value: 'TAC', label: 'TAC – Transportador Autônomo' },
  { value: 'ETC', label: 'ETC – Empresa de Transporte' },
  { value: 'CTC', label: 'CTC – Cooperativa de Transporte' },
]

export const ROLE_OPTIONS = [
  { value: 'tractor', label: 'Tração (cavalo/caminhão)' },
  { value: 'trailer', label: 'Reboque' },
]

export {UF_OPTIONS} from '@/lib/schemas/entity'

const ownerSchema = z.object({
  cpf_cnpj: z.string().min(11, 'CPF/CNPJ obrigatório').max(14),
  rntrc: z.string().regex(/^\d{8,12}$/, 'RNTRC deve ter 8–12 dígitos'),
  name: z.string().min(2, 'Mínimo 2 caracteres').max(255),
  type: z.enum(['TAC', 'ETC', 'CTC'], { error: 'Tipo inválido' }),
})

export const vehicleSchema = z.object({
  plate: z
    .string()
    .regex(/^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/, 'Placa Mercosul inválida (ex: ABC1D23)'),
  plate_uf: z.enum(UF_LIST, { error: 'UF inválida' }),
  role: z.enum(['tractor', 'trailer'], { error: 'Papel do veículo obrigatório' }),
  wheelset: z.string().optional(),
  bodywork: z.string().optional(),
  renavam: z.string().regex(/^\d{9,11}$/, 'RENAVAM deve ter 9–11 dígitos').optional().or(z.literal('')),
  weight: z.string().regex(/^\d+$/, 'Tara deve ser um número inteiro positivo').optional().or(z.literal('')),
  cap_kg: z.string().regex(/^\d+$/, 'Capacidade deve ser um número inteiro positivo').optional().or(z.literal('')),
  cap_m3: z.string().regex(/^\d+$/, 'Capacidade deve ser um número inteiro positivo').optional().or(z.literal('')),
  cint: z.string().max(10).optional(),
  owner: ownerSchema.optional(),
})

export type VehicleFormData = z.infer<typeof vehicleSchema>
export type OwnerFormData = z.infer<typeof ownerSchema>
```

- [ ] **Step 4: Run to confirm it passes**

Run: `cd ui && npx vitest run src/__tests__/lib/schemas/vehicles.test.ts`
Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/src/lib/schemas/vehicles.ts ui/src/__tests__/lib/schemas/vehicles.test.ts
git commit -m "feat(ui): vehicle schema requires only plate/plate_uf/role"
```

---

### Task 11: UI — `VehicleForm.tsx` role selector + advanced section, drop reboque UI

**Files:**
- Modify: `ui/src/components/vehicles/VehicleForm.tsx`

**Interfaces:**
- Consumes: `vehicleSchema`, `ROLE_OPTIONS` (Task 10); `VehicleOut`/`VehicleCreate` (Task 9).
- Produces: `VehicleForm` renders a role selector + always-visible identification fields
  (plate/UF) + a collapsible "Configurações avançadas" section (wheelset only for tractor role;
  bodywork/renavam/weight/cap_kg/cap_m3/owner for both roles). No more inline reboque add/list
  UI — trailers are created via this same form with `role: 'trailer'`.

- [ ] **Step 1: Manual verification plan (no automated test — component behavior covered by
  the zod unit tests in Task 10 and this repo's `npx eslint` gate; component tests aren't
  currently used for this form)**

Before editing, note the exact current default/`fromOut` behavior so the rewrite doesn't
regress editing an existing vehicle (re-read `VehicleForm.tsx:44-73` if it changed since this
plan was written).

- [ ] **Step 2: Rewrite `VehicleForm.tsx`**

Replace the entire content of `ui/src/components/vehicles/VehicleForm.tsx` with:

```tsx
'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {
  BODYWORK_OPTIONS,
  OWNER_TYPE_OPTIONS,
  ROLE_OPTIONS,
  UF_OPTIONS,
  type VehicleFormData,
  vehicleSchema,
  WHEELSET_OPTIONS,
} from '@/lib/schemas/vehicles'
import type {VehicleCreate, VehicleOut} from '@/lib/types/api'
import {maskCpfCnpj} from '@/lib/utils/masks'

interface VehicleFormProps {
  initialData?: VehicleOut
  onSubmit: (data: VehicleCreate) => Promise<void>
  loading?: boolean
  /** Missing fields returned by GET /vehicles/:sk/requirements — when set, the
   * advanced section auto-expands so the user can fix them immediately. */
  highlightFields?: string[]
}

function fromOut(v: VehicleOut): VehicleFormData {
  return {
    plate: v.plate,
    plate_uf: v.plate_uf as VehicleFormData['plate_uf'],
    role: v.role,
    wheelset: v.wheelset ?? '',
    bodywork: v.bodywork ?? '',
    renavam: v.renavam ?? '',
    weight: v.weight ? String(v.weight) : '',
    cap_kg: v.cap_kg ? String(v.cap_kg) : '',
    cap_m3: v.cap_m3 ? String(v.cap_m3) : '',
    cint: v.cint ?? '',
    owner: v.owner
      ? {cpf_cnpj: v.owner.cpf_cnpj, rntrc: v.owner.rntrc, name: v.owner.name, type: v.owner.type as 'TAC' | 'ETC' | 'CTC'}
      : undefined,
  }
}

export function VehicleForm({ initialData, onSubmit, loading = false, highlightFields = [] }: VehicleFormProps) {
  const [advancedOpen, setAdvancedOpen] = useState(highlightFields.length > 0)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<VehicleFormData>({
    resolver: zodResolver(vehicleSchema),
    defaultValues: initialData
      ? fromOut(initialData)
      : {
          plate: '',
          plate_uf: 'SP',
          role: 'tractor',
          wheelset: '',
          bodywork: '',
          renavam: '',
          weight: '',
          cap_kg: '',
          cap_m3: '',
          cint: '',
        },
  })

  const role = useWatch({ control: form.control, name: 'role' })
  const wantsOwner = useWatch({ control: form.control, name: 'owner' })
  const ownerCpf = useWatch({ control: form.control, name: 'owner.cpf_cnpj' })

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      const payload: VehicleCreate = {
        plate: data.plate,
        plate_uf: data.plate_uf,
        role: data.role,
        wheelset: data.wheelset || undefined,
        bodywork: data.bodywork || undefined,
        renavam: data.renavam || undefined,
        weight: data.weight ? Number(data.weight) : undefined,
        cap_kg: data.cap_kg ? Number(data.cap_kg) : undefined,
        cap_m3: data.cap_m3 ? Number(data.cap_m3) : undefined,
        cint: data.cint || undefined,
        owner: data.owner ? {...data.owner, cpf_cnpj: data.owner.cpf_cnpj.replace(/\D/g, '')} : undefined,
      }
      await onSubmit(payload)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  const isMissing = (field: string) => highlightFields.includes(field)

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-6">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}
        {highlightFields.length > 0 && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            Este veículo está incompleto para este documento. Preencha os campos destacados abaixo.
          </div>
        )}

        {/* Identificação */}
        <section className="space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Identificação</p>
          <FormField
            control={form.control}
            name="role"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Tipo de veículo *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={ROLE_OPTIONS} />
                <FormMessage />
              </FormItem>
            )}
          />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField
              control={form.control}
              name="plate"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Placa (Mercosul) *</FormLabel>
                  <Input {...field} id={field.name} placeholder="ABC1D23" maxLength={7}
                         onChange={(e) => field.onChange(e.target.value.toUpperCase())} />
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="plate_uf"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>UF da placa *</FormLabel>
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={UF_OPTIONS} />
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </section>

        {/* Configurações avançadas */}
        <section className="space-y-3">
          <Button type="button" variant="ghost" size="xs" onClick={() => setAdvancedOpen(!advancedOpen)}
                  className="text-brand-600 hover:text-brand-700">
            {advancedOpen ? '− Ocultar configurações avançadas' : '+ Configurações avançadas'}
          </Button>

          {advancedOpen && (
            <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {role === 'tractor' && (
                  <FormField
                    control={form.control}
                    name="wheelset"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel className={isMissing('wheelset') ? 'text-amber-700' : ''}>Tipo de eixo</FormLabel>
                        <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange} options={WHEELSET_OPTIONS} />
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name="bodywork"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('bodywork') ? 'text-amber-700' : ''}>Carroceria</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange} options={BODYWORK_OPTIONS} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField
                  control={form.control}
                  name="renavam"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>RENAVAM</FormLabel>
                      <NumericInput {...field} id={field.name} placeholder="12345678901" maxLength={11} onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="weight"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('weight') ? 'text-amber-700' : ''}>Tara</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="KG" placeholder="12000" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField
                  control={form.control}
                  name="cap_kg"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('cap_kg') ? 'text-amber-700' : ''}>Capacidade (KG)</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="KG" placeholder="9000" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="cap_m3"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Capacidade (M³)</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="M³" placeholder="40" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="cint"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Código interno</FormLabel>
                    <Input {...field} id={field.name} placeholder="Opcional" maxLength={10} />
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="space-y-3 border-t border-gray-200 pt-3">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Proprietário (se não for da própria empresa)</p>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => form.setValue('owner', wantsOwner ? undefined : {cpf_cnpj: '', rntrc: '', name: '', type: 'TAC'})}>
                    {wantsOwner ? 'Remover' : '+ Adicionar'}
                  </Button>
                </div>
                {wantsOwner && (
                  <div className="space-y-3">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <FormField
                        control={form.control}
                        name="owner.cpf_cnpj"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>CPF/CNPJ do proprietário</FormLabel>
                            <Input ref={field.ref} name={field.name} onBlur={field.onBlur} id={field.name}
                                   value={ownerCpf ?? ''} placeholder="000.000.000-00" maxLength={18}
                                   onChange={(e) => form.setValue('owner.cpf_cnpj', maskCpfCnpj(e.target.value))} />
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="owner.rntrc"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>RNTRC</FormLabel>
                            <NumericInput {...field} id={field.name} placeholder="12345678" maxLength={12} />
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                    <FormField
                      control={form.control}
                      name="owner.name"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Nome</FormLabel>
                          <Input {...field} id={field.name} placeholder="Proprietário LTDA" maxLength={255} />
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="owner.type"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Tipo</FormLabel>
                          <OptionsSelect id={field.name} value={field.value ?? 'TAC'} onValueChange={field.onChange} options={OWNER_TYPE_OPTIONS} />
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                )}
              </div>
            </div>
          )}
        </section>

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? 'Salvando...' : initialData ? 'Salvar alterações' : 'Cadastrar veículo'}
        </Button>
      </form>
    </Form>
  )
}
```

- [ ] **Step 3: Run ESLint**

Run: `cd ui && npx eslint src/components/vehicles/VehicleForm.tsx`
Expected: zero errors/warnings. Fix any reported issue (unused imports, etc.) before moving on.

- [ ] **Step 4: Run the zod schema tests + full typecheck**

Run: `cd ui && npx vitest run src/__tests__/lib/schemas/vehicles.test.ts && npx tsc --noEmit`
Expected: schema tests still PASS; no new type errors from this file (any remaining errors
should only be in `MdfeEmitForm.tsx`, fixed in Task 12).

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/vehicles/VehicleForm.tsx
git commit -m "feat(ui): VehicleForm gets role selector, advanced section, drops inline reboque UI"
```

---

### Task 12: UI — `MdfeEmitForm.tsx` trailer picker + emission-time gating modal

**Files:**
- Modify: `ui/src/components/mdfe/MdfeEmitForm.tsx`

**Interfaces:**
- Consumes: `apiClient.getVehicles({role})`, `apiClient.getVehicleRequirements` (Task 9),
  `VehicleForm` with `highlightFields` (Task 11).
- Produces: tractor picker (existing, now gated) + new trailer picker (0-3, `role=trailer`);
  `MdfeEmit.trailers` populated on submit; incomplete-vehicle selection opens an edit modal
  instead of silently allowing submission.

- [ ] **Step 1: Manual verification plan**

No automated test for this component (matches this form's existing lack of component tests —
`ui/CLAUDE.md`'s testing table lists "New component | Component test" but this is a modification
to an existing, already-untested component; adding a full RTL harness for the whole multi-step
wizard is out of scope for this plan). Verify manually per Step 4 below instead.

- [ ] **Step 2: Update `VehicleRegisterModal` to also handle "incomplete" (not just "create")**

In `ui/src/components/mdfe/MdfeEmitForm.tsx`, replace the `VehicleRegisterModal` function
(lines 202-234) with a version that can both create and edit-in-place:

```tsx
// ─── vehicle modal (reuses VehicleForm) — handles both "no vehicle yet" and
// "vehicle selected but incomplete for this doc-type/role" ──────────────────

function VehicleRegisterModal({open, onClose, onSaved, editing, missing}: {
  open: boolean
  onClose: () => void
  onSaved: (v: VehicleOut) => void
  editing?: VehicleOut
  missing?: string[]
}) {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const createMutation = useMutation({
    mutationFn: (d: VehicleCreate) => apiClient.createVehicle(d),
    onSuccess: (v) => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
      onSaved(v)
    },
  })
  const updateMutation = useMutation({
    mutationFn: (d: VehicleCreate) => apiClient.updateVehicle(editing!.sk, d),
    onSuccess: (v) => {
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
      onSaved(v)
    },
  })
  const mutation = editing ? updateMutation : createMutation
  if (!open || typeof document === 'undefined') return null
  return createPortal(
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
      <div className="bg-white rounded-xl shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="sticky top-0 bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between rounded-t-xl">
          <h2 className="text-lg font-semibold text-gray-900">
            {editing ? 'Completar dados do veículo' : 'Cadastrar veículo'}
          </h2>
          <Button variant="ghost" size="icon-sm" onClick={onClose} aria-label="Fechar"
                  className="text-gray-400 hover:text-gray-600">×</Button>
        </div>
        <div className="p-6">
          <VehicleForm initialData={editing} highlightFields={missing}
                       onSubmit={async (d) => { await mutation.mutateAsync(d) }}
                       loading={mutation.isPending}/>
        </div>
      </div>
    </div>,
    document.body,
  )
}
```

- [ ] **Step 3: Wire tractor + trailer pickers with gating**

In the main `MdfeEmitForm` component, replace the vehicle-related state (around line 343-345)
and the vehicle query (line 361-371) with:

```tsx
  // Vehicles (registered only) + register/edit modal.
  const [vehicleSk, setVehicleSk] = useState<string | null>(null)
  const [trailerSks, setTrailerSks] = useState<string[]>([])
  const [gateModal, setGateModal] = useState<{vehicle: VehicleOut; missing: string[]} | null>(null)
  const [registerOpen, setRegisterOpen] = useState(false)
```

```tsx
  const {data: tractorsData} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk, 'tractor'),
    queryFn: () => apiClient.getVehicles({role: 'tractor', limit: 50}),
    enabled: !!selectedOrg,
  })
  const {data: trailersData} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk, 'trailer'),
    queryFn: () => apiClient.getVehicles({role: 'trailer', limit: 50}),
    enabled: !!selectedOrg,
  })

  const tractorOptions = (tractorsData?.items ?? []).map((v: VehicleOut) => ({
    value: v.sk, label: `${v.plate} · ${v.plate_uf}`, display: v.plate,
  }))
  const trailerOptions = (trailersData?.items ?? []).map((v: VehicleOut) => ({
    value: v.sk, label: `${v.plate} · ${v.plate_uf}`, display: v.plate,
  }))

  const checkVehicle = async (v: VehicleOut, role: 'tractor' | 'trailer') => {
    const {missing} = await apiClient.getVehicleRequirements(v.sk, 'mdfe', role)
    if (missing.length > 0) setGateModal({vehicle: v, missing})
  }

  const onSelectTractor = (sk: string | null) => {
    setVehicleSk(sk)
    const v = tractorsData?.items.find((x) => x.sk === sk)
    if (v) void checkVehicle(v, 'tractor')
  }

  const onSelectTrailer = (sk: string) => {
    setTrailerSks((prev) => prev.includes(sk) ? prev : [...prev, sk])
    const v = trailersData?.items.find((x) => x.sk === sk)
    if (v) void checkVehicle(v, 'trailer')
  }

  const removeTrailer = (sk: string) => setTrailerSks((prev) => prev.filter((s) => s !== sk))
```

Update `canEmit` (line 454) to keep requiring `vehicleSk` (unchanged condition — trailers stay
optional, matching the XSD's `minOccurs="0"` on `veicReboque`):

```tsx
  const canEmit = docs.length > 0 && !!vehicleSk && drivers.length > 0 && allWeightsKnown
    && (!needsBulk || (cepCarrega.replace(/\D/g, '').length === 8 && cepDescarrega.replace(/\D/g, '').length === 8))
```

Update the submit payload (around line 463-480) to include `trailers`:

```tsx
    const payload: MdfeEmit = {
      modal: 'rodoviario',
      documents: docs.map((d) => {
        const override = weightOverrides[d.sk]?.trim()
        return {type: 'nfe', access_key: d.sk, ...(override ? {weight: override} : {})}
      }),
      uf_start: ufIni || undefined,
      uf_end: ufFim || undefined,
      route: route.length ? route : undefined,
      loadings: loadings.length ? loadings : undefined,
      unloadings: unloadings.length ? unloadings : undefined,
      drivers,
      vehicle: {sk: vehicleSk},
      trailers: trailerSks.length ? trailerSks.map((sk) => ({sk})) : undefined,
      trip_start: tripStart ? `${tripStart}:00-03:00` : undefined,
      bulk_cargo: needsBulk
        ? {cep_loading: cepCarrega.replace(/\D/g, ''), cep_unloading: cepDescarrega.replace(/\D/g, '')}
        : undefined,
    }
```

Update the "Veículo" step render (lines 643-663) to add the trailer picker and wire the gated
`onSelectTractor`:

```tsx
      {step === 'veiculo' && (
        <div className="space-y-4">
          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Veículo (tração)</p>
              <Button type="button" size="xs" variant="outline"
                      onClick={() => setRegisterOpen(true)}>+ Cadastrar veículo</Button>
            </div>
            {tractorOptions.length > 0 ? (
              <Combobox value={vehicleSk} onValueChange={onSelectTractor} options={tractorOptions}
                        placeholder="Selecione um veículo" searchPlaceholder="Buscar placa..."/>
            ) : (
              <p className="text-sm text-gray-500">
                Nenhum veículo cadastrado. Cadastre um para continuar.
              </p>
            )}
          </div>

          <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Reboques (opcional, até 3)</p>
            {trailerSks.length > 0 && (
              <div className="space-y-1.5">
                {trailerSks.map((sk) => {
                  const t = trailerOptions.find((o) => o.value === sk)
                  return (
                    <div key={sk} className="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-2 text-sm">
                      <span className="text-gray-700">{t?.label ?? sk}</span>
                      <Button type="button" variant="ghost" size="xs" onClick={() => removeTrailer(sk)}
                              className="text-red-500 hover:text-red-700">remover</Button>
                    </div>
                  )
                })}
              </div>
            )}
            {trailerSks.length < 3 && trailerOptions.length > 0 && (
              <Combobox value={null} onValueChange={(sk) => sk && onSelectTrailer(sk)}
                        options={trailerOptions.filter((o) => !trailerSks.includes(o.value))}
                        placeholder="Adicionar reboque" searchPlaceholder="Buscar placa..."/>
            )}
          </div>
```

(leave the rest of the `veiculo` step — condutores block, `submitError` block — unchanged).

Finally, replace the `VehicleRegisterModal` mount at the bottom (around line 717) to also render
the gating modal:

```tsx
      <VehicleRegisterModal open={registerOpen} onClose={() => setRegisterOpen(false)}
                            onSaved={(v) => { setVehicleSk(v.sk); setRegisterOpen(false) }}/>
      <VehicleRegisterModal open={!!gateModal} onClose={() => setGateModal(null)}
                            editing={gateModal?.vehicle} missing={gateModal?.missing}
                            onSaved={() => setGateModal(null)}/>
```

- [ ] **Step 4: Manual browser verification**

Run: `cd ui && npm run dev`, then in a browser:
1. Go to the MDF-e emit flow, reach the "Veículo" step.
2. Select a tractor with missing `weight`/`wheelset`/`bodywork` (create one with only
   plate/UF via "+ Cadastrar veículo" to set this up) → confirm the edit modal opens
   automatically with "Configurações avançadas" expanded and the missing fields highlighted
   amber.
3. Fill the missing fields, save → confirm the modal closes and the tractor stays selected.
4. Add a trailer with missing `cap_kg` → confirm the same gating fires for the trailer role.
5. Confirm emission with a fully-complete tractor + trailer succeeds and no longer silently
   defaults `tpRod`/`tpCar`.
Test at 375px viewport per `ui/CLAUDE.md` mobile-first rules (modal `max-w-2xl w-full mx-4`
already responsive from the existing component — just confirm no overflow).

- [ ] **Step 5: Run ESLint, typecheck, commit**

Run: `cd ui && npx eslint src/components/mdfe/MdfeEmitForm.tsx && npx tsc --noEmit`
Expected: zero errors/warnings; no type errors.

```bash
git add ui/src/components/mdfe/MdfeEmitForm.tsx
git commit -m "feat(ui): MDF-e emit form gains trailer picker and vehicle-completeness gating"
```

---

### Task 13: Docs — `DynamoDB-Tables.md`, `DOCS.md`, `CONDUCT.md`

**Files:**
- Modify: `DynamoDB-Tables.md:129-144`
- Modify: `DOCS.md` (vehicle CRUD section + MDF-e emit section — locate via
  `grep -n "organization_vehicles\|/vehicles\|MdfeEmitBody" DOCS.md` first)
- Modify: `CONDUCT.md` (append a new dated entry, matching this file's existing format —
  check its most recent entries for the exact heading style before adding)

**Interfaces:** none (documentation only).

- [ ] **Step 1: Update `DynamoDB-Tables.md`**

Replace lines 129-144 with:

```markdown
## 5. `organization_vehicles`

Fleet registry for CT-e and MDF-e operations. Only `plate`/`plate_uf`/`role` are required at
cadastro — everything else is optional and gated per doc-type/role at emission time (see
`api/internal/services/vehicle_requirements.go`).

| Attribute    | Type | Notes                                                    |
|--------------|------|-----------------------------------------------------------|
| `pk`         | S    | `{org_pk}` — partition key                                |
| `sk`         | S    | `VEHICLE_{id}` — sort key                                  |
| `role`       | S    | `tractor` \| `trailer`. GSI: `role-index`                  |
| `plate`      | S    | Vehicle plate. GSI: `plate-index`                          |
| `plate_uf`   | S    | UF of plate registration                                   |
| `wheelset`   | S    | Tipo de rodado (MDF-e `tpRod`, tractor only). Optional      |
| `bodywork`   | S    | Tipo de carroceria (MDF-e `tpCar`). Optional                |
| `renavam`    | S    | RENAVAM (optional)                                          |
| `weight`     | N    | Tare weight in kg (MDF-e `tara`). Optional                  |
| `cap_kg`     | N    | Capacity in kg (MDF-e `capKG`, required for trailers). Optional |
| `cap_m3`     | N    | Capacity in m³ (optional)                                   |
| `cint`       | S    | Internal code (optional)                                    |
| `owner`      | M    | `{cpf_cnpj, rntrc, name, type}` — optional fleet metadata, NOT used for MDF-e `prop` (that's per-emission, see `MdfeEmitBody.owner`) |
| `created_at` | S    | ISO-8601 UTC                                                |
| `updated_at` | S    | ISO-8601 UTC                                                |

**GSIs:** `plate-index` (PK: `pk`, SK: `plate`), `role-index` (PK: `pk`, SK: `role`).

---
```

- [ ] **Step 2: Update `DOCS.md`**

Run `grep -n "organization_vehicles\|GET /vehicles\|POST /vehicles" DOCS.md` to find the current
vehicle CRUD section, then update its field list to match the new optional-fields schema above
(drop any mention of `trailers[]` as a nested array; note the new
`GET /vehicles/:sk/requirements?doc_type=&role=` endpoint and its `{"missing": [...]}` response
shape; note `GET /vehicles?role=` filter). Run
`grep -n "MdfeEmitBody\|veicTracao\|veicReboque" DOCS.md` to find the MDF-e emit section and add
the new `trailers: [{sk}]` request field and note `veicReboque` is now emitted in the XML.

- [ ] **Step 3: Update `CONDUCT.md`**

Read the last 2-3 entries in `CONDUCT.md` to match its exact heading/dating convention, then
append a new entry summarizing:
- `services.Missing` (in `api/internal/services/vehicle_requirements.go`) is the single source
  of truth for vehicle field completeness per doc-type/role — never duplicate this matrix in
  `ui`; call `GET /vehicles/:sk/requirements` instead.
- MDF-e no longer silently defaults `tpRod`/`tpCar` when a registered vehicle is incomplete —
  it now returns `400 Bad Request` naming the missing fields.
- Vehicle `owner` (cpf_cnpj/rntrc/name/type) is optional fleet metadata only — it is NOT the
  source of MDF-e's `prop`/third-party-owner group, which remains a per-emission input
  (`MdfeEmitBody.owner` / `MdfeOwner`).

- [ ] **Step 4: Proofread**

Re-read all three edited doc files end-to-end for consistency with the actual code shipped in
Tasks 1-12 (field names, endpoint paths, constant names) — fix any drift found.

- [ ] **Step 5: Commit**

```bash
git add DynamoDB-Tables.md DOCS.md CONDUCT.md
git commit -m "docs: document vehicle cadastro schema, requirements endpoint, MDF-e trailer support"
```

---

## Final Verification

After all 13 tasks:

- [ ] `cd api && go build ./... && go test ./... -race` — full backend suite passes.
- [ ] `cd ui && npx eslint src --ext .ts,.tsx && npx tsc --noEmit && npm test` — full frontend
      gate passes.
- [ ] `cd cdk && npx cdk synth CtechDfe-Prod-DynamoDB > /dev/null` — infra still synths.
- [ ] Manually re-run the Task 12 Step 4 browser walkthrough once more end-to-end.
- [ ] Confirm `grep -rn "defaultTpRod\|defaultTpCar" api/` returns nothing.
- [ ] Confirm `grep -rn "TrailerOut\|VehicleTrailerBody\|trailerSchema" api/ ui/` returns
      nothing (all superseded by `role`-tagged first-class vehicles).
