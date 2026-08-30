import {render, screen} from '@testing-library/react'
import {describe, expect, it, vi} from 'vitest'
import {OrganizationsTable} from '@/components/organizations/OrganizationsTable'
import {organizationOutToFormData} from '@/lib/utils/converters'
import type {OrganizationOut} from '@/lib/types/api'

// A company linked from the ctech-account handoff is created with its identity
// and nothing else: the addresses, the CRT and the inscrições are what this
// product asks for next. Both of these read `org.person` and both crashed on
// the first linked company — the list on 'fantasy_name' of undefined, and the
// edit screen the link navigates to right after.
const linkedCompany = {
  pk: '01a05335-0927-750f-b984-f5c4fff31ce2',
  name: 'ACME LTDA',
  description: '',
  tax_id: '11520224000140',
  tax_id_kind: 'cnpj',
  created_at: '2026-08-30T14:51:21Z',
  updated_at: '2026-08-30T14:51:21Z',
} as OrganizationOut

describe('uma empresa vinculada ainda não tem o lado fiscal', () => {
  it('aparece na lista sem quebrar', () => {
    render(<OrganizationsTable organizations={[linkedCompany]} onEdit={vi.fn()} loading={false}/>)

    expect(screen.getByText('ACME LTDA')).toBeInTheDocument()
    expect(screen.getByText('11.520.224/0001-40')).toBeInTheDocument()
  })

  it('abre no formulário vazio, com um endereço em branco para preencher', () => {
    const data = organizationOutToFormData(linkedCompany)

    expect(data.name).toBe('ACME LTDA')
    expect(data.cpf_or_cnpj).toBe('11520224000140')
    expect(data.person.addresses).toHaveLength(1)
    expect(data.person.addresses[0].street).toBe('')
    expect(data.person.state_registrations).toEqual([])
  })
})
