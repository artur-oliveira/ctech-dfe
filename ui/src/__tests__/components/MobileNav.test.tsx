import {describe, it, expect, vi, afterEach} from 'vitest';
import {render, screen, fireEvent, cleanup} from '@testing-library/react';
import {MobileNav} from '@/components/layout/MobileNav';

let pathname = '/dashboard';

vi.mock('next/navigation', () => ({
  usePathname: () => pathname,
}));

function renderAt(path: string, props: Partial<{onOpenMenu: () => void; onOpenSearch: () => void}> = {}) {
  pathname = path;
  return render(
    <MobileNav onOpenMenu={props.onOpenMenu ?? (() => {})} onOpenSearch={props.onOpenSearch ?? (() => {})}/>,
  );
}

describe('MobileNav', () => {
  afterEach(cleanup);

  it('points the primary action at the current document type', () => {
    renderAt('/nfse');
    expect(screen.getByRole('link', {name: 'Emitir NFS-e'})).toHaveAttribute('href', '/nfse/emit');
  });

  it('falls back to a type that can be issued when the context cannot', () => {
    // CT-e ainda não tem tela de emissão.
    renderAt('/cte');
    expect(screen.getByRole('link', {name: 'Emitir NF-e'})).toHaveAttribute('href', '/nfe/emit');
  });

  it('opens a sheet with the document types and the context registries', () => {
    renderAt('/mdfe');
    const toggle = screen.getByRole('button', {expanded: false});
    fireEvent.click(toggle);
    expect(screen.getByRole('link', {name: 'NFC-e'})).toHaveAttribute('href', '/nfce');
    expect(screen.getByRole('link', {name: 'Composições veiculares'})).toBeInTheDocument();
  });

  it('delegates search and the full menu to the layout', () => {
    const onOpenSearch = vi.fn();
    const onOpenMenu = vi.fn();
    renderAt('/dashboard', {onOpenSearch, onOpenMenu});
    fireEvent.click(screen.getByRole('button', {name: /Buscar/}));
    fireEvent.click(screen.getByRole('button', {name: 'Menu'}));
    expect(onOpenSearch).toHaveBeenCalledOnce();
    expect(onOpenMenu).toHaveBeenCalledOnce();
  });
});
