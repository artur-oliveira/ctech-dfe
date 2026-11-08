import {describe, expect, it} from 'vitest';
import {
  buildReformBody, emptyReformDraft, emptyReformItem, fixedDecimal, reformEventDef, reformEventsFor, reformIssues,
  REFORM_EVENTS, type ReformDraft,
} from '@/lib/data/nfe_reform_events';
import {EVENT_TYPE_LABELS} from '@/lib/data/dfe_event';

const def = (code: string) => reformEventDef(code)!;
const item = (patch: Partial<ReturnType<typeof emptyReformItem>> = {}) => ({...emptyReformItem(1), item_number: '1', ...patch});
const draft = (patch: Partial<ReformDraft> = {}): ReformDraft => ({...emptyReformDraft(), ...patch});

describe('tabela de eventos da reforma', () => {
  it('tem os 12 eventos, cada um com rótulo no histórico', () => {
    expect(REFORM_EVENTS.map((e) => e.code)).toEqual([
      '112110', '112140', '112150', '211110', '211124', '211128', '211130', '211140', '211150', '212110', '212120', '110300',
    ]);
    for (const e of REFORM_EVENTS) expect(EVENT_TYPE_LABELS[e.code]).toBeTruthy();
  });

  it('separa o que a NF-e emitida e a recebida oferecem', () => {
    expect(reformEventsFor(false).map((e) => e.code)).toEqual(['112110', '112140', '112150', '211110', '110300']);
    expect(reformEventsFor(true).map((e) => e.code)).toContain('211150');
    expect(reformEventsFor(true).map((e) => e.code)).not.toContain('112110');
  });
});

describe('pendências por evento', () => {
  it('112110 só aceita o indicador 1', () => {
    expect(reformIssues(def('112110'), draft())).not.toEqual([]);
    expect(reformIssues(def('112110'), draft({indicator: '0'}))).not.toEqual([]);
    expect(reformIssues(def('112110'), draft({indicator: '1'}))).toEqual([]);
    expect(buildReformBody(def('112110'), draft({indicator: '1'}))).toEqual({event_type: '112110', indicator: 1});
  });

  it('aceite de débito e sucessão pedem 0 ou 1', () => {
    for (const c of ['211128', '212110', '212120']) {
      expect(reformIssues(def(c), draft())).not.toEqual([]);
      expect(reformIssues(def(c), draft({indicator: '0'}))).toEqual([]);
      expect(buildReformBody(def(c), draft({indicator: '0'})).indicator).toBe(0);
    }
  });

  it('112150 pede a data e manda delivery_forecast', () => {
    expect(reformIssues(def('112150'), draft())).not.toEqual([]);
    expect(buildReformBody(def('112150'), draft({forecast: '2026-12-01', sequence: '2'})))
      .toEqual({event_type: '112150', sequence_number: 2, delivery_forecast: '2026-12-01'});
  });

  it('eventos de estoque pedem item, IBS, CBS, quantidade e unidade, e repetem o nItem como erro', () => {
    for (const c of ['112140', '211124', '211130', '211140']) {
      expect(reformIssues(def(c), draft({items: [item()]}))).not.toEqual([]);
    }
    const full = item({ibs: '1.5', cbs: '2', quantity: '3', unit: 'UNID'});
    expect(reformIssues(def('112140'), draft({items: [full]}))).toEqual([]);
    expect(reformIssues(def('112140'), draft({items: [full, {...full, id: 2}]}))).toContain('Há item repetido.');
    expect(buildReformBody(def('112140'), draft({items: [full]})).items).toEqual([
      {item_number: 1, ibs: '1.50', cbs: '2.00', quantity: '3', unit: 'UNID'},
    ]);
  });

  it('211150 manda só os créditos', () => {
    const it = item({credit_ibs: '10', credit_cbs: '4.2'});
    expect(reformIssues(def('211150'), draft({items: [it]}))).toEqual([]);
    expect(buildReformBody(def('211150'), draft({items: [it]})).items).toEqual([{item_number: 1, credit_ibs: '10.00', credit_cbs: '4.20'}]);
  });

  it('211110 exige autor, base e código; grupo presumido incompleto é recusado', () => {
    const base = item({base_value: '100', presumed_credit_code: '01'});
    expect(reformIssues(def('211110'), draft({items: [base]}))).toContain('Escolha o autor do evento.');
    const ok = draft({author: '1', items: [{...base, ibs_presumed: {percent: '2.5', value: '2'}}]});
    expect(reformIssues(def('211110'), ok)).toEqual([]);
    expect(buildReformBody(def('211110'), ok)).toEqual({
      event_type: '211110', author_type: '1',
      items: [{item_number: 1, base_value: '100.00', presumed_credit_code: '01', ibs_presumed_credit: {code: '01', percent: '2.50', value: '2.00'}}],
    });
    const half = draft({author: '2', items: [{...base, cbs_presumed: {percent: '2', value: ''}}]});
    expect(reformIssues(def('211110'), half).join(' ')).toMatch(/CBS/);
  });

  it('só o 211110 leva author_type', () => {
    expect(buildReformBody(def('112110'), draft({indicator: '1', author: '1'})).author_type).toBeUndefined();
  });

  it('110300 exige transação, meio, recebedor e PSP com CNPJ válido, e manda a raiz de 8 dígitos do PSP', () => {
    expect(reformIssues(def('110300'), draft())).toHaveLength(4);
    const payment = {transaction_id: 'TX-1', payment_method: '17', receiver: {doc: '11222333000181', name: 'A'}, psp: {doc: '11222333000181', name: 'B'}};
    expect(reformIssues(def('110300'), draft({payment}))).toEqual([]);
    expect(buildReformBody(def('110300'), draft({payment})).payment).toEqual({
      transaction_id: 'TX-1', payment_method: '17', receiver_cnpj: '11222333000181', psp_base_cnpj: '11222333',
    });
  });
});

describe('fixedDecimal', () => {
  it('completa e corta casas sem float', () => {
    expect(fixedDecimal('12.5', 2)).toBe('12.50');
    expect(fixedDecimal('12,345', 2)).toBe('12.34');
    expect(fixedDecimal('7', 4, 0)).toBe('7');
    expect(fixedDecimal('abc', 2)).toBe('');
  });
});
