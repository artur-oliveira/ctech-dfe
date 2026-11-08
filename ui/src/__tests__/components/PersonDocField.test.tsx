import {describe, expect, it, vi} from 'vitest';
import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';

vi.mock('@/components/persons/PersonPicker', () => ({
  PersonPicker: ({onChange}: {onChange: (p: unknown) => void}) => (
    <>
      <button onClick={() => onChange({sk: 'CNPJ_12ABC34501DE35', name: 'Empresa Alfa'})}>alfa</button>
      <button onClick={() => onChange({sk: 'CNPJ_11222333000181', name: 'Empresa Num'})}>num</button>
    </>
  ),
}));

import {PersonDocField} from '@/components/dfe/PersonDocField';

describe('PersonDocField', () => {
  it('recusa CNPJ alfanumérico: o esquema do evento só aceita dígitos', async () => {
    const onChange = vi.fn();
    render(<PersonDocField id="x" label="Recebedor" value={null} onChange={onChange}/>);
    await userEvent.click(screen.getByText('alfa'));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByRole('alert').textContent).toMatch(/só aceita CNPJ numérico/);
  });

  it('aceita CNPJ numérico válido', async () => {
    const onChange = vi.fn();
    render(<PersonDocField id="x" label="Recebedor" value={null} onChange={onChange}/>);
    await userEvent.click(screen.getByText('num'));
    expect(onChange).toHaveBeenCalledWith({doc: '11222333000181', name: 'Empresa Num'});
  });
});
