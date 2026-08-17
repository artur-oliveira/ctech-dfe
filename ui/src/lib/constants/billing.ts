/**
 * The vocabulary the billing screens share with the backend.
 *
 * Every key here is a contract with `ctech-billing/api/tenants/ctech.json` (the
 * price metadata) and `api/internal/services/billing.go` (the meter names). They
 * are named once so a rename shows up as a compile error instead of a screen
 * that silently stops finding a quota.
 */

/** Meter names — the suffix of the `quota_*` keys in price metadata. */
export const METER_NFE = 'nfe'
export const METER_NFCE = 'nfce'
export const METER_CTE = 'cte'
export const METER_MDFE = 'mdfe'
export const METER_NFSE = 'nfse'
export const METER_COMPANIES = 'companies'
export const METER_USERS = 'users'

/** Meters counted per issuance, in the order the screens list them. */
export const DOCUMENT_METERS = [METER_NFE, METER_NFCE, METER_CTE, METER_MDFE, METER_NFSE] as const
export type DocumentMeter = (typeof DOCUMENT_METERS)[number]

/** Meters that count current state rather than accumulated issuance. */
export const ACCOUNT_METERS = [METER_COMPANIES, METER_USERS] as const

export const METER_LABELS: Record<string, string> = {
  [METER_NFE]: 'NF-e',
  [METER_NFCE]: 'NFC-e',
  [METER_CTE]: 'CT-e',
  [METER_MDFE]: 'MDF-e',
  [METER_NFSE]: 'NFS-e',
  [METER_COMPANIES]: 'Empresas',
  [METER_USERS]: 'Usuários',
}

/** Price metadata keys. */
export const META_PLAN = 'plan'
export const META_METER = 'meter'
export const META_QUOTA_PREFIX = 'quota_'
export const META_VISIBILITY = 'visibility'

/**
 * Plans seeded for internal use — the zero-priced unlimited the CTech team runs
 * on. Offering it in the chooser would hand every visitor a free unlimited plan.
 */
export const VISIBILITY_INTERNAL = 'internal'

/** `-1` is the limit meaning "no ceiling". */
export const QUOTA_UNLIMITED = -1

export const PLAN_FREE = 'free'
export const PLAN_ONDEMAND = 'ondemand'
export const PLAN_PRO = 'pro'
export const PLAN_UNLIMITED = 'unlimited'

/**
 * Display order and voice for each plan.
 *
 * The catalogue itself — names, prices, quotas — always comes from
 * `GET /v1.0/billing/plans`. What lives here is only what billing has no opinion
 * about: which plan leads, and the one line that says who it is for.
 */
export const PLAN_PRESENTATION: Record<string, { order: number; tagline: string; recommended?: boolean }> = {
  [PLAN_FREE]: {
    order: 1,
    tagline: 'Para experimentar e emitir os primeiros documentos.',
  },
  [PLAN_ONDEMAND]: {
    order: 2,
    tagline: 'Sem mensalidade. Você paga por documento emitido.',
  },
  [PLAN_PRO]: {
    order: 3,
    tagline: 'Para quem emite todo dia e não quer pensar em limite.',
    recommended: true,
  },
  [PLAN_UNLIMITED]: {
    order: 4,
    tagline: 'Volume alto, sem teto em nenhum documento.',
  },
}

export const PLAN_LABELS: Record<string, string> = {
  [PLAN_FREE]: 'Free',
  [PLAN_ONDEMAND]: 'Sob demanda',
  [PLAN_PRO]: 'Pro',
  [PLAN_UNLIMITED]: 'Ilimitado',
}

/** Subscription statuses, spelled the way billing sends them. */
export const STATUS_ACTIVE = 'ACTIVE'
export const STATUS_TRIALING = 'TRIALING'
export const STATUS_INCOMPLETE = 'INCOMPLETE'
export const STATUS_PAST_DUE = 'PAST_DUE'
export const STATUS_PAUSED = 'PAUSED'
export const STATUS_CANCELED = 'CANCELED'

export const STATUS_LABELS: Record<string, string> = {
  [STATUS_ACTIVE]: 'Ativa',
  [STATUS_TRIALING]: 'Em teste',
  [STATUS_INCOMPLETE]: 'Aguardando pagamento',
  [STATUS_PAST_DUE]: 'Pagamento em atraso',
  [STATUS_PAUSED]: 'Pausada',
  [STATUS_CANCELED]: 'Cancelada',
}

/**
 * Badge colours, on the same fixed semantic palette the document statuses use
 * (DESIGN.md §7) — never recoloured by `data-dfe-theme`, because "em atraso" has
 * to read the same on every screen.
 */
export const STATUS_BADGE_CLASSES: Record<string, string> = {
  [STATUS_ACTIVE]: 'bg-green-100 text-green-700',
  [STATUS_TRIALING]: 'bg-blue-50 text-blue-700',
  [STATUS_INCOMPLETE]: 'bg-amber-50 text-amber-700',
  [STATUS_PAST_DUE]: 'bg-red-100 text-red-700',
  [STATUS_PAUSED]: 'bg-gray-100 text-gray-500',
  [STATUS_CANCELED]: 'bg-gray-100 text-gray-500',
}

/** Invoice statuses, as billing spells them. */
export const INVOICE_STATUS_LABELS: Record<string, string> = {
  DRAFT: 'Rascunho',
  OPEN: 'Em aberto',
  PAID: 'Paga',
  VOID: 'Cancelada',
  UNCOLLECTIBLE: 'Não recebida',
}

export const INVOICE_STATUS_CLASSES: Record<string, string> = {
  DRAFT: 'bg-gray-100 text-gray-500',
  OPEN: 'bg-amber-50 text-amber-700',
  PAID: 'bg-green-100 text-green-700',
  VOID: 'bg-gray-100 text-gray-500',
  UNCOLLECTIBLE: 'bg-red-100 text-red-700',
}

/** Formats cents the way an invoice reads them. */
export function formatCents(cents: number): string {
  return (cents / 100).toLocaleString('pt-BR', {style: 'currency', currency: 'BRL'})
}
