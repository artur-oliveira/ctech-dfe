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
import {ServiceIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TableShell, TABLE_ROW, TABLE_CELL, RowCheckbox} from '@/components/ui/table-shell'
import {BulkActionBar} from '@/components/ui/bulk-action-bar'
import {useRowSelection} from '@/lib/hooks/useRowSelection'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {ServiceOut} from '@/lib/types/api'
import {formatCurrency} from '@/lib/utils/helpers'

function ServicesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ServiceOut>({
      queryKey: queryKeys.services.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getServices({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ServiceOut>({
    mutationFn: (id) => apiClient.deleteService(id),
    getId: (s) => extractId(s.sk, SK_PREFIX.SERVICE),
    getDeletedMessage: (s) => `Serviço "${s.description}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.services.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  const rowId = (s: ServiceOut) => extractId(s.sk, SK_PREFIX.SERVICE)
  const selection = useRowSelection(visibleItems.map(rowId))
  const bulkDelete = () => {
    const byId = new Map(visibleItems.map((s) => [rowId(s), s]))
    selection.selectedIds.forEach((id) => {
      const s = byId.get(id)
      if (s) handleDelete(s)
    })
    selection.clear()
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Serviços"
          description="Catálogo de serviços para emissão de NFS-e"
          action={selectedOrg ? {
            label: 'Novo serviço',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/services/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum serviço cadastrado"
            description="Adicione serviços para usar na emissão de NFS-e."
            action={{label: 'Novo serviço', onClick: () => router.push('/services/new')}}
            icon={<ServiceIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Serviços cadastrados"
            minWidth={560}
            headers={[
              {label: '__select', className: 'w-10', node: (
                <RowCheckbox
                  checked={selection.allSelected}
                  indeterminate={selection.someSelected}
                  onChange={selection.toggleAll}
                  ariaLabel="Selecionar todos"
                />
              )},
              'Código', 'Descrição', 'Cód. tributação nacional', 'Alíquota ISS', 'Valor', {label: '', align: 'right'},
            ]}
          >
            {visibleItems.map((s) => (
              <tr key={s.sk} className={TABLE_ROW}>
                <td className={TABLE_CELL}>
                  <RowCheckbox
                    checked={selection.isSelected(rowId(s))}
                    onChange={() => selection.toggle(rowId(s))}
                    ariaLabel={`Selecionar ${s.description}`}
                  />
                </td>
                <td data-label="Código" className={`${TABLE_CELL} font-medium text-gray-900`}>{s.code}</td>
                <td data-label="Descrição" className={`${TABLE_CELL} text-gray-700`}>{s.description}</td>
                <td data-label="Cód. tributação nacional" className={`${TABLE_CELL} text-gray-700`}>{s.trib_nacional_code}</td>
                <td data-label="Alíquota ISS" className={`${TABLE_CELL} text-gray-700`}>{s.iss.tax_rate}%</td>
                <td data-label="Valor" className={`${TABLE_CELL} text-gray-700`}>{formatCurrency(s.value)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => router.push(`/services/edit?id=${extractId(s.sk, SK_PREFIX.SERVICE)}`)}
                      className="min-h-11 sm:min-h-0 text-brand-600 hover:text-brand-700"
                    >
                      Editar
                    </Button>
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => handleDelete(s)}
                      disabled={isDeleting}
                      className="min-h-11 sm:min-h-0 text-danger hover:text-red-700"
                    >
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
        <BulkActionBar count={selection.count} onClear={selection.clear}>
          <Button
            variant="ghost"
            size="sm"
            onClick={bulkDelete}
            disabled={isDeleting}
            className="text-red-600 hover:text-red-700"
          >
            Excluir selecionados
          </Button>
        </BulkActionBar>
      </div>
    </RootLayout>
  )
}

export default function ServicesPage() {
  return (
    <ProtectedRoute>
      <ServicesContent/>
    </ProtectedRoute>
  )
}
