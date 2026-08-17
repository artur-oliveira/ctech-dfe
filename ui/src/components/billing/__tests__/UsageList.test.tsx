import {describe, expect, it} from 'vitest'
import {render, screen} from '@testing-library/react'
import {UsageList} from '@/components/billing/UsageList'

describe('UsageList', () => {
  it('shows used against the limit for a capped meter', () => {
    render(<UsageList quotas={{nfe: 1000}} usage={{nfe: {used: 812, limit: 1000}}}/>)
    expect(screen.getByText('812')).toBeInTheDocument()
    expect(screen.getByText('1.000')).toBeInTheDocument()
  })

  it('writes "ilimitado" and draws no bar', () => {
    // A full bar next to "ilimitado" reads as "you are at your limit", which is
    // the opposite of what it means.
    const {container} = render(<UsageList quotas={{nfe: -1}} usage={{nfe: {used: 214, limit: -1}}}/>)
    expect(screen.getByText('ilimitado')).toBeInTheDocument()
    expect(container.querySelectorAll('.rounded-full')).toHaveLength(0)
  })

  it('lists only the meters the plan grants', () => {
    // Zero is "not included", not "you ran out" — listing it would advertise a
    // document type the plan refuses.
    render(<UsageList quotas={{nfe: 3, cte: 0}}/>)
    expect(screen.getByText('NF-e')).toBeInTheDocument()
    expect(screen.queryByText('CT-e')).not.toBeInTheDocument()
  })

  it('says so when the plan grants nothing at all', () => {
    render(<UsageList quotas={{}}/>)
    expect(screen.getByText(/não inclui nenhum documento/i)).toBeInTheDocument()
  })
})
