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
import {ProductForm} from '@/components/products/ProductForm'
import type {ProductCreate} from '@/lib/types/api'

function EditProductContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: product, isLoading} = useQuery({
    queryKey: queryKeys.products.detail(id),
    queryFn: () => apiClient.getProduct(id),
    enabled: !!id && !!selectedOrg,
  })

  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ProductCreate) => apiClient.updateProduct(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.products.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.products.detail(id)})
      router.push('/products')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/products" className="hover:text-brand-600">Produtos</Link>
          <span>/</span>
          <span className="text-gray-600">Editar produto</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar produto</h1>

        {isLoading ? (
          <div className="space-y-4">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="h-16 bg-gray-100 rounded-xl animate-pulse"/>
            ))}
          </div>
        ) : !product ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Produto não encontrado.
          </div>
        ) : (
          <ProductForm
            initialData={product}
            crt={org?.person.crt}
            uf={org?.person.state_registrations[0]?.uf}
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

export default function EditProductPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditProductContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
