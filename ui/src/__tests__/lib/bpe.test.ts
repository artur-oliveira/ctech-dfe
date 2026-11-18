import {describe, expect, it} from 'vitest';
import {bpeTicketSchema, emptyBpeTicket, toBpeTicketBody, toOffsetDateTime, type BpeTicketForm} from '@/lib/schemas/bpe';
import {bpeTmSchema, emptyBpeTm, toBpeTmBody, type BpeTmForm} from '@/lib/schemas/bpe-tm';
import {bpeLineSchema, bpeLineToForm, emptyBpeLine, toBpeLine, type BpeLineFormData} from '@/lib/schemas/bpe-line';
import {toTaTax} from '@/lib/schemas/bpe-tax';
import {BPE_KEY_PATTERN} from '@/lib/constants/bpe';

const KEY = '35261112345678000190630010000000101012345671';

function normal(): BpeTicketForm {
  const f = emptyBpeTicket('normal');
  f.boarding = '2026-11-10T08:00';
  f.validity = '2026-11-20T08:00';
  f.trips = [{...f.trips[0], line_id: 'BPELINE_1', departure: '2026-11-10T08:00', seat: '12'}];
  f.components = [{type: '01', value: '100.00'}];
  f.payments = [{type: '01', value: '100.00', desc: ''}];
  return f;
}

function ta(): BpeTicketForm {
  const f = emptyBpeTicket('ta');
  f.boarding = '2026-11-10T08:00';
  f.validity = '2026-11-20T08:00';
  f.passenger = {...f.passenger, mode: 'avulso', name: 'Maria Passageira', doc_type: '1', doc_number: '123456789'};
  f.trips = [{...f.trips[0], line_id: 'BPELINE_2', departure: '2026-11-10T08:00'}];
  f.components = [{type: '01', value: '300.00'}];
  f.payments = [{type: '06', value: '300.00', desc: ''}];
  return f;
}

const paths = (r: { success: boolean; error?: { issues: { path: PropertyKey[] }[] } }) => (r.success ? [] : (r.error?.issues ?? []).map((i) => i.path.join('.')));

describe('bpeTicketSchema', () => {
  it('aceita o bilhete comum (passageiro opcional) e o TA (passageiro avulso)', () => {
    expect(bpeTicketSchema.safeParse(normal()).success).toBe(true);
    expect(bpeTicketSchema.safeParse(ta()).success).toBe(true);
  });

  it('exige linha escolhida em cada trecho e a data da conexão', () => {
    const f = normal();
    f.trips[0].line_id = '';
    f.trips[0].leg_type = '3';
    expect(paths(bpeTicketSchema.safeParse(f))).toEqual(expect.arrayContaining(['trips.0.line_id', 'trips.0.connection']));
  });

  it('desconto pede tipo e descrição; tipo 99 pede o código', () => {
    const f = normal();
    f.discount = '10';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.discount_type = '02';
    f.discount_desc = 'Idoso';
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    f.discount_type = '99';
    expect(paths(bpeTicketSchema.safeParse(f))).toContain('discount_code');
  });

  it('a validade não pode ser anterior ao embarque', () => {
    const f = normal();
    f.validity = '2026-11-09T08:00';
    expect(paths(bpeTicketSchema.safeParse(f))).toContain('validity');
  });

  it('passageiro avulso exige nome e documento; texto livre é validado', () => {
    const f = normal();
    f.passenger = {mode: 'avulso', name: 'M', doc_type: '1', doc_number: '', person_id: ''};
    expect(paths(bpeTicketSchema.safeParse(f))).toEqual(expect.arrayContaining(['passenger.name', 'passenger.doc_number']));
    f.passenger = {mode: 'avulso', name: 'Maria <b>', doc_type: '1', doc_number: '1', person_id: ''};
    expect(paths(bpeTicketSchema.safeParse(f))).toContain('passenger.name');
  });

  it('passageiro cadastrado exige a pessoa; o TA normal não aceita não identificar', () => {
    const f = normal();
    f.passenger.mode = 'person';
    expect(paths(bpeTicketSchema.safeParse(f))).toContain('passenger.person_id');
    const t = ta();
    t.passenger.mode = 'none';
    expect(paths(bpeTicketSchema.safeParse(t))).toContain('passenger.mode');
  });

  it('substituição pede chave de 44 caracteres e o tipo', () => {
    const f = normal();
    f.substitution = {enabled: true, original_key: '123', type: ''};
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.substitution = {enabled: true, original_key: KEY, type: '1'};
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    expect(toBpeTicketBody(f).substitution).toEqual({original_key: KEY, type: '1'});
  });

  it('o PDF417 (só no comum) tem 50 a 1000 caracteres', () => {
    const f = normal();
    f.board_pass = 'curto';
    expect(bpeTicketSchema.safeParse(f).success).toBe(false);
    f.board_pass = 'A'.repeat(60);
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    expect(toBpeTicketBody(f).board_pass).toBe('A'.repeat(60));
  });

  it('o múltiplo exige comprador e não leva passageiro; o vinculado leva a chave e nenhum valor', () => {
    const f = ta();
    f.purchase_type = 'multiple';
    f.passenger.mode = 'none';
    expect(paths(bpeTicketSchema.safeParse(f))).toContain('buyer_person_id');
    f.buyer_person_id = 'CNPJ_11222333000181';
    expect(bpeTicketSchema.safeParse(f).success).toBe(true);
    expect(toBpeTicketBody(f)).toMatchObject({purchase_type: 'multiple', buyer_person_id: 'CNPJ_11222333000181'});

    const l = ta();
    l.purchase_type = 'linked';
    expect(paths(bpeTicketSchema.safeParse(l))).toContain('multiple_key');
    l.multiple_key = KEY;
    l.components = [];
    l.payments = [];
    expect(bpeTicketSchema.safeParse(l).success).toBe(true);
    const body = toBpeTicketBody(l);
    expect(body).toMatchObject({purchase_type: 'linked', multiple_key: KEY});
    expect(body.components).toBeUndefined();
    expect(body.payments).toBeUndefined();
  });
});

describe('toBpeTicketBody', () => {
  it('monta a venda com ids e data-hora com deslocamento, sem campos vazios nem dados de linha', () => {
    const body = toBpeTicketBody(normal());
    expect(body).toMatchObject({variant: 'normal', trips: [{line_id: 'BPELINE_1', leg_type: '1', seat: '12'}]});
    expect(body.boarding).toMatch(/^2026-11-10T08:00:00[+-]\d{2}:\d{2}$/);
    expect(body.discount).toBeUndefined();
    expect(body.passenger).toBeUndefined();
    const json = JSON.stringify(body);
    for (const gone of ['icms', 'route_code', 'origin', 'destination', 'flight_number', 'tax']) expect(json).not.toContain(`"${gone}"`);
  });

  it('o passageiro avulso vai direto, normalizado; a pessoa vai por id', () => {
    const f = ta();
    f.passenger.name = '  Maria   Passageira ';
    expect(toBpeTicketBody(f).passenger).toEqual({name: 'Maria Passageira', doc_type: '1', doc_number: '123456789'});
    f.passenger = {...f.passenger, mode: 'person', person_id: 'CPF_52998224725'};
    const body = toBpeTicketBody(f);
    expect(body.passenger).toBeUndefined();
    expect(body.passenger_person_id).toBe('CPF_52998224725');
  });

  it('o TA não leva poltrona nem plataforma; o comum não leva IATA', () => {
    const f = ta();
    f.trips[0].seat = '12';
    f.iata_number = 'ABC123';
    const body = toBpeTicketBody(f);
    expect(body.trips[0].seat).toBeUndefined();
    expect(body.iata_number).toBe('ABC123');
    const n = normal();
    n.iata_number = 'ABC123';
    expect(toBpeTicketBody(n).iata_number).toBeUndefined();
  });

  it('toOffsetDateTime sempre devolve o deslocamento', () => {
    expect(toOffsetDateTime('2026-11-10T08:00')).toMatch(/:00[+-]\d{2}:\d{2}$/);
  });
});

describe('bpeLineSchema', () => {
  function normalLine(): BpeLineFormData {
    return {
      ...emptyBpeLine('normal'), name: 'SP x RJ',
      origin: {uf: 'SP', ibge_code: '3550308', city: 'Sao Paulo'}, destination: {uf: 'RJ', ibge_code: '3304557', city: 'Rio de Janeiro'},
      route_code: 'L100', route_name: 'SAO PAULO X RIO', icms_cst: '00', icms_rate: '12',
    };
  }
  function taLine(): BpeLineFormData {
    return {
      ...emptyBpeLine('ta'), name: 'GRU x GIG',
      origin: {uf: 'SP', ibge_code: '3550308', city: 'Sao Paulo'}, destination: {uf: 'RJ', ibge_code: '3304557', city: 'Rio de Janeiro'},
      carrier: 'la', flight_number: 'LA3000', from_airport: 'gru', to_airport: 'gig',
      tax: {...emptyBpeLine('ta').tax, class_trib: '000001', cbs_rate: '8.8'},
    };
  }

  it('aceita a linha comum e mapeia só os campos dela', () => {
    const parsed = bpeLineSchema.safeParse(normalLine());
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      const body = toBpeLine(parsed.data);
      expect(body).toMatchObject({variant: 'normal', modal: 'rodoviario', route_code: 'L100', icms: {cst: '00', rate: '12'}, prefix: null});
      expect('carrier' in body || 'legs' in body || 'tax' in body).toBe(false);
    }
  });

  it('comum: municípios, código, percurso e alíquota são obrigatórios; texto e código restritos', () => {
    const f = {...normalLine(), origin: {uf: '', ibge_code: '', city: ''}, route_code: '', route_name: 'a<b', icms_rate: ''};
    expect(paths(bpeLineSchema.safeParse(f))).toEqual(expect.arrayContaining(['origin.ibge_code', 'route_code', 'route_name', 'icms_rate']));
    expect(bpeLineSchema.safeParse({...normalLine(), route_code: 'L 100'}).success).toBe(false);
  });

  it('TA: sigla e aeroportos em caixa alta, e a classificação vem da tabela', () => {
    const parsed = bpeLineSchema.safeParse(taLine());
    expect(parsed.success).toBe(true);
    if (parsed.success) expect(toBpeLine(parsed.data)).toMatchObject({variant: 'ta', carrier: 'LA', from_airport: 'GRU', to_airport: 'GIG', tax: {class_trib: '000001'}});
    const bad = taLine();
    bad.tax.class_trib = '999999';
    bad.from_airport = 'G';
    expect(paths(bpeLineSchema.safeParse(bad))).toEqual(expect.arrayContaining(['tax.class_trib', 'from_airport']));
  });

  it('TA: a classificação manda no indicador de doação, no estorno e na compra governamental', () => {
    const f = taLine();
    f.tax.class_trib = '410003';
    expect(paths(bpeLineSchema.safeParse(f))).toContain('tax.donation'); // exige doação
    f.tax.donation = true;
    expect(bpeLineSchema.safeParse(f).success).toBe(true);
    f.tax.class_trib = '410026';
    expect(paths(bpeLineSchema.safeParse(f))).toEqual(expect.arrayContaining(['tax.reversal.ibs', 'tax.reversal.cbs']));
    f.tax.reversal = {ibs: '1.00', cbs: '2.00'};
    expect(bpeLineSchema.safeParse(f).success).toBe(true);
    f.tax.class_trib = '410001';
    f.tax.donation = true;
    expect(paths(bpeLineSchema.safeParse(f))).toContain('tax.donation'); // veda doação

    const g = taLine();
    g.tax.government = {enabled: true, entity: '1', reducer: '0', operation: '2', references: ''};
    expect(paths(bpeLineSchema.safeParse(g))).toContain('tax.government.references');
    g.tax.government.references = KEY;
    expect(bpeLineSchema.safeParse(g).success).toBe(true);
  });

  it('toTaTax só envia alíquota onde a lei do ano não a fixa e nunca em classe sem tributo', () => {
    const t = {...emptyBpeLine('ta').tax, class_trib: '000001', ibs_uf_rate: '0.1', ibs_mun_rate: '0.1', cbs_rate: '8.8'};
    expect(toTaTax(t, 2026)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: undefined});
    expect(toTaTax(t, 2027)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: '8.8'});
    expect(toTaTax(t, 2029)).toMatchObject({ibs_uf_rate: '0.1', ibs_mun_rate: '0.1', cbs_rate: '8.8'});
    expect(toTaTax({...t, class_trib: '410004'}, 2029)).toMatchObject({ibs_uf_rate: undefined, cbs_rate: undefined});
  });

  it('TM: CFOP sem 00 no fim, trecho com ICMS e tipos de passageiro de até 15 caracteres', () => {
    const f: BpeLineFormData = {
      ...emptyBpeLine('tm'), name: 'Metrô L1', modal: 'ferroviario', cfop: '5353', group_origin_uf: 'SP',
      legs: [{origin_ibge_code: '3550308', dest_ibge_code: '', icms_cst: '40', icms_rate: ''}],
      passenger_types: [{name: 'Inteira'}, {name: 'Meia'}],
    };
    const parsed = bpeLineSchema.safeParse(f);
    expect(parsed.success).toBe(true);
    if (parsed.success) expect(toBpeLine(parsed.data)).toMatchObject({variant: 'tm', cfop: '5353', origin_uf: 'SP', passenger_types: ['Inteira', 'Meia'], dest_uf: null});
    expect(paths(bpeLineSchema.safeParse({...f, cfop: '5300'}))).toContain('cfop');
    expect(bpeLineSchema.safeParse({...f, passenger_types: [{name: 'x'.repeat(16)}]}).success).toBe(false);
    expect(paths(bpeLineSchema.safeParse({...f, legs: [{origin_ibge_code: '3550308', dest_ibge_code: '', icms_cst: '00', icms_rate: ''}]}))).toContain('legs.0.icms_rate');
  });

  it('a linha gravada volta para o formulário', () => {
    const parsed = bpeLineSchema.safeParse(normalLine());
    if (!parsed.success) throw new Error('linha inválida');
    const back = bpeLineToForm({pk: 'p', sk: 'BPELINE_1', ...toBpeLine(parsed.data)});
    expect(back).toMatchObject({name: 'SP x RJ', variant: 'normal', route_code: 'L100', icms_cst: '00', icms_rate: '12', origin: {ibge_code: '3550308'}});
  });
});

describe('bpeTmSchema', () => {
  function tm(): BpeTmForm {
    const f = emptyBpeTm();
    f.competence = '2026-11-01';
    f.groups[0] = {
      line_id: 'BPELINE_3', counter_id: '',
      items: [{leg: '1', trip_number: '', start_counter: '10', end_counter: '20', passengers: '10', value: '35.00', components: [{name: 'Inteira', quantity: '10'}]}],
    };
    return f;
  }

  it('aceita a apuração e monta o corpo com ids e números', () => {
    expect(bpeTmSchema.safeParse(tm()).success).toBe(true);
    const body = toBpeTmBody(tm());
    expect(body).toMatchObject({variant: 'tm', competence: '2026-11-01'});
    expect(body.groups[0]).toMatchObject({line_id: 'BPELINE_3', counter_id: undefined});
    expect(body.groups[0].items[0]).toMatchObject({leg: 1, passengers: '10', start_counter: '10', components: [{name: 'Inteira', quantity: 10}]});
    expect(JSON.stringify(body)).not.toContain('"cfop"');
  });

  it('exige linha, trecho, tipo de passageiro e números só com dígitos', () => {
    const f = tm();
    f.groups[0].line_id = '';
    f.groups[0].items[0].leg = '';
    f.groups[0].items[0].components[0].name = '';
    f.groups[0].items[0].start_counter = '1a';
    expect(paths(bpeTmSchema.safeParse(f))).toEqual(expect.arrayContaining([
      'groups.0.line_id', 'groups.0.items.0.leg', 'groups.0.items.0.components.0.name', 'groups.0.items.0.start_counter',
    ]));
  });
});

describe('BPE_KEY_PATTERN', () => {
  it('aceita 44 caracteres alfanuméricos e recusa o resto', () => {
    expect(BPE_KEY_PATTERN.test('3'.repeat(44))).toBe(true);
    expect(BPE_KEY_PATTERN.test('../' + '3'.repeat(41))).toBe(false);
  });
});
