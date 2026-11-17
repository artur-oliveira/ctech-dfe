import {describe, expect, it} from 'vitest';
import {bpeTicketSchema, emptyBpeTicket, toBpeTicketBody, toOffsetDateTime, toTaTax, type BpeTicketForm} from '@/lib/schemas/bpe';
import {bpeTmSchema, emptyBpeTm, toBpeTmBody, type BpeTmForm} from '@/lib/schemas/bpe-tm';
import {BPE_KEY_PATTERN} from '@/lib/constants/bpe';

function normal(): BpeTicketForm {
  const f = emptyBpeTicket('normal');
  f.origin = {uf: 'SP', ibge_code: '3550308'};
  f.destination = {uf: 'RJ', ibge_code: '3304557'};
  f.passage = {...f.passage, from_code: '3550308', from_name: 'SAO PAULO', to_code: '3304557', to_name: 'RIO DE JANEIRO', boarding: '2026-11-10T08:00', validity: '2026-11-20T08:00'};
  f.trips = [{...f.trips[0], route_code: 'L1', route_name: 'SAO PAULO X RIO', departure: '2026-11-10T08:00', seat: '12'}];
  f.components = [{type: '01', value: '100.00'}];
  f.payments = [{type: '01', value: '100.00', desc: ''}];
  return f;
}

function ta(): BpeTicketForm {
  const f = emptyBpeTicket('ta');
  f.origin = {uf: 'SP', ibge_code: '3550308'};
  f.destination = {uf: 'RJ', ibge_code: '3304557'};
  f.passage = {...f.passage, boarding: '2026-11-10T08:00', validity: '2026-11-20T08:00'};
  f.trips = [{...f.trips[0], flight_number: '1234', carrier: 'LA', from_airport: 'GRU', to_airport: 'SDU', departure: '2026-11-10T08:00'}];
  f.components = [{type: '01', value: '300.00'}];
  f.payments = [{type: '06', value: '300.00', desc: ''}];
  f.tax = {...f.tax, class_trib: '000001', cbs_rate: '8.8'};
  return f;
}

describe('toTaTax', () => {
  it('só envia alíquota onde a lei do ano não a fixa e nunca em classe sem tributo', () => {
    const f = ta();
    f.tax = {...f.tax, ibs_uf_rate: '0.1', ibs_mun_rate: '0.1', cbs_rate: '8.8'};
    expect(toTaTax(f.tax, 2026)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: undefined});
    expect(toTaTax(f.tax, 2027)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: '8.8'});
    expect(toTaTax(f.tax, 2029)).toMatchObject({ibs_uf_rate: '0.1', ibs_mun_rate: '0.1', cbs_rate: '8.8'});
    f.tax.class_trib = '410004';
    expect(toTaTax(f.tax, 2029)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: undefined});
  });
});

describe('bpeTicketSchema', () => {
  it('aceita o bilhete comum e o TA completos', () => {
    expect(bpeTicketSchema.safeParse(normal()).success).toBe(true);
    expect(bpeTicketSchema.safeParse(ta()).success).toBe(true);
  });

  it('desconto pede tipo e descrição; tipo 99 pede o código', () => {
    const f = normal();
    f.discount = '10';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.discount_type = '02';
    f.discount_desc = 'Idoso';
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    f.discount_type = '99';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });

  it('exige a alíquota do ICMS 00 e 90 e a data da conexão', () => {
    const f = normal();
    f.icms = {cst: '00', rate: ''};
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.icms = {cst: '00', rate: '12'};
    f.trips[0].leg_type = '3';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });

  it('o TA exige classificação da tabela e aeroportos', () => {
    const f = ta();
    f.tax.class_trib = '999999';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.tax.class_trib = '000001';
    f.trips[0].from_airport = 'G';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });

  it('a classificação manda no indicador de doação, no estorno e na compra governamental', () => {
    const f = ta();
    f.tax.class_trib = '410003';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false); // exige doação
    f.tax.donation = true;
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    f.tax.class_trib = '410026';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false); // exige estorno
    f.tax.reversal = {ibs: '1.00', cbs: '2.00'};
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    f.tax.class_trib = '410001';
    f.tax.donation = true;
    expect(bpeTicketSchema.safeParse(f).success).toBe(false); // veda doação
  });

  it('compra governamental valida ente, operação e chaves por operação', () => {
    const key = '35261112345678000190630010000000101012345671';
    const f = ta();
    f.tax.government = {enabled: true, entity: '1', reducer: '0', operation: '2', references: ''};
    expect(bpeTicketSchema.safeParse(f).success).toBe(false); // operação 2 pede uma chave
    f.tax.government.references = key;
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    f.tax.government.operation = '1';
    expect(bpeTicketSchema.safeParse(f).success).toBe(true); // texto sobrando é ignorado
    expect(toTaTax(f.tax).government).toEqual({entity: '1', reducer: '0', operation: '1', references: []});
    f.tax.government = {enabled: true, entity: '', reducer: '0', operation: '', references: ''};
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });

  it('a validade não pode ser anterior ao embarque', () => {
    const f = normal();
    f.passage.validity = '2026-11-09T08:00';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });
});

describe('comprador opcional', () => {
  const buyer = (over: Partial<BpeTicketForm['buyer']> = {}): BpeTicketForm['buyer'] => ({
    enabled: true, name: 'Agencia Exemplo', doc_kind: 'cnpj', document: '11.222.333/0001-81', ie: '', street: 'Rua A', number: '10',
    complement: '', neighborhood: 'Centro', ibge_code: '3550308', city: 'Sao Paulo', postal_code: '01001-000', uf: 'SP', ...over,
  });

  it('desligado não vai no corpo e não valida os campos', () => {
    const f = normal();
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    expect(toBpeTicketBody(f).buyer).toBeUndefined();
  });

  it('ligado exige documento válido e endereço, e monta o corpo com um só documento', () => {
    const f = normal();
    f.buyer = buyer({document: '11.222.333/0001-80'});
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.buyer = buyer();
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    const b = toBpeTicketBody(f).buyer;
    expect(b).toMatchObject({name: 'Agencia Exemplo', cnpj: '11222333000181', address: {postal_code: '01001000', uf: 'SP'}});
    expect(b?.cpf).toBeUndefined();
    expect(b?.foreign_id).toBeUndefined();
  });

  it('identificador estrangeiro vai como foreign_id', () => {
    const f = normal();
    f.buyer = buyer({doc_kind: 'foreign', document: 'AB123456'});
    expect(toBpeTicketBody(f).buyer).toMatchObject({foreign_id: 'AB123456'});
  });
});

describe('toBpeTicketBody', () => {
  it('monta o corpo comum com data-hora e deslocamento, sem campos vazios', () => {
    const body = toBpeTicketBody(normal());
    expect(body.variant).toBe('normal');
    if (body.variant !== 'normal') return;
    expect(body.passage.boarding).toMatch(/^2026-11-10T08:00:00[+-]\d{2}:\d{2}$/);
    expect(body.discount).toBeUndefined();
    expect(body.icms).toEqual({cst: '40'});
    expect(body.trips[0]).toMatchObject({route_code: 'L1', seat: '12'});
    expect(body.passage.passenger).toBeUndefined();
  });

  it('monta o corpo do TA com voos e IBS e CBS no lugar do ICMS', () => {
    const body = toBpeTicketBody(ta());
    expect(body.variant).toBe('ta');
    if (body.variant !== 'ta') return;
    expect(body.trips[0]).toMatchObject({flight_number: '1234', carrier: 'LA', from_airport: 'GRU'});
    expect(body.tax).toMatchObject({class_trib: '000001'});
    expect('cst' in body.tax).toBe(false);
    expect('icms' in body).toBe(false);
  });

  it('toOffsetDateTime sempre devolve o deslocamento', () => {
    expect(toOffsetDateTime('2026-11-10T08:00')).toMatch(/:00[+-]\d{2}:\d{2}$/);
  });
});

describe('bpeTmSchema', () => {
  function tm(): BpeTmForm {
    const f = emptyBpeTm();
    f.cfop = '5353';
    f.competence = '2026-11-01';
    f.groups[0] = {
      origin_uf: 'SP', dest_uf: '', plate: '', prefix: '',
      items: [{origin_ibge_code: '3550308', dest_ibge_code: '', passengers: '10', value: '35.00', components: [{name: 'Tarifa', quantity: '10'}], icms: {cst: '40', rate: ''}}],
    };
    return f;
  }

  it('aceita a apuração completa e monta o corpo', () => {
    expect(bpeTmSchema.safeParse(tm()).success).toBe(true);
    const body = toBpeTmBody(tm());
    expect(body.variant).toBe('tm');
    expect(body.groups[0].items[0].components).toEqual([{name: 'Tarifa', quantity: 10}]);
    expect(body.groups[0].dest_uf).toBeUndefined();
  });

  it('recusa CFOP terminado em 00 e nome de componente com mais de 15 caracteres', () => {
    const f = tm();
    f.cfop = '5300';
    expect(bpeTmSchema.safeParse(f).success).toBe(false);
    f.cfop = '5353';
    f.groups[0].items[0].components[0].name = 'x'.repeat(16);
    expect(bpeTmSchema.safeParse(f).success).toBe(false);
  });
});

describe('BPE_KEY_PATTERN', () => {
  it('aceita 44 caracteres alfanuméricos e recusa o resto', () => {
    expect(BPE_KEY_PATTERN.test('3'.repeat(44))).toBe(true);
    expect(BPE_KEY_PATTERN.test('../' + '3'.repeat(41))).toBe(false);
  });
});
