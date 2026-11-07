import {describe, expect, it, vi} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {localFieldErrors, parseEmitIssues} from '@/lib/api/emit-problem';
import {emitFailure} from '@/lib/billing/notice';
import {nfeFiscalPrevalidationFixture} from '@/lib/mock/fixtures';

// Corpos no formato de api/internal/services/nfes/prevalidate.go e v.go.
const ruleProblem = {
  code: 'fiscal_prevalidation',
  detail: 'a nota não passa nas validações do leiaute e do MOC',
  errors: [{
    field: 'I08-30', code: 'fiscal_rule',
    message: 'regra I08-30 do MOC (rejeição 731): CFOP de operação com exterior e idDest diferente de 3',
    params: {rule: 'I08-30', cstat: 731},
  }],
};

describe('parseEmitIssues', () => {
  it('too_long diz entidade, item, campo, limite e quantos caracteres cortar', () => {
    const [issue] = parseEmitIssues({
      code: 'fiscal_prevalidation',
      errors: [{field: 'description', code: 'too_long', params: {entity: 'produto', item: 2, limit: 120, got: 134}}],
    })!;
    expect(issue).toMatchObject({target: 'produto', item: 2, field: 'description'});
    expect(issue.text).toBe('Produto do item 2: Descrição tem 134 caracteres e o leiaute aceita até 120.');
    expect(issue.hint).toBe('Corte 14 caracteres.');
  });

  it('too_short pede o que falta', () => {
    const [issue] = parseEmitIssues({
      code: 'fiscal_prevalidation',
      errors: [{field: 'entrega.x_lgr', code: 'too_short', params: {entity: 'entrega', limit: 2, got: 1}}],
    })!;
    expect(issue.text).toBe('Local de entrega: Logradouro tem 1 caractere e o leiaute pede ao menos 2.');
    expect(issue.hint).toBe('Complete com mais 1 caractere.');
  });

  it('fiscal_rule mostra a regra, o cStat, a descrição e a correção da família', () => {
    const [issue] = parseEmitIssues(ruleProblem)!;
    expect(issue.text).toBe(
      'Regra I08-30 do MOC; a SEFAZ rejeitaria a nota com o código 731: CFOP de operação com exterior e idDest diferente de 3.',
    );
    expect(issue.target).toBe('itens');
    expect(issue.hint).toMatch(/7xxx para o exterior/);
  });

  it('a regra do isento vence a família do exterior', () => {
    const [issue] = parseEmitIssues({
      code: 'fiscal_prevalidation',
      errors: [{field: 'E16a-30', code: 'fiscal_rule', message: 'x', params: {rule: 'E16a-30', cstat: 805}}],
    })!;
    expect(issue.target).toBe('destinatario');
    expect(issue.hint).toMatch(/não aceita contribuinte isento/);
  });

  it('registration_incomplete nomeia o cadastro e o campo', () => {
    const [issue] = parseEmitIssues({
      code: 'registration_incomplete',
      errors: [{field: 'person.addresses[0].country_code', code: 'required', params: {entity: 'destinatario'}}],
    })!;
    expect(issue).toMatchObject({target: 'destinatario', text: 'Destinatário: falta País.'});
  });

  it('invalid_decimal nomeia a chave lida', () => {
    const [issue] = parseEmitIssues({code: 'invalid_decimal', errors: [{field: 'p_icms', code: 'invalid_decimal'}]})!;
    expect(issue.text).toBe('Número malformado em p_icms.');
  });

  it('ignora o que não é recusa local da emissão', () => {
    expect(parseEmitIssues({code: 'validation_failed', errors: [{field: 'x'}]})).toBeNull();
    expect(parseEmitIssues(undefined)).toBeNull();
  });

  it('localFieldErrors devolve o erro pelo campo do local', () => {
    const issues = parseEmitIssues({
      code: 'registration_incomplete',
      errors: [{field: 'entrega.c_pais', code: 'required', params: {entity: 'entrega'}}],
    })!;
    expect(localFieldErrors(issues, 'entrega')).toEqual({c_pais: 'Local de entrega: falta País.'});
    expect(localFieldErrors(issues, 'retirada')).toEqual({});
  });
});

describe('emitFailure com recusa local', () => {
  it('liga cada recusa à correção que o formulário oferece', () => {
    const err = new ApiError(422, nfeFiscalPrevalidationFixture.detail, nfeFiscalPrevalidationFixture);
    const actionFor = vi.fn((target: string) =>
      target === 'emitente' ? {label: 'Abrir cadastro', href: '/organizations/edit?pk=o'} : undefined);
    const failure = emitFailure(err, 'falhou', actionFor);
    expect(failure.message).toBe(nfeFiscalPrevalidationFixture.detail);
    expect(failure.issues?.map((i) => i.target)).toEqual(['destinatario', 'emitente']);
    expect(failure.issues?.[1].action).toEqual({label: 'Abrir cadastro', href: '/organizations/edit?pk=o'});
    expect(failure.issues?.[0].hint).toBe('Corte 8 caracteres.');
  });

  it('erro comum continua só com o detail', () => {
    expect(emitFailure(new ApiError(400, 'ruim', {code: 'bad_request'}), 'falhou')).toEqual({message: 'ruim'});
  });
});
