'use client'

import {Suspense, useMemo, useState} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {PageHeader} from '@/components/ui/page-header'
import {SectionCard} from '@/components/ui/section-card'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {StatusBadge} from '@/components/ui/status-badge'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button, buttonVariants} from '@/components/ui/button'
import {UsageList} from '@/components/billing/UsageList'
import {ChangePlanDialog} from '@/components/billing/ChangePlanDialog'
import {CancelSubscriptionDialog} from '@/components/billing/CancelSubscriptionDialog'
import {useAuth} from '@/lib/hooks/useAuth'
import {useSubscription} from '@/lib/hooks/useSubscription'
import {ROLE_OWNER} from '@/lib/data/roles'
import {cn} from '@/lib/utils'
import {formatISODateBR} from '@/lib/utils/dfe'
import {QUERY_CHANGE_PLAN} from '@/lib/billing/notice'
import {ONBOARDING_ROOT, STEP_PLAN} from '@/lib/constants/onboarding'
import {
  formatCents,
  INVOICE_STATUS_CLASSES,
  INVOICE_STATUS_LABELS,
  PLAN_LABELS,
  STATUS_BADGE_CLASSES,
  STATUS_LABELS,
} from '@/lib/constants/billing'
import type {AccountSubscription} from '@/lib/types/billing'

const MONTH_OPTIONS_COUNT = 12
const MONTH_NAMES = [
  'janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho',
  'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro',
]

/** `YYYY-MM`, the value the month picker carries. */
function monthKey(year: number, month: number): string {
  return `${year}-${String(month).padStart(2, '0')}`
}

function recentMonths(now: Date): { value: string; label: string }[] {
  return Array.from({length: MONTH_OPTIONS_COUNT}, (_, i) => {
    const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
    return {
      value: monthKey(d.getFullYear(), d.getMonth() + 1),
      label: `${MONTH_NAMES[d.getMonth()]} de ${d.getFullYear()}`,
    }
  })
}

function PlanSummary({subscription}: { subscription: AccountSubscription }) {
  return (
    <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
      <div>
        <p className="text-xl font-semibold text-gray-900">
          {PLAN_LABELS[subscription.plan] ?? subscription.plan}
        </p>
        {subscription.period_start && subscription.period_end && (
          <p className="mt-1 text-sm text-gray-500">
            Período atual: {formatISODateBR(subscription.period_start)} a{' '}
            {formatISODateBR(subscription.period_end)}
          </p>
        )}
      </div>
      <StatusBadge
        label={STATUS_LABELS[subscription.status] ?? subscription.status}
        className={STATUS_BADGE_CLASSES[subscription.status] ?? 'bg-gray-100 text-gray-600'}
        size="md"
      />
    </div>
  )
}

/** What an ADMIN sees: the plan governing this organization, and no buttons. */
function OrganizationPlanView({orgPk}: { orgPk: string }) {
  const {data, isPending, error} = useQuery({
    queryKey: queryKeys.billing.orgPlan(orgPk),
    queryFn: () => apiClient.getOrganizationPlan(orgPk),
  })

  if (isPending) return <LoadingSkeleton/>
  if (error) {
    return (
      <p className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger">
        Não foi possível carregar o plano. {error.message}
      </p>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <SectionCard title="Plano desta organização">
        <PlanSummary subscription={data}/>
        <p className="text-sm leading-relaxed text-gray-600">
          A assinatura pertence ao proprietário da conta — só ele pode contratar, mudar ou cancelar
          o plano. Fale com ele se um limite estiver atrapalhando a operação.
        </p>
      </SectionCard>

      <SectionCard title="Limites do plano">
        <UsageList quotas={data.quotas}/>
      </SectionCard>
    </div>
  )
}

/** What the OWNER sees: everything, and the two actions that spend money. */
function OwnerSubscriptionView() {
  const params = useSearchParams()
  const {subscription, isPending, error} = useSubscription()
  const [changeOpen, setChangeOpen] = useState(params.get(QUERY_CHANGE_PLAN) === '1')
  const [cancelOpen, setCancelOpen] = useState(false)
  const [month, setMonth] = useState(() => {
    const now = new Date()
    return monthKey(now.getFullYear(), now.getMonth() + 1)
  })

  const months = useMemo(() => recentMonths(new Date()), [])
  const [year, monthNumber] = month.split('-').map(Number)

  const invoicesQuery = useQuery({
    queryKey: queryKeys.billing.invoices(year, monthNumber),
    queryFn: () => apiClient.listBillingInvoices(year, monthNumber),
    enabled: !!subscription?.has_subscription,
  })

  if (isPending) return <LoadingSkeleton/>
  if (error) {
    return (
      <p className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-danger">
        Não foi possível carregar a assinatura. {error.message}
      </p>
    )
  }
  if (!subscription) return null

  if (subscription.no_charge) {
    return (
      <SectionCard title="Assinatura">
        <p className="text-sm leading-relaxed text-gray-600">
          Esta instalação não cobra nada. Todos os recursos estão liberados e não há faturas.
        </p>
      </SectionCard>
    )
  }

  if (!subscription.has_subscription) {
    return (
      <SectionCard title="Assinatura">
        <p className="text-sm leading-relaxed text-gray-600">
          A conta ainda não tem um plano. Escolher um libera a emissão de documentos fiscais.
        </p>
        <Link href={`${ONBOARDING_ROOT}/${STEP_PLAN}`} className={cn(buttonVariants(), 'w-full sm:w-auto')}>
          Escolher plano
        </Link>
      </SectionCard>
    )
  }

  const invoices = invoicesQuery.data?.data ?? []
  const openInvoice = subscription.open_invoice

  return (
    <div className="flex flex-col gap-6">
      <SectionCard title="Plano atual">
        <PlanSummary subscription={subscription}/>

        {subscription.cancel_at_period_end && (
          <p className="rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-800">
            A assinatura foi cancelada e termina em{' '}
            {subscription.period_end ? formatISODateBR(subscription.period_end) : 'breve'}. Até lá
            você continua emitindo normalmente.
          </p>
        )}

        <div className="flex flex-col gap-2 sm:flex-row">
          <Button variant="brand" className="w-full sm:w-auto" onClick={() => setChangeOpen(true)}>
            Mudar de plano
          </Button>
          {!subscription.cancel_at_period_end && (
            <Button variant="ghost" className="w-full sm:w-auto" onClick={() => setCancelOpen(true)}>
              Cancelar assinatura
            </Button>
          )}
        </div>
      </SectionCard>

      {openInvoice && (
        <SectionCard title="Fatura em aberto">
          <div className="flex flex-wrap items-baseline justify-between gap-4">
            <div>
              <p className="text-xl font-semibold text-gray-900 tabular-nums">
                {formatCents(openInvoice.total_cents)}
              </p>
              <p className="mt-1 text-sm text-gray-500">
                Vencimento em {formatISODateBR(openInvoice.due_date)}
              </p>
            </div>
            {openInvoice.checkout_url && (
              <a href={openInvoice.checkout_url} className={cn(buttonVariants(), 'w-full sm:w-auto')}>
                Pagar agora
              </a>
            )}
          </div>
        </SectionCard>
      )}

      <SectionCard title="Uso do período">
        <UsageList quotas={subscription.quotas} usage={subscription.usage}/>
      </SectionCard>

      <SectionCard title="Faturas">
        <OptionsSelect
          value={month}
          onValueChange={setMonth}
          options={months}
          ariaLabel="Mês das faturas"
          className="w-full sm:w-64"
        />

        {invoicesQuery.isPending ? (
          <LoadingSkeleton/>
        ) : invoices.length === 0 ? (
          <p className="text-sm text-gray-500">Nenhuma fatura neste mês.</p>
        ) : (
          <ul className="flex flex-col divide-y divide-gray-100">
            {invoices.map((invoice) => (
              <li key={invoice.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
                <div className="min-w-0">
                  <p className="text-sm font-medium text-gray-900">
                    Fatura {invoice.number}
                    <span className="ml-2 font-normal text-gray-500 tabular-nums">
                      {formatCents(invoice.total)}
                    </span>
                  </p>
                  <p className="mt-0.5 text-xs text-gray-500">
                    Vencimento em {formatISODateBR(invoice.due_date)}
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <StatusBadge
                    label={INVOICE_STATUS_LABELS[invoice.status] ?? invoice.status}
                    className={INVOICE_STATUS_CLASSES[invoice.status] ?? 'bg-gray-100 text-gray-600'}
                  />
                  {invoice.amount_due > 0 && invoice.checkout_url && (
                    <a
                      href={invoice.checkout_url}
                      className={cn(buttonVariants({variant: 'outline', size: 'sm'}), 'shrink-0')}
                    >
                      Pagar
                    </a>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </SectionCard>

      <ChangePlanDialog
        isOpen={changeOpen}
        onClose={() => setChangeOpen(false)}
        subscription={subscription}
      />
      <CancelSubscriptionDialog
        isOpen={cancelOpen}
        onClose={() => setCancelOpen(false)}
        subscription={subscription}
      />
    </div>
  )
}

function SubscriptionContent() {
  const {selectedOrg} = useAuth()
  const role = selectedOrg?.role

  if (!selectedOrg) return <NoOrgBanner/>

  return (
    <div className="p-4 md:p-8 max-w-3xl">
      <PageHeader
        title="Assinatura"
        description={
          role === ROLE_OWNER
            ? 'Seu plano, o que já foi usado dele e as faturas da conta.'
            : 'O plano que governa esta organização.'
        }
      />
      {role === ROLE_OWNER ? <OwnerSubscriptionView/> : <OrganizationPlanView orgPk={selectedOrg.pk}/>}
    </div>
  )
}

export default function SubscriptionPage() {
  return (
    <ProtectedRoute>
      <RootLayout>
        {/* useSearchParams needs a boundary — the plan dialog opens from `?mudar=1`. */}
        <Suspense fallback={<div className="p-4 md:p-8"><LoadingSkeleton/></div>}>
          <SubscriptionContent/>
        </Suspense>
      </RootLayout>
    </ProtectedRoute>
  )
}
