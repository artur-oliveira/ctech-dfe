import type {Metadata} from 'next';
import type {ReactNode} from 'react';
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld';
import {absoluteUrl} from '@/lib/seo/site';

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Webhooks',
  description: 'Receba um aviso assinado no seu sistema quando um documento for autorizado, rejeitado ou falhar: cadastro, verificação da assinatura, tentativas e reenvio.',
  alternates: {canonical: absoluteUrl('/guide/webhooks')},
};

const LD = guideTopicLd({
  href: '/guide/webhooks',
  title: 'Webhooks',
  description: 'Receba um aviso assinado no seu sistema quando um documento for autorizado, rejeitado ou falhar: cadastro, verificação da assinatura, tentativas e reenvio.',
});

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  );
}
