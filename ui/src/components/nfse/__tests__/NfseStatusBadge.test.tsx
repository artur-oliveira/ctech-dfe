import {describe, expect, it} from 'vitest'
import {nfseStatusLabel, nfseStatusTone} from '@/components/nfse/NfseStatusBadge'

describe('status de NFS-e', () => {
  it('rotula todos os status que o backend produz', () => {
    expect(nfseStatusLabel('processing')).toBe('Processando')
    expect(nfseStatusLabel('authorized')).toBe('Autorizada')
    expect(nfseStatusLabel('rejected')).toBe('Rejeitada')
    expect(nfseStatusLabel('cancelled')).toBe('Cancelada')
  })

  it('não inventa rótulo para status desconhecido', () => {
    expect(nfseStatusLabel('quantum')).toBe('quantum')
  })

  it('usa tom de alerta para rejeitada', () => {
    expect(nfseStatusTone('rejected')).toBe('danger')
    expect(nfseStatusTone('authorized')).toBe('success')
  })
})
