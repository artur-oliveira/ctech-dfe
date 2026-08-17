'use client'

import {formatCents, METER_LABELS, PLAN_ONDEMAND, QUOTA_UNLIMITED} from '@/lib/constants/billing'
import {grantedMeters, type PlanOption} from '@/lib/billing/catalog'

interface PlanChooserProps {
  options: PlanOption[]
  value: string | null
  onChange: (productId: string) => void
  /** The plan already in force, so the current one is not offered as a change. */
  currentPlan?: string
}

function quotaText(limit: number): string {
  return limit === QUOTA_UNLIMITED ? 'ilimitado' : limit.toLocaleString('pt-BR')
}

/** The price line: a monthly amount, "grátis", or per-document pricing. */
function priceLabel(option: PlanOption): {amount: string; unit: string} {
  if (option.plan === PLAN_ONDEMAND) return {amount: 'Por uso', unit: 'sem mensalidade'}
  if (option.monthlyCents === 0) return {amount: 'Grátis', unit: 'para sempre'}
  return {amount: formatCents(option.monthlyCents), unit: 'por mês'}
}

/**
 * The plan comparison, built from the billing catalogue.
 *
 * Rows rather than a wall of cards: at 375px three columns become three cards
 * stacked anyway, and comparing prices down a single edge is what the decision
 * actually needs. The selected plan opens to its full allowance; the others stay
 * on one summary line so the list keeps fitting on a phone.
 */
export function PlanChooser({options, value, onChange, currentPlan}: PlanChooserProps) {
  return (
    <div role="radiogroup" aria-label="Planos" className="flex flex-col gap-3">
      {options.map((option) => {
        const selected = value === option.productId
        const isCurrent = currentPlan === option.plan
        const price = priceLabel(option)
        const meters = grantedMeters(option.quotas)

        return (
          <div
            key={option.productId}
            role="radio"
            aria-checked={selected}
            tabIndex={0}
            onClick={() => onChange(option.productId)}
            onKeyDown={(e) => {
              if (e.key === ' ' || e.key === 'Enter') {
                e.preventDefault()
                onChange(option.productId)
              }
            }}
            className={`cursor-pointer rounded-xl border bg-white p-4 transition-all outline-none focus-visible:ring-3 focus-visible:ring-brand-200 md:p-5 ${
              selected
                ? 'border-brand-600 shadow-card-hover ring-1 ring-brand-600'
                : 'border-gray-200 hover:border-gray-300 hover:shadow-card'
            }`}
          >
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-base font-semibold text-gray-900">{option.name}</span>
                  {option.recommended && !isCurrent && (
                    <span className="rounded-md bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">
                      Mais escolhido
                    </span>
                  )}
                  {isCurrent && (
                    <span className="rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                      Plano atual
                    </span>
                  )}
                </div>
                <p className="mt-1 text-sm leading-relaxed text-gray-600 text-pretty">{option.tagline}</p>
              </div>
              <div className="shrink-0 text-right">
                <p className="text-base font-semibold text-gray-900 tabular-nums">{price.amount}</p>
                <p className="text-xs text-gray-500">{price.unit}</p>
              </div>
            </div>

            {selected ? (
              <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-2 border-t border-gray-100 pt-4">
                {meters.map((meter) => (
                  <div key={meter} className="flex items-baseline justify-between gap-2">
                    <dt className="text-sm text-gray-600">{METER_LABELS[meter] ?? meter}</dt>
                    <dd className="text-sm font-medium text-gray-900 tabular-nums">
                      {quotaText(option.quotas[meter])}
                    </dd>
                  </div>
                ))}
                {option.metered.map((m) => (
                  <div key={m.meter} className="flex items-baseline justify-between gap-2">
                    <dt className="text-sm text-gray-600">{METER_LABELS[m.meter] ?? m.meter}</dt>
                    <dd className="text-sm font-medium text-gray-900 tabular-nums">
                      {formatCents(m.unitAmount)}
                      <span className="font-normal text-gray-500"> /doc</span>
                    </dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="mt-3 text-xs text-gray-500">
                {meters.length > 0
                  ? meters
                      .slice(0, 3)
                      .map((m) => `${quotaText(option.quotas[m])} ${METER_LABELS[m] ?? m}`)
                      .join(' · ')
                  : option.metered.map((m) => METER_LABELS[m.meter] ?? m.meter).join(' · ')}
              </p>
            )}
          </div>
        )
      })}
    </div>
  )
}
