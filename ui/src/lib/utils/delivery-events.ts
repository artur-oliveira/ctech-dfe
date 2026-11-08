import {fixedDecimal} from '@/lib/data/nfe_reform_events';
import {datetimeLocalToOffset} from '@/lib/utils/datetime';
import {cleanDocument, validateCNPJ, validateCPF} from '@/lib/utils/validators';

/**
 * Comprovante de entrega, insucesso (CT-e) e ator interessado: rascunho, pendências e corpo de cada rota.
 * Limites espelham api/internal/services/{nfes,ctes}/delivery_events.go.
 */

export const DELIVERY_DOC_MIN = 2;
export const DELIVERY_DOC_MAX = 20;
export const DELIVERY_NAME_MIN = 2;
export const DELIVERY_NAME_MAX = 60;
export const DELIVERY_HASH_LENGTH = 28;
export const DELIVERY_PROTOCOL_LENGTH = 15;
export const FAILURE_JUSTIFICATION_MIN = 25;
export const FAILURE_JUSTIFICATION_MAX = 250;
export const FAILURE_REASON_OTHER = '4';
export const FAILURE_ATTEMPT_MAX = 999;

export const FAILURE_REASONS = [
  {value: '1', label: 'Recebedor ausente'},
  {value: '2', label: 'Endereço não localizado'},
  {value: '3', label: 'Recusa do recebedor'},
  {value: '4', label: 'Outro motivo'},
];

const reHash = /^[A-Za-z0-9+/]{27}=$/;

export interface ProofDraft {
  when: string        // datetime-local
  hashAt: string      // datetime-local
  hash: string
  docNumber: string
  name: string
  latitude: string
  longitude: string
  nfeKeys: string[]
  sequence: string
}
export const emptyProof = (): ProofDraft => ({
  when: '', hashAt: '', hash: '', docNumber: '', name: '', latitude: '', longitude: '', nfeKeys: [], sequence: '',
});

export type ProofKind = 'nfe-receipt' | 'cte-receipt' | 'cte-failure';

/** NF-e: hash_at é opcional; CT-e: obrigatório. */
export function proofIssues(kind: ProofKind, d: ProofDraft): string[] {
  const out: string[] = [];
  if (!datetimeLocalToOffset(d.when)) out.push('Informe a data e hora da ocorrência.');
  if (kind !== 'nfe-receipt' && !datetimeLocalToOffset(d.hashAt)) out.push('Informe a data e hora do hash.');
  if (!reHash.test(d.hash)) out.push(`Hash SHA-1 em Base64 com ${DELIVERY_HASH_LENGTH} caracteres.`);
  if (kind !== 'cte-failure') {
    const doc = d.docNumber.trim().length;
    if (doc < DELIVERY_DOC_MIN || doc > DELIVERY_DOC_MAX) out.push(`Documento do recebedor de ${DELIVERY_DOC_MIN} a ${DELIVERY_DOC_MAX} caracteres.`);
    const name = d.name.trim().length;
    if (name < DELIVERY_NAME_MIN || name > DELIVERY_NAME_MAX) out.push(`Nome do recebedor de ${DELIVERY_NAME_MIN} a ${DELIVERY_NAME_MAX} caracteres.`);
  }
  if ((d.latitude === '') !== (d.longitude === '')) out.push('Informe latitude e longitude juntas.');
  return out;
}

const gps = (v: string) => fixedDecimal(v.replace(/^-/, ''), 6) ? (v.startsWith('-') ? '-' : '') + fixedDecimal(v.replace(/^-/, ''), 6) : '';

const seq = (d: ProofDraft) => (d.sequence ? {sequence_number: Number(d.sequence)} : {});
const geo = (d: ProofDraft) => (d.latitude ? {latitude: gps(d.latitude), longitude: gps(d.longitude)} : {});

export function nfeReceiptBody(d: ProofDraft) {
  return {
    delivered_at: datetimeLocalToOffset(d.when), doc_number: d.docNumber.trim(), name: d.name.trim(),
    receipt_hash: d.hash, ...(d.hashAt ? {hash_at: datetimeLocalToOffset(d.hashAt)} : {}), ...geo(d), ...seq(d),
  };
}

export function cteReceiptBody(d: ProofDraft) {
  return {
    occurred_at: datetimeLocalToOffset(d.when), hash_at: datetimeLocalToOffset(d.hashAt), hash: d.hash,
    doc_number: d.docNumber.trim(), name: d.name.trim(),
    ...geo(d), ...(d.nfeKeys.length ? {nfe_keys: d.nfeKeys} : {}), ...seq(d),
  };
}

export interface FailureDraft { reason: string; attempt: string; justification: string }

export function failureIssues(f: FailureDraft): string[] {
  if (!f.reason) return ['Escolha o motivo do insucesso.'];
  const n = f.justification.trim().length;
  if (f.reason === FAILURE_REASON_OTHER && (n < FAILURE_JUSTIFICATION_MIN || n > FAILURE_JUSTIFICATION_MAX)) {
    return [`Justificativa de ${FAILURE_JUSTIFICATION_MIN} a ${FAILURE_JUSTIFICATION_MAX} caracteres.`];
  }
  return [];
}

export function cteFailureBody(d: ProofDraft, f: FailureDraft) {
  return {
    occurred_at: datetimeLocalToOffset(d.when), hash_at: datetimeLocalToOffset(d.hashAt), hash: d.hash,
    ...geo(d), ...(d.nfeKeys.length ? {nfe_keys: d.nfeKeys} : {}), ...seq(d),
    reason_type: Number(f.reason),
    ...(f.attempt ? {attempt: Number(f.attempt)} : {}),
    // A api só aceita justification quando o motivo é "outro".
    ...(f.reason === FAILURE_REASON_OTHER ? {justification: f.justification.trim()} : {}),
  };
}

/** 110150: o transportador é CNPJ (14, também alfanumérico) ou CPF (11); a chave do corpo segue o tamanho. */
export function actorIssues(doc: string): string[] {
  const c = cleanDocument(doc);
  return c.length === 14 && validateCNPJ(c) || c.length === 11 && validateCPF(c) ? [] : ['Escolha o transportador (CPF ou CNPJ válido).'];
}

export function interestedActorBody(doc: string, allowSubcontracts: boolean, sequence: string, authorType?: '1' | '2') {
  const c = cleanDocument(doc);
  return {
    ...(c.length === 14 ? {cnpj: c} : {cpf: c}), allow_subcontracts: allowSubcontracts,
    ...(authorType ? {author_type: authorType} : {}),
    ...(sequence ? {sequence_number: Number(sequence)} : {}),
  };
}

export const receiptCancelBody = (deliveryProtocol: string, sequence: string) => ({
  delivery_protocol: deliveryProtocol, ...(sequence ? {sequence_number: Number(sequence)} : {}),
});

export const cteDeliveryCancelBody = (kind: 'receipt' | 'failure', eventSk: string, sequence: string) => ({
  kind, event_sk: eventSk, ...(sequence ? {sequence_number: Number(sequence)} : {}),
});

/** Permissão do usuário na organização; `*` e `grupo.*` valem como curinga. */
export function hasPermission(permissions: readonly string[] | undefined, needed: string): boolean {
  return !!permissions?.some((p) => p === '*' || p === needed || (p.endsWith('.*') && needed.startsWith(p.slice(0, -1))));
}
