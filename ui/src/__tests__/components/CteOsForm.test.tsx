import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {CteOsForm} from '@/components/cte/CteOsForm';
import {ApiError, apiClient} from '@/lib/api/client';
import type {PersonItemOut} from '@/lib/types/api';

const ORG = '11222333000181';
const push = vi.fn();
vi.mock('next/navigation', () => ({useRouter: () => ({push})}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG, permissions: ['*']}})}));
vi.mock('@/components/persons/PersonPicker', () => ({
  PersonPicker: ({onChange, value}: { onChange: (p: PersonItemOut) => void; value: PersonItemOut | null }) => (
    value ? <span>{value.name}</span>
      : <button onClick={() => onChange({sk: 'pessoa-1', cpf_or_cnpj: '12345678909', name: 'Cliente PF'} as PersonItemOut)}>Escolher pessoa</button>
  ),
}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});

beforeEach(() => {
  vi.restoreAllMocks();
  push.mockReset();
  vi.spyOn(apiClient, 'getCTeConfig').mockResolvedValue({environment: 2} as never);
  vi.spyOn(apiClient, 'getOrganization').mockResolvedValue({pk: ORG, person: {crt: 3}} as never);
  vi.spyOn(apiClient, 'getVehicles').mockResolvedValue(page([{sk: 'v1', plate: 'ABC1D23', plate_uf: 'SP', renavam: '12345678901'}]) as never);
  vi.spyOn(apiClient, 'emitCteOs').mockResolvedValue({sk: 'X', status: 'pending'} as never);
});

function renderForm() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><CteOsForm/></QueryClientProvider>);
}

function typeMoney(label: string, value: string) {
  const input = screen.getByLabelText(label);
  fireEvent.focus(input);
  fireEvent.change(input, {target: {value}});
  fireEvent.blur(input);
}

async function pickCity(id: string, search: string, label: string) {
  fireEvent.click(document.getElementById(id) as HTMLElement);
  fireEvent.change(await screen.findByLabelText('Buscar município...'), {target: {value: search}});
  const options = await screen.findAllByRole('option');
  fireEvent.click(options.find((o) => o.textContent === label) as HTMLElement);
}

/** Pessoas, SP para MG, TAF, fretamento contínuo, R$ 2.400,00, ICMS 00 a 12%. */
async function fillPeople() {
  fireEvent.click(screen.getByRole('radio', {name: /Transporte de pessoas/}));
  fireEvent.click(within(screen.getByRole('region', {name: 'Quem paga o serviço?'})).getByRole('button', {name: 'Escolher pessoa'}));
  fireEvent.change(screen.getByLabelText('Descrição do serviço'), {target: {value: 'Fretamento turístico'}});
  await pickCity('cte-os-origin', 'São Paulo', 'São Paulo / SP');
  await pickCity('cte-os-destination', 'Belo Horizonte', 'Belo Horizonte / MG');
  fireEvent.change(screen.getByLabelText(/^TAF/), {target: {value: '123456789012'}});
  await userEvent.click(document.getElementById('cte-os-charter') as HTMLElement);
  await userEvent.click(await screen.findByRole('option', {name: 'Contínuo'}));
  typeMoney('Valor total do serviço', '2400');
  fireEvent.click(screen.getByText('Escolha o CFOP'));
  fireEvent.click(await screen.findByRole('option', {name: /^6353 ·/}));
  fireEvent.click(screen.getByText('Imposto'));
  await userEvent.click(document.getElementById('cte-icms-cst') as HTMLElement);
  await userEvent.click(await screen.findByRole('option', {name: /00 · Tributação normal/}));
  fireEvent.change(document.getElementById('cte-icms-rate') as HTMLElement, {target: {value: '12'}});
}

describe('CT-e OS', () => {
  it('antes de escolher o tipo só aparece o tipo, e o que falta é o tipo', () => {
    renderForm();
    expect(screen.getAllByRole('radio')).toHaveLength(3);
    expect(screen.queryByText('Quem paga o serviço?')).toBeNull();
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Escolha o tipo de serviço');
    expect(screen.getByRole('button', {name: 'Emitir CT-e OS'})).toBeDisabled();
  });

  it('pessoas entre UFs: o formulário pede o TAF e o fretamento, e emite o corpo certo', async () => {
    renderForm();
    await fillPeople();
    expect(screen.getByText('Entre UFs, o transporte de pessoas exige o TAF.')).toBeInTheDocument();
    const emit = screen.getByRole('button', {name: 'Emitir CT-e OS'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));

    await waitFor(() => expect(apiClient.emitCteOs).toHaveBeenCalledTimes(1));
    const body = vi.mocked(apiClient.emitCteOs).mock.calls[0][0];
    expect(body).toMatchObject({
      service_type: 'people', taker: {person_id: 'pessoa-1'}, service: {description: 'Fretamento turístico'},
      origin: {uf: 'SP'}, destination: {uf: 'MG'}, road: {taf: '123456789012', charter: {type: 2}},
      freight: {total: '2400.00'}, icms: {cst: '00', p_icms: '12.00'},
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith('/cte'));
  });

  it('um 422 com code aparece em pt-BR e não sai da tela', async () => {
    vi.spyOn(apiClient, 'emitCteOs').mockRejectedValue(new ApiError(422, 'cru', {code: 'os_trip_at_past'}));
    renderForm();
    await fillPeople();
    const emit = screen.getByRole('button', {name: 'Emitir CT-e OS'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('depois do momento da emissão');
    expect(push).not.toHaveBeenCalled();
  });

  it('transporte de valores: soma das GTV-e confere com o frete antes de emitir', async () => {
    renderForm();
    fireEvent.click(screen.getByRole('radio', {name: /Transporte de valores/}));
    expect(screen.getByText('Quais GTV-e estão neste transporte?')).toBeInTheDocument();
    expect(screen.queryByText('Transporte rodoviário')).toBeNull();
    fireEvent.click(screen.getByRole('button', {name: 'Adicionar GTV-e'}));
    fireEvent.change(screen.getByLabelText('Chave da GTV-e 1'), {target: {value: '5'.repeat(44)}});
    await userEvent.click(screen.getByLabelText('Tipo do componente 1 da GTV-e 1'));
    await userEvent.click(await screen.findByRole('option', {name: 'Malote'}));
    typeMoney('Valor do componente 1 da GTV-e 1', '100');
    typeMoney('Valor total do serviço', '150');
    expect(screen.getByTestId('cte-os-gtve-total')).toHaveTextContent('Soma das GTV-e: R$ 100,00 (precisa fechar com o frete)');
    typeMoney('Valor total do serviço', '100');
    expect(screen.getByTestId('cte-os-gtve-total')).not.toHaveTextContent('precisa fechar');
  });

  it('excesso de bagagem pede ao menos um documento referenciado', () => {
    renderForm();
    fireEvent.click(screen.getByRole('radio', {name: /Excesso de bagagem/}));
    expect(screen.getByText('Qual bilhete tem excesso de bagagem?')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Adicionar documento'}));
    expect(within(screen.getByRole('list', {name: 'Documentos referenciados'})).getAllByRole('listitem')).toHaveLength(1);
    expect(screen.getByLabelText('Chave do BP-e 1')).toBeInTheDocument();
  });

  it('o ICMS do CT-e OS não oferece o CST 60', async () => {
    renderForm();
    fireEvent.click(screen.getByRole('radio', {name: /Transporte de valores/}));
    fireEvent.click(screen.getByText('Imposto'));
    fireEvent.click(document.getElementById('cte-icms-cst') as HTMLElement);
    expect(await screen.findByRole('option', {name: /00 · Tributação normal/})).toBeInTheDocument();
    expect(screen.queryByRole('option', {name: /60 ·/})).toBeNull();
  });
});
