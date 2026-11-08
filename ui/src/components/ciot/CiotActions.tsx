'use client';

import {Modal} from '@aoctech/ui';
import {type ReactNode, useState} from 'react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {EmitError} from '@/components/ui/emit-error';
import {Input} from '@/components/ui/input';
import {JustificationField} from '@/components/ui/justification-field';
import {NumericInput} from '@/components/ui/numeric-input';
import {Combobox} from '@/components/ui/combobox';
import {
  CIOT_CANCEL_REASON_MAX_LENGTH,
  CIOT_CANCEL_WINDOW_HOURS,
  OPERATION_LOTACAO,
  OPERATION_TAC_AGREGADO,
} from '@/lib/constants/ciot';
import {CITY_OPTIONS} from '@/lib/data/cities';
import {cancelWindow, problemLines} from '@/lib/utils/ciot';
import type {CiotCloseBody, CiotOut} from '@/lib/types/api';

type Target = Pick<CiotOut, 'id' | 'ciot' | 'operation_type' | 'trip_start'>

const MAX_WEIGHT = 9_999_999.99;
const label = (t: Target) => t.ciot ?? t.id;

/**
 * Cancelar e encerrar CIOT: mutações, estado dos diálogos e os próprios diálogos, para a lista e o detalhe
 * compartilharem uma implementação. A retificação é uma tela (`/ciot/emit?rectify=`), não um diálogo.
 * O resultado chega por WebSocket; aqui só marca a operação como em andamento.
 */
export function useCiotActions(orgPk?: string) {
  const qc = useQueryClient();
  const [cancelTarget, setCancelTarget] = useState<Target | null>(null);
  const [reason, setReason] = useState('');
  const [closeTarget, setCloseTarget] = useState<Target | null>(null);
  const [closedAt, setClosedAt] = useState('');
  const [weight, setWeight] = useState('');
  const [tripCount, setTripCount] = useState('');
  const [origin, setOrigin] = useState('');
  const [destination, setDestination] = useState('');
  const [failure, setFailure] = useState<EmitFailure | null>(null);
  const [lines, setLines] = useState<string[]>([]);

  const refresh = (id: string) => {
    void qc.invalidateQueries({queryKey: queryKeys.ciots.detail(id)});
    void qc.invalidateQueries({queryKey: queryKeys.ciots.lists(orgPk)});
  };
  const fail = (e: unknown, fallback: string) => {
    setFailure(emitFailure(e, fallback));
    setLines(problemLines(e));
  };
  const clearFailure = () => {
    setFailure(null);
    setLines([]);
  };

  const cancel = useMutation({
    mutationFn: ({id, text}: { id: string; text: string }) => apiClient.cancelCiot(id, text),
    onSuccess: (_d, {id}) => {
      setCancelTarget(null);
      setReason('');
      refresh(id);
      toast.success('Cancelamento enviado à ANTT.');
    },
    onError: (e) => fail(e, 'Erro ao cancelar o CIOT.'),
  });

  const close = useMutation({
    mutationFn: ({id, body}: { id: string; body: CiotCloseBody }) => apiClient.closeCiot(id, body),
    onSuccess: (_d, {id}) => {
      setCloseTarget(null);
      refresh(id);
      toast.success('Encerramento enviado à ANTT.');
    },
    onError: (e) => fail(e, 'Erro ao encerrar o CIOT.'),
  });

  const openCancel = (t: Target) => {
    clearFailure();
    setReason('');
    setCancelTarget(t);
  };
  const openClose = (t: Target) => {
    clearFailure();
    setClosedAt('');
    setWeight('');
    setTripCount('');
    setOrigin('');
    setDestination('');
    setCloseTarget(t);
  };

  const window24 = cancelWindow(cancelTarget?.trip_start);
  const isFull = closeTarget?.operation_type === OPERATION_LOTACAO;
  const isTac = closeTarget?.operation_type === OPERATION_TAC_AGREGADO;
  const weightOk = Number(weight) > 0 && Number(weight) < MAX_WEIGHT;
  const tacOk = tripCount !== '' && !!origin && !!destination;
  const closeReady = isFull ? weightOk : isTac ? tacOk : true;

  const submitClose = () => {
    if (!closeTarget || !closeReady) return;
    const body: CiotCloseBody = {
      ...(closedAt && {closed_at: closedAt}),
      ...(isFull && {cargo_weight: weight}),
      ...(isTac && {trip_count: Number(tripCount), routes: [{origin_ibge: origin, destination_ibge: destination}]}),
    };
    close.mutate({id: closeTarget.id, body});
  };

  const errors = (
    <>
      <EmitError failure={failure}/>
      {lines.length > 0 && (
        <ul role="alert" className="list-disc space-y-1 pl-5 text-xs text-danger">
          {lines.map((l) => <li key={l}>{l}</li>)}
        </ul>
      )}
    </>
  );

  const modals: ReactNode = (
    <>
      <Modal
        open={cancelTarget !== null}
        title={cancelTarget ? `Cancelar o CIOT ${label(cancelTarget)}` : ''}
        onClose={() => setCancelTarget(null)}
        onSubmit={() => cancelTarget && cancel.mutate({id: cancelTarget.id, text: reason.trim()})}
        submitLabel="Confirmar cancelamento"
        cancelLabel="Voltar"
        danger
        loading={cancel.isPending}
        submitDisabled={reason.trim().length === 0}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            O cancelamento só vale até {CIOT_CANCEL_WINDOW_HOURS} horas depois do início da viagem. Depois disso a ANTT
            recusa.
          </p>
          {window24 && (
            <p role={window24.expired ? 'alert' : undefined}
               className={`rounded-lg border px-3 py-2 text-xs ${window24.expired
                 ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-gray-200 bg-gray-50 text-gray-600'}`}>
              {window24.expired ? 'O prazo parece ter passado: ' : 'Prazo: '}
              {window24.deadline.toLocaleString('pt-BR', {dateStyle: 'short', timeStyle: 'short'})}
              {window24.expired && '. Você pode tentar, mas a ANTT deve recusar.'}
            </p>
          )}
          <JustificationField
            id="ciot-cancel-reason"
            label="Motivo"
            value={reason}
            onChange={setReason}
            minLength={1}
            maxLength={CIOT_CANCEL_REASON_MAX_LENGTH}
            placeholder="Descreva o motivo do cancelamento…"
          />
          {errors}
        </div>
      </Modal>

      <Modal
        open={closeTarget !== null}
        title={closeTarget ? `Encerrar o CIOT ${label(closeTarget)}` : ''}
        onClose={() => setCloseTarget(null)}
        onSubmit={submitClose}
        submitLabel="Confirmar encerramento"
        cancelLabel="Voltar"
        loading={close.isPending}
        submitDisabled={!closeReady}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            O encerramento informa à ANTT que a operação foi cumprida.
            {isFull && ' Na carga lotação, informe o peso da carga transportada.'}
            {isTac && ' No TAC-Agregado, informe a quantidade de viagens e o trecho.'}
          </p>
          <div className="flex flex-col gap-1">
            <label htmlFor="ciot-closed-at" className="text-xs font-medium text-gray-600">Data do encerramento (opcional)</label>
            <Input id="ciot-closed-at" type="date" className="h-11 w-full" value={closedAt}
                   onChange={(e) => setClosedAt(e.target.value)}/>
          </div>
          {isFull && (
            <div className="flex flex-col gap-1">
              <label htmlFor="ciot-close-weight" className="text-xs font-medium text-gray-600">Peso da carga (kg)</label>
              <NumericInput id="ciot-close-weight" decimal decimalPlaces={2} integerPlaces={7} value={weight}
                            className="h-11 w-full" onChange={setWeight}/>
            </div>
          )}
          {isTac && (
            <>
              <div className="flex flex-col gap-1">
                <label htmlFor="ciot-close-trips" className="text-xs font-medium text-gray-600">Quantidade de viagens</label>
                <NumericInput id="ciot-close-trips" integerPlaces={4} value={tripCount} className="h-11 w-full"
                              onChange={setTripCount}/>
              </div>
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div className="flex flex-col gap-1">
                  <label htmlFor="ciot-close-origin" className="text-xs font-medium text-gray-600">Município de origem</label>
                  <Combobox id="ciot-close-origin" value={origin} onValueChange={setOrigin} options={CITY_OPTIONS}
                            placeholder="Selecione o município" searchPlaceholder="Buscar município…"/>
                </div>
                <div className="flex flex-col gap-1">
                  <label htmlFor="ciot-close-destination" className="text-xs font-medium text-gray-600">Município de destino</label>
                  <Combobox id="ciot-close-destination" value={destination} onValueChange={setDestination}
                            options={CITY_OPTIONS} placeholder="Selecione o município" searchPlaceholder="Buscar município…"/>
                </div>
              </div>
            </>
          )}
          {errors}
        </div>
      </Modal>
    </>
  );

  return {openCancel, openClose, modals};
}
