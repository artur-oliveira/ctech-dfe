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
import {FuelPumpIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {FuelPumpItemOut} from '@/lib/types/api'

function str(v: unknown): string {
  return typeof v === 'string' && v ? v : '—'
}

function FuelPumpsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<FuelPumpItemOut>({
      queryKey: queryKeys.fuelPumps.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getFuelPumps({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<FuelPumpItemOut>({
    mutationFn: (id) => apiClient.deleteFuelPump(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.FUEL_PUMP),
    getDeletedMessage: (p) => `"${p.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.fuelPumps.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Bombas de combustível"
          description="Bico, bomba e tanque cadastrados uma vez; a leitura do encerrante avança sozinha a cada venda"
          action={selectedOrg ? {
            label: 'Nova bomba',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/fuel-pumps/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma bomba cadastrada"
            description="A bomba guarda bico, bomba e tanque. Na emissão você informa só onde o marcador parou — a leitura inicial é a final da venda anterior."
            action={{label: 'Nova bomba', onClick: () => router.push('/fuel-pumps/new')}}
            icon={<FuelPumpIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Bombas de combustível"
            minWidth={480}
            headers={['Nome', 'Bico', 'Bomba', 'Tanque', 'Última leitura', {label: '', align: 'right'}]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.name}</td>
                <td data-label="Bico" className={`${TABLE_CELL} text-gray-600`}>{str(p.n_bico)}</td>
                <td data-label="Bomba" className={`${TABLE_CELL} text-gray-600`}>{str(p.n_bomba)}</td>
                <td data-label="Tanque" className={`${TABLE_CELL} text-gray-600`}>{str(p.n_tanque)}</td>
                <td data-label="Última leitura" className={`${TABLE_CELL} text-gray-600`}>{str(p.last_v_enc_fin)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/fuel-pumps/edit?id=${extractId(p.sk, SK_PREFIX.FUEL_PUMP)}`)}
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

export default function FuelPumpsPage() {
  return (
    <ProtectedRoute>
      <FuelPumpsContent/>
    </ProtectedRoute>
  )
}
