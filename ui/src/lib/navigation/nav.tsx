import type {ReactNode} from 'react'
import {BookOpen} from 'lucide-react'
import {
  BriefcaseIcon,
  CalendarClockIcon,
  CreditCardIcon,
  CteIcon,
  FuelPumpIcon,
  ImportIcon,
  InsuranceIcon,
  LotIcon,
  MdfeIcon,
  NfceIcon,
  NfeIcon,
  NfseIcon,
  PackageIcon,
  PercentIcon,
  RouteIcon,
  ServiceIcon,
  SettingsIcon,
  ShieldIcon,
  ShoppingBagIcon,
  TruckIcon,
  UsersIcon,
  VehicleSetIcon,
} from '@/components/ui/icon'
import {BuildingIcon, CardIcon, ClipboardIcon, GridIcon} from '@/components/ui/nav-icons'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {ROLE_ADMIN, ROLE_OWNER} from '@/lib/data/roles'
import {SUBSCRIPTION_PATH} from '@/lib/billing/notice'
import type {DfeThemeKey} from '@/lib/theme/dfe-theme'

/**
 * Fonte única da navegação do app: barra lateral, navegação de contexto por
 * documento, tema DF-e por rota e o índice da busca global (⌘K).
 *
 * Toda página nova entra aqui — é isso que a torna navegável e pesquisável.
 * Ver `../../../CLAUDE.md`, "Navegação e busca global".
 */

export interface NavItem {
  href: string
  label: string
  icon: ReactNode
  /** Termos extras para a busca global (sinônimos, jargão fiscal, sigla). */
  keywords?: string[]
  /** Quando definido, o item só aparece para membros com um destes papéis. */
  roles?: string[]
}

export interface NavGroup {
  label: string
  items: NavItem[]
}

/** Um tipo de documento e os cadastros que só existem por causa dele. */
export interface DocContext {
  key: DfeThemeKey
  href: string
  label: string
  icon: ReactNode
  keywords?: string[]
  /** Ação primária do contexto (emissão), quando existe. */
  emit?: NavItem
  /** Cadastros exclusivos deste contexto. */
  items: NavItem[]
}

export const DOC_CONTEXTS: DocContext[] = [
  {
    key: 'nfe',
    href: '/nfe',
    label: 'NF-e',
    icon: <NfeIcon/>,
    keywords: ['nota fiscal eletrônica', 'modelo 55', 'danfe', 'mercadoria'],
    emit: {href: '/nfe/emit', label: 'Emitir NF-e', icon: <NfeIcon/>, keywords: ['nova nota', 'emissão']},
    items: [
      {
        href: '/operations', label: 'Naturezas de operação', icon: <RouteIcon/>,
        keywords: ['cfop', 'natureza', 'venda', 'devolução', 'remessa'],
      },
      {
        href: '/payment-terms', label: 'Condições de pagamento', icon: <CalendarClockIcon/>,
        keywords: ['prazo', 'parcelamento', 'duplicata', 'fatura'],
      },
      {
        href: '/tax-profiles', label: 'Perfis fiscais', icon: <PercentIcon/>,
        keywords: ['tributação', 'icms', 'pis', 'cofins', 'ipi', 'cst', 'csosn'],
      },
      {
        href: '/import-declarations', label: 'Declarações de importação', icon: <ImportIcon/>,
        keywords: ['di', 'importação', 'adição', 'siscomex'],
      },
      {
        href: '/product-lots', label: 'Lotes de produção', icon: <LotIcon/>,
        keywords: ['rastreabilidade', 'lote', 'validade', 'medicamento'],
      },
    ],
  },
  {
    key: 'nfce',
    href: '/nfce',
    label: 'NFC-e',
    icon: <NfceIcon/>,
    keywords: ['cupom', 'consumidor final', 'modelo 65', 'balcão', 'pdv'],
    emit: {href: '/nfce/emit', label: 'Emitir NFC-e', icon: <NfceIcon/>, keywords: ['venda', 'cupom']},
    items: [
      {
        href: '/payment-terminals', label: 'Terminais de pagamento', icon: <CreditCardIcon/>,
        keywords: ['maquininha', 'cartão', 'credenciadora', 'tef', 'pos'],
      },
      {
        href: '/fuel-pumps', label: 'Bombas de combustível', icon: <FuelPumpIcon/>,
        keywords: ['posto', 'encerrante', 'bico', 'tanque'],
      },
    ],
  },
  {
    key: 'cte',
    href: '/cte',
    label: 'CT-e',
    icon: <CteIcon/>,
    keywords: ['conhecimento de transporte', 'frete', 'modelo 57'],
    items: [],
  },
  {
    key: 'mdfe',
    href: '/mdfe',
    label: 'MDF-e',
    icon: <MdfeIcon/>,
    keywords: ['manifesto', 'carga', 'viagem', 'modelo 58'],
    emit: {href: '/mdfe/emit', label: 'Emitir MDF-e', icon: <MdfeIcon/>, keywords: ['manifesto', 'viagem']},
    items: [
      {
        href: '/vehicles', label: 'Veículos', icon: <TruckIcon/>,
        keywords: ['placa', 'renavam', 'tara', 'caminhão', 'reboque'],
      },
      {
        href: '/vehicle-sets', label: 'Composições veiculares', icon: <VehicleSetIcon/>,
        keywords: ['carreta', 'conjunto', 'engate', 'bitrem'],
      },
      {
        href: '/cargo-units', label: 'Unidades de carga', icon: <PackageIcon/>,
        keywords: ['container', 'pallet', 'lacre', 'ulc'],
      },
      {
        href: '/toll-providers', label: 'Vale-pedágio', icon: <RouteIcon/>,
        keywords: ['pedágio', 'fornecedor', 'ciot'],
      },
      {
        href: '/insurance-policies', label: 'Apólices de seguro', icon: <InsuranceIcon/>,
        keywords: ['seguradora', 'averbação', 'apólice', 'rctrc'],
      },
    ],
  },
  {
    key: 'nfse',
    href: '/nfse',
    label: 'NFS-e',
    icon: <NfseIcon/>,
    keywords: ['serviço', 'iss', 'município', 'adn', 'padrão nacional'],
    emit: {href: '/nfse/emit', label: 'Emitir NFS-e', icon: <NfseIcon/>, keywords: ['serviço', 'iss']},
    items: [
      {
        href: '/services', label: 'Serviços', icon: <ServiceIcon/>,
        keywords: ['item lc 116', 'código de tributação', 'iss', 'catálogo'],
      },
      {
        href: '/service-locations', label: 'Locais de prestação', icon: <BriefcaseIcon/>,
        keywords: ['município', 'obra', 'endereço da prestação'],
      },
      {
        href: '/reference-documents', label: 'Documentos referenciados', icon: <ImportIcon/>,
        keywords: ['nota referenciada', 'documento anterior', 'substituição'],
      },
    ],
  },
]

/** Cadastros usados por mais de um contexto — só estes ficam globais. */
export const SHARED_REGISTRIES: NavItem[] = [
  {
    href: '/persons', label: 'Pessoas', icon: <UsersIcon/>,
    keywords: ['cliente', 'fornecedor', 'destinatário', 'transportadora', 'condutor', 'tomador', 'cpf', 'cnpj'],
  },
  {
    href: '/products', label: 'Produtos', icon: <ShoppingBagIcon/>,
    keywords: ['mercadoria', 'item', 'ncm', 'cest', 'gtin', 'ean', 'catálogo'],
  },
]

export const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Visão Geral',
    items: [
      {href: '/dashboard', label: 'Painel', icon: <GridIcon/>, keywords: ['home', 'início', 'resumo', 'dashboard']},
      {href: '/guide', label: 'Guia', icon: <BookOpen size={16}/>, keywords: ['ajuda', 'documentação', 'tutorial']},
    ],
  },
  {
    label: 'Documentos Fiscais',
    items: DOC_CONTEXTS.map(({href, label, icon, keywords}) => ({href, label, icon, keywords})),
  },
  {
    label: 'Cadastros',
    items: SHARED_REGISTRIES,
  },
  {
    label: 'Configurações',
    items: [
      {
        href: '/organizations', label: 'Organizações', icon: <BuildingIcon/>,
        keywords: ['empresa', 'cnpj', 'filial', 'emitente'],
      },
      {
        href: '/members', label: 'Usuários', icon: <UsersIcon/>, roles: [ROLE_OWNER, ROLE_ADMIN],
        keywords: ['membros', 'equipe', 'permissões', 'convite'],
      },
      // USER e VIEWER nunca veem: não podem agir sobre o plano nem lê-lo —
      // `GET /organizations/{pk}/plan` é OWNER/ADMIN.
      {
        href: SUBSCRIPTION_PATH, label: 'Assinatura', icon: <CardIcon/>, roles: [ROLE_OWNER, ROLE_ADMIN],
        keywords: ['plano', 'cobrança', 'fatura', 'pagamento', 'billing'],
      },
      {
        href: '/fiscal-config', label: 'Configuração Fiscal', icon: <SettingsIcon/>,
        keywords: ['série', 'numeração', 'ambiente', 'homologação', 'produção', 'csc'],
      },
      {
        href: '/certificates', label: 'Certificados', icon: <ShieldIcon/>,
        keywords: ['a1', 'pfx', 'certificado digital', 'validade'],
      },
      {
        href: '/audit-logs', label: 'Log de Auditoria', icon: <ClipboardIcon/>,
        keywords: ['histórico', 'auditoria', 'quem fez'],
      },
    ],
  },
]

/** Páginas que não moram na barra lateral mas precisam ser encontráveis. */
const EXTRA_SEARCHABLE: NavItem[] = [
  {href: '/profile', label: 'Meu perfil', icon: <UsersIcon/>, keywords: ['conta', 'senha', 'nome', 'e-mail']},
  {
    href: '/onboarding', label: 'Onboarding', icon: <SettingsIcon/>,
    keywords: ['primeiros passos', 'configuração inicial', 'começar'],
  },
]

export interface SearchEntry {
  href: string
  label: string
  icon: ReactNode
  /** Trilha exibida no resultado ("NFS-e · Cadastros"). */
  context: string
  keywords: string[]
  roles?: string[]
}

function toEntry(item: NavItem, context: string): SearchEntry {
  return {
    href: item.href,
    label: item.label,
    icon: item.icon,
    context,
    keywords: item.keywords ?? [],
    roles: item.roles,
  }
}

/**
 * Índice da busca global. Derivado da mesma configuração que desenha a
 * navegação, então uma página nova na barra lateral ou num contexto já entra
 * na busca sem passo extra.
 */
export const SEARCH_ENTRIES: SearchEntry[] = [
  ...NAV_GROUPS.flatMap(group => group.items.map(item => toEntry(item, group.label))),
  ...DOC_CONTEXTS.flatMap(ctx => [
    ...(ctx.emit ? [toEntry(ctx.emit, ctx.label)] : []),
    ...ctx.items.map(item => toEntry(item, `${ctx.label} · Cadastros`)),
  ]),
  ...EXTRA_SEARCHABLE.map(item => toEntry(item, 'Conta')),
  // Os tópicos do guia entram pelas próprias tags — quem procura "atalhos" ou
  // "certificado" cai na explicação, não só na tela.
  ...GUIDE_TOPICS.map(topic => ({
    href: topic.href,
    label: topic.title,
    icon: topic.icon,
    context: 'Guia',
    keywords: [topic.label, ...topic.tags],
  })),
]

/** Rota -> contexto de documento, para tema e navegação secundária. */
export const CONTEXT_BY_HREF: Record<string, DfeThemeKey> = Object.fromEntries(
  DOC_CONTEXTS.flatMap(ctx => [
    [ctx.href, ctx.key] as const,
    ...(ctx.emit ? [[ctx.emit.href, ctx.key] as const] : []),
    ...ctx.items.map(item => [item.href, ctx.key] as const),
  ]),
)

/** O contexto de documento ao qual a rota pertence, ou `null` fora deles. */
export function contextForPath(pathname: string): DocContext | null {
  const match = Object.keys(CONTEXT_BY_HREF)
    .filter(href => pathname === href || pathname.startsWith(href + '/'))
    // A rota mais específica vence (`/nfe/emit` antes de `/nfe`).
    .sort((a, b) => b.length - a.length)[0]
  if (!match) return null
  const key = CONTEXT_BY_HREF[match]
  return DOC_CONTEXTS.find(ctx => ctx.key === key) ?? null
}

/** Todas as rotas da navegação — usado para resolver o item ativo mais específico. */
const ALL_HREFS = [
  ...NAV_GROUPS.flatMap(g => g.items.map(i => i.href)),
  ...DOC_CONTEXTS.flatMap(c => [...(c.emit ? [c.emit.href] : []), ...c.items.map(i => i.href)]),
]

export function isItemActive(href: string, pathname: string): boolean {
  if (pathname === href) return true
  if (!pathname.startsWith(href + '/')) return false
  // Um item mais profundo tem precedência — não marque o pai como ativo.
  return !ALL_HREFS.some(
    other =>
      other !== href &&
      other.startsWith(href + '/') &&
      (pathname === other || pathname.startsWith(other + '/')),
  )
}
