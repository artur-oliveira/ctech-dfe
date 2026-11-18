import {z} from 'zod';
import {BPE_BOARD_PASS_MAX, BPE_BOARD_PASS_MIN, BPE_KEY_PATTERN, BPE_PAYMENT_OTHER} from '@/lib/constants/bpe';
import {cleanOptional, cleanText, FISCAL_TEXT, FISCAL_TEXT_MESSAGE} from '@/lib/utils/fiscal-text';
import type {BpeTicketEmit} from '@/lib/types/api';

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

const tripSchema = z.object({
  line_id: z.string().min(1, 'Selecione a linha'),
  leg_type: z.string().min(1, 'Selecione o trecho'),
  departure: z.string().min(1, 'Informe a partida'),
  connection: z.string(),
  seat: z.string().refine((v) => blank(v) || /^\d{1,3}$/.test(v), 'Poltrona com até 3 dígitos'),
  platform: z.string().refine((v) => blank(v) || /^[A-Za-z0-9._/-]{1,20}$/.test(v.trim()), 'Use só letras, números, ponto, barra, hífen e sublinhado (até 20)'),
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

/** Passageiro: avulso (nome e documento direto, o caminho principal), pessoa do cadastro ou, no que não exige, nenhum. */
export const PASSENGER_MODES = [
  {value: 'avulso', label: 'Passageiro avulso (nome e documento)'},
  {value: 'person', label: 'Pessoa do cadastro'},
  {value: 'none', label: 'Não identificar'},
] as const;

export const bpeTicketSchema = z.object({
  variant: z.enum(['normal', 'ta']),
  presence: z.string().min(1, 'Selecione a presença'),
  trips: z.array(tripSchema).min(1, 'Informe ao menos um trecho').max(20),
  boarding: z.string().min(1, 'Informe o embarque'),
  validity: z.string().min(1, 'Informe a validade'),
  passenger: z.object({
    mode: z.enum(['avulso', 'person', 'none']),
    name: z.string(),
    doc_type: z.string(),
    doc_number: z.string().max(20),
    person_id: z.string(),
  }),
  buyer_person_id: z.string(),
  agency_person_id: z.string(),
  purchase_type: z.enum(['normal', 'multiple', 'linked']),
  multiple_key: z.string(),
  iata_number: z.string().refine((v) => blank(v) || /^[A-Za-z0-9._/-]{1,20}$/.test(v.trim()), 'Use só letras, números, ponto, barra, hífen e sublinhado (até 20)'),
  substitution: z.object({enabled: z.boolean(), original_key: z.string(), type: z.string()}),
  components: z.array(componentSchema).max(20),
  discount: z.string(),
  discount_type: z.string(),
  discount_desc: z.string().max(100),
  discount_code: z.string().refine((v) => blank(v) || /^[A-Za-z0-9._/-]{1,20}$/.test(v.trim()), 'Use só letras, números, ponto, barra, hífen e sublinhado (até 20)'),
  payments: z.array(paymentSchema).max(10),
  board_pass: z.string(),
  additional_info: z.string().max(5000, 'Até 5000 caracteres'),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const ta = f.variant === 'ta';
  const linked = ta && f.purchase_type === 'linked';
  const multiple = ta && f.purchase_type === 'multiple';
  if (multiple && blank(f.buyer_person_id)) issue(['buyer_person_id'], 'O BP-e múltiplo exige o comprador');
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
    const desc = cleanText(f.discount_desc);
    if (desc.length < 2) issue(['discount_desc'], 'Descreva o desconto (mínimo 2 caracteres)');
    else if (!FISCAL_TEXT.test(desc)) issue(['discount_desc'], FISCAL_TEXT_MESSAGE);
  }
  if (f.discount_type === '99' && blank(f.discount_code)) issue(['discount_code'], 'Informe o código do desconto');
  f.payments.forEach((p, i) => {
    if (p.type === BPE_PAYMENT_OTHER && p.desc.trim().length < 2) issue(['payments', i, 'desc'], 'Descreva a forma de pagamento');
  });
  if (f.validity && f.boarding && f.validity < f.boarding) issue(['validity'], 'A validade não pode ser anterior ao embarque');
  const info = cleanText(f.additional_info);
  if (info !== '' && !FISCAL_TEXT.test(info)) issue(['additional_info'], FISCAL_TEXT_MESSAGE);

  const pax = f.passenger;
  if (multiple && pax.mode !== 'none') issue(['passenger', 'mode'], 'O BP-e múltiplo não identifica passageiro');
  if (ta && !multiple && pax.mode === 'none') issue(['passenger', 'mode'], 'O TA exige a identificação do passageiro');
  if (pax.mode === 'avulso') {
    const name = cleanText(pax.name);
    if (name.length < 2) issue(['passenger', 'name'], 'Informe o nome do passageiro');
    else if (name.length > 60 || !FISCAL_TEXT.test(name)) issue(['passenger', 'name'], name.length > 60 ? 'Até 60 caracteres' : FISCAL_TEXT_MESSAGE);
    if (blank(pax.doc_type)) issue(['passenger', 'doc_type'], 'Selecione o documento');
    const doc = cleanText(pax.doc_number);
    if (doc === '') issue(['passenger', 'doc_number'], 'Informe o número do documento');
    else if (!FISCAL_TEXT.test(doc)) issue(['passenger', 'doc_number'], FISCAL_TEXT_MESSAGE);
  }
  if (pax.mode === 'person' && blank(pax.person_id)) issue(['passenger', 'person_id'], 'Selecione a pessoa');
  f.trips.forEach((t, i) => {
    if (t.leg_type === '3' && blank(t.connection)) issue(['trips', i, 'connection'], 'Informe a conexão');
  });
});

export type BpeTicketForm = z.infer<typeof bpeTicketSchema>
export type BpeTripForm = BpeTicketForm['trips'][number]

export function emptyBpeTrip(): BpeTripForm {
  return {line_id: '', leg_type: '1', departure: '', connection: '', seat: '', platform: ''};
}

export function emptyBpeTicket(variant: 'normal' | 'ta' = 'normal'): BpeTicketForm {
  return {
    variant, presence: '1', trips: [emptyBpeTrip()], boarding: '', validity: '',
    passenger: {mode: variant === 'ta' ? 'avulso' : 'none', name: '', doc_type: '1', doc_number: '', person_id: ''},
    buyer_person_id: '', agency_person_id: '', purchase_type: 'normal', multiple_key: '', iata_number: '',
    substitution: {enabled: false, original_key: '', type: ''},
    components: [{type: '01', value: ''}],
    discount: '', discount_type: '', discount_desc: '', discount_code: '',
    payments: [{type: '01', value: '', desc: ''}],
    board_pass: '', additional_info: '',
  };
}

/** Corpo de `POST /bpes` do bilhete comum ou do TA: linhas e pessoas por id, mais o que varia na venda. */
export function toBpeTicketBody(f: BpeTicketForm): BpeTicketEmit {
  const ta = f.variant === 'ta';
  const linked = ta && f.purchase_type === 'linked';
  const pax = f.passenger;
  const hasDiscount = Number(f.discount) > 0;
  return {
    variant: f.variant,
    presence: f.presence,
    trips: f.trips.map((t) => ({
      line_id: t.line_id, leg_type: t.leg_type, departure: toOffsetDateTime(t.departure),
      connection: t.leg_type === '3' && !blank(t.connection) ? toOffsetDateTime(t.connection) : undefined,
      seat: ta ? undefined : orUndefined(t.seat), platform: ta ? undefined : orUndefined(t.platform),
    })),
    boarding: toOffsetDateTime(f.boarding),
    validity: toOffsetDateTime(f.validity),
    passenger: pax.mode === 'avulso' ? {name: cleanText(pax.name), doc_type: pax.doc_type, doc_number: cleanText(pax.doc_number)} : undefined,
    passenger_person_id: pax.mode === 'person' ? pax.person_id : undefined,
    buyer_person_id: orUndefined(f.buyer_person_id),
    agency_person_id: orUndefined(f.agency_person_id),
    purchase_type: ta ? f.purchase_type : undefined,
    multiple_key: linked ? f.multiple_key.trim().toUpperCase() : undefined,
    iata_number: ta ? orUndefined(f.iata_number) : undefined,
    substitution: f.substitution.enabled ? {original_key: f.substitution.original_key.trim().toUpperCase(), type: f.substitution.type} : undefined,
    // O vinculado a múltiplo não leva valores nem pagamento.
    components: linked ? undefined : f.components.map((c) => ({type: c.type, value: c.value})),
    discount: !linked && hasDiscount ? f.discount : undefined,
    discount_type: !linked && hasDiscount ? f.discount_type : undefined,
    discount_desc: !linked && hasDiscount ? cleanText(f.discount_desc) : undefined,
    discount_code: !linked && hasDiscount && f.discount_type === '99' ? f.discount_code.trim() : undefined,
    payments: linked ? undefined : f.payments.map((x) => ({type: x.type, value: x.value, desc: x.type === BPE_PAYMENT_OTHER ? cleanText(x.desc) : undefined})),
    board_pass: ta ? undefined : orUndefined(f.board_pass),
    additional_info: cleanOptional(f.additional_info),
  };
}
