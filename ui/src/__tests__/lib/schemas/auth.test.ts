import { describe, it, expect } from 'vitest'
import { loginSchema, registerSchema } from '@/lib/schemas/auth'

describe('loginSchema', () => {
  it('aceita credenciais válidas', () => {
    const result = loginSchema.safeParse({ email: 'user@example.com', password: 'senha123' })
    expect(result.success).toBe(true)
  })

  it('rejeita email inválido', () => {
    const result = loginSchema.safeParse({ email: 'nao-é-email', password: 'senha123' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toMatch(/email/i)
  })

  it('rejeita email vazio', () => {
    const result = loginSchema.safeParse({ email: '', password: 'senha123' })
    expect(result.success).toBe(false)
  })

  it('rejeita senha com menos de 6 caracteres', () => {
    const result = loginSchema.safeParse({ email: 'user@example.com', password: '12345' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toMatch(/6/i)
  })

  it('rejeita senha vazia', () => {
    const result = loginSchema.safeParse({ email: 'user@example.com', password: '' })
    expect(result.success).toBe(false)
  })

  it('rejeita email com formato completamente inválido', () => {
    const result = loginSchema.safeParse({ email: 'semdominio@', password: 'senha123' })
    expect(result.success).toBe(false)
  })
})

describe('registerSchema', () => {
  const valid = {
    email: 'user@example.com',
    username: 'joao.silva',
    password: 'Senha1234',
    first_name: 'João',
    last_name: 'Silva',
  }

  it('aceita dados válidos', () => {
    expect(registerSchema.safeParse(valid).success).toBe(true)
  })

  it('rejeita username com menos de 3 caracteres', () => {
    const result = registerSchema.safeParse({ ...valid, username: 'ab' })
    expect(result.success).toBe(false)
  })

  it('rejeita username com caracteres inválidos', () => {
    const result = registerSchema.safeParse({ ...valid, username: 'user name!' })
    expect(result.success).toBe(false)
  })

  it('rejeita senha sem letra maiúscula', () => {
    const result = registerSchema.safeParse({ ...valid, password: 'senha1234' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toMatch(/maiúscula/i)
  })

  it('rejeita senha sem número', () => {
    const result = registerSchema.safeParse({ ...valid, password: 'SenhaSemNumero' })
    expect(result.success).toBe(false)
    expect(result.error?.issues[0].message).toMatch(/número/i)
  })

  it('rejeita senha com menos de 8 caracteres', () => {
    const result = registerSchema.safeParse({ ...valid, password: 'Abc123' })
    expect(result.success).toBe(false)
  })
})
