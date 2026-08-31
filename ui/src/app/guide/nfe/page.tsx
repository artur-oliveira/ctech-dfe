'use client'

import Link from 'next/link'
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideNfe() {
  return (
    <GuidePage
      currentHref="/guide/nfe"
      title="Emitir NF-e"
      description="A Nota Fiscal Eletrônica de mercadoria, modelo 55. A emissão tem quatro passos e o sistema salva rascunho sozinho — dá para parar no meio e voltar."
      sections={[
        {
          id: 'list',
          title: 'A lista de notas',
          summary:
            'Cinco abas, cinco recortes da mesma operação. A primeira, Emitidas, é a que você usa todo dia.',
          image: {
            src: '/guide/nfe-list.webp',
            alt: 'Lista de NF-e emitidas com número, destinatário, total, status e data de emissão',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Emitidas">Notas em que a sua empresa é o emitente.</GuideTerm>
                <GuideTerm term="Recebidas">Notas emitidas contra o seu CNPJ como destinatário.</GuideTerm>
                <GuideTerm term="Transportadas">Notas em que você aparece como transportador.</GuideTerm>
                <GuideTerm term="Importação/Distribuição">
                  O que a SEFAZ entregou por NSU e ainda não virou documento seu —{' '}
                  <Link href="/guide/distributions" className="font-medium text-primary-700 underline underline-offset-2">
                    ver distribuições
                  </Link>.
                </GuideTerm>
                <GuideTerm term="Inutilizações">
                  As lacunas de numeração já inutilizadas, e as que a SEFAZ ainda espera que você
                  justifique.
                </GuideTerm>
              </GuideTerms>
              <p>
                Cada linha traz as ações possíveis para aquele status: <b>Detalhes</b> sempre,{' '}
                <b>DANFE</b> quando há autorização, <b>Cancelar</b> enquanto o prazo permite. Uma
                nota em processamento mostra só Detalhes — não há o que baixar ainda.
              </p>
            </>
          ),
        },
        {
          id: 'receiver',
          title: 'Passo 1 — Destinatário',
          summary:
            'Para quem é a nota e qual a natureza da operação. Os dois definem CFOP e finalidade do resto do preenchimento.',
          image: {
            src: '/guide/nfe-emit-receiver.webp',
            alt: 'Primeiro passo da emissão de NF-e, com natureza da operação, atalhos de destinatários recentes e busca',
          },
          body: (
            <>
              <GuideBullets>
                <li>
                  <b>Recentes</b> traz os últimos destinatários — um clique substitui a busca.
                </li>
                <li>
                  <b>Para si mesmo</b> emite contra a própria empresa (remessa, transferência,
                  ajuste), sem escolher destinatário.
                </li>
                <li>
                  A <b>natureza da operação</b> preenche natureza, finalidade e o CFOP dos itens.
                  Qualquer campo que você editar depois vence a operação.
                </li>
              </GuideBullets>
              <p>
                Locais de retirada e entrega são opcionais e só aparecem no XML quando diferentes do
                endereço cadastrado. Endereços usados com frequência podem ser salvos na pessoa.
              </p>
            </>
          ),
        },
        {
          id: 'products',
          title: 'Passo 2 — Produtos',
          summary:
            'Itens vêm do catálogo com NCM, unidade, valor e perfil fiscal já preenchidos. Você ajusta quantidade e desconto na linha.',
          image: {
            src: '/guide/nfe-emit-products.webp',
            alt: 'Segundo passo da emissão, com um produto adicionado à nota e o total parcial',
          },
          body: (
            <>
              <p>
                O CFOP de cada item sai do perfil fiscal do produto combinado com a UF do
                destinatário — operação dentro do estado e interestadual não usam o mesmo código.
              </p>
              <GuideCallout kind="warning" title="Entrada e saída não se misturam">
                Uma NF-e é de entrada (CFOP 1/2/3) ou de saída (5/6/7), nunca das duas. O tipo é
                definido pelo primeiro produto; se um item seguinte divergir, a emissão avisa antes
                de deixar avançar.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'review',
          title: 'Passos 3 e 4 — Pagamento e revisão',
          summary:
            'A condição de pagamento vem do cadastro e já monta as parcelas. A revisão mostra a nota inteira antes do envio.',
          image: {
            src: '/guide/nfe-emit-review.webp',
            alt: 'Passo de revisão da NF-e, com destinatário, produtos, pagamento e total, e o botão Emitir NF-e',
          },
          body: (
            <>
              <p>
                Cada bloco da revisão tem um <b>Editar</b> que volta direto ao passo certo, sem
                perder o resto do preenchimento. Ao emitir, a nota entra na fila: fica{' '}
                <b>Processando</b> até a SEFAZ responder e muda para <b>Autorizada</b> ou{' '}
                <b>Rejeitada</b> sem você precisar atualizar a tela.
              </p>
              <p>
                A soma dos pagamentos tem que fechar com o total da nota para o passo avançar: a
                diferença aparece ao lado do botão <b>Próximo</b>, e <b>Ajustar última parcela</b>{' '}
                absorve o centavo de arredondamento. O mesmo vale para as parcelas da fatura quando
                há pagamento a prazo. É a rejeição mais comum da NF-e, e é conta — o sistema fecha,
                não você.
              </p>
              <GuideCallout kind="tip" title="Rascunho automático">
                A emissão em andamento é salva no seu navegador. Se você fechar a aba, a próxima
                visita à tela de emissão oferece retomar de onde parou — ou descartar.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'detail',
          title: 'Depois de autorizada',
          summary:
            'O detalhe reúne protocolo, partes, itens, pagamentos e a linha do tempo de eventos do documento.',
          image: {
            src: '/guide/nfe-detail.webp',
            alt: 'Detalhe de uma NF-e autorizada, com código e protocolo de autorização, emitente, destinatário e produtos',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>XML</b> — o arquivo assinado e autorizado, o documento fiscal de fato.</li>
                <li><b>DANFE</b> — o PDF de acompanhamento da mercadoria.</li>
                <li><b>Carta de Correção</b> e <b>Cancelar</b> — eventos, cada um com a sua regra.</li>
              </GuideBullets>
              <p>
                Código e protocolo de autorização são a prova de que a SEFAZ aceitou. Guarde o XML:
                é ele, não o DANFE, que vale como documento fiscal.
              </p>
            </>
          ),
        },
        {
          id: 'rejection',
          title: 'Quando a SEFAZ rejeita',
          summary:
            'Rejeição não é erro do sistema: é a SEFAZ recusando o conteúdo da nota, com um código e um motivo.',
          image: {
            src: '/guide/nfe-rejection.webp',
            alt: 'Modal com o motivo da rejeição de uma NF-e, exibindo a mensagem devolvida pela SEFAZ',
          },
          body: (
            <>
              <p>
                O status <b>Rejeitada</b> vira botão: clicar abre o motivo exato devolvido pela
                SEFAZ. Corrija o que a mensagem aponta — cadastro, CFOP, valor — e emita de novo. A
                nota rejeitada não consome numeração autorizada.
              </p>
              <p>
                <b>Tentando novamente</b> é diferente: aí a SEFAZ não respondeu, e a fila reenvia
                sozinha. Não há nada a fazer além de esperar.
              </p>
            </>
          ),
        },
        {
          id: 'mobile',
          title: 'Consultando no celular',
          summary:
            'A lista vira cartão em tela pequena, com as mesmas ações — é o caso de uso mais comum fora do escritório.',
          image: {
            src: '/guide/mobile-nfe-list.webp',
            alt: 'Lista de NF-e em tela de celular, com cada nota em formato de cartão',
          },
          body: (
            <p>
              Número, destinatário, total e status continuam visíveis sem rolagem horizontal, e
              baixar XML ou DANFE funciona igual. Emitir também funciona, embora notas com muitos
              itens sejam mais confortáveis no desktop.
            </p>
          ),
        },
      ]}
    />
  )
}
