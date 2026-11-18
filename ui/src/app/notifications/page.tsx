'use client';

import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Button, PageHeader, Skeleton} from '@aoctech/ui';
import {toast} from 'sonner';
import {apiClient, ApiError} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {usePush} from '@/lib/providers/PushProvider';
import {currentPushDeviceId} from '@/lib/push/device';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {NoPermission} from '@/components/ui/no-permission';
import {DeviceList} from '@/components/notifications/DeviceList';
import {NotificationPermissionCard} from '@/components/notifications/NotificationPermissionCard';
import {NotificationPreferences} from '@/components/notifications/NotificationPreferences';
import type {PushDevice} from '@/lib/types/push';

const MSG_TEST_THROTTLED = 'Já enviamos um teste há pouco. Aguarde um minuto para pedir outro.';

const failure = (e: unknown, fallback: string) => (e instanceof ApiError ? e.detail : fallback);

function NotificationsContent() {
  const {selectedOrg} = useAuth();
  const qc = useQueryClient();
  const push = usePush();
  const pk = selectedOrg?.pk ?? '';

  const config = useQuery({queryKey: queryKeys.push.config(), queryFn: () => apiClient.getPushConfig()});
  const available = config.data?.enabled === true;
  const devices = useQuery({queryKey: queryKeys.push.devices(), queryFn: () => apiClient.listPushDevices(), enabled: available});
  const preferences = useQuery({
    queryKey: queryKeys.push.preferences(pk),
    queryFn: () => apiClient.getPushPreferences(),
    enabled: !!pk && available,
  });
  // Which row is this browser: derived from the subscription, since the API never returns endpoints.
  const thisDevice = useQuery({
    queryKey: [...queryKeys.push.devices(), 'this', push.status],
    queryFn: () => currentPushDeviceId(),
    enabled: push.status === 'subscribed',
  });

  const savePreferences = useMutation({
    mutationFn: (events: string[]) => apiClient.putPushPreferences(events),
    onSuccess: (saved) => qc.setQueryData(queryKeys.push.preferences(pk), saved),
    onError: (e) => toast.error(failure(e, 'Não foi possível salvar a escolha')),
  });
  const sendTest = useMutation({
    mutationFn: () => apiClient.sendPushTest(),
    onSuccess: () => toast.success('Teste enviado. Ele chega em instantes neste e nos outros dispositivos.'),
    onError: (e) => toast.error(e instanceof ApiError && e.status === 429 ? MSG_TEST_THROTTLED : failure(e, 'Não foi possível enviar o teste')),
  });
  // A new device starts with the defaults on the server, so both lists change when one is turned on or off.
  const refreshAll = () => Promise.all([
    qc.invalidateQueries({queryKey: queryKeys.push.devices()}),
    qc.invalidateQueries({queryKey: queryKeys.push.preferences(pk)}),
  ]);
  const toggleDevice = useMutation({
    mutationFn: (on: boolean) => (on ? push.subscribe() : push.unsubscribe()),
    onSuccess: refreshAll,
    onError: () => toast.error('Não foi possível concluir. Tente de novo.'),
  });
  const removeDevice = useMutation({
    // This browser's own row goes through the push client, so the browser subscription is removed with it.
    mutationFn: async (device: PushDevice) => {
      if (device.id === thisDevice.data) await push.unsubscribe();
      else await apiClient.deletePushDevice(device.id);
    },
    onSuccess: () => {
      toast.success('Dispositivo removido');
      return refreshAll();
    },
    onError: (e) => toast.error(failure(e, 'Não foi possível remover o dispositivo')),
  });

  if (!selectedOrg) return <NoOrgBanner/>;

  const forbidden = config.error instanceof ApiError && config.error.status === 403;
  const busy = toggleDevice.isPending || sendTest.isPending || removeDevice.isPending;

  return (
    <div className="max-w-3xl p-4 md:p-8">
      <PageHeader
        title="Notificações"
        description="Receba um aviso no celular ou no computador quando um documento for autorizado, rejeitado ou quando chegar uma nota para a sua empresa."
      />

      {config.isPending ? (
        <div className="mt-6 space-y-3">{[0, 1].map((i) => <Skeleton key={i} className="h-28 rounded-xl"/>)}</div>
      ) : forbidden ? (
        <div className="mt-6"><NoPermission what="gerenciar as notificações"/></div>
      ) : config.isError || !config.data ? (
        <div role="alert" className="mt-6 flex flex-wrap items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Não foi possível carregar as notificações.
          <Button variant="outline" size="sm" onClick={() => void config.refetch()}>Tentar de novo</Button>
        </div>
      ) : (
        <div className="mt-6 space-y-4">
          <NotificationPermissionCard
            status={push.status}
            available={available}
            busy={busy}
            onSubscribe={() => toggleDevice.mutate(true)}
            onUnsubscribe={() => toggleDevice.mutate(false)}
            onTest={() => sendTest.mutate()}
            canInstall={push.canInstall}
            onInstall={() => void push.install()}
          />
          {available && (
            preferences.isPending ? (
              <Skeleton className="h-40 rounded-xl"/>
            ) : preferences.isError ? (
              <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                Não foi possível carregar a sua escolha de avisos.
                <Button variant="outline" size="sm" onClick={() => void preferences.refetch()}>Tentar de novo</Button>
              </div>
            ) : (
              <NotificationPreferences
                offered={config.data.event_types}
                selected={preferences.data?.events ?? []}
                disabled={savePreferences.isPending}
                companyName={selectedOrg.name}
                onChange={(events) => savePreferences.mutate(events)}
              />
            )
          )}
          {available && (
            devices.isPending ? (
              <Skeleton className="h-32 rounded-xl"/>
            ) : devices.isError ? (
              <div role="alert" className="flex flex-wrap items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
                Não foi possível carregar os seus dispositivos.
                <Button variant="outline" size="sm" onClick={() => void devices.refetch()}>Tentar de novo</Button>
              </div>
            ) : (
              <DeviceList
                devices={devices.data?.data ?? []}
                max={devices.data?.max_subscriptions ?? config.data.max_subscriptions}
                thisDeviceId={thisDevice.data ?? null}
                busy={busy}
                onRemove={(d) => removeDevice.mutate(d)}
              />
            )
          )}
        </div>
      )}
    </div>
  );
}

export default function NotificationsPage() {
  return (
    <ProtectedRoute>
      <RootLayout>
        <NotificationsContent/>
      </RootLayout>
    </ProtectedRoute>
  );
}
