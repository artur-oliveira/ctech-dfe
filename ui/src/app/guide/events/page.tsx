'use client'

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideEventos() {
  return (
    <GuidePage
      currentHref="/guide/events"
      title="Eventos do documento"
      description="Documento autorizado não se edita — se corrige por evento. Cancelamento, carta de correção, encerramento e manifestação são todos eventos, cada um com a sua regra e o seu prazo."
      sections={[
        {
          id: 'timeline',
          title: 'A linha do tempo',
          summary:
            'Todo documento tem uma tabela de eventos no fim do detalhe. A própria emissão é o primeiro deles.',
          image: {
            src: '/guide/nfe-events.webp',
            alt: 'Linha do tempo de eventos de uma NF-e, com a emissão e uma carta de correção registradas',
          },
          body: (
            <>
              <p>
                Cada linha traz o tipo do evento, a sequência, o status e o XML do próprio evento —
                que também é documento fiscal e também deve ser guardado.
              </p>
              <GuideBullets>
                <li><b>Registrado</b> — a SEFAZ aceitou o evento.</li>
                <li><b>Processando</b> — o pedido está em voo; a linha atualiza sozinha.</li>
                <li><b>Rejeitado</b> — a SEFAZ recusou; o motivo abre no próprio status.</li>
              </GuideBullets>
              <p>
                A sequência importa em eventos que podem se repetir: uma segunda carta de correção
                substitui a primeira, e é sempre a de maior sequência que vale.
              </p>
            </>
          ),
        },
        {
          id: 'cancel',
          title: 'Cancelamento',
          summary:
            'Desfaz a operação inteira. Precisa de justificativa e só vale dentro do prazo da SEFAZ.',
          image: {
            src: '/guide/nfe-cancel.webp',
            alt: 'Modal de cancelamento de NF-e pedindo a justificativa do cancelamento',
          },
          body: (
            <>
              <p>
                A justificativa tem mínimo de caracteres exigido pela SEFAZ e vai no XML do evento —
                escreva o motivo real, não um texto de preenchimento.
              </p>
              <GuideCallout kind="warning" title="Prazo e mercadoria em trânsito">
                O prazo de cancelamento da NF-e é contado a partir da autorização e é bem mais curto
                para NFC-e. Fora do prazo, ou com mercadoria já circulando, o caminho é outro:
                emitir uma nota de devolução.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'correction',
          title: 'Carta de Correção',
          summary:
            'Corrige informação que não muda a essência da operação — e só isso.',
          body: (
            <>
              <GuideBullets>
                <li>
                  <b>Serve para</b>: dados cadastrais que não alterem o destinatário, endereço de
                  entrega, códigos que não mudem a tributação, informações complementares.
                </li>
                <li>
                  <b>Não serve para</b>: valores, quantidades, alíquotas, data de emissão ou troca de
                  destinatário. Nesses casos, cancele e emita de novo.
                </li>
              </GuideBullets>
              <p>
                Cada nova carta reenvia o texto completo da correção, não só a diferença — a última
                sequência é a que a SEFAZ considera válida.
              </p>
            </>
          ),
        },
        {
          id: 'manifestation',
          title: 'Manifestação do destinatário',
          summary:
            'Vale para as notas emitidas contra o seu CNPJ: é como você diz à SEFAZ o que fez com aquela operação.',
          image: {
            src: '/guide/nfe-detail.webp',
            alt: 'Detalhe de NF-e com as ações disponíveis para o documento',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Ciência da Operação">
                  Você sabe que a nota existe, mas ainda não confirmou a operação. É o passo que
                  libera o download do XML completo.
                </GuideTerm>
                <GuideTerm term="Confirmação da Operação">
                  A mercadoria ou o serviço foi recebido como descrito.
                </GuideTerm>
                <GuideTerm term="Desconhecimento da Operação">
                  A nota foi emitida contra o seu CNPJ e você não reconhece a operação.
                </GuideTerm>
                <GuideTerm term="Operação não Realizada">
                  Você reconhece a nota, mas a operação não se concretizou. Exige justificativa.
                </GuideTerm>
              </GuideTerms>
              <p>
                Cada tipo só pode ser enviado uma vez por nota: os que já foram registrados somem da
                lista de opções.
              </p>
            </>
          ),
        },
        {
          id: 'closing',
          title: 'Encerramento do MDF-e',
          summary:
            'Exclusivo do manifesto: informa à SEFAZ que a viagem terminou e libera o veículo para o próximo MDF-e.',
          image: {
            src: '/guide/mdfe-detail.webp',
            alt: 'Detalhe de MDF-e autorizado com as ações de encerramento e inclusão',
          },
          body: (
            <p>
              O encerramento pede o município e a data em que a carga foi entregue. Manifesto
              autorizado que nunca é encerrado fica pendente na SEFAZ e trava a emissão do próximo
              para o mesmo veículo — encerre assim que a viagem terminar.
            </p>
          ),
        },
      ]}
    />
  )
}
