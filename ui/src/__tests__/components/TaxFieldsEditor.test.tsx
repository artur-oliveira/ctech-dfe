import {describe, expect, it} from 'vitest'
import {render, screen} from '@testing-library/react'
import {EMPTY_TAX_GROUPS, TaxFieldsEditor} from '@/components/tax/TaxFieldsEditor'
import type {CfopConfigFormData} from '@/lib/schemas/products'

const baseValue = {
  cfop: '5102', icms: '00', pis: '01', cofins: '01',
  ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
} as CfopConfigFormData

describe('TaxFieldsEditor — grupos opcionais novos', () => {
  it('não mostra os campos do grupo IBS/CBS por padrão', () => {
    render(<TaxFieldsEditor value={baseValue} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.queryByText('IBS UF %')).not.toBeInTheDocument()
  })

  it('mostra valor de pauta quando icms_mod_bc é Pauta fiscal', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '1'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.getByText(/Valor da pauta fiscal/)).toBeInTheDocument()
  })

  it('mostra valor de pauta quando icms_mod_bc é PMPF', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '2'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.getByText(/Valor da pauta fiscal/)).toBeInTheDocument()
  })

  it('não mostra valor de pauta para modo de cálculo padrão', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '3'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.queryByText(/Valor da pauta fiscal/)).not.toBeInTheDocument()
  })

  it('mostra o grupo PIS/COFINS-ST quando habilitado', () => {
    render(<TaxFieldsEditor value={baseValue} onChange={() => {}} simples={false}
                            groups={{...EMPTY_TAX_GROUPS, pisCofinsSt: true}} onGroupsChange={() => {}}/>)
    expect(screen.getByText(/PIS\/COFINS-ST/)).toBeInTheDocument()
    expect(screen.getByText('Alíquota PIS-ST %')).toBeInTheDocument()
  })
})

describe('TaxFieldsEditor — rótulos ligados ao controle', () => {
  const groupsWithMono = {...EMPTY_TAX_GROUPS, icmsMono: true}

  it('associa cada rótulo ao seu campo (clicar no rótulo foca o controle)', () => {
    render(<TaxFieldsEditor value={{...baseValue, icms_mod_bc: '1'}} onChange={() => {}} simples={false}
                            groups={EMPTY_TAX_GROUPS} onGroupsChange={() => {}}/>)
    expect(screen.getByLabelText('CFOP *')).toBeInTheDocument()
    expect(screen.getByLabelText('PIS *')).toBeInTheDocument()
    expect(screen.getByLabelText('COFINS *')).toBeInTheDocument()
    expect(screen.getByLabelText('Alíquota ICMS %')).toBeInTheDocument()
    expect(screen.getByLabelText('Valor da pauta fiscal (R$)')).toBeInTheDocument()
  })

  it('não repete id entre dois editores na mesma tela', () => {
    const {container} = render(
      <div>
        <TaxFieldsEditor value={baseValue} onChange={() => {}} simples={false}
                         groups={groupsWithMono} onGroupsChange={() => {}}/>
        <TaxFieldsEditor value={baseValue} onChange={() => {}} simples={false}
                         groups={groupsWithMono} onGroupsChange={() => {}}/>
      </div>,
    )
    const ids = [...container.querySelectorAll('[id]')].map((el) => el.id)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
