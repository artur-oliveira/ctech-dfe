'use client';

import {GuideBullets, GuideCallout, GuidePage, GuideSteps, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideNotifications() {
  return (
    <GuidePage
      currentHref="/guide/notifications"
      title="Notificações no navegador"
      description="Saiba na hora quando um documento é autorizado ou rejeitado e quando chega uma nota para a sua empresa, mesmo com o CTech DF-e fechado."
      sections={[
        {
          id: 'enable',
          title: 'Ativar neste dispositivo',
          summary: 'Cada pessoa ativa as notificações em cada aparelho que usa; ninguém recebe sem ter pedido.',
          image: {
            src: '/guide/notifications.webp',
            alt: 'Tela Notificações com o botão Ativar neste dispositivo, os avisos que podem ser escolhidos e a lista Meus dispositivos',
          },
          body: (
            <>
              <GuideSteps>
                <li>Em <b>Configurações</b>, abra <b>Notificações</b>.</li>
                <li>Clique em <b>Ativar neste dispositivo</b>. O navegador pergunta se permite os avisos; escolha <b>Permitir</b>.</li>
                <li>Pronto: o card passa a dizer <b>Notificações ativas neste dispositivo</b>. Use <b>Enviar teste</b> para conferir.</li>
              </GuideSteps>
              <GuideBullets>
                <li>O botão <b>Desativar neste dispositivo</b> para os avisos só deste aparelho; os outros continuam.</li>
                <li>Ao sair da conta, o aparelho é removido sozinho, para o próximo usuário não receber os seus avisos.</li>
                <li>Se o navegador bloqueou os avisos, a tela explica como liberar nas configurações do navegador.</li>
              </GuideBullets>
              <GuideCallout kind="info" title="O aviso nunca traz dados fiscais">
                A notificação diz só o que aconteceu (por exemplo, &quot;NF-e rejeitada&quot;) e o nome curto da empresa. Valores,
                clientes e números ficam dentro do sistema: toque no aviso para abrir o documento.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'ios',
          title: 'No iPhone e no iPad',
          summary: 'O Safari só envia notificações para o site instalado na Tela de Início.',
          image: {
            src: '/guide/notifications-ios.webp',
            alt: 'Tela Notificações no iPhone, com o passo a passo para adicionar o CTech DF-e à Tela de Início',
          },
          body: (
            <>
              <GuideSteps>
                <li>Abra o CTech DF-e no <b>Safari</b> do iPhone.</li>
                <li>Toque em <b>Compartilhar</b>, o ícone do quadrado com a seta para cima.</li>
                <li>Escolha <b>Adicionar à Tela de Início</b> e confirme.</li>
                <li>Abra o aplicativo pelo ícone novo, entre na conta uma vez e ative as notificações em <b>Configurações &gt; Notificações</b>.</li>
              </GuideSteps>
              <GuideCallout kind="warning" title="Exige iOS 16.4 ou mais">
                Em versões mais antigas do iOS não há notificações para sites. No Android e no computador não precisa instalar nada,
                mas o botão <b>Instalar o aplicativo</b> aparece quando o navegador oferece, para abrir o DF-e como um aplicativo.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'choose',
          title: 'Escolher o que avisar',
          summary: 'A escolha vale por empresa e por pessoa; não existe campo para digitar.',
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Nota recebida">Chegou um documento emitido contra a sua empresa (distribuição). Já vem marcado.</GuideTerm>
                <GuideTerm term="Documento autorizado">A SEFAZ ou a prefeitura autorizou uma emissão sua. Vem desmarcado.</GuideTerm>
                <GuideTerm term="Documento rejeitado">A autoridade recusou uma emissão; o motivo está no documento. Vem desmarcado.</GuideTerm>
                <GuideTerm term="Falha na emissão">O emissor não conseguiu concluir uma emissão. Vem desmarcado.</GuideTerm>
              </GuideTerms>
              <p>
                Marque ou desmarque na hora; a mudança é salva sozinha. A lista vale para <b>a empresa selecionada</b> e para todos os
                seus dispositivos. Para escolher de outra empresa, troque a empresa no topo da tela.
              </p>
            </>
          ),
        },
        {
          id: 'devices',
          title: 'Meus dispositivos',
          summary: 'Veja onde as notificações estão ativas e remova o que não usa mais.',
          body: (
            <>
              <p>
                A lista mostra o aparelho (por exemplo, Chrome no Android), a data do último uso e marca <b>Este dispositivo</b>.
                O limite é de <b>10 dispositivos</b> por pessoa: ao ativar o 11º, o que ficou mais tempo sem uso sai.
              </p>
              <GuideBullets>
                <li><b>Remover</b> apaga o dispositivo; para voltar a receber nele, ative de novo.</li>
                <li>Um aparelho que deixa de responder é removido pelo sistema sozinho.</li>
                <li>Todo clique numa notificação leva ao documento, trocando para a empresa certa se for preciso.</li>
              </GuideBullets>
            </>
          ),
        },
      ]}
    />
  );
}
