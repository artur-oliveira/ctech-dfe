import {
  ACCOUNT_METERS,
  DOCUMENT_METERS,
  META_METER,
  META_PLAN,
  META_QUOTA_PREFIX,
  META_VISIBILITY,
  PLAN_PRESENTATION,
  VISIBILITY_INTERNAL,
} from '@/lib/constants/billing'
import type {BillingPrice, BillingProduct} from '@/lib/types/billing'

/** A plan as the chooser shows it: one product, all of its prices. */
export interface PlanOption {
  /** `free`, `ondemand`, `pro`, `unlimited` — from the price metadata. */
  plan: string
  productId: string
  /** Billing's own name for the product. Never a local copy. */
  name: string
  tagline: string
  recommended: boolean
  /** Everything the subscription is composed of. */
  priceIds: string[]
  /** Cents billed up front every month; 0 for a purely metered plan. */
  monthlyCents: number
  /** Per-document prices, for the plans that charge by use. */
  metered: { meter: string; unitAmount: number }[]
  /** Limit per meter; -1 unlimited, absent means the plan does not grant it. */
  quotas: Record<string, number>
}

function isOffered(price: BillingPrice): boolean {
  return !price.archived && price.metadata[META_VISIBILITY] !== VISIBILITY_INTERNAL
}

/**
 * Turns the billing catalogue into the plans a person can choose.
 *
 * Everything shown on the chooser is derived here — never from a local plan
 * list. Two price lists that nobody compares is how a site advertises R$ 350 and
 * the invoice charges R$ 400.
 */
export function buildPlanOptions(products: BillingProduct[]): PlanOption[] {
  const options: PlanOption[] = []

  for (const product of products) {
    if (!product.active) continue
    const prices = product.prices.filter(isOffered)
    if (prices.length === 0) continue

    const plan = prices.find((p) => p.metadata[META_PLAN])?.metadata[META_PLAN] ?? ''
    if (!plan) continue

    const quotas: Record<string, number> = {}
    const metered: PlanOption['metered'] = []
    let monthlyCents = 0

    for (const price of prices) {
      for (const [key, value] of Object.entries(price.metadata)) {
        if (!key.startsWith(META_QUOTA_PREFIX)) continue
        const parsed = Number(value)
        // An unreadable quota is not a quota of zero — dropping it shows
        // "não incluído", which is the honest reading of a broken value.
        if (Number.isFinite(parsed)) quotas[key.slice(META_QUOTA_PREFIX.length)] = parsed
      }
      if (price.type === 'metered') {
        const meter = price.metadata[META_METER]
        if (meter) metered.push({meter, unitAmount: price.unit_amount})
      } else {
        monthlyCents += price.unit_amount
      }
    }

    const presentation = PLAN_PRESENTATION[plan]
    options.push({
      plan,
      productId: product.id,
      name: product.name,
      tagline: presentation?.tagline ?? '',
      recommended: presentation?.recommended ?? false,
      priceIds: prices.map((p) => p.id),
      monthlyCents,
      metered,
      quotas,
    })
  }

  return options.sort(
    (a, b) => (PLAN_PRESENTATION[a.plan]?.order ?? 99) - (PLAN_PRESENTATION[b.plan]?.order ?? 99),
  )
}

/** The meters a plan grants, in the order the screens list them. */
export function grantedMeters(quotas: Record<string, number>): string[] {
  if (!quotas) {
    return [];
  }
  return [...DOCUMENT_METERS, ...ACCOUNT_METERS].filter((m) => quotas[m] !== undefined && quotas[m] !== 0)
}
