import Fuse from 'fuse.js'
import {ALL_NCMS, type NcmEntry} from '@/lib/data/ncm'

const fuse = new Fuse<NcmEntry>(ALL_NCMS, {
  keys: [
    {name: 'code', weight: 0.45},
    {name: 'description', weight: 0.35},
    {name: 'search', weight: 0.2},
  ],
  threshold: 0.28,
  includeMatches: true,
  minMatchCharLength: 2,
  ignoreLocation: true,
})

self.onmessage = (e: MessageEvent<{ query: string; id: number }>) => {
  const {query, id} = e.data
  const q = query.trim()
  const results = q.length < 2 ? [] : fuse.search(q, {limit: 30})
  self.postMessage({id, results})
}
