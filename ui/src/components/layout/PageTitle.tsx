'use client'

import {useEffect} from 'react'
import {usePathname} from 'next/navigation'
import {documentTitleForPath} from '@/lib/navigation/page-title'

/** Rotas públicas — o título delas vem do `metadata` do Next, que é o que os
 *  buscadores leem. Sobrescrever aqui só atrapalharia. */
const isPublicRoute = (pathname: string) =>
  pathname === '/' || pathname === '/guide' || pathname.startsWith('/guide/')

/**
 * Dá nome à aba em toda tela autenticada. As páginas do app são client
 * components — não podem exportar `metadata` —, e um `layout.tsx` por rota seria
 * boilerplate que sai de sincronia. O nome sai de `lib/navigation/page-title`,
 * derivado da mesma configuração da navegação.
 *
 * O `metadata` do Next é aplicado depois da hidratação, em momento que nenhum
 * timer acerta: ele reescreve o título logo depois do nosso efeito. Por isso o
 * título é reafirmado por um observer no `<head>` — determinístico, e não uma
 * corrida contra o framework.
 */
export function PageTitle() {
  const pathname = usePathname()

  useEffect(() => {
    if (isPublicRoute(pathname)) return
    const desired = documentTitleForPath(pathname)
    const apply = () => {
      if (document.title !== desired) document.title = desired
    }
    apply()

    const observer = new MutationObserver(apply)
    observer.observe(document.head, {childList: true, subtree: true, characterData: true})
    return () => observer.disconnect()
  }, [pathname])

  return null
}
