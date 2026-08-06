export interface Step<Id extends string> {
  id: Id
  label: string
}

export function StepIndicator<Id extends string>(
  {current, steps}: { current: Id; steps: readonly Step<Id>[] },
) {
  const idx = steps.findIndex(s => s.id === current)
  return (
    <div className="flex items-center gap-0 mb-6">
      {steps.map((step, i) => {
        const done = i < idx
        const active = i === idx
        return (
          <div key={step.id} className="flex items-center flex-1 last:flex-none"
               aria-current={active ? 'step' : undefined}>
            <div className="flex flex-col items-center gap-1 shrink-0">
              <div
                className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-semibold transition-colors ${
                  done ? 'bg-brand-600 text-white' : active ? 'bg-brand-600 text-white ring-2 ring-brand-200' : 'bg-gray-100 text-gray-400'
                }`}>
                {done ? '✓' : i + 1}
              </div>
              <span
                className={`text-xs sr-only sm:not-sr-only sm:block ${active ? 'text-brand-600 font-medium' : done ? 'text-gray-500' : 'text-gray-400'}`}>
                {step.label}
              </span>
            </div>
            {i < steps.length - 1 && (
              <div className={`flex-1 h-0.5 mx-2 ${i < idx ? 'bg-brand-500' : 'bg-gray-200'}`}/>
            )}
          </div>
        )
      })}
    </div>
  )
}
