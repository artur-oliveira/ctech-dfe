'use client'

import {useEffect, useState} from 'react'
import type {Resolver} from 'react-hook-form'
import {useFieldArray, useForm, useWatch} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'
import {OptionsSelect} from '@/components/ui/options-select'
import {Button} from '@/components/ui/button'
import {AddressFields} from '@/components/ui/address-fields'
import {SectionCard} from '@/components/ui/section-card'
import {CnpjLookupBadge} from '@/components/ui/cnpj-lookup-badge'
import {
  CRT_NONE_VALUE,
  CRT_OPTIONS_ORG_PF,
  CRT_OPTIONS_PJ,
  type EntityFormData,
  entitySchema,
  UF_OPTIONS,
} from '@/lib/schemas/entity'
import {maskCnpj, maskCpf, maskPhone} from '@/lib/utils/masks'
import {useCnpjLookup} from '@/lib/hooks/useCnpjLookup'
import {useAuth} from '@/lib/hooks/useAuth'
import {Crt} from "@/lib/types/api";

type Tipo = 'pf' | 'pj'

export interface EntityFormProps {
  /** 'person' hides description; 'organization' shows both. */
  variant: 'person' | 'organization'
  initialData?: EntityFormData
  /** For edit mode: the org/person pk (e.g. "CNPJ_12345…") used to lock tipo */
  entityPk?: string
  /** Force the PF/PJ type and hide the toggle (e.g. NFC-e consumer = always PF). */
  lockTipo?: Tipo
  /** Prefill the document field (used together with lockTipo). */
  initialCpfCnpj?: string
  onSubmit: (data: EntityFormData) => Promise<void>
  loading?: boolean
}

/* ── Icons ───────────────────────────────────────────────────────────── */
const BuildingIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="9" width="18" height="13"/>
    <path d="M9 22V12h6v10"/>
    <path d="M3 9l9-7 9 7"/>
  </svg>
)
const UserIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
    <circle cx="12" cy="7" r="4"/>
  </svg>
)
const MapPinIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
       strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z"/>
    <circle cx="12" cy="10" r="3"/>
  </svg>
)
const MailIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
       strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="16" rx="2"/>
    <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/>
  </svg>
)
const PhoneIcon = () => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
       strokeLinecap="round" strokeLinejoin="round">
    <path
      d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07A19.5 19.5 0 0 1 4.69 12a19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 3.62 2h3a2 2 0 0 1 2 1.72c.127.96.361 1.903.7 2.81a2 2 0 0 1-.45 2.11L7.91 9.91a16 16 0 0 0 6.1 6.1l1.27-.95a2 2 0 0 1 2.11-.45c.907.339 1.85.573 2.81.7A2 2 0 0 1 22 16.92z"/>
  </svg>
)
const PlusIcon = () => (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"
       strokeLinecap="round" strokeLinejoin="round">
    <line x1="12" y1="5" x2="12" y2="19"/>
    <line x1="5" y1="12" x2="19" y2="12"/>
  </svg>
)
const XIcon = () => (
  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"
       strokeLinecap="round" strokeLinejoin="round">
    <line x1="18" y1="6" x2="6" y2="18"/>
    <line x1="6" y1="6" x2="18" y2="18"/>
  </svg>
)

const EMPTY_ADDRESS: EntityFormData['person']['addresses'][number] = {
  city_ibge_code: '', street: '', neighborhood: '', number: '',
  city: '', state_federation: 'SP', postal_code: '', complement: '',
}

const DEFAULT_VALUES: EntityFormData = {
  tipo: 'pj',
  cpf_or_cnpj: '',
  name: '',
  description: '',
  person: {
    fantasy_name: '',
    crt: '1',
    state_registrations: [],
    addresses: [EMPTY_ADDRESS],
    contacts: {emails: [], phones: []},
  },
}

function deriveTipo(entityPk?: string, data?: EntityFormData): Tipo {
  if (entityPk) return entityPk.startsWith('CPF_') ? 'pf' : 'pj'
  return data?.tipo ?? 'pj'
}

/* ── Component ───────────────────────────────────────────────────────── */
export function EntityForm({variant, initialData, entityPk, lockTipo, initialCpfCnpj, onSubmit, loading = false}: EntityFormProps) {
  const isEdit = !!initialData
  const isOrg = variant === 'organization'
  const [tipo, setTipo] = useState<Tipo>(() => lockTipo ?? deriveTipo(entityPk, initialData))
  const isPJ = tipo === 'pj'
  const [submitError, setSubmitError] = useState<string | null>(null)

  const {selectedOrg} = useAuth()
  const orgUf = selectedOrg?.state_federation ?? 'SP'
  const {state: lookupState, lookup, reset: resetLookup} = useCnpjLookup()

  // Contact state (only rendered for organizations)
  const [emailInput, setEmailInput] = useState('')
  const [phoneInput, setPhoneInput] = useState('')

  const form = useForm<EntityFormData>({
    resolver: zodResolver(entitySchema) as Resolver<EntityFormData>,
    defaultValues: initialData ?? {
      ...DEFAULT_VALUES,
      ...(lockTipo ? {tipo: lockTipo} : {}),
      ...(initialCpfCnpj ? {cpf_or_cnpj: initialCpfCnpj} : {}),
      person: {
        ...DEFAULT_VALUES.person,
        crt: lockTipo === 'pf' ? '4' : DEFAULT_VALUES.person.crt,
      },
    },
  })

  const {fields: addressFields, append: appendAddress, remove: removeAddress} = useFieldArray({
    control: form.control, name: 'person.addresses',
  })
  const {fields: ieFields, append: appendIE, remove: removeIE} = useFieldArray({
    control: form.control, name: 'person.state_registrations',
  })

  const watchedIEs = useWatch({control: form.control, name: 'person.state_registrations'}) ?? []
  const selectedUFs = watchedIEs.map((r) => r?.uf).filter(Boolean) as string[]
  const emails = useWatch({control: form.control, name: 'person.contacts.emails'}) ?? []
  const phones = useWatch({control: form.control, name: 'person.contacts.phones'}) ?? []

  // Autofill from SEFAZ lookup
  useEffect(() => {
    if (lookupState.status !== 'found' || !lookupState.result) return
    const r = lookupState.result
    form.setValue('name', r.name, {shouldValidate: true})
    form.setValue('person.fantasy_name', r.name, {shouldValidate: true})
    form.setValue('person.crt', (r.crt ?? '1').toString() as Crt, {shouldValidate: true})
    if (r.state_registrations.length > 0) {
      form.setValue('person.state_registrations', r.state_registrations.map((sr) => ({
        uf: sr.uf as EntityFormData['person']['state_registrations'][number]['uf'],
        state_registration: sr.state_registration,
      })), {shouldValidate: true})
    }
    if (r.addresses.length > 0) {
      const addr = r.addresses[0]
      if (addr.state_federation) {
        form.setValue('person.addresses.0.street', addr.street ?? '', {shouldValidate: true})
        form.setValue('person.addresses.0.number', addr.number ?? '', {shouldValidate: true})
        form.setValue('person.addresses.0.neighborhood', addr.neighborhood ?? '', {shouldValidate: true})
        form.setValue('person.addresses.0.complement', addr.complement ?? '', {shouldValidate: true})
        form.setValue('person.addresses.0.city', addr.city ?? '', {shouldValidate: true})
        form.setValue('person.addresses.0.state_federation', addr.state_federation as EntityFormData['person']['addresses'][number]['state_federation'], {shouldValidate: true})
        if (addr.postal_code) form.setValue('person.addresses.0.postal_code', addr.postal_code.replace(/\D/g, ''), {shouldValidate: true})
        if (addr.city_ibge_code) form.setValue('person.addresses.0.city_ibge_code', addr.city_ibge_code, {shouldValidate: true})
      }
    }
  }, [lookupState.status, lookupState.result, isPJ, form])

  const switchTipo = (next: Tipo) => {
    setTipo(next)
    form.setValue('tipo', next)
    form.setValue('cpf_or_cnpj', '')
    form.setValue('person.state_registrations', [])
    // Person PF: CRT is optional ("Não especificar"). Org PF (MEI) keeps a default.
    form.setValue('person.crt', next === 'pf' ? (isOrg ? '4' : CRT_NONE_VALUE) : '1')
    resetLookup()
  }

  const addEmail = () => {
    if (!emailInput || emails.length >= 5) return
    form.setValue('person.contacts.emails', [...emails, emailInput], {shouldValidate: true})
    setEmailInput('')
  }
  const removeEmail = (i: number) =>
    form.setValue('person.contacts.emails', emails.filter((_, idx) => idx !== i), {shouldValidate: true})

  const addPhone = () => {
    const raw = phoneInput.replace(/\D/g, '')
    if (raw.length < 10 || phones.length >= 5) return
    form.setValue('person.contacts.phones', [...phones, raw], {shouldValidate: true})
    setPhoneInput('')
  }
  const removePhone = (i: number) =>
    form.setValue('person.contacts.phones', phones.filter((_, idx) => idx !== i), {shouldValidate: true})

  const handleSubmit = form.handleSubmit(async (data) => {
    setSubmitError(null)
    try {
      await onSubmit(data)
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'Erro ao salvar')
    }
  })

  // Pessoa física (person variant) may leave CRT unspecified — backend omits it
  // and defaults to Simples Nacional on emission. Org PF (MEI) still picks a regime.
  const allowNoCrt = variant === 'person' && !isPJ
  const crtOptions = isPJ
    ? CRT_OPTIONS_PJ
    : allowNoCrt
      ? [{value: CRT_NONE_VALUE, label: 'Não especificar'}, ...CRT_OPTIONS_ORG_PF]
      : CRT_OPTIONS_ORG_PF

  return (
    <Form {...form}>
      <form onSubmit={handleSubmit} className="space-y-5">
        {submitError && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {submitError}
          </div>
        )}

        {/* PJ / PF toggle */}
        {!isEdit && !lockTipo && (
          <div className="inline-flex rounded-lg border border-gray-200 bg-gray-100 p-1 gap-1">
            {(['pj', 'pf'] as Tipo[]).map((t) => (
              <button key={t} type="button" onClick={() => switchTipo(t)}
                      className={['flex items-center gap-2 rounded-md px-4 py-1.5 text-sm font-medium transition-all',
                        tipo === t ? 'bg-white shadow-sm text-gray-900' : 'text-gray-500 hover:text-gray-700',
                      ].join(' ')}>
                {t === 'pj' ? <BuildingIcon/> : <UserIcon/>}
                {t === 'pj' ? 'Pessoa Jurídica' : 'Pessoa Física'}
              </button>
            ))}
          </div>
        )}

        {/* Identificação + endereço principal lado a lado */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">

          {/* Identificação */}
          <SectionCard icon={isPJ ? <BuildingIcon/> : <UserIcon/>} title="Identificação">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <FormField control={form.control as never} name="cpf_or_cnpj"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>{isPJ ? 'CNPJ *' : 'CPF *'}</FormLabel>
                             <Input
                               id={field.name}
                               disabled={isEdit}
                               maxLength={isPJ ? 18 : 14}
                               placeholder={isPJ ? '00.000.000/0000-00' : '000.000.000-00'}
                               value={isPJ ? maskCnpj(String(field.value ?? '')) : maskCpf(String(field.value ?? ''))}
                               onChange={(e) => {
                                 const raw = isPJ
                                   ? e.target.value.replace(/[^A-Z0-9]/gi, '').toUpperCase().slice(0, 14)
                                   : e.target.value.replace(/\D/g, '').slice(0, 11)
                                 field.onChange(raw)
                                 const expectedLen = isPJ ? 14 : 11
                                 if (raw.length === expectedLen) void lookup(raw, orgUf)
                                 else if (lookupState.status !== 'idle') resetLookup()
                               }}
                               onBlur={field.onBlur}
                               ref={field.ref}
                             />
                             {!isEdit && <CnpjLookupBadge state={lookupState}/>}
                             <FormMessage/>
                           </FormItem>
                         )}
              />
              <FormField control={form.control as never} name="name"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>{isPJ ? 'Razão Social *' : 'Nome *'}</FormLabel>
                             <Input {...field} id={field.name}
                                    placeholder={isPJ ? 'Empresa LTDA' : 'João da Silva'} maxLength={255}
                                    onChange={variant === 'person'
                                      ? (e) => field.onChange(e.target.value.toUpperCase())
                                      : field.onChange}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            </div>

            {isPJ && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField control={form.control as never} name="person.fantasy_name"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Nome Fantasia *</FormLabel>
                               <Input {...field} id={field.name} value={field.value ?? ''} placeholder="Minha Loja"
                                      maxLength={255}/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
                {isOrg ? (
                  <FormField control={form.control as never} name="description"
                             render={({field}) => (
                               <FormItem>
                                 <FormLabel>Apelido interno</FormLabel>
                                 <Input {...field} id={field.name} value={field.value ?? ''}
                                        placeholder="Ex: Filial Sul" maxLength={120}/>
                                 <FormMessage/>
                               </FormItem>
                             )}
                  />
                ) : null}
              </div>
            )}

            {!isPJ && isOrg && (
              <FormField control={form.control as never} name="description"
                         render={({field}) => (
                           <FormItem>
                             <FormLabel>Apelido interno</FormLabel>
                             <Input {...field} id={field.name} value={field.value ?? ''} placeholder="Ex: MEI Principal"
                                    maxLength={120}/>
                             <FormMessage/>
                           </FormItem>
                         )}
              />
            )}

            {/* CRT */}
            {crtOptions.length > 0 && (
              <div className="pt-1 border-t border-gray-100">
                <p className="text-xs font-semibold uppercase tracking-wider text-gray-400 mb-3">Fiscal</p>
                <FormField control={form.control as never} name="person.crt"
                           render={({field}) => (
                             <FormItem>
                               <FormLabel>Regime tributário (CRT){allowNoCrt ? '' : ' *'}</FormLabel>
                               <OptionsSelect id={field.name}
                                              value={field.value ?? (allowNoCrt ? CRT_NONE_VALUE : '')}
                                              onValueChange={field.onChange}
                                              options={crtOptions}/>
                               <FormMessage/>
                             </FormItem>
                           )}
                />
              </div>
            )}

            {/* Inscrições Estaduais — só PJ */}
            {isPJ && (
              <div className="pt-1 border-t border-gray-100">
                <div className="flex items-center justify-between mb-2">
                  <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
                    Inscrições Estaduais
                  </p>
                  <Button type="button" variant="ghost" size="xs"
                          onClick={() => {
                            const first = UF_OPTIONS.find((o) => !selectedUFs.includes(o.value))?.value ?? 'SP'
                            appendIE({
                              uf: first as EntityFormData['person']['state_registrations'][number]['uf'],
                              state_registration: ''
                            })
                          }}
                          disabled={selectedUFs.length >= UF_OPTIONS.length}
                          className="gap-1 text-brand-600 hover:text-brand-700">
                    <PlusIcon/> Adicionar
                  </Button>
                </div>
                {ieFields.length === 0 && <p className="text-xs text-gray-400">Nenhuma IE cadastrada.</p>}
                {ieFields.map((field, index) => {
                  const ufOpts = UF_OPTIONS.filter((o) => !selectedUFs.includes(o.value) || o.value === watchedIEs[index]?.uf)
                  return (
                    <div key={field.id} className="flex items-end gap-2 mb-2">
                      <FormField control={form.control as never} name={`person.state_registrations.${index}.uf`}
                                 render={({field: f}) => (
                                   <FormItem className="w-20 shrink-0">
                                     {index === 0 && <FormLabel htmlFor={f.name}>UF</FormLabel>}
                                     <OptionsSelect id={f.name} value={f.value} onValueChange={f.onChange} options={ufOpts}/>
                                     <FormMessage/>
                                   </FormItem>
                                 )}
                      />
                      <FormField control={form.control as never}
                                 name={`person.state_registrations.${index}.state_registration`}
                                 render={({field: f}) => (
                                   <FormItem className="flex-1">
                                     {index === 0 && <FormLabel>IE</FormLabel>}
                                     <Input {...f} placeholder="Número da IE" maxLength={20}/>
                                     <FormMessage/>
                                   </FormItem>
                                 )}
                      />
                      <Button type="button" variant="ghost" size="icon-xs" onClick={() => removeIE(index)}
                              className="mb-0.5 text-gray-400 hover:text-red-500 hover:bg-red-50">
                        <XIcon/>
                      </Button>
                    </div>
                  )
                })}
              </div>
            )}
          </SectionCard>

          {/* Endereço principal */}
          {addressFields[0] && (
            <SectionCard icon={<MapPinIcon/>} title="Endereço Principal">
              <AddressFields control={form.control} setValue={form.setValue}
                             basePath="person.addresses.0"/>
            </SectionCard>
          )}
        </div>

        {/* Endereços adicionais */}
        {addressFields.slice(1).map((field, idx) => (
          <SectionCard key={field.id} icon={<MapPinIcon/>} title={`Endereço ${idx + 2}`} className="relative">
            <Button type="button" variant="ghost" size="icon-sm" onClick={() => removeAddress(idx + 1)}
                    className="absolute top-3 right-4 text-gray-400 hover:text-red-500 hover:bg-red-50">
              <XIcon/>
            </Button>
            <AddressFields control={form.control} setValue={form.setValue}
                           basePath={`person.addresses.${idx + 1}`}/>
          </SectionCard>
        ))}

        <Button type="button" variant="ghost" size="sm" onClick={() => appendAddress(EMPTY_ADDRESS)}
                className="gap-1.5 text-brand-600 hover:text-brand-700 px-0">
          <PlusIcon/> Adicionar endereço
        </Button>

        <SectionCard icon={<MailIcon/>} title="Contatos">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {/* E-mails */}
            <div className="space-y-2">
              <div className="flex items-center gap-1.5 mb-1 text-sm font-medium text-gray-700">
                <MailIcon/>E-mails <span className="text-xs font-normal text-gray-400">({emails.length}/5)</span>
              </div>
              <div className="flex gap-2">
                <Input type="email" placeholder="email@empresa.com" value={emailInput}
                       onChange={(e) => setEmailInput(e.target.value)}
                       onKeyDown={(e) => {
                         if (e.key === 'Enter') {
                           e.preventDefault();
                           addEmail()
                         }
                       }}
                       disabled={emails.length >= 5}/>
                <Button type="button" variant="outline" size="icon-lg" onClick={addEmail}
                        disabled={emails.length >= 5 || !emailInput}
                        className="shrink-0 text-gray-400 hover:text-gray-600">
                  <PlusIcon/>
                </Button>
              </div>
              {emails.length > 0 && (
                <ul className="space-y-1">
                  {emails.map((e, i) => (
                    <li key={i}
                        className="flex items-center justify-between rounded-lg bg-gray-50 border border-gray-100 px-3 py-2 text-sm">
                      <span className="truncate text-gray-700">{e}</span>
                      <Button type="button" variant="ghost" size="icon-xs" onClick={() => removeEmail(i)}
                              className="ml-3 shrink-0 text-gray-400 hover:text-red-500"><XIcon/>
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
            {/* Telefones */}
            <div className="md:border-l md:border-gray-100 md:pl-6 space-y-2">
              <div className="flex items-center gap-1.5 mb-1 text-sm font-medium text-gray-700">
                <PhoneIcon/>Telefones <span className="text-xs font-normal text-gray-400">({phones.length}/5)</span>
              </div>
              <div className="flex gap-2">
                <Input type="tel" placeholder="(11) 98765-4321" value={phoneInput}
                       onChange={(e) => setPhoneInput(maskPhone(e.target.value))}
                       onKeyDown={(e) => {
                         if (e.key === 'Enter') {
                           e.preventDefault();
                           addPhone()
                         }
                       }}
                       disabled={phones.length >= 5}/>
                <Button type="button" variant="outline" size="icon-lg" onClick={addPhone}
                        disabled={phones.length >= 5 || phoneInput.replace(/\D/g, '').length < 10}
                        className="shrink-0 text-gray-400 hover:text-gray-600">
                  <PlusIcon/>
                </Button>
              </div>
              {phones.length > 0 && (
                <ul className="space-y-1">
                  {phones.map((p, i) => (
                    <li key={i}
                        className="flex items-center justify-between rounded-lg bg-gray-50 border border-gray-100 px-3 py-2 text-sm">
                      <span className="text-gray-700">{maskPhone(p)}</span>
                      <Button type="button" variant="ghost" size="icon-xs" onClick={() => removePhone(i)}
                              className="ml-3 shrink-0 text-gray-400 hover:text-red-500"><XIcon/>
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        </SectionCard>

        <div className="flex justify-end pt-1">
          <Button type="submit" disabled={loading} className="min-w-44">
            {loading ? 'Salvando...' : isEdit ? 'Salvar alterações' : isOrg ? 'Criar organização' : 'Cadastrar pessoa'}
          </Button>
        </div>
      </form>
    </Form>
  )
}
