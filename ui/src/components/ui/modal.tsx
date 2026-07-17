'use client'

import {ReactNode, useEffect, useRef} from 'react'
import {createPortal} from 'react-dom'
import {Button} from '@/components/ui/button'

interface ModalProps {
  isOpen: boolean
  title: string
  children: ReactNode
  onClose: () => void
  onSubmit?: () => void
  submitLabel?: string
  cancelLabel?: string
  loading?: boolean
  danger?: boolean
  submitDisabled?: boolean
  /** Largura máxima do modal. 'md'=512px (padrão), 'lg'=672px, 'xl'=896px */
  size?: 'md' | 'lg' | 'xl'
}

const SIZE_CLASSES = {
  md: 'max-w-lg',
  lg: 'max-w-2xl',
  xl: 'max-w-4xl',
}

const FOCUSABLE =
  'a[href],button:not([disabled]),textarea,input:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])'

export function Modal({
                        isOpen,
                        title,
                        children,
                        onClose,
                        onSubmit,
                        submitLabel = 'Salvar',
                        cancelLabel = 'Cancelar',
                        loading = false,
                        danger = false,
                        submitDisabled = false,
                        size = 'md',
                      }: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null)
  const previouslyFocused = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!isOpen) return
    previouslyFocused.current = document.activeElement as HTMLElement | null
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const panel = panelRef.current
    const focusables = panel?.querySelectorAll<HTMLElement>(FOCUSABLE)
    const first = focusables && focusables.length ? focusables[0] : panel
    first?.focus()

    return () => {
      document.body.style.overflow = prevOverflow
      previouslyFocused.current?.focus?.()
    }
  }, [isOpen])

  useEffect(() => {
    if (!isOpen) return
    const panel = panelRef.current

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key === 'Tab' && panel) {
        const items = panel.querySelectorAll<HTMLElement>(FOCUSABLE)
        if (items.length === 0) return
        const firstEl = items[0]
        const lastEl = items[items.length - 1]
        if (e.shiftKey && document.activeElement === firstEl) {
          e.preventDefault()
          lastEl.focus()
        } else if (!e.shiftKey && document.activeElement === lastEl) {
          e.preventDefault()
          firstEl.focus()
        }
      }
    }
    document.addEventListener('keydown', onKeyDown)

    return () => document.removeEventListener('keydown', onKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null
  if (typeof document === 'undefined') return null

  const titleId = 'modal-title'

  return createPortal(
    <div
      className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center"
      onClick={onClose}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        className={`bg-white rounded-xl shadow-modal outline-none ${SIZE_CLASSES[size]} w-full mx-4 max-h-[90vh] overflow-y-auto`}
      >
        <div
          className="sticky top-0 bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between rounded-t-xl">
          <h2 id={titleId} className="text-lg font-semibold text-gray-900">{title}</h2>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600"
            aria-label="Fechar"
          >
            ×
          </Button>
        </div>

        <div className="p-6">{children}</div>

        <div
          className="border-t border-gray-200 px-6 py-4 flex justify-end gap-3 sticky bottom-0 bg-gray-50 rounded-b-xl">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
          >
            {cancelLabel}
          </Button>
          {onSubmit && (
            <Button
              type="button"
              variant={danger ? 'danger' : 'brand'}
              onClick={onSubmit}
              disabled={loading || submitDisabled}
            >
              {loading ? 'Aguarde...' : submitLabel}
            </Button>
          )}
        </div>
      </div>
    </div>,
    document.body
  )
}
