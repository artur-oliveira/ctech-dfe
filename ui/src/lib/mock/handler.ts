/**
 * Axios adapter that serves fixtures for the dev mock API. Attached to the
 * singleton `apiClient` instance when `NEXT_PUBLIC_MOCK_API=true` (see index.ts).
 * Routes by method + path; returns an AxiosResponse on success and rejects an
 * AxiosError on a forced error so `client.ts`'s response interceptor turns it
 * into a real `ApiError` exactly as the backend would.
 */

import {AxiosError, type AxiosAdapter, type AxiosResponse, type InternalAxiosRequestConfig} from 'axios';
import {
  auditLogsFixture,
  billingInvoicesFixture,
  billingPlansFixture,
  billingSubscriptionFixtures,
  certificatesFixture,
  cteConfigFixture,
  contingencyConfigOverrides,
  emissionProblemFixtures,
  printFormatsByRoute,
  printProblemFixtures,
  eventProblemFixtures,
  cteDeriveNotesFixture,
  ctePersonIdsFixture,
  cteDetailFixture,
  cteEventsFixture,
  cteFiscalPrevalidationFixture,
  cteSefazStatusFixture,
  ctesFixture,
  distributionsFixture,
  inutilizationsFixture,
  numberGapsFixture,
  mdfeConfigFixture,
  mdfeDetailFixture,
  mdfeEventsFixture,
  mdfeFiscalPrevalidationFixture,
  mdfeSefazStatusClosedFixture,
  mdfeSefazStatusFixture,
  mdfeUnclosedFixture,
  mdfesFixture,
  ciotCarrierFleetFixture,
  ciotCarrierStatusFixture,
  ciotFloorFixture,
  ciotsFixture,
  ciotValidationProblemFixture,
  meFixture,
  membersFixture,
  nfceConfigFixture,
  nfceDetailFixture,
  nfceEventsFixture,
  nfcesFixture,
  nfeConfigFixture,
  nfeDetailFixture,
  nfeEventsFixture,
  nfesFixture,
  nfeFiscalPrevalidationFixture,
  nfseConfigFixture,
  nfseDistributionsFixture,
  nfseEventsFixture,
  nfsesFixture,
  operationsFixture,
  cargoUnitsFixture,
  fuelPumpsFixture,
  importDeclarationsFixture,
  insurancePoliciesFixture,
  paymentTerminalsFixture,
  productLotsFixture,
  referenceDocumentsFixture,
  tollProvidersFixture,
  serviceLocationsFixture,
  organizationsFixture,
  paymentTermsFixture,
  personsFixture,
  productsFixture,
  receivedNfesFixture,
  rolesFixture,
  servicesFixture,
  taxProfilesFixture,
  vehiclesFixture,
  vehicleSetsFixture,
} from './fixtures';
import type {ContingencyDocFields} from '@/lib/contingency';
import {getMockState, shouldError} from './state';

/** The ctech-account organization the mock plan belongs to, managed by the mock user. */
const mockOrganizationPlan = {organization: {id: 'org_mock', name: 'Escritório Modelo'}, manageable: true};

const MOCK_LATENCY_MS = 250;

/**
 * Cadastros reutilizáveis: mesmo CRUD, mesma paginação, só muda a coleção.
 * `[segmento de rota, fixture, sufixo da chave sintética]`.
 */
const REUSABLE_REGISTRIES: [string, unknown[], string][] = [
  ['tax-profiles', taxProfilesFixture, 'tax-profile'],
  ['operations', operationsFixture, 'operation'],
  ['payment-terms', paymentTermsFixture, 'payment-term'],
  ['vehicle-sets', vehicleSetsFixture, 'vehicle-set'],
  ['service-locations', serviceLocationsFixture, 'service-location'],
  ['reference-documents', referenceDocumentsFixture, 'reference-document'],
  ['payment-terminals', paymentTerminalsFixture, 'payment-terminal'],
  ['fuel-pumps', fuelPumpsFixture, 'fuel-pump'],
  ['cargo-units', cargoUnitsFixture, 'cargo-unit'],
  ['toll-providers', tollProvidersFixture, 'toll-provider'],
  ['insurance-policies', insurancePoliciesFixture, 'insurance-policy'],
  ['product-lots', productLotsFixture, 'product-lot'],
  ['import-declarations', importDeclarationsFixture, 'import-declaration'],
];

function paginated<T>(items: T[]): { items: T[]; next_cursor: string | null; has_next: boolean; previous_cursor: string | null; has_previous: boolean } {
  return {
    items,
    next_cursor: null,
    has_next: false,
    previous_cursor: null,
    has_previous: false,
  };
}

/** Echoes a created/updated entity with a synthetic key so forms resolve. */
function echo(body: unknown, suffix: string) {
  const base = (body && typeof body === 'object' ? body : {}) as Record<string, unknown>;
  return {pk: `mock-${suffix}-${Date.now()}`, sk: `MOCK#${suffix}`, ...base};
}

/**
 * `POST /v1.0/ctes/derive` sobre as fixtures, com as regras da spec
 * (docs/specs/2026-11-05-cte-emissao-rodoviaria.md): remetente/destinatário
 * nulos quando divergem, tomador sugerido por modFrete, somas da carga e o
 * produto de maior valor. Chave fora das fixtures vira `note_not_found`.
 */
function mockCteDerive(body: unknown): RouteResult {
  const parsed = typeof body === 'string' ? JSON.parse(body) : body;
  const keys: string[] = (parsed as {nfe_keys?: string[]})?.nfe_keys ?? [];
  const notes = keys.map((k) => cteDeriveNotesFixture[k]).filter(Boolean);
  const warnings: {code: string; message: string; access_key: string | null}[] = keys
    .filter((k) => !cteDeriveNotesFixture[k])
    .map((k) => ({code: 'note_not_found', message: 'Nota fora do cadastro da organização.', access_key: k}));
  if (notes.length === 0) {
    return {error: {status: 422, data: {title: 'Notas não encontradas', detail: 'Nenhuma das notas está no cadastro.', code: 'note_not_found'}}};
  }

  const single = <T,>(pick: (n: (typeof notes)[number]) => T, by: (v: T) => string): T | null => {
    const first = pick(notes[0]);
    return notes.every((n) => by(pick(n)) === by(first)) ? first : null;
  };
  const sender = single((n) => n.emitter, (p) => p.doc + p.name);
  const recipient = single((n) => n.recipient, (p) => p.doc + p.name);
  if (!sender) warnings.push({code: 'mixed_sender', message: 'Notas com remetentes diferentes.', access_key: null});
  if (!recipient) warnings.push({code: 'mixed_recipient', message: 'Notas com destinatários diferentes.', access_key: null});

  const mod = single((n) => n.mod_frete, (m) => m);
  const taker_suggestion = mod === '0' ? {role: 'remetente', reason: 'mod_frete_cif'}
    : mod === '1' ? {role: 'destinatario', reason: 'mod_frete_fob'} : null;
  if (!taker_suggestion) {
    warnings.push({code: 'taker_ambiguous', message: 'As notas não indicam um único responsável pelo frete.', access_key: null});
  }

  const sum = (pick: (n: (typeof notes)[number]) => string | number | null, dec: number) =>
    notes.reduce((acc, n) => acc + Number(pick(n) ?? 0), 0).toFixed(dec);
  const top = [...notes].sort((a, b) => Number(b.product_value) - Number(a.product_value))[0];
  const party = (p: typeof sender) => (p ? {...p, person_id: ctePersonIdsFixture[p.name] ?? null} : null);
  const place = (p: typeof sender) => (p ? {uf: p.uf, city_ibge: p.city_ibge, city: p.city} : null);

  return {
    data: {
      notes: notes.map((n) => {
        const out: Record<string, unknown> = {...n};
        delete out.product;
        delete out.product_value;
        return out;
      }),
      sender: party(sender),
      recipient: party(recipient),
      taker_suggestion,
      cargo: {
        total_value: sum((n) => n.value, 2),
        total_weight_kg: sum((n) => n.weight_kg, 3),
        volumes: Number(sum((n) => n.volumes, 0)),
        predominant_product: top.product,
      },
      origin: place(sender),
      destination: place(recipient),
      warnings,
    },
  };
}

const CONFIG_FIXTURES: Record<string, object> = {
  nfe: nfeConfigFixture, nfce: nfceConfigFixture, cte: cteConfigFixture, mdfe: mdfeConfigFixture,
};

/** Forma de emissão gravada pelo `PUT /{doc}-emission`, por documento; vale até recarregar a página. */
const savedEmission: Record<string, object> = {};

function mockConfig(doc: string): object {
  const overrides = getMockState().contingency ? contingencyConfigOverrides[doc] : undefined;
  return {...CONFIG_FIXTURES[doc], ...overrides, ...savedEmission[doc]};
}

const EMISSION_PROBLEMS = {invalid: 'invalid', reason: 'reason'} as const;

/**
 * `PUT /{doc}-emission`: grava a forma de emissão como a API (início da contingência ao sair de `normal`, apagado
 * ao voltar) e devolve a configuração. `?mock=emission-422|emission-reason` força os dois 422.
 */
function mockEmission(doc: string, raw: unknown): RouteResult {
  const body = (typeof raw === 'string' ? JSON.parse(raw) : raw) as {emission_type: string; auto_contingency?: boolean; auto_contingency_type?: string; justification?: string};
  const forced = getMockState().emissionProblem;
  if (forced === EMISSION_PROBLEMS.invalid || forced === EMISSION_PROBLEMS.reason) {
    return {error: {status: 422, data: emissionProblemFixtures[forced]}};
  }
  // A API exige o tipo com a flag ligada e só aceita uma das `emission_options` além de `normal`.
  if (body.auto_contingency) {
    if (!body.auto_contingency_type) return {error: {status: 422, data: emissionProblemFixtures.autoRequired}};
    const valid = ((mockConfig(doc) as {emission_options?: string[]}).emission_options ?? []).filter((o) => o !== 'normal');
    if (!valid.includes(body.auto_contingency_type)) return {error: {status: 422, data: emissionProblemFixtures.autoInvalid}};
  }
  const leaving = body.emission_type !== 'normal';
  if (leaving && (body.justification ?? '').trim().length < 15) {
    return {error: {status: 422, data: emissionProblemFixtures.reason}};
  }
  const before = mockConfig(doc) as {contingency_since?: string};
  savedEmission[doc] = {
    emission_type: body.emission_type,
    auto_contingency: body.auto_contingency ?? false,
    auto_contingency_type: body.auto_contingency ? body.auto_contingency_type : null,
    contingency_since: leaving ? before.contingency_since ?? new Date().toISOString() : null,
    contingency_reason: leaving ? body.justification : null,
  };
  return {data: mockConfig(doc)};
}

/** Recusa forçada dos eventos F9 por `?mock=event-invalid|event-not-authorized|event-not-found|no-certificate`. */
function eventProblem(): RouteResult {
  const p = eventProblemFixtures[getMockState().eventProblem!];
  return {error: {status: p.status, data: p}};
}

type RouteResult = {data: unknown; status?: number} | {error: {status: number; data: unknown}}

function route(method: string, path: string, body: unknown, params?: Record<string, unknown>): RouteResult {
  const m = method.toLowerCase();
  const key = `${m} ${path}`;

  // Auth (always succeed — see SAFE_PATHS in state.ts)
  if (key === 'get /v1.0/auth/me') return {data: meFixture};
  if (key === 'get /v1.0/auth/roles') return {data: rolesFixture};

  // Organizations
  if (key === 'get /v1.0/organizations') return {data: organizationsFixture};
  if (m === 'post' && path === '/v1.0/organizations') return {data: organizationsFixture[0]};
  if (m === 'put' && path.startsWith('/v1.0/organizations/') && !path.includes('-config')) return {data: organizationsFixture[0]};
  if (m === 'delete' && path.startsWith('/v1.0/organizations/')) return {data: undefined};
  // Detalhe da organização — traz `person`, que a listagem de /auth/me não tem.
  if (m === 'get' && /^\/v1\.0\/organizations\/[^/]+$/.test(path)) {
    const org = organizationsFixture[0];
    // `?mock=regime-normal`: CRT 3, para rever o bloco IBS/CBS do CT-e.
    return {data: getMockState().regimeNormal ? {...org, person: {...org.person, crt: 3}} : org};
  }

  // Configs (path-based org in URL)
  {
    const emission = /\/(nfe|nfce|cte|mdfe)-emission$/.exec(path);
    if (m === 'put' && emission) return mockEmission(emission[1], body);
    const cfg = /\/(nfe|nfce|cte|mdfe)-config$/.exec(path);
    if (cfg) return {data: mockConfig(cfg[1])};
  }
  if (path.endsWith('/nfse-config')) return {data: nfseConfigFixture};

  // Certificates / members
  if (path.endsWith('/certificates')) return {data: certificatesFixture};
  if (m === 'post' && path.endsWith('/certificates')) return {data: echo(body, 'cert')};
  if (m === 'delete' && path.includes('/certificates/')) return {data: undefined};
  if (path.endsWith('/members')) return {data: membersFixture};
  if (path.endsWith('/invitations')) return {data: []};

  // Catalog
  if (key === 'get /v1.0/products') return {data: paginated(productsFixture)};
  if (m === 'post' && path === '/v1.0/products') return {data: echo(body, 'product')};
  if (m === 'put' && path.startsWith('/v1.0/products/')) return {data: echo(body, 'product')};
  if (m === 'delete' && path.startsWith('/v1.0/products/')) return {data: undefined};

  if (key === 'get /v1.0/vehicles') return {data: paginated(vehiclesFixture)};
  if (m === 'post' && path === '/v1.0/vehicles') return {data: echo(body, 'vehicle')};
  if (m === 'put' && path.startsWith('/v1.0/vehicles/')) return {data: echo(body, 'vehicle')};
  if (m === 'delete' && path.startsWith('/v1.0/vehicles/')) return {data: undefined};

  if (key === 'get /v1.0/persons') return {data: paginated(personsFixture)};
  if (m === 'post' && path === '/v1.0/persons') return {data: echo(body, 'person')};
  if (m === 'put' && path.startsWith('/v1.0/persons/')) return {data: echo(body, 'person')};
  if (m === 'delete' && path.startsWith('/v1.0/persons/')) return {data: undefined};
  // Busca por CPF/CNPJ — usada pelos atalhos "Recentes" da emissão.
  if (m === 'get' && path.startsWith('/v1.0/persons/')) return {data: personsFixture[0]};

  if (key === 'get /v1.0/services') return {data: paginated(servicesFixture)};
  if (m === 'post' && path === '/v1.0/services') return {data: echo(body, 'service')};
  if (m === 'put' && path.startsWith('/v1.0/services/')) return {data: echo(body, 'service')};
  if (m === 'delete' && path.startsWith('/v1.0/services/')) return {data: undefined};
  if (m === 'get' && path.startsWith('/v1.0/services/')) return {data: servicesFixture[0]};

  // Cadastros reutilizáveis — mesma forma para os quatro, só muda a fixture.
  for (const [segment, fixture, suffix] of REUSABLE_REGISTRIES) {
    if (key === `get /v1.0/${segment}`) return {data: paginated(fixture)};
    if (m === 'post' && path === `/v1.0/${segment}`) return {data: echo(body, suffix)};
    if (m === 'put' && path.startsWith(`/v1.0/${segment}/`)) return {data: echo(body, suffix)};
    if (m === 'delete' && path.startsWith(`/v1.0/${segment}/`)) return {data: undefined};
    if (m === 'get' && path.startsWith(`/v1.0/${segment}/`)) return {data: fixture[0]};
  }

  // Documents
  // Recebidas (incoming 1) têm fixture própria: a emissão de CT-e precisa de nota de outro remetente.
  if (key === 'get /v1.0/nfes') return {data: paginated(Number(params?.incoming) === 1 ? receivedNfesFixture : nfesFixture)};
  if (m === 'post' && path === '/v1.0/nfes') {
    if (getMockState().emitPrevalidation) return {error: {status: 422, data: nfeFiscalPrevalidationFixture}};
    if (getMockState().emissionProblem === 'svcUf') return {error: {status: 400, data: emissionProblemFixtures.svcUf}};
    const epec = {epecUf: 'epecUf', epecStale: 'epecStale', epecInvalid: 'epecInvalid'} as const;
    const forced = getMockState().emissionProblem;
    if (forced && forced in epec) return {error: {status: 422, data: emissionProblemFixtures[forced as keyof typeof epec]}};
    return {data: echo(body, 'nfe')};
  }
  if (key === 'get /v1.0/nfces') return {data: paginated(nfcesFixture)};
  if (m === 'post' && path === '/v1.0/nfces') {
    if (getMockState().emissionProblem === 'qr444') return {error: {status: 400, data: emissionProblemFixtures.qr444}};
    return {data: echo(body, 'nfce')};
  }
  if (key === 'get /v1.0/mdfes') return {data: paginated(mdfesFixture)};
  if (m === 'post' && path === '/v1.0/mdfes') {
    if (getMockState().emitPrevalidation) return {error: {status: 422, data: mdfeFiscalPrevalidationFixture}};
    return {data: echo(body, 'mdfe')};
  }
  // Consultas síncronas na SEFAZ. Antes das rotas de detalhe, que também casam com `/mdfes/{key}`.
  if (key === 'get /v1.0/mdfes/unclosed') return {data: mdfeUnclosedFixture};
  {
    const status = /^get \/v1\.0\/mdfes\/([^/]+)\/sefaz-status$/.exec(key);
    // O terceiro manifesto está com encerramento pendente aqui e já encerrado na SEFAZ: a divergência da tela.
    if (status) {
      const closed = status[1] === mdfesFixture[1].sk || status[1] === mdfesFixture[2].sk;
      return {data: {...(closed ? mdfeSefazStatusClosedFixture : mdfeSefazStatusFixture), access_key: status[1]}};
    }
  }
  if (m === 'post' && path === '/v1.0/mdfes/cargo-preview') {
    return {data: {items: mdfeDetailFixture.documents, total_weight: '2450.000', total_value: '3589.80'}};
  }
  if (key === 'post /v1.0/ctes/derive') return mockCteDerive(body);
  if (key === 'get /v1.0/ctes') return {data: paginated(ctesFixture)};
  if (key === 'post /v1.0/ctes') {
    if (getMockState().emitPrevalidation) return {error: {status: 422, data: cteFiscalPrevalidationFixture}};
    return {data: {...ctesFixture[2], nfe_keys: [], origin: null, destination: null, cargo: null, xml_s3_key: null}, status: 202};
  }
  {
    const cte = /^(get|post) \/v1\.0\/ctes\/([^/]+)\/(sefaz-status|events|cancel|correction-letter|service-disagreement|service-disagreement-cancel|delivery-receipt|delivery-failure|delivery-cancel)$/.exec(key);
    if (cte) {
      const [, , sk, action] = cte;
      if (action === 'sefaz-status') return {data: cteSefazStatusFixture(sk)};
      if (action === 'events') return {data: paginated(cteEventsFixture(sk))};
      if (m === 'post' && /^delivery-/.test(action) && getMockState().eventProblem) return eventProblem();
      return {data: cteDetailFixture(sk)};
    }
    if (/^get \/v1\.0\/ctes\/[^/]+\/events\/[^/]+\/xml$/.test(key)) {
      return {data: {url: 'data:text/xml,%3CprocEventoCTe%2F%3E', expires_at: '2099-01-01T00:00:00Z', filename: 'evento.xml', content_type: 'application/xml'}};
    }
  }
  if (key === 'get /v1.0/nfses') return {data: paginated(nfsesFixture)};
  if (m === 'post' && path === '/v1.0/nfses') return {data: echo(body, 'nfse')};

  // Inutilização de numeração — antes das rotas de detalhe, que também casam
  // com `/nfes/{key}`.
  if (/^get \/v1\.0\/nfc?es\/inutilizations\/gaps$/.test(key)) return {data: {items: numberGapsFixture}};
  if (/^get \/v1\.0\/nfc?es\/inutilizations$/.test(key)) return {data: paginated(inutilizationsFixture)};
  if (/^post \/v1\.0\/nfc?es\/inutilizations$/.test(key)) return {data: inutilizationsFixture[1]};
  if (/^get \/v1\.0\/nfc?es\/inutilizations\/[^/]+\/xml$/.test(key)) return {data: '<ProcInutNFe/>'};

  // Event timelines. Antes das rotas de detalhe: `/nfes/{key}/events` também
  // casa com o padrão de detalhe.
  if (/^get \/v1\.0\/nfes\/[^/]+\/events$/.test(key)) return {data: paginated(nfeEventsFixture)};
  if (/^get \/v1\.0\/nfces\/[^/]+\/events$/.test(key)) return {data: paginated(nfceEventsFixture)};
  if (/^get \/v1\.0\/mdfes\/[^/]+\/events$/.test(key)) return {data: paginated(mdfeEventsFixture)};
  if (/^get \/v1\.0\/nfses\/[^/]+\/events$/.test(key)) return {data: paginated(nfseEventsFixture)};

  // Impressos: o nome do arquivo é sempre `{chave}.pdf`; `format` fora da lista da rota é 400 e, na NF-e em EPEC
  // (tpEmis 4, posição 35 da chave), simplificado e etiqueta são 422.
  {
    const printed = /^get \/v1\.0\/(?:nfes|nfces|mdfes|ctes)\/([^/]+)\/(danfe|danfce|damdfe|dacte)$/.exec(key);
    if (printed) {
      const [, sk, doc] = printed;
      const forced = getMockState().printProblem;
      if (forced) return {error: {status: printProblemFixtures[forced].status, data: printProblemFixtures[forced]}};
      const format = params?.format as string | undefined;
      if (format && !printFormatsByRoute[doc].includes(format)) return {error: {status: 400, data: printProblemFixtures.invalidFormat}};
      if ((format === 'simplificado' || format === 'etiqueta') && sk[34] === '4') {
        return {error: {status: 422, data: printProblemFixtures.notAllowed}};
      }
      return {data: {url: 'data:application/pdf;base64,JVBERi0xLjQK', expires_at: '2099-01-01T00:00:00Z', filename: `${sk}.pdf`, content_type: 'application/pdf', cached: false}};
    }
  }

  // Detail routes (any access key)
  {
    const found = /^get \/v1\.0\/(nfes|nfces)\/([^/]+)$/.exec(key);
    if (found) {
      const [, kind, sk] = found;
      const base = kind === 'nfes' ? nfeDetailFixture : nfceDetailFixture;
      const row = (kind === 'nfes' ? nfesFixture : nfcesFixture).find((d) => d.sk === sk) as ContingencyDocFields & typeof nfesFixture[number] | undefined;
      // Documento em contingência traz o próprio estado; o pendente ainda não tem XML autorizado.
      return {data: row?.contingency_state ? {...base, ...row, xml_s3_key: (row.contingency_state === 'pending' || row.contingency_state === 'awaiting_epec') ? null : base.xml_s3_key} : base};
    }
  }
  {
    const found = /^get \/v1\.0\/mdfes\/([^/]+)$/.exec(key);
    if (found) {
      const row = mdfesFixture.find((d) => d.sk === found[1]) as ContingencyDocFields & typeof mdfesFixture[number] | undefined;
      return {data: row?.contingency_state ? {...mdfeDetailFixture, ...row, xml_s3_key: null} : mdfeDetailFixture};
    }
  }
  {
    const cte = /^get \/v1\.0\/ctes\/([^/]+)$/.exec(key);
    if (cte) return {data: cteDetailFixture(cte[1])};
  }
  if (/^get \/v1\.0\/nfses\/[^/]+$/.test(key)) return {data: nfsesFixture[0]};
  if (/^post \/v1\.0\/nfes\/.+\/(ibscbs-events|delivery-receipt|delivery-receipt-cancel|interested-actor)$/.test(key)) {
    return getMockState().eventProblem ? eventProblem() : {data: nfeDetailFixture};
  }
  if (/^post \/v1\.0\/nfes\/.+\/(cancel|correction-letter|manifestation)$/.test(key)) return {data: nfeDetailFixture};
  if (/^post \/v1\.0\/nfces\/.+\/(cancel|substitute)$/.test(key)) return {data: nfceDetailFixture};
  if (/^post \/v1\.0\/mdfes\/.+\/(cancel|close|include-condutor|include-dfe)$/.test(key)) return {data: mdfeDetailFixture};
  if (/^post \/v1\.0\/nfses\/.+\/(cancel|substitute|events)$/.test(key)) return {data: nfsesFixture[0]};

  // CIOT (ANTT): a declaração entra `pending` e o resultado chegaria por WebSocket; o mock só devolve a linha.
  if (key === 'get /v1.0/ciots') return {data: paginated(ciotsFixture)};
  if (m === 'post' && path === '/v1.0/ciots') {
    if (getMockState().ciotProblem === 'invalid') return {error: {status: 422, data: ciotValidationProblemFixture}};
    return {data: {...ciotsFixture[1], id: 'f6a1b2c3d4e5'}, status: 202};
  }
  {
    const ciot = /^get \/v1\.0\/ciots\/([^/]+)$/.exec(key);
    if (ciot) return {data: ciotsFixture.find((c) => c.id === ciot[1]) ?? ciotsFixture[0]};
    const op = /^post \/v1\.0\/ciots\/([^/]+)\/(cancellation|rectification|closure)$/.exec(key);
    if (op) {
      const pending = {cancellation: 'cancel', rectification: 'rectify', closure: 'close'}[op[2] as 'cancellation'];
      return {data: {...(ciotsFixture.find((c) => c.id === op[1]) ?? ciotsFixture[0]), pending_operation: pending}, status: 202};
    }
  }
  if (/^get \/v1\.0\/ciot-carriers\/[^/]+\/(status|fleet)$/.test(key)) {
    if (getMockState().anttDown) return {error: {status: 503, data: {detail: 'serviço da ANTT indisponível; tente novamente', code: 'ciot_unavailable'}}};
    return {data: key.endsWith('/fleet') ? ciotCarrierFleetFixture : ciotCarrierStatusFixture};
  }
  // Piso mínimo: enquanto a tabela oficial não foi verificada a api responde 503, nunca um número.
  if (key === 'get /v1.0/freight-floor') {
    if (getMockState().freightFloor) return {data: ciotFloorFixture};
    return {error: {status: 503, data: {detail: 'Tabela do piso mínimo de frete ainda não verificada.', code: 'ciot_floor_unverified'}}};
  }

  // Distributions
  if (key === 'get /v1.0/nfse/distributions') return {data: paginated(nfseDistributionsFixture)};
  if (/^get \/v1\.0\/distributions\/\w+\/history$/.test(key)) return {data: paginated(distributionsFixture)};
  if (m === 'post' && /^post \/v1\.0\/distributions\/\w+\/sync$/.test(key)) return {data: {enqueued: true, job_id: 'mock-job'}};
  if (/^get \/v1\.0\/distributions\/\w+\/(nsu|key)\//.test(key)) return {data: distributionsFixture[0]};

  // Billing — the scenario decides what the account's standing is.
  if (key === 'get /v1.0/billing/plans') return {data: billingPlansFixture};
  if (key === 'get /v1.0/billing/subscription') {
    // The organization's plan, seen by one of its owners or admins.
    return {data: {...billingSubscriptionFixtures[getMockState().billing], ...mockOrganizationPlan}};
  }
  if (key === 'get /v1.0/billing/invoices') return {data: {data: billingInvoicesFixture}};
  if (m === 'post' && path === '/v1.0/billing/subscription') {
    // Choosing a paid plan lands on INCOMPLETE with an invoice, which is what
    // production does — the checkout is a redirect away, not a state jump.
    return {data: {...billingSubscriptionFixtures.checkout_pending, invoice: billingInvoicesFixture[1]}, status: 201};
  }
  if (m === 'post' && path === '/v1.0/billing/subscription/change') {
    return {data: {...billingSubscriptionFixtures.pro_active, invoice: billingInvoicesFixture[1]}};
  }
  if (m === 'post' && path === '/v1.0/billing/subscription/cancel') {
    return {data: {...billingSubscriptionFixtures.pro_active, cancel_at_period_end: true}};
  }
  if (/^get \/v1\.0\/organizations\/[^/]+\/plan$/.test(key)) {
    return {data: {...billingSubscriptionFixtures[getMockState().billing], manageable: false}};
  }

  // Audit logs
  if (key === 'get /v1.0/audit-logs') return {data: paginated(auditLogsFixture)};

  // Fallback: never hard-crash an unmodeled endpoint.
  if (m === 'get') return {data: paginated([])};
  return {data: echo(body, 'generic')};
}

export const mockAdapter: AxiosAdapter = async (config) => {
  const req = config as InternalAxiosRequestConfig;
  const method = (req.method ?? 'get').toLowerCase();
  const path = (req.url ?? '').split('?')[0];

  await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));

  if (shouldError(path)) {
    const {status, message} = getMockState();
    const body = {detail: message};
    const error = new AxiosError(message, 'ERR_BAD_RESPONSE', req, undefined, {
      status,
      statusText: 'Mock Error',
      data: body,
      headers: {},
      config: req,
    } as never);
    throw error;
  }

  const result = route(method, path, req.data, req.params as Record<string, unknown> | undefined);
  if ('error' in result) {
    const error = new AxiosError('Mock error', 'ERR_BAD_RESPONSE', req, undefined, {
      status: result.error.status,
      statusText: 'Mock Error',
      data: result.error.data,
      headers: {},
      config: req,
    } as never);
    throw error;
  }

  const response: AxiosResponse = {
    data: result.data,
    status: result.status ?? 200,
    statusText: 'OK',
    headers: {},
    config: req,
    request: {},
  };
  return response;
};
