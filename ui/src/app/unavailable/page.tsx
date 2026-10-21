'use client';

import {UnavailableScreen} from '@aoctech/ui';
import {takeMaintenanceReturn} from '@/lib/network/maintenance';

/**
 * Where a 503 or a sustained outage lands.
 *
 * The NetworkProvider keeps probing /v1.0/health, and the person goes back to
 * where they were as soon as it answers (the button probes now). It never
 * reloads the screen they came from: a reload during maintenance costs another
 * failed request and puts them right back here.
 */
export default function UnavailablePage() {
  return (
    <UnavailableScreen
      locale="pt-BR"
      className="min-h-screen justify-center"
      title="Em manutenção"
      description="O serviço está fora do ar por pouco tempo. Nenhum documento foi perdido, e nada que você já emitiu foi afetado. Você volta para onde estava assim que o serviço responder."
      labels={{retry: 'Verificar agora'}}
      onRecovered={() => window.location.replace(takeMaintenanceReturn())}
    />
  );
}
