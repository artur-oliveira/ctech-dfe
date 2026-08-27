'use client'

import {useQuery} from '@tanstack/react-query'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Input} from '@/components/ui/input'
import {OptionsSelect} from '@/components/ui/options-select'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import type {MdfeInsuranceIn} from '@/lib/types/api'

export interface InsurancePoliciesFieldsProps {
  policies: MdfeInsuranceIn[]
  onChange: (policies: MdfeInsuranceIn[]) => void
}

/** Averbações são digitadas numa linha só; o leiaute quer uma lista de nAver. */
const splitAver = (raw: string): string[] =>
  raw.split(',').map((s) => s.trim()).filter(Boolean)

/**
 * Seguro da carga (infMDFe/seg). Responsável, seguradora e número da apólice
 * vivem no cadastro; aqui entra só o que muda a cada viagem — qual apólice e
 * as averbações emitidas para ela.
 */
export function InsurancePoliciesFields({policies, onChange}: InsurancePoliciesFieldsProps) {
  const {selectedOrg} = useAuth()

  const {data: page} = useQuery({
    queryKey: queryKeys.insurancePolicies.list(selectedOrg?.pk),
    queryFn: () => apiClient.getInsurancePolicies({limit: 100}),
    enabled: !!selectedOrg,
  })
  const registered = page?.items ?? []

  const patch = (i: number, p: Partial<MdfeInsuranceIn>) =>
    onChange(policies.map((v, k) => (k === i ? {...v, ...p} : v)))

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Seguro da carga</p>
        <Button type="button" variant="ghost" size="xs" disabled={registered.length === 0}
                onClick={() => onChange([...policies, {insurance_policy_id: '', n_aver: []}])}>
          + Apólice
        </Button>
      </div>

      {registered.length === 0 ? (
        <p className="text-xs text-gray-500">
          Cadastre uma apólice em <span className="font-medium">Cadastros → Apólices de seguro</span> para
          informar o seguro aqui.
        </p>
      ) : (
        policies.length === 0 && (
          <p className="text-xs text-gray-500">
            Exigido no MDF-e de CT-e. A apólice guarda responsável e seguradora; por viagem entram só
            as averbações.
          </p>
        )
      )}

      {policies.map((v, i) => (
        <div key={i} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
          <div className="flex flex-col gap-1">
            <Label htmlFor={`seg-policy-${i}`} className="text-xs font-medium text-gray-600">Apólice</Label>
            <OptionsSelect id={`seg-policy-${i}`} value={v.insurance_policy_id}
                           placeholder="Selecione"
                           onValueChange={(id: string) => patch(i, {insurance_policy_id: id})}
                           options={registered.map((p) => ({
                             value: extractId(p.sk, SK_PREFIX.INSURANCE_POLICY),
                             label: p.name,
                           }))}/>
          </div>
          <div className="flex flex-col gap-1">
            <Label htmlFor={`seg-aver-${i}`} className="text-xs font-medium text-gray-600">
              Averbações (separadas por vírgula)
            </Label>
            <Input id={`seg-aver-${i}`} className="w-full" placeholder="AV-1, AV-2"
                   defaultValue={(v.n_aver ?? []).join(', ')}
                   onChange={(e) => patch(i, {n_aver: splitAver(e.target.value)})}/>
          </div>
          <Button type="button" variant="ghost" size="xs"
                  onClick={() => onChange(policies.filter((_, k) => k !== i))}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}
