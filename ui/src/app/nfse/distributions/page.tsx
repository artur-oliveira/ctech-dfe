'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {EmptyState} from '@/components/ui/empty-state'
import {ServiceIcon} from '@/components/ui/icon'
import {NoOrgBanner} from '@/components/ui/no-org-banner'
import {Pagination} from '@/components/ui/pagination'
import {Button} from '@/components/ui/button'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import type {NfseDistributionOut} from '@/lib/types/api'
import {formatDatetimeBR, formatNsu, triggerDownload} from '@/lib/utils/dfe'
import {EVENT_LABELS} from '@/lib/schemas/nfse'

function DistributionRow({item}: { item: NfseDistributionOut }) {
  const [xmlLoading, setXmlLoading] = useState(false)

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      triggerDownload(await apiClient.downloadDistributionXml('nfse', item.nsu), `NSU_${formatNsu(item.nsu)}.xml`)
    } catch {
      toast.error('Erro ao baixar XML.')
    } finally {
      setXmlLoading(false)
    }
  }

  return (
    <tr className={TABLE_ROW}>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="NSU">{formatNsu(item.nsu)}</td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-400`} data-label="Chave de acesso">
        {item.access_key ?? <span className="text-gray-300">—</span>}
      </td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Tipo">{item.schema_type}</td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Evento">
        {item.event_type ? (EVENT_LABELS[item.event_type] ?? item.event_type) : <span className="text-gray-300">—</span>}
      </td>
      <td className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`} data-label="Recebido em">
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`}>
        {item.xml_s3_key && (
          <Button variant="ghost" size="xs" onClick={handleDownloadXml} disabled={xmlLoading}
                  className="text-brand-600 hover:text-brand-700">
            {xmlLoading ? 'Baixando…' : 'XML'}
          </Button>
        )}
      </td>
    </tr>
  )
}

function NfseDistributionsContent() {
  const {selectedOrg} = useAuth()
  const orgPk = selectedOrg?.pk

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NfseDistributionOut>({
    queryKey: queryKeys.nfseDistributions(orgPk),
    queryFn: (cursor) => apiClient.listNfseDistributions({limit: 10, cursor}),
    enabled: !!orgPk,
  })

  if (!selectedOrg) {
    return (
      <RootLayout>
        <div className="p-4 md:p-8">
          <NoOrgBanner/>
        </div>
      </RootLayout>
    )
  }

  return (
    <RootLayout>
      <div className="p-4 md:p-8 space-y-6">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Distribuição NFS-e</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            Notas de Serviço emitidas contra o CNPJ da organização, recebidas via ADN
          </p>
        </div>

        <TableShell
          ariaLabel="Distribuição de NFS-e"
          minWidth={480}
          dimmed={isFetching}
          headers={['NSU', 'Chave de acesso', 'Tipo', 'Evento', 'Recebido em', {label: '', align: 'right'}]}
        >
          {isLoading ? (
            <tr><td className={TABLE_CELL} colSpan={6}><DistributionSkeleton/></td></tr>
          ) : items.length === 0 ? (
            <tr><td className={TABLE_CELL} colSpan={6}>
              <EmptyState
                title="Nenhum documento recebido"
                description="A distribuição roda periodicamente. Documentos emitidos contra o CNPJ da organização aparecerão aqui."
                icon={<ServiceIcon width={20} height={20}/>}
              />
            </td></tr>
          ) : (
            items.map((item) => <DistributionRow key={item.nsu} item={item}/>)
          )}
        </TableShell>

        {(hasNext || hasPrevious) && (
          <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious} isLoading={isFetching}/>
        )}
      </div>
    </RootLayout>
  )
}

export default function NfseDistributionsPage() {
  return (
    <ProtectedRoute>
      <NfseDistributionsContent/>
    </ProtectedRoute>
  )
}
