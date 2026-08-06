'use client'

import {useQuery} from '@tanstack/react-query'
import {apiClient} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {Combobox, type ComboboxOption} from '@/components/ui/combobox'
import {Button} from '@/components/ui/button'
import type {ServiceOut} from '@/lib/types/api'

interface NfseServicePickerProps {
  /** service.service_id do formulário — a SK completa do serviço (ver NfseEmitForm.handleSelectService). */
  value?: string
  onSelect: (service: ServiceOut) => void
  onClear: () => void
}

/**
 * Combobox sobre o catálogo de serviços. O catálogo é ponto de partida, não
 * trava: ao selecionar, o chamador copia os campos para o formulário e o
 * usuário pode sobrescrevê-los livremente (NfseServiceItem aceita overrides).
 */
export function NfseServicePicker({value, onSelect, onClear}: NfseServicePickerProps) {
  const {data, isLoading} = useQuery({
    queryKey: queryKeys.services.list(undefined),
    queryFn: () => apiClient.getServices({limit: 100}),
  })

  const services = data?.items ?? []
  const options: ComboboxOption[] = services.map((s) => ({
    value: s.sk,
    label: `${s.code} – ${s.description}`,
    display: s.code,
  }))

  const handleSelect = (sk: string) => {
    const service = services.find((s) => s.sk === sk)
    if (service) onSelect(service)
  }

  return (
    <div className="flex items-center gap-2">
      <div className="flex-1">
        <Combobox
          value={value}
          onValueChange={handleSelect}
          options={options}
          placeholder={isLoading ? 'Carregando serviços…' : 'Buscar serviço do catálogo'}
        />
      </div>
      {value && (
        <Button type="button" variant="ghost" size="xs" onClick={onClear}>
          Limpar seleção
        </Button>
      )}
    </div>
  )
}
