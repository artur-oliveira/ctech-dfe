import type {ReactNode} from 'react'
import {cn} from '@/lib/utils'

interface SectionCardProps {
  icon?: ReactNode
  title: string
  children: ReactNode
  className?: string
}

export function SectionCard({icon, title, children, className}: SectionCardProps) {
  return (
    <div className={cn('rounded-xl border border-gray-200 bg-white overflow-hidden', className)}>
      <div className="flex items-center gap-2 px-5 py-3.5 border-b border-gray-100 bg-gray-50/60">
        {icon && <span className="text-gray-400">{icon}</span>}
        <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500">{title}</h3>
      </div>
      <div className="p-5 space-y-4">{children}</div>
    </div>
  )
}
