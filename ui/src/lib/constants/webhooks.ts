import type {WebhookDeliveryStatus, WebhookEventType} from '@/lib/types/webhooks';

/** Events a webhook can subscribe to, in the order the form lists them. */
export const WEBHOOK_EVENTS: { value: WebhookEventType; label: string; hint: string }[] = [
  {value: 'document.authorized', label: 'Autorizada', hint: 'A SEFAZ ou a prefeitura autorizou o documento'},
  {value: 'document.rejected', label: 'Rejeitada', hint: 'A autoridade recusou o documento; o motivo vai junto'},
  {value: 'document.failed', label: 'Falha na emissão', hint: 'O emissor não conseguiu concluir a emissão'},
];

export const WEBHOOK_EVENT_LABELS: Record<string, string> = {
  ...Object.fromEntries(WEBHOOK_EVENTS.map((e) => [e.value, e.label])),
  ping: 'Teste',
};

/** Consecutive failures that disable an endpoint (go-common webhooks.MaxFailureRun). */
export const WEBHOOK_MAX_FAILURE_RUN = 12;

export const DELIVERY_STATUS: Record<WebhookDeliveryStatus, { label: string; className: string }> = {
  pending: {label: 'Pendente', className: 'bg-amber-50 text-amber-800'},
  delivered: {label: 'Entregue', className: 'bg-green-50 text-green-800'},
  failed: {label: 'Falhou', className: 'bg-red-50 text-red-800'},
};

export const DELIVERIES_PAGE_SIZE = 20;
