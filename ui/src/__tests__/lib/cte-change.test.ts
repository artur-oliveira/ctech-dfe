import {describe, expect, it} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {
  buildChargeBody,
  chargeMissing,
  cteChangeBlocker,
  cteDocFailure,
  cteKind,
  daysSince,
  substituteWindow,
  type CteChargeState,
} from '@/lib/utils/cte';
import {
  buildCteOsBody,
  cteOsMissing,
  emptyCteOs,
  emptyDocLine,
  emptyGtve,
  gtveTotalCloses,
  osInssRequired,
  osRegistrationKind,
  tripAtToIso,
  type CteOsState,
} from '@/lib/utils/cte-os';
import {cteComplementBodySchema, cteOsBodySchema, cteSubstituteBodySchema} from '@/lib/schemas/cte';
import type {CteListOut, PersonItemOut} from '@/lib/types/api';

const KEY = '3'.repeat(44);
const OTHER_KEY = '4'.repeat(44);
const charge = (over: Partial<CteChargeState> = {}): CteChargeState => ({
  freightTotal: '150', components: [], icms: {cst: 'SN', p_icms: '', p_red_bc: ''}, cfop: '6353', ibsCbs: null, authorizedXml: [], ...over,
});
const person = (doc: string): PersonItemOut => ({sk: 'p1', cpf_or_cnpj: doc, name: 'Tomador'} as PersonItemOut);
const place = (uf: string) => ({city_ibge: uf === 'SP' ? '3550308' : '3106200', city: 'X', uf});

describe('espécie do documento', () => {
  it('CT-e OS tem model 67; complemento e substituto vêm de cte_type; o resto é normal', () => {
    expect(cteKind({model: '67'})).toBe('os');
    expect(cteKind({cte_type: 'complement'})).toBe('complement');
    expect(cteKind({cte_type: 'substitute'})).toBe('substitute');
    expect(cteKind({})).toBe('normal');
  });

  it('só o autorizado, do emitente, normal ou substituto, aceita complemento e substituto', () => {
    const doc = (o: Partial<CteListOut>) => ({status: 'authorized', ...o} as CteListOut);
    expect(cteChangeBlocker(doc({}), true)).toBeNull();
    expect(cteChangeBlocker(doc({cte_type: 'substitute'}), true)).toBeNull();
    expect(cteChangeBlocker(doc({cte_type: 'complement'}), true)).toMatch(/complementar/);
    expect(cteChangeBlocker(doc({model: '67'}), true)).toMatch(/CT-e OS/);
    expect(cteChangeBlocker(doc({status: 'pending'}), true)).toMatch(/autorizado/);
    expect(cteChangeBlocker(doc({}), false)).toMatch(/emitente/);
  });
});

describe('prazo do substituto', () => {
  const now = new Date('2026-11-08T12:00:00Z');
  it('conta dias inteiros desde a emissão', () => {
    expect(daysSince('2026-11-01T12:00:00Z', now)).toBe(7);
    expect(daysSince(null, now)).toBeNull();
  });

  it('livre até 60 dias, SEFAZ decide até 67, recusado depois', () => {
    expect(substituteWindow(null)).toBe('open');
    expect(substituteWindow(60)).toBe('open');
    expect(substituteWindow(61)).toBe('sefaz');
    expect(substituteWindow(67)).toBe('sefaz');
    expect(substituteWindow(68)).toBe('expired');
  });
});

describe('valor e imposto compartilhados', () => {
  it('chargeMissing nomeia o valor e pede CFOP e CST', () => {
    const empty = charge({freightTotal: '', cfop: '', icms: {cst: '', p_icms: '', p_red_bc: ''}});
    expect(chargeMissing(empty, 'valor complementar')).toEqual([
      'Informe o valor do valor complementar', 'Escolha o CFOP da prestação', 'Escolha o CST do ICMS',
    ]);
    expect(chargeMissing(charge(), 'frete')).toEqual([]);
    expect(chargeMissing(charge({icms: {cst: '00', p_icms: '', p_red_bc: ''}}), 'frete')).toEqual(['Informe a alíquota do ICMS']);
  });

  it('buildChargeBody emite valor com 2 casas, componentes, ICMS com alíquota e autXML', () => {
    const body = buildChargeBody(charge({
      freightTotal: '150.5', components: [{id: 'a', name: 'Pedágio', value: '20'}],
      icms: {cst: '00', p_icms: '12', p_red_bc: ''}, authorizedXml: [{doc: '11222333000181', name: 'X'}],
    }));
    expect(body).toEqual({
      cfop: '6353',
      freight: {total: '150.50', components: [{name: 'Pedágio', value: '20.00'}]},
      icms: {cst: '00', p_icms: '12.00'},
      authorized_xml: [{doc: '11222333000181'}],
    });
  });
});

describe('mapeamento dos 422 para pt-BR', () => {
  const err = (code: string, detail = 'servidor') => new ApiError(422, detail, {code});
  it('troca o código por texto de quem emite e mantém o do servidor quando o código é desconhecido', () => {
    expect(cteDocFailure(err('substitution_window_expired'), 'x').message).toMatch(/60 dias/);
    expect(cteDocFailure(err('os_taf_required'), 'x').message).toMatch(/TAF/);
    expect(cteDocFailure(err('desconhecido', 'texto do servidor'), 'x').message).toBe('texto do servidor');
    expect(cteDocFailure(new Error('rede'), 'Falhou').message).toBe('rede');
  });

  it('RNTRC ausente ainda leva à configuração', () => {
    expect(cteDocFailure(err('rntrc_required'), 'x').action?.href).toBe('/fiscal-config?tab=mdfe');
  });
});

describe('schemas de complemento e substituto', () => {
  const base = {freight: {total: '10.00'}, cfop: '6353'};
  it('complemento: 1 a 10 chaves sem repetição', () => {
    expect(cteComplementBodySchema.safeParse({...base, original_keys: [KEY]}).success).toBe(true);
    expect(cteComplementBodySchema.safeParse({...base, original_keys: []}).success).toBe(false);
    expect(cteComplementBodySchema.safeParse({...base, original_keys: [KEY, KEY]}).success).toBe(false);
  });

  it('substituto: taker só com change_taker, e outro exige person_id', () => {
    const ok = {...base, original_key: KEY};
    expect(cteSubstituteBodySchema.safeParse(ok).success).toBe(true);
    expect(cteSubstituteBodySchema.safeParse({...ok, taker: {role: 'remetente'}}).success).toBe(false);
    expect(cteSubstituteBodySchema.safeParse({...ok, change_taker: true}).success).toBe(false);
    expect(cteSubstituteBodySchema.safeParse({...ok, change_taker: true, taker: {role: 'outro'}}).success).toBe(false);
    expect(cteSubstituteBodySchema.safeParse({...ok, change_taker: true, taker: {role: 'outro', person_id: 'p'}}).success).toBe(true);
  });
});

describe('CT-e OS: regras', () => {
  it('pessoas: mesma UF pede registro estadual, entre UFs pede TAF; sem trajeto a escolha é do usuário', () => {
    expect(osRegistrationKind('people', place('SP'), place('SP'))).toBe('state_registration');
    expect(osRegistrationKind('people', place('SP'), place('MG'))).toBe('taf');
    expect(osRegistrationKind('people', place('SP'), null)).toBeNull();
    expect(osRegistrationKind('baggage', place('SP'), place('MG'))).toBeNull();
  });

  it('INSS é obrigatório em pessoas com tomador CNPJ', () => {
    expect(osInssRequired({serviceType: 'people', taker: person('11222333000181')})).toBe(true);
    expect(osInssRequired({serviceType: 'people', taker: person('12345678909')})).toBe(false);
    expect(osInssRequired({serviceType: 'values', taker: person('11222333000181')})).toBe(false);
  });

  it('a soma das GTV-e fecha com o frete com tolerância de R$ 0,10', () => {
    const g = emptyGtve();
    g.components[0] = {...g.components[0], type: '1', value: '100.00'};
    expect(gtveTotalCloses([g], '100.10')).toBe(true);
    expect(gtveTotalCloses([g], '100.11')).toBe(false);
  });

  it('data da viagem sai em RFC 3339 sem milissegundos', () => {
    expect(tripAtToIso('2027-01-10T08:30')).toMatch(/^2027-01-10T\d{2}:30:00Z$/);
  });
});

describe('CT-e OS: o que falta e o corpo por tipo', () => {
  const people = (over: Partial<CteOsState> = {}): CteOsState => ({
    ...emptyCteOs(), serviceType: 'people', taker: person('12345678909'), description: 'Fretamento',
    origin: '3550308', destination: '3106200', registrationKind: 'taf', taf: '123456789012',
    charterType: '2', ...over,
  });

  it('sem tipo, pede o tipo', () => {
    expect(cteOsMissing(emptyCteOs(), charge())).toEqual(['Escolha o tipo de serviço']);
  });

  it('pessoas completas não têm pendência; eventual pede a viagem; PJ pede INSS', () => {
    expect(cteOsMissing(people(), charge())).toEqual([]);
    expect(cteOsMissing(people({charterType: '1'}), charge())).toContain('Informe a data e a hora da viagem');
    expect(cteOsMissing(people({taker: person('11222333000181')}), charge())).toContain('Informe o INSS dos tributos federais');
    expect(cteOsMissing(people({taf: '123'}), charge())).toContain('Informe o TAF com 12 dígitos');
  });

  it('valores: pede GTV-e e que a soma feche com o frete', () => {
    const base = {...emptyCteOs(), serviceType: 'values' as const, taker: person('11222333000181'), description: 'Malote', destination: '3106200'};
    expect(cteOsMissing(base, charge())).toContain('Informe ao menos uma GTV-e');
    const g = {...emptyGtve(), key: KEY};
    g.components[0] = {...g.components[0], type: '4', value: '100'};
    expect(cteOsMissing({...base, gtve: [g]}, charge({freightTotal: '150'}))).toContain('A soma das GTV-e precisa fechar com o frete');
    expect(cteOsMissing({...base, gtve: [g]}, charge({freightTotal: '100'}))).toEqual([]);
  });

  it('excesso de bagagem: pede documento; BP-e precisa de 44 dígitos', () => {
    const base = {...emptyCteOs(), serviceType: 'baggage' as const, taker: person('11222333000181'), description: 'Bagagem',
      registrationKind: 'state_registration' as const, stateRegistration: '1'.repeat(25)};
    expect(cteOsMissing(base, charge())).toContain('Informe ao menos um documento referenciado');
    expect(cteOsMissing({...base, docRefs: [{...emptyDocLine(), bpe_key: '12'}]}, charge())).toContain('Complete os documentos referenciados');
    expect(cteOsMissing({...base, docRefs: [{...emptyDocLine(), bpe_key: KEY}]}, charge())).toEqual([]);
  });

  it('o corpo de pessoas leva modal, fretamento e trajeto, e passa no schema', () => {
    const body = buildCteOsBody(people({quantity: '40', routeUfs: ['SP', 'MG'], federal: {...emptyCteOs().federal, inss: '12.5'}}), charge());
    expect(body).toMatchObject({
      service_type: 'people', taker: {person_id: 'p1'}, service: {description: 'Fretamento', quantity: '40'},
      origin: {city_ibge: '3550308', uf: 'SP'}, destination: {city_ibge: '3106200', uf: 'MG'}, route_ufs: ['SP', 'MG'],
      road: {taf: '123456789012', charter: {type: 2}}, federal_taxes: {inss: '12.50'}, cfop: '6353',
    });
    expect(body.gtve).toBeUndefined();
    expect(body.doc_refs).toBeUndefined();
    expect(cteOsBodySchema.safeParse(body).success).toBe(true);
  });

  it('o corpo de valores não leva modal; o de bagagem leva documentos', () => {
    const g = {...emptyGtve(), key: OTHER_KEY};
    g.components[0] = {...g.components[0], type: '6', value: '100', name: ' Escolta '};
    const values = buildCteOsBody({...emptyCteOs(), serviceType: 'values', taker: person('11222333000181'), description: 'Malote', destination: '3106200', gtve: [g]}, charge({freightTotal: '100'}));
    expect(values.road).toBeUndefined();
    expect(values.gtve).toEqual([{key: OTHER_KEY, components: [{type: 6, value: '100.00', name: 'Escolta'}]}]);
    expect(cteOsBodySchema.safeParse(values).success).toBe(true);

    const bag = buildCteOsBody({...emptyCteOs(), serviceType: 'baggage', taker: person('11222333000181'), description: 'Bagagem',
      registrationKind: 'state_registration', stateRegistration: '1'.repeat(25),
      docRefs: [{...emptyDocLine(), bpe_key: KEY}, {...emptyDocLine(), mode: 'other', number: '99', issued_on: '2026-11-01'}]}, charge());
    expect(bag.doc_refs).toEqual([{bpe_key: KEY}, {number: '99', issued_on: '2026-11-01'}]);
    expect(bag.road).toEqual({state_registration: '1'.repeat(25)});
    expect(cteOsBodySchema.safeParse(bag).success).toBe(true);
  });
});
