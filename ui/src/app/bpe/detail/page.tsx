'use client';

import {Suspense, useState} from 'react';
import Link from 'next/link';
import {useSearchParams} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {LoadingSkeleton} from '@/components/ui/loading-skeleton';
import {Button} from '@aoctech/ui';
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge';
import {CopyableCode} from '@/components/ui/copyable-code';
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton';
import {CancelDfeModal, CANCEL_JUSTIFICATION_MIN_LENGTH} from '@/components/dfe/CancelDfeModal';
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell';
import {BPE_EVENT_LABELS, BPE_MODAL_LABELS, BPE_KEY_PATTERN} from '@/lib/constants/bpe';
import dynamic from 'next/dynamic';
import type {PaymentLinkKind} from '@/components/dfe/PaymentLinkModal';
import {PAYMENT_LINK_EVENT_TYPE} from '@/lib/constants/payment-link';
import {BpeEventModal, type BpeEventKind, type BpeEventValues} from '@/components/bpe/BpeEventModal';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR, triggerRemoteDownload} from '@/lib/utils/dfe';

const SEND: Record<BpeEventKind, (key: string, v: BpeEventValues) => Promise<unknown>> = {
  'no-show': (key, v) => apiClient.bpeNoShow(key, v.justification.trim()),
  'seat-change': (key, v) => apiClient.bpeSeatChange(key, v.seat),
  'excess-baggage': (key, v) => apiClient.bpeExcessBaggage(key, v.quantity.trim(), v.value),
  'supply-not-performed': (key, v) => apiClient.bpeSupplyNotPerformed(key, v.ibs, v.cbs),
};

const VARIANT_LABEL: Record<string, string> = {normal: 'Comum', tm: 'TM', ta: 'TA'};

// O modal puxa o seletor de pessoas e os campos de pagamento: só carrega quando o usuário abre a vinculação.
const PaymentLinkModal = dynamic(() => import('@/components/dfe/PaymentLinkModal').then((m) => m.PaymentLinkModal), {ssr: false});

function Field({label, children}: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs font-medium text-gray-500">{label}</dt>
      <dd className="mt-0.5 text-sm text-gray-900 break-words">{children}</dd>
    </div>
  );
}

function BpeDetailContent() {
  const params = useSearchParams();
  // A chave vem da URL: só 44 caracteres alfanuméricos (CNPJ alfanumérico) entram nas chamadas da API.
  const rawKey = params.get('key') ?? '';
  const key = BPE_KEY_PATTERN.test(rawKey) ? rawKey : '';
  const qc = useQueryClient();
  const [cancelOpen, setCancelOpen] = useState(false);
  const [justification, setJustification] = useState('');
  const [eventKind, setEventKind] = useState<BpeEventKind | null>(null);
  const [paymentKind, setPaymentKind] = useState<PaymentLinkKind | null>(null);

  const doc = useQuery({queryKey: queryKeys.bpes.detail(key), queryFn: () => apiClient.getBpe(key), enabled: !!key});
  const events = useQuery({queryKey: queryKeys.bpes.events(key), queryFn: () => apiClient.getBpeEvents(key), enabled: !!key});

  const cancel = useMutation({
    mutationFn: (reason: string) => apiClient.cancelBpe(key, reason),
    onSuccess: () => {
      setCancelOpen(false);
      setJustification('');
      void qc.invalidateQueries({queryKey: queryKeys.bpes.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.bpes.events(key)});
    },
  });

  const registerEvent = useMutation({
    mutationFn: ({kind, v}: { kind: BpeEventKind; v: BpeEventValues }) => SEND[kind](key, v),
    onSuccess: () => {
      setEventKind(null);
      void qc.invalidateQueries({queryKey: queryKeys.bpes.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.bpes.events(key)});
    },
  });

  const paymentLink = useMutation({
    mutationFn: ({route, body}: { route: 'payment-link' | 'payment-link-cancel'; body: object }) => apiClient.bpePaymentLink(key, route, body),
    onSuccess: () => {
      setPaymentKind(null);
      void qc.invalidateQueries({queryKey: queryKeys.bpes.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.bpes.events(key)});
    },
  });
  const links = (events.data?.items ?? []).filter((e) => e.event_type === PAYMENT_LINK_EVENT_TYPE && e.status === 'success');

  const downloadXml = async (download: () => Promise<{ url: string }>) => {
    try {
      triggerRemoteDownload((await download()).url);
    } catch {
      toast.error('Erro ao baixar o XML.');
    }
  };

  const nf = doc.data;
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="mb-6">
          <div className="flex items-center gap-2 text-sm text-gray-400 mb-3">
            <Link href="/bpe" className="hover:text-brand-600">BP-e</Link>
            <span>/</span>
            <span className="text-gray-600">{nf ? `Nº ${nf.number}` : 'Detalhe'}</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">BP-e {nf ? `nº ${nf.number} / ${nf.serie}` : ''}</h1>
        </div>

        {doc.isLoading || !nf ? <LoadingSkeleton/> : (
          <div className="space-y-6">
            <section className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <DfeStatusBadge status={nf.status} gender="m"/>
                <div className="flex flex-wrap items-center gap-2">
                  {nf.status === 'authorized' && (
                    <>
                      <Button variant="outline" size="sm" onClick={() => downloadXml(() => apiClient.downloadBpeXml(key))}>XML</Button>
                      {nf.variant !== 'tm' && <DownloadPdfButton fetchPdf={() => apiClient.downloadBpeDabpe(key)} label="DABPE" variant="outline"/>}
                      {nf.variant !== 'tm' && <DownloadPdfButton fetchPdf={() => apiClient.downloadBpeDabpe(key, 'reduzido')} label="DABPE reduzido" variant="outline"/>}
                      {nf.variant === 'normal' && (
                        <>
                          <Button variant="outline" size="sm" onClick={() => setEventKind('no-show')}>Não embarque</Button>
                          <Button variant="outline" size="sm" onClick={() => setEventKind('seat-change')}>Trocar poltrona</Button>
                          <Button variant="outline" size="sm" onClick={() => setEventKind('excess-baggage')}>Excesso de bagagem</Button>
                        </>
                      )}
                      {nf.variant === 'ta' && (
                        <>
                          <Button variant="outline" size="sm" onClick={() => setEventKind('no-show')}>Não embarque</Button>
                          <Button variant="outline" size="sm" onClick={() => setEventKind('supply-not-performed')}>Fornecimento não realizado</Button>
                        </>
                      )}
                      <Button variant="outline" size="sm" onClick={() => setPaymentKind('link')}>Vincular pagamento</Button>
                      {links.length > 0 && <Button variant="outline" size="sm" onClick={() => setPaymentKind('cancel')}>Cancelar vinculação</Button>}
                      <Button variant="outline" size="sm" className="text-danger" onClick={() => setCancelOpen(true)}>Cancelar</Button>
                    </>
                  )}
                </div>
              </div>
              {nf.sefaz_motive && <p className="mt-3 text-sm text-gray-600">{nf.sefaz_status ? `${nf.sefaz_status} - ` : ''}{nf.sefaz_motive}</p>}
              <CopyableCode value={nf.sk}/>
              <dl className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <Field label="Tipo">{VARIANT_LABEL[nf.variant] ?? nf.variant}</Field>
                <Field label="Modal">{BPE_MODAL_LABELS[nf.modal] ?? nf.modal}</Field>
                {nf.variant === 'tm' ? (
                  <>
                    <Field label="Competência">{nf.competence ?? '-'}</Field>
                    <Field label="Passageiros">{nf.passengers ?? '-'}</Field>
                    <Field label="Grupos">{nf.groups ?? '-'}</Field>
                    <Field label="Itens">{nf.items ?? '-'}</Field>
                  </>
                ) : (
                  <>
                    <Field label="Passageiro">{nf.passenger_name || 'Não identificado'}</Field>
                    <Field label="Origem">{nf.from_name ?? '-'}</Field>
                    <Field label="Destino">{nf.to_name ?? '-'}</Field>
                    <Field label="Embarque">{nf.boarding ? formatDatetimeBR(nf.boarding) : '-'}</Field>
                    <Field label="Trechos">{nf.trips ?? '-'}</Field>
                  </>
                )}
                <Field label="Valor total">{formatCurrency(nf.total)}</Field>
                <Field label="Emissão">{formatDatetimeBR(nf.dh_emi ?? nf.created_at)}</Field>
                <Field label="Protocolo">{nf.sefaz_protocol ?? '-'}</Field>
              </dl>
            </section>

            <section>
              <h2 className="mb-2 text-base font-semibold text-gray-900">Eventos</h2>
              {events.isLoading ? <LoadingSkeleton/> : (events.data?.items.length ?? 0) === 0 ? (
                <p className="text-sm text-gray-500">Nenhum evento registrado.</p>
              ) : (
                <TableShell ariaLabel="Eventos da BP-e" headers={['Evento', 'Status', 'Protocolo', 'Registrado em', {label: '', align: 'right'}]}>
                  {events.data!.items.map((ev) => (
                    <tr key={ev.sk} className={TABLE_ROW}>
                      <td className={TABLE_CELL} data-label="Evento">{BPE_EVENT_LABELS[ev.event_type] ?? ev.event_type}</td>
                      <td className={TABLE_CELL} data-label="Status"><DfeStatusBadge status={ev.status} gender="m" size="sm"/></td>
                      <td className={`${TABLE_CELL} font-mono text-xs`} data-label="Protocolo">{ev.sefaz_protocol ?? '-'}</td>
                      <td className={`${TABLE_CELL} text-xs text-gray-500`} data-label="Registrado em">{formatDatetimeBR(ev.created_at)}</td>
                      <td className={`${TABLE_CELL} text-right`} data-label="Ações">
                        {ev.xml_s3_key && (
                          <Button variant="ghost" size="sm" className="text-brand-600"
                                  onClick={() => downloadXml(() => apiClient.downloadBpeEventXml(key, ev.sk))}>XML</Button>
                        )}
                      </td>
                    </tr>
                  ))}
                </TableShell>
              )}
            </section>
          </div>
        )}
      </div>

      <BpeEventModal
        kind={eventKind}
        docNumber={nf?.number ?? ''}
        loading={registerEvent.isPending}
        error={registerEvent.error}
        onClose={() => { setEventKind(null); registerEvent.reset(); }}
        onConfirm={(kind, v) => registerEvent.mutate({kind, v})}
      />
      <PaymentLinkModal
        kind={paymentKind}
        docLabel="BP-e"
        docNumber={nf?.number ?? ''}
        links={links}
        loading={paymentLink.isPending}
        error={paymentLink.error}
        onClose={() => { setPaymentKind(null); paymentLink.reset(); }}
        onConfirm={(route, body) => paymentLink.mutate({route, body})}
      />
      <CancelDfeModal
        isOpen={cancelOpen}
        docLabel="BP-e"
        docNumber={nf?.number ?? ''}
        justification={justification}
        onJustificationChange={setJustification}
        loading={cancel.isPending}
        error={cancel.error}
        onClose={() => { setCancelOpen(false); setJustification(''); }}
        onConfirm={() => {
          if (justification.trim().length >= CANCEL_JUSTIFICATION_MIN_LENGTH) cancel.mutate(justification.trim());
        }}
      />
    </RootLayout>
  );
}

export default function BpeDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <BpeDetailContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
