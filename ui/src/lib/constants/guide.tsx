import type {ReactNode} from 'react'
import {
  BriefcaseIcon,
  CteIcon,
  MdfeIcon,
  NfceIcon,
  NfeIcon,
  NfseIcon,
  SettingsIcon,
  ShieldIcon,
  TruckIcon,
} from '@/components/ui/icon'

/**
 * Índice único do guia (/guide). Alimenta a home do guia, a navegação entre
 * tópicos e o teste que garante que todo tópico listado existe como rota.
 *
 * Toda feature nova entra num tópico existente (ou cria um aqui) — ver
 * `DOCS.md §5`, "Guia do produto (/guide) e capturas de tela".
 */
export interface GuideTopic {
  href: string
  label: string
  title: string
  time: string
  description: string
  tags: string[]
  icon: ReactNode
  /** Acento do tema por tipo de documento; ausente = verde da marca. */
  accent?: string
}

export const GUIDE_TOPICS: GuideTopic[] = [
  {
    href: '/guide/getting-started',
    label: 'Primeiros passos',
    title: 'Primeiros passos',
    time: '4 min',
    description:
      'Do cadastro da empresa ao certificado digital: o que precisa estar pronto antes da primeira emissão, e por que homologação vem antes de produção.',
    tags: ['Onboarding', 'Certificado', 'Homologação'],
    icon: <SettingsIcon width={20} height={20}/>,
  },
  {
    href: '/guide/nfe',
    label: 'NF-e',
    title: 'Emitir NF-e',
    time: '7 min',
    description:
      'Os quatro passos da emissão — destinatário, produtos, pagamento e revisão —, o rascunho automático e o que fazer com a nota depois de autorizada.',
    tags: ['Emissão', 'DANFE', 'Rascunho'],
    icon: <NfeIcon width={20} height={20}/>,
    accent: '#2ea87f',
  },
  {
    href: '/guide/nfce',
    label: 'NFC-e',
    title: 'Emitir NFC-e',
    time: '4 min',
    description:
      'A nota do balcão: venda a consumidor final, CSC por ambiente, cancelamento dentro do prazo e substituição de nota.',
    tags: ['PDV', 'CSC', 'Substituição'],
    icon: <NfceIcon width={20} height={20}/>,
    accent: '#3b82f6',
  },
  {
    href: '/guide/cte-mdfe',
    label: 'CT-e e MDF-e',
    title: 'Transporte — CT-e e MDF-e',
    time: '6 min',
    description:
      'Manifesto de carga com veículos, condutores e notas vinculadas; encerramento da viagem; e o que já dá para fazer com CT-e hoje.',
    tags: ['Manifesto', 'Encerramento', 'Veículos'],
    icon: <MdfeIcon width={20} height={20}/>,
    accent: '#f59e0b',
  },
  {
    href: '/guide/nfse',
    label: 'NFS-e',
    title: 'Emitir NFS-e',
    time: '5 min',
    description:
      'Nota de serviço no padrão nacional: catálogo de serviços, competência, tributação do ISS e cancelamento com motivo.',
    tags: ['Serviços', 'ISS', 'Competência'],
    icon: <NfseIcon width={20} height={20}/>,
    accent: '#0f766e',
  },
  {
    href: '/guide/events',
    label: 'Eventos',
    title: 'Eventos do documento',
    time: '5 min',
    description:
      'Cancelamento, carta de correção, encerramento e manifestação: o que cada evento faz, quando cabe e onde ler a resposta da SEFAZ.',
    tags: ['Cancelamento', 'CC-e', 'Manifestação'],
    icon: <ShieldIcon width={20} height={20}/>,
  },
  {
    href: '/guide/distributions',
    label: 'Distribuições',
    title: 'Documentos recebidos',
    time: '5 min',
    description:
      'A SEFAZ entrega por NSU tudo que foi emitido contra o seu CNPJ. Como consultar, importar por chave ou XML e manifestar-se.',
    tags: ['NSU', 'Importação', 'SEFAZ'],
    icon: <TruckIcon width={20} height={20}/>,
  },
  {
    href: '/guide/registries',
    label: 'Cadastros',
    title: 'Cadastros que a emissão usa',
    time: '6 min',
    description:
      'Pessoas, produtos, serviços, perfis fiscais, naturezas de operação, condições de pagamento e veículos — cada um preenche um pedaço da emissão.',
    tags: ['Produtos', 'Perfis fiscais', 'Operações'],
    icon: <BriefcaseIcon width={20} height={20}/>,
  },
  {
    href: '/guide/account',
    label: 'Conta',
    title: 'Organização, usuários e assinatura',
    time: '5 min',
    description:
      'Várias empresas numa conta, papéis e convites, plano e cobrança, configuração fiscal por documento e log de auditoria.',
    tags: ['Organizações', 'Plano', 'Auditoria'],
    icon: <CteIcon width={20} height={20}/>,
  },
]
