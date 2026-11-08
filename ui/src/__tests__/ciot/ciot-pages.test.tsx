import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CiotPage from '@/app/ciot/page';
import CiotDetailPage from '@/app/ciot/detail/page';
import CiotEmitPage from '@/app/ciot/emit/page';
import {ApiError, apiClient} from '@/lib/api/client';
import {ciotsFixture} from '@/lib/mock/fixtures';

let permissions: string[] = ['*'];
let search = 'id=a1b2c3d4e5f6';

vi.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(search),
  useRouter: () => ({push: vi.fn()}),
}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: { children: React.ReactNode }) => children}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: { children: React.ReactNode }) => children}));
vi.mock('@/lib/hooks/useAuth', () => ({
  useAuth: () => ({selectedOrg: {pk: 'org', name: 'Empresa Mock', tax_id: '11222333000181', permissions}}),
}));
vi.mock('@/lib/hooks/useCiotEnvironment', () => ({useCiotEnvironment: () => ['prod', vi.fn()]}));
vi.mock('sonner', () => ({toast: {success: vi.fn(), error: vi.fn(), info: vi.fn()}}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, has_next: false, previous_cursor: null, has_previous: false});
const renderPage = (ui: React.ReactNode) =>
  render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>{ui}</QueryClientProvider>);

beforeEach(() => {
  vi.restoreAllMocks();
  permissions = ['*'];
  search = 'id=a1b2c3d4e5f6';
});

describe('/ciot (lista)', () => {
  it('lista os CIOTs e filtra pelo status', async () => {
    vi.spyOn(apiClient, 'listCiots').mockResolvedValue(page(ciotsFixture) as never);
    renderPage(<CiotPage/>);
    const table = await screen.findByRole('table', {name: 'CIOTs declarados'});
    expect(within(table).getAllByRole('row').length).toBe(1 + ciotsFixture.length);

    fireEvent.click(screen.getByRole('button', {name: 'Rejeitado'}));
    expect(within(table).getAllByRole('row').length).toBe(2);
    expect(within(table).getByText('Rejeitado')).toBeInTheDocument();
  });

  it('filtro sem resultado explica, sem tabela vazia', async () => {
    vi.spyOn(apiClient, 'listCiots').mockResolvedValue(page([ciotsFixture[0]]) as never);
    renderPage(<CiotPage/>);
    await screen.findByRole('table', {name: 'CIOTs declarados'});
    fireEvent.click(screen.getByRole('button', {name: 'Encerrado'}));
    expect(screen.getByText('Nenhum CIOT com esse status')).toBeInTheDocument();
  });

  it('lista vazia convida a declarar o primeiro', async () => {
    vi.spyOn(apiClient, 'listCiots').mockResolvedValue(page([]) as never);
    renderPage(<CiotPage/>);
    expect(await screen.findByText('Nenhum CIOT declarado')).toBeInTheDocument();
  });

  it('sem list.ciots não chama a api e avisa; o botão de declarar depende de create.ciots', async () => {
    permissions = ['create.ciots'];
    const spy = vi.spyOn(apiClient, 'listCiots');
    renderPage(<CiotPage/>);
    expect(screen.getByRole('alert')).toHaveTextContent('Você não tem permissão para listar CIOTs');
    expect(spy).not.toHaveBeenCalled();
    expect(screen.getByText('Declarar CIOT').closest('a')).toHaveAttribute('href', '/ciot/emit');
  });

  it('sem create.ciots não oferece declarar', async () => {
    permissions = ['list.ciots'];
    vi.spyOn(apiClient, 'listCiots').mockResolvedValue(page([]) as never);
    renderPage(<CiotPage/>);
    await screen.findByText('Nenhum CIOT declarado');
    expect(screen.queryByText('Declarar CIOT')).toBeNull();
  });
});

describe('/ciot/detail', () => {
  it('CIOT declarado mostra aviso ao transportador e as três ações', async () => {
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue({...ciotsFixture[0], operation_type: 2} as never);
    renderPage(<CiotDetailPage/>);
    expect(await screen.findByText('Aviso ao transportador')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Encerrar'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Cancelar'})).toBeInTheDocument();
    expect(screen.getByText('Retificar').closest('a')).toHaveAttribute('href', '/ciot/emit?rectify=a1b2c3d4e5f6');
  });

  it('carga lotação não oferece retificar (B121)', async () => {
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue(ciotsFixture[0] as never);
    renderPage(<CiotDetailPage/>);
    await screen.findByText('Aviso ao transportador');
    expect(screen.queryByText('Retificar')).toBeNull();
    expect(screen.getByRole('button', {name: 'Cancelar'})).toBeInTheDocument();
  });

  it('o botão Cancelar abre o diálogo com a dica das 24 horas', async () => {
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue(ciotsFixture[0] as never);
    renderPage(<CiotDetailPage/>);
    fireEvent.click(await screen.findByRole('button', {name: 'Cancelar'}));
    expect(await screen.findByText(/24 horas depois do início da viagem/)).toBeInTheDocument();
  });

  it('rejeição mostra a mensagem e os códigos da ANTT, e nenhuma ação', async () => {
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue(ciotsFixture[3] as never);
    renderPage(<CiotDetailPage/>);
    expect(await screen.findByText('Campo obrigatório não informado: RNTRC do contratado.')).toBeInTheDocument();
    expect(screen.getByText('Código ANTT: 229')).toBeInTheDocument();
    expect(screen.getByText('Código interno: CIOT_DECL_229')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Cancelar'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Encerrar'})).toBeNull();
  });

  it('operação em andamento avisa e esconde as ações', async () => {
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue({...ciotsFixture[0], pending_operation: 'close'} as never);
    renderPage(<CiotDetailPage/>);
    expect(await screen.findByRole('status')).toHaveTextContent('Encerramento em andamento');
    expect(screen.queryByRole('button', {name: 'Cancelar'})).toBeNull();
  });

  it('sem update.ciots mostra o CIOT sem ações', async () => {
    permissions = ['get.ciots'];
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue(ciotsFixture[0] as never);
    renderPage(<CiotDetailPage/>);
    await screen.findByText('Aviso ao transportador');
    expect(screen.queryByRole('button', {name: 'Cancelar'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Encerrar'})).toBeNull();
  });

  it('sem get.ciots não chama a api', () => {
    permissions = ['list.ciots'];
    const spy = vi.spyOn(apiClient, 'getCiot');
    renderPage(<CiotDetailPage/>);
    expect(screen.getByRole('alert')).toHaveTextContent('ver CIOTs');
    expect(spy).not.toHaveBeenCalled();
  });

  it('CIOT inexistente (404) mostra não encontrado', async () => {
    vi.spyOn(apiClient, 'getCiot').mockRejectedValue(new ApiError(404, 'nada'));
    renderPage(<CiotDetailPage/>);
    expect(await screen.findByText('CIOT não encontrado.')).toBeInTheDocument();
  });
});

describe('/ciot/emit', () => {
  it('sem create.ciots mostra o aviso e nenhum formulário', () => {
    permissions = ['list.ciots'];
    search = '';
    renderPage(<CiotEmitPage/>);
    expect(screen.getByRole('alert')).toHaveTextContent('declarar CIOT');
    expect(screen.queryByRole('button', {name: 'Declarar CIOT'})).toBeNull();
  });

  it('retificar exige update.ciots', () => {
    permissions = ['create.ciots'];
    search = 'rectify=a1b2c3d4e5f6';
    renderPage(<CiotEmitPage/>);
    expect(screen.getByRole('alert')).toHaveTextContent('retificar CIOT');
  });

  it('retificar carga lotação é recusado na tela', async () => {
    search = 'rectify=a1b2c3d4e5f6';
    vi.spyOn(apiClient, 'getCiot').mockResolvedValue(ciotsFixture[0] as never);
    renderPage(<CiotEmitPage/>);
    expect(await screen.findByText(/não pode ser retificado agora/)).toBeInTheDocument();
  });
});
