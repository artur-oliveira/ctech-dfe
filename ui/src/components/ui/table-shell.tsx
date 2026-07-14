import {cn} from '@/lib/utils'
import React from "react";

export type TableColumn =
    | string
    | {
    label: string
    align?: 'left' | 'right' | 'center'
    /** Extra classes for this header cell (e.g. a fixed width). */
    className?: string
    /** Custom header content (e.g. a select-all checkbox). `label` is still the key. */
    node?: React.ReactNode
}

/**
 * Standard data-table shell: scroll wrapper + <table> + unified header.
 *
 * Centralizes the container (rounded-xl border, horizontal scroll), the header
 * styling (gray-50 fill, uppercase 12px labels), the row divider, and the
 * accessible name — the parts that had drifted across ~14 hand-rolled tables.
 * Pass column markup as `<tr>`/`<td>` children (the <tbody> rows); use
 * `TABLE_ROW` / `TABLE_CELL` for the standard row hover and cell padding.
 *
 * `dimmed` shows the background-refetch state (opacity fade) mandated for
 * already-loaded lists.
 *
 * Mobile: below `sm` the table collapses to a stacked card per row (the
 * `.ts-mobile` block in globals.css). Add `data-label="<column name>"` to each
 * body <td> so its column name shows beside the value in the card. Control /
 * action cells (select checkbox, edit/delete cluster) omit `data-label` and are
 * right-aligned by the stylesheet.
 */
export function TableShell({
                               headers,
                               minWidth,
                               ariaLabel,
                               dimmed,
                               className,
                               children,
                           }: {
    headers: TableColumn[]
    /** Minimum table width in px before horizontal scroll kicks in. */
    minWidth?: number
    ariaLabel: string
    dimmed?: boolean
    className?: string
    children: React.ReactNode
}) {
    return (
        <div
            className={cn(
                'overflow-hidden overflow-x-auto rounded-xl border border-gray-200 bg-white',
                dimmed && 'opacity-60 transition-opacity',
                className,
            )}
        >
            <table
                aria-label={ariaLabel}
                className="ts-mobile w-full text-sm"
                style={minWidth ? {minWidth: `${minWidth}px`} : undefined}
            >
                <thead className="border-b border-gray-200 bg-gray-50">
                <tr>
                    {headers.map((h, i) => {
                        const col = typeof h === 'string' ? {
                            label: h,
                            align: 'left' as const,
                            className: undefined,
                            node: undefined
                        } : h
                        return (
                            <th
                                key={col.label || `col-${i}`}
                                className={cn(
                                    'px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-500',
                                    col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left',
                                    col.className,
                                )}
                            >
                                {col.node ?? col.label}
                            </th>
                        )
                    })}
                </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">{children}</tbody>
            </table>
        </div>
    )
}

/** Standard hover row for TableShell bodies. */
export const TABLE_ROW = 'hover:bg-gray-50 transition-colors'
/** Standard cell padding for TableShell bodies. */
export const TABLE_CELL = 'px-4 py-3'

/**
 * Row-select checkbox for a table cell. `indeterminate` renders the mixed state
 * (used in the header select-all). 44px touch target via padding on the cell.
 */
export function RowCheckbox({
  checked,
  indeterminate,
  onChange,
  ariaLabel,
}: {
  checked: boolean
  indeterminate?: boolean
  onChange: () => void
  ariaLabel: string
}) {
  return (
    <input
      type="checkbox"
      checked={checked}
      aria-label={ariaLabel}
      ref={el => {
        if (el) el.indeterminate = !!indeterminate && !checked
      }}
      onChange={onChange}
      className="size-4 cursor-pointer rounded border-gray-300 text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    />
  )
}
