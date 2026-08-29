'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ProductLotForm} from '@/components/product-lots/ProductLotForm'
import type {ProductLotCreate} from '@/lib/types/api'

function NewProductLotContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: ProductLotCreate) => apiClient.createProductLot(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.productLots.list(selectedOrg?.pk)})
      router.push('/product-lots')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/product-lots" className="hover:text-brand-600">Lotes de produção</Link>
          <span>/</span>
          <span className="text-gray-600">Novo lote</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo lote de produção</h1>
        <ProductLotForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewProductLotPage() {
  return (
    <ProtectedRoute>
      <NewProductLotContent/>
    </ProtectedRoute>
  )
}
