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
import {TruckIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TableShell, TABLE_ROW, TABLE_CELL, RowCheckbox} from '@/components/ui/table-shell'
import {BulkActionBar} from '@/components/ui/bulk-action-bar'
import {useRowSelection} from '@/lib/hooks/useRowSelection'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {VehicleOut} from '@/lib/types/api'

function VehiclesContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<VehicleOut>({
      queryKey: queryKeys.vehicles.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getVehicles({cursor}),
      enabled: !!selectedOrg,
    })
  
  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<VehicleOut>({
    mutationFn: (id) => apiClient.deleteVehicle(id),
    getId: (v) => extractId(v.sk, SK_PREFIX.VEHICLE),
    getDeletedMessage: (v) => `Veículo ${v.plate} excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.vehicles.list(selectedOrg?.pk)})
    },
  })
  
  // Rows inside the undo window are hidden until the delete commits (or is undone).
  const visibleItems = filterVisible(items)

  const rowId = (v: VehicleOut) => extractId(v.sk, SK_PREFIX.VEHICLE)
  const selection = useRowSelection(visibleItems.map(rowId))
  const bulkDelete = () => {
    const byId = new Map(visibleItems.map((v) => [rowId(v), v]))
    selection.selectedIds.forEach((id) => {
      const v = byId.get(id)
      if (v) handleDelete(v)
    })
    selection.clear()
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Veículos"
          description="Frota cadastrada para CT-e e MDF-e"
          action={selectedOrg ? {
            label: 'Novo veículo',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/vehicles/new'),
          } : undefined}
        />
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum veículo cadastrado"
            description="Cadastre veículos para usar na emissão de CT-e e MDF-e."
            action={{label: 'Novo veículo', onClick: () => router.push('/vehicles/new')}}
            icon={<TruckIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Veículos cadastrados"
            minWidth={640}
            headers={[
              {label: '__select', className: 'w-10', node: (
                <RowCheckbox
                  checked={selection.allSelected}
                  indeterminate={selection.someSelected}
                  onChange={selection.toggleAll}
                  ariaLabel="Selecionar todos"
                />
              )},
              'Placa', 'UF', 'Tipo', 'Carroceria', 'Tara', 'Proprietário', {label: '', align: 'right'},
            ]}
          >
            {visibleItems.map((v) => (
              <tr key={v.sk} className={TABLE_ROW}>
                <td className={TABLE_CELL}>
                  <RowCheckbox
                    checked={selection.isSelected(rowId(v))}
                    onChange={() => selection.toggle(rowId(v))}
                    ariaLabel={`Selecionar ${v.plate}`}
                  />
                </td>
                <td data-label="Placa" className={`${TABLE_CELL} font-mono font-medium text-gray-900`}>{v.plate}</td>
                <td data-label="UF" className={`${TABLE_CELL} text-gray-600`}>{v.plate_uf}</td>
                <td data-label="Tipo" className={`${TABLE_CELL} text-gray-700`}>{v.role === 'trailer' ? 'Reboque' : 'Tração'}</td>
                <td data-label="Carroceria" className={`${TABLE_CELL} text-gray-600`}>{v.bodywork ?? '—'}</td>
                <td
                  data-label="Tara" className={`${TABLE_CELL} text-gray-600`}>{v.weight ? `${v.weight.toLocaleString('pt-BR')} kg` : '—'}</td>
                <td data-label="Proprietário" className={`${TABLE_CELL} text-gray-600 max-w-[160px] truncate`}>{v.owner?.name ?? '—'}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => router.push(`/vehicles/edit?id=${extractId(v.sk, SK_PREFIX.VEHICLE)}`)}
                      className="text-brand-600 hover:text-brand-700"
                    >
                      Editar
                    </Button>
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => handleDelete(v)}
                      disabled={isDeleting}
                      className="text-danger hover:text-red-700"
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

export default function VehiclesPage() {
  return (
    <ProtectedRoute>
      <VehiclesContent/>
    </ProtectedRoute>
  )
}
