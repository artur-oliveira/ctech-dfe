'use client'

import {Suspense} from 'react'
import Link from 'next/link'
import {useRouter, useSearchParams} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {ProductLotForm} from '@/components/product-lots/ProductLotForm'
import type {ProductLotCreate} from '@/lib/types/api'

function EditProductLotContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: lot, isLoading} = useQuery({
    queryKey: queryKeys.productLots.detail(id),
    queryFn: () => apiClient.getProductLot(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ProductLotCreate) => apiClient.updateProductLot(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.productLots.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.productLots.detail(id)})
      router.push('/product-lots')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/product-lots" className="hover:text-brand-600">Lotes de produção</Link>
          <span>/</span>
          <span className="text-gray-600">Editar lote</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar lote de produção</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !lot ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Lote de produção não encontrado.
          </div>
        ) : (
          <ProductLotForm
            initialData={lot}
            onSubmit={async (d) => {
              await updateMutation.mutateAsync(d)
            }}
            loading={updateMutation.isPending}
          />
        )}
      </div>
    </RootLayout>
  )
}

export default function EditProductLotPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditProductLotContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
