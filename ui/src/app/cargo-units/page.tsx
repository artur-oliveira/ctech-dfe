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
import {PackageIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {
  CARGO_UNIT_KIND_OPTIONS,
  TP_UNID_CARGA_OPTIONS,
  TP_UNID_TRANSP_OPTIONS,
} from '@/lib/schemas/cargo-units'
import type {CargoUnitItemOut} from '@/lib/types/api'

const label = (options: {value: string; label: string}[], code: string) =>
  options.find((o) => o.value === code)?.label ?? code

function CargoUnitsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<CargoUnitItemOut>({
      queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getCargoUnits({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<CargoUnitItemOut>({
    mutationFn: (id) => apiClient.deleteCargoUnit(id),
    getId: (u) => extractId(u.sk, SK_PREFIX.CARGO_UNIT),
    getDeletedMessage: (u) => `"${u.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.cargoUnits.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Unidades de transporte e de carga"
          description="Carretas, vagões, contêineres e pallets que recorrem entre viagens; o rateio da carga é calculado na emissão"
          action={selectedOrg ? {
            label: 'Nova unidade',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/cargo-units/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma unidade cadastrada"
            description="Uma unidade guarda tipo, identificação e lacres fixos. Na emissão do MDF-e basta apontar quais documentos ela leva — o rateio sai dos pesos."
            action={{label: 'Nova unidade', onClick: () => router.push('/cargo-units/new')}}
            icon={<PackageIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Unidades de transporte e de carga"
            minWidth={480}
            headers={['Nome', 'Tipo', 'Classificação', 'Identificação', {label: '', align: 'right'}]}
          >
            {visibleItems.map((u) => (
              <tr key={u.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{u.name}</td>
                <td data-label="Tipo" className={`${TABLE_CELL} text-gray-600`}>
                  {label(CARGO_UNIT_KIND_OPTIONS, u.kind)}
                </td>
                <td data-label="Classificação" className={`${TABLE_CELL} text-gray-600`}>
                  {label(u.kind === 'cargo' ? TP_UNID_CARGA_OPTIONS : TP_UNID_TRANSP_OPTIONS, u.tp_unid)}
                </td>
                <td data-label="Identificação" className={`${TABLE_CELL} text-gray-600`}>{u.id_unid}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/cargo-units/edit?id=${extractId(u.sk, SK_PREFIX.CARGO_UNIT)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(u)} disabled={isDeleting}
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

export default function CargoUnitsPage() {
  return (
    <ProtectedRoute>
      <CargoUnitsContent/>
    </ProtectedRoute>
  )
}
