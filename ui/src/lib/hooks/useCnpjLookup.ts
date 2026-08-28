'use client'

import {useCallback, useRef, useState} from 'react'
import {apiClient, ApiError} from '@/lib/api/client'
import type {
  LookupAddressOut,
  LookupOrganizationOut,
  OpenCnpjOffice,
} from '@/lib/types/api'
import {bfsUfOrder} from '@/lib/utils/uf-graph'
import {useAuth} from '@/lib/hooks/useAuth'

export const CNPJ_LOOKUP_SOURCE = {
  OPEN_CNPJ: 'cnpja',
  SEFAZ: 'sefaz',
} as const

export type CnpjLookupSource = typeof CNPJ_LOOKUP_SOURCE[keyof typeof CNPJ_LOOKUP_SOURCE]
export type CnpjLookupPhase = CnpjLookupSource | null

export interface CnpjLookupConflict {
  field: 'name' | 'address'
  message: string
}

export interface CnpjLookupResult extends LookupOrganizationOut {
  fantasyName: string | null
  cnae: string | null
  isufEmit: string | null
  contacts: {emails: string[]; phones: string[]}
  nfseSimpleOption: '' | '1' | '2' | '3'
  sources: CnpjLookupSource[]
  updatedAt: string | null
  warnings: string[]
  conflicts: CnpjLookupConflict[]
}

export type CnpjLookupStatus =
  | 'idle'
  | 'searching'
  | 'found'
  | 'not_found'
  | 'no_certificate'
  | 'sefaz_rejection'
  | 'error'

export interface CnpjLookupState {
  status: CnpjLookupStatus
  result: CnpjLookupResult | null
  phase: CnpjLookupPhase
  currentUf: string | null
  errorMessage: string | null
}

const NO_CERTIFICATE_TYPE = '/problems/no-certificate'
const SEFAZ_REJECTION_TYPE = '/problems/sefaz-rejection'
const DEFAULT_UF = 'SP'
const CPF_LENGTH = 11
const CNPJ_LENGTH = 14

const INITIAL_STATE: CnpjLookupState = {
  status: 'idle',
  result: null,
  phase: null,
  currentUf: null,
  errorMessage: null,
}

function value(input: string | number | null | undefined): string {
  return input == null ? '' : String(input).trim()
}

function normalizeComparable(input: string | null | undefined): string {
  return value(input).normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/\W/g, '').toUpperCase()
}

function openCnpjCrt(office: OpenCnpjOffice): string {
  if (office.company?.simei?.optant) return '4'
  if (office.company?.simples?.optant) return '1'
  return '3'
}

function openCnpjNfseOption(office: OpenCnpjOffice): CnpjLookupResult['nfseSimpleOption'] {
  if (office.company?.simei?.optant) return '2'
  if (office.company?.simples?.optant) return '3'
  return '1'
}

/** Converte a resposta pública ao mesmo contrato preenchível usado pela SEFAZ. */
export function openCnpjOfficeToLookup(office: OpenCnpjOffice, requestedCnpj: string): CnpjLookupResult {
  const address = office.address
  const state = value(address?.state).toUpperCase()
  const addressResult: LookupAddressOut | null = address
    ? {
      street: value(address.street) || null,
      number: value(address.number) || null,
      complement: value(address.details) || null,
      neighborhood: value(address.district) || null,
      city: value(address.city) || null,
      postal_code: value(address.zip).replace(/\D/g, '') || null,
      state_federation: state || null,
      city_ibge_code: value(address.municipality) || null,
    }
    : null
  const activeSuframa = office.suframa?.find((registration) => registration.active !== false)

  return {
    cpf_cnpj: value(office.taxId).replace(/\D/g, '') || requestedCnpj,
    name: value(office.company?.name),
    fantasyName: value(office.alias) || null,
    crt: openCnpjCrt(office),
    uf: state,
    status: value(office.status?.text),
    addresses: addressResult ? [addressResult] : [],
    state_registrations: [],
    cnae: value(office.mainActivity?.id).replace(/\D/g, '') || null,
    isufEmit: value(activeSuframa?.number).replace(/\D/g, '') || null,
    contacts: {
      emails: (office.emails ?? []).map((email) => value(email.address)).filter(Boolean),
      phones: (office.phones ?? [])
        .map((phone) => `${value(phone.area)}${value(phone.number)}`.replace(/\D/g, ''))
        .filter(Boolean),
    },
    nfseSimpleOption: openCnpjNfseOption(office),
    sources: [CNPJ_LOOKUP_SOURCE.OPEN_CNPJ],
    updatedAt: value(office.updated) || null,
    warnings: [],
    conflicts: [],
  }
}

function sefazToLookup(result: LookupOrganizationOut): CnpjLookupResult {
  return {
    ...result,
    addresses: result.addresses ?? [],
    state_registrations: result.state_registrations ?? [],
    fantasyName: null,
    cnae: null,
    isufEmit: null,
    contacts: {emails: [], phones: []},
    nfseSimpleOption: '',
    sources: [CNPJ_LOOKUP_SOURCE.SEFAZ],
    updatedAt: null,
    warnings: [],
    conflicts: [],
  }
}

function addressesConflict(publicAddress?: LookupAddressOut, sefazAddress?: LookupAddressOut): boolean {
  if (!publicAddress || !sefazAddress) return false
  const publicValue = [publicAddress.street, publicAddress.number, publicAddress.city, publicAddress.state_federation]
    .map(normalizeComparable)
    .join('|')
  const sefazValue = [sefazAddress.street, sefazAddress.number, sefazAddress.city, sefazAddress.state_federation]
    .map(normalizeComparable)
    .join('|')
  return Boolean(publicValue && sefazValue && publicValue !== sefazValue)
}

/** A SEFAZ prevalece nos campos fiscais; o CNPJá complementa cadastro e contato. */
export function mergeCnpjLookupResults(
  openCnpj: CnpjLookupResult | null,
  sefaz: LookupOrganizationOut | null,
  warnings: string[] = [],
): CnpjLookupResult | null {
  if (!openCnpj && !sefaz) return null
  if (!openCnpj && sefaz) return {...sefazToLookup(sefaz), warnings}
  if (!openCnpj) return null
  if (!sefaz) return {...openCnpj, warnings: [...openCnpj.warnings, ...warnings]}

  const conflicts: CnpjLookupConflict[] = []
  if (normalizeComparable(openCnpj.name) !== normalizeComparable(sefaz.name)) {
    conflicts.push({field: 'name', message: 'A razão social diverge entre CNPJá e SEFAZ.'})
  }
  const sefazAddresses = sefaz.addresses ?? []
  const sefazRegistrations = sefaz.state_registrations ?? []
  if (addressesConflict(openCnpj.addresses[0], sefazAddresses[0])) {
    conflicts.push({field: 'address', message: 'O endereço diverge entre CNPJá e SEFAZ.'})
  }

  return {
    ...openCnpj,
    cpf_cnpj: sefaz.cpf_cnpj || openCnpj.cpf_cnpj,
    name: sefaz.name || openCnpj.name,
    crt: sefaz.crt ?? openCnpj.crt,
    uf: sefaz.uf || openCnpj.uf,
    status: sefaz.status || openCnpj.status,
    addresses: sefazAddresses.length > 0 ? sefazAddresses : openCnpj.addresses,
    state_registrations: sefazRegistrations,
    sources: [CNPJ_LOOKUP_SOURCE.OPEN_CNPJ, CNPJ_LOOKUP_SOURCE.SEFAZ],
    warnings: [...openCnpj.warnings, ...warnings],
    conflicts,
  }
}

function problemType(error: ApiError): string | undefined {
  return error.raw && typeof error.raw === 'object'
    ? (error.raw as {type?: string}).type
    : undefined
}

/** Consulta o CNPJá primeiro e, havendo organização ativa, valida na SEFAZ.
 * O CNPJá mantém o primeiro cadastro funcional; a SEFAZ acrescenta a fonte
 * fiscal sem exigir dois fluxos ou dois botões do usuário. */
export function useCnpjLookup() {
  const [state, setState] = useState<CnpjLookupState>(INITIAL_STATE)
  const {selectedOrg} = useAuth()
  const requestIdRef = useRef(0)

  const cancel = useCallback(() => {
    requestIdRef.current += 1
    setState((current) => current.status === 'searching' ? INITIAL_STATE : current)
  }, [])

  const lookup = useCallback(async (cpfCnpj: string, orgUf: string) => {
    const clean = cpfCnpj.replace(/\D/g, '')
    if (clean.length !== CPF_LENGTH && clean.length !== CNPJ_LENGTH) return

    const requestId = requestIdRef.current + 1
    requestIdRef.current = requestId
    const isCurrent = () => requestIdRef.current === requestId
    const warnings: string[] = []
    let openCnpj: CnpjLookupResult | null = null
    let publicError: ApiError | null = null

    setState({
      ...INITIAL_STATE,
      status: 'searching',
      phase: clean.length === CNPJ_LENGTH ? CNPJ_LOOKUP_SOURCE.OPEN_CNPJ : CNPJ_LOOKUP_SOURCE.SEFAZ,
    })

    if (clean.length === CNPJ_LENGTH) {
      try {
        const office = await apiClient.lookupOpenCnpjOffice(clean)
        if (!isCurrent()) return
        openCnpj = openCnpjOfficeToLookup(office, clean)
      } catch (error) {
        if (!isCurrent()) return
        publicError = error instanceof ApiError ? error : new ApiError(0, 'Erro de conexão com o CNPJá.')
      }
    }

    if (!selectedOrg) {
      if (openCnpj) {
        warnings.push('Validação fiscal pela SEFAZ disponível após selecionar uma organização com certificado.')
        setState({...INITIAL_STATE, status: 'found', result: mergeCnpjLookupResults(openCnpj, null, warnings)})
      } else {
        setState({
          ...INITIAL_STATE,
          status: publicError?.status === 404 ? 'not_found' : 'error',
          errorMessage: publicError?.detail ?? 'Selecione uma organização para consultar a SEFAZ.',
        })
      }
      return
    }

    const preferredUf = openCnpj?.uf || orgUf || DEFAULT_UF
    const ufsToTry = bfsUfOrder(preferredUf)
    let sefazResult: LookupOrganizationOut | null = null

    setState((current) => ({...current, phase: CNPJ_LOOKUP_SOURCE.SEFAZ, currentUf: preferredUf}))

    for (const uf of ufsToTry) {
      if (!isCurrent()) return
      setState((current) => ({...current, currentUf: uf}))

      try {
        sefazResult = await apiClient.lookupOrganization(clean, uf)
        break
      } catch (error) {
        if (!isCurrent()) return
        if (!(error instanceof ApiError)) {
          warnings.push('Não foi possível validar os dados na SEFAZ.')
          break
        }

        if (problemType(error) === NO_CERTIFICATE_TYPE) {
          if (openCnpj) {
            warnings.push('Consulta SEFAZ indisponível: a organização ativa não possui certificado digital.')
            break
          }
          setState({...INITIAL_STATE, status: 'no_certificate', errorMessage: error.detail})
          return
        }
        if (problemType(error) === SEFAZ_REJECTION_TYPE) {
          if (openCnpj) {
            warnings.push(error.detail || 'A SEFAZ rejeitou a validação fiscal.')
            break
          }
          setState({...INITIAL_STATE, status: 'sefaz_rejection', errorMessage: error.detail})
          return
        }
        if (error.status === 404) continue
        warnings.push(error.detail || 'Não foi possível validar os dados na SEFAZ.')
        break
      }
    }

    if (!isCurrent()) return
    if (!sefazResult) warnings.push('Cadastro não localizado na consulta da SEFAZ.')
    if (publicError && publicError.status !== 404) warnings.push(publicError.detail)

    const result = mergeCnpjLookupResults(openCnpj, sefazResult, warnings)
    setState(result
      ? {...INITIAL_STATE, status: 'found', result}
      : {...INITIAL_STATE, status: 'not_found', errorMessage: publicError?.detail ?? null})
  }, [selectedOrg])

  const reset = useCallback(() => {
    requestIdRef.current += 1
    setState(INITIAL_STATE)
  }, [])

  return {state, lookup, cancel, reset}
}
