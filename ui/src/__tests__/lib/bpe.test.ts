import {describe, expect, it} from 'vitest';
import {bpeTicketSchema, emptyBpeTicket, toBpeTicketBody, toOffsetDateTime, type BpeTicketForm} from '@/lib/schemas/bpe';
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
  f.tax = {...f.tax, cst: '000', class_trib: '000001', cbs_rate: '0.9'};
  return f;
}

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

  it('o TA exige CST de 3 dígitos, classificação de 6 e aeroportos', () => {
    const f = ta();
    f.tax.cst = '00';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.tax.cst = '000';
    f.trips[0].from_airport = 'G';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
  });

  it('a validade não pode ser anterior ao embarque', () => {
    const f = normal();
    f.passage.validity = '2026-11-09T08:00';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
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
    expect(body.tax).toMatchObject({cst: '000', class_trib: '000001', cbs_rate: '0.9'});
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
