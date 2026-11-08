import {describe, it, expect, vi, beforeEach} from 'vitest';
import {render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {ApiError} from '@/lib/api/client';
import {nfeFormats, PRINT_FORMATS, MSG_FORMAT_INVALID, MSG_FORMAT_NOT_ALLOWED, MSG_XML_NOT_READY} from '@/lib/constants/document-formats';
import {DownloadPdfButton} from '@/components/dfe/DownloadPdfButton';

const triggerRemoteDownload = vi.fn();
const toastError = vi.fn();

vi.mock('@/lib/utils/dfe', () => ({
  triggerRemoteDownload: (...args: unknown[]) => triggerRemoteDownload(...args),
}));
vi.mock('sonner', () => ({
  toast: {error: (...args: unknown[]) => toastError(...args)},
}));

describe('DownloadPdfButton', () => {
  beforeEach(() => {
    triggerRemoteDownload.mockClear();
    toastError.mockClear();
  });

  it('opens the presigned URL of the generated document', async () => {
    const fetchPdf = vi.fn().mockResolvedValue({
      url: 'https://s3.example/ABC.pdf',
      expires_at: '2026-08-29T12:00:00Z',
      filename: 'ABC.pdf',
      content_type: 'application/pdf',
      cached: true,
    });
    render(<DownloadPdfButton fetchPdf={fetchPdf}/>);

    await userEvent.click(screen.getByRole('button', {name: 'DANFE'}));

    await waitFor(() => expect(triggerRemoteDownload).toHaveBeenCalledWith('https://s3.example/ABC.pdf'));
    expect(fetchPdf).toHaveBeenCalledOnce();
    expect(toastError).not.toHaveBeenCalled();
  });

  it('keeps the custom label while idle', async () => {
    const fetchPdf = vi.fn().mockResolvedValue({
      url: 'https://s3.example/MDFE.pdf',
      expires_at: '2026-08-29T12:00:00Z',
      filename: 'MDFE.pdf',
      content_type: 'application/pdf',
    });
    render(<DownloadPdfButton fetchPdf={fetchPdf} label="DAMDFE"/>);

    await userEvent.click(screen.getByRole('button', {name: 'DAMDFE'}));

    await waitFor(() => expect(triggerRemoteDownload).toHaveBeenCalledWith('https://s3.example/MDFE.pdf'));
  });

  it('shows an error toast when generation fails', async () => {
    const fetchPdf = vi.fn().mockRejectedValue(new Error('boom'));
    render(<DownloadPdfButton fetchPdf={fetchPdf}/>);

    await userEvent.click(screen.getByRole('button', {name: 'DANFE'}));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith('boom'));
    expect(triggerRemoteDownload).not.toHaveBeenCalled();
  });

  describe('seletor de formato', () => {
    const ok = {url: 'https://s3.example/x.pdf', expires_at: '2026-08-29T12:00:00Z', filename: 'x.pdf', content_type: 'application/pdf'};
    // A chave de 44 dígitos traz o tpEmis na posição 35.
    const keyWith = (tpEmis: string) => '3'.repeat(34) + tpEmis + '3'.repeat(9);

    it('sem escolha não envia format', async () => {
      const fetchPdf = vi.fn().mockResolvedValue(ok);
      render(<DownloadPdfButton fetchPdf={fetchPdf} formats={PRINT_FORMATS.nfe}/>);
      await userEvent.click(screen.getByRole('button', {name: 'DANFE'}));
      await waitFor(() => expect(fetchPdf).toHaveBeenCalledWith(undefined));
    });

    it('envia o formato escolhido', async () => {
      const fetchPdf = vi.fn().mockResolvedValue(ok);
      render(<DownloadPdfButton fetchPdf={fetchPdf} formats={PRINT_FORMATS.nfe}/>);
      await userEvent.selectOptions(screen.getByLabelText('Formato do impresso'), 'etiqueta');
      await userEvent.click(screen.getByRole('button', {name: 'DANFE'}));
      await waitFor(() => expect(fetchPdf).toHaveBeenCalledWith('etiqueta'));
    });

    it('EPEC esconde simplificado e etiqueta', () => {
      expect(nfeFormats(keyWith('4')).map((f) => f.value)).toEqual(['retrato', 'paisagem']);
      expect(nfeFormats(keyWith('1')).map((f) => f.value)).toContain('etiqueta');
    });

    it.each([
      [new ApiError(400, 'x', {code: 'invalid-document-format'}), MSG_FORMAT_INVALID],
      [new ApiError(422, 'x', {code: 'document-format-not-allowed'}), MSG_FORMAT_NOT_ALLOWED],
      [new ApiError(404, 'nada', {}), MSG_XML_NOT_READY],
    ])('mostra o problema em texto simples', async (err, message) => {
      render(<DownloadPdfButton fetchPdf={vi.fn().mockRejectedValue(err)} formats={PRINT_FORMATS.nfce} label="DANFC-e"/>);
      await userEvent.click(screen.getByRole('button', {name: 'DANFC-e'}));
      await waitFor(() => expect(toastError).toHaveBeenCalledWith(message));
    });
  });
});
