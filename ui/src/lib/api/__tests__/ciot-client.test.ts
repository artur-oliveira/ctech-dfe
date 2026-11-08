import {beforeEach, describe, expect, it, vi} from 'vitest';
import {apiClient} from '@/lib/api/client';
import {ciotsFixture} from '@/lib/mock/fixtures';

// Mesmo padrão de mdfe-client.test.ts: espia os wrappers privados.
const priv = apiClient as unknown as {
  get: (url: string, config?: unknown, maintenance?: boolean) => Promise<unknown>
  post: (url: string, body?: unknown) => Promise<unknown>
};

describe('ciot api client', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('lista e detalha', async () => {
    const get = vi.spyOn(priv, 'get').mockResolvedValue(ciotsFixture[0] as never);
    await apiClient.listCiots({environment: 'hom', cursor: 'c'});
    expect(get).toHaveBeenCalledWith('/v1.0/ciots', {params: {environment: 'hom', cursor: 'c'}});
    await apiClient.getCiot('a b');
    expect(get).toHaveBeenCalledWith('/v1.0/ciots/a%20b');
  });

  it('cancela, retifica e encerra pelas rotas de ação', async () => {
    const post = vi.spyOn(priv, 'post').mockResolvedValue({} as never);
    await apiClient.cancelCiot('id1', 'motivo');
    expect(post).toHaveBeenCalledWith('/v1.0/ciots/id1/cancellation', {reason: 'motivo'});
    await apiClient.closeCiot('id1', {cargo_weight: '10.00'});
    expect(post).toHaveBeenCalledWith('/v1.0/ciots/id1/closure', {cargo_weight: '10.00'});
    await apiClient.rectifyCiot('id1', {} as never);
    expect(post).toHaveBeenCalledWith('/v1.0/ciots/id1/rectification', {});
  });

  it('consultas da ANTT e piso não mandam a tela para a manutenção num 503', async () => {
    const get = vi.spyOn(priv, 'get').mockResolvedValue({} as never);
    await apiClient.getCiotCarrierStatus('52998224725', 'prod');
    expect(get).toHaveBeenLastCalledWith('/v1.0/ciot-carriers/52998224725/status', {params: {environment: 'prod'}}, false);
    await apiClient.getCiotCarrierFleet('52998224725', ['ABC1D23', 'XYZ9K87'], 'hom');
    expect(get).toHaveBeenLastCalledWith(
      '/v1.0/ciot-carriers/52998224725/fleet', {params: {plates: 'ABC1D23,XYZ9K87', environment: 'hom'}}, false);
    await apiClient.getFreightFloor({cargo_type: 1});
    expect(get).toHaveBeenLastCalledWith('/v1.0/freight-floor', {params: {cargo_type: 1}}, false);
  });
});
