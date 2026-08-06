'use client'

import {Suspense, useState} from 'react'
import Link from 'next/link'
import {useSearchParams} from 'next/navigation'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {apiClient, ApiError} from '@/lib/api/client'
import {useAuth} from '@/lib/hooks/useAuth'
import {queryKeys} from '@/lib/api/query-keys'
import {ProtectedRoute} from '@/components/ProtectedRoute'
import {RootLayout} from '@/components/layout/RootLayout'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {OptionsSelect} from '@/components/ui/options-select'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {StatusBadge} from '@/components/ui/status-badge'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'
import {CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal'
import {NfseCancelModal} from '@/components/nfse/NfseCancelModal'
import {NFSE_STATUS_CLASSES, isTransitionalNfseStatus, nfseStatusLabel} from '@/components/nfse/NfseStatusBadge'
import {CONTRIBUINTE_EVENTS, EVENT_LABELS, nfseEventSchema, type NfseEventFormData} from '@/lib/schemas/nfse'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {formatCpfCnpj} from '@/lib/utils/document'
import {formatCurrency} from '@/lib/utils/helpers'
import {formatDatetimeBR, triggerDownload} from '@/lib/utils/dfe'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {toast} from 'sonner'

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

const TP_EMIT_LABELS: Record<number, string> = {1: 'Prestador', 2: 'Tomador', 3: 'Intermediário'}

const EVENT_TYPE_OPTIONS = CONTRIBUINTE_EVENTS.map((code) => ({value: code, label: `${code} — ${EVENT_LABELS[code]}`}))

function NfseEventModal({idDps, isOpen, onClose}: { idDps: string; isOpen: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const form = useForm<NfseEventFormData>({
    resolver: zodResolver(nfseEventSchema),
    defaultValues: {event_type: CONTRIBUINTE_EVENTS[0]},
  })

  const mutation = useMutation({
    mutationFn: (data: NfseEventFormData) => apiClient.sendNfseEvent(idDps, {
      event_type: data.event_type,
      sequence_number: data.sequence_number,
      reason_code: data.reason_code || undefined,
      reason_description: data.reason_description || undefined,
    }),
    onSuccess: () => {
      form.reset({event_type: CONTRIBUINTE_EVENTS[0]})
      onClose()
      void qc.invalidateQueries({queryKey: queryKeys.nfses.events(idDps)})
      void qc.invalidateQueries({queryKey: queryKeys.nfses.detail(idDps)})
      toast.success('Evento enviado.')
    },
  })

  return (
    <Modal
      isOpen={isOpen}
      title="Registrar evento"
      onClose={onClose}
      onSubmit={form.handleSubmit((data) => mutation.mutate(data))}
      submitLabel="Enviar evento"
      cancelLabel="Voltar"
      loading={mutation.isPending}
    >
      <Form {...form}>
        <div className="space-y-4">
          <FormField control={form.control} name="event_type" render={({field}) => (
            <FormItem>
              <FormLabel>Evento</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={EVENT_TYPE_OPTIONS}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="reason_code" render={({field}) => (
            <FormItem>
              <FormLabel>Código do motivo</FormLabel>
              <Input {...field} id={field.name} maxLength={2} className="w-20"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="reason_description" render={({field}) => (
            <FormItem>
              <FormLabel>Descrição do motivo</FormLabel>
              <Input {...field} id={field.name} maxLength={255}/>
              <FormMessage/>
            </FormItem>
          )}/>
          {mutation.isError && (
            <p className="text-xs text-red-600">
              {mutation.error instanceof ApiError ? mutation.error.detail : 'Erro ao enviar evento.'}
            </p>
          )}
        </div>
      </Form>
    </Modal>
  )
}

function NfseDetail({idDps}: { idDps: string }) {
  const {selectedOrg} = useAuth()
  const qc = useQueryClient()
  const [showCancelModal, setShowCancelModal] = useState(false)
  const [showEventModal, setShowEventModal] = useState(false)
  const [xmlLoading, setXmlLoading] = useState<'xml' | 'dps' | 'danfse' | null>(null)
  const [eventXmlLoading, setEventXmlLoading] = useState<string | null>(null)

  const {data: doc, isLoading, error} = useQuery({
    queryKey: queryKeys.nfses.detail(idDps),
    queryFn: () => apiClient.getNfse(idDps),
    enabled: !!idDps && !!selectedOrg,
  })

  const {data: eventsData, isLoading: eventsLoading} = useQuery({
    queryKey: queryKeys.nfses.events(idDps),
    queryFn: () => apiClient.getNfseEvents(idDps),
    enabled: !!idDps && !!selectedOrg,
  })

  const cancelMutation = useMutation({
    mutationFn: ({reasonCode, reasonDescription}: { reasonCode: string; reasonDescription: string }) =>
      apiClient.cancelNfse(idDps, reasonCode, reasonDescription),
    onSuccess: () => {
      setShowCancelModal(false)
      void qc.invalidateQueries({queryKey: queryKeys.nfses.detail(idDps)})
      void qc.invalidateQueries({queryKey: queryKeys.nfses.lists(selectedOrg?.pk)})
    },
  })

  const handleDownload = async (kind: 'xml' | 'dps' | 'danfse') => {
    setXmlLoading(kind)
    try {
      if (kind === 'xml') triggerDownload(await apiClient.downloadNfseXml(idDps), `${idDps}.xml`)
      if (kind === 'dps') triggerDownload(await apiClient.downloadNfseDpsXml(idDps), `${idDps}_dps.xml`)
      if (kind === 'danfse') triggerDownload(await apiClient.downloadDanfse(idDps), `${idDps}.pdf`)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao baixar arquivo.')
    } finally {
      setXmlLoading(null)
    }
  }

  const handleDownloadEventXml = async (eventSk: string, eventType: string) => {
    setEventXmlLoading(eventSk)
    try {
      triggerDownload(await apiClient.downloadNfseEventXml(idDps, eventSk), `${eventType}-${idDps}.xml`)
    } finally {
      setEventXmlLoading(null)
    }
  }

  if (isLoading) {
    return <LoadingSkeleton count={3} height="h-24" rounded="rounded-xl"/>
  }

  if (error || !doc) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
        NFS-e não encontrada.
      </div>
    )
  }

  const canCancel = doc.status === 'authorized'
  const cityLabel = CITY_OPTIONS.find((c) => c.value === doc.c_loc_emi)?.label ?? doc.c_loc_emi

  return (
    <div className="space-y-6 max-w-3xl">
      {/* Cabeçalho */}
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <p className="text-2xl font-semibold text-gray-900">
            NFS-e {doc.number}
            <span className="ml-2 text-base font-normal text-gray-400">série {doc.serie}</span>
          </p>
          {doc.access_key && <p className="text-xs text-gray-400 font-mono mt-1 break-all">Chave: {doc.access_key}</p>}
          <p className="text-xs text-gray-400 font-mono mt-0.5 break-all">id_dps: {doc.sk}</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <StatusBadge
            size="md"
            label={nfseStatusLabel(doc.status)}
            className={NFSE_STATUS_CLASSES[doc.status] ?? 'bg-gray-100 text-gray-600'}
            isTransitional={isTransitionalNfseStatus(doc.status)}
          />

          {doc.xml_s3_key && (
            <Button variant="outline" size="sm" onClick={() => handleDownload('xml')} disabled={xmlLoading === 'xml'}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              {xmlLoading === 'xml' ? 'Baixando…' : 'XML'}
            </Button>
          )}
          {doc.dps_xml_s3_key && (
            <Button variant="outline" size="sm" onClick={() => handleDownload('dps')} disabled={xmlLoading === 'dps'}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              {xmlLoading === 'dps' ? 'Baixando…' : 'XML da DPS'}
            </Button>
          )}
          {doc.status === 'authorized' && doc.provider === 'nacional' && (
            <DownloadPdfButton fetchPdf={() => apiClient.downloadDanfse(idDps)} filename={idDps} label="DANFSE"
                                variant="outline" size="sm"/>
          )}

          <Button variant="outline" size="sm" render={<Link href={`/nfse/emit?substitute=${encodeURIComponent(idDps)}`}/>}
                  className="text-amber-600 border-amber-200 hover:bg-amber-50">
            Substituir
          </Button>

          <Button variant="outline" size="sm" onClick={() => setShowEventModal(true)}>
            Registrar evento
          </Button>

          {canCancel && (
            <Button variant="outline" size="sm" onClick={() => setShowCancelModal(true)}
                    className="text-red-600 border-red-200 hover:bg-red-50">
              Cancelar
            </Button>
          )}
        </div>
      </div>

      {/* Motivo da rejeição — em destaque, não escondido */}
      {doc.status === 'rejected' && doc.sefaz_motive && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm">
          <p className="text-xs font-semibold uppercase tracking-wider text-red-400 mb-1.5">Motivo da rejeição</p>
          <p className="text-red-700 whitespace-pre-wrap wrap-break-word">{doc.sefaz_motive}</p>
        </div>
      )}

      {/* Prestador / Tomador / Intermediário */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
            Organização emitente <span className="font-normal">— {TP_EMIT_LABELS[doc.tp_emit] ?? doc.tp_emit} emite</span>
          </p>
          <p className="font-medium text-gray-900">{doc.emit_name}</p>
          <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.emit_cpf_cnpj)}</p>
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Tomador</p>
          <p className="font-medium text-gray-900">{doc.dest_name || <span className="text-gray-300">—</span>}</p>
          {doc.dest_cpf_cnpj && <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.dest_cpf_cnpj)}</p>}
        </div>
      </div>

      {/* Serviço e valores */}
      <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Serviço e valores</p>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 text-sm">
          <div>
            <p className="text-gray-400 text-xs">Competência</p>
            <p className="text-gray-900">{doc.competence}</p>
          </div>
          <div>
            <p className="text-gray-400 text-xs">Município de emissão</p>
            <p className="text-gray-900">{cityLabel}</p>
          </div>
          <div>
            <p className="text-gray-400 text-xs">Valor do serviço</p>
            <p className="text-gray-900 font-medium">{formatCurrency(doc.total)}</p>
          </div>
        </div>
      </div>

      {/* Timeline de eventos */}
      <TableShell ariaLabel="Eventos da NFS-e" minWidth={120} headers={['Evento', 'Status', 'Data', '']}>
        {eventsLoading ? (
          <tr>
            <td colSpan={4} className={TABLE_CELL}>
              <div className="h-4 w-32 bg-gray-100 rounded animate-pulse"/>
            </td>
          </tr>
        ) : !eventsData?.items.length ? (
          <tr>
            <td colSpan={4} className={TABLE_CELL}>Nenhum evento registrado.</td>
          </tr>
        ) : (
          eventsData.items.map((evt) => (
            <tr key={evt.sk} className={`${TABLE_ROW} align-top`}>
              <td data-label="Evento" className={TABLE_CELL}>
                <p className="font-medium text-gray-900">{EVENT_LABELS[evt.event_type] ?? evt.event_type}</p>
                {evt.sefaz_motive && <p className="text-xs text-gray-400 mt-0.5 max-w-65 wrap-break-word">{evt.sefaz_motive}</p>}
              </td>
              <td data-label="Status" className={TABLE_CELL}>
                <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${EVENT_STATUS_CLASSES[evt.status] ?? 'bg-gray-100 text-gray-600'}`}>
                  {EVENT_STATUS_LABELS[evt.status] ?? evt.status}
                </span>
              </td>
              <td data-label="Data" className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`}>{formatDatetimeBR(evt.created_at)}</td>
              <td className={`${TABLE_CELL} text-right`}>
                {evt.xml_s3_key && (
                  <Button variant="ghost" size="xs" onClick={() => handleDownloadEventXml(evt.sk, evt.event_type)}
                          disabled={eventXmlLoading === evt.sk} className="text-brand-600 hover:text-brand-700">
                    {eventXmlLoading === evt.sk ? 'Baixando…' : 'XML'}
                  </Button>
                )}
              </td>
            </tr>
          ))
        )}
      </TableShell>

      <NfseCancelModal
        isOpen={showCancelModal}
        docNumber={doc.number}
        loading={cancelMutation.isPending}
        error={cancelMutation.error}
        onClose={() => setShowCancelModal(false)}
        onConfirm={({reasonCode, reasonDescription}) => {
          if (reasonDescription.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return
          cancelMutation.mutate({reasonCode, reasonDescription})
        }}
      />

      <NfseEventModal idDps={idDps} isOpen={showEventModal} onClose={() => setShowEventModal(false)}/>
    </div>
  )
}

function NfseDetailContent() {
  const params = useSearchParams()
  const idDps = params.get('id') ?? ''

  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/nfse" className="hover:text-brand-600">NFS-e</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate min-w-0 max-w-[200px]">{idDps || 'Detalhe'}</span>
        </div>
        {idDps ? (
          <NfseDetail idDps={idDps}/>
        ) : (
          <p className="text-sm text-gray-500">id_dps não informado.</p>
        )}
      </div>
    </RootLayout>
  )
}

export default function NfseDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfseDetailContent/>
      </Suspense>
    </ProtectedRoute>
  )
}
