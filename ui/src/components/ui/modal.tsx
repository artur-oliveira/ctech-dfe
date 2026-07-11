'use client'

import {ReactNode} from 'react'
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
  if (!isOpen) return null
  if (typeof document === 'undefined') return null

  return createPortal(
    <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center">
      <div className={`bg-white rounded-xl shadow-xl ${SIZE_CLASSES[size]} w-full mx-4 max-h-[90vh] overflow-y-auto`}>
        <div className="sticky top-0 bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between rounded-t-xl">
          <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
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

        <div className="border-t border-gray-200 px-6 py-4 flex justify-end gap-3 sticky bottom-0 bg-gray-50 rounded-b-xl">
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
