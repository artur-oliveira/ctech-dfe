import {z} from 'zod';
import {BPE_ICMS_WITH_RATE, BPE_PAYMENT_OTHER} from '@/lib/constants/bpe';
import type {BpeIcmsIn, BpeNormalEmit, BpeTaEmit, BpeTripIn} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());
const optDecimal = (s: string) => blank(s) || decimal.test(s);

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
  trips: z.array(tripSchema).min(1, 'Informe ao menos um trecho').max(20),
  components: z.array(componentSchema).min(1, 'Informe ao menos um componente').max(20),
  discount: z.string(),
  discount_type: z.string(),
  discount_desc: z.string().max(100),
  discount_code: z.string().max(20),
  payments: z.array(paymentSchema).min(1, 'Informe ao menos um pagamento').max(10),
  icms: z.object({cst: z.string(), rate: z.string()}),
  tax: z.object({
    cst: z.string(),
    class_trib: z.string(),
    donation: z.boolean(),
    ibs_uf_rate: z.string(),
    ibs_mun_rate: z.string(),
    cbs_rate: z.string(),
  }),
  iata_number: z.string().max(20),
  additional_info: z.string().max(5000),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const ta = f.variant === 'ta';
  if (!ta && blank(f.modal)) issue(['modal'], 'Selecione o modal');
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
  if (f.passage.identified) {
    if (f.passage.passenger.name.trim().length < 2) issue(['passage', 'passenger', 'name'], 'Informe o nome do passageiro');
    if (blank(f.passage.passenger.doc_type)) issue(['passage', 'passenger', 'doc_type'], 'Selecione o documento');
    if (blank(f.passage.passenger.doc_number)) issue(['passage', 'passenger', 'doc_number'], 'Informe o número do documento');
  }
  if (ta) {
    if (!/^\d{3}$/.test(f.tax.cst)) issue(['tax', 'cst'], 'CST com 3 dígitos');
    if (!/^\d{6}$/.test(f.tax.class_trib)) issue(['tax', 'class_trib'], 'Classificação com 6 dígitos');
    (['ibs_uf_rate', 'ibs_mun_rate', 'cbs_rate'] as const).forEach((k) => {
      if (!optDecimal(f.tax[k])) issue(['tax', k], 'Alíquota inválida');
    });
  } else {
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
      identified: false, passenger: {name: '', doc_type: '1', doc_number: ''},
    },
    trips: [emptyBpeTrip(variant)],
    components: [{type: '01', value: ''}],
    discount: '', discount_type: '', discount_desc: '', discount_code: '',
    payments: [{type: '01', value: '', desc: ''}],
    icms: {cst: '40', rate: ''},
    tax: {cst: '', class_trib: '', donation: false, ibs_uf_rate: '', ibs_mun_rate: '', cbs_rate: ''},
    iata_number: '', additional_info: '',
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

/** Corpo de `POST /bpes` do bilhete comum ou do TA (`variant` escolhe o DTO da API). */
export function toBpeTicketBody(f: BpeTicketForm): BpeNormalEmit | BpeTaEmit {
  const ta = f.variant === 'ta';
  const p = f.passage;
  const passenger = p.identified
    ? {name: p.passenger.name.trim(), doc_type: p.passenger.doc_type, doc_number: p.passenger.doc_number.trim()}
    : undefined;
  const hasDiscount = Number(f.discount) > 0;
  const base = {
    presence: f.presence,
    origin: f.origin,
    destination: f.destination,
    components: f.components.map((c) => ({type: c.type, value: c.value})),
    discount: hasDiscount ? f.discount : undefined,
    discount_type: hasDiscount ? f.discount_type : undefined,
    discount_desc: hasDiscount ? f.discount_desc.trim() : undefined,
    discount_code: hasDiscount && f.discount_type === '99' ? f.discount_code.trim() : undefined,
    payments: f.payments.map((x) => ({type: x.type, value: x.value, desc: x.type === BPE_PAYMENT_OTHER ? x.desc.trim() : undefined})),
    additional_info: orUndefined(f.additional_info),
  };
  const trips = f.trips.map((t) => toTrip(t, ta));
  const boarding = toOffsetDateTime(p.boarding);
  const validity = toOffsetDateTime(p.validity);
  if (ta) {
    const t = f.tax;
    return {
      ...base, variant: 'ta', trips,
      passage: {boarding, validity, passenger},
      tax: {
        cst: t.cst, class_trib: t.class_trib, donation: t.donation ? true : undefined,
        ibs_uf_rate: orUndefined(t.ibs_uf_rate), ibs_mun_rate: orUndefined(t.ibs_mun_rate), cbs_rate: orUndefined(t.cbs_rate),
      },
      iata_number: orUndefined(f.iata_number),
    };
  }
  const icms: BpeIcmsIn = {cst: f.icms.cst, ...(BPE_ICMS_WITH_RATE.includes(f.icms.cst) ? {rate: f.icms.rate} : {})};
  return {
    ...base, variant: 'normal', modal: f.modal, trips, icms,
    passage: {
      from_code: p.from_code.trim(), from_name: p.from_name.trim(),
      to_code: p.to_code.trim(), to_name: p.to_name.trim(), boarding, validity, passenger,
    },
  };
}
