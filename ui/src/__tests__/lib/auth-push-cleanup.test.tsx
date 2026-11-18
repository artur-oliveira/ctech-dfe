import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useContext} from 'react';
import {act, renderHook, waitFor} from '@testing-library/react';
import {AuthContext, AuthProvider} from '@/lib/context/AuthContext';

const calls: string[] = [];
const mocks = vi.hoisted(() => ({
  doRefresh: vi.fn(),
  removePush: vi.fn(),
  setToken: vi.fn(),
}));

vi.mock('@/lib/api/client', () => ({
  apiClient: {me: vi.fn(), setToken: (t: unknown) => mocks.setToken(t)},
  registerRefreshFn: vi.fn(),
  ApiError: class ApiError extends Error {},
}));
vi.mock('@/lib/auth/oauth', () => ({
  currentReturnTo: () => '/',
  decodeIdToken: vi.fn(),
  doRefresh: () => mocks.doRefresh(),
  endSessionRedirect: vi.fn(),
  revokeToken: vi.fn().mockResolvedValue(undefined),
  startOAuthFlow: vi.fn(),
}));
vi.mock('@/lib/push/logout', () => ({removeThisDevicePush: () => mocks.removePush()}));

function renderAuth() {
  return renderHook(() => useContext(AuthContext)!, {wrapper: ({children}) => <AuthProvider>{children}</AuthProvider>});
}

describe('AuthContext e a inscrição de push deste navegador', () => {
  beforeEach(() => {
    localStorage.clear();
    calls.length = 0;
    mocks.doRefresh.mockReset();
    mocks.removePush.mockReset().mockImplementation(async () => void calls.push('push'));
    mocks.setToken.mockReset().mockImplementation(() => void calls.push('token'));
  });

  it('sem sessão (o refresh responde que não há), este navegador para de notificar quem se foi', async () => {
    mocks.doRefresh.mockResolvedValue(null);
    renderAuth();
    await waitFor(() => expect(mocks.removePush).toHaveBeenCalledTimes(1));
  });

  it('falha transitória do refresh não é prova de logout: a inscrição fica', async () => {
    mocks.doRefresh.mockRejectedValue(new Error('rede'));
    const {result} = renderAuth();
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(mocks.removePush).not.toHaveBeenCalled();
  });

  it('logout apaga o dispositivo ANTES de limpar o token (o DELETE precisa dele)', async () => {
    mocks.doRefresh.mockRejectedValue(new Error('rede'));
    const {result} = renderAuth();
    await waitFor(() => expect(result.current.loading).toBe(false));
    await act(async () => {
      await result.current.logout('/');
    });
    expect(calls).toEqual(['push', 'token']);
  });
});
