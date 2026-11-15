// Resolves a `dfe_result` realtime message into the toast that should be shown.
//
// A `dfe_result` is either a DOCUMENT result (carries the document status) or a
// SEFAZ EVENT result (cancellation, encerramento — carries the event outcome).
// For events the EVENT status must drive the message: the worker may revert the
// document to "authorized" after a failed/rejected event, and reporting that
// document status would mask the real event failure.

import {DOC_GENDER, dfeStatusLabel} from '@/lib/data/dfe_status';

// result_kind values (mirror worker constants).
export const RESULT_KIND_EVENT = 'event';

// SEFAZ event status values (mirror worker EventStatus* constants).
export const EVENT_STATUS_SUCCESS = 'success';
export const EVENT_STATUS_REJECTED = 'rejected';
// Falha de transporte — o worker reprocessa; não é desfecho do evento.
const STATUS_RETRYABLE_FAILED = 'retryable_failed';

// SEFAZ event type codes. 110112 is "encerramento" for MDF-e but
// "cancelamento por substituição" for NF-e/NFC-e — disambiguated by table_name.
const EVENT_TYPE_ENCERRAMENTO = '110112';
const TABLE_MDFE = 'mdfes';

// Inutilização de numeração (api: nfes.EventTypeInutilizacao). Travels through
// the same event-result channel as cancellation, but is not an event *on* a
// document — reporting it as a cancellation told users the wrong thing happened.
export const EVENT_TYPE_INUTILIZACAO = 'INUT';

const TABLE_LABEL: Record<string, string> = {
  nfes: 'NF-e',
  nfces: 'NFC-e',
  nfses: 'NFS-e',
  ctes: 'CT-e',
  mdfes: 'MDF-e',
};

// Eventos que cancelam o documento (110112 só cancela NF-e/NFC-e; no MDF-e encerra) e carta de correção.
const EVENT_TYPE_CANCEL = '110111';
const EVENT_TYPE_NFSE_CANCEL = '101101';
const EVENT_TYPE_CCE = '110110';

/** Rota de detalhe e parâmetro que identifica o documento, por tabela. */
const DETAIL_ROUTE: Record<string, { path: string; param: string }> = {
  nfes: {path: '/nfe/detail', param: 'key'},
  nfces: {path: '/nfce/detail', param: 'key'},
  nfses: {path: '/nfse/detail', param: 'id'},
  ctes: {path: '/cte/detail', param: 'key'},
  mdfes: {path: '/mdfe/detail', param: 'key'},
};

export const LABEL_VER_PDF = 'Ver PDF';
export const LABEL_VER_DETALHES = 'Ver detalhes';

export interface DfeResultMessage {
  result_kind?: string
  table_name?: string
  status?: string
  sefaz_motive?: string
  event_type?: string
  event_sk?: string
  access_key?: string
}

export type ToastVariant = 'success' | 'error' | 'info'

/** Botão do toast: `pdf` baixa o auxiliar do documento; `link` abre `href` na própria aplicação. */
export type ToastAction =
  | { kind: 'pdf'; label: string; tableName: string; id: string }
  | { kind: 'link'; label: string; href: string }

export interface ResolvedToast {
  variant: ToastVariant
  message: string
  action?: ToastAction
}

function docLabel(tableName?: string): string {
  return TABLE_LABEL[tableName ?? ''] ?? 'Documento';
}

/** "NF-e 123", ou só "NF-e" quando o número não é conhecido. */
function docWithNumber(tableName: string | undefined, number: number | null): string {
  return number == null ? docLabel(tableName) : `${docLabel(tableName)} ${number}`;
}

function detailsAction(msg: DfeResultMessage): ToastAction | undefined {
  const route = DETAIL_ROUTE[msg.table_name ?? ''];
  if (!route || !msg.access_key) return undefined;
  const qs = new URLSearchParams({[route.param]: msg.access_key});
  if (msg.event_sk) qs.set('event', msg.event_sk);
  return {kind: 'link', label: LABEL_VER_DETALHES, href: `${route.path}?${qs.toString()}`};
}

function motiveSuffix(motive?: string): string {
  return motive ? `; ${motive}` : '';
}

function resolveDocumentToast(msg: DfeResultMessage, number: number | null): ResolvedToast {
  const doc = docLabel(msg.table_name);
  const docNum = docWithNumber(msg.table_name, number);
  const motive = motiveSuffix(msg.sefaz_motive);
  // Concorda com o substantivo: nota autorizada, manifesto autorizado.
  const a = DOC_GENDER[msg.table_name ?? ''] === 'm' ? 'o' : 'a';

  switch (msg.status) {
    case 'authorized':
      return {
        variant: 'success',
        message: `${docNum} autorizad${a}`,
        action: msg.access_key && msg.table_name
          ? {kind: 'pdf', label: LABEL_VER_PDF, tableName: msg.table_name, id: msg.access_key}
          : undefined,
      };
    case 'cancelled':
      return {variant: 'success', message: `${docNum} cancelad${a}`, action: detailsAction(msg)};
    case 'closed':
      return {variant: 'success', message: `${docNum} encerrad${a}`};
    case 'rejected':
      return {variant: 'error', message: `${doc} rejeitad${a} pela SEFAZ${motive}`};
    case 'failed':
    case 'error':
      return {variant: 'error', message: `Falha ao processar ${doc}${motive}`};
    // Não terminal: o worker reprocessa sozinho na próxima entrega SQS, então o
    // usuário é informado sem ser mandado agir.
    case 'retryable_failed':
      return {variant: 'info', message: `${doc} não pôde ser enviad${a} agora; tentando novamente${motive}`};
    default:
      return {variant: 'info', message: `${doc} atualizad${a}; status: ${dfeStatusLabel(msg.status ?? '', a === 'o' ? 'm' : 'f')}`};
  }
}

function resolveEventToast(msg: DfeResultMessage, number: number | null): ResolvedToast {
  const doc = docLabel(msg.table_name);
  const docNum = docWithNumber(msg.table_name, number);
  const motive = motiveSuffix(msg.sefaz_motive);
  const a = DOC_GENDER[msg.table_name ?? ''] === 'm' ? 'o' : 'a';

  let wording: { retry: string; success: string; fail: string; reject: string };
  if (msg.event_type === EVENT_TYPE_INUTILIZACAO) {
    wording = {
      retry: 'Inutilização de numeração não concluída',
      success: `Faixa de numeração de ${doc} inutilizada com sucesso`,
      fail: `Falha ao inutilizar a numeração de ${doc}`,
      reject: `Inutilização de numeração de ${doc} rejeitada pela SEFAZ`,
    };
  } else if (msg.event_type === EVENT_TYPE_ENCERRAMENTO && msg.table_name === TABLE_MDFE) {
    wording = {
      retry: `Encerramento de ${doc} não concluído`,
      success: `${docNum} encerrad${a}`,
      fail: `Falha ao encerrar ${doc}`,
      reject: `Encerramento de ${doc} rejeitado pela SEFAZ`,
    };
  } else if (msg.event_type === EVENT_TYPE_CCE) {
    wording = {
      retry: `Correção de ${doc} não concluída`,
      success: `Correção da ${docNum} enviada`,
      fail: `Falha ao enviar a correção de ${doc}`,
      reject: `Correção de ${doc} rejeitada pela SEFAZ`,
    };
  } else if (msg.event_type === EVENT_TYPE_CANCEL || msg.event_type === EVENT_TYPE_NFSE_CANCEL
    || msg.event_type === EVENT_TYPE_ENCERRAMENTO) {
    wording = {
      retry: `Cancelamento de ${doc} não concluído`,
      success: `${docNum} cancelad${a}`,
      fail: `Falha ao cancelar ${doc}`,
      reject: `Cancelamento de ${doc} rejeitado pela SEFAZ`,
    };
  } else {
    // Manifestação do destinatário, prorrogação, ator interessado e demais eventos.
    wording = {
      retry: `Evento de ${doc} não concluído`,
      success: `Evento registrado em ${docNum}`,
      fail: `Falha ao registrar o evento de ${doc}`,
      reject: `Evento de ${doc} rejeitado pela SEFAZ`,
    };
  }

  switch (msg.status) {
    case EVENT_STATUS_SUCCESS:
      return {variant: 'success', message: wording.success, action: detailsAction(msg)};
    case EVENT_STATUS_REJECTED:
      return {variant: 'error', message: `${wording.reject}${motive}`};
    case STATUS_RETRYABLE_FAILED:
      return {
        variant: 'info',
        message: `${wording.retry}; tentando novamente${motive}`,
      };
    default:
      // 'error' or any unexpected status — treat as a processing failure.
      return {variant: 'error', message: `${wording.fail}${motive}`};
  }
}

/** `number` é o número do documento (da chave de acesso); sem ele a mensagem sai sem número. */
export function resolveDfeResultToast(msg: DfeResultMessage, number: number | null = null): ResolvedToast {
  return msg.result_kind === RESULT_KIND_EVENT
    ? resolveEventToast(msg, number)
    : resolveDocumentToast(msg, number);
}
