'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {Combobox} from '@/components/ui/combobox'
import {deriveTaxGroups, EMPTY_TAX_GROUPS, TaxFieldsEditor, type TaxGroups} from '@/components/tax/TaxFieldsEditor'
import {UfOverridesEditor} from '@/components/tax/UfOverridesEditor'
import {type TaxProfileFormData, taxProfileSchema} from '@/lib/schemas/tax-profiles'
import type {CfopConfigFormData} from '@/lib/schemas/products'
import type {TaxProfileCreate, TaxProfileItemOut} from '@/lib/types/api'
import {getAllCfopOptionsFlat} from '@/lib/data/cfop'
import {isRegimeSimples} from '@/lib/constants/tax'
import {ApiError} from '@/lib/api/client'

interface TaxProfileFormProps {
  initialData?: TaxProfileItemOut
  /** CRT da organização — decide entre CSOSN e CST de ICMS. */
  crt?: number | string
  onSubmit: (data: TaxProfileCreate) => Promise<void>
  loading?: boolean
}

const EMPTY_TAX_FIELDS: CfopConfigFormData = {
  cfop: '', csosn: '', icms: '', pis: '', cofins: '',
  ibs_cbs_cst: '', ibs_cbs_class_trib: '', ibs_uf_aliq: '', ibs_mun_aliq: '', cbs_aliq: '',
} as CfopConfigFormData

function toFormData(p: TaxProfileItemOut): TaxProfileFormData {
  // O item vem do DynamoDB com os campos de tributação no nível de cima, do
  // mesmo jeito que uma entrada de cfop_config — é o mesmo TaxFieldsBody.
  const {pk, sk, created_at, updated_at, ...rest} = p
  void pk; void sk; void created_at; void updated_at
  return {
    ...(rest as unknown as TaxProfileFormData),
    description: p.description ?? '',
    cfops: p.cfops ?? [],
  }
}

/**
 * Cadastro de perfil fiscal. Reusa o mesmo TaxFieldsEditor do produto — a única
 * diferença é que aqui o CFOP não é um campo da linha, e sim uma lista: o
 * perfil vale para todos os CFOPs escolhidos.
 */
export function TaxProfileForm({initialData, crt = 3, onSubmit, loading = false}: TaxProfileFormProps) {
  const simples = isRegimeSimples(crt)
  const cfopOptions = getAllCfopOptionsFlat()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [taxGroups, setTaxGroups] = useState<TaxGroups>(
    () => initialData ? deriveTaxGroups(toFormData(initialData)) : EMPTY_TAX_GROUPS,
  )

  const form = useForm<TaxProfileFormData>({
    resolver: zodResolver(taxProfileSchema),
    defaultValues: initialData
      ? toFormData(initialData)
      : {...(EMPTY_TAX_FIELDS as unknown as TaxProfileFormData), name: '', description: '', cfops: []},
  })

  const cfops = useWatch({control: form.control, name: 'cfops'}) ?? []

  // O combobox lista cada variante (5xxx/6xxx/7xxx) como opção própria — o perfil
  // pode cobrir só um CFOP específico (ex.: só o 6102) ou vários, um de cada vez.
  // Cada CFOP adicionado é um chip independente: remover um não afeta os outros,
  // mesmo que pertençam ao mesmo grupo (ex.: 5920 e 6920 adicionados separadamente).
  const addCfop = (cfop: string) => {
    if (cfops.includes(cfop)) return
    form.setValue('cfops', [...cfops, cfop], {shouldValidate: true})
  }

  const removeCfop = (cfop: string) => {
    form.setValue('cfops', cfops.filter((c) => c !== cfop), {shouldValidate: true})
  }

  // O TaxFieldsEditor edita a linha inteira; aqui a "linha" é o próprio
  // formulário menos nome/descrição/cfops.
  const taxValue = useWatch({control: form.control}) as unknown as CfopConfigFormData
  const setTaxValue = (updater: (r: CfopConfigFormData) => CfopConfigFormData) => {
    const next = updater(form.getValues() as unknown as CfopConfigFormData) as unknown as TaxProfileFormData
    for (const [key, value] of Object.entries(next)) {
      form.setValue(key as keyof TaxProfileFormData, value as never)
    }
  }

  const handleSubmit = async (data: TaxProfileFormData) => {
    setSubmitError(null)
    try {
      const payload: Record<string, unknown> = {...data, description: data.description || null}
      for (const key of Object.keys(payload)) {
        if (payload[key] === '') payload[key] = undefined
      }
      await onSubmit(payload as unknown as TaxProfileCreate)
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o perfil.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Identificação</p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="name"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Nome *</FormLabel>
                           <Input {...field} id={field.name} maxLength={120} className="w-full"
                                  placeholder="Venda de mercadoria — Simples Nacional"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="description"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Descrição</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={255}
                                  className="w-full" placeholder="Quando usar este perfil"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>

          <div className="space-y-2">
            <FormLabel>CFOPs cobertos *</FormLabel>
            <p className="text-xs text-gray-500">
              Escolha os CFOPs cobertos por este perfil — cada variante (interna/interestadual/exterior, ex.:
              5102/6102/7102) é uma opção própria. Se o tratamento é o mesmo nas três, adicione as três aqui;
              se difere por CFOP (ex.: um perfil só pro 5102, outro só pro 6102), crie perfis separados. Quando o
              tratamento difere só pela UF de destino dentro do mesmo CFOP, use os overrides por UF abaixo.
            </p>
            <div className="w-full sm:max-w-md">
              <Combobox value="" onValueChange={addCfop} options={cfopOptions}
                        placeholder="Adicionar CFOP" searchPlaceholder="Código ou descrição..."/>
            </div>
            {cfops.length > 0 && (
              <div className="flex flex-wrap gap-2 pt-1">
                {cfops.map((cfop) => (
                  <span key={cfop}
                        className="inline-flex items-center gap-1.5 rounded-full bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700">
                    {cfop}
                    <button type="button" onClick={() => removeCfop(cfop)}
                            aria-label={`Remover CFOP ${cfop}`}
                            className="text-brand-600 hover:text-red-600">×</button>
                  </span>
                ))}
              </div>
            )}
            {/* FormField só pelo FormMessage: o controle de edição são os chips acima. */}
            <FormField control={form.control} name="cfops" render={() => (<FormItem><FormMessage/></FormItem>)}/>
          </div>
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-5">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Tributação</p>
            <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${
              simples ? 'bg-emerald-50 text-emerald-700' : 'bg-blue-50 text-blue-700'
            }`}>
              {simples ? 'Simples Nacional — CSOSN' : 'Regime Normal — ICMS CST'}
            </span>
          </div>

          <TaxFieldsEditor value={taxValue} onChange={setTaxValue} simples={simples} hideCfop
                           groups={taxGroups} onGroupsChange={setTaxGroups}/>

          <div className="space-y-2 pt-2 border-t border-gray-200">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
              Overrides por UF de destino (opcional)
            </p>
            <UfOverridesEditor
              value={taxValue.uf_overrides ?? []}
              onChange={(next) => setTaxValue((r) => ({...r, uf_overrides: next}))}
              simples={simples}
            />
          </div>
        </div>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar perfil'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
