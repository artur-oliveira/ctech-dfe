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
import {LotIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {formatISODateBR} from '@/lib/utils/dfe'
import type {ProductLotItemOut} from '@/lib/types/api'

function str(v: unknown): string {
  return typeof v === 'string' && v ? v : '—'
}

/** Data ISO do cadastro no formato brasileiro; em branco quando ausente. */
function date(v: unknown): string {
  return typeof v === 'string' && v ? formatISODateBR(v) : '—'
}

function ProductLotsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ProductLotItemOut>({
      queryKey: queryKeys.productLots.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getProductLots({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ProductLotItemOut>({
    mutationFn: (id) => apiClient.deleteProductLot(id),
    getId: (l) => extractId(l.sk, SK_PREFIX.PRODUCT_LOT),
    getDeletedMessage: (l) => `"${l.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.productLots.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Lotes de produção"
          description="Número, validade e quantidade cadastrados uma vez; na emissão o item só aponta de qual lote saiu"
          action={selectedOrg ? {
            label: 'Novo lote',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/product-lots/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum lote de produção"
            description="O lote guarda número, fabricação, validade e quantidade. Ele reaparece em várias notas até acabar, e a quantidade de cada nota é rateada da quantidade vendida."
            action={{label: 'Novo lote', onClick: () => router.push('/product-lots/new')}}
            icon={<LotIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Lotes de produção"
            minWidth={520}
            headers={['Nome', 'Lote', 'Fabricação', 'Validade', 'Quantidade', {label: '', align: 'right'}]}
          >
            {visibleItems.map((l) => (
              <tr key={l.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{l.name}</td>
                <td data-label="Lote" className={`${TABLE_CELL} text-gray-600`}>{str(l.n_lote)}</td>
                <td data-label="Fabricação" className={`${TABLE_CELL} text-gray-600`}>{date(l.d_fab)}</td>
                <td data-label="Validade" className={`${TABLE_CELL} text-gray-600`}>{date(l.d_val)}</td>
                <td data-label="Quantidade" className={`${TABLE_CELL} text-gray-600`}>{str(l.q_lote)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/product-lots/edit?id=${extractId(l.sk, SK_PREFIX.PRODUCT_LOT)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(l)} disabled={isDeleting}
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

export default function ProductLotsPage() {
  return (
    <ProtectedRoute>
      <ProductLotsContent/>
    </ProtectedRoute>
  )
}
