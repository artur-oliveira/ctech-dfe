import {describe, it, expect} from 'vitest';
import {resolveDfeResultToast} from '@/lib/utils/dfe-result-toast';

describe('resolveDfeResultToast · document results', () => {
  it('maps authorized to a success toast', () => {
    expect(resolveDfeResultToast({table_name: 'nfes', status: 'authorized'})).toEqual({
      variant: 'success',
      message: 'NF-e autorizada',
      action: undefined,
    });
  });

  it('maps rejected to an error toast with the motive', () => {
    expect(resolveDfeResultToast({table_name: 'nfes', status: 'rejected', sefaz_motive: 'Duplicidade'})).toEqual({
      variant: 'error',
      message: 'NF-e rejeitada pela SEFAZ; Duplicidade',
    });
  });

  it('concorda o gênero com o documento', () => {
    expect(resolveDfeResultToast({table_name: 'mdfes', status: 'authorized'}).message).toBe('MDF-e autorizado');
  });

  // retryable_failed não é terminal: o worker reprocessa sozinho, então o toast
  // informa sem mandar o usuário agir.
  it('trata retryable_failed como aviso, não como erro', () => {
    const r = resolveDfeResultToast({table_name: 'nfes', status: 'retryable_failed', sefaz_motive: 'timeout SEFAZ'});
    expect(r.variant).toBe('info');
    expect(r.message).toBe('NF-e não pôde ser enviada agora; tentando novamente; timeout SEFAZ');
  });

  it('rotula o status no fallback em vez de vazar o valor cru', () => {
    expect(resolveDfeResultToast({table_name: 'mdfes', status: 'processing'})).toEqual({
      variant: 'info',
      message: 'MDF-e atualizado; status: Processando',
    });
  });
});

describe('resolveDfeResultToast · event results', () => {
  // Regression: a failed MDF-e cancellation used to surface the reverted
  // document status ("authorized" → "Documento autorizado"), masking the error.
  it('reports the event error, NOT the reverted document status', () => {
    const result = resolveDfeResultToast({
      result_kind: 'event',
      table_name: 'mdfes',
      event_type: '110111',
      status: 'error',
      sefaz_motive: 'Failed to sign XML: Unable to resolve reference URI: #',
    });
    expect(result.variant).toBe('error');
    expect(result.message).toBe('Falha ao cancelar MDF-e; Failed to sign XML: Unable to resolve reference URI: #');
  });

  it('does not show "autorizada" even when an authorized status leaks into an event', () => {
    const result = resolveDfeResultToast({
      result_kind: 'event',
      table_name: 'mdfes',
      event_type: '110111',
      status: 'authorized',
    });
    // authorized is not a valid event status → treated as a failure, never a success.
    expect(result.variant).toBe('error');
    expect(result.message).not.toContain('autorizada');
  });

  it('reports a successful cancellation', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'mdfes', event_type: '110111', status: 'success'}),
    ).toMatchObject({variant: 'success', message: 'MDF-e cancelado'});
  });

  it('avisa retentativa de evento sem chamar de falha', () => {
    const r = resolveDfeResultToast({
      result_kind: 'event', table_name: 'mdfes', event_type: '110112', status: 'retryable_failed',
    });
    expect(r.variant).toBe('info');
    expect(r.message).toBe('Encerramento de MDF-e não concluído; tentando novamente');
  });

  it('reports a rejected cancellation with the motive', () => {
    expect(
      resolveDfeResultToast({
        result_kind: 'event',
        table_name: 'nfes',
        event_type: '110111',
        status: 'rejected',
        sefaz_motive: 'Prazo excedido',
      }),
    ).toEqual({variant: 'error', message: 'Cancelamento de NF-e rejeitado pela SEFAZ; Prazo excedido'});
  });

  it('uses encerramento wording for MDF-e event 110112', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'mdfes', event_type: '110112', status: 'success'}),
    ).toMatchObject({variant: 'success', message: 'MDF-e encerrado'});
  });

  it('treats 110112 as cancellation (substituição) for NF-e', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'nfes', event_type: '110112', status: 'success'}),
    ).toMatchObject({variant: 'success', message: 'NF-e cancelada'});
  });

  // Regressão: INUT caía no ramo de cancelamento e a inutilização de uma faixa
  // de numeração era anunciada como "NFC-e cancelada com sucesso".
  it('uses inutilização wording for an INUT event', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'nfces', event_type: 'INUT', status: 'success'}),
    ).toEqual({variant: 'success', message: 'Faixa de numeração de NFC-e inutilizada com sucesso'});
  });

  it('reports a rejected INUT event with the SEFAZ motive', () => {
    expect(
      resolveDfeResultToast({
        result_kind: 'event',
        table_name: 'nfes',
        event_type: 'INUT',
        status: 'rejected',
        sefaz_motive: 'Ja existe NF-e autorizada para a faixa informada',
      }),
    ).toEqual({
      variant: 'error',
      message: 'Inutilização de numeração de NF-e rejeitada pela SEFAZ; Ja existe NF-e autorizada para a faixa informada',
    });
  });

  it('reports a failed INUT event as an inutilização failure', () => {
    expect(
      resolveDfeResultToast({result_kind: 'event', table_name: 'nfces', event_type: 'INUT', status: 'error'}),
    ).toEqual({variant: 'error', message: 'Falha ao inutilizar a numeração de NFC-e'});
  });

  it('keeps an INUT retryable failure non-alarming', () => {
    expect(
      resolveDfeResultToast({
        result_kind: 'event',
        table_name: 'nfces',
        event_type: 'INUT',
        status: 'retryable_failed',
      }),
    ).toEqual({variant: 'info', message: 'Inutilização de numeração não concluída; tentando novamente'});
  });
});

describe('resolveDfeResultToast · número e ação do botão', () => {
  const KEY_NFE = '35261162787449000107550010000001231000000010';

  it('autorizada: cita o número e oferece o PDF', () => {
    expect(resolveDfeResultToast({table_name: 'nfes', status: 'authorized', access_key: KEY_NFE}, 123)).toEqual({
      variant: 'success',
      message: 'NF-e 123 autorizada',
      action: {kind: 'pdf', label: 'Ver PDF', tableName: 'nfes', id: KEY_NFE},
    });
  });

  it('NFS-e leva o rótulo próprio, não o genérico "Documento"', () => {
    const r = resolveDfeResultToast({table_name: 'nfses', status: 'authorized', access_key: 'DPS1'}, 1);
    expect(r.message).toBe('NFS-e 1 autorizada');
  });

  it('CT-e e MDF-e concordam no masculino', () => {
    expect(resolveDfeResultToast({table_name: 'ctes', status: 'authorized'}, 7).message).toBe('CT-e 7 autorizado');
    expect(resolveDfeResultToast({table_name: 'mdfes', status: 'authorized'}, 8).message).toBe('MDF-e 8 autorizado');
  });

  it('cancelamento: ver detalhes com o evento destacado por query param', () => {
    const r = resolveDfeResultToast({
      result_kind: 'event', table_name: 'nfes', event_type: '110111', status: 'success',
      access_key: KEY_NFE, event_sk: 'EVT#1',
    }, 123);
    expect(r.message).toBe('NF-e 123 cancelada');
    expect(r.action).toEqual({
      kind: 'link', label: 'Ver detalhes', href: `/nfe/detail?key=${KEY_NFE}&event=EVT%231`,
    });
  });

  it('cancelamento de NFS-e usa o evento 101101 e o id da DPS', () => {
    const r = resolveDfeResultToast({
      result_kind: 'event', table_name: 'nfses', event_type: '101101', status: 'success',
      access_key: 'DPS123', event_sk: 'E1',
    }, 1);
    expect(r.message).toBe('NFS-e 1 cancelada');
    expect(r.action).toEqual({kind: 'link', label: 'Ver detalhes', href: '/nfse/detail?id=DPS123&event=E1'});
  });

  it('carta de correção', () => {
    const r = resolveDfeResultToast({
      result_kind: 'event', table_name: 'nfes', event_type: '110110', status: 'success', access_key: KEY_NFE,
    }, 123);
    expect(r.message).toBe('Correção da NF-e 123 enviada');
  });

  it('sem número conhecido a mensagem sai sem número', () => {
    expect(resolveDfeResultToast({table_name: 'nfces', status: 'authorized'}).message).toBe('NFC-e autorizada');
  });
});
