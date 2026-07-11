interface HomologationBannerProps {
  environment: number | undefined
}

export function HomologationBanner({environment}: HomologationBannerProps) {
  if (environment !== 2) return null
  return (
    <div className="flex items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 mb-6">
      <p className="text-sm font-medium text-amber-800">
        <span className="text-amber-600 mr-2">⚠</span>
        Ambiente de <strong>Homologação</strong> — sem validade fiscal.
      </p>
      <a href="/fiscal-config" className="shrink-0 text-xs font-semibold text-amber-700 underline">
        Usar produção
      </a>
    </div>
  )
}
