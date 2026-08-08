'use client'

import {Button} from '@/components/ui/button'

function savedAgo(savedAt: number): string {
  const minutes = Math.round((Date.now() - savedAt) / 60_000)
  if (minutes < 1) return 'agora há pouco'
  if (minutes < 60) return `há ${minutes} min`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `há ${hours} h`
  return `há ${Math.round(hours / 24)} dia(s)`
}

/**
 * Offers back an emission the user left unfinished. Shown only when a draft
 * exists, and never applied automatically — restoring silently would be worse
 * than losing it.
 */
export function DraftRecoveryBanner({savedAt, onRestore, onDiscard}: {
  savedAt: number
  onRestore: () => void
  onDiscard: () => void
}) {
  return (
    <div className="mb-4 flex flex-col sm:flex-row sm:items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3">
      <p className="flex-1 text-sm text-gray-700">
        Você tem uma emissão não finalizada, salva {savedAgo(savedAt)}.
      </p>
      <div className="flex items-center gap-2 shrink-0">
        <Button type="button" variant="outline" onClick={onDiscard}>Descartar</Button>
        <Button type="button" variant="brand" onClick={onRestore}>Retomar</Button>
      </div>
    </div>
  )
}
