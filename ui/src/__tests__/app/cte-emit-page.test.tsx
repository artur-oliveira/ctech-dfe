import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import CteEmitPage from '@/app/cte/emit/page';

let permissions: string[] = [];
vi.mock('@/components/ProtectedRoute', () => ({ProtectedRoute: ({children}: {children: React.ReactNode}) => children}));
vi.mock('@/components/layout/RootLayout', () => ({RootLayout: ({children}: {children: React.ReactNode}) => children}));
vi.mock('@/components/dfe/RequireFiscalConfig', () => ({RequireFiscalConfig: ({children}: {children: React.ReactNode}) => children}));
vi.mock('@/components/cte/CteEmitForm', () => ({CteEmitForm: () => <p>formulário de emissão</p>}));
vi.mock('@/lib/hooks/useAuth', () => ({useAuth: () => ({selectedOrg: {pk: 'CNPJ_11222333000181', permissions}})}));

describe('página de emissão de CT-e', () => {
  it('sem create.ctes mostra o aviso de permissão e não monta o formulário', () => {
    permissions = ['read.ctes'];
    render(<CteEmitPage/>);
    expect(screen.getByText(/não tem permissão/i)).toBeInTheDocument();
    expect(screen.queryByText('formulário de emissão')).not.toBeInTheDocument();
  });

  it('com create.ctes monta o formulário', () => {
    permissions = ['create.ctes'];
    render(<CteEmitPage/>);
    expect(screen.getByText('formulário de emissão')).toBeInTheDocument();
    expect(screen.getByRole('heading', {level: 1, name: 'Emitir CT-e'})).toBeInTheDocument();
  });
});
