import {describe, expect, it} from 'vitest'
import {render, screen} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {useForm} from 'react-hook-form'
import {zodResolver} from '@hookform/resolvers/zod'
import {z} from 'zod'
import {Form, FormField, FormItem, FormLabel, FormMessage} from '@/components/ui/form'
import {Input} from '@/components/ui/input'

const schema = z.object({code: z.string().min(3, 'Mínimo 3 caracteres')})

function Harness() {
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {code: ''},
  })
  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(() => {})}>
        <FormField control={form.control} name="code" render={({field}) => (
          <FormItem>
            <FormLabel>Código</FormLabel>
            <Input {...field} id={field.name}/>
            <FormMessage/>
          </FormItem>
        )}/>
        <button type="submit">Salvar</button>
      </form>
    </Form>
  )
}

describe('erro de campo anunciado por leitor de tela', () => {
  it('liga aria-invalid e aria-describedby à mensagem depois do submit inválido', async () => {
    const user = userEvent.setup()
    render(<Harness/>)

    const input = screen.getByLabelText('Código')
    expect(input).not.toHaveAttribute('aria-invalid')

    await user.click(screen.getByRole('button', {name: 'Salvar'}))

    const message = await screen.findByText('Mínimo 3 caracteres')
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAttribute('aria-describedby', message.id)
  })
})
