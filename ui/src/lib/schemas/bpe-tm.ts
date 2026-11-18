import {z} from 'zod';
import {BPE_TM_COMPONENTS_MAX, BPE_TM_GROUPS_MAX, BPE_TM_ITEMS_MAX} from '@/lib/constants/bpe';
import {cleanOptional, cleanText, FISCAL_TEXT, FISCAL_TEXT_MESSAGE} from '@/lib/utils/fiscal-text';
import type {BpeTmEmit} from '@/lib/types/api';

const decimal = /^\d+(\.\d+)?$/;
const blank = (s: string) => s.trim() === '';
const orUndefined = (s: string): string | undefined => (blank(s) ? undefined : s.trim());
const digitsOrBlank = (max: number, label: string) => z.string().refine((v) => blank(v) || new RegExp(`^\\d{1,${max}}$`).test(v), `${label} com até ${max} dígitos`);

const componentSchema = z.object({
  name: z.string().min(1, 'Selecione o tipo de passageiro'),
  quantity: z.string().regex(/^\d{1,5}$/, 'Quantidade de até 5 dígitos'),
});

const itemSchema = z.object({
  leg: z.string().min(1, 'Selecione o trecho'),
  trip_number: digitsOrBlank(3, 'Número da viagem'),
  start_counter: digitsOrBlank(20, 'Contador'),
  end_counter: digitsOrBlank(20, 'Contador'),
  passengers: z.string().regex(/^\d{1,6}$/, 'Passageiros com até 6 dígitos'),
  value: z.string().regex(decimal, 'Valor inválido'),
  components: z.array(componentSchema).min(1, 'Informe ao menos um tipo de passageiro').max(BPE_TM_COMPONENTS_MAX),
});

const groupSchema = z.object({
  line_id: z.string().min(1, 'Selecione a linha'),
  counter_id: digitsOrBlank(3, 'Contador'),
  items: z.array(itemSchema).min(1, 'Informe ao menos um item').max(BPE_TM_ITEMS_MAX),
});

export const bpeTmSchema = z.object({
  competence: z.string().regex(/^\d{4}-\d{2}-\d{2}$/, 'Informe a competência'),
  supply_indicator: z.boolean(),
  groups: z.array(groupSchema).min(1, 'Informe ao menos um grupo de viagem').max(BPE_TM_GROUPS_MAX),
  additional_info: z.string().max(5000, 'Até 5000 caracteres'),
}).superRefine((f, ctx) => {
  const info = cleanText(f.additional_info);
  if (info !== '' && !FISCAL_TEXT.test(info)) ctx.addIssue({code: 'custom', path: ['additional_info'], message: FISCAL_TEXT_MESSAGE});
});

export type BpeTmForm = z.infer<typeof bpeTmSchema>
export type BpeTmItemForm = BpeTmForm['groups'][number]['items'][number]

export function emptyBpeTmItem(): BpeTmItemForm {
  return {leg: '', trip_number: '', start_counter: '', end_counter: '', passengers: '', value: '', components: [{name: '', quantity: ''}]};
}

export function emptyBpeTm(): BpeTmForm {
  return {competence: '', supply_indicator: false, groups: [{line_id: '', counter_id: '', items: [emptyBpeTmItem()]}], additional_info: ''};
}

const num = (s: string): number | undefined => (blank(s) ? undefined : Number(s));

/** Corpo de `POST /bpes` com `variant` tm: linhas por id; a apuração de cada trecho é o que varia. */
export function toBpeTmBody(f: BpeTmForm): BpeTmEmit {
  return {
    variant: 'tm', competence: f.competence,
    supply_indicator: f.supply_indicator ? true : undefined,
    groups: f.groups.map((g) => ({
      line_id: g.line_id, counter_id: num(g.counter_id),
      items: g.items.map((it) => ({
        leg: Number(it.leg), trip_number: num(it.trip_number),
        start_counter: orUndefined(it.start_counter), end_counter: orUndefined(it.end_counter),
        passengers: it.passengers, value: it.value,
        components: it.components.map((c) => ({name: c.name, quantity: Number(c.quantity)})),
      })),
    })),
    additional_info: cleanOptional(f.additional_info),
  };
}
