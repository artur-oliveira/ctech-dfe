/**
 * Códigos de Situação Tributária (CST) e Classificação Tributária do IBS e CBS.
 * Aplicam-se igualmente ao IBS e ao CBS — NT 2024.001 (Reforma Tributária).
 * Gerado a partir de classificacao_tributaria.json.
 */

export interface IbsCbsClassTrib {
  code: string   // 6 dígitos (cClassTrib)
  desc: string
}

export interface IbsCbsCstEntry {
  cst: string    // 3 dígitos
  desc: string
  /** true = exige vBC / pAliq / vIBS ou vCBS no XML */
  requiresTax: boolean
  classCodes: IbsCbsClassTrib[]
}

export const IBS_CBS_CST: IbsCbsCstEntry[] = [
  {
    cst: '000', desc: 'Tributação integral', requiresTax: true,
    classCodes: [
      {code: '000001', desc: 'Situações tributadas integralmente pelo IBS e CBS.'},
      {code: '000002', desc: 'Exploração de via (art. 11 LC 214/2025).'},
      {code: '000003', desc: 'Regime automotivo – projetos incentivados (art. 311 LC 214/2025).'},
      {code: '000004', desc: 'Regime automotivo – projetos incentivados (art. 312 LC 214/2025).'},
      {code: '000005', desc: 'Operação com EAC destinado à mistura com gasolina A.'},
    ],
  },
  {
    cst: '010', desc: 'Tributação com alíquotas uniformes', requiresTax: true,
    classCodes: [
      {code: '010001', desc: 'Operações do FGTS não realizadas pela Caixa Econômica Federal.'},
      {code: '010002', desc: 'Operações do serviço financeiro.'},
    ],
  },
  {
    cst: '011', desc: 'Tributação com alíquotas uniformes reduzidas', requiresTax: true,
    classCodes: [
      {code: '011001', desc: 'Planos de assistência funerária (art. 236 LC 214/2025).'},
      {code: '011002', desc: 'Planos de assistência à saúde (art. 237 LC 214/2025).'},
      {code: '011003', desc: 'Intermediação de planos de assistência à saúde (art. 240 LC 214/2025).'},
      {code: '011004', desc: 'Concursos e prognósticos (art. 246 LC 214/2025).'},
      {code: '011005', desc: 'Planos de assistência à saúde de animais domésticos (art. 252 LC 214/2025).'},
    ],
  },
  {
    cst: '200', desc: 'Alíquota reduzida', requiresTax: true,
    classCodes: [
      {code: '200001', desc: 'Serviços de transporte de bens até zonas de processamento de exportação.'},
      {code: '200003', desc: 'Produtos destinados à alimentação humana (Anexo I LC 214/2025).'},
      {code: '200004', desc: 'Dispositivos médicos (especificação Anvisa).'},
      {code: '200009', desc: 'Medicamentos registrados na Anvisa (art. 124 LC 214/2025).'},
      {code: '200014', desc: 'Produtos hortícolas, frutas e ovos.'},
      {code: '200028', desc: 'Serviços de educação (Anexo II LC 214/2025).'},
      {code: '200029', desc: 'Serviços de saúde humana (Anexo III LC 214/2025).'},
      {code: '200036', desc: 'Produtos agropecuários, aquícolas, pesqueiros, florestais.'},
      {code: '200038', desc: 'Insumos agropecuários e aquícolas (Anexo VIII LC 214/2025).'},
      {code: '200039', desc: 'Bens e serviços listados no Anexo X.'},
      {code: '200047', desc: 'Bares e Restaurantes (art. 275 LC 214/2025).'},
      {code: '200048', desc: 'Hotelaria, Parques de Diversão e Parques Temáticos.'},
      {code: '200049', desc: 'Transporte coletivo rodoviário, ferroviário e hidroviário.'},
      {code: '200052', desc: 'Profissões intelectuais de natureza científica, literária ou artística.'},
    ],
  },
  {
    cst: '220', desc: 'Alíquota fixa', requiresTax: true,
    classCodes: [
      {code: '220001', desc: 'Incorporação imobiliária – regime especial (art. 257 LC 214/2025).'},
      {code: '220002', desc: 'Incorporação imobiliária – regime especial por unidade (art. 258 LC 214/2025).'},
      {code: '220003', desc: 'Alienação de imóvel decorrente de parcelamento do solo (art. 259 LC 214/2025).'},
    ],
  },
  {
    cst: '221', desc: 'Alíquota fixa proporcional', requiresTax: true,
    classCodes: [
      {code: '221001', desc: 'Locação, cessão onerosa ou arrendamento de bem imóvel com alíquota sobre receita.'},
    ],
  },
  {
    cst: '222', desc: 'Redução de Base de Cálculo', requiresTax: true,
    classCodes: [
      {code: '222001', desc: 'Transporte internacional de passageiros (trechos de ida e volta).'},
    ],
  },
  {
    cst: '400', desc: 'Isenção', requiresTax: false,
    classCodes: [
      {code: '400001', desc: 'Transporte público coletivo de passageiros – municipal e intermunicipal.'},
      {code: '400002', desc: 'Transporte público coletivo de passageiros – transporte aéreo regional.'},
    ],
  },
  {
    cst: '410', desc: 'Imunidade e não incidência', requiresTax: false,
    classCodes: [
      {code: '410001', desc: 'Bonificações constantes do respectivo documento fiscal.'},
      {code: '410002', desc: 'Transferências entre estabelecimentos do mesmo contribuinte.'},
      {code: '410004', desc: 'Exportações de bens e serviços (art. 8 LC 214/2025).'},
      {code: '410005', desc: 'Fornecimentos realizados pela União, Estados, DF e Municípios.'},
      {code: '410006', desc: 'Fornecimentos realizados por entidades religiosas e templos.'},
      {code: '410008', desc: 'Livros, jornais, periódicos e papel destinado à sua impressão.'},
      {code: '410028', desc: 'Operações com bens imóveis realizadas por pessoas físicas.'},
      {code: '410029', desc: 'Operações não sujeitas à incidência de IBS e CBS.'},
      {code: '410999', desc: 'Operações não onerosas sem previsão de tributação, não especificadas.'},
    ],
  },
  {
    cst: '510', desc: 'Diferimento', requiresTax: true,
    classCodes: [
      {code: '510001', desc: 'Operações com energia elétrica ou direito de acesso a redes elétricas.'},
    ],
  },
  {
    cst: '515', desc: 'Diferimento com redução de alíquota', requiresTax: true,
    classCodes: [
      {code: '515001', desc: 'Operações com insumos agropecuários e aquícolas (diferimento parcial).'},
    ],
  },
  {
    cst: '550', desc: 'Suspensão', requiresTax: true,
    classCodes: [
      {code: '550001', desc: 'Exportações de bens materiais (art. 82 LC 214/2025).'},
      {code: '550002', desc: 'Regime de Trânsito (art. 84 LC 214/2025).'},
      {code: '550003', desc: 'Regimes de Depósito (art. 85 LC 214/2025).'},
      {code: '550014', desc: 'Zona de Processamento de Exportação.'},
      {code: '550018', desc: 'Desoneração da aquisição de bens de capital (art. 109 LC 214/2025).'},
      {code: '550020', desc: 'Áreas de livre comércio (art. 461 LC 214/2025).'},
    ],
  },
  {
    cst: '620', desc: 'Tributação Monofásica', requiresTax: false,
    classCodes: [
      {code: '620001', desc: 'Tributação monofásica sobre combustíveis.'},
      {code: '620002', desc: 'Tributação monofásica com responsabilidade de retenção sobre combustíveis.'},
      {code: '620006', desc: 'Tributação monofásica sobre combustíveis cobrada anteriormente.'},
    ],
  },
  {
    cst: '800', desc: 'Transferência de crédito', requiresTax: false,
    classCodes: [
      {code: '800001', desc: 'Fusão, cisão ou incorporação (art. 55 LC 214/2025).'},
      {code: '800002', desc: 'Transferência de crédito do associado / cooperativas.'},
    ],
  },
  {
    cst: '810', desc: 'Ajuste de IBS na ZFM', requiresTax: false,
    classCodes: [
      {code: '810001', desc: 'Crédito presumido sobre fornecimentos a partir da Zona Franca de Manaus.'},
    ],
  },
  {
    cst: '811', desc: 'Ajustes', requiresTax: false,
    classCodes: [
      {code: '811001', desc: 'Anulação de crédito proporcional às operações imunes e isentas.'},
      {code: '811002', desc: 'Débitos de notas fiscais não processadas na apuração.'},
      {code: '811003', desc: 'Débitos após desenquadramento do Simples Nacional.'},
    ],
  },
  {
    cst: '820', desc: 'Tributação em documento específico', requiresTax: false,
    classCodes: [
      {code: '820001', desc: 'Informações de fornecimento de serviços de planos de assistência à saúde.'},
      {code: '820005', desc: 'Informações de alienação de bens imóveis.'},
      {code: '820008', desc: 'Informações de fornecimento com tributação realizada separadamente.'},
    ],
  },
  {
    cst: '830', desc: 'Exclusão da Base de Cálculo', requiresTax: true,
    classCodes: [
      {code: '830001', desc: 'Exclusão da base de cálculo do CBS e do IBS referente à gorjeta.'},
    ],
  },
]

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

/** CSTs que NÃO exigem vBC / pAliq / vIBS no XML */
export const IBS_CBS_EXEMPT_CST = new Set(
  IBS_CBS_CST.filter((e) => !e.requiresTax).map((e) => e.cst)
)
