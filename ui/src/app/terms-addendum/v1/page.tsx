import type {Metadata} from 'next'
import {LegalPage, LegalSection} from '@/components/legal-page'

export const metadata: Metadata = {
  title: 'Termos Adicionais — CTech DFe',
}

const ADDENDUM_VERSION = '1.0'
const UPDATED_AT = '10 de julho de 2026'

export default function TermsAddendumPage() {
  return (
    <LegalPage title="Termos Adicionais — CTech DFe" updatedAt={UPDATED_AT}>
      <p className="text-xs text-gray-400">Versão {ADDENDUM_VERSION}</p>
      
      <LegalSection heading="1. Sobre este documento">
        <p>
          Este documento complementa os{' '}
          <a href="https://accounts.aoctech.app/terms" className="underline underline-offset-4" target="_blank"
             rel="noreferrer">
            Termos de Uso
          </a>{' '}
          e a{' '}
          <a href="https://accounts.aoctech.app/privacy" className="underline underline-offset-4" target="_blank"
             rel="noreferrer">
            Política de Privacidade
          </a>{' '}
          gerais da CTech, que você já aceitou ao criar sua conta. Ele descreve regras específicas do CTech DFe — a
          emissão e gestão de notas fiscais eletrônicas (NF-e, NFC-e, CT-e, MDF-e).
        </p>
      </LegalSection>
      
      <LegalSection heading="2. Dados de terceiros nas suas notas">
        <p>
          Para emitir uma nota fiscal, você informa dados de outras pessoas ou empresas — seus clientes,
          fornecedores ou destinatários (nome, CPF/CNPJ, endereço). Você é responsável por ter uma base legal
          válida para tratar esses dados (normalmente, a própria relação comercial) e por garantir que as
          informações estão corretas. O CTech DFe trata esses dados apenas para gerar e transmitir o documento
          fiscal correspondente.
        </p>
      </LegalSection>
      
      <LegalSection heading="3. Certificado digital">
        <p>
          O certificado digital (A1) que você envia é usado exclusivamente para assinar os documentos fiscais da
          sua própria empresa. Ele é armazenado de forma criptografada e nunca é compartilhado com outra
          organização dentro da plataforma.
        </p>
      </LegalSection>
      
      <LegalSection heading="4. Envio para a SEFAZ">
        <p>
          Notas fiscais são, por lei, transmitidas à Secretaria da Fazenda (SEFAZ) do seu estado. Esse envio é
          obrigatório para que o documento tenha validade fiscal — não é um compartilhamento opcional, é parte do
          próprio serviço.
        </p>
      </LegalSection>
      
      <LegalSection heading="5. Guarda dos documentos">
        <p>
          Documentos fiscais autorizados (XML e DANFE) ficam disponíveis para consulta e download na plataforma
          pelo prazo exigido pela legislação fiscal brasileira (em geral, 5 anos). Após esse período, podem ser
          arquivados ou removidos.
        </p>
      </LegalSection>
      
      <LegalSection heading="6. Alterações">
        <p>
          Alterações materiais a este documento exigem um novo aceite antes de continuar usando o CTech DFe. A
          versão vigente é sempre a publicada nesta página.
        </p>
      </LegalSection>
      
      <LegalSection heading="7. Contato">
        <p>
          Dúvidas sobre este documento:{' '}
          <a href="mailto:dpo@aoctech.app" className="underline underline-offset-4">
            dpo@aoctech.app
          </a>
          .
        </p>
      </LegalSection>
    </LegalPage>
  )
}
