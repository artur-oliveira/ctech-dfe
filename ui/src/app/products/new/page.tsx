'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ProductForm} from '@/components/products/ProductForm'
import type {ProductCreate} from '@/lib/types/api'

function NewProductContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  
  const {data: org} = useQuery({
    queryKey: queryKeys.organizations.detail(selectedOrg?.pk ?? ''),
    queryFn: () => apiClient.getOrganization(selectedOrg!.pk),
    enabled: !!selectedOrg,
  })
  
  const createMutation = useMutation({
    mutationFn: (d: ProductCreate) => apiClient.createProduct(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.products.list(selectedOrg?.pk)})
      router.push('/products')
    },
  })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/products" className="hover:text-brand-600">Produtos</Link>
          <span>/</span>
          <span className="text-gray-600">Novo produto</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo produto</h1>
        <ProductForm
          crt={org?.person?.crt}
          uf={org?.person?.state_registrations?.[0]?.uf}
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewProductPage() {
  return (
    <ProtectedRoute>
      <NewProductContent/>
    </ProtectedRoute>
  )
}
