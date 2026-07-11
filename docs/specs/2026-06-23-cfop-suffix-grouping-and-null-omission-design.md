# Design — CFOP suffix grouping (UF-dynamic) + application-wide null omission

**Date:** 2026-06-23
**Projects:** `ui` (Next.js), `api` (Go), `worker` (Go)
**Status:** Approved for planning

---

## 1. Background & Goals

Two independent changes, bundled because both touch the NF-e issuance flow / data layer.

### Feature A — CFOP suffix grouping with UF-dynamic selection (`ui` only)

A product's `cfop_config` may contain several CFOPs that represent the **same fiscal
nature** but differ only by destination scope. In Brazilian fiscal rules, a saída
(outgoing) CFOP is `[scope][suffix]`:

- **scope digit** — `5` = intra-state (same UF), `6` = inter-state (other UF), `7` = exterior.
- **suffix** (last 3 digits) — the fiscal nature (e.g. `920` = "Remessa de vasilhame").

So `5920` and `6920` are the same nature ("920") differing only by whether the
destinatário is in the issuer's UF. Today the NF-e emit form lists every CFOP from
`cfop_config` as a separate dropdown entry, forcing the operator to manually pick the
right scope.

**Goal:** Group same-suffix CFOPs into a single dropdown option. The concrete CFOP sent
to the backend is resolved automatically from whether the recipient is in the same UF as
the issuer, and re-resolves dynamically when the recipient changes.

### Feature B — Omit null values to reduce DynamoDB storage (`ui` + `api` + `worker`)

Items are persisted to DynamoDB with many explicit `NULL` attributes (a product item
carries ~70 fields, most null). Each stored `NULL` consumes item size. Goal: stop
persisting null/absent attributes, application-wide, **without** changing the API contract
(responses still expose `null`; clients can still clear a field).

---

## 2. Feature A — CFOP suffix grouping

### 2.1 Decisions

- **Dropdown UX:** collapse same-suffix CFOPs into **one option** per nature; scope
  (`5`↔`6`) resolved automatically from UF. (Not "list all + preselect".)
- **Missing variant → block emission.** If the selected nature group has no variant for
  the required scope (e.g. inter-state sale but only `5405` configured), block issuance
  with a validation error instructing the user to configure the `6xxx` CFOP on the product.
  No auto-synthesis of CFOPs (tax config may differ inter-UF).
- Client-side only. Backend NF-e contract is unchanged — it still receives a concrete
  `cfop` string per product line.

### 2.2 UF resolution

- **Issuer UF:** `useAuth().n.state_federation` (`UserOrganization.state_federation`,
  already present in `src/lib/types/api.ts`).
- **Recipient UF:** `receiver.person.addresses[0].state_federation`
  (`PersonAddressOut.state_federation`). When `selfIssuance` is true, recipient = issuer ⇒
  `sameUf = true`.
- `sameUf: boolean` is computed in `NfeEmitForm` and passed to each `ProductRow`.
- If recipient UF is unknown (no receiver selected yet, address missing), treat as
  `sameUf = null` → CFOP stays unresolved and the product step cannot advance until a
  recipient with a UF is chosen (current flow already requires a receiver before products).

### 2.3 New helpers — `src/lib/data/cfop.ts`

```ts
// scope digit: '5' intra-UF, '6' inter-UF, '7' exterior
export const cfopScope = (cfop: string): string => cfop.charAt(0)
// fiscal nature (last 3 digits) — shared across intra/inter variants
export const cfopSuffix = (cfop: string): string => cfop.slice(1)

export interface CfopSuffixGroup {
  suffix: string
  intra?: string   // 5xxx member
  inter?: string   // 6xxx member
  label: string    // nature description, from getCfopDescription/getCfopHint
}

// Group a product's cfop_config entries by suffix.
export function groupCfopConfigBySuffix(config: CfopConfigItem[]): CfopSuffixGroup[]

// Resolve the concrete CFOP for a group given same-UF flag.
// Returns null when the required-scope variant is not configured.
export function resolveCfopForUf(group: CfopSuffixGroup, sameUf: boolean): string | null
```

`CfopConfigItem` already exists (used in `NfeEmitForm.tsx:535`). The nature label reuses
the existing `getCfopDescription` (`cfop.ts`) and may enrich from `getCfopHint`
(`cfop_rules.ts`); no new description tables.

### 2.4 NfeEmitForm / ProductRow changes

- `EmitProduct` gains `cfopSuffix: string` (the selected nature group). `cfop` (concrete,
  sent to backend) becomes **derived** from `cfopSuffix` + `sameUf`.
- `ProductRow` receives `sameUf: boolean | null` as a prop. Its dropdown options come from
  `groupCfopConfigBySuffix(item.product.cfop_config)` — one option per suffix,
  `value = suffix`, label = group label.
- On suffix select: store `cfopSuffix`; derive `cfop = resolveCfopForUf(group, sameUf)`.
- Below the dropdown, show the resolved concrete CFOP read-only (e.g. badge "→ 6920") so
  the operator sees what will be transmitted. When unresolved (missing variant), show the
  block message instead.
- **Dynamic re-resolution:** when `sameUf` changes (recipient changed), a `useEffect` /
  derivation re-maps every product line's `cfop` from its stored `cfopSuffix`. No manual
  re-pick required.
- Products with no `cfop_config` keep the existing free-text `Input` fallback
  (`NfeEmitForm.tsx:573`) unchanged.

### 2.5 Validation / blocking

- New per-line validity: `resolveCfopForUf` returned `null` ⇒ line is invalid.
- `canAdvance('produtos')` (currently `products.length > 0 && !cfopMixError`) gains
  `&& products.every(p => p.cfop)` (every line resolved).
- Submit (`NfeEmitForm.tsx:1013` area) guards on the same condition with an inline error
  listing the offending product(s) and the missing scope.
- The existing `cfopMixError` (intra/inter mix) logic in `NfeEmitForm.tsx:867-869` keeps
  working — it reads the resolved concrete `cfop`, so a single note can't mix scopes.

### 2.6 Tests (`ui`)

- Unit: `cfopScope`, `cfopSuffix`, `groupCfopConfigBySuffix`, `resolveCfopForUf`
  (same-UF, other-UF, missing-variant → null). Extend `src/__tests__/lib/cfop.test.ts`.
- Component: `ProductRow` renders one option per suffix; selecting a nature + flipping
  `sameUf` flips the resolved CFOP; missing variant shows block message.

---

## 3. Feature B — Null omission

### 3.1 Decisions

- **Scope:** both `ui` and backend (`api` + `worker`), in this one effort.
- **Storage-only semantics:** the API contract stays nullable. Responses still expose
  `null`; a client can still clear a field. Only the *persisted* item drops null attrs.
- Clearing a field must keep working (PATCH/PUT with explicit `null`).

### 3.2 Backend — `api` (and mirrored in `worker`)

The request→storage path: `bindAV` (`api/internal/api/v1/helpers.go:164`) binds JSON into
`map[string]any` then `attributevalue.MarshalMap(body)`. JSON `null` → Go `nil` → default
encoder emits `AttributeValueMemberNULL`. Two write paths exist:

1. **PutItem (full replace)** — `Base.PutItem` (`base.go:86`). Used by Create and
   full-object updates. Dropping null attrs here = attribute simply absent = effectively
   cleared on replace. Safe.
2. **UpdateItem (SET expression)** — `Base.UpdateItem` (`base.go:96`). Builds
   `SET #x = :x` for each key; a nil value currently marshals to `:NULL` and **writes a
   NULL attribute** (wastes space, does not remove).

**Changes:**

- **Shared omit-null encoder.** Add a package-level encoder configured with
  `OmitNullAttributeValues: true` (available in
  `feature/dynamodb/attributevalue v1.20.48`). Provide a helper, e.g.
  `MarshalMapOmitNull(v any)`, and use it in `bindAV` and the other `MarshalMap` call
  sites that persist user data (`organizations.go:72`, `documents.go:230`,
  `repositories/base.go:312`, etc.). Audit each `MarshalMap`/`Marshal` site; apply where
  the result is written to DynamoDB (skip cache-serialization sites if behavior must be
  preserved — to be confirmed during implementation).
- **UpdateItem REMOVE-on-nil.** In `Base.UpdateItem`, partition `updates`: nil values →
  `REMOVE #x` clause; non-nil → `SET #x = :x`. Build a combined
  `SET ... REMOVE ...` expression. This clears the field **and** stores nothing.
- **Drop explicit NULL writes:** `dfe_events.go:77`
  (`item[key] = &types.AttributeValueMemberNULL{...}`) and the `fiscal_config.go`
  default-fill that marshals null defaults — omit the attribute instead of writing NULL.
- **Reads unchanged.** `unmarshal`/`sendItem` already reconstruct absent attributes as
  Go zero/`nil` → JSON `null`. API responses keep their current nullable shape, so the
  contract and the `ui` types are unaffected.

### 3.3 Backend tests

- Unit/contract: a Create with null fields persists an item with those attributes absent;
  a subsequent Get returns them as `null` in JSON (contract preserved).
- Unit: `UpdateItem` with a nil value issues a `REMOVE` (regression: field gone, no NULL
  stored); with a non-nil value issues `SET`.
- Integration (AWS): round-trip Put→Get and Update(clear)→Get against DynamoDB Local.

### 3.4 UI — `src/lib/api/client.ts`

- Add a `stripNulls` deep utility and apply it in the Axios **request interceptor**:
  - Always drop `undefined`.
  - Drop `null` on `POST` (create — no clear semantics).
  - Keep `null` on `PUT`/`PATCH` (explicit `null` = clear; backend now turns that into a
    `REMOVE`).
- Applied centrally → DRY, app-wide (products, persons, vehicles, fiscal-config, NF-e,
  etc.). No per-form changes.
- `ORG_HEADER` injection and the silent-refresh interceptor are untouched; `stripNulls`
  is a separate, ordering-independent request transform.

### 3.5 UI tests

- Unit: `stripNulls` removes nulls/undefined deeply; preserves `0`, `''`, `false`, arrays.
- Unit (ApiClient, mocked axios): POST body has nulls stripped; PUT/PATCH body retains
  explicit nulls.

---

## 4. Cross-project impact & docs

- **ui ↔ api ↔ worker.** Feature A is `ui`-only. Feature B spans all three.
- Update `DOCS.md` / `INTEGRATION.md`: document that **storage omits null attributes while
  the API contract remains nullable** (responses still emit `null`; clients clear fields
  by sending `null`, persisted as a DynamoDB `REMOVE`).
- Update `CONDUCT.md`: new constraint — "Do not write `NULL` attributes to DynamoDB; use
  the omit-null encoder for Puts and `REMOVE` for cleared fields on Updates."

## 5. Out of scope (YAGNI)

- No CFOP auto-synthesis (deriving `6xxx` from `5xxx`).
- No exterior (`7xxx`) CFOP handling beyond what already exists.
- No migration/back-fill to strip NULLs from already-stored items (new writes only).
- No change to API response shapes or `ui` response types.

## 6. Risks

- **Encoder audit completeness (B).** `MarshalMap` is used for both storage and cache
  serialization. Applying omit-null to a cache path could change cache round-trips. Each
  call site is classified during implementation; only storage writes get omit-null.
- **UpdateItem expression edge case (B).** An update whose keys are *all* nil produces a
  `REMOVE`-only expression (no `SET`) — must build the expression accordingly (no dangling
  `SET`).
- **Blocking strictness (A).** Blocking inter-state issuance when only an intra CFOP is
  configured is intentional (user decision) but changes operator workflow; the error
  message must clearly state which product and which scope CFOP to add.
