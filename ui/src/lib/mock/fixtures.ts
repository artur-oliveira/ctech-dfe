/**
 * Fixture data for the dev mock API. Shapes mirror the backend contracts in
 * `lib/types/api.ts` so authenticated pages render as they would against a
 * real server. All values are synthetic.
 */

const ORG_PK = '11222333000181'

export const meFixture = {
  user_id: 'mock-user-1',
  username: 'mock@ctech.dev',
  email: 'mock@ctech.dev',
  first_name: 'Mock',
  last_name: 'User',
  email_verified: true,
  is_enabled: true,
  last_login_at: null,
  organizations: [
    {
      pk: ORG_PK,
      name: 'Empresa Mock LTDA',
      description: 'Organização de demonstração',
      role: 'owner',
      permissions: ['*'],
      state_federation: 'SP',
    },
  ],
  terms_addendum_accepted: true,
}

export const rolesFixture = [
  {name: 'owner', description: 'Controle total da organização'},
  {name: 'admin', description: 'Gestão operacional'},
  {name: 'viewer', description: 'Leitura apenas'},
]

export const organizationsFixture = [
  {
    pk: ORG_PK,
    name: 'Empresa Mock LTDA',
    description: 'Organização de demonstração',
    person: {
      fantasy_name: 'Empresa Mock LTDA',
      crt: 1,
      state_registrations: [{uf: 'SP', state_registration: '123456789012'}],
      addresses: [
        {
          city_ibge_code: '3550308',
          street: 'Rua Exemplo',
          neighborhood: 'Centro',
          number: '100',
          city: 'São Paulo',
          state_federation: 'SP',
          postal_code: '01001-000',
          complement: 'Sala 1',
        },
      ],
      contacts: {emails: ['contato@empresamock.dev'], phones: ['(11) 99999-0000']},
    },
    created_at: '2025-01-15T10:00:00Z',
    updated_at: '2025-06-01T12:00:00Z',
  },
]

export const productsFixture = [
  {
    pk: `${ORG_PK}#product#1`,
    sk: 'PRODUCT#1',
    code: 'P001',
    description: 'Notebook 15 polegadas',
    brand: 'ACME',
    ncm: '84713012',
    cest: null,
    origin: '0',
    unit: 'UN',
    taxable_unit: 'UN',
    cean: '7890000000001',
    taxable_cean: '7890000000001',
    value: '3499.90',
    value_resale: '3999.90',
    net_weight: '2.10',
    gross_weight: '2.60',
    cfop_nfce: '5102',
    cfop_config: [],
    conversion_factors: [],
  },
  {
    pk: `${ORG_PK}#product#2`,
    sk: 'PRODUCT#2',
    code: 'P002',
    description: 'Mouse óptico sem fio',
    brand: 'ACME',
    ncm: '84716052',
    cest: null,
    origin: '0',
    unit: 'UN',
    taxable_unit: 'UN',
    cean: '7890000000002',
    taxable_cean: '7890000000002',
    value: '89.90',
    value_resale: '119.90',
    net_weight: '0.12',
    gross_weight: '0.20',
    cfop_nfce: '5102',
    cfop_config: [],
    conversion_factors: [],
  },
  {
    pk: `${ORG_PK}#product#3`,
    sk: 'PRODUCT#3',
    code: 'P003',
    description: 'Teclado mecânico',
    brand: 'ACME',
    ncm: '84716052',
    cest: null,
    origin: '0',
    unit: 'UN',
    taxable_unit: 'UN',
    cean: '7890000000003',
    taxable_cean: '7890000000003',
    value: '259.90',
    value_resale: '299.90',
    net_weight: '0.80',
    gross_weight: '1.10',
    cfop_nfce: '5102',
    cfop_config: [],
    conversion_factors: [],
  },
]

export const vehiclesFixture = [
  {
    pk: `${ORG_PK}#vehicle#1`,
    sk: 'VEHICLE#1',
    plate: 'ABC1D23',
    plate_uf: 'SP',
    role: 'tractor' as const,
    wheelset: 'RR',
    bodywork: 'TR',
    renavam: '12345678901',
    weight: 8000,
    cap_kg: 23000,
    cap_m3: 90,
    cint: '1234567',
    created_at: '2025-02-10T09:00:00Z',
    updated_at: '2025-05-20T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#vehicle#2`,
    sk: 'VEHICLE#2',
    plate: 'XYZ9K87',
    plate_uf: 'SP',
    role: 'trailer' as const,
    wheelset: 'SR',
    bodywork: 'RB',
    renavam: '10987654321',
    weight: 5200,
    cap_kg: 30000,
    cap_m3: 110,
    cint: '7654321',
    created_at: '2025-03-12T09:00:00Z',
    updated_at: '2025-06-01T09:00:00Z',
  },
]

export const personsFixture = [
  {
    pk: `${ORG_PK}#person#1`,
    sk: 'PERSON#1',
    name: 'Cliente Exemplo LTDA',
    person: {
      fantasy_name: 'Cliente Exemplo',
      crt: 1,
      state_registrations: [{uf: 'SP', state_registration: '987654321098'}],
      addresses: [
        {
          city_ibge_code: '3550308',
          street: 'Av. Cliente',
          neighborhood: 'Vila Nova',
          number: '500',
          city: 'São Paulo',
          state_federation: 'SP',
          postal_code: '02020-000',
          complement: '',
        },
      ],
      contacts: {emails: ['cliente@exemplo.dev'], phones: ['(11) 98888-1111']},
    },
    created_at: '2025-01-20T09:00:00Z',
    updated_at: '2025-04-15T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#person#2`,
    sk: 'PERSON#2',
    name: 'Fornecedor Modelo ME',
    person: {
      fantasy_name: 'Fornecedor Modelo',
      crt: 1,
      state_registrations: [{uf: 'RJ', state_registration: '112233445566'}],
      addresses: [
        {
          city_ibge_code: '3304557',
          street: 'Rua Fornecedor',
          neighborhood: 'Centro',
          number: '200',
          city: 'Rio de Janeiro',
          state_federation: 'RJ',
          postal_code: '20040-000',
          complement: '',
        },
      ],
      contacts: {emails: ['contato@fornecedor.dev'], phones: ['(21) 97777-2222']},
    },
    created_at: '2025-02-05T09:00:00Z',
    updated_at: '2025-05-30T09:00:00Z',
  },
]

function nfeList(status: string, number: number, total: string, day: number) {
  return {
    pk: `${ORG_PK}#nfe#${number}`,
    sk: ''.padStart(44, String(number)).slice(-44),
    incoming: 0,
    year: 2026,
    month: 7,
    day,
    status,
    sefaz_status: status === 'Autorizada' ? '100' : null,
    sefaz_motive: status === 'Rejeitada' ? 'Rejeição: Duplicidade de NF-e' : null,
    emit_cpf_cnpj: ORG_PK,
    emit_name: 'Empresa Mock LTDA',
    dest_cpf_cnpj: '12345678000199',
    dest_name: 'Cliente Exemplo LTDA',
    number,
    serie: 1,
    total,
    dh_emi: `2026-07-${String(day).padStart(2, '0')}T14:30:00Z`,
    created_at: `2026-07-${String(day).padStart(2, '0')}T14:30:00Z`,
  }
}

export const nfesFixture = [
  nfeList('Autorizada', 1001, '3499.90', 1),
  nfeList('Autorizada', 1002, '89.90', 2),
  nfeList('Pendente', 1003, '259.90', 3),
  nfeList('Rejeitada', 1004, '1299.00', 5),
  nfeList('Cancelada', 1005, '599.00', 8),
]

export const nfcesFixture = [
  {...nfeList('Autorizada', 2001, '45.00', 1), sk: ''.padStart(44, '2').slice(-44)},
  {...nfeList('Autorizada', 2002, '12.50', 2), sk: ''.padStart(44, '3').slice(-44)},
  {...nfeList('Pendente', 2003, '88.90', 4), sk: ''.padStart(44, '4').slice(-44)},
]

export const mdfesFixture = [
  {
    pk: `${ORG_PK}#mdfe#1`,
    sk: ''.padStart(44, '5').slice(-44),
    incoming: 0,
    year: 2026,
    month: 7,
    day: 1,
    status: 'Autorizada',
    sefaz_status: '100',
    sefaz_motive: null,
    emit_cpf_cnpj: ORG_PK,
    emit_name: 'Empresa Mock LTDA',
    dest_cpf_cnpj: '',
    dest_name: '',
    number: 1,
    serie: 1,
    total: '0.00',
    dh_emi: '2026-07-01T08:00:00Z',
    created_at: '2026-07-01T08:00:00Z',
    vehicles: ['ABC1D23', 'XYZ9K87'],
    route: 'São Paulo → Rio de Janeiro',
  },
]

export const distributionsFixture = [
  {
    nsu: 100000001,
    schema: 'resNFe',
    status: 'Autorizada',
    emit_cpf_cnpj: '12345678000199',
    emit_name: 'Cliente Exemplo LTDA',
    dest_cpf_cnpj: ORG_PK,
    dest_name: 'Empresa Mock LTDA',
    dh_emi: '2026-07-01T10:00:00Z',
    total: '3499.90',
    access_key: ''.padStart(44, '9').slice(-44),
  },
  {
    nsu: 100000002,
    schema: 'procEventoNFe',
    status: 'Cancelada',
    emit_cpf_cnpj: ORG_PK,
    emit_name: 'Empresa Mock LTDA',
    dest_cpf_cnpj: '12345678000199',
    dest_name: 'Cliente Exemplo LTDA',
    dh_emi: '2026-07-02T11:00:00Z',
    total: '0.00',
    access_key: ''.padStart(44, '8').slice(-44),
  },
]

export const nfeConfigFixture = {
  pk: `${ORG_PK}#nfe-config`,
  org_pk: ORG_PK,
  serie: 1,
  next_number: 1006,
  environment: 2,
  default_cfop: '5102',
  default_nature_operation: 'Venda',
  finalized: true,
}

export const nfceConfigFixture = {
  ...nfeConfigFixture,
  pk: `${ORG_PK}#nfce-config`,
  next_number: 2004,
}

export const cteConfigFixture = {
  pk: `${ORG_PK}#cte-config`,
  org_pk: ORG_PK,
  serie: 1,
  next_number: 10,
  environment: 2,
  default_cfop: '5351',
  finalized: true,
}

export const mdfeConfigFixture = {
  pk: `${ORG_PK}#mdfe-config`,
  org_pk: ORG_PK,
  serie: 1,
  next_number: 2,
  environment: 2,
  finalized: true,
}

export const certificatesFixture: unknown[] = []

export const membersFixture = [
  {
    user_id: 'mock-user-1',
    name: 'Mock User',
    role: 'owner',
    permissions: ['*'],
    invited_by: 'system',
    created_at: '2025-01-15T10:00:00Z',
  },
]

export const auditLogsFixture = [
  {
    pk: `${ORG_PK}#audit#1`,
    sk: 'AUDIT#1',
    actor: 'mock@ctech.dev',
    action: 'emit_nfe',
    resource_type: 'nfe',
    resource_id: '1001',
    detail: 'NF-e 1001 autorizada',
    created_at: '2026-07-01T14:31:00Z',
  },
  {
    pk: `${ORG_PK}#audit#2`,
    sk: 'AUDIT#2',
    actor: 'mock@ctech.dev',
    action: 'upload_certificate',
    resource_type: 'certificate',
    resource_id: 'cert-1',
    detail: 'Certificado A1 enviado',
    created_at: '2026-07-01T09:00:00Z',
  },
]

export const ORG_PK_VALUE = ORG_PK
