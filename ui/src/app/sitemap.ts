import type {MetadataRoute} from 'next'
import {GUIDE_TOPICS} from '@/lib/constants/guide'
import {absoluteUrl} from '@/lib/seo/site'

// `output: 'export'` exige a rota estática — ela é gerada no build.
export const dynamic = 'force-static'

/**
 * Gerado estaticamente em `/sitemap.xml`. Só entram as rotas públicas: a landing
 * e o guia, cujos tópicos saem de `GUIDE_TOPICS` — tópico novo entra no sitemap
 * sozinho.
 */
export default function sitemap(): MetadataRoute.Sitemap {
  const lastModified = new Date()
  return [
    {url: absoluteUrl('/'), lastModified, changeFrequency: 'monthly', priority: 1},
    {url: absoluteUrl('/guide'), lastModified, changeFrequency: 'monthly', priority: 0.8},
    ...GUIDE_TOPICS.map(topic => ({
      url: absoluteUrl(topic.href),
      lastModified,
      changeFrequency: 'monthly' as const,
      priority: 0.6,
    })),
  ]
}
