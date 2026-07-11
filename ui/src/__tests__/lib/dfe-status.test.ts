import {describe, it, expect} from 'vitest'
import {QueryClient} from '@tanstack/react-query'
import {setDocStatusOptimistic} from '@/lib/utils/dfe-status'
import type {NfeListOut, PaginatedResponse} from '@/lib/types/api'

function listOf(...statuses: Array<[string, NfeListOut['status']]>): PaginatedResponse<NfeListOut> {
  return {
    items: statuses.map(([sk, status]) => ({sk, status} as NfeListOut)),
    next_cursor: null,
    has_next: false,
  } as PaginatedResponse<NfeListOut>
}

describe('setDocStatusOptimistic', () => {
  it('patches the matching document across paginated cache pages', () => {
    const qc = new QueryClient()
    // Two cached pages under the ['nfces', org] prefix.
    qc.setQueryData(['nfces', 'CNPJ_1', {}, {cursor: undefined}], listOf(['AK1', 'authorized'], ['AK2', 'authorized']))
    qc.setQueryData(['nfces', 'CNPJ_1', {}, {cursor: 'c2'}], listOf(['AK3', 'authorized']))

    setDocStatusOptimistic(qc, ['nfces', 'CNPJ_1'], 'AK1', 'cancel_pending')

    const page1 = qc.getQueryData<PaginatedResponse<NfeListOut>>(['nfces', 'CNPJ_1', {}, {cursor: undefined}])
    expect(page1?.items.find((i) => i.sk === 'AK1')?.status).toBe('cancel_pending')
    expect(page1?.items.find((i) => i.sk === 'AK2')?.status).toBe('authorized')
  })

  it('does not touch caches outside the prefix', () => {
    const qc = new QueryClient()
    qc.setQueryData(['nfes', 'CNPJ_1', {}, {cursor: undefined}], listOf(['AK1', 'authorized']))
    setDocStatusOptimistic(qc, ['nfces', 'CNPJ_1'], 'AK1', 'cancel_pending')
    const nfe = qc.getQueryData<PaginatedResponse<NfeListOut>>(['nfes', 'CNPJ_1', {}, {cursor: undefined}])
    expect(nfe?.items[0].status).toBe('authorized')
  })

  it('patches MDF-e close_pending (generic over status string)', () => {
    const qc = new QueryClient()
    qc.setQueryData(['mdfes', 'CNPJ_1', {}, {cursor: undefined}], listOf(['MK1', 'authorized']))
    setDocStatusOptimistic(qc, ['mdfes', 'CNPJ_1'], 'MK1', 'close_pending')
    const page = qc.getQueryData<PaginatedResponse<NfeListOut>>(['mdfes', 'CNPJ_1', {}, {cursor: undefined}])
    expect(page?.items.find((i) => i.sk === 'MK1')?.status).toBe('close_pending')
  })
})
