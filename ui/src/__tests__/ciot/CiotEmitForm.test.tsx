import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {CiotEmitForm} from '@/components/ciot/CiotEmitForm';
import {ApiError, apiClient} from '@/lib/api/client';
import {CIOT_CARRIER_DOC, ciotsFixture, vehiclesFixture} from '@/lib/mock/fixtures';
import type {CiotOut} from '@/lib/types/api';

const push = vi.fn();
vi.mock('next/navigation', () => ({useRouter: () => ({push})}));
vi.mock('sonner', () => ({toast: {success: vi.fn(), error: vi.fn(), info: vi.fn()}}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'org', name: 'Empresa Mock', tax_id: '11222333000181', permissions: ['*']}}),
}));
// O combobox de NCM busca num Web Worker, que o jsdom não tem.
vi.mock('@/components/ui/ncm-combobox', () => ({NcmCombobox: ({id}: { id?: string }) => <input id={id} readOnly/>}));
vi.mock('@/lib/hooks/useCiotEnvironment', () => ({useCiotEnvironment: () => ['hom', vi.fn()]}));

const wrap = (ui: React.ReactNode) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{ui}</QueryClientProvider>);

const page = <T,>(items: T[]) => ({items, next_cursor: null, has_next: false, previous_cursor: null, has_previous: false});
// Fracionada declarada: carga lotação não retifica; é o caso em que a tela vem preenchida e válida.
const BASE = ciotsFixture[0] as unknown as CiotOut;
const DECLARED: CiotOut = {...BASE, operation_type: 2, declaration: {...BASE.declaration, operation_type: 2}};

beforeEach(() => {
  vi.restoreAllMocks();
  push.mockReset();
  vi.spyOn(apiClient, 'getPersons').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getVehicles').mockResolvedValue(page(vehiclesFixture) as never);
  vi.spyOn(apiClient, 'getMdfes').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getFreightFloor').mockRejectedValue(new ApiError(503, 'x', {code: 'ciot_floor_unverified'}));
});

describe('CiotEmitForm: declarar', () => {
  it('formulário vazio mostra os erros por campo e não chama a api', async () => {
    const declare = vi.spyOn(apiClient, 'declareCiot');
    wrap(<CiotEmitForm/>);
    fireEvent.click(await screen.findByRole('button', {name: 'Declarar CIOT'}));
    expect(await screen.findByText('Escolha ao menos um veículo')).toBeInTheDocument();
    expect(screen.getAllByText('CPF ou CNPJ inválido').length).toBeGreaterThan(1);
    expect(declare).not.toHaveBeenCalled();
  });

  it('lista os veículos do cadastro e limita a escolha', async () => {
    wrap(<CiotEmitForm/>);
    const box = await screen.findByLabelText('Usar o veículo ABC1D23');
    fireEvent.click(box);
    expect(screen.getByLabelText('Eixos de ABC1D23')).toBeInTheDocument();
    fireEvent.click(box);
    expect(screen.queryByLabelText('Eixos de ABC1D23')).toBeNull();
  });

  it('piso indisponível não impede o formulário de existir', async () => {
    wrap(<CiotEmitForm/>);
    expect(await screen.findByText(/Piso mínimo de frete: informe tipo de carga/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Declarar CIOT'})).toBeEnabled();
  });

  it('o vínculo com o MDF-e só oferece manifestos autorizados', async () => {
    vi.spyOn(apiClient, 'getMdfes').mockResolvedValue(page([
      {sk: 'K1', number: 7, uf_start: 'SP', uf_end: 'RJ', status: 'authorized'},
      {sk: 'K2', number: 8, uf_start: 'SP', uf_end: 'MG', status: 'closed'},
    ]) as never);
    wrap(<CiotEmitForm/>);
    expect(await screen.findByText('MDF-e da viagem (opcional)')).toBeInTheDocument();
  });
});

describe('CiotEmitForm: retificar', () => {
  it('vem preenchido com a declaração e não mostra o vínculo com o MDF-e', async () => {
    wrap(<CiotEmitForm rectifyOf={DECLARED}/>);
    expect(await screen.findByRole('button', {name: 'Retificar CIOT'})).toBeInTheDocument();
    expect(screen.getByText('Natureza atual: 1001. Escolha outro NCM só para trocar.')).toBeInTheDocument();
    expect(screen.queryByText('MDF-e da viagem (opcional)')).toBeNull();
  });

  it('pede confirmação e envia o conteúdo sem ambiente', async () => {
    const rectify = vi.spyOn(apiClient, 'rectifyCiot').mockResolvedValue({...DECLARED, pending_operation: 'rectify'});
    wrap(<CiotEmitForm rectifyOf={DECLARED}/>);
    fireEvent.click(await screen.findByRole('button', {name: 'Retificar CIOT'}));
    expect(await screen.findByText('Retificar o CIOT?')).toBeInTheDocument();
    expect(rectify).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', {name: 'Confirmar retificação'}));
    await waitFor(() => expect(rectify).toHaveBeenCalledTimes(1));
    const [id, body] = rectify.mock.calls[0];
    expect(id).toBe(DECLARED.id);
    expect(body).toMatchObject({operation_type: 2, carrier_doc: CIOT_CARRIER_DOC, freight_value: '4800.00'});
    expect('environment' in body).toBe(false);
    await waitFor(() => expect(push).toHaveBeenCalledWith(`/ciot/detail?id=${DECLARED.id}`));
  });

  it('422 ciot_validation mostra uma linha por regra recusada', async () => {
    vi.spyOn(apiClient, 'rectifyCiot').mockRejectedValue(new ApiError(422, 'A declaração não passou nas regras locais.', {
      code: 'ciot_validation',
      errors: [{message: 'O valor do frete deve ser maior que zero.', params: {rule: 'B120', antt_code: 319}}],
    }));
    wrap(<CiotEmitForm rectifyOf={DECLARED}/>);
    fireEvent.click(await screen.findByRole('button', {name: 'Retificar CIOT'}));
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar retificação'}));
    expect(await screen.findByText('O valor do frete deve ser maior que zero. (B120, ANTT 319)')).toBeInTheDocument();
    expect(push).not.toHaveBeenCalled();
  });
});
