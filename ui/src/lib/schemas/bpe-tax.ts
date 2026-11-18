import {z} from 'zod';
import {BPE_CBS_RATE_FROM_YEAR, BPE_IBS_RATE_FROM_YEAR, BPE_TA_CLASSES, BPE_TA_NO_TAX_CST} from '@/lib/constants/bpe';
import type {BpeTaTaxIn} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());
const optDecimal = (s: string) => blank(s) || decimal.test(s);

/** IBS/CBS do TA: guardado na linha, uma vez. */
export const taTaxSchema = z.object({
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
});

export type TaTaxForm = z.infer<typeof taTaxSchema>

export function emptyTaTax(): TaTaxForm {
  return {
    class_trib: '', donation: false, ibs_uf_rate: '', ibs_mun_rate: '', cbs_rate: '',
    devolution: {ibs_uf: '', ibs_mun: '', cbs: ''}, reversal: {ibs: '', cbs: ''},
    government: {enabled: false, entity: '', reducer: '0', operation: '', references: ''},
    simples: {enabled: false, ibs_rate: '', cbs_rate: ''}, total_taxes: '', fiscal_info: '',
  };
}

/** Chaves de DFe anteriores, uma por linha ou separadas por espaço ou vírgula (campo livre, validado pela regex da chave). */
export function govKeys(raw: string): string[] {
  return raw.split(/[\s,;]+/).map((k) => k.trim().toUpperCase()).filter((k) => k !== '');
}

/** Só as operações 2 e 3 levam chave de DFe anterior; o campo some nas outras, e o texto que sobrar nele não segue. */
function govRefs(g: TaTaxForm['government']): string[] {
  return g.operation === '2' || g.operation === '3' ? govKeys(g.references) : [];
}

/** Regras do imposto do TA (classificação, doação, alíquotas do ano, estorno, compra governamental e Simples). */
export function taTaxIssues(t: TaTaxForm, issue: (path: (string | number)[], message: string) => void, year = new Date().getFullYear()): void {
  const cls = BPE_TA_CLASSES.find((c) => c.value === t.class_trib);
  if (!cls) issue(['class_trib'], 'Selecione a classificação tributária');
  if (cls?.donation === 1 && !t.donation) issue(['donation'], 'Esta classificação exige o indicador de doação');
  if (cls?.donation === 0 && t.donation) issue(['donation'], 'Esta classificação não admite doação');
  const taxed = cls !== undefined && cls.cst !== BPE_TA_NO_TAX_CST;
  if (taxed && year >= BPE_CBS_RATE_FROM_YEAR && !decimal.test(t.cbs_rate)) issue(['cbs_rate'], 'Informe a alíquota da CBS');
  if (taxed && year >= BPE_IBS_RATE_FROM_YEAR) {
    (['ibs_uf_rate', 'ibs_mun_rate'] as const).forEach((k) => {
      if (!decimal.test(t[k])) issue([k], 'Informe a alíquota do IBS');
    });
  }
  (['ibs_uf_rate', 'ibs_mun_rate', 'cbs_rate', 'total_taxes'] as const).forEach((k) => {
    if (!optDecimal(t[k])) issue([k], 'Valor inválido');
  });
  (['ibs_uf', 'ibs_mun', 'cbs'] as const).forEach((k) => {
    if (!optDecimal(t.devolution[k])) issue(['devolution', k], 'Valor inválido');
  });
  if (cls?.reversal) {
    (['ibs', 'cbs'] as const).forEach((k) => {
      if (!decimal.test(t.reversal[k])) issue(['reversal', k], 'Informe o valor estornado');
    });
  }
  if (t.government.enabled) {
    const g = t.government;
    if (blank(g.entity)) issue(['government', 'entity'], 'Selecione o ente');
    if (blank(g.operation)) issue(['government', 'operation'], 'Selecione a operação');
    if (!decimal.test(g.reducer)) issue(['government', 'reducer'], 'Informe o redutor (0 se não houver)');
    const keys = govRefs(g);
    if (keys.some((k) => !/^[0-9]{6}[A-Z0-9]{12}[0-9]{26}$/.test(k))) issue(['government', 'references'], 'Chave de acesso inválida');
    if ((g.operation === '1' || g.operation === '4') && keys.length > 0) issue(['government', 'references'], 'Esta operação não leva chave referenciada');
    if (g.operation === '2' && keys.length !== 1) issue(['government', 'references'], 'Informe uma chave referenciada');
    if (g.operation === '3' && keys.length < 1) issue(['government', 'references'], 'Informe ao menos uma chave referenciada');
  }
  if (t.simples.enabled) {
    (['ibs_rate', 'cbs_rate'] as const).forEach((k) => {
      if (!optDecimal(t.simples[k])) issue(['simples', k], 'Alíquota inválida');
    });
  }
}

export function toTaTax(t: TaTaxForm, year = new Date().getFullYear()): BpeTaTaxIn {
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

/** Converte o imposto gravado na linha de volta para o formulário. */
export function taTaxFromLine(t: BpeTaTaxIn | null | undefined): TaTaxForm {
  if (!t) return emptyTaTax();
  const e = emptyTaTax();
  return {
    ...e,
    class_trib: t.class_trib ?? '', donation: !!t.donation,
    ibs_uf_rate: t.ibs_uf_rate ?? '', ibs_mun_rate: t.ibs_mun_rate ?? '', cbs_rate: t.cbs_rate ?? '',
    devolution: {ibs_uf: t.devolution?.ibs_uf ?? '', ibs_mun: t.devolution?.ibs_mun ?? '', cbs: t.devolution?.cbs ?? ''},
    reversal: {ibs: t.credit_reversal?.ibs ?? '', cbs: t.credit_reversal?.cbs ?? ''},
    government: t.government
      ? {enabled: true, entity: t.government.entity, reducer: t.government.reducer, operation: t.government.operation, references: t.government.references.join('\n')}
      : e.government,
    simples: t.simples ? {enabled: true, ibs_rate: t.simples.ibs_rate ?? '', cbs_rate: t.simples.cbs_rate ?? ''} : e.simples,
    total_taxes: t.total_taxes ?? '', fiscal_info: t.fiscal_info ?? '',
  };
}
