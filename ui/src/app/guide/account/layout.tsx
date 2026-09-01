import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Organização, usuários e assinatura',
  description: 'Várias empresas numa conta, papéis e convites, plano e cobrança, configuração fiscal por documento e log de auditoria.',
  alternates: {canonical: absoluteUrl('/guide/account')},
}

const LD = guideTopicLd({
  href: '/guide/account',
  title: 'Organização, usuários e assinatura',
  description: 'Várias empresas numa conta, papéis e convites, plano e cobrança, configuração fiscal por documento e log de auditoria.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
