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
import {BriefcaseIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {SERVICE_LOCATION_ROLES} from '@/lib/schemas/service-locations'
import type {ServiceLocationItemOut} from '@/lib/types/api'

function str(v: unknown): string {
  return typeof v === 'string' && v ? v : '—'
}

/** Rótulos dos papéis, na ordem do cadastro. */
function roleLabels(roles: unknown): string {
  if (!Array.isArray(roles) || roles.length === 0) return '—'
  return SERVICE_LOCATION_ROLES
    .filter((r) => roles.includes(r.value))
    .map((r) => r.label)
    .join(', ')
}

/**
 * Município do endereço nacional, ou cidade e região no exterior. O código IBGE
 * é resolvido para o nome: quem lê a lista não decora tabela do IBGE.
 */
function where(address: unknown): string {
  if (!address || typeof address !== 'object') return '—'
  const a = address as Record<string, unknown>
  if (typeof a.foreign_city === 'string' && a.foreign_city) {
    return [a.foreign_city, a.foreign_region].filter(Boolean).join(' / ')
  }
  const code = str(a.city_ibge_code)
  return CITY_OPTIONS.find((c) => c.value === code)?.label ?? code
}

function ServiceLocationsContent() {
  const {selectedOrg} = useAuth()
  const router = useRouter()
  const qc = useQueryClient()

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious, reset} =
    usePagination<ServiceLocationItemOut>({
      queryKey: queryKeys.serviceLocations.list(selectedOrg?.pk),
      queryFn: (cursor) => apiClient.getServiceLocations({cursor}),
      enabled: !!selectedOrg,
    })

  const {handleDelete, filterVisible, isPending: isDeleting} = useEntityDelete<ServiceLocationItemOut>({
    mutationFn: (id) => apiClient.deleteServiceLocation(id),
    getId: (e) => extractId(e.sk, SK_PREFIX.SERVICE_LOCATION),
    getDeletedMessage: (e) => `"${e.name}" excluído`,
    onSuccess: () => {
      reset()
      void qc.invalidateQueries({queryKey: queryKeys.serviceLocations.list(selectedOrg?.pk)})
    },
  })

  const visibleItems = filterVisible(items)

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Locais de prestação"
          description="Obra, imóvel e local de evento cadastrados uma vez; na emissão a nota só aponta qual local usar"
          action={selectedOrg ? {
            label: 'Novo local',
            icon: <span className="text-base leading-none">+</span>,
            onClick: () => router.push('/service-locations/new'),
          } : undefined}
        />

        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : isLoading ? (
          <LoadingSkeleton/>
        ) : visibleItems.length === 0 ? (
          <EmptyState
            title="Nenhum local de prestação"
            description="O mesmo endereço serve obra, imóvel e local de evento — os papéis são combináveis, então um canteiro que também é o imóvel tributado é um cadastro só."
            action={{label: 'Novo local', onClick: () => router.push('/service-locations/new')}}
            icon={<BriefcaseIcon width={20} height={20}/>}
          />
        ) : (
          <TableShell
            ariaLabel="Locais de prestação"
            minWidth={520}
            headers={['Nome', 'Papéis', 'Endereço', 'Município', {label: '', align: 'right'}]}
          >
            {visibleItems.map((e) => (
              <tr key={e.sk} className={TABLE_ROW}>
                <td data-label="Nome" className={`${TABLE_CELL} font-medium text-gray-900`}>{e.name}</td>
                <td data-label="Papéis" className={`${TABLE_CELL} text-gray-600`}>{roleLabels(e.roles)}</td>
                <td data-label="Endereço" className={`${TABLE_CELL} text-gray-600`}>
                  {str((e.address as Record<string, unknown> | undefined)?.street)}
                </td>
                <td data-label="Município" className={`${TABLE_CELL} text-gray-600`}>{where(e.address)}</td>
                <td className={`${TABLE_CELL} text-right`}>
                  <div className="flex items-center justify-end gap-1">
                    <Button variant="ghost" size="xs"
                            onClick={() => router.push(`/service-locations/edit?id=${extractId(e.sk, SK_PREFIX.SERVICE_LOCATION)}`)}
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

export default function ServiceLocationsPage() {
  return (
    <ProtectedRoute>
      <ServiceLocationsContent/>
    </ProtectedRoute>
  )
}
