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
import type {OperationItemOut} from '@/lib/types/api'

function OperationsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<OperationItemOut>({
      queryKey: queryKeys.operations.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getOperations({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<OperationItemOut>({
    mutationFn: (id) => apiClient.deleteOperation(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.OPERATION),
    getDeletedMessage: (p) => `"${p.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.operations.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Naturezas de operação"
          description="Os campos que sempre andam juntos por cenário de negócio, respondidos uma vez"
          action={selectedOrg ? {
            label: 'Nova operação',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/operations/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma natureza de operação"
            description="Uma operação guarda natureza, finalidade, CFOP e mensagem fiscal de um cenário de negócio (venda para revenda, remessa para conserto) para não redigitar tudo a cada emissão."
            action={{label: 'Nova operação', onClick: () => router.push('/operations/new')}}
            icon={<RouteIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Naturezas de operação"
            minWidth={480}
            headers={['Nome', 'Natureza fiscal', 'Documentos', 'Padrão', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="Natureza fiscal" className={`${TABLE_CELL} font-mono text-xs text-gray-600`}>
                  {p.cfop_suffix ? `x${p.cfop_suffix}` : '—'}
                </td>
                <td data-label="Documentos" className={`${TABLE_CELL} text-gray-600`}>
                  {(p.doc_types ?? []).join(', ').toUpperCase() || '—'}
                </td>
                <td data-label="Padrão" className={TABLE_CELL}>
                  {p.is_default ? (
                    <span className="inline-flex items-center rounded bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">
                      Padrão
                    </span>
                  ) : (
                    <span className="text-xs text-gray-500">—</span>
                  )}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/operations/edit?id=${extractId(p.sk, SK_PREFIX.OPERATION)}`)}
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

export default function OperationsPage() {
  return (
    <ProtectedRoute>
      <OperationsContent/>
    </ProtectedRoute>
  )
}
