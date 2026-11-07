import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {useMdfeActions} from '@/components/mdfe/MdfeActions';
import {apiClient} from '@/lib/api/client';
import {mdfesFixture, nfesFixture} from '@/lib/mock/fixtures';
import type {NfeListOut} from '@/lib/types/api';

vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'org'}})}));

const page = <T, >(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});

function Harness({includes}: { includes: number }) {
  const {openInclude, modals} = useMdfeActions('org');
  return (
    <>
      <button type="button" onClick={() => openInclude({sk: mdfesFixture[0].sk, number: 1, includes})}>abrir</button>
      {modals}
    </>
  );
}

function pick(label: string, search: string, option: RegExp) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.change(screen.getByLabelText('Buscar município…'), {target: {value: search}});
  fireEvent.click(screen.getByRole('option', {name: option}));
}

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(apiClient, 'getNfes').mockResolvedValue(page(nfesFixture as NfeListOut[]) as never);
});

describe('Incluir NF-e (110115)', () => {
  it('escolhe as notas e os municípios e envia com o próximo nSeqEvento', async () => {
    const spy = vi.spyOn(apiClient, 'includeMdfeDFe').mockResolvedValue({} as never);
    render(<QueryClientProvider client={new QueryClient()}><Harness includes={1}/></QueryClientProvider>);
    fireEvent.click(screen.getByText('abrir'));
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByRole('button', {name: 'Incluir NF-e'})).toBeDisabled();
    pick('Município de carregamento', 'Campinas', /^Campinas ?\/ SP$/);
    pick('Município de descarregamento', 'Campinas', /^Campinas ?\/ SP$/);
    const authorized = nfesFixture.find((n) => n.status === 'authorized')!;
    fireEvent.click(await within(dialog).findByText(new RegExp(`Nº ${authorized.number} ·`)));
    fireEvent.click(within(dialog).getByRole('button', {name: 'Incluir NF-e'}));
    await vi.waitFor(() => expect(spy).toHaveBeenCalled());
    expect(spy).toHaveBeenCalledWith(mdfesFixture[0].sk, '3509502', 'Campinas', [
      {unloading_ibge_code: '3509502', unloading_city: 'Campinas', nfe_key: authorized.sk},
    ], 2);
  });
});
