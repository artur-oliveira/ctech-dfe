import {afterEach, describe, expect, it, vi} from 'vitest';
import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {EmissionSection} from '@/components/fiscal-config/EmissionSection';

const mutateAsync = vi.fn();
vi.mock('@/lib/hooks/useFiscalConfig', () => ({
  useEmissionMutation: () => ({mutateAsync, isPending: false}),
}));
vi.mock('sonner', () => ({toast: {success: vi.fn(), error: vi.fn()}}));

const REASON = 'SEFAZ do estado fora do ar desde as 10h';
const NFE_CFG = {emission_type: 'normal' as const, emission_options: ['normal', 'svc', 'epec'] as ('normal' | 'svc' | 'epec')[], auto_contingency: false};

async function pick(name: RegExp) {
  await userEvent.click(screen.getByRole('combobox', {name: /Forma de emissão atual de/}));
  await userEvent.click(await screen.findByRole('option', {name}));
}

describe('EmissionSection', () => {
  afterEach(() => {
    cleanup();
    mutateAsync.mockReset();
  });

  it('lista só as opções de emission_options, com o SVC explicado', () => {
    render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG}/>);
    fireEvent.click(screen.getByRole('combobox', {name: /Forma de emissão atual de/}));
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([
      'Normal', 'Contingência SVC (SVC-AN ou SVC-RS conforme a UF)', 'EPEC',
    ]);
  });

  it('a justificativa só aparece fora de normal e trava abaixo de 15 caracteres', async () => {
    render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG}/>);
    expect(screen.queryByLabelText('Motivo da contingência')).not.toBeInTheDocument();
    await pick(/^EPEC/);
    const field = screen.getByLabelText('Motivo da contingência');
    fireEvent.change(field, {target: {value: 'curto'}});
    expect(screen.getByText('5/256')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Aplicar forma de emissão'})).toBeDisabled();
    fireEvent.change(field, {target: {value: REASON}});
    expect(screen.getByRole('button', {name: 'Aplicar forma de emissão'})).toBeEnabled();
  });

  it('envia o tipo, a flag e a justificativa', async () => {
    mutateAsync.mockResolvedValue({});
    render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG}/>);
    await pick(/^Contingência SVC/);
    fireEvent.change(screen.getByLabelText('Motivo da contingência'), {target: {value: REASON}});
    fireEvent.click(screen.getByRole('button', {name: 'Aplicar forma de emissão'}));
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({
      emission_type: 'svc', auto_contingency: false, justification: REASON,
    }));
  });

  it('voltar a normal não pede nem envia justificativa e explica que o início é apagado', async () => {
    mutateAsync.mockResolvedValue({});
    render(<EmissionSection doc="nfe" pk="1" config={{
      ...NFE_CFG, emission_type: 'svc', contingency_since: '2026-07-02T10:00:00Z', contingency_reason: REASON,
    }}/>);
    expect(screen.getByRole('status')).toHaveTextContent(/Em contingência desde/);
    expect(screen.getByRole('status')).toHaveTextContent(REASON);
    await pick(/^Normal$/);
    expect(screen.queryByLabelText('Motivo da contingência')).not.toBeInTheDocument();
    expect(screen.getByText(/próximas notas seguem direto/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: 'Aplicar forma de emissão'}));
    await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({emission_type: 'normal', auto_contingency: false}));
  });

  it('a contingência automática aparece travada enquanto a API não a executa', () => {
    render(<EmissionSection doc="nfce" pk="1" config={{...NFE_CFG, emission_options: ['normal', 'offline']}}/>);
    expect(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/})).toBeDisabled();
    expect(screen.getByText('Disponível em breve')).toBeInTheDocument();
  });

  it('sem o campo na configuração, não mostra o interruptor', () => {
    render(<EmissionSection doc="nfe" pk="1" config={{emission_options: ['normal']}}/>);
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  it('mostra a recusa 422 da API', async () => {
    const {ApiError} = await import('@/lib/api/client');
    mutateAsync.mockRejectedValue(new ApiError(422, 'x', {code: 'emission_reason_required', detail: 'x'}));
    render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG}/>);
    await pick(/^EPEC/);
    fireEvent.change(screen.getByLabelText('Motivo da contingência'), {target: {value: REASON}});
    fireEvent.click(screen.getByRole('button', {name: 'Aplicar forma de emissão'}));
    expect(await screen.findByRole('alert')).toHaveTextContent(/Explique o motivo da contingência/);
  });

  describe('contingência automática liberada (autoAvailable)', () => {
    const NFCE = {...NFE_CFG, emission_options: ['normal', 'offline'] as ('normal' | 'offline')[]};
    const apply = () => fireEvent.click(screen.getByRole('button', {name: 'Aplicar forma de emissão'}));

    it('liga o interruptor, explica o padrão desligado e a regeração, e pede o tipo só da lista sem normal', async () => {
      render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG} autoAvailable/>);
      const toggle = screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/});
      expect(toggle).toBeEnabled();
      expect(toggle).not.toBeChecked();
      expect(screen.queryByText('Disponível em breve')).not.toBeInTheDocument();
      expect(screen.getByText(/Vem desligado/)).toBeInTheDocument();
      expect(screen.getByText(/gera a NF-e de novo/)).toBeInTheDocument();
      expect(screen.getByText(/data de início da contingência é apagada/)).toBeInTheDocument();
      expect(screen.queryByLabelText('Contingência usada na troca automática')).not.toBeInTheDocument();
      await userEvent.click(toggle);
      fireEvent.click(screen.getByRole('combobox', {name: 'Contingência usada na troca automática'}));
      expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual([
        'Contingência SVC (SVC-AN ou SVC-RS conforme a UF)', 'EPEC',
      ]);
    });

    it('com a flag ligada o tipo é obrigatório: o botão trava até escolher e o corpo leva o tipo', async () => {
      mutateAsync.mockResolvedValue({});
      render(<EmissionSection doc="nfe" pk="1" config={NFE_CFG} autoAvailable/>);
      await userEvent.click(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/}));
      expect(screen.getByRole('button', {name: 'Aplicar forma de emissão'})).toBeDisabled();
      expect(screen.getByText(/Obrigatório: escolha a contingência/)).toBeInTheDocument();
      await userEvent.click(screen.getByRole('combobox', {name: 'Contingência usada na troca automática'}));
      await userEvent.click(await screen.findByRole('option', {name: 'EPEC'}));
      apply();
      await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({
        emission_type: 'normal', auto_contingency: true, auto_contingency_type: 'epec',
      }));
    });

    it('com uma única contingência possível (NFC-e: offline) o tipo já vem escolhido', async () => {
      mutateAsync.mockResolvedValue({});
      render(<EmissionSection doc="nfce" pk="1" config={NFCE} autoAvailable/>);
      await userEvent.click(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/}));
      expect(screen.getByRole('button', {name: 'Aplicar forma de emissão'})).toBeEnabled();
      apply();
      await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({
        emission_type: 'normal', auto_contingency: true, auto_contingency_type: 'offline',
      }));
    });

    it('desligar a flag não envia o tipo', async () => {
      mutateAsync.mockResolvedValue({});
      render(<EmissionSection doc="nfce" pk="1" config={{...NFCE, auto_contingency: true, auto_contingency_type: 'offline'}} autoAvailable/>);
      await userEvent.click(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/}));
      apply();
      await waitFor(() => expect(mutateAsync).toHaveBeenCalledWith({emission_type: 'normal', auto_contingency: false}));
    });

    it('mostra a recusa auto_contingency_type_required da API', async () => {
      const {ApiError} = await import('@/lib/api/client');
      mutateAsync.mockRejectedValue(new ApiError(422, 'x', {code: 'auto_contingency_type_required', detail: 'x'}));
      render(<EmissionSection doc="nfce" pk="1" config={NFCE} autoAvailable/>);
      await userEvent.click(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/}));
      apply();
      expect(await screen.findByRole('alert')).toHaveTextContent(/Escolha a contingência que o sistema deve usar/);
    });

    // Regressão da troca de abas: a página remonta a seção por `key` (`emission-${aba}`); o estado do interruptor de
    // uma aba não pode vazar para a outra nem a seção se acumular.
    it('trocar de aba (outra key) remonta uma única seção, sem levar o interruptor da aba anterior', async () => {
      const {rerender} = render(<EmissionSection key="emission-nfe" doc="nfe" pk="1" config={NFE_CFG} autoAvailable/>);
      await userEvent.click(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/}));
      expect(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/})).toBeChecked();
      rerender(<EmissionSection key="emission-nfce" doc="nfce" pk="1" config={NFCE} autoAvailable/>);
      expect(screen.getAllByRole('region', {name: 'Forma de emissão'})).toHaveLength(1);
      expect(screen.getAllByRole('checkbox')).toHaveLength(1);
      expect(screen.getByRole('checkbox', {name: /Entrar em contingência sozinho/})).not.toBeChecked();
    });
  });
});
