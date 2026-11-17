import {describe, expect, it} from 'vitest';
import {emptyNfcomForm, nfcomEmitSchema, toNfcomEmit, type NfcomFormData} from '@/lib/schemas/nfcom';
import {NFCOM_CLASS_CODE_LENGTH, NFCOM_ICMS_OPTIONS, NFCOM_SUBSCRIBER_TYPES, NFCOM_UNIT_OPTIONS} from '@/lib/constants/nfcom';

function valid(): NfcomFormData {
  const f = emptyNfcomForm();
  f.customer = {
    ...f.customer, name: 'Cliente Exemplo', document_kind: 'cnpj', document: '33.000.167/0001-01', ie_indicator: '9',
    address: {street: 'Rua B', number: '20', neighborhood: 'Bairro', ibge_code: '3550308', city: 'Sao Paulo', postal_code: '01002-000', uf: 'SP', complement: '', phone: '', email: ''},
  };
  f.subscriber = {...f.subscriber, code: 'ASS-001', type: '1', service_used: '6'};
  f.items = [{
    code: 'PLANO100', description: 'Internet 100 Mbps', class_code: '0100301', cfop: '5307', unit: '4', quantity: '1', unit_value: '100.00',
    discount: '', other_charges: '', icms_cst: '00', icms_rate: '18', pis_cst: '01', pis_rate: '0.65', cofins_cst: '01', cofins_rate: '3',
  }];
  f.invoice = {competence: '202611', due_date: '2026-12-10', bar_code: '12345678901234567890'};
  return f;
}

describe('nfcomEmitSchema', () => {
  it('aceita um formulário completo', () => {
    expect(nfcomEmitSchema.safeParse(valid()).success).toBe(true);
  });

  it('exige a fatura, salvo em serviço pré-pago', () => {
    const f = valid();
    f.invoice = {competence: '', due_date: '', bar_code: ''};
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
    f.prepaid = true;
    expect(nfcomEmitSchema.safeParse(f).success).toBe(true);
  });

  it('exige o CFOP, salvo em ICMS sem CST (que o proíbe)', () => {
    const f = valid();
    f.items[0].cfop = '';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
    f.items[0].icms_cst = 'sem_cst';
    f.items[0].icms_rate = '';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(true);
    f.items[0].cfop = '5307';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
  });

  it('exige a alíquota do ICMS 00 e 90', () => {
    const f = valid();
    f.items[0].icms_rate = '';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
  });

  it('exige IE do contribuinte e proíbe IE do não contribuinte', () => {
    const f = valid();
    f.customer.ie_indicator = '1';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
    f.customer.ie = '110042490114';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(true);
    f.customer.ie_indicator = '9';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
  });

  it('recusa documento do cliente inválido e fim de contrato antes do início', () => {
    const f = valid();
    f.customer.document = '11.111.111/1111-11';
    expect(nfcomEmitSchema.safeParse(f).success).toBe(false);
    const g = valid();
    g.subscriber.contract_start = '2026-02-01';
    g.subscriber.contract_end = '2026-01-01';
    expect(nfcomEmitSchema.safeParse(g).success).toBe(false);
  });
});

describe('toNfcomEmit', () => {
  it('monta o corpo da API: só dígitos nos documentos e sem campos vazios', () => {
    const body = toNfcomEmit(valid());
    expect(body.customer.cnpj).toBe('33000167000101');
    expect(body.customer.cpf).toBeUndefined();
    expect(body.customer.address.postal_code).toBe('01002000');
    expect(body.customer.address.complement).toBeUndefined();
    expect(body.items[0]).toMatchObject({class_code: '0100301', cfop: '5307', icms: {cst: '00', rate: '18'}, pis: {cst: '01', rate: '0.65'}});
    expect(body.items[0].discount).toBeUndefined();
    expect(body.invoice).toEqual({competence: '202611', due_date: '2026-12-10', bar_code: '12345678901234567890'});
    expect(body.prepaid).toBeUndefined();
  });

  it('em pré-pago não envia a fatura', () => {
    const f = valid();
    f.prepaid = true;
    expect(toNfcomEmit(f).invoice).toBeUndefined();
    expect(toNfcomEmit(f).prepaid).toBe(true);
  });
});

describe('constantes da NFCom', () => {
  it('espelham os valores do leiaute', () => {
    expect(NFCOM_CLASS_CODE_LENGTH).toBe(7);
    expect(NFCOM_UNIT_OPTIONS.map((o) => o.value)).toEqual(['1', '2', '3', '4']);
    expect(NFCOM_SUBSCRIBER_TYPES.map((o) => o.value)).toEqual(['1', '2', '3', '4', '5', '6', '7', '8', '99']);
    expect(NFCOM_ICMS_OPTIONS.map((o) => o.value)).toEqual(['00', '40', '41', '51', '90', 'sn', 'sem_cst']);
  });
});
