import type {Metadata} from 'next';
import type {ReactNode} from 'react';
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld';
import {absoluteUrl} from '@/lib/seo/site';

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Notificações no navegador',
  description: 'Receba um aviso no celular ou no computador quando um documento for autorizado, rejeitado ou quando chegar uma nota para a sua empresa: ativar, instalar no iPhone, escolher os avisos e remover dispositivos.',
  alternates: {canonical: absoluteUrl('/guide/notifications')},
};

const LD = guideTopicLd({
  href: '/guide/notifications',
  title: 'Notificações no navegador',
  description: 'Receba um aviso no celular ou no computador quando um documento for autorizado, rejeitado ou quando chegar uma nota para a sua empresa: ativar, instalar no iPhone, escolher os avisos e remover dispositivos.',
});

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  );
}
