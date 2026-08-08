import {describe, expect, it} from 'vitest'
import {
  dfeStatusClasses,
  dfeStatusMotiveTitle,
  dfeStatusLabel,
  dfeStatusOptions,
  dfeStatusTone,
  isTransitionalDfeStatus,
  NFSE_STATUSES,
} from '@/lib/data/dfe_status'

// Espelha worker/internal/service/helpers.go — se o backend ganhar um status
// novo e ele não estiver aqui, este teste quebra antes do usuário ver o valor cru.
const BACKEND_STATUSES = [
  'pending', 'processing', 'retryable_failed', 'cancel_pending', 'close_pending',
  'authorized', 'rejected', 'failed', 'cancelled', 'closed',
  // NFS-e (StatusError) e eventos SEFAZ (EventStatusSuccess).
  'error', 'success',
]

describe('vocabulário de status de DF-e', () => {
  it('rotula todos os status que o backend produz', () => {
    for (const status of BACKEND_STATUSES) {
      expect(dfeStatusLabel(status), status).not.toBe(status)
    }
  })

  it('concorda o gênero com o substantivo do documento', () => {
    expect(dfeStatusLabel('authorized')).toBe('Autorizada')
    expect(dfeStatusLabel('authorized', 'm')).toBe('Autorizado')
    expect(dfeStatusLabel('cancelled', 'm')).toBe('Cancelado')
    expect(dfeStatusLabel('closed', 'm')).toBe('Encerrado')
    // Status sem gênero passa igual nos dois.
    expect(dfeStatusLabel('failed', 'm')).toBe('Falha')
    expect(dfeStatusLabel('processing')).toBe('Processando')
  })

  it('não inventa rótulo para status desconhecido', () => {
    expect(dfeStatusLabel('quantum')).toBe('quantum')
    expect(dfeStatusClasses('quantum')).toBe('bg-gray-100 text-gray-600')
    expect(isTransitionalDfeStatus('quantum')).toBe(false)
  })

  it('marca como em voo tudo que o worker ainda vai reprocessar', () => {
    for (const status of ['pending', 'processing', 'retryable_failed', 'cancel_pending', 'close_pending']) {
      expect(isTransitionalDfeStatus(status), status).toBe(true)
    }
    for (const status of ['authorized', 'rejected', 'failed', 'cancelled', 'closed', 'success']) {
      expect(isTransitionalDfeStatus(status), status).toBe(false)
    }
  })

  // retryable_failed é falha de transporte, não recusa do fisco: avisa (âmbar,
  // pulsando) em vez de alarmar (vermelho terminal).
  it('trata retryable_failed como aviso em voo, não como falha terminal', () => {
    expect(dfeStatusTone('retryable_failed')).toBe('warning')
    expect(dfeStatusTone('failed')).toBe('danger')
    expect(dfeStatusLabel('retryable_failed')).toBe('Tentando novamente')
  })

  it('abre o motivo só nos status que um motivo explica, nomeando a causa', () => {
    expect(dfeStatusMotiveTitle('rejected')).toBe('Motivo da rejeição')
    expect(dfeStatusMotiveTitle('failed')).toBe('Motivo da falha')
    expect(dfeStatusMotiveTitle('error')).toBe('Motivo da falha')
    // Retentativa não é rejeição — o modal não pode chamá-la assim.
    expect(dfeStatusMotiveTitle('retryable_failed')).toBe('Motivo da retentativa')
    for (const status of ['authorized', 'pending', 'cancelled', 'closed']) {
      expect(dfeStatusMotiveTitle(status), status).toBeNull()
    }
  })

  it('usa tom de alerta para rejeitada e sucesso para autorizada', () => {
    expect(dfeStatusTone('rejected')).toBe('danger')
    expect(dfeStatusTone('authorized')).toBe('success')
  })

  it('monta opções de filtro já rotuladas', () => {
    const options = dfeStatusOptions(NFSE_STATUSES)
    expect(options).toContainEqual({value: 'authorized', label: 'Autorizada'})
    // NFS-e não encerra: encerramento é só de MDF-e.
    expect(options.map((o) => o.value)).not.toContain('closed')
  })
})
