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
import {ImportDeclarationForm} from '@/components/import-declarations/ImportDeclarationForm'
import type {ImportDeclarationCreate} from '@/lib/types/api'

function EditImportDeclarationContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: unit, isLoading} = useQuery({
    queryKey: queryKeys.importDeclarations.detail(id),
    queryFn: () => apiClient.getImportDeclaration(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ImportDeclarationCreate) => apiClient.updateImportDeclaration(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.importDeclarations.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.importDeclarations.detail(id)})
      router.push('/import-declarations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/import-declarations" className="hover:text-brand-600">Declarações de importação</Link>
          <span>/</span>
          <span className="text-gray-600">Editar declaração</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar declaração</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !unit ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Declaração de importação não encontrada.
          </div>
        ) : (
          <ImportDeclarationForm
            initialData={unit}
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

export default function EditImportDeclarationPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditImportDeclarationContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
