import {describe, expect, it} from 'vitest'
import {advancedErrorLabels, listPtBR} from '@/lib/utils/advanced-errors'

describe('erros dos dados complementares', () => {
  // O aviso contava todo erro em `person`, e a IE da organização é renderizada
  // no cartão principal — então a seção anunciava "1" sem ter nenhum campo com
  // erro dentro dela, mandando procurar onde não estava.
  it('não conta a inscrição estadual de uma organização', () => {
    const errors = {state_registrations: {message: 'UF duplicada'}}
    expect(advancedErrorLabels(errors, true)).toEqual([])
    expect(advancedErrorLabels(errors, false)).toEqual(['Inscrições estaduais'])
  })

  it('nomeia os campos, na ordem em que aparecem', () => {
    const labels = advancedErrorLabels({cnae: {}, contacts: {}, fantasy_name: {}}, true)
    expect(labels).toEqual(['Nome fantasia', 'CNAE', 'Contatos'])
  })

  // O endereço principal fica no cartão acima; só os adicionais são da seção.
  it('conta endereços adicionais e ignora o principal', () => {
    expect(advancedErrorLabels({addresses: [{street: {}}]}, true)).toEqual([])
    expect(advancedErrorLabels({addresses: [undefined, {street: {}}]}, true))
      .toEqual(['Endereços adicionais'])
  })

  it('não diz nada quando não há erro', () => {
    expect(advancedErrorLabels(undefined, true)).toEqual([])
    expect(advancedErrorLabels({}, false)).toEqual([])
  })
})

describe('listPtBR', () => {
  it('escreve a lista como se escreve', () => {
    expect(listPtBR([])).toBe('')
    expect(listPtBR(['CNAE'])).toBe('CNAE')
    expect(listPtBR(['CNAE', 'Contatos'])).toBe('CNAE e Contatos')
    expect(listPtBR(['CNAE', 'Contatos', 'NFS-e'])).toBe('CNAE, Contatos e NFS-e')
  })
})
