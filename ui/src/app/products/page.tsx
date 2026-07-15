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
import {TableShell, TABLE_ROW, TABLE_CELL, RowCheckbox} from '@/components/ui/table-shell'
import {BulkActionBar} from '@/components/ui/bulk-action-bar'
import {useRowSelection} from '@/lib/hooks/useRowSelection'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {ProductOut} from '@/lib/types/api'
import {formatCurrency} from "@/lib/utils/helpers";

function ProductsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ProductOut>({
      queryKey: queryKeys.products.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getProducts({cursor}),
      enabled: !!selectedOrg,
    })
  
  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ProductOut>({
    mutationFn: (id) => apiClient.deleteProduct(id),
    getId: (p) => extractId(p.sk, SK_PREFIX.PRODUCT),
    getDeletedMessage: (p) => `Produto "${p.description}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.products.list(selectedOrg?.pk)})
    },
  })
  
  // Rows inside the undo window are hidden until the delete commits (or is undone).
  const visibleItems = filterVisible(items)

  const rowId = (p: ProductOut) => extractId(p.sk, SK_PREFIX.PRODUCT)
  const selection = useRowSelection(visibleItems.map(rowId))
  const bulkDelete = () => {
    const byId = new Map(visibleItems.map((p) => [rowId(p), p]))
    selection.selectedIds.forEach((id) => {
      const p = byId.get(id)
      if (p) handleDelete(p)
    })
    selection.clear()
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Produtos"
          description="Cadastro de produtos e mercadorias"
          action={selectedOrg ? {
            label: 'Novo produto',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/products/new'),
          } : undefined}
        />
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum produto cadastrado"
            description="Adicione produtos para usar na emissão de NF-e e NFC-e."
            action={{label: 'Novo produto', onClick: () => router.push('/products/new')}}
          />
        ) : (
          <TableShell
            ariaLabel="Produtos cadastrados"
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
              'Descrição', 'Valor (Revenda)', 'Valor (Consumidor final)', {label: '', align: 'right'},
            ]}
          >
            {visibleItems.map((p) => (
              <tr key={p.sk} className={TABLE_ROW}>
                <td className={TABLE_CELL}>
                  <RowCheckbox
                    checked={selection.isSelected(rowId(p))}
                    onChange={() => selection.toggle(rowId(p))}
                    ariaLabel={`Selecionar ${p.description}`}
                  />
                </td>
                <td data-label="Descrição" className={`${TABLE_CELL} font-medium text-gray-900`}>{p.description + (p.brand ? ' ' + p.brand : '')}</td>
                <td data-label="Valor (Revenda)" className={`${TABLE_CELL} text-gray-700`}>{p.value_resale ? formatCurrency(p.value_resale) : '-'}</td>
                <td data-label="Valor (Consumidor final)" className={`${TABLE_CELL} text-gray-700`}>{formatCurrency(p.value)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => router.push(`/products/edit?id=${extractId(p.sk, SK_PREFIX.PRODUCT)}`)}
                      className="min-h-11 sm:min-h-0 text-brand-600 hover:text-brand-700"
                    >
                      Editar
                    </Button>
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => handleDelete(p)}
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

export default function ProductsPage() {
  return (
    <ProtectedRoute>
      <ProductsContent/>
    </ProtectedRoute>
  )
}
