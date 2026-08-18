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
      role: 'OWNER',
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

/**
 * Chave de acesso sintética com a estrutura real de 44 dígitos:
 * cUF(2) AAMM(4) CNPJ(14) mod(2) série(3) nNF(9) tpEmis(1) cNF(8) cDV(1).
 * Telas mostram a chave formatada — um `1` repetido 44 vezes denuncia o mock.
 */
function accessKey(cnpj: string, model: string, serie: number, number: number, code: number) {
  const body = `35260${''}7${cnpj}${model}${String(serie).padStart(3, '0')}${String(number).padStart(9, '0')}1${String(code).padStart(8, '0')}`
  const digit = String(body.split('').reduce((sum, char) => sum + Number(char), 0) % 10)
  return `${body}${digit}`
}

function nfeList(status: string, number: number, total: string, day: number) {
  return {
    pk: `${ORG_PK}#nfe#${number}`,
    sk: accessKey(ORG_PK, '55', 1, number, number * 7),
    incoming: 0,
    year: 2026,
    month: 7,
    day,
    status,
    sefaz_status: status === 'authorized' ? '100' : status === 'rejected' ? '204' : null,
    sefaz_motive: status === 'rejected' ? 'Rejeição: Duplicidade de NF-e [nRec:000000000000000]' : null,
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
  nfeList('authorized', 1001, '3679.70', 1),
  nfeList('authorized', 1002, '89.90', 2),
  nfeList('processing', 1003, '259.90', 3),
  nfeList('rejected', 1004, '1299.00', 5),
  nfeList('cancelled', 1005, '599.00', 8),
]

export const nfcesFixture = [
  {...nfeList('authorized', 2001, '45.00', 1), sk: accessKey(ORG_PK, '65', 1, 2001, 11)},
  {...nfeList('authorized', 2002, '12.50', 2), sk: accessKey(ORG_PK, '65', 1, 2002, 22)},
  {...nfeList('pending', 2003, '88.90', 4), sk: accessKey(ORG_PK, '65', 1, 2003, 33)},
]

function mdfeList(status: string, number: number, day: number, ufEnd: string) {
  return {
    pk: `${ORG_PK}#mdfe#${number}`,
    sk: accessKey(ORG_PK, '58', 1, number, number * 13),
    incoming: 0,
    year: 2026,
    month: 7,
    day,
    status,
    sefaz_status: status === 'authorized' || status === 'closed' ? '100' : null,
    sefaz_motive: null,
    emit_cpf_cnpj: ORG_PK,
    emit_name: 'Empresa Mock LTDA',
    number,
    serie: 1,
    modal: '1',
    doc_type: 'nfe',
    uf_start: 'SP',
    uf_end: ufEnd,
    cargo_weight: '2450.000',
    cargo_value: '3589.80',
    dh_emi: `2026-07-${String(day).padStart(2, '0')}T08:00:00Z`,
    created_at: `2026-07-${String(day).padStart(2, '0')}T08:00:00Z`,
  }
}

export const mdfesFixture = [
  mdfeList('authorized', 1, 1, 'RJ'),
  mdfeList('closed', 2, 4, 'MG'),
  mdfeList('close_pending', 3, 9, 'PR'),
]

export const distributionsFixture = [
  {
    nsu: 100000001,
    doc_schema: 'resNFe',
    schema_type: 'resNFe',
    access_key: accessKey('12345678000199', '55', 1, 4821, 91),
    emit_name: 'Fornecedor Modelo ME',
    emit_cpf_cnpj: '12345678000199',
    dest_name: 'Empresa Mock LTDA',
    total: '1280.00',
    sefaz_status: '100',
    sefaz_motive: null,
    sefaz_protocol: '135260000778899',
    event_type: null,
    dh_emi: '2026-07-01T10:00:00Z',
    parse_error: false,
    xml_s3_key: `xml/${ORG_PK}/dist/100000001.xml`,
    created_at: '2026-07-01T10:05:00Z',
  },
  {
    nsu: 100000002,
    doc_schema: 'procEventoNFe',
    schema_type: 'procEventoNFe',
    access_key: accessKey('12345678000199', '55', 1, 4790, 44),
    emit_name: 'Fornecedor Modelo ME',
    emit_cpf_cnpj: '12345678000199',
    dest_name: 'Empresa Mock LTDA',
    total: null,
    sefaz_status: '135',
    sefaz_motive: null,
    sefaz_protocol: '135260000778123',
    event_type: '110111',
    dh_emi: '2026-07-02T11:00:00Z',
    parse_error: false,
    xml_s3_key: `xml/${ORG_PK}/dist/100000002.xml`,
    created_at: '2026-07-02T11:04:00Z',
  },
]

export const nfeConfigFixture = {
  pk: `${ORG_PK}#nfe-config`,
  timezone: 'America/Sao_Paulo',
  environment: 2,
  prod_current_number: 1,
  prod_current_serie: 1,
  hom_current_number: 1006,
  hom_current_serie: 1,
  prod_nsu: 0,
  prod_last_dist_nsu_at: null,
  hom_nsu: 100000002,
  hom_last_dist_nsu_at: '2026-07-02T11:05:00Z',
  updated_at: '2026-07-02T11:05:00Z',
}

export const nfceConfigFixture = {
  pk: `${ORG_PK}#nfce-config`,
  timezone: 'America/Sao_Paulo',
  environment: 2,
  prod_current_number: 1,
  prod_current_serie: 1,
  prod_csc: '',
  prod_csc_id: 0,
  hom_current_number: 2004,
  hom_current_serie: 1,
  hom_csc: 'MOCK-CSC-HOMOLOGACAO',
  hom_csc_id: 1,
  updated_at: '2026-07-02T09:00:00Z',
}

export const cteConfigFixture = {
  ...nfeConfigFixture,
  pk: `${ORG_PK}#cte-config`,
  hom_current_number: 10,
  hom_nsu: 100000002,
}

export const mdfeConfigFixture = {
  ...nfeConfigFixture,
  pk: `${ORG_PK}#mdfe-config`,
  hom_current_number: 2,
  hom_nsu: 0,
  hom_last_dist_nsu_at: null,
}

export const nfseConfigFixture = {
  pk: `${ORG_PK}#nfse-config`,
  provider: 'nacional' as const,
  environment: 2,
  timezone: 'America/Sao_Paulo',
  c_loc_emi: '3550308',
  serie: '1',
  prod_current_number: 1,
  hom_current_number: 12,
  certificate_sk: 'CERT#mock-a1',
  abrasf: null,
  prod_nsu: 0,
  prod_last_dist_nsu_at: null,
  hom_nsu: 42,
  hom_last_dist_nsu_at: '2026-07-02T08:30:00Z',
  updated_at: '2026-07-02T08:30:00Z',
}

export const certificatesFixture = [
  {
    pk: ORG_PK,
    sk: 'CERT#mock-a1',
    alias: 'EMPRESA MOCK LTDA:11222333000181',
    md5: 'd41d8cd98f00b204e9800998ecf8427e',
    s3_key: `certificates/${ORG_PK}/mock-a1.pfx`,
    expires_at: '2027-01-20T23:59:59Z',
    created_at: '2026-01-20T10:00:00Z',
  },
]

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
    pk: ORG_PK,
    sk: 'AUDIT#2026-07-01T14:31:00Z#1',
    resource_type: 'nfe',
    resource_id: '1001',
    action: 'CREATE' as const,
    modifications: [],
    user_id: 'mock-user-1',
    user_name: 'Mock User',
    created_at: '2026-07-01T14:31:00Z',
  },
  {
    pk: ORG_PK,
    sk: 'AUDIT#2026-06-28T09:12:00Z#1',
    resource_type: 'product',
    resource_id: 'PRODUCT#1',
    action: 'UPDATE' as const,
    modifications: [
      {name: 'value', before: '3299.90', after: '3499.90'},
      {name: 'value_resale', before: '3799.90', after: '3999.90'},
    ],
    user_id: 'mock-user-1',
    user_name: 'Mock User',
    created_at: '2026-06-28T09:12:00Z',
  },
  {
    pk: ORG_PK,
    sk: 'AUDIT#2026-06-20T16:04:00Z#1',
    resource_type: 'person',
    resource_id: 'PERSON#2',
    action: 'DELETE' as const,
    modifications: [],
    user_id: 'mock-user-1',
    user_name: 'Mock User',
    created_at: '2026-06-20T16:04:00Z',
  },
]

export const ORG_PK_VALUE = ORG_PK

/**
 * Billing fixtures.
 *
 * Half of the states below cannot be produced against a real backend in any
 * reasonable time — a past-due invoice needs a due date to pass, an incomplete
 * checkout needs a payment that never lands. They are scenarios rather than one
 * fixture for exactly that reason; the dev panel switches between them.
 */

export const billingPlansFixture = {
  billing_enabled: true,
  data: [
    {
      id: 'prod_dfe_free',
      name: 'DF-e Free',
      active: true,
      prices: [
        {
          id: 'price_dfe_free_monthly',
          product_id: 'prod_dfe_free',
          type: 'fixed' as const,
          unit_amount: 0,
          billing_timing: 'advance' as const,
          archived: false,
          metadata: {plan: 'free', quota_nfe: '3', quota_nfce: '3', quota_companies: '1', quota_users: '1'},
        },
      ],
    },
    {
      id: 'prod_dfe_ondemand',
      name: 'DF-e Sob Demanda',
      active: true,
      prices: [
        {
          id: 'price_dfe_ondemand_nfe',
          product_id: 'prod_dfe_ondemand',
          type: 'metered' as const,
          unit_amount: 39,
          billing_timing: 'arrears' as const,
          archived: false,
          metadata: {plan: 'ondemand', meter: 'nfe', quota_nfe: '-1'},
        },
        {
          id: 'price_dfe_ondemand_nfce',
          product_id: 'prod_dfe_ondemand',
          type: 'metered' as const,
          unit_amount: 19,
          billing_timing: 'arrears' as const,
          archived: false,
          metadata: {plan: 'ondemand', meter: 'nfce', quota_nfce: '-1'},
        },
      ],
    },
    {
      id: 'prod_dfe_pro',
      name: 'DF-e Pro',
      active: true,
      prices: [
        {
          id: 'price_dfe_pro_monthly',
          product_id: 'prod_dfe_pro',
          type: 'fixed' as const,
          unit_amount: 14900,
          billing_timing: 'advance' as const,
          archived: false,
          metadata: {
            plan: 'pro',
            quota_nfe: '1000', quota_nfce: '1000', quota_cte: '500',
            quota_mdfe: '500', quota_nfse: '500', quota_companies: '3', quota_users: '10',
          },
        },
      ],
    },
  ],
}

const PRO_QUOTAS = {nfe: 1000, nfce: 1000, cte: 500, mdfe: 500, nfse: 500, companies: 3, users: 10}
const FREE_QUOTAS = {nfe: 3, nfce: 3, companies: 1, users: 1}

export const billingSubscriptionFixtures = {
  /** Fresh account: the plan layer of onboarding has not been answered. */
  none: {
    has_subscription: false,
    status: '',
    plan: '',
    grants_service: false,
    cancel_at_period_end: false,
    period_start: '',
    period_end: '',
    quotas: {},
    no_charge: false,
  },
  /** Free plan with every document already spent — the 402 the upgrade screen answers. */
  free_at_limit: {
    has_subscription: true,
    status: 'ACTIVE',
    plan: 'free',
    grants_service: true,
    cancel_at_period_end: false,
    period_start: '2026-08-01',
    period_end: '2026-09-01',
    quotas: FREE_QUOTAS,
    no_charge: false,
    usage: {nfe: {used: 3, limit: 3}, nfce: {used: 1, limit: 3}, companies: {used: 1, limit: 1}, users: {used: 1, limit: 1}},
  },
  /** Pro, paid, healthy. */
  pro_active: {
    has_subscription: true,
    status: 'ACTIVE',
    plan: 'pro',
    grants_service: true,
    cancel_at_period_end: false,
    period_start: '2026-08-01',
    period_end: '2026-09-01',
    quotas: PRO_QUOTAS,
    no_charge: false,
    usage: {nfe: {used: 812, limit: 1000}, nfce: {used: 40, limit: 1000}, cte: {used: 0, limit: 500}, mdfe: {used: 0, limit: 500}, nfse: {used: 0, limit: 500}, companies: {used: 2, limit: 3}, users: {used: 4, limit: 10}},
  },
  /** Pro with a bill nobody paid: issuance blocked, banner with the amount. */
  pro_past_due: {
    has_subscription: true,
    status: 'PAST_DUE',
    plan: 'pro',
    grants_service: false,
    cancel_at_period_end: false,
    period_start: '2026-08-01',
    period_end: '2026-09-01',
    quotas: PRO_QUOTAS,
    no_charge: false,
    usage: {nfe: {used: 120, limit: 1000}, companies: {used: 1, limit: 3}, users: {used: 2, limit: 10}},
    open_invoice: {
      id: 'inv_mock_overdue',
      total_cents: 14900,
      due_date: '2026-08-05',
      checkout_url: 'https://billing.example.test/pay/inv_mock_overdue',
    },
  },
  /** On-demand, metered, nothing capped. */
  ondemand: {
    has_subscription: true,
    status: 'ACTIVE',
    plan: 'ondemand',
    grants_service: true,
    cancel_at_period_end: false,
    period_start: '2026-08-01',
    period_end: '2026-09-01',
    quotas: {nfe: -1, nfce: -1, companies: -1, users: -1},
    no_charge: false,
    usage: {nfe: {used: 214, limit: -1}, nfce: {used: 57, limit: -1}, companies: {used: 1, limit: -1}, users: {used: 1, limit: -1}},
  },
  /** Chose the paid plan and never paid — what `/onboarding/retorno` polls on. */
  checkout_pending: {
    has_subscription: true,
    status: 'INCOMPLETE',
    plan: 'pro',
    grants_service: false,
    cancel_at_period_end: false,
    period_start: '2026-08-16',
    period_end: '2026-09-16',
    quotas: PRO_QUOTAS,
    no_charge: false,
    open_invoice: {
      id: 'inv_mock_first',
      total_cents: 14900,
      due_date: '2026-08-21',
      checkout_url: 'https://billing.example.test/pay/inv_mock_first',
    },
  },
}

export type BillingScenario = keyof typeof billingSubscriptionFixtures

export const billingInvoicesFixture = [
  {
    id: 'inv_mock_1',
    number: 1042,
    status: 'PAID' as const,
    overdue: false,
    due_date: '2026-07-05',
    total: 14900,
    amount_due: 0,
  },
  {
    id: 'inv_mock_2',
    number: 1043,
    status: 'OPEN' as const,
    overdue: true,
    due_date: '2026-08-05',
    total: 14900,
    amount_due: 14900,
    checkout_url: 'https://billing.example.test/pay/inv_mock_overdue',
  },
]

/**
 * Detalhes de documento. As listas trazem só a projeção de listagem; o detalhe
 * acrescenta itens, pagamentos e protocolo — sem eles a tela de detalhe (e as
 * capturas do guia) sai vazia.
 */
export const nfeDetailFixture = {
  ...nfesFixture[0],
  products: [
    {
      product_id: 'PRODUCT#1',
      product_code: 'P001',
      description: 'Notebook 15 polegadas',
      ncm: '84713012',
      cfop: '5102',
      unit: 'UN',
      quantity: '1.0000',
      unit_value: '3499.90',
      discount: '0.00',
      total: '3499.90',
    },
    {
      product_id: 'PRODUCT#2',
      product_code: 'P002',
      description: 'Mouse óptico sem fio',
      ncm: '84716052',
      cfop: '5102',
      unit: 'UN',
      quantity: '2.0000',
      unit_value: '89.90',
      discount: '0.00',
      total: '179.80',
    },
  ],
  payments: [{payment_type: '03', value: '3679.70'}],
  additional_info: 'Documento emitido em ambiente de homologação — sem valor fiscal.',
  xml_s3_key: `xml/${ORG_PK}/nfe/1001.xml`,
  sefaz_protocol: '135260000123456',
}

export const nfceDetailFixture = {
  ...nfcesFixture[0],
  products: [
    {
      product_id: 'PRODUCT#2',
      product_code: 'P002',
      description: 'Mouse óptico sem fio',
      ncm: '84716052',
      cfop: '5102',
      unit: 'UN',
      quantity: '1.0000',
      unit_value: '45.00',
      discount: '0.00',
      total: '45.00',
    },
  ],
  payments: [{payment_type: '01', value: '45.00'}],
  additional_info: null,
  xml_s3_key: `xml/${ORG_PK}/nfce/2001.xml`,
  sefaz_protocol: '135260000987654',
}

export const mdfeDetailFixture = {
  ...mdfesFixture[0],
  documents: [
    {type: 'nfe' as const, access_key: nfesFixture[0].sk},
    {type: 'nfe' as const, access_key: nfesFixture[1].sk},
  ],
  route: ['SP', 'RJ'],
  loadings: [{ibge_code: '3550308', city: 'São Paulo'}],
  unloadings: [
    {ibge_code: '3304557', city: 'Rio de Janeiro', access_keys: [nfesFixture[0].sk]},
  ],
  predominant: {tp_carga: '05', x_prod: 'Equipamentos de informática', ncm: '84713012'},
  vehicle: {placa: 'ABC1D23', uf: 'SP', tara: '8000', rntrc: '12345678'},
  drivers: [{name: 'João Motorista', cpf: '12345678909'}],
  trip_start: '2026-07-01T09:00:00Z',
  bulk_cargo: null,
  xml_s3_key: `xml/${ORG_PK}/mdfe/1.xml`,
  sefaz_protocol: '135260000555111',
}

/**
 * Eventos SEFAZ. O worker grava a emissão como evento também, então a timeline
 * de qualquer documento começa em `emission` — replicar isso é o que faz a tela
 * de detalhe parecer com produção.
 */
function dfeEvent(accessKey: string, eventType: string, seq: number, status: string, at: string) {
  return {
    pk: ORG_PK,
    sk: `${at}_${eventType}_${seq}`,
    access_key: accessKey,
    event_type: eventType,
    sequence_number: seq,
    status,
    sefaz_status: status === 'success' || status === 'authorized' ? '135' : null,
    sefaz_motive: null,
    sefaz_protocol: `1352600001${seq}${eventType}`.slice(0, 15),
    xml_s3_key: `xml/${ORG_PK}/events/${eventType}-${seq}.xml`,
    created_at: at,
    updated_at: at,
  }
}

export const nfeEventsFixture = [
  dfeEvent(nfesFixture[0].sk, 'emission', 1, 'authorized', '2026-07-01T14:30:00Z'),
  dfeEvent(nfesFixture[0].sk, '110110', 1, 'success', '2026-07-03T10:12:00Z'),
]

export const nfceEventsFixture = [
  dfeEvent(nfcesFixture[0].sk, 'emission', 1, 'authorized', '2026-07-01T14:30:00Z'),
]

export const mdfeEventsFixture = [
  dfeEvent(mdfesFixture[0].sk, 'emission', 1, 'authorized', '2026-07-01T08:00:00Z'),
  dfeEvent(mdfesFixture[0].sk, '110112', 1, 'success', '2026-07-02T18:40:00Z'),
]

// Catálogo de serviços (NFS-e)
export const servicesFixture = [
  {
    pk: `${ORG_PK}#service#1`,
    sk: 'SERVICE#1',
    code: 'S001',
    description: 'Desenvolvimento de software sob encomenda',
    trib_nacional_code: '010701',
    trib_municipal_code: null,
    nbs_code: null,
    cnae: '6201501',
    unit: 'UN',
    value: '4500.00',
    iss: {trib_issqn: 1, tax_rate: '2.00', tp_ret_issqn: 1, tp_imunidade: null, c_pais_resultado: null},
    federal: null,
    ibs_cbs: {c_ind_op: '1', cst: '000', c_class_trib: '000001', ind_dest: 1, tp_oper: null, fin_nfse: 0 as const},
    tot_trib: {ind_tot_trib: 0, p_tot_trib_sn: null},
    created_at: '2026-02-01T09:00:00Z',
    updated_at: '2026-06-01T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#service#2`,
    sk: 'SERVICE#2',
    code: 'S002',
    description: 'Suporte técnico mensal',
    trib_nacional_code: '010702',
    trib_municipal_code: null,
    nbs_code: null,
    cnae: '6209100',
    unit: 'MES',
    value: '890.00',
    iss: {trib_issqn: 1, tax_rate: '3.00', tp_ret_issqn: 1, tp_imunidade: null, c_pais_resultado: null},
    federal: null,
    ibs_cbs: {c_ind_op: '1', cst: '000', c_class_trib: '000001', ind_dest: 1, tp_oper: null, fin_nfse: 0 as const},
    tot_trib: {ind_tot_trib: 0, p_tot_trib_sn: null},
    created_at: '2026-02-10T09:00:00Z',
    updated_at: '2026-06-10T09:00:00Z',
  },
]

// Cadastros reutilizáveis
export const taxProfilesFixture = [
  {
    pk: `${ORG_PK}#tax-profile#1`,
    sk: 'TAXPROFILE#1',
    name: 'Venda dentro do estado — Simples Nacional',
    description: 'CSOSN 102, sem crédito de ICMS',
    cfops: ['5102'],
    created_at: '2026-01-10T09:00:00Z',
    updated_at: '2026-05-02T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#tax-profile#2`,
    sk: 'TAXPROFILE#2',
    name: 'Venda interestadual — consumidor final',
    description: 'Partilha DIFAL destino',
    cfops: ['6108'],
    created_at: '2026-01-12T09:00:00Z',
    updated_at: '2026-05-02T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#tax-profile#3`,
    sk: 'TAXPROFILE#3',
    name: 'Devolução de venda',
    description: null,
    cfops: ['1202', '2202'],
    created_at: '2026-03-01T09:00:00Z',
    updated_at: '2026-05-02T09:00:00Z',
  },
]

export const operationsFixture = [
  {
    pk: `${ORG_PK}#operation#1`,
    sk: 'OPERATION#1',
    name: 'Venda de mercadoria',
    doc_types: ['nfe', 'nfce'],
    nat_op: 'Venda de mercadoria adquirida de terceiros',
    cfop_suffix: '102',
    is_default: true,
    created_at: '2026-01-10T09:00:00Z',
    updated_at: '2026-04-20T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#operation#2`,
    sk: 'OPERATION#2',
    name: 'Remessa para conserto',
    doc_types: ['nfe'],
    nat_op: 'Remessa para conserto ou reparo',
    cfop_suffix: '915',
    is_default: false,
    created_at: '2026-02-14T09:00:00Z',
    updated_at: '2026-04-20T09:00:00Z',
  },
]

export const paymentTermsFixture = [
  {
    pk: `${ORG_PK}#payment-term#1`,
    sk: 'PAYMENTTERM#1',
    name: 'À vista — Pix',
    payment_type: '17',
    ind_pag: '0',
    installments: 1,
    interval_days: 0,
    first_due_days: 0,
    created_at: '2026-01-10T09:00:00Z',
    updated_at: '2026-04-01T09:00:00Z',
  },
  {
    pk: `${ORG_PK}#payment-term#2`,
    sk: 'PAYMENTTERM#2',
    name: '3x sem juros — cartão',
    payment_type: '03',
    ind_pag: '1',
    installments: 3,
    interval_days: 30,
    first_due_days: 30,
    created_at: '2026-01-11T09:00:00Z',
    updated_at: '2026-04-01T09:00:00Z',
  },
]

export const vehicleSetsFixture = [
  {
    pk: `${ORG_PK}#vehicle-set#1`,
    sk: 'VEHICLESET#1',
    name: 'Cavalo + carreta — rota Sudeste',
    tractor_sk: 'VEHICLE#1',
    trailer_sks: ['VEHICLE#2'],
    driver_docs: ['12345678909'],
    rntrc: '12345678',
    ciot: null,
    created_at: '2026-03-05T09:00:00Z',
    updated_at: '2026-06-05T09:00:00Z',
  },
]

// NFS-e
function nfseList(status: string, number: number, day: number, total: string) {
  const id = `${ORG_PK}${String(number).padStart(15, '0')}`.padStart(45, '0').slice(-45)
  return {
    pk: `hom#${ORG_PK}`,
    sk: id,
    provider: 'nacional' as const,
    status,
    tp_emit: 1,
    serie: '1',
    number,
    competence: '2026-07',
    dh_emi: `2026-07-${String(day).padStart(2, '0')}T11:00:00Z`,
    c_loc_emi: '3550308',
    year: 2026,
    month: 7,
    emit_cpf_cnpj: ORG_PK,
    emit_name: 'Empresa Mock LTDA',
    dest_cpf_cnpj: '12345678000199',
    dest_name: 'Cliente Exemplo LTDA',
    total,
    payload: {},
    access_key: status === 'authorized' ? `${accessKey(ORG_PK, '99', 1, number, number * 3)}000000` : null,
    xml_s3_key: status === 'authorized' ? `xml/${ORG_PK}/nfse/${number}.xml` : null,
    dps_xml_s3_key: `xml/${ORG_PK}/dps/${number}.xml`,
    sefaz_motive: null,
    c_motivo_emis_ti: null,
    user_id: 'mock-user-1',
    user_name: 'Mock User',
    created_at: `2026-07-${String(day).padStart(2, '0')}T11:00:00Z`,
    updated_at: `2026-07-${String(day).padStart(2, '0')}T11:02:00Z`,
  }
}

export const nfsesFixture = [
  nfseList('authorized', 11, 2, '4500.00'),
  nfseList('authorized', 12, 6, '890.00'),
  nfseList('processing', 13, 10, '1200.00'),
]

export const nfseEventsFixture = [
  {
    ...dfeEvent(nfsesFixture[0].sk, 'emission', 1, 'authorized', '2026-07-02T11:00:00Z'),
    pk: nfsesFixture[0].sk,
  },
]

/**
 * Distribuição NFS-e. Diferente de NF-e/CT-e/MDF-e, o worker não faz parsing do
 * XML aqui — só NSU, chave e schema (ver `NfseDistributionOut`).
 */
export const nfseDistributionsFixture = [
  {
    nsu: 42,
    doc_type: 'nfse',
    schema_type: 'NFSe',
    access_key: `${accessKey('12345678000199', '99', 1, 77, 5)}000000`,
    event_type: null,
    xml_s3_key: `xml/${ORG_PK}/dist-nfse/42.xml`,
    created_at: '2026-07-02T08:30:00Z',
  },
]
