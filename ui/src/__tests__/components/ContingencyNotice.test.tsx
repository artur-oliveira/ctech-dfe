import {afterEach, describe, expect, it} from 'vitest';
import {cleanup, render, screen} from '@testing-library/react';
import {ContingencyBadge, ContingencyNotice} from '@/components/dfe/ContingencyNotice';
import {DfeStatusCell} from '@/components/dfe/DfeStatusBadge';

describe('contingência no documento', () => {
  afterEach(cleanup);

  it('aviso do pendente traz o prazo', () => {
    render(<ContingencyNotice doc={{contingency_state: 'pending', contingency_deadline: '2026-07-09T12:00:00Z'}}/>);
    expect(screen.getByRole('status')).toHaveTextContent(/Emitida em contingência, pendente de autorização; prazo até/);
  });

  it.each([
    ['authorized_late', /autorizada depois pela SEFAZ/],
    ['rejected_late', /rejeitada pela SEFAZ/],
    ['overdue', /prazo para transmitir/],
  ] as const)('estado %s tem frase própria', (state, text) => {
    render(<ContingencyNotice doc={{contingency_state: state}}/>);
    expect(screen.getByRole('status')).toHaveTextContent(text);
  });

  it('documento normal não mostra nada', () => {
    const {container} = render(<><ContingencyNotice doc={{}}/><ContingencyBadge state={null}/></>);
    expect(container).toBeEmptyDOMElement();
  });

  it('a lista acrescenta o selo sob o status', () => {
    render(<DfeStatusCell status="pending" sefazMotive={null} contingencyState="pending"/>);
    expect(screen.getByText('Contingência pendente')).toBeInTheDocument();
  });
});
