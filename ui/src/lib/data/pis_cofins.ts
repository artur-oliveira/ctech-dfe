// Rótulos do XSD (leiauteNFe, PIS/COFINS CST). O 03 é PISQtde/COFINSQtde; o 05 é PISNT/COFINSNT.
// Os CSTs 50 a 75 e 98 de PISOutr ficam de fora: o editor ainda não tem os campos deles.
export const PIS_COFINS_OPTIONS = [
  {value: '01', label: '01 · Tributável (BC = valor da operação, alíquota normal)'},
  {value: '02', label: '02 · Tributável (BC = valor da operação, alíquota diferenciada)'},
  {value: '03', label: '03 · Tributável (BC = quantidade vendida x alíquota por unidade)'},
  {value: '04', label: '04 · Tributável (monofásica, alíquota zero)'},
  {value: '05', label: '05 · Tributável (substituição tributária)'},
  {value: '06', label: '06 · Tributável (alíquota zero)'},
  {value: '07', label: '07 · Isenta da contribuição'},
  {value: '08', label: '08 · Sem incidência da contribuição'},
  {value: '09', label: '09 · Com suspensão da contribuição'},
  {value: '49', label: '49 · Outras operações de saída'},
  {value: '99', label: '99 · Outras operações'},
];
