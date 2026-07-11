'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {Button} from '@/components/ui/button'
import {triggerDownload} from '@/lib/utils/dfe'

interface DownloadPdfButtonProps {
  /** Fetches the PDF blob (e.g. DANFC-e / DAMDFE) from the API. */
  fetchPdf: () => Promise<Blob>
  /** Downloaded filename without extension (e.g. the access key). */
  filename: string
  /** Button text when idle. Defaults to "DANFE". */
  label?: string
  variant?: 'outline' | 'ghost'
  size?: 'sm' | 'xs'
  className?: string
}

/**
 * Shared button that downloads a generated PDF (DANFC-e / DAMDFE) with an inline
 * loading state. Used in both list rows and detail headers across doc types.
 */
export function DownloadPdfButton({
  fetchPdf,
  filename,
  label = 'DANFE',
  variant = 'ghost',
  size = 'xs',
  className,
}: DownloadPdfButtonProps) {
  const [loading, setLoading] = useState(false)

  const handleClick = async () => {
    setLoading(true)
    try {
      triggerDownload(await fetchPdf(), `${filename}.pdf`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Erro ao gerar o PDF.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button variant={variant} size={size} onClick={handleClick} disabled={loading}
            className={className ?? 'text-brand-600 hover:text-brand-700'}>
      {loading ? 'Gerando…' : label}
    </Button>
  )
}
