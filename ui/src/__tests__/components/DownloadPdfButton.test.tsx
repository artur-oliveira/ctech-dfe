import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'

const triggerRemoteDownload = vi.fn()
const toastError = vi.fn()

vi.mock('@/lib/utils/dfe', () => ({
  triggerRemoteDownload: (...args: unknown[]) => triggerRemoteDownload(...args),
}))
vi.mock('sonner', () => ({
  toast: {error: (...args: unknown[]) => toastError(...args)},
}))

describe('DownloadPdfButton', () => {
  beforeEach(() => {
    triggerRemoteDownload.mockClear()
    toastError.mockClear()
  })

  it('opens the presigned URL of the generated document', async () => {
    const fetchPdf = vi.fn().mockResolvedValue({
      url: 'https://s3.example/ABC.pdf',
      expires_at: '2026-08-29T12:00:00Z',
      filename: 'ABC.pdf',
      content_type: 'application/pdf',
      cached: true,
    })
    render(<DownloadPdfButton fetchPdf={fetchPdf}/>)

    await userEvent.click(screen.getByRole('button', {name: 'DANFE'}))

    await waitFor(() => expect(triggerRemoteDownload).toHaveBeenCalledWith('https://s3.example/ABC.pdf'))
    expect(fetchPdf).toHaveBeenCalledOnce()
    expect(toastError).not.toHaveBeenCalled()
  })

  it('keeps the custom label while idle', async () => {
    const fetchPdf = vi.fn().mockResolvedValue({
      url: 'https://s3.example/MDFE.pdf',
      expires_at: '2026-08-29T12:00:00Z',
      filename: 'MDFE.pdf',
      content_type: 'application/pdf',
    })
    render(<DownloadPdfButton fetchPdf={fetchPdf} label="DAMDFE"/>)

    await userEvent.click(screen.getByRole('button', {name: 'DAMDFE'}))

    await waitFor(() => expect(triggerRemoteDownload).toHaveBeenCalledWith('https://s3.example/MDFE.pdf'))
  })

  it('shows an error toast when generation fails', async () => {
    const fetchPdf = vi.fn().mockRejectedValue(new Error('boom'))
    render(<DownloadPdfButton fetchPdf={fetchPdf}/>)

    await userEvent.click(screen.getByRole('button', {name: 'DANFE'}))

    await waitFor(() => expect(toastError).toHaveBeenCalledWith('boom'))
    expect(triggerRemoteDownload).not.toHaveBeenCalled()
  })
})
