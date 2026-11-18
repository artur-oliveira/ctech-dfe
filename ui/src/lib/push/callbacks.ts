import type {PushCallbacks} from '@aoctech/push-client';
import {apiClient} from '@/lib/api/client';
import {pushDeviceId} from '@/lib/push/device';

/** Thrown by `getConfig` when the installation has no VAPID key; the push client turns it into the `error` status. */
export const PUSH_DISABLED = 'push-disabled';

/**
 * Wires `@aoctech/push-client` to the DF-e API. `save` sends only what the API binds (endpoint and keys, never the
 * client's own bookkeeping), and `remove` deletes by the id derived from the endpoint.
 */
export const pushCallbacks: PushCallbacks = {
  getConfig: async () => {
    const config = await apiClient.getPushConfig();
    if (!config.enabled || !config.public_key || !config.vapid_kid) throw new Error(PUSH_DISABLED);
    return {publicKey: config.public_key, kid: config.vapid_kid};
  },
  save: async (record) => {
    await apiClient.putPushSubscription({endpoint: record.endpoint, keys: record.keys});
  },
  remove: async (endpoint) => {
    await apiClient.deletePushDevice(await pushDeviceId(endpoint));
  },
};
