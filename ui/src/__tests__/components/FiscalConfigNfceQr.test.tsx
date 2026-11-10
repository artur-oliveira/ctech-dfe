import {afterEach, describe, expect, it, vi} from 'vitest';
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {FiscalConfigForm} from '@/components/fiscal-config/FiscalConfigForm';
import {nfceConfigFixture} from '@/lib/mock/fixtures';
import type {NFCeConfigOut} from '@/lib/types/api';

const cfg = (over: Partial<NFCeConfigOut>) => ({...nfceConfigFixture, ...over}) as unknown as NFCeConfigOut;

describe('FiscalConfigForm: versão do QR Code da NFC-e', () => {
  afterEach(cleanup);

  it('na versão 3 o CSC vira opcional e salvar sem CSC não manda os campos', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<FiscalConfigForm variant="nfce" initialData={cfg({qr_code_version: 3, prod_csc: '', prod_csc_id: 0, hom_csc: '', hom_csc_id: 0})} onSave={onSave}/>);
    expect(screen.getAllByText(/CSC \(token\) \(opcional\)/)).toHaveLength(2);
    fireEvent.click(screen.getByRole('button', {name: 'Salvar alterações'}));
    await waitFor(() => expect(onSave).toHaveBeenCalled());
    const body = onSave.mock.calls[0][0];
    expect(body.qr_code_version).toBe(3);
    expect(body).not.toHaveProperty('prod_csc');
    expect(body).not.toHaveProperty('hom_csc_id');
  });

  it('na versão 2 sem CSC de produção o formulário não salva', async () => {
    const onSave = vi.fn();
    render(<FiscalConfigForm variant="nfce" initialData={cfg({qr_code_version: 2, prod_csc: '', prod_csc_id: 0})} onSave={onSave}/>);
    await userEvent.click(screen.getByRole('button', {name: 'Salvar alterações'}));
    expect((await screen.findAllByText('CSC obrigatório')).length).toBeGreaterThan(0);
    expect(onSave).not.toHaveBeenCalled();
  });

  it('trocar para a versão 3 pelo seletor libera o CSC', async () => {
    render(<FiscalConfigForm variant="nfce" initialData={cfg({qr_code_version: 2})} onSave={vi.fn()}/>);
    expect(screen.queryByText(/\(opcional\)/)).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('combobox', {name: 'Versão do QR Code'}));
    await userEvent.click(await screen.findByRole('option', {name: 'Versão 3 (sem CSC)'}));
    expect(screen.getAllByText(/\(opcional\)/).length).toBeGreaterThan(0);
  });
});
