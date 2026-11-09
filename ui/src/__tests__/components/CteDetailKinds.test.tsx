import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CteDetailPage from '@/app/cte/detail/page';
import {apiClient} from '@/lib/api/client';
import {cteDetailFixture, cteEventsFixture, ctesFixture} from '@/lib/mock/fixtures';

const ORG = '11222333000181';
let currentKey = '';
let permissions: string[] = ['*'];
vi.mock('next/navigation', () => ({
  useRouter: () => ({push: vi.fn()}),
  useSearchParams: () => new URLSearchParams({key: currentKey}),
}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG, permissions}})}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => <>{children}</>}));

const page = <T,>(items: T[]) => ({items, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false});
const [NORMAL, SUBSTITUTED, , , , , , , COMPLEMENT, SUBSTITUTE, OS] = ctesFixture;

function renderDetail(sk: string) {
  currentKey = sk;
  vi.spyOn(apiClient, 'getCte').mockResolvedValue(cteDetailFixture(sk) as never);
  vi.spyOn(apiClient, 'getCteEvents').mockResolvedValue(page(cteEventsFixture(sk)) as never);
  const client = new QueryClient({defaultOptions: {queries: {retry: false}}});
  return render(<QueryClientProvider client={client}><CteDetailPage/></QueryClientProvider>);
}

beforeEach(() => {
  vi.restoreAllMocks();
  permissions = ['*'];
});

describe('detalhe do CT-e: espécie e relacionados', () => {
  it('CT-e normal autorizado: espécie Normal e as ações de complemento e substituto levam ao original', async () => {
    renderDetail(NORMAL.sk);
    await screen.findByText(/CT-e 31/);
    expect(screen.getByTestId('cte-kind')).toHaveTextContent('Normal');
    expect(screen.getByText('Emitir complemento').closest('a')).toHaveAttribute('href', `/cte/complement?key=${NORMAL.sk}`);
    expect(screen.getByText('Emitir substituto').closest('a')).toHaveAttribute('href', `/cte/substitute?key=${NORMAL.sk}`);
    expect(screen.queryByRole('region', {name: 'Complementa'})).toBeNull();
  });

  it('sem a permissão create.ctes as ações de emissão não aparecem', async () => {
    permissions = ['get.ctes'];
    renderDetail(NORMAL.sk);
    await screen.findByText(/CT-e 31/);
    expect(screen.queryByText('Emitir complemento')).toBeNull();
    expect(screen.queryByText('Emitir substituto')).toBeNull();
  });

  it('complemento: mostra a espécie e o link para o CT-e complementado, sem participantes nem notas, sem nova emissão', async () => {
    renderDetail(COMPLEMENT.sk);
    await screen.findByText(/CT-e 38/);
    expect(screen.getByTestId('cte-kind')).toHaveTextContent('Complemento');
    const related = screen.getByRole('region', {name: 'Complementa'});
    expect(related.querySelector('a')).toHaveAttribute('href', `/cte/detail?key=${NORMAL.sk}`);
    expect(screen.queryByText('Remetente')).toBeNull();
    expect(screen.queryByText(/^Notas \(/)).toBeNull();
    expect(screen.queryByText('Emitir complemento')).toBeNull();
  });

  it('substituto: link para o CT-e substituído', async () => {
    renderDetail(SUBSTITUTE.sk);
    await screen.findByText(/CT-e 39/);
    expect(screen.getByTestId('cte-kind')).toHaveTextContent('Substituto');
    const related = screen.getByRole('region', {name: 'Substitui'});
    expect(related.querySelector('a')).toHaveAttribute('href', `/cte/detail?key=${SUBSTITUTED.sk}`);
  });

  it('original: links para o complementar e o substituto, e expedidor e recebedor do XML', async () => {
    renderDetail(NORMAL.sk);
    await screen.findByText(/CT-e 31/);
    const comp = screen.getByRole('region', {name: 'Complementado por'});
    expect(comp.querySelector('a')).toHaveAttribute('href', `/cte/detail?key=${COMPLEMENT.sk}`);
    expect(screen.getByText('Expedidor')).toBeInTheDocument();
    expect(screen.getByText('Recebedora Mock LTDA')).toBeInTheDocument();
  });

  it('original com substituto: link em Substituído por', async () => {
    renderDetail(SUBSTITUTED.sk);
    await screen.findByText(/CT-e 30/);
    expect(screen.getByRole('region', {name: 'Substituído por'}).querySelector('a')).toHaveAttribute('href', `/cte/detail?key=${SUBSTITUTE.sk}`);
  });

  it('CT-e OS: DACTE e só cancelamento e carta de correção; sem entrega nem desacordo', async () => {
    renderDetail(OS.sk);
    await screen.findByText(/CT-e 40/);
    expect(screen.getByRole('button', {name: 'Baixar DACTE'})).toBeInTheDocument();
    expect(screen.getByText('Carta de correção')).toBeInTheDocument();
    expect(screen.getByText('Cancelar')).toBeInTheDocument();
    for (const hidden of ['Comprovante de entrega', 'Insucesso de entrega', 'Prestação em desacordo', 'Cancelar entrega', 'Cancelar desacordo']) {
      expect(screen.queryByText(hidden)).toBeNull();
    }
    expect(screen.queryByText(/apenas o tomador/)).toBeNull();
  });

  it('CT-e normal segue com todos os eventos', async () => {
    renderDetail(NORMAL.sk);
    await screen.findByText(/CT-e 31/);
    expect(screen.getByText('Comprovante de entrega')).toBeInTheDocument();
    expect(screen.getByText('Insucesso de entrega')).toBeInTheDocument();
  });

  it('CT-e OS: chave modelo 67 abre, mostra o serviço e não oferece complemento', async () => {
    renderDetail(OS.sk);
    await screen.findByText(/CT-e 40/);
    expect(screen.getByTestId('cte-kind')).toHaveTextContent('CT-e OS');
    expect(screen.getByText('Transporte de pessoas')).toBeInTheDocument();
    expect(screen.queryByText('Emitir complemento')).toBeNull();
    expect(screen.queryByText(/não é de um CT-e válido/)).toBeNull();
  });
});
