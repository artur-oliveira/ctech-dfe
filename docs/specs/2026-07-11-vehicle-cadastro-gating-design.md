# Design — Vehicle cadastro: minimal fields + emission-time gating

**Date:** 2026-07-11 **Scope:** `api/`, `ui/` (schema, service, repo, routes, form, emit forms). `py-dfe`/`worker`
touched only for field-set awareness in XML building (`xsd_order.py`), no model changes.

## Problem

Vehicle cadastro (`organization_vehicles`) today forces every field (plate, UF, RENAVAM, tara, owner block, etc.) as
required in both the Go service/zod schema and `VehicleForm.tsx`, even though NF-e, MDF-e and CT-e OS each require a
different, much smaller subset per their XSDs. This makes registering a vehicle needlessly heavy and doesn't match
reality — some fields (tara, capacities, tipo de rodado/carroceria, owner/RNTRC) only matter for specific doc types and
specific roles (tractor vs. trailer).

Additionally, MDF-e emission (`mdfes/emit.go` `resolveVehicle`) already silently defaults missing `TpRod`→`01` and
`TpCar`→`00` instead of surfacing the gap — masking incomplete registrations instead of prompting the user to fix them.

Trailers are currently nested as `trailers[]` inside a parent vehicle record, which prevents reusing one trailer across
multiple tractors (MDF-e allows up to 3 reboques), even though a trailer plate is a vehicle in its own right.
Separately, MDF-e emission today has **no
`veicReboque` support at all** — `mdfes/builder.go`/`emit.go` only ever build the single traction vehicle — so making
`trailer` a selectable cadastro role requires also wiring trailer selection + `veicReboque` XML output into MDF-e
emission, otherwise the new role has no consumer.

NF-e's XSD allows `reboque` (up to 5), but the current NF-e builder (`nfes/builders_doc.go:130-144`) only ever emits
`veicTransp` (single vehicle, no trailers) — adding NF-e reboque support is a pre-existing gap, out of scope here (not
requested; NF-e's
`veicTransp`/`reboque` fields have no required fields beyond `placa` anyway, so gating has nothing to enforce there
regardless). CT-e OS has no emission service in `api/` at all yet (no `internal/services/ctes_os` or equivalent) — its
row in the matrix below is forward-compat only (also no required fields beyond `placa`), not wired into any emit path
since none exists.

## Goals

- Creating a vehicle requires only `plate` + `plate_uf` (the only fields universal to every doc-type/role per XSD, minus
  a deliberately stricter UF requirement for lookup/display).
- All other fields live behind an "advanced" section in `VehicleForm`, shown/hidden based on the vehicle's `role`.
- At emission time (NF-e, MDF-e, CT-e OS), selecting a vehicle missing a field required for that specific doc-type +
  role blocks progress and opens an edit modal (reusing `VehicleForm`)
  until the gap is filled.
- `condutor` (driver) stays out of the vehicle record entirely — collected per emission, since it varies too much to
  cadastro.
- Trailers become first-class `organization_vehicles` rows (`role=trailer`), independently selectable by any tractor,
  not nested under one.
- MDF-e emission gains trailer selection (up to 3) and `veicReboque` XML output, so the new
  `trailer` role is actually usable end-to-end, not just cadastro metadata.
- No data migration needed — table is pre-launch/empty.
- Field names in `organization_vehicles` are **not** renamed (`wheelset`, `bodywork`, `weight`
  stay as-is) — only relaxed from required to optional, plus new fields added. Renaming would touch every reader
  (`emit.go`, `builder.go`, DTOs, UI zod) for no functional gain (scope control — CLAUDE.md "no unrelated refactors").

## Non-goals

- No change to `condutor`/driver modeling (already separate).
- No py-dfe/worker business-logic changes — they receive pre-resolved, pre-validated dicts from
  `api`; only field-ordering/optionality awareness in XML builders needs review.

## Data model — `organization_vehicles`

Flat schema, `trailers[]` removed:

| Attribute                  | Type | Notes                                                                                                                                                                                                                                        |
|----------------------------|------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `pk`                       | S    | `{org_pk}` — partition key                                                                                                                                                                                                                   |
| `sk`                       | S    | `VEHICLE_{id}` — sort key                                                                                                                                                                                                                    |
| `role`                     | S    | **New.** `tractor` \| `trailer` — required, chosen at creation. GSI: `role-index`                                                                                                                                                            |
| `plate`                    | S    | Required (unchanged). GSI: `plate-index` (existing)                                                                                                                                                                                          |
| `plate_uf`                 | S    | Required (unchanged)                                                                                                                                                                                                                         |
| `renavam`                  | S    | Now optional (was required)                                                                                                                                                                                                                  |
| `weight` (tara)            | N    | Now optional at cadastro; required-for-emission per matrix below (was required)                                                                                                                                                              |
| `cap_kg`                   | N    | **New.** Optional at cadastro; required-for-emission per matrix below (trailer/mdfe)                                                                                                                                                         |
| `cap_m3`                   | N    | **New.** Optional, never required                                                                                                                                                                                                            |
| `wheelset` (tpRod)         | S    | Now optional; required-for-emission (mdfe tractor only) (was required)                                                                                                                                                                       |
| `bodywork` (tpCar)         | S    | Now optional; required-for-emission (mdfe, both roles) (was required)                                                                                                                                                                        |
| `cint`                     | S    | **New.** Optional, internal code                                                                                                                                                                                                             |
| `owner`                    | M    | Now fully optional (was `validate:"required"`); shape unchanged (`cpf_cnpj`, `rntrc`, `name`, `type`). Not part of the gating matrix — kept only as static metadata; `owner.rntrc` remains the existing infANTT fallback (`emit.go:551-555`) |
| `created_at`, `updated_at` | S    | ISO-8601 UTC (unchanged)                                                                                                                                                                                                                     |

**Correction from initial draft:** third-party vehicle ownership (`prop`/`owner` — CPF/CNPJ, RNTRC, xNome, tpProp,
TAF/NroRegEstadual) is **not** part of the cadastro or the gating matrix. It already exists purely as a per-emission
input (`MdfeOwner` in `MdfeEmitBody`,
`mdfes/emit.go:83-92`, validated in `resolveOwner`, `emit.go:571-600`) because who leases/operates a truck for a given
haul can legitimately vary trip-to-trip even for the same plate — the same "varies per emission" reasoning the task
gives for excluding `condutor`. This is left untouched; no `owned_by_org`/`owner` fields are added to
`organization_vehicles`.

**New GSI:** `role-index` (PK: `pk`, SK: `role`) — enables `Query(pk=org, role=trailer)` for trailer pickers without a
Scan (per `api/CLAUDE.md` DynamoDB rules: Query > Scan, no scans in production). Documented here as the access pattern
justifying its creation.

## Required-fields matrix (single source of truth, in `api`)

`internal/services/vehicles/requirements.go` — pure function
`Missing(vehicle map[string]types.AttributeValue, docType, role string) []string`:

| doc_type | role            | required beyond plate+plate_uf                    |
|----------|-----------------|---------------------------------------------------|
| mdfe     | tractor         | weight (tara), wheelset (tpRod), bodywork (tpCar) |
| mdfe     | trailer         | weight (tara), cap_kg, bodywork (tpCar)           |
| nfe      | tractor/trailer | (none)                                            |
| cte_os   | tractor         | (none)                                            |

(No owner/third-party-ownership row — see data model correction above; that stays a per-emission input, already
validated in `resolveOwner`.)

This is the only place the matrix is defined — `ui` never duplicates it (calls the endpoint below instead), avoiding
drift between Go and TS.

## API changes

- `internal/repositories/vehicles.go`: drop `trailers []map[string]any` param from
  `Create`/`BuildCreateTxItem` (still flat `owner` map, now optional); add `role` field to the item; add
  `ListByRole(ctx, orgPK, role, opts) (*QueryResult, error)` using the new
  `role-index`.
- `internal/services/vehicles.go`: `role` required (`tractor`/`trailer`) on create; drop the
  `trailers` handling; relax the currently-hardcoded required checks — no code change needed for
  renavam/weight/wheelset/bodywork/owner since `VehicleService.Create` never enforced them itself (validation lived in
  the DTO tags, see below).
- `internal/api/v1/dto.go`: `VehicleCreateBody`/`VehicleUpdateBody`/`VehicleTrailerBody` — add `Role string \`validate:
  "required,oneof=tractor trailer"\``; change `Wheelset`,
  `Bodywork`, `Renavam`, `Weight`, `Owner` from `validate:"required..."` to
  `validate:"omitempty..."`; add `CapKG *int`, `CapM3 *int`, `Cint *string` (all
  `omitempty`); remove `Trailers []VehicleTrailerBody` and `VehicleTrailerBody` itself
  (trailers are now plain vehicles with `role=trailer`, created via the same CRUD endpoints).
- `internal/api/v1/vehicles.go`:
    - `GET /vehicles?role=trailer` filter (uses `ListByRole`/`role-index`).
    - New `GET /vehicles/:sk/requirements?doc_type=&role=` → `{"missing": ["weight","bodywork"]}`
      (empty array = ready). Validates `doc_type`/`role` against named constants, 400 via
      `problem.BadRequest` on invalid values.
- `internal/services/mdfes/emit.go`:
    - `MdfeEmitBody` gains `Trailers []MdfeTrailer` (`{SK string}`, up to 3, validated
      `max=3`).
    - `resolveVehicle` calls `vehicles.Missing(vehicle, "mdfe", "tractor")` on the *registered*
      vehicle record (before merging request overrides) when `v.SK != nil`; non-empty ⇒
      `problem.BadRequest` with code `VEHICLE_INCOMPLETE` and the missing-field list. This replaces the silent
      `defaultTpRod`/`defaultTpCar` fallback (`emit.go:558-562`) — removed entirely; `defaultTpRod`/`defaultTpCar`
      constants in `builder.go` removed too.
    - New `resolveTrailers(ctx, orgPK, trailers []MdfeTrailer) ([]resolvedVehicle, error)`
      mirrors `resolveVehicle` for each trailer SK, gating on
      `vehicles.Missing(vehicle, "mdfe", "trailer")`.
- `internal/services/mdfes/builder.go`: `buildParams` gains `trailers []resolvedVehicle`;
  `buildRodo` emits a `veicReboque` array (one node per trailer: `placa`, `tara`, `capKG`,
  `tpCar`, optional `RENAVAM`/`capM3`/`UF`) alongside the existing `veicTracao`.
- CT-e OS: no emit service exists yet, so no wiring — `vehicles.Missing(v, "cte_os", "tractor")`
  is implemented and tested now so it's ready when that service is built.

## UI changes

- `ui/src/lib/schemas/vehicles.ts`: base schema (`plate`, `plate_uf`, `role`) required; all else (`renavam`, `weight`,
  `wheelset`, `bodywork`, `cap_kg`, `cap_m3`, `cint`, `owner`)
  optional. Drop the `trailers` array field entirely.
- `ui/src/components/vehicles/VehicleForm.tsx`:
    - Role selector (`Tração` / `Reboque`) at top, changes which advanced fields render (trailer role hides `wheelset`
      /tipo-de-rodado; both roles show `weight`/`bodywork`/ capacities/owner block).
    - Advanced fields behind a chevron, collapsed by default.
- `ui/src/app/vehicles/{page,new,edit}.tsx`: no structural change beyond following the schema/ form updates; trailer
  vehicles now list and edit exactly like tractor vehicles (same pages), distinguished by `role`.
- `MdfeEmitForm.tsx`:
    - Add a trailer picker (0–3, `GET /vehicles?role=trailer`) alongside the existing tractor picker, replacing the
      current nested-array trailer UI (which had no wiring to emission anyway).
    - On tractor or trailer selection, call `GET /vehicles/:sk/requirements?doc_type=mdfe&role=`. Non-empty `missing` ⇒
      open a modal wrapping `VehicleForm` (advanced section pre-expanded), blocking that slot until saved. Reuses the
      existing `VehicleRegisterModal` component (currently create-only) for both "no vehicle yet" and "vehicle
      incomplete" cases.
- `NfeEmitForm.tsx`: no gating needed (NF-e has no required fields beyond `placa` in the matrix) — left as-is including
  its existing free-text fallback path.

## py-dfe / worker

No changes needed. Confirmed `py-dfe` already fully supports `veicReboque`:
`xsmlops/xsd_order.py:520,527` already lists `veicReboque` under `rodo` and defines its own field order
(`cInt, placa, RENAVAM, tara, capKG, capM3, prop, tpCar, UF`), and
`danfe/mdfe58.py:127` already renders a `veicReboque` list in the DANFE — both were built ready for this, just never had
a caller populating the data. `api` only needs to start sending a non-empty `veicReboque` list.

## Testing

- Go: unit tests for `Missing()` covering every `doc_type`×`role` combination in the matrix table above. Integration
  test for the new `/vehicles/:sk/requirements` endpoint, for emit-time blocking on an incomplete tractor (expect
  `problem.BadRequest` /
  `VEHICLE_INCOMPLETE`), and for successful emission with a complete tractor + 1-2 trailers producing `veicReboque`
  nodes. Regression test: confirm the `defaultTpRod`/`defaultTpCar`
  fallback path no longer exists (incomplete tractor now errors, not defaults).
- Repository integration test for `role-index` query (`ListByRole`).
- UI: no test suite currently enforced beyond `npx eslint src --ext .ts,.tsx` (zero errors/warnings) per `ui/CLAUDE.md`.

## Docs to update after implementation

- `DynamoDB-Tables.md` §5 (`organization_vehicles`): new schema (`role`, `cap_kg`, `cap_m3`,
  `cint`; `renavam`/`weight`/`wheelset`/`bodywork`/`owner` now optional; `trailers[]` removed)
    + `role-index` GSI.
- `DOCS.md`: new `/vehicles/:sk/requirements` endpoint, updated vehicle CRUD contract (`trailers[]` removed, `role`
  added), `MdfeEmitBody.trailers[]` + `veicReboque` output.
- `CONDUCT.md`: note the required-fields matrix (`vehicles.Missing`) as the single source of truth for vehicle
  completeness, and the removal of silent `TpRod`/`TpCar` defaulting.
