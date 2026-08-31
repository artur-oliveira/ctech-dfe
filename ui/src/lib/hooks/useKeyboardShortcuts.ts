'use client'

import {useEffect} from 'react'

export type ShortcutSpec = {
  /** Single key (case-insensitive) or a KeyboardEvent.key value like '?' / 'Escape'. */
  key: string
  handler: (e: KeyboardEvent) => void
  /** Run even while a text field is focused (e.g. Escape). Default false. */
  global?: boolean
  /** Require a modifier (shift/ctrl/meta/alt). Default: handled by key string. */
  shift?: boolean
  /** Require Ctrl (Windows/Linux) or Cmd (macOS) — e.g. the ⌘K palette. */
  mod?: boolean
}

function isTypingTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false
  const tag = el.tagName
  return (
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    tag === 'SELECT' ||
    el.isContentEditable
  )
}

/**
 * Global keyboard shortcuts. A shortcut fires only when no modifier is held,
 * unless `global` (then it ignores the typing guard too — for Escape). Modifier
 * combos are passed through verbatim (e.g. key: '?' with shift: true).
 */
export function useKeyboardShortcuts(shortcuts: ShortcutSpec[], deps: unknown[] = []) {
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      for (const s of shortcuts) {
        const modOk = s.mod
          ? e.ctrlKey || e.metaKey
          : s.shift
            ? e.shiftKey
            : !e.ctrlKey && !e.metaKey && !e.altKey
        if (e.key.toLowerCase() !== s.key.toLowerCase()) continue
        if (!modOk) continue
        if (!s.global && isTypingTarget(e.target)) return
        e.preventDefault()
        s.handler(e)
        return
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
}
