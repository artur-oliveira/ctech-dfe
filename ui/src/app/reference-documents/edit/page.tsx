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
import {ReferenceDocumentForm} from '@/components/reference-documents/ReferenceDocumentForm'
import type {ReferenceDocumentCreate} from '@/lib/types/api'

function EditReferenceDocumentsContent() {
  const params = useSearchParams()
  const id = params.get('id') ?? ''
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {data: entity, isLoading} = useQuery({
    queryKey: queryKeys.referenceDocuments.detail(id),
    queryFn: () => apiClient.getReferenceDocument(id),
    enabled: !!id && !!selectedOrg,
  })

  const updateMutation = useMutation({
    mutationFn: (d: ReferenceDocumentCreate) => apiClient.updateReferenceDocument(id, d),
    onSuccess: () => {
      void qc.invalidateQueries({queryKey: queryKeys.referenceDocuments.list(selectedOrg?.pk)})
      void qc.invalidateQueries({queryKey: queryKeys.referenceDocuments.detail(id)})
      router.push('/reference-documents')
    },
  })

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/reference-documents" className="hover:text-brand-600">Documentos referenciados</Link>
          <span>/</span>
          <span className="text-gray-600">Editar documento</span>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900 mb-6">Editar documento</h1>

        {isLoading ? (
          <LoadingSkeleton/>
        ) : !entity ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Documento referenciado não encontrado.
          </div>
        ) : (
          <ReferenceDocumentForm
            initialData={entity}
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

export default function EditReferenceDocumentsPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <EditReferenceDocumentsContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
