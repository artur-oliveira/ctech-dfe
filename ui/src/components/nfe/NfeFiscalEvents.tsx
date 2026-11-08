'use client';

import dynamic from 'next/dynamic';
import {type ReactNode, useState} from 'react';
import {Modal} from '@aoctech/ui';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {DeliveryProofFields} from '@/components/dfe/DeliveryProofFields';
import {EventSequenceField} from '@/components/dfe/EventSequenceField';
import {EventsMenu, type EventsMenuGroup} from '@/components/dfe/EventsMenu';
import type {PickedDoc} from '@/components/dfe/PersonDocField';
import {OptionsSelect} from '@/components/ui/options-select';
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event';
import {
  MOC_LAYOUT_CODES, MOC_LAYOUT_NOTICE, buildReformBody, emptyReformDraft, reformEventDef, reformEventsFor, reformIssues, REFORM_EVENTS_NOTICE,
  type ReformDraft,
} from '@/lib/data/nfe_reform_events';
import {
  actorIssues, emptyProof, interestedActorBody, nfeReceiptBody, proofIssues, receiptCancelBody,
  DELIVERY_PROTOCOL_LENGTH, type ProofDraft,
} from '@/lib/utils/delivery-events';
import {cteEventFailureMessage} from '@/lib/utils/event-failure';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import type {NfeEventOut} from '@/lib/types/api';

// O seletor de pessoas e os campos da reforma arrastam o cadastro de pessoas e as tabelas; só carregam ao abrir um modal.
const PersonDocField = dynamic(() => import('@/components/dfe/PersonDocField').then((m) => m.PersonDocField));
const ReformEventFields = dynamic(() => import('@/components/nfe/ReformEventFields').then((m) => m.ReformEventFields));

const EVENT_RECEIPT = '110130';
const EVENT_RECEIPT_CANCEL = '110131';
const EVENT_ACTOR = '110150';
const MODE_RECEIPT = 'receipt';
const MODE_RECEIPT_CANCEL = 'receiptCancel';
const MODE_ACTOR = 'actor';
const REFORM_PREFIX = 'reform:';

export interface NfeEventsTarget {
  sk: string
  number: number | string
  /** NF-e recebida (incoming) em vez de emitida pela organização. */
  incoming: boolean
  /** Eventos 110130 com status `success`: os que o 110131 pode cancelar. */
  receipts: NfeEventOut[]
}

/**
 * Menu "Eventos fiscais" da NF-e autorizada e os modais dos eventos F9 (comprovante de entrega, ator interessado e
 * os 12 da reforma tributária). Quem pode e o status a api confere; aqui só se oferece o que cabe e se explica o 422.
 */
export function useNfeFiscalEvents(orgPk?: string) {
  const qc = useQueryClient();
  const [target, setTarget] = useState<NfeEventsTarget | null>(null);
  const [mode, setMode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sequence, setSequence] = useState('');
  const [reform, setReform] = useState<ReformDraft>(emptyReformDraft());
  const [nextId, setNextId] = useState(2);
  const [proof, setProof] = useState<ProofDraft>(emptyProof());
  const [protocol, setProtocol] = useState('');
  const [actor, setActor] = useState<PickedDoc | null>(null);
  const [allowSub, setAllowSub] = useState(false);

  const close = () => {
    setMode(null); setTarget(null); setError(null); setSequence(''); setReform(emptyReformDraft()); setNextId(2);
    setProof(emptyProof()); setProtocol(''); setActor(null); setAllowSub(false);
  };
  const open = (t: NfeEventsTarget, m: string) => { close(); setTarget(t); setMode(m); };

  const def = mode?.startsWith(REFORM_PREFIX) ? reformEventDef(mode.slice(REFORM_PREFIX.length)) : undefined;
  const issues: string[] = !mode ? []
    : def ? reformIssues(def, reform)
      : mode === MODE_RECEIPT ? proofIssues('nfe-receipt', proof)
        : mode === MODE_RECEIPT_CANCEL ? (protocol ? [] : ['Escolha o comprovante a cancelar.'])
          : actorIssues(actor?.doc ?? '');

  const mutation = useMutation({
    mutationFn: async () => {
      const key = target!.sk;
      if (def) return apiClient.requestNfeFiscalEvent(key, 'ibscbs-events', buildReformBody(def, {...reform, sequence}));
      if (mode === MODE_RECEIPT) return apiClient.requestNfeFiscalEvent(key, 'delivery-receipt', nfeReceiptBody({...proof, sequence}));
      if (mode === MODE_RECEIPT_CANCEL) return apiClient.requestNfeFiscalEvent(key, 'delivery-receipt-cancel', receiptCancelBody(protocol, sequence));
      return apiClient.requestNfeFiscalEvent(key, 'interested-actor',
        interestedActorBody(actor!.doc, allowSub, sequence, target!.incoming ? '2' : undefined));
    },
    onSuccess: () => {
      const key = target!.sk;
      void qc.invalidateQueries({queryKey: queryKeys.nfes.detail(key)});
      void qc.invalidateQueries({queryKey: queryKeys.nfes.events(key)});
      void qc.invalidateQueries({queryKey: queryKeys.nfes.lists(orgPk)});
      close();
      toast.success('Evento enviado à SEFAZ. O resultado aparece na lista de eventos.');
    },
    onError: (e) => setError(cteEventFailureMessage(e, 'Não foi possível enviar o evento.')),
  });

  const title = !target || !mode ? ''
    : def ? `${def.label}: NF-e nº ${target.number}`
      : { [MODE_RECEIPT]: 'Comprovante de entrega', [MODE_RECEIPT_CANCEL]: 'Cancelar comprovante de entrega', [MODE_ACTOR]: 'Ator interessado' }[mode] + `: NF-e nº ${target.number}`;

  const menu = (t: NfeEventsTarget, can: boolean): ReactNode => {
    if (!can) return null;
    const groups: EventsMenuGroup[] = [
      {
        label: 'Entrega',
        items: t.incoming ? [] : [
          {key: MODE_RECEIPT, label: 'Comprovante de entrega', hint: '110130', onSelect: () => open(t, MODE_RECEIPT)},
          ...(t.receipts.length ? [{key: MODE_RECEIPT_CANCEL, label: 'Cancelar comprovante', hint: '110131', onSelect: () => open(t, MODE_RECEIPT_CANCEL)}] : []),
        ],
      },
      {label: 'Transporte', items: [{key: MODE_ACTOR, label: 'Ator interessado', hint: '110150', onSelect: () => open(t, MODE_ACTOR)}]},
      {
        label: 'Reforma tributária',
        note: 'A SEFAZ pode não aceitar estes eventos antes de 16/11/2026.',
        items: reformEventsFor(t.incoming).map((e) => ({key: e.code, label: e.label, hint: `${e.code} · ${e.who}`, onSelect: () => open(t, REFORM_PREFIX + e.code)})),
      },
    ];
    return <EventsMenu groups={groups}/>;
  };

  const receiptOptions = (target?.receipts ?? []).filter((e) => e.sefaz_protocol).map((e) => ({
    value: e.sefaz_protocol as string,
    label: `${EVENT_TYPE_LABELS[e.event_type] ?? e.event_type} · seq. ${e.sequence_number} · ${formatDatetimeBR(e.created_at)}`,
  }));

  const modals: ReactNode = (
    <Modal open={mode !== null} title={title} onClose={close} onSubmit={() => issues.length === 0 && mutation.mutate()}
           submitLabel="Enviar evento" cancelLabel="Voltar" loading={mutation.isPending} submitDisabled={issues.length > 0}>
      <div className="space-y-4">
        {def && (
          <>
            <p className="text-sm text-gray-600">{def.summary} Emitido por: {def.who.toLowerCase()}.</p>
            <p className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">{REFORM_EVENTS_NOTICE}</p>
            <ReformEventFields def={def} draft={reform} nextId={nextId} onNextId={setNextId}
                               onChange={(patch) => setReform((d) => ({...d, ...patch}))}/>
          </>
        )}
        {mode === MODE_RECEIPT && (
          <>
            <p className="text-sm text-gray-600">Registra no Ambiente Nacional que a mercadoria foi entregue. Só o emitente envia.</p>
            <DeliveryProofFields prefix="nfe-receipt" kind="nfe-receipt" value={proof} onChange={(p) => setProof((d) => ({...d, ...p}))}/>
          </>
        )}
        {mode === MODE_RECEIPT_CANCEL && (
          <div className="flex flex-col gap-1">
            <p className="text-sm text-gray-600">Escolha o comprovante que a SEFAZ já autorizou e que você quer desfazer.</p>
            <label htmlFor="nfe-receipt-event" className="text-xs font-medium text-gray-600">Comprovante a cancelar</label>
            <OptionsSelect id="nfe-receipt-event" value={protocol} onValueChange={setProtocol} options={receiptOptions} placeholder="Escolha o comprovante"/>
            {protocol && <p className="text-xs text-gray-500">Protocolo de {DELIVERY_PROTOCOL_LENGTH} dígitos: <span className="font-mono">{protocol}</span></p>}
          </div>
        )}
        {mode === MODE_ACTOR && (
          <>
            <p className="text-sm text-gray-600">Indica o transportador que pode baixar esta NF-e no portal da SEFAZ.</p>
            <PersonDocField id="nfe-actor" label="Transportador (CPF ou CNPJ)" value={actor} onChange={setActor}/>
            <label className="flex min-h-11 items-start gap-2 text-sm text-gray-800">
              <input type="checkbox" className="mt-1 size-5" checked={allowSub} onChange={(e) => setAllowSub(e.target.checked)}/>
              <span>Permitir que o transportador autorize subcontratados e redespachos a baixar a NF-e</span>
            </label>
          </>
        )}
        {mode && (def ? MOC_LAYOUT_CODES.includes(def.code) : true) && (
          <p className="text-xs text-gray-500">{MOC_LAYOUT_NOTICE}</p>
        )}
        <EventSequenceField id="nfe-fiscal-seq" value={sequence} onChange={setSequence}/>
        {issues.length > 0 && mode && (
          <ul className="list-disc space-y-0.5 pl-5 text-xs text-gray-500" aria-label="Falta preencher">
            {issues.slice(0, 3).map((i) => <li key={i}>{i}</li>)}
          </ul>
        )}
        {error && <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-danger">{error}</p>}
      </div>
    </Modal>
  );

  return {menu, modals};
}

export const NFE_EVENT = {RECEIPT: EVENT_RECEIPT, RECEIPT_CANCEL: EVENT_RECEIPT_CANCEL, ACTOR: EVENT_ACTOR} as const;
