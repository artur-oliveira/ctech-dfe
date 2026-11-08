'use client';

import Link from 'next/link';
import {GuideBullets, GuideCallout, GuidePage} from '@/components/guide/GuidePage';

export default function GuideNfce() {
  return (
    <GuidePage
      currentHref="/guide/nfce"
      title="Emitir NFC-e"
      description="A nota do balcão, modelo 65. Venda a consumidor final, emissão em uma tela só e sem passo de destinatário; o CPF é opcional."
      sections={[
        {
          id: 'emit',
          title: 'A tela de venda',
          summary:
            'Feita para ser operada com leitor de código de barras: escaneia, confere o total, emite.',
          image: {
            src: '/guide/nfce-emit.webp',
            alt: 'Tela de emissão de NFC-e no formato PDV, com busca de produto, lista de itens e total da venda',
          },
          body: (
            <>
              <GuideBullets>
                <li>O campo de busca aceita código de barras (GTIN), código interno ou descrição.</li>
                <li><b>Enter</b> adiciona o item destacado; dá para operar sem tirar a mão do teclado.</li>
                <li>O CPF do consumidor é opcional; informe só quando o cliente pedir na nota.</li>
              </GuideBullets>
              <p>
                O total é recalculado a cada item, e a emissão só libera com pelo menos um produto na
                venda. O restante (CFOP, tributação, numeração) sai do cadastro do produto e da
                configuração fiscal da NFC-e.
              </p>
            </>
          ),
        },
        {
          id: 'csc',
          title: 'CSC: o código que valida o QR Code',
          summary:
            'A NFC-e carrega um QR Code que o consumidor consulta na SEFAZ. Quem assina esse código é o CSC, e ele é por ambiente.',
          image: {
            src: '/guide/nfce-list.webp',
            alt: 'Lista de NFC-e emitidas, com número, total, status e data',
          },
          body: (
            <>
              <p>
                O CSC (Código de Segurança do Contribuinte) e o seu identificador são fornecidos pela
                SEFAZ do seu estado e cadastrados em{' '}
                <b>Configurações → Configuração Fiscal → NFC-e</b>. Homologação e produção têm CSCs
                diferentes: o de teste não autoriza nota valendo.
              </p>
              <GuideCallout kind="warning" title="Sem CSC, sem NFC-e">
                Diferente da NF-e, aqui o certificado sozinho não basta. Se o CSC do ambiente ativo
                estiver em branco, a emissão é rejeitada antes mesmo de chegar à SEFAZ.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'qr-version',
          title: 'Versão do QR Code',
          summary:
            'A versão 3 do QR Code dispensa o CSC, mas a SEFAZ da sua UF precisa aceitá-la.',
          image: {
            src: '/guide/fiscal-config-nfce-qr.webp',
            alt: 'Configuração fiscal da NFC-e com o seletor de versão do QR Code e os campos de CSC',
          },
          body: (
            <>
              <p>
                Em <b>Configuração Fiscal → NFC-e</b> escolha a versão 2 (padrão, usa CSC) ou a 3 (sem CSC). Com a
                versão 3 os campos de CSC ficam opcionais. A versão 3 é obrigatória para produtor rural pessoa física,
                exceto no Paraná.
              </p>
              <GuideCallout kind="warning" title="Emitente com CPF e QR Code versão 2">
                A emissão é recusada (rejeição 444) quando o emitente é pessoa física e o QR Code está na versão 2. A
                mensagem traz o botão que abre esta configuração: troque para a versão 3 e emita de novo. Se a SEFAZ
                recusar a versão 3 (rejeição 407), a UF ainda não a aceita.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'offline',
          title: 'NFC-e emitida em contingência offline',
          summary:
            'Com a forma de emissão em Contingência offline, a venda sai na hora e a transmissão fica pendente.',
          image: {
            src: '/guide/nfce-contingency.webp',
            alt: 'Detalhe de uma NFC-e pendente em contingência, com o aviso de prazo e o botão do DANFE',
          },
          body: (
            <>
              <p>
                A lista e o detalhe mostram o selo da contingência e o aviso{' '}
                <b>Emitida em contingência, pendente de autorização; prazo até</b> a data limite para transmitir. O
                DANFC-e, com as duas vias, pode ser baixado mesmo pendente; o XML só fica disponível depois da
                autorização.
              </p>
              <GuideBullets>
                <li><b>Autorizada após contingência</b>: a SEFAZ aceitou; a nota é válida.</li>
                <li><b>Rejeitada após contingência</b>: a nota não tem validade; emita outra corrigindo o motivo.</li>
                <li><b>Prazo da contingência vencido</b>: confira a situação na SEFAZ antes de emitir outro documento.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'after',
          title: 'Cancelar ou substituir',
          summary:
            'Duas saídas diferentes para dois problemas diferentes; e o prazo da NFC-e é curto.',
          image: {
            src: '/guide/nfce-detail.webp',
            alt: 'Detalhe de uma NFC-e autorizada, com protocolo, itens e pagamentos',
          },
          body: (
            <>
              <p>
                O botão <b>DANFE</b> baixa o DANFC-e. O seletor ao lado escolhe o formato; sem escolha, sai o
                formato do próprio documento:
              </p>
              <GuideBullets>
                <li><b>DANFC-e Completo</b>: com a divisão de itens, o padrão do balcão.</li>
                <li><b>DANFC-e Resumido</b>: sem a divisão de itens, para economizar bobina.</li>
                <li>
                  <b>Mensagem eletrônica</b>: cartão com a chave, o QR Code e o link da nota, para enviar ao
                  cliente que dispensa o papel.
                </li>
              </GuideBullets>
              <GuideBullets>
                <li>
                  <b>Cancelar</b>: a venda não aconteceu. Exige justificativa e só vale dentro do
                  prazo legal, bem mais curto que o da NF-e.
                </li>
                <li>
                  <b>Substituir</b>: a venda aconteceu, mas a nota saiu errada. Você emite a nota
                  correta e informa a chave da substituída, amarrando as duas.
                </li>
              </GuideBullets>
              <p>
                Os dois são eventos e ficam registrados na linha do tempo do documento:{' '}
                <Link href="/guide/events" className="font-medium text-primary-700 underline underline-offset-2">
                  ver eventos
                </Link>.
              </p>
            </>
          ),
        },
      ]}
    />
  );
}
