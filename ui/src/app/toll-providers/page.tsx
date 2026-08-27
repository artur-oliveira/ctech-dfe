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
import {RouteIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {formatCpfCnpj} from '@/lib/utils/document'
import {TP_VALE_PED_OPTIONS} from '@/lib/schemas/toll-providers'
import type {TollProviderItemOut} from '@/lib/types/api'

/** Rótulo do tipo do vale; em branco quando a fornecedora não define um. */
function tpValeLabel(code: unknown): string {
  if (typeof code !== 'string' || !code) return '—'
  return TP_VALE_PED_OPTIONS.find((o) => o.value === code)?.label ?? code
}

function TollProvidersContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<TollProviderItemOut>({
      queryKey: queryKeys.tollProviders.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getTollProviders({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<TollProviderItemOut>({
    mutationFn: (id) => apiClient.deleteTollProvider(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.TOLL_PROVIDER),
    getDeletedMessage: (p) => `"${p.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.tollProviders.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Fornecedoras de vale-pedágio"
          description="CNPJ da fornecedora e do pagador, cadastrados uma vez; por viagem entram só número da compra e valor"
          action={selectedOrg ? {
            label: 'Nova fornecedora',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/toll-providers/new'),
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
            action={{label: 'Nova fornecedora', onClick: () => router.push('/toll-providers/new')}}
            icon={<RouteIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Fornecedoras de vale-pedágio"
            minWidth={480}
            headers={['Nome', 'CNPJ da fornecedora', 'Tipo do vale', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="CNPJ da fornecedora" className={`${TABLE_CELL} text-gray-600`}>{formatCpfCnpj(p.cnpj_forn)}</td>
                                <td data-label="Tipo do vale" className={`${TABLE_CELL} text-gray-600`}>{tpValeLabel(p.tp_vale_ped)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/toll-providers/edit?id=${extractId(p.sk, SK_PREFIX.TOLL_PROVIDER)}`)}
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

export default function TollProvidersPage() {
  return (
    <ProtectedRoute>
      <TollProvidersContent/>
    </ProtectedRoute>
  )
}
