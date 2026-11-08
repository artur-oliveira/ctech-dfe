/**
 * Declaração de CIOT (ANTT, PEF). Espelha `CiotDeclareBody` (api/internal/api/v1/openapi/ciots.yaml).
 *
 * A api e o validador do go-dfe (`ciot.Validate`) são a autoridade e devolvem 422 `ciot_validation` com uma linha por
 * regra; aqui fica só o que poupa a ida e volta (formato, obrigatório por tipo de operação, limites). Escolhas
 * (tipo, carga, pagamento, veículos) vêm de seletores; texto livre só onde não há lista: RNTRC, peso, valor, chave Pix.
 */
import {z} from 'zod';
import {
  CIOT_MAX_VEHICLES,
  CIOT_NATURE_DIGITS,
  CIOT_RNTRC_DIGITS,
  CIOT_RNTRC_MIN_DIGITS,
  MDFE_ACCESS_KEY_LENGTH,
  OPERATION_FRACIONADA,
  OPERATION_LOTACAO,
  PAYMENT_BANK_TYPES,
  PAYMENT_PIX,
} from '@/lib/constants/ciot';
import {validateCNPJ, validateCPF} from '@/lib/utils/validators';
import type {CiotDeclareBody, CiotOperation, CiotOut} from '@/lib/types/api';

const CPF_LENGTH = 11;
const CNPJ_LENGTH = 14;
const MAX_WEIGHT = 9_999_999.99;
const MIN_AUTOMOTOR_AXLES = 2;
const MAX_AXLES = 4;
const MONEY_RE = /^\d+(\.\d{1,2})?$/;
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/;

/** CPF (11) ou CNPJ (14) só com dígitos e dígitos verificadores corretos. */
export const isCpfCnpj = (doc: string): boolean =>
  (doc.length === CPF_LENGTH && validateCPF(doc)) || (doc.length === CNPJ_LENGTH && validateCNPJ(doc));

const docField = z.string().refine(isCpfCnpj, 'CPF ou CNPJ inválido');
const optionalDoc = z.string().refine((d) => d === '' || isCpfCnpj(d), 'CPF ou CNPJ inválido');

export const ciotVehicleSchema = z.object({
  plate: z.string().min(1),
  rntrc: z.string(),
  axles: z.string().regex(/^[1-4]$/, 'Escolha de 1 a 4 eixos'),
  automotor: z.boolean(),
  tractor: z.boolean(),
}).refine((v) => !v.automotor || Number(v.axles) >= MIN_AUTOMOTOR_AXLES,
  {path: ['axles'], message: `Veículo automotor tem de ${MIN_AUTOMOTOR_AXLES} a ${MAX_AXLES} eixos`});

export const ciotDeclareSchema = z.object({
  environment: z.enum(['prod', 'hom']),
  operation_type: z.enum(['1', '2', '3']),
  carrier_doc: docField,
  carrier_rntrc: z.string().refine(
    (r) => new RegExp(`^\\d{${CIOT_RNTRC_MIN_DIGITS},${CIOT_RNTRC_DIGITS}}$`).test(r),
    `RNTRC: ${CIOT_RNTRC_MIN_DIGITS} ou ${CIOT_RNTRC_DIGITS} dígitos`),
  hirer_doc: docField,
  recipient_doc: optionalDoc,
  freight_value: z.string().regex(MONEY_RE, 'Informe o valor do frete').refine((v) => Number(v) > 0, 'O frete deve ser maior que zero'),
  trip_start: z.string(),
  trip_end: z.string().regex(DATE_RE, 'Informe o fim da viagem'),
  vehicles: z.array(ciotVehicleSchema).min(1, 'Escolha ao menos um veículo').max(CIOT_MAX_VEHICLES, `Máximo ${CIOT_MAX_VEHICLES} veículos`),
  origin_ibge: z.string(),
  destination_ibge: z.string(),
  distance_km: z.string(),
  cargo_ncm: z.string(),
  cargo_weight: z.string(),
  cargo_type: z.string(),
  high_performance: z.boolean(),
  empty_return: z.boolean(),
  vehicle_composition: z.boolean(),
  payment_type: z.enum(['1', '2', '3', '4', '5', '6']),
  credited_doc: docField,
  pix_key: z.string(),
  pix_id: z.string(),
  bank_code: z.string(),
  branch: z.string(),
  account: z.string(),
  mdfe_access_key: z.string().refine((k) => k === '' || new RegExp(`^\\d{${MDFE_ACCESS_KEY_LENGTH}}$`).test(k),
    `A chave do MDF-e tem ${MDFE_ACCESS_KEY_LENGTH} dígitos`),
}).superRefine((v, ctx) => {
  const issue = (path: string, message: string) => ctx.addIssue({code: 'custom', path: [path], message});
  const op = Number(v.operation_type);
  const hasRoute = op === OPERATION_LOTACAO || op === OPERATION_FRACIONADA;

  if (hasRoute) {
    if (!v.recipient_doc) issue('recipient_doc', 'Escolha o destinatário');
    if (!DATE_RE.test(v.trip_start)) issue('trip_start', 'Informe o início da viagem');
    if (!v.origin_ibge) issue('origin_ibge', 'Escolha o município de origem');
    if (!v.destination_ibge) issue('destination_ibge', 'Escolha o município de destino');
    if (!/^\d+$/.test(v.cargo_ncm) || v.cargo_ncm.length < CIOT_NATURE_DIGITS) issue('cargo_ncm', 'Escolha o NCM da carga');
    const weight = Number(v.cargo_weight);
    if (!MONEY_RE.test(v.cargo_weight) || weight <= 0 || weight >= MAX_WEIGHT) issue('cargo_weight', 'Peso maior que zero e menor que 9.999.999,99');
    if (!v.cargo_type) issue('cargo_type', 'Escolha o tipo de carga');
  }
  if (DATE_RE.test(v.trip_start) && DATE_RE.test(v.trip_end) && v.trip_end < v.trip_start) {
    issue('trip_end', 'O fim da viagem não pode ser antes do início');
  }
  if (Number(v.payment_type) === PAYMENT_PIX) {
    if (!v.pix_key.trim()) issue('pix_key', 'Informe a chave Pix');
    if (!v.pix_id.trim()) issue('pix_id', 'Informe o identificador do Pix');
  }
  if (PAYMENT_BANK_TYPES.includes(Number(v.payment_type)) && (!v.bank_code || !v.branch || !v.account)) {
    issue('account', 'Informe banco, agência e conta');
  }
});

export type CiotDeclareFormData = z.infer<typeof ciotDeclareSchema>

export const emptyCiotForm = (environment: 'prod' | 'hom'): CiotDeclareFormData => ({
  environment,
  operation_type: '1', // OPERATION_LOTACAO
  carrier_doc: '', carrier_rntrc: '', hirer_doc: '', recipient_doc: '',
  freight_value: '', trip_start: '', trip_end: '',
  vehicles: [],
  origin_ibge: '', destination_ibge: '', distance_km: '',
  cargo_ncm: '', cargo_weight: '', cargo_type: '',
  high_performance: false, empty_return: false, vehicle_composition: false,
  payment_type: '2', credited_doc: '', pix_key: '', pix_id: '', bank_code: '', branch: '', account: '',
  mdfe_access_key: '',
});

/** Conteúdo da declaração a partir do formulário já validado; campos que não valem para o tipo escolhido não saem. */
export function toOperation(v: CiotDeclareFormData): CiotOperation {
  const op = Number(v.operation_type);
  const hasRoute = op === OPERATION_LOTACAO || op === OPERATION_FRACIONADA;
  const pix = Number(v.payment_type) === PAYMENT_PIX;
  const bank = PAYMENT_BANK_TYPES.includes(Number(v.payment_type));
  return {
    operation_type: op,
    carrier_doc: v.carrier_doc,
    carrier_rntrc: v.carrier_rntrc,
    hirer_doc: v.hirer_doc,
    ...(hasRoute && {recipient_doc: v.recipient_doc, trip_start: v.trip_start}),
    freight_value: v.freight_value,
    trip_end: v.trip_end,
    vehicles: v.vehicles.map((x) => ({
      plate: x.plate, axles: Number(x.axles), automotor: x.automotor, tractor: x.tractor,
      ...(x.rntrc && {rntrc: x.rntrc}),
    })),
    ...(hasRoute && {
      routes: [{
        origin_ibge: v.origin_ibge, destination_ibge: v.destination_ibge,
        ...(v.distance_km && {distance_km: Number(v.distance_km)}),
      }],
      cargo: {nature: v.cargo_ncm.slice(0, CIOT_NATURE_DIGITS), weight: v.cargo_weight, cargo_type: Number(v.cargo_type)},
    }),
    ...(op === OPERATION_LOTACAO && {
      indicators: {high_performance: v.high_performance, empty_return: v.empty_return, vehicle_composition: v.vehicle_composition},
    }),
    payment: {
      type: Number(v.payment_type),
      indicator: 0,
      credited_doc: v.credited_doc,
      ...(pix && {pix_key: v.pix_key.trim(), pix_id: v.pix_id.trim()}),
      ...(bank && {bank_code: v.bank_code, branch: v.branch, account: v.account}),
    },
  };
}

/** Corpo do POST /ciots. */
export function toDeclareBody(v: CiotDeclareFormData): CiotDeclareBody {
  return {
    environment: v.environment,
    ...toOperation(v),
    ...(v.mdfe_access_key && {mdfe_access_key: v.mdfe_access_key}),
  };
}

/** Formulário preenchido com a declaração gravada, para retificar. A natureza guarda só 4 dígitos (o NCM completo não vai à ANTT). */
export function formFromCiot(c: CiotOut): CiotDeclareFormData {
  const d = c.declaration ?? {};
  const route = d.routes?.[0];
  const pay = d.payment;
  const base = emptyCiotForm(c.environment);
  return {
    ...base,
    operation_type: String(d.operation_type ?? c.operation_type ?? 1) as CiotDeclareFormData['operation_type'],
    carrier_doc: d.carrier_doc ?? c.carrier_doc ?? '',
    carrier_rntrc: (d.carrier_rntrc ?? '').replace(/\D/g, ''),
    hirer_doc: d.hirer_doc ?? c.hirer_doc ?? '',
    recipient_doc: d.recipient_doc ?? '',
    freight_value: d.freight_value ?? c.freight_value ?? '',
    trip_start: (d.trip_start ?? c.trip_start ?? '').slice(0, 10),
    trip_end: (d.trip_end ?? c.trip_end ?? '').slice(0, 10),
    vehicles: (d.vehicles ?? []).map((x) => ({
      plate: x.plate, rntrc: x.rntrc ?? '', axles: String(x.axles), automotor: x.automotor, tractor: x.tractor,
    })),
    origin_ibge: route?.origin_ibge ?? '',
    destination_ibge: route?.destination_ibge ?? '',
    distance_km: route?.distance_km ? String(route.distance_km) : '',
    cargo_ncm: d.cargo?.nature ?? '',
    cargo_weight: d.cargo?.weight ?? '',
    cargo_type: d.cargo?.cargo_type ? String(d.cargo.cargo_type) : '',
    high_performance: !!d.indicators?.high_performance,
    empty_return: !!d.indicators?.empty_return,
    vehicle_composition: !!d.indicators?.vehicle_composition,
    payment_type: String(pay?.type ?? 2) as CiotDeclareFormData['payment_type'],
    credited_doc: pay?.credited_doc ?? '',
    pix_key: pay?.pix_key ?? '',
    pix_id: pay?.pix_id ?? '',
    bank_code: pay?.bank_code ?? '',
    branch: pay?.branch ?? '',
    account: pay?.account ?? '',
    mdfe_access_key: c.mdfe_access_key ?? '',
  };
}
