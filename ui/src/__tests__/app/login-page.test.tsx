import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, waitFor} from '@testing-library/react'
import LoginPage from '@/app/login/page'

const startOAuthFlowMock = vi.fn()

vi.mock('@/lib/auth/oauth', () => ({
  startOAuthFlow: (returnTo: string) => startOAuthFlowMock(returnTo),
}))

let searchParams: URLSearchParams

vi.mock('next/navigation', () => ({
  useSearchParams: () => searchParams,
}))

describe('LoginPage', () => {
  beforeEach(() => {
    startOAuthFlowMock.mockReset()
  })

  it('defaults returnTo to /dashboard when no returnTo query param is present', async () => {
    // Regression: landing page and 404 page link to /login with no ?returnTo,
    // which used to default to '/' and strand the user back on the landing page after login.
    searchParams = new URLSearchParams()
    render(<LoginPage />)

    await waitFor(() => expect(startOAuthFlowMock).toHaveBeenCalledWith('/dashboard'))
  })

  it('honors an explicit returnTo query param', async () => {
    searchParams = new URLSearchParams({returnTo: '/nfe/123'})
    render(<LoginPage />)

    await waitFor(() => expect(startOAuthFlowMock).toHaveBeenCalledWith('/nfe/123'))
  })
})
