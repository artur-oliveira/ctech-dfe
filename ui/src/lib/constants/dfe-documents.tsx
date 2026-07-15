import type {ReactNode} from 'react'
import {CteIcon, MdfeIcon, NfceIcon, NfeIcon} from '@/components/ui/icon'

export interface DfeDocument {
  code: string
  title: string
  description: string
  href: string
  icon: ReactNode
  accent: string
}

export const DFE_DOCUMENTS: DfeDocument[] = [
  {
    code: 'NF-e',
    title: 'Emitir NF-e',
    description: 'Nota Fiscal Eletrônica para serviços e mercadorias',
    href: '/nfe',
    icon: <NfeIcon/>,
    accent: '#2ea87f',
  },
  {
    code: 'NFC-e',
    title: 'Emitir NFC-e',
    description: 'Nota Fiscal de Consumidor Eletrônica (PDV)',
    href: '/nfce',
    icon: <NfceIcon/>,
    accent: '#3b82f6',
  },
  {
    code: 'CT-e',
    title: 'Emitir CT-e',
    description: 'Conhecimento de Transporte Eletrônico',
    href: '/cte',
    icon: <CteIcon/>,
    accent: '#8b5cf6',
  },
  {
    code: 'MDF-e',
    title: 'Emitir MDF-e',
    description: 'Manifesto Eletrônico de Documentos Fiscais',
    href: '/mdfe',
    icon: <MdfeIcon/>,
    accent: '#f59e0b',
  },
]
