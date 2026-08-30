'use client'

import Link from 'next/link'
import {useRouter} from 'next/navigation'
import {useMutation, useQueryClient} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {ReferenceDocumentForm} from '@/components/reference-documents/ReferenceDocumentForm'
import type {ReferenceDocumentCreate} from '@/lib/types/api'

function NewReferenceDocumentsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const createMutation = useMutation({
    mutationFn: (d: ReferenceDocumentCreate) => apiClient.createReferenceDocument(d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.referenceDocuments.list(selectedOrg?.pk)})
      router.push('/reference-documents')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/reference-documents" className="hover:text-brand-600">Documentos referenciados</Link>
          <span>/</span>
          <span className="text-gray-600">Novo documento</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Novo documento</h1>
        <ReferenceDocumentForm
          onSubmit={async (d) => {
            await createMutation.mutateAsync(d)
          }}
          loading={createMutation.isPending}
        />
      </div>
    </RootLayout>
  )
}

export default function NewReferenceDocumentsPage() {
  return (
    <ProtectedRoute>
      <NewReferenceDocumentsContent/>
    </ProtectedRoute>
  )
}
