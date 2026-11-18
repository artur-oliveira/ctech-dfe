import {z} from 'zod';
import {BPE_ICMS_WITH_RATE, BPE_TM_COMPONENT_NAME_MAX, BPE_TM_ITEMS_MAX} from '@/lib/constants/bpe';
import {cleanText, FISCAL_TEXT, FISCAL_TEXT_MESSAGE, REF_CODE, REF_CODE_MESSAGE} from '@/lib/utils/fiscal-text';
import {emptyTaTax, taTaxFromLine, taTaxIssues, taTaxSchema, toTaTax} from '@/lib/schemas/bpe-tax';
import type {BpeIcmsIn, BpeLineCreate, BpeLineItemOut, BpeLinePlaceIn} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const nullIfBlank = (s: string): string | null => (blank(s) ? null : s.trim());

const fiscalText = (label: string, min: number, max: number) =>
  z.string().transform(cleanText).pipe(z.string().min(min, `Informe ${label}`).max(max, `Até ${max} caracteres`).regex(FISCAL_TEXT, FISCAL_TEXT_MESSAGE));
const optionalRef = (max: number) => z.string().refine((v) => blank(v) || (REF_CODE.test(v.trim()) && v.trim().length <= max), REF_CODE_MESSAGE);

const placeFields = z.object({uf: z.string(), ibge_code: z.string(), city: z.string()});
const legSchema = z.object({
  origin_ibge_code: z.string(),
  dest_ibge_code: z.string(),
  icms_cst: z.string(),
  icms_rate: z.string(),
});

export const bpeLineSchema = z.object({
  name: fiscalText('o nome', 2, 60),
  variant: z.enum(['normal', 'ta', 'tm']),
  modal: z.string(),
  origin: placeFields,
  destination: placeFields,
  route_code: optionalRef(20),
  route_name: z.string().transform(cleanText).pipe(z.string().max(100, 'Até 100 caracteres').refine((v) => v === '' || FISCAL_TEXT.test(v), FISCAL_TEXT_MESSAGE)),
  prefix: optionalRef(20),
  trip_type: z.string(),
  service_type: z.string(),
  accommodation: z.string(),
  icms_cst: z.string(),
  icms_rate: z.string(),
  carrier: z.string().transform((v) => v.trim().toUpperCase()).pipe(z.string().refine((v) => v === '' || /^[A-Z0-9]{2,3}$/.test(v), 'Sigla com 2 ou 3 caracteres')),
  flight_number: optionalRef(20),
  from_airport: z.string().transform((v) => v.trim().toUpperCase()).pipe(z.string().refine((v) => v === '' || /^[A-Z0-9]{3,4}$/.test(v), 'Aeroporto com 3 ou 4 caracteres')),
  to_airport: z.string().transform((v) => v.trim().toUpperCase()).pipe(z.string().refine((v) => v === '' || /^[A-Z0-9]{3,4}$/.test(v), 'Aeroporto com 3 ou 4 caracteres')),
  tax: taTaxSchema,
  cfop: z.string(),
  plate: z.string().transform((v) => v.trim().toUpperCase()).pipe(z.string().refine((v) => v === '' || /^[A-Z]{3}[0-9][A-Z0-9][0-9]{2}$/.test(v), 'Placa inválida')),
  group_origin_uf: z.string(),
  group_dest_uf: z.string(),
  legs: z.array(legSchema).max(BPE_TM_ITEMS_MAX),
  passenger_types: z.array(z.object({name: z.string().transform(cleanText)})).max(20),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const need = (path: string, value: string, message: string) => { if (blank(value)) issue([path], message); };
  const icms = (cst: string, rate: string, path: (string | number)[]) => {
    if (BPE_ICMS_WITH_RATE.includes(cst) && !decimal.test(rate)) issue(path, 'Informe a alíquota do ICMS');
  };
  if (f.variant !== 'tm') {
    if (blank(f.origin.ibge_code)) issue(['origin', 'ibge_code'], 'Selecione o município de início');
    if (blank(f.destination.ibge_code)) issue(['destination', 'ibge_code'], 'Selecione o município de fim');
    need('trip_type', f.trip_type, 'Selecione o tipo de viagem');
    need('service_type', f.service_type, 'Selecione o serviço');
    need('accommodation', f.accommodation, 'Selecione a acomodação');
  }
  if (f.variant === 'normal') {
    need('modal', f.modal, 'Selecione o modal');
    need('route_code', f.route_code, 'Informe o código da linha');
    need('route_name', f.route_name, 'Informe o percurso');
    need('icms_cst', f.icms_cst, 'Selecione o ICMS');
    icms(f.icms_cst, f.icms_rate, ['icms_rate']);
  } else if (f.variant === 'ta') {
    need('carrier', f.carrier, 'Informe a companhia');
    need('flight_number', f.flight_number, 'Informe o número do voo');
    need('from_airport', f.from_airport, 'Informe o aeroporto de origem');
    need('to_airport', f.to_airport, 'Informe o aeroporto de destino');
    taTaxIssues(f.tax, (path, message) => issue(['tax', ...path], message));
  } else {
    need('modal', f.modal, 'Selecione o modal');
    if (!/^[123567]\d([0-9][1-9]|[1-9][0-9])$/.test(f.cfop)) issue(['cfop'], 'CFOP inválido (4 dígitos; os dois últimos não são 00)');
    need('group_origin_uf', f.group_origin_uf, 'Selecione a UF de início');
    if (f.legs.length < 1) issue(['legs'], 'Informe ao menos um trecho');
    if (f.passenger_types.length < 1) issue(['passenger_types'], 'Informe ao menos um tipo de passageiro');
    f.legs.forEach((l, i) => {
      if (!/^\d{7}$/.test(l.origin_ibge_code)) issue(['legs', i, 'origin_ibge_code'], 'Selecione o município');
      if (blank(l.icms_cst)) issue(['legs', i, 'icms_cst'], 'Selecione o ICMS');
      icms(l.icms_cst, l.icms_rate, ['legs', i, 'icms_rate']);
    });
    f.passenger_types.forEach((p, i) => {
      if (p.name === '') issue(['passenger_types', i, 'name'], 'Informe o nome');
      else if (p.name.length > BPE_TM_COMPONENT_NAME_MAX) issue(['passenger_types', i, 'name'], `Até ${BPE_TM_COMPONENT_NAME_MAX} caracteres`);
      else if (!FISCAL_TEXT.test(p.name)) issue(['passenger_types', i, 'name'], FISCAL_TEXT_MESSAGE);
    });
  }
});

export type BpeLineFormData = z.input<typeof bpeLineSchema>
export type BpeLineOutput = z.output<typeof bpeLineSchema>

const emptyPlace = () => ({uf: '', ibge_code: '', city: ''});

export function emptyBpeLine(variant: BpeLineFormData['variant'] = 'normal'): BpeLineFormData {
  const ta = variant === 'ta';
  return {
    name: '', variant, modal: ta ? '' : 'rodoviario', origin: emptyPlace(), destination: emptyPlace(),
    route_code: '', route_name: '', prefix: '', trip_type: '00', service_type: ta ? '12' : '1', accommodation: ta ? '6' : '1',
    icms_cst: '40', icms_rate: '', carrier: '', flight_number: '', from_airport: '', to_airport: '', tax: emptyTaTax(),
    cfop: '', plate: '', group_origin_uf: '', group_dest_uf: '',
    legs: [{origin_ibge_code: '', dest_ibge_code: '', icms_cst: '40', icms_rate: ''}],
    passenger_types: [{name: ''}],
  };
}

const str = (v: unknown): string => (typeof v === 'string' ? v : '');
const place = (p: BpeLinePlaceIn | null | undefined) => ({uf: p?.uf ?? '', ibge_code: p?.ibge_code ?? '', city: p?.city ?? ''});

/** Linha gravada para o formulário de edição. */
export function bpeLineToForm(l: BpeLineItemOut): BpeLineFormData {
  const e = emptyBpeLine(l.variant);
  return {
    ...e,
    name: str(l.name), variant: l.variant, modal: str(l.modal), origin: place(l.origin), destination: place(l.destination),
    route_code: str(l.route_code), route_name: str(l.route_name), prefix: str(l.prefix),
    trip_type: str(l.trip_type) || e.trip_type, service_type: str(l.service_type) || e.service_type, accommodation: str(l.accommodation) || e.accommodation,
    icms_cst: str(l.icms?.cst) || e.icms_cst, icms_rate: str(l.icms?.rate),
    carrier: str(l.carrier), flight_number: str(l.flight_number), from_airport: str(l.from_airport), to_airport: str(l.to_airport),
    tax: taTaxFromLine(l.tax), cfop: str(l.cfop), plate: str(l.plate), group_origin_uf: str(l.origin_uf), group_dest_uf: str(l.dest_uf),
    legs: l.legs && l.legs.length > 0
      ? l.legs.map((g) => ({origin_ibge_code: g.origin_ibge_code, dest_ibge_code: str(g.dest_ibge_code), icms_cst: g.icms.cst, icms_rate: str(g.icms.rate)}))
      : e.legs,
    passenger_types: l.passenger_types && l.passenger_types.length > 0 ? l.passenger_types.map((name) => ({name})) : e.passenger_types,
  };
}

const icmsOf = (cst: string, rate: string): BpeIcmsIn => ({cst, rate: BPE_ICMS_WITH_RATE.includes(cst) ? rate : null});

/** Corpo de `POST /bpe-lines`: só os campos da variante; o resto fica de fora. */
export function toBpeLine(f: BpeLineOutput): BpeLineCreate {
  if (f.variant === 'tm') {
    return {
      name: f.name, variant: 'tm', modal: f.modal, cfop: f.cfop, plate: nullIfBlank(f.plate), prefix: nullIfBlank(f.prefix),
      origin_uf: f.group_origin_uf, dest_uf: nullIfBlank(f.group_dest_uf),
      legs: f.legs.map((l) => ({origin_ibge_code: l.origin_ibge_code, dest_ibge_code: nullIfBlank(l.dest_ibge_code), icms: icmsOf(l.icms_cst, l.icms_rate)})),
      passenger_types: f.passenger_types.map((p) => p.name),
    };
  }
  const common = {
    name: f.name, variant: f.variant, origin: f.origin, destination: f.destination,
    trip_type: f.trip_type, service_type: f.service_type, accommodation: f.accommodation,
  };
  if (f.variant === 'ta') {
    return {
      ...common, carrier: f.carrier, flight_number: f.flight_number.trim(), from_airport: f.from_airport, to_airport: f.to_airport,
      tax: toTaTax(f.tax),
    };
  }
  return {
    ...common, modal: f.modal, route_code: f.route_code.trim(), route_name: f.route_name, prefix: nullIfBlank(f.prefix),
    icms: icmsOf(f.icms_cst, f.icms_rate),
  };
}
