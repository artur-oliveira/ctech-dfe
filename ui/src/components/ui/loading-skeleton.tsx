interface LoadingSkeletonProps {
  count?: number
  height?: string
}

export function LoadingSkeleton({count = 4, height = 'h-12'}: LoadingSkeletonProps) {
  return (
    <div className="space-y-2">
      {Array.from({length: count}, (_, i) => (
        <div key={i} className={`${height} bg-gray-100 rounded-lg animate-pulse`}/>
      ))}
    </div>
  )
}

export function DistributionSkeleton({rows = 5}: {rows?: number}) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({length: rows}, (_, i) => (
        <div key={i} className="flex items-center gap-4 px-4 py-3">
          <div className="h-4 w-32 bg-gray-100 rounded animate-pulse"/>
          <div className="h-4 w-40 bg-gray-100 rounded animate-pulse"/>
          <div className="h-4 w-28 bg-gray-100 rounded animate-pulse"/>
        </div>
      ))}
    </div>
  )
}
