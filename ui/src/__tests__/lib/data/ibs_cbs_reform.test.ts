import {describe, expect, it} from 'vitest'
import {
  ALC_ZFM_TP_CBS_OPTIONS,
  COMPRA_GOV_TP_ENTE_OPTIONS,
  COMPRA_GOV_TP_OPER_COM_REFERENCIA,
  COMPRA_GOV_TP_OPER_OPTIONS,
  COMPRA_GOV_TP_OPER_REFERENCIA_UNICA,
  IBS_CBS_C_CRED_PRES_OPTIONS,
  IBS_IND_DOACAO_SIM,
  TP_CRED_PRES_IBS_ZFM_OPTIONS,
  TP_NF_CREDITO_OPTIONS,
  TP_NF_DEBITO_OPTIONS,
} from '@/lib/data/ibs_cbs_reform'

describe('domínios da reforma tributária', () => {
  it('indDoacao aceita 1 e nada mais (TIndDoacao)', () => {
    expect(IBS_IND_DOACAO_SIM).toBe('1')
  })

  it('tpNFDebito cobre 01 a 08 com dois dígitos', () => {
    expect(TP_NF_DEBITO_OPTIONS.map((o) => o.value)).toEqual(
      ['01', '02', '03', '04', '05', '06', '07', '08'],
    )
  })

  it('tpNFCredito cobre 01 a 06 com dois dígitos', () => {
    expect(TP_NF_CREDITO_OPTIONS.map((o) => o.value)).toEqual(
      ['01', '02', '03', '04', '05', '06'],
    )
  })

  it('cCredPres tem os 13 códigos da tabela, com zero à esquerda', () => {
    expect(IBS_CBS_C_CRED_PRES_OPTIONS).toHaveLength(13)
    expect(IBS_CBS_C_CRED_PRES_OPTIONS[0].value).toBe('01')
    expect(IBS_CBS_C_CRED_PRES_OPTIONS[12].value).toBe('13')
    for (const opt of IBS_CBS_C_CRED_PRES_OPTIONS) {
      expect(opt.value).toMatch(/^\d{2}$/)
    }
  })

  it('tpCredPresIBSZFM cobre 0 a 4', () => {
    expect(TP_CRED_PRES_IBS_ZFM_OPTIONS.map((o) => o.value)).toEqual(['0', '1', '2', '3', '4'])
  })

  it('tpALCZFMCBS cobre 1 e 2', () => {
    expect(ALC_ZFM_TP_CBS_OPTIONS.map((o) => o.value)).toEqual(['1', '2'])
  })

  it('tpEnteGov cobre 1 a 6', () => {
    expect(COMPRA_GOV_TP_ENTE_OPTIONS.map((o) => o.value)).toEqual(['1', '2', '3', '4', '5', '6'])
  })
})

describe('regra do refDFeAnt por tipo de operação governamental', () => {
  it('tpOperGov cobre 1 a 4', () => {
    expect(COMPRA_GOV_TP_OPER_OPTIONS.map((o) => o.value)).toEqual(['1', '2', '3', '4'])
  })

  it('só os tipos 2 e 3 exigem documento anterior', () => {
    expect(COMPRA_GOV_TP_OPER_COM_REFERENCIA.has('2')).toBe(true)
    expect(COMPRA_GOV_TP_OPER_COM_REFERENCIA.has('3')).toBe(true)
    expect(COMPRA_GOV_TP_OPER_COM_REFERENCIA.has('1')).toBe(false)
    expect(COMPRA_GOV_TP_OPER_COM_REFERENCIA.has('4')).toBe(false)
  })

  it('o tipo 2 é o que aceita uma chave só', () => {
    expect(COMPRA_GOV_TP_OPER_REFERENCIA_UNICA).toBe('2')
  })
})
