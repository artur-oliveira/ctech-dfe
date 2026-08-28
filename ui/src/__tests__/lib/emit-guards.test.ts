import {describe, expect, it} from 'vitest'
import {duplicataSumGap, paymentBalanceGap, unitDataGap} from '@/lib/utils/emit-guards'

describe('unitDataGap', () => {
  const veiculo = {
    prodType: 'veiculo',
    chassi: '9BWZZZ377VT004251',
    nSerie: '123456',
    nMotor: 'MOT-1',
    armaCount: 0,
  }

  it('aceita o veículo completo', () => {
    expect(unitDataGap(veiculo)).toBeNull()
  })

  it('cobra o chassi com 17 caracteres', () => {
    expect(unitDataGap({...veiculo, chassi: '9BWZZZ377VT'})).toMatch(/17 caracteres/)
    expect(unitDataGap({...veiculo, chassi: null})).toMatch(/17 caracteres/)
  })

  it('recusa I, O e Q no chassi', () => {
    expect(unitDataGap({...veiculo, chassi: '9BWZZZ377VT00425I'})).toMatch(/I, O ou Q/)
    expect(unitDataGap({...veiculo, chassi: '9BWZZZ377VT00425O'})).toMatch(/I, O ou Q/)
    expect(unitDataGap({...veiculo, chassi: '9BWZZZ377VT00425Q'})).toMatch(/I, O ou Q/)
  })

  it('cobra série e motor', () => {
    expect(unitDataGap({...veiculo, nSerie: '  '})).toMatch(/número de série/)
    expect(unitDataGap({...veiculo, nMotor: null})).toMatch(/número do motor/)
  })

  it('cobra ao menos uma arma no item de armamento', () => {
    expect(unitDataGap({prodType: 'arma', armaCount: 0})).toMatch(/ao menos uma arma/)
    expect(unitDataGap({prodType: 'arma', armaCount: 1})).toBeNull()
  })

  it('não cobra nada de um produto comum', () => {
    expect(unitDataGap({prodType: 'generic', armaCount: 0})).toBeNull()
    expect(unitDataGap({prodType: null, armaCount: 0})).toBeNull()
  })
})

describe('paymentBalanceGap', () => {
  it('trata diferença de arredondamento como fechada', () => {
    expect(paymentBalanceGap(0.009, false)).toBeNull()
    expect(paymentBalanceGap(-0.009, false)).toBeNull()
  })

  it('cobra o que falta', () => {
    expect(paymentBalanceGap(12.5, false)).toMatch(/Faltam/)
  })

  it('recusa excedente na NF-e e aceita como troco na NFC-e', () => {
    expect(paymentBalanceGap(-5, false)).toMatch(/não admite troco/)
    expect(paymentBalanceGap(-5, true)).toBeNull()
  })
})

describe('duplicataSumGap', () => {
  it('aceita parcelas que somam a fatura', () => {
    expect(duplicataSumGap(300, 300)).toBeNull()
    expect(duplicataSumGap(300, 299.995)).toBeNull()
  })

  it('acusa parcela faltando e parcela sobrando', () => {
    expect(duplicataSumGap(300, 200)).toMatch(/de R\$/)
    expect(duplicataSumGap(300, 350)).toMatch(/excedem a fatura/)
  })
})
