# Product Price Segmentation — Revenda vs Consumidor Final

**Date:** 2026-06-22
**Scope:** `ui/` only (backend products are schema-less; no api/worker/py-dfe code change)

## Problem

Product registration has a single price field (`value`, "Preço de venda"). Need to segment
into two prices:

- **Preço (Revenda)** — resale price (B2B).
- **Preço (Consumidor Final)** — final-consumer price (B2C).

This changes the default unit price prefilled in the emission screens:

- **NFC-e:** always use the consumer-final price.
- **NF-e:** use consumer-final price if recipient is a CPF; use resale price if recipient is a
  CNPJ.

## Decisions

1. **Migration:** Keep existing `value` field as-is, semantically reinterpreted as
   **Preço (Consumidor Final)**. Add a new optional `value_resale` field. Zero data migration;
   existing products keep working.
2. **Resale fallback:** `value_resale` is optional. When empty, NF-e to a CNPJ falls back to
   `value` (consumer price). No emission ever breaks for lack of a resale price.
3. **Recipient switch:** Unit price is chosen at product-add time based on the current recipient.
   Switching the recipient (CPF↔CNPJ) after items are added does NOT re-price existing items.
4. **Self-issuance NF-e:** treated as CNPJ → resale price (with consumer fallback).

## Field Model

| Field          | Meaning                  | Required | Format                     |
|----------------|--------------------------|----------|----------------------------|
| `value`        | Preço (Consumidor Final) | yes      | `/^\d+(\.\d{1,4})?$/`      |
| `value_resale` | Preço (Revenda)          | no       | `/^\d+(\.\d{1,4})?$/`      |

Backend (`api/internal/api/v1/products.go`) stores products as a dynamic DynamoDB map via
`bindAV` (POST) and pass-through `map[string]any` (PUT). No server-side schema validates price,
so no backend code change is required — the new field is persisted automatically. Worker/py-dfe
emit the price carried in the emission request's `unit_value`, not the product record, so they
are unaffected.

## Price Selection Helper (DRY)

New pure function, single source of truth, placed in `ui/src/lib/data/product-price.ts`:

```ts
// recipientDoc: unformatted CPF (11) or CNPJ (14) digits, or '' when unknown.
export function resolveUnitPrice(
  product: Pick<ProductOut, 'value' | 'value_resale'>,
  recipientDoc: string,
): string {
  const digits = recipientDoc.replace(/\D/g, '')
  const isCnpj = digits.length === 14
  if (isCnpj && product.value_resale) return product.value_resale
  return product.value
}
```

## UI Changes

1. **`lib/schemas/products.ts`** — add
   `value_resale: nullableStr(z.string().regex(/^\d+(\.\d{1,4})?$/, 'Valor inválido (ex: 99.90)'))`.
   Relabel of `value` is UI-text only (schema field name unchanged).
2. **`lib/types/api.ts`** — add `value_resale?: string` to `ProductOut`, `ProductCreate`,
   `ProductUpdate`.
3. **`components/products/ProductForm.tsx`** —
   - Relabel the `value` field label "Preço de venda *" → "Preço (Consumidor Final) *".
   - Add a "Preço (Revenda)" `CurrencyInput` bound to `value_resale`.
   - Wire `value_resale` into form reset (mapping from `ProductOut`, ~line 202) and submit
     payload (~line 325) and default values (~line 484).
4. **`components/nfce/NfceEmitForm.tsx`** — line 429: `unitValue: product.value` stays
   (consumer-final price); now semantically correct. No logic change.
5. **`components/nfe/NfeEmitForm.tsx`** — line 940: replace `unitValue: product.value` with
   `unitValue: resolveUnitPrice(product, recipientDoc)`, where `recipientDoc` is the org CNPJ
   when `selfIssuance`, else `unformatCpfCnpj(receiver.sk)`.

## Testing

| Target               | Test                                                                |
|----------------------|---------------------------------------------------------------------|
| `productSchema`      | Unit (Zod): `value_resale` optional, accepts valid, rejects bad     |
| `resolveUnitPrice`   | Unit: CPF→consumer, CNPJ→resale, CNPJ w/o resale→consumer fallback, unknown→consumer |

ESLint must pass with zero errors/warnings. `npm test` must pass.

## Out of Scope

- No backend (api/worker/py-dfe) code change.
- No re-pricing of items already added when recipient changes.
- No backfill migration of existing product records.
