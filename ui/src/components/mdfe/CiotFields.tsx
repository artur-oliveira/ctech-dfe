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
import type {MdfeCiotIn} from '@/lib/types/api';

export const CIOT_LENGTH = 12;

export interface CiotFieldsProps {
  ciots: MdfeCiotIn[]
  onChange: (ciots: MdfeCiotIn[]) => void
}

/**
 * CIOT da viagem (infANTT/infCIOT, 0 a N). Cada CIOT tem 12 dígitos e o CPF ou CNPJ de
 * quem o gerou na ANTT; o responsável é escolhido entre a própria organização e as
 * pessoas do cadastro, nunca digitado. O MDF-e de transportador e o de TAC exigem CIOT
 * (regra F113 do MOC, NT 2026.001).
 */
export function CiotFields({ciots, onChange}: CiotFieldsProps) {
  const {selectedOrg} = useAuth();

  const {data: page} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, 'ciot_responsible'),
    queryFn: () => apiClient.getPersons({limit: 100}),
    enabled: !!selectedOrg,
  });

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
        const invalid = c.ciot !== '' && c.ciot.length !== CIOT_LENGTH;
        return (
          <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto] gap-2 items-end">
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
