import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {QueryClient, QueryClientProvider} from '@tanstack/react-query'
import {InutilizationsTab} from '@/components/dfe/InutilizationsTab'

const listNumberGaps = vi.fn()
const listInutilizations = vi.fn()
const createInutilization = vi.fn()
const downloadInutilizationXml = vi.fn()
const triggerDownload = vi.fn()

vi.mock('@/lib/api/client', () => ({
  apiClient: {
    listNumberGaps: (...a: unknown[]) => listNumberGaps(...a),
    listInutilizations: (...a: unknown[]) => listInutilizations(...a),
    createInutilization: (...a: unknown[]) => createInutilization(...a),
    downloadInutilizationXml: (...a: unknown[]) => downloadInutilizationXml(...a),
  },
  ApiError: class ApiError extends Error {
    detail = ''
  },
}))
vi.mock('sonner', () => ({toast: {info: vi.fn(), error: vi.fn()}}))
vi.mock('@/lib/utils/dfe', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/utils/dfe')>()),
  triggerDownload: (...a: unknown[]) => triggerDownload(...a),
}))

function renderTab() {
  const qc = new QueryClient({defaultOptions: {queries: {retry: false}}})
  return render(
    <QueryClientProvider client={qc}>
      <InutilizationsTab docType="nfe" docLabel="NF-e" orgPk="CNPJ_11222333000181"/>
    </QueryClientProvider>,
  )
}

describe('InutilizationsTab', () => {
  beforeEach(() => {
    listNumberGaps.mockReset()
    listInutilizations.mockReset()
    createInutilization.mockReset()
    downloadInutilizationXml.mockReset()
    triggerDownload.mockReset()
    listInutilizations.mockResolvedValue({items: [], next_cursor: null})
  })

  it('confirms clean numbering when there are no gaps', async () => {
    listNumberGaps.mockResolvedValue({items: []})
    renderTab()

    expect(await screen.findByText(/Numeração sem lacunas/)).toBeInTheDocument()
  })

  it('submits the gap range prefilled from the detected gap', async () => {
    listNumberGaps.mockResolvedValue({items: [{serie: 1, number_start: 131, number_end: 133}]})
    createInutilization.mockResolvedValue({sk: 'x'})
    renderTab()

    // The gap teaches the range: 3 numbers, series 1.
    expect(await screen.findByText('3 números sem documento utilizável')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', {name: 'Inutilizar'}))

    const submit = screen.getByRole('button', {name: 'Enviar à SEFAZ'})
    // Justification below the SEFAZ minimum keeps the action disabled.
    await userEvent.type(screen.getByLabelText('Justificativa'), 'curta')
    expect(submit).toBeDisabled()

    await userEvent.clear(screen.getByLabelText('Justificativa'))
    await userEvent.type(screen.getByLabelText('Justificativa'), 'numeros perdidos em falha de transmissao')
    await userEvent.click(submit)

    await waitFor(() => expect(createInutilization).toHaveBeenCalledWith('nfe', {
      serie: 1,
      number_start: 131,
      number_end: 133,
      justification: 'numeros perdidos em falha de transmissao',
    }))
  })

  it('blocks an inverted range', async () => {
    listNumberGaps.mockResolvedValue({items: []})
    renderTab()

    await userEvent.click(await screen.findByRole('button', {name: 'Inutilizar faixa'}))
    await userEvent.type(screen.getByLabelText('Número inicial'), '20')
    await userEvent.type(screen.getByLabelText('Número final'), '10')
    await userEvent.type(screen.getByLabelText('Justificativa'), 'faixa invertida para teste de validacao')

    expect(screen.getByText('O número final deve ser maior ou igual ao inicial.')).toBeInTheDocument()
    expect(screen.getByRole('button', {name: 'Enviar à SEFAZ'})).toBeDisabled()
    expect(createInutilization).not.toHaveBeenCalled()
  })
})

describe('InutilizationsTab XML download', () => {
  const homologated = {
    sk: 'inut-1', year: 2026, serie: 1, number_start: 118, number_end: 120,
    justification: 'numeros perdidos por falha de transmissao',
    status: 'success', xml_s3_key: 'nfe/hom/CNPJ_1/inut.xml',
    sefaz_status: '102', sefaz_motive: null,
    pk: 'INUT#hom#CNPJ_1', event_type: 'INUT', event_key: 'INUT#2026',
    user_id: 'u', user_name: 'U',
    created_at: '2026-07-04T09:15:00Z', updated_at: '2026-07-04T09:15:00Z',
  }

  beforeEach(() => {
    listNumberGaps.mockReset()
    downloadInutilizationXml.mockReset()
    triggerDownload.mockReset()
    listNumberGaps.mockResolvedValue({items: []})
  })

  it('downloads the ProcInutNFe of a homologated range', async () => {
    listInutilizations.mockResolvedValue({items: [homologated], next_cursor: null})
    const blob = new Blob(['<ProcInutNFe/>'], {type: 'application/xml'})
    downloadInutilizationXml.mockResolvedValue(blob)
    renderTab()

    await userEvent.click(await screen.findByRole('button', {name: 'XML'}))

    await waitFor(() => expect(downloadInutilizationXml).toHaveBeenCalledWith('nfe', 'inut-1'))
    expect(triggerDownload).toHaveBeenCalledWith(blob, 'inutilizacao_2026_1_118-120.xml')
  })

  // Antes da homologação não existe documento nenhum — o botão não deve aparecer.
  it('hides the download while the range has no stored XML', async () => {
    listInutilizations.mockResolvedValue({
      items: [{...homologated, status: 'pending', xml_s3_key: null}],
      next_cursor: null,
    })
    renderTab()

    expect(await screen.findByText('118 – 120')).toBeInTheDocument()
    expect(screen.queryByRole('button', {name: 'XML'})).not.toBeInTheDocument()
  })
})
