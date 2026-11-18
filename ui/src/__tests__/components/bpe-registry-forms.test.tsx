import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {BpeLineForm} from '@/components/bpe/BpeLineForm';
import {BpeTicketForm} from '@/components/bpe/BpeTicketForm';
import {apiClient} from '@/lib/api/client';
import type {BpeLineItemOut} from '@/lib/types/api';

const push = vi.fn();
vi.mock('next/navigation', () => ({useRouter: () => ({push, replace: vi.fn()}), useSearchParams: () => new URLSearchParams('')}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181', permissions: ['create.bpes']}})}));
vi.mock('@/components/ui/municipality-field', () => ({MunicipalityField: ({label}: { label: string }) => <p>{label}</p>}));
vi.mock('@/components/persons/PersonPicker', () => ({PersonPicker: () => <p>seletor de pessoa</p>}));

const normalLine: BpeLineItemOut = {
  pk: 'p', sk: 'BPELINE_1', name: 'SP x RJ', variant: 'normal', modal: 'rodoviario', route_code: 'L100', route_name: 'SAO PAULO X RIO',
  origin: {uf: 'SP', ibge_code: '3550308', city: 'Sao Paulo'}, destination: {uf: 'RJ', ibge_code: '3304557', city: 'Rio'},
  icms: {cst: '40'},
};

function wrap(node: React.ReactNode) {
  return render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{node}</QueryClientProvider>);
}

beforeEach(() => push.mockClear());
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('formulário de linha', () => {
  it('cada variante mostra só os campos dela', () => {
    wrap(<BpeLineForm onSubmit={vi.fn()}/>);
    expect(screen.getByLabelText(/Código da linha/)).toBeInTheDocument();
    expect(screen.getByText('Imposto (ICMS)')).toBeInTheDocument();
    expect(screen.queryByLabelText(/Número do voo/)).toBeNull();
  });

  it('a edição trava o tipo e o envio leva só os campos da variante', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    wrap(<BpeLineForm initialData={{...normalLine, trip_type: '00', service_type: '6', accommodation: '1'}} onSubmit={onSubmit}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Salvar'}));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    const body = onSubmit.mock.calls[0][0];
    expect(body).toMatchObject({variant: 'normal', route_code: 'L100', icms: {cst: '40', rate: null}});
    expect('carrier' in body || 'legs' in body).toBe(false);
  });

  it('linha sem código não é enviada e mostra o erro', async () => {
    const onSubmit = vi.fn();
    wrap(<BpeLineForm initialData={{...normalLine, route_code: '', trip_type: '00', service_type: '6', accommodation: '1'}} onSubmit={onSubmit}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Salvar'}));
    expect(await screen.findByText('Informe o código da linha')).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe('venda do bilhete', () => {
  it('sem linha cadastrada avisa, leva ao cadastro e bloqueia a emissão', async () => {
    vi.spyOn(apiClient, 'getBpeLines').mockResolvedValue({items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false});
    wrap(<BpeTicketForm variant="normal"/>);
    expect(await screen.findByRole('link', {name: /uma linha do bilhete comum/})).toHaveAttribute('href', '/bpe-lines/new');
    expect(screen.getByRole('button', {name: 'Emitir BP-e'})).toBeDisabled();
  });

  it('com linha cadastrada a venda mostra só o que varia, sem percurso nem imposto', async () => {
    vi.spyOn(apiClient, 'getBpeLines').mockResolvedValue({items: [normalLine], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false});
    wrap(<BpeTicketForm variant="normal"/>);
    expect(await screen.findByText('Selecione a linha…')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Emitir BP-e'})).toBeEnabled();
    expect(screen.queryByText('Imposto (ICMS)')).toBeNull();
    expect(screen.queryByLabelText(/Código da linha/)).toBeNull();
    expect(screen.getByLabelText('Poltrona')).toBeInTheDocument();
  });
});
