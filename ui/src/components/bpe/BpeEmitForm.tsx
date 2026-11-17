'use client';

import {useState} from 'react';
import {HomologationBanner} from '@/components/ui/homologation-banner';
import {OptionsSelect} from '@/components/ui/options-select';
import {useAuth} from '@/lib/hooks/useAuth';
import {useFiscalConfig} from '@/lib/hooks/useFiscalConfig';
import {BPE_VARIANTS, type BpeVariant} from '@/lib/constants/bpe';
import {BpeTicketForm} from './BpeTicketForm';
import {BpeTmForm} from './BpeTmForm';

/** Escolhe a variante (comum, TA ou TM); cada uma tem corpo próprio na API, então o formulário remonta ao trocar. */
export function BpeEmitForm() {
  const {selectedOrg} = useAuth();
  const {config} = useFiscalConfig('bpe', selectedOrg?.pk);
  const [variant, setVariant] = useState<BpeVariant>('normal');
  return (
    <div className="space-y-6">
      <HomologationBanner environment={config?.environment}/>
      <div className="max-w-md">
        <label htmlFor="bpe-variant" className="mb-1 block text-sm font-medium text-gray-700">Tipo de bilhete</label>
        <OptionsSelect id="bpe-variant" value={variant} onValueChange={(v) => setVariant(v as BpeVariant)} options={[...BPE_VARIANTS]}/>
        {variant !== 'normal' && (
          <p className="mt-1 text-xs text-gray-500">O {variant === 'tm' ? 'TM' : 'TA'} exige credenciamento específico na SEFAZ.</p>
        )}
      </div>
      {variant === 'tm' ? <BpeTmForm key="tm"/> : <BpeTicketForm key={variant} variant={variant}/>}
    </div>
  );
}
