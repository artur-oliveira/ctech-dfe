import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {act, render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {NetworkBoundary} from '@/lib/network/NetworkBoundary';
import {getApiStatus, requireApiAvailable, setApiStatus} from '@/lib/network/api-status';

let healthy = false;
const fetchMock = vi.fn(async () => new Response('{}', {status: healthy ? 200 : 503}));

beforeEach(() => {
  healthy = false;
  setApiStatus('online');
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe('NetworkBoundary', () => {
  it('shows the banner in the product copy, then refetches and clears it on reconnect', async () => {
    const onReconnect = vi.fn();
    render(<NetworkBoundary onReconnect={onReconnect}><p>app</p></NetworkBoundary>);

    // First probe fails: degraded, banner up, the API client is told.
    expect(await screen.findByRole('button', {name: 'Verificar agora'})).toBeInTheDocument();
    expect(getApiStatus()).toBe('degraded');
    expect(onReconnect).not.toHaveBeenCalled();

    healthy = true;
    await act(async () => {
      await userEvent.click(screen.getByRole('button', {name: 'Verificar agora'}));
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
    expect(getApiStatus()).toBe('online');
    expect(screen.queryByRole('button', {name: 'Verificar agora'})).not.toBeInTheDocument();
  });

  it('makes the client fail fast only once the outage is confirmed', () => {
    setApiStatus('degraded');
    expect(() => requireApiAvailable()).not.toThrow();
    setApiStatus('unavailable');
    expect(() => requireApiAvailable()).toThrow('Servidor temporariamente indisponível');
    setApiStatus('offline');
    expect(() => requireApiAvailable()).toThrow('Sem conexão com a internet');
  });
});
