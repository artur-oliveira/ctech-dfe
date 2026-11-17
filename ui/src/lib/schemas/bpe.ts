import {z} from 'zod';
import {
  BPE_BOARD_PASS_MAX, BPE_BOARD_PASS_MIN, BPE_CBS_RATE_FROM_YEAR, BPE_IBS_RATE_FROM_YEAR, BPE_ICMS_WITH_RATE, BPE_KEY_PATTERN,
  BPE_PAYMENT_OTHER, BPE_TA_CLASSES, BPE_TA_NO_TAX_CST,
} from '@/lib/constants/bpe';
import {validateCNPJ, validateCPF} from '@/lib/utils/validators';
import type {BpeAddressIn, BpeAgencyIn, BpeBuyerIn, BpeIcmsIn, BpeNormalEmit, BpeSubstitutionIn, BpeTaEmit, BpeTripIn} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());
const optDecimal = (s: string) => blank(s) || decimal.test(s);
const digits = (s: string) => s.replace(/\D/g, '');

const agencyAddressSchema = z.object({
  street: z.string(),
  number: z.string(),
  neighborhood: z.string(),
  ibge_code: z.string(),
  city: z.string(),
  uf: z.string(),
  postal_code: z.string(),
});

const emptyAgencyAddress = (): z.infer<typeof agencyAddressSchema> => ({street: '', number: '', neighborhood: '', ibge_code: '', city: '', uf: '', postal_code: ''});

/**
 * `datetime-local` entrega AAAA-MM-DDThh:mm sem fuso; o leiaute exige o deslocamento (AAAA-MM-DDThh:mm:ss±hh:mm),
 * aqui o do navegador (o Brasil usa -03:00).
 */
export function toOffsetDateTime(local: string): string {
  const offset = -new Date(local).getTimezoneOffset();
  const sign = offset < 0 ? '-' : '+';
  const abs = Math.abs(offset);
  const hh = String(Math.floor(abs / 60)).padStart(2, '0');
  const mm = String(abs % 60).padStart(2, '0');
  return `${local}:00${sign}${hh}:${mm}`;
}

const placeSchema = z.object({
  uf: z.string().length(2, 'Selecione a UF'),
  ibge_code: z.string().regex(/^\d{7}$/, 'Código IBGE com 7 dígitos'),
});

const tripSchema = z.object({
  route_code: z.string(),
  route_name: z.string(),
  flight_number: z.string(),
  carrier: z.string(),
  from_airport: z.string(),
  to_airport: z.string(),
  trip_type: z.string().min(1, 'Selecione o tipo'),
  service_type: z.string().min(1, 'Selecione o serviço'),
  accommodation: z.string().min(1, 'Selecione a acomodação'),
  leg_type: z.string().min(1, 'Selecione o trecho'),
  departure: z.string().min(1, 'Informe a partida'),
  connection: z.string(),
  prefix: z.string().max(20),
  seat: z.string().refine((v) => blank(v) || /^\d{1,3}$/.test(v), 'Poltrona com até 3 dígitos'),
  platform: z.string().max(20),
});

const componentSchema = z.object({
  type: z.string().min(1, 'Selecione o componente'),
  value: z.string().regex(decimal, 'Valor inválido'),
});

const paymentSchema = z.object({
  type: z.string().min(1, 'Selecione a forma'),
  value: z.string().regex(decimal, 'Valor inválido'),
  desc: z.string().max(60),
});

export const bpeTicketSchema = z.object({
  variant: z.enum(['normal', 'ta']),
  modal: z.string(),
  presence: z.string().min(1, 'Selecione a presença'),
  origin: placeSchema,
  destination: placeSchema,
  passage: z.object({
    from_code: z.string(),
    from_name: z.string(),
    to_code: z.string(),
    to_name: z.string(),
    boarding: z.string().min(1, 'Informe o embarque'),
    validity: z.string().min(1, 'Informe a validade'),
    identified: z.boolean(),
    passenger: z.object({
      name: z.string(),
      doc_type: z.string(),
      doc_number: z.string().max(20),
    }),
  }),
  buyer: z.object({
    enabled: z.boolean(),
    name: z.string().max(60),
    doc_kind: z.enum(['cnpj', 'cpf', 'foreign']),
    document: z.string(),
    ie: z.string().max(14),
    street: z.string().max(60),
    number: z.string().max(60),
    complement: z.string().max(60),
    neighborhood: z.string().max(60),
    ibge_code: z.string(),
    city: z.string().max(60),
    postal_code: z.string(),
    uf: z.string(),
  }),
  trips: z.array(tripSchema).min(1, 'Informe ao menos um trecho').max(20),
  components: z.array(componentSchema).max(20),
  discount: z.string(),
  discount_type: z.string(),
  discount_desc: z.string().max(100),
  discount_code: z.string().max(20),
  payments: z.array(paymentSchema).max(10),
  icms: z.object({cst: z.string(), rate: z.string()}),
  tax: z.object({
    class_trib: z.string(),
    donation: z.boolean(),
    ibs_uf_rate: z.string(),
    ibs_mun_rate: z.string(),
    cbs_rate: z.string(),
    devolution: z.object({ibs_uf: z.string(), ibs_mun: z.string(), cbs: z.string()}),
    reversal: z.object({ibs: z.string(), cbs: z.string()}),
    government: z.object({enabled: z.boolean(), entity: z.string(), reducer: z.string(), operation: z.string(), references: z.string()}),
    simples: z.object({enabled: z.boolean(), ibs_rate: z.string(), cbs_rate: z.string()}),
    total_taxes: z.string(),
    fiscal_info: z.string().max(2000),
  }),
  iata_number: z.string().max(20),
  purchase_type: z.enum(['normal', 'multiple', 'linked']),
  multiple_key: z.string(),
  agency: z.object({enabled: z.boolean(), name: z.string(), cnpj: z.string(), address: agencyAddressSchema}),
  substitution: z.object({enabled: z.boolean(), original_key: z.string(), type: z.string()}),
  board_pass: z.string(),
  additional_info: z.string().max(5000),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const ta = f.variant === 'ta';
  const linked = ta && f.purchase_type === 'linked';
  const multiple = ta && f.purchase_type === 'multiple';
  if (!ta && blank(f.modal)) issue(['modal'], 'Selecione o modal');
  if (multiple && !f.buyer.enabled) issue(['buyer', 'enabled'], 'O BP-e múltiplo exige o comprador');
  if (f.agency.enabled) {
    const a = f.agency;
    if (a.name.trim().length < 2) issue(['agency', 'name'], 'Informe o nome da agência');
    if (!validateCNPJ(a.cnpj)) issue(['agency', 'cnpj'], 'CNPJ inválido');
    const at = (k: string, message: string) => issue(['agency', 'address', k], message);
    if (a.address.street.trim().length < 2) at('street', 'Informe o logradouro');
    if (blank(a.address.number)) at('number', 'Informe o número');
    if (a.address.neighborhood.trim().length < 2) at('neighborhood', 'Informe o bairro');
    if (!/^\d{7}$/.test(a.address.ibge_code)) at('ibge_code', 'Código IBGE com 7 dígitos');
    if (a.address.city.trim().length < 2) at('city', 'Informe o município');
    if (a.address.uf.length !== 2) at('uf', 'Selecione a UF');
    if (!blank(a.address.postal_code) && digits(a.address.postal_code).length !== 8) at('postal_code', 'CEP com 8 dígitos');
  }
  if (f.substitution.enabled) {
    if (!BPE_KEY_PATTERN.test(f.substitution.original_key.trim().toUpperCase())) issue(['substitution', 'original_key'], 'Chave de acesso com 44 caracteres');
    if (blank(f.substitution.type)) issue(['substitution', 'type'], 'Selecione o tipo da substituição');
  }
  if (!ta && !blank(f.board_pass) && (f.board_pass.length < BPE_BOARD_PASS_MIN || f.board_pass.length > BPE_BOARD_PASS_MAX)) {
    issue(['board_pass'], `Conteúdo do PDF417 com ${BPE_BOARD_PASS_MIN} a ${BPE_BOARD_PASS_MAX} caracteres`);
  }
  if (linked) {
    if (!BPE_KEY_PATTERN.test(f.multiple_key.trim().toUpperCase())) issue(['multiple_key'], 'Chave de acesso do BP-e múltiplo com 44 caracteres');
  } else {
    if (f.components.length < 1) issue(['components'], 'Informe ao menos um componente');
    if (f.payments.length < 1) issue(['payments'], 'Informe ao menos um pagamento');
  }
  if (!optDecimal(f.discount)) issue(['discount'], 'Desconto inválido');
  if (Number(f.discount) > 0) {
    if (blank(f.discount_type)) issue(['discount_type'], 'Selecione o tipo do desconto');
    if (f.discount_desc.trim().length < 2) issue(['discount_desc'], 'Descreva o desconto (mínimo 2 caracteres)');
  }
  if (f.discount_type === '99' && blank(f.discount_code)) issue(['discount_code'], 'Informe o código do desconto');
  f.payments.forEach((p, i) => {
    if (p.type === BPE_PAYMENT_OTHER && p.desc.trim().length < 2) issue(['payments', i, 'desc'], 'Descreva a forma de pagamento');
  });
  if (f.passage.validity && f.passage.boarding && f.passage.validity < f.passage.boarding) {
    issue(['passage', 'validity'], 'A validade não pode ser anterior ao embarque');
  }
  if (ta && !multiple && !f.passage.identified) issue(['passage', 'identified'], 'O TA exige a identificação do passageiro');
  if (f.passage.identified) {
    if (f.passage.passenger.name.trim().length < 2) issue(['passage', 'passenger', 'name'], 'Informe o nome do passageiro');
    if (blank(f.passage.passenger.doc_type)) issue(['passage', 'passenger', 'doc_type'], 'Selecione o documento');
    if (blank(f.passage.passenger.doc_number)) issue(['passage', 'passenger', 'doc_number'], 'Informe o número do documento');
  }
  const b = f.buyer;
  if (b.enabled) {
    const at = (k: string, message: string) => issue(['buyer', k], message);
    if (b.name.trim().length < 2) at('name', 'Informe o nome do comprador');
    if (b.doc_kind === 'cnpj' && !validateCNPJ(b.document)) at('document', 'CNPJ inválido');
    if (b.doc_kind === 'cpf' && !validateCPF(b.document)) at('document', 'CPF inválido');
    if (b.doc_kind === 'foreign' && (b.document.trim().length < 2 || b.document.trim().length > 20)) at('document', 'Identificador de 2 a 20 caracteres');
    if (b.street.trim().length < 2) at('street', 'Informe o logradouro');
    if (blank(b.number)) at('number', 'Informe o número');
    if (b.neighborhood.trim().length < 2) at('neighborhood', 'Informe o bairro');
    if (!/^\d{7}$/.test(b.ibge_code)) at('ibge_code', 'Código IBGE com 7 dígitos');
    if (b.city.trim().length < 2) at('city', 'Informe o município');
    if (b.uf.length !== 2) at('uf', 'Selecione a UF');
    if (!blank(b.postal_code) && b.postal_code.replace(/\D/g, '').length !== 8) at('postal_code', 'CEP com 8 dígitos');
  }
  if (ta && !linked) {
    const t = f.tax;
    const cls = BPE_TA_CLASSES.find((c) => c.value === t.class_trib);
    if (!cls) issue(['tax', 'class_trib'], 'Selecione a classificação tributária');
    if (cls?.donation === 1 && !t.donation) issue(['tax', 'donation'], 'Esta classificação exige o indicador de doação');
    if (cls?.donation === 0 && t.donation) issue(['tax', 'donation'], 'Esta classificação não admite doação');
    const year = new Date().getFullYear();
    const taxed = cls !== undefined && cls.cst !== BPE_TA_NO_TAX_CST;
    if (taxed && year >= BPE_CBS_RATE_FROM_YEAR && !decimal.test(t.cbs_rate)) issue(['tax', 'cbs_rate'], 'Informe a alíquota da CBS');
    if (taxed && year >= BPE_IBS_RATE_FROM_YEAR) {
      (['ibs_uf_rate', 'ibs_mun_rate'] as const).forEach((k) => {
        if (!decimal.test(t[k])) issue(['tax', k], 'Informe a alíquota do IBS');
      });
    }
    (['ibs_uf_rate', 'ibs_mun_rate', 'cbs_rate', 'total_taxes'] as const).forEach((k) => {
      if (!optDecimal(t[k])) issue(['tax', k], 'Valor inválido');
    });
    (['ibs_uf', 'ibs_mun', 'cbs'] as const).forEach((k) => {
      if (!optDecimal(t.devolution[k])) issue(['tax', 'devolution', k], 'Valor inválido');
    });
    if (cls?.reversal) {
      (['ibs', 'cbs'] as const).forEach((k) => {
        if (!decimal.test(t.reversal[k])) issue(['tax', 'reversal', k], 'Informe o valor estornado');
      });
    }
    if (t.government.enabled) {
      const g = t.government;
      if (blank(g.entity)) issue(['tax', 'government', 'entity'], 'Selecione o ente');
      if (blank(g.operation)) issue(['tax', 'government', 'operation'], 'Selecione a operação');
      if (!decimal.test(g.reducer)) issue(['tax', 'government', 'reducer'], 'Informe o redutor (0 se não houver)');
      const keys = govRefs(g);
      if (keys.some((k) => !/^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$/.test(k))) issue(['tax', 'government', 'references'], 'Chave de acesso inválida');
      if ((g.operation === '1' || g.operation === '4') && keys.length > 0) issue(['tax', 'government', 'references'], 'Esta operação não leva chave referenciada');
      if (g.operation === '2' && keys.length !== 1) issue(['tax', 'government', 'references'], 'Informe uma chave referenciada');
      if (g.operation === '3' && keys.length < 1) issue(['tax', 'government', 'references'], 'Informe ao menos uma chave referenciada');
    }
    if (t.simples.enabled) {
      (['ibs_rate', 'cbs_rate'] as const).forEach((k) => {
        if (!optDecimal(t.simples[k])) issue(['tax', 'simples', k], 'Alíquota inválida');
      });
    }
  } else if (!ta) {
    if (blank(f.icms.cst)) issue(['icms', 'cst'], 'Selecione o ICMS');
    if (BPE_ICMS_WITH_RATE.includes(f.icms.cst) && !decimal.test(f.icms.rate)) issue(['icms', 'rate'], 'Informe a alíquota do ICMS');
    if (f.passage.from_code.trim().length < 1) issue(['passage', 'from_code'], 'Informe o código do local de origem');
    if (f.passage.from_name.trim().length < 2) issue(['passage', 'from_name'], 'Informe o local de origem');
    if (f.passage.to_code.trim().length < 1) issue(['passage', 'to_code'], 'Informe o código do local de destino');
    if (f.passage.to_name.trim().length < 2) issue(['passage', 'to_name'], 'Informe o local de destino');
  }
  f.trips.forEach((t, i) => {
    const at = (k: string, message: string) => issue(['trips', i, k], message);
    if (ta) {
      if (blank(t.flight_number)) at('flight_number', 'Informe o número do voo');
      if (t.carrier.trim().length < 2) at('carrier', 'Sigla da companhia (2 ou 3 caracteres)');
      if (!/^[A-Za-z0-9]{3,4}$/.test(t.from_airport)) at('from_airport', 'Aeroporto com 3 ou 4 caracteres');
      if (!/^[A-Za-z0-9]{3,4}$/.test(t.to_airport)) at('to_airport', 'Aeroporto com 3 ou 4 caracteres');
    } else {
      if (blank(t.route_code)) at('route_code', 'Informe o código da linha');
      if (t.route_name.trim().length < 2) at('route_name', 'Informe a linha');
    }
    if (t.leg_type === '3' && blank(t.connection)) at('connection', 'Informe a conexão');
  });
});

export type BpeTicketForm = z.infer<typeof bpeTicketSchema>
export type BpeTripForm = BpeTicketForm['trips'][number]

export function emptyBpeTrip(variant: 'normal' | 'ta'): BpeTripForm {
  const ta = variant === 'ta';
  return {
    route_code: '', route_name: '', flight_number: '', carrier: '', from_airport: '', to_airport: '',
    trip_type: '00', service_type: ta ? '12' : '1', accommodation: ta ? '6' : '1', leg_type: '1',
    departure: '', connection: '', prefix: '', seat: '', platform: '',
  };
}

export function emptyBpeTicket(variant: 'normal' | 'ta' = 'normal'): BpeTicketForm {
  return {
    variant, modal: variant === 'ta' ? '' : 'rodoviario', presence: '1',
    origin: {uf: '', ibge_code: ''}, destination: {uf: '', ibge_code: ''},
    passage: {
      from_code: '', from_name: '', to_code: '', to_name: '', boarding: '', validity: '',
      identified: variant === 'ta', passenger: {name: '', doc_type: '1', doc_number: ''},
    },
    buyer: {
      enabled: false, name: '', doc_kind: 'cpf', document: '', ie: '', street: '', number: '', complement: '',
      neighborhood: '', ibge_code: '', city: '', postal_code: '', uf: '',
    },
    trips: [emptyBpeTrip(variant)],
    components: [{type: '01', value: ''}],
    discount: '', discount_type: '', discount_desc: '', discount_code: '',
    payments: [{type: '01', value: '', desc: ''}],
    icms: {cst: '40', rate: ''},
    tax: {
      class_trib: '', donation: false, ibs_uf_rate: '', ibs_mun_rate: '', cbs_rate: '',
      devolution: {ibs_uf: '', ibs_mun: '', cbs: ''}, reversal: {ibs: '', cbs: ''},
      government: {enabled: false, entity: '', reducer: '0', operation: '', references: ''},
      simples: {enabled: false, ibs_rate: '', cbs_rate: ''}, total_taxes: '', fiscal_info: '',
    },
    iata_number: '', additional_info: '',
    purchase_type: 'normal', multiple_key: '',
    agency: {enabled: false, name: '', cnpj: '', address: emptyAgencyAddress()},
    substitution: {enabled: false, original_key: '', type: ''},
    board_pass: '',
  };
}

/** Chaves de DFe anteriores, uma por linha ou separadas por espaço ou vírgula (campo livre, validado pela regex da chave). */
export function govKeys(raw: string): string[] {
  return raw.split(/[\s,;]+/).map((k) => k.trim().toUpperCase()).filter((k) => k !== '');
}

/** Só as operações 2 e 3 levam chave de DFe anterior; o campo some nas outras, e o texto que sobrar nele não segue. */
function govRefs(g: BpeTicketForm['tax']['government']): string[] {
  return g.operation === '2' || g.operation === '3' ? govKeys(g.references) : [];
}

export function toTaTax(t: BpeTicketForm['tax'], year = new Date().getFullYear()): NonNullable<BpeTaEmit['tax']> {
  const cls = BPE_TA_CLASSES.find((c) => c.value === t.class_trib);
  const taxed = cls !== undefined && cls.cst !== BPE_TA_NO_TAX_CST;
  const dev = t.devolution;
  const hasDev = taxed && (dev.ibs_uf !== '' || dev.ibs_mun !== '' || dev.cbs !== '');
  return {
    class_trib: t.class_trib,
    donation: t.donation ? true : undefined,
    ibs_uf_rate: taxed && year >= BPE_IBS_RATE_FROM_YEAR ? orUndefined(t.ibs_uf_rate) : undefined,
    ibs_mun_rate: taxed && year >= BPE_IBS_RATE_FROM_YEAR ? orUndefined(t.ibs_mun_rate) : undefined,
    cbs_rate: taxed && year >= BPE_CBS_RATE_FROM_YEAR ? orUndefined(t.cbs_rate) : undefined,
    devolution: hasDev ? {ibs_uf: orUndefined(dev.ibs_uf), ibs_mun: orUndefined(dev.ibs_mun), cbs: orUndefined(dev.cbs)} : undefined,
    credit_reversal: cls?.reversal ? {ibs: t.reversal.ibs, cbs: t.reversal.cbs} : undefined,
    government: taxed && t.government.enabled
      ? {entity: t.government.entity, reducer: t.government.reducer, operation: t.government.operation, references: govRefs(t.government)}
      : undefined,
    simples: t.simples.enabled ? {ibs_rate: orUndefined(t.simples.ibs_rate), cbs_rate: orUndefined(t.simples.cbs_rate)} : undefined,
    total_taxes: orUndefined(t.total_taxes),
    fiscal_info: orUndefined(t.fiscal_info),
  };
}

function toTrip(t: BpeTripForm, ta: boolean): BpeTripIn {
  const common = {
    trip_type: t.trip_type, service_type: t.service_type, accommodation: t.accommodation, leg_type: t.leg_type,
    departure: toOffsetDateTime(t.departure),
    connection: t.leg_type === '3' && !blank(t.connection) ? toOffsetDateTime(t.connection) : undefined,
  };
  if (ta) {
    return {
      ...common, flight_number: t.flight_number.trim(), carrier: t.carrier.trim().toUpperCase(),
      from_airport: t.from_airport.trim().toUpperCase(), to_airport: t.to_airport.trim().toUpperCase(),
    };
  }
  return {
    ...common, route_code: t.route_code.trim(), route_name: t.route_name.trim(),
    prefix: orUndefined(t.prefix), seat: orUndefined(t.seat), platform: orUndefined(t.platform),
  };
}

function toAgency(f: BpeTicketForm): BpeAgencyIn | undefined {
  const a = f.agency;
  if (!a.enabled) return undefined;
  const address: BpeAddressIn = {
    street: a.address.street.trim(), number: a.address.number.trim(), neighborhood: a.address.neighborhood.trim(),
    ibge_code: a.address.ibge_code, city: a.address.city.trim(), uf: a.address.uf,
    postal_code: blank(a.address.postal_code) ? undefined : digits(a.address.postal_code),
  };
  return {name: a.name.trim(), cnpj: a.cnpj.replace(/[^0-9A-Za-z]/g, '').toUpperCase(), address};
}

function toSubstitution(f: BpeTicketForm): BpeSubstitutionIn | undefined {
  const s = f.substitution;
  return s.enabled ? {original_key: s.original_key.trim().toUpperCase(), type: s.type} : undefined;
}

/** Corpo de `POST /bpes` do bilhete comum ou do TA (`variant` escolhe o DTO da API). */
export function toBpeTicketBody(f: BpeTicketForm): BpeNormalEmit | BpeTaEmit {
  const ta = f.variant === 'ta';
  const p = f.passage;
  const linked = ta && f.purchase_type === 'linked';
  const passenger = p.identified && !(ta && f.purchase_type === 'multiple')
    ? {name: p.passenger.name.trim(), doc_type: p.passenger.doc_type, doc_number: p.passenger.doc_number.trim()}
    : undefined;
  const hasDiscount = Number(f.discount) > 0;
  const bu = f.buyer;
  const buyer: BpeBuyerIn | undefined = bu.enabled
    ? {
      name: bu.name.trim(),
      cnpj: bu.doc_kind === 'cnpj' ? bu.document.replace(/[^0-9A-Za-z]/g, '').toUpperCase() : undefined,
      cpf: bu.doc_kind === 'cpf' ? digits(bu.document) : undefined,
      foreign_id: bu.doc_kind === 'foreign' ? bu.document.trim() : undefined,
      ie: orUndefined(bu.ie),
      address: {
        street: bu.street.trim(), number: bu.number.trim(), complement: orUndefined(bu.complement),
        neighborhood: bu.neighborhood.trim(), ibge_code: bu.ibge_code, city: bu.city.trim(),
        postal_code: blank(bu.postal_code) ? undefined : digits(bu.postal_code), uf: bu.uf,
      },
    }
    : undefined;
  const base = {
    presence: f.presence,
    origin: f.origin,
    destination: f.destination,
    buyer,
    agency: toAgency(f),
    substitution: toSubstitution(f),
    // O vinculado a múltiplo não leva valores, imposto nem pagamento.
    components: linked ? undefined : f.components.map((c) => ({type: c.type, value: c.value})),
    discount: !linked && hasDiscount ? f.discount : undefined,
    discount_type: !linked && hasDiscount ? f.discount_type : undefined,
    discount_desc: !linked && hasDiscount ? f.discount_desc.trim() : undefined,
    discount_code: !linked && hasDiscount && f.discount_type === '99' ? f.discount_code.trim() : undefined,
    payments: linked ? undefined : f.payments.map((x) => ({type: x.type, value: x.value, desc: x.type === BPE_PAYMENT_OTHER ? x.desc.trim() : undefined})),
    additional_info: orUndefined(f.additional_info),
  };
  const trips = f.trips.map((t) => toTrip(t, ta));
  const boarding = toOffsetDateTime(p.boarding);
  const validity = toOffsetDateTime(p.validity);
  if (ta) {
    const t = f.tax;
    return {
      ...base, variant: 'ta', trips,
      purchase_type: f.purchase_type,
      multiple_key: linked ? f.multiple_key.trim().toUpperCase() : undefined,
      passage: {boarding, validity, passenger},
      tax: linked ? undefined : toTaTax(t),
      iata_number: orUndefined(f.iata_number),
    };
  }
  const icms: BpeIcmsIn = {cst: f.icms.cst, ...(BPE_ICMS_WITH_RATE.includes(f.icms.cst) ? {rate: f.icms.rate} : {})};
  return {
    ...base, variant: 'normal', modal: f.modal, trips, icms, board_pass: orUndefined(f.board_pass),
    passage: {
      from_code: p.from_code.trim(), from_name: p.from_name.trim(),
      to_code: p.to_code.trim(), to_name: p.to_name.trim(), boarding, validity, passenger,
    },
  };
}
