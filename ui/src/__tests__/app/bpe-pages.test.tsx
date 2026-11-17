import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import BpesPage from '@/app/bpe/page';
import BpeEmitPage from '@/app/bpe/emit/page';
import {BpeEventModal} from '@/components/bpe/BpeEventModal';
import {apiClient} from '@/lib/api/client';
import type {BpeListOut} from '@/lib/types/api';

vi.mock('next/navigation', () => ({
  useRouter: () => ({push: vi.fn(), replace: vi.fn()}),
  useSearchParams: () => new URLSearchParams(''),
}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('@/components/dfe/RequireFiscalConfig', () => ({RequireFiscalConfig: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('@/components/bpe/BpeTicketForm', () => ({BpeTicketForm: ({variant}: { variant: string }) => <p>formulário do bilhete {variant}</p>}));
vi.mock('@/components/bpe/BpeTmForm', () => ({BpeTmForm: () => <p>formulário do TM</p>}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181', permissions: ['create.bpes']}})}));
vi.mock('@/lib/hooks/useFiscalConfig', () => ({useFiscalConfig: () => ({config: {environment: 2}, isMissing: false})}));

const KEY = '35261112345678000190630010000000101012345671';

function ticket(over: Partial<BpeListOut>): BpeListOut {
  return {
    pk: 'hom#CNPJ_11222333000181', sk: KEY, incoming: 0, year: 2026, month: 11, day: 5, status: 'authorized', sefaz_status: '100',
    sefaz_motive: null, emit_cpf_cnpj: '12345678000190', emit_name: 'VIACAO TESTE', number: 10, serie: 1, variant: 'normal',
    modal: 'rodoviario', total: '90.00', passenger_name: 'Maria Souza', from_name: 'SAO PAULO', to_name: 'RIO', dh_emi: null,
    created_at: '2026-11-05T10:00:00Z', ...over,
  } as BpeListOut;
}

function wrap(node: React.ReactNode) {
  return render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{node}</QueryClientProvider>);
}

beforeEach(() => {
  vi.spyOn(apiClient, 'getBpes').mockResolvedValue({
    items: [ticket({}), ticket({sk: KEY.replace(/1$/, '2'), number: 11, variant: 'tm', competence: '2026-11-01', passengers: '120', passenger_name: undefined})],
    next_cursor: null, has_next: false, previous_cursor: null, has_previous: false,
  });
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('lista de BP-e', () => {
  it('mostra o trecho do comum, a competência do TM e esconde o DABPE do TM', async () => {
    wrap(<BpesPage/>);
    expect(await screen.findByText('SAO PAULO para RIO')).toBeInTheDocument();
    expect(screen.getByText(/Competência 2026-11-01 · 120 passageiros/)).toBeInTheDocument();
    expect(screen.getAllByRole('button', {name: /DABPE/})).toHaveLength(1);
    expect(screen.getByRole('button', {name: /Emitir BP-e/})).toBeInTheDocument();
  });
});

describe('emissão de BP-e', () => {
  it('começa no comum e troca o formulário ao escolher o tipo', async () => {
    wrap(<BpeEmitPage/>);
    expect(screen.getByRole('heading', {level: 1, name: 'Emitir BP-e'})).toBeInTheDocument();
    expect(screen.getByText('formulário do bilhete normal')).toBeInTheDocument();
    expect(screen.queryByText(/exige credenciamento/)).not.toBeInTheDocument();
  });
});

describe('modal de evento', () => {
  const noop = () => undefined;

  function open(kind: Parameters<typeof BpeEventModal>[0]['kind'], onConfirm = vi.fn()) {
    render(<BpeEventModal kind={kind} docNumber={10} loading={false} onClose={noop} onConfirm={onConfirm}/>);
    return onConfirm;
  }

  it('não renderiza sem tipo', () => {
    open(null);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('poltrona só habilita com 1 a 3 dígitos e envia o valor', async () => {
    const onConfirm = open('seat-change');
    const submit = screen.getByRole('button', {name: 'Registrar'});
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Nova poltrona'), {target: {value: '12'}});
    await waitFor(() => expect(submit).toBeEnabled());
    fireEvent.click(submit);
    expect(onConfirm).toHaveBeenCalledWith('seat-change', expect.objectContaining({seat: '12'}));
  });

  it('excesso de bagagem exige quantidade e valor decimal', async () => {
    open('excess-baggage');
    const submit = screen.getByRole('button', {name: 'Registrar'});
    fireEvent.change(screen.getByLabelText(/Quantidade/), {target: {value: '2 volumes'}});
    fireEvent.change(screen.getByLabelText(/Valor total cobrado/), {target: {value: '12,5'}});
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/Valor total cobrado/), {target: {value: '12.50'}});
    await waitFor(() => expect(submit).toBeEnabled());
  });

  it('fornecimento não realizado pede IBS e CBS', async () => {
    open('supply-not-performed');
    const submit = screen.getByRole('button', {name: 'Registrar'});
    fireEvent.change(screen.getByLabelText(/IBS do serviço/), {target: {value: '1.00'}});
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/CBS do serviço/), {target: {value: '2.00'}});
    await waitFor(() => expect(submit).toBeEnabled());
  });

  it('não embarque exige justificativa de 15 caracteres', async () => {
    open('no-show');
    const submit = screen.getByRole('button', {name: 'Registrar'});
    expect(submit).toBeDisabled();
    fireEvent.change(screen.getByRole('textbox'), {target: {value: 'passageiro ausente no embarque'}});
    await waitFor(() => expect(submit).toBeEnabled());
  });
});
