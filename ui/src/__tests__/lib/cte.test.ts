import {describe, expect, it} from 'vitest';
import type {InternalAxiosRequestConfig} from 'axios';
import {ApiError} from '@/lib/api/client';
import {mockAdapter} from '@/lib/mock/handler';
import {nfesFixture, receivedNfesFixture} from '@/lib/mock/fixtures';
import {cteEmitBodySchema} from '@/lib/schemas/cte';
import {
  buildCteEmitBody,
  computeCteIcms,
  cteCfopOptions,
  cteEmitFailure,
  cteMissing,
  cteSummary,
  divHalfEven,
  effectiveTakerRole,
  emptyCteForm,
  type CteFormState,
} from '@/lib/utils/cte';
import type {CteDeriveOut, PersonItemOut} from '@/lib/types/api';

const KEY_A = nfesFixture[0].sk;
const KEY_B = nfesFixture[1].sk;
const KEY_IN = receivedNfesFixture[0].sk;
const KEY_IN_2 = receivedNfesFixture[1].sk;

async function derive(keys: string[]): Promise<CteDeriveOut> {
  const res = await mockAdapter({
    method: 'post', url: '/v1.0/ctes/derive', headers: {}, data: JSON.stringify({nfe_keys: keys}),
  } as unknown as InternalAxiosRequestConfig);
  return res.data as CteDeriveOut;
}

const person = (sk: string, name: string) => ({sk, name} as PersonItemOut);

const ready = (over: Partial<CteFormState> = {}): CteFormState => ({
  ...emptyCteForm('SN'),
  nfeKeys: [KEY_A, KEY_B],
  freightTotal: '1850.00',
  cfop: '6353',
  ...over,
});

describe('ICMS do CT-e ao vivo', () => {
  it('arredonda para o par (RoundBank)', () => {
    expect(divHalfEven(1025, 10)).toBe(102);
    expect(divHalfEven(1035, 10)).toBe(104);
    expect(divHalfEven(1036, 10)).toBe(104);
  });

  it('calcula vBC e vICMS com e sem redução', () => {
    expect(computeCteIcms('1850.00', {cst: '00', p_icms: '12', p_red_bc: ''})).toEqual({base: '1850.00', value: '222.00'});
    // 10,25 x 10% = 1,025 → 1,02 (meio para o par), não 1,03.
    expect(computeCteIcms('10.25', {cst: '00', p_icms: '10', p_red_bc: ''})?.value).toBe('1.02');
    expect(computeCteIcms('1000', {cst: '20', p_icms: '12', p_red_bc: '33.33'})).toEqual({base: '666.70', value: '80.00'});
    // Redução só vale para CST com redução no leiaute.
    expect(computeCteIcms('1000', {cst: '00', p_icms: '12', p_red_bc: '50'})?.base).toBe('1000.00');
  });

  it('não calcula sem alíquota ou com CST que dispensa', () => {
    expect(computeCteIcms('1000', {cst: '00', p_icms: '', p_red_bc: ''})).toBeNull();
    expect(computeCteIcms('1000', {cst: '40', p_icms: '12', p_red_bc: ''})).toBeNull();
    expect(computeCteIcms('1000', {cst: 'SN', p_icms: '', p_red_bc: ''})).toBeNull();
  });
});

describe('derive do mock (fixtures da emissão de CT-e)', () => {
  it('notas emitidas com frete CIF sugerem o remetente', async () => {
    const d = await derive([KEY_A, KEY_B]);
    expect(d.taker_suggestion).toEqual({role: 'remetente', reason: 'mod_frete_cif'});
    expect(d.sender?.name).toBe('Empresa Mock LTDA');
    expect(cteSummary(d, {})).toBe('2 notas · 338,5 kg · R$ 3.769,60 · SP para MG');
    expect(d.warnings).toEqual([]);
  });

  it('notas recebidas com frete FOB sugerem o destinatário', async () => {
    const d = await derive([KEY_IN, KEY_IN_2]);
    expect(d.taker_suggestion).toEqual({role: 'destinatario', reason: 'mod_frete_fob'});
    expect(d.sender?.person_id).toBe('PERSON#2');
  });

  it('remetentes diferentes deixam o remetente nulo e avisam', async () => {
    const d = await derive([KEY_A, KEY_IN]);
    expect(d.sender).toBeNull();
    expect(d.taker_suggestion).toBeNull();
    expect(d.warnings.map((w) => w.code)).toEqual(['mixed_sender', 'mixed_recipient', 'taker_ambiguous']);
  });
});

describe('tomador', () => {
  it('segue a sugestão até o usuário escolher', async () => {
    const d = await derive([KEY_A, KEY_B]);
    expect(effectiveTakerRole(ready(), d)).toBe('remetente');
    expect(effectiveTakerRole(ready({takerRole: 'destinatario'}), d)).toBe('destinatario');
    expect(effectiveTakerRole(ready(), undefined)).toBeNull();
  });

  it('expedidor, recebedor e outro exigem pessoa do cadastro', async () => {
    const d = await derive([KEY_A, KEY_B]);
    expect(cteMissing(ready({takerRole: 'outro'}), d, false)).toContain('Escolha a pessoa que paga o frete');
    expect(cteMissing(ready({takerRole: 'outro', takerPerson: person('PERSON#1', 'Cliente')}), d, false)).toEqual([]);
  });
});

describe('CFOP da prestação', () => {
  it('filtra pelo escopo da prestação', async () => {
    const d = await derive([KEY_A, KEY_B]); // SP para MG
    const inter = cteCfopOptions(d.origin, d.destination).map((o) => o.value);
    expect(inter.length).toBeGreaterThan(0);
    expect(inter.every((c) => c.startsWith('6'))).toBe(true);
    const sp = {uf: 'SP', city_ibge: '3550308', city: 'São Paulo'};
    expect(cteCfopOptions(sp, sp).every((o) => o.value.startsWith('5'))).toBe(true);
    expect(cteCfopOptions(null, null).some((o) => o.value === '5353')).toBe(true);
  });
});

describe('o que falta para emitir', () => {
  it('diz o que falta em ordem', async () => {
    expect(cteMissing(emptyCteForm('SN'), undefined, false)).toEqual(['Escolha ao menos uma nota']);
    expect(cteMissing(ready(), undefined, true)).toEqual(['Aguarde a leitura das notas']);
    const d = await derive([KEY_A, KEY_IN]);
    expect(cteMissing(ready({freightTotal: '', cfop: '', icms: {cst: '00', p_icms: '', p_red_bc: ''}}), d, false)).toEqual([
      'Escolha quem paga o frete',
      'Informe o valor do frete',
      'Escolha o CFOP da prestação',
      'Informe a alíquota do ICMS',
      'Escolha o remetente',
      'Escolha o destinatário',
    ]);
  });

  it('override resolve o remetente que as notas não deram', async () => {
    const d = await derive([KEY_A, KEY_IN]);
    const state = ready({
      takerRole: 'remetente',
      overrides: {sender: person('PERSON#2', 'Fornecedor Modelo ME'), recipient: person('PERSON#1', 'Cliente Exemplo LTDA')},
    });
    expect(cteMissing(state, d, false)).toEqual([]);
  });
});

describe('payload do POST /v1.0/ctes', () => {
  it('manda só decisões e overrides, e passa no schema', async () => {
    const d = await derive([KEY_A, KEY_B]);
    const body = buildCteEmitBody(ready({
      components: [{id: '1', name: 'Pedágio', value: '120'}],
      icms: {cst: '20', p_icms: '12', p_red_bc: '10'},
      overrides: {total_weight_kg: '400.5', destination: '3304557', predominant_product: ' Eletrônicos '},
    }), d);
    expect(body).toEqual({
      nfe_keys: [KEY_A, KEY_B],
      cfop: '6353',
      taker: {role: 'remetente'},
      freight: {total: '1850.00', components: [{name: 'Pedágio', value: '120.00'}]},
      icms: {cst: '20', p_icms: '12.00', p_red_bc: '10.00'},
      overrides: {
        cargo: {total_weight_kg: '400.5', predominant_product: 'Eletrônicos'},
        destination: {city_ibge: '3304557'},
      },
    });
    expect(cteEmitBodySchema.safeParse(body).success).toBe(true);
  });

  it('tomador do cadastro leva person_id; SN não leva alíquota; sem override não há overrides', async () => {
    const d = await derive([KEY_A, KEY_B]);
    const body = buildCteEmitBody(ready({takerRole: 'outro', takerPerson: person('PERSON#1', 'Cliente')}), d);
    expect(body.taker).toEqual({role: 'outro', person_id: 'PERSON#1'});
    expect(body.icms).toEqual({cst: 'SN'});
    expect(body.overrides).toBeUndefined();
    expect(body.freight.components).toBeUndefined();
  });

  it('o schema recusa componentes acima do total e alíquota ausente', () => {
    const base = {nfe_keys: [KEY_A], cfop: '5353', taker: {role: 'remetente'}, icms: {cst: 'SN'}};
    expect(cteEmitBodySchema.safeParse({...base, freight: {total: '100.00', components: [{name: 'GRIS', value: '100.00'}]}}).success).toBe(false);
    expect(cteEmitBodySchema.safeParse({...base, freight: {total: '100.00'}, icms: {cst: '00'}}).success).toBe(false);
    expect(cteEmitBodySchema.safeParse({...base, freight: {total: '100.00'}, taker: {role: 'outro'}}).success).toBe(false);
    expect(cteEmitBodySchema.safeParse({...base, freight: {total: '100.00'}, cfop: ''}).success).toBe(false);
    expect(cteEmitBodySchema.safeParse({...base, freight: {total: '100.00'}}).success).toBe(true);
  });
});

describe('falha da emissão', () => {
  it('rntrc_required vira mensagem com link para a configuração do MDF-e', () => {
    const err = new ApiError(422, 'Cadastre o RNTRC da transportadora.', {code: 'rntrc_required'});
    expect(cteEmitFailure(err)).toEqual({
      message: 'Cadastre o RNTRC da transportadora.',
      action: {label: 'Cadastrar RNTRC', href: '/fiscal-config?tab=mdfe'},
    });
  });

  it('outros 422 seguem a mensagem do servidor, sem ação', () => {
    const err = new ApiError(422, 'Informe o CFOP.', {code: 'cfop_required'});
    expect(cteEmitFailure(err)).toEqual({message: 'Informe o CFOP.'});
  });
});
