'use client'

import {useState} from 'react'
import {useFieldArray, useForm, type UseFormReturn, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {useQuery} from '@tanstack/react-query'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {Textarea} from '@/components/ui/textarea'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {CollapsibleSection} from '@/components/ui/collapsible-section'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {PERSON_ROLE_INTERMEDIARY} from '@/lib/schemas/entity'
import {Combobox} from '@/components/ui/combobox'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {
  COMPRA_GOV_TP_ENTE_OPTIONS,
  COMPRA_GOV_TP_OPER_OPTIONS,
  TP_NF_CREDITO_OPTIONS,
  TP_NF_DEBITO_OPTIONS,
} from '@/lib/data/ibs_cbs_reform'
import {
  DOC_TYPE_OPTIONS,
  OPERATION_PLACEHOLDERS,
  type OperationFormData,
  operationSchema,
  safraOptions,
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

// Calculadas uma vez por carga do módulo: a lista não muda durante a sessão.
const SAFRA_OPTIONS = safraOptions()


const EMPTY: OperationFormData = {
  name: '', doc_types: ['nfe'], nat_op: '', tp_nf: '1', fin_nfe: '1',
  ind_final: '1', ind_pres: '1', cfop_suffix: '', tax_profile_id: '',
  payment_term_id: '', mod_frete: '', vol_esp: '', vol_marca: '',
  inf_ad_fisco: '', inf_cpl: '', obs_cont: [], obs_fisco: [],
  compra_x_n_emp: '', cana_safra: '',
  intermediary_person_id: '', ind_intermed: '', dh_sai_ent_offset_days: '',
  c_ind_op: '', c_mun_fg_ibs: '', tp_nf_debito: '', tp_nf_credito: '',
  compra_gov_tp_ente: '', compra_gov_p_redutor: '', compra_gov_tp_oper: '',
  ret_trib: {p_ret_pis: '', p_ret_cofins: '', p_ret_csll: '', p_ret_irrf: '', p_ret_prev_inss: ''},
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
    vol_esp: str(op.vol_esp),
    vol_marca: str(op.vol_marca),
    inf_ad_fisco: str(op.inf_ad_fisco),
    obs_cont: Array.isArray(op.obs_cont) ? (op.obs_cont as OperationFormData['obs_cont']) : [],
    ret_trib: {
      p_ret_pis: retTribField(op, 'p_ret_pis'),
      p_ret_cofins: retTribField(op, 'p_ret_cofins'),
      p_ret_csll: retTribField(op, 'p_ret_csll'),
      p_ret_irrf: retTribField(op, 'p_ret_irrf'),
      p_ret_prev_inss: retTribField(op, 'p_ret_prev_inss'),
    },
    obs_fisco: Array.isArray(op.obs_fisco) ? (op.obs_fisco as OperationFormData['obs_fisco']) : [],
    inf_cpl: str(op.inf_cpl),
    compra_x_n_emp: str(op.compra_x_n_emp),
    cana_safra: str(op.cana_safra),
    intermediary_person_id: str(op.intermediary_person_id),
    ind_intermed: str(op.ind_intermed) as OperationFormData['ind_intermed'],
    dh_sai_ent_offset_days: typeof op.dh_sai_ent_offset_days === 'number'
      ? String(op.dh_sai_ent_offset_days)
      : '',
    c_ind_op: str(op.c_ind_op),
    c_mun_fg_ibs: str(op.c_mun_fg_ibs),
    tp_nf_debito: str(op.tp_nf_debito) as OperationFormData['tp_nf_debito'],
    tp_nf_credito: str(op.tp_nf_credito) as OperationFormData['tp_nf_credito'],
    compra_gov_tp_ente: str(op.compra_gov_tp_ente) as OperationFormData['compra_gov_tp_ente'],
    compra_gov_p_redutor: str(op.compra_gov_p_redutor),
    compra_gov_tp_oper: str(op.compra_gov_tp_oper) as OperationFormData['compra_gov_tp_oper'],
    requires_receiver: op.requires_receiver !== false,
    is_default: op.is_default === true,
  }
}

/**
 * Lista de pares campo/texto de infAdic (obsCont ou obsFisco). Observação fixa
 * por operação: o texto que se repete por cenário mora no cadastro, não na nota.
 */
/** Lê um percentual do grupo ret_trib de uma operação persistida. */
function retTribField(op: Record<string, unknown>, key: string): string {
  const group = op.ret_trib as Record<string, unknown> | undefined
  const v = group?.[key]
  return typeof v === 'string' ? v : ''
}

/** Grupo todo vazio vira null: perfil sem percentual nenhum não é perfil. */
function retTribPayload(v: OperationFormData['ret_trib']): Record<string, string> | null {
  const filled = Object.entries(v).filter(([, value]) => !!value)
  return filled.length ? Object.fromEntries(filled) : null
}

function ObsListField({form, name, label}: {
  form: UseFormReturn<OperationFormData>
  name: 'obs_cont' | 'obs_fisco'
  label: string
}) {
  const {fields, append, remove} = useFieldArray({control: form.control, name})
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <FormLabel>{label}</FormLabel>
        <Button type="button" variant="ghost" size="xs" disabled={fields.length >= 10}
                onClick={() => append({x_campo: '', x_texto: ''})}>
          + Observação
        </Button>
      </div>
      {fields.map((field, index) => (
        <div key={field.id} className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto] gap-2 items-end">
          <FormField control={form.control} name={`${name}.${index}.x_campo`}
                     render={({field: f}) => (
                       <FormItem>
                         <Input {...f} id={f.name} placeholder="Campo" maxLength={20} className="w-full"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name={`${name}.${index}.x_texto`}
                     render={({field: f}) => (
                       <FormItem>
                         <Input {...f} id={f.name} placeholder="Texto" maxLength={60} className="w-full"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <Button type="button" variant="ghost" size="xs" onClick={() => remove(index)}>
            Remover
          </Button>
        </div>
      ))}
    </div>
  )
}

/** Campo string vazio vira null: um "" gravado é um default silenciosamente vazio. */
const nullify = (v: string | undefined) => (v ? v : null)

/**
 * Cadastro de natureza de operação — o formulário curto que responde de uma vez
 * as perguntas que hoje o operador refaz a cada emissão.
 */

/** Campos de cada seção avançada — o badge de erro precisa saber onde eles moram. */
const MESSAGE_FIELDS = ['inf_ad_fisco', 'inf_cpl', 'obs_cont', 'obs_fisco'] as const
const TAX_FIELDS = [
  'ret_trib', 'c_ind_op', 'c_mun_fg_ibs', 'tp_nf_debito', 'tp_nf_credito',
  'compra_gov_tp_ente', 'compra_gov_p_redutor', 'compra_gov_tp_oper',
] as const
const NICHE_FIELDS = [
  'intermediary_person_id', 'ind_intermed', 'dh_sai_ent_offset_days',
  'compra_x_n_emp', 'cana_safra', 'export_uf_saida_pais', 'export_loc_despacho_index',
] as const

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

  // Intermediadores cadastrados: a operação aponta a plataforma, e o "seller
  // id" vem do cadastro dela. Sem nenhum cadastrado, o select fica só com a
  // opção de canal próprio — não há campo livre para errar o CNPJ.
  const {data: intermediaryPage} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk, PERSON_ROLE_INTERMEDIARY),
    queryFn: () => apiClient.getPersons({role: PERSON_ROLE_INTERMEDIARY, limit: 100}),
    enabled: !!selectedOrg,
  })
  const intermediaryOptions = [
    {value: '', label: 'Venda em canal próprio'},
    ...(intermediaryPage?.items ?? []).map((p) => ({value: p.sk, label: p.name})),
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
        vol_esp: nullify(data.vol_esp),
        vol_marca: nullify(data.vol_marca),
        inf_ad_fisco: nullify(data.inf_ad_fisco),
        obs_cont: data.obs_cont.length > 0 ? data.obs_cont : null,
        ret_trib: retTribPayload(data.ret_trib),
        obs_fisco: data.obs_fisco.length > 0 ? data.obs_fisco : null,
        inf_cpl: nullify(data.inf_cpl),
        compra_x_n_emp: nullify(data.compra_x_n_emp),
        cana_safra: nullify(data.cana_safra),
        intermediary_person_id: nullify(data.intermediary_person_id),
        ind_intermed: nullify(data.ind_intermed),
        dh_sai_ent_offset_days: data.dh_sai_ent_offset_days
          ? Number(data.dh_sai_ent_offset_days)
          : null,
        c_ind_op: nullify(data.c_ind_op),
        c_mun_fg_ibs: nullify(data.c_mun_fg_ibs),
        tp_nf_debito: nullify(data.tp_nf_debito),
        tp_nf_credito: nullify(data.tp_nf_credito),
        compra_gov_tp_ente: nullify(data.compra_gov_tp_ente),
        compra_gov_p_redutor: nullify(data.compra_gov_p_redutor),
        compra_gov_tp_oper: nullify(data.compra_gov_tp_oper),
        requires_receiver: data.requires_receiver,
        is_default: data.is_default,
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a operação.')
    }
  }

  /**
   * Marca da seção fechada: quantos campos dela estão com erro. Uma seção
   * colapsada escondendo o erro que impede o salvamento é submit que falha sem
   * nada mudar na tela.
   */
  const sectionBadge = (fields: readonly string[]) => {
    const count = fields.filter((f) => f in form.formState.errors).length
    if (count === 0) return null
    return (
      <span aria-label={`${count} campo(s) com erro`}
            className="rounded-full bg-red-100 px-1.5 py-0.5 text-xs font-semibold text-red-700">
        {count}
      </span>
    )
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

            {([
              ['vol_esp', 'Espécie do volume', 'CAIXA'],
              ['vol_marca', 'Marca do volume', 'ACME'],
            ] as const).map(([name, label, placeholder]) => (
              <FormField key={name} control={form.control} name={name}
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>{label}</FormLabel>
                             <Input {...field} id={field.name} value={field.value ?? ''} maxLength={60}
                                    className="w-full" placeholder={placeholder}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            ))}

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

        <CollapsibleSection title="Mensagens fiscais"
                            description="Texto que vai em infAdic de toda nota da operação"
                            badge={sectionBadge(MESSAGE_FIELDS)}>
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

            {(['obs_cont', 'obs_fisco'] as const).map((name) => (
              <ObsListField key={name} form={form} name={name}
                            label={name === 'obs_cont' ? 'Observações do contribuinte' : 'Observações ao fisco'}/>
            ))}
          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Retenções e reforma tributária"
                            description="Percentuais de retenção federal e identificação de IBS/CBS"
                            badge={sectionBadge(TAX_FIELDS)}>
          <div className="space-y-4">
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                Retenções federais (%)
              </p>
              <p className="text-xs text-gray-500">
                Percentuais do cenário. Os valores retidos são calculados sobre a base da nota na emissão.
              </p>
              <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
                {([
                  ['p_ret_pis', 'PIS'],
                  ['p_ret_cofins', 'COFINS'],
                  ['p_ret_csll', 'CSLL'],
                  ['p_ret_irrf', 'IRRF'],
                  ['p_ret_prev_inss', 'INSS'],
                ] as const).map(([key, label]) => (
                  <FormField key={key} control={form.control} name={`ret_trib.${key}` as const}
                             render={({field}) => (
                               <FormItem>
                                 <FormLabel>{label}</FormLabel>
                                 <NumericInput id={field.name} decimal integerPlaces={3} decimalPlaces={4}
                                               value={field.value ?? ''} placeholder="0.0000"
                                               onChange={field.onChange}/>
                                 <FormMessage/>
                               </FormItem>
                             )}/>
                ))}
              </div>
            </div>


            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                Reforma tributária (IBS/CBS)
              </p>
              <p className="text-xs text-gray-500">
                Campos de identificação do documento. As alíquotas e os CSTs ficam no perfil fiscal
                ou no produto, não aqui.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <FormField control={form.control} name="c_ind_op"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Local da operação (cIndOp)</FormLabel>
                               <NumericInput id={field.name} value={field.value ?? ''} maxLength={6}
                                             placeholder="6 dígitos" onChange={field.onChange}/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="c_mun_fg_ibs"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Município do fato gerador do IBS/CBS</FormLabel>
                               <Combobox id={field.name} value={field.value ?? ''}
                                         onValueChange={field.onChange} options={CITY_OPTIONS}
                                         placeholder="Buscar município"/>
                               <p className="text-xs text-gray-500">
                                 Só quando a presença é 5 (fora do estabelecimento) e não há endereço
                                 de destinatário nem local de entrega.
                               </p>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="tp_nf_debito"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Nota de débito</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={field.onChange}
                                              options={TP_NF_DEBITO_OPTIONS} placeholder="Não se aplica"/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="tp_nf_credito"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Nota de crédito</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={field.onChange}
                                              options={TP_NF_CREDITO_OPTIONS} placeholder="Não se aplica"/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="compra_gov_tp_ente"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Ente governamental comprador</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={field.onChange}
                                              options={COMPRA_GOV_TP_ENTE_OPTIONS}
                                              placeholder="Não é compra governamental"/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="compra_gov_p_redutor"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>% Redutor da compra governamental</FormLabel>
                               <NumericInput id={field.name} decimal integerPlaces={3} decimalPlaces={4}
                                             value={field.value ?? ''} placeholder="0.0000"
                                             onChange={field.onChange}/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="compra_gov_tp_oper"
                           render={({field}) => (
                             <FormItem className="sm:col-span-2">
                               <FormLabel>Tipo da operação governamental</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={field.onChange}
                                              options={COMPRA_GOV_TP_OPER_OPTIONS}
                                              placeholder="Não se aplica"/>
                               <p className="text-xs text-gray-500">
                                 O tipo decide se a emissão pede as chaves dos documentos anteriores —
                                 o formulário de emissão só mostra o campo quando ele é aceito.
                               </p>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
              </div>
            </div>

          </div>
        </CollapsibleSection>

        <CollapsibleSection title="Grupos setoriais"
                            description="Marketplace, prazo de saída, compra pública e cana"
                            badge={sectionBadge(NICHE_FIELDS)}>
          <div className="space-y-4">
            <div className="space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                Grupos de nicho
              </p>
              <p className="text-xs text-gray-500">
                Só aparecem no XML quando preenchidos. Pedido, contrato e os fornecimentos diários
                de cana variam por nota e são pedidos na emissão.
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <FormField control={form.control} name="intermediary_person_id"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Intermediador (marketplace)</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={(v) => {
                                                field.onChange(v)
                                                // O indicador acompanha a escolha: plataforma de
                                                // terceiros é 1, canal próprio é 0.
                                                form.setValue('ind_intermed', v ? '1' : '0')
                                              }}
                                              options={intermediaryOptions}/>
                               <p className="text-xs text-gray-500">
                                 Cadastre a plataforma como pessoa com o papel “Intermediador”.
                               </p>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="dh_sai_ent_offset_days"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Prazo de saída (dias após a emissão)</FormLabel>
                               <NumericInput id={field.name} value={field.value ?? ''} maxLength={3}
                                             placeholder="Em branco: não declara saída"
                                             onChange={field.onChange}/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="compra_x_n_emp"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Nota de empenho (compra/xNEmp)</FormLabel>
                               <Input {...field} id={field.name} value={field.value ?? ''} maxLength={22}
                                      className="w-full" placeholder="2026NE000123"/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                <FormField control={form.control} name="cana_safra"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Safra da cana (cana/safra)</FormLabel>
                               <OptionsSelect id={field.name} value={field.value ?? ''}
                                              onValueChange={field.onChange} options={SAFRA_OPTIONS}
                                              placeholder="Não se aplica"/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
              </div>
            </div>
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
