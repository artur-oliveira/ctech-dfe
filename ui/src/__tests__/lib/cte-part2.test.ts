import {describe, expect, it} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {parseEmitIssues} from '@/lib/api/emit-problem';
import {cteFiscalPrevalidationFixture} from '@/lib/mock/fixtures';
import {
  buildCteEmitBody, computeCteIbsCbs, cteEmitFailure, cteEventFailureMessage, cteMissing, emptyCteForm, emptyIbsCbs, sameDoc,
} from '@/lib/utils/cte';
import {CTE_EVENT_ERRORS} from '@/lib/constants/cte';
import {correctionGroup, CTE_CORRECTION_GROUPS} from '@/lib/data/cte_correction';
import {cteEmitBodySchema} from '@/lib/schemas/cte';

const KEY = '35250512345678000195570010000000011000000015';

describe('IBS/CBS: prévia dos valores do servidor', () => {
  const s = {...emptyIbsCbs(), base_value: '1850.00', ibs_uf_rate: '0.1000', ibs_mun_rate: '0.0500', cbs_rate: '0.9000'};

  it('vBC x alíquota, 2 casas, IBS = UF + município', () => {
    expect(computeCteIbsCbs(s, '1850.00', 2026)).toEqual({
      ibsUf: '1.85', ibsMun: '0.92', ibs: '2.77', cbs: '16.65', total: '1850.00',
    });
  });

  it('arredonda para o par (meio centavo): 0.50 e 1.50 centavos', () => {
    // 1850,00 x 0,05% = 0,925 -> 0,92 (par) tambem aparece acima. 10,50 x 0,1% = 0,0105 -> 0,01 ; 10,50 x 0,05% = 0,00525 -> 0,01
    const r = computeCteIbsCbs({...s, base_value: '10.50', ibs_uf_rate: '0.1000', ibs_mun_rate: '0.0500', cbs_rate: '0'}, '10.50', 2026);
    expect(r?.ibsUf).toBe('0.01');
    // 5,00 x 0,1% = 0,005 -> 0,00 (par) e 15,00 x 0,1% = 0,015 -> 0,02 (par)
    expect(computeCteIbsCbs({...s, base_value: '5.00', cbs_rate: '0.1000'}, '5.00', 2026)?.cbs).toBe('0.00');
    expect(computeCteIbsCbs({...s, base_value: '15.00', cbs_rate: '0.1000'}, '15.00', 2026)?.cbs).toBe('0.02');
  });

  it('vTotDFe soma IBS e CBS só a partir de 2027', () => {
    expect(computeCteIbsCbs(s, '1850.00', 2027)?.total).toBe('1869.42');
  });

  it('sem base não há prévia', () => {
    expect(computeCteIbsCbs(emptyIbsCbs(), '100.00', 2027)).toBeNull();
  });
});

describe('corpo da emissão com IBS/CBS e autXML', () => {
  const base = {
    ...emptyCteForm('00'), nfeKeys: [KEY], takerRole: 'remetente' as const, freightTotal: '100.00', cfop: '5353',
    icms: {cst: '00' as const, p_icms: '12', p_red_bc: ''},
  };

  it('envia ibs_cbs e authorized_xml quando preenchidos', () => {
    const body = buildCteEmitBody({
      ...base,
      ibsCbs: {...emptyIbsCbs(), cst: '000', class_trib: '000001', donation: true, base_value: '100', ibs_uf_rate: '0.1', ibs_mun_rate: '0', cbs_rate: '0.9'},
      authorizedXml: [{doc: '12345678000195', name: 'Contador'}],
    }, undefined);
    expect(body.ibs_cbs).toEqual({
      cst: '000', class_trib: '000001', donation: true, base_value: '100.00', ibs_uf_rate: '0.1', ibs_mun_rate: '0', cbs_rate: '0.9',
    });
    expect(body.authorized_xml).toEqual([{doc: '12345678000195'}]);
    expect(cteEmitBodySchema.safeParse(body).success).toBe(true);
  });

  it('omite os dois grupos quando vazios', () => {
    const body = buildCteEmitBody({...base, ibsCbs: null, authorizedXml: []}, undefined);
    expect(body).not.toHaveProperty('ibs_cbs');
    expect(body).not.toHaveProperty('authorized_xml');
  });

  it('IBS/CBS incompleto bloqueia a emissão; autXML repetido ou acima de 10 não passa no schema', () => {
    const missing = cteMissing({...base, ibsCbs: emptyIbsCbs()}, {
      notes: [], sender: {doc: '1', name: 'A', uf: 'SP', city_ibge: '1', city: 'x', person_id: null},
      recipient: {doc: '2', name: 'B', uf: 'SP', city_ibge: '1', city: 'x', person_id: null}, taker_suggestion: null,
      cargo: {total_value: '1', total_weight_kg: '1', volumes: 1, predominant_product: 'p'}, origin: null, destination: null, warnings: [],
    }, false);
    expect(missing).toContain('Complete o IBS/CBS ou desligue o bloco');
    const body = buildCteEmitBody(base, undefined);
    const dup = {doc: '12345678000195'};
    expect(cteEmitBodySchema.safeParse({...body, authorized_xml: [dup, dup]}).success).toBe(false);
    const many = Array.from({length: 11}, (_, i) => ({doc: `1234567800${String(1000 + i)}`}));
    expect(cteEmitBodySchema.safeParse({...body, authorized_xml: many}).success).toBe(false);
  });
});

describe('sameDoc', () => {
  it('ignora máscara e caixa', () => {
    expect(sameDoc('12.345.678/0001-95', '12345678000195')).toBe(true);
    expect(sameDoc('12abc34501de35', '12ABC34501DE35')).toBe(true);
    expect(sameDoc('', '12345678000195')).toBe(false);
  });
});

describe('erros 422 do CT-e', () => {
  const apiError = (raw: object) => new ApiError(422, 'detalhe da api', raw);

  it.each(Object.keys(CTE_EVENT_ERRORS))('%s vira frase em pt-BR', (code) => {
    const msg = cteEventFailureMessage(apiError({code}), 'x');
    expect(msg).toBe(CTE_EVENT_ERRORS[code]);
    expect(msg).not.toMatch(/—/);
  });

  it('código desconhecido segue a mensagem da api', () => {
    expect(cteEventFailureMessage(apiError({code: 'outro'}), 'x')).toBe('detalhe da api');
  });

  it('fiscal_prevalidation do CT-e usa a tabela G, não a da NF-e', () => {
    const issues = parseEmitIssues(cteFiscalPrevalidationFixture, 'cte')!;
    expect(issues.map((i) => i.target)).toEqual(['aut_xml', 'partes']);
    expect(issues[0].text).toContain('a SEFAZ rejeitaria o CT-e');
    expect(issues[0].hint).toMatch(/autorizado a baixar o XML/);
  });

  it('cteEmitFailure lista as regras do CT-e', () => {
    const f = cteEmitFailure(apiError(cteFiscalPrevalidationFixture));
    expect(f.issues).toHaveLength(2);
  });
});

describe('carta de correção: grupos fechados', () => {
  it('grupos que se repetem pedem o número do item', () => {
    expect(correctionGroup('ObsCont')?.repeats).toBe(true);
    expect(correctionGroup('rem')?.repeats).toBeFalsy();
  });
  it('todo grupo tem campos e nomes únicos', () => {
    expect(new Set(CTE_CORRECTION_GROUPS.map((g) => g.value)).size).toBe(CTE_CORRECTION_GROUPS.length);
    expect(CTE_CORRECTION_GROUPS.every((g) => g.fields.length > 0)).toBe(true);
  });
});
