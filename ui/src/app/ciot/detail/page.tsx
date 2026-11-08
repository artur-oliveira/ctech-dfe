'use client';

import {Suspense} from 'react';
import Link from 'next/link';
import {useSearchParams} from 'next/navigation';
import {useQuery} from '@tanstack/react-query';
import {apiClient} from '@/lib/api/client';
import {useAuth} from '@/lib/hooks/useAuth';
import {useCiotPermissions} from '@/lib/hooks/useCiotPermissions';
import {queryKeys} from '@/lib/api/query-keys';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {Button, Skeleton} from '@aoctech/ui';
import {CiotStatusBadge} from '@/components/ciot/CiotStatusBadge';
import {useCiotActions} from '@/components/ciot/CiotActions';
import {CopyableCode} from '@/components/ui/copyable-code';
import {NoPermission} from '@/components/ui/no-permission';
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell';
import {CIOT_ENVIRONMENTS, CIOT_OPERATION_TYPES, CIOT_PENDING_LABELS} from '@/lib/constants/ciot';
import {canCancelCiot, canCloseCiot, canRectifyCiot} from '@/lib/utils/ciot';
import {formatCpfCnpj} from '@/lib/utils/document';
import {formatCurrency} from '@/lib/utils/helpers';
import {formatDatetimeBR} from '@/lib/utils/dfe';

const HISTORY_OPERATION_LABELS: Record<string, string> = {
  declare: 'Declaração', cancel: 'Cancelamento', rectify: 'Retificação', close: 'Encerramento',
};

function InfoCard({label, children}: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-1">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">{label}</p>
      {children}
    </div>
  );
}

function CiotDetail({id}: { id: string }) {
  const {selectedOrg} = useAuth();
  const {canGet, canUpdate} = useCiotPermissions();
  const {openCancel, openClose, modals} = useCiotActions(selectedOrg?.pk);

  const {data: ciot, isLoading, error} = useQuery({
    queryKey: queryKeys.ciots.detail(id),
    queryFn: () => apiClient.getCiot(id),
    enabled: !!id && !!selectedOrg && canGet,
  });

  if (!canGet) return <NoPermission what="ver CIOTs"/>;
  if (isLoading) {
    return <div className="space-y-4">{[...Array(3)].map((_, i) => <Skeleton key={i} className="h-24 rounded-xl"/>)}</div>;
  }
  if (error || !ciot) {
    return <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">CIOT não encontrado.</div>;
  }

  const target = {id: ciot.id, ciot: ciot.ciot, operation_type: ciot.operation_type, trip_start: ciot.trip_start};
  const decl = ciot.declaration;
  const operation = CIOT_OPERATION_TYPES.find((o) => o.value === String(ciot.operation_type))?.label;
  const env = CIOT_ENVIRONMENTS.find((e) => e.value === ciot.environment)?.label;
  const rejected = ciot.status === 'rejected' || ciot.status === 'failed' || !!ciot.last_error;

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-start justify-between gap-4 flex-wrap">
        <div>
          <p className="text-2xl font-semibold text-gray-900">
            {ciot.ciot ? <>CIOT <span className="font-mono">{ciot.ciot}</span></> : 'CIOT sem número ainda'}
          </p>
          {ciot.ciot && <CopyableCode value={ciot.ciot}/>}
          <p className="mt-1 text-xs text-gray-500">{operation}{env ? ` · ${env}` : ''}</p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <CiotStatusBadge ciot={ciot} size="md"/>
          {canUpdate && canRectifyCiot(ciot) && (
            <Button variant="outline" size="sm" render={<Link href={`/ciot/emit?rectify=${encodeURIComponent(ciot.id)}`}/>}
                    className="text-brand-600 border-brand-200 hover:bg-brand-50">
              Retificar
            </Button>
          )}
          {canUpdate && canCloseCiot(ciot) && (
            <Button variant="outline" size="sm" onClick={() => openClose(target)}
                    className="text-blue-600 border-blue-200 hover:bg-blue-50">
              Encerrar
            </Button>
          )}
          {canUpdate && canCancelCiot(ciot) && (
            <Button variant="outline" size="sm" onClick={() => openCancel(target)}
                    className="text-red-600 border-red-200 hover:bg-red-50">
              Cancelar
            </Button>
          )}
        </div>
      </div>

      {ciot.pending_operation && (
        <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          {CIOT_PENDING_LABELS[ciot.pending_operation]}. O resultado da ANTT chega aqui e por aviso; não é preciso repetir o pedido.
        </div>
      )}

      {ciot.carrier_warning && (
        <div className="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-wider text-blue-700">Aviso ao transportador</p>
          <p className="whitespace-pre-wrap wrap-break-word">{ciot.carrier_warning}</p>
          <p className="text-xs text-blue-800">A ANTT exige mostrar este aviso ao transportador e imprimi-lo.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => window.print()} className="min-h-11 bg-white sm:min-h-0">
            Imprimir
          </Button>
        </div>
      )}

      {rejected && ciot.last_error && (
        <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm space-y-1">
          <p className="text-xs font-semibold uppercase tracking-wider text-red-700">Rejeição</p>
          {ciot.last_error.antt_code && <p className="font-mono text-xs text-gray-600">Código ANTT: {ciot.last_error.antt_code}</p>}
          {ciot.last_error.internal_code && <p className="font-mono text-xs text-gray-600">Código interno: {ciot.last_error.internal_code}</p>}
          {ciot.last_error.antt_message && <p className="text-gray-800 whitespace-pre-wrap wrap-break-word">{ciot.last_error.antt_message}</p>}
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <InfoCard label="Transportador">
          <p className="font-mono text-sm text-gray-900">{ciot.carrier_doc ? formatCpfCnpj(ciot.carrier_doc) : '-'}</p>
          {decl?.carrier_rntrc && <p className="text-xs text-gray-500">RNTRC: {decl.carrier_rntrc}</p>}
        </InfoCard>
        <InfoCard label="Contratante">
          <p className="font-mono text-sm text-gray-900">{ciot.hirer_doc ? formatCpfCnpj(ciot.hirer_doc) : '-'}</p>
        </InfoCard>
        <InfoCard label="Frete">
          <p className="font-medium text-gray-900">{ciot.freight_value ? formatCurrency(ciot.freight_value) : '-'}</p>
        </InfoCard>
        <InfoCard label="Viagem">
          <p className="text-sm text-gray-900">{ciot.trip_start ? `${ciot.trip_start} a ` : 'Até '}{ciot.trip_end ?? '-'}</p>
        </InfoCard>
      </div>

      {ciot.mdfe_access_key && (
        <InfoCard label="MDF-e vinculado">
          <Link href={`/mdfe/detail?key=${ciot.mdfe_access_key}`} className="font-mono text-xs text-brand-600 hover:text-brand-700 break-all">
            {ciot.mdfe_access_key}
          </Link>
        </InfoCard>
      )}

      {decl?.vehicles && decl.vehicles.length > 0 && (
        <InfoCard label="Veículos">
          {decl.vehicles.map((v) => (
            <p key={v.plate} className="text-sm text-gray-700">
              <span className="font-mono font-medium">{v.plate}</span>
              <span className="ml-2 text-xs text-gray-500">{v.axles} eixos</span>
            </p>
          ))}
        </InfoCard>
      )}

      <TableShell ariaLabel="Histórico do CIOT" minWidth={120} headers={['Operação', 'Resultado', 'Código ANTT', 'Data']}>
        {(ciot.history ?? []).length === 0 ? (
          <tr><td colSpan={4} className={TABLE_CELL}>Nenhuma operação registrada ainda.</td></tr>
        ) : (
          (ciot.history ?? []).map((h) => (
            <tr key={`${h.at}-${h.operation}`} className={TABLE_ROW}>
              <td data-label="Operação" className={`${TABLE_CELL} font-medium text-gray-900`}>
                {HISTORY_OPERATION_LABELS[h.operation] ?? h.operation}
              </td>
              <td data-label="Resultado" className={TABLE_CELL}>
                <CiotStatusBadge ciot={{status: h.status as typeof ciot.status}}/>
                {h.internal_code && <p className="mt-0.5 font-mono text-xs text-gray-400">{h.internal_code}</p>}
              </td>
              <td data-label="Código ANTT" className={`${TABLE_CELL} font-mono text-xs text-gray-500`}>{h.antt_code ?? '-'}</td>
              <td data-label="Data" className={`${TABLE_CELL} text-xs text-gray-400 whitespace-nowrap`}>{formatDatetimeBR(h.at)}</td>
            </tr>
          ))
        )}
      </TableShell>

      {modals}
    </div>
  );
}

function CiotDetailContent() {
  const id = useSearchParams().get('id') ?? '';
  return (
    <RootLayout>
      <div className="p-4 md:p-8">
        <div className="flex items-center gap-2 text-sm text-gray-400 mb-6">
          <Link href="/ciot" className="hover:text-brand-600">CIOT</Link>
          <span>/</span>
          <span className="text-gray-600 font-mono truncate max-w-[200px]">{id || 'Detalhe'}</span>
        </div>
        {id ? <CiotDetail id={id}/> : <p className="text-sm text-gray-500">CIOT não informado.</p>}
      </div>
    </RootLayout>
  );
}

export default function CiotDetailPage() {
  return (
    <ProtectedRoute>
      <Suspense>
        <CiotDetailContent/>
      </Suspense>
    </ProtectedRoute>
  );
}
