import {cleanup, render, screen, waitFor} from '@testing-library/react'
import {QueryClientProvider, QueryClient} from '@tanstack/react-query'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import LinkCompanyPage from '@/app/organizations/link/page'
import {apiClient} from '@/lib/api/client'

const replace = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({replace, push: vi.fn()}),
}))

vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({children}: {children: React.ReactNode}) => <>{children}</>,
}))

vi.mock('@/components/layout/RootLayout', () => ({
  RootLayout: ({children}: {children: React.ReactNode}) => <>{children}</>,
}))

const refreshUser = vi.fn()
const setSelectedOrg = vi.fn()
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({refreshUser, setSelectedOrg}),
}))

afterEach(cleanup)

// The real URL, not a mocked hook. This app builds with `output: 'export'`,
// where useSearchParams() answers empty on the hydrating render — the page read
// that empty set and refused a complete return. Driving the test through
// window.location is what makes it fail on that bug.
function renderPage(query: string) {
  window.history.replaceState({}, '', `/organizations/link?${query}`)
  return render(
    <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
      <LinkCompanyPage/>
    </QueryClientProvider>,
  )
}

describe('a empresa criada na conta CTech volta para o DF-e', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    refreshUser.mockResolvedValue({organizations: [{pk: 'cmp_1', name: 'Acme'}]})
    vi.spyOn(apiClient, 'linkCompany').mockResolvedValue({pk: 'cmp_1'} as never)
  })

  it('vincula e leva para a tela da empresa', async () => {
    sessionStorage.setItem('dfe:handoff-state', 'abc')
    renderPage('organization_id=org_1&company_id=cmp_1&state=abc')

    await waitFor(() =>
      expect(apiClient.linkCompany).toHaveBeenCalledWith('org_1', 'cmp_1'),
    )
    await waitFor(() =>
      expect(replace).toHaveBeenCalledWith('/organizations/edit?pk=cmp_1'),
    )
  })

  // O state é o que distingue um retorno real de alguém abrindo esta URL com
  // ids que digitou. Sem a checagem, a rota vincula o que vier na query.
  it('recusa um retorno cujo state não é o que enviamos', async () => {
    sessionStorage.setItem('dfe:handoff-state', 'abc')
    renderPage('organization_id=org_1&company_id=cmp_1&state=outro')

    expect(await screen.findByText(/não corresponde ao cadastro iniciado/i)).toBeInTheDocument()
    expect(apiClient.linkCompany).not.toHaveBeenCalled()
  })

  // Cancelar é ação, não botão voltar: a conta avisa, e a pessoa não fica
  // olhando uma tela de carregamento que nunca termina.
  it('explica quando a pessoa desistiu na conta CTech', async () => {
    renderPage('cancelled=1')
    expect(await screen.findByText(/Nenhuma empresa foi criada/i)).toBeInTheDocument()
    expect(apiClient.linkCompany).not.toHaveBeenCalled()
  })

  // Um retorno sem ID nenhum não é vinculável nem recuperável: não há
  // organização para completar, então a saída é recomeçar.
  it('avisa quando o retorno vem incompleto', async () => {
    renderPage('state=abc')
    expect(await screen.findByText(/retorno está incompleto/i)).toBeInTheDocument()
    expect(apiClient.linkCompany).not.toHaveBeenCalled()
  })

  // A recusa do servidor é mostrada como ela é. É onde aparece "você não tem
  // acesso a esta empresa", que é a resposta a ids que alguém montou.
  it('mostra a recusa do servidor', async () => {
    const {ApiError} = await import('@/lib/api/client')
    vi.spyOn(apiClient, 'linkCompany').mockRejectedValue(
      new ApiError(403, 'você não tem acesso a esta empresa'),
    )
    sessionStorage.setItem('dfe:handoff-state', 'abc')
    renderPage('organization_id=org_1&company_id=cmp_1&state=abc')

    // The server's own words, not a generic failure: this is where "você não
    // tem acesso a esta empresa" surfaces, which is the answer to ids somebody
    // assembled by hand.
    expect(await screen.findByText(/não tem acesso a esta empresa/i)).toBeInTheDocument()
    expect(replace).not.toHaveBeenCalled()
  })

  // O buraco que isto fecha: a organização foi criada e a empresa não. Dizer
  // "comece de novo" estaria errado duas vezes — o espaço de trabalho existe, e
  // recomeçar cria um segundo.
  it('oferece cadastrar a empresa quando só a organização voltou', async () => {
    sessionStorage.setItem('dfe:handoff-state', 'abc')
    renderPage('organization_id=org_1&company_id=&state=abc')

    expect(await screen.findByText(/Falta a empresa/i)).toBeInTheDocument()
    expect(screen.getByRole('button', {name: /Cadastrar a empresa/i})).toBeInTheDocument()
    expect(screen.queryByText(/Comece novamente/i)).toBeNull()
    expect(apiClient.linkCompany).not.toHaveBeenCalled()
  })

  // Navegar sem a empresa na lista deixaria a empresa ANTERIOR selecionada, e a
  // tela da empresa mandaria o pk dela em cada requisição — a pessoa edita uma
  // empresa e escreve em outra.
  it('não navega quando a empresa vinculada ainda não aparece em /auth/me', async () => {
    refreshUser.mockResolvedValue({organizations: [{pk: 'outra', name: 'Antiga'}]})
    renderPage('organization_id=org_1&company_id=cmp_1&state=abc')

    expect(await screen.findByText(/ainda não apareceu na sua lista/i)).toBeInTheDocument()
    expect(setSelectedOrg).not.toHaveBeenCalled()
    expect(replace).not.toHaveBeenCalled()
  })
})
