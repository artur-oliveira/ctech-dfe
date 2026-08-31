'use client'

import Link from 'next/link'
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideCadastros() {
  return (
    <GuidePage
      currentHref="/guide/registries"
      title="Cadastros que a emissão usa"
      description="Cada cadastro existe para tirar uma decisão da hora da emissão. Quanto mais completo o cadastro, menos campo você preenche por nota — e menos rejeição você toma."
      sections={[
        {
          id: 'where',
          title: 'Onde cada cadastro fica',
          summary:
            'Compartilhado fica em Cadastros; exclusivo mora dentro do documento que o usa.',
          image: {
            src: '/guide/nav-sidebar.webp',
            alt: 'Barra lateral com NFS-e aberta, mostrando Serviços, Locais de prestação e Documentos referenciados aninhados',
          },
          body: (
            <>
              <p>
                O bloco <b>Cadastros</b> da barra lateral guarda só o que vários documentos usam:
                <b> Pessoas</b> e <b>Produtos</b>. Todo cadastro que existe por causa de um único
                tipo de documento fica aninhado nele — <b>Serviços</b> dentro da NFS-e,
                <b> Veículos</b> dentro do MDF-e, <b>Naturezas de operação</b> dentro da NF-e.
              </p>
              <GuideCallout kind="tip" title="Não procure na barra: busque">
                <b>⌘K</b> (ou <b>Ctrl+K</b>) acha qualquer cadastro pelo nome ou pelo termo fiscal —
                <i> CFOP</i>, <i>placa</i>, <i>CST</i>. Ver <b>Navegação</b> no guia.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'persons',
          title: 'Pessoas',
          summary:
            'Um cadastro único e progressivo: informe o essencial e os papéis revelam somente os campos relevantes.',
          image: {
            src: '/guide/persons-form.webp',
            alt: 'Formulário de nova pessoa com identificação, papéis, endereço e dados complementares recolhidos',
          },
          body: (
            <>
              <p>
                Comece por documento, nome, papel e endereço. Marcar <b>Transportadora</b>,
                <b> Condutor</b> ou <b>Prestador</b> faz os respectivos dados de frete, pagamento
                ou NFS-e aparecerem em <b>Dados complementares e fiscais</b>. Um mesmo cadastro
                pode acumular vários papéis.
              </p>
              <GuideBullets>
                <li>Ao completar um CNPJ, o sistema consulta o CNPJá e tenta validar os dados fiscais na SEFAZ.</li>
                <li>Campos já editados não são sobrescritos pela consulta; divergências ficam sinalizadas para revisão.</li>
                <li>CNPJá funciona no primeiro cadastro. A validação SEFAZ depende de uma organização ativa com certificado.</li>
              </GuideBullets>
              <GuideCallout kind="tip" title="Inscrição estadual não é detalhe">
                É a IE do destinatário que define se a operação é para contribuinte ou consumidor
                final — e isso muda CFOP e tributação. Uma IE errada no cadastro vira rejeição na
                emissão.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'person-search',
          title: 'Encontre pelo papel',
          summary: 'A lista reúne todos os cadastros e deixa cada papel visível para a busca certa na emissão.',
          image: {
            src: '/guide/persons.webp',
            alt: 'Lista de pessoas com nome, documento e papéis de cliente e fornecedor',
          },
          body: (
            <p>
              Cliente, fornecedor, transportadora e prestador não precisam virar registros duplicados.
              Use os papéis para localizar a mesma pessoa no contexto certo e edite o cadastro uma só vez.
            </p>
          ),
        },
        {
          id: 'products',
          title: 'Produtos',
          summary:
            'O item da nota. Traz NCM, unidade, valor e o vínculo com o perfil fiscal que decide a tributação.',
          image: {
            src: '/guide/products.webp',
            alt: 'Catálogo de produtos com código, descrição, NCM e valores',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>NCM</b> — a classificação fiscal da mercadoria; define boa parte dos impostos.</li>
                <li><b>GTIN</b> — o código de barras, o que faz a busca por leitor funcionar na NFC-e.</li>
                <li><b>Unidade tributável e fatores de conversão</b> — para quem vende em caixa e tributa em unidade.</li>
                <li><b>Valor de revenda</b> — preço sugerido, separado do valor de custo.</li>
              </GuideBullets>
              <p>
                Na aba <b>Tipo Especial</b>, marque em <b>Este produto também tem</b> só o que se
                aplica: origem importada, regime da reforma, selo do IPI ou classificação de produto
                perigoso. O que não é marcado não aparece — um parafuso não precisa responder sobre
                número ONU.
              </p>
              <p>
                O catálogo de <b>serviços</b> é o equivalente para NFS-e: código de tributação
                nacional, alíquota de ISS e valor padrão do serviço.
              </p>
            </>
          ),
        },
        {
          id: 'tax-profiles',
          title: 'Perfis fiscais e naturezas de operação',
          summary:
            'Os dois cadastros que fazem o CFOP e a tributação aparecerem prontos na emissão.',
          image: {
            src: '/guide/tax-profiles.webp',
            alt: 'Lista de perfis fiscais com nome, descrição e CFOPs vinculados',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="Perfil fiscal">
                  A regra de tributação de um grupo de produtos: CST/CSOSN, alíquotas, CFOPs por
                  destino. Um produto aponta para um perfil e herda tudo.
                </GuideTerm>
                <GuideTerm term="Natureza de operação">
                  O motivo da nota — venda, remessa para conserto, devolução. Define natureza,
                  finalidade e o sufixo do CFOP dos itens.
                </GuideTerm>
                <GuideTerm term="Condição de pagamento">
                  Forma, número de parcelas e intervalo. Monta as parcelas da nota sem cálculo manual.
                </GuideTerm>
              </GuideTerms>
              <p>
                A natureza marcada como padrão já vem selecionada em toda emissão nova. Sobrescrever
                um campo na nota sempre vence o que veio do cadastro.
              </p>
              <p>
                Com um perfil escolhido no produto, a aba <b>Tributação</b> mostra só um resumo — a
                tributação já está respondida. O botão <b>Sobrescrever neste produto</b> abre a tabela
                completa quando um item precisa fugir da regra do perfil.
              </p>
            </>
          ),
        },
        {
          id: 'service-locations',
          title: 'Locais de prestação',
          summary:
            'Obra, imóvel e local de evento no mesmo cadastro — os papéis são combináveis.',
          image: {
            src: '/guide/service-locations.webp',
            alt: 'Lista de locais de prestação com nome, papéis, endereço e município',
          },
          body: (
            <>
              <p>
                A NFS-e pede o mesmo endereço em três lugares do leiaute: obra, atividade de evento
                e imóvel do IBS/CBS. Aqui ele é <b>um cadastro só</b>, com os papéis marcados —
                um canteiro que também é o imóvel tributado não vira dois registros iguais.
              </p>
              <GuideBullets>
                <li>Informe o código da obra (CNO) <b>ou</b> o CIB, nunca os dois: o leiaute escolhe um dos dois, ou o endereço.</li>
                <li>Locais no exterior não aceitam CNO, CIB nem inscrição imobiliária — são registros brasileiros.</li>
                <li>Nome e período do evento mudam a cada nota e continuam sendo pedidos na emissão.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'reference-documents',
          title: 'Documentos referenciados',
          summary:
            'Os documentos citados em dedução, redução, reembolso, repasse e ressarcimento.',
          image: {
            src: '/guide/reference-documents.webp',
            alt: 'Lista de documentos referenciados com nome, tipo, emissão e competência',
          },
          body: (
            <>
              <p>
                Escolha a família do documento e o formulário pede só o que aquela família exige:
                chave de acesso de um DF-e nacional, número e código de verificação de uma NFS-e
                municipal antiga, número/modelo/série de uma nota não eletrônica, ou o número de um
                documento fiscal ou não fiscal.
              </p>
              <GuideCallout kind="tip" title="Um cadastro, dois grupos do leiaute">
                O mesmo documento alimenta a dedução/redução e o reembolso/repasse/ressarcimento. O
                leiaute pede formas diferentes dele em cada grupo — cadastrar duas vezes seria
                convite à divergência.
              </GuideCallout>
              <GuideBullets>
                <li>A chave de NFS-e tem 50 dígitos e a de NF-e tem 44; o tipo escolhido valida o tamanho.</li>
                <li>O fornecedor aponta uma pessoa do cadastro — o documento referencia, nunca copia CNPJ e nome.</li>
                <li>A competência não pode ser anterior à emissão do documento.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'vehicles',
          title: 'Veículos e composições',
          summary:
            'A frota que aparece no MDF-e e no transporte da NF-e.',
          image: {
            src: '/guide/vehicles.webp',
            alt: 'Cadastro de veículos com placa, UF, tipo de rodado e capacidade',
          },
          body: (
            <>
              <p>
                Cada veículo guarda placa, UF, RENAVAM, tara, capacidade e tipo de rodado e
                carroceria — os campos que o manifesto exige. Reboques são cadastrados como veículos
                próprios e depois amarrados a uma tração.
              </p>
              <p>
                Uma <b>composição veicular</b> junta tração, reboques, condutor e RNTRC sob um nome.
                Na emissão do MDF-e, escolher a composição preenche o passo do veículo inteiro.
              </p>
            </>
          ),
        },
        {
          id: 'support',
          title: 'Os cadastros de apoio',
          summary:
            'Menos usados, mas cada um evita um campo repetido — ou uma rejeição — na emissão.',
          body: (
            <>
              <p>
                Todos vivem dentro do documento que os usa, na barra lateral. Nenhum é obrigatório
                para começar a emitir: cadastre quando a sua operação pedir.
              </p>
              <GuideTerms>
                <GuideTerm term="Condições de pagamento (NF-e)">
                  Prazo, número de parcelas e intervalo. Escolher a condição na emissão gera as
                  duplicatas com vencimento calculado.
                </GuideTerm>
                <GuideTerm term="Declarações de importação (NF-e)">
                  Número da DI, data, local de desembaraço e adições. Item importado sem DI é
                  rejeição certa.
                </GuideTerm>
                <GuideTerm term="Lotes de produção (NF-e)">
                  Lote, fabricação e validade dos produtos rastreados — medicamentos e itens sob
                  rastreabilidade obrigatória.
                </GuideTerm>
                <GuideTerm term="Terminais de pagamento (NFC-e)">
                  Maquininha, credenciadora e CNPJ. Identifica a transação de cartão no cupom.
                </GuideTerm>
                <GuideTerm term="Bombas de combustível (NFC-e)">
                  Bico, tanque e encerrante — os dados que o posto informa a cada abastecimento.
                </GuideTerm>
                <GuideTerm term="Unidades de carga (MDF-e)">
                  Contêiner, pallet ou vagão, com os lacres. Entra no manifesto junto com a carga.
                </GuideTerm>
                <GuideTerm term="Vale-pedágio (MDF-e)">
                  Fornecedor, CNPJ e comprovante. Obrigatório no transporte rodoviário de carga
                  quando há pedágio.
                </GuideTerm>
                <GuideTerm term="Apólices de seguro (MDF-e)">
                  Seguradora, apólice e averbação da carga transportada.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="info" title="Serviços da NFS-e ficam no tópico da NFS-e">
                O catálogo de serviços, com item da LC 116 e tributação do ISS, é explicado em{' '}
                <Link href="/guide/nfse" className="font-medium text-primary-700 underline underline-offset-2">
                  Emitir NFS-e
                </Link>.
              </GuideCallout>
            </>
          ),
        },
      ]}
    />
  )
}
