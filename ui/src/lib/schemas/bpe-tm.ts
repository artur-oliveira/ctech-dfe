import {z} from 'zod';
import {BPE_ICMS_WITH_RATE, BPE_TM_COMPONENT_NAME_MAX, BPE_TM_COMPONENTS_MAX, BPE_TM_GROUPS_MAX, BPE_TM_ITEMS_MAX} from '@/lib/constants/bpe';
import type {BpeTmEmit} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());

const componentSchema = z.object({
  name: z.string().trim().min(1, 'Informe o nome').max(BPE_TM_COMPONENT_NAME_MAX, `Até ${BPE_TM_COMPONENT_NAME_MAX} caracteres`),
  quantity: z.string().regex(/^\d{1,5}$/, 'Quantidade de até 5 dígitos'),
});

const itemSchema = z.object({
  origin_ibge_code: z.string().regex(/^\d{7}$/, 'Código IBGE com 7 dígitos'),
  dest_ibge_code: z.string().refine((v) => blank(v) || /^\d{7}$/.test(v), 'Código IBGE com 7 dígitos'),
  passengers: z.string().regex(/^\d{1,6}$/, 'Passageiros com até 6 dígitos'),
  value: z.string().regex(decimal, 'Valor inválido'),
  components: z.array(componentSchema).min(1, 'Informe ao menos um componente').max(BPE_TM_COMPONENTS_MAX),
  icms: z.object({cst: z.string().min(1, 'Selecione o ICMS'), rate: z.string()}),
}).superRefine((it, ctx) => {
  if (BPE_ICMS_WITH_RATE.includes(it.icms.cst) && !decimal.test(it.icms.rate)) {
    ctx.addIssue({code: 'custom', path: ['icms', 'rate'], message: 'Informe a alíquota do ICMS'});
  }
});

const groupSchema = z.object({
  origin_uf: z.string().length(2, 'Selecione a UF'),
  dest_uf: z.string(),
  plate: z.string().refine((v) => blank(v) || /^[A-Za-z0-9]{7}$/.test(v), 'Placa com 7 caracteres'),
  prefix: z.string().max(20),
  items: z.array(itemSchema).min(1, 'Informe ao menos um item').max(BPE_TM_ITEMS_MAX),
});

export const bpeTmSchema = z.object({
  modal: z.string().min(1, 'Selecione o modal'),
  cfop: z.string().regex(/^[123567]\d([0-9][1-9]|[1-9][0-9])$/, 'CFOP inválido (4 dígitos; os dois últimos não são 00)'),
  competence: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Informe a competência'),
  supply_indicator: z.boolean(),
  groups: z.array(groupSchema).min(1, 'Informe ao menos um grupo de viagem').max(BPE_TM_GROUPS_MAX),
  additional_info: z.string().max(5000),
});

export type BpeTmForm = z.infer<typeof bpeTmSchema>
export type BpeTmItemForm = BpeTmForm['groups'][number]['items'][number]

export function emptyBpeTmItem(): BpeTmItemForm {
  return {
    origin_ibge_code: '', dest_ibge_code: '', passengers: '', value: '',
    components: [{name: '', quantity: ''}], icms: {cst: '40', rate: ''},
  };
}

export function emptyBpeTm(): BpeTmForm {
  return {
    modal: 'rodoviario', cfop: '', competence: '', supply_indicator: false,
    groups: [{origin_uf: '', dest_uf: '', plate: '', prefix: '', items: [emptyBpeTmItem()]}],
    additional_info: '',
  };
}

/** Corpo de `POST /bpes` com `variant` tm. O servidor soma o total e usa a posição para `idEqpCont` e `nViagem`. */
export function toBpeTmBody(f: BpeTmForm): BpeTmEmit {
  return {
    variant: 'tm', modal: f.modal, cfop: f.cfop, competence: f.competence,
    supply_indicator: f.supply_indicator ? true : undefined,
    groups: f.groups.map((g) => ({
      origin_uf: g.origin_uf, dest_uf: orUndefined(g.dest_uf), plate: orUndefined(g.plate)?.toUpperCase(), prefix: orUndefined(g.prefix),
      items: g.items.map((it) => ({
        origin_ibge_code: it.origin_ibge_code, dest_ibge_code: orUndefined(it.dest_ibge_code),
        passengers: it.passengers, value: it.value,
        components: it.components.map((c) => ({name: c.name.trim(), quantity: Number(c.quantity)})),
        icms: {cst: it.icms.cst, ...(BPE_ICMS_WITH_RATE.includes(it.icms.cst) ? {rate: it.icms.rate} : {})},
      })),
    })),
    additional_info: orUndefined(f.additional_info),
  };
}
