'use client';

import Link from 'next/link';
import {usePathname} from 'next/navigation';
import {BottomNav, type RenderLink} from '@aoctech/ui';
import {FileText, LayoutGrid, Menu, Plus, Search} from 'lucide-react';
import {contextForPath, DOC_CONTEXTS, isItemActive} from '@/lib/navigation/nav';

const renderLink: RenderLink = props => <Link {...props}/>;

interface MobileNavProps {
  onOpenMenu: () => void
  onOpenSearch: () => void
}

/**
 * Navegação primária do mobile: painel, troca de documento, emissão, busca e menu.
 * Só traduz o contexto fiscal da rota para o `BottomNav` de `@aoctech/ui`; a barra
 * lateral segue disponível em "Menu" para tudo o que é secundário.
 */
export function MobileNav({onOpenMenu, onOpenSearch}: MobileNavProps) {
  const pathname = usePathname();
  const activeContext = contextForPath(pathname);
  // CT-e ainda não emite: nesse contexto a ação cai no primeiro tipo que emite.
  const emitContext = activeContext?.emit ? activeContext : DOC_CONTEXTS.find(c => c.emit)!;

  return (
    <BottomNav
      renderLink={renderLink}
      items={[
        {label: 'Painel', icon: <LayoutGrid/>, href: '/dashboard', active: isItemActive('/dashboard', pathname)},
        {
          type: 'more',
          label: activeContext?.label ?? 'Documentos',
          icon: <FileText/>,
          title: 'Documentos fiscais',
          active: activeContext !== null,
          items: [
            ...DOC_CONTEXTS.map(ctx => ({
              label: ctx.label,
              icon: ctx.icon,
              href: ctx.href,
              active: isItemActive(ctx.href, pathname),
              group: 'Documentos fiscais',
            })),
            ...(activeContext?.items ?? []).map(item => ({
              label: item.label,
              icon: item.icon,
              href: item.href,
              active: isItemActive(item.href, pathname),
              group: `Cadastros de ${activeContext!.label}`,
            })),
          ],
        },
        {label: 'Buscar', icon: <Search/>, onClick: onOpenSearch},
        {label: 'Menu', icon: <Menu/>, onClick: onOpenMenu},
      ]}
      action={{label: `Emitir ${emitContext.label}`, icon: <Plus/>, href: emitContext.emit!.href}}
    />
  );
}
