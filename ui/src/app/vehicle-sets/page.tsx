'use client'

import {useQuery, useQueryClient} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {VehicleSetIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {VehicleSetItemOut} from '@/lib/types/api'

function VehicleSetsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<VehicleSetItemOut>({
      queryKey: queryKeys.vehicleSets.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getVehicleSets({cursor}),
      enabled: !!selectedOrg,
    })

  // As placas moram no cadastro de veículos; sem elas a lista mostraria só SKs.
  const {data: vehiclePage} = useQuery({
    queryKey: queryKeys.vehicles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getVehicles({limit: 100}),
    enabled: !!selectedOrg,
  })
  const plateOf = (sk: string) => vehiclePage?.items.find((v) => v.sk === sk)?.plate ?? '—'

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<VehicleSetItemOut>({
    mutationFn: (id) => apiClient.deleteVehicleSet(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.VEHICLE_SET),
    getDeletedMessage: (p) => `"${p.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.vehicleSets.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Composições veiculares"
          description="Cavalo, reboques e condutores que sempre andam juntos, escolhidos de uma vez no MDF-e"
          action={selectedOrg ? {
            label: 'Nova composição',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/vehicle-sets/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma composição veicular"
            description="Uma composição guarda o veículo de tração, os reboques, os condutores, o RNTRC e o CIOT. No MDF-e, escolher a composição preenche tudo isso de uma vez — e cada campo continua editável."
            action={{label: 'Nova composição', onClick: () => router.push('/vehicle-sets/new')}}
            icon={<VehicleSetIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Composições veiculares"
            minWidth={480}
            headers={['Nome', 'Tração', 'Reboques', 'Condutores', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="Tração" className={`${TABLE_CELL} font-mono text-xs text-gray-600`}>
                  {plateOf(p.tractor_sk)}
                </td>
                <td data-label="Reboques" className={`${TABLE_CELL} font-mono text-xs text-gray-600`}>
                  {(p.trailer_sks ?? []).map(plateOf).join(', ') || '—'}
                </td>
                <td data-label="Condutores" className={`${TABLE_CELL} text-gray-600`}>
                  {(p.driver_docs ?? []).length || '—'}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/vehicle-sets/edit?id=${extractId(p.sk, SK_PREFIX.VEHICLE_SET)}`)}
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

export default function VehicleSetsPage() {
  return (
    <ProtectedRoute>
      <VehicleSetsContent/>
    </ProtectedRoute>
  )
}
