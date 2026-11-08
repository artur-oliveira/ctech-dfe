import {render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import {CiotStatusBadge} from '@/components/ciot/CiotStatusBadge';
import {CIOT_STATUS_FILTERS, CIOT_STATUS_LABELS} from '@/lib/constants/ciot';

describe('CiotStatusBadge', () => {
  it('rotula cada status, no masculino', () => {
    for (const [status, label] of Object.entries(CIOT_STATUS_LABELS)) {
      const {unmount} = render(<CiotStatusBadge ciot={{status: status as keyof typeof CIOT_STATUS_LABELS}}/>);
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it('operação em andamento aparece no lugar do status', () => {
    render(<CiotStatusBadge ciot={{status: 'declared', pending_operation: 'cancel'}}/>);
    expect(screen.getByText('Cancelamento em andamento')).toBeInTheDocument();
    expect(screen.queryByText('Declarado')).toBeNull();
  });

  it('os filtros da lista cobrem todos os status e começam por Todos', () => {
    expect(CIOT_STATUS_FILTERS[0]).toEqual({value: '', label: 'Todos'});
    expect(CIOT_STATUS_FILTERS.slice(1).map((f) => f.value).sort()).toEqual(Object.keys(CIOT_STATUS_LABELS).sort());
  });

  it('nenhum rótulo leva travessão', () => {
    expect(Object.values(CIOT_STATUS_LABELS).join('')).not.toContain('—');
  });
});
