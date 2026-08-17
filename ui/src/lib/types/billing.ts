/**
 * Mirrors `api/internal/api/v1/openapi/billing.yaml`.
 *
 * Every field here is read-only to the UI: the subscription is a snapshot the
 * API keeps up to date from ctech-billing webhooks, never something the browser
 * computes. In particular `grants_service` is the single authority on "can this
 * account issue right now" — reimplementing it from `status` would drift the
 * day a status is added.
 */

/** Billing's own subscription status, untranslated. */
export type SubscriptionStatus =
  | 'ACTIVE'
  | 'TRIALING'
  | 'INCOMPLETE'
  | 'PAST_DUE'
  | 'PAUSED'
  | 'CANCELED'
  | ''

export type InvoiceStatus = 'DRAFT' | 'OPEN' | 'PAID' | 'VOID' | 'UNCOLLECTIBLE'

export interface BillingPrice {
  id: string
  product_id: string
  type: 'fixed' | 'metered'
  /** Cents. */
  unit_amount: number
  billing_timing: 'advance' | 'arrears'
  archived: boolean
  /** Where the quotas (`quota_nfe`, `quota_users`, …) and the `meter` live. */
  metadata: Record<string, string>
}

export interface BillingProduct {
  id: string
  name: string
  active: boolean
  prices: BillingPrice[]
}

export interface BillingPlansResponse {
  /** False when the installation charges nobody; `data` comes back empty. */
  billing_enabled: boolean
  data: BillingProduct[]
}

/** The invoice attached to a live subscription, when there is one to pay. */
export interface BillingOpenInvoice {
  id: string
  total_cents: number
  due_date: string
  /** Present only while the invoice is payable — never build this URL. */
  checkout_url?: string
}

export interface BillingInvoice {
  id: string
  number: number
  status: InvoiceStatus
  overdue: boolean
  due_date: string
  total: number
  amount_due: number
  checkout_url?: string
}

export interface MeterUsage {
  used: number
  /** -1 is unlimited. */
  limit: number
}

export interface AccountSubscription {
  has_subscription: boolean
  status: SubscriptionStatus
  /** free, pro, unlimited or ondemand. */
  plan: string
  /** The only field that decides whether issuance is allowed. */
  grants_service: boolean
  cancel_at_period_end: boolean
  period_start: string
  period_end: string
  /** Limit per meter; -1 unlimited, absent means the plan does not grant it. */
  quotas: Record<string, number>
  /** Installation with no billing configured — everything is released. */
  no_charge: boolean
  /** Present only on `GET /v1.0/billing/subscription`. */
  usage?: Record<string, MeterUsage>
  open_invoice?: BillingOpenInvoice
}

export interface AccountSubscriptionWithInvoice extends AccountSubscription {
  invoice?: BillingInvoice
}

export interface PlanChoice {
  /**
   * A list because the on-demand plan meters each document type with its own
   * price — one subscription, several items.
   */
  price_ids: string[]
}
