import type {MetadataRoute} from 'next'
import {absoluteUrl, CRAWLABLE_PATTERNS} from '@/lib/seo/site'

// `output: 'export'` exige a rota estática — ela é gerada no build.
export const dynamic = 'force-static'

/**
 * Gerado estaticamente em `/robots.txt` pelo `output: 'export'`.
 *
 * A regra é pelo avesso: bloqueia tudo e libera a landing e o guia. Rota nova de
 * app nasce bloqueada — não há lista para alguém esquecer de atualizar.
 */
export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: '*',
      allow: [...CRAWLABLE_PATTERNS],
      disallow: '/',
    },
    sitemap: absoluteUrl('/sitemap.xml'),
    host: absoluteUrl(''),
  }
}
