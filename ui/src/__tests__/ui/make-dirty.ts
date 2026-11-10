import {fireEvent} from '@testing-library/react';

/** Config já gravada e intacta trava o "Salvar alterações"; estes testes precisam de um formulário alterado. */
export function makeDirty() {
  const input = document.querySelector<HTMLInputElement>('input[name][type="text"], input[name]:not([type])');
  if (!input) throw new Error('sem campo de texto para alterar');
  fireEvent.change(input, {target: {value: `${input.value}1`}});
}
