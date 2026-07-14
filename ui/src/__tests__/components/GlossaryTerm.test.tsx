import {describe, it, expect} from 'vitest'
import {render, screen} from '@testing-library/react'
import {GlossaryTerm} from '@/components/ui/glossary-term'
import {GLOSSARY} from '@/lib/constants/glossary'

describe('GlossaryTerm', () => {
  it('renders children and an accessible trigger + definition for the term', () => {
    render(<GlossaryTerm term="cfop"><span>CFOP</span></GlossaryTerm>)

    // Label passed as children stays visible.
    expect(screen.getAllByText('CFOP').length).toBeGreaterThan(0)

    // Trigger is a labelled button wired to a popover.
    const trigger = screen.getByRole('button', {name: `O que é ${GLOSSARY.cfop.label}?`})
    expect(trigger.getAttribute('popovertarget')).toBeTruthy()

    // Definition copy is present in the DOM (closed popover is hidden until invoked).
    expect(screen.getByRole('tooltip', {hidden: true})).toHaveTextContent(GLOSSARY.cfop.definition)
  })
})
