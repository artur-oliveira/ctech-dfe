'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ImportDeclarationForm} from '@/components/import-declarations/ImportDeclarationForm'
import type {ImportDeclarationCreate} from '@/lib/types/api'

function NewImportDeclarationContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: ImportDeclarationCreate) => apiClient.createImportDeclaration(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.importDeclarations.list(selectedOrg?.pk)})
      router.push('/import-declarations')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/import-declarations" className="hover:text-brand-600">Declarações de importação</Link>
          <span>/</span>
          <span className="text-gray-600">Nova declaração</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Nova declaração</h1>
        <ImportDeclarationForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewImportDeclarationPage() {
  return (
    <ProtectedRoute>
      <NewImportDeclarationContent/>
    </ProtectedRoute>
  )
}
