'use client'

import Link from 'next/link'
import {GuideBullets, GuideCallout, GuidePage, GuideTerm, GuideTerms} from '@/components/guide/GuidePage'

export default function GuideCteMdfe() {
  return (
    <GuidePage
      currentHref="/guide/cte-mdfe"
      title="Transporte — CT-e e MDF-e"
      description="O MDF-e é o manifesto da viagem: amarra veículo, condutor e as notas que estão na carga. O CT-e, por enquanto, entra pelo lado do recebimento."
      sections={[
        {
          id: 'mdfe-list',
          title: 'Manifestos da operação',
          summary:
            'A lista mostra o trecho (UF de início e fim), peso e valor da carga, e em que ponto do ciclo cada manifesto está.',
          image: {
            src: '/guide/mdfe-list.webp',
            alt: 'Lista de MDF-e com número, série, trecho entre UFs, peso e valor da carga e status',
          },
          body: (
            <p>
              Um MDF-e passa por <b>Autorizado</b> quando a viagem começa e por <b>Encerrado</b>{' '}
              quando termina. Manifesto autorizado e não encerrado é pendência com a SEFAZ — encerrar
              faz parte da operação, não é opcional.
            </p>
          ),
        },
        {
          id: 'mdfe-emit',
          title: 'Emissão em cinco passos',
          summary:
            'Cada passo responde uma pergunta da viagem: como, o quê, quanto, por onde e com qual veículo.',
          image: {
            src: '/guide/mdfe-emit.webp',
            alt: 'Primeiro passo da emissão de MDF-e, com a escolha do modal de transporte',
          },
          body: (
            <>
              <GuideTerms>
                <GuideTerm term="1. Transporte">
                  O modal. Rodoviário está disponível; aéreo, aquaviário e ferroviário aparecem
                  marcados como em breve.
                </GuideTerm>
                <GuideTerm term="2. Documentos">
                  As chaves de NF-e ou CT-e que viajam na carga, agrupadas por município de descarga.
                </GuideTerm>
                <GuideTerm term="3. Carga">
                  Peso total, valor e produto predominante. A prévia soma o que veio dos documentos.
                </GuideTerm>
                <GuideTerm term="4. Trajeto">
                  UF de início e fim e as UFs de percurso, além dos municípios de carregamento.
                </GuideTerm>
                <GuideTerm term="5. Veículo">
                  Tração, reboques, condutor e RNTRC — vindos do cadastro de veículos e composições.
                </GuideTerm>
              </GuideTerms>
              <GuideCallout kind="tip" title="Composição veicular economiza o passo 5">
                Se o mesmo cavalo e carreta rodam sempre juntos, cadastre a composição uma vez em{' '}
                <b>Cadastros → Composições veiculares</b>. Na emissão, ela preenche veículo, reboques
                e condutor de uma vez.
              </GuideCallout>
            </>
          ),
        },
        {
          id: 'mdfe-detail',
          title: 'Durante e depois da viagem',
          summary:
            'O manifesto autorizado ainda aceita mudanças — condutor novo, nota que entrou depois — e precisa ser encerrado no destino.',
          image: {
            src: '/guide/mdfe-detail.webp',
            alt: 'Detalhe de um MDF-e autorizado, com documentos vinculados, trajeto, veículo e condutor',
          },
          body: (
            <>
              <GuideBullets>
                <li><b>Incluir condutor</b> — troca de motorista no meio do trajeto.</li>
                <li><b>Incluir DF-e</b> — nota que entrou na carga depois da autorização.</li>
                <li><b>Encerrar</b> — informa que a viagem terminou, com município e data.</li>
                <li><b>Cancelar</b> — a viagem não aconteceu, e o manifesto ainda não foi encerrado.</li>
              </GuideBullets>
              <p>
                Todos são eventos e ficam registrados na linha do tempo do documento —{' '}
                <Link href="/guide/events" className="font-medium text-primary-700 underline underline-offset-2">
                  ver eventos
                </Link>.
              </p>
            </>
          ),
        },
        {
          id: 'cte',
          title: 'CT-e hoje',
          summary:
            'A emissão de CT-e ainda está em desenvolvimento. O que já funciona é o lado de quem recebe.',
          image: {
            src: '/guide/cte-distribution.webp',
            alt: 'Tela de CT-e com as abas de recebidos e distribuição',
          },
          body: (
            <p>
              Os CT-e emitidos contra o seu CNPJ chegam pela distribuição da SEFAZ e ficam listados
              nas abas <b>Recebidos</b> e <b>Importação/Distribuição</b>, com XML disponível. A
              numeração e o ambiente de CT-e já podem ser configurados em Configuração Fiscal, para
              quando a emissão entrar no ar.
            </p>
          ),
        },
      ]}
    />
  )
}
