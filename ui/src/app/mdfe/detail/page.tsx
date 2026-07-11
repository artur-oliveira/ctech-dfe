'use client'

import {Suspense, useState} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {Button} from '@/components/ui/button'
import {MdfeStatusBadge} from '@/components/mdfe/MdfeStatusBadge'
import {useMdfeActions} from '@/components/mdfe/MdfeActions'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {formatDatetimeBR, triggerDownload} from '@/lib/utils/dfe'
import type {NfeEventOut} from '@/lib/types/api'

const MDFE_EVENT_LABELS: Record<string, string> = {
  '110111': 'Cancelamento',
  '110112': 'Encerramento',
  '110114': 'Inclusão de Condutor',
  '110115': 'Inclusão de DF-e',
  '110116': 'Pagamento da Operação',
}

const EVENT_STATUS_CLASSES: Record<string, string> = {
  pending: 'bg-amber-50 text-amber-700',
  processing: 'bg-blue-50 text-blue-700',
  success: 'bg-green-100 text-green-700',
  rejected: 'bg-red-100 text-red-700',
  failed: 'bg-red-200 text-red-800',
  retry: 'bg-orange-100 text-orange-700',
}

const EVENT_STATUS_LABELS: Record<string, string> = {
  pending: 'Pendente',
  processing: 'Processando',
  success: 'Registrado',
  rejected: 'Rejeitado',
  failed: 'Falha',
  retry: 'Tentando novamente',
}

function InfoCard({label, children}: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">{label}</p>
      {children}
    </div>
  )
}

function MdfeDetail({accessKey}: { accessKey: string }) {
  const {selectedOrg} = useAuth()
  const [xmlLoading, setXmlLoading] = useState(false)
  const [eventXmlLoading, setEventXmlLoading] = useState<string | null>(null)
  const {openCancel, openClose, modals} = useMdfeActions(selectedOrg?.pk)

  const {data: doc, isLoading, error} = useQuery({
    queryKey: queryKeys.mdfes.detail(accessKey),
    queryFn: () => apiClient.getMdfe(accessKey),
    enabled: !!accessKey && !!selectedOrg,
  })

  const {data: eventsData, isLoading: eventsLoading} = useQuery({
    queryKey: queryKeys.mdfes.events(accessKey),
    queryFn: () => apiClient.getMdfeEvents(accessKey),
    enabled: !!accessKey && !!selectedOrg,
  })

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      triggerDownload(await apiClient.downloadMdfeXml(accessKey), `${accessKey}.xml`)
    } finally {
      setXmlLoading(false)
    }
  }

  const handleDownloadEventXml = async (event: NfeEventOut) => {
    setEventXmlLoading(event.sk)
    try {
      triggerDownload(await apiClient.downloadMdfeEventXml(accessKey, event.sk),
        `${event.event_type}-${event.sequence_number || 1}-${accessKey}.xml`)
    } finally {
      setEventXmlLoading(null)
    }
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        {[...Array(3)].map((_, i) => <div key={i} className="h-24 bg-gray-100 rounded-xl animate-pulse"/>)}
      </div>
    )
  }
  if (error || !doc) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        MDF-e não encontrado.
      </div>
    )
  }

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Header */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <p className="text-2xl font-semibold text-gray-900">
            MDF-e {doc.number}
            <span className="ml-2 text-base font-normal text-gray-400">série {doc.serie}</span>
          </p>
          <p className="text-xs text-gray-400 font-mono mt-1 break-all">{accessKey}</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <MdfeStatusBadge status={doc.status}/>
          {doc.xml_s3_key && (
            <Button variant="outline" size="sm" onClick={handleDownloadXml} disabled={xmlLoading}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              {xmlLoading ? 'Baixando…' : 'XML'}
            </Button>
          )}
          {(doc.status === 'authorized' || doc.status === 'closed' || doc.status === 'cancelled') && (
            <DownloadPdfButton fetchPdf={() => apiClient.downloadMdfeDamdfe(accessKey)}
                               filename={accessKey} label="DAMDFE" variant="outline" size="sm"
                               className="text-brand-600 border-brand-200 hover:bg-brand-50"/>
          )}
          {doc.status === 'authorized' && (
            <>
              <Button variant="outline" size="sm"
                      onClick={() => openClose({sk: accessKey, number: doc.number, uf_end: doc.uf_end})}
                      className="text-blue-600 border-blue-200 hover:bg-blue-50">
                Encerrar
              </Button>
              <Button variant="outline" size="sm"
                      onClick={() => openCancel({sk: accessKey, number: doc.number, uf_end: doc.uf_end})}
                      className="text-red-600 border-red-200 hover:bg-red-50">
                Cancelar
              </Button>
            </>
          )}
        </div>
      </div>

      {/* SEFAZ info */}
      {(doc.sefaz_status || doc.sefaz_motive) && (
        <div className={`rounded-lg border px-4 py-3 text-sm ${
          doc.status === 'authorized' || doc.status === 'closed' ? 'border-green-200 bg-green-50'
            : doc.status === 'rejected' || doc.status === 'failed' ? 'border-red-200 bg-red-50'
              : 'border-gray-200 bg-gray-50'}`}>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-1.5">
            {doc.status === 'rejected' || doc.status === 'failed' ? 'Rejeição' : 'Autorização'}
          </p>
          <div className="space-y-0.5">
            {doc.sefaz_status && <p className="font-mono text-xs text-gray-500">Código: {doc.sefaz_status}</p>}
            {doc.sefaz_motive && <p className="text-gray-700 whitespace-pre-wrap wrap-break-word">{doc.sefaz_motive}</p>}
            {doc.sefaz_protocol && <p className="font-mono text-xs text-gray-400">Protocolo: {doc.sefaz_protocol}</p>}
          </div>
        </div>
      )}

      {/* Trajeto + carga */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <InfoCard label="Trajeto">
          <p className="font-medium text-gray-900">
            {doc.uf_start} <span className="text-gray-400 mx-1">→</span> {doc.uf_end}
          </p>
          {doc.route && doc.route.length > 0 && (
            <p className="text-xs text-gray-500">Percurso: {doc.route.join(' · ')}</p>
          )}
        </InfoCard>
        <InfoCard label="Carga">
          <p className="font-medium text-gray-900">{formatCurrency(doc.cargo_value)}</p>
          <p className="text-xs text-gray-500">Peso: {doc.cargo_weight} kg</p>
          {doc.predominant?.x_prod && (
            <p className="text-xs text-gray-500">Predominante: {doc.predominant.x_prod}</p>
          )}
        </InfoCard>
      </div>

      {/* Emitente */}
      <InfoCard label="Emitente">
        <p className="font-medium text-gray-900">{doc.emit_name}</p>
        <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.emit_cpf_cnpj)}</p>
      </InfoCard>

      {/* Veículo + condutores */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        {doc.vehicle && (
          <InfoCard label="Veículo">
            <p className="font-medium text-gray-900">{doc.vehicle.placa} · {doc.vehicle.uf}</p>
            <p className="text-xs text-gray-500">Tara: {doc.vehicle.tara} kg</p>
            {doc.vehicle.rntrc && <p className="text-xs text-gray-500">RNTRC: {doc.vehicle.rntrc}</p>}
          </InfoCard>
        )}
        {doc.drivers && doc.drivers.length > 0 && (
          <InfoCard label="Condutores">
            {doc.drivers.map((c) => (
              <p key={c.cpf} className="text-sm text-gray-700">
                {c.name} <span className="font-mono text-xs text-gray-400">{formatCpfCnpj(c.cpf)}</span>
              </p>
            ))}
          </InfoCard>
        )}
      </div>

      {/* Documentos */}
      {doc.documents && doc.documents.length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden">
          <p className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">
            Documentos ({doc.documents.length})
          </p>
          <div className="divide-y divide-gray-100">
            {doc.documents.map((d) => (
              <div key={d.access_key} className="flex items-center gap-2 px-4 py-2.5 text-sm">
                <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500 uppercase">{d.type}</span>
                <span className="font-mono text-xs text-gray-500 truncate">{d.access_key}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Eventos */}
      <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
        <p className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">
          Eventos
        </p>
        {eventsLoading ? (
          <div className="divide-y divide-gray-100">
            {[0, 1].map((i) => (
              <div key={i} className="flex items-center gap-4 px-4 py-3">
                <div className="h-4 w-32 bg-gray-100 rounded animate-pulse"/>
                <div className="ml-auto h-4 w-20 bg-gray-100 rounded animate-pulse"/>
              </div>
            ))}
          </div>
        ) : !eventsData?.items.length ? (
          <p className="px-4 py-4 text-sm text-gray-400">Nenhum evento registrado.</p>
        ) : (
          <table className="w-full text-sm min-w-120">
            <thead className="bg-gray-50 border-b border-gray-100">
            <tr>
              {['Evento', 'Seq.', 'Status', 'Data', ''].map((h) => (
                <th key={h} className="px-4 py-2.5 text-left text-xs font-semibold text-gray-500">{h}</th>
              ))}
            </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
            {eventsData.items.map((evt) => (
              <tr key={evt.sk} className="hover:bg-gray-50 transition-colors align-top">
                <td className="px-4 py-3">
                  <p className="font-medium text-gray-900">{MDFE_EVENT_LABELS[evt.event_type] ?? evt.event_type}</p>
                  {evt.sefaz_motive && (
                    <p className="text-xs text-gray-400 mt-0.5 max-w-65 wrap-break-word">{evt.sefaz_motive}</p>
                  )}
                </td>
                <td className="px-4 py-3 text-gray-500 font-mono text-xs">{String(evt.sequence_number).padStart(3, '0')}</td>
                <td className="px-4 py-3">
                  <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${EVENT_STATUS_CLASSES[evt.status] ?? 'bg-gray-100 text-gray-600'}`}>
                    {EVENT_STATUS_LABELS[evt.status] ?? evt.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-xs text-gray-400 whitespace-nowrap">{formatDatetimeBR(evt.created_at)}</td>
                <td className="px-4 py-3 text-right">
                  {evt.xml_s3_key && (
                    <Button variant="ghost" size="xs" onClick={() => handleDownloadEventXml(evt)}
                            disabled={eventXmlLoading === evt.sk} className="text-brand-600 hover:text-brand-700">
                      {eventXmlLoading === evt.sk ? 'Baixando…' : 'XML'}
                    </Button>
                  )}
                </td>
              </tr>
            ))}
            </tbody>
          </table>
        )}
      </div>

      <p className="text-xs text-gray-400">Emissão: {formatDate(doc.year, doc.month, doc.day)}</p>

      {modals}
    </div>
  )
}

function MdfeDetailContent() {
  const params = useSearchParams()
  const accessKey = params.get('key') ?? ''

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/mdfe" className="hover:text-brand-600">MDF-e</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate max-w-[200px]">{accessKey || 'Detalhe'}</span>
        </div>
        {accessKey ? (
          <MdfeDetail accessKey={accessKey}/>
        ) : (
          <p className="text-sm text-gray-500">Chave de acesso não informada.</p>
        )}
      </div>
    </RootLayout>
  )
}

export default function MdfeDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <MdfeDetailContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
