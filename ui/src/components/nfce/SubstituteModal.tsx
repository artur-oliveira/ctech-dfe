'use client'

import {useState} from 'react'
import {apiClient} from '@/lib/api/client'
import {Modal} from '@/components/ui/modal'
import {JustificationField} from '@/components/ui/justification-field'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import type {NfeListOut} from '@/lib/types/api'
import {formatCurrency} from '@/lib/utils/helpers'
import {NfeStatusCell} from '@/components/nfe/NfeStatusBadge'

const CANCEL_JUSTIFICATION_MIN_LENGTH = 15
const CANCEL_JUSTIFICATION_MAX_LENGTH = 255

/**
 * Cancelamento por substituição (evento 110112). The user provides the number or
 * access key of an already-authorized replacement NFC-e; we look it up so values
 * can be compared and divergences flagged before confirming.
 */
export function SubstituteModal({target, onClose, onConfirm, loading}: {
  target: Pick<NfeListOut, 'sk' | 'number' | 'serie' | 'total' | 'status' | 'sefaz_motive'>
  onClose: () => void
  onConfirm: (substituteKey: string, justification: string) => void
  loading: boolean
}) {
  const [queryStr, setQueryStr] = useState('')
  const [justification, setJustification] = useState('')
  const [lookupLoading, setLookupLoading] = useState(false)
  const [substitute, setSubstitute] = useState<NfeListOut | null>(null)
  const [lookupError, setLookupError] = useState<string | null>(null)

  const lookup = async () => {
    setLookupError(null)
    setSubstitute(null)
    const digits = queryStr.replace(/\D/g, '')
    try {
      setLookupLoading(true)
      let found: NfeListOut | null = null
      if (digits.length === 44) {
        found = await apiClient.getNfce(digits)
      } else if (digits.length > 0) {
        const res = await apiClient.listNfces({number: parseInt(digits, 10), limit: 1, sort: 'desc'})
        found = res.items[0] ?? null
      }
      if (!found) {
        setLookupError('NFC-e substituta não encontrada.')
        return
      }
      if (found.sk === target.sk) {
        setLookupError('A NFC-e substituta deve ser diferente da que será cancelada.')
        return
      }
      setSubstitute(found)
    } catch {
      setLookupError('Erro ao consultar NFC-e substituta.')
    } finally {
      setLookupLoading(false)
    }
  }

  const totalDiverges = substitute && Math.abs(parseFloat(substitute.total) - parseFloat(target.total)) > 0.01
  const notAuthorized = substitute && substitute.status !== 'authorized'
  const canConfirm = !!substitute && !notAuthorized && justification.trim().length >= CANCEL_JUSTIFICATION_MIN_LENGTH

  return (
    <Modal
      isOpen
      title={`Substituir NFC-e nº ${target.number}`}
      onClose={onClose}
      onSubmit={() => substitute && onConfirm(substitute.sk, justification.trim())}
      submitLabel="Confirmar substituição"
      cancelLabel="Voltar"
      danger
      loading={loading}
      submitDisabled={!canConfirm}
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          O cancelamento por substituição cancela esta NFC-e referenciando uma <span className="font-medium">nova
          NFC-e já autorizada</span> que a substitui. Informe o número ou a chave de acesso da nova NFC-e.
        </p>
        <div className="flex flex-col sm:flex-row gap-2 items-stretch sm:items-end">
          <div className="flex-1 flex flex-col gap-1">
            <label htmlFor="substitute-query" className="text-xs font-medium text-gray-600">Número ou chave de acesso
              (44 dígitos)</label>
            <Input id="substitute-query" value={queryStr} onChange={(e) => setQueryStr(e.target.value)}
                   placeholder="Ex: 43 ou 3526…" className="w-full font-mono"/>
          </div>
          <Button type="button" variant="brand" size="sm" onClick={lookup}
                  disabled={lookupLoading || !queryStr.trim()} className="h-11">
            {lookupLoading ? 'Buscando…' : 'Buscar'}
          </Button>
        </div>

        {lookupError && <p className="text-sm text-red-600">{lookupError}</p>}

        {substitute && (
          <div className="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm space-y-1">
            <div className="flex justify-between"><span className="text-gray-500">Substituta nº</span>
              <span className="font-medium">{substitute.number} / {substitute.serie}</span></div>
            <div className="flex justify-between"><span className="text-gray-500">Total</span>
              <span className="font-medium">{formatCurrency(substitute.total)}</span></div>
            <div className="flex justify-between"><span className="text-gray-500">Status</span>
              <NfeStatusCell status={substitute.status} sefazMotive={substitute.sefaz_motive}/></div>
            {notAuthorized && <p className="text-xs text-red-600 pt-1">A NFC-e substituta precisa estar autorizada.</p>}
            {totalDiverges && !notAuthorized && (
              <p className="text-xs text-amber-600 pt-1">
                Atenção: o total da substituta ({formatCurrency(substitute.total)}) difere desta NFC-e
                ({formatCurrency(target.total)}). Verifique se selecionou a nota correta.
              </p>
            )}
          </div>
        )}

        <JustificationField
          id="substitute-justification"
          value={justification}
          onChange={setJustification}
          minLength={CANCEL_JUSTIFICATION_MIN_LENGTH}
          maxLength={CANCEL_JUSTIFICATION_MAX_LENGTH}
          rows={3}
          placeholder="Motivo da substituição (mínimo 15 caracteres)…"
        />
      </div>
    </Modal>
  )
}
