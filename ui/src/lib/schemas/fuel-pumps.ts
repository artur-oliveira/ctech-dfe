/**
 * Bico, bomba e tanque do posto (NF-e prod/comb/encerrante). Espelha
 * FuelPumpBody (api/internal/api/v1/dto.go).
 *
 * A leitura do encerrante (`last_v_enc_fin`) não está aqui de propósito: quem a
 * escreve é a emissão, na mesma transação que reserva o número da nota. O
 * `vEncIni` de cada venda é o `vEncFin` da venda anterior da mesma bomba.
 */
import {z} from 'zod'

const numero3 = /^\d{1,3}$/

export const fuelPumpSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  n_bico: z.string().regex(numero3, 'Número do bico: até 3 dígitos'),
  n_bomba: z.string().regex(numero3, 'Número da bomba: até 3 dígitos').optional().or(z.literal('')),
  n_tanque: z.string().regex(numero3, 'Número do tanque: até 3 dígitos').optional().or(z.literal('')),
})

export type FuelPumpFormData = z.infer<typeof fuelPumpSchema>
