import {describe, expect, it, vi} from 'vitest';
import {fireEvent, render, screen} from '@testing-library/react';
import {
  ciotRequired,
  defaultEmitterType,
  emitterTypeBlock,
  EmitterTypeField,
  type EmitterContext,
} from '@/components/mdfe/EmitterTypeField';

const nfe: EmitterContext = {docType: 'nfe', laterLoading: false, interstate: false};

describe('regras do tipo de emitente', () => {
  it('padrão é o do documento: NF-e 2, CT-e 1, carregamento posterior 2', () => {
    expect(defaultEmitterType(nfe)).toBe('2');
    expect(defaultEmitterType({...nfe, docType: 'cte'})).toBe('1');
    expect(defaultEmitterType({...nfe, docType: 'cte', laterLoading: true})).toBe('2');
  });

  it('F14: tipo 1 não manifesta NF-e; F15/F16: 2 e 3 não manifestam CT-e', () => {
    expect(emitterTypeBlock('1', nfe)).toMatch(/Exige CT-e/);
    expect(emitterTypeBlock('2', {...nfe, docType: 'cte'})).toMatch(/só NF-e/);
    expect(emitterTypeBlock('3', {...nfe, docType: 'cte'})).toMatch(/só NF-e/);
  });

  it('F17: tipo 3 só dentro de um estado', () => {
    expect(emitterTypeBlock('3', nfe)).toBeNull();
    expect(emitterTypeBlock('3', {...nfe, interstate: true})).toMatch(/dentro de um estado/);
  });

  it('F24: carregamento posterior só com tipo 2', () => {
    expect(emitterTypeBlock('3', {...nfe, laterLoading: true})).toMatch(/carga própria/);
    expect(emitterTypeBlock('2', {...nfe, laterLoading: true})).toBeNull();
  });

  it('F113: CIOT obrigatório no rodoviário com tipo 1 ou 3 e RNTRC', () => {
    expect(ciotRequired('3', true, '12345678')).toBe(true);
    expect(ciotRequired('1', true, '12345678')).toBe(true);
    expect(ciotRequired('2', true, '12345678')).toBe(false);
    expect(ciotRequired('3', false, '12345678')).toBe(false);
    expect(ciotRequired('3', true, '')).toBe(false);
  });
});

describe('EmitterTypeField', () => {
  it('desabilita o que não vale e explica o motivo', () => {
    const onChange = vi.fn();
    render(<EmitterTypeField value="2" onChange={onChange} context={{...nfe, interstate: true}}/>);
    expect(screen.getByRole('radio', {name: /1\. Prestador de serviço/})).toBeDisabled();
    expect(screen.getByRole('radio', {name: /3\. Prestador que emitirá/})).toBeDisabled();
    expect(screen.getByText('Vale só para viagem dentro de um estado.')).toBeInTheDocument();
  });

  it('escolhe o tipo 3 dentro de um estado', () => {
    const onChange = vi.fn();
    render(<EmitterTypeField value="2" onChange={onChange} context={nfe}/>);
    fireEvent.click(screen.getByRole('radio', {name: /3\. Prestador que emitirá/}));
    expect(onChange).toHaveBeenCalledWith('3');
  });

  it('mostra no campo a escolha que deixou de valer e a recusa da api', () => {
    render(<EmitterTypeField value="3" onChange={vi.fn()} context={{...nfe, interstate: true}}
                             errors={['Escolha carga própria (2).']}/>);
    expect(screen.getAllByRole('alert').map((a) => a.textContent)).toEqual([
      'Vale só para viagem dentro de um estado.', 'Escolha carga própria (2).',
    ]);
  });
});
