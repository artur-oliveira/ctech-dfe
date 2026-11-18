import {apiClient} from '@/lib/api/client';
import {pushDeviceId} from '@/lib/push/device';

/** Logout must never wait on the network for long: past this the device row is left to the server's TTL. */
export const PUSH_LOGOUT_TIMEOUT_MS = 2000;

/**
 * Removes this device's push subscription (server row and browser subscription) before the session is revoked, so
 * the next person who signs in on a shared device does not receive the previous person's notifications. Best effort:
 * every failure is swallowed, because logging out is not allowed to depend on it.
 */
export async function removeThisDevicePush(timeoutMs = PUSH_LOGOUT_TIMEOUT_MS): Promise<void> {
  if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return;
  const work = (async () => {
    const registration = await navigator.serviceWorker.getRegistration('/');
    const subscription = await registration?.pushManager.getSubscription();
    if (!subscription) return;
    try {
      await apiClient.deletePushDevice(await pushDeviceId(subscription.endpoint));
    } catch {
      // The row expires by itself (`PushSubscriptionTTL`); the browser subscription is removed below either way.
    }
    await subscription.unsubscribe();
  })().catch(() => undefined);
  await Promise.race([work, new Promise<void>((resolve) => setTimeout(resolve, timeoutMs))]);
}
