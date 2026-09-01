'use client'

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
          id: 'services',
          title: 'Serviços',
          summary:
            'O catálogo que responde item da LC 116, tributação do ISS e valor antes da emissão da NFS-e.',
          image: {
            src: '/guide/services.webp',
            alt: 'Lista de serviços com código, descrição, código de tributação nacional, incidência, retenção, alíquota e valor',
          },
          body: (
            <>
              <p>
                Cada serviço guarda o <b>código de tributação nacional</b> (o item da LC 116 em seis
                dígitos), a incidência do ISS, se há retenção pelo tomador e o valor padrão. Na
                emissão você escolhe o serviço e a tributação vem junto.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> código interno, descrição e código de tributação nacional.</li>
                <li>Serviço <b>tributável</b> exige alíquota do ISS; <b>imune</b>, <b>isento</b> e <b>não incidente</b> exigem o motivo, que vai no XML.</li>
                <li>Marcar <b>ISS retido</b> transfere o recolhimento ao tomador — a nota sai com o valor líquido destacado.</li>
                <li>O valor do cadastro é um padrão: a emissão aceita outro sem alterar o catálogo.</li>
              </GuideBullets>
              <GuideCallout kind="warning" title="O município valida o código, não nós">
                Código de tributação incompatível com a atividade do prestador é rejeitado pelo
                ambiente nacional. Na dúvida, confira o item da LC 116 no CNAE da empresa antes de
                cadastrar.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'operations',
          title: 'Naturezas de operação',
          summary:
            'O motivo da nota — venda, devolução, remessa — e o CFOP que ele determina.',
          image: {
            src: '/guide/operations.webp',
            alt: 'Lista de naturezas de operação com nome, finalidade, tipo e CFOP padrão',
          },
          body: (
            <>
              <p>
                A natureza responde três campos da NF-e de uma vez: o texto da natureza da operação,
                a finalidade (normal, complementar, ajuste ou devolução) e o sufixo do CFOP dos
                itens. O prefixo do CFOP sai do destino — mesma natureza, CFOP diferente para dentro
                e fora do estado.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome, finalidade e tipo de operação (entrada ou saída).</li>
                <li>Uma natureza pode ser marcada como <b>padrão</b> — vem selecionada em toda emissão nova.</li>
                <li>Devolução exige a nota referenciada na emissão; a natureza sozinha não basta.</li>
                <li>Sobrescrever o CFOP direto no item sempre vence o que veio da natureza.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'payment-terms',
          title: 'Condições de pagamento',
          summary: 'Forma, parcelas e vencimentos calculados — sem digitar duplicata a duplicata.',
          image: {
            src: '/guide/payment-terms.webp',
            alt: 'Lista de condições de pagamento com nome, forma, parcelas e intervalo entre vencimentos',
          },
          body: (
            <>
              <p>
                Escolher a condição na emissão gera as parcelas com vencimento calculado a partir da
                data de emissão: <i>À vista — Pix</i> fecha em uma parcela no dia; <i>30/60/90</i>
                {' '}gera três, espaçadas pelo intervalo cadastrado.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome, forma de pagamento e número de parcelas.</li>
                <li>Forma <b>a prazo</b> exige intervalo entre parcelas; à vista ignora o campo.</li>
                <li>A soma das parcelas tem de bater com o total da nota — a emissão avisa antes de enviar.</li>
                <li>Pagamento em cartão pede o terminal (ver <b>Terminais de pagamento</b>) para identificar a transação.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'import-declarations',
          title: 'Declarações de importação',
          summary: 'A DI que todo item importado precisa citar na NF-e.',
          image: {
            src: '/guide/import-declarations.webp',
            alt: 'Lista de declarações de importação com número da DI, data, local de desembaraço e UF',
          },
          body: (
            <>
              <p>
                Produto de origem estrangeira exige o grupo <b>DI</b> no item da nota. Cadastrar a
                declaração uma vez evita redigitar número, datas e adições em cada venda daquele
                lote importado.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> número da DI, data de registro, local e UF de desembaraço, data do desembaraço, via de transporte, forma de intermediação e código do exportador.</li>
                <li>Pelo menos <b>uma adição</b>, com número e código do fabricante.</li>
                <li>Intermediação <b>por conta e ordem</b> ou <b>por encomenda</b> exige o CNPJ do adquirente e a UF do terceiro.</li>
                <li>AFRMM só se aplica a transporte marítimo.</li>
              </GuideBullets>
              <GuideCallout kind="warning" title="Origem do produto tem de combinar">
                Item com origem nacional apontando para uma DI é rejeitado. Ajuste a origem no
                cadastro do produto antes de vincular a declaração.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'product-lots',
          title: 'Lotes de produção',
          summary: 'Rastreabilidade de medicamentos e afins: lote, fabricação e validade.',
          image: {
            src: '/guide/product-lots.webp',
            alt: 'Lista de lotes de produção com nome, produto, número do lote e validade',
          },
          body: (
            <>
              <p>
                Produtos sujeitos a rastreabilidade levam o grupo <b>rastro</b> na NF-e. O lote
                guarda número, quantidade produzida, fabricação e validade; a quantidade de cada
                nota é rateada da quantidade vendida.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> produto, número do lote, quantidade, data de fabricação e data de validade.</li>
                <li>A validade não pode ser anterior à fabricação.</li>
                <li>O código de agregação (ANVISA) é opcional e só se aplica a medicamentos.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'payment-terminals',
          title: 'Terminais de pagamento',
          summary: 'A maquininha que identifica a transação de cartão no cupom.',
          image: {
            src: '/guide/payment-terminals.webp',
            alt: 'Lista de terminais de pagamento com nome, CNPJ da credenciadora e identificador do terminal',
          },
          body: (
            <>
              <p>
                Pagamento em cartão na NFC-e leva o grupo <b>cartão</b>, com a credenciadora e o
                identificador do terminal. Cadastrar cada maquininha deixa a emissão em um clique.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome, CNPJ da credenciadora e identificador do terminal.</li>
                <li>Bandeira e UF são opcionais, mas algumas UFs as exigem em contingência.</li>
                <li>O identificador tem de ser o mesmo que a maquininha imprime no comprovante — é por ele que a SEFAZ concilia.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'fuel-pumps',
          title: 'Bombas de combustível',
          summary: 'Bico, bomba, tanque e o encerrante que a venda de posto exige.',
          image: {
            src: '/guide/fuel-pumps.webp',
            alt: 'Lista de bombas de combustível com nome, bico, bomba, tanque e última leitura do encerrante',
          },
          body: (
            <>
              <p>
                Venda de combustível a consumidor final exige o grupo <b>encerrante</b>: a leitura
                do bico antes e depois do abastecimento. O sistema guarda a última leitura final de
                cada bico e propõe a inicial da venda seguinte.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome e número do bico. Bomba e tanque são opcionais, mas a maioria dos estados os exige.</li>
                <li>A leitura final tem de ser maior que a inicial — encerrante que anda para trás é rejeitado.</li>
                <li>A última leitura é escrita pela emissão, não editável à mão.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'vehicle-sets',
          title: 'Composições veiculares',
          summary: 'Tração, reboques, condutor e RNTRC salvos sob um nome só.',
          image: {
            src: '/guide/vehicle-sets.webp',
            alt: 'Lista de composições veiculares com nome, veículo de tração, reboques e RNTRC',
          },
          body: (
            <>
              <p>
                Quem repete a mesma configuração de frota em toda viagem cadastra a composição uma
                vez: escolher a composição no MDF-e preenche o passo do veículo inteiro — tração,
                reboques e condutor.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome e veículo de tração.</li>
                <li>Reboques e condutores saem dos cadastros de <b>Veículos</b> e <b>Pessoas</b> — a composição referencia, não duplica.</li>
                <li>O RNTRC é obrigatório no transporte rodoviário de carga por conta de terceiros.</li>
                <li>Excluir um veículo usado por uma composição quebra a composição; ajuste-a antes.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'cargo-units',
          title: 'Unidades de transporte e de carga',
          summary: 'Contêineres, pallets, carretas e vagões, com os lacres.',
          image: {
            src: '/guide/cargo-units.webp',
            alt: 'Lista de unidades de transporte e de carga com nome, tipo, classificação e identificação',
          },
          body: (
            <>
              <p>
                O MDF-e separa <b>unidade de transporte</b> (a carreta ou o vagão que leva a carga) de
                <b> unidade de carga</b> (o contêiner ou pallet dentro dela). O mesmo cadastro atende
                os dois, com o tipo escolhido na criação.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome, classificação (transporte ou carga), tipo e identificação.</li>
                <li>Lacres são opcionais e podem ser vários — carga lacrada normalmente exige pelo menos um.</li>
                <li>O rateio da carga entre unidades é calculado na emissão, não no cadastro.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'toll-providers',
          title: 'Vale-pedágio',
          summary: 'A fornecedora do vale e quem paga por ele.',
          image: {
            src: '/guide/toll-providers.webp',
            alt: 'Lista de fornecedoras de vale-pedágio com nome, CNPJ da fornecedora e responsável pelo pagamento',
          },
          body: (
            <>
              <p>
                No transporte rodoviário de carga por conta de terceiros, o vale-pedágio é
                obrigatório e vai no manifesto com fornecedora, pagador e comprovante.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome e CNPJ da fornecedora.</li>
                <li>O pagador é CNPJ <b>ou</b> CPF, nunca os dois.</li>
                <li>O número do comprovante muda a cada viagem e continua sendo pedido na emissão.</li>
              </GuideBullets>
            </>
          ),
        },
        {
          id: 'insurance-policies',
          title: 'Apólices de seguro',
          summary: 'Seguradora, apólice e quem responde pelo seguro da carga.',
          image: {
            src: '/guide/insurance-policies.webp',
            alt: 'Lista de apólices de seguro com nome, responsável pelo seguro, seguradora e número da apólice',
          },
          body: (
            <>
              <p>
                O manifesto declara quem contratou o seguro da carga: o emitente do MDF-e ou o
                contratante do transporte. Cadastrar a apólice evita redigitar seguradora e número a
                cada viagem.
              </p>
              <GuideBullets>
                <li><b>Obrigatórios:</b> nome e responsável pelo seguro.</li>
                <li>Responsável identificado por CNPJ <b>ou</b> CPF, nunca os dois.</li>
                <li>Informar a apólice exige informar a seguradora; averbações são adicionadas na emissão.</li>
              </GuideBullets>
            </>
          ),
        },
      ]}
    />
  )
}
