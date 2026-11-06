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
                  O modal. Rodoviário está disponível; aéreo, aquaviário e ferroviário aparecem
                  marcados como em breve.
                </GuideTerm>
                <GuideTerm term="2. Documentos">
                  As chaves de NF-e ou CT-e que viajam na carga, agrupadas por município de descarga.
                </GuideTerm>
                <GuideTerm term="3. Carga">
                  Peso total, valor e produto predominante. A prévia soma o que veio dos documentos.
                </GuideTerm>
                <GuideTerm term="4. Trajeto">
                  UF de início e fim e as UFs de percurso, além dos municípios de carregamento.
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
                <li><b>Incluir DF-e</b>: nota que entrou na carga depois da autorização.</li>
                <li><b>Encerrar</b>: informa que a viagem terminou, com município e data.</li>
                <li><b>Cancelar</b>: a viagem não aconteceu, e o manifesto ainda não foi encerrado.</li>
              </GuideBullets>
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
