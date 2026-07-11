'use client'

import {useState} from 'react'
import {useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage,} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {NumericInput} from '@/components/ui/numeric-input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {
  BODYWORK_OPTIONS,
  OWNER_TYPE_OPTIONS,
  ROLE_OPTIONS,
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
  /** Missing fields returned by GET /vehicles/:sk/requirements — when set, the
   * advanced section auto-expands so the user can fix them immediately. */
  highlightFields?: string[]
}

function fromOut(v: VehicleOut): VehicleFormData {
  return {
    plate: v.plate,
    plate_uf: v.plate_uf as VehicleFormData['plate_uf'],
    role: v.role,
    wheelset: v.wheelset ?? '',
    bodywork: v.bodywork ?? '',
    renavam: v.renavam ?? '',
    weight: v.weight ? String(v.weight) : '',
    cap_kg: v.cap_kg ? String(v.cap_kg) : '',
    cap_m3: v.cap_m3 ? String(v.cap_m3) : '',
    cint: v.cint ?? '',
    owner: v.owner
      ? {cpf_cnpj: v.owner.cpf_cnpj, rntrc: v.owner.rntrc, name: v.owner.name, type: v.owner.type as 'TAC' | 'ETC' | 'CTC'}
      : undefined,
  }
}

export function VehicleForm({ initialData, onSubmit, loading = false, highlightFields = [] }: VehicleFormProps) {
  const [advancedOpen, setAdvancedOpen] = useState(highlightFields.length > 0)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const form = useForm<VehicleFormData>({
    resolver: zodResolver(vehicleSchema),
    defaultValues: initialData
      ? fromOut(initialData)
      : {
          plate: '',
          plate_uf: 'SP',
          role: 'tractor',
          wheelset: '',
          bodywork: '',
          renavam: '',
          weight: '',
          cap_kg: '',
          cap_m3: '',
          cint: '',
        },
  })

  const role = useWatch({ control: form.control, name: 'role' })
  const wantsOwner = useWatch({ control: form.control, name: 'owner' })
  const ownerCpf = useWatch({ control: form.control, name: 'owner.cpf_cnpj' })

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      const payload: VehicleCreate = {
        plate: data.plate,
        plate_uf: data.plate_uf,
        role: data.role,
        wheelset: data.wheelset || undefined,
        bodywork: data.bodywork || undefined,
        renavam: data.renavam || undefined,
        weight: data.weight ? Number(data.weight) : undefined,
        cap_kg: data.cap_kg ? Number(data.cap_kg) : undefined,
        cap_m3: data.cap_m3 ? Number(data.cap_m3) : undefined,
        cint: data.cint || undefined,
        owner: data.owner ? {...data.owner, cpf_cnpj: data.owner.cpf_cnpj.replace(/\D/g, '')} : undefined,
      }
      await onSubmit(payload)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  const isMissing = (field: string) => highlightFields.includes(field)

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-6">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}
        {highlightFields.length > 0 && (
          <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
            Este veículo está incompleto para este documento. Preencha os campos destacados abaixo.
          </div>
        )}

        {/* Identificação */}
        <section className="space-y-3">
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Identificação</p>
          <FormField
            control={form.control}
            name="role"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Tipo de veículo *</FormLabel>
                <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={ROLE_OPTIONS} />
                <FormMessage />
              </FormItem>
            )}
          />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <FormField
              control={form.control}
              name="plate"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Placa (Mercosul) *</FormLabel>
                  <Input {...field} id={field.name} placeholder="ABC1D23" maxLength={7}
                         onChange={(e) => field.onChange(e.target.value.toUpperCase())} />
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
                  <OptionsSelect id={field.name} value={field.value} onValueChange={field.onChange} options={UF_OPTIONS} />
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </section>

        {/* Configurações avançadas */}
        <section className="space-y-3">
          <Button type="button" variant="ghost" size="xs" onClick={() => setAdvancedOpen(!advancedOpen)}
                  className="text-brand-600 hover:text-brand-700">
            {advancedOpen ? '− Ocultar configurações avançadas' : '+ Configurações avançadas'}
          </Button>

          {advancedOpen && (
            <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {role === 'tractor' && (
                  <FormField
                    control={form.control}
                    name="wheelset"
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel className={isMissing('wheelset') ? 'text-amber-700' : ''}>Tipo de eixo</FormLabel>
                        <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange} options={WHEELSET_OPTIONS} />
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name="bodywork"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('bodywork') ? 'text-amber-700' : ''}>Carroceria</FormLabel>
                      <OptionsSelect id={field.name} value={field.value ?? ''} onValueChange={field.onChange} options={BODYWORK_OPTIONS} />
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
                      <FormLabel>RENAVAM</FormLabel>
                      <NumericInput {...field} id={field.name} placeholder="12345678901" maxLength={11} onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="weight"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('weight') ? 'text-amber-700' : ''}>Tara</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="KG" placeholder="12000" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField
                  control={form.control}
                  name="cap_kg"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className={isMissing('cap_kg') ? 'text-amber-700' : ''}>Capacidade (KG)</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="KG" placeholder="9000" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="cap_m3"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Capacidade (M³)</FormLabel>
                      <NumericInput {...field} id={field.name} suffix="M³" placeholder="40" onChange={field.onChange} />
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name="cint"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Código interno</FormLabel>
                    <Input {...field} id={field.name} placeholder="Opcional" maxLength={10} />
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className="space-y-3 border-t border-gray-200 pt-3">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Proprietário (se não for da própria empresa)</p>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => form.setValue('owner', wantsOwner ? undefined : {cpf_cnpj: '', rntrc: '', name: '', type: 'TAC'})}>
                    {wantsOwner ? 'Remover' : '+ Adicionar'}
                  </Button>
                </div>
                {wantsOwner && (
                  <div className="space-y-3">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <FormField
                        control={form.control}
                        name="owner.cpf_cnpj"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>CPF/CNPJ do proprietário</FormLabel>
                            <Input ref={field.ref} name={field.name} onBlur={field.onBlur} id={field.name}
                                   value={ownerCpf ?? ''} placeholder="000.000.000-00" maxLength={18}
                                   onChange={(e) => form.setValue('owner.cpf_cnpj', maskCpfCnpj(e.target.value))} />
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name="owner.rntrc"
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>RNTRC</FormLabel>
                            <NumericInput {...field} id={field.name} placeholder="12345678" maxLength={12} />
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>
                    <FormField
                      control={form.control}
                      name="owner.name"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Nome</FormLabel>
                          <Input {...field} id={field.name} placeholder="Proprietário LTDA" maxLength={255} />
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                    <FormField
                      control={form.control}
                      name="owner.type"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>Tipo</FormLabel>
                          <OptionsSelect id={field.name} value={field.value ?? 'TAC'} onValueChange={field.onChange} options={OWNER_TYPE_OPTIONS} />
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                )}
              </div>
            </div>
          )}
        </section>

        <Button type="submit" disabled={loading} className="w-full">
          {loading ? 'Salvando...' : initialData ? 'Salvar alterações' : 'Cadastrar veículo'}
        </Button>
      </form>
    </Form>
  )
}
