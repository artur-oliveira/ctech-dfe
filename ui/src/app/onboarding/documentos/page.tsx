'use client'

import {useMemo, useState} from 'react'
import {useRouter} from 'next/navigation'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {OnboardingShell} from '@/components/onboarding/OnboardingShell'
import {DocumentPicker} from '@/components/onboarding/DocumentPicker'
import {FiscalConfigForm} from '@/components/fiscal-config/FiscalConfigForm'
import {Button} from '@/components/ui/button'
import {useAuth} from '@/lib/hooks/useAuth'
import {useFiscalConfigMutation} from '@/lib/hooks/useFiscalConfig'
import {useOnboarding} from '@/lib/hooks/useOnboarding'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {METER_LABELS} from '@/lib/constants/billing'
import {
  DISTRIBUTION_DEPENDENT_VARIANTS,
  DISTRIBUTION_SOURCE_VARIANT,
  ONBOARDING_ROOT,
  PRODUCT_DOC_VARIANTS,
  SERVICE_DOC_VARIANTS,
  STEP_DOCUMENTS,
  STEP_DONE,
  STEP_PRODUCTS,
  STEP_SERVICES,
} from '@/lib/constants/onboarding'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

/**
 * The numbering an organization that has never issued starts from.
 *
 * Series 1, number 0: the next document out is number 1. A company migrating in
 * overwrites this with the last number it issued elsewhere, which is the whole
 * reason the step asks.
 */
const FRESH_SERIE = '1'
const FRESH_NUMBER = '0'
const DEFAULT_TIMEZONE = 'America/Sao_Paulo'
const ENVIRONMENT_PRODUCTION = '1'

/** The NF-e configuration a carrier gets without issuing a single NF-e. */
function receiveOnlyNfeConfig(source: Record<string, unknown> | undefined) {
  return {
    timezone: source?.timezone ?? DEFAULT_TIMEZONE,
    environment: source?.environment ?? ENVIRONMENT_PRODUCTION,
    prod_current_serie: FRESH_SERIE,
    prod_current_number: FRESH_NUMBER,
    hom_current_serie: FRESH_SERIE,
    hom_current_number: FRESH_NUMBER,
  }
}

function DocumentsStepContent() {
  const router = useRouter()
  const {selectedOrg} = useAuth()
  const {subscription} = useSubscription()
  const {configured} = useOnboarding()

  const [selected, setSelected] = useState<DocVariant[]>([])
  const [queue, setQueue] = useState<DocVariant[] | null>(null)
  const [queueIndex, setQueueIndex] = useState(0)

  const pk = selectedOrg?.pk
  const current = queue?.[queueIndex]
  const saveCurrent = useFiscalConfigMutation(current ?? DISTRIBUTION_SOURCE_VARIANT, pk)
  const saveDistributionSource = useFiscalConfigMutation(DISTRIBUTION_SOURCE_VARIANT, pk)

  const quotas = subscription?.quotas ?? {}
  // A no-charge installation grants everything; without quotas the picker would
  // claim nothing is included.
  const effectiveQuotas = subscription?.no_charge
    ? Object.fromEntries(Object.keys(METER_LABELS).map((m) => [m, -1]))
    : quotas

  /**
   * Where the flow goes next, decided from what was just selected rather than
   * from the derived progress — the configs were saved a moment ago and the
   * queries that would answer "is the product layer applicable now?" are still
   * settling.
   */
  const nextStepAfter = useMemo(() => {
    if (selected.some((v) => PRODUCT_DOC_VARIANTS.includes(v))) return `${ONBOARDING_ROOT}/${STEP_PRODUCTS}`
    if (selected.some((v) => SERVICE_DOC_VARIANTS.includes(v))) return `${ONBOARDING_ROOT}/${STEP_SERVICES}`
    return `${ONBOARDING_ROOT}/${STEP_DONE}`
  }, [selected])

  /**
   * A CT-e is written against the NF-e of the cargo and an MDF-e lists them,
   * and both read those notes from NF-e distribution — which only runs for an
   * organization that has an NF-e configuration. So one is created with its
   * numbering at zero: a configuration that receives and never issues.
   */
  const needsDistributionSource =
    selected.some((v) => DISTRIBUTION_DEPENDENT_VARIANTS.includes(v)) &&
    !configured[DISTRIBUTION_SOURCE_VARIANT] &&
    !selected.includes(DISTRIBUTION_SOURCE_VARIANT)

  async function finish(lastSaved: Record<string, unknown> | undefined) {
    if (needsDistributionSource) {
      await saveDistributionSource.mutateAsync(receiveOnlyNfeConfig(lastSaved))
    }
    router.push(nextStepAfter)
  }

  async function handleSave(data: Record<string, unknown>) {
    await saveCurrent.mutateAsync(data)
    if (queue && queueIndex < queue.length - 1) {
      setQueueIndex(queueIndex + 1)
      return
    }
    await finish(data)
  }

  // Phase 1 — which documents does this company issue?
  if (!queue) {
    return (
      <OnboardingShell
        current={STEP_DOCUMENTS}
        title="O que você emite"
        description="Marque os documentos que a sua empresa emite hoje. Na sequência configuramos a numeração de cada um."
      >
        <DocumentPicker
          quotas={effectiveQuotas}
          configured={configured}
          selected={selected}
          onToggle={(v) => setSelected((s) => (s.includes(v) ? s.filter((x) => x !== v) : [...s, v]))}
        />

        {needsDistributionSource && (
          <p className="mt-4 rounded-lg border border-gray-200 bg-white px-4 py-3 text-sm leading-relaxed text-gray-600">
            Também vamos ativar a NF-e em modo recebimento, sem numeração. É por ela que chegam as notas da carga que o
            CT-e e o MDF-e precisam citar.
          </p>
        )}

        <div className="sticky bottom-0 -mx-4 mt-6 border-t border-gray-200 bg-gray-50/95 px-4 py-3 backdrop-blur md:-mx-8 md:px-8">
          <Button
            size="lg"
            className="w-full sm:w-auto"
            disabled={selected.length === 0}
            onClick={() => {
              setQueue(selected)
              setQueueIndex(0)
            }}
          >
            Continuar
          </Button>
        </div>
      </OnboardingShell>
    )
  }

  // Phase 2 — numbering, one document at a time.
  const position = `${queueIndex + 1} de ${queue.length}`
  return (
    <OnboardingShell
      current={STEP_DOCUMENTS}
      title={`Numeração da ${METER_LABELS[current!]}`}
      description="Se você já emite hoje, informe a série e o último número emitido — o próximo documento sai a partir dele. Se está começando agora, deixe zero."
      action={
        queue.length > 1 ? (
          <span className="text-sm text-gray-500 tabular-nums">{position}</span>
        ) : undefined
      }
    >
      <div className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
        <FiscalConfigForm
          key={current}
          variant={current!}
          initialData={null}
          onSave={handleSave}
          loading={saveCurrent.isPending || saveDistributionSource.isPending}
        />
      </div>

      {saveCurrent.error && (
        <p className="mt-4 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {saveCurrent.error.message}
        </p>
      )}
    </OnboardingShell>
  )
}

export default function DocumentsStepPage() {
  return (
    <ProtectedRoute>
      <DocumentsStepContent/>
    </ProtectedRoute>
  )
}
