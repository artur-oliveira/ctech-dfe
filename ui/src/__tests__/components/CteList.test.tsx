import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CTePage from '@/app/cte/page';
import {apiClient} from '@/lib/api/client';
import {ctesFixture} from '@/lib/mock/fixtures';

const ORG = '11222333000181';
vi.mock('next/navigation', () => ({useRouter: () => ({push: vi.fn()}), useSearchParams: () => new URLSearchParams()}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: ORG, tax_id: ORG, permissions: ['*']}})}));
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => <>{children}</>}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => <>{children}</>}));

beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(apiClient, 'getCTeConfig').mockResolvedValue({environment: 2} as never);
  vi.spyOn(apiClient, 'getCtes').mockResolvedValue(
    {items: ctesFixture, next_cursor: null, previous_cursor: null, has_next: false, has_previous: false} as never,
  );
});

describe('lista de CT-e emitidos', () => {
  it('mostra a espécie de cada documento e não quebra sem remetente, destinatário ou tomador', async () => {
    render(<QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}><CTePage/></QueryClientProvider>);
    await screen.findByRole('link', {name: 'Abrir CT-e 40'});
    const kinds = screen.getAllByTestId('cte-kind').map((e) => e.textContent);
    expect(kinds.filter((k) => k === 'Normal')).toHaveLength(8);
    expect(kinds).toEqual(expect.arrayContaining(['Complemento', 'Substituto', 'CT-e OS']));
    expect(screen.getAllByText('Participantes do CT-e original')).toHaveLength(2);
    // O CT-e OS não tem remetente e destinatário: a linha diz o serviço.
    expect(screen.getByText('Transporte de pessoas')).toBeInTheDocument();
    expect(screen.getByText('Emitir CT-e OS').closest('a')).toHaveAttribute('href', '/cte/os');
  });
});
