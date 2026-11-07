import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {MdfeEmitForm} from '@/components/mdfe/MdfeEmitForm';
import {ApiError, apiClient} from '@/lib/api/client';
import {mdfeFiscalPrevalidationFixture, vehiclesFixture} from '@/lib/mock/fixtures';
import type {MdfeEmit, PersonItemOut} from '@/lib/types/api';

const push = vi.fn();
vi.mock('next/navigation', () => ({useRouter: () => ({push})}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: '11222333000181', tax_id: '11222333000181'}})}));
// O picker real busca na api; aqui ele devolve um condutor ou uma pessoa do cadastro.
vi.mock('@/components/persons/PersonPicker', () => ({
  PersonPicker: ({onChange, role}: { onChange: (p: PersonItemOut) => void; role?: string }) => (
    <button type="button" onClick={() => onChange((role === 'driver'
      ? {sk: 'CPF_52998224725', name: 'João Motorista'}
      : {sk: 'CNPJ_12ABC34501DE35', name: 'Contabilidade'}) as PersonItemOut)}>
      {role === 'driver' ? 'pick driver' : 'pick person'}
    </button>
  ),
}));

const page = <T, >(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(apiClient, 'getMDFeConfig').mockResolvedValue({environment: 2, ind_carrega_posterior: false} as never);
  vi.spyOn(apiClient, 'getNfes').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getVehicles').mockImplementation(async (p) =>
    page(vehiclesFixture.filter((v) => v.role === p?.role)) as never);
  vi.spyOn(apiClient, 'getVehicleSets').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getVehicleRequirements').mockResolvedValue({missing: []});
  vi.spyOn(apiClient, 'getPersons').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getTollProviders').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getInsurancePolicies').mockResolvedValue(page([]) as never);
  vi.spyOn(apiClient, 'getCargoUnits').mockResolvedValue(page([]) as never);
});

function renderForm() {
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><MdfeEmitForm/></QueryClientProvider>);
}

const next = () => fireEvent.click(screen.getByRole('button', {name: 'Próximo'}));

function pickOption(trigger: HTMLElement, search: string, option: RegExp) {
  fireEvent.click(trigger);
  const box = screen.getAllByRole('textbox').find((t) => t.getAttribute('aria-label')?.startsWith('Buscar'))!;
  fireEvent.change(box, {target: {value: search}});
  fireEvent.click(screen.getByRole('option', {name: option}));
}

async function laterLoadingUntilVehicle() {
  renderForm();
  fireEvent.click(await screen.findByRole('switch'));
  // Carregamento posterior é só rodoviário (F23).
  expect(screen.getByRole('button', {name: /Aéreo/})).toBeDisabled();
  next();
  // Sem o passo Documentos: a carga pede o município único.
  expect(screen.getByText(/o manifesto sai sem notas/)).toBeInTheDocument();
  pickOption(screen.getByRole('combobox'), 'Campinas', /^Campinas ?\/ SP$/);
  next();
  expect(screen.getByText('Campinas / SP')).toBeInTheDocument();
  // F24: só carga própria.
  expect(screen.getByRole('radio', {name: /3\. Prestador que emitirá/})).toBeDisabled();
  expect(screen.getByRole('radio', {name: /2\. Transportador de carga própria/})).toBeChecked();
  fireEvent.click(screen.getByText('pick person'));
  next();
  pickOption((await screen.findByText('Selecione um veículo')).closest('button')!, 'ABC', /ABC1D23/);
  fireEvent.click(screen.getByText('pick driver'));
}

async function emit() {
  fireEvent.click(screen.getByRole('button', {name: 'Emitir MDF-e'}));
  const dialog = await screen.findByRole('dialog');
  fireEvent.click(within(dialog).getByRole('button', {name: 'Confirmar e emitir'}));
}

describe('MdfeEmitForm: carregamento posterior', () => {
  it('emite sem documentos, com um município igual na carga e na descarga e tipo 2', async () => {
    const emitSpy = vi.spyOn(apiClient, 'emitMdfe').mockResolvedValue({} as never);
    await laterLoadingUntilVehicle();
    await emit();
    await vi.waitFor(() => expect(emitSpy).toHaveBeenCalled());
    const body = emitSpy.mock.calls[0][0] as MdfeEmit;
    expect(body).toMatchObject({
      modal: 'rodoviario',
      later_loading: true,
      emitter_type: '2',
      documents: [],
      loadings: [{ibge_code: '3509502', city: 'Campinas'}],
      unloadings: [{ibge_code: '3509502', city: 'Campinas'}],
      uf_start: 'SP',
      uf_end: 'SP',
      authorized_xml: ['12ABC34501DE35'],
    });
    expect(body.bulk_cargo).toBeUndefined();
  });

  it('recusa local do MOC aparece no campo e na lista, com o atalho para corrigir', async () => {
    vi.spyOn(apiClient, 'emitMdfe').mockRejectedValue(
      new ApiError(422, mdfeFiscalPrevalidationFixture.detail, mdfeFiscalPrevalidationFixture));
    await laterLoadingUntilVehicle();
    await emit();
    expect(await screen.findByText(/Regra F17 do MOC; a SEFAZ rejeitaria o manifesto/)).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', {name: 'Corrigir'})[0]);
    // Volta ao Trajeto, onde ficam o tipo de emitente e os autorizados ao XML.
    expect(screen.getByText(/CT-e globalizado \(3\) vale só para viagem dentro de um estado/)).toBeInTheDocument();
    expect(screen.getByText(/aparece duas vezes/)).toBeInTheDocument();
  });
});
