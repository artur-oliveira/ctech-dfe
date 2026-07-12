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
          <div className="bg-white rounded-xl border border-gray-200 overflow-hidden overflow-x-auto">
            <table className="w-full text-sm min-w-125">
              <thead className="bg-gray-50 border-b border-gray-200">
              <tr>
                {['Descrição', 'Valor (Revenda)', 'Valor (Consumidor final)', ''].map((h) => (
                  <th key={h}
                      className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    {h}
                  </th>
                ))}
              </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
              {visibleItems.map((p) => (
                <tr key={p.sk} className="hover:bg-gray-50 transition-colors">
                  <td
                    className="px-5 py-3.5 font-medium text-gray-900">{p.description + (p.brand ? ' ' + p.brand : '')}</td>
                  <td className="px-5 py-3.5 text-gray-700">{p.value_resale ? formatCurrency(p.value_resale) : '-'}</td>
                  <td className="px-5 py-3.5 text-gray-700">{formatCurrency(p.value)}</td>
                  <td className="px-5 py-3.5 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => router.push(`/products/edit?id=${extractId(p.sk, SK_PREFIX.PRODUCT)}`)}
                        className="text-brand-600 hover:text-brand-700"
                      >
                        Editar
                      </Button>
                      <Button
                        variant="ghost"
                        size="xs"
                        onClick={() => handleDelete(p)}
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

export default function ProductsPage() {
  return (
    <ProtectedRoute>
      <ProductsContent/>
    </ProtectedRoute>
  )
}
