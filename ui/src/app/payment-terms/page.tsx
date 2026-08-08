'use client'

import {useQueryClient} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {CalendarClockIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {PAYMENT_OPTIONS} from '@/lib/data/payment-options'
import type {PaymentTermItemOut} from '@/lib/types/api'

function paymentLabel(code: string): string {
  return PAYMENT_OPTIONS.find((o) => o.value === code)?.label ?? code
}

function PaymentTermsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<PaymentTermItemOut>({
      queryKey: queryKeys.paymentTerms.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getPaymentTerms({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<PaymentTermItemOut>({
    mutationFn: (id) => apiClient.deletePaymentTerm(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.PAYMENT_TERM),
    getDeletedMessage: (p) => `"${p.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerms.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Condições de pagamento"
          description="Parcelas, vencimentos e forma de pagamento definidos uma vez e reaproveitados na emissão"
          action={selectedOrg ? {
            label: 'Nova condição',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/payment-terms/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma condição de pagamento"
            description="Uma condição guarda forma de pagamento, número de parcelas e vencimentos. Na emissão, o total da nota vira fatura e duplicatas automaticamente."
            action={{label: 'Nova condição', onClick: () => router.push('/payment-terms/new')}}
            icon={<CalendarClockIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Condições de pagamento"
            minWidth={480}
            headers={['Nome', 'Forma', 'Parcelas', 'Vencimentos', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="Forma" className={`${TABLE_CELL} text-gray-600`}>{paymentLabel(p.payment_type)}</td>
                <td data-label="Parcelas" className={`${TABLE_CELL} text-gray-600`}>{p.installments}×</td>
                <td data-label="Vencimentos" className={`${TABLE_CELL} text-gray-600`}>
                  {p.installments > 1
                    ? `${p.first_due_days ?? 0} dias, depois a cada ${p.interval_days ?? 0}`
                    : `${p.first_due_days ?? 0} dias`}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/payment-terms/edit?id=${extractId(p.sk, SK_PREFIX.PAYMENT_TERM)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(p)} disabled={isDeleting}
                            className="text-danger hover:text-red-700">
                      Excluir
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </TableShell>
        )}
        <Pagination
          hasNext={hasNext}
          hasPrevious={hasPrevious}
          onNext={goNext}
          onPrevious={goPrevious}
          isLoading={isFetching}
        />
      </div>
    </RootLayout>
  )
}

export default function PaymentTermsPage() {
  return (
    <ProtectedRoute>
      <PaymentTermsContent/>
    </ProtectedRoute>
  )
}
