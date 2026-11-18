'use client';

import type {ReactNode} from 'react';
import type {PushStatus} from '@aoctech/push-client';
import {Button, Skeleton} from '@aoctech/ui';
import {PUSH_IOS_INSTALL_STEPS} from '@/lib/constants/push';

export type PermissionCardStatus = PushStatus | 'loading'

interface NotificationPermissionCardProps {
  status: PermissionCardStatus
  /** False: this installation has no push key; nothing can be turned on. */
  available: boolean
  /** A subscribe/unsubscribe/test request is in flight. */
  busy: boolean
  onSubscribe: () => void
  onUnsubscribe: () => void
  onTest: () => void
  canInstall: boolean
  onInstall: () => void
}

function Panel({title, children}: { title: string; children: ReactNode }) {
  return (
    <section className="rounded-xl border border-gray-200 bg-white p-4 md:p-6" aria-labelledby="push-card-title">
      <h2 id="push-card-title" className="text-base font-semibold text-gray-900">{title}</h2>
      <div className="mt-2 space-y-3 text-sm text-gray-600">{children}</div>
    </section>
  );
}

/**
 * This device's notification permission, one block per state. Presentational only: the state and the actions come
 * from the page (`usePush`), and turning on is always a click (the browser refuses a permission prompt without one).
 */
export function NotificationPermissionCard({status, available, busy, onSubscribe, onUnsubscribe, onTest, canInstall, onInstall}: NotificationPermissionCardProps) {
  if (status === 'loading') return <Skeleton className="h-32 rounded-xl"/>;

  if (!available) {
    return (
      <Panel title="Notificações indisponíveis">
        <p>As notificações ainda não estão disponíveis nesta instalação. Quando forem ligadas, esta tela passa a mostrar o botão de ativar.</p>
      </Panel>
    );
  }

  const install = canInstall && (
    <Button variant="outline" className="h-11 w-full sm:h-10 sm:w-auto" onClick={onInstall}>Instalar o aplicativo</Button>
  );

  switch (status) {
    case 'unsupported':
      return (
        <Panel title="Este navegador não recebe notificações">
          <p>Para receber avisos fora desta aba, use uma versão recente do Chrome, Edge, Firefox ou Safari.</p>
        </Panel>
      );
    case 'needs-install':
      return (
        <Panel title="Instale o aplicativo para receber notificações">
          <p>No iPhone e no iPad as notificações só funcionam com o site na Tela de Início (iOS 16.4 ou mais).</p>
          <ol className="list-decimal space-y-1 pl-5">
            {PUSH_IOS_INSTALL_STEPS.map((step) => <li key={step}>{step}</li>)}
          </ol>
        </Panel>
      );
    case 'denied':
      return (
        <Panel title="As notificações estão bloqueadas">
          <p>
            Este navegador bloqueou os avisos do CTech DF-e. Para voltar a receber, libere as notificações para este site
            nas configurações do navegador (no cadeado ao lado do endereço) e recarregue a página.
          </p>
        </Panel>
      );
    case 'subscribed':
      return (
        <Panel title="Notificações ativas neste dispositivo">
          <p>Este dispositivo recebe os avisos escolhidos abaixo, mesmo com o CTech DF-e fechado.</p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Button variant="outline" className="h-11 sm:h-10" disabled={busy} onClick={onTest}>Enviar teste</Button>
            <Button variant="outline" className="h-11 sm:h-10" disabled={busy} onClick={onUnsubscribe}>Desativar neste dispositivo</Button>
          </div>
        </Panel>
      );
    case 'error':
      return (
        <Panel title="Não foi possível ativar">
          <p role="alert">Algo falhou ao ativar as notificações neste dispositivo. Tente de novo; se continuar, recarregue a página.</p>
          <Button className="h-11 w-full sm:h-10 sm:w-auto" disabled={busy} onClick={onSubscribe}>Tentar de novo</Button>
        </Panel>
      );
    default:
      return (
        <Panel title="Receba avisos fora desta aba">
          <p>
            Ative as notificações neste dispositivo para saber quando um documento é autorizado, rejeitado ou quando chega uma nota
            emitida contra a sua empresa. O aviso nunca traz valores nem dados do cliente.
          </p>
          <div className="flex flex-col gap-2 sm:flex-row">
            <Button className="h-11 sm:h-10" disabled={busy} onClick={onSubscribe}>Ativar neste dispositivo</Button>
            {install}
          </div>
        </Panel>
      );
  }
}
