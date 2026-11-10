'use client';

import Link from 'next/link';
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage';

export default function GuideConta() {
  return (
    <GuidePage
      currentHref="/guide/account"
      title="Organização, usuários e assinatura"
      description="Uma conta pode operar várias empresas, cada uma com o seu certificado, a sua numeração e o seu catálogo. Quem vê o quê é definido por papel."
      sections={[
        {
          id: 'organizations',
          title: 'Empresas e papéis',
          summary:
            'A empresa selecionada no topo é o contexto de tudo: documentos, cadastros e configuração pertencem a ela.',
          image: {
            src: '/guide/organizations.webp',
            alt: 'Lista de organizações da conta, com razão social, CNPJ e papel do usuário em cada uma',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Proprietário">
                  Controle total, incluindo plano, cobrança e exclusão da empresa.
                </GuideTerm>
                <GuideTerm term="Administrador">
                  Gestão operacional: cadastros, configuração fiscal, certificados e convites.
                </GuideTerm>
                <GuideTerm term="Leitura">
                  Consulta documentos e baixa XML, sem emitir nem alterar cadastro.
                </GuideTerm>
              </GuideTerms>
              <p>
                Convites são por e-mail e por empresa; quem aceita entra só naquela organização, com
                o papel escolhido no convite. Trocar de empresa não exige sair da conta.
              </p>
              <p>
                Na tela da empresa, em <b>Enquadramento fiscal</b>, o campo <b>Regime de apuração do IBS e da
                CBS</b> só aparece para o Simples Nacional (CRT 1 ou 2), que escolhe se optou pelo regime
                regular, e para o Regime Normal (CRT 3) com opção pendente pelo Simples. Os demais deixam em
                branco; o valor vai na nota como <b>tpRegimeIBSCBS</b> (NT 2026.005).
              </p>
            </>
          ),
        },
        {
          id: 'organizations-manage',
          title: 'Criar, editar e vincular empresa',
          summary:
            'Empresa nova entra pelo CNPJ; empresa que já existe na conta de outra pessoa entra por vínculo.',
          body: (
            <>
              <p>
                <b>Nova organização</b> pede o CNPJ e consulta os dados públicos: razão social, nome
                fantasia, endereço e regime tributário chegam preenchidos, e você revisa. <b>Vincular
                organização</b> serve para o caso oposto: a empresa já está cadastrada por outra
                conta e você recebeu acesso a ela.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> CNPJ, razão social, endereço completo e regime tributário (CRT).</li>
                <li>A inscrição estadual determina se a empresa emite com destaque de ICMS; sem ela, só operações que dispensam IE.</li>
                <li>Criar e excluir empresa é ação de <b>proprietário</b>; editar é de proprietário ou administrador.</li>
                <li>O CNPJ não muda depois de criado; empresa errada se exclui e se recria.</li>
              </GuideBullets>
              <GuideCallout kind="warning" title="Excluir empresa apaga o histórico dela">
                Documentos, cadastros e certificados da empresa vão junto. Baixe os XML que precisa
                guardar antes; a guarda fiscal de cinco anos é sua responsabilidade.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'members',
          title: 'Usuários e convites',
          summary: 'Quem entra, com que papel, e como tirar acesso.',
          image: {
            src: '/guide/members.webp',
            alt: 'Lista de usuários da organização com nome, e-mail, papel e situação do convite',
          },
          body: (
            <>
              <p>
                A tela lista quem tem acesso à empresa selecionada e o papel de cada um. O convite é
                por e-mail: enquanto não for aceito, aparece como <b>pendente</b> e pode ser
                cancelado.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios no convite:</b> e-mail e papel.</li>
                <li>Só <b>proprietário</b> e <b>administrador</b> veem esta tela e enviam convites.</li>
                <li>O papel pode ser alterado depois; remover o acesso não apaga o que a pessoa emitiu.</li>
                <li>Toda empresa precisa de pelo menos um proprietário; o último não pode ser removido nem rebaixado.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'profile',
          title: 'Meu perfil',
          summary: 'Seus dados pessoais, separados dos dados da empresa.',
          image: {
            src: '/guide/profile.webp',
            alt: 'Tela de perfil do usuário com nome, sobrenome e e-mail da conta',
          },
          body: (
            <>
              <p>
                Nome e sobrenome são o que aparece no log de auditoria e para os colegas de equipe.
                A conta é da pessoa, não da empresa: o mesmo login atende todas as organizações em
                que você foi convidado.
              </p>
              <GuideBullets>
                <li>O e-mail identifica a conta e é o endereço dos convites; trocá-lo é feito na conta CTech, não aqui.</li>
                <li>Senha e sessão também moram na conta CTech; sair encerra a sessão em todas as empresas.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'assinatura',
          title: 'Plano e cobrança',
          summary:
            'O plano é da organização na conta CTech e vale para todas as empresas dela: define quantas podem emitir e quantos documentos de cada tipo por mês.',
          image: {
            src: '/guide/subscription.webp',
            alt: 'Tela do plano da organização com o plano ativo, o consumo do período e as faturas',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>Free</b>: uma empresa emitindo e uma cota pequena por tipo de documento.</li>
                <li><b>Pro</b>: várias empresas emitindo, com cota mensal alta.</li>
                <li><b>Sob demanda</b>: sem cota fixa; você paga por documento emitido e por empresa habilitada no mês.</li>
              </GuideBullets>
              <p>
                Proprietários e administradores da organização na conta CTech escolhem, trocam,
                cancelam e pagam o plano, e veem as faturas. Os demais membros veem o plano e o
                consumo, sem os botões. Trocar de empresa dentro da mesma organização mostra o mesmo
                plano.
              </p>
              <p>
                Uma empresa passa a contar no limite quando a primeira configuração fiscal dela é
                salva. Empresas apenas vinculadas, sem configuração, não contam. Trocar de plano vale a
                partir da mudança; cancelar no fim do período mantém o acesso até a data de renovação.
              </p>
              <GuideCallout kind="warning" title="Cota estourada bloqueia emissão">
                Ao atingir o limite do plano, novas emissões são recusadas até a renovação do período
                ou a troca de plano. Consulta, download de XML e eventos continuam liberados. Acima do
                limite de empresas, as que já emitem continuam emitindo; só habilitar uma nova é
                recusado.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'settings',
          title: 'Configuração fiscal por documento',
          summary:
            'Uma aba por tipo de documento, cada uma com ambiente, série, numeração e o que só ela exige.',
          image: {
            src: '/guide/fiscal-config.webp',
            alt: 'Configuração fiscal com abas por tipo de documento, trilho de seções à esquerda e a barra Salvar alterações',
          },
          body: (
            <>
            <p>
              Dentro de cada aba, um trilho de seções à esquerda (no celular, um seletor) mostra uma seção por vez:
              Ambiente, Numeração e série, Responsável técnico e Forma de emissão, mais CSC e QR Code na NFC-e e
              Manifesto no MDF-e; a NFS-e tem Provedor e ambiente, Município e numeração e, com o provedor
              ABRASF, uma seção própria. Um ponto na seção indica alteração não salva e um número, erros de
              validação; <b>Salvar alterações</b> grava tudo de uma vez e, se houver erro, abre a seção e leva
              o foco ao campo. A forma de emissão tem o próprio botão Aplicar.
            </p>
            <p>
              NF-e e CT-e mostram também o NSU da distribuição; NFC-e acrescenta a versão do QR Code e o CSC; MDF-e, o
              Canal Verde, a mensagem ao fisco e o RNTRC da transportadora (8 dígitos, da ANTT; opcional
              no MDF-e e obrigatório para emitir CT-e rodoviário); NFS-e, o provedor e o município de
              emissão. Produção e homologação têm série e número atual
              independentes, então testar nunca queima numeração válida.
            </p>
            </>
          ),
        },
        {
          id: 'emission',
          title: 'Forma de emissão e contingência',
          summary:
            'Em cada aba, a forma como o documento é emitido agora: normal ou uma contingência, para quando a SEFAZ está fora do ar.',
          image: {
            src: '/guide/fiscal-config-emission.webp',
            alt: 'Configuração fiscal da NF-e em contingência SVC, com o aviso de início e motivo, o seletor da forma de emissão e o aviso no topo do app',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>Normal</b>: a nota vai direto para a SEFAZ do seu estado.</li>
                <li><b>Contingência SVC</b>: usa o ambiente de contingência da SEFAZ (NF-e: SVC-AN ou SVC-RS conforme a UF; CT-e: SVC-RS ou SVC-SP).</li>
                <li><b>EPEC</b>: registro prévio na SEFAZ Nacional; só NF-e.</li>
                <li><b>Contingência offline</b>: a nota é assinada e impressa agora e fica pendente até a SEFAZ voltar (NFC-e, CT-e e MDF-e).</li>
              </GuideBullets>
              <p>
                Cada aba lista só as formas que valem para o documento. Sair de Normal pede um motivo de 15 a
                256 caracteres, que vai no XML; a data e hora de início ficam registradas e aparecem como{' '}
                <b>Em contingência desde</b>. Enquanto algum documento estiver fora de Normal, o topo do app mostra
                um aviso que leva direto a esta aba. Voltar para Normal não pede motivo e apaga a data de início.
              </p>
            </>
          ),
        },
        {
          id: 'auto-contingency',
          title: 'Contingência automática',
          summary:
            'Opcional e desligada por padrão: o sistema troca a forma de emissão sozinho quando a SEFAZ cai e volta a Normal quando ela se recupera.',
          image: {
            src: '/guide/fiscal-config-auto-contingency.webp',
            alt: 'Seção Forma de emissão da NF-e com Entrar em contingência sozinho ligado e o seletor obrigatório da contingência usada na troca',
          },
          body: (
            <>
              <p>
                A opção <b>Entrar em contingência sozinho</b> vem <b>desligada</b>. Ao ligar, escolha a contingência
                que o sistema deve usar (obrigatório): NF-e, <b>SVC</b> ou <b>EPEC</b>; NFC-e, <b>offline</b>; CT-e,{' '}
                <b>offline</b> ou <b>SVC</b>; MDF-e, <b>offline</b>. O sistema nunca troca para uma forma que você não escolheu.
              </p>
              <GuideBullets>
                <li>Vale para as emissões novas: quando a SEFAZ está fora do ar, o documento é gerado de novo na contingência, com outra chave de acesso.</li>
                <li>Quando a SEFAZ se recupera, o sistema volta a Normal sozinho e a data de início da contingência é apagada.</li>
                <li>Uma contingência que você escolheu manualmente nunca é desfeita pelo sistema.</li>
                <li>Na NF-e em EPEC, a nota fica com o selo <b>Aguardando EPEC</b> até o evento ser registrado na SEFAZ Nacional; depois passa a <b>Contingência pendente</b>.</li>
              </GuideBullets>
              <GuideCallout kind="info" title="Disponível em breve">
                A troca automática ainda não está ativa no servidor: o interruptor aparece travado com o selo Disponível em breve e
                hoje a mudança é sempre manual.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'audit',
          title: 'Log de auditoria',
          summary:
            'Quem mudou o quê, quando, e qual era o valor antes.',
          image: {
            src: '/guide/audit-logs.webp',
            alt: 'Log de auditoria com data, recurso, ação e usuário responsável',
          },
          body: (
            <p>
              Criações, alterações e exclusões de cadastro ficam registradas com autor e horário. Em
              alterações, o registro guarda o valor anterior e o novo, campo a campo; útil para
              entender por que uma nota saiu diferente do esperado na semana passada.
            </p>
          ),
        },
        {
          id: 'mobile',
          title: 'No celular',
          summary:
            'Todas as telas funcionam em tela pequena; consultar status e baixar XML fora do escritório é o caso de uso mais comum.',
          image: {
            src: '/guide/mobile-dashboard.webp',
            alt: 'Painel do CTech DF-e em tela de celular, com os atalhos de emissão empilhados',
          },
          body: (
            <p>
              A navegação principal desce para a barra inferior (Painel, documento atual, Emitir,
              Buscar e Menu), as tabelas viram cartões e os botões respeitam área de toque. A
              emissão completa também funciona no celular, embora o preenchimento de muitos itens
              seja mais confortável no desktop. Detalhes em{' '}
              <Link href="/guide/navigation" className="font-medium text-primary-700 underline underline-offset-2">
                Navegação
              </Link>.
            </p>
          ),
        },
      ]}
    />
  );
}
