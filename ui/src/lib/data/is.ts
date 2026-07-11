// IS — Imposto Seletivo (NT 2024.001 / Reforma Tributária)
export const IS_CST_OPTIONS = [
  {value: '000', label: '000 – Tributada'},
  {value: '010', label: '010 – Suspensão'},
  {value: '020', label: '020 – Imunidade'},
  {value: '030', label: '030 – Não tributada'},
  {value: '040', label: '040 – Isenção'},
  {value: '999', label: '999 – Outras'},
]

// ICMS motivos de desoneração — usados nos CSTs 40, 41, 50 e 51
export const ICMS_MOT_DESONE_OPTIONS = [
  {value: '1', label: '1 – Táxi'},
  {value: '2', label: '2 – Deficiente físico ou mental'},
  {value: '3', label: '3 – Produtor agropecuário'},
  {value: '4', label: '4 – Frotista / locadora'},
  {value: '5', label: '5 – Diplomático / consular'},
  {value: '6', label: '6 – Utilitários e motocicletas (Amazônia)'},
  {value: '7', label: '7 – SUFRAMA'},
  {value: '8', label: '8 – Venda para órgão público'},
  {value: '9', label: '9 – Outros'},
  {value: '10', label: '10 – Deficiente condutor (Lei 8.989/95)'},
  {value: '11', label: '11 – Deficiente não condutor (Lei 8.989/95)'},
  {value: '16', label: '16 – Olimpíadas / Paraolimpíadas 2016'},
  {value: '90', label: '90 – Solicitado pelo fisco'},
]
