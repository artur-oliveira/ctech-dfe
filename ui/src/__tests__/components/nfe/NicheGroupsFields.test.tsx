import {describe, expect, it, vi} from 'vitest'
import {fireEvent, render, screen} from '@testing-library/react'
import {
  EMPTY_NICHE_GROUPS,
  NicheGroupsFields,
  type NicheGroupsValue,
} from '@/components/nfe/NicheGroupsFields'

function setup(value: NicheGroupsValue = EMPTY_NICHE_GROUPS, props: Partial<{
  canaSafra: string | null
  technicalManagerCpf: string | null
}> = {}) {
  const onChange = vi.fn()
  render(
    <NicheGroupsFields value={value} onChange={onChange}
                      canaSafra={props.canaSafra ?? null}
                      technicalManagerCpf={props.technicalManagerCpf ?? null}/>,
  )
  return {onChange}
}

describe('NicheGroupsFields', () => {
  it('cana fica desabilitada sem safra cadastrada na operação', () => {
    setup()
    expect(screen.getByLabelText(/Aquisição de cana/)).toBeDisabled()
    expect(screen.getByText(/Cadastre a safra na natureza de operação/)).toBeInTheDocument()
  })

  it('com safra cadastrada, marcar o grupo cria um lançamento diário e o mês de referência', () => {
    const {onChange} = setup(EMPTY_NICHE_GROUPS, {canaSafra: '2025/2026'})
    fireEvent.click(screen.getByLabelText(/Aquisição de cana/))
    const cana = onChange.mock.calls[0][0].cana
    expect(cana.deliveries).toHaveLength(1)
    expect(cana.ref).toMatch(/^(0[1-9]|1[0-2])\/2\d{3}$/)
  })

  it('a safra aparece somente leitura — vem do cadastro, não da nota', () => {
    setup({...EMPTY_NICHE_GROUPS, cana: {ref: '09/2026', deliveries: [{dia: '1', qtde: '10'}]}},
      {canaSafra: '2025/2026'})
    expect(screen.getByDisplayValue('2025/2026')).toBeDisabled()
  })

  it('não expõe os totais derivados da cana', () => {
    setup({...EMPTY_NICHE_GROUPS, cana: {ref: '09/2026', deliveries: [{dia: '1', qtde: '10'}]}},
      {canaSafra: '2025/2026'})
    expect(screen.queryByText(/Total do mês/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/Total geral/i)).not.toBeInTheDocument()
    expect(screen.getByText(/são calculados na emissão/)).toBeInTheDocument()
  })

  it('o choice do agropecuario é um radio: escolher guia zera o receituário', () => {
    const {onChange} = setup({...EMPTY_NICHE_GROUPS, agro: {receituarios: ['REC-1']}})
    fireEvent.click(screen.getByLabelText('Guia de trânsito'))
    const agro = onChange.mock.calls[0][0].agro
    expect(agro.guia).toBeTruthy()
    expect(agro.receituarios).toBeUndefined()
  })

  it('escolher "não se aplica" remove o grupo inteiro', () => {
    const {onChange} = setup({...EMPTY_NICHE_GROUPS, agro: {receituarios: ['REC-1']}})
    fireEvent.click(screen.getByLabelText('Não se aplica'))
    expect(onChange.mock.calls[0][0].agro).toBeNull()
  })

  it('avisa quando o receituário é escolhido sem CPF do responsável técnico', () => {
    setup({...EMPTY_NICHE_GROUPS, agro: {receituarios: ['']}})
    expect(screen.getByText(/responsável técnico agronômico/)).toBeInTheDocument()
  })

  it('com o CPF cadastrado, o aviso desaparece', () => {
    setup({...EMPTY_NICHE_GROUPS, agro: {receituarios: ['']}},
      {technicalManagerCpf: '11144477735'})
    expect(screen.queryByText(/Cadastre o CPF do responsável técnico/)).not.toBeInTheDocument()
  })

  it('pedido e contrato da compra pública chegam ao onChange', () => {
    const {onChange} = setup()
    fireEvent.change(screen.getByLabelText('Pedido'), {target: {value: 'PED-1'}})
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({compraXPed: 'PED-1'}))
  })
})
