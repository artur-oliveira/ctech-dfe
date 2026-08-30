'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {useQuery} from '@tanstack/react-query'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {Combobox} from '@/components/ui/combobox'
import {OptionsSelect} from '@/components/ui/options-select'
import {CITY_OPTIONS} from '@/lib/data/cities'
import {ApiError, apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {useAuth} from '@/lib/hooks/useAuth'
import {
  REFERENCE_DFE_KEY_TYPES,
  REFERENCE_DOCUMENT_KINDS,
  type ReferenceDocumentFormData,
  referenceDocumentSchema,
} from '@/lib/schemas/reference-documents'
import type {
  ReferenceDocumentCreate,
  ReferenceDocumentItemOut,
  ReferenceDocumentKind,
} from '@/lib/types/api'

const EMPTY: ReferenceDocumentFormData = {
  name: '', kind: 'dfe', issued_at: '', competence_at: '', description: '', supplier_person_id: '',
  tipo_chave_dfe: '', chave_dfe: '',
  c_mun_nfse_mun: '', n_nfse_mun: '', c_verif_nfse_mun: '',
  n_nfs: '', mod_nfs: '', serie_nfs: '',
  n_doc_fiscal: '', c_mun_doc_fiscal: '', x_doc_fiscal: '',
  n_doc: '', x_doc: '',
}

export interface ReferenceDocumentFormProps {
  initialData?: ReferenceDocumentItemOut
  onSubmit: (data: ReferenceDocumentCreate) => Promise<void>
  loading?: boolean
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

function group(item: ReferenceDocumentItemOut, key: string): Record<string, unknown> {
  const value = item[key]
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

function toFormData(d: ReferenceDocumentItemOut): ReferenceDocumentFormData {
  const dfe = group(d, 'dfe')
  const mun = group(d, 'nfse_municipal')
  const nf = group(d, 'nf_nfs')
  const fiscal = group(d, 'doc_fiscal_outro')
  const other = group(d, 'doc_nao_fiscal')
  return {
    name: d.name,
    kind: d.kind,
    issued_at: str(d.issued_at),
    competence_at: str(d.competence_at),
    description: str(d.description),
    supplier_person_id: str(d.supplier_person_id),
    tipo_chave_dfe: str(dfe.tipo_chave_dfe),
    chave_dfe: str(dfe.chave_dfe),
    c_mun_nfse_mun: str(mun.c_mun_nfse_mun),
    n_nfse_mun: str(mun.n_nfse_mun),
    c_verif_nfse_mun: str(mun.c_verif_nfse_mun),
    n_nfs: str(nf.n_nfs),
    mod_nfs: str(nf.mod_nfs),
    serie_nfs: str(nf.serie_nfs),
    n_doc_fiscal: str(fiscal.n_doc_fiscal),
    c_mun_doc_fiscal: str(fiscal.c_mun_doc_fiscal),
    x_doc_fiscal: str(fiscal.x_doc_fiscal),
    n_doc: str(other.n_doc),
    x_doc: str(other.x_doc),
  }
}

function orNull(value: string | undefined): string | null {
  return value ? value : null
}

/**
 * Só o subobjeto da família escolhida é enviado. O backend recusa dois
 * preenchidos, e mandar os outros como `null` mantém o payload explícito
 * quando o usuário troca de família numa edição.
 */
function buildUnion(data: ReferenceDocumentFormData): Partial<ReferenceDocumentCreate> {
  const empty = {
    dfe: null, nfse_municipal: null, nf_nfs: null, doc_fiscal_outro: null, doc_nao_fiscal: null,
  }
  switch (data.kind) {
    case 'dfe':
      return {
        ...empty,
        dfe: {
          tipo_chave_dfe: data.tipo_chave_dfe ?? '',
          chave_dfe: data.chave_dfe ?? '',
          x_tipo_chave_dfe: null,
        },
      }
    case 'nfse_municipal':
      return {
        ...empty,
        nfse_municipal: {
          c_mun_nfse_mun: data.c_mun_nfse_mun ?? '',
          n_nfse_mun: data.n_nfse_mun ?? '',
          c_verif_nfse_mun: data.c_verif_nfse_mun ?? '',
        },
      }
    case 'nf_nfs':
      return {
        ...empty,
        nf_nfs: {
          n_nfs: data.n_nfs ?? '', mod_nfs: data.mod_nfs ?? '', serie_nfs: data.serie_nfs ?? '',
        },
      }
    case 'doc_fiscal_outro':
      return {
        ...empty,
        doc_fiscal_outro: {
          n_doc_fiscal: data.n_doc_fiscal ?? '',
          c_mun_doc_fiscal: orNull(data.c_mun_doc_fiscal),
          x_doc_fiscal: orNull(data.x_doc_fiscal),
        },
      }
    case 'doc_nao_fiscal':
      return {
        ...empty,
        doc_nao_fiscal: {n_doc: data.n_doc ?? '', x_doc: orNull(data.x_doc)},
      }
  }
}

export function ReferenceDocumentForm({initialData, onSubmit, loading}: ReferenceDocumentFormProps) {
  const {selectedOrg} = useAuth()
  const [submitError, setSubmitError] = useState<string | null>(null)
  const form = useForm<ReferenceDocumentFormData>({
    resolver: zodResolver(referenceDocumentSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })
  const kind: ReferenceDocumentKind = useWatch({control: form.control, name: 'kind'})

  // O fornecedor é sempre uma pessoa do cadastro: o documento referencia, nunca
  // copia CNPJ e nome.
  const {data: personsPage} = useQuery({
    queryKey: queryKeys.persons.list(selectedOrg?.pk),
    queryFn: () => apiClient.getPersons({limit: 100}),
    enabled: !!selectedOrg,
  })
  const supplierOptions = [
    {value: '', label: 'Sem fornecedor'},
    ...(personsPage?.items ?? []).map((p) => ({value: p.sk, label: p.name})),
  ]

  const handleSubmit = async (data: ReferenceDocumentFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        kind: data.kind,
        issued_at: data.issued_at,
        competence_at: orNull(data.competence_at),
        description: orNull(data.description),
        supplier_person_id: orNull(data.supplier_person_id),
        ...buildUnion(data),
      } as ReferenceDocumentCreate)
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o documento.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="name"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Nome *</FormLabel>
                         <Input {...field} id={field.name} maxLength={120} className="w-full"
                                placeholder="NF-e do fornecedor X"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="kind"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Tipo de documento *</FormLabel>
                         <OptionsSelect id={field.name} value={field.value} className="w-full"
                                        options={[...REFERENCE_DOCUMENT_KINDS]}
                                        onValueChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
        </div>

        {kind === 'dfe' && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="tipo_chave_dfe"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Documento da chave *</FormLabel>
                           <OptionsSelect id={field.name} value={field.value ?? ''} className="w-full"
                                          options={[...REFERENCE_DFE_KEY_TYPES]}
                                          onValueChange={field.onChange}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="chave_dfe"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Chave de acesso *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={50}
                                  className="w-full" inputMode="numeric"
                                  onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                           <p className="text-xs text-gray-500">NFS-e tem 50 dígitos; NF-e, 44.</p>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        )}

        {kind === 'nfse_municipal' && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <FormField control={form.control} name="c_mun_nfse_mun"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Município emissor *</FormLabel>
                           <Combobox id={field.name} value={field.value ?? ''} options={CITY_OPTIONS}
                                     className="w-full" placeholder="Selecione o município"
                                     searchPlaceholder="Buscar município…" fuzzySearch
                                     onValueChange={field.onChange}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="n_nfse_mun"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Número da NFS-e *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={15}
                                  className="w-full" inputMode="numeric"
                                  onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="c_verif_nfse_mun"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Código de verificação *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={9}
                                  className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        )}

        {kind === 'nf_nfs' && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <FormField control={form.control} name="n_nfs"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Número *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={7}
                                  className="w-full" inputMode="numeric"
                                  onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="mod_nfs"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Modelo *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={15}
                                  className="w-full" inputMode="numeric"
                                  onChange={(e) => field.onChange(e.target.value.replace(/\D/g, ''))}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="serie_nfs"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Série *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={15}
                                  className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        )}

        {kind === 'doc_fiscal_outro' && (
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <FormField control={form.control} name="n_doc_fiscal"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Número do documento *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={255}
                                  className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="c_mun_doc_fiscal"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Município</FormLabel>
                           <Combobox id={field.name} value={field.value ?? ''} options={CITY_OPTIONS}
                                     className="w-full" placeholder="Opcional"
                                     searchPlaceholder="Buscar município…" fuzzySearch
                                     onValueChange={field.onChange}/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="x_doc_fiscal"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Descrição</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={255}
                                  className="w-full" placeholder="Opcional"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <p className="sm:col-span-3 text-xs text-gray-500">
              Município e descrição só são usados em reembolso, repasse e ressarcimento; na dedução
              o leiaute pede apenas o número.
            </p>
          </div>
        )}

        {kind === 'doc_nao_fiscal' && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <FormField control={form.control} name="n_doc"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Número do documento *</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={255}
                                  className="w-full"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
            <FormField control={form.control} name="x_doc"
                       render={({field}) => (
                         <FormItem>
                           <FormLabel>Descrição</FormLabel>
                           <Input {...field} id={field.name} value={field.value ?? ''} maxLength={255}
                                  className="w-full" placeholder="Opcional"/>
                           <FormMessage/>
                         </FormItem>
                       )}
            />
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="issued_at"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Data de emissão *</FormLabel>
                         <input {...field} id={field.name} type="date"
                                className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="competence_at"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Data de competência</FormLabel>
                         <input {...field} id={field.name} type="date" value={field.value ?? ''}
                                className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="supplier_person_id"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Fornecedor</FormLabel>
                         <OptionsSelect id={field.name} value={field.value ?? ''} className="w-full"
                                        options={supplierOptions} onValueChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="description"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Descrição da dedução</FormLabel>
                         <Input {...field} id={field.name} value={field.value ?? ''} maxLength={150}
                                className="w-full" placeholder="Opcional"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
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
