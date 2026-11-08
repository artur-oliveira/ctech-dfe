import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {CarrierStatusLookup, FleetCheck} from '@/components/ciot/CarrierLookup';
import {ApiError, apiClient} from '@/lib/api/client';
import {ciotCarrierFleetFixture, ciotCarrierStatusFixture, CIOT_CARRIER_DOC} from '@/lib/mock/fixtures';

const wrap = (ui: React.ReactNode) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{ui}</QueryClientProvider>);

beforeEach(() => vi.restoreAllMocks());

describe('CarrierStatusLookup', () => {
  it('só consulta quando a pessoa pede, mostra a resposta e entrega o RNTRC lido', async () => {
    const spy = vi.spyOn(apiClient, 'getCiotCarrierStatus').mockResolvedValue(ciotCarrierStatusFixture);
    const onRntrc = vi.fn();
    wrap(<CarrierStatusLookup doc={CIOT_CARRIER_DOC} environment="prod" onRntrc={onRntrc}/>);
    expect(spy).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', {name: 'Consultar situação na ANTT'}));
    expect(await screen.findByText('Consulta realizada na ANTT')).toBeInTheDocument();
    expect(screen.getByText('Ativo')).toBeInTheDocument();
    expect(spy).toHaveBeenCalledWith(CIOT_CARRIER_DOC, 'prod');
    expect(onRntrc).toHaveBeenCalledWith('012345678');
  });

  it('o botão fica desligado sem transportador', () => {
    wrap(<CarrierStatusLookup doc="" environment="prod" onRntrc={vi.fn()}/>);
    expect(screen.getByRole('button', {name: 'Consultar situação na ANTT'})).toBeDisabled();
  });

  it('ANTT fora do ar (503) vira aviso e não bloqueia a declaração', async () => {
    vi.spyOn(apiClient, 'getCiotCarrierStatus').mockRejectedValue(new ApiError(503, 'indisponível'));
    const onRntrc = vi.fn();
    wrap(<CarrierStatusLookup doc={CIOT_CARRIER_DOC} environment="prod" onRntrc={onRntrc}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Consultar situação na ANTT'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Você pode seguir com a declaração');
    expect(onRntrc).not.toHaveBeenCalled();
  });

  it('resposta que a ANTT não confirma não é mostrada como elegível', async () => {
    vi.spyOn(apiClient, 'getCiotCarrierStatus').mockResolvedValue({cached: false, result: {codigo: 205, mensagem: 'Transportador sem RNTRC ativo'}});
    wrap(<CarrierStatusLookup doc={CIOT_CARRIER_DOC} environment="prod" onRntrc={vi.fn()}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Consultar situação na ANTT'}));
    expect(await screen.findByText('A ANTT não confirmou')).toBeInTheDocument();
    expect(screen.getByText('Transportador sem RNTRC ativo')).toBeInTheDocument();
  });

  it('avisa quando a resposta veio da guarda de 1 hora', async () => {
    vi.spyOn(apiClient, 'getCiotCarrierStatus').mockResolvedValue({...ciotCarrierStatusFixture, cached: true});
    wrap(<CarrierStatusLookup doc={CIOT_CARRIER_DOC} environment="prod" onRntrc={vi.fn()}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Consultar situação na ANTT'}));
    expect(await screen.findByText(/guardada há menos de 1 hora/)).toBeInTheDocument();
  });
});

describe('FleetCheck', () => {
  it('confere as placas escolhidas e manda todas na mesma consulta', async () => {
    const spy = vi.spyOn(apiClient, 'getCiotCarrierFleet').mockResolvedValue(ciotCarrierFleetFixture);
    wrap(<FleetCheck doc={CIOT_CARRIER_DOC} plates={['ABC1D23', 'XYZ9K87']} environment="hom"/>);
    fireEvent.click(screen.getByRole('button', {name: 'Conferir frota na ANTT'}));
    expect(await screen.findByText('Consulta realizada na ANTT')).toBeInTheDocument();
    expect(spy).toHaveBeenCalledWith(CIOT_CARRIER_DOC, ['ABC1D23', 'XYZ9K87'], 'hom');
  });

  it('sem placa ou sem transportador não consulta e explica', () => {
    const {rerender} = wrap(<FleetCheck doc="" plates={['ABC1D23']} environment="prod"/>);
    expect(screen.getByRole('button', {name: 'Conferir frota na ANTT'})).toBeDisabled();
    expect(screen.getByText('Escolha o transportador para conferir a frota.')).toBeInTheDocument();
    rerender(<QueryClientProvider client={new QueryClient()}><FleetCheck doc={CIOT_CARRIER_DOC} plates={[]} environment="prod"/></QueryClientProvider>);
    expect(screen.getByRole('button', {name: 'Conferir frota na ANTT'})).toBeDisabled();
  });

  it('mais de 7 placas não consulta', () => {
    wrap(<FleetCheck doc={CIOT_CARRIER_DOC} plates={Array.from({length: 8}, (_, i) => `AAA${i}B23`)} environment="prod"/>);
    expect(screen.getByRole('button', {name: 'Conferir frota na ANTT'})).toBeDisabled();
  });
});
