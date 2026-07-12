import React, {useMemo} from 'react'

type Range = [number, number]

const DIACRITIC_RE = /\p{Diacritic}/gu

function normalizeStr(str: string): string {
  return str.normalize('NFD').replace(DIACRITIC_RE, '').toLowerCase()
}

// Builds normalized text + a map from each norm char index → original char index.
// Needed because NFD expansion changes char counts (e.g. "á" → "á").
function buildNormMap(text: string): { normText: string; origPos: Int32Array } {
  const origPos = new Int32Array(text.length * 2) // worst case: every char expands to 2
  let normText = ''
  let normIdx = 0
  for (let i = 0; i < text.length; i++) {
    const norm = normalizeStr(text[i])
    for (let j = 0; j < norm.length; j++) {
      normText += norm[j]
      origPos[normIdx++] = i
    }
  }
  return {normText, origPos: origPos.subarray(0, normIdx)}
}

function mergeRanges(ranges: Range[]): Range[] {
  if (!ranges.length) return []
  ranges.sort((a, b) => a[0] - b[0])
  const merged: Range[] = [ranges[0]]
  for (let i = 1; i < ranges.length; i++) {
    const last = merged[merged.length - 1]
    if (ranges[i][0] <= last[1]) last[1] = Math.max(last[1], ranges[i][1])
    else merged.push(ranges[i])
  }
  return merged
}

export function Highlighted({text, query}: { text: string; query: string }) {
  const ranges = useMemo((): Range[] => {
    const tokens = query.trim().split(/\s+/).filter(Boolean).map(normalizeStr)
    if (!tokens.length) return []

    const {normText, origPos} = buildNormMap(text)
    const found: Range[] = []

    for (const token of tokens) {
      let idx = 0
      while ((idx = normText.indexOf(token, idx)) !== -1) {
        found.push([origPos[idx], origPos[idx + token.length - 1] + 1])
        idx++
      }
    }

    return mergeRanges(found)
  }, [text, query])

  if (!ranges.length) return <>{text}</>

  const parts: React.ReactNode[] = []
  let last = 0
  for (const [start, end] of ranges) {
    if (start > last) parts.push(<span key={`t${last}`}>{text.slice(last, start)}</span>)
    parts.push(
      <mark key={`m${start}`} className="bg-yellow-200 text-yellow-900 rounded-[2px] px-[1px]">
        {text.slice(start, end)}
      </mark>
    )
    last = end
  }
  if (last < text.length) parts.push(<span key={`t${last}`}>{text.slice(last)}</span>)

  return <>{parts}</>
}
