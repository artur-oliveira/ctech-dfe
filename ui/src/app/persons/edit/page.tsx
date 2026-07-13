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
import {PersonForm} from '@/components/persons/PersonForm'
import type {PersonCreate} from '@/lib/types/api'

function EditPersonContent() {
  const params = useSearchParams()
  // id is the raw cpfCnpj (no prefix, no mask)
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()
  
  const {data: person, isLoading} = useQuery({
    queryKey: queryKeys.persons.detail(id),
    queryFn: () => apiClient.getPerson(id),
    enabled: !!id && !!selectedOrg,
  })
  
  const updateMutation = useMutation({
    mutationFn: (d: PersonCreate) =>
      apiClient.updatePerson(id, {name: d.name, person: d.person}),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.persons.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.persons.detail(id)})
      router.push('/persons')
    },
  })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/persons" className="hover:text-brand-600">Pessoas</Link>
          <span>/</span>
          <span className="text-gray-600">Editar pessoa</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar pessoa</h1>
        
        {isLoading ? (
          <div className="space-y-4 max-w-2xl">
            {[...Array(4)].map((_, i) => (
              <div key={i} className="h-14 bg-gray-100 rounded-xl animate-pulse"/>
            ))}
          </div>
        ) : !person ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Pessoa não encontrada.
          </div>
        ) : (
          <PersonForm
            initialData={person}
            onSubmit={async (d) => {
              await updateMutation.mutateAsync(d)
            }}
            loading={updateMutation.isPending}
          />
        )}
        {updateMutation.error && (
          <p className="mt-4 text-sm text-red-600">{updateMutation.error.message}</p>
        )}
      </div>
    </RootLayout>
  )
}

export default function EditPersonPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditPersonContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
