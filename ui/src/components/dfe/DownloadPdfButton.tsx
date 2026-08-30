'use client'

import {useState} from 'react'
import {toast} from 'sonner'
import {Button} from '@/components/ui/button'
import {triggerRemoteDownload} from '@/lib/utils/dfe'
import type {SignedFileDownload} from '@/lib/types/api'

interface DownloadPdfButtonProps {
  /** Fetches the presigned URL of the generated auxiliary document. */
  fetchPdf: () => Promise<SignedFileDownload>
  /** Button text when idle. Defaults to "DANFE". */
  label?: string
  variant?: 'outline' | 'ghost'
  size?: 'sm' | 'xs'
  className?: string
}

/**
 * Shared button that downloads a generated PDF (DANFE / DANFC-e / DAMDFE /
 * DANFSe) from its presigned S3 URL, with an inline loading state. The API
 * never streams the file, so there is no Blob path here.
 */
export function DownloadPdfButton({
                                    fetchPdf,
                                    label = 'DANFE',
                                    variant = 'ghost',
                                    size = 'xs',
                                    className,
                                  }: DownloadPdfButtonProps) {
  const [loading, setLoading] = useState(false)

  const handleClick = async () => {
    setLoading(true)
    try {
      triggerRemoteDownload((await fetchPdf()).url)
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
