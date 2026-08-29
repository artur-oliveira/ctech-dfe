'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {Button} from '@/components/ui/button'
import {triggerDownload, triggerRemoteDownload} from '@/lib/utils/dfe'
import type {AuxiliaryDocumentDownload} from '@/lib/types/api'

type PdfDownload = Blob | AuxiliaryDocumentDownload

interface DownloadPdfButtonProps {
  /** Fetches either a legacy PDF blob or a cached auxiliary-document URL. */
  fetchPdf: () => Promise<PdfDownload>
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
      const download = await fetchPdf()
      if (download instanceof Blob) {
        triggerDownload(download, `${filename}.pdf`)
      } else {
        triggerRemoteDownload(download.url)
      }
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
