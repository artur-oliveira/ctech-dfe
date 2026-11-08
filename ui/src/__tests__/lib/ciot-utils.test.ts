import {describe, expect, it} from 'vitest';
import {ApiError} from '@/lib/api/client';
import {
  canCancelCiot,
  canCloseCiot,
  canRectifyCiot,
  cancelWindow,
  ciotIdFromSk,
  floorUnavailable,
  problemLines,
  resolveCiotToast,
  summarizeAntt,
} from '@/lib/utils/ciot';

describe('ciotIdFromSk', () => {
  it('tira o prefixo da chave da linha', () => {
    expect(ciotIdFromSk('CIOT_a1b2c3d4e5f6')).toBe('a1b2c3d4e5f6');
    expect(ciotIdFromSk('a1b2')).toBe('a1b2');
  });
});

describe('ações permitidas', () => {
  it('só CIOT em vigor e sem operação em andamento recebe cancelar e encerrar', () => {
    expect(canCancelCiot({status: 'declared'})).toBe(true);
    expect(canCloseCiot({status: 'rectified'})).toBe(true);
    expect(canCancelCiot({status: 'declared', pending_operation: 'close'})).toBe(false);
    for (const status of ['pending', 'cancelled', 'closed', 'rejected', 'failed'] as const) {
      expect(canCancelCiot({status})).toBe(false);
    }
  });

  it('carga lotação não retifica (B121)', () => {
    expect(canRectifyCiot({status: 'declared', operation_type: 1})).toBe(false);
    expect(canRectifyCiot({status: 'declared', operation_type: 2})).toBe(true);
    expect(canRectifyCiot({status: 'declared', operation_type: 3})).toBe(true);
  });
});

describe('cancelWindow', () => {
  it('conta 24 h a partir do início da viagem, em horário de Brasília', () => {
    const w = cancelWindow('2026-07-12', new Date('2026-07-12T20:00:00Z'))!;
    expect(w.deadline.toISOString()).toBe('2026-07-13T03:00:00.000Z');
    expect(w.expired).toBe(false);
  });

  it('avisa quando o prazo já passou', () => {
    expect(cancelWindow('2026-07-12', new Date('2026-07-13T03:00:01Z'))!.expired).toBe(true);
  });

  it('sem data de início não há prazo', () => {
    expect(cancelWindow(undefined)).toBeNull();
    expect(cancelWindow('lixo')).toBeNull();
  });
});

describe('summarizeAntt', () => {
  it('lê RNTRC e campos escalares sem presumir o formato', () => {
    const s = summarizeAntt({codigo: 111, mensagem: 'ok', body: {RNTRC: '012345678', Situacao: 'Ativo', EquiparadoTAC: false, Lista: [1]}});
    expect(s.ok).toBe(true);
    expect(s.rntrc).toBe('012345678');
    expect(s.fields).toEqual([
      {label: 'RNTRC', value: '012345678'}, {label: 'Situacao', value: 'Ativo'}, {label: 'EquiparadoTAC', value: 'Não'},
    ]);
  });

  it('código diferente de 110 e 111 não é sucesso e não inventa RNTRC', () => {
    const s = summarizeAntt({codigo: 205, mensagem: 'Transportador não encontrado'});
    expect(s.ok).toBe(false);
    expect(s.rntrc).toBeNull();
    expect(summarizeAntt(undefined).ok).toBe(false);
  });
});

describe('piso mínimo indisponível', () => {
  it('503 e ciot_floor_unverified são "indisponível", não pane', () => {
    expect(floorUnavailable(new ApiError(503, 'x'))).toBe(true);
    expect(floorUnavailable(new ApiError(400, 'x', {code: 'ciot_floor_unverified'}))).toBe(true);
    expect(floorUnavailable(new ApiError(422, 'x'))).toBe(false);
    expect(floorUnavailable(new Error('x'))).toBe(false);
  });
});

describe('problemLines', () => {
  it('uma linha por regra recusada, com a regra e o código da ANTT', () => {
    const err = new ApiError(422, 'x', {errors: [
      {message: 'Frete inválido', params: {rule: 'B120', antt_code: 319}},
      {message: 'Sem referência'},
      {code: 'sem mensagem'},
    ]});
    expect(problemLines(err)).toEqual(['Frete inválido (B120, ANTT 319)', 'Sem referência']);
    expect(problemLines(new Error('x'))).toEqual([]);
  });
});

describe('resolveCiotToast', () => {
  it('sucesso por operação, com o CIOT masculino', () => {
    expect(resolveCiotToast({status: 'declared', ciot: 'A1'})).toEqual({variant: 'success', message: 'CIOT A1 declarado na ANTT'});
    expect(resolveCiotToast({status: 'cancelled'}).message).toBe('CIOT cancelado na ANTT');
    expect(resolveCiotToast({status: 'closed'}).message).toBe('CIOT encerrado na ANTT');
    expect(resolveCiotToast({status: 'rectified'}).message).toBe('CIOT retificado na ANTT');
  });

  it('recusa da declaração e recusa de operação de ciclo de vida são erro com a mensagem da ANTT', () => {
    expect(resolveCiotToast({status: 'rejected', antt_message: 'RNTRC inativo'})).toEqual({
      variant: 'error', message: 'ANTT recusou a operação do CIOT; RNTRC inativo',
    });
    expect(resolveCiotToast({status: 'declared', internal_code: 'CIOT_CANC_221', antt_message: 'Prazo vencido'}).variant).toBe('error');
  });

  it('falha esgotada vira erro', () => {
    expect(resolveCiotToast({status: 'failed'}).variant).toBe('error');
    expect(resolveCiotToast({status: 'operation_failed'}).variant).toBe('error');
  });

  it('nenhum texto leva travessão', () => {
    for (const status of ['declared', 'rectified', 'cancelled', 'closed', 'rejected', 'failed', 'x']) {
      expect(resolveCiotToast({status}).message).not.toContain('—');
    }
  });
});
