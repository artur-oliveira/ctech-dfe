import {describe, expect, it} from 'vitest'
import {ApiError} from '@/lib/api/client'
import {
  CHANGE_PLAN_PATH,
  emitFailure,
  noticeForSubscription,
  parseBillingProblem,
  PROBLEM_TYPE_PAYMENT_REQUIRED,
  REASON_QUOTA_EXCEEDED,
} from '@/lib/billing/notice'
import type {AccountSubscription} from '@/lib/types/billing'

function quotaError(): ApiError {
  return new ApiError(402, 'cota esgotada', {
    type: PROBLEM_TYPE_PAYMENT_REQUIRED,
    reason: REASON_QUOTA_EXCEEDED,
    meter: 'nfe',
    quota_limit: 3,
    quota_used: 3,
    plan: 'free',
  })
}

const ACTIVE: AccountSubscription = {
  has_subscription: true,
  status: 'ACTIVE',
  plan: 'pro',
  grants_service: true,
  cancel_at_period_end: false,
  period_start: '2026-08-01',
  period_end: '2026-09-01',
  quotas: {nfe: 1000},
  no_charge: false,
}

describe('parseBillingProblem', () => {
  it('reads the numbers a 402 carries so the screen needs no second call', () => {
    expect(parseBillingProblem(quotaError())).toEqual({
      reason: REASON_QUOTA_EXCEEDED,
      meter: 'nfe',
      quotaLimit: 3,
      quotaUsed: 3,
      plan: 'free',
      detail: 'cota esgotada',
    })
  })

  it('ignores errors that are not about money', () => {
    expect(parseBillingProblem(new ApiError(422, 'CFOP inválido', {type: '/problems/validation-error'}))).toBeNull()
    expect(parseBillingProblem(new Error('rede'))).toBeNull()
  })
})

describe('emitFailure', () => {
  it('turns a quota refusal into the sentence plus the link that fixes it', () => {
    const failure = emitFailure(quotaError(), 'Erro ao emitir NF-e.')
    expect(failure.message).toContain('3')
    expect(failure.message).toContain('NF-e')
    expect(failure.action).toEqual({label: 'Mudar de plano', href: CHANGE_PLAN_PATH})
  })

  it('keeps the API detail for anything else, with no action', () => {
    const failure = emitFailure(new ApiError(422, 'CFOP inválido', {}), 'Erro ao emitir NF-e.')
    expect(failure).toEqual({message: 'CFOP inválido'})
  })

  it('falls back only when the error says nothing', () => {
    expect(emitFailure({}, 'Erro ao emitir NF-e.')).toEqual({message: 'Erro ao emitir NF-e.'})
  })
})

describe('noticeForSubscription', () => {
  it('stays silent while the account may issue', () => {
    expect(noticeForSubscription(ACTIVE)).toBeNull()
    expect(noticeForSubscription(undefined)).toBeNull()
  })

  it('stays silent on an installation that charges nobody', () => {
    // `no_charge` grants everything; a banner here would be a bill that does
    // not exist.
    expect(noticeForSubscription({...ACTIVE, grants_service: false, no_charge: true})).toBeNull()
  })

  it('names the overdue payment rather than a generic failure', () => {
    const notice = noticeForSubscription({...ACTIVE, status: 'PAST_DUE', grants_service: false})
    expect(notice?.title).toBe('Pagamento em atraso')
    expect(notice?.actionLabel).toBe('Pagar fatura')
  })

  it('asks for a plan when there is none, before asking for a payment', () => {
    const notice = noticeForSubscription({...ACTIVE, has_subscription: false, status: '', grants_service: false})
    expect(notice?.title).toBe('Escolha um plano para emitir')
  })

  it('speaks up for a status it does not know, because grants_service is the authority', () => {
    // A status added in billing tomorrow must not produce a silent screen on an
    // account that cannot issue.
    const notice = noticeForSubscription({...ACTIVE, status: 'SOMETHING_NEW' as never, grants_service: false})
    expect(notice).not.toBeNull()
  })
})
