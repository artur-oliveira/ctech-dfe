import type {Metadata} from 'next'
import {LegalPage, LegalSection} from '@/components/legal-page'

export const metadata: Metadata = {
  title: 'Termos Adicionais — CTech DF-e',
  description:
    'Termos adicionais aplicáveis à utilização do CTech DF-e.',
}

const ADDENDUM_VERSION = '2.0'
const UPDATED_AT = '12 de julho de 2026'

export default function TermsAddendumPage() {
  return (
    <LegalPage
      title="Termos Adicionais — CTech DF-e"
      updatedAt={UPDATED_AT}
    >
      <p className="text-xs text-gray-400">
        Versão {ADDENDUM_VERSION}
      </p>

      <p>
        Este documento complementa os{' '}
        <a
          href="https://accounts.aoctech.app/terms"
          target="_blank"
          rel="noreferrer"
          className="underline underline-offset-4"
        >
          Termos de Uso
        </a>{' '}
        e a{' '}
        <a
          href="https://accounts.aoctech.app/privacy"
          target="_blank"
          rel="noreferrer"
          className="underline underline-offset-4"
        >
          Política de Privacidade
        </a>{' '}
        da plataforma CTech e disciplina especificamente a utilização do
        serviço CTech DF-e.
      </p>

      <LegalSection heading="1. Objeto do serviço">
        <p>
          O CTech DF-e é uma plataforma destinada à emissão, transmissão,
          gerenciamento, armazenamento e consulta de documentos fiscais
          eletrônicos, incluindo, entre outros:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>NF-e (Nota Fiscal Eletrônica);</li>
          <li>NFC-e (Nota Fiscal de Consumidor Eletrônica);</li>
          <li>CT-e (Conhecimento de Transporte Eletrônico);</li>
          <li>MDF-e (Manifesto Eletrônico de Documentos Fiscais).</li>
        </ul>

        <p>
          O serviço poderá ser expandido futuramente para suportar outros
          documentos fiscais eletrônicos.
        </p>
      </LegalSection>

      <LegalSection heading="2. Responsabilidade pelas informações">
        <p>
          O usuário é o único responsável pela veracidade, integridade,
          legalidade e atualização das informações inseridas na plataforma.
        </p>

        <p>
          A CTech não realiza validação jurídica, contábil ou tributária das
          informações fornecidas pelo usuário.
        </p>

        <p>
          Erros decorrentes de informações incorretas poderão resultar em:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>rejeições pela SEFAZ;</li>
          <li>cancelamentos de documentos;</li>
          <li>multas e penalidades fiscais;</li>
          <li>demais consequências legais.</li>
        </ul>

        <p>
          Tais consequências são de exclusiva responsabilidade do usuário.
        </p>
      </LegalSection>

      <LegalSection heading="3. Dados pessoais de terceiros">
        <p>
          Para emissão de documentos fiscais, o usuário poderá inserir dados
          pessoais de terceiros, incluindo clientes, fornecedores,
          transportadores, destinatários e demais participantes da operação.
        </p>

        <p>
          O usuário declara possuir base legal adequada para o tratamento
          desses dados, nos termos da legislação aplicável.
        </p>

        <p>
          Para fins da Lei Geral de Proteção de Dados (LGPD):
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>
            o usuário atua como <strong>Controlador</strong> dos dados;
          </li>

          <li>
            a CTech atua como <strong>Operadora</strong>, tratando os dados
            exclusivamente para prestação do serviço contratado.
          </li>
        </ul>
      </LegalSection>

      <LegalSection heading="4. Certificados digitais">
        <p>
          A emissão de determinados documentos fiscais exige utilização de
          certificado digital válido.
        </p>

        <p>
          Os certificados digitais enviados à plataforma:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>
            são armazenados em infraestrutura privada da Amazon S3;
          </li>

          <li>
            utilizam criptografia gerenciada por AWS KMS;
          </li>

          <li>
            possuem acesso restrito aos componentes necessários para emissão
            fiscal.
          </li>
        </ul>

        <p>
          O usuário permanece integralmente responsável:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>pela legitimidade do certificado;</li>
          <li>por sua validade;</li>
          <li>por sua renovação;</li>
          <li>pela autorização de sua utilização.</li>
        </ul>
      </LegalSection>

      <LegalSection heading="5. Transmissão à SEFAZ">
        <p>
          A transmissão dos documentos fiscais aos órgãos governamentais
          competentes constitui parte essencial do serviço.
        </p>

        <p>
          A autorização dos documentos depende exclusivamente da
          disponibilidade e funcionamento dos sistemas governamentais,
          incluindo:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>Secretarias de Fazenda Estaduais;</li>
          <li>Receita Federal do Brasil;</li>
          <li>demais órgãos competentes.</li>
        </ul>

        <p>
          A CTech não se responsabiliza por indisponibilidades, rejeições,
          lentidões ou falhas decorrentes desses sistemas.
        </p>
      </LegalSection>

      <LegalSection heading="6. Armazenamento de documentos">
        <p>
          XMLs, DANFEs e demais documentos fiscais poderão ser armazenados
          pela plataforma para facilitar sua consulta e recuperação.
        </p>

        <p>
          Os documentos são armazenados utilizando infraestrutura da Amazon
          Web Services (AWS), incluindo armazenamento redundante em Amazon
          S3.
        </p>

        <p>
          Embora sejam adotadas medidas razoáveis de segurança e alta
          disponibilidade, o usuário permanece responsável por manter cópias
          próprias de seus documentos fiscais.
        </p>

        <p>
          A manutenção de backups locais é fortemente recomendada.
        </p>
      </LegalSection>

      <LegalSection heading="7. Retenção">
        <p>
          Salvo solicitação formal em contrário, os documentos fiscais
          poderão ser mantidos por prazo indeterminado.
        </p>

        <p>
          A exclusão de documentos poderá ser recusada quando houver
          obrigação legal de retenção ou necessidade de exercício regular de
          direitos.
        </p>
      </LegalSection>

      <LegalSection heading="8. Disponibilidade do serviço">
        <p>
          O serviço é disponibilizado em regime de melhores esforços.
        </p>

        <p>
          A CTech poderá realizar:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>manutenções programadas;</li>
          <li>atualizações técnicas;</li>
          <li>intervenções emergenciais.</li>
        </ul>

        <p>
          Tais atividades poderão ocasionar indisponibilidades temporárias.
        </p>
      </LegalSection>

      <LegalSection heading="9. Limitação de responsabilidade">
        <p>
          Na máxima extensão permitida pela legislação brasileira, a CTech
          não será responsável por:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>erros de preenchimento realizados pelo usuário;</li>
          <li>multas fiscais;</li>
          <li>penalidades tributárias;</li>
          <li>indisponibilidade de sistemas governamentais;</li>
          <li>falhas de terceiros;</li>
          <li>lucros cessantes ou danos indiretos.</li>
        </ul>

        <p>
          Nada neste documento limita direitos indisponíveis assegurados pela
          legislação brasileira.
        </p>
      </LegalSection>

      <LegalSection heading="10. Exclusão e encerramento">
        <p>
          O usuário poderá solicitar o encerramento da utilização do serviço
          a qualquer momento.
        </p>

        <p>
          Determinados registros poderão ser mantidos após o encerramento,
          inclusive para:
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>cumprimento de obrigações legais;</li>
          <li>auditorias;</li>
          <li>resolução de disputas;</li>
          <li>exercício regular de direitos.</li>
        </ul>
      </LegalSection>

      <LegalSection heading="11. Alterações">
        <p>
          Este documento poderá ser alterado periodicamente.
        </p>

        <p>
          Alterações materiais serão comunicadas previamente e poderão
          exigir novo aceite do usuário.
        </p>
      </LegalSection>

      <LegalSection heading="12. Contato">
        <p>
          A O CARVALHO TECH
        </p>

        <ul className="list-disc pl-5 space-y-2">
          <li>CNPJ: 62.787.449/0001-07</li>
          <li>DPO: Artur Oliveira Carvalho</li>
          <li>dpo@aoctech.app</li>
          <li>legal@aoctech.app</li>
          <li>(86) 9 8803-3430</li>
        </ul>
      </LegalSection>
    </LegalPage>
  )
}
