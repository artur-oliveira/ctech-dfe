import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {cleanup, render as rtlRender, screen} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import {PaymentLinkModal} from '@/components/dfe/PaymentLinkModal';
import type {NfeEventOut} from '@/lib/types/api';

afterEach(cleanup);

const render = (ui: React.ReactElement) => rtlRender(<QueryClientProvider client={new QueryClient()}>{ui}</QueryClientProvider>);

const base = {docLabel: 'NFCom', docNumber: 12, links: [] as NfeEventOut[], loading: false, onClose: vi.fn(), onConfirm: vi.fn()};

describe('PaymentLinkModal (110300 e 110301 da NFCom e do BP-e)', () => {
  it('sem tipo não renderiza nada', () => {
    const {container} = render(<PaymentLinkModal {...base} kind={null}/>);
    expect(container.querySelector('[role=dialog]')).toBeNull();
  });

  it('vinculação: título com o documento, campos do grupo pgto e envio travado até preencher', () => {
    render(<PaymentLinkModal {...base} kind="link"/>);
    expect(screen.getByText('Vincular pagamento (NFCom nº 12)')).toBeInTheDocument();
    expect(screen.getByLabelText('Número do pagamento')).toBeInTheDocument();
    expect(screen.getByLabelText(/Identificador da transação/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Vincular pagamento'})).toBeDisabled();
  });

  it('cancelamento: pede a vinculação autorizada e não envia sem escolher', () => {
    render(<PaymentLinkModal {...base} docLabel="BP-e" kind="cancel"
                             links={[{sk: 'ev#1', event_type: '110300', sequence_number: 1, status: 'success', created_at: '2026-11-05T10:00:00Z'} as NfeEventOut]}/>);
    expect(screen.getByText('Cancelar vinculação de pagamento (BP-e nº 12)')).toBeInTheDocument();
    expect(screen.getByLabelText('Vinculação a cancelar')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Cancelar vinculação'})).toBeDisabled();
  });
});
