import {describe, expect, it} from 'vitest'
import {currentReturnTo} from '@/lib/auth/oauth'

// The ctech-account handoff comes back to /organizations/link carrying the
// organization and company it just created. Somebody without a live session is
// sent through OAuth on arrival, and the return address is what survives that
// round-trip — so a return address without the query is the whole bug: the page
// came back to a bare /organizations/link and refused a return with no ids.
describe('currentReturnTo', () => {
  const at = (url: string) => {
    window.history.replaceState({}, '', url)
    return currentReturnTo()
  }

  it('keeps the query the handoff came back with', () => {
    expect(at('/organizations/link?organization_id=org_1&company_id=co_1&state=abc')).toBe(
      '/organizations/link?organization_id=org_1&company_id=co_1&state=abc',
    )
  })

  it('returns the path alone when there is no query', () => {
    expect(at('/dashboard')).toBe('/dashboard')
  })

  it('never sends anybody back to /callback, where the code is already spent', () => {
    expect(at('/callback?code=abc&state=def')).toBe('/')
  })
})
