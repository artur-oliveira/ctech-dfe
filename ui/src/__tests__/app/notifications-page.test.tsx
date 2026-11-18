import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {cleanup, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import NotificationsPage from '@/app/notifications/page';
import {toast} from 'sonner';
import {ApiError, apiClient} from '@/lib/api/client';
import type {PushConfig, PushDeviceList} from '@/lib/types/push';

vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: { children: React.ReactNode }) => <>{children}</>}));
vi.mock('sonner', () => ({toast: Object.assign(vi.fn(), {success: vi.fn(), error: vi.fn()})}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'c1', name: 'Alfa Ltda', role: 'VIEWER'}}),
}));

const push = vi.hoisted(() => ({
  value: {status: 'default', subscribe: vi.fn(), unsubscribe: vi.fn(), canInstall: false, install: vi.fn()} as Record<string, unknown>,
}));
vi.mock('@/lib/providers/PushProvider', () => ({usePush: () => push.value}));

const thisDevice = vi.hoisted(() => ({id: null as string | null}));
vi.mock('@/lib/push/device', () => ({currentPushDeviceId: async () => thisDevice.id}));

const CONFIG: PushConfig = {
  enabled: true, public_key: 'BEl6', vapid_kid: 'k1', max_subscriptions: 10,
  event_types: ['distribution.received', 'document.authorized', 'document.rejected', 'document.failed'],
  defaults: ['distribution.received'],
};
const DEVICES: PushDeviceList = {
  max_subscriptions: 10,
  data: [
    {id: '0a1b2c3d4e5f6a7b', device: 'Chrome no Android', last_seen_at: '2026-11-17T09:30:00Z'},
    {id: '1b2c3d4e5f6a7b8c', device: 'Edge no Windows'},
  ],
};

function setPush(over: Record<string, unknown>) {
  push.value = {status: 'default', subscribe: vi.fn(), unsubscribe: vi.fn(), canInstall: false, install: vi.fn(), ...over};
}

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
      <NotificationsPage/>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  setPush({});
  thisDevice.id = null;
  vi.spyOn(apiClient, 'getPushConfig').mockResolvedValue(CONFIG);
  vi.spyOn(apiClient, 'listPushDevices').mockResolvedValue(DEVICES);
  vi.spyOn(apiClient, 'getPushPreferences').mockResolvedValue({events: ['distribution.received']});
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('estados do dispositivo', () => {
  it('ainda sem permissão: oferece ativar, e o clique é o que pede a permissão', async () => {
    const subscribe = vi.fn().mockResolvedValue(undefined);
    setPush({status: 'default', subscribe});
    renderPage();
    await userEvent.click(await screen.findByRole('button', {name: 'Ativar neste dispositivo'}));
    await waitFor(() => expect(subscribe).toHaveBeenCalledTimes(1));
  });

  it('iPhone fora da Tela de Início: mostra o passo a passo e nenhum botão de ativar', async () => {
    setPush({status: 'needs-install'});
    renderPage();
    expect(await screen.findByText(/Adicionar à Tela de Início/)).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Ativar neste dispositivo'})).not.toBeInTheDocument();
  });

  it('bloqueado e sem suporte: explicam o que fazer, sem botão', async () => {
    setPush({status: 'denied'});
    const {unmount} = renderPage();
    expect(await screen.findByText(/libere as notificações para este site/)).toBeInTheDocument();
    unmount();
    setPush({status: 'unsupported'});
    renderPage();
    expect(await screen.findByText(/Este navegador não recebe notificações/)).toBeInTheDocument();
  });

  it('erro: mostra o alerta e deixa tentar de novo', async () => {
    const subscribe = vi.fn().mockResolvedValue(undefined);
    setPush({status: 'error', subscribe});
    renderPage();
    expect(await screen.findByRole('alert')).toHaveTextContent(/Algo falhou/);
    await userEvent.click(screen.getByRole('button', {name: 'Tentar de novo'}));
    await waitFor(() => expect(subscribe).toHaveBeenCalled());
  });

  it('o navegador que oferece instalar mostra o botão', async () => {
    const install = vi.fn();
    setPush({status: 'default', canInstall: true, install});
    renderPage();
    await userEvent.click(await screen.findByRole('button', {name: 'Instalar o aplicativo'}));
    expect(install).toHaveBeenCalled();
  });

  it('instalação sem chave: diz que está indisponível e não lista nada', async () => {
    vi.spyOn(apiClient, 'getPushConfig').mockResolvedValue({...CONFIG, enabled: false, public_key: undefined});
    renderPage();
    expect(await screen.findByText('Notificações indisponíveis')).toBeInTheDocument();
    expect(screen.queryByText('Meus dispositivos')).not.toBeInTheDocument();
    expect(apiClient.listPushDevices).not.toHaveBeenCalled();
  });

  it('403 na configuração mostra a falta de permissão em vez de carregar para sempre', async () => {
    vi.spyOn(apiClient, 'getPushConfig').mockRejectedValue(new ApiError(403, 'escopo do token insuficiente', {}));
    renderPage();
    expect(await screen.findByText(/não tem permissão para/i)).toBeInTheDocument();
  });
});

describe('ativo neste dispositivo', () => {
  beforeEach(() => {
    setPush({status: 'subscribed'});
    thisDevice.id = '0a1b2c3d4e5f6a7b';
  });

  it('marca os eventos escolhidos e salva a lista na ordem da tela', async () => {
    const save = vi.spyOn(apiClient, 'putPushPreferences').mockResolvedValue({events: []});
    renderPage();
    const rejected = await screen.findByRole('checkbox', {name: 'Documento rejeitado'});
    expect(screen.getByRole('checkbox', {name: 'Nota recebida'})).toBeChecked();
    expect(rejected).not.toBeChecked();
    await userEvent.click(rejected);
    await waitFor(() => expect(save).toHaveBeenCalledWith(['distribution.received', 'document.rejected']));
  });

  it('marca o próprio dispositivo na lista e remove outro por id', async () => {
    const remove = vi.spyOn(apiClient, 'deletePushDevice').mockResolvedValue(undefined);
    renderPage();
    const mine = (await screen.findByText('Chrome no Android')).closest('li') as HTMLElement;
    await waitFor(() => expect(within(mine).getByText('Este dispositivo')).toBeInTheDocument());
    const other = screen.getByText('Edge no Windows').closest('li') as HTMLElement;
    await userEvent.click(within(other).getByRole('button', {name: 'Remover'}));
    await waitFor(() => expect(remove).toHaveBeenCalledWith('1b2c3d4e5f6a7b8c'));
  });

  it('remover o próprio dispositivo passa pelo cliente de push, que apaga também a inscrição do navegador', async () => {
    const unsubscribe = vi.fn().mockResolvedValue(undefined);
    setPush({status: 'subscribed', unsubscribe});
    const remove = vi.spyOn(apiClient, 'deletePushDevice').mockResolvedValue(undefined);
    renderPage();
    const mine = (await screen.findByText('Chrome no Android')).closest('li') as HTMLElement;
    await waitFor(() => expect(within(mine).getByText('Este dispositivo')).toBeInTheDocument());
    await userEvent.click(within(mine).getByRole('button', {name: 'Remover'}));
    await waitFor(() => expect(unsubscribe).toHaveBeenCalled());
    expect(remove).not.toHaveBeenCalled();
  });

  it('o teste confirma o envio e, em 429, pede para aguardar', async () => {
    const test = vi.spyOn(apiClient, 'sendPushTest').mockResolvedValueOnce(undefined);
    renderPage();
    await userEvent.click(await screen.findByRole('button', {name: 'Enviar teste'}));
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    test.mockRejectedValueOnce(new ApiError(429, 'limite', {}));
    await userEvent.click(screen.getByRole('button', {name: 'Enviar teste'}));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/Aguarde um minuto/)));
  });

  it('erro ao salvar a escolha avisa e mantém a tela', async () => {
    vi.spyOn(apiClient, 'putPushPreferences').mockRejectedValue(new ApiError(400, 'evento desconhecido', {}));
    renderPage();
    await userEvent.click(await screen.findByRole('checkbox', {name: 'Documento autorizado'}));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('evento desconhecido'));
  });
});
