'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideNfcom() {
  return (
    <GuidePage
      currentHref="/guide/nfcom"
      title="Emitir NFCom"
      description="A nota fiscal fatura de serviço de comunicação (modelo 62): internet, telefonia e TV por assinatura. Você cadastra assinantes e planos uma vez; na fatura só escolhe e informa quantidades, e o sistema calcula valores, tributos e totais."
      sections={[
        {
          id: 'config',
          title: 'Antes de emitir',
          summary: 'A NFCom tem configuração fiscal própria: ambiente, série e numeração por ambiente.',
          body: (
            <>
              <p>
                Em <b>Configuração Fiscal</b>, aba <b>NFCom</b>, escolha o ambiente (homologação ou produção) e
                a série de cada um. A NFCom não tem distribuição, então não há cursor de notas recebidas.
              </p>
              <GuideCallout kind="info" title="Plano e cobrança">
                A NFCom é cobrada por um medidor próprio. Plano que não inclui a NFCom recusa a emissão com a
                mensagem &ldquo;seu plano não inclui a emissão de NFCOM&rdquo;.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'subscribers',
          title: 'Cadastre o assinante uma vez',
          summary: 'Pessoa, código, tipo, serviço e contrato ficam num cadastro; a fatura só escolhe o assinante.',
          image: {
            src: '/guide/nfcom-subscribers.webp',
            alt: 'Lista de assinantes da NFCom, com nome, código, tipo de assinante e serviço utilizado',
          },
          body: (
            <>
              <p>
                Em <b>NFCom &gt; Assinantes</b>, cada assinante aponta uma <b>pessoa</b> do cadastro de Pessoas: o
                documento, o endereço e a inscrição estadual vêm de lá, então nada disso é digitado de novo a cada
                fatura. A pessoa precisa ter o endereço nacional completo (logradouro, bairro, município, CEP e UF).
              </p>
              <GuideTerms>
                <GuideTerm term="Código do assinante">Identificador do contrato no seu sistema; só letras, números, ponto, barra, hífen e sublinhado.</GuideTerm>
                <GuideTerm term="Tipo e serviço">Seletores do leiaute: comercial, residencial, órgão público...; telefonia, dados, TV, internet...</GuideTerm>
                <GuideTerm term="Contrato e terminal">Opcionais: número, datas de início e fim (o fim não pode ser anterior ao início), terminal principal e município dele.</GuideTerm>
              </GuideTerms>
            </>
          ),
        },
        {
          id: 'plans',
          title: 'Cadastre os planos uma vez',
          summary: 'Classificação, unidade, valor padrão e tributação ficam no plano; a fatura informa só a quantidade.',
          image: {
            src: '/guide/nfcom-plans.webp',
            alt: 'Lista de planos da NFCom, com nome, código, unidade e valor padrão',
          },
          body: (
            <>
              <p>
                Em <b>NFCom &gt; Planos</b> ficam o código, a descrição que sai na fatura, a classificação (cClass, 7
                dígitos), a unidade e os tributos do item: ICMS (com o CFOP de uma lista), PIS e COFINS.
              </p>
              <GuideBullets>
                <li>Com <b>valor padrão</b>, a fatura não pede o valor; sem ele, a fatura informa o valor de cada linha.</li>
                <li>O CFOP é obrigatório, exceto quando o ICMS é &ldquo;sem situação tributária&rdquo; (aí ele não pode ser informado).</li>
                <li>A alíquota do ICMS só vale nas situações 00 e 90.</li>
                <li>O plano é recusado com as mesmas regras de item da emissão, então um plano salvo emite.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'emit',
          title: 'A fatura: escolher e informar quantidades',
          summary: 'Assinante, linhas de plano e fatura. Nada de cliente, endereço ou imposto digitado.',
          image: {
            src: '/guide/nfcom-emit.webp',
            alt: 'Emissão de NFCom com a escolha do assinante, as linhas de plano com quantidade e o bloco da fatura',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Assinante">Escolhido da lista; o cliente, o endereço e a situação na inscrição estadual vêm do cadastro.</GuideTerm>
                <GuideTerm term="Itens">
                  Uma linha por plano, com a quantidade. Desconto e outras despesas são opcionais; o valor unitário só
                  aparece quando o plano não tem valor padrão.
                </GuideTerm>
                <GuideTerm term="Fatura">
                  Competência (lista dos últimos 12 meses) e vencimento já vêm sugeridos; informe o código de barras. Em
                  serviço pré-pago a fatura é dispensada.
                </GuideTerm>
              </GuideTerms>
              <p>
                O servidor calcula o valor de cada item (quantidade x valor - desconto + outras despesas), a base e
                o valor dos tributos e os totais. O total mostrado na barra inferior é só uma estimativa.
              </p>
              <GuideBullets>
                <li>Sem assinante ou plano cadastrado, a tela avisa e leva ao cadastro; a emissão fica bloqueada.</li>
                <li>Antes de reservar o número, a nota passa pelas regras do MOC; o que a SEFAZ rejeitaria com certeza volta como erro apontando o campo.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'list',
          title: 'Acompanhando as notas',
          summary: 'A lista mostra número, cliente, valor e status; o resultado chega sozinho, sem recarregar.',
          image: {
            src: '/guide/nfcom-list.webp',
            alt: 'Lista de NFCom emitidas, com número, cliente, valor, status e as ações Detalhes, XML, DANFE-COM e Cancelar',
          },
          body: (
            <>
              <p>
                O ciclo é o dos demais documentos: <b>Pendente</b> enquanto a SEFAZ responde, depois{' '}
                <b>Autorizada</b> ou <b>Rejeitada</b>. Autorizada, a nota tem XML e DANFE-COM para baixar e pode ser
                cancelada.
              </p>
              <p>
                O resultado também sai por <b>webhook</b>, com <code>document_type</code> <code>nfcom</code>.
              </p>
            </>
          ),
        },
        {
          id: 'detail',
          title: 'Detalhe, eventos e cancelamento',
          summary: 'O detalhe traz a chave de acesso, o protocolo e o histórico de eventos.',
          image: {
            src: '/guide/nfcom-detail.webp',
            alt: 'Detalhe de uma NFCom autorizada com chave de acesso, dados do cliente, protocolo e a tabela de eventos',
          },
          body: (
            <>
              <p>
                O cancelamento (evento 110111) pede uma justificativa de 15 a 255 caracteres. O prazo é do
                autorizador: uma nota autorizada há mais de 120 horas volta com a rejeição 220 e continua
                autorizada.
              </p>
              <GuideCallout kind="warning" title="Contingência offline">
                A API aceita a emissão em contingência offline (<code>PUT /nfcom-emission</code>): a nota é
                assinada na hora, sai com o aviso &ldquo;EMITIDO EM CONTINGÊNCIA Pendente de autorização&rdquo; e é
                transmitida depois com a mesma chave. A tela de configuração ainda não oferece essa troca.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'payment-link',
          title: 'Vinculação de pagamento',
          summary: 'Liga à NFCom a transação de pagamento; o meio vem de uma lista.',
          image: {
            src: '/guide/nfcom-payment-link.webp',
            alt: 'Janela de vinculação de pagamento da NFCom, com número, identificador da transação, meio, recebedor e PSP',
          },
          body: (
            <>
              <p>
                No detalhe de uma NFCom autorizada, <b>Vincular pagamento</b> registra o evento 110300: o número do
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
