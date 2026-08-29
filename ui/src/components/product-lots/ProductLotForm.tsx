'use client'

import {useState} from 'react'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {Button} from '@/components/ui/button'
import {NumericInput} from '@/components/ui/numeric-input'
import {ProductSearch} from '@/components/ui/product-search'
import {ApiError} from '@/lib/api/client'
import {extractId, SK_PREFIX} from '@/lib/constants/entity-keys'
import {type ProductLotFormData, productLotSchema} from '@/lib/schemas/product-lots'
import type {ProductLotCreate, ProductLotItemOut} from '@/lib/types/api'

const EMPTY: ProductLotFormData = {
  name: '', product_id: '', n_lote: '', q_lote: '', d_fab: '', d_val: '', c_agreg: '',
}

export interface ProductLotFormProps {
  initialData?: ProductLotItemOut
  onSubmit: (data: ProductLotCreate) => Promise<void>
  loading?: boolean
}

function toFormData(l: ProductLotItemOut): ProductLotFormData {
  const str = (v: unknown) => (typeof v === 'string' ? v : '')
  return {
    name: l.name,
    product_id: str(l.product_id),
    n_lote: str(l.n_lote),
    q_lote: str(l.q_lote),
    d_fab: str(l.d_fab),
    d_val: str(l.d_val),
    c_agreg: str(l.c_agreg),
  }
}

export function ProductLotForm({initialData, onSubmit, loading}: ProductLotFormProps) {
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [productLabel, setProductLabel] = useState(
    typeof initialData?.product_description === 'string' ? initialData.product_description : '',
  )
  const form = useForm<ProductLotFormData>({
    resolver: zodResolver(productLotSchema),
    defaultValues: initialData ? toFormData(initialData) : EMPTY,
  })

  const handleSubmit = async (data: ProductLotFormData) => {
    setSubmitError(null)
    try {
      await onSubmit({
        name: data.name,
        product_id: data.product_id,
        n_lote: data.n_lote,
        q_lote: data.q_lote,
        d_fab: data.d_fab,
        d_val: data.d_val,
        c_agreg: data.c_agreg || null,
      })
    } catch (err) {
      setSubmitError(err instanceof ApiError ? err.message : 'Não foi possível salvar o lote.')
    }
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-5">
        <FormField control={form.control} name="product_id"
                   render={({field}) => (
                     <FormItem>
                       <FormLabel>Produto do lote *</FormLabel>
                       {productLabel && (
                         <p className="text-sm text-gray-700">
                           {productLabel}{' '}
                           <Button type="button" variant="ghost" size="xs"
                                   onClick={() => {
                                     setProductLabel('')
                                     field.onChange('')
                                   }}
                                   className="text-brand-600 hover:text-brand-700">trocar</Button>
                         </p>
                       )}
                       {!productLabel && (
                         <ProductSearch
                           placeholder="Código ou descrição do produto..."
                           onSelect={(p) => {
                             field.onChange(extractId(p.sk, SK_PREFIX.PRODUCT))
                             setProductLabel(`${p.code} · ${p.description}`)
                           }}/>
                       )}
                       <FormMessage/>
                     </FormItem>
                   )}
        />

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <FormField control={form.control} name="name"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Nome *</FormLabel>
                         <Input {...field} id={field.name} maxLength={120} className="w-full"
                                placeholder="Lote 2026/001"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="n_lote"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Número do lote *</FormLabel>
                         <Input {...field} id={field.name} maxLength={20} className="w-full"
                                placeholder="ABC1234"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="q_lote"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Quantidade produzida *</FormLabel>
                         <NumericInput id={field.name} value={field.value} decimal decimalPlaces={3}
                                       className="w-full" onChange={field.onChange}/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="c_agreg"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Código de agregação</FormLabel>
                         <Input {...field} id={field.name} value={field.value ?? ''} maxLength={20}
                                className="w-full" placeholder="Opcional"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="d_fab"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Data de fabricação *</FormLabel>
                         <input {...field} id={field.name} type="date"
                                className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <FormField control={form.control} name="d_val"
                     render={({field}) => (
                       <FormItem>
                         <FormLabel>Data de validade *</FormLabel>
                         <input {...field} id={field.name} type="date"
                                className="w-full h-11 rounded-md border border-gray-300 px-3 text-sm"/>
                         <FormMessage/>
                       </FormItem>
                     )}
          />
          <p className="sm:col-span-2 text-xs text-gray-500">
            A quantidade produzida é o saldo do lote. Na emissão, o item aponta de quais lotes ele
            saiu — a quantidade de cada um é rateada da quantidade vendida, não digitada de novo.
          </p>
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
