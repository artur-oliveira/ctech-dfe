import {z} from 'zod';
import {NFCOM_CLASS_CODE_LENGTH, NFCOM_ICMS_WITH_RATE, NFCOM_ITEMS_MAX} from '@/lib/constants/nfcom';
import {cleanOptional, cleanText, FISCAL_TEXT, FISCAL_TEXT_MESSAGE, REF_CODE, REF_CODE_MESSAGE} from '@/lib/utils/fiscal-text';
import type {NfcomEmit, NfcomPlanCreate, NfcomSubscriberCreate} from '@/lib/types/api';

const SEM_CST = 'sem_cst';
const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const optionalDecimal = (label: string) => z.string().refine((v) => blank(v) || decimal.test(v), `${label} inválido`);

/** Texto livre: normalizado pelo mapeador antes de enviar; aqui valida o que a pessoa digitou. */
const fiscalText = (label: string, min: number, max: number) =>
  z.string().transform(cleanText).pipe(z.string().min(min, `Informe ${label}`).max(max, `Até ${max} caracteres`).regex(FISCAL_TEXT, FISCAL_TEXT_MESSAGE));
const refCode = (label: string, max: number) =>
  z.string().trim().min(1, `Informe ${label}`).max(max, `Até ${max} caracteres`).regex(REF_CODE, REF_CODE_MESSAGE);

// ── Emissão: assinante e planos por id; só o que varia a cada fatura ───────────────────────────────────────────────

const itemSchema = z.object({
  plan_id: z.string().min(1, 'Selecione o plano'),
  quantity: z.string().regex(decimal, 'Quantidade inválida').refine((v) => Number(v) > 0, 'A quantidade deve ser maior que zero'),
  unit_value: optionalDecimal('Valor'),
  discount: optionalDecimal('Desconto'),
  other_charges: optionalDecimal('Valor'),
});

export const nfcomEmitSchema = z.object({
  subscriber_id: z.string().min(1, 'Selecione o assinante'),
  items: z.array(itemSchema).min(1, 'Informe ao menos um item').max(NFCOM_ITEMS_MAX),
  prepaid: z.boolean(),
  invoice: z.object({competence: z.string(), due_date: z.string(), bar_code: z.string()}),
  additional_info: z.string().max(3000, 'Até 3000 caracteres'),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const info = cleanText(f.additional_info);
  if (info !== '' && !FISCAL_TEXT.test(info)) issue(['additional_info'], FISCAL_TEXT_MESSAGE);
  if (!f.prepaid) {
    const inv = f.invoice;
    if (!/^\d{6}$/.test(inv.competence)) issue(['invoice', 'competence'], 'Selecione a competência');
    if (!/^\d{4}-\d{2}-\d{2}$/.test(inv.due_date)) issue(['invoice', 'due_date'], 'Informe o vencimento');
    if (!/^\d{1,48}$/.test(inv.bar_code)) issue(['invoice', 'bar_code'], 'Código de barras com até 48 dígitos');
  }
});

export type NfcomFormData = z.infer<typeof nfcomEmitSchema>
export type NfcomItemForm = NfcomFormData['items'][number]

export function emptyNfcomItem(): NfcomItemForm {
  return {plan_id: '', quantity: '1', unit_value: '', discount: '', other_charges: ''};
}

/** Competência sugerida: o mês corrente (AAAAMM); o vencimento sugerido é 10 dias adiante. */
export function suggestedInvoice(now = new Date()): NfcomFormData['invoice'] {
  const due = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 10);
  const pad = (n: number) => String(n).padStart(2, '0');
  return {
    competence: `${now.getFullYear()}${pad(now.getMonth() + 1)}`,
    due_date: `${due.getFullYear()}-${pad(due.getMonth() + 1)}-${pad(due.getDate())}`,
    bar_code: '',
  };
}

export function emptyNfcomForm(now = new Date()): NfcomFormData {
  return {subscriber_id: '', items: [emptyNfcomItem()], prepaid: false, invoice: suggestedInvoice(now), additional_info: ''};
}

const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());

/** Corpo de `POST /nfcoms`: ids e valores que variam, sem campo vazio. */
export function toNfcomEmit(f: NfcomFormData): NfcomEmit {
  return {
    subscriber_id: f.subscriber_id,
    items: f.items.map((it) => ({
      plan_id: it.plan_id, quantity: it.quantity,
      unit_value: orUndefined(it.unit_value), discount: orUndefined(it.discount), other_charges: orUndefined(it.other_charges),
    })),
    invoice: f.prepaid ? undefined : {competence: f.invoice.competence, due_date: f.invoice.due_date, bar_code: f.invoice.bar_code},
    prepaid: f.prepaid ? true : undefined,
    additional_info: cleanOptional(f.additional_info),
  };
}

// ── Cadastro de assinante ────────────────────────────────────────────────────────────────────────────────────────

export const nfcomSubscriberSchema = z.object({
  name: fiscalText('o nome', 2, 60),
  person_id: z.string().min(1, 'Selecione a pessoa'),
  code: refCode('o código do assinante', 30),
  type: z.string().min(1, 'Selecione o tipo'),
  service_used: z.string().min(1, 'Selecione o serviço'),
  contract_number: z.string().refine((v) => blank(v) || (REF_CODE.test(v.trim()) && v.trim().length <= 20), REF_CODE_MESSAGE),
  contract_start: z.string(),
  contract_end: z.string(),
  primary_terminal: z.string().refine((v) => blank(v) || /^\d{1,12}$/.test(v.trim()), 'Até 12 dígitos'),
  primary_ibge_code: z.string(),
  primary_uf: z.string(),
}).superRefine((f, ctx) => {
  if (!blank(f.contract_start) && !blank(f.contract_end) && f.contract_end < f.contract_start) {
    ctx.addIssue({code: 'custom', path: ['contract_end'], message: 'O fim do contrato não pode ser anterior ao início'});
  }
});

export type NfcomSubscriberFormData = z.input<typeof nfcomSubscriberSchema>

export function emptyNfcomSubscriber(): NfcomSubscriberFormData {
  return {
    name: '', person_id: '', code: '', type: '', service_used: '', contract_number: '', contract_start: '', contract_end: '',
    primary_terminal: '', primary_ibge_code: '', primary_uf: '',
  };
}

export function toNfcomSubscriber(f: z.output<typeof nfcomSubscriberSchema>): NfcomSubscriberCreate {
  return {
    name: f.name, person_id: f.person_id, code: f.code.trim(), type: f.type, service_used: f.service_used,
    contract_number: orUndefined(f.contract_number) ?? null,
    contract_start: orUndefined(f.contract_start) ?? null,
    contract_end: orUndefined(f.contract_end) ?? null,
    primary_terminal: orUndefined(f.primary_terminal) ?? null,
    primary_uf: orUndefined(f.primary_uf) ?? null,
    primary_ibge_code: orUndefined(f.primary_ibge_code) ?? null,
  };
}

// ── Cadastro de plano ─────────────────────────────────────────────────────────────────────────────────────────────

const taxRate = z.string().refine((v) => decimal.test(v) && Number(v) <= 100, 'Alíquota entre 0 e 100');

export const nfcomPlanSchema = z.object({
  name: fiscalText('o nome', 2, 60),
  code: refCode('o código', 60),
  description: fiscalText('a descrição', 1, 120),
  class_code: z.string().regex(new RegExp(`^\\d{${NFCOM_CLASS_CODE_LENGTH}}$`), 'Classificação com 7 dígitos'),
  unit: z.string().min(1, 'Selecione a unidade'),
  unit_value: optionalDecimal('Valor'),
  cfop: z.string(),
  icms_cst: z.string().min(1, 'Selecione o ICMS'),
  icms_rate: z.string(),
  pis_cst: z.string(),
  pis_rate: z.string(),
  cofins_cst: z.string(),
  cofins_rate: z.string(),
}).superRefine((p, ctx) => {
  const issue = (path: string, message: string) => ctx.addIssue({code: 'custom', path: [path], message});
  const semCst = p.icms_cst === SEM_CST;
  if (semCst && !blank(p.cfop)) issue('cfop', 'Plano sem situação tributária não leva CFOP');
  if (!semCst && !/^\d{4}$/.test(p.cfop)) issue('cfop', 'Selecione o CFOP');
  if (NFCOM_ICMS_WITH_RATE.includes(p.icms_cst) && !taxRate.safeParse(p.icms_rate).success) issue('icms_rate', 'Informe a alíquota do ICMS');
  if (!blank(p.pis_cst) && !taxRate.safeParse(p.pis_rate).success) issue('pis_rate', 'Informe a alíquota do PIS');
  if (!blank(p.cofins_cst) && !taxRate.safeParse(p.cofins_rate).success) issue('cofins_rate', 'Informe a alíquota da COFINS');
});

export type NfcomPlanFormData = z.input<typeof nfcomPlanSchema>

export function emptyNfcomPlan(): NfcomPlanFormData {
  return {
    name: '', code: '', description: '', class_code: '', unit: '4', unit_value: '', cfop: '',
    icms_cst: '00', icms_rate: '', pis_cst: '', pis_rate: '', cofins_cst: '', cofins_rate: '',
  };
}

export function toNfcomPlan(f: z.output<typeof nfcomPlanSchema>): NfcomPlanCreate {
  return {
    name: f.name, code: f.code.trim(), description: f.description, class_code: f.class_code, unit: f.unit,
    unit_value: orUndefined(f.unit_value) ?? null,
    cfop: f.icms_cst === SEM_CST ? null : f.cfop,
    icms: {cst: f.icms_cst, rate: NFCOM_ICMS_WITH_RATE.includes(f.icms_cst) ? f.icms_rate : null},
    pis: blank(f.pis_cst) ? null : {cst: f.pis_cst, rate: f.pis_rate},
    cofins: blank(f.cofins_cst) ? null : {cst: f.cofins_cst, rate: f.cofins_rate},
  };
}
