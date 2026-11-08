import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {CteChangeForm} from '@/components/cte/CteChangeForm';
import {CteOriginalGate} from '@/components/cte/CteOriginalGate';
import {ApiError, apiClient} from '@/lib/api/client';
import {cteDetailFixture, ctesFixture} from '@/lib/mock/fixtures';
import type {CteDetailOut, PersonItemOut} from '@/lib/types/api';

const ORG = '11222333000181';
const push = vi.fn();
let currentKey = '';
vi.mock('next/navigation', () => ({
  useRouter: () => ({push}),
  useSearchParams: () => new URLSearchParams({key: currentKey}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG, permissions: ['*']}})}));
vi.mock('@/components/persons/PersonPicker', () => ({
  PersonPicker: ({onChange, value}: { onChange: (p: PersonItemOut) => void; value: PersonItemOut | null }) => (
    value ? <span>{value.name}</span> : <button onClick={() => onChange({sk: 'pessoa-1', name: 'Terceiro SA'} as PersonItemOut)}>Escolher pessoa</button>
  ),
}));

const original = () => cteDetailFixture(ctesFixture[0].sk) as unknown as CteDetailOut;

beforeEach(() => {
  vi.restoreAllMocks();
  push.mockReset();
  vi.spyOn(apiClient, 'getCTeConfig').mockResolvedValue({environment: 2} as never);
  vi.spyOn(apiClient, 'getOrganization').mockResolvedValue({pk: ORG, person: {crt: 1}} as never);
  vi.spyOn(apiClient, 'complementCte').mockResolvedValue({sk: 'X', status: 'pending'} as never);
  vi.spyOn(apiClient, 'substituteCte').mockResolvedValue({sk: 'X', status: 'pending'} as never);
});

function renderForm(mode: 'complement' | 'substitute', doc: CteDetailOut = original()) {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><CteChangeForm mode={mode} original={doc}/></QueryClientProvider>);
}

function typeMoney(label: string, value: string) {
  const input = screen.getByLabelText(label);
  fireEvent.focus(input);
  fireEvent.change(input, {target: {value}});
  fireEvent.blur(input);
}

async function chooseCfop() {
  fireEvent.click(screen.getByText('Escolha o CFOP'));
  fireEvent.click(await screen.findByRole('option', {name: /^6353 ·/}));
}

describe('complemento', () => {
  it('explica quando usar, pede só o valor complementar e envia a chave do original', async () => {
    renderForm('complement');
    expect(screen.getByText('Quando usar o complemento')).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Emitir substituto'})).toHaveAttribute('href', `/cte/substitute?key=${ctesFixture[0].sk}`);
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Informe o valor do valor complementar');

    typeMoney('Valor complementar', '150');
    await chooseCfop();
    const emit = screen.getByRole('button', {name: 'Emitir complemento'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));

    await waitFor(() => expect(apiClient.complementCte).toHaveBeenCalledTimes(1));
    expect(apiClient.complementCte).toHaveBeenCalledWith({
      original_keys: [ctesFixture[0].sk], cfop: '6353', freight: {total: '150.00'}, icms: {cst: 'SN'},
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith('/cte'));
  });

  it('um 422 com code vira a explicação em pt-BR', async () => {
    vi.spyOn(apiClient, 'complementCte').mockRejectedValue(new ApiError(422, 'texto cru', {code: 'original_too_old'}));
    renderForm('complement');
    typeMoney('Valor complementar', '10');
    await chooseCfop();
    const emit = screen.getByRole('button', {name: 'Emitir complemento'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('mais de 12 meses');
    expect(push).not.toHaveBeenCalled();
  });
});

describe('substituto', () => {
  it('sem trocar o tomador não envia change_taker; o novo valor vai no corpo', async () => {
    renderForm('substitute');
    expect(screen.getByText('Quando usar o substituto')).toBeInTheDocument();
    typeMoney('Valor total do frete', '1900');
    await chooseCfop();
    const emit = screen.getByRole('button', {name: 'Emitir substituto'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));
    await waitFor(() => expect(apiClient.substituteCte).toHaveBeenCalledTimes(1));
    expect(apiClient.substituteCte).toHaveBeenCalledWith({
      original_key: ctesFixture[0].sk, cfop: '6353', freight: {total: '1900.00'}, icms: {cst: 'SN'},
    });
  });

  it('trocar o tomador oferece o outro papel e exige a pessoa quando for outro', async () => {
    renderForm('substitute');
    fireEvent.click(screen.getByRole('checkbox', {name: /Informar outro tomador/}));
    // O tomador atual é o remetente: só destinatário e outro são oferecidos.
    expect(screen.queryByRole('radio', {name: /Remetente/})).toBeNull();
    expect(screen.getByRole('radio', {name: /Destinatário/})).toBeInTheDocument();
    typeMoney('Valor total do frete', '100');
    await chooseCfop();
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Escolha o novo tomador');

    fireEvent.click(screen.getByRole('radio', {name: /Outro/}));
    expect(screen.getByTestId('cte-missing')).toHaveTextContent('Escolha a pessoa que paga o frete');
    fireEvent.click(within(screen.getByRole('region', {name: 'Trocar o tomador?'})).getByRole('button', {name: 'Escolher pessoa'}));
    const emit = screen.getByRole('button', {name: 'Emitir substituto'});
    await waitFor(() => expect(emit).toBeEnabled());
    fireEvent.click(emit);
    fireEvent.click(await screen.findByRole('button', {name: 'Confirmar e emitir'}));
    await waitFor(() => expect(apiClient.substituteCte).toHaveBeenCalledTimes(1));
    expect(apiClient.substituteCte).toHaveBeenCalledWith(expect.objectContaining({
      change_taker: true, taker: {role: 'outro', person_id: 'pessoa-1'},
    }));
  });

  it('além de 67 dias o prazo acabou: avisa e não deixa emitir', () => {
    const old = {...original(), dh_emi: new Date(Date.now() - 80 * 86_400_000).toISOString()};
    renderForm('substitute', old);
    expect(screen.getByRole('alert')).toHaveTextContent('o prazo para substituir acabou');
    expect(screen.getByRole('button', {name: 'Emitir substituto'})).toBeDisabled();
  });

  it('entre 60 e 67 dias avisa que a SEFAZ decide, sem bloquear', () => {
    const old = {...original(), dh_emi: new Date(Date.now() - 63 * 86_400_000).toISOString()};
    renderForm('substitute', old);
    expect(screen.getByRole('status')).toHaveTextContent('a SEFAZ decide');
  });
});

describe('porta do CT-e original', () => {
  const renderGate = (sk: string, doc?: unknown) => {
    currentKey = sk;
    vi.spyOn(apiClient, 'getCte').mockResolvedValue((doc ?? cteDetailFixture(sk)) as never);
    const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
    return render(<QueryClientProvider client={client}><CteOriginalGate>{() => <p>formulário</p>}</CteOriginalGate></QueryClientProvider>);
  };

  it('libera o formulário para um CT-e autorizado do emitente', async () => {
    renderGate(ctesFixture[0].sk);
    expect(await screen.findByText('formulário')).toBeInTheDocument();
    expect(screen.getByText(/CT-e original: 31/)).toBeInTheDocument();
  });

  it('recusa um CT-e pendente, um complemento e um CT-e OS, dizendo por quê', async () => {
    const {unmount} = renderGate(ctesFixture[2].sk);
    expect(await screen.findByRole('alert')).toHaveTextContent('Só um CT-e autorizado');
    unmount();
    const complement = renderGate(ctesFixture[8].sk);
    expect(await screen.findByRole('alert')).toHaveTextContent('complementar não pode');
    complement.unmount();
    renderGate(ctesFixture[8].sk, cteDetailFixture(ctesFixture[10].sk));
    expect(await screen.findByRole('alert')).toHaveTextContent('CT-e OS');
  });

  it('chave que não é de CT-e 57 não consulta a api', () => {
    currentKey = '123';
    const spy = vi.spyOn(apiClient, 'getCte');
    const client = new QueryClient();
    render(<QueryClientProvider client={client}><CteOriginalGate>{() => <p>x</p>}</CteOriginalGate></QueryClientProvider>);
    expect(screen.getByRole('alert')).toHaveTextContent('não é de um CT-e válido');
    expect(spy).not.toHaveBeenCalled();
  });
});
