import {z} from 'zod';
import {
  CTE_ADDITIONAL_INFO_MAX_LENGTH,
  CTE_AUTHORIZED_XML_MAX,
  CTE_CST_WITHOUT_RATE,
  CTE_MAX_NOTES,
  CTE_PRODUCT_MAX_LENGTH,
  CTE_TAKER_NEEDS_PERSON,
  CTE_TAKER_ROLES,
} from '@/lib/constants/cte';
import type {CteIcmsCst, CteTakerRole} from '@/lib/types/api';

/** Espelho do `CteEmitBody` (docs/specs/2026-11-05-cte-emissao-rodoviaria.md). */

const ACCESS_KEY = /^\d{44}$/;
const MONEY = /^\d+(\.\d{1,2})?$/;
const WEIGHT = /^\d+(\.\d{1,3})?$/;
const PERCENT = /^\d{1,3}(\.\d{1,2})?$/;
const RATE = /^\d{1,3}(\.\d{1,4})?$/;
const IBGE = /^\d{7}$/;

const positiveMoney = z.string().regex(MONEY, 'Valor inválido').refine((v) => parseFloat(v) > 0, 'Informe um valor maior que zero');

export const cteEmitBodySchema = z.object({
  nfe_keys: z.array(z.string().regex(ACCESS_KEY, 'Chave de acesso inválida'))
    .min(1, 'Escolha ao menos uma nota')
    .max(CTE_MAX_NOTES, `No máximo ${CTE_MAX_NOTES} notas por CT-e`)
    .refine((keys) => new Set(keys).size === keys.length, 'Nota repetida'),
  taker: z.object({
    role: z.enum(CTE_TAKER_ROLES as [CteTakerRole, ...CteTakerRole[]]),
    person_id: z.string().min(1).optional(),
  }).refine((t) => !CTE_TAKER_NEEDS_PERSON.has(t.role) || !!t.person_id, {
    message: 'Escolha a pessoa que paga o frete', path: ['person_id'],
  }),
  freight: z.object({
    total: positiveMoney,
    components: z.array(z.object({name: z.string().min(1).max(15), value: positiveMoney})).optional(),
  }).refine((f) => {
    const sum = (f.components ?? []).reduce((acc, c) => acc + parseFloat(c.value), 0);
    return sum < parseFloat(f.total);
  }, {message: 'Os componentes somam mais que o total do frete', path: ['components']}),
  cfop: z.string().regex(/^[5-7]\d{3}$/, 'Escolha o CFOP da prestação'),
  icms: z.object({
    cst: z.enum(['00', '20', '40', '41', '51', '60', '90', 'SN']),
    p_icms: z.string().regex(PERCENT).optional(),
    p_red_bc: z.string().regex(PERCENT).optional(),
  }).refine((i) => CTE_CST_WITHOUT_RATE.has(i.cst as CteIcmsCst) || !!i.p_icms, {
    message: 'Informe a alíquota do ICMS', path: ['p_icms'],
  }).optional(),
  overrides: z.object({
    sender_person_id: z.string().min(1).optional(),
    recipient_person_id: z.string().min(1).optional(),
    cargo: z.object({
      total_value: z.string().regex(MONEY).optional(),
      total_weight_kg: z.string().regex(WEIGHT).optional(),
      volumes: z.number().int().nonnegative().optional(),
      predominant_product: z.string().min(1).max(CTE_PRODUCT_MAX_LENGTH).optional(),
    }).optional(),
    origin: z.object({city_ibge: z.string().regex(IBGE)}).optional(),
    destination: z.object({city_ibge: z.string().regex(IBGE)}).optional(),
  }).optional(),
  ibs_cbs: z.object({
    cst: z.string().regex(/^\d{3}$/),
    class_trib: z.string().regex(/^\d{6}$/),
    donation: z.boolean().optional(),
    base_value: positiveMoney,
    ibs_uf_rate: z.string().regex(RATE),
    ibs_mun_rate: z.string().regex(RATE),
    cbs_rate: z.string().regex(RATE),
  }).optional(),
  authorized_xml: z.array(z.object({doc: z.string().min(11)})).max(CTE_AUTHORIZED_XML_MAX, `No máximo ${CTE_AUTHORIZED_XML_MAX} autorizados`)
    .refine((a) => new Set(a.map((x) => x.doc)).size === a.length, 'Autorizado repetido').optional(),
  additional_info: z.string().max(CTE_ADDITIONAL_INFO_MAX_LENGTH).optional(),
});
