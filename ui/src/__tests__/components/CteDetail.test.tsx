import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {InternalAxiosRequestConfig} from 'axios';
import CteDetailPage from '@/app/cte/detail/page';
import {CteEmitForm} from '@/components/cte/CteEmitForm';
import {CteIbsCbsFields} from '@/components/cte/CteIbsCbsFields';
import {ApiError, apiClient} from '@/lib/api/client';
import {mockAdapter} from '@/lib/mock/handler';
import {cteDetailFixture, cteEventsFixture, cteSefazStatusFixture, ctesFixture, nfesFixture} from '@/lib/mock/fixtures';
import {emptyIbsCbs} from '@/lib/utils/cte';
import type {CteDeriveOut, NfeListOut} from '@/lib/types/api';

const ORG = '11222333000181';
let currentKey = '';
vi.mock('next/navigation', () => ({
  useRouter: () => ({push: vi.fn()}),
  useSearchParams: () => new URLSearchParams({key: currentKey}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG}})}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => <>{children}</>}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});
const [AUTH_TAKER, , , CANCELLED, DIVERGENT, OTHER_TAKER] = ctesFixture;

function mockCte(sk: string) {
  vi.spyOn(apiClient, 'getCte').mockResolvedValue(cteDetailFixture(sk) as never);
  vi.spyOn(apiClient, 'getCteEvents').mockResolvedValue(page(cteEventsFixture(sk)) as never);
}

function renderDetail(sk: string) {
  currentKey = sk;
  mockCte(sk);
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><CteDetailPage/></QueryClientProvider>);
}

beforeEach(() => vi.restoreAllMocks());

describe('detalhe do CT-e', () => {
  it('mostra cabeçalho, partes, frete, CFOP e eventos; com Baixar DACTE e sem XML do documento', async () => {
    renderDetail(AUTH_TAKER.sk);
    expect(await screen.findByText(/CT-e 31/)).toBeInTheDocument();
    expect(screen.getByText((_, el) => el?.tagName === 'P' && el.textContent === 'CFOP 6353 · ICMS CST SN')).toBeInTheDocument();
    expect(screen.getByText('Tomador (Remetente)')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Baixar DACTE'})).toBeInTheDocument();
    expect(await screen.findByText('Carta de Correção')).toBeInTheDocument();
    expect(screen.getByText('Prestação de serviço em desacordo')).toBeInTheDocument();
    expect(screen.getAllByRole('button', {name: /Baixar XML do evento/})).toHaveLength(4);
  });

  it('autorizado e tomador: oferece as quatro ações', async () => {
    renderDetail(AUTH_TAKER.sk);
    await screen.findByText(/CT-e 31/);
    for (const name of ['Carta de correção', 'Cancelar', 'Prestação em desacordo']) {
      expect(screen.getByRole('button', {name})).toBeInTheDocument();
    }
    expect(await screen.findByRole('button', {name: 'Cancelar desacordo'})).toBeInTheDocument();
  });

  it('autorizado e não tomador: mostra "apenas o tomador" no lugar do desacordo', async () => {
    renderDetail(OTHER_TAKER.sk);
    await screen.findByText(/CT-e 34/);
    expect(screen.getByText('apenas o tomador')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Prestação em desacordo'})).toBeNull();
    expect(screen.getByRole('button', {name: 'Cancelar'})).toBeInTheDocument();
  });

  it('cancelado: nenhuma ação', async () => {
    renderDetail(CANCELLED.sk);
    await screen.findByText(/CT-e 33/);
    expect(screen.queryByRole('region', {name: 'Ações do CT-e'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Cancelar'})).toBeNull();
  });

  it('DACTE: baixa autorizado e pendente offline; cancelado não oferece', async () => {
    const spy = vi.spyOn(apiClient, 'downloadCteDacte').mockResolvedValue({url: 'data:application/pdf;base64,', expires_at: '', filename: 'k.pdf', content_type: 'application/pdf'});
    const {unmount} = renderDetail(ctesFixture[6].sk);
    fireEvent.click(await screen.findByRole('button', {name: 'Baixar DACTE'}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith(ctesFixture[6].sk));
    unmount();
    renderDetail(CANCELLED.sk);
    await screen.findByText(/CT-e 33/);
    expect(screen.queryByRole('button', {name: 'Baixar DACTE'})).toBeNull();
  });

  it('Consultar na SEFAZ avisa a divergência sem corrigir', async () => {
    renderDetail(DIVERGENT.sk);
    vi.spyOn(apiClient, 'getCteSefazStatus').mockResolvedValue(cteSefazStatusFixture(DIVERGENT.sk));
    fireEvent.click(await screen.findByRole('button', {name: 'Consultar na SEFAZ'}));
    expect(await screen.findByRole('status')).toHaveTextContent(/A SEFAZ diz Cancelado, e aqui o CT-e está/);
    expect(screen.getByText('Cancelamento')).toBeInTheDocument();
  });

  it('chave que não é de CT-e não consulta a api', () => {
    currentKey = nfesFixture[0].sk;
    const spy = vi.spyOn(apiClient, 'getCte');
    render(<QueryClientProvider client={new QueryClient()}><CteDetailPage/></QueryClientProvider>);
    expect(screen.getByRole('alert')).toHaveTextContent(/não é de um CT-e válido/);
    expect(spy).not.toHaveBeenCalled();
  });
});

describe('ações do CT-e', () => {
  it('cancelar exige 15 caracteres, mostra o contador e envia a justificativa', async () => {
    renderDetail(AUTH_TAKER.sk);
    const spy = vi.spyOn(apiClient, 'cancelCte').mockResolvedValue({} as never);
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar'}));
    const dialog = await screen.findByRole('dialog');
    const submit = within(dialog).getByRole('button', {name: 'Confirmar cancelamento'});
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Justificativa'), {target: {value: 'Valor do frete errado'}});
    expect(within(dialog).getByText('21/255')).toBeInTheDocument();
    fireEvent.click(submit);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH_TAKER.sk, 'Valor do frete errado'));
  });

  it('traduz o 422 not_cte_issuer dentro do modal', async () => {
    renderDetail(AUTH_TAKER.sk);
    vi.spyOn(apiClient, 'cancelCte').mockRejectedValue(new ApiError(422, 'x', {code: 'not_cte_issuer'}));
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar'}));
    const dialog = await screen.findByRole('dialog');
    fireEvent.change(within(dialog).getByLabelText('Justificativa'), {target: {value: 'Valor do frete errado'}});
    fireEvent.click(within(dialog).getByRole('button', {name: 'Confirmar cancelamento'}));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(/Só o emitente do CT-e/);
  });

  it('carta de correção: grupo e campo são escolhas; grupo repetido pede o item', async () => {
    renderDetail(AUTH_TAKER.sk);
    const spy = vi.spyOn(apiClient, 'correctCte').mockResolvedValue({} as never);
    fireEvent.click(await screen.findByRole('button', {name: 'Carta de correção'}));
    const dialog = await screen.findByRole('dialog');
    const send = within(dialog).getByRole('button', {name: 'Enviar carta de correção'});
    expect(send).toBeDisabled();
    expect(within(dialog).getByLabelText('Campo da correção 1')).toBeDisabled();

    await userEvent.click(within(dialog).getByLabelText('Grupo da correção 1'));
    await userEvent.click(await screen.findByRole('option', {name: /ObsCont/}));
    await userEvent.click(within(dialog).getByLabelText('Campo da correção 1'));
    await userEvent.click(await screen.findByRole('option', {name: 'Texto'}));
    fireEvent.change(within(dialog).getByLabelText('Novo valor da correção 1'), {target: {value: 'Entrega no portão 2'}});
    expect(send).toBeDisabled(); // falta o número do item
    fireEvent.change(within(dialog).getByLabelText('Número do item da correção 1'), {target: {value: '1'}});
    expect(send).toBeEnabled();
    fireEvent.click(send);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(
      AUTH_TAKER.sk, [{group: 'ObsCont', field: 'xTexto', value: 'Entrega no portão 2', item_number: 1}], undefined));
  });

  it('prestação em desacordo: observações obrigatórias, de 15 a 255 caracteres', async () => {
    renderDetail(AUTH_TAKER.sk);
    const spy = vi.spyOn(apiClient, 'disagreeCteService').mockResolvedValue({} as never);
    fireEvent.click(await screen.findByRole('button', {name: 'Prestação em desacordo'}));
    const dialog = await screen.findByRole('dialog');
    const submit = within(dialog).getByRole('button', {name: 'Registrar desacordo'});
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Observações'), {target: {value: 'curto'}});
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Observações'), {target: {value: 'Carga chegou avariada ao destino'}});
    fireEvent.click(submit);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH_TAKER.sk, 'Carga chegou avariada ao destino', undefined));
  });

  it('cancelar desacordo escolhe o 610110 com sucesso', async () => {
    renderDetail(AUTH_TAKER.sk);
    const spy = vi.spyOn(apiClient, 'cancelCteServiceDisagreement').mockResolvedValue({} as never);
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar desacordo'}));
    const dialog = await screen.findByRole('dialog');
    const submit = within(dialog).getByRole('button', {name: 'Cancelar desacordo'});
    expect(submit).toBeDisabled();
    await userEvent.click(within(dialog).getByLabelText('Desacordo a cancelar'));
    await userEvent.click(await screen.findByRole('option', {name: /Prestação de serviço em desacordo/}));
    fireEvent.click(submit);
    const evt = cteEventsFixture(AUTH_TAKER.sk).find((e) => e.event_type === '610110')!;
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH_TAKER.sk, evt.sk, undefined));
  });
});

describe('IBS/CBS na emissão', () => {
  function renderForm(crt: number) {
    vi.spyOn(apiClient, 'getCTeConfig').mockResolvedValue({environment: 2} as never);
    vi.spyOn(apiClient, 'getOrganization').mockResolvedValue({pk: ORG, person: {crt}} as never);
    vi.spyOn(apiClient, 'getNfes').mockResolvedValue(page(nfesFixture as NfeListOut[]) as never);
    vi.spyOn(apiClient, 'deriveCte').mockImplementation(async (keys) =>
      (await mockAdapter({method: 'post', url: '/v1.0/ctes/derive', headers: {}, data: JSON.stringify({nfe_keys: keys})} as unknown as InternalAxiosRequestConfig)).data as CteDeriveOut);
    return render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}><CteEmitForm/></QueryClientProvider>);
  }

  it('Simples Nacional e MEI nunca veem o bloco; autXML aparece para todos', async () => {
    renderForm(1);
    await screen.findByText('Autorizados a baixar o XML (opcional)');
    expect(screen.queryByText('IBS e CBS (opcional)')).toBeNull();
  });

  it('regime normal vê o bloco recolhido', async () => {
    renderForm(3);
    expect(await screen.findByText('IBS e CBS (opcional)')).toBeInTheDocument();
    expect(screen.getByTestId('cte-ibscbs-summary')).toHaveTextContent('Não informado');
  });

  it('prévia usa o arredondamento bancário do servidor e o vTotDFe só soma imposto em 2027', () => {
    const onChange = vi.fn();
    const value = {...emptyIbsCbs(), base_value: '1850.00', ibs_uf_rate: '0.1000', ibs_mun_rate: '0.0500', cbs_rate: '0.9000'};
    const {unmount} = render(<CteIbsCbsFields value={value} onChange={onChange} freightTotal="1850.00" year={2026}/>);
    expect(screen.getByTestId('cte-ibs-total')).toHaveTextContent('2,77');
    expect(screen.getByTestId('cte-cbs-total')).toHaveTextContent('16,65');
    expect(screen.getByTestId('cte-vtotdfe')).toHaveTextContent('1.850,00');
    unmount();
    render(<CteIbsCbsFields value={value} onChange={onChange} freightTotal="1850.00" year={2027}/>);
    expect(screen.getByTestId('cte-vtotdfe')).toHaveTextContent('1.869,42');
  });

  it('o interruptor liga e desliga o grupo', () => {
    const onChange = vi.fn();
    render(<CteIbsCbsFields value={null} onChange={onChange} freightTotal=""/>);
    fireEvent.click(screen.getByRole('checkbox', {name: /Informar IBS e CBS/}));
    expect(onChange).toHaveBeenCalledWith(emptyIbsCbs());
  });
});
