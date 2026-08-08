/**
 * Vocabulário canônico de status de DF-e — espelha `worker/internal/service/helpers.go`.
 *
 * Todo badge, lista, detalhe e toast lê daqui, para que um status novo no
 * backend seja descrito da mesma forma em toda a UI. Documentos e eventos
 * compartilham a tabela: o worker grava os mesmos valores nos dois (um evento
 * também passa por `processing` e `retryable_failed`), e só `success` é
 * exclusivo de evento.
 */

export type DfeStatus =
  | 'pending'
  | 'processing'
  | 'retryable_failed'
  | 'cancel_pending'
  | 'close_pending'
  | 'authorized'
  | 'rejected'
  | 'failed'
  | 'error'
  | 'cancelled'
  | 'closed'
  // Exclusivo de evento SEFAZ (worker: EventStatusSuccess).
  | 'success'

/** Gênero do substantivo do documento: nota (f) vs manifesto/conhecimento/evento (m). */
export type DfeGender = 'f' | 'm'

export type DfeStatusTone = 'success' | 'danger' | 'warning' | 'info' | 'neutral'

interface StatusMeta {
  /** `@` vira a vogal de gênero: 'Autorizad@' → Autorizada / Autorizado. */
  label: string
  tone: DfeStatusTone
  /** Em voo: o worker ainda deve uma resposta. Ganha pulso + ponto animado. */
  transitional?: boolean
  /**
   * Título do modal de motivo. Presente = este status é explicado por um motivo
   * (SEFAZ ou worker) e o badge vira botão. Rejeição, falha e retentativa têm
   * causas diferentes e não podem sair todas como "motivo da rejeição".
   */
  motiveTitle?: string
}

const STATUS_META: Record<DfeStatus, StatusMeta> = {
  pending: {label: 'Pendente', tone: 'warning', transitional: true},
  processing: {label: 'Processando', tone: 'info', transitional: true},
  // Falha de transporte/infra: não é terminal, a próxima entrega SQS reclama o
  // documento. O usuário não precisa agir — por isso "tentando novamente", e
  // não "falha".
  retryable_failed: {label: 'Tentando novamente', tone: 'warning', transitional: true, motiveTitle: 'Motivo da retentativa'},
  cancel_pending: {label: 'Cancelando', tone: 'warning', transitional: true},
  close_pending: {label: 'Encerrando', tone: 'info', transitional: true},
  authorized: {label: 'Autorizad@', tone: 'success'},
  rejected: {label: 'Rejeitad@', tone: 'danger', motiveTitle: 'Motivo da rejeição'},
  failed: {label: 'Falha', tone: 'danger', motiveTitle: 'Motivo da falha'},
  error: {label: 'Erro', tone: 'danger', motiveTitle: 'Motivo da falha'},
  cancelled: {label: 'Cancelad@', tone: 'neutral'},
  closed: {label: 'Encerrad@', tone: 'info'},
  success: {label: 'Registrad@', tone: 'success'},
}

/**
 * Paleta semântica fixa — NÃO recolorida por `data-dfe-theme` (DESIGN.md §7):
 * "Autorizada" precisa ser reconhecível igual em qualquer tipo de documento.
 * Um tom por significado; o rótulo (e o pulso) distingue status do mesmo tom.
 */
const TONE_CLASSES: Record<DfeStatusTone, string> = {
  success: 'bg-green-100 text-green-700',
  danger: 'bg-red-100 text-red-700',
  warning: 'bg-amber-50 text-amber-700',
  info: 'bg-blue-50 text-blue-700',
  neutral: 'bg-gray-100 text-gray-500',
}

const UNKNOWN_CLASSES = 'bg-gray-100 text-gray-600'

const meta = (status: string): StatusMeta | undefined => STATUS_META[status as DfeStatus]

/** Status desconhecido devolve o próprio valor — nunca "Desconhecido", que esconde a informação de quem depura. */
export const dfeStatusLabel = (status: string, gender: DfeGender = 'f'): string =>
  meta(status)?.label.replace('@', gender === 'f' ? 'a' : 'o') ?? status

export const dfeStatusTone = (status: string): DfeStatusTone => meta(status)?.tone ?? 'neutral'

export const dfeStatusClasses = (status: string): string => {
  const m = meta(status)
  return m ? TONE_CLASSES[m.tone] : UNKNOWN_CLASSES
}

export const isTransitionalDfeStatus = (status: string): boolean => meta(status)?.transitional ?? false

/** Título do modal de motivo, ou null quando nenhum motivo explica este status. */
export const dfeStatusMotiveTitle = (status: string): string | null => meta(status)?.motiveTitle ?? null

/** Gênero por tabela do backend — usado pelos toasts, que só recebem `table_name`. */
export const DOC_GENDER: Record<string, DfeGender> = {
  nfes: 'f',
  nfces: 'f',
  nfses: 'f',
  ctes: 'm',
  mdfes: 'm',
}

/** Status que uma NFS-e alcança (spec §3.4) — alimenta o filtro da lista. */
export const NFSE_STATUSES: readonly DfeStatus[] = [
  'pending', 'processing', 'retryable_failed', 'authorized', 'rejected', 'cancelled', 'error',
]

export const dfeStatusOptions = (statuses: readonly DfeStatus[], gender: DfeGender = 'f') =>
  statuses.map((value) => ({value, label: dfeStatusLabel(value, gender)}))
