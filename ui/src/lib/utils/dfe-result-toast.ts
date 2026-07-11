// Resolves a `dfe_result` realtime message into the toast that should be shown.
//
// A `dfe_result` is either a DOCUMENT result (carries the document status) or a
// SEFAZ EVENT result (cancellation, encerramento — carries the event outcome).
// For events the EVENT status must drive the message: the worker may revert the
// document to "authorized" after a failed/rejected event, and reporting that
// document status would mask the real event failure.

// result_kind values (mirror worker constants).
export const RESULT_KIND_EVENT = 'event'

// SEFAZ event status values (mirror worker EventStatus* constants).
export const EVENT_STATUS_SUCCESS = 'success'
export const EVENT_STATUS_REJECTED = 'rejected'

// SEFAZ event type codes. 110112 is "encerramento" for MDF-e but
// "cancelamento por substituição" for NF-e/NFC-e — disambiguated by table_name.
const EVENT_TYPE_ENCERRAMENTO = '110112'
const TABLE_MDFE = 'mdfes'

const TABLE_LABEL: Record<string, string> = {
  nfes: 'NF-e',
  nfces: 'NFC-e',
  ctes: 'CT-e',
  mdfes: 'MDF-e',
}

export interface DfeResultMessage {
  result_kind?: string
  table_name?: string
  status?: string
  sefaz_motive?: string
  event_type?: string
}

export type ToastVariant = 'success' | 'error' | 'info'

export interface ResolvedToast {
  variant: ToastVariant
  message: string
}

function docLabel(tableName?: string): string {
  return TABLE_LABEL[tableName ?? ''] ?? 'Documento'
}

function motiveSuffix(motive?: string): string {
  return motive ? ` — ${motive}` : ''
}

function resolveDocumentToast(msg: DfeResultMessage): ResolvedToast {
  const doc = docLabel(msg.table_name)
  const motive = motiveSuffix(msg.sefaz_motive)

  switch (msg.status) {
    case 'authorized':
      return {variant: 'success', message: `${doc} autorizada pela SEFAZ`}
    case 'cancelled':
      return {variant: 'success', message: `${doc} cancelada com sucesso`}
    case 'rejected':
      return {variant: 'error', message: `${doc} rejeitada pela SEFAZ${motive}`}
    case 'failed':
      return {variant: 'error', message: `Falha ao processar ${doc}${motive}`}
    default:
      return {variant: 'info', message: `${doc} atualizada — status: ${msg.status}`}
  }
}

function resolveEventToast(msg: DfeResultMessage): ResolvedToast {
  const doc = docLabel(msg.table_name)
  const motive = motiveSuffix(msg.sefaz_motive)
  const isEncerramento = msg.event_type === EVENT_TYPE_ENCERRAMENTO && msg.table_name === TABLE_MDFE

  const wording = isEncerramento
    ? {success: `${doc} encerrada com sucesso`, fail: `Falha ao encerrar ${doc}`, reject: `Encerramento de ${doc} rejeitado pela SEFAZ`}
    : {success: `${doc} cancelada com sucesso`, fail: `Falha ao cancelar ${doc}`, reject: `Cancelamento de ${doc} rejeitado pela SEFAZ`}

  switch (msg.status) {
    case EVENT_STATUS_SUCCESS:
      return {variant: 'success', message: wording.success}
    case EVENT_STATUS_REJECTED:
      return {variant: 'error', message: `${wording.reject}${motive}`}
    default:
      // 'error' or any unexpected status — treat as a processing failure.
      return {variant: 'error', message: `${wording.fail}${motive}`}
  }
}

export function resolveDfeResultToast(msg: DfeResultMessage): ResolvedToast {
  return msg.result_kind === RESULT_KIND_EVENT
    ? resolveEventToast(msg)
    : resolveDocumentToast(msg)
}
