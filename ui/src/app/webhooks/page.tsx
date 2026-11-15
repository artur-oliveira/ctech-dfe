'use client';

import {useMemo, useState} from 'react';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {Button, PageHeader, Skeleton} from '@aoctech/ui';
import {toast} from 'sonner';
import {apiClient, ApiError} from '@/lib/api/client';
import {queryKeys} from '@/lib/api/query-keys';
import {useAuth} from '@/lib/hooks/useAuth';
import {useEntityDelete} from '@/lib/hooks/useEntityDelete';
import {ProtectedRoute} from '@/components/ProtectedRoute';
import {RootLayout} from '@/components/layout/RootLayout';
import {NoOrgBanner} from '@/components/ui/no-org-banner';
import {NoPermission} from '@/components/ui/no-permission';
import {SlideOver} from '@/components/ui/slide-over';
import {WebhookForm, type CompanyOption} from '@/components/webhooks/WebhookForm';
import {SecretReveal} from '@/components/webhooks/SecretReveal';
import {WebhookRow} from '@/components/webhooks/WebhookRow';
import {ROLE_ADMIN, ROLE_OWNER} from '@/lib/data/roles';
import {describeChallengeError} from '@/lib/webhooks/describe';
import type {WebhookEndpoint} from '@/lib/types/webhooks';

type Panel =
  | { kind: 'closed' }
  | { kind: 'form' }
  | { kind: 'secret'; secret: string; reason: 'created' | 'rotated' }

function WebhooksContent() {
  const {user, selectedOrg} = useAuth();
  const qc = useQueryClient();
  const pk = selectedOrg?.pk ?? '';
  const [panel, setPanel] = useState<Panel>({kind: 'closed'});
  const mayManage = selectedOrg?.role === ROLE_OWNER || selectedOrg?.role === ROLE_ADMIN;

  const list = useQuery({
    queryKey: queryKeys.webhooks.list(pk),
    queryFn: () => apiClient.listWebhooks(),
    enabled: !!pk && mayManage,
  });

  // Names come from the companies the person already has; the API sends only the keys of the organization's.
  const companyNames = useMemo(
    () => new Map((user?.organizations ?? []).map((o) => [o.pk, o.name] as const)),
    [user],
  );
  const companies: CompanyOption[] = (list.data?.company_pks ?? []).map((cpk) => ({pk: cpk, label: companyNames.get(cpk) ?? cpk}));

  const refresh = () => qc.invalidateQueries({queryKey: queryKeys.webhooks.list(pk)});

  const {handleDelete, filterVisible} = useEntityDelete<WebhookEndpoint>({
    mutationFn: (id) => apiClient.deleteWebhook(id),
    getId: (e) => e.id,
    getDeletedMessage: (e) => `Webhook ${new URL(e.url).hostname} removido`,
    onSuccess: refresh,
  });

  const ping = useMutation({
    mutationFn: (id: string) => apiClient.pingWebhook(id),
    onSuccess: (_d, id) => {
      toast.success('Evento de teste enviado. O resultado está em Entregas.');
      void qc.invalidateQueries({queryKey: queryKeys.webhooks.deliveries(pk, id)});
      void refresh();
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.detail : 'Não foi possível enviar o teste'),
  });
  const enable = useMutation({
    mutationFn: (id: string) => apiClient.enableWebhook(id),
    onSuccess: () => {
      toast.success('Webhook reativado');
      void refresh();
    },
    // A 422 is the verification failing again: say why in plain Portuguese; the endpoint stays disabled.
    onError: (e) => toast.error(
      e instanceof ApiError && e.status === 422 ? `O endereço ainda não passou na verificação. ${describeChallengeError(e.detail)}`
        : e instanceof ApiError ? e.detail : 'Não foi possível reativar'),
  });
  const rotate = useMutation({
    mutationFn: (id: string) => apiClient.rotateWebhookSecret(id),
    onSuccess: (r) => setPanel({kind: 'secret', secret: r.secret, reason: 'rotated'}),
    onError: (e) => toast.error(e instanceof ApiError ? e.detail : 'Não foi possível trocar o segredo'),
  });

  if (!selectedOrg) return <NoOrgBanner/>;
  if (!mayManage) {
    return <div className="max-w-4xl p-4 md:p-8"><NoPermission what="gerenciar webhooks"/></div>;
  }

  const data = list.data;
  const endpoints = filterVisible(data?.data ?? []);
  const max = data?.max_endpoints ?? 5;
  const atLimit = endpoints.length >= max;
  const forbidden = list.error instanceof ApiError && list.error.status === 403;

  return (
    <div className="max-w-4xl p-4 md:p-8">
      <PageHeader
        title="Webhooks"
        description="Avise o seu sistema assim que um documento for autorizado, rejeitado ou falhar, sem precisar consultar."
        action={
          <Button className="h-11 sm:h-10" disabled={!data?.enabled || atLimit} onClick={() => setPanel({kind: 'form'})}>
            Novo webhook
          </Button>
        }
      />

      {list.isPending && !forbidden ? (
        <div className="mt-6 space-y-3">{[0, 1].map((i) => <Skeleton key={i} className="h-28 rounded-xl"/>)}</div>
      ) : forbidden ? (
        <div className="mt-6"><NoPermission what="gerenciar webhooks desta organização"/></div>
      ) : list.isError || !data ? (
        <div role="alert" className="mt-6 flex flex-wrap items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          Não foi possível carregar os webhooks.
          <Button variant="outline" size="sm" onClick={() => void list.refetch()}>Tentar de novo</Button>
        </div>
      ) : (
        <>
          {!data.enabled && (
            <p role="note" className="mt-6 rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-900">
              Webhooks ainda não estão disponíveis nesta instalação.
            </p>
          )}
          {data.enabled && atLimit && (
            <p className="mt-4 text-sm text-gray-500">
              Você usa {endpoints.length} de {max} webhooks. Remova um para cadastrar outro.
            </p>
          )}
          {endpoints.length === 0 ? (
            <section className="mt-6 rounded-xl border border-dashed border-gray-300 px-6 py-10 text-center">
              <h2 className="text-base font-semibold text-gray-900">Nenhum webhook cadastrado</h2>
              <p className="mx-auto mt-1 max-w-md text-sm text-gray-500">
                Cadastre o endereço do seu sistema. Validamos que ele responde, e a partir daí cada resultado chega assinado, com histórico de entregas e reenvio.
              </p>
              {data.enabled && (
                <Button className="mt-4 h-11 sm:h-10" onClick={() => setPanel({kind: 'form'})}>Cadastrar o primeiro</Button>
              )}
            </section>
          ) : (
            <ul className="mt-6 divide-y divide-gray-100 rounded-xl border border-gray-200">
              {endpoints.map((e) => (
                <WebhookRow
                  key={e.id}
                  endpoint={e}
                  companyPk={pk}
                  companyNames={companyNames}
                  busy={ping.isPending || rotate.isPending || enable.isPending}
                  onPing={() => ping.mutate(e.id)}
                  onEnable={() => enable.mutate(e.id)}
                  onRotate={() => rotate.mutate(e.id)}
                  onRemove={() => handleDelete(e)}
                />
              ))}
            </ul>
          )}
        </>
      )}

      <SlideOver
        open={panel.kind !== 'closed'}
        title={panel.kind === 'secret' ? 'Segredo de assinatura' : 'Novo webhook'}
        width="md"
        // The secret is shown once: closing by the backdrop or Esc would lose it, so only the button closes that panel.
        onClose={() => panel.kind === 'form' && setPanel({kind: 'closed'})}
      >
        {panel.kind === 'form' && (
          <WebhookForm
            companies={companies}
            onCancel={() => setPanel({kind: 'closed'})}
            onCreated={(created) => {
              setPanel({kind: 'secret', secret: created.secret, reason: 'created'});
              void refresh();
            }}
          />
        )}
        {panel.kind === 'secret' && (
          <SecretReveal secret={panel.secret} reason={panel.reason} onDone={() => setPanel({kind: 'closed'})}/>
        )}
      </SlideOver>
    </div>
  );
}

export default function WebhooksPage() {
  return (
    <ProtectedRoute>
      <RootLayout>
        <WebhooksContent/>
      </RootLayout>
    </ProtectedRoute>
  );
}
