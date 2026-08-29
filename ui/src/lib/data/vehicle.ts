// Tabelas de domínio do grupo veicProd (veículos novos) da NF-e.
//
// Fonte de tpVeic e espVeic: DENATRAN/SERPRO, "Pré-cadastro de Veículos por
// Lotes de Registros", §6.5 Tabela de Tipo de Veículo e §6.6 Tabela de Espécie.
// O XSD não enumera nenhum dos dois (tpVeic é [0-9]{1,2} e espVeic é [0-9]{1}),
// mas a SEFAZ valida o par contra a tabela publicada (rejeição 843), então
// digitar o código à mão é errar por conta própria.

export const VEIC_TP_OP_OPTIONS = [
  {value: '0', label: '0 – Outros'},
  {value: '1', label: '1 – Venda concessionária'},
  {value: '2', label: '2 – Faturamento direto'},
  {value: '3', label: '3 – Venda direta'},
]

export const VEIC_TP_COMB_OPTIONS = [
  {value: '01', label: '01 – Álcool'},
  {value: '02', label: '02 – Gasolina'},
  {value: '03', label: '03 – Diesel'},
  {value: '16', label: '16 – Álcool/Gasolina'},
  {value: '17', label: '17 – Gasolina/Álcool/GNV'},
  {value: '18', label: '18 – Gasolina/Elétrico'},
]

export const VEIC_COND_OPTIONS = [
  {value: '1', label: '1 – Acabado'},
  {value: '2', label: '2 – Inacabado'},
  {value: '3', label: '3 – Semi-acabado'},
]

export const VEIC_TP_REST_OPTIONS = [
  {value: '0', label: '0 – Sem restrição'},
  {value: '1', label: '1 – Alienação Fiduciária'},
  {value: '2', label: '2 – Arrendamento Mercantil'},
  {value: '3', label: '3 – Reserva de Domínio'},
  {value: '4', label: '4 – Penhor de Veículos'},
  {value: '9', label: '9 – Outras'},
]

export const VEIC_VIN_OPTIONS = [
  {value: 'N', label: 'N – Normal'},
  {value: 'R', label: 'R – Remarcado'},
]

export const VEIC_COR_DENATRAN_OPTIONS = [
  {value: '01', label: '01 – Amarelo'}, {value: '02', label: '02 – Azul'},
  {value: '03', label: '03 – Bege'}, {value: '04', label: '04 – Branca'},
  {value: '05', label: '05 – Cinza'}, {value: '06', label: '06 – Dourada'},
  {value: '07', label: '07 – Grena'}, {value: '08', label: '08 – Laranja'},
  {value: '09', label: '09 – Marrom'}, {value: '10', label: '10 – Prata'},
  {value: '11', label: '11 – Preta'}, {value: '12', label: '12 – Rosa'},
  {value: '13', label: '13 – Roxa'}, {value: '14', label: '14 – Verde'},
  {value: '15', label: '15 – Vermelha'}, {value: '16', label: '16 – Fantasia'},
]

/** Tabela de Tipo de Veículo do RENAVAM (§6.5). */
export const VEIC_TP_VEIC_OPTIONS = [
  {value: '01', label: '01 – Bicicleta'},
  {value: '02', label: '02 – Ciclomotor'},
  {value: '03', label: '03 – Motoneta'},
  {value: '04', label: '04 – Motocicleta'},
  {value: '05', label: '05 – Triciclo'},
  {value: '06', label: '06 – Automóvel'},
  {value: '07', label: '07 – Micro-ônibus'},
  {value: '08', label: '08 – Ônibus'},
  {value: '09', label: '09 – Bonde'},
  {value: '10', label: '10 – Reboque'},
  {value: '11', label: '11 – Semi-reboque'},
  {value: '12', label: '12 – Charrete'},
  {value: '13', label: '13 – Camioneta'},
  {value: '14', label: '14 – Caminhão'},
  {value: '15', label: '15 – Carroça'},
  {value: '16', label: '16 – Carro de mão'},
  {value: '17', label: '17 – Caminhão trator'},
  {value: '18', label: '18 – Trator de rodas'},
  {value: '19', label: '19 – Trator de esteiras'},
  {value: '20', label: '20 – Trator misto'},
  {value: '21', label: '21 – Quadriciclo'},
  {value: '22', label: '22 – Chassi/plataforma'},
  {value: '23', label: '23 – Caminhonete'},
  {value: '24', label: '24 – Sidecar'},
  {value: '25', label: '25 – Utilitário'},
  {value: '26', label: '26 – Motor-casa'},
]

/** Tabela de Espécie do RENAVAM (§6.6). O XSD aceita um dígito só. */
export const VEIC_ESP_VEIC_OPTIONS = [
  {value: '1', label: '1 – Passageiro'},
  {value: '2', label: '2 – Carga'},
  {value: '3', label: '3 – Misto'},
  {value: '4', label: '4 – Corrida'},
  {value: '5', label: '5 – Tração'},
  {value: '6', label: '6 – Especial'},
  {value: '7', label: '7 – Coleção'},
]

/**
 * tpPint não é enumerado no XSD nem tem regra de validação própria na SEFAZ; o
 * domínio de fato é o do acabamento de fábrica, e é o que os emissores aceitam.
 */
export const VEIC_TP_PINT_OPTIONS = [
  {value: 'S', label: 'S – Sólida'},
  {value: 'M', label: 'M – Metálica'},
  {value: 'P', label: 'P – Perolizada'},
  {value: 'F', label: 'F – Fosca'},
]

/**
 * Anos de modelo e de fabricação aceitos: o modelo vai até o ano seguinte (é
 * assim que a indústria fatura), e a fabricação nunca é futura. Um select
 * fecha o campo — 1899 digitado é rejeição 610.
 */
export function vehicleYearOptions(now: Date = new Date()): { value: string; label: string }[] {
  const current = now.getFullYear()
  const years: string[] = []
  for (let y = current + 1; y >= current - 5; y--) years.push(String(y))
  return years.map((y) => ({value: y, label: y}))
}
