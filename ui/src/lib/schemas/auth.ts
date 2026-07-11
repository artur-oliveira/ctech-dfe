import {z} from 'zod'

export const loginSchema = z.object({
  email: z
    .email('Email inválido')
    .min(1, 'Email é obrigatório')
    .max(320, 'Email deve possuir no máximo 320 caracteres'),
  password: z
    .string()
    .min(6, 'Senha deve ter no mínimo 6 caracteres')
    .min(1, 'Senha é obrigatória'),
})

export type LoginFormData = z.infer<typeof loginSchema>

export const registerSchema = z.object({
  email: z
    .email('Email inválido')
    .min(1, 'Email é obrigatório'),
  username: z
    .string()
    .min(3, 'Usuário deve ter no mínimo 3 caracteres')
    .max(30, 'Usuário deve ter no máximo 30 caracteres')
    .regex(/^[a-z0-9_.]+$/, 'Usuário pode conter apenas letras minúsculas, números, underscore e ponto')
    .min(1, 'Usuário é obrigatório'),
  password: z
    .string()
    .min(8, 'Senha deve ter no mínimo 8 caracteres')
    .regex(/[A-Z]/, 'Senha deve conter pelo menos uma letra maiúscula')
    .regex(/[0-9]/, 'Senha deve conter pelo menos um número')
    .min(1, 'Senha é obrigatória'),
  first_name: z
    .string()
    .min(1, 'Nome é obrigatório')
    .max(50, 'Nome deve ter no máximo 50 caracteres'),
  last_name: z
    .string()
    .min(1, 'Sobrenome é obrigatório')
    .max(50, 'Sobrenome deve ter no máximo 50 caracteres'),
})

export type RegisterFormData = z.infer<typeof registerSchema>
