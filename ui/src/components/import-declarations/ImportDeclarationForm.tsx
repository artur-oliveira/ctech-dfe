'use client'

import {useState} from 'react'
import {useFieldArray, useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {OptionsSelect} from '@/components/ui/options-select'
import {CurrencyInput} from '@/components/ui/currency-input'
import {ApiError} from '@/lib/api/client'
import {UF_OPTIONS} from '@/lib/schemas/entity'
import {
  TP_INTERMEDIO_OPTIONS,
  TP_VIA_TRANSP_MARITIMA,
  TP_VIA_TRANSP_OPTIONS,
  type ImportDeclarationFormData,
  importDeclarationSchema,
} from '@/lib/schemas/import-declarations'
import type {ImportAdditionIn, ImportDeclarationCreate, ImportDeclarationItemOut} from '@/lib/types/api'

const EMPTY_ADDITION = {n_adicao: '1', c_fabricante: '', v_desc_di: '', n_draw: ''}

const EMPTY: ImportDeclarationFormData = {
  name: '', n_di: '', d_di: '', x_loc_desemb: '', uf_desemb: 'SP', d_desemb: '',
  tp_via_transp: '01', v_afrmm: '', tp_intermedio: '1', cnpj: '', uf_terceiro: '',
  c_exportador: '', additions: [EMPTY_ADDITION],
}

export interface ImportDeclarationFormProps {
  initialData?: ImportDeclarationItemOut
  onSubmit: (data: ImportDeclarationCreate) => Promise<void>
  loading?: boolean
}

const str = (v: unknown) => (typeof v === 'string' ? v : '')

function toFormData(di: ImportDeclarationItemOut): ImportDeclarationFormData {
  const additions = Array.isArray(di.additions) ? di.additions : []
  return {
    name: di.name,
    n_di: di.n_di,
    d_di: str(di.d_di),
    x_loc_desemb: str(di.x_loc_desemb),
    uf_desemb: str(di.uf_desemb) || 'SP',
    d_desemb: str(di.d_desemb),
    tp_via_transp: str(di.tp_via_transp) || '01',
    v_afrmm: str(di.v_afrmm),
    tp_intermedio: (str(di.tp_intermedio) || '1') as ImportDeclarationFormData['tp_intermedio'],
    cnpj: str(di.cnpj),
    uf_terceiro: str(di.uf_terceiro),
    c_exportador: str(di.c_exportador),
    additions: additions.length
      ? additions.map((a) => ({
        n_adicao: a.n_adicao,
        c_fabricante: a.c_fabricante,
        v_desc_di: a.v_desc_di ?? '',
        n_draw: a.n_draw ?? '',
      }))
      : [EMPTY_ADDITION],
  }
}

const nullify = (v: string | undefined) => (v ? v : null)

export function ImportDeclarationForm({initialData, onSubmit, loading}: ImportDeclarationFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<ImportDeclarationFormData>({
    resolver: zodResolver(importDeclarationSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })
  const {fields, append, remove} = useFieldArray({control: form.control, name: 'additions'})
  const viaTransp = useWatch({control: form.control, name: 'tp_via_transp'})

  const handleSubmit = async (data: ImportDeclarationFormData) => {
    setSubmitError(null)
    const additions: ImportAdditionIn[] = data.additions.map((a) => ({
      n_adicao: a.n_adicao,
      c_fabricante: a.c_fabricante,
      v_desc_di: nullify(a.v_desc_di),
      n_draw: nullify(a.n_draw),
    }))
    try {
      await onSubmit({
        name: data.name,
        n_di: data.n_di,
        d_di: data.d_di,
        x_loc_desemb: data.x_loc_desemb,
        uf_desemb: data.uf_desemb,
        d_desemb: data.d_desemb,
        tp_via_transp: data.tp_via_transp,
        v_afrmm: nullify(data.v_afrmm),
        tp_intermedio: data.tp_intermedio,
        cnpj: nullify(data.cnpj?.replace(/\D/g, '')),
        uf_terceiro: nullify(data.uf_terceiro),
        c_exportador: data.c_exportador,
        additions,
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar a declaração.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="name" render={({field}) => (
            <FormItem>
              <FormLabel>Nome *</FormLabel>
              <Input {...field} id={field.name} maxLength={120} className="w-full"
                     placeholder="DI 2026/0000001 — Itaqui"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="n_di" render={({field}) => (
            <FormItem>
              <FormLabel>Número da DI *</FormLabel>
              <Input {...field} id={field.name} maxLength={15} className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="d_di" render={({field}) => (
            <FormItem>
              <FormLabel>Data de registro *</FormLabel>
              <Input {...field} id={field.name} type="date" className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="d_desemb" render={({field}) => (
            <FormItem>
              <FormLabel>Data do desembaraço *</FormLabel>
              <Input {...field} id={field.name} type="date" className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="x_loc_desemb" render={({field}) => (
            <FormItem>
              <FormLabel>Local do desembaraço *</FormLabel>
              <Input {...field} id={field.name} maxLength={60} className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="uf_desemb" render={({field}) => (
            <FormItem>
              <FormLabel>UF do desembaraço *</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                             options={UF_OPTIONS}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="tp_via_transp" render={({field}) => (
            <FormItem>
              <FormLabel>Via de transporte *</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                             options={TP_VIA_TRANSP_OPTIONS}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="v_afrmm" render={({field}) => (
            <FormItem>
              <FormLabel>AFRMM {viaTransp === TP_VIA_TRANSP_MARITIMA ? '*' : ''}</FormLabel>
              <CurrencyInput id={field.name} value={field.value ?? ''} className="w-full"
                             onChange={field.onChange}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="tp_intermedio" render={({field}) => (
            <FormItem>
              <FormLabel>Intermediação *</FormLabel>
              <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange}
                             options={TP_INTERMEDIO_OPTIONS}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="c_exportador" render={({field}) => (
            <FormItem>
              <FormLabel>Código do exportador *</FormLabel>
              <Input {...field} id={field.name} maxLength={60} className="w-full"/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="cnpj" render={({field}) => (
            <FormItem>
              <FormLabel>CNPJ do adquirente/encomendante</FormLabel>
              <Input {...field} id={field.name} value={field.value ?? ''} maxLength={18} className="w-full"
                     placeholder="Só na conta e ordem ou encomenda"
                     onChange={(e) => field.onChange(e.target.value.replace(/\D/g, '').slice(0, 14))}/>
              <FormMessage/>
            </FormItem>
          )}/>
          <FormField control={form.control} name="uf_terceiro" render={({field}) => (
            <FormItem>
              <FormLabel>UF do adquirente/encomendante</FormLabel>
              <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange}
                             placeholder="Não informado" options={UF_OPTIONS}/>
              <FormMessage/>
            </FormItem>
          )}/>
        </div>

        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Adições</p>
            <Button type="button" variant="ghost" size="xs"
                    onClick={() => append({...EMPTY_ADDITION, n_adicao: String(fields.length + 1)})}>
              + Adição
            </Button>
          </div>
          <p className="text-xs text-gray-500">
            A emissão aponta qual adição representa cada item; o número da adição e a sequência saem
            desse vínculo, nunca são digitados na nota.
          </p>
          {fields.map((f, i) => (
            <div key={f.id}
                 className="grid grid-cols-1 sm:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_minmax(0,1fr)_minmax(0,1fr)_auto] gap-2 items-end">
              <FormField control={form.control} name={`additions.${i}.n_adicao`} render={({field}) => (
                <FormItem>
                  <FormLabel>Nº</FormLabel>
                  <Input {...field} id={field.name} maxLength={3} inputMode="numeric" className="w-full"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name={`additions.${i}.c_fabricante`} render={({field}) => (
                <FormItem>
                  <FormLabel>Fabricante</FormLabel>
                  <Input {...field} id={field.name} maxLength={60} className="w-full"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name={`additions.${i}.v_desc_di`} render={({field}) => (
                <FormItem>
                  <FormLabel>Desconto</FormLabel>
                  <CurrencyInput id={field.name} value={field.value ?? ''} className="w-full"
                                 onChange={field.onChange}/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <FormField control={form.control} name={`additions.${i}.n_draw`} render={({field}) => (
                <FormItem>
                  <FormLabel>Drawback</FormLabel>
                  <Input {...field} id={field.name} value={field.value ?? ''} maxLength={20} className="w-full"/>
                  <FormMessage/>
                </FormItem>
              )}/>
              <Button type="button" variant="ghost" size="xs" disabled={fields.length === 1}
                      onClick={() => remove(i)}>
                Remover
              </Button>
            </div>
          ))}
        </div>

        {submitError && (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
            {submitError}
          </div>
        )}

        <div className="flex justify-end">
          <Button type="submit" variant="brand" disabled={loading} className="min-h-11">
            {loading ? 'Salvando…' : 'Salvar'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
