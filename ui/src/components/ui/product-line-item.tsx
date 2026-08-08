'use client'

import type {ReactNode} from 'react'
import {Button} from '@/components/ui/button'
import {CurrencyInput} from '@/components/ui/currency-input'
import {Label} from '@/components/ui/label'
import {NumericInput} from '@/components/ui/numeric-input'

export interface ProductLineValues {
  qty: string
  unitValue: string
  discount: string
}

interface ProductLineItemProps extends ProductLineValues {
  /** Unique per row — namespaces the field ids for label association. */
  idPrefix: string
  description: string
  brand?: string | null
  unit?: string | null
  /** Document-specific tags (veículo, armamento, …). */
  badges?: ReactNode
  /** Document-specific CFOP control, including its own label and errors. */
  cfopSlot: ReactNode
  /** Document-specific per-unit fields rendered below the grid. */
  children?: ReactNode
  total: number
  onChange: (patch: Partial<ProductLineValues>) => void
  onRemove: () => void
}

function fmt(n: number): string {
  return n.toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}

/**
 * One product line in an emit form — shared by NF-e and NFC-e so the quantity,
 * price and discount controls behave identically in both documents. Everything
 * that genuinely differs between documents arrives through `cfopSlot`,
 * `badges` and `children`.
 */
export function ProductLineItem({
  idPrefix,
  description,
  brand,
  unit,
  badges,
  cfopSlot,
  children,
  qty,
  unitValue,
  discount,
  total,
  onChange,
  onRemove,
}: ProductLineItemProps) {
  const step = (delta: number) => onChange({qty: String(Math.max(0, (parseFloat(qty) || 0) + delta))})

  return (
    <div className="rounded-lg border border-gray-200 bg-white p-3 md:p-4 space-y-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="font-medium text-gray-900 text-sm">
            {description}
            {brand && <span className="ml-1.5 text-xs text-gray-500 font-normal">{brand}</span>}
          </p>
          {badges && <div className="flex flex-wrap items-center gap-1 mt-0.5">{badges}</div>}
        </div>
        <Button type="button" variant="ghost" size="xs" onClick={onRemove}
                className="shrink-0 text-danger hover:text-red-700">
          Remover
        </Button>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-12 gap-2 items-end">
        <div className="col-span-2 md:col-span-6 flex flex-col gap-1">{cfopSlot}</div>

        <div className="col-span-2 md:col-span-2 flex flex-col gap-1">
          <Label htmlFor={`${idPrefix}-qty`} className="text-xs font-medium text-gray-600">
            Qtd ({unit ?? 'UN'})
          </Label>
          <div className="flex items-center">
            <button type="button" aria-label="Diminuir quantidade" onClick={() => step(-1)}
                    className="h-11 w-11 sm:h-8 sm:w-7 shrink-0 flex items-center justify-center rounded-l-lg border border-r-0 border-input bg-muted/30 text-gray-600 hover:bg-muted/60 font-medium select-none text-sm">
              −
            </button>
            <NumericInput id={`${idPrefix}-qty`} decimal integerPlaces={7} decimalPlaces={4} value={qty}
                          onChange={(v) => onChange({qty: v})} placeholder="1"
                          className="rounded-none border-x-0 text-center h-11 sm:h-8"/>
            <button type="button" aria-label="Aumentar quantidade" onClick={() => step(1)}
                    className="h-11 w-11 sm:h-8 sm:w-7 shrink-0 flex items-center justify-center rounded-r-lg border border-l-0 border-input bg-muted/30 text-gray-600 hover:bg-muted/60 font-medium select-none text-sm">
              +
            </button>
          </div>
        </div>

        <div className="col-span-1 md:col-span-2 flex flex-col gap-1">
          <Label htmlFor={`${idPrefix}-unit-value`} className="text-xs font-medium text-gray-600">Valor unitário</Label>
          <CurrencyInput id={`${idPrefix}-unit-value`} decimalPlaces={2} maxDecimalPlaces={10} value={unitValue}
                         onChange={(v) => onChange({unitValue: v})} placeholder="0,00"/>
        </div>

        <div className="col-span-1 md:col-span-2 flex flex-col gap-1">
          <Label htmlFor={`${idPrefix}-discount`} className="text-xs font-medium text-gray-600">Desconto</Label>
          <CurrencyInput id={`${idPrefix}-discount`} decimalPlaces={2} value={discount}
                         onChange={(v) => onChange({discount: v})} placeholder="0,00"/>
        </div>
      </div>

      {children}

      <div className="text-right text-sm font-medium text-gray-700">
        Total: <span className="font-semibold">{fmt(total)}</span>
      </div>
    </div>
  )
}
