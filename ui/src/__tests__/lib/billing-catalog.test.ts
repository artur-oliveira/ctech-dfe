import {describe, expect, it} from 'vitest'
import {buildPlanOptions, grantedMeters} from '@/lib/billing/catalog'
import type {BillingProduct} from '@/lib/types/billing'

function price(overrides: Partial<BillingProduct['prices'][number]> = {}) {
  return {
    id: 'price_x',
    product_id: 'prod_x',
    type: 'fixed' as const,
    unit_amount: 0,
    billing_timing: 'advance' as const,
    archived: false,
    metadata: {},
    ...overrides,
  }
}

describe('buildPlanOptions', () => {
  it('hides the internal plan — offering it would give away an unlimited subscription', () => {
    const products: BillingProduct[] = [
      {
        id: 'prod_dfe_unlimited_internal',
        name: 'DF-e Ilimitado - Interno',
        active: true,
        prices: [price({id: 'p_internal', metadata: {plan: 'unlimited', visibility: 'internal'}})],
      },
    ]
    expect(buildPlanOptions(products)).toEqual([])
  })

  it('drops archived prices and products billing has deactivated', () => {
    const products: BillingProduct[] = [
      {
        id: 'prod_old',
        name: 'Antigo',
        active: false,
        prices: [price({metadata: {plan: 'pro'}})],
      },
      {
        id: 'prod_free',
        name: 'DF-e Free',
        active: true,
        prices: [price({id: 'p_old', archived: true, metadata: {plan: 'free'}})],
      },
    ]
    expect(buildPlanOptions(products)).toEqual([])
  })

  it('reads the quotas out of price metadata and keeps -1 as unlimited', () => {
    const products: BillingProduct[] = [
      {
        id: 'prod_dfe_free',
        name: 'DF-e Free',
        active: true,
        prices: [
          price({
            id: 'price_dfe_free_monthly',
            metadata: {plan: 'free', quota_nfe: '3', quota_cte: '0', quota_companies: '-1'},
          }),
        ],
      },
    ]
    const [free] = buildPlanOptions(products)
    expect(free.plan).toBe('free')
    expect(free.quotas).toEqual({nfe: 3, cte: 0, companies: -1})
    expect(free.priceIds).toEqual(['price_dfe_free_monthly'])
    expect(free.monthlyCents).toBe(0)
  })

  it('an unreadable quota is dropped, not read as zero', () => {
    const products: BillingProduct[] = [
      {
        id: 'prod_dfe_free',
        name: 'DF-e Free',
        active: true,
        prices: [price({metadata: {plan: 'free', quota_nfe: 'três'}})],
      },
    ]
    // Absent means "not included", which is honest about a broken value; zero
    // would claim the plan grants the meter and gives you none of it.
    expect(buildPlanOptions(products)[0].quotas).toEqual({})
  })

  it('composes the on-demand plan out of all of its metered prices', () => {
    const products: BillingProduct[] = [
      {
        id: 'prod_dfe_ondemand',
        name: 'DF-e Sob Demanda',
        active: true,
        prices: [
          price({
            id: 'price_dfe_ondemand_nfe',
            type: 'metered',
            unit_amount: 5,
            billing_timing: 'arrears',
            metadata: {plan: 'ondemand', meter: 'nfe', quota_nfe: '-1'},
          }),
          price({
            id: 'price_dfe_ondemand_nfce',
            type: 'metered',
            unit_amount: 1,
            billing_timing: 'arrears',
            metadata: {plan: 'ondemand', meter: 'nfce', quota_nfce: '-1'},
          }),
        ],
      },
    ]
    const [ondemand] = buildPlanOptions(products)
    // One subscription, several items — sending only one price would sell a
    // plan that meters a single document type.
    expect(ondemand.priceIds).toEqual(['price_dfe_ondemand_nfe', 'price_dfe_ondemand_nfce'])
    expect(ondemand.metered).toEqual([
      {meter: 'nfe', unitAmount: 5},
      {meter: 'nfce', unitAmount: 1},
    ])
    expect(ondemand.monthlyCents).toBe(0)
  })

  it('orders the plans cheapest commitment first', () => {
    const make = (id: string, plan: string): BillingProduct => ({
      id,
      name: plan,
      active: true,
      prices: [price({id: `p_${plan}`, metadata: {plan}})],
    })
    const options = buildPlanOptions([
      make('p1', 'unlimited'),
      make('p2', 'free'),
      make('p3', 'pro'),
      make('p4', 'ondemand'),
    ])
    expect(options.map((o) => o.plan)).toEqual(['free', 'ondemand', 'pro', 'unlimited'])
  })
})

describe('grantedMeters', () => {
  it('leaves out a meter granted zero — "not included" is not "you ran out"', () => {
    expect(grantedMeters({nfe: 3, cte: 0, companies: 1})).toEqual(['nfe', 'companies'])
  })

  it('leaves out a meter the plan never mentions', () => {
    expect(grantedMeters({nfe: 3})).toEqual(['nfe'])
  })
})
