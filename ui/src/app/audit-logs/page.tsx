'use client'

import {useState} from 'react'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {PageHeader} from '@/components/ui/page-header'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'
import {formatDatetimeBR} from '@/lib/utils/dfe'
import type {AuditLogOut} from '@/lib/types/api'

const RESOURCE_TYPE_OPTIONS = [
  {value: 'ORGANIZATION', label: 'Organização'},
  {value: 'CERTIFICATE', label: 'Certificado'},
  {value: 'PRODUCT', label: 'Produto'},
  {value: 'VEHICLE', label: 'Veículo'},
  {value: 'PERSON', label: 'Pessoa'},
  {value: 'NFE_CONFIG', label: 'Configuração NF-e'},
  {value: 'NFCE_CONFIG', label: 'Configuração NFC-e'},
  {value: 'CTE_CONFIG', label: 'Configuração CT-e'},
  {value: 'MDFE_CONFIG', label: 'Configuração MDF-e'},
]

const ACTION_LABELS: Record<string, string> = {
  CREATE: 'Criação',
  UPDATE: 'Alteração',
  DELETE: 'Exclusão',
}

const ACTION_BADGE_CLASSES: Record<string, string> = {
  CREATE: 'bg-emerald-50 text-emerald-700',
  UPDATE: 'bg-blue-50 text-blue-700',
  DELETE: 'bg-red-50 text-red-700',
}

function resourceTypeLabel(resourceType: string): string {
  return RESOURCE_TYPE_OPTIONS.find((o) => o.value === resourceType)?.label ?? resourceType
}

function formatValue(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

function AuditLogsContent() {
  const {selectedOrg} = useAuth()
  const [resourceType, setResourceType] = useState<string>('')
  const [selected, setSelected] = useState<AuditLogOut | null>(null)
  
  const isOwnerOrAdmin = selectedOrg?.role === 'OWNER' || selectedOrg?.role === 'ADMIN'
  
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} =
    usePagination<AuditLogOut>({
      queryKey: queryKeys.auditLogs.list(selectedOrg?.pk, {resourceType}),
      queryFn: (cursor) => apiClient.getAuditLogs({resourceType: resourceType || undefined, cursor}),
      enabled: !!selectedOrg && isOwnerOrAdmin,
    })
  
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <PageHeader
          title="Log de Auditoria"
          description="Histórico de alterações realizadas por usuários na organização"
        />
        
        {!selectedOrg ? (
          <NoOrgBanner/>
        ) : !isOwnerOrAdmin ? (
          <EmptyState
            title="Acesso restrito"
            description="Apenas proprietários e administradores podem ver o log de auditoria."
          />
        ) : (
          <>
            <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 mb-4">
              <div className="flex flex-col gap-1">
                <label htmlFor="audit-log-resource" className="text-xs font-medium text-gray-600">Recurso</label>
                <OptionsSelect
                  id="audit-log-resource"
                  value={resourceType || null}
                  onValueChange={setResourceType}
                  options={RESOURCE_TYPE_OPTIONS}
                  placeholder="Todos os recursos"
                  className="w-full"
                />
              </div>
            </div>
            
            {isLoading ? (
              <LoadingSkeleton/>
            ) : items.length === 0 ? (
              <EmptyState
                title="Nenhum registro de auditoria"
                description="Alterações em produtos, veículos, pessoas e configurações aparecerão aqui."
              />
            ) : (
              <TableShell
                ariaLabel="Logs de auditoria"
                minWidth={125}
                dimmed={isFetching}
                headers={['Data/Hora', 'Recurso', 'Ação', 'Usuário', {label: '', align: 'right'}]}
              >
                {items.map((log) => (
                  <tr key={log.sk} className={TABLE_ROW}>
                    <td
                      data-label="Data/Hora" className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`}>{formatDatetimeBR(log.created_at)}</td>
                    <td data-label="Recurso" className={`${TABLE_CELL} font-medium text-gray-900`}>
                      {resourceTypeLabel(log.resource_type)}
                      <span className="block text-xs text-gray-400 font-normal">{log.resource_id}</span>
                    </td>
                    <td data-label="Ação" className={TABLE_CELL}>
                      <span
                        className={`inline-flex px-2 py-0.5 rounded-md text-xs font-medium ${ACTION_BADGE_CLASSES[log.action] ?? 'bg-gray-100 text-gray-700'}`}>
                        {ACTION_LABELS[log.action] ?? log.action}
                      </span>
                    </td>
                    <td data-label="Usuário" className={`${TABLE_CELL} text-gray-700`}>{log.user_name}</td>
                    <td className={`${TABLE_CELL} text-right`}>
                      {log.modifications.length > 0 && (
                        <Button variant="ghost" size="xs" onClick={() => setSelected(log)}
                                className="text-brand-600 hover:text-brand-700">
                          Ver alterações
                        </Button>
                      )}
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
          </>
        )}
      </div>
      
      <Modal
        isOpen={!!selected}
        title="Alterações"
        onClose={() => setSelected(null)}
        cancelLabel="Fechar"
      >
        {selected && (
          <div className="space-y-3">
            {selected.modifications.map((mod) => (
              <div key={mod.name} className="border border-gray-200 rounded-lg p-3">
                <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1">{mod.name}</p>
                <div className="flex flex-col sm:flex-row gap-2 text-sm">
                  <span className="text-gray-500 line-through break-all">{formatValue(mod.before)}</span>
                  <span className="text-gray-900 break-all">{formatValue(mod.after)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Modal>
    </RootLayout>
  )
}

export default function AuditLogsPage() {
  return (
    <ProtectedRoute>
      <AuditLogsContent/>
    </ProtectedRoute>
  )
}
