/** Mirrors `api/internal/api/v1/openapi/push.yaml`. */

export type PushEventType = 'distribution.received' | 'document.authorized' | 'document.rejected' | 'document.failed'

export interface PushConfig {
  /** False: the installation has no VAPID key or sender Lambda; the screen says so and writes answer 503. */
  enabled: boolean
  /** VAPID public key, base64url (uncompressed P-256 point). Absent when disabled. */
  public_key?: string
  /** Short key id; the client resubscribes when it changes. */
  vapid_kid?: string
  event_types: PushEventType[]
  /** Events a new device starts with. */
  defaults: string[]
  max_subscriptions: number
}

export interface PushDevice {
  id: string
  /** Coarse label such as "Chrome no Android". */
  device: string
  created_at?: string
  last_seen_at?: string
}

export interface PushDeviceList {
  data: PushDevice[]
  max_subscriptions: number
}

export interface PushSubscriptionBody {
  endpoint: string
  keys: { p256dh: string; auth: string }
}

export interface PushPreferences {
  events: string[]
}
