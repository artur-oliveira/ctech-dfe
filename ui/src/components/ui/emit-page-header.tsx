import Link from 'next/link';
import {PageHeader} from '@aoctech/ui';

/**
 * Cabeçalho compacto das telas de emissão em passos: trilha + título (+ subtítulo,
 * escondido no mobile e em viewport baixa para o passo caber sem rolar). Fino sobre o
 * `PageHeader` de `@aoctech/ui`; só adiciona a trilha e a regra do subtítulo.
 */
export function EmitPageHeader({parent, parentHref, crumb, title, subtitle}: {
  parent: string
  parentHref: string
  crumb: string
  title: string
  subtitle?: string
}) {
  return (
    <PageHeader
      title={title}
      description={subtitle}
      className="mb-3 space-y-1 md:mb-4 [&_p]:max-md:hidden [&_p]:[@media(max-height:820px)]:hidden"
      lead={(
        <nav aria-label="Trilha" className="flex items-center gap-2 text-sm text-gray-400">
          <Link href={parentHref} className="hover:text-brand-600">{parent}</Link>
          <span aria-hidden="true">/</span>
          <span className="text-gray-600">{crumb}</span>
        </nav>
      )}
    />
  );
}
