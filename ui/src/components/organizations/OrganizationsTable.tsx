'use client'

import type {OrganizationOut} from '@/lib/types/api'
import {Button} from '@/components/ui/button'
import {formatCpfCnpj} from "@/lib/utils/document";

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
    <div className="overflow-x-auto bg-white rounded-lg border border-gray-200">
      <table className="w-full min-w-[560px]">
        <thead className="bg-primary-50 border-b border-gray-200">
        <tr>
          <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900">Nome</th>
          <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900">CNPJ/CPF</th>
          <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900">Nome Fantasia</th>
          <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900">Data de Criação</th>
          <th className="px-6 py-3 text-left text-sm font-semibold text-gray-900">Ações</th>
        </tr>
        </thead>
        <tbody>
        {organizations.map((org, idx) => (
          <tr key={org.pk} className={idx % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
            <td className="px-6 py-4 text-sm text-gray-900 font-medium">{org.name}</td>
            <td className="px-6 py-4 text-sm text-gray-600">{formatCpfCnpj(org.pk.replace('CNPJ_', '').replace('CPF_', ''))}</td>
            <td className="px-6 py-4 text-sm text-gray-600">{org.person.fantasy_name}</td>
            <td className="px-6 py-4 text-sm text-gray-600">
              {new Date(org.created_at).toLocaleDateString('pt-BR')}
            </td>
            <td className="px-6 py-4 text-sm">
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
        </tbody>
      </table>
    </div>
  )
}
