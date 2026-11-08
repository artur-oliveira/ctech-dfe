import {afterEach, describe, expect, it, vi} from 'vitest';
import {cleanup, render, screen} from '@testing-library/react';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {ContingencyIndicator} from '@/components/layout/ContingencyIndicator';
import {DfeDetail} from '@/components/dfe/DfeDetail';
import {nfceDetailFixture, nfcesFixture} from '@/lib/mock/fixtures';
import type {NfeDetailOut} from '@/lib/types/api';

let docs: string[] = [];
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: '1'}})}));
vi.mock('@/lib/hooks/useFiscalConfig', () => ({useContingencyDocs: () => docs}));

describe('ContingencyIndicator', () => {
  afterEach(cleanup);

  it('some enquanto tudo é normal', () => {
    docs = [];
    const {container} = render(<ContingencyIndicator/>);
    expect(container).toBeEmptyDOMElement();
  });

  it('avisa e leva à aba do primeiro documento afetado', () => {
    docs = ['nfce', 'mdfe'];
    render(<ContingencyIndicator/>);
    const link = screen.getByRole('link', {name: /Em contingência: NFC-e, MDF-e/});
    expect(link).toHaveAttribute('href', '/fiscal-config?tab=nfce');
  });
});

describe('DfeDetail: NFC-e offline pendente', () => {
  afterEach(cleanup);

  const pending = {
    ...nfceDetailFixture, ...nfcesFixture[3], xml_s3_key: null,
  } as unknown as NfeDetailOut;

  function renderDetail(doc: NfeDetailOut, danfeWhilePending = true) {
    const qc = new QueryClient({defaultOptions: {queries: {retry: false}}});
    return render(
      <QueryClientProvider client={qc}>
        <DfeDetail
          accessKey={doc.sk} docLabel="NFC-e" enabled
          detailQueryKey={['d']} eventsQueryKey={['e']} listQueryKey={['l']}
          fetchDoc={async () => doc}
          fetchEvents={async () => ({items: [], next_cursor: null, has_next: false, previous_cursor: null, has_previous: false})}
          cancelFn={async () => undefined}
          downloadXml={async () => ({url: '', expires_at: '', filename: '', content_type: ''})}
          downloadEventXml={async () => ({url: '', expires_at: '', filename: '', content_type: ''})}
          downloadDanfe={async () => ({url: '', expires_at: '', filename: '', content_type: ''})}
          danfeWhilePending={danfeWhilePending}
        />
      </QueryClientProvider>,
    );
  }

  it('permite baixar o DANFC-e, mas não o XML, e mostra o aviso com prazo', async () => {
    renderDetail(pending);
    expect(await screen.findByRole('button', {name: 'DANFE'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'XML'})).not.toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent(/pendente de autorização; prazo até/);
  });

  it('sem a opção, o pendente continua sem DANFE', async () => {
    renderDetail(pending, false);
    await screen.findByRole('status');
    expect(screen.queryByRole('button', {name: 'DANFE'})).not.toBeInTheDocument();
  });
});
