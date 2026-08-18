'use client'

import Link from 'next/link'
import {GuideBullets, GuideCallout, GuidePage} from '@/components/guide/GuidePage'

export default function GuideDistribuicoes() {
  return (
    <GuidePage
      currentHref="/guide/distributions"
      title="Documentos recebidos"
      description="Tudo que é emitido contra o seu CNPJ fica disponível na SEFAZ, entregue em ordem por um número sequencial: o NSU. A distribuição é como você busca esse lote."
      sections={[
        {
          id: 'nsu',
          title: 'Como a SEFAZ entrega',
          summary:
            'Você não recebe notificação — você pergunta. A cada consulta o sistema pede o que veio depois do último NSU processado.',
          image: {
            src: '/guide/nfe-distribution.webp',
            alt: 'Aba de importação e distribuição de NF-e, com último NSU consultado e os documentos recebidos',
          },
          body: (
            <>
              <p>
                O cabeçalho mostra o <b>último NSU</b> processado, quando foi a última consulta e a
                próxima estimada. As consultas acontecem sozinhas em intervalo regular; o botão{' '}
                <b>Consultar SEFAZ</b> antecipa quando você está esperando um documento específico.
              </p>
              <GuideBullets>
                <li><b>Resumo NF-e</b> — os dados básicos da nota, sem o XML completo.</li>
                <li><b>Evento</b> — cancelamento, CC-e ou manifestação de um documento seu.</li>
                <li><b>NF-e completa</b> — o XML inteiro, disponível depois da manifestação.</li>
              </GuideBullets>
              <GuideCallout kind="info" title="Limite da SEFAZ">
                A SEFAZ limita quantas consultas você pode fazer por intervalo. Por isso o sistema
                respeita a próxima consulta estimada em vez de perguntar em looping.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'import',
          title: 'Importar por chave ou XML',
          summary:
            'Quando o documento não chegou pela distribuição, ou você já tem o arquivo em mãos.',
          body: (
            <>
              <GuideBullets>
                <li>
                  <b>Importar NF-e</b> — você informa a chave de acesso de 44 dígitos e o sistema
                  busca o documento na SEFAZ.
                </li>
                <li>
                  <b>Importar XML</b> — o fornecedor mandou o arquivo por e-mail e você o sobe
                  direto, sem depender do lote.
                </li>
              </GuideBullets>
              <p>
                Nos dois casos o documento passa a valer como recebido e aparece na aba{' '}
                <b>Recebidas</b>, com as mesmas ações de um documento vindo do lote.
              </p>
            </>
          ),
        },
        {
          id: 'manifest',
          title: 'Manifestar-se sobre o recebido',
          summary:
            'O resumo mostra que a nota existe. Para baixar o XML completo, você precisa se manifestar.',
          image: {
            src: '/guide/empty-state.webp',
            alt: 'Aba sem documentos recebidos, com a mensagem de estado vazio explicando o que aparece ali',
          },
          body: (
            <p>
              Abra o documento e escolha a manifestação — Ciência, Confirmação, Desconhecimento ou
              Operação não realizada. Cada uma tem um significado fiscal próprio;{' '}
              <Link href="/guide/events#manifestation" className="font-medium text-primary-700 underline underline-offset-2">
                o tópico de eventos explica quando usar cada uma
              </Link>.
            </p>
          ),
        },
        {
          id: 'other-documents',
          title: 'CT-e, MDF-e e NFS-e',
          summary:
            'A distribuição não é exclusiva da NF-e: cada tipo tem o seu próprio contador de NSU.',
          image: {
            src: '/guide/cte-distribution.webp',
            alt: 'Distribuição de CT-e, com os conhecimentos de transporte recebidos',
          },
          body: (
            <p>
              CT-e emitidos contra você — como tomador do frete — chegam pela mesma mecânica, e é por
              aí que hoje se acompanha CT-e no sistema. A NFS-e tem distribuição própria no ambiente
              nacional. Como cada tipo mantém o seu NSU, consultar um não afeta os demais.
            </p>
          ),
        },
      ]}
    />
  )
}
