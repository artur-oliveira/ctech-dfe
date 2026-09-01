import type {Metadata} from 'next'
import type {ReactNode} from 'react'
import {JsonLd, softwareApplicationLd} from '@/lib/seo/json-ld'
import {PUBLIC_ORIGIN} from '@/lib/seo/site'

/**
 * A landing é a única rota do app que os buscadores devem indexar além do guia.
 * O layout raiz marca `noindex` para todo o resto (o produto é privado), então a
 * liberação vive aqui — por isso a página mora num route group, e não em
 * `app/page.tsx`.
 */
export const metadata: Metadata = {
  title: {
    absolute: 'CTech DF-e — Emissão de NF-e, NFC-e, CT-e, MDF-e e NFS-e',
  },
  description:
    'Emita NF-e, NFC-e, CT-e, MDF-e e NFS-e com comunicação direta com a SEFAZ, status em tempo real e API para integrar ao seu sistema.',
  alternates: {canonical: PUBLIC_ORIGIN},
  robots: {index: true, follow: true},
  openGraph: {
    title: 'CTech DF-e — Emissão de documentos fiscais eletrônicos',
    description:
      'Emita NF-e, NFC-e, CT-e, MDF-e e NFS-e com comunicação direta com a SEFAZ e status em tempo real.',
    url: PUBLIC_ORIGIN,
    type: 'website',
  },
}

export default function PublicLayout({children}: { children: ReactNode }) {
  return (
    <>
      <JsonLd data={softwareApplicationLd}/>
      {children}
    </>
  )
}
