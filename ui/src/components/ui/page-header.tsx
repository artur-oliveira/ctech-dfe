import {ReactNode} from 'react'
import {Button} from '@/components/ui/button'

interface PageHeaderProps {
  title: string
  description: string
  action?: {
    label: string
    icon?: ReactNode
    onClick: () => void
  }
}

export function PageHeader({title, description, action}: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between mb-6 gap-4">
      <div>
        <h1 className="text-2xl font-semibold text-gray-900">{title}</h1>
        <p className="text-gray-500 text-sm mt-0.5">{description}</p>
      </div>
      {action && (
        <Button
          variant="brand"
          onClick={action.onClick}
          className="shrink-0 gap-1.5"
        >
          {action.icon}
          {action.label}
        </Button>
      )}
    </div>
  )
}
