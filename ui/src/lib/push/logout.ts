import {apiClient} from '@/lib/api/client';
import {STORAGE_KEY_PUSH_STAMP, STORAGE_KEY_PUSH_USER} from '@/lib/constants/storage';
import {pushDeviceId} from '@/lib/push/device';

/** Logout must never wait on the network for long: past this the page goes on and leaves the rest to the push service. */
export const PUSH_LOGOUT_TIMEOUT_MS = 2000;

function forgetPushBookkeeping(): void {
  try {
    localStorage.removeItem(STORAGE_KEY_PUSH_STAMP);
    localStorage.removeItem(STORAGE_KEY_PUSH_USER);
  } catch {
    // Storage unavailable: the stamp's own 24 h expiry still makes the next login resend the subscription.
  }
}

/**
 * Stops this browser from notifying the person who is leaving (shared computers). In this order, because logging out
 * must not fail open:
 *   1. forget the sync stamp and the person it was for (synchronous), so the next login resends the subscription;
 *   2. cancel the browser subscription (local and fast): from here the push service answers 410 for it and the sender
 *      drops the server row by itself, even if nothing below succeeds;
 *   3. delete the server row while the token still exists, bounded by `timeoutMs`.
 * Best effort and never throws: logging out does not depend on the network. Also used when the session is found
 * missing (a refresh that fails for good, a 401 that cannot be renewed), where step 3 simply fails without a token.
 */
export async function removeThisDevicePush(timeoutMs = PUSH_LOGOUT_TIMEOUT_MS): Promise<void> {
  forgetPushBookkeeping();
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return;
  const work = (async () => {
    const registration = await navigator.serviceWorker.getRegistration('/');
    const subscription = await registration?.pushManager.getSubscription();
    if (!subscription) return;
    const id = await pushDeviceId(subscription.endpoint);
    await subscription.unsubscribe().catch(() => false);
    await apiClient.deletePushDevice(id).catch(() => undefined);
  })().catch(() => undefined);
  await Promise.race([work, new Promise<void>((resolve) => setTimeout(resolve, timeoutMs))]);
}
