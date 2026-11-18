import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, cleanup, render} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {PushProvider, usePush} from '@/lib/providers/PushProvider';

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  register: vi.fn().mockResolvedValue(null),
  onNavigate: vi.fn(),
  subscription: vi.fn(),
  setSelectedOrg: vi.fn(),
  user: null as { organizations: { pk: string; name: string }[] } | null,
  selectedOrg: {pk: 'c1', name: 'Alfa'} as { pk: string; name: string } | null,
  handler: null as null | ((path: string) => void),
}));

vi.mock('next/navigation', () => ({useRouter: () => ({push: mocks.push})}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({user: mocks.user, selectedOrg: mocks.selectedOrg, setSelectedOrg: mocks.setSelectedOrg}),
}));
vi.mock('@aoctech/push-client', () => ({
  registerServiceWorker: mocks.register,
  onNavigateMessage: (handler: (path: string) => void) => {
    mocks.handler = handler;
    return () => {
      mocks.handler = null;
    };
  },
  usePushSubscription: (options: unknown) => {
    mocks.subscription(options);
    return {status: 'default', subscribe: vi.fn(), unsubscribe: vi.fn()};
  },
}));

const ORGS = [{pk: 'c1', name: 'Alfa'}, {pk: 'c2', name: 'Beta'}];

function Probe() {
  const {status, canInstall} = usePush();
  return <p data-testid="probe">{status}/{String(canInstall)}</p>;
}

function renderProvider() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <PushProvider><Probe/></PushProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  mocks.user = {organizations: ORGS};
  mocks.selectedOrg = ORGS[0];
  window.history.replaceState(null, '', '/dashboard');
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('PushProvider', () => {
  it('registra o service worker e liga a sincronização só com sessão e empresa', () => {
    renderProvider();
    expect(mocks.register).toHaveBeenCalledTimes(1);
    expect(mocks.subscription).toHaveBeenLastCalledWith(expect.objectContaining({enabled: true}));
    cleanup();
    mocks.user = null;
    renderProvider();
    expect(mocks.subscription).toHaveBeenLastCalledWith(expect.objectContaining({enabled: false}));
  });

  it('o clique numa notificação de outra empresa troca para ela e roteia', () => {
    renderProvider();
    act(() => mocks.handler?.('/nfe/detail?key=K&org=c2'));
    expect(mocks.setSelectedOrg).toHaveBeenCalledWith(ORGS[1]);
    expect(mocks.push).toHaveBeenCalledWith('/nfe/detail?key=K&org=c2');
  });

  it('empresa do link da qual a pessoa não é membro não é selecionada', () => {
    renderProvider();
    act(() => mocks.handler?.('/nfe/detail?key=K&org=forjada'));
    expect(mocks.setSelectedOrg).not.toHaveBeenCalled();
    expect(mocks.push).toHaveBeenCalledWith('/nfe/detail?key=K&org=forjada');
  });

  it('a empresa que já está selecionada não é trocada de novo', () => {
    renderProvider();
    act(() => mocks.handler?.('/nfe?org=c1'));
    expect(mocks.setSelectedOrg).not.toHaveBeenCalled();
  });

  it('abertura a frio pelo link da notificação respeita a empresa uma vez, depois do login', () => {
    window.history.replaceState(null, '', '/nfe/detail?key=K&org=c2');
    renderProvider();
    expect(mocks.setSelectedOrg).toHaveBeenCalledTimes(1);
    expect(mocks.setSelectedOrg).toHaveBeenCalledWith(ORGS[1]);
  });

  it('expõe o convite de instalação do navegador quando ele dispara o evento', () => {
    const {getByTestId} = renderProvider();
    expect(getByTestId('probe').textContent).toBe('default/false');
    const event = Object.assign(new Event('beforeinstallprompt', {cancelable: true}), {prompt: vi.fn().mockResolvedValue(undefined)});
    act(() => {
      window.dispatchEvent(event);
    });
    expect(event.defaultPrevented).toBe(true);
    expect(getByTestId('probe').textContent).toBe('default/true');
  });
});
