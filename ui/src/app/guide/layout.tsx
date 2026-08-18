import type {Metadata} from 'next'

export const metadata: Metadata = {
  title: 'Guia',
  description:
    'Como emitir NF-e, NFC-e, CT-e, MDF-e e NFS-e no CTech DF-e: emissão passo a passo, eventos, distribuições e cadastros — com as telas reais do sistema.',
}

export default function GuideLayout({children}: { children: React.ReactNode }) {
  return children
}
