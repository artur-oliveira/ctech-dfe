'use client'

import {useState} from 'react'
import {SystemState, SystemStateRetry} from '@/components/SystemState'
import {checkApiLiveness} from '@/lib/network/liveness'
import {takeMaintenanceReturn} from '@/lib/network/maintenance'

/**
 * Where a 503 lands.
 *
 * The retry asks the health probe rather than reloading the screen the person
 * came from: a reload during maintenance costs another failed request and puts
 * them right back here, having learned nothing.
 */
export default function UnavailablePage() {
  const [checking, setChecking] = useState(false)
  const [stillDown, setStillDown] = useState(false)

  const retry = async () => {
    if (checking) return
    setChecking(true)
    setStillDown(false)
    if (await checkApiLiveness()) {
      window.location.replace(takeMaintenanceReturn())
      return
    }
    setChecking(false)
    setStillDown(true)
  }

  return (
    <SystemState
      code="503"
      title="Em manutenção"
      description="O serviço está fora do ar por pouco tempo. Nenhum documento foi perdido, e nada que você já emitiu foi afetado."
      detail={
        checking
          ? 'Verificando se o serviço voltou…'
          : stillDown
            ? 'Ainda em manutenção. Tente de novo em alguns instantes.'
            : 'Você volta para onde estava assim que o serviço responder.'
      }
    >
      <SystemStateRetry onRetry={() => void retry()} label="Verificar agora"/>
    </SystemState>
  )
}
