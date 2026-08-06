import {describe, expect, it} from 'vitest'
import {NFSE_STEPS} from '@/components/nfse/NfseEmitForm'

describe('passos do wizard de NFS-e', () => {
  it('tem cinco passos na ordem da spec', () => {
    expect(NFSE_STEPS.map((s) => s.id)).toEqual(
      ['prestador', 'tomador', 'servico', 'valores', 'revisao'],
    )
  })
})
