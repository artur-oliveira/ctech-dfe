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
import {CreditCardIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {CARD_BAND_OPTIONS} from '@/components/nfe/PaymentCardFields'
import {formatCpfCnpj} from '@/lib/utils/document'
import type {PaymentTerminalItemOut} from '@/lib/types/api'

/** Rótulo da bandeira padrão; em branco quando o terminal não define uma. */
function bandLabel(code: unknown): string {
  if (typeof code !== 'string' || !code) return '—'
  return CARD_BAND_OPTIONS.find((o) => o.value === code)?.label ?? code
}

function PaymentTerminalsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<PaymentTerminalItemOut>({
      queryKey: queryKeys.paymentTerminals.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getPaymentTerminals({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<PaymentTerminalItemOut>({
    mutationFn: (id) => apiClient.deletePaymentTerminal(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.PAYMENT_TERMINAL),
    getDeletedMessage: (p) => `"${p.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.paymentTerminals.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Terminais de pagamento"
          description="CNPJ recebedor e identificador de cada maquininha, cadastrados uma vez e apontados na emissão"
          action={selectedOrg ? {
            label: 'Novo terminal',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/payment-terminals/new'),
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
            action={{label: 'Novo terminal', onClick: () => router.push('/payment-terminals/new')}}
            icon={<CreditCardIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Terminais de pagamento"
            minWidth={480}
            headers={['Nome', 'CNPJ recebedor', 'Identificador', 'Bandeira padrão', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="CNPJ recebedor" className={`${TABLE_CELL} text-gray-600`}>{formatCpfCnpj(p.cnpj_receb)}</td>
                <td data-label="Identificador" className={`${TABLE_CELL} text-gray-600`}>{p.id_term_pag}</td>
                <td data-label="Bandeira padrão" className={`${TABLE_CELL} text-gray-600`}>
                  {bandLabel(p.t_band)}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/payment-terminals/edit?id=${extractId(p.sk, SK_PREFIX.PAYMENT_TERMINAL)}`)}
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

export default function PaymentTerminalsPage() {
  return (
    <ProtectedRoute>
      <PaymentTerminalsContent/>
    </ProtectedRoute>
  )
}
