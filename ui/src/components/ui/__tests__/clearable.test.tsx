import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi} from 'vitest';
import {Combobox} from '@/components/ui/combobox';
import {OptionsSelect} from '@/components/ui/options-select';
import {useForm} from 'react-hook-form';
import {Form, FormItem, FormLabel} from '@/components/ui/form';

const OPTIONS = [{value: '1', label: 'Um'}];

function Field({label, onChange}: { label: string; onChange: (v: string) => void }) {
  const form = useForm();
  return (
    <Form {...form}>
      <FormItem>
        <FormLabel>{label}</FormLabel>
        <Combobox value="1" onValueChange={onChange} options={OPTIONS}/>
        <OptionsSelect value="1" onValueChange={onChange} options={OPTIONS}/>
      </FormItem>
    </Form>
  );
}

describe('X de limpar automático', () => {
  it('aparece em campo opcional (rótulo sem asterisco) e limpa', async () => {
    const onChange = vi.fn();
    render(<Field label="Código NBS" onChange={onChange}/>);
    const buttons = await screen.findAllByRole('button', {name: 'Limpar seleção'});
    expect(buttons).toHaveLength(2);
    await userEvent.setup().click(buttons[0]);
    expect(onChange).toHaveBeenCalledWith('');
  });

  it('não aparece em campo obrigatório nem fora de formulário', () => {
    render(<Field label="Unidade *" onChange={vi.fn()}/>);
    render(<Combobox value="1" options={OPTIONS}/>);
    expect(screen.queryByRole('button', {name: 'Limpar seleção'})).toBeNull();
  });
});
