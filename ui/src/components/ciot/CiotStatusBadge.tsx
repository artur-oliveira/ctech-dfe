'use client';

import {StatusBadge} from '@/components/ui/status-badge';
import {CIOT_PENDING_LABELS, CIOT_STATUS_CLASSES, CIOT_STATUS_LABELS} from '@/lib/constants/ciot';
import type {CiotOut} from '@/lib/types/api';

/** Status do CIOT; uma operação em andamento (cancelar, retificar, encerrar) aparece pulsando no lugar do status. */
export function CiotStatusBadge({ciot, size}: { ciot: Pick<CiotOut, 'status' | 'pending_operation'>; size?: 'sm' | 'md' }) {
  if (ciot.pending_operation && ciot.status !== 'pending') {
    return <StatusBadge label={CIOT_PENDING_LABELS[ciot.pending_operation]} className="bg-amber-50 text-amber-700" isTransitional size={size}/>;
  }
  return (
    <StatusBadge
      label={CIOT_STATUS_LABELS[ciot.status] ?? ciot.status}
      className={CIOT_STATUS_CLASSES[ciot.status] ?? 'bg-gray-100 text-gray-600'}
      isTransitional={ciot.status === 'pending'}
      size={size}
    />
  );
}
