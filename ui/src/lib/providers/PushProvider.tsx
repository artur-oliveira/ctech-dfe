'use client';

import {createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode} from 'react';
import {useRouter} from 'next/navigation';
import {useQueryClient} from '@tanstack/react-query';
import {onNavigateMessage, registerServiceWorker, usePushSubscription, type PushCallbacks, type UsePushSubscriptionResult} from '@aoctech/push-client';
import {queryKeys} from '@/lib/api/query-keys';
import {STORAGE_KEY_PUSH_STAMP, STORAGE_KEY_PUSH_USER} from '@/lib/constants/storage';
import {useAuth} from '@/lib/hooks/useAuth';
import {pushCallbacks} from '@/lib/push/callbacks';
import {orgOfPath} from '@/lib/push/navigate';

/** Chromium's install prompt event; not in the DOM lib types. */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
}

export interface PushContextValue extends UsePushSubscriptionResult {
  /** The browser offered to install the app (Chromium); iPhone never fires this, see the install steps instead. */
  canInstall: boolean
  install: () => Promise<void>
}

const PushContext = createContext<PushContextValue | null>(null);

/** Push status and actions for this device; the single instance lives in `PushProvider`. */
export function usePush(): PushContextValue {
  const value = useContext(PushContext);
  if (!value) throw new Error('usePush precisa estar dentro do PushProvider');
  return value;
}

/**
 * Registers the push service worker, keeps this device's subscription fresh once there is a session
 * (`syncOnLoad`: it never prompts) and routes notification clicks.
 *
 * A click on a notification of another company carries `?org=<company_pk>`: the company is switched first, but only
 * to one the person is a member of, so a forged link cannot select a company they do not belong to.
 */
export function PushProvider({children}: { children: ReactNode }) {
  const {user, selectedOrg, setSelectedOrg} = useAuth();
  const router = useRouter();
  const qc = useQueryClient();

  useEffect(() => {
    void registerServiceWorker();
  }, []);

  // Switch to the company a link names, when the person belongs to it.
  const selectOrgOf = useCallback((path: string) => {
    const orgPk = orgOfPath(path);
    if (!orgPk || orgPk === selectedOrg?.pk) return;
    const member = user?.organizations.find((o) => o.pk === orgPk);
    if (member) setSelectedOrg(member);
  }, [user, selectedOrg?.pk, setSelectedOrg]);

  useEffect(() => onNavigateMessage((path) => {
    selectOrgOf(path);
    router.push(path);
  }), [router, selectOrgOf]);

  // A cold start from a notification (`openWindow`) lands on the link itself: honour its company once, after login.
  // Once only: later changes of the selected company are the person's own choice.
  const coldStartHandled = useRef(false);
  useEffect(() => {
    if (!user || coldStartHandled.current) return;
    coldStartHandled.current = true;
    selectOrgOf(`${window.location.pathname}${window.location.search}`);
  }, [user, selectOrgOf]);

  const callbacks = useMemo<PushCallbacks>(() => ({
    getConfig: pushCallbacks.getConfig,
    save: async (record) => {
      await pushCallbacks.save(record);
      void qc.invalidateQueries({queryKey: queryKeys.push.devices()});
    },
    remove: async (endpoint) => {
      await pushCallbacks.remove(endpoint);
      void qc.invalidateQueries({queryKey: queryKeys.push.devices()});
    },
  }), [qc]);

  // One browser, one person. The sync stamp is per browser, so a different person signing in on a shared browser would be
  // told "synced 5 minutes ago" and the subscription would stay on the previous person's row. Dropping the stamp when
  // the signed-in person changes makes `syncOnLoad` resend it, and the API moves the endpoint to the current person.
  // Declared before `usePushSubscription` on purpose: effects run in order, so this one runs before the sync.
  const userId = user?.user_id;
  useEffect(() => {
    if (!userId) return;
    try {
      if (localStorage.getItem(STORAGE_KEY_PUSH_USER) !== userId) {
        localStorage.removeItem(STORAGE_KEY_PUSH_STAMP);
        localStorage.setItem(STORAGE_KEY_PUSH_USER, userId);
      }
    } catch {
      // Storage unavailable: the stamp's own 24 h expiry still resends the subscription.
    }
  }, [userId]);

  // The API needs the company header on every write, so wait for one.
  const push = usePushSubscription({...callbacks, enabled: !!user && !!selectedOrg});

  // `beforeinstallprompt` fires once, early, usually before the notifications screen exists: keep it here.
  const [installEvent, setInstallEvent] = useState<BeforeInstallPromptEvent | null>(null);
  useEffect(() => {
    const onPrompt = (e: Event) => {
      e.preventDefault();
      setInstallEvent(e as BeforeInstallPromptEvent);
    };
    const onInstalled = () => setInstallEvent(null);
    window.addEventListener('beforeinstallprompt', onPrompt);
    window.addEventListener('appinstalled', onInstalled);
    return () => {
      window.removeEventListener('beforeinstallprompt', onPrompt);
      window.removeEventListener('appinstalled', onInstalled);
    };
  }, []);
  const install = useCallback(async () => {
    if (!installEvent) return;
    await installEvent.prompt();
    setInstallEvent(null);
  }, [installEvent]);

  const value = useMemo<PushContextValue>(
    () => ({...push, canInstall: installEvent !== null, install}),
    [push, installEvent, install],
  );

  return <PushContext.Provider value={value}>{children}</PushContext.Provider>;
}
