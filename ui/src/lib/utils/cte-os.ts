import {
  CTE_OS_CHARTER_EVENTUAL,
  CTE_OS_GTVE_COMPONENT_OTHERS,
  CTE_OS_GTVE_TOLERANCE_CENTS,
  CTE_OS_FEDERAL_TAXES,
  CTE_OS_STATE_REGISTRATION_LENGTH,
  CTE_OS_TAF_LENGTH,
  CTE_ACCESS_KEY_LENGTH,
  type CteOsFederalTaxKey,
} from '@/lib/constants/cte';
import {personTaxId} from '@/lib/utils/document';
import {cityPlace, chargeMissingItems, buildChargeBody, fromCents, toCents, type CteChargeState, type StepIssue} from '@/lib/utils/cte';
import type {CteOsDocRef, CteOsEmitBody, CteOsPlace, CteOsServiceType, PersonItemOut, VehicleOut} from '@/lib/types/api';

/** Estado da tela do CT-e OS (modelo 67). Contrato: `POST /v1.0/ctes/os`, OpenAPI `CteOsEmitBody`. */

export interface CteOsDocLine {
  id: string
  /** `bpe`: chave do BP-e; `other`: número, série e data. */
  mode: 'bpe' | 'other'
  bpe_key: string
  number: string
  serie: string
  subserie: string
  issued_on: string
  amount: string
}

export interface CteOsGtveComponentLine {
  id: string
  type: string
  value: string
  name: string
}

export interface CteOsGtveLine {
  id: string
  key: string
  components: CteOsGtveComponentLine[]
}

export type CteOsRegistrationKind = 'taf' | 'state_registration'

export interface CteOsState {
  serviceType: CteOsServiceType | null
  taker: PersonItemOut | null
  description: string
  quantity: string
  /** Código IBGE do município; vazio = não informado. */
  origin: string
  destination: string
  routeUfs: string[]
  registrationKind: CteOsRegistrationKind | null
  taf: string
  stateRegistration: string
  vehicle: VehicleOut | null
  charterType: '1' | '2' | ''
  /** Valor do `datetime-local` (hora local, sem fuso). */
  tripAt: string
  docRefs: CteOsDocLine[]
  gtve: CteOsGtveLine[]
  federal: Record<CteOsFederalTaxKey, string>
}

export const emptyCteOs = (): CteOsState => ({
  serviceType: null, taker: null, description: '', quantity: '', origin: '', destination: '', routeUfs: [],
  registrationKind: null, taf: '', stateRegistration: '', vehicle: null, charterType: '', tripAt: '',
  docRefs: [], gtve: [], federal: {pis: '', cofins: '', ir: '', inss: '', csll: ''},
});

export const newOsId = () => `os-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
export const emptyDocLine = (): CteOsDocLine => ({
  id: newOsId(), mode: 'bpe', bpe_key: '', number: '', serie: '', subserie: '', issued_on: '', amount: '',
});
export const emptyGtveComponent = (): CteOsGtveComponentLine => ({id: newOsId(), type: '', value: '', name: ''});
export const emptyGtve = (): CteOsGtveLine => ({id: newOsId(), key: '', components: [emptyGtveComponent()]});

/** O que cada tipo de serviço leva: modal rodoviário, fretamento, documentos referenciados, GTV-e. */
export const osHasRoad = (t: CteOsServiceType | null) => t === 'people' || t === 'baggage';
export const osHasCharter = (t: CteOsServiceType | null) => t === 'people';
export const osNeedsOrigin = (t: CteOsServiceType | null) => t === 'people';
export const osNeedsDestination = (t: CteOsServiceType | null) => t === 'people' || t === 'values';

/**
 * Pessoas: dentro da mesma UF vale o registro estadual (H023), entre UFs o TAF (H024). Sem origem e destino, ou
 * com `EX`, e nos demais serviços, a escolha é do usuário (`null`).
 */
export function osRegistrationKind(
  type: CteOsServiceType | null, origin: CteOsPlace | null, destination: CteOsPlace | null,
): CteOsRegistrationKind | null {
  if (type !== 'people' || !origin || !destination || origin.uf === 'EX' || destination.uf === 'EX') return null;
  return origin.uf === destination.uf ? 'state_registration' : 'taf';
}

/** Município do picker no formato da api (o servidor não tem tabela de nomes). */
export function osPlace(ibge: string): CteOsPlace | null {
  const p = cityPlace(ibge);
  return p ? {city_ibge: p.city_ibge, city: p.city.slice(0, 60), uf: p.uf} : null;
}

/** INSS é obrigatório em pessoas com tomador pessoa jurídica (H057). */
export const osInssRequired = (s: Pick<CteOsState, 'serviceType' | 'taker'>) =>
  s.serviceType === 'people' && !!s.taker && personTaxId(s.taker).length === 14;

export const gtveComponentsCents = (g: CteOsGtveLine) => g.components.reduce((sum, c) => sum + toCents(c.value), 0);

/** H053: a soma dos componentes de todas as GTV-e fecha com o valor do frete, com tolerância de R$ 0,10. */
export const gtveTotalCents = (lines: CteOsGtveLine[]) => lines.reduce((sum, g) => sum + gtveComponentsCents(g), 0);
export const gtveTotalCloses = (lines: CteOsGtveLine[], freightTotal: string) =>
  Math.abs(gtveTotalCents(lines) - toCents(freightTotal)) <= CTE_OS_GTVE_TOLERANCE_CENTS;

const isKey = (v: string) => new RegExp(`^\\d{${CTE_ACCESS_KEY_LENGTH}}$`).test(v);

/** Data e hora local do `datetime-local` em RFC 3339 com fuso, sem milissegundos. */
export const tripAtToIso = (local: string): string => new Date(local).toISOString().replace(/\.\d{3}Z$/, 'Z');

/** Passos do `CteOsForm` (o de dados do tipo é `road` em pessoas e bagagem, `docs` em bagagem e valores). */
export const OS_STEP_SERVICE = 'service';
export const OS_STEP_ROUTE = 'route';
export const OS_STEP_ROAD = 'road';
export const OS_STEP_DOCS = 'docs';
export const OS_STEP_CHARGE = 'charge';

/** O que falta para emitir, cada pendência com o passo dono, na ordem de preenchimento. */
export function cteOsMissingItems(s: CteOsState, charge: CteChargeState): StepIssue[] {
  const t = s.serviceType;
  if (!t) return [{step: OS_STEP_SERVICE, text: 'Escolha o tipo de serviço', field: 'cte-os-service-type'}];
  const m: StepIssue[] = [];
  const add = (step: string, text: string, field?: string) => m.push({step, text, field});
  if (!s.taker) add(OS_STEP_ROUTE, 'Escolha o tomador do serviço');
  if (!s.description.trim()) add(OS_STEP_SERVICE, 'Descreva o serviço', 'cte-os-desc');
  if (osNeedsOrigin(t) && !s.origin) add(OS_STEP_ROUTE, 'Escolha a origem', 'cte-os-origin');
  if (osNeedsDestination(t) && !s.destination) add(OS_STEP_ROUTE, 'Escolha o destino', 'cte-os-destination');

  if (osHasRoad(t)) {
    const kind = s.registrationKind;
    if (!kind) add(OS_STEP_ROAD, 'Escolha entre TAF e registro estadual', 'cte-os-reg-kind');
    else if (kind === 'taf' && s.taf.length !== CTE_OS_TAF_LENGTH) add(OS_STEP_ROAD, `Informe o TAF com ${CTE_OS_TAF_LENGTH} dígitos`, 'cte-os-taf');
    else if (kind === 'state_registration' && s.stateRegistration.length !== CTE_OS_STATE_REGISTRATION_LENGTH) {
      add(OS_STEP_ROAD, `Informe o registro estadual com ${CTE_OS_STATE_REGISTRATION_LENGTH} dígitos`);
    }
  }
  if (osHasCharter(t)) {
    if (!s.charterType) add(OS_STEP_ROAD, 'Escolha o tipo de fretamento', 'cte-os-charter');
    else if (s.charterType === CTE_OS_CHARTER_EVENTUAL && !s.tripAt) add(OS_STEP_ROAD, 'Informe a data e a hora da viagem');
  }
  if (t === 'baggage') {
    if (s.docRefs.length === 0) add(OS_STEP_DOCS, 'Informe ao menos um documento referenciado');
    else if (s.docRefs.some((d) => (d.mode === 'bpe' ? !isKey(d.bpe_key) : !d.number || !d.issued_on))) {
      add(OS_STEP_DOCS, 'Complete os documentos referenciados');
    }
  }
  if (t === 'values') {
    if (s.gtve.length === 0) add(OS_STEP_DOCS, 'Informe ao menos uma GTV-e');
    else if (s.gtve.some((g) => !isKey(g.key) || g.components.length === 0
      || g.components.some((c) => !c.type || toCents(c.value) <= 0
        || (c.type === CTE_OS_GTVE_COMPONENT_OTHERS && !c.name.trim())))) {
      add(OS_STEP_DOCS, 'Complete as GTV-e e os componentes');
    }
  }
  m.push(...chargeMissingItems(charge, 'frete', OS_STEP_CHARGE));
  if (t === 'values' && s.gtve.length > 0 && toCents(charge.freightTotal) > 0 && !gtveTotalCloses(s.gtve, charge.freightTotal)) {
    add(OS_STEP_CHARGE, 'A soma das GTV-e precisa fechar com o frete', 'cte-freight-total');
  }
  if (osInssRequired(s) && toCents(s.federal.inss) <= 0) add(OS_STEP_CHARGE, 'Informe o INSS dos tributos federais', 'cte-os-fed-inss');
  return m;
}

/** O que falta para emitir, em linguagem de quem preenche. Vazio = pode emitir. */
export const cteOsMissing = (s: CteOsState, charge: CteChargeState): string[] =>
  cteOsMissingItems(s, charge).map((i) => i.text);

const docRefBody = (d: CteOsDocLine): CteOsDocRef => d.mode === 'bpe'
  ? {bpe_key: d.bpe_key}
  : {
    number: d.number.trim(), issued_on: d.issued_on,
    ...(d.serie ? {serie: d.serie.trim()} : {}), ...(d.subserie ? {subserie: d.subserie.trim()} : {}),
    ...(d.amount && toCents(d.amount) > 0 ? {amount: fromCents(toCents(d.amount))} : {}),
  };

/** Corpo do `POST /v1.0/ctes/os`: só o que o tipo de serviço aceita (a api recusa o que sobra). */
export function buildCteOsBody(s: CteOsState, charge: CteChargeState): CteOsEmitBody {
  if (!s.serviceType || !s.taker) throw new Error('CT-e OS incompleto: tipo de serviço e tomador são obrigatórios');
  const t = s.serviceType;
  const origin = osPlace(s.origin);
  const destination = osPlace(s.destination);
  const kind = s.registrationKind;
  const federal = Object.fromEntries(
    CTE_OS_FEDERAL_TAXES.filter((f) => toCents(s.federal[f.key]) > 0).map((f) => [f.key, fromCents(toCents(s.federal[f.key]))]),
  );
  const plate = s.vehicle?.plate.replace(/[^A-Za-z0-9]/g, '');
  return {
    service_type: t,
    taker: {person_id: s.taker.sk},
    service: {
      description: s.description.trim(),
      ...(s.quantity && Number(s.quantity) > 0 ? {quantity: s.quantity} : {}),
    },
    ...(origin ? {origin} : {}),
    ...(destination ? {destination} : {}),
    ...(s.routeUfs.length ? {route_ufs: s.routeUfs} : {}),
    ...(osHasRoad(t) ? {
      road: {
        ...(kind === 'taf' ? {taf: s.taf} : {state_registration: s.stateRegistration}),
        ...(plate ? {vehicle: {
          plate,
          ...(s.vehicle?.renavam && /^\d{9,11}$/.test(s.vehicle.renavam) ? {renavam: s.vehicle.renavam} : {}),
          ...(s.vehicle?.plate_uf ? {uf: s.vehicle.plate_uf} : {}),
        }} : {}),
        ...(osHasCharter(t) && s.charterType ? {
          charter: {
            type: s.charterType === '1' ? 1 as const : 2 as const,
            ...(s.charterType === CTE_OS_CHARTER_EVENTUAL ? {trip_at: tripAtToIso(s.tripAt)} : {}),
          },
        } : {}),
      },
    } : {}),
    ...(t === 'baggage' ? {doc_refs: s.docRefs.map(docRefBody)} : {}),
    ...(t === 'values' ? {
      gtve: s.gtve.map((g) => ({
        key: g.key,
        components: g.components.map((c) => ({
          type: Number(c.type), value: fromCents(toCents(c.value)),
          ...(c.type === CTE_OS_GTVE_COMPONENT_OTHERS ? {name: c.name.trim()} : {}),
        })),
      })),
    } : {}),
    ...(Object.keys(federal).length ? {federal_taxes: federal} : {}),
    ...buildChargeBody(charge),
  };
}
