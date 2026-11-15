'use client';

import {useState} from 'react';
import {useMutation} from '@tanstack/react-query';
import {Button} from '@aoctech/ui';
import {apiClient, ApiError} from '@/lib/api/client';
import {Input} from '@/components/ui/input';
import {CheckboxField} from '@/components/ui/checkbox-field';
import {WEBHOOK_EVENTS} from '@/lib/constants/webhooks';
import {describeChallengeError, validateWebhookUrl} from '@/lib/webhooks/describe';
import type {WebhookCreated} from '@/lib/types/webhooks';

const URL_FIELD_ID = 'wh-url';
const URL_HELP_ID = 'wh-url-help';
const URL_ERROR_ID = 'wh-url-error';

export interface CompanyOption {
  pk: string
  label: string
}

interface WebhookFormProps {
  /** The organization's companies the filter may pick from. */
  companies: CompanyOption[]
  onCreated: (created: WebhookCreated) => void
  onCancel: () => void
}

/** What the person sees for a refused registration; the server's text for the cases it words itself. */
function describeFailure(e: unknown): { title: string; detail: string } {
  if (!(e instanceof ApiError)) return {title: 'Não foi possível cadastrar', detail: 'Tente novamente em instantes.'};
  switch (e.status) {
    case 422:
      return {
        title: 'O endereço não passou na verificação',
        detail: `${describeChallengeError(e.detail)} Confira se o seu sistema responde à verificação (veja o guia) e tente de novo.`,
      };
    case 409:
      return {title: 'Limite de webhooks atingido', detail: 'Remova um webhook para cadastrar outro.'};
    case 501:
      return {title: 'Webhooks indisponíveis', detail: 'Esta instalação ainda não habilitou webhooks.'};
    default:
      return {title: 'Não foi possível cadastrar', detail: e.detail || 'Tente novamente em instantes.'};
  }
}

function toggled(set: Set<string>, value: string, on: boolean): Set<string> {
  const next = new Set(set);
  if (on) next.add(value); else next.delete(value);
  return next;
}

export function WebhookForm({companies, onCreated, onCancel}: WebhookFormProps) {
  const [url, setUrl] = useState('');
  const [urlError, setUrlError] = useState<string | undefined>();
  const [events, setEvents] = useState<Set<string>>(new Set());
  const [companyPks, setCompanyPks] = useState<Set<string>>(new Set());

  const create = useMutation({
    mutationFn: () => apiClient.createWebhook({url: url.trim(), events: [...events], company_pks: [...companyPks]}),
    onSuccess: onCreated,
  });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const problem = validateWebhookUrl(url);
    setUrlError(problem ?? undefined);
    if (problem) return;
    create.mutate();
  }

  const failure = create.isError ? describeFailure(create.error) : null;

  return (
    <form onSubmit={submit} className="flex h-full flex-col" noValidate>
      <div className="flex-1 space-y-6 overflow-y-auto px-6 py-5">
        {/* Label wired to the field by hand: LabeledInput renders its <label> without htmlFor. */}
        <div>
          <label htmlFor={URL_FIELD_ID} className="mb-1 block text-sm font-medium text-gray-700">
            Endereço que recebe os avisos
          </label>
          <Input
            id={URL_FIELD_ID}
            type="url"
            inputMode="url"
            autoComplete="off"
            placeholder="https://seu-sistema.com.br/hooks/dfe"
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              if (urlError) setUrlError(undefined);
            }}
            onBlur={() => url && setUrlError(validateWebhookUrl(url) ?? undefined)}
            aria-invalid={!!urlError}
            aria-describedby={urlError ? URL_ERROR_ID : URL_HELP_ID}
            className={urlError ? 'border-red-500' : undefined}
          />
          {urlError
            ? <p id={URL_ERROR_ID} className="mt-1 text-sm text-red-600">{urlError}</p>
            : <p id={URL_HELP_ID} className="mt-1 text-sm text-gray-500">Precisa usar https e responder a uma verificação antes de ser ativado.</p>}
        </div>

        <fieldset>
          <legend className="text-sm font-medium text-gray-700">Quais eventos avisar</legend>
          <p className="mt-0.5 text-sm text-gray-500">Sem nenhum marcado, você recebe todos.</p>
          <div className="mt-2 space-y-1">
            {WEBHOOK_EVENTS.map((ev) => (
              <div key={ev.value}>
                <CheckboxField
                  id={`wh-event-${ev.value}`}
                  label={ev.label}
                  checked={events.has(ev.value)}
                  onCheckedChange={(on) => setEvents((s) => toggled(s, ev.value, on))}
                />
                <p className="ml-6 text-xs text-gray-500">{ev.hint}</p>
              </div>
            ))}
          </div>
        </fieldset>

        {companies.length > 1 && (
          <fieldset>
            <legend className="text-sm font-medium text-gray-700">De quais empresas</legend>
            <p className="mt-0.5 text-sm text-gray-500">Sem nenhuma marcada, vale para todas as empresas da organização.</p>
            <div className="mt-2 space-y-1">
              {companies.map((c) => (
                <CheckboxField
                  key={c.pk}
                  id={`wh-company-${c.pk}`}
                  label={c.label}
                  checked={companyPks.has(c.pk)}
                  onCheckedChange={(on) => setCompanyPks((s) => toggled(s, c.pk, on))}
                />
              ))}
            </div>
          </fieldset>
        )}

        {failure && (
          <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm">
            <p className="font-medium text-destructive">{failure.title}</p>
            <p className="mt-1 text-gray-700">{failure.detail}</p>
          </div>
        )}
      </div>

      <div className="flex flex-col-reverse gap-2 border-t border-gray-200 px-6 py-4 sm:flex-row sm:justify-end">
        <Button type="button" variant="outline" className="h-11 sm:h-10" onClick={onCancel} disabled={create.isPending}>
          Cancelar
        </Button>
        <Button type="submit" className="h-11 sm:h-10" disabled={create.isPending}>
          {create.isPending ? 'Validando o endereço...' : 'Cadastrar e validar'}
        </Button>
      </div>
    </form>
  );
}
