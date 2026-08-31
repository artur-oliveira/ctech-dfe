'use client'

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideNavegacao() {
  return (
    <GuidePage
      currentHref="/guide/navigation"
      title="Como circular pelo sistema"
      description="A navegação segue o documento que você está emitindo. O que serve a todos fica sempre à vista; o que só existe por causa de um tipo de documento mora dentro dele — e a busca global alcança qualquer tela em duas teclas."
      sections={[
        {
          id: 'sidebar',
          title: 'Barra lateral por contexto',
          summary:
            'Quatro blocos fixos e, dentro de cada documento, a emissão e os cadastros que só ele usa.',
          image: {
            src: '/guide/nav-sidebar.webp',
            alt: 'Barra lateral com NFS-e aberta, mostrando Emitir NFS-e, Serviços, Locais de prestação e Documentos referenciados aninhados',
          },
          body: (
            <>
              <p>
                A barra lateral tem quatro blocos: <b>Visão Geral</b> (painel e guia),
                <b> Documentos Fiscais</b>, <b>Cadastros</b> e <b>Configurações</b>. Abrir um tipo
                de documento revela, logo abaixo dele, a emissão e os cadastros exclusivos daquele
                documento. O contexto da tela em que você está já vem aberto; para espiar outro sem
                sair de onde está, use a seta.
              </p>
              <GuideTerms>
                <GuideTerm term="Cadastros">
                  Só o que vários documentos usam: <b>Pessoas</b> e <b>Produtos</b>.
                </GuideTerm>
                <GuideTerm term="NF-e">
                  Naturezas de operação, condições de pagamento, perfis fiscais, declarações de
                  importação e lotes de produção.
                </GuideTerm>
                <GuideTerm term="NFC-e">
                  Terminais de pagamento e bombas de combustível.
                </GuideTerm>
                <GuideTerm term="MDF-e">
                  Veículos, composições veiculares, unidades de carga, vale-pedágio e apólices de
                  seguro.
                </GuideTerm>
                <GuideTerm term="NFS-e">
                  Serviços, locais de prestação e documentos referenciados.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="tip" title="A cor diz onde você está">
                Cada documento tem seu acento — verde na NF-e, azul na NFC-e, violeta no CT-e, âmbar
                no MDF-e e teal na NFS-e. O acento acompanha os cadastros do contexto: a tela de
                <b> Serviços</b> é teal porque pertence à NFS-e.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'search',
          title: 'Busca global',
          summary:
            'Toda página do sistema encontrável pelo nome, pelo contexto ou pelo termo fiscal.',
          image: {
            src: '/guide/nav-search.webp',
            alt: 'Busca global aberta sobre a lista de NF-e, listando páginas com o respectivo contexto',
          },
          body: (
            <>
              <p>
                Clique em <b>Buscar…</b> na barra superior, ou pressione <b>⌘K</b> (<b>Ctrl+K</b> no
                Windows e Linux) — a barra <b>/</b> também abre. A busca tolera erro de digitação e
                procura muito além do rótulo da tela.
              </p>
              <GuideBullets>
                <li>Procure pelo nome da tela: <i>veículos</i>, <i>certificados</i>, <i>assinatura</i>.</li>
                <li>Ou pelo termo fiscal: <i>CFOP</i> leva a naturezas de operação; <i>placa</i>, a veículos; <i>CST</i>, a perfis fiscais.</li>
                <li>Cada resultado mostra o contexto a que pertence, então dá para distinguir cadastros parecidos.</li>
                <li>Setas ↑ ↓ percorrem, <b>Enter</b> navega, <b>Esc</b> fecha.</li>
              </GuideBullets>
              <GuideCallout kind="info" title="A busca vê o que você pode ver">
                Páginas restritas a proprietário e administrador — usuários e assinatura — não
                aparecem para os demais papéis, igual à barra lateral.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'mobile',
          title: 'No celular',
          summary:
            'Barra inferior com os destinos primários; o menu completo continua a um toque.',
          image: {
            src: '/guide/mobile-nav.webp',
            alt: 'Navegação inferior no celular com Painel, MDF-e, Emitir, Buscar e Menu, e a folha de documentos aberta',
          },
          body: (
            <>
              <p>
                No celular a navegação principal fica embaixo, ao alcance do polegar:
                <b> Painel</b>, o documento atual, <b>Emitir</b>, <b>Buscar</b> e <b>Menu</b>.
                Tocar no documento atual abre uma folha com os cinco tipos e os cadastros do
                contexto; <b>Menu</b> abre a barra lateral inteira, com configurações e cadastros
                compartilhados.
              </p>
              <GuideBullets>
                <li><b>Emitir</b> emite o tipo do contexto em que você está — na NFS-e, abre a emissão de NFS-e.</li>
                <li>As barras de ação das telas de emissão ficam acima da navegação, nunca escondidas por ela.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'shortcuts',
          title: 'Atalhos de teclado',
          summary: 'Para quem emite o dia inteiro, menos mouse.',
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="⌘K / Ctrl+K">Abre a busca global de qualquer tela.</GuideTerm>
                <GuideTerm term="/">Também abre a busca, quando o foco não está num campo.</GuideTerm>
                <GuideTerm term="n">Nova emissão do tipo da tela atual.</GuideTerm>
                <GuideTerm term="?">Lista os atalhos disponíveis.</GuideTerm>
                <GuideTerm term="Esc">Fecha diálogo, busca ou painel aberto.</GuideTerm>
              </GuideTerms>
              <p>
                Os atalhos de uma tecla só disparam fora de campos de texto — digitar
                <b> n</b> numa descrição de produto não abre emissão nenhuma.
              </p>
            </>
          ),
        },
      ]}
    />
  )
}
