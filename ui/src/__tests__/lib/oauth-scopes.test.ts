import {describe, expect, it, vi, beforeEach} from 'vitest';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';

vi.mock('@aoctech/auth-client', () => ({
  OAuthClient: class { startOAuthFlow = vi.fn(async () => undefined); },
  decodeIdToken: vi.fn(),
}));

process.env.NEXT_PUBLIC_CTECH_URL = 'https://account.test';
process.env.NEXT_PUBLIC_CTECH_CLIENT_ID = 'client';

const manifest = JSON.parse(readFileSync(resolve(__dirname, '../../../../api/internal/oauthresource/scope-manifest.json'), 'utf8')) as {
  scopes: {name: string}[]
};

describe('escopos pedidos no login', () => {
  // O login só pede o que está em DFE_USER_SCOPES; escopo fora dela dá 403 em toda rota nova (aconteceu com NFCom e BP-e).
  it('cobre todo escopo do manifesto da API', async () => {
    const {DFE_USER_SCOPES} = await import('@/lib/auth/oauth');
    const missing = manifest.scopes.map((s) => s.name).filter((n) => !(DFE_USER_SCOPES as readonly string[]).includes(n));
    expect(missing).toEqual([]);
  });
});

describe('reauthForMissingScopes', () => {
  beforeEach(() => sessionStorage.clear());

  it('refaz o login uma vez por conjunto de escopos e depois deixa o 403 passar', async () => {
    const {reauthForMissingScopes} = await import('@/lib/auth/oauth');
    expect(await reauthForMissingScopes()).toBe(true);
    expect(await reauthForMissingScopes()).toBe(false);
  });
});
