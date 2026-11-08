import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {FreightFloorHint} from '@/components/ciot/FreightFloorHint';
import {ApiError, apiClient} from '@/lib/api/client';
import {ciotFloorFixture} from '@/lib/mock/fixtures';

const ready = {cargo_type: 1, axles: 5, distance_km: 430};
const wrap = (ui: React.ReactNode) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{ui}</QueryClientProvider>);

beforeEach(() => vi.restoreAllMocks());

describe('FreightFloorHint', () => {
  it('sem tipo de carga, eixos e distância nem consulta', () => {
    const spy = vi.spyOn(apiClient, 'getFreightFloor');
    wrap(<FreightFloorHint params={{cargo_type: 1}} freight="1000"/>);
    expect(screen.getByText(/informe tipo de carga, eixos e distância/)).toBeInTheDocument();
    expect(spy).not.toHaveBeenCalled();
  });

  it('503 ciot_floor_unverified mostra "indisponível" sem bloquear', async () => {
    vi.spyOn(apiClient, 'getFreightFloor').mockRejectedValue(new ApiError(503, 'não verificada', {code: 'ciot_floor_unverified'}));
    wrap(<FreightFloorHint params={ready} freight="1000"/>);
    expect(await screen.findByTestId('floor-unavailable')).toHaveTextContent('Piso mínimo de frete indisponível');
    expect(screen.getByTestId('floor-unavailable')).toHaveTextContent('A declaração segue normalmente');
  });

  it('com o piso disponível mostra o valor e avisa quando o frete está abaixo', async () => {
    vi.spyOn(apiClient, 'getFreightFloor').mockResolvedValue(ciotFloorFixture);
    wrap(<FreightFloorHint params={ready} freight="1000"/>);
    expect(await screen.findByTestId('floor-value')).toHaveTextContent('4.820,00');
    expect(screen.getByTestId('floor-value')).toHaveTextContent('abaixo do piso');
  });

  it('frete acima do piso não avisa', async () => {
    vi.spyOn(apiClient, 'getFreightFloor').mockResolvedValue(ciotFloorFixture);
    wrap(<FreightFloorHint params={ready} freight="5000"/>);
    expect(await screen.findByTestId('floor-value')).not.toHaveTextContent('abaixo do piso');
  });

  it('outro erro também não bloqueia', async () => {
    vi.spyOn(apiClient, 'getFreightFloor').mockRejectedValue(new ApiError(422, 'parâmetro'));
    wrap(<FreightFloorHint params={ready} freight="1000"/>);
    expect(await screen.findByTestId('floor-unavailable')).toHaveTextContent('Não foi possível consultar');
  });
});
