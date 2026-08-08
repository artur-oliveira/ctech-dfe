'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {apiClient} from '@/lib/api/client'
import {usePagination} from '@/lib/hooks/usePagination'
import {queryKeys} from '@/lib/api/query-keys'
import {EmptyState} from '@/components/ui/empty-state'
import {ServiceIcon} from '@/components/ui/icon'
import {Pagination} from '@/components/ui/pagination'
import {Button} from '@/components/ui/button'
import {DistributionSkeleton} from '@/components/ui/loading-skeleton'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import type {NfseDistributionOut} from '@/lib/types/api'
import {formatDatetimeBR, formatNsu, triggerDownload} from '@/lib/utils/dfe'
import {EVENT_LABELS} from '@/lib/schemas/nfse'

interface NfseDistributionTabProps {
  docType: 'nfse'
  orgPk: string
}

function DistributionRow({item, docType}: { item: NfseDistributionOut; docType: 'nfse' }) {
  const [xmlLoading, setXmlLoading] = useState(false)

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      triggerDownload(await apiClient.downloadDistributionXml(docType, item.nsu), `NSU_${formatNsu(item.nsu)}.xml`)
    } catch {
      toast.error('Não foi possível baixar o XML.')
    } finally {
      setXmlLoading(false)
    }
  }

  return (
    <tr className={TABLE_ROW}>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="NSU">{formatNsu(item.nsu)}</td>
      <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="Chave de acesso">
        {item.access_key ?? '—'}
      </td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Tipo">{item.schema_type}</td>
      <td className={`${TABLE_CELL} text-sm text-gray-700`} data-label="Evento">
        {item.event_type ? (EVENT_LABELS[item.event_type] ?? item.event_type) : '—'}
      </td>
      <td className={`${TABLE_CELL} whitespace-nowrap text-xs text-gray-500`} data-label="Recebido em">
        {formatDatetimeBR(item.created_at)}
      </td>
      <td className={`${TABLE_CELL} text-right`} data-label="Ações">
        {item.xml_s3_key && (
          <Button variant="ghost" size="xs" onClick={handleDownloadXml} disabled={xmlLoading}
                  className="text-brand-700">
            {xmlLoading ? 'Baixando…' : 'XML'}
          </Button>
        )}
      </td>
    </tr>
  )
}

export function NfseDistributionTab({docType, orgPk}: NfseDistributionTabProps) {
  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} = usePagination<NfseDistributionOut>({
    queryKey: queryKeys.distributions.history(docType, orgPk),
    // NFS-e usa a rota ADN dedicada; docType ainda parametriza cache, download e shell.
    queryFn: (cursor) => apiClient.listNfseDistributions({limit: 10, cursor}),
    enabled: !!orgPk,
  })

  return (
    <div className="space-y-4">
      <p className="text-sm text-gray-500">
        NFS-e recebidas pela organização via Ambiente de Dados Nacional (ADN).
      </p>
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
              description="A distribuição roda periodicamente. As NFS-e recebidas aparecerão aqui."
              icon={<ServiceIcon width={20} height={20}/>}
            />
          </td></tr>
        ) : items.map((item) => <DistributionRow key={item.nsu} item={item} docType={docType}/>) }
      </TableShell>

      {(hasNext || hasPrevious) && (
        <Pagination hasNext={hasNext} hasPrevious={hasPrevious} onNext={goNext} onPrevious={goPrevious}
                    isLoading={isFetching}/>
      )}
    </div>
  )
}
