import type { CfopConfigItem } from '@/lib/types/api'

type CfopPrefix = '1' | '2' | '3' | '5' | '6' | '7'
type AnyDigit = '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9' | '0';
export type CfopCode = `${CfopPrefix}${AnyDigit}${AnyDigit}${AnyDigit}`

interface CfopEntry {
  code: CfopCode
  variants: CfopCode[]
  description: string
  nfe: boolean
  nfce: boolean
  devolution: boolean
  incoming: boolean
  cte: boolean
}

export interface DisplayCfop {
  value: string;
  label: string;
}

/**
 * CFOPs that represent operations without payment (e.g. remessa em bonificação,
 * doação ou brinde). When a product with one of these CFOPs is added to a fiscal
 * document, the payment must be set to "Sem pagamento" (tPag 90).
 */
export const NO_PAYMENT_CFOPS: CfopCode[] = ['5920', '6920']

const ALL_CFOPS: CfopEntry[] = [
  {
    code: "1101",
    variants: [
      "1101",
      "2101",
      "3101"
    ],
    description: "Compra p/ industrialização ou produção rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1102",
    variants: [
      "1102",
      "2102",
      "3102"
    ],
    description: "Compra p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1111",
    variants: [
      "1111",
      "2111"
    ],
    description: "Compra p/ industrialização de mercadoria recebida anteriormente em consignação industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1113",
    variants: [
      "1113",
      "2113"
    ],
    description: "Compra p/ comercialização, de mercadoria recebida anteriormente em consignação mercantil",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1116",
    variants: [
      "1116",
      "2116"
    ],
    description: "Compra p/ industrialização ou produção rural originada de encomenda p/ recebimento futuro",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1117",
    variants: [
      "1117",
      "2117"
    ],
    description: "Compra p/ comercialização originada de encomenda p/ recebimento futuro",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1118",
    variants: [
      "1118",
      "2118"
    ],
    description: "Compra de mercadoria p/ comercialização pelo adquirente originário, entregue pelo vendedor remetente ao destinatário, em venda à ordem.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1120",
    variants: [
      "1120",
      "2120"
    ],
    description: "Compra p/ industrialização, em venda à ordem, já recebida do vendedor remetente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1121",
    variants: [
      "1121",
      "2121"
    ],
    description: "Compra p/ comercialização, em venda à ordem, já recebida do vendedor remetente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1122",
    variants: [
      "1122",
      "2122"
    ],
    description: "Compra p/ industrialização em que a mercadoria foi remetida pelo fornecedor ao industrializador sem transitar pelo estabelecimento adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1124",
    variants: [
      "1124",
      "2124"
    ],
    description: "Industrialização efetuada por outra empresa",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1125",
    variants: [
      "1125",
      "2125"
    ],
    description: "Industrialização efetuada por outra empresa quando a mercadoria remetida p/ utilização no processo de industrialização não transitou pelo estabelecimento adquirente da mercadoria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1126",
    variants: [
      "1126",
      "2126",
      "3126"
    ],
    description: "Compra p/ utilização na prestação de serviço sujeita ao ICMS",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1128",
    variants: [
      "1128",
      "2128",
      "3128"
    ],
    description: "Compra p/ utilização na prestação de serviço sujeita ao ISSQN",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1151",
    variants: [
      "1151",
      "2151"
    ],
    description: "Transferência p/ industrialização ou produção rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1152",
    variants: [
      "1152",
      "2152"
    ],
    description: "Transferência p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1153",
    variants: [
      "1153",
      "2153"
    ],
    description: "Transferência de energia elétrica p/ distribuição",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1154",
    variants: [
      "1154",
      "2154"
    ],
    description: "Transferência p/ utilização na prestação de serviço",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1201",
    variants: [
      "1201",
      "2201",
      "3201"
    ],
    description: "Devolução de venda de produção do estabelecimento ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1202",
    variants: [
      "1202",
      "2202",
      "3202"
    ],
    description: "Devolução de venda de mercadoria adquirida ou recebida de terceiros",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1203",
    variants: [
      "1203",
      "2203"
    ],
    description: "Devolução de venda de produção do estabelecimento, destinada à ZFM ou ALC",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1204",
    variants: [
      "1204",
      "2204"
    ],
    description: "Devolução de venda de mercadoria adquirida ou recebida de terceiros, destinada à ZFM ou ALC",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1205",
    variants: [
      "1205",
      "2205",
      "3205"
    ],
    description: "Anulação de valor relativo à prestação de serviço de comunicação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1206",
    variants: [
      "1206",
      "2206",
      "3206"
    ],
    description: "Anulação de valor relativo à prestação de serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1207",
    variants: [
      "1207",
      "2207",
      "3207"
    ],
    description: "Anulação de valor relativo à venda de energia elétrica",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1208",
    variants: [
      "1208",
      "2208"
    ],
    description: "Devolução de produção do estabelecimento, remetida em transferência",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1209",
    variants: [
      "1209",
      "2209"
    ],
    description: "Devolução de mercadoria adquirida ou recebida de terceiros, remetida em transferência",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1212",
    variants: [
      "1212",
      "2212",
      "3212"
    ],
    description: "Devolução de venda no mercado interno de mercadoria industrializada e insumo importado sob o Regime Aduaneiro Especial de Entreposto Industrial (Recof-Sped)",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1251",
    variants: [
      "1251",
      "2251",
      "3251"
    ],
    description: "Compra de energia elétrica p/ distribuição ou comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1252",
    variants: [
      "1252",
      "2252"
    ],
    description: "Compra de energia elétrica por estabelecimento industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1253",
    variants: [
      "1253",
      "2253"
    ],
    description: "Compra de energia elétrica por estabelecimento comercial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1254",
    variants: [
      "1254",
      "2254"
    ],
    description: "Compra de energia elétrica por estabelecimento prestador de serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1255",
    variants: [
      "1255",
      "2255"
    ],
    description: "Compra de energia elétrica por estabelecimento prestador de serviço de comunicação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1256",
    variants: [
      "1256",
      "2256"
    ],
    description: "Compra de energia elétrica por estabelecimento de produtor rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1257",
    variants: [
      "1257",
      "2257"
    ],
    description: "Compra de energia elétrica p/ consumo por demanda contratada",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1301",
    variants: [
      "1301",
      "2301",
      "3301"
    ],
    description: "Aquisição de serviço de comunicação p/ execução de serviço da mesma natureza",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1302",
    variants: [
      "1302",
      "2302"
    ],
    description: "Aquisição de serviço de comunicação por estabelecimento industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1303",
    variants: [
      "1303",
      "2303"
    ],
    description: "Aquisição de serviço de comunicação por estabelecimento comercial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1304",
    variants: [
      "1304",
      "2304"
    ],
    description: "Aquisição de serviço de comunicação por estabelecimento de prestador de serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1305",
    variants: [
      "1305",
      "2305"
    ],
    description: "Aquisição de serviço de comunicação por estabelecimento de geradora ou de distribuidora de energia elétrica",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1306",
    variants: [
      "1306",
      "2306"
    ],
    description: "Aquisição de serviço de comunicação por estabelecimento de produtor rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1351",
    variants: [
      "1351",
      "2351",
      "3351"
    ],
    description: "Aquisição de serviço de transporte p/ execução de serviço da mesma natureza",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1352",
    variants: [
      "1352",
      "2352",
      "3352"
    ],
    description: "Aquisição de serviço de transporte por estabelecimento industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1353",
    variants: [
      "1353",
      "2353",
      "3353"
    ],
    description: "Aquisição de serviço de transporte por estabelecimento comercial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1354",
    variants: [
      "1354",
      "2354",
      "3354"
    ],
    description: "Aquisição de serviço de transporte por estabelecimento de prestador de serviço de comunicação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1355",
    variants: [
      "1355",
      "2355",
      "3355"
    ],
    description: "Aquisição de serviço de transporte por estabelecimento de geradora ou de distribuidora de energia elétrica",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1356",
    variants: [
      "1356",
      "2356",
      "3356"
    ],
    description: "Aquisição de serviço de transporte por estabelecimento de produtor rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1360",
    variants: [
      "1360"
    ],
    description: "Aquisição de serviço de transporte por contribuinte-substituto em relação ao serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1401",
    variants: [
      "1401",
      "2401"
    ],
    description: "Compra p/ industrialização ou produção rural de mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1403",
    variants: [
      "1403",
      "2403"
    ],
    description: "Compra p/ comercialização em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1406",
    variants: [
      "1406",
      "2406"
    ],
    description: "Compra de bem p/ o ativo imobilizado cuja mercadoria está sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1407",
    variants: [
      "1407",
      "2407"
    ],
    description: "Compra de mercadoria p/ uso ou consumo cuja mercadoria está sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1408",
    variants: [
      "1408",
      "2408"
    ],
    description: "Transferência p/ industrialização ou produção rural de mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1409",
    variants: [
      "1409",
      "2409"
    ],
    description: "Transferência p/ comercialização em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1410",
    variants: [
      "1410",
      "2410"
    ],
    description: "Devolução de venda de mercadoria, de produção do estabelecimento, sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1411",
    variants: [
      "1411",
      "2411"
    ],
    description: "Devolução de venda de mercadoria adquirida ou recebida de terceiros em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1414",
    variants: [
      "1414",
      "2414"
    ],
    description: "Retorno de mercadoria de produção do estabelecimento, remetida p/ venda fora do estabelecimento, sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1415",
    variants: [
      "1415",
      "2415"
    ],
    description: "Retorno de mercadoria adquirida ou recebida de terceiros, remetida p/ venda fora do estabelecimento em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1451",
    variants: [
      "1451"
    ],
    description: "Retorno de animal do estabelecimento produtor",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1452",
    variants: [
      "1452"
    ],
    description: "Retorno de insumo não utilizado na produção",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1501",
    variants: [
      "1501",
      "2501"
    ],
    description: "Entrada de mercadoria recebida com fim específico de exportação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1503",
    variants: [
      "1503",
      "2503",
      "3503"
    ],
    description: "Entrada decorrente de devolução de produto, de fabricação do estabelecimento, remetido com fim específico de exportação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1504",
    variants: [
      "1504",
      "2504"
    ],
    description: "Entrada decorrente de devolução de mercadoria remetida com fim específico de exportação, adquirida ou recebida de terceiros",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1505",
    variants: [
      "1505",
      "2505"
    ],
    description: "Entrada decorrente de devolução simbólica de mercadoria remetida p/ formação de lote de exportação, de produto industrializado ou produzido pelo próprio estabelecimento.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1506",
    variants: [
      "1506",
      "2506"
    ],
    description: "Entrada decorrente de devolução simbólica de mercadoria, adquirida ou recebida de terceiros, remetida p/ formação de lote de exportação.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1551",
    variants: [
      "1551",
      "2551",
      "3551"
    ],
    description: "Compra de bem p/ o ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1552",
    variants: [
      "1552",
      "2552"
    ],
    description: "Transferência de bem do ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1553",
    variants: [
      "1553",
      "2553",
      "3553"
    ],
    description: "Devolução de venda de bem do ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1554",
    variants: [
      "1554",
      "2554"
    ],
    description: "Retorno de bem do ativo imobilizado remetido p/ uso fora do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1555",
    variants: [
      "1555",
      "2555"
    ],
    description: "Entrada de bem do ativo imobilizado de terceiro, remetido p/ uso no estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1556",
    variants: [
      "1556",
      "2556",
      "3556"
    ],
    description: "Compra de material p/ uso ou consumo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1557",
    variants: [
      "1557",
      "2557"
    ],
    description: "Transferência de material p/ uso ou consumo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1601",
    variants: [
      "1601"
    ],
    description: "Recebimento, por transferência, de crédito de ICMS",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1602",
    variants: [
      "1602"
    ],
    description: "Recebimento, por transferência, de saldo credor do ICMS, de outro estabelecimento da mesma empresa, p/ compensação de saldo devedor do imposto. ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1603",
    variants: [
      "1603",
      "2603"
    ],
    description: "Ressarcimento de ICMS retido por substituição tributária",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1604",
    variants: [
      "1604"
    ],
    description: "Lançamento do crédito relativo à compra de bem p/ o ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1605",
    variants: [
      "1605"
    ],
    description: "Recebimento, por transferência, de saldo devedor do ICMS de outro estabelecimento da mesma empresa",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1651",
    variants: [
      "1651",
      "2651",
      "3651"
    ],
    description: "Compra de combustível ou lubrificante p/ industrialização subseqüente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1652",
    variants: [
      "1652",
      "2652",
      "2652",
      "3652"
    ],
    description: "Compra de combustível ou lubrificante p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1653",
    variants: [
      "1653",
      "2653",
      "3653"
    ],
    description: "Compra de combustível ou lubrificante por consumidor ou usuário final",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1658",
    variants: [
      "1658",
      "2658"
    ],
    description: "Transferência de combustível ou lubrificante p/ industrialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1659",
    variants: [
      "1659",
      "2659"
    ],
    description: "Transferência de combustível ou lubrificante p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1660",
    variants: [
      "1660",
      "2660"
    ],
    description: "Devolução de venda de combustível ou lubrificante destinados à industrialização subseqüente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1661",
    variants: [
      "1661",
      "2661"
    ],
    description: "Devolução de venda de combustível ou lubrificante destinados à comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1662",
    variants: [
      "1662",
      "2662"
    ],
    description: "Devolução de venda de combustível ou lubrificante destinados a consumidor ou usuário final",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1663",
    variants: [
      "1663",
      "2663"
    ],
    description: "Entrada de combustível ou lubrificante p/ armazenagem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1664",
    variants: [
      "1664",
      "2664"
    ],
    description: "Retorno de combustível ou lubrificante remetidos p/ armazenagem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1901",
    variants: [
      "1901",
      "2901"
    ],
    description: "Entrada p/ industrialização por encomenda",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1902",
    variants: [
      "1902",
      "2902"
    ],
    description: "Retorno de mercadoria remetida p/ industrialização por encomenda",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1903",
    variants: [
      "1903",
      "2903"
    ],
    description: "Entrada de mercadoria remetida p/ industrialização e não aplicada no referido processo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1904",
    variants: [
      "1904",
      "2904"
    ],
    description: "Retorno de remessa p/ venda fora do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1905",
    variants: [
      "1905",
      "2905"
    ],
    description: "Entrada de mercadoria recebida p/ depósito em depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1906",
    variants: [
      "1906",
      "2906"
    ],
    description: "Retorno de mercadoria remetida p/ depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1907",
    variants: [
      "1907",
      "2907"
    ],
    description: "Retorno simbólico de mercadoria remetida p/ depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1908",
    variants: [
      "1908",
      "2908"
    ],
    description: "Entrada de bem por conta de contrato de comodato",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1909",
    variants: [
      "1909",
      "2909"
    ],
    description: "Retorno de bem remetido por conta de contrato de comodato",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1910",
    variants: [
      "1910",
      "2910"
    ],
    description: "Entrada de bonificação, doação ou brinde",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1911",
    variants: [
      "1911",
      "2911"
    ],
    description: "Entrada de amostra grátis",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1912",
    variants: [
      "1912",
      "2912"
    ],
    description: "Entrada de mercadoria ou bem recebido p/ demonstração",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1913",
    variants: [
      "1913",
      "2913"
    ],
    description: "Retorno de mercadoria ou bem remetido p/ demonstração",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1914",
    variants: [
      "1914",
      "2914"
    ],
    description: "Retorno de mercadoria ou bem remetido p/ exposição ou feira",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1915",
    variants: [
      "1915",
      "2915"
    ],
    description: "Entrada de mercadoria ou bem recebido p/ conserto ou reparo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1916",
    variants: [
      "1916",
      "2916"
    ],
    description: "Retorno de mercadoria ou bem remetido p/ conserto ou reparo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1917",
    variants: [
      "1917",
      "2917"
    ],
    description: "Entrada de mercadoria recebida em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1918",
    variants: [
      "1918",
      "2918"
    ],
    description: "Devolução de mercadoria remetida em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1919",
    variants: [
      "1919",
      "2919"
    ],
    description: "Devolução simbólica de mercadoria vendida ou utilizada em processo industrial, remetida anteriormente em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "1920",
    variants: [
      "1920",
      "2920"
    ],
    description: "Entrada de vasilhame ou sacaria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1921",
    variants: [
      "1921",
      "2921"
    ],
    description: "Retorno de vasilhame ou sacaria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1922",
    variants: [
      "1922",
      "2922"
    ],
    description: "Lançamento efetuado a título de simples faturamento decorrente de compra p/ recebimento futuro",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1923",
    variants: [
      "1923",
      "2923"
    ],
    description: "Entrada de mercadoria recebida do vendedor remetente, em venda à ordem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1924",
    variants: [
      "1924",
      "2924"
    ],
    description: "Entrada p/ industrialização por conta e ordem do adquirente da mercadoria, quando esta não transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1925",
    variants: [
      "1925",
      "2925"
    ],
    description: "Retorno de mercadoria remetida p/ industrialização por conta e ordem do adquirente da mercadoria, quando esta não transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1926",
    variants: [
      "1926"
    ],
    description: "Lançamento efetuado a título de reclassificação de mercadoria decorrente de formação de kit ou de sua desagregação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1931",
    variants: [
      "1931",
      "2931"
    ],
    description: "Lançamento efetuado pelo tomador do serviço de transporte, quando a responsabilidade de retenção do imposto for atribuída ao remetente ou alienante da mercadoria, pelo serviço de transporte realizado por transportador autônomo ou por transportador não-inscrito na UF onde se tenha iniciado o serviço.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1932",
    variants: [
      "1932",
      "2932"
    ],
    description: "Aquisição de serviço de transporte iniciado em UF diversa daquela onde esteja inscrito o prestador",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1933",
    variants: [
      "1933",
      "2933"
    ],
    description: "Aquisição de serviço tributado pelo Imposto sobre Serviços de Qualquer Natureza",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1934",
    variants: [
      "1934",
      "2934"
    ],
    description: "Entrada simbólica de mercadoria recebida p/ depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "1949",
    variants: [
      "1949",
      "2949",
      "3949"
    ],
    description: "Outra entrada de mercadoria ou prestação de serviço não especificada",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "3127",
    variants: [
      "3127"
    ],
    description: "Compra p/ industrialização sob o regime de drawback ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "3129",
    variants: [
      "3129"
    ],
    description: "Compra para industrialização sob o Regime Aduaneiro Especial de Entreposto Industrial (Recof-Sped)",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "3211",
    variants: [
      "3211"
    ],
    description: "Devolução de venda de produção do estabelecimento sob o regime de drawback ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: true
  },
  {
    code: "3930",
    variants: [
      "3930"
    ],
    description: "Lançamento efetuado a título de entrada de bem sob amparo de regime especial aduaneiro de admissão temporária",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: true
  },
  {
    code: "5101",
    variants: [
      "5101",
      "6101",
      "7101"
    ],
    description: "Venda de produção do estabelecimento",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5102",
    variants: [
      "5102",
      "6102",
      "7102"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5103",
    variants: [
      "5103",
      "6103"
    ],
    description: "Venda de produção do estabelecimento efetuada fora do estabelecimento",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5104",
    variants: [
      "5104",
      "6104"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros, efetuada fora do estabelecimento",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5105",
    variants: [
      "5105",
      "6105",
      "7105"
    ],
    description: "Venda de produção do estabelecimento que não deva por ele transitar",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5106",
    variants: [
      "5106",
      "6106",
      "7106"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros, que não deva por ele transitar ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5109",
    variants: [
      "5109",
      "6109"
    ],
    description: "Venda de produção do estabelecimento destinada à ZFM ou ALC",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5110",
    variants: [
      "5110",
      "6110"
    ],
    description: "Venda de mercadoria, adquirida ou recebida de terceiros, destinada à ZFM ou ALC",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5111",
    variants: [
      "5111",
      "6111"
    ],
    description: "Venda de produção do estabelecimento remetida anteriormente em consignação industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5112",
    variants: [
      "5112",
      "6112"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros remetida anteriormente em consignação industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5113",
    variants: [
      "5113",
      "6113"
    ],
    description: "Venda de produção do estabelecimento remetida anteriormente em consignação mercantil",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5114",
    variants: [
      "5114",
      "6114"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros remetida anteriormente em consignação mercantil",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5115",
    variants: [
      "5115",
      "6115"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros, recebida anteriormente em consignação mercantil",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5116",
    variants: [
      "5116",
      "6116"
    ],
    description: "Venda de produção do estabelecimento originada de encomenda p/ entrega futura",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5117",
    variants: [
      "5117",
      "6117"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros, originada de encomenda p/ entrega futura",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5118",
    variants: [
      "5118",
      "6118"
    ],
    description: "Venda de produção do estabelecimento entregue ao destinatário por conta e ordem do adquirente originário, em venda à ordem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5119",
    variants: [
      "5119",
      "6119"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros entregue ao destinatário por conta e ordem do adquirente originário, em venda à ordem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5120",
    variants: [
      "5120",
      "6120"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros entregue ao destinatário pelo vendedor remetente, em venda à ordem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5122",
    variants: [
      "5122",
      "6122"
    ],
    description: "Venda de produção do estabelecimento remetida p/ industrialização, por conta e ordem do adquirente, sem transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5123",
    variants: [
      "5123",
      "6123"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros remetida p/ industrialização, por conta e ordem do adquirente, sem transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5124",
    variants: [
      "5124",
      "6124"
    ],
    description: "Industrialização efetuada p/ outra empresa",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5125",
    variants: [
      "5125",
      "6125"
    ],
    description: "Industrialização efetuada p/ outra empresa quando a mercadoria recebida p/ utilização no processo de industrialização não transitar pelo estabelecimento adquirente da mercadoria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5129",
    variants: [
      "5129",
      "6129",
      "7129"
    ],
    description: "Venda de insumo importado e de mercadoria industrializada sob o amparo do Regime Aduaneiro Especial de Entreposto Industrial (Recof-Sped)",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5151",
    variants: [
      "5151",
      "6151"
    ],
    description: "",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5152",
    variants: [
      "5152",
      "6152"
    ],
    description: "Transferência de mercadoria adquirida ou recebida de terceiros",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5153",
    variants: [
      "5153",
      "6153"
    ],
    description: "Transferência de energia elétrica",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5155",
    variants: [
      "5155",
      "6155"
    ],
    description: "Transferência de produção do estabelecimento, que não deva por ele transitar",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5156",
    variants: [
      "5156",
      "6156"
    ],
    description: "Transferência de mercadoria adquirida ou recebida de terceiros, que não deva por ele transitar",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5201",
    variants: [
      "5201",
      "6201",
      "7201"
    ],
    description: "Devolução de compra p/ industrialização ou produção rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5202",
    variants: [
      "5202",
      "6202",
      "7202"
    ],
    description: "Devolução de compra p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5205",
    variants: [
      "5205",
      "6205",
      "7205"
    ],
    description: "Anulação de valor relativo a aquisição de serviço de comunicação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5206",
    variants: [
      "5206",
      "6206",
      "7206"
    ],
    description: "Anulação de valor relativo a aquisição de serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5207",
    variants: [
      "5207",
      "6207",
      "7207"
    ],
    description: "Anulação de valor relativo à compra de energia elétrica",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5208",
    variants: [
      "5208",
      "6208"
    ],
    description: "Devolução de mercadoria recebida em transferência p/ industrialização ou produção rural ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5209",
    variants: [
      "5209",
      "6209"
    ],
    description: "Devolução de mercadoria recebida em transferência p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5210",
    variants: [
      "5210",
      "6210",
      "7210"
    ],
    description: "Devolução de compra p/ utilização na prestação de serviço",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5251",
    variants: [
      "5251",
      "6251",
      "7251"
    ],
    description: "Venda de energia elétrica p/ distribuição ou comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5252",
    variants: [
      "5252",
      "6252"
    ],
    description: "Venda de energia elétrica p/ estabelecimento industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5253",
    variants: [
      "5253",
      "6253"
    ],
    description: "Venda de energia elétrica p/ estabelecimento comercial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5254",
    variants: [
      "5254",
      "6254"
    ],
    description: "Venda de energia elétrica p/ estabelecimento prestador de serviço de transporte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5255",
    variants: [
      "5255",
      "6255"
    ],
    description: "Venda de energia elétrica p/ estabelecimento prestador de serviço de comunicação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5256",
    variants: [
      "5256",
      "6256"
    ],
    description: "Venda de energia elétrica p/ estabelecimento de produtor rural",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5257",
    variants: [
      "5257",
      "6257"
    ],
    description: "Venda de energia elétrica p/ consumo por demanda contratada",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5258",
    variants: [
      "5258",
      "6258"
    ],
    description: "Venda de energia elétrica a não contribuinte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5301",
    variants: [
      "5301",
      "6301",
      "7301"
    ],
    description: "Prestação de serviço de comunicação p/ execução de serviço da mesma natureza",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5302",
    variants: [
      "5302",
      "6302"
    ],
    description: "Prestação de serviço de comunicação a estabelecimento industrial",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5303",
    variants: [
      "5303",
      "6303"
    ],
    description: "Prestação de serviço de comunicação a estabelecimento comercial",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5304",
    variants: [
      "5304",
      "6304"
    ],
    description: "Prestação de serviço de comunicação a estabelecimento de prestador de serviço de transporte",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5305",
    variants: [
      "5305",
      "6305"
    ],
    description: "Prestação de serviço de comunicação a estabelecimento de geradora ou de distribuidora de energia elétrica",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5306",
    variants: [
      "5306",
      "6306"
    ],
    description: "Prestação de serviço de comunicação a estabelecimento de produtor rural",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5307",
    variants: [
      "5307",
      "6307"
    ],
    description: "Prestação de serviço de comunicação a não contribuinte",
    nfe: false,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5351",
    variants: [
      "5351",
      "6351"
    ],
    description: "Prestação de serviço de transporte p/ execução de serviço da mesma natureza",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5352",
    variants: [
      "5352",
      "6352"
    ],
    description: "Prestação de serviço de transporte a estabelecimento industrial",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5353",
    variants: [
      "5353",
      "6353"
    ],
    description: "Prestação de serviço de transporte a estabelecimento comercial",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5354",
    variants: [
      "5354",
      "6354"
    ],
    description: "Prestação de serviço de transporte a estabelecimento de prestador de serviço de comunicação",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5355",
    variants: [
      "5355",
      "6355"
    ],
    description: "Prestação de serviço de transporte a estabelecimento de geradora ou de distribuidora de energia elétrica",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5356",
    variants: [
      "5356",
      "6356"
    ],
    description: "Prestação de serviço de transporte a estabelecimento de produtor rural",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5357",
    variants: [
      "5357",
      "6357"
    ],
    description: "Prestação de serviço de transporte a não contribuinte",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5359",
    variants: [
      "5359",
      "6359"
    ],
    description: "Prestação de serviço de transporte a contribuinte ou a não-contribuinte, quando a mercadoria transportada esteja dispensada de emissão de Nota Fiscal  ",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5360",
    variants: [
      "5360",
      "6360"
    ],
    description: "Prestação de serviço de transporte a contribuinte-substituto em relação ao serviço de transporte",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5401",
    variants: [
      "5401",
      "6401"
    ],
    description: "Venda de produção do estabelecimento quando o produto esteja sujeito a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5402",
    variants: [
      "5402",
      "6402"
    ],
    description: "Venda de produção do estabelecimento de produto sujeito a ST, em operação entre contribuintes substitutos do mesmo produto",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5403",
    variants: [
      "5403",
      "6403"
    ],
    description: "Venda de mercadoria, adquirida ou recebida de terceiros, sujeita a ST, na condição de contribuinte-substituto",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5405",
    variants: [
      "5405"
    ],
    description: "Venda de mercadoria, adquirida ou recebida de terceiros, sujeita a ST, na condição de contribuinte-substituído",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5408",
    variants: [
      "5408",
      "6408"
    ],
    description: "Transferência de produção do estabelecimento quando o produto sujeito a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5409",
    variants: [
      "5409",
      "6409"
    ],
    description: "Transferência de mercadoria adquirida ou recebida de terceiros em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5410",
    variants: [
      "5410",
      "6410"
    ],
    description: "Devolução de compra p/ industrialização de mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5411",
    variants: [
      "5411",
      "6411"
    ],
    description: "Devolução de compra p/ comercialização em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5412",
    variants: [
      "5412",
      "6412"
    ],
    description: "Devolução de bem do ativo imobilizado, em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5413",
    variants: [
      "5413",
      "6413"
    ],
    description: "Devolução de mercadoria destinada ao uso ou consumo, em operação com mercadoria sujeita a ST.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5414",
    variants: [
      "5414",
      "6414"
    ],
    description: "Remessa de produção do estabelecimento p/ venda fora do estabelecimento, quando o produto sujeito a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5415",
    variants: [
      "5415",
      "6415"
    ],
    description: "Remessa de mercadoria adquirida ou recebida de terceiros p/ venda fora do estabelecimento, em operação com mercadoria sujeita a ST",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5451",
    variants: [
      "5451"
    ],
    description: "Remessa de animal e de insumo p/ estabelecimento produtor",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5501",
    variants: [
      "5501",
      "6501",
      "7501"
    ],
    description: "Remessa de produção do estabelecimento, com fim específico de exportação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5502",
    variants: [
      "5502",
      "6502"
    ],
    description: "Remessa de mercadoria adquirida ou recebida de terceiros, com fim específico de exportação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5503",
    variants: [
      "5503",
      "6503"
    ],
    description: "Devolução de mercadoria recebida com fim específico de exportação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5504",
    variants: [
      "5504",
      "6504"
    ],
    description: "Remessa de mercadoria p/ formação de lote de exportação, de produto industrializado ou produzido pelo próprio estabelecimento.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5505",
    variants: [
      "5505",
      "6505"
    ],
    description: "Remessa de mercadoria, adquirida ou recebida de terceiros, p/ formação de lote de exportação.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5551",
    variants: [
      "5551",
      "6551",
      "7551"
    ],
    description: "Venda de bem do ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5552",
    variants: [
      "5552",
      "6552"
    ],
    description: "Transferência de bem do ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5553",
    variants: [
      "5553",
      "6553",
      "7553"
    ],
    description: "Devolução de compra de bem p/ o ativo imobilizado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5554",
    variants: [
      "5554",
      "6554"
    ],
    description: "Remessa de bem do ativo imobilizado p/ uso fora do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5555",
    variants: [
      "5555",
      "6555"
    ],
    description: "Devolução de bem do ativo imobilizado de terceiro, recebido p/ uso no estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5556",
    variants: [
      "5556",
      "6556",
      "7556"
    ],
    description: "Devolução de compra de material de uso ou consumo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5557",
    variants: [
      "5557",
      "6557"
    ],
    description: "Transferência de material de uso ou consumo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5601",
    variants: [
      "5601"
    ],
    description: "Transferência de crédito de ICMS acumulado",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5602",
    variants: [
      "5602"
    ],
    description: "Transferência de saldo credor do ICMS, p/ outro estabelecimento da mesma empresa, destinado à compensação de saldo devedor do ICMS",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5603",
    variants: [
      "5603",
      "6603"
    ],
    description: "Ressarcimento de ICMS retido por substituição tributária",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5605",
    variants: [
      "5605"
    ],
    description: "Transferência de saldo devedor do ICMS de outro estabelecimento da mesma empresa  ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5606",
    variants: [
      "5606"
    ],
    description: "Utilização de saldo credor do ICMS p/ extinção por compensação de débitos fiscais",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5651",
    variants: [
      "5651",
      "6651",
      "7651"
    ],
    description: "Venda de combustível ou lubrificante de produção do estabelecimento destinados à industrialização subseqüente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5652",
    variants: [
      "5652",
      "6652"
    ],
    description: "Venda de combustível ou lubrificante, de produção do estabelecimento, destinados à comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5653",
    variants: [
      "5653",
      "6653"
    ],
    description: "Venda de combustível ou lubrificante, de produção do estabelecimento, destinados a consumidor ou usuário final",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5654",
    variants: [
      "5654",
      "6654",
      "7654"
    ],
    description: "Venda de combustível ou lubrificante, adquiridos ou recebidos de terceiros, destinados à industrialização subseqüente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5655",
    variants: [
      "5655",
      "6655"
    ],
    description: "Venda de combustível ou lubrificante, adquiridos ou recebidos de terceiros, destinados à comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5656",
    variants: [
      "5656",
      "6656"
    ],
    description: "Venda de combustível ou lubrificante, adquiridos ou recebidos de terceiros, destinados a consumidor ou usuário final",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5657",
    variants: [
      "5657",
      "6657"
    ],
    description: "Remessa de combustível ou lubrificante, adquiridos ou recebidos de terceiros, p/ venda fora do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5658",
    variants: [
      "5658",
      "6658"
    ],
    description: "Transferência de combustível ou lubrificante de produção do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5659",
    variants: [
      "5659",
      "6659"
    ],
    description: "Transferência de combustível ou lubrificante adquiridos ou recebidos de terceiros",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5660",
    variants: [
      "5660",
      "6660"
    ],
    description: "Devolução de compra de combustível ou lubrificante adquiridos p/ industrialização subseqüente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5661",
    variants: [
      "5661",
      "6661"
    ],
    description: "Devolução de compra de combustível ou lubrificante adquiridos p/ comercialização",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5662",
    variants: [
      "5662",
      "6662"
    ],
    description: "Devolução de compra de combustível ou lubrificante adquiridos por consumidor ou usuário final",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5663",
    variants: [
      "5663",
      "6663"
    ],
    description: "Remessa p/ armazenagem de combustível ou lubrificante",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5664",
    variants: [
      "5664",
      "6664"
    ],
    description: "Retorno de combustível ou lubrificante recebidos p/ armazenagem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5665",
    variants: [
      "5665",
      "6665"
    ],
    description: "Retorno simbólico de combustível ou lubrificante recebidos p/ armazenagem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5666",
    variants: [
      "5666",
      "6666"
    ],
    description: "Remessa, por conta e ordem de terceiros, de combustível ou lubrificante recebidos p/ armazenagem",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5667",
    variants: [
      "5667",
      "6667",
      "7667"
    ],
    description: "Venda de combustível ou lubrificante a consumidor ou usuário final estabelecido em outra UF",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5901",
    variants: [
      "5901",
      "6901"
    ],
    description: "Remessa p/ industrialização por encomenda",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5902",
    variants: [
      "5902",
      "6902"
    ],
    description: "Retorno de mercadoria utilizada na industrialização por encomenda",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5903",
    variants: [
      "5903",
      "6903"
    ],
    description: "Retorno de mercadoria recebida p/ industrialização e não aplicada no referido processo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5904",
    variants: [
      "5904",
      "6904"
    ],
    description: "Remessa p/ venda fora do estabelecimento",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5905",
    variants: [
      "5905",
      "6905"
    ],
    description: "Remessa p/ depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5906",
    variants: [
      "5906",
      "6906"
    ],
    description: "Retorno de mercadoria depositada em depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5907",
    variants: [
      "5907",
      "6907"
    ],
    description: "Retorno simbólico de mercadoria depositada em depósito fechado ou armazém geral",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5908",
    variants: [
      "5908",
      "6908"
    ],
    description: "Remessa de bem por conta de contrato de comodato",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5909",
    variants: [
      "5909",
      "6909"
    ],
    description: "Retorno de bem recebido por conta de contrato de comodato",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5910",
    variants: [
      "5910",
      "6910"
    ],
    description: "Remessa em bonificação, doação ou brinde",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5911",
    variants: [
      "5911",
      "6911"
    ],
    description: "Remessa de amostra grátis",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5912",
    variants: [
      "5912",
      "6912"
    ],
    description: "Remessa de mercadoria ou bem p/ demonstração",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5913",
    variants: [
      "5913",
      "6913"
    ],
    description: "Retorno de mercadoria ou bem recebido p/ demonstração",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5914",
    variants: [
      "5914",
      "6914"
    ],
    description: "Remessa de mercadoria ou bem p/ exposição ou feira",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5915",
    variants: [
      "5915",
      "6915"
    ],
    description: "Remessa de mercadoria ou bem p/ conserto ou reparo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5916",
    variants: [
      "5916",
      "6916"
    ],
    description: "Retorno de mercadoria ou bem recebido p/ conserto ou reparo",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5917",
    variants: [
      "5917",
      "6917"
    ],
    description: "Remessa de mercadoria em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5918",
    variants: [
      "5918",
      "6918"
    ],
    description: "Devolução de mercadoria recebida em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5919",
    variants: [
      "5919",
      "6919"
    ],
    description: "Devolução simbólica de mercadoria vendida ou utilizada em processo industrial, recebida anteriormente em consignação mercantil ou industrial",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5920",
    variants: [
      "5920",
      "6920"
    ],
    description: "Remessa de vasilhame ou sacaria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5921",
    variants: [
      "5921",
      "6921"
    ],
    description: "Devolução de vasilhame ou sacaria",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "5922",
    variants: [
      "5922",
      "6922"
    ],
    description: "Lançamento efetuado a título de simples faturamento decorrente de venda p/ entrega futura",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5923",
    variants: [
      "5923",
      "6923"
    ],
    description: "Remessa de mercadoria por conta e ordem de terceiros, em venda à ordem ou em operações com armazém geral ou depósito fechado.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5924",
    variants: [
      "5924",
      "6924"
    ],
    description: "Remessa p/ industrialização por conta e ordem do adquirente da mercadoria, quando esta não transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5925",
    variants: [
      "5925",
      "6925"
    ],
    description: "Retorno de mercadoria recebida p/ industrialização por conta e ordem do adquirente da mercadoria, quando aquela não transitar pelo estabelecimento do adquirente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5926",
    variants: [
      "5926"
    ],
    description: "Lançamento efetuado a título de reclassificação de mercadoria decorrente de formação de kit ou de sua desagregação",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5927",
    variants: [
      "5927"
    ],
    description: "Lançamento efetuado a título de baixa de estoque decorrente de perda, roubo ou deterioração",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5928",
    variants: [
      "5928"
    ],
    description: "Lançamento efetuado a título de baixa de estoque decorrente do encerramento da atividade da empresa",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5929",
    variants: [
      "5929",
      "6929"
    ],
    description: "Lançamento efetuado em decorrência de emissão de documento fiscal relativo a operação ou prestação também registrada em equipamento Emissor de Cupom Fiscal - ECF",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5931",
    variants: [
      "5931",
      "6931"
    ],
    description: "Lançamento efetuado em decorrência da responsabilidade de retenção do imposto por substituição tributária, atribuída ao remetente ou alienante da mercadoria, pelo serviço de transporte realizado por transportador autônomo ou por transportador não inscrito na UF onde iniciado o serviço",
    nfe: true,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5932",
    variants: [
      "5932",
      "6932"
    ],
    description: "Prestação de serviço de transporte iniciada em UF diversa daquela onde inscrito o prestador",
    nfe: true,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "5933",
    variants: [
      "5933",
      "6933"
    ],
    description: "Prestação de serviço tributado pelo Imposto Sobre Serviços de Qualquer Natureza",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5934",
    variants: [
      "5934",
      "6934"
    ],
    description: "Remessa simbólica de mercadoria depositada em armazém geral ou depósito fechado.",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "5949",
    variants: [
      "5949",
      "6949",
      "7949"
    ],
    description: "Outra saída de mercadoria ou prestação de serviço não especificado",
    nfe: true,
    nfce: true,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "6107",
    variants: [
      "6107"
    ],
    description: "Venda de produção do estabelecimento, destinada a não contribuinte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "6108",
    variants: [
      "6108"
    ],
    description: "Venda de mercadoria adquirida ou recebida de terceiros, destinada a não contribuinte",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "6404",
    variants: [
      "6404"
    ],
    description: "Venda de mercadoria sujeita a ST, cujo imposto já tenha sido retido anteriormente",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "7127",
    variants: [
      "7127"
    ],
    description: "Venda de produção do estabelecimento sob o regime de drawback ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  },
  {
    code: "7211",
    variants: [
      "7211"
    ],
    description: "Devolução de compras p/ industrialização sob o regime de drawback ",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "7212",
    variants: [
      "7212"
    ],
    description: "Devolução de compras para industrialização sob o regime de Regime Aduaneiro Especial de Entreposto Industrial (Recof-Sped)",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: true,
    incoming: false
  },
  {
    code: "7358",
    variants: [
      "7358"
    ],
    description: "Prestação de serviço de transporte",
    nfe: false,
    nfce: false,
    cte: true,
    devolution: false,
    incoming: false
  },
  {
    code: "7930",
    variants: [
      "7930"
    ],
    description: "Lançamento efetuado a título de devolução de bem cuja entrada tenha ocorrido sob amparo de regime especial aduaneiro de admissão temporária",
    nfe: true,
    nfce: false,
    cte: false,
    devolution: false,
    incoming: false
  }
]

const ALL_CFOPS_NFCE = ALL_CFOPS.filter(it => it.nfce).map(it => {
  return {
    ...it,
    variants: [it.code]
  }
});
const ALL_CFOPS_NFE = ALL_CFOPS.filter(it => it.nfe);
const ALL_CFOPS_CTE = ALL_CFOPS.filter(it => it.cte);
const ALL_CFOPS_GROUP: Record<CfopCode, CfopEntry> = {} as Record<CfopCode, CfopEntry>
ALL_CFOPS.forEach(it => {
  ALL_CFOPS_GROUP[it.code] = it
})

const displayVariants = (it: CfopEntry): string => {
  return (it.variants ?? [it.code]).join('/');
}

const displayName = (it: CfopEntry) => {
  return `${displayVariants(it)}` + ' - ' + it.description;
}

export const getCfopVariants = (code: CfopCode): string[] => {
  return ALL_CFOPS_GROUP[code].variants;
}

export const getCfopDescription = (code: string): string | null => {
  return ALL_CFOPS_GROUP[code as CfopCode]?.description ?? null
}

const displayCfops = (entries: CfopEntry[]): DisplayCfop[] => {
  return entries.map(it => {
    const value: string = it.code;
    const label = displayName(it);
    return {
      value,
      label,
    }
  });
}

export const getCfopOptionsForNfe = (): DisplayCfop[] => {
  return displayCfops(ALL_CFOPS_NFE);
}

export const getAllCfopOptions = (): DisplayCfop[] => {
  return displayCfops(ALL_CFOPS);
}

export const getCfopOptionsForNfce = (): DisplayCfop[] => {
  return displayCfops(ALL_CFOPS_NFCE);
}

export const getCfopOptionsForCte = (): DisplayCfop[] => {
  return displayCfops(ALL_CFOPS_CTE);
}

// ─── Operation direction (tp_nf) ──────────────────────────────────────────────
// Incoming CFOPs start with 1/2/3 (entrada → tp_nf '0'); outgoing CFOPs start
// with 5/6/7 (saída → tp_nf '1'). Anything else is unknown.

export type CfopDirection = 'in' | 'out'

export const cfopDirection = (cfop: string): CfopDirection | null => {
  switch (cfop?.[0]) {
    case '1':
    case '2':
    case '3':
      return 'in'
    case '5':
    case '6':
    case '7':
      return 'out'
    default:
      return null
  }
}

/** tp_nf for a CFOP: '0' = entrada, '1' = saída. Defaults to '1' when unknown. */
export const cfopTpNf = (cfop: string): '0' | '1' => (cfopDirection(cfop) === 'in' ? '0' : '1')

const NAT_OP_MAX_LEN = 60

const truncateNatOp = (s: string): string =>
  s.length <= NAT_OP_MAX_LEN ? s : `${s.slice(0, NAT_OP_MAX_LEN - 3)}...`

/** First significant word of a CFOP description (e.g. "Venda de mercadoria…" → "Venda"). */
const firstTerm = (desc: string): string => {
  const word = desc.trim().split(/\s+/)[0] ?? ''
  return word.replace(/[.,;:]+$/, '')
}

const joinNatural = (terms: string[]): string => {
  if (terms.length <= 1) return terms[0] ?? ''
  return `${terms.slice(0, -1).join(', ')} e ${terms[terms.length - 1]}`
}

/**
 * Builds the NF-e/NFC-e ide.natOp from the selected CFOPs (max 60 chars).
 * A single CFOP yields its (truncated) description; multiple CFOPs yield one
 * term per distinct CFOP joined naturally (e.g. "Venda e Remessa").
 */
export const buildNatOpFromCfops = (cfops: string[]): string => {
  const distinct = [...new Set(cfops.filter(Boolean))]
  if (distinct.length === 0) return ''
  if (distinct.length === 1) {
    return truncateNatOp(getCfopDescription(distinct[0]) ?? distinct[0])
  }
  const terms = [...new Set(distinct.map((c) => firstTerm(getCfopDescription(c) ?? c)))]
  return truncateNatOp(joinNatural(terms))
}

// ─── CFOP suffix grouping (UF-dynamic selection) ──────────────────────────────
// A saída CFOP is [scope][suffix]: scope '5' = intra-UF, '6' = inter-UF,
// '7' = exterior. The 3-digit suffix is the fiscal nature, shared across the
// intra/inter variants (e.g. 5920 and 6920 are both nature "920").

/** Scope digit of a CFOP ('5' intra-UF, '6' inter-UF, '7' exterior). */
export const cfopScope = (cfop: string): string => cfop.charAt(0)

/** Fiscal-nature suffix (last 3 digits), shared across intra/inter variants. */
export const cfopSuffix = (cfop: string): string => cfop.slice(1)

export interface CfopSuffixGroup {
  suffix: string
  intra?: string   // 5xxx member
  inter?: string   // 6xxx member
  label: string    // nature description (from getCfopDescription)
}

/** Groups a product's cfop_config entries by fiscal-nature suffix. */
export const groupCfopConfigBySuffix = (config: CfopConfigItem[]): CfopSuffixGroup[] => {
  const bySuffix = new Map<string, CfopSuffixGroup>()
  for (const item of config) {
    const cfop = item.cfop
    if (!cfop) continue
    const suffix = cfopSuffix(cfop)
    const group = bySuffix.get(suffix) ?? {suffix, label: ''}
    if (cfopScope(cfop) === '6') group.inter = cfop
    else group.intra = cfop
    // Prefer a description from whichever variant resolves one.
    if (!group.label) group.label = getCfopDescription(cfop) ?? ''
    bySuffix.set(suffix, group)
  }
  return [...bySuffix.values()]
}

/** Escopo do destino: '5' dentro da UF, '6' outra UF, '7' exterior. */
export const CFOP_SCOPE_INTRA_UF = '5'
export const CFOP_SCOPE_INTER_UF = '6'
export const CFOP_SCOPE_FOREIGN = '7'
/** UF que representa destino no exterior. */
export const UF_FOREIGN = 'EX'

/**
 * Monta o CFOP concreto a partir da natureza fiscal e das UFs.
 *
 * A **fonte da verdade** desta regra é `services.ResolveCFOPScope`
 * (api/internal/services/cfop.go); esta cópia existe para o dropdown resolver o
 * escopo sem ida ao servidor. O teste de paridade roda sobre a mesma tabela de
 * casos (api/internal/services/testdata/cfop_scope_cases.json) — divergir é
 * falha de build.
 *
 * Devolve null quando a entrada é inválida; quem chama bloqueia a emissão.
 */
export const resolveCfopScope = (suffix: string, emitUf: string, destUf: string): string | null => {
  const nature = suffix.trim()
  if (!/^\d{3}$/.test(nature)) return null
  const emit = emitUf.trim().toUpperCase()
  const dest = destUf.trim().toUpperCase()
  if (!emit || !dest) return null
  if (dest === UF_FOREIGN) return CFOP_SCOPE_FOREIGN + nature
  return (emit === dest ? CFOP_SCOPE_INTRA_UF : CFOP_SCOPE_INTER_UF) + nature
}

/**
 * Resolves the concrete CFOP for a group given whether the recipient is in the
 * issuer's UF. Returns null when the required-scope variant is not configured
 * (caller must block emission).
 */
export const resolveCfopForUf = (group: CfopSuffixGroup, sameUf: boolean): string | null =>
  (sameUf ? group.intra : group.inter) ?? null

/**
 * Concrete CFOP codes of a group joined for display, intra (5xxx) first
 * (e.g. "5920/6920", or just "5405" when only one variant is configured).
 */
export const cfopGroupCodes = (group: CfopSuffixGroup): string =>
  [group.intra, group.inter].filter(Boolean).join('/')