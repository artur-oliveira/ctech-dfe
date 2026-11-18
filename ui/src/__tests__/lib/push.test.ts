import {createHash} from 'node:crypto';
import {existsSync, readFileSync} from 'node:fs';
import {resolve} from 'node:path';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {apiClient} from '@/lib/api/client';
import {STORAGE_KEY_PUSH_STAMP, STORAGE_KEY_PUSH_USER} from '@/lib/constants/storage';
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

describe('contrato com o pacote', () => {
  // The app removes the stamp by name to force a resend; a rename in the package would silently turn that into a no-op.
  it('a chave do carimbo é a mesma que o @aoctech/push-client grava', () => {
    const client = readFileSync(resolve(ROOT, 'node_modules/@aoctech/push-client/dist/client.js'), 'utf8');
    expect(client).toContain(`'${STORAGE_KEY_PUSH_STAMP}'`);
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
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers({shouldAdvanceTime: true});
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('cancela a inscrição do navegador ANTES de apagar a linha do servidor', async () => {
    const calls: string[] = [];
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe: async () => (calls.push('local'), true)});
    const del = vi.spyOn(apiClient, 'deletePushDevice').mockImplementation(async () => {
      calls.push('server');
    });
    await removeThisDevicePush();
    expect(calls).toEqual(['local', 'server']);
    expect(del).toHaveBeenCalledWith(await pushDeviceId(ENDPOINT));
  });

  it('falha de rede no DELETE não deixa a inscrição local viva nem trava o logout', async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe});
    vi.spyOn(apiClient, 'deletePushDevice').mockRejectedValue(new Error('rede'));
    await expect(removeThisDevicePush()).resolves.toBeUndefined();
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('esquece o carimbo da última sincronização e de quem era, mesmo sem inscrição', async () => {
    localStorage.setItem(STORAGE_KEY_PUSH_STAMP, '{"at":1,"kid":"k"}');
    localStorage.setItem(STORAGE_KEY_PUSH_USER, 'u1');
    stubServiceWorker(null);
    await removeThisDevicePush();
    expect(localStorage.getItem(STORAGE_KEY_PUSH_STAMP)).toBeNull();
    expect(localStorage.getItem(STORAGE_KEY_PUSH_USER)).toBeNull();
  });

  it('o carimbo cai antes de qualquer espera de rede (o próximo login reenvia)', async () => {
    localStorage.setItem(STORAGE_KEY_PUSH_STAMP, '{"at":1,"kid":"k"}');
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe: () => new Promise(() => undefined)});
    void removeThisDevicePush();
    expect(localStorage.getItem(STORAGE_KEY_PUSH_STAMP)).toBeNull();
  });

  it('sem inscrição não chama a API', async () => {
    stubServiceWorker(null);
    const del = vi.spyOn(apiClient, 'deletePushDevice');
    await removeThisDevicePush();
    expect(del).not.toHaveBeenCalled();
  });

  it('um servidor que não responde não segura o logout além do limite, com a inscrição local já cancelada', async () => {
    const unsubscribe = vi.fn().mockResolvedValue(true);
    stubServiceWorker({endpoint: ENDPOINT, unsubscribe});
    vi.spyOn(apiClient, 'deletePushDevice').mockReturnValue(new Promise(() => undefined));
    // Real timers and a short limit: the point is that the call returns although the server never answers.
    vi.useRealTimers();
    const started = Date.now();
    await removeThisDevicePush(60);
    expect(Date.now() - started).toBeLessThan(PUSH_LOGOUT_TIMEOUT_MS);
    expect(unsubscribe).toHaveBeenCalledTimes(1);
  });

  it('o limite padrão do logout é de 2 s', () => {
    expect(PUSH_LOGOUT_TIMEOUT_MS).toBe(2000);
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
