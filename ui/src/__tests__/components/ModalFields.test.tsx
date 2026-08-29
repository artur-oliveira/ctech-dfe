import {describe, expect, it} from 'vitest'
import {airComplete, railComplete, waterComplete} from '@/components/mdfe/ModalFields'
import type {MdfeAirModalIn, MdfeRailModalIn, MdfeWaterModalIn} from '@/lib/types/api'

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

const water: MdfeWaterModalIn = {
  irin: 'IR1', vessel_type: '01', vessel_code: 'EMB1', vessel_name: 'NAVIO X',
  voyage_number: 'V1', origin_port: 'BRSSZ', dest_port: 'BRRIO',
}

describe('waterComplete', () => {
  it('aceita a embarcação com os sete campos obrigatórios', () => {
    expect(waterComplete(water)).toBe(true)
  })

  it('recusa porto fora do formato UN/LOCODE de 5 caracteres', () => {
    expect(waterComplete({...water, origin_port: 'BRS'})).toBe(false)
    expect(waterComplete({...water, dest_port: ''})).toBe(false)
  })

  it('não exige balsas, terminais nem unidades vazias — todos são minOccurs=0', () => {
    expect(waterComplete({...water, barges: [], loading_terminals: [], empty_cargo_unit_ids: []})).toBe(true)
  })
})
