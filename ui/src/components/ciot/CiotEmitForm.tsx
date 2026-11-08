'use client';

import {useState} from 'react';
import {useRouter} from 'next/navigation';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {toast} from 'sonner';
import {Button, Label, Modal, Switch} from '@aoctech/ui';
import {Input} from '@/components/ui/input';
import {Combobox} from '@/components/ui/combobox';
import {CurrencyInput} from '@/components/ui/currency-input';
import {EmitError} from '@/components/ui/emit-error';
import {NcmCombobox} from '@/components/ui/ncm-combobox';
import {NumericInput} from '@/components/ui/numeric-input';
import {OptionsSelect} from '@/components/ui/options-select';
import {SectionCard} from '@/components/ui/section-card';
import {RowCheckbox} from '@/components/ui/table-shell';
import {CarrierStatusLookup, FleetCheck} from '@/components/ciot/CarrierLookup';
import {FreightFloorHint} from '@/components/ciot/FreightFloorHint';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {emitFailure} from '@/lib/billing/notice';
import {
  CIOT_CARGO_TYPES,
  CIOT_ENVIRONMENTS,
  CIOT_MAX_VEHICLES,
  CIOT_OPERATION_TYPES,
  CIOT_PAYMENT_TYPES,
  OPERATION_LOTACAO,
  OPERATION_FRACIONADA,
  PAYMENT_BANK_TYPES,
  PAYMENT_PIX,
} from '@/lib/constants/ciot';
import {CITY_OPTIONS} from '@/lib/data/cities';
import {useAuth} from '@/lib/hooks/useAuth';
import {useCiotEnvironment} from '@/lib/hooks/useCiotEnvironment';
import {
  ciotDeclareSchema,
  type CiotDeclareFormData,
  emptyCiotForm,
  formFromCiot,
  isCpfCnpj,
  toDeclareBody,
  toOperation,
} from '@/lib/schemas/ciots';
import type {CiotEnvironment, CiotOut, VehicleOut} from '@/lib/types/api';
import {problemLines} from '@/lib/utils/ciot';
import {formatCpfCnpj, orgTaxId, personTaxId} from '@/lib/utils/document';

const AXLE_OPTIONS = [1, 2, 3, 4].map((n) => ({value: String(n), label: `${n} ${n === 1 ? 'eixo' : 'eixos'}`}));
const PLATE_ROLE_LABELS = {tractor: 'Trator', trailer: 'Reboque'} as const;
const LIST_LIMIT = 100;

type Errors = Record<string, string>

/** Erros do zod por caminho (`vehicles.0.axles`); o primeiro de cada campo vale. */
function errorsFrom(issues: { path: PropertyKey[]; message: string }[]): Errors {
  const out: Errors = {};
  for (const i of issues) {
    const key = i.path.map(String).join('.');
    if (!(key in out)) out[key] = i.message;
  }
  return out;
}

function Field({id, label, error, hint, children}: {
  id: string; label: string; error?: string; hint?: string; children: React.ReactNode
}) {
  return (
    <div className="flex flex-col gap-1 min-w-0">
      <Label htmlFor={id} className="text-xs font-medium text-gray-600">{label}</Label>
      {children}
      {hint && !error && <p className="text-xs text-gray-400">{hint}</p>}
      {error && <p id={`${id}-error`} role="alert" className="text-xs text-red-600">{error}</p>}
    </div>
  );
}

/**
 * Declaração de CIOT; com `rectifyOf`, a mesma tela retifica o CIOT em vigor (ambiente e MDF-e ficam como estão,
 * e a ANTT recebe o conteúdo inteiro de novo, por isso a confirmação).
 */
export function CiotEmitForm({rectifyOf}: { rectifyOf?: CiotOut }) {
  const router = useRouter();
  const qc = useQueryClient();
  const {selectedOrg} = useAuth();
  const [defaultEnv] = useCiotEnvironment();
  const [form, setForm] = useState<CiotDeclareFormData>(() => (rectifyOf ? formFromCiot(rectifyOf) : emptyCiotForm(defaultEnv)));
  const [confirm, setConfirm] = useState<CiotDeclareFormData | null>(null);
  const [touchedEnv, setTouchedEnv] = useState(false);
  const [errors, setErrors] = useState<Errors>({});
  const [failure, setFailure] = useState<ReturnType<typeof emitFailure> | null>(null);
  const [lines, setLines] = useState<string[]>([]);

  const environment: CiotEnvironment = rectifyOf ? rectifyOf.environment : touchedEnv ? form.environment : defaultEnv;
  const set = <K extends keyof CiotDeclareFormData>(key: K, value: CiotDeclareFormData[K]) =>
    setForm((f) => ({...f, [key]: value}));

  const {data: persons} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, 'ciot'),
    queryFn: () => apiClient.getPersons({limit: LIST_LIMIT}),
    enabled: !!selectedOrg,
  });
  const {data: vehicles} = useQuery({
    queryKey: ['ciot-vehicles', selectedOrg?.pk],
    queryFn: () => apiClient.getVehicles({limit: LIST_LIMIT}),
    enabled: !!selectedOrg,
  });
  const {data: mdfes} = useQuery({
    queryKey: queryKeys.mdfes.list(selectedOrg?.pk, {for: 'ciot'}),
    queryFn: () => apiClient.getMdfes({limit: LIST_LIMIT, sort: 'desc'}),
    enabled: !!selectedOrg,
  });

  // Quem pode ser transportador, contratante, destinatário ou favorecido: a organização e as pessoas com CPF/CNPJ válido.
  const orgDoc = selectedOrg ? orgTaxId(selectedOrg) : '';
  const docOptions = [
    ...(isCpfCnpj(orgDoc) ? [{value: orgDoc, label: `Esta organização (${selectedOrg?.name ?? ''})`}] : []),
    ...(persons?.items ?? [])
      .map((p) => ({doc: personTaxId(p), name: p.name}))
      .filter((p) => isCpfCnpj(p.doc) && p.doc !== orgDoc)
      .map((p) => ({value: p.doc, label: `${p.name} (${formatCpfCnpj(p.doc)})`})),
  ];
  const mdfeOptions = (mdfes?.items ?? [])
    .filter((m) => m.status === 'authorized')
    .map((m) => ({value: m.sk, label: `MDF-e nº ${m.number}: ${m.uf_start} para ${m.uf_end}`}));

  const op = Number(form.operation_type);
  const hasRoute = op === OPERATION_LOTACAO || op === OPERATION_FRACIONADA;
  const plates = form.vehicles.map((v) => v.plate);

  const toggleVehicle = (v: VehicleOut) => {
    if (form.vehicles.some((x) => x.plate === v.plate)) {
      set('vehicles', form.vehicles.filter((x) => x.plate !== v.plate));
    } else if (form.vehicles.length < CIOT_MAX_VEHICLES) {
      const tractor = v.role === 'tractor';
      set('vehicles', [...form.vehicles, {plate: v.plate, rntrc: '', axles: '2', automotor: tractor, tractor}]);
    }
  };
  const setAxles = (plate: string, axles: string) =>
    set('vehicles', form.vehicles.map((x) => (x.plate === plate ? {...x, axles} : x)));

  const maxAxles = Math.max(0, ...form.vehicles.map((v) => Number(v.axles)));
  const floorParams = {
    cargo_type: Number(form.cargo_type) || undefined,
    axles: maxAxles || undefined,
    distance_km: Number(form.distance_km) || undefined,
    empty_return: form.empty_return,
  };

  const declare = useMutation({
    mutationFn: (data: CiotDeclareFormData) => rectifyOf
      ? apiClient.rectifyCiot(rectifyOf.id, toOperation(data))
      : apiClient.declareCiot(toDeclareBody({...data, environment})),
    onSuccess: (ciot) => {
      void qc.invalidateQueries({queryKey: queryKeys.ciots.lists(selectedOrg?.pk)});
      void qc.invalidateQueries({queryKey: queryKeys.ciots.detail(ciot.id)});
      toast.success(rectifyOf
        ? 'Retificação enviada à ANTT. O resultado aparece aqui e por aviso.'
        : 'Declaração enviada à ANTT. O resultado aparece aqui e por aviso.');
      router.push(`/ciot/detail?id=${encodeURIComponent(ciot.id)}`);
    },
    onError: (e) => {
      setConfirm(null);
      setFailure(emitFailure(e, rectifyOf ? 'Erro ao retificar o CIOT.' : 'Erro ao declarar o CIOT.'));
      setLines(problemLines(e));
    },
  });

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    setFailure(null);
    setLines([]);
    const parsed = ciotDeclareSchema.safeParse({...form, environment});
    if (!parsed.success) {
      setErrors(errorsFrom(parsed.error.issues));
      toast.error('Confira os campos destacados.');
      // Depois do render dos erros: leva o primeiro à vista, que no celular costuma estar fora da tela.
      requestAnimationFrame(() => document.querySelector('p[role="alert"]')?.scrollIntoView?.({block: 'center'}));
      return;
    }
    setErrors({});
    if (rectifyOf) setConfirm(parsed.data);
    else declare.mutate(parsed.data);
  };

  return (
    <form onSubmit={submit} noValidate className="space-y-6 max-w-3xl pb-24">
      <SectionCard title="Operação">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field id="ciot-env" label="Ambiente">
            <OptionsSelect id="ciot-env" value={environment} options={CIOT_ENVIRONMENTS} disabled={!!rectifyOf}
                           onValueChange={(v) => { setTouchedEnv(true); set('environment', v as CiotEnvironment); }}/>
          </Field>
          <Field id="ciot-op" label="Tipo de operação" error={errors.operation_type}>
            <OptionsSelect id="ciot-op" value={form.operation_type} options={CIOT_OPERATION_TYPES}
                           onValueChange={(v) => set('operation_type', v as CiotDeclareFormData['operation_type'])}/>
          </Field>
        </div>
        {!rectifyOf && (
          <Field id="ciot-mdfe" label="MDF-e da viagem (opcional)" error={errors.mdfe_access_key}
                 hint="Vincula o CIOT a um manifesto autorizado.">
            <OptionsSelect id="ciot-mdfe" value={form.mdfe_access_key} options={mdfeOptions} clearable
                           placeholder="Sem vínculo" onValueChange={(v) => set('mdfe_access_key', v)}/>
          </Field>
        )}
      </SectionCard>

      <SectionCard title="Transportador (contratado)">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          <Field id="ciot-carrier" label="Transportador" error={errors.carrier_doc}>
            <OptionsSelect id="ciot-carrier" value={form.carrier_doc} options={docOptions} placeholder="Selecione"
                           onValueChange={(v) => setForm((f) => ({...f, carrier_doc: v, credited_doc: f.credited_doc || v}))}/>
          </Field>
          <Field id="ciot-rntrc" label="RNTRC" error={errors.carrier_rntrc} hint="8 ou 9 dígitos">
            <NumericInput id="ciot-rntrc" integerPlaces={9} value={form.carrier_rntrc} className="w-full h-11"
                          aria-invalid={!!errors.carrier_rntrc || undefined}
                          onChange={(v) => set('carrier_rntrc', v)}/>
          </Field>
        </div>
        <CarrierStatusLookup doc={form.carrier_doc} environment={environment}
                             onRntrc={(r) => setForm((f) => (f.carrier_rntrc ? f : {...f, carrier_rntrc: r}))}/>
      </SectionCard>

      <SectionCard title="Contratante e destinatário">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field id="ciot-hirer" label="Contratante" error={errors.hirer_doc}>
            <OptionsSelect id="ciot-hirer" value={form.hirer_doc} options={docOptions} placeholder="Selecione"
                           onValueChange={(v) => set('hirer_doc', v)}/>
          </Field>
          {hasRoute && (
            <Field id="ciot-recipient" label="Destinatário" error={errors.recipient_doc}>
              <OptionsSelect id="ciot-recipient" value={form.recipient_doc} options={docOptions} placeholder="Selecione"
                             onValueChange={(v) => set('recipient_doc', v)}/>
            </Field>
          )}
        </div>
      </SectionCard>

      <SectionCard title="Veículos">
        <p className="text-xs text-gray-500">
          Escolha até {CIOT_MAX_VEHICLES} veículos do cadastro. O trator conta como automotor e cavalo-trator.
        </p>
        {(vehicles?.items ?? []).length === 0 ? (
          <p className="text-sm text-gray-500">Nenhum veículo cadastrado. Cadastre em Veículos.</p>
        ) : (
          <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200">
            {(vehicles?.items ?? []).map((v) => {
              const picked = form.vehicles.find((x) => x.plate === v.plate);
              return (
                <li key={v.sk} className="flex flex-col gap-2 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between">
                  <label className="flex min-h-11 items-center gap-3 text-sm sm:min-h-0">
                    <RowCheckbox checked={!!picked} onChange={() => toggleVehicle(v)} ariaLabel={`Usar o veículo ${v.plate}`}/>
                    <span className="font-mono font-medium">{v.plate}</span>
                    <span className="text-xs text-gray-500">{PLATE_ROLE_LABELS[v.role]} · {v.plate_uf}</span>
                  </label>
                  {picked && (
                    <div className="w-full sm:w-40">
                      <OptionsSelect ariaLabel={`Eixos de ${v.plate}`} value={picked.axles} options={AXLE_OPTIONS}
                                     onValueChange={(a) => setAxles(v.plate, a)}/>
                      {errors[`vehicles.${form.vehicles.indexOf(picked)}.axles`] && (
                        <p role="alert" className="text-xs text-red-600">{errors[`vehicles.${form.vehicles.indexOf(picked)}.axles`]}</p>
                      )}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        {errors.vehicles && <p role="alert" className="text-xs text-red-600">{errors.vehicles}</p>}
        <FleetCheck doc={form.carrier_doc} plates={plates} environment={environment}/>
      </SectionCard>

      <SectionCard title="Viagem e carga">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {hasRoute && (
            <Field id="ciot-start" label="Início da viagem" error={errors.trip_start}>
              <Input id="ciot-start" type="date" className="h-11 w-full" value={form.trip_start}
                     onChange={(e) => set('trip_start', e.target.value)}/>
            </Field>
          )}
          <Field id="ciot-end" label="Fim da viagem" error={errors.trip_end}>
            <Input id="ciot-end" type="date" className="h-11 w-full" value={form.trip_end}
                   onChange={(e) => set('trip_end', e.target.value)}/>
          </Field>
        </div>
        {hasRoute && (
          <>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_8rem]">
              <Field id="ciot-origin" label="Município de origem" error={errors.origin_ibge}>
                <Combobox id="ciot-origin" value={form.origin_ibge} options={CITY_OPTIONS} placeholder="Selecione o município"
                          searchPlaceholder="Buscar município…" onValueChange={(v) => set('origin_ibge', v)}/>
              </Field>
              <Field id="ciot-destination" label="Município de destino" error={errors.destination_ibge}>
                <Combobox id="ciot-destination" value={form.destination_ibge} options={CITY_OPTIONS} placeholder="Selecione o município"
                          searchPlaceholder="Buscar município…" onValueChange={(v) => set('destination_ibge', v)}/>
              </Field>
              <Field id="ciot-distance" label="Distância (km)">
                <NumericInput id="ciot-distance" integerPlaces={5} value={form.distance_km} className="w-full h-11"
                              onChange={(v) => set('distance_km', v)}/>
              </Field>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <Field id="ciot-ncm" label="Natureza da carga (NCM)" error={errors.cargo_ncm}
                     hint={form.cargo_ncm.length === 4 ? `Natureza atual: ${form.cargo_ncm}. Escolha outro NCM só para trocar.` : 'Vão os 4 primeiros dígitos.'}>
                <NcmCombobox id="ciot-ncm" value={form.cargo_ncm} onValueChange={(v) => set('cargo_ncm', v)}/>
              </Field>
              <Field id="ciot-weight" label="Peso (kg)" error={errors.cargo_weight}>
                <NumericInput id="ciot-weight" decimal decimalPlaces={2} integerPlaces={7} value={form.cargo_weight}
                              className="w-full h-11" onChange={(v) => set('cargo_weight', v)}/>
              </Field>
              <Field id="ciot-cargo-type" label="Tipo de carga" error={errors.cargo_type}>
                <OptionsSelect id="ciot-cargo-type" value={form.cargo_type} options={CIOT_CARGO_TYPES}
                               onValueChange={(v) => set('cargo_type', v)}/>
              </Field>
            </div>
          </>
        )}
        {op === OPERATION_LOTACAO && (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {([
              ['high_performance', 'Alto desempenho'],
              ['empty_return', 'Retorno vazio'],
              ['vehicle_composition', 'Composição veicular'],
            ] as const).map(([key, label]) => (
              <label key={key} className="flex min-h-11 items-center gap-2 text-sm text-gray-700 sm:min-h-0">
                <Switch checked={form[key]} onCheckedChange={(c: boolean) => set(key, c)} aria-label={label}/>
                {label}
              </label>
            ))}
          </div>
        )}
      </SectionCard>

      <SectionCard title="Frete e pagamento">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Field id="ciot-freight" label="Valor do frete" error={errors.freight_value}>
            <CurrencyInput id="ciot-freight" value={form.freight_value} className="w-full h-11"
                           allowZero={false} onChange={(v) => set('freight_value', v)}/>
          </Field>
          <Field id="ciot-payment" label="Meio de pagamento" error={errors.payment_type}>
            <OptionsSelect id="ciot-payment" value={form.payment_type} options={CIOT_PAYMENT_TYPES}
                           onValueChange={(v) => set('payment_type', v as CiotDeclareFormData['payment_type'])}/>
          </Field>
        </div>
        {hasRoute && <FreightFloorHint params={floorParams} freight={form.freight_value}/>}
        <Field id="ciot-credited" label="Favorecido do pagamento" error={errors.credited_doc}>
          <OptionsSelect id="ciot-credited" value={form.credited_doc} options={docOptions} placeholder="Selecione"
                         onValueChange={(v) => set('credited_doc', v)}/>
        </Field>
        {Number(form.payment_type) === PAYMENT_PIX && (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <Field id="ciot-pix-key" label="Chave Pix" error={errors.pix_key}>
              <Input id="ciot-pix-key" className="h-11 w-full" value={form.pix_key} maxLength={77}
                     onChange={(e) => set('pix_key', e.target.value)}/>
            </Field>
            <Field id="ciot-pix-id" label="Identificador do Pix" error={errors.pix_id}>
              <Input id="ciot-pix-id" className="h-11 w-full" value={form.pix_id} maxLength={35}
                     onChange={(e) => set('pix_id', e.target.value)}/>
            </Field>
          </div>
        )}
        {PAYMENT_BANK_TYPES.includes(Number(form.payment_type)) && (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <Field id="ciot-bank" label="Banco (código)" error={errors.account && !form.bank_code ? 'Informe o banco' : undefined}>
              <NumericInput id="ciot-bank" integerPlaces={3} value={form.bank_code} className="w-full h-11"
                            onChange={(v) => set('bank_code', v)}/>
            </Field>
            <Field id="ciot-branch" label="Agência" error={errors.account && !form.branch ? 'Informe a agência' : undefined}>
              <NumericInput id="ciot-branch" integerPlaces={6} value={form.branch} className="w-full h-11"
                            onChange={(v) => set('branch', v)}/>
            </Field>
            <Field id="ciot-account" label="Conta" error={errors.account && !form.account ? 'Informe a conta' : undefined}>
              <Input id="ciot-account" className="h-11 w-full" value={form.account} maxLength={20}
                     onChange={(e) => set('account', e.target.value)}/>
            </Field>
          </div>
        )}
      </SectionCard>

      <EmitError failure={failure}/>
      {lines.length > 0 && (
        <ul role="alert" className="list-disc space-y-1 rounded-lg border border-red-200 bg-red-50 py-3 pl-8 pr-4 text-sm text-danger">
          {lines.map((l) => <li key={l}>{l}</li>)}
        </ul>
      )}

      <div className="sticky bottom-(--bottomnav-height) -mx-4 flex flex-col gap-2 border-t border-gray-200 bg-gray-50 px-4 py-3 sm:flex-row sm:justify-end md:-mx-8 md:px-8">
        <Button type="button" variant="outline" className="min-h-11 sm:min-h-0" onClick={() => router.push('/ciot')}>
          Cancelar
        </Button>
        <Button type="submit" variant="brand" className="min-h-11 sm:min-h-0" disabled={declare.isPending}>
          {declare.isPending ? 'Enviando…' : rectifyOf ? 'Retificar CIOT' : 'Declarar CIOT'}
        </Button>
      </div>

      <Modal
        open={confirm !== null}
        title="Retificar o CIOT?"
        onClose={() => setConfirm(null)}
        onSubmit={() => confirm && declare.mutate(confirm)}
        submitLabel="Confirmar retificação"
        cancelLabel="Voltar"
        loading={declare.isPending}
      >
        <p className="text-sm text-gray-600">
          A ANTT recebe o conteúdo inteiro de novo e o CIOT segue o mesmo número. Confira o transportador, os veículos e o frete antes de confirmar.
        </p>
      </Modal>
    </form>
  );
}
