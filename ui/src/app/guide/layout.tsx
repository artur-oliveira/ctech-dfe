import type {Metadata} from 'next'
import {absoluteUrl} from '@/lib/seo/site'

// O layout raiz marca `noindex` (o produto é privado); o guia é público, então a
// liberação vale para este layout e para todos os tópicos abaixo dele.
export const metadata: Metadata = {
  // `title` como string consome o template da raiz e não o repassa adiante; o
  // par default/template mantém "<tópico> | CTech DF-e" nos filhos.
  title: {default: 'Guia', template: '%s | CTech DF-e'},
  alternates: {canonical: absoluteUrl('/guide')},
  robots: {index: true, follow: true},
  description:
    'Como emitir NF-e, NFC-e, CT-e, MDF-e e NFS-e no CTech DF-e: emissão passo a passo, eventos, distribuições e cadastros — com as telas reais do sistema.',
}

export default function GuideLayout({children}: { children: React.ReactNode }) {
  return children
}
