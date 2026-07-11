// Entradas (00–49) são menos comuns no cadastro de produto, mas incluídas para completude
export const IPI_CST_OPTIONS = [
  {value: '00', label: '00 – Entrada com recuperação de crédito',},
  {value: '01', label: '01 – Entrada tributável com alíquota zero',},
  {value: '02', label: '02 – Entrada isenta',},
  {value: '03', label: '03 – Entrada não-tributada',},
  {value: '04', label: '04 – Entrada imune',},
  {value: '05', label: '05 – Entrada com suspensão',},
  {value: '49', label: '49 – Outras entradas',},
  {value: '50', label: '50 – Saída tributada',},
  {value: '51', label: '51 – Saída tributável com alíquota zero',},
  {value: '52', label: '52 – Saída isenta',},
  {value: '53', label: '53 – Saída não-tributada',},
  {value: '54', label: '54 – Saída imune',},
  {value: '55', label: '55 – Saída com suspensão',},
  {value: '99', label: '99 – Outras saídas',},
]

// Separados por direção para facilitar filtragem contextual
export const IPI_CST_ENTRADA = IPI_CST_OPTIONS.filter((o) => parseInt(o.value) < 50)
export const IPI_CST_SAIDA = IPI_CST_OPTIONS.filter((o) => parseInt(o.value) >= 50)
