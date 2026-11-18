import {describe, expect, it} from 'vitest';
import {
  emptyNfcomForm,
  emptyNfcomPlan,
  emptyNfcomSubscriber,
  nfcomEmitSchema,
  nfcomPlanSchema,
  nfcomSubscriberSchema,
  suggestedInvoice,
  toNfcomEmit,
  toNfcomPlan,
  toNfcomSubscriber,
  type NfcomFormData,
} from '@/lib/schemas/nfcom';
import {NFCOM_CFOP_OPTIONS, nfcomCompetenceOptions} from '@/lib/constants/nfcom';
import {cleanOptional, cleanText, FISCAL_TEXT, REF_CODE} from '@/lib/utils/fiscal-text';

const NOW = new Date(2026, 10, 17); // 17/11/2026

function valid(): NfcomFormData {
  const f = emptyNfcomForm(NOW);
  f.subscriber_id = 'NFCOMSUB_1';
  f.items[0] = {plan_id: 'NFCOMPLAN_1', quantity: '2', unit_value: '', discount: '', other_charges: ''};
  f.invoice.bar_code = '12345678901234567890';
  return f;
}

describe('nfcomEmitSchema', () => {
  it('aceita assinante, plano e fatura sugerida', () => {
    expect(nfcomEmitSchema.safeParse(valid()).success).toBe(true);
  });

  it('exige assinante e plano escolhidos', () => {
    const f = valid();
    f.subscriber_id = '';
    f.items[0].plan_id = '';
    const paths = nfcomEmitSchema.safeParse(f).error?.issues.map((i) => i.path.join('.')) ?? [];
    expect(paths).toContain('subscriber_id');
    expect(paths).toContain('items.0.plan_id');
  });

  it('quantidade positiva e decimais válidos', () => {
    const f = valid();
    f.items[0].quantity = '0';
    f.items[0].discount = 'abc';
    const paths = nfcomEmitSchema.safeParse(f).error?.issues.map((i) => i.path.join('.')) ?? [];
    expect(paths).toContain('items.0.quantity');
    expect(paths).toContain('items.0.discount');
  });

  it('pré-pago dispensa a fatura; a fatura normal pede competência, vencimento e código de barras', () => {
    const f = valid();
    f.invoice = {competence: '', due_date: '', bar_code: ''};
    expect(nfcomEmitSchema.safeParse(f).error?.issues.map((i) => i.path.join('.'))).toEqual(
      expect.arrayContaining(['invoice.competence', 'invoice.due_date', 'invoice.bar_code']));
    f.prepaid = true;
    expect(nfcomEmitSchema.safeParse(f).success).toBe(true);
  });

  it('informações adicionais seguem o texto fiscal', () => {
    const f = valid();
    f.additional_info = 'a < b';
    expect(nfcomEmitSchema.safeParse(f).error?.issues.map((i) => i.path.join('.'))).toContain('additional_info');
  });
});

describe('toNfcomEmit', () => {
  it('leva só ids e o que varia, sem campo vazio', () => {
    const body = toNfcomEmit({...valid(), additional_info: '  Obrigado   pela preferência \n'});
    expect(body.subscriber_id).toBe('NFCOMSUB_1');
    expect(body.items).toEqual([{plan_id: 'NFCOMPLAN_1', quantity: '2', unit_value: undefined, discount: undefined, other_charges: undefined}]);
    expect(body.additional_info).toBe('Obrigado pela preferência');
    expect(body.invoice?.competence).toBe('202611');
    expect(JSON.stringify(body)).not.toContain('customer');
  });

  it('pré-pago sai sem fatura', () => {
    const body = toNfcomEmit({...valid(), prepaid: true});
    expect(body.invoice).toBeUndefined();
    expect(body.prepaid).toBe(true);
  });
});

describe('sugestões da fatura', () => {
  it('competência do mês e vencimento em 10 dias', () => {
    expect(suggestedInvoice(NOW)).toEqual({competence: '202611', due_date: '2026-11-27', bar_code: ''});
  });

  it('as competências oferecidas vão do mês corrente para trás', () => {
    const options = nfcomCompetenceOptions(NOW, 3);
    expect(options.map((o) => o.value)).toEqual(['202611', '202610', '202609']);
    expect(options[0].label).toBe('novembro de 2026');
  });
});

describe('nfcomSubscriberSchema', () => {
  const ok = () => ({...emptyNfcomSubscriber(), name: 'Contrato Matriz', person_id: 'CNPJ_33000167000101', code: 'ASS-001', type: '1', service_used: '4'});

  it('aceita o mínimo e mapeia o corpo', () => {
    const parsed = nfcomSubscriberSchema.safeParse(ok());
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      const body = toNfcomSubscriber(parsed.data);
      expect(body).toMatchObject({person_id: 'CNPJ_33000167000101', code: 'ASS-001', contract_number: null, primary_uf: null});
    }
  });

  it('recusa código fora do padrão, terminal não numérico e fim de contrato antes do início', () => {
    const paths = nfcomSubscriberSchema.safeParse({
      ...ok(), code: 'ASS 001', primary_terminal: '11a', contract_start: '2026-05-10', contract_end: '2026-05-09',
    }).error?.issues.map((i) => i.path.join('.')) ?? [];
    expect(paths).toEqual(expect.arrayContaining(['code', 'primary_terminal', 'contract_end']));
  });

  it('o nome é normalizado e validado como texto fiscal', () => {
    const parsed = nfcomSubscriberSchema.safeParse({...ok(), name: '  Contrato   Matriz '});
    expect(parsed.success && parsed.data.name).toBe('Contrato Matriz');
    expect(nfcomSubscriberSchema.safeParse({...ok(), name: 'a<b'}).success).toBe(false);
  });
});

describe('nfcomPlanSchema', () => {
  const ok = () => ({
    ...emptyNfcomPlan(), name: 'Internet 100', code: 'PLANO100', description: 'Internet 100 Mbps', class_code: '0100301',
    cfop: '5307', icms_cst: '00', icms_rate: '18',
  });

  it('aceita um plano completo e mapeia os tributos', () => {
    const parsed = nfcomPlanSchema.safeParse({...ok(), unit_value: '99.9', pis_cst: '01', pis_rate: '0.65'});
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(toNfcomPlan(parsed.data)).toMatchObject({
        unit_value: '99.9', cfop: '5307', icms: {cst: '00', rate: '18'}, pis: {cst: '01', rate: '0.65'}, cofins: null,
      });
    }
  });

  it('ICMS 00 pede alíquota e CFOP; sem CST não leva CFOP', () => {
    expect(nfcomPlanSchema.safeParse({...ok(), icms_rate: ''}).error?.issues.map((i) => i.path.join('.'))).toContain('icms_rate');
    expect(nfcomPlanSchema.safeParse({...ok(), cfop: ''}).error?.issues.map((i) => i.path.join('.'))).toContain('cfop');
    expect(nfcomPlanSchema.safeParse({...ok(), icms_cst: 'sem_cst', icms_rate: ''}).error?.issues.map((i) => i.path.join('.'))).toContain('cfop');
    const semCst = nfcomPlanSchema.safeParse({...ok(), icms_cst: 'sem_cst', icms_rate: '', cfop: ''});
    expect(semCst.success).toBe(true);
    if (semCst.success) expect(toNfcomPlan(semCst.data)).toMatchObject({cfop: null, icms: {cst: 'sem_cst', rate: null}});
  });

  it('classificação com 7 dígitos e código de referência restrito', () => {
    const paths = nfcomPlanSchema.safeParse({...ok(), class_code: '123', code: 'plano 1'}).error?.issues.map((i) => i.path.join('.')) ?? [];
    expect(paths).toEqual(expect.arrayContaining(['class_code', 'code']));
  });

  it('PIS informado pede a alíquota', () => {
    expect(nfcomPlanSchema.safeParse({...ok(), pis_cst: '01', pis_rate: ''}).error?.issues.map((i) => i.path.join('.'))).toContain('pis_rate');
  });
});

describe('CFOP da NFCom', () => {
  it('é a lista fechada de 15 códigos do MOC', () => {
    expect(NFCOM_CFOP_OPTIONS).toHaveLength(15);
    expect(NFCOM_CFOP_OPTIONS.every((o) => /^[567]30[1-7]$/.test(o.value))).toBe(true);
  });
});

describe('texto fiscal', () => {
  it('cleanText tira controles e junta espaços', () => {
    expect(cleanText('  a \t b\n\nc\u0000d ')).toBe('a b cd');
    expect(cleanOptional('   ')).toBeUndefined();
  });

  it('FISCAL_TEXT recusa < > euro e espaços nas pontas; REF_CODE recusa acento e espaço', () => {
    expect(FISCAL_TEXT.test('Cobrança de março')).toBe(true);
    for (const bad of ['a<b', 'a>b', 'preço € 1', ' a', 'a ']) expect(FISCAL_TEXT.test(bad)).toBe(false);
    expect(REF_CODE.test('PLANO-100_A/1.2')).toBe(true);
    for (const bad of ['A B', 'AÇÃO', '']) expect(REF_CODE.test(bad)).toBe(false);
  });
});
