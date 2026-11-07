import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {MdfeSefazStatus, sefazDiverges} from '@/components/mdfe/MdfeSefazStatus';
import {ApiError, apiClient} from '@/lib/api/client';
import {mdfeSefazStatusClosedFixture, mdfeSefazStatusFixture} from '@/lib/mock/fixtures';

const KEY = mdfeSefazStatusFixture.access_key;

function renderStatus(local: string) {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><MdfeSefazStatus accessKey={KEY} localStatus={local}/></QueryClientProvider>);
}

beforeEach(() => vi.restoreAllMocks());

describe('sefazDiverges', () => {
  it('compara sem reconciliar', () => {
    expect(sefazDiverges('authorized', 'authorized')).toBe(false);
    expect(sefazDiverges('closed', 'close_pending')).toBe(true);
    expect(sefazDiverges('not_found', 'authorized')).toBe(true);
    expect(sefazDiverges('not_found', 'rejected')).toBe(false);
  });
});

describe('MdfeSefazStatus', () => {
  it('só consulta quando pedido e mostra status, protocolo e eventos', async () => {
    const spy = vi.spyOn(apiClient, 'getMdfeSefazStatus').mockResolvedValue(mdfeSefazStatusClosedFixture);
    renderStatus('closed');
    expect(spy).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', {name: 'Consultar na SEFAZ'}));
    expect(await screen.findByText(/cStat 132/)).toBeInTheDocument();
    expect(screen.getByText('958260000555111')).toBeInTheDocument();
    expect(screen.getByText('Encerramento')).toBeInTheDocument();
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('avisa quando a SEFAZ e o registro local discordam', async () => {
    vi.spyOn(apiClient, 'getMdfeSefazStatus').mockResolvedValue(mdfeSefazStatusClosedFixture);
    renderStatus('authorized');
    fireEvent.click(screen.getByRole('button', {name: 'Consultar na SEFAZ'}));
    expect(await screen.findByRole('status')).toHaveTextContent(/não corrige o registro/);
  });

  it('mostra não encontrado e o erro da SEFAZ', async () => {
    vi.spyOn(apiClient, 'getMdfeSefazStatus')
      .mockResolvedValueOnce({...mdfeSefazStatusFixture, status: 'not_found', c_stat: '217', protocol: null})
      .mockRejectedValueOnce(new ApiError(400, 'Rejeição: chave de acesso inválida'));
    renderStatus('rejected');
    fireEvent.click(screen.getByRole('button', {name: 'Consultar na SEFAZ'}));
    expect(await screen.findByText('Não encontrado na SEFAZ')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Consultar de novo'}));
    expect(await screen.findByRole('alert')).toHaveTextContent('Rejeição: chave de acesso inválida');
  });
});
