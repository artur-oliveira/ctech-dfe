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

const heading = () => screen.getByRole('heading', {level: 2});
const next = () => fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
const atStep = (label: string) => waitFor(() => expect(heading()).toHaveTextContent(label));

/** Preenche o valor e o CFOP e termina na revisão (o substituto passa antes pelo passo do tomador). */
async function valueToReview(total: string, noun = 'Valor total do frete') {
  await atStep('Valores');
  typeMoney(noun, total);
  await chooseCfop();
  next();
  await atStep('Revisão');
}

describe('complemento', () => {
  it('explica quando usar, pede só o valor complementar e envia a chave do original', async () => {
    renderForm('complement');
    expect(heading()).toHaveTextContent('Original');
    expect(screen.getByText('Quando usar o complemento')).toBeInTheDocument();
    expect(screen.getByText(/CT-e original: 31/)).toBeInTheDocument();
    expect(screen.getByRole('link', {name: 'Emitir substituto'})).toHaveAttribute('href', `/cte/substitute?key=${ctesFixture[0].sk}`);
    next();
    await atStep('Valores');
    next();
    expect(await screen.findByText('Informe o valor do valor complementar')).toBeInTheDocument();
    expect(heading()).toHaveTextContent('Valores');

    await valueToReview('150', 'Valor complementar');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir complemento'}));

    await waitFor(() => expect(apiClient.complementCte).toHaveBeenCalledTimes(1));
    expect(apiClient.complementCte).toHaveBeenCalledWith({
      original_keys: [ctesFixture[0].sk], cfop: '6353', freight: {total: '150.00'}, icms: {cst: 'SN'},
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith('/cte'));
  });

  it('um 422 com code sem passo dono vira a explicação em pt-BR na revisão', async () => {
    vi.spyOn(apiClient, 'complementCte').mockRejectedValue(new ApiError(422, 'texto cru', {code: 'original_too_old'}));
    renderForm('complement');
    next();
    await valueToReview('10', 'Valor complementar');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir complemento'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('mais de 12 meses');
    expect(heading()).toHaveTextContent('Revisão');
    expect(push).not.toHaveBeenCalled();
  });

  it('um 422 sobre o valor volta ao passo Valores', async () => {
    vi.spyOn(apiClient, 'complementCte').mockRejectedValue(new ApiError(422, 'cru', {code: 'components_exceed_total'}));
    renderForm('complement');
    next();
    await valueToReview('10', 'Valor complementar');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir complemento'}));
    await atStep('Valores');
    expect(await screen.findByRole('alert')).toHaveTextContent('Os componentes somam mais que o valor');
  });
});

describe('substituto', () => {
  it('sem trocar o tomador não envia change_taker; o novo valor vai no corpo', async () => {
    renderForm('substitute');
    expect(screen.getByText('Quando usar o substituto')).toBeInTheDocument();
    next();
    await atStep('Tomador');
    next();
    await valueToReview('1900');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir substituto'}));
    await waitFor(() => expect(apiClient.substituteCte).toHaveBeenCalledTimes(1));
    expect(apiClient.substituteCte).toHaveBeenCalledWith({
      original_key: ctesFixture[0].sk, cfop: '6353', freight: {total: '1900.00'}, icms: {cst: 'SN'},
    });
  });

  it('trocar o tomador oferece o outro papel e exige a pessoa quando for outro', async () => {
    renderForm('substitute');
    next();
    await atStep('Tomador');
    fireEvent.click(screen.getByRole('checkbox', {name: /Informar outro tomador/}));
    // O tomador atual é o remetente: só destinatário e outro são oferecidos.
    expect(screen.queryByRole('radio', {name: /Remetente/})).toBeNull();
    expect(screen.getByRole('radio', {name: /Destinatário/})).toBeInTheDocument();
    next();
    expect(await screen.findByText('Escolha o novo tomador')).toBeInTheDocument();
    expect(heading()).toHaveTextContent('Tomador');

    fireEvent.click(screen.getByRole('radio', {name: /Outro/}));
    expect(await screen.findByText('Escolha a pessoa que paga o frete')).toBeInTheDocument();
    next();
    expect(heading()).toHaveTextContent('Tomador');
    fireEvent.click(within(screen.getByRole('region', {name: 'Trocar o tomador?'})).getByRole('button', {name: 'Escolher pessoa'}));
    next();
    await valueToReview('100');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir substituto'}));
    await waitFor(() => expect(apiClient.substituteCte).toHaveBeenCalledTimes(1));
    expect(apiClient.substituteCte).toHaveBeenCalledWith(expect.objectContaining({
      change_taker: true, taker: {role: 'outro', person_id: 'pessoa-1'},
    }));
  });

  it('troca para expedidor ou recebedor do original: sem pessoa do cadastro, só o papel', async () => {
    renderForm('substitute');
    next();
    await atStep('Tomador');
    fireEvent.click(screen.getByRole('checkbox', {name: /Informar outro tomador/}));
    // Remetente é o atual; o original tem expedidor e recebedor (XML), então os quatro participantes e Outro aparecem.
    for (const r of [/Expedidor/, /Recebedor/, /Destinatário/, /Outro/]) expect(screen.getByRole('radio', {name: r})).toBeInTheDocument();
    fireEvent.click(screen.getByRole('radio', {name: /Expedidor/}));
    expect(screen.queryByText('Pessoa que paga o frete')).toBeNull();
    next();
    await valueToReview('100');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir substituto'}));
    await waitFor(() => expect(apiClient.substituteCte).toHaveBeenCalledTimes(1));
    expect(apiClient.substituteCte).toHaveBeenCalledWith(expect.objectContaining({change_taker: true, taker: {role: 'expedidor'}}));
  });

  it('um 422 do tomador volta ao passo Tomador', async () => {
    vi.spyOn(apiClient, 'substituteCte').mockRejectedValue(new ApiError(422, 'cru', {code: 'taker_unchanged'}));
    renderForm('substitute');
    next();
    await atStep('Tomador');
    next();
    await valueToReview('100');
    fireEvent.click(screen.getByRole('button', {name: 'Emitir substituto'}));
    await atStep('Tomador');
    expect(await screen.findByRole('alert')).toHaveTextContent('mesmo do CT-e original');
  });

  it('original sem expedidor nem recebedor não os oferece', async () => {
    renderForm('substitute', {...original(), expedidor: undefined, recebedor: undefined});
    next();
    await atStep('Tomador');
    fireEvent.click(screen.getByRole('checkbox', {name: /Informar outro tomador/}));
    expect(screen.queryByRole('radio', {name: /Expedidor/})).toBeNull();
    expect(screen.queryByRole('radio', {name: /Recebedor/})).toBeNull();
  });

  it('além de 67 dias o prazo acabou: avisa e não deixa passar do primeiro passo', async () => {
    const old = {...original(), dh_emi: new Date(Date.now() - 80 * 86_400_000).toISOString()};
    renderForm('substitute', old);
    expect(screen.getByRole('alert')).toHaveTextContent('o prazo para substituir acabou');
    next();
    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument());
    expect(heading()).toHaveTextContent('Original');
  });

  it('entre 60 e 67 dias avisa que a SEFAZ decide, sem bloquear', async () => {
    const old = {...original(), dh_emi: new Date(Date.now() - 63 * 86_400_000).toISOString()};
    renderForm('substitute', old);
    expect(screen.getByText(/a SEFAZ decide/)).toBeInTheDocument();
    next();
    await atStep('Tomador');
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
