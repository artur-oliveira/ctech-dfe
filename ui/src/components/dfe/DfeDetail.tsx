'use client'

import {type ReactNode, useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {CancelDfeModal, CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {type AuxiliaryDocumentDownload, displayPaymentTypeLabel, type NfeDetailOut, type NfeEventOut, type PaginatedResponse, type SignedFileDownload} from '@/lib/types/api'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency, formatDate} from '@/lib/utils/helpers'
import {formatDatetimeBR, triggerRemoteDownload} from '@/lib/utils/dfe'
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge'
import {ApiError} from '@/lib/api/client'
import {toast} from 'sonner'
import {EVENT_TYPE_LABELS} from "@/lib/data/dfe_event";
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'

const CANCEL_EVENT_TYPES = ['110111', '110112']

export interface DfeDetailProps {
  accessKey: string
  /** 'NF-e' | 'NFC-e' — used in titles and download filenames. */
  docLabel: string
  destLabel?: string
  enabled: boolean
  detailQueryKey: readonly unknown[]
  eventsQueryKey: readonly unknown[]
  listQueryKey: readonly unknown[]
  fetchDoc: () => Promise<NfeDetailOut>
  fetchEvents: () => Promise<PaginatedResponse<NfeEventOut>>
  cancelFn: (justification: string) => Promise<unknown>
  downloadXml: () => Promise<SignedFileDownload>
  downloadEventXml: (eventSk: string) => Promise<SignedFileDownload>
  downloadDanfe?: () => Promise<AuxiliaryDocumentDownload>
  /** Doc-specific header buttons (e.g. CC-e for NF-e, Substituir for NFC-e). */
  headerActions?: (doc: NfeDetailOut) => ReactNode
  /** Doc-specific extra content/modals rendered with the loaded doc. */
  renderExtra?: (doc: NfeDetailOut) => ReactNode
}

export function DfeDetail({
                            accessKey, docLabel, destLabel = 'Destinatário', enabled,
                            detailQueryKey, eventsQueryKey, listQueryKey,
                            fetchDoc, fetchEvents, cancelFn, downloadXml, downloadEventXml, downloadDanfe,
                            headerActions, renderExtra,
                          }: DfeDetailProps) {
  const qc = useQueryClient()
  const [showCancelModal, setShowCancelModal] = useState(false)
  const [justification, setJustification] = useState('')
  const [xmlLoading, setXmlLoading] = useState(false)
  const [danfeLoading, setDanfeLoading] = useState(false)
  const [eventXmlLoading, setEventXmlLoading] = useState<string | null>(null)

  const {data: doc, isLoading, error} = useQuery<NfeDetailOut>({
    queryKey: detailQueryKey,
    queryFn: fetchDoc,
    enabled,
  })

  const {data: eventsData, isLoading: eventsLoading} = useQuery({
    queryKey: eventsQueryKey,
    queryFn: fetchEvents,
    enabled,
  })

  const cancelMutation = useMutation({
    mutationFn: (j: string) => cancelFn(j),
    onSuccess: () => {
      setShowCancelModal(false)
      setJustification('')
      void qc.invalidateQueries({queryKey: detailQueryKey})
      void qc.invalidateQueries({queryKey: listQueryKey})
    },
  })

  const handleDownloadXml = async () => {
    setXmlLoading(true)
    try {
      triggerRemoteDownload((await downloadXml()).url)
    } finally {
      setXmlLoading(false)
    }
  }

  const handleDownloadEventXml = async (event: NfeEventOut) => {
    setEventXmlLoading(event.sk)
    try {
      triggerRemoteDownload((await downloadEventXml(event.sk)).url)
    } finally {
      setEventXmlLoading(null)
    }
  }

  const handleDownloadDanfe = async () => {
    if (!downloadDanfe) return
    setDanfeLoading(true)
    try {
      triggerRemoteDownload((await downloadDanfe()).url)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.detail : String(err));
    } finally {
      setDanfeLoading(false)
    }
  }

  if (isLoading) {
    return <LoadingSkeleton count={3} height="h-24" rounded="rounded-xl"/>
  }

  if (error || !doc) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        {docLabel} não encontrada.
      </div>
    )
  }

  const totalDiscount = (doc.products || []).reduce((s, p) => s + parseFloat(p.discount || '0'), 0)
  const hasXml = !!doc.xml_s3_key
  const isOwnEmission = doc.incoming === 0
  const canCancel = doc.status === 'authorized' && isOwnEmission
  const isCancelled = doc.status === 'cancelled'
  const cancelEvent = (eventsData?.items ?? []).filter(e => CANCEL_EVENT_TYPES.includes(e.event_type)).at(-1) ?? null

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Header */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <p className="text-2xl font-semibold text-gray-900">
            {docLabel} {doc.number}
            <span className="ml-2 text-base font-normal text-gray-400">série {doc.serie}</span>
          </p>
          <p className="text-xs text-gray-400 font-mono mt-1 break-all">{accessKey}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <DfeStatusBadge status={doc.status} size="md"/>

          {hasXml && (
            <>
              <Button variant="outline" size="sm" onClick={handleDownloadXml} disabled={xmlLoading}
                      className="text-brand-600 border-brand-200 hover:bg-brand-50">
                {xmlLoading ? 'Baixando…' : 'XML'}
              </Button>
              {downloadDanfe && (
                <Button variant="outline" size="sm" onClick={handleDownloadDanfe} disabled={danfeLoading}
                        className="text-brand-600 border-brand-200 hover:bg-brand-50">
                  {danfeLoading ? 'Gerando…' : 'DANFE'}
                </Button>
              )}
            </>
          )}

          {headerActions?.(doc)}

          {canCancel && (
            <Button variant="outline" size="sm"
                    onClick={() => {
                      setJustification('')
                      setShowCancelModal(true)
                    }}
                    className="text-red-600 border-red-200 hover:bg-red-50">
              Cancelar
            </Button>
          )}
        </div>
      </div>

      {/* SEFAZ info — full rejection/authorization motive (not truncated) */}
      {(doc.sefaz_status || doc.sefaz_motive) && (
        <div className={`rounded-lg border px-4 py-3 text-sm ${
          isCancelled ? 'border-gray-200 bg-gray-50 opacity-60'
            : doc.status === 'authorized' ? 'border-green-200 bg-green-50'
              : doc.status === 'rejected' || doc.status === 'failed' ? 'border-red-200 bg-red-50'
                : 'border-gray-200 bg-gray-50'}`}>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-1.5">
            {doc.status === 'rejected' || doc.status === 'failed' ? 'Rejeição' : 'Autorização'}
          </p>
          <div className={`space-y-0.5 ${isCancelled ? 'line-through' : ''}`}>
            {doc.sefaz_status && <p className="font-mono text-xs text-gray-500">Código: {doc.sefaz_status}</p>}
            {doc.sefaz_motive &&
                <p className="text-gray-700 whitespace-pre-wrap wrap-break-word">{doc.sefaz_motive}</p>}
            {doc.sefaz_protocol && <p className="font-mono text-xs text-gray-400">Protocolo: {doc.sefaz_protocol}</p>}
          </div>
        </div>
      )}

      {/* Cancellation event */}
      {isCancelled && cancelEvent && (
        <div className="rounded-lg border border-gray-300 bg-white px-4 py-3 text-sm">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-1.5">
            {EVENT_TYPE_LABELS[cancelEvent.event_type] ?? 'Cancelamento'}
          </p>
          <div className="space-y-0.5">
            {cancelEvent.sefaz_status &&
                <p className="font-mono text-xs text-gray-500">Código: {cancelEvent.sefaz_status}</p>}
            {cancelEvent.sefaz_motive &&
                <p className="text-gray-700 whitespace-pre-wrap wrap-break-word">{cancelEvent.sefaz_motive}</p>}
            {cancelEvent.sefaz_protocol &&
                <p className="font-mono text-xs text-gray-400">Protocolo do evento: {cancelEvent.sefaz_protocol}</p>}
            <div className="pt-0.5">
              <DfeStatusBadge status={cancelEvent.status} gender="m"/>
            </div>
          </div>
        </div>
      )}

      {/* Emitente / Destinatário */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Emitente</p>
          <p className="font-medium text-gray-900">{doc.emit_name}</p>
          <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.emit_cpf_cnpj)}</p>
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">{destLabel}</p>
          <p
            className="font-medium text-gray-900">{doc.dest_name || (doc.dest_cpf_cnpj ? '—' : 'Consumidor não identificado')}</p>
          {doc.dest_cpf_cnpj && <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.dest_cpf_cnpj)}</p>}
        </div>
      </div>

      {/* Produtos (sem NCM/Código) */}
      {doc.products !== null && doc.products !== undefined && doc.products.length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden overflow-x-auto">
          <p
            className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">
            Produtos ({doc.products.length})
          </p>
          <TableShell
            ariaLabel="Resumo do documento"
            minWidth={120}
            headers={['Descrição', 'CFOP', 'Qtd', 'Vl. unit.', 'Desconto', 'Total']}
          >
            {doc.products.map((p, i) => (
              <tr key={i} className={TABLE_ROW}>
                <td data-label="Descrição" className={`${TABLE_CELL} text-gray-900`}>{p.description}</td>
                <td data-label="CFOP" className={`${TABLE_CELL} font-mono text-xs text-gray-500`}>{p.cfop}</td>
                <td data-label="Qtd" className={`${TABLE_CELL} text-gray-700 whitespace-nowrap`}>{p.quantity} {p.unit}</td>
                <td data-label="Vl. unit." className={`${TABLE_CELL} text-gray-700`}>{formatCurrency(p.unit_value)}</td>
                <td data-label="Desconto" className={`${TABLE_CELL} text-gray-700`}>{formatCurrency(p.discount)}</td>
                <td data-label="Total" className={`${TABLE_CELL} font-medium text-gray-900`}>{formatCurrency(p.total)}</td>
              </tr>
            ))}
          </TableShell>
          <div className="px-4 py-3 border-t border-gray-100 text-sm text-right space-x-6 text-gray-500">
            {totalDiscount > 0 && (
              <span>Desconto: <span className="font-medium text-red-600">-{formatCurrency(String(totalDiscount))}</span></span>
            )}
            <span>Total: <span
              className="font-semibold text-gray-900 text-base">{formatCurrency(doc.total)}</span></span>
          </div>
        </div>
      )}

      {/* Pagamentos */}
      {doc.payments !== null && doc.payments !== undefined && doc.payments.length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden">
          <p
            className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">
            Pagamentos
          </p>
          <div className="divide-y divide-gray-100">
            {doc.payments.map((p, i) => (
              <div key={i} className="flex items-center justify-between px-4 py-3 text-sm">
                <span className="text-gray-700">{displayPaymentTypeLabel(p.payment_type)}</span>
                <span className="font-medium text-gray-900">{formatCurrency(p.value)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Informações adicionais */}
      {doc.additional_info && (
        <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Informações adicionais</p>
          <p className="text-sm text-gray-700 whitespace-pre-wrap">{doc.additional_info}</p>
        </div>
      )}

      {/* Eventos */}
      <TableShell
        ariaLabel="Eventos do documento"
        minWidth={120}
        headers={['Evento', 'Seq.', 'Status', 'Data', '']}
      >
        {eventsLoading ? (
          <tr>
            <td colSpan={5} className={TABLE_CELL}>
              <div className="divide-y divide-gray-100">
                {[0, 1].map((i) => (
                  <div key={i} className="flex items-center gap-4">
                    <div className="h-4 w-32 bg-gray-100 rounded animate-pulse"/>
                    <div className="h-4 w-8 bg-gray-100 rounded animate-pulse"/>
                    <div className="h-5 w-20 bg-gray-100 rounded-full animate-pulse"/>
                    <div className="ml-auto h-4 w-20 bg-gray-100 rounded animate-pulse"/>
                  </div>
                ))}
              </div>
            </td>
          </tr>
        ) : !eventsData?.items.length ? (
          <tr>
            <td colSpan={5} className={TABLE_CELL}>Nenhum evento registrado.</td>
          </tr>
        ) : (
          eventsData.items.map((evt) => (
            <tr key={evt.sk} className={`${TABLE_ROW} align-top`}>
              <td data-label="Evento" className={TABLE_CELL}>
                <p className="font-medium text-gray-900">{EVENT_TYPE_LABELS[evt.event_type] ?? evt.event_type}</p>
                {evt.sefaz_motive && (
                  <p className="text-xs text-gray-400 mt-0.5 max-w-65 wrap-break-word">{evt.sefaz_motive}</p>
                )}
              </td>
              <td
                data-label="Seq." className={`${TABLE_CELL} text-gray-500 font-mono text-xs`}>{String(evt.sequence_number).padStart(3, '0')}</td>
              <td data-label="Status" className={TABLE_CELL}>
                <DfeStatusBadge status={evt.status} gender="m"/>
              </td>
              <td
                data-label="Data" className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`}>{formatDatetimeBR(evt.created_at)}</td>
              <td className={`${TABLE_CELL} text-right`}>
                {evt.xml_s3_key && (
                  <Button variant="ghost" size="xs" onClick={() => handleDownloadEventXml(evt)}
                          disabled={eventXmlLoading === evt.sk} className="text-brand-600 hover:text-brand-700">
                    {eventXmlLoading === evt.sk ? 'Baixando…' : 'XML'}
                  </Button>
                )}
              </td>
            </tr>
          ))
        )}
      </TableShell>

      <p className="text-xs text-gray-400">Emissão: {formatDate(doc.year, doc.month, doc.day)}</p>

      {/* Cancel modal (common) */}
      <CancelDfeModal
        isOpen={showCancelModal}
        docLabel={docLabel}
        docNumber={doc.number}
        justification={justification}
        onJustificationChange={setJustification}
        onClose={() => setShowCancelModal(false)}
        onConfirm={() => {
          if (justification.trim().length >= CANCEL_JUSTIFICATION_MIN_LENGTH) cancelMutation.mutate(justification.trim())
        }}
        loading={cancelMutation.isPending}
        error={cancelMutation.error}
      />

      {renderExtra?.(doc)}
    </div>
  )
}
