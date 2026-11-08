import {describe, expect, it} from 'vitest';
import {
  actorIssues, cteDeliveryCancelBody, cteFailureBody, cteReceiptBody, emptyProof, failureIssues, hasPermission,
  interestedActorBody, nfeReceiptBody, proofIssues, receiptCancelBody,
} from '@/lib/utils/delivery-events';

const HASH = 'qUqP5cyxm6YcTAhz05Hph5gvu9M=';
const proof = (patch = {}) => ({
  ...emptyProof(), when: '2026-11-08T10:00', hashAt: '2026-11-08T10:05', hash: HASH, docNumber: '12345', name: 'Maria', ...patch,
});

describe('comprovante de entrega', () => {
  it('NF-e: hash_at é opcional, CT-e exige', () => {
    expect(proofIssues('nfe-receipt', proof({hashAt: ''}))).toEqual([]);
    expect(proofIssues('cte-receipt', proof({hashAt: ''}))).not.toEqual([]);
  });

  it('recusa hash fora de SHA-1 em Base64 e GPS pela metade', () => {
    expect(proofIssues('nfe-receipt', proof({hash: 'curto'}))).not.toEqual([]);
    expect(proofIssues('nfe-receipt', proof({latitude: '-5.1'}))).toContain('Informe latitude e longitude juntas.');
  });

  it('o insucesso do CT-e não pede recebedor', () => {
    expect(proofIssues('cte-failure', proof({docNumber: '', name: ''}))).toEqual([]);
  });

  it('corpo da NF-e (110130)', () => {
    const b = nfeReceiptBody(proof({hashAt: '', latitude: '-5.1', longitude: '-42.8'}));
    expect(b).toMatchObject({doc_number: '12345', name: 'Maria', receipt_hash: HASH, latitude: '-5.100000', longitude: '-42.800000'});
    expect(b.delivered_at).toMatch(/^2026-11-08T10:00:00[+-]\d\d:\d\d$/);
    expect(b).not.toHaveProperty('hash_at');
    expect(b).not.toHaveProperty('sequence_number');
  });

  it('corpo do CT-e (110180) com notas e sequência', () => {
    const b = cteReceiptBody(proof({nfeKeys: ['1'.repeat(44)], sequence: '3'}));
    expect(b).toMatchObject({hash: HASH, doc_number: '12345', name: 'Maria', nfe_keys: ['1'.repeat(44)], sequence_number: 3});
    expect(b).toHaveProperty('occurred_at');
    expect(b).toHaveProperty('hash_at');
  });
});

describe('insucesso de entrega (110190)', () => {
  it('justificativa de 25 a 250 só no motivo 4', () => {
    expect(failureIssues({reason: '', attempt: '', justification: ''})).not.toEqual([]);
    for (const r of ['1', '2', '3']) expect(failureIssues({reason: r, attempt: '', justification: ''})).toEqual([]);
    expect(failureIssues({reason: '4', attempt: '', justification: 'curta'})).not.toEqual([]);
    expect(failureIssues({reason: '4', attempt: '', justification: 'x'.repeat(25)})).toEqual([]);
    expect(failureIssues({reason: '4', attempt: '', justification: 'x'.repeat(251)})).not.toEqual([]);
  });

  it('só manda justification no motivo 4', () => {
    const p = proof();
    expect(cteFailureBody(p, {reason: '2', attempt: '2', justification: 'ignorada'})).toMatchObject({reason_type: 2, attempt: 2});
    expect(cteFailureBody(p, {reason: '2', attempt: '', justification: 'ignorada'})).not.toHaveProperty('justification');
    expect(cteFailureBody(p, {reason: '4', attempt: '', justification: ' ' + 'x'.repeat(25) + ' '}))
      .toMatchObject({reason_type: 4, justification: 'x'.repeat(25)});
  });
});

describe('ator interessado (110150)', () => {
  it('escolhe cnpj ou cpf pelo tamanho do documento', () => {
    expect(interestedActorBody('11222333000181', true, '')).toEqual({cnpj: '11222333000181', allow_subcontracts: true});
    expect(interestedActorBody('52998224725', false, '2')).toEqual({cpf: '52998224725', allow_subcontracts: false, sequence_number: 2});
  });

  it('aceita CNPJ alfanumérico e recusa dígito inválido', () => {
    expect(actorIssues('12.ABC.345/01DE-35')).toEqual([]);
    expect(interestedActorBody('12.ABC.345/01DE-35', false, '')).toMatchObject({cnpj: '12ABC34501DE35'});
    expect(actorIssues('11222333000182')).not.toEqual([]);
    expect(actorIssues('')).not.toEqual([]);
  });

  it('manda author_type quando a NF-e é recebida', () => {
    expect(interestedActorBody('52998224725', false, '', '2').author_type).toBe('2');
  });
});

describe('cancelamentos e permissão', () => {
  it('corpo dos cancelamentos', () => {
    expect(receiptCancelBody('135260000000001', '')).toEqual({delivery_protocol: '135260000000001'});
    expect(cteDeliveryCancelBody('failure', 'sk1', '4')).toEqual({kind: 'failure', event_sk: 'sk1', sequence_number: 4});
  });

  it('hasPermission aceita curinga', () => {
    expect(hasPermission(['*'], 'create.nfe_events')).toBe(true);
    expect(hasPermission(['create.*'], 'create.cte_events')).toBe(true);
    expect(hasPermission(['create.cte_events'], 'create.nfe_events')).toBe(false);
    expect(hasPermission(undefined, 'create.nfe_events')).toBe(false);
  });
});
