'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useQuery} from '@tanstack/react-query'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Textarea} from '@/components/ui/textarea'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {CollapsibleSection} from '@/components/ui/collapsible-section'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {
  DOC_TYPE_OPTIONS,
  OPERATION_PLACEHOLDERS,
  type OperationFormData,
  operationSchema,
} from '@/lib/schemas/operations'
import {
  FIN_NFE_OPTIONS,
  IND_FINAL_OPTIONS,
  IND_PRES_OPTIONS,
  MOD_FRETE_OPTIONS,
  TP_NF_OPTIONS,
} from '@/lib/data/nfe_fields'
import type {OperationCreate, OperationItemOut} from '@/lib/types/api'

interface OperationFormProps {
  initialData?: OperationItemOut
  onSubmit: (data: OperationCreate) => Promise<void>
  loading?: boolean
}

const EMPTY: OperationFormData = {
  name: '', doc_types: ['nfe'], nat_op: '', tp_nf: '1', fin_nfe: '1',
  ind_final: '1', ind_pres: '1', cfop_suffix: '', tax_profile_id: '',
  payment_term_id: '', mod_frete: '', inf_ad_fisco: '', inf_cpl: '',
  requires_receiver: true, is_default: false,
}

function toFormData(op: OperationItemOut): OperationFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {
    ...EMPTY,
    name: op.name,
    doc_types: (op.doc_types ?? ['nfe']) as OperationFormData['doc_types'],
    nat_op: op.nat_op ?? '',
    tp_nf: str(op.tp_nf) as OperationFormData['tp_nf'],
    fin_nfe: str(op.fin_nfe) as OperationFormData['fin_nfe'],
    ind_final: str(op.ind_final) as OperationFormData['ind_final'],
    ind_pres: str(op.ind_pres) as OperationFormData['ind_pres'],
    cfop_suffix: op.cfop_suffix ?? '',
    tax_profile_id: str(op.tax_profile_id),
    payment_term_id: str(op.payment_term_id),
    mod_frete: str(op.mod_frete) as OperationFormData['mod_frete'],
    inf_ad_fisco: str(op.inf_ad_fisco),
    inf_cpl: str(op.inf_cpl),
    requires_receiver: op.requires_receiver !== false,
    is_default: op.is_default === true,
  }
}

/** Campo string vazio vira null: um "" gravado é um default silenciosamente vazio. */
const nullify = (v: string | undefined) => (v ? v : null)

/**
 * Cadastro de natureza de operação — o formulário curto que responde de uma vez
 * as perguntas que hoje o operador refaz a cada emissão.
 */
export function OperationForm({initialData, onSubmit, loading = false}: OperationFormProps) {
  const {selectedOrg} = useAuth()
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<OperationFormData>({
    resolver: zodResolver(operationSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const {data: taxProfilePage} = useQuery({
    queryKey: queryKeys.taxProfiles.list(selectedOrg?.pk),
    queryFn: () => apiClient.getTaxProfiles({limit: 100}),
    enabled: !!selectedOrg,
  })
  const taxProfileOptions = [
    {value: '', label: 'Usar o perfil do produto'},
    ...(taxProfilePage?.items ?? []).map((tp) => ({
      value: extractId(tp.sk, SK_PREFIX.TAX_PROFILE),
      label: tp.name,
    })),
  ]

  const docTypes = useWatch({control: form.control, name: 'doc_types'}) ?? []
  const toggleDocType = (value: OperationFormData['doc_types'][number]) => {
    form.setValue('doc_types', docTypes.includes(value)
      ? docTypes.filter((d) => d !== value)
      : [...docTypes, value])
  }

  const handleSubmit = async (data: OperationFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        doc_types: data.doc_types,
        nat_op: nullify(data.nat_op),
        tp_nf: nullify(data.tp_nf),
        fin_nfe: nullify(data.fin_nfe),
        ind_final: nullify(data.ind_final),
        ind_pres: nullify(data.ind_pres),
        cfop_suffix: nullify(data.cfop_suffix),
        tax_profile_id: nullify(data.tax_profile_id),
        payment_term_id: nullify(data.payment_term_id),
        mod_frete: nullify(data.mod_frete),
        inf_ad_fisco: nullify(data.inf_ad_fisco),
        inf_cpl: nullify(data.inf_cpl),
        requires_receiver: data.requires_receiver,
        is_default: data.is_default,
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a operação.')
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
                                  placeholder="Venda para revenda"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="nat_op"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Natureza da operação (natOp)</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}
                                  className="w-full" placeholder="Venda de mercadoria"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>

          <div className="space-y-1">
            <FormLabel>Documentos</FormLabel>
            <div className="flex flex-wrap gap-x-5 gap-y-2 pt-1">
              {DOC_TYPE_OPTIONS.map((opt) => (
                <label key={opt.value}
                       className="flex min-h-11 sm:min-h-0 cursor-pointer items-center gap-2 text-sm text-gray-700">
                  <input type="checkbox" checked={docTypes.includes(opt.value)}
                         onChange={() => toggleDocType(opt.value)}
                         className="size-4 cursor-pointer rounded border-gray-300 text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"/>
                  {opt.label}
                </label>
              ))}
            </div>
          </div>

          <FormField control={form.control} name="is_default"
                     render={({field}) => (
                       <FormItem>
                         <label className="flex min-h-11 sm:min-h-0 cursor-pointer items-center gap-2 text-sm text-gray-700">
                           <input type="checkbox" checked={field.value}
                                  onChange={(e) => field.onChange(e.target.checked)}
                                  className="size-4 cursor-pointer rounded border-gray-300 text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"/>
                           Operação padrão da organização
                         </label>
                         <p className="text-xs text-gray-500">
                           Vem pré-selecionada na emissão. Só uma pode ser a padrão — marcar esta desmarca a anterior.
                         </p>
                       </FormItem>
                     )}
          />
        </div>

        <div className="rounded-xl border border-gray-200 bg-white p-5 space-y-4">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Defaults da emissão</p>
          <p className="text-xs text-gray-500">
            Estes valores entram na nota quando o request não traz os seus.
            Um valor informado na emissão sempre vence a operação.
          </p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {([
              ['tp_nf', 'Tipo', TP_NF_OPTIONS],
              ['fin_nfe', 'Finalidade', FIN_NFE_OPTIONS],
              ['ind_final', 'Consumidor final', IND_FINAL_OPTIONS],
              ['ind_pres', 'Presença do comprador', IND_PRES_OPTIONS],
              ['mod_frete', 'Modalidade do frete', MOD_FRETE_OPTIONS],
            ] as const).map(([name, label, options]) => (
              <FormField key={name} control={form.control} name={name}
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>{label}</FormLabel>
                             <OptionsSelect id={field.name} value={field.value ?? ''}
                                            onValueChange={field.onChange}
                                            options={[...options]}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            ))}

            <FormField control={form.control} name="cfop_suffix"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Natureza fiscal do CFOP</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={3}
                                  inputMode="numeric" className="w-full" placeholder="102"/>
                           <p className="text-xs text-gray-500">
                             Só os 3 últimos dígitos. O primeiro (5 dentro da UF, 6 outra UF, 7 exterior)
                             é resolvido na emissão pelo endereço do destinatário.
                           </p>
                           <FormMessage/>
                         </FormItem>
                       )}
            />

            <FormField control={form.control} name="tax_profile_id"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Perfil fiscal</FormLabel>
                           <OptionsSelect id={field.name} value={field.value ?? ''}
                                          onValueChange={field.onChange} options={taxProfileOptions}/>
                           <p className="text-xs text-gray-500">Usado quando o produto não define um.</p>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>

          <FormField control={form.control} name="requires_receiver"
                     render={({field}) => (
                       <FormItem>
                         <label className="flex min-h-11 sm:min-h-0 cursor-pointer items-center gap-2 text-sm text-gray-700">
                           <input type="checkbox" checked={field.value}
                                  onChange={(e) => field.onChange(e.target.checked)}
                                  className="size-4 cursor-pointer rounded border-gray-300 text-brand-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"/>
                           Exige destinatário
                         </label>
                         <p className="text-xs text-gray-500">
                           Desmarque para operações emitidas contra a própria organização.
                         </p>
                       </FormItem>
                     )}
          />
        </div>

        <CollapsibleSection title="Mensagens fiscais">
          <div className="space-y-4">
            <p className="text-xs text-gray-500">
              Disponíveis:{' '}
              {OPERATION_PLACEHOLDERS.map((ph) => (
                <code key={ph.key} className="mr-2 rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs">
                  {`{{${ph.key}}}`}
                </code>
              ))}
              Uma chave fora dessa lista é recusada aqui — nunca vira um espaço em branco na nota.
            </p>

            <FormField control={form.control} name="inf_ad_fisco"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Informações ao fisco</FormLabel>
                           <Textarea {...field} id={field.name} value={field.value ?? ''} rows={3}
                                     className="w-full" maxLength={2000}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="inf_cpl"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Informações complementares</FormLabel>
                           <Textarea {...field} id={field.name} value={field.value ?? ''} rows={3}
                                     className="w-full" maxLength={5000}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        </CollapsibleSection>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar operação'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
