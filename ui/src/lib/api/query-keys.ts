export const queryKeys = {
  me: ['auth', 'me'] as const,
  roles: ['auth', 'roles'] as const,
  organizations: {
    all: (cursor?: string) => ['organizations', cursor] as const,
    detail: (pk: string) => ['organizations', pk] as const,
  },
  nfeConfig: (pk: string) => ['nfe-config', pk] as const,
  nfceConfig: (pk: string) => ['nfce-config', pk] as const,
  cteConfig: (pk: string) => ['cte-config', pk] as const,
  mdfeConfig: (pk: string) => ['mdfe-config', pk] as const,
  nfseConfig: (pk: string) => ['nfse-config', pk] as const,
  certificates: (pk: string) => ['certificates', pk] as const,
  products: {
    list: (orgPk: string | undefined) => ['products', orgPk] as const,
    /** "Is there at least one?" — a separate key so the onboarding probe never
     *  serves its 1-item page to the catalogue screen, or the other way round. */
    probe: (orgPk: string | undefined) => ['products', orgPk, 'probe'] as const,
    detail: (id: string) => ['product', id] as const,
  },
  services: {
    list: (orgPk: string | undefined) => ['services', orgPk] as const,
    probe: (orgPk: string | undefined) => ['services', orgPk, 'probe'] as const,
    page: (orgPk: string | undefined) => ['services', orgPk, 'page'] as const,
    detail: (id: string) => ['service', id] as const,
  },
  taxProfiles: {
    list: (orgPk: string | undefined) => ['tax-profiles', orgPk] as const,
    detail: (id: string) => ['tax-profile', id] as const,
  },
  operations: {
    list: (orgPk: string | undefined) => ['operations', orgPk] as const,
    detail: (id: string) => ['operation', id] as const,
  },
  paymentTerms: {
    list: (orgPk: string | undefined) => ['payment-terms', orgPk] as const,
    detail: (id: string) => ['payment-term', id] as const,
  },
  paymentTerminals: {
    list: (orgPk: string | undefined) => ['payment-terminals', orgPk] as const,
    detail: (id: string) => ['payment-terminal', id] as const,
  },
  tollProviders: {
    list: (orgPk: string | undefined) => ['toll-providers', orgPk] as const,
    detail: (id: string) => ['toll-provider', id] as const,
  },
  cargoUnits: {
    list: (orgPk: string | undefined) => ['cargo-units', orgPk] as const,
    detail: (id: string) => ['cargo-unit', id] as const,
  },
  importDeclarations: {
    list: (orgPk: string | undefined) => ['import-declarations', orgPk] as const,
    detail: (id: string) => ['import-declaration', id] as const,
  },
  insurancePolicies: {
    list: (orgPk: string | undefined) => ['insurance-policies', orgPk] as const,
    detail: (id: string) => ['insurance-policy', id] as const,
  },
  vehicleSets: {
    list: (orgPk: string | undefined) => ['vehicle-sets', orgPk] as const,
    detail: (id: string) => ['vehicle-set', id] as const,
  },
  vehicles: {
    list: (orgPk: string | undefined, role?: string) => ['vehicles', orgPk, role] as const,
    detail: (id: string) => ['vehicle', id] as const,
    requirements: (id: string, docType: string, role: string) => ['vehicle-requirements', id, docType, role] as const,
  },
  persons: {
    // role separa o cache da listagem geral do cache de cada papel: são
    // respostas diferentes da mesma rota.
    list: (orgPk: string | undefined, role?: string) =>
      role ? (['persons', orgPk, role] as const) : (['persons', orgPk] as const),
    detail: (cpfCnpj: string) => ['person', cpfCnpj] as const,
    search: (query: string) => ['persons-search', query] as const,
  },
  nfes: {
    // lists() is the partial-match prefix for ALL paginated list queries of this org.
    lists: (orgPk: string | undefined) => ['nfes', orgPk] as const,
    list: (orgPk: string | undefined, params?: object) => ['nfes', orgPk, params] as const,
    detail: (accessKey: string) => ['nfe', accessKey] as const,
    events: (accessKey: string) => ['nfe-events', accessKey] as const,
  },
  nfces: {
    lists: (orgPk: string | undefined) => ['nfces', orgPk] as const,
    list: (orgPk: string | undefined, params?: object) => ['nfces', orgPk, params] as const,
    detail: (accessKey: string) => ['nfce', accessKey] as const,
    events: (accessKey: string) => ['nfce-events', accessKey] as const,
  },
  mdfes: {
    lists: (orgPk: string | undefined) => ['mdfes', orgPk] as const,
    list: (orgPk: string | undefined, params?: object) => ['mdfes', orgPk, params] as const,
    detail: (accessKey: string) => ['mdfe', accessKey] as const,
    events: (accessKey: string) => ['mdfe-events', accessKey] as const,
    cargoPreview: (orgPk: string | undefined, keys: string[]) => ['mdfe-cargo-preview', orgPk, keys] as const,
  },
  nfses: {
    lists: (orgPk: string | undefined) => ['nfses', orgPk] as const,
    list: (orgPk: string | undefined, params?: object) => ['nfses', orgPk, params] as const,
    detail: (id: string) => ['nfse', id] as const,
    events: (id: string) => ['nfse-events', id] as const,
  },
  /** Inutilizações e lacunas de numeração, por tipo de documento e organização. */
  inutilizations: {
    list: (docType: string, orgPk: string | undefined) => [`${docType}-inutilizations`, orgPk] as const,
    gaps: (docType: string, orgPk: string | undefined) => [`${docType}-number-gaps`, orgPk] as const,
  },
  distributions: {
    history: (docType: string, orgPk: string | undefined) => [`${docType}-distributions`, orgPk] as const,
  },
  municipalParams: (city: string, kind: string, params?: object) => ['municipal-params', city, kind, params] as const,
  auditLogs: {
    list: (orgPk: string | undefined, params?: object) => ['audit-logs', orgPk, params] as const,
  },
  billing: {
    plans: () => ['billing', 'plans'] as const,
    // Keyed on the account, not the active org: the subscription belongs to the
    // token holder, and switching orgs must not refetch or invalidate it.
    subscription: () => ['billing', 'subscription'] as const,
    invoices: (year?: number, month?: number) => ['billing', 'invoices', year, month] as const,
    orgPlan: (orgPk: string | undefined) => ['billing', 'org-plan', orgPk] as const,
  },
  members: (orgPk: string | undefined) => ['members', orgPk] as const,
  invitations: (orgPk: string | undefined) => ['invitations', orgPk] as const,
  invitation: (token: string) => ['invitation', token] as const,
}
