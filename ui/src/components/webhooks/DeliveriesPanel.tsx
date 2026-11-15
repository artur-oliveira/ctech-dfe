'use client';

import {useState} from 'react';
import {useInfiniteQuery, useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Button, Skeleton} from '@aoctech/ui';
import {toast} from 'sonner';
import {apiClient, ApiError} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {StatusBadge} from '@/components/ui/status-badge';
import {DELIVERY_STATUS, WEBHOOK_EVENT_LABELS} from '@/lib/constants/webhooks';
import {formatDateTime} from '@/lib/webhooks/format';
import type {WebhookDelivery} from '@/lib/types/webhooks';

interface DeliveriesPanelProps {
  companyPk: string
  endpointId: string
  /** A disabled endpoint cannot be replayed to (the server refuses it). */
  canReplay: boolean
}

export function DeliveriesPanel({companyPk, endpointId, canReplay}: DeliveriesPanelProps) {
  const qc = useQueryClient();
  const query = useInfiniteQuery({
    queryKey: queryKeys.webhooks.deliveries(companyPk, endpointId),
    queryFn: ({pageParam}) => apiClient.listWebhookDeliveries(endpointId, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });
  const [open, setOpen] = useState<string | null>(null);

  const replay = useMutation({
    mutationFn: (eventId: string) => apiClient.replayWebhookDelivery(endpointId, eventId),
    onSuccess: () => {
      toast.success('Entrega reenviada');
      void qc.invalidateQueries({queryKey: queryKeys.webhooks.deliveries(companyPk, endpointId)});
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.detail : 'Não foi possível reenviar'),
  });

  if (query.isPending) {
    return <div className="space-y-2 p-4">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-9 rounded"/>)}</div>;
  }
  if (query.isError) {
    return (
      <div className="flex flex-wrap items-center gap-3 p-4 text-sm text-destructive" role="alert">
        Não foi possível carregar as entregas.
        <Button variant="outline" size="sm" onClick={() => void query.refetch()}>Tentar de novo</Button>
      </div>
    );
  }
  const rows = query.data.pages.flatMap((p) => p.data);
  if (rows.length === 0) {
    return <p className="p-4 text-sm text-gray-500">Nenhuma entrega ainda. Use Testar para enviar um evento de exemplo.</p>;
  }

  return (
    <div>
      <ul className="divide-y divide-gray-100">
        {rows.map((d) => (
          <DeliveryRow
            key={d.event_id}
            delivery={d}
            companyPk={companyPk}
            endpointId={endpointId}
            expanded={open === d.event_id}
            onToggle={() => setOpen((cur) => (cur === d.event_id ? null : d.event_id))}
            canReplay={canReplay}
            replaying={replay.isPending && replay.variables === d.event_id}
            onReplay={() => replay.mutate(d.event_id)}
          />
        ))}
      </ul>
      {query.hasNextPage && (
        <div className="border-t border-gray-100 p-3 text-center">
          <Button variant="ghost" size="sm" disabled={query.isFetchingNextPage} onClick={() => void query.fetchNextPage()}>
            {query.isFetchingNextPage ? 'Carregando...' : 'Mostrar mais'}
          </Button>
        </div>
      )}
    </div>
  );
}

function DeliveryRow({delivery: d, companyPk, endpointId, expanded, onToggle, canReplay, replaying, onReplay}: {
  delivery: WebhookDelivery
  companyPk: string
  endpointId: string
  expanded: boolean
  onToggle: () => void
  canReplay: boolean
  replaying: boolean
  onReplay: () => void
}) {
  const status = DELIVERY_STATUS[d.status];
  const when = formatDateTime(d.delivered_at ?? d.created_at);
  return (
    <li>
      <div className="flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-gray-900">{WEBHOOK_EVENT_LABELS[d.type] ?? d.type}</p>
          <p className="text-xs text-gray-500 tabular-nums">
            {when}{when && ' · '}{d.attempts} {d.attempts === 1 ? 'tentativa' : 'tentativas'}
            {d.status === 'pending' && d.next_attempt_at && ` · próxima ${formatDateTime(d.next_attempt_at)}`}
          </p>
        </div>
        <StatusBadge label={status.label} className={status.className}/>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="sm" className="h-11 sm:h-9" aria-expanded={expanded} onClick={onToggle}>
            {expanded ? 'Ocultar' : 'Detalhes'}
          </Button>
          <Button variant="outline" size="sm" className="h-11 sm:h-9" disabled={!canReplay || replaying} onClick={onReplay}
                  title={canReplay ? undefined : 'O endpoint está desativado'}>
            {replaying ? 'Reenviando...' : 'Reenviar'}
          </Button>
        </div>
      </div>
      {expanded && <Attempts companyPk={companyPk} endpointId={endpointId} eventId={d.event_id}/>}
    </li>
  );
}

function Attempts({companyPk, endpointId, eventId}: { companyPk: string; endpointId: string; eventId: string }) {
  const q = useQuery({
    queryKey: queryKeys.webhooks.delivery(companyPk, endpointId, eventId),
    queryFn: () => apiClient.getWebhookDelivery(endpointId, eventId),
  });
  if (q.isPending) return <Skeleton className="mx-4 mb-3 h-16 rounded"/>;
  if (q.isError) return <p className="px-4 pb-3 text-sm text-destructive" role="alert">Não foi possível carregar as tentativas.</p>;
  return (
    <div className="mx-4 mb-3 overflow-x-auto rounded-lg border border-gray-200">
      <table className="w-full text-left text-xs">
        <thead className="bg-gray-50 text-gray-500">
          <tr>
            <th className="px-3 py-2 font-medium">#</th>
            <th className="px-3 py-2 font-medium">Quando</th>
            <th className="px-3 py-2 font-medium">Resposta</th>
            <th className="px-3 py-2 font-medium">Duração</th>
            <th className="px-3 py-2 font-medium">Detalhe</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 tabular-nums">
          {q.data.attempts.map((a) => (
            <tr key={a.number}>
              <td className="px-3 py-2">{a.number}</td>
              <td className="px-3 py-2 whitespace-nowrap">{formatDateTime(a.at)}</td>
              <td className="px-3 py-2">{a.status_code ? `HTTP ${a.status_code}` : 'sem resposta'}</td>
              <td className="px-3 py-2 whitespace-nowrap">{a.duration_ms} ms</td>
              <td className="max-w-[18rem] break-words px-3 py-2 font-mono text-gray-600">{a.response || a.error || ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
