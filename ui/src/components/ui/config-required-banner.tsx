import Link from 'next/link'
import type {DocVariant} from '@/lib/schemas/fiscal-configs'

interface ConfigRequiredBannerProps {
  show: boolean
  variant: DocVariant
  docLabel: string
}

export function ConfigRequiredBanner({show, variant, docLabel}: ConfigRequiredBannerProps) {
  if (!show) return null
  return (
    <div
      className="flex items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 mb-6">
      <p className="text-sm font-medium text-amber-800">
        <span className="text-amber-600 mr-2">⚠</span>
        Configuração fiscal de <strong>{docLabel}</strong> pendente — emissão indisponível até configurar.
      </p>
      <Link href={`/fiscal-config?tab=${variant}`} className="shrink-0 text-xs font-semibold text-amber-700 underline">
        Configurar agora
      </Link>
    </div>
  )
}
