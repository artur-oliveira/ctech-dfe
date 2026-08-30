'use client'

import {useCallback, useMemo, useSyncExternalStore} from 'react'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {orgTaxId} from '@/lib/utils/document'
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX} from '@/lib/constants/storage'
import {
  ONBOARDING_STEPS,
  PRODUCT_DOC_VARIANTS,
  SERVICE_DOC_VARIANTS,
  STEP_CERTIFICATE,
  STEP_COMPANY,
  STEP_DOCUMENTS,
  STEP_DONE,
  STEP_PLAN,
  STEP_PRODUCTS,
  STEP_SERVICES,
  type OnboardingStep,
  type StepDefinition,
} from '@/lib/constants/onboarding'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

/** One item of a probe list — we only ever ask "is there at least one?". */
const EXISTENCE_PROBE_LIMIT = 1

export interface OnboardingStepState extends StepDefinition {
  done: boolean
  /** Conditional steps are hidden when the chosen documents don't need them. */
  applicable: boolean
  /** Optional steps end the flow instead of blocking it. */
  optional: boolean
}

function skipStorageKey(orgPk: string | undefined): string {
  return `${STORAGE_KEY_ONBOARDING_SKIPPED_PREFIX}_${orgPk ?? 'none'}`
}

function readSkipped(key: string): OnboardingStep[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as OnboardingStep[]) : []
  } catch {
    // A corrupted preference is not worth an error boundary — the cost of
    // treating it as "nothing skipped" is one extra card on the dashboard.
    return []
  }
}

/** localStorage fires no event for same-tab writes, so skips broadcast their own. */
const SKIP_EVENT = 'dfe:onboarding-skip'

function subscribeSkips(onChange: () => void) {
  window.addEventListener('storage', onChange)
  window.addEventListener(SKIP_EVENT, onChange)
  return () => {
    window.removeEventListener('storage', onChange)
    window.removeEventListener(SKIP_EVENT, onChange)
  }
}

/**
 * Where the user is in first-run setup, derived rather than stored.
 *
 * Every required layer is answered by something the product already knows: the
 * subscription says whether a plan was chosen, `/auth/me` says whether a company
 * exists, the fiscal configs say which documents were set up. Nothing tracks
 * progress separately, so the flow is resumable from any device and cannot
 * disagree with reality — a progress row that says "company: done" for an
 * account with no company is the failure mode this avoids.
 *
 * The optional layers (products, services) are the exception: declining them is
 * a preference, kept in localStorage.
 */
export function useOnboarding() {
  const {user, selectedOrg} = useAuth()
  const {subscription, isPending: subPending, error: subError} = useSubscription()

  const orgPk = selectedOrg?.pk
  const storageKey = skipStorageKey(orgPk)

  const skipped = useSyncExternalStore(
    subscribeSkips,
    useCallback(() => window.localStorage.getItem(storageKey) ?? '', [storageKey]),
    () => '',
  )
  const skippedSteps = useMemo(() => (skipped ? readSkipped(storageKey) : []), [skipped, storageKey])

  const skip = useCallback(
    (step: OnboardingStep) => {
      const next = Array.from(new Set([...readSkipped(storageKey), step]))
      window.localStorage.setItem(storageKey, JSON.stringify(next))
      window.dispatchEvent(new Event(SKIP_EVENT))
    },
    [storageKey],
  )

  // One query per document type; each treats 404 as "not configured yet".
  const nfe = useFiscalConfig('nfe', orgPk)
  const nfce = useFiscalConfig('nfce', orgPk)
  const cte = useFiscalConfig('cte', orgPk)
  const mdfe = useFiscalConfig('mdfe', orgPk)
  const nfse = useFiscalConfig('nfse', orgPk)

  const configured = useMemo<Record<DocVariant, boolean>>(
    () => ({
      nfe: !!nfe.config,
      nfce: !!nfce.config,
      cte: !!cte.config,
      mdfe: !!mdfe.config,
      nfse: !!nfse.config,
    }),
    [nfe.config, nfce.config, cte.config, mdfe.config, nfse.config],
  )

  const configsPending =
    nfe.isPending || nfce.isPending || cte.isPending || mdfe.isPending || nfse.isPending

  // A failed lookup is not an answer. Treating an unreachable API as "nothing
  // configured" is what puts a finished account back in front of the setup
  // checklist, so an error suppresses the flow instead of contradicting it.
  const configsFailed = !!(nfe.error || nfce.error || cte.error || mdfe.error || nfse.error)

  const needsProducts = PRODUCT_DOC_VARIANTS.some((v) => configured[v])
  const needsServices = SERVICE_DOC_VARIANTS.some((v) => configured[v])

  // The certificate layer. `enabled` on the org because the endpoint is scoped
  // to one, and a company is the thing a certificate belongs to.
  const certificatesQuery = useQuery({
    queryKey: queryKeys.certificates(orgPk ?? ''),
    queryFn: () => apiClient.getCertificates(orgPk as string),
    enabled: !!orgPk,
    // The expiry is compared in `select`, not in render: reading the clock
    // during a render is impure, and the answer only changes when the list does.
    select: (items) => items.some((c) => new Date(c.expires_at).getTime() > Date.now()),
  })

  /**
   * A filial that can sign with the matriz's certificate.
   *
   * Without this the certificate layer would never close for a branch: it has
   * no certificate of its own, by design, and would sit unfinished on the
   * dashboard forever asking for a file it must not upload.
   */
  const orgTaxID = selectedOrg ? orgTaxId(selectedOrg) : ''
  const certRequirementQuery = useQuery({
    queryKey: queryKeys.certificateRequirement(orgTaxID),
    queryFn: () => apiClient.certificateRequirement(orgTaxID),
    enabled: !!orgTaxID,
    staleTime: 60_000,
  })

  const productsQuery = useQuery({
    queryKey: queryKeys.products.probe(orgPk),
    queryFn: () => apiClient.getProducts({limit: EXISTENCE_PROBE_LIMIT}),
    enabled: !!orgPk && needsProducts,
  })
  const servicesQuery = useQuery({
    queryKey: queryKeys.services.probe(orgPk),
    queryFn: () => apiClient.getServices({limit: EXISTENCE_PROBE_LIMIT}),
    enabled: !!orgPk && needsServices,
  })

  // `isLoading` — not `isPending` — because a disabled query stays pending
  // forever, and gating the checklist on that would hide it permanently for
  // anyone whose documents do not need a catalogue.
  const probesPending =
    (needsProducts && productsQuery.isLoading) ||
    (needsServices && servicesQuery.isLoading) ||
    (!!orgPk && certificatesQuery.isLoading)
  const probesFailed = !!productsQuery.error || !!servicesQuery.error || !!certificatesQuery.error

  const hasSubscription = !!subscription?.has_subscription || !!subscription?.no_charge
  const hasCompany = (user?.organizations.length ?? 0) > 0
  const hasAnyConfig = Object.values(configured).some(Boolean)
  // An expired certificate is not a certificate: the SEFAZ refuses the
  // signature, so the layer is unanswered and the step has to say so rather
  // than tick itself off because a file was uploaded once.
  const hasOwnCertificate = certificatesQuery.data === true
  // `required === false` means a matriz certificate covers this CNPJ root. The
  // default is `true`: until the answer arrives, a company with no certificate
  // needs one.
  const certificateInherited = certRequirementQuery.data?.required === false
  const hasCertificate = hasOwnCertificate || certificateInherited
  const hasProducts = (productsQuery.data?.items.length ?? 0) > 0
  const hasServices = (servicesQuery.data?.items.length ?? 0) > 0

  const steps = useMemo<OnboardingStepState[]>(() => {
    const doneById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: hasSubscription,
      [STEP_COMPANY]: hasCompany,
      [STEP_CERTIFICATE]: hasCertificate,
      [STEP_DOCUMENTS]: hasAnyConfig,
      [STEP_PRODUCTS]: hasProducts || skippedSteps.includes(STEP_PRODUCTS),
      [STEP_SERVICES]: hasServices || skippedSteps.includes(STEP_SERVICES),
      [STEP_DONE]: false,
    }
    const applicableById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: true,
      [STEP_COMPANY]: true,
      // Nothing to send a certificate to before there is a company.
      [STEP_CERTIFICATE]: hasCompany,
      [STEP_DOCUMENTS]: true,
      [STEP_PRODUCTS]: needsProducts,
      [STEP_SERVICES]: needsServices,
      [STEP_DONE]: true,
    }
    const optionalById: Record<OnboardingStep, boolean> = {
      [STEP_PLAN]: false,
      [STEP_COMPANY]: false,
      [STEP_CERTIFICATE]: false,
      [STEP_DOCUMENTS]: false,
      [STEP_PRODUCTS]: true,
      [STEP_SERVICES]: true,
      [STEP_DONE]: false,
    }
    return ONBOARDING_STEPS.map((s) => ({
      ...s,
      done: doneById[s.id],
      applicable: applicableById[s.id],
      optional: optionalById[s.id],
    }))
  }, [hasSubscription, hasCompany, hasCertificate, hasAnyConfig, hasProducts, hasServices, needsProducts, needsServices, skippedSteps])

  const visibleSteps = useMemo(() => steps.filter((s) => s.applicable), [steps])
  const nextStep = useMemo(() => visibleSteps.find((s) => !s.done), [visibleSteps])

  return {
    steps: visibleSteps,
    /** The first unfinished layer — where "continuar" goes. */
    nextStep,
    /** Every required layer answered; only the optional ones may remain. */
    complete: visibleSteps.every((s) => s.done || s.id === STEP_DONE),
    configured,
    hasSubscription,
    hasCompany,
    hasCertificate,
    /** The certificate is the matriz's — this company must not upload one. */
    certificateInherited,
    /**
     * True until every layer has a real answer. The checklist is derived from
     * five queries, and a half-loaded derivation reads as "nothing is set up" —
     * which is how a configured account gets shown a first-run card for the
     * half second before its products probe lands.
     */
    isPending: subPending || (!!orgPk && (configsPending || probesPending)),
    /** No answer at all — the caller should say nothing rather than guess. */
    isUnknown: !!subError || configsFailed || probesFailed,
    skip,
  }
}
