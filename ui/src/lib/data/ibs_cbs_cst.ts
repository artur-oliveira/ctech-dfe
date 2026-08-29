/**
 * Códigos de Situação Tributária (CST) e Classificação Tributária (`cClassTrib`)
 * do IBS e da CBS. Fonte: Portal Nacional da NF-e, "cClassTrib 2026-06-22"
 * (Informe Técnico 2025.002), abas de CST e de cClassTrib publicadas.
 *
 * `requiresTax` sai da coluna `ind_gIBSCBS` da própria tabela: é ela que diz se
 * o CST exige o grupo com base de cálculo e alíquota no XML.
 */

export interface IbsCbsClassTrib {
  code: string   // 6 dígitos (cClassTrib)
  desc: string
}

export interface IbsCbsCstEntry {
  cst: string    // 3 dígitos
  desc: string
  /** true = exige vBC / pAliq / vIBS ou vCBS no XML (coluna ind_gIBSCBS) */
  requiresTax: boolean
  classCodes: IbsCbsClassTrib[]
}

export const IBS_CBS_CST: IbsCbsCstEntry[] = [
  {
    cst: "000", desc: "Tributação integral", requiresTax: true,
    classCodes: [
      {code: "000001", desc: "Situações tributadas integralmente pelo IBS e CBS."},
      {code: "000002", desc: "Exploração de via, observado o art. 11 da Lei Complementar nº 214, de 2025."},
      {code: "000003", desc: "Regime automotivo - projetos incentivados, observado o art. 311 da Lei Complementar nº 214, de 2025."},
      {code: "000004", desc: "Regime automotivo - projetos incentivados, observado o art. 312 da Lei Complementar nº 214, de 2025."},
      {code: "000005", desc: "Operação com EAC destinado à mistura com gasolina A, mas com saída do biocombustível com destinação diversa, observado o art. 179 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "010", desc: "Tributação com alíquotas uniformes", requiresTax: true,
    classCodes: [
      {code: "010001", desc: "Operações do FGTS não realizadas pela Caixa Econômica Federal, observado o art. 212 da Lei Complementar nº 214, de 2025."},
      {code: "010002", desc: "Operações do serviço financeiro"},
    ],
  },
  {
    cst: "011", desc: "Tributação com alíquotas uniformes reduzidas", requiresTax: true,
    classCodes: [
      {code: "011001", desc: "Planos de assistência funerária, observado o art. 236 da Lei Complementar nº 214, de 2025."},
      {code: "011002", desc: "Planos de assistência à saúde, observado o art. 237 da Lei Complementar nº 214, de 2025."},
      {code: "011003", desc: "Intermediação de planos de assistência à saúde, observado o art. 240 da Lei Complementar nº 214, de 2025."},
      {code: "011004", desc: "Concursos e prognósticos, observado o art. 246 da Lei Complementar nº 214, de 2025."},
      {code: "011005", desc: "Planos de assistência à saúde de animais domésticos, observado o art. 243 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "200", desc: "Alíquota reduzida", requiresTax: true,
    classCodes: [
      {code: "200001", desc: "Serviços de transporte de bens até as zonas de processamento de exportação e bens exportados a partir das zonas de processamento de exportação, observado o art. 103 da Lei Complementar nº 214, de 2025."},
      {code: "200002", desc: "Fornecimento ou importação de tratores, máquinas e implementos agrícolas, destinados a produtor rural não contribuinte, e de veículos de transporte de carga destinados a transportador autônomo de carga pessoa física não contribuinte, observado o art. 110 da Lei Complementar nº 214, de 2025."},
      {code: "200003", desc: "Vendas de produtos destinados à alimentação humana relacionados no Anexo I da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, que compõem a Cesta Básica Nacional de Alimentos, criada nos termos do art. 8º da Emenda Constitucional nº 132, de 20 de dezembro de 2023, observado o art. 125 da Lei Complementar nº 214, de 2025."},
      {code: "200004", desc: "Fornecimento de dispositivos médicos com a especificação das respectivas classificações da NCM/SH previstas no Anexo XII da Lei Complementar nº 214, de 2025, observado o art. 144 da Lei Complementar nº 214, de 2025."},
      {code: "200005", desc: "Fornecimento de dispositivos médicos com a especificação das respectivas classificações da NCM/SH previstas no Anexo IV da Lei Complementar nº 214, de 2025, quando adquiridos por órgãos da administração pública direta, autarquias, fundações públicas e entidades de saúde imunes, observado o art. 144 da Lei Complementar nº 214, de 2025."},
      {code: "200006", desc: "Situação de emergência de saúde pública reconhecida pelo Poder Legislativo federal, estadual, distrital ou municipal competente, ato conjunto do Ministro da Fazenda e do Comitê Gestor do IBS poderá ser editado, a qualquer momento, para incluir dispositivos não listados no Anexo XII da Lei Complementar nº 214, de 2025, limitada a vigência do benefício ao período e à localidade da emergência de saúde pública, observado o art. 144 da Lei Complementar nº 214, de 2025."},
      {code: "200007", desc: "Fornecimento dos dispositivos de acessibilidade próprios para pessoas com deficiência relacionados no Anexo XIII da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 145 da Lei Complementar nº 214, de 2025."},
      {code: "200008", desc: "Fornecimento dos dispositivos de acessibilidade próprios para pessoas com deficiência relacionados no Anexo V da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, quando adquiridos por órgãos da administração pública direta, autarquias, fundações públicas e entidades imunes, observado o art. 145 da Lei Complementar nº 214, de 2025."},
      {code: "200009", desc: "Fornecimento dos medicamentos registrados na Anvisa, observado o art. 146 da Lei Complementar nº 214, de 2025."},
      {code: "200010", desc: "Fornecimento dos medicamentos registrados na Anvisa, quando adquiridos por órgãos da administração pública direta, autarquias, fundações públicas e entidades imunes, observado o art. 146 da Lei Complementar nº 214, de 2025."},
      {code: "200011", desc: "Fornecimento das composições para nutrição enteral e parenteral, composições especiais e fórmulas nutricionais destinadas às pessoas com erros inatos do metabolismo relacionadas no Anexo VI da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, quando adquiridas por órgãos da administração pública direta, autarquias e fundações públicas, observado o art. 146 da Lei Complementar nº 214, de 2025."},
      {code: "200012", desc: "Situação de emergência de saúde pública reconhecida pelo Poder Legislativo federal, estadual, distrital ou municipal competente, ato conjunto do Ministro da Fazenda e do Comitê Gestor do IBS poderá ser editado, a qualquer momento, limitada a vigência do benefício ao período e à localidade da emergência de saúde pública, observado o art. 146 da Lei Complementar nº 214, de 2025."},
      {code: "200013", desc: "Fornecimento de tampões higiênicos, absorventes higiênicos internos ou externos, descartáveis ou reutilizáveis, calcinhas absorventes e coletores menstruais, observado o art. 147 da Lei Complementar nº 214, de 2025."},
      {code: "200014", desc: "Fornecimento dos produtos hortícolas, frutas e ovos, relacionados no Anexo XV da Lei Complementar nº 214 , de 2025, com a especificação das respectivas classificações da NCM/SH e desde que não cozidos, observado o art. 148 da Lei Complementar nº 214, de 2025."},
      {code: "200015", desc: "Venda de automóveis de passageiros de fabricação nacional de, no mínimo, 4 (quatro) portas, inclusive a de acesso ao bagageiro, quando adquiridos por motoristas profissionais que exerçam, comprovadamente, em automóvel de sua propriedade, atividade de condutor autônomo de passageiros, na condição de titular de autorização, permissão ou concessão do poder público, e que destinem o automóvel à utilização na categoria de aluguel (táxi), ou por pessoas com deficiência física, visual, auditiva, deficiência mental severa ou profunda, transtorno do espectro autista, com prejuízos na comunicação social e em padrões restritos ou repetitivos de comportamento de nível moderado ou grave, nos termos da legislação relativa à matéria, observado o disposto no art. 149 da Lei Complementar nº 214, de 2025."},
      {code: "200016", desc: "Prestação de serviços de pesquisa e desenvolvimento por Instituição Científica, Tecnológica e de Inovação (ICT) sem fins lucrativos para a administração pública direta, autarquias e fundações públicas ou para o contribuinte sujeito ao regime regular do IBS e da CBS, observado o disposto no art. 156 da Lei Complementar nº 214, de 2025."},
      {code: "200017", desc: "Operações relacionadas ao FGTS, considerando aquelas necessárias à aplicação da Lei nº 8.036, de 1990, realizadas pelo Conselho Curador ou Secretaria Executiva do FGTS, observado o art. 212 da Lei Complementar nº 214, de 2025."},
      {code: "200018", desc: "Operações de resseguro e retrocessão ficam sujeitas à incidência à alíquota zero, inclusive quando os prêmios de resseguro e retrocessão forem cedidos ao exterior, observado o art. 223 da Lei Complementar nº 214, de 2025."},
      {code: "200019", desc: "Importador dos serviços financeiros que seja contribuinte e tenha direito de apropriação de créditos na aquisição do mesmo serviço financeiro no País, observado o art. 231 da Lei Complementar nº 214, de 2025."},
      {code: "200020", desc: "Operação praticada por sociedades cooperativas optantes por regime específico do IBS e CBS, quando o associado destinar bem ou serviço à cooperativa de que participa, e a cooperativa fornecer bem ou serviço ao associado sujeito ao regime regular do IBS e da CBS, observado o art. 271 da Lei Complementar nº 214, de 2025."},
      {code: "200021", desc: "Serviços de transporte público coletivo de passageiros ferroviário e hidroviário urbanos, semiurbanos e metropolitanos, observado o art. 285 da Lei Complementar nº 214, de 2025."},
      {code: "200022", desc: "Operação originada fora da Zona Franca de Manaus que destine bem material industrializado de origem nacional a contribuinte estabelecido na Zona Franca de Manaus que seja habilitado nos termos do art. 442 da Lei Complementar nº 214, de 2025, e sujeito ao regime regular do IBS e da CBS ou optante pelo regime do Simples Nacional de que trata o art. 12 da Lei Complementar nº 123, de 2006, observado o art. 445 da Lei Complementar nº 214, de 2025."},
      {code: "200023", desc: "Operação realizada por indústria incentivada que destine bem material intermediário para outra indústria incentivada na Zona Franca de Manaus, desde que a entrega ou disponibilização dos bens ocorra dentro da referida área, observado o art. 448 da Lei Complementar nº 214, de 2025."},
      {code: "200024", desc: "Operação originada fora das Áreas de Livre Comércio que destine bem material industrializado de origem nacional a contribuinte estabelecido nas Áreas de Livre Comércio que seja habilitado nos termos do art. 456 da Lei Complementar nº 214, de 2025, e sujeito ao regime regular do IBS e da CBS ou optante pelo regime do Simples Nacional de que trata o art. 12 da Lei Complementar nº 123, de 2006, observado o art. 463 da Lei Complementar nº 214, de 2025."},
      {code: "200025", desc: "Fornecimento dos serviços de educação relacionados ao Programa Universidade para Todos (Prouni), instituído pela Lei nº 11.096, de 13 de janeiro de 2005, observado o art. 308 da Lei Complementar nº 214, de 2025."},
      {code: "200026", desc: "Locação de imóveis localizados nas zonas reabilitadas, pelo prazo de 5 (cinco) anos, contado da data de expedição do habite-se, e relacionados a projetos de reabilitação urbana de zonas históricas e de áreas críticas de recuperação e reconversão urbanística dos Municípios ou do Distrito Federal, a serem delimitadas por lei municipal ou distrital, observado o art. 158 da Lei Complementar nº 214, de 2025."},
      {code: "200027", desc: "Operações de locação, cessão onerosa e arrendamento de bens imóveis, observado o art. 261 da Lei Complementar nº 214, de 2025."},
      {code: "200028", desc: "Fornecimento dos serviços de educação relacionados no Anexo II da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da Nomenclatura Brasileira de Serviços, Intangíveis e Outras Operações que Produzam Variações no Patrimônio (NBS), observado o art. 129 da Lei Complementar nº 214, de 2025."},
      {code: "200029", desc: "Fornecimento dos serviços de saúde humana relacionados no Anexo III da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NBS, observado o art. 130 da Lei Complementar nº 214, de 2025."},
      {code: "200030", desc: "Venda dos dispositivos médicos relacionados no Anexo IV da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 131 da Lei Complementar nº 214, de 2025."},
      {code: "200031", desc: "Fornecimento dos dispositivos de acessibilidade próprios para pessoas com deficiência relacionados no Anexo V da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 132 da Lei Complementar nº 214, de 2025."},
      {code: "200032", desc: "Fornecimento dos medicamentos registrados na Anvisa ou produzidos por farmácias de manipulação, ressalvados os medicamentos sujeitos à alíquota zero de que trata o art. 146 da Lei Complementar nº 214, de 2025, observado o art. 133 da Lei Complementar nº 214, de 2025."},
      {code: "200033", desc: "Fornecimento das composições para nutrição enteral e parenteral, composições especiais e fórmulas nutricionais destinadas às pessoas com erros inatos do metabolismo relacionadas no Anexo VI da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 133 da Lei Complementar nº 214, de 2025."},
      {code: "200034", desc: "Fornecimento dos alimentos destinados ao consumo humano relacionados no Anexo VII da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 135 da Lei Complementar nº 214, de 2025."},
      {code: "200035", desc: "Fornecimento dos produtos de higiene pessoal e limpeza relacionados no Anexo VIII da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH, observado o art. 136 da Lei Complementar nº 214, de 2025."},
      {code: "200036", desc: "Fornecimento de produtos agropecuários, aquícolas, pesqueiros, florestais e extrativistas vegetais in natura, observado o art. 137 da Lei Complementar nº 214, de 2025."},
      {code: "200037", desc: "Fornecimento de serviços ambientais de conservação ou recuperação da vegetação nativa, mesmo que fornecidos sob a forma de manejo sustentável de sistemas agrícolas, agroflorestais e agrossilvopastoris, em conformidade com as definições e requisitos da legislação específica, observado o art. 137 da Lei Complementar nº 214, de 2025."},
      {code: "200038", desc: "Fornecimento dos insumos agropecuários e aquícolas relacionados no Anexo IX da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH e da NBS, observado o art. 138 da Lei Complementar nº 214, de 2025."},
      {code: "200039", desc: "Fornecimento dos bens e serviços listados no Anexo X da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NCM/SH e NBS, nos casos relacionados com produções nacionais artísticas, culturais, de eventos, jornalísticas e audiovisuais, observado o art. 139 da Lei Complementar nº 214, de 2025."},
      {code: "200040", desc: "Fornecimento dos seguintes serviços de comunicação institucional à administração pública direta, autarquias e fundações públicas: serviços direcionados ao planejamento, criação, programação e manutenção de páginas eletrônicas da administração pública, ao monitoramento e gestão de suas redes sociais e à otimização de páginas e canais digitais para mecanismos de buscas e produção de mensagens, infográficos, painéis interativos e conteúdo institucional, serviços de relações com a imprensa, que reúnem estratégias organizacionais para promover e reforçar a comunicação dos órgãos e das entidades contratantes com seus públicos de interesse, por meio da interação com profissionais da imprensa, e serviços de relações públicas, que compreendem o esforço de comunicação planejado, coeso e contínuo que tem por objetivo estabelecer adequada percepção da atuação e dos objetivos institucionais, a partir do estímulo à compreensão mútua e da manutenção de padrões de relacionamento e fluxos de informação entre os órgãos e as entidades contratantes e seus públicos de interesse, no País e no exterior, observado o art. 140 da Lei Complementar nº 214, de 2025."},
      {code: "200041", desc: "Operações relacionadas às seguintes atividades desportivas: fornecimento de serviço de educação desportiva, classificado no código 1.2205.12.00 da NBS, observado o art. 141 da Lei Complementar nº 214, de 2025."},
      {code: "200042", desc: "Operações relacionadas às seguintes atividades desportivas: gestão e exploração do desporto por associações e clubes esportivos filiados ao órgão estadual ou federal responsável pela coordenação dos desportos, observado o art. 141 da Lei Complementar nº 214, de 2025."},
      {code: "200043", desc: "Fornecimento à administração pública direta, autarquias e fundações púbicas dos serviços e dos bens relativos à soberania e à segurança nacional, à segurança da informação e à segurança cibernética relacionados no Anexo XI da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NBS e da NCM/SH, observado o art. 142 da Lei Complementar nº 214, de 2025."},
      {code: "200044", desc: "Operações e prestações de serviços de segurança da informação e segurança cibernética desenvolvidos por sociedade que tenha sócio brasileiro com o mínimo de 20% (vinte por cento) do seu capital social, relacionados no Anexo XI da Lei Complementar nº 214, de 2025, com a especificação das respectivas classificações da NBS e da NCM/SH, observado o art. 142 da Lei Complementar nº 214, de 2025."},
      {code: "200045", desc: "Operações relacionadas a projetos de reabilitação urbana de zonas históricas e de áreas críticas de recuperação e reconversão urbanística dos Municípios ou do Distrito Federal, a serem delimitadas por lei municipal ou distrital, observado o art. 158 da Lei Complementar nº 214, de 2025."},
      {code: "200046", desc: "Operações com bens imóveis, observado o art. 261 da Lei Complementar nº 214, de 2025."},
      {code: "200047", desc: "Bares e Restaurantes, observado o art. 275 da Lei Complementar nº 214, de 2025."},
      {code: "200048", desc: "Hotelaria, Parques de Diversão e Parques Temáticos, observado o art. 281 da Lei Complementar nº 214, de 2025."},
      {code: "200049", desc: "Transporte coletivo de passageiros rodoviário, ferroviário e hidroviário intermunicipais e interestaduais, observado o art. 286 da Lei Complementar nº 214, de 2025."},
      {code: "200050", desc: "Serviços de transporte aéreo regional coletivo de passageiros ou de carga, observado o art. 287 da Lei Complementar nº 214, de 2025."},
      {code: "200051", desc: "Agências de Turismo, observado o art. 289 da Lei Complementar nº 214, de 2025."},
      {code: "200052", desc: "Prestação de serviços das seguintes profissões intelectuais de natureza científica, literária ou artística, submetidas à fiscalização por conselho profissional: administradores, advogados, arquitetos e urbanistas, assistentes sociais, bibliotecários, biólogos, contabilistas, economistas, economistas domésticos, profissionais de educação física, engenheiros e agrônomos, estatísticos, médicos veterinários e zootecnistas, museólogos, químicos, profissionais de relações públicas, técnicos industriais e técnicos agrícolas, observado o art. 127 da Lei Complementar nº 214, de 2025."},
      {code: "200053", desc: "Fornecimento de medicamentos registrados na Anvisa, quando classificados como soros ou vacinas, observado o art. 146 da Lei Complementar nº 214, de 2025."},
      {code: "200054", desc: "Fornecimento de bem material pela cooperativa de produção agropecuária a associado não sujeito ao regime regular do IBS e da CBS com anulação de créditos referentes ao bem fornecido, observado o art. 271 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "220", desc: "Alíquota fixa", requiresTax: true,
    classCodes: [
      {code: "220001", desc: "Incorporação imobiliária submetida ao regime especial de tributação, observado o art. 485 da Lei Complementar nº 214, de 2025."},
      {code: "220002", desc: "Incorporação imobiliária submetida ao regime especial de tributação, observado o art. 485 da Lei Complementar nº 214, de 2025."},
      {code: "220003", desc: "Alienação de imóvel decorrente de parcelamento do solo, observado o art. 486 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "221", desc: "Alíquota fixa proporcional", requiresTax: true,
    classCodes: [
      {code: "221001", desc: "Locação, cessão onerosa ou arrendamento de bem imóvel com alíquota sobre a receita bruta, observado o art. 487 da Lei Complementar nº 214, de 2025."},
      {code: "221002", desc: "Incorporação imobiliária submetida ao regime especial de tributação, observado o art. 485 da Lei Complementar nº 214, de 2025."},
      {code: "221003", desc: "Incorporação imobiliária submetida ao regime especial de tributação, observado o art. 485 da Lei Complementar nº 214, de 2025."},
      {code: "221004", desc: "Alienação de imóvel decorrente de parcelamento do solo, observado o art. 486 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "222", desc: "Redução de base de cálculo", requiresTax: true,
    classCodes: [
      {code: "222001", desc: "Transporte internacional de passageiros, caso os trechos de ida e volta sejam vendidos em conjunto, a base de cálculo será a metade do valor cobrado, observado o Art. 12 § 8º da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "400", desc: "Isenção", requiresTax: false,
    classCodes: [
      {code: "400001", desc: "Fornecimento de serviços de transporte público coletivo de passageiros rodoviário e metroviário de caráter urbano, semiurbano e metropolitano, sob regime de autorização, permissão ou concessão pública, observado o art. 157 da Lei Complementar nº 214, de 2025."},
      {code: "400002", desc: "Fornecimento de serviços de transporte público coletivo de passageiros rodoviário e metroviário de caráter urbano, semiurbano e metropolitano, sob regime de autorização, permissão ou concessão pública, com medição por quilômetro rodado, observado o art. 157 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "410", desc: "Imunidade e não incidência", requiresTax: false,
    classCodes: [
      {code: "410001", desc: "Fornecimento de bonificações quando constem do respectivo documento fiscal e que não dependam de evento posterior, observado o art. 5º da Lei Complementar nº 214, de 2025."},
      {code: "410002", desc: "Transferências entre estabelecimentos pertencentes ao mesmo contribuinte, observado o art. 6º da Lei Complementar nº 214, de 2025."},
      {code: "410003", desc: "Doações que não tenham por objeto bens ou serviços que tenham permitido a apropriação de créditos pelo doador, observado o art. 6º da Lei Complementar nº 214, de 2025."},
      {code: "410004", desc: "Exportações de bens e serviços, observado o art. 8º da Lei Complementar nº 214, de 2025."},
      {code: "410005", desc: "Fornecimentos realizados pela União, pelos Estados, pelo Distrito Federal e pelos Municípios, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410006", desc: "Fornecimentos realizados por entidades religiosas e templos de qualquer culto, inclusive suas organizações assistenciais e beneficentes, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410007", desc: "Fornecimentos realizados por partidos políticos, inclusive suas fundações, entidades sindicais dos trabalhadores e instituições de educação e de assistência social, sem fins lucrativos, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410008", desc: "Fornecimentos de livros, jornais, periódicos e do papel destinado a sua impressão, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410009", desc: "Fornecimentos de fonogramas e videofonogramas musicais produzidos no Brasil contendo obras musicais ou literomusicais de autores brasileiros e/ou obras em geral interpretadas por artistas brasileiros, bem como os suportes materiais ou arquivos digitais que os contenham, salvo na etapa de replicação industrial de mídias ópticas de leitura a laser, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410010", desc: "Fornecimentos de serviço de comunicação nas modalidades de radiodifusão sonora e de sons e imagens de recepção livre e gratuita, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410011", desc: "Fornecimentos de ouro, quando definido em lei como ativo financeiro ou instrumento cambial, observado o art. 9º da Lei Complementar nº 214, de 2025."},
      {code: "410012", desc: "Fornecimento de condomínio edilício não optante pelo regime regular, observado o art. 26 da Lei Complementar nº 214, de 2025."},
      {code: "410013", desc: "Exportações de combustíveis, observado o art. 98 da Lei Complementar nº 214, de 2025."},
      {code: "410014", desc: "Fornecimento de produtor rural não contribuinte, observado o art. 164 da Lei Complementar nº 214, de 2025."},
      {code: "410015", desc: "Fornecimento por transportador autônomo não contribuinte, observado o art. 169 da Lei Complementar nº 214, de 2025."},
      {code: "410016", desc: "Fornecimento ou aquisição de resíduos sólidos, observado o art. 170 da Lei Complementar nº 214, de 2025."},
      {code: "410017", desc: "Aquisição de bem móvel com crédito presumido sob condição de revenda realizada, observado o art. 171 da Lei Complementar nº 214, de 2025."},
      {code: "410018", desc: "Operações relacionadas aos fundos garantidores e executores de políticas públicas, inclusive de habitação, previstos em lei, assim entendidas os serviços prestados ao fundo pelo seu agente operador e por entidade encarregada da sua administração, observado o art. 213 da Lei Complementar nº 214, de 2025."},
      {code: "410019", desc: "Exclusão da gorjeta na base de cálculo no fornecimento de alimentação, observado o art. 274 da Lei Complementar nº 214, de 2025."},
      {code: "410020", desc: "Exclusão do valor de intermediação na base de cálculo no fornecimento de alimentação, observado o art. 274 da Lei Complementar nº 214, de 2025."},
      {code: "410021", desc: "Contribuição de que trata o art. 149-A da Constituição Federal, observado o art. 12 da Lei Complementar nº 214, de 2025."},
      {code: "410022", desc: "Consolidação da propriedade pelo credor de bens móveis ou imóveis que tenham sido objeto de garantia, observado o art. 200 da Lei Complementar nº 214, de 2025."},
      {code: "410023", desc: "Alienação de bens móveis ou imóveis que tenham sido objeto de garantia constituída em favor de credor em que o prestador da garantia não seja contribuinte, observado o art. 200 da Lei Complementar nº 214, de 2025."},
      {code: "410024", desc: "Consolidação da propriedade pelo grupo de consórcio de bem que tenha sido objeto de garantia, observado o art. 204 da Lei Complementar nº 214, de 2025."},
      {code: "410025", desc: "Alienação de bem que tenha sido objeto de garantia constituída em favor do grupo de consórcio em que o prestador da garantia não seja contribuinte, observado o art. 204 da Lei Complementar nº 214, de 2025."},
      {code: "410026", desc: "Doações sem contraprestação em benefício do doador, com anulação de crédito apropriados pelo doador referente ao fornecimento doado, observado o art. 6º da Lei Complementar nº 214, de 2025."},
      {code: "410027", desc: "Fornecimento de bens e serviços, desde que vinculados direta e exclusivamente à exportação de bens materiais ou associados à entrega no exterior de bens materiais, observado o art. 6º da Lei Complementar nº 214, de 2025."},
      {code: "410028", desc: "Operações com bens imóveis realizadas por pessoas físicas não consideradas contribuintes do regime regular do IBS e da CBS, observado o art. 251 da Lei Complementar nº 214, de 2025."},
      {code: "410029", desc: "Operações não sujeitas à incidência de IBS e de CBS, alcançadas apenas por obrigação acessória do ICMS, observado o art. 4º da Lei Complementar nº 214, de 2025."},
      {code: "410030", desc: "Estorno de crédito apropriado de bens adquiridos e venham a perecer, deteriorar-se ou ser objeto de roubo, furto ou extravio, observado o art. 47 da Lei Complementar nº 214, de 2025."},
      {code: "410031", desc: "Fornecimento em período anterior ao início de vigência de incidências de CBS e IBS, observado o art. 544 da Lei Complementar nº 214, de 2025."},
      {code: "410032", desc: "Tributos incidentes na operação que não integram a base de cálculo do IBS e da CBS, observado o art. 12 da Lei Complementar nº 214, de 2025."},
      {code: "410033", desc: "Operações com bens imóveis, inclusive operações com direitos reais sobre bens imóveis, realizadas por Fundos de Investimento Imobiliário (FII) e Fundos de Investimento nas Cadeias Produtivas do Agronegócio (Fiagro), observado o art. 26 da Lei Complementar nº 214, de 2025."},
      {code: "410034", desc: "Fundos de investimento cujo patrimônio seja constituído exclusivamente por aplicações em participações societárias, certificados, direitos, títulos, valores mobiliários e demais ativos financeiros permitidos pela Comissão de Valores Mobiliários, observado o art. 26 da Lei Complementar nº 214, de 2025."},
      {code: "410035", desc: "Fornecimento realizado por nanoempreendedor, observado o art. 26 da Lei Complementar nº 214, de 2025."},
      {code: "410036", desc: "Descontos incondicionais, observado o art. 12 da Lei Complementar nº 214, de 2025."},
      {code: "410037", desc: "Importação os bens materiais sem incidência de IBS e CBS, observado o art. 66 da Lei Complementar nº 214, de 2025."},
      {code: "410999", desc: "Operações não onerosas sem previsão de tributação, não especificadas anteriormente, observado o art. 4º da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "510", desc: "Diferimento", requiresTax: true,
    classCodes: [
      {code: "510001", desc: "Operações, sujeitas a diferimento, com energia elétrica ou com direitos a ela relacionados, relativas à importação, geração, comercialização, distribuição e transmissão, observado o art. 28 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "515", desc: "Diferimento com redução de alíquota", requiresTax: true,
    classCodes: [
      {code: "515001", desc: "Operações, sujeitas a diferimento, com insumos agropecuários e aquícolas, observado o art. 138 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "550", desc: "Suspensão", requiresTax: true,
    classCodes: [
      {code: "550001", desc: "Exportações de bens materiais, observado o art. 82 da Lei Complementar nº 214, de 2025."},
      {code: "550002", desc: "Regime de Trânsito, observado o art. 84 da Lei Complementar nº 214, de 2025."},
      {code: "550003", desc: "Regimes de Depósito, observado o art. 85 da Lei Complementar nº 214, de 2025."},
      {code: "550004", desc: "Regimes de Depósito, observado o art. 87 da Lei Complementar nº 214, de 2025."},
      {code: "550005", desc: "Regimes de Depósito, observado o art. 87 da Lei Complementar nº 214, de 2025."},
      {code: "550006", desc: "Regimes de Permanência Temporária, observado o art. 88 da Lei Complementar nº 214, de 2025."},
      {code: "550007", desc: "Regimes de Aperfeiçoamento, observado o art. 90 da Lei Complementar nº 214, de 2025."},
      {code: "550008", desc: "Importação de bens para o Regime de Repetro-Temporário, de que tratam o inciso I do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550009", desc: "GNL-Temporário, de que trata o inciso II do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550010", desc: "Repetro-Permanente, de que trata o inciso III do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550011", desc: "Repetro-Industrialização, de que trata o inciso IV do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550012", desc: "Repetro-Nacional, de que trata o inciso V do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550013", desc: "Repetro-Entreposto, de que trata o inciso VI do art. 93 da Lei Complementar nº 214, de 2025."},
      {code: "550014", desc: "Zona de Processamento de Exportação, observado os arts. 99, 100 e 102 da Lei Complementar nº 214, de 2025."},
      {code: "550015", desc: "Regime Tributário para Incentivo à Modernização e à Ampliação da Estrutura Portuária - Reporto, observado o art. 105 da Lei Complementar nº 214, de 2025."},
      {code: "550016", desc: "Regime Especial de Incentivos para o Desenvolvimento da Infraestrutura - Reidi, observado o art. 106 da Lei Complementar nº 214, de 2025."},
      {code: "550017", desc: "Fornecimentos de embarcações registradas ou pré-registradas no Registro Especial Brasileiro - REB para incorporação ao ativo imobilizado de adquirente sujeito ao regime regular do IBS e da CBS, observado o art. 107 da Lei Complementar nº 214, de 2025."},
      {code: "550018", desc: "Desoneração da aquisição de bens de capital, observado o art. 109 da Lei Complementar nº 214, de 2025."},
      {code: "550019", desc: "Importação de bem material por indústria incentivada para utilização na Zona Franca de Manaus, observado o art. 443 da Lei Complementar nº 214, de 2025."},
      {code: "550020", desc: "Áreas de livre comércio, observado o art. 461 da Lei Complementar nº 214, de 2025."},
      {code: "550021", desc: "Fornecimento de produtos agropecuários in natura para contribuinte do regime regular que promova industrialização destinada a exportação, observado o art. 82 da Lei Complementar nº 214, de 2025."},
      {code: "550022", desc: "Regime Especial de Incentivos para a Produção de Hidrogênio de Baixa Emissão de Carbono (Rehidro), observado o art. 106 da Lei Complementar nº 214, de 2025."},
      {code: "550023", desc: "Operações com hidrocarbonetos líquidos derivados de petróleo não combustíveis ou de gás natural, inclusive nafta, observado o art. 172 da Lei Complementar nº 214, de 2025."},
      {code: "550024", desc: "Importações e nas aquisições no mercado interno de máquinas, equipamentos e veículos destinados a utilização nas atividades de que trata o inciso IIIdo art. 107 efetuadas para incorporação a seu ativo imobilizado, observado o art. 107 da Lei Complementar nº 214, de 2025."},
      {code: "550025", desc: "Importações e nas aquisições no mercado interno de matérias-primas, produtos intermediários, partes, peças e componentes para utilização na construção, conservação, modernização e reparo de embarcações pré-registradas ou registradas no REB, observado o art. 107 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "620", desc: "Tributação monofásica", requiresTax: false,
    classCodes: [
      {code: "620001", desc: "Tributação monofásica sobre combustíveis, observados os art. 172 da Lei Complementar nº 214, de 2025."},
      {code: "620002", desc: "Tributação monofásica com responsabilidade pela retenção sobre combustíveis, observado o art. 178 da Lei Complementar nº 214, de 2025."},
      {code: "620003", desc: "Tributação monofásica com responsabilidade de retenção de tributos por terceiros, observado o art. 178 da Lei Complementar nº 214, de 2025."},
      {code: "620004", desc: "Tributação monofásica sobre mistura de EAC com gasolina A em percentual superior ou inferior ao obrigatório, observado o art. 179 da Lei Complementar nº 214, de 2025."},
      {code: "620005", desc: "Tributação monofásica sobre mistura de EAC com gasolina A em percentual superior ou inferior ao obrigatório, observado o art. 179 da Lei Complementar nº 214, de 2025."},
      {code: "620006", desc: "Tributação monofásica sobre combustíveis cobrada anteriormente, observador o art. 180 da Lei Complementar nº 214, de 2025."},
      {code: "620007", desc: "Perecimento, deteriorização, roubo, furto ou extravio no regime monofásico sem estorno de crédito, observado o art. 47 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "800", desc: "Transferência de crédito", requiresTax: false,
    classCodes: [
      {code: "800001", desc: "Fusão, cisão ou incorporação, observado o art. 55 da Lei Complementar nº 214, de 2025."},
      {code: "800002", desc: "Transferência de crédito do associado, inclusive as cooperativas singulares, para cooperativa de que participa das operações antecedentes às operações em que fornece bens e serviços e os créditos presumidos, observado o art. 272 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "810", desc: "Ajuste de IBS na ZFM", requiresTax: false,
    classCodes: [
      {code: "810001", desc: "Crédito presumido de IBS sobre o valor apurado nos fornecimentos a partir da Zona Franca de Manaus, observado o art. 450 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "811", desc: "Ajustes", requiresTax: false,
    classCodes: [
      {code: "811001", desc: "Anulação de crédito proporcional ao valor das operações imunes e isentas, observado o art. 51 da Lei Complementar nº 214, de 2025."},
      {code: "811002", desc: "Débitos de notas fiscais não processadas na apuração, observado o art. 45 da Lei Complementar nº 214, de 2025."},
      {code: "811003", desc: "Débitos apurados após o desenquadramento do regime Simples Nacional, observado o art. 41 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "820", desc: "Tributação em documento específico", requiresTax: false,
    classCodes: [
      {code: "820001", desc: "Documento com informações de fornecimento de serviços de planos de assistência à saúde elencados no art. 234 da Lei Complementar nº 214, de 2025, mas com tributação realizada por outro meio"},
      {code: "820002", desc: "Documento com informações de fornecimento de serviços de planos de assinstência funerária, mas com tributação realizada por outro meio, observado o art. 236 da Lei Complementar nº 214, de 2025."},
      {code: "820003", desc: "Documento com informações de fornecimento de serviços de planos de assinstência à saúde de animais domésticos, mas com tributação realizada por outro meio, observado o art. 243 da Lei Complementar nº 214, de 2025."},
      {code: "820004", desc: "Documento com informações de prestação de serviços de consursos de prognósticos, mas com tributação realizada por outro meio, observado o art. 248 da Lei Complementar nº 214, de 2025."},
      {code: "820005", desc: "Documento com informações de alienação de bens imóveis, mas com tributação realizada por outro meio, observado o art. 254 da Lei Complementar nº 214, de 2025."},
      {code: "820006", desc: "Documento com informações de fornecimento de serviços de exploração de via, mas com tributação realizada por outro meio, observado o art. 11 da Lei Complementar nº 214, de 2025."},
      {code: "820007", desc: "Documento com informações de fornecimento de serviços financeiros, mas com tributação realizada por outro meio, observado o art. 181 da Lei Complementar nº 214, de 2025."},
      {code: "820008", desc: "Documento com informações de fornecimento de serviço continuado, mas com tributação realizada em fatura anterior, observado o art. 10 da Lei Complementar nº 214, de 2025."},
      {code: "820009", desc: "Cobrança relativa a fornecimentos declarados em outro documento, observado o art. 60 da Lei Complementar nº 214, de 2025."},
    ],
  },
  {
    cst: "830", desc: "Exclusão de base de cálculo", requiresTax: true,
    classCodes: [
      {code: "830001", desc: "Documento com exclusão da base de cálculo da CBS e do IBS refrente à energia elétrica fornecida pela distribuidora à unidade consumidora, conforme Art 28, parágrafos 3° e 4°."},
    ],
  },]

export const IBS_CBS_CST_OPTIONS = IBS_CBS_CST.map(({cst, desc}) => ({
  value: cst,
  label: `${cst} – ${desc}`,
}))

export const IBS_CBS_CLASS_BY_CST: Record<string, Array<{ value: string; label: string; }>> =
  Object.fromEntries(
    IBS_CBS_CST.map(({cst, classCodes}) => [
      cst,
      classCodes.map(({code, desc}) => ({
        value: code,
        label: `${code} – ${desc}`,
      })),
    ])
  )

/** Todo cClassTrib publicado, para validação. */
export const IBS_CBS_CLASS_CODES: ReadonlySet<string> =
  new Set(IBS_CBS_CST.flatMap((e) => e.classCodes.map((c) => c.code)))
