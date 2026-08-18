'use client'

import {GuideBullets, GuideCallout, GuidePage, GuideSteps, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function PrimeirosPassos() {
  return (
    <GuidePage
      currentHref="/guide/getting-started"
      title="Primeiros passos"
      description="Antes da primeira nota, três coisas precisam estar no lugar: a empresa cadastrada, o certificado digital enviado e a numeração definida. O onboarding cuida das três."
      sections={[
        {
          id: 'onboarding',
          title: 'Onboarding em seis etapas',
          summary:
            'Aparece no primeiro acesso e pergunta só o que a SEFAZ vai exigir. Dá para sair no meio e voltar depois — o painel guarda o que falta.',
          image: {
            src: '/guide/onboarding.webp',
            alt: 'Etapa final do onboarding, com os cinco tipos de documento marcados como habilitados',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Plano">
                  Free, Pro ou sob demanda. Dá para começar no Free e trocar depois, sem perder nada.
                </GuideTerm>
                <GuideTerm term="Empresa">
                  CNPJ, inscrição estadual, endereço e regime tributário. Vira o emitente de todo documento.
                </GuideTerm>
                <GuideTerm term="Documentos">
                  Quais tipos você vai emitir. Cada um liga a sua própria numeração e configuração.
                </GuideTerm>
                <GuideTerm term="Produtos e serviços">
                  Um mínimo de catálogo para a primeira emissão não travar na hora de escolher o item.
                </GuideTerm>
                <GuideTerm term="Pronto">
                  Resumo do que ficou habilitado, com atalho direto para a primeira NF-e.
                </GuideTerm>
              </GuideTerms>
              <p>
                Nada disso é definitivo: empresa, numeração e catálogo mudam a qualquer momento em
                Configurações e Cadastros.
              </p>
            </>
          ),
        },
        {
          id: 'certificate',
          title: 'Certificado digital A1',
          summary:
            'É o certificado que assina o XML. Sem ele o documento não sai do rascunho — a SEFAZ não aceita nada sem assinatura.',
          image: {
            src: '/guide/certificates.webp',
            alt: 'Tela de certificados com um certificado A1 ativo, exibindo apelido, hash e data de expiração',
          },
          body: (
            <>
              <GuideSteps>
                <li>Envie o arquivo <code className="font-mono text-xs">.pfx</code> e informe a senha.</li>
                <li>O sistema lê titular e validade e guarda o arquivo criptografado.</li>
                <li>A partir daí toda emissão assina com ele, sem pedir a senha de novo.</li>
              </GuideSteps>
              <GuideCallout kind="warning" title="Fique de olho na validade">
                Certificado A1 vale um ano. A tela mostra a data de expiração e sinaliza quando está
                perto do fim — vencido, a emissão para no mesmo dia.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'environment',
          title: 'Homologação antes de produção',
          summary:
            'São dois ambientes independentes na SEFAZ, cada um com a sua numeração. Homologação existe para você errar à vontade.',
          image: {
            src: '/guide/fiscal-config.webp',
            alt: 'Configuração fiscal da NF-e com ambiente ativo, série e número atual separados por produção e homologação',
          },
          body: (
            <>
              <GuideBullets>
                <li>
                  <b>Homologação</b> — documento é autorizado de verdade pela SEFAZ, mas sem valor
                  fiscal. Toda tela mostra a tarja amarela avisando.
                </li>
                <li>
                  <b>Produção</b> — vale para valer. Cancelar tem prazo, e nota autorizada entra na
                  sua apuração.
                </li>
              </GuideBullets>
              <p>
                Série e número atual são por ambiente: trocar de ambiente não embaralha a sua
                numeração de produção. A configuração fica em <b>Configurações → Configuração Fiscal</b>,
                com uma aba por tipo de documento.
              </p>
              <GuideCallout kind="tip" title="Comece com uma nota de teste">
                Emita uma NF-e em homologação para um cliente fictício antes de virar a chave. Você vê
                o fluxo inteiro — autorização, DANFE, cancelamento — sem consequência nenhuma.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'dashboard',
          title: 'O painel',
          summary:
            'Ponto de partida do dia a dia: atalhos de emissão por tipo de documento e o que ainda falta configurar.',
          image: {
            src: '/guide/dashboard.webp',
            alt: 'Painel do CTech DF-e com os atalhos de emissão para NF-e, NFC-e, CT-e e MDF-e',
          },
          body: (
            <p>
              O seletor no topo troca de empresa quando a conta tem mais de uma; tudo abaixo dele —
              documentos, cadastros, configuração — pertence à empresa selecionada. A lista de
              pendências some sozinha à medida que a configuração fica completa.
            </p>
          ),
        },
      ]}
    />
  )
}
