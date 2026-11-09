import {z} from 'zod';
import {
  CTE_ADDITIONAL_INFO_MAX_LENGTH,
  CTE_AUTHORIZED_XML_MAX,
  CTE_CST_WITHOUT_RATE,
  CTE_MAX_NOTES,
  CTE_OS_DESCRIPTION_MAX,
  CTE_OS_DOCS_MAX,
  CTE_OS_GTVE_COMPONENT_NAME_MAX,
  CTE_OS_ROUTE_UFS_MAX,
  CTE_PRODUCT_MAX_LENGTH,
  CTE_SUBSTITUTE_NEEDS_PERSON,
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

const freightSchema = z.object({
    total: positiveMoney,
    components: z.array(z.object({name: z.string().min(1).max(15), value: positiveMoney})).optional(),
  }).refine((f) => {
    const sum = (f.components ?? []).reduce((acc, c) => acc + parseFloat(c.value), 0);
    return sum < parseFloat(f.total);
  }, {message: 'Os componentes somam mais que o total do frete', path: ['components']});

const icmsSchema = z.object({
    cst: z.enum(['00', '20', '40', '41', '51', '60', '90', 'SN']),
    p_icms: z.string().regex(PERCENT).optional(),
    p_red_bc: z.string().regex(PERCENT).optional(),
  }).refine((i) => CTE_CST_WITHOUT_RATE.has(i.cst as CteIcmsCst) || !!i.p_icms, {
    message: 'Informe a alíquota do ICMS', path: ['p_icms'],
  });

const ibsCbsSchema = z.object({
    cst: z.string().regex(/^\d{3}$/),
    class_trib: z.string().regex(/^\d{6}$/),
    donation: z.boolean().optional(),
    base_value: positiveMoney,
    ibs_uf_rate: z.string().regex(RATE),
    ibs_mun_rate: z.string().regex(RATE),
    cbs_rate: z.string().regex(RATE),
  });

const authorizedXmlSchema = z.array(z.object({doc: z.string().min(11)})).max(CTE_AUTHORIZED_XML_MAX, `No máximo ${CTE_AUTHORIZED_XML_MAX} autorizados`)
    .refine((a) => new Set(a.map((x) => x.doc)).size === a.length, 'Autorizado repetido');

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
  freight: freightSchema,
  cfop: z.string().regex(/^[5-7]\d{3}$/, 'Escolha o CFOP da prestação'),
  icms: icmsSchema.optional(),
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
  ibs_cbs: ibsCbsSchema.optional(),
  authorized_xml: authorizedXmlSchema.optional(),
  additional_info: z.string().max(CTE_ADDITIONAL_INFO_MAX_LENGTH).optional(),
});

/** Valor e tributos que o complementar, o substituto e o CT-e OS repetem (OpenAPI `CteChargeProps`). */
const chargeShape = {
  freight: freightSchema,
  cfop: z.string().regex(/^[5-7]\d{3}$/, 'Escolha o CFOP da prestação'),
  icms: icmsSchema.optional(),
  ibs_cbs: ibsCbsSchema.optional(),
  authorized_xml: authorizedXmlSchema.optional(),
  additional_info: z.string().max(CTE_ADDITIONAL_INFO_MAX_LENGTH).optional(),
};

export const cteComplementBodySchema = z.object({
  original_keys: z.array(z.string().regex(ACCESS_KEY, 'Chave de acesso inválida')).min(1).max(10)
    .refine((keys) => new Set(keys).size === keys.length, 'CT-e repetido'),
  ...chargeShape,
});

export const cteSubstituteBodySchema = z.object({
  original_key: z.string().regex(ACCESS_KEY, 'Chave de acesso inválida'),
  change_taker: z.boolean().optional(),
  taker: z.object({
    role: z.enum(CTE_TAKER_ROLES as [CteTakerRole, ...CteTakerRole[]]),
    person_id: z.string().min(1).optional(),
  }).optional(),
  ...chargeShape,
}).refine((b) => !b.change_taker || !!b.taker, {message: 'Escolha o novo tomador', path: ['taker']})
  .refine((b) => !b.taker || b.change_taker, {message: 'O tomador só vale com a troca de tomador', path: ['taker']})
  .refine((b) => !b.taker || !CTE_SUBSTITUTE_NEEDS_PERSON.has(b.taker.role) || !!b.taker.person_id,
    {message: 'Escolha a pessoa do cadastro', path: ['taker', 'person_id']});

const osPlace = z.object({city_ibge: z.string().regex(IBGE), city: z.string().min(2).max(60), uf: z.string().length(2)});

export const cteOsBodySchema = z.object({
  service_type: z.enum(['people', 'values', 'baggage']),
  taker: z.object({person_id: z.string().min(1, 'Escolha o tomador do serviço')}),
  service: z.object({
    description: z.string().trim().min(1, 'Descreva o serviço').max(CTE_OS_DESCRIPTION_MAX),
    quantity: z.string().regex(/^\d+(\.\d{1,4})?$/).optional(),
  }),
  origin: osPlace.optional(),
  destination: osPlace.optional(),
  route_ufs: z.array(z.string().length(2)).max(CTE_OS_ROUTE_UFS_MAX).optional(),
  road: z.object({
    taf: z.string().regex(/^\d{12}$/, 'O TAF tem 12 dígitos').optional(),
    state_registration: z.string().regex(/^\d{25}$/, 'O registro estadual tem 25 dígitos').optional(),
    vehicle: z.object({
      plate: z.string().regex(/^[A-Za-z0-9]{7}$/),
      renavam: z.string().regex(/^\d{9,11}$/).optional(),
      uf: z.string().length(2).optional(),
    }).optional(),
    charter: z.object({type: z.union([z.literal(1), z.literal(2)]), trip_at: z.string().optional()}).optional(),
  }).optional(),
  doc_refs: z.array(z.object({
    bpe_key: z.string().regex(ACCESS_KEY, 'Chave do BP-e inválida').optional(),
    number: z.string().min(1).max(20).optional(),
    serie: z.string().max(3).optional(),
    subserie: z.string().max(3).optional(),
    issued_on: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).optional(),
    amount: z.string().regex(MONEY).optional(),
  })).max(CTE_OS_DOCS_MAX).optional(),
  gtve: z.array(z.object({
    key: z.string().regex(ACCESS_KEY, 'Chave da GTV-e inválida'),
    components: z.array(z.object({
      type: z.number().int().min(1).max(6),
      value: positiveMoney,
      name: z.string().max(CTE_OS_GTVE_COMPONENT_NAME_MAX).optional(),
    })).min(1).max(6),
  })).max(CTE_OS_DOCS_MAX).optional(),
  federal_taxes: z.object({
    pis: z.string().regex(MONEY).optional(), cofins: z.string().regex(MONEY).optional(), ir: z.string().regex(MONEY).optional(),
    inss: z.string().regex(MONEY).optional(), csll: z.string().regex(MONEY).optional(),
  }).optional(),
  ...chargeShape,
});
