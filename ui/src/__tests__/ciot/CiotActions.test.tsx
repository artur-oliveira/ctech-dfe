import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useCiotActions} from '@/components/ciot/CiotActions';
import {ApiError, apiClient} from '@/lib/api/client';

vi.mock('sonner', () => ({toast: {success: vi.fn(), error: vi.fn(), info: vi.fn()}}));

const LOTACAO = {id: 'id1', ciot: 'ABC1234567890123', operation_type: 1, trip_start: '2099-01-10'};
const TAC = {id: 'id2', ciot: undefined, operation_type: 3, trip_start: undefined};

function Harness({target}: { target: typeof LOTACAO | typeof TAC }) {
  const {openCancel, openClose, modals} = useCiotActions('org');
  return (
    <>
      <button onClick={() => openCancel(target)}>abrir cancelar</button>
      <button onClick={() => openClose(target)}>abrir encerrar</button>
      {modals}
    </>
  );
}

const renderHarness = (target: typeof LOTACAO | typeof TAC) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}><Harness target={target}/></QueryClientProvider>);

beforeEach(() => vi.restoreAllMocks());

describe('cancelar CIOT', () => {
  it('mostra o aviso das 24 horas e só habilita com o motivo', async () => {
    renderHarness(LOTACAO);
    fireEvent.click(screen.getByText('abrir cancelar'));
    expect(await screen.findByText('Cancelar o CIOT ABC1234567890123')).toBeInTheDocument();
    expect(screen.getByText(/só vale até 24 horas depois do início da viagem/)).toBeInTheDocument();
    expect(screen.getByText(/^Prazo:/)).toBeInTheDocument();
    const confirm = screen.getByRole('button', {name: 'Confirmar cancelamento'});
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Motivo'), {target: {value: 'Viagem desfeita'}});
    expect(confirm).toBeEnabled();
  });

  it('prazo vencido avisa, mas deixa tentar (a ANTT decide)', async () => {
    renderHarness({...LOTACAO, trip_start: '2020-01-10'});
    fireEvent.click(screen.getByText('abrir cancelar'));
    expect(await screen.findByText(/O prazo parece ter passado/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Motivo'), {target: {value: 'Tentativa'}});
    expect(screen.getByRole('button', {name: 'Confirmar cancelamento'})).toBeEnabled();
  });

  it('envia o motivo sem espaços sobrando', async () => {
    const spy = vi.spyOn(apiClient, 'cancelCiot').mockResolvedValue({} as never);
    renderHarness(LOTACAO);
    fireEvent.click(screen.getByText('abrir cancelar'));
    fireEvent.change(await screen.findByLabelText('Motivo'), {target: {value: '  Viagem desfeita  '}});
    fireEvent.click(screen.getByRole('button', {name: 'Confirmar cancelamento'}));
    await waitFor(() => expect(spy).toHaveBeenCalledWith('id1', 'Viagem desfeita'));
  });

  it('mostra a recusa da api dentro do diálogo', async () => {
    vi.spyOn(apiClient, 'cancelCiot').mockRejectedValue(new ApiError(409, 'Já existe uma operação em andamento'));
    renderHarness(LOTACAO);
    fireEvent.click(screen.getByText('abrir cancelar'));
    fireEvent.change(await screen.findByLabelText('Motivo'), {target: {value: 'Motivo'}});
    fireEvent.click(screen.getByRole('button', {name: 'Confirmar cancelamento'}));
    expect(await screen.findByText('Já existe uma operação em andamento')).toBeInTheDocument();
  });
});

describe('encerrar CIOT', () => {
  it('carga lotação pede o peso (B65) antes de liberar', async () => {
    const spy = vi.spyOn(apiClient, 'closeCiot').mockResolvedValue({} as never);
    renderHarness(LOTACAO);
    fireEvent.click(screen.getByText('abrir encerrar'));
    const confirm = await screen.findByRole('button', {name: 'Confirmar encerramento'});
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Peso da carga (kg)'), {target: {value: '23500.5'}});
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    await waitFor(() => expect(spy).toHaveBeenCalledWith('id1', {cargo_weight: '23500.5'}));
  });

  it('TAC-Agregado pede viagens e trecho em vez do peso', async () => {
    renderHarness(TAC);
    fireEvent.click(screen.getByText('abrir encerrar'));
    expect(await screen.findByLabelText('Quantidade de viagens')).toBeInTheDocument();
    expect(screen.queryByLabelText('Peso da carga (kg)')).toBeNull();
    expect(screen.getByRole('button', {name: 'Confirmar encerramento'})).toBeDisabled();
  });

  it('carga fracionada encerra sem campos extras', async () => {
    renderHarness({...LOTACAO, operation_type: 2});
    fireEvent.click(screen.getByText('abrir encerrar'));
    expect(await screen.findByRole('button', {name: 'Confirmar encerramento'})).toBeEnabled();
  });
});
