import {z} from 'zod';
import {validateCNPJ, validateCPF} from '@/lib/utils/validators';
import {NFCOM_CLASS_CODE_LENGTH, NFCOM_ICMS_WITH_RATE, NFCOM_ITEMS_MAX} from '@/lib/constants/nfcom';
import type {NfcomEmit, NfcomItemIn} from '@/lib/types/api';

const SEM_CST = 'sem_cst';
const digits = (s: string) => s.replace(/\D/g, '');
const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';

const addressSchema = z.object({
  street: z.string().trim().min(2, 'Informe o logradouro').max(60),
  number: z.string().trim().min(1, 'Informe o número').max(60),
  complement: z.string().max(60),
  neighborhood: z.string().trim().min(2, 'Informe o bairro').max(60),
  ibge_code: z.string().regex(/^\d{7}$/, 'Código IBGE com 7 dígitos'),
  city: z.string().trim().min(2, 'Informe o município').max(60),
  postal_code: z.string().refine((v) => digits(v).length === 8, 'CEP com 8 dígitos'),
  uf: z.string().length(2, 'Selecione a UF'),
  phone: z.string(),
  email: z.string(),
});

const itemSchema = z.object({
  code: z.string().trim().min(1, 'Informe o código').max(60),
  description: z.string().trim().min(1, 'Informe a descrição').max(120),
  class_code: z.string().regex(new RegExp(`^\\d{${NFCOM_CLASS_CODE_LENGTH}}$`), 'Classificação com 7 dígitos'),
  cfop: z.string(),
  unit: z.string().min(1, 'Selecione a unidade'),
  quantity: z.string().regex(decimal, 'Quantidade inválida').refine((v) => Number(v) > 0, 'A quantidade deve ser maior que zero'),
  unit_value: z.string().regex(decimal, 'Valor inválido'),
  discount: z.string().refine((v) => blank(v) || decimal.test(v), 'Desconto inválido'),
  other_charges: z.string().refine((v) => blank(v) || decimal.test(v), 'Valor inválido'),
  icms_cst: z.string().min(1, 'Selecione o ICMS'),
  icms_rate: z.string(),
  pis_cst: z.string(),
  pis_rate: z.string(),
  cofins_cst: z.string(),
  cofins_rate: z.string(),
}).superRefine((it, ctx) => {
  const issue = (path: string, message: string) => ctx.addIssue({code: 'custom', path: [path], message});
  const semCst = it.icms_cst === SEM_CST;
  if (semCst && !blank(it.cfop)) issue('cfop', 'Item sem situação tributária não leva CFOP');
  if (!semCst && !/^\d{4}$/.test(it.cfop)) issue('cfop', 'Informe o CFOP (4 dígitos)');
  if (NFCOM_ICMS_WITH_RATE.includes(it.icms_cst) && !decimal.test(it.icms_rate)) issue('icms_rate', 'Informe a alíquota do ICMS');
  if (!blank(it.pis_cst) && !decimal.test(it.pis_rate)) issue('pis_rate', 'Informe a alíquota do PIS');
  if (!blank(it.cofins_cst) && !decimal.test(it.cofins_rate)) issue('cofins_rate', 'Informe a alíquota da COFINS');
});

export const nfcomEmitSchema = z.object({
  customer: z.object({
    name: z.string().trim().min(2, 'Informe o nome').max(60),
    document_kind: z.enum(['cnpj', 'cpf', 'other']),
    document: z.string().trim().min(1, 'Informe o documento'),
    ie_indicator: z.enum(['1', '2', '9']),
    ie: z.string(),
    address: addressSchema,
  }),
  subscriber: z.object({
    code: z.string().trim().min(1, 'Informe o código do assinante').max(30),
    type: z.string().min(1, 'Selecione o tipo'),
    service_used: z.string().min(1, 'Selecione o serviço'),
    contract_number: z.string().max(20),
    contract_start: z.string(),
    contract_end: z.string(),
    primary_terminal: z.string().max(12),
  }),
  items: z.array(itemSchema).min(1, 'Informe ao menos um item').max(NFCOM_ITEMS_MAX),
  prepaid: z.boolean(),
  invoice: z.object({competence: z.string(), due_date: z.string(), bar_code: z.string()}),
  additional_info: z.string().max(3000),
}).superRefine((f, ctx) => {
  const issue = (path: (string | number)[], message: string) => ctx.addIssue({code: 'custom', path, message});
  const c = f.customer;
  if (c.document_kind === 'cnpj' && !validateCNPJ(c.document)) issue(['customer', 'document'], 'CNPJ inválido');
  if (c.document_kind === 'cpf' && !validateCPF(c.document)) issue(['customer', 'document'], 'CPF inválido');
  if (c.ie_indicator === '1' && blank(c.ie)) issue(['customer', 'ie'], 'Informe a IE do cliente contribuinte');
  if (c.ie_indicator === '2' && c.ie.trim().toUpperCase() !== 'ISENTO') issue(['customer', 'ie'], 'A IE do cliente isento deve ser ISENTO');
  if (c.ie_indicator === '9' && !blank(c.ie)) issue(['customer', 'ie'], 'Cliente não contribuinte não tem IE');
  const s = f.subscriber;
  if (!blank(s.contract_start) && !blank(s.contract_end) && s.contract_end < s.contract_start) {
    issue(['subscriber', 'contract_end'], 'O fim do contrato não pode ser anterior ao início');
  }
  if (!f.prepaid) {
    const inv = f.invoice;
    if (!/^\d{6}$/.test(inv.competence)) issue(['invoice', 'competence'], 'Competência no formato AAAAMM');
    if (!/^\d{4}-\d{2}-\d{2}$/.test(inv.due_date)) issue(['invoice', 'due_date'], 'Informe o vencimento');
    if (!/^\d{1,48}$/.test(inv.bar_code)) issue(['invoice', 'bar_code'], 'Código de barras com até 48 dígitos');
  }
});

export type NfcomFormData = z.infer<typeof nfcomEmitSchema>
export type NfcomItemForm = NfcomFormData['items'][number]

export function emptyNfcomItem(): NfcomItemForm {
  return {
    code: '', description: '', class_code: '', cfop: '', unit: '4', quantity: '1', unit_value: '', discount: '', other_charges: '',
    icms_cst: '00', icms_rate: '', pis_cst: '', pis_rate: '', cofins_cst: '', cofins_rate: '',
  };
}

export function emptyNfcomForm(): NfcomFormData {
  return {
    customer: {
      name: '', document_kind: 'cnpj', document: '', ie_indicator: '9', ie: '',
      address: {street: '', number: '', complement: '', neighborhood: '', ibge_code: '', city: '', postal_code: '', uf: '', phone: '', email: ''},
    },
    subscriber: {code: '', type: '', service_used: '', contract_number: '', contract_start: '', contract_end: '', primary_terminal: ''},
    items: [emptyNfcomItem()],
    prepaid: false,
    invoice: {competence: '', due_date: '', bar_code: ''},
    additional_info: '',
  };
}

const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());

function toItem(it: NfcomItemForm): NfcomItemIn {
  return {
    code: it.code.trim(), description: it.description.trim(), class_code: it.class_code,
    cfop: orUndefined(it.cfop), unit: it.unit, quantity: it.quantity, unit_value: it.unit_value,
    discount: orUndefined(it.discount), other_charges: orUndefined(it.other_charges),
    icms: {cst: it.icms_cst, ...(NFCOM_ICMS_WITH_RATE.includes(it.icms_cst) ? {rate: it.icms_rate} : {})},
    pis: blank(it.pis_cst) ? undefined : {cst: it.pis_cst, rate: it.pis_rate},
    cofins: blank(it.cofins_cst) ? undefined : {cst: it.cofins_cst, rate: it.cofins_rate},
  };
}

/** Corpo de `POST /nfcoms`: documentos só com dígitos (CNPJ alfanumérico mantém as letras) e sem campo vazio. */
export function toNfcomEmit(f: NfcomFormData): NfcomEmit {
  const c = f.customer;
  const doc = c.document_kind === 'cpf' ? digits(c.document) : c.document.replace(/[^0-9A-Za-z]/g, '').toUpperCase();
  const a = c.address;
  return {
    customer: {
      name: c.name.trim(),
      cnpj: c.document_kind === 'cnpj' ? doc : undefined,
      cpf: c.document_kind === 'cpf' ? doc : undefined,
      other_id: c.document_kind === 'other' ? c.document.trim() : undefined,
      ie_indicator: c.ie_indicator,
      ie: orUndefined(c.ie),
      address: {
        street: a.street.trim(), number: a.number.trim(), complement: orUndefined(a.complement), neighborhood: a.neighborhood.trim(),
        ibge_code: a.ibge_code, city: a.city.trim(), postal_code: digits(a.postal_code), uf: a.uf,
        phone: orUndefined(digits(a.phone)), email: orUndefined(a.email),
      },
    },
    subscriber: {
      code: f.subscriber.code.trim(), type: f.subscriber.type, service_used: f.subscriber.service_used,
      contract_number: orUndefined(f.subscriber.contract_number), contract_start: orUndefined(f.subscriber.contract_start),
      contract_end: orUndefined(f.subscriber.contract_end), primary_terminal: orUndefined(f.subscriber.primary_terminal),
    },
    items: f.items.map(toItem),
    invoice: f.prepaid ? undefined : {competence: f.invoice.competence, due_date: f.invoice.due_date, bar_code: f.invoice.bar_code},
    prepaid: f.prepaid ? true : undefined,
    additional_info: orUndefined(f.additional_info),
  };
}
