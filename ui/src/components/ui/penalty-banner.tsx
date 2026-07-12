interface PenaltyBannerProps {
  message: string
  onDismiss: () => void
}

export function PenaltyBanner({message, onDismiss}: PenaltyBannerProps) {
  return (
    <div
      className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
      <span className="mt-0.5 shrink-0">⚠</span>
      <p className="flex-1">{message}</p>
      <button onClick={onDismiss} className="shrink-0 text-amber-600 hover:text-amber-800" aria-label="Fechar">✕
      </button>
    </div>
  )
}
