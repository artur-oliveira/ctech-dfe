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
          id: 'persons',
          title: 'Pessoas',
          summary:
            'Clientes, fornecedores e transportadores no mesmo cadastro — o papel é definido pelo uso, não por um tipo fixo.',
          image: {
            src: '/guide/persons.webp',
            alt: 'Cadastro de pessoas com clientes e fornecedores, exibindo nome e CNPJ',
          },
          body: (
            <>
              <p>
                Uma pessoa guarda CPF ou CNPJ, inscrição estadual, endereços e contatos. Na emissão
                ela vira destinatário, transportador ou tomador conforme o campo em que você a
                escolhe.
              </p>
              <GuideCallout kind="tip" title="Inscrição estadual não é detalhe">
                É a IE do destinatário que define se a operação é para contribuinte ou consumidor
                final — e isso muda CFOP e tributação. Uma IE errada no cadastro vira rejeição na
                emissão.
              </GuideCallout>
            </>
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
      ]}
    />
  )
}
