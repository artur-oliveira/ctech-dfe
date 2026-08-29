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
import {TP_VIA_TRANSP_OPTIONS} from '@/lib/schemas/import-declarations'
import type {ImportDeclarationItemOut} from '@/lib/types/api'

const viaLabel = (code: unknown) =>
  TP_VIA_TRANSP_OPTIONS.find((o) => o.value === code)?.label ?? '—'

function ImportDeclarationsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ImportDeclarationItemOut>({
      queryKey: queryKeys.importDeclarations.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getImportDeclarations({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ImportDeclarationItemOut>({
    mutationFn: (id) => apiClient.deleteImportDeclaration(id),
    getId: (u) => extractId(u.sk, SK_PREFIX.IMPORT_DI),
    getDeletedMessage: (u) => `"${u.name}" excluída`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.importDeclarations.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Declarações de importação"
          description="Uma DI cobre várias notas e vários itens; na emissão o item só aponta qual adição o representa"
          action={selectedOrg ? {
            label: 'Nova declaração',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/import-declarations/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhuma declaração de importação"
            description="A DI guarda número, desembaraço, via de transporte e adições. Na emissão o item aponta a adição, e nAdicao/nSeqAdic saem do vínculo."
            action={{label: 'Nova declaração', onClick: () => router.push('/import-declarations/new')}}
            icon={<ImportIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Declarações de importação"
            minWidth={480}
            headers={['Nome', 'Nº da DI', 'Via de transporte', 'Adições', {label: '', align: 'right'}]}
          >
            {visibleItems.map((u) => (
              <tr key={u.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{u.name}</td>
                <td data-label="Nº da DI" className={`${TABLE_CELL} text-gray-600`}>{u.n_di}</td>
                <td data-label="Via de transporte" className={`${TABLE_CELL} text-gray-600`}>
                  {viaLabel(u.tp_via_transp)}
                </td>
                <td data-label="Adições" className={`${TABLE_CELL} text-gray-600`}>
                  {Array.isArray(u.additions) ? u.additions.length : 0}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/import-declarations/edit?id=${extractId(u.sk, SK_PREFIX.IMPORT_DI)}`)}
                            className="text-brand-600 hover:text-brand-700">
                      Editar
                    </Button>
                    <Button variant="ghost" size="xs" onClick={() => handleDelete(u)} disabled={isDeleting}
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

export default function ImportDeclarationsPage() {
  return (
    <ProtectedRoute>
      <ImportDeclarationsContent/>
    </ProtectedRoute>
  )
}
