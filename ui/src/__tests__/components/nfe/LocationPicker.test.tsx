import {describe, it, expect, vi} from 'vitest'
import {render, screen, fireEvent} from '@testing-library/react'
import {LocationPicker} from '@/components/nfe/LocationPicker'
import type {NfeLocalOut} from '@/lib/types/api'

const savedLocation: NfeLocalOut = {
  x_lgr: 'Rua Salva', nro: '10', x_bairro: 'Centro', c_mun: '3550308', x_mun: 'São Paulo', uf: 'SP',
}

describe('LocationPicker', () => {
  it('renders collapsed by default (only the toggle button)', () => {
    render(
      <LocationPicker label="Local de entrega" savedLocations={[]} value={null}
                      onChange={() => {}} save={false} onSaveChange={() => {}}/>
    )
    expect(screen.getByText('+ Local de entrega')).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('Logradouro')).not.toBeInTheDocument()
  })

  it('clicking a saved-location chip calls onChange with that location', () => {
    const onChange = vi.fn()
    render(
      <LocationPicker label="Local de entrega" savedLocations={[savedLocation]} value={null}
                      onChange={onChange} save={false} onSaveChange={() => {}}/>
    )
    fireEvent.click(screen.getByText('+ Local de entrega'))
    fireEvent.click(screen.getByText('Rua Salva, 10'))
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({x_lgr: 'Rua Salva', nro: '10'}))
  })

  it('toggling "endereço diferente" reveals the manual form', () => {
    render(
      <LocationPicker label="Local de entrega" savedLocations={[savedLocation]} value={null}
                      onChange={() => {}} save={false} onSaveChange={() => {}}/>
    )
    fireEvent.click(screen.getByText('+ Local de entrega'))
    expect(screen.queryByPlaceholderText('Logradouro')).not.toBeInTheDocument()
    fireEvent.click(screen.getByText('+ Endereço diferente'))
    expect(screen.getByPlaceholderText('Logradouro')).toBeInTheDocument()
  })

  it('shows the manual form directly when there are no saved locations', () => {
    render(
      <LocationPicker label="Local de retirada" savedLocations={[]} value={null}
                      onChange={() => {}} save={false} onSaveChange={() => {}}/>
    )
    fireEvent.click(screen.getByText('+ Local de retirada'))
    expect(screen.getByPlaceholderText('Logradouro')).toBeInTheDocument()
  })
})
