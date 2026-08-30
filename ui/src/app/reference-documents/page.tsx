'use client'

import {useQueryClient} from '@tanstack/react-query'
import {useRouter} from 'next/navigation'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {useEntityDelete} from '@/lib/hooks/useEntityDelete'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {ImportIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {formatISODateBR} from '@/lib/utils/dfe'
import {REFERENCE_DOCUMENT_KINDS} from '@/lib/schemas/reference-documents'
import type {ReferenceDocumentItemOut} from '@/lib/types/api'

/** Rótulo da família documental; o código cru nunca vai para a tela. */
function kindLabel(kind: unknown): string {
  return REFERENCE_DOCUMENT_KINDS.find((k) => k.value === kind)?.label ?? '—'
}

function date(v: unknown): string {
  return typeof v === 'string' && v ? formatISODateBR(v) : '—'
}

function ReferenceDocumentsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ReferenceDocumentItemOut>({
      queryKey: queryKeys.referenceDocuments.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getReferenceDocuments({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ReferenceDocumentItemOut>({
    mutationFn: (id) => apiClient.deleteReferenceDocument(id),
    getId: (e) => extractId(e.sk, SK_PREFIX.REFERENCE_DOCUMENT),
    getDeletedMessage: (e) => `"${e.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.referenceDocuments.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Documentos referenciados"
          description="Documentos citados em dedução, redução, reembolso, repasse e ressarcimento da NFS-e"
          action={selectedOrg ? {
            label: 'Novo documento',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/reference-documents/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum documento referenciado"
            description="O mesmo cadastro alimenta a dedução/redução e o reembolso/repasse/ressarcimento: o leiaute pede formas diferentes do mesmo documento nos dois grupos."
            action={{label: 'Novo documento', onClick: () => router.push('/reference-documents/new')}}
            icon={<ImportIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Documentos referenciados"
            minWidth={520}
            headers={['Nome', 'Tipo', 'Emissão', 'Competência', {label: '', align: 'right'}]}
          >
            {visibleItems.map((e) => (
              <tr key={e.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{e.name}</td>
                <td data-label="Tipo" className={`${TABLE_CELL} text-gray-600`}>{kindLabel(e.kind)}</td>
                <td data-label="Emissão" className={`${TABLE_CELL} text-gray-600`}>{date(e.issued_at)}</td>
                <td data-label="Competência" className={`${TABLE_CELL} text-gray-600`}>{date(e.competence_at)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/reference-documents/edit?id=${extractId(e.sk, SK_PREFIX.REFERENCE_DOCUMENT)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(e)} disabled={isDeleting}
                            className="text-danger hover:text-red-700">
                      Excluir
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </TableShell>
        )}
        <Pagination
          hasNext={hasNext}
          hasPrevious={hasPrevious}
          onNext={goNext}
          onPrevious={goPrevious}
          isLoading={isFetching}
        />
      </div>
    </RootLayout>
  )
}

export default function ReferenceDocumentsPage() {
  return (
    <ProtectedRoute>
      <ReferenceDocumentsContent/>
    </ProtectedRoute>
  )
}
