import {describe, expect, it} from 'vitest';
import {ciotDeclareSchema, emptyCiotForm, formFromCiot, toDeclareBody, toOperation} from '@/lib/schemas/ciots';
import {ciotsFixture, CIOT_CARRIER_DOC} from '@/lib/mock/fixtures';

const ORG = '11222333000181';

/** Lotação completa e válida. */
const valid = () => ({
  ...emptyCiotForm('hom'),
  carrier_doc: CIOT_CARRIER_DOC, carrier_rntrc: '012345678', hirer_doc: ORG, recipient_doc: ORG,
  freight_value: '4800.00', trip_start: '2026-07-12', trip_end: '2026-07-14',
  vehicles: [{plate: 'ABC1D23', rntrc: '', axles: '3', automotor: true, tractor: true}],
  origin_ibge: '3550308', destination_ibge: '3304557', distance_km: '430',
  cargo_ncm: '10019900', cargo_weight: '24000.00', cargo_type: '1', credited_doc: CIOT_CARRIER_DOC,
  bank_code: '341', branch: '0001', account: '12345-6',
});

const issues = (data: object) => {
  const r = ciotDeclareSchema.safeParse(data);
  return r.success ? [] : r.error.issues.map((i) => i.path.join('.'));
};

describe('ciotDeclareSchema', () => {
  it('aceita uma lotação completa', () => {
    expect(issues(valid())).toEqual([]);
  });

  it('recusa CPF/CNPJ com dígito verificador errado', () => {
    expect(issues({...valid(), carrier_doc: '52998224726'})).toContain('carrier_doc');
    expect(issues({...valid(), hirer_doc: '11222333000182'})).toContain('hirer_doc');
  });

  it('RNTRC tem 8 ou 9 dígitos', () => {
    expect(issues({...valid(), carrier_rntrc: '1234567'})).toContain('carrier_rntrc');
    expect(issues({...valid(), carrier_rntrc: '12345678'})).toEqual([]);
  });

  it('frete maior que zero com até duas casas', () => {
    expect(issues({...valid(), freight_value: '0'})).toContain('freight_value');
    expect(issues({...valid(), freight_value: '10.123'})).toContain('freight_value');
  });

  it('exige ao menos um veículo e no máximo cinco', () => {
    expect(issues({...valid(), vehicles: []})).toContain('vehicles');
    const six = Array.from({length: 6}, (_, i) => ({plate: `AAA${i}B23`, rntrc: '', axles: '2', automotor: false, tractor: false}));
    expect(issues({...valid(), vehicles: six})).toContain('vehicles');
  });

  it('automotor tem de 2 a 4 eixos', () => {
    const v = {plate: 'ABC1D23', rntrc: '', axles: '1', automotor: true, tractor: true};
    expect(issues({...valid(), vehicles: [v]})).toContain('vehicles.0.axles');
  });

  it('lotação exige destinatário, rota, carga e início da viagem', () => {
    const found = issues({...valid(), recipient_doc: '', trip_start: '', origin_ibge: '', cargo_ncm: '', cargo_weight: '', cargo_type: ''});
    expect(found).toEqual(expect.arrayContaining(['recipient_doc', 'trip_start', 'origin_ibge', 'cargo_ncm', 'cargo_weight', 'cargo_type']));
  });

  it('TAC-Agregado dispensa destinatário, rota e carga', () => {
    const tac = {...valid(), operation_type: '3', recipient_doc: '', trip_start: '', origin_ibge: '', destination_ibge: '', cargo_ncm: '', cargo_weight: '', cargo_type: ''};
    expect(issues(tac)).toEqual([]);
  });

  it('o fim da viagem não pode ser antes do início', () => {
    expect(issues({...valid(), trip_end: '2026-07-01'})).toContain('trip_end');
  });

  it('Pix exige chave e identificador; conta exige banco, agência e conta', () => {
    expect(issues({...valid(), payment_type: '6'})).toEqual(expect.arrayContaining(['pix_key', 'pix_id']));
    expect(issues({...valid(), payment_type: '2', account: ''})).toContain('account');
    expect(issues({...valid(), payment_type: '5'})).toEqual([]);
  });

  it('a chave do MDF-e, quando vem, tem 44 dígitos', () => {
    expect(issues({...valid(), mdfe_access_key: '123'})).toContain('mdfe_access_key');
  });
});

describe('toDeclareBody', () => {
  it('monta a lotação com rota, carga (4 dígitos do NCM), indicadores e pagamento à vista', () => {
    const body = toDeclareBody({...valid(), payment_type: '5', high_performance: true});
    expect(body).toMatchObject({
      environment: 'hom', operation_type: 1, carrier_doc: CIOT_CARRIER_DOC, freight_value: '4800.00',
      routes: [{origin_ibge: '3550308', destination_ibge: '3304557', distance_km: 430}],
      cargo: {nature: '1001', weight: '24000.00', cargo_type: 1},
      indicators: {high_performance: true, empty_return: false, vehicle_composition: false},
      payment: {type: 5, indicator: 0, credited_doc: CIOT_CARRIER_DOC},
    });
    expect(body.vehicles).toEqual([{plate: 'ABC1D23', axles: 3, automotor: true, tractor: true}]);
    expect('mdfe_access_key' in body).toBe(false);
  });

  it('TAC-Agregado não leva rota, carga, destinatário nem indicadores', () => {
    const body = toDeclareBody({...valid(), operation_type: '3', payment_type: '5'});
    expect(body.routes).toBeUndefined();
    expect(body.cargo).toBeUndefined();
    expect(body.indicators).toBeUndefined();
    expect(body.recipient_doc).toBeUndefined();
  });

  it('leva Pix e o vínculo com o MDF-e', () => {
    const key = '3'.repeat(44);
    const body = toDeclareBody({...valid(), payment_type: '6', pix_key: ' a@b.dev ', pix_id: 'ID1', mdfe_access_key: key});
    expect(body.payment).toMatchObject({type: 6, pix_key: 'a@b.dev', pix_id: 'ID1'});
    expect(body.mdfe_access_key).toBe(key);
  });

  it('a retificação envia o conteúdo sem ambiente nem vínculo', () => {
    const op = toOperation(valid());
    expect('environment' in op).toBe(false);
    expect('mdfe_access_key' in op).toBe(false);
  });
});

describe('formFromCiot', () => {
  it('devolve ao formulário a declaração gravada', () => {
    const form = formFromCiot(ciotsFixture[0] as never);
    expect(form).toMatchObject({
      environment: 'prod', operation_type: '1', carrier_doc: CIOT_CARRIER_DOC, carrier_rntrc: '012345678',
      freight_value: '4800.00', origin_ibge: '3550308', distance_km: '430', cargo_ncm: '1001', cargo_type: '1',
      payment_type: '2', bank_code: '341',
    });
    expect(form.vehicles.map((v) => [v.plate, v.axles])).toEqual([['ABC1D23', '3'], ['XYZ9K87', '3']]);
  });
});
