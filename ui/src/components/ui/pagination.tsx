import {ChevronLeft, ChevronRight} from 'lucide-react'
import {Button} from '@/components/ui/button'
import {cn} from '@/lib/utils'

interface PaginationProps {
  hasNext: boolean
  hasPrevious: boolean
  onNext: () => void
  onPrevious: () => void
  isLoading?: boolean
  className?: string
}

export function Pagination({
                             hasNext,
                             hasPrevious,
                             onNext,
                             onPrevious,
                             isLoading = false,
                             className,
                           }: PaginationProps) {
  if (!hasNext && !hasPrevious) return null

  return (
    <div className={cn('flex items-center gap-4 justify-end pt-4', className)}>
      <Button
        variant="outline"
        size="sm"
        onClick={onPrevious}
        disabled={!hasPrevious || isLoading}
      >
        <ChevronLeft/>
      </Button>

      <Button
        variant="outline"
        size="sm"
        onClick={onNext}
        disabled={!hasNext || isLoading}
      >
        <ChevronRight/>
      </Button>
    </div>
  )
}
