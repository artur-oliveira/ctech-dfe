'use client'

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideNfse() {
  return (
    <GuidePage
      currentHref="/guide/nfse"
      title="Emitir NFS-e"
      description="A nota de serviço no padrão nacional. Diferente dos outros documentos, ela nasce de uma DPS — a declaração que você envia — e o serviço vem de um catálogo próprio."
      sections={[
        {
          id: 'emit',
          title: 'Uma tela, uma prévia',
          summary:
            'Tomador, serviço, valor e competência de um lado; do outro, a prévia da DPS exatamente como vai ser enviada.',
          image: {
            src: '/guide/nfse-emit.webp',
            alt: 'Emissão de NFS-e com os campos de tomador, serviço, valor e competência ao lado da prévia da DPS',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Tomador">
                  Quem contrata. Pode ser uma pessoa do cadastro, uma nova cadastrada na hora, ou a
                  própria empresa.
                </GuideTerm>
                <GuideTerm term="Serviço">
                  Vem do catálogo de serviços, com o código de tributação nacional e a alíquota de
                  ISS já definidos.
                </GuideTerm>
                <GuideTerm term="Valor e alíquota">
                  Chegam preenchidos do catálogo e podem ser ajustados só nesta emissão, sem alterar
                  o cadastro.
                </GuideTerm>
                <GuideTerm term="Competência">
                  O mês a que o serviço se refere — nem sempre o mês em que você emite.
                </GuideTerm>
              </GuideTerms>
              <p>
                A prévia recalcula o ISS a cada mudança, então o valor que você vê é o que a
                prefeitura vai receber. A emissão só libera com um serviço selecionado.
              </p>
            </>
          ),
        },
        {
          id: 'list',
          title: 'Acompanhando as notas',
          summary:
            'A NFS-e é identificada pelo id da DPS, não por uma chave de 44 dígitos como os outros documentos.',
          image: {
            src: '/guide/nfse-list.webp',
            alt: 'Lista de NFS-e emitidas, com número, série, tomador, valor e status',
          },
          body: (
            <>
              <p>
                O ciclo é o mesmo dos demais: <b>Processando</b> enquanto o pedido está em voo,
                depois <b>Autorizada</b> ou <b>Rejeitada</b>. Autorizada, a nota tem XML e DANFSE
                para download.
              </p>
              <GuideCallout kind="info" title="Padrão nacional e ABRASF">
                O ambiente nacional é o padrão. Municípios que ainda operam no padrão ABRASF 2.04
                exigem endpoint e código do município na configuração fiscal da NFS-e.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'cancel',
          title: 'Cancelamento com motivo',
          summary:
            'Aqui a justificativa não é texto livre: a NFS-e pede um código de motivo definido pelo padrão nacional.',
          image: {
            src: '/guide/nfse-detail.webp',
            alt: 'Detalhe de uma NFS-e autorizada, com dados do serviço, tributação e eventos',
          },
          body: (
            <>
              <GuideBullets>
                <li>Escolha o motivo do cancelamento e descreva o que aconteceu.</li>
                <li>O cancelamento entra como evento e aparece na linha do tempo da nota.</li>
                <li>Quando o caso é nota errada e não serviço inexistente, use <b>substituição</b>.</li>
              </GuideBullets>
            </>
          ),
        },
      ]}
    />
  )
}
