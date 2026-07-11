import {describe, it, expect, vi, beforeEach} from 'vitest'
import {render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton'

const triggerDownload = vi.fn()
const toastError = vi.fn()

vi.mock('@/lib/utils/dfe', () => ({
  triggerDownload: (...args: unknown[]) => triggerDownload(...args),
}))
vi.mock('sonner', () => ({
  toast: {error: (...args: unknown[]) => toastError(...args)},
}))

describe('DownloadPdfButton', () => {
  beforeEach(() => {
    triggerDownload.mockClear()
    toastError.mockClear()
  })

  it('downloads the fetched PDF with a .pdf filename', async () => {
    const blob = new Blob(['%PDF'], {type: 'application/pdf'})
    const fetchPdf = vi.fn().mockResolvedValue(blob)
    render(<DownloadPdfButton fetchPdf={fetchPdf} filename="ABC" label="DAMDFE"/>)

    await userEvent.click(screen.getByRole('button', {name: 'DAMDFE'}))

    await waitFor(() => expect(triggerDownload).toHaveBeenCalledWith(blob, 'ABC.pdf'))
    expect(fetchPdf).toHaveBeenCalledOnce()
    expect(toastError).not.toHaveBeenCalled()
  })

  it('shows an error toast when generation fails', async () => {
    const fetchPdf = vi.fn().mockRejectedValue(new Error('boom'))
    render(<DownloadPdfButton fetchPdf={fetchPdf} filename="ABC"/>)

    await userEvent.click(screen.getByRole('button', {name: 'DANFE'}))

    await waitFor(() => expect(toastError).toHaveBeenCalledWith('boom'))
    expect(triggerDownload).not.toHaveBeenCalled()
  })
})
