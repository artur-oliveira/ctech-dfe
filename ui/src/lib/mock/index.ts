/**
 * Dev mock API entry point. Imported for its side effect from the root layout
 * (and only when `NEXT_PUBLIC_MOCK_API=true`) so the axios adapter is attached
 * before any provider effect runs. Exports the dev panel for layout use.
 */

import {apiClient} from '@/lib/api/client'
import {mockAdapter} from './handler'
import {MOCK_ENABLED} from './env'
import {initMockStateFromUrl} from './state'
import {MockDevPanel} from './MockDevPanel'

if (MOCK_ENABLED) {
  if (typeof window !== 'undefined') {
    initMockStateFromUrl(window.location.search)
  }
  apiClient.setAdapter(mockAdapter)
}

export {MockDevPanel, MOCK_ENABLED}
