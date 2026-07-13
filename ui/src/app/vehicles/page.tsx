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
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
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
          />
        ) : (
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden overflow-x-auto">
            <table className="w-full text-sm min-w-[640px]">
              <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['Placa', 'UF', 'Tipo', 'Carroceria', 'Tara', 'Proprietário', ''].map((h) => (
                  <th key={h}
                      className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    {h}
                  </th>
                ))}
              </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
              {visibleItems.map((v) => (
                <tr key={v.sk} className="hover:bg-gray-50 transition-colors">
                  <td className="px-5 py-3.5 font-mono font-medium text-gray-900">{v.plate}</td>
                  <td className="px-5 py-3.5 text-gray-600">{v.plate_uf}</td>
                  <td className="px-5 py-3.5 text-gray-700">{v.role === 'trailer' ? 'Reboque' : 'Tração'}</td>
                  <td className="px-5 py-3.5 text-gray-600">{v.bodywork ?? '—'}</td>
                  <td
                    className="px-5 py-3.5 text-gray-600">{v.weight ? `${v.weight.toLocaleString('pt-BR')} kg` : '—'}</td>
                  <td className="px-5 py-3.5 text-gray-600 max-w-[160px] truncate">{v.owner?.name ?? '—'}</td>
                  <td className="px-5 py-3.5 text-right">
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
                        className="text-red-500 hover:text-red-700"
                      >
                        Excluir
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              </tbody>
            </table>
          </div>
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

export default function VehiclesPage() {
  return (
    <ProtectedRoute>
      <VehiclesContent/>
    </ProtectedRoute>
  )
}
