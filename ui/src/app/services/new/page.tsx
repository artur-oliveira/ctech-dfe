'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ServiceForm} from '@/components/services/ServiceForm'
import type {ServiceCreate} from '@/lib/types/api'

function NewServiceContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: ServiceCreate) => apiClient.createService(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.services.list(selectedOrg?.pk)})
      router.push('/services')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/services" className="hover:text-brand-600">Serviços</Link>
          <span>/</span>
          <span className="text-gray-600">Novo serviço</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo serviço</h1>
        <ServiceForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewServicePage() {
  return (
    <ProtectedRoute>
      <NewServiceContent/>
    </ProtectedRoute>
  )
}
