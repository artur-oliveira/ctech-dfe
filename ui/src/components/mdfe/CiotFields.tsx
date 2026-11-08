'use client';

import {useQuery} from '@tanstack/react-query';
import {Button, Label} from '@aoctech/ui';
import {Input} from '@/components/ui/input';
import {OptionsSelect} from '@/components/ui/options-select';
import {apiClient} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {unformatCpfCnpj} from '@/lib/utils/document';
import type {CiotEnvironment, MdfeCiotIn} from '@/lib/types/api';

export const CIOT_LENGTH = 12;

/** Valor do seletor para o CIOT emitido por fora (código e responsável digitados e validados). */
export const CIOT_EXTERNAL = 'external';

/** Um CIOT está pronto quando é um declarado escolhido ou um externo com 12 dígitos. */
export const ciotComplete = (c: MdfeCiotIn) => !!c.ciot_id || c.ciot.length === CIOT_LENGTH;

const OPEN_STATUSES = ['declared', 'rectified'];

export interface CiotFieldsProps {
  ciots: MdfeCiotIn[]
  onChange: (ciots: MdfeCiotIn[]) => void
  /** Ambiente do MDF-e: só os CIOTs dele podem entrar. */
  environment: CiotEnvironment
}

/**
 * CIOT da viagem (infANTT/infCIOT, 0 a N). Cada CIOT tem 12 dígitos e o CPF ou CNPJ de
 * quem o gerou na ANTT; o responsável é escolhido entre a própria organização e as
 * pessoas do cadastro, nunca digitado. O MDF-e de transportador e o de TAC exigem CIOT
 * (regra F113 do MOC, NT 2026.001).
 */
export function CiotFields({ciots, onChange, environment}: CiotFieldsProps) {
  const {selectedOrg} = useAuth();

  const {data: page} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, 'ciot_responsible'),
    queryFn: () => apiClient.getPersons({limit: 100}),
    enabled: !!selectedOrg,
  });

  const {data: declaredPage} = useQuery({
    queryKey: queryKeys.ciots.list(selectedOrg?.pk, {environment, picker: 'mdfe'}),
    queryFn: () => apiClient.listCiots({environment, limit: 100}),
    enabled: !!selectedOrg,
  });
  // Declarados e livres (sem MDF-e), mais o que esta linha já escolheu.
  const declared = (declaredPage?.items ?? []).filter((c) => OPEN_STATUSES.includes(c.status) && !c.pending_operation);
  const sourceOptions = (current?: string) => [
    ...declared.filter((c) => !c.mdfe_access_key || c.id === current)
      .filter((c) => c.id === current || !ciots.some((x) => x.ciot_id === c.id))
      .map((c) => ({value: c.id, label: `${c.code ?? c.id} (declarado${c.freight_value ? `, frete ${c.freight_value}` : ''})`})),
    {value: CIOT_EXTERNAL, label: 'Emitido fora do sistema'},
  ];

  const orgDoc = unformatCpfCnpj(selectedOrg?.tax_id ?? selectedOrg?.cpf_or_cnpj ?? '');
  const options = [
    ...(orgDoc ? [{value: orgDoc, label: `Esta organização (${selectedOrg?.name ?? ''})`}] : []),
    ...(page?.items ?? []).map((p) => ({
      value: p.sk.startsWith(SK_PREFIX.FOREIGN) ? p.sk : unformatCpfCnpj(p.sk),
      label: p.name,
    })).filter((o) => !o.value.startsWith(SK_PREFIX.FOREIGN) && o.value !== orgDoc),
  ];

  const patch = (i: number, p: Partial<MdfeCiotIn>) =>
    onChange(ciots.map((c, k) => (k === i ? {...c, ...p} : c)));

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">CIOT (opcional)</p>
        <Button type="button" variant="ghost" size="sm"
                onClick={() => onChange([...ciots, {ciot: '', responsible_doc: orgDoc}])}>
          + CIOT
        </Button>
      </div>

      {ciots.map((c, i) => {
        const external = !c.ciot_id;
        const invalid = c.ciot !== '' && c.ciot.length !== CIOT_LENGTH;
        return (
          <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_minmax(0,1.5fr)_auto] gap-2 items-end">
            <div className="flex flex-col gap-1">
              <Label htmlFor={`ciot-src-${i}`} className="text-xs font-medium text-gray-600">Origem do CIOT</Label>
              <OptionsSelect id={`ciot-src-${i}`} value={c.ciot_id ?? CIOT_EXTERNAL} options={sourceOptions(c.ciot_id)}
                             onValueChange={(v: string) => patch(i, v === CIOT_EXTERNAL
                               ? {ciot_id: undefined, ciot: '', responsible_doc: orgDoc}
                               : {ciot_id: v, ciot: '', responsible_doc: ''})}/>
            </div>
            {external ? (
              <>
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`ciot-code-${i}`} className="text-xs font-medium text-gray-600">CIOT (12 dígitos)</Label>
                  <Input id={`ciot-code-${i}`} inputMode="numeric" maxLength={CIOT_LENGTH} value={c.ciot}
                         aria-invalid={invalid || undefined} className="w-full"
                         onChange={(e) => patch(i, {ciot: e.target.value.replace(/\D/g, '')})}/>
                  {invalid && <p className="text-xs text-red-600">O CIOT tem {CIOT_LENGTH} dígitos.</p>}
                </div>
                <div className="flex flex-col gap-1">
                  <Label htmlFor={`ciot-resp-${i}`} className="text-xs font-medium text-gray-600">
                    Responsável pela geração
                  </Label>
                  <OptionsSelect id={`ciot-resp-${i}`} value={c.responsible_doc} options={options}
                                 placeholder="Selecione"
                                 onValueChange={(doc: string) => patch(i, {responsible_doc: doc})}/>
                </div>
              </>
            ) : (
              <p className="sm:col-span-2 text-xs text-gray-500">
                O código e o responsável vêm do CIOT declarado; o CIOT fica vinculado a este MDF-e.
              </p>
            )}
            <Button type="button" variant="ghost" size="sm"
                    onClick={() => onChange(ciots.filter((_, k) => k !== i))}>
              Remover
            </Button>
          </div>
        );
      })}
    </div>
  );
}
