'use client'

import Link from 'next/link'
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

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
                Convites são por e-mail e por empresa — quem aceita entra só naquela organização, com
                o papel escolhido no convite. Trocar de empresa não exige sair da conta.
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
                organização</b> serve para o caso oposto — a empresa já está cadastrada por outra
                conta e você recebeu acesso a ela.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> CNPJ, razão social, endereço completo e regime tributário (CRT).</li>
                <li>A inscrição estadual determina se a empresa emite com destaque de ICMS; sem ela, só operações que dispensam IE.</li>
                <li>Criar e excluir empresa é ação de <b>proprietário</b>; editar é de proprietário ou administrador.</li>
                <li>O CNPJ não muda depois de criado — empresa errada se exclui e se recria.</li>
              </GuideBullets>
              <GuideCallout kind="warning" title="Excluir empresa apaga o histórico dela">
                Documentos, cadastros e certificados da empresa vão junto. Baixe os XML que precisa
                guardar antes — a guarda fiscal de cinco anos é sua responsabilidade.
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
                <li>Toda empresa precisa de pelo menos um proprietário — o último não pode ser removido nem rebaixado.</li>
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
                <li>O e-mail identifica a conta e é o endereço dos convites — trocá-lo é feito na conta CTech, não aqui.</li>
                <li>Senha e sessão também moram na conta CTech; sair encerra a sessão em todas as empresas.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'assinatura',
          title: 'Plano e cobrança',
          summary:
            'O plano define quantas empresas, quantos usuários e quantos documentos de cada tipo por mês.',
          image: {
            src: '/guide/subscription.webp',
            alt: 'Tela de assinatura com o plano ativo, o consumo do período e as faturas',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>Free</b> — uma empresa, um usuário e uma cota pequena por tipo de documento.</li>
                <li><b>Pro</b> — várias empresas e usuários, com cota mensal alta.</li>
                <li><b>Sob demanda</b> — sem cota fixa: você paga por documento emitido.</li>
              </GuideBullets>
              <p>
                A tela mostra o consumo do período corrente e o histórico de faturas. Trocar de plano
                vale a partir da mudança; cancelar no fim do período mantém o acesso até a data de
                renovação.
              </p>
              <GuideCallout kind="warning" title="Cota estourada bloqueia emissão">
                Ao atingir o limite do plano, novas emissões são recusadas até a renovação do período
                ou a troca de plano. Consulta, download de XML e eventos continuam liberados.
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
            alt: 'Configuração fiscal com abas por tipo de documento e numeração separada por ambiente',
          },
          body: (
            <p>
              NF-e e CT-e mostram também o NSU da distribuição; NFC-e acrescenta o CSC; NFS-e, o
              provedor e o município de emissão. Produção e homologação têm série e número atual
              independentes, então testar nunca queima numeração válida.
            </p>
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
              alterações, o registro guarda o valor anterior e o novo, campo a campo — útil para
              entender por que uma nota saiu diferente do esperado na semana passada.
            </p>
          ),
        },
        {
          id: 'mobile',
          title: 'No celular',
          summary:
            'Todas as telas funcionam em tela pequena — consultar status e baixar XML fora do escritório é o caso de uso mais comum.',
          image: {
            src: '/guide/mobile-dashboard.webp',
            alt: 'Painel do CTech DF-e em tela de celular, com os atalhos de emissão empilhados',
          },
          body: (
            <p>
              A navegação principal desce para a barra inferior — Painel, documento atual, Emitir,
              Buscar e Menu —, as tabelas viram cartões e os botões respeitam área de toque. A
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
  )
}
