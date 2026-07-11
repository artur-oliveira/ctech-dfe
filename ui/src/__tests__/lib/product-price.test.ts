import {describe, it, expect} from 'vitest'
import {resolveUnitPrice} from '@/lib/data/product-price'

const product = {value: '10.00', value_resale: '8.00'}

describe('resolveUnitPrice', () => {
  it('usa preço de consumidor final para CPF (11 dígitos)', () => {
    expect(resolveUnitPrice(product, '12345678901')).toBe('10.00')
    expect(resolveUnitPrice(product, '123.456.789-01')).toBe('10.00')
  })

  it('usa preço de revenda para CNPJ (14 dígitos)', () => {
    expect(resolveUnitPrice(product, '12345678000199')).toBe('8.00')
    expect(resolveUnitPrice(product, '12.345.678/0001-99')).toBe('8.00')
  })

  it('faz fallback para consumidor final quando revenda ausente em CNPJ', () => {
    expect(resolveUnitPrice({value: '10.00'}, '12345678000199')).toBe('10.00')
    expect(resolveUnitPrice({value: '10.00', value_resale: ''}, '12345678000199')).toBe('10.00')
    expect(resolveUnitPrice({value: '10.00', value_resale: null}, '12345678000199')).toBe('10.00')
  })

  it('usa consumidor final quando destinatário desconhecido', () => {
    expect(resolveUnitPrice(product, '')).toBe('10.00')
  })
})
