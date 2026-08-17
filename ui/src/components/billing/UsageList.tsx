'use client'

import {METER_LABELS, QUOTA_UNLIMITED} from '@/lib/constants/billing'
import {grantedMeters} from '@/lib/billing/catalog'
import type {MeterUsage} from '@/lib/types/billing'

interface UsageListProps {
  quotas: Record<string, number>
  /** Absent on the organization-scoped view, which publishes limits only. */
  usage?: Record<string, MeterUsage>
}

/** Where a bar stops warning and starts alarming. */
const NEAR_LIMIT_RATIO = 0.8

function barColor(ratio: number): string {
  if (ratio >= 1) return 'bg-danger'
  if (ratio >= NEAR_LIMIT_RATIO) return 'bg-amber-500'
  return 'bg-brand-600'
}

/**
 * What the plan allows and what is left of it.
 *
 * An unlimited meter gets no bar: a full-width bar next to "ilimitado" reads as
 * "you are at your limit", which is the opposite of what it means.
 */
export function UsageList({quotas, usage}: UsageListProps) {
  const meters = grantedMeters(quotas)
  if (meters.length === 0) {
    return <p className="text-sm text-gray-500">Este plano não inclui nenhum documento.</p>
  }

  return (
    <ul className="flex flex-col gap-4">
      {meters.map((meter) => {
        const limit = quotas[meter]
        const used = usage?.[meter]?.used
        const unlimited = limit === QUOTA_UNLIMITED
        const ratio = unlimited || used == null ? 0 : Math.min(used / limit, 1)

        return (
          <li key={meter}>
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-sm text-gray-700">{METER_LABELS[meter] ?? meter}</span>
              <span className="text-sm text-gray-900 tabular-nums">
                {used != null && <span className="font-medium">{used.toLocaleString('pt-BR')}</span>}
                {used != null && <span className="text-gray-400"> / </span>}
                <span className={used == null ? 'font-medium' : 'text-gray-500'}>
                  {unlimited ? 'ilimitado' : limit.toLocaleString('pt-BR')}
                </span>
              </span>
            </div>
            {!unlimited && used != null && (
              <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-gray-100">
                <div
                  className={`h-full rounded-full ${barColor(used / limit)}`}
                  style={{width: `${Math.round(ratio * 100)}%`}}
                />
              </div>
            )}
          </li>
        )
      })}
    </ul>
  )
}
