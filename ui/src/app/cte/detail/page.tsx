'use client';

import {Suspense, useState} from 'react';
import Link from 'next/link';
import {useSearchParams} from 'next/navigation';
import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {usePagination} from '@/lib/hooks/usePagination';
import {queryKeys} from '@/lib/api/query-keys';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {Button, Skeleton} from '@aoctech/ui';
import {DfeStatusBadge} from '@/components/dfe/DfeStatusBadge';
import {ContingencyBadge, ContingencyNotice} from '@/components/dfe/ContingencyNotice';
import {CopyableCode} from '@/components/ui/copyable-code';
import {Pagination} from '@/components/ui/pagination';
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell';
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton';
import {CONTINGENCY_PENDING} from '@/lib/contingency';
import {MdfeSefazStatus} from '@/components/mdfe/MdfeSefazStatus';
import {useCteActions} from '@/components/cte/CteActions';
import {CTE_EVENT, CTE_OS_SERVICES, CTE_STATUS_AUTHORIZED, CTE_TAKER_LABELS, PERMISSION_CTE_CREATE} from '@/lib/constants/cte';
import {CteDocKindBadge} from '@/components/cte/CteKindBadge';
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event';
import {formatCpfCnpj, orgTaxId} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR, triggerRemoteDownload} from '@/lib/utils/dfe';
import {cteChangeBlocker, cteKind, sameDoc} from '@/lib/utils/cte';
import {ACCESS_KEY_MOD_CTE, ACCESS_KEY_MOD_CTE_OS, validateAccessKey} from '@/lib/utils/access-key';
import {hasPermission} from '@/lib/utils/delivery-events';
import type {NfeEventOut} from '@/lib/types/api';

const PERMISSION_CTE_EVENTS = 'create.cte_events';

const ACTION_BTN = 'min-h-11 sm:min-h-0';

/** CT-e relacionados (complemento ou substituto): a api só devolve a referência do documento para o original. */
function RelatedDocs({label, keys}: { label: string; keys: string[] }) {
  return (
    <section aria-label={label} className="rounded-xl border border-gray-200 bg-white overflow-hidden">
      <p className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">{label}</p>
      <ul className="divide-y divide-gray-100">
        {keys.map((k) => (
          <li key={k}>
            <Link href={`/cte/detail?key=${k}`}
                  className="flex min-h-11 items-center px-4 py-2.5 font-mono text-xs text-brand-600 hover:text-brand-700 truncate sm:min-h-0">
              {k}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

function InfoCard({label, children}: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">{label}</p>
      {children}
    </div>
  );
}

const Party = ({label, p}: { label: string; p?: { doc: string; name: string } }) => (
  <InfoCard label={label}>
    <p className="font-medium text-gray-900">{p?.name || '-'}</p>
    {p?.doc && <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(p.doc)}</p>}
  </InfoCard>
);

function CteDetail({accessKey}: { accessKey: string }) {
  const {selectedOrg} = useAuth();
  const [eventXmlLoading, setEventXmlLoading] = useState<string | null>(null);
  const {openCancel, openCorrection, openDisagree, openDisagreeCancel, openReceipt, openFailure, openDeliveryCancel, modals} = useCteActions(selectedOrg?.pk);

  const {data: doc, isLoading, error} = useQuery({
    queryKey: queryKeys.ctes.detail(accessKey),
    queryFn: () => apiClient.getCte(accessKey),
    enabled: !!accessKey && !!selectedOrg,
  });

  const events = usePagination<NfeEventOut>({
    queryKey: queryKeys.ctes.events(accessKey),
    queryFn: (cursor) => apiClient.getCteEvents(accessKey, cursor),
    enabled: !!accessKey && !!selectedOrg,
  });

  const handleDownloadEventXml = async (event: NfeEventOut) => {
    setEventXmlLoading(event.sk);
    try {
      triggerRemoteDownload((await apiClient.downloadCteEventXml(accessKey, event.sk)).url);
    } finally {
      setEventXmlLoading(null);
    }
  };

  if (isLoading) {
    return <div className="space-y-4">{[...Array(3)].map((_, i) => <Skeleton key={i} className="h-24 rounded-xl"/>)}</div>;
  }
  if (error || !doc) {
    return <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">CT-e não encontrado.</div>;
  }

  const orgDoc = selectedOrg ? orgTaxId(selectedOrg) : '';
  const authorized = doc.status === CTE_STATUS_AUTHORIZED;
  // Sem o emitente na resposta a api decide (422 not_cte_issuer); com ele, o botão some para quem não emitiu.
  const isIssuer = !doc.emit_cpf_cnpj || sameDoc(doc.emit_cpf_cnpj, orgDoc);
  const isTaker = sameDoc(doc.taker?.doc, orgDoc);
  const kind = cteKind(doc);
  const canChange = hasPermission(selectedOrg?.permissions, PERMISSION_CTE_CREATE) && !cteChangeBlocker(doc, isIssuer);
  const target = {
    sk: accessKey, number: doc.number,
    disagreements: events.items.filter((e) => e.event_type === CTE_EVENT.DISAGREEMENT && e.status === 'success'),
    deliveries: events.items.filter((e) => (e.event_type === CTE_EVENT.DELIVERY_RECEIPT || e.event_type === CTE_EVENT.DELIVERY_FAILURE) && e.status === 'success'),
    nfeKeys: doc.nfe_keys ?? [],
  };
  const canEvents = hasPermission(selectedOrg?.permissions, PERMISSION_CTE_EVENTS);
  const place = (p: { city: string; uf: string } | null) => (p ? `${p.city} / ${p.uf}` : '-');

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div className="min-w-0">
          <p className="text-2xl font-semibold text-gray-900">
            CT-e {doc.number}
            <span className="ml-2 text-base font-normal text-gray-400">série {doc.serie}</span>
          </p>
          <CopyableCode value={accessKey}/>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <CteDocKindBadge doc={doc} size="md"/>
          <DfeStatusBadge status={doc.status} gender="m" size="md"/>
          <ContingencyBadge state={doc.contingency_state} male size="md"/>
          {kind !== 'os' && (authorized || doc.contingency_state === CONTINGENCY_PENDING) && (
            <DownloadPdfButton fetchPdf={() => apiClient.downloadCteDacte(accessKey)} label="Baixar DACTE" variant="outline"
                               className={`${ACTION_BTN} text-brand-600 border-brand-200 hover:bg-brand-50`}/>
          )}
        </div>
      </div>

      <ContingencyNotice doc={doc} male/>

      {authorized && (
        <section aria-label="Ações do CT-e" className="flex flex-wrap gap-2">
          {canChange && (
            <>
              <Button variant="outline" render={<Link href={`/cte/complement?key=${accessKey}`}/>} className={`${ACTION_BTN} text-brand-600 border-brand-200 hover:bg-brand-50`}>
                Emitir complemento
              </Button>
              <Button variant="outline" render={<Link href={`/cte/substitute?key=${accessKey}`}/>} className={`${ACTION_BTN} text-brand-600 border-brand-200 hover:bg-brand-50`}>
                Emitir substituto
              </Button>
            </>
          )}
          {isIssuer && (
            <>
              <Button variant="outline" onClick={() => openCorrection(target)} className={`${ACTION_BTN} text-brand-600 border-brand-200 hover:bg-brand-50`}>
                Carta de correção
              </Button>
              <Button variant="outline" onClick={() => openCancel(target)} className={`${ACTION_BTN} text-red-600 border-red-200 hover:bg-red-50`}>
                Cancelar
              </Button>
              {canEvents && (
                <>
                  <Button variant="outline" onClick={() => openReceipt(target)} className={ACTION_BTN}>
                    Comprovante de entrega
                  </Button>
                  <Button variant="outline" onClick={() => openFailure(target)} className={ACTION_BTN}>
                    Insucesso de entrega
                  </Button>
                  {target.deliveries.length > 0 && (
                    <Button variant="outline" onClick={() => openDeliveryCancel(target)} className={ACTION_BTN}>
                      Cancelar entrega
                    </Button>
                  )}
                </>
              )}
            </>
          )}
          {isTaker ? (
            <>
              <Button variant="outline" onClick={() => openDisagree(target)} className={ACTION_BTN}>
                Prestação em desacordo
              </Button>
              {target.disagreements.length > 0 && (
                <Button variant="outline" onClick={() => openDisagreeCancel(target)} className={ACTION_BTN}>
                  Cancelar desacordo
                </Button>
              )}
            </>
          ) : (
            <p className="flex min-h-11 items-center text-sm text-gray-500 sm:min-h-0">
              Prestação em desacordo: <b className="ml-1 font-medium text-gray-700">apenas o tomador</b>
            </p>
          )}
        </section>
      )}

      {(doc.sefaz_status || doc.sefaz_motive) && (
        <div className={`rounded-lg border px-4 py-3 text-sm ${
          authorized ? 'border-green-200 bg-green-50'
            : doc.status === 'rejected' || doc.status === 'failed' ? 'border-red-200 bg-red-50'
              : 'border-gray-200 bg-gray-50'}`}>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-1.5">
            {doc.status === 'rejected' || doc.status === 'failed' ? 'Rejeição' : 'Autorização'}
          </p>
          {doc.sefaz_status && <p className="font-mono text-xs text-gray-500">Código: {doc.sefaz_status}</p>}
          {doc.sefaz_motive && <p className="text-gray-700 whitespace-pre-wrap wrap-break-word">{doc.sefaz_motive}</p>}
        </div>
      )}

      <MdfeSefazStatus accessKey={accessKey} localStatus={doc.status} docType="cte"/>

      {doc.complemented_keys && doc.complemented_keys.length > 0 && (
        <RelatedDocs label="Complementa" keys={doc.complemented_keys}/>
      )}
      {doc.substituted_key && <RelatedDocs label="Substitui" keys={[doc.substituted_key]}/>}

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <InfoCard label="Frete">
          <p className="font-medium text-gray-900">{formatCurrency(doc.freight_total)}</p>
          <p className="text-xs text-gray-500">CFOP {doc.cfop} · ICMS CST {doc.icms_cst}</p>
        </InfoCard>
        {doc.os_service && (
          <InfoCard label="Serviço">
            <p className="font-medium text-gray-900">{CTE_OS_SERVICES.find((x) => x.value === doc.os_service)?.label ?? doc.os_service}</p>
            <p className="text-xs text-gray-500">CT-e OS, modelo 67</p>
          </InfoCard>
        )}
        <InfoCard label="Trajeto">
          <p className="font-medium text-gray-900">{place(doc.origin)} <span className="text-gray-400 mx-1">→</span> {place(doc.destination)}</p>
          {doc.cargo && (
            <p className="text-xs text-gray-500">
              {doc.cargo.predominant_product} · {formatCurrency(doc.cargo.total_value)}
              {doc.cargo.total_weight_kg && <> · {doc.cargo.total_weight_kg} kg</>}
            </p>
          )}
        </InfoCard>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        {doc.sender && <Party label="Remetente" p={doc.sender}/>}
        {doc.recipient && <Party label="Destinatário" p={doc.recipient}/>}
        {doc.taker && (
          <InfoCard label={`Tomador (${CTE_TAKER_LABELS[doc.taker.role] ?? doc.taker.role})`}>
            <p className="font-medium text-gray-900">{doc.taker.name || '-'}</p>
            {doc.taker.doc && <p className="text-xs text-gray-500 font-mono">{formatCpfCnpj(doc.taker.doc)}</p>}
          </InfoCard>
        )}
      </div>
      {kind !== 'normal' && !doc.sender && kind !== 'os' && (
        <p className="text-sm text-gray-500">Remetente e destinatário são os do CT-e original; este documento não os repete aqui.</p>
      )}

      {doc.nfe_keys && doc.nfe_keys.length > 0 && (
        <div className="rounded-xl border border-gray-200 bg-white overflow-hidden">
          <p className="px-4 py-3 text-xs font-semibold uppercase tracking-wider text-gray-400 border-b border-gray-100">
            Notas ({doc.nfe_keys.length})
          </p>
          <div className="divide-y divide-gray-100">
            {doc.nfe_keys.map((k) => (
              <p key={k} className="px-4 py-2.5 font-mono text-xs text-gray-500 truncate">{k}</p>
            ))}
          </div>
        </div>
      )}

      <TableShell ariaLabel="Eventos do CT-e" minWidth={120} headers={['Evento', 'Seq.', 'Status', 'Data', '']}
                  dimmed={events.isFetching && !events.isLoading}>
        {events.isLoading ? (
          <tr><td colSpan={5} className={TABLE_CELL}><Skeleton className="h-4 w-40 rounded"/></td></tr>
        ) : events.items.length === 0 ? (
          <tr><td colSpan={5} className={TABLE_CELL}>Nenhum evento registrado.</td></tr>
        ) : (
          events.items.map((evt) => (
            <tr key={evt.sk} className={`${TABLE_ROW} align-top`}>
              <td data-label="Evento" className={TABLE_CELL}>
                <p className="font-medium text-gray-900">{EVENT_TYPE_LABELS[evt.event_type] ?? evt.event_type}</p>
                {evt.sefaz_motive && <p className="text-xs text-gray-400 mt-0.5 max-w-65 wrap-break-word">{evt.sefaz_motive}</p>}
              </td>
              <td data-label="Seq." className={`${TABLE_CELL} text-gray-500 font-mono text-xs`}>{String(evt.sequence_number).padStart(3, '0')}</td>
              <td data-label="Status" className={TABLE_CELL}><DfeStatusBadge status={evt.status} gender="m"/></td>
              <td data-label="Data" className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`}>{formatDatetimeBR(evt.created_at)}</td>
              <td className={`${TABLE_CELL} text-right`}>
                {evt.xml_s3_key && (
                  <Button variant="ghost" size="sm" onClick={() => handleDownloadEventXml(evt)}
                          disabled={eventXmlLoading === evt.sk} aria-label={`Baixar XML do evento ${evt.sequence_number}`}
                          className="min-h-11 text-brand-600 hover:text-brand-700 sm:min-h-0">
                    {eventXmlLoading === evt.sk ? 'Baixando…' : 'XML'}
                  </Button>
                )}
              </td>
            </tr>
          ))
        )}
      </TableShell>
      {(events.hasNext || events.hasPrevious) && (
        <Pagination hasNext={events.hasNext} hasPrevious={events.hasPrevious} onNext={events.goNext}
                    onPrevious={events.goPrevious} isLoading={events.isFetching}/>
      )}

      {modals}
    </div>
  );
}

function CteDetailContent() {
  const accessKey = useSearchParams().get('key') ?? '';
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/cte" className="hover:text-brand-600">CT-e</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate max-w-[200px]">{accessKey || 'Detalhe'}</span>
        </div>
        {!accessKey ? (
          <p className="text-sm text-gray-500">Chave de acesso não informada.</p>
        ) : !validateAccessKey(accessKey, [ACCESS_KEY_MOD_CTE, ACCESS_KEY_MOD_CTE_OS]).valid ? (
          <p role="alert" className="text-sm text-red-700">
            A chave de acesso não é de um CT-e válido; confira o link ou abra o CT-e pela lista.
          </p>
        ) : (
          <CteDetail accessKey={accessKey}/>
        )}
      </div>
    </RootLayout>
  );
}

export default function CteDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <CteDetailContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
