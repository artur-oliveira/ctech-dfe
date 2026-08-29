'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {PersonForm} from '@/components/persons/PersonForm'
import type {PersonCreate} from '@/lib/types/api'

function NewPersonContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  
  const createMutation = useMutation({
    mutationFn: (d: PersonCreate) => apiClient.createPerson(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.persons.list(selectedOrg?.pk)})
      router.push('/persons')
    },
  })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/persons" className="hover:text-brand-600">Pessoas</Link>
          <span>/</span>
          <span className="text-gray-600">Nova pessoa</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova pessoa</h1>
        <PersonForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewPersonPage() {
  return (
    <ProtectedRoute>
      <NewPersonContent/>
    </ProtectedRoute>
  )
}
