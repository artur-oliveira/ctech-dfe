import {describe, expect, it} from 'vitest';
import {mockAdapter} from '@/lib/mock/handler';
import {
  nfeDetailFixture,
  nfeEventsFixture,
  organizationsFixture,
  servicesFixture,
  taxProfilesFixture,
} from '@/lib/mock/fixtures';
import {DOC_CONTEXTS, SHARED_REGISTRIES} from '@/lib/navigation/nav';
import type {InternalAxiosRequestConfig} from 'axios';

/**
 * O mock é o que alimenta as capturas do guia. Uma rota não modelada cai no
 * fallback (lista vazia) e a captura sai com a tela vazia — sem erro nenhum.
 * Estes testes cobrem as rotas cuja ausência já produziu imagem errada.
 */
async function get(url: string) {
  const response = await mockAdapter({method: 'get', url, headers: {}} as InternalAxiosRequestConfig);
  return response.data as Record<string, unknown>;
}

describe('mock handler', () => {
  it('serve o detalhe da organização com `person`', async () => {
    // Sem isto, `org?.person?.crt` fica indefinido e o passo de produtos do
    // onboarding renderiza sem CRT nem UF.
    const data = await get(`/v1.0/organizations/${organizationsFixture[0].pk}`);
    expect(data.person).toBeDefined();
  });

  it('serve os cadastros reutilizáveis com itens', async () => {
    const services = await get('/v1.0/services');
    const profiles = await get('/v1.0/tax-profiles');
    expect(services.items).toHaveLength(servicesFixture.length);
    expect(profiles.items).toHaveLength(taxProfilesFixture.length);
  });

  /**
   * Todo cadastro que aparece na navegação vira captura do guia. Sem fixture, a
   * rota cai no fallback de lista vazia e a imagem sai em branco — sem erro.
   */
  it('serve itens para todo cadastro da navegação', async () => {
    const hrefs = [
      ...SHARED_REGISTRIES.map(i => i.href),
      ...DOC_CONTEXTS.flatMap(ctx => ctx.items.map(i => i.href)),
    ];
    const empty: string[] = [];
    for (const href of hrefs) {
      const data = await get(`/v1.0${href}`);
      if (!Array.isArray(data.items) || data.items.length === 0) empty.push(href);
    }
    expect(empty).toEqual([]);
  });

  it('distingue a rota de eventos da rota de detalhe', async () => {
    const key = nfeDetailFixture.sk;
    const events = await get(`/v1.0/nfes/${key}/events`);
    const detail = await get(`/v1.0/nfes/${key}`);
    expect(events.items).toHaveLength(nfeEventsFixture.length);
    expect(detail.products).toBeDefined();
  });
});

describe('mock handler: recusa local da NF-e', () => {
  it('com ?mock=prevalidation a emissão volta 422 fiscal_prevalidation', async () => {
    const {initMockStateFromUrl, setMockState} = await import('@/lib/mock/state');
    initMockStateFromUrl('?mock=prevalidation');
    try {
      await expect(mockAdapter({method: 'post', url: '/v1.0/nfes', data: '{}', headers: {}} as InternalAxiosRequestConfig))
        .rejects.toMatchObject({response: {status: 422, data: {code: 'fiscal_prevalidation'}}});
    } finally {
      setMockState({emitPrevalidation: false});
    }
  });
});

describe('mock handler: MDF-e na SEFAZ', () => {
  it('serve os não encerrados antes da rota de detalhe', async () => {
    const data = await get('/v1.0/mdfes/unclosed');
    expect(data).toMatchObject({c_stat: '111'});
    expect(data.items).toHaveLength(2);
  });

  it('serve a situação na SEFAZ, com a divergência no manifesto de encerramento pendente', async () => {
    const {mdfesFixture} = await import('@/lib/mock/fixtures');
    expect(await get(`/v1.0/mdfes/${mdfesFixture[0].sk}/sefaz-status`)).toMatchObject({status: 'authorized'});
    expect(await get(`/v1.0/mdfes/${mdfesFixture[2].sk}/sefaz-status`))
      .toMatchObject({status: 'closed', access_key: mdfesFixture[2].sk});
  });

  it('com ?mock=prevalidation a emissão do MDF-e volta 422 com regras F', async () => {
    const {initMockStateFromUrl, setMockState} = await import('@/lib/mock/state');
    initMockStateFromUrl('?mock=prevalidation');
    try {
      await expect(mockAdapter({method: 'post', url: '/v1.0/mdfes', data: '{}', headers: {}} as InternalAxiosRequestConfig))
        .rejects.toMatchObject({response: {status: 422, data: {errors: [{field: 'F17'}, {field: 'F107'}]}}});
    } finally {
      setMockState({emitPrevalidation: false});
    }
  });

  it('as chaves das fixtures passam na mesma checagem que a UI faz antes de usar', async () => {
    const {mdfesFixture, nfesFixture} = await import('@/lib/mock/fixtures');
    const {ACCESS_KEY_MOD_MDFE, validateAccessKey} = await import('@/lib/utils/access-key');
    for (const m of mdfesFixture) expect(validateAccessKey(m.sk, [ACCESS_KEY_MOD_MDFE])).toEqual({valid: true});
    for (const n of nfesFixture) expect(validateAccessKey(n.sk)).toEqual({valid: true});
  });
});
