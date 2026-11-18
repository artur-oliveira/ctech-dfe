'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideBpe() {
  return (
    <GuidePage
      currentHref="/guide/bpe"
      title="Emitir BP-e"
      description="O bilhete de passagem eletrônico (modelo 63) para transporte rodoviário, aquaviário, ferroviário, metropolitano (TM) e aéreo (TA). Você informa a viagem, os valores e o pagamento; o sistema calcula totais, troco e tributos."
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
          id: 'emit',
          title: 'Escolha o tipo e preencha',
          summary: 'Comum, TA ou TM: cada tipo tem seu formulário. Tudo que é escolha fechada é um seletor.',
          image: {
            src: '/guide/bpe-emit.webp',
            alt: 'Emissão de BP-e com o seletor do tipo de bilhete e o bloco da viagem: modal, presença, origem, destino, embarque e validade',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Comum">
                  Modal (rodoviário, aquaviário ou ferroviário), viagem com origem e destino, passageiro, trechos, componentes do valor,
                  pagamento e ICMS. Em viagem em conexão, cada trecho é uma linha e o trecho de conexão pede a data da conexão.
                </GuideTerm>
                <GuideTerm term="TA">
                  Transporte aéreo: cada trecho é um voo (companhia, número, aeroportos de origem e destino), sem poltrona nem linha. O
                  imposto é IBS e CBS no lugar do ICMS: a classificação tributária (tabela oficial, define o CST) e, quando o caso pede, doação, estorno de crédito, compra governamental e devolução. As alíquotas fixadas em lei o servidor aplica.
                </GuideTerm>
                <GuideTerm term="TM">
                  Transporte metropolitano: não há bilhete individual. É uma apuração por grupo de viagem (UF, placa, prefixo) com itens
                  por trecho e componentes de tarifa; o total sai da soma dos itens.
                </GuideTerm>
              </GuideTerms>
              <p>
                O servidor calcula o valor total (soma dos componentes), o valor a pagar (total menos desconto), o troco (o que os
                pagamentos passam do valor a pagar) e o ICMS. O total na barra inferior é só uma estimativa.
              </p>
              <GuideBullets>
                <li>Com desconto, informe o tipo e a descrição; o tipo Outros pede também o código do desconto.</li>
                <li>Identifique o passageiro em viagem interestadual e sempre que houver desconto.</li>
                <li>O comprador é opcional (agência ou quem pagou): marque a opção, escolha CPF, CNPJ ou identificador estrangeiro e informe o endereço. Só um documento vale.</li>
                <li>Os municípios saem de uma lista do IBGE: escolha pelo nome e a UF é preenchida junto.</li>
                <li>No bilhete impresso, o DABPE tem o formato A4 e o reduzido (bobina de 58 mm). Em contingência sai com duas vias, a segunda marcada &quot;Via do Estabelecimento&quot;.</li>
                <li>Os horários seguem o fuso do seu navegador (no Brasil, -03:00).</li>
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
