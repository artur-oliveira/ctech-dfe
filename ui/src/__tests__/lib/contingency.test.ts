import {describe, expect, it} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {emitFailure} from '@/lib/billing/notice';
import {
  contingencyFailure, contingencyLabel, contingencyNotice, currentEmission, emissionLabel, inContingency,
} from '@/lib/contingency';
import {nfceConfigSchema} from '@/lib/schemas/fiscal-configs';

describe('forma de emissão', () => {
  it('ausente é normal', () => {
    expect(currentEmission({})).toBe('normal');
    expect(inContingency({})).toBe(false);
    expect(inContingency({emission_type: 'offline'})).toBe(true);
  });

  it('o SVC se explica por documento', () => {
    expect(emissionLabel('svc', 'nfe')).toBe('Contingência SVC (SVC-AN ou SVC-RS conforme a UF)');
    expect(emissionLabel('svc', 'cte')).toBe('Contingência SVC (SVC-RS ou SVC-SP)');
    expect(emissionLabel('offline', 'mdfe')).toBe('Contingência offline');
    expect(emissionLabel('epec', 'nfe')).toBe('EPEC');
  });
});

describe('documento em contingência', () => {
  it('pendente mostra o prazo e concorda o gênero', () => {
    expect(contingencyNotice('pending', false, '10/07/2026 12:00'))
      .toBe('Emitida em contingência, pendente de autorização; prazo até 10/07/2026 12:00.');
    expect(contingencyNotice('authorized_late', true)).toMatch(/^Emitido em contingência e autorizado/);
    expect(contingencyLabel('rejected_late')).toBe('Rejeitada após contingência');
  });
});

describe('recusas da contingência', () => {
  const apiError = (status: number, body: Record<string, unknown>) => new ApiError(status, String(body.detail), body);

  it('QR Code v2 com CPF aponta a configuração da NFC-e (rejeição 444)', () => {
    const f = emitFailure(apiError(400, {detail: 'emitente pessoa física (CPF) exige o QR Code versão 3; configure qr_code_version = 3 (rejeição 444)'}), 'x');
    expect(f.message).toMatch(/QR Code versão 3/);
    expect(f.action).toEqual({label: 'Abrir configuração da NFC-e', href: '/fiscal-config?tab=nfce'});
  });

  it('SVC sem UF designada leva à aba do documento', () => {
    const f = emitFailure(apiError(400, {detail: 'UF do emitente sem SVC designada: AC'}), 'x', undefined, 'cte');
    expect(f.message).toMatch(/não tem SVC designada/);
    expect(f.action?.href).toBe('/fiscal-config?tab=cte');
  });

  it('os dois 422 viram frase, por code', () => {
    expect(contingencyFailure({code: 'emission_type_invalid'}, '', 'mdfe')?.message).toMatch(/não existe para este documento/);
    expect(contingencyFailure({code: 'emission_reason_required'}, '')?.message).toMatch(/15 a 256/);
    expect(contingencyFailure({code: 'outro'}, 'qualquer coisa')).toBeNull();
  });
});

describe('nfceConfigSchema: CSC e versão do QR Code', () => {
  const base = {
    timezone: 'America/Sao_Paulo', environment: '2',
    prod_current_serie: '1', prod_current_number: '1', hom_current_serie: '1', hom_current_number: '1',
    prod_csc: '', prod_csc_id: '', hom_csc: '', hom_csc_id: '',
  };

  it('versão 2 exige os quatro campos de CSC', () => {
    const r = nfceConfigSchema.safeParse({...base, qr_code_version: '2'});
    expect(r.success).toBe(false);
    expect(r.error?.issues.map((i) => i.path[0]).sort()).toEqual(['hom_csc', 'hom_csc_id', 'prod_csc', 'prod_csc_id']);
  });

  it('versão 3 dispensa o CSC', () => {
    expect(nfceConfigSchema.safeParse({...base, qr_code_version: '3'}).success).toBe(true);
  });

  it('versão 2 com CSC completo passa', () => {
    expect(nfceConfigSchema.safeParse({
      ...base, qr_code_version: '2', prod_csc: 'A', prod_csc_id: '1', hom_csc: 'B', hom_csc_id: '1',
    }).success).toBe(true);
  });
});
