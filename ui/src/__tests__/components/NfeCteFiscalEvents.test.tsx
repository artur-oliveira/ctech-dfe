import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CteDetailPage from '@/app/cte/detail/page';
import {useNfeFiscalEvents} from '@/components/nfe/NfeFiscalEvents';
import {apiClient} from '@/lib/api/client';
import {cteDetailFixture, cteEventsFixture, ctesFixture} from '@/lib/mock/fixtures';

const ORG = '11222333000181';
let currentKey = '';
let permissions: string[] = ['*'];
vi.mock('next/navigation', () => ({
  useRouter: () => ({push: vi.fn()}),
  useSearchParams: () => new URLSearchParams({key: currentKey}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG, permissions}})}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => <>{children}</>}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});
const wrap = (ui: React.ReactElement) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{ui}</QueryClientProvider>);

const KEY = '1'.repeat(44);
const receipt = {sk: 'r1', event_type: '110130', sequence_number: 1, status: 'success', sefaz_protocol: '135260000000001', created_at: '2026-07-05T16:40:00Z'};

function NfeHarness({can = true, incoming = false}: { can?: boolean; incoming?: boolean }) {
  const {menu, modals} = useNfeFiscalEvents(ORG);
  return <>{menu({sk: KEY, number: 9, incoming, receipts: [receipt] as never}, can)}{modals}</>;
}

beforeEach(() => {
  vi.restoreAllMocks();
  permissions = ['*'];
});

describe('menu Eventos fiscais da NF-e', () => {
  it('some sem permissão e agrupa entregas, transporte e reforma', async () => {
    const {unmount} = wrap(<NfeHarness can={false}/>);
    expect(screen.queryByRole('button', {name: 'Eventos fiscais'})).toBeNull();
    unmount();
    wrap(<NfeHarness/>);
    await userEvent.click(screen.getByRole('button', {name: 'Eventos fiscais'}));
    for (const n of ['Comprovante de entrega', 'Cancelar comprovante', 'Ator interessado', 'Pagamento integral', 'Vinculação de pagamento']) {
      expect(screen.getByRole('button', {name: new RegExp(n)})).toBeInTheDocument();
    }
    expect(screen.getByText(/16\/11\/2026/)).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /Aceite de débito/})).toBeNull();
  });

  it('112110: confirma o pagamento e envia só event_type e indicator', async () => {
    const spy = vi.spyOn(apiClient, 'requestNfeFiscalEvent').mockResolvedValue({} as never);
    wrap(<NfeHarness/>);
    await userEvent.click(screen.getByRole('button', {name: 'Eventos fiscais'}));
    await userEvent.click(screen.getByRole('button', {name: /Pagamento integral/}));
    const dialog = await screen.findByRole('dialog');
    const submit = within(dialog).getByRole('button', {name: 'Enviar evento'});
    expect(submit).toBeDisabled();
    await userEvent.click(await within(dialog).findByRole('checkbox'));
    expect(submit).toBeEnabled();
    fireEvent.click(submit);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(KEY, 'ibscbs-events', {event_type: '112110', indicator: 1}));
  });

  it('cancelar comprovante envia o protocolo do evento escolhido', async () => {
    const spy = vi.spyOn(apiClient, 'requestNfeFiscalEvent').mockResolvedValue({} as never);
    wrap(<NfeHarness/>);
    await userEvent.click(screen.getByRole('button', {name: 'Eventos fiscais'}));
    await userEvent.click(screen.getByRole('button', {name: /Cancelar comprovante/}));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByLabelText('Comprovante a cancelar'));
    await userEvent.click(await screen.findByRole('option', {name: /seq\. 1/}));
    fireEvent.click(within(dialog).getByRole('button', {name: 'Enviar evento'}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith(KEY, 'delivery-receipt-cancel', {delivery_protocol: '135260000000001'}));
  });

  it('mostra o 422 event_invalid em pt-BR dentro do modal', async () => {
    const {ApiError} = await import('@/lib/api/client');
    vi.spyOn(apiClient, 'requestNfeFiscalEvent').mockRejectedValue(
      new ApiError(422, 'detalhe da api', {code: 'event_invalid'}));
    wrap(<NfeHarness/>);
    await userEvent.click(screen.getByRole('button', {name: 'Eventos fiscais'}));
    await userEvent.click(screen.getByRole('button', {name: /Pagamento integral/}));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(await within(dialog).findByRole('checkbox'));
    fireEvent.click(within(dialog).getByRole('button', {name: 'Enviar evento'}));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(/não passam na validação/);
  });
});

describe('entrega no detalhe do CT-e', () => {
  const [AUTH] = ctesFixture;
  function renderCte() {
    currentKey = AUTH.sk;
    vi.spyOn(apiClient, 'getCte').mockResolvedValue(cteDetailFixture(AUTH.sk) as never);
    vi.spyOn(apiClient, 'getCteEvents').mockResolvedValue(page(cteEventsFixture(AUTH.sk)) as never);
    return wrap(<CteDetailPage/>);
  }

  it('sem permissão não oferece as ações de entrega', async () => {
    permissions = [];
    renderCte();
    await screen.findByRole('button', {name: 'Carta de correção'});
    expect(screen.queryByRole('button', {name: 'Insucesso de entrega'})).toBeNull();
  });

  it('insucesso: justificativa só no motivo 4 e corpo da rota', async () => {
    const spy = vi.spyOn(apiClient, 'registerCteDeliveryEvent').mockResolvedValue({} as never);
    renderCte();
    fireEvent.click(await screen.findByRole('button', {name: 'Insucesso de entrega'}));
    const dialog = await screen.findByRole('dialog');
    const submit = within(dialog).getByRole('button', {name: 'Registrar insucesso'});
    expect(within(dialog).queryByLabelText('Justificativa')).toBeNull();
    await userEvent.click(within(dialog).getByLabelText('Motivo'));
    await userEvent.click(await screen.findByRole('option', {name: 'Outro motivo'}));
    fireEvent.change(within(dialog).getByLabelText('Justificativa'), {target: {value: 'curta'}});
    fireEvent.change(within(dialog).getByLabelText('Data e hora da ocorrência'), {target: {value: '2026-11-08T10:00'}});
    fireEvent.change(within(dialog).getByLabelText('Hash do comprovante (SHA-1 em Base64)'), {target: {value: 'qUqP5cyxm6YcTAhz05Hph5gvu9M='}});
    fireEvent.change(within(dialog).getByLabelText('Data e hora do hash'), {target: {value: '2026-11-08T10:05'}});
    expect(submit).toBeDisabled();
    fireEvent.change(within(dialog).getByLabelText('Justificativa'), {target: {value: 'x'.repeat(25)}});
    expect(submit).toBeEnabled();
    fireEvent.click(submit);
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH.sk, 'delivery-failure',
      expect.objectContaining({reason_type: 4, justification: 'x'.repeat(25), hash: 'qUqP5cyxm6YcTAhz05Hph5gvu9M='})));
  });

  it('cancelar entrega usa o tipo do evento escolhido (110180 vira receipt)', async () => {
    const spy = vi.spyOn(apiClient, 'registerCteDeliveryEvent').mockResolvedValue({} as never);
    renderCte();
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar entrega'}));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByLabelText('Entrega a cancelar'));
    await userEvent.click(await screen.findByRole('option', {name: /Comprovante de entrega do CT-e/}));
    fireEvent.click(within(dialog).getByRole('button', {name: 'Cancelar entrega'}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH.sk, 'delivery-cancel', expect.objectContaining({kind: 'receipt'})));
  });

  it('sem permissão não oferece a vinculação de pagamento', async () => {
    permissions = [];
    renderCte();
    await screen.findByRole('button', {name: 'Carta de correção'});
    expect(screen.queryByRole('button', {name: 'Vincular pagamento'})).toBeNull();
  });

  it('vincular pagamento abre o formulário com número, meio, recebedor e PSP', async () => {
    renderCte();
    fireEvent.click(await screen.findByRole('button', {name: 'Vincular pagamento'}));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByLabelText('Número do pagamento')).toHaveValue('1');
    expect(within(dialog).getByLabelText(/Identificador da transação/)).toBeInTheDocument();
    expect(within(dialog).getByLabelText('Meio de pagamento')).toBeInTheDocument();
    expect(within(dialog).getByText('Recebedor do pagamento')).toBeInTheDocument();
    expect(within(dialog).getByRole('button', {name: 'Vincular pagamento'})).toBeDisabled();
  });

  it('cancelar vinculação envia o event_sk escolhido ao 110301', async () => {
    const spy = vi.spyOn(apiClient, 'registerCtePaymentLink').mockResolvedValue({} as never);
    renderCte();
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar vinculação'}));
    const dialog = await screen.findByRole('dialog');
    await userEvent.click(within(dialog).getByLabelText('Vinculação a cancelar'));
    await userEvent.click(await screen.findByRole('option', {name: /Vinculação de pagamento/}));
    fireEvent.click(within(dialog).getByRole('button', {name: 'Cancelar vinculação'}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith(AUTH.sk, 'payment-link-cancel', expect.objectContaining({event_sk: expect.any(String)})));
  });
});
