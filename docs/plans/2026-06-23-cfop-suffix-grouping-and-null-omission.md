# CFOP Suffix Grouping + Null Omission Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:
> executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Group same-suffix saída CFOPs into one NF-e dropdown option resolved by destination UF, and stop persisting
null attributes to DynamoDB across `ui`, `api`, and `worker` while keeping the API contract nullable.

**Architecture:** Feature A is client-only: helpers in `cfop.ts` group `cfop_config` by the 3-digit suffix;
`NfeEmitForm` derives the concrete CFOP (5xxx intra / 6xxx inter) from issuer-vs-recipient UF and re-resolves
dynamically. Feature B centralizes null omission at each storage encode point: a shared omit-null encoder in `api`, a
`REMOVE`-on-nil UpdateItem, the worker's `mapToAttr` skipping nils, and a `stripNulls` request transform in the UI
ApiClient. Reads are untouched, so JSON responses keep emitting `null`.

**Tech Stack:** Next.js 16 / React 19 / TypeScript / Vitest+RTL (ui); Go 1.x / aws-sdk-go-v2 / Fiber v3 (api, worker).

## Global Constraints

- `ui`: `npx eslint src --ext .ts,.tsx` must pass with **zero errors and zero warnings** before any commit.
- `ui`: all API calls go through `ApiClient`; never hardcode `ORG_HEADER` (`'Dfe-Organization-Pk'`, defined once in
  `client.ts`); never write `access_token` to storage.
- `ui`: mobile-first — inputs `w-full`, touch targets, no horizontal overflow at 375px.
- Backend storage encoder option: `OmitNullAttributeValues` (available in `feature/dynamodb/attributevalue v1.20.48`).
- API contract stays nullable: responses still emit `null`; only persisted items drop null attributes.
- `api`/`worker`: all errors via `problem.*` / structured errors (existing pattern; no new error paths introduced here).
- Conventional Commits, no emojis.

---

## File Structure

**Feature A (ui):**

- `src/lib/data/cfop.ts` — add `cfopScope`, `cfopSuffix`, `groupCfopConfigBySuffix`, `resolveCfopForUf`,
  `CfopSuffixGroup`.
- `src/components/nfe/NfeEmitForm.tsx` — `sameUf` computation, `EmitProduct.cfopSuffix`, grouped dropdown, dynamic
  re-resolution, block validation.
- `src/__tests__/lib/cfop.test.ts` — unit tests for new helpers.

**Feature B (api):**

- `api/internal/repositories/marshal.go` — new: `MarshalMapOmitNull`; route `Encode`/`EncodeItem` through it.
- `api/internal/repositories/base.go` — `UpdateItem` REMOVE-on-nil via extracted `buildUpdateExpr`.
- `api/internal/repositories/persons.go`, `fiscal_config.go`, `dfe_events.go` — skip nil instead of writing NULL.
- `api/internal/api/v1/helpers.go`, `organizations.go` — use the omit-null encode helper.
- `api/internal/repositories/marshal_test.go`, `base_test.go` — unit tests.

**Feature B (worker):**

- `worker/internal/service/distribution.go` — `mapToAttr` skips nil values.
- `worker/internal/service/distribution_maptoattr_test.go` — unit test.

**Feature B (ui):**

- `src/lib/utils/strip-nulls.ts` — new: `stripNulls`.
- `src/lib/api/client.ts` — apply `stripNulls` in request interceptor.
- `src/__tests__/lib/strip-nulls.test.ts` — unit test.

**Docs:** `DOCS.md`, `INTEGRATION.md`, `CONDUCT.md`.

---

## Task 1: CFOP suffix helpers (ui)

**Files:**

- Modify: `src/lib/data/cfop.ts` (append near `cfopDirection`, ~line 3631)
- Test: `src/__tests__/lib/cfop.test.ts`

**Interfaces:**

- Consumes: `getCfopDescription(code: string): string | null` (cfop.ts:3592); `CfopConfigItem` (`@/lib/types/api`, has
  `cfop: string`).
- Produces:
    - `cfopScope(cfop: string): string`
    - `cfopSuffix(cfop: string): string`
    - `interface CfopSuffixGroup { suffix: string; intra?: string; inter?: string; label: string }`
    - `groupCfopConfigBySuffix(config: CfopConfigItem[]): CfopSuffixGroup[]`
    - `resolveCfopForUf(group: CfopSuffixGroup, sameUf: boolean): string | null`

- [ ] **Step 1: Write the failing tests**

Add to `src/__tests__/lib/cfop.test.ts` (create the file if absent, mirroring existing Vitest imports
`import {describe, it, expect} from 'vitest'`):

```ts
import {describe, it, expect} from 'vitest'
import {
  cfopScope, cfopSuffix, groupCfopConfigBySuffix, resolveCfopForUf,
} from '@/lib/data/cfop'
import type {CfopConfigItem} from '@/lib/types/api'

const cc = (cfop: string): CfopConfigItem => ({cfop} as CfopConfigItem)

describe('cfop scope/suffix', () => {
  it('splits scope and suffix', () => {
    expect(cfopScope('5920')).toBe('5')
    expect(cfopSuffix('5920')).toBe('920')
    expect(cfopScope('6920')).toBe('6')
    expect(cfopSuffix('6920')).toBe('920')
  })
})

describe('groupCfopConfigBySuffix', () => {
  it('pairs intra and inter variants under one suffix', () => {
    const groups = groupCfopConfigBySuffix([cc('5405'), cc('5920'), cc('6920')])
    const g920 = groups.find(g => g.suffix === '920')!
    const g405 = groups.find(g => g.suffix === '405')!
    expect(groups).toHaveLength(2)
    expect(g920.intra).toBe('5920')
    expect(g920.inter).toBe('6920')
    expect(g405.intra).toBe('5405')
    expect(g405.inter).toBeUndefined()
  })
})

describe('resolveCfopForUf', () => {
  const groups = groupCfopConfigBySuffix([cc('5405'), cc('5920'), cc('6920')])
  const g920 = groups.find(g => g.suffix === '920')!
  const g405 = groups.find(g => g.suffix === '405')!

  it('returns intra variant when same UF', () => {
    expect(resolveCfopForUf(g920, true)).toBe('5920')
    expect(resolveCfopForUf(g405, true)).toBe('5405')
  })
  it('returns inter variant when other UF', () => {
    expect(resolveCfopForUf(g920, false)).toBe('6920')
  })
  it('returns null when required scope variant is missing', () => {
    expect(resolveCfopForUf(g405, false)).toBeNull()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ui && npx vitest run src/__tests__/lib/cfop.test.ts`
Expected: FAIL — `cfopScope`/`cfopSuffix`/`groupCfopConfigBySuffix`/`resolveCfopForUf` are not exported.

- [ ] **Step 3: Implement the helpers**

Append to `src/lib/data/cfop.ts` (after the `cfopTpNf` export, ~line 3647). `CfopConfigItem` is already importable from
`@/lib/types/api`; add the import at the top of the file if not present (
`import type {CfopConfigItem} from '@/lib/types/api'`):

```ts
// ─── CFOP suffix grouping (UF-dynamic selection) ──────────────────────────────
// A saída CFOP is [scope][suffix]: scope '5' = intra-UF, '6' = inter-UF,
// '7' = exterior. The 3-digit suffix is the fiscal nature, shared across the
// intra/inter variants (e.g. 5920 and 6920 are both nature "920").

/** Scope digit of a CFOP ('5' intra-UF, '6' inter-UF, '7' exterior). */
export const cfopScope = (cfop: string): string => cfop.charAt(0)

/** Fiscal-nature suffix (last 3 digits), shared across intra/inter variants. */
export const cfopSuffix = (cfop: string): string => cfop.slice(1)

export interface CfopSuffixGroup {
  suffix: string
  intra?: string   // 5xxx member
  inter?: string   // 6xxx member
  label: string    // nature description (from getCfopDescription)
}

/** Groups a product's cfop_config entries by fiscal-nature suffix. */
export const groupCfopConfigBySuffix = (config: CfopConfigItem[]): CfopSuffixGroup[] => {
  const bySuffix = new Map<string, CfopSuffixGroup>()
  for (const item of config) {
    const cfop = item.cfop
    if (!cfop) continue
    const suffix = cfopSuffix(cfop)
    const group = bySuffix.get(suffix) ?? {suffix, label: ''}
    if (cfopScope(cfop) === '6') group.inter = cfop
    else group.intra = cfop
    // Prefer a description from whichever variant resolves one.
    if (!group.label) group.label = getCfopDescription(cfop) ?? ''
    bySuffix.set(suffix, group)
  }
  return [...bySuffix.values()]
}

/**
 * Resolves the concrete CFOP for a group given whether the recipient is in the
 * issuer's UF. Returns null when the required-scope variant is not configured
 * (caller must block emission).
 */
export const resolveCfopForUf = (group: CfopSuffixGroup, sameUf: boolean): string | null =>
  (sameUf ? group.intra : group.inter) ?? null
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ui && npx vitest run src/__tests__/lib/cfop.test.ts`
Expected: PASS (all cases green).

- [ ] **Step 5: Lint**

Run: `cd ui && npx eslint src/lib/data/cfop.ts src/__tests__/lib/cfop.test.ts`
Expected: zero errors, zero warnings.

- [ ] **Step 6: Commit**

```bash
git add ui/src/lib/data/cfop.ts ui/src/__tests__/lib/cfop.test.ts
git commit -m "feat(ui): add CFOP suffix grouping and UF-based resolution helpers"
```

---

## Task 2: NfeEmitForm CFOP grouping integration (ui)

**Files:**

- Modify: `src/components/nfe/NfeEmitForm.tsx`
    - `EmitProduct` interface (~line 43)
    - `ProductRowProps` + `ProductRow` (~lines 527-575)
    - `handleSelectProduct` (~line 939)
    - `sameUf` computation + `canAdvance`/submit guards (~lines 867-906, 1013)
    - `ProductRow` render site (~line 1178)
- Test: `src/__tests__/components/NfeProductRow.test.tsx` (new)

**Interfaces:**

- Consumes: `groupCfopConfigBySuffix`, `resolveCfopForUf`, `cfopSuffix`, `CfopSuffixGroup` (Task 1);
  `useAuth().selectedOrg.state_federation` (`UserOrganization`, api.ts);
  `receiver.person.addresses[0].state_federation` (`PersonAddressOut`); existing `OptionsSelect`.
- Produces: `EmitProduct.cfopSuffix: string`; per-line resolved `cfop` sent to backend unchanged.

- [ ] **Step 1: Add `cfopSuffix` to `EmitProduct`**

In `src/components/nfe/NfeEmitForm.tsx`, the `EmitProduct` interface (line 43):

```ts
interface EmitProduct {
  product: ProductOut
  cfop: string
  cfopSuffix: string
  qty: string
  unitValue: string
  discount: string
  // veicProd — por unidade
  veic_chassi?: string
  veic_n_serie?: string
  veic_n_motor?: string
  veic_c_cor?: string
  veic_x_cor?: string
  // arma — por unidade (list)
  armas?: NfeArmaIn[]
}
```

- [ ] **Step 2: Compute `sameUf` and pass it down**

Add the import (top of file, with the other `@/lib/data/cfop` import on line 36):

```ts
import {getCfopDescription, cfopDirection, cfopTpNf, buildNatOpFromCfops,
        cfopSuffix, groupCfopConfigBySuffix, resolveCfopForUf} from "@/lib/data/cfop"
```

In the component body (near the other derived values, after the `selectedOrg` usage around line 844), add:

```ts
// Recipient in the issuer's UF? Self-issuance ⇒ always same UF.
const issuerUf = selectedOrg?.state_federation ?? null
const recipientUf = selfIssuance
  ? issuerUf
  : (receiver?.person.addresses?.[0]?.state_federation ?? null)
const sameUf: boolean | null =
  issuerUf && recipientUf ? issuerUf === recipientUf : null
```

- [ ] **Step 3: Re-resolve every line's CFOP when `sameUf` changes**

Add a `useEffect` after the `sameUf` computation (uses the existing `setProducts`):

```ts
useEffect(() => {
  if (sameUf === null) return
  setProducts(prev => prev.map(item => {
    if (!item.cfopSuffix) return item
    const groups = groupCfopConfigBySuffix(item.product.cfop_config)
    const group = groups.find(g => g.suffix === item.cfopSuffix)
    if (!group) return item
    return {...item, cfop: resolveCfopForUf(group, sameUf) ?? ''}
  }))
}, [sameUf])
```

- [ ] **Step 4: Seed `cfopSuffix` on product add**

In `handleSelectProduct` (line 939), replace the `firstCfop`/`setProducts` block:

```ts
const handleSelectProduct = (product: ProductOut) => {
  const groups = groupCfopConfigBySuffix(product.cfop_config)
  const firstGroup = groups[0]
  const firstSuffix = firstGroup?.suffix ?? (product.cfop_nfce ? cfopSuffix(product.cfop_nfce) : '')
  const resolvedCfop = firstGroup && sameUf !== null
    ? (resolveCfopForUf(firstGroup, sameUf) ?? '')
    : (product.cfop_config[0]?.cfop ?? product.cfop_nfce ?? '')
  // NF-e: consumer-final price for CPF, resale price for CNPJ (self-issuance = org CNPJ).
  const recipientDoc = selfIssuance
    ? unformatCpfCnpj(selectedOrg?.pk ?? '')
    : unformatCpfCnpj(receiver?.sk ?? '')
  setProducts(prev => [...prev, {
    product, cfop: resolvedCfop, cfopSuffix: firstSuffix, qty: '1',
    unitValue: resolveUnitPrice(product, recipientDoc), discount: '0',
    armas: product.prod_type === 'arma' ? [] : undefined,
  }])
  setShowProductPicker(false)
}
```

- [ ] **Step 5: Grouped dropdown in `ProductRow`**

Add `sameUf: boolean | null` to `ProductRowProps` (line 527) and the destructure (line 534). Replace the `cfopOptions`
block (lines 535-539) and the CFOP `<div>` (lines 567-576):

```ts
interface ProductRowProps {
  item: EmitProduct
  index: number
  sameUf: boolean | null
  onChange: (index: number, updated: Partial<EmitProduct>) => void
  onRemove: (index: number) => void
}

function ProductRow({item, index, sameUf, onChange, onRemove}: ProductRowProps) {
  const cfopGroups = groupCfopConfigBySuffix(item.product.cfop_config)
  const cfopOptions = cfopGroups.map((g) => {
    const label = g.label ? `${g.suffix} – ${g.label}` : g.suffix
    return {value: g.suffix, label, display: label}
  })
  const selectedGroup = cfopGroups.find(g => g.suffix === item.cfopSuffix) ?? null
  const cfopUnresolved = selectedGroup !== null && sameUf !== null
    && resolveCfopForUf(selectedGroup, sameUf) === null
  // ... existing total/isVeiculo/isArma/newArma unchanged ...
```

Selection handler must store the suffix and derive the concrete cfop. Replace the CFOP field markup:

```tsx
<div className="flex flex-col gap-1">
  <Label className="text-xs font-medium text-gray-600">CFOP</Label>
  {cfopOptions.length > 0 ? (
    <OptionsSelect
      value={item.cfopSuffix}
      onValueChange={(suffix) => {
        const group = cfopGroups.find(g => g.suffix === suffix)
        const resolved = group && sameUf !== null ? resolveCfopForUf(group, sameUf) : null
        onChange(index, {cfopSuffix: suffix, cfop: resolved ?? ''})
      }}
      options={cfopOptions} placeholder="CFOP"/>
  ) : (
    <Input type="text" value={item.cfop} onChange={(e) => onChange(index, {cfop: e.target.value})}
           maxLength={4} placeholder="5102"/>
  )}
  {item.cfop && !cfopUnresolved && (
    <span className="text-xs text-gray-400">→ {item.cfop}</span>
  )}
  {cfopUnresolved && (
    <span className="text-xs text-red-600">
      Configure o CFOP {sameUf ? '5' : '6'}xxx neste produto para esta UF de destino.
    </span>
  )}
</div>
```

Pass `sameUf` at the render site (line 1178):

```tsx
<ProductRow key={`${item.product.sk}-${i}`} item={item} index={i} sameUf={sameUf}
            onChange={handleProductChange} onRemove={handleProductRemove}/>
```

- [ ] **Step 6: Block emission when any line is unresolved**

Add a derived flag near `cfopMixError` (line 868):

```ts
const cfopUnresolvedError = products.some(p => p.cfopSuffix && !p.cfop)
```

Extend `canAdvance('produtos')` (line 906):

```ts
if (step === 'produtos') return products.length > 0 && !cfopMixError && !cfopUnresolvedError
```

In `handleSubmit` (after the `cfopMixError` guard, ~line 1016):

```ts
if (cfopUnresolvedError) {
  setSubmitError('Há produtos sem CFOP válido para a UF do destinatário. Configure o CFOP de mesma natureza para a UF correta.')
  return
}
```

- [ ] **Step 7: Write the component test**

Create `src/__tests__/components/NfeProductRow.test.tsx`. (Verify the export style of `ProductRow` first; if it is
module-private, export it for test, or test through `NfeEmitForm`. The plan assumes `ProductRow` is exported — add
`export` to its declaration if needed.)

```tsx
import {describe, it, expect, vi} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {ProductRow} from '@/components/nfe/NfeEmitForm'
import type {ProductOut} from '@/lib/types/api'

const product = {
  sk: 'PRODUCT_1', description: 'Vasilhame', cfop_config: [
    {cfop: '5405'}, {cfop: '5920'}, {cfop: '6920'},
  ],
} as unknown as ProductOut

const baseItem = {product, cfop: '5920', cfopSuffix: '920', qty: '1', unitValue: '10', discount: '0'}

describe('ProductRow CFOP grouping', () => {
  it('shows one option per suffix and resolves intra when same UF', () => {
    const onChange = vi.fn()
    render(<ProductRow item={baseItem} index={0} sameUf={true} onChange={onChange} onRemove={() => {}}/>)
    expect(screen.getByText('→ 5920')).toBeInTheDocument()
  })

  it('shows block message when inter variant is missing for other UF', () => {
    const onChange = vi.fn()
    const item = {...baseItem, cfop: '', cfopSuffix: '405'}
    render(<ProductRow item={item} index={0} sameUf={false} onChange={onChange} onRemove={() => {}}/>)
    expect(screen.getByText(/Configure o CFOP 6xxx/)).toBeInTheDocument()
  })
})
```

- [ ] **Step 8: Run the component test**

Run: `cd ui && npx vitest run src/__tests__/components/NfeProductRow.test.tsx`
Expected: PASS. (If `OptionsSelect` needs a portal/provider, wrap in the existing test render helper used elsewhere in
`src/__tests__/components`.)

- [ ] **Step 9: Lint + full test sweep**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero eslint errors/warnings; all tests pass.

- [ ] **Step 10: Commit**

```bash
git add ui/src/components/nfe/NfeEmitForm.tsx ui/src/__tests__/components/NfeProductRow.test.tsx
git commit -m "feat(ui): group saida CFOPs by suffix and resolve by destination UF in NF-e emit"
```

---

## Task 3: Omit-null encoder (api)

**Files:**

- Create: `api/internal/repositories/marshal.go`
- Modify: `api/internal/repositories/base.go` (`Encode`, line 311), `documents.go` (`EncodeItem`, line 229)
- Modify: `api/internal/api/v1/helpers.go` (`bindAV`, line 159), `api/internal/api/v1/organizations.go` (line 72)
- Test: `api/internal/repositories/marshal_test.go`

**Interfaces:**

- Produces: `repositories.MarshalMapOmitNull(v any) (map[string]types.AttributeValue, error)` — marshals omitting null
  attributes; `Encode`/`EncodeItem` delegate to it.

- [ ] **Step 1: Write the failing test**

Create `api/internal/repositories/marshal_test.go`:

```go
package repositories

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestMarshalMapOmitNull(t *testing.T) {
	in := map[string]any{
		"name":  "Vasilhame",
		"cest":  nil,
		"value": "20.00",
		"empty": "",
	}
	out, err := MarshalMapOmitNull(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := out["cest"]; ok {
		t.Errorf("expected null attribute 'cest' to be omitted, got %#v", out["cest"])
	}
	if _, ok := out["name"]; !ok {
		t.Errorf("expected 'name' to be present")
	}
	if s, ok := out["empty"].(*types.AttributeValueMemberS); !ok || s.Value != "" {
		t.Errorf("expected empty string to be preserved, got %#v", out["empty"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/ -run TestMarshalMapOmitNull -v`
Expected: FAIL — `MarshalMapOmitNull` undefined.

- [ ] **Step 3: Implement the encoder helper**

Create `api/internal/repositories/marshal.go`:

```go
package repositories

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// MarshalMapOmitNull marshals v into a DynamoDB attribute map, omitting any
// attribute whose value is null. This keeps stored items small without
// changing the API contract — reads reconstruct absent attributes as null.
func MarshalMapOmitNull(v any) (map[string]types.AttributeValue, error) {
	enc := attributevalue.NewEncoder(func(o *attributevalue.EncoderOptions) {
		o.OmitNullAttributeValues = true
	})
	av, err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	m, ok := av.(*types.AttributeValueMemberM)
	if !ok {
		return nil, fmt.Errorf("marshal: expected map attribute value, got %T", av)
	}
	return m.Value, nil
}
```

- [ ] **Step 4: Route existing encode helpers through it**

In `base.go` (line 311), `Encode`:

```go
// Encode marshals a value into DynamoDB attribute values, omitting nulls.
func Encode(v any) (map[string]types.AttributeValue, error) {
	return MarshalMapOmitNull(v)
}
```

In `documents.go` (line 229), `EncodeItem`:

```go
// EncodeItem encodes a map[string]any into DynamoDB attribute values, omitting nulls.
func EncodeItem(item map[string]any) (map[string]types.AttributeValue, error) {
	return MarshalMapOmitNull(item)
}
```

In `api/internal/api/v1/helpers.go` (`bindAV`, line 159) — replace the direct `attributevalue.MarshalMap`:

```go
func bindAV(c fiber.Ctx) (map[string]types.AttributeValue, error) {
	var body map[string]any
	if err := c.Bind().JSON(&body); err != nil {
		return nil, err
	}
	return repositories.MarshalMapOmitNull(body)
}
```

(Remove the now-unused `attributevalue` import from helpers.go if nothing else uses it; `go build` will flag it.)

In `api/internal/api/v1/organizations.go` (line 72) — replace `attributevalue.MarshalMap(body)` with
`repositories.MarshalMapOmitNull(body)` (drop the `attributevalue` import if unused).

- [ ] **Step 5: Run test + build**

Run: `cd api && go test ./internal/repositories/ -run TestMarshalMapOmitNull -v && go build ./...`
Expected: PASS, build OK (no unused-import errors).

- [ ] **Step 6: Commit**

```bash
git add api/internal/repositories/marshal.go api/internal/repositories/marshal_test.go api/internal/repositories/base.go api/internal/repositories/documents.go api/internal/api/v1/helpers.go api/internal/api/v1/organizations.go
git commit -m "feat(api): omit null attributes when encoding items for DynamoDB"
```

---

## Task 4: UpdateItem REMOVE-on-nil (api)

**Files:**

- Modify: `api/internal/repositories/base.go` (`UpdateItem`, lines 96-135)
- Test: `api/internal/repositories/base_test.go`

**Interfaces:**

- Produces:
  `buildUpdateExpr(updates map[string]any) (expr string, names map[string]string, values map[string]types.AttributeValue, err error)` —
  nil values become `REMOVE`, non-nil become `SET`.

- [ ] **Step 1: Write the failing test**

Create/append `api/internal/repositories/base_test.go`:

```go
package repositories

import (
	"strings"
	"testing"
)

func TestBuildUpdateExpr_SetAndRemove(t *testing.T) {
	expr, names, values, err := buildUpdateExpr(map[string]any{
		"name": "X",
		"cest": nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(expr, "SET #name = :name") {
		t.Errorf("expected SET clause for name, got %q", expr)
	}
	if !strings.Contains(expr, "REMOVE #cest") {
		t.Errorf("expected REMOVE clause for cest, got %q", expr)
	}
	if _, ok := values[":cest"]; ok {
		t.Errorf("nil value must not be in ExpressionAttributeValues")
	}
	if names["#cest"] != "cest" {
		t.Errorf("expected name mapping for cest")
	}
}

func TestBuildUpdateExpr_RemoveOnly(t *testing.T) {
	expr, _, values, err := buildUpdateExpr(map[string]any{"cest": nil})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(expr, "SET") {
		t.Errorf("expected no SET clause, got %q", expr)
	}
	if !strings.HasPrefix(expr, "REMOVE") {
		t.Errorf("expected REMOVE-only expression, got %q", expr)
	}
	if len(values) != 0 {
		t.Errorf("expected no expression values, got %d", len(values))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd api && go test ./internal/repositories/ -run TestBuildUpdateExpr -v`
Expected: FAIL — `buildUpdateExpr` undefined.

- [ ] **Step 3: Extract `buildUpdateExpr` and wire it into `UpdateItem`**

In `base.go`, add the helper (above `UpdateItem`):

```go
// buildUpdateExpr builds a combined SET/REMOVE update expression. Nil values
// become REMOVE clauses (clearing the attribute without storing a NULL);
// non-nil values become SET clauses.
func buildUpdateExpr(updates map[string]any) (string, map[string]string, map[string]types.AttributeValue, error) {
	setParts := make([]string, 0, len(updates))
	removeParts := make([]string, 0)
	exprNames := make(map[string]string, len(updates))
	exprValues := make(map[string]types.AttributeValue)

	for attr, val := range updates {
		exprNames["#"+attr] = attr
		if val == nil {
			removeParts = append(removeParts, "#"+attr)
			continue
		}
		av, err := attributevalue.Marshal(val)
		if err != nil {
			return "", nil, nil, err
		}
		setParts = append(setParts, fmt.Sprintf("#%s = :%s", attr, attr))
		exprValues[":"+attr] = av
	}

	clauses := make([]string, 0, 2)
	if len(setParts) > 0 {
		clauses = append(clauses, "SET "+strings.Join(setParts, ", "))
	}
	if len(removeParts) > 0 {
		clauses = append(clauses, "REMOVE "+strings.Join(removeParts, ", "))
	}
	return strings.Join(clauses, " "), exprNames, exprValues, nil
}
```

Replace the body of `UpdateItem` (the manual `setParts`/`expr` building, lines 105-134) so it calls the helper and only
passes `ExpressionAttributeValues` when non-empty:

```go
func (b *Base) UpdateItem(ctx context.Context, pk string, sk *string, updates map[string]any) (bool, error) {
	key := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: pk},
	}
	if sk != nil {
		key["sk"] = &types.AttributeValueMemberS{Value: *sk}
	}

	expr, exprNames, exprValues, err := buildUpdateExpr(updates)
	if err != nil {
		return false, err
	}

	input := &dynamodb.UpdateItemInput{
		TableName:                aws.String(b.TableName),
		Key:                      key,
		UpdateExpression:         aws.String(expr),
		ExpressionAttributeNames: exprNames,
		ConditionExpression:      aws.String("attribute_exists(pk)"),
	}
	if len(exprValues) > 0 {
		input.ExpressionAttributeValues = exprValues
	}

	_, err = b.db.UpdateItem(ctx, input)
	if err != nil {
		if isConditionFailed(err) {
```

(Keep the existing post-`UpdateItem` error/return tail below line 134 unchanged.)

- [ ] **Step 4: Run test + build**

Run: `cd api && go test ./internal/repositories/ -run TestBuildUpdateExpr -v && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/base.go api/internal/repositories/base_test.go
git commit -m "feat(api): clear fields via REMOVE instead of storing NULL on UpdateItem"
```

---

## Task 5: Drop explicit NULL writes (api)

**Files:**

- Modify: `api/internal/repositories/dfe_events.go` (`setNullableStr`, lines 73-79)
- Modify: `api/internal/repositories/persons.go` (`Create`, lines 30-37)
- Modify: `api/internal/repositories/fiscal_config.go` (preserve defaults, lines 48-60)

**Interfaces:**

- Consumes: nothing new. Behavioral change only: nil → attribute omitted.

- [ ] **Step 1: `setNullableStr` — omit instead of NULL**

In `dfe_events.go` (lines 73-79):

```go
func setNullableStr(item map[string]types.AttributeValue, key string, val *string) {
	if val != nil {
		item[key] = &types.AttributeValueMemberS{Value: *val}
	}
	// nil → omit the attribute (no NULL stored)
}
```

- [ ] **Step 2: `persons.go` Create — skip nil fields**

In `persons.go` (the `for k, v := range fields` loop, lines 30-37):

```go
	for k, v := range fields {
		if _, exists := item[k]; exists {
			continue
		}
		if v == nil {
			continue // omit null attributes
		}
		av, err := attributevalue.Marshal(v)
		if err == nil {
			item[k] = av
		}
	}
```

- [ ] **Step 3: `fiscal_config.go` — skip nil defaults**

In `fiscal_config.go` (lines 56-59):

```go
		// set default if not yet present
		if _, alreadySet := fields[field]; !alreadySet {
			if defVal == nil {
				continue // omit null defaults
			}
			av, _ := attributevalue.Marshal(defVal)
			fields[field] = av
		}
```

- [ ] **Step 4: Build + existing repo tests**

Run: `cd api && go build ./... && go test ./internal/repositories/...`
Expected: build OK; tests pass.

- [ ] **Step 5: Commit**

```bash
git add api/internal/repositories/dfe_events.go api/internal/repositories/persons.go api/internal/repositories/fiscal_config.go
git commit -m "refactor(api): omit attributes instead of writing NULL to DynamoDB"
```

---

## Task 6: Worker mapToAttr omit-null

**Files:**

- Modify: `worker/internal/service/distribution.go` (`mapToAttr`, lines 1219-1228)
- Test: `worker/internal/service/distribution_maptoattr_test.go` (new)

**Interfaces:**

- Consumes: existing `toAttr` (distribution.go:1230). Behavioral change: `mapToAttr` skips keys whose value is `nil`.

- [ ] **Step 1: Write the failing test**

Create `worker/internal/service/distribution_maptoattr_test.go`:

```go
package service

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestMapToAttr_OmitsNil(t *testing.T) {
	av := mapToAttr(map[string]any{
		"name": "X",
		"cest": nil,
	})
	if _, ok := av.Value["cest"]; ok {
		t.Errorf("expected nil 'cest' to be omitted, got %#v", av.Value["cest"])
	}
	if _, ok := av.Value["name"]; !ok {
		t.Errorf("expected 'name' to be present")
	}
	_ = types.AttributeValueMemberNULL{} // ensure types import used
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd worker && go test ./internal/service/ -run TestMapToAttr_OmitsNil -v`
Expected: FAIL — `cest` present as NULL.

- [ ] **Step 3: Skip nil in `mapToAttr`**

In `distribution.go` (lines 1221-1228):

```go
func mapToAttr(m map[string]any) *types.AttributeValueMemberM {
	out := make(map[string]types.AttributeValue, len(m))
	for k, v := range m {
		if v == nil {
			continue // omit null attributes (reduce stored item size)
		}
		out[k] = toAttr(v)
	}
	return &types.AttributeValueMemberM{Value: out}
}
```

- [ ] **Step 4: Run test + build**

Run: `cd worker && go test ./internal/service/ -run TestMapToAttr_OmitsNil -v && go build ./...`
Expected: PASS, build OK.

- [ ] **Step 5: Commit**

```bash
git add worker/internal/service/distribution.go worker/internal/service/distribution_maptoattr_test.go
git commit -m "feat(worker): omit null attributes when building DynamoDB items"
```

---

## Task 7: UI stripNulls in ApiClient

**Files:**

- Create: `src/lib/utils/strip-nulls.ts`
- Modify: `src/lib/api/client.ts` (request interceptor, lines 74-89)
- Test: `src/__tests__/lib/strip-nulls.test.ts`

**Interfaces:**

- Produces: `stripNulls<T>(value: T, dropNull: boolean): T` — deep-removes `undefined` always; removes `null` only when
  `dropNull` is true; preserves `0`, `''`, `false`, arrays.

- [ ] **Step 1: Write the failing test**

Create `src/__tests__/lib/strip-nulls.test.ts`:

```ts
import {describe, it, expect} from 'vitest'
import {stripNulls} from '@/lib/utils/strip-nulls'

describe('stripNulls', () => {
  it('drops null and undefined when dropNull=true', () => {
    expect(stripNulls({a: 1, b: null, c: undefined, d: ''}, true)).toEqual({a: 1, d: ''})
  })
  it('keeps null but drops undefined when dropNull=false', () => {
    expect(stripNulls({a: 1, b: null, c: undefined}, false)).toEqual({a: 1, b: null})
  })
  it('recurses into nested objects and arrays', () => {
    expect(stripNulls({a: {b: null, c: 2}, list: [{x: null, y: 3}]}, true))
      .toEqual({a: {c: 2}, list: [{y: 3}]})
  })
  it('preserves falsy non-null values', () => {
    expect(stripNulls({a: 0, b: false, c: ''}, true)).toEqual({a: 0, b: false, c: ''})
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ui && npx vitest run src/__tests__/lib/strip-nulls.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement `stripNulls`**

Create `src/lib/utils/strip-nulls.ts`:

```ts
/**
 * Deep-removes `undefined` from a value always, and `null` when `dropNull` is
 * true. Used to shrink request payloads: drop nulls on create (POST), keep them
 * on update (PUT/PATCH) where an explicit null means "clear this field".
 * Preserves falsy non-null values (0, '', false) and array element order.
 */
export function stripNulls<T>(value: T, dropNull: boolean): T {
  if (Array.isArray(value)) {
    return value.map((v) => stripNulls(v, dropNull)) as unknown as T
  }
  if (value !== null && typeof value === 'object') {
    const out: Record<string, unknown> = {}
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (v === undefined) continue
      if (v === null && dropNull) continue
      out[k] = stripNulls(v, dropNull)
    }
    return out as T
  }
  return value
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd ui && npx vitest run src/__tests__/lib/strip-nulls.test.ts`
Expected: PASS.

- [ ] **Step 5: Apply in the request interceptor**

In `src/lib/api/client.ts`, add the import at the top:

```ts
import {stripNulls} from '@/lib/utils/strip-nulls'
```

In the request interceptor (after the org-header block, before `return config`, ~line 88):

```ts
    if (config.data && typeof config.data === 'object') {
      const method = (config.method ?? 'get').toLowerCase()
      const dropNull = method === 'post' // create: no clear semantics; updates keep null
      config.data = stripNulls(config.data, dropNull)
    }
    return config
```

- [ ] **Step 6: Add the ApiClient interceptor test**

Append to an appropriate client test (or create `src/__tests__/lib/client-stripnulls.test.ts`). Confirm the existing
test harness for `ApiClient`/axios first; mirror its mock setup. Minimum assertion:

```ts
import {describe, it, expect} from 'vitest'
import {stripNulls} from '@/lib/utils/strip-nulls'

describe('request payload stripping policy', () => {
  it('POST drops nulls', () => {
    expect(stripNulls({cest: null, name: 'x'}, true)).toEqual({name: 'x'})
  })
  it('PUT keeps explicit null (field clear)', () => {
    expect(stripNulls({cest: null, name: 'x'}, false)).toEqual({cest: null, name: 'x'})
  })
})
```

- [ ] **Step 7: Lint + full test sweep**

Run: `cd ui && npx eslint src --ext .ts,.tsx && npm test`
Expected: zero eslint errors/warnings; all tests pass.

- [ ] **Step 8: Commit**

```bash
git add ui/src/lib/utils/strip-nulls.ts ui/src/lib/api/client.ts ui/src/__tests__/lib/strip-nulls.test.ts ui/src/__tests__/lib/client-stripnulls.test.ts
git commit -m "feat(ui): strip null fields from create payloads in ApiClient"
```

---

## Task 8: Documentation

**Files:**

- Modify: `DOCS.md`, `INTEGRATION.md`, `CONDUCT.md`

- [ ] **Step 1: DOCS.md / INTEGRATION.md — storage vs contract**

Add a note (DOCS.md data-layer section and INTEGRATION.md): "DynamoDB items omit null attributes to reduce item size.
The API contract remains nullable — responses still emit `null` for absent attributes, and clients clear a field by
sending `null` (persisted as a DynamoDB `REMOVE` on updates). CFOP: the NF-e emit form groups same-suffix saída CFOPs
and sends the concrete intra (5xxx) / inter (6xxx) variant resolved from the destinatário's UF."

- [ ] **Step 2: CONDUCT.md — new constraint**

Add: "Do not write `NULL` attributes to DynamoDB. Encode items via `repositories.MarshalMapOmitNull` (or `Encode`/
`EncodeItem`, which delegate to it); clear fields on update via `REMOVE` (handled in `Base.UpdateItem`). The worker's
`mapToAttr` skips nil values. UI strips null fields from POST payloads only."

- [ ] **Step 3: Commit**

```bash
git add DOCS.md INTEGRATION.md CONDUCT.md
git commit -m "docs: document null-omission storage policy and CFOP UF resolution"
```

---

## Self-Review Notes

- **Spec coverage:** A (helpers→T1, form→T2), B-api (encoder→T3, UpdateItem→T4, explicit NULLs→T5), B-worker (T6), B-ui
  (T7), docs (T8). All spec sections mapped.
- **Type consistency:** `CfopSuffixGroup`/`resolveCfopForUf`/`groupCfopConfigBySuffix` names identical across T1/T2.
  `MarshalMapOmitNull` consistent across T3 and CONDUCT note. `buildUpdateExpr` signature consistent T4.
- **Known verification points flagged inline:** `ProductRow` export for testing (T2 S7); `attributevalue` import removal
  after T3 edits; existing ApiClient test harness shape (T7 S6); `OptionsSelect` portal in component test (T2 S8).
