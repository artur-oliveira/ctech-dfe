'use client';

import {useState} from 'react';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {unformatCpfCnpj} from '@/lib/utils/document';
import {cleanDocument, validateCNPJ, validateCPF} from '@/lib/utils/validators';
import type {PersonItemOut} from '@/lib/types/api';

export interface PickedDoc { doc: string; name: string }

/**
 * CPF ou CNPJ de uma pessoa do cadastro, nunca digitado: recusa pessoa no exterior e documento com dígito
 * inválido. O esquema dos eventos de NF-e (TCnpj v1.03) aceita só 14 dígitos numéricos: CNPJ alfanumérico é
 * recusado aqui até o esquema oficial permitir (a api também recusa, 422 event_invalid). `accept` limita o que
 * serve ao campo.
 */
export function PersonDocField({id, label, value, onChange, accept = 'any', placeholder}: {
  id: string; label: string; value: PickedDoc | null; onChange: (v: PickedDoc | null) => void
  accept?: 'any' | 'cnpj'; placeholder?: string
}) {
  const [refusal, setRefusal] = useState<string | null>(null);
  const pick = (p: PersonItemOut | null) => {
    if (!p) { setRefusal(null); onChange(null); return; }
    if (p.sk.startsWith(SK_PREFIX.FOREIGN)) { setRefusal(`${p.name} está no exterior; só CPF ou CNPJ serve aqui.`); return; }
    const doc = cleanDocument(unformatCpfCnpj(p.sk));
    const isCnpj = doc.length === 14;
    if (!/^\d+$/.test(doc)) {
      setRefusal(`O CNPJ de ${p.name} tem letras e o evento fiscal ainda só aceita CNPJ numérico.`); return;
    }
    if (accept === 'cnpj' && !isCnpj) { setRefusal(`${p.name} é pessoa física; escolha uma empresa (CNPJ).`); return; }
    if (!(isCnpj ? validateCNPJ(doc) : doc.length === 11 && validateCPF(doc))) {
      setRefusal(`O CPF ou CNPJ de ${p.name} tem dígito inválido; corrija o cadastro da pessoa.`); return;
    }
    setRefusal(null);
    onChange({doc, name: p.name});
  };
  return (
    <div className="flex flex-col gap-1">
      <span id={id} className="text-xs font-medium text-gray-600">{label}</span>
      <PersonPicker value={value ? ({sk: value.doc, name: value.name} as PersonItemOut) : null} onChange={pick}
                    placeholder={placeholder ?? 'Buscar pessoa cadastrada'}/>
      {refusal && <p role="alert" className="text-xs text-red-600">{refusal}</p>}
    </div>
  );
}
