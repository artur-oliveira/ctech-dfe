/**
 * Mirrors `api/internal/api/v1/openapi/webhooks.yaml`.
 *
 * `events` and `company_pks` come back `null` when empty (a Go nil slice); empty
 * means "everything", and the UI reads both through `?? []`.
 */

export type WebhookStatus = 'active' | 'disabled'

export type WebhookEventType = 'document.authorized' | 'document.rejected' | 'document.failed'

export interface WebhookEndpoint {
  id: string
  organization_id: string
  url: string
  events: string[] | null
  company_pks: string[] | null
  status: WebhookStatus
  failure_run: number
  disabled_at?: string
  last_delivery_ok: boolean
  last_delivery_at?: string
  last_delivery_detail?: string
  created_at?: string
}

export interface WebhookList {
  data: WebhookEndpoint[]
  /** Companies of the organization, keys only; the names are the user's own list. */
  company_pks: string[]
  /** False: the installation has no key to seal secrets, so registering is unavailable. */
  enabled: boolean
  max_endpoints: number
  event_types: string[]
}

/** The endpoint plus its signing secret, shown once. */
export interface WebhookCreated extends WebhookEndpoint {
  secret: string
}

export interface WebhookRegistration {
  url: string
  events: string[]
  company_pks: string[]
}

export type WebhookDeliveryStatus = 'pending' | 'delivered' | 'failed'

export interface WebhookDelivery {
  endpoint_id: string
  event_id: string
  type: string
  status: WebhookDeliveryStatus
  attempts: number
  next_attempt_at?: string
  last_status_code?: number
  last_error?: string
  delivered_at?: string
  created_at?: string
}

export interface WebhookAttempt {
  number: number
  at: string
  status_code?: number
  duration_ms: number
  error?: string
  response?: string
}

export interface WebhookDeliveriesPage {
  data: WebhookDelivery[]
  next_cursor: string | null
}

export interface WebhookDeliveryDetail {
  delivery: WebhookDelivery
  attempts: WebhookAttempt[]
}
