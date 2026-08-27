import {describe, expect, it} from 'vitest'
import {airComplete, railComplete} from '@/components/mdfe/ModalFields'
import type {MdfeAirModalIn, MdfeRailModalIn} from '@/lib/types/api'

const air: MdfeAirModalIn = {
  nationality: 'PP', registration: 'ABC123', flight_number: 'JJ1234',
  origin_airport: 'GRU', dest_airport: 'SDU', flight_date: '2026-09-01',
}

const rail: MdfeRailModalIn = {
  train_prefix: 'TR1', origin_station: 'PORTO', dest_station: 'TERMINAL',
  wagons: [{weight_bc: '1000.000', weight_real: '1100.000', series: 'A', number: '1', tu: '10.000'}],
}

describe('airComplete', () => {
  it('aceita o voo com os seis campos do XSD', () => {
    expect(airComplete(air)).toBe(true)
  })

  it('recusa qualquer campo faltando — nenhum é opcional no modal aéreo', () => {
    for (const key of Object.keys(air) as (keyof MdfeAirModalIn)[]) {
      expect(airComplete({...air, [key]: ''})).toBe(false)
    }
  })
})

describe('railComplete', () => {
  it('aceita o trem com um vagão completo', () => {
    expect(railComplete(rail)).toBe(true)
  })

  it('recusa trem sem vagão — qVag sai desta lista', () => {
    expect(railComplete({...rail, wagons: []})).toBe(false)
  })

  it('recusa vagão sem tonelada útil', () => {
    expect(railComplete({...rail, wagons: [{...rail.wagons[0], tu: ''}]})).toBe(false)
  })

  it('aceita vagão sem os campos opcionais tpVag e nSeq', () => {
    expect(railComplete({...rail, wagons: [{...rail.wagons[0], wagon_type: '', sequence: ''}]})).toBe(true)
  })
})
