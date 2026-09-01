import {absoluteUrl, PUBLIC_ORIGIN} from '@/lib/seo/site'

const PRODUCT_NAME = 'CTech DF-e'

/**
 * Renderiza um bloco JSON-LD. O objeto é sempre constante deste repositório —
 * nunca entrada de usuário. Ainda assim `<` é escapado, que é o único caractere
 * capaz de fechar a tag `script` a partir de dentro de uma string JSON.
 */
export function JsonLd({data}: { data: Record<string, unknown> }) {
  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{__html: JSON.stringify(data).replace(/</g, '\\u003c')}}
    />
  )
}

/** Dados estruturados da landing: o que o produto é e quem o publica. */
export const softwareApplicationLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: PRODUCT_NAME,
  url: PUBLIC_ORIGIN,
  applicationCategory: 'BusinessApplication',
  operatingSystem: 'Web',
  inLanguage: 'pt-BR',
  description:
    'Plataforma SaaS para emissão de NF-e, NFC-e, CT-e, MDF-e e NFS-e com comunicação direta com a SEFAZ.',
  featureList: [
    'Emissão de NF-e (modelo 55)',
    'Emissão de NFC-e (modelo 65)',
    'Emissão de MDF-e (modelo 58)',
    'Emissão de NFS-e (padrão nacional)',
    'Distribuição de documentos recebidos por NSU',
    'Eventos: cancelamento, carta de correção, manifestação e encerramento',
    'API REST para integração',
  ],
  offers: {
    '@type': 'Offer',
    priceCurrency: 'BRL',
    category: 'subscription',
    url: PUBLIC_ORIGIN,
  },
  publisher: {
    '@type': 'Organization',
    name: 'AOC Tech',
    url: PUBLIC_ORIGIN,
    logo: absoluteUrl('/app.svg'),
  },
}

/** Dados estruturados de um tópico do guia: artigo técnico + trilha. */
export function guideTopicLd({href, title, description}: {
  href: string
  title: string
  description: string
}) {
  return {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'TechArticle',
        headline: title,
        description,
        inLanguage: 'pt-BR',
        url: absoluteUrl(href),
        isPartOf: {'@type': 'WebSite', name: PRODUCT_NAME, url: PUBLIC_ORIGIN},
        publisher: {'@type': 'Organization', name: 'AOC Tech', url: PUBLIC_ORIGIN},
      },
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          {'@type': 'ListItem', position: 1, name: PRODUCT_NAME, item: PUBLIC_ORIGIN},
          {'@type': 'ListItem', position: 2, name: 'Guia', item: absoluteUrl('/guide')},
          {'@type': 'ListItem', position: 3, name: title, item: absoluteUrl(href)},
        ],
      },
    ],
  }
}
