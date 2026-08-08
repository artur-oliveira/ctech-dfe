/**
 * Perfil fiscal — um tratamento tributário nomeado, aplicado a um conjunto de
 * CFOPs e reutilizado por vários produtos. Espelha TaxProfileBody
 * (api/internal/api/v1/dto.go): os campos de tributação são exatamente os de
 * `cfopConfigSchema` menos o CFOP, porque no perfil os CFOPs são uma lista à
 * parte.
 */
import {z} from 'zod'
import {cfopConfigSchema} from '@/lib/schemas/products'

export const taxProfileSchema = cfopConfigSchema
  .omit({cfop: true})
  .extend({
    name: z.string().min(2, 'Mínimo 2 caracteres').max(120),
    description: z.string().max(255).optional().or(z.literal('')),
    cfops: z.array(z.string().regex(/^\d{4}$/, 'CFOP deve ter 4 dígitos'))
      .min(1, 'Escolha ao menos um CFOP'),
  })

export type TaxProfileFormData = z.infer<typeof taxProfileSchema>
