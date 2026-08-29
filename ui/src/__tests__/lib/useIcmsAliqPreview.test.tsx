import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import {act, renderHook} from '@testing-library/react'
import {useIcmsAliqPreview} from '@/lib/hooks/useIcmsAliqPreview'
import {apiClient} from '@/lib/api/client'

describe('useIcmsAliqPreview', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('consulta uma vez só e não re-dispara enquanto as UFs não mudam', async () => {
    // Regressão: a query era um objeto literal novo a cada render, e o
    // useDebounce compara por identidade — o timer rearmava para sempre,
    // disparando um GET a cada 300ms enquanto o formulário estivesse montado.
    const spy = vi.spyOn(apiClient, 'getIcmsAliqPreview')
      .mockResolvedValue({icms_aliq: '18.00', fcp_aliq: '2.00'})

    const {result, rerender} = renderHook(() => useIcmsAliqPreview('SP', 'RJ', '84713012'))

    await act(() => vi.advanceTimersByTimeAsync(400))
    expect(result.current).toEqual({icms_aliq: '18.00', fcp_aliq: '2.00'})
    expect(spy).toHaveBeenCalledTimes(1)

    rerender()
    await act(() => vi.advanceTimersByTimeAsync(2000))
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('reconsulta quando a UF de destino muda', async () => {
    const spy = vi.spyOn(apiClient, 'getIcmsAliqPreview')
      .mockResolvedValue({icms_aliq: '18.00', fcp_aliq: '2.00'})

    const {rerender} = renderHook(
      ({dest}: {dest: string}) => useIcmsAliqPreview('SP', dest, '84713012'),
      {initialProps: {dest: 'RJ'}},
    )

    await act(() => vi.advanceTimersByTimeAsync(400))
    expect(spy).toHaveBeenCalledTimes(1)

    rerender({dest: 'MG'})
    await act(() => vi.advanceTimersByTimeAsync(400))
    expect(spy).toHaveBeenCalledTimes(2)
  })
})
