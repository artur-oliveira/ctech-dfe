import {act, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import {FormWizard, useWizard, type WizardStep} from '@/components/ui/form-wizard';

function Demo({onSubmit = vi.fn(), apiErrorStep}: { onSubmit?: () => void; apiErrorStep?: string }) {
  const [name, setName] = useState('');
  const [error, setError] = useState('');
  const steps: WizardStep[] = [
    {id: 'a', label: 'Dados', validate: () => { setError(name ? '' : 'Informe o nome'); return !!name; }, content: (
      <>
        <input id="name" aria-label="Nome" value={name} onChange={(e) => setName(e.target.value)}/>
        {error && <p id="name-error" role="alert">{error}</p>}
      </>
    )},
    {id: 'b', label: 'Extras', content: <input aria-label="Extra"/>},
    {id: 'c', label: 'Revisão', content: (w) => <button type="button" onClick={() => w.goTo('a')}>Editar dados</button>},
  ];
  const wizard = useWizard(steps);
  return (
    <FormWizard wizard={wizard} steps={steps} onSubmit={onSubmit} submitLabel="Enviar">
      {apiErrorStep && <button type="button" onClick={() => wizard.goTo(apiErrorStep)}>falha da api</button>}
    </FormWizard>
  );
}

const heading = () => screen.getByRole('heading', {level: 2});

describe('FormWizard', () => {
  it('Continuar fica no passo com erro e foca o campo inválido', async () => {
    render(<Demo/>);
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    expect(await screen.findByText('Informe o nome')).toBeInTheDocument();
    expect(heading()).toHaveTextContent('Dados');
    await waitFor(() => expect(screen.getByLabelText('Nome')).toHaveFocus());
  });

  it('avança válido, marca aria-current, anuncia e leva o foco ao título', async () => {
    render(<Demo/>);
    fireEvent.change(screen.getByLabelText('Nome'), {target: {value: 'x'}});
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    expect(heading()).toHaveTextContent('Extras');
    expect(heading()).toHaveFocus();
    expect(screen.getByRole('status')).toHaveTextContent('Passo 2 de 3: Extras');
    expect(document.querySelector('[aria-current="step"]')).toHaveTextContent('Extras');
  });

  it('Enter num campo avança em vez de enviar; só o último passo envia', async () => {
    const onSubmit = vi.fn();
    render(<Demo onSubmit={onSubmit}/>);
    fireEvent.change(screen.getByLabelText('Nome'), {target: {value: 'x'}});
    fireEvent.submit(screen.getByLabelText('Nome').closest('form')!);
    expect(heading()).toHaveTextContent('Extras');
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    expect(onSubmit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', {name: 'Enviar'}));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it('voltar é livre; pular para frente só até o passo já visitado', () => {
    render(<Demo/>);
    expect(screen.getByRole('button', {name: /3\. Revisão/})).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Nome'), {target: {value: 'x'}});
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    fireEvent.click(screen.getByRole('button', {name: 'Voltar'}));
    expect(heading()).toHaveTextContent('Dados');
    expect(screen.getByRole('button', {name: /2\. Extras/})).toBeEnabled();
    expect(screen.getByRole('button', {name: /3\. Revisão/})).toBeDisabled();
  });

  it('o estado dos passos sobrevive à navegação e goTo leva ao passo dono do erro', () => {
    render(<Demo apiErrorStep="a"/>);
    fireEvent.change(screen.getByLabelText('Nome'), {target: {value: 'Maria'}});
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    fireEvent.click(screen.getByRole('button', {name: 'Continuar'}));
    expect(heading()).toHaveTextContent('Revisão');
    act(() => fireEvent.click(screen.getByRole('button', {name: 'Editar dados'})));
    expect(heading()).toHaveTextContent('Dados');
    expect(screen.getByLabelText('Nome')).toHaveValue('Maria');
    fireEvent.click(screen.getByRole('button', {name: 'falha da api'}));
    expect(heading()).toHaveTextContent('Dados');
  });
});
