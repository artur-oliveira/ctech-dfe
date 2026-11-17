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
import dynamic from 'next/dynamic';
import type {PaymentLinkKind} from '@/components/dfe/PaymentLinkModal';
import {PAYMENT_LINK_EVENT_TYPE} from '@/lib/constants/payment-link';
import {NFCOM_EVENT_LABELS} from '@/lib/constants/nfcom';
import {formatCpfCnpj} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR, triggerRemoteDownload} from '@/lib/utils/dfe';

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

function NfcomDetailContent() {
  const params = useSearchParams();
  // A chave vem da URL: só 44 caracteres alfanuméricos (CNPJ alfanumérico) entram nas chamadas da API.
  const rawKey = params.get('key') ?? '';
  const key = /^[0-9A-Z]{44}$/.test(rawKey) ? rawKey : '';
  const qc = useQueryClient();
  const [cancelOpen, setCancelOpen] = useState(false);
  const [justification, setJustification] = useState('');
  const [paymentKind, setPaymentKind] = useState<PaymentLinkKind | null>(null);

  const doc = useQuery({queryKey: queryKeys.nfcoms.detail(key), queryFn: () => apiClient.getNfcom(key), enabled: !!key});
  const events = useQuery({queryKey: queryKeys.nfcoms.events(key), queryFn: () => apiClient.getNfcomEvents(key), enabled: !!key});

  const cancel = useMutation({
    mutationFn: (reason: string) => apiClient.cancelNfcom(key, reason),
    onSuccess: () => {
      setCancelOpen(false);
      setJustification('');
      void qc.invalidateQueries({queryKey: queryKeys.nfcoms.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.nfcoms.events(key)});
    },
  });

  const paymentLink = useMutation({
    mutationFn: ({route, body}: { route: 'payment-link' | 'payment-link-cancel'; body: object }) => apiClient.nfcomPaymentLink(key, route, body),
    onSuccess: () => {
      setPaymentKind(null);
      void qc.invalidateQueries({queryKey: queryKeys.nfcoms.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.nfcoms.events(key)});
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
            <Link href="/nfcom" className="hover:text-brand-600">NFCom</Link>
            <span>/</span>
            <span className="text-gray-600">{nf ? `Nº ${nf.number}` : 'Detalhe'}</span>
          </div>
          <h1 className="text-2xl font-semibold text-gray-900">NFCom {nf ? `nº ${nf.number} / ${nf.serie}` : ''}</h1>
        </div>

        {doc.isLoading || !nf ? <LoadingSkeleton/> : (
          <div className="space-y-6">
            <section className="rounded-xl border border-gray-200 bg-white p-4 md:p-6">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <DfeStatusBadge status={nf.status} gender="f"/>
                <div className="flex flex-wrap items-center gap-2">
                  {nf.status === 'authorized' && (
                    <>
                      <Button variant="outline" size="sm" onClick={() => downloadXml(() => apiClient.downloadNfcomXml(key))}>XML</Button>
                      <DownloadPdfButton fetchPdf={() => apiClient.downloadNfcomDanfecom(key)} label="DANFE-COM" variant="outline"/>
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
                <Field label="Cliente">{nf.customer_name}</Field>
                <Field label="Documento">{nf.customer_doc ? formatCpfCnpj(nf.customer_doc) : '-'}</Field>
                <Field label="Assinante">{nf.subscriber_code}</Field>
                <Field label="Valor total">{formatCurrency(nf.total)}</Field>
                <Field label="Itens">{nf.items}</Field>
                <Field label="Emissão">{formatDatetimeBR(nf.dh_emi ?? nf.created_at)}</Field>
                <Field label="Protocolo">{nf.sefaz_protocol ?? '-'}</Field>
              </dl>
            </section>

            <section>
              <h2 className="mb-2 text-base font-semibold text-gray-900">Eventos</h2>
              {events.isLoading ? <LoadingSkeleton/> : (events.data?.items.length ?? 0) === 0 ? (
                <p className="text-sm text-gray-500">Nenhum evento registrado.</p>
              ) : (
                <TableShell ariaLabel="Eventos da NFCom" headers={['Evento', 'Status', 'Protocolo', 'Registrado em', {label: '', align: 'right'}]}>
                  {events.data!.items.map((ev) => (
                    <tr key={ev.sk} className={TABLE_ROW}>
                      <td className={TABLE_CELL} data-label="Evento">{NFCOM_EVENT_LABELS[ev.event_type] ?? ev.event_type}</td>
                      <td className={TABLE_CELL} data-label="Status"><DfeStatusBadge status={ev.status} gender="m" size="sm"/></td>
                      <td className={`${TABLE_CELL} font-mono text-xs`} data-label="Protocolo">{ev.sefaz_protocol ?? '-'}</td>
                      <td className={`${TABLE_CELL} text-xs text-gray-500`} data-label="Registrado em">{formatDatetimeBR(ev.created_at)}</td>
                      <td className={`${TABLE_CELL} text-right`} data-label="Ações">
                        {ev.xml_s3_key && (
                          <Button variant="ghost" size="sm" className="text-brand-600"
                                  onClick={() => downloadXml(() => apiClient.downloadNfcomEventXml(key, ev.sk))}>XML</Button>
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

      <PaymentLinkModal
        kind={paymentKind}
        docLabel="NFCom"
        docNumber={nf?.number ?? ''}
        links={links}
        loading={paymentLink.isPending}
        error={paymentLink.error}
        onClose={() => { setPaymentKind(null); paymentLink.reset(); }}
        onConfirm={(route, body) => paymentLink.mutate({route, body})}
      />
      <CancelDfeModal
        isOpen={cancelOpen}
        docLabel="NFCom"
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

export default function NfcomDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <NfcomDetailContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
