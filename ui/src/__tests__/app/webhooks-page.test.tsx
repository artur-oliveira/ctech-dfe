import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import WebhooksPage from '@/app/webhooks/page';
import {toast} from 'sonner';
import {ApiError, apiClient} from '@/lib/api/client';
import type {WebhookEndpoint, WebhookList} from '@/lib/types/webhooks';

vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('sonner', () => ({toast: Object.assign(vi.fn(), {success: vi.fn(), error: vi.fn()})}));

let role = 'OWNER';
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({
    user: {organizations: [{pk: 'c1', name: 'Alfa Ltda'}, {pk: 'c2', name: 'Beta Ltda'}]},
    selectedOrg: {pk: 'c1', name: 'Alfa Ltda', role},
  }),
}));

const SECRET = 'whsec_deadbeef0123456789';

const active: WebhookEndpoint = {
  id: 'wh1', organization_id: 'o1', url: 'https://erp.exemplo.com.br/hooks', events: null, company_pks: null,
  status: 'active', failure_run: 0, last_delivery_ok: true, last_delivery_at: '2026-11-14T13:02:11Z',
};
const disabled: WebhookEndpoint = {
  ...active, id: 'wh2', url: 'https://velho.exemplo.com.br/x', events: ['document.rejected'], company_pks: ['c2'],
  status: 'disabled', failure_run: 12, disabled_at: '2026-11-13T21:40:00Z', last_delivery_ok: false, last_delivery_detail: 'HTTP 404',
};

function list(data: WebhookEndpoint[], over: Partial<WebhookList> = {}): WebhookList {
  return {data, company_pks: ['c1', 'c2'], enabled: true, max_endpoints: 5, event_types: [], ...over};
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
      <WebhooksPage/>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  role = 'OWNER';
  vi.spyOn(apiClient, 'listWebhooks').mockResolvedValue(list([active, disabled]));
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('lista', () => {
  it('mostra cada endpoint com estado, filtros e a última entrega', async () => {
    renderPage();
    expect(await screen.findByText('https://erp.exemplo.com.br/hooks')).toBeInTheDocument();
    expect(screen.getByText('Ativo')).toBeInTheDocument();
    expect(screen.getByText('Desativado')).toBeInTheDocument();
    expect(screen.getAllByText('Todos os eventos')).toHaveLength(1);
    expect(screen.getByText('Rejeitada')).toBeInTheDocument();
    expect(screen.getByText('Beta Ltda')).toBeInTheDocument();
  });

  it('explica o endpoint desativado e troca Testar por Reativar', async () => {
    renderPage();
    const note = await screen.findByRole('note');
    expect(note).toHaveTextContent(/após 12 falhas seguidas \(HTTP 404\)/);
    expect(note).toHaveTextContent(/corrija o seu sistema e reative/i);
    const row = screen.getByText('https://velho.exemplo.com.br/x').closest('li') as HTMLElement;
    expect(within(row).queryByRole('button', {name: 'Testar'})).not.toBeInTheDocument();
  });

  it('só o endpoint desativado oferece Reativar', async () => {
    renderPage();
    await screen.findByText('https://erp.exemplo.com.br/hooks');
    expect(screen.getAllByRole('button', {name: 'Reativar'})).toHaveLength(1);
    const row = screen.getByText('https://velho.exemplo.com.br/x').closest('li') as HTMLElement;
    expect(within(row).getByRole('button', {name: 'Reativar'})).toBeEnabled();
  });

  it('reativar pede ao servidor, que repete a verificação do endereço', async () => {
    const enable = vi.spyOn(apiClient, 'enableWebhook').mockResolvedValue(undefined);
    renderPage();
    const row = (await screen.findByText('https://velho.exemplo.com.br/x')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Reativar'}));
    await waitFor(() => expect(enable).toHaveBeenCalledWith('wh2'));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith(expect.stringMatching(/reativado/i)));
  });

  it('se o endereço ainda não responde, explica em português e segue desativado', async () => {
    vi.spyOn(apiClient, 'enableWebhook').mockRejectedValue(new ApiError(422, 'the endpoint did not answer'));
    renderPage();
    const row = (await screen.findByText('https://velho.exemplo.com.br/x')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Reativar'}));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('O endereço não respondeu.')));
    expect(screen.getByText('Desativado')).toBeInTheDocument();
  });

  it('vazio ensina o que fazer', async () => {
    vi.mocked(apiClient.listWebhooks).mockResolvedValue(list([]));
    renderPage();
    expect(await screen.findByText('Nenhum webhook cadastrado')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Cadastrar o primeiro'})).toBeEnabled();
  });

  it('no limite de cinco, bloqueia o cadastro e diz por quê', async () => {
    const five = Array.from({length: 5}, (_, i) => ({...active, id: `w${i}`, url: `https://h${i}.exemplo.com.br/x`}));
    vi.mocked(apiClient.listWebhooks).mockResolvedValue(list(five));
    renderPage();
    expect(await screen.findByText(/Você usa 5 de 5 webhooks/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Novo webhook'})).toBeDisabled();
  });

  it('instalação sem chave avisa e não deixa cadastrar', async () => {
    vi.mocked(apiClient.listWebhooks).mockResolvedValue(list([], {enabled: false}));
    renderPage();
    expect(await screen.findByText(/ainda não estão disponíveis nesta instalação/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Novo webhook'})).toBeDisabled();
  });

  it('quem não é proprietário nem administrador não vê nada e a api nem é chamada', async () => {
    role = 'USER';
    renderPage();
    expect(await screen.findByRole('alert')).toHaveTextContent(/não tem permissão para gerenciar webhooks/);
    expect(apiClient.listWebhooks).not.toHaveBeenCalled();
  });

  it('um 403 da api também vira aviso de permissão', async () => {
    vi.mocked(apiClient.listWebhooks).mockRejectedValue(new ApiError(403, 'negado'));
    renderPage();
    expect(await screen.findByRole('alert')).toHaveTextContent(/não tem permissão/);
  });

  it('falha de rede oferece tentar de novo', async () => {
    vi.mocked(apiClient.listWebhooks).mockRejectedValueOnce(new ApiError(500, 'boom'));
    renderPage();
    expect(await screen.findByText(/Não foi possível carregar os webhooks/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Tentar de novo'}));
    expect(await screen.findByText('https://erp.exemplo.com.br/hooks')).toBeInTheDocument();
  });
});

describe('cadastro', () => {
  async function openForm() {
    renderPage();
    await userEvent.click(await screen.findByRole('button', {name: 'Novo webhook'}));
    return screen.findByLabelText('Endereço que recebe os avisos');
  }

  it('recusa http antes de falar com o servidor', async () => {
    const create = vi.spyOn(apiClient, 'createWebhook');
    const url = await openForm();
    await userEvent.type(url, 'http://erp.exemplo.com.br/x');
    await userEvent.click(screen.getByRole('button', {name: 'Cadastrar e validar'}));
    expect(await screen.findByText('O endereço precisa usar https.')).toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
  });

  it('eventos e empresas são caixas de marcar, nunca texto livre', async () => {
    await openForm();
    for (const name of ['Autorizada', 'Rejeitada', 'Falha na emissão', 'Alfa Ltda', 'Beta Ltda']) {
      expect(screen.getByRole('checkbox', {name})).toBeInTheDocument();
    }
    expect(screen.getAllByRole('textbox')).toHaveLength(1);
  });

  it('envia o que foi marcado e mostra o segredo uma única vez', async () => {
    const create = vi.spyOn(apiClient, 'createWebhook').mockResolvedValue({...active, id: 'wh3', secret: SECRET});
    const url = await openForm();
    await userEvent.type(url, 'https://novo.exemplo.com.br/hooks');
    await userEvent.click(screen.getByRole('checkbox', {name: 'Rejeitada'}));
    await userEvent.click(screen.getByRole('checkbox', {name: 'Beta Ltda'}));
    await userEvent.click(screen.getByRole('button', {name: 'Cadastrar e validar'}));

    await waitFor(() => expect(create).toHaveBeenCalledWith({
      url: 'https://novo.exemplo.com.br/hooks', events: ['document.rejected'], company_pks: ['c2'],
    }));
    expect(await screen.findByText(SECRET)).toBeInTheDocument();
    expect(screen.getByText(/não será mostrado de novo/)).toBeInTheDocument();

    // Esc e fundo não fecham: perder o segredo por um clique errado não tem volta.
    fireEvent.keyDown(window, {key: 'Escape'});
    expect(screen.getByText(SECRET)).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Guardei o segredo'}));
    expect(screen.queryByText(SECRET)).not.toBeInTheDocument();
  });

  it('explica em português quando o endereço não passa na verificação', async () => {
    vi.spyOn(apiClient, 'createWebhook').mockRejectedValue(
      new ApiError(422, 'the endpoint answered HTTP 404, expected 200 with the challenge'));
    const url = await openForm();
    await userEvent.type(url, 'https://novo.exemplo.com.br/hooks');
    await userEvent.click(screen.getByRole('button', {name: 'Cadastrar e validar'}));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('O endereço não passou na verificação');
    expect(alert).toHaveTextContent('O endereço respondeu HTTP 404');
    expect(screen.queryByText(SECRET)).not.toBeInTheDocument();
  });

  it('com uma empresa só, não pergunta de quais', async () => {
    vi.mocked(apiClient.listWebhooks).mockResolvedValue(list([], {company_pks: ['c1']}));
    renderPage();
    await userEvent.click(await screen.findByRole('button', {name: 'Novo webhook'}));
    await screen.findByLabelText('Endereço que recebe os avisos');
    expect(screen.queryByText('De quais empresas')).not.toBeInTheDocument();
  });
});

describe('ações do endpoint', () => {
  it('trocar o segredo pede confirmação e depois mostra o novo uma vez', async () => {
    const rotate = vi.spyOn(apiClient, 'rotateWebhookSecret').mockResolvedValue({secret: SECRET});
    renderPage();
    const row = (await screen.findByText('https://erp.exemplo.com.br/hooks')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Trocar segredo'}));
    expect(rotate).not.toHaveBeenCalled();
    expect(within(row).getByText(/O segredo atual para de valer na hora/)).toBeInTheDocument();
    await userEvent.click(within(row).getByRole('button', {name: 'Trocar o segredo'}));
    expect(await screen.findByText(SECRET)).toBeInTheDocument();
    expect(rotate).toHaveBeenCalledWith('wh1');
  });

  it('testar envia o ping para aquele endpoint', async () => {
    const ping = vi.spyOn(apiClient, 'pingWebhook').mockResolvedValue(undefined);
    renderPage();
    const row = (await screen.findByText('https://erp.exemplo.com.br/hooks')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Testar'}));
    await waitFor(() => expect(ping).toHaveBeenCalledWith('wh1'));
  });

  it('mostra as entregas e reenvia uma', async () => {
    vi.spyOn(apiClient, 'listWebhookDeliveries').mockResolvedValue({
      next_cursor: null,
      data: [{endpoint_id: 'wh1', event_id: 'ev9', type: 'document.rejected', status: 'failed', attempts: 8, last_error: 'timeout'}],
    });
    const replay = vi.spyOn(apiClient, 'replayWebhookDelivery').mockResolvedValue(undefined);
    renderPage();
    const row = (await screen.findByText('https://erp.exemplo.com.br/hooks')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Entregas'}));
    expect(await within(row).findByText('Falhou')).toBeInTheDocument();
    await userEvent.click(within(row).getByRole('button', {name: 'Reenviar'}));
    await waitFor(() => expect(replay).toHaveBeenCalledWith('wh1', 'ev9'));
  });

  it('sem entregas, aponta para o teste', async () => {
    vi.spyOn(apiClient, 'listWebhookDeliveries').mockResolvedValue({data: [], next_cursor: null});
    renderPage();
    const row = (await screen.findByText('https://erp.exemplo.com.br/hooks')).closest('li') as HTMLElement;
    await userEvent.click(within(row).getByRole('button', {name: 'Entregas'}));
    expect(await within(row).findByText(/Use Testar para enviar um evento de exemplo/)).toBeInTheDocument();
  });
});
