'use client'

import type {OrganizationOut} from '@/lib/types/api'
import {Button} from '@/components/ui/button'
import {TableShell, TABLE_ROW, TABLE_CELL} from '@/components/ui/table-shell'
import {formatCpfCnpj, orgTaxId} from "@/lib/utils/document";

export type Organization = OrganizationOut

interface OrganizationsTableProps {
  organizations: Organization[]
  onEdit: (org: Organization) => void
  loading: boolean
}

export function OrganizationsTable({
                                     organizations,
                                     onEdit,
                                     loading,
                                   }: OrganizationsTableProps) {
  if (loading) {
    return (
      <div className="space-y-2">
        {[...Array(5)].map((_, i) => (
          <div key={i} className="h-20 bg-gray-100 rounded-lg animate-pulse"/>
        ))}
      </div>
    )
  }
  if (organizations.length === 0) {
    return (
      <div className="text-center py-12 bg-white rounded-lg border border-gray-200">
        <p className="text-gray-500 text-lg">Nenhuma organização encontrada</p>
        <p className="text-gray-400 text-sm mt-1">Crie sua primeira organização para começar</p>
      </div>
    )
  }

  return (
    <TableShell
      ariaLabel="Organizações"
      minWidth={560}
      headers={['Nome', 'CNPJ/CPF', 'Nome Fantasia', 'Data de Criação', '']}
    >
      {organizations.map((org) => (
        <tr key={org.pk} className={TABLE_ROW}>
          <td data-label="Nome" className={`${TABLE_CELL} text-sm text-gray-900 font-medium`}>{org.name}</td>
          <td
            data-label="CNPJ/CPF" className={`${TABLE_CELL} text-sm text-gray-600`}>{formatCpfCnpj(orgTaxId(org)) || '—'}</td>
          {/* A linked company has no fiscal side until somebody fills it in. */}
          <td data-label="Nome Fantasia"
              className={`${TABLE_CELL} text-sm text-gray-600`}>{org.person?.fantasy_name ?? '—'}</td>
          <td data-label="Data de Criação" className={`${TABLE_CELL} text-sm text-gray-600`}>
            {new Date(org.created_at).toLocaleDateString('pt-BR')}
          </td>
          <td className={`${TABLE_CELL} text-sm`}>
            <div className="flex items-center space-x-3">
              <Button
                variant="ghost"
                size="xs"
                onClick={() => onEdit(org)}
                className="text-brand-600 hover:text-brand-700"
              >
                Editar
              </Button>
            </div>
          </td>
        </tr>
      ))}
    </TableShell>
  )
}
