import {createHash} from 'node:crypto';
import {existsSync, readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {apiClient} from '@/lib/api/client';
import {currentPushDeviceId, pushDeviceId} from '@/lib/push/device';
import {PUSH_LOGOUT_TIMEOUT_MS, removeThisDevicePush} from '@/lib/push/logout';
import {orgOfPath} from '@/lib/push/navigate';
import {PUSH_DISABLED, pushCallbacks} from '@/lib/push/callbacks';

const ROOT = resolve(__dirname, '../../..');
const ENDPOINT = 'https://fcm.googleapis.com/fcm/send/abc:def-123';

describe('service worker e manifest', () => {
  // The worker is copied to public/ by `prebuild`; a stale copy would ship an old version of the push logic.
  it('public/sw.js é byte a byte o do @aoctech/push-client', () => {
    const published = readFileSync(resolve(ROOT, 'public/sw.js'));
    const packaged = readFileSync(resolve(ROOT, 'node_modules/@aoctech/push-client/dist/sw.js'));
    expect(published.equals(packaged)).toBe(true);
  });

  it('o worker não tem cache nem handler de fetch (um deploy nunca serve bundle velho)', () => {
    const sw = readFileSync(resolve(ROOT, 'public/sw.js'), 'utf8');
    expect(sw).not.toMatch(/addEventListener\(\s*['"]fetch['"]/);
    expect(sw).not.toMatch(/\bcaches\./);
  });

  it('o manifest nomeia o app, abre no painel e aponta ícones que existem', () => {
    const manifest = JSON.parse(readFileSync(resolve(ROOT, 'public/site.webmanifest'), 'utf8')) as {
      name: string; short_name: string; display: string; start_url: string; scope: string;
      icons: { src: string; purpose?: string }[]
    };
    expect(manifest).toMatchObject({name: 'CTech DF-e', short_name: 'DF-e', display: 'standalone', start_url: '/dashboard', scope: '/'});
    expect(manifest.icons.some((i) => i.purpose === 'maskable')).toBe(true);
    for (const icon of manifest.icons) expect(existsSync(resolve(ROOT, 'public', icon.src.slice(1))), icon.src).toBe(true);
  });
});

describe('pushDeviceId', () => {
  // Same rule as `repositories.PushSubscriptionID` in the API: 16 hex characters of SHA-256(endpoint).
  it('é o prefixo de 8 bytes do SHA-256 do endpoint, em hexadecimal', async () => {
    const expected = createHash('sha256').update(ENDPOINT).digest('hex').slice(0, 16);
    expect(await pushDeviceId(ENDPOINT)).toBe(expected);
    expect(await pushDeviceId(ENDPOINT)).toMatch(/^[0-9a-f]{16}$/);
  });

  it('endpoints diferentes dão ids diferentes', async () => {
    expect(await pushDeviceId(ENDPOINT)).not.toBe(await pushDeviceId(`${ENDPOINT}x`));
  });
});

function stubServiceWorker(subscription: { endpoint: string; unsubscribe: () => Promise<boolean> } | null) {
  const registration = {pushManager: {getSubscription: async () => subscription}};
  vi.stubGlobal('navigator', {...navigator, serviceWorker: {getRegistration: async () => registration}});
}

describe('currentPushDeviceId', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('devolve o id da inscrição do navegador, ou null sem inscrição', async () => {
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe: async () => true});
    expect(await currentPushDeviceId()).toBe(await pushDeviceId(ENDPOINT));
    stubServiceWorker(null);
    expect(await currentPushDeviceId()).toBeNull();
  });
});

describe('removeThisDevicePush (logout)', () => {
  beforeEach(() => vi.useFakeTimers({shouldAdvanceTime: true}));
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('apaga a linha do servidor e a inscrição do navegador', async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe});
    const del = vi.spyOn(apiClient, 'deletePushDevice').mockResolvedValue(undefined);
    await removeThisDevicePush();
    expect(del).toHaveBeenCalledWith(await pushDeviceId(ENDPOINT));
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('falha no servidor não impede de apagar a inscrição local nem de sair', async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe});
    vi.spyOn(apiClient, 'deletePushDevice').mockRejectedValue(new Error('rede'));
    await expect(removeThisDevicePush()).resolves.toBeUndefined();
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('sem inscrição não chama a API', async () => {
    stubServiceWorker(null);
    const del = vi.spyOn(apiClient, 'deletePushDevice');
    await removeThisDevicePush();
    expect(del).not.toHaveBeenCalled();
  });

  it('um servidor que não responde não segura o logout além do limite', async () => {
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe: async () => true});
    vi.spyOn(apiClient, 'deletePushDevice').mockReturnValue(new Promise(() => undefined));
    const done = vi.fn();
    const pending = removeThisDevicePush().then(done);
    await vi.advanceTimersByTimeAsync(PUSH_LOGOUT_TIMEOUT_MS - 100);
    expect(done).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(200);
    await pending;
    expect(done).toHaveBeenCalled();
  });
});

describe('orgOfPath', () => {
  it('lê a empresa do link e ignora link sem ela', () => {
    expect(orgOfPath('/nfe/detail?key=35260112345678000190550010000000011000000011&org=ORG_X')).toBe('ORG_X');
    expect(orgOfPath('/dashboard')).toBeNull();
  });
});

describe('pushCallbacks', () => {
  afterEach(() => vi.restoreAllMocks());

  it('getConfig traduz a configuração do servidor e recusa quando está desligada', async () => {
    const config = vi.spyOn(apiClient, 'getPushConfig');
    config.mockResolvedValueOnce({enabled: true, public_key: 'BEl6', vapid_kid: 'k1', event_types: [], defaults: [], max_subscriptions: 10});
    expect(await pushCallbacks.getConfig()).toEqual({publicKey: 'BEl6', kid: 'k1'});
    config.mockResolvedValueOnce({enabled: false, event_types: [], defaults: [], max_subscriptions: 10});
    await expect(pushCallbacks.getConfig()).rejects.toThrow(PUSH_DISABLED);
  });

  it('save envia só endpoint e chaves; remove apaga pelo id derivado do endpoint', async () => {
    const put = vi.spyOn(apiClient, 'putPushSubscription').mockResolvedValue({id: 'x', device: 'd'});
    await pushCallbacks.save({endpoint: ENDPOINT, keys: {p256dh: 'a', auth: 'b'}, vapid_kid: 'k1', expiration_time: null});
    expect(put).toHaveBeenCalledWith({endpoint: ENDPOINT, keys: {p256dh: 'a', auth: 'b'}});
    const del = vi.spyOn(apiClient, 'deletePushDevice').mockResolvedValue(undefined);
    await pushCallbacks.remove(ENDPOINT);
    expect(del).toHaveBeenCalledWith(await pushDeviceId(ENDPOINT));
  });
});
