import {render, screen} from '@testing-library/react';
import {useForm} from 'react-hook-form';
import {describe, expect, it} from 'vitest';
import {Form} from '@/components/ui/form';
import {MunicipalityField} from '@/components/ui/municipality-field';

function Harness({initial}: { initial?: string }) {
  const form = useForm<{ ibge: string }>({defaultValues: {ibge: initial ?? ''}});
  return (
    <Form {...form}>
      <MunicipalityField control={form.control} setValue={form.setValue} name="ibge" label="Município"/>
    </Form>
  );
}

describe('MunicipalityField', () => {
  it('carrega a tabela do IBGE sob demanda e mostra o município escolhido pelo código', async () => {
    render(<Harness initial="3550308"/>);
    expect(screen.getByText('Município')).toBeInTheDocument();
    expect(await screen.findByText(/São Paulo \/ SP/)).toBeInTheDocument();
  });

  it('sem valor mostra o convite para selecionar', async () => {
    render(<Harness/>);
    expect(await screen.findByText('Selecione o município…')).toBeInTheDocument();
  });
});
