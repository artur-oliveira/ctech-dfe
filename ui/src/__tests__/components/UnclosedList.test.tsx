import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {UnclosedList} from '@/components/mdfe/UnclosedList';
import {ApiError, apiClient} from '@/lib/api/client';
import {mdfeUnclosedEmptyFixture, mdfeUnclosedFixture} from '@/lib/mock/fixtures';

function renderList() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><UnclosedList orgPk="org"/></QueryClientProvider>);
}

beforeEach(() => vi.restoreAllMocks());

describe('UnclosedList', () => {
  it('lista chave, número e protocolo, e abre o encerramento de cada um', async () => {
    vi.spyOn(apiClient, 'getMdfeUnclosed').mockResolvedValue(mdfeUnclosedFixture);
    renderList();
    const list = await screen.findByRole('list', {name: 'MDF-e não encerrados'});
    expect(within(list).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByText('MDF-e nº 1')).toBeInTheDocument();
    expect(screen.getByText('958260000555333')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', {name: 'Encerrar'})[0]);
    expect(await screen.findByText('Encerrar MDF-e nº 1')).toBeInTheDocument();
  });

  it('cStat 112 é o estado vazio', async () => {
    vi.spyOn(apiClient, 'getMdfeUnclosed').mockResolvedValue(mdfeUnclosedEmptyFixture);
    renderList();
    expect(await screen.findByText('Nenhum MDF-e aberto')).toBeInTheDocument();
  });

  it('erro da SEFAZ mostra a mensagem e deixa consultar de novo', async () => {
    const spy = vi.spyOn(apiClient, 'getMdfeUnclosed')
      .mockRejectedValueOnce(new ApiError(400, 'Rejeição: serviço paralisado'))
      .mockResolvedValueOnce(mdfeUnclosedEmptyFixture);
    renderList();
    expect(await screen.findByRole('alert')).toHaveTextContent('serviço paralisado');
    fireEvent.click(screen.getByRole('button', {name: 'Consultar de novo'}));
    expect(await screen.findByText('Nenhum MDF-e aberto')).toBeInTheDocument();
    expect(spy).toHaveBeenCalledTimes(2);
  });
});
