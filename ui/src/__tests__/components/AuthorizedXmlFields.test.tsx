import {describe, expect, it, vi} from 'vitest';
import {useState} from 'react';
import {fireEvent, render, screen} from '@testing-library/react';
import {AUTHORIZED_XML_MAX, AuthorizedXmlFields, type AuthorizedXmlEntry} from '@/components/mdfe/AuthorizedXmlFields';
import type {PersonItemOut} from '@/lib/types/api';

// O picker real busca na api; aqui cada botão "escolhe" uma pessoa do cadastro.
const PEOPLE: Record<string, Partial<PersonItemOut>> = {
  cnpj: {sk: 'CNPJ_11222333000181', name: 'Contabilidade Exemplo'},
  alnum: {sk: 'CNPJ_12ABC34501DE35', name: 'Embarcador Alfanumérico'},
  badDv: {sk: 'CNPJ_11222333000182', name: 'Cadastro Errado'},
  foreign: {sk: 'IDEST_X1', name: 'Exterior SA'},
};
vi.mock('@/components/persons/PersonPicker', () => ({
  PersonPicker: ({onChange}: { onChange: (p: PersonItemOut) => void }) => (
    <div>{Object.entries(PEOPLE).map(([k, p]) => (
      <button key={k} type="button" onClick={() => onChange(p as PersonItemOut)}>pick {k}</button>
    ))}</div>
  ),
}));

function Harness({initial = []}: { initial?: AuthorizedXmlEntry[] }) {
  const [entries, setEntries] = useState(initial);
  return (
    <>
      <AuthorizedXmlFields entries={entries} onChange={setEntries} errors={['erro da api']}/>
      <output data-testid="state">{JSON.stringify(entries.map((e) => e.doc))}</output>
    </>
  );
}

describe('AuthorizedXmlFields', () => {
  it('inclui CNPJ numérico e alfanumérico do cadastro', () => {
    render(<Harness/>);
    fireEvent.click(screen.getByText('pick cnpj'));
    fireEvent.click(screen.getByText('pick alnum'));
    expect(screen.getByTestId('state').textContent).toBe('["11222333000181","12ABC34501DE35"]');
  });

  it('recusa repetido, DV inválido e pessoa no exterior', () => {
    render(<Harness/>);
    fireEvent.click(screen.getByText('pick cnpj'));
    fireEvent.click(screen.getByText('pick cnpj'));
    expect(screen.getByText('Contabilidade Exemplo já está na lista.')).toBeInTheDocument();
    fireEvent.click(screen.getByText('pick badDv'));
    expect(screen.getByText(/dígito inválido/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('pick foreign'));
    expect(screen.getByText(/está no exterior/)).toBeInTheDocument();
    expect(screen.getByTestId('state').textContent).toBe('["11222333000181"]');
  });

  it('esconde o picker no limite de 10 e mostra o erro da api no campo', () => {
    const full = Array.from({length: AUTHORIZED_XML_MAX}, (_, i) => ({doc: `doc${i}`, name: `P${i}`}));
    render(<Harness initial={full}/>);
    expect(screen.queryByText('pick cnpj')).toBeNull();
    expect(screen.getByText(/Limite de 10 atingido/)).toBeInTheDocument();
    expect(screen.getByText('erro da api')).toBeInTheDocument();
  });

  it('remove pelo botão', () => {
    render(<Harness initial={[{doc: '11222333000181', name: 'Contabilidade Exemplo'}]}/>);
    fireEvent.click(screen.getByRole('button', {name: 'Remover Contabilidade Exemplo'}));
    expect(screen.getByTestId('state').textContent).toBe('[]');
  });
});
