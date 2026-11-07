'use client';

import {Modal} from '@aoctech/ui';
import {type ReactNode, useState} from 'react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {JustificationField} from '@/components/ui/justification-field';
import {Combobox} from '@/components/ui/combobox';
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status';
import {CITIES, CITY_OPTIONS} from '@/lib/data/cities';
import {DocumentPicker} from '@/components/mdfe/DocumentPicker';
import {EmitError} from '@/components/ui/emit-error';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {validateAccessKey} from '@/lib/utils/access-key';
import type {NfeListOut} from '@/lib/types/api';

const CANCEL_JUSTIFICATION_MIN_LENGTH = 15;
const CANCEL_JUSTIFICATION_MAX_LENGTH = 255;

/** Minimal shape needed to dispatch MDF-e cancel/close actions. */
export interface MdfeActionTarget {
  sk: string
  number: number | string
  /** Default UF for the close (encerramento) modal — usually the trip end UF. */
  uf_end?: string
  /** Inclusões de DF-e (110115) já registradas; a próxima usa o seguinte nSeqEvento. */
  includes?: number
}

/** Município escolhido pelo código IBGE, com o nome que o evento leva. */
const cityOf = (code: string) => CITIES.find((c) => c.code === code);

/**
 * useMdfeActions centralizes the MDF-e cancel (110111), close/encerramento
 * (110112) and inclusão de DF-e (110115, the follow-up of a carregamento posterior) flows — mutations, modal state, and the rendered modals — so the list
 * and detail pages share one implementation. Render the returned `modals` once
 * and wire trigger buttons to `openCancel`/`openClose`/`openInclude`.
 */
export function useMdfeActions(orgPk?: string) {
  const qc = useQueryClient();
  const [cancelTarget, setCancelTarget] = useState<MdfeActionTarget | null>(null);
  const [justification, setJustification] = useState('');
  const [closeTarget, setCloseTarget] = useState<MdfeActionTarget | null>(null);
  const [closeUf, setCloseUf] = useState('');
  const [closeMun, setCloseMun] = useState('');
  const [includeTarget, setIncludeTarget] = useState<MdfeActionTarget | null>(null);
  const [includeDocs, setIncludeDocs] = useState<NfeListOut[]>([]);
  const [includeLoadMun, setIncludeLoadMun] = useState('');
  const [includeUnloadMun, setIncludeUnloadMun] = useState('');
  const [includeError, setIncludeError] = useState<EmitFailure | null>(null);

  // Optimistically show the transitional status (the GSI is eventually
  // consistent); the WebSocket delivers the final status when the worker finishes.
  const patchStatus = (accessKey: string, status: 'cancel_pending' | 'close_pending') => {
    setDocStatusOptimistic(qc, queryKeys.mdfes.lists(orgPk), accessKey, status);
    void qc.invalidateQueries({queryKey: queryKeys.mdfes.detail(accessKey)});
  };

  const cancelMutation = useMutation({
    mutationFn: ({accessKey, justification}: { accessKey: string; justification: string }) =>
      apiClient.cancelMdfe(accessKey, justification),
    onSuccess: (_data, {accessKey}) => {
      setCancelTarget(null);
      setJustification('');
      patchStatus(accessKey, 'cancel_pending');
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao cancelar MDF-e.'),
  });

  const closeMutation = useMutation({
    mutationFn: ({accessKey, cMun, uf}: { accessKey: string; cMun: string; uf: string }) =>
      apiClient.closeMdfe(accessKey, cMun, uf || undefined),
    onSuccess: (_data, {accessKey}) => {
      setCloseTarget(null);
      setCloseMun('');
      setCloseUf('');
      patchStatus(accessKey, 'close_pending');
      toast.success('Encerramento enviado à SEFAZ.');
    },
    onError: (e) => toast.error(e instanceof Error ? e.message : 'Erro ao encerrar MDF-e.'),
  });

  const resetInclude = () => {
    setIncludeTarget(null);
    setIncludeDocs([]);
    setIncludeLoadMun('');
    setIncludeUnloadMun('');
    setIncludeError(null);
  };

  const includeMutation = useMutation({
    mutationFn: ({target, docs}: { target: MdfeActionTarget; docs: NfeListOut[] }) => {
      const load = cityOf(includeLoadMun)!;
      const unload = cityOf(includeUnloadMun)!;
      return apiClient.includeMdfeDFe(target.sk, load.code, load.description, docs.map((d) => ({
        unloading_ibge_code: unload.code, unloading_city: unload.description, nfe_key: d.sk,
      })), (target.includes ?? 0) + 1);
    },
    onSuccess: (_data, {target}) => {
      resetInclude();
      void qc.invalidateQueries({queryKey: queryKeys.mdfes.detail(target.sk)});
      void qc.invalidateQueries({queryKey: queryKeys.mdfes.events(target.sk)});
      toast.success('Inclusão de NF-e enviada à SEFAZ.');
    },
    onError: (e) => setIncludeError(emitFailure(e, 'Erro ao incluir NF-e.', undefined, 'mdfe')),
  });

  // A chave vem da nota escolhida, mas passa pela mesma checagem da api antes de sair (modelo 55, DV).
  const invalidInclude = includeDocs.find((d) => !validateAccessKey(d.sk).valid);
  const includeReady = includeDocs.length > 0 && !!includeLoadMun && !!includeUnloadMun && !invalidInclude;

  const handleConfirmInclude = () => {
    if (!includeTarget || !includeReady) return;
    includeMutation.mutate({target: includeTarget, docs: includeDocs});
  };

  const toggleIncludeDoc = (n: NfeListOut) =>
    setIncludeDocs((prev) => prev.some((d) => d.sk === n.sk) ? prev.filter((d) => d.sk !== n.sk) : [...prev, n]);

  const openInclude = (m: MdfeActionTarget) => {
    resetInclude();
    setIncludeTarget(m);
  };

  const openCancel = (m: MdfeActionTarget) => {
    setJustification('');
    setCancelTarget(m);
  };
  const openClose = (m: MdfeActionTarget) => {
    setCloseMun('');
    setCloseUf(m.uf_end ?? '');
    setCloseTarget(m);
  };

  const handleConfirmCancel = () => {
    if (!cancelTarget || justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH) return;
    cancelMutation.mutate({accessKey: cancelTarget.sk, justification: justification.trim()});
  };
  const handleConfirmClose = () => {
    if (!closeTarget || closeMun.length !== 7) return;
    closeMutation.mutate({accessKey: closeTarget.sk, cMun: closeMun, uf: closeUf});
  };

  const modals: ReactNode = (
    <>
      <Modal
        open={cancelTarget !== null}
        title={cancelTarget ? `Cancelar MDF-e nº ${cancelTarget.number}` : ''}
        onClose={() => {
          setCancelTarget(null);
          setJustification('');
        }}
        onSubmit={handleConfirmCancel}
        submitLabel="Confirmar cancelamento"
        cancelLabel="Voltar"
        danger
        loading={cancelMutation.isPending}
        submitDisabled={justification.trim().length < CANCEL_JUSTIFICATION_MIN_LENGTH}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. O MDF-e será cancelado junto à
            SEFAZ.
            Só é possível cancelar antes do início do transporte.
          </p>
          <JustificationField
            id="mdfe-cancel-justification"
            value={justification}
            onChange={setJustification}
            minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
            maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
            placeholder="Descreva o motivo do cancelamento (mínimo 15 caracteres)…"
          />
        </div>
      </Modal>

      <Modal
        open={closeTarget !== null}
        title={closeTarget ? `Encerrar MDF-e nº ${closeTarget.number}` : ''}
        onClose={() => {
          setCloseTarget(null);
          setCloseMun('');
          setCloseUf('');
        }}
        onSubmit={handleConfirmClose}
        submitLabel="Confirmar encerramento"
        cancelLabel="Voltar"
        loading={closeMutation.isPending}
        submitDisabled={closeMun.length !== 7 || !closeUf}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            O encerramento informa à SEFAZ que o transporte terminou. Informe o município onde a viagem foi concluída.
          </p>
          <div className="flex flex-col gap-1">
            <label className="text-xs font-medium text-gray-600">Município de encerramento</label>
            <Combobox
              value={closeMun}
              onValueChange={(code) => {
                setCloseMun(code);
                const city = CITIES.find((c) => c.code === code);
                setCloseUf(city?.uf ?? '');
              }}
              options={CITY_OPTIONS}
              placeholder="Selecione o município"
              searchPlaceholder="Buscar município…"
            />
          </div>
        </div>
      </Modal>

      <Modal
        open={includeTarget !== null}
        title={includeTarget ? `Incluir NF-e no MDF-e nº ${includeTarget.number}` : ''}
        onClose={resetInclude}
        onSubmit={handleConfirmInclude}
        submitLabel="Incluir NF-e"
        cancelLabel="Voltar"
        loading={includeMutation.isPending}
        submitDisabled={!includeReady}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            As notas entram na carga por evento de inclusão de DF-e. No carregamento posterior, inclua em até 168 horas
            da autorização; depois disso a SEFAZ recusa novos manifestos do emitente.
          </p>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div className="flex flex-col gap-1">
              <label htmlFor="mdfe-include-load" className="text-xs font-medium text-gray-600">Município de carregamento</label>
              <Combobox id="mdfe-include-load" value={includeLoadMun} onValueChange={setIncludeLoadMun}
                        options={CITY_OPTIONS} placeholder="Selecione o município" searchPlaceholder="Buscar município…"/>
            </div>
            <div className="flex flex-col gap-1">
              <label htmlFor="mdfe-include-unload" className="text-xs font-medium text-gray-600">Município de descarregamento</label>
              <Combobox id="mdfe-include-unload" value={includeUnloadMun} onValueChange={setIncludeUnloadMun}
                        options={CITY_OPTIONS} placeholder="Selecione o município" searchPlaceholder="Buscar município…"/>
            </div>
          </div>
          <DocumentPicker selected={includeDocs} onToggle={toggleIncludeDoc}/>
          {invalidInclude && (
            <p role="alert" className="text-xs text-red-600">
              A chave da NF-e nº {invalidInclude.number} não confere; tire ela da seleção.
            </p>
          )}
          <EmitError failure={includeError}/>
        </div>
      </Modal>
    </>
  );

  return {openCancel, openClose, openInclude, modals};
}
