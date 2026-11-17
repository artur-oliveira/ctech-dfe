import {describe, expect, it, vi} from 'vitest';
import {render, screen} from '@testing-library/react';
import {ApiError} from '@/lib/api/client';
import {RequireFiscalConfig} from '@/components/dfe/RequireFiscalConfig';

const state = vi.hoisted(() => ({fiscal: {} as Record<string, unknown>}));

vi.mock('next/navigation', () => ({useRouter: () => ({replace: vi.fn()})}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'org-1'}})}));
vi.mock('@/lib/hooks/useFiscalConfig', () => ({useFiscalConfig: () => state.fiscal}));
vi.mock('@/lib/hooks/useSubscriptionNotice', () => ({useSubscriptionNotice: () => ({notice: null, isPending: false})}));

describe('RequireFiscalConfig', () => {
  it('403 na configuração mostra o aviso de permissão, sem skeleton e sem abrir o formulário', () => {
    state.fiscal = {isPending: false, isMissing: false, config: undefined, error: new ApiError(403, 'escopo do token insuficiente', {})};
    render(<RequireFiscalConfig variant="bpe"><div>formulario</div></RequireFiscalConfig>);
    expect(screen.getByRole('alert').textContent).toContain('Sem permissão');
    expect(screen.queryByText('formulario')).toBeNull();
  });

  it('configuração carregada abre o formulário', () => {
    state.fiscal = {isPending: false, isMissing: false, config: {}, error: null};
    render(<RequireFiscalConfig variant="bpe"><div>formulario</div></RequireFiscalConfig>);
    expect(screen.getByText('formulario')).toBeTruthy();
  });
});
