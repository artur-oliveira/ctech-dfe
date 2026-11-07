import {describe, expect, it, vi} from 'vitest';
import {fireEvent, render, screen} from '@testing-library/react';
import {EmitError} from '@/components/ui/emit-error';

describe('EmitError', () => {
  it('lista cada recusa local com a correção: link para cadastro, botão para bloco da nota', () => {
    const onClick = vi.fn();
    render(
      <EmitError failure={{
        message: 'a nota não passa nas validações',
        issues: [
          {text: 'Produto do item 1: Descrição tem 134 caracteres e o leiaute aceita até 120.', hint: 'Corte 14 caracteres.',
            target: 'produto', item: 1, field: 'description', action: {label: 'Abrir cadastro', href: '/products/edit?id=1'}},
          {text: 'Regra YA03-20 do MOC; a SEFAZ rejeitaria a nota com o código 866.', target: 'pagamento',
            field: 'YA03-20', action: {label: 'Editar', onClick}},
        ],
      }}/>,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('Corte 14 caracteres.');
    expect(screen.getByRole('link', {name: 'Abrir cadastro'})).toHaveAttribute('href', '/products/edit?id=1');
    fireEvent.click(screen.getByRole('button', {name: 'Editar'}));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
