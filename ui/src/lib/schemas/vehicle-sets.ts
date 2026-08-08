/**
 * Composição veicular — trator, até 3 reboques e condutores escolhidos de uma
 * vez. Espelha VehicleSetBody (api/internal/api/v1/dto.go).
 */
import {z} from 'zod'

/** Teto de reboques do leiaute do MDF-e. */
export const MAX_TRAILERS = 3

export const vehicleSetSchema = z.object({
  name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
  tractor_sk: z.string().min(1, 'Escolha o veículo de tração'),
  trailer_sks: z.array(z.string()).max(MAX_TRAILERS, `Máximo ${MAX_TRAILERS} reboques`),
  driver_docs: z.array(z.string()),
  rntrc: z.string().regex(/^\d{8,12}$/, 'RNTRC: 8 a 12 dígitos').optional().or(z.literal('')),
  ciot: z.string().max(20).optional().or(z.literal('')),
})

export type VehicleSetFormData = z.infer<typeof vehicleSetSchema>
