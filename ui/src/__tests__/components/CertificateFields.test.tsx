import {describe, it, expect, vi} from 'vitest'
import {render, screen} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {CertificateFields} from '@/components/organizations/CertificateFields'

describe('CertificateFields', () => {
  it('emits the picked file and typed password', async () => {
    const onFileChange = vi.fn()
    const onPasswordChange = vi.fn()
    render(
      <CertificateFields
        file={null}
        onFileChange={onFileChange}
        password=""
        onPasswordChange={onPasswordChange}
      />,
    )

    const file = new File(['pfx-bytes'], 'cert.pfx', {type: 'application/x-pkcs12'})
    await userEvent.upload(screen.getByLabelText(/Arquivo do certificado/i), file)
    expect(onFileChange).toHaveBeenCalledWith(file)

    await userEvent.type(screen.getByLabelText(/Senha do certificado/i), 'x')
    expect(onPasswordChange).toHaveBeenCalledWith('x')
  })

  it('shows field errors and the selected file name', () => {
    const file = new File(['x'], 'meu-cert.pfx')
    render(
      <CertificateFields
        file={file}
        onFileChange={vi.fn()}
        password=""
        onPasswordChange={vi.fn()}
        fileError="Selecione um arquivo"
        passwordError="Senha é obrigatória"
        hint="Envie o certificado A1"
      />,
    )
    expect(screen.getByText('meu-cert.pfx')).toBeInTheDocument()
    expect(screen.getByText('Selecione um arquivo')).toBeInTheDocument()
    expect(screen.getByText('Senha é obrigatória')).toBeInTheDocument()
    expect(screen.getByText('Envie o certificado A1')).toBeInTheDocument()
  })
})
