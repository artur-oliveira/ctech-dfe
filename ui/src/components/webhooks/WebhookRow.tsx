'use client';

import {useState} from 'react';
import {Button} from '@aoctech/ui';
import {StatusBadge} from '@/components/ui/status-badge';
import {DeliveriesPanel} from '@/components/webhooks/DeliveriesPanel';
import {WEBHOOK_MAX_FAILURE_RUN} from '@/lib/constants/webhooks';
import {filterSummary} from '@/lib/webhooks/describe';
import {formatDateTime} from '@/lib/webhooks/format';
import type {WebhookEndpoint} from '@/lib/types/webhooks';

interface WebhookRowProps {
  endpoint: WebhookEndpoint
  companyPk: string
  companyNames: Map<string, string>
  busy: boolean
  onPing: () => void
  onRotate: () => void
  onRemove: () => void
}

function lastDelivery(e: WebhookEndpoint): string {
  const when = formatDateTime(e.last_delivery_at);
  if (!when) return 'Nenhuma entrega ainda';
  return e.last_delivery_ok ? `Última entrega ok em ${when}` : `Última entrega falhou em ${when}${e.last_delivery_detail ? ` (${e.last_delivery_detail})` : ''}`;
}

export function WebhookRow({endpoint: e, companyPk, companyNames, busy, onPing, onRotate, onRemove}: WebhookRowProps) {
  const [showDeliveries, setShowDeliveries] = useState(false);
  const [confirmRotate, setConfirmRotate] = useState(false);
  const active = e.status === 'active';
  const summary = filterSummary(e.events, e.company_pks, companyNames);

  return (
    <li className="px-4 py-4 md:px-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <p className="min-w-0 break-all font-mono text-sm text-gray-900">{e.url}</p>
        <StatusBadge
          label={active ? 'Ativo' : 'Desativado'}
          className={active ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'}
        />
      </div>

      <dl className="mt-2 grid gap-x-6 gap-y-1 text-xs text-gray-500 sm:grid-cols-2">
        <div><dt className="inline font-medium text-gray-600">Eventos: </dt><dd className="inline">{summary.events}</dd></div>
        <div><dt className="inline font-medium text-gray-600">Empresas: </dt><dd className="inline">{summary.companies}</dd></div>
        <div className="sm:col-span-2 tabular-nums">{lastDelivery(e)}</div>
      </dl>

      {!active && (
        <p role="note" className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-900">
          Desativado em {formatDateTime(e.disabled_at) || 'data desconhecida'} após {WEBHOOK_MAX_FAILURE_RUN} falhas seguidas
          {e.last_delivery_detail ? ` (${e.last_delivery_detail})` : ''}. Corrija o seu sistema, remova este webhook e cadastre-o de novo.
        </p>
      )}

      {confirmRotate ? (
        <div className="mt-3 flex flex-col gap-2 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-900 sm:flex-row sm:items-center">
          <p className="flex-1">O segredo atual para de valer na hora. Atualize o seu sistema com o novo segredo logo em seguida.</p>
          <div className="flex gap-2">
            <Button size="sm" className="h-11 sm:h-9" disabled={busy}
                    onClick={() => { setConfirmRotate(false); onRotate(); }}>
              Trocar o segredo
            </Button>
            <Button size="sm" variant="outline" className="h-11 sm:h-9" onClick={() => setConfirmRotate(false)}>Cancelar</Button>
          </div>
        </div>
      ) : (
        <div className="mt-3 flex flex-wrap gap-2">
          <Button size="sm" variant="outline" className="h-11 sm:h-9" disabled={!active || busy} onClick={onPing}
                  title={active ? undefined : 'O endpoint está desativado'}>
            Testar
          </Button>
          <Button size="sm" variant="outline" className="h-11 sm:h-9" aria-expanded={showDeliveries}
                  onClick={() => setShowDeliveries((v) => !v)}>
            {showDeliveries ? 'Ocultar entregas' : 'Entregas'}
          </Button>
          <Button size="sm" variant="ghost" className="h-11 sm:h-9" disabled={busy} onClick={() => setConfirmRotate(true)}>
            Trocar segredo
          </Button>
          <Button size="sm" variant="ghost" className="h-11 text-danger hover:text-red-700 sm:h-9" disabled={busy} onClick={onRemove}>
            Remover
          </Button>
        </div>
      )}

      {showDeliveries && (
        <div className="mt-3 rounded-lg border border-gray-200">
          <DeliveriesPanel companyPk={companyPk} endpointId={e.id} canReplay={active}/>
        </div>
      )}
    </li>
  );
}
