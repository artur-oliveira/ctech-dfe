import {afterEach, describe, expect, it, vi} from 'vitest';
import {cleanup, fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {FiscalConfigForm} from '@/components/fiscal-config/FiscalConfigForm';
import {mdfeConfigFixture, nfceConfigFixture, nfeConfigFixture, nfseConfigFixture} from '@/lib/mock/fixtures';
import type {NFCeConfigOut, NFeConfigOut, NfseConfigOut} from '@/lib/types/api';

const nfe = nfeConfigFixture as unknown as NFeConfigOut;
const nfce = (over: Partial<NFCeConfigOut> = {}) => ({...nfceConfigFixture, ...over}) as unknown as NFCeConfigOut;
const panels = () => [...document.querySelectorAll<HTMLElement>('[data-rail-panel]')];
const visible = () => panels().filter((p) => !p.hidden).map((p) => p.dataset.railPanel);
const rail = () => within(screen.getByRole('navigation', {name: 'Seções da configuração'}));

describe('SettingsRail', () => {
  afterEach(cleanup);

  it('mostra uma seção por vez e navega pelo trilho', async () => {
    render(<FiscalConfigForm variant="nfe" initialData={nfe} onSave={vi.fn()}/>);
    expect(visible()).toEqual(['ambiente']);
    await userEvent.click(rail().getByRole('button', {name: 'Numeração e série'}));
    expect(visible()).toEqual(['numeracao']);
    expect(rail().getByRole('button', {name: 'Numeração e série'})).toHaveAttribute('aria-current', 'true');
  });

  it('lista as seções reais de cada documento', () => {
    const labels = (v: Parameters<typeof FiscalConfigForm>[0]['variant'], data: unknown) => {
      render(<FiscalConfigForm variant={v} initialData={data as never} onSave={vi.fn()}/>);
      const l = [...document.querySelectorAll('#settings-rail-select option')].map((o) => o.textContent);
      cleanup();
      return l;
    };
    expect(labels('nfe', nfe)).toEqual(['Ambiente', 'Numeração e série', 'Responsável técnico']);
    expect(labels('nfce', nfce())).toContain('CSC e QR Code');
    expect(labels('mdfe', mdfeConfigFixture)).toContain('Manifesto');
    expect(labels('nfse', nfseConfigFixture)).toEqual(['Provedor e ambiente', 'Município e numeração']);
  });

  it('marca a seção alterada e a barra avisa que há alterações não salvas', async () => {
    render(<FiscalConfigForm variant="nfe" initialData={nfe} onSave={vi.fn()}/>);
    expect(screen.queryByText('Alterações não salvas')).not.toBeInTheDocument();
    await userEvent.click(rail().getByRole('button', {name: 'Responsável técnico'}));
    await userEvent.type(screen.getByLabelText('CSRT (36 caracteres)'), 'abc');
    expect(screen.getByText('Alterações não salvas')).toBeInTheDocument();
    expect(rail().getByRole('img', {name: 'alterado'})).toBeInTheDocument();
  });

  it('salvar com erro pula para a seção do erro, conta o erro e foca o campo', async () => {
    const onSave = vi.fn();
    render(<FiscalConfigForm variant="nfce" initialData={nfce({qr_code_version: 2, prod_csc: '', prod_csc_id: 0})} onSave={onSave}/>);
    expect(visible()).toEqual(['ambiente']);
    fireEvent.click(screen.getByRole('button', {name: 'Salvar alterações'}));
    await waitFor(() => expect(visible()).toEqual(['csc']));
    expect(onSave).not.toHaveBeenCalled();
    expect(rail().getByLabelText(/erros?$/)).toBeInTheDocument();
    await waitFor(() => expect(document.activeElement?.getAttribute('name')).toMatch(/_csc(_id)?$/));
  });

  it('o seletor do mobile troca a seção', () => {
    render(<FiscalConfigForm variant="nfe" initialData={nfe} onSave={vi.fn()}/>);
    fireEvent.change(screen.getByLabelText('Seção'), {target: {value: 'csrt'}});
    expect(visible()).toEqual(['csrt']);
  });

  it('a seção de emissão fica no trilho, sem barra de salvar, e a troca de chave não acumula', async () => {
    const {rerender} = render(
      <FiscalConfigForm key="form-nfe" variant="nfe" initialData={nfe} onSave={vi.fn()}
                        emission={<div key="emission-nfe" data-testid="emission">emissão nfe</div>}/>);
    await userEvent.click(rail().getByRole('button', {name: 'Forma de emissão'}));
    expect(screen.queryByRole('button', {name: 'Salvar alterações'})).not.toBeInTheDocument();
    rerender(
      <FiscalConfigForm key="form-nfce" variant="nfce" initialData={nfce()} onSave={vi.fn()}
                        emission={<div key="emission-nfce" data-testid="emission">emissão nfce</div>}/>);
    expect(screen.getAllByTestId('emission')).toHaveLength(1);
    expect(panels().filter((p) => p.dataset.railPanel === 'emissao')).toHaveLength(1);
  });

  it('documento em contingência abre na forma de emissão', () => {
    render(<FiscalConfigForm variant="nfe" initialData={nfe} onSave={vi.fn()} emissionAlert
                             emission={<div>emissão</div>}/>);
    expect(visible()).toEqual(['emissao']);
  });

  it('NFS-e: a seção ABRASF só existe com o provedor ABRASF e o erro dela é roteado', async () => {
    const abrasf = {...nfseConfigFixture, provider: 'abrasf204', abrasf: {endpoint_url: '', wsdl_version: '', municipality_code: '', synchronous: false}} as unknown as NfseConfigOut;
    render(<FiscalConfigForm variant="nfse" initialData={abrasf} onSave={vi.fn()}/>);
    expect([...document.querySelectorAll('#settings-rail-select option')].map((o) => o.textContent)).toContain('ABRASF 2.04');
    fireEvent.click(screen.getByRole('button', {name: 'Salvar alterações'}));
    await waitFor(() => expect(visible()).toEqual(['abrasf']));
  });
});
