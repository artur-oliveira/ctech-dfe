import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {act, cleanup, render} from '@testing-library/react';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {PushProvider, usePush} from '@/lib/providers/PushProvider';
import {STORAGE_KEY_PUSH_STAMP, STORAGE_KEY_PUSH_USER} from '@/lib/constants/storage';

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  register: vi.fn().mockResolvedValue(null),
  onNavigate: vi.fn(),
  subscription: vi.fn(),
  stampWhenSyncRan: vi.fn(),
  setSelectedOrg: vi.fn(),
  user: null as { user_id: string; organizations: { pk: string; name: string }[] } | null,
  selectedOrg: {pk: 'c1', name: 'Alfa'} as { pk: string; name: string } | null,
  handler: null as null | ((path: string) => void),
}));

vi.mock('next/navigation', () => ({useRouter: () => ({push: mocks.push})}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({user: mocks.user, selectedOrg: mocks.selectedOrg, setSelectedOrg: mocks.setSelectedOrg}),
}));
vi.mock('@aoctech/push-client', async () => {
  const {useEffect} = await import('react');
  return {
    registerServiceWorker: mocks.register,
    onNavigateMessage: (handler: (path: string) => void) => {
      mocks.handler = handler;
      return () => {
        mocks.handler = null;
      };
    },
    // Like the real hook, the "sync" happens in an effect once `enabled` is true; it reads the stamp the way the client does.
    usePushSubscription: (options: { enabled: boolean }) => {
      mocks.subscription(options);
      useEffect(() => {
        if (options.enabled) mocks.stampWhenSyncRan(localStorage.getItem('ctech-push:last-sync'));
      }, [options.enabled]);
      return {status: 'default', subscribe: vi.fn(), unsubscribe: vi.fn()};
    },
  };
});

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
  localStorage.clear();
  mocks.user = {user_id: 'u1', organizations: ORGS};
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

  describe('um navegador, uma pessoa', () => {
    const STAMP = '{"at":1763400000000,"kid":"k1"}';

    it('outra pessoa entra no mesmo navegador: o carimbo cai e a sincronização reenvia a inscrição', () => {
      localStorage.setItem(STORAGE_KEY_PUSH_STAMP, STAMP);
      localStorage.setItem(STORAGE_KEY_PUSH_USER, 'u-anterior');
      renderProvider();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_STAMP)).toBeNull();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_USER)).toBe('u1');
    });

    it('a mesma pessoa de novo: o carimbo fica, sem reenviar à toa', () => {
      localStorage.setItem(STORAGE_KEY_PUSH_STAMP, STAMP);
      localStorage.setItem(STORAGE_KEY_PUSH_USER, 'u1');
      renderProvider();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_STAMP)).toBe(STAMP);
    });

    it('navegador sem registro de quem sincronizou por último (logout limpou, ou primeiro uso): reenvia', () => {
      localStorage.setItem(STORAGE_KEY_PUSH_STAMP, STAMP);
      renderProvider();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_STAMP)).toBeNull();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_USER)).toBe('u1');
    });

    it('o carimbo já caiu quando a sincronização roda (a ordem dos efeitos é o que garante o reenvio)', () => {
      localStorage.setItem(STORAGE_KEY_PUSH_STAMP, STAMP);
      localStorage.setItem(STORAGE_KEY_PUSH_USER, 'u-anterior');
      renderProvider();
      expect(mocks.stampWhenSyncRan).toHaveBeenCalledWith(null);
    });

    it('a mesma pessoa chega na sincronização com o carimbo intacto', () => {
      localStorage.setItem(STORAGE_KEY_PUSH_STAMP, STAMP);
      localStorage.setItem(STORAGE_KEY_PUSH_USER, 'u1');
      renderProvider();
      expect(mocks.stampWhenSyncRan).toHaveBeenCalledWith(STAMP);
    });

    it('sem sessão não registra ninguém', () => {
      mocks.user = null;
      renderProvider();
      expect(localStorage.getItem(STORAGE_KEY_PUSH_USER)).toBeNull();
    });
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
