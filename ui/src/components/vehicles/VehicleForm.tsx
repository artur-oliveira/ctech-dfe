'use client'

import {useState} from 'react'
import type {Control, FieldPath} from 'react-hook-form'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {
  BODYWORK_OPTIONS,
  OWNER_TYPE_OPTIONS,
  type TrailerFormData,
  UF_OPTIONS,
  type VehicleFormData,
  vehicleSchema,
  WHEELSET_OPTIONS,
} from '@/lib/schemas/vehicles'
import type {VehicleCreate, VehicleOut} from '@/lib/types/api'
import {maskCpfCnpj} from '@/lib/utils/masks'

interface VehicleFormProps {
  initialData?: VehicleOut
  onSubmit: (data: VehicleCreate) => Promise<void>
  loading?: boolean
}

type TrailerInput = Omit<TrailerFormData, 'weight'> & { weight: string }

const emptyOwner = () => ({ cpf_cnpj: '', rntrc: '', name: '', type: 'TAC' as const })
const emptyTrailer = (): TrailerInput => ({
  plate: '',
  plate_uf: 'SP',
  wheelset: '01',
  bodywork: '01',
  renavam: '',
  weight: '',
  owner: emptyOwner(),
})

function fromOut(v: VehicleOut): VehicleFormData {
  return {
    plate: v.plate,
    plate_uf: v.plate_uf as VehicleFormData['plate_uf'],
    wheelset: v.wheelset,
    bodywork: v.bodywork,
    renavam: v.renavam,
    weight: String(v.weight),
    owner: {
      cpf_cnpj: v.owner.cpf_cnpj,
      rntrc: v.owner.rntrc,
      name: v.owner.name,
      type: v.owner.type as 'TAC' | 'ETC' | 'CTC',
    },
    trailers: v.trailers.map((t) => ({
      plate: t.plate,
      plate_uf: t.plate_uf as VehicleFormData['plate_uf'],
      wheelset: t.wheelset,
      bodywork: t.bodywork,
      renavam: t.renavam,
      weight: String(t.weight),
      owner: {
        cpf_cnpj: t.owner.cpf_cnpj,
        rntrc: t.owner.rntrc,
        name: t.owner.name,
        type: t.owner.type as 'TAC' | 'ETC' | 'CTC',
      },
    })),
  }
}

interface OwnerFieldsProps {
  control: Control<VehicleFormData>
  namePrefix: 'owner' | `trailers.${number}.owner`
  cpfValue: string
  onCpfChange: (v: string) => void
}

function OwnerFields({ control, namePrefix, cpfValue, onCpfChange }: OwnerFieldsProps) {
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <FormField
          control={control}
          name={`${namePrefix}.cpf_cnpj` as FieldPath<VehicleFormData>}
          render={({ field }) => (
            <FormItem>
              <FormLabel>CPF/CNPJ do proprietário *</FormLabel>
              <Input
                ref={field.ref}
                name={field.name as string}
                onBlur={field.onBlur}
                id={field.name as string}
                value={cpfValue}
                placeholder="000.000.000-00"
                maxLength={18}
                onChange={(e) => onCpfChange(maskCpfCnpj(e.target.value))}
              />
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={control}
          name={`${namePrefix}.rntrc` as FieldPath<VehicleFormData>}
          render={({ field }) => (
            <FormItem>
              <FormLabel>RNTRC *</FormLabel>
              <NumericInput
                ref={field.ref}
                name={field.name as string}
                onBlur={field.onBlur}
                onChange={field.onChange}
                value={field.value as string}
                id={field.name as string}
                placeholder="12345678"
                maxLength={12}
              />
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
      <FormField
        control={control}
        name={`${namePrefix}.name` as FieldPath<VehicleFormData>}
        render={({ field }) => (
          <FormItem>
            <FormLabel>Nome *</FormLabel>
            <Input
              ref={field.ref}
              name={field.name as string}
              onBlur={field.onBlur}
              onChange={field.onChange}
              value={field.value as string}
              id={field.name as string}
              placeholder="Proprietário LTDA"
              maxLength={255}
            />
            <FormMessage />
          </FormItem>
        )}
      />
      <FormField
        control={control}
        name={`${namePrefix}.type` as FieldPath<VehicleFormData>}
        render={({ field }) => (
          <FormItem>
            <FormLabel>Tipo *</FormLabel>
            <OptionsSelect
              id={field.name as string}
              value={field.value as string}
              onValueChange={field.onChange}
              options={OWNER_TYPE_OPTIONS}
            />
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}

export function VehicleForm({ initialData, onSubmit, loading = false }: VehicleFormProps) {
  const [trailerOpen, setTrailerOpen] = useState(false)
  const [newTrailer, setNewTrailer] = useState<TrailerInput>(emptyTrailer)
  const [trailerErrors, setTrailerErrors] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<VehicleFormData>({
    resolver: zodResolver(vehicleSchema),
    defaultValues: initialData
      ? fromOut(initialData)
      : {
          plate: '',
          plate_uf: 'SP',
          wheelset: '01',
          bodywork: '01',
          renavam: '',
          weight: '',
          owner: emptyOwner(),
          trailers: [],
        },
  })

  const trailers = useWatch({ control: form.control, name: 'trailers' })
  const ownerCpf = useWatch({ control: form.control, name: 'owner.cpf_cnpj' })

  const addTrailer = () => {
    if (!newTrailer.plate || !newTrailer.renavam || !newTrailer.weight || !newTrailer.owner.cpf_cnpj) {
      setTrailerErrors('Preencha placa, RENAVAM, tara e CPF/CNPJ do proprietário')
      return
    }
    if (!/^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/.test(newTrailer.plate)) {
      setTrailerErrors('Placa Mercosul inválida')
      return
    }
    setTrailerErrors(null)
    form.setValue('trailers', [...trailers, newTrailer as unknown as TrailerFormData])
    setNewTrailer(emptyTrailer())
    setTrailerOpen(false)
  }

  const removeTrailer = (i: number) => {
    form.setValue('trailers', trailers.filter((_, idx) => idx !== i))
  }

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      const payload: VehicleCreate = {
        plate: data.plate,
        plate_uf: data.plate_uf,
        wheelset: data.wheelset,
        bodywork: data.bodywork,
        renavam: data.renavam,
        weight: Number(data.weight),
        owner: {
          ...data.owner,
          cpf_cnpj: data.owner.cpf_cnpj.replace(/\D/g, ''),
        },
        trailers: data.trailers.map((t) => ({
          ...t,
          weight: Number(t.weight),
          owner: { ...t.owner, cpf_cnpj: t.owner.cpf_cnpj.replace(/\D/g, '') },
        })),
      }
      await onSubmit(payload)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-6">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}

        {/* Identificação */}
        <section className="space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
            Identificação
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField
              control={form.control}
              name="plate"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Placa (Mercosul) *</FormLabel>
                  <Input
                    {...field}
                    id={field.name}
                    placeholder="ABC1D23"
                    maxLength={7}
                    onChange={(e) => field.onChange(e.target.value.toUpperCase())}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="plate_uf"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>UF da placa *</FormLabel>
                  <OptionsSelect
                    id={field.name}
                    value={field.value}
                    onValueChange={field.onChange}
                    options={UF_OPTIONS}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField
              control={form.control}
              name="wheelset"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Tipo de eixo *</FormLabel>
                  <OptionsSelect
                    id={field.name}
                    value={field.value}
                    onValueChange={field.onChange}
                    options={WHEELSET_OPTIONS}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="bodywork"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Carroceria *</FormLabel>
                  <OptionsSelect
                    id={field.name}
                    value={field.value}
                    onValueChange={field.onChange}
                    options={BODYWORK_OPTIONS}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField
              control={form.control}
              name="renavam"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>RENAVAM *</FormLabel>
                  <NumericInput
                    {...field}
                    id={field.name}
                    placeholder="12345678901"
                    maxLength={11}
                    onChange={field.onChange}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="weight"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Tara *</FormLabel>
                  <NumericInput
                    {...field}
                    id={field.name}
                    suffix="KG"
                    placeholder="12000"
                    onChange={field.onChange}
                  />
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </section>

        {/* Proprietário */}
        <section className="space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
            Proprietário
          </p>
          <OwnerFields
            control={form.control}
            namePrefix="owner"
            cpfValue={ownerCpf}
            onCpfChange={(v) => form.setValue('owner.cpf_cnpj', v)}
          />
        </section>

        {/* Reboques */}
        <section className="space-y-2">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
              Reboques ({trailers.length})
            </p>
            <Button
              type="button"
              variant="ghost"
              size="xs"
              onClick={() => setTrailerOpen(!trailerOpen)}
              className="text-brand-600 hover:text-brand-700"
            >
              {trailerOpen ? 'Cancelar' : '+ Adicionar reboque'}
            </Button>
          </div>

          {trailerOpen && (
            <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="grid gap-1">
                  <Label>Placa</Label>
                  <Input
                    value={newTrailer.plate}
                    placeholder="XYZ1A23"
                    maxLength={7}
                    onChange={(e) =>
                      setNewTrailer((t) => ({ ...t, plate: e.target.value.toUpperCase() }))
                    }
                  />
                </div>
                <div className="grid gap-1">
                  <Label>UF</Label>
                  <OptionsSelect
                    value={newTrailer.plate_uf}
                    onValueChange={(v) => setNewTrailer((t) => ({ ...t, plate_uf: v as typeof newTrailer.plate_uf }))}
                    options={UF_OPTIONS}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="grid gap-1">
                  <Label>Eixo</Label>
                  <OptionsSelect
                    value={newTrailer.wheelset}
                    onValueChange={(v) => setNewTrailer((t) => ({ ...t, wheelset: v }))}
                    options={WHEELSET_OPTIONS}
                  />
                </div>
                <div className="grid gap-1">
                  <Label>Carroceria</Label>
                  <OptionsSelect
                    value={newTrailer.bodywork}
                    onValueChange={(v) => setNewTrailer((t) => ({ ...t, bodywork: v }))}
                    options={BODYWORK_OPTIONS}
                  />
                </div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="grid gap-1">
                  <Label>RENAVAM</Label>
                  <NumericInput
                    value={newTrailer.renavam}
                    placeholder="987654321"
                    maxLength={11}
                    onChange={(v) => setNewTrailer((t) => ({ ...t, renavam: v }))}
                  />
                </div>
                <div className="grid gap-1">
                  <Label>Tara</Label>
                  <NumericInput
                    value={newTrailer.weight}
                    suffix="KG"
                    placeholder="8000"
                    onChange={(v) => setNewTrailer((t) => ({ ...t, weight: v }))}
                  />
                </div>
              </div>

              <div className="space-y-3">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="grid gap-1">
                    <Label>CPF/CNPJ</Label>
                    <Input
                      value={newTrailer.owner.cpf_cnpj}
                      placeholder="000.000.000-00"
                      maxLength={18}
                      onChange={(e) =>
                        setNewTrailer((t) => ({
                          ...t,
                          owner: { ...t.owner, cpf_cnpj: maskCpfCnpj(e.target.value) },
                        }))
                      }
                    />
                  </div>
                  <div className="grid gap-1">
                    <Label>RNTRC</Label>
                    <NumericInput
                      value={newTrailer.owner.rntrc}
                      placeholder="12345678"
                      maxLength={12}
                      onChange={(v) =>
                        setNewTrailer((t) => ({ ...t, owner: { ...t.owner, rntrc: v } }))
                      }
                    />
                  </div>
                </div>
                <div className="grid gap-1">
                  <Label>Nome</Label>
                  <Input
                    value={newTrailer.owner.name}
                    placeholder="Proprietário LTDA"
                    onChange={(e) =>
                      setNewTrailer((t) => ({ ...t, owner: { ...t.owner, name: e.target.value } }))
                    }
                  />
                </div>
                <div className="grid gap-1">
                  <Label>Tipo</Label>
                  <OptionsSelect
                    value={newTrailer.owner.type}
                    onValueChange={(v) =>
                      setNewTrailer((t) => ({
                        ...t,
                        owner: { ...t.owner, type: v as 'TAC' | 'ETC' | 'CTC' },
                      }))
                    }
                    options={OWNER_TYPE_OPTIONS}
                  />
                </div>
              </div>

              {trailerErrors && (
                <p className="text-[0.8rem] font-medium text-destructive">{trailerErrors}</p>
              )}

              <Button
                type="button"
                onClick={addTrailer}
                className="w-full bg-gray-700 hover:bg-gray-800 text-white"
              >
                Confirmar reboque
              </Button>
            </div>
          )}

          {trailers.map((t, i) => (
            <div
              key={i}
              className="flex items-center justify-between rounded-lg bg-gray-50 px-4 py-2.5 text-sm"
            >
              <span className="font-medium text-gray-700">
                {t.plate} · {t.plate_uf} ·{' '}
                {WHEELSET_OPTIONS.find((o) => o.value === t.wheelset)?.label ?? t.wheelset}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="xs"
                onClick={() => removeTrailer(i)}
                className="ml-4 text-red-500 hover:text-red-700"
              >
                remover
              </Button>
            </div>
          ))}
        </section>

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? 'Salvando...' : initialData ? 'Salvar alterações' : 'Cadastrar veículo'}
        </Button>
      </form>
    </Form>
  )
}
