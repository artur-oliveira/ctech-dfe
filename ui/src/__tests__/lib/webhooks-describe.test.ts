import {describe, expect, it} from 'vitest';
import {describeChallengeError, filterSummary, validateWebhookUrl} from '@/lib/webhooks/describe';

describe('validateWebhookUrl', () => {
  it('aceita https com host público', () => {
    expect(validateWebhookUrl('https://hooks.example.com/dfe')).toBeNull();
    expect(validateWebhookUrl('https://203.0.113.10:8443/x?y=1')).toBeNull();
  });

  it('recusa o que o servidor recusaria, sem esperar a ida e volta', () => {
    for (const url of ['', 'hooks.example.com', 'http://hooks.example.com', 'https://user:pw@hooks.example.com', 'https://', 'not a url']) {
      expect(validateWebhookUrl(url), url).not.toBeNull();
    }
    for (const url of ['https://localhost/x', 'https://127.0.0.1/x', 'https://10.0.0.5/x', 'https://192.168.1.2/x', 'https://169.254.169.254/x', 'https://[::1]/x', 'https://app.local/x', 'https://svc.internal/x', 'https://intranet/x']) {
      expect(validateWebhookUrl(url), url).toMatch(/site público na internet/);
    }
  });

  it('exige https com mensagem própria para http', () => {
    expect(validateWebhookUrl('http://hooks.example.com')).toMatch(/https/);
  });
});

describe('describeChallengeError', () => {
  it('traduz o motivo que o servidor devolve', () => {
    expect(describeChallengeError('the endpoint did not answer')).toBe('O endereço não respondeu.');
    expect(describeChallengeError('the endpoint answered HTTP 404, expected 200 with the challenge'))
      .toBe('O endereço respondeu HTTP 404; esperávamos HTTP 200 devolvendo o código de verificação.');
    expect(describeChallengeError('the endpoint did not return the hub.challenge value'))
      .toMatch(/não devolveu o código de verificação/);
  });

  it('mostra o texto original quando não reconhece o motivo', () => {
    expect(describeChallengeError('algo novo')).toBe('algo novo');
  });
});

describe('filterSummary', () => {
  const names = new Map([['c1', 'Alfa Ltda'], ['c2', 'Beta Ltda'], ['c3', 'Gama Ltda']]);

  it('vazio é tudo', () => {
    expect(filterSummary([], [], names)).toEqual({events: 'Todos os eventos', companies: 'Todas as empresas'});
    expect(filterSummary(null, null, names)).toEqual({events: 'Todos os eventos', companies: 'Todas as empresas'});
  });

  it('nomeia o que foi escolhido', () => {
    expect(filterSummary(['document.rejected', 'document.failed'], ['c2'], names)).toEqual({
      events: 'Rejeitada, Falha na emissão',
      companies: 'Beta Ltda',
    });
  });

  it('resume muitas empresas e não quebra com uma que o usuário não enxerga', () => {
    expect(filterSummary([], ['c1', 'c2', 'c3'], names).companies).toBe('3 empresas');
    expect(filterSummary([], ['zz'], names).companies).toBe('1 empresa');
  });
});
