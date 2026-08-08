'use client'

import {useEffect, useRef, useState} from 'react'
import {STORAGE_KEY_EMIT_DRAFT_PREFIX} from '@/lib/constants/storage'

/** How long the form must be idle before the draft is written. */
const AUTOSAVE_DEBOUNCE_MS = 500

/** Drafts older than this are ignored — a week-old cart is noise, not work. */
const DRAFT_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000

interface StoredDraft<T> {
  savedAt: number
  state: T
}

function storageKey(docType: string, orgPk: string | undefined): string {
  return `${STORAGE_KEY_EMIT_DRAFT_PREFIX}_${docType}_${orgPk ?? 'none'}`
}

/** Reads a still-valid draft, dropping anything corrupt or expired. */
function loadDraft<T>(key: string): {state: T; savedAt: number} | null {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as StoredDraft<T>
    if (parsed?.savedAt && Date.now() - parsed.savedAt < DRAFT_MAX_AGE_MS) {
      return {state: parsed.state, savedAt: parsed.savedAt}
    }
    window.localStorage.removeItem(key)
  } catch {
    // Corrupt or unavailable storage is never worth breaking the form over.
    try {
      window.localStorage.removeItem(key)
    } catch { /* storage unavailable — nothing to clean up */ }
  }
  return null
}

export interface EmitDraft<T> {
  /** A recoverable draft found on mount, or null. Cleared once acted on. */
  recovered: {state: T; savedAt: number} | null
  /** Drop the recovered draft without applying it. */
  discard: () => void
  /** Acknowledge the recovered draft (the caller applies the state). */
  accept: () => void
  /** Remove the stored draft — call after a successful emission. */
  clear: () => void
}

/**
 * Autosaves an in-progress emission to localStorage, per document type and
 * organization, and surfaces any draft left behind by a refresh or a closed tab.
 *
 * Emission is irreversible but everything before it is disposable, so the form
 * state is worth keeping: a 20-item NF-e should survive an accidental reload.
 * Nothing is sent anywhere — this is local recovery only.
 */
export function useEmitDraft<T>(
  docType: string,
  orgPk: string | undefined,
  state: T,
  hasContent: boolean,
): EmitDraft<T> {
  const key = storageKey(docType, orgPk)
  const [recovered, setRecovered] = useState<{state: T; savedAt: number} | null>(() => loadDraft<T>(key))
  // Suspends autosave until the user has answered the recovery prompt, so an
  // empty form doesn't overwrite the draft it is offering to restore.
  const settled = useRef(recovered === null)

  // Autosave, debounced.
  useEffect(() => {
    if (!settled.current) return
    if (!hasContent) {
      window.localStorage.removeItem(key)
      return
    }
    const timer = window.setTimeout(() => {
      try {
        window.localStorage.setItem(key, JSON.stringify({savedAt: Date.now(), state} satisfies StoredDraft<T>))
      } catch {
        // Quota or private-mode failure — autosave is best-effort.
      }
    }, AUTOSAVE_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [key, state, hasContent])

  return {
    recovered,
    accept: () => {
      settled.current = true
      setRecovered(null)
    },
    discard: () => {
      settled.current = true
      window.localStorage.removeItem(key)
      setRecovered(null)
    },
    clear: () => {
      settled.current = true
      window.localStorage.removeItem(key)
      setRecovered(null)
    },
  }
}
