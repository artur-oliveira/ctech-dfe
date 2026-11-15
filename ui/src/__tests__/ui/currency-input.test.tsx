import {describe, expect, it} from 'vitest';
import {render, screen} from '@testing-library/react';
import {CurrencyInput} from '@/components/ui/currency-input';

describe('CurrencyInput · unidade', () => {
  it('moeda (padrão): prefixo R$', () => {
    render(<CurrencyInput id="v" value="10" onChange={() => {}}/>);
    expect(screen.getByText('R$')).toBeTruthy();
    expect(screen.queryByText('%')).toBeNull();
  });

  it('percentual: sufixo %, sem R$', () => {
    render(<CurrencyInput id="p" unit="percent" value="2" onChange={() => {}}/>);
    expect(screen.getByText('%')).toBeTruthy();
    expect(screen.queryByText('R$')).toBeNull();
  });
});
