/**
 * Mock auth refresh. Stands in for `doRefresh()` (real OAuth + HttpOnly cookie)
 * when the dev mock is enabled, so the AuthProvider mount flow establishes a
 * session from fixtures instead of redirecting to ctech-account.
 */

export async function mockDoRefresh(): Promise<{ accessToken: string }> {
  return {accessToken: 'mock-access-token'}
}
