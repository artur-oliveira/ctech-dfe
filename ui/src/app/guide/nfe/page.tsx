'use client';

import Link from 'next/link';
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideNfe() {
  return (
    <GuidePage
      currentHref="/guide/nfe"
      title="Emitir NF-e"
      description="A Nota Fiscal Eletrônica de mercadoria, modelo 55. A emissão tem quatro passos e o sistema salva rascunho sozinho; dá para parar no meio e voltar."
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
                  O que a SEFAZ entregou por NSU e ainda não virou documento seu;{' '}
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
                nota em processamento mostra só Detalhes; não há o que baixar ainda.
              </p>
            </>
          ),
        },
        {
          id: 'receiver',
          title: 'Passo 1: Destinatário',
          summary:
            'Para quem é a nota e qual a natureza da operação. Os dois definem CFOP e finalidade do resto do preenchimento.',
          image: {
            src: '/guide/nfe-emit-receiver.webp',
            alt: 'Primeiro passo da emissão de NF-e, com natureza da operação, finalidade, atalhos de destinatários recentes e busca',
          },
          body: (
            <>
              <GuideBullets>
                <li>
                  <b>Recentes</b> traz os últimos destinatários; um clique substitui a busca.
                </li>
                <li>
                  <b>Para si mesmo</b> emite contra a própria empresa (remessa, transferência,
                  ajuste), sem escolher destinatário.
                </li>
                <li>
                  A <b>natureza da operação</b> preenche natureza, finalidade e o CFOP dos itens.
                  Qualquer campo que você editar depois vence a operação.
                </li>
                <li>
                  <b>Finalidade</b> vem da operação e pode ser trocada só nesta nota: normal,
                  complementar, ajuste, devolução, <b>nota de crédito</b> ou <b>nota de débito</b>.
                </li>
              </GuideBullets>
              <GuideCallout kind="info" title="Notas de crédito e de débito (reforma tributária)">
                Escolhida a finalidade 5 ou 6, aparece o seletor do tipo: o motivo do crédito (multa e
                juros, redução de valores, retorno por recusa na entrega...) ou do débito (perda em
                estoque, pagamento antecipado, desenquadramento do Simples...). O tipo é obrigatório
                para avançar. A nota de crédito é sempre de entrada e a de débito, de saída: o CFOP
                dos itens precisa concordar. Os itens levam só IBS e CBS, salvo os tipos que a regra
                da NT 2025.002 libera.
              </GuideCallout>
              <p>
                Locais de retirada e entrega são opcionais e só aparecem no XML quando diferentes do
                endereço cadastrado. Endereços usados com frequência podem ser salvos na pessoa. O
                local tem CEP, cidade escolhida na lista e inscrição estadual opcional (com a opção{' '}
                <b>Isento de inscrição</b>); marcado <b>Local no exterior</b>, o CEP sai, o país vem
                da tabela do Banco Central e o município fica 9999999.
              </p>
            </>
          ),
        },
        {
          id: 'products',
          title: 'Passo 2: Produtos',
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
                destinatário; operação dentro do estado e interestadual não usam o mesmo código.
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
          title: 'Passos 3 e 4: Pagamento e revisão',
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
                há pagamento a prazo. É a rejeição mais comum da NF-e, e é conta; o sistema fecha,
                não você.
              </p>
              <GuideCallout kind="tip" title="Rascunho automático">
                A emissão em andamento é salva no seu navegador. Se você fechar a aba, a próxima
                visita à tela de emissão oferece retomar de onde parou, ou descartar.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'local-refusal',
          title: 'Quando a nota volta antes da SEFAZ',
          summary:
            'Antes de reservar o número, o sistema confere a nota com o leiaute e com as regras do Manual de Orientação. O que não passa volta na hora, item por item, com o lugar de corrigir.',
          image: {
            src: '/guide/nfe-emit-error.webp',
            alt: 'Aviso de emissão recusada antes da SEFAZ, listando o logradouro do destinatário e o nome fantasia do emitente acima do limite do leiaute, cada um com o link Abrir cadastro',
          },
          body: (
            <>
              <p>
                Nada é enviado nem numerado: corrija o que a lista aponta e emita de novo. Cada linha
                diz o que está errado e traz o atalho para o lugar certo; <b>Abrir cadastro</b> leva
                à pessoa, à empresa ou ao produto, e <b>Editar</b> volta ao passo da nota.
              </p>
              <GuideTerms>
                <GuideTerm term="Texto acima do limite">
                  Cadastro antigo com texto maior que o leiaute aceita (o logradouro passa de 60
                  caracteres, a descrição do produto de 120). O sistema não corta por você: a linha
                  diz quantos caracteres sobram e onde editar.
                </GuideTerm>
                <GuideTerm term="Cadastro incompleto">
                  Falta um dado que a nota exige: UF ou município do endereço, o país de um endereço
                  no exterior, o município do ISSQN de um item de serviço.
                </GuideTerm>
                <GuideTerm term="Regra do Manual de Orientação">
                  A nota seria rejeitada pela SEFAZ. A linha mostra a regra e o código da rejeição que
                  ela evitou, e como corrigir: CFOP fora do destino, destinatário no exterior sem os
                  dados do país, nota de crédito sem o tipo, pagamentos que não fecham com o total.
                </GuideTerm>
                <GuideTerm term="Número malformado">
                  Um valor do cadastro (preço, alíquota) que não é um número válido; a linha nomeia o
                  campo.
                </GuideTerm>
              </GuideTerms>
              <p>
                Problema num local de retirada ou de entrega aparece também no próprio campo, quando
                você volta ao primeiro passo.
              </p>
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
                <li><b>XML</b>: o arquivo assinado e autorizado, o documento fiscal de fato.</li>
                <li>
                  <b>DANFE</b>: o PDF de acompanhamento da mercadoria. O seletor ao lado do botão escolhe o
                  formato; sem escolha, sai o formato do próprio documento.
                </li>
                <li><b>Carta de Correção</b> e <b>Cancelar</b>: eventos, cada um com a sua regra.</li>
              </GuideBullets>
              <p>
                Código e protocolo de autorização são a prova de que a SEFAZ aceitou. Guarde o XML:
                é ele, não o DANFE, que vale como documento fiscal.
              </p>
              <GuideBullets>
                <li><b>DANFE Retrato A4</b>: o padrão para arquivar e entregar com a mercadoria.</li>
                <li><b>DANFE Paisagem A4</b>: para notas com muitas colunas ou muitos itens.</li>
                <li><b>Simplificado (bobina)</b>: cupom para impressora térmica de 80 mm, para entrega rápida.</li>
                <li><b>Etiqueta</b>: para colar no volume, útil no envio de loja virtual.</li>
              </GuideBullets>
              <p>
                O simplificado e a etiqueta não existem para NF-e emitida em EPEC, então somem da lista nesse caso.
                Se um formato for recusado, o aviso diz o motivo e basta escolher outro.
              </p>
            </>
          ),
        },
        {
          id: 'fiscal-events',
          title: 'Eventos fiscais: entrega, ator interessado e reforma tributária',
          summary: 'Um único botão, Eventos fiscais, reúne os eventos que não são carta de correção nem cancelamento.',
          image: {
            src: '/guide/nfe-fiscal-events-menu.webp',
            alt: 'Menu Eventos fiscais da NF-e, com os grupos Entrega, Transporte e Reforma tributária',
          },
          body: (
            <>
              <p>
                O menu aparece em NF-e autorizada, para quem tem a permissão de eventos de NF-e. Cada entrada abre
                uma janela só com os campos daquele evento; o número de sequência é opcional e, vazio, vale o
                próximo livre. O resultado volta pela linha do tempo de eventos.
              </p>
              <GuideTerms>
                <GuideTerm term="Comprovante de entrega (110130)">
                  Só o emitente. Data e hora da entrega, nome e documento do recebedor, hash SHA-1 em Base64
                  (28 caracteres) e, se quiser, a data do hash e as coordenadas. Vai ao Ambiente Nacional.
                </GuideTerm>
                <GuideTerm term="Cancelar comprovante (110131)">
                  Escolha na lista o comprovante que a SEFAZ já autorizou; o protocolo vem do registro.
                </GuideTerm>
                <GuideTerm term="Ator interessado (110150)">
                  Indica o transportador (CPF ou CNPJ, escolhido no cadastro de pessoas) que pode baixar a NF-e. A
                  opção de subcontratados permite que ele autorize outros transportadores.
                </GuideTerm>
              </GuideTerms>
              <p>
                O leiaute de 110130, 110131, 110150 e 110300 segue as tabelas do MOC e ainda não foi conferido
                contra o XSD oficial; uma rejeição de layout pode acontecer.
              </p>
              <GuideCallout kind="warning" title="Reforma tributária">
                A SEFAZ pode não aceitar os eventos da reforma antes de 16/11/2026. Até lá, uma rejeição é esperada.
              </GuideCallout>
              <GuideTerms>
                <GuideTerm term="Emitente: 112110, 112140, 112150, 110300">
                  Pagamento integral (confirmação, que libera o crédito presumido do adquirente), fornecimento não
                  realizado (itens com IBS, CBS, quantidade e unidade), data de previsão de entrega e vinculação de
                  pagamento (transação, meio de pagamento, recebedor e PSP, ambos por CNPJ).
                </GuideTerm>
                <GuideTerm term="Destinatário: 211124, 211128, 211130, 211140, 211150">
                  Perecimento, aceite de débito (aceitar ou recusar), imobilização, crédito de combustível e crédito
                  dependente do adquirente. Aparecem nas NF-e recebidas.
                </GuideTerm>
                <GuideTerm term="Emitente ou destinatário: 211110">
                  Crédito presumido por item: base, código de 2 dígitos e, se houver, o percentual e o valor de IBS
                  e de CBS. Você escolhe quem emite o evento.
                </GuideTerm>
                <GuideTerm term="Sucessora: 212110 e 212120">
                  Aceitar ou recusar o pedido de transferência de crédito de IBS ou de CBS em sucessão.
                </GuideTerm>
              </GuideTerms>
              <p>
                Valores têm duas casas; quantidade, até quatro. Unidade, meio de pagamento e respostas vêm de listas.
                Cada item é identificado pelo número dele na nota, sem repetir.
              </p>
            </>
          ),
        },
        {
          id: 'reform-event-modal',
          title: 'Janela de um evento da reforma',
          summary: 'Só os campos do evento escolhido; o botão de envio espera tudo estar preenchido.',
          image: {
            src: '/guide/nfe-reform-event.webp',
            alt: 'Janela do evento Crédito presumido, com autor, item, base, código e os grupos de IBS e CBS',
          },
          body: (
            <p>
              No crédito presumido, o código de 2 dígitos é do item; dentro dos grupos de IBS e de CBS você informa
              só o percentual (até 4 casas) e o valor. Os itens são acrescentados com <b>Adicionar item</b>, até 990.
              O que falta preencher aparece em lista abaixo dos campos.
            </p>
          ),
        },
        {
          id: 'contingency',
          title: 'Quando a SEFAZ está fora do ar',
          summary:
            'SVC e EPEC são escolhidos na Configuração Fiscal da NF-e; a lista passa a mostrar o selo de contingência.',
          image: {
            src: '/guide/fiscal-config-emission.webp',
            alt: 'Aba NF-e da configuração fiscal em contingência SVC, com início e motivo',
          },
          body: (
            <>
              <p>
                Em <b>Configuração Fiscal → NF-e</b>, escolha <b>Contingência SVC</b> (SVC-AN ou SVC-RS conforme a UF
                do emitente) ou <b>EPEC</b> e informe o motivo. Se a UF da empresa não tem SVC designada, a emissão
                avisa e leva à configuração: escolha EPEC ou volte para Normal. Notas emitidas em EPEC ficam com o
                selo <b>Contingência pendente</b> até a transmissão, e o detalhe mostra o prazo.
              </p>
              <p>
                Ao voltar para Normal, a data de início da contingência é apagada. Veja o passo a passo em{' '}
                <Link href="/guide/account" className="font-medium text-primary-700 underline underline-offset-2">
                  Configuração fiscal
                </Link>.
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
                SEFAZ. Corrija o que a mensagem aponta (cadastro, CFOP, valor) e emita de novo. A
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
            'A lista vira cartão em tela pequena, com as mesmas ações; é o caso de uso mais comum fora do escritório.',
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
  );
}
