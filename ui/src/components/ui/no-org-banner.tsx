import Link from 'next/link'

export function NoOrgBanner() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-center">
      <div className="w-12 h-12 rounded-xl bg-amber-50 flex items-center justify-center mb-4 text-amber-500">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75"
             strokeLinecap="round" strokeLinejoin="round">
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
      </div>
      <p className="text-sm font-medium text-gray-900">Nenhuma organização selecionada</p>
      <p className="text-sm text-gray-500 mt-1">
        Selecione uma organização na barra superior ou{' '}
        <Link href="/organizations" className="text-brand-600 hover:underline">
          crie uma nova
        </Link>
        .
      </p>
    </div>
  )
}
