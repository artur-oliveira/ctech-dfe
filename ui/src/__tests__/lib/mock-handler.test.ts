import {describe, expect, it} from 'vitest';
import {mockAdapter} from '@/lib/mock/handler';
import {setMockState} from '@/lib/mock/state';
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

describe('mock handler: impressos', () => {
  const KEY = '3'.repeat(34) + '1' + '3'.repeat(9);
  const EPEC_KEY = '3'.repeat(34) + '4' + '3'.repeat(9);
  const print = (url: string, format?: string) =>
    mockAdapter({method: 'get', url, params: format ? {format} : undefined, headers: {}} as unknown as InternalAxiosRequestConfig);

  it('serve os quatro impressos com {chave}.pdf, inclusive o DACTE', async () => {
    for (const path of ['nfes/' + KEY + '/danfe', 'nfces/' + KEY + '/danfce', 'mdfes/' + KEY + '/damdfe', 'ctes/' + KEY + '/dacte']) {
      expect((await print('/v1.0/' + path)).data).toMatchObject({filename: KEY + '.pdf', content_type: 'application/pdf'});
    }
    expect((await print(`/v1.0/nfces/${KEY}/danfce`, 'mensagem')).status).toBe(200);
  });

  it('recusa formato fora da lista (400) e simplificado em EPEC (422)', async () => {
    await expect(print(`/v1.0/mdfes/${KEY}/damdfe`, 'etiqueta')).rejects.toMatchObject({response: {status: 400, data: {code: 'invalid-document-format'}}});
    await expect(print(`/v1.0/nfes/${EPEC_KEY}/danfe`, 'etiqueta')).rejects.toMatchObject({response: {status: 422, data: {code: 'document-format-not-allowed'}}});
    expect((await print(`/v1.0/nfes/${EPEC_KEY}/danfe`, 'paisagem')).status).toBe(200);
  });

  it('?mock= força 400, 422 e 404', async () => {
    for (const [printProblem, status] of [['invalidFormat', 400], ['notAllowed', 422], ['noXml', 404]] as const) {
      setMockState({printProblem});
      await expect(print(`/v1.0/ctes/${KEY}/dacte`)).rejects.toMatchObject({response: {status}});
    }
    setMockState({printProblem: null});
  });
});

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

describe('mock handler: CT-e detalhe, eventos e SEFAZ', () => {
  it('serve o detalhe de cada CT-e da lista, com notas e trajeto', async () => {
    const {ctesFixture} = await import('@/lib/mock/fixtures');
    const detail = await get(`/v1.0/ctes/${ctesFixture[3].sk}`) as {nfe_keys: string[]};
    expect(detail).toMatchObject({status: 'cancelled', number: 33});
    expect(detail.nfe_keys.length).toBeGreaterThan(0);
  });

  it('o CT-e 34 tem outro tomador e o 31 tem eventos, inclusive um desacordo autorizado', async () => {
    const {ctesFixture} = await import('@/lib/mock/fixtures');
    const other = await get(`/v1.0/ctes/${ctesFixture[5].sk}`) as {taker: {doc: string}; emit_cpf_cnpj: string};
    expect(other.taker.doc).not.toBe(other.emit_cpf_cnpj);
    const events = await get(`/v1.0/ctes/${ctesFixture[0].sk}/events`) as {items: {event_type: string}[]};
    expect(events.items.map((e) => e.event_type)).toEqual(['emission', '110110', '610110', '110180']);
  });

  it('a consulta na SEFAZ diverge no CT-e 35 e acusa chave desconhecida', async () => {
    const {ctesFixture} = await import('@/lib/mock/fixtures');
    expect(await get(`/v1.0/ctes/${ctesFixture[0].sk}/sefaz-status`)).toMatchObject({status: 'authorized'});
    expect(await get(`/v1.0/ctes/${ctesFixture[4].sk}/sefaz-status`)).toMatchObject({status: 'cancelled'});
    expect(await get('/v1.0/ctes/00000000000000000000000000000000000000000000/sefaz-status')).toMatchObject({status: 'not_found'});
  });

  it('com ?mock=prevalidation a emissão do CT-e volta 422 com regras G', async () => {
    const {initMockStateFromUrl, setMockState} = await import('@/lib/mock/state');
    initMockStateFromUrl('?mock=prevalidation');
    try {
      await expect(mockAdapter({method: 'post', url: '/v1.0/ctes', data: '{}', headers: {}} as InternalAxiosRequestConfig))
        .rejects.toMatchObject({response: {status: 422, data: {errors: [{field: 'G193'}, {field: 'G122'}]}}});
    } finally {
      setMockState({emitPrevalidation: false});
    }
  });

  it('as chaves das fixtures de CT-e passam na validação de chave (57, e 67 no CT-e OS)', async () => {
    const {ctesFixture} = await import('@/lib/mock/fixtures');
    const {validateAccessKey, ACCESS_KEY_MOD_CTE, ACCESS_KEY_MOD_CTE_OS} = await import('@/lib/utils/access-key');
    for (const c of ctesFixture) {
      expect(validateAccessKey(c.sk, ['model' in c ? ACCESS_KEY_MOD_CTE_OS : ACCESS_KEY_MOD_CTE]).valid).toBe(true);
    }
  });

  it('complemento, substituto e CT-e OS respondem 202 pending; os flags devolvem o 422 com code', async () => {
    const post = async (path: string) => mockAdapter({method: 'post', url: path, headers: {}, data: '{}'} as never)
      .then((r) => r, (e) => e.response ?? e);
    for (const path of ['complement', 'substitute', 'os']) {
      const res = await post(`/v1.0/ctes/${path}`);
      expect(res.status).toBe(202);
      expect((res.data as {status: string}).status).toBe('pending');
    }
    setMockState({cteProblem: 'window'});
    expect((await post('/v1.0/ctes/substitute')).data).toMatchObject({code: 'substitution_window_expired'});
    setMockState({cteProblem: 'osGtve'});
    expect((await post('/v1.0/ctes/os')).data).toMatchObject({code: 'os_gtve_total_mismatch'});
    setMockState({cteProblem: null});
  });
});

describe('mock handler: CIOT', () => {
  const call = (method: string, url: string, params?: object) =>
    mockAdapter({method, url, params, headers: {}} as unknown as InternalAxiosRequestConfig);

  it('lista, detalha e responde as ações com a operação em andamento', async () => {
    const list = (await get('/v1.0/ciots')) as { items: { id: string }[] };
    expect(list.items.length).toBeGreaterThanOrEqual(4);
    expect(((await get(`/v1.0/ciots/${list.items[2].id}`)) as { id: string }).id).toBe(list.items[2].id);
    const cancel = await call('post', `/v1.0/ciots/${list.items[0].id}/cancellation`);
    expect(cancel.status).toBe(202);
    expect((cancel.data as { pending_operation: string }).pending_operation).toBe('cancel');
  });

  it('declarar devolve 202 pendente; ?mock=ciot-invalid devolve 422 ciot_validation', async () => {
    const ok = await call('post', '/v1.0/ciots');
    expect(ok.status).toBe(202);
    expect((ok.data as { status: string }).status).toBe('pending');
    setMockState({ciotProblem: 'invalid'});
    try {
      await expect(call('post', '/v1.0/ciots')).rejects.toMatchObject({response: {status: 422, data: {code: 'ciot_validation'}}});
    } finally {
      setMockState({ciotProblem: null});
    }
  });

  it('o piso vem 503 ciot_floor_unverified até o `?mock=floor` calcular', async () => {
    await expect(call('get', '/v1.0/freight-floor')).rejects.toMatchObject({
      response: {status: 503, data: {code: 'ciot_floor_unverified'}},
    });
    setMockState({freightFloor: true});
    try {
      expect((await get('/v1.0/freight-floor')).value).toBe('4820.00');
    } finally {
      setMockState({freightFloor: false});
    }
  });

  it('situação e frota do transportador; ANTT fora do ar responde 503', async () => {
    expect(((await get('/v1.0/ciot-carriers/52998224725/status')) as { result: { codigo: number } }).result.codigo).toBe(111);
    expect((await get('/v1.0/ciot-carriers/52998224725/fleet')).cached).toBe(false);
    setMockState({anttDown: true});
    try {
      await expect(call('get', '/v1.0/ciot-carriers/52998224725/status')).rejects.toMatchObject({response: {status: 503}});
    } finally {
      setMockState({anttDown: false});
    }
  });
});
