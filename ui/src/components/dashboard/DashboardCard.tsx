export function DashboardCard({
                                title,
                                value,
                                icon,
                                color = 'primary'
                              }: {
  title: string
  value: string | number
  icon: string
  color?: 'primary' | 'green' | 'blue' | 'orange'
}) {
  const colorClasses = {
    primary: 'bg-primary-50 border-primary-200',
    green: 'bg-green-50 border-green-200',
    blue: 'bg-blue-50 border-blue-200',
    orange: 'bg-orange-50 border-orange-200',
  }

  return (
    <div className={`${colorClasses[color]} border rounded-lg p-6`}>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-gray-600 text-sm">{title}</p>
          <p className="text-3xl font-bold mt-2">{value}</p>
        </div>
        <span className="text-4xl">{icon}</span>
      </div>
    </div>
  )
}
