import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {guideTopicLd, JsonLd} from '@/lib/seo/json-ld'
import {absoluteUrl} from '@/lib/seo/site'

// Título e descrição saem do mesmo texto que o índice do guia mostra — ver
// `lib/constants/guide.tsx`.
export const metadata: Metadata = {
  title: 'Emitir NFS-e',
  description: 'Nota de serviço no padrão nacional: catálogo de serviços, competência, tributação do ISS e cancelamento com motivo.',
  alternates: {canonical: absoluteUrl('/guide/nfse')},
}

const LD = guideTopicLd({
  href: '/guide/nfse',
  title: 'Emitir NFS-e',
  description: 'Nota de serviço no padrão nacional: catálogo de serviços, competência, tributação do ISS e cancelamento com motivo.',
})

export default function Layout({children}: {children: ReactNode}) {
  return (
    <>
      <JsonLd data={LD}/>
      {children}
    </>
  )
}
