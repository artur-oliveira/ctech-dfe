'use client';

import {useState} from 'react';
import {Modal} from '@aoctech/ui';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {EventSequenceField} from '@/components/dfe/EventSequenceField';
import {PaymentLinkFields} from '@/components/nfe/ReformEventFields';
import {
  emptyPaymentLinkDraft, paymentLinkBody, paymentLinkCancelBody, paymentLinkIssues, PAYMENT_NUMBER_MAX,
} from '@/lib/utils/cte-payment-link';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import type {NfeEventOut} from '@/lib/types/api';

export type PaymentLinkKind = 'link' | 'cancel'
export type PaymentLinkRoute = 'payment-link' | 'payment-link-cancel'

interface Props {
  kind: PaymentLinkKind | null
  /** Nome do documento no texto ("NFCom", "BP-e"). */
  docLabel: string
  docNumber: string | number
  /** Vinculações (110300) já autorizadas, as que o 110301 pode cancelar. */
  links: NfeEventOut[]
  loading: boolean
  error?: unknown
  onClose: () => void
  onConfirm: (route: PaymentLinkRoute, body: object) => void
}

/**
 * Vinculação de pagamento (110300) e cancelamento dela (110301), igual para NFCom e BP-e. A vinculação pede número do
 * pagamento, transação, meio (seletor), recebedor e PSP; o cancelamento escolhe uma vinculação autorizada, e o protocolo
 * dela vem do servidor.
 */
export function PaymentLinkModal({kind, docLabel, docNumber, links, loading, error, onClose, onConfirm}: Props) {
  const [draft, setDraft] = useState(emptyPaymentLinkDraft());
  const [eventSk, setEventSk] = useState('');
  const [sequence, setSequence] = useState('');
  if (!kind) return null;
  const close = () => { setDraft(emptyPaymentLinkDraft()); setEventSk(''); setSequence(''); onClose(); };
  const ready = kind === 'link' ? paymentLinkIssues(draft).length === 0 : eventSk !== '';
  const submit = () => {
    if (!ready) return;
    if (kind === 'link') onConfirm('payment-link', paymentLinkBody(draft, sequence));
    else onConfirm('payment-link-cancel', paymentLinkCancelBody(eventSk, sequence));
  };
  return (
    <Modal open title={`${kind === 'link' ? 'Vincular pagamento' : 'Cancelar vinculação de pagamento'} (${docLabel} nº ${docNumber})`}
           onClose={close} onSubmit={submit} submitLabel={kind === 'link' ? 'Vincular pagamento' : 'Cancelar vinculação'}
           cancelLabel="Voltar" danger={kind === 'cancel'} loading={loading} submitDisabled={!ready}>
      <div className="space-y-4">
        {kind === 'link' ? (
          <>
            <p className="text-sm text-gray-600">Vincula a este documento a transação de pagamento (meio, recebedor e PSP). Cada vinculação tem o seu número.</p>
            <div className="flex flex-col gap-1">
              <label htmlFor="payment-link-number" className="text-xs font-medium text-gray-600">Número do pagamento</label>
              <NumericInput id="payment-link-number" integerPlaces={3} value={draft.number} placeholder={`1 a ${PAYMENT_NUMBER_MAX}`}
                            onChange={(number) => setDraft((d) => ({...d, number: number && Number(number) > 0 ? String(Math.min(Number(number), PAYMENT_NUMBER_MAX)) : ''}))}/>
            </div>
            <PaymentLinkFields prefix="payment-link" value={draft.payment} onChange={(payment) => setDraft((d) => ({...d, payment}))}/>
          </>
        ) : (
          <>
            <p className="text-sm text-gray-600">Escolha a vinculação de pagamento que o autorizador já aceitou e que você quer desfazer.</p>
            <div className="flex flex-col gap-1">
              <label htmlFor="payment-link-event" className="text-xs font-medium text-gray-600">Vinculação a cancelar</label>
              <OptionsSelect id="payment-link-event" value={eventSk} onValueChange={setEventSk} placeholder="Escolha o evento"
                             options={links.map((e) => ({value: e.sk, label: `Vinculação de pagamento · seq. ${e.sequence_number} · ${formatDatetimeBR(e.created_at)}`}))}/>
            </div>
          </>
        )}
        <EventSequenceField id="payment-link-seq" value={sequence} onChange={setSequence}/>
        {error != null && <p className="text-xs text-red-600">{(error as Error)?.message ?? 'Erro ao registrar o evento.'}</p>}
      </div>
    </Modal>
  );
}
