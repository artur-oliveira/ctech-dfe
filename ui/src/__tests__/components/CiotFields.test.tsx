import {describe, it, expect, vi} from 'vitest';
import {useState} from 'react';
import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {CiotFields} from '@/components/mdfe/CiotFields';
import type {MdfeCiotIn} from '@/lib/types/api';

vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'org#1', name: 'Transportes Exemplo', tax_id: '12.345.678/0001-90'}}),
}));
vi.mock('@/lib/api/client', () => ({
  apiClient: {
    getPersons: () => Promise.resolve({items: [], has_next: false, next_cursor: null}),
    listCiots: () => Promise.resolve({
      items: [
        {id: 'C1', status: 'declared', environment: 'hom', code: '123456789012', freight_value: '1500,00'},
        {id: 'C2', status: 'declared', environment: 'hom', code: '210987654321', mdfe_access_key: 'x'.repeat(44)},
        {id: 'C3', status: 'cancelled', environment: 'hom', code: '111111111111'},
      ],
      has_next: false, next_cursor: null,
    }),
  },
}));

function Harness({initial = []}: { initial?: MdfeCiotIn[] }) {
  const [ciots, setCiots] = useState<MdfeCiotIn[]>(initial);
  return (
    <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
      <CiotFields ciots={ciots} onChange={setCiots} environment="hom"/>
      <output data-testid="state">{JSON.stringify(ciots)}</output>
    </QueryClientProvider>
  );
}

describe('CiotFields', () => {
  it('adiciona um CIOT com a organização como responsável', async () => {
    render(<Harness/>);
    await userEvent.setup().click(screen.getByRole('button', {name: '+ CIOT'}));
    expect(screen.getByTestId('state').textContent).toBe('[{"ciot":"","responsible_doc":"12345678000190"}]');
  });

  it('aceita só dígitos e avisa quando não tem 12', async () => {
    const user = userEvent.setup();
    render(<Harness initial={[{ciot: '', responsible_doc: '12345678000190'}]}/>);
    await user.type(screen.getByLabelText('CIOT (12 dígitos)'), '12ab34');
    expect(screen.getByTestId('state').textContent).toContain('"ciot":"1234"');
    expect(screen.getByText('O CIOT tem 12 dígitos.')).toBeTruthy();
  });

  it('remove um CIOT', async () => {
    render(<Harness initial={[{ciot: '123456789012', responsible_doc: '12345678000190'}]}/>);
    await userEvent.setup().click(screen.getByRole('button', {name: 'Remover'}));
    expect(screen.getByTestId('state').textContent).toBe('[]');
  });

  it('oferece só os CIOTs declarados e livres, e escolher um guarda o id sem digitar código', async () => {
    const user = userEvent.setup();
    render(<Harness initial={[{ciot: '', responsible_doc: '12345678000190'}]}/>);
    await user.click(screen.getByLabelText('Origem do CIOT'));
    expect(await screen.findByText(/123456789012 \(declarado/)).toBeTruthy();
    expect(screen.queryByText(/210987654321/)).toBeNull(); // já vinculado a outro MDF-e
    expect(screen.queryByText(/111111111111/)).toBeNull(); // cancelado
    await user.click(screen.getByText(/123456789012 \(declarado/));
    expect(screen.getByTestId('state').textContent).toBe('[{"ciot":"","responsible_doc":"","ciot_id":"C1"}]');
    expect(screen.queryByLabelText('CIOT (12 dígitos)')).toBeNull();
  });
});
