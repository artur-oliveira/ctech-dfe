'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {TollProviderForm} from '@/components/toll-providers/TollProviderForm'
import type {TollProviderCreate} from '@/lib/types/api'

function NewTollProviderContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: TollProviderCreate) => apiClient.createTollProvider(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.tollProviders.list(selectedOrg?.pk)})
      router.push('/toll-providers')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/toll-providers" className="hover:text-brand-600">Fornecedoras de vale-pedágio</Link>
          <span>/</span>
          <span className="text-gray-600">Nova fornecedora</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova fornecedora de vale-pedágio</h1>
        <TollProviderForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewTollProviderPage() {
  return (
    <ProtectedRoute>
      <NewTollProviderContent/>
    </ProtectedRoute>
  )
}
