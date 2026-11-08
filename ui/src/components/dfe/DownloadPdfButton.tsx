'use client';

import {useState} from 'react';
import {toast} from 'sonner';
import {Button} from '@aoctech/ui';
import {triggerRemoteDownload} from '@/lib/utils/dfe';
import type {SignedFileDownload} from '@/lib/types/api';
import {type PrintFormat, printProblemMessage} from '@/lib/constants/document-formats';

const DEFAULT_FORMAT = '';

interface DownloadPdfButtonProps {
  /** Fetches the presigned URL of the generated auxiliary document. */
  fetchPdf: (format?: string) => Promise<SignedFileDownload>
  /** Formatos oferecidos no seletor ao lado do botão; sem a lista, só o botão. Sem escolha, `format` não é enviado. */
  formats?: readonly PrintFormat[]
  /** Button text when idle. Defaults to "DANFE". */
  label?: string
  variant?: 'outline' | 'ghost'
  size?: 'sm'
  className?: string
}

/**
 * Shared button that downloads a generated PDF (DANFE / DANFC-e / DAMDFE /
 * DANFSe) from its presigned S3 URL, with an inline loading state. The API
 * never streams the file, so there is no Blob path here.
 */
export function DownloadPdfButton({
                                    fetchPdf,
                                    formats,
                                    label = 'DANFE',
                                    variant = 'ghost',
                                    size = 'sm',
                                    className,
                                  }: DownloadPdfButtonProps) {
  const [loading, setLoading] = useState(false);
  const [format, setFormat] = useState(DEFAULT_FORMAT);

  const handleClick = async () => {
    setLoading(true);
    try {
      triggerRemoteDownload((await fetchPdf(format || undefined)).url);
    } catch (e) {
      toast.error(printProblemMessage(e));
    } finally {
      setLoading(false);
    }
  };

  const button = (
    <Button variant={variant} size={size} onClick={handleClick} disabled={loading}
            className={className ?? 'text-brand-600 hover:text-brand-700'}>
      {loading ? 'Gerando…' : label}
    </Button>
  );
  if (!formats?.length) return button;

  return (
    <span className="inline-flex items-center gap-1">
      {button}
      <select aria-label="Formato do impresso" value={format} disabled={loading}
              onChange={(e) => setFormat(e.target.value)}
              className="h-11 sm:h-8 max-w-40 rounded-md border border-gray-200 bg-white px-2 text-sm text-gray-700">
        <option value={DEFAULT_FORMAT}>Formato padrão</option>
        {formats.map((f) => <option key={f.value} value={f.value}>{f.label}</option>)}
      </select>
    </span>
  );
}
