import {NFSE_TRIB_NACIONAL} from '@/lib/data/nfse_trib_nacional';
import {MUNICIPAL_TAX_CATALOG} from '@/lib/data/municipal_tax_codes_data';

export interface MunicipalTaxCode {
  municipalityCode: string
  nationalItem: string
  municipalCode: string
  description: string
  taxRate: number | null
}

export const TERESINA_IBGE_CODE = '2211001';

// Catálogo gerado por scripts/generate-municipal-tax-codes.mjs: código municipal
// de tributação e alíquota por município (IBGE). O código vale apenas para o
// município emissor da organização (c_loc_emi); a descrição vem da tabela
// nacional já versionada no projeto.

function nationalDescription(itemCode: string): string {
  const [item, subitem] = itemCode.split('.').map(Number);
  return NFSE_TRIB_NACIONAL.find((entry) =>
    Number(entry.item) === item && Number(entry.subitem) === subitem,
  )?.description ?? `Serviço do subitem ${itemCode}`;
}

function buildCodes(municipalityCode: string): readonly MunicipalTaxCode[] {
  const seen = new Set<string>();
  return (MUNICIPAL_TAX_CATALOG[municipalityCode] ?? []).flatMap(([nationalItem, municipalCode, taxRate]) => {
    if (seen.has(municipalCode)) return []; // vários itens LC 116 podem dividir o mesmo código
    seen.add(municipalCode);
    return [{municipalityCode, nationalItem, municipalCode, description: nationalDescription(nationalItem), taxRate}];
  });
}

export const TERESINA_MUNICIPAL_TAX_CODES: readonly MunicipalTaxCode[] = buildCodes(TERESINA_IBGE_CODE);

const cache = new Map<string, readonly MunicipalTaxCode[]>();

export function getMunicipalTaxCodes(municipalityCode?: string): readonly MunicipalTaxCode[] {
  if (!municipalityCode || !(municipalityCode in MUNICIPAL_TAX_CATALOG)) return [];
  if (!cache.has(municipalityCode)) cache.set(municipalityCode, buildCodes(municipalityCode));
  return cache.get(municipalityCode) ?? [];
}
