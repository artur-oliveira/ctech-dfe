import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig} from 'axios';
import {apiClient, ApiError} from '@/lib/api/client';
import {setApiStatus} from '@/lib/network/api-status';

// Reads are retried (twice, jittered, Retry-After); writes never are; a known
// outage fails fast without touching the network.
function failing(status: number, calls: {n: number}, headers: Record<string, string> = {}): AxiosAdapter {
  return async (config) => {
    calls.n += 1;
    const response = {status, statusText: '', data: {}, headers, config: config as InternalAxiosRequestConfig};
    throw new AxiosError('fail', 'ERR_BAD_RESPONSE', config as InternalAxiosRequestConfig, null, response);
  };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.spyOn(Math, 'random').mockReturnValue(0);
  setApiStatus('online');
  Object.defineProperty(window, 'location', {configurable: true, value: {pathname: '/nfe', search: '', replace: vi.fn()}});
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('client retry (withRetry)', () => {
  it('retries a read twice on 503 and then gives up with the ApiError', async () => {
    const calls = {n: 0};
    apiClient.setAdapter(failing(503, calls, {'retry-after': '1'}));
    const result = apiClient.getProduct('p1').catch((e) => e);
    await vi.advanceTimersByTimeAsync(5_000);
    expect(await result).toBeInstanceOf(ApiError);
    expect(calls.n).toBe(3);
  });

  it('does not retry a write', async () => {
    const calls = {n: 0};
    apiClient.setAdapter(failing(503, calls));
    await expect(apiClient.acceptTermsAddendum()).rejects.toBeInstanceOf(ApiError);
    expect(calls.n).toBe(1);
  });

  it('does not retry a client error', async () => {
    const calls = {n: 0};
    apiClient.setAdapter(failing(404, calls));
    await expect(apiClient.getProduct('p1')).rejects.toMatchObject({status: 404});
    expect(calls.n).toBe(1);
  });

  it('fails fast, with no request, while the API is known to be down', async () => {
    const calls = {n: 0};
    apiClient.setAdapter(failing(503, calls));
    setApiStatus('unavailable');
    await expect(apiClient.getProduct('p1')).rejects.toMatchObject({status: 0, raw: {reason: 'server'}});
    expect(calls.n).toBe(0);
  });
});
