'use client';

import {Button, Modal, Textarea} from '@aoctech/ui';
import {type ReactNode, useState} from 'react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {JustificationField} from '@/components/ui/justification-field';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status';
import {cteEventFailureMessage} from '@/lib/utils/cte';
import {
  CTE_CORRECTION_ITEM_MAX,
  CTE_CORRECTION_VALUE_MAX,
  CTE_CORRECTIONS_MAX,
  CTE_EVENT_TEXT_MAX,
  CTE_EVENT_TEXT_MIN,
} from '@/lib/constants/cte';
import {CTE_CORRECTION_GROUPS, correctionGroup} from '@/lib/data/cte_correction';
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event';
import {formatDatetimeBR} from '@/lib/utils/dfe';
import type {CteCorrection, NfeEventOut} from '@/lib/types/api';

/** O que as ações precisam do CT-e aberto. */
export interface CteActionTarget {
  sk: string
  number: number | string
  /** Eventos 610110 com status `success`: os que o 610111 pode cancelar. */
  disagreements?: NfeEventOut[]
}

interface CorrectionRow extends CteCorrection {
  id: number
}

const SEQUENCE_OPTIONS = Array.from({length: 20}, (_, i) => ({value: String(i + 1), label: `Sequência ${i + 1}`}));
const GROUP_OPTIONS = CTE_CORRECTION_GROUPS.map((g) => ({value: g.value, label: g.label}));
const emptyRow = (id: number): CorrectionRow => ({id, group: '', field: '', value: ''});

const textOk = (v: string) => v.trim().length >= CTE_EVENT_TEXT_MIN;

function ModalError({message}: { message: string | null }) {
  return message ? <p role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-danger">{message}</p> : null;
}

/**
 * Cancelamento (110111), carta de correção (110110), prestação em desacordo (610110) e o cancelamento dela
 * (610111) do CT-e: mutações, estado dos modais e os próprios modais, como `useMdfeActions`. A api confere quem
 * pode (emitente ou tomador) e o status; a tela só oferece o botão quando vale e explica o 422.
 */
export function useCteActions(orgPk?: string) {
  const qc = useQueryClient();
  const [target, setTarget] = useState<CteActionTarget | null>(null);
  const [kind, setKind] = useState<'cancel' | 'correction' | 'disagree' | 'disagreeCancel' | null>(null);
  const [text, setText] = useState('');
  const [rows, setRows] = useState<CorrectionRow[]>([emptyRow(1)]);
  const [nextId, setNextId] = useState(2);
  const [sequence, setSequence] = useState('');
  const [eventSk, setEventSk] = useState('');
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setKind(null);
    setTarget(null);
    setText('');
    setRows([emptyRow(1)]);
    setNextId(2);
    setSequence('');
    setEventSk('');
    setError(null);
  };
  const open = (k: NonNullable<typeof kind>) => (t: CteActionTarget) => {
    close();
    setTarget(t);
    setKind(k);
  };

  const refresh = (accessKey: string) => {
    void qc.invalidateQueries({queryKey: queryKeys.ctes.detail(accessKey)});
    void qc.invalidateQueries({queryKey: queryKeys.ctes.events(accessKey)});
    void qc.invalidateQueries({queryKey: queryKeys.ctes.lists(orgPk)});
  };
  const seq = sequence ? Number(sequence) : undefined;

  const mutation = useMutation({
    mutationFn: async (): Promise<string> => {
      const key = target!.sk;
      switch (kind) {
        case 'cancel':
          await apiClient.cancelCte(key, text.trim());
          setDocStatusOptimistic(qc, queryKeys.ctes.lists(orgPk), key, 'cancel_pending');
          return 'Cancelamento enviado à SEFAZ.';
        case 'correction':
          await apiClient.correctCte(key, rows.map((r) => ({group: r.group, field: r.field, value: r.value.trim(), item_number: r.item_number})), seq);
          return 'Carta de correção enviada à SEFAZ.';
        case 'disagree':
          await apiClient.disagreeCteService(key, text.trim() || undefined, seq);
          return 'Prestação em desacordo enviada à SEFAZ.';
        default:
          await apiClient.cancelCteServiceDisagreement(key, eventSk, seq);
          return 'Cancelamento do desacordo enviado à SEFAZ.';
      }
    },
    onSuccess: (message) => {
      refresh(target!.sk);
      close();
      toast.success(message);
    },
    onError: (e) => setError(cteEventFailureMessage(e, 'Não foi possível enviar o evento.')),
  });

  const updateRow = (id: number, patch: Partial<CorrectionRow>) =>
    setRows((prev) => prev.map((r) => (r.id === id ? {...r, ...patch} : r)));

  const rowsReady = rows.length > 0 && rows.every((r) => {
    const g = correctionGroup(r.group);
    return !!g && !!r.field && r.value.trim().length > 0 && (!g.repeats || !!r.item_number);
  });
  const disagreements = target?.disagreements ?? [];

  const ready = kind === 'cancel' ? textOk(text)
    : kind === 'correction' ? rowsReady
      : kind === 'disagree' ? text.trim() === '' || textOk(text)
        : !!eventSk;

  const title = !target || !kind ? ''
    : {
      cancel: `Cancelar CT-e nº ${target.number}`,
      correction: `Carta de correção do CT-e nº ${target.number}`,
      disagree: `Prestação em desacordo do CT-e nº ${target.number}`,
      disagreeCancel: `Cancelar desacordo do CT-e nº ${target.number}`,
    }[kind];

  const submitLabel = {
    cancel: 'Confirmar cancelamento', correction: 'Enviar carta de correção',
    disagree: 'Registrar desacordo', disagreeCancel: 'Cancelar desacordo',
  };

  const sequenceField = (
    <div className="flex flex-col gap-1">
      <label htmlFor="cte-event-seq" className="text-xs font-medium text-gray-600">Número de sequência (opcional)</label>
      <OptionsSelect id="cte-event-seq" value={sequence} onValueChange={setSequence} clearable
                     options={SEQUENCE_OPTIONS} placeholder="Automático: o próximo livre"/>
    </div>
  );

  const modals: ReactNode = (
    <Modal
      open={kind !== null}
      title={title}
      onClose={close}
      onSubmit={() => ready && mutation.mutate()}
      submitLabel={kind ? submitLabel[kind] : ''}
      cancelLabel="Voltar"
      danger={kind === 'cancel'}
      loading={mutation.isPending}
      submitDisabled={!ready}
    >
      {kind === 'cancel' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. O CT-e será cancelado junto à
            SEFAZ, que confere o prazo e se há MDF-e ou CT-e de substituição ligado a ele.
          </p>
          <JustificationField id="cte-cancel-justification" value={text} onChange={setText}
                              minLength={CTE_EVENT_TEXT_MIN} maxLength={CTE_EVENT_TEXT_MAX}
                              placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"/>
          <ModalError message={error}/>
        </div>
      )}

      {kind === 'correction' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            A carta corrige dados do CT-e sem cancelá-lo; ele continua autorizado. A SEFAZ recusa o que não pode ser
            corrigido (valores do imposto, tomador, datas).
          </p>
          <ul className="space-y-3" aria-label="Correções">
            {rows.map((r, i) => {
              const g = correctionGroup(r.group);
              return (
                <li key={r.id} className="space-y-2 rounded-lg border border-gray-200 p-3">
                  <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                    <OptionsSelect id={`cte-corr-group-${i}`} ariaLabel={`Grupo da correção ${i + 1}`} value={r.group}
                                   options={GROUP_OPTIONS} placeholder="Grupo"
                                   onValueChange={(group) => updateRow(r.id, {group, field: '', item_number: undefined})}/>
                    <OptionsSelect id={`cte-corr-field-${i}`} ariaLabel={`Campo da correção ${i + 1}`} value={r.field}
                                   options={g?.fields ?? []} placeholder={g ? 'Campo' : 'Escolha o grupo antes'}
                                   disabled={!g} onValueChange={(field) => updateRow(r.id, {field})}/>
                  </div>
                  {g?.repeats && (
                    <NumericInput aria-label={`Número do item da correção ${i + 1}`} integerPlaces={2}
                                  value={r.item_number ? String(r.item_number) : ''} placeholder={`Número do item (1 a ${CTE_CORRECTION_ITEM_MAX})`}
                                  onChange={(v) => updateRow(r.id, {item_number: v ? Math.min(Number(v), CTE_CORRECTION_ITEM_MAX) || undefined : undefined})}/>
                  )}
                  <Textarea aria-label={`Novo valor da correção ${i + 1}`} rows={2} maxLength={CTE_CORRECTION_VALUE_MAX}
                            value={r.value} placeholder="Valor correto" className="resize-none"
                            onChange={(e) => updateRow(r.id, {value: e.target.value})}/>
                  {rows.length > 1 && (
                    <Button type="button" variant="ghost" size="sm" className="min-h-11 text-danger sm:min-h-0"
                            aria-label={`Remover correção ${i + 1}`}
                            onClick={() => setRows((prev) => prev.filter((x) => x.id !== r.id))}>
                      Remover
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
          <Button type="button" variant="outline" className="min-h-11 sm:min-h-0" disabled={rows.length >= CTE_CORRECTIONS_MAX}
                  onClick={() => {
                    setRows((prev) => [...prev, emptyRow(nextId)]);
                    setNextId(nextId + 1);
                  }}>
            Adicionar correção
          </Button>
          {sequenceField}
          <ModalError message={error}/>
        </div>
      )}

      {kind === 'disagree' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Informa à SEFAZ que o serviço de transporte não foi prestado como consta no CT-e. Só o tomador faz isto, em
            até 45 dias da autorização.
          </p>
          <JustificationField id="cte-disagree-notes" label="Observações (opcional)" value={text} onChange={setText}
                              minLength={CTE_EVENT_TEXT_MIN} maxLength={CTE_EVENT_TEXT_MAX}
                              placeholder="Explique o desacordo (15 a 255 caracteres)…"/>
          {sequenceField}
          <ModalError message={error}/>
        </div>
      )}

      {kind === 'disagreeCancel' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">Escolha o desacordo que a SEFAZ já autorizou e que você quer desfazer.</p>
          <div className="flex flex-col gap-1">
            <label htmlFor="cte-disagree-event" className="text-xs font-medium text-gray-600">Desacordo a cancelar</label>
            <OptionsSelect id="cte-disagree-event" value={eventSk} onValueChange={setEventSk}
                           placeholder="Escolha o evento"
                           options={disagreements.map((e) => ({
                             value: e.sk,
                             label: `${EVENT_TYPE_LABELS[e.event_type] ?? e.event_type} · seq. ${e.sequence_number} · ${formatDatetimeBR(e.created_at)}`,
                           }))}/>
          </div>
          {sequenceField}
          <ModalError message={error}/>
        </div>
      )}
    </Modal>
  );

  return {
    openCancel: open('cancel'), openCorrection: open('correction'),
    openDisagree: open('disagree'), openDisagreeCancel: open('disagreeCancel'), modals,
  };
}
