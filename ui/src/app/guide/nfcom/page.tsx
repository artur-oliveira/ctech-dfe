'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideNfcom() {
  return (
    <GuidePage
      currentHref="/guide/nfcom"
      title="Emitir NFCom"
      description="A nota fiscal fatura de serviço de comunicação (modelo 62): internet, telefonia e TV por assinatura. Você informa cliente, assinante, itens e fatura; o sistema calcula valores, tributos e totais."
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
          id: 'emit',
          title: 'Uma tela, quatro blocos',
          summary: 'Cliente, assinante, itens e fatura. Tudo que é escolha fechada é um seletor.',
          image: {
            src: '/guide/nfcom-emit.webp',
            alt: 'Emissão de NFCom com o bloco do cliente: nome, documento, situação na inscrição estadual e endereço',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Cliente">
                  O destinatário da fatura: CNPJ, CPF ou outro identificador, a situação na inscrição estadual e o
                  endereço. Cliente contribuinte exige a IE; isento leva ISENTO; não contribuinte não tem IE.
                </GuideTerm>
                <GuideTerm term="Assinante">
                  Código do assinante, tipo (comercial, residencial, órgão público...) e o serviço utilizado. O
                  contrato e o terminal principal são opcionais.
                </GuideTerm>
                <GuideTerm term="Itens">
                  Código, descrição, classificação (7 dígitos), unidade, quantidade e valor. O ICMS é uma escolha
                  entre as situações do leiaute; PIS e COFINS são opcionais.
                </GuideTerm>
                <GuideTerm term="Fatura">
                  Competência, vencimento e código de barras. Em serviço pré-pago a fatura é dispensada.
                </GuideTerm>
              </GuideTerms>
              <p>
                O servidor calcula o valor de cada item (quantidade x valor - desconto + outras despesas), a base e
                o valor dos tributos e os totais. O total mostrado na barra inferior é só uma estimativa.
              </p>
              <GuideBullets>
                <li>O CFOP é obrigatório, exceto quando o ICMS é &ldquo;sem situação tributária&rdquo; (aí ele não pode ser informado).</li>
                <li>A alíquota do ICMS só vale nas situações 00 e 90.</li>
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
      ]}
    />
  );
}
