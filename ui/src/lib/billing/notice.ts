/**
 * What to say when billing refuses something, in one place.
 *
 * Two sources feed the same vocabulary: a 402 the API returned (`reason`,
 * `meter`, `quota_limit`) and the subscription snapshot the screens already
 * hold. Both end in the same sentence and the same button, so they are written
 * once — a "erro interno" on a payment refusal is how a customer ends up calling
 * support about an invoice.
 */

import {ApiError} from '@/lib/api/client'
import {METER_LABELS, PLAN_LABELS, STATUS_CANCELED, STATUS_INCOMPLETE, STATUS_PAST_DUE, STATUS_PAUSED} from '@/lib/constants/billing'
import type {AccountSubscription} from '@/lib/types/billing'
import {ONBOARDING_ROOT, STEP_PLAN} from '@/lib/constants/onboarding'

/** RFC 7807 type for every refusal money fixes (`api/internal/problem`). */
export const PROBLEM_TYPE_PAYMENT_REQUIRED = '/problems/payment-required'

/** `reason` keys the API sends with a 402. The UI branches on these, never on text. */
export const REASON_SUBSCRIPTION_MISSING = 'subscription_missing'
export const REASON_SUBSCRIPTION_PAST_DUE = 'subscription_past_due'
export const REASON_SUBSCRIPTION_INCOMPLETE = 'subscription_incomplete'
export const REASON_SUBSCRIPTION_CANCELED = 'subscription_canceled'
export const REASON_SUBSCRIPTION_PAUSED = 'subscription_paused'
export const REASON_QUOTA_EXCEEDED = 'quota_exceeded'

export const SUBSCRIPTION_PATH = '/assinatura'
/** Opens the plan-change dialog straight from a link. */
export const QUERY_CHANGE_PLAN = 'mudar'
export const CHANGE_PLAN_PATH = `${SUBSCRIPTION_PATH}?${QUERY_CHANGE_PLAN}=1`
const PLAN_CHOICE_PATH = `${ONBOARDING_ROOT}/${STEP_PLAN}`

export interface BillingProblem {
  reason: string
  meter?: string
  quotaLimit?: number
  quotaUsed?: number
  plan?: string
  detail: string
}

export interface BillingNotice {
  title: string
  message: string
  actionLabel: string
  href: string
}

/** The billing half of a 402, or null when the error is something else. */
export function parseBillingProblem(err: unknown): BillingProblem | null {
  if (!(err instanceof ApiError)) return null
  const raw = err.raw as Record<string, unknown> | undefined
  if (raw?.type !== PROBLEM_TYPE_PAYMENT_REQUIRED) return null
  return {
    reason: typeof raw.reason === 'string' ? raw.reason : '',
    meter: typeof raw.meter === 'string' ? raw.meter : undefined,
    quotaLimit: typeof raw.quota_limit === 'number' ? raw.quota_limit : undefined,
    quotaUsed: typeof raw.quota_used === 'number' ? raw.quota_used : undefined,
    plan: typeof raw.plan === 'string' ? raw.plan : undefined,
    detail: err.detail,
  }
}

function planName(plan?: string): string {
  if (!plan) return 'seu plano'
  return `o plano ${PLAN_LABELS[plan] ?? plan}`
}

function meterName(meter?: string): string {
  if (!meter) return 'documentos'
  return METER_LABELS[meter] ?? meter
}

/** The sentence and the button for a refusal, by reason. */
export function noticeForReason(p: BillingProblem): BillingNotice {
  switch (p.reason) {
    case REASON_QUOTA_EXCEEDED:
      return {
        title: 'Limite do plano atingido',
        message:
          p.quotaLimit != null && p.quotaUsed != null
            ? `${planName(p.plan)} inclui ${p.quotaLimit.toLocaleString('pt-BR')} ${meterName(p.meter)} por mês e você já emitiu ${p.quotaUsed.toLocaleString('pt-BR')}. Mudar de plano libera a emissão agora.`
            : p.detail,
        actionLabel: 'Mudar de plano',
        href: CHANGE_PLAN_PATH,
      }
    case REASON_SUBSCRIPTION_MISSING:
      return {
        title: 'Escolha um plano para emitir',
        message: 'A conta ainda não tem assinatura. Escolher um plano leva menos de um minuto e libera a emissão.',
        actionLabel: 'Escolher plano',
        href: PLAN_CHOICE_PATH,
      }
    case REASON_SUBSCRIPTION_INCOMPLETE:
      return {
        title: 'Pagamento pendente',
        message: 'A assinatura foi criada, mas o primeiro pagamento ainda não foi confirmado. Assim que ele cair, a emissão volta sozinha.',
        actionLabel: 'Pagar agora',
        href: SUBSCRIPTION_PATH,
      }
    case REASON_SUBSCRIPTION_PAST_DUE:
      return {
        title: 'Pagamento em atraso',
        message: 'Há uma fatura vencida. A emissão fica suspensa até o pagamento ser confirmado; os documentos já emitidos continuam disponíveis.',
        actionLabel: 'Pagar fatura',
        href: SUBSCRIPTION_PATH,
      }
    case REASON_SUBSCRIPTION_CANCELED:
      return {
        title: 'Assinatura cancelada',
        message: 'A emissão está suspensa. Assinar de novo reativa a conta com os mesmos dados e documentos.',
        actionLabel: 'Assinar de novo',
        href: SUBSCRIPTION_PATH,
      }
    case REASON_SUBSCRIPTION_PAUSED:
      return {
        title: 'Assinatura pausada',
        message: 'A emissão está suspensa enquanto a assinatura estiver pausada.',
        actionLabel: 'Ver assinatura',
        href: SUBSCRIPTION_PATH,
      }
    default:
      return {
        title: 'Emissão indisponível',
        message: p.detail,
        actionLabel: 'Ver assinatura',
        href: SUBSCRIPTION_PATH,
      }
  }
}

/** A failed emission, as the form shows it: what happened and what to do. */
export interface EmitFailure {
  message: string
  action?: { label: string; href: string }
}

/**
 * Turns an emission failure into the sentence the form renders.
 *
 * A 402 becomes the specific billing message plus the link that fixes it; a
 * quota that ran out mid-session is the common case, and "erro interno" there is
 * a support call about an invoice. Everything else keeps the API's own detail.
 */
export function emitFailure(err: unknown, fallback: string): EmitFailure {
  const billing = parseBillingProblem(err)
  if (billing) {
    const notice = noticeForReason(billing)
    return {message: notice.message, action: {label: notice.actionLabel, href: notice.href}}
  }
  if (err instanceof ApiError) return {message: err.detail}
  return {message: err instanceof Error ? err.message : fallback}
}

/** Maps a status to the reason the API would have sent for it. */
const STATUS_REASONS: Record<string, string> = {
  [STATUS_INCOMPLETE]: REASON_SUBSCRIPTION_INCOMPLETE,
  [STATUS_PAST_DUE]: REASON_SUBSCRIPTION_PAST_DUE,
  [STATUS_CANCELED]: REASON_SUBSCRIPTION_CANCELED,
  [STATUS_PAUSED]: REASON_SUBSCRIPTION_PAUSED,
}

/**
 * The same notice, derived from the snapshot instead of from a failed request —
 * so a screen can say it before the user fills a form, not after.
 *
 * `grants_service` is the authority: a status this file does not know about but
 * that still grants service must stay silent, and one that does not grant
 * service must always say something.
 */
export function noticeForSubscription(sub: AccountSubscription | undefined): BillingNotice | null {
  if (!sub || sub.no_charge || sub.grants_service) return null
  const reason = !sub.has_subscription
    ? REASON_SUBSCRIPTION_MISSING
    : STATUS_REASONS[sub.status] ?? REASON_SUBSCRIPTION_INCOMPLETE
  return noticeForReason({reason, plan: sub.plan, detail: ''})
}
