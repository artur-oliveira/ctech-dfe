'use client';

import Link from 'next/link';
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideCteMdfe() {
  return (
    <GuidePage
      currentHref="/guide/cte-mdfe"
      title="Transporte: CT-e e MDF-e"
      description="O CT-e é o documento do frete: sai das NF-e da carga com três decisões suas. O MDF-e é o manifesto da viagem: amarra veículo, condutor e as notas que estão na carga."
      sections={[
        {
          id: 'mdfe-list',
          title: 'Manifestos da operação',
          summary:
            'A lista mostra o trecho (UF de início e fim), peso e valor da carga, e em que ponto do ciclo cada manifesto está.',
          image: {
            src: '/guide/mdfe-list.webp',
            alt: 'Lista de MDF-e com número, série, trecho entre UFs, peso e valor da carga e status',
          },
          body: (
            <p>
              Um MDF-e passa por <b>Autorizado</b> quando a viagem começa e por <b>Encerrado</b>{' '}
              quando termina. Manifesto autorizado e não encerrado é pendência com a SEFAZ; encerrar
              faz parte da operação, não é opcional.
            </p>
          ),
        },
        {
          id: 'mdfe-emit',
          title: 'Emissão em cinco passos',
          summary:
            'Cada passo responde uma pergunta da viagem: como, o quê, quanto, por onde e com qual veículo.',
          image: {
            src: '/guide/mdfe-emit.webp',
            alt: 'Primeiro passo da emissão de MDF-e, com a escolha do modal de transporte',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="1. Transporte">
                  O modal (rodoviário, aéreo, aquaviário ou ferroviário) e, se for o caso, o{' '}
                  <b>carregamento posterior</b>.
                </GuideTerm>
                <GuideTerm term="2. Documentos">
                  As chaves de NF-e ou CT-e que viajam na carga, agrupadas por município de descarga.
                </GuideTerm>
                <GuideTerm term="3. Carga">
                  Peso total, valor e produto predominante. A prévia soma o que veio dos documentos.
                </GuideTerm>
                <GuideTerm term="4. Trajeto">
                  UF de início e fim e as UFs de percurso, os municípios de carregamento, o{' '}
                  <b>tipo de emitente</b> e quem mais pode baixar o XML.
                </GuideTerm>
                <GuideTerm term="5. Veículo">
                  Tração, reboques, condutor e RNTRC; vindos do cadastro de veículos e composições.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="tip" title="Composição veicular economiza o passo 5">
                Se o mesmo cavalo e carreta rodam sempre juntos, cadastre a composição uma vez em{' '}
                <b>Cadastros → Composições veiculares</b>. Na emissão, ela preenche veículo, reboques
                e condutor de uma vez.
              </GuideCallout>
              <GuideCallout kind="info" title="Tipo de emitente">
                Três opções, com os nomes do leiaute: <b>1. Prestador de serviço de transporte</b> (manifesta CT-e),{' '}
                <b>2. Transportador de carga própria</b> (manifesta NF-e; é o padrão aqui, porque este formulário
                leva NF-e) e <b>3. Prestador que emitirá CT-e globalizado</b> (manifesta NF-e e só vale para viagem
                dentro de um estado). A opção que não vale para a viagem fica desabilitada com o motivo escrito
                nela. Com o tipo 1 ou 3 no rodoviário e o RNTRC na configuração, a SEFAZ exige ao menos um CIOT, e o
                botão Próximo espera por ele.
              </GuideCallout>
              <GuideCallout kind="info" title="Autorizados a baixar o XML">
                Até 10 pessoas do cadastro (contador, embarcador) que podem baixar o XML no portal da SEFAZ. A
                busca é a do cadastro de pessoas; o CPF ou CNPJ nunca é digitado. Repetido, documento com dígito
                inválido ou pessoa no exterior são recusados na hora, antes de chegar à SEFAZ.
              </GuideCallout>
              <GuideCallout kind="info" title="CIOT no passo do veículo">
                Em viagem rodoviária, o bloco <b>CIOT</b> aceita um ou mais códigos de <b>12 dígitos</b>,
                cada um com o <b>responsável pela geração</b> (a própria organização ou uma pessoa do
                cadastro; o CPF ou CNPJ vai no manifesto). O MDF-e de transportadora e o de TAC exigem
                CIOT (regra F113 do MOC, NT 2026.001, em produção em 23/11/2026). O sistema não declara o
                CIOT na ANTT por você: informe aqui o que o seu sistema de frete ou a sua administradora
                de pagamento já gerou.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'mdfe-later-loading',
          title: 'Carregamento posterior',
          summary:
            'O manifesto sai antes das notas: a viagem começa e as NF-e entram depois, pela inclusão de DF-e.',
          image: {
            src: '/guide/mdfe-emit-later-loading.webp',
            alt: 'Emissão de MDF-e com carregamento posterior, pedindo o município único, o valor e o peso da carga',
          },
          body: (
            <>
              <p>
                Ligue <b>Carregamento posterior</b> no primeiro passo. O passo Documentos some, e a carga pede só o
                município onde o veículo carrega e descarrega, mais o valor e o peso (vazios, saem como zero). O
                padrão vem de <b>Configuração Fiscal → MDF-e</b>, em &quot;Inclui carga depois de emitir&quot;.
              </p>
              <GuideBullets>
                <li>Só no modal <b>rodoviário</b>; os outros ficam desabilitados.</li>
                <li>Só com o tipo de emitente <b>2. Transportador de carga própria</b>.</li>
                <li>Um único município, que é o de carregamento e o de descarregamento, dentro de um estado.</li>
                <li>Nenhuma nota na emissão: as NF-e entram depois, em <b>Incluir NF-e</b> no detalhe.</li>
              </GuideBullets>
              <GuideCallout kind="warning" title="Prazo de 168 horas">
                Manifesto com carregamento posterior que passa 168 horas sem inclusão de NF-e trava a emissão de
                novos MDF-e do emitente na SEFAZ. O detalhe do manifesto avisa enquanto ele está sem notas.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'mdfe-detail',
          title: 'Durante e depois da viagem',
          summary:
            'O manifesto autorizado ainda aceita mudanças (condutor novo, nota que entrou depois) e precisa ser encerrado no destino.',
          image: {
            src: '/guide/mdfe-detail.webp',
            alt: 'Detalhe de um MDF-e autorizado, com documentos vinculados, trajeto, veículo e condutor',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>Incluir condutor</b>: troca de motorista no meio do trajeto.</li>
                <li>
                  <b>Incluir NF-e</b>: nota que entrou na carga depois da autorização, escolhida da lista de NF-e
                  autorizadas, com os municípios de carregamento e descarregamento.
                </li>
                <li><b>Encerrar</b>: informa que a viagem terminou, com município e data.</li>
                <li><b>Cancelar</b>: a viagem não aconteceu, e o manifesto ainda não foi encerrado.</li>
              </GuideBullets>
              <p>
                <b>Consultar na SEFAZ</b> pergunta ao autorizador a situação do manifesto (autorizado, cancelado,
                encerrado ou não encontrado), o protocolo e os eventos registrados lá. A consulta só lê: quando a
                SEFAZ e o status mostrado aqui discordam, aparece um aviso, e nada é corrigido sozinho.
              </p>
              <p>
                Todos são eventos e ficam registrados na linha do tempo do documento;{' '}
                <Link href="/guide/events" className="font-medium text-primary-700 underline underline-offset-2">
                  ver eventos
                </Link>.
              </p>
            </>
          ),
        },
        {
          id: 'mdfe-unclosed',
          title: 'MDF-e não encerrados',
          summary:
            'O que a SEFAZ ainda tem como aberto para o emitente, emitido aqui ou em outro sistema, com o atalho para encerrar.',
          image: {
            src: '/guide/mdfe-unclosed.webp',
            alt: 'Lista de MDF-e não encerrados na SEFAZ com número, chave, protocolo e o botão Encerrar',
          },
          body: (
            <>
              <p>
                Em <b>MDF-e → Não encerrados</b> a lista vem direto da SEFAZ, com número, chave e protocolo de cada
                manifesto aberto. <b>Encerrar</b> abre o mesmo encerramento do detalhe: escolha o município onde a
                viagem terminou. <b>Consultar de novo</b> refaz a pergunta; a resposta vale por um minuto.
              </p>
              <p>
                Lista vazia quer dizer que nada está pendente de encerramento. Se a SEFAZ recusar a consulta, a
                mensagem dela aparece no lugar da lista.
              </p>
            </>
          ),
        },
        {
          id: 'cte-emit',
          title: 'Emitir CT-e a partir das notas',
          summary:
            'Uma tela só, com três decisões: quais notas, quem paga o frete e quanto custa. Remetente, destinatário, carga e trajeto saem das notas.',
          image: {
            src: '/guide/cte-emit.webp',
            alt: 'Emissão de CT-e com duas notas escolhidas, a faixa de resumo e o remetente sugerido como tomador por frete CIF',
          },
          body: (
            <>
              <p>
                Em <b>CT-e → Emitir CT-e</b>, escolha as NF-e autorizadas da carga no seletor; ele busca
                por número, nome ou chave entre as notas emitidas e recebidas mais recentes. Abaixo da
                lista aparece o resumo lido das notas, por exemplo <b>2 notas · 338,5 kg · R$ 3.769,60 · SP
                para MG</b>. Cada mudança de notas lê tudo de novo.
              </p>
              <GuideTerms>
                <GuideTerm term="1. Notas">
                  De 1 a 50 NF-e, sem repetir. Nota não autorizada ou fora do cadastro aparece com um aviso
                  dentro do bloco e o que fazer.
                </GuideTerm>
                <GuideTerm term="2. Quem paga o frete">
                  Remetente, Destinatário, Expedidor, Recebedor ou Outro, cada um com o nome de quem seria.
                </GuideTerm>
                <GuideTerm term="3. Valor do frete">
                  O total da prestação e, se quiser, componentes como pedágio e GRIS; o restante vira o
                  componente Frete valor. Logo abaixo, o CFOP da prestação: a lista já vem filtrada para
                  a mesma UF (5) ou entre UFs (6), conforme a origem e o destino; nenhum CFOP é presumido.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="info" title="Imposto: nenhuma alíquota é presumida">
                Em <b>Imposto</b> fica o CST do ICMS. Empresa do Simples Nacional ou MEI (CRT 1, 2 ou 4) já vem com
                Simples Nacional. Nos demais casos escolha o CST e informe a alíquota; o ICMS é calculado
                na hora sobre o valor do frete, com a redução da base quando o CST tem uma (20 e 90). CST
                40, 41 e 51 dispensam alíquota.
              </GuideCallout>
              <p>
                A barra fixa no rodapé mostra o total do frete e diz o que ainda falta. <b>Emitir CT-e</b>{' '}
                só habilita com notas, tomador, valor, CFOP, ICMS, remetente, destinatário e peso resolvidos, e
                pede confirmação antes de enviar à SEFAZ.
              </p>
              <GuideCallout kind="tip" title="RNTRC antes do primeiro CT-e">
                O CT-e rodoviário leva o RNTRC da transportadora, cadastrado uma vez em{' '}
                <b>Configuração Fiscal → MDF-e</b>. Sem ele a emissão é recusada, e a mensagem de erro traz
                o botão <b>Cadastrar RNTRC</b>, que abre essa aba.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'cte-taker',
          title: 'Quem paga o frete',
          summary:
            'A sugestão vem da modalidade do frete das notas, com o motivo escrito na opção.',
          image: {
            src: '/guide/cte-emit-fob.webp',
            alt: 'Escolha do tomador do CT-e com o destinatário sugerido por frete FOB na nota',
          },
          body: (
            <>
              <GuideBullets>
                <li>Todas as notas com frete <b>CIF</b>: o remetente vem marcado, com &quot;Frete CIF na nota&quot;.</li>
                <li>Todas com frete <b>FOB</b>: o destinatário vem marcado, com &quot;Frete FOB na nota&quot;.</li>
                <li>Mistura, terceiros ou sem frete: nada vem marcado e um aviso pede a escolha.</li>
              </GuideBullets>
              <p>
                Expedidor, Recebedor e Outro abrem a busca no cadastro de pessoas; o CNPJ nunca é
                digitado aqui. Quem ainda não está no cadastro pode ser criado pela própria busca.
              </p>
            </>
          ),
        },
        {
          id: 'cte-transport',
          title: 'Dados do transporte',
          summary:
            'Remetente, destinatário, origem, destino, peso, valor da carga e produto predominante, recolhidos e derivados das notas.',
          image: {
            src: '/guide/cte-emit-mixed.webp',
            alt: 'Dados do transporte do CT-e abertos, com o aviso de notas com remetentes diferentes',
          },
          body: (
            <>
              <p>
                Cada campo mostra o valor lido das notas. <b>Editar</b> troca o valor só neste CT-e; o
                campo ganha o marcador <b>editado</b>, mostra o derivado logo abaixo e oferece{' '}
                <b>Voltar ao derivado</b>. Remetente e destinatário vêm do cadastro de pessoas; origem e
                destino, da lista de municípios.
              </p>
              <GuideCallout kind="tip" title="Notas de remetentes diferentes">
                Um CT-e tem um remetente e um destinatário. Se as notas divergem, o bloco abre sozinho
                com o aviso: emita um CT-e por remetente ou escolha o remetente em Dados do transporte.
                O mesmo vale para nota sem peso: informe o peso da carga aqui.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'cte-ibscbs',
          title: 'IBS e CBS no CT-e',
          summary:
            'Bloco recolhido na emissão, só para empresas do regime normal (CRT 3). Nada é preenchido por você sem querer: CST, classificação, base e alíquotas são escolha sua.',
          image: {
            src: '/guide/cte-emit-ibscbs.webp',
            alt: 'Emissão de CT-e com o bloco IBS e CBS aberto, mostrando CST, classificação, base, alíquotas e a prévia dos valores',
          },
          body: (
            <>
              <p>
                Abra <b>IBS e CBS (opcional)</b>, marque <b>Informar IBS e CBS neste CT-e</b> e escolha o{' '}
                <b>CST</b> e a <b>classificação</b> nas listas. Informe a base de cálculo e as alíquotas de
                IBS UF, IBS município e CBS (em %). A <b>doação</b> é um interruptor à parte.
              </p>
              <p>
                A prévia mostra os valores como o servidor os calcula: base vezes alíquota, duas casas,
                arredondamento bancário (meio centavo vai para o par). O <b>valor total do documento</b>{' '}
                soma IBS e CBS ao frete só a partir de 2027; até 2026 é o próprio frete.
              </p>
              <GuideCallout kind="tip" title="Simples Nacional e MEI">
                Empresas do Simples Nacional e MEI não veem este bloco: o CT-e com IBS/CBS ainda não é
                suportado nesses regimes e a emissão seria recusada.
              </GuideCallout>
              <p>
                Logo abaixo fica <b>Autorizados a baixar o XML</b>: contador, embarcador ou quem mais
                precisa do arquivo. Escolha as pessoas do cadastro (até 10, sem repetir). Quem já é
                parte do CT-e não entra na lista.
              </p>
            </>
          ),
        },
        {
          id: 'cte-detail',
          title: 'Detalhe do CT-e',
          summary:
            'Clique no número do CT-e na lista para ver status, chave, partes, frete, CFOP, ICMS, trajeto, notas e eventos.',
          image: {
            src: '/guide/cte-detail.webp',
            alt: 'Detalhe de um CT-e autorizado, com ações, situação na SEFAZ, frete, partes e notas',
          },
          body: (
            <p>
              O topo traz o status e a chave de acesso (toque para copiar). O DACTE ainda não está
              disponível, por isso não há botão de PDF. O XML é baixado por evento, na linha de cada
              um.
            </p>
          ),
        },
        {
          id: 'contingency',
          title: 'CT-e e MDF-e em contingência',
          summary:
            'Contingência offline para os dois; o CT-e também aceita SVC-RS ou SVC-SP, conforme a UF.',
          image: {
            src: '/guide/cte-contingency.webp',
            alt: 'Detalhe de um CT-e pendente em contingência, com o aviso de prazo até a data limite',
          },
          body: (
            <>
              <p>
                Escolha a forma de emissão em <b>Configuração Fiscal</b>, na aba do documento. No modo offline o
                documento é assinado na hora e fica com o selo <b>Contingência pendente</b>; o detalhe mostra{' '}
                <b>Emitido em contingência, pendente de autorização; prazo até</b> a data limite (168 horas). Os
                estados finais são Autorizado após contingência, Rejeitado após contingência e Prazo da contingência
                vencido. O DAMDFE pendente pode ser baixado; o XML só depois da autorização.
              </p>
              <p>
                Se a UF da empresa não tem SVC designada, a emissão do CT-e avisa e leva à configuração.
              </p>
            </>
          ),
        },
        {
          id: 'cte-actions',
          title: 'Cancelar, corrigir e contestar',
          summary:
            'Os botões aparecem só quando valem: CT-e autorizado e a sua empresa no papel certo (emitente ou tomador).',
          image: {
            src: '/guide/cte-cancel.webp',
            alt: 'Janela de cancelamento do CT-e com o campo de justificativa e o contador de caracteres',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Cancelar (emitente)">
                  Justificativa de 15 a 255 caracteres, com contador. É irreversível; a SEFAZ confere
                  prazo, MDF-e e CT-e de substituição ligados.
                </GuideTerm>
                <GuideTerm term="Carta de correção (emitente)">
                  Corrige dados sem cancelar. Cada correção escolhe o grupo e o campo nas listas e
                  informa o novo valor (até 500 caracteres); grupos que se repetem pedem o número do
                  item. A sequência é opcional: vazia, vale a próxima livre. A SEFAZ recusa o que não
                  pode ser corrigido.
                </GuideTerm>
                <GuideTerm term="Prestação em desacordo (tomador)">
                  Informa que o serviço não foi prestado como consta. Observações de 15 a 255
                  caracteres, opcionais; em até 45 dias da autorização.
                </GuideTerm>
                <GuideTerm term="Cancelar desacordo (tomador)">
                  Escolha na lista o desacordo que a SEFAZ já autorizou e que você quer desfazer.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="tip" title="Quando você não é o tomador">
                Se quem paga o frete é o cliente, a tela mostra <b>Apenas o tomador</b> no lugar da
                prestação em desacordo. Se a sua empresa não é a emitente, cancelar e corrigir também
                somem. Quando a SEFAZ ou a api recusam, o motivo aparece em português dentro da janela.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'cte-correction',
          title: 'Carta de correção',
          summary: 'Grupo e campo vêm de listas; o novo valor é o único texto livre.',
          image: {
            src: '/guide/cte-correction.webp',
            alt: 'Janela da carta de correção do CT-e, com grupo, campo e novo valor',
          },
          body: (
            <p>
              Use <b>Adicionar correção</b> para corrigir mais de um campo na mesma carta (até 20). O
              grupo escolhido define os campos disponíveis.
            </p>
          ),
        },
        {
          id: 'cte-not-taker',
          title: 'CT-e de outro tomador',
          summary: 'Prestação em desacordo é do tomador; a tela diz isso em vez de esconder o motivo.',
          image: {
            src: '/guide/cte-detail-not-taker.webp',
            alt: 'Detalhe de CT-e em que a empresa não é a tomadora, com o aviso Apenas o tomador',
          },
          body: (
            <p>
              Nesse CT-e a sua empresa continua podendo cancelar e corrigir, por ser a emitente. O
              cliente que paga o frete é quem registra o desacordo.
            </p>
          ),
        },
        {
          id: 'cte-events',
          title: 'Eventos do CT-e',
          summary:
            'Emissão, cancelamento, cartas de correção e desacordos, em ordem, com status, sequência e o XML assinado de cada um.',
          image: {
            src: '/guide/cte-events.webp',
            alt: 'Tabela de eventos de um CT-e com emissão, carta de correção e prestação em desacordo',
          },
          body: (
            <p>
              A lista é paginada. Um evento novo entra como pendente e muda sozinho quando a SEFAZ
              responde; <b>XML</b> baixa o arquivo assinado do evento.
            </p>
          ),
        },
        {
          id: 'cte-sefaz',
          title: 'Consultar na SEFAZ',
          summary:
            'Pergunta direto ao autorizador a situação do CT-e, com protocolo e eventos registrados lá. Só lê: não altera o registro daqui.',
          image: {
            src: '/guide/cte-sefaz-divergence.webp',
            alt: 'Consulta na SEFAZ mostrando CT-e cancelado na SEFAZ e autorizado no registro local, com o aviso de divergência',
          },
          body: (
            <p>
              A resposta é <b>Autorizado</b>, <b>Cancelado</b> ou <b>Não encontrado</b>. Quando ela
              discorda do status daqui, aparece um aviso amarelo; nada é corrigido automaticamente, então
              confira os eventos antes de agir.
            </p>
          ),
        },
        {
          id: 'cte-list',
          title: 'CT-e emitidos',
          summary:
            'A aba Emitidos lista número e série, remetente para destinatário, tomador, valor do frete e o status na SEFAZ.',
          image: {
            src: '/guide/cte-list.webp',
            alt: 'Lista de CT-e emitidos com número, remetente para destinatário, tomador, frete e status',
          },
          body: (
            <p>
              Depois de enviado, o CT-e entra como <b>Pendente</b> e muda sozinho para <b>Autorizado</b>{' '}
              ou <b>Rejeitado</b> quando a SEFAZ responde; não é preciso recarregar a página.
            </p>
          ),
        },
        {
          id: 'cte',
          title: 'CT-e recebidos',
          summary:
            'Os CT-e emitidos contra o seu CNPJ chegam pela distribuição da SEFAZ.',
          image: {
            src: '/guide/cte-distribution.webp',
            alt: 'Tela de CT-e na aba de importação e distribuição, com a consulta à SEFAZ',
          },
          body: (
            <p>
              Ficam nas abas <b>Recebidos</b> e <b>Importação/Distribuição</b>, com XML disponível. Em{' '}
              <b>Importação/Distribuição</b>, <b>Consultar SEFAZ</b> busca os documentos novos.
            </p>
          ),
        },
      ]}
    />
  );
}
