import {ApiError} from '@/lib/api/client';
import {
  CIOT_CANCEL_WINDOW_HOURS,
  CIOT_SK_PREFIX,
  OPERATION_LOTACAO,
  PROBLEM_CIOT_FLOOR_UNVERIFIED,
} from '@/lib/constants/ciot';
import type {CiotAnttResult, CiotOut} from '@/lib/types/api';

const ANTT_CODE_INSERTED = 110;
const ANTT_CODE_QUERIED = 111;
const HOUR_MS = 3_600_000;
/** Horário de Brasília (UTC-3 fixo desde 2019): a data de início da viagem é um dia civil de lá. */
const BRASILIA_OFFSET = '-03:00';

/** A mensagem `dfe_result` traz a chave da linha (`CIOT_<id>`); as rotas usam o `id`. */
export const ciotIdFromSk = (sk: string): string => sk.startsWith(CIOT_SK_PREFIX) ? sk.slice(CIOT_SK_PREFIX.length) : sk;

const IN_FORCE = new Set(['declared', 'rectified']);

/** Só CIOT em vigor e sem operação em andamento aceita uma nova operação (a api devolve 409 `ciot_invalid_state` no resto). */
export function ciotIdle(c: Pick<CiotOut, 'status' | 'pending_operation'>): boolean {
  return IN_FORCE.has(c.status) && !c.pending_operation;
}

export const canCancelCiot = ciotIdle;
export const canCloseCiot = ciotIdle;
/** A retificação é proibida em carga lotação (regra B121). */
export const canRectifyCiot = (c: Pick<CiotOut, 'status' | 'pending_operation' | 'operation_type'>): boolean =>
  ciotIdle(c) && c.operation_type !== OPERATION_LOTACAO;

export interface CancelWindow {
  deadline: Date
  /** `true` quando o prazo de 24 h já passou; a ANTT é quem decide, então é só aviso. */
  expired: boolean
}

/** Prazo de cancelamento a partir da data de início da viagem; `null` quando a data não existe. */
export function cancelWindow(tripStart: string | undefined, now: Date = new Date()): CancelWindow | null {
  if (!tripStart) return null;
  const start = new Date(`${tripStart.slice(0, 10)}T00:00:00${BRASILIA_OFFSET}`);
  if (Number.isNaN(start.getTime())) return null;
  const deadline = new Date(start.getTime() + CIOT_CANCEL_WINDOW_HOURS * HOUR_MS);
  return {deadline, expired: now.getTime() > deadline.getTime()};
}

export interface AnttSummary {
  /** Códigos 110 e 111 são sucesso na ANTT. */
  ok: boolean
  message: string
  /** RNTRC lido da resposta, só dígitos; `null` quando a resposta não traz um. */
  rntrc: string | null
  /** Campos escalares da resposta, como vieram: o formato de `body` não é fixado pelo contrato. */
  fields: { label: string; value: string }[]
}

/** Leitura tolerante da resposta de situação/frota; nunca afirma elegibilidade que a ANTT não disse. */
export function summarizeAntt(result: CiotAnttResult | undefined): AnttSummary {
  const body = result?.body ?? {};
  const fields = Object.entries(body)
    .filter(([, v]) => ['string', 'number', 'boolean'].includes(typeof v) && String(v) !== '')
    .map(([label, v]) => ({label, value: typeof v === 'boolean' ? (v ? 'Sim' : 'Não') : String(v)}));
  const rntrcField = fields.find((f) => /rntrc/i.test(f.label) && /^\d{8,9}$/.test(f.value.replace(/\D/g, '')));
  return {
    ok: result?.codigo === ANTT_CODE_INSERTED || result?.codigo === ANTT_CODE_QUERIED,
    message: result?.mensagem ?? '',
    rntrc: rntrcField ? rntrcField.value.replace(/\D/g, '') : null,
    fields,
  };
}

/** A tabela do piso está marcada como não verificada (ou a api está fora): a dica some, a declaração segue. */
export function floorUnavailable(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false;
  const code = (err.raw as { code?: string } | undefined)?.code;
  return err.status === 503 || code === PROBLEM_CIOT_FLOOR_UNVERIFIED;
}

/** Uma linha por regra recusada (`errors[]` do 422 `ciot_validation`), com o código da regra e o da ANTT quando vêm. */
export function problemLines(err: unknown): string[] {
  if (!(err instanceof ApiError)) return [];
  const errors = (err.raw as { errors?: { message?: string; params?: { rule?: string; antt_code?: number } }[] } | undefined)?.errors;
  return (errors ?? []).filter((e) => e.message).map((e) => {
    const refs = [e.params?.rule, e.params?.antt_code ? `ANTT ${e.params.antt_code}` : ''].filter(Boolean).join(', ');
    return refs ? `${e.message} (${refs})` : String(e.message);
  });
}

export interface CiotResultMessage {
  status?: string
  operation?: string
  ciot?: string
  antt_message?: string
  /** Preenchido só em recusa de negócio da ANTT. */
  internal_code?: string
}

/** Toast do resultado assíncrono de uma operação do CIOT. O CIOT é masculino: declarado, retificado, cancelado, encerrado. */
export function resolveCiotToast(msg: CiotResultMessage): { variant: 'success' | 'error' | 'info'; message: string } {
  const reason = msg.antt_message ? `; ${msg.antt_message}` : '';
  // Operação de ciclo de vida recusada deixa o status como estava e só traz o código interno.
  if (msg.internal_code && msg.status !== 'rejected') {
    return {variant: 'error', message: `ANTT recusou a operação do CIOT${reason}`};
  }
  switch (msg.status) {
    case 'declared':
      return {variant: 'success', message: msg.ciot ? `CIOT ${msg.ciot} declarado na ANTT` : 'CIOT declarado na ANTT'};
    case 'rectified':
      return {variant: 'success', message: 'CIOT retificado na ANTT'};
    case 'cancelled':
      return {variant: 'success', message: 'CIOT cancelado na ANTT'};
    case 'closed':
      return {variant: 'success', message: 'CIOT encerrado na ANTT'};
    case 'failed':
    case 'operation_failed':
      return {variant: 'error', message: 'Não foi possível concluir a operação do CIOT; tente novamente'};
    case 'rejected':
      return {variant: 'error', message: `ANTT recusou a operação do CIOT${reason}`};
    default:
      return {variant: 'info', message: 'CIOT em processamento'};
  }
}
