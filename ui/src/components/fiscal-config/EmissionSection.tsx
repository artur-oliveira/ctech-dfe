'use client';

import {useState} from 'react';
import {toast} from 'sonner';
import {Button} from '@aoctech/ui';
import {EmitError} from '@/components/ui/emit-error';
import {JustificationField} from '@/components/ui/justification-field';
import {OptionsSelect} from '@/components/ui/options-select';
import {emitFailure, type EmitFailure} from '@/lib/billing/notice';
import {
  AUTO_CONTINGENCY_AVAILABLE,
  currentEmission,
  DOC_LABELS,
  EMISSION_NORMAL,
  EMISSION_REASON_MAX,
  EMISSION_REASON_MIN,
  type EmissionDoc,
  type EmissionFields,
  type EmissionType,
  emissionHint,
  emissionLabel,
  inContingency,
} from '@/lib/contingency';
import {useEmissionMutation} from '@/lib/hooks/useFiscalConfig';
import {formatDatetimeBR} from '@/lib/utils/dfe';

interface EmissionSectionProps {
  doc: EmissionDoc
  pk: string
  config: EmissionFields
}

/**
 * Forma de emissão do documento (normal ou contingência), gravada em `PUT /{doc}-emission`, separada do
 * "Salvar configuração" porque é outro endpoint e muda o que a próxima emissão faz. Só oferece o que a API
 * listou em `emission_options` para este documento.
 */
export function EmissionSection({doc, pk, config}: EmissionSectionProps) {
  const current = currentEmission(config);
  const mutation = useEmissionMutation(doc, pk);
  const [type, setType] = useState<EmissionType>(current);
  const [reason, setReason] = useState(config.contingency_reason ?? '');
  const [failure, setFailure] = useState<EmitFailure | null>(null);

  const listed = config.emission_options?.length ? config.emission_options : [EMISSION_NORMAL];
  const options = listed.includes(current) ? listed : [current, ...listed];
  const leavingNormal = type !== EMISSION_NORMAL;
  const trimmed = reason.trim();
  const reasonInvalid = leavingNormal && (trimmed.length < EMISSION_REASON_MIN || trimmed.length > EMISSION_REASON_MAX);
  const dirty = type !== current || (leavingNormal && trimmed !== (config.contingency_reason ?? '').trim());

  const save = async () => {
    setFailure(null);
    try {
      await mutation.mutateAsync({
        emission_type: type,
        auto_contingency: config.auto_contingency ?? false,
        ...(leavingNormal ? {justification: trimmed} : {}),
      });
      toast.success(type === EMISSION_NORMAL ? 'Emissão normal restabelecida.' : 'Forma de emissão alterada.');
    } catch (err) {
      setFailure(emitFailure(err, 'Não foi possível alterar a forma de emissão.', undefined, doc === 'nfce' ? undefined : doc));
    }
  };

  return (
    <section className="space-y-3 rounded-lg border border-gray-200 p-3" aria-label="Forma de emissão">
      <p className="text-xs font-semibold uppercase tracking-wider text-gray-400">Forma de emissão</p>

      {inContingency(config) && (
        <div role="status" className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          <p className="font-medium">
            Em contingência
            {config.contingency_since && ` desde ${formatDatetimeBR(config.contingency_since)}`}
            : {emissionLabel(current, doc)}
          </p>
          {config.contingency_reason && <p className="mt-1 text-pretty">Motivo: {config.contingency_reason}</p>}
        </div>
      )}

      <div>
        <label htmlFor={`${doc}-emission-type`} className="mb-1.5 block text-sm font-medium text-gray-700">
          Forma de emissão atual de {DOC_LABELS[doc]}
        </label>
        <OptionsSelect
          id={`${doc}-emission-type`}
          value={type}
          onValueChange={(v) => setType(v as EmissionType)}
          options={options.map((o) => ({value: o, label: emissionLabel(o, doc)}))}
          clearable={false}
          className="w-full sm:max-w-md"
        />
        <p className="mt-1 text-xs text-gray-500">{emissionHint(type)}</p>
      </div>

      {leavingNormal && (
        <div>
          <JustificationField
            id={`${doc}-emission-reason`}
            label="Motivo da contingência"
            value={reason}
            onChange={setReason}
            minLength={EMISSION_REASON_MIN}
            maxLength={EMISSION_REASON_MAX}
            rows={3}
            placeholder="Ex.: SEFAZ do estado fora do ar desde as 10h"
          />
          <p className="text-xs text-gray-500">Vai no XML de cada nota emitida nesta forma, e a data e hora de início é registrada ao confirmar.</p>
        </div>
      )}
      {!leavingNormal && inContingency(config) && (
        <p className="text-xs text-gray-500">
          Ao voltar para Normal, a data de início da contingência é apagada e as próximas notas seguem direto para a SEFAZ.
        </p>
      )}

      {'auto_contingency' in config && (
        <div>
          <label className="flex min-h-11 items-start gap-2 py-2">
            <input
              type="checkbox"
              checked={!!config.auto_contingency}
              disabled={!AUTO_CONTINGENCY_AVAILABLE}
              readOnly
              className="mt-0.5 size-4 rounded border-gray-300 text-brand-600 disabled:cursor-not-allowed"
            />
            <span className="text-sm text-gray-700">
              Entrar em contingência sozinho
              {!AUTO_CONTINGENCY_AVAILABLE && (
                <span className="ml-2 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">Disponível em breve</span>
              )}
              <span className="block text-xs text-gray-500">
                Quando a SEFAZ ficar fora do ar, o sistema troca a forma de emissão por conta própria e emite o documento de
                novo, com outra chave de acesso. Ao voltar para Normal, a data de início da contingência é apagada.
              </span>
            </span>
          </label>
        </div>
      )}

      <EmitError failure={failure}/>

      <div className="flex justify-end">
        <Button type="button" onClick={() => void save()} disabled={!dirty || reasonInvalid || mutation.isPending}
                className="min-h-11 sm:min-h-0">
          {mutation.isPending ? 'Salvando…' : 'Aplicar forma de emissão'}
        </Button>
      </div>
    </section>
  );
}
