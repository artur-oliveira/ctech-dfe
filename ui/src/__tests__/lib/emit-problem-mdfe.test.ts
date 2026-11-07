import {describe, expect, it} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {issuesFor, parseEmitIssues} from '@/lib/api/emit-problem';
import {emitFailure} from '@/lib/billing/notice';
import {mdfeFiscalPrevalidationFixture} from '@/lib/mock/fixtures';

const rule = (id: string, cstat: number) => ({
  code: 'fiscal_prevalidation',
  errors: [{field: id, code: 'fiscal_rule', message: `regra ${id} do MOC (rejeição ${cstat}): descrição`, params: {rule: id, cstat}}],
});

describe('parseEmitIssues: MDF-e', () => {
  it.each([
    ['F14', 638, 'tipo_emitente', /só CT-e/],
    ['F17', 541, 'tipo_emitente', /dentro de um estado/],
    ['F113', 684, 'ciot', /ao menos um CIOT/],
    ['F105', 660, 'aut_xml', /CNPJ/],
    ['F106', 661, 'aut_xml', /CPF/],
    ['F107', 459, 'aut_xml', /duas vezes/],
    ['F21', 703, 'carregamento_posterior', /são o mesmo/],
    ['F22', 704, 'carregamento_posterior', /dentro de um estado/],
    ['F23', 705, 'carregamento_posterior', /rodoviário/],
    ['F24', 707, 'carregamento_posterior', /carga própria \(2\)/],
    ['F27', 706, 'carregamento_posterior', /Incluir NF-e/],
  ])('%s vai para %s com a correção em pt-BR', (id, cstat, target, hint) => {
    const [issue] = parseEmitIssues(rule(id as string, cstat as number), 'mdfe')!;
    expect(issue.target).toBe(target);
    expect(issue.hint).toMatch(hint as RegExp);
    expect(issue.text).toContain('a SEFAZ rejeitaria o manifesto');
  });

  it('a mesma regra F na NF-e continua sendo da retirada', () => {
    const [issue] = parseEmitIssues(rule('F07', 999))!;
    expect(issue.target).toBe('retirada');
  });

  it('invalid_access_key nomeia a chave e o motivo', () => {
    const [issue] = parseEmitIssues({
      code: 'invalid_access_key',
      errors: [{
        field: 'documents[0].access_key', code: 'invalid_access_key',
        message: 'chave de acesso inválida (dígito verificador inválido): 3525', params: {access_key: '3525'},
      }],
    }, 'mdfe')!;
    expect(issue).toMatchObject({target: 'documentos', field: 'documents[0].access_key'});
    expect(issue.text).toBe('Chave de acesso 3525 recusada: dígito verificador inválido.');
  });

  it('422 de validação com dfe_access_key e cpfcnpj vira recusa no campo', () => {
    const issues = parseEmitIssues({
      type: '/problems/validation-error',
      errors: [
        {
          field: 'transported_mdfes[0].access_key', tag: 'dfe_access_key',
          message: 'chave de acesso inválida (modelo 55 não aceito aqui (esperado 58)): 3525',
        },
        {field: 'authorized_xml[1]', tag: 'cpfcnpj', message: 'CPF/CNPJ inválido'},
      ],
    }, 'mdfe')!;
    expect(issues[0].text).toBe('Chave de acesso 3525 recusada: modelo 55 não aceito aqui (esperado 58).');
    expect(issues[1]).toMatchObject({target: 'aut_xml', field: 'authorized_xml[1]'});
    expect(issuesFor(issues, 'aut_xml')).toHaveLength(1);
  });

  it('422 de validação sem essas tags segue com a mensagem da api', () => {
    expect(parseEmitIssues({type: '/problems/validation-error', errors: [{field: 'modal', tag: 'required'}]}, 'mdfe')).toBeNull();
  });

  it('emitFailure com doc mdfe usa as dicas do MDF-e', () => {
    const err = new ApiError(422, mdfeFiscalPrevalidationFixture.detail, mdfeFiscalPrevalidationFixture);
    const failure = emitFailure(err, 'falhou', undefined, 'mdfe');
    expect(failure.issues?.map((i) => i.target)).toEqual(['tipo_emitente', 'aut_xml']);
  });
});
