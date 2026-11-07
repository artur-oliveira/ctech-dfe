'use client';

import {useState} from 'react';
import {Button} from '@aoctech/ui';
import {PersonPicker} from '@/components/persons/PersonPicker';
import {SK_PREFIX} from '@/lib/constants/entity-keys';
import {formatCpfCnpj, unformatCpfCnpj} from '@/lib/utils/document';
import {validateCNPJ, validateCPF} from '@/lib/utils/validators';
import type {PersonItemOut} from '@/lib/types/api';

/** `infMDFe/autXML` aceita até 10 CPF/CNPJ (MOC, grupo autXML 0-10). */
export const AUTHORIZED_XML_MAX = 10;

export interface AuthorizedXmlEntry {
  doc: string
  name: string
}

export interface AuthorizedXmlFieldsProps {
  entries: AuthorizedXmlEntry[]
  onChange: (entries: AuthorizedXmlEntry[]) => void
  /** Recusas da api para o grupo (F105 a F107, cpfcnpj), mostradas junto do campo. */
  errors?: string[]
}

/**
 * Quem mais pode baixar o XML do manifesto (contador, embarcador). Cada CPF ou CNPJ vem do cadastro de pessoas,
 * nunca digitado; a lista recusa aqui o que a SEFAZ recusaria: dígito inválido (F105, F106), repetição (F107),
 * pessoa no exterior (o grupo só aceita CPF ou CNPJ) e mais de 10.
 */
export function AuthorizedXmlFields({entries, onChange, errors = []}: AuthorizedXmlFieldsProps) {
  const [refusal, setRefusal] = useState<string | null>(null);
  const full = entries.length >= AUTHORIZED_XML_MAX;

  const add = (person: PersonItemOut | null) => {
    if (!person) return;
    if (person.sk.startsWith(SK_PREFIX.FOREIGN)) {
      setRefusal(`${person.name} está no exterior; só CPF ou CNPJ pode baixar o XML.`);
      return;
    }
    const doc = unformatCpfCnpj(person.sk);
    const valid = doc.length === 11 ? validateCPF(doc) : validateCNPJ(doc);
    if (!valid) {
      setRefusal(`O CPF ou CNPJ de ${person.name} tem dígito inválido; corrija o cadastro da pessoa.`);
      return;
    }
    if (entries.some((e) => e.doc === doc)) {
      setRefusal(`${person.name} já está na lista.`);
      return;
    }
    setRefusal(null);
    onChange([...entries, {doc, name: person.name}]);
  };

  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 space-y-3">
      <div>
        <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">
          Autorizados a baixar o XML (opcional)
        </p>
        <p className="mt-1 text-xs text-gray-500">
          Contador, embarcador ou quem mais precisa do XML no portal da SEFAZ. Até {AUTHORIZED_XML_MAX}.
        </p>
      </div>

      {entries.length > 0 && (
        <ul className="space-y-1.5" aria-label="Autorizados a baixar o XML">
          {entries.map((e) => (
            <li key={e.doc} className="flex items-center justify-between gap-2 rounded-lg bg-gray-50 px-3 py-2 text-sm">
              <span className="min-w-0 truncate text-gray-700">
                {e.name} · <span className="font-mono text-gray-500">{formatCpfCnpj(e.doc)}</span>
              </span>
              <Button type="button" variant="ghost" size="sm" className="min-h-11 shrink-0 text-danger sm:min-h-0"
                      aria-label={`Remover ${e.name}`}
                      onClick={() => onChange(entries.filter((x) => x.doc !== e.doc))}>
                remover
              </Button>
            </li>
          ))}
        </ul>
      )}

      {full ? (
        <p className="text-xs text-gray-500">Limite de {AUTHORIZED_XML_MAX} atingido; remova um para incluir outro.</p>
      ) : (
        <PersonPicker value={null} onChange={add} placeholder="Buscar pessoa cadastrada"/>
      )}
      {refusal && <p role="alert" className="text-xs text-red-600">{refusal}</p>}
      {errors.map((e) => <p key={e} role="alert" className="text-xs text-red-600">{e}</p>)}
    </div>
  );
}
