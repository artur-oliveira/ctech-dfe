import {cleanup, render, screen} from '@testing-library/react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {afterEach, describe, expect, it, vi} from 'vitest';
import {RecentDocuments} from '@/components/dashboard/RecentDocuments';
import {apiClient} from '@/lib/api/client';

vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'org1'}})}));

afterEach(cleanup);

function renderIt() {
  return render(
    <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
      <RecentDocuments/>
    </QueryClientProvider>,
  );
}

describe('RecentDocuments', () => {
  it('lists emitted NF-e with a link to the detail', async () => {
    vi.spyOn(apiClient, 'getNfes').mockResolvedValue({
      items: [{sk: '3'.repeat(44), number: 42, dest_name: 'Cliente X', status: 'authorized', total: '10.00',
        dh_emi: '2026-01-02T10:00:00Z', created_at: '2026-01-02T10:00:00Z'}],
    } as never);
    renderIt();
    expect(await screen.findByText(/NF-e 42/)).toBeTruthy();
    expect(screen.getByRole('link', {name: /NF-e 42/}).getAttribute('href')).toBe(`/nfe/detail?key=${'3'.repeat(44)}`);
  });

  it('renders nothing when there are no documents', async () => {
    vi.spyOn(apiClient, 'getNfes').mockResolvedValue({items: []} as never);
    const {container} = renderIt();
    await vi.waitFor(() => expect(container.querySelector('section')).toBeNull());
  });
});
