/**
 * Pares válidos de tipo e espécie de veículo (`veicProd/tpVeic` e `espVeic`),
 * segundo a tabela do Portal Nacional da NF-e ("Tabela de Tipo e Espécie de
 * Veículo", registros vigentes).
 *
 * Não é o produto cartesiano: a SEFAZ publica quais espécies existem para cada
 * tipo. Um "motoneta de carga" existe; um "caminhão de passageiro" não — e essa
 * combinação, hoje escolhida em dois selects independentes, só era recusada na
 * emissão.
 */

export interface VehicleTypePair {
  tpVeic: string
  espVeic: string
  description: string
}

export const VEHICLE_TYPE_PAIRS: readonly VehicleTypePair[] = [
  {tpVeic: "2", espVeic: "1", description: "2-Ciclomotor; 1-Passageiro"},
  {tpVeic: "3", espVeic: "1", description: "3-Motoneta; 1-Passageiro"},
  {tpVeic: "3", espVeic: "2", description: "3-Motoneta; 2-Carga"},
  {tpVeic: "4", espVeic: "1", description: "4-Motocicleta; 1-Passageiro"},
  {tpVeic: "4", espVeic: "2", description: "4-Motocicleta; 2-Carga"},
  {tpVeic: "4", espVeic: "6", description: "4-Motocicleta; 6-Especial"},
  {tpVeic: "5", espVeic: "1", description: "5-Triciclo; 1-Passageiro"},
  {tpVeic: "5", espVeic: "2", description: "5-Triciclo; 2-Carga"},
  {tpVeic: "5", espVeic: "6", description: "5-Triciclo; 6-Especial"},
  {tpVeic: "6", espVeic: "1", description: "6-Automóvel; 1-Passageiro"},
  {tpVeic: "6", espVeic: "6", description: "6-Automóvel; 6-Especial"},
  {tpVeic: "7", espVeic: "1", description: "7-Micro-Ônibus; 1-Passageiro"},
  {tpVeic: "7", espVeic: "6", description: "7-Micro-Ônibus; 6-Especial"},
  {tpVeic: "8", espVeic: "1", description: "8-Ônibus; 1-Passageiro"},
  {tpVeic: "8", espVeic: "6", description: "8-Ônibus; 6-Especial"},
  {tpVeic: "10", espVeic: "1", description: "10-Reboque; 1-Passageiro"},
  {tpVeic: "10", espVeic: "2", description: "10-Reboque; 2-Carga"},
  {tpVeic: "10", espVeic: "6", description: "10-Reboque; 6-Especial"},
  {tpVeic: "11", espVeic: "1", description: "11-Semirreboque; 1-Passageiro"},
  {tpVeic: "11", espVeic: "2", description: "11-Semirreboque; 2-Carga"},
  {tpVeic: "11", espVeic: "6", description: "11-Semirreboque; 6-Especial"},
  {tpVeic: "13", espVeic: "3", description: "13-Camioneta; 3-Misto"},
  {tpVeic: "13", espVeic: "6", description: "13-Camioneta; 6-Especial"},
  {tpVeic: "14", espVeic: "2", description: "14-Caminhão; 2-Carga"},
  {tpVeic: "14", espVeic: "6", description: "14-Caminhão; 6-Especial"},
  {tpVeic: "17", espVeic: "5", description: "17-Caminhão Trator; 5-Tração"},
  {tpVeic: "17", espVeic: "6", description: "17-Caminhão Trator; 6-Especial"},
  {tpVeic: "18", espVeic: "5", description: "18-Tr Rodas; 5-Tração"},
  {tpVeic: "19", espVeic: "5", description: "19-Tr Esteiras; 5-Tração"},
  {tpVeic: "20", espVeic: "5", description: "20-Tr Mistos; 5-Tração"},
  {tpVeic: "21", espVeic: "1", description: "21-Quadriciclo; 1-Passageiro"},
  {tpVeic: "21", espVeic: "2", description: "21-Quadriciclo; 2-Carga"},
  {tpVeic: "22", espVeic: "1", description: "22-Chassi Plataforma; 1-Passageiro"},
  {tpVeic: "22", espVeic: "6", description: "22-Chassi Plataforma; 6-Especial"},
  {tpVeic: "23", espVeic: "2", description: "23-Caminhonete; 2-Carga"},
  {tpVeic: "23", espVeic: "6", description: "23-Caminhonete; 6-Especial"},
  {tpVeic: "25", espVeic: "3", description: "25-Utilitário; 3-Misto"},
  {tpVeic: "25", espVeic: "6", description: "25-Utilitário; 6-Especial"},
  {tpVeic: "26", espVeic: "6", description: "26-Motor-Casa; 6-Especial"},
]

/** Espécies válidas para o tipo escolhido, na ordem da tabela oficial. */
export function especieOptionsForTipo(tpVeic?: string | null): { value: string; label: string }[] {
  if (!tpVeic) return []
  return VEHICLE_TYPE_PAIRS
    .filter((p) => p.tpVeic === tpVeic)
    .map((p) => ({value: p.espVeic, label: p.description.split(';').pop()?.trim() ?? p.espVeic}))
}

export function isValidVehicleTypePair(tpVeic?: string | null, espVeic?: string | null): boolean {
  if (!tpVeic || !espVeic) return true
  return VEHICLE_TYPE_PAIRS.some((p) => p.tpVeic === tpVeic && p.espVeic === espVeic)
}
