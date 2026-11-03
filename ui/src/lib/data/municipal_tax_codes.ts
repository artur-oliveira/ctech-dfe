import {NFSE_TRIB_NACIONAL} from '@/lib/data/nfse_trib_nacional';
import {MUNICIPAL_TAX_CATALOG} from '@/lib/data/municipal_tax_codes_data';
import {TERESINA_TAX_ROWS} from '@/lib/data/teresina_tax_codes_data';

export interface MunicipalTaxCode {
  municipalityCode: string
  nationalItem: string
  /** cTribNac de 6 dígitos; só existe quando o catálogo é oficial e o cTribMun depende dele. */
  nationalCode?: string
  municipalCode: string
  description: string
  taxRate: number | null
}

export const TERESINA_IBGE_CODE = '2211001';

// Teresina vem da planilha oficial da SEMF (cTribMun = 4º segmento do código completo,
// repetido entre cTribNac diferentes). Os demais municípios vêm de catálogo raspado
// (scripts/generate-municipal-tax-codes.mjs), com código único por subitem da LC 116.

function nationalDescription(itemCode: string): string {
  const [item, subitem] = itemCode.split('.').map(Number);
  return NFSE_TRIB_NACIONAL.find((entry) =>
    Number(entry.item) === item && Number(entry.subitem) === subitem,
  )?.description ?? `Serviço do subitem ${itemCode}`;
}

function buildTeresinaCodes(): readonly MunicipalTaxCode[] {
  return TERESINA_TAX_ROWS.map(([nationalCode, municipalCode, taxRate, description]) => {
    const nationalItem = `${nationalCode.slice(0, 2)}.${nationalCode.slice(2, 4)}`;
    return {
      municipalityCode: TERESINA_IBGE_CODE, nationalItem, nationalCode, municipalCode, taxRate,
      description: description || nationalDescription(nationalItem),
    };
  });
}

function buildCodes(municipalityCode: string): readonly MunicipalTaxCode[] {
  if (municipalityCode === TERESINA_IBGE_CODE) return buildTeresinaCodes();
  const seen = new Set<string>();
  return (MUNICIPAL_TAX_CATALOG[municipalityCode] ?? []).flatMap(([nationalItem, municipalCode, taxRate]) => {
    if (seen.has(municipalCode)) return []; // vários itens LC 116 podem dividir o mesmo código
    seen.add(municipalCode);
    return [{municipalityCode, nationalItem, municipalCode, taxRate, description: nationalDescription(nationalItem)}];
  });
}

export const TERESINA_MUNICIPAL_TAX_CODES: readonly MunicipalTaxCode[] = buildCodes(TERESINA_IBGE_CODE);

const cache = new Map<string, readonly MunicipalTaxCode[]>();

export function hasMunicipalTaxCatalog(municipalityCode?: string): boolean {
  return !!municipalityCode && (municipalityCode === TERESINA_IBGE_CODE || municipalityCode in MUNICIPAL_TAX_CATALOG);
}

/**
 * Códigos do município. Em catálogo oficial (com `nationalCode`) o cTribMun só vale junto do
 * cTribNac, então sem `nationalCode` informado a lista vem vazia.
 */
export function getMunicipalTaxCodes(municipalityCode?: string, nationalCode?: string): readonly MunicipalTaxCode[] {
  if (!municipalityCode || !hasMunicipalTaxCatalog(municipalityCode)) return [];
  if (!cache.has(municipalityCode)) cache.set(municipalityCode, buildCodes(municipalityCode));
  const codes = cache.get(municipalityCode) ?? [];
  return codes.filter((entry) => !entry.nationalCode || entry.nationalCode === nationalCode);
}
