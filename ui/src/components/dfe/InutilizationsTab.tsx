'use client'

import {useState} from 'react'
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query'
import {toast} from 'sonner'
import {apiClient, ApiError} from '@/lib/api/client'
import {queryKeys} from '@/lib/api/query-keys'
import {usePagination} from '@/lib/hooks/usePagination'
import {Button} from '@/components/ui/button'
import {Modal} from '@/components/ui/modal'
import {NumericInput} from '@/components/ui/numeric-input'
import {JustificationField} from '@/components/ui/justification-field'
import {EmptyState} from '@/components/ui/empty-state'
import {NfeIcon} from '@/components/ui/icon'
import {Pagination} from '@/components/ui/pagination'
import {LoadingSkeleton} from '@/components/ui/loading-skeleton'
import {TABLE_CELL, TABLE_ROW, TableShell} from '@/components/ui/table-shell'
import {DfeStatusCell} from '@/components/dfe/DfeStatusBadge'
import {formatDatetimeBR, triggerDownload} from '@/lib/utils/dfe'
import type {InutilizationOut, NumberGapOut} from '@/lib/types/api'

/** SEFAZ exige no mínimo 15 caracteres na justificativa da inutilização. */
export const INUT_JUSTIFICATION_MIN_LENGTH = 15
export const INUT_JUSTIFICATION_MAX_LENGTH = 255

type DocType = 'nfe' | 'nfce'

interface InutilizationsTabProps {
  docType: DocType
  /** 'NF-e' | 'NFC-e' — usado nos títulos e nas mensagens. */
  docLabel: string
  orgPk: string
}

function rangeLabel(start: number, end: number): string {
  return start === end ? `${start}` : `${start} – ${end}`
}

function rangeSize(start: number, end: number): number {
  return end - start + 1
}

/**
 * Baixa o ProcInutNFe da faixa. Só existe depois que a SEFAZ homologa — antes
 * disso não há documento nenhum para comprovar coisa alguma, então o botão não
 * aparece em vez de aparecer quebrado.
 */
function InutilizationXmlButton({docType, item}: { docType: DocType; item: InutilizationOut }) {
  const [loading, setLoading] = useState(false)

  if (!item.xml_s3_key) return null

  const handleDownload = async () => {
    setLoading(true)
    try {
      const blob = await apiClient.downloadInutilizationXml(docType, item.sk)
      triggerDownload(blob, `inutilizacao_${item.year}_${item.serie}_${item.number_start}-${item.number_end}.xml`)
    } catch {
      toast.error('Erro ao baixar o XML da inutilização.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button
      variant="ghost"
      size="xs"
      onClick={handleDownload}
      disabled={loading}
      className="text-brand-600 hover:text-brand-700"
    >
      {loading ? 'Baixando…' : 'XML'}
    </Button>
  )
}

/**
 * Aba de inutilizações de numeração.
 *
 * Numeração fiscal não pode ter buraco: todo número consumido sem gerar
 * documento autorizado deixa uma lacuna que o fisco cobra. A tela abre pelas
 * lacunas detectadas — não por um formulário em branco — para que o usuário
 * saiba o que precisa fechar antes de precisar perguntar.
 */
export function InutilizationsTab({docType, docLabel, orgPk}: InutilizationsTabProps) {
  const qc = useQueryClient()
  const [draft, setDraft] = useState<{ serie: number; start: string; end: string } | null>(null)
  const [justification, setJustification] = useState('')

  const gapsQuery = useQuery({
    queryKey: queryKeys.inutilizations.gaps(docType, orgPk),
    queryFn: () => apiClient.listNumberGaps(docType),
  })

  const {items, isLoading, isFetching, hasNext, hasPrevious, goNext, goPrevious} =
    usePagination<InutilizationOut>({
      queryKey: queryKeys.inutilizations.list(docType, orgPk),
      queryFn: (cursor) => apiClient.listInutilizations(docType, {limit: 10, cursor}),
    })

  const mutation = useMutation({
    mutationFn: () => apiClient.createInutilization(docType, {
      serie: draft!.serie,
      number_start: Number(draft!.start),
      number_end: Number(draft!.end),
      justification: justification.trim(),
    }),
    onSuccess: () => {
      closeModal()
      toast.info('Inutilização enviada à SEFAZ. O status é atualizado automaticamente.')
      void qc.invalidateQueries({queryKey: queryKeys.inutilizations.list(docType, orgPk)})
      void qc.invalidateQueries({queryKey: queryKeys.inutilizations.gaps(docType, orgPk)})
    },
    onError: (err: unknown) => {
      toast.error(err instanceof ApiError ? err.detail : 'Erro ao inutilizar a faixa.')
    },
  })

  const openModal = (gap?: NumberGapOut) => {
    setJustification('')
    setDraft({
      serie: gap?.serie ?? 0,
      start: gap ? String(gap.number_start) : '',
      end: gap ? String(gap.number_end) : '',
    })
  }

  const closeModal = () => {
    setDraft(null)
    setJustification('')
  }

  const gaps = gapsQuery.data?.items ?? []
  const start = Number(draft?.start)
  const end = Number(draft?.end)
  const rangeValid = Number.isInteger(start) && Number.isInteger(end) && start >= 1 && end >= start
  const canSubmit = rangeValid && justification.trim().length >= INUT_JUSTIFICATION_MIN_LENGTH

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <p className="max-w-[68ch] text-sm text-gray-500">
          A numeração de {docLabel} não pode ter buracos. Quando um número é consumido sem gerar
          documento autorizado, inutilize a faixa para fechar a lacuna junto à SEFAZ.
        </p>
        <Button
          variant="outline"
          size="sm"
          onClick={() => openModal()}
          className="text-brand-600 border-brand-200 hover:bg-brand-50"
        >
          Inutilizar faixa
        </Button>
      </div>

      {/* Lacunas detectadas — o que a tela existe para resolver, antes da tabela histórica. */}
      <section aria-labelledby="gaps-heading" className="space-y-3">
        <h2 id="gaps-heading" className="text-xs font-semibold uppercase tracking-wider text-gray-500">
          Lacunas detectadas
        </h2>

        {gapsQuery.isLoading ? (
          <LoadingSkeleton count={2}/>
        ) : gapsQuery.isError ? (
          <p className="text-sm text-gray-500">
            Não foi possível verificar a numeração agora. Configure a {docLabel} em Configuração Fiscal
            ou tente novamente.
          </p>
        ) : gaps.length === 0 ? (
          <p className="flex items-center gap-2 text-sm text-success">
            <span aria-hidden="true">✓</span>
            Numeração sem lacunas. Nada a inutilizar.
          </p>
        ) : (
          <ul className="divide-y divide-gray-200 rounded-xl border border-gray-200 bg-white">
            {gaps.map(gap => (
              <li
                key={`${gap.serie}-${gap.number_start}-${gap.number_end}`}
                className="flex flex-wrap items-center justify-between gap-3 px-4 py-3"
              >
                <div>
                  <p className="font-mono text-sm text-gray-900">
                    Série {gap.serie} · {rangeLabel(gap.number_start, gap.number_end)}
                  </p>
                  <p className="mt-0.5 text-xs text-gray-500">
                    {rangeSize(gap.number_start, gap.number_end) === 1
                      ? '1 número sem documento utilizável'
                      : `${rangeSize(gap.number_start, gap.number_end)} números sem documento utilizável`}
                  </p>
                </div>
                <Button variant="outline" onClick={() => openModal(gap)}>
                  Inutilizar
                </Button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="inut-heading" className="space-y-3">
        <h2 id="inut-heading" className="text-xs font-semibold uppercase tracking-wider text-gray-500">
          Faixas inutilizadas
        </h2>

        <TableShell
          ariaLabel={`Inutilizações de ${docLabel}`}
          minWidth={140}
          dimmed={isFetching && !isLoading}
          headers={['Ano / Série', 'Faixa', 'Justificativa', 'Status', 'Enviada em', {label: '', align: 'right'}]}
        >
          {isLoading ? (
            <tr>
              <td colSpan={6} className={TABLE_CELL}>
                <LoadingSkeleton/>
              </td>
            </tr>
          ) : items.length === 0 ? (
            <tr>
              <td colSpan={6} className={TABLE_CELL}>
                <EmptyState
                  title="Nenhuma inutilização registrada"
                  icon={<NfeIcon width={20} height={20}/>}
                  description={`As faixas de numeração de ${docLabel} inutilizadas junto à SEFAZ aparecerão aqui.`}
                />
              </td>
            </tr>
          ) : (
            items.map(item => (
              <tr key={item.sk} className={TABLE_ROW}>
                <td className={`${TABLE_CELL} font-mono text-xs text-gray-500`} data-label="Ano / Série">
                  {item.year} / {item.serie}
                </td>
                <td className={`${TABLE_CELL} font-mono text-sm text-gray-900`} data-label="Faixa">
                  {rangeLabel(item.number_start, item.number_end)}
                </td>
                <td className={`${TABLE_CELL} max-w-[36ch] text-gray-600`} data-label="Justificativa">
                  <span className="line-clamp-2">{item.justification}</span>
                </td>
                <td className={TABLE_CELL} data-label="Status">
                  <DfeStatusCell status={item.status} sefazMotive={item.sefaz_motive} gender="f"/>
                </td>
                <td className={`${TABLE_CELL} whitespace-nowrap text-xs text-gray-400`} data-label="Enviada em">
                  {formatDatetimeBR(item.created_at)}
                </td>
                <td className={`${TABLE_CELL} text-right`}>
                  <InutilizationXmlButton docType={docType} item={item}/>
                </td>
              </tr>
            ))
          )}
        </TableShell>

        {(hasNext || hasPrevious) && (
          <Pagination
            hasNext={hasNext} hasPrevious={hasPrevious}
            onNext={goNext} onPrevious={goPrevious} isLoading={isFetching}
          />
        )}
      </section>

      <Modal
        isOpen={draft !== null}
        title={`Inutilizar numeração de ${docLabel}`}
        onClose={closeModal}
        onSubmit={() => canSubmit && mutation.mutate()}
        submitLabel="Enviar à SEFAZ"
        cancelLabel="Voltar"
        danger
        loading={mutation.isPending}
        submitDisabled={!canSubmit}
      >
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Esta ação é <span className="font-medium text-red-600">irreversível</span>. Os números da faixa
            deixam de poder ser usados em qualquer {docLabel}.
          </p>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div>
              <label htmlFor="inut-serie" className="mb-1.5 block text-sm font-medium text-gray-700">
                Série
              </label>
              <NumericInput
                id="inut-serie"
                integerPlaces={3}
                className="w-full"
                value={String(draft?.serie ?? 0)}
                onChange={(v) => setDraft(d => d && {...d, serie: Number(v || 0)})}
              />
            </div>
            <div>
              <label htmlFor="inut-start" className="mb-1.5 block text-sm font-medium text-gray-700">
                Número inicial
              </label>
              <NumericInput
                id="inut-start"
                integerPlaces={9}
                className="w-full"
                value={draft?.start ?? ''}
                onChange={(v) => setDraft(d => d && {...d, start: v})}
              />
            </div>
            <div>
              <label htmlFor="inut-end" className="mb-1.5 block text-sm font-medium text-gray-700">
                Número final
              </label>
              <NumericInput
                id="inut-end"
                integerPlaces={9}
                className="w-full"
                value={draft?.end ?? ''}
                onChange={(v) => setDraft(d => d && {...d, end: v})}
              />
            </div>
          </div>

          {draft && draft.start !== '' && draft.end !== '' && !rangeValid && (
            <p className="text-xs text-red-600">
              O número final deve ser maior ou igual ao inicial.
            </p>
          )}
          {rangeValid && (
            <p className="text-xs text-gray-500">
              {rangeSize(start, end) === 1
                ? 'Será inutilizado 1 número.'
                : `Serão inutilizados ${rangeSize(start, end)} números.`}
            </p>
          )}

          <JustificationField
            id="inut-justification"
            value={justification}
            onChange={setJustification}
            minLength={INUT_JUSTIFICATION_MIN_LENGTH}
            maxLength={INUT_JUSTIFICATION_MAX_LENGTH}
            placeholder="Descreva o motivo da inutilização (mínimo 15 caracteres)…"
          />
        </div>
      </Modal>
    </div>
  )
}
