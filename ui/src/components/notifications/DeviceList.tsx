'use client';

import {Button} from '@aoctech/ui';
import {formatDateTime} from '@/lib/webhooks/format';
import type {PushDevice} from '@/lib/types/push';

interface DeviceListProps {
  devices: PushDevice[]
  max: number
  /** Id of this browser's own subscription, to mark it. */
  thisDeviceId: string | null
  /** A removal is in flight. */
  busy: boolean
  onRemove: (device: PushDevice) => void
}

/** The person's devices (no endpoints: the API never returns them), each removable. */
export function DeviceList({devices, max, thisDeviceId, busy, onRemove}: DeviceListProps) {
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-4 md:p-6" aria-labelledby="push-devices-title">
      <h2 id="push-devices-title" className="text-base font-semibold text-gray-900">Meus dispositivos</h2>
      <p className="mt-1 text-sm text-gray-500">
        Até {max} dispositivos. Ao passar do limite, o que ficou mais tempo sem uso é substituído.
      </p>
      {devices.length === 0 ? (
        <p className="mt-3 text-sm text-gray-500">Nenhum dispositivo ativo. Ative as notificações em um deles acima.</p>
      ) : (
        <ul className="mt-3 divide-y divide-gray-100">
          {devices.map((d) => {
            const seen = formatDateTime(d.last_seen_at);
            return (
              <li key={d.id} className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="min-w-0">
                  <p className="text-sm font-medium text-gray-900">
                    {d.device}
                    {d.id === thisDeviceId && <span className="ml-2 rounded bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">Este dispositivo</span>}
                  </p>
                  {seen && <p className="text-xs text-gray-500">Último uso em {seen}</p>}
                </div>
                <Button variant="outline" className="h-11 sm:h-9" disabled={busy} onClick={() => onRemove(d)}>Remover</Button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
