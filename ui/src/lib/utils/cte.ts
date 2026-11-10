import {CITIES} from '@/lib/data/cities';
import {ApiError} from '@/lib/api/client';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import type {EmitTarget} from '@/lib/api/emit-problem';
import {
  CTE_CFOP_SCOPE_INTER,
  CTE_DOC_ERRORS,
  CTE_ERROR_RNTRC_REQUIRED,
  CTE_RNTRC_CONFIG_HREF,
  CTE_CFOP_SCOPE_INTRA,
  CTE_CST_WITH_REDUCTION,
  CTE_CST_WITHOUT_RATE,
  CTE_MODEL_OS,
  CTE_SUBSTITUTE_LIMIT_DAYS,
  CTE_SUBSTITUTE_SEFAZ_DAYS,
  CTE_TAKER_NEEDS_PERSON,
  CTE_TOTAL_WITH_TAXES_FROM_YEAR,
  type CteKind,
} from '@/lib/constants/cte';
import {getCfopDescription, getCfopOptionsForCte, getCfopVariants, type CfopCode} from '@/lib/data/cfop';
import type {
  CteChargeBody,
  CteDeriveOut,
  CteEmitBody,
  CteEmitOverrides,
  CteIbsCbs,
  CteIcmsCst,
  CteListOut,
  CtePlace,
  CteTakerRole,
  PersonItemOut,
} from '@/lib/types/api';
import {formatCurrency} from '@/lib/utils/helpers';

/**
 * Estado da tela de emissão de CT-e e as funções puras que a ligam ao contrato
 * (docs/specs/2026-11-05-cte-emissao-rodoviaria.md). O `derive` é a fonte dos
 * valores; o estado guarda só o que o usuário decidiu ou editou por cima.
 */

export interface CteFreightLine {
  id: string
  name: string
  value: string
}

export interface CteIcmsState {
  cst: CteIcmsCst | ''
  p_icms: string
  p_red_bc: string
}

/** Campos de "Dados do transporte" que o usuário sobrescreveu. Chave ausente = vale o derivado. */
export interface CteOverrides {
  sender?: PersonItemOut
  recipient?: PersonItemOut
  origin?: string        // código IBGE do município
  destination?: string
  total_value?: string
  total_weight_kg?: string
  predominant_product?: string
}

export type CteOverrideField = keyof CteOverrides

export interface CteFormState {
  nfeKeys: string[]
  /** Escolha explícita do usuário; `null` segue a sugestão do `derive`. */
  takerRole: CteTakerRole | null
  takerPerson: PersonItemOut | null
  freightTotal: string
  components: CteFreightLine[]
  icms: CteIcmsState
  /** CFOP da prestação; nunca presumido. */
  cfop: string
  overrides: CteOverrides
  /** IBS/CBS (só CRT 3); `null` = grupo fora do CT-e. */
  ibsCbs?: CteIbsCbsState | null
  /** autXML: CPF/CNPJ do cadastro de pessoas. */
  authorizedXml?: { doc: string; name: string }[]
}

export interface CteIbsCbsState {
  cst: string
  class_trib: string
  donation: boolean
  base_value: string
  ibs_uf_rate: string
  ibs_mun_rate: string
  cbs_rate: string
}

export const emptyIbsCbs = (): CteIbsCbsState => ({
  cst: '', class_trib: '', donation: false, base_value: '', ibs_uf_rate: '', ibs_mun_rate: '', cbs_rate: '',
});

/** Alíquota em percentual com até 4 casas → décimos de milésimo (0.1000 → 1000). */
const toRate4 = (v: string): number => {
  const n = parseFloat(v);
  return isNaN(n) ? 0 : Math.round(n * 10000);
};

/**
 * Prévia dos valores que o servidor calcula (`ctes/ibscbs.go`): cada valor = vBC x alíquota / 100, 2 casas,
 * arredondamento bancário; IBS = UF + município; vTotDFe = vTPrest, mais IBS e CBS só a partir de 2027
 * (regra IBSCBS-365). `null` enquanto falta a base.
 */
export function computeCteIbsCbs(
  state: CteIbsCbsState, freightTotal: string, emissionYear: number,
): { ibsUf: string; ibsMun: string; ibs: string; cbs: string; total: string } | null {
  const base = toCents(state.base_value);
  if (base <= 0) return null;
  const val = (rate: string) => divHalfEven(base * toRate4(rate), 1_000_000);
  const uf = val(state.ibs_uf_rate), mun = val(state.ibs_mun_rate), cbs = val(state.cbs_rate);
  const ibs = uf + mun;
  const total = toCents(freightTotal) + (emissionYear >= CTE_TOTAL_WITH_TAXES_FROM_YEAR ? ibs + cbs : 0);
  return {ibsUf: fromCents(uf), ibsMun: fromCents(mun), ibs: fromCents(ibs), cbs: fromCents(cbs), total: fromCents(total)};
}

/** O IBS/CBS está pronto para sair no corpo? Nenhum campo é presumido. */
export const ibsCbsComplete = (s: CteIbsCbsState): boolean =>
  !!s.cst && !!s.class_trib && toCents(s.base_value) > 0
  && [s.ibs_uf_rate, s.ibs_mun_rate, s.cbs_rate].every((r) => r !== '' && !isNaN(parseFloat(r)));

/** Mesmo CPF/CNPJ, ignorando máscara e caixa. */
export const sameDoc = (a: string | null | undefined, b: string | null | undefined): boolean => {
  const norm = (x: string) => x.replace(/[^A-Za-z0-9]/g, '').toUpperCase();
  return !!a && !!b && norm(a) === norm(b);
};

export const emptyCteForm = (cst: CteIcmsCst | ''): CteFormState => ({
  nfeKeys: [],
  takerRole: null,
  takerPerson: null,
  freightTotal: '',
  components: [],
  icms: {cst, p_icms: '', p_red_bc: ''},
  cfop: '',
  overrides: {},
});

// ─── dinheiro em centavos (sem float no arredondamento) ──────────────────────

/** "1850.5" → 185050. Entradas com até 2 casas, então o `Math.round` é exato. */
export const toCents = (v: string): number => {
  const n = parseFloat(v);
  return isNaN(n) ? 0 : Math.round(n * 100);
};

export const fromCents = (cents: number): string => (cents / 100).toFixed(2);

/** Divisão inteira com arredondamento bancário (meio para o par), como o `RoundBank` do backend. */
export function divHalfEven(num: number, den: number): number {
  const q = Math.floor(num / den);
  const r = num - q * den;
  if (2 * r > den) return q + 1;
  if (2 * r === den) return q % 2 === 0 ? q : q + 1;
  return q;
}

/**
 * ICMS ao vivo: vBC = vTPrest com redução opcional, vICMS = vBC x pICMS, 2 casas,
 * arredondamento bancário. `null` quando o CST não destaca ICMS ou falta dado.
 */
export function computeCteIcms(total: string, icms: CteIcmsState): { base: string; value: string } | null {
  if (!icms.cst || CTE_CST_WITHOUT_RATE.has(icms.cst) || !icms.p_icms) return null;
  const totalCents = toCents(total);
  if (totalCents <= 0) return null;
  // Percentuais com 2 casas viram centésimos de ponto: 12.00 → 1200.
  const rate = toCents(icms.p_icms);
  const reduction = CTE_CST_WITH_REDUCTION.has(icms.cst) ? toCents(icms.p_red_bc) : 0;
  const base = divHalfEven(totalCents * (10000 - reduction), 10000);
  const value = divHalfEven(base * rate, 10000);
  return {base: fromCents(base), value: fromCents(value)};
}

export const componentsTotal = (lines: CteFreightLine[]): number =>
  lines.reduce((sum, l) => sum + toCents(l.value), 0);

// ─── derivado x editado ──────────────────────────────────────────────────────

export const effectiveTakerRole = (state: CteFormState, derive: CteDeriveOut | undefined): CteTakerRole | null =>
  state.takerRole ?? derive?.taker_suggestion?.role ?? null;

export const cityPlace = (ibge: string): CtePlace | null => {
  const c = CITIES.find((x) => x.code === ibge);
  return c ? {uf: c.uf, city_ibge: c.code, city: c.description} : null;
};

export const placeLabel = (p: CtePlace | null): string | null => (p ? `${p.city} / ${p.uf}` : null);

/** Valores efetivos de "Dados do transporte": o editado vence o derivado. */
export function cteTransport(derive: CteDeriveOut | undefined, o: CteOverrides) {
  return {
    senderName: o.sender?.name ?? derive?.sender?.name ?? null,
    recipientName: o.recipient?.name ?? derive?.recipient?.name ?? null,
    origin: o.origin ? cityPlace(o.origin) : derive?.origin ?? null,
    destination: o.destination ? cityPlace(o.destination) : derive?.destination ?? null,
    weightKg: o.total_weight_kg ?? derive?.cargo.total_weight_kg ?? null,
    cargoValue: o.total_value ?? derive?.cargo.total_value ?? null,
    product: o.predominant_product ?? derive?.cargo.predominant_product ?? null,
  };
}

/**
 * CFOPs de prestação de transporte. Com origem e destino conhecidos, só os do
 * escopo certo (5 na mesma UF, 6 entre UFs); sem eles, todos.
 */
export function cteCfopOptions(origin: CtePlace | null, destination: CtePlace | null) {
  const scope = origin && destination
    ? (origin.uf === destination.uf ? CTE_CFOP_SCOPE_INTRA : CTE_CFOP_SCOPE_INTER)
    : null;
  return getCfopOptionsForCte()
    .flatMap((o) => getCfopVariants(o.value as CfopCode))
    .filter((code) => !scope || code.startsWith(scope))
    .map((code) => ({value: code, label: `${code} · ${getCfopDescription(code) ?? ''}`}));
}

export const formatWeightKg = (kg: string): string =>
  `${Number(kg).toLocaleString('pt-BR', {maximumFractionDigits: 3})} kg`;

/** "3 notas · 960 kg · R$ 4.500,00 · SP para MG". */
export function cteSummary(derive: CteDeriveOut, o: CteOverrides): string {
  const t = cteTransport(derive, o);
  const n = derive.notes.length;
  const parts = [`${n} ${n === 1 ? 'nota' : 'notas'}`];
  if (t.weightKg) parts.push(formatWeightKg(t.weightKg));
  if (t.cargoValue) parts.push(formatCurrency(t.cargoValue));
  if (t.origin && t.destination) parts.push(`${t.origin.uf} para ${t.destination.uf}`);
  return parts.join(' · ');
}

/** Pendência de um formulário em passos: o passo dono, o texto e (se houver) o id do campo para o foco. */
export interface StepIssue {
  step: string
  text: string
  field?: string
}

/** Passos de pendência da emissão a partir de NF-e (ids dos passos do `CteEmitForm`). */
export const CTE_STEP_NOTES = 'notes';
export const CTE_STEP_PARTIES = 'parties';
export const CTE_STEP_FREIGHT = 'freight';

/** Pendências da emissão, cada uma com o passo dono. A ordem é a de preenchimento. */
export function cteMissingItems(
  state: CteFormState,
  derive: CteDeriveOut | undefined,
  deriving: boolean,
): StepIssue[] {
  if (state.nfeKeys.length === 0) return [{step: CTE_STEP_NOTES, text: 'Escolha ao menos uma nota', field: 'cte-notes-picker'}];
  if (deriving || !derive) return [{step: CTE_STEP_NOTES, text: 'Aguarde a leitura das notas'}];

  const missing: StepIssue[] = [];
  const role = effectiveTakerRole(state, derive);
  const t = cteTransport(derive, state.overrides);
  const parties = (text: string) => missing.push({step: CTE_STEP_PARTIES, text});
  if (!role) parties('Escolha quem paga o frete');
  else if (CTE_TAKER_NEEDS_PERSON.has(role) && !state.takerPerson) parties('Escolha a pessoa que paga o frete');

  missing.push(...chargeMissingItems(state, 'frete', CTE_STEP_FREIGHT));

  if (!t.senderName) parties('Escolha o remetente');
  if (!t.recipientName) parties('Escolha o destinatário');
  if (!t.weightKg) parties('Informe o peso da carga');
  if (state.ibsCbs && !ibsCbsComplete(state.ibsCbs)) missing.push({step: CTE_STEP_FREIGHT, text: 'Complete o IBS/CBS ou desligue o bloco'});
  return missing;
}

/**
 * O que falta para emitir, em linguagem de quem preenche. Vazio = pode emitir.
 */
export const cteMissing = (state: CteFormState, derive: CteDeriveOut | undefined, deriving: boolean): string[] =>
  cteMissingItems(state, derive, deriving).map((i) => i.text);

/** Parte da emissão que o complementar, o substituto e o CT-e OS repetem: valor, CFOP e ICMS. */
export type CteChargeState = Pick<CteFormState, 'freightTotal' | 'components' | 'icms' | 'cfop' | 'ibsCbs' | 'authorizedXml'>

/** O que falta no valor, no CFOP e no ICMS, já com o passo `step` e o campo; `noun` é o que o valor representa. */
export function chargeMissingItems(state: CteChargeState, noun: string, step: string): StepIssue[] {
  const missing: StepIssue[] = [];
  const add = (text: string, field?: string) => missing.push({step, text, field});
  const total = toCents(state.freightTotal);
  if (total <= 0) add(`Informe o valor do ${noun}`, 'cte-freight-total');
  else if (componentsTotal(state.components) >= total) add(`Componentes somam mais que o ${noun}`, 'cte-freight-total');
  if (state.components.some((c) => !c.name || toCents(c.value) <= 0)) add(`Complete os componentes do ${noun}`);

  if (!state.cfop) add('Escolha o CFOP da prestação', 'cte-cfop');
  if (!state.icms.cst) add('Escolha o CST do ICMS', 'cte-icms-cst');
  else if (!CTE_CST_WITHOUT_RATE.has(state.icms.cst) && !state.icms.p_icms) add('Informe a alíquota do ICMS', 'cte-icms-rate');
  return missing;
}

/** O que falta no valor, no CFOP e no ICMS; `noun` é o que o valor representa ("frete", "valor complementar"). */
export const chargeMissing = (state: CteChargeState, noun: string): string[] =>
  chargeMissingItems(state, noun, '').map((i) => i.text);

/** `{code: passo}` para uma lista de códigos 422 que moram no mesmo passo. */
export const stepOfCodes = (step: string, codes: readonly string[]): Record<string, string> =>
  Object.fromEntries(codes.map((c) => [c, step]));

/**
 * Passo dono de uma recusa 422: o `code` (CTE_DOC_ERRORS) manda; sem ele vale o alvo da recusa local (`aut_xml`...);
 * sem os dois, `fallback` (a revisão, onde o erro aparece).
 */
export function cteErrorStep(
  err: unknown, failure: EmitFailure, byCode: Record<string, string>, byTarget: Partial<Record<EmitTarget, string>>,
  fallback: string,
): string {
  const code = err instanceof ApiError ? (err.raw as {code?: unknown} | undefined)?.code : undefined;
  if (typeof code === 'string' && byCode[code]) return byCode[code];
  const target = failure.issues?.map((i) => i.target).find((t) => byTarget[t]);
  return (target && byTarget[target]) || fallback;
}

/** Valor, CFOP, ICMS, IBS/CBS e autXML no formato do corpo das rotas de emissão. */
export function buildChargeBody(state: CteChargeState): CteChargeBody {
  if (!state.icms.cst) throw new Error('CT-e incompleto: o ICMS é obrigatório');
  const cst = state.icms.cst;
  const withRate = !CTE_CST_WITHOUT_RATE.has(cst);
  return {
    cfop: state.cfop,
    freight: {
      total: fromCents(toCents(state.freightTotal)),
      ...(state.components.length
        ? {components: state.components.map((c) => ({name: c.name, value: fromCents(toCents(c.value))}))}
        : {}),
    },
    icms: {
      cst,
      ...(withRate ? {p_icms: fromCents(toCents(state.icms.p_icms))} : {}),
      ...(withRate && CTE_CST_WITH_REDUCTION.has(cst) && state.icms.p_red_bc
        ? {p_red_bc: fromCents(toCents(state.icms.p_red_bc))}
        : {}),
    },
    ...(state.ibsCbs ? {ibs_cbs: buildIbsCbs(state.ibsCbs)} : {}),
    ...(state.authorizedXml?.length ? {authorized_xml: state.authorizedXml.map((a) => ({doc: a.doc}))} : {}),
  };
}

/** Corpo do `POST /v1.0/ctes`: só o que o usuário decidiu ou editou; o resto o servidor deriva de novo. */
export function buildCteEmitBody(state: CteFormState, derive: CteDeriveOut | undefined): CteEmitBody {
  const role = effectiveTakerRole(state, derive);
  if (!role || !state.icms.cst) throw new Error('CT-e incompleto: tomador e ICMS são obrigatórios');

  const o = state.overrides;
  const cargo: CteEmitOverrides['cargo'] = {};
  if (o.total_value !== undefined) cargo.total_value = o.total_value;
  if (o.total_weight_kg !== undefined) cargo.total_weight_kg = o.total_weight_kg;
  if (o.predominant_product !== undefined) cargo.predominant_product = o.predominant_product.trim();

  const overrides: CteEmitOverrides = {};
  if (o.sender) overrides.sender_person_id = o.sender.sk;
  if (o.recipient) overrides.recipient_person_id = o.recipient.sk;
  if (Object.keys(cargo).length) overrides.cargo = cargo;
  if (o.origin) overrides.origin = {city_ibge: o.origin};
  if (o.destination) overrides.destination = {city_ibge: o.destination};

  return {
    nfe_keys: state.nfeKeys,
    taker: CTE_TAKER_NEEDS_PERSON.has(role) && state.takerPerson
      ? {role, person_id: state.takerPerson.sk}
      : {role},
    ...buildChargeBody(state),
    ...(Object.keys(overrides).length ? {overrides} : {}),
  };
}

function buildIbsCbs(s: CteIbsCbsState): CteIbsCbs {
  return {
    cst: s.cst, class_trib: s.class_trib, donation: s.donation || undefined,
    base_value: fromCents(toCents(s.base_value)),
    ibs_uf_rate: s.ibs_uf_rate, ibs_mun_rate: s.ibs_mun_rate, cbs_rate: s.cbs_rate,
  };
}

/** Falha da emissão com a saída que o usuário consegue tomar: RNTRC ausente leva à configuração. */
export function cteEmitFailure(err: unknown, fallback = 'Erro ao emitir CT-e.'): EmitFailure {
  if (err instanceof ApiError && (err.raw as {code?: unknown} | undefined)?.code === CTE_ERROR_RNTRC_REQUIRED) {
    return {
      message: err.detail || 'Cadastre o RNTRC da transportadora antes de emitir CT-e.',
      action: {label: 'Cadastrar RNTRC', href: CTE_RNTRC_CONFIG_HREF},
    };
  }
  return emitFailure(err, fallback, undefined, 'cte');
}

/** Espécie do registro: o CT-e OS tem `model` 67; o complementar e o substituto, `cte_type`; o resto é normal. */
export function cteKind(doc: Pick<CteListOut, 'cte_type' | 'model'>): CteKind {
  if (doc.model === CTE_MODEL_OS) return 'os';
  return doc.cte_type ?? 'normal';
}

/** Dias inteiros desde a emissão, para o prazo do substituto. */
export function daysSince(iso: string | null | undefined, now: Date = new Date()): number | null {
  const t = iso ? Date.parse(iso) : NaN;
  return isNaN(t) ? null : Math.floor((now.getTime() - t) / 86_400_000);
}

export type SubstituteWindow = 'open' | 'sefaz' | 'expired'

/** Até 60 dias: livre; de 60 a 67: a SEFAZ decide (G096); depois: o servidor recusa. */
export function substituteWindow(days: number | null): SubstituteWindow {
  if (days === null || days <= CTE_SUBSTITUTE_SEFAZ_DAYS) return 'open';
  return days > CTE_SUBSTITUTE_LIMIT_DAYS ? 'expired' : 'sefaz';
}

/** Por que o CT-e não aceita complemento nem substituto, ou `null` quando aceita. */
export function cteChangeBlocker(doc: CteListOut, isIssuer: boolean): string | null {
  const kind = cteKind(doc);
  if (kind === 'os') return 'O CT-e OS ainda não pode ser complementado nem substituído.';
  if (kind === 'complement') return 'Um CT-e complementar não pode ser complementado nem substituído.';
  if (doc.status !== 'authorized') return 'Só um CT-e autorizado pode ser complementado ou substituído.';
  if (!isIssuer) return 'Só o emitente do CT-e pode complementá-lo ou substituí-lo.';
  return null;
}

/** Falha do complemento, do substituto e do CT-e OS: o código do 422 vira texto de quem emite. */
export function cteDocFailure(err: unknown, fallback: string): EmitFailure {
  const code = err instanceof ApiError ? (err.raw as {code?: unknown} | undefined)?.code : undefined;
  if (typeof code === 'string' && CTE_DOC_ERRORS[code]) return {message: CTE_DOC_ERRORS[code]};
  return cteEmitFailure(err, fallback);
}

/** Falha de evento: mora em `event-failure` (sem as tabelas deste arquivo) para o menu da NF-e não carregá-las. */
export {cteEventFailureMessage} from '@/lib/utils/event-failure';

