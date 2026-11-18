'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideBpe() {
  return (
    <GuidePage
      currentHref="/guide/bpe"
      title="Emitir BP-e"
      description="O bilhete de passagem eletrônico (modelo 63) para transporte rodoviário, aquaviário, ferroviário, metropolitano (TM) e aéreo (TA). Você cadastra as linhas uma vez; na venda só escolhe a linha e informa data, poltrona, valores e pagamento, e o sistema calcula totais, troco e tributos."
      sections={[
        {
          id: 'config',
          title: 'Antes de emitir',
          summary: 'O BP-e tem configuração fiscal própria: ambiente, série e numeração por ambiente.',
          body: (
            <>
              <p>
                Em <b>Configuração Fiscal</b>, aba <b>BP-e</b>, escolha o ambiente (homologação ou produção) e a série
                de cada um. O BP-e não tem distribuição, então não há cursor de documentos recebidos.
              </p>
              <GuideCallout kind="info" title="Plano e cobrança">
                O BP-e é cobrado por um medidor próprio, sob demanda. Plano que não inclui o BP-e recusa a emissão.
              </GuideCallout>
              <GuideCallout kind="warning" title="TM e TA exigem credenciamento">
                O bilhete metropolitano (TM) e o aéreo (TA) usam serviços próprios da SEFAZ e exigem credenciamento
                específico do emitente. O TA só funciona pela SVRS. Sem o credenciamento, a autorização é rejeitada.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'lines',
          title: 'Cadastre as linhas uma vez',
          summary: 'Percurso, voo ou grupo de viagem ficam num cadastro; a venda só escolhe a linha.',
          image: {
            src: '/guide/bpe-lines.webp',
            alt: 'Lista de linhas do BP-e, com nome, tipo de bilhete (comum, TA ou TM) e o percurso, o voo ou o veículo',
          },
          body: (
            <>
              <p>
                Em <b>BP-e &gt; Linhas</b> você cadastra o que não muda de uma venda para outra. Cada linha serve a um tipo de bilhete,
                que não muda depois de salvo:
              </p>
              <GuideTerms>
                <GuideTerm term="Comum">
                  Modal, município de início e de fim (escolhidos na lista do IBGE), código e nome do percurso, tipo de viagem, serviço,
                  acomodação, prefixo e o ICMS (com a alíquota nas situações 00 e 90).
                </GuideTerm>
                <GuideTerm term="TA">
                  O voo: companhia, número, aeroportos de origem e destino, serviço, acomodação e o imposto IBS e CBS (classificação
                  tributária da tabela oficial; doação, estorno, compra governamental, devolução e Simples quando o caso pede).
                </GuideTerm>
                <GuideTerm term="TM">
                  O grupo de viagem: modal, CFOP, UF de início e de fim, placa, prefixo, os trechos (cada um com o ICMS) e os tipos de
                  passageiro (inteira, meia...), de até 15 caracteres.
                </GuideTerm>
              </GuideTerms>
              <GuideBullets>
                <li>Código da linha e prefixo aceitam só letras, números, ponto, barra, hífen e sublinhado; nomes e percurso, texto comum sem &lt; nem &gt;.</li>
                <li>A linha é recusada com as mesmas regras da emissão, então uma linha salva emite.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'emit',
          title: 'A venda: escolher a linha e informar o que varia',
          summary: 'Comum, TA ou TM. Nada de percurso, imposto ou endereço digitado.',
          image: {
            src: '/guide/bpe-emit.webp',
            alt: 'Emissão de BP-e com o seletor do tipo de bilhete, a escolha da linha com data de partida e poltrona e o bloco do embarque',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Comum e TA">
                  Um trecho (ou voo) por linha do cadastro, com a partida, o tipo de trecho e, no comum, a poltrona e a plataforma. O
                  embarque e a validade já vêm sugeridos a partir da primeira partida. Depois, o passageiro, os componentes do valor,
                  o desconto e o pagamento.
                </GuideTerm>
                <GuideTerm term="Passageiro">
                  O caminho principal é o <b>passageiro avulso</b>: nome e documento (tipo em lista) direto na venda. Se preferir,
                  escolha uma pessoa física do cadastro (o CPF vai como documento). O TA exige identificar o passageiro.
                </GuideTerm>
                <GuideTerm term="Comprador e agência">
                  Opcionais e escolhidos no cadastro de pessoas, com o endereço completo; a agência é pessoa jurídica. O comprador é
                  obrigatório no BP-e múltiplo.
                </GuideTerm>
                <GuideTerm term="TM">
                  Não há bilhete individual: a apuração é por grupo de viagem. Cada grupo escolhe a linha do TM; cada item escolhe o
                  trecho da linha e informa os contadores, o total de passageiros, o valor e a quantidade por tipo de passageiro.
                </GuideTerm>
              </GuideTerms>
              <p>
                O servidor calcula o valor total (soma dos componentes), o valor a pagar (total menos desconto), o troco e o imposto da
                linha. O total na barra inferior é só uma estimativa.
              </p>
              <GuideBullets>
                <li>Sem linha cadastrada para o tipo escolhido, a tela avisa e leva ao cadastro; a emissão fica bloqueada.</li>
                <li>Com desconto, informe o tipo e a descrição; o tipo Outros pede também o código do desconto.</li>
                <li>No TA, o tipo de compra pode ser normal (passageiro identificado), BP-e múltiplo (fretamento: sem passageiro e com comprador) ou vinculado a um múltiplo (só o passageiro e a chave do múltiplo; sem valores, imposto nem pagamento).</li>
                <li>
                  <b>Substituição</b>: marque que o bilhete substitui outro e informe a chave e o tipo (remarcação, transferência e,
                  no TA, também erro de tarifa, de tributação, decisão judicial, erro cadastral e outros). O original precisa estar
                  autorizado, repetir origem, destino e comprador e, se o embarque já passou, ter o evento de não embarque. Autorizado
                  o substituto, o original passa a <b>Substituído</b>.
                </li>
                <li>No bilhete comum, o conteúdo do código de embarque (PDF417) é opcional e sai na Divisão IX do DABPE.</li>
                <li>No bilhete impresso, o DABPE tem o formato A4 e o reduzido (bobina de 58 mm). Em contingência sai com duas vias, a segunda marcada &quot;Via do Estabelecimento&quot;.</li>
                <li>Os horários seguem o fuso do seu navegador (no Brasil, -03:00). Texto livre (informações adicionais, nome do passageiro) é normalizado e não aceita &lt; nem &gt;.</li>
                <li>Antes de reservar o número, o bilhete passa pelas regras do MOC; o que a SEFAZ rejeitaria com certeza volta como erro apontando o campo.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'list',
          title: 'Acompanhando os bilhetes',
          summary: 'A lista mostra número, trecho, valor e status; o resultado chega sozinho, sem recarregar.',
          image: {
            src: '/guide/bpe-list.webp',
            alt: 'Lista de BP-e emitidos, com número, trecho, tipo, valor, status e as ações Detalhes, XML, DABPE e Cancelar',
          },
          body: (
            <>
              <p>
                O ciclo é o dos demais documentos: <b>Pendente</b> enquanto a SEFAZ responde, depois <b>Autorizado</b> ou{' '}
                <b>Rejeitado</b>. Autorizado, o bilhete tem XML e DABPE para baixar (o TM não tem DABPE) e pode ser cancelado.
              </p>
              <p>
                O resultado também sai por <b>webhook</b>, com <code>document_type</code> <code>bpe</code>.
              </p>
            </>
          ),
        },
        {
          id: 'detail',
          title: 'Detalhe, eventos e cancelamento',
          summary: 'O detalhe traz a chave de acesso, o protocolo, os eventos e as ações de cada tipo.',
          image: {
            src: '/guide/bpe-detail.webp',
            alt: 'Detalhe de um BP-e autorizado com chave de acesso, dados da viagem, botões de evento e a tabela de eventos',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Cancelamento">
                  Antes do embarque, com justificativa de 15 a 255 caracteres. Vale para os três tipos. O prazo e as
                  condições são do autorizador (por exemplo, bilhete com não embarque registrado não pode ser cancelado).
                </GuideTerm>
                <GuideTerm term="Não embarque">Até 24 horas depois do embarque, com justificativa. Comum e TA.</GuideTerm>
                <GuideTerm term="Alteração de poltrona">Informa a nova poltrona, diferente da original. Só o comum.</GuideTerm>
                <GuideTerm term="Excesso de bagagem">Quantidade e valor cobrado. Só o comum.</GuideTerm>
                <GuideTerm term="Fornecimento não realizado">IBS e CBS do serviço não fornecido, em pagamento antecipado. Só o TA.</GuideTerm>
              </GuideTerms>
              <GuideCallout kind="warning" title="Contingência offline">
                A API aceita a emissão em contingência offline (<code>PUT /bpe-emission</code>): o bilhete é assinado na hora, leva o
                QR Code com assinatura, sai com o aviso &ldquo;EMITIDO EM CONTINGÊNCIA Pendente de autorização&rdquo; e é transmitido
                depois com a mesma chave. A tela de configuração ainda não oferece essa troca.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'payment-link',
          title: 'Vinculação de pagamento',
          summary: 'Liga ao BP-e a transação de pagamento; o meio vem de uma lista.',
          image: {
            src: '/guide/bpe-payment-link.webp',
            alt: 'Janela de vinculação de pagamento do BP-e, com número, identificador da transação, meio, recebedor e PSP',
          },
          body: (
            <>
              <p>
                No detalhe de um BP-e autorizado, <b>Vincular pagamento</b> registra o evento 110300: o número do
                pagamento (de 1 a 990), o identificador da transação (de 2 a 35 caracteres), o meio de pagamento na
                lista, o recebedor e o PSP (só a raiz de 8 dígitos do CNPJ). Cada vinculação ganha o próximo número de
                sequência sozinho; a lista do número é opcional.
              </p>
              <p>
                Para desfazer, <b>Cancelar vinculação</b> (evento 110301) pede a vinculação que o autorizador já aceitou;
                o protocolo dela é lido do registro, não digitado. 
              </p>
            </>
          ),
        },
      ]}
    />
  );
}
