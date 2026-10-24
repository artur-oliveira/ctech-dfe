import {Skeleton} from '@aoctech/ui';

interface LoadingSkeletonProps {
  count?: number
  height?: string
  rounded?: string
}

export function LoadingSkeleton({count = 4, height = 'h-12', rounded = 'rounded-lg'}: LoadingSkeletonProps) {
  return (
    <div className="space-y-2">
      {Array.from({length: count}, (_, i) => (
        <Skeleton key={i} className={`${height} ${rounded}`}/>
      ))}
    </div>
  );
}

export function DistributionSkeleton({rows = 5}: { rows?: number }) {
  return (
    <div className="divide-y divide-gray-100">
      {Array.from({length: rows}, (_, i) => (
        <div key={i} className="flex items-center gap-4 px-4 py-3">
          <Skeleton className="h-4 w-32"/>
          <Skeleton className="h-4 w-40"/>
          <Skeleton className="h-4 w-28"/>
        </div>
      ))}
    </div>
  );
}
