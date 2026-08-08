import {NFSE_TRIB_NACIONAL} from '@/lib/data/nfse_trib_nacional'

export interface MunicipalTaxCode {
  municipalityCode: string
  nationalItem: string
  municipalCode: string
  description: string
  taxRate: number
}

export const TERESINA_IBGE_CODE = '2211001'

// Subitens publicados pela Prefeitura de Teresina para o código municipal de
// tributação. O valor enviado ao DPS é a concatenação sem pontuação (04.15 →
// 415); a descrição vem da tabela nacional já versionada no projeto.
const TERESINA_NATIONAL_ITEMS = `
01.01 01.02 01.03 01.04 01.05 01.06 01.07 01.08 01.09
02.01
03.02 03.03 03.04 03.05
04.01 04.02 04.03 04.04 04.05 04.06 04.07 04.08 04.09 04.10 04.11 04.12 04.13 04.14 04.15 04.16 04.17 04.19 04.20 04.22 04.23
05.01 05.02 05.03 05.04 05.05 05.06 05.07 05.08 05.09
06.01 06.02 06.03 06.04 06.05 06.06
07.01 07.02 07.03 07.04 07.05 07.06 07.07 07.08 07.09 07.10 07.11 07.12 07.13 07.16 07.17 07.18 07.19 07.20 07.21 07.22
08.01 08.02
09.01 09.02 09.03
10.01 10.02 10.03 10.04 10.05 10.06 10.07 10.08 10.09 10.10
11.01 11.02 11.03 11.04
12.01 12.02 12.03 12.04 12.05 12.06 12.07 12.08 12.09 12.10 12.11 12.12 12.13 12.14 12.15 12.16 12.17
13.02 13.03 13.04 13.05
14.01 14.02 14.03 14.04 14.05 14.06 14.07 14.08 14.09 14.10 14.11 14.12 14.13 14.14
15.01 15.02 15.03 15.04 15.05 15.06 15.07 15.08 15.09 15.10 15.11 15.12 15.13 15.14 15.15 15.16 15.17 15.18
16.01 16.02
17.01 17.02 17.03 17.04 17.05 17.06 17.08 17.09 17.10 17.11 17.12 17.13 17.14 17.15 17.16 17.17 17.18 17.19 17.20 17.21 17.22 17.23 17.24 17.25
18.01 19.01 20.01 20.02 20.03 21.01 22.01 23.01 24.01 25.01 25.02 25.03 25.04 25.05 26.01 27.01 28.01 29.01 30.01 31.01 32.01 33.01 34.01 35.01 36.01 37.01 38.01 39.01 40.01
`.trim().split(/\s+/)

const TERESINA_RATE_TWO = new Set(['16.02'])
const TERESINA_RATE_THREE = new Set([
  '04.01', '04.02', '04.03', '04.04', '04.05', '04.06', '04.07', '04.08', '04.09', '04.10',
  '04.11', '04.12', '04.13', '04.14', '04.15', '04.16', '04.17', '04.19', '04.20', '04.22', '04.23',
  '07.01', '07.02', '07.03', '07.04', '07.05', '07.06', '07.07', '07.08', '07.09', '07.10',
  '07.11', '07.12', '07.13', '07.17', '07.18', '07.19', '07.20', '07.21', '07.22',
  '08.01', '08.02', '09.01', '16.01',
])
const TERESINA_RATE_FOUR = new Set([
  '10.01', '10.02', '10.03', '10.04', '10.05', '10.06', '10.07', '10.08', '10.09', '10.10',
  '20.01', '20.02', '20.03', '25.01', '25.02', '25.03', '25.04', '26.01',
])
const DEFAULT_TERESINA_RATE = 5

function teresinaRate(item: string): number {
  if (TERESINA_RATE_TWO.has(item)) return 2
  if (TERESINA_RATE_THREE.has(item)) return 3
  if (TERESINA_RATE_FOUR.has(item)) return 4
  return DEFAULT_TERESINA_RATE
}

function nationalDescription(itemCode: string): string {
  const [item, subitem] = itemCode.split('.').map(Number)
  return NFSE_TRIB_NACIONAL.find((entry) =>
    Number(entry.item) === item && Number(entry.subitem) === subitem,
  )?.description ?? `Serviço do subitem ${itemCode}`
}

export const TERESINA_MUNICIPAL_TAX_CODES: readonly MunicipalTaxCode[] = TERESINA_NATIONAL_ITEMS.map((nationalItem) => ({
  municipalityCode: TERESINA_IBGE_CODE,
  nationalItem,
  municipalCode: String(Number(nationalItem.replace('.', ''))),
  description: nationalDescription(nationalItem),
  taxRate: teresinaRate(nationalItem),
}))

const MUNICIPAL_TAX_CODES: Readonly<Record<string, readonly MunicipalTaxCode[]>> = {
  [TERESINA_IBGE_CODE]: TERESINA_MUNICIPAL_TAX_CODES,
}

export function getMunicipalTaxCodes(municipalityCode?: string): readonly MunicipalTaxCode[] {
  return municipalityCode ? (MUNICIPAL_TAX_CODES[municipalityCode] ?? []) : []
}
