import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {MAINTENANCE_PATH, redirectOnMaintenance, takeMaintenanceReturn} from '@/lib/network/maintenance'

// jsdom's location cannot navigate and cannot be spied on, so the whole object
// is stood in for. The replace is what we assert on.
const replace = vi.fn()

function at(pathname: string, search = '') {
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {pathname, search, replace},
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  at('/nfe', '?page=2')
})
afterEach(() => sessionStorage.clear())

describe('manutenção (503)', () => {
  it('leva para a tela de manutenção e lembra onde a pessoa estava', () => {
    expect(redirectOnMaintenance(503)).toBe(true)
    expect(replace).toHaveBeenCalledWith(MAINTENANCE_PATH)
    expect(takeMaintenanceReturn()).toBe('/nfe?page=2')
  })

  it('ignora qualquer outro status', () => {
    expect(redirectOnMaintenance(500)).toBe(false)
    expect(redirectOnMaintenance(undefined)).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  })

  // Um segundo 503 chegando enquanto a tela já está aberta não pode reescrever
  // a URL: o destino guardado seria a própria tela de manutenção.
  it('não redireciona de novo quando já está na tela', () => {
    at(MAINTENANCE_PATH)
    sessionStorage.setItem('dfe:return-after-maintenance', '/nfe?page=2')

    expect(redirectOnMaintenance(503)).toBe(true)
    expect(replace).not.toHaveBeenCalled()
    expect(takeMaintenanceReturn()).toBe('/nfe?page=2')
  })

  it('volta para o painel quando não há destino guardado', () => {
    expect(takeMaintenanceReturn()).toBe('/dashboard')
  })
})
