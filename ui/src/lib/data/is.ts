// IS — Imposto Seletivo (NT 2024.001 / Reforma Tributária)
export const IS_CST_OPTIONS = [
  {value: '000', label: '000 · Tributada'},
  {value: '010', label: '010 · Suspensão'},
  {value: '020', label: '020 · Imunidade'},
  {value: '030', label: '030 · Não tributada'},
  {value: '040', label: '040 · Isenção'},
  {value: '999', label: '999 · Outras'},
];

// Motivos de desoneração (motDesICMS) e as enumerações do XSD por CST. Cada grupo do leiaute aceita
// só o seu subconjunto; o 2 não existe em nenhuma enumeração.
export const ICMS_MOT_DESONE_OPTIONS = [
  {value: '1', label: '1 · Táxi'},
  {value: '3', label: '3 · Produtor agropecuário (uso na agropecuária)'},
  {value: '4', label: '4 · Frotista / locadora'},
  {value: '5', label: '5 · Diplomático / consular'},
  {value: '6', label: '6 · Utilitários e motocicletas (Amazônia Ocidental e ALC)'},
  {value: '7', label: '7 · SUFRAMA'},
  {value: '8', label: '8 · Venda para órgão público'},
  {value: '9', label: '9 · Outros'},
  {value: '10', label: '10 · Deficiente condutor'},
  {value: '11', label: '11 · Deficiente não condutor'},
  {value: '12', label: '12 · Fomento agropecuário'},
  {value: '16', label: '16 · Olimpíadas Rio 2016'},
  {value: '90', label: '90 · Solicitado pelo fisco'},
];

/** motDesICMS por CST (XSD): ICMS20, ICMS30, ICMS40 (40, 41, 50), ICMS70 e ICMS90. */
export const ICMS_MOT_DES_BY_CST: Record<string, string[]> = {
  '20': ['3', '9', '10', '11', '12'],
  '30': ['6', '7', '9'],
  '40': ['1', '3', '4', '5', '6', '7', '8', '9', '10', '11', '16', '90'],
  '41': ['1', '3', '4', '5', '6', '7', '8', '9', '10', '11', '16', '90'],
  '50': ['1', '3', '4', '5', '6', '7', '8', '9', '10', '11', '16', '90'],
  '70': ['3', '9', '12'],
  '90': ['3', '9', '12'],
};

/** motDesICMSST (ICMS10, ICMS70, ICMS90): 3, 9 e 12. */
export const ICMS_MOT_DES_ST_BY_CST: Record<string, string[]> = {
  '10': ['3', '9', '12'],
  '70': ['3', '9', '12'],
  '90': ['3', '9', '12'],
};

const pick = (codes: string[] | undefined) =>
  ICMS_MOT_DESONE_OPTIONS.filter((o) => (codes ?? []).includes(o.value));

export const icmsMotDesOptions = (cst: string) => pick(ICMS_MOT_DES_BY_CST[cst]);
export const icmsMotDesStOptions = (cst: string) => pick(ICMS_MOT_DES_ST_BY_CST[cst]);
