import {describe, it, expect} from 'vitest'
import {resolveDfeResultToast} from '@/lib/utils/dfe-result-toast'

describe('resolveDfeResultToast — document results', () => {
  it('maps authorized to a success toast', () => {
    expect(resolveDfeResultToast({table_name: 'nfes', status: 'authorized'})).toEqual({
      variant: 'success',
      message: 'NF-e autorizada pela SEFAZ',
    })
  })

  it('maps rejected to an error toast with the motive', () => {
    expect(resolveDfeResultToast({table_name: 'nfes', status: 'rejected', sefaz_motive: 'Duplicidade'})).toEqual({
      variant: 'error',
      message: 'NF-e rejeitada pela SEFAZ — Duplicidade',
    })
  })

  it('falls back to an info toast for unknown statuses', () => {
    expect(resolveDfeResultToast({table_name: 'mdfes', status: 'processing'})).toEqual({
      variant: 'info',
      message: 'MDF-e atualizada — status: processing',
    })
  })
})

describe('resolveDfeResultToast — event results', () => {
  // Regression: a failed MDF-e cancellation used to surface the reverted
  // document status ("authorized" → "Documento autorizado"), masking the error.
  it('reports the event error, NOT the reverted document status', () => {
    const result = resolveDfeResultToast({
      result_kind: 'event',
      table_name: 'mdfes',
      event_type: '110111',
      status: 'error',
      sefaz_motive: 'Failed to sign XML: Unable to resolve reference URI: #',
    })
    expect(result.variant).toBe('error')
    expect(result.message).toBe('Falha ao cancelar MDF-e — Failed to sign XML: Unable to resolve reference URI: #')
  })

  it('does not show "autorizada" even when an authorized status leaks into an event', () => {
    const result = resolveDfeResultToast({
      result_kind: 'event',
      table_name: 'mdfes',
      event_type: '110111',
      status: 'authorized',
    })
    // authorized is not a valid event status → treated as a failure, never a success.
    expect(result.variant).toBe('error')
    expect(result.message).not.toContain('autorizada')
  })

  it('reports a successful cancellation', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'mdfes', event_type: '110111', status: 'success'}),
    ).toEqual({variant: 'success', message: 'MDF-e cancelada com sucesso'})
  })

  it('reports a rejected cancellation with the motive', () => {
    expect(
      resolveDfeResultToast({
        result_kind: 'event',
        table_name: 'nfes',
        event_type: '110111',
        status: 'rejected',
        sefaz_motive: 'Prazo excedido',
      }),
    ).toEqual({variant: 'error', message: 'Cancelamento de NF-e rejeitado pela SEFAZ — Prazo excedido'})
  })

  it('uses encerramento wording for MDF-e event 110112', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'mdfes', event_type: '110112', status: 'success'}),
    ).toEqual({variant: 'success', message: 'MDF-e encerrada com sucesso'})
  })

  it('treats 110112 as cancellation (substituição) for NF-e', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'nfes', event_type: '110112', status: 'success'}),
    ).toEqual({variant: 'success', message: 'NF-e cancelada com sucesso'})
  })
})
