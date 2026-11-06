import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {InternalAxiosRequestConfig} from 'axios';
import {CteEmitForm} from '@/components/cte/CteEmitForm';
import {apiClient} from '@/lib/api/client';
import {mockAdapter} from '@/lib/mock/handler';
import {nfesFixture, receivedNfesFixture} from '@/lib/mock/fixtures';
import type {CteDeriveOut, NfeListOut} from '@/lib/types/api';

const push = vi.fn();
vi.mock('next/navigation', () => ({useRouter: () => ({push})}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: '11222333000181'}})}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});

beforeEach(() => {
  vi.restoreAllMocks();
  push.mockReset();
  vi.spyOn(apiClient, 'getCTeConfig').mockResolvedValue({environment: 2} as never);
  vi.spyOn(apiClient, 'getOrganization').mockResolvedValue({pk: '11222333000181', person: {crt: 1}} as never);
  vi.spyOn(apiClient, 'getNfes').mockImplementation(async (params) =>
    page((params?.incoming === 1 ? receivedNfesFixture : nfesFixture) as NfeListOut[]));
  // O derive passa pelo mock da API: as regras da spec já estão lá e testadas.
  vi.spyOn(apiClient, 'deriveCte').mockImplementation(async (keys) => {
    const res = await mockAdapter({
      method: 'post', url: '/v1.0/ctes/derive', headers: {}, data: JSON.stringify({nfe_keys: keys}),
    } as unknown as InternalAxiosRequestConfig);
    return res.data as CteDeriveOut;
  });
  vi.spyOn(apiClient, 'emitCte').mockResolvedValue({sk: 'X', status: 'pending'} as never);
});

function renderForm() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><CteEmitForm/></QueryClientProvider>);
}

async function addNote(number: number) {
  fireEvent.click(await screen.findByText('Adicionar nota autorizada'));
  fireEvent.click(await screen.findByRole('option', {name: new RegExp(`Nº ${number} ·`)}));
}

function typeMoney(label: string, value: string) {
  const input = screen.getByLabelText(label);
  fireEvent.focus(input);
  fireEvent.change(input, {target: {value}});
  fireEvent.blur(input);
}

describe('CteEmitForm', () => {
  it('mostra o derivado: resumo, tomador sugerido e o que falta', async () => {
    renderForm();
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Escolha ao menos uma nota');

    await addNote(1001);
    await addNote(1002);

    expect(await screen.findByTestId('cte-summary')).toHaveTextContent(/^2 notas · 338,5 kg · R\$\s3\.769,60 · SP para MG$/);
    expect(screen.getByText(/Sugerido · Frete CIF na nota/)).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /Remetente/})).toBeChecked();
    expect(screen.getByTestId('cte-icms-summary')).toHaveTextContent('Simples Nacional');
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Informe o valor do frete');
    expect(screen.getByRole('button', {name: 'Emitir CT-e'})).toBeDisabled();
  });

  it('notas de remetentes diferentes abrem Dados do transporte com o aviso', async () => {
    renderForm();
    await addNote(1001);
    await addNote(4821);

    const transport = screen.getByRole('region', {name: /Dados do transporte/});
    expect(await within(transport).findByText('Notas com remetentes diferentes.')).toBeInTheDocument();
    expect(within(transport).getByText(/Emita um CT-e por remetente/)).toBeInTheDocument();
    expect(screen.getByRole('radio', {name: /Remetente/})).toBeDisabled();
    expect(screen.getByText('As notas não indicam um único responsável pelo frete.')).toBeInTheDocument();
  });

  it('edição marca "editado", e voltar ao derivado desfaz', async () => {
    renderForm();
    await addNote(1001);
    await screen.findByText(/Sugerido · Frete CIF/);

    fireEvent.click(screen.getByRole('button', {name: /Dados do transporte/}));
    fireEvent.click(screen.getByRole('button', {name: 'Editar produto predominante'}));
    fireEvent.change(screen.getByLabelText('Produto predominante'), {target: {value: 'Eletrônicos'}});

    const row = document.querySelector('[data-field="predominant_product"]') as HTMLElement;
    expect(within(row).getByText('editado')).toBeInTheDocument();
    expect(within(row).getByText('Derivado: Notebook 15 polegadas')).toBeInTheDocument();
    expect(screen.getByText(/1 campo editado/)).toBeInTheDocument();

    fireEvent.click(within(row).getByRole('button', {name: 'Voltar ao derivado'}));
    expect(within(row).queryByText('editado')).not.toBeInTheDocument();
    expect(within(row).getByText('Notebook 15 polegadas')).toBeInTheDocument();
  });

  it('emite com o tomador sugerido, o frete e os overrides', async () => {
    renderForm();
    await addNote(4821);
    await addNote(4822);
    await screen.findByText(/Sugerido · Frete FOB na nota/);

    typeMoney('Valor total do frete', '620');
    fireEvent.click(screen.getByText('Escolha o CFOP'));
    fireEvent.click(await screen.findByRole('option', {name: /^6353 ·/}));
    fireEvent.click(screen.getByRole('button', {name: /Dados do transporte/}));
    fireEvent.click(screen.getByRole('button', {name: 'Editar peso da carga'}));
    fireEvent.change(screen.getByLabelText('Peso da carga em kg'), {target: {value: '1100'}});

    const emit = screen.getByRole('button', {name: 'Emitir CT-e'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));

    await waitFor(() => expect(apiClient.emitCte).toHaveBeenCalledTimes(1));
    expect(apiClient.emitCte).toHaveBeenCalledWith({
      nfe_keys: [receivedNfesFixture[0].sk, receivedNfesFixture[1].sk],
      cfop: '6353',
      taker: {role: 'destinatario'},
      freight: {total: '620.00'},
      icms: {cst: 'SN'},
      overrides: {cargo: {total_weight_kg: '1100'}},
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith('/cte'));
  });
});
