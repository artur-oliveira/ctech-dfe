interface ComingSoonProps {
  title: string
  description?: string
}

export function ComingSoon({title, description}: ComingSoonProps) {
  return (
    <div className="flex flex-1 items-center justify-center p-12">
      <div className="text-center max-w-sm">
        <div
          className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-amber-50 text-amber-500">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5"
               strokeLinecap="round" strokeLinejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <polyline points="12 6 12 12 16 14"/>
          </svg>
        </div>
        <h2 className="text-base font-semibold text-gray-900">{title}</h2>
        <p className="mt-1.5 text-sm text-gray-500">
          {description ?? 'Esta funcionalidade está em desenvolvimento e estará disponível em breve.'}
        </p>
      </div>
    </div>
  )
}
