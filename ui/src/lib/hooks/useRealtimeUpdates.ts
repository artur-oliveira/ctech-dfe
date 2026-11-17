'use client';

import {useCallback} from 'react';
import {useRouter} from 'next/navigation';
import {useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {useAuth} from './useAuth';
import {useWebSocket, type WSStatus} from '@aoctech/ws-client';
import {queryKeys} from '@/lib/api/query-keys';
import {apiClient, getAccessToken, subscribeAccessToken} from '@/lib/api/client';
import {EVENT_TYPE_INUTILIZACAO, resolveDfeResultToast, type ResolvedToast} from '@/lib/utils/dfe-result-toast';
import {docNumberFromKey, triggerRemoteDownload} from '@/lib/utils/dfe';
import {printProblemMessage} from '@/lib/constants/document-formats';
import {CIOT_TABLE_NAME} from '@/lib/constants/ciot';
import {ciotIdFromSk, resolveCiotToast} from '@/lib/utils/ciot';

// `next dev` rewrites do not proxy the WebSocket upgrade, so local development
// points NEXT_PUBLIC_WS_URL straight at the API. Deployed environments now set it
// explicitly too: nothing is proxied at the edge, and the CSP connect-src is
// built from these literals, so the wss:// origin has to appear as one.
const WS_BASE_URL = process.env.NEXT_PUBLIC_WS_URL || process.env.NEXT_PUBLIC_API_URL || '';

function buildWsUrl(orgPk: string): string {
  const origin = WS_BASE_URL || window.location.origin;
  const base = origin.replace(/^http/, 'ws');
  return `${base}/v1.0/ws?org_pk=${encodeURIComponent(orgPk)}`;
}

interface RealtimeMessage {
  type: string
  // result_kind distinguishes a document result from a SEFAZ event result
  // (cancellation, encerramento). Sent by the worker on every dfe_result.
  result_kind?: string
  access_key?: string
  doc_pk?: string
  table_name?: string
  status?: string
  sefaz_motive?: string
  event_type?: string
  event_sk?: string
  emit_name?: string
  total?: string
  nsu?: number
  // reason carries the rejection detail on import_xml_failed.
  reason?: string
  // CIOT: resultado da operação na ANTT.
  operation?: string
  ciot?: string
  antt_message?: string
  internal_code?: string
}

// Query-key namespaces that own list/detail/events caches, keyed by the
// document's DynamoDB table name.
const DOC_QUERY_KEYS = {
  nfes: queryKeys.nfes,
  nfces: queryKeys.nfces,
  mdfes: queryKeys.mdfes,
  ctes: queryKeys.ctes,
  nfses: queryKeys.nfses,
  nfcoms: queryKeys.nfcoms,
  bpes: queryKeys.bpes,
} as const;

// Inutilização queries are keyed by doc_type ('nfe'/'nfce'), not by the
// DynamoDB table name the result message carries.
const INUT_DOC_TYPE: Record<string, string> = {
  nfes: 'nfe',
  nfces: 'nfce',
};

// PDF auxiliar de cada documento (DANFE, DANFC-e, DACTE, DAMDFE, DANFSe), pela tabela da mensagem.
const AUXILIARY_PDF: Record<string, (id: string) => Promise<{ url: string }>> = {
  nfes: (id) => apiClient.downloadNfeDanfe(id),
  nfces: (id) => apiClient.downloadNfceDanfe(id),
  nfses: (id) => apiClient.downloadDanfse(id),
  ctes: (id) => apiClient.downloadCteDacte(id),
  mdfes: (id) => apiClient.downloadMdfeDamdfe(id),
  nfcoms: (id) => apiClient.downloadNfcomDanfecom(id),
  bpes: (id) => apiClient.downloadBpeDabpe(id),
};

async function openAuxiliaryPdf(tableName: string, id: string): Promise<void> {
  try {
    triggerRemoteDownload((await AUXILIARY_PDF[tableName](id)).url);
  } catch (e) {
    const message = printProblemMessage(e);
    // O BP-e TM não tem DABPE (a mensagem de realtime não traz a variante): não é falha para o usuário.
    if (tableName === 'bpes' && message.includes('não tem DABPE')) return;
    toast.error(message);
  }
}

const NUMBERED_STATUSES = new Set(['authorized', 'cancelled', 'closed', 'success']);

export function useRealtimeUpdates(): { wsStatus: WSStatus } {
  const {selectedOrg} = useAuth();
  const qc = useQueryClient();
  const router = useRouter();

  // O número do documento sai da chave de acesso; na NFS-e a mensagem traz o id da DPS,
  // então a chave vem do detalhe (já invalidado pela mensagem).
  const resolveNumber = useCallback(async (msg: RealtimeMessage): Promise<number | null> => {
    if (msg.table_name !== 'nfses') return docNumberFromKey(msg.access_key);
    // Só as mensagens que citam o número fazem a consulta extra.
    if (!msg.status || !NUMBERED_STATUSES.has(msg.status)) return null;
    try {
      const id = msg.access_key ?? '';
      const doc = await qc.fetchQuery({queryKey: queryKeys.nfses.detail(id), queryFn: () => apiClient.getNfse(id)});
      return docNumberFromKey(doc.access_key);
    } catch {
      return null;
    }
  }, [qc]);

  const showToast = useCallback(({variant, message, action}: ResolvedToast) => {
    toast[variant](message, action ? {
      action: {
        label: action.label,
        onClick: () => {
          if (action.kind === 'link') router.push(action.href);
          else void openAuxiliaryPdf(action.tableName, action.id);
        },
      },
    } : undefined);
  }, [router]);

  const token = getAccessToken();

  const wsUrl = token && selectedOrg?.pk
    ? buildWsUrl(selectedOrg.pk)
    : null;

  const handleMessage = useCallback((data: unknown) => {
    const msg = data as RealtimeMessage;
    if (!msg?.type || msg.type === 'ping' || msg.type === 'connected') return;

    // CIOT: a mensagem traz a chave da linha (`CIOT_<id>`); a lista e o detalhe saem do cache, sem eventos de SEFAZ.
    if (msg.type === 'dfe_result' && msg.access_key && msg.table_name === CIOT_TABLE_NAME) {
      void qc.invalidateQueries({queryKey: queryKeys.ciots.detail(ciotIdFromSk(msg.access_key))});
      void qc.invalidateQueries({queryKey: queryKeys.ciots.lists(selectedOrg?.pk)});
      const {variant, message} = resolveCiotToast(msg);
      toast[variant](message);
      return;
    }

    if (msg.type === 'dfe_result' && msg.access_key) {
      // An inutilização is not an event on a document — its access_key is
      // synthetic ("INUT#{env}#{org_pk}") and there is no detail/list/event
      // cache to refresh. Its own list and the detected-gaps list are what
      // must go stale, or the closed gap keeps showing after the toast.
      if (msg.event_type === EVENT_TYPE_INUTILIZACAO) {
        const inutDocType = INUT_DOC_TYPE[msg.table_name ?? ''];
        if (inutDocType) {
          void qc.invalidateQueries({queryKey: queryKeys.inutilizations.list(inutDocType, selectedOrg?.pk)});
          void qc.invalidateQueries({queryKey: queryKeys.inutilizations.gaps(inutDocType, selectedOrg?.pk)});
        }
      } else {
        // Route invalidation by document type so each document's updates reach
        // its own queries (detail, list, and event history).
        const doc = DOC_QUERY_KEYS[msg.table_name as keyof typeof DOC_QUERY_KEYS];
        if (doc) {
          void qc.invalidateQueries({queryKey: doc.detail(msg.access_key)});
          void qc.invalidateQueries({queryKey: doc.lists(selectedOrg?.pk)});
          void qc.invalidateQueries({queryKey: doc.events(msg.access_key)});
        }
      }
      // Resolve the toast from the result — event results report the event
      // outcome, not the (possibly reverted) document status.
      void resolveNumber(msg).then((number) => showToast(resolveDfeResultToast(msg, number)));
    }

    if (msg.type === 'new_distribution_nfe') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('nfe', selectedOrg?.pk)});
      const label = msg.emit_name ? ` de ${msg.emit_name}` : '';
      const value = msg.total ? `: R$ ${parseFloat(msg.total).toLocaleString('pt-BR', {minimumFractionDigits: 2})}` : '';
      toast.info(`Nova NF-e recebida${label}${value}`);
    }

    if (msg.type === 'new_distribution_cte') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('cte', selectedOrg?.pk)});
      const label = msg.emit_name ? ` de ${msg.emit_name}` : '';
      toast.info(`Novo CT-e recebido${label}`);
    }

    if (msg.type === 'new_distribution_mdfe') {
      void qc.invalidateQueries({queryKey: queryKeys.distributions.history('mdfe', selectedOrg?.pk)});
      toast.info('Novo MDF-e recebido');
    }

    if (msg.type === 'import_xml_failed') {
      toast.error(msg.reason ? `Falha ao importar XML: ${msg.reason}` : 'Falha ao importar XML.');
    }
  }, [qc, selectedOrg?.pk, resolveNumber, showToast]);

  const {status: wsStatus} = useWebSocket({
    url: wsUrl,
    onMessage: handleMessage,
    enabled: !!wsUrl,
    authToken: token ?? undefined,
    subscribeToken: subscribeAccessToken,
  });

  return {wsStatus};
}